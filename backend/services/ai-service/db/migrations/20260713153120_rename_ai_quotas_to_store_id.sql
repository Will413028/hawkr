-- Modify "ai_quotas" table
-- account_id → store_id 是純改名（Admin 額度上限改綁 store，account_id 只做登入身份查詢，
-- 不當業務表的 key）。手動改成 RENAME COLUMN，理由同 ai_usage 那次遷移：atlas migrate diff
-- 預設產出的 DROP+ADD 對已有資料的表會直接失敗。RENAME COLUMN 自動帶著 ai_quotas_pkey 走。
ALTER TABLE "ai_quotas" RENAME COLUMN "account_id" TO "store_id";
