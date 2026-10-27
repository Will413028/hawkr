package outbox

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_TicksCallRun(t *testing.T) {
	var calls int32
	s := NewScheduler(10*time.Millisecond, func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}, nil, nil)

	s.Start(context.Background())
	time.Sleep(55 * time.Millisecond)
	s.Stop()

	if got := atomic.LoadInt32(&calls); got < 3 {
		t.Fatalf("calls = %d in 55ms at 10ms interval, want >= 3", got)
	}
}

func TestScheduler_StopHaltsFurtherTicks(t *testing.T) {
	var calls int32
	s := NewScheduler(10*time.Millisecond, func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}, nil, nil)

	s.Start(context.Background())
	time.Sleep(25 * time.Millisecond)
	s.Stop()
	afterStop := atomic.LoadInt32(&calls)
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != afterStop {
		t.Fatalf("calls advanced from %d to %d after Stop", afterStop, got)
	}
}

func TestScheduler_PanicInRunIsRecoveredAndReported(t *testing.T) {
	var recovered any
	done := make(chan struct{}, 1)
	s := NewScheduler(10*time.Millisecond, func(_ context.Context) error {
		panic("boom")
	}, func(_ context.Context, r any) {
		recovered = r
		select {
		case done <- struct{}{}:
		default:
		}
	}, nil)

	s.Start(context.Background())
	defer s.Stop()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("onPanic was not called within 200ms")
	}
	if recovered != "boom" {
		t.Fatalf("recovered = %v, want boom", recovered)
	}
}

func TestScheduler_RunErrorReportedViaOnError(t *testing.T) {
	errCh := make(chan error, 1)
	s := NewScheduler(10*time.Millisecond, func(_ context.Context) error {
		return errors.New("db down")
	}, nil, func(_ context.Context, err error) {
		select {
		case errCh <- err:
		default:
		}
	})

	s.Start(context.Background())
	defer s.Stop()

	select {
	case err := <-errCh:
		if err.Error() != "db down" {
			t.Fatalf("onError err = %v, want db down", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("onError was not called within 200ms")
	}
}

func TestScheduler_WithInitialDelayZero_RunsImmediately(t *testing.T) {
	done := make(chan struct{}, 1)
	s := NewScheduler(10*time.Second, func(_ context.Context) error {
		select {
		case done <- struct{}{}:
		default:
		}
		return nil
	}, nil, nil, WithInitialDelay(0))

	s.Start(context.Background())
	defer s.Stop()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("run was not called within 50ms of Start with WithInitialDelay(0)")
	}
}

func TestScheduler_WithInitialDelay_FirstTickFiresAfterDelayNotInterval(t *testing.T) {
	var calls int32
	s := NewScheduler(10*time.Second, func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}, nil, nil, WithInitialDelay(5*time.Millisecond))

	s.Start(context.Background())
	defer s.Stop()

	time.Sleep(30 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got < 1 {
		t.Fatalf("calls = %d after 30ms with a 5ms initial delay and 10s interval, want >= 1", got)
	}
}

func TestScheduler_WithoutInitialDelay_FirstTickWaitsFullInterval(t *testing.T) {
	var calls int32
	s := NewScheduler(30*time.Millisecond, func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}, nil, nil)

	s.Start(context.Background())
	defer s.Stop()

	time.Sleep(15 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("calls = %d at 15ms into a 30ms interval with no WithInitialDelay, want 0 (regression check: existing callers must be unaffected)", got)
	}
}

// ── Stop 是同步的：三個性質各一支斷言 ──────────────────────────────────────────

// Stop 必須等到 loop 的 goroutine 真的收尾才返回。
//
// 這支不靠時間長短判定，所以不隨 CPU 競爭飄：run 卡在一個由測試控制的 channel 上，
// 只要 Stop 在 run 還卡著的時候就回來了，就是「已經 cancel、但還有一次 tick 在飛」
// ——shutdown 時最難查的那種殘留（那一跳仍在寫 DB、仍在發 MQ，而程序其餘部分已經在
// 拆連線池）。TestScheduler_StopHaltsFurtherTicks 斷言的「Stop 之後不再有 tick」也是
// 靠這個性質才成立，否則它只是在賭排程器搶不到 CPU。
func TestScheduler_StopWaitsForTheLoopToFinish(t *testing.T) {
	release := make(chan struct{})
	var entered, finished atomic.Bool

	s := NewScheduler(time.Hour, func(_ context.Context) error {
		entered.Store(true)
		<-release
		finished.Store(true)
		return nil
	}, nil, nil, WithInitialDelay(0))

	s.Start(context.Background())
	deadline := time.Now().Add(2 * time.Second)
	for !entered.Load() {
		if time.Now().After(deadline) {
			t.Fatal("run 從未被呼叫，這支測試失去對象")
		}
		time.Sleep(time.Millisecond)
	}

	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()

	select {
	case <-stopped:
		t.Fatal("run 還卡在半路，Stop 就回來了 —— 關機之後仍有一次 tick 在飛")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("run 結束後 Stop 仍未返回")
	}
	if !finished.Load() {
		t.Fatal("Stop 返回時 run 尚未跑完")
	}
}

// Start 從未呼叫過也可以安全 Stop（fx 在 OnStart 失敗後仍會跑已註冊的 OnStop）。
func TestScheduler_StopWithoutStartIsSafe(t *testing.T) {
	s := NewScheduler(time.Hour, func(_ context.Context) error { return nil }, nil, nil)
	done := make(chan struct{})
	go func() { s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("未 Start 的 Stop 卡住了 —— nil channel 的讀取會永久阻塞，那個分支必須先返回")
	}
}

// 重複 Stop 不得 panic，也不得永久阻塞。
//
// 兩個風險都是實作細節直接決定的：close 已關閉的 channel 會 panic（所以 close 只能由
// goroutine 自己 defer 一次），而讀取已關閉的 channel 立刻返回（所以第二次 Stop 不會卡）。
func TestScheduler_StopIsIdempotent(t *testing.T) {
	var calls int32
	s := NewScheduler(10*time.Millisecond, func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}, nil, nil)

	s.Start(context.Background())
	time.Sleep(25 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Stop()
		s.Stop()
		s.Stop()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("重複 Stop 卡住了")
	}
}
