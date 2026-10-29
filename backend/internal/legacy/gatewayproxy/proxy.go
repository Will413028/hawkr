// Package gatewayproxy 把還沒搬進單體的 REST、SSE 與 WebSocket 轉給舊的 gateway（遷移計畫 A2：入口從 gateway
// 移到單體，ADR 2026-09-26-external-contract-protobuf-connect D6），並帶上入口解析的呼叫者業務身分（ADR
// 2026-09-29-seller-suspension-and-business-identity-in-monolith）。最後一個模組搬完、gateway 刪除時一起刪除。
//
// 認證在單體的入口完成：gateway 不再讀 cookie 或 Bearer，改成信任這裡帶的身分 header（API 設計規範（Connect）§二
// 的「單體→gateway 的身分」）。client 自己帶來的身分 header 一律先刪掉。
package gatewayproxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/Will413028/hawkr/backend/internal/legacy/accounttype"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerreason"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Prefixes 是轉給 gateway 的路徑：舊的 REST（/api/v1/），以及金流、物流的回跳、webhook 與網域驗證檔這些本來就
// 在根目錄的路由。
var Prefixes = []string{"/api/v1/", "/webhook/", "/payment/", "/store_map/", "/atm-refund/", "/.well-known/"}

// 身分 header（gateway 讀它們重建身分）。X-Active-Role 由單體解析成 X-Login-Role，不往下傳。
const (
	HeaderAccountID       = "X-Account-Id"
	HeaderAccountType     = "X-Account-Type"
	HeaderLoginRole       = "X-Login-Role"
	HeaderBuyerID         = "X-Buyer-Id"
	HeaderStoreID         = "X-Store-Id"
	HeaderSellerSuspended = "X-Seller-Suspended"
	// HeaderSellerCapabilities 是 seller 模組推導的現任店家能力（逗號分隔的能力 token，與 X-Store-Id 同時出現）；gateway
	// 只檢查成員資格，不自己從停權或生命週期推導。
	HeaderSellerCapabilities = "X-Seller-Capabilities"
	headerActiveRole         = "X-Active-Role"
)

// vendor portal 的 header（gateway 讀它們重建供應商 session 的身分，過渡到 A9 把 RFQ 路由改成 Connect 為止）。入口對每個轉發的
// 請求先刪掉所有以 VendorPortalHeaderPrefix 開頭的 header（不分大小寫），只在 VendorPortalRFQPrefix 底下、路徑的收件人 ID 那張邀請的
// cookie 是有效 session 而且 session 的收件人就是那個 ID 時設上（路徑沒有合法的 ID、別張邀請的 cookie 都不設）。
const (
	VendorPortalHeaderPrefix = "X-Vendor-Portal-"
	// VendorPortalRFQPrefix 是需要供應商 session 的 gateway 路由。
	VendorPortalRFQPrefix = "/api/v1/public/vendor_portal/rfqs/"

	HeaderVendorRfqRecipientID     = "X-Vendor-Portal-Rfq-Recipient-Id"
	HeaderVendorSessionID          = "X-Vendor-Portal-Session-Id"
	HeaderVendorVerificationMethod = "X-Vendor-Portal-Verification-Method"
	HeaderVendorVerifiedEmail      = "X-Vendor-Portal-Verified-Email"
	HeaderVendorVerifiedAt         = "X-Vendor-Portal-Verified-At"
)

// 連 gateway 的逾時。SSE 與 WebSocket 會長時間佔著連線，所以不設整體的逾時，只限制連線與等到回應 header 的時間。
const (
	dialTimeout           = 5 * time.Second
	responseHeaderTimeout = 60 * time.Second
)

// Caller 是入口認證過的呼叫者。組裝點從入口的 ctx 取出它（httpapi 的 rpc.PrincipalFrom），legacy 因此不依賴傳輸層。
type Caller struct {
	Principal       identity.Principal
	BuyerID         int64
	StoreID         int64
	SellerSuspended string
	// SellerCapabilities 是現任店家目前啟用的動作類能力（seller 模組推導，含 KYC 流程的前置條件）。
	SellerCapabilities []seller.Capability
}

