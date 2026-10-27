package gatewayproxy

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/Will413028/hawkr/backend/internal/modules/agent"
)

var vendorSessionID = uuid.MustParse("0198a000-0000-7000-8000-000000000001")

// vendorHeaders 把 gateway 收到的 X-Vendor-Portal-* 列成排序後的 "name=value"。
func vendorHeaders(r *http.Request) []string {
	var out []string
	for k, v := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-vendor-portal-") {
			out = append(out, k+"="+strings.Join(v, ","))
		}
	}
	slices.Sort(out)
	return out
}

type vendorFront struct {
	url          string
	gatewayCalls *int
	seen         *[]string
	sessionCalls *int
}

func startVendor(t *testing.T, of VendorSessionOf) vendorFront {
	t.Helper()
	var seen []string
	gatewayCalls, sessionCalls := 0, 0
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gatewayCalls++
		seen = vendorHeaders(r)
	}))
	t.Cleanup(gw.Close)
	counted := func(r *http.Request, id int64) (agent.SessionInfo, bool, error) {
		sessionCalls++
		return of(r, id)
	}
	p, err := New(gw.URL, callerOf, counted, emptyBusiness{}, emptyBusiness{}, sdktrace.NewTracerProvider(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(p)
	t.Cleanup(front.Close)
	return vendorFront{url: front.URL, gatewayCalls: &gatewayCalls, seen: &seen, sessionCalls: &sessionCalls}
}

// do 送一個帶偽造 header 的請求，回傳狀態碼與回應內容。
func (f vendorFront) do(t *testing.T, path string, forged map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, f.url+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range forged {
		req.Header[k] = []string{v} // 不經 canonical 化，模擬任意大小寫
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

var forgedVendorHeaders = map[string]string{
	"X-Vendor-Portal-Rfq-Recipient-Id":     "999",
	"x-vendor-portal-session-id":           "forged",
	"X-VENDOR-PORTAL-VERIFICATION-METHOD":  "GOOGLE",
	"X-Vendor-Portal-Verified-Email":       "attacker@example.com",
	"X-Vendor-Portal-Verified-At":          "2026-01-01T00:00:00Z",
	"X-Vendor-Portal-Something-Not-Listed": "x",
}

func validSession(verified bool) VendorSessionOf {
	return func(_ *http.Request, id int64) (agent.SessionInfo, bool, error) {
		info := agent.SessionInfo{SessionID: vendorSessionID, RfqRecipientID: id}
		if verified {
			info.Verification = &agent.Verification{
				Method: agent.VerificationGoogle, Email: "vendor@example.com",
				At: time.Date(2026, 10, 4, 12, 30, 0, 123456789, time.FixedZone("UTC+8", 8*3600)),
			}
		}
		return info, true, nil
	}
}

func TestVendorHeadersAreStrippedOnEveryForwardedPath(t *testing.T) {
	for _, path := range []string{"/api/v1/public/products", "/api/v1/app/orders", "/webhook/x", VendorPortalRFQPrefix + "42"} {
		t.Run(path, func(t *testing.T) {
			// 即使有有效的 session，非 RFQ 路徑也不設；RFQ 路徑沒有 session 時什麼都不帶。
			f := startVendor(t, func(*http.Request, int64) (agent.SessionInfo, bool, error) { return agent.SessionInfo{}, false, nil })
			f.do(t, path, forgedVendorHeaders)
			if *f.gatewayCalls != 1 || len(*f.seen) != 0 {
				t.Fatalf("gateway 收到偽造的 header：%v（呼叫 %d 次）", *f.seen, *f.gatewayCalls)
			}
		})
	}
}

func TestVendorSessionIsOnlyReadUnderTheRFQRoutes(t *testing.T) {
	f := startVendor(t, validSession(true))
	f.do(t, "/api/v1/public/vendor_portal/invites/x", forgedVendorHeaders)
	f.do(t, "/api/v1/public/products", forgedVendorHeaders)
	if *f.sessionCalls != 0 || len(*f.seen) != 0 {
		t.Fatalf("RFQ 路由以外不該驗證 session 或帶 header：呼叫 %d 次，%v", *f.sessionCalls, *f.seen)
	}
}

func TestVendorHeadersForAnUnverifiedSession(t *testing.T) {
	f := startVendor(t, validSession(false))
	f.do(t, VendorPortalRFQPrefix+"42/response_draft", forgedVendorHeaders)
	want := []string{
		"X-Vendor-Portal-Rfq-Recipient-Id=42",
		"X-Vendor-Portal-Session-Id=" + vendorSessionID.String(),
	}
	if !slices.Equal(*f.seen, want) {
		t.Fatalf("headers = %v，預期 %v", *f.seen, want)
	}
}

func TestVendorHeadersForAVerifiedSession(t *testing.T) {
	f := startVendor(t, validSession(true))
	f.do(t, VendorPortalRFQPrefix+"42", forgedVendorHeaders)
	want := []string{
		"X-Vendor-Portal-Rfq-Recipient-Id=42",
		"X-Vendor-Portal-Session-Id=" + vendorSessionID.String(),
		"X-Vendor-Portal-Verification-Method=GOOGLE",
		"X-Vendor-Portal-Verified-At=2026-10-04T04:30:00.123456789Z",
		"X-Vendor-Portal-Verified-Email=vendor@example.com",
	}
	if !slices.Equal(*f.seen, want) {
		t.Fatalf("headers = %v，預期 %v", *f.seen, want)
	}
}

func TestInvalidVendorSessionForwardsWithoutVendorHeaders(t *testing.T) {
	f := startVendor(t, func(*http.Request, int64) (agent.SessionInfo, bool, error) { return agent.SessionInfo{}, false, nil })
	status, _ := f.do(t, VendorPortalRFQPrefix+"42", forgedVendorHeaders)
	if status != http.StatusOK || *f.gatewayCalls != 1 || len(*f.seen) != 0 {
		t.Fatalf("status=%d gateway=%d headers=%v", status, *f.gatewayCalls, *f.seen)
	}
}

func TestVendorSessionLookupFailureIsA503AndNotForwarded(t *testing.T) {
	f := startVendor(t, func(*http.Request, int64) (agent.SessionInfo, bool, error) {
		return agent.SessionInfo{}, false, errors.New("db down")
	})
	status, body := f.do(t, VendorPortalRFQPrefix+"42", forgedVendorHeaders)
	if status != http.StatusServiceUnavailable || !strings.Contains(body, `"code":503`) || *f.gatewayCalls != 0 {
		t.Fatalf("status=%d body=%s gateway=%d", status, body, *f.gatewayCalls)
	}
}

func TestNewRequiresVendorSessions(t *testing.T) {
	if _, err := New("http://127.0.0.1:1", callerOf, nil, emptyBusiness{}, emptyBusiness{}, nil, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("沒有 VendorSessionOf 應該失敗")
	}
}

func TestVendorSessionIsSelectedByThePathRecipient(t *testing.T) {
	var asked []int64
	f := startVendor(t, func(_ *http.Request, id int64) (agent.SessionInfo, bool, error) {
		asked = append(asked, id)
		return agent.SessionInfo{SessionID: vendorSessionID, RfqRecipientID: id}, true, nil
	})
	f.do(t, VendorPortalRFQPrefix+"41/response_draft", nil)
	if got := *f.seen; !slices.Contains(got, "X-Vendor-Portal-Rfq-Recipient-Id=41") {
		t.Fatalf("41 的 headers = %v", got)
	}
	f.do(t, VendorPortalRFQPrefix+"42", nil)
	if got := *f.seen; !slices.Contains(got, "X-Vendor-Portal-Rfq-Recipient-Id=42") {
		t.Fatalf("42 的 headers = %v", got)
	}
	if !slices.Equal(asked, []int64{41, 42}) {
		t.Fatalf("以路徑的收件人驗證：%v", asked)
	}
}

func TestVendorHeadersAreOnlySetWhenTheSessionBelongsToThePathRecipient(t *testing.T) {
	f := startVendor(t, func(*http.Request, int64) (agent.SessionInfo, bool, error) {
		return agent.SessionInfo{SessionID: vendorSessionID, RfqRecipientID: 7}, true, nil
	})
	f.do(t, VendorPortalRFQPrefix+"8/response_draft", forgedVendorHeaders)
	if *f.gatewayCalls != 1 || len(*f.seen) != 0 {
		t.Fatalf("別張邀請的 session 不得設 header：%v", *f.seen)
	}
}

func TestMalformedPathRecipientGetsNoVendorHeadersAndNoLookup(t *testing.T) {
	for _, id := range []string{"", "abc", "0", "-1", "007", "+7", "7x", "99999999999999999999"} {
		t.Run(id, func(t *testing.T) {
			f := startVendor(t, validSession(true))
			f.do(t, VendorPortalRFQPrefix+id, forgedVendorHeaders)
			if *f.sessionCalls != 0 || len(*f.seen) != 0 {
				t.Fatalf("id=%q：呼叫 %d 次，headers=%v", id, *f.sessionCalls, *f.seen)
			}
		})
	}
}
