package product

import (
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"go.uber.org/fx"
)

var Module = fx.Module("product_client",
	fx.Provide(
		fx.Annotate(
			NewProductClient,
			fx.As(new(usecase.ProductClient)),
		),
	),
)
