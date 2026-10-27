package model

import (
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// KYCResubmitUnavailableReason 是 SaveKYCDraft 與 SubmitKYC 此刻不能做的原因。兩個 RPC 能不能做是同一個條件（流程狀態的
// CanResubmit），原因與句子只在這裡產生。
type KYCResubmitUnavailableReason string

const (
	// ResubmitUnderReviewCancelable 申請送審中，PayUni 還沒收下店家，可以先取消再修改。
	ResubmitUnderReviewCancelable KYCResubmitUnavailableReason = "UNDER_REVIEW_CANCELABLE"
	// ResubmitUnderReview 申請送審中，PayUni 已收下店家，不能取消。
	ResubmitUnderReview KYCResubmitUnavailableReason = "UNDER_REVIEW"
	// ResubmitPayUniReviewing 已核准，PayUni 綁定中或審核中。
	ResubmitPayUniReviewing KYCResubmitUnavailableReason = "PAYUNI_REVIEWING"
	// ResubmitPayUniPendingInfo 已核准，PayUni 要求補件。
	ResubmitPayUniPendingInfo KYCResubmitUnavailableReason = "PAYUNI_PENDING_INFO"
	// ResubmitPayUniFixViaSupport 已核准，PayUni 指出的是店家欄位。
	ResubmitPayUniFixViaSupport KYCResubmitUnavailableReason = "PAYUNI_FIX_VIA_SUPPORT"
	// ResubmitPayUniContactSupport 已核准，問題無法由賣家修正。
	ResubmitPayUniContactSupport KYCResubmitUnavailableReason = "PAYUNI_CONTACT_SUPPORT"
	// ResubmitPayUniRetryLater 已核准，PayUni 暫時無法處理。
	ResubmitPayUniRetryLater KYCResubmitUnavailableReason = "PAYUNI_RETRY_LATER"
	// ResubmitPayUniTerminated 已核准，PayUni 端的商店已終止。
	ResubmitPayUniTerminated KYCResubmitUnavailableReason = "PAYUNI_TERMINATED"
	// ResubmitPayUniApproved 已核准，PayUni 也核准。
	ResubmitPayUniApproved KYCResubmitUnavailableReason = "PAYUNI_APPROVED"
)

// KYCResubmitUnavailableReasonValues 回傳全部原因。
func KYCResubmitUnavailableReasonValues() []KYCResubmitUnavailableReason {
	return []KYCResubmitUnavailableReason{
		ResubmitUnderReviewCancelable, ResubmitUnderReview, ResubmitPayUniReviewing, ResubmitPayUniPendingInfo,
		ResubmitPayUniFixViaSupport, ResubmitPayUniContactSupport, ResubmitPayUniRetryLater, ResubmitPayUniTerminated, ResubmitPayUniApproved,
	}
}

// ResubmitUnavailableReasons 回答申請此刻不能儲存草稿或重新送出的原因；能做時（flow.CanResubmit）是空的。payuni 是同一次推導的
// SellerPayUniOf 結果（申請已核准時一定有值）。
func ResubmitUnavailableReasons(status KYCStatus, flow KYCFlow, payuni *SellerPayUni) []KYCResubmitUnavailableReason {
	if flow.CanResubmit {
		return nil
	}
	switch status {
	case KYCStatusSubmitted:
		if flow.IdentityLocked {
			return []KYCResubmitUnavailableReason{ResubmitUnderReview}
		}
		return []KYCResubmitUnavailableReason{ResubmitUnderReviewCancelable}
	case KYCStatusApproved:
		if payuni == nil {
			return nil
		}
		if r, ok := payuniResubmitReasons[payuni.State]; ok {
			return []KYCResubmitUnavailableReason{r}
		}
	case KYCStatusDraft, KYCStatusRejected, KYCStatusCancelled:
	}
	return nil
}

// payuniResubmitReasons 把已核准時不能重送的 PayUni 狀態對到原因；FIXABLE 能重送，不在表裡。
var payuniResubmitReasons = map[PayUniState]KYCResubmitUnavailableReason{
	PayUniStateReviewing:      ResubmitPayUniReviewing,
	PayUniStatePendingInfo:    ResubmitPayUniPendingInfo,
	PayUniStateFixViaSupport:  ResubmitPayUniFixViaSupport,
	PayUniStateContactSupport: ResubmitPayUniContactSupport,
	PayUniStateRetryLater:     ResubmitPayUniRetryLater,
	PayUniStateTerminated:     ResubmitPayUniTerminated,
	PayUniStateApproved:       ResubmitPayUniApproved,
}

// Text 是原因給使用者看的一句話（語系 l）；KYCFlow.resubmit_unavailabilities 的 message 與 SaveKYCDraft、SubmitKYC 被擋下時
// 錯誤的訊息都由它產生。不認得的原因回零值。
func (r KYCResubmitUnavailableReason) Text(l uitext.Locale) uitext.Message {
	switch r {
	case ResubmitUnderReviewCancelable:
		return text.KycResubmitUnavailableUnderReviewCancelable(l)
	case ResubmitUnderReview:
		return text.KycResubmitUnavailableUnderReview(l)
	case ResubmitPayUniReviewing:
		return text.KycResubmitUnavailablePayuniReviewing(l)
	case ResubmitPayUniPendingInfo:
		return text.KycResubmitUnavailablePayuniPendingInfo(l)
	case ResubmitPayUniFixViaSupport:
		return text.KycResubmitUnavailablePayuniFixViaSupport(l)
	case ResubmitPayUniContactSupport:
		return text.KycResubmitUnavailablePayuniContactSupport(l)
	case ResubmitPayUniRetryLater:
		return text.KycResubmitUnavailablePayuniRetryLater(l)
	case ResubmitPayUniTerminated:
		return text.KycResubmitUnavailablePayuniTerminated(l)
	case ResubmitPayUniApproved:
		return text.KycResubmitUnavailablePayuniApproved(l)
	}
	return uitext.Message{}
}

// ErrResubmitUnavailable 是 SaveKYCDraft 與 SubmitKYC 被擋下時的錯誤：FailedPrecondition，訊息是第一個原因的 Text(l)。
// 沒有原因（不該發生：能做或到不了的狀態）回 internal 錯誤。
func ErrResubmitUnavailable(reasons []KYCResubmitUnavailableReason, l uitext.Locale) error {
	if len(reasons) == 0 {
		return fmt.Errorf("seller：申請不能儲存或重新送出，但沒有原因")
	}
	msg := reasons[0].Text(l)
	if msg.Text == "" {
		return fmt.Errorf("seller：不認得的重新送出原因 %q", string(reasons[0]))
	}
	return apperr.FailedPrecondition("%s", msg.Text)
}
