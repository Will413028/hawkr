//go:build integration

package social_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

type eventRecorder[E any] struct {
	mu  sync.Mutex
	got []E
}

func recordEvents[E any](f *fixture) *eventRecorder[E] {
	r := &eventRecorder[E]{}
	events.Subscribe(f.bus, func(_ context.Context, e E) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.got = append(r.got, e)
		return nil
	})
	return r
}

func (r *eventRecorder[E]) all() []E {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]E(nil), r.got...)
}

func (f *fixture) hideableComment(t *testing.T) (social.Comment, int64, social.Target) {
	t.Helper()
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	c := mustV[social.Comment](t)(f.module.CreateComment(context.Background(), f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	return c, b.BuyerID, social.Target{Kind: social.TargetComment, ID: c.ID}
}

func TestHidePublishesContentHiddenOnceAndRestorePublishesContentRestored(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	hidden, restored := recordEvents[social.ContentHidden](f), recordEvents[social.ContentRestored](f)
	_, buyerID, target := f.hideableComment(t)
	in := social.HideInput{Target: target, Category: "SPAM", Text: "管理員自由文字", SourceRef: "moderation:case:7", AdminID: 5}
	h := mustV[social.Hide](t)(f.module.Hide(ctx, in))
	got := hidden.all()
	if len(got) != 1 {
		t.Fatalf("events=%d", len(got))
	}
	e := got[0]
	if e.SanctionID != h.ID || e.Target != target || e.Author != (social.Actor{Kind: social.ActorBuyer, ID: buyerID}) || e.Category != policy.Spam ||
		e.SourceRef != "moderation:case:7" || e.AppealPath != policy.AppealNone || e.EventID == uuid.Nil() || e.OccurredAt.IsZero() {
		t.Fatalf("event=%+v hide=%+v", e, h)
	}
	if again := mustV[social.Hide](t)(f.module.Hide(ctx, in)); again.Created || len(hidden.all()) != 1 {
		t.Fatalf("冪等的重複隱藏不得再發事件：%+v events=%d", again, len(hidden.all()))
	}
	r := mustV[social.Hide](t)(f.module.Restore(ctx, social.RestoreInput{Target: target, AdminID: 5, Reason: "誤判", SourceRef: "moderation:case:7"}))
	gotR := restored.all()
	if len(gotR) != 1 {
		t.Fatalf("restored events=%d", len(gotR))
	}
	er := gotR[0]
	if er.SanctionID != r.ID || er.SanctionID != h.ID || er.Target != target || er.Author.ID != buyerID || er.Category != policy.Spam || er.SourceRef != "moderation:case:7" || er.AppealPath != policy.AppealNone {
		t.Fatalf("event=%+v", er)
	}
}

func TestHideEventFailureRollsBackTheHide(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	events.Subscribe(f.bus, func(context.Context, social.ContentHidden) error { return errors.New("reject") })
	c, _, target := f.hideableComment(t)
	if _, err := f.module.Hide(ctx, social.HideInput{Target: target, Category: "SPAM", AdminID: 5}); err == nil {
		t.Fatal("event failure lost")
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.hides WHERE comment_id = $1", c.ID); n != 0 {
		t.Fatalf("hides=%d", n)
	}
}

func TestRestoreEventFailureRollsBackTheRestore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	events.Subscribe(f.bus, func(context.Context, social.ContentRestored) error { return errors.New("reject") })
	c, _, target := f.hideableComment(t)
	mustV[social.Hide](t)(f.module.Hide(ctx, social.HideInput{Target: target, Category: "SPAM", AdminID: 5}))
	if _, err := f.module.Restore(ctx, social.RestoreInput{Target: target, AdminID: 5, Reason: "誤判"}); err == nil {
		t.Fatal("event failure lost")
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.hides WHERE comment_id = $1 AND restored_at IS NULL", c.ID); n != 1 {
		t.Fatalf("active hides=%d", n)
	}
}

func TestWarnPublishesBuyerWarnedAndRollsBackWithTheWarning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	warned := recordEvents[social.BuyerWarned](f)
	b := f.newBuyer(t)
	w := mustV[social.Sanction](t)(f.module.Warn(ctx, social.WarnInput{BuyerID: b.BuyerID, Category: "HARASSMENT", Text: "自由文字", SourceRef: "moderation:case:9", AdminID: 5}))
	got := warned.all()
	if len(got) != 1 {
		t.Fatalf("events=%d", len(got))
	}
	e := got[0]
	if e.SanctionID != w.ID || e.BuyerID != b.BuyerID || e.Category != policy.Harassment || e.SourceRef != "moderation:case:9" || e.AppealPath != policy.AppealNone || e.EventID == uuid.Nil() || e.OccurredAt.IsZero() {
		t.Fatalf("event=%+v", e)
	}
	events.Subscribe(f.bus, func(context.Context, social.BuyerWarned) error { return errors.New("reject") })
	if _, err := f.module.Warn(ctx, social.WarnInput{BuyerID: b.BuyerID, Category: "SPAM", AdminID: 5}); err == nil {
		t.Fatal("event failure lost")
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.sanctions WHERE buyer_id = $1", b.BuyerID); n != 1 {
		t.Fatalf("sanctions=%d", n)
	}
}
