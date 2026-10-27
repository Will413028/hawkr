package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/social-service/internal/domain"
)

// Dedicated mocks for this file only — avoids coupling to the mockPostRepo/
// mockCommentRepo defined in account_deleted_test.go/buyer_deleted_test.go,
// which only override the one method each of their own tests need.

type counterDeltaMockPostRepo struct {
	domain.PostRepository
	incrCommentCountFn func(ctx context.Context, id int64) error
	decrCommentCountFn func(ctx context.Context, id int64) error
	incrLikeCountFn    func(ctx context.Context, id int64) error
	decrLikeCountFn    func(ctx context.Context, id int64) error
}

func (m *counterDeltaMockPostRepo) IncrCommentCount(ctx context.Context, id int64) error {
	return m.incrCommentCountFn(ctx, id)
}
func (m *counterDeltaMockPostRepo) DecrCommentCount(ctx context.Context, id int64) error {
	return m.decrCommentCountFn(ctx, id)
}
func (m *counterDeltaMockPostRepo) IncrLikeCount(ctx context.Context, id int64) error {
	return m.incrLikeCountFn(ctx, id)
}
func (m *counterDeltaMockPostRepo) DecrLikeCount(ctx context.Context, id int64) error {
	return m.decrLikeCountFn(ctx, id)
}

type counterDeltaMockCommentRepo struct {
	domain.CommentRepository
	incrReplyCountFn func(ctx context.Context, id int64) error
	decrReplyCountFn func(ctx context.Context, id int64) error
	incrLikeCountFn  func(ctx context.Context, id int64) error
	decrLikeCountFn  func(ctx context.Context, id int64) error
}

func (m *counterDeltaMockCommentRepo) IncrReplyCount(ctx context.Context, id int64) error {
	return m.incrReplyCountFn(ctx, id)
}
func (m *counterDeltaMockCommentRepo) DecrReplyCount(ctx context.Context, id int64) error {
	return m.decrReplyCountFn(ctx, id)
}
func (m *counterDeltaMockCommentRepo) IncrLikeCount(ctx context.Context, id int64) error {
	return m.incrLikeCountFn(ctx, id)
}
func (m *counterDeltaMockCommentRepo) DecrLikeCount(ctx context.Context, id int64) error {
	return m.decrLikeCountFn(ctx, id)
}

func validCounterDeltaEvent(entityType string, entityID int64, field string, delta int32) []byte {
	b, _ := json.Marshal(counterDeltaPayload{
		EntityType: entityType,
		EntityID:   entityID,
		Field:      field,
		Delta:      delta,
	})
	return b
}

func noopPost() *counterDeltaMockPostRepo {
	fail := func(context.Context, int64) error { return nil }
	return &counterDeltaMockPostRepo{incrCommentCountFn: fail, decrCommentCountFn: fail, incrLikeCountFn: fail, decrLikeCountFn: fail}
}
func noopComment() *counterDeltaMockCommentRepo {
	fail := func(context.Context, int64) error { return nil }
	return &counterDeltaMockCommentRepo{incrReplyCountFn: fail, decrReplyCountFn: fail, incrLikeCountFn: fail, decrLikeCountFn: fail}
}

