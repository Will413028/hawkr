package run

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var allStatuses = []string{
	StatusQueued, StatusRunning, StatusWaitingForInput, StatusCancelRequested,
	StatusSucceeded, StatusFailed, StatusCancelled,
}

// 期望的邊直接寫在測試裡，不從 edges 推導：表被改動時這裡要同步改。
var wantEdges = map[string][]string{
	StatusQueued:          {StatusRunning, StatusCancelRequested, StatusFailed},
	StatusRunning:         {StatusWaitingForInput, StatusSucceeded, StatusFailed, StatusCancelRequested},
	StatusCancelRequested: {StatusCancelled, StatusFailed},
	StatusWaitingForInput: {StatusSucceeded, StatusCancelled},
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
	terminal := []string{StatusSucceeded, StatusFailed, StatusCancelled}
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
		StatusCancelled:       {StatusCancelRequested, StatusWaitingForInput},
		StatusSucceeded:       {StatusRunning, StatusWaitingForInput},
		StatusWaitingForInput: {StatusRunning},
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
		{StatusFailed, ""}, {StatusFailed, "nope"}, {StatusFailed, "EXPIRED"}, {"EXPIRED", ""}, {"EXPIRED", "EXPIRED"},
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
	}
	for _, c := range good {
		if err := checkTarget(c.to, c.reason); err != nil {
			t.Errorf("checkTarget(%q, %q) = %v", c.to, c.reason, err)
		}
	}
}

