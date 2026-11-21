package run

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

var allStatuses = []string{
	StatusQueued, StatusRunning, StatusWaitingForInput, StatusCancelRequested,
	StatusSucceeded, StatusFailed, StatusCancelled, StatusResumed, StatusExpired,
}

// 期望的邊直接寫在測試裡，不從 edges 推導：表被改動時這裡要同步改。
var wantEdges = map[string][]string{
	StatusQueued:          {StatusRunning, StatusCancelRequested, StatusFailed},
	StatusRunning:         {StatusWaitingForInput, StatusSucceeded, StatusFailed, StatusCancelRequested},
	StatusCancelRequested: {StatusCancelled, StatusFailed},
	StatusWaitingForInput: {StatusResumed, StatusExpired},
}

func TestEveryPairOfStatusesIsAllowedOrRejectedAsTheTableSays(t *testing.T) {
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			want := slices.Contains(wantEdges[from], to)
			if got := CanTransition(from, to); got != want {
				t.Errorf("CanTransition(%s, %s) = %v，要 %v", from, to, got, want)
			}
		}
	}
	for _, bad := range []string{"", "UNKNOWN", "queued"} {
		if CanTransition(bad, StatusRunning) || CanTransition(StatusQueued, bad) || IsTerminal(bad) {
			t.Errorf("%q 不是狀態，不得出現在任何邊或終態", bad)
		}
	}
}

func TestTerminalStatesAreAbsorbingAndOnlyActiveOnesAcceptEvents(t *testing.T) {
	terminal := []string{StatusSucceeded, StatusFailed, StatusCancelled, StatusResumed, StatusExpired}
	for _, s := range allStatuses {
		if got, want := IsTerminal(s), slices.Contains(terminal, s); got != want {
			t.Errorf("IsTerminal(%s) = %v", s, got)
		}
	}
	for _, s := range terminal {
		for _, to := range allStatuses {
			if slices.Contains(allowedFrom(to), s) {
				t.Errorf("終態 %s 不得是任何轉換的來源（→ %s）", s, to)
			}
		}
	}
	active := activeStatuses()
	slices.Sort(active)
	if want := []string{StatusCancelRequested, StatusQueued, StatusRunning, StatusWaitingForInput}; !slices.Equal(active, want) {
		t.Errorf("activeStatuses = %v，要 %v", active, want)
	}
}

func TestAllowedFromIsTheInverseOfTheEdges(t *testing.T) {
	cases := map[string][]string{
		StatusRunning:         {StatusQueued},
		StatusCancelRequested: {StatusQueued, StatusRunning},
		StatusFailed:          {StatusCancelRequested, StatusQueued, StatusRunning},
		StatusCancelled:       {StatusCancelRequested},
		StatusSucceeded:       {StatusRunning},
		StatusWaitingForInput: {StatusRunning},
		StatusResumed:         {StatusWaitingForInput},
		StatusExpired:         {StatusWaitingForInput},
		StatusQueued:          nil,
	}
	for to, want := range cases {
		if got := allowedFrom(to); !slices.Equal(got, want) {
			t.Errorf("allowedFrom(%s) = %v，要 %v", to, got, want)
		}
	}
}

func TestCheckTargetRejectsStatesAndReasonsThatAreNotValid(t *testing.T) {
	bad := []struct{ to, reason string }{
		{"", ""}, {"UNKNOWN", ""}, {StatusQueued, ""},
		{StatusFailed, ""}, {StatusFailed, "nope"}, {StatusFailed, FailureExpired},
		{StatusExpired, ""}, {StatusExpired, FailureWorkerLost},
		{StatusSucceeded, FailureWorkerLost}, {StatusCancelled, FailureWorkerLost},
	}
	for _, c := range bad {
		if err := checkTarget(c.to, c.reason); err == nil {
			t.Errorf("checkTarget(%q, %q) 要拒絕", c.to, c.reason)
		}
	}
	good := []struct{ to, reason string }{
		{StatusRunning, ""}, {StatusSucceeded, ""}, {StatusCancelled, ""}, {StatusCancelRequested, ""},
		{StatusFailed, FailureWorkerLost}, {StatusFailed, FailureSearchFailed}, {StatusFailed, FailureTurnPipelineErr},
		{StatusExpired, FailureExpired},
	}
	for _, c := range good {
		if err := checkTarget(c.to, c.reason); err != nil {
			t.Errorf("checkTarget(%q, %q) = %v", c.to, c.reason, err)
		}
	}
}

func TestAppendRefusesTheStreamTerminalEvents(t *testing.T) {
	for _, typ := range []string{EventFinished, EventError} {
		if _, err := (Ledger{}).Append(context.Background(), uuid.NewV7(), typ, []byte(`{}`)); err == nil {
			t.Errorf("Append(%s) 要拒絕：結尾事件只隨終態轉換寫入", typ)
		}
	}
}

