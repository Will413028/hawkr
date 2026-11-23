package reply

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// SlotLabel 是欄位名稱；不認得的欄位（不該發生）退回欄位代碼，不讓買家看到空字串。
func SlotLabel(l uitext.Locale, key model.SlotKey) string {
	switch key {
	case slots.Item:
		return text.SlotItem(l).Text
	case slots.ItemResolution:
		return text.SlotItemResolution(l).Text
	case slots.Quantity:
		return text.SlotQuantity(l).Text
	case slots.Unit:
		return text.SlotUnit(l).Text
	case slots.Packaging:
		return text.SlotPackaging(l).Text
	case slots.BudgetPerUnit:
		return text.SlotBudgetPerUnit(l).Text
	case slots.DeliveryCity:
		return text.SlotDeliveryCity(l).Text
	case slots.DeliveryDate:
		return text.SlotDeliveryDate(l).Text
	case slots.ExactMatchRequired:
		return text.SlotExactMatchRequired(l).Text
	}
	return string(key)
}

// RecordedValue 是本輪記下的欄位值的顯示字串；是否欄位依語系轉成「是／否」，單位代碼換成該語系的單位名稱。
func RecordedValue(l uitext.Locale, r slots.Recorded) string {
	switch r.Key {
	case slots.ExactMatchRequired:
		if r.Value == "true" {
			return text.ValueYes(l).Text
		}
		return text.ValueNo(l).Text
	case slots.Unit:
		return unitLabel(l, r.Value)
	}
	return r.Value
}

// unitLabel 是單位代碼的顯示名稱；不認得的代碼原樣回傳，不讓買家看到空字串。
func unitLabel(l uitext.Locale, code string) string {
	switch code {
	case "KG":
		return text.UnitKg(l).Text
	case "CATTY":
		return text.UnitCatty(l).Text
	case "GRAM":
		return text.UnitGram(l).Text
	}
	return code
}

func allowedUnits(l uitext.Locale) string {
	labels := make([]string, 0, len(slots.AllowedUnits))
	for _, u := range slots.AllowedUnits {
		labels = append(labels, unitLabel(l, u))
	}
	return join(l, labels)
}

// RejectionReason 是欄位被拒或被修正的原因文字。
func RejectionReason(l uitext.Locale, r slots.Rejection) string {
	switch r.Reason {
	case slots.ReasonEmpty:
		return text.RejectEmpty(l).Text
	case slots.ReasonTruncated:
		return text.RejectTruncated(l, text.RejectTruncatedArgs{Max: slots.MaxTextLen}).Text
	case slots.ReasonQuantityRange:
		return text.RejectQuantityRange(l, text.RejectQuantityRangeArgs{Max: slots.MaxQuantity}).Text
	case slots.ReasonBudgetRange:
		return text.RejectBudgetRange(l, text.RejectBudgetRangeArgs{Max: slots.MaxBudgetPerUnit}).Text
	case slots.ReasonUnitNotAllowed:
		return text.RejectUnitNotAllowed(l, text.RejectUnitNotAllowedArgs{Allowed: allowedUnits(l)}).Text
	case slots.ReasonCityUnrecognized:
		return text.RejectCityUnrecognized(l).Text
	case slots.ReasonDateFormat:
		return text.RejectDateFormat(l).Text
	case slots.ReasonDateMissingYear:
		return text.RejectDateMissingYear(l).Text
	case slots.ReasonDatePast:
		return text.RejectDatePast(l).Text
	case slots.ReasonDateTooFar:
		return text.RejectDateTooFar(l, text.RejectDateTooFarArgs{Max: slots.MaxLeadDays}).Text
	case slots.ReasonResolutionNoItem:
		return text.RejectResolutionNoItem(l).Text
	case slots.ReasonResolutionNoLabel:
		return text.RejectResolutionNoLabel(l).Text
	case slots.ReasonResolutionUnknownKind:
		return text.RejectResolutionUnknownKind(l).Text
	}
	return string(r.Reason)
}

// RenderedOption 是問題卡上的一個選項，只有 ID 與顯示文字（Value 不出伺服器）。
type RenderedOption struct {
	ID    string
	Label string
}

// RenderedQuestion 是依語系組好的問題卡。
type RenderedQuestion struct {
	Slot          model.SlotKey
	Prompt        string
	Options       []RenderedOption
	UnitOptions   []RenderedOption
	AllowFreeText bool
	CanSkip       bool
}

