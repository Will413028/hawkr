//go:build integration

package seller_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
)

// purgeHold 停住一次清理：某一段清理的第一個語句鎖住候選之後（hook）停在那裡，直到 release。
type purgeHold struct {
	locked  chan struct{}
	proceed chan struct{}
	done    chan struct{}
	n       int
	err     error
}

// holdPurgeAfterCandidates 在背景跑一次 PurgePII，停在第一段清理（退場）「候選已鎖住、再判斷之前」；回傳時清理已經停在那裡。
func (f *fixture) holdPurgeAfterCandidates(t *testing.T) *purgeHold {
	t.Helper()
	return f.holdPurgeStage(t, f.module.SetPurgeAfterCandidates)
}

// holdPurgeStageTwoAfterCandidates 同上，停在第二段清理（個資）。
func (f *fixture) holdPurgeStageTwoAfterCandidates(t *testing.T) *purgeHold {
	t.Helper()
	return f.holdPurgeStage(t, f.module.SetPurgeAfterPurgeCandidates)
}

func (f *fixture) holdPurgeStage(t *testing.T, set func(func())) *purgeHold {
	t.Helper()
	h := &purgeHold{locked: make(chan struct{}), proceed: make(chan struct{}), done: make(chan struct{})}
	var once sync.Once
	set(func() {
		once.Do(func() { close(h.locked) })
		<-h.proceed
	})
	t.Cleanup(func() { set(nil) })
	go func() {
		defer close(h.done)
		h.n, h.err = f.module.PurgePII(context.Background())
	}()
	select {
	case <-h.locked:
	case <-h.done:
		t.Fatalf("清理沒有候選，hook 沒有執行（n=%d err=%v）", h.n, h.err)
	case <-time.After(20 * time.Second):
		t.Fatal("清理沒有到達 hook")
	}
	return h
}

// release 放行清理並等它結束。
func (h *purgeHold) release(t *testing.T) {
	t.Helper()
	close(h.proceed)
	select {
	case <-h.done:
	case <-time.After(20 * time.Second):
		t.Fatal("清理沒有結束")
	}
	must(t, h.err)
}

// lockWaiters 是此刻在等鎖的連線數。
func (f *fixture) lockWaiters(t *testing.T) int64 {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'")
}

// waitBlocked 等到有一條連線在等鎖，或 done 結束（沒有被擋住）；回傳是否被擋住。
func (f *fixture) waitBlocked(t *testing.T, done <-chan struct{}) bool {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-done:
			return false
		case <-deadline:
			t.Fatal("等不到被擋住，也等不到結束")
		case <-time.After(20 * time.Millisecond):
			if f.lockWaiters(t) > 0 {
				return true
			}
		}
	}
}

// 清理鎖住候選之後再判斷一次：寫入 hold 的一方（這裡是終止措施的待處理申覆，最壞的情形：不鎖店家列）在第一個語句開始之後 commit，
// 第二個語句的新快照看得到它，這家店不清（step10e-3cb §2.4 情形 2、§7.3）。
// mutation：拿掉 purgeBatch 的再判斷（直接清第一次的候選）→ 店家被清，斷言失敗。
func TestPurgeRechecksTheAppealHoldAfterLockingTheCandidates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, old := f.open(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	closedAt := f.now
	f.now = closedAt.Add(10 * 24 * time.Hour) // 終止晚於舊 stint 關店：申覆期限（生效後 184 天）才晚於保留期（關店後 184 天，同值）
	second, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	m := f.terminate(t, second.StoreID, GroundOtherSerious)
	f.effect(t, m)
	f.now = closedAt.Add(StoreRetention + time.Hour) // 舊 stint 關閉超過保留期，終止的申覆期限還沒到

	writer, err := f.db.Pool().Begin(ctx) // 連線 B：插入待處理的申覆，不鎖店家列、不 commit
	must(t, err)
	defer func() { _ = writer.Rollback(ctx) }()
	_, err = writer.Exec(ctx, `INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, basis)
		SELECT gen_random_uuid(), id, 'APPEAL', 'PENDING', '異議', current_revision, $2, basis FROM seller.measures WHERE id = $1`, m.ID, f.now)
	must(t, err)

	h := f.holdPurgeAfterCandidates(t) // 連線 A：第一個語句看不到 B 的插入，鎖住舊 stint
	must(t, writer.Commit(ctx))        // B 在 A 的第一個語句之後 commit
	h.release(t)
	if h.n != 0 || f.purged(t, old.StoreID) {
		t.Fatalf("再判斷看到 hold，舊 stint 不能被清（n=%d）", h.n)
	}
}

