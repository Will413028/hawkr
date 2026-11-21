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
	if err != nil || !ok || byRun.ID != irq || byRun.Status != "PENDING" || string(byRun.Payload) != `{"question": "尺寸？"}` {
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

func TestSupersededWaitingRunKeepsItsInterruptButItIsNoLongerOpen(t *testing.T) {
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
	if _, err := s.Submit(ctx, next); err != nil {
		t.Fatal(err)
	}
	if runOf(ctx, t, d, id).Status != run.StatusCancelled {
		t.Fatal("等待中的 run 要被取代")
	}
	l := ledger(d, now)
	if _, ok, _ := l.OpenInterruptForRun(ctx, 1, id); ok {
		t.Fatal("被取代的 run 沒有未回答的中斷")
	}
	if _, ok, _ := l.OpenInterruptForRequest(ctx, 1, req); ok {
		t.Fatal("需求沒有未回答的中斷")
	}
	if st := count(ctx, t, d, `SELECT count(*) FROM procurement.agent_run_interrupts WHERE id = $1 AND status = 'PENDING'`, irq); st != 1 {
		t.Fatal("中斷維持 PENDING，RESOLVED 只表示被子 run 回答")
	}
	if _, err := s.Resume(ctx, run.ResumeInput{BuyerID: 1, InterruptID: irq, ClientRequestID: uuid.NewV7(), Message: "L"}); err == nil {
		t.Fatal("被取代的 run 的中斷不能再回答")
	} else if reason, _ := reasonOf(err); reason != apperr.ReasonRunNotActive {
		t.Fatalf("要 RUN_NOT_ACTIVE：%v", err)
	}
}

func TestResumeCreatesAChildRunWithoutChargingAndResolvesTheInterrupt(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	jobs := &recordingJobs{}
	s := submitter(d, jobs, wide())
	parent, irq, req := waitingParent(ctx, t, d, s, 1)
	usedBefore, jobsBefore := used(ctx, t, d, 1), jobs.n.Load()
	in := run.ResumeInput{BuyerID: 1, InterruptID: irq, ClientRequestID: uuid.NewV7(), Message: "大尺寸"}
	child, err := s.Resume(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	c, p := runOf(ctx, t, d, child), runOf(ctx, t, d, parent)
	if c.RunType != run.RunTypeResume || c.Status != run.StatusQueued || c.ParentRunID == nil || *c.ParentRunID != parent || c.QuotaPeriodStart != nil || c.RequestID != req || c.JobID == nil {
		t.Fatalf("child = %+v", c)
	}
	if p.Status != run.StatusResumed || p.ResumedIntoRunID == nil || *p.ResumedIntoRunID != child || lastEventType(ctx, t, d, parent) != run.EventFinished {
		t.Fatalf("parent = %+v", p)
	}
	if used(ctx, t, d, 1) != usedBefore || jobs.n.Load() != jobsBefore+1 {
		t.Fatal("子 run 不扣額度、排一個 job")
	}
	if n := count(ctx, t, d, `SELECT count(*) FROM procurement.agent_run_interrupts WHERE id = $1 AND status = 'RESOLVED' AND resolved_by_child_run_id = $2`, irq, child); n != 1 {
		t.Fatal("中斷要被子 run 回答")
	}
	if lastEventType(ctx, t, d, child) != run.EventRunAccepted {
		t.Fatal("子 run 的第一個事件是 run_accepted")
	}
	if again, err := s.Resume(ctx, in); err != nil || again != child || jobs.n.Load() != jobsBefore+1 {
		t.Fatalf("同一個 client_request_id 重送回同一個子 run：%v %v", again, err)
	}
	in2 := in
	in2.ClientRequestID = uuid.NewV7()
	if _, err := s.Resume(ctx, in2); err == nil {
		t.Fatal("成功之後以同一個中斷再答要被拒絕")
	} else if reason, kind := reasonOf(err); reason != apperr.ReasonApprovalAlreadyResolved || kind != apperr.KindFailedPrecondition {
		t.Fatalf("要 APPROVAL_ALREADY_RESOLVED：%v", err)
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
			_, err := s.Resume(ctx, run.ResumeInput{BuyerID: 1, InterruptID: irq, ClientRequestID: uuid.NewV7(), Message: "L"})
			if err == nil {
				ok.Add(1)
			} else if reason, _ := reasonOf(err); reason == apperr.ReasonApprovalAlreadyResolved || reason == apperr.ReasonRunNotActive {
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

func TestFailedChildRunReopensTheInterruptSoTheBuyerCanAnswerAgain(t *testing.T) {
	for name, fail := range map[string]func(ctx context.Context, t *testing.T, d *db.DB, child uuid.UUID){
		"executor 失敗": func(ctx context.Context, t *testing.T, d *db.DB, child uuid.UUID) {
			disp := run.Dispatcher{DB: d, Ledger: ledger(d, now), Executors: map[string]run.Executor{run.RunTypeResume: failingExecutor{}}}
			if err := disp.Run(ctx, run.RunArgs{RunID: child}); err != nil {
				t.Fatal(err)
			}
		},
		"worker 遺失": func(ctx context.Context, t *testing.T, d *db.DB, child uuid.UUID) {
			if _, err := reconciler(d).Reconcile(ctx, &fakeStates{states: map[int64]rivertype.JobState{}}); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			d := newDB(t)
			s := submitter(d, &recordingJobs{}, wide())
			parent, irq, _ := waitingParent(ctx, t, d, s, 1)
			child, err := s.Resume(ctx, run.ResumeInput{BuyerID: 1, InterruptID: irq, ClientRequestID: uuid.NewV7(), Message: "L"})
			if err != nil {
				t.Fatal(err)
			}
			usedBefore := used(ctx, t, d, 1)
			fail(ctx, t, d, child)
			if r := runOf(ctx, t, d, child); r.Status != run.StatusFailed {
				t.Fatalf("child = %s", r.Status)
			}
			p := runOf(ctx, t, d, parent)
			if p.Status != run.StatusWaitingForInput || p.ResumedIntoRunID != nil || p.CompletedAt != nil {
				t.Fatalf("父 run 要回到等待輸入：%+v", p)
			}
			if got, ok, err := ledger(d, now).OpenInterruptForRun(ctx, 1, parent); err != nil || !ok || got.ID != irq || got.ResolvedByChildRunID != nil || got.ResolvedAt != nil {
				t.Fatalf("中斷要重新開放：%+v %v %v", got, ok, err)
			}
			if used(ctx, t, d, 1) != usedBefore {
				t.Fatal("子 run 失敗不退額度（它沒扣）")
			}
			again, err := s.Resume(ctx, run.ResumeInput{BuyerID: 1, InterruptID: irq, ClientRequestID: uuid.NewV7(), Message: "中尺寸"})
			if err != nil || again == child {
				t.Fatalf("買家可以再答一次：%v", err)
			}
		})
	}
}