func TestAppendRefusesTheStreamTerminalEvents(t *testing.T) {
	for _, typ := range []string{EventFinished} {
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
	}
	want[key{StatusQueued, rivertype.JobStateCompleted}] = violation
	want[key{StatusRunning, rivertype.JobStateCompleted}] = violation
	want[key{StatusCancelRequested, rivertype.JobStateCompleted}] = cancel
	want[key{StatusCancelRequested, rivertype.JobStateCancelled}] = cancel
	want[key{StatusCancelRequested, rivertype.JobStateDiscarded}] = cancel
	want[key{StatusQueued, rivertype.JobStateDiscarded}] = lost
	want[key{StatusRunning, rivertype.JobStateDiscarded}] = lost
	want[key{StatusQueued, rivertype.JobStateCancelled}] = lost
	want[key{StatusRunning, rivertype.JobStateCancelled}] = lost
	for k, w := range want {
		r := store.ProcurementAgentRun{Status: k.status, RunType: "TURN"}
		if got := decide(r, k.state, true); got != w {
			t.Errorf("decide(%s, job %s) = %+v，要 %+v", k.status, k.state, got, w)
		}
	}
	for _, status := range []string{StatusQueued, StatusRunning, StatusCancelRequested} {
		w := lost
		if status == StatusCancelRequested {
			w = cancel
		}
		if got := decide(store.ProcurementAgentRun{Status: status, RunType: "TURN"}, "", false); got != w {
			t.Errorf("decide(%s, job 不存在) = %+v，要 %+v", status, got, w)
		}
	}
	search := store.ProcurementAgentRun{Status: StatusRunning, RunType: RunTypeCandidateSearch}
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
	res, err := reconcileEach(context.Background(), tx, ids, func(_ context.Context, id uuid.UUID) (action, error) {
		tx.staged = append(tx.staged, id)
		switch id {
		case ids[1]:
			return actionSkip, errors.New("job 讀取失敗")
		case ids[2]:
			return actionCancel, nil
		}
		return actionFail, nil
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
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

func TestReconcileEachStopsOnceTheContextIsDoneWithoutLoggingEachRow(t *testing.T) {
	cases := map[string]func(context.Context) error{
		"處理完才發現關機":       func(context.Context) error { return nil },
		"處理中因關機而失敗（不記錄）": func(ctx context.Context) error { return ctx.Err() },
	}
	for name, outcomeOf := range cases {
		t.Run(name, func(t *testing.T) {
			ids := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}
			ctx, cancel := context.WithCancel(context.Background())
			var processed int
			var logs bytes.Buffer
			res, err := reconcileEach(ctx, &fakeTx{}, ids, func(ctx context.Context, _ uuid.UUID) (action, error) {
				processed++
				cancel()
				return actionSkip, outcomeOf(ctx)
			}, slog.New(slog.NewTextHandler(&logs, nil)))
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v，要 context.Canceled", err)
			}
			if processed != 1 || res.Errors != 0 || logs.Len() != 0 {
				t.Fatalf("processed = %d、errors = %d、log = %q：關機後不得再處理其他列或逐列記錄失敗", processed, res.Errors, logs.String())
			}
		})
	}
}

func TestSettleOutcomeDistinguishesBuyerCancelShutdownAndPipelineFailure(t *testing.T) {
	lost := outcome{action: actionFail, reason: FailureWorkerLost, refund: true}
	cases := []struct {
		name, status, runType string
		ctxErr, execErr       error
		want                  outcome
	}{
		{"買家取消", StatusCancelRequested, "TURN", context.Canceled, nil, outcome{action: actionCancel}},
		{"買家取消且 ctx 沒取消", StatusCancelRequested, "TURN", nil, nil, outcome{action: actionCancel}},
		{"關機", StatusRunning, "TURN", context.Canceled, nil, lost},
		{"逾時", StatusRunning, RunTypeCandidateSearch, context.DeadlineExceeded, nil, lost},
		{"一般失敗：搜尋", StatusRunning, RunTypeCandidateSearch, nil, nil, outcome{action: actionFail, reason: FailureSearchFailed, refund: true}},
		{"一般失敗：輪次", StatusRunning, "TURN", nil, nil, outcome{action: actionFail, reason: FailureTurnPipelineErr, refund: true}},
		{"模型不可用：輪次", StatusRunning, "TURN", nil, &FailureError{Reason: FailureModelUnavailable, Err: errors.New("x")}, outcome{action: actionFail, reason: FailureModelUnavailable, refund: true}},
		{"未知原因退回 pipeline 失敗", StatusRunning, "TURN", nil, &FailureError{Reason: "NOPE", Err: errors.New("x")}, outcome{action: actionFail, reason: FailureTurnPipelineErr, refund: true}},
		{"關機優先於分類", StatusRunning, "TURN", context.Canceled, &FailureError{Reason: FailureModelUnavailable, Err: errors.New("x")}, lost},
	}
	for _, c := range cases {
		if got := settleOutcome(c.status, c.runType, c.ctxErr, c.execErr); got != c.want {
			t.Errorf("%s：settleOutcome = %+v，要 %+v", c.name, got, c.want)
		}
	}
}

func TestClaimResultSkipsRunsThatCannotBeClaimedIncludingDeletedOnes(t *testing.T) {
	for name, err := range map[string]error{
		"已終結": ErrRunTerminal, "已被領走": &InvalidTransitionError{From: StatusRunning, To: StatusRunning},
		"已被刪除": apperr.NotFound("找不到這個 run"),
	} {
		if claimed, ret := claimResult(err); claimed || ret != nil {
			t.Errorf("%s：claimResult = %v, %v，要略過且不回錯誤", name, claimed, ret)
		}
	}
	if claimed, ret := claimResult(nil); !claimed || ret != nil {
		t.Errorf("成功轉 RUNNING 要領到：%v, %v", claimed, ret)
	}
	if claimed, ret := claimResult(errors.New("連線中斷")); claimed || ret == nil {
		t.Errorf("其他錯誤要回傳給 River：%v, %v", claimed, ret)
	}
}

func TestReadRunErrorTurnsAMissingRunIntoNotFound(t *testing.T) {
	if got := readRunError(pgx.ErrNoRows); apperr.KindOf(got) != apperr.KindNotFound {
		t.Fatalf("readRunError(ErrNoRows) = %v，要 NOT_FOUND", got)
	}
	boom := errors.New("boom")
	if got := readRunError(boom); !errors.Is(got, boom) || apperr.KindOf(got) != apperr.KindInternal {
		t.Fatalf("其他錯誤原樣回傳：%v", got)
	}
}

func TestOnlyACancelledJobSettlesTheCancelWithoutTheReconciler(t *testing.T) {
	for state, want := range map[rivertype.JobState]bool{
		rivertype.JobStateCancelled: true, rivertype.JobStateRunning: false, rivertype.JobStateAvailable: false,
		rivertype.JobStateCompleted: false, rivertype.JobStateDiscarded: false, rivertype.JobStateScheduled: false,
	} {
		if got := jobCancelledBeforeStart(state); got != want {
			t.Errorf("jobCancelledBeforeStart(%s) = %v", state, got)
		}
	}
}

func TestCancelAllCancelsEveryRunAndStopsAtTheFirstError(t *testing.T) {
	ids := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}
	var got []uuid.UUID
	if err := cancelAll(context.Background(), ids, func(_ context.Context, id uuid.UUID) error {
		got = append(got, id)
		return nil
	}); err != nil || !slices.Equal(got, ids) {
		t.Fatalf("cancelled = %v, err = %v，要全部 %v", got, err, ids)
	}
	got = nil
	boom := errors.New("轉換失敗")
	err := cancelAll(context.Background(), ids, func(_ context.Context, id uuid.UUID) error {
		got = append(got, id)
		if id == ids[1] {
			return boom
		}
		return nil
	})
	if !errors.Is(err, boom) || len(got) != 2 {
		t.Fatalf("err = %v、cancelled = %d：失敗要讓送出回滾，不得略過", err, len(got))
	}
}

