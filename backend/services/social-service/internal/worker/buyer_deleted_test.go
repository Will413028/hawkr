package worker

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/social-service/internal/domain"
)

// Embed interfaces — unimplemented methods panic, which is what we want:
// if the handler calls something unexpected, the test fails immediately.

type mockPostRepoForBuyer struct {
	domain.PostRepository
	deleteLikesByUserIDFn func(ctx context.Context, kind int16, userID int64) error
}

func (m *mockPostRepoForBuyer) DeleteLikesByActor(ctx context.Context, kind int16, userID int64) error {
	return m.deleteLikesByUserIDFn(ctx, kind, userID)
}

type mockCommentRepo struct {
	domain.CommentRepository
	softDeleteByUserIDFn  func(ctx context.Context, kind int16, userID int64) error
	deleteLikesByUserIDFn func(ctx context.Context, kind int16, userID int64) error
}

func (m *mockCommentRepo) SoftDeleteByActor(ctx context.Context, kind int16, userID int64) error {
	return m.softDeleteByUserIDFn(ctx, kind, userID)
}

func (m *mockCommentRepo) DeleteLikesByActor(ctx context.Context, kind int16, userID int64) error {
	return m.deleteLikesByUserIDFn(ctx, kind, userID)
}

type mockForumRepo struct {
	domain.ForumRepository
	softDeleteByUserIDFn func(ctx context.Context, kind int16, userID int64) error
}

func (m *mockForumRepo) SoftDeleteByActor(ctx context.Context, kind int16, userID int64) error {
	return m.softDeleteByUserIDFn(ctx, kind, userID)
}

func validBuyerDeletedEvent(accountID, buyerID int64) []byte {
	event := mq.BuyerDeletedEvent{
		BaseEvent: mq.NewBaseEvent(mq.EventTypeBuyerDeleted, "test"),
		AccountID: accountID,
		BuyerID:   buyerID,
	}
	b, _ := json.Marshal(event)
	return b
}

func noopBuyerRepos(t *testing.T) (*mockPostRepoForBuyer, *mockCommentRepo, *mockForumRepo) {
	t.Helper()
	noop := func(context.Context, int16, int64) error { t.Fatal("should not be called"); return nil }
	return &mockPostRepoForBuyer{deleteLikesByUserIDFn: noop},
		&mockCommentRepo{softDeleteByUserIDFn: noop, deleteLikesByUserIDFn: noop},
		&mockForumRepo{softDeleteByUserIDFn: noop}
}

func TestBuyerDeletedHandler_Success(t *testing.T) {
	var postLikeUID, commentUID, commentLikeUID, forumUID int64
	var kinds []int16
	post := &mockPostRepoForBuyer{deleteLikesByUserIDFn: func(_ context.Context, kind int16, uid int64) error {
		postLikeUID = uid
		kinds = append(kinds, kind)
		return nil
	}}
	comment := &mockCommentRepo{
		softDeleteByUserIDFn: func(_ context.Context, kind int16, uid int64) error {
			commentUID = uid
			kinds = append(kinds, kind)
			return nil
		},
		deleteLikesByUserIDFn: func(_ context.Context, kind int16, uid int64) error {
			commentLikeUID = uid
			kinds = append(kinds, kind)
			return nil
		},
	}
	forum := &mockForumRepo{softDeleteByUserIDFn: func(_ context.Context, kind int16, uid int64) error {
		forumUID = uid
		kinds = append(kinds, kind)
		return nil
	}}

	handler := newBuyerDeletedHandler(post, comment, forum, zap.NewNop())
	// accountID (100) deliberately != buyerID (42): asserts the handler keys
	// off BuyerID, not AccountID.
	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 42)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commentUID != 42 {
		t.Errorf("commentRepo.SoftDeleteByUserID got %d, want 42 (buyerId)", commentUID)
	}
	if forumUID != 42 {
		t.Errorf("forumRepo.SoftDeleteByUserID got %d, want 42 (buyerId)", forumUID)
	}
	if postLikeUID != 42 {
		t.Errorf("postRepo.DeleteLikesByUserID got %d, want 42 (buyerId)", postLikeUID)
	}
	if commentLikeUID != 42 {
		t.Errorf("commentRepo.DeleteLikesByUserID got %d, want 42 (buyerId)", commentLikeUID)
	}
	// 同號的店家身分不屬於這個買家：四個清理都只能碰 buyer 這一種身分。
	if len(kinds) != 4 {
		t.Fatalf("cleanup calls = %d, want 4", len(kinds))
	}
	for _, k := range kinds {
		if k != domain.ActorKindBuyer {
			t.Errorf("cleanup used actor kind %d, want only %d (buyer)", k, domain.ActorKindBuyer)
		}
	}
}

func TestBuyerDeletedHandler_MalformedJSON(t *testing.T) {
	post, comment, forum := noopBuyerRepos(t)
	handler := newBuyerDeletedHandler(post, comment, forum, zap.NewNop())

	err := handler(context.Background(), "", []byte("not json"))
	if err == nil {
		t.Fatal("malformed JSON should return err (framework retries → DLQ; 36 §1.5)")
	}
	if !strings.Contains(err.Error(), "unmarshal event") {
		t.Errorf("error = %q, want it to come from mq.BindTypedHandler's wrapped message (\"unmarshal event...\")", err.Error())
	}
}

