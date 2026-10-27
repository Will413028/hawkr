package gatewayproxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

type buyerLookupFunc func(context.Context, int64) (int64, error)

func (f buyerLookupFunc) CurrentBuyerID(ctx context.Context, id int64) (int64, error) {
	return f(ctx, id)
}

type sellerLookupFunc func(context.Context, int64) (seller.Store, error)

func (f sellerLookupFunc) CurrentStore(ctx context.Context, id int64) (seller.Store, error) {
	return f(ctx, id)
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func capabilityTokens(h http.Header) []string {
	v := h.Get(HeaderSellerCapabilities)
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}

func tokensOf(caps []seller.Capability) []string {
	out := make([]string, len(caps))
	for i, c := range caps {
		out[i] = string(c)
	}
	return out
}

func TestSetIdentityBusinessHeaders(t *testing.T) {
	caps := []seller.Capability{seller.CapOrderFulfill, seller.CapNotification}
	for _, tc := range []struct {
		name                             string
		kind                             identity.PrincipalKind
		roles                            []identity.Role
		authenticated                    bool
		store                            int64
		wantBuyer, wantStore, wantReason string
		wantCaps                         []string
	}{
		{"訪客", identity.PrincipalAccount, nil, false, 82, "", "", "", nil},
		{"買家", identity.PrincipalAccount, nil, true, 82, "71", "", "", nil},
		{"賣家也是買家", identity.PrincipalAccount, []identity.Role{identity.RoleSeller}, true, 82, "71", "82", "9601", tokensOf(caps)},
		{"管理員", identity.PrincipalAdmin, []identity.Role{identity.RoleSeller}, true, 82, "", "", "", nil},
		{"沒有店家", identity.PrincipalAccount, []identity.Role{identity.RoleSeller}, true, 0, "71", "", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for _, k := range []string{HeaderBuyerID, HeaderStoreID, HeaderSellerSuspended, HeaderSellerCapabilities} {
				h.Set(k, "FORGED")
			}
			c := Caller{Principal: identity.Principal{Kind: tc.kind, ID: 7, Roles: tc.roles}, BuyerID: 71, StoreID: tc.store, SellerSuspended: "9601", SellerCapabilities: caps}
			SetIdentity(h, c, tc.authenticated)
			for k, want := range map[string]string{HeaderBuyerID: tc.wantBuyer, HeaderStoreID: tc.wantStore, HeaderSellerSuspended: tc.wantReason} {
				if got := h.Get(k); got != want {
					t.Errorf("%s = %q, want %q", k, got, want)
				}
			}
			if got := capabilityTokens(h); !slices.Equal(got, tc.wantCaps) {
				t.Errorf("%s = %v, want %v", HeaderSellerCapabilities, got, tc.wantCaps)
			}
		})
	}
}

func riskMeasure(ground seller.RiskGround) seller.EffectiveMeasure {
	return seller.EffectiveMeasure{Kind: seller.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: seller.BasisRisk, RiskGround: &ground}
}

func violationMeasure() seller.EffectiveMeasure {
	category := policy.Fraud
	return seller.EffectiveMeasure{Kind: seller.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: seller.BasisViolation, LeadCategory: &category}
}

func suspendedByRisk(ground seller.RiskGround) seller.Store {
	return storeIn(seller.Active, riskMeasure(ground))
}

// storeIn 是 seller 讀出的店家：條件（Conditions）照 status 帶上，KYC 已核准、店名、logo 與鄉鎮市區齊全；KYC 的影響由
// TestHeaderCarriesOnlyActiveActionCapabilities 驗證。
func storeIn(status seller.Status, effective ...seller.EffectiveMeasure) seller.Store {
	var st seller.Store
	st.StoreID, st.Status, st.Sanctions = 82, status, seller.SanctionedBy(effective...)
	st.Conditions = seller.Conditions{
		Derived: true, Active: status != seller.Closed, NotPendingDeletion: status != seller.PendingDeletion, KYCApproved: true,
		StoreName: true, Logo: true, TownCode: true,
	}
	return st
}