// pushBackAppealDeadline 以另一條連線 commit 一個「帳號上更晚才生效的終止合作」（把唯一一筆終止的 starts_at 與 created_at 移到現在），讓時間條件
// （關店後 hours 與帳號上最晚的申覆期限，RetireDueStores 的 due_after）變成不成立；它不碰店家列，所以不被清理鎖住的候選擋住。
func (f *fixture) pushBackAppealDeadline(t *testing.T, measureID any) {
	t.Helper()
	_, err := f.db.Pool().Exec(context.Background(), "UPDATE seller.measures SET starts_at = $2, created_at = $2 WHERE id = $1", measureID, f.now)
	must(t, err)
}

// 退場（第一段）鎖住候選之後再判斷一次「時間」條件：第一個語句開始之後才 commit 的、讓申覆期限往後的寫入（帳號上更晚生效的終止，不鎖店家列），
// 第二個語句的新快照看得到，這家店這一輪不退場。
// mutation：拿掉 retireBatch 第二次 RetireDueStores（直接用第一次的候選）→ 店家被退場，斷言失敗。
func TestRetireRechecksTheDueTimeAfterLockingTheCandidates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s, m := f.terminated(t, GroundCeasedBusiness)
	f.now = m.StartsAt.Add(4416*time.Hour + time.Hour) // 申覆期限與關店後的期間都已過：到期
	h := f.holdPurgeAfterCandidates(t)
	f.pushBackAppealDeadline(t, m.ID)
	h.release(t)
	if f.retired(t, s.StoreID) {
		t.Fatal("再判斷看到申覆期限往後，這一輪不能退場")
	}
}

// 個資清理（第二段）同上：退場之後、鎖住候選之後才 commit 的更晚的申覆期限，第二次 RetireDueStores（第二段）看得到，這一輪不清。
// mutation：拿掉 purgeBatch 第二次 RetireDueStores（直接用第一次的候選）→ 個資被清，斷言失敗。
func TestPurgeRechecksTheDueTimeAfterLockingTheCandidates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s, m := f.terminated(t, GroundCeasedBusiness)
	f.now = m.StartsAt.Add(4416*time.Hour + time.Hour)
	h := f.holdPurgeStageTwoAfterCandidates(t) // 同一輪先退場（第一段，到期時間相同），再停在第二段
	f.pushBackAppealDeadline(t, m.ID)
	h.release(t)
	if !f.retired(t, s.StoreID) {
		t.Fatal("第一段已退場")
	}
	if f.purged(t, s.StoreID) {
		t.Fatal("再判斷看到申覆期限往後，這一輪不能清個資")
	}
}

// 退場（第一段清理）先鎖住店家、申覆等鎖：申覆醒來時店家已退場，即使申覆這一方的時鐘比清理早（多實例的時鐘差，仍在期限前），
// 也要回「申覆期限已過」，不能在已退場的店家上留下待處理的申覆（§2.4 的第二個窗）。
// mutation：拿掉 submitRequest 鎖後的 retired_at 檢查 → 申覆成功、留下待處理的申覆，斷言失敗（結果不依賴時鐘）。
func TestAppealAfterTheRetirementLockedTheStoreSeesItRetired(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m := f.terminated(t, GroundCeasedBusiness)
	deadline := m.StartsAt.Add(4416 * time.Hour)
	keys, mac := sealtest.Keys(t)
	early := f.newModuleClock(t, f.payuni, f.jobs, keys, mac, func() time.Time { return deadline.Add(-time.Hour) })
	f.now = deadline.Add(time.Millisecond) // A 的時鐘：申覆期限已過，清理到期

	h := f.holdPurgeAfterCandidates(t)
	var appealErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, appealErr = early.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "異議"})
	}()
	if !f.waitBlocked(t, done) {
		t.Fatalf("申覆要被清理鎖住的店家列擋住：%v", appealErr)
	}
	h.release(t)
	<-done
	if !f.retired(t, s.StoreID) {
		t.Fatal("店家要已退場")
	}
	if apperr.KindOf(appealErr) != apperr.KindFailedPrecondition {
		t.Fatalf("退場之後的申覆要回「申覆期限已過」（FailedPrecondition）：%v", appealErr)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_requests WHERE measure_id = $1", m.ID); n != 0 {
		t.Fatalf("已退場的店家上不能留下申覆：%d", n)
	}
}

