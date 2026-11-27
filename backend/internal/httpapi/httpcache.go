package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

func cachePolicy(a rpc.Audience, viewerDependent func(procedure string) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a != rpc.AudiencePublic || r.Method != http.MethodGet {
			w.Header().Set("Cache-Control", "private, no-store")
			next.ServeHTTP(w, r)
			return
		}
		rec := &bufferedResponse{header: http.Header{}, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		h := w.Header()
		for k, v := range rec.header {
			h[k] = v
		}
		addVary(h, "Accept-Encoding", "Authorization", "Cookie")
		if token, _ := rpc.Credential(r); token != "" && viewerDependent(r.URL.Path) {
			h.Set("Cache-Control", "private, no-store")
			w.WriteHeader(rec.status)
			_, _ = w.Write(rec.body.Bytes())
			return
		}
		h.Set("Cache-Control", "public, no-cache")
		if rec.status == http.StatusOK {
			tag := weakETag(rec.body.Bytes())
			h.Set("ETag", tag)
			if etagMatches(r.Header.Get("If-None-Match"), tag) {
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

func weakETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `W/"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`
}

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

func addVary(h http.Header, names ...string) {
	for _, name := range names {
		if !varyIncludes(h, name) {
			h.Add("Vary", name)
		}
	}
}

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

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header { return b.header }

func (b *bufferedResponse) WriteHeader(status int) { b.status = status }

func (b *bufferedResponse) Write(p []byte) (int, error) { return b.body.Write(p) }
