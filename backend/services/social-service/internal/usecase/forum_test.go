package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

func newTestForumUC(repo *mockForumRepo) *ForumUseCase {
	return NewForumUseCase(repo, &mockBuyerClient{}, &mockSellerClient{}, func() int64 { return 99999 })
}

func TestForumUC_ListForum_PageDefaults(t *testing.T) {
	var gotLimit, gotOffset int64
	repo := &mockForumRepo{
		listTopLevelFn: func(_ context.Context, _ int64, limit, offset int64) ([]*gen.CmsSellerForum, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		countTopLevelFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
	}
	uc := newTestForumUC(repo)
	_, _, err := uc.ListForum(context.Background(), 1, 0, 0)
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

func TestForumUC_CreateComment_Success(t *testing.T) {
	var createCalled bool
	repo := &mockForumRepo{
		createFn: func(_ context.Context, _, sellerID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			createCalled = true
			return &gen.CmsSellerForum{ID: 1, SellerID: sellerID, UserID: userID}, nil
		},
	}
	uc := newTestForumUC(repo)
	entry, err := uc.CreateComment(context.Background(), 10, Caller{BuyerID: 100}, "hello seller", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Error("Create was not called")
	}
	if entry.UserID != 100 {
		t.Errorf("UserID = %d, want 100", entry.UserID)
	}
}

func TestForumUC_ReplyComment_NotFound(t *testing.T) {
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsSellerForum, error) {
			return nil, errors.New("no rows")
		},
	}
	uc := newTestForumUC(repo)
	_, err := uc.ReplyComment(context.Background(), 999, Caller{BuyerID: 100}, "reply", false)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestForumUC_ReplyComment_Success(t *testing.T) {
	var incrCalled bool
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{
				ID:       id,
				SellerID: 10,
				UserID:   50,
				ParentID: 0,
				UserName: testStringPointer("Seller"),
			}, nil
		},
		createFn: func(_ context.Context, _, sellerID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: 2, SellerID: sellerID, UserID: userID, ParentID: 1}, nil
		},
		incrReplyFn: func(_ context.Context, _ int64) error {
			incrCalled = true
			return nil
		},
	}
	uc := newTestForumUC(repo)
	entry, err := uc.ReplyComment(context.Background(), 1, Caller{BuyerID: 100}, "my reply", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.UserID != 100 {
		t.Errorf("UserID = %d, want 100", entry.UserID)
	}
	if !incrCalled {
		t.Error("IncrReplyCount was not called")
	}
}

func TestForumUC_DeleteComment_NotFound(t *testing.T) {
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsSellerForum, error) {
			return nil, errors.New("no rows")
		},
	}
	uc := newTestForumUC(repo)
	err := uc.DeleteComment(context.Background(), 999, Caller{BuyerID: 100})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestForumUC_DeleteComment_Forbidden(t *testing.T) {
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: id, UserID: 50, SellerID: 10}, nil
		},
	}
	uc := newTestForumUC(repo)
	// requester 999 is neither owner (50) nor seller (10)
	err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 999})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("error = %v, want ErrForbidden", err)
	}
}

func TestForumUC_DeleteComment_OwnerSuccess(t *testing.T) {
	var softDeleteCalled bool
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: id, UserID: 100, SellerID: 10, ParentID: 0, Role: domain.ActorKindBuyer}, nil
		},
		softDeleteFn: func(_ context.Context, _ int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	uc := newTestForumUC(repo)
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 100}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !softDeleteCalled {
		t.Error("SoftDelete was not called")
	}
}

func TestForumUC_DeleteComment_StoreOwnerSuccess(t *testing.T) {
	var softDeleteCalled bool
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: id, UserID: 50, SellerID: 777, ParentID: 0}, nil
		},
		softDeleteFn: func(_ context.Context, _ int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	uc := newTestForumUC(repo)
	// requester account 99 is not the author, but IS the current store 777
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 99, StoreID: 777}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !softDeleteCalled {
		t.Error("SoftDelete was not called")
	}
}

