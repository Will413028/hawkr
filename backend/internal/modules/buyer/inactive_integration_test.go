//go:build integration

package buyer_test

import (
	"context"
	"strings"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// pub_inactive_buyers 是「買家此刻不是營運中」唯一的定義：ACTIVE 不在其中，進入刪除中與關閉之後都在。BuyerDeletionRequested
// 只在 ACTIVE 轉成 PENDING_DELETION 的那一次發布，重送與之後的關閉不再發布。
func TestInactiveBuyersAndDeletionRequestedEvent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	active := profile(t, f, 71)
	leaving := profile(t, f, 72)
	inactive := func(id int64) int64 {
		return count(t, f, "SELECT count(*) FROM buyer.pub_inactive_buyers() b WHERE b.buyer_id = $1", id)
	}
	var requested []BuyerDeletionRequested
	events.Subscribe(f.bus, func(_ context.Context, e BuyerDeletionRequested) error {
		requested = append(requested, e)
		return nil
	})
	if inactive(active.BuyerID) != 0 || inactive(leaving.BuyerID) != 0 {
		t.Fatal("營運中的買家不該在 pub_inactive_buyers")
	}

	must(t, f.module.BeginDeletion(ctx, leaving.BuyerID))
	if len(requested) != 1 || requested[0].BuyerID != leaving.BuyerID || requested[0].AccountID != leaving.AccountID ||
		!requested[0].OccurredAt.Equal(f.now) || requested[0].EventID.String() == "" {
		t.Fatalf("進入刪除中要在同一個 transaction 發一次 BuyerDeletionRequested：%+v", requested)
	}
	if inactive(leaving.BuyerID) != 1 || inactive(active.BuyerID) != 0 {
		t.Fatal("刪除中的買家要在 pub_inactive_buyers，其他買家不受影響")
	}

	must(t, f.module.BeginDeletion(ctx, leaving.BuyerID))
	must(t, f.module.CloseForDeletion(ctx, leaving.BuyerID))
	if len(requested) != 1 {
		t.Fatalf("重送與關閉不再發布 BuyerDeletionRequested：%d 次", len(requested))
	}
	if inactive(leaving.BuyerID) != 1 {
		t.Fatal("已關閉的買家要在 pub_inactive_buyers")
	}
}

// LockActive 是其他模組的買家寫入與刪帳序列化的鎖：營運中通過、刪除中與已關閉被拒、不在 transaction 內是呼叫端的錯誤，
// 而且持有共享鎖的 transaction 提交之前，BeginDeletion 的排他鎖拿不到。
func TestLockActiveSerializesWithBeginDeletion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b := profile(t, f, 81)
	pending := profile(t, f, 82)
	closed := profile(t, f, 83)
	must(t, f.module.BeginDeletion(ctx, pending.BuyerID))
	must(t, f.module.CloseForDeletion(ctx, closed.BuyerID))

	if err := f.module.LockActive(ctx, b.BuyerID); err == nil {
		t.Fatal("沒有 transaction 時要拒絕：鎖在語句結束就放掉")
	}
	must(t, f.database.InTx(ctx, func(ctx context.Context) error { return f.module.LockActive(ctx, b.BuyerID) }))
	for _, c := range []struct {
		id   int64
		kind apperr.Kind
	}{{pending.BuyerID, apperr.KindFailedPrecondition}, {closed.BuyerID, apperr.KindNotFound}, {999999, apperr.KindNotFound}} {
		err := f.database.InTx(ctx, func(ctx context.Context) error { return f.module.LockActive(ctx, c.id) })
		if apperr.KindOf(err) != c.kind {
			t.Fatalf("買家 %d：要 %v，得到 %v", c.id, c.kind, err)
		}
	}

	done := make(chan error, 1)
	err := f.database.InTx(ctx, func(txCtx context.Context) error {
		if err := f.module.LockActive(txCtx, b.BuyerID); err != nil {
			return err
		}
		go func() { done <- f.module.BeginDeletion(ctx, b.BuyerID) }()
		if err := waitForBuyerLock(ctx, f); err != nil {
			return err
		}
		select {
		case err := <-done:
			t.Errorf("共享鎖還在時 BeginDeletion 不該完成：%v", err)
		default:
		}
		return nil
	})
	must(t, err)
	must(t, <-done)
	if err := f.database.InTx(ctx, func(ctx context.Context) error { return f.module.LockActive(ctx, b.BuyerID) }); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("刪帳申請之後的寫入要被拒絕：%v", err)
	}
}

