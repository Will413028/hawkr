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
	"testing"
	"time"

	"github.com/riverqueue/river"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
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
}

func (q *queue) Enqueue(ctx context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	q.mu.Lock()
	defer q.mu.Unlock()
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

// owners 是擁有者查詢的 fake：預設營運中，closed 裡的不是。
type owners struct {
	mu     sync.Mutex
	closed map[int64]bool
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
	f := &fixture{db: pg.NewDB(t, steps...), bus: events.NewBus(), jobs: &queue{}, fake: NewFake(), owners: &owners{closed: map[int64]bool{}}, now: time.Date(2026, 11, 6, 4, 0, 0, 0, time.UTC)}
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

var nextStore int64 = 100

// request 模擬擁有者核准的 transaction：寫「建立中」的帳戶列並排入建立 job。
func (f *fixture) request(t *testing.T, storeID int64, email string) {
	t.Helper()
	must(t, f.db.InTx(context.Background(), func(ctx context.Context) error { return f.module.RequestAccount(ctx, storeID, &email, f.now) }))
}

func (f *fixture) newStore(t *testing.T) int64 {
	t.Helper()
	nextStore++
	f.request(t, nextStore, "seller@example.com")
	return nextStore
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

// 已有帳戶列時 RequestAccount 不改它（INSERT ON CONFLICT DO NOTHING：核准的 transaction 不鎖 payment 的既有列，platform/lockorder）：聯絡信箱維持
// 第一次的快照；還沒有外部帳戶的列重新排入建立 job，已建立或建立失敗的不排。沒有信箱時 failed_precondition。
// mutation：InsertPaymentAccount 改成 ON CONFLICT DO UPDATE SET contact_email → 第二次的信箱覆寫了列，失敗。
func TestRequestKeepsAnExistingRowAndItsContactEmail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.request(t, 7, "first@example.com")
	f.request(t, 7, "second@example.com")
	var email string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT contact_email FROM payment.accounts WHERE store_id = 7").Scan(&email))
	if email != "first@example.com" {
		t.Fatalf("既有列的信箱=%q，預期保留第一次的快照", email)
	}
	if got := f.jobs.storeIDs(t, kindEnsure); !slices.Equal(got, []int64{7, 7}) {
		t.Fatalf("還沒有外部帳戶時的建立 job=%v", got)
	}
	f.ensure(t, 7)
	f.request(t, 7, "third@example.com")
	if got := f.jobs.storeIDs(t, kindEnsure); len(got) != 2 {
		t.Fatalf("已建立的帳戶又排了建立 job：%v", got)
	}
	if got := f.fake.EnsureEmails(); !slices.Equal(got, []string{"first@example.com"}) {
		t.Fatalf("provider 收到的信箱=%v", got)
	}
	err := f.db.InTx(ctx, func(ctx context.Context) error { return f.module.RequestAccount(ctx, 8, nil, f.now) })
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = 8") != 0 {
		t.Fatalf("沒有信箱 err=%v", err)
	}
}

// 沒有設定 provider 的模組：RequestAccount 什麼都不做，開通相關的呼叫回 failed_precondition，webhook 回 failed_precondition。
func TestUnconfiguredProviderWritesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	m := New(Deps{DB: f.db, Events: f.bus, Jobs: f.jobs, Inbox: inbox.New(inbox.Deps{DB: f.db, Jobs: f.jobs, Now: time.Now, Logger: slog.Default()}), Owners: f.owners, Now: time.Now, Logger: slog.Default()})
	email := "a@example.com"
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return m.RequestAccount(ctx, 1, &email, f.now) }))
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
// mutation：SyncArgs 拿掉 `river:"unique"`（或 ByArgs）→ 同一擁有者排了多個 job，失敗。
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
// mutation：Dispatch 在 handler 失敗時仍記下 dispatched_at → 補掃不重派，失敗。
func TestAFailedDispatchIsRetriedAndSweptAgain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	s := f.newStore(t)
	external := f.ensure(t, s)
	h, body := event("evt_1", external)
	must(t, f.module.ReceiveEvent(ctx, h, body))
	_, err := f.db.Conn(ctx).Exec(ctx, "ALTER TABLE payment.accounts RENAME TO accounts_gone")
	must(t, err)
	if err := f.inbox.Dispatch(ctx, FakeName, "evt_1"); err == nil {
		t.Fatal("handler 失敗時派送沒有回錯誤")
	}
	if n := f.count(t, "SELECT count(*) FROM inbox.events WHERE event_id = 'evt_1' AND dispatched_at IS NULL"); n != 1 {
		t.Fatal("失敗的派送記成了派送成功")
	}
	_, err = f.db.Conn(ctx).Exec(ctx, "ALTER TABLE payment.accounts_gone RENAME TO accounts")
	must(t, err)
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
			(store_id, provider, external_account_id, contact_email, requirement_collection, status, transfers_active, status_reason, created_at, updated_at)
			VALUES ($1, 'fake', $2, 'seller@example.com', 'PROVIDER', $3, $3 = 'ENABLED', $4, $5, $5)`, storeID, external, status, reason, f.now.Add(-time.Hour))
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

// 擁有者的個資清理：ForgetOwners 在呼叫方的 transaction 內只排入刪除 job（不碰帳戶列），job 刪掉那些擁有者的列，其他擁有者的不動。
// mutation：Purge 不帶 store_ids 條件 → 其他擁有者的列也被刪，失敗。
func TestForgetOwnersDeletesTheirAccountsInItsOwnJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	gone, kept := f.newStore(t), f.newStore(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.ForgetOwners(ctx, []int64{gone}) }))
	if f.count(t, "SELECT count(*) FROM payment.accounts") != 2 || f.jobs.count(kindPurge) != 1 {
		t.Fatal("ForgetOwners 直接刪了列或沒有排 job")
	}
	must(t, f.module.PurgeAccounts(ctx, []int64{gone}))
	if f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", gone) != 0 || f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id = $1", kept) != 1 {
		t.Fatal("清理刪錯了列")
	}
}
