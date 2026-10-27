package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

func newTestPostHider(repo *mockPostRepo, pub ContentEventPublisher, sellerClient SellerClient) *PostHider {
	return NewPostHider(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), pub, sellerClient, nil)
}

func TestPostHider_HidePost_PublishesResolvedAccountID_NotRawStoreID(t *testing.T) {
	const storeID int64 = 10             // post.SellerID — a store surrogate
	const resolvedAccountID int64 = 5001 // what sellerClient.ResolvePrincipal resolves storeID to
	var hideCalledWithID int64
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: storeID}, nil
		},
		hidePostFn: func(_ context.Context, id int64, _ string) error {
			hideCalledWithID = id
			return nil
		},
	}
	pub := &mockContentEventPublisher{}
	sellerClient := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			if sid == storeID {
				return resolvedAccountID
			}
			return sid
		},
	}
	h := newTestPostHider(repo, pub, sellerClient)

	if err := h.HidePost(context.Background(), 42, "違規內容"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hideCalledWithID != 42 {
		t.Errorf("postRepo.HidePost postID=%d, want 42", hideCalledWithID)
	}
	if len(pub.postHiddenCalls) != 1 {
		t.Fatalf("PublishPostHiddenByPlatform called %d times, want 1", len(pub.postHiddenCalls))
	}
	got := pub.postHiddenCalls[0]
	if got.sellerID != resolvedAccountID || got.postID != 42 || got.reason != "違規內容" {
		t.Errorf("event = %+v, want {sellerID:%d postID:42 reason:違規內容}", got, resolvedAccountID)
	}
	// storeID (Phase A 新欄位) 必須是未解析的 raw cms_post.seller_id，與已解析的
	// sellerID 落在不同身分空間；兩者必須不相等，否則就是誤把 resolved 值複製進了
	// raw 欄位。
	if got.storeID != storeID {
		t.Errorf("storeID = %d, want raw store_id %d (must not be the resolved value)", got.storeID, storeID)
	}
	if got.storeID == got.sellerID {
		t.Errorf("storeID (%d) must differ from resolved sellerID (%d)", got.storeID, got.sellerID)
	}
}

func TestPostHider_HidePost_AlreadyHidden_NoEvent(t *testing.T) {
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 10, HiddenAt: testTimePointer(time.Now())}, nil
		},
		hidePostFn: func(_ context.Context, _ int64, _ string) error {
			t.Fatal("HidePost should not be called for an already-hidden post")
			return nil
		},
	}
	pub := &mockContentEventPublisher{}
	h := newTestPostHider(repo, pub, &mockSellerClient{})

	if err := h.HidePost(context.Background(), 42, "x"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.postHiddenCalls) != 0 {
		t.Errorf("expected no event for already-hidden post, got %d", len(pub.postHiddenCalls))
	}
}

func TestPostHider_HidePost_NotFound(t *testing.T) {
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) {
			return nil, errors.New("no rows")
		},
	}
	h := newTestPostHider(repo, &mockContentEventPublisher{}, &mockSellerClient{})
	if err := h.HidePost(context.Background(), 99, "reason"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestPostHider_HidePost_DeletedPost_NotFound(t *testing.T) {
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 10, DeletedAt: testTimePointer(time.Now())}, nil
		},
	}
	h := newTestPostHider(repo, &mockContentEventPublisher{}, &mockSellerClient{})
	if err := h.HidePost(context.Background(), 42, "reason"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestPostHider_HidePost_NilPublisher_Safe(t *testing.T) {
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 10}, nil
		},
		hidePostFn: func(_ context.Context, _ int64, _ string) error { return nil },
	}
	h := NewPostHider(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), nil, &mockSellerClient{}, nil)
	if err := h.HidePost(context.Background(), 42, "reason"); err != nil {
		t.Fatalf("unexpected error with nil publisher: %v", err)
	}
}
