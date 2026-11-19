package rpc

import (
	"context"
	"net/http"
	"strings"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// SessionCookie 是 web 的 session cookie 的名稱（ADR 2026-10-30-identity-self-hosted-federated-login D1）：單體在登入的 callback 設定，值是
// opaque session token。`__Host-` 前綴要求 Secure、Path=/、沒有 Domain，cookie 因此只落在 web 的 host（web 經第一方同源反代打單體）。
const SessionCookie = "__Host-hk_session"

// CredentialSource 是請求的憑證從哪裡來；CSRF 的檢查只針對 ambient 的 cookie（ADR 同上 D6），Bearer 要呼叫方自己帶，不檢查。
type CredentialSource int

const (
	// CredentialNone 是請求沒有帶憑證。
	CredentialNone CredentialSource = iota
	// CredentialBearer 是 Authorization: Bearer。
	CredentialBearer
	// CredentialCookie 是 SessionCookie。
	CredentialCookie
)

// Credential 回傳請求帶的 session token 與它的來源：只收 `Authorization: Bearer` 與 cookie SessionCookie（ADR 同上 D1），兩者都有時
// 以 Bearer 為準。其他位置（query、token header、Bearer 以外的 scheme）一律不讀（#2511）。cookie 重複出現（同名兩個）視為沒有：
// 無法判斷哪一個是我們設的。
func Credential(r *http.Request) (string, CredentialSource) {
	return CredentialFromHeader(r.Header)
}

// CredentialFromHeader 是 Credential 對 header 的版本，給只拿得到 header 的 Connect handler（req.Header()）用，規則完全相同。
func CredentialFromHeader(h http.Header) (string, CredentialSource) {
	if scheme, v, ok := strings.Cut(h.Get("Authorization"), " "); ok && strings.EqualFold(scheme, "Bearer") {
		if token := strings.TrimSpace(v); token != "" {
			return token, CredentialBearer
		}
		return "", CredentialNone
	}
	var token string
	for _, ck := range (&http.Request{Header: h}).Cookies() {
		if ck.Name != SessionCookie {
			continue
		}
		if token != "" {
			return "", CredentialNone
		}
		token = ck.Value
	}
	if token == "" {
		return "", CredentialNone
	}
	return token, CredentialCookie
}

// Session 是請求所用的 session（不是憑證）：ID 與 client 種類，以及憑證從哪裡來（決定登出要不要清 cookie）。
type Session struct {
	ID     uuid.UUID
	Client identity.ClientKind
	Source CredentialSource
}

type sessionKey struct{}

// WithSession 把請求所用的 session 放進 ctx；只有入口的認證中介層（withSession）呼叫，管理員的請求沒有。
func WithSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

// SessionFrom 回傳請求所用的 session；以訪客執行或管理員的請求 ok 是 false。
func SessionFrom(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(Session)
	return s, ok
}

type subjectKey struct{}

// WithSubject 把限流的身分放進 ctx；只有入口的中介層呼叫。
func WithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

// SubjectFrom 回傳入口判定的限流身分：帳號是 a:<ID>、管理員是 m:<ID>、訪客是 v:<IP>，網頁的 SSR 與 CI 各是一個身分（見 httpapi 的 origin-auth）。
func SubjectFrom(ctx context.Context) string {
	s, _ := ctx.Value(subjectKey{}).(string)
	return s
}

type clientIPKey struct{}

// WithClientIP 把入口判定的連線端 IP 放進 ctx；只有入口的中介層呼叫（與限流身分同一個來源，不另外讀 RemoteAddr 或 header）。
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// ClientIPFrom 回傳入口判定的連線端 IP（部署在 CDN 後面時是 CDN 帶的 header，否則是 TCP 連線的位址），已登入的請求也有；
// ctx 沒有時是空字串。內容是入口收到的原樣文字，不保證是合法的 IP。
func ClientIPFrom(ctx context.Context) string {
	s, _ := ctx.Value(clientIPKey{}).(string)
	return s
}
