//go:build integration

package accountdeletion

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/gcis"

	"github.com/riverqueue/river"

	processrequest "github.com/Will413028/hawkr/backend/internal/modules/accountdeletion/internal/request"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// testQueue 記下排入的 job 的 kind。
type testQueue struct {
	mu    sync.Mutex
	kinds []string
}

func (q *testQueue) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.kinds = append(q.kinds, args.Kind())
	return nil
}

func (q *testQueue) EnqueueReport(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (bool, error) {
	return true, q.Enqueue(ctx, args, opts)
}

func (q *testQueue) count(kind string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, k := range q.kinds {
		if k == kind {
			n++
		}
	}
	return n
}

// noProducts 是商品 port 的替身：這裡的測試不彙總瀏覽事件。
// noRegistry 是登記資料來源的替身：這個測試不送公司型申請，來源不會被呼叫。
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

// testOrders 是訂單 port 的替身：買家側（open、err）與店家側（storeOpen、storeErr）各自設定。onStoreCall 在店家側的查詢
// 時執行一次（鎖外），用來製造「查完訂單、進交易之前」的交錯。
type testOrders struct {
	mu          sync.Mutex
	open        bool
	err         error
	onCall      func()
	calls       int
	storeOpen   bool
	storeErr    error
	storeCalls  []int64
	onStoreCall func()
}

func (o *testOrders) HasOpenStoreOrders(_ context.Context, storeID int64) (bool, error) {
	o.mu.Lock()
	o.storeCalls = append(o.storeCalls, storeID)
	hook := o.onStoreCall
	o.onStoreCall = nil
	open, err := o.storeOpen, o.storeErr
	o.mu.Unlock()
	if hook != nil {
		hook()
	}
	return open, err
}

func (o *testOrders) HasOpenOrders(context.Context, int64) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls++
	if o.onCall != nil {
		o.onCall()
	}
	return o.open, o.err
}

type fixture struct {
	db       *db.DB
	identity *identity.Module
	buyer    *buyer.Module
	seller   *seller.Module
	module   *Module
	orders   *testOrders
	queue    *testQueue
	bus      *events.Bus
}

func setup(t *testing.T) *fixture { return setupWith(t) }

// setupWith 是 setup 加上額外要套用 migration 的模組（訂閱者模組的測試用）。
func setupWith(t *testing.T, extra ...migrate.Module) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(append([]migrate.Module{{Name: "identity", Dir: identity.Migrations()}, {Name: "buyer", Dir: buyer.Migrations()}, {Name: "seller", Dir: seller.Migrations()}, {Name: "accountdeletion", Dir: Migrations()}}, extra...)...))
	bus := events.NewBus()
	queue := &testQueue{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	id := identity.New(identity.Deps{DB: d, Events: bus, Jobs: queue, Now: time.Now, Logger: log})
	b := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: queue, Now: time.Now, Logger: log})
	kycKeys, kycMAC := sealtest.Keys(t)
	orders := &testOrders{}
	sl := seller.New(seller.Deps{KYCKeys: kycKeys, KYCMAC: kycMAC, DB: d, Events: bus, Jobs: queue, Identity: id, Media: testMedia{}, Orders: orders, ProductStores: noProducts{}, Registry: noRegistry{}, Now: time.Now, Logger: log})
	m := New(Deps{DB: d, Identity: id, Buyer: b, Seller: sl, Orders: orders, Now: time.Now})
	return &fixture{db: d, identity: id, buyer: b, seller: sl, orders: orders, module: m, queue: queue, bus: bus}
}

// account 走正式的建帳事件建立帳號與它唯一的買家。
func (f *fixture) account(t *testing.T) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	id := f.accountWithoutBuyer(t)
	if err := f.buyer.OnAccountCreated(ctx, identity.AccountCreated{AccountID: id, Name: "測試買家"}); err != nil {
		t.Fatal(err)
	}
	p, err := f.buyer.Profile(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return id, p.BuyerID
}

// accountWithoutBuyer 直接寫入帳號、不建買家，只用來製造被破壞的不變量。
func (f *fixture) accountWithoutBuyer(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := f.db.Conn(ctx).QueryRow(ctx, "INSERT INTO identity.accounts DEFAULT VALUES RETURNING id").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *fixture) count(t *testing.T, accountID int64) int {
	t.Helper()
	var n int
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT count(*) FROM accountdeletion.deletion_requests WHERE account_id=$1", accountID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRequestClosedDeletesAccountAndCompletes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, buyerID := f.account(t)
	state, err := f.module.Request(context.Background(), RequestInput{AccountID: id, Requester: AccountOwner})
	if err != nil || state != Completed {
		t.Fatalf("state=%s err=%v", state, err)
	}
	a, err := f.identity.Account(context.Background(), id)
	if err != nil || !a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	var status string
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT status FROM buyer.buyers WHERE buyer_id=$1", buyerID).Scan(&status); err != nil || status != "CLOSED" {
		t.Fatalf("buyer=%s err=%v", status, err)
	}
	if f.count(t, id) != 1 {
		t.Fatal("request not recorded")
	}
}

