package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Mock ReviewServiceClient ====================

type reviewMockClient struct {
	GetProductReviewsFn         func(ctx context.Context, in *pb.GetProductReviewsRequest, opts ...grpc.CallOption) (*pb.ProductReviewsResponse, error)
	SubmitOrderReviewFn         func(ctx context.Context, in *pb.SubmitOrderReviewRequest, opts ...grpc.CallOption) (*pb.SubmitOrderReviewResponse, error)
	ListMyOrderReviewsFn        func(ctx context.Context, in *pb.ListMyOrderReviewsRequest, opts ...grpc.CallOption) (*pb.ListMyOrderReviewsResponse, error)
	GetSellerOrderReviewsFn     func(ctx context.Context, in *pb.GetSellerOrderReviewsRequest, opts ...grpc.CallOption) (*pb.GetSellerOrderReviewsResponse, error)
	GetSellerOrderReviewStatsFn func(ctx context.Context, in *pb.GetSellerOrderReviewStatsRequest, opts ...grpc.CallOption) (*pb.GetSellerOrderReviewStatsResponse, error)
	ReplyReviewFn               func(ctx context.Context, in *pb.ReplyReviewRequest, opts ...grpc.CallOption) (*pb.ReviewResponse, error)
}

func (m *reviewMockClient) GetProductReviews(ctx context.Context, in *pb.GetProductReviewsRequest, opts ...grpc.CallOption) (*pb.ProductReviewsResponse, error) {
	if m.GetProductReviewsFn != nil {
		return m.GetProductReviewsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *reviewMockClient) GetReview(ctx context.Context, in *pb.GetReviewRequest, opts ...grpc.CallOption) (*pb.ReviewResponse, error) {
	return nil, nil
}
func (m *reviewMockClient) ReplyReview(ctx context.Context, in *pb.ReplyReviewRequest, opts ...grpc.CallOption) (*pb.ReviewResponse, error) {
	if m.ReplyReviewFn != nil {
		return m.ReplyReviewFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *reviewMockClient) SubmitOrderReview(ctx context.Context, in *pb.SubmitOrderReviewRequest, opts ...grpc.CallOption) (*pb.SubmitOrderReviewResponse, error) {
	if m.SubmitOrderReviewFn != nil {
		return m.SubmitOrderReviewFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *reviewMockClient) ListMyOrderReviews(ctx context.Context, in *pb.ListMyOrderReviewsRequest, opts ...grpc.CallOption) (*pb.ListMyOrderReviewsResponse, error) {
	if m.ListMyOrderReviewsFn != nil {
		return m.ListMyOrderReviewsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *reviewMockClient) GetSellerOrderReviews(ctx context.Context, in *pb.GetSellerOrderReviewsRequest, opts ...grpc.CallOption) (*pb.GetSellerOrderReviewsResponse, error) {
	if m.GetSellerOrderReviewsFn != nil {
		return m.GetSellerOrderReviewsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *reviewMockClient) GetSellerOrderReviewStats(ctx context.Context, in *pb.GetSellerOrderReviewStatsRequest, opts ...grpc.CallOption) (*pb.GetSellerOrderReviewStatsResponse, error) {
	if m.GetSellerOrderReviewStatsFn != nil {
		return m.GetSellerOrderReviewStatsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *reviewMockClient) HideReview(ctx context.Context, in *pb.HideReviewRequest, opts ...grpc.CallOption) (*pb.HideReviewResponse, error) {
	return nil, nil
}
func (m *reviewMockClient) ListStoreRatingStats(ctx context.Context, in *pb.ListStoreRatingStatsRequest, opts ...grpc.CallOption) (*pb.ListStoreRatingStatsResponse, error) {
	return nil, nil
}

// ==================== Router Setup ====================

func setupReviewRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	pub := v1.Group("/public/products")
	pub.Use(fakeOptionalAuth())
	pub.GET("/:id/reviews", h.GetProductReviews)
	return r
}

// ==================== GetProductReviews ====================

func TestGetProductReviews_MapsReviewFields(t *testing.T) {
	mock := &reviewMockClient{
		GetProductReviewsFn: func(ctx context.Context, in *pb.GetProductReviewsRequest, opts ...grpc.CallOption) (*pb.ProductReviewsResponse, error) {
			return &pb.ProductReviewsResponse{
				Code: 0,
				Reviews: []*pb.ReviewInfo{
					{
						Id:            "123456",
						ProductId:     "843856384063471789",
						BuyerId:       "789",
						UserName:      "Cora Chou",
						UserAvatarKey: "https://cdn.example.com/avatar.jpg",
						Rating:        5,
						Content:       "新鮮好吃",
						ImageKeys:     []string{"https://cdn.example.com/img1.jpg", "https://cdn.example.com/img2.jpg"},
						CreatedAt:     "2026-01-01T00:00:00Z",
					},
				},
				Total:    1,
				Page:     1,
				PageSize: 20,
			}, nil
		},
	}

	h := &Holder{
		Review: grpcclient.NewReviewClientForTest(mock),
	}

	router := setupReviewRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/42/reviews", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	var resp struct {
		Data struct {
			Records []ReviewListItem `json:"records"`
		} `json:"data"`
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.Data.Records) != 1 {
		t.Fatalf("data.records len = %d, want 1", len(resp.Data.Records))
	}
	item := resp.Data.Records[0]

	if item.ReviewID != "123456" {
		t.Errorf("reviewId = %q, want %q", item.ReviewID, "123456")
	}
	if item.BuyerID != "789" {
		t.Errorf("buyerId = %q, want %q", item.BuyerID, "789")
	}
	if item.ProductID != "843856384063471789" {
		t.Errorf("productId = %q, want %q", item.ProductID, "843856384063471789")
	}
	// "Cora Chou" → "Co** Ch**"
	if item.Username != "Co** Ch**" {
		t.Errorf("username = %q, want %q", item.Username, "Co** Ch**")
	}
	if item.UserAvatar != "https://cdn.example.com/avatar.jpg" {
		t.Errorf("userAvatar = %q, want non-empty", item.UserAvatar)
	}
	if item.Rating != "5" {
		t.Errorf("rating = %q, want %q", item.Rating, "5")
	}
	if item.Content != "新鮮好吃" {
		t.Errorf("content = %q, want %q", item.Content, "新鮮好吃")
	}
	if len(item.Images) != 2 {
		t.Errorf("images len = %d, want 2", len(item.Images))
	}
}

func TestGetProductReviews_BuyerRoleResolvesViewerID(t *testing.T) {
	var captured *pb.GetProductReviewsRequest
	mock := &reviewMockClient{
		GetProductReviewsFn: func(ctx context.Context, in *pb.GetProductReviewsRequest, opts ...grpc.CallOption) (*pb.ProductReviewsResponse, error) {
			captured = in
			return &pb.ProductReviewsResponse{Code: 0, Reviews: []*pb.ReviewInfo{}}, nil
		},
	}
	h := &Holder{
		Review:  grpcclient.NewReviewClientForTest(mock),
		BuyerID: buyerResolverForTest(9999),
	}
	router := setupReviewRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/42/reviews", nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Login-Role", "1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ViewerId != 9999 {
		t.Errorf("captured.ViewerId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

func TestGetProductReviews_AnonymousViewerIDZero(t *testing.T) {
	var captured *pb.GetProductReviewsRequest
	mock := &reviewMockClient{
		GetProductReviewsFn: func(ctx context.Context, in *pb.GetProductReviewsRequest, opts ...grpc.CallOption) (*pb.ProductReviewsResponse, error) {
			captured = in
			return &pb.ProductReviewsResponse{Code: 0, Reviews: []*pb.ReviewInfo{}}, nil
		},
	}
	h := &Holder{Review: grpcclient.NewReviewClientForTest(mock)}
	router := setupReviewRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/42/reviews", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ViewerId != 0 {
		t.Errorf("captured.ViewerId = %v, want 0 (anonymous caller, no X-Account-Id header)", captured)
	}
}

func TestGetProductReviews_EmptyImages(t *testing.T) {
	mock := &reviewMockClient{
		GetProductReviewsFn: func(ctx context.Context, in *pb.GetProductReviewsRequest, opts ...grpc.CallOption) (*pb.ProductReviewsResponse, error) {
			return &pb.ProductReviewsResponse{
				Code: 0,
				Reviews: []*pb.ReviewInfo{
					{
						Id:        "1",
						BuyerId:   "2",
						Rating:    3,
						Content:   "普通",
						ImageKeys: nil,
					},
				},
				Total:    1,
				Page:     1,
				PageSize: 20,
			}, nil
		},
	}

	h := &Holder{Review: grpcclient.NewReviewClientForTest(mock)}
	router := setupReviewRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/1/reviews", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	var resp struct {
		Data struct {
			Records []ReviewListItem `json:"records"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(resp.Data.Records) == 0 {
		t.Fatal("data.records empty")
	}
	if resp.Data.Records[0].Images == nil {
		t.Error("images should be [] not null")
	}
	if len(resp.Data.Records[0].Images) != 0 {
		t.Errorf("images len = %d, want 0", len(resp.Data.Records[0].Images))
	}
}

// TestGetProductReviews_PageParamParsing pins page/size fallback behavior.
// rating is deliberately excluded — 0 is a valid "no filter" sentinel there,
// not a parse-error fallback target, so it keeps its current strconv.Atoi parse.
func TestGetProductReviews_PageParamParsing(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantPage     int32
		wantPageSize int32
	}{
		{"合法數字", "page=3&size=5", 3, 5},
		{"非數字字串 fallback 到 default", "page=abc&size=xyz", 1, 10},
		{"空值 fallback 到 default", "", 1, 10},
		{"負數 fallback 到 default", "page=-1&size=-5", 1, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured *pb.GetProductReviewsRequest
			mock := &reviewMockClient{
				GetProductReviewsFn: func(ctx context.Context, in *pb.GetProductReviewsRequest, opts ...grpc.CallOption) (*pb.ProductReviewsResponse, error) {
					captured = in
					return &pb.ProductReviewsResponse{Code: 0}, nil
				},
			}
			h := &Holder{Review: grpcclient.NewReviewClientForTest(mock)}
			r := setupReviewRouter(h)
			url := "/api/v1/public/products/857045288972403572/reviews"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assertHTTPStatus(t, w, http.StatusOK)
			if captured == nil {
				t.Fatal("GetProductReviews was not called")
			}
			if captured.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", captured.Page, tt.wantPage)
			}
			if captured.PageSize != tt.wantPageSize {
				t.Errorf("PageSize = %d, want %d", captured.PageSize, tt.wantPageSize)
			}
		})
	}
}

// ==================== SubmitOrderItemReview (TSID precision) ====================

func setupSubmitReviewRouter(h *Holder, accountID int64) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("accountId", accountID) })
	r.POST("/api/v1/app/reviews", h.SubmitOrderItemReview)
	return r
}

// Regression: order_item_id / product_id are 64-bit TSIDs that exceed
// Number.MAX_SAFE_INTEGER. The web client sends them as JSON *strings* to avoid
// float64 rounding; the gateway must parse the string into int64 exactly via
// the `,string` json tag, otherwise review-service can no longer match the item
// against the order and replies 400 "item not in order".
func TestSubmitOrderItemReview_ParsesStringTSID(t *testing.T) {
	const (
		wantOrderItemID = "853917025239746789"
		wantProductID   = "843856384063471789"
	)
	var captured *pb.SubmitOrderReviewRequest
	mock := &reviewMockClient{
		SubmitOrderReviewFn: func(ctx context.Context, in *pb.SubmitOrderReviewRequest, opts ...grpc.CallOption) (*pb.SubmitOrderReviewResponse, error) {
			captured = in
			return &pb.SubmitOrderReviewResponse{Code: 0, ReviewId: "1"}, nil
		},
	}
	h := &Holder{OrderReview: grpcclient.NewOrderReviewClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupSubmitReviewRouter(h, 555)

	body := `{"orderNo":"O214578729","orderItemId":"853917025239746789","productId":"843856384063471789","rating":5,"content":"good"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// #2198：提交評論已從 HTTP 201 收斂成統一的 200 + envelope（業務結果讀 code，不用 status 分流）
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("SubmitOrderReview was not called")
	}
	if captured.OrderItemId != wantOrderItemID {
		t.Errorf("OrderItemId = %s, want %s", captured.OrderItemId, wantOrderItemID)
	}
	if captured.ProductId != wantProductID {
		t.Errorf("ProductId = %s, want %s", captured.ProductId, wantProductID)
	}
}

// The `,string` tag makes the string form the contract on purpose (every web
// review carries a >2^53 TSID). A bare JSON number must be rejected at bind, so
// dropping the tag — or a client reverting to a rounded number — fails loudly
// here instead of silently mismatching the order. Revisit this test (not just
// delete it) if the contract is ever widened to accept both forms.
func TestSubmitOrderItemReview_RejectsNumericTSID(t *testing.T) {
	called := false
	mock := &reviewMockClient{
		SubmitOrderReviewFn: func(ctx context.Context, in *pb.SubmitOrderReviewRequest, opts ...grpc.CallOption) (*pb.SubmitOrderReviewResponse, error) {
			called = true
			return &pb.SubmitOrderReviewResponse{Code: 0, ReviewId: "1"}, nil
		},
	}
	h := &Holder{OrderReview: grpcclient.NewOrderReviewClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupSubmitReviewRouter(h, 555)

	body := `{"orderNo":"O1","orderItemId":853917025239746789,"rating":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if called {
		t.Error("review-service should not be called when binding fails")
	}
}

func TestSubmitOrderItemReview_UsesResolvedBuyerID(t *testing.T) {
	var captured *pb.SubmitOrderReviewRequest
	mock := &reviewMockClient{
		SubmitOrderReviewFn: func(ctx context.Context, in *pb.SubmitOrderReviewRequest, opts ...grpc.CallOption) (*pb.SubmitOrderReviewResponse, error) {
			captured = in
			return &pb.SubmitOrderReviewResponse{Code: 0, ReviewId: "1"}, nil
		},
	}
	h := &Holder{OrderReview: grpcclient.NewOrderReviewClientForTest(mock), BuyerID: buyerResolverForTest(9999)}
	router := setupSubmitReviewRouter(h, 555)

	body := `{"orderNo":"O214578729","orderItemId":"853917025239746789","productId":"843856384063471789","rating":5,"content":"good"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// #2198：提交評論已從 HTTP 201 收斂成統一的 200 + envelope（業務結果讀 code，不用 status 分流）
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("SubmitOrderReview was not called")
	}
	if captured.BuyerId != 9999 {
		t.Errorf("BuyerId = %d, want 9999 (resolved buyer id, not raw accountId 555)", captured.BuyerId)
	}
}

// ==================== ListMyOrderReviews ====================

func setupListMyOrderReviewsRouter(h *Holder, accountID int64) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("accountId", accountID) })
	r.GET("/api/v1/app/me/reviews", h.ListMyOrderReviews)
	return r
}

func TestListMyOrderReviews_UsesResolvedBuyerID(t *testing.T) {
	var captured *pb.ListMyOrderReviewsRequest
	mock := &reviewMockClient{
		ListMyOrderReviewsFn: func(ctx context.Context, in *pb.ListMyOrderReviewsRequest, opts ...grpc.CallOption) (*pb.ListMyOrderReviewsResponse, error) {
			captured = in
			return &pb.ListMyOrderReviewsResponse{Code: 0, Records: nil, Total: "0", Page: "1", PageSize: "20"}, nil
		},
	}
	h := &Holder{OrderReview: grpcclient.NewOrderReviewClientForTest(mock), BuyerID: buyerResolverForTest(9999)}
	router := setupListMyOrderReviewsRouter(h, 555)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/me/reviews", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("ListMyOrderReviews was not called")
	}
	if captured.BuyerId != 9999 {
		t.Errorf("BuyerId = %d, want 9999 (resolved buyer id, not raw accountId 555)", captured.BuyerId)
	}
}

// ==================== maskUsername ====================

func TestMaskUsername(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"A", "A"},
		{"AB", "A*"},
		{"ABC", "A**"},
		{"ABCD", "AB**"},
		{"Cora Chou", "Co** Ch**"},
		{"王小明", "王**"},
		{"李四", "李*"},
	}
	for _, c := range cases {
		got := maskUsername(c.input)
		if got != c.want {
			t.Errorf("maskUsername(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// 評價的父資源檢查：店家此刻不可曝光（can_accept_orders=false）時回 404，
// 且不得讀 review-service。
func TestGetPublicSellerReviews_NonExposableStore_Returns404(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Data: &sellerpb.SellerInfo{StoreId: "2683382637"}}, nil
		},
		BatchGetSellerMetaFn: func(ctx context.Context, in *sellerpb.BatchGetSellerMetaRequest, opts ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
			return &sellerpb.BatchGetSellerMetaResponse{Sellers: map[int64]*sellerpb.SellerMeta{
				2683382637: {CanAcceptOrders: false, HasPublicCard: true},
			}}, nil
		},
	}
	reviewMock := &reviewMockClient{
		GetSellerOrderReviewsFn: func(ctx context.Context, in *pb.GetSellerOrderReviewsRequest, opts ...grpc.CallOption) (*pb.GetSellerOrderReviewsResponse, error) {
			t.Fatal("review-service must not be read for a non-exposable store")
			return nil, nil
		},
		GetSellerOrderReviewStatsFn: func(ctx context.Context, in *pb.GetSellerOrderReviewStatsRequest, opts ...grpc.CallOption) (*pb.GetSellerOrderReviewStatsResponse, error) {
			t.Fatal("review-service must not be read for a non-exposable store")
			return nil, nil
		},
	}
	h := &Holder{
		Seller:      grpcclient.NewSellerClientForTest(sellerMock),
		OrderReview: grpcclient.NewOrderReviewClientForTest(reviewMock),
	}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id/reviews", h.GetPublicSellerReviews)
	r.GET("/api/v1/public/sellers/:id/reviews/stats", h.GetPublicSellerReviewStats)

	for _, path := range []string{
		"/api/v1/public/sellers/2683382637/reviews",
		"/api/v1/public/sellers/2683382637/reviews/stats",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		assertHTTPStatus(t, w, http.StatusNotFound)
	}
}

// ==================== GetPublicSellerReviews (productId mapping, #1155) ====================

func setupPublicSellerReviewsRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id/reviews", h.GetPublicSellerReviews)
	return r
}

// #1155: PublicSellerReviewItem must expose productId so the web seller-reviews
// page can link each review to /products/{productId}. Legacy rows with
// product_id="0" must map to empty so the frontend renders no dead /products/0 link.
func TestGetPublicSellerReviews_MapsProductID(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{
				Code: 0,
				Data: &sellerpb.SellerInfo{StoreId: "2683382637"},
			}, nil
		},
	}
	mock := &reviewMockClient{
		GetSellerOrderReviewsFn: func(ctx context.Context, in *pb.GetSellerOrderReviewsRequest, opts ...grpc.CallOption) (*pb.GetSellerOrderReviewsResponse, error) {
			return &pb.GetSellerOrderReviewsResponse{
				Code: 0,
				Records: []*pb.OrderReviewInfo{
					{Id: "1", ProductId: "843856384063471789", Rating: 5, Content: "好", BuyerName: "小王"},
					{Id: "2", ProductId: "0", Rating: 4, Content: "舊資料", BuyerName: ""}, // legacy → 留空
				},
				Total:    "2",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		Seller:      grpcclient.NewSellerClientForTest(sellerMock),
		OrderReview: grpcclient.NewOrderReviewClientForTest(mock),
	}
	router := setupPublicSellerReviewsRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/sellers/2683382637/reviews", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	var resp struct {
		Data struct {
			Records []PublicSellerReviewItem `json:"records"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(resp.Data.Records) != 2 {
		t.Fatalf("records len = %d, want 2", len(resp.Data.Records))
	}
	if resp.Data.Records[0].ProductID != "843856384063471789" {
		t.Errorf("records[0].productId = %q, want %q", resp.Data.Records[0].ProductID, "843856384063471789")
	}
	if resp.Data.Records[1].ProductID != "" {
		t.Errorf("records[1].productId = %q, want empty (legacy product_id=0 must be omitted)", resp.Data.Records[1].ProductID)
	}
	// 匿名買家 fallback 既有行為不回歸
	if resp.Data.Records[1].UserName != "匿名買家" {
		t.Errorf("records[1].userName = %q, want 匿名買家", resp.Data.Records[1].UserName)
	}
}

// TestOrderReviewHandlers_PageParamParsing covers all three order_review.go
// pagination call sites in one table: rating (GetSellerOrderReviews' third
// field) is out of scope, same reasoning as TestGetProductReviews_PageParamParsing.
func TestOrderReviewHandlers_PageParamParsing(t *testing.T) {
	scenarios := []struct {
		name         string
		query        string
		wantPage     int32
		wantPageSize int32
	}{
		{"合法數字", "page=3&size=5", 3, 5},
		{"非數字字串 fallback 到 default", "page=abc&size=xyz", 1, 10},
		{"空值 fallback 到 default", "", 1, 10},
		{"負數 fallback 到 default", "page=-1&size=-5", 1, 10},
	}

	t.Run("ListMyOrderReviews", func(t *testing.T) {
		for _, tt := range scenarios {
			t.Run(tt.name, func(t *testing.T) {
				var captured *pb.ListMyOrderReviewsRequest
				mock := &reviewMockClient{
					ListMyOrderReviewsFn: func(ctx context.Context, in *pb.ListMyOrderReviewsRequest, opts ...grpc.CallOption) (*pb.ListMyOrderReviewsResponse, error) {
						captured = in
						return &pb.ListMyOrderReviewsResponse{Code: 0}, nil
					},
				}
				h := &Holder{OrderReview: grpcclient.NewOrderReviewClientForTest(mock), BuyerID: buyerResolverForTest(9999)}
				r := setupListMyOrderReviewsRouter(h, 555)
				url := "/api/v1/app/me/reviews"
				if tt.query != "" {
					url += "?" + tt.query
				}
				req := httptest.NewRequest(http.MethodGet, url, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assertHTTPStatus(t, w, http.StatusOK)
				if captured == nil {
					t.Fatal("ListMyOrderReviews was not called")
				}
				if captured.Page != tt.wantPage || captured.PageSize != tt.wantPageSize {
					t.Errorf("Page/PageSize = %d/%d, want %d/%d", captured.Page, captured.PageSize, tt.wantPage, tt.wantPageSize)
				}
			})
		}
	})

	t.Run("GetPublicSellerReviews", func(t *testing.T) {
		// This handler's query param is page_size (see its own Swagger @Param
		// at order_review.go:282), not size like the other two handlers in this
		// table — deviating from the shared `scenarios` table's query string here
		// to match the actual contract instead of reusing "size=" verbatim.
		publicScenarios := []struct {
			name         string
			query        string
			wantPage     int32
			wantPageSize int32
		}{
			{"合法數字", "page=3&page_size=5", 3, 5},
			{"非數字字串 fallback 到 default", "page=abc&page_size=xyz", 1, 10},
			{"空值 fallback 到 default", "", 1, 10},
			{"負數 fallback 到 default", "page=-1&page_size=-5", 1, 10},
		}
		for _, tt := range publicScenarios {
			t.Run(tt.name, func(t *testing.T) {
				sellerMock := &mockSellerServiceClient{
					GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
						return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{StoreId: "2683382637"}}, nil
					},
				}
				var captured *pb.GetSellerOrderReviewsRequest
				mock := &reviewMockClient{
					GetSellerOrderReviewsFn: func(ctx context.Context, in *pb.GetSellerOrderReviewsRequest, opts ...grpc.CallOption) (*pb.GetSellerOrderReviewsResponse, error) {
						captured = in
						return &pb.GetSellerOrderReviewsResponse{Code: 0}, nil
					},
				}
				h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock), OrderReview: grpcclient.NewOrderReviewClientForTest(mock)}
				r := setupPublicSellerReviewsRouter(h)
				url := "/api/v1/public/sellers/2683382637/reviews"
				if tt.query != "" {
					url += "?" + tt.query
				}
				req := httptest.NewRequest(http.MethodGet, url, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assertHTTPStatus(t, w, http.StatusOK)
				if captured == nil {
					t.Fatal("GetSellerOrderReviews (public) was not called")
				}
				if captured.Page != tt.wantPage || captured.PageSize != tt.wantPageSize {
					t.Errorf("Page/PageSize = %d/%d, want %d/%d", captured.Page, captured.PageSize, tt.wantPage, tt.wantPageSize)
				}
			})
		}
	})

	t.Run("GetSellerOrderReviews_app", func(t *testing.T) {
		for _, tt := range scenarios {
			t.Run(tt.name, func(t *testing.T) {
				var captured *pb.GetSellerOrderReviewsRequest
				mock := &reviewMockClient{
					GetSellerOrderReviewsFn: func(ctx context.Context, in *pb.GetSellerOrderReviewsRequest, opts ...grpc.CallOption) (*pb.GetSellerOrderReviewsResponse, error) {
						captured = in
						return &pb.GetSellerOrderReviewsResponse{Code: 0}, nil
					},
				}
				h := &Holder{OrderReview: grpcclient.NewOrderReviewClientForTest(mock), Store: grandfatherResolverForTest()}
				r := gin.New()
				v1 := r.Group("/api/v1/app/sellers/me")
				v1.Use(fakeSellerAuth())
				v1.GET("/reviews", h.GetSellerOrderReviews)

				url := "/api/v1/app/sellers/me/reviews"
				if tt.query != "" {
					url += "?" + tt.query
				}
				req := httptest.NewRequest(http.MethodGet, url, nil)
				req.Header.Set("X-Account-Id", "99")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assertHTTPStatus(t, w, http.StatusOK)
				if captured == nil {
					t.Fatal("GetSellerOrderReviews (app) was not called")
				}
				if captured.Page != tt.wantPage || captured.PageSize != tt.wantPageSize {
					t.Errorf("Page/PageSize = %d/%d, want %d/%d", captured.Page, captured.PageSize, tt.wantPage, tt.wantPageSize)
				}
			})
		}
	})
}

// TestMaskBuyerDisplayName 釘住公開評論列表的三種狀態各自的顯示字串。
//
// 背景（#2830，2026-08-29）：曾寫成 maskUsername(buyerDisplayName(x))，
// 於是墓碑字串被遮成「已刪***」、gateway 自己的 fallback 被遮成「匿名**」。
// 遮罩是為了保護真人的暱稱；墓碑與 fallback 都是我方常數，遮它們不保護任何人，
// 只讓同一個買家在三個評論畫面出現三種寫法。
//
// 鑑別力：把 deleted 分支拿掉 → 第一個 case 轉紅；
// 把 name == "" 分支拿掉 → 第二個 case 轉紅；
// 把 maskUsername 拿掉 → 第三、四個 case 轉紅。
func TestMaskBuyerDisplayName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		deleted bool
		want    string
	}{
		{"已註銷買家：墓碑原樣顯示，不遮罩", "已刪除帳號", true, "已刪除帳號"},
		{"解析失敗：回 fallback，不遮罩", "", false, "匿名買家"},
		{"一般買家：真名照遮", "王小明", false, "王**"},
		{"英文名：真名照遮", "alexis", false, "al****"},
		{"未設暱稱的在籍買家：空字串走 fallback", "", false, "匿名買家"},
		{"上游降級同時標了 deleted：仍走 fallback 不假裝知道", "", true, "匿名買家"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := maskBuyerDisplayName(c.input, c.deleted); got != c.want {
				t.Errorf("maskBuyerDisplayName(%q, %v) = %q, want %q", c.input, c.deleted, got, c.want)
			}
		})
	}
}
