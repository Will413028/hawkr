-- name: ClaimIdempotencyKey :one
-- 認領一把冪等鍵。**與業務寫入同一個 transaction**（#3015 裁決 A）。
--
-- 🔴 用 `ON CONFLICT DO NOTHING` ＋ 回讀，而不是「先 SELECT 再 INSERT」：
-- 後者在併發下擋不住（兩個請求同時查不到、同時插入），而唯一鍵是這裡唯一
-- 真正生效的守衛。同 invoice_correction_requests 的序列閘門那條理由。
--
-- 回傳的是**最終在表上的那一列**（不論本次有沒有插進去）：
--   claimed = true  ⇒ 這一次是第一次，呼叫端往下做業務寫入
--   claimed = false ⇒ 已經有人認領過，看 request_hash 決定回放還是 9350
WITH ins AS (
  INSERT INTO invoice_idempotency_keys (scope, actor_id, idem_key, request_hash, created_at)
  VALUES (sqlc.arg(scope), sqlc.arg(actor_id), sqlc.arg(idem_key), sqlc.arg(request_hash), sqlc.arg(now))
  ON CONFLICT (scope, actor_id, idem_key) DO NOTHING
  RETURNING scope, actor_id, idem_key, request_hash, result_ref, created_at
)
SELECT scope, actor_id, idem_key, request_hash, result_ref, created_at, true AS claimed FROM ins
UNION ALL
SELECT scope, actor_id, idem_key, request_hash, result_ref, created_at, false AS claimed
  FROM invoice_idempotency_keys
 WHERE scope = sqlc.arg(scope) AND actor_id = sqlc.arg(actor_id) AND idem_key = sqlc.arg(idem_key)
   AND NOT EXISTS (SELECT 1 FROM ins)
LIMIT 1;

-- name: SetIdempotencyResult :execrows
-- 業務寫入成功後把結果主鍵補上，供之後的重送回放。
-- 同一個 transaction 內執行 ⇒ 業務寫入 rollback 時這一列也會一起消失。
UPDATE invoice_idempotency_keys
   SET result_ref = sqlc.arg(result_ref)
 WHERE scope = sqlc.arg(scope) AND actor_id = sqlc.arg(actor_id) AND idem_key = sqlc.arg(idem_key);
