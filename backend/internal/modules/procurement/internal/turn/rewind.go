package turn

import (
	"slices"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// RewindInput 是 Rewind 的輸入（A9-D31、A9-D33(3)(4)）。UNIT 視同 QUANTITY（單位折進數量這一格）。
type RewindInput struct {
	BuyerID            int64
	RequestID          uuid.UUID
	Slot               model.SlotKey
	ExpectedRevisionID uuid.UUID
	ClientRequestID    uuid.UUID
	Locale             uitext.Locale
}

// rewindSlot 回倒回的欄位（UNIT 折進 QUANTITY）；不是可倒回的欄位回 INVALID_ARGUMENT（slot_key）。
func rewindSlot(k model.SlotKey) (model.SlotKey, error) {
	if k == slots.Unit {
		k = slots.Quantity
	}
	if !slices.Contains(answerable, k) {
		return "", apperr.InvalidInputOn([]string{fieldSlotKey}, "這個欄位不能倒回")
	}
	return k, nil
}

func errNothingToRewind() error {
	return apperr.FailedPrecondition("這個欄位沒有可以倒回的內容")
}

// rewindTarget 沿內容祖先鏈從現行版本往上找倒回的目標版（A9-D33(3)）：最近一個改動了這一格的版本的祖先。
// 現況這一格沒有值，或改動它的版本沒有祖先（第一個寫入）時沒有東西可倒回（FAILED_PRECONDITION，沒有 reason）。
// 倒回產生的新版本內容同目標版、祖先是目標版，所以再倒回同一格會沿鏈繼續往前退，不會在兩個值之間來回。
func rewindTarget(revs []store.ProcurementRequestRevision, active store.ProcurementRequestRevision, slot model.SlotKey) (*store.ProcurementRequestRevision, error) {
	if !slotHasValue(slot, setFromRevision(active)) {
		return nil, errNothingToRewind()
	}
	byID := make(map[uuid.UUID]*store.ProcurementRequestRevision, len(revs))
	for i := range revs {
		byID[revs[i].ID] = &revs[i]
	}
	cur := &active
	for cur != nil {
		var parent *store.ProcurementRequestRevision
		var before slots.Set
		if cur.ParentRevisionID != nil {
			parent = byID[*cur.ParentRevisionID]
		}
		if parent != nil {
			before = setFromRevision(*parent)
		}
		if !sameSlot(slot, before, setFromRevision(*cur)) {
			if parent == nil {
				return nil, errNothingToRewind()
			}
			return parent, nil
		}
		cur = parent
	}
	return nil, errNothingToRewind()
}

func slotHasValue(k model.SlotKey, s slots.Set) bool {
	switch k {
	case slots.Item:
		return s.Item != nil
	case slots.Quantity:
		return s.Quantity != nil || s.Unit != nil
	case slots.Packaging:
		return s.Packaging != nil
	case slots.BudgetPerUnit:
		return s.BudgetPerUnit != nil
	case slots.DeliveryCity:
		return s.DeliveryCity != nil
	case slots.DeliveryDate:
		return s.DeliveryDate != nil
	}
	return false
}

// sameSlot 比較兩個內容在這一格的值。品項只比原詞（解析由伺服器補上，不算買家改動這一格）；數量連單位一起比；到貨日比日曆日。
func sameSlot(k model.SlotKey, a, b slots.Set) bool {
	switch k {
	case slots.Item:
		return samePtr(a.Item, b.Item)
	case slots.Quantity:
		return samePtr(a.Quantity, b.Quantity) && samePtr(a.Unit, b.Unit)
	case slots.Packaging:
		return samePtr(a.Packaging, b.Packaging)
	case slots.BudgetPerUnit:
		return samePtr(a.BudgetPerUnit, b.BudgetPerUnit)
	case slots.DeliveryCity:
		return samePtr(a.DeliveryCity, b.DeliveryCity)
	case slots.DeliveryDate:
		return samePtr(day(a.DeliveryDate), day(b.DeliveryDate))
	}
	return true
}

func samePtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// rewindDecision 把倒回接成這一輪的 decision：沒有 I/O。內容是目標版的內容，問題是倒回那一格的問題（planner.BuildFor 以目標內容決定型態），
// 連續選填計數歸零，倒回的欄位不再算被略過。
func rewindDecision(l uitext.Locale, prior priorState, next slots.Set, slot model.SlotKey) decision {
	d := render(l, prior, slots.Patch{}, next, nil, nil, planner.BuildFor(slot, next), false)
	d.plan.OptionalStreak = 0
	d.plan.Skipped = slices.DeleteFunc(slices.Clone(d.plan.Skipped), func(k model.SlotKey) bool { return k == slot })
	return d
}
