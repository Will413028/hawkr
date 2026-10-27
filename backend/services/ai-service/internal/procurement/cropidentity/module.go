package cropidentity

import (
	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
	"go.uber.org/fx"
)

var Module = fx.Module("procurement-cropidentity",
	fx.Provide(func(pricebook pbusecase.PricebookClient, categories *cache.CategoryIndex, provider llmprovider.Procurement) *DisambiguationUseCase {
		return NewDisambiguationUseCase(pricebook, categories, newCropCategorySuggester(provider))
	}),
)

func NewResolverForEvaluation(pricebook pbusecase.PricebookClient, categories *cache.CategoryIndex, provider llmprovider.Procurement) Resolver {
	return NewDisambiguationUseCase(pricebook, categories, newCropCategorySuggester(provider))
}
