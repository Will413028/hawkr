package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

func newTestCommentUC(commentRepo *mockCommentRepo, postRepo *mockPostRepo) *CommentUseCase {
	return NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, &mockSellerClient{}, nil, func() int64 { return 99999 })
}

func TestCommentUC_ListTopLevel_PageDefaults(t *testing.T) {
	var gotLimit, gotOffset int64
	repo := &mockCommentRepo{
		listTopLevelFn: func(_ context.Context, _ int64, limit, offset int64) ([]*gen.CmsComment, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		countTopLevelFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
	}
	uc := newTestCommentUC(repo, &mockPostRepo{})
	_, _, err := uc.ListTopLevel(context.Background(), 1, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 20 {
		t.Errorf("limit = %d, want 20 (default)", gotLimit)
	}
	if gotOffset != 0 {
		t.Errorf("offset = %d, want 0", gotOffset)
	}
}

func TestCommentUC_CreateComment_TopLevel_Success(t *testing.T) {
	var createCalled bool
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			createCalled = true
			return &gen.CmsComment{ID: 1, PostID: postID, UserID: userID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 50}, nil
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	comment, err := uc.CreateComment(context.Background(), 10, Caller{BuyerID: 100}, "hello", 0, 0, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Error("Create was not called")
	}
	if comment.UserID != 100 {
		t.Errorf("UserID = %d, want 100", comment.UserID)
	}
}

// #1485 同 root cause class: 對不存在的 post 留言必須回 ErrNotFound,不得寫入孤兒 comment row。
func TestCommentUC_CreateComment_PostNotFound(t *testing.T) {
	var createCalled bool
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			createCalled = true
			return &gen.CmsComment{ID: 1, PostID: postID, UserID: userID}, nil
		},
	}
	// mockPostRepo 預設 GetByID 回 errNotFoundMock（post 不存在/已刪除）
	uc := newTestCommentUC(commentRepo, &mockPostRepo{})
	_, err := uc.CreateComment(context.Background(), 888888888888888888, Caller{BuyerID: 100}, "hello", 0, 0, "", false)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if createCalled {
		t.Error("Create must not be called when post does not exist (would insert orphan comment)")
	}
}

// #1485 同 root cause class: 回覆不存在的 parent comment 必須回 ErrNotFound（dangling parent_id）。
func TestCommentUC_CreateComment_Reply_ParentNotFound(t *testing.T) {
	var createCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsComment, error) {
			return nil, errNotFoundMock
		},
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			createCalled = true
			return &gen.CmsComment{ID: 2, PostID: postID, UserID: userID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 100}, nil
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: 100}, "reply", 999, 0, "", false)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if createCalled {
		t.Error("Create must not be called when parent comment does not exist")
	}
}

// #1485 同 root cause class: 回覆已軟刪除的 parent、或 parent 屬於別的貼文,同樣 404。
func TestCommentUC_CreateComment_Reply_ParentDeletedOrWrongPost(t *testing.T) {
	cases := []struct {
		name   string
		parent *gen.CmsComment
	}{
		{"soft-deleted parent", &gen.CmsComment{ID: 1, PostID: 10, DeletedAt: testTimePointer(time.Now())}},
		{"parent belongs to another post", &gen.CmsComment{ID: 1, PostID: 77}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var createCalled bool
			commentRepo := &mockCommentRepo{
				getByIDFn: func(_ context.Context, _ int64) (*gen.CmsComment, error) {
					return tc.parent, nil
				},
				createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
					createCalled = true
					return &gen.CmsComment{ID: 2, PostID: postID, UserID: userID}, nil
				},
			}
			postRepo := &mockPostRepo{
				getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
					return &gen.CmsPost{ID: id, SellerID: 100}, nil
				},
			}
			uc := newTestCommentUC(commentRepo, postRepo)
			_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: 100}, "reply", 1, 0, "", false)
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("error = %v, want ErrNotFound", err)
			}
			if createCalled {
				t.Error("Create must not be called for an invalid reply target")
			}
		})
	}
}

func TestCommentUC_CreateComment_Reply_Forbidden(t *testing.T) {
	// parentID > 0, but user is not the post seller → ErrForbidden
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			// parent comment has ParentID=0 (top-level), belongs to post 10
			return &gen.CmsComment{ID: id, PostID: 10, ParentID: 0}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 50}, nil // post seller is 50
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: 999}, "reply", 1, 0, "", false)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("error = %v, want ErrForbidden", err)
	}
}

func TestCommentUC_UpdateComment_EmptyContent(t *testing.T) {
	uc := newTestCommentUC(&mockCommentRepo{}, &mockPostRepo{})
	_, err := uc.UpdateComment(context.Background(), 1, Caller{BuyerID: 100}, "   ")
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("error = %v, want ErrBadRequest", err)
	}
}

func TestCommentUC_UpdateComment_NotFound(t *testing.T) {
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsComment, error) {
			return nil, errors.New("no rows")
		},
	}
	uc := newTestCommentUC(commentRepo, &mockPostRepo{})
	_, err := uc.UpdateComment(context.Background(), 1, Caller{BuyerID: 100}, "content")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestCommentUC_UpdateComment_Forbidden(t *testing.T) {
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, UserID: 50}, nil // owner is 50
		},
	}
	uc := newTestCommentUC(commentRepo, &mockPostRepo{})
	_, err := uc.UpdateComment(context.Background(), 1, Caller{BuyerID: 999}, "content") // requester is 999
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("error = %v, want ErrForbidden", err)
	}
}

func TestCommentUC_UpdateComment_Success(t *testing.T) {
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, UserID: 100, Role: domain.ActorKindBuyer}, nil
		},
		updateContentFn: func(_ context.Context, id int64, content string) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, UserID: 100, Content: testStringPointer(content), Role: domain.ActorKindBuyer}, nil
		},
	}
	uc := newTestCommentUC(commentRepo, &mockPostRepo{})
	comment, err := uc.UpdateComment(context.Background(), 1, Caller{BuyerID: 100}, "updated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comment.Content == nil || *comment.Content != "updated" {
		t.Errorf("content = %v, want %q", comment.Content, "updated")
	}
}

func TestCommentUC_DeleteComment_Forbidden(t *testing.T) {
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, UserID: 50, PostID: 10}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{SellerID: 50}, nil // post seller is also 50, requester is 999
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 999})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("error = %v, want ErrForbidden", err)
	}
}

func TestCommentUC_DeleteComment_Success(t *testing.T) {
	var softDeleteCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, UserID: 100, PostID: 10, ParentID: 0, Role: domain.ActorKindBuyer}, nil
		},
		softDeleteFn: func(_ context.Context, _ int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	uc := newTestCommentUC(commentRepo, &mockPostRepo{})
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 100}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !softDeleteCalled {
		t.Error("SoftDelete was not called")
	}
}

// --- 測試 CreateComment 根據 parentID 選擇正確的 client 取用戶資訊 ---

func TestCommentUC_CreateComment_TopLevel_UsesBuyerClient(t *testing.T) {
	// 買家留言 (role=1) 應呼叫 buyerClient，不呼叫 sellerClient
	var buyerCalled, sellerCalled bool
	var gotUserName, gotUserAvatar string

	// #2223：身分解析改走 *MetaBatch —— 與 REST 列表的 enrichCommentUsers 同一組 RPC，
	// 兩條路徑因此不可能給出不同答案；批次版的 map 命中與否也才分得出
	// 「查不到」與「查到但沒設頭像」。
	buyer := &mockBuyerClient{
		getBuyerMetaBatchFn: func(ctx context.Context, buyerIDs []int64) (map[int64]*AccountInfo, error) {
			buyerCalled = true
			out := make(map[int64]*AccountInfo, len(buyerIDs))
			for _, id := range buyerIDs {
				out[id] = &AccountInfo{Nickname: "買家Alice", Avatar: "buyer_avatar.jpg"}
			}
			return out, nil
		},
	}
	seller := &mockSellerClient{
		getSellerMetaBatchFn: func(ctx context.Context, storeIDs []int64) (map[int64]*AccountInfo, error) {
			sellerCalled = true
			out := make(map[int64]*AccountInfo, len(storeIDs))
			for _, id := range storeIDs {
				out[id] = &AccountInfo{Nickname: "不該出現", Avatar: "不該出現"}
			}
			return out, nil
		},
	}
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, _, _, _ int64, userName, userAvatar, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			gotUserName = userName
			gotUserAvatar = userAvatar
			return &gen.CmsComment{ID: 1, PostID: 10, UserID: 100, Role: domain.ActorKindBuyer}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 50}, nil
		},
	}
	uc := NewCommentUseCase(commentRepo, postRepo, buyer, seller, nil, func() int64 { return 99999 })

	_, err := uc.CreateComment(context.Background(), 10, Caller{BuyerID: 100}, "hello", 0, 0, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !buyerCalled {
		t.Error("buyerClient.GetBuyerMetaBatch was NOT called for top-level comment")
	}
	if sellerCalled {
		t.Error("sellerClient.GetSellerMetaBatch should NOT be called for top-level comment")
	}
	if gotUserName != "買家Alice" {
		t.Errorf("userName = %q, want %q", gotUserName, "買家Alice")
	}
	if gotUserAvatar != "buyer_avatar.jpg" {
		t.Errorf("userAvatar = %q, want %q", gotUserAvatar, "buyer_avatar.jpg")
	}
}

