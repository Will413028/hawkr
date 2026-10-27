package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

func TestForumUC_ListReplies_ClampsAndOffsets(t *testing.T) {
	var gotLimit, gotOffset int64
	repo := &mockForumRepo{
		listRepliesFn: func(_ context.Context, _ int64, limit, offset int64) ([]*gen.CmsSellerForum, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		countRepliesFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
	}

	_, _, err := newTestForumUC(repo).ListReplies(context.Background(), 1, 3, 999)
	if err != nil {
		t.Fatalf("ListReplies() error = %v", err)
	}
	if gotLimit != int64(domain.MaxPageSize) {
		t.Errorf("limit = %d, want %d", gotLimit, domain.MaxPageSize)
	}
	if gotOffset != int64(2*domain.MaxPageSize) {
		t.Errorf("offset = %d, want %d", gotOffset, 2*domain.MaxPageSize)
	}
}

func TestCommentUC_ListReplies_ClampsAndOffsets(t *testing.T) {
	var gotLimit, gotOffset int64
	repo := &mockCommentRepo{
		listRepliesFn: func(_ context.Context, _ int64, limit, offset int64) ([]*gen.CmsComment, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		countRepliesFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
	}

	_, _, err := newTestCommentUC(repo, &mockPostRepo{}).ListReplies(context.Background(), 1, 3, 999)
	if err != nil {
		t.Fatalf("ListReplies() error = %v", err)
	}
	if gotLimit != int64(domain.MaxPageSize) {
		t.Errorf("limit = %d, want %d", gotLimit, domain.MaxPageSize)
	}
	if gotOffset != int64(2*domain.MaxPageSize) {
		t.Errorf("offset = %d, want %d", gotOffset, 2*domain.MaxPageSize)
	}
}

func TestPostUC_AdminListPosts_ClampsAndOffsets(t *testing.T) {
	var gotLimit, gotOffset int64
	repo := &mockPostRepo{
		adminListFn: func(_ context.Context, _ string, limit, offset int64) ([]gen.AdminListPostsRow, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		adminCountFn: func(_ context.Context, _ string) (int64, error) {
			return 0, nil
		},
	}

	_, _, err := newTestPostUC(repo, &mockMediaClient{}, &mockSellerClient{}).AdminListPosts(context.Background(), "", 3, 999)
	if err != nil {
		t.Fatalf("AdminListPosts() error = %v", err)
	}
	if gotLimit != int64(domain.MaxModerationPageSize) {
		t.Errorf("limit = %d, want %d", gotLimit, domain.MaxModerationPageSize)
	}
	if gotOffset != int64(2*domain.MaxModerationPageSize) {
		t.Errorf("offset = %d, want %d", gotOffset, 2*domain.MaxModerationPageSize)
	}
}

func TestPostUC_ListPostsByTag_PreservesPagination(t *testing.T) {
	var gotTag string
	var gotLimit, gotOffset int64
	repo := &mockPostRepo{
		listByHashtagFn: func(_ context.Context, tagName string, limit, offset int64) ([]*gen.CmsPost, error) {
			gotTag = tagName
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		countByHashtagFn: func(_ context.Context, _ string) (int64, error) {
			return 0, nil
		},
	}

	_, _, err := newTestPostUC(repo, &mockMediaClient{}, &mockSellerClient{}).ListPostsByTag(context.Background(), "#Fresh", 2, 10)
	if err != nil {
		t.Fatalf("ListPostsByTag() error = %v", err)
	}
	if gotTag != "fresh" {
		t.Errorf("tag = %q, want %q", gotTag, "fresh")
	}
	if gotLimit != 10 {
		t.Errorf("limit = %d, want 10", gotLimit)
	}
	if gotOffset != 10 {
		t.Errorf("offset = %d, want 10", gotOffset)
	}
}
