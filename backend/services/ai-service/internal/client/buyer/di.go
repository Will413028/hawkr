package buyer

import "go.uber.org/fx"

// Module provides the buyer-service gRPC client as buyer.Client.
var Module = fx.Module("buyer_client",
	fx.Provide(
		fx.Annotate(
			NewClient,
			fx.As(new(Client)),
		),
	),
)
