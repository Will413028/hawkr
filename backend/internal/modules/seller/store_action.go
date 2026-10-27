package seller

import (
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// StoreAction 是 admin 對店家（或它的帳號、商家認證申請、PayUni 註冊、食品業者登錄）做的、有前置條件的動作。
type StoreAction = model.StoreAction

// 店家動作。
const (
	StoreActionCreateMeasure           = model.StoreActionCreateMeasure
	StoreActionDelegate                = model.StoreActionDelegate
	StoreActionApproveKYC              = model.StoreActionApproveKYC
	StoreActionRejectKYC               = model.StoreActionRejectKYC
	StoreActionRequeryKYC              = model.StoreActionRequeryKYC
	StoreActionRebindPayUni            = model.StoreActionRebindPayUni
	StoreActionApproveFoodRegistration = model.StoreActionApproveFoodRegistration
	StoreActionRejectFoodRegistration  = model.StoreActionRejectFoodRegistration
)

// StoreActionUnavailableReason 是 admin 此刻不能做某個店家動作的原因（與 API 的 StoreActionUnavailableReason 一致）。
type StoreActionUnavailableReason = model.StoreActionUnavailableReason

// StoreActionUnavailableReasonValues 回傳全部原因。
func StoreActionUnavailableReasonValues() []StoreActionUnavailableReason {
	return model.StoreActionUnavailableReasonValues()
}

// StoreActionValues 回傳全部店家動作。
func StoreActionValues() []StoreAction { return model.StoreActionValues() }

// StoreActionUnavailableText 是原因給員工看的一句話；API 的可用性訊息、方法被拒絕時的錯誤、案件與通報的原因句都由它產生。表外與 idempotent 的組合回零值。
func StoreActionUnavailableText(r StoreActionUnavailableReason, a StoreAction) uitext.Message {
	return model.StoreActionUnavailableText(r, a)
}

// 店家動作不能做的原因。
const (
	StoreReasonKYCNotSubmitted            = model.StoreReasonKYCNotSubmitted
	StoreReasonKYCNotBusiness             = model.StoreReasonKYCNotBusiness
	StoreReasonPayUniMerchantTerminal     = model.StoreReasonPayUniMerchantTerminal
	StoreReasonPayUniBindInProgress       = model.StoreReasonPayUniBindInProgress
	StoreReasonPayUniBound                = model.StoreReasonPayUniBound
	StoreReasonFoodRegistrationNotPending = model.StoreReasonFoodRegistrationNotPending
	StoreReasonKYCNotApproved             = model.StoreReasonKYCNotApproved
	StoreReasonStoreClosed                = model.StoreReasonStoreClosed
	StoreReasonStorePendingDeletion       = model.StoreReasonStorePendingDeletion
	StoreReasonAccountSuspended           = model.StoreReasonAccountSuspended
)