// CallerOf 從請求的 ctx 取出呼叫者；沒有登入時回傳 false。
type CallerOf func(context.Context) (Caller, bool)

// BuyerLookup 讀取帳號的現任 buyer_id；buyer 模組滿足它，找不到時回 apperr 的 NotFound。
type BuyerLookup interface {
	CurrentBuyerID(context.Context, int64) (int64, error)
}

// SellerLookup 讀取帳號現任店家與它的停權狀態和生效中的措施（同一次讀取）；seller 模組滿足它，沒有現任店家（沒有或已關閉）時回
// apperr 的 NotFound。
type SellerLookup interface {
	CurrentStore(context.Context, int64) (seller.Store, error)
}

// suspensionCode 是現任店家停權在舊契約 X-Seller-Suspended 的值：未停權是空字串，依據風險類型的停權（舊契約的管理員停權）是
// 原因碼 9603，只依據認定的停權（舊契約的違規停權）是 SUSPENDED_GOVERNANCE；沒有推導過停權狀態、不認得的措施都回錯誤，
// 不當成有效值往下傳。
func suspensionCode(st seller.Store) (string, error) {
	class, err := sellerreason.Classify(st.Sanctions)
	if err != nil {
		return "", err
	}
	switch class {
	case sellerreason.Governance:
		return "SUSPENDED_GOVERNANCE", nil
	case sellerreason.Admin:
		code, _, err := sellerreason.Of(st.Sanctions)
		return code, err
	case sellerreason.None:
	}
	return "", nil
}

// currentStoreOf 把 seller 模組的讀取轉成 Caller 的店家欄位。能力只放啟用的動作類能力（statuses 是 CapabilitiesOf 的結果；
// 狀態 CapPublicListing 不進 header）。店家與能力要成對：沒有能力的店家（已關閉）不會以現任店家出現，出現時是不變量被破壞，回錯誤。
func currentStoreOf(st seller.Store, statuses []seller.CapabilityStatus, c *Caller) error {
	suspended, err := suspensionCode(st)
	if err != nil {
		return err
	}
	caps := seller.ActiveCapabilities(statuses)
	if st.StoreID <= 0 || len(caps) == 0 {
		return fmt.Errorf("gatewayproxy：現任店家 %d 沒有任何能力（狀態 %q）", st.StoreID, st.Status)
	}
	known := seller.Capabilities()
	for _, cp := range caps {
		if !slices.Contains(known, cp) {
			return fmt.Errorf("gatewayproxy：不認得的店家能力 %q", cp)
		}
	}
	c.StoreID, c.SellerSuspended, c.SellerCapabilities = st.StoreID, suspended, caps
	return nil
}

// capabilitiesHeader 是 X-Seller-Capabilities 的值。seller 的能力名稱與 gateway 的 authz 能力 token 是同一組字串，
// 對應只在這一處（gateway 對照由 TestCapabilityTokensMatchGateway 守）。
func capabilitiesHeader(caps []seller.Capability) string {
	tokens := make([]string, len(caps))
	for i, cp := range caps {
		tokens[i] = string(cp)
	}
	return strings.Join(tokens, ",")
}

// VendorSessionOf 驗證請求帶的、rfqRecipientID 那張邀請的 vendor portal session（cookie）：有效回 (session, true, nil)；沒有
// cookie 或 session 不可用回 (零值, false, nil)，gateway 會回 401；其他錯誤（資料庫等）回 error，入口回 503。
type VendorSessionOf func(r *http.Request, rfqRecipientID int64) (agent.SessionInfo, bool, error)

