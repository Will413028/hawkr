package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func setupShareRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	app := v1.Group("/app")
	app.Use(fakeJWTAuth())
	app.GET("/share_url", h.GetShareURL)
	return r
}

func setupShareHolder(sellerMock *mockSellerServiceClient, socialMock *mockSocialServiceClient, productMock *mockProductServiceClient) *Holder {
	h := &Holder{}
	if sellerMock != nil {
		h.Seller = grpcclient.NewSellerClientForTest(sellerMock)
	}
	if socialMock != nil {
		h.Social = grpcclient.NewSocialClientForTest(socialMock)
	}
	if productMock != nil {
		h.Product = grpcclient.NewProductClientForTest(productMock)
	}
	return h
}

func TestGetShareURL_Post(t *testing.T) {
	// 賣家共 15 篇貼文，目標貼文 id=123 位於第 12 個（索引 11），
	// 依 websitePostPageSize=10 應計算為 page=2。
	postList := make([]*socialpb.PostListItem, 0, 15)
	for i := 0; i < 15; i++ {
		id := int64(1000 + i)
		if i == 11 {
			id = 123
		}
		postList = append(postList, &socialpb.PostListItem{Id: strconv.FormatInt(id, 10), SellerId: "1001"})
	}
	socialMock := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			return &socialpb.PostListResponse{Code: 0, Data: postList}, nil
		},
	}
	socialMock.GetPostDetailFn = func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
		return &socialpb.PostDetailResponse{
			Data: &socialpb.PostDetailInfo{
				Id:            "123",
				Content:       "好吃的水果推薦",
				SellerId:      "1001",
				SellerName:    "黑貓測試家",
				SellerLogoKey: "https://test-media.example.com/logo.jpg",
				ImageKeys:     []string{"https://test-media.example.com/post1.jpg"},
			},
		}, nil
	}
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{
				Code: 0,
				Data: &sellerpb.SellerInfo{StoreCode: "2601001000"},
			}, nil
		},
	}
	h := setupShareHolder(sellerMock, socialMock, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=post&id=123", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	wantURL := "http://localhost:3000/sellers/2601001000/posts?page=2#post-123"
	if data["url"] != wantURL {
		t.Errorf("url = %q, want %q", data["url"], wantURL)
	}
	if data["title"] != "黑貓測試家 的貼文" {
		t.Errorf("title = %q, want %q", data["title"], "黑貓測試家 的貼文")
	}
	if data["imageUrl"] != "https://test-media.example.com/post1.jpg" {
		t.Errorf("imageUrl = %q, want %q", data["imageUrl"], "https://test-media.example.com/post1.jpg")
	}
}

func TestGetShareURL_Post_SellerResolveFailsFallsBackToStoreID(t *testing.T) {
	socialMock := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			return &socialpb.PostListResponse{Code: 0, Data: nil}, nil
		},
	}
	socialMock.GetPostDetailFn = func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
		return &socialpb.PostDetailResponse{
			Data: &socialpb.PostDetailInfo{Id: "123", SellerId: "1001", SellerName: "黑貓測試家"},
		}, nil
	}
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return nil, fmt.Errorf("seller-service unavailable")
		},
	}
	h := setupShareHolder(sellerMock, socialMock, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=post&id=123", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	wantURL := "http://localhost:3000/sellers/1001/posts?page=1#post-123"
	if data["url"] != wantURL {
		t.Errorf("url = %q, want %q (falls back to raw store_id, share must not hard-fail)", data["url"], wantURL)
	}
}

