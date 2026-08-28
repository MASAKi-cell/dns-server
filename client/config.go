package client

import "time"

// クライアントの設定を保持する。
type Config struct {
	Servers    []string      // DNS サーバーリスト (例: "8.8.8.8:53")
	Timeout    time.Duration // 1回のクエリのタイムアウト
	MaxRetries int           // 最大リトライ回数
	RetryDelay time.Duration // リトライ間隔
}

// デフォルト値
const (
	DefaultTimeout    = 2 * time.Second
	DefaultMaxRetries = 3
	DefaultRetryDelay = 0
)

type Option func(*Config)

// 使用する DNS サーバーを設定する。
func WithServers(servers ...string) Option {
	return func(c *Config) {
		c.Servers = servers
	}
}

// 1回のクエリのタイムアウトを設定する。
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// 最大リトライ回数を設定する。
func WithMaxRetries(n int) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

// リトライ間隔を設定する。
func WithRetryDelay(d time.Duration) Option {
	return func(c *Config) {
		c.RetryDelay = d
	}
}
