//go:build integration

package accountsuspension

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type testQueue struct{}

func (testQueue) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }
func (testQueue) EnqueueReport(context.Context, river.JobArgs, *river.InsertOpts) (bool, error) {
	return true, nil
}

type noRegistry struct{}

func (noRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []int64) (map[int64]int64, error) { return nil, nil }

type testMedia struct{}

func (testMedia) Finalize(context.Context, media.Uploader, string, string) (media.Asset, error) {
	return media.Asset{}, errors.New("not used")
}
func (testMedia) Link(context.Context, media.Ref, []string) error   { return nil }
func (testMedia) Unlink(context.Context, media.Ref, []string) error { return nil }
func (testMedia) ReadSecure(context.Context, string, media.Ref) (media.SecureAsset, error) {
	return media.SecureAsset{}, errors.New("not used")
}

type noOrders struct{}

func (noOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (noOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

type fixture struct {
	db       *db.DB
	identity *identity.Module
	buyer    *buyer.Module
	seller   *seller.Module
	deletion *accountdeletion.Module
	module   *Module

	mu          sync.Mutex
	suspended   []Suspended
	unsuspended []Unsuspended
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
	))
	bus := events.NewBus()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := testQueue{}
	id := identity.New(identity.Deps{DB: d, Events: bus, Jobs: q, Now: time.Now, Logger: log})
	b := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: q, Now: time.Now, Logger: log})
	kycKeys, kycMAC := sealtest.Keys(t)
	sl := seller.New(seller.Deps{KYCKeys: kycKeys, KYCMAC: kycMAC, DB: d, Events: bus, Jobs: q, Identity: id, Media: testMedia{}, Orders: noOrders{}, ProductStores: noProducts{}, Registry: noRegistry{}, Now: time.Now, Logger: log})
	f := &fixture{
		db: d, identity: id, buyer: b, seller: sl,
		deletion: accountdeletion.New(accountdeletion.Deps{DB: d, Identity: id, Buyer: b, Seller: sl, Orders: noOrders{}, Now: time.Now}),
		module:   New(Deps{DB: d, Events: bus, Identity: id, Seller: sl, Logger: log}),
	}
	events.Subscribe(bus, func(ctx context.Context, e Suspended) error {
		if _, ok := db.TxFrom(ctx); !ok {
			return errors.New("事件不在 transaction 內")
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.suspended = append(f.suspended, e)
		return nil
	})
	events.Subscribe(bus, func(ctx context.Context, e Unsuspended) error {
		if _, ok := db.TxFrom(ctx); !ok {
			return errors.New("事件不在 transaction 內")
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.unsuspended = append(f.unsuspended, e)
		return nil
	})
	events.Subscribe(bus, func(ctx context.Context, e identity.AccountCreated) error {
		return b.OnAccountCreated(ctx, e)
	})
	return f
}

// account 走正式的建帳事件建立帳號與它唯一的買家。
func (f *fixture) account(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := f.db.Conn(ctx).QueryRow(ctx, "INSERT INTO identity.accounts (clerk_user_id) VALUES ($1) RETURNING id", uuid.NewV7().String()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := f.buyer.OnAccountCreated(ctx, identity.AccountCreated{AccountID: id, Name: "測試買家"}); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *fixture) open(t *testing.T, accountID int64) (seller.Store, error) {
	t.Helper()
	return f.seller.Apply(context.Background(), seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: accountID, StoreName: "測試小店"})
}

func (f *fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *fixture) suspendedAccount(t *testing.T, id int64) bool {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM identity.accounts WHERE id = $1 AND suspended_at IS NOT NULL`, id) == 1
}

func (f *fixture) records(t *testing.T, id int64) (all, unreleased int) {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM identity.account_suspensions WHERE account_id = $1`, id),
		f.count(t, `SELECT count(*) FROM identity.account_suspensions WHERE account_id = $1 AND released_at IS NULL`, id)
}

func (f *fixture) events() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.suspended), len(f.unsuspended)
}

func (f *fixture) hasStore(t *testing.T, id int64) bool {
	t.Helper()
	_, err := f.seller.CurrentStore(context.Background(), id)
	if err != nil && apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatal(err)
	}
	return err == nil
}

