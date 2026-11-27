-- name: NotifySignal :exec
SELECT pg_notify(sqlc.arg(channel)::text, sqlc.arg(payload)::text);
