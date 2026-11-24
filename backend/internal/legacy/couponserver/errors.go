package couponserver

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

// connectError 把模組的非業務錯誤轉成 gRPC 錯誤，讓呼叫方（Java）走自己的重試或補償：狀態碼由 apperr.KindOf 查 kindCodes。
// 取消與逾時優先於分類。
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

// userMessage 是回給買家看的原因：apperr 的 Message；不是 apperr 的錯誤（參數解析）用它自己的文字。
// 不用 err.Error()，它會帶分類前綴（"failed_precondition: …"）。
func userMessage(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Message
	}
	return err.Error()
}
