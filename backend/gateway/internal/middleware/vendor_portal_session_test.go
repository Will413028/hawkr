package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	testVendorSessionID = "6f1c3a52-0d7e-4c55-9d52-3a2f4f0f9a10"
	testVendorVerified  = "2026-10-04T08:30:00Z"
)

type vendorPortalRun struct {
	code         int
	reached      bool
	recipientID  string
	verification VendorPortalVerification
	verified     bool
}

func runVendorPortalChain(t *testing.T, headers map[string]string, handlers ...gin.HandlerFunc) vendorPortalRun {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var out vendorPortalRun
	r := gin.New()
	chain := append(handlers, func(c *gin.Context) {
		out.reached = true
		raw, _ := c.Get(vendorPortalRecipientIDKey)
		out.recipientID, _ = raw.(string)
		out.verification, out.verified = VendorPortalVerificationFrom(c)
		c.Status(http.StatusOK)
	})
	r.GET("/x", chain...)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out.code = w.Code
	return out
}

func sessionHeaders() map[string]string {
	return map[string]string{
		VendorPortalRecipientIDHeader: "8899",
		VendorPortalSessionIDHeader:   testVendorSessionID,
	}
}

func verifiedHeaders() map[string]string {
	h := sessionHeaders()
	h[VendorPortalVerificationMethodHeader] = "GOOGLE"
	h[VendorPortalVerifiedEmailHeader] = "vendor@example.com"
	h[VendorPortalVerifiedAtHeader] = testVendorVerified
	return h
}

func TestVendorPortalSession_NoRecipientHeaderIs401(t *testing.T) {
	for name, headers := range map[string]map[string]string{
		"no headers":                 nil,
		"session id only":            {VendorPortalSessionIDHeader: testVendorSessionID},
		"verification without ids":   {VendorPortalVerificationMethodHeader: "GOOGLE", VendorPortalVerifiedEmailHeader: "a@b.c", VendorPortalVerifiedAtHeader: testVendorVerified},
		"empty recipient":            {VendorPortalRecipientIDHeader: "", VendorPortalSessionIDHeader: testVendorSessionID},
		"missing session id":         {VendorPortalRecipientIDHeader: "8899"},
		"malformed session id":       {VendorPortalRecipientIDHeader: "8899", VendorPortalSessionIDHeader: "not-a-uuid"},
		"nil uuid session id":        {VendorPortalRecipientIDHeader: "8899", VendorPortalSessionIDHeader: "00000000-0000-0000-0000-000000000000"},
		"non numeric recipient":      {VendorPortalRecipientIDHeader: "abc", VendorPortalSessionIDHeader: testVendorSessionID},
		"zero recipient":             {VendorPortalRecipientIDHeader: "0", VendorPortalSessionIDHeader: testVendorSessionID},
		"negative recipient":         {VendorPortalRecipientIDHeader: "-5", VendorPortalSessionIDHeader: testVendorSessionID},
		"non canonical recipient":    {VendorPortalRecipientIDHeader: "08899", VendorPortalSessionIDHeader: testVendorSessionID},
		"recipient overflows int64":  {VendorPortalRecipientIDHeader: "9223372036854775808", VendorPortalSessionIDHeader: testVendorSessionID},
		"recipient with whitespace":  {VendorPortalRecipientIDHeader: "8899 ", VendorPortalSessionIDHeader: testVendorSessionID},
		"recipient is a hex literal": {VendorPortalRecipientIDHeader: "0x22b3", VendorPortalSessionIDHeader: testVendorSessionID},
	} {
		t.Run(name, func(t *testing.T) {
			got := runVendorPortalChain(t, headers, VendorPortalSession())
			if got.code != http.StatusUnauthorized || got.reached {
				t.Fatalf("code = %d reached = %v, want 401/false", got.code, got.reached)
			}
		})
	}
}

func TestVendorPortalSession_MalformedVerificationHeadersAre401(t *testing.T) {
	for name, mutate := range map[string]func(h map[string]string){
		"method only": func(h map[string]string) {
			delete(h, VendorPortalVerifiedEmailHeader)
			delete(h, VendorPortalVerifiedAtHeader)
		},
		"email only": func(h map[string]string) {
			delete(h, VendorPortalVerificationMethodHeader)
			delete(h, VendorPortalVerifiedAtHeader)
		},
		"time only": func(h map[string]string) {
			delete(h, VendorPortalVerificationMethodHeader)
			delete(h, VendorPortalVerifiedEmailHeader)
		},
		"unknown method":  func(h map[string]string) { h[VendorPortalVerificationMethodHeader] = "EMAIL" },
		"lowercase":       func(h map[string]string) { h[VendorPortalVerificationMethodHeader] = "google" },
		"empty email":     func(h map[string]string) { delete(h, VendorPortalVerifiedEmailHeader) },
		"bad time":        func(h map[string]string) { h[VendorPortalVerifiedAtHeader] = "yesterday" },
		"unix time":       func(h map[string]string) { h[VendorPortalVerifiedAtHeader] = "1790000000" },
		"time no zone":    func(h map[string]string) { h[VendorPortalVerifiedAtHeader] = "2026-10-04T08:30:00" },
		"empty method":    func(h map[string]string) { h[VendorPortalVerificationMethodHeader] = "" },
		"empty time":      func(h map[string]string) { h[VendorPortalVerifiedAtHeader] = "" },
		"empty email set": func(h map[string]string) { h[VendorPortalVerifiedEmailHeader] = "" },
	} {
		t.Run(name, func(t *testing.T) {
			h := verifiedHeaders()
			mutate(h)
			got := runVendorPortalChain(t, h, VendorPortalSession())
			if got.code != http.StatusUnauthorized || got.reached {
				t.Fatalf("code = %d reached = %v, want 401/false", got.code, got.reached)
			}
		})
	}
}

