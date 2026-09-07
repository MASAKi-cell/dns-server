// UDP DNSサーバーの実装。
// リクエストを受信し、Handlerに処理を委譲してレスポンスを返す。
package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/MASAKi-cell/dns/message"
)

// DNSリクエストを処理してレスポンスを返す。
type Handler interface {
	// リクエストを処理してレスポンスを返す。
	// nilを返した場合、クライアントには何も送信されない。
	ServeDNS(req *message.Message) *message.Message
}

// 関数をHandlerとして使えるようにするアダプタ。
type HandlerFunc func(req *message.Message) *message.Message

func (f HandlerFunc) ServeDNS(req *message.Message) *message.Message {
	return f(req)
}

// UDP DNSサーバー。
type Server struct {
	Addr    string  // リッスンするアドレス（例: ":53", "127.0.0.1:5353"）
	Handler Handler // リクエストハンドラ

	conn     net.PacketConn
	mu       sync.Mutex
	shutdown bool
	wg       sync.WaitGroup
}

// サーバーを起動してリクエストの処理を開始する。
// Shutdown が呼ばれるまでブロックする。
func (s *Server) ListenAndServe() error {
	conn, err := net.ListenPacket("udp", s.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	return s.serve()
}

// 既存のPacketConnを使ってリクエストの処理を開始する。テスト用途に便利。
func (s *Server) Serve(conn net.PacketConn) error {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	return s.serve()
}

func (s *Server) serve() error {
	buf := make([]byte, 4096)

	for {
		n, addr, err := s.conn.ReadFrom(buf)
		if err != nil {
			s.mu.Lock()
			shutdown := s.shutdown
			s.mu.Unlock()

			if shutdown {
				return nil
			}
			log.Printf("read error: %v", err)
			continue
		}

		// リクエストをコピーしてゴルーチンで処理
		req := make([]byte, n)
		copy(req, buf[:n])

		s.wg.Add(1)
		go s.handleRequest(req, addr)
	}
}

func (s *Server) handleRequest(data []byte, addr net.Addr) {
	defer s.wg.Done()

	// リクエストをデコード
	req, err := message.Unmarshal(data)
	if err != nil {
		log.Printf("unmarshal error from %s: %v", addr, err)
		return
	}

	// ハンドラがなければServerFailure
	if s.Handler == nil {
		resp := s.serverFailure(req)
		s.sendResponse(resp, addr)
		return
	}

	// ハンドラを呼び出し
	resp := s.Handler.ServeDNS(req)
	if resp == nil {
		return
	}

	s.sendResponse(resp, addr)
}

func (s *Server) sendResponse(resp *message.Message, addr net.Addr) {
	data, err := resp.Marshal()
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn == nil {
		return
	}

	if _, err := conn.WriteTo(data, addr); err != nil {
		log.Printf("write error to %s: %v", addr, err)
	}
}

func (s *Server) serverFailure(req *message.Message) *message.Message {
	return &message.Message{
		Header: message.Header{
			ID:     req.Header.ID,
			QR:     true,
			Opcode: req.Header.Opcode,
			RCode:  message.RCodeServerFailure,
		},
		Questions: req.Questions,
	}
}

// サーバーを停止する。処理中のリクエストが完了するまで待機する。
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.shutdown = true
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		conn.Close()
	}

	// 処理中のリクエストの完了を待つ
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// サーバーがリッスンしているアドレスを返す。サーバー起動前はnilを返す。
func (s *Server) LocalAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil
	}
	return s.conn.LocalAddr()
}