func TestCommentUC_CreateComment_Reply_UsesSellerClient(t *testing.T) {
	var buyerCalled, sellerCalled bool
	var gotUserName, gotUserAvatar string
	var gotCommentUserID int64

	const rawAccountID int64 = 700489000
	const currentStoreID int64 = 50

	buyer := &mockBuyerClient{
		getBuyerMetaBatchFn: func(ctx context.Context, buyerIDs []int64) (map[int64]*AccountInfo, error) {
			buyerCalled = true
			out := make(map[int64]*AccountInfo, len(buyerIDs))
			for _, id := range buyerIDs {
				out[id] = &AccountInfo{Nickname: "不該出現", Avatar: "不該出現"}
			}
			return out, nil
		},
	}
	seller := &mockSellerClient{
		getSellerMetaBatchFn: func(ctx context.Context, storeIDs []int64) (map[int64]*AccountInfo, error) {
			sellerCalled = true
			out := make(map[int64]*AccountInfo, len(storeIDs))
			for _, id := range storeIDs {
				if id != currentStoreID {
					t.Errorf("sellerClient.GetSellerMetaBatch called with storeID=%d, want %d (the resolved store surrogate, not the raw account_id %d)", id, currentStoreID, rawAccountID)
				}
				out[id] = &AccountInfo{Nickname: "賣家Bob", Avatar: "seller_logo.jpg"}
			}
			return out, nil
		},
	}
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, ParentID: 0}, nil // parent is top-level, belongs to post 10
		},
		createFn: func(_ context.Context, _, _, userID int64, userName, userAvatar, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			gotCommentUserID = userID
			gotUserName = userName
			gotUserAvatar = userAvatar
			return &gen.CmsComment{ID: 2, PostID: 10, UserID: userID, ParentID: 1}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: currentStoreID}, nil
		},
	}
	uc := NewCommentUseCase(commentRepo, postRepo, buyer, seller, nil, func() int64 { return 99999 })

	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: currentStoreID}, "seller reply", 1, 0, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sellerCalled {
		t.Error("sellerClient.GetSellerMetaBatch was NOT called for seller reply")
	}
	if buyerCalled {
		t.Error("buyerClient.GetBuyerProfile should NOT be called for seller reply")
	}
	if gotUserName != "賣家Bob" {
		t.Errorf("userName = %q, want %q (empty means GetSellerMetaBatch silently failed to resolve)", gotUserName, "賣家Bob")
	}
	if gotUserAvatar != "seller_logo.jpg" {
		t.Errorf("userAvatar = %q, want %q", gotUserAvatar, "seller_logo.jpg")
	}
	if gotCommentUserID != currentStoreID {
		t.Errorf("comment.UserID = %d, want %d (store identity is keyed by store_id, never the raw account_id %d)", gotCommentUserID, currentStoreID, rawAccountID)
	}
}

func TestCommentUC_CreateComment_NonSellerReply_Forbidden(t *testing.T) {
	// 非賣家嘗試回覆 (parentID>0) 應被擋住，兩個 client 都不該被呼叫
	var buyerCalled, sellerCalled bool

	buyer := &mockBuyerClient{
		getBuyerProfileFn: func(_ context.Context, _ int64) (*AccountInfo, error) {
			buyerCalled = true
			return &AccountInfo{}, nil
		},
	}
	seller := &mockSellerClient{
		getSellerInfoFn: func(_ context.Context, _ int64) (*AccountInfo, error) {
			sellerCalled = true
			return &AccountInfo{}, nil
		},
	}
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, ParentID: 0}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 50}, nil // seller is 50
		},
	}
	uc := NewCommentUseCase(commentRepo, postRepo, buyer, seller, nil, func() int64 { return 99999 })

	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: 999}, "imposter reply", 1, 0, "", false)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("error = %v, want ErrForbidden", err)
	}
	if buyerCalled {
		t.Error("buyerClient should NOT be called when permission check fails")
	}
	if sellerCalled {
		t.Error("sellerClient should NOT be called when permission check fails")
	}
}

func TestCommentUC_IsLiked_AnonymousUser(t *testing.T) {
	// userID=0 should always return false without hitting repo
	var getLikeCalled bool
	repo := &mockCommentRepo{
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
			getLikeCalled = true
			return &gen.CmsCommentLike{}, nil
		},
	}
	uc := newTestCommentUC(repo, &mockPostRepo{})
	liked := uc.IsLiked(context.Background(), 1, Actor{Kind: domain.ActorKindBuyer, ID: 0})
	if liked {
		t.Error("expected liked=false for anonymous user")
	}
	if getLikeCalled {
		t.Error("GetLike should NOT be called for userID=0")
	}
}

func TestCommentUC_IsLiked_Liked(t *testing.T) {
	repo := &mockCommentRepo{
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
			return &gen.CmsCommentLike{}, nil
		},
	}
	uc := newTestCommentUC(repo, &mockPostRepo{})
	if !uc.IsLiked(context.Background(), 1, Actor{Kind: domain.ActorKindBuyer, ID: 100}) {
		t.Error("expected liked=true")
	}
}

func TestCommentUC_IsLiked_NotLiked(t *testing.T) {
	repo := &mockCommentRepo{
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
			return nil, errNotFoundMock
		},
	}
	uc := newTestCommentUC(repo, &mockPostRepo{})
	if uc.IsLiked(context.Background(), 1, Actor{Kind: domain.ActorKindBuyer, ID: 100}) {
		t.Error("expected liked=false")
	}
}

func TestCommentUC_ListTopLevel_Pagination(t *testing.T) {
	var gotLimit, gotOffset int64
	repo := &mockCommentRepo{
		listTopLevelFn: func(_ context.Context, _ int64, limit, offset int64) ([]*gen.CmsComment, error) {
			gotLimit = limit
			gotOffset = offset
			return []*gen.CmsComment{
				{ID: 1, PostID: 10, UserID: 100},
				{ID: 2, PostID: 10, UserID: 200},
			}, nil
		},
		countTopLevelFn: func(_ context.Context, _ int64) (int64, error) {
			return 5, nil
		},
	}
	uc := newTestCommentUC(repo, &mockPostRepo{})
	comments, total, err := uc.ListTopLevel(context.Background(), 10, 2, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 10 {
		t.Errorf("limit = %d, want 10", gotLimit)
	}
	if gotOffset != 10 {
		t.Errorf("offset = %d, want 10 (page=2, pageSize=10 → offset=10)", gotOffset)
	}
	if len(comments) != 2 {
		t.Errorf("len(comments) = %d, want 2", len(comments))
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
}

func TestCommentUC_ToggleLike_Like(t *testing.T) {
	var createLikeCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, UserID: 200, LikeCount: 4, Role: domain.ActorKindBuyer}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
			return nil, errNotFoundMock
		},
		createLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error {
			createLikeCalled = true
			return nil
		},
	}
	pub := &mockContentEventPublisher{}
	uc := NewCommentUseCase(commentRepo, &mockPostRepo{}, &mockBuyerClient{}, &mockSellerClient{}, pub, func() int64 { return 99999 })
	liked, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !liked {
		t.Error("expected liked=true")
	}
	if !createLikeCalled {
		t.Error("CreateLike was not called")
	}
	if len(pub.publishCounterDeltaCalls) != 1 {
		t.Fatalf("publishCounterDeltaCalls = %d, want 1", len(pub.publishCounterDeltaCalls))
	}
	wantDelta := publishCounterDeltaCall{entityType: "comment", entityID: 1, field: "like_count", delta: 1}
	if pub.publishCounterDeltaCalls[0] != wantDelta {
		t.Errorf("publishCounterDeltaCalls[0] = %+v, want %+v", pub.publishCounterDeltaCalls[0], wantDelta)
	}
	if len(pub.commentLikeChangedCalls) != 1 {
		t.Fatalf("commentLikeChangedCalls = %d, want 1", len(pub.commentLikeChangedCalls))
	}
	if got := pub.commentLikeChangedCalls[0].likeCount; got != 5 {
		t.Errorf("PublishCommentLikeChanged likeCount = %d, want 5 (existing.LikeCount=4 + 1, not a stale re-fetch)", got)
	}
}

func TestCommentUC_ToggleLike_Unlike(t *testing.T) {
	var deleteLikeCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, UserID: 200, LikeCount: 4, Role: domain.ActorKindBuyer}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
			return &gen.CmsCommentLike{}, nil // already liked
		},
		deleteLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error {
			deleteLikeCalled = true
			return nil
		},
	}
	pub := &mockContentEventPublisher{}
	uc := NewCommentUseCase(commentRepo, &mockPostRepo{}, &mockBuyerClient{}, &mockSellerClient{}, pub, func() int64 { return 99999 })
	liked, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if liked {
		t.Error("expected liked=false (unlike)")
	}
	if !deleteLikeCalled {
		t.Error("DeleteLike was not called")
	}
	if len(pub.publishCounterDeltaCalls) != 1 {
		t.Fatalf("publishCounterDeltaCalls = %d, want 1", len(pub.publishCounterDeltaCalls))
	}
	wantDelta := publishCounterDeltaCall{entityType: "comment", entityID: 1, field: "like_count", delta: -1}
	if pub.publishCounterDeltaCalls[0] != wantDelta {
		t.Errorf("publishCounterDeltaCalls[0] = %+v, want %+v", pub.publishCounterDeltaCalls[0], wantDelta)
	}
	if len(pub.commentLikeChangedCalls) != 1 {
		t.Fatalf("commentLikeChangedCalls = %d, want 1", len(pub.commentLikeChangedCalls))
	}
	if got := pub.commentLikeChangedCalls[0].likeCount; got != 3 {
		t.Errorf("PublishCommentLikeChanged likeCount = %d, want 3 (existing.LikeCount=4 - 1, not a stale re-fetch)", got)
	}
}

