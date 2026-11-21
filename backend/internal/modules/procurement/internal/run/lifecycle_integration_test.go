//go:build integration

package run_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river/rivertype"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

func seedRun(ctx context.Context, t *testing.T, d *db.DB, req uuid.UUID, buyer int64, status string, jobID *int64) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	if _, err := d.Pool().Exec(ctx, `INSERT INTO procurement.agent_runs (id, buyer_id, request_id, run_type, status, client_request_id, job_id, quota_period_start, created_at, updated_at)
		VALUES ($1, $2, $3, 'TURN', $4, $5, $6, $7, $7, $7)`, id, buyer, req, status, uuid.NewV7(), jobID, now); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedUsage(ctx context.Context, t *testing.T, d *db.DB, buyer int64, usedCount int) {
	t.Helper()
	if _, err := d.Pool().Exec(ctx, `INSERT INTO procurement.usage (buyer_id, period_start, used, updated_at) VALUES ($1, $2, $3, $2)
		ON CONFLICT (buyer_id, period_start) DO UPDATE SET used = EXCLUDED.used`, buyer, now, usedCount); err != nil {
		t.Fatal(err)
	}
}

func runOf(ctx context.Context, t *testing.T, d *db.DB, id uuid.UUID) store.ProcurementAgentRun {
	t.Helper()
	r, err := store.New(d.Conn(ctx)).GetRun(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func eventCount(ctx context.Context, t *testing.T, d *db.DB, id uuid.UUID) int {
	t.Helper()
	return count(ctx, t, d, `SELECT count(*) FROM procurement.agent_run_events WHERE run_id = $1`, id)
}

func i64(v int64) *int64 { return &v }

var allStatuses = []string{
	run.StatusQueued, run.StatusRunning, run.StatusWaitingForInput, run.StatusCancelRequested,
	run.StatusSucceeded, run.StatusFailed, run.StatusCancelled,
}

func reasonFor(to string) string {
	switch to {
	case run.StatusFailed:
		return run.FailureWorkerLost
	}
	return ""
}

func TestTransitionAppliesOnlyTheEdgesOfTheTable(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	l := ledger(d, now)
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			if to == run.StatusQueued {
				continue
			}
			t.Run(fmt.Sprintf("%s_to_%s", from, to), func(t *testing.T) {
				id := seedRun(ctx, t, d, req, 1, from, nil)
				r, err := l.Transition(ctx, id, to, reasonFor(to))
				var inv *run.InvalidTransitionError
				viaOtherEntry := to == run.StatusWaitingForInput || from == run.StatusWaitingForInput && to == run.StatusSucceeded
				switch {
				case to == run.StatusWaitingForInput:
					if err == nil {
						t.Fatal("公開的 Transition 不得轉入 WAITING_FOR_INPUT（只能經 Interrupt）")
					}
				case from == run.StatusWaitingForInput && to == run.StatusSucceeded:
					if !errors.As(err, &inv) || inv.From != from || inv.To != to {
						t.Fatalf("WAITING_FOR_INPUT 轉 SUCCEEDED 只在回答的提交點，公開入口要 InvalidTransitionError：%v", err)
					}
				case run.CanTransition(from, to):
					if err != nil || r.Status != to {
						t.Fatalf("允許的邊要成功：%v %s", err, r.Status)
					}
					if run.IsTerminal(to) != (eventCount(ctx, t, d, id) == 1 && lastEventType(ctx, t, d, id) == run.EventFinished) {
						t.Fatalf("終態要恰好寫一筆 finished，非終態不寫")
					}
					if run.IsTerminal(to) != (runOf(ctx, t, d, id).CompletedAt != nil) {
						t.Fatal("終態要有 completed_at")
					}
				case run.IsTerminal(from):
					if !errors.Is(err, run.ErrRunTerminal) {
						t.Fatalf("終態是吸收的，要 ErrRunTerminal：%v", err)
					}
				default:
					if !errors.As(err, &inv) || inv.From != from || inv.To != to {
						t.Fatalf("沒有這條邊要 InvalidTransitionError：%v", err)
					}
				}
				if !run.CanTransition(from, to) || viaOtherEntry {
					if got := runOf(ctx, t, d, id); got.Status != from || eventCount(ctx, t, d, id) != 0 {
						t.Fatalf("被拒絕的轉換不得改狀態或寫事件：%s", got.Status)
					}
				}
			})
		}
	}
	if _, err := l.Transition(ctx, uuid.NewV7(), run.StatusRunning, ""); err == nil || apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的 run 要 NOT_FOUND：%v", err)
	}
}