// vendorRecipientID 解析 VendorPortalRFQPrefix 底下路徑的收件人 ID（路徑的第一段）：只接受正整數的十進位寫法，其他回 false。
func vendorRecipientID(path string) (int64, bool) {
	rest, ok := strings.CutPrefix(path, VendorPortalRFQPrefix)
	if !ok {
		return 0, false
	}
	segment, _, _ := strings.Cut(rest, "/")
	id, err := strconv.ParseInt(segment, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != segment {
		return 0, false
	}
	return id, true
}

type forwarder struct {
	proxy    *httputil.ReverseProxy
	callerOf CallerOf
	vendors  VendorSessionOf
	buyers   BuyerLookup
	sellers  SellerLookup
	log      *slog.Logger
}

type resolvedCallerKey struct{}

type vendorSessionKey struct{}

func (f *forwarder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, ok := f.callerOf(r.Context())
	c.BuyerID, c.StoreID, c.SellerSuspended, c.SellerCapabilities = 0, 0, "", nil
	var err error
	if ok && c.Principal.Kind == identity.PrincipalAccount {
		// 每個帳號都是買家：gateway 以 X-Buyer-Id 缺省表示「沒有買家」、不再查 RPC，所以每個轉發的帳號請求都要解析。
		c.BuyerID, err = f.buyers.CurrentBuyerID(r.Context(), c.Principal.ID)
		if apperr.KindOf(err) == apperr.KindNotFound {
			// 帳號活著 ⇔ 買家活著（ADR 2026-09-30-account-created-provisions-buyer）：認證過的帳號沒有現任買家是
			// 不變量被破壞，不以「沒有買家」轉發。
			f.log.ErrorContext(r.Context(), "gatewayproxy：存活的帳號沒有現任買家", "account_id", c.Principal.ID, "path", r.URL.Path)
			internalError(w)
			return
		}
		if err == nil && c.Principal.HasRole(identity.RoleSeller) {
			var st seller.Store
			st, err = f.sellers.CurrentStore(r.Context(), c.Principal.ID)
			if apperr.KindOf(err) == apperr.KindNotFound {
				// SELLER 角色 ⇔ 有現任店家（關店在同一個 transaction 移除角色）：沒有現任店家是不變量被破壞，不以「沒有店家」轉發。
				f.log.ErrorContext(r.Context(), "gatewayproxy：有賣家角色的帳號沒有現任店家", "account_id", c.Principal.ID, "path", r.URL.Path)
				internalError(w)
				return
			}
			if err == nil {
				err = currentStoreOf(st, seller.CapabilitiesOf(st), &c)
			}
		}
	}
	if err != nil {
		if r.Context().Err() != nil {
			return // client 自己斷線
		}
		f.log.ErrorContext(r.Context(), "gatewayproxy：解析業務身分失敗", "path", r.URL.Path, "error", err)
		unavailable(w)
		return
	}
	if !ok {
		c = Caller{}
	}
	ctx := context.WithValue(r.Context(), resolvedCallerKey{}, c)
	if id, ok := vendorRecipientID(r.URL.Path); ok {
		info, valid, err := f.vendors(r, id)
		if err != nil {
			if r.Context().Err() != nil {
				return // client 自己斷線
			}
			f.log.ErrorContext(r.Context(), "gatewayproxy：驗證 vendor portal session 失敗", "path", r.URL.Path, "error", err)
			unavailable(w)
			return
		}
		if valid && info.RfqRecipientID == id {
			ctx = context.WithValue(ctx, vendorSessionKey{}, info)
		}
	}
	f.proxy.ServeHTTP(w, r.WithContext(ctx))
}

func unavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"code":503,"message":"服務暫時無法使用，請稍後再試"}`))
}

func internalError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(`{"code":500,"message":"系統錯誤，請稍後再試"}`))
}

// New 建立轉給 gateway 的 handler。gatewayURL 例如 http://127.0.0.1:9900。
func New(gatewayURL string, callerOf CallerOf, vendors VendorSessionOf, buyers BuyerLookup, sellers SellerLookup, tp trace.TracerProvider, log *slog.Logger) (http.Handler, error) {
	if callerOf == nil || vendors == nil {
		return nil, errors.New("gatewayproxy：需要 CallerOf 與 VendorSessionOf")
	}
	target, err := url.Parse(gatewayURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("gatewayproxy：gateway 的網址 %q 不合法", gatewayURL)
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: dialTimeout}).DialContext,
		ResponseHeaderTimeout: responseHeaderTimeout,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
			c, _ := pr.In.Context().Value(resolvedCallerKey{}).(Caller)
			ok := c.Principal.ID > 0
			SetIdentity(pr.Out.Header, c, ok)
			info, valid := pr.In.Context().Value(vendorSessionKey{}).(agent.SessionInfo)
			SetVendorSession(pr.Out.Header, info, valid)
		},
		// 串流（SSE）逐筆送出。
		FlushInterval: -1,
		// 以 W3C Trace Context 把入口的 trace 接給 gateway。
		Transport: otelhttp.NewTransport(transport,
			otelhttp.WithTracerProvider(tp), otelhttp.WithPropagators(propagation.TraceContext{})),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, context.Canceled) {
				return // client 自己斷線
			}
			log.ErrorContext(r.Context(), "gatewayproxy：轉給 gateway 失敗", "path", r.URL.Path, "error", err)
			unavailable(w)
		},
	}
	return &forwarder{proxy: proxy, callerOf: callerOf, vendors: vendors, buyers: buyers, sellers: sellers, log: log}, nil
}

// SetIdentity 刪掉 h 裡 client 帶來的身分 header，登入時（ok）再依入口認證過的呼叫者與解析的業務身分設上（#1859）。
// 沒有登入時不設，gateway 當成訪客。
func SetIdentity(h http.Header, c Caller, ok bool) {
	// X-Active-Role 是舊 REST client 的工作區提示，先讀出來再連同其他身分 header 一起刪；gateway 只收到解析後的 X-Login-Role。
	hint := h.Get(headerActiveRole)
	for _, k := range []string{HeaderAccountID, HeaderAccountType, HeaderLoginRole, headerActiveRole, HeaderBuyerID, HeaderStoreID, HeaderSellerSuspended, HeaderSellerCapabilities} {
		h.Del(k)
	}
	if !ok {
		return
	}
	h.Set(HeaderAccountID, strconv.FormatInt(c.Principal.ID, 10))
	if c.Principal.Kind == identity.PrincipalAdmin {
		h.Set(HeaderAccountType, strconv.Itoa(accounttype.Admin))
		h.Set(HeaderLoginRole, strconv.Itoa(accounttype.Admin))
		return
	}
	// 買家位元與 X-Buyer-Id 由這裡合成（accounttype）：identity 不把買家存成角色。
	if c.BuyerID > 0 {
		h.Set(HeaderBuyerID, strconv.FormatInt(c.BuyerID, 10))
	}
	if c.Principal.HasRole(identity.RoleSeller) && c.StoreID > 0 {
		h.Set(HeaderStoreID, strconv.FormatInt(c.StoreID, 10))
		h.Set(HeaderSellerCapabilities, capabilitiesHeader(c.SellerCapabilities))
		if c.SellerSuspended != "" {
			h.Set(HeaderSellerSuspended, c.SellerSuspended)
		}
	}
	h.Set(HeaderAccountType, strconv.Itoa(accounttype.OfLiveAccount(c.Principal.Roles)))
	if v := accounttype.OfLoginRole(c.Principal, hint); v != 0 {
		h.Set(HeaderLoginRole, strconv.Itoa(v))
	}
}

// SetVendorSession 刪掉 h 裡 client 帶來的所有 X-Vendor-Portal-* header（header 名稱不分大小寫），入口驗證過 session 時（ok）
// 再設上收件人與 session，session 已驗證時加上驗證紀錄；沒有驗證時不設驗證紀錄的三個 header。
func SetVendorSession(h http.Header, info agent.SessionInfo, ok bool) {
	for k := range h {
		if strings.HasPrefix(strings.ToLower(k), strings.ToLower(VendorPortalHeaderPrefix)) {
			delete(h, k)
		}
	}
	if !ok {
		return
	}
	h.Set(HeaderVendorRfqRecipientID, strconv.FormatInt(info.RfqRecipientID, 10))
	h.Set(HeaderVendorSessionID, info.SessionID.String())
	if v := info.Verification; v != nil {
		h.Set(HeaderVendorVerificationMethod, string(v.Method))
		h.Set(HeaderVendorVerifiedEmail, v.Email)
		h.Set(HeaderVendorVerifiedAt, v.At.UTC().Format(time.RFC3339Nano))
	}
}
