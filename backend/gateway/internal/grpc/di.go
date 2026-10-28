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
	fx.Provide(NewMediaClient),
	fx.Provide(NewCartClient),
	fx.Provide(NewSocialClient),
	fx.Provide(NewIMClient),
	fx.Provide(NewNotificationClient),
	fx.Provide(NewShippingClient),
	fx.Provide(NewCouponClient),
	fx.Provide(NewCrmClient),
	fx.Provide(newAIServiceConn),
	fx.Provide(NewAIClient),
	fx.Provide(NewAIAssistantClient),
	fx.Provide(NewAIProcurementClient),
	fx.Provide(NewCategoryClient),
	fx.Provide(NewReviewServiceClient),
	fx.Provide(NewOrderReviewServiceClient),
	fx.Provide(NewAftersalesClient),
	fx.Provide(NewProductImportClient),
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
	Media         *MediaClient
	Cart          *CartClient
	Social        *SocialClient
	IM            *IMClient
	Notification  *NotificationClient
	Shipping      *ShippingClient
	Coupon        *CouponClient
	Crm           *CrmClient
	AIServiceConn *aiServiceConn
	Category      *CategoryClient
	Review        *ReviewServiceClient
	OrderReview   *OrderReviewServiceClient
	Aftersales    *AftersalesClient
	ProductImport *ProductImportClient
	Invoice       *InvoiceServiceClient
}

func registerLifecycle(lc fx.Lifecycle, l *zap.Logger, c clientClosers) {
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			l.Info("Closing gRPC client connections")
			closers := []interface{ Close() error }{
				c.Account, c.Seller, c.Buyer, c.Product, c.Order,
				c.Payment, c.Media, c.Cart, c.Social, c.IM,
				c.Notification, c.Shipping,
				c.Coupon, c.Crm, c.AIServiceConn, c.Category, c.Review,
				c.OrderReview, c.Aftersales, c.ProductImport,
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
