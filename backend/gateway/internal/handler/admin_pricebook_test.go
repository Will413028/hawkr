package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pricebookpb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// mockCategoryMappingServiceClient is a dedicated mock for the two admin
// category-mapping RPCs (kept separate from pricebook_test.go's
// mockPricebookServiceClient, which does not override these two methods).
type mockCategoryMappingServiceClient struct {
	pricebookpb.PricebookServiceClient
	UpsertCategoryMappingFn       func(ctx context.Context, in *pricebookpb.UpsertCategoryMappingRequest, opts ...grpc.CallOption) (*pricebookpb.UpsertCategoryMappingResponse, error)
	ListCategoryMappingWorklistFn func(ctx context.Context, in *pricebookpb.ListCategoryMappingWorklistRequest, opts ...grpc.CallOption) (*pricebookpb.ListCategoryMappingWorklistResponse, error)
}

func (m *mockCategoryMappingServiceClient) UpsertCategoryMapping(ctx context.Context, in *pricebookpb.UpsertCategoryMappingRequest, opts ...grpc.CallOption) (*pricebookpb.UpsertCategoryMappingResponse, error) {
	if m.UpsertCategoryMappingFn != nil {
		return m.UpsertCategoryMappingFn(ctx, in, opts...)
	}
	return &pricebookpb.UpsertCategoryMappingResponse{}, nil
}

func (m *mockCategoryMappingServiceClient) ListCategoryMappingWorklist(ctx context.Context, in *pricebookpb.ListCategoryMappingWorklistRequest, opts ...grpc.CallOption) (*pricebookpb.ListCategoryMappingWorklistResponse, error) {
	if m.ListCategoryMappingWorklistFn != nil {
		return m.ListCategoryMappingWorklistFn(ctx, in, opts...)
	}
	return &pricebookpb.ListCategoryMappingWorklistResponse{}, nil
}

// setupAdminPricebookRouter mounts the two admin category-mapping routes
// directly (no JWTAuth/requireAdmin middleware) — matching
// setupAdminNotificationTemplateRouter's convention: admin-role enforcement
// lives at the /admin route-group level in production (routes.go), not in
// these handlers, so handler-level tests set accountId directly and bypass
// it. accountID lets each test control what getAccountID(c) returns.
func setupAdminPricebookRouter(h *Holder, accountID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	m := r.Group("/api/v1/admin/pricebook/category-mapping")
	m.Use(func(c *gin.Context) {
		c.Set("accountId", accountID)
		c.Next()
	})
	m.POST("", h.UpsertCategoryMapping)
	m.GET("", h.ListCategoryMappingWorklist)
	return r
}

