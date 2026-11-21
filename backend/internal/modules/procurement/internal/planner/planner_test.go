package planner

import (
	"reflect"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
)

// 對照測試的「舊」指舊 ai-service：backend/services/ai-service/internal/procurement/ 的 domain/ 與 usecase/。
// 輸入與預期取自舊測試，註解標出來源 file:line。

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
		src     string
		set     slots.Set
		missing []model.SlotKey
		skipped map[model.SlotKey]bool
		streak  int
		want    model.SlotKey // "" 表示沒有問題
		capHit  bool
	}{
		{"required wins over optional", "pick_next_question_test.go:8", slots.Set{}, []model.SlotKey{slots.DeliveryCity}, nil, 0, slots.DeliveryCity, false},
		{"unit-only missing routes to the quantity compound card", "pick_next_question_test.go:44", required(), []model.SlotKey{slots.Unit}, nil, 0, slots.Quantity, false},
		{"first optional is quantity", "pick_next_question_test.go:72", required(), nil, nil, 0, slots.Quantity, false},
		{"then delivery date", "pick_next_question_test.go:72", withQty(required()), nil, nil, 0, slots.DeliveryDate, false},
		{"then packaging", "pick_next_question_test.go:72", withDate(withQty(required())), nil, nil, 0, slots.Packaging, false},
		{"budget last, needs unit", "pick_next_question_test.go:102", withPkg(withDate(withQty(required()))), nil, nil, 0, slots.BudgetPerUnit, false},
		{"quantity known, unit missing asks the unit follow-up, not budget", "pick_next_question_test.go:125", qtyOnly, nil, nil, 0, slots.Quantity, false},
		{"cap stops asking and reports it", "pick_next_question_test.go:150", required(), nil, nil, MaxConsecutiveOptionalQuestions, "", true},
		{"nothing left, not a cap hit", "pick_next_question_test.go:166", full, nil, nil, 0, "", false},
		{"nothing left even at the cap is not a cap hit", "pick_next_question_test.go:166", full, nil, nil, MaxConsecutiveOptionalQuestions, "", false},
		{"skipped optional slot is passed over", "turn.go:456 (skip set consumed by PickNextQuestion)", required(), nil, map[model.SlotKey]bool{slots.Quantity: true}, 0, slots.DeliveryDate, false},
		{"streak below the cap still asks", "pick_next_question_test.go:150", required(), nil, nil, MaxConsecutiveOptionalQuestions - 1, slots.Quantity, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q, capHit := PickNext(c.set, c.missing, c.skipped, c.streak)
			if capHit != c.capHit {
				t.Errorf("(%s) capExhausted = %v, want %v", c.src, capHit, c.capHit)
			}
			switch {
			case c.want == "" && q != nil:
				t.Errorf("(%s) want no question, got %+v", c.src, q)
			case c.want != "" && (q == nil || q.Slot != c.want):
				t.Errorf("(%s) want question for %s, got %+v", c.src, c.want, q)
			}
		})
	}
}