// #1485: 對不存在的 comment id 按讚必須回 ErrNotFound,不得寫入孤兒 like row。
func TestCommentUC_ToggleLike_CommentNotFound(t *testing.T) {
	var createLikeCalled, getLikeCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsComment, error) {
			return nil, errNotFoundMock
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
			getLikeCalled = true
			return nil, errNotFoundMock
		},
		createLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error {
			createLikeCalled = true
			return nil
		},
	}
	uc := newTestCommentUC(commentRepo, &mockPostRepo{})
	_, err := uc.ToggleLike(context.Background(), 999999999999999999, Caller{BuyerID: 100})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if getLikeCalled {
		t.Error("GetLike must not be called when comment does not exist")
	}
	if createLikeCalled {
		t.Error("CreateLike must not be called when comment does not exist (would insert orphan row)")
	}
}

// #1485: 對已軟刪除的 comment 按讚同樣回 ErrNotFound（GetComment 未過濾 deleted_at,需顯式判斷）。
func TestCommentUC_ToggleLike_CommentSoftDeleted(t *testing.T) {
	var createLikeCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, DeletedAt: testTimePointer(time.Now())}, nil
		},
		createLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error {
			createLikeCalled = true
			return nil
		},
	}
	uc := newTestCommentUC(commentRepo, &mockPostRepo{})
	_, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: 100})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if createLikeCalled {
		t.Error("CreateLike must not be called for a soft-deleted comment")
	}
}

// --- comment.created event 的 postAuthorType：貼文作者恆為店家 ---

func TestCommentUC_CreateComment_PublishesFixedSellerPostAuthorType(t *testing.T) {
	const postAuthorID int64 = 50
	const commenterID int64 = 100

	pub := &mockContentEventPublisher{}
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, id, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: postID, UserID: userID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: postAuthorID}, nil
		},
	}
	// buyer 與 store 的 client 都對這個數字回「查得到」：type 仍是固定的 seller，不看 client。
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{
		getBuyerProfileFn: func(_ context.Context, _ int64) (*AccountInfo, error) {
			return &AccountInfo{Nickname: "buyerSameNumber"}, nil
		},
	}, &mockSellerClient{}, pub, func() int64 { return 99999 })

	_, err := uc.CreateComment(context.Background(), 10, Caller{BuyerID: commenterID}, "hello", 0, 0, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentCreatedCalls) != 1 {
		t.Fatalf("expected 1 comment.created publish, got %d", len(pub.commentCreatedCalls))
	}
	got := pub.commentCreatedCalls[0]
	if got.postAuthorType != "seller" {
		t.Errorf("postAuthorType = %q, want %q", got.postAuthorType, "seller")
	}
	if got.postAuthorStoreID != postAuthorID {
		t.Errorf("postAuthorStoreID = %d, want %d", got.postAuthorStoreID, postAuthorID)
	}
}

// postAuthorStoreID 必須是 cms_post.seller_id 的原始值(未經 ResolvePrincipal)，與已解析的
// postAuthorID 落在不同身分空間。fixture 讓 ResolvePrincipal 對 raw store_id 回傳一個刻意不同
// 的值,兩者若被誤植成同一個(例如複製 resolved 值)這裡就會轉紅。
func TestCommentUC_CreateComment_PostAuthorIsSeller_PublishesRawStoreID(t *testing.T) {
	const postAuthorStoreID int64 = 50     // cms_post.seller_id 原始值(raw store_id)
	const resolvedPostAuthorID int64 = 950 // ResolvePrincipal 解析後的 account_id,刻意不同於 raw
	const commenterID int64 = 100

	pub := &mockContentEventPublisher{}
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, id, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: postID, UserID: userID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: postAuthorStoreID}, nil
		},
	}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, storeID int64) int64 {
			if storeID == postAuthorStoreID {
				return resolvedPostAuthorID
			}
			return storeID
		},
	}, pub, func() int64 { return 99999 })

	_, err := uc.CreateComment(context.Background(), 10, Caller{BuyerID: commenterID}, "hello", 0, 0, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentCreatedCalls) != 1 {
		t.Fatalf("expected 1 comment.created publish, got %d", len(pub.commentCreatedCalls))
	}
	got := pub.commentCreatedCalls[0]
	if got.postAuthorID != resolvedPostAuthorID {
		t.Errorf("postAuthorID = %d, want %d (resolved account_id)", got.postAuthorID, resolvedPostAuthorID)
	}
	if got.postAuthorStoreID != postAuthorStoreID {
		t.Errorf("postAuthorStoreID = %d, want %d (raw store_id, must not be the resolved value)", got.postAuthorStoreID, postAuthorStoreID)
	}
	if got.postAuthorStoreID == got.postAuthorID {
		t.Errorf("postAuthorStoreID (%d) must differ from resolved postAuthorID (%d) — fixture failed to distinguish raw vs resolved", got.postAuthorStoreID, got.postAuthorID)
	}
}

// --- comment.replied publish 帶新建 comment.ID ---

func TestCommentUC_CreateComment_Reply_PublishesCommentId(t *testing.T) {
	const sellerID int64 = 50
	const parentCommentID int64 = 1
	const newCommentID int64 = 999
	const parentAuthorID int64 = 200 // top-level 留言者，將收到 reply 通知

	pub := &mockContentEventPublisher{}
	commentRepo := &mockCommentRepo{
		// GetByID 同時被 reply 權限檢查、replyToName 取得、publishCommentReplied 對象查詢使用
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{
				ID:       id,
				PostID:   10,
				ParentID: 0,
				UserID:   parentAuthorID,
				Role:     int16(domain.ActorKindBuyer),
			}, nil
		},
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: newCommentID, PostID: postID, UserID: userID, ParentID: parentID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: sellerID}, nil
		},
	}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, &mockSellerClient{}, pub, func() int64 { return newCommentID })

	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: sellerID}, "回覆", parentCommentID, parentCommentID, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentRepliedCalls) != 1 {
		t.Fatalf("expected 1 comment.replied publish, got %d", len(pub.commentRepliedCalls))
	}
	got := pub.commentRepliedCalls[0]
	if got.commentID != newCommentID {
		t.Errorf("commentID = %d, want %d (新建 comment.ID)", got.commentID, newCommentID)
	}
	if got.replyToUserID != parentAuthorID {
		t.Errorf("replyToUserID = %d, want %d", got.replyToUserID, parentAuthorID)
	}
	if got.replierID != sellerID {
		t.Errorf("replierID = %d, want %d (店家身分的 id 是 store_id)", got.replierID, sellerID)
	}
	if got.replyToUserType != "buyer" {
		t.Errorf("replyToUserType = %q, want %q", got.replyToUserType, "buyer")
	}
}

// replyToUserStoreID 必須是被回覆留言的 cms_comment.user_id 原始值(未經 ResolvePrincipal)，
// 只在 replyTo.Role == seller 時才是真正的 store_id。fixture 讓 ResolvePrincipal 對 raw
// 值回傳一個刻意不同的值，兩者若被誤植成同一個(例如複製 resolved 值)這裡就會轉紅。
func TestCommentUC_CreateComment_ReplyToSeller_PublishesRawStoreID(t *testing.T) {
	const sellerID int64 = 50
	const parentCommentID int64 = 1
	const newCommentID int64 = 999
	const parentAuthorStoreID int64 = 200   // 被回覆留言作者的 cms_comment.user_id 原始值(raw store_id)
	const resolvedReplyToUserID int64 = 970 // ResolvePrincipal 解析後的 account_id，刻意不同於 raw

	pub := &mockContentEventPublisher{}
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{
				ID:       id,
				PostID:   10,
				ParentID: 0,
				UserID:   parentAuthorStoreID,
				Role:     int16(domain.ActorKindStore),
			}, nil
		},
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: newCommentID, PostID: postID, UserID: userID, ParentID: parentID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: sellerID}, nil
		},
	}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, storeID int64) int64 {
			if storeID == parentAuthorStoreID {
				return resolvedReplyToUserID
			}
			return storeID
		},
	}, pub, func() int64 { return newCommentID })

	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: sellerID}, "回覆", parentCommentID, parentCommentID, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentRepliedCalls) != 1 {
		t.Fatalf("expected 1 comment.replied publish, got %d", len(pub.commentRepliedCalls))
	}
	got := pub.commentRepliedCalls[0]
	if got.replyToUserID != resolvedReplyToUserID {
		t.Errorf("replyToUserID = %d, want %d (resolved account_id)", got.replyToUserID, resolvedReplyToUserID)
	}
	if got.replyToUserStoreID != parentAuthorStoreID {
		t.Errorf("replyToUserStoreID = %d, want %d (raw store_id, must not be the resolved value)", got.replyToUserStoreID, parentAuthorStoreID)
	}
	if got.replyToUserStoreID == got.replyToUserID {
		t.Errorf("replyToUserStoreID (%d) must differ from resolved replyToUserID (%d) — fixture failed to distinguish raw vs resolved", got.replyToUserStoreID, got.replyToUserID)
	}
}

