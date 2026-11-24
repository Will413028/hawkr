package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/config"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
)

// Holder aggregates all gRPC clients needed by HTTP handlers.
// Constructed via fx DI; each client is lazily-connected by grpc.NewClient.
type Holder struct {
	Account       *grpcclient.AccountClient
	Seller        *grpcclient.SellerClient
	Buyer         *grpcclient.BuyerClient
	Product       *grpcclient.ProductClient
	Order         *grpcclient.OrderClient
	Payment       *grpcclient.PaymentClient
	Cart          *grpcclient.CartClient
	Shipping      *grpcclient.ShippingClient
	Coupon        *grpcclient.CouponClient
	AI            *grpcclient.AIClient
	AIAssistant   *grpcclient.AIAssistantClient
	AIProcurement *grpcclient.AIProcurementClient
	Category      *grpcclient.CategoryClient
	Aftersales    *grpcclient.AftersalesClient
	Invoice       *grpcclient.InvoiceServiceClient
	Store         *storeid.Resolver
	BuyerID       *buyerid.Resolver
	MediaBaseURL  string
	WebBaseURL    string
	Docs          *Docs
	// Delegation carries the per-hop gRPC deadlines for ADM 代行營運 (56 §8.5.1 (A)).
	// Deadlines belong to the caller, and the caller of every delegated hop is the
	// Gateway — so the numbers travel with the holder rather than being re-read from
	// the environment at each call site, where they would drift into a second copy.
	Delegation config.DelegationConfig
	// ProcurementReplaySseEnabled 是 config.Config.ProcurementReplaySseEnabled 的
	// copy-through（同 MediaBaseURL／WebBaseURL／Delegation 既有慣例），gate
	// ProxyProcurementRunEvents（見 ai.go）。
	ProcurementReplaySseEnabled bool
}