func TestUpsertCategoryMapping_InvalidStatus_ReturnsBadRequest(t *testing.T) {
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(&mockCategoryMappingServiceClient{})}
	r := setupAdminPricebookRouter(h, 1)

	body, _ := json.Marshal(map[string]string{
		"categoryId": "cat-1",
		"status":     "not-a-real-status",
	})
	req, _ := http.NewRequest("POST", "/api/v1/admin/pricebook/category-mapping", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestUpsertCategoryMapping_MissingCategoryID_ReturnsBadRequest(t *testing.T) {
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(&mockCategoryMappingServiceClient{})}
	r := setupAdminPricebookRouter(h, 1)

	body, _ := json.Marshal(map[string]string{"status": "unmapped"})
	req, _ := http.NewRequest("POST", "/api/v1/admin/pricebook/category-mapping", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

// TestUpsertCategoryMapping_ValidStatuses_Accepted proves all three oneof
// values pass binding and reach the gRPC client (200), not just that a bad
// value is rejected.
func TestUpsertCategoryMapping_ValidStatuses_Accepted(t *testing.T) {
	for _, status := range []string{"resolved", "unmapped", "ambiguous"} {
		t.Run(status, func(t *testing.T) {
			mock := &mockCategoryMappingServiceClient{
				UpsertCategoryMappingFn: func(ctx context.Context, in *pricebookpb.UpsertCategoryMappingRequest, opts ...grpc.CallOption) (*pricebookpb.UpsertCategoryMappingResponse, error) {
					return &pricebookpb.UpsertCategoryMappingResponse{}, nil
				},
			}
			h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
			r := setupAdminPricebookRouter(h, 1)

			body, _ := json.Marshal(map[string]string{
				"categoryId": "cat-1",
				"status":     status,
			})
			req, _ := http.NewRequest("POST", "/api/v1/admin/pricebook/category-mapping", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestUpsertCategoryMapping_UpdatedByComesFromAccountContextOnly proves the
// operator identity (updated_by) is always taken from getAccountID(c) —
// server-side, from the JWT-derived context — and never from the request
// body: the JSON body has no updatedBy/accountId field at all, yet the
// gRPC request's UpdatedBy still reflects the context accountId.
func TestUpsertCategoryMapping_UpdatedByComesFromAccountContextOnly(t *testing.T) {
	var gotReq *pricebookpb.UpsertCategoryMappingRequest
	mock := &mockCategoryMappingServiceClient{
		UpsertCategoryMappingFn: func(ctx context.Context, in *pricebookpb.UpsertCategoryMappingRequest, opts ...grpc.CallOption) (*pricebookpb.UpsertCategoryMappingResponse, error) {
			gotReq = in
			return &pricebookpb.UpsertCategoryMappingResponse{}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	const wantAccountID int64 = 42
	r := setupAdminPricebookRouter(h, wantAccountID)

	body, _ := json.Marshal(map[string]string{
		"categoryId": "cat-1",
		"status":     "unmapped",
	})
	req, _ := http.NewRequest("POST", "/api/v1/admin/pricebook/category-mapping", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if gotReq == nil {
		t.Fatal("gRPC client was not called")
	}
	if gotReq.GetUpdatedBy() != wantAccountID {
		t.Errorf("UpdatedBy = %d, want %d (must come from getAccountID(c), not request body)", gotReq.GetUpdatedBy(), wantAccountID)
	}
}

func TestListCategoryMappingWorklist_EmptyStatus_NoFilterPassed(t *testing.T) {
	var gotReq *pricebookpb.ListCategoryMappingWorklistRequest
	mock := &mockCategoryMappingServiceClient{
		ListCategoryMappingWorklistFn: func(ctx context.Context, in *pricebookpb.ListCategoryMappingWorklistRequest, opts ...grpc.CallOption) (*pricebookpb.ListCategoryMappingWorklistResponse, error) {
			gotReq = in
			return &pricebookpb.ListCategoryMappingWorklistResponse{}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupAdminPricebookRouter(h, 1)

	req, _ := http.NewRequest("GET", "/api/v1/admin/pricebook/category-mapping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if gotReq == nil {
		t.Fatal("gRPC client was not called")
	}
	if gotReq.GetStatus() != "" {
		t.Errorf("Status = %q, want empty (no filter) when query param omitted", gotReq.GetStatus())
	}
}

func TestListCategoryMappingWorklist_StatusFilter_Passed(t *testing.T) {
	var gotReq *pricebookpb.ListCategoryMappingWorklistRequest
	mock := &mockCategoryMappingServiceClient{
		ListCategoryMappingWorklistFn: func(ctx context.Context, in *pricebookpb.ListCategoryMappingWorklistRequest, opts ...grpc.CallOption) (*pricebookpb.ListCategoryMappingWorklistResponse, error) {
			gotReq = in
			return &pricebookpb.ListCategoryMappingWorklistResponse{}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupAdminPricebookRouter(h, 1)

	req, _ := http.NewRequest("GET", "/api/v1/admin/pricebook/category-mapping?status=unmapped", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if gotReq.GetStatus() != "unmapped" {
		t.Errorf("Status = %q, want \"unmapped\"", gotReq.GetStatus())
	}
}

// TestUpsertCategoryMapping_UsecaseInvalidArgument_SurfacesAs400 證明
// pricebook-service 回 InvalidArgument（形狀驗證的 ErrIncompleteSelection 走這
// 條）在 Gateway 是 400，不是 503——後者正是修正前的行為。
func TestUpsertCategoryMapping_UsecaseInvalidArgument_SurfacesAs400(t *testing.T) {
	mock := &mockCategoryMappingServiceClient{
		UpsertCategoryMappingFn: func(ctx context.Context, in *pricebookpb.UpsertCategoryMappingRequest, opts ...grpc.CallOption) (*pricebookpb.UpsertCategoryMappingResponse, error) {
			return nil, grpcstatus.Error(grpccodes.InvalidArgument, "selection fields incomplete or inconsistent with status")
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupAdminPricebookRouter(h, 1)

	// status=unmapped 但帶了 groupCode：binding tag 表達不了這個跨欄位條件，
	// 所以會通過 gin 驗證、由 usecase 層擋下。
	body, _ := json.Marshal(map[string]string{
		"categoryId": "cat-1", "status": "unmapped", "groupCode": "G1",
	})
	req, _ := http.NewRequest("POST", "/api/v1/admin/pricebook/category-mapping", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (不得是 503), body = %s", w.Code, w.Body.String())
	}
}

// TestListCategoryMappingWorklist_InvalidStatus_ReturnsBadRequest：proto 契約
// 只允許空字串｜unmapped｜ambiguous。修正前 ?status=resolved 會靜默回空清單。
func TestListCategoryMappingWorklist_InvalidStatus_ReturnsBadRequest(t *testing.T) {
	for _, bad := range []string{"resolved", "xxx", "UNMAPPED"} {
		t.Run(bad, func(t *testing.T) {
			var called bool
			mock := &mockCategoryMappingServiceClient{
				ListCategoryMappingWorklistFn: func(ctx context.Context, in *pricebookpb.ListCategoryMappingWorklistRequest, opts ...grpc.CallOption) (*pricebookpb.ListCategoryMappingWorklistResponse, error) {
					called = true
					return &pricebookpb.ListCategoryMappingWorklistResponse{}, nil
				},
			}
			h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
			r := setupAdminPricebookRouter(h, 1)

			req, _ := http.NewRequest("GET", "/api/v1/admin/pricebook/category-mapping?status="+bad, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
			}
			if called {
				t.Error("不合法的 status 不該送到 pricebook-service")
			}
		})
	}
}

// TestListCategoryMappingWorklist_ValidStatuses_Passed 證明驗證沒有把兩個合法
// 值也一起擋掉。
func TestListCategoryMappingWorklist_ValidStatuses_Passed(t *testing.T) {
	for _, ok := range []string{"unmapped", "ambiguous"} {
		t.Run(ok, func(t *testing.T) {
			var gotReq *pricebookpb.ListCategoryMappingWorklistRequest
			mock := &mockCategoryMappingServiceClient{
				ListCategoryMappingWorklistFn: func(ctx context.Context, in *pricebookpb.ListCategoryMappingWorklistRequest, opts ...grpc.CallOption) (*pricebookpb.ListCategoryMappingWorklistResponse, error) {
					gotReq = in
					return &pricebookpb.ListCategoryMappingWorklistResponse{}, nil
				},
			}
			h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
			r := setupAdminPricebookRouter(h, 1)

			req, _ := http.NewRequest("GET", "/api/v1/admin/pricebook/category-mapping?status="+ok, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
			}
			if gotReq.GetStatus() != ok {
				t.Errorf("Status = %q, want %q", gotReq.GetStatus(), ok)
			}
		})
	}
}
