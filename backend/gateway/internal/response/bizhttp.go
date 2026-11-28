package response

import (
	"net/http"

	"github.com/Will413028/hawkr/go-pkg/envelope"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/gin-gonic/gin"
)

var bizHTTPStatus = map[int32]int{
	bizcode.DelegationInvalidCommand:       http.StatusBadRequest,
	bizcode.DelegationAdminSessionRequired: http.StatusUnauthorized,
	bizcode.DelegationCredentialInvalid:    http.StatusUnauthorized,
	bizcode.DelegationCapabilityDenied:     http.StatusForbidden,
	bizcode.DelegationTargetMismatch:       http.StatusForbidden,
	bizcode.DelegationAuthorizationRevoked: http.StatusForbidden,
	bizcode.DelegationStoreNotDelegable:    http.StatusForbidden,
	bizcode.DelegationResourceNotOwned:     http.StatusForbidden,
	bizcode.DelegationActorMismatch:        http.StatusForbidden,
	bizcode.DelegationSessionExpired:       http.StatusGone,
	bizcode.DelegationSessionRevoked:       http.StatusGone,
	bizcode.DelegationSessionNotFound:      http.StatusGone,
	bizcode.DelegationHandoffInvalid:       http.StatusGone,
	bizcode.DelegationStoreNotOperable:     http.StatusUnprocessableEntity,
	bizcode.DelegationRateLimited:          http.StatusTooManyRequests,
	bizcode.DelegationUpstreamUnavailable:  http.StatusFailedDependency,

	bizcode.DelegationAuthorizationAlreadyActive:     http.StatusConflict,
	bizcode.DelegationAuthorizationNotFound:          http.StatusNotFound,
	bizcode.DelegationCapabilityDependencyIncomplete: http.StatusBadRequest,
	bizcode.DelegationAdminNotAllowlisted:            http.StatusForbidden,

	bizcode.DelegationInternalContractMismatch: http.StatusFailedDependency,
	bizcode.DelegationResourceAlreadyInState:   http.StatusConflict,
	bizcode.DelegationFeatureDisabled:          http.StatusForbidden,

	bizcode.CertApplicationNotFound:         http.StatusNotFound,
	bizcode.CertApplicationVersionConflict:  http.StatusConflict,
	bizcode.CertApplicationStateInvalid:     http.StatusConflict,
	bizcode.CertReviewReasonRequired:        http.StatusBadRequest,
	bizcode.CertApplicationDuplicatePending: http.StatusConflict,

	bizcode.CertReverifyCooldown: http.StatusTooManyRequests,

	bizcode.InvalidPlanCode: http.StatusBadRequest,

	bizcode.PayuniMerchantProfileVersionConflict:      http.StatusPreconditionFailed,
	bizcode.PayuniMerchantProfileIdempotencyKeyReused: http.StatusConflict,
	bizcode.PayuniMerchantProfileNotActive:            http.StatusConflict,

	bizcode.SellerInvoiceObligationNotFound: http.StatusNotFound,
	bizcode.SettlementNotFound:              http.StatusNotFound,
	bizcode.SettlementHoldNotFound:          http.StatusNotFound,
	bizcode.LedgerEntryNotFound:             http.StatusNotFound,
	bizcode.ExceptionCaseNotFound:           http.StatusNotFound,
	bizcode.ExceptionCaseResolutionNotFound: http.StatusNotFound,

	bizcode.FulfillmentConsignmentAgreementRequired: http.StatusForbidden,
	bizcode.FulfillmentProviderNotEnabled:           http.StatusForbidden,
	bizcode.SettlementPayoutNotApplicable:           http.StatusForbidden,
	bizcode.ExceptionCaseReviewSelfApprovalDenied:   http.StatusForbidden,

	bizcode.FulfillmentCommerceModelUnknown:  http.StatusFailedDependency,
	bizcode.SettlementPayoutInputUnavailable: http.StatusFailedDependency,

	bizcode.SellerInvoiceObligationFileRequired:         http.StatusBadRequest,
	bizcode.SellerInvoiceObligationRejectReasonRequired: http.StatusBadRequest,
	bizcode.SettlementHoldReasonRequired:                http.StatusBadRequest,
	bizcode.LedgerAdjustmentReasonRequired:              http.StatusBadRequest,

	bizcode.FulfillmentOperationInProgress: http.StatusConflict,

	bizcode.FeatureAddressLookupDisabled: http.StatusFailedDependency,

	bizcode.FeaturePayuniMerchantDisabled:   http.StatusFailedDependency,
	bizcode.FeatureShippingProviderDisabled: http.StatusFailedDependency,
}

// StatusForBizCode 回傳業務碼對應的 HTTP status，未登錄者塌縮成 409。
func StatusForBizCode(code int32) int {
	if s, ok := bizHTTPStatus[code]; ok {
		return s
	}
	return http.StatusConflict
}

// BizStatus 送出「業務碼 + 登錄表決定的 HTTP status」的 envelope。
// 形狀與 BizDeny 完全相同（Code / Message / MessageZh / traceId），差別只在 status
// 由登錄表決定而不是恆為 409。
func BizStatus(c *gin.Context, code int32, message string) {
	if guard(c, code, nil) {
		return
	}
	c.JSON(StatusForBizCode(code), Envelope{
		Code:      code,
		Message:   message,
		MessageZh: message,
		TraceID:   envelope.ExtractTraceID(c),
	})
}

// AbortBizStatus 是 BizStatus 加上中止 handler chain，供 guard 使用：守門判定為拒絕
// 之後，後續 handler 一律不得執行。
func AbortBizStatus(c *gin.Context, code int32, message string) {
	c.Abort()
	BizStatus(c, code, message)
}

// BizStatusData 同 BizStatus 但帶 data。典型消費者是 9266（409 ＋ 版本衝突的 diff
// payload）。簽章刻意沒有 httpStatus 參數：多一個參數就等於多一條繞過登錄表的路徑。
func BizStatusData(c *gin.Context, code int32, message string, data interface{}) {
	if guard(c, code, data) {
		return
	}
	c.JSON(StatusForBizCode(code), Envelope{
		Code:      code,
		Message:   message,
		MessageZh: message,
		Data:      data,
		TraceID:   envelope.ExtractTraceID(c),
	})
}

func writeBizCodeEnvelopeMapped(c *gin.Context, fallbackStatus int, msg string) bool {
	code, bizMsg := envelope.ParseBizMessage(msg)
	if code <= 0 {
		return false
	}
	if guard(c, code, nil) {
		return true
	}
	status := fallbackStatus
	if s, ok := bizHTTPStatus[code]; ok {
		status = s
	}
	c.JSON(status, Envelope{
		Code:      code,
		Message:   bizMsg,
		MessageZh: bizMsg,
		TraceID:   envelope.ExtractTraceID(c),
	})
	return true
}
