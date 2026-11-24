//go:build integration

package im_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
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

func TestHidePublishesMessageHiddenForTheSenderAndRestorePublishesMessageRestored(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	hidden, restored := record[im.MessageHidden](f), record[im.MessageRestored](f)
	buyerMsg := f.say(t, f.buyerParty(b1), s1, "買家說的")
	storeMsg := f.say(t, f.storeParty(t, s1), b1, "店家說的")
	in := im.HideInput{MessageID: buyerMsg.ID, AdminID: 1, Category: string(policy.Spam), Text: "管理員自由文字", SourceRef: "moderation:case:5"}
	h := mustHide(t)(f.m.Hide(ctx, in))
	got := hidden.all()
	if len(got) != 1 {
		t.Fatalf("events=%d", len(got))
	}
	e := got[0]
	if e.SanctionID != h.ID || e.MessageID != buyerMsg.ID || e.ConversationID != buyerMsg.ConversationID || e.AuthorKind != im.PartyBuyer || e.AuthorID != b1 ||
		e.Category != policy.Spam || e.SourceRef != "moderation:case:5" || e.AppealPath != policy.AppealNone || e.EventID == uuid.Nil() || e.OccurredAt.IsZero() {
		t.Fatalf("event=%+v", e)
	}
	if again := mustHide(t)(f.m.Hide(ctx, in)); again.Created || len(hidden.all()) != 1 {
		t.Fatalf("冪等的重複隱藏不得再發事件：%+v events=%d", again, len(hidden.all()))
	}
	if _, err := f.m.Hide(ctx, im.HideInput{MessageID: storeMsg.ID, AdminID: 1, Category: string(policy.Spam)}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("店家寄的訊息不能隱藏（店家內容走 seller 措施）：%v", err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.hides WHERE message_id = $1", storeMsg.ID); n != 0 || len(hidden.all()) != 1 {
		t.Fatalf("被拒的隱藏不得留列或發事件：hides=%d events=%d", n, len(hidden.all()))
	}
	mustHide(t)(f.m.Restore(ctx, im.RestoreInput{MessageID: buyerMsg.ID, AdminID: 1, Reason: "誤判", SourceRef: "moderation:case:5"}))
	gotR := restored.all()
	if len(gotR) != 1 || gotR[0].SanctionID != h.ID || gotR[0].MessageID != buyerMsg.ID || gotR[0].AuthorKind != im.PartyBuyer || gotR[0].AuthorID != b1 ||
		gotR[0].Category != policy.Spam || gotR[0].SourceRef != "moderation:case:5" || gotR[0].AppealPath != policy.AppealNone {
		t.Fatalf("restored=%+v", gotR)
	}
}

func TestMessageHideEventFailureRollsBackTheHide(t *testing.T) {
	t.Parallel()
	f := setup(t)
	events.Subscribe(f.bus, func(context.Context, im.MessageHidden) error { return errors.New("reject") })
	m := f.say(t, f.buyerParty(b1), s1, "會被隱藏")
	if _, err := f.m.Hide(context.Background(), f.hideInput(m.ID)); err == nil {
		t.Fatal("event failure lost")
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.hides WHERE message_id = $1", m.ID); n != 0 {
		t.Fatalf("hides=%d", n)
	}
}

func TestMessageRestoreEventFailureRollsBackTheRestore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	events.Subscribe(f.bus, func(context.Context, im.MessageRestored) error { return errors.New("reject") })
	m := f.say(t, f.buyerParty(b1), s1, "會被隱藏")
	mustHide(t)(f.m.Hide(context.Background(), f.hideInput(m.ID)))
	if _, err := f.m.Restore(context.Background(), im.RestoreInput{MessageID: m.ID, AdminID: 1, Reason: "誤判"}); err == nil {
		t.Fatal("event failure lost")
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.hides WHERE message_id = $1 AND restored_at IS NULL", m.ID); n != 1 {
		t.Fatalf("active hides=%d", n)
	}
}

func mustHide(t *testing.T) func(im.Hide, error) im.Hide {
	return func(h im.Hide, err error) im.Hide {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
}
