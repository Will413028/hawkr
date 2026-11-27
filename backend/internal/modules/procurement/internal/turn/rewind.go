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

func rewindDecision(l uitext.Locale, prior priorState, next slots.Set, slot model.SlotKey) decision {
	d := render(l, prior, slots.Patch{}, next, nil, nil, planner.BuildFor(slot, next), false)
	d.plan.OptionalStreak = 0
	d.plan.Skipped = slices.DeleteFunc(slices.Clone(d.plan.Skipped), func(k model.SlotKey) bool { return k == slot })
	return d
}
