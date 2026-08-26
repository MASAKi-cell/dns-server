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

// Option は Config を変更する関数。
type Option func(*Config)

// WithServers は使用する DNS サーバーを設定する。
func WithServers(servers ...string) Option {
	return func(c *Config) {
		c.Servers = servers
	}
}

// WithTimeout は1回のクエリのタイムアウトを設定する。
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithMaxRetries は最大リトライ回数を設定する。
func WithMaxRetries(n int) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

// WithRetryDelay はリトライ間隔を設定する。
func WithRetryDelay(d time.Duration) Option {
	return func(c *Config) {
		c.RetryDelay = d
	}
}
