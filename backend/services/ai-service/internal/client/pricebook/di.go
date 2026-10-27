package pricebook

import (
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"go.uber.org/fx"
)

// Module provides the pricebook-service gRPC client as usecase.PricebookClient.
var Module = fx.Module("pricebook_client",
	fx.Provide(
		fx.Annotate(
			NewPricebookClient,
			fx.As(new(usecase.PricebookClient)),
		),
	),
)
