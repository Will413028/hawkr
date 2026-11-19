package httpapi

import (
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// 公開 GET 的回應一定帶 Vary: Accept-Encoding，但 handler（connect-go）已經加過時不重複。
func TestCachePolicyVary(t *testing.T) {
	cases := []struct {
		name string
		set  []string // handler 自己寫的 Vary
		want []string
	}{
		{"handler 沒有寫", nil, []string{"Accept-Encoding", "Authorization", "Cookie"}},
		{"handler 已經寫了（connect-go 的 GET）", []string{"Accept-Encoding"}, []string{"Accept-Encoding", "Authorization", "Cookie"}},
		{"寫在逗號清單裡、大小寫不同", []string{"Origin, accept-encoding"}, []string{"Origin, accept-encoding", "Authorization", "Cookie"}},
		{"只有別的 header", []string{"Origin"}, []string{"Origin", "Accept-Encoding", "Authorization", "Cookie"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for _, v := range c.set {
					w.Header().Add("Vary", v)
				}
				_, _ = w.Write([]byte("{}"))
			})
			rec := httptest.NewRecorder()
			cachePolicy(rpc.AudiencePublic, func(string) bool { return false }, next).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil))
			if got := rec.Header().Values("Vary"); !slices.Equal(got, c.want) {
				t.Errorf("Vary = %q，預期 %q", got, c.want)
			}
		})
	}
}

const (
	procViewer    = "/hawkr.social.v1alpha1.SocialPublicService/GetPost"
	procNonViewer = "/hawkr.social.v1alpha1.SocialPublicService/SuggestTags"
)

// Public 的 GET：依觀看者而定的 procedure，請求帶憑證才是 private, no-store（A7-D23 O1）；判斷依據是請求，不是 ctx 有沒有呼叫者，
// 停權帳號與無效 token 以訪客執行，ctx 沒有呼叫者，回應一樣不得進共享快取。不依觀看者的 procedure 帶 cookie 也照常 public。
func TestCachePolicyCredential(t *testing.T) {
	cases := []struct {
		name         string
		procedure    string
		header       http.Header
		callerInCtx  bool
		wantCache    string
		wantETagSent bool
	}{
		{"不帶憑證", procViewer, nil, false, "public, no-cache", true},
		{"正常登入", procViewer, http.Header{"Authorization": {"Bearer valid"}}, true, "private, no-store", false},
		{"停權帳號（以訪客執行）", procViewer, http.Header{"Authorization": {"Bearer suspended"}}, false, "private, no-store", false},
		{"無效 token（以訪客執行）", procViewer, http.Header{"Authorization": {"Bearer invalid"}}, false, "private, no-store", false},
		{"cookie 的 session", procViewer, http.Header{"Cookie": {rpc.SessionCookie + "=tok"}}, false, "private, no-store", false},
		{"不認得的 cookie 不算憑證", procViewer, http.Header{"Cookie": {"locale=zh-TW"}}, false, "public, no-cache", true},
		{"不依觀看者的 procedure 帶 session cookie 仍是 public", procNonViewer, http.Header{"Cookie": {rpc.SessionCookie + "=tok"}}, false, "public, no-cache", true},
		{"不依觀看者的 procedure 帶 Bearer 仍是 public", procNonViewer, http.Header{"Authorization": {"Bearer valid"}}, true, "public, no-cache", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{}")) })
			ctx := t.Context()
			if c.callerInCtx {
				ctx = rpc.WithPrincipal(ctx, identity.Principal{})
			}
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, c.procedure, nil)
			for k, v := range c.header {
				req.Header[k] = v
			}
			rec := httptest.NewRecorder()
			cachePolicy(rpc.AudiencePublic, isViewerDependent, next).ServeHTTP(rec, req)
			if got := rec.Header().Get("Cache-Control"); got != c.wantCache {
				t.Errorf("Cache-Control = %q，預期 %q", got, c.wantCache)
			}
			if got := rec.Header().Get("ETag") != ""; got != c.wantETagSent {
				t.Errorf("有 ETag = %v，預期 %v", got, c.wantETagSent)
			}
			for _, name := range []string{"Authorization", "Cookie", "Accept-Encoding"} {
				if !varyIncludes(rec.Header(), name) {
					t.Errorf("Vary = %q，缺 %s", rec.Header().Values("Vary"), name)
				}
			}
		})
	}
}

// 掛載的 Public service 每個 procedure 都要宣告是否依觀看者：全部掛載時通過，缺宣告或多宣告都不能啟動。
func TestPublicViewerDependenceIsDeclaredForEveryProcedure(t *testing.T) {
	services, err := connectServices(Deps{Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), Images: characterize.Images(t), Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("connectServices：%v", err)
	}
	descriptors, err := serviceDescriptors(services, "公開", func(a rpc.Audience) bool { return a == rpc.AudiencePublic })
	if err != nil {
		t.Fatal(err)
	}
	if err := checkViewerDependenceTable("t", publicViewerDependent, descriptors); err != nil {
		t.Fatalf("宣告與掛載應該一致：%v", err)
	}
	missing := maps.Clone(publicViewerDependent)
	delete(missing, procViewer)
	if err := checkViewerDependenceTable("t", missing, descriptors); err == nil || !strings.Contains(err.Error(), procViewer+" 沒有授權宣告") {
		t.Fatalf("缺宣告要不能啟動：%v", err)
	}
	extra := maps.Clone(publicViewerDependent)
	extra["/hawkr.social.v1alpha1.SocialPublicService/NotAMethod"] = true
	if err := checkViewerDependenceTable("t", extra, descriptors); err == nil || !strings.Contains(err.Error(), "NotAMethod 有宣告，但 service 沒有這個方法") {
		t.Fatalf("多宣告要不能啟動：%v", err)
	}
}