// --- #1171: comment_count 只計頂層留言（回覆不進 comment_count）---

func TestCommentUC_CreateComment_TopLevel_PublishesCommentCountDelta(t *testing.T) {
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: 1, PostID: postID, UserID: userID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 50}, nil
		},
	}
	pub := &mockContentEventPublisher{}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, &mockSellerClient{}, pub, func() int64 { return 99999 })
	if _, err := uc.CreateComment(context.Background(), 10, Caller{BuyerID: 100}, "hello", 0, 0, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.publishCounterDeltaCalls) != 1 {
		t.Fatalf("publishCounterDeltaCalls = %d, want 1", len(pub.publishCounterDeltaCalls))
	}
	got := pub.publishCounterDeltaCalls[0]
	want := publishCounterDeltaCall{entityType: "post", entityID: 10, field: "comment_count", delta: 1}
	if got != want {
		t.Errorf("publishCounterDeltaCalls[0] = %+v, want %+v", got, want)
	}
}

func TestCommentUC_CreateComment_Reply_PublishesReplyCountDelta(t *testing.T) {
	const sellerID int64 = 50
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, ParentID: 0}, nil // parent is top-level, belongs to post 10
		},
		createFn: func(_ context.Context, _, _, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: 2, PostID: 10, UserID: userID, ParentID: 1}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: sellerID}, nil
		},
	}
	pub := &mockContentEventPublisher{}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, &mockSellerClient{}, pub, func() int64 { return 99999 })
	if _, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: sellerID}, "reply", 1, 0, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.publishCounterDeltaCalls) != 1 {
		t.Fatalf("publishCounterDeltaCalls = %d, want 1", len(pub.publishCounterDeltaCalls))
	}
	got := pub.publishCounterDeltaCalls[0]
	want := publishCounterDeltaCall{entityType: "comment", entityID: 1, field: "reply_count", delta: 1}
	if got != want {
		t.Errorf("publishCounterDeltaCalls[0] = %+v, want %+v", got, want)
	}
}

func TestCommentUC_DeleteComment_TopLevel_PublishesCommentCountDelta(t *testing.T) {
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, UserID: 100, PostID: 10, ParentID: 0, Role: domain.ActorKindBuyer}, nil // top-level
		},
	}
	pub := &mockContentEventPublisher{}
	uc := NewCommentUseCase(commentRepo, &mockPostRepo{}, &mockBuyerClient{}, &mockSellerClient{}, pub, func() int64 { return 99999 })
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 100}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.publishCounterDeltaCalls) != 1 {
		t.Fatalf("publishCounterDeltaCalls = %d, want 1", len(pub.publishCounterDeltaCalls))
	}
	got := pub.publishCounterDeltaCalls[0]
	want := publishCounterDeltaCall{entityType: "post", entityID: 10, field: "comment_count", delta: -1}
	if got != want {
		t.Errorf("publishCounterDeltaCalls[0] = %+v, want %+v", got, want)
	}
}

func TestCommentUC_DeleteComment_Reply_PublishesReplyCountDelta(t *testing.T) {
	// 刪除回覆應 publish {comment, parentID, reply_count, -1}，不動貼文 comment_count。
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, UserID: 100, PostID: 10, ParentID: 5, Role: domain.ActorKindBuyer}, nil // reply
		},
	}
	pub := &mockContentEventPublisher{}
	uc := NewCommentUseCase(commentRepo, &mockPostRepo{}, &mockBuyerClient{}, &mockSellerClient{}, pub, func() int64 { return 99999 })
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 100}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.publishCounterDeltaCalls) != 1 {
		t.Fatalf("publishCounterDeltaCalls = %d, want 1", len(pub.publishCounterDeltaCalls))
	}
	got := pub.publishCounterDeltaCalls[0]
	want := publishCounterDeltaCall{entityType: "comment", entityID: 5, field: "reply_count", delta: -1}
	if got != want {
		t.Errorf("publishCounterDeltaCalls[0] = %+v, want %+v", got, want)
	}
}

// ==================== 賣家停權 self-vs-other 細分（46 §4.4 / 06 §四.10） ====================

// TestCommentUC_CreateComment_TopLevel_SelfSuspended_Blocked: the caller is a
// suspended seller commenting on THEIR OWN post (self, "客服/經營" management
// case per 06 §四.10) — must be blocked.
func TestCommentUC_CreateComment_TopLevel_SelfSuspended_Blocked(t *testing.T) {
	var createCalled bool
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			createCalled = true
			return &gen.CmsComment{ID: 1, PostID: postID, UserID: userID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 100}, nil // post owned by the caller (100)
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: 100}, "hello", 0, 0, "", true)
	if !errors.Is(err, ErrSellerSuspendedOperationBlocked) {
		t.Errorf("error = %v, want ErrSellerSuspendedOperationBlocked", err)
	}
	if createCalled {
		t.Error("Create must not be called when self-post comment is blocked by suspension")
	}
}

// TestCommentUC_CreateComment_TopLevel_OtherSuspended_Allowed: a suspended
// seller commenting on SOMEONE ELSE's post (buyer capacity, or a seller merely
// participating elsewhere) is never gated — ALLOW-ALWAYS per 06 §四.10.
func TestCommentUC_CreateComment_TopLevel_OtherSuspended_Allowed(t *testing.T) {
	var createCalled bool
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			createCalled = true
			return &gen.CmsComment{ID: 1, PostID: postID, UserID: userID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 50}, nil // post owned by someone else (50), not caller (100)
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	_, err := uc.CreateComment(context.Background(), 10, Caller{BuyerID: 100, StoreID: 100}, "hello", 0, 0, "", true)
	if err != nil {
		t.Fatalf("unexpected error for other-post comment by a suspended seller: %v", err)
	}
	if !createCalled {
		t.Error("Create should be called — commenting on someone else's post is never gated")
	}
}

// TestCommentUC_CreateComment_Reply_SuspendedSelf_Blocked: replies are always
// "self" by construction (only the post's own seller may reply — see the
// existing ErrForbidden check) — a suspended seller replying on their own post
// must be blocked.
func TestCommentUC_CreateComment_Reply_SuspendedSelf_Blocked(t *testing.T) {
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, ParentID: 0}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 100}, nil
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: 100}, "reply", 1, 0, "", true)
	if !errors.Is(err, ErrSellerSuspendedOperationBlocked) {
		t.Errorf("error = %v, want ErrSellerSuspendedOperationBlocked", err)
	}
}

func TestCommentUC_CreateComment_Reply_StaleSurrogateForbidden(t *testing.T) {
	const staleSurrogate int64 = 1001
	const currentStoreID int64 = 9999
	var createCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, ParentID: 0}, nil
		},
		createFn: func(_ context.Context, _, postID, uid int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			createCalled = true
			return &gen.CmsComment{ID: 2, PostID: postID, UserID: uid}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: staleSurrogate}, nil
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: currentStoreID}, "reply", 1, 0, "", false)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("error = %v, want ErrForbidden (new incarnation must not reply on the old store's post)", err)
	}
	if createCalled {
		t.Error("Create must not be called: post belongs to the old store, not the caller's current store")
	}
}

func TestCommentUC_CreateComment_Reply_CurrentSurrogateAllowed(t *testing.T) {
	const currentStoreID int64 = 2002
	var createCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, ParentID: 0}, nil
		},
		createFn: func(_ context.Context, _, postID, uid int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			createCalled = true
			return &gen.CmsComment{ID: 2, PostID: postID, UserID: uid, ParentID: parentID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: currentStoreID}, nil
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: currentStoreID}, "reply", 1, 0, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v (current seller must be able to reply on their own post)", err)
	}
	if !createCalled {
		t.Error("Create should be called: post.SellerID matches the caller's current store")
	}
}

// site 2：CreateComment 頂層 self-suspend 判斷（對自己貼文留言＝管理行為，停權時擋）。
// Stale：貼文屬舊 store 1001（== userID），現任 store 9999。舊碼 post.SellerID==userID → 擋（bug）。
func TestCommentUC_CreateComment_TopLevel_SelfSuspend_StaleSurrogateNotBlocked(t *testing.T) {
	const userID int64 = 1001
	const staleSurrogate int64 = 1001
	const currentStoreID int64 = 9999
	var createCalled bool
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, _, postID, uid int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			createCalled = true
			return &gen.CmsComment{ID: 1, PostID: postID, UserID: uid}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: staleSurrogate}, nil
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	_, err := uc.CreateComment(context.Background(), 10, Caller{BuyerID: userID, StoreID: currentStoreID}, "hi", 0, 0, "", true)
	if err != nil {
		t.Fatalf("must NOT be blocked: current store (9999) != post.SellerID (1001), not a self-post: %v", err)
	}
	if !createCalled {
		t.Error("Create should be called — commenting on the old store's post is not a suspended self-action")
	}
}

