-- ============================================================================
-- V13: idempotency_request 加上 (key, store_id) 唯一約束（#2306 階段 1）
--
-- 背景：主鍵是單欄 key，而 key 的內容前端送什麼就是什麼。三個 endpoint
--       （CreateProduct / UpdateProduct / CreateProductFromImportInternal）
--       共用同一個全域命名空間，連 endpoint 也不在鍵裡。於是兩個賣家用同一把
--       key 就會互撞，後到的那位在 24–48 小時內無法使用它。
--
--       這不是「機率極低」的理論問題：dev 實查 132 筆記錄裡 127 筆是批次上架
--       腳本產的短鍵（`b5-072009-9` 這種 `b<批次>-<時分秒>-<序號>`，最短 11 字），
--       格式本身不含任何主體資訊。兩個賣家同一天跑同一支腳本必撞。
--
-- 為什麼本階段只「加」不「換」：滾動更新期間新舊 pod 並存，舊 pod 仍執行
--       `ON CONFLICT (key)`。舊 PK 一旦在這裡 drop，那句 SQL 會直接報錯。
--       所以階段 1 讓新約束與舊 PK 並存、新 code 走新約束；等舊 pod 全退場後，
--       階段 2（V14）才 drop 舊 PK 並清空舊列。
--
--       ⚠️ 階段 1 期間舊 PK 仍在，跨租戶撞鍵**還是會被舊 PK 擋**——本階段不修好
--       那個症狀，它只是讓階段 2 可以零窗口完成。真正解除要等 V14。
-- ============================================================================
ALTER TABLE idempotency_request
  ADD CONSTRAINT uq_idempotency_key_store UNIQUE (key, store_id);

COMMENT ON CONSTRAINT uq_idempotency_key_store ON idempotency_request IS
  '冪等鍵的真正唯一性維度（#2306）：同一把 key 在不同賣家之間必須互不干擾。'
  '單欄 key 主鍵將於 V14 移除，屆時本約束成為唯一的衝突判定依據。';
