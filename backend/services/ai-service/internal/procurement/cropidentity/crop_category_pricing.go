package cropidentity

import (
	"context"
	"strings"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
)

type cropCategoryMappingReader interface {
	ResolveCategoryMapping(context.Context, string) (pbusecase.PricebookCategoryMapping, error)
}

func (uc *DisambiguationUseCase) priceSelectedCategory(ctx context.Context, selection *domain.CropSelection) (DisambiguationResult, error) {
	if selection == nil || selection.CategoryID == "" || uc.categoryMapping == nil {
		return categoryPrice(selection), nil
	}
	mapping, err := uc.categoryMapping.ResolveCategoryMapping(ctx, selection.CategoryID)
	if err != nil {
		return DisambiguationResult{}, err
	}

	if mapping.CategoryID != selection.CategoryID || mapping.Status != "resolved" ||
		mapping.Label == nil || strings.TrimSpace(*mapping.Label) == "" {
		return categoryPrice(selection), nil
	}

	if code := trimmedPtr(mapping.CommodityCode); code != nil {
		return DisambiguationResult{
			CommodityCode: code, Label: *mapping.Label, Selection: selection,
		}, nil
	}

	groupCodePtr := trimmedPtr(mapping.GroupCode)
	if groupCodePtr == nil {
		return categoryPrice(selection), nil
	}
	groupCode := *groupCodePtr

	matches, err := uc.pricebook.SearchPriceableEntities(ctx, []string{*mapping.Label}, recall.DefaultEntityLimit)
	if err != nil {
		return DisambiguationResult{}, err
	}
	forecastable := false
	for _, m := range matches {
		if m.Kind == string(recall.KindCropGroup) && m.Code == groupCode && m.HasForecast {
			forecastable = true
			break
		}
	}
	if !forecastable {
		return categoryPrice(selection), nil
	}
	if len(mapping.CropVarieties) == 1 {
		v := mapping.CropVarieties[0]
		label := v.Label
		if strings.TrimSpace(label) == "" {
			label = *mapping.Label
		}
		return DisambiguationResult{
			CropCode: &v.CropCode, FallbackGroupCode: &groupCode,
			Label: label, Selection: selection,
		}, nil
	}
	return DisambiguationResult{
		GroupCode: &groupCode, FallbackGroupCode: &groupCode,
		Label: *mapping.Label, Selection: selection,
	}, nil
}

func trimmedPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}
