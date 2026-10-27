package usecase

import (
	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// 買家申請修改發票資料（B06）的分流判定 —— 58 號 §7.3.5 的 R02／R03／R04。
//
// 三條邊的守衛條件**都在送出當下評估**，所以它們不是三個獨立入口，而是同一個提交
// 流程的三個分支。把判定抽成純函式的理由：它是這條鏈唯一「全部輸入都拿得到、
// 且沒有副作用」的一段，也是唯一能用表格窮舉測完的一段。
//
// 🔴 **順序不可換：先確保可開再作廢。** 作廢不可逆（且明令不可用 DB restore 把
// ISSUED 回復為未開立）。四類**可預先判定**的失敗必須擋在作廢之前，否則會落在
// 「原發票已作廢、新發票開不出來」——買家手上一張都沒有。

// CorrectionRoute 是分流結果。
type CorrectionRoute int

const (
	// RouteReject ＝ R02：預檢不過，整個請求 REJECTED，原發票留著不動。
	RouteReject CorrectionRoute = iota
	// RouteUpdateBeforeIssue ＝ R03：還沒送出 AMEGO，直接改快照。
	//
	// ⚠️ 本判定只說「**應該**走 R03」。真正的守衛是 operation 的 HELD CAS rows=1，
	// 那要打 DB 才知道——搶不到代表那筆已經在打 AMEGO，屆時要退回 R04。
	// 這也是為什麼本函式不能宣稱自己是最終答案。
	RouteUpdateBeforeIssue
	// RouteVoidAndReissue ＝ R04：document 已 ISSUED 且預檢通過，作廢重開。
	RouteVoidAndReissue
	// RouteOperationInProgress ＝ 這張憑證正在被處理（ISSUING／VOIDING），
	// **既不是婉拒也不是可執行的邊**：呼叫端回 9351「請稍後再試」。
	//
	// 它與 RouteReject 的差別對買家是決定性的：REJECTED 的每一個理由碼都在說
	// 「這件事做不成了」，而這一格說的是「等幾秒再按一次」——兩者的下一步相反。
	RouteOperationInProgress
)

// CorrectionFacts 是分流需要的全部事實。
//
// 刻意做成「呼叫端先把事實查齊、再交給純函式判」而不是讓函式自己去查：
// 前者測得完（8 個布林組合窮舉），後者只能靠 mock 逐條擺。
type CorrectionFacts struct {
	// DocumentStatus 取自 invoice_documents.status。
	DocumentStatus string

	// OperationOpen ＝ 這張憑證還有一筆未送出的 operation（READY／RETRY_SCHEDULED）。
	// 它決定 R03 這條邊**有沒有對象可以 HELD**；真正搶得到與否要打 DB。
	OperationOpen bool

	// ---- 以下四項是 R04 的預檢（§7.3.5「先確保可開再作廢」）----
	// 每一項為 false 就是一個婉拒理由，且**都必須在作廢之前判**。

	// TrackSufficient ＝ 重開需要的新號碼，字軌餘量夠。
	TrackSufficient bool
	// CarrierValid ＝ 更正後的載具驗證通過。
	CarrierValid bool
	// WithinVoidDeadline ＝ 仍在法定可作廢期限內。
	WithinVoidDeadline bool
	// NoExistingAllowance ＝ 該張發票沒有既存折讓（有折讓則作廢會使折讓失去依據）。
	NoExistingAllowance bool

	// ProviderAvailable ＝ 服務商當下可用。false 時買家稍後可再申請一次
	// （REJECTED 是終態且不佔 uq_invcorr_open_per_document 的名額）。
	ProviderAvailable bool

	// AlreadyReported ＝ 已隨期別申報，依規定不得作廢。
	AlreadyReported bool
}

// CorrectionDecision 是分流結果 ＋（婉拒時）理由碼。
type CorrectionDecision struct {
	Route CorrectionRoute
	// RejectReason 只在 Route == RouteReject 時非空，值域＝InvoiceCorrectionRejectReason。
	RejectReason string
}

// decideCorrectionRoute 判定一筆更正請求該走哪條邊。
//
// 判定順序本身就是規格的一部分，不是實作細節：
//
//  1. **還沒開立 ⇒ 一律 R03**，不做任何 R04 預檢。那四項預檢問的是「作廢重開會不會失敗」，
//     而這條路徑根本不作廢——拿它們擋 R03 會讓「字軌暫時不足」這種與本案無關的狀況
//     擋掉一個只是改快照的請求。
//  2. 已開立 ⇒ 走 R04 的預檢，**任一不過就 REJECT**。
//  3. 預檢的檢查次序決定買家看到哪一個理由。已申報排最前面：它是最終局的
//     （其餘四項都可能隨時間或補配號而改變，只有「已申報」不會回頭）。
func decideCorrectionRoute(f CorrectionFacts) CorrectionDecision {
	issued := f.DocumentStatus == codemeta.InvoiceDocumentStatusIssued.Value

	if !issued {
		// 尚未開立：只要還有 operation 可以攔，就走 R03。
		if f.OperationOpen {
			return CorrectionDecision{Route: RouteUpdateBeforeIssue}
		}
		// 🔴 進行中的實體狀態要先判，而且**不是婉拒**。
		//
		// ISSUING／VOIDING 期間那筆 operation 已經是 SENDING，而 OperationOpen 的述詞
		// 刻意只含 READY／RETRY_SCHEDULED ⇒ 上面那一格接不住它，會一路掉到下面的
		// fallback。這正是 2026-09-02（#3020）dev 實測到的路徑：買家送出第一筆修改
		// 把原票推進 VOIDING，15 秒後再送第二筆，得到的理由是
		// PAST_VOID_DEADLINE——而那張發票 68 秒前才開立。
		//
		// 兩者對買家的意思完全相反：PAST_VOID_DEADLINE ＝「你錯過期限了，再也改不了」
		// （終局），實情是「前一筆正在處理，等幾秒」（暫時）。客服會照那個碼回覆
		// 「已逾期無法處理」，而真相是再等一下就能重試。
		if isDocumentInProgress(f.DocumentStatus) {
			return CorrectionDecision{Route: RouteOperationInProgress}
		}
		// 走到這裡的是**真的終態**（VOIDED／SUPERSEDED／CANCELLED_BEFORE_ISSUE／
		// NOT_APPLICABLE／RESOLVED_EXTERNALLY）：沒有開立、沒有可攔的 operation、
		// 也沒有進行中的作業 ⇒ 沒有東西可以改。
		//
		// ⚠️ PAST_VOID_DEADLINE 在這一格**仍然不精準**（真因不是逾期），但值域裡
		// 沒有「這張憑證已是終態」那一格，而擴值域是前端可見的碼表變更（§8.5）。
		// 保留它並在此標明，比悄悄換一個同樣不對的碼誠實。
		// 原本這段註解寫著「這一格在今天的資料下取不到」——那句話是錯的，
		// 取不到的前提只在「非 ISSUED 必有 open operation」成立時才對，
		// 而 SENDING 中的 operation 不算 open。
		return CorrectionDecision{
			Route:        RouteReject,
			RejectReason: codemeta.InvoiceCorrectionRejectReasonPastVoidDeadline.Value,
		}
	}

	// 已開立：R04 的四項預檢 ＋ 兩項狀態檢查，任一不過就婉拒。
	switch {
	case f.AlreadyReported:
		return CorrectionDecision{Route: RouteReject,
			RejectReason: codemeta.InvoiceCorrectionRejectReasonAlreadyReported.Value}
	case !f.WithinVoidDeadline:
		return CorrectionDecision{Route: RouteReject,
			RejectReason: codemeta.InvoiceCorrectionRejectReasonPastVoidDeadline.Value}
	case !f.NoExistingAllowance:
		return CorrectionDecision{Route: RouteReject,
			RejectReason: codemeta.InvoiceCorrectionRejectReasonAllowanceExists.Value}
	case !f.TrackSufficient:
		return CorrectionDecision{Route: RouteReject,
			RejectReason: codemeta.InvoiceCorrectionRejectReasonTrackInsufficient.Value}
	case !f.CarrierValid:
		return CorrectionDecision{Route: RouteReject,
			RejectReason: codemeta.InvoiceCorrectionRejectReasonCarrierInvalid.Value}
	case !f.ProviderAvailable:
		return CorrectionDecision{Route: RouteReject,
			RejectReason: codemeta.InvoiceCorrectionRejectReasonProviderUnavailable.Value}
	}

	return CorrectionDecision{Route: RouteVoidAndReissue}
}

// isDocumentInProgress 回報這張憑證此刻是否有一次**已送出、尚未落地**的實體作業。
//
// 值域刻意只有兩個：它們是「operation 已離開 READY／RETRY_SCHEDULED（所以
// OperationOpen 為 false）但流程還沒結束」的兩個實體狀態。
// PENDING／ISSUE_FAILED／AWAITING_MANUAL **不在內**——那三個在沒有 open operation
// 時代表流程停住而不是進行中，對它們回「請稍後再試」會讓買家永遠等一個不會來的結果。
func isDocumentInProgress(status string) bool {
	switch status {
	case codemeta.InvoiceDocumentStatusIssuing.Value,
		codemeta.InvoiceDocumentStatusVoiding.Value:
		return true
	default:
		return false
	}
}
