package domain

import (
	"fmt"
	"strconv"
	"strings"
)

type ItemResolutionKind string

const ItemResolvedSelfPriced ItemResolutionKind = "SELF_PRICED"

type ItemResolutionProvenance string

const (
	ItemResolutionBySystem ItemResolutionProvenance = "SYSTEM"

	ItemResolutionByBuyerChoice ItemResolutionProvenance = "BUYER_CHOICE"
)

type ItemResolution struct {
	Kind ItemResolutionKind `json:"kind"`

	Label string `json:"label,omitempty"`

	CategoryID string `json:"category_id,omitempty"`

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
	return strings.TrimSpace(r.Label)
}

func (r *ItemResolution) Equal(o *ItemResolution) bool {
	if r == nil || o == nil {
		return r == o
	}
	return r.Kind == o.Kind &&
		r.Label == o.Label &&
		r.CategoryID == o.CategoryID &&
		intPtrEqual(r.PricePerKg, o.PricePerKg) &&
		r.Provenance == o.Provenance
}

func validateItemResolution(r ItemResolution) string {
	switch r.Kind {
	case ItemResolvedSelfPriced:
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

func NewSelfPricedItemResolution(pricePerKg int, label string, origin CategoryOrigin) *ItemResolution {
	if !origin.Declared() {
		return nil
	}
	return &ItemResolution{
		Kind: ItemResolvedSelfPriced, PricePerKg: &pricePerKg, Label: label,
		CategoryID: origin.ID(), Provenance: ItemResolutionByBuyerChoice,
	}
}

func (r *ItemResolution) IsSelfPriced() bool {
	return r != nil && r.Kind == ItemResolvedSelfPriced
}
