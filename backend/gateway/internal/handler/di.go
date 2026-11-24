package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/config"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	"go.uber.org/fx"
)

var Module = fx.Module("handler",
	fx.Provide(newStoreResolver),
	fx.Provide(newBuyerResolver),
	fx.Provide(NewHolder),
)

// newStoreResolver 建立店家身分解析器。
func newStoreResolver(seller *grpcclient.SellerClient) *storeid.Resolver {
	return storeid.New(seller)
}

// newBuyerResolver 建立買家身分解析器。
func newBuyerResolver(buyer *grpcclient.BuyerClient) *buyerid.Resolver {
	return buyerid.New(buyer)
}

func NewHolder(
	account *grpcclient.AccountClient,
	seller *grpcclient.SellerClient,
	buyer *grpcclient.BuyerClient,
	product *grpcclient.ProductClient,
	order *grpcclient.OrderClient,
	payment *grpcclient.PaymentClient,
	cart *grpcclient.CartClient,
	shipping *grpcclient.ShippingClient,
	coupon *grpcclient.CouponClient,
	category *grpcclient.CategoryClient,
	aftersales *grpcclient.AftersalesClient,
	invoice *grpcclient.InvoiceServiceClient,
	storeID *storeid.Resolver,
	buyerID *buyerid.Resolver,
	cfg *config.Config,
) *Holder {
	return &Holder{
		Account:      account,
		Seller:       seller,
		Buyer:        buyer,
		Product:      product,
		Order:        order,
		Payment:      payment,
		Cart:         cart,
		Shipping:     shipping,
		Coupon:       coupon,
		Category:     category,
		Aftersales:   aftersales,
		Invoice:      invoice,
		Store:        storeID,
		BuyerID:      buyerID,
		MediaBaseURL: cfg.MediaBaseURL,
		WebBaseURL:   cfg.WebBaseURL,
		Delegation:   cfg.Delegation,

		Docs: NewDocs(cfg.Docs),
	}
}