func TestAppendAfterTerminalIsRejectedWithATypedError(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	l := ledger(d, now)
	for _, status := range allStatuses {
		id := seedRun(ctx, t, d, req, 1, status, nil)
		_, err := l.Append(ctx, id, "text", []byte(`{"text":{"text":"x"}}`))
		if run.IsTerminal(status) {
			if !errors.Is(err, run.ErrRunTerminal) || eventCount(ctx, t, d, id) != 0 || runOf(ctx, t, d, id).LastEventSeq != 0 {
				t.Fatalf("%s：終態之後不得寫事件或配發序號：%v", status, err)
			}
		} else if err != nil || eventCount(ctx, t, d, id) != 1 {
			t.Fatalf("%s：非終態要能寫事件：%v", status, err)
		}
	}
	if _, err := l.Append(ctx, uuid.NewV7(), "text", []byte(`{}`)); err == nil || apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的 run 要 NOT_FOUND：%v", err)
	}
}

func TestSubmitStoresTheJobAndThePeriodOnTheRun(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: jobs}
	id, err := s.Submit(ctx, input(1, seedRequest(ctx, t, d, 1), 5))
	if err != nil {
		t.Fatal(err)
	}
	r := runOf(ctx, t, d, id)
	if r.JobID == nil || *r.JobID != 1 || !r.QuotaPeriodStart.Equal(now) || r.Status != run.StatusQueued {
		t.Fatalf("run = %+v", r)
	}
}

type fakeStates struct {
	mu     sync.Mutex
	states map[int64]rivertype.JobState
	fail   map[int64]error
	reads  []int64
}

func (f *fakeStates) JobState(_ context.Context, id int64) (rivertype.JobState, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, id)
	if err := f.fail[id]; err != nil {
		return "", false, err
	}
	st, ok := f.states[id]
	return st, ok, nil
}

func reconciler(d *db.DB) run.Reconciler {
	return run.Reconciler{DB: d, Ledger: ledger(d, now), Logger: discard()}
}