// 買家刪帳時兩個方向的封鎖都刪：buyer 擁有的（他發起的兩種，以及以他為對象的買家封鎖買家）與 seller 擁有的（各店對他的封鎖）；
// 無關的封鎖不受影響。拿掉 closeParticipants 對 Seller.ClearBlocksAgainstBuyer 的呼叫，各店對他的封鎖會留下，這個測試失敗。
func TestRequestClosedDeletesBlocksInBothDirections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	id, leaving := f.account(t)
	otherID, other := f.account(t)
	st := f.store(t, otherID)
	for _, err := range []error{
		func() error { _, err := f.seller.BlockBuyer(ctx, st.StoreID, leaving); return err }(),
		func() error { _, err := f.seller.BlockBuyer(ctx, st.StoreID, other); return err }(),
		func() error { _, err := f.buyer.BlockStore(ctx, leaving, st.StoreID); return err }(),
		func() error { _, err := f.buyer.BlockBuyer(ctx, leaving, other); return err }(),
		func() error { _, err := f.buyer.BlockBuyer(ctx, other, leaving); return err }(),
		func() error { _, err := f.buyer.BlockStore(ctx, other, st.StoreID); return err }(),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if state, err := f.module.Request(ctx, RequestInput{AccountID: id, Requester: AccountOwner}); err != nil || state != Completed {
		t.Fatalf("state=%s err=%v", state, err)
	}
	count := func(query string, args ...any) int {
		var n int
		if err := f.db.Conn(ctx).QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count("SELECT count(*) FROM seller.store_blocks WHERE buyer_id = $1", leaving); n != 0 {
		t.Fatalf("各店對他的封鎖要刪：%d", n)
	}
	if n := count("SELECT count(*) FROM buyer.blocked_stores WHERE blocker_buyer_id = $1", leaving); n != 0 {
		t.Fatalf("他封鎖店的列要刪：%d", n)
	}
	if n := count("SELECT count(*) FROM buyer.blocked_buyers WHERE blocker_buyer_id = $1 OR blocked_buyer_id = $1", leaving); n != 0 {
		t.Fatalf("買家封鎖買家的兩個方向都要刪：%d", n)
	}
	if count("SELECT count(*) FROM seller.store_blocks WHERE buyer_id = $1", other) != 1 || count("SELECT count(*) FROM buyer.blocked_stores WHERE blocker_buyer_id = $1", other) != 1 {
		t.Fatal("無關的封鎖不受影響")
	}
}

func TestAlreadyDeletedRepeatsCompleted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.account(t)
	for range 2 {
		state, err := f.module.Request(context.Background(), RequestInput{AccountID: id, Requester: AccountOwner})
		if err != nil || state != Completed {
			t.Fatalf("state=%s err=%v", state, err)
		}
	}
	if f.count(t, id) != 1 {
		t.Fatal("deleted account created a second request")
	}
	var attempts int
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT count(*) FROM accountdeletion.request_attempts").Scan(&attempts); err != nil || attempts != 2 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

// 若移除 partial UNIQUE，此測試的 DB 插入斷言會失敗。
func TestPendingRequestUniqueAndRepeatedRequest(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.account(t)
	f.orders.open = true
	for range 2 {
		state, err := f.module.Request(context.Background(), RequestInput{AccountID: id, Requester: AccountOwner})
		if err != nil || state != PendingOrders {
			t.Fatalf("state=%s err=%v", state, err)
		}
	}
	if f.count(t, id) != 1 {
		t.Fatal("duplicate pending request")
	}
	a, err := f.identity.Account(context.Background(), id)
	if err != nil || a.Deleted {
		t.Fatalf("account deleted while orders are open: %+v %v", a, err)
	}
	if p, err := f.buyer.Profile(context.Background(), id); err != nil || p.Status != buyer.PendingDeletion {
		t.Fatalf("buyer lost: %+v %v", p, err)
	}
	_, err = f.db.Conn(context.Background()).Exec(context.Background(), "INSERT INTO accountdeletion.deletion_requests (id,account_id,status,requested_at) VALUES ($1,$2,'PENDING_ORDERS',now())", uuid.NewV7(), id)
	if err == nil {
		t.Fatal("partial UNIQUE allowed a second pending request")
	}
}

func TestCompletePendingChecksOrdersAgain(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.account(t)
	f.orders.open = true
	if _, err := f.module.Request(context.Background(), RequestInput{AccountID: id, Requester: AccountOwner}); err != nil {
		t.Fatal(err)
	}
	state, err := f.module.CompletePending(context.Background(), id)
	if err != nil || state != PendingOrders {
		t.Fatalf("still open: %s %v", state, err)
	}
	f.orders.open = false
	state, err = f.module.CompletePending(context.Background(), id)
	if err != nil || state != Completed {
		t.Fatalf("completed: %s %v", state, err)
	}
	a, err := f.identity.Account(context.Background(), id)
	if err != nil || !a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	state, err = f.module.CompletePending(context.Background(), id)
	if err != nil || state != Completed {
		t.Fatalf("repeat=%s %v", state, err)
	}
}

// 沒有待辦的帳號不必對訂單發 RPC：先確認待辦存在，才查未結訂單。
func TestCompletePendingWithoutARequestDoesNotQueryOrders(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.account(t)
	_, err := f.module.CompletePending(context.Background(), id)
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("no pending request: %v", err)
	}
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	if f.orders.calls != 0 || len(f.orders.storeCalls) != 0 {
		t.Fatalf("訂單 RPC 被呼叫：calls=%d storeCalls=%v", f.orders.calls, f.orders.storeCalls)
	}
}

func TestOrdersErrorWritesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.account(t)
	f.orders.err = errors.New("offline")
	_, err := f.module.Request(context.Background(), RequestInput{AccountID: id, Requester: AccountOwner})
	if apperr.KindOf(err) != apperr.KindUnavailable || f.count(t, id) != 0 {
		t.Fatalf("orders error: %v count=%d", err, f.count(t, id))
	}
}

// 查完買家訂單之後、進交易之前帳號才開店：交易前沒有把店家納入查詢，交易內讀到店家就不寫入、要求重試。拿掉交易內的
// 店家比對，這次交錯會在沒查過店家訂單的情況下刪帳並關店。
func TestStoreOpenedBetweenOrdersReadAndTransactionWritesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.account(t)
	f.orders.onCall = func() {
		f.orders.onCall = nil
		f.store(t, id)
	}
	_, err := f.module.Request(context.Background(), RequestInput{AccountID: id, Requester: AccountOwner})
	if apperr.KindOf(err) != apperr.KindUnavailable || f.count(t, id) != 0 {
		t.Fatalf("err=%v rows=%d", err, f.count(t, id))
	}
	a, err := f.identity.Account(context.Background(), id)
	if err != nil || a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
}

// grantSellerBeforeLock 在 transaction 取得帳號鎖之前授予 SELLER，模擬交易外讀取之後、鎖定之前的交錯。
type grantSellerBeforeLock struct {
	inner     *identity.Module
	accountID int64
}

func (g *grantSellerBeforeLock) Account(ctx context.Context, id int64) (identity.Account, error) {
	return g.inner.Account(ctx, id)
}
func (g *grantSellerBeforeLock) LockAccount(ctx context.Context, id int64) error {
	if err := g.inner.GrantRole(context.Background(), identity.GrantRoleInput{AccountID: g.accountID, Role: identity.RoleSeller}); err != nil {
		return err
	}
	return g.inner.LockAccount(ctx, id)
}
func (g *grantSellerBeforeLock) BeginDeletion(ctx context.Context, id int64) error {
	return g.inner.BeginDeletion(ctx, id)
}
func (g *grantSellerBeforeLock) DeleteAccount(ctx context.Context, id int64) error {
	return g.inner.DeleteAccount(ctx, id)
}

// 交易外讀帳號時還沒有 SELLER，取得帳號鎖之前才授予（沒有店家）：交易內的成對檢查回 internal，不刪帳、不寫入。
func TestSellerGrantedBeforeTransactionLockIsNotDeleted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.account(t)
	identityWithHook := &grantSellerBeforeLock{inner: f.identity, accountID: id}
	s := processrequest.New(processrequest.Deps{DB: f.db, Identity: identityWithHook, Buyer: f.buyer, Seller: f.seller, HasOpenOrders: f.orders.HasOpenOrders, HasOpenStoreOrders: f.orders.HasOpenStoreOrders, Now: time.Now, NewID: uuid.NewV7})
	_, err := s.Request(context.Background(), RequestInput{AccountID: id, Requester: AccountOwner})
	if apperr.KindOf(err) != apperr.KindInternal || f.count(t, id) != 0 {
		t.Fatalf("err=%v rows=%d", err, f.count(t, id))
	}
	a, err := f.identity.Account(context.Background(), id)
	if err != nil || a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
}

func TestConcurrentRequestsReuseOneProcessRow(t *testing.T) {
	t.Parallel()
	for _, open := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "pending"}[open], func(t *testing.T) {
			f := setup(t)
			id, _ := f.account(t)
			f.orders.open = open
			start := make(chan struct{})
			results := make(chan error, 2)
			for range 2 {
				go func() {
					<-start
					_, err := f.module.Request(context.Background(), RequestInput{AccountID: id, Requester: AccountOwner})
					results <- err
				}()
			}
			close(start)
			for range 2 {
				if err := <-results; err != nil {
					t.Fatal(err)
				}
			}
			var status string
			if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT status FROM accountdeletion.deletion_requests WHERE account_id=$1", id).Scan(&status); err != nil {
				t.Fatal(err)
			}
			want := "COMPLETED"
			if open {
				want = "PENDING_ORDERS"
			}
			if f.count(t, id) != 1 || status != want {
				t.Fatalf("count=%d status=%s", f.count(t, id), status)
			}
			var attempts int
			if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT count(*) FROM accountdeletion.request_attempts").Scan(&attempts); err != nil || attempts != 2 {
				t.Fatalf("attempts=%d err=%v", attempts, err)
			}
		})
	}
}

// 帳號活著 ⇔ 買家活著；若把沒有買家的帳號當成「無 buyer participant」刪帳，這裡會觀察到刪帳或寫入。
func TestAccountWithoutBuyerIsInternalError(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*fixture, int64){
		"account without buyer": func(*fixture, int64) {},
		"seller without buyer": func(f *fixture, id int64) {
			if err := f.identity.GrantRole(context.Background(), identity.GrantRoleInput{AccountID: id, Role: identity.RoleSeller}); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			id := f.accountWithoutBuyer(t)
			prepare(f, id)
			for _, call := range []func() error{
				func() error {
					_, err := f.module.Request(context.Background(), RequestInput{AccountID: id, Requester: AccountOwner})
					return err
				},
				func() error { _, err := f.module.CompletePending(context.Background(), id); return err },
			} {
				if err := call(); err == nil || apperr.KindOf(err) != apperr.KindInternal {
					t.Fatalf("err=%v kind=%v", err, apperr.KindOf(err))
				}
			}
			if f.orders.calls != 0 || f.count(t, id) != 0 {
				t.Fatalf("orders=%d rows=%d", f.orders.calls, f.count(t, id))
			}
			a, err := f.identity.Account(context.Background(), id)
			if err != nil || a.Deleted {
				t.Fatalf("account=%+v err=%v", a, err)
			}
		})
	}
}

