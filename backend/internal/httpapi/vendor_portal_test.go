package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	httpagent "github.com/Will413028/hawkr/backend/internal/httpapi/agent"
	agentconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1/agentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

var vendorProcedures = []string{
	agentconnect.VendorPortalPublicServicePreviewInviteProcedure,
	agentconnect.VendorPortalPublicServiceExchangeInviteProcedure,
	agentconnect.VendorPortalInviteeServiceVerifySessionProcedure,
	agentconnect.VendorPortalInviteeServiceGetSessionProcedure,
}

func vendorEntry(t *testing.T, now *time.Time, services ...service) http.Handler {
	t.Helper()
	h, err := assemble(Deps{
		Now: func() time.Time { return *now }, Origins: []string{testOrigin},
		OriginAuth: OriginAuth{Main: []byte("main-secret"), CI: []byte("ci-secret")}, Logger: slog.New(slog.DiscardHandler),
	}, services)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func vendorPost(t *testing.T, h http.Handler, procedure, ip string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, procedure, strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = ip + ":4000"
	return do(h, r)
}

func ok200(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// 預覽與兌換共用同一個計數器：每個來源 5 分鐘 20 次是兩個方法加起來，不是各 20 次。
func TestVendorPortalInviteProceduresShareOneQuota(t *testing.T) {
	now := entryNow
	h := vendorEntry(t, &now, service{"/hawkr.agent.v1alpha1.VendorPortalPublicService/", http.HandlerFunc(ok200)}, service{"/hawkr.agent.v1alpha1.VendorPortalInviteeService/", http.HandlerFunc(ok200)})
	preview, exchange := vendorProcedures[0], vendorProcedures[1]
	if tierVendorPortalInvite.visitor != 20 || tierVendorPortalInvite.window != 5*time.Minute {
		t.Fatalf("額度 = %+v", tierVendorPortalInvite)
	}
	for i := range 20 {
		p := preview
		if i%2 == 1 {
			p = exchange
		}
		if rec := vendorPost(t, h, p, "192.0.2.1"); rec.Code != http.StatusOK {
			t.Fatalf("第 %d 次 = %d", i+1, rec.Code)
		}
	}
	for _, p := range []string{preview, exchange} {
		rec := vendorPost(t, h, p, "192.0.2.1")
		if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"code":"resource_exhausted"`) || !strings.Contains(rec.Body.String(), `"retryDelay":"300s"`) {
			t.Fatalf("%s 第 21 次 = %d %q", p, rec.Code, rec.Body)
		}
	}
	if rec := vendorPost(t, h, preview, "192.0.2.2"); rec.Code != http.StatusOK {
		t.Fatalf("另一個來源不該被擋：%d", rec.Code)
	}
	if rec := vendorPost(t, h, vendorProcedures[2], "192.0.2.1"); rec.Code != http.StatusOK {
		t.Fatalf("邀請的額度用完不該擋住 VerifySession：%d", rec.Code)
	}
	now = now.Add(5 * time.Minute)
	if rec := vendorPost(t, h, exchange, "192.0.2.1"); rec.Code != http.StatusOK {
		t.Fatalf("5 分鐘後應該重新計數：%d", rec.Code)
	}
}

func TestVendorPortalVerifyIsLimitedToTenPerTenMinutes(t *testing.T) {
	now := entryNow
	h := vendorEntry(t, &now, service{"/hawkr.agent.v1alpha1.VendorPortalPublicService/", http.HandlerFunc(ok200)}, service{"/hawkr.agent.v1alpha1.VendorPortalInviteeService/", http.HandlerFunc(ok200)})
	verify := vendorProcedures[2]
	if tierVendorPortalVerify.visitor != 10 || tierVendorPortalVerify.window != 10*time.Minute {
		t.Fatalf("額度 = %+v", tierVendorPortalVerify)
	}
	for i := range 10 {
		if rec := vendorPost(t, h, verify, "192.0.2.1"); rec.Code != http.StatusOK {
			t.Fatalf("第 %d 次 = %d", i+1, rec.Code)
		}
	}
	rec := vendorPost(t, h, verify, "192.0.2.1")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"retryDelay":"600s"`) {
		t.Fatalf("第 11 次 = %d %q", rec.Code, rec.Body)
	}
	if rec := vendorPost(t, h, vendorProcedures[0], "192.0.2.1"); rec.Code != http.StatusOK {
		t.Fatalf("VerifySession 的額度不該擋住預覽：%d", rec.Code)
	}
	now = now.Add(10 * time.Minute)
	if rec := vendorPost(t, h, verify, "192.0.2.1"); rec.Code != http.StatusOK {
		t.Fatalf("10 分鐘後應該重新計數：%d", rec.Code)
	}
}

func TestVendorPortalTiersAreDeclaredForEveryProcedure(t *testing.T) {
	for _, p := range vendorProcedures {
		if p == agentconnect.VendorPortalInviteeServiceGetSessionProcedure {
			continue
		}
		if _, ok := procedureTiers[p]; !ok {
			t.Errorf("%s 沒有自己的級別，會落到 service 的使用者類別的額度", p)
		}
	}
}

// GetSession 沒有自己的級別：落到受邀供應商類別的一般額度（每個來源每分鐘 300 次），不是 Public 的 600。
func TestInviteeGetSessionUsesTheInviteeAudienceTier(t *testing.T) {
	tiers := tiersOf(agentconnect.VendorPortalInviteeServiceGetSessionProcedure)
	if len(tiers) != 1 || tiers[0].name != "invitee" || tiers[0].visitor != 300 || tiers[0].span() != time.Minute {
		t.Fatalf("tiers = %+v", tiers)
	}
	now := entryNow
	h := vendorEntry(t, &now, service{"/hawkr.agent.v1alpha1.VendorPortalInviteeService/", http.HandlerFunc(ok200)})
	for i := range 300 {
		if rec := vendorPost(t, h, vendorProcedures[3], "192.0.2.1"); rec.Code != http.StatusOK {
			t.Fatalf("第 %d 次 = %d", i+1, rec.Code)
		}
	}
	if rec := vendorPost(t, h, vendorProcedures[3], "192.0.2.1"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("第 301 次 = %d", rec.Code)
	}
}

type cacheVendors struct{}

func (cacheVendors) PreviewInvite(context.Context, string) (agent.InvitePreview, error) {
	return agent.InvitePreview{RfqRecipientID: 1, MaskedEmail: "v***@example.com", ExpiresAt: entryNow.Add(time.Hour)}, nil
}

func (cacheVendors) ExchangeInvite(context.Context, string) (agent.IssuedSession, error) {
	return agent.IssuedSession{Token: "tok", RfqRecipientID: 1, AbsoluteExpiresAt: entryNow.Add(time.Hour)}, nil
}

func (cacheVendors) ValidateSession(context.Context, string) (agent.SessionInfo, error) {
	return agent.SessionInfo{RfqRecipientID: 1}, nil
}

func (cacheVendors) VerifySession(_ context.Context, _ string, m agent.VerificationMethod, e string) (agent.SessionInfo, error) {
	return agent.SessionInfo{RfqRecipientID: 1, Verification: &agent.Verification{Method: m, Email: e, At: entryNow}}, nil
}

type cacheGoogle struct{}

func (cacheGoogle) VerifyGoogle(context.Context, string, string) (identity.Identity, error) {
	return identity.Identity{Email: "vendor@example.com", EmailVerified: true}, nil
}

// Public 的兩個方法是 POST，不標 NO_SIDE_EFFECTS：公開 service 的規則只讓 GET 成為共享快取的對象（public, no-cache），其餘一律
// private, no-store，GET 也不被服務（connect-go 對沒有標 NO_SIDE_EFFECTS 的方法回 405）。Invitee 的回應帶驗證結果與 nonce，一律
// private, no-store，連可以用 GET 的 GetSession 也是（不是 Public 的 public, no-cache）。
func TestVendorPortalResponsesAreNeverCacheable(t *testing.T) {
	now := entryNow
	vendor, err := httpagent.NewInterceptor(cacheVendors{})
	if err != nil {
		t.Fatal(err)
	}
	opts := []connect.HandlerOption{connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler)))}
	publicPath, publicHandler := agentconnect.NewVendorPortalPublicServiceHandler(httpagent.NewPublic(cacheVendors{}, func() time.Time { return now }), opts...)
	inviteePath, inviteeHandler := agentconnect.NewVendorPortalInviteeServiceHandler(httpagent.NewInvitee(cacheVendors{}, cacheGoogle{}, nil), append(opts, connect.WithInterceptors(vendor))...)
	h := vendorEntry(t, &now, service{publicPath, publicHandler}, service{inviteePath, inviteeHandler})
	bodies := map[string]string{
		vendorProcedures[0]: `{"secret":"s"}`,
		vendorProcedures[1]: `{"secret":"s"}`,
		vendorProcedures[2]: `{"rfqRecipientId":"1","googleIdToken":"t"}`,
		vendorProcedures[3]: `{"rfqRecipientId":"1"}`,
	}
	for _, p := range vendorProcedures {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, p, strings.NewReader(bodies[p]))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Cookie", rpc.VendorPortalCookieName(1)+"=tok")
		r.RemoteAddr = "192.0.2.1:4000"
		rec := do(h, r)
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Errorf("%s = %d，Cache-Control %q，body %q", p, rec.Code, rec.Header().Get("Cache-Control"), rec.Body)
		}
		if rec.Header().Get("ETag") != "" {
			t.Errorf("%s 不該有 ETag", p)
		}
		get := connectGET(t, p, bodies[p])
		get.RemoteAddr = "192.0.2.1:4000"
		get.Header.Set("Cookie", rpc.VendorPortalCookieName(1)+"=tok")
		rec = do(h, get)
		if p == vendorProcedures[3] {
			if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "private, no-store" || rec.Header().Get("ETag") != "" {
				t.Errorf("GET %s = %d，Cache-Control %q，ETag %q，body %q", p, rec.Code, rec.Header().Get("Cache-Control"), rec.Header().Get("ETag"), rec.Body)
			}
			continue
		}
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d，預期 405", p, rec.Code)
		}
	}
}
