package usecase

// PricebookCategoryMapping is the existing ResolveCategoryMapping wire result.
type PricebookCategoryMapping struct {
	CategoryID     string
	Status         string // resolved | unmapped | ambiguous
	GroupCode      *string
	CommodityCode  *string
	Label          *string
	MappingVersion string
	CropVarieties  []PricebookCropVariety
}
type PricebookCropVariety struct {
	CropCode string
	Label    string
}
