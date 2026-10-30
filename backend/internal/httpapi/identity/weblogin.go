package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// web 的聯合登入入口（ADR 2026-10-30-identity-self-hosted-federated-login D3）：純 HTTP 的 start 與 callback，每家 provider 各一組路徑
// （防 mix-up），經 web 的同源反代到達單體。不走 Connect，也不經動作性質宣告：兩條路由都只收 GET，不改變既有帳號的狀態，
// 登入本身是新建 session 與（首次登入時）帳號。

const (
	// AuthPathPrefix 是所有 web 登入路由的前綴。
	AuthPathPrefix = "/api/auth/"

	// LoginCookie 是 start 設的短效登入前 cookie（防 login CSRF）：值是登入流程的 binding，callback 比對它與暫存列的雜湊。
	LoginCookie = "__Host-hk_login"
	// loginCookieMaxAge 與登入流程的暫存列同為 10 分鐘（D3）；逾時的判斷以暫存列為準，cookie 的壽命只是少留垃圾。
	loginCookieMaxAge = 10 * time.Minute

	// loginPagePath 是 website 的登入頁；失敗時帶 error 代碼回到那裡。
	loginPagePath = "/login"
	// providerTimeout 是一次 callback 對 provider 的外部呼叫的時間上限（換 token、抓公鑰）。
	providerTimeout = 20 * time.Second
)

// 回到登入頁時帶的 error 代碼：只有這些固定值，不反映 provider 回的文字。
const (
	ErrorAccessDenied = "access_denied"
	ErrorInvalidFlow  = "invalid_flow"
	ErrorProvider     = "provider_error"
	ErrorLoginFailed  = "login_failed"
	ErrorUnavailable  = "unavailable"
)

// webLoginModule 是 web 登入需要的 identity 方法；*identity.Module 滿足它。
type webLoginModule interface {
	BeginLoginFlow(ctx context.Context, in identity.LoginFlowInput) (identity.LoginFlowStart, error)
	ConsumeLoginFlow(ctx context.Context, state, binding string, want identity.Provider) (identity.LoginFlow, error)
	Login(ctx context.Context, in identity.LoginInput) (identity.LoginResult, error)
}

// WebProvider 是一家 provider 的 web 登入；*identity.WebLogin 滿足它。
type WebProvider interface {
	Provider() identity.Provider
	AuthURL(state, nonce, verifier string) string
	CheckIssuer(iss string) bool
	Exchange(ctx context.Context, code, verifier, wantNonce string) (identity.ProviderIdentity, error)
}

// WebLoginDeps 是 NewWebLogin 的依賴。
type WebLoginDeps struct {
	Module webLoginModule
	// Providers 是已設定的 provider；沒有設定的 provider，它的 start 與 callback 回 404。
	Providers []WebProvider
	// SessionMaxAge 是 session cookie 的 Max-Age；零值是 session cookie（瀏覽器關閉即失效）。伺服器端的閒置與絕對逾時不受它影響。
	SessionMaxAge time.Duration
	Logger        *slog.Logger
}

// WebLogin 是 web 登入的路由。
type WebLogin struct {
	d         WebLoginDeps
	providers map[identity.Provider]WebProvider
}

// NewWebLogin 建立 web 登入的路由；同一家 provider 設定兩次是錯誤。
func NewWebLogin(d WebLoginDeps) (*WebLogin, error) {
	if d.Module == nil {
		return nil, errors.New("httpapi：web 登入需要 identity 模組")
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	w := &WebLogin{d: d, providers: map[identity.Provider]WebProvider{}}
	for _, p := range d.Providers {
		if !p.Provider().Valid() {
			return nil, fmt.Errorf("httpapi：web 登入的 provider %q 不是已知的值", p.Provider())
		}
		if _, dup := w.providers[p.Provider()]; dup {
			return nil, fmt.Errorf("httpapi：web 登入的 provider %q 設定了兩次", p.Provider())
		}
		w.providers[p.Provider()] = p
	}
	return w, nil
}

// Route 是一條 web 登入路由：Pattern 含 method（net/http 的樣式）。
type Route struct {
	Pattern string
	Handler http.Handler
}

// PathOf 回傳 provider 在路徑上的名稱（google、line）。
func PathOf(p identity.Provider) string { return strings.ToLower(string(p)) }

// Routes 回傳所有 provider 的 start 與 callback，每家獨立的路徑；沒有設定的 provider 也有路由（回 404）。
func (w *WebLogin) Routes() []Route {
	var routes []Route
	for _, p := range identity.LoginProviders() {
		base := AuthPathPrefix + PathOf(p)
		routes = append(routes,
			Route{"GET " + base + "/start", w.guard(p, w.start)},
			Route{"GET " + base + "/callback", w.guard(p, w.callback)},
		)
	}
	return routes
}

// guard 套上所有登入回應共通的 header，只收 GET，沒設定的 provider 回 404。
func (w *WebLogin) guard(p identity.Provider, h func(http.ResponseWriter, *http.Request, WebProvider)) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		header := rw.Header()
		header.Set("Cache-Control", "no-store")
		header.Set("Content-Security-Policy", "frame-ancestors 'none'")
		if r.Method != http.MethodGet {
			header.Set("Allow", http.MethodGet)
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		provider, ok := w.providers[p]
		if !ok {
			http.NotFound(rw, r)
			return
		}
		h(rw, r, provider)
	})
}

