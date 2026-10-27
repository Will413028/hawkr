-- invoice_provider_cutovers｜自動開立切點歷史（端點 46）
-- SSOT：設計書 58 號 §5.1.15
--
-- 寫入只有兩個路徑，且都必須與 invoice_provider_accounts 的那一句 CAS 同一交易：
--   端點 26（賣家設定切點）  ⇒ CloseOpenProviderCutover ＋ InsertProviderCutover
--   端點 67（ADM 撤銷連線）  ⇒ CloseOpenProviderCutover（不新增列，§5.1.15）
-- 分兩個交易寫的後果是歷史與現值不一致，而端點 46 是唯一會讀到它的地方 ——
-- 沒有任何東西會發現。

-- name: CloseOpenProviderCutover :execrows
-- 關掉目前有效的那一列。冪等：沒有開著的列時 rows=0，呼叫端不得視為錯誤
-- （第一次設定切點時本來就沒有前一列）。
UPDATE invoice_provider_cutovers
SET deactivated_at = sqlc.arg(now)
WHERE provider_account_id = sqlc.arg(provider_account_id)
  AND deactivated_at IS NULL;

-- name: InsertProviderCutover :one
-- account_version 存的是**設定之後**的版本（SetProviderAutomationCutover 已 +1）：
-- ProviderCutoverItem.version 要能讓 ADM 對回 invoice_provider_accounts 當時那一版。
INSERT INTO invoice_provider_cutovers (
  provider_account_id, issuer_id, account_version, cutover_at,
  mode, created_by_account_id, reason, created_at)
VALUES (
  sqlc.arg(provider_account_id), sqlc.arg(issuer_id), sqlc.arg(account_version),
  sqlc.arg(cutover_at), 'AMEGO_AUTO', sqlc.arg(created_by_account_id),
  sqlc.narg(reason), sqlc.arg(now))
RETURNING *;

-- name: ListProviderCutoversForAdmin :many
-- 端點 46。以 provider_account_id 過濾（呼叫端先用 store_id 解析出帳戶，查無回 9330）。
-- 排序取 created_at DESC 並補 id DESC 當穩定第二鍵：同一交易內關舊列＋插新列會讓
-- 兩列的時間戳相同，只用時間排序的分頁會在頁界重複或漏列。
SELECT * FROM invoice_provider_cutovers
WHERE provider_account_id = sqlc.arg(provider_account_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountProviderCutoversForAdmin :one
SELECT COUNT(*)::bigint FROM invoice_provider_cutovers
WHERE provider_account_id = sqlc.arg(provider_account_id);
