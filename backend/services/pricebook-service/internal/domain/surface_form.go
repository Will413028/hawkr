package domain

import "context"

// 可定價實體的表面形式索引（priceable_surface_form）。crop / crop_group / commodity
// 的正式名、品種名、來源術語、別名全部住同一張表；正規化是 generated column，
// 寫入端只寫 form_raw。

type EntityKind string

const (
	EntityKindCropGroup EntityKind = "CROP_GROUP"
	EntityKindCrop      EntityKind = "CROP"
	EntityKindCommodity EntityKind = "COMMODITY"
)

type FormKind string

const (
	FormKindCanonical  FormKind = "CANONICAL"
	FormKindSourceTerm FormKind = "SOURCE_TERM"
	FormKindVariant    FormKind = "VARIANT"
	FormKindAlias      FormKind = "ALIAS"
)

// FormKindRank 是 form_kind 的權威度（越小越權威）。寫入順序（讓 ON CONFLICT DO
// NOTHING 保住最權威的 kind）與跨查詢變體的去重都用它，全 repo 只有這一份。
func FormKindRank(k FormKind) int {
	switch k {
	case FormKindCanonical:
		return 0
	case FormKindSourceTerm:
		return 1
	case FormKindVariant:
		return 2
	default:
		return 3
	}
}

// SurfaceFormHit 是一列搜尋命中。Tier 0 精確／1 前綴／2 子串。展示欄位依
// EntityKind 只有一組有值：CROP_GROUP 用 GroupCode/GroupName；CROP 另有
// CropCode/CropName/VariantName；COMMODITY 用 CommodityName。
type SurfaceFormHit struct {
	EntityKind    EntityKind
	EntityCode    string
	FormRaw       string
	FormKind      FormKind
	Tier          int32
	GroupCode     string
	GroupName     string
	CropCode      string
	CropName      string
	VariantName   string
	CommodityName string
}

// EntityKey 是 (kind, code) 的字串鍵，去重與 CategoryIDs 的 map 都用它。
func (h SurfaceFormHit) EntityKey() string {
	return string(h.EntityKind) + "|" + h.EntityCode
}

type SurfaceFormRegistration struct {
	EntityKind EntityKind
	EntityCode string
	FormRaw    string
	FormKind   FormKind
}

// SurfaceFormRepository 只有讀。寫入必須與 ingest 的 upsert 在同一個交易裡，
// 所以是 postgres 套件的匯出函式 RegisterSurfaceForms，不在介面上。
type SurfaceFormRepository interface {
	Search(ctx context.Context, query string, limit int32) ([]SurfaceFormHit, error)
	ListAliases(ctx context.Context, limit int32) ([]SurfaceFormHit, error)
	// CategoryIDs 回每個實體映射到的分類 id（status = resolved），key 為
	// SurfaceFormHit.EntityKey()；查無映射的實體不在 map 裡。
	CategoryIDs(ctx context.Context, hits []SurfaceFormHit) (map[string][]string, error)
}