// 守門（LockActive，FOR KEY SHARE）與同一個買家的業務寫入（FOR NO KEY UPDATE）不互相等待：守門的 transaction 還沒結束，
// 業務寫入就能完成。業務寫入若改回 FOR UPDATE，會等到 context 的期限而失敗。
func TestOwnerGuardDoesNotBlockBusinessWritesOnTheSameBuyer(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	b := profile(t, f, 73)
	name := "Concurrent"
	err := f.database.InTx(ctx, func(txCtx context.Context) error {
		if err := f.module.LockActive(txCtx, b.BuyerID); err != nil {
			return err
		}
		writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := f.module.UpdateProfile(writeCtx, UpdateProfileInput{BuyerID: b.BuyerID, ExpectedVersion: b.Version, DisplayName: &name})
		return err
	})
	must(t, err)
}

// pub_closed_buyers 只含已關閉（CLOSED）的買家：營運中、刪除中、不存在都不在其中。LockNotClosed 在沒有 transaction 時拒絕，
// 營運中與刪除中放行，已關閉與不存在回 NotFound。
func TestClosedBuyersAndLockNotClosed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	active := profile(t, f, 91)
	pending := profile(t, f, 92)
	closed := profile(t, f, 93)
	must(t, f.module.BeginDeletion(ctx, pending.BuyerID))
	must(t, f.module.CloseForDeletion(ctx, closed.BuyerID))
	isClosed := func(id int64) int64 {
		return count(t, f, "SELECT count(*) FROM buyer.pub_closed_buyers() b WHERE b.buyer_id = $1", id)
	}
	if isClosed(active.BuyerID) != 0 || isClosed(pending.BuyerID) != 0 || isClosed(999999) != 0 {
		t.Fatal("營運中、刪除中與不存在的買家不該在 pub_closed_buyers")
	}
	if isClosed(closed.BuyerID) != 1 {
		t.Fatal("已關閉的買家要在 pub_closed_buyers")
	}
	if err := f.module.LockNotClosed(ctx, active.BuyerID); err == nil {
		t.Fatal("沒有 transaction 時要拒絕：鎖在語句結束就放掉")
	}
	for _, c := range []struct {
		id int64
		ok bool
	}{{active.BuyerID, true}, {pending.BuyerID, true}, {closed.BuyerID, false}, {999999, false}} {
		err := f.database.InTx(ctx, func(ctx context.Context) error { return f.module.LockNotClosed(ctx, c.id) })
		if c.ok {
			must(t, err)
		} else if apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("買家 %d：要 NotFound，得到 %v", c.id, err)
		}
	}
}

// LockNotClosed 持有期間，BeginDeletion 與 CloseForDeletion 的 FOR UPDATE 要等它所在的 transaction 提交；提交後轉移完成，
// 下一次 LockNotClosed 對已關閉的買家回 NotFound。
func TestLockNotClosedBlocksBeginDeletionUntilCommit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b := profile(t, f, 94)
	for _, step := range []struct {
		name string
		run  func(context.Context, int64) error
	}{
		{"BeginDeletion", f.module.BeginDeletion},
		{"CloseForDeletion", f.module.CloseForDeletion},
	} {
		done := make(chan error, 1)
		err := f.database.InTx(ctx, func(txCtx context.Context) error {
			if err := f.module.LockNotClosed(txCtx, b.BuyerID); err != nil {
				return err
			}
			go func() { done <- step.run(ctx, b.BuyerID) }()
			if err := waitForBuyerLock(ctx, f); err != nil {
				return err
			}
			select {
			case err := <-done:
				t.Errorf("%s：守門鎖還在時不該完成：%v", step.name, err)
			default:
			}
			return nil
		})
		must(t, err)
		must(t, <-done)
	}
	err := f.database.InTx(ctx, func(ctx context.Context) error { return f.module.LockNotClosed(ctx, b.BuyerID) })
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("關閉之後的寫入要被拒絕：%v", err)
	}
}

// 消費端的 EXISTS 要能把 pub_closed_buyers() 內聯（單一 SELECT，計畫測試鎖住）：計畫裡沒有 Function Scan。
func TestPubClosedBuyersIsInlined(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	rows, err := f.database.Conn(ctx).Query(ctx, "EXPLAIN SELECT 1 FROM buyer.buyers o WHERE o.buyer_id = 1 AND NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() b WHERE b.buyer_id = o.buyer_id)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, line)
	}
	if got := strings.Join(plan, "\n"); strings.Contains(got, "Function Scan") || strings.Contains(got, "pub_closed_buyers") {
		t.Fatalf("pub_closed_buyers 沒有被內聯：\n%s", got)
	}
}
