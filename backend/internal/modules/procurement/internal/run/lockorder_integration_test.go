//go:build integration

package run_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func benignCommitLoss(err error) bool {
	var invalid *run.InvalidTransitionError
	return err == nil || errors.Is(err, run.ErrInterruptNotPending) || errors.Is(err, run.ErrRunTerminal) || errors.As(err, &invalid)
}

type racePair struct {
	parent, irq, answer uuid.UUID
	ready               chan struct{}
}

func TestAnswerCommitRacingWithEveryLockingPathNeitherDeadlocksNorSplitsState(t *testing.T) {
	const pairs = 8
	for _, variant := range []string{"cancel_answer", "cancel_asking", "settle_failure", "reconcile"} {
		t.Run(variant, func(t *testing.T) {
			ctx := context.Background()
			d := newDB(t)
			jobs := &recordingJobs{}
			s := submitter(d, jobs, wide())
			l := ledger(d, now)
			gate := make(chan struct{})
			ps := make([]*racePair, pairs)
			byAnswer := map[uuid.UUID]*racePair{}
			for i := range ps {
				parent, irq, _ := waitingParent(ctx, t, d, s, 1)
				answer, err := s.Resume(ctx, resumeInput(irq, "L"))
				if err != nil {
					t.Fatal(err)
				}
				ps[i] = &racePair{parent: parent, irq: irq, answer: answer, ready: make(chan struct{})}
				byAnswer[answer] = ps[i]
				if variant != "settle_failure" {
					start(ctx, t, d, answer)
					close(ps[i].ready)
				}
			}
			disp := run.Dispatcher{DB: d, Ledger: l, Logger: discard(), Executors: map[string]run.Executor{
				run.RunTypeResume: funcExecutor(func(_ context.Context, r store.ProcurementAgentRun) error {
					close(byAnswer[r.ID].ready)
					<-gate
					return errors.New("executor failed")
				}),
			}}
			canceller := run.Canceller{DB: d, Ledger: l, Jobs: jobs}
			var wg sync.WaitGroup
			errs := make(chan error, 4*pairs+4)
			spawn := func(fn func() error) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-gate
					errs <- fn()
				}()
			}
			for _, p := range ps {
				spawn(func() error {
					<-p.ready
					if _, err := l.Transition(ctx, p.answer, run.StatusSucceeded, ""); !benignCommitLoss(err) {
						return err
					}
					return nil
				})
				switch variant {
				case "cancel_answer":
					spawn(func() error { _, err := canceller.Cancel(ctx, 1, p.answer); return err })
				case "cancel_asking":
					spawn(func() error { _, err := canceller.Cancel(ctx, 1, p.parent); return err })
				case "settle_failure":
					wg.Add(1)
					go func() {
						defer wg.Done()
						errs <- disp.Run(ctx, run.RunArgs{RunID: p.answer})
					}()
				}
			}
			if variant == "reconcile" {
				spawn(func() error {
					res, err := reconciler(d).Reconcile(ctx, &fakeStates{})
					if err == nil && res.Errors != 0 {
						err = errors.New("reconciler 有失敗的列（死結或鎖錯誤）")
					}
					return err
				})
			}
			if variant == "settle_failure" {
				for _, p := range ps {
					<-p.ready
				}
			}
			close(gate)
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Errorf("競爭中出現錯誤（含死結）：%v", err)
				}
			}
			for _, p := range ps {
				parent, answer, irq := runOf(ctx, t, d, p.parent), runOf(ctx, t, d, p.answer), interruptStatus(ctx, t, d, p.irq)
				committed := parent.Status == run.StatusSucceeded
				if committed != (irq == "RESOLVED") || committed != (answer.Status == run.StatusSucceeded) {
					t.Errorf("狀態分裂：提問 %s、中斷 %s、回答 %s", parent.Status, irq, answer.Status)
				}
				if parent.Status == run.StatusWaitingForInput && irq != "PENDING" {
					t.Errorf("提問的 run 仍在等待，中斷卻是 %s", irq)
				}
				if parent.Status == run.StatusCancelled && irq != "CANCELLED" {
					t.Errorf("提問的 run 已取消，中斷卻是 %s", irq)
				}
			}
		})
	}
}

