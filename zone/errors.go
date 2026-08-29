package zone

import "errors"

var (
	// ErrMissingOrigin は$ORIGINディレクティブがない場合のエラー
	ErrMissingOrigin = errors.New("missing $ORIGIN directive")

	// ErrMissingSOA はSOAレコードがない場合のエラー
	ErrMissingSOA = errors.New("zone must have exactly one SOA record")

	// ErrMultipleSOA はSOAレコードが複数ある場合のエラー
	ErrMultipleSOA = errors.New("zone must have exactly one SOA record, found multiple")
)
