	srv := &server.Server{
		Addr:    *addr,
		Handler: handler,
	}

	// シグナルハンドラを設定
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("starting authoritative DNS server on %s", *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func loadZone(path string) (*zone.Zone, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return zone.Parse(f)
}

// AuthHandler は権威DNSサーバーのハンドラ。
type AuthHandler struct {
	zone *zone.Zone
}

// ServeDNS はDNSリクエストを処理する。
func (h *AuthHandler) ServeDNS(req *message.Message) *message.Message {
	// 標準クエリ以外は未実装
	if req.Header.Opcode != message.OpcodeQuery {
		return h.notImplemented(req)
	}

	// 質問がない場合はエラー
	if len(req.Questions) == 0 {
		return h.formatError(req)
	}

	// 最初の質問のみ処理（RFC1035では複数質問を許容しているが、実運用ではほぼ1つ）
	q := req.Questions[0]

	// このゾーンの管轄かチェック
	if !h.zone.IsAuthoritative(string(q.Name)) {
		return h.refused(req)
	}

	// レコードを検索
	answers := h.zone.Lookup(string(q.Name), q.Type)

	// レスポンスを構築
	resp := &message.Message{
		Header: message.Header{
			ID:     req.Header.ID,
			QR:     true,
			Opcode: req.Header.Opcode,
			AA:     true, // 権威サーバーなのでAA=true
			RD:     req.Header.RD,
			RA:     false, // 再帰は提供しない
		},
		Questions: req.Questions,
		Answers:   answers,
	}

	// レコードが見つからない場合
	if len(answers) == 0 {
		// 名前自体が存在しないか確認
		allRecords := h.zone.LookupAll(string(q.Name))
		if len(allRecords) == 0 {
			// 名前が存在しない（NXDOMAIN）
			resp.Header.RCode = message.RCodeNameError
		} else {
			// 名前は存在するが要求されたタイプがない（NOERROR, 空の応答）
			resp.Header.RCode = message.RCodeSuccess
		}

		// AuthorityセクションにSOAを追加（ネガティブキャッシュ用）
		if soa := h.zone.SOA(); soa != nil {
			resp.Authorities = []message.ResourceRecord{*soa}
		}
	} else {
		resp.Header.RCode = message.RCodeSuccess
	}

	return resp
}

func (h *AuthHandler) notImplemented(req *message.Message) *message.Message {
	return &message.Message{
		Header: message.Header{
			ID:     req.Header.ID,
			QR:     true,
			Opcode: req.Header.Opcode,
			RCode:  message.RCodeNotImplemented,
		},
		Questions: req.Questions,
	}
}

func (h *AuthHandler) formatError(req *message.Message) *message.Message {
	return &message.Message{
		Header: message.Header{
			ID:     req.Header.ID,
			QR:     true,
			Opcode: req.Header.Opcode,
			RCode:  message.RCodeFormatError,
		},
		Questions: req.Questions,
	}
}

func (h *AuthHandler) refused(req *message.Message) *message.Message {
	return &message.Message{
		Header: message.Header{
			ID:     req.Header.ID,
			QR:     true,
			Opcode: req.Header.Opcode,
			RCode:  message.RCodeRefused,
		},
		Questions: req.Questions,
	}
}
