package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/payuni"
	shippingpb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// #1394 反射 XSS 修復 — Gateway 端 regression。
//
// 舊實作把 client round-trip 的 base64 `?data=` 以 fmt.Sprintf 拼進 HTML（無 escape、
// 無 host allowlist）→ 反射 XSS + open form-action。新實作改讀 opaque `?id=` → gRPC
// 取回 canonical → html/template render + host allowlist。本檔以攻擊向量表格驅動驗證：
//   1. 各欄位含 " / > / <script> / tag 分隔字元 → 輸出不形成新可執行 DOM node
//   2. action host 非 allowlist（http/https-evil/javascript:）→ 受控 400、不 render 表單
//   3. redirect_id 不存在（gRPC NOT_FOUND）→ 受控 404、不 render 表單

// ---- mock clients（只覆寫本檔需要的 RPC；其餘靠內嵌 interface）----

type mockStoreMapRedirectFormClient struct {
	shippingpb.ShippingServiceClient
	fn func(context.Context, *shippingpb.GetStoreMapRedirectFormRequest, ...grpc.CallOption) (*shippingpb.GetStoreMapRedirectFormResponse, error)
}

func (m *mockStoreMapRedirectFormClient) GetStoreMapRedirectForm(ctx context.Context, in *shippingpb.GetStoreMapRedirectFormRequest, opts ...grpc.CallOption) (*shippingpb.GetStoreMapRedirectFormResponse, error) {
	return m.fn(ctx, in, opts...)
}

func setupPayUniRedirectRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/store_map/redirect", h.StoreMapRedirect)
	return r
}

// xssProbe 是塞進各欄位的攻擊字串，涵蓋 attribute 跳脫 + script 注入 + tag 分隔。
const xssProbe = `"><script>alert(1)</script>`

// assertNoActiveInjection 驗證 render 輸出未原樣帶入攻擊 payload（= 已被 escape）。
func assertNoActiveInjection(t *testing.T, body string) {
	t.Helper()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("output contains un-escaped <script>alert(1)</script>:\n%s", body)
	}
	// 屬性 context 的 " 必被 escape 成 &#34;，否則可跳出 value="..." 破框
	if strings.Contains(body, `value="">`) {
		t.Fatalf("output contains attribute-breaking un-escaped quote:\n%s", body)
	}
}

// ==================== store map /store_map/redirect ====================

func TestStoreMapRedirect_EscapesAllFields(t *testing.T) {
	mock := &mockStoreMapRedirectFormClient{
		fn: func(_ context.Context, in *shippingpb.GetStoreMapRedirectFormRequest, _ ...grpc.CallOption) (*shippingpb.GetStoreMapRedirectFormResponse, error) {
			if in.RedirectId != "sm-1" {
				t.Errorf("RedirectId = %q, want sm-1", in.RedirectId)
			}
			return &shippingpb.GetStoreMapRedirectFormResponse{
				FormAction:  payuni.ProdBaseURL + "/api/storemap",
				MerId:       xssProbe,
				Version:     xssProbe,
				EncryptInfo: xssProbe,
				HashInfo:    xssProbe,
			}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock)}
	r := setupPayUniRedirectRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/store_map/redirect?id=sm-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	assertNoActiveInjection(t, body)
	if !strings.Contains(body, `action="`+payuni.ProdBaseURL+"/api/storemap") {
		t.Errorf("valid PayUni form action missing from output:\n%s", body)
	}
	if !strings.Contains(body, `name="IsPlatForm" value="1"`) {
		t.Errorf("store map must render fixed IsPlatForm=1:\n%s", body)
	}
}

func TestStoreMapRedirect_RejectsDisallowedActionHost(t *testing.T) {
	cases := []struct {
		name   string
		action string
	}{
		{"http scheme", "http://sandbox-api.payuni.com.tw/api/storemap"},
		{"evil https host", "https://evil.com/storemap"},
		{"javascript scheme", "javascript:alert(document.cookie)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockStoreMapRedirectFormClient{
				fn: func(_ context.Context, _ *shippingpb.GetStoreMapRedirectFormRequest, _ ...grpc.CallOption) (*shippingpb.GetStoreMapRedirectFormResponse, error) {
					return &shippingpb.GetStoreMapRedirectFormResponse{FormAction: tc.action, MerId: "HAWK123"}, nil
				},
			}
			h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock)}
			r := setupPayUniRedirectRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/store_map/redirect?id=x", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (rejected action host)", w.Code)
			}
			if strings.Contains(w.Body.String(), "<form") {
				t.Errorf("rejected request must not render form:\n%s", w.Body.String())
			}
		})
	}
}

func TestStoreMapRedirect_NotFound(t *testing.T) {
	mock := &mockStoreMapRedirectFormClient{
		fn: func(_ context.Context, _ *shippingpb.GetStoreMapRedirectFormRequest, _ ...grpc.CallOption) (*shippingpb.GetStoreMapRedirectFormResponse, error) {
			return nil, grpcstatus.Error(codes.NotFound, "redirect record not found")
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock)}
	r := setupPayUniRedirectRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/store_map/redirect?id=gone", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if strings.Contains(w.Body.String(), "<form") {
		t.Errorf("NOT_FOUND must not render form:\n%s", w.Body.String())
	}
}

func TestStoreMapRedirect_MissingID(t *testing.T) {
	h := &Holder{}
	r := setupPayUniRedirectRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/store_map/redirect", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (missing id)", w.Code)
	}
}

// TestIsAllowedPayUniActionURL 直接單測 host allowlist 判定。
func TestIsAllowedPayUniActionURL(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{payuni.SandboxBaseURL + "/api/upp", true},
		{payuni.ProdBaseURL + "/api/storemap", true},
		{"http://api.payuni.com.tw/upp", false},   // 非 https
		{"https://evil.com/upp", false},           // 非白名單 host
		{"https://payuni.com.tw.evil.com", false}, // lookalike
		{"javascript:alert(1)", false},
		{"", false},
		{"://bad", false},
	}
	for _, tc := range cases {
		if got := isAllowedPayUniActionURL(tc.raw); got != tc.want {
			t.Errorf("isAllowedPayUniActionURL(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
