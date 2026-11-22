package turn

import (
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func TestCheckAnswerShape(t *testing.T) {
	item := itemPayload(t)
	describe := *interrupt(uitext.ZhTW, describeCard("q")).Next
	for name, tc := range map[string]struct {
		p    InterruptPayload
		a    Answer
		want InvalidReason
	}{
		"option on the card":            {item, Answer{OptionID: "category-2"}, ""},
		"free text":                     {item, Answer{FreeText: "蘋果乾"}, ""},
		"free text on a describe card":  {describe, Answer{FreeText: "自家種的"}, ""},
		"both":                          {item, Answer{OptionID: "category-2", FreeText: "x"}, InvalidNeedExactlyOne},
		"neither":                       {item, Answer{}, InvalidNeedExactlyOne},
		"blank text":                    {item, Answer{FreeText: " \t　"}, InvalidBlankText},
		"option not on the card":        {item, Answer{OptionID: "category-999"}, InvalidOptionNotOnCard},
		"option on a describe card":     {describe, Answer{OptionID: "category-2"}, InvalidOptionNotOnCard},
		"unknown stage":                 {InterruptPayload{Stage: "OTHER"}, Answer{FreeText: "x"}, InvalidUnknownStage},
		"whitespace text beside option": {item, Answer{OptionID: "category-2", FreeText: " "}, ""},
	} {
		if got := CheckAnswer(tc.p, tc.a); got != tc.want {
			t.Errorf("%s：CheckAnswer = %q，要 %q", name, got, tc.want)
		}
	}
}

func TestCheckShapeReportsFieldViolationsWithoutAReason(t *testing.T) {
	item := itemPayload(t)
	long := strings.Repeat("字", MaxFreeTextRunes+1)
	for name, tc := range map[string]struct {
		in     ResumeInput
		fields []string
	}{
		"both set":                 {ResumeInput{OptionID: "category-2", FreeText: "x"}, []string{"OptionID", "FreeText"}},
		"neither set":              {ResumeInput{}, []string{"OptionID", "FreeText"}},
		"blank text":               {ResumeInput{FreeText: "  "}, []string{"FreeText"}},
		"option not on it":         {ResumeInput{OptionID: "category-999"}, []string{"OptionID"}},
		"text too long":            {ResumeInput{FreeText: long}, []string{"FreeText"}},
		"option beside blank text": {ResumeInput{OptionID: "category-2", FreeText: " "}, []string{"OptionID", "FreeText"}},
	} {
		err := checkShape(item, tc.in)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
			t.Errorf("%s：要 INVALID_ARGUMENT，得到 %v", name, err)
			continue
		}
		if ae.Reason != (apperr.Reason{}) {
			t.Errorf("%s：不該帶 ErrorReason：%v", name, ae.Reason)
		}
		var got []string
		for _, v := range ae.FieldViolations() {
			got = append(got, v.Field)
		}
		if strings.Join(got, ",") != strings.Join(tc.fields, ",") {
			t.Errorf("%s：欄位 %v，要 %v", name, got, tc.fields)
		}
	}
	for name, in := range map[string]ResumeInput{
		"option":            {OptionID: "category-2"},
		"text":              {FreeText: "蘋果乾"},
		"text at the limit": {FreeText: strings.Repeat("字", MaxFreeTextRunes)},
	} {
		if err := checkShape(item, in); err != nil {
			t.Errorf("%s：形狀合法：%v", name, err)
		}
	}
}

func TestDecideAnswerResolvedRecordsTheResolutionAndMovesTheStatus(t *testing.T) {
	item := "蘋果"
	prior := priorState{current: slots.Set{Item: &item}, requestOf: statusClarifying}
	out := Outcome{Kind: OutcomeResolved, Resolution: &slots.Resolution{Kind: slots.CategoryConfirmed, CategoryID: 2, Label: "蘋果乾"}}
	d, err := decideAnswer(uitext.ZhTW, now, prior, out)
	if err != nil {
		t.Fatal(err)
	}
	if d.next.Resolution == nil || d.next.Resolution.CategoryID != 2 || d.interrupt != nil {
		t.Fatalf("解析要進新版本、不再問：%+v", d)
	}
	if len(d.recorded) != 1 {
		t.Errorf("這一輪要記下品項解析：%+v", d.recorded)
	}
}

func TestDecideAnswerReaskKeepsTheCurrentFieldsAndCarriesTheNewCard(t *testing.T) {
	item := "蘋果"
	prior := priorState{current: slots.Set{Item: &item}, requestOf: statusClarifying}
	card := buildCard("蘋果", []resolver.Match{match(1, "蘋果", "水果 > 蘋果"), match(2, "蘋果乾", "加工品 > 蘋果乾")})
	out := interrupt(uitext.ZhTW, card)
	d, err := decideAnswer(uitext.ZhTW, now, prior, out)
	if err != nil {
		t.Fatal(err)
	}
	if d.interrupt == nil || d.interrupt.Stage != StageItem || d.next.Resolution != nil {
		t.Fatalf("要停在新卡、不寫解析：%+v", d)
	}
	if d.status != statusClarifying {
		t.Errorf("品項未解析不算齊全：%s", d.status)
	}
}

func TestDecideAnswerInvalidFailsAsAnswerRejected(t *testing.T) {
	for name, out := range map[string]Outcome{
		"invalid":            invalid(InvalidResolutionReject),
		"interrupt w/o card": {Kind: OutcomeInterrupt},
		"zero outcome":       {},
	} {
		_, err := decideAnswer(uitext.ZhTW, now, priorState{}, out)
		var f *run.FailureError
		if name == "interrupt w/o card" {
			if err == nil {
				t.Errorf("%s：要錯誤", name)
			}
			continue
		}
		if !errors.As(err, &f) || f.Reason != run.FailureAnswerRejected {
			t.Errorf("%s：要 ANSWER_REJECTED，得到 %v", name, err)
		}
	}
}

func TestAnswerTextShowsTheOptionLabelOrTheTrimmedText(t *testing.T) {
	p := itemPayload(t)
	label := ""
	for _, o := range p.Question.Options {
		if o.OptionID == "category-2" {
			label = o.Label
		}
	}
	if label == "" {
		t.Fatal("前置：卡上要有 category-2")
	}
	if got := answerText(p, "category-2", ""); got != label {
		t.Errorf("選項：%q，要 %q", got, label)
	}
	if got := answerText(p, "", "  自家種的 "); got != "自家種的" {
		t.Errorf("文字：%q", got)
	}
}
