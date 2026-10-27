// Package httpapi 是單體對外的 HTTP 入口。
//
// 這一層只做傳輸與畫面需要的讀取組裝，業務規則一律在模組裡。對外的 API 是根目錄 api/ 的
// Protobuf＋Connect（ADR docs/adr/2026-09-26-external-contract-protobuf-connect.md），慣例見
// docs/開發規範/API 設計規範（Connect）.md；共用的傳輸元件在 rpc，各模組的 handler 在同名的子套件。
package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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
	httpagent "github.com/Will413028/hawkr/backend/internal/httpapi/agent"
	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	httpcontent "github.com/Will413028/hawkr/backend/internal/httpapi/content"
	httpdiscovery "github.com/Will413028/hawkr/backend/internal/httpapi/discovery"
	adminbuyerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1/buyerv1alpha1connect"
	admincontentconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1/contentv1alpha1connect"
	adminidentityconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1/identityv1alpha1connect"
	adminreviewqueueconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/reviewqueue/v1alpha1/reviewqueuev1alpha1connect"
	adminsellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1/agentv1alpha1connect"
	buyerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1/buyerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1/contentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/discovery/v1alpha1/discoveryv1alpha1connect"
	identityconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	sellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	httpidentity "github.com/Will413028/hawkr/backend/internal/httpapi/identity"
	httpreviewqueue "github.com/Will413028/hawkr/backend/internal/httpapi/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

