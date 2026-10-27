package client

import (
	buyerclient "github.com/Will413028/hawkr/review-service/internal/client/buyer"
	mediaclient "github.com/Will413028/hawkr/review-service/internal/client/media"
	orderclient "github.com/Will413028/hawkr/review-service/internal/client/order"
	sellerclient "github.com/Will413028/hawkr/review-service/internal/client/seller"
	"github.com/Will413028/hawkr/review-service/internal/usecase"
	"go.uber.org/fx"
)

var Module = fx.Module("client",
	fx.Provide(fx.Annotate(orderclient.NewClient, fx.As(new(usecase.OrderClient)))),
	fx.Provide(fx.Annotate(mediaclient.NewClient, fx.As(new(usecase.MediaClient)))),
	fx.Provide(fx.Annotate(buyerclient.NewClient, fx.As(new(usecase.BuyerClient)))),
	fx.Provide(fx.Annotate(sellerclient.NewClient, fx.As(new(usecase.SellerClient)))),
)
