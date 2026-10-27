package usecase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// tagRecorder stands in for the website's /api/revalidate endpoint so a test can
// assert the exact tag set a mutation emits. BustAsync fires in a goroutine, so
// received tags arrive on a channel rather than a plain field.
type tagRecorder struct {
	server *httptest.Server
	posts  chan []string
}

func newTagRecorder(t *testing.T) *tagRecorder {
	t.Helper()
	rec := &tagRecorder{posts: make(chan []string, 8)}
	rec.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Tags []string `json:"tags"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		rec.posts <- body.Tags
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(rec.server.Close)
	return rec
}

func (r *tagRecorder) client() *revalidate.Client {
	return revalidate.New(revalidate.Config{BaseURL: r.server.URL, Secret: "test-secret"}, nil)
}

// awaitTags waits for one bust. Failing here (rather than timing out silently)
// is the point: it is what turns "the producer was removed" into a red test.
func (r *tagRecorder) awaitTags(t *testing.T) []string {
	t.Helper()
	select {
	case tags := <-r.posts:
		return tags
	case <-time.After(3 * time.Second):
		t.Fatal("expected a revalidate POST, got none — the producer is missing")
		return nil
	}
}

func (r *tagRecorder) expectNoBust(t *testing.T) {
	t.Helper()
	select {
	case tags := <-r.posts:
		t.Fatalf("expected no revalidate POST, got %v", tags)
	case <-time.After(300 * time.Millisecond):
	}
}

func assertTags(t *testing.T, got, want []string) {
	t.Helper()
	gotSorted := slices.Clone(got)
	wantSorted := slices.Clone(want)
	slices.Sort(gotSorted)
	slices.Sort(wantSorted)
	if !slices.Equal(gotSorted, wantSorted) {
		t.Errorf("tags = %v, want %v", got, want)
	}
}

func newRevalidatingPostUC(repo *mockPostRepo, hashtagRepo *mockHashtagRepo, rec *tagRecorder) *PostUseCase {
	return NewPostUseCase(repo, NewHashtagUseCase(hashtagRepo, func() int64 { return 88888 }),
		&mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{},
		&mockProductClient{}, nil, &config.Config{}, func() int64 { return 4242 }, rec.client())
}

func TestPostUC_CreatePost_BustsPostAndHashtagTags(t *testing.T) {
	rec := newTagRecorder(t)
	repo := &mockPostRepo{
		createFn: func(_ context.Context, id, sellerID int64, _ string, _ string, _ *int64, _ *int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: sellerID}, nil
		},
	}
	hashtagRepo := &mockHashtagRepo{
		upsertByNameFn: func(_ context.Context, _ gen.DBTX, _ int64, canonical string) (int64, error) {
			if canonical == "vegetables" {
				return 501, nil
			}
			return 502, nil
		},
	}
	uc := newRevalidatingPostUC(repo, hashtagRepo, rec)

	if _, err := uc.CreatePost(context.Background(), 10, "hello", "tok", nil,
		[]string{"vegetables", "fruit"}, domain.PostPublishStatusPublished); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	assertTags(t, rec.awaitTags(t), []string{"post:4242", "hashtag:501", "hashtag:502"})
}

func TestPostUC_CreatePost_NoTags_BustsPostOnly(t *testing.T) {
	rec := newTagRecorder(t)
	repo := &mockPostRepo{
		createFn: func(_ context.Context, id, sellerID int64, _ string, _ string, _ *int64, _ *int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: sellerID}, nil
		},
	}
	uc := newRevalidatingPostUC(repo, &mockHashtagRepo{}, rec)

	if _, err := uc.CreatePost(context.Background(), 10, "hello", "tok", nil, nil, domain.PostPublishStatusPublished); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	assertTags(t, rec.awaitTags(t), []string{"post:4242"})
}

func TestPostUC_CreatePost_RepoFailure_DoesNotBust(t *testing.T) {
	rec := newTagRecorder(t)
	repo := &mockPostRepo{
		createFn: func(_ context.Context, _, _ int64, _ string, _ string, _ *int64, _ *int64) (*gen.CmsPost, error) {
			return nil, context.DeadlineExceeded
		},
	}
	uc := newRevalidatingPostUC(repo, &mockHashtagRepo{}, rec)

	if _, err := uc.CreatePost(context.Background(), 10, "hello", "", nil, nil, domain.PostPublishStatusPublished); err == nil {
		t.Fatal("expected CreatePost to fail")
	}
	// Nothing was written, so nothing may be invalidated.
	rec.expectNoBust(t)
}

func TestPostUC_DeletePost_BustsPostAndBoundHashtags(t *testing.T) {
	rec := newTagRecorder(t)
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10}, nil
		},
		softDeleteFn: func(_ context.Context, _, _ int64) (int64, error) { return 1, nil },
	}
	hashtagRepo := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			return []int64{601, 602}, nil
		},
	}
	uc := newRevalidatingPostUC(repo, hashtagRepo, rec)

	if err := uc.DeletePost(context.Background(), 777, 10); err != nil {
		t.Fatalf("DeletePost: %v", err)
	}

	assertTags(t, rec.awaitTags(t), []string{"post:777", "hashtag:601", "hashtag:602"})
}

func TestPostUC_DeletePost_AlreadyHidden_BustsPostWithoutHashtags(t *testing.T) {
	rec := newTagRecorder(t)
	hidden := testTimePointer(time.Now())
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10, HiddenAt: hidden}, nil
		},
		softDeleteFn: func(_ context.Context, _, _ int64) (int64, error) { return 1, nil },
	}
	hashtagRepo := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			return []int64{601}, nil
		},
	}
	uc := newRevalidatingPostUC(repo, hashtagRepo, rec)

	if err := uc.DeletePost(context.Background(), 777, 10); err != nil {
		t.Fatalf("DeletePost: %v", err)
	}

	// A hidden post was already out of the public counts, so no hashtag ranking
	// moved — busting hashtags:list here would be over-invalidation.
	assertTags(t, rec.awaitTags(t), []string{"post:777"})
}

func TestPostUC_RestorePost_Busts(t *testing.T) {
	rec := newTagRecorder(t)
	hidden := testTimePointer(time.Now())
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10, HiddenAt: hidden}, nil
		},
		restorePostFn: func(_ context.Context, _ int64) error { return nil },
	}
	hashtagRepo := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			return []int64{701}, nil
		},
	}
	uc := newRevalidatingPostUC(repo, hashtagRepo, rec)

	if err := uc.RestorePost(context.Background(), 555); err != nil {
		t.Fatalf("RestorePost: %v", err)
	}

	assertTags(t, rec.awaitTags(t), []string{"post:555", "hashtag:701"})
}

func TestPostUC_UpdatePost_BustsPostTag(t *testing.T) {
	rec := newTagRecorder(t)
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10, Image: testStringPointer("posts/a.jpg")}, nil
		},
		updateFn: func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error { return nil },
	}
	uc := newRevalidatingPostUC(repo, &mockHashtagRepo{}, rec)

	if _, err := uc.UpdatePost(context.Background(), 888, 10, "edited", "111", "", nil, false, nil, nil); err != nil {
		t.Fatalf("UpdatePost: %v", err)
	}

	// tagsPresent=false → no hashtag counters moved.
	assertTags(t, rec.awaitTags(t), []string{"post:888"})
}

func TestPostHider_HidePost_BustsPostAndHashtags(t *testing.T) {
	rec := newTagRecorder(t)
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10}, nil
		},
		hidePostFn: func(_ context.Context, _ int64, _ string) error { return nil },
	}
	hashtagRepo := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			return []int64{801}, nil
		},
	}
	h := NewPostHider(repo, NewHashtagUseCase(hashtagRepo, func() int64 { return 88888 }),
		nil, &mockSellerClient{}, rec.client())

	if err := h.HidePost(context.Background(), 909, "spam"); err != nil {
		t.Fatalf("HidePost: %v", err)
	}

	assertTags(t, rec.awaitTags(t), []string{"post:909", "hashtag:801"})
}

func TestPostHider_HidePost_AlreadyHidden_DoesNotBust(t *testing.T) {
	rec := newTagRecorder(t)
	hidden := testTimePointer(time.Now())
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10, HiddenAt: hidden}, nil
		},
	}
	h := NewPostHider(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }),
		nil, &mockSellerClient{}, rec.client())

	if err := h.HidePost(context.Background(), 909, "spam"); err != nil {
		t.Fatalf("HidePost: %v", err)
	}

	// Idempotent no-op: nothing changed, so nothing is invalidated.
	rec.expectNoBust(t)
}
