package grpc

import (
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	aftersalespb "github.com/Will413028/hawkr/go-pkg/proto/aftersales"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	cartpb "github.com/Will413028/hawkr/go-pkg/proto/cart"
	couponpb "github.com/Will413028/hawkr/go-pkg/proto/coupon"
	crmpb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	impb "github.com/Will413028/hawkr/go-pkg/proto/im"
	notificationpb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	paymentpb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	productimportpb "github.com/Will413028/hawkr/go-pkg/proto/product_import"
	reviewpb "github.com/Will413028/hawkr/go-pkg/proto/review"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	shippingpb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"google.golang.org/grpc"
)

// ==================== Account ====================

// NewAccountClientForTest creates an AccountClient with the given mock proto client.
//
// NOTE: this deliberately bypasses the dial options, so NOTHING in
// DefaultDialOpts runs — in particular EnvelopeGuardInterceptor, which in
// production converts a non-zero envelope code into a gRPC status error before
// the generated client ever returns the reply. A mock injected here answers with
// the raw envelope instead, which is a DIFFERENT shape from what production
// code sees. Use it for tests about business logic; use
// NewAccountClientOverConnForTest when the interceptor chain is part of what is
// being asserted (see #2050).
func NewAccountClientForTest(c accountpb.AccountServiceClient) *AccountClient {
	return &AccountClient{client: c}
}

// NewAccountDelegationClientForTest creates an AccountClient whose DelegationService
// half is the given mock. The two halves are separate fields on AccountClient (they
// share one connection in production), so a delegation test that used
// NewAccountClientForTest would get a nil delegation client and panic rather than
// fail — which is why this constructor exists instead of a second parameter.
func NewAccountDelegationClientForTest(c accountpb.DelegationServiceClient) *AccountClient {
	return &AccountClient{delegation: c}
}

// NewAccountClientOverConnForTest creates an AccountClient over a real
// *grpc.ClientConn, so a test can dial an in-process account-service with
// DefaultDialOpts() and exercise the production interceptor chain end to end.
//
// This exists because its absence hid a live security hole: every #2050 test
// injected a mock proto client, so the 404-means-soft-deleted branch was only
// ever reached with a raw envelope that the real chain never delivers. The
// tests were green and the deployed gateway let deleted accounts through.
func NewAccountClientOverConnForTest(conn *grpc.ClientConn) *AccountClient {
	return &AccountClient{conn: conn, client: accountpb.NewAccountServiceClient(conn)}
}

// ==================== Seller ====================

// NewSellerClientForTest creates a SellerClient with the given mock proto client.
func NewSellerClientForTest(c sellerpb.SellerServiceClient) *SellerClient {
	return &SellerClient{client: c}
}

// ==================== AI ====================

// #3519 把 legacy AIService 依授權軸拆成三個 service，AIClient 因此持三個 stub。
// 每個 stub 各有一個測試建構子，比照 NewAccountClientForTest /
// NewAccountDelegationClientForTest 的既有分法：注入哪一個軸的 fake，就只有那個
// 軸可用，拿錯建構子會在 nil stub 上 panic 而不是靜默走到別的軸。

// NewAIImageClientForTest creates an AIClient whose AIImageService half is the
// given mock（TransformImage／ListAiStyles）。
func NewAIImageClientForTest(c aipb.AIImageServiceClient) *AIClient {
	return &AIClient{image: c}
}

// NewRecommendationClientForTest creates an AIClient whose RecommendationService
// half is the given mock（PersonalizedRecommend）。
func NewRecommendationClientForTest(c aipb.RecommendationServiceClient) *AIClient {
	return &AIClient{recommend: c}
}

// NewAIAssistantClientForTest creates an AIAssistantClient with the given mock proto client.
func NewAIAssistantClientForTest(c aipb.AIAssistantServiceClient) *AIAssistantClient {
	return &AIAssistantClient{client: c}
}

// NewAIProcurementClientForTest creates an AIProcurementClient with the given
// mock proto client, mirroring NewAIAssistantClientForTest — Task 11 added
// AIProcurementClient itself but not this constructor, so Task 13 (the first
// caller that needs to inject a fake AIProcurementServiceClient into
// Holder.AIProcurement for a handler test) adds it here rather than in a
// _test.go file: the `client` field is unexported, and handler package tests
// (a different package) need an exported constructor to reach it.
func NewAIProcurementClientForTest(c aipb.AIProcurementServiceClient) *AIProcurementClient {
	return &AIProcurementClient{client: c}
}

