-- invoice_manual_unlocks｜A4 人工出口的雙人解鎖（端點 54／55）
-- SSOT：設計書 58 號 §5.1.16
--
-- 三條貫穿本檔的規則：
--
--   ① **雙人守衛在 WHERE，不在應用層的 if**。兩位管理員是不同的 HTTP session、
--      不同的時間點，應用層先讀再判的窗口擋不住「兩個分頁同時送出」。
--   ② **逾期不由排程器收斂**。EXPIRED 是時間函數不是事件；任何掃描它的 worker 都
--      要嘛是永久旗標、要嘛是沒有搶佔的裸迴圈（《多副本排程搶佔規範》）。本檔的
--      收斂點是 ExpireStaleManualUnlocks，掛在端點 54 的請求路徑上。
--   ③ **每一句寫入都是 CAS**，rows=0 一律由呼叫端回明確業務碼，絕不當成功往下走。

-- name: ExpireStaleManualUnlocks :execrows
-- 把該 job 底下已過期而仍掛著的解鎖請求就地轉 EXPIRED。
--
-- 🔴 它不是「順手清理」，而是 uq_invmu_open_per_operation 的名額釋放：少了這一句，
--    一個 job 逾期一次之後就**永遠再也發起不了第二次解鎖**（唯一索引擋住），而症狀是
--    端點 54 回 23505 而不是任何業務碼。58A §A06 明寫 EXPIRED 之後「可重新發起」。
-- ⚠️ approved_by_account_id／decided_at 保持 NULL —— chk_invmu_decided_pair 要求
--    非 APPROVED／REJECTED 的列不得掛第二人。
UPDATE invoice_manual_unlocks
SET status = 'EXPIRED',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE operation_id = sqlc.arg(operation_id)
  AND status = 'AWAITING_SECOND_APPROVAL'
  AND expires_at <= sqlc.arg(now);

-- name: InsertManualUnlock :one
-- 端點 54。expires_at 由呼叫端算（requested_at + 24h，§6.1.14 定版），不寫在 SQL：
-- 時間來源必須是可注入的 clock，寫成 now() + interval 會讓測試無法造出「逾期」那一格。
-- cancel_evidence_* 服務 J28（#3089 A 組第 2 項）：只有從 CANCEL_BLOCKED_UNKNOWN
--   發起的解鎖要填，從 CANCELLED_BY_ADMIN 進來的傳 NULL（那條路的取消已在 J21→J24
--   完成過查證）。⚠️ 覆核者要在端點 55 看得到它——沒有它，第二個人是在對一個
--   看不見證據的請求按同意。
INSERT INTO invoice_manual_unlocks (
  id, operation_id, status, requested_by_account_id, request_reason,
  provider_check_evidence, provider_checked_at, requested_at, expires_at,
  cancel_evidence_kind, cancel_evidence_object_key,
  version, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(operation_id), 'AWAITING_SECOND_APPROVAL',
  sqlc.arg(requested_by_account_id), sqlc.arg(request_reason),
  sqlc.arg(provider_check_evidence), sqlc.arg(provider_checked_at),
  sqlc.arg(now), sqlc.arg(expires_at),
  sqlc.narg(cancel_evidence_kind), sqlc.narg(cancel_evidence_object_key),
  0, sqlc.arg(now), sqlc.arg(now))
RETURNING *;

-- name: GetManualUnlockByID :one
SELECT * FROM invoice_manual_unlocks WHERE id = sqlc.arg(id);

-- name: GetOpenManualUnlockByOperation :one
-- 「現在還開著」＝ 尚未決策**且**尚未逾期。兩個條件缺一不可：只看 status 會把逾期的
-- 那一列當成開著的，端點 54 於是回 9344 叫 ADM 去核准一個核准不了的請求。
SELECT * FROM invoice_manual_unlocks
WHERE operation_id = sqlc.arg(operation_id)
  AND status = 'AWAITING_SECOND_APPROVAL'
  AND expires_at > sqlc.arg(now);

-- name: GetLatestManualUnlockByOperation :one
-- 端點 51 的 manualUnlock 卡：不管什麼狀態，回最近的那一列（null ＝ 從未發起過）。
SELECT * FROM invoice_manual_unlocks
WHERE operation_id = sqlc.arg(operation_id)
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: DecideManualUnlock :execrows
-- 端點 55（核准或駁回）。四個守衛全部在 WHERE：
--
--   status = 'AWAITING_SECOND_APPROVAL'        —— 已決策／已逾期的列不得被再決策一次
--   expires_at > now                           —— 24 小時窗口；逾期的列**不是**可核准的列
--   requested_by_account_id <> approver        —— 🔴 雙人確認，9345 的唯一守衛
--   version = expected_version                 —— 樂觀鎖
--
-- ⚠️ `<>` 的 NULL 陷阱不存在：requested_by_account_id 是 NOT NULL。
-- ⚠️ decision_reason 對 REJECTED 是必填（chk_invmu_reject_reason 兜底），APPROVED 可空。
UPDATE invoice_manual_unlocks
SET status = sqlc.arg(next_status),
    approved_by_account_id = sqlc.arg(approved_by_account_id),
    decision_reason = sqlc.narg(decision_reason),
    decided_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND operation_id = sqlc.arg(operation_id)
  AND status = 'AWAITING_SECOND_APPROVAL'
  AND expires_at > sqlc.arg(now)
  AND requested_by_account_id <> sqlc.arg(approved_by_account_id)
  AND version = sqlc.arg(expected_version);
