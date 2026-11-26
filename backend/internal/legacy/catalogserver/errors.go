package catalogserver

import (
	"context"
	"errors"

	"connectrpc.com/connect"
)

// connectError 把讀取的非業務錯誤轉成 gRPC 錯誤，讓呼叫方（Java）走自己的重試或補償：取消與逾時保留原狀態碼，其餘是 Internal。
// 輸入檢查的錯誤（ID 數超過上限、含零值）在 server 已先擋下，這裡不會遇到。
func connectError(err error) error {
	code := connect.CodeInternal
	switch {
	case errors.Is(err, context.Canceled):
		code = connect.CodeCanceled
	case errors.Is(err, context.DeadlineExceeded):
		code = connect.CodeDeadlineExceeded
	}
	return connect.NewError(code, err)
}