func TestForumUC_DeleteComment_StaleIncarnationForbidden(t *testing.T) {
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsSellerForum, error) {
			// entry belongs to OLD stint 777; author is buyer 50.
			return &gen.CmsSellerForum{ID: id, UserID: 50, SellerID: 777}, nil
		},
	}
	uc := newTestForumUC(repo)
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 777, StoreID: 888}); !errors.Is(err, ErrForbidden) {
		t.Errorf("error = %v, want ErrForbidden (new incarnation must not moderate the old store's forum)", err)
	}
}

// ==================== 賣家停權 self-vs-other 細分（46 §4.4 / 06 §四.10） ====================

// TestForumUC_CreateComment_SelfSuspended_Blocked: the caller is a suspended
// seller posting in THEIR OWN forum (self) — must be blocked.
func TestForumUC_CreateComment_SelfSuspended_Blocked(t *testing.T) {
	repo := &mockForumRepo{
		createFn: func(_ context.Context, _, sellerID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			t.Fatal("Create must not be called when self-forum comment is blocked by suspension")
			return nil, nil
		},
	}
	uc := newTestForumUC(repo)
	_, err := uc.CreateComment(context.Background(), 100, Caller{BuyerID: 100, StoreID: 100}, "hello", true)
	if !errors.Is(err, ErrSellerSuspendedOperationBlocked) {
		t.Errorf("error = %v, want ErrSellerSuspendedOperationBlocked", err)
	}
}

// TestForumUC_CreateComment_OtherSuspended_Allowed: a suspended seller (or
// buyer) posting in SOMEONE ELSE's forum is never gated.
func TestForumUC_CreateComment_OtherSuspended_Allowed(t *testing.T) {
	var createCalled bool
	repo := &mockForumRepo{
		createFn: func(_ context.Context, _, sellerID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			createCalled = true
			return &gen.CmsSellerForum{ID: 1, SellerID: sellerID, UserID: userID}, nil
		},
	}
	uc := newTestForumUC(repo)
	_, err := uc.CreateComment(context.Background(), 50, Caller{BuyerID: 100, StoreID: 100}, "hello", true) // forum owner 50, caller store 100
	if err != nil {
		t.Fatalf("unexpected error for other-forum comment by a suspended seller: %v", err)
	}
	if !createCalled {
		t.Error("Create should be called — commenting on someone else's forum is never gated")
	}
}

// TestForumUC_ReplyComment_SelfSuspended_Blocked: the caller is a suspended
// seller replying in THEIR OWN forum thread (self) — must be blocked.
func TestForumUC_ReplyComment_SelfSuspended_Blocked(t *testing.T) {
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: id, SellerID: 100, ParentID: 0}, nil
		},
		createFn: func(_ context.Context, _, sellerID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			t.Fatal("Create must not be called when self-forum reply is blocked by suspension")
			return nil, nil
		},
	}
	uc := newTestForumUC(repo)
	_, err := uc.ReplyComment(context.Background(), 1, Caller{BuyerID: 100, StoreID: 100}, "reply", true)
	if !errors.Is(err, ErrSellerSuspendedOperationBlocked) {
		t.Errorf("error = %v, want ErrSellerSuspendedOperationBlocked", err)
	}
}

// TestForumUC_ReplyComment_OtherSuspended_Allowed: a buyer (or the suspended
// seller acting as a mere participant) replying in SOMEONE ELSE's forum thread
// is never gated.
func TestForumUC_ReplyComment_OtherSuspended_Allowed(t *testing.T) {
	var createCalled bool
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: id, SellerID: 50, ParentID: 0}, nil // thread owned by 50
		},
		createFn: func(_ context.Context, _, sellerID, userID int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			createCalled = true
			return &gen.CmsSellerForum{ID: 1, SellerID: sellerID, UserID: userID}, nil
		},
	}
	uc := newTestForumUC(repo)
	_, err := uc.ReplyComment(context.Background(), 1, Caller{BuyerID: 100, StoreID: 100}, "reply", true) // caller store 100, thread owner 50
	if err != nil {
		t.Fatalf("unexpected error for other-forum reply by a suspended seller: %v", err)
	}
	if !createCalled {
		t.Error("Create should be called — replying in someone else's forum is never gated")
	}
}

