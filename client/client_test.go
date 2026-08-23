package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/MASAKi-cell/dns/message"
)

func TestNewClient_DefaultConfig(t *testing.T) {
	c := NewClient()

	if c.config.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", c.config.Timeout, DefaultTimeout)
	}
	if c.config.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %v, want %v", c.config.MaxRetries, DefaultMaxRetries)
	}
	if c.config.RetryDelay != DefaultRetryDelay {
		t.Errorf("RetryDelay = %v, want %v", c.config.RetryDelay, DefaultRetryDelay)
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	c := NewClient(
		WithServers("8.8.8.8:53", "8.8.4.4:53"),
		WithTimeout(5*time.Second),
		WithMaxRetries(5),
		WithRetryDelay(100*time.Millisecond),
	)

	if len(c.config.Servers) != 2 {
		t.Errorf("Servers count = %d, want 2", len(c.config.Servers))
	}
	if c.config.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", c.config.Timeout)
	}
	if c.config.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want 5", c.config.MaxRetries)
	}
	if c.config.RetryDelay != 100*time.Millisecond {
		t.Errorf("RetryDelay = %v, want 100ms", c.config.RetryDelay)
	}
}

func TestExchange_NoServers(t *testing.T) {
	c := NewClient() // サーバー未設定

	msg := &message.Message{
		Header: message.Header{ID: 0x1234},
	}

	_, err := c.Exchange(context.Background(), msg)
	if err != ErrNoServers {
		t.Errorf("err = %v, want ErrNoServers", err)
	}
}

// mockDNSServer はテスト用の UDP サーバーを起動する。
// 受信したクエリに対して固定のレスポンスを返す。
func mockDNSServer(t *testing.T, response []byte) (addr string, close func()) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}

	go func() {
		buf := make([]byte, 512)
		for {
			n, clientAddr, err := conn.ReadFrom(buf)
			if err != nil {
				return // 終了
			}

			// クエリの ID を取得してレスポンスにコピー
			if n >= 2 && len(response) >= 2 {
				responseCopy := make([]byte, len(response))
				copy(responseCopy, response)
				responseCopy[0] = buf[0] // ID 上位
				responseCopy[1] = buf[1] // ID 下位
				conn.WriteTo(responseCopy, clientAddr)
			}
		}
	}()

	return conn.LocalAddr().String(), func() { conn.Close() }
}

func TestExchange_Success(t *testing.T) {
	// A レコードのレスポンス（example.com -> 93.184.216.34）
	response := []byte{
		0x00, 0x00, // ID (will be overwritten)
		0x81, 0x80, // Flags: QR=1, RD=1, RA=1
		0x00, 0x01, // QDCOUNT: 1
		0x00, 0x01, // ANCOUNT: 1
		0x00, 0x00, // NSCOUNT: 0
		0x00, 0x00, // ARCOUNT: 0
		// Question: example.com A IN
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,       // null terminator
		0x00, 0x01, // TYPE A
		0x00, 0x01, // CLASS IN
		// Answer: example.com A 93.184.216.34
		0xc0, 0x0c, // Name (compression pointer to offset 12)
		0x00, 0x01, // TYPE A
		0x00, 0x01, // CLASS IN
		0x00, 0x00, 0x00, 0x3c, // TTL: 60
		0x00, 0x04, // RDLENGTH: 4
		93, 184, 216, 34, // RDATA: 93.184.216.34
	}

	addr, cleanup := mockDNSServer(t, response)
	defer cleanup()

	c := NewClient(
		WithServers(addr),
		WithTimeout(1*time.Second),
	)

	resp, err := c.Query(context.Background(), "example.com", message.TypeA)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if !resp.Header.QR {
		t.Error("QR should be true (response)")
	}
	if len(resp.Answers) != 1 {
		t.Fatalf("Answers count = %d, want 1", len(resp.Answers))
	}

	aData, ok := resp.Answers[0].RData.(message.AData)
	if !ok {
		t.Fatalf("RData type = %T, want AData", resp.Answers[0].RData)
	}

	expectedIP := [4]byte{93, 184, 216, 34}
	if aData.Address != expectedIP {
		t.Errorf("Address = %v, want %v", aData.Address, expectedIP)
	}
}

func TestExchange_Timeout(t *testing.T) {
	// 応答しないサーバー
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer conn.Close()
	// 読み取りはするが応答しない
	go func() {
		buf := make([]byte, 512)
		for {
			_, _, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			// 応答しない
		}
	}()

	c := NewClient(
		WithServers(conn.LocalAddr().String()),
		WithTimeout(100*time.Millisecond),
		WithMaxRetries(0), // リトライなし
	)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = c.Query(ctx, "example.com", message.TypeA)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}
