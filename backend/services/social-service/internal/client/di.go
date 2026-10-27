package client

import (
	"github.com/Will413028/hawkr/social-service/internal/client/account"
	"github.com/Will413028/hawkr/social-service/internal/client/buyer"
	"github.com/Will413028/hawkr/social-service/internal/client/im"
	"github.com/Will413028/hawkr/social-service/internal/client/media"
	"github.com/Will413028/hawkr/social-service/internal/client/product"
	"github.com/Will413028/hawkr/social-service/internal/client/review"
	"github.com/Will413028/hawkr/social-service/internal/client/seller"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"go.uber.org/fx"
)

var Module = fx.Module("client",
	fx.Provide(
		// One constructor, two interfaces: the same *account.Client (and so the
		// same grpc.ClientConn) serves account lookups and the delegated-write
		// audit trail. Providing account.NewClient twice would open a second
		// connection to the same service and split the circuit breaker's view.
		fx.Annotate(account.NewClient,
			fx.As(new(usecase.AccountClient)),
			fx.As(new(usecase.DelegationAuditClient)),
		),
		fx.Annotate(buyer.NewClient, fx.As(new(usecase.BuyerClient))),
		fx.Annotate(media.NewClient, fx.As(new(usecase.MediaClient))),
		fx.Annotate(product.NewClient, fx.As(new(usecase.ProductClient))),
		fx.Annotate(seller.NewClient, fx.As(new(usecase.SellerClient))),
		fx.Annotate(review.NewClient, fx.As(new(usecase.ReviewModerationClient))),
		fx.Annotate(im.NewClient, fx.As(new(usecase.IMModerationClient))),
	),
)
