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
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/riverqueue/river"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/account"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// createIdleForTest 讓「已呼叫」的列夠舊，週期掃描才會選到它（createIdle 之後）。
const createIdleForTest = 11 * time.Minute

const (
	kindEnsure   = "payment.account_ensure"
	kindSync     = "payment.account_sync"
	kindPurge    = "payment.account_purge"
	kindResolve  = "payment.account_resolve"
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
	opts []river.InsertOpts
	out  []bool
	// fail 不是 nil 時，排入這個種類的 job 回 fail（只有一次）：模擬 handler 在 Go 端失敗而 transaction 本身沒有壞。
	failKind string
	fail     error
	// before 不是 nil 時，每次排入先呼叫它（在取鎖之前，所以它可以阻塞而不擋住別的排入）：測試用它讓一個 transaction 停在已持有鎖、尚未提交的位置。
	before func(args river.JobArgs)
}

func (q *queue) Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	q.mu.Lock()
	hook := q.before
	q.mu.Unlock()
	if hook != nil {
		hook(args)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.fail != nil && args.Kind() == q.failKind {
		err := q.fail
		q.fail = nil
		return err
	}
	_, inTx := db.TxFrom(ctx)
	q.args = append(q.args, args)
	var o river.InsertOpts
	if opts != nil {
		o = *opts
	}
	q.opts = append(q.opts, o)
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

// scheduledAt 是某個種類的 job 排入時帶的 ScheduledAt（依排入順序；零值是立刻執行）。
func (q *queue) scheduledAt(kind string) []time.Time {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []time.Time
	for i, a := range q.args {
		if a.Kind() == kind {
			out = append(out, q.opts[i].ScheduledAt)
		}
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
	audit   *adminaudit.Log
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
	steps = append(steps, testdb.Migrations(migrate.Module{Name: inbox.Schema, Dir: inbox.Migrations()}, migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()}, migrate.Module{Name: "payment", Dir: Migrations()}))
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
	f.audit = adminaudit.New(f.db, clock)
	f.module = New(Deps{DB: f.db, Events: f.bus, Jobs: jc, Inbox: f.inbox, Audit: f.audit, Provider: f.fake, Owners: f.owners, Now: clock, Logger: log})
}

// 後台紀錄的動作名稱（真實的 procedure 名稱由 httpapi 的 handler 帶進來；這裡只要模組原樣寫進紀錄）。
const (
	actionRetryCreate   = "/hawkr.admin.payment.v1alpha1.PaymentAdminService/RetryPaymentAccountCreate"
	actionRetryPurge    = "/hawkr.admin.payment.v1alpha1.PaymentAdminService/RetryPaymentAccountPurge"
	actionDeleteMapping = "/hawkr.admin.payment.v1alpha1.PaymentAdminService/DeletePaymentAccountMapping"
)

func admin(action string) AdminActor { return AdminActor{AdminID: 7, Action: action} }

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
	if a := f.account(t, storeID); a.Status != StatusNotCreated {
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
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET external_account_id = NULL, create_completed_at = NULL, status = 'NOT_CREATED' WHERE store_id = $1", s)
	must(t, err)
	second := f.ensure(t, s)
	var stored string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT create_idempotency_key::text FROM payment.accounts WHERE store_id = $1", s).Scan(&stored))
	if keys := f.fake.AcquireKeys(); len(keys) != 2 || keys[0] != stored || keys[1] != stored || len(stored) != 36 || stored[14] != '4' {
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
	if got := f.fake.ConfigureEmails(); !slices.Equal(got, []string{"second@example.com"}) {
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
	if got := f.fake.ConfigureEmails(); !slices.Equal(got, []string{"new@example.com"}) {
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
	_, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false)
	must(t, err)
	f.ensure(t, s)
	if got := f.fake.ConfigureEmails(); !slices.Equal(got, []string{"late@example.com"}) {
		t.Fatalf("重試時 provider 收到的信箱=%v", got)
	}
}

// 沒有設定 provider 的模組：RequestAccount 什麼都不做，開通相關的呼叫回 failed_precondition，webhook 回 failed_precondition。
func TestUnconfiguredProviderWritesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	m := New(Deps{DB: f.db, Audit: f.audit, Events: f.bus, Jobs: f.jobs, Inbox: inbox.New(inbox.Deps{DB: f.db, Jobs: f.jobs, Now: time.Now, Logger: slog.Default()}), Owners: f.owners, Now: time.Now, Logger: slog.Default()})
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return m.RequestAccount(ctx, 1, f.now) }))
	if m.Configured() || f.count(t, "SELECT count(*) FROM payment.accounts") != 0 || len(f.jobs.args) != 0 {
		t.Fatal("沒有 provider 仍寫了列或排了 job")
	}
	h, body := event("evt", "acct")
	if err := m.ReceiveEvent(ctx, h, body); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("webhook err=%v", err)
	}
	if _, err := m.RefreshAccount(ctx, 1); apperr.KindOf(err) != apperr.KindFailedPrecondition {
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
	h, body := event("evt_while_running", external)
	must(t, f.module.ReceiveEvent(ctx, h, body))
	must(t, f.inbox.Dispatch(ctx, FakeName, "evt_while_running"))
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
			(store_id, provider, external_account_id, requirement_collection, status, transfers_active, status_reason, create_called_at, create_completed_at, created_at, updated_at)
			VALUES ($1, 'fake', $2, 'PROVIDER', $3, $3 = 'ENABLED', $4, CASE WHEN $2::text IS NULL THEN NULL ELSE $5::timestamptz END, CASE WHEN $2::text IS NULL THEN NULL ELSE $5::timestamptz END, $5, $5)`, storeID, external, status, reason, f.now.Add(-time.Hour))
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
	insert(24, "DISABLED", "acct_24", ReasonClosed)
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
	keys := f.fake.AcquireKeys()
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
	f.fake.FailAcquire(boom, boom)
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
	retried, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false)
	must(t, err)
	if retried.CreateFailedAt != nil || len(f.jobs.storeIDs(t, kindEnsure)) != 2 || len(f.reviewItems(t, f.now)) != 0 {
		t.Fatalf("重試後=%+v", retried)
	}
	if _, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("不是建立失敗的狀態重試 err=%v", err)
	}
	if _, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s+1000, false); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有帳戶的擁有者重試 err=%v", err)
	}
	f.fake.FailAcquire(ErrPermanent)
	must(t, f.module.EnsureAccount(ctx, s, false))
	if a := f.account(t, s); a.CreateFailedAt == nil {
		t.Fatalf("永久失敗沒有記成建立失敗：%+v", a)
	}
	_, err = f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false)
	must(t, err)
	f.ensure(t, s)
}

// 開通 session：外部帳戶建立之前回 failed_precondition，建立之後回 provider 的 session；被停用的帳戶不給 session。外部呼叫不在 transaction 內。
func TestOnboardingSessionNeedsAnExternalAccountThatIsNotDisabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	if _, err := f.module.CreateOnboardingSession(ctx, s, "https://web.example.test/ok", "https://web.example.test/again", nil); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("還沒有外部帳戶 err=%v", err)
	}
	external := f.ensure(t, s)
	session, err := f.module.CreateOnboardingSession(ctx, s, "https://web.example.test/ok", "https://web.example.test/again", nil)
	must(t, err)
	if h, ok := session.(HostedSession); !ok || h.URL == "" {
		t.Fatalf("session=%+v", session)
	}
	f.fake.Set(external, rejectedSnapshot)
	_, err = f.module.SyncAccount(ctx, s)
	must(t, err)
	if a := f.account(t, s); a.Status != StatusDisabled || a.StatusReason == nil || *a.StatusReason != "rejected.fraud" {
		t.Fatalf("帳戶=%+v", a)
	}
	if _, err := f.module.CreateOnboardingSession(ctx, s, "https://web.example.test/ok", "https://web.example.test/again", nil); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被停用的帳戶 err=%v", err)
	}
	if v := f.fake.Violations(); len(v) != 0 {
		t.Fatalf("違反呼叫規則：%v", v)
	}
}

// 前置條件與呼叫端宣告的內嵌能力無關：沒有帳戶、沒有外部帳戶、被停用、關閉中都是 not_found 或 failed_precondition，且不向 provider 建立 session；
// 通過後 fake provider 一律回託管連結（契約的內嵌分支只有 Stripe）。
// mutation：略過前置條件 → 沒有外部帳戶與被停用的案例回成功，失敗。
func TestOnboardingSessionKeepsThePreconditionsWhateverTheCallerDeclares(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.fake.WithCapabilities(Capabilities{RequirementCollection: CollectionPlatform})
	open := func(s int64) (OnboardingSession, error) {
		return f.module.CreateOnboardingSession(ctx, s, "https://web.example.test/ok", "https://web.example.test/again", []EmbeddedRuntime{RuntimeStripeConnectJS})
	}
	if _, err := open(987654); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有帳戶列 err=%v", err)
	}
	s := f.newStore(t)
	if _, err := open(s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("還沒有外部帳戶 err=%v", err)
	}
	if _, _, sessions := f.fake.Calls(); sessions != 0 {
		t.Fatalf("前置條件沒過卻向 provider 建立了 %d 個 session", sessions)
	}
	external := f.ensure(t, s)
	session, err := open(s)
	must(t, err)
	if h, ok := session.(HostedSession); !ok || h.URL == "" {
		t.Fatalf("fake provider 一律回託管連結：%+v", session)
	}
	f.fake.Set(external, rejectedSnapshot)
	_, err = f.module.SyncAccount(ctx, s)
	must(t, err)
	if _, err := open(s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被停用 err=%v", err)
	}
	closing := f.newStore(t)
	f.ensure(t, closing)
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET purge_pending_at = now() WHERE store_id = $1", closing)
	must(t, err)
	if _, err := open(closing); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("關閉中 err=%v", err)
	}
	if _, _, sessions := f.fake.Calls(); sessions != 1 {
		t.Fatalf("向 provider 建立了 %d 個 session，預期只有通過前置條件的那一個", sessions)
	}
	if v := f.fake.Violations(); len(v) != 0 {
		t.Fatalf("違反呼叫規則：%v", v)
	}
}

// stripeSessionCalls 回傳一個 Stripe 的假伺服器（重播實測回應）與它收到的路徑紀錄。
func stripeSessionCalls(t *testing.T) (client *stripe.Client, paths func() []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "platform", "stripe", "testdata", "account_session_platform.json"))
	must(t, err)
	var fx struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
	}
	must(t, json.Unmarshal(raw, &fx))
	var mu sync.Mutex
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.URL.Path)
		mu.Unlock()
		if r.URL.Path == "/v1/account_sessions" {
			w.WriteHeader(fx.Status)
			_, _ = w.Write(fx.Body)
			return
		}
		_, _ = w.Write([]byte(`{"object":"v2.core.account_link","url":"https://connect.stripe.com/x","expires_at":"2026-11-04T01:02:03Z"}`))
	}))
	t.Cleanup(srv.Close)
	c, err := stripe.New(stripe.Config{BaseURL: srv.URL, SecretKey: "sk_test_x", WebhookSecret: "whsec_test", AccountNamespace: "env-a"})
	must(t, err)
	return c, func() []string { mu.Lock(); defer mu.Unlock(); return slices.Clone(got) }
}

// session 的形狀取自帳戶列保存的收件方式，不看目前的行程設定：列是平台收件而設定改成金流商收件時，宣告內嵌的呼叫端仍拿到內嵌 session；
// 列是金流商收件而設定改成平台收件時，宣告內嵌的呼叫端仍拿到託管連結。
// mutation：改讀行程設定（provider 的目前收件方式）→ 兩個案例的形狀互換，失敗。
func TestOnboardingSessionShapeComesFromTheAccountRowNotTheCurrentConfiguration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.fake.WithCapabilities(Capabilities{RequirementCollection: CollectionPlatform})
	platformRow := f.newStore(t)
	f.ensure(t, platformRow)
	f.fake.WithCapabilities(Capabilities{RequirementCollection: CollectionProvider})
	providerRow := f.newStore(t)
	f.ensure(t, providerRow)
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET provider = 'stripe' WHERE store_id = ANY($1)", []int64{platformRow, providerRow})
	must(t, err)

	// 每個模組用自己的 inbox：同一個 inbox 登記兩次 stripe 會 panic（組裝錯誤）。
	embedded := []EmbeddedRuntime{RuntimeStripeConnectJS}
	client, paths := stripeSessionCalls(t)
	configured, err := NewStripe(client, CollectionProvider, "pk_test_x")
	must(t, err)
	f.module = New(Deps{DB: f.db, Audit: f.audit, Events: f.bus, Jobs: f.jobs, Inbox: inbox.New(inbox.Deps{DB: f.db, Jobs: f.jobs, Now: func() time.Time { return f.now }, Logger: slog.Default()}), Provider: configured, Owners: f.owners, Now: func() time.Time { return f.now }, Logger: slog.New(slog.NewTextHandler(&f.logs, nil))})
	got, err := f.module.CreateOnboardingSession(ctx, platformRow, "https://web.example.test/ok", "https://web.example.test/again", embedded)
	must(t, err)
	if e, ok := got.(StripeEmbeddedSession); !ok || e.ClientSecret == "" || e.PublishableKey != "pk_test_x" {
		t.Fatalf("平台收件的列、設定是金流商收件：%+v", got)
	}
	configured, err = NewStripe(client, CollectionPlatform, "pk_test_x")
	must(t, err)
	f.module = New(Deps{DB: f.db, Audit: f.audit, Events: f.bus, Jobs: f.jobs, Inbox: inbox.New(inbox.Deps{DB: f.db, Jobs: f.jobs, Now: func() time.Time { return f.now }, Logger: slog.Default()}), Provider: configured, Owners: f.owners, Now: func() time.Time { return f.now }, Logger: slog.New(slog.NewTextHandler(&f.logs, nil))})
	got, err = f.module.CreateOnboardingSession(ctx, providerRow, "https://web.example.test/ok", "https://web.example.test/again", embedded)
	must(t, err)
	if h, ok := got.(HostedSession); !ok || h.URL == "" {
		t.Fatalf("金流商收件的列、設定是平台收件：%+v", got)
	}
	if fmt.Sprint(paths()) != "[/v1/account_sessions /v2/core/account_links]" {
		t.Fatalf("呼叫的路徑 %v", paths())
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
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET external_account_id = $2, create_called_at = $3, create_completed_at = $3, status = 'ACTION_REQUIRED' WHERE store_id = $1", s, external, f.now)
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
	if _, err := f.module.RefreshAccount(ctx, s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
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

// 關閉被金流商永久拒絕（例如沒有這個帳戶，重試不會成功）：第一次嘗試就進終局（D59）：列留著、記下進終局的時刻與原因，進後台待辦 PAYMENT_ACCOUNT_PURGE_STUCK，
// 週期掃描不再重排清理，清理 job 再跑也不動它。
// mutation：永久失敗只回傳錯誤等重試／只標記不進終局（不呼叫 BlockPaymentAccountPurge）→ purge_blocked_at 是 NULL，失敗；Sweep 的清理查詢不排除終局的列 → 掃描重排了它，失敗。
func TestAPermanentCloseRefusalPutsTheRowInTheTerminalStateAndTheSweepStops(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	f.fake.FailClose(fmt.Errorf("%w：沒有這個帳戶", ErrPermanent))
	must(t, f.module.PurgeAccount(ctx, s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at IS NOT NULL AND purge_blocked_at = $2 AND purge_blocked_reason LIKE 'provider_refused%'", s, f.now) != 1 {
		t.Fatal("永久拒絕後列應該留著並進終局（時刻是現在、原因以 provider_refused 開頭）")
	}
	a := f.account(t, s)
	if a.PurgeBlockedAt == nil || a.PurgeBlockedReason == nil || a.ExternalAccountID == nil || *a.ExternalAccountID != external {
		t.Fatalf("帳戶 %+v，預期帶終局的時刻與原因、外部 ID 仍在", a)
	}
	if n := f.count(t, "SELECT count(*) FROM payment.pub_review_items($1) WHERE store_id = $2 AND kind = 'PAYMENT_ACCOUNT_PURGE_STUCK' AND start_at = $3", f.now.Add(time.Hour), s, f.now); n != 1 {
		t.Fatalf("終局的列在後台待辦的項目數=%d，預期 1（開始時間是進終局的時刻）", n)
	}
	closes, purges := len(f.fake.Closes()), f.jobs.count(kindPurge)
	synced, _, err := f.module.SweepAccounts(ctx)
	must(t, err)
	if synced != 0 || f.jobs.count(kindPurge) != purges {
		t.Fatalf("週期掃描重排了終局的列：synced=%d 清理 job %d→%d", synced, purges, f.jobs.count(kindPurge))
	}
	f.now = f.now.Add(30 * 24 * time.Hour)
	must(t, f.module.PurgeAccount(ctx, s, true))
	if len(f.fake.Closes()) != closes || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 1 {
		t.Fatalf("終局的列又被清理 job 處理：關閉 %d→%d", closes, len(f.fake.Closes()))
	}
}

// 逾期（D60：標成等待清理 7 天）：仍然問不到 provider（沒有設定）或暫時性的失敗一直沒好，列進終局，原因是 provider_unconfigured 或 deadline_exceeded；期限內不進。
// mutation：不檢查期限（expired 恆為 false）→ 兩個逾期案例失敗；期限改成 8 天 → 邊界（7 天又 1 秒）失敗。
func TestAPurgeThatCannotReachTheProviderForSevenDaysEntersTheTerminalState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	noProvider := New(Deps{DB: f.db, Audit: f.audit, Events: f.bus, Jobs: f.jobs, Now: func() time.Time { return f.now }, Logger: slog.Default()})

	// 呼叫過、沒有外部 ID、沒有設定 provider：期限內留著，7 天後進終局。
	called := f.newStore(t)
	f.fake.FailAcquire(errors.New("down"))
	_ = f.module.EnsureAccount(ctx, called, false)
	must(t, noProvider.PurgeAccount(ctx, called, true))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL", called) != 1 {
		t.Fatal("期限內應該留著並標成等待清理、不進終局")
	}
	f.now = f.now.Add(PurgeDeadline).Add(time.Second)
	must(t, noProvider.PurgeAccount(ctx, called, true))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_blocked_at = $2 AND purge_blocked_reason LIKE 'provider_unconfigured%'", called, f.now) != 1 {
		t.Fatal("逾期仍問不到 provider，應該進終局（provider_unconfigured）")
	}

	// 有外部 ID、provider 可用但關閉一直暫時性失敗：期限內回傳錯誤重試，逾期進終局。
	flaky := f.newStore(t)
	f.ensure(t, flaky)
	boom := errors.New("provider down")
	f.fake.FailClose(boom, boom)
	if err := f.module.PurgeAccount(ctx, flaky, false); !errors.Is(err, boom) {
		t.Fatalf("期限內的暫時性失敗要回傳給 River 重試：%v", err)
	}
	f.now = f.now.Add(PurgeDeadline).Add(time.Second)
	must(t, f.module.PurgeAccount(ctx, flaky, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_blocked_at = $2 AND purge_blocked_reason LIKE 'deadline_exceeded%'", flaky, f.now) != 1 {
		t.Fatal("逾期的暫時性失敗應該進終局（deadline_exceeded）")
	}
}

// calledRow 回傳一家「呼叫過 provider、沒有外部 ID」的店家：建立的第一次呼叫在 provider 那邊建好帳戶但回應遺失（結果不明），列上有「已呼叫」的時刻、沒有外部 ID。
// provider 那邊的帳戶 ID 一併回傳。
func (f *fixture) calledRow(t *testing.T) (storeID int64, providerID string) {
	t.Helper()
	storeID = f.newStore(t)
	f.fake.FailAcquireAfterCreate(errors.New("timeout"))
	if err := f.module.EnsureAccount(context.Background(), storeID, false); err == nil {
		t.Fatal("回應遺失的建立應回錯誤")
	}
	a := f.account(t, storeID)
	if a.ExternalAccountID != nil || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND create_called_at IS NOT NULL", storeID) != 1 {
		t.Fatalf("呼叫過的列應該有已呼叫時刻、沒有外部 ID：%+v", a)
	}
	ids := f.fake.StoreAccounts(storeID)
	if len(ids) != 1 {
		t.Fatalf("provider 那邊應該已有一個帳戶：ids=%v", ids)
	}
	return storeID, ids[0]
}

// 從沒呼叫過 provider 的列（建立 job 還沒跑、沒有「已呼叫」時刻）清理時直接刪，不需要 provider（D59）：有設定 provider 與沒有設定都不呼叫它；呼叫過的列（結果不明）不直接刪，
// 沒有 provider 可用時留著。
// mutation 1：建立不寫「已呼叫」時刻（markCalled 不寫）→ 呼叫過的列也被直接刪，「呼叫過的不被刪」失敗；mutation 2：從沒呼叫過的列也走 provider（planPurge 只看外部 ID）→
// 從沒呼叫過的列在沒有 provider 時留著，失敗。
func TestPurgeDeletesNeverCalledRowsWithoutAProviderAndKeepsCalledRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	noProvider := New(Deps{DB: f.db, Audit: f.audit, Events: f.bus, Jobs: f.jobs, Now: func() time.Time { return f.now }, Logger: slog.Default()})

	never := f.newStore(t)
	must(t, noProvider.PurgeAccount(ctx, never, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", never) != 0 {
		t.Fatal("從沒呼叫過的列在沒有 provider 時應該直接刪")
	}
	neverConfigured := f.newStore(t)
	must(t, f.module.PurgeAccount(ctx, neverConfigured, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", neverConfigured) != 0 {
		t.Fatal("從沒呼叫過的列有 provider 時也應該直接刪")
	}
	if ensures, _, _ := f.fake.Calls(); ensures != 0 || len(f.fake.Closes()) != 0 {
		t.Fatalf("直接刪的列不該呼叫 provider：取得 %d 關閉 %v", ensures, f.fake.Closes())
	}

	called, _ := f.calledRow(t)
	must(t, noProvider.PurgeAccount(ctx, called, true))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at IS NOT NULL", called) != 1 {
		t.Fatal("呼叫過的列在沒有 provider 時不能刪，要留著並標成等待清理")
	}
}

// 別的 provider 的列（呼叫過）不關也不刪，記警示 log 並標成等待清理，等設定回來再清理（刪掉就永遠找不到外部帳戶）；沒有設定 provider 時同樣不刪。
// mutation：provider 不符也關閉或刪列 → 關閉紀錄多一筆或列不見，失敗。
func TestPurgeKeepsRowsOfAnotherProviderAndWithoutAConfiguredProvider(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	other := f.newStore(t)
	f.ensure(t, other)
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET provider = 'stripe' WHERE store_id = $1", other)
	must(t, err)
	closedBefore := f.fake.Closes()
	must(t, f.module.PurgeAccount(ctx, other, true))
	if got := f.fake.Closes(); !slices.Equal(got, closedBefore) {
		t.Fatalf("別的 provider 的帳戶被關閉：之前=%v 之後=%v", closedBefore, got)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL", other) != 1 {
		t.Fatal("別的 provider 的列應該留著並標成等待清理（期限內不進終局）")
	}
	if !bytes.Contains(f.logs.Bytes(), []byte("level=WARN")) || !bytes.Contains(f.logs.Bytes(), []byte("store_id="+fmt.Sprint(other))) {
		t.Fatalf("沒有警示 log：%s", f.logs.String())
	}
	noProvider := New(Deps{DB: f.db, Audit: f.audit, Events: f.bus, Jobs: f.jobs, Now: func() time.Time { return f.now }, Logger: slog.Default()})
	must(t, noProvider.PurgeAccount(ctx, other, true))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", other) != 1 {
		t.Fatal("沒有設定 provider 時不該刪列")
	}
}

// 呼叫過、沒有外部 ID、冪等鍵 30 天內：以同一把鍵取得 ID（同一個帳戶）、先存回標記的列、才關閉；關閉成功但刪列失敗時，重試走有外部 ID 的路，不再取得
// （已關閉帳戶的同一把鍵重放是永久失敗），列最後被刪除。
// mutation：關閉放在存 ID 之前 → 刪列失敗後列上沒有外部 ID，重試重放已關閉帳戶的鍵失敗而進終局，列留著，失敗。
func TestPurgeRetryAfterCloseSucceededButDeleteFailedDoesNotReplayAClosedAccountsKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, providerID := f.calledRow(t)
	_, err := f.db.Conn(ctx).Exec(ctx, `CREATE OR REPLACE FUNCTION payment.test_block_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'delete blocked'; END $$`)
	must(t, err)
	_, err = f.db.Conn(ctx).Exec(ctx, `CREATE TRIGGER test_block_delete BEFORE DELETE ON payment.accounts FOR EACH ROW WHEN (OLD.store_id = `+fmt.Sprint(s)+`) EXECUTE FUNCTION payment.test_block_delete()`)
	must(t, err)
	if err := f.module.PurgeAccount(ctx, s, false); err == nil {
		t.Fatal("刪列被擋，清理應回錯誤")
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND external_account_id = $2 AND purge_pending_at IS NOT NULL", s, providerID) != 1 || !slices.Equal(f.fake.Closes(), []string{providerID}) {
		t.Fatalf("關閉之前應該已把外部 ID 存回標記的列：關閉=%v", f.fake.Closes())
	}
	_, err = f.db.Conn(ctx).Exec(ctx, `DROP TRIGGER test_block_delete ON payment.accounts`)
	must(t, err)
	acquires := len(f.fake.AcquireKeys())
	must(t, f.module.PurgeAccount(ctx, s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 || len(f.fake.AcquireKeys()) != acquires {
		t.Fatalf("重試應該刪列且不再取得帳戶：取得 %d→%d", acquires, len(f.fake.AcquireKeys()))
	}
}

// 呼叫過、沒有外部 ID 的列取得帳戶失敗：清理 job 在呼叫 provider 之前已標記，最後一次失敗後列留著（期限內不進終局），週期掃描重排清理；標記的列不被建立、
// 不被管理員重試建立、不被閒置建立的補排；重排的清理成功後刪列。
// mutation 0：沒有外部 ID 的列不標記 → 失敗後沒有標記，失敗。mutation 1：閒置建立的查詢不排除 purge_pending_at → 標記的列被補排建立，失敗。
func TestAFailedAcquireKeepsTheMarkedRowWithoutExternalIDAndTheSweepRetriesIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.fake.FailAcquire(errors.New("down"))
	_ = f.module.EnsureAccount(ctx, s, false)
	boom := errors.New("provider down")
	f.fake.FailAcquire(boom, boom)
	if err := f.module.PurgeAccount(ctx, s, false); !errors.Is(err, boom) {
		t.Fatalf("暫時性的失敗要回傳給 River 重試：%v", err)
	}
	must(t, f.module.PurgeAccount(ctx, s, true))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND external_account_id IS NULL AND purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL", s) != 1 {
		t.Fatal("最後一次失敗後沒有外部 ID 的列應該留著並標成等待清理")
	}
	before := f.jobs.count(kindEnsure)
	must(t, f.module.EnsureAccount(ctx, s, false))
	if got := f.fake.Accounts(); got != 0 || f.jobs.count(kindEnsure) != before {
		t.Fatalf("標記的列被建立：帳戶數=%d", got)
	}
	if _, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false); apperr.KindOf(err) != apperr.KindFailedPrecondition {
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

// 建立的第一次呼叫結果未知、信箱又改了：重試沿用同一把鍵，provider 回同一個帳戶，完成設定帶的是這次的信箱（D55）；不會多出第二個帳戶。
// mutation：Fake 的取得不以鍵回同一個帳戶 → 出現第二個帳戶，失敗。
func TestARetriedCreateWithAChangedEmailKeepsOneExternalAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.newStore(t)
	f.owners.setEmail(s, "old@example.com")
	f.fake.FailAcquireAfterCreate(errors.New("timeout"))
	_ = f.module.EnsureAccount(context.Background(), s, false)
	f.owners.setEmail(s, "new@example.com")
	f.ensure(t, s)
	if got := f.fake.ConfigureEmails(); !slices.Equal(got, []string{"new@example.com"}) || f.fake.Accounts() != 1 {
		t.Fatalf("信箱=%v 帳戶數=%d", got, f.fake.Accounts())
	}
	if a := f.account(t, s); a.ExternalAccountID == nil || a.Status != StatusActionRequired {
		t.Fatalf("帳戶=%+v", a)
	}
}

// 清理遇到呼叫過、沒有外部 ID、但 provider 那邊已經建好帳戶的列（建立成功、ID 還沒寫入前程序被殺；冪等鍵 30 天內）：重送同一把鍵取得的是同一個帳戶，關閉它再刪列，
// 不留下沒人關的帳戶。
// mutation：沒有外部 ID 直接刪列 → 沒有關閉，失敗。
func TestPurgeOfARowWhoseAccountWasCreatedButNeverStoredClosesThatAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, id := f.calledRow(t)
	must(t, f.module.PurgeAccount(ctx, s, false))
	if got := f.fake.Closes(); !slices.Equal(got, []string{id}) || f.fake.Accounts() != 1 || len(f.fake.AcquireKeys()) != 2 {
		t.Fatalf("關閉=%v 帳戶數=%d，預期重送同一把鍵（取得 2 次）並關閉 %s", got, f.fake.Accounts(), id)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 {
		t.Fatal("列應該已刪除")
	}
}

// 建立的第一步拿到外部 ID 就寫進列，不等第二步（D59）：第二步失敗時列上已有外部 ID、已呼叫時刻，但還沒完成建立：對外仍是未建立（狀態 NOT_CREATED），不可開通、不輪詢、
// 不同步、不重新整理、收款事件也不排同步；重試從第二步接續，不再取得 ID（同一個帳戶），第二步成功才完成。
// mutation：改回兩步都成功才寫 ID（SetPaymentAccountExternal 移到第二步之後）→ 第二步失敗後列上沒有外部 ID，失敗；半建立的列改回看外部 ID 判斷已建立（sessionTarget、syncRow、
// 輪詢）→ 可開通／被同步，失敗。
func TestTheFirstStepIDIsKeptWhenTheSecondStepFailsAndAHalfCreatedRowIsNotOnboardable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	boom := errors.New("configure failed")
	f.fake.FailConfigure(boom)
	if err := f.module.EnsureAccount(ctx, s, false); !errors.Is(err, boom) {
		t.Fatalf("第二步暫時性失敗要回傳給 River 重試：%v", err)
	}
	a := f.account(t, s)
	if a.ExternalAccountID == nil || a.Status != StatusNotCreated || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND create_called_at IS NOT NULL AND create_completed_at IS NULL", s) != 1 {
		t.Fatalf("第二步失敗後列上應該已有外部 ID、已呼叫時刻、沒有完成時刻：%+v", a)
	}
	external := *a.ExternalAccountID
	if got := f.jobs.storeIDs(t, kindSync); slices.Contains(got, s) {
		t.Fatalf("半建立的列排了同步：%v", got)
	}
	if _, err := f.module.CreateOnboardingSession(ctx, s, "https://r", "https://f", nil); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("半建立的列開通 err=%v，預期 failed_precondition", err)
	}
	if _, err := f.module.RefreshAccount(ctx, s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("半建立的列重新整理 err=%v，預期 failed_precondition", err)
	}
	if err := f.module.SyncAccountUnlessFresh(ctx, s); err != nil {
		t.Fatalf("賣家重新整理 err=%v，預期成功且不讀 provider", err)
	}
	if again, err := f.module.SyncAccount(ctx, s); err != nil || again {
		t.Fatalf("同步 job again=%v err=%v", again, err)
	}
	f.fake.Set(external, needsSellerSnapshot)
	f.receive(t, "evt_half", external)
	if again, err := f.module.SyncAccount(ctx, s); err != nil || again {
		t.Fatalf("收事件之後同步 job again=%v err=%v", again, err)
	}
	synced, _, err := f.module.SweepAccounts(ctx)
	must(t, err)
	if _, fetches, sessions := f.fake.Calls(); fetches != 0 || sessions != 0 || synced != 0 || f.jobs.count(kindSync) != 0 {
		t.Fatalf("半建立的列被讀或被輪詢：讀取 %d 開通 %d 掃描排入 %d 同步 job %d", fetches, sessions, synced, f.jobs.count(kindSync))
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND status = 'NOT_CREATED' AND fetched_at IS NULL", s) != 1 {
		t.Fatal("半建立的列被同步改寫")
	}

	// 重試從第二步接續：不再取得 ID，同一個帳戶，完成之後才可以開通與同步。
	acquires := len(f.fake.AcquireKeys())
	must(t, f.module.EnsureAccount(ctx, s, false))
	done := f.account(t, s)
	if len(f.fake.AcquireKeys()) != acquires || done.ExternalAccountID == nil || *done.ExternalAccountID != external || done.Status != StatusActionRequired || f.fake.Accounts() != 1 {
		t.Fatalf("重試應該沿用已寫入的外部 ID 直接完成第二步：取得 %d→%d %+v", acquires, len(f.fake.AcquireKeys()), done)
	}
	if got := f.jobs.storeIDs(t, kindSync); !slices.Equal(got, []int64{s}) {
		t.Fatalf("完成建立後排入的同步 job=%v", got)
	}
}

// 第二步用完重試仍失敗：外部 ID 留在列上，記下建立失敗的時刻進後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED；管理員重試建立之後從第二步接續。
// mutation：失敗路徑清掉外部 ID → 重試重新取得 ID（取得次數增加），失敗。
func TestAFinalSecondStepFailureKeepsTheIDAndRetryResumesAtTheSecondStep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.fake.FailConfigure(fmt.Errorf("%w：拒絕", ErrPermanent))
	must(t, f.module.EnsureAccount(ctx, s, false))
	a := f.account(t, s)
	if a.ExternalAccountID == nil || a.CreateFailedAt == nil || a.Status != StatusNotCreated {
		t.Fatalf("帳戶=%+v，預期外部 ID 仍在、記下建立失敗", a)
	}
	if n := f.count(t, "SELECT count(*) FROM payment.pub_review_items($1) WHERE store_id = $2 AND kind = 'PAYMENT_ACCOUNT_CREATE_FAILED'", f.now, s); n != 1 {
		t.Fatalf("建立失敗的待辦項目數=%d", n)
	}
	acquires := len(f.fake.AcquireKeys())
	_, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false)
	must(t, err)
	must(t, f.module.EnsureAccount(ctx, s, false))
	if len(f.fake.AcquireKeys()) != acquires || f.account(t, s).Status != StatusActionRequired || f.fake.Accounts() != 1 {
		t.Fatalf("管理員重試之後應該從第二步接續：取得 %d→%d", acquires, len(f.fake.AcquireKeys()))
	}
}

// 管理員「重試關閉」：只有終局的列可以（清除終局、等待清理的時刻重設成現在、重新排入清理，寫一筆後台紀錄）；「已在金流商處理，刪除對應」：只有終局的列可以，同一個 transaction
// 寫一筆 adminaudit 紀錄（對象 payment_account/店家、操作的管理員、時間、detail 帶 provider 與外部 ID）並刪列、不呼叫 provider，紀錄在列刪了之後仍可讀回。權限（payment.accounts.delete_mapping）由 adminauth 的宣告表擋，見 adminauth 的測試。
// mutation 1：DeleteMapping 不呼叫 audit.Record → 讀回 0 筆，失敗；mutation 2：DeleteMapping 不檢查終局 → 非終局的列也被刪，失敗；mutation 3：RetryPurge 不重設等待清理的時刻 → 重試後立刻又逾期，失敗。
func TestAdminRetryPurgeAndDeleteMappingActOnlyOnTheTerminalStateAndLeaveARecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.ensure(t, s)
	if _, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("非終局的重試關閉 err=%v，預期 failed_precondition", err)
	}
	if err := f.module.DeleteAccountMapping(ctx, admin(actionDeleteMapping), s); apperr.KindOf(err) != apperr.KindFailedPrecondition || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 1 {
		t.Fatalf("非終局的刪除對應 err=%v，預期 failed_precondition 且列還在", err)
	}
	f.fake.FailClose(fmt.Errorf("%w：拒絕", ErrPermanent))
	must(t, f.module.PurgeAccount(ctx, s, false))
	if f.account(t, s).PurgeBlockedAt == nil {
		t.Fatal("應該進終局")
	}

	f.now = f.now.Add(3 * 24 * time.Hour)
	purges := f.jobs.count(kindPurge)
	a, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), s)
	must(t, err)
	if a.PurgeBlockedAt != nil || f.jobs.count(kindPurge) != purges+1 || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at = $2", s, f.now) != 1 {
		t.Fatalf("重試關閉應清除終局、重設等待清理的時刻、排入清理：%+v", a)
	}
	must(t, f.module.PurgeAccount(ctx, s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 {
		t.Fatal("重試關閉成功後應該刪列")
	}

	// 刪除對應：再做一次終局，再由管理員確認已在金流商處理。
	s2 := f.newStore(t)
	external2 := f.ensure(t, s2)
	f.fake.FailClose(fmt.Errorf("%w：拒絕", ErrPermanent))
	must(t, f.module.PurgeAccount(ctx, s2, false))
	closes := len(f.fake.Closes())
	f.now = f.now.Add(time.Hour)
	must(t, f.module.DeleteAccountMapping(ctx, admin(actionDeleteMapping), s2))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s2) != 0 || len(f.fake.Closes()) != closes {
		t.Fatalf("刪除對應應該刪列且不呼叫 provider：關閉 %d→%d", closes, len(f.fake.Closes()))
	}
	got, err := f.audit.List(ctx, adminaudit.Filter{Action: actionDeleteMapping})
	must(t, err)
	if len(got) != 1 || got[0].TargetType != "payment_account" || got[0].TargetID != fmt.Sprint(s2) || got[0].AdminID != 7 || !got[0].OccurredAt.Equal(f.now) {
		t.Fatalf("紀錄=%+v，預期對象 payment_account/%d、管理員 7、時間 %s", got, s2, f.now)
	}
	var detail map[string]any
	must(t, json.Unmarshal(got[0].Detail, &detail))
	if detail["provider"] != FakeName || detail["external_account_id"] != external2 || detail["blocked_reason"] != "provider_refused" {
		t.Fatalf("detail=%v，預期 provider、外部 ID %s、終局原因代碼（不含金流商的說明）", detail, external2)
	}
	if other, _ := f.audit.List(ctx, adminaudit.Filter{TargetType: "payment_account", TargetID: fmt.Sprint(s), Action: actionDeleteMapping}); len(other) != 0 {
		t.Fatalf("別家店的紀錄不該出現：%+v", other)
	}
	if err := f.module.DeleteAccountMapping(ctx, admin(actionDeleteMapping), s2); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("列已刪的再刪除對應 err=%v，預期 not_found", err)
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

// 管理員的重新整理走賣家同一條同步重讀（D49-3）：同步向 provider 讀、回傳寫入後的帳戶；與賣家共用名額（任一方的受理都占住窗，窗內另一方不讀 provider）；
// 失敗與窗內重試的結果與賣家相同；沒有外部帳戶回 FailedPrecondition，沒有列回 NotFound。時間用注入的 f.now，不睡眠。待 CI（integration）。
// mutation：RefreshAccount 只排同步 job 不同步重讀 → 第一段 fetch 次數是 0、回傳的帳戶不是 ENABLED，失敗；RefreshAccount 繞過名額直接 Sync → 賣家剛讀完窗內又多一次 fetch，失敗。
func TestAdminRefreshRereadsSynchronouslyThroughTheSellerPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.set(t, s, needsSellerSnapshot)
	_, base, _ := f.fake.Calls()
	external := *f.account(t, s).ExternalAccountID
	fetches := func() int { _, n, _ := f.fake.Calls(); return n - base }

	f.now = f.now.Add(account.ReadDebounce)
	f.fake.Set(external, enabledSnapshot)
	got, err := f.module.RefreshAccount(ctx, s)
	must(t, err)
	if n := fetches(); n != 1 {
		t.Fatalf("管理員重新整理向 provider 讀了 %d 次，要 1 次", n)
	}
	if got.Status != StatusEnabled || got.FetchedAt == nil || !got.FetchedAt.Equal(f.now) {
		t.Fatalf("回傳的不是重讀後的帳戶：status=%s fetched=%v", got.Status, got.FetchedAt)
	}
	if cached := f.account(t, s); cached.Status != got.Status {
		t.Fatalf("回傳的帳戶與快取不一致：%s / %s", got.Status, cached.Status)
	}

	must(t, f.module.SyncAccountUnlessFresh(ctx, s))
	if n := fetches(); n != 1 {
		t.Fatalf("管理員剛讀完，窗內賣家重讀又讀了 provider，共 %d 次", n)
	}

	f.now = f.now.Add(account.ReadDebounce)
	must(t, f.module.SyncAccountUnlessFresh(ctx, s))
	if _, err := f.module.RefreshAccount(ctx, s); err != nil {
		t.Fatalf("賣家剛讀完，窗內管理員重新整理 err=%v，預期成功", err)
	}
	if n := fetches(); n != 2 {
		t.Fatalf("賣家剛讀完，窗內管理員重新整理又讀了 provider，共 %d 次", n)
	}

	f.now = f.now.Add(account.ReadDebounce)
	f.fake.FailFetch(errors.New("provider 逾時"))
	if _, err := f.module.RefreshAccount(ctx, s); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("暫時性失敗是 %v，要 Unavailable", err)
	}
	f.now = f.now.Add(4 * time.Second)
	_, err = f.module.RefreshAccount(ctx, s)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindResourceExhausted || ae.RetryAfter != account.ReadDebounce-4*time.Second {
		t.Fatalf("失敗後窗內的重試是 %v，要帶剩餘 %v 的 ResourceExhausted", err, account.ReadDebounce-4*time.Second)
	}
	if n := fetches(); n != 3 {
		t.Fatalf("失敗的那次沒有占名額：窗內又讀了，共 %d 次", n)
	}

	if _, err := f.module.RefreshAccount(ctx, f.newStore(t)); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("還沒有外部帳戶：%v，要 FailedPrecondition", err)
	}
	if _, err := f.module.RefreshAccount(ctx, s+100000); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有帳戶列：%v，要 NotFound", err)
	}
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

// 已標記等待清理的列：RequestAccount 回 FailedPrecondition，不排建立 job，也不改列（尚未本機執行，需要資料庫）。
// mutation：已標記時回 nil → 沒有錯誤，失敗。
func TestRequestOnAPurgePendingRowFailsWithoutTouchingIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.request(t, s, "owner@example.com")
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET purge_pending_at = $2 WHERE store_id = $1", s, f.now)
	must(t, err)
	before := f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at = $2 AND provider = 'fake'", s, f.now)
	ensures := len(f.jobs.storeIDs(t, kindEnsure))
	err = f.db.InTx(ctx, func(ctx context.Context) error { return f.module.RequestAccount(ctx, s, f.now.Add(time.Hour)) })
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("err=%v，預期 FailedPrecondition", err)
	}
	if got := len(f.jobs.storeIDs(t, kindEnsure)); got != ensures {
		t.Fatalf("建立 job 從 %d 變成 %d", ensures, got)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at = $2 AND provider = 'fake'", s, f.now) != before || before != 1 {
		t.Fatal("既有列被改動")
	}
}

// 沒有設定 provider 時 Sweep 仍重排等待清理的列（同步與建立都要 provider，照舊不排）：呼叫過 provider 的列在 provider 沒有設定時被清理 job 留著，
// 只有掃描重排它，它才會在標記 7 天後進終局並出現在後台待辦（D59、D60；plan 9g 清理表「provider 沒有設定…超過期限進終局」）。清理 job 的目標只取自掃描排入的 job。
// mutation：Sweep 在 provider 為 nil 時直接回傳（還原成 `if p == nil { return 0, 0, nil }`）→ 沒有排入任何清理 job，列永遠不進終局，失敗。
func TestSweepWithoutAProviderRequeuesPurgeAndTheRowReachesTheTerminalStateAfterTheDeadline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	noProvider := New(Deps{DB: f.db, Audit: f.audit, Events: f.bus, Jobs: f.jobs, Owners: f.owners, Now: func() time.Time { return f.now }, Logger: slog.Default()})

	called := f.newStore(t)
	f.fake.FailAcquire(errors.New("down"))
	_ = f.module.EnsureAccount(ctx, called, false)
	must(t, noProvider.PurgeAccount(ctx, called, true))
	healthy := f.newStore(t)
	f.set(t, healthy, needsSellerSnapshot)
	creating := f.newStore(t)

	runQueuedPurges := func() {
		t.Helper()
		queued := f.jobs.storeIDs(t, kindPurge)
		f.jobs.mu.Lock()
		f.jobs.args, f.jobs.opts = nil, nil
		f.jobs.out = nil
		f.jobs.mu.Unlock()
		for _, id := range queued {
			must(t, noProvider.PurgeAccount(ctx, id, true))
		}
	}
	syncsBefore, ensuresBefore := f.jobs.count(kindSync), f.jobs.count(kindEnsure)
	synced, ensured, err := noProvider.SweepAccounts(ctx)
	must(t, err)
	queued := f.jobs.storeIDs(t, kindPurge)
	slices.Sort(queued)
	if synced != 0 || ensured != 0 || f.jobs.count(kindSync) != syncsBefore || f.jobs.count(kindEnsure) != ensuresBefore || !slices.Equal(queued, []int64{called}) {
		t.Fatalf("沒有 provider 的掃描：同步 %d 建立 %d 清理 job=%v，預期不排同步與建立（%d 與 %d 這兩家有同步或建立可排）、只重排清理 %d", synced, ensured, queued, healthy, creating, called)
	}
	runQueuedPurges()
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL", called) != 1 {
		t.Fatal("期限內列應該留著、還沒進終局")
	}

	f.now = f.now.Add(PurgeDeadline).Add(time.Second)
	_, _, err = noProvider.SweepAccounts(ctx)
	must(t, err)
	if got := f.jobs.storeIDs(t, kindPurge); !slices.Contains(got, called) {
		t.Fatalf("逾期後掃描仍要重排清理：%v", got)
	}
	runQueuedPurges()
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_blocked_at = $2 AND purge_blocked_reason LIKE 'provider_unconfigured%'", called, f.now) != 1 {
		t.Fatal("逾期後經掃描重排的清理應該把呼叫過的列送進終局（provider_unconfigured）")
	}
	if n := f.count(t, "SELECT count(*) FROM payment.pub_review_items($1) WHERE store_id = $2 AND kind = 'PAYMENT_ACCOUNT_PURGE_STUCK'", f.now.Add(time.Hour), called); n != 1 {
		t.Fatalf("終局的列在後台待辦的項目數=%d，預期 1", n)
	}
}

// 同步讀到已在 provider 關閉的外部帳戶（D61）：列變成 DISABLED、原因碼 closed、轉帳能力關閉並發布給 seller；之後輪詢不再排它的同步（終局），
// 不像原因碼非終局的 DISABLED 會一直被輪詢。
// mutation 1：StatusOf 不看 Snapshot.Closed → 狀態不是 DISABLED／原因不是 closed，失敗；mutation 2：拿掉 PollablePaymentAccountStoreIDs 的 `a.status_reason <> 'closed'` →
// 掃描為這一列排同步，失敗。
func TestSyncOfAnAccountClosedAtTheProviderIsDisabledWithTheClosedReasonAndNeverPolledAgain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.set(t, s, enabledSnapshot)
	external := *f.account(t, s).ExternalAccountID
	f.fake.Set(external, Snapshot{Closed: true})
	f.changes = nil
	again, err := f.module.SyncAccount(ctx, s)
	must(t, err)
	a := f.account(t, s)
	if again || a.Status != StatusDisabled || a.StatusReason == nil || *a.StatusReason != ReasonClosed || a.TransfersActive || a.PayoutsActive {
		t.Fatalf("again=%v 帳戶=%+v，預期 DISABLED、原因 closed、能力關閉", again, a)
	}
	if len(f.changes) != 1 || f.changes[0].TransfersActive {
		t.Fatalf("發布的變更=%+v，預期一筆 TransfersActive=false", f.changes)
	}
	_, fetches, _ := f.fake.Calls()
	f.jobs.mu.Lock()
	f.jobs.args, f.jobs.out = nil, nil
	f.jobs.mu.Unlock()
	synced, _, err := f.module.SweepAccounts(ctx)
	must(t, err)
	if synced != 0 || f.jobs.count(kindSync) != 0 {
		t.Fatalf("已關閉的帳戶被輪詢排了同步：synced=%d", synced)
	}
	if _, after, _ := f.fake.Calls(); after != fetches {
		t.Fatalf("掃描讀了 provider：%d→%d", fetches, after)
	}
}

// 半建立的列（有外部 ID、沒有 create_completed_at）收到事件：不排同步，事件直接標記已處理（和對不到帳戶的事件相同）；建立完成後才排的同步讀到最新狀態。
// mutation：handleEvent 拿掉 CreateCompletedAt 的檢查 → 排了同步而事件沒標記，失敗。
func TestAnEventForAHalfCreatedRowEnqueuesNoSyncAndIsMarkedProcessed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	boom := errors.New("configure failed")
	f.fake.FailConfigure(boom)
	if err := f.module.EnsureAccount(ctx, s, false); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	external := *f.account(t, s).ExternalAccountID
	f.receive(t, "evt_half_row", external)
	if !f.processed(t, "evt_half_row") || f.jobs.count(kindSync) != 0 {
		t.Fatalf("processed=%v 同步 job=%d，預期事件已處理且沒有同步", f.processed(t, "evt_half_row"), f.jobs.count(kindSync))
	}
}

// cancelOnClose 在關閉時取消 job 的 ctx 並回 ctx 的錯誤，模擬 job 自己逾時被取消。
type cancelOnClose struct {
	*Fake
	cancel context.CancelFunc
}

func (c *cancelOnClose) CloseAccount(ctx context.Context, externalID string) error {
	c.cancel()
	return ctx.Err()
}

// 過了 7 天的列，失敗只是 job 自己的 ctx 被取消或逾時：不是 provider 的失敗，不能記成 deadline_exceeded 進終局，維持可重試。
// mutation：purgeFailed 的 expired 分支拿掉 ctx.Err() == nil → 列進終局，失敗。
func TestAJobCancelledPastTheDeadlineIsNotRecordedAsDeadlineExceeded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	f.ensure(t, s)
	boom := errors.New("provider down")
	f.fake.FailClose(boom)
	if err := f.module.PurgeAccount(ctx, s, false); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	f.now = f.now.Add(PurgeDeadline).Add(time.Second)
	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	log := slog.New(slog.NewTextHandler(&f.logs, nil))
	clock := func() time.Time { return f.now }
	// 第二個模組實例要有自己的 inbox：同一個 inbox 登記同一個 provider 的路由兩次會 panic（platform/inbox）。
	m := New(Deps{DB: f.db, Audit: f.audit, Events: f.bus, Jobs: f.jobs, Inbox: inbox.New(inbox.Deps{DB: f.db, Jobs: f.jobs, Now: clock, Logger: log}), Provider: &cancelOnClose{Fake: f.fake, cancel: cancel}, Owners: f.owners, Now: clock, Logger: log})
	if err := m.PurgeAccount(jobCtx, s, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v，預期回傳取消的錯誤給 River 重試", err)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_blocked_at IS NULL", s) != 1 {
		t.Fatal("job 自己被取消不能進終局")
	}
}

// 了結：週期掃描對每一筆「呼叫過、沒有外部 ID」的非清理列以同一把冪等鍵重送取回 ID（D63），不論擁有者是否營運中；取回之後列是半建立（有 ID、沒完成建立），不多出第二個帳戶。
// mutation：Sweep 不排了結 job（拿掉 sweepUnresolved）→ 沒有 resolve job，失敗；了結前先檢查 OwnerActive → 擁有者關閉時沒有 job，失敗；了結換新鍵 → 帳戶數 2，失敗。
func TestSweepResolvesACalledRowWithoutAnIDForANonOperatingOwnerWithTheSameKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, id := f.calledRow(t)
	key := f.key(t, s)
	f.owners.mu.Lock()
	f.owners.closed[s] = true
	f.owners.mu.Unlock()
	f.now = f.now.Add(createIdleForTest)
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(f.jobs.storeIDs(t, kindResolve), s) {
		t.Fatalf("週期掃描沒有為結果不明的列排了結 job：%v", f.jobs.storeIDs(t, kindResolve))
	}
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	a := f.account(t, s)
	if a.ExternalAccountID == nil || *a.ExternalAccountID != id || f.fake.Accounts() != 1 || f.key(t, s) != key || a.Status != StatusNotCreated {
		t.Fatalf("了結後=%+v 帳戶數=%d，預期取回 %s、同一把鍵、仍是未建立", a, f.fake.Accounts(), id)
	}
	if keys := f.fake.AcquireKeys(); len(keys) != 2 || keys[0] != keys[1] || keys[0] != key {
		t.Fatalf("重送的冪等鍵=%v", keys)
	}
	// 已有外部 ID 之後不再選到。
	before := f.jobs.count(kindResolve)
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if f.jobs.count(kindResolve) != before {
		t.Fatal("已取回外部 ID 的列又被排了結")
	}
}

// 已記建立失敗（還在 7 天內）的列同樣由掃描了結；暫時性的失敗回傳錯誤、最後一次嘗試只留著給下一次掃描，不進建立失敗以外的狀態。
// mutation：選取條件排除 create_failed_at 的列 → 沒有 job，失敗；最後一次暫時失敗記成建立失敗 → 待辦出現，失敗。
func TestSweepResolvesAFailedCreateRowWithinTheWindowAndATransientFailureLeavesItForTheNextSweep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, id := f.calledRow(t)
	boom := errors.New("provider down")
	f.fake.FailAcquire(boom)
	must(t, f.module.EnsureAccount(ctx, s, true))
	if f.account(t, s).CreateFailedAt == nil {
		t.Fatal("最後一次嘗試失敗後應該記建立失敗")
	}
	f.now = f.now.Add(createIdleForTest)
	f.fake.FailAcquire(boom, boom)
	if err := f.module.ResolveUnresolvedAccount(ctx, s, false); !errors.Is(err, boom) {
		t.Fatalf("暫時性失敗要回傳給 River 重試：%v", err)
	}
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, true))
	if a := f.account(t, s); a.ExternalAccountID != nil {
		t.Fatalf("失敗時不該有外部 ID：%+v", a)
	}
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(f.jobs.storeIDs(t, kindResolve), s) {
		t.Fatal("建立失敗但還在期限內的列沒有被排了結")
	}
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	if a := f.account(t, s); a.ExternalAccountID == nil || *a.ExternalAccountID != id || a.CreateFailedAt == nil {
		t.Fatalf("了結後=%+v，預期取回 %s、仍在建立失敗的待辦", a, id)
	}
}

// 等待清理的列也由週期掃描了結：掃描重排清理 job，清理以同一把鍵重送取回 ID 再關閉並刪列（D63）。
// mutation：清理的重送改成只在擁有者營運中時才做 → 列留著、沒有關閉，失敗。
func TestSweepResolvesAPurgePendingRowThroughThePurgeJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, id := f.calledRow(t)
	boom := errors.New("provider down")
	f.fake.FailAcquire(boom)
	must(t, f.module.PurgeAccount(ctx, s, true))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_pending_at IS NOT NULL AND external_account_id IS NULL", s) != 1 {
		t.Fatal("清理失敗後列應該標成等待清理、沒有外部 ID")
	}
	f.now = f.now.Add(createIdleForTest)
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(f.jobs.storeIDs(t, kindPurge), s) {
		t.Fatal("掃描沒有為等待清理的列重排清理")
	}
	must(t, f.module.PurgeAccount(ctx, s, false))
	if got := f.fake.Closes(); !slices.Equal(got, []string{id}) || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 {
		t.Fatalf("關閉=%v，預期取回 %s 後關閉並刪列", got, id)
	}
}

// 結果不明超過 7 天仍了結不了：非清理的列進建立失敗（後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED），清理的列進終局（purge_blocked，原因 unresolved_creation，待辦 PAYMENT_ACCOUNT_PURGE_STUCK）；
// 兩條路都不再向 provider 送任何呼叫（超過冪等鍵保存期之後也一樣），不換新鍵、不列出。
// mutation：Resolve 或 planPurge 在 7 天後仍重送 → AcquireKeys 增加，失敗；清理的逾期不進終局而是刪列 → 列不見，失敗；了結的逾期不記建立失敗 → 待辦沒有這一列，失敗。
func TestAnUnresolvedCreatePast7DaysReachesTheTerminalStateAndNothingIsEverReplayedPastTheRetention(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	create, _ := f.calledRow(t)
	purge, _ := f.calledRow(t)
	acquires := len(f.fake.AcquireKeys())
	closes := len(f.fake.Closes())
	f.now = f.now.Add(UnresolvedDeadline).Add(time.Hour)
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	must(t, f.module.ResolveUnresolvedAccount(ctx, create, false))
	must(t, f.module.PurgeAccount(ctx, purge, false))
	a := f.account(t, create)
	if a.CreateFailedAt == nil || !a.CreateFailedAt.Equal(f.now) || a.CreateUnresolved || a.ExternalAccountID != nil {
		t.Fatalf("非清理的列=%+v，預期進建立失敗；還在冪等鍵保存期內，所以 create_unresolved 為假", a)
	}
	var blocked bool
	var reason string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT purge_blocked_at IS NOT NULL, COALESCE(purge_blocked_reason, '') FROM payment.accounts WHERE store_id = $1", purge).Scan(&blocked, &reason))
	if !blocked || !strings.HasPrefix(reason, "unresolved_creation") {
		t.Fatalf("清理的列 blocked=%v reason=%q", blocked, reason)
	}
	kinds := map[int64]string{}
	for _, r := range f.reviewItems(t, f.now) {
		kinds[r.storeID] = r.kind
	}
	if kinds[create] != "PAYMENT_ACCOUNT_CREATE_FAILED" || kinds[purge] != "PAYMENT_ACCOUNT_PURGE_STUCK" {
		t.Fatalf("待辦=%v", kinds)
	}
	// 之後的掃描與重跑，包括超過冪等鍵保存期，都不送任何呼叫。
	for _, advance := range []time.Duration{time.Hour, IdempotencyRetention} {
		f.now = f.now.Add(advance)
		if _, _, err := f.module.SweepAccounts(ctx); err != nil {
			t.Fatal(err)
		}
		must(t, f.module.ResolveUnresolvedAccount(ctx, create, false))
		must(t, f.module.EnsureAccount(ctx, create, false))
		must(t, f.module.PurgeAccount(ctx, purge, false))
	}
	if got := len(f.fake.AcquireKeys()); got != acquires || len(f.fake.Closes()) != closes {
		t.Fatalf("逾期之後 provider 又被呼叫：取得 %d→%d 關閉 %d→%d", acquires, got, closes, len(f.fake.Closes()))
	}
	if a := f.account(t, create); a.ExternalAccountID != nil || !a.CreateUnresolved {
		t.Fatalf("逾期的列=%+v，預期沒有外部 ID，且超過冪等鍵保存期後 create_unresolved 為真", a)
	}
	// 結果不明的清理終局不能「重試關閉」（不會有結果），只能確認後刪除對應。
	if _, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), purge); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("結果不明的終局重試關閉 err=%v，預期 failed_precondition", err)
	}
}

// 管理員重試建立結果不明的列，冪等鍵還在保存期（30 天）內（含已超過自動重送的 7 天）：以同一把鍵重送，不需要確認、不換鍵、已呼叫的時刻不變（D63，Stripe：保存期內重送同一把鍵回同一個結果）。
// 列在取回 ID 之前保持建立失敗（週期掃描不會把它再標成終局），取回後同一個 transaction 清空失敗並排入建立；全程只有一個外部帳戶。
// mutation：RetryCreate 在超過 UnresolvedDeadline 時要求確認或換新鍵 → 無確認的呼叫失敗，或鍵變了、帳戶數 2，失敗；RetryCreate 清空失敗而不排 replay → 掃描把列再標成終局，列沒有取回 ID，失敗；
// Replay 不以列上的鍵重送 → AcquireKeys 不同，失敗；重設已呼叫的時刻 → called 變了，失敗。
func TestRetryCreateWithinTheKeyRetentionReplaysTheSameKeyWithoutAcknowledgement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, id := f.calledRow(t)
	key := f.key(t, s)
	var called time.Time
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT create_called_at FROM payment.accounts WHERE store_id = $1", s).Scan(&called))
	f.now = f.now.Add(UnresolvedDeadline).Add(time.Hour)
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	if a := f.account(t, s); a.CreateFailedAt == nil || a.CreateUnresolved {
		t.Fatalf("7 天後=%+v，預期建立失敗、還在冪等鍵保存期內所以不是 create_unresolved", a)
	}
	resolves, ensures := f.jobs.count(kindResolve), f.jobs.count(kindEnsure)
	retried, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false)
	must(t, err)
	if f.key(t, s) != key || retried.CreateFailedAt == nil || f.jobs.count(kindResolve) != resolves+1 || f.jobs.count(kindEnsure) != ensures {
		t.Fatalf("重試後=%+v 鍵變了=%v：預期同一把鍵、仍是建立失敗、只排了重送 job", retried, f.key(t, s) != key)
	}
	// 週期掃描在重送完成前不會把列再標成終局，也不會自己重送。
	f.now = f.now.Add(createIdleForTest)
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if a := f.account(t, s); a.ExternalAccountID != nil || a.CreateFailedAt == nil || f.fake.Accounts() != 1 {
		t.Fatalf("掃描之後=%+v", a)
	}
	must(t, f.module.ReplayUnresolvedAccount(ctx, s, false))
	a := f.account(t, s)
	if a.ExternalAccountID == nil || *a.ExternalAccountID != id || a.CreateFailedAt != nil || f.fake.Accounts() != 1 || f.jobs.count(kindEnsure) != ensures+1 {
		t.Fatalf("重送後=%+v 帳戶數=%d，預期取回 %s、清空失敗、排入建立", a, f.fake.Accounts(), id)
	}
	if keys := f.fake.AcquireKeys(); len(keys) != 2 || keys[0] != key || keys[1] != key || f.key(t, s) != key {
		t.Fatalf("重送的冪等鍵=%v，預期兩次都是 %s", keys, key)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND create_called_at = $2", s, called) != 1 {
		t.Fatal("已呼叫的時刻不得改變")
	}
	recs, err := f.audit.List(ctx, adminaudit.Filter{Action: actionRetryCreate, TargetID: fmt.Sprint(s)})
	must(t, err)
	if len(recs) != 1 || !strings.Contains(string(recs[0].Detail), `"replayed_same_key": true`) || !strings.Contains(string(recs[0].Detail), `"key_rotated": false`) {
		t.Fatalf("紀錄=%+v，預期一筆同一把鍵重送、沒有換鍵", recs)
	}
	must(t, f.module.EnsureAccount(ctx, s, false))
	if a := f.account(t, s); a.Status == StatusNotCreated || f.fake.Accounts() != 1 {
		t.Fatalf("接續建立後=%+v 帳戶數=%d", a, f.fake.Accounts())
	}
}

// 超過冪等鍵保存期（30 天）仍結果不明：沒有確認回 failed_precondition、列不變；帶確認才換新的冪等鍵、已呼叫的時刻重設成現在、清空建立失敗並重新排入建立，建立出新的帳戶，
// 並留下已確認、已換鍵的後台紀錄（D63）。create_unresolved 只在這個條件為真。
// mutation：RetryCreate 不檢查確認 → 無確認的呼叫成功，失敗；確認後不換鍵 → 鍵不變，失敗；不重設已呼叫時刻 → 建立立刻又進終局，失敗；保存期內也要求確認 → 上一個測試失敗。
func TestRetryCreateOfAnUnresolvedTerminalRowNeedsAcknowledgementAndRotatesTheKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, _ := f.calledRow(t)
	key := f.key(t, s)
	// 期限內：記建立失敗之後重試，不需要確認、沿用同一把鍵（排入建立 job）。
	f.fake.FailAcquire(ErrPermanent)
	must(t, f.module.EnsureAccount(ctx, s, true))
	if a, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false); err != nil || f.key(t, s) != key || a.CreateUnresolved {
		t.Fatalf("期限內重試 err=%v key 變了=%v", err, f.key(t, s) != key)
	}
	// 超過冪等鍵保存期：進終局，create_unresolved 為真。
	f.now = f.now.Add(IdempotencyRetention).Add(time.Hour)
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	if !f.account(t, s).CreateUnresolved {
		t.Fatal("超過保存期的建立失敗應該標成結果不明")
	}
	var failedAt time.Time
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT create_failed_at FROM payment.accounts WHERE store_id = $1", s).Scan(&failedAt))
	ensures := f.jobs.count(kindEnsure)
	if _, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("沒有確認 err=%v，預期 failed_precondition", err)
	}
	var again time.Time
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT create_failed_at FROM payment.accounts WHERE store_id = $1", s).Scan(&again))
	if f.key(t, s) != key || !again.Equal(failedAt) || f.jobs.count(kindEnsure) != ensures {
		t.Fatal("沒有確認的重試不得改列或排入建立")
	}
	retried, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, true)
	must(t, err)
	if f.key(t, s) == key || retried.CreateFailedAt != nil || retried.CreateUnresolved || f.jobs.count(kindEnsure) != ensures+1 {
		t.Fatalf("確認後=%+v 鍵沒換=%v", retried, f.key(t, s) == key)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND create_called_at = $2", s, f.now) != 1 {
		t.Fatal("已呼叫的時刻應該重設成現在")
	}
	recs, err := f.audit.List(ctx, adminaudit.Filter{Action: actionRetryCreate, TargetID: fmt.Sprint(s)})
	must(t, err)
	if len(recs) != 2 || !strings.Contains(string(recs[0].Detail), `"key_rotated": true`) || !strings.Contains(string(recs[0].Detail), `"acknowledged_no_provider_account": true`) {
		t.Fatalf("確認後的重試要留下已確認與已換鍵的紀錄（沒有確認而被拒絕的不留）：%+v", recs)
	}
	must(t, f.module.EnsureAccount(ctx, s, false))
	if a := f.account(t, s); a.ExternalAccountID == nil || f.fake.Accounts() != 2 || a.CreateFailedAt != nil {
		t.Fatalf("確認重試後應建立新的帳戶：%+v 帳戶數=%d", a, f.fake.Accounts())
	}
}

func (f *fixture) auditCount(t *testing.T, action string) int {
	t.Helper()
	got, err := f.audit.List(context.Background(), adminaudit.Filter{Action: action})
	must(t, err)
	return len(got)
}

// 重試建立與重試關閉在領域變更的同一個 transaction 內寫一筆「已要求」的後台紀錄：領域變更的後半（排入 job）失敗時整個 rollback，紀錄跟著消失、列不變；成功時恰好一筆，
// 不論 provider 之後的呼叫（那在 job 裡）。
// mutation：Record 放在 transaction 之外（用自己的連線）→ 失敗的呼叫之後紀錄仍在；RetryCreate 或 RetryPurge 不寫紀錄 → 成功之後 0 筆，失敗。
func TestRetryCreateAndRetryPurgeRecordOneRequestedEntryAndRollBackWithTheDomainChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	boom := errors.New("enqueue failed")

	create := f.newStore(t)
	f.fake.FailAcquire(ErrPermanent)
	must(t, f.module.EnsureAccount(ctx, create, false))
	f.jobs.failKind, f.jobs.fail = kindEnsure, boom
	if _, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), create, false); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if f.auditCount(t, actionRetryCreate) != 0 || f.account(t, create).CreateFailedAt == nil {
		t.Fatal("領域變更失敗時紀錄必須跟著 rollback、列必須不變")
	}
	_, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), create, false)
	must(t, err)
	recs, err := f.audit.List(ctx, adminaudit.Filter{Action: actionRetryCreate})
	must(t, err)
	if len(recs) != 1 || recs[0].AdminID != 7 || recs[0].TargetType != "payment_account" || recs[0].TargetID != fmt.Sprint(create) || !strings.Contains(string(recs[0].Detail), `"requested"`) {
		t.Fatalf("重試建立的紀錄=%+v", recs)
	}

	purge := f.newStore(t)
	f.ensure(t, purge)
	f.fake.FailClose(fmt.Errorf("%w：拒絕", ErrPermanent))
	must(t, f.module.PurgeAccount(ctx, purge, false))
	f.jobs.failKind, f.jobs.fail = kindPurge, boom
	if _, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), purge); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if f.auditCount(t, actionRetryPurge) != 0 || f.account(t, purge).PurgeBlockedAt == nil {
		t.Fatal("領域變更失敗時紀錄必須跟著 rollback、終局必須不變")
	}
	_, err = f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), purge)
	must(t, err)
	if f.auditCount(t, actionRetryPurge) != 1 {
		t.Fatal("重試關閉成功應該恰好一筆紀錄")
	}
}

// 管理員的重送不能被併進還在重試的掃描 job：掃描 job 對超過 7 天的列只標建立失敗、不重送，後台紀錄卻寫「已重送同一把鍵」（R8）。
// 這裡用真的 River：先有一個排隊中的掃描了結 job，管理員在保存期內重試，必須另有一個 Replay job，跑它之後同一把鍵確實又送了一次。
// mutation：ResolveArgs.Replay 拿掉 `river:"unique"` → 兩個 job 合併成一個（open 的了結 job=1、沒有 replay=true），失敗。
func TestAnAdminReplayIsNotAbsorbedByAPendingSweepResolveJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t, testdb.Jobs())
	jc, err := jobs.NewInsertOnly(f.db)
	must(t, err)
	f.build(t, jc)
	s, id := f.calledRow(t)
	key := f.key(t, s)
	f.now = f.now.Add(createIdleForTest)
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	open := []string{"available", "scheduled", "running", "retryable", "pending"}
	resolveJobs := func(extra string) int64 {
		return f.count(t, "SELECT count(*) FROM river.river_job WHERE kind=$1 AND args->>'store_id'=$2 AND state = ANY($3)"+extra, kindResolve, fmt.Sprint(s), open)
	}
	if n := resolveJobs(""); n != 1 {
		t.Fatalf("掃描排入的了結 job=%d，預期 1", n)
	}
	f.now = f.now.Add(UnresolvedDeadline).Add(time.Hour)
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	if _, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false); err != nil {
		t.Fatal(err)
	}
	if n := resolveJobs(" AND (args->>'replay')='true'"); n != 1 {
		t.Fatalf("管理員的重送被併進掃描 job：replay job=%d，預期 1", n)
	}
	if n := resolveJobs(""); n != 2 {
		t.Fatalf("了結 job=%d，預期掃描一個加重送一個", n)
	}
	must(t, f.module.ReplayUnresolvedAccount(ctx, s, false))
	a := f.account(t, s)
	if keys := f.fake.AcquireKeys(); len(keys) != 2 || keys[1] != key || a.ExternalAccountID == nil || *a.ExternalAccountID != id || a.CreateFailedAt != nil {
		t.Fatalf("重送後=%+v 鍵=%v，預期同一把鍵取回 %s", a, f.fake.AcquireKeys(), id)
	}
}

// 重送期間列被標成等待清理：取回的 ID 不能丟掉（D52）。沒進終局的列寫回 ID 並排入清理，清理走有外部 ID 的路關閉帳戶並刪列；
// 已進終局、原因是 unresolved_creation 的列，有了 ID 之後原因不再成立：清除終局、重設等待清理、重新排入清理，不留在錯誤原因的終局。其他原因的終局不動。
// mutation：Replay 不呼叫 replayIntoPurge（只走 SetPaymentAccountExternal）→ ID 沒寫進列、帳戶沒關，失敗；replayIntoPurge 不 Restart → 終局的列仍 blocked，失敗；
// replayIntoPurge 對任何終局原因都 Restart → provider_refused 的列被清除終局，失敗。
func TestAReplayThatFindsTheRowMarkedForPurgeKeepsTheIDAndPurgeClosesTheAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	failedRow := func() (int64, string) {
		s, id := f.calledRow(t)
		_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET create_failed_at = $2 WHERE store_id = $1", s, f.now)
		must(t, err)
		return s, id
	}
	mark := func(s int64, blockedReason string) {
		_, err := f.db.Conn(ctx).Exec(ctx, `UPDATE payment.accounts SET purge_pending_at = $2,
			purge_blocked_at = CASE WHEN $3 = '' THEN NULL ELSE $2::timestamptz END, purge_blocked_reason = NULLIF($3, '') WHERE store_id = $1`, s, f.now, blockedReason)
		must(t, err)
	}
	type tc struct {
		name, reason string
		restarts     bool
	}
	for _, c := range []tc{{"pending", "", false}, {"unresolved_terminal", "unresolved_creation：建立結果不明", true}, {"other_terminal", "provider_refused：拒絕", false}} {
		s, id := failedRow()
		f.fake.OnAcquire = func() { mark(s, c.reason) }
		purges := f.jobs.count(kindPurge)
		must(t, f.module.ReplayUnresolvedAccount(ctx, s, false))
		var ext *string
		var blocked bool
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT external_account_id, purge_blocked_at IS NOT NULL FROM payment.accounts WHERE store_id = $1", s).Scan(&ext, &blocked))
		if ext == nil || *ext != id {
			t.Fatalf("%s：取回的 ID 被丟掉：%v，預期 %s", c.name, ext, id)
		}
		wantBlocked := c.reason != "" && !c.restarts
		if blocked != wantBlocked {
			t.Fatalf("%s：blocked=%v，預期 %v", c.name, blocked, wantBlocked)
		}
		wantPurges := purges
		if !wantBlocked {
			wantPurges++
		}
		if f.jobs.count(kindPurge) != wantPurges {
			t.Fatalf("%s：清理 job %d→%d，預期 %d", c.name, purges, f.jobs.count(kindPurge), wantPurges)
		}
		if wantBlocked {
			continue
		}
		must(t, f.module.PurgeAccount(ctx, s, false))
		if !slices.Contains(f.fake.Closes(), id) || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 {
			t.Fatalf("%s：清理應關閉 %s 並刪列，關閉=%v", c.name, id, f.fake.Closes())
		}
	}
}

// 管理員在冪等鍵保存期內「重試關閉」結果不明的清理終局（K1，D63，與 RetryCreate 一致）：以同一把鍵重送取回 ID，取回後清除終局並由清理關閉帳戶、刪列；
// 超過保存期不重送，重試關閉被拒絕（只剩刪除對應），create_unresolved 為真讓後台隱藏「重試關閉」。
// mutation：RetryPurge 對 unresolved_creation 一律拒絕（舊的檢查）→ 期限內的重試回 failed_precondition，失敗；replayable 不認清理終局 → Replay 不取回 ID，列仍 blocked，失敗；
// RetryPurge 不看 keyExpired → 超過保存期仍排入重送、AcquireKeys 增加，失敗；createUnresolved 不含清理終局 → 超過保存期的 create_unresolved 為假，失敗。
func TestRetryPurgeOfAnUnresolvedTerminalRowReplaysTheSameKeyWithinTheRetentionAndIsRejectedAfter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	within, id := f.calledRow(t)
	past, _ := f.calledRow(t)
	key := f.key(t, within)
	f.now = f.now.Add(UnresolvedDeadline).Add(time.Hour)
	must(t, f.module.PurgeAccount(ctx, within, false))
	must(t, f.module.PurgeAccount(ctx, past, false))
	blocked := func(s int64) bool {
		return f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_blocked_at IS NOT NULL AND purge_blocked_reason LIKE 'unresolved_creation%'", s) == 1
	}
	if !blocked(within) || !blocked(past) || f.account(t, within).CreateUnresolved {
		t.Fatal("7 天後兩列都應該進 unresolved_creation 終局，且還在保存期內 create_unresolved 為假")
	}
	a, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), within)
	must(t, err)
	if !blocked(within) || a.PurgeBlockedAt == nil {
		t.Fatal("取回 ID 之前列保持終局（掃描不會再標成終局）")
	}
	recs, err := f.audit.List(ctx, adminaudit.Filter{Action: actionRetryPurge, TargetID: fmt.Sprint(within)})
	must(t, err)
	if len(recs) != 1 || !strings.Contains(string(recs[0].Detail), `"replayed_same_key": true`) {
		t.Fatalf("紀錄=%+v，預期一筆 replayed_same_key", recs)
	}
	must(t, f.module.ReplayUnresolvedAccount(ctx, within, false))
	if blocked(within) || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND external_account_id = $2 AND purge_blocked_at IS NULL AND purge_pending_at IS NOT NULL", within, id) != 1 {
		t.Fatal("取回 ID 後應清除終局、寫入 ID、保持等待清理")
	}
	if keys := f.fake.AcquireKeys(); keys[len(keys)-1] != key {
		t.Fatalf("重送的鍵=%v，預期 %s", keys, key)
	}
	must(t, f.module.PurgeAccount(ctx, within, false))
	if !slices.Contains(f.fake.Closes(), id) || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", within) != 0 {
		t.Fatalf("清理應關閉 %s 並刪列，關閉=%v", id, f.fake.Closes())
	}
	// 超過保存期：拒絕、不重送、不留紀錄、create_unresolved 為真。
	f.now = f.now.Add(IdempotencyRetention)
	acquires := len(f.fake.AcquireKeys())
	if !f.account(t, past).CreateUnresolved {
		t.Fatal("超過保存期的清理終局 create_unresolved 應為真")
	}
	if _, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), past); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("超過保存期 err=%v，預期 failed_precondition", err)
	}
	must(t, f.module.ReplayUnresolvedAccount(ctx, past, false))
	if len(f.fake.AcquireKeys()) != acquires || f.auditCount(t, actionRetryPurge) != 1 {
		t.Fatal("超過保存期不得重送，被拒絕的重試不留紀錄")
	}
}

// 建立（Ensure）取得 ID 時，並行的了結（Resolve，掃描排的）先用同一把鍵取回並寫進同一個 ID：Ensure 的寫入被擋下，但同一把鍵同一個帳戶，要繼續第二步（K3）。
// mutation：acquire 在寫入被擋下時不看列上是不是同一個 ID（拿掉 holdsExternal）→ Ensure 提早結束，列沒有完成建立，失敗。
func TestEnsureContinuesToTheSecondStepWhenAConcurrentResolveStoredTheSameID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, id := f.calledRow(t)
	f.fake.OnAcquire = func() { must(t, f.module.ResolveUnresolvedAccount(ctx, s, false)) }
	must(t, f.module.EnsureAccount(ctx, s, false))
	a := f.account(t, s)
	if a.ExternalAccountID == nil || *a.ExternalAccountID != id || a.Status == StatusNotCreated || f.fake.Accounts() != 1 {
		t.Fatalf("交錯之後=%+v 帳戶數=%d，預期完成建立、同一個帳戶 %s", a, f.fake.Accounts(), id)
	}
}

// 了結取回 ID 的呼叫期間列被標成等待清理：ID 不能丟掉，走 replayIntoPurge 寫回並排入清理，清理關閉帳戶（K5，D52）。
// mutation：Resolve 寫入時不看等待清理（一律 SetPaymentAccountExternal）→ ID 沒寫進列，失敗。
func TestResolveThatFindsTheRowMarkedForPurgeKeepsTheIDAndPurgeClosesTheAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, id := f.calledRow(t)
	f.fake.OnAcquire = func() {
		_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE payment.accounts SET purge_pending_at = $2 WHERE store_id = $1", s, f.now)
		must(t, err)
	}
	purges := f.jobs.count(kindPurge)
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND external_account_id = $2", s, id) != 1 || f.jobs.count(kindPurge) != purges+1 {
		t.Fatalf("取回的 ID 被丟掉或沒有排入清理：清理 job %d→%d", purges, f.jobs.count(kindPurge))
	}
	must(t, f.module.PurgeAccount(ctx, s, false))
	if !slices.Contains(f.fake.Closes(), id) || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 0 {
		t.Fatalf("清理應關閉 %s 並刪列，關閉=%v", id, f.fake.Closes())
	}
}

// 了結被金流商永久拒絕之後，週期掃描不再為這列重送（K4）；已排入的了結 job 也不送；管理員重試建立照常清除標記、之後掃描又可以了結。
// mutation：Resolve 的永久失敗不寫 create_refused_at（Refused 固定 false）→ 下一次掃描又排了結 job，失敗；Resolve 不看 create_refused_at → 直接重跑仍送出呼叫，失敗；
// ClearPaymentAccountCreateFailed 不清 create_refused_at → 重試後掃描仍不選它，失敗（或違反 accounts_create_refused_check）。
func TestAPermanentRefusalOfAResolveStopsTheSweepReplaysUntilAnAdminRetries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, _ := f.calledRow(t)
	f.now = f.now.Add(createIdleForTest)
	f.fake.FailAcquire(ErrPermanent)
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND create_failed_at IS NOT NULL AND create_refused_at IS NOT NULL", s) != 1 {
		t.Fatal("永久拒絕應記建立失敗與 create_refused_at")
	}
	acquires, resolves := len(f.fake.AcquireKeys()), f.jobs.count(kindResolve)
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	if f.jobs.count(kindResolve) != resolves || len(f.fake.AcquireKeys()) != acquires {
		t.Fatalf("被拒絕的列又被重送：了結 job %d→%d 取得 %d→%d", resolves, f.jobs.count(kindResolve), acquires, len(f.fake.AcquireKeys()))
	}
	if _, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false); err != nil {
		t.Fatal(err)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND create_failed_at IS NULL AND create_refused_at IS NULL", s) != 1 {
		t.Fatal("管理員重試建立應清除失敗與拒絕標記")
	}
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if f.jobs.count(kindResolve) != resolves+1 {
		t.Fatal("重試之後掃描應該又能了結這列")
	}
}

// 重試建立的後台紀錄只在「需要確認而且照做」（換了新鍵）時記 acknowledged_no_provider_account；不需要確認的重試即使帶了旗標也記 false（K9）。
// mutation：紀錄直接寫呼叫端傳入的旗標 → 這裡記成 true，失敗。
func TestRetryCreateAuditRecordsTheAcknowledgementOnlyWhenItWasRequiredAndActedOn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, _ := f.calledRow(t)
	f.fake.FailAcquire(ErrPermanent)
	must(t, f.module.EnsureAccount(ctx, s, true))
	_, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, true)
	must(t, err)
	recs, err := f.audit.List(ctx, adminaudit.Filter{Action: actionRetryCreate, TargetID: fmt.Sprint(s)})
	must(t, err)
	if len(recs) != 1 || !strings.Contains(string(recs[0].Detail), `"acknowledged_no_provider_account": false`) || !strings.Contains(string(recs[0].Detail), `"key_rotated": false`) {
		t.Fatalf("紀錄=%+v", recs)
	}
}

// unresolvedPurgeTerminal 造一列「呼叫過、沒有外部 ID、清理超過 7 天進 unresolved_creation 終局」的列（還在冪等鍵保存期內），回傳店家與 provider 那邊的帳戶 ID。
func (f *fixture) unresolvedPurgeTerminal(t *testing.T) (int64, string) {
	t.Helper()
	s, id := f.calledRow(t)
	f.now = f.now.Add(UnresolvedDeadline).Add(time.Hour)
	must(t, f.module.PurgeAccount(context.Background(), s, false))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND purge_blocked_reason LIKE 'unresolved_creation%'", s) != 1 {
		t.Fatal("應該進 unresolved_creation 終局")
	}
	return s, id
}

// 管理員要求的重送（RetryPurge 的 Replay）排隊中或進行中時，「刪除對應」與再次「重試關閉」都被拒絕、列與紀錄不變、對外的帳戶 purge_replay_pending 為真；暫時性失敗還會重試時標記留著，
// 重送結束（永久失敗、最後一次嘗試失敗）清除它，之後可以刪除對應；取回 ID 重啟清理時一併清除（K1 後續）。
// mutation 1：DeleteMapping 不看 purge_replay_requested_at → 重送期間刪除成功，失敗；mutation 2：RetryPurge 不寫 RequestPaymentAccountPurgeReplay → 標記從沒有，失敗；
// mutation 3：Replay 結束不清除（拿掉 clearReplayRequest）→ 永久失敗之後仍被拒絕，失敗；mutation 4：Replay 在暫時性失敗還會重試時也清除 → 標記提早消失，失敗。
func TestDeleteMappingAndRetryPurgeAreRejectedWhileAnAdminReplayIsPendingAndAllowedAfterItEnds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, id := f.unresolvedPurgeTerminal(t)
	a, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), s)
	must(t, err)
	if !a.PurgeReplayPending || !f.account(t, s).PurgeReplayPending {
		t.Fatal("重送排入後 purge_replay_pending 應為真")
	}
	reject := func(label string) {
		t.Helper()
		if err := f.module.DeleteAccountMapping(ctx, admin(actionDeleteMapping), s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("%s：刪除對應 err=%v，預期 failed_precondition", label, err)
		}
		if _, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("%s：再次重試關閉 err=%v，預期 failed_precondition", label, err)
		}
		if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", s) != 1 || f.auditCount(t, actionDeleteMapping) != 0 || f.auditCount(t, actionRetryPurge) != 1 {
			t.Fatalf("%s：被拒絕的請求不得改列、不得留紀錄", label)
		}
	}
	reject("排隊中")
	boom := errors.New("provider down")
	f.fake.FailAcquire(boom)
	if err := f.module.ReplayUnresolvedAccount(ctx, s, false); !errors.Is(err, boom) {
		t.Fatalf("暫時性失敗要回傳給 River 重試：%v", err)
	}
	reject("暫時性失敗、還會重試")
	f.fake.FailAcquire(ErrPermanent)
	must(t, f.module.ReplayUnresolvedAccount(ctx, s, false))
	if f.account(t, s).PurgeReplayPending || f.account(t, s).PurgeBlockedAt == nil {
		t.Fatal("永久失敗後標記應清除、列留在終局")
	}
	// 結束後可以再要求一次；取回 ID 重啟清理時標記一併清除。
	if _, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), s); err != nil {
		t.Fatal(err)
	}
	must(t, f.module.ReplayUnresolvedAccount(ctx, s, false))
	if a := f.account(t, s); a.PurgeReplayPending || a.PurgeBlockedAt != nil || a.ExternalAccountID == nil || *a.ExternalAccountID != id {
		t.Fatalf("取回 ID 後應清除終局與標記：%+v", a)
	}
	// 最後一次嘗試仍失敗也清除，這時可以刪除對應。
	other, _ := f.unresolvedPurgeTerminal(t)
	_, err = f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), other)
	must(t, err)
	f.fake.FailAcquire(boom)
	must(t, f.module.ReplayUnresolvedAccount(ctx, other, true))
	if err := f.module.DeleteAccountMapping(ctx, admin(actionDeleteMapping), other); err != nil {
		t.Fatalf("重送結束後應可刪除對應：%v", err)
	}
}

// 防線：重送取回外部 ID、列卻已不在時（正常不會發生，後端在重送期間拒絕刪除對應），就地關閉那個外部帳戶，並記 error 等級的 log 帶 store_id 與帳戶 ID；
// 這是系統動作，沒有管理員可以歸屬，所以不寫 adminaudit 紀錄。
// mutation：row-gone 分支回 nil、不呼叫 closeOrphan → 帳戶沒被關閉、沒有 error log，失敗。
func TestAReplayThatFindsTheRowGoneClosesTheObtainedAccountAndLogsAnError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, id := f.unresolvedPurgeTerminal(t)
	_, err := f.module.RetryAccountPurge(ctx, admin(actionRetryPurge), s)
	must(t, err)
	f.fake.OnAcquire = func() {
		if _, err := f.db.Conn(ctx).Exec(ctx, "DELETE FROM payment.accounts WHERE store_id = $1", s); err != nil {
			t.Error(err)
		}
	}
	must(t, f.module.ReplayUnresolvedAccount(ctx, s, false))
	if !slices.Contains(f.fake.Closes(), id) {
		t.Fatalf("列已不在時應關閉取回的外部帳戶 %s，關閉=%v", id, f.fake.Closes())
	}
	var found bool
	for _, line := range strings.Split(f.logs.String(), "\n") {
		if strings.Contains(line, "level=ERROR") && strings.Contains(line, fmt.Sprintf("store_id=%d", s)) && strings.Contains(line, "external_account_id="+id) {
			found = true
		}
	}
	if !found {
		t.Fatalf("缺少帶 store_id 與帳戶 ID 的 error log：%s", f.logs.String())
	}
	if f.auditCount(t, actionDeleteMapping) != 0 || f.auditCount(t, actionRetryPurge) != 1 {
		t.Fatal("系統關閉不寫後台紀錄")
	}
}

// 已經建立失敗（重試用完，沒有拒絕標記）的列，Resolve 遇到金流商的永久拒絕也要記 create_refused_at（保留原本的失敗時刻），之後的掃描不再重送；管理員重試建立仍可用並清除它（K4 後續）。
// mutation：MarkPaymentAccountCreateFailed 的 WHERE 加回 `create_failed_at IS NULL` → 已失敗的列沒有 create_refused_at，掃描繼續重送，失敗；
// create_failed_at 改成 sqlc.arg(now) 不用 COALESCE → 失敗時刻被改寫，失敗。
func TestAPermanentRefusalOfAResolveOnAnAlreadyFailedRowStopsTheSweepAndKeepsTheFailureTime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s, _ := f.calledRow(t)
	f.fake.FailAcquire(errors.New("provider down"))
	must(t, f.module.EnsureAccount(ctx, s, true))
	failedAt := f.account(t, s).CreateFailedAt
	if failedAt == nil || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND create_refused_at IS NULL", s) != 1 {
		t.Fatal("暫時性失敗用完重試：應有失敗時刻、沒有拒絕標記")
	}
	f.now = f.now.Add(createIdleForTest)
	f.fake.FailAcquire(ErrPermanent)
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	if a := f.account(t, s); a.CreateFailedAt == nil || !a.CreateFailedAt.Equal(*failedAt) {
		t.Fatalf("失敗時刻應保留原本的 %v：%+v", failedAt, a)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND create_refused_at IS NOT NULL", s) != 1 {
		t.Fatal("永久拒絕應對已失敗的列記 create_refused_at")
	}
	acquires, resolves := len(f.fake.AcquireKeys()), f.jobs.count(kindResolve)
	if _, _, err := f.module.SweepAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	must(t, f.module.ResolveUnresolvedAccount(ctx, s, false))
	if f.jobs.count(kindResolve) != resolves || len(f.fake.AcquireKeys()) != acquires {
		t.Fatal("被拒絕的已失敗列不應再被掃描重送")
	}
	if _, err := f.module.RetryAccountCreate(ctx, admin(actionRetryCreate), s, false); err != nil {
		t.Fatal(err)
	}
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1 AND create_failed_at IS NULL AND create_refused_at IS NULL", s) != 1 {
		t.Fatal("管理員重試建立應清除失敗與拒絕標記")
	}
}

// 同步存待補條件的最早期限（每次整列覆寫）與第一次啟用的時間（只寫一次）；事件帶這次讀到的期限，FirstActivation 只在第一次啟用的那一次為真。
// mutation：FirstActivation 改成 applied.FirstActivatedAt != nil → 之後每次啟用都為真；first_activated_at 拿掉 COALESCE → 再啟用時被重設。
func TestSyncStoresTheDeadlineAndFirstActivationAndPublishesThem(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s := f.newStore(t)
	deadline := f.now.Add(7 * 24 * time.Hour).UTC()

	needs := needsSellerSnapshot
	needs.RequirementsDeadline = &deadline
	f.set(t, s, needs)
	if a := f.account(t, s); a.RequirementsDeadline == nil || !a.RequirementsDeadline.Equal(deadline) || a.FirstActivatedAt != nil {
		t.Fatalf("補件中：%+v", a)
	}

	f.now = f.now.Add(time.Hour)
	activatedAt := f.now
	f.set(t, s, enabledSnapshot)
	if a := f.account(t, s); a.RequirementsDeadline != nil || a.FirstActivatedAt == nil || !a.FirstActivatedAt.Equal(activatedAt) {
		t.Fatalf("第一次啟用：%+v", a)
	}

	f.now = f.now.Add(time.Hour)
	f.set(t, s, needs)
	f.now = f.now.Add(time.Hour)
	f.set(t, s, enabledSnapshot)
	if a := f.account(t, s); a.FirstActivatedAt == nil || !a.FirstActivatedAt.Equal(activatedAt) {
		t.Fatalf("再啟用不得重設第一次啟用的時間：%+v", a)
	}

	if len(f.changes) != 4 {
		t.Fatalf("events=%+v", f.changes)
	}
	type view struct {
		first    bool
		deadline *time.Time
	}
	var got []view
	for _, e := range f.changes {
		got = append(got, view{e.FirstActivation, e.RequirementsDeadline})
	}
	if got[0].first || got[0].deadline == nil || !got[0].deadline.Equal(deadline) ||
		!got[1].first || got[1].deadline != nil ||
		got[2].first || got[2].deadline == nil || !got[2].deadline.Equal(deadline) ||
		got[3].first || got[3].deadline != nil {
		t.Fatalf("事件欄位：%+v", got)
	}
}

// 訂閱者失敗時整筆同步 rollback：期限與第一次啟用的時間都不寫，重試後仍是第一次啟用。
// mutation：Publish 的錯誤被吞掉 → 帳戶已寫入第一次啟用的時間，失敗。
func TestSyncRollbackKeepsTheFirstActivationForTheRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	boom := errors.New("訂閱者失敗")
	failing := true
	events.Subscribe(f.bus, func(context.Context, PayoutAccountChanged) error {
		if failing {
			return boom
		}
		return nil
	})
	f.fake.Set(external, enabledSnapshot)
	if _, err := f.module.SyncAccount(ctx, s); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if a := f.account(t, s); a.FirstActivatedAt != nil || a.TransfersActive {
		t.Fatalf("rollback 後的帳戶=%+v", a)
	}
	failing = false
	f.changes = nil
	if _, err := f.module.SyncAccount(ctx, s); err != nil {
		t.Fatal(err)
	}
	if len(f.changes) != 1 || !f.changes[0].FirstActivation {
		t.Fatalf("重試要是第一次啟用：%+v", f.changes)
	}
}
