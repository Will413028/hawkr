package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// buyer 7 與 store 7 是兩個不同的人：編輯、刪除一律比 (kind, id)。
const sameNumber int64 = 7

func commentOwnedBy(kind int16, id int64) *mockCommentRepo {
	return &mockCommentRepo{
		getByIDFn: func(_ context.Context, cid int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: cid, PostID: 10, UserID: id, Role: kind}, nil
		},
		updateContentFn: func(_ context.Context, cid int64, content string) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: cid, PostID: 10, UserID: id, Role: kind, Content: &content}, nil
		},
		softDeleteFn: func(_ context.Context, _ int64) error { return nil },
	}
}

func postOwnedBy(storeID int64) *mockPostRepo {
	return &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: storeID}, nil
		},
	}
}

func TestComment_EditDelete_SameNumberDifferentKind(t *testing.T) {
	tests := []struct {
		name      string
		authorKnd int16
		caller    Caller
		wantOK    bool
	}{
		{"buyer 7 不能動 store 7 的留言", domain.ActorKindStore, Caller{BuyerID: sameNumber}, false},
		{"store 7 不能動 buyer 7 的留言", domain.ActorKindBuyer, Caller{StoreID: sameNumber}, false},
		{"buyer 7 動自己的留言", domain.ActorKindBuyer, Caller{BuyerID: sameNumber}, true},
		{"store 7 動自己的留言", domain.ActorKindStore, Caller{StoreID: sameNumber}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 貼文屬於店 999，不是呼叫者的店：只比留言者身分。
			uc := newTestCommentUC(commentOwnedBy(tt.authorKnd, sameNumber), postOwnedBy(999))
			_, updErr := uc.UpdateComment(context.Background(), 1, tt.caller, "x")
			delErr := uc.DeleteComment(context.Background(), 1, tt.caller)
			if tt.wantOK {
				if updErr != nil || delErr != nil {
					t.Fatalf("update=%v delete=%v, want both nil", updErr, delErr)
				}
				return
			}
			if !errors.Is(updErr, ErrForbidden) || !errors.Is(delErr, ErrForbidden) {
				t.Fatalf("update=%v delete=%v, want both ErrForbidden", updErr, delErr)
			}
		})
	}
}

func TestComment_Delete_PostOwnerStoreCanDeleteOthers(t *testing.T) {
	uc := newTestCommentUC(commentOwnedBy(domain.ActorKindBuyer, 50), postOwnedBy(sameNumber))
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 1, StoreID: sameNumber}); err != nil {
		t.Fatalf("post owner store delete: %v", err)
	}
	// 同號的買家不是貼文的店。
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: sameNumber}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("buyer with the same number as the post store: %v, want ErrForbidden", err)
	}
}

func forumOwnedBy(kind int16, id, forumStoreID int64) *mockForumRepo {
	return &mockForumRepo{
		getByIDFn: func(_ context.Context, eid int64) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: eid, SellerID: forumStoreID, UserID: id, Role: kind}, nil
		},
		updateContentFn: func(_ context.Context, eid int64, content string) (*gen.CmsSellerForum, error) {
			return &gen.CmsSellerForum{ID: eid, SellerID: forumStoreID, UserID: id, Role: kind}, nil
		},
		softDeleteFn: func(_ context.Context, _ int64) error { return nil },
	}
}