// ==================== CRM ====================

// NewCrmClientForTest creates a CrmClient with the given mock proto client.
func NewCrmClientForTest(c crmpb.CrmServiceClient) *CrmClient {
	return &CrmClient{client: c}
}

// ==================== Coupon ====================

// NewCouponClientForTest creates a CouponClient with the given mock proto client.
func NewCouponClientForTest(c couponpb.CouponServiceClient) *CouponClient {
	return &CouponClient{client: c}
}

// ==================== Order ====================

// NewOrderClientForTest creates an OrderClient with the given mock proto client.
func NewOrderClientForTest(c orderpb.OrderServiceClient) *OrderClient {
	return &OrderClient{orderClient: c}
}

// ==================== Payment ====================

// NewPaymentClientForTest creates a PaymentClient with the given mock proto client.
func NewPaymentClientForTest(c paymentpb.PaymentServiceClient) *PaymentClient {
	return &PaymentClient{client: c}
}

// NewPaymentResponsibilityClientForTest creates a PaymentClient whose
// FulfillmentResponsibilityService half is the given mock. Mirrors
// NewAccountDelegationClientForTest: the two halves are separate fields on
// PaymentClient (one connection in production), so a responsibility test that used
// NewPaymentClientForTest would get a nil stub and panic rather than fail.
//
// It exists for a specific kind of assertion: the A-5 decision guard must be shown to
// leave the downstream UNCALLED on a value outside {APPROVE, REJECT}. A guard that
// only returns 400 but still fires the RPC would pass a status-only assertion while
// letting an unreviewed proposal through.
func NewPaymentResponsibilityClientForTest(c paymentpb.FulfillmentResponsibilityServiceClient) *PaymentClient {
	return &PaymentClient{responsibility: c}
}

// ==================== Buyer ====================

// NewBuyerClientForTest creates a BuyerClient with the given mock proto client.
func NewBuyerClientForTest(c buyerpb.BuyerServiceClient) *BuyerClient {
	return &BuyerClient{client: c}
}

// ==================== Social ====================

// NewSocialClientForTest creates a SocialClient with the given mock proto client.
func NewSocialClientForTest(c socialpb.SocialServiceClient) *SocialClient {
	return &SocialClient{client: c}
}

// ==================== Cart ====================

// NewCartClientForTest creates a CartClient with the given mock proto client.
func NewCartClientForTest(c cartpb.CartServiceClient) *CartClient {
	return &CartClient{client: c}
}

// ==================== Product ====================

// NewProductClientForTest creates a ProductClient with the given mock proto client.
func NewProductClientForTest(c productpb.ProductServiceClient) *ProductClient {
	return &ProductClient{client: c}
}

// ==================== IM ====================

// NewIMClientForTest creates an IMClient with the given mock proto client.
func NewIMClientForTest(c impb.IMServiceClient) *IMClient {
	return &IMClient{client: c}
}

// ==================== Notification ====================

// NewNotificationClientForTest creates a NotificationClient with the given mock proto client.
func NewNotificationClientForTest(c notificationpb.NotificationServiceClient) *NotificationClient {
	return &NotificationClient{client: c}
}

// ==================== Aftersales ====================

// NewAftersalesClientForTest creates an AftersalesClient with the given mock proto client.
func NewAftersalesClientForTest(c aftersalespb.AfterSalesServiceClient) *AftersalesClient {
	return &AftersalesClient{client: c}
}

// ==================== Review ====================

// NewReviewClientForTest creates a ReviewServiceClient with the given mock proto client.
func NewReviewClientForTest(c reviewpb.ReviewServiceClient) *ReviewServiceClient {
	return &ReviewServiceClient{client: c}
}

// NewOrderReviewClientForTest creates an OrderReviewServiceClient with the given mock proto client.
func NewOrderReviewClientForTest(c reviewpb.ReviewServiceClient) *OrderReviewServiceClient {
	return &OrderReviewServiceClient{client: c}
}

// ==================== Shipping ====================

// NewShippingClientForTest creates a ShippingClient with the given mock proto client.
func NewShippingClientForTest(c shippingpb.ShippingServiceClient) *ShippingClient {
	return &ShippingClient{shippingClient: c}
}

// ==================== ProductImport ====================

// NewProductImportClientForTest creates a ProductImportClient with the given mock proto client.
func NewProductImportClientForTest(c productimportpb.ProductImportServiceClient) *ProductImportClient {
	return &ProductImportClient{client: c}
}