func TestAWaitingRunCanBeCancelledButNeverFailedOrExpired(t *testing.T) {
	if !CanTransition(StatusWaitingForInput, StatusCancelled) || CanTransition(StatusWaitingForInput, StatusFailed) {
		t.Fatal("WAITING_FOR_INPUT 只能到 SUCCEEDED 與 CANCELLED")
	}
	if _, known := edges["EXPIRED"]; known {
		t.Fatal("EXPIRED 已刪除，沒有寫入者")
	}
}

func TestOnlyTurnRunsChargeQuota(t *testing.T) {
	for runType, want := range map[string]bool{RunTypeTurn: true, RunTypeCandidateSearch: false, RunTypeResume: false, "PUBLICATION": false} {
		if got := ChargesQuota(runType); got != want {
			t.Errorf("ChargesQuota(%s) = %v，要 %v", runType, got, want)
		}
	}
}

func TestLimitsDefaultToTwoInProgressAndTenPerMinute(t *testing.T) {
	var l Limits
	if l.active() != 2 || l.perMinute() != 10 {
		t.Fatalf("預設 %d、%d", l.active(), l.perMinute())
	}
	l = Limits{MaxActiveRuns: 5, MaxSubmitsPerMinute: 7}
	if l.active() != 5 || l.perMinute() != 7 {
		t.Fatalf("設定值 %d、%d", l.active(), l.perMinute())
	}
}

func TestPublicTransitionRejectsEnteringWaitingForInputWithoutTouchingTheDatabase(t *testing.T) {
	if _, err := (Ledger{}).Transition(context.Background(), uuid.NewV7(), StatusWaitingForInput, ""); err == nil {
		t.Fatal("公開的 Transition 要拒絕轉入 WAITING_FOR_INPUT")
	}
}

func TestPublicTransitionSourcesExcludeTheAnswerCommitEdge(t *testing.T) {
	if from := publicFrom(StatusSucceeded); slices.Contains(from, StatusWaitingForInput) || !slices.Contains(from, StatusRunning) {
		t.Fatalf("SUCCEEDED 的公開來源 = %v，不得含 WAITING_FOR_INPUT、要含 RUNNING", from)
	}
	if from := publicFrom(StatusCancelled); !slices.Contains(from, StatusWaitingForInput) {
		t.Fatalf("CANCELLED 的公開來源 = %v，要含 WAITING_FOR_INPUT", from)
	}
}

func TestLocaleCodeRoundTripsAndRejectsUnsupportedLocales(t *testing.T) {
	for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
		code, err := LocaleCode(l)
		if err != nil {
			t.Fatalf("LocaleCode(%q)：%v", l, err)
		}
		if got, err := LocaleOf(code); err != nil || got != l {
			t.Errorf("LocaleOf(%q) = %q, %v，要 %q", code, got, err, l)
		}
	}
	if _, err := LocaleCode(""); err == nil {
		t.Error("空語系不得被接受")
	}
	if _, err := LocaleOf("zh-TW"); err == nil {
		t.Error("未知代碼不得被接受")
	}
}
