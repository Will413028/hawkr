package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/config"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/realtime"
	"github.com/Will413028/hawkr/gateway/internal/sse"
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
	Media         *grpcclient.MediaClient
	Cart          *grpcclient.CartClient
	Social        *grpcclient.SocialClient
	IM            *grpcclient.IMClient
	Notification  *grpcclient.NotificationClient
	Shipping      *grpcclient.ShippingClient
	Coupon        *grpcclient.CouponClient
	Crm           *grpcclient.CrmClient
	AI            *grpcclient.AIClient
	AIAssistant   *grpcclient.AIAssistantClient
	AIProcurement *grpcclient.AIProcurementClient
	Category      *grpcclient.CategoryClient
	Review        *grpcclient.ReviewServiceClient
	OrderReview   *grpcclient.OrderReviewServiceClient
	Aftersales    *grpcclient.AftersalesClient
	ProductImport *grpcclient.ProductImportClient
	Pricebook     *grpcclient.PricebookClient
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
	SSEHub                      *sse.Hub
	RealtimeReg                 *realtime.Registry
	CommentChannel              *realtime.CommentChannel
	// NotifChannel 以介面型別持有（fx 仍注入 *realtime.NotificationChannel）：
	// serveSSE 本來就吃 realtime.Channel，介面化後測試才注入得了 spy channel 去
	// 斷言「訂閱時真正送進 Hub 的 topic 字串」——那正是身分空間錯了會壞掉的地方，
	// 而具體型別會逼測試起一整個 SSE Hub 才驗得到。
	NotifChannel realtime.Channel
}