// Current：post.SellerID == 現任 store 2002 → 真正的 self，停權時擋。
func TestCommentUC_CreateComment_TopLevel_SelfSuspend_CurrentSurrogateBlocked(t *testing.T) {
	const currentStoreID int64 = 2002
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: currentStoreID}, nil
		},
	}
	uc := newTestCommentUC(&mockCommentRepo{}, postRepo)
	_, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: currentStoreID}, "hi", 0, 0, "", true)
	if !errors.Is(err, ErrSellerSuspendedOperationBlocked) {
		t.Errorf("error = %v, want ErrSellerSuspendedOperationBlocked (self-post while suspended)", err)
	}
}

// site 3：DeleteComment 貼文賣家可刪自己貼文下留言。
// Stale：貼文屬舊 store 1001（== userID），現任 store 9999 →新化身不得刪舊 store 貼文的留言。
// 舊碼 post.SellerID(1001)==userID(1001) → 放行（bug）。留言作者是別人(50)故非 owner 分支。
func TestCommentUC_DeleteComment_StaleSurrogateForbidden(t *testing.T) {
	const userID int64 = 1001
	const staleSurrogate int64 = 1001
	const currentStoreID int64 = 9999
	var softDeleteCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, UserID: 50, PostID: 10}, nil // author 50, not the caller
		},
		softDeleteFn: func(_ context.Context, _ int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{SellerID: staleSurrogate}, nil
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: userID, StoreID: currentStoreID}); !errors.Is(err, ErrForbidden) {
		t.Errorf("error = %v, want ErrForbidden (new incarnation must not moderate the old store's comments)", err)
	}
	if softDeleteCalled {
		t.Error("SoftDelete must not be called: post belongs to the old store, not the caller's current store")
	}
}

// Current：post.SellerID == 現任 store 2002 → 賣家可刪自己貼文下的留言（登入 id 1001 不同也可）。
func TestCommentUC_DeleteComment_CurrentSurrogateSuccess(t *testing.T) {
	const userID int64 = 1001
	const currentStoreID int64 = 2002
	var softDeleteCalled bool
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, UserID: 50, PostID: 10, ParentID: 0}, nil // author 50, not the caller
		},
		softDeleteFn: func(_ context.Context, _ int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{SellerID: currentStoreID}, nil
		},
	}
	uc := newTestCommentUC(commentRepo, postRepo)
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: userID, StoreID: currentStoreID}); err != nil {
		t.Fatalf("unexpected error: %v (current seller must be able to moderate their own post's comments)", err)
	}
	if !softDeleteCalled {
		t.Error("SoftDelete should be called: post.SellerID matches the caller's current store")
	}
}

func TestCommentUC_CreateComment_PublishesResolvedAccountID_NotRawStoreID(t *testing.T) {
	const storeID int64 = 9999           // post.SellerID — a store surrogate
	const resolvedAccountID int64 = 1001 // what sellerClient.ResolvePrincipal resolves storeID to
	const commenterID int64 = 100

	pub := &mockContentEventPublisher{}
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, id, postID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: postID, UserID: userID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: storeID}, nil
		},
	}
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			if sid == storeID {
				return resolvedAccountID
			}
			return sid
		},
	}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, seller, pub, func() int64 { return 99999 })

	// Top-level comment (parentID=0), storeID(caller)=0 → not the post's current seller.
	_, err := uc.CreateComment(context.Background(), 10, Caller{BuyerID: commenterID}, "hello", 0, 0, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentCreatedCalls) != 1 {
		t.Fatalf("expected 1 comment.created publish, got %d", len(pub.commentCreatedCalls))
	}
	if pub.commentCreatedCalls[0].postAuthorID != resolvedAccountID {
		t.Errorf("PublishCommentCreated postAuthorID = %d, want resolved account_id %d, not raw store_id surrogate %d",
			pub.commentCreatedCalls[0].postAuthorID, resolvedAccountID, storeID)
	}
}

// ==================== #1626: 通知收件人身分空間 ====================
//
// notifications.user_id 的正確內容由讀取端決定，而讀取端是不對稱的
// （gateway resolveInboxUserID）：user_type='seller' 以 account_id 查、
// user_type='buyer' 以現任 buyer_id 查。cms_comment.user_id 存的是留言當下
// active role 的 surrogate（買家 buyer_id / 賣家 store_id），所以賣家目標必須
// 先解析成 account_id，買家目標則必須維持原樣。
//
// 錯了不會有任何錯誤訊號：template engine 取到什麼就寫什麼
// （template_handler.go extractUserID 零驗證），inbox row 寫入成功、收件人永遠查不到。

func TestCommentUC_CreateComment_Reply_SellerTarget_PublishesResolvedAccountID(t *testing.T) {
	const postStoreID int64 = 50       // 貼文所屬店（＝回覆者的 store surrogate）
	const parentCommentID int64 = 1    //
	const newCommentID int64 = 999     //
	const targetStoreID int64 = 8888   // 被回覆的那則留言的作者：賣家 store surrogate
	const targetAccountID int64 = 3001 // 其登入 principal

	pub := &mockContentEventPublisher{}
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{
				ID:       id,
				PostID:   10,
				ParentID: 0,
				UserID:   targetStoreID,
				Role:     int16(domain.ActorKindStore),
			}, nil
		},
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: newCommentID, PostID: postID, UserID: userID, ParentID: parentID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: postStoreID}, nil
		},
	}
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			if sid == targetStoreID {
				return targetAccountID
			}
			return sid
		},
	}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, seller, pub, func() int64 { return newCommentID })

	if _, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: postStoreID}, "回覆", parentCommentID, parentCommentID, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentRepliedCalls) != 1 {
		t.Fatalf("expected 1 comment.replied publish, got %d", len(pub.commentRepliedCalls))
	}
	got := pub.commentRepliedCalls[0]
	if got.replyToUserType != "seller" {
		t.Fatalf("replyToUserType = %q, want seller (payload_filter 靠它分流到 comment.replied.seller template)", got.replyToUserType)
	}
	if got.replyToUserID != targetAccountID {
		t.Errorf("replyToUserID = %d, want resolved account_id %d, not raw store_id surrogate %d (賣家收件匣以 account_id 查，送 store_id 等於寫一列永遠查不到的 inbox row)",
			got.replyToUserID, targetAccountID, targetStoreID)
	}
}

func TestCommentUC_CreateComment_Reply_BuyerTarget_KeepsBuyerIDUnresolved(t *testing.T) {
	const postStoreID int64 = 50
	const parentCommentID int64 = 1
	const newCommentID int64 = 999
	const targetBuyerID int64 = 7777 // 被回覆者：買家 buyer_id surrogate

	pub := &mockContentEventPublisher{}
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{
				ID:       id,
				PostID:   10,
				ParentID: 0,
				UserID:   targetBuyerID,
				Role:     int16(domain.ActorKindBuyer),
			}, nil
		},
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: newCommentID, PostID: postID, UserID: userID, ParentID: parentID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: postStoreID}, nil
		},
	}
	// 這個 resolver 對「任何」id 都會回一個不同的值。若 buyer 分支誤走解析，
	// replyToUserID 就會變成 4242 而非 7777 —— 這正是本測試要擋的迴歸。
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, _ int64) int64 { return 4242 },
	}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, seller, pub, func() int64 { return newCommentID })

	if _, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: postStoreID}, "回覆", parentCommentID, parentCommentID, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentRepliedCalls) != 1 {
		t.Fatalf("expected 1 comment.replied publish, got %d", len(pub.commentRepliedCalls))
	}
	got := pub.commentRepliedCalls[0]
	if got.replyToUserType != "buyer" {
		t.Fatalf("replyToUserType = %q, want buyer", got.replyToUserType)
	}
	if got.replyToUserID != targetBuyerID {
		t.Errorf("replyToUserID = %d, want buyer_id %d unresolved (買家收件匣以 buyer_id 查；解析成 account_id 會讓 comment.replied.buyer 靜默失投)",
			got.replyToUserID, targetBuyerID)
	}
}

