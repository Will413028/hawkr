-- issue #2446 契約凍結 Wave B：把既有的 error_code 值複製到新的 failure_reason 欄位。
--
-- 前一支 migration 的 `ADD COLUMN failure_reason TEXT NOT NULL DEFAULT ''` 會把既有列
-- 一律填成空字串，那對「本來就沒失敗」的列是對的，但對 error_code 非空的列會遺失原因。
-- 這一支補齊。
--
-- Data-only、idempotent：WHERE 條件確保重跑第二次匹配 0 列（第一次跑完之後兩欄已相等）。
-- 同 seller-service 20260714083918_1534_suspension_reason_code_name_to_value.sql 的寫法。
--
-- ⚠️ 這是 expand 階段：**error_code 不刪、不改名**。兩欄在整個過渡期並存，
-- 新版 code 雙寫兩欄，舊版 code 只寫 error_code——所以舊 pod 在 rolling update 期間
-- 仍然完全正常運作，這正是 docs/開發規範/DB Migration 相容性規範（expand-contract）.md
-- 要求的形狀。DROP COLUMN error_code 排在 contract 階段（Wave C）。
--
-- 實測（2026-08-19 查 DB）：dev 的 procurement_candidate_sets 共 88 列，error_code 全為
-- 空字串，所以這句實際會匹配 0 列；prod 該表 0 rows。仍然保留這一句——靠「現在的資料剛好
-- 沒有」來省略回填，是把資料現況當成不變式；reap 路徑（ReapExpiredProcurementCandidateSet）
-- 隨時可能寫入 WORKER_LOST，這支 migration 與那次寫入之間沒有任何順序保證。
UPDATE "procurement_candidate_sets"
SET "failure_reason" = "error_code"
WHERE "error_code" <> '' AND "failure_reason" = '';