// 每個帳號都是買家（W5）：轉發的帳號請求一律解析 buyer_id 並帶 X-Buyer-Id（gateway 以缺省表示沒有買家、不再查 RPC），
// 管理員與訪客不查、不帶；存活的帳號查不到現任買家、有賣家角色卻沒有現任店家都是不變量被破壞（500，不轉發）。賣家的
// Connect 方法不受 buyer 查詢失敗影響由 internal/httpapi 的測試保證（入口不解析 buyer_id）。
func TestForwardBusinessIdentity(t *testing.T) {
	seller2 := []identity.Role{identity.RoleSeller}
	for _, tc := range []struct {
		name                                        string
		roles                                       []identity.Role
		admin, guest                                bool
		store                                       seller.Store
		sellerErr                                   error
		buyerError, buyerMissing                    bool
		wantStatus, wantSellerCalls, wantBuyerCalls int
		wantReason                                  string
		logged                                      string
	}{
		{name: "買家", wantStatus: 200, wantBuyerCalls: 1},
		{name: "賣家也是買家", roles: seller2, store: storeIn(seller.Active), wantStatus: 200, wantSellerCalls: 1, wantBuyerCalls: 1},
		{name: "管理員", admin: true, roles: seller2, wantStatus: 200},
		{name: "訪客", guest: true, wantStatus: 200},
		{name: "風險停權（舊的管理員停權）", roles: seller2, store: suspendedByRisk(seller.RiskFraud), wantStatus: 200, wantSellerCalls: 1, wantBuyerCalls: 1, wantReason: "9603"},
		{name: "認定停權（舊的治理停權）", roles: seller2, store: storeIn(seller.Active, violationMeasure()), wantStatus: 200, wantSellerCalls: 1, wantBuyerCalls: 1, wantReason: "SUSPENDED_GOVERNANCE"},
		{name: "兩種停權並存：風險優先", roles: seller2, store: storeIn(seller.Active, violationMeasure(), riskMeasure(seller.RiskSecurity)), wantStatus: 200, wantSellerCalls: 1, wantBuyerCalls: 1, wantReason: "9603"},
		{name: "等待刪除", roles: seller2, store: storeIn(seller.PendingDeletion), wantStatus: 200, wantSellerCalls: 1, wantBuyerCalls: 1},
		{name: "沒有現任店家", roles: seller2, sellerErr: apperr.NotFound("沒有現任店家"), wantStatus: 500, wantSellerCalls: 1, wantBuyerCalls: 1, logged: "沒有現任店家"},
		{name: "賣家失敗", roles: seller2, sellerErr: errors.New("seller unavailable"), wantStatus: 503, wantSellerCalls: 1, wantBuyerCalls: 1},
		{name: "沒有推導過停權狀態", roles: seller2, store: seller.Store{StoreID: 82, Status: seller.Active}, wantStatus: 503, wantSellerCalls: 1, wantBuyerCalls: 1},
		{name: "不認得的風險類型", roles: seller2, store: suspendedByRisk("NOT_A_GROUND"), wantStatus: 503, wantSellerCalls: 1, wantBuyerCalls: 1},
		{name: "不認得的措施依據", roles: seller2, store: storeIn(seller.Active, seller.EffectiveMeasure{Kind: seller.MeasureSuspension, Basis: "MYSTERY"}), wantStatus: 503, wantSellerCalls: 1, wantBuyerCalls: 1},
		{name: "現任店家沒有任何能力", roles: seller2, store: storeIn(seller.Closed), wantStatus: 503, wantSellerCalls: 1, wantBuyerCalls: 1},
		{name: "買家查詢失敗", buyerError: true, wantStatus: 503, wantBuyerCalls: 1},
		{name: "買家查詢失敗時不查店家", roles: seller2, buyerError: true, wantStatus: 503, wantBuyerCalls: 1},
		{name: "存活的帳號沒有買家", roles: seller2, buyerMissing: true, wantStatus: 500, wantBuyerCalls: 1, logged: "沒有現任買家"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sellerCalls, buyerCalls, upstreamCalls := 0, 0, 0
			kind := identity.PrincipalAccount
			if tc.admin {
				kind = identity.PrincipalAdmin
			}
			var logs strings.Builder
			h, err := New("http://gateway.invalid", func(context.Context) (Caller, bool) {
				// 入口放進 ctx 的 BuyerID 一律被忽略：forwarder 自己解析。
				return Caller{Principal: identity.Principal{Kind: kind, ID: 7, Roles: tc.roles}, BuyerID: 999}, !tc.guest
			}, noVendorSession, buyerLookupFunc(func(_ context.Context, id int64) (int64, error) {
				buyerCalls++
				if id != 7 {
					t.Fatalf("account = %d", id)
				}
				if tc.buyerError {
					return 0, errors.New("buyer unavailable")
				}
				if tc.buyerMissing {
					return 0, apperr.NotFound("找不到買家資料")
				}
				return 71, nil
			}), sellerLookupFunc(func(_ context.Context, id int64) (seller.Store, error) {
				sellerCalls++
				if id != 7 {
					t.Fatalf("account = %d", id)
				}
				return tc.store, tc.sellerErr
			}), nil, slog.New(slog.NewTextHandler(&logs, nil)))
			if err != nil {
				t.Fatal(err)
			}
			h.(*forwarder).proxy.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
				upstreamCalls++
				wantBuyer, wantStore := "", ""
				var wantCaps []string
				if !tc.admin && !tc.guest {
					wantBuyer = "71"
					if slices.Contains(tc.roles, identity.RoleSeller) {
						wantStore, wantCaps = "82", tokensOf(seller.ActiveCapabilities(seller.CapabilitiesOf(tc.store)))
					}
				}
				for k, want := range map[string]string{HeaderBuyerID: wantBuyer, HeaderStoreID: wantStore, HeaderSellerSuspended: tc.wantReason} {
					if got := req.Header.Get(k); got != want {
						t.Errorf("%s = %q, want %q", k, got, want)
					}
				}
				if got := capabilityTokens(req.Header); !slices.Equal(got, wantCaps) || (wantStore != "" && len(got) == 0) {
					t.Errorf("%s = %v, want %v", HeaderSellerCapabilities, got, wantCaps)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("ok"))}, nil
			})
			req := httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/test", nil)
			for _, k := range []string{HeaderBuyerID, HeaderStoreID, HeaderSellerSuspended, HeaderSellerCapabilities} {
				req.Header.Set(k, "FORGED")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.wantStatus || sellerCalls != tc.wantSellerCalls || buyerCalls != tc.wantBuyerCalls {
				t.Fatalf("status=%d seller=%d buyer=%d", w.Code, sellerCalls, buyerCalls)
			}
			switch tc.wantStatus {
			case 503:
				if upstreamCalls != 0 || w.Body.String() != `{"code":503,"message":"服務暫時無法使用，請稍後再試"}` || logs.Len() == 0 {
					t.Fatalf("upstream=%d body=%s logs=%s", upstreamCalls, w.Body, logs.String())
				}
			case 500:
				if upstreamCalls != 0 || !strings.Contains(logs.String(), tc.logged) {
					t.Fatalf("upstream=%d body=%s logs=%s", upstreamCalls, w.Body, logs.String())
				}
			default:
				if upstreamCalls != 1 {
					t.Fatalf("upstream=%d", upstreamCalls)
				}
			}
		})
	}
}