func TestGetShareURL_Product(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id:       "10001",
					Name:     "有機蘋果 1kg",
					PriceMin: "200",
					ImageKey: "https://test-media.example.com/apple.jpg",
					Seller: &productpb.SellerInfo{
						StoreId: "1001",
						Name:    "黑貓測試家",
					},
				},
			}, nil
		},
	}
	h := setupShareHolder(nil, nil, productMock)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=product&id=10001", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	if data["url"] != "http://localhost:3000/products/10001" {
		t.Errorf("url = %q, want %q", data["url"], "http://localhost:3000/products/10001")
	}
	if data["title"] != "有機蘋果 1kg" {
		t.Errorf("title = %q, want %q", data["title"], "有機蘋果 1kg")
	}
	if data["description"] != "NT$200 - 黑貓測試家" {
		t.Errorf("description = %q, want %q", data["description"], "NT$200 - 黑貓測試家")
	}
	if data["imageUrl"] != "https://test-media.example.com/apple.jpg" {
		t.Errorf("imageUrl = %q, want %q", data["imageUrl"], "https://test-media.example.com/apple.jpg")
	}
}

func TestGetShareURL_Seller(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{
				Code: 0,
				Data: &sellerpb.SellerInfo{
					AccountId: 2619961976,
					StoreCode: "2601001000",
					Name:      "黑貓測試家",
					LogoKey:   "https://test-media.example.com/logo.jpg",
					City:      "台北市",
					District:  "中正區",
					Rating:    4.8,
				},
			}, nil
		},
	}
	h := setupShareHolder(sellerMock, nil, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=seller&id=2619961976", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	if data["url"] != "http://localhost:3000/sellers/2601001000" {
		t.Errorf("url = %q, want %q", data["url"], "http://localhost:3000/sellers/2601001000")
	}
	if data["title"] != "黑貓測試家" {
		t.Errorf("title = %q, want %q", data["title"], "黑貓測試家")
	}
	if data["description"] != "台北市中正區 | 4.8 評分" {
		t.Errorf("description = %q, want %q", data["description"], "台北市中正區 | 4.8 評分")
	}
	if data["imageUrl"] != "https://test-media.example.com/logo.jpg" {
		t.Errorf("imageUrl = %q, want %q", data["imageUrl"], "https://test-media.example.com/logo.jpg")
	}
}

func TestGetShareURL_Seller_UsesStoreIdNotAccountId(t *testing.T) {
	var captured *sellerpb.GetSellerRequest
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			captured = in
			return &sellerpb.SellerResponse{
				Code: 0,
				Data: &sellerpb.SellerInfo{Name: "黑貓測試家"},
			}, nil
		},
	}
	h := setupShareHolder(sellerMock, nil, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=seller&id=7001", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("GetSeller not called")
	}
	if captured.StoreId != 7001 {
		t.Errorf("StoreId = %d, want 7001", captured.StoreId)
	}
	if captured.AccountId != 0 {
		t.Errorf("AccountId = %d, want 0 (must query by StoreId, not AccountId — regression: backend-payuni#1736)", captured.AccountId)
	}
}

func TestGetShareURL_MissingParams(t *testing.T) {
	h := setupShareHolder(nil, nil, nil)
	router := setupShareRouter(h)

	tests := []struct {
		name string
		url  string
	}{
		{"missing type and id", "/api/v1/app/share_url"},
		{"missing id", "/api/v1/app/share_url?type=post"},
		{"missing type", "/api/v1/app/share_url?id=123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			req.Header.Set("X-Account-Id", "9999")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assertHTTPStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestGetShareURL_InvalidType(t *testing.T) {
	h := setupShareHolder(nil, nil, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=unknown&id=123", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestGetShareURL_PostNotFound(t *testing.T) {
	socialMock := &mockSocialServiceClient{}
	socialMock.GetPostDetailFn = func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
		return nil, grpcstatus.Error(grpccodes.NotFound, "post not found")
	}
	h := setupShareHolder(nil, socialMock, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=post&id=999", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
}

func TestGetShareURL_ServiceDown(t *testing.T) {
	socialMock := &mockSocialServiceClient{}
	socialMock.GetPostDetailFn = func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
		return nil, fmt.Errorf("connection refused")
	}
	h := setupShareHolder(nil, socialMock, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=post&id=123", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestGetShareURL_Unauthorized(t *testing.T) {
	h := setupShareHolder(nil, nil, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=post&id=123", nil)
	// No X-Account-Id header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}
