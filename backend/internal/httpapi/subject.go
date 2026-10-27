package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// 限流的身分（subject）回答「這個請求算在誰頭上」，與額度是兩個問題。網頁的 SSR 與 CI 各以一個位址
// 代表很多人，所以它們以 origin-auth 自己簽出身分（#2264）：
//
//	X-HK-Origin-Auth: base64url(payload) "." base64url(HMAC-SHA256(key, "hkorigin1:" || base64url(payload)))
//	payload = {"v":1,"p":"visitor"|"ssr"|"ci","ip":<訪客 IP，p=visitor 時>,"ts":<unix 秒>,"m":<method>,"u":<path>}
//
// 簽章綁定這個請求的 method 與 path，時間誤差 ±60 秒。驗證不過或沒有簽章時，身分是連線的 IP（v:<IP>）。
// 格式與 website 的簽章端（origin-auth）相同，搬自 gateway 的 ratelimit_subject.go。
const (
	originAuthHeader  = "X-HK-Origin-Auth"
	originAuthDomain  = "hkorigin1:"
	originAuthVersion = 1
	originAuthSkew    = 60 * time.Second
)

// OriginAuth 是驗證 origin-auth 的金鑰。Main（換金鑰時加上 Prev）給網頁的 worker，CI 另一把：持有 CI 金鑰的
// 人不能簽 visitor，否則能以任何人的 IP 把對方擋在登入之外。空的金鑰表示停用那一類的驗證（以空的 key 驗 HMAC
// 等於任何人都能簽）。
type OriginAuth struct {
	Main []byte
	Prev []byte
	CI   []byte
}

// ClientIP 決定連線端的 IP。Header 不是空字串時讀那個 header（部署在 CDN 後面時是 CDN 帶的 CF-Connecting-IP，
// 入口只接受 CDN 的連線才能信任它，屬於後端 CD 的網路設定）；否則是 TCP 連線的位址。
type ClientIP struct {
	Header string
}

func (c ClientIP) of(r *http.Request) string {
	if c.Header != "" {
		if v := strings.TrimSpace(r.Header.Get(c.Header)); v != "" {
			return v
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// withSubject 判定限流的身分，放進 ctx（rpc.WithSubject），並把連線端 IP 放進 ctx（rpc.WithClientIP，已登入的請求也有）。
func withSubject(keys OriginAuth, clientIP ClientIP, now func() time.Time, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP.of(r)
		subject := visitorSubject(ip)
		if raw := r.Header.Get(originAuthHeader); raw != "" {
			if s, ok := verifyOriginAuth(keys, r, raw, now()); ok {
				subject = s
			}
		}
		if p, ok := rpc.PrincipalFrom(r.Context()); ok {
			prefix := "a:"
			if p.Kind == identity.PrincipalAdmin {
				prefix = "m:"
			}
			subject = prefix + strconv.FormatInt(p.ID, 10)
		}
		next.ServeHTTP(w, r.WithContext(rpc.WithClientIP(rpc.WithSubject(r.Context(), subject), ip)))
	})
}

// visitorSubject 是訪客的限流身分：IPv4 是完整位址；IPv6 以 /64 聚合（RFC 4291：一個 /64 是一個子網，通常就是
// 一個使用者或一條線路，逐位址計數等於每人有 2^64 份額度）。
func visitorSubject(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "v:invalid"
	}
	if v4 := parsed.To4(); v4 != nil {
		return "v:" + v4.String()
	}
	return "v:" + (&net.IPNet{IP: parsed.Mask(ipv6Prefix), Mask: ipv6Prefix}).String()
}

var ipv6Prefix = net.CIDRMask(64, 128)

type originAuthPayload struct {
	V  int    `json:"v"`
	P  string `json:"p"`
	IP string `json:"ip,omitempty"`
	TS int64  `json:"ts"`
	M  string `json:"m"`
	U  string `json:"u"`
}

func verifyOriginAuth(keys OriginAuth, r *http.Request, raw string, now time.Time) (string, bool) {
	body, sig, ok := strings.Cut(raw, ".")
	if !ok || body == "" || sig == "" {
		return "", false
	}
	b, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", false
	}
	var p originAuthPayload
	if err := json.Unmarshal(b, &p); err != nil || p.V != originAuthVersion {
		return "", false
	}
	var candidates [][]byte
	switch p.P {
	case "visitor", "ssr":
		candidates = nonEmpty(keys.Main, keys.Prev)
	case "ci":
		candidates = nonEmpty(keys.CI)
	default:
		return "", false
	}
	mac, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !anyMACMatches(candidates, body, mac) {
		return "", false
	}
	if p.M != r.Method || p.U != r.URL.Path {
		return "", false
	}
	if d := now.Sub(time.Unix(p.TS, 0)); d > originAuthSkew || d < -originAuthSkew {
		return "", false
	}
	switch p.P {
	case "ssr":
		return "s:ssr", true
	case "ci":
		return "s:ci", true
	}
	if net.ParseIP(p.IP) == nil {
		return "", false
	}
	return visitorSubject(p.IP), true
}

func nonEmpty(keys ...[]byte) [][]byte {
	var out [][]byte
	for _, k := range keys {
		if len(k) > 0 {
			out = append(out, k)
		}
	}
	return out
}

func anyMACMatches(keys [][]byte, body string, got []byte) bool {
	ok := false
	for _, k := range keys {
		m := hmac.New(sha256.New, k)
		m.Write([]byte(originAuthDomain))
		m.Write([]byte(body))
		if hmac.Equal(got, m.Sum(nil)) {
			ok = true
		}
	}
	return ok
}
