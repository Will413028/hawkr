package cropidentity

import (
	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"go.uber.org/fx"
)

var Module = fx.Module("procurement-cropidentity",
	fx.Provide(func(categories *cache.CategoryIndex, provider llmprovider.Procurement) *DisambiguationUseCase {
		return NewDisambiguationUseCase(categories, newCropCategorySuggester(provider))
	}),
)

func NewResolverForEvaluation(categories *cache.CategoryIndex, provider llmprovider.Procurement) Resolver {
	return NewDisambiguationUseCase(categories, newCropCategorySuggester(provider))
}
