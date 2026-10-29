package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

var suspendedSince = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

// suspendedAuth：suspended 是停權的帳號，good 是沒有停權的帳號。
type suspendedAuth struct{}

func (suspendedAuth) Authenticate(_ context.Context, token string) (identity.Authenticated, error) {
	a := identity.Authenticated{Principal: identity.Principal{Kind: identity.PrincipalAccount, ID: 9}}
	switch token {
	case "suspended":
		a.SuspendedAt = &suspendedSince
	case "good":
	default:
		return identity.Authenticated{}, fmt.Errorf("unexpected token %q", token)
	}
	return a, nil
}

// seen 是 handler 看到的請求。
type seen struct {
	Called    bool      `json:"called"`
	Principal bool      `json:"principal"`
	Suspended bool      `json:"suspended"`
	At        time.Time `json:"at"`
}

// suspendedEntry 在 withSession 之後掛一個回報 ctx 內容的 handler。
func suspendedEntry() http.Handler {
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, principal := rpc.PrincipalFrom(r.Context())
		at, suspended := rpc.SuspensionFrom(r.Context())
		_ = json.NewEncoder(w).Encode(seen{Called: true, Principal: principal, Suspended: suspended, At: at})
	})
	return withSession(suspendedAuth{}, slog.New(slog.DiscardHandler), probe)
}

func callAs(t *testing.T, h http.Handler, token, method, path string) (*httptest.ResponseRecorder, seen) {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	var s seen
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatalf("%s %s：%v %q", method, path, err, rec.Body)
		}
	}
	return rec, s
}

// reasonOf 讀 Connect 錯誤 JSON 的 code 與 ErrorInfo.reason。
func reasonOf(t *testing.T, body []byte) (code, reason string) {
	t.Helper()
	var e struct {
		Code    string `json:"code"`
		Details []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("不是 Connect 錯誤：%v %q", err, body)
	}
	for _, d := range e.Details {
		if d.Type != "google.rpc.ErrorInfo" {
			continue
		}
		raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(d.Value, "="))
		if err != nil {
			t.Fatal(err)
		}
		var info errdetails.ErrorInfo
		if err := proto.Unmarshal(raw, &info); err != nil {
			t.Fatal(err)
		}
		reason = info.GetReason()
	}
	return e.Code, reason
}

const (
	procGetSession      = "/hawkr.identity.v1alpha1.IdentityAppService/GetSession"
	procDeleteAccount   = "/hawkr.identity.v1alpha1.IdentityAppService/DeleteAccount"
	procLinkApple       = "/hawkr.identity.v1alpha1.IdentityAppService/LinkAppleCredential"
	procGetProfile      = "/hawkr.buyer.v1alpha1.BuyerAppService/GetProfile"
	procAddFavorite     = "/hawkr.buyer.v1alpha1.BuyerAppService/AddFavorite"
	procSubmitFoodSafe  = "/hawkr.seller.v1alpha1.SellerReportPublicService/SubmitFoodSafetyReport"
	procInviteeSession  = "/hawkr.agent.v1alpha1.VendorPortalInviteeService/GetSession"
	procListLiveBanners = "/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners"
)

