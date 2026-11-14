//go:build integration

package inbox_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type queue struct {
	mu   sync.Mutex
	args []inbox.DispatchArgs
}

func (q *queue) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if a, ok := args.(inbox.DispatchArgs); ok {
		q.args = append(q.args, a)
	}
	return nil
}

type fixture struct {
	db    *db.DB
	jobs  *queue
	inbox *inbox.Inbox
	now   time.Time
	calls []inbox.Event
	fail  error
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{db: pg.NewDB(t, testdb.Migrations(migrate.Module{Name: inbox.Schema, Dir: inbox.Migrations()})), jobs: &queue{}, now: time.Date(2026, 11, 6, 4, 0, 0, 0, time.UTC)}
	f.inbox = inbox.New(inbox.Deps{DB: f.db, Jobs: f.jobs, Now: func() time.Time { return f.now }, Logger: slog.New(slog.DiscardHandler)})
	f.inbox.Register(inbox.Source{Provider: "fake", Retention: 24 * time.Hour, Handle: func(_ context.Context, e inbox.Event) error {
		f.calls = append(f.calls, e)
		return f.fail
	}})
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) record(t *testing.T, id, route string) bool {
	t.Helper()
	var inserted bool
	must(t, f.db.InTx(context.Background(), func(ctx context.Context) error {
		var err error
		inserted, err = f.inbox.Record(ctx, inbox.Event{Provider: "fake", EventID: id, RouteKey: route, Payload: []byte("payload-" + id)})
		return err
	}))
	return inserted
}

func (f *fixture) count(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, sql, args...).Scan(&n))
	return n
}

// (provider, event_id) 唯一：重送的事件不寫入、不排派送；寫入與排入派送必須在 transaction 內；沒有登記的 provider 是錯誤。
// mutation：insert 拿掉 ON CONFLICT DO NOTHING 的判斷（一律排派送）→ 第二次也排了派送，失敗。
func TestRecordDeduplicatesOnProviderAndEventID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	if !f.record(t, "evt_1", "acct_1") || f.record(t, "evt_1", "acct_1") {
		t.Fatal("第一次應寫入、第二次不寫入")
	}
	if n := f.count(t, "SELECT count(*) FROM inbox.events"); n != 1 || len(f.jobs.args) != 1 || f.jobs.args[0] != (inbox.DispatchArgs{Provider: "fake", EventID: "evt_1"}) {
		t.Fatalf("列=%d 派送=%v", n, f.jobs.args)
	}
	if _, err := f.inbox.Record(ctx, inbox.Event{Provider: "fake", EventID: "evt_2", RouteKey: "a", Payload: []byte("x")}); !errors.Is(err, inbox.ErrNoTransaction) {
		t.Fatalf("transaction 外 err=%v", err)
	}
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		_, err := f.inbox.Record(ctx, inbox.Event{Provider: "other", EventID: "evt_3", RouteKey: "a", Payload: []byte("x")})
		return err
	})
	if !errors.Is(err, inbox.ErrUnknownProvider) {
		t.Fatalf("沒有登記的 provider err=%v", err)
	}
}

// 派送在 transaction 內呼叫 handler：handler 失敗時 rollback、不記派送成功（River 重試）；補掃只重派收到超過寬限、沒有派送成功也沒有處理的列；
// 派送成功或已處理的列不再派送。
// mutation 1：Dispatch 不論 handler 結果都記 dispatched_at → 補掃沒有重派失敗的事件，失敗。
// mutation 2：補掃的條件拿掉 dispatched_at IS NULL → 派送成功的事件也被重派，失敗。
func TestAFailedDispatchIsRetriedAndOnlyUndispatchedEventsAreSwept(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.record(t, "evt_fail", "acct_1")
	f.record(t, "evt_ok", "acct_2")
	f.record(t, "evt_done", "acct_3")
	f.fail = errors.New("handler 失敗")
	if err := f.inbox.Dispatch(ctx, "fake", "evt_fail"); !errors.Is(err, f.fail) {
		t.Fatalf("err=%v", err)
	}
	f.fail = nil
	must(t, f.inbox.Dispatch(ctx, "fake", "evt_ok"))
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.inbox.MarkProcessed(ctx, "fake", "evt_done", "test") }))
	must(t, f.inbox.Dispatch(ctx, "fake", "evt_done"))
	if len(f.calls) != 2 || string(f.calls[1].Payload) != "payload-evt_ok" || f.calls[1].RouteKey != "acct_2" {
		t.Fatalf("handler 呼叫=%+v（已處理的事件不派送）", f.calls)
	}
	f.jobs.args = nil
	if n, err := f.inbox.Sweep(ctx); err != nil || n != 0 {
		t.Fatalf("寬限內的補掃=%d %v", n, err)
	}
	f.now = f.now.Add(10 * time.Minute)
	n, err := f.inbox.Sweep(ctx)
	must(t, err)
	if n != 1 || len(f.jobs.args) != 1 || f.jobs.args[0].EventID != "evt_fail" {
		t.Fatalf("補掃=%d %v，應該只有失敗的事件", n, f.jobs.args)
	}
	must(t, f.inbox.Dispatch(ctx, "fake", "evt_fail"))
	if n, err := f.inbox.Sweep(ctx); err != nil || n != 0 {
		t.Fatalf("重試成功後補掃=%d %v", n, err)
	}
}

// 整批標記只涵蓋同一個路由鍵、收到時間不晚於上限的未處理事件；RoutePending 與 StaleRoutes 依未處理的列回答。清理只刪已處理而且過了保存期的列。
// mutation 1：MarkRouteProcessed 拿掉 received_at 的上限 → 晚到的事件也被標記，失敗。
// mutation 2：Purge 拿掉 processed_at IS NOT NULL → 未處理的舊事件被刪，失敗。
func TestRouteMarkingAndPurgeRespectTheirBounds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	t0 := f.now
	f.record(t, "evt_a1", "acct_a")
	f.record(t, "evt_b1", "acct_b")
	f.now = t0.Add(time.Second)
	f.record(t, "evt_a2", "acct_a")
	var marked int64
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		marked, err = f.inbox.MarkRouteProcessed(ctx, "fake", "acct_a", t0, "sync")
		return err
	}))
	if marked != 1 {
		t.Fatalf("標記=%d", marked)
	}
	if pending, err := f.inbox.RoutePending(ctx, "fake", "acct_a"); err != nil || !pending {
		t.Fatalf("acct_a 還有晚到的事件：%v %v", pending, err)
	}
	routes, err := f.inbox.StaleRoutes(ctx, "fake", t0.Add(time.Millisecond), "", 10)
	must(t, err)
	if len(routes) != 1 || routes[0] != "acct_b" {
		t.Fatalf("舊的未處理路由=%v", routes)
	}
	f.now = t0.Add(25 * time.Hour)
	n, err := f.inbox.Purge(ctx)
	must(t, err)
	if n != 1 || f.count(t, "SELECT count(*) FROM inbox.events WHERE event_id = 'evt_a1'") != 0 || f.count(t, "SELECT count(*) FROM inbox.events") != 2 {
		t.Fatalf("清理=%d", n)
	}
}