// Deps 是 handler 需要的外部依賴，全部在組裝點提供。
type Deps struct {
	// Ready 回報這個 instance 是否能服務請求（例如資料庫連得上）。
	Ready      func(context.Context) error
	Content    *content.Module
	Buyer      *buyer.Module
	BuyerPorts httpbuyer.Ports
	Deletion   *accountdeletion.Module
	Delegation *delegation.Module
	Discovery  *discovery.Module
	// ReviewQueue 是後台待辦（ReviewQueueAdminService）；時鐘用 Now。
	ReviewQueue *reviewqueue.Module
	Identity    *identity.Module
	Seller      *seller.Module
	// Agent 是 vendor portal 的邀請與 session（VendorPortalPublicService、VendorPortalInviteeService）；Google 的 step-up 經 Identity。
	Agent *agent.Module
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
	// Origins 是自己的 web 與後台的 origin：CORS 允許它們帶 cookie 呼叫，CSRF 信任它們。web 在各環境都經第一方同源
	// 反代打單體，只有選擇跨來源的開發模式用得到。
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

// maxRequestBytes 是單一請求的大小上限（API 設計規範（Connect）§十）：模組的文字欄位不設長度上限，
// 大小由這裡擋，與 gateway 的 BodyGuard 一般路由相同。
const maxRequestBytes = 2 << 20

// compressMinBytes 以下的回應不壓縮：壓縮的額外負擔大於省下的傳輸量。
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
	if deps.Agent == nil {
		return nil, fmt.Errorf("httpapi：vendor portal 的 service 需要 Agent")
	}
	if deps.ReviewQueue == nil {
		return nil, fmt.Errorf("httpapi：ReviewQueueAdminService 需要 ReviewQueue")
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Logger == nil {
		deps.Logger = slog.New(slog.DiscardHandler)
	}
	opts := []connect.HandlerOption{
		connect.WithInterceptors(rpc.NewInterceptor(deps.Logger), adminauth.NewInterceptor(deps.Logger)),
		connect.WithReadMaxBytes(maxRequestBytes),
		connect.WithCompressMinBytes(compressMinBytes),
	}
	services, err := connectServices(deps, opts...)
	if err != nil {
		return nil, err
	}
	return assemble(deps, services, fileRoutes(deps)...)
}

// fileRoute 是不經 Connect 的檔案回應（API 設計規範（Connect）§十三）：樣式含 method，掛在 /api/files/ 底下，與 Connect 同一條入口
// 中介鏈與限流，授權由 handler 自己做。
type fileRoute struct {
	pattern string
	handler http.Handler
	// permission 是後台路由（/api/files/admin/…）宣告的權限（platform/adminperm）；後台路由必須有，其他路由不得有。
	// handler 之前先檢查呼叫者的角色允許它（adminauth.GuardFile），模組自己的授權在那之後。
	permission adminperm.Permission
}

// fileRoutes 是單體的檔案回應：後台讀 KYC 證件圖片（限時授權、每次讀取寫稽核）與治理佐證（通報、措施上請求與詢問回覆的附件，每次讀取寫紀錄）。
func fileRoutes(deps Deps) []fileRoute {
	if deps.Seller == nil {
		return nil
	}
	return []fileRoute{{
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

// assemble 把 service 掛上入口的中介層：tracing、CORS、CSRF、session、限流的身分，以及 Connect 的限流、快取規則
// 與 interceptor 之前的錯誤 log。測試以它掛上假的 service，驗證中介層本身的性質（例如串流不被緩衝），不必依賴
// 某個正式的 service。
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
	if deps.Legacy != nil {
		// 管理員的請求要有 legacy.gateway.forward 才轉發（ADR 2026-10-04-admin-roles）；其他呼叫者原樣放行。
		legacy := adminauth.GuardLegacy(deps.Legacy, deps.Logger)
		for _, prefix := range deps.LegacyPrefixes {
			mux.Handle(prefix, legacy)
		}
	}
	var h http.Handler = mux
	h = withSubject(deps.OriginAuth, deps.ClientIP, deps.Now, h)
	h = withSession(deps.Identity, deps.Logger, h)
	h = csrf.Handler(h)
	h = withCORS(deps.Origins, h)
	return withTracing(deps.Tracer, h), nil
}

type service struct {
	path    string
	handler http.Handler
}

// connectServices 是單體掛的 Connect service：根目錄 api/ 宣告的每個 service 都要在這裡
// （TestEveryServiceInTheContractIsMounted）。入口在單體，gateway 不再代理 Connect（遷移計畫 A2）。
func connectServices(deps Deps, opts ...connect.HandlerOption) ([]service, error) {
	var out []service
	add := func(path string, h http.Handler) { out = append(out, service{path, h}) }
	// 店面頁、賣家自己的店與所有清單上的店家卡片共用同一份統計組裝（deps.Stats，必填）。
	stats := deps.Stats
	add(contentv1alpha1connect.NewContentPublicServiceHandler(httpcontent.NewPublic(deps.Content, deps.Images), opts...))
	add(admincontentconnect.NewContentAdminServiceHandler(httpcontent.NewAdmin(deps.Content, deps.Images), opts...))
	buyerApp, err := httpbuyer.NewApp(deps.Buyer, deps.Images, deps.BuyerPorts, stats)
	if err != nil {
		return nil, err
	}
	add(buyerconnect.NewBuyerAppServiceHandler(buyerApp, opts...))
	add(adminbuyerconnect.NewBuyerAdminServiceHandler(httpbuyer.NewAdmin(deps.Buyer), opts...))
	add(identityconnect.NewIdentityAppServiceHandler(httpidentity.NewApp(deps.Identity, deps.Deletion), opts...))
	add(adminidentityconnect.NewIdentityAdminServiceHandler(httpidentity.NewAdmin(deps.Identity, deps.Delegation, deps.Deletion), opts...))
	// 賣家區每個方法的能力宣告由這支 interceptor 執行，只掛在 SellerAppService、在 rpc.NewInterceptor 之內；宣告與 service
	// 的方法對不上時不能啟動。
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
	sellerApp, err := httpseller.NewApp(deps.Seller, deps.Buyer, deps.Images, dashboard, stats)
	if err != nil {
		return nil, err
	}
	add(adminsellerconnect.NewSellerAdminServiceHandler(httpseller.NewAdmin(deps.Seller, deps.Images), opts...))
	add(sellerconnect.NewSellerPublicServiceHandler(sellerPublic, opts...))
	add(sellerconnect.NewSellerAppServiceHandler(sellerApp, append(slices.Clone(opts), connect.WithInterceptors(acting))...))
	add(adminsellerconnect.NewSellerGovernanceAdminServiceHandler(httpseller.NewGovernance(deps.Seller, deps.Images), opts...))
	// 通報人是買家或匿名者，不是店家：這兩個 service 不掛賣家能力的 interceptor。
	reports, err := httpseller.NewReports(deps.Seller, deps.Buyer, accounts)
	if err != nil {
		return nil, err
	}
	add(sellerconnect.NewSellerReportAppServiceHandler(reports, opts...))
	add(sellerconnect.NewSellerReportPublicServiceHandler(reports, opts...))
	add(adminsellerconnect.NewSellerProspectAdminServiceHandler(httpseller.NewProspects(deps.Seller, deps.Images), opts...))
	add(adminreviewqueueconnect.NewReviewQueueAdminServiceHandler(httpreviewqueue.NewAdmin(deps.ReviewQueue, deps.Now), opts...))
	// 受邀供應商的 service 以請求的 rfq_recipient_id 選 cookie 並驗證 session，由這支 interceptor 執行；只掛在
	// VendorPortalInviteeService、在 rpc.NewInterceptor 之內。
	invitee, err := httpagent.NewInterceptor(deps.Agent)
	if err != nil {
		return nil, err
	}
	add(agentv1alpha1connect.NewVendorPortalPublicServiceHandler(httpagent.NewPublic(deps.Agent, deps.Now), opts...))
	add(agentv1alpha1connect.NewVendorPortalInviteeServiceHandler(httpagent.NewInvitee(deps.Agent, deps.Identity, deps.Logger), append(slices.Clone(opts), connect.WithInterceptors(invitee))...))
	if err := checkAdminDeclarations(out); err != nil {
		return nil, err
	}
	return out, nil
}

// adminServiceDescriptors 是掛載的 service 中 audience 為 admin 的 descriptor，由路徑上的 service 全名從 protoregistry 查。
// 查不到的 admin service 回錯誤：沒有 descriptor 就無法確認它的方法都有授權宣告。
func adminServiceDescriptors(services []service) ([]protoreflect.ServiceDescriptor, error) {
	var out []protoreflect.ServiceDescriptor
	for _, svc := range services {
		if audience, _ := rpc.AudienceOf(svc.path); audience != rpc.AudienceAdmin {
			continue
		}
		name := protoreflect.FullName(strings.Trim(svc.path, "/"))
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(name)
		if err != nil {
			return nil, fmt.Errorf("httpapi：找不到 admin service %s 的 descriptor：%w", name, err)
		}
		sd, ok := d.(protoreflect.ServiceDescriptor)
		if !ok {
			return nil, fmt.Errorf("httpapi：%s 不是 service", name)
		}
		out = append(out, sd)
	}
	return out, nil
}

// checkAdminDeclarations 對照 adminauth 的宣告表與實際掛載的 admin service（ADR 2026-10-04-admin-roles：宣告＝掛載）：
// 掛了沒有宣告的 admin 方法、或宣告了沒有掛載的方法，都不能啟動。授權由掛在每個 service 的 adminauth interceptor 執行，
// 新增的 admin service 不必自己選用它。
func checkAdminDeclarations(services []service) error {
	descriptors, err := adminServiceDescriptors(services)
	if err != nil {
		return err
	}
	return adminauth.Check(descriptors)
}

// mount 掛上一個 Connect service，並依它的使用者類別加上 HTTP 的快取規則。授權跟著 service 的名稱
// 走（rpc.AudienceOf），名稱不符合規則的 service 在組裝時就 panic，不會以沒有授權的狀態上線。
func mount(mux *http.ServeMux, path string, h http.Handler) {
	audience, ok := rpc.AudienceOf(path)
	if !ok {
		panic(fmt.Sprintf("httpapi：%s 不符合 service 的命名規則（<模組>PublicService、<模組>AppService、<模組>InviteeService、hawkr.admin.* 的 <模組>AdminService）", path))
	}
	mux.Handle(path, cachePolicy(audience, h))
}

// withTracing 為每個請求開一個新的 trace：單體是入口，不接續 client 帶來的 traceparent（client 不可信，而且同一頁
// 的多個請求會共用 trace id，legacy-backlog 的 L265）。轉給 gateway 時由 gatewayproxy 注入 traceparent，兩邊的 log
// 以同一個 trace id 對得起來。trace id 寫在回應的 X-Trace-Id（API 設計規範（Connect）§八：不放進錯誤內容），
// interceptor 的 log 也帶著它。
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
	return otelhttp.NewHandler(h, "hawkr",
		otelhttp.WithTracerProvider(tp),
		otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator()),
		otelhttp.WithSpanNameFormatter(spanName))
}

// spanName 以 procedure 命名 Connect 的請求；其他路徑只用 method，掃描器亂打的路徑不會變成大量的 span 名稱。
func spanName(_ string, r *http.Request) string {
	if _, ok := rpc.AudienceOf(r.URL.Path); ok {
		return r.URL.Path
	}
	return r.Method
}
