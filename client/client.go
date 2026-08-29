package client

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/MASAKi-cell/dns/message"
)

// DNS クエリを実行するスタブリゾルバ
type Client struct {
	config *Config
}

// 新しい Client を作成
func NewClient(opts ...Option) *Client {
	config := &Config{
		Timeout:    DefaultTimeout,
		MaxRetries: DefaultMaxRetries,
		RetryDelay: DefaultRetryDelay,
	}

	// 可変長引数で任意の数のオプションを受け取る
	for _, opt := range opts {
		opt(config)
	}

	return &Client{config: config}
}

// 指定した名前と TYPE で DNS クエリを実行する簡易API
func (c *Client) Query(ctx context.Context, name string, typ message.Type) (*message.Message, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}

	// 末尾のドットがなければ追加（FQDN形式）
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	msg := &message.Message{
		Header: message.Header{
			ID: id,
			RD: true, // 再帰的解決を要求（スタブリゾルバの為）
		},
		Questions: []message.Question{
			{
				Name:  message.Name(name),
				Type:  typ,
				Class: message.ClassIN,
			},
		},
	}

	return c.Exchange(ctx, msg)
}

// 構築済みの Message を送信し、応答を受信する詳細 API
// リトライと複数サーバーへのフェイルオーバーを行う
func (c *Client) Exchange(ctx context.Context, msg *message.Message) (*message.Message, error) {
	if len(c.config.Servers) == 0 {
		return nil, ErrNoServers
	}

	var lastErr error

	for _, server := range c.config.Servers {
		resp, err := c.exchangeWithRetry(ctx, msg, server)
		if err == nil {
			return resp, nil
		}
		lastErr = &ServerError{Server: server, Err: err}
	}

	return nil, fmt.Errorf("%w: %v", ErrAllServersFailed, lastErr)
}

// 単一サーバーに対してリトライ付きでクエリを実行する
func (c *Client) exchangeWithRetry(ctx context.Context, msg *message.Message, server string) (*message.Message, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 && c.config.RetryDelay > 0 {
			select {
			// キャンセル時は即座に終了
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
		}

		resp, err := c.exchange(ctx, msg, server)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

// 単一サーバーへの1回の送受信を行う
func (c *Client) exchange(ctx context.Context, msg *message.Message, server string) (*message.Message, error) {
	// タイムアウト付きのコンテキストを作成
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// UDP 接続
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "udp", server)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// メッセージをエンコード
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	// 送信
	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// 受信バッファ（UDP DNS は通常 512 バイト以下、EDNS0 で最大 4096 程度）
	buf := make([]byte, 4096)

	// タイムアウト設定
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetReadDeadline(deadline)
	}

	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	// レスポンスをデコード
	resp, err := message.Unmarshal(buf[:n])
	if err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	// ID の一致を確認
	if resp.Header.ID != msg.Header.ID {
		return nil, fmt.Errorf("id mismatch: sent %d, received %d", msg.Header.ID, resp.Header.ID)
	}

	return resp, nil
}

// ランダムな 16 ビット ID を生成する
func generateID() (uint16, error) {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return uint16(b[0])<<8 | uint16(b[1]), nil
}
