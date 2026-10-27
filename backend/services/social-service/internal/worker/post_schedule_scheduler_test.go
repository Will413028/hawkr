package worker

import (
	"context"
	"runtime/debug"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestPostSchedulePublisher_SurvivesPanic 鎖住換掉裸 ticker 迴圈的那個理由。
//
// 原本的 run() 是 `for { select { case <-ticker.C: p.tick(ctx) } }`：tick 一旦
// panic，goroutine 就地死掉，之後每一則排程貼文永不發布——而且**沒有任何訊號**。
// 服務仍然健康、/metrics 照常、log 只有那一次 stack，逾期告警要等 24 小時候選
// 窗口過完才響，那時已經沒有人會把它連到「worker 幾天前死了」。
//
// 這支測試證明第一個 tick panic 之後，第二個 tick 仍然發生。它驗的是 di.go 選
// 用的那條路徑（outbox.Scheduler + onPanic），不是 tick 本身的邏輯。
func TestPostSchedulePublisher_SurvivesPanic(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	var ticks int32
	s := outbox.NewScheduler(
		10*time.Millisecond,
		func(_ context.Context) error {
			if atomic.AddInt32(&ticks, 1) == 1 {
				panic("first tick blows up")
			}
			return nil
		},
		func(_ context.Context, r any) {
			logger.Error("scheduled post publisher panicked",
				zap.Any("recovered", r), zap.ByteString("stack", debug.Stack()))
		},
		nil,
	)

	s.Start(context.Background())
	defer s.Stop()

	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&ticks) < 2 {
		select {
		case <-deadline:
			t.Fatalf("panic 之後沒有第二個 tick（ticks=%d）—— worker 已經死了，"+
				"而這正是不能用裸 ticker 迴圈的原因", atomic.LoadInt32(&ticks))
		case <-time.After(5 * time.Millisecond):
		}
	}

	if got := logs.FilterMessage("scheduled post publisher panicked").Len(); got == 0 {
		t.Error("panic 被吞掉了：沒有任何 error log —— 迴圈活著但沒人知道發生過什麼，" +
			"比直接死掉更難查")
	}
}
