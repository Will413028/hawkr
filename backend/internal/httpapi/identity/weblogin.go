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
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)


const (
	// AuthPathPrefix 是所有 web 登入路由的前綴。
	AuthPathPrefix = "/api/auth/"

	// LoginCookie 是 start 設的短效登入前 cookie（防 login CSRF）：值是登入流程的 binding，callback 比對它與暫存列的雜湊。
	LoginCookie = "__Host-hk_login"
	loginCookieMaxAge = 10 * time.Minute

	loginPagePath = "/login"
	providerTimeout = 20 * time.Second
)

// 回到登入頁時帶的 error 代碼：只有這些固定值，不反映 provider 回的文字。
const (
	ErrorAccessDenied = "access_denied"
	ErrorInvalidFlow  = "invalid_flow"
	ErrorProvider     = "provider_error"
	ErrorLoginFailed  = "login_failed"
	ErrorUnavailable  = "unavailable"
	// ErrorLoginRequired 是要求連結（link=1）卻沒有登入。
	ErrorLoginRequired = "login_required"
)

// 手動連結失敗時，附在返回網址上的 link_error 代碼（成功時返回網址不變）：固定值。
const (
	LinkErrorParam = "link_error"
	// LinkErrorAlreadyLinked 是這個 provider 身分已經連結到另一個帳號（含已刪除帳號留下的）。
	LinkErrorAlreadyLinked = "identity_already_linked"
)

type webLoginModule interface {
	BeginLoginFlow(ctx context.Context, in identity.LoginFlowInput) (identity.LoginFlowStart, error)
	ConsumeLoginFlow(ctx context.Context, state, binding string, want identity.Provider) (identity.LoginFlow, error)
	Login(ctx context.Context, in identity.LoginInput) (identity.LoginResult, error)
	LinkIdentity(ctx context.Context, accountID int64, id identity.ProviderIdentity) error
}

