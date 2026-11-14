package model

import (
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// CanEditKYC 回答申請在這個狀態下能不能儲存草稿與送出：草稿與被退回的可以；審核中只能取消，已核准的不能改也不能重送。
// 儲存草稿與送出只看申請的狀態，沒有其他條件。
func CanEditKYC(status KYCStatus) bool {
	return status == KYCStatusDraft || status == KYCStatusRejected
}

// ErrKYCNotEditable 是 SaveKYCDraft 與 SubmitKYC 在 CanEditKYC 為假時回的錯誤：FailedPrecondition，訊息依狀態（語系 l）。
// 能編輯的狀態或不認得的狀態回 internal 錯誤（呼叫方只在 CanEditKYC 為假時呼叫）。
func ErrKYCNotEditable(status KYCStatus, l uitext.Locale) error {
	var msg uitext.Message
	switch status {
	case KYCStatusSubmitted:
		msg = text.KycEditUnavailableUnderReview(l)
	case KYCStatusApproved:
		msg = text.KycEditUnavailableApproved(l)
	case KYCStatusDraft, KYCStatusRejected, KYCStatusCancelled:
		return fmt.Errorf("seller：申請狀態 %q 沒有不能編輯的原因", string(status))
	default:
		return fmt.Errorf("seller：申請狀態 %q 沒有不能編輯的原因", string(status))
	}
	return apperr.FailedPrecondition("%s", msg.Text)
}
