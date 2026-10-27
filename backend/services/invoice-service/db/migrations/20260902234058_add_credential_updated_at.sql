-- Modify "invoice_provider_accounts" table
ALTER TABLE "invoice_provider_accounts" ADD COLUMN "credential_updated_at" timestamptz NULL;
-- 回填既有列（#3051）：憑證的真實設定時間查不回來，但這一列的 created_at 就是它
-- 第一次被寫入憑證的時刻（本表沒有「無憑證」的列——credential_ciphertext NOT NULL）。
-- 之後若曾輪替過，回填值會偏早；那比留 NULL 好，因為 NULL 在 UI 上會顯示成「—」，
-- 讓賣家以為自己從沒設定過。
--
-- 🔴 這是一次性歷史資料清整，走 migration 不進 application code（§22）：
-- 不建 backfill job、不建 reconciliation task。
UPDATE "invoice_provider_accounts"
   SET "credential_updated_at" = "created_at"
 WHERE "credential_updated_at" IS NULL;
