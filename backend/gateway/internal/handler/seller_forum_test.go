package handler

import (
	"context"
	"net/http"
	"testing"

	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func setupSellerForumRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	app := v1.Group("/app")
	app.Use(fakeJWTAuth())
	{
		app.POST("/sellers/:id/forum", h.CreateForumComment)
		app.POST("/sellers/forum/:id/reply", h.ReplyForumComment)
		app.PUT("/sellers/forum/:id", h.UpdateForumComment)
		app.DELETE("/sellers/forum/:id", h.DeleteForumComment)
	}
	pub := v1.Group("/public")
	pub.Use(fakeOptionalAuth())
	{
		pub.GET("/sellers/:id/forum", h.GetSellerForum)
		pub.GET("/sellers/forum/:id/replies", h.GetForumReplies)
	}
	return r
}

type forumCase struct {
	name   string
	method string
	path   string
	body   string
	build  func() (mc *mockSocialServiceClient, get func() (buyerID, storeID int64, ok bool))
}

func forumCases() []forumCase {
	const sellerPath = "/api/v1/app/sellers/857045288972403572/forum"
	return []forumCase{
		{"CreateForumComment", "POST", sellerPath, `{"content":"hi"}`, func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.CreateForumCommentRequest
			return &mockSocialServiceClient{CreateForumCommentFn: func(_ context.Context, in *socialpb.CreateForumCommentRequest, _ ...grpc.CallOption) (*socialpb.ForumCommentResponse, error) {
				got = in
				return &socialpb.ForumCommentResponse{Data: &socialpb.ForumEntryInfo{Id: "1"}}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"ReplyForumComment", "POST", "/api/v1/app/sellers/forum/1/reply", `{"content":"re"}`, func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.ReplyForumCommentRequest
			return &mockSocialServiceClient{ReplyForumCommentFn: func(_ context.Context, in *socialpb.ReplyForumCommentRequest, _ ...grpc.CallOption) (*socialpb.ForumCommentResponse, error) {
				got = in
				return &socialpb.ForumCommentResponse{Data: &socialpb.ForumEntryInfo{Id: "2"}}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"UpdateForumComment", "PUT", "/api/v1/app/sellers/forum/1", `{"content":"edit"}`, func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.UpdateForumCommentRequest
			return &mockSocialServiceClient{UpdateForumCommentFn: func(_ context.Context, in *socialpb.UpdateForumCommentRequest, _ ...grpc.CallOption) (*socialpb.ForumCommentResponse, error) {
				got = in
				return &socialpb.ForumCommentResponse{Data: &socialpb.ForumEntryInfo{Id: "1"}}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"DeleteForumComment", "DELETE", "/api/v1/app/sellers/forum/1", "", func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.DeleteForumCommentRequest
			return &mockSocialServiceClient{DeleteForumCommentFn: func(_ context.Context, in *socialpb.DeleteForumCommentRequest, _ ...grpc.CallOption) (*socialpb.ForumOperationResponse, error) {
				got = in
				return &socialpb.ForumOperationResponse{}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"GetSellerForum", "GET", "/api/v1/public/sellers/857045288972403572/forum", "", func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.GetSellerForumRequest
			return &mockSocialServiceClient{GetSellerForumFn: func(_ context.Context, in *socialpb.GetSellerForumRequest, _ ...grpc.CallOption) (*socialpb.ForumListResponse, error) {
				got = in
				return &socialpb.ForumListResponse{}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"GetForumReplies", "GET", "/api/v1/public/sellers/forum/1/replies", "", func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.GetForumRepliesRequest
			return &mockSocialServiceClient{GetForumRepliesFn: func(_ context.Context, in *socialpb.GetForumRepliesRequest, _ ...grpc.CallOption) (*socialpb.ForumListResponse, error) {
				got = in
				return &socialpb.ForumListResponse{}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
	}
}

func forumHolder(mc *mockSocialServiceClient) *Holder {
	h := newPostHolder(mc)
	h.BuyerID = buyerResolverForTest(socialTestBuyerID)
	h.Store = storeResolverForTest(socialTestStoreID)
	return h
}

func TestForumPaths_ForwardBuyerAndStoreIDsRegardlessOfLoginRole(t *testing.T) {
	for _, tc := range forumCases() {
		t.Run(tc.name, func(t *testing.T) {
			for _, loginRole := range []string{"", "1", "2"} {
				mc, get := tc.build()
				r := setupSellerForumRouter(forumHolder(mc))
				w := doSocialReq(r, tc.method, tc.path, "42", "3", loginRole, tc.body)
				assertHTTPStatus(t, w, http.StatusOK)
				buyerID, storeID, ok := get()
				if !ok || buyerID != socialTestBuyerID || storeID != socialTestStoreID {
					t.Errorf("loginRole=%q forwarded (buyer_id, store_id) = (%d, %d) ok=%v, want (%d, %d)", loginRole, buyerID, storeID, ok, socialTestBuyerID, socialTestStoreID)
				}
			}
		})
	}
}

func TestForumPaths_NoAccountFallback(t *testing.T) {
	for _, tc := range forumCases() {
		t.Run(tc.name, func(t *testing.T) {
			mc, get := tc.build()
			h := forumHolder(mc)
			h.BuyerID = noBuyerResolverForTest()
			h.Store = noStoreResolverForTest()
			w := doSocialReq(setupSellerForumRouter(h), tc.method, tc.path, "42", "", "", tc.body)
			assertHTTPStatus(t, w, http.StatusOK)
			if buyerID, storeID, ok := get(); !ok || buyerID != 0 || storeID != 0 {
				t.Errorf("forwarded (buyer_id, store_id) = (%d, %d) ok=%v, want (0, 0); raw account_id 42 must not leak", buyerID, storeID, ok)
			}
		})
	}
}

func TestForumReads_AnonymousForwardsZeroIDsWithoutResolving(t *testing.T) {
	for _, tc := range forumCases()[4:] {
		t.Run(tc.name, func(t *testing.T) {
			mc, get := tc.build()
			h := forumHolder(mc)
			h.BuyerID = mustNotResolveBuyer(t)
			h.Store = mustNotResolveStore(t)
			w := doSocialReq(setupSellerForumRouter(h), tc.method, tc.path, "", "", "", "")
			assertHTTPStatus(t, w, http.StatusOK)
			if buyerID, storeID, ok := get(); !ok || buyerID != 0 || storeID != 0 {
				t.Errorf("anonymous forwarded (%d, %d) ok=%v, want (0, 0)", buyerID, storeID, ok)
			}
		})
	}
}

func TestForumJSON_CarriesIsMineAndNoUserID(t *testing.T) {
	entry := func(id string, mine bool) *socialpb.ForumEntryInfo {
		return &socialpb.ForumEntryInfo{Id: id, SellerId: "5", UserName: "n", Content: "c", ActorKind: 2, IsMine: mine}
	}
	mc := &mockSocialServiceClient{
		GetSellerForumFn: func(_ context.Context, _ *socialpb.GetSellerForumRequest, _ ...grpc.CallOption) (*socialpb.ForumListResponse, error) {
			return &socialpb.ForumListResponse{Data: []*socialpb.ForumEntryInfo{entry("1", true), entry("2", false)}, Total: 2, Page: 1, PageSize: 10}, nil
		},
		CreateForumCommentFn: func(_ context.Context, _ *socialpb.CreateForumCommentRequest, _ ...grpc.CallOption) (*socialpb.ForumCommentResponse, error) {
			return &socialpb.ForumCommentResponse{Data: entry("3", true)}, nil
		},
	}
	r := setupSellerForumRouter(forumHolder(mc))

	list := doSocialReq(r, "GET", "/api/v1/public/sellers/5/forum", "42", "3", "2", "")
	assertHTTPStatus(t, list, http.StatusOK)
	records := parseJSON(t, list.Body)["data"].(map[string]interface{})["records"].([]interface{})
	for i, want := range []bool{true, false} {
		rec := records[i].(map[string]interface{})
		if rec["isMine"] != want {
			t.Errorf("record %d isMine = %v, want %v", i, rec["isMine"], want)
		}
		if _, has := rec["userId"]; has {
			t.Errorf("record %d must not carry userId", i)
		}
	}
	created := doSocialReq(r, "POST", "/api/v1/app/sellers/5/forum", "42", "3", "2", `{"content":"x"}`)
	assertHTTPStatus(t, created, http.StatusOK)
	data := parseJSON(t, created.Body)["data"].(map[string]interface{})
	if _, has := data["userId"]; has || data["isMine"] != true {
		t.Errorf("created forum entry JSON = %v, want isMine=true and no userId", data)
	}
}