// 若週期工作遇錯就停止，後面可完成的帳號仍會維持 pending。
func TestReconcileBatchesAndContinuesAfterError(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var failedID int64
	for i := 0; i < 101; i++ {
		id, _ := f.account(t)
		if i == 0 {
			failedID = id
			if err := f.identity.GrantRole(ctx, identity.GrantRoleInput{AccountID: id, Role: identity.RoleSeller}); err != nil {
				t.Fatal(err)
			}
		}
		_, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO accountdeletion.deletion_requests (id,account_id,status,requested_at) VALUES ($1,$2,'PENDING_ORDERS',now())", uuid.NewV7(), id)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := f.module.request.Reconcile(ctx); err == nil {
		t.Fatal("seller error not reported")
	}
	var pending int
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM accountdeletion.deletion_requests WHERE status='PENDING_ORDERS'").Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("pending=%d err=%v", pending, err)
	}
	if f.count(t, failedID) != 1 {
		t.Fatal("failed account request missing")
	}
}

// 若重送時跳過 attempt insert，此測試的第二筆計數會失敗。
func TestAdminAttemptAfterOwnerPendingIsRecorded(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.account(t)
	f.orders.open = true
	ctx := context.Background()
	if _, err := f.module.Request(ctx, RequestInput{AccountID: id, Requester: AccountOwner}); err != nil {
		t.Fatal(err)
	}
	adminID := int64(99)
	if _, err := f.module.Request(ctx, RequestInput{AccountID: id, Requester: Admin, OperatorAdminID: &adminID, Reason: "管理員申請"}); err != nil {
		t.Fatal(err)
	}
	if f.count(t, id) != 1 {
		t.Fatal("repeat created another state row")
	}
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT requester,operator_admin_id,reason FROM accountdeletion.request_attempts ORDER BY attempted_at,id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var requester, reason string
		var operator *int64
		if err := rows.Scan(&requester, &operator, &reason); err != nil {
			t.Fatal(err)
		}
		if requester == "ADMIN" && operator != nil && *operator == adminID && reason == "管理員申請" {
			got["admin"]++
		}
		if requester == "ACCOUNT_OWNER" && operator == nil && reason == "" {
			got["owner"]++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if got["admin"] != 1 || got["owner"] != 1 {
		t.Fatal(got)
	}
}

func TestRequestConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.account(t)
	ctx := context.Background()
	requestID := uuid.NewV7()
	_, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO accountdeletion.deletion_requests (id,account_id,status,requested_at,completed_at) VALUES ($1,$2,'COMPLETED',now(),now())", requestID, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		status    string
		completed *time.Time
	}{{"UNKNOWN", nil}, {"PENDING_ORDERS", ptrTime(time.Now())}, {"COMPLETED", nil}} {
		_, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO accountdeletion.deletion_requests (id,account_id,status,requested_at,completed_at) VALUES ($1,$2,$3,now(),$4)", uuid.NewV7(), id, pair.status, pair.completed)
		if err == nil {
			t.Fatalf("state constraint accepted %+v", pair)
		}
	}
	for _, attempt := range []struct {
		requester string
		operator  *int64
		reason    string
	}{{"UNKNOWN", nil, ""}, {"ADMIN", nil, "原因"}, {"ADMIN", ptrInt(1), ""}, {"ACCOUNT_OWNER", ptrInt(1), ""}, {"ACCOUNT_OWNER", nil, strings.Repeat("a", 1001)}} {
		_, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO accountdeletion.request_attempts (id,request_id,requester,operator_admin_id,reason,attempted_at) VALUES ($1,$2,$3,$4,$5,now())", uuid.NewV7(), requestID, attempt.requester, attempt.operator, attempt.reason)
		if err == nil {
			t.Fatalf("attempt constraint accepted %+v", attempt)
		}
	}
}
func ptrTime(v time.Time) *time.Time { return &v }
func ptrInt(v int64) *int64          { return &v }

