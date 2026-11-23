package turn

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func fieldsOf(t *testing.T, err error) []string {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("要 INVALID_ARGUMENT，得到 %v", err)
	}
	var out []string
	for _, v := range ae.FieldViolations() {
		out = append(out, v.Field)
	}
	return out
}

func TestPlanAnswerChecksTheShapeAndNamesTheFields(t *testing.T) {
	for name, tc := range map[string]struct {
		in     AnswerSlotInput
		fields []string // 空是合法
	}{
		"city by option":                {AnswerSlotInput{Slot: slots.DeliveryCity, OptionID: "city-taipei"}, nil},
		"city by text":                  {AnswerSlotInput{Slot: slots.DeliveryCity, FreeText: "宜蘭縣"}, nil},
		"city option not on the card":   {AnswerSlotInput{Slot: slots.DeliveryCity, OptionID: "city-mars"}, []string{"OptionID"}},
		"city option and text":          {AnswerSlotInput{Slot: slots.DeliveryCity, OptionID: "city-taipei", FreeText: "x"}, []string{"OptionID", "FreeText", "Skip"}},
		"nothing":                       {AnswerSlotInput{Slot: slots.DeliveryCity}, []string{"OptionID", "FreeText", "Skip"}},
		"blank text":                    {AnswerSlotInput{Slot: slots.Packaging, FreeText: " \t　"}, []string{"FreeText"}},
		"unit option on a single slot":  {AnswerSlotInput{Slot: slots.Packaging, FreeText: "箱", UnitOptionID: "unit-kg"}, []string{"UnitOptionID"}},
		"skip packaging":                {AnswerSlotInput{Slot: slots.Packaging, Skip: true}, nil},
		"skip budget":                   {AnswerSlotInput{Slot: slots.BudgetPerUnit, Skip: true}, nil},
		"skip city is required":         {AnswerSlotInput{Slot: slots.DeliveryCity, Skip: true}, []string{"Skip"}},
		"skip date is quote required":   {AnswerSlotInput{Slot: slots.DeliveryDate, Skip: true}, []string{"Skip"}},
		"skip quantity":                 {AnswerSlotInput{Slot: slots.Quantity, Skip: true}, []string{"Skip"}},
		"skip with text":                {AnswerSlotInput{Slot: slots.Packaging, Skip: true, FreeText: "箱"}, []string{"OptionID", "FreeText", "Skip"}},
		"quantity text and unit":        {AnswerSlotInput{Slot: slots.Quantity, FreeText: "20", UnitOptionID: "unit-kg"}, nil},
		"quantity text only":            {AnswerSlotInput{Slot: slots.Quantity, FreeText: "20"}, nil},
		"quantity unit only":            {AnswerSlotInput{Slot: slots.Quantity, UnitOptionID: "unit-catty"}, nil},
		"quantity neither":              {AnswerSlotInput{Slot: slots.Quantity}, []string{"FreeText", "UnitOptionID"}},
		"quantity option id":            {AnswerSlotInput{Slot: slots.Quantity, OptionID: "x", FreeText: "20"}, []string{"OptionID"}},
		"quantity not a number":         {AnswerSlotInput{Slot: slots.Quantity, FreeText: "二十"}, []string{"FreeText"}},
		"quantity bad unit":             {AnswerSlotInput{Slot: slots.Quantity, FreeText: "2", UnitOptionID: "unit-ton"}, []string{"UnitOptionID"}},
		"budget not a number":           {AnswerSlotInput{Slot: slots.BudgetPerUnit, FreeText: "很多"}, []string{"FreeText"}},
		"item by option":                {AnswerSlotInput{Slot: slots.Item, OptionID: "category-1"}, []string{"OptionID"}},
		"item text":                     {AnswerSlotInput{Slot: slots.Item, FreeText: "蘋果"}, nil},
		"item skip":                     {AnswerSlotInput{Slot: slots.Item, Skip: true}, []string{"Skip"}},
		"unit slot folds into quantity": {AnswerSlotInput{Slot: slots.Unit, OptionID: "unit-kg"}, []string{"SlotKey"}},
		"disambiguation is run owned":   {AnswerSlotInput{Slot: slots.ItemDisambiguation, FreeText: "x"}, []string{"SlotKey"}},
		"resolution is server filled":   {AnswerSlotInput{Slot: slots.ItemResolution, FreeText: "x"}, []string{"SlotKey"}},
		"no slot":                       {AnswerSlotInput{FreeText: "x"}, []string{"SlotKey"}},
		"text too long":                 {AnswerSlotInput{Slot: slots.Packaging, FreeText: strings.Repeat("字", MaxFreeTextRunes+1)}, []string{"FreeText"}},
	} {
		_, err := planAnswer(tc.in)
		if len(tc.fields) == 0 {
			if err != nil {
				t.Errorf("%s：%v，要合法", name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s：要 INVALID_ARGUMENT %v", name, tc.fields)
			continue
		}
		if got := fieldsOf(t, err); !slices.Equal(got, tc.fields) {
			t.Errorf("%s：欄位 %v，要 %v", name, got, tc.fields)
		}
	}
}

func TestPlanAnswerMessageIsTheOptionValueOrTheTrimmedText(t *testing.T) {
	for name, tc := range map[string]struct {
		in   AnswerSlotInput
		want string
	}{
		"city option":   {AnswerSlotInput{Slot: slots.DeliveryCity, OptionID: "city-taipei"}, "臺北市"},
		"text":          {AnswerSlotInput{Slot: slots.Packaging, FreeText: "  箱  "}, "箱"},
		"quantity":      {AnswerSlotInput{Slot: slots.Quantity, FreeText: "20", UnitOptionID: "unit-kg"}, "20 KG"},
		"quantity text": {AnswerSlotInput{Slot: slots.Quantity, FreeText: "20"}, "20"},
		"skip":          {AnswerSlotInput{Slot: slots.Packaging, Skip: true}, ""},
	} {
		p, err := planAnswer(tc.in)
		if err != nil || p.message != tc.want {
			t.Errorf("%s：message = %q %v，要 %q", name, p.message, err, tc.want)
		}
	}
}

func ip32(i int32) *int32 { return &i }
func dp() *time.Time      { d := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC); return &d }

func readyPrior() priorState {
	return priorState{current: resolved(), requestOf: statusRequirementReady}
}

func TestAnswerDecisionAppliesTheAnswerAndRecordsIt(t *testing.T) {
	prior := priorState{current: slots.Set{Item: sp("蘋果"), Unit: sp("KG")}, requestOf: statusClarifying}
	p, err := planAnswer(AnswerSlotInput{Slot: slots.DeliveryCity, OptionID: "city-taipei"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := answerDecision(uitext.ZhTW, now, prior, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.next.DeliveryCity == nil || *d.next.DeliveryCity != "臺北市" || len(d.recorded) != 1 || d.interrupt != nil {
		t.Fatalf("decision = %+v", d)
	}
	if d.question == nil && d.status == statusClarifying {
		t.Errorf("還缺品項解析卻沒有問題：%+v", d)
	}
}

func TestAnswerDecisionQuantityNeedsAnExistingUnitOrAUnitOption(t *testing.T) {
	prior := priorState{current: slots.Set{Item: sp("蘋果")}, requestOf: statusClarifying}
	p, _ := planAnswer(AnswerSlotInput{Slot: slots.Quantity, FreeText: "20"})
	_, err := answerDecision(uitext.ZhTW, now, prior, p, nil)
	if got := fieldsOf(t, err); !slices.Equal(got, []string{"UnitOptionID"}) {
		t.Fatalf("缺單位 = %v", got)
	}
	p, _ = planAnswer(AnswerSlotInput{Slot: slots.Quantity, UnitOptionID: "unit-kg"})
	_, err = answerDecision(uitext.ZhTW, now, prior, p, nil)
	if got := fieldsOf(t, err); !slices.Equal(got, []string{"FreeText"}) {
		t.Fatalf("缺數量 = %v", got)
	}
	prior.current.Unit = sp("CATTY")
	p, _ = planAnswer(AnswerSlotInput{Slot: slots.Quantity, FreeText: "20"})
	d, err := answerDecision(uitext.ZhTW, now, prior, p, nil)
	if err != nil || d.next.Quantity == nil || *d.next.Quantity != 20 || *d.next.Unit != "CATTY" {
		t.Fatalf("已有單位時只給數量 = %+v %v", d.next, err)
	}
}

func TestAnswerDecisionOutOfRangeValuesAreRejectedInTheResultNotAsAnError(t *testing.T) {
	p, _ := planAnswer(AnswerSlotInput{Slot: slots.BudgetPerUnit, FreeText: "0"})
	d, err := answerDecision(uitext.ZhTW, now, readyPrior(), p, nil)
	if err != nil || len(d.rejected) != 1 || d.rejected[0].Key != slots.BudgetPerUnit || d.next.BudgetPerUnit != nil {
		t.Fatalf("rejected = %+v err = %v", d.rejected, err)
	}
}

func TestAnswerDecisionSkipCarriesTheSkippedSlotAndAsksSomethingElse(t *testing.T) {
	prior := readyPrior()
	prior.current.Quantity, prior.current.DeliveryDate = ip32(3), dp()
	p, _ := planAnswer(AnswerSlotInput{Slot: slots.Packaging, Skip: true})
	d, err := answerDecision(uitext.ZhTW, now, prior, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(d.plan.Skipped, slots.Packaging) {
		t.Fatalf("skipped = %v", d.plan.Skipped)
	}
	if d.question == nil || d.question.Slot == slots.Packaging {
		t.Errorf("略過的欄位不再問：%+v", d.question)
	}
	if !sameContent(prior.current, d.next) {
		t.Error("略過不改內容")
	}
}

func TestAnswerDecisionItemTakesTheResolutionAndRegressesMatching(t *testing.T) {
	prior := readyPrior()
	prior.requestOf = statusMatching
	res := &slots.Resolution{Kind: slots.CategoryConfirmed, CategoryID: 9, Label: "梨"}
	p, _ := planAnswer(AnswerSlotInput{Slot: slots.Item, FreeText: "梨"})
	d, err := answerDecision(uitext.ZhTW, now, prior, p, res)
	if err != nil {
		t.Fatal(err)
	}
	if *d.next.Item != "梨" || d.next.Resolution == nil || d.next.Resolution.CategoryID != 9 || d.interrupt != nil {
		t.Fatalf("next = %+v", d.next)
	}
	if d.status != statusRequirementReady {
		t.Errorf("MATCHING 時改了品項解析要退回，status = %s", d.status)
	}
}

func TestAnswerDecisionSkipIsNotARejectedInput(t *testing.T) {
	for _, slot := range []model.SlotKey{slots.Packaging, slots.BudgetPerUnit} {
		prior := readyPrior()
		prior.current.Quantity, prior.current.DeliveryDate = ip32(3), dp()
		p, err := planAnswer(AnswerSlotInput{Slot: slot, Skip: true})
		if err != nil {
			t.Fatal(err)
		}
		d, err := answerDecision(uitext.ZhTW, now, prior, p, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.rejected) != 0 || len(d.recorded) != 0 {
			t.Errorf("%s：略過不該有拒絕或記錄：rejected=%+v recorded=%+v", slot, d.rejected, d.recorded)
		}
		if d.plan.Act == planner.InputRejected {
			t.Errorf("%s：略過不該出 INPUT_REJECTED", slot)
		}
	}
}

func TestAnswerDecisionSkippingTheSameSlotTwiceStoresItOnce(t *testing.T) {
	prior := readyPrior()
	prior.current.Quantity, prior.current.DeliveryDate = ip32(3), dp()
	p, _ := planAnswer(AnswerSlotInput{Slot: slots.Packaging, Skip: true})
	d1, err := answerDecision(uitext.ZhTW, now, prior, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	prior.prevTurn = &prevTurn{skipped: d1.plan.Skipped}
	d2, err := answerDecision(uitext.ZhTW, now, prior, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := slices.Index(d2.plan.Skipped, slots.Packaging); n < 0 || slices.Index(d2.plan.Skipped[n+1:], slots.Packaging) >= 0 {
		t.Errorf("skipped = %v，要只出現一次", d2.plan.Skipped)
	}
}

func TestPlanAnswerRejectsUnitSlotBecauseUnitAnswersUseQuantity(t *testing.T) {
	_, err := planAnswer(AnswerSlotInput{Slot: slots.Unit, UnitOptionID: "unit-kg"})
	if got := fieldsOf(t, err); !slices.Equal(got, []string{"SlotKey"}) {
		t.Fatalf("UNIT 回答 = %v，要 SlotKey", got)
	}
}
