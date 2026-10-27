-- #2295 ①推論 3：target_seller_id 是 nullable，而 PostgreSQL 的 UNIQUE 視每個 NULL
-- 互異 —— 舊 index 直接把它放進鍵，於是「不指定賣家」的通報 (phone, NULL, week)
-- 可以無限重複，整條週上限對它完全失效。
-- dev 實測：NULL target 同 phone 同週插 3 筆全部成功；對照組（target 有值）第二筆
-- 才被擋。改用 COALESCE(target_seller_id, 0) 讓 NULL 也參與唯一性判定。
--
-- ⚠️ 前置清理是必要的，不是防禦性冗餘：新 index 比舊的嚴格，既有資料若已存在
-- 「同 phone + NULL target + 同週」多筆，CREATE UNIQUE INDEX 會直接失敗，而那是
-- 在部署當下才炸。prod 的這張表我查不到（監控用的 exporter 帳號對它 permission
-- denied），所以不能靠「應該沒有」來賭。
--
-- 刪的是違反新約束意圖的重複列，保留每組最早一筆（id 最小）——那筆才是使用者
-- 真正送出的第一則通報，後面的本來就該被週上限擋掉。無衝突時影響 0 列。
DELETE FROM food_safety_reports a
USING food_safety_reports b
WHERE a.target_seller_id IS NULL
  AND b.target_seller_id IS NULL
  AND a.reporter_phone = b.reporter_phone
  AND date_trunc('week', (a.created_at AT TIME ZONE 'UTC')) = date_trunc('week', (b.created_at AT TIME ZONE 'UTC'))
  AND a.id > b.id;

-- Drop index "uq_food_safety_reports_phone_seller_week" from table: "food_safety_reports"
DROP INDEX "uq_food_safety_reports_phone_seller_week";
-- Create index "uq_food_safety_reports_phone_seller_week" to table: "food_safety_reports"
CREATE UNIQUE INDEX "uq_food_safety_reports_phone_seller_week" ON "food_safety_reports" ("reporter_phone", (COALESCE(target_seller_id, (0)::bigint)), (date_trunc('week'::text, (created_at AT TIME ZONE 'UTC'::text))));
