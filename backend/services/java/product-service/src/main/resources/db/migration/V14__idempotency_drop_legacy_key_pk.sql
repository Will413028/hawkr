-- ============================================================================
-- V14: 移除 idempotency_request 的單欄 key 主鍵（#2306 階段 2）
--
-- 前提：V13 已上線並部署完成，所有 pod 都跑在「tryInsert 不指名任何約束、
--       查詢/更新/刪除一律帶 store_id」的版本上。在那之前執行本檔會讓舊 pod
--       的 `ON CONFLICT (key)` 找不到約束而對每一次 INSERT 報錯。
--
-- 為什麼不需要清空既有資料：舊 PK 保證 key 全域唯一，因此現有每一筆的
--       (key, store_id) 必然也唯一 ⇒ 新的唯一性約束對既有資料完全相容。
--       （#2306 原文寫「必須清空舊列」是未經驗證的推論，已在票內更正。）
--
-- 三句話在同一個 migration 的交易內完成，中間不存在「沒有唯一保證」的窗口。
-- 先 drop 兩個既有約束再建主鍵，是為了不留下與主鍵索引重複的第二份索引：
-- V13 的 UNIQUE 是以 constraint 形式建立的，PostgreSQL 不允許用它當
-- `ADD PRIMARY KEY USING INDEX` 的來源。
-- ============================================================================
ALTER TABLE idempotency_request DROP CONSTRAINT idempotency_request_pkey;

ALTER TABLE idempotency_request DROP CONSTRAINT uq_idempotency_key_store;

ALTER TABLE idempotency_request ADD PRIMARY KEY (key, store_id);

COMMENT ON TABLE idempotency_request IS
  '商品寫入的冪等記錄。主鍵 (key, store_id)：key 由前端提供、不保證跨賣家唯一，'
  '因此唯一性必須帶上賣家維度（#2306）。記錄壽命上限 24-48 小時，由 '
  'IdempotencyCleanupJob 每日 02:00 清理。';
