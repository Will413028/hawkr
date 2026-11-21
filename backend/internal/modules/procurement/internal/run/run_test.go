package run

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

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

func TestOnlyFinishedAndErrorEndTheStream(t *testing.T) {
	if !IsTerminalEvent(EventFinished) || !IsTerminalEvent(EventError) || IsTerminalEvent(EventRunAccepted) || IsTerminalEvent("text") {
		t.Fatal("串流的結尾只有 finished 與 error")
	}
}

type sweepProbe struct {
	river.WorkerDefaults[SweepArgs]
}

func (sweepProbe) Work(context.Context, *river.Job[SweepArgs]) error { return nil }

type purgeProbe struct {
	river.WorkerDefaults[PurgeArgs]
}

func (purgeProbe) Work(context.Context, *river.Job[PurgeArgs]) error { return nil }

type runProbe struct{ river.WorkerDefaults[RunArgs] }

func (runProbe) Work(context.Context, *river.Job[RunArgs]) error { return nil }

// 以「再登記同一個 kind 會失敗」探測 worker 是否已登記：沒登記 sweeper，逾時的 run 就永遠停在 QUEUED／RUNNING（AD2）。
func TestAddWorkersRegistersTheSweeperAndThePurger(t *testing.T) {
	workers := river.NewWorkers()
	AddWorkers(workers, Sweeper{}, Purger{})
	if err := river.AddWorkerSafely(workers, &sweepProbe{}); err == nil {
		t.Error("sweeper 沒有登記")
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

func TestPeriodicJobsCoverTheSweeperAndThePurger(t *testing.T) {
	if n := len(PeriodicJobs()); n != 2 {
		t.Fatalf("排程 = %d 個，要 sweeper 與清除各一個", n)
	}
}
