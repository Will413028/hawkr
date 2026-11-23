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

// 欄位違規指出的欄位名（AnswerSlotInput 的 Go 欄位名）。
const (
	fieldSlotKey      = "SlotKey"
	fieldUnitOptionID = "UnitOptionID"
	fieldSkip         = "Skip"
)

// answerable 是 SubmitAnswer 收的欄位：UNIT 折進 QUANTITY，品項消歧與品項解析由 run 或伺服器擁有。
var answerable = []model.SlotKey{
	slots.Item, slots.Quantity, slots.Packaging, slots.BudgetPerUnit, slots.DeliveryCity, slots.DeliveryDate,
}

// answerPlan 是形狀檢查過的回答：不讀資料庫。
type answerPlan struct {
	slot model.SlotKey
	skip bool
	// text 是去頭尾空白的文字；option、unit 是白名單選項的值；number 是 QUANTITY、BUDGET_PER_UNIT 的文字解成的整數。
	text, option, unit string
	number             *int64
	// message 是對話裡顯示的買家回答（輪次的 buyer_message）：選項的值或文字，數量帶單位，略過是空字串。
	message string
}

// planAnswer 檢查回答的形狀（slot 認得、option_id 在白名單上、數量與預算是整數、略過只給可略過的欄位），不合回 INVALID_ARGUMENT 並指出欄位。
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

// patch 是這個回答對現況的改動；QUANTITY 缺數量或缺單位時要現況已有，否則回 INVALID_ARGUMENT。res 只給 ITEM（唯一對到的末層分類）。
func (p answerPlan) patch(current slots.Set, res *slots.Resolution) (slots.Patch, error) {
	var patch slots.Patch
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

// answerDecision 把回答接成這一輪的 decision：沒有 I/O。略過把欄位記進上一輪帶來的 skipped，之後挑問題不再問它；
// 值被 slots 驗證拒絕不是錯誤，記在 rejected（TurnResult.rejected）。
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
		pt.skipped = append(slices.Clone(pt.skipped), p.slot)
		prior.prevTurn = &pt
	}
	return finish(l, prior, patch, next, rejected, nil), nil
}
