package httpapi

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

var csrfBypass = []string{
	"POST /payment/return",
	"POST /store_map/return",
	"POST /atm-refund/return",
}

func newCSRF(trusted []string) (*http.CrossOriginProtection, error) {
	p := http.NewCrossOriginProtection()
	for _, o := range trusted {
		if err := p.AddTrustedOrigin(o); err != nil {
			return nil, fmt.Errorf("httpapi：CORS／CSRF 的 origin %q：%w", o, err)
		}
	}
	for _, pattern := range csrfBypass {
		p.AddInsecureBypassPattern(pattern)
	}
	return p, nil
}

func withCookieCSRF(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		if _, src := rpc.Credential(r); src != rpc.CredentialCookie {
			next.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
			if site != "same-origin" {
				http.Error(w, "cross-origin request detected from Sec-Fetch-Site header", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			http.Error(w, "cross-origin request detected, and/or browser is out of date: Sec-Fetch-Site is missing, and Origin is missing", http.StatusForbidden)
			return
		}
		if !slices.Contains(allowed, origin) {
			http.Error(w, "cross-origin request detected, and/or browser is out of date: Sec-Fetch-Site is missing, and Origin does not match", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func cookieCSRFOrigins(web rpc.WebOrigin, trusted []string) []string {
	out := slices.Clone(trusted)
	if o := web.Origin(); o != "" && !slices.Contains(out, o) {
		out = append(out, o)
	}
	return out
}