func TestForumUC_CreateComment_SelfSuspend_StaleSurrogateNotBlocked(t *testing.T) {
	const userID int64 = 1001
	const staleSurrogate int64 = 1001
	const currentStoreID int64 = 9999
	var createCalled bool
	repo := &mockForumRepo{
		createFn: func(_ context.Context, _, sellerID, uid int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			createCalled = true
			return &gen.CmsSellerForum{ID: 1, SellerID: sellerID, UserID: uid}, nil
		},
	}
	uc := newTestForumUC(repo)
	_, err := uc.CreateComment(context.Background(), staleSurrogate, Caller{BuyerID: userID, StoreID: currentStoreID}, "hi", true)
	if err != nil {
		t.Fatalf("must NOT be blocked: current store (9999) != forum surrogate (1001): %v", err)
	}
	if !createCalled {
		t.Error("Create should be called — posting in the old store's forum is not a suspended self-action")
	}
}

// Current：討論區 surrogate == 現任 store 2002 → 真正的 self，停權時擋。
func TestForumUC_CreateComment_SelfSuspend_CurrentSurrogateBlocked(t *testing.T) {
	const userID int64 = 1001
	const currentStoreID int64 = 2002
	repo := &mockForumRepo{
		createFn: func(_ context.Context, _, _, _ int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			t.Fatal("Create must not be called when self-forum comment is blocked by suspension")
			return nil, nil
		},
	}
	uc := newTestForumUC(repo)
	_, err := uc.CreateComment(context.Background(), currentStoreID, Caller{BuyerID: userID, StoreID: currentStoreID}, "hi", true)
	if !errors.Is(err, ErrSellerSuspendedOperationBlocked) {
		t.Errorf("error = %v, want ErrSellerSuspendedOperationBlocked (self-forum while suspended)", err)
	}
}

// site 5：ForumUseCase.ReplyComment self-suspend。
// Stale：thread 屬舊 store 1001（parent.SellerID == userID），現任 store 9999。
// 舊碼 parent.SellerID(1001)==userID(1001) → 擋（bug）；新碼不擋。
func TestForumUC_ReplyComment_SelfSuspend_StaleSurrogateNotBlocked(t *testing.T) {
	const userID int64 = 1001
	const staleSurrogate int64 = 1001
	const currentStoreID int64 = 9999
	var createCalled bool
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: id, SellerID: staleSurrogate, ParentID: 0}, nil
		},
		createFn: func(_ context.Context, _, sellerID, uid int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			createCalled = true
			return &gen.CmsSellerForum{ID: 2, SellerID: sellerID, UserID: uid}, nil
		},
	}
	uc := newTestForumUC(repo)
	_, err := uc.ReplyComment(context.Background(), 1, Caller{BuyerID: userID, StoreID: currentStoreID}, "reply", true)
	if err != nil {
		t.Fatalf("must NOT be blocked: current store (9999) != thread surrogate (1001): %v", err)
	}
	if !createCalled {
		t.Error("Create should be called — replying in the old store's thread is not a suspended self-action")
	}
}

// Current：thread surrogate == 現任 store 2002 → 真正的 self，停權時擋。
func TestForumUC_ReplyComment_SelfSuspend_CurrentSurrogateBlocked(t *testing.T) {
	const userID int64 = 1001
	const currentStoreID int64 = 2002
	repo := &mockForumRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: id, SellerID: currentStoreID, ParentID: 0}, nil
		},
		createFn: func(_ context.Context, _, _, _ int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsSellerForum, error) {
			t.Fatal("Create must not be called when self-forum reply is blocked by suspension")
			return nil, nil
		},
	}
	uc := newTestForumUC(repo)
	_, err := uc.ReplyComment(context.Background(), 1, Caller{BuyerID: userID, StoreID: currentStoreID}, "reply", true)
	if !errors.Is(err, ErrSellerSuspendedOperationBlocked) {
		t.Errorf("error = %v, want ErrSellerSuspendedOperationBlocked (self-thread while suspended)", err)
	}
}
