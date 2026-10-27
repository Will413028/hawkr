// Package outbox provides a generic, multi-replica-safe polling worker for
// the Transactional Outbox pattern: claim a due row with a CAS lease, run a
// caller-supplied side effect, then mark the row done or retry it with
// exponential backoff. See docs/開發規範/多副本排程搶佔規範.md for the
// design this implements — Claim is the preemption gate (rule ①/③/⑤/⑦),
// and Worker never uses FOR UPDATE SKIP LOCKED (rule ②: it protects nothing
// once the candidate SELECT runs under autocommit).
package outbox

import (
	"context"
	"fmt"
	"time"
)

// Row is the minimal shape Worker needs from any outbox row type.
type Row interface {
	OutboxID() int64
	OutboxAttemptCount() int
}

// Repository is implemented per service against its own sqlc-generated
// queries. Claim is the multi-replica preemption gate: it must push
// next_attempt_at (or an equivalent lease column) to leaseUntil with a CAS
// whose predicate mirrors FetchPendingDue's candidate predicate, and return
// the number of rows affected — 1 means this replica now owns the row, 0
// means another replica already claimed it or the row left the candidate set
// (e.g. concurrently marked done/failed).
type Repository[T Row] interface {
	FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]T, error)
	Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error)
	MarkDone(ctx context.Context, id int64, now time.Time) error
	MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error
	UpdateAttempt(ctx context.Context, id int64, attemptCount int, nextAttemptAt time.Time, lastError string) error
}

// ProcessFunc performs a claimed row's side effect. result is opaque to
// Worker and is forwarded verbatim to Hooks.OnDone — this lets a caller carry
// business data (e.g. a human-readable outcome note) from Process into its
// own success hook without Worker needing to know that data's shape.
type ProcessFunc[T Row] func(ctx context.Context, row T) (result any, err error)

// Hooks lets callers observe Worker outcomes (metrics, audit logs) without
// Worker depending on any specific observability stack. All fields are
// optional (nil is checked before every call).
type Hooks[T Row] struct {
	// OnDone fires once, after MarkDone has succeeded for a row whose
	// Process call completed without error.
	OnDone func(ctx context.Context, row T, result any)
	// OnAttemptFailed fires for every row that did not reach OnDone this tick:
	// a Claim error, or any reason bumpAttempt was invoked (Process error,
	// MarkDone error, or terminal failure). It does NOT fire when Claim
	// simply lost the race (rows affected = 0) — this replica never
	// attempted that row.
	//
	// cause identifies which call failed: "claim", "process", "mark_done", or
	// "update_attempt". err is that call's error. It normally fires once per
	// row per tick, but fires a SECOND time with cause="update_attempt" when
	// the row's own retry-bookkeeping write (attempt_count/next_attempt_at)
	// also fails — a real, independent failure (previously silently
	// discarded) that leaves the row without a backoff and so due for
	// reclaim on the very next tick instead of after its intended delay.
	OnAttemptFailed func(ctx context.Context, row T, cause string, err error)
	// OnTerminalFailed fires when a row's attempt count reaches
	// Config.MaxAttempts. markFailedErr is nil when the MarkFailed write
	// itself succeeded; non-nil means the row is still 'pending' in the DB
	// and MarkFailed will be retried next tick — callers typically only bump
	// their own terminal-failure counters when markFailedErr == nil, mirroring
	// MarkFailed's own success.
	OnTerminalFailed func(ctx context.Context, row T, newAttempt int, lastErr string, markFailedErr error)
}

// Config holds the retry/backoff/batch tuning for a Worker. See
// docs/開發規範/多副本排程搶佔規範.md §三 for how to size ClaimLease.
type Config struct {
	// MaxAttempts is the cap; once a row's attempt count would reach this
	// value, it is terminally failed instead of retried. The first call is
	// attempt 1.
	MaxAttempts int
	// BaseBackoff is the unit interval; retry delay = 2^attempt * BaseBackoff.
	BaseBackoff time.Duration
	// MaxBackoff caps the per-retry delay regardless of attempt count.
	MaxBackoff time.Duration
	// FetchBatch limits how many rows RunOnce pulls per pass.
	FetchBatch int
	// ClaimLease is how far into the future a successful claim pushes the
	// row's lease. Must exceed the worst-case time to process one row.
	ClaimLease time.Duration
}

