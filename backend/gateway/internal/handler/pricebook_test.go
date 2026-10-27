package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pricebookpb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type mockPricebookServiceClient struct {
	pricebookpb.PricebookServiceClient
	GetObservedPriceFn       func(ctx context.Context, in *pricebookpb.GetObservedPriceRequest, opts ...grpc.CallOption) (*pricebookpb.GetObservedPriceResponse, error)
	GetForecastCurveFn       func(ctx context.Context, in *pricebookpb.GetForecastCurveRequest, opts ...grpc.CallOption) (*pricebookpb.GetForecastCurveResponse, error)
	SearchCropCatalogFn      func(ctx context.Context, in *pricebookpb.SearchCropCatalogRequest, opts ...grpc.CallOption) (*pricebookpb.SearchCropCatalogResponse, error)
	GetTrendSummaryFn        func(ctx context.Context, in *pricebookpb.GetTrendSummaryRequest, opts ...grpc.CallOption) (*pricebookpb.GetTrendSummaryResponse, error)
	GetSpatialSurfaceFn      func(ctx context.Context, in *pricebookpb.GetSpatialSurfaceRequest, opts ...grpc.CallOption) (*pricebookpb.GetSpatialSurfaceResponse, error)
	GetRegionalComparisonFn  func(ctx context.Context, in *pricebookpb.GetRegionalComparisonRequest, opts ...grpc.CallOption) (*pricebookpb.GetRegionalComparisonResponse, error)
	ResolveCategoryMappingFn func(ctx context.Context, in *pricebookpb.ResolveCategoryMappingRequest, opts ...grpc.CallOption) (*pricebookpb.ResolveCategoryMappingResponse, error)
	GetCategoryPriceRollupFn func(ctx context.Context, in *pricebookpb.GetCategoryPriceRollupRequest, opts ...grpc.CallOption) (*pricebookpb.GetCategoryPriceRollupResponse, error)
}

func (m *mockPricebookServiceClient) GetObservedPrice(ctx context.Context, in *pricebookpb.GetObservedPriceRequest, opts ...grpc.CallOption) (*pricebookpb.GetObservedPriceResponse, error) {
	if m.GetObservedPriceFn != nil {
		return m.GetObservedPriceFn(ctx, in, opts...)
	}
	return &pricebookpb.GetObservedPriceResponse{}, nil
}

func (m *mockPricebookServiceClient) GetForecastCurve(ctx context.Context, in *pricebookpb.GetForecastCurveRequest, opts ...grpc.CallOption) (*pricebookpb.GetForecastCurveResponse, error) {
	if m.GetForecastCurveFn != nil {
		return m.GetForecastCurveFn(ctx, in, opts...)
	}
	return &pricebookpb.GetForecastCurveResponse{}, nil
}

func (m *mockPricebookServiceClient) SearchCropCatalog(ctx context.Context, in *pricebookpb.SearchCropCatalogRequest, opts ...grpc.CallOption) (*pricebookpb.SearchCropCatalogResponse, error) {
	if m.SearchCropCatalogFn != nil {
		return m.SearchCropCatalogFn(ctx, in, opts...)
	}
	return &pricebookpb.SearchCropCatalogResponse{}, nil
}

func (m *mockPricebookServiceClient) GetTrendSummary(ctx context.Context, in *pricebookpb.GetTrendSummaryRequest, opts ...grpc.CallOption) (*pricebookpb.GetTrendSummaryResponse, error) {
	if m.GetTrendSummaryFn != nil {
		return m.GetTrendSummaryFn(ctx, in, opts...)
	}
	return &pricebookpb.GetTrendSummaryResponse{}, nil
}

func (m *mockPricebookServiceClient) GetSpatialSurface(ctx context.Context, in *pricebookpb.GetSpatialSurfaceRequest, opts ...grpc.CallOption) (*pricebookpb.GetSpatialSurfaceResponse, error) {
	if m.GetSpatialSurfaceFn != nil {
		return m.GetSpatialSurfaceFn(ctx, in, opts...)
	}
	return &pricebookpb.GetSpatialSurfaceResponse{}, nil
}

func (m *mockPricebookServiceClient) GetRegionalComparison(ctx context.Context, in *pricebookpb.GetRegionalComparisonRequest, opts ...grpc.CallOption) (*pricebookpb.GetRegionalComparisonResponse, error) {
	if m.GetRegionalComparisonFn != nil {
		return m.GetRegionalComparisonFn(ctx, in, opts...)
	}
	return &pricebookpb.GetRegionalComparisonResponse{}, nil
}