func TestPendingDeletionKeepsFulfilmentAndDropsBusinessCapabilities(t *testing.T) {
	got := tokensOf(seller.ActiveCapabilities(seller.CapabilitiesOf(storeIn(seller.PendingDeletion))))
	if slices.Contains(got, "CAP_PRODUCT_MANAGE") || slices.Contains(got, "CAP_MARKETING") || slices.Contains(got, "CAP_SHIPPING_SETUP") || !slices.Contains(got, "CAP_ORDER_FULFILL") {
		t.Fatalf("等待刪除的能力 = %v", got)
	}
}

// 經 ServeHTTP：header 的 CAP_SHIPPING_SETUP 跟著 seller 讀出的 KYC 條件（Store.Conditions.KYCApproved）。
func TestForwardedHeaderFollowsTheKYCCondition(t *testing.T) {
	do := func(st seller.Store) (status int, tokens []string, upstream int) {
		l := sellerLookupFunc(func(context.Context, int64) (seller.Store, error) { return st, nil })
		h, err := New("http://gateway.invalid", func(context.Context) (Caller, bool) {
			return Caller{Principal: identity.Principal{Kind: identity.PrincipalAccount, ID: 7, Roles: []identity.Role{identity.RoleSeller}}}, true
		}, noVendorSession, buyerLookupFunc(func(context.Context, int64) (int64, error) { return 71, nil }), l, nil, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		h.(*forwarder).proxy.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
			upstream++
			tokens = capabilityTokens(req.Header)
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("ok"))}, nil
		})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/test", nil))
		return w.Code, tokens, upstream
	}
	for _, kyc := range []bool{false, true} {
		state := storeIn(seller.Active)
		state.Conditions.KYCApproved = kyc
		status, tokens, upstream := do(state)
		if status != 200 || upstream != 1 || !slices.Contains(tokens, "CAP_PRODUCT_MANAGE") {
			t.Fatalf("kyc %v: status=%d upstream=%d tokens=%v", kyc, status, upstream, tokens)
		}
		if got := slices.Contains(tokens, "CAP_SHIPPING_SETUP"); got != kyc {
			t.Errorf("kyc %v: header has CAP_SHIPPING_SETUP=%v (%v)", kyc, got, tokens)
		}
	}
}

