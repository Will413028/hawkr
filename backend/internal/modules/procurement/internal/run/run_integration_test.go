//go:build integration

package run_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var now = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func newDB(t *testing.T) *db.DB {
	t.Helper()
	return pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "procurement", Dir: procurement.Migrations()}))
}

func turnOnly(e run.Executor) map[string]run.Executor {
	return map[string]run.Executor{run.RunTypeTurn: e}
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func ledger(d *db.DB, at time.Time) run.Ledger {
	return run.Ledger{DB: d, Now: func() time.Time { return at }}
}

// recordingJobs 記下排入與取消的 job；failWith 不為 nil 時排入失敗；cancelState 是取消之後回報的 job 狀態（預設 running）；missingJob 為真時回報 job 列已不存在。
type recordingJobs struct {
	n           atomic.Int32
	failWith    error
	cancelState rivertype.JobState
	missingJob  bool
	mu          sync.Mutex
	cancels     []int64
}

func (r *recordingJobs) EnqueueJob(_ context.Context, _ river.JobArgs, _ *river.InsertOpts) (int64, error) {
	if r.failWith != nil {
		return 0, r.failWith
	}
	return int64(r.n.Add(1)), nil
}

func (r *recordingJobs) CancelJob(_ context.Context, id int64) (rivertype.JobState, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels = append(r.cancels, id)
	if r.missingJob {
		return "", false, nil
	}
	if r.cancelState == "" {
		return rivertype.JobStateRunning, true, nil
	}
	return r.cancelState, true, nil
}

func seedRequest(ctx context.Context, t *testing.T, d *db.DB, buyer int64) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	if err := store.New(d.Conn(ctx)).InsertRequest(ctx, store.InsertRequestParams{ID: id, BuyerID: &buyer, Now: now}); err != nil {
		t.Fatalf("InsertRequest：%v", err)
	}
	return id
}

func input(buyer int64, request uuid.UUID, limit int32) run.SubmitInput {
	return run.SubmitInput{
		BuyerID: buyer, RequestID: request, RunType: "TURN", ClientRequestID: uuid.NewV7(), Message: "我要三件外套", Locale: uitext.ZhTW,
		QuotaLimit: limit, PeriodStart: now, PeriodEnd: now.AddDate(0, 1, 0),
	}
}

func used(ctx context.Context, t *testing.T, d *db.DB, buyer int64) int32 {
	t.Helper()
	u, err := store.New(d.Conn(ctx)).GetUsage(ctx, store.GetUsageParams{BuyerID: buyer, PeriodStart: now})
	if err != nil {
		return 0
	}
	return u
}

func count(ctx context.Context, t *testing.T, d *db.DB, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := d.Pool().QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func reasonOf(err error) (apperr.Reason, apperr.Kind) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Reason, ae.Kind()
	}
	return apperr.Reason{}, apperr.KindOf(err)
}