func TestReconcilerSettlesEachRunJobStateCombination(t *testing.T) {
	type want struct {
		status, reason string
		refunded       bool
	}
	cases := []struct {
		name   string
		status string
		job    rivertype.JobState
		absent bool
		want   want
	}{
		{"queued_discarded", run.StatusQueued, rivertype.JobStateDiscarded, false, want{run.StatusFailed, run.FailureWorkerLost, true}},
		{"running_discarded", run.StatusRunning, rivertype.JobStateDiscarded, false, want{run.StatusFailed, run.FailureWorkerLost, true}},
		{"running_not_found", run.StatusRunning, "", true, want{run.StatusFailed, run.FailureWorkerLost, true}},
		{"queued_not_found", run.StatusQueued, "", true, want{run.StatusFailed, run.FailureWorkerLost, true}},
		{"running_completed", run.StatusRunning, rivertype.JobStateCompleted, false, want{run.StatusFailed, run.FailureTurnPipelineErr, true}},
		{"queued_completed", run.StatusQueued, rivertype.JobStateCompleted, false, want{run.StatusFailed, run.FailureTurnPipelineErr, true}},
		{"cancel_completed", run.StatusCancelRequested, rivertype.JobStateCompleted, false, want{run.StatusCancelled, "", false}},
		{"cancel_cancelled", run.StatusCancelRequested, rivertype.JobStateCancelled, false, want{run.StatusCancelled, "", false}},
		{"cancel_discarded", run.StatusCancelRequested, rivertype.JobStateDiscarded, false, want{run.StatusCancelled, "", false}},
		{"cancel_not_found", run.StatusCancelRequested, "", true, want{run.StatusCancelled, "", false}},
		{"running_cancelled", run.StatusRunning, rivertype.JobStateCancelled, false, want{run.StatusFailed, run.FailureWorkerLost, true}},
		{"queued_available", run.StatusQueued, rivertype.JobStateAvailable, false, want{run.StatusQueued, "", false}},
		{"running_running", run.StatusRunning, rivertype.JobStateRunning, false, want{run.StatusRunning, "", false}},
		{"running_retryable", run.StatusRunning, rivertype.JobStateRetryable, false, want{run.StatusRunning, "", false}},
		{"cancel_running", run.StatusCancelRequested, rivertype.JobStateRunning, false, want{run.StatusCancelRequested, "", false}},
	}
	ctx := context.Background()
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := newDB(t)
			buyer := int64(i + 1)
			req := seedRequest(ctx, t, d, buyer)
			seedUsage(ctx, t, d, buyer, 1)
			jobID := int64(100 + i)
			id := seedRun(ctx, t, d, req, buyer, c.status, i64(jobID))
			states := &fakeStates{states: map[int64]rivertype.JobState{}}
			if !c.absent {
				states.states[jobID] = c.job
			}
			if _, err := reconciler(d).Reconcile(ctx, states); err != nil {
				t.Fatal(err)
			}
			r := runOf(ctx, t, d, id)
			if r.Status != c.want.status {
				t.Fatalf("status = %s，要 %s", r.Status, c.want.status)
			}
			if c.want.reason != "" && (r.FailureReason == nil || *r.FailureReason != c.want.reason) {
				t.Fatalf("failure_reason = %v，要 %s", r.FailureReason, c.want.reason)
			}
			terminal := run.IsTerminal(c.want.status)
			if terminal != (eventCount(ctx, t, d, id) == 1 && lastEventType(ctx, t, d, id) == run.EventFinished) {
				t.Fatal("終態要在同一個交易寫 finished 事件，非終態不寫")
			}
			if refunded := used(ctx, t, d, buyer) == 0; refunded != c.want.refunded {
				t.Fatalf("refunded = %v，要 %v", refunded, c.want.refunded)
			}
			if _, err := reconciler(d).Reconcile(ctx, states); err != nil || runOf(ctx, t, d, id).Status != c.want.status {
				t.Fatalf("再對帳一次要冪等：%v", err)
			}
		})
	}
}

func TestReconcilerIgnoresWaitingForInputAndTerminalRunsWithoutReadingJobs(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	waiting := seedRun(ctx, t, d, req, 1, run.StatusWaitingForInput, i64(7))
	var terminals []uuid.UUID
	for _, s := range []string{run.StatusSucceeded, run.StatusFailed, run.StatusCancelled} {
		terminals = append(terminals, seedRun(ctx, t, d, req, 1, s, i64(8)))
	}
	states := &fakeStates{states: map[int64]rivertype.JobState{7: rivertype.JobStateDiscarded, 8: rivertype.JobStateDiscarded}}
	if _, err := reconciler(d).Reconcile(ctx, states); err != nil {
		t.Fatal(err)
	}
	if len(states.reads) != 0 {
		t.Fatalf("等待輸入與終態的 run 不查 job：%v", states.reads)
	}
	if runOf(ctx, t, d, waiting).Status != run.StatusWaitingForInput || runOf(ctx, t, d, terminals[0]).Status != run.StatusSucceeded {
		t.Fatal("等待輸入與終態的 run 不得被動")
	}
}

