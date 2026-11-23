//go:build integration

package turn_test

import (
	"context"
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/turn"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// txQuota 記錄額度是在 transaction 內讀的（買家的送出鎖之後）。
type txQuota struct {
	inTx  bool
	calls int
}

func (q *txQuota) TurnQuota(ctx context.Context, _ int64) (turn.Quota, error) {
	q.calls++
	_, q.inTx = db.TxFrom(ctx)
	return turn.Quota{Limit: 5, PeriodStart: now, PeriodEnd: now.AddDate(0, 1, 0)}, nil
}

func newSubmitter(f *fixture, q turn.QuotaSource) turn.Submitter {
	return turn.Submitter{Submitter: run.Submitter{DB: f.d, Ledger: f.ledger(), Jobs: f.jobs}, Quota: q}
}

func TestSubmitReadsTheQuotaInsideTheTransaction(t *testing.T) { // A9-D24：額度與扣額度同一個 transaction、買家鎖之後
	f := newFixture(t)
	q := &txQuota{}
	if _, err := newSubmitter(f, q).Submit(context.Background(), turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: uuid.NewV7()}); err != nil {
		t.Fatal(err)
	}
	if q.calls != 1 || !q.inTx {
		t.Fatalf("calls=%d inTx=%v，額度要在 transaction 內讀一次", q.calls, q.inTx)
	}
}

func TestReplayOfADeletedConversationIsNotFound(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	s := newSubmitter(f, &txQuota{})
	cid := uuid.NewV7()
	first, err := s.Submit(ctx, turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: cid})
	if err != nil {
		t.Fatal(err)
	}
	if again, err := s.Submit(ctx, turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: cid}); err != nil || again != first {
		t.Fatalf("重送 = %+v, %v，要回放 %+v", again, err, first)
	}
	n, err := store.New(f.d.Conn(ctx)).SoftDeleteConversation(ctx, store.SoftDeleteConversationParams{ID: first.ConversationID, BuyerID: buyer, Now: &now})
	if err != nil || n != 1 {
		t.Fatalf("刪除對話 = %d, %v", n, err)
	}
	_, err = s.Submit(ctx, turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: cid})
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("對話已刪除後重送 = %v，要 NOT_FOUND", err)
	}
}
