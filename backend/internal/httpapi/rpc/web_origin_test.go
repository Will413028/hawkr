package rpc

import "testing"

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
		"/seller-terms":               "https://www.example/seller-terms",
		"/privacy#seller-kyc-notice":  "https://www.example/privacy#seller-kyc-notice",
		"https://www.payuni.com.tw/t": "https://www.payuni.com.tw/t",
		"http://localhost:3000/x":     "http://localhost:3000/x",
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