// 停權的帳號：讀取與自助放行（principal 與停權時間都在 ctx）；寫入回 permission_denied 與 ACCOUNT_SUSPENDED，handler 不被呼叫；
// 公開與受邀供應商的 service 以訪客執行（ctx 沒有 principal）。
func TestSuspendedAccountIsAuthorizedByTheNatureOfTheAction(t *testing.T) {
	h := suspendedEntry()
	for _, proc := range []string{procGetSession, procGetProfile, procDeleteAccount} {
		rec, s := callAs(t, h, "suspended", http.MethodPost, proc)
		if rec.Code != http.StatusOK || !s.Principal || !s.Suspended || !s.At.Equal(suspendedSince) {
			t.Fatalf("%s：%d %+v，預期放行、帶 principal 與停權時間", proc, rec.Code, s)
		}
	}
	for _, proc := range []string{procAddFavorite, procLinkApple} {
		rec, s := callAs(t, h, "suspended", http.MethodPost, proc)
		code, reason := reasonOf(t, rec.Body.Bytes())
		if rec.Code != http.StatusForbidden || code != "permission_denied" || reason != "ACCOUNT_SUSPENDED" || s.Called {
			t.Fatalf("%s：%d code=%q reason=%q called=%v，預期 403 permission_denied ACCOUNT_SUSPENDED", proc, rec.Code, code, reason, s.Called)
		}
	}
	for _, proc := range []string{procSubmitFoodSafe, procListLiveBanners, procInviteeSession} {
		rec, s := callAs(t, h, "suspended", http.MethodPost, proc)
		if rec.Code != http.StatusOK || s.Principal || s.Suspended {
			t.Fatalf("%s：%d %+v，預期以訪客執行（ctx 沒有 principal）", proc, rec.Code, s)
		}
	}
	// 沒有宣告的顧客 procedure 當寫入：預設拒絕。
	rec, _ := callAs(t, h, "suspended", http.MethodPost, "/hawkr.buyer.v1alpha1.BuyerAppService/NotDeclared")
	if _, reason := reasonOf(t, rec.Body.Bytes()); rec.Code != http.StatusForbidden || reason != "ACCOUNT_SUSPENDED" {
		t.Fatalf("沒有宣告的 procedure：%d %q", rec.Code, rec.Body)
	}
}

// 沒有停權的帳號不受影響：寫入照樣到 handler，ctx 沒有停權時間。
func TestNonSuspendedAccountIsNotRestricted(t *testing.T) {
	h := suspendedEntry()
	for _, proc := range []string{procGetSession, procAddFavorite, procLinkApple, procDeleteAccount} {
		rec, s := callAs(t, h, "good", http.MethodPost, proc)
		if rec.Code != http.StatusOK || !s.Principal || s.Suspended {
			t.Fatalf("%s：%d %+v", proc, rec.Code, s)
		}
	}
	if rec, s := callAs(t, h, "good", http.MethodPost, "/api/v1/app/orders"); rec.Code != http.StatusOK || !s.Principal || s.Suspended {
		t.Fatalf("未停權的 POST /api/v1：%d %+v", rec.Code, s)
	}
}

// 每個宣告的 procedure 的結果都等於它的宣告：寫入擋下，讀取與自助放行，公開的一律訪客。
func TestEveryDeclaredProcedureBehavesAsDeclaredForASuspendedAccount(t *testing.T) {
	h := suspendedEntry()
	for proc, effect := range customerEffects {
		audience, _ := rpc.AudienceOf(proc)
		rec, s := callAs(t, h, "suspended", http.MethodPost, proc)
		switch {
		case audience == rpc.AudiencePublic || audience == rpc.AudienceInvitee:
			if rec.Code != http.StatusOK || s.Principal {
				t.Errorf("%s：%d %+v，預期訪客", proc, rec.Code, s)
			}
		case effect == EffectWrite:
			if _, reason := reasonOf(t, rec.Body.Bytes()); rec.Code != http.StatusForbidden || reason != "ACCOUNT_SUSPENDED" {
				t.Errorf("%s：%d %q，預期 ACCOUNT_SUSPENDED", proc, rec.Code, rec.Body)
			}
		default:
			if rec.Code != http.StatusOK || !s.Principal || !s.Suspended {
				t.Errorf("%s：%d %+v，預期放行", proc, rec.Code, s)
			}
		}
	}
}

// 舊 gateway 的 REST（不是 Connect）依 HTTP method：GET、HEAD 放行，其餘回 gateway 的 403 envelope。
func TestSuspendedAccountOnLegacyRESTIsDecidedByMethod(t *testing.T) {
	h := suspendedEntry()
	rec, got := callAs(t, h, "suspended", http.MethodGet, "/api/v1/app/orders")
	if rec.Code != http.StatusOK || !got.Principal || !got.Suspended {
		t.Fatalf("GET /api/v1：%d %+v", rec.Code, got)
	}
	if rec, _ := callAs(t, h, "suspended", http.MethodHead, "/api/v1/app/orders"); rec.Code != http.StatusOK {
		t.Fatalf("HEAD /api/v1：%d", rec.Code)
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec, s := callAs(t, h, "suspended", m, "/api/v1/app/orders")
		var body struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusForbidden || body.Code != http.StatusForbidden || body.Message == "" || s.Called {
			t.Fatalf("%s /api/v1：%d %q called=%v，預期 gateway 形狀的 403", m, rec.Code, rec.Body, s.Called)
		}
	}
}