func TestReconcilerKeepsGoingWhenOneRowFails(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	seedUsage(ctx, t, d, 1, 3)
	a := seedRun(ctx, t, d, req, 1, run.StatusRunning, i64(1))
	bad := seedRun(ctx, t, d, req, 1, run.StatusRunning, i64(2))
	c := seedRun(ctx, t, d, req, 1, run.StatusQueued, i64(3))
	states := &fakeStates{
		states: map[int64]rivertype.JobState{1: rivertype.JobStateDiscarded, 3: rivertype.JobStateDiscarded},
		fail:   map[int64]error{2: errors.New("river 讀取失敗")},
	}
	res, err := reconciler(d).Reconcile(ctx, states)
	if err != nil || res.Errors != 1 || res.Failed != 2 {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if runOf(ctx, t, d, a).Status != run.StatusFailed || runOf(ctx, t, d, c).Status != run.StatusFailed {
		t.Fatal("失敗那一列前後的 run 都要被收斂")
	}
	if r := runOf(ctx, t, d, bad); r.Status != run.StatusRunning || eventCount(ctx, t, d, bad) != 0 {
		t.Fatal("失敗的那一列要整列回滾")
	}
	if used(ctx, t, d, 1) != 1 {
		t.Fatalf("只退成功收斂的兩列：used = %d", used(ctx, t, d, 1))
	}
}

func TestReconcilerSkipsARowLockedByAnotherWriter(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	id := seedRun(ctx, t, d, req, 1, run.StatusRunning, i64(1))
	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- d.InTx(ctx, func(ctx context.Context) error {
			if _, err := store.New(d.Conn(ctx)).LockRun(ctx, id); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	states := &fakeStates{states: map[int64]rivertype.JobState{1: rivertype.JobStateDiscarded}}
	_, err := reconciler(d).Reconcile(ctx, states)
	close(release)
	if e := <-done; e != nil || err != nil {
		t.Fatalf("%v %v", e, err)
	}
	if runOf(ctx, t, d, id).Status != run.StatusRunning || len(states.reads) != 0 {
		t.Fatal("被別的寫入者鎖住的列要略過、下一輪再看")
	}
}

func submitOne(ctx context.Context, t *testing.T, d *db.DB, jobs run.JobQueue, buyer int64) uuid.UUID {
	t.Helper()
	id, err := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: jobs}.Submit(ctx, input(buyer, seedRequest(ctx, t, d, buyer), 5))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCancelRequestsCancellationAndCancelsTheJobInOneTransaction(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	id := submitOne(ctx, t, d, jobs, 1)
	c := run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: jobs}
	out, err := c.Cancel(ctx, 1, id)
	if err != nil || out.Disposition != run.DispositionRequested || out.Status != run.StatusCancelRequested {
		t.Fatalf("out = %+v, %v", out, err)
	}
	if len(jobs.cancels) != 1 || jobs.cancels[0] != *runOf(ctx, t, d, id).JobID {
		t.Fatalf("要取消這個 run 的 job：%v", jobs.cancels)
	}
	if used(ctx, t, d, 1) != 1 {
		t.Fatal("買家取消不退額度")
	}
	if out, err := c.Cancel(ctx, 1, id); err != nil || out.Disposition != run.DispositionRequested || len(jobs.cancels) != 1 {
		t.Fatalf("重複取消要冪等且不再取消 job：%+v %v %v", out, err, jobs.cancels)
	}
	if _, err := c.Cancel(ctx, 2, id); err == nil || apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("別人的 run 要 NOT_FOUND：%v", err)
	}
}

type failingCancelJobs struct{ recordingJobs }

func (*failingCancelJobs) CancelJob(context.Context, int64) (rivertype.JobState, error) {
	return "", errors.New("river 取消失敗")
}

func TestCancelRollsBackTheTransitionWhenTheJobCannotBeCancelled(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	id := submitOne(ctx, t, d, &recordingJobs{}, 1)
	c := run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: &failingCancelJobs{}}
	if _, err := c.Cancel(ctx, 1, id); err == nil {
		t.Fatal("取消 job 失敗要回錯誤")
	}
	if runOf(ctx, t, d, id).Status != run.StatusQueued {
		t.Fatal("狀態轉換要和取消 job 一起回滾")
	}
}

