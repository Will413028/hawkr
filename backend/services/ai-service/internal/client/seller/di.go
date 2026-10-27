package seller

import (
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"go.uber.org/fx"
)

var Module = fx.Module("seller_client",
	fx.Provide(
		fx.Annotate(
			NewSellerClient,
			fx.As(new(usecase.SellerClient)),
		),
	),
)
