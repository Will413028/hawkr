package rpc

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// SessionCookie 是 Clerk 在 web 設的 session cookie 的名稱（ADR 2026-09-27-identity-clerk D3）：同源的請求（web 經
// 第一方反代打單體）自動帶上，值是 Clerk 的 session token。cookie 由 Clerk 的 SDK 寫入與更新（只在分頁有焦點時），
// 單體只讀不寫。web 的請求以 Bearer 為主，cookie 給帶不了 header 的瀏覽器導覽下載與子資源（例如發票 PDF 與圖片）。
const SessionCookie = "__session"

// Credential 回傳請求帶的 session token：四端都以 Authorization: Bearer 帶，沒有時僅在 GET REST 瀏覽器導覽或 no-cors 子資源讀 Clerk 的 cookie。兩者都有時
// 以 Bearer 為準。其他位置（query、token header、Bearer 以外的 scheme）一律不讀（#2511）。
func Credential(r *http.Request) string {
	h := r.Header
	if scheme, v, ok := strings.Cut(h.Get("Authorization"), " "); ok && strings.EqualFold(scheme, "Bearer") {
		return strings.TrimSpace(v)
	}
	mode := h.Get("Sec-Fetch-Mode")
	if r.Method != http.MethodGet || (mode != "navigate" && mode != "no-cors") || !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		return ""
	}
	if ck, err := r.Cookie(SessionCookie); err == nil {
		return ck.Value
	}
	return ""
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

// VendorPortalCookiePrefix 是 vendor portal 的 session cookie 名稱的前綴（API 設計規範（Connect）§二）：每張邀請一個 cookie，
// 名稱是前綴加 rfq_recipient_id（十進位）。`__Host-` 要求 Secure、Path=/ 且不帶 Domain，兄弟子網域塞不進同名的 cookie。
// 這個檔案是名稱與屬性的唯一建構與解析處。
const VendorPortalCookiePrefix = "__Host-hk_vp_"

// MaxVendorPortalCookies 是瀏覽器同時可以帶的 vendor portal cookie 上限；ExchangeInvite 的回應清掉多出的。
const MaxVendorPortalCookies = 20

// VendorPortalCookieName 是 rfqRecipientID 那張邀請的 cookie 名稱。
func VendorPortalCookieName(rfqRecipientID int64) string {
	return VendorPortalCookiePrefix + strconv.FormatInt(rfqRecipientID, 10)
}

// vendorPortalCookieID 解析 cookie 名稱裡的收件人 ID：只接受 VendorPortalCookieName 會產生的正整數寫法。
func vendorPortalCookieID(name string) (int64, bool) {
	suffix, ok := strings.CutPrefix(name, VendorPortalCookiePrefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(suffix, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != suffix {
		return 0, false
	}
	return id, true
}

// NewVendorPortalCookie 是一張邀請的 session cookie：Max-Age 是到 session 絕對期限的剩餘秒數（已經被邀請的到期時間截斷），至少 1。
func NewVendorPortalCookie(rfqRecipientID int64, token string, absoluteExpiresAt, now time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     VendorPortalCookieName(rfqRecipientID),
		Value:    token,
		Path:     "/",
		MaxAge:   max(1, int(absoluteExpiresAt.Sub(now)/time.Second)),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// ExpiredVendorPortalCookie 清掉名稱為 name 的 cookie（屬性與設定時相同，Max-Age=0）。
func ExpiredVendorPortalCookie(name string) *http.Cookie {
	return &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}

// VendorPortalCookieToken 讀請求裡 rfqRecipientID 那張邀請的 cookie（所有 Cookie header 行，格式不對的 cookie 略過，不影響其他
// cookie）；沒有時回空字串。同名的 cookie 出現不只一個時視同沒有：`__Host-` 前綴保證瀏覽器只會送出一個，多個代表 cookie 被別處
// 塞入（cookie tossing），取哪一個都是猜，所以拒絕。其他邀請的 cookie 不讀。
func VendorPortalCookieToken(h http.Header, rfqRecipientID int64) string {
	cookies := (&http.Request{Header: h}).CookiesNamed(VendorPortalCookieName(rfqRecipientID))
	if len(cookies) != 1 {
		return ""
	}
	return cookies[0].Value
}

// ExcessVendorPortalCookies 回傳請求帶的 vendor portal cookie 超過 MaxVendorPortalCookies 時要清掉的名稱（不含 keep 那一個，
// 它一定留下）；沒超過回 nil。名稱以前綴判斷（格式不對的也算，它們只佔 header）。留下的是 keep 加上收件人 ID 最大的
// MaxVendorPortalCookies-1 個：看不到 cookie 的年齡，而收件人 ID 由 ai-service 依序發號，大的是較新的邀請，格式不對的最先清。
// 輸出依名稱排序，結果只取決於請求帶的名稱。
func ExcessVendorPortalCookies(h http.Header, keep int64) []string {
	keepName := VendorPortalCookieName(keep)
	seen := map[string]bool{}
	type entry struct {
		name string
		id   int64
	}
	var all []entry
	for _, c := range (&http.Request{Header: h}).Cookies() {
		if !strings.HasPrefix(c.Name, VendorPortalCookiePrefix) || c.Name == keepName || seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		id, _ := vendorPortalCookieID(c.Name)
		all = append(all, entry{c.Name, id})
	}
	room := MaxVendorPortalCookies - 1
	if len(all) <= room {
		return nil
	}
	slices.SortFunc(all, func(a, b entry) int {
		if a.id != b.id {
			return cmp.Compare(b.id, a.id)
		}
		return cmp.Compare(a.name, b.name)
	})
	out := make([]string, 0, len(all)-room)
	for _, e := range all[room:] {
		out = append(out, e.name)
	}
	slices.Sort(out)
	return out
}
