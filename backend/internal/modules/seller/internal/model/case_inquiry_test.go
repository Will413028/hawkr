package model

import (
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

var inquiryEpoch = time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)

func openInquiry() CaseInquiry {
	return CaseInquiry{
		ID: uuid.NewV7(), CaseID: uuid.NewV7(), StoreID: 1, Kind: InquiryKindRecords, Message: "請提供批次", CreatedAt: inquiryEpoch, CreatedByAdminID: 7,
		DueAt: inquiryEpoch.Add(InquiryDefaultDue),
	}
}

func repliedInquiry(received time.Time) CaseInquiry {
	i := openInquiry()
	i.Reply = &InquiryReply{Body: "已附上", Channel: InquiryChannelEmail, ReceivedAt: received, RecordedAt: received.Add(time.Hour), RecordedByAdminID: 8}
	return i
}

func withdrawnInquiry(at time.Time, cause InquiryWithdrawCause) CaseInquiry {
	i := openInquiry()
	i.Withdrawn = &InquiryWithdrawal{At: at, AdminID: 9, Cause: cause}
	return i
}

// 詢問的狀態只由事實推導：沒有回覆也沒有撤回是等賣家回覆（Open）。
func TestInquiryStateIsDerivedFromTheFacts(t *testing.T) {
	for name, tt := range map[string]struct {
		inquiry CaseInquiry
		want    InquiryState
	}{
		"沒有回覆也沒有撤回": {openInquiry(), InquiryStateOpen},
		"已回覆":       {repliedInquiry(inquiryEpoch.Add(time.Hour)), InquiryStateReplied},
		"管理員撤回":     {withdrawnInquiry(inquiryEpoch.Add(time.Hour), InquiryWithdrawAdmin), InquiryStateWithdrawn},
		"案件結案時撤回":   {withdrawnInquiry(inquiryEpoch.Add(time.Hour), InquiryWithdrawCaseClosed), InquiryStateWithdrawn},
	} {
		if got := tt.inquiry.State(); got != tt.want {
			t.Errorf("%s：State=%s，預期 %s", name, got, tt.want)
		}
		if got := tt.inquiry.Open(); got != (tt.want == InquiryStateOpen) {
			t.Errorf("%s：Open=%v", name, got)
		}
	}
	if got := InquiryStateValues(); !slices.Equal(got, []InquiryState{InquiryStateOpen, InquiryStateReplied, InquiryStateWithdrawn}) {
		t.Errorf("InquiryStateValues=%v", got)
	}
}

// 進行中的案件，「等賣家」與「待決定」（進行中而且沒有詢問，SQL 的 seller.review_case_decision）剛好一個成立；已結案兩者都不成立。
func TestAwaitingSellerAndAwaitingDecisionAreExclusive(t *testing.T) {
	id := uuid.NewV7()
	open := openCase(policy.FoodSafety)
	waiting := openCase(policy.FoodSafety)
	waiting.OpenInquiryID = &id
	closed := closedCase(policy.FoodSafety, CaseClosedDecided)
	for name, tt := range map[string]struct {
		c                Case
		seller, decision bool
	}{
		"進行中、沒有詢問": {open, false, true},
		"進行中、有詢問":  {waiting, true, false},
		"已結案":      {closed, false, false},
	} {
		if tt.c.AwaitingSeller() != tt.seller || (tt.c.Open() && tt.c.OpenInquiryID == nil) != tt.decision {
			t.Errorf("%s：AwaitingSeller=%v AwaitingDecision=%v", name, tt.c.AwaitingSeller(), tt.c.Open() && tt.c.OpenInquiryID == nil)
		}
	}
}

// 案件發詢問的原因依序是：已結案、店家個資已清除、已有還沒結束的詢問；只是關店（個資還在）不擋。
func TestCaseCreateInquiryAvailability(t *testing.T) {
	id := uuid.NewV7()
	waiting := openCase(policy.FoodSafety)
	waiting.OpenInquiryID = &id
	closed := closedCase(policy.FoodSafety, CaseClosedDecided)
	closedWaiting := closed
	closedWaiting.OpenInquiryID = &id
	for name, tt := range map[string]struct {
		facts CaseFacts
		want  []CaseBlock
	}{
		"進行中":       {CaseFacts{Case: openCase(policy.FoodSafety)}, nil},
		"只是關店不擋":    {CaseFacts{Case: openCase(policy.FoodSafety), StoreClosed: true}, nil},
		"店家個資已清除":   {CaseFacts{Case: openCase(policy.FoodSafety), StoreClosed: true, StorePurged: true}, []CaseBlock{CaseBlockStorePurged}},
		"已有還沒結束的詢問": {CaseFacts{Case: waiting}, []CaseBlock{CaseBlockInquiryOpen}},
		"已結案":       {CaseFacts{Case: closed}, []CaseBlock{CaseBlockClosed}},
		"原因依序：已結案、已清除、詢問還開著": {CaseFacts{Case: closedWaiting, StorePurged: true}, []CaseBlock{CaseBlockClosed, CaseBlockStorePurged, CaseBlockInquiryOpen}},
	} {
		if got := tt.facts.Availability().CreateInquiry; !slices.Equal(got, tt.want) {
			t.Errorf("%s：CreateInquiry=%v，預期 %v", name, got, tt.want)
		}
	}
}

// 詢問的可用性：已回覆的不能再記回覆也不能撤回；已撤回的不能記回覆也不能再撤回；原因就是現在的狀態。案件是否進行中不看（結案時詢問已被撤回）。
func TestInquiryAvailability(t *testing.T) {
	for name, tt := range map[string]struct {
		inquiry               CaseInquiry
		recordReply, withdraw []InquiryBlock
	}{
		"等賣家回覆": {openInquiry(), nil, nil},
		"已回覆":   {repliedInquiry(inquiryEpoch.Add(time.Hour)), []InquiryBlock{InquiryBlockReplied}, []InquiryBlock{InquiryBlockReplied}},
		"已撤回":   {withdrawnInquiry(inquiryEpoch.Add(time.Hour), InquiryWithdrawCaseClosed), []InquiryBlock{InquiryBlockWithdrawn}, []InquiryBlock{InquiryBlockWithdrawn}},
	} {
		a := tt.inquiry.Availability()
		if !slices.Equal(a.RecordReply, tt.recordReply) || !slices.Equal(a.Withdraw, tt.withdraw) {
			t.Errorf("%s：RecordReply=%v Withdraw=%v", name, a.RecordReply, a.Withdraw)
		}
	}
}

// 詢問的每個原因都有 zh-TW 的句子，而且彼此不同；不認得的原因沒有句子。
func TestEveryInquiryBlockHasADistinctSentence(t *testing.T) {
	seen := map[string]InquiryBlock{}
	for _, b := range InquiryBlockValues() {
		msg := b.Text().Text
		if msg == "" {
			t.Errorf("InquiryBlock %s 沒有句子", b)
		}
		if prev, dup := seen[msg]; dup {
			t.Errorf("InquiryBlock %s 與 %s 是同一句：%q", b, prev, msg)
		}
		seen[msg] = b
	}
	if InquiryBlock("UNKNOWN").Text().Text != "" {
		t.Error("不認得的原因不得有句子")
	}
}
