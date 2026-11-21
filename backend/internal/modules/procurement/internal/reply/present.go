package reply

import (
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/text"
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

// RecordedValue 是本輪記下的欄位值的顯示字串；是否欄位依語系轉成「是／否」。
func RecordedValue(l uitext.Locale, r slots.Recorded) string {
	if r.Key == slots.ExactMatchRequired {
		if r.Value == "true" {
			return text.ValueYes(l).Text
		}
		return text.ValueNo(l).Text
	}
	return r.Value
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
		return text.RejectUnitNotAllowed(l, text.RejectUnitNotAllowedArgs{Allowed: strings.Join(slots.AllowedUnits, "/")}).Text
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
		switch o.Value {
		case "KG":
			return text.UnitKg(l).Text
		case "CATTY":
			return text.UnitCatty(l).Text
		case "GRAM":
			return text.UnitGram(l).Text
		}
	case planner.OptionOther:
		return text.OptionOther(l).Text
	case planner.OptionDescribe:
		return text.OptionDescribe(l).Text
	case planner.OptionValue:
		return o.Value
	}
	return o.Value
}
