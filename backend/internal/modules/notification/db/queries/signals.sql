-- name: NotifySignal :exec
-- 串流訊號：在呼叫端的 transaction 內 pg_notify，commit 後才送達（rollback 就不送）。payload 只有業務鍵，不含內容。
SELECT pg_notify(sqlc.arg(channel)::text, sqlc.arg(payload)::text);
