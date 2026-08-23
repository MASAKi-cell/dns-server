package client

import (
	"errors"
	"fmt"
)

var (
	// ErrNoServers は DNS サーバーが設定されていない場合に返される。
	ErrNoServers = errors.New("no DNS servers configured")

	// ErrAllServersFailed は全ての DNS サーバーへのクエリが失敗した場合に返される。
	ErrAllServersFailed = errors.New("all DNS servers failed")

	// ErrTruncated はレスポンスが切り詰められている場合に返される。
	// 将来の TCP フォールバック実装で使用。
	ErrTruncated = errors.New("response truncated")
)

// ServerError は特定のサーバーへのクエリ失敗を表す。
type ServerError struct {
	Server string
	Err    error
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("server %s: %v", e.Server, e.Err)
}

func (e *ServerError) Unwrap() error {
	return e.Err
}