func TestCommentUC_ToggleLike_SellerAuthor_PublishesResolvedAccountID(t *testing.T) {
	const authorStoreID int64 = 8888
	const authorAccountID int64 = 3001
	const likerID int64 = 100

	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			if sid == authorStoreID {
				return authorAccountID
			}
			return sid
		},
	}
	newRepo := func(alreadyLiked bool) *mockCommentRepo {
		return &mockCommentRepo{
			getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
				return &gen.CmsComment{ID: id, PostID: 10, UserID: authorStoreID, LikeCount: 4, Role: int16(domain.ActorKindStore)}, nil
			},
			getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
				if alreadyLiked {
					return &gen.CmsCommentLike{}, nil
				}
				return nil, errNotFoundMock
			},
			createLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error { return nil },
			deleteLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error { return nil },
		}
	}

	// LIKE 分支：這是唯一會觸發 template 的 action（payload_filter action=LIKE）。
	t.Run("like", func(t *testing.T) {
		pub := &mockContentEventPublisher{}
		uc := NewCommentUseCase(newRepo(false), &mockPostRepo{}, &mockBuyerClient{}, seller, pub, func() int64 { return 99999 })
		if _, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: likerID}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pub.commentLikeChangedCalls) != 1 {
			t.Fatalf("commentLikeChangedCalls = %d, want 1", len(pub.commentLikeChangedCalls))
		}
		got := pub.commentLikeChangedCalls[0]
		if got.commentAuthorID != authorAccountID {
			t.Errorf("commentAuthorID = %d, want resolved account_id %d, not raw store_id surrogate %d (comment.like_changed template 是 target_user_type='seller'，賣家收件匣以 account_id 查)",
				got.commentAuthorID, authorAccountID, authorStoreID)
		}
		// commentAuthorStoreID 必須是未解析的 raw cms_comment.user_id（=authorStoreID），
		// 與已解析的 commentAuthorID 落在不同身分空間；兩者必須不相等，否則就是誤把
		// resolved 值複製進了 raw 欄位。
		if got.commentAuthorStoreID != authorStoreID {
			t.Errorf("commentAuthorStoreID = %d, want raw store_id %d (must not be the resolved value)", got.commentAuthorStoreID, authorStoreID)
		}
		if got.commentAuthorStoreID == got.commentAuthorID {
			t.Errorf("commentAuthorStoreID (%d) must differ from resolved commentAuthorID (%d)", got.commentAuthorStoreID, got.commentAuthorID)
		}
		// userId（按讚者）刻意不解析：buyer App 的 SSE handler 拿它跟自己的
		// surrogate 比對判斷 fromMe（comments_bottom_sheet.dart _onCommentLikeChanged）。
		if got.userID != likerID {
			t.Errorf("userID = %d, want %d unchanged (SSE fromMe 判斷依賴它維持 surrogate 空間)", got.userID, likerID)
		}
		// commentAuthorType 是 template payload_filter 的分流鍵：漏發或發錯，
		// filter 永不命中 ⇒ 賣家作者的按讚通知整條靜默消失（#1626）。
		if got.commentAuthorType != "seller" {
			t.Errorf("commentAuthorType = %q, want \"seller\"（payload_filter 分流鍵，錯了整則通知不發）", got.commentAuthorType)
		}
	})

	// UNLIKE 是獨立的 publish 呼叫點，同樣要帶解析後的值，否則哪天 template
	// 的 action filter 一放寬就會退回舊行為。
	t.Run("unlike", func(t *testing.T) {
		pub := &mockContentEventPublisher{}
		uc := NewCommentUseCase(newRepo(true), &mockPostRepo{}, &mockBuyerClient{}, seller, pub, func() int64 { return 99999 })
		if _, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: likerID}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pub.commentLikeChangedCalls) != 1 {
			t.Fatalf("commentLikeChangedCalls = %d, want 1", len(pub.commentLikeChangedCalls))
		}
		if got := pub.commentLikeChangedCalls[0]; got.commentAuthorID != authorAccountID {
			t.Errorf("commentAuthorID = %d, want resolved account_id %d", got.commentAuthorID, authorAccountID)
		} else if got.commentAuthorType != "seller" {
			t.Errorf("commentAuthorType = %q, want \"seller\"", got.commentAuthorType)
		} else if got.commentAuthorStoreID != authorStoreID {
			t.Errorf("commentAuthorStoreID = %d, want raw store_id %d (must not be the resolved value)", got.commentAuthorStoreID, authorStoreID)
		}
	})
}

func TestCommentUC_ToggleLike_BuyerAuthor_KeepsBuyerIDUnresolved(t *testing.T) {
	const authorBuyerID int64 = 7777
	const likerID int64 = 100

	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, UserID: authorBuyerID, LikeCount: 4, Role: int16(domain.ActorKindBuyer)}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
			return nil, errNotFoundMock
		},
		createLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error { return nil },
	}
	// 對任何 id 都回不同值；買家分支誤走解析就會被抓到。
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, _ int64) int64 { return 4242 },
	}
	pub := &mockContentEventPublisher{}
	uc := NewCommentUseCase(commentRepo, &mockPostRepo{}, &mockBuyerClient{}, seller, pub, func() int64 { return 99999 })

	if _, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: likerID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentLikeChangedCalls) != 1 {
		t.Fatalf("commentLikeChangedCalls = %d, want 1", len(pub.commentLikeChangedCalls))
	}
	got := pub.commentLikeChangedCalls[0]
	if got.commentAuthorID != authorBuyerID {
		t.Errorf("commentAuthorID = %d, want buyer_id %d unresolved (買家留言的身分空間今天沒有對應 template；解析成 account_id 會誤投進雙重身分使用者的賣家收件匣)",
			got.commentAuthorID, authorBuyerID)
	}
	// 而 commentAuthorType 必須誠實回報 "buyer"：唯一的 template 是
	// target_user_type='seller' + payload_filter {"commentAuthorType":"seller"}，
	// 靠這個值把買家作者的事件擋在 template 之外。發錯成 "seller" 就會退回
	// #1626 的原狀——buyer_id 被寫進 user_type='seller' 的列，永遠沒有人讀得到。
	if got.commentAuthorType != "buyer" {
		t.Errorf("commentAuthorType = %q, want \"buyer\"（發錯就會讓 buyer_id 被寫進賣家收件匣，那一列永遠讀不到）", got.commentAuthorType)
	}
}

// TestCommentUC_ToggleLike_LikerPrincipalIDMatchesAuthorIdentitySpace 守住
// likerPrincipalId 的唯一存在理由：它必須與 commentAuthorId 落在**同一個身分空間**，
// notification-service 的自我通知抑制才比得中
// （template_handler.go actorFieldByEventType["comment.like_changed"]）。
//
// 身分由貼文歸屬推導：留言所在的貼文屬於呼叫者的店 → 店家的讚，其餘一律本人（buyer）。
// 每個 subtest 同時斷言 userId／likerKind 是按讚者的 (kind, id)，SSE 依它算 isMine。
func TestCommentUC_ToggleLike_LikerPrincipalIDMatchesAuthorIdentitySpace(t *testing.T) {
	const postStoreID int64 = 8888 // 貼文所屬店
	const postAccountID int64 = 3001
	const authorBuyerID int64 = 7777
	const callerBuyerID int64 = 100
	const otherStoreID int64 = 5555 // 別家的店
	const otherAccountID int64 = 6001

	// 只有真的是 store_id 的 id 會被解析；其餘原樣回傳 —— 對齊 seller-service
	// ResolvePrincipals 對未知 id 的 echo-back 行為。
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			switch sid {
			case postStoreID:
				return postAccountID
			case otherStoreID:
				return otherAccountID
			default:
				return sid
			}
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: postStoreID}, nil
		},
	}
	newRepo := func(authorID int64, role int16) *mockCommentRepo {
		return &mockCommentRepo{
			getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
				return &gen.CmsComment{ID: id, PostID: 10, UserID: authorID, LikeCount: 4, Role: role}, nil
			},
			getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
				return nil, errNotFoundMock
			},
			createLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error { return nil },
		}
	}

	tests := []struct {
		name                 string
		authorID             int64
		authorRole           int16
		caller               Caller
		wantLikerID          int64
		wantLikerKind        int32
		wantLikerPrincipalID int64
		wantCommentAuthorID  int64
		wantSelfSkip         bool // likerPrincipalId == commentAuthorId ⇒ consumer 會 skip
	}{
		{
			name:                 "店家按自己的留言 → 兩值皆 account_id，比得中",
			authorID:             postStoreID,
			authorRole:           domain.ActorKindStore,
			caller:               Caller{BuyerID: callerBuyerID, StoreID: postStoreID},
			wantLikerID:          postStoreID,
			wantLikerKind:        2,
			wantLikerPrincipalID: postAccountID,
			wantCommentAuthorID:  postAccountID,
			wantSelfSkip:         true,
		},
		{
			name:                 "買家按自己的留言 → 兩值皆 buyer_id，比得中",
			authorID:             authorBuyerID,
			authorRole:           domain.ActorKindBuyer,
			caller:               Caller{BuyerID: authorBuyerID},
			wantLikerID:          authorBuyerID,
			wantLikerKind:        1,
			wantLikerPrincipalID: authorBuyerID,
			wantCommentAuthorID:  authorBuyerID,
			wantSelfSkip:         true,
		},
		{
			name:                 "店主按自己貼文下買家的留言 → 店家身分，解析成 account_id 且不等於作者",
			authorID:             authorBuyerID,
			authorRole:           domain.ActorKindBuyer,
			caller:               Caller{BuyerID: callerBuyerID, StoreID: postStoreID},
			wantLikerID:          postStoreID,
			wantLikerKind:        2,
			wantLikerPrincipalID: postAccountID,
			wantCommentAuthorID:  authorBuyerID,
			wantSelfSkip:         false,
		},
		{
			name:                 "買家按店家的留言 → buyer_id 原樣，不誤觸 skip",
			authorID:             postStoreID,
			authorRole:           domain.ActorKindStore,
			caller:               Caller{BuyerID: callerBuyerID},
			wantLikerID:          callerBuyerID,
			wantLikerKind:        1,
			wantLikerPrincipalID: callerBuyerID,
			wantCommentAuthorID:  postAccountID,
			wantSelfSkip:         false,
		},
		{
			// 別家的店主按讚別人貼文下的留言：店家身分只用在自己的內容，這裡是本人（buyer）。
			name:                 "別家的店主按讚 → 本人身分，buyer_id 原樣",
			authorID:             authorBuyerID,
			authorRole:           domain.ActorKindBuyer,
			caller:               Caller{BuyerID: callerBuyerID, StoreID: otherStoreID},
			wantLikerID:          callerBuyerID,
			wantLikerKind:        1,
			wantLikerPrincipalID: callerBuyerID,
			wantCommentAuthorID:  authorBuyerID,
			wantSelfSkip:         false,
		},
		{
			// buyer N 與 store N 同號：買家按店家的留言，不得被當成作者自己。
			name:                 "買家編號與店家編號相同 → 不是自己",
			authorID:             postStoreID,
			authorRole:           domain.ActorKindStore,
			caller:               Caller{BuyerID: postStoreID},
			wantLikerID:          postStoreID,
			wantLikerKind:        1,
			wantLikerPrincipalID: postStoreID,
			wantCommentAuthorID:  postAccountID,
			wantSelfSkip:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub := &mockContentEventPublisher{}
			uc := NewCommentUseCase(newRepo(tt.authorID, tt.authorRole), postRepo, &mockBuyerClient{}, seller, pub, func() int64 { return 99999 })
			if _, err := uc.ToggleLike(context.Background(), 1, tt.caller); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(pub.commentLikeChangedCalls) != 1 {
				t.Fatalf("commentLikeChangedCalls = %d, want 1", len(pub.commentLikeChangedCalls))
			}
			got := pub.commentLikeChangedCalls[0]
			if got.likerPrincipalID != tt.wantLikerPrincipalID {
				t.Errorf("likerPrincipalId = %d, want %d", got.likerPrincipalID, tt.wantLikerPrincipalID)
			}
			if got.commentAuthorID != tt.wantCommentAuthorID {
				t.Errorf("commentAuthorId = %d, want %d", got.commentAuthorID, tt.wantCommentAuthorID)
			}
			// 這才是消費端真正執行的判斷；上面兩條是它的組成。
			if selfSkip := got.likerPrincipalID == got.commentAuthorID; selfSkip != tt.wantSelfSkip {
				t.Errorf("likerPrincipalId(%d) == commentAuthorId(%d) 得到 %v, want %v —— 這是 notification-service self-skip 的判斷式",
					got.likerPrincipalID, got.commentAuthorID, selfSkip, tt.wantSelfSkip)
			}
			if got.userID != tt.wantLikerID || got.likerKind != tt.wantLikerKind {
				t.Errorf("(userId, likerKind) = (%d, %d), want (%d, %d)", got.userID, got.likerKind, tt.wantLikerID, tt.wantLikerKind)
			}
		})
	}
}

