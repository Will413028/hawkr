package domain

import "strings"

type ItemResolutionKind string

const (
	ItemResolvedCategoryConfirmed ItemResolutionKind = "CATEGORY_CONFIRMED"

	ItemResolvedBuyerDescribed ItemResolutionKind = "BUYER_DESCRIBED"
)

type ItemResolution struct {
	Kind ItemResolutionKind `json:"kind"`

	Label string `json:"label,omitempty"`

	CategoryID string `json:"category_id,omitempty"`
}

func (r ItemResolution) DisplayValue() string {
	return strings.TrimSpace(r.Label)
}

func (r *ItemResolution) Equal(o *ItemResolution) bool {
	if r == nil || o == nil {
		return r == o
	}
	return *r == *o
}

func validateItemResolution(r ItemResolution) string {
	switch r.Kind {
	case ItemResolvedCategoryConfirmed:
		if strings.TrimSpace(r.CategoryID) == "" {
			return "已確認分類的品項必須帶分類 ID"
		}
	case ItemResolvedBuyerDescribed:
		if strings.TrimSpace(r.Label) == "" {
			return "買家自行描述的品項必須帶描述"
		}
		if r.CategoryID != "" {
			return "買家自行描述的品項不可帶分類 ID"
		}
	default:
		return "未知的品項解析類型"
	}
	return ""
}

func NewCategoryConfirmedItemResolution(sel *CropSelection) *ItemResolution {
	if sel == nil || sel.CategoryID == "" {
		return nil
	}
	return &ItemResolution{
		Kind: ItemResolvedCategoryConfirmed, Label: sel.Label,
		CategoryID: sel.CategoryID,
	}
}

// NewBuyerDescribedItemResolution 記下買家在「找不到符合的品項」卡上的自行描述。
// 買家已確認過上層分類時，改記成掛在該分類下的 CATEGORY_CONFIRMED。
func NewBuyerDescribedItemResolution(sel *CropSelection, description string) *ItemResolution {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil
	}
	if id, _ := sel.ConfirmedCategory(); id != "" {
		return &ItemResolution{
			Kind: ItemResolvedCategoryConfirmed, Label: description,
			CategoryID: id,
		}
	}
	return &ItemResolution{
		Kind: ItemResolvedBuyerDescribed, Label: description,
	}
}