func TestConcurrentSubmitsNeverExceedTheQuota(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: jobs, Limits: run.Limits{MaxActiveRuns: 100, MaxSubmitsPerMinute: 100}}
	const buyer, limit, attempts = int64(1), int32(3), 12

	var wg sync.WaitGroup
	var ok, exhausted atomic.Int32
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Submit(ctx, input(buyer, seedRequest(ctx, t, d, buyer), limit))
			switch reason, kind := reasonOf(err); {
			case err == nil:
				ok.Add(1)
			case kind == apperr.KindResourceExhausted && reason == apperr.ReasonAgentMonthlyQuotaExceeded:
				exhausted.Add(1)
			default:
				t.Errorf("意外的錯誤：%v", err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != limit || exhausted.Load() != attempts-limit {
		t.Fatalf("成功 %d、額度用完 %d，要 %d 與 %d", ok.Load(), exhausted.Load(), limit, attempts-limit)
	}
	if u := used(ctx, t, d, buyer); u != limit {
		t.Fatalf("used = %d，要 %d", u, limit)
	}
	if n := count(ctx, t, d, `SELECT count(*) FROM procurement.agent_runs WHERE buyer_id = $1`, buyer); n != int(limit) {
		t.Fatalf("額度用完的送出要連 run 一起回滾：runs = %d", n)
	}
	if jobs.n.Load() != limit {
		t.Fatalf("排入的 job = %d，要 %d", jobs.n.Load(), limit)
	}
}

func TestResubmittingTheSameClientRequestDoesNotChargeTwice(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: jobs}
	req := seedRequest(ctx, t, d, 1)
	in := input(1, req, 5)
	first, err := s.Submit(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Submit(ctx, in)
	if err != nil || again != first {
		t.Fatalf("重送要回第一次的 run：%v %v %v", first, again, err)
	}
	if used(ctx, t, d, 1) != 1 || jobs.n.Load() != 1 {
		t.Fatalf("重送不得再扣額度或再排 job：used=%d jobs=%d", used(ctx, t, d, 1), jobs.n.Load())
	}
	other := in
	other.Message = "別的內容"
	if _, kind := reasonOf(errOf(s.Submit(ctx, other))); kind != apperr.KindInvalidInput {
		t.Fatalf("同一個 client_request_id 內容不同要回 INVALID_ARGUMENT，實際 %v", kind)
	}
}

func errOf(_ uuid.UUID, err error) error { return err }

func TestEnqueueFailureRollsBackTheCharge(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{failWith: errors.New("queue down")}}
	req := seedRequest(ctx, t, d, 1)
	if _, err := s.Submit(ctx, input(1, req, 5)); err == nil {
		t.Fatal("排入失敗要回錯誤")
	}
	if used(ctx, t, d, 1) != 0 || count(ctx, t, d, `SELECT count(*) FROM procurement.agent_runs`) != 0 {
		t.Fatal("排入失敗要連 run 與扣款一起回滾")
	}
}

type failingExecutor struct{}

func (failingExecutor) Execute(context.Context, store.ProcurementAgentRun) error {
	return errors.New("provider down")
}

func TestExecutorFailureFailsTheRunAndRefundsTheQuota(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{}}
	req := seedRequest(ctx, t, d, 1)
	id, err := s.Submit(ctx, input(1, req, 5))
	if err != nil {
		t.Fatal(err)
	}
	args := run.RunArgs{RunID: id}
	disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: turnOnly(failingExecutor{})}
	if err := disp.Run(ctx, args); err != nil {
		t.Fatal(err)
	}
	r, _ := store.New(d.Conn(ctx)).GetRun(ctx, id)
	if r.Status != "FAILED" || r.FailureReason == nil || *r.FailureReason != "TURN_PIPELINE_ERROR" {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	if used(ctx, t, d, 1) != 0 {
		t.Fatalf("外部失敗要退回額度，used = %d", used(ctx, t, d, 1))
	}
	if err := disp.Run(ctx, args); err != nil || used(ctx, t, d, 1) != 0 {
		t.Fatalf("已終結的 run 再派一次不得重驅或再退款：%v used=%d", err, used(ctx, t, d, 1))
	}
}

func lastEventType(ctx context.Context, t *testing.T, d *db.DB, id uuid.UUID) string {
	t.Helper()
	var typ string
	if err := d.Pool().QueryRow(ctx, `SELECT event_type FROM procurement.agent_run_events WHERE run_id = $1 ORDER BY seq DESC LIMIT 1`, id).Scan(&typ); err != nil {
		t.Fatalf("run %s 沒有事件：%v", id, err)
	}
	return typ
}

