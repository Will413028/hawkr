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
	fx.Provide(NewCartClient),
	fx.Provide(NewShippingClient),
	fx.Provide(NewCouponClient),
	fx.Provide(NewCategoryClient),
	fx.Provide(NewAftersalesClient),
	fx.Provide(NewInvoiceServiceClient),
	fx.Invoke(registerLifecycle),
)

type clientClosers struct {
	fx.In
	Account    *AccountClient
	Seller     *SellerClient
	Buyer      *BuyerClient
	Product    *ProductClient
	Order      *OrderClient
	Cart       *CartClient
	Shipping   *ShippingClient
	Coupon     *CouponClient
	Category   *CategoryClient
	Aftersales *AftersalesClient
	Invoice    *InvoiceServiceClient
}

func registerLifecycle(lc fx.Lifecycle, l *zap.Logger, c clientClosers) {
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			l.Info("Closing gRPC client connections")
			closers := []interface{ Close() error }{
				c.Account, c.Seller, c.Buyer, c.Product, c.Order,
				c.Cart,
				c.Shipping,
				c.Coupon, c.Category,
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
