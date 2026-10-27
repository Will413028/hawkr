package sellerserver

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// errorDomain 與對外 API 的 ErrorInfo.domain 相同。
const errorDomain = "hawkr"

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

// inviteReasons 是邀請的結果對應的 ErrorInfo.reason（列舉的名稱）。只有這四個結果帶 reason，其餘結果只靠狀態碼分辨；契約見
// proto 兩支邀請 rpc 的註解。
var inviteReasons = []struct {
	err    error
	reason pb.VendorPortalInviteErrorReason
}{
	{agent.ErrInviteRevoked, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_REVOKED},
	{agent.ErrProspectNotActive, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_PROSPECT_NOT_ACTIVE},
	{agent.ErrNoDeliverableEmail, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_NO_DELIVERABLE_EMAIL},
	{agent.ErrInviteSecretSuperseded, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_SECRET_SUPERSEDED},
}

// connectError 把模組的錯誤轉成 gRPC 錯誤（沒有業務碼的 RPC 用）：狀態碼由 apperr.KindOf 查 kindCodes，邀請的四個結果另帶
// ErrorInfo（grpc-go 的 status detail 相容）。取消與逾時優先於分類。用 errors.Is 比對 sentinel，不看訊息文字。
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
	ce := connect.NewError(code, err)
	for _, r := range inviteReasons {
		if !errors.Is(err, r.err) {
			continue
		}
		d, derr := connect.NewErrorDetail(&errdetails.ErrorInfo{Reason: r.reason.String(), Domain: errorDomain})
		if derr != nil {
			return connect.NewError(connect.CodeInternal, derr)
		}
		ce.AddDetail(d)
		break
	}
	return ce
}
