package slots

import "github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"

// FromRevision 把需求版本的列轉成 Set。品項解析的 kind、label 與分類 ID 一起存在或一起沒有（資料表的 CHECK）。
func FromRevision(r store.ProcurementRequestRevision) Set {
	s := Set{
		Item: r.Item, Quantity: r.Quantity, Unit: r.Unit, Packaging: r.Packaging, BudgetPerUnit: r.BudgetPerUnit,
		DeliveryCity: r.DeliveryCity, DeliveryDate: r.DeliveryDate, ExactMatchRequired: r.ExactMatchRequired,
	}
	if r.ItemResolutionKind != nil && r.ItemResolutionLabel != nil {
		res := Resolution{Kind: ItemResolutionKind(*r.ItemResolutionKind), Label: *r.ItemResolutionLabel}
		if r.ItemResolutionCategoryID != nil {
			res.CategoryID = *r.ItemResolutionCategoryID
		}
		s.Resolution = &res
	}
	return s
}
