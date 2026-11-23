package planner

import (
	"reflect"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
)

var base = struct {
	item, city, unit, pkg string
	qty                   int32
	budget                int64
	date                  time.Time
}{"水果禮盒", "桃園市", "KG", "簡易", 10, 500, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}

// required 是兩個必填都滿足的需求（單位與數量都還沒填）。
func required() slots.Set { return slots.Set{Item: &base.item, DeliveryCity: &base.city} }

func ready() slots.Set {
	s := required()
	s.Unit = &base.unit
	return s
}

func TestPickNextParity(t *testing.T) {
	withQty := func(s slots.Set) slots.Set { s.Quantity = &base.qty; s.Unit = &base.unit; return s }
	withDate := func(s slots.Set) slots.Set { s.DeliveryDate = &base.date; return s }
	withPkg := func(s slots.Set) slots.Set { s.Packaging = &base.pkg; return s }
	withBudget := func(s slots.Set) slots.Set { s.BudgetPerUnit = &base.budget; return s }
	qtyOnly := required()
	qtyOnly.Quantity = &base.qty
	qtyOnly = withDate(qtyOnly)
	full := withBudget(withPkg(withDate(withQty(required()))))

	cases := []struct {
		name    string
		set     slots.Set
		missing []model.SlotKey
		skipped map[model.SlotKey]bool
		streak  int
		want    model.SlotKey // "" 表示沒有問題
		capHit  bool
	}{
		{"required wins over optional", slots.Set{}, []model.SlotKey{slots.DeliveryCity}, nil, 0, slots.DeliveryCity, false},
		{"unit-only missing routes to the quantity compound card", required(), []model.SlotKey{slots.Unit}, nil, 0, slots.Quantity, false},
		{"first optional is quantity", required(), nil, nil, 0, slots.Quantity, false},
		{"then delivery date", withQty(required()), nil, nil, 0, slots.DeliveryDate, false},
		{"then packaging", withDate(withQty(required())), nil, nil, 0, slots.Packaging, false},
		{"budget last, needs unit", withPkg(withDate(withQty(required()))), nil, nil, 0, slots.BudgetPerUnit, false},
		{"quantity known, unit missing asks the unit follow-up, not budget", qtyOnly, nil, nil, 0, slots.Quantity, false},
		{"cap stops asking and reports it", required(), nil, nil, MaxConsecutiveOptionalQuestions, "", true},
		{"nothing left, not a cap hit", full, nil, nil, 0, "", false},
		{"nothing left even at the cap is not a cap hit", full, nil, nil, MaxConsecutiveOptionalQuestions, "", false},
		{"skipped optional slot is passed over", required(), nil, map[model.SlotKey]bool{slots.Quantity: true}, 0, slots.DeliveryDate, false},
		{"streak below the cap still asks", required(), nil, nil, MaxConsecutiveOptionalQuestions - 1, slots.Quantity, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q, capHit := PickNext(c.set, c.missing, c.skipped, c.streak)
			if capHit != c.capHit {
				t.Errorf("capExhausted = %v, want %v", capHit, c.capHit)
			}
			switch {
			case c.want == "" && q != nil:
				t.Errorf("want no question, got %+v", q)
			case c.want != "" && (q == nil || q.Slot != c.want):
				t.Errorf("want question for %s, got %+v", c.want, q)
			}
		})
	}
}

