-- name: CreatePushNotification :exec
INSERT INTO push_notifications (
  id, user_id, user_type, event_type, reference_id, status, error_log, sent_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: ExistsPushProcessed :one
-- Idempotency gate: has this logical notification already been processed for this
-- recipient? Keyed on identity (event_type, reference_id, user) and DECOUPLED from
-- delivery status — a redelivery must NOT re-push even if the prior attempt was
-- 'partial' / 'skipped' / 'failed'. (status='sent' was structurally unreachable
-- whenever any device token is dead, so the old gate never engaged → re-push storm.)
-- Mirrors the inbox ON CONFLICT (user_id, user_type, event_type, reference_id) semantics.
SELECT EXISTS(
  SELECT 1 FROM push_notifications
  WHERE event_type = $1 AND reference_id = $2 AND user_id = $3 AND user_type = $4
) AS exists;