func (m *mockPricebookServiceClient) ResolveCategoryMapping(ctx context.Context, in *pricebookpb.ResolveCategoryMappingRequest, opts ...grpc.CallOption) (*pricebookpb.ResolveCategoryMappingResponse, error) {
	if m.ResolveCategoryMappingFn != nil {
		return m.ResolveCategoryMappingFn(ctx, in, opts...)
	}
	return &pricebookpb.ResolveCategoryMappingResponse{}, nil
}

func (m *mockPricebookServiceClient) GetCategoryPriceRollup(ctx context.Context, in *pricebookpb.GetCategoryPriceRollupRequest, opts ...grpc.CallOption) (*pricebookpb.GetCategoryPriceRollupResponse, error) {
	if m.GetCategoryPriceRollupFn != nil {
		return m.GetCategoryPriceRollupFn(ctx, in, opts...)
	}
	return &pricebookpb.GetCategoryPriceRollupResponse{}, nil
}

func setupPricebookRouter(h *Holder) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/app/sellers/pricebook/crop-catalog/search", func(c *gin.Context) {
		c.Set("accountId", int64(1))
		h.SearchCropCatalog(c)
	})
	r.GET("/api/v1/app/sellers/pricebook/observed-price", func(c *gin.Context) {
		c.Set("accountId", int64(1))
		h.GetObservedPriceHistory(c)
	})
	r.GET("/api/v1/app/sellers/pricebook/forecast-curve", func(c *gin.Context) {
		c.Set("accountId", int64(1))
		h.GetForecastCurve(c)
	})
	r.GET("/api/v1/app/sellers/pricebook/trend-summary", func(c *gin.Context) {
		c.Set("accountId", int64(1))
		h.GetTrendSummary(c)
	})
	r.GET("/api/v1/app/sellers/pricebook/spatial-surface", func(c *gin.Context) {
		c.Set("accountId", int64(1))
		h.GetSpatialSurface(c)
	})
	r.GET("/api/v1/app/sellers/pricebook/regional-comparison", func(c *gin.Context) {
		c.Set("accountId", int64(1))
		h.GetRegionalComparison(c)
	})
	r.GET("/api/v1/app/sellers/pricebook/category-resolution", func(c *gin.Context) {
		c.Set("accountId", int64(1))
		h.ResolveCategoryMapping(c)
	})
	r.GET("/api/v1/app/sellers/pricebook/category-price-rollup", func(c *gin.Context) {
		c.Set("accountId", int64(1))
		h.GetCategoryPriceRollup(c)
	})
	return r
}

func TestSearchCropCatalog_PassesQueryAndLimit(t *testing.T) {
	var gotReq *pricebookpb.SearchCropCatalogRequest
	mock := &mockPricebookServiceClient{
		SearchCropCatalogFn: func(ctx context.Context, in *pricebookpb.SearchCropCatalogRequest, opts ...grpc.CallOption) (*pricebookpb.SearchCropCatalogResponse, error) {
			gotReq = in
			return &pricebookpb.SearchCropCatalogResponse{}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/crop-catalog/search?q=%E5%B0%8F%E9%BB%83%E7%93%9C&limit=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if gotReq.GetQuery() != "小黃瓜" || gotReq.GetLimit() != 5 {
		t.Errorf("gotReq = %+v, want Query=小黃瓜 Limit=5", gotReq)
	}
}

func TestGetCropNames_RouteIsGone(t *testing.T) {
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(&mockPricebookServiceClient{})}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/crop-names", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (route removed)", w.Code)
	}
}