func TestBuildForShapes(t *testing.T) {
	t.Run("delivery city offers the six cities and free text", func(t *testing.T) {
		q := BuildFor(slots.DeliveryCity, slots.Set{})
		if len(q.Options) != 6 || !q.AllowFreeText || q.Prompt != PromptDeliveryCity {
			t.Errorf("got %+v", q)
		}
		if q.Options[0].Value != "臺北市" || q.Options[0].Kind != OptionValue {
			t.Errorf("first option = %+v", q.Options[0])
		}
	})
	t.Run("free-text slot has empty non-nil options", func(t *testing.T) {
		if q := BuildFor(slots.Packaging, slots.Set{}); q.Options == nil || len(q.Options) != 0 {
			t.Errorf("options = %#v", q.Options)
		}
	})
	t.Run("quantity, both unknown: compound", func(t *testing.T) {
		q := BuildFor(slots.Quantity, slots.Set{})
		if q.Slot != slots.Quantity || q.Prompt != PromptQuantity || !q.AllowFreeText || len(q.UnitOptions) != 3 || q.CanSkip {
			t.Errorf("got %+v", q)
		}
	})
	t.Run("quantity known, unit missing: unit follow-up", func(t *testing.T) {
		q := BuildFor(slots.Quantity, slots.Set{Quantity: &base.qty})
		if q.Prompt != PromptUnit || q.AllowFreeText || len(q.UnitOptions) != 3 {
			t.Errorf("got %+v", q)
		}
	})
	t.Run("both known: amount only", func(t *testing.T) {
		q := BuildFor(slots.Quantity, slots.Set{Quantity: &base.qty, Unit: &base.unit})
		if q.Prompt != PromptQuantity || !q.AllowFreeText || len(q.UnitOptions) != 0 {
			t.Errorf("got %+v", q)
		}
	})
	t.Run("unit options map to unit codes", func(t *testing.T) {
		q := BuildFor(slots.Unit, slots.Set{})
		got := []string{q.UnitOptions[0].Value, q.UnitOptions[1].Value, q.UnitOptions[2].Value}
		if !reflect.DeepEqual(got, slots.AllowedUnits) {
			t.Errorf("unit option values = %v, want %v", got, slots.AllowedUnits)
		}
	})
	t.Run("returned options are copies of the whitelist", func(t *testing.T) {
		BuildFor(slots.DeliveryCity, slots.Set{}).Options[0].Value = "改過"
		if cityOptions[0].Value != "臺北市" {
			t.Error("mutating a built question changed the whitelist")
		}
	})
}

func TestCanSkip(t *testing.T) {
	want := map[model.SlotKey]bool{
		slots.Item: false, slots.DeliveryCity: false, slots.DeliveryDate: false, slots.Quantity: false,
		slots.Packaging: true, slots.BudgetPerUnit: true,
	}
	for key, w := range want {
		if got := BuildFor(key, slots.Set{}).CanSkip; got != w {
			t.Errorf("BuildFor(%s).CanSkip = %v, want %v", key, got, w)
		}
	}
}

func TestItemDisambiguation(t *testing.T) {
	t.Run("candidates plus the other option", func(t *testing.T) {
		q := BuildItemDisambiguation([]ItemCandidate{{CategoryID: 18, Label: "有機小白菜"}})
		if q.Slot != slots.ItemDisambiguation || !q.AllowFreeText || q.CanSkip || q.Prompt != PromptItemChoices || len(q.Options) != 2 {
			t.Fatalf("got %+v", q)
		}
		if q.Options[0].ID != "category-18" || q.Options[0].Value != "有機小白菜" || q.Options[0].Kind != OptionValue {
			t.Errorf("candidate option = %+v", q.Options[0])
		}
		if last := q.Options[1]; last.ID != ItemOtherOptionID || last.Value != "OTHER" || last.Kind != OptionOther {
			t.Errorf("other option = %+v", last)
		}
	})
	t.Run("no candidates: only the describe option with its own prompt", func(t *testing.T) {
		q := BuildItemDisambiguation(nil)
		if len(q.Options) != 1 || q.Options[0].ID != ItemOtherOptionID || q.Options[0].Kind != OptionDescribe {
			t.Fatalf("options = %+v", q.Options)
		}
		if q.Prompt != PromptItemDescribe || !q.AllowFreeText || q.CanSkip {
			t.Errorf("got %+v", q)
		}
	})
}

func TestItemDisambiguationCapsAtFiveAndLabelsWithThePath(t *testing.T) {
	var cands []ItemCandidate
	for i := int64(1); i <= 8; i++ {
		cands = append(cands, ItemCandidate{CategoryID: i, Label: "品項", Path: "大類 > 品項"})
	}
	q := BuildItemDisambiguation(cands)
	if len(q.Options) != MaxItemOptions+1 {
		t.Fatalf("options = %d，要 %d 個品項加「其他」", len(q.Options), MaxItemOptions+1)
	}
	if q.Options[0].Value != "大類 > 品項" || q.Options[0].ID != "category-1" || q.Options[MaxItemOptions-1].ID != "category-5" {
		t.Errorf("options = %+v：要帶路徑、依給定順序截前 5 個", q.Options)
	}
	if last := q.Options[MaxItemOptions]; last.ID != ItemOtherOptionID || last.Kind != OptionOther {
		t.Errorf("last = %+v", last)
	}
	if !q.AllowFreeText || q.CanSkip {
		t.Errorf("要開放自由輸入、不可跳過：%+v", q)
	}
}

