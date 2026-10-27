-- invoice_business_day_mirror｜台灣國定假日唯讀鏡像
-- SSOT：設計書 58 號 §5.1.14
--
-- 這張表**不是行事曆的 owner，是本地快取**（owner ＝ shipping-service 的 holidays 表）。
-- 每日由 INV.gov 呼叫 SHIP GetHolidaysInternal(from_date, to_date) 同步「今天 −30 天 ～ +400 天」，
-- **同一交易內整段 DELETE + INSERT**，失敗即整段不覆蓋（§5.2.3 第 12 組，本身即原子）。
-- 禁止人工寫入、禁止 ADM 編輯、禁止在本服務加 seed migration。
--
-- 要鏡像而不是每次現打：K01 建案是買家的**同步 HTTP 路徑**，invoice_complaints.sla_due_at 是
-- NOT NULL、必須當場算出來 ⇒ 掛在 shipping-service 的可用性上，shipping 一抖買家就送不出申訴。
-- 同步連續失敗 ⇒ 鏡像變舊 ⇒ 未涵蓋日期一律視為假日（偏晚）＋ invoice_business_calendar_stale 告警。

-- name: DeleteAllBusinessDays :execrows
-- 整段覆蓋的前半。必須與 InsertBusinessDay 在同一交易內。
DELETE FROM invoice_business_day_mirror;

-- name: InsertBusinessDay :exec
-- 整段覆蓋的後半。同一批的每一列 synced_at 相同（欄位註解逐字要求）。
INSERT INTO invoice_business_day_mirror (holiday_date, synced_at)
VALUES (sqlc.arg(holiday_date), sqlc.arg(synced_at));

-- name: ListBusinessDayHolidays :many
-- 給 pkg/bizcalendar.AddBusinessDays 的純函式吃（它不碰 DB、不碰網路）。
SELECT * FROM invoice_business_day_mirror
WHERE holiday_date >= sqlc.arg(from_date) AND holiday_date <= sqlc.arg(to_date)
ORDER BY holiday_date;

-- name: GetBusinessDayMirrorSyncedAt :one
-- 鏡像新鮮度（invoice_business_calendar_stale 告警的資料面）。空表回 pgx.ErrNoRows。
SELECT synced_at FROM invoice_business_day_mirror ORDER BY synced_at DESC LIMIT 1;