// 帳號早先自己關閉、已退場的舊 stint，不影響之後新店被終止的申覆：申覆期限內的申覆照常成功（鎖後的 retired_at 檢查只看措施所在的店家）。
// mutation：把檢查擴到帳號全部的 stint → 這裡申覆被拒，斷言失敗。
func TestAppealStillWorksWhenAnOlderStintOfTheAccountWasRetired(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, old := f.open(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetireAfter + time.Second)
	_, err = f.module.PurgePII(ctx)
	must(t, err)
	if !f.retired(t, old.StoreID) {
		t.Fatal("舊 stint 要已退場")
	}
	second, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	m := f.terminate(t, second.StoreID, GroundOtherSerious)
	f.effect(t, m)
	f.now = f.now.Add(24 * time.Hour)
	got := f.appeal(t, accountID, f.measureOf(t, m.ID), "異議")
	if got.Appeal() == nil {
		t.Fatalf("申覆要成功：%+v", got)
	}
}

// 停權沒有申覆期限，「已退場 ⇒ 申覆期限已過」不成立：賣家自己關店、關店 30 天後退場，帳號層級的停權仍然生效，申覆照常成功
// （鎖後的 retired_at 檢查只用在 closed_by 是 TERMINATION 的店家上的終止合作）。
// mutation：把檢查放寬回「措施所在店家只要已退場就拒絕」→ 申覆被拒，斷言失敗。
func TestSuspensionAppealStillWorksAfterTheSelfClosedStoreWasRetired(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	m := f.suspend(t, s.StoreID)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetireAfter + time.Second)
	_, err = f.module.PurgePII(ctx)
	must(t, err)
	if !f.retired(t, s.StoreID) {
		t.Fatal("自己關閉的店家要已退場")
	}
	got := f.measureOf(t, m.ID)
	if !got.CanAppeal() {
		t.Fatalf("停權沒有期限，讀取模型可申覆：%v", got.AppealUnavailableReasons())
	}
	got = f.appeal(t, accountID, got, "異議")
	if got.Appeal() == nil {
		t.Fatalf("申覆要成功：%+v", got)
	}
}

// 預告期間內賣家自己關店（closed_by 是 ACCOUNT_OWNER，退場只看關店後 30 天、不看申覆期限）：退場時終止的申覆期限（生效後 4416 小時）還沒到，
// 「已退場」不能證明期限已過，申覆照常成功（與讀取模型的 canAppeal 一致）。
// mutation：同上。
func TestNoticeTerminationAppealStillWorksAfterTheSelfClosedStoreWasRetired(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	notice := f.terminate(t, s.StoreID, GroundNotice)
	f.now = f.now.Add(time.Hour)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetireAfter + time.Second)
	_, err = f.module.PurgePII(ctx)
	must(t, err)
	if !f.retired(t, s.StoreID) {
		t.Fatal("自己關閉的店家要已退場")
	}
	got := f.measureOf(t, notice.ID)
	if !got.CanAppeal() {
		t.Fatalf("申覆期限還沒到，讀取模型可申覆：%v", got.AppealUnavailableReasons())
	}
	got = f.appeal(t, accountID, got, "異議")
	if got.Appeal() == nil {
		t.Fatalf("申覆要成功：%+v", got)
	}
}

