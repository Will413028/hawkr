package media

import "go.uber.org/fx"

// Module provides the media gRPC client (exposed as API for the handler layer).
var Module = fx.Module("media_client",
	fx.Provide(fx.Annotate(NewClient, fx.As(new(API)))),
)