func TestCancelOnRunsThatAreNotInFlight(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	jobs := &recordingJobs{}
	c := run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: jobs}
	cases := map[string]string{
		run.StatusCancelled: run.DispositionAlreadyCancelled,
		run.StatusSucceeded: run.DispositionAlreadyTerminal,
		run.StatusFailed:    run.DispositionAlreadyTerminal,
	}
	for status, want := range cases {
		id := seedRun(ctx, t, d, req, 1, status, i64(1))
		if out, err := c.Cancel(ctx, 1, id); err != nil || out.Disposition != want || out.Status != status {
			t.Fatalf("%s：%+v %v", status, out, err)
		}
	}
	if len(jobs.cancels) != 0 {
		t.Fatalf("這些都不該取消 job：%v", jobs.cancels)
	}
}

type cancellingExecutor struct {
	c  run.Canceller
	id uuid.UUID
}

func (e cancellingExecutor) Execute(ctx context.Context, _ store.ProcurementAgentRun) error {
	if _, err := e.c.Cancel(ctx, 1, e.id); err != nil {
		return err
	}
	return context.Canceled
}

func TestExecutorStoppedByCancellationEndsCancelledWithoutRefund(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	id := submitOne(ctx, t, d, jobs, 1)
	l := ledger(d, now)
	disp := run.Dispatcher{DB: d, Ledger: l, Executors: turnOnly(cancellingExecutor{c: run.Canceller{DB: d, Ledger: l, Jobs: jobs}, id: id})}
	if err := disp.Run(ctx, run.RunArgs{RunID: id}); err != nil {
		t.Fatal(err)
	}
	r := runOf(ctx, t, d, id)
	if r.Status != run.StatusCancelled || r.FailureReason != nil || lastEventType(ctx, t, d, id) != run.EventFinished {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	if used(ctx, t, d, 1) != 1 {
		t.Fatal("取消不退額度")
	}
}

type funcExecutor func(context.Context, store.ProcurementAgentRun) error

func (f funcExecutor) Execute(ctx context.Context, r store.ProcurementAgentRun) error {
	return f(ctx, r)
}

func TestExecutorThatReturnsWithoutSettlingIsFailedByTheReconcilerWithRefund(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	id := submitOne(ctx, t, d, jobs, 1)
	disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: turnOnly(funcExecutor(func(context.Context, store.ProcurementAgentRun) error { return nil }))}
	if err := disp.Run(ctx, run.RunArgs{RunID: id}); err != nil {
		t.Fatal(err)
	}
	if runOf(ctx, t, d, id).Status != run.StatusRunning {
		t.Fatal("Dispatcher 不替違約的 executor 收尾，交給 reconciler")
	}
	jobID := *runOf(ctx, t, d, id).JobID
	states := &fakeStates{states: map[int64]rivertype.JobState{jobID: rivertype.JobStateCompleted}}
	if _, err := reconciler(d).Reconcile(ctx, states); err != nil {
		t.Fatal(err)
	}
	r := runOf(ctx, t, d, id)
	if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != run.FailureTurnPipelineErr || used(ctx, t, d, 1) != 0 {
		t.Fatalf("run = %s %v used=%d", r.Status, r.FailureReason, used(ctx, t, d, 1))
	}
}

