package run

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func newTestHub(max int) *Hub {
	return NewHub(nil, max, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestSubscribeRejectsBeyondTheCap(t *testing.T) {
	h := newTestHub(MaxSubscriptions)
	runID := uuid.NewV7()
	var subs []*Subscription
	for i := 0; i < MaxSubscriptions; i++ {
		s, err := h.Subscribe(runID)
		if err != nil {
			t.Fatalf("第 %d 條訂閱不該被拒：%v", i+1, err)
		}
		subs = append(subs, s)
	}
	_, err := h.Subscribe(runID)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindResourceExhausted {
		t.Fatalf("超過上限要回 RESOURCE_EXHAUSTED，實際 %v", err)
	}
	subs[0].Close()
	subs[0].Close()
	if h.Count() != MaxSubscriptions-1 {
		t.Fatalf("Close 要釋放一個名額且可重複呼叫，Count = %d", h.Count())
	}
	if _, err := h.Subscribe(runID); err != nil {
		t.Fatalf("釋放名額之後要能再訂閱：%v", err)
	}
}

func TestNotifyWakesOnlyTheSubscribersOfThatRun(t *testing.T) {
	h := newTestHub(10)
	a, b := uuid.NewV7(), uuid.NewV7()
	sa1, _ := h.Subscribe(a)
	sa2, _ := h.Subscribe(a)
	sb, _ := h.Subscribe(b)
	h.notify(a)
	h.notify(a)
	for _, s := range []*Subscription{sa1, sa2} {
		select {
		case <-s.C:
		default:
			t.Fatal("run a 的訂閱者要被喚醒")
		}
	}
	select {
	case <-sb.C:
		t.Fatal("run b 的訂閱者不該被喚醒")
	default:
	}
}

func TestParseCursor(t *testing.T) {
	for in, want := range map[string]int64{"": 0, "1": 1, "42": 42} {
		if got, err := parseCursor(in); err != nil || got != want {
			t.Errorf("parseCursor(%q) = %d, %v", in, got, err)
		}
	}
	for _, in := range []string{"0", "-1", "abc", "1.5"} {
		if _, err := parseCursor(in); err == nil {
			t.Errorf("parseCursor(%q) 要回錯誤", in)
		}
	}
	if Cursor(0) != "" || Cursor(7) != "7" {
		t.Fatal("Cursor 與 parseCursor 要互為反函式")
	}
}

func TestFinishedPayloadIsProtoJSONOfTheFinishedEvent(t *testing.T) {
	b, err := finishedPayload(StatusFailed, FailureWorkerLost)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]string
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	f := got[EventFinished]
	if f["status"] != "RUN_STATUS_FAILED" || f["failureReason"] != "RUN_FAILURE_REASON_WORKER_LOST" {
		t.Fatalf("payload = %s", b)
	}
	b, _ = finishedPayload("SUCCEEDED", "")
	var ok map[string]map[string]string
	_ = json.Unmarshal(b, &ok)
	if _, has := ok[EventFinished]["failureReason"]; has {
		t.Fatalf("成功沒有失敗原因：%s", b)
	}
}

func TestOnlyFinishedEndsTheStream(t *testing.T) {
	if !IsTerminalEvent(EventFinished) || IsTerminalEvent("error") || IsTerminalEvent(EventRunAccepted) || IsTerminalEvent("text") {
		t.Fatal("串流的結尾只有 finished（失敗也以它收尾）")
	}
}

type reconcileProbe struct {
	river.WorkerDefaults[ReconcileArgs]
}

func (reconcileProbe) Work(context.Context, *river.Job[ReconcileArgs]) error { return nil }

type purgeProbe struct {
	river.WorkerDefaults[PurgeArgs]
}

func (purgeProbe) Work(context.Context, *river.Job[PurgeArgs]) error { return nil }

type runProbe struct{ river.WorkerDefaults[RunArgs] }

func (runProbe) Work(context.Context, *river.Job[RunArgs]) error { return nil }

// 以「再登記同一個 kind 會失敗」探測 worker 是否已登記：沒登記 reconciler，job 死了的 run 就永遠停在 QUEUED／RUNNING／CANCEL_REQUESTED（AD2）。
func TestAddWorkersRegistersTheReconcilerAndThePurger(t *testing.T) {
	workers := river.NewWorkers()
	AddWorkers(workers, Reconciler{}, Purger{})
	if err := river.AddWorkerSafely(workers, &reconcileProbe{}); err == nil {
		t.Error("reconciler 沒有登記")
	}
	if err := river.AddWorkerSafely(workers, &purgeProbe{}); err == nil {
		t.Error("事件清除沒有登記")
	}
	if err := river.AddWorkerSafely(workers, &runProbe{}); err != nil {
		t.Errorf("執行 run 的 worker 不在這個登記裡：%v", err)
	}
}

func TestDispatcherRegistersTheRunWorker(t *testing.T) {
	workers := river.NewWorkers()
	Dispatcher{}.AddWorker(workers)
	if err := river.AddWorkerSafely(workers, &runProbe{}); err == nil {
		t.Error("run worker 沒有登記")
	}
}

func TestPeriodicJobsCoverTheReconcilerAndThePurger(t *testing.T) {
	var kinds []string
	for _, spec := range periodicSpecs {
		kinds = append(kinds, spec.args.Kind())
	}
	want := []string{ReconcileArgs{}.Kind(), PurgeArgs{}.Kind()}
	if !slices.Equal(kinds, want) || len(PeriodicJobs()) != len(periodicSpecs) {
		t.Fatalf("排程 = %v（%d 個），要 %v", kinds, len(PeriodicJobs()), want)
	}
}

func TestHubBackoffResetsAfterASuccessfulListen(t *testing.T) {
	h := newTestHub(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var sleeps []time.Duration
	calls := 0
	h.listenFn = func(_ context.Context, ready func()) error {
		calls++
		if calls == 1 || calls == 3 {
			ready()
		}
		return errors.New("連線中斷")
	}
	h.sleep = func(_ context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		if len(sleeps) == 3 {
			cancel()
			return ctx.Err()
		}
		return nil
	}
	if err := h.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run = %v", err)
	}
	want := []time.Duration{reconnectMin, 2 * reconnectMin, reconnectMin}
	for i := range want {
		if sleeps[i] != want[i] {
			t.Fatalf("退避 = %v，要 %v：LISTEN 成功之後要從頭退避", sleeps, want)
		}
	}
}

func TestWatchStopsWithoutWaitingOnceAnEndedRunIsDrained(t *testing.T) {
	for _, status := range []string{"SUCCEEDED", "FAILED", "CANCELLED", "RESUMED"} {
		if !IsTerminal(status) || !drainedEndedRun(true, 0) {
			t.Fatalf("%s 的 run 讀完事件就要結束串流", status)
		}
	}
	for _, status := range []string{"QUEUED", "RUNNING", "WAITING_FOR_INPUT", "CANCEL_REQUESTED", ""} {
		if IsTerminal(status) {
			t.Fatalf("%s 不是結束狀態", status)
		}
	}
	if drainedEndedRun(false, 0) || drainedEndedRun(true, watchPage) {
		t.Fatal("未結束的 run、或這一頁填滿時不得結束串流")
	}
}

func TestDrainExpiredLoopsUntilNoRunAdvances(t *testing.T) {
	batches := [][2]int64{{100, 30}, {100, 20}, {3, 5}, {0, 0}}
	i := 0
	got, err := drainExpired(context.Background(), func(context.Context) (int64, int64, error) {
		b := batches[i]
		i++
		return b[0], b[1], nil
	})
	if err != nil || got != 55 || i != len(batches) {
		t.Fatalf("total = %d, steps = %d, err = %v；要 55 個事件、跑完 %d 批", got, i, err, len(batches))
	}
}

func TestAcceptedPayloadCarriesTheConversationID(t *testing.T) {
	run, conv := uuid.NewV7(), uuid.NewV7()
	read := func(c *uuid.UUID) map[string]string {
		b, err := acceptedPayload(run, c)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m[EventRunAccepted]
	}
	if got := read(&conv); got["runId"] != run.String() || got["conversationId"] != conv.String() {
		t.Fatalf("payload = %v", got)
	}
	if _, has := read(nil)["conversationId"]; has {
		t.Fatal("沒有對話時不寫 conversationId")
	}
}