// 關聯終止的來源申覆要鎖被關聯帳號的店家列：A（被關聯帳號的舊 stint 的退場）鎖住候選之後，對來源措施的申覆被擋在被關聯帳號的店家列上，
// 等退場 commit 才繼續（申覆太晚，被關聯帳號的舊 stint 已依期限退場，來源的申覆照常成功，§2.4 情形 3 與第三個窗的第 4 點）；申覆 commit 之後
// legal hold 成立，同一次清理的第二段不清被關聯帳號的個資。
// mutation：申覆只鎖來源帳號 → 申覆沒被擋住，斷言失敗；StoreHolds 不看關聯終止的來源 → 第二段把個資清掉，斷言失敗。
func TestLinkedSourceAppealBlocksThePurgeOfTheLinkedAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountA, sA, _ := f.submitted(t, idA, bankAcct)
	accountB, sB, _ := f.submitted(t, idA, "999888777666")
	_, err := f.module.CloseCurrent(ctx, accountB)
	must(t, err)
	closedAt := f.now
	f.now = closedAt.Add(10 * 24 * time.Hour) // 來源的終止晚於 B 關店：來源的申覆期限（生效後 184 天）才晚於 B 的保留期（關店後 184 天，同值）
	source := f.terminate(t, sA.StoreID, GroundFraud)
	_, err = f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID })
	must(t, err)
	f.effect(t, source)
	f.now = closedAt.Add(StoreRetention + time.Hour) // B 的舊 stint 關閉超過保留期；來源的申覆期限還沒到

	h := f.holdPurgeAfterCandidates(t)
	cur := f.measureOf(t, source.ID)
	var appealErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, appealErr = f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountA, MeasureID: source.ID, ExpectedVersion: cur.SellerVersion(), Reason: "異議"})
	}()
	blocked := f.waitBlocked(t, done)
	h.release(t)
	<-done
	if !blocked {
		t.Fatalf("來源申覆要被被關聯帳號的店家列擋住（它鎖了被關聯帳號）：%v", appealErr)
	}
	must(t, appealErr)
	if !f.retired(t, sB.StoreID) || f.purged(t, sB.StoreID) {
		t.Fatalf("申覆晚於退場的鎖：被關聯帳號的舊 stint 已退場（%v），個資被來源的申覆擋下（purged=%v）", f.retired(t, sB.StoreID), f.purged(t, sB.StoreID))
	}
	if f.count(t, "SELECT count(*) FROM seller.measure_requests WHERE measure_id = $1 AND state = 'PENDING'", source.ID) != 1 {
		t.Fatal("來源的申覆要成功")
	}
}

// 反方向：申覆先鎖住（來源與被關聯帳號的店家列），清理的第一個語句 SKIP LOCKED 跳過被關聯帳號的舊 stint，不被擋住也不清；
// 申覆 commit 之後 hold 成立，下一次清理仍不清（§2.4 情形 3、§7.3 的「B 先鎖住、A 跳過」）。
// mutation：申覆預讀與重讀都不含被關聯帳號（只鎖來源帳號）→ 清理沒被跳過，舊 stint 被清，斷言失敗。
func TestLinkedSourceAppealLockedFirstMakesThePurgeSkipTheLinkedAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountA, sA, _ := f.submitted(t, idA, bankAcct)
	accountB, sB, _ := f.submitted(t, idA, "999888777666")
	_, err := f.module.CloseCurrent(ctx, accountB)
	must(t, err)
	closedAt := f.now
	f.now = closedAt.Add(10 * 24 * time.Hour) // 來源的終止晚於 B 關店：來源的申覆期限（生效後 184 天）才晚於 B 的保留期（關店後 184 天，同值）
	source := f.terminate(t, sA.StoreID, GroundFraud)
	_, err = f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID })
	must(t, err)
	f.effect(t, source)
	f.now = closedAt.Add(StoreRetention + time.Hour)

	locked, proceed, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.module.SetRequestHooks(nil, func() {
		once.Do(func() { close(locked) })
		<-proceed
	})
	t.Cleanup(func() { f.module.SetRequestHooks(nil, nil) })
	cur := f.measureOf(t, source.ID)
	var appealErr error
	go func() {
		defer close(done)
		_, appealErr = f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountA, MeasureID: source.ID, ExpectedVersion: cur.SellerVersion(), Reason: "異議"})
	}()
	select {
	case <-locked:
	case <-done:
		t.Fatalf("申覆沒有到達鎖之後的 hook：%v", appealErr)
	case <-time.After(20 * time.Second):
		t.Fatal("申覆沒有到達 hook")
	}
	released := false
	release := func() {
		if !released {
			released = true
			close(proceed)
		}
	}
	defer release()

	type result struct {
		n   int
		err error
	}
	res := make(chan result, 1)
	go func() {
		n, err := f.module.PurgePII(ctx)
		res <- result{n, err}
	}()
	select {
	case r := <-res:
		must(t, r.err)
		if r.n != 0 || f.purged(t, sB.StoreID) {
			t.Fatalf("申覆先鎖住被關聯帳號的店家列，清理要跳過它（n=%d）", r.n)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("清理被申覆擋住了：第一個語句要 SKIP LOCKED 跳過而不是等")
	}
	release()
	<-done
	must(t, appealErr)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 0 || f.purged(t, sB.StoreID) {
		t.Fatalf("申覆 commit 之後 hold 成立，被關聯帳號的舊 stint 仍不能清（n=%d）", n)
	}
}

