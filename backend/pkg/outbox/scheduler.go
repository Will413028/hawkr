package outbox

import (
	"context"
	"time"
)

// Scheduler runs a func(ctx) error on a fixed ticker in its own goroutine,
// recovering panics so a bug in run cannot crash the host process. It has no
// dependency on any specific DI framework — callers typically wire
// Start/Stop into their own process lifecycle hooks (e.g. fx.Lifecycle).
type Scheduler struct {
	interval        time.Duration
	run             func(ctx context.Context) error
	onPanic         func(ctx context.Context, recovered any)
	onError         func(ctx context.Context, err error)
	hasInitialDelay bool
	initialDelay    time.Duration
	cancel          context.CancelFunc
	// done 由 loop 的 goroutine 在收尾時 close，是 Stop 等待的對象。
	done chan struct{}
}

// Option configures optional Scheduler behavior beyond the required
// interval/run/onPanic/onError constructor arguments.
type Option func(*Scheduler)

// WithInitialDelay makes the first tick fire after d instead of waiting a
// full interval — d may be 0 for "run immediately on Start". Every
// subsequent tick still fires on the regular interval cadence. Use this
// instead of reintroducing a hand-rolled "run once now/soon, then every N"
// ticker loop (see notification-service's cleanup job, which needs a 1
// minute startup delay, and ai-service's session cleaner, which needs an
// immediate first pass).
func WithInitialDelay(d time.Duration) Option {
	return func(s *Scheduler) {
		s.hasInitialDelay = true
		s.initialDelay = d
	}
}

// NewScheduler constructs a Scheduler. onPanic and onError may be nil.
func NewScheduler(
	interval time.Duration,
	run func(ctx context.Context) error,
	onPanic func(ctx context.Context, recovered any),
	onError func(ctx context.Context, err error),
	opts ...Option,
) *Scheduler {
	s := &Scheduler{interval: interval, run: run, onPanic: onPanic, onError: onError}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start begins ticking in a new goroutine. Pass context.Background(), NOT
// the ctx an fx.Lifecycle OnStart hook receives — that is fx's own
// start-timeout context and is cancelled the instant OnStart returns, which
// would end this loop immediately with no error and no log. Calling Start
// twice without an intervening Stop leaks the first goroutine — callers own
// lifecycle ordering (a single fx.Lifecycle OnStart/OnStop pair, typically).
func (s *Scheduler) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	// done 先於 cancel 指派：Stop 以 cancel != nil 判斷「Start 跑過了」，兩者的順序反過來
	// 會讓那個判斷在極短窗口內成立而 done 還是 nil。
	done := make(chan struct{})
	s.done = done
	s.cancel = cancel
	go func() {
		// 關 goroutine 自己持有的那個 channel，不是 s.done：Start 被呼叫兩次（本 doc
		// 上方註明會洩漏第一個 goroutine）時，第一個 goroutine 不該去關第二個的。
		defer close(done)
		s.loop(runCtx)
	}()
}

// Stop cancels the running loop and waits for it to finish. Safe to call even
// if Start was never called, and safe to call more than once.
//
// 它是同步的（cancel 之後等 goroutine 收尾），因為全部呼叫端都掛在 fx.Lifecycle 的
// OnStop 上——那個位置要的就是 graceful shutdown。只 cancel 不等，會留下「已經
// cancel、但還有一次 tick 在飛」的殘留：那一次 tick 仍在寫 DB、仍在發 MQ，而
// 程序的其餘部分已經在拆連線池了，症狀是關機時偶發的一批看不懂的錯誤。
//
// ⚠️ 責任邊界：run 若卡住不返回，Stop 會跟著卡住——這裡刻意不自己加 timeout。呼叫端
// 是 fx 的 OnStop hook，fx 本來就有 stop timeout 並會回報是哪一個 hook 逾時；在這裡
// 再加一層會讓「run 卡住」被吞成一次看似正常的關機，反而更難查。run 的耗時上界屬於
// run 自己的責任（它拿得到 ctx）。
func (s *Scheduler) Stop() {
	if s.cancel == nil {
		return
	}
	s.cancel()
	// 已 close 的 channel 讀取立刻返回 ⇒ 重複 Stop 不會 panic 也不會永久阻塞。
	<-s.done
}

func (s *Scheduler) loop(ctx context.Context) {
	if s.hasInitialDelay {
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.initialDelay):
		}
		s.tick(ctx)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil && s.onPanic != nil {
			s.onPanic(ctx, r)
		}
	}()
	if err := s.run(ctx); err != nil && s.onError != nil {
		s.onError(ctx, err)
	}
}
