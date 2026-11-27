package couponserver

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

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

func userMessage(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Message
	}
	return err.Error()
}
