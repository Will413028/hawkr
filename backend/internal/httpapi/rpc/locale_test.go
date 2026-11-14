package rpc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1/contentv1alpha1connect"
	contentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1"
	publicconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1/contentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// localeProbe 記下 handler 裡讀到的請求語系；failPublic 時回錯誤，用來看錯誤回應。
type localeProbe struct {
	publicconnect.UnimplementedContentPublicServiceHandler
	contentv1alpha1connect.UnimplementedContentAdminServiceHandler
	seen       uitext.Locale
	failPublic bool
}

func (p *localeProbe) ListLiveBanners(ctx context.Context, _ *connect.Request[contentpb.ListLiveBannersRequest]) (*connect.Response[contentpb.ListLiveBannersResponse], error) {
	p.seen = uitext.LocaleFrom(ctx)
	if p.failPublic {
		return nil, errors.New("boom")
	}
	return connect.NewResponse(&contentpb.ListLiveBannersResponse{}), nil
}

func (p *localeProbe) ListBanners(ctx context.Context, _ *connect.Request[adminpb.ListBannersRequest]) (*connect.Response[adminpb.ListBannersResponse], error) {
	p.seen = uitext.LocaleFrom(ctx)
	return connect.NewResponse(&adminpb.ListBannersResponse{}), nil
}

// localeServer 把 probe 掛在正式的 rpc.NewInterceptor 後面；admin 的呼叫者由外層 middleware 放進 ctx。
func localeServer(t *testing.T, p *localeProbe) *httptest.Server {
	t.Helper()
	opt := connect.WithInterceptors(NewInterceptor(slog.New(slog.DiscardHandler)))
	mux := http.NewServeMux()
	mux.Handle(publicconnect.NewContentPublicServiceHandler(p, opt))
	mux.Handle(contentv1alpha1connect.NewContentAdminServiceHandler(p, opt))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 1})
		mux.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func callPublic(t *testing.T, srv *httptest.Server, acceptLanguage string) (http.Header, error) {
	t.Helper()
	c := publicconnect.NewContentPublicServiceClient(srv.Client(), srv.URL)
	req := connect.NewRequest(&contentpb.ListLiveBannersRequest{})
	if acceptLanguage != "" {
		req.Header().Set("Accept-Language", acceptLanguage)
	}
	resp, err := c.ListLiveBanners(context.Background(), req)
	if err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) {
			return ce.Meta(), err
		}
		return nil, err
	}
	return resp.Header(), nil
}

func TestAcceptLanguageDecidesTheRequestLocale(t *testing.T) {
	cases := []struct {
		name, header string
		want         uitext.Locale
	}{
		{"en", "en", uitext.EN},
		{"帶地區的 en", "en-US,en;q=0.9", uitext.EN},
		{"zh-TW", "zh-TW", uitext.ZhTW},
		{"沒有 header", "", uitext.ZhTW},
		{"不支援的語言", "fr", uitext.ZhTW},
		{"解析失敗", ";;;not a tag", uitext.ZhTW},
		{"q 值較高的英文勝出", "zh-TW;q=0.3,en;q=0.9", uitext.EN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &localeProbe{}
			if _, err := callPublic(t, localeServer(t, p), tc.header); err != nil {
				t.Fatal(err)
			}
			if p.seen != tc.want {
				t.Fatalf("語系 = %q，要 %q", p.seen, tc.want)
			}
		})
	}
}

func TestResponsesVaryOnAcceptLanguage(t *testing.T) {
	t.Run("成功回應", func(t *testing.T) {
		h, err := callPublic(t, localeServer(t, &localeProbe{}), "en")
		if err != nil {
			t.Fatal(err)
		}
		if got := h.Values("Vary"); !containsFold(got, "Accept-Language") {
			t.Fatalf("Vary = %v", got)
		}
	})
	t.Run("錯誤回應", func(t *testing.T) {
		h, err := callPublic(t, localeServer(t, &localeProbe{failPublic: true}), "en")
		if err == nil {
			t.Fatal("要回錯誤")
		}
		if got := h.Values("Vary"); !containsFold(got, "Accept-Language") {
			t.Fatalf("Vary = %v", got)
		}
	})
}

func containsFold(vals []string, want string) bool {
	for _, v := range vals {
		if http.CanonicalHeaderKey(v) == http.CanonicalHeaderKey(want) {
			return true
		}
	}
	return false
}

func TestAdminServiceIgnoresAcceptLanguage(t *testing.T) {
	p := &localeProbe{}
	srv := localeServer(t, p)
	c := contentv1alpha1connect.NewContentAdminServiceClient(srv.Client(), srv.URL)
	req := connect.NewRequest(&adminpb.ListBannersRequest{})
	req.Header().Set("Accept-Language", "en")
	resp, err := c.ListBanners(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if p.seen != uitext.ZhTW {
		t.Fatalf("admin 的語系 = %q，要固定 zh-TW", p.seen)
	}
	if got := resp.Header().Values("Vary"); len(got) != 0 {
		t.Fatalf("admin 的回應不依 header 變化，不該有 Vary：%v", got)
	}
}

func TestNegotiateRequestFollowsTheInterceptorRules(t *testing.T) {
	adminPath := contentv1alpha1connect.ContentAdminServiceListBannersProcedure
	publicPath := publicconnect.ContentPublicServiceListLiveBannersProcedure
	for name, tc := range map[string]struct {
		path, header string
		want         uitext.Locale
		vary         bool
	}{
		"Connect 公開 en":     {publicPath, "en", uitext.EN, true},
		"Connect 沒有 header": {publicPath, "", uitext.ZhTW, true},
		"REST en":           {"/api/v1/public/products", "en", uitext.EN, true},
		"admin 固定 zh-TW":    {adminPath, "en", uitext.ZhTW, false},
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, tc.path, nil)
			if tc.header != "" {
				r.Header.Set("Accept-Language", tc.header)
			}
			w := httptest.NewRecorder()
			ctx := NegotiateRequest(w, r)
			if got := uitext.LocaleFrom(ctx); got != tc.want {
				t.Fatalf("語系 = %q，要 %q", got, tc.want)
			}
			if got := containsFold(w.Header().Values("Vary"), "Accept-Language"); got != tc.vary {
				t.Fatalf("Vary = %v，要 %v", w.Header().Values("Vary"), tc.vary)
			}
		})
	}
}
