//go:build integration

package payment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/riverqueue/river"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/account"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

const (
	kindEnsure   = "payment.account_ensure"
	kindSync     = "payment.account_sync"
	kindPurge    = "payment.account_purge"
	kindDispatch = "inbox.dispatch"
)

var (
	enabledSnapshot     = Snapshot{Transfers: CapabilityActive, PayoutsActive: true}
	needsSellerSnapshot = Snapshot{Transfers: CapabilityRestricted, Details: []StatusDetail{{Resolution: ResolutionSellerAction, Reason: "routine_onboarding"}}}
	rejectedSnapshot    = Snapshot{Transfers: CapabilityRejected, Details: []StatusDetail{{Resolution: ResolutionPlatformAction, Reason: "rejected.fraud"}}}
)

// queue 記下排入的 job（不執行）；排入必須在 transaction 內（River 的 InsertTx 也是）。
type queue struct {
	mu   sync.Mutex
	args []river.JobArgs
	out  []bool
	// fail 不是 nil 時，排入這個種類的 job 回 fail（只有一次）：模擬 handler 在 Go 端失敗而 transaction 本身沒有壞。
	failKind string
	fail     error
}

func (q *queue) Enqueue(ctx context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.fail != nil && args.Kind() == q.failKind {
		err := q.fail
		q.fail = nil
		return err
	}
	_, inTx := db.TxFrom(ctx)
	q.args = append(q.args, args)
	q.out = append(q.out, !inTx)
	return nil
}

// storeIDs 是某個種類的 job 排入過的擁有者（依排入順序，不去重）。
func (q *queue) storeIDs(t *testing.T, kind string) []int64 {
	t.Helper()
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []int64
	for _, a := range q.args {
		if a.Kind() != kind {
			continue
		}
		raw, err := json.Marshal(a)
		must(t, err)
		var v struct {
			StoreID int64 `json:"store_id"`
		}
		must(t, json.Unmarshal(raw, &v))
		out = append(out, v.StoreID)
	}
	return out
}

func (q *queue) count(kind string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, a := range q.args {
		if a.Kind() == kind {
			n++
		}
	}
	return n
}

func (q *queue) outsideTx() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return slices.Contains(q.out, true)
}

// owners 是擁有者查詢的 fake：預設營運中，closed 裡的不是；聯絡信箱是 emails 裡「現在」的值（沒有就是沒有信箱）。
type owners struct {
	mu     sync.Mutex
	closed map[int64]bool
	emails map[int64]string
}

func (o *owners) setEmail(storeID int64, email string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if email == "" {
		delete(o.emails, storeID)
		return
	}
	o.emails[storeID] = email
}

func (o *owners) OwnerContactEmail(_ context.Context, storeID int64) (string, bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	email, ok := o.emails[storeID]
	return email, ok, nil
}

func (o *owners) OwnerActive(_ context.Context, storeID int64) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.closed[storeID], nil
}

type fixture struct {
	db      *db.DB
	bus     *events.Bus
	jobs    *queue
	inbox   *inbox.Inbox
	fake    *Fake
	owners  *owners
	module  *Module
	now     time.Time
	changes []PayoutAccountChanged
	inTx    []bool
	logs    bytes.Buffer
}

func setup(t *testing.T, steps ...testdb.Step) *fixture {
	t.Helper()
	steps = append(steps, testdb.Migrations(migrate.Module{Name: inbox.Schema, Dir: inbox.Migrations()}, migrate.Module{Name: "payment", Dir: Migrations()}))
	f := &fixture{db: pg.NewDB(t, steps...), bus: events.NewBus(), jobs: &queue{}, fake: NewFake(), owners: &owners{closed: map[int64]bool{}, emails: map[int64]string{}}, now: time.Date(2026, 11, 6, 4, 0, 0, 0, time.UTC)}
	f.build(t, f.jobs)
	events.Subscribe(f.bus, func(ctx context.Context, e PayoutAccountChanged) error {
		_, ok := db.TxFrom(ctx)
		f.changes = append(f.changes, e)
		f.inTx = append(f.inTx, ok)
		return nil
	})
	return f
}

// build 以 jc 組模組與 inbox（同一個 DB、bus 與 provider）。
func (f *fixture) build(t *testing.T, jc jobs.Enqueuer) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(&f.logs, nil))
	clock := func() time.Time { return f.now }
	f.inbox = inbox.New(inbox.Deps{DB: f.db, Jobs: jc, Now: clock, Logger: log})
	f.module = New(Deps{DB: f.db, Events: f.bus, Jobs: jc, Inbox: f.inbox, Provider: f.fake, Owners: f.owners, Now: clock, Logger: log})
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) count(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, sql, args...).Scan(&n))
	return n
}

// nextStore 是平行測試共用的擁有者 ID 來源（atomic：每次取到不同的值）。
var nextStore atomic.Int64

// request 模擬擁有者核准的 transaction：寫「建立中」的帳戶列並排入建立 job。email 是擁有者此刻的聯絡信箱（空字串是沒有），不傳給 payment。
func (f *fixture) request(t *testing.T, storeID int64, email string) {
	t.Helper()
	f.owners.setEmail(storeID, email)
	must(t, f.db.InTx(context.Background(), func(ctx context.Context) error { return f.module.RequestAccount(ctx, storeID, f.now) }))
}

func (f *fixture) newStore(t *testing.T) int64 {
	t.Helper()
	id := nextStore.Add(1) + 100
	f.request(t, id, "seller@example.com")
	return id
}

func (f *fixture) key(t *testing.T, storeID int64) string {
	t.Helper()
	ctx := context.Background()
	var k string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT create_idempotency_key::text FROM payment.accounts WHERE store_id = $1", storeID).Scan(&k))
	return k
}

func (f *fixture) account(t *testing.T, storeID int64) Account {
	t.Helper()
	a, err := f.module.Account(context.Background(), storeID)
	must(t, err)
	return a
}

// ensure 執行建立 job 一輪，回傳外部帳戶 ID。
func (f *fixture) ensure(t *testing.T, storeID int64) string {
	t.Helper()
	must(t, f.module.EnsureAccount(context.Background(), storeID, false))
	a := f.account(t, storeID)
	if a.ExternalAccountID == nil {
		t.Fatalf("建立之後仍沒有外部帳戶：%+v", a)
	}
	return *a.ExternalAccountID
}

// set 讓外部帳戶回這個快照並執行同步一輪（還沒有外部帳戶時先建立）。
func (f *fixture) set(t *testing.T, storeID int64, s Snapshot) {
	t.Helper()
	external := ""
	if a := f.account(t, storeID); a.ExternalAccountID != nil {
		external = *a.ExternalAccountID
	} else {
		external = f.ensure(t, storeID)
	}
	f.fake.Set(external, s)
	again, err := f.module.SyncAccount(context.Background(), storeID)
	must(t, err)
	if again {
		t.Fatal("沒有事件卻要求再跑一輪")
	}
}

func event(id, external string) (http.Header, []byte) {
	h := http.Header{}
	h.Set(FakeEventHeader, id)
	return h, []byte(external)
}

// receive 收一個事件並執行它的派送 job（webhook 入口與 worker 的派送）。
func (f *fixture) receive(t *testing.T, id, external string) {
	t.Helper()
	ctx := context.Background()
	h, body := event(id, external)
	must(t, f.module.ReceiveEvent(ctx, h, body))
	must(t, f.inbox.Dispatch(ctx, FakeName, id))
}

func (f *fixture) processed(t *testing.T, eventID string) bool {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM inbox.events WHERE event_id = $1 AND processed_at IS NOT NULL", eventID) == 1
}