func TestGetForecastCurve_MissingCropName_ReturnsBadRequest(t *testing.T) {
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(&mockPricebookServiceClient{})}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/forecast-curve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestGetObservedPriceHistory_PassesHistoryQueryParam(t *testing.T) {
	var gotHistory bool
	mock := &mockPricebookServiceClient{
		GetObservedPriceFn: func(ctx context.Context, in *pricebookpb.GetObservedPriceRequest, opts ...grpc.CallOption) (*pricebookpb.GetObservedPriceResponse, error) {
			gotHistory = in.GetHistory()
			return &pricebookpb.GetObservedPriceResponse{}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/observed-price?cropName=%E9%AB%98%E9%BA%97%E8%8F%9C&history=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if !gotHistory {
		t.Error("expected history=true to reach the gRPC request")
	}
}

func TestGetObservedPriceHistory_PassesCropCodeAndGroupCode(t *testing.T) {
	var gotReq *pricebookpb.GetObservedPriceRequest
	mock := &mockPricebookServiceClient{
		GetObservedPriceFn: func(ctx context.Context, in *pricebookpb.GetObservedPriceRequest, opts ...grpc.CallOption) (*pricebookpb.GetObservedPriceResponse, error) {
			gotReq = in
			return &pricebookpb.GetObservedPriceResponse{}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/observed-price?cropCode=crop1&marketCode=109", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if gotReq.GetCropCode() != "crop1" || gotReq.GetMarketCode() != "109" {
		t.Errorf("gotReq = %+v, want CropCode=crop1 MarketCode=109", gotReq)
	}
}

func TestGetTrendSummary_RequiresExactlyOneSelector(t *testing.T) {
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(&mockPricebookServiceClient{})}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/trend-summary?from=2026-08-01&to=2026-08-01&granularity=day", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestGetTrendSummary_PassesCropCodeMarketCodeAndComparisons(t *testing.T) {
	var gotReq *pricebookpb.GetTrendSummaryRequest
	mock := &mockPricebookServiceClient{
		GetTrendSummaryFn: func(ctx context.Context, in *pricebookpb.GetTrendSummaryRequest, opts ...grpc.CallOption) (*pricebookpb.GetTrendSummaryResponse, error) {
			gotReq = in
			return &pricebookpb.GetTrendSummaryResponse{SelectionKind: "crop", Granularity: "day", Timezone: "Asia/Taipei"}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/trend-summary?cropCode=A1&marketCode=109&from=2026-08-01&to=2026-08-01&granularity=day&comparisons=wow,yoy364", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if gotReq.GetCropCode() != "A1" || gotReq.GetMarketCode() != "109" {
		t.Errorf("gotReq = %+v, want CropCode=A1 MarketCode=109", gotReq)
	}
	if len(gotReq.GetComparisons()) != 2 || gotReq.GetComparisons()[0] != "wow" || gotReq.GetComparisons()[1] != "yoy364" {
		t.Errorf("gotReq.Comparisons = %v, want [wow yoy364]", gotReq.GetComparisons())
	}
}

func TestGetSpatialSurface_RequiresExactlyOneSelector(t *testing.T) {
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(&mockPricebookServiceClient{})}
	r := setupPricebookRouter(h)

	for name, query := range map[string]string{
		"neither": "horizonDays=1",
		"both":    "cropCode=A1&groupCode=G1&horizonDays=1",
	} {
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/spatial-surface?"+query, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400, body = %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGetSpatialSurface_PassesCropCodeAndHorizonDays(t *testing.T) {
	var gotReq *pricebookpb.GetSpatialSurfaceRequest
	mock := &mockPricebookServiceClient{
		GetSpatialSurfaceFn: func(ctx context.Context, in *pricebookpb.GetSpatialSurfaceRequest, opts ...grpc.CallOption) (*pricebookpb.GetSpatialSurfaceResponse, error) {
			gotReq = in
			return &pricebookpb.GetSpatialSurfaceResponse{Status: "ready"}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/spatial-surface?cropCode=A1&horizonDays=3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if gotReq.GetCropCode() != "A1" || gotReq.GetHorizonDays() != 3 {
		t.Errorf("gotReq = %+v, want CropCode=A1 HorizonDays=3", gotReq)
	}
}

func TestGetSpatialSurface_SetsETagAndHonoursIfNoneMatch(t *testing.T) {
	mock := &mockPricebookServiceClient{
		GetSpatialSurfaceFn: func(ctx context.Context, in *pricebookpb.GetSpatialSurfaceRequest, opts ...grpc.CallOption) (*pricebookpb.GetSpatialSurfaceResponse, error) {
			return &pricebookpb.GetSpatialSurfaceResponse{Status: "ready", TargetDate: "2026-08-09"}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/spatial-surface?cropCode=A1&horizonDays=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header on 200 response")
	}
	if w.Header().Get("Cache-Control") == "" {
		t.Error("expected non-empty Cache-Control header on 200 response")
	}

	req2, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/spatial-surface?cropCode=A1&horizonDays=1", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304, body = %s", w2.Code, w2.Body.String())
	}
	if w2.Header().Get("ETag") != etag {
		t.Errorf("304 ETag = %q, want %q", w2.Header().Get("ETag"), etag)
	}
	if w2.Header().Get("Cache-Control") == "" {
		t.Error("expected non-empty Cache-Control header on 304 response")
	}
}

func TestGetSpatialSurface_GzipsWhenAcceptEncodingPresent(t *testing.T) {
	mock := &mockPricebookServiceClient{
		GetSpatialSurfaceFn: func(ctx context.Context, in *pricebookpb.GetSpatialSurfaceRequest, opts ...grpc.CallOption) (*pricebookpb.GetSpatialSurfaceResponse, error) {
			return &pricebookpb.GetSpatialSurfaceResponse{Status: "ready", TargetDate: "2026-08-09"}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/spatial-surface?cropCode=A1&horizonDays=1", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", w.Header().Get("Content-Encoding"))
	}
	gz, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("body is not valid gzip: %v", err)
	}
	body, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("reading gzip body: %v", err)
	}
	if !bytes.Contains(body, []byte(`"status":"ready"`)) {
		t.Errorf("decompressed body = %s, want it to contain the envelope's status field", body)
	}
}

func TestGetRegionalComparison_RequiresExactlyOneSelector(t *testing.T) {
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(&mockPricebookServiceClient{})}
	r := setupPricebookRouter(h)

	for name, query := range map[string]string{
		"neither": "horizonDays=1",
		"both":    "cropCode=A1&groupCode=G1&horizonDays=1",
	} {
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/regional-comparison?"+query, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400, body = %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGetRegionalComparison_PassesCropCodeAndHorizonDays(t *testing.T) {
	var gotReq *pricebookpb.GetRegionalComparisonRequest
	mock := &mockPricebookServiceClient{
		GetRegionalComparisonFn: func(ctx context.Context, in *pricebookpb.GetRegionalComparisonRequest, opts ...grpc.CallOption) (*pricebookpb.GetRegionalComparisonResponse, error) {
			gotReq = in
			return &pricebookpb.GetRegionalComparisonResponse{SelectionKind: "crop"}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/regional-comparison?cropCode=A1&horizonDays=3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if gotReq.GetCropCode() != "A1" || gotReq.GetHorizonDays() != 3 {
		t.Errorf("gotReq = %+v, want CropCode=A1 HorizonDays=3", gotReq)
	}
}

func TestResolveCategoryMapping_MissingCategoryID_ReturnsBadRequest(t *testing.T) {
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(&mockPricebookServiceClient{})}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/category-resolution", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

// TestResolveCategoryMapping_HandlerHasNoInlineCapabilityCheck verifies the
// handler function itself performs no capability/role check of its own —
// capability enforcement for this route lives entirely in the router-layer
// requireSeller(authz.CapMarketing) middleware (see
// routes_pricebook.go／router.TestSellerCapabilityRegistryMatchesMatrix),
// not here. setupPricebookRouter's test route only ever sets "accountId" in
// the gin.Context (no capability/role claim), so a 200 here is proof the
// handler doesn't duplicate that check.
func TestResolveCategoryMapping_HandlerHasNoInlineCapabilityCheck(t *testing.T) {
	mock := &mockPricebookServiceClient{
		ResolveCategoryMappingFn: func(ctx context.Context, in *pricebookpb.ResolveCategoryMappingRequest, opts ...grpc.CallOption) (*pricebookpb.ResolveCategoryMappingResponse, error) {
			return &pricebookpb.ResolveCategoryMappingResponse{CategoryId: in.GetCategoryId(), Status: "unmapped"}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/category-resolution?category_id=cat-123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (handler itself has no inline capability check), body = %s", w.Code, w.Body.String())
	}
}

func TestGetCategoryPriceRollup_MissingCategoryID_ReturnsBadRequest(t *testing.T) {
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(&mockPricebookServiceClient{})}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/category-price-rollup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

// TestGetCategoryPriceRollup_PassesThroughGrpcResponse verifies a successful
// gRPC response reaches the caller through response.OKProto's envelope/JSON
// shape unmodified — no handler-side reshaping of category_price_rollup's
// fields.
func TestGetCategoryPriceRollup_PassesThroughGrpcResponse(t *testing.T) {
	var gotReq *pricebookpb.GetCategoryPriceRollupRequest
	mock := &mockPricebookServiceClient{
		GetCategoryPriceRollupFn: func(ctx context.Context, in *pricebookpb.GetCategoryPriceRollupRequest, opts ...grpc.CallOption) (*pricebookpb.GetCategoryPriceRollupResponse, error) {
			gotReq = in
			latest := "42.5000"
			return &pricebookpb.GetCategoryPriceRollupResponse{
				CategoryId: in.GetCategoryId(), Level: 2, Status: "ready",
				LatestPrice: &latest, SourceGroupCount: 3, ComputedAt: "2026-08-30T01:00:00Z",
			}, nil
		},
	}
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)}
	r := setupPricebookRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/pricebook/category-price-rollup?category_id=cat-l2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	if gotReq.GetCategoryId() != "cat-l2" {
		t.Errorf("gotReq.CategoryId = %q, want cat-l2", gotReq.GetCategoryId())
	}
	body := w.Body.String()
	for _, want := range []string{`"categoryId":"cat-l2"`, `"level":2`, `"status":"ready"`, `"latestPrice":"42.5000"`, `"sourceGroupCount":3`, `"computedAt":"2026-08-30T01:00:00Z"`} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Errorf("response body = %s, want it to contain %s", body, want)
		}
	}
}
