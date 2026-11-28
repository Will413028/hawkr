// Package httpapi 是單體對外的 HTTP 入口。
//
// 這一層只做傳輸與畫面需要的讀取組裝，業務規則一律在模組裡。對外的 API 是根目錄 api/ 的
// Protobuf＋Connect（ADR docs/adr/2026-09-26-external-contract-protobuf-connect.md），慣例見
// `docs/共通規範/API 設計規範（Connect）.md`；共用的傳輸元件在 rpc，各模組的 handler 在同名的子套件。
package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/Will413028/hawkr/backend/internal/httpapi/adminauth"
	httpassistant "github.com/Will413028/hawkr/backend/internal/httpapi/assistant"
	httpbizcalendar "github.com/Will413028/hawkr/backend/internal/httpapi/bizcalendar"
	"github.com/Will413028/hawkr/backend/internal/httpapi/blocks"
	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	httpbuyerprocurement "github.com/Will413028/hawkr/backend/internal/httpapi/buyerprocurement"
	httpcatalog "github.com/Will413028/hawkr/backend/internal/httpapi/catalog"
	httpchat "github.com/Will413028/hawkr/backend/internal/httpapi/chat"
	httpcontent "github.com/Will413028/hawkr/backend/internal/httpapi/content"
	httpcoupon "github.com/Will413028/hawkr/backend/internal/httpapi/coupon"
	httpcrm "github.com/Will413028/hawkr/backend/internal/httpapi/crm"
	httpdelegation "github.com/Will413028/hawkr/backend/internal/httpapi/delegation"
	httpdiscovery "github.com/Will413028/hawkr/backend/internal/httpapi/discovery"
	adminbizcalendarconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1/bizcalendarv1alpha1connect"
	adminbuyerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1/buyerv1alpha1connect"
	admincatalogconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/catalog/v1alpha1/catalogv1alpha1connect"
	admincontentconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1/contentv1alpha1connect"
	admincouponconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/coupon/v1alpha1/couponv1alpha1connect"
	adminidentityconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1/identityv1alpha1connect"
	adminimageaiconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/imageai/v1alpha1/imageaiv1alpha1connect"
	adminmoderationconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/moderation/v1alpha1/moderationv1alpha1connect"
	adminnotificationconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/notification/v1alpha1/notificationv1alpha1connect"
	adminpaymentconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1/paymentv1alpha1connect"
	adminreviewqueueconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/reviewqueue/v1alpha1/reviewqueuev1alpha1connect"
	adminsellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	adminsocialconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/assistant/v1alpha1/assistantv1alpha1connect"
	buyerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1/buyerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1/chatv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1/contentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1/couponv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/crm/v1alpha1/crmv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/discovery/v1alpha1/discoveryv1alpha1connect"
	identityconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/imageai/v1alpha1/imageaiv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1/moderationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1/notificationv1alpha1connect"
	paymentconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1/paymentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1/procurementv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1/reviewv1alpha1connect"
	sellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	httpidentity "github.com/Will413028/hawkr/backend/internal/httpapi/identity"
	httpimageai "github.com/Will413028/hawkr/backend/internal/httpapi/imageai"
	httpmedia "github.com/Will413028/hawkr/backend/internal/httpapi/media"
	httpmoderation "github.com/Will413028/hawkr/backend/internal/httpapi/moderation"
	httpnotification "github.com/Will413028/hawkr/backend/internal/httpapi/notification"
	httppayment "github.com/Will413028/hawkr/backend/internal/httpapi/payment"
	httpreview "github.com/Will413028/hawkr/backend/internal/httpapi/review"
	httpreviewqueue "github.com/Will413028/hawkr/backend/internal/httpapi/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	httpsellerprocurement "github.com/Will413028/hawkr/backend/internal/httpapi/sellerprocurement"
	httpsocial "github.com/Will413028/hawkr/backend/internal/httpapi/social"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	modcatalog "github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
)

