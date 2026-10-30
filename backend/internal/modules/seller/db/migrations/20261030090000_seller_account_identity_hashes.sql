-- 防詐用的身分比對索引：與 KYC 個資分開保存，不隨 184 天的個資清理刪除（Stripe 計畫 D3，ADR 2026-10-19-seller-kyc-payuni-consolidated 的 2026-10-30 修訂）。
-- 系統未上線，不回填既有申請的 kyc_identity_hashes（開發資料以 dev-seed 重建）。
-- Create "account_identity_hashes" table
CREATE TABLE "seller"."account_identity_hashes" (
  "source_store_id" bigint NOT NULL,
  "field" text NOT NULL,
  "key_id" text NOT NULL,
  "hash" bytea NOT NULL,
  "account_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("source_store_id", "field", "key_id", "hash"),
  CONSTRAINT "account_identity_hashes_source_store_id_fkey" FOREIGN KEY ("source_store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "account_identity_hashes_field_check" CHECK (field = ANY (ARRAY['ID_CARD_NO'::text, 'REP_ID_CARD_NO'::text, 'TAX_ID'::text, 'BANK_ACCOUNT'::text])),
  CONSTRAINT "account_identity_hashes_hash_check" CHECK (octet_length(hash) = 32),
  CONSTRAINT "account_identity_hashes_key_check" CHECK (key_id <> ''::text)
);
-- Create index "account_identity_hashes_account" to table: "account_identity_hashes"
CREATE INDEX "account_identity_hashes_account" ON "seller"."account_identity_hashes" ("account_id");