// header 只放啟用的動作類能力：物流建店在 KYC 核准之前不放，店家的狀態 PUBLIC_LISTING 無論啟用與否都不放。
func TestHeaderCarriesOnlyActiveActionCapabilities(t *testing.T) {
	for _, kyc := range []bool{false, true} {
		state := storeIn(seller.Active)
		state.Conditions.KYCApproved = kyc
		var c Caller
		statuses := seller.CapabilitiesOf(state)
		if err := currentStoreOf(state, statuses, &c); err != nil {
			t.Fatal(err)
		}
		tokens := strings.Split(capabilitiesHeader(c.SellerCapabilities), ",")
		if got := slices.Contains(tokens, "CAP_SHIPPING_SETUP"); got != kyc {
			t.Errorf("kyc %v: header %v has CAP_SHIPPING_SETUP=%v", kyc, tokens, got)
		}
		if slices.Contains(tokens, string(seller.CapPublicListing)) || !slices.Contains(tokens, "CAP_PRODUCT_MANAGE") {
			t.Errorf("kyc %v: header %v", kyc, tokens)
		}
		for _, token := range tokens {
			if !slices.Contains(tokensOf(seller.Capabilities()), token) {
				t.Errorf("kyc %v: %s 不是可以授權的能力", kyc, token)
			}
		}
	}
}

// seller 模組的能力名稱與 gateway 的 authz 能力 token 必須一對一：任何一邊多或少，單體帶的集合就會被 gateway 以契約錯誤拒絕，
// 或有能力卻永遠不被承認。
func TestCapabilityTokensMatchGateway(t *testing.T) {
	src, err := os.ReadFile("../../../gateway/internal/authz/capability.go")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)var AllCapabilities = \[\]Capability\{(.*?)\}`).FindSubmatch(src)
	if m == nil {
		t.Fatal("gateway 的 AllCapabilities 找不到")
	}
	block := string(m[1])
	var want []string
	for _, name := range regexp.MustCompile(`Cap\w+`).FindAllString(block, -1) {
		m := regexp.MustCompile(name + ` Capability = "([A-Z_]+)"`).FindStringSubmatch(string(src))
		if m == nil {
			t.Fatalf("gateway 的 %s 找不到 token", name)
		}
		want = append(want, m[1])
	}
	got := tokensOf(seller.Capabilities())
	slices.Sort(want)
	slices.Sort(got)
	if len(want) == 0 || !slices.Equal(got, want) {
		t.Fatalf("seller 能力 %v 與 gateway authz 能力 %v 不是一對一", got, want)
	}
}