// store 以正式的開店流程為帳號開一間店。
func (f *fixture) store(t *testing.T, accountID int64) seller.Store {
	t.Helper()
	s, err := f.seller.Apply(context.Background(), seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: accountID, StoreName: "測試小店"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *fixture) storeStatus(t *testing.T, storeID int64) seller.Status {
	t.Helper()
	s, err := f.seller.Store(context.Background(), storeID)
	if err != nil {
		t.Fatal(err)
	}
	return s.Status
}

func (f *fixture) buyerStatus(t *testing.T, buyerID int64) string {
	t.Helper()
	var status string
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT status FROM buyer.buyers WHERE buyer_id=$1", buyerID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func adminRequest(accountID int64) RequestInput {
	adminID := int64(99)
	return RequestInput{AccountID: accountID, Requester: Admin, OperatorAdminID: &adminID, Reason: "管理員刪除帳號"}
}

// 管理員刪除整個帳號時，店家是參與者：關店、關閉買家、刪帳號在同一個 transaction（拿掉 seller 的關閉，
// 店家會留在 ACTIVE）。
func TestAdminRequestClosesStoreBuyerAndAccountTogether(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, buyerID := f.account(t)
	s := f.store(t, id)
	state, err := f.module.Request(context.Background(), adminRequest(id))
	if err != nil || state != Completed {
		t.Fatalf("state=%s err=%v", state, err)
	}
	a, err := f.identity.Account(context.Background(), id)
	if err != nil || !a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	if f.storeStatus(t, s.StoreID) != seller.Closed || f.buyerStatus(t, buyerID) != "CLOSED" {
		t.Fatalf("store=%s buyer=%s", f.storeStatus(t, s.StoreID), f.buyerStatus(t, buyerID))
	}
}

// 停權的賣家仍可以被管理員刪除（舊流程要求 IsActive，回誤導的 9204）。
func TestAdminRequestDeletesSuspendedSeller(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, _ := f.account(t)
	s := f.store(t, id)
	review := time.Now().Add(7 * 24 * time.Hour)
	if _, err := f.seller.CreateMeasure(ctx, seller.CreateMeasureInput{
		AdminID: 7, StoreID: s.StoreID, Kind: seller.MeasureSuspension, StatementToSeller: "風險", Note: "內部紀錄", ReviewBy: &review,
		Risk: &seller.RiskBasis{Ground: seller.RiskFraud},
	}); err != nil {
		t.Fatal(err)
	}
	state, err := f.module.Request(ctx, adminRequest(id))
	if err != nil || state != Completed || f.storeStatus(t, s.StoreID) != seller.Closed {
		t.Fatalf("state=%s err=%v store=%s", state, err, f.storeStatus(t, s.StoreID))
	}
}

// 有未結訂單時買家與店家一起進入等待；訂單結束後完成申請，兩者在同一個 transaction 關閉並刪帳號。
func TestAdminPendingIncludesStoreAndCompletesIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, buyerID := f.account(t)
	s := f.store(t, id)
	f.orders.open = true
	state, err := f.module.Request(ctx, adminRequest(id))
	if err != nil || state != PendingOrders {
		t.Fatalf("state=%s err=%v", state, err)
	}
	if a, err := f.identity.Account(ctx, id); err != nil || a.Lifecycle != identity.LifecyclePendingDeletion {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	if f.storeStatus(t, s.StoreID) != seller.PendingDeletion || f.buyerStatus(t, buyerID) != "PENDING_DELETION" {
		t.Fatalf("store=%s buyer=%s", f.storeStatus(t, s.StoreID), f.buyerStatus(t, buyerID))
	}
	f.orders.open = false
	state, err = f.module.CompletePending(ctx, id)
	if err != nil || state != Completed {
		t.Fatalf("complete: state=%s err=%v", state, err)
	}
	if f.storeStatus(t, s.StoreID) != seller.Closed || f.buyerStatus(t, buyerID) != "CLOSED" {
		t.Fatalf("store=%s buyer=%s", f.storeStatus(t, s.StoreID), f.buyerStatus(t, buyerID))
	}
	a, err := f.identity.Account(ctx, id)
	if err != nil || !a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
}

// 帳號等待刪除時不能開店（identity 的 GrantRole 拒絕，開店整筆回滾），所以「等待期間才開的店」不會出現。
func TestStoreCannotOpenWhileAccountPendingDeletion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, _ := f.account(t)
	f.orders.open = true
	if state, err := f.module.Request(ctx, RequestInput{AccountID: id, Requester: AccountOwner}); err != nil || state != PendingOrders {
		t.Fatalf("state=%s err=%v", state, err)
	}
	a, err := f.identity.Account(ctx, id)
	if err != nil || a.Lifecycle != identity.LifecyclePendingDeletion {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	if _, err := f.seller.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: id, StoreName: "測試小店"}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("apply while pending: %v", err)
	}
	if _, err := f.seller.CurrentStore(ctx, id); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("store opened while pending: %v", err)
	}
	f.orders.open = false
	if state, err := f.module.CompletePending(ctx, id); err != nil || state != Completed {
		t.Fatalf("complete: state=%s err=%v", state, err)
	}
	if a, err := f.identity.Account(ctx, id); err != nil || a.Lifecycle != identity.LifecycleDeleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
}

// 回歸（審查 CR-F1）：店家隨管理員的申請進入等待後，賣家不能自己關店（等待刪除的店家沒有 CapAccountClose），帳號也
// 不能再開店，所以「關店再開一間、完成申請卡住」的狀態做不出來；完成申請照樣關閉那間店。
func TestPendingStoreCannotBeClosedOrReplacedByOwner(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, _ := f.account(t)
	s := f.store(t, id)
	f.orders.open = true
	if state, err := f.module.Request(ctx, adminRequest(id)); err != nil || state != PendingOrders {
		t.Fatalf("state=%s err=%v", state, err)
	}
	if _, err := f.seller.CloseCurrent(ctx, id); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("owner closes a pending store: %v", err)
	}
	if _, err := f.seller.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: id, StoreName: "第二間"}); err == nil {
		t.Fatal("second store while pending")
	}
	f.orders.open = false
	if state, err := f.module.CompletePending(ctx, id); err != nil || state != Completed || f.storeStatus(t, s.StoreID) != seller.Closed {
		t.Fatalf("state=%s err=%v store=%s", state, err, f.storeStatus(t, s.StoreID))
	}
}