func TestTerminalErrorIsTypedForExecutorsToStopOn(t *testing.T) {
	wrapped := errors.Join(errors.New("executor"), ErrRunTerminal)
	if !errors.Is(wrapped, ErrRunTerminal) {
		t.Fatal("ErrRunTerminal 要能用 errors.Is 判斷")
	}
	var inv *InvalidTransitionError
	if !errors.As(error(&InvalidTransitionError{From: StatusQueued, To: StatusSucceeded}), &inv) || inv.To != StatusSucceeded {
		t.Fatal("InvalidTransitionError 要能用 errors.As 取出")
	}
}

func TestDecideMapsRunAndJobStatesAsTheLifecycleDecisionSays(t *testing.T) {
	type key struct {
		status string
		state  rivertype.JobState
	}
	skip := outcome{action: actionSkip}
	lost := outcome{action: actionFail, reason: FailureWorkerLost, refund: true}
	violation := outcome{action: actionFail, reason: FailureTurnPipelineErr, refund: true}
	cancel := outcome{action: actionCancel}
	active := []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRetryable, rivertype.JobStateRunning, rivertype.JobStateScheduled}
	want := map[key]outcome{}
	for _, status := range []string{StatusQueued, StatusRunning, StatusCancelRequested} {
		for _, st := range active {
			want[key{status, st}] = skip
		}
		want[key{status, rivertype.JobStateDiscarded}] = lost
	}
	want[key{StatusQueued, rivertype.JobStateCompleted}] = violation
	want[key{StatusRunning, rivertype.JobStateCompleted}] = violation
	want[key{StatusCancelRequested, rivertype.JobStateCompleted}] = cancel
	want[key{StatusCancelRequested, rivertype.JobStateCancelled}] = cancel
	want[key{StatusQueued, rivertype.JobStateCancelled}] = lost
	want[key{StatusRunning, rivertype.JobStateCancelled}] = lost
	for k, w := range want {
		r := store.ProcurementAgentRun{Status: k.status, RunType: "TURN"}
		if got := decide(r, k.state, true); got != w {
			t.Errorf("decide(%s, job %s) = %+v，要 %+v", k.status, k.state, got, w)
		}
		if got := decide(r, "", false); got != lost {
			t.Errorf("decide(%s, job 不存在) = %+v，要 %+v", k.status, got, lost)
		}
	}
	search := store.ProcurementAgentRun{Status: StatusRunning, RunType: runTypeCandidateSearch}
	if got := decide(search, rivertype.JobStateCompleted, true); got.reason != FailureSearchFailed || !got.refund {
		t.Errorf("候選搜尋的執行者違約要 SEARCH_FAILED 並退額度：%+v", got)
	}
	if got := decide(store.ProcurementAgentRun{Status: StatusRunning}, "bogus", true); got != skip {
		t.Errorf("未知的 job 狀態要略過、不收斂：%+v", got)
	}
}

type fakeTx struct {
	staged    []uuid.UUID
	committed []uuid.UUID
}

func (f *fakeTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	f.staged = nil
	if err := fn(ctx); err != nil {
		f.staged = nil
		return err
	}
	f.committed = append(f.committed, f.staged...)
	f.staged = nil
	return nil
}

func TestReconcileEachIsolatesARowThatFails(t *testing.T) {
	ids := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}
	tx := &fakeTx{}
	res := reconcileEach(context.Background(), tx, ids, func(_ context.Context, id uuid.UUID) (action, error) {
		tx.staged = append(tx.staged, id)
		switch id {
		case ids[1]:
			return actionSkip, errors.New("job 讀取失敗")
		case ids[2]:
			return actionCancel, nil
		}
		return actionFail, nil
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if want := []uuid.UUID{ids[0], ids[2], ids[3]}; !slices.Equal(tx.committed, want) {
		t.Fatalf("committed = %v，要 %v：一列失敗不得讓其他列回滾或被跳過", tx.committed, want)
	}
	if res != (ReconcileResult{Failed: 2, Cancelled: 1, Errors: 1}) {
		t.Fatalf("result = %+v", res)
	}
}

func TestRescueWindowCoversTheRunJobTimeout(t *testing.T) {
	if jobs.RescueStuckJobsAfter < RunTimeoutLimit {
		t.Fatalf("RescueStuckJobsAfter = %s 小於 run job 的 Timeout %s：執行中的 run 會被 River 丟棄", jobs.RescueStuckJobsAfter, RunTimeoutLimit)
	}
	if got := (&runWorker{}).Timeout(&river.Job[RunArgs]{}); got != RunTimeoutLimit {
		t.Fatalf("worker Timeout = %s，要 %s", got, RunTimeoutLimit)
	}
	opts := RunArgs{}.InsertOpts()
	if opts.MaxAttempts != 1 || opts.UniqueOpts.ByArgs {
		t.Fatalf("run job 要 MaxAttempts 1 且不設唯一鍵：%+v", opts)
	}
}