// TestCommentUC_ToggleLike_UnlikeAlsoCarriesLikerPrincipalID：UNLIKE 是獨立的
// publish 呼叫點，漏帶新欄位不會編譯錯（同型別、位置對得上就過），只會在 template
// 的 action filter 哪天放寬時退回自我通知。
func TestCommentUC_ToggleLike_UnlikeAlsoCarriesLikerPrincipalID(t *testing.T) {
	const authorStoreID int64 = 8888
	const authorAccountID int64 = 3001

	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, UserID: authorStoreID, LikeCount: 4, Role: int16(domain.ActorKindStore)}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
			return &gen.CmsCommentLike{}, nil // 已按過 → 走 UNLIKE 分支
		},
		deleteLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error { return nil },
	}
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			if sid == authorStoreID {
				return authorAccountID
			}
			return sid
		},
	}
	pub := &mockContentEventPublisher{}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: authorStoreID}, nil
		},
	}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, seller, pub, func() int64 { return 99999 })

	if _, err := uc.ToggleLike(context.Background(), 1, Caller{StoreID: authorStoreID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentLikeChangedCalls) != 1 {
		t.Fatalf("commentLikeChangedCalls = %d, want 1", len(pub.commentLikeChangedCalls))
	}
	got := pub.commentLikeChangedCalls[0]
	if got.action != "UNLIKE" {
		t.Fatalf("action = %q, want UNLIKE", got.action)
	}
	if got.likerPrincipalID != authorAccountID {
		t.Errorf("likerPrincipalId = %d, want %d（UNLIKE 分支漏帶不會編譯錯，只會靜默退化）", got.likerPrincipalID, authorAccountID)
	}
}

// TestCommentUC_CreateComment_CommenterPrincipalIDMatchesPostAuthorIdentitySpace 守住
// commenterPrincipalId 的唯一存在理由：它必須與 postAuthorId 落在**同一個身分空間**，
// notification-service 的自我通知抑制才比得中
// （template_handler.go actorFieldByEventType["comment.created"]）。
//
// 修復前 self-skip 比的是 commenterId（留言者的 active-role surrogate，賣家＝store_id），
// 而 postAuthorId 一律經 ResolvePrincipal ⇒ re-key 過的賣家在自己貼文下留言時兩值不等
// ⇒ 永遠比不中 ⇒ 收到自我通知＋FCM 推播。
//
// 每個 subtest 都同時斷言 commenterId **維持未解析**：那是 gateway SSE 判斷 fromMe 用的，
// 順手把它一起解析掉會打壞前端。
func TestCommentUC_CreateComment_CommenterPrincipalIDMatchesPostAuthorIdentitySpace(t *testing.T) {
	const postStoreID int64 = 8888   // 貼文所屬店（＝賣家的 store surrogate）
	const postAccountID int64 = 3001 // 其登入 principal
	const otherStoreID int64 = 5555  // 另一個賣家
	const otherAccountID int64 = 6001
	const buyerID int64 = 100

	// 只有真的是 store_id 的 id 會被解析；其餘原樣回傳 —— 對齊 seller-service
	// ResolvePrincipals 對未知 id 的 echo-back 行為。
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			switch sid {
			case postStoreID:
				return postAccountID
			case otherStoreID:
				return otherAccountID
			default:
				return sid
			}
		},
	}
	newRepos := func() (*mockCommentRepo, *mockPostRepo) {
		return &mockCommentRepo{
			createFn: func(_ context.Context, id, postID, userID int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
				return &gen.CmsComment{ID: id, PostID: postID, UserID: userID, ParentID: parentID}, nil
			},
		}, &mockPostRepo{
			getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
				return &gen.CmsPost{ID: id, SellerID: postStoreID}, nil
			},
		}
	}

	tests := []struct {
		name                     string
		caller                   Caller
		wantCommenterID          int64
		wantCommenterPrincipalID int64
		wantSelfSkip             bool // commenterPrincipalId == postAuthorId ⇒ consumer 會 skip
	}{
		{
			name:                     "賣家在自己貼文下留言 → 兩值皆 account_id，比得中",
			caller:                   Caller{BuyerID: buyerID, StoreID: postStoreID},
			wantCommenterID:          postStoreID,
			wantCommenterPrincipalID: postAccountID,
			wantSelfSkip:             true,
		},
		{
			name:                     "買家在賣家貼文下留言 → buyer_id 原樣，不誤觸 skip",
			caller:                   Caller{BuyerID: buyerID},
			wantCommenterID:          buyerID,
			wantCommenterPrincipalID: buyerID,
			wantSelfSkip:             false,
		},
		{
			// 別家的店主在此貼文留言：店家身分只用在自己的內容，這裡是本人（buyer）身分，
			// 通知的留言者側是 buyer_id，不是那家店的 account_id。
			name:                     "別家的店主在此貼文下留言 → 本人身分，buyer_id 原樣",
			caller:                   Caller{BuyerID: buyerID, StoreID: otherStoreID},
			wantCommenterID:          buyerID,
			wantCommenterPrincipalID: buyerID,
			wantSelfSkip:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub := &mockContentEventPublisher{}
			commentRepo, postRepo := newRepos()
			uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, seller, pub, func() int64 { return 99999 })

			if _, err := uc.CreateComment(context.Background(), 10, tt.caller, "留言", 0, 0, "", false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(pub.commentCreatedCalls) != 1 {
				t.Fatalf("commentCreatedCalls = %d, want 1", len(pub.commentCreatedCalls))
			}
			got := pub.commentCreatedCalls[0]
			if got.commenterPrincipalID != tt.wantCommenterPrincipalID {
				t.Errorf("commenterPrincipalId = %d, want %d", got.commenterPrincipalID, tt.wantCommenterPrincipalID)
			}
			if got.postAuthorID != postAccountID {
				t.Errorf("postAuthorId = %d, want resolved account_id %d", got.postAuthorID, postAccountID)
			}
			// 這才是消費端真正執行的判斷；上面兩條是它的組成。
			if selfSkip := got.commenterPrincipalID == got.postAuthorID; selfSkip != tt.wantSelfSkip {
				t.Errorf("commenterPrincipalId(%d) == postAuthorId(%d) 得到 %v, want %v —— 這是 notification-service self-skip 的判斷式",
					got.commenterPrincipalID, got.postAuthorID, selfSkip, tt.wantSelfSkip)
			}
			if got.commenterID != tt.wantCommenterID {
				t.Errorf("commenterId = %d, want %d 未解析（gateway SSE 的 fromMe 判斷依賴它維持 surrogate 空間）", got.commenterID, tt.wantCommenterID)
			}
		})
	}
}