// 擁有者核准時只 INSERT「建立中」的列並排入建立 job，不呼叫 provider；建立 job 重送（外部帳戶已建立但本地沒記到）時，provider 以列上保存的
// UUID 為冪等鍵回同一個帳戶，不會有第二個。
// mutation：Ensure 每次呼叫產生新的冪等鍵、不用列上保存的 → 重送建出第二個帳戶，且送出的鍵不等於列上的值，失敗。
func TestRequestQueuesOneAccountCreationAndAReplayedCreateKeepsOneExternalAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	if got := f.jobs.storeIDs(t, kindEnsure); !slices.Equal(got, []int64{s}) {
		t.Fatalf("排入的建立 job=%v", got)
	}
	if ensures, _, _ := f.fake.Calls(); ensures != 0 {
		t.Fatalf("RequestAccount 呼叫了 provider：%d 次", ensures)
	}
	a := f.account(t, s)
	if a.Status != StatusNotCreated || a.ExternalAccountID != nil || a.Provider != FakeName || a.TransfersActive || a.RequirementCollection != CollectionProvider {
		t.Fatalf("建立中的列=%+v", a)
	}
	first := f.ensure(t, s)
	if got := f.account(t, s); got.Status != StatusActionRequired {
		t.Fatalf("建立後的狀態=%s", got.Status)
	}
	if got := f.jobs.storeIDs(t, kindSync); !slices.Equal(got, []int64{s}) {
		t.Fatalf("建立後排入的同步 job=%v", got)
	}
	must(t, f.module.EnsureAccount(ctx, s, false))
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET external_account_id = NULL, status = 'NOT_CREATED' WHERE store_id = $1", s)
	must(t, err)
	second := f.ensure(t, s)
	var stored string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT create_idempotency_key::text FROM payment.accounts WHERE store_id = $1", s).Scan(&stored))
	if keys := f.fake.EnsureKeys(); len(keys) != 2 || keys[0] != stored || keys[1] != stored || len(stored) != 36 || stored[14] != '4' {
		t.Fatalf("兩次建立送出的冪等鍵=%v，列上保存的 v4 UUID=%q", keys, stored)
	}
	if second != first || f.fake.Accounts() != 1 {
		t.Fatalf("重送建出另一個帳戶：%s 與 %s（外部帳戶 %d 個）", first, second, f.fake.Accounts())
	}
	if v := f.fake.Violations(); len(v) != 0 {
		t.Fatalf("違反呼叫規則：%v", v)
	}
	if f.jobs.outsideTx() {
		t.Fatal("有 job 在 transaction 外排入")
	}
}

// 已有帳戶列時 RequestAccount 不改它（INSERT ON CONFLICT DO NOTHING：核准的 transaction 不鎖 payment 的既有列，platform/lockorder）；還沒有外部帳戶的列
// 重新排入建立 job，已建立或建立失敗的不排。列上不存聯絡信箱：provider 收到的是建立當下擁有者的信箱。
// mutation：InsertPaymentAccount 改成 ON CONFLICT DO UPDATE SET provider → 既有列被改寫（provider 欄），失敗。
func TestRequestKeepsAnExistingRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.request(t, 7, "first@example.com")
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET provider = 'other', created_at = created_at - interval '1 day' WHERE store_id = 7")
	must(t, err)
	f.request(t, 7, "second@example.com")
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = 7 AND provider = 'other'") != 1 {
		t.Fatal("既有列被 RequestAccount 改寫")
	}
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET provider = 'fake' WHERE store_id = 7")
	must(t, err)
	if got := f.jobs.storeIDs(t, kindEnsure); !slices.Equal(got, []int64{7, 7}) {
		t.Fatalf("還沒有外部帳戶時的建立 job=%v", got)
	}
	f.ensure(t, 7)
	f.request(t, 7, "third@example.com")
	if got := f.jobs.storeIDs(t, kindEnsure); len(got) != 2 {
		t.Fatalf("已建立的帳戶又排了建立 job：%v", got)
	}
	if got := f.fake.EnsureEmails(); !slices.Equal(got, []string{"second@example.com"}) {
		t.Fatalf("provider 收到的信箱=%v，預期建立當下擁有者的信箱", got)
	}
}

// 建立外部帳戶的當下才向擁有者讀聯絡信箱（D50）：核准之後、建立之前信箱改了，用新的；沒有存快照的欄位。
// mutation：Ensure 改回讀快照（建立中的列寫入時記下信箱）→ provider 收到 old@example.com，失敗。
func TestEnsureReadsTheOwnersContactEmailAtCreationTime(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.newStore(t)
	f.owners.setEmail(s, "new@example.com")
	f.ensure(t, s)
	if got := f.fake.EnsureEmails(); !slices.Equal(got, []string{"new@example.com"}) {
		t.Fatalf("provider 收到的信箱=%v", got)
	}
	if f.count(t, "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'payment' AND table_name = 'accounts' AND column_name = 'contact_email'") != 0 {
		t.Fatal("帳戶表還有 contact_email 欄")
	}
}

// 擁有者沒有聯絡信箱時不建立（不呼叫 provider），視為永久失敗：記下建立失敗的時刻（進後台待辦），重試不會再呼叫；補上信箱之後管理員重試建立可以建立。
// mutation：Ensure 沒有信箱也呼叫 provider → provider 被呼叫、帳戶建出來，失敗。
func TestEnsureWithoutAContactEmailRecordsACreateFailureAndNeverCallsTheProvider(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.owners.setEmail(s, "")
	must(t, f.module.EnsureAccount(ctx, s, false))
	if ensures, _, _ := f.fake.Calls(); ensures != 0 {
		t.Fatalf("沒有信箱仍呼叫了 provider：%d 次", ensures)
	}
	if a := f.account(t, s); a.ExternalAccountID != nil || a.CreateFailedAt == nil {
		t.Fatalf("帳戶=%+v，預期沒有外部帳戶且記下建立失敗", a)
	}
	f.owners.setEmail(s, "late@example.com")
	_, err := f.module.RetryAccountCreate(ctx, s)
	must(t, err)
	f.ensure(t, s)
	if got := f.fake.EnsureEmails(); !slices.Equal(got, []string{"late@example.com"}) {
		t.Fatalf("重試時 provider 收到的信箱=%v", got)
	}
}

// 沒有設定 provider 的模組：RequestAccount 什麼都不做，開通相關的呼叫回 failed_precondition，webhook 回 failed_precondition。
func TestUnconfiguredProviderWritesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	m := New(Deps{DB: f.db, Events: f.bus, Jobs: f.jobs, Inbox: inbox.New(inbox.Deps{DB: f.db, Jobs: f.jobs, Now: time.Now, Logger: slog.Default()}), Owners: f.owners, Now: time.Now, Logger: slog.Default()})
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return m.RequestAccount(ctx, 1, f.now) }))
	if m.Configured() || f.count(t, "SELECT count(*) FROM payment.accounts") != 0 || len(f.jobs.args) != 0 {
		t.Fatal("沒有 provider 仍寫了列或排了 job")
	}
	h, body := event("evt", "acct")
	if err := m.ReceiveEvent(ctx, h, body); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("webhook err=%v", err)
	}
	if err := m.RefreshAccount(ctx, 1); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("refresh err=%v", err)
	}
}

// 建立與同步 job 都以 store_id 為唯一鍵（排隊中、執行中、等重試不重複排）；收款事件的派送 job 以 (provider, event_id) 為唯一鍵。
// mutation：SyncArgs 或 PurgeArgs 拿掉 `river:"unique"`（或 ByArgs）→ 同一擁有者排了多個 job，失敗。
func TestJobsAreUniquePerOwnerInRiver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t, testdb.Jobs())
	jc, err := jobs.NewInsertOnly(f.db)
	must(t, err)
	f.fake = NewFake()
	f.build(t, jc)
	open := []string{"available", "scheduled", "running", "retryable", "pending"}
	jobCount := func(kind string, storeID int64) int64 {
		return f.count(t, "SELECT count(*) FROM river.river_job WHERE kind=$1 AND args->>'store_id'=$2 AND state = ANY($3)", kind, fmt.Sprint(storeID), open)
	}
	f.request(t, 1, "a@example.com")
	f.request(t, 1, "a@example.com")
	if n := jobCount(kindEnsure, 1); n != 1 {
		t.Fatalf("建立 job=%d", n)
	}
	external := f.ensure(t, 1)
	for i := range 3 {
		must(t, f.module.RefreshAccount(ctx, 1))
		h, body := event(fmt.Sprintf("evt_%d", i), external)
		must(t, f.module.ReceiveEvent(ctx, h, body))
		must(t, f.module.ReceiveEvent(ctx, h, body))
		must(t, f.inbox.Dispatch(ctx, FakeName, fmt.Sprintf("evt_%d", i)))
	}
	if n := jobCount(kindSync, 1); n != 1 {
		t.Fatalf("同一擁有者的同步 job=%d，應該合併成一個", n)
	}
	if n := f.count(t, "SELECT count(*) FROM river.river_job WHERE kind=$1 AND state = ANY($2)", kindDispatch, open); n != 3 {
		t.Fatalf("三個事件（各收兩次）的派送 job=%d", n)
	}
	f.request(t, 2, "b@example.com")
	f.ensure(t, 2)
	if n := f.count(t, "SELECT count(*) FROM river.river_job WHERE kind=$1 AND state = ANY($2)", kindSync, open); n != 2 {
		t.Fatalf("兩個擁有者的同步 job=%d", n)
	}
	for range 2 {
		must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.ForgetOwners(ctx, []int64{2}) }))
	}
	if n := jobCount(kindPurge, 2); n != 1 {
		t.Fatalf("同一擁有者的清理 job=%d，應該合併成一個", n)
	}
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE river.river_job SET state='running', attempted_at=now() WHERE kind=$1 AND args->>'store_id'='1'", kindSync)
	must(t, err)
	must(t, f.module.RefreshAccount(ctx, 1))
	if n := jobCount(kindSync, 1); n != 1 {
		t.Fatalf("執行中又排了新的同步 job：%d", n)
	}
}