func TestNextOptionalStreakParity(t *testing.T) {
	pkg := &Question{Slot: slots.Packaging}
	cases := []struct {
		name   string
		ready  bool
		q      *Question
		capHit bool
		prev   int
		want   int
	}{
		{"still clarifying resets", false, &Question{Slot: slots.DeliveryCity}, false, 5, 0},
		{"optional question asked increments", true, pkg, false, 2, 3},
		{"cap hit keeps the streak", true, nil, true, MaxConsecutiveOptionalQuestions, MaxConsecutiveOptionalQuestions},
		{"disambiguation does not move the streak", true, &Question{Slot: slots.ItemDisambiguation}, false, 2, 2},
		{"nothing left to ask resets", true, nil, false, 2, 0},
		{"required-slot question when ready resets", true, &Question{Slot: slots.DeliveryCity}, false, 2, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NextOptionalStreak(c.ready, c.q, c.capHit, c.prev); got != c.want {
				t.Errorf("= %d, want %d", got, c.want)
			}
		})
	}
}

func rec(key model.SlotKey, v string) []slots.Recorded { return []slots.Recorded{{Key: key, Value: v}} }

func TestDecideParity(t *testing.T) {
	disamb := BuildItemDisambiguation([]ItemCandidate{{CategoryID: 1, Label: "小白菜"}})
	opt := &Question{Slot: slots.Quantity}
	apple, fuji := "蘋果", "富士蘋果"
	cases := []struct {
		name string
		in   Input
		want Act
	}{
		{"missing required", Input{Next: slots.Set{Item: &apple}}, AckAndAskRequired},
		{"rejected beats correction", Input{
			Rejected: []slots.Rejection{{Key: slots.Item}}, Current: slots.Set{Item: &apple}, Next: ready(), Recorded: rec(slots.Item, "蘋果")}, InputRejected},
		{"correction of an existing field", Input{
			Current: slots.Set{Item: &apple}, Next: withItem(ready(), &fuji), Recorded: rec(slots.Item, fuji)}, CorrectionApplied},
		{"first-time fill is not a correction", Input{
			Next: ready(), Recorded: rec(slots.Item, apple)}, RequestBasicsReady},
		{"ready with a follow-up question", Input{Next: ready(), Question: opt}, AckAndAskOptional},
		{"ready, nothing to ask", Input{Next: ready()}, RequestBasicsReady},
		{"rfq stale", Input{Next: ready(), Rfq: RfqStale}, RfqReconfirmRequired},
		{"rfq confirmed selects approval, not a separate confirmed act", Input{Next: ready(), Rfq: RfqConfirmed}, PublicationApprovalRequired},
		{"correction beats not-ready", Input{
			Current: slots.Set{Item: &apple}, Next: withItem(slots.Set{Item: &apple}, &fuji), Recorded: rec(slots.Item, fuji)}, CorrectionApplied},
		{"question beats nothing to ask", Input{Next: ready(), Question: opt}, AckAndAskOptional},
		{"disambiguation beats not-ready", Input{Next: slots.Set{}, Question: disamb}, ItemNeedsDisambiguation},
		{"rejected beats disambiguation", Input{Rejected: []slots.Rejection{{Key: slots.Unit}}, Question: disamb}, InputRejected},
		{"corrected beats disambiguation", Input{Current: slots.Set{Item: &apple}, Next: ready(), Recorded: rec(slots.Item, "蘋果"), Question: disamb}, CorrectionApplied},
		{"disambiguation with a ready request", Input{Next: ready(), Question: disamb}, ItemNeedsDisambiguation},
		{"stale rfq beats an optional question", Input{Next: ready(), Rfq: RfqStale, Question: opt}, RfqReconfirmRequired},
		{"confirmed rfq beats an optional question", Input{Next: ready(), Rfq: RfqConfirmed, Question: opt}, PublicationApprovalRequired},
	}
	reached := map[Act]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.in).Act
			reached[got] = true
			if got != c.want {
				t.Errorf("act = %s, want %s", got, c.want)
			}
		})
	}
	for _, a := range All {
		if !reached[a] {
			t.Errorf("act %s is not reached by any case", a)
		}
	}
}

func withItem(s slots.Set, item *string) slots.Set { s.Item = item; return s }

func TestDecideSalientFactsCapped(t *testing.T) {
	rs := []slots.Recorded{{Key: slots.Item, Value: "a"}, {Key: slots.Quantity, Value: "1"}, {Key: slots.Unit, Value: "KG"}}
	p := Decide(Input{Next: ready(), Recorded: rs})
	if !reflect.DeepEqual(p.Salient, rs[:2]) {
		t.Errorf("salient = %v", p.Salient)
	}
	if got := Decide(Input{Next: ready()}).Salient; len(got) != 0 {
		t.Errorf("salient = %v, want none", got)
	}
}