// Question 把問題卡的代碼換成語系的文字。
func Question(l uitext.Locale, q *planner.Question) *RenderedQuestion {
	if q == nil {
		return nil
	}
	return &RenderedQuestion{
		Slot: q.Slot, Prompt: Prompt(l, q.Prompt),
		Options: options(l, q.Options), UnitOptions: options(l, q.UnitOptions),
		AllowFreeText: q.AllowFreeText, CanSkip: q.CanSkip,
	}
}

// Prompt 是問句文字。
func Prompt(l uitext.Locale, p planner.Prompt) string {
	switch p {
	case planner.PromptItem:
		return text.PromptItem(l).Text
	case planner.PromptDeliveryCity:
		return text.PromptDeliveryCity(l).Text
	case planner.PromptQuantity:
		return text.PromptQuantity(l).Text
	case planner.PromptUnit:
		return text.PromptUnit(l).Text
	case planner.PromptPackaging:
		return text.PromptPackaging(l).Text
	case planner.PromptBudgetPerUnit:
		return text.PromptBudgetPerUnit(l).Text
	case planner.PromptDeliveryDate:
		return text.PromptDeliveryDate(l).Text
	case planner.PromptItemChoices:
		return text.PromptItemChoices(l).Text
	case planner.PromptItemDescribe:
		return text.PromptItemDescribe(l).Text
	case planner.PromptItemGroupChoices:
		return text.PromptItemGroupChoices(l).Text
	}
	return string(p)
}

func options(l uitext.Locale, in []planner.Option) []RenderedOption {
	out := make([]RenderedOption, 0, len(in))
	for _, o := range in {
		out = append(out, RenderedOption{ID: o.ID, Label: optionLabel(l, o)})
	}
	return out
}

func optionLabel(l uitext.Locale, o planner.Option) string {
	switch o.Kind {
	case planner.OptionUnit:
		return unitLabel(l, o.Value)
	case planner.OptionOther:
		return text.OptionOther(l).Text
	case planner.OptionDescribe:
		return text.OptionDescribe(l).Text
	case planner.OptionValue:
		return o.Value
	}
	return o.Value
}

// CandidateFacts 由需求的現行版本組出候選的事實（讀取時組，不入庫，A9-D29(3)）：有品項就是符合「品項」；
// 預算、數量、縣市、交期有值的各是一項需向賣家確認的事。兩個回傳值都是非 nil 的 slice。
func CandidateFacts(l uitext.Locale, s slots.Set) ([]model.MatchedFact, []model.UnverifiableFact) {
	matched := []model.MatchedFact{}
	if s.Item != nil && trimtext.Trim(*s.Item) != "" {
		matched = append(matched, model.MatchedFact{SlotKey: slots.Item, Label: SlotLabel(l, slots.Item)})
	}
	unverifiable := []model.UnverifiableFact{}
	add := func(key model.SlotKey, reason string) {
		unverifiable = append(unverifiable, model.UnverifiableFact{SlotKey: key, Reason: reason})
	}
	if s.BudgetPerUnit != nil {
		if s.Unit != nil {
			add(slots.BudgetPerUnit, text.ConfirmBudgetUnit(l, text.ConfirmBudgetUnitArgs{Budget: int(*s.BudgetPerUnit), Unit: unitLabel(l, *s.Unit)}).Text)
		} else {
			add(slots.BudgetPerUnit, text.ConfirmBudget(l, text.ConfirmBudgetArgs{Budget: int(*s.BudgetPerUnit)}).Text)
		}
	}
	if s.Quantity != nil {
		if s.Unit != nil {
			add(slots.Quantity, text.ConfirmQuantityUnit(l, text.ConfirmQuantityUnitArgs{Quantity: int(*s.Quantity), Unit: unitLabel(l, *s.Unit)}).Text)
		} else {
			add(slots.Quantity, text.ConfirmQuantity(l, text.ConfirmQuantityArgs{Quantity: int(*s.Quantity)}).Text)
		}
	}
	if s.DeliveryCity != nil {
		add(slots.DeliveryCity, text.ConfirmDeliveryCity(l, text.ConfirmDeliveryCityArgs{City: *s.DeliveryCity}).Text)
	}
	if s.DeliveryDate != nil {
		add(slots.DeliveryDate, text.ConfirmDeliveryDate(l, text.ConfirmDeliveryDateArgs{Date: s.DeliveryDate.Format(time.DateOnly)}).Text)
	}
	return matched, unverifiable
}
