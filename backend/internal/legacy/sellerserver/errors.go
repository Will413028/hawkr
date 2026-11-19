package sellerserver

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// kindCodes 是模組錯誤的分類對應的 gRPC 狀態碼；沒有分類的錯誤（apperr.KindInternal 與非 apperr 的錯誤）是 Internal。
var kindCodes = map[apperr.Kind]connect.Code{
	apperr.KindInternal:           connect.CodeInternal,
	apperr.KindInvalidInput:       connect.CodeInvalidArgument,
	apperr.KindNotFound:           connect.CodeNotFound,
	apperr.KindVersionConflict:    connect.CodeAborted,
	apperr.KindAlreadyExists:      connect.CodeAlreadyExists,
	apperr.KindFailedPrecondition: connect.CodeFailedPrecondition,
	apperr.KindUnauthenticated:    connect.CodeUnauthenticated,
	apperr.KindPermissionDenied:   connect.CodePermissionDenied,
	apperr.KindUnavailable:        connect.CodeUnavailable,
	apperr.KindResourceExhausted:  connect.CodeResourceExhausted,
}

// connectError 把模組的錯誤轉成 gRPC 錯誤（沒有業務碼的 RPC 用）：狀態碼由 apperr.KindOf 查 kindCodes。取消與逾時優先於分類。
func connectError(err error) error {
	code, ok := kindCodes[apperr.KindOf(err)]
	switch {
	case errors.Is(err, context.Canceled):
		code = connect.CodeCanceled
	case errors.Is(err, context.DeadlineExceeded):
		code = connect.CodeDeadlineExceeded
	case !ok:
		code = connect.CodeInternal
	}
	return connect.NewError(code, err)
}
