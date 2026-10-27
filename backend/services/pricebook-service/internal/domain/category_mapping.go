package domain

import (
	"context"
	"time"
)

// CategoryMappingStatus 是 category_crop_mapping.status 的服務內部型別化常數，
// 對外 JSON／proto 一律用這三個小寫字串，不新增第四個值不跟 usecase 層先確認。
type CategoryMappingStatus string

const (
	CategoryMappingResolved  CategoryMappingStatus = "resolved"
	CategoryMappingUnmapped  CategoryMappingStatus = "unmapped"
	CategoryMappingAmbiguous CategoryMappingStatus = "ambiguous"
)

type CategoryMapping struct {
	CategoryID  string
	Status      CategoryMappingStatus
	GroupCode   *string
	CommodityID *int64
	// 填入，不是 category_crop_mapping 表的欄位。
	Label          *string
	MappingVersion string
	SourceNote     *string
	UpdatedBy      int64
	UpdatedAt      time.Time
	CropVarieties  []CropVariety
}

// CategoryMappingRepository 是 category_crop_mapping 表 + crop/crop_group
// active 驗證查詢的窄 interface（issue #2786）。
type CategoryMappingRepository interface {
	// Get 回傳 categoryId 對應的既有列；查無列回 (nil, nil)（不是 error）——
	// 呼叫端（usecase）自己決定查無列時的預設行為（resolve API 視為
	// unmapped，見設計書 §4），repository 層不預設業務語意。
	Get(ctx context.Context, categoryID string) (*CategoryMapping, error)
	// Upsert 寫入或覆蓋一列，UpdatedAt 由 DB now() 填，回傳寫入後的完整列。
	Upsert(ctx context.Context, m CategoryMapping) (*CategoryMapping, error)
	// ListByStatuses 回傳 status 落在 statuses 裡的既有列（待處理清單用，
	// 不含完全查無列的 categoryId——那部分由 usecase 層跟 category service
	// 的分類樹做差集，見 Task 9）。
	ListByStatuses(ctx context.Context, statuses []CategoryMappingStatus) ([]CategoryMapping, error)
	// ListAllCategoryIDs 回傳表裡所有 categoryId（待處理清單的差集運算用）。
	ListAllCategoryIDs(ctx context.Context) ([]string, error)
	// ActiveGroupName 回傳 groupCode 目前是否 active，active 時附帶名稱（供
	// resolve API 的 label 欄位）。查無或已停用回 (nil, nil)，不是 error。
	ActiveGroupName(ctx context.Context, groupCode string) (*string, error)
	ActiveCommodityName(ctx context.Context, commodityID int64) (*string, error)
	// ListResolvedCategoryGroupMappings returns every resolved (category_id,
	// group_code) pair — the daily rollup batch's L3 input (Task 7).
	ListResolvedCategoryGroupMappings(ctx context.Context) ([]CategoryGroupMapping, error)
	ListResolvedCategoryCommodityMappings(ctx context.Context) ([]CategoryCommodityMapping, error)
	ListResolvedCategoryCropVarietyMappings(ctx context.Context) ([]CategoryCropVarietyMapping, error)
	ListCategoryCropVarieties(ctx context.Context, categoryID string) ([]CropVariety, error)
}

// WorklistItem 是待處理清單的一列——可能來自既有 category_crop_mapping 列
// （status unmapped/ambiguous），也可能是分類樹裡完全查無列的 categoryId
// （UpdatedBy/UpdatedAt 皆為 nil，見 ListCategoryMappingWorklist usecase）。
type WorklistItem struct {
	CategoryID   string
	CategoryName string
	Status       CategoryMappingStatus
	UpdatedBy    *int64
	UpdatedAt    *time.Time
}

// CategoryGroupMapping is one resolved (category_id, group_code) pair.
type CategoryGroupMapping struct {
	CategoryID string
	GroupCode  string
}

type CategoryCommodityMapping struct {
	CategoryID  string
	CommodityID int64
}

type CategoryCropVarietyMapping struct {
	CategoryID string
	CropCodes  []string
}

type CropVariety struct {
	CropCode    string
	Name        string
	VariantName *string
}
