-- 申請的身分比對索引（每個識別欄位、金鑰環每一把金鑰各一列的 blind index）：既有的申請以已存的 *_hash 與 *_hash_key 回填一列，
-- 其他金鑰的列由 RehashKYCIdentities 在啟動時補算。
-- Create "kyc_identity_hashes" table
CREATE TABLE "seller"."kyc_identity_hashes" (
  "application_id" uuid NOT NULL,
  "field" text NOT NULL,
  "key_id" text NOT NULL,
  "hash" bytea NOT NULL,
  PRIMARY KEY ("application_id", "field", "key_id"),
  CONSTRAINT "kyc_identity_hashes_application_id_fkey" FOREIGN KEY ("application_id") REFERENCES "seller"."kyc_applications" ("application_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "kyc_identity_hashes_field_check" CHECK (field = ANY (ARRAY['ID_CARD_NO'::text, 'REP_ID_CARD_NO'::text, 'TAX_ID'::text, 'BANK_ACCOUNT'::text])),
  CONSTRAINT "kyc_identity_hashes_hash_check" CHECK (octet_length(hash) = 32),
  CONSTRAINT "kyc_identity_hashes_key_check" CHECK (key_id <> ''::text)
);
-- Create index "kyc_identity_hashes_hash" to table: "kyc_identity_hashes"
CREATE INDEX "kyc_identity_hashes_hash" ON "seller"."kyc_identity_hashes" ("hash", "key_id");
INSERT INTO "seller"."kyc_identity_hashes" ("application_id", "field", "key_id", "hash")
SELECT "application_id", 'ID_CARD_NO', "id_card_hash_key", "id_card_hash" FROM "seller"."kyc_applications" WHERE "id_card_hash" IS NOT NULL
UNION ALL
SELECT "application_id", 'REP_ID_CARD_NO', "rep_id_card_hash_key", "rep_id_card_hash" FROM "seller"."kyc_applications" WHERE "rep_id_card_hash" IS NOT NULL
UNION ALL
SELECT "application_id", 'TAX_ID', "tax_id_hash_key", "tax_id_hash" FROM "seller"."kyc_applications" WHERE "tax_id_hash" IS NOT NULL
UNION ALL
SELECT "application_id", 'BANK_ACCOUNT', "bank_account_hash_key", "bank_account_hash" FROM "seller"."kyc_applications" WHERE "bank_account_hash" IS NOT NULL;