func TestSuspendRecordsTheReasonAndPublishesTheEvent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t)
	s, err := f.module.Suspend(ctx, id, Record{Category: policy.Harassment, Reason: "持續騷擾", SourceRef: "case-1"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Category != policy.Harassment || s.Reason != "持續騷擾" || s.SourceRef != "case-1" || s.AccountID != id {
		t.Fatalf("紀錄 = %+v", s)
	}
	if !f.suspendedAccount(t, id) {
		t.Fatal("帳號沒有停權")
	}
	if len(f.suspended) != 1 || f.suspended[0].AccountID != id || f.suspended[0].Category != policy.Harassment ||
		f.suspended[0].SuspensionID != s.ID || !f.suspended[0].SuspendedAt.Equal(s.SuspendedAt) {
		t.Fatalf("Suspended = %+v", f.suspended)
	}
	// 重複停權：回傳原本的紀錄，沒有第二筆紀錄、沒有第二個事件。
	again, err := f.module.Suspend(ctx, id, Record{Category: policy.Spam})
	if err != nil || again.ID != s.ID {
		t.Fatalf("重複停權 = %+v, %v", again, err)
	}
	if all, open := f.records(t, id); all != 1 || open != 1 {
		t.Fatalf("紀錄 %d 筆、未解除 %d 筆", all, open)
	}
	if n, _ := f.events(); n != 1 {
		t.Fatalf("Suspended 事件 %d 個，預期 1", n)
	}
}

func TestSuspendRefusesAnAccountThatHasAStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t)
	if _, err := f.open(t, id); err != nil {
		t.Fatal(err)
	}
	_, err := f.module.Suspend(ctx, id, Record{Category: policy.Fraud})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonAccountHasStore {
		t.Fatalf("有店的帳號停權 = %v，預期 FailedPrecondition 與 ACCOUNT_HAS_STORE", err)
	}
	if f.suspendedAccount(t, id) {
		t.Fatal("有店的帳號被停權")
	}
	if all, _ := f.records(t, id); all != 0 {
		t.Fatalf("拒絕後留下 %d 筆紀錄", all)
	}
	if n, _ := f.events(); n != 0 {
		t.Fatalf("拒絕後發了 %d 個事件", n)
	}
}

func TestSuspendAndUnsuspendRunInTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t)
	boom := errors.New("呼叫方後面的步驟失敗")
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		if _, err := f.module.Suspend(ctx, id, Record{Category: policy.Spam}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if f.suspendedAccount(t, id) {
		t.Fatal("呼叫方回滾後帳號仍停權")
	}
	if all, _ := f.records(t, id); all != 0 {
		t.Fatalf("呼叫方回滾後留下 %d 筆紀錄", all)
	}

	if _, err := f.module.Suspend(ctx, id, Record{Category: policy.Spam}); err != nil {
		t.Fatal(err)
	}
	err = f.db.InTx(ctx, func(ctx context.Context) error {
		if _, err := f.module.Unsuspend(ctx, id, "誤判"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if !f.suspendedAccount(t, id) {
		t.Fatal("解除在呼叫方回滾後仍生效")
	}
	if _, open := f.records(t, id); open != 1 {
		t.Fatalf("未解除的紀錄 %d 筆，預期 1", open)
	}
}

func TestUnsuspendReleasesAndPublishesTheEventOnlyWhenSuspended(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t)
	released, err := f.module.Unsuspend(ctx, id, "沒有停權")
	if err != nil || released {
		t.Fatalf("沒有停權時解除 = %v, %v", released, err)
	}
	if _, u := f.events(); u != 0 {
		t.Fatalf("沒有停權卻發了 %d 個 Unsuspended", u)
	}
	s, err := f.module.Suspend(ctx, id, Record{Category: policy.Hate})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.Unsuspend(ctx, id, " "); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("沒有原因的解除 = %v，預期 invalid input", err)
	}
	if !f.suspendedAccount(t, id) {
		t.Fatal("無效的解除改了狀態")
	}
	released, err = f.module.Unsuspend(ctx, id, "申覆成立")
	if err != nil || !released {
		t.Fatalf("解除 = %v, %v", released, err)
	}
	if f.suspendedAccount(t, id) {
		t.Fatal("解除後仍停權")
	}
	var reason string
	if err := f.db.Conn(ctx).QueryRow(ctx, `SELECT release_reason FROM identity.account_suspensions WHERE id = $1`, s.ID).Scan(&reason); err != nil || reason != "申覆成立" {
		t.Fatalf("解除原因 = %q, %v", reason, err)
	}
	if len(f.unsuspended) != 1 || f.unsuspended[0].AccountID != id || f.unsuspended[0].SuspensionID != s.ID || f.unsuspended[0].Category != policy.Hate {
		t.Fatalf("Unsuspended = %+v", f.unsuspended)
	}
}

// waitForLockWaiter 等到資料庫裡有一個連線正在等鎖。
func waitForLockWaiter[T any](t *testing.T, f *fixture, done <-chan T) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if f.count(t, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`) > 0 {
			return
		}
		select {
		case <-done:
			t.Fatal("應該等鎖的呼叫已經結束")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("沒有任何連線在等鎖")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// 開店的 transaction 還沒 commit 時，停權要等它（兩者持有同一把帳號列鎖），等到之後看得到店家而被拒絕：拿掉 Suspend 的
// LockAccount，停權讀不到未 commit 的店家而成功，這個測試失敗。
func TestSuspendWaitsForAnOpeningStoreAndThenSeesIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t)
	opened, release, holdDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holdDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			if _, err := f.seller.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: id, StoreName: "測試小店"}); err != nil {
				return err
			}
			close(opened)
			<-release
			return nil
		})
	}()
	<-opened
	suspendDone := make(chan error, 1)
	go func() { _, err := f.module.Suspend(ctx, id, Record{Category: policy.Fraud}); suspendDone <- err }()
	waitForLockWaiter(t, f, suspendDone)
	close(release)
	if err := <-holdDone; err != nil {
		t.Fatal(err)
	}
	err := <-suspendDone
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Reason != apperr.ReasonAccountHasStore {
		t.Fatalf("開店 commit 之後的停權 = %v，預期 ACCOUNT_HAS_STORE", err)
	}
	if f.suspendedAccount(t, id) || !f.hasStore(t, id) {
		t.Fatalf("結果不一致：停權=%v 有店=%v", f.suspendedAccount(t, id), f.hasStore(t, id))
	}
}

// 反過來：停權的 transaction 還沒 commit 時，開店要等它，等到之後帳號已停權而被拒絕。
func TestOpeningWaitsForASuspensionAndThenIsRefused(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t)
	suspended, release, holdDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holdDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			if _, err := f.module.Suspend(ctx, id, Record{Category: policy.Fraud}); err != nil {
				return err
			}
			close(suspended)
			<-release
			return nil
		})
	}()
	<-suspended
	openDone := make(chan error, 1)
	go func() { _, err := f.open(t, id); openDone <- err }()
	waitForLockWaiter(t, f, openDone)
	close(release)
	if err := <-holdDone; err != nil {
		t.Fatal(err)
	}
	if err := <-openDone; err == nil {
		t.Fatal("帳號已停權，開店應該被拒絕")
	}
	if !f.suspendedAccount(t, id) || f.hasStore(t, id) {
		t.Fatalf("結果不一致：停權=%v 有店=%v", f.suspendedAccount(t, id), f.hasStore(t, id))
	}
}

// 並行的停權與開店：不論誰先，結果都不是「有店且被停權」。
func TestConcurrentSuspendAndOpenNeverLeaveASuspendedStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for range 15 {
		id := f.account(t)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _, _ = f.module.Suspend(ctx, id, Record{Category: policy.Fraud}) }()
		go func() { defer wg.Done(); <-start; _, _ = f.open(t, id) }()
		close(start)
		wg.Wait()
		if f.suspendedAccount(t, id) && f.hasStore(t, id) {
			t.Fatalf("帳號 %d 有店且被停權", id)
		}
	}
}

// 鎖順序：identity 帳號列 → buyer → seller。停權只用到帳號列與 seller 的讀取，與走完整條路徑的 accountdeletion 並行不死結，
// 也不留下「帳號已刪除卻有未解除的停權紀錄」。
func TestSuspendAndAccountDeletionRunConcurrentlyWithoutDeadlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for range 15 {
		id := f.account(t)
		start := make(chan struct{})
		var wg sync.WaitGroup
		var suspendErr, deleteErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, suspendErr = f.module.Suspend(ctx, id, Record{Category: policy.Spam})
		}()
		go func() {
			defer wg.Done()
			<-start
			_, deleteErr = f.deletion.Request(ctx, accountdeletion.RequestInput{AccountID: id, Requester: accountdeletion.AccountOwner})
		}()
		close(start)
		wg.Wait()
		if ctx.Err() != nil {
			t.Fatal("並行的停權與刪帳逾時：疑似死結")
		}
		if deleteErr != nil {
			t.Fatalf("刪帳失敗：%v", deleteErr)
		}
		if suspendErr != nil && apperr.KindOf(suspendErr) != apperr.KindNotFound {
			t.Fatalf("停權失敗（只允許帳號已刪除的 NotFound）：%v", suspendErr)
		}
		a, err := f.identity.Account(ctx, id)
		if err != nil || !a.Deleted {
			t.Fatalf("帳號沒有刪除：%+v %v", a, err)
		}
		if _, open := f.records(t, id); open != 0 {
			t.Fatalf("已刪除的帳號有 %d 筆未解除的停權紀錄", open)
		}
	}
}
