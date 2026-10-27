package domain

import (
	"fmt"
	"strconv"
	"strings"
)

type ItemResolutionKind string

const (
	ItemResolvedCrop ItemResolutionKind = "CROP"

	ItemResolvedGroup     ItemResolutionKind = "GROUP"
	ItemResolvedCommodity ItemResolutionKind = "COMMODITY"

	ItemResolvedSelfPriced ItemResolutionKind = "SELF_PRICED"
)

type ItemResolutionProvenance string

const (
	ItemResolutionBySystem ItemResolutionProvenance = "SYSTEM"

	ItemResolutionByBuyerChoice ItemResolutionProvenance = "BUYER_CHOICE"
)

type ItemResolution struct {
	Kind ItemResolutionKind `json:"kind"`

	Code *string `json:"code,omitempty"`

	Label string `json:"label,omitempty"`

	CategoryID string `json:"category_id,omitempty"`

	FallbackGroupCode *string `json:"fallback_group_code,omitempty"`

	PricePerKg *int `json:"price_per_kg,omitempty"`

	Provenance ItemResolutionProvenance `json:"provenance"`
}

func (r ItemResolution) DisplayValue() string {
	if r.Kind == ItemResolvedSelfPriced {
		label := "自訂品項"
		if r.CategoryID != "" && strings.TrimSpace(r.Label) != "" {
			label = strings.TrimSpace(r.Label)
		}
		if r.PricePerKg != nil {
			return label + "，單價 " + strconv.Itoa(*r.PricePerKg) + " 元／公斤"
		}
		return label
	}
	label := strings.TrimSpace(r.Label)
	if label == "" && r.Code != nil {
		label = *r.Code
	}
	if r.Kind == ItemResolvedGroup {
		return label + "（整個大類）"
	}
	return label
}

func (r *ItemResolution) Equal(o *ItemResolution) bool {
	if r == nil || o == nil {
		return r == o
	}
	return r.Kind == o.Kind &&
		stringPtrEqual(r.Code, o.Code) &&
		r.Label == o.Label &&
		r.CategoryID == o.CategoryID &&
		stringPtrEqual(r.FallbackGroupCode, o.FallbackGroupCode) &&
		intPtrEqual(r.PricePerKg, o.PricePerKg) &&
		r.Provenance == o.Provenance
}

func validateItemResolution(r ItemResolution) string {
	switch r.Kind {
	case ItemResolvedCrop, ItemResolvedGroup, ItemResolvedCommodity:
		if r.Code == nil || strings.TrimSpace(*r.Code) == "" {
			return string(r.Kind) + " 解析缺少分類代碼"
		}
		if r.PricePerKg != nil {
			return "只有自填價品項可以帶單價"
		}
	case ItemResolvedSelfPriced:
		if r.Code != nil {
			return "自填價品項不屬於任何分類代碼"
		}
		if r.PricePerKg == nil {
			return "自填價品項必須帶單價"
		}
		if *r.PricePerKg < 1 || *r.PricePerKg > MaxPricePerKg {
			return fmt.Sprintf("單價需介於 1 與 %d 元之間", MaxPricePerKg)
		}
	default:
		return "未知的品項解析類型"
	}
	if r.Provenance != ItemResolutionBySystem && r.Provenance != ItemResolutionByBuyerChoice {
		return "未知的品項解析來源"
	}
	return ""
}

func NewSystemItemResolution(cropCode, groupCode, commodityCode, fallbackGroupCode *string, label string, origin CategoryOrigin) *ItemResolution {
	if !origin.Declared() {
		return nil
	}
	categoryID := origin.ID()
	switch {
	case cropCode != nil:
		return &ItemResolution{
			Kind: ItemResolvedCrop, Code: cropCode, Label: label, CategoryID: categoryID,
			FallbackGroupCode: fallbackGroupCode, Provenance: ItemResolutionBySystem,
		}
	case groupCode != nil:
		return &ItemResolution{
			Kind: ItemResolvedGroup, Code: groupCode, Label: label, CategoryID: categoryID,
			FallbackGroupCode: fallbackGroupCode, Provenance: ItemResolutionBySystem,
		}
	case commodityCode != nil:
		return &ItemResolution{
			Kind: ItemResolvedCommodity, Code: commodityCode, Label: label, CategoryID: categoryID,
			Provenance: ItemResolutionBySystem,
		}
	default:
		return nil
	}
}

func NewBuyerChoiceItemResolution(opt CropDisambiguationOption) *ItemResolution {
	label := opt.ResolvedLabel
	if label == "" {
		label = opt.Label
	}
	r := NewSystemItemResolution(opt.CropCode, opt.GroupCode, nil, opt.FallbackGroupCode, label,
		CategoryFromEntity(opt.EntityCategoryID))
	if r == nil {
		return nil
	}
	r.Provenance = ItemResolutionByBuyerChoice
	return r
}

func NewSelfPricedItemResolution(pricePerKg int, label string, origin CategoryOrigin) *ItemResolution {
	if !origin.Declared() {
		return nil
	}
	return &ItemResolution{
		Kind: ItemResolvedSelfPriced, PricePerKg: &pricePerKg, Label: label,
		CategoryID: origin.ID(), Provenance: ItemResolutionByBuyerChoice,
	}
}

func (r *ItemResolution) PricebookQueryCodes() (cropCode, groupCode *string) {
	if r == nil {
		return nil, nil
	}
	switch r.Kind {
	case ItemResolvedCrop:
		return r.Code, nil
	case ItemResolvedGroup:
		return nil, r.Code
	default:
		return nil, nil
	}
}

func (r *ItemResolution) PricebookCommodityCode() *string {
	if r == nil || r.Kind != ItemResolvedCommodity {
		return nil
	}
	return r.Code
}

func (r *ItemResolution) QuoteEligible() bool {
	if r == nil {
		return false
	}
	return r.Kind == ItemResolvedCrop || r.Kind == ItemResolvedGroup ||
		r.Kind == ItemResolvedCommodity || r.Kind == ItemResolvedSelfPriced
}

func (r *ItemResolution) IsSelfPriced() bool {
	return r != nil && r.Kind == ItemResolvedSelfPriced
}
