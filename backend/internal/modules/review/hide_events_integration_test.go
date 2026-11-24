//go:build integration

package review_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

type recorded[E any] struct {
	mu  sync.Mutex
	got []E
}

func record[E any](f *fixture) *recorded[E] {
	r := &recorded[E]{}
	events.Subscribe(f.bus, func(_ context.Context, e E) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.got = append(r.got, e)
		return nil
	})
	return r
}

func (r *recorded[E]) all() []E {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]E(nil), r.got...)
}

func TestHidePublishesReviewHiddenOnceAndRestorePublishesReviewRestored(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	hidden, restored := record[review.ReviewHidden](f), record[review.ReviewRestored](f)
	b := f.newBuyer(t)
	r := f.submit(t, b, f.exposableStore(t), 700, 4, "評價")
	in := review.HideInput{ReviewID: r.ID, Category: "SPAM", Text: "管理員自由文字", SourceRef: "moderation:case:3", AdminID: 1}
	h := must[review.Hide](t)(f.module.Hide(ctx, in))
	got := hidden.all()
	if len(got) != 1 {
		t.Fatalf("events=%d", len(got))
	}
	e := got[0]
	if e.SanctionID != h.ID || e.ReviewID != r.ID || e.BuyerID != b.BuyerID || e.Category != policy.Spam || e.SourceRef != "moderation:case:3" ||
		e.AppealPath != policy.AppealNone || e.EventID == uuid.Nil() || e.OccurredAt.IsZero() {
		t.Fatalf("event=%+v", e)
	}
	if again := must[review.Hide](t)(f.module.Hide(ctx, in)); again.Created || len(hidden.all()) != 1 {
		t.Fatalf("冪等的重複隱藏不得再發事件：%+v events=%d", again, len(hidden.all()))
	}
	must[review.Hide](t)(f.module.Restore(ctx, review.RestoreInput{ReviewID: r.ID, AdminID: 1, Reason: "誤判", SourceRef: "moderation:case:3"}))
	gotR := restored.all()
	if len(gotR) != 1 || gotR[0].SanctionID != h.ID || gotR[0].ReviewID != r.ID || gotR[0].BuyerID != b.BuyerID || gotR[0].Category != policy.Spam ||
		gotR[0].SourceRef != "moderation:case:3" || gotR[0].AppealPath != policy.AppealNone {
		t.Fatalf("restored=%+v", gotR)
	}
}

func TestReviewHideEventFailureRollsBackTheHide(t *testing.T) {
	t.Parallel()
	f := setup(t)
	events.Subscribe(f.bus, func(context.Context, review.ReviewHidden) error { return errors.New("reject") })
	r := f.submit(t, f.newBuyer(t), f.exposableStore(t), 700, 4, "評價")
	if _, err := f.module.Hide(context.Background(), review.HideInput{ReviewID: r.ID, Category: "SPAM", Text: "廣告", AdminID: 1}); err == nil {
		t.Fatal("event failure lost")
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.hides WHERE review_id = $1", r.ID); n != 0 {
		t.Fatalf("hides=%d", n)
	}
}

func TestReviewRestoreEventFailureRollsBackTheRestore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	events.Subscribe(f.bus, func(context.Context, review.ReviewRestored) error { return errors.New("reject") })
	r := f.submit(t, f.newBuyer(t), f.exposableStore(t), 700, 4, "評價")
	hide(t, f, r.ID)
	if _, err := f.module.Restore(context.Background(), review.RestoreInput{ReviewID: r.ID, AdminID: 1, Reason: "誤判"}); err == nil {
		t.Fatal("event failure lost")
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.hides WHERE review_id = $1 AND restored_at IS NULL", r.ID); n != 1 {
		t.Fatalf("active hides=%d", n)
	}
}
