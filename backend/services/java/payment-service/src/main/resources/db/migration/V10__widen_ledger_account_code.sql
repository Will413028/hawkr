-- =============================================================================
-- V10：放寬 ledger_entries.account_code 的欄寬
-- =============================================================================
--
-- 🔴 V9 把這一欄定為 varchar(32)，而 CodeMeta 的 LedgerAccountCode 有一個值是 33 字元
--    （PLATFORM_LOGISTICS_SERVICE_INCOME）⇒ 寫入時 22001 value too long。
--
-- 它不是理論風險：LogisticsLedgerBuilders 有 4 處產生這個碼值，其中包含
-- LOGISTICS_RECEIVABLE_ACCRUAL 的**貸方科目**——線 B 的第一筆帳本交易。
-- 本波零消費者所以現在碰不到，接上 worker 的那一刻就會炸。
--
-- 實測（真 PostgreSQL 17，附對照組）：
--   'PLATFORM_COLLECTION_FEE_EXPENSE'   (31 字元) → INSERT 0 1   ← 對照組，證明語句形狀有效
--   'PLATFORM_LOGISTICS_SERVICE_INCOME' (33 字元) → ERROR 22001
--
-- 🔴 修的是欄寬不是碼值。把碼值改短是跨 repo 的破壞性變更（CodeMeta 是四份前端拷貝的
--    SSOT，website／ADM 逐位元比對、兩個 Flutter 走語意抽取），而且那個名字本身是對的。
--
-- ⚠️ V9 已 commit ⇒ 不得修改（CLAUDE.md §11 Flyway append-only）。
--
-- 為什麼是 64 而不是剛好 33：次長的值是 31，只差一格。留餘裕讓下一個碼值不必再開一支
-- migration；varchar 在 PostgreSQL 是變長儲存，宣告上限不影響實際佔用。
-- =============================================================================

ALTER TABLE "ledger_entries" ALTER COLUMN "account_code" TYPE varchar(64);

-- V9 的欄註解同時把值域數量寫錯了（寫 13，實際 18——ADR-010 加了 5 個「應收」側的科目
-- 之後沒有回灌）。一併更正，否則下一個讀它的人會以為值域比實際小。
COMMENT ON COLUMN "ledger_entries"."account_code" IS
  'CodeMeta LedgerAccountCode（18 值）。欄寬 64：最長的值是 33 字元，次長 31，留餘裕。';