// WebProvider 是一家 provider 的 web 登入；*identity.WebLogin 滿足它。
type WebProvider interface {
	Provider() identity.Provider
	AuthURL(state, nonce, verifier string) string
	CheckIssuer(iss string) bool
	// RequiresIssuer 回報這家 provider 的授權回應一定帶 iss（RFC 9207 §2.4：宣告了就必須帶，沒帶要拒絕）。
	RequiresIssuer() bool
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
	input := identity.LoginFlowInput{Provider: p.Provider(), ReturnPath: returnTo}
	if r.URL.Query().Has("link") {
		caller, ok := rpc.PrincipalFrom(r.Context())
		switch {
		case r.URL.Query().Get("link") != "1":
			redirectToLogin(rw, r, ErrorInvalidFlow)
			return
		case !ok || caller.Kind != identity.PrincipalAccount:
			redirectToLogin(rw, r, ErrorLoginRequired)
			return
		}
		input.LinkAccountID = &caller.ID
	}
	flow, err := w.d.Module.BeginLoginFlow(r.Context(), input)
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
	http.SetCookie(rw, expiredCookie(LoginCookie))

	q := r.URL.Query()
	state := q.Get("state")
	binding := ""
	if c, err := r.Cookie(LoginCookie); err == nil {
		binding = c.Value
	}
	flow, err := w.d.Module.ConsumeLoginFlow(ctx, state, binding, p.Provider())
	fail := func(code string) {
		if err == nil && flow.LinkAccountID != nil {
			redirectLinkError(rw, r, flow.ReturnPath, code)
			return
		}
		redirectToLogin(rw, r, code)
	}
	if providerError := q.Get("error"); providerError != "" {
		code := ErrorProvider
		if providerError == "access_denied" {
			code = ErrorAccessDenied
		}
		log.InfoContext(ctx, "httpapi：provider 回報登入失敗", "error", providerError)
		fail(code)
		return
	}
	if err != nil {
		if apperr.KindOf(err) == apperr.KindUnauthenticated {
			fail(ErrorInvalidFlow)
			return
		}
		log.ErrorContext(ctx, "httpapi：取出登入流程失敗", "error", err)
		fail(ErrorUnavailable)
		return
	}
	if flow.LinkAccountID != nil {
		if caller, ok := rpc.PrincipalFrom(ctx); !ok || caller.Kind != identity.PrincipalAccount || caller.ID != *flow.LinkAccountID {
			log.WarnContext(ctx, "httpapi：連結流程的帳號不是現在登入的帳號")
			fail(ErrorInvalidFlow)
			return
		}
	}
	if q.Has("iss") && !p.CheckIssuer(q.Get("iss")) || !q.Has("iss") && p.RequiresIssuer() {
		log.WarnContext(ctx, "httpapi：授權回應的 iss 缺少或不是這家 provider")
		fail(ErrorInvalidFlow)
		return
	}
	code := q.Get("code")
	if code == "" {
		fail(ErrorInvalidFlow)
		return
	}
	id, err := p.Exchange(ctx, code, flow.CodeVerifier, flow.Nonce)
	if err != nil {
		switch {
		case errors.Is(err, identity.ErrInvalidToken), errors.Is(err, identity.ErrRejected):
			log.WarnContext(ctx, "httpapi：provider 的 code 或 ID token 無效", "error", err)
			fail(ErrorInvalidFlow)
		default:
			log.ErrorContext(ctx, "httpapi：向 provider 換 token 失敗", "error", err)
			fail(ErrorUnavailable)
		}
		return
	}
	if flow.LinkAccountID != nil {
		w.link(ctx, rw, r, log, *flow.LinkAccountID, id, flow.ReturnPath)
		return
	}
	previous := ""
	if token, source := rpc.Credential(r); source == rpc.CredentialCookie {
		previous = token
	}
	res, err := w.d.Module.Login(ctx, identity.LoginInput{Identity: id, Client: identity.ClientWeb, PreviousToken: previous, Locale: returnPathLocale(flow.ReturnPath)})
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

func returnPathLocale(returnPath string) uitext.Locale {
	if !identity.ValidReturnPath(returnPath) {
		return ""
	}
	if rest, ok := strings.CutPrefix(returnPath, "/"+string(uitext.EN)); ok && (rest == "" || strings.ContainsRune("/?#", rune(rest[0]))) {
		return uitext.EN
	}
	return uitext.ZhTW
}

func (w *WebLogin) link(ctx context.Context, rw http.ResponseWriter, r *http.Request, log *slog.Logger, accountID int64, id identity.ProviderIdentity, returnPath string) {
	err := w.d.Module.LinkIdentity(ctx, accountID, id)
	switch {
	case err == nil:
		http.Redirect(rw, r, returnPath, http.StatusFound)
	case apperr.KindOf(err) == apperr.KindAlreadyExists:
		log.InfoContext(ctx, "httpapi：這個身分已連結到另一個帳號")
		redirectLinkError(rw, r, returnPath, LinkErrorAlreadyLinked)
	case apperr.KindOf(err) == apperr.KindNotFound, apperr.KindOf(err) == apperr.KindInvalidInput:
		log.WarnContext(ctx, "httpapi：這個身分不能連結", "error", err)
		redirectLinkError(rw, r, returnPath, ErrorLoginFailed)
	default:
		log.ErrorContext(ctx, "httpapi：連結失敗", "error", err)
		redirectLinkError(rw, r, returnPath, ErrorUnavailable)
	}
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

func redirectLinkError(rw http.ResponseWriter, r *http.Request, returnPath, code string) {
	base, fragment, hasFragment := strings.Cut(returnPath, "#")
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	location := base + separator + url.Values{LinkErrorParam: {code}}.Encode()
	if hasFragment {
		location += "#" + fragment
	}
	http.Redirect(rw, r, location, http.StatusFound) //nolint:gosec // returnPath 是建立流程時已經 ValidReturnPath 驗證的站內相對路徑，存在暫存列；附加的只有編碼過的固定代碼
}
