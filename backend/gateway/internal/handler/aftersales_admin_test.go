package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aftersalespb "github.com/Will413028/hawkr/go-pkg/proto/aftersales"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Mock：只 stub admin 申訴列表 ====================
// 介面 embed：未 stub 的方法呼叫即 nil-panic（沿用 aftersales_test.go 慣例）。

type mockAftersalesAppealsAdminClient struct {
	aftersalespb.AfterSalesServiceClient
	ListAppealsForAdminFn func(ctx context.Context, in *aftersalespb.ListAppealsForAdminRequest, opts ...grpc.CallOption) (*aftersalespb.ListAppealsForAdminResponse, error)
}

func (m *mockAftersalesAppealsAdminClient) ListAppealsForAdmin(ctx context.Context, in *aftersalespb.ListAppealsForAdminRequest, opts ...grpc.CallOption) (*aftersalespb.ListAppealsForAdminResponse, error) {
	return m.ListAppealsForAdminFn(ctx, in, opts...)
}

func setupAftersalesAdminRouter(h *Holder) *gin.Engine {
	r := gin.New()
	admin := r.Group("/api/v1/admin", fakeJWTAuth())
	admin.GET("/after_sales/appeals", h.ListAppealsForAdmin)
	return r
}

func doAftersalesAdminReq(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "4")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// appeal_state 一律是 AfterSaleAppealState 的 Value——回應欄位 appealState 與 DB 都存
// Value，請求端收 Name 會讓同一欄位有兩種表示法（#2104：admin 狀態欄印出 "30"、統計卡恆 0）。
func TestListAppealsForAdmin_ForwardsAppealStateValue(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"未帶 → 空字串交由微服務套預設", "", ""},
		{"APPEALED value", "?appeal_state=10", "10"},
		{"INVESTIGATING value", "?appeal_state=20", "20"},
		{"RESOLVED value", "?appeal_state=30", "30"},
		{"all 關鍵字", "?appeal_state=all", "all"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			mock := &mockAftersalesAppealsAdminClient{
				ListAppealsForAdminFn: func(_ context.Context, in *aftersalespb.ListAppealsForAdminRequest, _ ...grpc.CallOption) (*aftersalespb.ListAppealsForAdminResponse, error) {
					got = in.AppealState
					return &aftersalespb.ListAppealsForAdminResponse{Code: 0, Data: &aftersalespb.ListAppealsForAdminData{Total: "0", Page: "1", PageSize: "20"}}, nil
				},
			}
			h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock)}
			w := doAftersalesAdminReq(t, setupAftersalesAdminRouter(h), "/api/v1/admin/after_sales/appeals"+tc.query)

			assertHTTPStatus(t, w, 200)
			if got != tc.want {
				t.Errorf("ListAppealsForAdminRequest.AppealState = %q, want %q", got, tc.want)
			}
		})
	}
}

// Name（APPEALED / RESOLVED …）必須被擋在 gateway，不得靜默轉送——微服務至今仍收得下
// Name，若 gateway 放行就等於保留一條 Name/Value 雙軌，契約不對稱會再長回來。
func TestListAppealsForAdmin_RejectsNonValueAppealState(t *testing.T) {
	for _, q := range []string{"APPEALED", "RESOLVED", "INVESTIGATING", "appealed", "0", "BOGUS"} {
		t.Run(q, func(t *testing.T) {
			mock := &mockAftersalesAppealsAdminClient{
				ListAppealsForAdminFn: func(context.Context, *aftersalespb.ListAppealsForAdminRequest, ...grpc.CallOption) (*aftersalespb.ListAppealsForAdminResponse, error) {
					t.Fatalf("appeal_state=%q 不該被轉送到 aftersales-service", q)
					return nil, nil
				},
			}
			h := &Holder{Aftersales: grpcclient.NewAftersalesClientForTest(mock)}
			w := doAftersalesAdminReq(t, setupAftersalesAdminRouter(h), "/api/v1/admin/after_sales/appeals?appeal_state="+q)

			assertHTTPStatus(t, w, 400)
		})
	}
}
