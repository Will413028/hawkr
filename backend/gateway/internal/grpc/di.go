package grpc

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("grpc",
	fx.Provide(NewAccountClient),
	fx.Provide(NewSellerClient),
	fx.Provide(NewBuyerClient),
	fx.Provide(NewProductClient),
	fx.Provide(NewOrderClient),
	fx.Provide(NewPaymentClient),
	fx.Provide(NewCartClient),
	fx.Provide(NewNotificationClient),
	fx.Provide(NewCouponClient),
	fx.Provide(newAIServiceConn),
	fx.Provide(NewAIClient),
	fx.Provide(NewAIAssistantClient),
	fx.Provide(NewAIProcurementClient),
	fx.Provide(NewCategoryClient),
	fx.Provide(NewAftersalesClient),
	fx.Provide(NewInvoiceServiceClient),
	fx.Invoke(registerLifecycle),
)

type clientClosers struct {
	fx.In
	Account       *AccountClient
	Seller        *SellerClient
	Buyer         *BuyerClient
	Product       *ProductClient
	Order         *OrderClient
	Payment       *PaymentClient
	Cart          *CartClient
	Notification  *NotificationClient
	Coupon        *CouponClient
	AIServiceConn *aiServiceConn
	Category      *CategoryClient
	Aftersales    *AftersalesClient
	Invoice       *InvoiceServiceClient
}

func registerLifecycle(lc fx.Lifecycle, l *zap.Logger, c clientClosers) {
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			l.Info("Closing gRPC client connections")
			closers := []interface{ Close() error }{
				c.Account, c.Seller, c.Buyer, c.Product, c.Order,
				c.Payment, c.Cart,
				c.Notification,
				c.Coupon, c.AIServiceConn, c.Category,
				c.Aftersales,
				c.Invoice,
			}
			for _, cl := range closers {
				if err := cl.Close(); err != nil {
					l.Warn("failed to close gRPC conn", zap.Error(err))
				}
			}
			return nil
		},
	})
}