func (w *WebLogin) start(rw http.ResponseWriter, r *http.Request, p WebProvider) {
	returnTo := r.URL.Query().Get("return_to")
	if !identity.ValidReturnPath(returnTo) {
		returnTo = "/"
	}
	flow, err := w.d.Module.BeginLoginFlow(r.Context(), identity.LoginFlowInput{Provider: p.Provider(), ReturnPath: returnTo})
	if err != nil {
		w.d.Logger.ErrorContext(r.Context(), "httpapi：建立登入流程失敗", "provider", p.Provider(), "error", err)
		redirectToLogin(rw, r, ErrorUnavailable)
		return
	}
	http.SetCookie(rw, &http.Cookie{
		Name: LoginCookie, Value: flow.Binding, Path: "/", MaxAge: int(loginCookieMaxAge / time.Second),
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(rw, r, p.AuthURL(flow.State, flow.Nonce, flow.CodeVerifier), http.StatusFound)
}

func (w *WebLogin) callback(rw http.ResponseWriter, r *http.Request, p WebProvider) {
	ctx, cancel := context.WithTimeout(r.Context(), providerTimeout)
	defer cancel()
	log := w.d.Logger.With("provider", p.Provider())
	// 不論結果，登入前 cookie 到此為止：暫存列已是單次使用。
	http.SetCookie(rw, expiredCookie(LoginCookie))

	q := r.URL.Query()
	state := q.Get("state")
	binding := ""
	if c, err := r.Cookie(LoginCookie); err == nil {
		binding = c.Value
	}
	flow, err := w.d.Module.ConsumeLoginFlow(ctx, state, binding, p.Provider())
	if providerError := q.Get("error"); providerError != "" {
		// 使用者在 provider 拒絕授權或 provider 出錯：流程已作廢（上面的 consume），不看 flow 是否成立。
		code := ErrorProvider
		if providerError == "access_denied" {
			code = ErrorAccessDenied
		}
		log.InfoContext(ctx, "httpapi：provider 回報登入失敗", "error", providerError)
		redirectToLogin(rw, r, code)
		return
	}
	if err != nil {
		if apperr.KindOf(err) == apperr.KindUnauthenticated {
			redirectToLogin(rw, r, ErrorInvalidFlow)
			return
		}
		log.ErrorContext(ctx, "httpapi：取出登入流程失敗", "error", err)
		redirectToLogin(rw, r, ErrorUnavailable)
		return
	}
	// 手動連結的入口不在這裡；帶連結帳號的流程不由這個 callback 處理。
	if flow.LinkAccountID != nil {
		log.WarnContext(ctx, "httpapi：登入流程帶了連結帳號，這個 callback 不處理")
		redirectToLogin(rw, r, ErrorInvalidFlow)
		return
	}
	// RFC 9207：provider 回應帶 iss 時必須是這家的 issuer。
	if q.Has("iss") && !p.CheckIssuer(q.Get("iss")) {
		log.WarnContext(ctx, "httpapi：授權回應的 iss 不是這家 provider")
		redirectToLogin(rw, r, ErrorInvalidFlow)
		return
	}
	code := q.Get("code")
	if code == "" {
		redirectToLogin(rw, r, ErrorInvalidFlow)
		return
	}
	id, err := p.Exchange(ctx, code, flow.CodeVerifier, flow.Nonce)
	if err != nil {
		switch {
		case errors.Is(err, identity.ErrInvalidToken), errors.Is(err, identity.ErrRejected):
			log.WarnContext(ctx, "httpapi：provider 的 code 或 ID token 無效", "error", err)
			redirectToLogin(rw, r, ErrorInvalidFlow)
		default:
			log.ErrorContext(ctx, "httpapi：向 provider 換 token 失敗", "error", err)
			redirectToLogin(rw, r, ErrorUnavailable)
		}
		return
	}
	// 請求若帶著舊的 session cookie，登入換發新 session 時一併刪除它（session fixation）。
	previous := ""
	if token, source := rpc.Credential(r); source == rpc.CredentialCookie {
		previous = token
	}
	res, err := w.d.Module.Login(ctx, identity.LoginInput{Identity: id, Client: identity.ClientWeb, PreviousToken: previous})
	if err != nil {
		if apperr.KindOf(err) == apperr.KindUnauthenticated || apperr.KindOf(err) == apperr.KindInvalidInput {
			log.WarnContext(ctx, "httpapi：這個身分不能登入", "error", err)
			redirectToLogin(rw, r, ErrorLoginFailed)
			return
		}
		log.ErrorContext(ctx, "httpapi：登入失敗", "error", err)
		redirectToLogin(rw, r, ErrorUnavailable)
		return
	}
	http.SetCookie(rw, w.sessionCookie(res.Token))
	http.Redirect(rw, r, flow.ReturnPath, http.StatusFound)
}

func (w *WebLogin) sessionCookie(token string) *http.Cookie {
	c := &http.Cookie{
		Name: rpc.SessionCookie, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	}
	if w.d.SessionMaxAge > 0 {
		c.MaxAge = int(w.d.SessionMaxAge / time.Second)
	}
	return c
}

func expiredCookie(name string) *http.Cookie {
	return &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}

func redirectToLogin(rw http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(rw, r, loginPagePath+"?"+url.Values{"error": {code}}.Encode(), http.StatusFound)
}