// retryScope：不加鎖的預讀之後、鎖之前剛好新增一筆以這筆措施為來源的關聯終止（另一個帳號），重讀發現超出已鎖的範圍，整個 transaction 重來，
// 第二次預讀就含新帳號並鎖住它。
// mutation：retryScope 不重試（第一個 errScopeChanged 就回）→ 申覆失敗；預讀不含關聯帳號且不重讀 → 第二次沒有鎖到新帳號，NOWAIT 斷言失敗。
func TestAppealRetriesWhenALinkedTerminationAppearsAfterThePreRead(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountA, sA, _ := f.submitted(t, idA, bankAcct)
	_, sB, _ := f.submitted(t, idA, "999888777666")
	source := f.terminate(t, sA.StoreID, GroundFraud)
	f.effect(t, source)
	f.now = f.now.Add(24 * time.Hour)

	var preReads, locks int
	var lockErr error
	f.module.SetRequestHooks(func() {
		preReads++
		if preReads == 1 { // 第一次預讀之後、鎖之前：新增關聯終止（自己的 transaction，commit）
			_, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &source.ID })
			must(t, err)
		}
	}, func() {
		locks++
		// 鎖都成功之後：被關聯帳號的店家列要已被這個 transaction 鎖住（另一條連線 NOWAIT 拿不到）
		conn, err := f.db.Pool().Begin(ctx)
		must(t, err)
		defer func() { _ = conn.Rollback(ctx) }()
		_, lockErr = conn.Exec(ctx, "SELECT 1 FROM seller.stores WHERE store_id = $1 FOR UPDATE NOWAIT", sB.StoreID)
	})
	t.Cleanup(func() { f.module.SetRequestHooks(nil, nil) })

	cur := f.measureOf(t, source.ID)
	got, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountA, MeasureID: source.ID, ExpectedVersion: cur.SellerVersion(), Reason: "異議"})
	must(t, err)
	if got.Appeal() == nil {
		t.Fatalf("申覆要成功：%+v", got)
	}
	if preReads != 2 || locks != 1 {
		t.Fatalf("要重來一次：預讀 %d 次（要 2）、鎖成功 %d 次（要 1）", preReads, locks)
	}
	var pgErr *pgconn.PgError
	if !errors.As(lockErr, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("重來之後的鎖要含新增的被關聯帳號的店家列：%v", lockErr)
	}
}

// 不加鎖的預讀之後、鎖之前，這筆終止的店家被清理、終止又被撤銷（清理不重開已清理的店家，關店紀錄還在）：申覆的失敗原因要和讀模型同一個順序
// （REVOKED 先於 APPEAL_WINDOW_ENDED），不能因為「已清理」先回「申覆期限已過」。
// mutation：把鎖後的 pii_purged_at 檢查移回可用性檢查之前 → 回的是「申覆期限已過」，斷言失敗。
func TestAppealOfARevokedTerminationOnAPurgedStoreReportsRevokedFirst(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m := f.terminated(t, GroundCeasedBusiness)
	deadline := m.StartsAt.Add(4416 * time.Hour)
	var revokeErr error
	f.module.SetRequestHooks(func() {
		f.now = deadline.Add(time.Millisecond)
		if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
			t.Errorf("店家要被清理（n=%d err=%v）", n, err)
		}
		cur := f.measureOf(t, m.ID)
		_, revokeErr = f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: cur.Version, Reason: RevokeAdminCorrection})
	}, nil)
	t.Cleanup(func() { f.module.SetRequestHooks(nil, nil) })

	_, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "異議"})
	must(t, revokeErr)
	if !f.purged(t, s.StoreID) {
		t.Fatal("店家要已被清理")
	}
	reasons := f.measureOf(t, m.ID).AppealUnavailableReasons()
	if len(reasons) < 2 || reasons[0] != AppealReasonRevoked || reasons[1] != AppealReasonWindowEnded {
		t.Fatalf("讀模型的原因要是 [REVOKED APPEAL_WINDOW_ENDED]：%v", reasons)
	}
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "這筆措施已撤銷，不需要申覆") {
		t.Fatalf("命令要回讀模型的第一個原因（已撤銷），不是「申覆期限已過」：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_requests WHERE measure_id = $1", m.ID); n != 0 {
		t.Fatalf("不能留下申覆：%d", n)
	}
}
