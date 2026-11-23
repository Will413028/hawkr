//go:build integration

package turn_test

import (
	"context"
	"errors"
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/turn"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// txQuota 記錄額度是在 transaction 內讀的（買家的送出鎖之後）。
type txQuota struct {
	conn     func(context.Context) db.DBTX
	inTx     bool
	unlocked bool
	calls    int
}

const lockHeldSQL = `SELECT EXISTS (
  SELECT 1 FROM pg_locks l, (SELECT hashtextextended('procurement.run_submit:' || $1::bigint, 0) AS h) x
  WHERE l.locktype = 'advisory' AND l.pid = pg_backend_pid() AND l.granted AND l.objsubid = 1
    AND l.classid = ((x.h >> 32) & 4294967295)::oid AND l.objid = (x.h & 4294967295)::oid)`

func (q *txQuota) TurnQuota(ctx context.Context, buyerID int64) (turn.Quota, error) {
	q.calls++
	_, q.inTx = db.TxFrom(ctx)
	if q.conn != nil {
		var held bool
		if err := q.conn(ctx).QueryRow(ctx, lockHeldSQL, buyerID).Scan(&held); err != nil {
			return turn.Quota{}, err
		}
		q.unlocked = q.unlocked || !held
	}
	return turn.Quota{Limit: 5, PeriodStart: now, PeriodEnd: now.AddDate(0, 1, 0)}, nil
}

func newSubmitter(f *fixture, q turn.QuotaSource) turn.Submitter {
	return turn.Submitter{Submitter: run.Submitter{DB: f.d, Ledger: f.ledger(), Jobs: f.jobs}, Quota: q}
}

func TestSubmitReadsTheQuotaInsideTheTransaction(t *testing.T) { // A9-D24：額度與扣額度同一個 transaction、買家鎖之後
	f := newFixture(t)
	q := &txQuota{conn: f.d.Conn}
	if _, err := newSubmitter(f, q).Submit(context.Background(), turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: uuid.NewV7(), Locale: uitext.ZhTW}); err != nil {
		t.Fatal(err)
	}
	if q.calls != 1 || !q.inTx || q.unlocked {
		t.Fatalf("calls=%d inTx=%v unlocked=%v，額度要在 transaction 內、買家的送出鎖之後讀一次", q.calls, q.inTx, q.unlocked)
	}
}

func TestReplayOfADeletedConversationIsNotFound(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	s := newSubmitter(f, &txQuota{})
	cid := uuid.NewV7()
	first, err := s.Submit(ctx, turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: cid, Locale: uitext.ZhTW})
	if err != nil {
		t.Fatal(err)
	}
	if again, err := s.Submit(ctx, turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: cid, Locale: uitext.ZhTW}); err != nil || again != first {
		t.Fatalf("重送 = %+v, %v，要回放 %+v", again, err, first)
	}
	n, err := store.New(f.d.Conn(ctx)).SoftDeleteConversation(ctx, store.SoftDeleteConversationParams{ID: first.ConversationID, BuyerID: buyer, Now: &now})
	if err != nil || n != 1 {
		t.Fatalf("刪除對話 = %d, %v", n, err)
	}
	_, err = s.Submit(ctx, turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: cid, Locale: uitext.ZhTW})
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("對話已刪除後重送 = %v，要 NOT_FOUND", err)
	}
}

func TestReplayDoesNotReadTheQuotaAndMismatchesAreRejected(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	q := &txQuota{}
	s := newSubmitter(f, q)
	cid := uuid.NewV7()
	first, err := s.Submit(ctx, turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: cid, Locale: uitext.ZhTW})
	if err != nil {
		t.Fatal(err)
	}
	dead := newSubmitter(f, failingQuota{})
	if again, err := dead.Submit(ctx, turn.SubmitInput{BuyerID: buyer, Message: "要蘋果", ClientRequestID: cid, Locale: uitext.ZhTW}); err != nil || again != first {
		t.Fatalf("額度來源失敗時重送 = %+v, %v，要回放 %+v", again, err, first)
	}
	if _, err := s.Submit(ctx, turn.SubmitInput{BuyerID: buyer, Message: "要香蕉", ClientRequestID: cid, Locale: uitext.ZhTW}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("內容不同的重送 = %v，要 INVALID_ARGUMENT", err)
	}
	other := uuid.NewV7()
	if _, err := s.Submit(ctx, turn.SubmitInput{BuyerID: buyer, RequestID: &other, Message: "要蘋果", ClientRequestID: cid, Locale: uitext.ZhTW}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("重送指定不存在的需求 = %v，要 NOT_FOUND", err)
	}
}

type failingQuota struct{}

func (failingQuota) TurnQuota(context.Context, int64) (turn.Quota, error) {
	return turn.Quota{}, errors.New("額度來源不可用")
}
