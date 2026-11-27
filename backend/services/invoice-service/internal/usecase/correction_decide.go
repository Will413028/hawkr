package usecase

import (
	"github.com/Will413028/hawkr/go-pkg/codemeta"
)


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

func decideCorrectionRoute(f CorrectionFacts) CorrectionDecision {
	issued := f.DocumentStatus == codemeta.InvoiceDocumentStatusIssued.Value

	if !issued {
		if f.OperationOpen {
			return CorrectionDecision{Route: RouteUpdateBeforeIssue}
		}
		if isDocumentInProgress(f.DocumentStatus) {
			return CorrectionDecision{Route: RouteOperationInProgress}
		}
		return CorrectionDecision{
			Route:        RouteReject,
			RejectReason: codemeta.InvoiceCorrectionRejectReasonPastVoidDeadline.Value,
		}
	}

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

func isDocumentInProgress(status string) bool {
	switch status {
	case codemeta.InvoiceDocumentStatusIssuing.Value,
		codemeta.InvoiceDocumentStatusVoiding.Value:
		return true
	default:
		return false
	}
}
