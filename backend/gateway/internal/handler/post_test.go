package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// setupPostRouter mirrors registerPostRoutes' public/app groupings so the
// optional-auth (personalization) and required-auth (buyer/seller-shared)
// handlers see the same middleware wiring as production.
func setupPostRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")

	pub := v1.Group("/public")
	pub.Use(fakeOptionalAuth())
	{
		pub.GET("/sellers/:id/posts", h.GetSellerPosts)
		posts := pub.Group("/posts")
		{
			posts.GET("", h.GetLatestPosts)
			posts.GET("/:id", h.GetPostDetail)
			posts.GET("/:id/comments", h.GetPostComments)
			posts.GET("/comments/:id/replies", h.GetCommentReplies)
		}
	}

	app := v1.Group("/app")
	app.Use(fakeJWTAuth())
	{
		appPosts := app.Group("/posts")
		{
			appPosts.POST("/:id/like", h.TogglePostLike)
			appPosts.POST("/:id/share", h.SharePost)
			appPosts.POST("/:id/comments", h.CreatePostComment)
			appPosts.POST("/comments/:id/replies", h.ReplyPostComment)
			appPosts.PUT("/comments/:id", h.UpdatePostComment)
			appPosts.DELETE("/comments/:id", h.DeletePostComment)
			appPosts.POST("/comments/:id/like", h.ToggleCommentLike)
		}
		app.POST("/sellers/posts/:id/reports", h.ReportPost)
		// 賣家帖文 CRUD（router package 用 sellerHandle + authz.Cap* 包一層 capability
		// check — 那層在 router package 測試，這裡跟 ReportPost 同慣例，只掛裸路由
		// 驗證 handler 自己的 store_id 解析行為）。
		app.POST("/sellers/posts", h.CreatePost)
		app.GET("/sellers/posts/:id", h.GetMyPostDetail)
		app.PUT("/sellers/posts/:id", h.UpdatePost)
		app.DELETE("/sellers/posts/:id", h.DeletePost)
		app.GET("/sellers/me/posts", h.ListMyPosts)
	}
	return r
}

func newPostHolder(mc *mockSocialServiceClient) *Holder {
	return &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		BuyerID: buyerGrandfatherResolverForTest(),
		Store:   noStoreResolverForTest(),
	}
}

// sellerClientResolvingStoreIDForTest wires a Seller client whose
// GetSellerByStoreID (used by requireStore) always
// resolves to storeID, regardless of the id it was called with — enough to
// exercise handlers that resolve a storefront path id (store_id or
// store_code) before querying a downstream service keyed by store_id.
func sellerClientResolvingStoreIDForTest(storeID string) *grpcclient.SellerClient {
	return grpcclient.NewSellerClientForTest(&mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{StoreId: storeID}}, nil
		},
	})
}

// mustNotResolveBuyer returns a resolver that fails the test if consulted —
// proves an anonymous (accountID==0) call never attempts buyer resolution
// (resolveOptionalBuyerID short-circuits before touching the resolver).
func mustNotResolveBuyer(t *testing.T) *buyerid.Resolver {
	t.Helper()
	return buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
		t.Fatal("buyer resolver must not be called for anonymous request (accountID==0)")
		return nil, nil
	}))
}

func mustNotResolveBuyerSeller(t *testing.T) *buyerid.Resolver {
	t.Helper()
	return buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
		t.Fatal("buyer resolver must not be called for loginRole=2 (seller)")
		return nil, nil
	}))
}