// Deps 是 handler 需要的外部依賴，全部在組裝點提供。
type Deps struct {
	// Ready 回報這個 instance 是否能服務請求（例如資料庫連得上）。
	Ready      func(context.Context) error
	Content    *content.Module
	Buyer      *buyer.Module
	BuyerPorts httpbuyer.Ports
	// SocialProducts 是貼文卡片的商品顯示資料來源（批次）。
	SocialProducts httpsocial.Products
	Deletion       *accountdeletion.Module
	Delegation     *delegation.Module
	Discovery      *discovery.Module
	// ReviewQueue 是後台待辦（ReviewQueueAdminService）；時鐘用 Now。
	ReviewQueue *reviewqueue.Module
	Identity    *identity.Module
	// WebLogin 是 web 的聯合登入入口（/api/auth/{provider}/start 與 callback，經同源反代）；nil 時不掛。
	WebLogin *httpidentity.WebLogin
	// AppLogin 驗證 App 原生登入的 ID token（IdentityPublicService.ExchangeIdToken、IdentityAppService.LinkIdentity）；nil 時等於沒有設定任何
	// provider，這兩個方法回 unavailable。
	AppLogin *httpidentity.AppLogin
	Seller   *seller.Module
	// Payment 是收款帳戶（payment 模組）：賣家的收款帳戶 RPC（seller 確認擁有者之後呼叫）、PaymentAdminService 與收款 webhook。
	Payment *payment.Module
	// PaymentPublishableKey 是付款 provider（Stripe）的公開金鑰（不是 secret），PaymentAppService 隨待付款的付款回給 client 初始化 Stripe.js 與 PaymentSheet；沒有設定時 Stripe 付款的 session 回 internal。
	PaymentPublishableKey string
	// BizCalendar 是工作日行事曆（platform/bizcalendar）：BizCalendarAdminService 的匯入與涵蓋查詢。
	BizCalendar *bizcalendar.Store
	// Social 給後台建立內容下架措施前核對內容歸屬（SellerGovernanceAdminService.CreateMeasure）。
	Social *social.Module
	// Review 是評價（ReviewPublicService、ReviewAppService）；封鎖入口也用它解析評價參照的作者。
	Review *review.Module
	// IM 是買家與店家之間的聊天（ChatAppService）。
	IM *im.Module
	// Coupon 是優惠券：買家的券與可領的活動、店家發的活動（CouponAppService）。
	Coupon *coupon.Module
	// Catalog 是商品模組：CatalogPublicService、SellerCatalogAppService、CatalogAdminService，與買家「追蹤中」動態的商品來源。
	// 選填：舊服務處理商品流量到 B4-6，cmd/hawkr 不給（三支 service 不掛、ListFollowedStoreProducts 回 unimplemented），切換的同一變更才給；測試給。
	// 和 Media、Legacy 一樣是 nil 就不掛的選填依賴。
	Catalog *modcatalog.Module
	// CRM 是店家的客戶與標籤（CrmAppService）。
	CRM *crm.Module
	// Moderation 是檢舉的送出與讀自己的檢舉（ModerationAppService）。
	Moderation *moderation.Module
	// Media 是公開圖片的暫存上傳（MediaAppService）；nil 時每個方法回 unavailable。
	Media *media.Module
	// Notification 是通知：登入帳號的收件匣、偏好、裝置 token 與訊號串流（NotificationAppService）與後台公告（NotificationAdminService）。
	Notification *notification.Module
	// Assistant 是買家助理（AssistantAppService）。
	Assistant *assistant.Module
	// Procurement 是買家採購（BuyerProcurementAppService）。
	Procurement *procurement.Module
	// ImageAI 是賣家修圖（ImageAiAppService）與營運後台的風格、額度管理（ImageAiAdminService）。
	ImageAI *imageai.Module
	// Stats 是店家統計與卡片評分的組裝（粉絲、評價；review 還在舊服務），必填：店面頁、賣家自己的店與所有店家卡片共用同一份，由組裝點以
	// httpseller.NewStatsSource 建立。
	Stats *httpseller.StatsSource
	// DashboardOrders 是賣家儀表板的訂單來源（order 還在舊服務），必填：GetMyDashboard 的銷售與訂單狀態靠它，沒有它啟動就失敗。
	DashboardOrders httpseller.DashboardOrders
	// DashboardBudget 是儀表板讀每個來源的時間上限；零值用預設（httpseller.Dashboard.Budget）。測試用來讓逾時與機器速度無關。
	DashboardBudget time.Duration
	// Images 把 object key 組成對外的圖片（MEDIA_ORIGIN）。
	Images rpc.Images
	// WebOrigin 把站內路徑組成對外的絕對網址（PUBLIC_WEB_ORIGIN），必填：入駐文件的網址靠它，零值啟動就失敗。
	WebOrigin rpc.WebOrigin
	// Origins 是自己的 web 與後台的 origin：CSRF 信任它們，gateway 的 WebSocket 比對 Origin 也用它。web 在各環境都經
	// 第一方同源反代打單體，單體不回 CORS 的 header，不支援瀏覽器跨來源呼叫。
	Origins []string
	// OriginAuth 驗證網頁 SSR 與 CI 自己簽出的限流身分；ClientIP 決定訪客的 IP。
	OriginAuth OriginAuth
	ClientIP   ClientIP
	// Legacy 是轉給舊 gateway 的 handler，掛在 LegacyPrefixes 底下（internal/legacy/gatewayproxy）；nil 時不掛。
	Legacy         http.Handler
	LegacyPrefixes []string
	// Now 是目前時間；nil 時用 time.Now。
	Now    func() time.Time
	Logger *slog.Logger
	// Tracer 建立每個請求的 span；nil 時用不匯出的 SDK provider，trace id 照樣產生。
	Tracer trace.TracerProvider
}

