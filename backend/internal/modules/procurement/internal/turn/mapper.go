package turn

import (
	"reflect"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

func setFromRevision(r store.ProcurementRequestRevision) slots.Set { return slots.FromRevision(r) }

func revisionParams(s slots.Set, p store.InsertRevisionParams) store.InsertRevisionParams {
	p.Item, p.Quantity, p.Unit, p.Packaging, p.BudgetPerUnit = s.Item, s.Quantity, s.Unit, s.Packaging, s.BudgetPerUnit
	p.DeliveryCity, p.DeliveryDate, p.ExactMatchRequired = s.DeliveryCity, s.DeliveryDate, s.ExactMatchRequired
	p.ItemResolutionKind, p.ItemResolutionLabel, p.ItemResolutionCategoryID = nil, nil, nil
	if s.Resolution != nil {
		kind, label := string(s.Resolution.Kind), s.Resolution.Label
		p.ItemResolutionKind, p.ItemResolutionLabel = &kind, &label
		if s.Resolution.CategoryID != uuid.Nil() {
			id := s.Resolution.CategoryID
			p.ItemResolutionCategoryID = &id
		}
	}
	return p
}

func sameContent(a, b slots.Set) bool {
	a.DeliveryDate, b.DeliveryDate = day(a.DeliveryDate), day(b.DeliveryDate)
	return reflect.DeepEqual(a, b)
}

func day(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return &d
}