// Worker drains due outbox rows for one aggregate type T.
type Worker[T Row] struct {
	repo    Repository[T]
	process ProcessFunc[T]
	cfg     Config
	hooks   Hooks[T]
}

// New constructs a Worker. repo and process are required; cfg should be
// tuned per docs/開發規範/多副本排程搶佔規範.md; hooks is optional (its
// zero value disables all observability callbacks).
func New[T Row](repo Repository[T], process ProcessFunc[T], cfg Config, hooks Hooks[T]) *Worker[T] {
	return &Worker[T]{repo: repo, process: process, cfg: cfg, hooks: hooks}
}

// RunOnce processes one batch of pending+due outbox rows and returns.
// Callers (typically a Scheduler) invoke this on a fixed cadence.
func (w *Worker[T]) RunOnce(ctx context.Context) error {
	now := time.Now()
	rows, err := w.repo.FetchPendingDue(ctx, now, w.cfg.FetchBatch)
	if err != nil {
		return fmt.Errorf("fetch pending: %w", err)
	}
	for _, row := range rows {
		w.processRow(ctx, row, now)
	}
	return nil
}

// processRow claims row, then runs its side effect. tickNow is the batch's
// start instant and is only used as the CAS upper bound (mirroring the
// candidate predicate) — the lease baseline itself is taken per row, not per
// tick, because sharing one baseline would let later rows in a large batch
// inherit an already-eroded effective lease (docs/開發規範/多副本排程搶佔規範.md
// §三 第 6 點).
func (w *Worker[T]) processRow(ctx context.Context, row T, tickNow time.Time) {
	claimed, err := w.repo.Claim(ctx, row.OutboxID(), time.Now().Add(w.cfg.ClaimLease), tickNow)
	if err != nil {
		w.fireAttemptFailed(ctx, row, "claim", err)
		return
	}
	if claimed == 0 {
		return
	}

	result, err := w.process(ctx, row)
	if err != nil {
		w.bumpAttempt(ctx, row, "process", err)
		return
	}

	if err := w.repo.MarkDone(ctx, row.OutboxID(), time.Now()); err != nil {
		w.bumpAttempt(ctx, row, "mark_done", err)
		return
	}
	if w.hooks.OnDone != nil {
		w.hooks.OnDone(ctx, row, result)
	}
}

// bumpAttempt increments the attempt count and either schedules the next
// retry with exponential backoff, or terminally fails the row once
// Config.MaxAttempts is reached. cause/causeErr identify which call in
// processRow triggered this (Process or MarkDone) and are forwarded to
// OnAttemptFailed.
func (w *Worker[T]) bumpAttempt(ctx context.Context, row T, cause string, causeErr error) {
	lastErr := causeErr.Error()
	if cause == "mark_done" {
		lastErr = "mark_done: " + lastErr
	}
	w.fireAttemptFailed(ctx, row, cause, causeErr)

	newAttempt := row.OutboxAttemptCount() + 1
	now := time.Now()

	if newAttempt >= w.cfg.MaxAttempts {
		markErr := w.repo.MarkFailed(ctx, row.OutboxID(), lastErr, now)
		if w.hooks.OnTerminalFailed != nil {
			w.hooks.OnTerminalFailed(ctx, row, newAttempt, lastErr, markErr)
		}
		return
	}

	delay := backoffFor(newAttempt, w.cfg.BaseBackoff, w.cfg.MaxBackoff)
	if err := w.repo.UpdateAttempt(ctx, row.OutboxID(), newAttempt, now.Add(delay), lastErr); err != nil {
		// This write is the row's own retry bookkeeping. Failing it leaves
		// the row at its previous next_attempt_at, so the next tick reclaims
		// it immediately instead of after backoff — a distinct, previously
		// invisible failure mode, so it gets its own OnAttemptFailed fire
		// rather than being folded into cause's own report above.
		w.fireAttemptFailed(ctx, row, "update_attempt", err)
	}
}

func (w *Worker[T]) fireAttemptFailed(ctx context.Context, row T, cause string, err error) {
	if w.hooks.OnAttemptFailed != nil {
		w.hooks.OnAttemptFailed(ctx, row, cause, err)
	}
}

// backoffFor returns 2^attempt * base, capped at max.
func backoffFor(attempt int, base, max time.Duration) time.Duration {
	d := base
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= max {
			return max
		}
	}
	return d
}
