package handler

import (
	"net/http"
	"testing"
)

func TestWSCheckOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://web-dev.example.com")

	mk := func(origin string) *http.Request {
		r, _ := http.NewRequest("GET", "/api/v1/app/chat/ws", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}

	if !wsCheckOrigin(mk("")) {
		t.Error("no Origin header (native client) must be allowed")
	}
	if !wsCheckOrigin(mk("https://web-dev.example.com")) {
		t.Error("allowlisted browser Origin must be allowed")
	}
	if wsCheckOrigin(mk("https://evil.example.com")) {
		t.Error("non-allowlisted browser Origin must be rejected")
	}
}

func TestWSCheckOrigin_EmptyAllowlist_RejectsBrowserOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	r, _ := http.NewRequest("GET", "/api/v1/app/chat/ws", nil)
	r.Header.Set("Origin", "https://evil.example.com")
	if wsCheckOrigin(r) {
		t.Error("empty allowlist must reject any browser Origin")
	}
}
