-- invoice_business_day_mirror｜台灣國定假日唯讀鏡像
-- SSOT：設計書 58 號 §5.1.14
--
-- 這張表**不是行事曆的 owner，是本地快取**（owner ＝ 單體的 platform/bizcalendar）。
-- 單體在每次匯入成功後與每天一次的對帳呼叫 ReplaceBusinessDayMirror，帶入涵蓋區間與區間內的假日；
-- **同一交易內整段 DELETE + INSERT 並覆蓋涵蓋區間**，失敗即整段不覆蓋（本身即原子）。
-- 禁止人工寫入、禁止 ADM 編輯、禁止在本服務加 seed migration。這支推送隨 A5 搬遷刪除。
--
-- 要鏡像而不是每次現打：K01 建案是買家的**同步 HTTP 路徑**，invoice_complaints.sla_due_at 是
-- NOT NULL、必須當場算出來 ⇒ 不能掛在單體的可用性上。
-- 推送連續失敗 ⇒ 鏡像變舊 ⇒ 未涵蓋日期一律視為假日（偏晚）＋ invoice_business_calendar_stale 告警。

-- name: DeleteAllBusinessDays :execrows
-- 整段覆蓋的前半。必須與 InsertBusinessDay 在同一交易內。
DELETE FROM invoice_business_day_mirror;

-- name: InsertBusinessDay :exec
-- 整段覆蓋的後半。同一批的每一列 synced_at 相同。
INSERT INTO invoice_business_day_mirror (holiday_date, synced_at)
VALUES (sqlc.arg(holiday_date), sqlc.arg(synced_at));

-- name: UpsertBusinessDayCoverage :exec
-- 覆蓋涵蓋區間與推送時間。必須與 DeleteAllBusinessDays、InsertBusinessDay 在同一交易內。
INSERT INTO invoice_business_day_coverage (singleton, covered_from, covered_to, synced_at)
VALUES (true, sqlc.arg(covered_from), sqlc.arg(covered_to), sqlc.arg(synced_at))
ON CONFLICT (singleton) DO UPDATE
SET covered_from = EXCLUDED.covered_from,
    covered_to = EXCLUDED.covered_to,
    synced_at = EXCLUDED.synced_at;

-- name: SnapshotBusinessDayMirror :many
-- 涵蓋區間（含新鮮度）與區間內的假日，一個陳述式讀完：單一陳述式在 READ COMMITTED 下
-- 也只看到同一份快照，不會拿到推送前後混合的結果。
-- 沒有涵蓋列（從未推送）仍回一列，三個涵蓋欄位為 NULL；區間內沒有假日時 holiday_date 為 NULL。
SELECT c.covered_from, c.covered_to, c.synced_at, h.holiday_date
FROM (SELECT 1) AS base
LEFT JOIN invoice_business_day_coverage c ON true
LEFT JOIN invoice_business_day_mirror h
  ON h.holiday_date >= sqlc.arg(from_date) AND h.holiday_date <= sqlc.arg(to_date)
ORDER BY h.holiday_date;