func TestCounterDeltaHandler_PostCommentCountIncr(t *testing.T) {
	var gotID int64
	post := noopPost()
	post.incrCommentCountFn = func(_ context.Context, id int64) error { gotID = id; return nil }
	handler := newCounterDeltaHandler(post, noopComment(), zap.NewNop())
	if err := handler(context.Background(), "", validCounterDeltaEvent("post", 10, "comment_count", 1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotID != 10 {
		t.Errorf("IncrCommentCount got id=%d, want 10", gotID)
	}
}

func TestCounterDeltaHandler_PostCommentCountDecr(t *testing.T) {
	var gotID int64
	post := noopPost()
	post.decrCommentCountFn = func(_ context.Context, id int64) error { gotID = id; return nil }
	handler := newCounterDeltaHandler(post, noopComment(), zap.NewNop())
	if err := handler(context.Background(), "", validCounterDeltaEvent("post", 10, "comment_count", -1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotID != 10 {
		t.Errorf("DecrCommentCount got id=%d, want 10", gotID)
	}
}

func TestCounterDeltaHandler_PostLikeCount(t *testing.T) {
	var incrID, decrID int64
	post := noopPost()
	post.incrLikeCountFn = func(_ context.Context, id int64) error { incrID = id; return nil }
	post.decrLikeCountFn = func(_ context.Context, id int64) error { decrID = id; return nil }
	handler := newCounterDeltaHandler(post, noopComment(), zap.NewNop())
	if err := handler(context.Background(), "", validCounterDeltaEvent("post", 20, "like_count", 1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if incrID != 20 {
		t.Errorf("IncrLikeCount got id=%d, want 20", incrID)
	}
	if err := handler(context.Background(), "", validCounterDeltaEvent("post", 20, "like_count", -1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decrID != 20 {
		t.Errorf("DecrLikeCount got id=%d, want 20", decrID)
	}
}

func TestCounterDeltaHandler_CommentReplyCount(t *testing.T) {
	var incrID, decrID int64
	comment := noopComment()
	comment.incrReplyCountFn = func(_ context.Context, id int64) error { incrID = id; return nil }
	comment.decrReplyCountFn = func(_ context.Context, id int64) error { decrID = id; return nil }
	handler := newCounterDeltaHandler(noopPost(), comment, zap.NewNop())
	if err := handler(context.Background(), "", validCounterDeltaEvent("comment", 30, "reply_count", 1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if incrID != 30 {
		t.Errorf("IncrReplyCount got id=%d, want 30", incrID)
	}
	if err := handler(context.Background(), "", validCounterDeltaEvent("comment", 30, "reply_count", -1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decrID != 30 {
		t.Errorf("DecrReplyCount got id=%d, want 30", decrID)
	}
}

func TestCounterDeltaHandler_CommentLikeCount(t *testing.T) {
	var incrID, decrID int64
	comment := noopComment()
	comment.incrLikeCountFn = func(_ context.Context, id int64) error { incrID = id; return nil }
	comment.decrLikeCountFn = func(_ context.Context, id int64) error { decrID = id; return nil }
	handler := newCounterDeltaHandler(noopPost(), comment, zap.NewNop())
	if err := handler(context.Background(), "", validCounterDeltaEvent("comment", 40, "like_count", 1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if incrID != 40 {
		t.Errorf("IncrLikeCount got id=%d, want 40", incrID)
	}
	if err := handler(context.Background(), "", validCounterDeltaEvent("comment", 40, "like_count", -1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decrID != 40 {
		t.Errorf("DecrLikeCount got id=%d, want 40", decrID)
	}
}

func TestCounterDeltaHandler_ZeroEntityID(t *testing.T) {
	handler := newCounterDeltaHandler(noopPost(), noopComment(), zap.NewNop())
	if err := handler(context.Background(), "", validCounterDeltaEvent("post", 0, "comment_count", 1)); err == nil {
		t.Fatal("zero entityId should return err (framework retries → DLQ; 36 §1.5)")
	}
}

func TestCounterDeltaHandler_NegativeEntityID(t *testing.T) {
	handler := newCounterDeltaHandler(noopPost(), noopComment(), zap.NewNop())
	if err := handler(context.Background(), "", validCounterDeltaEvent("post", -1, "comment_count", 1)); err == nil {
		t.Fatal("negative entityId should return err (framework retries → DLQ; 36 §1.5)")
	}
}

func TestCounterDeltaHandler_MalformedJSON(t *testing.T) {
	handler := newCounterDeltaHandler(noopPost(), noopComment(), zap.NewNop())
	if err := handler(context.Background(), "", []byte("not json")); err == nil {
		t.Fatal("malformed JSON should return err (framework retries → DLQ; 36 §1.5)")
	}
}

func TestCounterDeltaHandler_UnknownEntityTypeOrField(t *testing.T) {
	handler := newCounterDeltaHandler(noopPost(), noopComment(), zap.NewNop())
	if err := handler(context.Background(), "", validCounterDeltaEvent("post", 1, "unknown_field", 1)); err == nil {
		t.Fatal("unknown field should return err (framework retries → DLQ)")
	}
	if err := handler(context.Background(), "", validCounterDeltaEvent("unknown_entity", 1, "like_count", 1)); err == nil {
		t.Fatal("unknown entityType should return err (framework retries → DLQ)")
	}
}

func TestCounterDeltaHandler_RepoError(t *testing.T) {
	dbErr := errors.New("db error")
	post := noopPost()
	post.incrCommentCountFn = func(context.Context, int64) error { return dbErr }
	handler := newCounterDeltaHandler(post, noopComment(), zap.NewNop())
	if err := handler(context.Background(), "", validCounterDeltaEvent("post", 10, "comment_count", 1)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}
