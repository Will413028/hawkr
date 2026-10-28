package outbox

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DailyScheduler runs fn once per day at a fixed UTC hour:minute, recovering
// panics so a bug in fn can never crash the host process. Unlike Scheduler's
// fixed-interval ticker, each cycle computes the next occurrence of
// hourUTC:minuteUTC and sleeps until then rather than firing on a regular
// interval — the shape every "run once a day at a fixed wall-clock time" cron
// job in this repo already hand-rolled independently (buyer-service
// and seller-service each had their own copy of this exact
// nextOccurrence/runDailyAt/runWithRecover trio before this type existed).
//
// fn does not return an error: none of the three call sites this type
// replaces ever propagated one — each handles its own failure via internal
// logging or an audit-trail write, matching how CLAUDE.md's transaction
// ownership rules already scope failure handling to the job itself, not a
// generic scheduler wrapper. If a future caller genuinely needs an onError
// hook, add it as an Option rather than changing this signature (mirrors how
// Scheduler's own onPanic/onError are separate from run's shape).
type DailyScheduler struct {
	hourUTC, minuteUTC int
	jobName            string
	run                func(ctx context.Context)
	logger             *zap.Logger

	mu     sync.Mutex // guards cancel — Start/Stop are not guaranteed to be called from the same goroutine (see Stop's doc comment)
	cancel context.CancelFunc
}

// NewDailyScheduler constructs a DailyScheduler. logger must not be nil —
// every cycle logs "cron next run scheduled" (info) before sleeping, and a
// panic in fn logs "cron job panicked" (error, with job/recovered/stack
// fields) — this is the only observability these jobs have today (no
// Prometheus metrics for any of the three call sites this type replaces),
// and matches the log shape already proven in seller-service's
// governance_jobs.go/di.go
// verbatim (field names "job"/"recovered"/"stack", debug.Stack() not
// zap.Stack()) so existing log-based alerting/dashboards keyed on those
// fields are unaffected by the migration.
func NewDailyScheduler(hourUTC, minuteUTC int, jobName string, run func(ctx context.Context), logger *zap.Logger) *DailyScheduler {
	return &DailyScheduler{hourUTC: hourUTC, minuteUTC: minuteUTC, jobName: jobName, run: run, logger: logger}
}

// Start begins the daily loop in a new goroutine. Pass context.Background(),
// NOT the ctx an fx.Lifecycle OnStart hook receives — see Scheduler.Start's
// doc comment for why (same fx start-timeout pitfall applies here). Calling
// Start twice without an intervening Stop leaks the first goroutine.
//
// Start and Stop may be called from different goroutines with no ordering
// guarantee between them (e.g. a caller that runs a slow startup step before
// calling Start from inside its own goroutine, while OnStop fires from fx's
// own goroutine) — the mutex below makes that safe. Without it, a Stop()
// racing a concurrent Start() would be a genuine data race on the cancel
// field (found via `go test -race` with a caller that runs
// `go func() { catchUp(ctx); Start(ctx) }()` while OnStop can fire
// concurrently).
func (s *DailyScheduler) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	go s.loop(runCtx)
}

// Stop cancels the running loop. Safe to call even if Start was never called,
// and safe to call concurrently with (or before) a Start() running in another
// goroutine — see Start's doc comment.
//
// ⚠️ 與 Scheduler.Stop 不同，這一支**不等** fn 收尾，而且那是刻意的：fn 是一天跑一次
// 的批次（預測、治理掃描…），等它結束等於讓 fx 的 OnStop 卡上好幾分鐘、逾時、把一次
// 正常關機報成失敗。fixed-interval 那支的每一跳是秒級的 outbox 派送，等得起。
// 兩者的差別在 tick 的耗時量級，不是在誰比較嚴謹——別把這裡「順手改成一致」。
func (s *DailyScheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *DailyScheduler) loop(ctx context.Context) {
	for {
		next := NextOccurrence(time.Now().UTC(), s.hourUTC, s.minuteUTC)
		s.logger.Info("cron next run scheduled", zap.String("job", s.jobName), zap.Time("at", next))

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}

		// Re-check after waking: select above picks a ready case at random
		// when both ctx.Done() and the timer are ready simultaneously (Stop()
		// racing the scheduled instant) — without this, the timer branch
		// could win and run fn against an already-cancelled ctx.
		if ctx.Err() != nil {
			return
		}
		s.tick(ctx)
	}
}

func (s *DailyScheduler) tick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("cron job panicked",
				zap.String("job", s.jobName),
				zap.Any("recovered", r),
				zap.ByteString("stack", debug.Stack()))
		}
	}()
	s.run(ctx)
}

// NextOccurrence returns the next UTC wall-clock time when hour:minute equals
// targetHourUTC:targetMinuteUTC. If that time today has already passed (or is
// exactly now), it returns tomorrow's occurrence.
func NextOccurrence(now time.Time, targetHourUTC, targetMinuteUTC int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), targetHourUTC, targetMinuteUTC, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