// 同步在寫快照的 transaction 內發 PayoutAccountChanged（每次同步都發；ENABLED 才是 transfers_active），帶 provider 與外部銀行帳號 fingerprint；
// 快取與事件一致。payload 與 fingerprint 不出現在 log。
// mutation：Sync 不發事件 → changes 是空的；TransfersActive 改成 snap.Transfers == active 以外的值 → 第二筆不符。
func TestSyncPublishesPayoutAccountChangedInsideTheWrite(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.newStore(t)
	snap := enabledSnapshot
	snap.BankFingerprints = []string{"fp_secret_1"}
	f.set(t, s, needsSellerSnapshot)
	f.set(t, s, snap)
	if len(f.changes) != 2 || !slices.Equal(f.inTx, []bool{true, true}) {
		t.Fatalf("事件=%+v（transaction 內：%v）", f.changes, f.inTx)
	}
	first, second := f.changes[0], f.changes[1]
	if first.StoreID != s || first.Provider != FakeName || first.TransfersActive || len(first.BankFingerprints) != 0 {
		t.Fatalf("第一筆=%+v", first)
	}
	if second.StoreID != s || !second.TransfersActive || !slices.Equal(second.BankFingerprints, []string{"fp_secret_1"}) || !second.OccurredAt.Equal(f.now) {
		t.Fatalf("第二筆=%+v", second)
	}
	if a := f.account(t, s); a.Status != StatusEnabled || !a.TransfersActive || !a.PayoutsActive {
		t.Fatalf("帳戶=%+v", a)
	}
	if bytes.Contains(f.logs.Bytes(), []byte("fp_secret_1")) {
		t.Fatal("fingerprint 寫進了 log")
	}
}

// 訂閱者失敗時同步整筆 rollback：快取不寫、事件不標記，River 重試同步。
// mutation：Publish 移到 transaction 之後 → 快取已寫入，失敗。
func TestSyncRollsBackWhenTheSubscriberFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	f.receive(t, "evt_1", external)
	boom := errors.New("訂閱者失敗")
	events.Subscribe(f.bus, func(context.Context, PayoutAccountChanged) error { return boom })
	f.fake.Set(external, enabledSnapshot)
	if _, err := f.module.SyncAccount(ctx, s); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if a := f.account(t, s); a.Status != StatusActionRequired || a.FetchedAt != nil {
		t.Fatalf("rollback 後的帳戶=%+v", a)
	}
	if f.processed(t, "evt_1") {
		t.Fatal("rollback 的同步標記了事件")
	}
}

// webhook 寫進 platform inbox：(provider, event_id) 唯一，同一個事件第二次不寫列、不排派送；派送時對得到帳戶的排同步、對不到的只留紀錄並標記已處理；
// 簽章不合的被拒絕、不留紀錄；原始 payload 原樣保存，保存期是 EventRetention，不寫 log。
// mutation：inbox.events 拿掉 (provider, event_id) 的主鍵並讓 insert 不去重 → 第二次也寫列、排派送，失敗。
func TestDuplicateProviderEventIsRecordedOnceAndDispatchedOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	syncs := len(f.jobs.storeIDs(t, kindSync))
	h, body := event("evt_1", external)
	must(t, f.module.ReceiveEvent(ctx, h, body))
	must(t, f.module.ReceiveEvent(ctx, h, body))
	if n := f.count(t, "SELECT count(*) FROM inbox.events WHERE provider = 'fake' AND event_id = 'evt_1'"); n != 1 {
		t.Fatalf("inbox 列=%d", n)
	}
	if n := f.jobs.count(kindDispatch); n != 1 {
		t.Fatalf("派送 job=%d", n)
	}
	var payload []byte
	var retain int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT payload, EXTRACT(EPOCH FROM purge_after - received_at)::bigint FROM inbox.events WHERE event_id = 'evt_1'").Scan(&payload, &retain))
	if string(payload) != external || retain != int64(EventRetention.Seconds()) {
		t.Fatalf("payload=%q 保存期=%v", payload, retain)
	}
	must(t, f.inbox.Dispatch(ctx, FakeName, "evt_1"))
	if got := f.jobs.storeIDs(t, kindSync)[syncs:]; !slices.Equal(got, []int64{s}) {
		t.Fatalf("派送排入的同步=%v", got)
	}
	if f.processed(t, "evt_1") {
		t.Fatal("handler 一收到就標記了已處理（應由涵蓋它的同步標記）")
	}
	f.receive(t, "evt_unknown", "fake_acct_unknown")
	if got := f.jobs.storeIDs(t, kindSync)[syncs:]; len(got) != 1 {
		t.Fatalf("對不到帳戶的事件排了同步：%v", got)
	}
	if !f.processed(t, "evt_unknown") {
		t.Fatal("對不到帳戶的事件沒有標記已處理")
	}
	if err := f.module.ReceiveEvent(ctx, http.Header{}, []byte(external)); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("簽章不合的事件 err=%v", err)
	}
	if bytes.Contains(f.logs.Bytes(), []byte(external)) {
		t.Fatal("payload 寫進了 log")
	}
}

// 驗過簽章但不處理的事件：回 nil，不寫 inbox、不排派送。
// mutation：ReceiveEvent 把 ErrIgnoredEvent 當成 ErrInvalidEvent → 回 InvalidInput，失敗。
func TestIgnoredProviderEventWritesNoInboxRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	h, body := event("evt_ignored", "acct")
	h.Set(FakeIgnoredEventHeader, "1")
	must(t, f.module.ReceiveEvent(ctx, h, body))
	if n := f.count(t, "SELECT count(*) FROM inbox.events"); n != 0 || f.jobs.count(kindDispatch) != 0 {
		t.Fatalf("不處理的事件寫了 inbox：%d", n)
	}
}

// 收款 handler 只排同步、不標記已處理（D29）：同步讀取期間到的事件，它的同步被執行中的 job 合併掉（唯一鍵），只有 inbox 記得它；同步只把讀取開始前
// 收到的事件標成已處理，所以寫入後回 again，下一輪再涵蓋它。
// mutation 1：handleEvent 一收到就 MarkProcessed → 讀取期間到的事件已被標記，again 為 false（事件漏掉），失敗。
// mutation 2：MarkRouteProcessed 拿掉 received_at 的上限 → 同上。
func TestAnEventArrivingWhileTheAccountIsBeingReadIsNotLostWhenItsSyncIsCoalesced(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	f.fake.Set(external, enabledSnapshot)
	f.receive(t, "evt_early", external)
	f.fake.OnFetch = func() {
		f.now = f.now.Add(time.Second)
		f.receive(t, "evt_late", external)
	}
	again, err := f.module.SyncAccount(ctx, s)
	must(t, err)
	if !again {
		t.Fatal("讀取期間到的事件沒有讓同步再跑一輪")
	}
	if !f.processed(t, "evt_early") || f.processed(t, "evt_late") {
		t.Fatalf("early=%v late=%v，只有讀取之前到的事件已處理", f.processed(t, "evt_early"), f.processed(t, "evt_late"))
	}
	f.now = f.now.Add(time.Second)
	again, err = f.module.SyncAccount(ctx, s)
	must(t, err)
	if again || !f.processed(t, "evt_late") {
		t.Fatalf("第二輪 again=%v late=%v", again, f.processed(t, "evt_late"))
	}
	var by string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT processed_by FROM inbox.events WHERE event_id = 'evt_late'").Scan(&by))
	if by != "payment.account_sync" {
		t.Fatalf("處理者=%q", by)
	}
}