func TestVendorPortalSession_UnverifiedSessionCarriesRecipientOnly(t *testing.T) {
	got := runVendorPortalChain(t, sessionHeaders(), VendorPortalSession())
	if got.code != http.StatusOK || !got.reached {
		t.Fatalf("code = %d reached = %v, want 200/true", got.code, got.reached)
	}
	if got.recipientID != "8899" || got.verified {
		t.Fatalf("recipient = %q verified = %v", got.recipientID, got.verified)
	}
}

func TestVendorPortalSession_VerifiedSessionCarriesEvidence(t *testing.T) {
	h := verifiedHeaders()
	h[VendorPortalVerifiedAtHeader] = "2026-10-04T16:30:00+08:00"
	got := runVendorPortalChain(t, h, VendorPortalSession())
	if got.code != http.StatusOK || !got.verified {
		t.Fatalf("code = %d verified = %v", got.code, got.verified)
	}
	want := VendorPortalVerification{
		Method: "GOOGLE", Email: "vendor@example.com", SessionID: testVendorSessionID,
		VerifiedAt: time.Date(2026, 10, 4, 8, 30, 0, 0, time.UTC),
	}
	if got.verification != want {
		t.Fatalf("verification = %+v, want %+v", got.verification, want)
	}
}

func TestVendorPortalSession_VerifiedAtKeepsNanosecondPrecision(t *testing.T) {
	h := verifiedHeaders()
	h[VendorPortalVerifiedAtHeader] = "2026-10-04T08:30:00.123456789Z"
	got := runVendorPortalChain(t, h, VendorPortalSession())
	if got.code != http.StatusOK || !got.verified {
		t.Fatalf("code = %d verified = %v", got.code, got.verified)
	}
	if want := time.Date(2026, 10, 4, 8, 30, 0, 123456789, time.UTC); !got.verification.VerifiedAt.Equal(want) {
		t.Fatalf("VerifiedAt = %v, want %v", got.verification.VerifiedAt, want)
	}
}

// 🔴 守「fail-closed 是結構性的，不靠順序寫對」：guard 單獨掛（或排在 session
// middleware 前面）時 context 沒有驗證紀錄，必須擋下。
func TestRequireVendorPortalGoogleVerified_DeniesWithoutSessionMiddleware(t *testing.T) {
	got := runVendorPortalChain(t, verifiedHeaders(), RequireVendorPortalGoogleVerified())
	if got.code != http.StatusForbidden || got.reached {
		t.Fatalf("code = %d reached = %v, want 403/false", got.code, got.reached)
	}
}

func TestRequireVendorPortalGoogleVerified_DeniesUnverifiedSession(t *testing.T) {
	got := runVendorPortalChain(t, sessionHeaders(), VendorPortalSession(), RequireVendorPortalGoogleVerified())
	if got.code != http.StatusForbidden || got.reached {
		t.Fatalf("code = %d reached = %v, want 403/false（不是 401：session 有效，只是沒有驗證紀錄）", got.code, got.reached)
	}
}

func TestRequireVendorPortalGoogleVerified_AllowsGoogleVerified(t *testing.T) {
	got := runVendorPortalChain(t, verifiedHeaders(), VendorPortalSession(), RequireVendorPortalGoogleVerified())
	if got.code != http.StatusOK || !got.reached {
		t.Fatalf("code = %d reached = %v, want 200/true", got.code, got.reached)
	}
}

func TestRequireVendorPortalGoogleVerified_DeniesNonVerificationContextValue(t *testing.T) {
	got := runVendorPortalChain(t, nil, func(c *gin.Context) {
		c.Set(vendorPortalSessionKey, 42)
	}, RequireVendorPortalGoogleVerified())
	if got.code != http.StatusForbidden || got.reached {
		t.Fatalf("code = %d reached = %v, want 403/false", got.code, got.reached)
	}
}