// 有 SELLER 卻沒有店家是不變量被破壞（關店與移除 SELLER 同一個 transaction）：回 internal，不寫入、不刪帳。
func TestSellerRoleWithoutStoreIsInternalError(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, _ := f.account(t)
	if err := f.identity.GrantRole(ctx, identity.GrantRoleInput{AccountID: id, Role: identity.RoleSeller}); err != nil {
		t.Fatal(err)
	}
	for _, in := range []RequestInput{{AccountID: id, Requester: AccountOwner}, adminRequest(id)} {
		_, err := f.module.Request(ctx, in)
		if err == nil || apperr.KindOf(err) != apperr.KindInternal || f.count(t, id) != 0 {
			t.Fatalf("%s: err=%v kind=%v rows=%d", in.Requester, err, apperr.KindOf(err), f.count(t, id))
		}
	}
	if _, err := f.module.CompletePending(ctx, id); err == nil {
		t.Fatal("沒有待辦的帳號不應完成")
	}
	a, err := f.identity.Account(ctx, id)
	if err != nil || a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
}

// 有店家卻沒有 SELLER 是不變量被破壞（開店與授予同一個 transaction），不論角色都讀店家才查得出來（審查 CR-F2）：
// 回 internal，不寫入、不刪帳。
func TestStoreWithoutSellerRoleIsInternalError(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, _ := f.account(t)
	s := f.store(t, id)
	if err := f.identity.RemoveRole(ctx, identity.RemoveRoleInput{AccountID: id, Role: identity.RoleSeller, SourceID: &s.StoreID}); err != nil {
		t.Fatal(err)
	}
	for _, in := range []RequestInput{{AccountID: id, Requester: AccountOwner}, adminRequest(id)} {
		_, err := f.module.Request(ctx, in)
		if err == nil || apperr.KindOf(err) != apperr.KindInternal || f.count(t, id) != 0 {
			t.Fatalf("%s: err=%v rows=%d", in.Requester, err, f.count(t, id))
		}
	}
}

// 店家是商家的訂單與買家的訂單一樣阻擋管理員刪帳號：買家沒有未結訂單、店家有，買家與店家一起進入等待，不刪帳號；店家
// 的訂單結清之後完成。拿掉管理員路徑的店家檢查，第一段會直接刪帳號。
func TestAdminRequestIsPendingWhileTheStoreHasOpenOrders(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, buyerID := f.account(t)
	s := f.store(t, id)
	f.orders.storeOpen = true
	state, err := f.module.Request(ctx, adminRequest(id))
	if err != nil || state != PendingOrders {
		t.Fatalf("state=%s err=%v", state, err)
	}
	if len(f.orders.storeCalls) != 1 || f.orders.storeCalls[0] != s.StoreID {
		t.Fatalf("store orders must be asked with the store_id (not the account): %v", f.orders.storeCalls)
	}
	if a, err := f.identity.Account(ctx, id); err != nil || a.Deleted || a.Lifecycle != identity.LifecyclePendingDeletion {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	if f.storeStatus(t, s.StoreID) != seller.PendingDeletion || f.buyerStatus(t, buyerID) != "PENDING_DELETION" {
		t.Fatalf("store=%s buyer=%s", f.storeStatus(t, s.StoreID), f.buyerStatus(t, buyerID))
	}
	// 還沒結清：重送與完成都維持等待。
	if state, err = f.module.CompletePending(ctx, id); err != nil || state != PendingOrders {
		t.Fatalf("still open: %s %v", state, err)
	}
	f.orders.storeOpen = false
	if state, err = f.module.CompletePending(ctx, id); err != nil || state != Completed {
		t.Fatalf("cleared: %s %v", state, err)
	}
	if a, err := f.identity.Account(ctx, id); err != nil || !a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	if f.storeStatus(t, s.StoreID) != seller.Closed || f.buyerStatus(t, buyerID) != "CLOSED" {
		t.Fatalf("store=%s buyer=%s", f.storeStatus(t, s.StoreID), f.buyerStatus(t, buyerID))
	}
}

// 買家那一側已經有未結訂單時不必再問店家（結果相同）；買家側結清之後，店家側仍會擋住 CompletePending。
func TestCompletePendingChecksTheStoreAfterTheBuyerSideClears(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, _ := f.account(t)
	f.store(t, id)
	f.orders.open = true
	if state, err := f.module.Request(ctx, adminRequest(id)); err != nil || state != PendingOrders {
		t.Fatalf("state=%s err=%v", state, err)
	}
	if len(f.orders.storeCalls) != 0 {
		t.Fatalf("buyer orders already block; the store was asked anyway: %v", f.orders.storeCalls)
	}
	f.orders.open, f.orders.storeOpen = false, true
	if state, err := f.module.CompletePending(ctx, id); err != nil || state != PendingOrders {
		t.Fatalf("store side must still block: %s %v", state, err)
	}
	f.orders.storeOpen = false
	if state, err := f.module.CompletePending(ctx, id); err != nil || state != Completed {
		t.Fatalf("both cleared: %s %v", state, err)
	}
}

// 店家側查詢失敗時 fail-closed：管理員的申請不寫入，等待中的完成也不刪。
func TestStoreOrdersErrorFailsClosed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, _ := f.account(t)
	s := f.store(t, id)
	f.orders.storeErr = errors.New("offline")
	_, err := f.module.Request(ctx, adminRequest(id))
	if apperr.KindOf(err) != apperr.KindUnavailable || f.count(t, id) != 0 || f.storeStatus(t, s.StoreID) != seller.Active {
		t.Fatalf("request: %v count=%d store=%s", err, f.count(t, id), f.storeStatus(t, s.StoreID))
	}
	if a, err := f.identity.Account(ctx, id); err != nil || a.Deleted || a.Lifecycle == identity.LifecyclePendingDeletion {
		t.Fatalf("account touched: %+v %v", a, err)
	}
	f.orders.storeErr = nil
	f.orders.storeOpen = true
	if state, err := f.module.Request(ctx, adminRequest(id)); err != nil || state != PendingOrders {
		t.Fatalf("state=%s err=%v", state, err)
	}
	f.orders.storeOpen, f.orders.storeErr = false, errors.New("offline")
	state, err := f.module.CompletePending(ctx, id)
	if apperr.KindOf(err) != apperr.KindUnavailable || state == Completed {
		t.Fatalf("complete: %s %v", state, err)
	}
	if a, err := f.identity.Account(ctx, id); err != nil || a.Deleted {
		t.Fatalf("account deleted on a failed check: %+v %v", a, err)
	}
	if f.storeStatus(t, s.StoreID) != seller.PendingDeletion {
		t.Fatalf("store=%s", f.storeStatus(t, s.StoreID))
	}
}

