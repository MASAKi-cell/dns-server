// resolved は再帰DNSリゾルバのコマンドラインツール。
// クライアントからのクエリを受け、ルートサーバーから権威サーバーを辿って解決する。
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MASAKi-cell/dns/message"
	"github.com/MASAKi-cell/dns/resolver"
	"github.com/MASAKi-cell/dns/server"
)

func main() {
	addr := flag.String("addr", ":5353", "listen address")
	flag.Parse()

	// リゾルバを作成
	res := resolver.NewResolver()

	// ハンドラを作成
	handler := &ResolverHandler{resolver: res}

	// サーバーを起動
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

	log.Printf("starting recursive DNS resolver on %s", *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// 再帰リゾルバのハンドラ。
type ResolverHandler struct {
	resolver *resolver.Resolver
}

// DNSリクエストを処理する。
func (h *ResolverHandler) ServeDNS(req *message.Message) *message.Message {
	// 標準クエリ以外は未実装
	if req.Header.Opcode != message.OpcodeQuery {
		return h.notImplemented(req)
	}

	// 質問がない場合はエラー
	if len(req.Questions) == 0 {
		return h.formatError(req)
	}

	// 最初の質問のみ処理
	q := req.Questions[0]

	// 再帰解決を実行
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	answers, err := h.resolver.Resolve(ctx, string(q.Name), q.Type)

	// レスポンスを構築
	resp := &message.Message{
		Header: message.Header{
			ID:     req.Header.ID,
			QR:     true,
			Opcode: req.Header.Opcode,
			RD:     req.Header.RD,
			RA:     true, // 再帰利用可能
		},
		Questions: req.Questions,
	}

	if err != nil {
		log.Printf("resolve error for %s %s: %v", q.Name, q.Type, err)
		// エラーの種類に応じてRCodeを設定
		if isNXDomain(err) {
			resp.Header.RCode = message.RCodeNameError
		} else {
			resp.Header.RCode = message.RCodeServerFailure
		}
	} else {
		resp.Header.RCode = message.RCodeSuccess
		resp.Answers = answers
	}

	return resp
}

func (h *ResolverHandler) notImplemented(req *message.Message) *message.Message {
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

func (h *ResolverHandler) formatError(req *message.Message) *message.Message {
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

// NXDOMAINエラーかどうかを判定する。
func isNXDomain(err error) bool {
	if err == nil {
		return false
	}
	return len(err.Error()) >= 8 && err.Error()[:8] == "NXDOMAIN"
}
