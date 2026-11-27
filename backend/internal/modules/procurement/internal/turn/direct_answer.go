package turn

import (
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// AnswerSlotInput 是 SubmitAnswer 的輸入（A9-D31、A9-D33(1)）：只收確定性的回答（選項、數量、略過；品項文字須唯一對到一個末層分類）。
type AnswerSlotInput struct {
	BuyerID            int64
	RequestID          uuid.UUID
	Slot               model.SlotKey
	OptionID           string
	FreeText           string
	UnitOptionID       string
	Skip               bool
	ExpectedRevisionID uuid.UUID
	ClientRequestID    uuid.UUID
	Locale             uitext.Locale
}

const (
	fieldSlotKey      = "SlotKey"
	fieldUnitOptionID = "UnitOptionID"
	fieldSkip         = "Skip"
)

var answerable = []model.SlotKey{
	slots.Item, slots.Quantity, slots.Packaging, slots.BudgetPerUnit, slots.DeliveryCity, slots.DeliveryDate,
}

type answerPlan struct {
	slot               model.SlotKey
	skip               bool
	text, option, unit string
	number             *int64
	message            string
}

func planAnswer(in AnswerSlotInput) (answerPlan, error) {
	if !slices.Contains(answerable, in.Slot) {
		return answerPlan{}, apperr.InvalidInputOn([]string{fieldSlotKey}, "這個欄位不能用回答送出")
	}
	text := trimtext.Trim(in.FreeText)
	if utf8.RuneCountInString(text) > MaxFreeTextRunes {
		return answerPlan{}, apperr.InvalidInputOn([]string{fieldFreeText}, "回答的文字不能超過 %d 個字元", MaxFreeTextRunes)
	}
	p := answerPlan{slot: in.Slot, skip: in.Skip, text: text}
	card := planner.BuildFor(in.Slot, slots.Set{})
	var err error
	if in.Slot == slots.Quantity {
		err = p.planQuantity(in, card)
	} else {
		err = p.planSingle(in, card)
	}
	if err != nil {
		return answerPlan{}, err
	}
	if in.Slot == slots.Quantity || in.Slot == slots.BudgetPerUnit {
		if text != "" {
			n, err := strconv.ParseInt(text, 10, 64)
			if err != nil {
				return answerPlan{}, apperr.InvalidInputOn([]string{fieldFreeText}, "要輸入整數")
			}
			p.number = &n
		}
	}
	p.message = p.messageText()
	return p, nil
}

func (p *answerPlan) planQuantity(in AnswerSlotInput, card *planner.Question) error {
	switch {
	case in.OptionID != "":
		return apperr.InvalidInputOn([]string{fieldOptionID}, "數量沒有選項，要輸入數字或選單位")
	case in.Skip:
		return apperr.InvalidInputOn([]string{fieldSkip}, "數量不能略過")
	case p.text == "" && in.FreeText != "" && in.UnitOptionID == "":
		return apperr.InvalidInputOn([]string{fieldFreeText}, "回答的文字不能是空白")
	case p.text == "" && in.UnitOptionID == "":
		return apperr.InvalidInputOn([]string{fieldFreeText, fieldUnitOptionID}, "要輸入數量，或選一個單位")
	}
	if in.UnitOptionID != "" {
		v, ok := optionValue(card.UnitOptions, in.UnitOptionID)
		if !ok {
			return apperr.InvalidInputOn([]string{fieldUnitOptionID}, "這個單位選項不在問題上")
		}
		p.unit = v
	}
	return nil
}

func (p *answerPlan) planSingle(in AnswerSlotInput, card *planner.Question) error {
	if in.UnitOptionID != "" {
		return apperr.InvalidInputOn([]string{fieldUnitOptionID}, "這個欄位沒有單位選項")
	}
	if p.text == "" && in.FreeText != "" && in.OptionID == "" && !in.Skip {
		return apperr.InvalidInputOn([]string{fieldFreeText}, "回答的文字不能是空白")
	}
	provided := 0
	for _, on := range []bool{in.OptionID != "", p.text != "", in.Skip} {
		if on {
			provided++
		}
	}
	if provided != 1 {
		return apperr.InvalidInputOn([]string{fieldOptionID, fieldFreeText, fieldSkip}, "要選一個選項、輸入文字，或略過，三者恰好一個")
	}
	switch {
	case in.Skip && !card.CanSkip:
		return apperr.InvalidInputOn([]string{fieldSkip}, "這個欄位不能略過")
	case in.OptionID != "":
		v, ok := optionValue(card.Options, in.OptionID)
		if !ok {
			return apperr.InvalidInputOn([]string{fieldOptionID}, "這個選項不在問題上")
		}
		p.option = v
	}
	return nil
}

func optionValue(opts []planner.Option, id string) (string, bool) {
	for _, o := range opts {
		if o.ID == id {
			return o.Value, true
		}
	}
	return "", false
}

func (p answerPlan) messageText() string {
	switch {
	case p.skip:
		return ""
	case p.slot == slots.Quantity:
		return trimtext.Trim(p.text + " " + p.unit)
	case p.option != "":
		return p.option
	}
	return p.text
}

func (p answerPlan) patch(current slots.Set, res *slots.Resolution) (slots.Patch, error) {
	var patch slots.Patch
	if p.skip {
		return patch, nil
	}
	value := p.text
	if p.option != "" {
		value = p.option
	}
	switch p.slot {
	case slots.Item:
		patch.Item, patch.Resolution = &value, res
	case slots.Packaging:
		patch.Packaging = &value
	case slots.DeliveryCity:
		patch.DeliveryCity = &value
	case slots.DeliveryDate:
		patch.DeliveryDate = &value
	case slots.BudgetPerUnit:
		patch.BudgetPerUnit = p.number
	case slots.Quantity:
		switch {
		case p.number != nil:
			patch.Quantity = p.number
		case current.Quantity == nil:
			return slots.Patch{}, apperr.InvalidInputOn([]string{fieldFreeText}, "要輸入數量")
		}
		switch {
		case p.unit != "":
			patch.Unit = &p.unit
		case current.Unit == nil:
			return slots.Patch{}, apperr.InvalidInputOn([]string{fieldUnitOptionID}, "要選一個單位")
		}
	}
	return patch, nil
}

func answerDecision(l uitext.Locale, now time.Time, prior priorState, p answerPlan, res *slots.Resolution) (decision, error) {
	patch, err := p.patch(prior.current, res)
	if err != nil {
		return decision{}, err
	}
	next, rejected := prior.current.Apply(patch, now)
	if p.skip {
		pt := prevTurn{}
		if prior.prevTurn != nil {
			pt = *prior.prevTurn
		}
		if !slices.Contains(pt.skipped, p.slot) {
			pt.skipped = append(slices.Clone(pt.skipped), p.slot)
		}
		prior.prevTurn = &pt
	}
	return finish(l, prior, patch, next, rejected, nil), nil
}