func doPostReq(r *gin.Engine, method, path, accountID, loginRole string, body interface{}) *httptest.ResponseRecorder {
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	if accountID != "" {
		req.Header.Set("X-Account-Id", accountID)
	}
	if loginRole != "" {
		req.Header.Set("X-Login-Role", loginRole)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ==================== GetLatestPosts (optional-auth personalization) ====================

func TestGetLatestPosts_Anonymous_NoPersonalization(t *testing.T) {
	var captured *socialpb.GetPostListRequest
	mc := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			captured = in
			return &socialpb.PostListResponse{Data: []*socialpb.PostListItem{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = mustNotResolveBuyer(t)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts", "", "", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 0 {
		t.Errorf("captured.BuyerId = %v, want 0 (anonymous)", captured)
	}
}

func TestGetLatestPosts_BuyerRoleResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.GetPostListRequest
	mc := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			captured = in
			return &socialpb.PostListResponse{Data: []*socialpb.PostListItem{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("captured.BuyerId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

// ==================== seller_logo_url (issue #1782 / B-238) ====================

// TestGetLatestPosts_SellerLogoURLField proves the public posts list exposes
// canonical camelCase logoKey/logoUrl (49 §6.1 / issue #1782).
func TestGetLatestPosts_SellerLogoURLField(t *testing.T) {
	mc := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			return &socialpb.PostListResponse{
				Data: []*socialpb.PostListItem{
					{Id: "1", SellerId: "42", SellerName: "老王鮮魚", SellerLogoKey: "public/seller/42/logo.png", Content: "test"},
				},
				Total:    1,
				Page:     1,
				PageSize: 20,
			}, nil
		},
	}
	// Provide a mock seller client that returns empty (fails gracefully) to skip enrichment.
	// The proto fields (seller_logo_key, seller_name) are already populated.
	sm := &mockSellerServiceClient{
		GetSellerBatchFn: func(ctx context.Context, in *sellerpb.GetSellerBatchRequest, opts ...grpc.CallOption) (*sellerpb.SellerBatchResponse, error) {
			return &sellerpb.SellerBatchResponse{Code: 0, Data: []*sellerpb.SellerInfo{}}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = mustNotResolveBuyer(t)
	h.Seller = grpcclient.NewSellerClientForTest(sm)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts", "", "", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	records := data["records"].([]interface{})
	first := records[0].(map[string]interface{})
	if got := first["logoKey"]; got != "public/seller/42/logo.png" {
		t.Errorf("logoKey = %v, want %q", got, "public/seller/42/logo.png")
	}
	logoURL := first["logoUrl"]
	if logoURL == nil || logoURL == "" {
		t.Errorf("logoUrl = %v, want a non-empty built URL", logoURL)
	}
}

// ==================== store_code (#2098 階段三) ====================

// TestGetLatestPosts_CarriesStoreCode proves the public posts list carries the
// seller's canonical store_code, so a feed card can link /sellers/{store_code}
// directly instead of linking the store_id and eating a 308 per tap.
//
// social-service only stores seller_id; the segment rides in on the
// GetSellerBatch response that enrichPostsWithSellerInfo already makes for the
// store name and logo — so this costs no extra RPC.
func TestGetLatestPosts_CarriesStoreCode(t *testing.T) {
	mc := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			return &socialpb.PostListResponse{
				Data: []*socialpb.PostListItem{
					{Id: "1", SellerId: "42", SellerName: "社群舊名", Content: "test"},
					// 查無此賣家（已關店 / batch 跳過）→ store_code 必須留空，
					// 前端據此不渲染連結；退回 seller_id 會在階段三收斂後變成 404。
					{Id: "2", SellerId: "99", SellerName: "查無此人", Content: "test"},
				},
				Total: 2, Page: 1, PageSize: 20,
			}, nil
		},
	}
	sm := &mockSellerServiceClient{
		GetSellerBatchFn: func(ctx context.Context, in *sellerpb.GetSellerBatchRequest, opts ...grpc.CallOption) (*sellerpb.SellerBatchResponse, error) {
			return &sellerpb.SellerBatchResponse{Code: 0, Data: []*sellerpb.SellerInfo{
				{AccountId: 42, StoreId: "42", StoreCode: "2649605557", Name: "老王鮮魚"},
			}}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = mustNotResolveBuyer(t)
	h.Seller = grpcclient.NewSellerClientForTest(sm)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts", "", "", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	records := parseJSON(t, w.Body)["data"].(map[string]interface{})["records"].([]interface{})

	first := records[0].(map[string]interface{})
	if got := first["storeCode"]; got != "2649605557" {
		t.Errorf("records[0].store_code = %v, want %q", got, "2649605557")
	}
	// seller_id 仍然要在——它是 is_liked / 追蹤等以 store_id 為鍵的呼叫用的。
	if got := first["sellerId"]; got != "42" {
		t.Errorf("records[0].seller_id = %v, want %q", got, "42")
	}

	second := records[1].(map[string]interface{})
	if got, ok := second["storeCode"]; !ok || got != "" {
		t.Errorf("records[1].store_code = %v, want empty (seller not resolvable)", got)
	}
}

// ==================== GetSellerPosts (Deviation 2: optional-auth personalization) ====================

func TestGetSellerPosts_Anonymous_NoPersonalization(t *testing.T) {
	var captured *socialpb.GetPostListRequest
	mc := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			captured = in
			return &socialpb.PostListResponse{Data: []*socialpb.PostListItem{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = mustNotResolveBuyer(t)
	h.Seller = sellerClientResolvingStoreIDForTest("857045288972403572")
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/sellers/857045288972403572/posts", "", "", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 0 {
		t.Errorf("captured.BuyerId = %v, want 0 (anonymous)", captured)
	}
}

func TestGetSellerPosts_BuyerRoleResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.GetPostListRequest
	mc := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			captured = in
			return &socialpb.PostListResponse{Data: []*socialpb.PostListItem{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	h.Seller = sellerClientResolvingStoreIDForTest("857045288972403572")
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/sellers/857045288972403572/posts", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("captured.BuyerId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

// ==================== GetPostDetail (Deviation 2: REST single-post is_liked) ====================

func TestGetPostDetail_Anonymous_NoPersonalization(t *testing.T) {
	var captured *socialpb.GetPostDetailRequest
	mc := &mockSocialServiceClient{
		GetPostDetailFn: func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
			captured = in
			return &socialpb.PostDetailResponse{Code: 0, Data: &socialpb.PostDetailInfo{Id: "1"}}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = mustNotResolveBuyer(t)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts/1", "", "", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 0 {
		t.Errorf("captured.BuyerId = %v, want 0 (anonymous)", captured)
	}
}

func TestGetPostDetail_BuyerRoleResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.GetPostDetailRequest
	mc := &mockSocialServiceClient{
		GetPostDetailFn: func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
			captured = in
			return &socialpb.PostDetailResponse{Code: 0, Data: &socialpb.PostDetailInfo{Id: "1"}}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts/1", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("captured.BuyerId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

// ==================== GetPostComments (Deviation 2: comment-list is_liked) ====================

func TestGetPostComments_Anonymous_NoPersonalization(t *testing.T) {
	var captured *socialpb.GetCommentListRequest
	mc := &mockSocialServiceClient{
		GetCommentListFn: func(ctx context.Context, in *socialpb.GetCommentListRequest, opts ...grpc.CallOption) (*socialpb.CommentListResponse, error) {
			captured = in
			return &socialpb.CommentListResponse{Code: 0, Data: []*socialpb.CommentInfo{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = mustNotResolveBuyer(t)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts/1/comments", "", "", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 0 {
		t.Errorf("captured.BuyerId = %v, want 0 (anonymous)", captured)
	}
}

func TestGetPostComments_BuyerRoleResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.GetCommentListRequest
	mc := &mockSocialServiceClient{
		GetCommentListFn: func(ctx context.Context, in *socialpb.GetCommentListRequest, opts ...grpc.CallOption) (*socialpb.CommentListResponse, error) {
			captured = in
			return &socialpb.CommentListResponse{Code: 0, Data: []*socialpb.CommentInfo{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts/1/comments", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("captured.BuyerId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

// ==================== GetCommentReplies (Deviation 2: reply-list is_liked) ====================

func TestGetCommentReplies_Anonymous_NoPersonalization(t *testing.T) {
	var captured *socialpb.GetCommentListRequest
	mc := &mockSocialServiceClient{
		GetCommentListFn: func(ctx context.Context, in *socialpb.GetCommentListRequest, opts ...grpc.CallOption) (*socialpb.CommentListResponse, error) {
			captured = in
			return &socialpb.CommentListResponse{Code: 0, Data: []*socialpb.CommentInfo{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = mustNotResolveBuyer(t)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts/comments/1/replies", "", "", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 0 {
		t.Errorf("captured.BuyerId = %v, want 0 (anonymous)", captured)
	}
}

func TestGetCommentReplies_BuyerRoleResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.GetCommentListRequest
	mc := &mockSocialServiceClient{
		GetCommentListFn: func(ctx context.Context, in *socialpb.GetCommentListRequest, opts ...grpc.CallOption) (*socialpb.CommentListResponse, error) {
			captured = in
			return &socialpb.CommentListResponse{Code: 0, Data: []*socialpb.CommentInfo{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupPostRouter(h)
	w := doPostReq(r, "GET", "/api/v1/public/posts/comments/1/replies", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("captured.BuyerId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

// ==================== SharePost ====================
//
// 2026-08-01 對抗式 review Bug 2: SharePost's userID is never read server-side
// (social-service's SharePost(ctx, postID) has no identity parameter at all —
// only share_count+1). It must therefore never resolve identity at all —
// neither axis, regardless of role — so a caller with no current buyer/
// seller stint is never blocked from sharing a post over something the
// action doesn't even use.

func TestSharePost_NeverResolvesIdentity(t *testing.T) {
	var captured *socialpb.SharePostRequest
	mc := &mockSocialServiceClient{
		SharePostFn: func(ctx context.Context, in *socialpb.SharePostRequest, opts ...grpc.CallOption) (*socialpb.SharePostResponse, error) {
			captured = in
			return &socialpb.SharePostResponse{ShareCount: 1}, nil
		},
	}
	h := &Holder{
		Social: grpcclient.NewSocialClientForTest(mc),
		BuyerID: buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
			t.Fatal("buyer resolver must not be called — SharePost's userID is never used server-side")
			return nil, nil
		})),
		Store: nil, // touching this would nil-panic — proves the seller axis is never consulted either
	}
	r := setupPostRouter(h)
	w := doPostReq(r, "POST", "/api/v1/app/posts/1/share", "42", "1", map[string]string{"platform": "line"})
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.UserId != 42 {
		t.Errorf("captured.UserId = %v, want 42 (raw account_id — SharePost never resolves identity)", captured)
	}
}

// TestSharePost_SellerRoleWithNoCurrentStoreStillSucceeds is the regression
// guard for 2026-08-01 對抗式 review Bug 2: before the fix, a seller-role
// caller with no current store got 403'd sharing a post — an action that
// doesn't even use the resolved identity. Store is wired to a resolver
// that WOULD return ErrNoCurrentStore if ever called, so this test fails
// loudly (403, not 200) if identity resolution is ever reintroduced here.
func TestSharePost_SellerRoleWithNoCurrentStoreStillSucceeds(t *testing.T) {
	var captured *socialpb.SharePostRequest
	mc := &mockSocialServiceClient{
		SharePostFn: func(ctx context.Context, in *socialpb.SharePostRequest, opts ...grpc.CallOption) (*socialpb.SharePostResponse, error) {
			captured = in
			return &socialpb.SharePostResponse{ShareCount: 1}, nil
		},
	}
	h := &Holder{
		Social: grpcclient.NewSocialClientForTest(mc),
		Store:  noStoreResolverForTest(),
	}
	r := setupPostRouter(h)
	req := httptest.NewRequest("POST", "/api/v1/app/posts/1/share", bytes.NewBufferString(`{"platform":"line"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.UserId != 42 {
		t.Errorf("captured.UserId = %v, want 42 (raw account_id; no current store must not block sharing)", captured)
	}
}

// ==================== ReportPost ====================
//
// ReportPost writes reports.reporter_id — the same column as moderation.go's
// SubmitReport (/app/reports) — so both routes must share one fault-tolerance
// policy: resolveModerationUserID (fallback, not a hard 403, when the current
// buyer/store cannot be resolved). ReportPost calls it directly, same as
// SubmitReport, and passes the resolved id as ReporterId.

func TestReportPost_BuyerRoleResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.ReportPostRequest
	mc := &mockSocialServiceClient{
		ReportPostFn: func(ctx context.Context, in *socialpb.ReportPostRequest, opts ...grpc.CallOption) (*socialpb.ReportPostResponse, error) {
			captured = in
			return &socialpb.ReportPostResponse{ReportId: "1"}, nil
		},
	}
	h := newPostHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupPostRouter(h)
	w := doPostReq(r, "POST", "/api/v1/app/sellers/posts/1/reports", "42", "1", map[string]string{"reasonCode": "SPAM"})
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ReporterId != 9999 {
		t.Errorf("captured.ReporterId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

func TestReportPost_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.ReportPostRequest
	mc := &mockSocialServiceClient{
		ReportPostFn: func(ctx context.Context, in *socialpb.ReportPostRequest, opts ...grpc.CallOption) (*socialpb.ReportPostResponse, error) {
			captured = in
			return &socialpb.ReportPostResponse{ReportId: "1"}, nil
		},
	}
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		Store:   storeResolverForTest(8001),
		BuyerID: mustNotResolveBuyerSeller(t),
	}
	r := setupPostRouter(h)
	req := httptest.NewRequest("POST", "/api/v1/app/sellers/posts/1/reports", bytes.NewBufferString(`{"reasonCode":"SPAM"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ReporterId != 8001 {
		t.Errorf("captured.ReporterId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

// reportPostAsSeller 打 ReportPost（resolveModerationUserID 在 moderation.go 之外
// 的唯一呼叫端），loginRole=2。
func reportPostAsSeller(t *testing.T, h *Holder) *httptest.ResponseRecorder {
	t.Helper()
	r := setupPostRouter(h)
	req := httptest.NewRequest("POST", "/api/v1/app/sellers/posts/1/reports", bytes.NewBufferString(`{"reasonCode":"SPAM"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func reportPostCaptor() (*mockSocialServiceClient, func() *socialpb.ReportPostRequest) {
	var captured *socialpb.ReportPostRequest
	mc := &mockSocialServiceClient{
		ReportPostFn: func(ctx context.Context, in *socialpb.ReportPostRequest, opts ...grpc.CallOption) (*socialpb.ReportPostResponse, error) {
			captured = in
			return &socialpb.ReportPostResponse{ReportId: "1"}, nil
		},
	}
	return mc, func() *socialpb.ReportPostRequest { return captured }
}

// 賣家軸查無現任 store 時改問買家軸（#2867）：raw account_id 在 stint re-key 之後
// 兩軸都查不到，而 social-service 自 #2789 起對這種 id 是 fail-closed 的。
// 詳細不變式說明見 moderation_test.go 的 TestBlockUser_NoCurrentBuyer* 同族。
func TestReportPost_NoCurrentSellerResolvesCurrentBuyerID(t *testing.T) {
	mc, captured := reportPostCaptor()
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		Store:   noStoreResolverForTest(),
		BuyerID: buyerResolverForTest(9001),
	}
	w := reportPostAsSeller(t, h)
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.ReporterId != 9001 {
		t.Errorf("captured.ReporterId = %v, want 9001 (current buyer_id, not raw account_id 42)", got)
	}
}

func TestReportPost_NoStintOnEitherAxisFallsBackToRaw(t *testing.T) {
	mc, captured := reportPostCaptor()
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		Store:   noStoreResolverForTest(),
		BuyerID: noBuyerResolverForTest(),
	}
	w := reportPostAsSeller(t, h)
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.ReporterId != 42 {
		t.Errorf("captured.ReporterId = %v, want 42 (no live stint on either axis: raw account_id, still NOT 403)", got)
	}
}

// TestReportPost_SellerServiceErrorPropagates：真正的後端錯誤（非
// ErrNoCurrentStore）仍照既有錯誤回應映射（503），不被 fallback 吞掉。
func TestReportPost_SellerServiceErrorPropagates(t *testing.T) {
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(&mockSocialServiceClient{}),
		Store:   storeServiceDownResolverForTest(),
		BuyerID: mustNotResolveBuyerSeller(t),
	}
	r := setupPostRouter(h)
	req := httptest.NewRequest("POST", "/api/v1/app/sellers/posts/1/reports", bytes.NewBufferString(`{"reasonCode":"SPAM"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// ==================== CreatePost / UpdatePost / DeletePost / ListMyPosts ====================
type postWriteCaptureClient struct {
	*mockSocialServiceClient
	CreatePostFn  func(ctx context.Context, in *socialpb.CreatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error)
	UpdatePostFn  func(ctx context.Context, in *socialpb.UpdatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error)
	DeletePostFn  func(ctx context.Context, in *socialpb.DeletePostRequest, opts ...grpc.CallOption) (*socialpb.PostOperationResponse, error)
	ListMyPostsFn func(ctx context.Context, in *socialpb.ListMyPostsRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error)
}

func (m *postWriteCaptureClient) CreatePost(ctx context.Context, in *socialpb.CreatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
	if m.CreatePostFn != nil {
		return m.CreatePostFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *postWriteCaptureClient) UpdatePost(ctx context.Context, in *socialpb.UpdatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
	if m.UpdatePostFn != nil {
		return m.UpdatePostFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *postWriteCaptureClient) DeletePost(ctx context.Context, in *socialpb.DeletePostRequest, opts ...grpc.CallOption) (*socialpb.PostOperationResponse, error) {
	if m.DeletePostFn != nil {
		return m.DeletePostFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *postWriteCaptureClient) ListMyPosts(ctx context.Context, in *socialpb.ListMyPostsRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
	if m.ListMyPostsFn != nil {
		return m.ListMyPostsFn(ctx, in, opts...)
	}
	return nil, nil
}

func TestCreatePost_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.CreatePostRequest
	mc := &postWriteCaptureClient{
		mockSocialServiceClient: &mockSocialServiceClient{},
		CreatePostFn: func(ctx context.Context, in *socialpb.CreatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
			captured = in
			return &socialpb.PostResponse{Data: &socialpb.PostDetailInfo{Id: "1"}}, nil
		},
	}
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		Store:   storeResolverForTest(8001),
		BuyerID: mustNotResolveBuyerSeller(t),
	}
	r := setupPostRouter(h)
	req := httptest.NewRequest("POST", "/api/v1/app/sellers/posts", bytes.NewBufferString(`{"content":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.SellerId != 8001 {
		t.Errorf("captured.SellerId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

func TestUpdatePost_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.UpdatePostRequest
	mc := &postWriteCaptureClient{
		mockSocialServiceClient: &mockSocialServiceClient{},
		UpdatePostFn: func(ctx context.Context, in *socialpb.UpdatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
			captured = in
			return &socialpb.PostResponse{Data: &socialpb.PostDetailInfo{Id: "1"}}, nil
		},
	}
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		Store:   storeResolverForTest(8001),
		BuyerID: mustNotResolveBuyerSeller(t),
	}
	r := setupPostRouter(h)
	req := httptest.NewRequest("PUT", "/api/v1/app/sellers/posts/1", bytes.NewBufferString(`{"content":"edited"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.SellerId != 8001 {
		t.Errorf("captured.SellerId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

func TestDeletePost_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.DeletePostRequest
	mc := &postWriteCaptureClient{
		mockSocialServiceClient: &mockSocialServiceClient{},
		DeletePostFn: func(ctx context.Context, in *socialpb.DeletePostRequest, opts ...grpc.CallOption) (*socialpb.PostOperationResponse, error) {
			captured = in
			return &socialpb.PostOperationResponse{}, nil
		},
	}
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		Store:   storeResolverForTest(8001),
		BuyerID: mustNotResolveBuyerSeller(t),
	}
	r := setupPostRouter(h)
	req := httptest.NewRequest("DELETE", "/api/v1/app/sellers/posts/1", nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.SellerId != 8001 {
		t.Errorf("captured.SellerId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

func TestListMyPosts_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.ListMyPostsRequest
	mc := &postWriteCaptureClient{
		mockSocialServiceClient: &mockSocialServiceClient{},
		ListMyPostsFn: func(ctx context.Context, in *socialpb.ListMyPostsRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			captured = in
			return &socialpb.PostListResponse{Data: []*socialpb.PostListItem{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
	}
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		Store:   storeResolverForTest(8001),
		BuyerID: mustNotResolveBuyerSeller(t),
	}
	r := setupPostRouter(h)
	req := httptest.NewRequest("GET", "/api/v1/app/sellers/me/posts", nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.SellerId != 8001 {
		t.Errorf("captured.SellerId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

// ==================== GetMyPostDetail (owner read) ====================

func doOwnerPostDetail(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/v1/app/sellers/posts/77", nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetMyPostDetail_InjectsCurrentStoreIDWithoutDelegation(t *testing.T) {
	var captured *socialpb.GetPostDetailRequest
	mc := &mockSocialServiceClient{
		GetPostDetailFn: func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
			captured = in
			return &socialpb.PostDetailResponse{Code: 0, Data: &socialpb.PostDetailInfo{
				Id: "77", Content: "草稿", PublishStatus: "4401",
				HiddenAt: "2026-10-01T00:00:00Z", HiddenReason: "違規",
			}}, nil
		},
	}
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		Store:   storeResolverForTest(8001),
		BuyerID: mustNotResolveBuyerSeller(t),
	}
	w := doOwnerPostDetail(setupPostRouter(h))
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("social GetPostDetail was not called")
	}
	if captured.OwnerStoreId != 8001 {
		t.Errorf("OwnerStoreId = %d, want 8001 (current store_id, not raw account_id 42)", captured.OwnerStoreId)
	}
	if captured.Delegation != nil {
		t.Errorf("Delegation = %v, want nil: a delegation context would select the delegated branch", captured.Delegation)
	}
	if captured.PostId != "77" || captured.BuyerId != 0 {
		t.Errorf("PostId=%q UserId=%d, want \"77\" and 0", captured.PostId, captured.BuyerId)
	}
	var body struct {
		Data struct {
			ID            string `json:"id"`
			Content       string `json:"content"`
			PublishStatus string `json:"publishStatus"`
			HiddenAt      int64  `json:"hiddenAt"`
			HiddenReason  string `json:"hiddenReason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.ID != "77" || body.Data.Content != "草稿" {
		t.Errorf("data = %+v, want the post detail passed through", body.Data)
	}
	if body.Data.PublishStatus != "4401" || body.Data.HiddenAt != 1790812800 || body.Data.HiddenReason != "違規" {
		t.Errorf("data = %+v, want publishStatus 4401 and hiddenAt 1790812800 (2026-10-01T00:00:00Z) with the reason", body.Data)
	}
}

func TestGetMyPostDetail_NotOwnedIsNotFoundEnvelope(t *testing.T) {
	mc := &mockSocialServiceClient{
		GetPostDetailFn: func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
			return &socialpb.PostDetailResponse{Code: 404, Message: "not found"}, nil
		},
	}
	h := &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		Store:   storeResolverForTest(8001),
		BuyerID: mustNotResolveBuyerSeller(t),
	}
	w := doOwnerPostDetail(setupPostRouter(h))
	// Same envelope as the public detail route: the upstream business code is
	// carried in the body, with no post data.
	var body struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != 404 || len(body.Data) != 0 && string(body.Data) != "null" {
		t.Errorf("body = %s, want code 404 and no data", w.Body.String())
	}
}