// 帳號本人的申請與管理員的申請一樣把店家當參與者：沒有未結訂單時，關閉買家、關閉店家、刪帳號在同一個 transaction，並且
// 用 store_id 問店家的訂單。拿掉本人路徑的店家參與，店家會留在 ACTIVE 或申請被拒絕。
func TestOwnerRequestClosesStoreBuyerAndAccountTogether(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, buyerID := f.account(t)
	s := f.store(t, id)
	state, err := f.module.Request(ctx, RequestInput{AccountID: id, Requester: AccountOwner})
	if err != nil || state != Completed {
		t.Fatalf("state=%s err=%v", state, err)
	}
	if len(f.orders.storeCalls) != 1 || f.orders.storeCalls[0] != s.StoreID {
		t.Fatalf("store orders must be asked with the store_id: %v", f.orders.storeCalls)
	}
	if a, err := f.identity.Account(ctx, id); err != nil || !a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	if f.storeStatus(t, s.StoreID) != seller.Closed || f.buyerStatus(t, buyerID) != "CLOSED" {
		t.Fatalf("store=%s buyer=%s", f.storeStatus(t, s.StoreID), f.buyerStatus(t, buyerID))
	}
}

// 店家側有未結訂單時，本人的申請讓帳號、買家與店家一起進入等待（買家側沒有訂單）；店家結清之後 CompletePending 完成。
func TestOwnerRequestIsPendingWhileTheStoreHasOpenOrders(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, buyerID := f.account(t)
	s := f.store(t, id)
	f.orders.storeOpen = true
	state, err := f.module.Request(ctx, RequestInput{AccountID: id, Requester: AccountOwner})
	if err != nil || state != PendingOrders {
		t.Fatalf("state=%s err=%v", state, err)
	}
	if a, err := f.identity.Account(ctx, id); err != nil || a.Deleted || a.Lifecycle != identity.LifecyclePendingDeletion {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	if f.storeStatus(t, s.StoreID) != seller.PendingDeletion || f.buyerStatus(t, buyerID) != "PENDING_DELETION" {
		t.Fatalf("store=%s buyer=%s", f.storeStatus(t, s.StoreID), f.buyerStatus(t, buyerID))
	}
	if _, err := f.seller.CloseCurrent(ctx, id); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("owner closes a pending store: %v", err)
	}
	if state, err = f.module.CompletePending(ctx, id); err != nil || state != PendingOrders {
		t.Fatalf("still open: %s %v", state, err)
	}
	f.orders.storeOpen = false
	if state, err = f.module.CompletePending(ctx, id); err != nil || state != Completed {
		t.Fatalf("cleared: %s %v", state, err)
	}
	if a, err := f.identity.Account(ctx, id); err != nil || !a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	if f.storeStatus(t, s.StoreID) != seller.Closed || f.buyerStatus(t, buyerID) != "CLOSED" {
		t.Fatalf("store=%s buyer=%s", f.storeStatus(t, s.StoreID), f.buyerStatus(t, buyerID))
	}
}