func TestForum_EditDelete_SameNumberDifferentKind(t *testing.T) {
	tests := []struct {
		name      string
		authorKnd int16
		caller    Caller
		wantOK    bool
	}{
		{"buyer 7 不能動 store 7 的留言", domain.ActorKindStore, Caller{BuyerID: sameNumber}, false},
		{"store 7 不能動 buyer 7 的留言", domain.ActorKindBuyer, Caller{StoreID: sameNumber}, false},
		{"buyer 7 動自己的留言", domain.ActorKindBuyer, Caller{BuyerID: sameNumber}, true},
		{"store 7 動自己的留言", domain.ActorKindStore, Caller{StoreID: sameNumber}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := newTestForumUC(forumOwnedBy(tt.authorKnd, sameNumber, 999))
			_, updErr := uc.UpdateComment(context.Background(), 1, tt.caller, "x")
			delErr := uc.DeleteComment(context.Background(), 1, tt.caller)
			if tt.wantOK {
				if updErr != nil || delErr != nil {
					t.Fatalf("update=%v delete=%v, want both nil", updErr, delErr)
				}
				return
			}
			if !errors.Is(updErr, ErrForbidden) || !errors.Is(delErr, ErrForbidden) {
				t.Fatalf("update=%v delete=%v, want both ErrForbidden", updErr, delErr)
			}
		})
	}
}

func TestForum_Delete_ForumOwnerStoreCanDeleteOthers(t *testing.T) {
	uc := newTestForumUC(forumOwnedBy(domain.ActorKindBuyer, 50, sameNumber))
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: 1, StoreID: sameNumber}); err != nil {
		t.Fatalf("forum owner store delete: %v", err)
	}
	if err := uc.DeleteComment(context.Background(), 1, Caller{BuyerID: sameNumber}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("buyer with the same number as the forum store: %v, want ErrForbidden", err)
	}
}

// 互動身分由資源歸屬推導：自己店的貼文 → 店家；別人的貼文 → 本人；停權時不退回本人。
func TestComment_Create_DerivesActorFromPostOwnership(t *testing.T) {
	const ownStore, otherStore, buyer int64 = 50, 60, 9
	type created struct {
		userID int64
		role   int16
	}
	run := func(caller Caller, postStore int64, suspended bool) (created, error) {
		var got created
		repo := &mockCommentRepo{
			createFn: func(_ context.Context, id, postID, userID int64, _, _, _ string, _, _ int64, _ string, role int16) (*gen.CmsComment, error) {
				got = created{userID: userID, role: role}
				return &gen.CmsComment{ID: id, PostID: postID, UserID: userID, Role: role}, nil
			},
		}
		uc := newTestCommentUC(repo, postOwnedBy(postStore))
		_, err := uc.CreateComment(context.Background(), 10, caller, "hi", 0, 0, "", suspended)
		return got, err
	}

	t.Run("自己店的貼文 → 店家身分", func(t *testing.T) {
		got, err := run(Caller{BuyerID: buyer, StoreID: ownStore}, ownStore, false)
		if err != nil || got != (created{ownStore, domain.ActorKindStore}) {
			t.Fatalf("got %+v err %v, want store identity", got, err)
		}
	})
	t.Run("別人的貼文 → 本人身分，不論有沒有店", func(t *testing.T) {
		got, err := run(Caller{BuyerID: buyer, StoreID: ownStore}, otherStore, false)
		if err != nil || got != (created{buyer, domain.ActorKindBuyer}) {
			t.Fatalf("got %+v err %v, want buyer identity", got, err)
		}
	})
	t.Run("停權的店在自己的貼文 → 擋，不退回本人", func(t *testing.T) {
		got, err := run(Caller{BuyerID: buyer, StoreID: ownStore}, ownStore, true)
		if !errors.Is(err, ErrSellerSuspendedOperationBlocked) || got != (created{}) {
			t.Fatalf("got %+v err %v, want blocked and nothing written", got, err)
		}
	})
	t.Run("停權的店在別人的貼文 → 本人身分照常", func(t *testing.T) {
		got, err := run(Caller{BuyerID: buyer, StoreID: ownStore}, otherStore, true)
		if err != nil || got != (created{buyer, domain.ActorKindBuyer}) {
			t.Fatalf("got %+v err %v, want buyer identity", got, err)
		}
	})
	t.Run("沒有買家身分且不是貼文的店 → forbidden", func(t *testing.T) {
		_, err := run(Caller{StoreID: ownStore}, otherStore, false)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("err = %v, want ErrForbidden", err)
		}
	})
	t.Run("兩個身分都沒有 → forbidden", func(t *testing.T) {
		_, err := run(Caller{}, otherStore, false)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("err = %v, want ErrForbidden", err)
		}
	})
}