const maxRequestBytes = 2 << 20

const imageAIMaxRequestBytes = 3 << 20

const compressMinBytes = 1 << 10

// NewHandler 組出對外 HTTP 的 handler：單體是對外的入口（遷移計畫 A2），Connect 在這裡處理，還沒搬的路徑轉給
// gateway。
//
// /healthz 是 liveness：process 還活著就回 204，不檢查依賴，以免資料庫短暫中斷時
// 整批 pod 被重啟。/readyz 是 readiness：依賴不可用時回 503，讓流量先避開這個 instance。
func NewHandler(deps Deps) (http.Handler, error) {
	if deps.Delegation == nil {
		return nil, fmt.Errorf("httpapi：IdentityAdminService 需要 Delegation")
	}
	if deps.Deletion == nil {
		return nil, fmt.Errorf("httpapi：IdentityAppService 需要 Deletion")
	}
	if deps.Seller == nil {
		return nil, fmt.Errorf("httpapi：SellerAppService 需要 Seller")
	}
	if deps.Discovery == nil {
		return nil, fmt.Errorf("httpapi：DiscoveryPublicService 需要 Discovery")
	}
	if deps.Stats == nil {
		return nil, fmt.Errorf("httpapi：店家統計與卡片評分需要 Stats")
	}
	if deps.DashboardOrders == nil {
		return nil, fmt.Errorf("httpapi：SellerAppService.GetMyDashboard 需要 DashboardOrders")
	}
	if deps.ReviewQueue == nil {
		return nil, fmt.Errorf("httpapi：ReviewQueueAdminService 需要 ReviewQueue")
	}
	if deps.Payment == nil {
		return nil, fmt.Errorf("httpapi：PaymentAdminService 與收款 webhook 需要 Payment")
	}
	if deps.BizCalendar == nil {
		return nil, fmt.Errorf("httpapi：BizCalendarAdminService 需要 BizCalendar")
	}
	if deps.Social == nil {
		return nil, fmt.Errorf("httpapi：SellerGovernanceAdminService 需要 Social")
	}
	if deps.Buyer == nil {
		return nil, fmt.Errorf("httpapi：BuyerAppService 與 IdentityAppService.GetSession 需要 Buyer")
	}
	if deps.Review == nil {
		return nil, fmt.Errorf("httpapi：ReviewPublicService、ReviewAppService 與封鎖入口的評價參照需要 Review")
	}
	if deps.IM == nil {
		return nil, fmt.Errorf("httpapi：ChatAppService 需要 IM")
	}
	if deps.Identity == nil {
		return nil, fmt.Errorf("httpapi：ChatAppService 串流的重新驗證需要 Identity")
	}
	if deps.Coupon == nil {
		return nil, fmt.Errorf("httpapi：CouponAppService 需要 Coupon")
	}
	if deps.CRM == nil {
		return nil, fmt.Errorf("httpapi：CrmAppService 需要 CRM")
	}
	if deps.Moderation == nil {
		return nil, fmt.Errorf("httpapi：ModerationAppService、ModerationAdminService 與 SocialAdminService 需要 Moderation")
	}
	if deps.Notification == nil {
		return nil, fmt.Errorf("httpapi：NotificationAppService 與 NotificationAdminService 需要 Notification")
	}
	if deps.Assistant == nil {
		return nil, fmt.Errorf("httpapi：AssistantAppService 需要 Assistant")
	}
	if deps.Procurement == nil {
		return nil, fmt.Errorf("httpapi：BuyerProcurementAppService 需要 Procurement")
	}
	if deps.ImageAI == nil {
		return nil, fmt.Errorf("httpapi：ImageAiAppService 需要 ImageAI")
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Logger == nil {
		deps.Logger = slog.New(slog.DiscardHandler)
	}
	services, err := connectServices(deps, handlerOptions(deps.Logger)...)
	if err != nil {
		return nil, err
	}
	return assemble(deps, services, fileRoutes(deps)...)
}

func mediaUploads(m *media.Module) httpmedia.DelegatedUploads {
	if m == nil {
		return nil
	}
	return m
}

func handlerOptions(log *slog.Logger) []connect.HandlerOption {
	return []connect.HandlerOption{
		connect.WithInterceptors(rpc.NewInterceptor(log), adminauth.NewInterceptor(log)),
		connect.WithReadMaxBytes(maxRequestBytes),
		connect.WithCompressMinBytes(compressMinBytes),
	}
}

type fileRoute struct {
	pattern    string
	handler    http.Handler
	permission adminperm.Permission
}

func fileRoutes(deps Deps) []fileRoute {
	routes := sellerFileRoutes(deps)
	if deps.Media != nil {
		secure := httpmedia.NewSecureUploads(deps.Media, deps.Logger)
		if deps.Delegation != nil {
			secure.WithDelegation(deps.Delegation)
		}
		routes = append(routes,
			fileRoute{pattern: httpmedia.ChunkRoute, handler: secure.Chunk()},
			fileRoute{pattern: httpmedia.DelegatedChunkRoute, handler: secure.DelegatedChunk()},
			fileRoute{pattern: httpmedia.PreviewRoute, handler: secure.Preview()},
			fileRoute{pattern: httpmedia.DelegatedPreviewRoute, handler: secure.DelegatedPreview()})
	}
	return routes
}

func sellerFileRoutes(deps Deps) []fileRoute {
	if deps.Seller == nil {
		return nil
	}
	return []fileRoute{{
		pattern: httpseller.OwnDocumentRoute,
		handler: httpseller.NewOwnDocuments(deps.Seller, deps.Logger),
	}, {
		pattern:    httpseller.DocumentRoute,
		handler:    httpseller.NewAdminDocuments(deps.Seller, deps.Logger),
		permission: adminperm.SellerKYCDocumentsGet,
	}, {
		pattern:    httpseller.EvidenceReportRoute,
		handler:    httpseller.NewAdminEvidence(deps.Seller, seller.EvidenceReport, deps.Logger),
		permission: adminperm.SellerReportEvidenceGet,
	}, {
		pattern:    httpseller.EvidenceRequestRoute,
		handler:    httpseller.NewAdminEvidence(deps.Seller, seller.EvidenceRequest, deps.Logger),
		permission: adminperm.SellerMeasureRequestEvidenceGet,
	}, {
		pattern:    httpseller.EvidenceInquiryRoute,
		handler:    httpseller.NewAdminEvidence(deps.Seller, seller.EvidenceInquiry, deps.Logger),
		permission: adminperm.SellerCaseInquiryEvidenceGet,
	}}
}

func assemble(deps Deps, services []service, files ...fileRoute) (http.Handler, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	csrf, err := newCSRF(deps.Origins)
	if err != nil {
		return nil, err
	}
	limiter := newRateLimiter(deps.Now)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := deps.Ready(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	for _, svc := range services {
		mount(mux, svc.path, withRateLimit(limiter, rpc.LogUnhandled(deps.Logger, svc.handler)))
	}
	for _, f := range files {
		h, err := adminauth.GuardFile(f.pattern, f.permission, f.handler, deps.Logger)
		if err != nil {
			return nil, err
		}
		mux.Handle(f.pattern, withFileRateLimit(limiter, h))
	}
	if deps.WebLogin != nil {
		for _, route := range deps.WebLogin.Routes() {
			mux.Handle(route.Pattern, withAuthRateLimit(limiter, route.Handler))
		}
	}
	if deps.Legacy != nil {
		legacy := adminauth.GuardLegacy(deps.Legacy, deps.Logger)
		for _, prefix := range deps.LegacyPrefixes {
			mux.Handle(prefix, legacy)
		}
	}
	var h http.Handler = mux
	h = withSubject(deps.OriginAuth, deps.ClientIP, deps.Now, h)
	h = withSession(deps.Identity, deps.Logger, h)
	h = withCookieCSRF(cookieCSRFOrigins(deps.WebOrigin, deps.Origins), h)
	h = csrf.Handler(h)
	h = withPaymentWebhook(deps.Payment, deps.Logger, h)
	h = withChargeWebhook(deps.Payment, deps.Logger, h)
	h = withResendWebhook(deps.Notification, deps.Logger, h)
	return withTracing(deps.Tracer, h), nil
}

func withPaymentWebhook(events httppayment.Events, log *slog.Logger, next http.Handler) http.Handler {
	root := http.NewServeMux()
	root.Handle(httppayment.WebhookRoute, httppayment.NewWebhook(events, log))
	root.Handle("/", next)
	return root
}

func withChargeWebhook(events httppayment.ChargeEvents, log *slog.Logger, next http.Handler) http.Handler {
	root := http.NewServeMux()
	root.Handle(httppayment.ChargeWebhookRoute, httppayment.NewChargeWebhook(events, log))
	root.Handle("/", next)
	return root
}

func withResendWebhook(events httpnotification.ResendEvents, log *slog.Logger, next http.Handler) http.Handler {
	root := http.NewServeMux()
	root.Handle(httpnotification.ResendWebhookRoute, httpnotification.NewResendWebhook(events, log))
	root.Handle("/", next)
	return root
}

type service struct {
	path    string
	handler http.Handler
}

func connectServices(deps Deps, opts ...connect.HandlerOption) ([]service, error) {
	var out []service
	add := func(path string, h http.Handler) { out = append(out, service{path, h}) }
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	stats := deps.Stats
	add(contentv1alpha1connect.NewContentPublicServiceHandler(httpcontent.NewPublic(deps.Content, deps.Images), opts...))
	add(admincontentconnect.NewContentAdminServiceHandler(httpcontent.NewAdmin(deps.Content, deps.Images), opts...))
	socialPublic, err := httpsocial.NewPublic(deps.Social, deps.Social, deps.Buyer, deps.Seller, deps.SocialProducts, deps.Images, deps.WebOrigin, deps.Now, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(socialv1alpha1connect.NewSocialPublicServiceHandler(socialPublic, opts...))
	socialApp, err := httpsocial.NewApp(deps.Social, deps.Social, deps.Social, deps.Buyer, deps.Seller, deps.SocialProducts, deps.Images, deps.WebOrigin, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(socialv1alpha1connect.NewSocialAppServiceHandler(socialApp, opts...))
	add(mediav1alpha1connect.NewMediaAppServiceHandler(httpmedia.NewApp(deps.Media, deps.Logger), opts...))
	reviewPublic, err := httpreview.NewPublic(deps.Review, deps.Buyer, deps.Images, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(reviewv1alpha1connect.NewReviewPublicServiceHandler(reviewPublic, opts...))
	reviewApp, err := httpreview.NewApp(deps.Review, deps.Review, deps.Buyer, deps.Seller, deps.Images, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(reviewv1alpha1connect.NewReviewAppServiceHandler(reviewApp, opts...))
	chatApp, err := httpchat.NewApp(deps.IM, deps.IM, httpchat.ModuleStreams(deps.IM), deps.Identity, deps.Buyer, deps.Seller, deps.Images, deps.Logger)
	if err != nil {
		return nil, err
	}
	chatPath, chatHandler := chatv1alpha1connect.NewChatAppServiceHandler(chatApp, opts...)
	add(chatPath, httpchat.WithStreamWriter(chatHandler))
	crmApp, err := httpcrm.NewApp(deps.CRM, deps.CRM, deps.Buyer, deps.Seller, deps.Images, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(crmv1alpha1connect.NewCrmAppServiceHandler(crmApp, opts...))
	couponApp, err := httpcoupon.NewApp(deps.Coupon, deps.Buyer, deps.CRM, deps.Seller, deps.Images, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(couponv1alpha1connect.NewCouponAppServiceHandler(couponApp, opts...))
	add(admincouponconnect.NewCouponAdminServiceHandler(httpcoupon.NewAdmin(deps.Coupon), opts...))
	moderationApp, err := httpmoderation.NewApp(deps.Moderation, deps.Seller, deps.Buyer, deps.IM, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(moderationv1alpha1connect.NewModerationAppServiceHandler(moderationApp, opts...))
	notificationApp, err := httpnotification.NewApp(deps.Notification, deps.Seller, deps.Images, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(notificationv1alpha1connect.NewNotificationAppServiceHandler(notificationApp, opts...))
	add(adminmoderationconnect.NewModerationAdminServiceHandler(httpmoderation.NewAdmin(deps.Moderation), opts...))
	add(adminnotificationconnect.NewNotificationAdminServiceHandler(httpnotification.NewAdmin(deps.Notification), opts...))
	socialAdmin, err := httpsocial.NewAdmin(deps.Social, deps.Moderation, deps.Images)
	if err != nil {
		return nil, err
	}
	add(adminsocialconnect.NewSocialAdminServiceHandler(socialAdmin, opts...))
	blocking := blocks.Sources{Authors: deps.Social, Reviews: deps.Review, Messages: deps.IM, Identities: blocks.Identities{Buyers: deps.Buyer, Stores: deps.Seller}}
	buyerPorts := deps.BuyerPorts
	if deps.Catalog != nil {
		catalogPresenter, err := httpcatalog.NewPresenter(deps.Seller, stats, deps.Images)
		if err != nil {
			return nil, err
		}
		buyerPorts.Followed = httpbuyer.FollowedProducts{Follows: deps.Buyer, Catalog: deps.Catalog, Cards: catalogPresenter, Now: now}
		catalogPublic, err := httpcatalog.NewPublic(deps.Catalog, catalogPresenter, now)
		if err != nil {
			return nil, err
		}
		add(catalogv1alpha1connect.NewCatalogPublicServiceHandler(catalogPublic, opts...))
		catalogApp, err := httpcatalog.NewApp(deps.Catalog, deps.Seller, deps.Images, deps.Logger)
		if err != nil {
			return nil, err
		}
		add(catalogv1alpha1connect.NewSellerCatalogAppServiceHandler(catalogApp, opts...))
		catalogDelegated, err := httpdelegation.NewInterceptor(deps.Delegation, deps.Logger, httpcatalog.DelegatedDeclaration)
		if err != nil {
			return nil, err
		}
		delegatedCatalog, err := httpcatalog.NewDelegated(deps.Delegation, deps.Catalog, deps.Images, deps.Logger)
		if err != nil {
			return nil, err
		}
		add(catalogv1alpha1connect.NewCatalogDelegatedServiceHandler(delegatedCatalog, append(slices.Clone(opts), connect.WithInterceptors(catalogDelegated))...))
		catalogAdmin, err := httpcatalog.NewAdmin(deps.Catalog, deps.Images)
		if err != nil {
			return nil, err
		}
		add(admincatalogconnect.NewCatalogAdminServiceHandler(catalogAdmin, opts...))
	}
	buyerApp, err := httpbuyer.NewApp(deps.Buyer, deps.Images, buyerPorts, stats, blocking)
	if err != nil {
		return nil, err
	}
	add(buyerconnect.NewBuyerAppServiceHandler(buyerApp, opts...))
	add(adminbuyerconnect.NewBuyerAdminServiceHandler(httpbuyer.NewAdmin(deps.Buyer), opts...))
	appLogin := deps.AppLogin
	if appLogin == nil {
		if appLogin, err = httpidentity.NewAppLogin(deps.Identity); err != nil {
			return nil, err
		}
	}
	add(identityconnect.NewIdentityPublicServiceHandler(httpidentity.NewPublic(appLogin), opts...))
	identityApp, err := httpidentity.NewApp(deps.Identity, deps.Deletion, appLogin, now, deps.Buyer, deps.Images, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(identityconnect.NewIdentityAppServiceHandler(identityApp, opts...))
	add(adminidentityconnect.NewIdentityAdminServiceHandler(httpidentity.NewAdmin(deps.Identity, deps.Delegation, deps.Deletion), opts...))
	accounts := newAccountLimiter(deps.Now)
	acting, err := httpseller.NewInterceptor(deps.Seller, accounts)
	if err != nil {
		return nil, err
	}
	dashboard := httpseller.Dashboard{Orders: deps.DashboardOrders, Views: deps.Seller, Followers: deps.Buyer, Now: deps.Now, Budget: deps.DashboardBudget, Log: deps.Logger}
	discoveryPublic, err := httpdiscovery.NewPublic(deps.Discovery, deps.Seller, deps.Buyer, stats, deps.Images, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(discoveryv1alpha1connect.NewDiscoveryPublicServiceHandler(discoveryPublic, opts...))
	sellerPublic, err := httpseller.NewPublic(deps.Seller, stats, deps.Images, deps.WebOrigin)
	if err != nil {
		return nil, err
	}
	sellerApp, err := httpseller.NewApp(deps.Seller, deps.Payment, deps.Buyer, deps.Images, dashboard, stats, deps.Social, deps.WebOrigin, blocking, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(adminsellerconnect.NewSellerAdminServiceHandler(httpseller.NewAdmin(deps.Seller, deps.Images, deps.Payment, deps.Social, deps.Logger), opts...))
	add(sellerconnect.NewSellerPublicServiceHandler(sellerPublic, opts...))
	add(sellerconnect.NewSellerAppServiceHandler(sellerApp, append(slices.Clone(opts), connect.WithInterceptors(acting))...))
	governance, err := httpseller.NewGovernance(httpseller.GovernanceDeps{Module: deps.Seller, Images: deps.Images, ContentSnapshots: deps.Social, Links: deps.Social})
	if err != nil {
		return nil, err
	}
	add(adminsellerconnect.NewSellerGovernanceAdminServiceHandler(governance, opts...))
	reports, err := httpseller.NewReports(deps.Seller, deps.Buyer, accounts)
	if err != nil {
		return nil, err
	}
	add(sellerconnect.NewSellerReportAppServiceHandler(reports, opts...))
	add(adminsellerconnect.NewSellerProspectAdminServiceHandler(httpseller.NewProspects(deps.Seller, deps.Images), opts...))
	add(adminpaymentconnect.NewPaymentAdminServiceHandler(httppayment.NewAdmin(deps.Payment), opts...))
	add(paymentconnect.NewPaymentAppServiceHandler(httppayment.NewApp(deps.Payment, deps.Buyer, deps.PaymentPublishableKey), opts...))
	add(adminbizcalendarconnect.NewBizCalendarAdminServiceHandler(httpbizcalendar.NewAdmin(deps.BizCalendar, deps.Now), opts...))
	add(adminreviewqueueconnect.NewReviewQueueAdminServiceHandler(httpreviewqueue.NewAdmin(deps.ReviewQueue, sellerStoreNames{deps.Seller}, deps.Now), opts...))
	add(assistantv1alpha1connect.NewAssistantAppServiceHandler(httpassistant.NewApp(deps.Assistant, deps.Buyer, deps.Logger), opts...))
	add(procurementv1alpha1connect.NewBuyerProcurementAppServiceHandler(httpbuyerprocurement.NewApp(deps.Procurement, deps.Buyer, deps.Seller, deps.Images, deps.Logger), opts...))
	sellerProcurementApp, err := httpsellerprocurement.NewApp(deps.Procurement, deps.Seller, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(procurementv1alpha1connect.NewSellerProcurementAppServiceHandler(sellerProcurementApp, opts...))
	add(imageaiv1alpha1connect.NewImageAiAppServiceHandler(httpimageai.NewApp(deps.ImageAI, deps.Seller, deps.Images, deps.Logger), append(slices.Clone(opts), connect.WithReadMaxBytes(imageAIMaxRequestBytes))...))
	add(adminimageaiconnect.NewImageAiAdminServiceHandler(httpimageai.NewAdmin(deps.ImageAI, deps.Images), opts...))
	socialDelegated, err := httpdelegation.NewInterceptor(deps.Delegation, deps.Logger, httpsocial.DelegatedDeclaration)
	if err != nil {
		return nil, err
	}
	delegatedSocial, err := httpsocial.NewDelegated(deps.Delegation, deps.Social, deps.Buyer, deps.Seller, deps.SocialProducts, deps.Images, deps.WebOrigin, deps.Logger)
	if err != nil {
		return nil, err
	}
	add(socialv1alpha1connect.NewSocialDelegatedServiceHandler(delegatedSocial, append(slices.Clone(opts), connect.WithInterceptors(socialDelegated))...))
	mediaDelegated, err := httpdelegation.NewInterceptor(deps.Delegation, deps.Logger, httpmedia.DelegatedDeclaration)
	if err != nil {
		return nil, err
	}
	add(mediav1alpha1connect.NewMediaDelegatedServiceHandler(httpmedia.NewDelegated(deps.Delegation, mediaUploads(deps.Media), deps.Logger), append(slices.Clone(opts), connect.WithInterceptors(mediaDelegated))...))
	if err := checkAdminDeclarations(out); err != nil {
		return nil, err
	}
	if err := checkCustomerEffects(out); err != nil {
		return nil, err
	}
	if err := checkPublicViewerDependence(out); err != nil {
		return nil, err
	}
	return out, nil
}

func adminServiceDescriptors(services []service) ([]protoreflect.ServiceDescriptor, error) {
	return serviceDescriptors(services, "admin", func(a rpc.Audience) bool { return a == rpc.AudienceAdmin })
}

func serviceDescriptors(services []service, label string, want func(rpc.Audience) bool) ([]protoreflect.ServiceDescriptor, error) {
	var out []protoreflect.ServiceDescriptor
	for _, svc := range services {
		if audience, _ := rpc.AudienceOf(svc.path); !want(audience) {
			continue
		}
		name := protoreflect.FullName(strings.Trim(svc.path, "/"))
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(name)
		if err != nil {
			return nil, fmt.Errorf("httpapi：找不到 %s service %s 的 descriptor：%w", label, name, err)
		}
		sd, ok := d.(protoreflect.ServiceDescriptor)
		if !ok {
			return nil, fmt.Errorf("httpapi：%s 不是 service", name)
		}
		out = append(out, sd)
	}
	return out, nil
}

func checkAdminDeclarations(services []service) error {
	descriptors, err := adminServiceDescriptors(services)
	if err != nil {
		return err
	}
	return adminauth.Check(descriptors)
}

func mount(mux *http.ServeMux, path string, h http.Handler) {
	audience, ok := rpc.AudienceOf(path)
	if !ok {
		panic(fmt.Sprintf("httpapi：%s 不符合 service 的命名規則（<模組>PublicService、<模組>AppService、<模組>DelegatedService、hawkr.admin.* 的 <模組>AdminService）", path))
	}
	mux.Handle(path, cachePolicy(audience, isViewerDependent, h))
}

func withTracing(tp trace.TracerProvider, next http.Handler) http.Handler {
	if tp == nil {
		tp = sdktrace.NewTracerProvider()
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sc := trace.SpanContextFromContext(r.Context()); sc.HasTraceID() {
			w.Header().Set("X-Trace-Id", sc.TraceID().String())
		}
		next.ServeHTTP(w, r)
	})
	traced := otelhttp.NewHandler(restoreURL(h), "hawkr",
		otelhttp.WithTracerProvider(tp),
		otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator()),
		otelhttp.WithSpanNameFormatter(spanName))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if red, ok := redactedTokenPath(r.URL.Path); ok {
			rc := r.Clone(context.WithValue(r.Context(), realURLKey{}, r.URL))
			u := *r.URL
			u.Path, u.RawPath = red, ""
			rc.URL = &u
			rc.RequestURI = u.RequestURI()
			r = rc
		}
		traced.ServeHTTP(w, r)
	})
}

type realURLKey struct{}

func restoreURL(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := r.Context().Value(realURLKey{}).(*url.URL); ok {
			r = r.Clone(r.Context())
			r.URL = u
			r.RequestURI = u.RequestURI()
		}
		next.ServeHTTP(w, r)
	})
}

const (
	secureUploadsPrefix          = "/api/files/secure_uploads/"
	delegatedSecureUploadsPrefix = "/api/files/delegated/secure_uploads/"
)

func redactedTokenPath(path string) (string, bool) {
	if !strings.Contains(path, "secure_uploads") {
		return "", false
	}
	for _, prefix := range []string{secureUploadsPrefix, delegatedSecureUploadsPrefix} {
		if rest, ok := strings.CutPrefix(path, prefix); ok {
			_, tail, _ := strings.Cut(rest, "/")
			if idx, isChunk := strings.CutPrefix(tail, "chunks/"); tail == "" || isChunk && idx != "" && !strings.Contains(idx, "/") {
				return prefix + "{token}" + strings.TrimSuffix("/"+tail, "/"), true
			}
			return prefix + "{redacted}", true
		}
	}
	return "/api/files/{redacted}", true
}

func spanName(_ string, r *http.Request) string {
	if _, ok := rpc.AudienceOf(r.URL.Path); ok {
		return r.URL.Path
	}
	return r.Method
}

type sellerStoreNames struct{ m *seller.Module }

func (s sellerStoreNames) StoreNames(ctx context.Context, storeIDs []int64) (map[int64]string, error) {
	cards, err := s.m.StoreCards(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(cards))
	for _, c := range cards {
		if c.Name != nil {
			out[c.StoreID] = *c.Name
		}
	}
	return out, nil
}
