-- name: RecordRejectedIdentity :one
INSERT INTO ingest_rejected_identity (
    source, crop_code, market_code, reason,
    occurrence_count, first_seen_at, last_seen_at, first_trade_date, last_trade_date
) VALUES (
    sqlc.arg(source)::text, sqlc.arg(crop_code)::text,
    sqlc.arg(market_code)::text, sqlc.arg(reason)::text,
    sqlc.arg(occurrences)::bigint, sqlc.arg(seen_at)::timestamptz,
    sqlc.arg(seen_at)::timestamptz, sqlc.arg(first_trade_date)::text,
    sqlc.arg(last_trade_date)::text
)
ON CONFLICT (source, crop_code, market_code, reason) DO UPDATE
  SET occurrence_count = ingest_rejected_identity.occurrence_count
                         + EXCLUDED.occurrence_count,
      last_seen_at     = EXCLUDED.last_seen_at,
      last_trade_date  = EXCLUDED.last_trade_date
RETURNING (ingest_rejected_identity.first_seen_at = sqlc.arg(seen_at)::timestamptz)
  AS is_new;