// 宣告表與掛載的顧客 service 一一對應：connectServices 啟動時比對，數量等於掛載的方法數。
func TestMountedCustomerServicesMatchTheEffectDeclarations(t *testing.T) {
	services, err := connectServices(Deps{Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	descriptors, err := customerServiceDescriptors(services)
	if err != nil {
		t.Fatal(err)
	}
	methods := 0
	for _, d := range descriptors {
		methods += d.Methods().Len()
	}
	if methods != len(customerEffects) || len(descriptors) != 10 {
		t.Fatalf("掛載 %d 個顧客 service、%d 支方法；宣告 %d 支", len(descriptors), methods, len(customerEffects))
	}
}

// 少一個、多一個、值不合法的宣告都不能啟動。
func TestCustomerEffectTableMustMatchTheServicesExactly(t *testing.T) {
	services, err := connectServices(Deps{Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	descriptors, err := customerServiceDescriptors(services)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkCustomerEffectTable("t", customerEffects, descriptors); err != nil {
		t.Fatal(err)
	}
	clone := func() map[string]Effect {
		c := map[string]Effect{}
		for k, v := range customerEffects {
			c[k] = v
		}
		return c
	}
	missing := clone()
	delete(missing, procGetProfile)
	if err := checkCustomerEffectTable("t", missing, descriptors); err == nil || !strings.Contains(err.Error(), procGetProfile+" 沒有授權宣告") {
		t.Fatalf("少一個宣告：%v", err)
	}
	extra := clone()
	extra["/hawkr.buyer.v1alpha1.BuyerAppService/NotAMethod"] = EffectRead
	if err := checkCustomerEffectTable("t", extra, descriptors); err == nil || !strings.Contains(err.Error(), "NotAMethod 有宣告，但 service 沒有這個方法") {
		t.Fatalf("多一個宣告：%v", err)
	}
	invalid := clone()
	invalid[procGetProfile] = 0
	if err := checkCustomerEffectTable("t", invalid, descriptors); err == nil || !strings.Contains(err.Error(), procGetProfile+" 的動作性質") {
		t.Fatalf("不合法的值：%v", err)
	}
	if err := checkCustomerEffects(nil); err == nil {
		t.Fatal("沒有掛載任何顧客 service，宣告都指向不存在的方法，應該失敗")
	}
}

// 宣告要誠實：proto 標 NO_SIDE_EFFECTS 的方法一定是讀取，標 IDEMPOTENT（只用於改變狀態的方法）的一定不是讀取。
func TestEffectDeclarationsAgreeWithTheIdempotencyLevels(t *testing.T) {
	for procedure, effect := range customerEffects {
		svc, method, _ := strings.Cut(strings.TrimPrefix(procedure, "/"), "/")
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(svc))
		if err != nil {
			t.Fatal(err)
		}
		m := d.(protoreflect.ServiceDescriptor).Methods().ByName(protoreflect.Name(method))
		level := m.Options().(*descriptorpb.MethodOptions).GetIdempotencyLevel()
		if level == descriptorpb.MethodOptions_NO_SIDE_EFFECTS && effect != EffectRead {
			t.Errorf("%s 是 NO_SIDE_EFFECTS，宣告卻是 %d", procedure, effect)
		}
		if level == descriptorpb.MethodOptions_IDEMPOTENT && effect == EffectRead {
			t.Errorf("%s 是 IDEMPOTENT（會改變狀態），宣告卻是讀取", procedure)
		}
	}
	if !slices.Contains([]Effect{EffectSelfService}, customerEffects[procDeleteAccount]) {
		t.Errorf("DeleteAccount 要宣告為自助")
	}
}