func TestDispatcherSkipsAnAnswerRunWhoseAskingRunIsGone(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	s := submitter(d, jobs, wide())
	boom := funcExecutor(func(context.Context, store.ProcurementAgentRun) error { return errors.New("不該執行") })
	disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: map[string]run.Executor{run.RunTypeResume: boom}}

	parent, irq, _ := waitingParent(ctx, t, d, s, 1)
	answer, err := s.Resume(ctx, resumeInput(irq, "L"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(ctx, `DELETE FROM procurement.agent_runs WHERE id = $1`, parent); err != nil {
		t.Fatal(err)
	}
	if err := disp.Run(ctx, run.RunArgs{RunID: answer}); err != nil {
		t.Fatalf("提問的 run 已被刪除，回答 run 的 job 不該報錯：%v", err)
	}

	parent, irq, _ = waitingParent(ctx, t, d, s, 1)
	answer, err = s.Resume(ctx, resumeInput(irq, "L"))
	if err != nil {
		t.Fatal(err)
	}
	l := ledger(d, now)
	disp.Executors = map[string]run.Executor{run.RunTypeResume: funcExecutor(func(ctx context.Context, r store.ProcurementAgentRun) error {
		if _, err := d.Pool().Exec(ctx, `DELETE FROM procurement.agent_runs WHERE id = $1`, parent); err != nil {
			return err
		}
		_, err := l.Transition(ctx, r.ID, run.StatusSucceeded, "")
		return err
	})}
	if err := disp.Run(ctx, run.RunArgs{RunID: answer}); err != nil {
		t.Fatalf("執行中提問的 run 被刪除也不該讓 job 報錯：%v", err)
	}
}

func TestLockingAnAnswerRunWhoseAskingRunIsGoneIsNotFound(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	parent, irq, _ := waitingParent(ctx, t, d, s, 1)
	answer, err := s.Resume(ctx, resumeInput(irq, "L"))
	if err != nil {
		t.Fatal(err)
	}
	start(ctx, t, d, answer)
	if _, err := d.Pool().Exec(ctx, `DELETE FROM procurement.agent_runs WHERE id = $1`, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger(d, now).Transition(ctx, answer, run.StatusSucceeded, ""); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("err = %v，要 NOT_FOUND", err)
	}
}

func TestOnlyTurnSubmitSupersedesWaitingRuns(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	parent, irq, req := waitingParent(ctx, t, d, s, 1)
	if _, err := s.Submit(ctx, searchInput(1, req)); err != nil {
		t.Fatal(err)
	}
	if runOf(ctx, t, d, parent).Status != run.StatusWaitingForInput || interruptStatus(ctx, t, d, irq) != "PENDING" || pendingInterrupts(ctx, t, d, req) != 1 {
		t.Fatal("候選搜尋的送出不得取代等待輸入的 run 與它的 PENDING 中斷")
	}
}

func TestCancelWhenTheJobRowIsGoneConvergesLikeTheReconciler(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		wantStatus string
		wantReason string
		refund     bool
	}{
		{"queued", run.StatusQueued, run.StatusFailed, run.FailureWorkerLost, true},
		{"running", run.StatusRunning, run.StatusFailed, run.FailureWorkerLost, true},
		{"cancel_requested", run.StatusCancelRequested, run.StatusCancelled, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			d := newDB(t)
			jobs := &recordingJobs{missingJob: true}
			req := seedRequest(ctx, t, d, 1)
			seedUsage(ctx, t, d, 1, 1)
			id := seedRun(ctx, t, d, req, 1, tc.status, i64(1))
			out, err := run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: jobs}.Cancel(ctx, 1, id)
			if err != nil || out.Status != tc.wantStatus {
				t.Fatalf("out = %+v, %v，預期 %s", out, err, tc.wantStatus)
			}
			r := runOf(ctx, t, d, id)
			reason := ""
			if r.FailureReason != nil {
				reason = *r.FailureReason
			}
			if r.Status != tc.wantStatus || reason != tc.wantReason || lastEventType(ctx, t, d, id) != run.EventFinished {
				t.Fatalf("run = %s %q", r.Status, reason)
			}
			wantUsed := int32(1)
			if tc.refund {
				wantUsed = 0
			}
			if got := used(ctx, t, d, 1); got != wantUsed {
				t.Fatalf("used = %d，預期 %d", got, wantUsed)
			}
		})
	}
}
