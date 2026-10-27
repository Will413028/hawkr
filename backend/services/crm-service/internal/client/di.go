package client

import (
	accountclient "github.com/Will413028/hawkr/crm-service/internal/client/account"
	buyerclient "github.com/Will413028/hawkr/crm-service/internal/client/buyer"
	couponclient "github.com/Will413028/hawkr/crm-service/internal/client/coupon"
	orderclient "github.com/Will413028/hawkr/crm-service/internal/client/order"
	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"go.uber.org/fx"
)

var Module = fx.Module("client",
	fx.Provide(fx.Annotate(accountclient.NewClient, fx.As(new(usecase.AccountClient)))),
	fx.Provide(fx.Annotate(buyerclient.NewClient, fx.As(new(usecase.BuyerClient)))),
	fx.Provide(fx.Annotate(couponclient.NewClient, fx.As(new(usecase.CouponClient)))),
	fx.Provide(fx.Annotate(orderclient.NewClient, fx.As(new(usecase.OrderClient)))),
)
