//go:build integration

package run_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"uuid"

	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

func wide() run.Limits { return run.Limits{MaxActiveRuns: 100, MaxSubmitsPerMinute: 100} }

func submitter(d *db.DB, jobs run.JobQueue, l run.Limits) run.Submitter {
	return run.Submitter{DB: d, Ledger: ledger(d, now), Jobs: jobs, Limits: l}
}

func searchInput(buyer int64, request uuid.UUID) run.SubmitInput {
	in := input(buyer, request, 5)
	in.RunType = run.RunTypeCandidateSearch
	return in
}

func TestOnlyTurnRunsChargeTheQuota(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	search, err := s.Submit(ctx, searchInput(1, seedRequest(ctx, t, d, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if r := runOf(ctx, t, d, search); r.QuotaPeriodStart != nil || used(ctx, t, d, 1) != 0 {
		t.Fatalf("候選搜尋不扣額度：period=%v used=%d", r.QuotaPeriodStart, used(ctx, t, d, 1))
	}
	turn, err := s.Submit(ctx, input(1, seedRequest(ctx, t, d, 1), 5))
	if err != nil {
		t.Fatal(err)
	}
	if r := runOf(ctx, t, d, turn); r.QuotaPeriodStart == nil || used(ctx, t, d, 1) != 1 {
		t.Fatalf("TURN 扣一次：period=%v used=%d", r.QuotaPeriodStart, used(ctx, t, d, 1))
	}
	exhausted := searchInput(1, seedRequest(ctx, t, d, 1))
	exhausted.QuotaLimit = 0
	if _, err := s.Submit(ctx, exhausted); err != nil {
		t.Fatalf("額度為 0 時候選搜尋仍可送出：%v", err)
	}
}

func TestNonTurnFailureDoesNotRefundAndTurnFailureRefundsOnce(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	seedUsage(ctx, t, d, 1, 2)
	search, err := s.Submit(ctx, searchInput(1, seedRequest(ctx, t, d, 1)))
	if err != nil {
		t.Fatal(err)
	}
	disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: map[string]run.Executor{run.RunTypeTurn: failingExecutor{}, run.RunTypeCandidateSearch: failingExecutor{}}}
	if err := disp.Run(ctx, run.RunArgs{RunID: search}); err != nil {
		t.Fatal(err)
	}
	if r := runOf(ctx, t, d, search); r.Status != run.StatusFailed || *r.FailureReason != run.FailureSearchFailed || used(ctx, t, d, 1) != 2 {
		t.Fatalf("候選搜尋失敗不退額度：%s used=%d", r.Status, used(ctx, t, d, 1))
	}
	turn, err := s.Submit(ctx, input(1, seedRequest(ctx, t, d, 1), 5))
	if err != nil || used(ctx, t, d, 1) != 3 {
		t.Fatalf("TURN 扣額度：%v used=%d", err, used(ctx, t, d, 1))
	}
	if err := disp.Run(ctx, run.RunArgs{RunID: turn}); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler(d).Reconcile(ctx, &fakeStates{}); err != nil {
		t.Fatal(err)
	}
	if used(ctx, t, d, 1) != 2 {
		t.Fatalf("TURN 失敗退一次，used = %d", used(ctx, t, d, 1))
	}
}

func TestRunTypeWithoutAnExecutorIsSettledAsThatTypesPipelineFailure(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: map[string]run.Executor{}, Logger: discard()}
	turn, _ := s.Submit(ctx, input(1, seedRequest(ctx, t, d, 1), 5))
	search, _ := s.Submit(ctx, searchInput(1, seedRequest(ctx, t, d, 1)))
	for _, id := range []uuid.UUID{turn, search} {
		if err := disp.Run(ctx, run.RunArgs{RunID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if r := runOf(ctx, t, d, turn); r.Status != run.StatusFailed || *r.FailureReason != run.FailureTurnPipelineErr {
		t.Fatalf("TURN：%s %v", r.Status, r.FailureReason)
	}
	if r := runOf(ctx, t, d, search); r.Status != run.StatusFailed || *r.FailureReason != run.FailureSearchFailed {
		t.Fatalf("候選搜尋：%s %v", r.Status, r.FailureReason)
	}
	if used(ctx, t, d, 1) != 0 {
		t.Fatalf("TURN 的額度要退回，候選搜尋沒扣：used = %d", used(ctx, t, d, 1))
	}
}

func TestAtMostTwoRunsInProgressPerBuyerUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, run.Limits{MaxSubmitsPerMinute: 100})
	reqs := make([]uuid.UUID, 8)
	for i := range reqs {
		reqs[i] = seedRequest(ctx, t, d, 1)
	}
	var wg sync.WaitGroup
	var ok, full atomic.Int32
	ids := make([]run.SubmitInput, len(reqs))
	for i, r := range reqs {
		ids[i] = input(1, r, 100)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Submit(ctx, ids[i])
			switch {
			case err == nil:
				ok.Add(1)
			case apperr.KindOf(err) == apperr.KindResourceExhausted:
				full.Add(1)
			default:
				t.Errorf("意外的錯誤：%v", err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 2 || full.Load() != int32(len(reqs)-2) {
		t.Fatalf("進行中上限 2：成功 %d、被擋 %d", ok.Load(), full.Load())
	}
	if n := count(ctx, t, d, `SELECT count(*) FROM procurement.agent_runs WHERE buyer_id = 1`); n != 2 {
		t.Fatalf("被擋的送出要連 run 一起回滾：runs = %d", n)
	}
	if used(ctx, t, d, 1) != 2 {
		t.Fatalf("被擋的送出不扣額度：used = %d", used(ctx, t, d, 1))
	}
}

func TestWaitingRunsDoNotCountAsInProgressAndReplayIsNotBlockedByLimits(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	for range 3 {
		seedRun(ctx, t, d, seedRequest(ctx, t, d, 1), 1, run.StatusWaitingForInput, nil)
	}
	s := submitter(d, &recordingJobs{}, run.Limits{MaxSubmitsPerMinute: 100})
	in1, in2 := input(1, seedRequest(ctx, t, d, 1), 5), input(1, seedRequest(ctx, t, d, 1), 5)
	first, err := s.Submit(ctx, in1)
	if err != nil {
		t.Fatalf("等待輸入的 run 不佔進行中的名額：%v", err)
	}
	if _, err := s.Submit(ctx, in2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Submit(ctx, input(1, seedRequest(ctx, t, d, 1), 5)); apperr.KindOf(err) != apperr.KindResourceExhausted {
		t.Fatalf("第三個進行中要被擋：%v", err)
	}
	again, err := s.Submit(ctx, in1)
	if err != nil || again != first {
		t.Fatalf("冪等重送不受上限擋：%v %v", again, err)
	}
}

func TestOneRunInProgressPerRequestUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	req := seedRequest(ctx, t, d, 1)
	var wg sync.WaitGroup
	var ok, conflict atomic.Int32
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Submit(ctx, input(1, req, 100))
			reason, kind := reasonOf(err)
			switch {
			case err == nil:
				ok.Add(1)
			case kind == apperr.KindFailedPrecondition && reason == apperr.ReasonProcurementRunInProgress:
				conflict.Add(1)
			default:
				t.Errorf("意外的錯誤：%v", err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || conflict.Load() != 5 {
		t.Fatalf("同一個需求只有一個進行中：成功 %d、衝突 %d", ok.Load(), conflict.Load())
	}
}

func TestAtMostTenSubmitsPerBuyerPerMinute(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, run.Limits{MaxActiveRuns: 100})
	for i := range run.DefaultMaxSubmitsPerMinute {
		if _, err := s.Submit(ctx, input(1, seedRequest(ctx, t, d, 1), 100)); err != nil {
			t.Fatalf("第 %d 次：%v", i+1, err)
		}
	}
	_, err := s.Submit(ctx, input(1, seedRequest(ctx, t, d, 1), 100))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindResourceExhausted {
		t.Fatalf("第 11 次要 RESOURCE_EXHAUSTED：%v", err)
	}
	if _, err := s.Submit(ctx, input(2, seedRequest(ctx, t, d, 2), 100)); err != nil {
		t.Fatalf("別的買家不受影響：%v", err)
	}
	later := run.Submitter{DB: d, Ledger: ledger(d, now.Add(61*time.Second)), Jobs: &recordingJobs{}, Limits: run.Limits{MaxActiveRuns: 100}}
	if _, err := later.Submit(ctx, input(1, seedRequest(ctx, t, d, 1), 100)); err != nil {
		t.Fatalf("一分鐘後可再送出：%v", err)
	}
}

// waitingParent 建一個 TURN run 並讓它停在 WAITING_FOR_INPUT，回傳 run 與它的中斷 ID。
func waitingParent(ctx context.Context, t *testing.T, d *db.DB, s run.Submitter, buyer int64) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	req := seedRequest(ctx, t, d, buyer)
	conv := seedConversation(ctx, t, d, buyer, req)
	in := input(buyer, req, 100)
	in.ConversationID = &conv
	id, err := s.Submit(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	l := ledger(d, now)
	if _, err := l.Transition(ctx, id, run.StatusRunning, ""); err != nil {
		t.Fatal(err)
	}
	irq, err := l.Interrupt(ctx, run.InterruptInput{RunID: id, Payload: []byte(`{"question":"尺寸？"}`), TurnResultEvent: []byte(`{"turn_result":{}}`)})
	if err != nil {
		t.Fatal(err)
	}
	return id, irq, req
}

func TestInterruptWritesEventInterruptAndWaitingStateTogether(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	id, irq, req := waitingParent(ctx, t, d, s, 1)
	if r := runOf(ctx, t, d, id); r.Status != run.StatusWaitingForInput {
		t.Fatalf("status = %s", r.Status)
	}
	if lastEventType(ctx, t, d, id) != run.EventTurnResult {
		t.Fatal("要寫 turn_result 事件")
	}
	l := ledger(d, now)
	byRun, ok, err := l.OpenInterruptForRun(ctx, 1, id)
	if err != nil || !ok || byRun.ID != irq || byRun.Status != "PENDING" || byRun.Processing || string(byRun.Payload) != `{"question": "尺寸？"}` {
		t.Fatalf("OpenInterruptForRun = %+v %v %v", byRun, ok, err)
	}
	if byReq, ok, err := l.OpenInterruptForRequest(ctx, 1, req); err != nil || !ok || byReq.ID != irq {
		t.Fatalf("OpenInterruptForRequest = %+v %v %v", byReq, ok, err)
	}
	if _, ok, _ := l.OpenInterruptForRun(ctx, 2, id); ok {
		t.Fatal("別的買家看不到")
	}
	if _, err := l.Interrupt(ctx, run.InterruptInput{RunID: id, Payload: []byte(`{}`)}); err == nil {
		t.Fatal("不在 RUNNING 的 run 不能再寫中斷")
	}
	if n := count(ctx, t, d, `SELECT count(*) FROM procurement.agent_run_interrupts WHERE run_id = $1`, id); n != 1 {
		t.Fatalf("失敗的中斷寫入要回滾：%d", n)
	}
}

func interruptStatus(ctx context.Context, t *testing.T, d *db.DB, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := d.Pool().QueryRow(ctx, `SELECT status FROM procurement.agent_run_interrupts WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func pendingInterrupts(ctx context.Context, t *testing.T, d *db.DB, req uuid.UUID) int {
	t.Helper()
	return count(ctx, t, d, `SELECT count(*) FROM procurement.agent_run_interrupts WHERE request_id = $1 AND status = 'PENDING'`, req)
}

func resumeInput(irq uuid.UUID, message string) run.ResumeInput {
	return run.ResumeInput{BuyerID: 1, InterruptID: irq, ClientRequestID: uuid.NewV7(), Message: message}
}

func start(ctx context.Context, t *testing.T, d *db.DB, id uuid.UUID) {
	t.Helper()
	if _, err := ledger(d, now).Transition(ctx, id, run.StatusRunning, ""); err != nil {
		t.Fatal(err)
	}
}

func TestSupersededWaitingRunCancelsItsInterruptAndFreesThePendingSlot(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	id, irq, req := waitingParent(ctx, t, d, s, 1)
	var conv uuid.UUID
	if err := d.Pool().QueryRow(ctx, `SELECT conversation_id FROM procurement.agent_runs WHERE id = $1`, id).Scan(&conv); err != nil {
		t.Fatal(err)
	}
	next := input(1, req, 100)
	next.ConversationID = &conv
	nextID, err := s.Submit(ctx, next)
	if err != nil {
		t.Fatal(err)
	}
	if runOf(ctx, t, d, id).Status != run.StatusCancelled {
		t.Fatal("等待中的 run 要被取代")
	}
	l := ledger(d, now)
	if _, ok, _ := l.OpenInterruptForRun(ctx, 1, id); ok {
		t.Fatal("被取代的 run 沒有未回答的中斷")
	}
	if st := interruptStatus(ctx, t, d, irq); st != "CANCELLED" || pendingInterrupts(ctx, t, d, req) != 0 {
		t.Fatalf("被取代的 run 的中斷要轉 CANCELLED 並放掉 PENDING 的名額：%s", st)
	}
	start(ctx, t, d, nextID)
	newIrq, err := l.Interrupt(ctx, run.InterruptInput{RunID: nextID, Payload: []byte(`{"question":"顏色？"}`)})
	if err != nil || pendingInterrupts(ctx, t, d, req) != 1 || interruptStatus(ctx, t, d, newIrq) != "PENDING" {
		t.Fatalf("取代的那一輪可以提出新的問題：%v", err)
	}
	if _, err := s.Resume(ctx, resumeInput(irq, "L")); err == nil {
		t.Fatal("被取代的 run 的中斷不能再回答")
	} else if reason, _ := reasonOf(err); reason != apperr.ReasonRunNotActive {
		t.Fatalf("要 RUN_NOT_ACTIVE：%v", err)
	}
}

func TestBuyerCancellingTheAskingRunCancelsItsInterrupt(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	id, irq, req := waitingParent(ctx, t, d, s, 1)
	c := run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{}}
	if out, err := c.Cancel(ctx, 1, id); err != nil || out.Status != run.StatusCancelled {
		t.Fatalf("%+v %v", out, err)
	}
	if interruptStatus(ctx, t, d, irq) != "CANCELLED" || pendingInterrupts(ctx, t, d, req) != 0 {
		t.Fatal("取消提問的 run 要讓中斷轉 CANCELLED")
	}
}

func TestResumeCreatesAnOrdinaryAnswerRunAndChangesNothingElse(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	s := submitter(d, jobs, wide())
	parent, irq, req := waitingParent(ctx, t, d, s, 1)
	usedBefore, jobsBefore := used(ctx, t, d, 1), jobs.n.Load()
	in := resumeInput(irq, "大尺寸")
	answer, err := s.Resume(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	c, p := runOf(ctx, t, d, answer), runOf(ctx, t, d, parent)
	if c.RunType != run.RunTypeResume || c.Status != run.StatusQueued || c.InterruptID == nil || *c.InterruptID != irq || c.QuotaPeriodStart != nil || c.RequestID != req || c.JobID == nil {
		t.Fatalf("answer = %+v", c)
	}
	if p.Status != run.StatusWaitingForInput || p.CompletedAt != nil || interruptStatus(ctx, t, d, irq) != "PENDING" {
		t.Fatalf("送出時不解決中斷也不動提問的 run：%s、中斷 %s", p.Status, interruptStatus(ctx, t, d, irq))
	}
	if used(ctx, t, d, 1) != usedBefore || jobs.n.Load() != jobsBefore+1 {
		t.Fatal("回答 run 不扣額度、排一個 job")
	}
	if lastEventType(ctx, t, d, answer) != run.EventRunAccepted {
		t.Fatal("回答 run 的第一個事件是 run_accepted")
	}
	l := ledger(d, now)
	if got, ok, err := l.OpenInterruptForRun(ctx, 1, parent); err != nil || !ok || got.ID != irq || !got.Processing {
		t.Fatalf("有回答的 run 進行中要呈現處理中：%+v %v %v", got, ok, err)
	}
	if got, ok, err := l.OpenInterruptForRequest(ctx, 1, req); err != nil || !ok || !got.Processing {
		t.Fatalf("以需求讀同樣是處理中：%+v %v %v", got, ok, err)
	}
	if again, err := s.Resume(ctx, in); err != nil || again != answer || jobs.n.Load() != jobsBefore+1 {
		t.Fatalf("同一個 client_request_id 重送回同一個回答 run：%v %v", again, err)
	}
	if _, err := s.Resume(ctx, resumeInput(irq, "大尺寸")); err == nil {
		t.Fatal("回答進行中，新的 client_request_id 要被拒絕")
	} else if reason, kind := reasonOf(err); reason != apperr.ReasonProcurementRunInProgress || kind != apperr.KindFailedPrecondition {
		t.Fatalf("要 PROCUREMENT_RUN_IN_PROGRESS：%v", err)
	}
	bad := in
	bad.Message = "別的"
	if _, kind := reasonOf(errOf(s.Resume(ctx, bad))); kind != apperr.KindInvalidInput {
		t.Fatalf("同一個 client_request_id 內容不同要 INVALID_ARGUMENT：%v", kind)
	}
	if _, err := s.Resume(ctx, run.ResumeInput{BuyerID: 2, InterruptID: irq, ClientRequestID: uuid.NewV7(), Message: "x"}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("別的買家的中斷要 NOT_FOUND：%v", err)
	}
}

func TestAnswerRunSuccessResolvesTheInterruptAndSucceedsTheAskingRunInOneTransaction(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	parent, irq, req := waitingParent(ctx, t, d, s, 1)
	in := resumeInput(irq, "大尺寸")
	answer, err := s.Resume(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	start(ctx, t, d, answer)
	l := ledger(d, now)
	if _, err := l.Transition(ctx, answer, run.StatusSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	p := runOf(ctx, t, d, parent)
	if interruptStatus(ctx, t, d, irq) != "RESOLVED" || p.Status != run.StatusSucceeded || p.CompletedAt == nil || lastEventType(ctx, t, d, parent) != run.EventFinished {
		t.Fatalf("提問的 run %s、中斷 %s", p.Status, interruptStatus(ctx, t, d, irq))
	}
	if _, ok, _ := l.OpenInterruptForRequest(ctx, 1, req); ok || runOf(ctx, t, d, answer).Status != run.StatusSucceeded {
		t.Fatal("成功之後沒有未回答的中斷")
	}
	if again, err := s.Resume(ctx, in); err != nil || again != answer {
		t.Fatalf("成功之後同一個 client_request_id 仍回放：%v %v", again, err)
	}
	if _, err := s.Resume(ctx, resumeInput(irq, "大尺寸")); err == nil {
		t.Fatal("成功之後以同一個中斷再答要被拒絕")
	} else if reason, kind := reasonOf(err); reason != apperr.ReasonApprovalAlreadyResolved || kind != apperr.KindFailedPrecondition {
		t.Fatalf("要 APPROVAL_ALREADY_RESOLVED：%v", err)
	}
}

func TestAnswerRunCommitRollsBackWhenResolvingTheInterruptFails(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	parent, irq, _ := waitingParent(ctx, t, d, s, 1)
	answer, err := s.Resume(ctx, resumeInput(irq, "L"))
	if err != nil {
		t.Fatal(err)
	}
	start(ctx, t, d, answer)
	if _, err := d.Pool().Exec(ctx, `
		CREATE FUNCTION procurement.fail_resolve() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected'; END $$;
		CREATE TRIGGER fail_resolve BEFORE UPDATE ON procurement.agent_run_interrupts FOR EACH ROW EXECUTE FUNCTION procurement.fail_resolve();`); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger(d, now).Transition(ctx, answer, run.StatusSucceeded, ""); err == nil {
		t.Fatal("注入的失敗要讓提交失敗")
	}
	if a, p := runOf(ctx, t, d, answer), runOf(ctx, t, d, parent); a.Status != run.StatusRunning || p.Status != run.StatusWaitingForInput || interruptStatus(ctx, t, d, irq) != "PENDING" {
		t.Fatalf("提交要整個回滾：回答 %s、提問 %s", a.Status, p.Status)
	}
}

func TestAnswerRunThatFailsOrIsCancelledLeavesTheInterruptPendingAndTheBuyerCanAnswerAgain(t *testing.T) {
	for name, end := range map[string]func(ctx context.Context, t *testing.T, d *db.DB, answer uuid.UUID){
		"executor 失敗": func(ctx context.Context, t *testing.T, d *db.DB, answer uuid.UUID) {
			disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: map[string]run.Executor{run.RunTypeResume: failingExecutor{}}}
			if err := disp.Run(ctx, run.RunArgs{RunID: answer}); err != nil {
				t.Fatal(err)
			}
		},
		"worker 遺失": func(ctx context.Context, t *testing.T, d *db.DB, answer uuid.UUID) {
			if _, err := reconciler(d).Reconcile(ctx, &fakeStates{states: map[int64]rivertype.JobState{}}); err != nil {
				t.Fatal(err)
			}
		},
		"買家取消": func(ctx context.Context, t *testing.T, d *db.DB, answer uuid.UUID) {
			start(ctx, t, d, answer)
			c := run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: &recordingJobs{}}
			if _, err := c.Cancel(ctx, 1, answer); err != nil {
				t.Fatal(err)
			}
			if _, err := ledger(d, now).Transition(ctx, answer, run.StatusCancelled, ""); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			d := newDB(t)
			s := submitter(d, &recordingJobs{}, wide())
			parent, irq, req := waitingParent(ctx, t, d, s, 1)
			answer, err := s.Resume(ctx, resumeInput(irq, "L"))
			if err != nil {
				t.Fatal(err)
			}
			end(ctx, t, d, answer)
			if r := runOf(ctx, t, d, answer); r.Status != run.StatusFailed && r.Status != run.StatusCancelled {
				t.Fatalf("answer = %s", r.Status)
			}
			if p := runOf(ctx, t, d, parent); p.Status != run.StatusWaitingForInput || p.CompletedAt != nil {
				t.Fatalf("提問的 run 維持等待輸入：%+v", p)
			}
			got, ok, err := ledger(d, now).OpenInterruptForRequest(ctx, 1, req)
			if err != nil || !ok || got.ID != irq || got.Status != "PENDING" || got.Processing {
				t.Fatalf("中斷維持 PENDING、不再是處理中：%+v %v %v", got, ok, err)
			}
			if again, err := s.Resume(ctx, resumeInput(irq, "中尺寸")); err != nil || again == answer {
				t.Fatalf("買家可以再答一次：%v", err)
			}
		})
	}
}

func TestAnswerRunThatAsksAgainResolvesTheOldInterruptAndLeavesOnlyTheNewOnePending(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	parent, irq, req := waitingParent(ctx, t, d, s, 1)
	answer, err := s.Resume(ctx, resumeInput(irq, "大"))
	if err != nil {
		t.Fatal(err)
	}
	start(ctx, t, d, answer)
	l := ledger(d, now)
	next, err := l.Interrupt(ctx, run.InterruptInput{RunID: answer, Payload: []byte(`{"question":"顏色？"}`), TurnResultEvent: []byte(`{"turn_result":{}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if interruptStatus(ctx, t, d, irq) != "RESOLVED" || runOf(ctx, t, d, parent).Status != run.StatusSucceeded || runOf(ctx, t, d, answer).Status != run.StatusWaitingForInput {
		t.Fatal("新問題寫入時，舊中斷解決、提問的 run 成功、回答的 run 改為等待")
	}
	if pendingInterrupts(ctx, t, d, req) != 1 {
		t.Fatalf("PENDING 只剩新的一筆：%d", pendingInterrupts(ctx, t, d, req))
	}
	if got, ok, err := l.OpenInterruptForRequest(ctx, 1, req); err != nil || !ok || got.ID != next || got.RunID != answer || got.Processing {
		t.Fatalf("需求目前等回答的是新中斷：%+v %v %v", got, ok, err)
	}
	second, err := s.Resume(ctx, resumeInput(next, "紅"))
	if err != nil {
		t.Fatal(err)
	}
	start(ctx, t, d, second)
	if _, err := l.Transition(ctx, second, run.StatusSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	if interruptStatus(ctx, t, d, next) != "RESOLVED" || runOf(ctx, t, d, answer).Status != run.StatusSucceeded || pendingInterrupts(ctx, t, d, req) != 0 {
		t.Fatal("第二輪回答成功要連鎖解決新中斷並讓回答的 run 成功")
	}
}

func TestAnswerCommitIsRefusedWhenTheAskingRunWasCancelledMeanwhile(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	parent, irq, _ := waitingParent(ctx, t, d, s, 1)
	answer, err := s.Resume(ctx, resumeInput(irq, "L"))
	if err != nil {
		t.Fatal(err)
	}
	start(ctx, t, d, answer)
	l := ledger(d, now)
	if _, err := l.Transition(ctx, parent, run.StatusCancelled, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Transition(ctx, answer, run.StatusSucceeded, ""); !errors.Is(err, run.ErrInterruptNotPending) {
		t.Fatalf("err = %v，要 ErrInterruptNotPending", err)
	}
	if runOf(ctx, t, d, answer).Status != run.StatusRunning || interruptStatus(ctx, t, d, irq) != "CANCELLED" {
		t.Fatal("提交被拒絕時回答的 run 留在原狀態")
	}
}

func TestAtMostOnePendingInterruptPerRequest(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	_, _, req := waitingParent(ctx, t, d, s, 1)
	other := seedRun(ctx, t, d, req, 1, run.StatusRunning, nil)
	if _, err := ledger(d, now).Interrupt(ctx, run.InterruptInput{RunID: other, Payload: []byte(`{}`)}); err == nil {
		t.Fatal("同一個需求已有 PENDING 中斷，第二筆要被唯一索引擋下")
	}
	if runOf(ctx, t, d, other).Status != run.StatusRunning || pendingInterrupts(ctx, t, d, req) != 1 {
		t.Fatal("被擋下的寫入要整個回滾")
	}
}

func TestConcurrentResumesOfOneInterruptAdmitExactlyOne(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	_, irq, _ := waitingParent(ctx, t, d, s, 1)
	var wg sync.WaitGroup
	var ok, rejected atomic.Int32
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Resume(ctx, resumeInput(irq, "L"))
			if err == nil {
				ok.Add(1)
			} else if reason, _ := reasonOf(err); reason == apperr.ReasonProcurementRunInProgress {
				rejected.Add(1)
			} else {
				t.Errorf("意外的錯誤：%v", err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || rejected.Load() != 5 {
		t.Fatalf("成功 %d、拒絕 %d", ok.Load(), rejected.Load())
	}
}

func TestSubmitSupersedesTheWaitingRunOfTheSameRequestEvenWithoutAConversation(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, wide())
	parent, irq, req := waitingParent(ctx, t, d, s, 1)
	nextID, err := s.Submit(ctx, input(1, req, 100))
	if err != nil {
		t.Fatal(err)
	}
	if runOf(ctx, t, d, parent).Status != run.StatusCancelled || interruptStatus(ctx, t, d, irq) != "CANCELLED" || pendingInterrupts(ctx, t, d, req) != 0 {
		t.Fatal("同一個需求的新一輪要取代等待中的 run 並取消它的 PENDING 中斷，不論對話")
	}
	start(ctx, t, d, nextID)
	if _, err := ledger(d, now).Interrupt(ctx, run.InterruptInput{RunID: nextID, Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("取代之後新的一輪可以提出新問題：%v", err)
	}
}

func TestPerRequestInProgressCheckComesBeforeTheBuyerWideLimit(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	s := submitter(d, &recordingJobs{}, run.Limits{MaxActiveRuns: 1, MaxSubmitsPerMinute: 100})
	req := seedRequest(ctx, t, d, 1)
	if _, err := s.Submit(ctx, input(1, req, 100)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Submit(ctx, input(1, req, 100)); err == nil {
		t.Fatal("同一個需求已有進行中的 run，要被擋下")
	} else if reason, kind := reasonOf(err); reason != apperr.ReasonProcurementRunInProgress || kind != apperr.KindFailedPrecondition {
		t.Fatalf("同需求與買家上限同時成立時要回 PROCUREMENT_RUN_IN_PROGRESS：%v", err)
	}
}

func TestCancellingTheAskingRunWhileItsAnswerRunIsInProgressCancelsTheAnswerRun(t *testing.T) {
	for name, tc := range map[string]struct {
		jobState rivertype.JobState
		want     string
	}{
		"job 執行中":  {rivertype.JobStateRunning, run.StatusCancelRequested},
		"job 還沒領走": {rivertype.JobStateCancelled, run.StatusCancelled},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			d := newDB(t)
			jobs := &recordingJobs{cancelState: tc.jobState}
			s := submitter(d, jobs, wide())
			parent, irq, _ := waitingParent(ctx, t, d, s, 1)
			answer, err := s.Resume(ctx, resumeInput(irq, "L"))
			if err != nil {
				t.Fatal(err)
			}
			start(ctx, t, d, answer)
			c := run.Canceller{DB: d, Ledger: ledger(d, now), Jobs: jobs}
			if out, err := c.Cancel(ctx, 1, parent); err != nil || out.Status != run.StatusCancelled {
				t.Fatalf("%+v %v", out, err)
			}
			if got := runOf(ctx, t, d, answer); got.Status != tc.want {
				t.Fatalf("提問的 run 被取消時回答 run = %s，預期 %s", got.Status, tc.want)
			}
			if len(jobs.cancels) != 1 || jobs.cancels[0] != *runOf(ctx, t, d, answer).JobID {
				t.Fatalf("要取消回答 run 的 job：%v", jobs.cancels)
			}
			if interruptStatus(ctx, t, d, irq) != "CANCELLED" {
				t.Fatal("中斷要轉 CANCELLED")
			}
		})
	}
}