// 派送失敗（handler 回錯誤）時派送的 transaction rollback、不記派送成功，River 重試；重試成功後才排同步。用完重試的事件由 platform 的補掃重派。
// handler 的失敗是排入同步 job 失敗（Go 端的錯誤，transaction 本身沒有因 SQL 錯誤而中止）：這樣 Dispatch 若忽略 handler 的錯誤仍能 commit
// dispatched_at，測試才分辨得出來（SQL 錯誤會讓 transaction 中止，之後的寫入本來就失敗，看不到差別）。
// mutation：Dispatch 在 handler 失敗時仍記下 dispatched_at → 第一次派送回 nil、補掃不重派，失敗。
func TestAFailedDispatchIsRetriedAndSweptAgain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	h, body := event("evt_1", external)
	must(t, f.module.ReceiveEvent(ctx, h, body))
	boom := errors.New("排入同步失敗")
	f.jobs.mu.Lock()
	f.jobs.failKind, f.jobs.fail = kindSync, boom
	f.jobs.mu.Unlock()
	if err := f.inbox.Dispatch(ctx, FakeName, "evt_1"); !errors.Is(err, boom) {
		t.Fatalf("handler 失敗時派送 err=%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM inbox.events WHERE event_id = 'evt_1' AND dispatched_at IS NULL"); n != 1 {
		t.Fatal("失敗的派送記成了派送成功")
	}
	dispatches := f.jobs.count(kindDispatch)
	f.now = f.now.Add(10 * time.Minute)
	n, err := f.inbox.Sweep(ctx)
	must(t, err)
	if n != 1 || f.jobs.count(kindDispatch) != dispatches+1 {
		t.Fatalf("補掃重派=%d", n)
	}
	syncs := len(f.jobs.storeIDs(t, kindSync))
	must(t, f.inbox.Dispatch(ctx, FakeName, "evt_1"))
	if got := f.jobs.storeIDs(t, kindSync)[syncs:]; !slices.Equal(got, []int64{s}) {
		t.Fatalf("重試成功後排入的同步=%v", got)
	}
	if n, err := f.inbox.Sweep(ctx); err != nil || n != 0 {
		t.Fatalf("派送成功的事件又被補掃：%d %v", n, err)
	}
}

