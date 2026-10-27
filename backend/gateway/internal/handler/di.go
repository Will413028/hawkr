package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/config"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/realtime"
	"github.com/Will413028/hawkr/gateway/internal/sse"
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
	media *grpcclient.MediaClient,
	cart *grpcclient.CartClient,
	social *grpcclient.SocialClient,
	im *grpcclient.IMClient,
	notification *grpcclient.NotificationClient,
	shipping *grpcclient.ShippingClient,
	coupon *grpcclient.CouponClient,
	crm *grpcclient.CrmClient,
	ai *grpcclient.AIClient,
	aiAssistant *grpcclient.AIAssistantClient,
	aiProcurement *grpcclient.AIProcurementClient,
	category *grpcclient.CategoryClient,
	review *grpcclient.ReviewServiceClient,
	orderReview *grpcclient.OrderReviewServiceClient,
	aftersales *grpcclient.AftersalesClient,
	productImport *grpcclient.ProductImportClient,
	pricebook *grpcclient.PricebookClient,
	invoice *grpcclient.InvoiceServiceClient,
	storeID *storeid.Resolver,
	buyerID *buyerid.Resolver,
	hub *sse.Hub,
	registry *realtime.Registry,
	commentCh *realtime.CommentChannel,
	notifCh *realtime.NotificationChannel,
	cfg *config.Config,
) *Holder {
	return &Holder{
		Account:        account,
		Seller:         seller,
		Buyer:          buyer,
		Product:        product,
		Order:          order,
		Payment:        payment,
		Media:          media,
		Cart:           cart,
		Social:         social,
		IM:             im,
		Notification:   notification,
		Shipping:       shipping,
		Coupon:         coupon,
		Crm:            crm,
		AI:             ai,
		AIAssistant:    aiAssistant,
		AIProcurement:  aiProcurement,
		Category:       category,
		Review:         review,
		OrderReview:    orderReview,
		Aftersales:     aftersales,
		ProductImport:  productImport,
		Pricebook:      pricebook,
		Invoice:        invoice,
		Store:          storeID,
		BuyerID:        buyerID,
		SSEHub:         hub,
		RealtimeReg:    registry,
		CommentChannel: commentCh,
		NotifChannel:   notifCh,
		MediaBaseURL:   cfg.MediaBaseURL,
		WebBaseURL:     cfg.WebBaseURL,
		Delegation:     cfg.Delegation,

		ProcurementReplaySseEnabled: cfg.ProcurementReplaySseEnabled,
		Docs:                        NewDocs(cfg.Docs),
	}
}