func TestExecutorStoppingOnATerminalRunChangesNothing(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	id := submitOne(ctx, t, d, &recordingJobs{}, 1)
	l := ledger(d, now)
	disp := run.Dispatcher{DB: d, Ledger: l, Executors: turnOnly(funcExecutor(func(ctx context.Context, r store.ProcurementAgentRun) error {
		if _, err := l.Transition(ctx, r.ID, run.StatusFailed, run.FailureWorkerLost); err != nil {
			return err
		}
		_, err := l.Append(ctx, r.ID, "text", []byte(`{"text":{"text":"late"}}`))
		return err
	}))}
	if err := disp.Run(ctx, run.RunArgs{RunID: id}); err != nil {
		t.Fatalf("executor 收到 ErrRunTerminal 就是正常結束：%v", err)
	}
	if r := runOf(ctx, t, d, id); r.Status != run.StatusFailed || eventCount(ctx, t, d, id) != 2 {
		t.Fatalf("status = %s, events = %d（run_accepted 與 finished）", r.Status, eventCount(ctx, t, d, id))
	}
	if used(ctx, t, d, 1) != 1 {
		t.Fatal("Dispatcher 不對已收斂的 run 退款")
	}
}

func TestDispatcherDoesNotRunACancelRequestedRun(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	id := submitOne(ctx, t, d, jobs, 1)
	if _, err := (run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: jobs}).Cancel(ctx, 1, id); err != nil {
		t.Fatal(err)
	}
	ran := false
	disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: turnOnly(funcExecutor(func(context.Context, store.ProcurementAgentRun) error { ran = true; return nil }))}
	if err := disp.Run(ctx, run.RunArgs{RunID: id}); err != nil || ran {
		t.Fatalf("已要求取消的 run 不得被領走：%v ran=%v", err, ran)
	}
}

func TestCancelOfAWaitingRunGoesStraightToCancelledWithoutAJobOrRefund(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	seedUsage(ctx, t, d, 1, 1)
	jobs := &recordingJobs{}
	waiting := seedRun(ctx, t, d, req, 1, run.StatusWaitingForInput, nil)
	c := run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: jobs}
	out, err := c.Cancel(ctx, 1, waiting)
	if err != nil || out.Status != run.StatusCancelled || out.Disposition != run.DispositionRequested {
		t.Fatalf("out = %+v, %v", out, err)
	}
	if r := runOf(ctx, t, d, waiting); r.Status != run.StatusCancelled || r.FailureReason != nil || lastEventType(ctx, t, d, waiting) != run.EventFinished {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	if len(jobs.cancels) != 0 || used(ctx, t, d, 1) != 1 {
		t.Fatalf("等待輸入沒有 job、取消不退額度：cancels=%v used=%d", jobs.cancels, used(ctx, t, d, 1))
	}
	if out, err := c.Cancel(ctx, 1, waiting); err != nil || out.Disposition != run.DispositionAlreadyCancelled {
		t.Fatalf("重複取消要冪等：%+v %v", out, err)
	}
}

func TestCancelSettlesInTheSameTransactionWhenTheJobIsAlreadyCancelled(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{cancelState: rivertype.JobStateCancelled}
	id := submitOne(ctx, t, d, jobs, 1)
	out, err := run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: jobs}.Cancel(ctx, 1, id)
	if err != nil || out.Status != run.StatusCancelled || out.Disposition != run.DispositionRequested {
		t.Fatalf("out = %+v, %v", out, err)
	}
	if r := runOf(ctx, t, d, id); r.Status != run.StatusCancelled || lastEventType(ctx, t, d, id) != run.EventFinished || used(ctx, t, d, 1) != 1 {
		t.Fatalf("job 已取消不必等 reconciler，也不退額度：%s", r.Status)
	}
}