// 查完店家的訂單之後、進交易之前，店家關閉又重新入駐：查到的訂單不屬於現在的店家，申請不寫入、要求重試。拿掉交易內的
// 同一個 stint 比對，這個交錯會直接刪帳號。
func TestStoreReplacedAfterTheOrdersCheckWritesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, _ := f.account(t)
	first := f.store(t, id)
	f.orders.onStoreCall = func() {
		if _, err := f.seller.CloseCurrent(ctx, id); err != nil {
			t.Error(err)
			return
		}
		if _, err := f.seller.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: id, StoreName: "新店"}); err != nil {
			t.Error(err)
		}
	}
	_, err := f.module.Request(ctx, adminRequest(id))
	if apperr.KindOf(err) != apperr.KindUnavailable || f.count(t, id) != 0 {
		t.Fatalf("err=%v count=%d", err, f.count(t, id))
	}
	if a, err := f.identity.Account(ctx, id); err != nil || a.Deleted {
		t.Fatalf("account=%+v err=%v", a, err)
	}
	current, err := f.seller.CurrentStore(ctx, id)
	if err != nil || current.StoreID == first.StoreID || current.Status != seller.Active {
		t.Fatalf("current=%+v err=%v", current, err)
	}
}

// ReadSecure 實作 media.Media；這些測試不讀機敏資產。
func (testMedia) ReadSecure(context.Context, string, media.Ref) (media.SecureAsset, error) {
	return media.SecureAsset{}, errors.New("not used")
}

const revokeAppleJob = "identity.revoke_apple_token"

// giveAppleToken 放一筆帳號保存的 Apple refresh token（密文內容在這裡不重要）。
func (f *fixture) giveAppleToken(t *testing.T, accountID int64) {
	t.Helper()
	if _, err := f.db.Conn(context.Background()).Exec(context.Background(),
		"INSERT INTO identity.apple_tokens (account_id, client_id, token) VALUES ($1, 'io.github.will413028.hawkr.buyer', '\\x01')", accountID); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) hasAppleToken(t *testing.T, accountID int64) bool {
	t.Helper()
	var n int
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT count(*) FROM identity.apple_tokens WHERE account_id=$1", accountID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// 每一條最終刪除的路徑（本人立即刪除、管理員刪除、未結訂單完成後的最終刪除）都在刪帳號的 transaction 內排入撤銷並刪掉
// 保存的 token；受理等待（PENDING_ORDERS）時還沒有刪帳號，不撤銷。
func TestEveryFinalDeletionPathRevokesAppleToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("本人立即刪除", func(t *testing.T) {
		f := setup(t)
		id, _ := f.account(t)
		f.giveAppleToken(t, id)
		if state, err := f.module.Request(ctx, RequestInput{AccountID: id, Requester: AccountOwner}); err != nil || state != Completed {
			t.Fatalf("state=%s err=%v", state, err)
		}
		if f.queue.count(revokeAppleJob) != 1 || f.hasAppleToken(t, id) {
			t.Fatalf("撤銷 job %d 個，token 還在 %v", f.queue.count(revokeAppleJob), f.hasAppleToken(t, id))
		}
	})
	t.Run("管理員刪除", func(t *testing.T) {
		f := setup(t)
		id, _ := f.account(t)
		f.giveAppleToken(t, id)
		if state, err := f.module.Request(ctx, adminRequest(id)); err != nil || state != Completed {
			t.Fatalf("state=%s err=%v", state, err)
		}
		if f.queue.count(revokeAppleJob) != 1 || f.hasAppleToken(t, id) {
			t.Fatalf("撤銷 job %d 個，token 還在 %v", f.queue.count(revokeAppleJob), f.hasAppleToken(t, id))
		}
	})
	t.Run("未結訂單完成後的最終刪除", func(t *testing.T) {
		f := setup(t)
		id, _ := f.account(t)
		f.giveAppleToken(t, id)
		f.orders.open = true
		if state, err := f.module.Request(ctx, RequestInput{AccountID: id, Requester: AccountOwner}); err != nil || state != PendingOrders {
			t.Fatalf("state=%s err=%v", state, err)
		}
		if state, err := f.module.CompletePending(ctx, id); err != nil || state != PendingOrders {
			t.Fatalf("仍有未結訂單：state=%s err=%v", state, err)
		}
		if f.queue.count(revokeAppleJob) != 0 || !f.hasAppleToken(t, id) {
			t.Fatalf("等待期間不該撤銷：job %d 個，token 還在 %v", f.queue.count(revokeAppleJob), f.hasAppleToken(t, id))
		}
		f.orders.open = false
		if state, err := f.module.CompletePending(ctx, id); err != nil || state != Completed {
			t.Fatalf("state=%s err=%v", state, err)
		}
		if f.queue.count(revokeAppleJob) != 1 || f.hasAppleToken(t, id) {
			t.Fatalf("撤銷 job %d 個，token 還在 %v", f.queue.count(revokeAppleJob), f.hasAppleToken(t, id))
		}
	})
	t.Run("被拒絕的刪除（有 SELLER 沒有店家）不撤銷", func(t *testing.T) {
		f := setup(t)
		id, _ := f.account(t)
		f.giveAppleToken(t, id)
		if err := f.identity.GrantRole(ctx, identity.GrantRoleInput{AccountID: id, Role: identity.RoleSeller}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.module.Request(ctx, RequestInput{AccountID: id, Requester: AccountOwner}); apperr.KindOf(err) != apperr.KindInternal {
			t.Fatalf("err=%v", err)
		}
		if f.queue.count(revokeAppleJob) != 0 || !f.hasAppleToken(t, id) {
			t.Fatalf("被拒絕的刪除不該撤銷：job %d 個，token 還在 %v", f.queue.count(revokeAppleJob), f.hasAppleToken(t, id))
		}
	})
}