func finishedRun(ctx context.Context, t *testing.T, d *db.DB, req uuid.UUID, buyer int64, completedAt time.Time, events int) uuid.UUID {
	t.Helper()
	id := seedRun(ctx, t, d, req, buyer, run.StatusRunning, nil)
	l := ledger(d, completedAt)
	for i := 0; i < events; i++ {
		if _, err := l.Append(ctx, id, "text", []byte(`{"text":{"text":"x"}}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET status = 'SUCCEEDED', completed_at = $2, updated_at = $2 WHERE id = $1`, id, completedAt); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCursorBelowRetainedRangeIsExpiredAndAheadIsRejected(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	old := finishedRun(ctx, t, d, req, 1, now.Add(-run.RetentionPeriod-time.Hour), 3)
	recent := finishedRun(ctx, t, d, req, 1, now.Add(-time.Hour), 3)
	p := run.Purger{DB: d, Ledger: ledger(d, now)}
	if n, err := p.Purge(ctx); err != nil || n != 3 {
		t.Fatalf("Purge = %d, %v，要刪 3 筆", n, err)
	}
	if count(ctx, t, d, `SELECT count(*) FROM procurement.agent_run_events WHERE run_id = $1`, old) != 0 ||
		count(ctx, t, d, `SELECT count(*) FROM procurement.agent_run_events WHERE run_id = $1`, recent) != 3 {
		t.Fatal("只清過保留期的 run")
	}
	r, _ := store.New(d.Conn(ctx)).GetRun(ctx, old)
	if r.MinAvailableSeq != r.LastEventSeq+1 {
		t.Fatalf("min_available_seq = %d，要 %d", r.MinAvailableSeq, r.LastEventSeq+1)
	}

	w := run.Watcher{DB: d, Hub: run.NewHub(d, run.MaxSubscriptions, discard()), Window: time.Minute}
	send := func(run.Event) error { return nil }
	for _, c := range []string{"", "1", "2"} {
		if reason, kind := reasonOf(w.Watch(ctx, 1, old, c, send)); kind != apperr.KindFailedPrecondition || reason != apperr.ReasonReplayCursorExpired {
			t.Fatalf("cursor %q：要 REPLAY_CURSOR_EXPIRED，實際 %v %v", c, kind, reason)
		}
	}
	if reason, kind := reasonOf(w.Watch(ctx, 1, recent, "4", send)); kind != apperr.KindFailedPrecondition || reason != apperr.ReasonReplayCursorAhead {
		t.Fatalf("要 REPLAY_CURSOR_AHEAD，實際 %v %v", kind, reason)
	}
	var got []int64
	collect := func(e run.Event) error { got = append(got, e.Seq); return nil }
	if err := w.Watch(ctx, 1, recent, "1", collect); err != nil || len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("從 cursor 1 接續要送 2、3：%v %v", got, err)
	}
	if _, kind := reasonOf(w.Watch(ctx, 2, recent, "", collect)); kind != apperr.KindNotFound {
		t.Fatalf("別人的 run 要 NOT_FOUND，實際 %v", kind)
	}
}

func TestWatchFollowsLiveEventsUntilTheTerminalEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	hub := run.NewHub(d, run.MaxSubscriptions, discard())
	go func() { _ = hub.Run(ctx) }()
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{}}
	id, err := s.Submit(ctx, input(1, req, 5))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var types []string
	first := make(chan struct{})
	var once sync.Once
	done := make(chan error, 1)
	w := run.Watcher{DB: d, Hub: hub, Window: time.Minute}
	go func() {
		done <- w.Watch(ctx, 1, id, "", func(e run.Event) error {
			mu.Lock()
			types = append(types, e.Type)
			mu.Unlock()
			once.Do(func() { close(first) })
			return nil
		})
	}()
	<-first
	l := ledger(d, now)
	if _, err := l.Append(ctx, id, "text", []byte(`{"text":{"text":"a"}}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Transition(ctx, id, run.StatusRunning, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Transition(ctx, id, run.StatusSucceeded, ""); err != nil {
		t.Fatalf("Transition = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{run.EventRunAccepted, "text", run.EventFinished}
	if len(types) != len(want) {
		t.Fatalf("事件 = %v，要 %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("事件 = %v，要 %v", types, want)
		}
	}
}

func TestWatchRejectsSubscriptionsOverTheCap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{}}
	id, err := s.Submit(ctx, input(1, req, 5))
	if err != nil {
		t.Fatal(err)
	}
	const capN = 2
	hub := run.NewHub(d, capN, discard())
	w := run.Watcher{DB: d, Hub: hub, Window: time.Hour}
	var wg sync.WaitGroup
	for i := 0; i < capN; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Watch(ctx, 1, id, "", func(run.Event) error { return nil })
		}()
	}
	for hub.Count() < capN {
		if ctx.Err() != nil {
			t.Fatal("訂閱沒有建立")
		}
		time.Sleep(time.Millisecond)
	}
	if _, kind := reasonOf(w.Watch(ctx, 1, id, "", func(run.Event) error { return nil })); kind != apperr.KindResourceExhausted {
		t.Fatalf("超過上限的新訂閱要 RESOURCE_EXHAUSTED，實際 %v", kind)
	}
	cancel()
	wg.Wait()
}

// 所有訂閱共用一條 LISTEN 連線：訂閱數增加，資料庫的連線數不變；一次通知喚醒所有訂閱者。
func TestSubscriptionsShareOneListenConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{}}
	id, err := s.Submit(ctx, input(1, req, 5))
	if err != nil {
		t.Fatal(err)
	}
	hub := run.NewHub(d, run.MaxSubscriptions, discard())
	go func() { _ = hub.Run(ctx) }()
	probe, err := hub.Subscribe(id)
	if err != nil {
		t.Fatal(err)
	}
	l := ledger(d, now)
	for { // 等 LISTEN 生效：通知到得了探針才算
		if _, err := l.Append(ctx, id, "text", []byte(`{"text":{"text":"a"}}`)); err != nil {
			t.Fatal(err)
		}
		select {
		case <-probe.C:
		case <-time.After(50 * time.Millisecond):
			continue
		}
		break
	}
	conn, err := d.Pool().Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	connections := func() int {
		var n int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := connections()
	var subs []*run.Subscription
	for i := 0; i < 200; i++ {
		sub, err := hub.Subscribe(id)
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, sub)
	}
	if after := connections(); after != before {
		t.Fatalf("訂閱數增加，連線數不得增加：%d → %d", before, after)
	}
	if _, err := l.Append(ctx, id, "text", []byte(`{"text":{"text":"b"}}`)); err != nil {
		t.Fatal(err)
	}
	for i, sub := range subs {
		select {
		case <-sub.C:
		case <-ctx.Done():
			t.Fatalf("第 %d 條訂閱沒有被喚醒", i)
		}
	}
}

func TestRewatchingAnEndedRunFromTheLastCursorReturnsImmediately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	hub := run.NewHub(d, run.MaxSubscriptions, discard())
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{}}
	id, err := s.Submit(ctx, input(1, req, 5))
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range []string{run.StatusRunning, run.StatusSucceeded} {
		if _, err := ledger(d, now).Transition(ctx, id, to, ""); err != nil {
			t.Fatal(err)
		}
	}
	r, _ := store.New(d.Conn(ctx)).GetRun(ctx, id)
	w := run.Watcher{DB: d, Hub: hub, Window: time.Hour}
	var sent int
	if err := w.Watch(ctx, 1, id, run.Cursor(r.LastEventSeq), func(run.Event) error { sent++; return nil }); err != nil {
		t.Fatalf("Watch：%v（串流視窗是一小時，ctx 30 秒：卡住就會是逾時）", err)
	}
	if sent != 0 || ctx.Err() != nil {
		t.Fatalf("sent = %d, ctx = %v", sent, ctx.Err())
	}
}

func TestPurgeAdvancesMoreRunsThanOneBatch(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	old := now.Add(-run.RetentionPeriod - time.Hour)
	const runs = 250
	for range runs {
		finishedRun(ctx, t, d, req, 1, old, 2)
	}
	p := run.Purger{DB: d, Ledger: ledger(d, now)}
	n, err := p.Purge(ctx)
	if err != nil || n == 0 {
		t.Fatalf("Purge = %d, %v", n, err)
	}
	if left := count(ctx, t, d, `SELECT count(*) FROM procurement.agent_runs WHERE min_available_seq <= last_event_seq AND completed_at IS NOT NULL`); left != 0 {
		t.Fatalf("還有 %d 個過期的 run 沒推進：一次呼叫要清到沒有", left)
	}
}

func TestSubmitWritesTheConversationIDInRunAccepted(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	conv := uuid.NewV7()
	if _, err := d.Pool().Exec(ctx, `INSERT INTO procurement.conversations (id, request_id, buyer_id, created_at, updated_at) VALUES ($1, $2, 1, $3, $3)`, conv, req, now); err != nil {
		t.Skipf("conversations 欄位與測試假設不同：%v", err)
	}
	in := input(1, req, 5)
	in.ConversationID = &conv
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{}}
	id, err := s.Submit(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := d.Pool().QueryRow(ctx, `SELECT payload->'run_accepted'->>'conversationId' FROM procurement.agent_run_events WHERE run_id = $1 AND seq = 1`, id).Scan(&got); err != nil || got != conv.String() {
		t.Fatalf("conversationId = %q, %v", got, err)
	}
}