func seedConversation(ctx context.Context, t *testing.T, d *db.DB, buyer int64, req uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	if err := store.New(d.Conn(ctx)).InsertConversation(ctx, store.InsertConversationParams{ID: id, BuyerID: buyer, RequestID: req, Now: now}); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedConversationRun(ctx context.Context, t *testing.T, d *db.DB, req uuid.UUID, buyer int64, conv uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := seedRun(ctx, t, d, req, buyer, status, nil)
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET conversation_id = $2 WHERE id = $1`, id, conv); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSubmitCancelsTheWaitingRunOfTheSameConversationOnly(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	conv := seedConversation(ctx, t, d, 1, req)
	other := seedConversation(ctx, t, d, 1, seedRequest(ctx, t, d, 1))
	waiting := seedConversationRun(ctx, t, d, req, 1, conv, run.StatusWaitingForInput)
	elsewhere := seedConversationRun(ctx, t, d, req, 1, other, run.StatusWaitingForInput)
	running := seedConversationRun(ctx, t, d, seedRequest(ctx, t, d, 1), 1, conv, run.StatusRunning)
	in := input(1, req, 5)
	in.ConversationID = &conv
	s := run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{}}
	id, err := s.Submit(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if r := runOf(ctx, t, d, waiting); r.Status != run.StatusCancelled || r.FailureReason != nil || lastEventType(ctx, t, d, waiting) != run.EventFinished {
		t.Fatalf("同對話等待輸入的 run 要自動取消：%s", r.Status)
	}
	if runOf(ctx, t, d, elsewhere).Status != run.StatusWaitingForInput || runOf(ctx, t, d, running).Status != run.StatusRunning || runOf(ctx, t, d, id).Status != run.StatusQueued {
		t.Fatal("其他對話與進行中的 run 不得被動")
	}
	if used(ctx, t, d, 1) != 1 {
		t.Fatal("自動取消不退額度，新的一輪扣一次")
	}
	if _, err := s.Submit(ctx, in); err != nil || runOf(ctx, t, d, id).Status != run.StatusQueued {
		t.Fatalf("冪等重送不得再取消任何 run：%v", err)
	}
}

func TestSubmitRollsBackTheAutoCancelWhenTheJobCannotBeEnqueued(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	conv := seedConversation(ctx, t, d, 1, req)
	waiting := seedConversationRun(ctx, t, d, req, 1, conv, run.StatusWaitingForInput)
	in := input(1, req, 5)
	in.ConversationID = &conv
	if _, err := (run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{failWith: errors.New("排入失敗")}}).Submit(ctx, in); err == nil {
		t.Fatal("排入失敗要回錯誤")
	}
	if runOf(ctx, t, d, waiting).Status != run.StatusWaitingForInput {
		t.Fatal("自動取消要和送出一起回滾")
	}
}

func TestShutdownOrTimeoutOfTheJobContextFailsTheRunAsWorkerLostWithRefund(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	id := submitOne(ctx, t, d, &recordingJobs{}, 1)
	jobCtx, stop := context.WithCancel(ctx)
	disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: turnOnly(funcExecutor(func(c context.Context, _ store.ProcurementAgentRun) error {
		stop()
		return c.Err()
	}))}
	if err := disp.Run(jobCtx, run.RunArgs{RunID: id}); err != nil {
		t.Fatal(err)
	}
	r := runOf(ctx, t, d, id)
	if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != run.FailureWorkerLost || used(ctx, t, d, 1) != 0 {
		t.Fatalf("run = %s %v used=%d", r.Status, r.FailureReason, used(ctx, t, d, 1))
	}
}

func TestDispatcherSkipsARunDeletedBeforeOrDuringExecution(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: turnOnly(funcExecutor(func(context.Context, store.ProcurementAgentRun) error { return errors.New("不該執行") }))}
	if err := disp.Run(ctx, run.RunArgs{RunID: uuid.NewV7()}); err != nil {
		t.Fatalf("run 已被刪除（BuyerDeleted）job 不該報錯：%v", err)
	}
	id := submitOne(ctx, t, d, &recordingJobs{}, 1)
	l := ledger(d, now)
	disp.Executors = turnOnly(funcExecutor(func(ctx context.Context, r store.ProcurementAgentRun) error {
		if _, err := d.Pool().Exec(ctx, `DELETE FROM procurement.agent_runs WHERE id = $1`, r.ID); err != nil {
			return err
		}
		_, err := l.Append(ctx, r.ID, "text", []byte(`{}`))
		return err
	}))
	if err := disp.Run(ctx, run.RunArgs{RunID: id}); err != nil {
		t.Fatalf("執行中被刪除也不該讓 job 報錯：%v", err)
	}
}

func TestPublicTransitionCannotLeaveWaitingForInputToSucceededWithoutResolvingTheInterrupt(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	parent, irq, _ := waitingParent(ctx, t, d, s, 1)
	var inv *run.InvalidTransitionError
	if _, err := ledger(d, now).Transition(ctx, parent, run.StatusSucceeded, ""); !errors.As(err, &inv) {
		t.Fatalf("err = %v，要 InvalidTransitionError", err)
	}
	if runOf(ctx, t, d, parent).Status != run.StatusWaitingForInput || interruptStatus(ctx, t, d, irq) != "PENDING" {
		t.Fatal("被拒絕之後提問的 run 與中斷維持原狀")
	}
}

func TestInterruptOnARunThatIsNotRunningReturnsAnInvalidTransition(t *testing.T) {
	for name, event := range map[string][]byte{"沒有事件": nil, "帶 turn_result": []byte(`{"turn_result":{}}`)} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			d := newDB(t)
			req := seedRequest(ctx, t, d, 1)
			id := seedRun(ctx, t, d, req, 1, run.StatusCancelRequested, nil)
			_, err := ledger(d, now).Interrupt(ctx, run.InterruptInput{RunID: id, Payload: []byte(`{}`), TurnResultEvent: event})
			var inv *run.InvalidTransitionError
			if !errors.As(err, &inv) || inv.From != run.StatusCancelRequested || inv.To != run.StatusWaitingForInput {
				t.Fatalf("err = %v，要 InvalidTransitionError", err)
			}
			if eventCount(ctx, t, d, id) != 0 || pendingInterrupts(ctx, t, d, req) != 0 {
				t.Fatal("被拒絕的寫入不得留下事件或中斷")
			}
		})
	}
}

func TestRunsTableTiesTheQuotaPeriodToTurnRuns(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	req := seedRequest(ctx, t, d, 1)
	for name, tc := range map[string]struct {
		runType string
		period  *time.Time
		wantErr bool
	}{
		"TURN 有額度期間":  {"TURN", &now, false},
		"TURN 沒有額度期間": {"TURN", nil, true},
		"候選搜尋沒有額度期間":  {"CANDIDATE_SEARCH", nil, false},
		"候選搜尋帶了額度期間":  {"CANDIDATE_SEARCH", &now, true},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := d.Pool().Exec(ctx, `INSERT INTO procurement.agent_runs (id, buyer_id, request_id, run_type, status, client_request_id, quota_period_start, created_at, updated_at)
				VALUES ($1, 1, $2, $3, 'QUEUED', $4, $5, $6, $6)`, uuid.NewV7(), req, tc.runType, uuid.NewV7(), tc.period, now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v，wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestBuyerSubmitLockUsesTheNamespacedKeyOfThatBuyer(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- d.InTx(ctx, func(ctx context.Context) error {
			if err := store.New(d.Conn(ctx)).LockBuyerSubmits(ctx, 7); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	canLock := func(key string) bool {
		tx, err := d.Pool().Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		var got bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))`, key).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if canLock("procurement.run_submit:7") {
		t.Error("買家 7 的送出鎖要佔住 procurement.run_submit:7 這個鍵")
	}
	if !canLock("procurement.run_submit:8") {
		t.Error("別的買家的鍵不受影響")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
