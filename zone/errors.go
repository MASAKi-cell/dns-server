package zone

import "errors"

var (
	ErrMissingOrigin = errors.New("missing $ORIGIN directive")                             // $ORIGINディレクティブがない場合のエラー
	ErrMissingSOA    = errors.New("zone must have exactly one SOA record")                 // SOAレコードがない場合のエラー
	ErrMultipleSOA   = errors.New("zone must have exactly one SOA record, found multiple") // SOAレコードが複数ある場合のエラー
)
