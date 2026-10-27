package model

import (
	"fmt"
	"slices"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// StoreAction 是 admin 對店家（或它的帳號、商家認證申請、PayUni 註冊、食品業者登錄）做的、有前置條件的動作；原因清單與句子都依動作決定。
type StoreAction string

const (
	// StoreActionCreateMeasure 建立警告、停權或風險措施。
	StoreActionCreateMeasure StoreAction = "CREATE_MEASURE"
	// StoreActionDelegate 代行營運（只含店家狀態那一半：授權、管理員資格與進行中的代行由 delegation 判斷）。
	StoreActionDelegate StoreAction = "DELEGATE"
	// StoreActionApproveKYC 核准商家認證申請。
	StoreActionApproveKYC StoreAction = "APPROVE_KYC"
	// StoreActionRejectKYC 退回商家認證申請。
	StoreActionRejectKYC StoreAction = "REJECT_KYC"
	// StoreActionRequeryKYC 重新查詢商家認證申請的登記資料。
	StoreActionRequeryKYC StoreAction = "REQUERY_KYC"
	// StoreActionRebindPayUni 對綁定失敗的 PayUni 註冊重新綁定。
	StoreActionRebindPayUni StoreAction = "REBIND_PAYUNI"
	// StoreActionApproveFoodRegistration 核准食品業者登錄。
	StoreActionApproveFoodRegistration StoreAction = "APPROVE_FOOD_REGISTRATION"
	// StoreActionRejectFoodRegistration 退回食品業者登錄。
	StoreActionRejectFoodRegistration StoreAction = "REJECT_FOOD_REGISTRATION"
)

// StoreActionValues 回傳全部有前置條件的店家動作。
func StoreActionValues() []StoreAction {
	return []StoreAction{
		StoreActionCreateMeasure, StoreActionDelegate, StoreActionApproveKYC, StoreActionRejectKYC, StoreActionRequeryKYC,
		StoreActionRebindPayUni, StoreActionApproveFoodRegistration, StoreActionRejectFoodRegistration,
	}
}

// StoreActionUnavailableReason 是 admin 此刻不能做某個店家動作的原因；值與 API 的 StoreActionUnavailableReason 一致。
type StoreActionUnavailableReason string

const (
	// StoreReasonKYCNotSubmitted 商家認證申請不在審核中。表示沒有入口。
	StoreReasonKYCNotSubmitted StoreActionUnavailableReason = "KYC_NOT_SUBMITTED"
	// StoreReasonKYCNotBusiness 不是公司型的申請，沒有登記資料可以查詢。表示沒有入口。
	StoreReasonKYCNotBusiness StoreActionUnavailableReason = "KYC_NOT_BUSINESS"
	// StoreReasonPayUniMerchantTerminal PayUni 端的商店已是終止狀態。
	StoreReasonPayUniMerchantTerminal StoreActionUnavailableReason = "PAYUNI_MERCHANT_TERMINAL"
	// StoreReasonPayUniBindInProgress PayUni 註冊正在綁定（PENDING）。表示沒有入口；不是失敗原因（idempotent）。
	StoreReasonPayUniBindInProgress StoreActionUnavailableReason = "PAYUNI_BIND_IN_PROGRESS"
	// StoreReasonPayUniBound PayUni 註冊已綁定。表示沒有入口。
	StoreReasonPayUniBound StoreActionUnavailableReason = "PAYUNI_BOUND"
	// StoreReasonFoodRegistrationNotPending 食品業者登錄不在待審。表示沒有入口。
	StoreReasonFoodRegistrationNotPending StoreActionUnavailableReason = "FOOD_REGISTRATION_NOT_PENDING"
	// StoreReasonKYCNotApproved 店家目前的商家認證還沒核准（重新綁定時是註冊所根據的申請不是已核准）。
	StoreReasonKYCNotApproved StoreActionUnavailableReason = "KYC_NOT_APPROVED"
	// StoreReasonStoreClosed 店家已關閉。
	StoreReasonStoreClosed StoreActionUnavailableReason = "STORE_CLOSED"
	// StoreReasonStorePendingDeletion 店家等待刪除。
	StoreReasonStorePendingDeletion StoreActionUnavailableReason = "STORE_PENDING_DELETION"
	// StoreReasonAccountSuspended 帳號有生效中的停權措施。
	StoreReasonAccountSuspended StoreActionUnavailableReason = "ACCOUNT_SUSPENDED"
)

// StoreActionUnavailableReasonValues 回傳全部原因（與 API 的 StoreActionUnavailableReason 一致）。
func StoreActionUnavailableReasonValues() []StoreActionUnavailableReason {
	return []StoreActionUnavailableReason{
		StoreReasonKYCNotSubmitted, StoreReasonKYCNotBusiness, StoreReasonPayUniMerchantTerminal, StoreReasonPayUniBindInProgress,
		StoreReasonPayUniBound, StoreReasonFoodRegistrationNotPending, StoreReasonKYCNotApproved, StoreReasonStoreClosed,
		StoreReasonStorePendingDeletion, StoreReasonAccountSuspended,
	}
}

// KYCApplicationState 是商家認證申請裡推導原因要用的事實。
type KYCApplicationState struct {
	Status     KYCStatus
	SellerType KYCSellerType
}

// storeActionFacts 是推導需要的事實；各動作只填它用得到的部分。PayUniApp 是 PayUni 註冊所根據的申請的狀態（REBIND_PAYUNI 的 KYC_NOT_APPROVED）。
type storeActionFacts struct {
	Store     Store
	KYC       KYCApplicationState
	PayUni    PayUniRegistration
	PayUniApp KYCStatus
	FoodReg   FoodRegistrationReview
}

// storeActionRule 是有效組合表的一列：原因、成立的條件、給員工的句子。Text 是 nil 表示 idempotent：方法不回錯誤、回目前的狀態，沒有句子
// （只有 REBIND_PAYUNI 的 PAYUNI_BIND_IN_PROGRESS）。句子、種類與組合因此只有這一份。
type storeActionRule struct {
	Reason  StoreActionUnavailableReason
	Applies func(storeActionFacts) bool
	Text    func() uitext.Message
}

// 各動作共用的條件。FoodRegistrationUnavailableReasons 沒有載入停權（只有 StoreStatus），所以食品業者登錄的動作不能用 whenSuspended。
var (
	whenStoreClosed = func(f storeActionFacts) bool { return f.Store.Status == Closed }
	whenPending     = func(f storeActionFacts) bool { return f.Store.Status == PendingDeletion }
	whenSuspended   = func(f storeActionFacts) bool { return f.Store.Suspended() }
	whenKYCNotSub   = func(f storeActionFacts) bool { return f.KYC.Status != KYCStatusSubmitted }
	whenFoodNotPend = func(f storeActionFacts) bool { return f.FoodReg.State != FoodRegPending }
)

// missesDelegating 回報店家缺 CapStorefrontWrite 的哪個條件（代行營運看的就是店家能不能寫店面）。
func missesDelegating(r Requirement) func(storeActionFacts) bool {
	return func(f storeActionFacts) bool { return slices.Contains(missingFor(CapStorefrontWrite, f.Store), r) }
}

// storeActionRules 是有效組合表，也是規則與句子本身：每個動作是一串依先後排列的 (原因, 條件, 句子)，推導就是依序跑條件、成立的原因依表的順序收集。
// 每個動作有哪些原因、什麼順序、說什麼只有這一份；四支推導函式只組 facts，不另寫條件、順序與句子。
var storeActionRules = map[StoreAction][]storeActionRule{
	StoreActionCreateMeasure: {
		{StoreReasonStoreClosed, func(f storeActionFacts) bool {
			return !(TerminationTarget{Closed: f.Store.Status == Closed}).AllowsMeasure()
		}, text.StoreActionUnavailableStoreClosedCreateMeasure},
	},
	StoreActionDelegate: {
		{StoreReasonStoreClosed, missesDelegating(RequirementActive), text.StoreActionUnavailableStoreClosedDelegate},
		{StoreReasonStorePendingDeletion, missesDelegating(RequirementNotPendingDeletion), text.StoreActionUnavailableStorePendingDeletionDelegate},
		{StoreReasonAccountSuspended, missesDelegating(RequirementNotSuspended), text.StoreActionUnavailableAccountSuspendedDelegate},
	},
	StoreActionApproveKYC: {
		{StoreReasonKYCNotSubmitted, whenKYCNotSub, text.StoreActionUnavailableKycNotSubmittedReviewKyc},
		{StoreReasonStoreClosed, whenStoreClosed, text.StoreActionUnavailableStoreClosedReviewKyc},
		{StoreReasonStorePendingDeletion, whenPending, text.StoreActionUnavailableStorePendingDeletionReviewKyc},
		{StoreReasonAccountSuspended, whenSuspended, text.StoreActionUnavailableAccountSuspendedApproveKyc},
	},
	StoreActionRejectKYC: {
		{StoreReasonKYCNotSubmitted, whenKYCNotSub, text.StoreActionUnavailableKycNotSubmittedReviewKyc},
		{StoreReasonStoreClosed, whenStoreClosed, text.StoreActionUnavailableStoreClosedReviewKyc},
		{StoreReasonStorePendingDeletion, whenPending, text.StoreActionUnavailableStorePendingDeletionReviewKyc},
	},
	StoreActionRequeryKYC: {
		{StoreReasonKYCNotSubmitted, whenKYCNotSub, text.StoreActionUnavailableKycNotSubmittedRequeryKyc},
		{StoreReasonKYCNotBusiness, func(f storeActionFacts) bool { return f.KYC.SellerType != KYCCompany }, text.StoreActionUnavailableKycNotBusinessRequeryKyc},
		{StoreReasonStoreClosed, whenStoreClosed, text.StoreActionUnavailableStoreClosedReviewKyc},
		{StoreReasonStorePendingDeletion, whenPending, text.StoreActionUnavailableStorePendingDeletionReviewKyc},
	},
	// KYC_NOT_APPROVED 排在最後：PENDING 而申請不是已核准到不了；BOUND 而申請是草稿到得了（PayUni 拒絕、賣家修正中），那時第一個原因應該是
	// 結構性的 PAYUNI_BOUND，卡片不顯示句子。PAYUNI_BIND_IN_PROGRESS 沒有句子（nil）：idempotent。
	StoreActionRebindPayUni: {
		{StoreReasonPayUniMerchantTerminal, func(f storeActionFacts) bool { return f.PayUni.NeedsReonboarding }, text.StoreActionUnavailablePayuniMerchantTerminalRebindPayuni},
		{StoreReasonStoreClosed, whenStoreClosed, text.StoreActionUnavailableStoreClosedRebindPayuni},
		{StoreReasonStorePendingDeletion, whenPending, text.StoreActionUnavailableStorePendingDeletionRebindPayuni},
		{StoreReasonPayUniBindInProgress, func(f storeActionFacts) bool { return f.PayUni.BindStatus == BindPending }, nil},
		{StoreReasonPayUniBound, func(f storeActionFacts) bool { return f.PayUni.BindStatus == BindBound }, text.StoreActionUnavailablePayuniBoundRebindPayuni},
		{StoreReasonKYCNotApproved, func(f storeActionFacts) bool { return f.PayUniApp != KYCStatusApproved }, text.StoreActionUnavailableKycNotApprovedRebindPayuni},
	},
	StoreActionApproveFoodRegistration: {
		{StoreReasonFoodRegistrationNotPending, whenFoodNotPend, text.StoreActionUnavailableFoodRegistrationNotPendingDecideFoodRegistration},
		{StoreReasonStoreClosed, whenStoreClosed, text.StoreActionUnavailableStoreClosedDecideFoodRegistration},
		{StoreReasonStorePendingDeletion, whenPending, text.StoreActionUnavailableStorePendingDeletionDecideFoodRegistration},
		{StoreReasonKYCNotApproved, func(f storeActionFacts) bool {
			return f.FoodReg.KYCStatus == nil || *f.FoodReg.KYCStatus != KYCStatusApproved
		}, text.StoreActionUnavailableKycNotApprovedApproveFoodRegistration},
	},
	StoreActionRejectFoodRegistration: {
		{StoreReasonFoodRegistrationNotPending, whenFoodNotPend, text.StoreActionUnavailableFoodRegistrationNotPendingDecideFoodRegistration},
		{StoreReasonStoreClosed, whenStoreClosed, text.StoreActionUnavailableStoreClosedDecideFoodRegistration},
		{StoreReasonStorePendingDeletion, whenPending, text.StoreActionUnavailableStorePendingDeletionDecideFoodRegistration},
	},
}

// unavailableReasons 依表的順序跑 a 的條件，回傳成立的原因（空表示可以做）；未知的動作回 nil。
func unavailableReasons(a StoreAction, f storeActionFacts) []StoreActionUnavailableReason {
	var out []StoreActionUnavailableReason
	for _, rule := range storeActionRules[a] {
		if rule.Applies(f) {
			out = append(out, rule.Reason)
		}
	}
	return out
}

// ActionUnavailableReasons 回傳此刻不能對店家做 a 的原因（依優先序，空表示可以）：CREATE_MEASURE 與 DELEGATE 只看店家。DELEGATE 對應 CapStorefrontWrite
// 缺的條件（已關閉、等待刪除、停權）；其他動作回 nil（它們還要申請、註冊或登錄的事實，用下面的函式）。
func (s Store) ActionUnavailableReasons(a StoreAction) []StoreActionUnavailableReason {
	if a != StoreActionCreateMeasure && a != StoreActionDelegate {
		return nil
	}
	return unavailableReasons(a, storeActionFacts{Store: s})
}

// KYCActionUnavailableReasons 回傳此刻不能對店家的商家認證申請做 a（APPROVE_KYC、REJECT_KYC、REQUERY_KYC）的原因；s 要帶停權（stint.Load）。
func KYCActionUnavailableReasons(s Store, app KYCApplicationState, a StoreAction) []StoreActionUnavailableReason {
	switch a {
	case StoreActionApproveKYC, StoreActionRejectKYC, StoreActionRequeryKYC:
		return unavailableReasons(a, storeActionFacts{Store: s, KYC: app})
	}
	return nil
}

// RebindUnavailableReasons 回傳此刻不能重新綁定 PayUni 的原因，app 是註冊所根據的申請的狀態。第一個原因若是 idempotent 的（PAYUNI_BIND_IN_PROGRESS），
// RebindPayUni 回目前的註冊；其他原因回 failed_precondition；空表示會開始一次新的綁定。
func RebindUnavailableReasons(s Store, reg PayUniRegistration, app KYCStatus) []StoreActionUnavailableReason {
	return unavailableReasons(StoreActionRebindPayUni, storeActionFacts{Store: s, PayUni: reg, PayUniApp: app})
}

// FoodRegistrationUnavailableReasons 回傳此刻不能裁定食品業者登錄 r 的原因（a 是核准或退回）；店家的條件只看 r.StoreStatus。
func FoodRegistrationUnavailableReasons(r FoodRegistrationReview, a StoreAction) []StoreActionUnavailableReason {
	switch a {
	case StoreActionApproveFoodRegistration, StoreActionRejectFoodRegistration:
		return unavailableReasons(a, storeActionFacts{Store: Store{Status: r.StoreStatus}, FoodReg: r})
	}
	return nil
}

// StoreActionIdempotent 回報 (a, reason) 在表上是不是 idempotent（沒有句子）：方法不回錯誤、回目前的狀態。表外的組合不是。
func StoreActionIdempotent(a StoreAction, reason StoreActionUnavailableReason) bool {
	for _, rule := range storeActionRules[a] {
		if rule.Reason == reason {
			return rule.Text == nil
		}
	}
	return false
}

// StoreActionUnavailableText 是原因給員工看的一句話（只有 zh-TW），API 的 StoreActionUnavailability.message、方法被拒絕時錯誤的 LocalizedMessage、
// 案件與通報建立措施被擋下的原因都由它產生，句子只在規則表的那一列。每句有主詞、以「。」結尾、不含畫面名稱。表外與 idempotent 的組合回零值（Text 是空字串）。
func StoreActionUnavailableText(r StoreActionUnavailableReason, a StoreAction) uitext.Message {
	for _, rule := range storeActionRules[a] {
		if rule.Reason == r && rule.Text != nil {
			return rule.Text()
		}
	}
	return uitext.Message{}
}

// ErrStoreActionUnavailable 是不能做 a 的錯誤（failed_precondition）：訊息是第一個原因的句子，與回應欄位的 message 同一個函式。清單是空、或第一個原因沒有句子
// （idempotent 的原因不是失敗，呼叫方要先處理它）時是內部錯誤：不寫通用句，讓漏掉的組合在第一次執行就被發現。
func ErrStoreActionUnavailable(reasons []StoreActionUnavailableReason, a StoreAction) error {
	if len(reasons) == 0 {
		return fmt.Errorf("seller: %s 沒有不能做的原因，不該回 failed_precondition", a)
	}
	msg := StoreActionUnavailableText(reasons[0], a).Text
	if msg == "" {
		return fmt.Errorf("seller: %s 的原因 %s 沒有句子", a, reasons[0])
	}
	return apperr.FailedPrecondition("%s", msg)
}

// AdminPayUniRegistration 是後台看到的 PayUni 註冊：註冊本身，加上它所在的 PayUni 階段（PayUniPhaseOf）與此刻不能重新綁定的原因
// （RebindUnavailableReasons，空表示可以）。兩者與 RebindPayUni 的檢查由同一次讀取推導。
type AdminPayUniRegistration struct {
	PayUniRegistration
	FlowStatus        KYCFlowStatus
	RebindUnavailable []StoreActionUnavailableReason
}