func TestBuildForShapes(t *testing.T) {
	t.Run("delivery city offers the six cities and free text (pick_next_question_test.go:8)", func(t *testing.T) {
		q := BuildFor(slots.DeliveryCity, slots.Set{})
		if len(q.Options) != 6 || !q.AllowFreeText || q.Prompt != PromptDeliveryCity {
			t.Errorf("got %+v", q)
		}
		if q.Options[0].Value != "臺北市" || q.Options[0].Kind != OptionValue {
			t.Errorf("first option = %+v", q.Options[0])
		}
	})
	t.Run("free-text slot has empty non-nil options (pick_next_question_test.go:192)", func(t *testing.T) {
		if q := BuildFor(slots.Packaging, slots.Set{}); q.Options == nil || len(q.Options) != 0 {
			t.Errorf("options = %#v", q.Options)
		}
	})
	t.Run("quantity, both unknown: compound (pick_next_question_test.go:201)", func(t *testing.T) {
		q := BuildFor(slots.Quantity, slots.Set{})
		if q.Slot != slots.Quantity || q.Prompt != PromptQuantity || !q.AllowFreeText || len(q.UnitOptions) != 3 || q.CanSkip {
			t.Errorf("got %+v", q)
		}
	})
	t.Run("quantity known, unit missing: unit follow-up (pick_next_question_test.go:224)", func(t *testing.T) {
		q := BuildFor(slots.Quantity, slots.Set{Quantity: &base.qty})
		if q.Prompt != PromptUnit || q.AllowFreeText || len(q.UnitOptions) != 3 {
			t.Errorf("got %+v", q)
		}
	})
	t.Run("both known: amount only (pick_next_question_test.go:242)", func(t *testing.T) {
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

func TestCanSkip(t *testing.T) { // pick_next_question_test.go:258
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
	t.Run("candidates plus the other option (pick_next_question_test.go:311)", func(t *testing.T) {
		q := BuildItemDisambiguation([]ItemCandidate{{CategoryID: 18, Label: "有機小白菜"}})
		if q.Slot != slots.ItemDisambiguation || q.AllowFreeText || q.CanSkip || q.Prompt != PromptItemChoices || len(q.Options) != 2 {
			t.Fatalf("got %+v", q)
		}
		if q.Options[0].ID != "category-18" || q.Options[0].Value != "有機小白菜" || q.Options[0].Kind != OptionValue {
			t.Errorf("candidate option = %+v", q.Options[0])
		}
		if last := q.Options[1]; last.ID != ItemOtherOptionID || last.Value != "OTHER" || last.Kind != OptionOther {
			t.Errorf("other option = %+v", last)
		}
	})
	t.Run("no candidates: only the describe option with its own prompt (pick_next_question_test.go:346)", func(t *testing.T) {
		q := BuildItemDisambiguation(nil)
		if len(q.Options) != 1 || q.Options[0].ID != ItemOtherOptionID || q.Options[0].Kind != OptionDescribe {
			t.Fatalf("options = %+v", q.Options)
		}
		if q.Prompt != PromptItemDescribe || q.AllowFreeText || q.CanSkip {
			t.Errorf("got %+v", q)
		}
	})
}

func TestNextOptionalStreakParity(t *testing.T) { // usecase/question_history_test.go:115-181，舊 computeOptionalStreak（turn.go:538）
	pkg := &Question{Slot: slots.Packaging}
	cases := []struct {
		name   string
		src    string
		ready  bool
		q      *Question
		capHit bool
		prev   int
		want   int
	}{
		{"still clarifying resets", "question_history_test.go:115", false, &Question{Slot: slots.DeliveryCity}, false, 5, 0},
		{"optional question asked increments", "question_history_test.go:122", true, pkg, false, 2, 3},
		{"cap hit keeps the streak", "question_history_test.go:132", true, nil, true, MaxConsecutiveOptionalQuestions, MaxConsecutiveOptionalQuestions},
		{"disambiguation does not move the streak", "question_history_test.go:153", true, &Question{Slot: slots.ItemDisambiguation}, false, 2, 2},
		{"nothing left to ask resets", "question_history_test.go:166", true, nil, false, 2, 0},
		{"required-slot question when ready resets", "question_history_test.go:176", true, &Question{Slot: slots.DeliveryCity}, false, 2, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NextOptionalStreak(c.ready, c.q, c.capHit, c.prev); got != c.want {
				t.Errorf("(%s) = %d, want %d", c.src, got, c.want)
			}
		})
	}
}

func rec(key model.SlotKey, v string) []slots.Recorded { return []slots.Recorded{{Key: key, Value: v}} }

func TestDecideParity(t *testing.T) { // dialogue_planner_test.go:11-226 的每一個 act 與優先序
	disamb := BuildItemDisambiguation([]ItemCandidate{{CategoryID: 1, Label: "小白菜"}})
	opt := &Question{Slot: slots.Quantity}
	apple, fuji := "蘋果", "富士蘋果"
	cases := []struct {
		name string
		src  string
		in   Input
		want Act
	}{
		{"missing required", "dialogue_planner_test.go:11", Input{Next: slots.Set{Item: &apple}}, AckAndAskRequired},
		{"degraded beats everything", "dialogue_planner_test.go:20", Input{Degraded: true, Rejected: []slots.Rejection{{Key: slots.Item}}}, ExtractionDegraded},
		{"rejected beats correction", "dialogue_planner_test.go:27", Input{
			Rejected: []slots.Rejection{{Key: slots.Item}}, Current: slots.Set{Item: &apple}, Next: ready(), Recorded: rec(slots.Item, "蘋果")}, InputRejected},
		{"correction of an existing field", "dialogue_planner_test.go:36", Input{
			Current: slots.Set{Item: &apple}, Next: withItem(ready(), &fuji), Recorded: rec(slots.Item, fuji)}, CorrectionApplied},
		{"first-time fill is not a correction", "dialogue_planner_test.go:46", Input{
			Next: ready(), Recorded: rec(slots.Item, apple)}, RequestBasicsReady},
		{"ready with a follow-up question", "dialogue_planner_test.go:58", Input{Next: ready(), Question: opt}, AckAndAskOptional},
		{"ready, nothing to ask", "dialogue_planner_test.go:65", Input{Next: ready()}, RequestBasicsReady},
		{"price preview available", "dialogue_planner_test.go:70", Input{Next: ready(), PricePreviewAvailable: true}, PricePreviewAvailable},
		{"rfq stale", "dialogue_planner_test.go:78", Input{Next: ready(), PricePreviewAvailable: true, Rfq: RfqStale}, RfqReconfirmRequired},
		{"rfq confirmed selects approval, not a separate confirmed act", "dialogue_planner_test.go:86", Input{Next: ready(), PricePreviewAvailable: true, Rfq: RfqConfirmed}, PublicationApprovalRequired},
		{"correction beats not-ready", "dialogue_planner_test.go:104", Input{
			Current: slots.Set{Item: &apple}, Next: withItem(slots.Set{Item: &apple}, &fuji), Recorded: rec(slots.Item, fuji)}, CorrectionApplied},
		{"question beats price preview", "dialogue_planner_test.go:128", Input{Next: ready(), PricePreviewAvailable: true, Question: opt}, AckAndAskOptional},
		{"disambiguation beats not-ready", "dialogue_planner_test.go:161", Input{Next: slots.Set{}, Question: disamb}, ItemNeedsDisambiguation},
		{"rejected beats disambiguation", "dialogue_planner_test.go:170", Input{Rejected: []slots.Rejection{{Key: slots.Unit}}, Question: disamb}, InputRejected},
		{"disambiguation with a ready request", "dialogue_planner_test.go:177", Input{Next: ready(), Question: disamb}, ItemNeedsDisambiguation},
		{"stale rfq beats an optional question", "selectActCode order: ready, then rfq state, then question", Input{Next: ready(), Rfq: RfqStale, Question: opt}, RfqReconfirmRequired},
		{"confirmed rfq beats price preview", "selectActCode order", Input{Next: ready(), Rfq: RfqConfirmed, PricePreviewAvailable: true}, PublicationApprovalRequired},
	}
	reached := map[Act]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.in).Act
			reached[got] = true
			if got != c.want {
				t.Errorf("(%s) act = %s, want %s", c.src, got, c.want)
			}
		})
	}
	for _, a := range All { // dialogue_planner_test.go:136：每個 act 至少有一組輸入選得到
		if !reached[a] {
			t.Errorf("act %s is not reached by any case", a)
		}
	}
}

func withItem(s slots.Set, item *string) slots.Set { s.Item = item; return s }

func TestDecideSalientFactsCapped(t *testing.T) { // 舊 salientFactRefsFor（dialogue_planner.go:148）：最多 2 個，維持 recorded 的順序
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