func TestBuyerDeletedHandler_ZeroBuyerID(t *testing.T) {
	post, comment, forum := noopBuyerRepos(t)
	handler := newBuyerDeletedHandler(post, comment, forum, zap.NewNop())
	// accountId present but buyerId zero — must still reject, since buyerId is
	// the field actually used for the soft-delete queries.
	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 0)); err == nil {
		t.Fatal("zero buyerId should return err (framework retries → DLQ; 36 §1.5)")
	}
}

func TestBuyerDeletedHandler_CommentRepoError(t *testing.T) {
	dbErr := errors.New("db connection refused")
	post := &mockPostRepoForBuyer{deleteLikesByUserIDFn: func(context.Context, int16, int64) error { t.Fatal("should not be called"); return nil }}
	comment := &mockCommentRepo{
		softDeleteByUserIDFn:  func(context.Context, int16, int64) error { return dbErr },
		deleteLikesByUserIDFn: func(context.Context, int16, int64) error { t.Fatal("should not be called"); return nil },
	}
	forum := &mockForumRepo{softDeleteByUserIDFn: func(context.Context, int16, int64) error { t.Fatal("should not be called"); return nil }}
	handler := newBuyerDeletedHandler(post, comment, forum, zap.NewNop())
	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 42)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

func TestBuyerDeletedHandler_ForumRepoError(t *testing.T) {
	dbErr := errors.New("db timeout")
	post := &mockPostRepoForBuyer{deleteLikesByUserIDFn: func(context.Context, int16, int64) error { t.Fatal("should not be called"); return nil }}
	comment := &mockCommentRepo{
		softDeleteByUserIDFn:  func(context.Context, int16, int64) error { return nil },
		deleteLikesByUserIDFn: func(context.Context, int16, int64) error { t.Fatal("should not be called"); return nil },
	}
	forum := &mockForumRepo{softDeleteByUserIDFn: func(context.Context, int16, int64) error { return dbErr }}
	handler := newBuyerDeletedHandler(post, comment, forum, zap.NewNop())
	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 42)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

func TestBuyerDeletedHandler_PostLikeRepoError(t *testing.T) {
	dbErr := errors.New("db error on post like cleanup")
	post := &mockPostRepoForBuyer{deleteLikesByUserIDFn: func(context.Context, int16, int64) error { return dbErr }}
	comment := &mockCommentRepo{
		softDeleteByUserIDFn:  func(context.Context, int16, int64) error { return nil },
		deleteLikesByUserIDFn: func(context.Context, int16, int64) error { return nil },
	}
	forum := &mockForumRepo{softDeleteByUserIDFn: func(context.Context, int16, int64) error { return nil }}
	handler := newBuyerDeletedHandler(post, comment, forum, zap.NewNop())
	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 42)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

func TestBuyerDeletedHandler_CommentLikeRepoError(t *testing.T) {
	dbErr := errors.New("db error on comment like cleanup")
	post := &mockPostRepoForBuyer{deleteLikesByUserIDFn: func(context.Context, int16, int64) error { return nil }}
	comment := &mockCommentRepo{
		softDeleteByUserIDFn:  func(context.Context, int16, int64) error { return nil },
		deleteLikesByUserIDFn: func(context.Context, int16, int64) error { return dbErr },
	}
	// forumRepo.SoftDeleteByUserID runs earlier in the handler's sequence
	// (commentSoft -> forumSoft -> postLike -> commentLike) and must succeed
	// here so the test isolates the commentLike failure; it is NOT a
	// "should not be called" case (deviation from the task brief, which
	// mistakenly used a t.Fatal stub here — see task-3-report.md).
	forum := &mockForumRepo{softDeleteByUserIDFn: func(context.Context, int16, int64) error { return nil }}
	handler := newBuyerDeletedHandler(post, comment, forum, zap.NewNop())
	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 42)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

// TestBuyerDeletedHandler_NoLocalPayloadStructRedeclaration guards against
// reintroducing a locally-redeclared payload struct that can silently drift
// from pkg/mq's canonical wire shape — exactly what happened to the 4
// account.deleted consumers fixed elsewhere in this plan (wrong json tag,
// silently decoding to a zero value against real traffic). buyer_deleted.go
// must decode directly into mq.BuyerDeletedEvent; im-service's
// buyer_deleted.go is the reference for correct reuse.
func TestBuyerDeletedHandler_NoLocalPayloadStructRedeclaration(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "buyer_deleted.go", nil, 0)
	if err != nil {
		t.Fatalf("parse buyer_deleted.go: %v", err)
	}
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == "buyerDeletedPayload" {
				t.Error("buyer_deleted.go must not locally redeclare buyerDeletedPayload — decode directly into mq.BuyerDeletedEvent (pkg/mq/events.go:604)")
			}
		}
	}
}
