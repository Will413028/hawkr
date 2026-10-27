package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

const (
	socialTestBuyerID = 9999
	socialTestStoreID = 8001
)

// socialIdentityHolder 的買家與店家 id 刻意相同數字之外的值，讓「用錯軸」的斷言不會因巧合通過。
func socialIdentityHolder(mc *mockSocialServiceClient) *Holder {
	h := newPostHolder(mc)
	h.BuyerID = buyerResolverForTest(socialTestBuyerID)
	h.Store = storeResolverForTest(socialTestStoreID)
	return h
}

func doSocialReq(r http.Handler, method, path, accountID, accountType, loginRole, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if accountID != "" {
		req.Header.Set("X-Account-Id", accountID)
	}
	if accountType != "" {
		req.Header.Set("X-Account-Type", accountType)
	}
	if loginRole != "" {
		req.Header.Set("X-Login-Role", loginRole)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// socialWriteCase 是一條 social 寫入或個人化讀取路徑：capture 回傳 gRPC 收到的 request 中的
// (buyer_id, store_id)。
type socialWriteCase struct {
	name   string
	method string
	path   string
	body   string
	// build 建出會記錄 request 的 mock；get 回傳記錄到的 (buyer_id, store_id)。
	build func() (mc *mockSocialServiceClient, get func() (buyerID, storeID int64, ok bool))
}

func socialWriteCases() []socialWriteCase {
	return []socialWriteCase{
		{"TogglePostLike", "POST", "/api/v1/app/posts/1/like", "", func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.TogglePostLikeRequest
			return &mockSocialServiceClient{TogglePostLikeFn: func(_ context.Context, in *socialpb.TogglePostLikeRequest, _ ...grpc.CallOption) (*socialpb.TogglePostLikeResponse, error) {
				got = in
				return &socialpb.TogglePostLikeResponse{Liked: true}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"CreateComment", "POST", "/api/v1/app/posts/1/comments", `{"content":"nice"}`, func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.CreateCommentRequest
			return &mockSocialServiceClient{CreateCommentFn: func(_ context.Context, in *socialpb.CreateCommentRequest, _ ...grpc.CallOption) (*socialpb.CommentResponse, error) {
				got = in
				return &socialpb.CommentResponse{Data: &socialpb.CommentInfo{Id: "1"}}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"ReplyComment", "POST", "/api/v1/app/posts/comments/1/replies", `{"postId":"1","content":"reply"}`, func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.CreateCommentRequest
			return &mockSocialServiceClient{CreateCommentFn: func(_ context.Context, in *socialpb.CreateCommentRequest, _ ...grpc.CallOption) (*socialpb.CommentResponse, error) {
				got = in
				return &socialpb.CommentResponse{Data: &socialpb.CommentInfo{Id: "2"}}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"UpdateComment", "PUT", "/api/v1/app/posts/comments/1", `{"content":"edited"}`, func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.UpdateCommentRequest
			return &mockSocialServiceClient{UpdateCommentFn: func(_ context.Context, in *socialpb.UpdateCommentRequest, _ ...grpc.CallOption) (*socialpb.CommentResponse, error) {
				got = in
				return &socialpb.CommentResponse{Data: &socialpb.CommentInfo{Id: "1"}}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"DeleteComment", "DELETE", "/api/v1/app/posts/comments/1", "", func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.DeleteCommentRequest
			return &mockSocialServiceClient{DeleteCommentFn: func(_ context.Context, in *socialpb.DeleteCommentRequest, _ ...grpc.CallOption) (*socialpb.CommentOperationResponse, error) {
				got = in
				return &socialpb.CommentOperationResponse{}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"ToggleCommentLike", "POST", "/api/v1/app/posts/comments/1/like", "", func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.ToggleCommentLikeRequest
			return &mockSocialServiceClient{ToggleCommentLikeFn: func(_ context.Context, in *socialpb.ToggleCommentLikeRequest, _ ...grpc.CallOption) (*socialpb.ToggleCommentLikeResponse, error) {
				got = in
				return &socialpb.ToggleCommentLikeResponse{Liked: true}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"GetPostComments", "GET", "/api/v1/public/posts/1/comments", "", func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.GetCommentListRequest
			return &mockSocialServiceClient{GetCommentListFn: func(_ context.Context, in *socialpb.GetCommentListRequest, _ ...grpc.CallOption) (*socialpb.CommentListResponse, error) {
				got = in
				return &socialpb.CommentListResponse{}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
		{"GetCommentReplies", "GET", "/api/v1/public/posts/comments/1/replies", "", func() (*mockSocialServiceClient, func() (int64, int64, bool)) {
			var got *socialpb.GetCommentListRequest
			return &mockSocialServiceClient{GetCommentListFn: func(_ context.Context, in *socialpb.GetCommentListRequest, _ ...grpc.CallOption) (*socialpb.CommentListResponse, error) {
				got = in
				return &socialpb.CommentListResponse{}, nil
			}}, func() (int64, int64, bool) {
				if got == nil {
					return 0, 0, false
				}
				return got.BuyerId, got.StoreId, true
			}
		}},
	}
}

// 身分由 social 依目標歸屬推導：gateway 轉送的兩個 id 與 X-Login-Role 無關。
func TestSocialPaths_ForwardBuyerAndStoreIDsRegardlessOfLoginRole(t *testing.T) {
	for _, tc := range socialWriteCases() {
		t.Run(tc.name, func(t *testing.T) {
			forwarded := make([][2]int64, 0, 3)
			for _, loginRole := range []string{"", "1", "2"} {
				mc, get := tc.build()
				r := setupPostRouter(socialIdentityHolder(mc))
				w := doSocialReq(r, tc.method, tc.path, "42", "3", loginRole, tc.body)
				assertHTTPStatus(t, w, http.StatusOK)
				buyerID, storeID, ok := get()
				if !ok {
					t.Fatalf("loginRole=%q: social was not called", loginRole)
				}
				forwarded = append(forwarded, [2]int64{buyerID, storeID})
			}
			for i, f := range forwarded {
				if f != [2]int64{socialTestBuyerID, socialTestStoreID} {
					t.Errorf("loginRole case %d forwarded (buyer_id, store_id) = %v, want (%d, %d)", i, f, socialTestBuyerID, socialTestStoreID)
				}
			}
		})
	}
}

// 沒有買家身分的帳號：不退回 account_id（42），buyer_id 就是 0，由 social 拒絕或略過。
func TestSocialPaths_NoBuyerIdentityIsZeroNotAccountID(t *testing.T) {
	for _, tc := range socialWriteCases() {
		t.Run(tc.name, func(t *testing.T) {
			mc, get := tc.build()
			h := socialIdentityHolder(mc)
			h.BuyerID = noBuyerResolverForTest()
			r := setupPostRouter(h)
			w := doSocialReq(r, tc.method, tc.path, "42", "2", "2", tc.body)
			assertHTTPStatus(t, w, http.StatusOK)
			buyerID, storeID, ok := get()
			if !ok || buyerID != 0 || storeID != socialTestStoreID {
				t.Errorf("forwarded (buyer_id, store_id) = (%d, %d) ok=%v, want (0, %d)", buyerID, storeID, ok, socialTestStoreID)
			}
		})
	}
}

func TestSocialPaths_NoStoreIdentityIsZeroStoreID(t *testing.T) {
	for _, tc := range socialWriteCases() {
		t.Run(tc.name, func(t *testing.T) {
			mc, get := tc.build()
			h := socialIdentityHolder(mc)
			h.Store = noStoreResolverForTest()
			r := setupPostRouter(h)
			w := doSocialReq(r, tc.method, tc.path, "42", "1", "1", tc.body)
			assertHTTPStatus(t, w, http.StatusOK)
			buyerID, storeID, ok := get()
			if !ok || buyerID != socialTestBuyerID || storeID != 0 {
				t.Errorf("forwarded (buyer_id, store_id) = (%d, %d) ok=%v, want (%d, 0)", buyerID, storeID, ok, socialTestBuyerID)
			}
		})
	}
}

func TestSocialPaths_BackendResolverErrorIs503(t *testing.T) {
	for _, tc := range socialWriteCases() {
		t.Run(tc.name+"/buyer", func(t *testing.T) {
			mc, get := tc.build()
			h := socialIdentityHolder(mc)
			h.BuyerID = buyerServiceDownResolverForTest()
			w := doSocialReq(setupPostRouter(h), tc.method, tc.path, "42", "3", "1", tc.body)
			assertHTTPStatus(t, w, http.StatusServiceUnavailable)
			if _, _, called := get(); called {
				t.Error("social must not be called when the buyer axis is unavailable")
			}
		})
		t.Run(tc.name+"/store", func(t *testing.T) {
			mc, get := tc.build()
			h := socialIdentityHolder(mc)
			h.Store = storeServiceDownResolverForTest()
			w := doSocialReq(setupPostRouter(h), tc.method, tc.path, "42", "3", "2", tc.body)
			assertHTTPStatus(t, w, http.StatusServiceUnavailable)
			if _, _, called := get(); called {
				t.Error("social must not be called when the store axis is unavailable")
			}
		})
	}
}

// 賣家工作區（loginRole=2）按貼文讚：貼文讚永遠是本人，帶 buyer_id，成功。
func TestTogglePostLike_SellerWorkspaceLikesAsBuyer(t *testing.T) {
	var got *socialpb.TogglePostLikeRequest
	mc := &mockSocialServiceClient{TogglePostLikeFn: func(_ context.Context, in *socialpb.TogglePostLikeRequest, _ ...grpc.CallOption) (*socialpb.TogglePostLikeResponse, error) {
		got = in
		return &socialpb.TogglePostLikeResponse{Liked: true}, nil
	}}
	w := doSocialReq(setupPostRouter(socialIdentityHolder(mc)), "POST", "/api/v1/app/posts/857045288972403572/like", "42", "3", "2", "")
	assertHTTPStatus(t, w, http.StatusOK)
	data := parseJSON(t, w.Body)["data"].(map[string]interface{})
	if data["liked"] != true {
		t.Errorf("liked = %v, want true", data["liked"])
	}
	want := &socialpb.TogglePostLikeRequest{BuyerId: socialTestBuyerID, PostId: "857045288972403572", StoreId: socialTestStoreID}
	if !proto.Equal(got, want) {
		t.Errorf("request = %v, want %v", got, want)
	}
}

// 貼文列表／詳情的 is_liked 只看 buyer_id（貼文讚永遠是本人），不因 loginRole 改用 store_id。
func TestPostReads_IsLikedUsesBuyerIDInEveryWorkspace(t *testing.T) {
	for _, loginRole := range []string{"", "1", "2"} {
		var listGot *socialpb.GetPostListRequest
		var detailGot *socialpb.GetPostDetailRequest
		mc := &mockSocialServiceClient{
			GetPostListFn: func(_ context.Context, in *socialpb.GetPostListRequest, _ ...grpc.CallOption) (*socialpb.PostListResponse, error) {
				listGot = in
				return &socialpb.PostListResponse{}, nil
			},
			GetPostDetailFn: func(_ context.Context, in *socialpb.GetPostDetailRequest, _ ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
				detailGot = in
				return &socialpb.PostDetailResponse{Data: &socialpb.PostDetailInfo{Id: "1"}}, nil
			},
		}
		h := socialIdentityHolder(mc)
		h.Seller = sellerClientResolvingStoreIDForTest("857045288972403572")
		r := setupPostRouter(h)
		for _, path := range []string{"/api/v1/public/posts", "/api/v1/public/sellers/857045288972403572/posts", "/api/v1/public/posts/1"} {
			assertHTTPStatus(t, doSocialReq(r, "GET", path, "42", "3", loginRole, ""), http.StatusOK)
		}
		if listGot.GetBuyerId() != socialTestBuyerID || detailGot.GetBuyerId() != socialTestBuyerID {
			t.Errorf("loginRole=%q: is_liked viewer = list %d / detail %d, want buyer_id %d", loginRole, listGot.GetBuyerId(), detailGot.GetBuyerId(), socialTestBuyerID)
		}
	}
}

func TestPostComments_JSONCarriesIsMineAndNoUserID(t *testing.T) {
	comment := func(id string, kind int32, mine bool, replies ...*socialpb.CommentInfo) *socialpb.CommentInfo {
		return &socialpb.CommentInfo{Id: id, PostId: "1", UserName: "n", Content: "c", ActorKind: kind, IsMine: mine, Replies: replies}
	}
	mc := &mockSocialServiceClient{
		GetCommentListFn: func(_ context.Context, _ *socialpb.GetCommentListRequest, _ ...grpc.CallOption) (*socialpb.CommentListResponse, error) {
			return &socialpb.CommentListResponse{Data: []*socialpb.CommentInfo{
				comment("10", 2, true, comment("11", 1, false)),
				comment("12", 1, false),
			}, Total: 2, Page: 1, PageSize: 20}, nil
		},
		CreateCommentFn: func(_ context.Context, _ *socialpb.CreateCommentRequest, _ ...grpc.CallOption) (*socialpb.CommentResponse, error) {
			return &socialpb.CommentResponse{Data: comment("13", 1, true)}, nil
		},
		UpdateCommentFn: func(_ context.Context, _ *socialpb.UpdateCommentRequest, _ ...grpc.CallOption) (*socialpb.CommentResponse, error) {
			return &socialpb.CommentResponse{Data: comment("13", 2, true)}, nil
		},
	}
	r := setupPostRouter(socialIdentityHolder(mc))

	var walk func(v interface{}, visit func(m map[string]interface{}))
	walk = func(v interface{}, visit func(m map[string]interface{})) {
		switch x := v.(type) {
		case map[string]interface{}:
			visit(x)
			for _, c := range x {
				walk(c, visit)
			}
		case []interface{}:
			for _, c := range x {
				walk(c, visit)
			}
		}
	}
	assertNoUserID := func(t *testing.T, w *httptest.ResponseRecorder) {
		t.Helper()
		var body interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		walk(body, func(m map[string]interface{}) {
			if _, has := m["userId"]; has {
				t.Errorf("public comment JSON must not carry userId: %s", w.Body.String())
			}
		})
	}

	list := doSocialReq(r, "GET", "/api/v1/public/posts/1/comments", "42", "3", "2", "")
	assertHTTPStatus(t, list, http.StatusOK)
	assertNoUserID(t, list)
	records := parseJSON(t, list.Body)["data"].(map[string]interface{})["records"].([]interface{})
	first := records[0].(map[string]interface{})
	if first["isMine"] != true || first["role"] != float64(2) {
		t.Errorf("first comment isMine/role = %v/%v, want true/2", first["isMine"], first["role"])
	}
	if reply := first["replies"].([]interface{})[0].(map[string]interface{}); reply["isMine"] != false || reply["role"] != float64(1) {
		t.Errorf("reply isMine/role = %v/%v, want false/1", reply["isMine"], reply["role"])
	}
	if records[1].(map[string]interface{})["isMine"] != false {
		t.Errorf("second comment isMine = %v, want false", records[1].(map[string]interface{})["isMine"])
	}

	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/v1/app/posts/1/comments", `{"content":"x"}`},
		{"PUT", "/api/v1/app/posts/comments/13", `{"content":"x"}`},
	} {
		w := doSocialReq(r, tc.method, tc.path, "42", "3", "1", tc.body)
		assertHTTPStatus(t, w, http.StatusOK)
		assertNoUserID(t, w)
		if parseJSON(t, w.Body)["data"].(map[string]interface{})["isMine"] != true {
			t.Errorf("%s %s: isMine missing or false", tc.method, tc.path)
		}
	}
}
