-- name: EffectiveMeasures :many
-- 指定帳號此刻生效中的停權與終止措施。「措施生效中」的定義只有 SQL 函式 seller.effective_measures(at)（objects.sql），這裡只選它的結果：
-- 帳號是措施自己的 account_id，帳號任何一個 stint 上的措施都算（含已關閉的）；now 由呼叫方傳入（注入的 clock），不用 SQL 的 now()。
-- 函式內聯後帳號條件是 measures_enforced_suspension 的 Index Cond（帳號的未撤銷停權與終止，筆數是帳號自己的）。停權狀態的推導（stint.Derive：
-- 能力檢查、讀店家、開店與 suspension.Recompute）、批次的推導（stint.EffectiveMany）和身分快照的清理（stint.PruneIdentities）都經這一支；
-- 問全部的帳號是下一支 EffectiveMeasuresAll，曝光的 pub_exposable_stores(at) 也讀同一個函式。預定生效的是 ScheduledMeasureIDs（starts_at
-- 在未來），model.StateOf 是另一種寫法。一致由資料集測試鎖住：TestEffectiveMeasuresAndStateAgreeAtTheBoundaries（逐帳號的讀取與 StateOf
-- 在邊界時刻）與 TestEffectiveMeasuresPerAccountAllAccountsAndStateOfAgreeOnADataset（含已撤銷、警告、邊界、多版本、已結束、開放式 RISK、
-- 預告終止與永久終止的資料集，三者的生效中集合與人工標註相等）。
SELECT m.account_id, m.id AS measure_id, m.kind, m.termination_ground, m.basis, m.risk_ground, f.category AS lead_category
FROM seller.measures m
LEFT JOIN seller.violations f ON f.id = m.lead_finding_id
WHERE m.account_id = ANY(sqlc.arg(account_ids)::bigint[])
  AND m.id IN (
    SELECT e.id FROM seller.effective_measures(sqlc.arg(now)::timestamptz) e WHERE e.account_id = ANY(sqlc.arg(account_ids)::bigint[])
  )
ORDER BY m.account_id, m.created_at DESC, m.id DESC;

-- name: EffectiveMeasuresAll :many
-- 全部帳號此刻生效中的停權與終止措施（後台清單的停權篩選 stint.SuspendedAccounts、措施列表的 state 篩選 stint.ActiveMeasureIDs、
-- 每小時的對帳），定義是 seller.effective_measures(at)，與 EffectiveMeasures 同一個函式。讀取量不隨歷史的措施筆數成長：函式的 OR 兩邊各配
-- measures 上的一個部分索引（BitmapOr）、都是 Index Cond（有固定期間的用 measures_active_timed 範圍掃 ends_at > now，已結束的在範圍外、已撤銷的不在索引裡；
-- 沒有固定期間的用 measures_active_open，索引只含未撤銷也未結束的列，以 starts_at <= now 為 Index Cond）。
SELECT m.account_id, m.id AS measure_id, m.kind, m.termination_ground, m.basis, m.risk_ground, f.category AS lead_category
FROM seller.measures m
LEFT JOIN seller.violations f ON f.id = m.lead_finding_id
WHERE m.id IN (SELECT e.id FROM seller.effective_measures(sqlc.arg(now)::timestamptz) e)
ORDER BY m.account_id, m.created_at DESC, m.id DESC;

-- name: ScheduledMeasureIDs :many
-- 「預定生效」的定義：未撤銷的停權或終止合作措施，還沒開始（starts_at 晚於 now），而且 ends_at 為空或晚於 now（預告期間內因店家關閉而結束的，
-- ends_at 不晚於 now，是 ENDED，不在這裡）。account_id 為 NULL 時問全部的帳號。兩段各用 EffectiveMeasuresAll 的那兩個部分索引。措施列表的
-- SCHEDULED 篩選用它，ENDED 篩選以「不在生效中也不在預定生效」排除。
WITH timed AS (
  SELECT m.id
  FROM seller.measures m
  WHERE m.ends_at > sqlc.arg(now)::timestamptz
    AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.starts_at > sqlc.arg(now)::timestamptz
    AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id)::bigint)
), open_ended AS (
  SELECT m.id
  FROM seller.measures m
  WHERE m.ends_at IS NULL
    AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.starts_at > sqlc.arg(now)::timestamptz
    AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id)::bigint)
)
SELECT id::uuid AS id FROM (SELECT id FROM timed UNION ALL SELECT id FROM open_ended) e;

-- name: OpenSuspensionPeriods :many
-- 帳號各 stint 進行中的停權期間（通常最多一段；開店晚於措施結束、job 還沒結束舊期間時會同時有兩段）。
SELECT p.*
FROM seller.suspension_periods p
JOIN seller.stores st ON st.store_id = p.store_id
WHERE st.account_id = $1 AND p.ended_at IS NULL
ORDER BY p.store_id;

-- name: StartSuspensionPeriod :one
INSERT INTO seller.suspension_periods (store_id, started_at)
VALUES (sqlc.arg(store_id), sqlc.arg(now))
RETURNING *;

-- name: EndSuspensionPeriod :one
UPDATE seller.suspension_periods
SET ended_at = sqlc.arg(now)
WHERE period_id = sqlc.arg(period_id) AND ended_at IS NULL
RETURNING *;

-- 措施對帳的對象是「有進行中停權期間」（可能該結束）或「此刻有生效中停權措施」（可能該開始）的帳號：措施只會在寫入的
-- transaction 內開始生效，時間經過只會讓它失效，所以這就是全部可能與推導不一致的帳號。進行中的期間由這支查詢以 account_id
-- 分批（keyset）、只走 suspension_periods_open；生效中的措施由 EffectiveMeasuresAll（讀取量隨生效中的措施成長），
-- 由 suspension 套件合併。

-- name: AccountsWithOpenPeriod :many
SELECT DISTINCT st.account_id
FROM seller.suspension_periods p
JOIN seller.stores st ON st.store_id = p.store_id
WHERE p.ended_at IS NULL AND st.account_id > sqlc.arg(after_account_id)
ORDER BY st.account_id
LIMIT sqlc.arg(batch_size);
