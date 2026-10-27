-- =============================================
-- V9: 移除死表 refund_execute_tasks（#2228）
--
-- 依據：
--   1. 該表由 V1 建立，原規劃為「退款執行重試任務佇列」，但對應的執行器
--      從未實作 —— 全 repo（Java / Go / SQL / script）零讀寫 caller，
--      唯一殘留是 V1 的 DDL 與若干文件描述。
--   2. 退款執行現況走 aftersales → payment 同步 gRPC + MQ 事件鏈，
--      不存在 application-level 補償任務表（CLAUDE.md §22：
--      補償 / backfill / 漂移偵測不進 application code）。
--   3. dev 殘留 12 列為 2026-03 舊資料（bank / uid 等欄位實查全 NULL，
--      無 PII），prod 為 0 筆；刪除前已 pg_dump 備份留存。
--
-- 刻意不帶 CASCADE：本表無任何 FK 被他表引用（已以 pg_constraint 實查），
-- 若未來出現非預期依賴，寧可讓 migration 失敗曝光，也不要靜默連坐刪除。
-- 相關索引與 trigger 隨 DROP TABLE 一併移除，不需個別處理。
-- =============================================

DROP TABLE IF EXISTS refund_execute_tasks;
