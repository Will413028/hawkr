# Canonical sqlc query template for a new outbox table

Copy this into a new service's `db/queries/<name>_outbox.sql`, replacing
`<table>` and `<Table>` (PascalCase for generated Go names) throughout. This
is a template to copy, not shared code — sqlc compiles each service's SQL
independently, and the claim predicate legitimately varies per service (see
docs/開發規範/多副本排程搶佔規範.md rule ⑦: the predicate must be
non-PK and must mirror each table's own candidate query).

```sql
-- name: FetchPending<Table> :many
-- Deliberately NOT `FOR UPDATE SKIP LOCKED` — this runs under autocommit, so
-- any row lock it took would be released the instant the SELECT returns.
-- Multi-replica exclusivity comes from Claim<Table>, not this query.
SELECT * FROM <table>
WHERE status = 'pending' AND next_attempt_at <= $1
ORDER BY next_attempt_at
LIMIT $2;

-- name: Claim<Table> :execrows
-- Lease CAS: pushes next_attempt_at to lease_until. rows=1 ⇒ this replica
-- owns the row; rows=0 ⇒ another replica already claimed it, or it left the
-- candidate set. Zero schema change: next_attempt_at is already the
-- candidate predicate's time column, so reusing it as the lease costs
-- nothing extra. status='pending' is the required non-PK predicate (rule ⑦).
UPDATE <table>
SET next_attempt_at = sqlc.arg(lease_until)
WHERE id = sqlc.arg(id)
  AND status = 'pending'
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: UpdateAttempt<Table> :exec
UPDATE <table>
SET attempt_count = $2,
    next_attempt_at = $3,
    last_error = $4
WHERE id = $1;

-- name: MarkDone<Table> :exec
UPDATE <table>
SET status = 'done',
    completed_at = $2
WHERE id = $1;

-- name: MarkFailed<Table> :exec
-- Persist last_error here too — a table that only writes last_error on
-- non-terminal retries (via UpdateAttempt) and drops it on the terminal
-- transition loses exactly the error message an operator needs when the row
-- finally lands on 'failed'. See the account-service apple_revoke_outbox fix
-- this template is drawn from.
UPDATE <table>
SET status = 'failed',
    last_error = $2,
    completed_at = $3
WHERE id = $1;

-- name: CountPending<Table> :one
SELECT COUNT(*) FROM <table> WHERE status = 'pending';

-- name: OldestPending<Table>CreatedAt :one
-- Returns pgx.ErrNoRows when empty — deliberately not a nullable MIN()
-- aggregate, so the Go caller gets an unambiguous "no rows" signal instead of
-- having to special-case a NULL aggregate value.
SELECT created_at FROM <table>
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT 1;

-- name: DeleteDone<Table>Before :execrows
-- Postgres has no DELETE ... LIMIT; the subquery bounds one cleanup tick.
DELETE FROM <table>
WHERE id IN (
  SELECT sub.id FROM <table> AS sub
  WHERE sub.status = 'done' AND sub.completed_at < sqlc.arg(cutoff)
  LIMIT sqlc.arg(row_limit)
);
```

The inner `SELECT` must alias the table (`AS sub`) and qualify its columns
even though it self-references the same table as the outer `DELETE`: sqlc's
analyzer treats the subquery's `status`/`completed_at` columns as ambiguous
without the alias, even though plain Postgres resolves them fine unaliased.

## Wiring into pkg/outbox

1. Domain struct: add `OutboxID() int64` / `OutboxAttemptCount() int` methods
   (or equivalent) to satisfy `outbox.Row`.
2. Repository adapter: implement `outbox.Repository[T]` (`FetchPendingDue`,
   `Claim`, `MarkDone`, `MarkFailed`, `UpdateAttempt`) and `outbox.MonitorRepo`
   (`CountPending`, `OldestPendingAge`, `DeleteDoneBefore`) against the
   generated sqlc queries above.
3. Worker: write a `ProcessFunc[T]` closure with the service-specific side
   effect, and `outbox.Hooks[T]` for audit/metric side effects, then
   `outbox.New[T](repo, process, cfg, hooks)`.
4. Scheduler: `outbox.NewScheduler(interval, worker.RunOnce, onPanic, onError)`,
   wired into `fx.Lifecycle` the same way as account-service's
   `internal/applerevoke/scheduler.go`.
5. Monitoring: `outbox.NewMetrics("<name>")` as an `fx.Provide`'d singleton
   shared between the Worker's `Hooks.OnTerminalFailed` and a second
   `Scheduler` that calls `Metrics.Refresh` + `outbox.CleanupDone` — see
   `services/account-service/internal/applerevoke/monitoring.go`.

## Sizing Config

`ClaimLease` must exceed the worst-case time to process one row (side effect
+ any downstream RPC timeouts) — see
`docs/開發規範/多副本排程搶佔規範.md §三`. `MaxAttempts`/`BaseBackoff`/
`MaxBackoff`/`FetchBatch` are workload judgment calls, not derived from a
formula; account-service's apple_revoke_outbox uses `MaxAttempts=5`,
`BaseBackoff=1min`, `MaxBackoff=1hr`, `FetchBatch=50` as a reasonable
starting point for a low-volume, non-latency-sensitive outbox.
