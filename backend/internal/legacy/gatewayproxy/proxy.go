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
	// SellerCapabilities 是現任店家目前啟用的動作類能力（seller 模組推導，含開通閘門的前置條件）。
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

func capabilitiesHeader(caps []seller.Capability) string {
	tokens := make([]string, len(caps))
	for i, cp := range caps {
		tokens[i] = string(cp)
	}
	return strings.Join(tokens, ",")
}

type forwarder struct {
	proxy    *httputil.ReverseProxy
	callerOf CallerOf
	buyers   BuyerLookup
	sellers  SellerLookup
	log      *slog.Logger
}

type resolvedCallerKey struct{}

func (f *forwarder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, ok := f.callerOf(r.Context())
	c.BuyerID, c.StoreID, c.SellerSuspended, c.SellerCapabilities = 0, 0, "", nil
	var err error
	if ok && c.Principal.Kind == identity.PrincipalAccount {
		c.BuyerID, err = f.buyers.CurrentBuyerID(r.Context(), c.Principal.ID)
		if apperr.KindOf(err) == apperr.KindNotFound {
			f.log.ErrorContext(r.Context(), "gatewayproxy：存活的帳號沒有現任買家", "account_id", c.Principal.ID, "path", r.URL.Path)
			internalError(w)
			return
		}
		if err == nil && c.Principal.HasRole(identity.RoleSeller) {
			var st seller.Store
			st, err = f.sellers.CurrentStore(r.Context(), c.Principal.ID)
			if apperr.KindOf(err) == apperr.KindNotFound {
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
			return
		}
		f.log.ErrorContext(r.Context(), "gatewayproxy：解析業務身分失敗", "path", r.URL.Path, "error", err)
		unavailable(w)
		return
	}
	if !ok {
		c = Caller{}
	}
	ctx := context.WithValue(r.Context(), resolvedCallerKey{}, c)
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
func New(gatewayURL string, callerOf CallerOf, buyers BuyerLookup, sellers SellerLookup, tp trace.TracerProvider, log *slog.Logger) (http.Handler, error) {
	if callerOf == nil {
		return nil, errors.New("gatewayproxy：需要 CallerOf")
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
			StripSessionCredentials(pr.Out.Header)
			SetIdentity(pr.Out.Header, c, ok)
		},
		FlushInterval: -1,
		Transport: otelhttp.NewTransport(transport,
			otelhttp.WithTracerProvider(tp), otelhttp.WithPropagators(propagation.TraceContext{})),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.ErrorContext(r.Context(), "gatewayproxy：轉給 gateway 失敗", "path", r.URL.Path, "error", err)
			unavailable(w)
		},
	}
	return &forwarder{proxy: proxy, callerOf: callerOf, buyers: buyers, sellers: sellers, log: log}, nil
}

// SessionCookie 是單體 session cookie 的名稱，必須等於 rpc.SessionCookie（legacy 不依賴 httpapi，httpapi 的 TestGatewayProxyStripsTheSessionCookieName 守住兩者相同）。
const SessionCookie = "__Host-hk_session"

// StripSessionCredentials 刪掉 h 裡單體的 session 憑證：Authorization 整個刪除，Cookie 只移除 session cookie（SessionCookie），其他 cookie
// 原樣保留。gateway 只信單體解析出的身分 header（SetIdentity），不收原始憑證（ADR 2026-10-30-identity-self-hosted-federated-login D1）。
func StripSessionCredentials(h http.Header) {
	h.Del("Authorization")
	lines := h.Values("Cookie")
	if len(lines) == 0 {
		return
	}
	h.Del("Cookie")
	for _, line := range lines {
		var kept []string
		for part := range strings.SplitSeq(line, ";") {
			name, _, _ := strings.Cut(strings.TrimSpace(part), "=")
			if strings.TrimSpace(name) == SessionCookie {
				continue
			}
			if strings.TrimSpace(part) != "" {
				kept = append(kept, strings.TrimSpace(part))
			}
		}
		if len(kept) > 0 {
			h.Add("Cookie", strings.Join(kept, "; "))
		}
	}
}

// SetIdentity 刪掉 h 裡 client 帶來的身分 header，登入時（ok）再依入口認證過的呼叫者與解析的業務身分設上（#1859）。
// 沒有登入時不設，gateway 當成訪客。
func SetIdentity(h http.Header, c Caller, ok bool) {
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
