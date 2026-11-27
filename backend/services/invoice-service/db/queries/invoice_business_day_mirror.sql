
-- name: DeleteAllBusinessDays :execrows
DELETE FROM invoice_business_day_mirror;

-- name: InsertBusinessDay :exec
INSERT INTO invoice_business_day_mirror (holiday_date, synced_at)
VALUES (sqlc.arg(holiday_date), sqlc.arg(synced_at));

-- name: UpsertBusinessDayCoverage :exec
INSERT INTO invoice_business_day_coverage (singleton, covered_from, covered_to, synced_at)
VALUES (true, sqlc.arg(covered_from), sqlc.arg(covered_to), sqlc.arg(synced_at))
ON CONFLICT (singleton) DO UPDATE
SET covered_from = EXCLUDED.covered_from,
    covered_to = EXCLUDED.covered_to,
    synced_at = EXCLUDED.synced_at;

-- name: SnapshotBusinessDayMirror :many
SELECT c.covered_from, c.covered_to, c.synced_at, h.holiday_date
FROM (SELECT 1) AS base
LEFT JOIN invoice_business_day_coverage c ON true
LEFT JOIN invoice_business_day_mirror h
  ON h.holiday_date >= sqlc.arg(from_date) AND h.holiday_date <= sqlc.arg(to_date)
ORDER BY h.holiday_date;
