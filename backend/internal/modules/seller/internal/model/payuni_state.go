package model

import (
	"slices"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// PayUniState 是賣家看到的 PayUni 收款狀態，一個值對應賣家的一種動作：等、看 PayUni 的信、改申請後重送、改店家設定後找客服、
// 找客服、等平台重送、關店重新入駐、無。由 SellerPayUniOf 推導，不存。
type PayUniState string

const (
	// PayUniStateReviewing 綁定中或 PayUni 審核中。
	PayUniStateReviewing PayUniState = "REVIEWING"
	// PayUniStatePendingInfo PayUni 要求補件。
	PayUniStatePendingInfo PayUniState = "PENDING_INFO"
	// PayUniStateFixable 申請的資料有問題，賣家修正申請後自己重新送出。
	PayUniStateFixable PayUniState = "FIXABLE"
	// PayUniStateFixViaSupport 店家設定的資料有問題，賣家修正後由平台重新處理。
	PayUniStateFixViaSupport PayUniState = "FIX_VIA_SUPPORT"
	// PayUniStateContactSupport 沒有賣家能修正的資料。
	PayUniStateContactSupport PayUniState = "CONTACT_SUPPORT"
	// PayUniStateRetryLater PayUni 暫時無法處理，等平台重新送出。
	PayUniStateRetryLater PayUniState = "RETRY_LATER"
	// PayUniStateTerminated PayUni 端的商店已是終止狀態，同一個店家無法再綁定。
	PayUniStateTerminated PayUniState = "TERMINATED"
	// PayUniStateApproved 可以收款。
	PayUniStateApproved PayUniState = "APPROVED"
)

// PayUniStateValues 回傳全部 PayUni 狀態。
func PayUniStateValues() []PayUniState {
	return []PayUniState{
		PayUniStateReviewing, PayUniStatePendingInfo, PayUniStateFixable, PayUniStateFixViaSupport, PayUniStateContactSupport, PayUniStateRetryLater,
		PayUniStateTerminated, PayUniStateApproved,
	}
}

// Explanation 是狀態給賣家看的說明（語系 l），只由狀態決定。APPROVED（畫面不顯示面板）與不認得的值回 false。
func (s PayUniState) Explanation(l uitext.Locale) (uitext.Message, bool) {
	switch s {
	case PayUniStateReviewing:
		return text.PayuniStateReviewing(l), true
	case PayUniStatePendingInfo:
		return text.PayuniStatePendingInfo(l), true
	case PayUniStateFixable:
		return text.PayuniStateFixable(l), true
	case PayUniStateFixViaSupport:
		return text.PayuniStateFixViaSupport(l), true
	case PayUniStateContactSupport:
		return text.PayuniStateContactSupport(l), true
	case PayUniStateRetryLater:
		return text.PayuniStateRetryLater(l), true
	case PayUniStateTerminated:
		return text.PayuniStateTerminated(l), true
	case PayUniStateApproved:
	}
	return uitext.Message{}, false
}

// SellerPayUni 是賣家看到的 PayUni 狀態與要處理的事。Reason 是 PayUni 的審核意見原文（去掉前後空白，沒有時空字串）；
// FixFields 是賣家此刻能在申請上修正的欄位（只在 FIXABLE，已依賣家型態與身分鎖過濾），FixStoreFields 是能在店家設定修正的欄位
// （只在 FIXABLE、FIX_VIA_SUPPORT）。
type SellerPayUni struct {
	State          PayUniState
	Reason         string
	FixFields      []string
	FixStoreFields []string
}

// SellerPayUniOf 由申請的狀態與 PayUni 註冊（沒有時為 nil）推導賣家看到的 PayUni 狀態。sameApplication 是註冊屬於目前這份申請。
// 有值的條件：申請已核准（沒有註冊時是綁定中）；或申請是草稿、註冊屬於這份申請、而且推導結果是 FIXABLE（賣家正在修正 PayUni
// 拒絕或綁定失敗的那份申請，問題要留著）。其他沒有，畫面看流程狀態。階段一律由 PayUniPhaseOf 決定。
func SellerPayUniOf(app KYCStatus, sameApplication bool, reg *PayUniRegistration) (SellerPayUni, bool) {
	switch app {
	case KYCStatusApproved:
	case KYCStatusDraft:
		if !sameApplication || reg == nil {
			return SellerPayUni{}, false
		}
	case KYCStatusSubmitted, KYCStatusRejected, KYCStatusCancelled:
		return SellerPayUni{}, false
	}
	out := sellerPayUniOf(reg)
	if app == KYCStatusDraft && out.State != PayUniStateFixable {
		return SellerPayUni{}, false
	}
	return out, true
}

// sellerPayUniOf 是推導本身（階段由 PayUniPhaseOf 決定）；DeriveKYCFlow 的 CanResubmit 與 SellerPayUniOf 都呼叫它，所以「能不能重送」
// 和賣家看到的 PayUni 狀態出自同一次推導、同一份註冊。reg 要帶賣家型態與店家缺的欄位（欄位清單依它們過濾），見 payuni.SellerViewOf。
func sellerPayUniOf(reg *PayUniRegistration) SellerPayUni {
	if reg == nil {
		return SellerPayUni{State: PayUniStateReviewing}
	}
	p := ProblemOf(*reg)
	if reg.NeedsReonboarding {
		return withReason(SellerPayUni{State: PayUniStateTerminated}, p)
	}
	if reg.PaymentEnabled {
		return SellerPayUni{State: PayUniStateApproved}
	}
	switch PayUniPhaseOf(*reg) {
	case FlowBinding, FlowForwarded, FlowUnderReview:
		return SellerPayUni{State: PayUniStateReviewing}
	case FlowPendingInfo:
		return withReason(SellerPayUni{State: PayUniStatePendingInfo}, p)
	case FlowPayUniRejected:
		// PayUni 拒絕（商家層級的終止碼 MER_2、MER_8、MER_92、MER_99 在上面已是 TERMINATED，剩下會員層級的 MEMBER_8、MEMBER_92、MEMBER_99）：
		// 補救方式是聯絡客服（MEMBER_99）時賣家什麼都改不到，否則（整份表單的碼）修正申請後重送。
		if p == nil || p.Remedy == RemedyContactSupport {
			return withReason(SellerPayUni{State: PayUniStateContactSupport}, p)
		}
		return withReason(SellerPayUni{State: PayUniStateFixable}, p)
	case FlowBindFailed:
		if p == nil {
			return SellerPayUni{State: PayUniStateContactSupport}
		}
		switch p.Remedy {
		case RemedyRetryLater:
			return SellerPayUni{State: PayUniStateRetryLater}
		case RemedySellerFix:
			if s := sellerFix(reg, p); s.State != PayUniStateContactSupport {
				return s
			}
		case RemedyContactSupport:
		}
		return SellerPayUni{State: PayUniStateContactSupport}
	case FlowApproved:
		return SellerPayUni{State: PayUniStateApproved}
	case FlowNotStarted, FlowDraft, FlowSubmitted, FlowRejected:
	}
	return SellerPayUni{State: PayUniStateContactSupport}
}

// sellerFix 是賣家能自己動手的問題：整份表單的碼是 FIXABLE（不列欄位）；有申請欄位（依身分鎖過濾後仍有）是 FIXABLE；只有店家欄位
// 是 FIX_VIA_SUPPORT；賣家什麼都改不到時是 CONTACT_SUPPORT。
func sellerFix(reg *PayUniRegistration, p *PayUniProblem) SellerPayUni {
	if p.WholeForm {
		return SellerPayUni{State: PayUniStateFixable}
	}
	fields := slices.Clone(p.Fields)
	if IdentityLockedBy(*reg) {
		locked := LockedIdentityFields()
		fields = slices.DeleteFunc(fields, func(f string) bool { return slices.Contains(locked, f) })
	}
	switch {
	case len(fields) > 0:
		return SellerPayUni{State: PayUniStateFixable, FixFields: fields, FixStoreFields: slices.Clone(p.StoreFields)}
	case len(p.StoreFields) > 0:
		return SellerPayUni{State: PayUniStateFixViaSupport, FixStoreFields: slices.Clone(p.StoreFields)}
	}
	return SellerPayUni{State: PayUniStateContactSupport}
}

// withReason 帶上 PayUni 的審核意見（p 是這份註冊的問題，沒有時不帶）。
func withReason(s SellerPayUni, p *PayUniProblem) SellerPayUni {
	if p != nil {
		s.Reason = Deref(p.Reason)
	}
	return s
}

// lockedIdentityFields 是 bind_merchant 送給 PayUni 的身分欄位（加上賣家型態，賣家型態不是欄位）：Name／UBN（姓名或公司名稱、
// 身分證號或統編）、MCName／MCID（負責人）、MerEngName、MerURL（官網）、MerDesc 的後備（商品類型）。
var lockedIdentityFields = []string{
	"real_name", "english_name", "product_type", "website_url",
	"individual.id_card_no", "business.company_name", "business.tax_id", "business.rep_id_card_no",
}

// LockedIdentityFields 回傳註冊已有 MerID 之後鎖住的申請欄位（KYCFieldNames 的路徑）。
func LockedIdentityFields() []string { return slices.Clone(lockedIdentityFields) }
