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
	RequestBasicsReady          Act = "REQUEST_BASICS_READY"
	PricePreviewAvailable       Act = "PRICE_PREVIEW_AVAILABLE"
	RfqReconfirmRequired        Act = "RFQ_RECONFIRM_REQUIRED"
	PublicationApprovalRequired Act = "PUBLICATION_APPROVAL_REQUIRED"
	ItemNeedsDisambiguation     Act = "ITEM_NEEDS_DISAMBIGUATION"
)

// All 是所有 act，供測試窮舉。
var All = []Act{
	AckAndAskRequired, AckAndAskOptional, CorrectionApplied, InputRejected,
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
	Recorded      []slots.Recorded
	// Question 是這一輪算出的下一題，沒有時 nil；品項消歧中時是 BuildItemDisambiguation 的結果。
	Question              *Question
	PricePreviewAvailable bool
	Rfq                   RfqState
	// PrevStreak 與 Skipped 是上一輪持久化的連續選填計數與買家跳過的欄位；CapExhausted 是 PickNext 這一輪回的第二個值。
	PrevStreak   int
	Skipped      []model.SlotKey
	CapExhausted bool
}

// Plan 是這一輪的決策。
type Plan struct {
	Act     Act
	Salient []slots.Recorded
	// Adjusted 是值有保留但被調整過的欄位（例如過長被截斷）；它們不會讓這一輪變成「輸入被拒」。
	Adjusted []slots.Rejection
	Question *Question
	// OptionalStreak 與 Skipped 是這一輪要持久化的選填進度（turns.optional_streak、turns.skipped_slots），下一輪帶回 Input。
	OptionalStreak int
	Skipped        []model.SlotKey
}

// Decide 選出這一輪的 act。優先序：有欄位被拒、更正已有的欄位、品項消歧、必填未齊、詢價快照過期、快照已確認待核准、
// 還有選填問題、有價格參考、需求齊備。被截斷但值有保留的欄位只是調整提示，不算被拒。
// 品項消歧中（執行器已中斷）時，被拒與更正的回覆由 reply 在警告或更正文字之後接上消歧問句。
// 更正的欄位排在 Salient 最前面，而且一定全數帶入（其餘事實補到 MaxSalientFacts 為止），敘述才說得出改了什麼。
func Decide(in Input) Plan {
	rejected, adjusted := splitRejections(in.Rejected)
	corrected, others := splitCorrected(in)
	salient := append([]slots.Recorded{}, corrected...)
	for _, r := range others {
		if len(salient) >= MaxSalientFacts {
			break
		}
		salient = append(salient, r)
	}
	ready := len(in.Next.MissingRequired()) == 0
	return Plan{
		Act: selectAct(in, len(rejected) > 0, len(corrected) > 0), Salient: salient, Adjusted: adjusted, Question: in.Question,
		OptionalStreak: NextOptionalStreak(ready, in.Question, in.CapExhausted, in.PrevStreak),
		Skipped:        append([]model.SlotKey{}, in.Skipped...),
	}
}

func selectAct(in Input, rejected, corrected bool) Act {
	ready := len(in.Next.MissingRequired()) == 0
	switch {
	case rejected:
		return InputRejected
	case corrected:
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

// splitRejections 把值有保留的截斷分出來：它不丟棄任何內容，不該讓整輪變成「輸入被拒」。
func splitRejections(in []slots.Rejection) (rejected, adjusted []slots.Rejection) {
	for _, r := range in {
		if r.Reason == slots.ReasonTruncated {
			adjusted = append(adjusted, r)
		} else {
			rejected = append(rejected, r)
		}
	}
	return rejected, adjusted
}

// splitCorrected 把這一輪記下的欄位分成「Current 已有值（更正）」與「第一次填」，各自維持 recorded 的順序。
func splitCorrected(in Input) (corrected, others []slots.Recorded) {
	for _, r := range in.Recorded {
		if wasSet(in.Current, r.Key) {
			corrected = append(corrected, r)
		} else {
			others = append(others, r)
		}
	}
	return corrected, others
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