// TestCommentUC_CreateComment_SelfComment_ReusesResolvedPostAuthorID 守住「同源複用」：
// ResolvePrincipal 失敗時會原樣回傳輸入，若貼文作者側與留言者側各解析各的，一成一敗
// 就會讓兩值不等而漏掉 skip。用一個「第二次呼叫就退化成 echo-back」的 resolver 模擬
// 那個一成一敗，若改成解析兩次，本測試會紅。
func TestCommentUC_CreateComment_SelfComment_ReusesResolvedPostAuthorID(t *testing.T) {
	const postStoreID int64 = 8888
	const postAccountID int64 = 3001

	calls := 0
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			calls++
			if calls == 1 && sid == postStoreID {
				return postAccountID
			}
			return sid // 第二次起「解析失敗」，原樣回傳
		},
	}
	commentRepo := &mockCommentRepo{
		createFn: func(_ context.Context, id, postID, userID int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: postID, UserID: userID, ParentID: parentID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: postStoreID}, nil
		},
	}
	pub := &mockContentEventPublisher{}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, seller, pub, func() int64 { return 99999 })

	if _, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: postStoreID}, "自言自語", 0, 0, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentCreatedCalls) != 1 {
		t.Fatalf("commentCreatedCalls = %d, want 1", len(pub.commentCreatedCalls))
	}
	got := pub.commentCreatedCalls[0]
	if got.commenterPrincipalID != got.postAuthorID {
		t.Errorf("commenterPrincipalId(%d) != postAuthorId(%d) —— 自留言必須複用貼文作者側算好的值，各解析各的會在一成一敗時漏掉 skip",
			got.commenterPrincipalID, got.postAuthorID)
	}
}

// TestCommentUC_CreateComment_Reply_ReplierPrincipalIDMatchesTargetIdentitySpace：
// reply 分支的對應條，理由同 commenterPrincipalId。回覆一律只有貼文所屬賣家做得到
// （CreateComment 的 parentID>0 分支要求 storeID == post.SellerID），所以回覆者恆為賣家，
// 目標則可能是賣家（自己先前的留言）或買家。
func TestCommentUC_CreateComment_Reply_ReplierPrincipalIDMatchesTargetIdentitySpace(t *testing.T) {
	const postStoreID int64 = 8888   // 貼文所屬店＝回覆者的 store surrogate
	const postAccountID int64 = 3001 // 其登入 principal
	const targetBuyerID int64 = 7777
	const parentCommentID int64 = 1
	const newCommentID int64 = 999

	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			if sid == postStoreID {
				return postAccountID
			}
			return sid
		},
	}
	newRepos := func(targetUserID int64, targetRole int16) (*mockCommentRepo, *mockPostRepo) {
		return &mockCommentRepo{
			getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
				return &gen.CmsComment{ID: id, PostID: 10, ParentID: 0, UserID: targetUserID, Role: targetRole}, nil
			},
			createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
				return &gen.CmsComment{ID: newCommentID, PostID: postID, UserID: userID, ParentID: parentID}, nil
			},
		}, &mockPostRepo{
			getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
				return &gen.CmsPost{ID: id, SellerID: postStoreID}, nil
			},
		}
	}

	tests := []struct {
		name                   string
		targetUserID           int64
		targetRole             int16
		wantReplyToUserID      int64
		wantReplierPrincipalID int64
		wantSelfSkip           bool
	}{
		{
			// 目標格：修復前 self-skip 比的是 replierId(=store_id 8888) vs
			// replyToUserId(=account_id 3001) ⇒ 比不中 ⇒ 賣家回自己的留言收到自我通知。
			name:                   "賣家回覆自己的留言 → 兩值皆 account_id，比得中",
			targetUserID:           postStoreID,
			targetRole:             domain.ActorKindStore,
			wantReplyToUserID:      postAccountID,
			wantReplierPrincipalID: postAccountID,
			wantSelfSkip:           true,
		},
		{
			name:                   "賣家回覆買家的留言 → 目標維持 buyer_id、回覆者解析成 account_id，不誤觸 skip",
			targetUserID:           targetBuyerID,
			targetRole:             domain.ActorKindBuyer,
			wantReplyToUserID:      targetBuyerID,
			wantReplierPrincipalID: postAccountID,
			wantSelfSkip:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub := &mockContentEventPublisher{}
			commentRepo, postRepo := newRepos(tt.targetUserID, tt.targetRole)
			uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, seller, pub, func() int64 { return newCommentID })

			if _, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: postStoreID}, "回覆", parentCommentID, parentCommentID, "", false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(pub.commentRepliedCalls) != 1 {
				t.Fatalf("commentRepliedCalls = %d, want 1", len(pub.commentRepliedCalls))
			}
			got := pub.commentRepliedCalls[0]
			if got.replyToUserID != tt.wantReplyToUserID {
				t.Errorf("replyToUserId = %d, want %d", got.replyToUserID, tt.wantReplyToUserID)
			}
			if got.replierPrincipalID != tt.wantReplierPrincipalID {
				t.Errorf("replierPrincipalId = %d, want %d", got.replierPrincipalID, tt.wantReplierPrincipalID)
			}
			if selfSkip := got.replierPrincipalID == got.replyToUserID; selfSkip != tt.wantSelfSkip {
				t.Errorf("replierPrincipalId(%d) == replyToUserId(%d) 得到 %v, want %v —— 這是 notification-service self-skip 的判斷式",
					got.replierPrincipalID, got.replyToUserID, selfSkip, tt.wantSelfSkip)
			}
			if got.replierID != postStoreID {
				t.Errorf("replierId = %d, want %d 未解析（gateway SSE 的 fromMe 判斷依賴它維持 surrogate 空間）", got.replierID, postStoreID)
			}
		})
	}
}

// TestCommentUC_CreateComment_SelfReply_ReusesResolvedReplyToUserID：reply 分支的
// 「同源複用」對應條，理由與 TestCommentUC_CreateComment_SelfComment_ReusesResolvedPostAuthorID
// 相同。resolver 第二次起退化成 echo-back，模擬 ResolvePrincipal 一成一敗。
func TestCommentUC_CreateComment_SelfReply_ReusesResolvedReplyToUserID(t *testing.T) {
	const postStoreID int64 = 8888
	const postAccountID int64 = 3001
	const parentCommentID int64 = 1
	const newCommentID int64 = 999

	calls := 0
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			calls++
			if calls == 1 && sid == postStoreID {
				return postAccountID
			}
			return sid // 第二次起「解析失敗」，原樣回傳
		},
	}
	commentRepo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, ParentID: 0, UserID: postStoreID, Role: int16(domain.ActorKindStore)}, nil
		},
		createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: newCommentID, PostID: postID, UserID: userID, ParentID: parentID}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: postStoreID}, nil
		},
	}
	pub := &mockContentEventPublisher{}
	uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, seller, pub, func() int64 { return newCommentID })

	if _, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: postStoreID}, "自問自答", parentCommentID, parentCommentID, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.commentRepliedCalls) != 1 {
		t.Fatalf("commentRepliedCalls = %d, want 1", len(pub.commentRepliedCalls))
	}
	got := pub.commentRepliedCalls[0]
	if got.replierPrincipalID != got.replyToUserID {
		t.Errorf("replierPrincipalId(%d) != replyToUserId(%d) —— 自回覆必須複用被回覆者側算好的值，各解析各的會在一成一敗時漏掉 skip",
			got.replierPrincipalID, got.replyToUserID)
	}
}

// 留言的身分種類不是 1／2（DB 有 CHECK，正常到不了）：不得當成買家處理。
func TestCommentUC_UnknownAuthorKind_NeverTreatedAsBuyer(t *testing.T) {
	const badKind int16 = 9

	t.Run("like fails before any write", func(t *testing.T) {
		created := false
		commentRepo := &mockCommentRepo{
			getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
				return &gen.CmsComment{ID: id, PostID: 10, UserID: 7, Role: badKind}, nil
			},
			getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
				return nil, errNotFoundMock
			},
			createLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error { created = true; return nil },
		}
		pub := &mockContentEventPublisher{}
		uc := NewCommentUseCase(commentRepo, &mockPostRepo{}, &mockBuyerClient{}, &mockSellerClient{}, pub, func() int64 { return 1 })
		if _, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: 100}); err == nil {
			t.Fatal("want error for unknown author kind")
		}
		if created || len(pub.commentLikeChangedCalls) != 0 {
			t.Errorf("created=%v events=%d, want no write and no event", created, len(pub.commentLikeChangedCalls))
		}
	})

	t.Run("reply publishes no notification", func(t *testing.T) {
		const storeID int64 = 50
		commentRepo := &mockCommentRepo{
			getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
				return &gen.CmsComment{ID: id, PostID: 10, UserID: 7, Role: badKind}, nil
			},
			createFn: func(_ context.Context, _, postID, userID int64, _, _, _ string, parentID, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
				return &gen.CmsComment{ID: 999, PostID: postID, UserID: userID, ParentID: parentID}, nil
			},
		}
		postRepo := &mockPostRepo{
			getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
				return &gen.CmsPost{ID: id, SellerID: storeID}, nil
			},
		}
		pub := &mockContentEventPublisher{}
		uc := NewCommentUseCase(commentRepo, postRepo, &mockBuyerClient{}, &mockSellerClient{}, pub, func() int64 { return 999 })
		if _, err := uc.CreateComment(context.Background(), 10, Caller{StoreID: storeID}, "回覆", 1, 1, "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pub.commentRepliedCalls) != 0 {
			t.Errorf("commentRepliedCalls = %d, want 0", len(pub.commentRepliedCalls))
		}
	})
}
