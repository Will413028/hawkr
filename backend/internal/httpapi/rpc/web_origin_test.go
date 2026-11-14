package rpc

import (
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func TestNewWebOriginAcceptsOnlyAnOrigin(t *testing.T) {
	cases := []struct {
		origin string
		ok     bool
	}{
		{"https://www.example", true},
		{"https://www.example/", true},
		{"http://localhost:3000", true},
		{"http://127.0.0.1:3000", true},
		{"http://www.example", false},
		{"https://www.example/app", false},
		{"https://www.example?x=1", false},
		{"https://user@www.example", false},
		{"www.example", false},
		{"", false},
	}
	for _, c := range cases {
		_, err := NewWebOrigin(c.origin)
		if (err == nil) != c.ok {
			t.Errorf("NewWebOrigin(%q) 的錯誤 = %v，預期可用 = %v", c.origin, err, c.ok)
		}
	}
}

func TestWebOriginURLJoinsSitePathsAndPassesAbsoluteURLsThrough(t *testing.T) {
	w, err := NewWebOrigin("https://www.example/")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"/seller-terms":              "https://www.example/seller-terms",
		"/privacy#seller-kyc-notice": "https://www.example/privacy#seller-kyc-notice",
		"https://pay.example.com/t":  "https://pay.example.com/t",
		"http://localhost:3000/x":    "http://localhost:3000/x",
	}
	for in, want := range cases {
		got, err := w.URL(in)
		if err != nil || got != want {
			t.Errorf("URL(%q) = %q, %v，預期 %q", in, got, err, want)
		}
	}
}

func TestWebOriginURLRejectsAnythingNotAbsolute(t *testing.T) {
	w, err := NewWebOrigin("https://www.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{"", "seller-terms", "//evil.example/x", "ftp://x.example/a", "javascript:alert(1)", "https://", "mailto:a@b.c"} {
		if got, err := w.URL(in); err == nil {
			t.Errorf("URL(%q) = %q，預期錯誤", in, got)
		}
	}
}

const websiteRoot = "../../../../website"

// websiteRoutes 掃 website/app/[locale] 下每一支 page.tsx，回傳不含語系與 route group 的路由。
func websiteRoutes(t *testing.T) map[string]bool {
	t.Helper()
	root := filepath.Join(websiteRoot, "app", "[locale]")
	routes := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "page.tsx" {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		var segs []string
		for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
			if seg == "." || (strings.HasPrefix(seg, "(") && strings.HasSuffix(seg, ")")) {
				continue
			}
			segs = append(segs, seg)
		}
		routes["/"+strings.Join(segs, "/")] = true
		return nil
	})
	if err != nil || len(routes) == 0 {
		t.Fatalf("讀不到 website 的路由（%s）：%v", root, err)
	}
	return routes
}

func websiteLocales(t *testing.T) (supported []string, def string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(websiteRoot, "lib", "intl", "locales.ts"))
	if err != nil {
		t.Fatal(err)
	}
	list := regexp.MustCompile(`supportedLocales = \[([^\]]*)\]`).FindStringSubmatch(string(b))
	d := regexp.MustCompile(`defaultLocale = "([^"]+)"`).FindStringSubmatch(string(b))
	if list == nil || d == nil {
		t.Fatal("website/lib/intl/locales.ts 讀不到 supportedLocales 或 defaultLocale")
	}
	for _, m := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(list[1], -1) {
		supported = append(supported, m[1])
	}
	return supported, d[1]
}

func TestOnboardingReturnPagesExistInTheWebsiteRoutes(t *testing.T) {
	routes := websiteRoutes(t)
	w, err := NewWebOrigin("https://www.example")
	if err != nil {
		t.Fatal(err)
	}
	supported, def := websiteLocales(t)
	for _, page := range []ReturnPage{ReturnPageCertification, ReturnPageSellerApp} {
		for _, locale := range supported {
			done, expired, err := w.OnboardingReturnURLs(page, uitext.Locale(locale))
			if err != nil {
				t.Fatalf("page=%d locale=%s：%v", page, locale, err)
			}
			for _, raw := range []string{done, expired} {
				u, err := url.Parse(raw)
				if err != nil || u.Scheme+"://"+u.Host != w.Origin() {
					t.Errorf("回跳網址 %q 不在網站的 origin %q 之下", raw, w.Origin())
					continue
				}
				path := u.Path
				if locale != def && page == ReturnPageCertification {
					if !strings.HasPrefix(path, "/"+locale+"/") {
						t.Errorf("%q 沒有語系前綴 /%s", raw, locale)
					}
					path = strings.TrimPrefix(path, "/"+locale)
				} else if strings.HasPrefix(path, "/"+locale+"/") {
					t.Errorf("%q 不應該有語系前綴", raw)
				}
				if !routes[path] {
					t.Errorf("回跳網址 %q 的路徑 %q 在 website/app/[locale] 沒有對應的 page.tsx", raw, path)
				}
			}
		}
	}
}

func TestWebLocalesMatchTheWebsiteRouting(t *testing.T) {
	supported, def := websiteLocales(t)
	for _, locale := range supported {
		l, ok := ValidWebLocale(locale)
		if !ok {
			t.Errorf("website 路由的語系 %q 伺服器不認得", locale)
			continue
		}
		prefix, err := sitePrefix(l)
		want := "/" + locale
		if locale == def {
			want = ""
		}
		if err != nil || prefix != want {
			t.Errorf("語系 %q 的前綴 = %q（%v），website 的 as-needed 路由是 %q", locale, prefix, err, want)
		}
	}
	// 反向：伺服器接受的語系網站都要有。sitePrefix 是 switch、列不出來，所以逐一探測候選語系。
	site := map[string]bool{}
	for _, locale := range supported {
		site[locale] = true
	}
	for _, candidate := range []string{"zh-TW", "en", "zh-CN", "zh-HK", "ja", "ko", "fr", "de", "es", "th", "vi", "id", "en-US", "en-GB", "ja-JP"} {
		if _, ok := ValidWebLocale(candidate); ok && !site[candidate] {
			t.Errorf("伺服器接受語系 %q，但 website/lib/intl/locales.ts 的 supportedLocales 沒有它", candidate)
		}
	}
	for _, bad := range []string{"", "fr", "EN", "zh", "zh-tw", "/en"} {
		if _, ok := ValidWebLocale(bad); ok {
			t.Errorf("ValidWebLocale(%q) 應該不通過", bad)
		}
	}
}

func TestOnboardingReturnURLsRejectUnknownInputs(t *testing.T) {
	w, _ := NewWebOrigin("https://www.example")
	if _, _, err := w.OnboardingReturnURLs(0, uitext.ZhTW); err == nil {
		t.Error("未指定的回跳頁被接受")
	}
	if _, _, err := w.OnboardingReturnURLs(ReturnPageCertification, "fr"); err == nil {
		t.Error("不支援的語系被接受")
	}
}
