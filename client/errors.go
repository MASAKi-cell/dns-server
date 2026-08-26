package client

import (
	"errors"
	"fmt"
)

var (
	ErrNoServers        = errors.New("no DNS servers configured") // DNS サーバーが設定されていない場合に使用。
	ErrAllServersFailed = errors.New("all DNS servers failed")    // 全ての DNS サーバーへのクエリが失敗した場合に使用。
	ErrTruncated        = errors.New("response truncated")        // レスポンスが切り詰められている場合に使用。
)

// 特定のサーバーへのクエリ失敗を表す。
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
