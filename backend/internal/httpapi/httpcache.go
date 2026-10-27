package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

// cachePolicy 設定回應的 Cache-Control（API 設計規範（Connect）§九）：
//   - Public 的 GET：public, no-cache，加上回應內容的弱 ETag。client 與 CDN 每次都向來源確認，
//     沒變時回 304、不送內容。不設 max-age：website 在寫入後以 tag 立即失效，CDN 若在 max-age
//     內回舊資料，website 會把舊資料再快取一輪。
//   - 其他（Public 的 POST、App、Admin）：private, no-store。
//
// 這裡的 ETag 是回應內容的雜湊，與資源的 etag（版本）無關。
func cachePolicy(a rpc.Audience, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a != rpc.AudiencePublic || r.Method != http.MethodGet {
			w.Header().Set("Cache-Control", "private, no-store")
			next.ServeHTTP(w, r)
			return
		}
		// Connect 的 GET 只用在 unary，回應不大，整個緩衝起來才能算雜湊。
		rec := &bufferedResponse{header: http.Header{}, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		h := w.Header()
		for k, v := range rec.header {
			h[k] = v
		}
		h.Set("Cache-Control", "public, no-cache")
		// 回應依 Accept-Encoding 壓縮與否，共享快取必須以它區分。connect-go 對 GET 已經加上，這裡只確保存在。
		if !varyIncludes(h, "Accept-Encoding") {
			h.Add("Vary", "Accept-Encoding")
		}
		if rec.status == http.StatusOK {
			tag := weakETag(rec.body.Bytes())
			h.Set("ETag", tag)
			if etagMatches(r.Header.Get("If-None-Match"), tag) {
				// 304 只帶快取需要的 header，不帶內容的描述（RFC 9110 §15.4.5）。
				for _, k := range []string{"Content-Length", "Content-Type", "Content-Encoding"} {
					h.Del(k)
				}
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		w.WriteHeader(rec.status)
		_, _ = w.Write(rec.body.Bytes())
	})
}

// weakETag 是回應內容的雜湊。弱比較就夠：同一份內容換個壓縮方式也算相同。
func weakETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `W/"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`
}

// etagMatches 以弱比較判斷 If-None-Match（可能是逗號分隔的多個 tag，或 *）是否含 tag。
func etagMatches(header, tag string) bool {
	if header == "" {
		return false
	}
	want := strings.TrimPrefix(tag, "W/")
	for _, t := range strings.Split(header, ",") {
		t = strings.TrimSpace(t)
		if t == "*" || strings.TrimPrefix(t, "W/") == want {
			return true
		}
	}
	return false
}

// varyIncludes 判斷 Vary（可能有多行，每行是逗號分隔的 header 名稱）是否已含 name；header 名稱不分大小寫。
func varyIncludes(h http.Header, name string) bool {
	for _, line := range h.Values("Vary") {
		for _, v := range strings.Split(line, ",") {
			if strings.EqualFold(strings.TrimSpace(v), name) {
				return true
			}
		}
	}
	return false
}

// bufferedResponse 先把回應收下來，等算完 ETag 再決定要送 200 還是 304。
type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header { return b.header }

func (b *bufferedResponse) WriteHeader(status int) { b.status = status }

func (b *bufferedResponse) Write(p []byte) (int, error) { return b.body.Write(p) }
