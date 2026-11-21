package planner

import (
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
)

// Act 是這一輪要對買家說的哪一類話。
type Act string

// 這一輪會被選中的 act。
const (
	AckAndAskRequired           Act = "ACK_AND_ASK_REQUIRED"
	AckAndAskOptional           Act = "ACK_AND_ASK_OPTIONAL"
	CorrectionApplied           Act = "CORRECTION_APPLIED"
	InputRejected               Act = "INPUT_REJECTED"
	ExtractionDegraded          Act = "EXTRACTION_DEGRADED"
	RequestBasicsReady          Act = "REQUEST_BASICS_READY"
	PricePreviewAvailable       Act = "PRICE_PREVIEW_AVAILABLE"
	RfqReconfirmRequired        Act = "RFQ_RECONFIRM_REQUIRED"
	PublicationApprovalRequired Act = "PUBLICATION_APPROVAL_REQUIRED"
	ItemNeedsDisambiguation     Act = "ITEM_NEEDS_DISAMBIGUATION"
)

// All 是所有 act，供測試窮舉。
var All = []Act{
	AckAndAskRequired, AckAndAskOptional, CorrectionApplied, InputRejected, ExtractionDegraded,
	RequestBasicsReady, PricePreviewAvailable, RfqReconfirmRequired, PublicationApprovalRequired,
	ItemNeedsDisambiguation,
}

// RfqState 是這個需求的詢價確認快照相對於現行版本的狀態。
type RfqState int

// 詢價確認狀態。
const (
	// RfqNone 是沒有任何確認快照。
	RfqNone RfqState = iota
	// RfqStale 是有快照，但對應的不是現行版本。
	RfqStale
	// RfqConfirmed 是有快照，且對得上現行版本。
	RfqConfirmed
)

// MaxSalientFacts 是敘述最多帶幾個這一輪記下的事實。
const MaxSalientFacts = 2

// Input 是 Plan 的輸入；planner 不做 I/O，只消費呼叫端算好的狀態。
type Input struct {
	Current, Next slots.Set
	Rejected      []slots.Rejection
	Degraded      bool
	Recorded      []slots.Recorded
	// Question 是這一輪算出的下一題，沒有時 nil；品項消歧中時是 BuildItemDisambiguation 的結果。
	Question              *Question
	PricePreviewAvailable bool
	Rfq                   RfqState
}

// Plan 是這一輪的決策。
type Plan struct {
	Act      Act
	Salient  []slots.Recorded
	Question *Question
}

// Decide 選出這一輪的 act。優先序：降級、有欄位被拒、更正已有的欄位、品項消歧、必填未齊、詢價快照過期、快照已確認待核准、
// 還有選填問題、有價格參考、需求齊備。
func Decide(in Input) Plan {
	n := len(in.Recorded)
	if n > MaxSalientFacts {
		n = MaxSalientFacts
	}
	return Plan{Act: selectAct(in), Salient: in.Recorded[:n:n], Question: in.Question}
}

func selectAct(in Input) Act {
	ready := len(in.Next.MissingRequired()) == 0
	switch {
	case in.Degraded:
		return ExtractionDegraded
	case len(in.Rejected) > 0:
		return InputRejected
	case hasCorrection(in):
		return CorrectionApplied
	case in.Question != nil && in.Question.Slot == slots.ItemDisambiguation:
		return ItemNeedsDisambiguation
	case !ready:
		return AckAndAskRequired
	case in.Rfq == RfqStale:
		return RfqReconfirmRequired
	case in.Rfq == RfqConfirmed:
		return PublicationApprovalRequired
	case in.Question != nil:
		return AckAndAskOptional
	case in.PricePreviewAvailable:
		return PricePreviewAvailable
	}
	return RequestBasicsReady
}

// hasCorrection 判斷這一輪記下的欄位裡有沒有任何一個在 Current 就已經有值：是的話是更正，不是第一次填。
func hasCorrection(in Input) bool {
	for _, r := range in.Recorded {
		if wasSet(in.Current, r.Key) {
			return true
		}
	}
	return false
}

// wasSet 不含 ItemResolution：它由伺服器而不是買家填入，第一次填與換掉都不算買家更正。
func wasSet(s slots.Set, key model.SlotKey) bool {
	switch key {
	case slots.Item:
		return s.Item != nil
	case slots.Quantity:
		return s.Quantity != nil
	case slots.Unit:
		return s.Unit != nil
	case slots.Packaging:
		return s.Packaging != nil
	case slots.BudgetPerUnit:
		return s.BudgetPerUnit != nil
	case slots.DeliveryCity:
		return s.DeliveryCity != nil
	case slots.DeliveryDate:
		return s.DeliveryDate != nil
	case slots.ExactMatchRequired:
		return s.ExactMatchRequired != nil
	}
	return false
}