func TestForum_Create_DerivesActorFromForumOwnership(t *testing.T) {
	const ownStore, otherStore, buyer int64 = 50, 60, 9
	var gotUser int64
	var gotRole int16
	repo := &mockForumRepo{
		createFn: func(_ context.Context, id, sellerID, userID int64, _, _, _ string, _, _ int64, _ string, role int16) (*gen.CmsSellerForum, error) {
			gotUser, gotRole = userID, role
			return &gen.CmsSellerForum{ID: id, SellerID: sellerID, UserID: userID, Role: role}, nil
		},
	}
	uc := newTestForumUC(repo)
	caller := Caller{BuyerID: buyer, StoreID: ownStore}

	if _, err := uc.CreateComment(context.Background(), ownStore, caller, "hi", false); err != nil || gotUser != ownStore || gotRole != domain.ActorKindStore {
		t.Fatalf("own forum: user=%d role=%d err=%v, want store identity", gotUser, gotRole, err)
	}
	if _, err := uc.CreateComment(context.Background(), otherStore, caller, "hi", false); err != nil || gotUser != buyer || gotRole != domain.ActorKindBuyer {
		t.Fatalf("other forum: user=%d role=%d err=%v, want buyer identity", gotUser, gotRole, err)
	}
	gotUser = 0
	if _, err := uc.CreateComment(context.Background(), ownStore, caller, "hi", true); !errors.Is(err, ErrSellerSuspendedOperationBlocked) || gotUser != 0 {
		t.Fatalf("suspended own forum: err=%v user=%d, want blocked, nothing written", err, gotUser)
	}
	if _, err := uc.CreateComment(context.Background(), otherStore, Caller{StoreID: ownStore}, "hi", false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("no buyer identity on another store's forum: %v, want ErrForbidden", err)
	}
}

// 讚以 (kind, id) 為鍵：buyer 7 與 store 7 各按各的、互不覆蓋。
func TestComment_Like_SameNumberDifferentKindAreIndependent(t *testing.T) {
	const ownStore int64 = sameNumber
	likes := map[[2]int64]bool{}
	repo := &mockCommentRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsComment, error) {
			return &gen.CmsComment{ID: id, PostID: 10, UserID: 500, Role: domain.ActorKindBuyer}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, kind int16, uid int64) (*gen.CmsCommentLike, error) {
			if likes[[2]int64{int64(kind), uid}] {
				return &gen.CmsCommentLike{}, nil
			}
			return nil, errNotFoundMock
		},
		createLikeFn: func(_ context.Context, _ int64, kind int16, uid int64) error {
			likes[[2]int64{int64(kind), uid}] = true
			return nil
		},
		deleteLikeFn: func(_ context.Context, _ int64, kind int16, uid int64) error {
			delete(likes, [2]int64{int64(kind), uid})
			return nil
		},
	}
	uc := NewCommentUseCase(repo, postOwnedBy(ownStore), &mockBuyerClient{}, &mockSellerClient{}, nil, func() int64 { return 1 })

	// 貼文屬於 store 7：買家 7（沒有那家店）按讚是買家身分；store 7 的店主按讚是店家身分。
	if liked, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: sameNumber}); err != nil || !liked {
		t.Fatalf("buyer 7 like: liked=%v err=%v", liked, err)
	}
	if liked, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: 1, StoreID: ownStore}); err != nil || !liked {
		t.Fatalf("store 7 like must not be read as buyer 7's existing like: liked=%v err=%v", liked, err)
	}
	if !likes[[2]int64{int64(domain.ActorKindBuyer), sameNumber}] || !likes[[2]int64{int64(domain.ActorKindStore), sameNumber}] {
		t.Fatalf("likes = %v, want both (buyer,7) and (store,7)", likes)
	}
	// 店主取消自己的讚，不影響買家 7 的讚。
	if liked, err := uc.ToggleLike(context.Background(), 1, Caller{BuyerID: 1, StoreID: ownStore}); err != nil || liked {
		t.Fatalf("store 7 unlike: liked=%v err=%v", liked, err)
	}
	if !likes[[2]int64{int64(domain.ActorKindBuyer), sameNumber}] {
		t.Fatal("buyer 7's like was removed by store 7's unlike")
	}

	viewerOwner := uc.ViewerForPost(context.Background(), 10, Caller{BuyerID: 1, StoreID: ownStore})
	if viewerOwner != (Actor{Kind: domain.ActorKindStore, ID: ownStore}) {
		t.Fatalf("viewer on own store's post = %+v, want store identity", viewerOwner)
	}
	viewerBuyer := uc.ViewerForPost(context.Background(), 10, Caller{BuyerID: sameNumber})
	if viewerBuyer != (Actor{Kind: domain.ActorKindBuyer, ID: sameNumber}) {
		t.Fatalf("viewer = %+v, want buyer identity", viewerBuyer)
	}
	if !uc.IsLiked(context.Background(), 1, viewerBuyer) || uc.IsLiked(context.Background(), 1, viewerOwner) {
		t.Fatal("isLiked must follow the viewer's own (kind, id)")
	}
}