// 輪詢補漏排 UNDER_REVIEW、ACTION_REQUIRED 的帳戶，以及 DISABLED 但原因碼不是終局（rejected*、unsupported*）的帳戶；已啟用、終局的 DISABLED 與還沒
// 建立的帳戶不排同步。閒置的「建立中」在擁有者營運中時補排建立，關閉的擁有者不排。
// mutation 1：PollablePaymentAccountStoreIDs 的條件改成排除 'DISABLED' 而不是列出狀態 → ENABLED 的帳戶也被排，失敗。
// mutation 2：stale 的補排不問 Owners → 關閉的擁有者也排了建立，失敗。
func TestSweepQueuesSyncForOpenAndLiftableStatusesOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	insert := func(storeID int64, status, external, reason any) {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO payment.accounts
			(store_id, provider, external_account_id, requirement_collection, status, transfers_active, status_reason, created_at, updated_at)
			VALUES ($1, 'fake', $2, 'PROVIDER', $3, $3 = 'ENABLED', $4, $5, $5)`, storeID, external, status, reason, f.now.Add(-time.Hour))
		must(t, err)
	}
	byStatus := map[AccountStatus]int64{}
	for i, st := range AccountStatusValues() {
		id := int64(10 + i)
		byStatus[st] = id
		var external any
		if st != StatusNotCreated {
			external = fmt.Sprintf("acct_%d", id)
		}
		insert(id, string(st), external, nil)
	}
	insert(20, "DISABLED", "acct_20", "risk_under_review")
	insert(21, "DISABLED", "acct_21", "rejected_fraud")
	insert(22, "DISABLED", "acct_22", "unsupported_country")
	insert(23, "NOT_CREATED", nil, nil)
	f.owners.closed[23] = true
	synced, ensured, err := f.module.SweepAccounts(ctx)
	must(t, err)
	want := []int64{byStatus[StatusUnderReview], byStatus[StatusActionRequired], byStatus[StatusDisabled], 20}
	got := f.jobs.storeIDs(t, kindSync)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) || synced != 4 {
		t.Fatalf("輪詢排入的擁有者=%v（回報 %d），應該是 %v", got, synced, want)
	}
	if ensured != 1 || !slices.Equal(f.jobs.storeIDs(t, kindEnsure), []int64{byStatus[StatusNotCreated]}) {
		t.Fatalf("補排的建立=%v（回報 %d）", f.jobs.storeIDs(t, kindEnsure), ensured)
	}
}

// 輪詢補漏也看 inbox：任何狀態（包含 ENABLED）的帳戶，有超過 1 分鐘還沒處理的事件就排同步；剛到的事件與已處理的事件不排。
// mutation：Sweep 拿掉 staleEventStores 的迴圈 → ENABLED 帳戶的舊事件沒有同步，失敗。
func TestSweepQueuesSyncForAnOldUnprocessedEventOnAnEnabledAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	old, fresh, done := f.newStore(t), f.newStore(t), f.newStore(t)
	externals := map[int64]string{}
	for _, s := range []int64{old, fresh, done} {
		externals[s] = f.ensure(t, s)
		f.set(t, s, enabledSnapshot)
	}
	f.receive(t, "evt_done", externals[done])
	_, err := f.module.SyncAccount(ctx, done)
	must(t, err)
	f.receive(t, "evt_old", externals[old])
	f.now = f.now.Add(2 * time.Minute)
	f.receive(t, "evt_fresh", externals[fresh])
	before := len(f.jobs.storeIDs(t, kindSync))
	synced, _, err := f.module.SweepAccounts(ctx)
	must(t, err)
	got := f.jobs.storeIDs(t, kindSync)[before:]
	if !slices.Equal(got, []int64{old}) || synced != 1 {
		t.Fatalf("輪詢排入的擁有者=%v（回報 %d），應該只有舊事件的 %d", got, synced, old)
	}
}

// 每個擁有者的「建立中」列寫入時各自產生一個 UUID 冪等鍵。
// mutation：create_idempotency_key 的預設值改成常數 → 兩個擁有者共用一把鍵，外部帳戶只有一個，失敗。
func TestEachOwnerGetsItsOwnCreateIdempotencyKey(t *testing.T) {
	t.Parallel()
	f := setup(t)
	a, b := f.newStore(t), f.newStore(t)
	ea, eb := f.ensure(t, a), f.ensure(t, b)
	keys := f.fake.EnsureKeys()
	if len(keys) != 2 || keys[0] == keys[1] || ea == eb {
		t.Fatalf("keys=%v accounts=%s %s", keys, ea, eb)
	}
}

// 擁有者關閉時建立 job 不呼叫 provider、不記失敗。
// mutation：Ensure 不問 Owners → provider 被呼叫，失敗。
func TestEnsureSkipsAnInactiveOwner(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.newStore(t)
	f.owners.closed[s] = true
	must(t, f.module.EnsureAccount(context.Background(), s, true))
	if ensures, _, _ := f.fake.Calls(); ensures != 0 {
		t.Fatalf("關閉的擁有者呼叫了 provider：%d 次", ensures)
	}
	if a := f.account(t, s); a.ExternalAccountID != nil || a.CreateFailedAt != nil {
		t.Fatalf("帳戶=%+v", a)
	}
}

type reviewRow struct {
	kind, itemID string
	storeID      int64
	start, due   time.Time
}

func (f *fixture) reviewItems(t *testing.T, at time.Time) []reviewRow {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT kind, item_id, store_id, start_at, due_at FROM payment.pub_review_items($1) ORDER BY item_id", at)
	must(t, err)
	defer rows.Close()
	var out []reviewRow
	for rows.Next() {
		var r reviewRow
		must(t, rows.Scan(&r.kind, &r.itemID, &r.storeID, &r.start, &r.due))
		out = append(out, r)
	}
	must(t, rows.Err())
	return out
}

// 建立重試用完（最後一次嘗試）或 provider 永久拒絕：記下建立失敗的時刻並進後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED（payment.pub_review_items），期限 24 小時；
// 中途的暫時性失敗只回傳錯誤。管理員重試建立清空失敗並重新排入，待辦消失；不是建立失敗的狀態不能重試。
// mutation：Ensure 在最後一次嘗試仍只回傳錯誤、不記 create_failed_at → 待辦沒有這一列，失敗。
func TestCreateFailureAfterTheLastAttemptQueuesAReviewItemAndRetryClearsIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	boom := errors.New("provider 暫時無法使用")
	f.fake.FailEnsure(boom, boom)
	if err := f.module.EnsureAccount(ctx, s, false); !errors.Is(err, boom) {
		t.Fatalf("中途的失敗 err=%v", err)
	}
	if a := f.account(t, s); a.CreateFailedAt != nil || len(f.reviewItems(t, f.now)) != 0 {
		t.Fatal("中途的暫時性失敗就記成建立失敗")
	}
	must(t, f.module.EnsureAccount(ctx, s, true))
	a := f.account(t, s)
	if a.CreateFailedAt == nil || !a.CreateFailedAt.Equal(f.now) || a.ExternalAccountID != nil || a.Status != StatusNotCreated {
		t.Fatalf("最後一次嘗試後=%+v", a)
	}
	rows := f.reviewItems(t, f.now)
	if len(rows) != 1 || rows[0].kind != "PAYMENT_ACCOUNT_CREATE_FAILED" || rows[0].itemID != fmt.Sprint(s) || rows[0].storeID != s ||
		!rows[0].start.Equal(f.now) || !rows[0].due.Equal(f.now.Add(24*time.Hour)) {
		t.Fatalf("待辦=%+v", rows)
	}
	must(t, f.module.EnsureAccount(ctx, s, false))
	if ensures, _, _ := f.fake.Calls(); ensures != 2 {
		t.Fatalf("失敗之後 job 又呼叫了 provider：%d 次", ensures)
	}
	retried, err := f.module.RetryAccountCreate(ctx, s)
	must(t, err)
	if retried.CreateFailedAt != nil || len(f.jobs.storeIDs(t, kindEnsure)) != 2 || len(f.reviewItems(t, f.now)) != 0 {
		t.Fatalf("重試後=%+v", retried)
	}
	if _, err := f.module.RetryAccountCreate(ctx, s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("不是建立失敗的狀態重試 err=%v", err)
	}
	if _, err := f.module.RetryAccountCreate(ctx, s+1000); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有帳戶的擁有者重試 err=%v", err)
	}
	f.fake.FailEnsure(ErrPermanent)
	must(t, f.module.EnsureAccount(ctx, s, false))
	if a := f.account(t, s); a.CreateFailedAt == nil {
		t.Fatalf("永久失敗沒有記成建立失敗：%+v", a)
	}
	_, err = f.module.RetryAccountCreate(ctx, s)
	must(t, err)
	f.ensure(t, s)
}

// 開通 session：外部帳戶建立之前回 failed_precondition，建立之後回 provider 的 session；被停用的帳戶不給 session。外部呼叫不在 transaction 內。
func TestOnboardingSessionNeedsAnExternalAccountThatIsNotDisabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	if _, err := f.module.CreateOnboardingSession(ctx, s, "https://web.example.test/ok", "https://web.example.test/again"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("還沒有外部帳戶 err=%v", err)
	}
	external := f.ensure(t, s)
	session, err := f.module.CreateOnboardingSession(ctx, s, "https://web.example.test/ok", "https://web.example.test/again")
	must(t, err)
	if session.Surface != SurfaceHosted || session.URL == "" || session.ClientSecret != "" {
		t.Fatalf("session=%+v", session)
	}
	f.fake.Set(external, rejectedSnapshot)
	_, err = f.module.SyncAccount(ctx, s)
	must(t, err)
	if a := f.account(t, s); a.Status != StatusDisabled || a.StatusReason == nil || *a.StatusReason != "rejected.fraud" {
		t.Fatalf("帳戶=%+v", a)
	}
	if _, err := f.module.CreateOnboardingSession(ctx, s, "https://web.example.test/ok", "https://web.example.test/again"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被停用的帳戶 err=%v", err)
	}
	if v := f.fake.Violations(); len(v) != 0 {
		t.Fatalf("違反呼叫規則：%v", v)
	}
}

// 擁有者的個資清理：ForgetOwners 在呼叫方的 transaction 內只為每個擁有者排入「關閉後刪除」job（不碰帳戶列、不呼叫 provider），job 先關閉外部帳戶、
// 成功之後才刪那一個擁有者的列，其他擁有者的不動。
// mutation 1：Purge 不呼叫 CloseAccount → 沒有關閉紀錄，失敗。mutation 2：Purge 不帶 store_id 條件 → 其他擁有者的列也被刪，失敗。
func TestForgetOwnersClosesTheExternalAccountThenDeletesTheRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	gone, kept := f.newStore(t), f.newStore(t)
	goneExternal := f.ensure(t, gone)
	f.ensure(t, kept)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.ForgetOwners(ctx, []int64{gone}) }))
	if f.count(t, "SELECT count(*) FROM payment.accounts") != 2 || f.jobs.count(kindPurge) != 1 || len(f.fake.Closes()) != 0 {
		t.Fatal("ForgetOwners 直接刪了列、呼叫了 provider 或沒有排 job")
	}
	if got := f.jobs.storeIDs(t, kindPurge); !slices.Equal(got, []int64{gone}) {
		t.Fatalf("清理 job 的擁有者=%v", got)
	}
	must(t, f.module.PurgeAccount(ctx, gone, false))
	if got := f.fake.Closes(); !slices.Equal(got, []string{goneExternal}) {
		t.Fatalf("關閉的外部帳戶=%v，預期只有 %s", got, goneExternal)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", gone) != 0 || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", kept) != 1 {
		t.Fatal("清理刪錯了列")
	}
	if v := f.fake.Violations(); len(v) != 0 {
		t.Fatalf("違反呼叫規則：%v", v)
	}
}

// 已經關閉的外部帳戶再清理一次（job 重送、列還在）也成功：關閉是冪等的，列照樣刪。
// mutation：Purge 只在第一次關閉時刪列（以本地狀態判斷已關閉就略過刪除）→ 第二次清理後列還在，失敗。
func TestPurgeOfAnAlreadyClosedAccountStillDeletesTheRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	must(t, f.module.PurgeAccount(ctx, s, false))
	f.request(t, s, "again@example.com")
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET external_account_id = $2, status = 'ACTION_REQUIRED' WHERE store_id = $1", s, external)
	must(t, err)
	must(t, f.module.PurgeAccount(ctx, s, false))
	if got := f.fake.Closes(); !slices.Equal(got, []string{external, external}) || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 {
		t.Fatalf("關閉=%v，列應該刪掉", got)
	}
}

// 關閉失敗：暫時性的錯誤回傳給 River 重試、列照舊；最後一次嘗試仍失敗，或失敗是永久的，列留著並標成等待清理（保住 store_id 與外部 ID 的對應），記警示 log
// （store_id 與外部帳戶 ID），不進後台待辦；等待清理的列不進同步輪詢、不被賣家或管理員的重新整理讀；週期掃描重排它的清理，關閉成功才刪列。
// mutation 0：清理 job 不先標記 → 暫時性失敗後沒有標記，失敗（尚未本機執行，需要資料庫）。mutation 1：失敗仍刪列 → 列不見，失敗。mutation 2：輪詢查詢不排除 purge_pending_at → 等待清理的列被排同步，失敗。mutation 3：Sweep 不重排 → 沒有清理 job，失敗。
func TestAFailedCloseKeepsTheRowMarkedExcludedFromPollingAndRetriedByTheSweep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	f.fake.Set(external, needsSellerSnapshot)
	boom := errors.New("provider down")
	f.fake.FailClose(boom, boom)
	if err := f.module.PurgeAccount(ctx, s, false); !errors.Is(err, boom) {
		t.Fatalf("暫時性的失敗要回傳給 River 重試：%v", err)
	}
	if a := f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at IS NOT NULL", s); a != 1 {
		t.Fatal("清理 job 在呼叫 provider 之前就該標記列，暫時性的失敗之後也要在")
	}
	if got := f.jobs.storeIDs(t, kindPurge); slices.Contains(got, s) {
		t.Fatalf("暫時性的失敗由 River 重試，不該由 PurgeAccount 自己排：%v", got)
	}
	must(t, f.module.PurgeAccount(ctx, s, true))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND external_account_id = $2 AND purge_pending_at IS NOT NULL", s, external) != 1 {
		t.Fatal("最後一次失敗後列應該留著並標成等待清理")
	}
	if !bytes.Contains(f.logs.Bytes(), []byte("store_id="+fmt.Sprint(s))) || !bytes.Contains(f.logs.Bytes(), []byte(external)) {
		t.Fatalf("警示 log 沒有 store_id 與外部帳戶 ID：%s", f.logs.String())
	}
	if n := f.count(t, "SELECT count(*) FROM payment.pub_review_items($1) WHERE store_id = $2", f.now, s); n != 0 {
		t.Fatalf("等待清理的列進了後台待辦：%d 項", n)
	}
	before := f.jobs.count(kindSync)
	synced, _, err := f.module.SweepAccounts(ctx)
	must(t, err)
	if synced != 0 || f.jobs.count(kindSync) != before {
		t.Fatalf("等待清理的列被排了同步：synced=%d", synced)
	}
	if got := f.jobs.storeIDs(t, kindPurge); !slices.Equal(got, []int64{s}) {
		t.Fatalf("週期掃描沒有重排清理：%v", got)
	}
	if _, fetches, _ := f.fake.Calls(); fetches != 0 {
		t.Fatalf("等待清理的列被讀了 provider：%d 次", fetches)
	}
	if err := f.module.RefreshAccount(ctx, s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("管理員重新整理 err=%v，預期 failed_precondition", err)
	}
	if err := f.module.SyncAccountUnlessFresh(ctx, s); err != nil {
		t.Fatalf("賣家重新整理 err=%v，預期成功且不讀 provider", err)
	}
	if again, err := f.module.SyncAccount(ctx, s); err != nil || again {
		t.Fatalf("同步 job again=%v err=%v", again, err)
	}
	if _, fetches, _ := f.fake.Calls(); fetches != 0 {
		t.Fatalf("重新整理與同步讀了 provider：%d 次", fetches)
	}
	must(t, f.module.PurgeAccount(ctx, s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 {
		t.Fatal("重排的清理關閉成功後應該刪列")
	}
}

// 關閉的失敗是永久的（金流商沒有這個帳戶，重試不會成功）：第一次嘗試就把列標成等待清理，不等重試用完。
// mutation：永久失敗也回傳錯誤等重試 → 列沒有被標記，失敗。
func TestAPermanentCloseFailureMarksTheRowOnTheFirstAttempt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.ensure(t, s)
	f.fake.FailClose(fmt.Errorf("%w：沒有這個帳戶", ErrPermanent))
	must(t, f.module.PurgeAccount(ctx, s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at IS NOT NULL", s) != 1 {
		t.Fatal("永久失敗後列應該留著並標成等待清理")
	}
}

// 沒有外部 ID 的列（D55）：以列上的冪等鍵取得外部帳戶、關閉它、再刪列；provider 與目前設定不同的列不關也不刪，記警示 log 並標成等待清理，等設定回來再清理
// （刪掉就永遠找不到外部帳戶）。
// mutation 1：沒有外部 ID 直接刪列（m2）→ 沒有取得也沒有關閉，失敗。mutation 2：provider 不符也刪列 → 列不見，失敗。
func TestPurgeDeletesRowsWithoutAnExternalAccountAndKeepsRowsOfAnotherProvider(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	fresh := f.newStore(t)
	must(t, f.module.PurgeAccount(ctx, fresh, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", fresh) != 0 || len(f.fake.Closes()) != 1 || len(f.fake.Resolves()) != 1 {
		t.Fatalf("沒有外部 ID 的列應該以冪等鍵取得外部帳戶、關閉、再刪：關閉=%v 取得=%v", f.fake.Closes(), f.fake.Resolves())
	}
	other := f.newStore(t)
	f.ensure(t, other)
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET provider = 'stripe' WHERE store_id = $1", other)
	must(t, err)
	must(t, f.module.PurgeAccount(ctx, other, true))
	if len(f.fake.Closes()) != 0 {
		t.Fatalf("別的 provider 的帳戶被關閉：%v", f.fake.Closes())
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at IS NOT NULL", other) != 1 {
		t.Fatal("別的 provider 的列應該留著並標成等待清理")
	}
	if !bytes.Contains(f.logs.Bytes(), []byte("level=WARN")) || !bytes.Contains(f.logs.Bytes(), []byte("store_id="+fmt.Sprint(other))) {
		t.Fatalf("沒有警示 log：%s", f.logs.String())
	}
	noProvider := New(Deps{DB: f.db, Events: f.bus, Jobs: f.jobs, Now: func() time.Time { return f.now }, Logger: slog.Default()})
	must(t, noProvider.PurgeAccount(ctx, other, true))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", other) != 1 {
		t.Fatal("沒有設定 provider 時不該刪列")
	}
}

// 沒有外部 ID 的列清理：取得外部帳戶之後先把 ID 存回已標記的列、才關閉；關閉成功但刪列失敗時，重試走有外部 ID 的路，不再取得（已關閉帳戶的同一把鍵重放是永久失敗），列最後被刪除。
// mutation：關閉放在存 ID 之前 → 刪列失敗後列上沒有外部 ID，重試取得已關閉的帳戶失敗，列留著，失敗（尚未本機執行，需要資料庫）。
func TestPurgeRetryAfterCloseSucceededButDeleteFailedDoesNotResolveAClosedAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	_, err := f.db.Conn(ctx).Exec(ctx, `CREATE OR REPLACE FUNCTION payment.test_block_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'delete blocked'; END $$`)
	must(t, err)
	_, err = f.db.Conn(ctx).Exec(ctx, `CREATE TRIGGER test_block_delete BEFORE DELETE ON payment.accounts FOR EACH ROW WHEN (OLD.store_id = `+fmt.Sprint(s)+`) EXECUTE FUNCTION payment.test_block_delete()`)
	must(t, err)
	if err := f.module.PurgeAccount(ctx, s, false); err == nil {
		t.Fatal("刪列被擋，清理應回錯誤")
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND external_account_id IS NOT NULL AND purge_pending_at IS NOT NULL", s) != 1 || len(f.fake.Closes()) != 1 {
		t.Fatalf("關閉之前應該已把外部 ID 存回標記的列：關閉=%v", f.fake.Closes())
	}
	_, err = f.db.Conn(ctx).Exec(ctx, `DROP TRIGGER test_block_delete ON payment.accounts`)
	must(t, err)
	must(t, f.module.PurgeAccount(ctx, s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 || len(f.fake.Resolves()) != 1 {
		t.Fatalf("重試應該刪列且不再取得帳戶：取得=%v", f.fake.Resolves())
	}
}

// 沒有外部 ID 的列取得帳戶失敗：清理 job 在呼叫 provider 之前已標記，最後一次失敗後列留著，週期掃描重排清理；標記的列不被建立、不被管理員重試建立、不被閒置建立的補排；重排的清理成功後刪列。
// mutation 0：沒有外部 ID 的列不標記 → 失敗後沒有標記，失敗（尚未本機執行，需要資料庫）。mutation 1：閒置建立的查詢不排除 purge_pending_at → 標記的列被補排建立，失敗。
func TestAFailedResolveKeepsTheMarkedRowWithoutExternalIDAndTheSweepRetriesIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	boom := errors.New("provider down")
	f.fake.FailResolve(boom, boom)
	if err := f.module.PurgeAccount(ctx, s, false); !errors.Is(err, boom) {
		t.Fatalf("暫時性的失敗要回傳給 River 重試：%v", err)
	}
	must(t, f.module.PurgeAccount(ctx, s, true))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND external_account_id IS NULL AND purge_pending_at IS NOT NULL", s) != 1 {
		t.Fatal("最後一次失敗後沒有外部 ID 的列應該留著並標成等待清理")
	}
	before := f.jobs.count(kindEnsure)
	must(t, f.module.EnsureAccount(ctx, s, false))
	if got := f.fake.Accounts(); got != 0 || f.jobs.count(kindEnsure) != before {
		t.Fatalf("標記的列被建立：帳戶數=%d", got)
	}
	if _, err := f.module.RetryAccountCreate(ctx, s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("標記的列重試建立 err=%v，預期 failed_precondition", err)
	}
	f.now = f.now.Add(24 * time.Hour)
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if f.jobs.count(kindEnsure) != before {
		t.Fatal("閒置建立的補排排了標記的列")
	}
	if got := f.jobs.storeIDs(t, kindPurge); !slices.Contains(got, s) {
		t.Fatalf("週期掃描沒有重排沒有外部 ID 的標記列的清理：%v", got)
	}
	must(t, f.module.PurgeAccount(ctx, s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 {
		t.Fatal("重排的清理成功後應該刪列")
	}
}

// 建立的第一次呼叫結果未知、信箱又改了：重試沿用同一把鍵，provider 回同一個帳戶，帶的是這次的信箱（D55）；不會多出第二個帳戶。
// mutation：Fake 的建立不以鍵回同一個帳戶 → 出現第二個帳戶，失敗。
func TestARetriedCreateWithAChangedEmailKeepsOneExternalAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.newStore(t)
	f.owners.setEmail(s, "old@example.com")
	f.fake.FailEnsureAfterCreate(errors.New("timeout"))
	_ = f.module.EnsureAccount(context.Background(), s, false)
	f.owners.setEmail(s, "new@example.com")
	f.ensure(t, s)
	if got := f.fake.EnsureEmails(); !slices.Equal(got, []string{"old@example.com", "new@example.com"}) || f.fake.Accounts() != 1 {
		t.Fatalf("信箱=%v 帳戶數=%d", got, f.fake.Accounts())
	}
	if a := f.account(t, s); a.ExternalAccountID == nil {
		t.Fatalf("帳戶=%+v", a)
	}
}

// 清理遇到沒有外部 ID 但 provider 那邊已經建好帳戶的列（建立成功、ID 還沒寫入前程序被殺）：取得到的是同一個帳戶，關閉它再刪列，不留下沒人關的帳戶。
// mutation：沒有外部 ID 直接刪列 → 沒有關閉，失敗。
func TestPurgeOfARowWhoseAccountWasCreatedButNeverStoredClosesThatAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	key := f.key(t, s)
	id, err := f.fake.ResolveAccount(ctx, account.ResolveInput{StoreID: s, IdempotencyKey: key})
	must(t, err)
	must(t, f.module.PurgeAccount(ctx, s, false))
	if got := f.fake.Closes(); !slices.Equal(got, []string{id}) || f.fake.Accounts() != 1 {
		t.Fatalf("關閉=%v 帳戶數=%d，預期關閉 %s", got, f.fake.Accounts(), id)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 {
		t.Fatal("列應該已刪除")
	}
}

// 擁有者被清理後，它外部帳戶在 inbox 裡還沒處理的事件在刪除帳戶列的同一個 transaction 標為已處理，過了保存期由 platform 的清理刪除（原始 payload 不會一直留著）。
// mutation：Purge 不呼叫 MarkRouteProcessed → 事件仍未處理，清理之後列還在，失敗。
func TestForgottenOwnersEventsArePurgedAfterRetention(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	f.receive(t, "evt_forgotten", external)
	if f.processed(t, "evt_forgotten") {
		t.Fatal("事件在清理前就已處理")
	}
	must(t, f.module.PurgeAccount(ctx, s, false))
	if !f.processed(t, "evt_forgotten") {
		t.Fatal("清理沒有把事件標為已處理")
	}
	f.now = f.now.Add(EventRetention + time.Hour)
	_, err := f.inbox.Purge(ctx)
	must(t, err)
	if n := f.count(t, "SELECT count(*) FROM inbox.events WHERE event_id = 'evt_forgotten'"); n != 0 {
		t.Fatalf("過了保存期仍留著 %d 列", n)
	}
}

// 同步可以與別的同步並行（job 與 seller 的同步重讀，D32）：較晚開始的讀取先寫入之後，較早開始的那次讀到的是較舊的快照，不能覆蓋它，
// 也不能再發布 PayoutAccountChanged。讀取開始時間由注入的時鐘決定，交錯由 OnFetch 排好，沒有 sleep。
// mutation：ApplyPaymentSnapshot 拿掉 fetched_at 條件 → 較舊的快照覆蓋 ENABLED、再發一次 PayoutAccountChanged，失敗。
func TestAnOlderReadNeverOverwritesANewerCommittedRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	f.fake.Set(external, needsSellerSnapshot)
	f.receive(t, "evt_before_both_reads", external)
	f.now = f.now.Add(time.Second)
	older, newer := f.now, f.now.Add(time.Minute)
	f.fake.OnFetch = func() {
		f.now = newer
		f.fake.Set(external, enabledSnapshot)
		again, err := f.module.SyncAccount(ctx, s)
		must(t, err)
		if again {
			t.Error("較晚開始的讀取沒有事件卻要求再跑一輪")
		}
		f.fake.Set(external, needsSellerSnapshot)
	}
	again, err := f.module.SyncAccount(ctx, s)
	must(t, err)
	if again {
		t.Fatal("被擋下的讀取要求再跑一輪")
	}
	got := f.account(t, s)
	if got.Status != StatusEnabled || !got.TransfersActive || got.FetchedAt == nil || !got.FetchedAt.Equal(newer) {
		t.Fatalf("較舊的快照覆蓋了較新的：status=%s transfers=%v fetched=%v，要 ENABLED 與 %v", got.Status, got.TransfersActive, got.FetchedAt, newer)
	}
	if len(f.changes) != 1 || !f.changes[0].TransfersActive {
		t.Fatalf("PayoutAccountChanged 共 %d 次：%+v，要只有較晚開始的那一次", len(f.changes), f.changes)
	}
	if !f.processed(t, "evt_before_both_reads") {
		t.Fatal("較晚開始的讀取沒有標記讀取之前收到的事件")
	}
	if older.After(newer) || older.Equal(newer) {
		t.Fatal("測試的兩個讀取開始時間沒有先後")
	}
}

// 被擋下的同步（讀取開始時間早於已存的 fetched_at）整個什麼都不做：不寫快取、不發布 PayoutAccountChanged、不把它之前收到的事件標為已處理；
// 讀取開始時間等於 fetched_at 時照常寫入（「不早於」）。fetched_at 是 NULL（還沒同步過）一律寫入由 set 的第一次同步涵蓋。
// mutation 1：被擋下之後仍呼叫 MarkRouteProcessed → 事件被標為已處理，失敗。
// mutation 2：被擋下之後仍發布 PayoutAccountChanged → 多一次事件，失敗。
// mutation 3：條件改成 fetched_at < 讀取開始時間 → 相等時不寫，失敗。
// mutation 4：被擋下的分支回 again=false（不問 RoutePending）→ 還有未處理事件卻不要求再跑，失敗。
func TestARejectedSyncDoesNotMarkEventsOrPublish(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	f.set(t, s, enabledSnapshot)
	stored := *f.account(t, s).FetchedAt
	published := len(f.changes)

	f.now = stored.Add(-time.Minute)
	f.receive(t, "evt_older_read", external)
	f.now = stored.Add(-time.Second)
	f.fake.Set(external, needsSellerSnapshot)
	again, err := f.module.SyncAccount(ctx, s)
	must(t, err)
	if !again {
		t.Fatal("被擋下的同步沒有回報還有未處理的事件（唯讀的 RoutePending），較晚才到的事件要等輪詢")
	}
	got := f.account(t, s)
	if got.Status != StatusEnabled || got.FetchedAt == nil || !got.FetchedAt.Equal(stored) {
		t.Fatalf("被擋下的同步改了快取：status=%s fetched=%v", got.Status, got.FetchedAt)
	}
	if len(f.changes) != published {
		t.Fatalf("被擋下的同步發布了 PayoutAccountChanged：%d → %d", published, len(f.changes))
	}
	if f.processed(t, "evt_older_read") {
		t.Fatal("被擋下的同步把事件標為已處理")
	}

	f.now = stored
	_, err = f.module.SyncAccount(ctx, s)
	must(t, err)
	got = f.account(t, s)
	if got.Status != StatusActionRequired || len(f.changes) != published+1 || !f.processed(t, "evt_older_read") {
		t.Fatalf("讀取開始時間等於 fetched_at 時沒有寫入：status=%s changes=%d processed=%v", got.Status, len(f.changes), f.processed(t, "evt_older_read"))
	}
}

// 賣家觸發的重讀（D33）：窗內（上次寫入的讀取或上次被受理的賣家重讀，含失敗的）不呼叫 provider、快取不變；滿 ReadDebounce 才讀；重讀失敗回 Unavailable，
// 永久失敗回 FailedPrecondition，失敗的那次也占了名額：窗內立刻重試不讀 provider，回 ResourceExhausted 並帶窗的剩餘秒數（不被舊快取蓋成成功）；成功的那次
// 之後立刻重試是成功。時間用注入的 f.now，不睡眠。待 CI（integration）。
// mutation：SyncUnlessFresh 不搶名額 → 窗內多一次 fetch，失敗；搶到卻不讀 → 滿窗後 fetch 次數不變，失敗；失敗時把名額還回去 → 失敗後同一時刻又讀一次，失敗；
// 搶不到時一律回成功 → 失敗後的重試不是 ResourceExhausted，失敗。
func TestSellerTriggeredRereadIsDebouncedAndFailsLoudly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.set(t, s, needsSellerSnapshot)
	_, base, _ := f.fake.Calls()
	external := *f.account(t, s).ExternalAccountID
	fetches := func() int { _, n, _ := f.fake.Calls(); return n - base }

	f.now = f.now.Add(account.ReadDebounce - time.Second)
	f.fake.Set(external, enabledSnapshot)
	must(t, f.module.SyncAccountUnlessFresh(ctx, s))
	if n := fetches(); n != 0 {
		t.Fatalf("窗內向 provider 讀了 %d 次，要 0 次", n)
	}
	if got := f.account(t, s); got.Status == StatusEnabled {
		t.Fatal("窗內的重讀改了快取")
	}

	f.now = f.now.Add(time.Second)
	must(t, f.module.SyncAccountUnlessFresh(ctx, s))
	if n := fetches(); n != 1 {
		t.Fatalf("滿窗後向 provider 讀了 %d 次，要 1 次", n)
	}
	if got := f.account(t, s); got.Status != StatusEnabled {
		t.Fatalf("滿窗後的重讀沒有寫入快取：%s", got.Status)
	}
	must(t, f.module.SyncAccountUnlessFresh(ctx, s))
	if n := fetches(); n != 1 {
		t.Fatalf("成功之後立刻重試又讀了 provider，共 %d 次", n)
	}

	f.now = f.now.Add(account.ReadDebounce)
	f.fake.FailFetch(errors.New("provider 逾時"))
	if err := f.module.SyncAccountUnlessFresh(ctx, s); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("暫時性失敗是 %v，要 Unavailable", err)
	}
	if n := fetches(); n != 2 {
		t.Fatalf("失敗的重讀共讀了 %d 次，要 2 次", n)
	}
	f.now = f.now.Add(4 * time.Second)
	err := f.module.SyncAccountUnlessFresh(ctx, s)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindResourceExhausted || ae.RetryAfter != account.ReadDebounce-4*time.Second {
		t.Fatalf("失敗後窗內的重試是 %v，要帶剩餘 %v 的 ResourceExhausted", err, account.ReadDebounce-4*time.Second)
	}
	if n := fetches(); n != 2 {
		t.Fatalf("失敗的那次沒有占名額：窗內又讀了，共 %d 次", n)
	}

	f.now = f.now.Add(account.ReadDebounce)
	f.fake.FailFetch(fmt.Errorf("%w：外部帳戶不存在", ErrPermanent))
	if err := f.module.SyncAccountUnlessFresh(ctx, s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("永久失敗是 %v，要 FailedPrecondition", err)
	}

	// 還沒有外部帳戶的列：什麼都不做、不算失敗。
	must(t, f.module.SyncAccountUnlessFresh(ctx, f.newStore(t)))
}

// 並行的賣家重讀只有一個讀 provider（D33，搶名額）：第一個讀取進行中、同一時刻（注入的時鐘）又來一個，第二個搶不到、不呼叫 provider，
// 因為快取還沒有涵蓋進行中的讀取，回 ResourceExhausted 並帶整個窗。
// 交錯由 OnFetch 排好，沒有 sleep。待 CI（integration）。
// mutation：搶名額改回「先查 fetched_at 再讀」（名額只在讀取成功寫入後才占）→ 第二個也讀，fetch 共 2 次，失敗。
func TestConcurrentSellerRefreshesReadTheProviderOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	f.fake.Set(external, needsSellerSnapshot)
	_, base, _ := f.fake.Calls()
	var inner error
	f.fake.OnFetch = func() { inner = f.module.SyncAccountUnlessFresh(ctx, s) }
	must(t, f.module.SyncAccountUnlessFresh(ctx, s))
	var ae *apperr.Error
	if !errors.As(inner, &ae) || ae.Kind() != apperr.KindResourceExhausted || ae.RetryAfter != account.ReadDebounce {
		t.Fatalf("進行中的讀取旁的第二個重讀是 %v，要帶 %v 的 ResourceExhausted", inner, account.ReadDebounce)
	}
	if _, n, _ := f.fake.Calls(); n-base != 1 {
		t.Fatalf("並行的兩個賣家重讀共讀了 %d 次，要 1 次", n-base)
	}
	if got := f.account(t, s); got.Status != StatusActionRequired {
		t.Fatalf("搶到名額的那次沒有寫入快取：%s", got.Status)
	}
}

// 賣家重讀被 D32 擋下、而擋下它的讀取之後又收到事件時，賣家路徑自己排一個同步 job（唯一鍵合併），不留給輪詢。待 CI（integration）。
// mutation：SyncUnlessFresh 丟掉 again → 沒有新的同步 job，失敗；被擋下的分支不回報 again → 同上。
func TestASellerRefreshRejectedByTheGuardQueuesASyncForLaterEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	f.fake.Set(external, needsSellerSnapshot)
	f.now = f.now.Add(time.Second)
	newer := f.now.Add(time.Minute)
	f.fake.OnFetch = func() {
		f.now = newer
		f.fake.Set(external, enabledSnapshot)
		_, err := f.module.SyncAccount(ctx, s)
		must(t, err)
		f.fake.Set(external, needsSellerSnapshot)
		f.now = newer.Add(time.Second)
		f.receive(t, "evt_after_newer_read", external)
	}
	syncs := len(f.jobs.storeIDs(t, kindSync))
	must(t, f.module.SyncAccountUnlessFresh(ctx, s))
	if got := f.account(t, s); got.Status != StatusEnabled || got.FetchedAt == nil || !got.FetchedAt.Equal(newer) {
		t.Fatalf("被擋下的賣家重讀改了快取：status=%s fetched=%v", got.Status, got.FetchedAt)
	}
	if f.processed(t, "evt_after_newer_read") {
		t.Fatal("被擋下的重讀把較晚到的事件標為已處理")
	}
	// receive 的 handler 排了一個同步 job；賣家路徑要再排一個（唯一鍵在 River 合併），共 +2。
	if got := len(f.jobs.storeIDs(t, kindSync)) - syncs; got != 2 {
		t.Fatalf("排入 %d 個同步 job，要 2 個（事件 handler 一個、賣家路徑的 again 一個）", got)
	}
	if f.jobs.outsideTx() {
		t.Fatal("有 job 在 transaction 外排入")
	}
}
