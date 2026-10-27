package cropidentity

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
)

const searchCropCatalogLimit = 10

type DisambiguationResult struct {
	NeedsDisambiguation bool
	CropCode            *string
	GroupCode           *string
	CommodityCode       *string

	FallbackGroupCode *string

	Label       string
	Candidates  []domain.CropDisambiguationOption
	Unavailable bool

	Selection *domain.CropSelection `json:",omitempty"`

	CategoryOrigin domain.CategoryOrigin `json:"-"`
}

type DisambiguationUseCase struct {
	pricebook       pbusecase.PricebookClient
	categories      cropCategoryReader
	categoryModel   cropCategorySuggester
	categoryMapping cropCategoryMappingReader
}

func NewDisambiguationUseCase(pricebook pbusecase.PricebookClient, categories cropCategoryReader, model cropCategorySuggester) *DisambiguationUseCase {
	mapping, _ := pricebook.(cropCategoryMappingReader)
	return &DisambiguationUseCase{pricebook: pricebook, categories: categories, categoryModel: model, categoryMapping: mapping}
}

func (uc *DisambiguationUseCase) drillDownGroup(ctx context.Context, groupCode, groupName, entityCategoryID string) ([]domain.CropDisambiguationOption, error) {
	crops, err := uc.pricebook.ListGroupCropOptions(ctx, groupCode, searchCropCatalogLimit)
	if err != nil {
		return nil, err
	}

	code := groupCode
	options := make([]domain.CropDisambiguationOption, 0, len(crops)+1)
	for _, c := range crops {

		if !c.HasForecast || c.CropCode == nil {
			continue
		}
		options = append(options, domain.CropDisambiguationOption{
			Label: cropOptionLabel(c), CropCode: c.CropCode, FallbackGroupCode: &code,
			EntityCategoryID: entityCategoryID,
		})
	}
	if len(options) < 2 {
		return nil, nil
	}

	return append(options, domain.CropDisambiguationOption{
		Label: "整個" + groupName + "都可以", ResolvedLabel: groupName,
		GroupCode: &code, FallbackGroupCode: &code,
		EntityCategoryID: entityCategoryID,
	}), nil
}

func cropOptionLabel(c pbusecase.CropCatalogMatch) string {
	switch {
	case c.CropName != nil && *c.CropName != "":
		return *c.CropName
	case c.VariantName != nil && *c.VariantName != "":
		return *c.VariantName
	case c.CropCode != nil:
		return *c.CropCode
	}
	return ""
}