func TestDecidePassesQuestionThrough(t *testing.T) {
	q := &Question{Slot: slots.Packaging}
	if got := Decide(Input{Next: ready(), Question: q}).Question; got != q {
		t.Errorf("question = %v", got)
	}
}

func TestTruncatedValueIsAnAdjustmentNotARejection(t *testing.T) {
	trunc := slots.Rejection{Key: slots.Item, Reason: slots.ReasonTruncated}
	p := Decide(Input{Next: ready(), Rejected: []slots.Rejection{trunc}, Recorded: rec(slots.Item, "a")})
	if p.Act != RequestBasicsReady {
		t.Errorf("act = %s, want %s", p.Act, RequestBasicsReady)
	}
	if !reflect.DeepEqual(p.Adjusted, []slots.Rejection{trunc}) {
		t.Errorf("adjusted = %v", p.Adjusted)
	}
	missing := Decide(Input{Next: slots.Set{}, Rejected: []slots.Rejection{trunc}})
	if missing.Act != AckAndAskRequired {
		t.Errorf("act with missing required = %s, want %s", missing.Act, AckAndAskRequired)
	}
	both := Decide(Input{Next: ready(), Rejected: []slots.Rejection{trunc, {Key: slots.Unit, Reason: slots.ReasonUnitNotAllowed}}})
	if both.Act != InputRejected || len(both.Adjusted) != 1 {
		t.Errorf("act = %s, adjusted = %v; a real rejection still wins", both.Act, both.Adjusted)
	}
}

func TestCorrectedFieldsComeFirstInSalient(t *testing.T) {
	apple, fuji := "蘋果", "富士蘋果"
	rs := []slots.Recorded{{Key: slots.Quantity, Value: "5"}, {Key: slots.Packaging, Value: "箱"}, {Key: slots.Item, Value: fuji}}
	p := Decide(Input{Current: slots.Set{Item: &apple}, Next: withItem(ready(), &fuji), Recorded: rs})
	if p.Act != CorrectionApplied || len(p.Salient) != MaxSalientFacts || p.Salient[0].Key != slots.Item {
		t.Errorf("act = %s, salient = %v; the corrected item must lead", p.Act, p.Salient)
	}
	q := 3
	qty := int32(q)
	cur := slots.Set{Item: &apple, Quantity: &qty, DeliveryCity: &base.city}
	three := []slots.Recorded{{Key: slots.Item, Value: fuji}, {Key: slots.Quantity, Value: "4"}, {Key: slots.DeliveryCity, Value: "臺北市"}}
	if got := Decide(Input{Current: cur, Next: ready(), Recorded: three}).Salient; len(got) != 3 {
		t.Errorf("salient = %v, want every corrected field", got)
	}
}

func TestDecideCarriesOptionalProgress(t *testing.T) {
	pkg := &Question{Slot: slots.Packaging}
	skipped := []model.SlotKey{slots.Quantity}
	p := Decide(Input{Next: ready(), Question: pkg, PrevStreak: 2, Skipped: skipped})
	if p.OptionalStreak != 3 || !reflect.DeepEqual(p.Skipped, skipped) {
		t.Errorf("streak = %d, skipped = %v", p.OptionalStreak, p.Skipped)
	}
	capped := Decide(Input{Next: ready(), PrevStreak: MaxConsecutiveOptionalQuestions, CapExhausted: true})
	if capped.OptionalStreak != MaxConsecutiveOptionalQuestions {
		t.Errorf("streak when capped = %d", capped.OptionalStreak)
	}
	if Decide(Input{Next: slots.Set{}, PrevStreak: 2}).OptionalStreak != 0 {
		t.Error("streak must reset while required fields are missing")
	}
}

func TestFirstExplicitExactMatchIsNotACorrection(t *testing.T) {
	buyerSet := false
	rs := rec(slots.ExactMatchRequired, "false")
	first := Decide(Input{Current: ready(), Next: func() slots.Set { s := ready(); s.ExactMatchRequired = &buyerSet; return s }(), Recorded: rs})
	if first.Act == CorrectionApplied {
		t.Error("setting ExactMatchRequired for the first time must not count as a correction")
	}
	cur := ready()
	yes := true
	cur.ExactMatchRequired = &yes
	if Decide(Input{Current: cur, Next: ready(), Recorded: rs}).Act != CorrectionApplied {
		t.Error("changing a buyer-set value is a correction")
	}
}