// 貼文讚永遠是本人（buyer）身分，店主從任何工作區按讚都一樣；沒有買家身分就拒絕。
func TestPost_Like_AlwaysBuyerIdentity(t *testing.T) {
	var gotKind int16
	var gotUser int64
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 50}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsPostLike, error) {
			return nil, errNotFoundMock
		},
		createLikeFn: func(_ context.Context, _ int64, kind int16, uid int64) error {
			gotKind, gotUser = kind, uid
			return nil
		},
	}
	uc := newTestPostUC(repo, &mockMediaClient{}, &mockSellerClient{})

	// 店主（buyer 9、store 50）讚自己店的貼文：記在 (buyer, 9) 名下。
	if liked, err := uc.ToggleLike(context.Background(), 1, 9, 50); err != nil || !liked {
		t.Fatalf("liked=%v err=%v", liked, err)
	}
	if gotKind != domain.ActorKindBuyer || gotUser != 9 {
		t.Fatalf("like stored as (%d, %d), want (buyer, 9)", gotKind, gotUser)
	}
	if _, err := uc.ToggleLike(context.Background(), 1, 0, 50); !errors.Is(err, ErrForbidden) {
		t.Fatalf("no buyer identity: %v, want ErrForbidden", err)
	}
}

// 未知的 kind 不猜成買家：不查任何服務，名稱留空、principal 為 0。
func TestUnknownActorKind_NeverTreatedAsBuyer(t *testing.T) {
	// mockBuyerClient / mockSellerClient 沒設任何 fn：被呼叫就會 panic，等於斷言沒有呼叫。
	uc := NewCommentUseCase(&mockCommentRepo{}, &mockPostRepo{}, &mockBuyerClient{}, &mockSellerClient{}, nil, func() int64 { return 1 })
	ctx := context.Background()
	unknown := Actor{Kind: 0, ID: sameNumber}

	if name, avatar := uc.resolveCommentIdentity(ctx, unknown); name != "" || avatar != "" {
		t.Errorf("resolveCommentIdentity(unknown kind) = %q, %q, want empty", name, avatar)
	}
	if got := uc.likerName(ctx, unknown); got != "" {
		t.Errorf("likerName(unknown kind) = %q, want empty", got)
	}
	if got := uc.principalOf(ctx, 0, sameNumber); got != 0 {
		t.Errorf("principalOf(unknown kind) = %d, want 0", got)
	}
}
