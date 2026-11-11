package model

import (
	"fmt"
	"slices"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// StoreAction 是 admin 對店家（或它的帳號、商家認證申請）做的、有前置條件的動作；原因清單與句子都依動作決定。
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
)

// StoreActionValues 回傳全部有前置條件的店家動作。
func StoreActionValues() []StoreAction {
	return []StoreAction{
		StoreActionCreateMeasure, StoreActionDelegate, StoreActionApproveKYC, StoreActionRejectKYC, StoreActionRequeryKYC,
	}
}

// StoreActionUnavailableReason 是 admin 此刻不能做某個店家動作的原因；值與 API 的 StoreActionUnavailableReason 一致。
type StoreActionUnavailableReason string

const (
	// StoreReasonKYCNotSubmitted 商家認證申請不在審核中。表示沒有入口。
	StoreReasonKYCNotSubmitted StoreActionUnavailableReason = "KYC_NOT_SUBMITTED"
	// StoreReasonKYCNotBusiness 不是公司型的申請，沒有登記資料可以查詢。表示沒有入口。
	StoreReasonKYCNotBusiness StoreActionUnavailableReason = "KYC_NOT_BUSINESS"
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
		StoreReasonKYCNotSubmitted, StoreReasonKYCNotBusiness, StoreReasonStoreClosed,
		StoreReasonStorePendingDeletion, StoreReasonAccountSuspended,
	}
}

// KYCApplicationState 是商家認證申請裡推導原因要用的事實。
type KYCApplicationState struct {
	Status     KYCStatus
	SellerType KYCSellerType
}

// storeActionFacts 是推導需要的事實；各動作只填它用得到的部分。
type storeActionFacts struct {
	Store Store
	KYC   KYCApplicationState
}

// storeActionRule 是有效組合表的一列：原因、成立的條件、給員工的句子。句子、種類與組合因此只有這一份。
type storeActionRule struct {
	Reason  StoreActionUnavailableReason
	Applies func(storeActionFacts) bool
	Text    func() uitext.Message
}

// 各動作共用的條件。
var (
	whenStoreClosed = func(f storeActionFacts) bool { return f.Store.Status == Closed }
	whenPending     = func(f storeActionFacts) bool { return f.Store.Status == PendingDeletion }
	whenSuspended   = func(f storeActionFacts) bool { return f.Store.Suspended() }
	whenKYCNotSub   = func(f storeActionFacts) bool { return f.KYC.Status != KYCStatusSubmitted }
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
// 缺的條件（已關閉、等待刪除、停權）；其他動作回 nil（它們還要申請或註冊的事實，用下面的函式）。
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
	case StoreActionCreateMeasure, StoreActionDelegate:
		return nil
	}
	return nil
}

// StoreActionUnavailableText 是原因給員工看的一句話（只有 zh-TW），API 的 StoreActionUnavailability.message、方法被拒絕時錯誤的 LocalizedMessage、
// 案件與通報建立措施被擋下的原因都由它產生，句子只在規則表的那一列。每句有主詞、以「。」結尾、不含畫面名稱。表外的組合回零值（Text 是空字串）。
func StoreActionUnavailableText(r StoreActionUnavailableReason, a StoreAction) uitext.Message {
	for _, rule := range storeActionRules[a] {
		if rule.Reason == r {
			return rule.Text()
		}
	}
	return uitext.Message{}
}

// ErrStoreActionUnavailable 是不能做 a 的錯誤（failed_precondition）：訊息是第一個原因的句子，與回應欄位的 message 同一個函式。清單是空、或第一個原因沒有句子時是內部錯誤：不寫通用句，讓漏掉的組合在第一次執行就被發現。
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
