-- Drop index "kyc_reveals_application" from table: "kyc_reveals"
DROP INDEX "seller"."kyc_reveals_application";
-- Modify "kyc_reveals" table
ALTER TABLE "seller"."kyc_reveals" ADD COLUMN "expires_at" timestamptz NULL;
-- 既有的顯示全文紀錄（上線前只有本機與測試資料）回填成已到期的授權（顯示之後 15 分鐘），再設 NOT NULL 與 CHECK
UPDATE "seller"."kyc_reveals" SET "expires_at" = "revealed_at" + interval '15 minutes';
ALTER TABLE "seller"."kyc_reveals" ALTER COLUMN "expires_at" SET NOT NULL, ADD CONSTRAINT "kyc_reveals_expiry_check" CHECK (expires_at > revealed_at), ADD CONSTRAINT "kyc_reveals_grant_key" UNIQUE ("reveal_id", "application_id");
-- Create index "kyc_reveals_application" to table: "kyc_reveals"
CREATE INDEX "kyc_reveals_application" ON "seller"."kyc_reveals" ("application_id", "revealed_at" DESC, "reveal_id" DESC);
-- Create "kyc_document_accesses" table
CREATE TABLE "seller"."kyc_document_accesses" (
  "access_id" uuid NOT NULL,
  "application_id" uuid NOT NULL,
  "reveal_id" uuid NOT NULL,
  "slot" text NOT NULL,
  "position" integer NOT NULL,
  "asset_id" text NOT NULL,
  "accessed_at" timestamptz NOT NULL,
  PRIMARY KEY ("access_id"),
  CONSTRAINT "kyc_document_accesses_grant_fkey" FOREIGN KEY ("reveal_id", "application_id") REFERENCES "seller"."kyc_reveals" ("reveal_id", "application_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "kyc_document_accesses_asset_check" CHECK (asset_id <> ''::text),
  CONSTRAINT "kyc_document_accesses_position_check" CHECK (("position" >= 0) AND ("position" < 10) AND ((slot = 'COMPANY_DOCUMENT'::text) OR ("position" = 0))),
  CONSTRAINT "kyc_document_accesses_slot_check" CHECK (slot = ANY (ARRAY['ID_CARD_FRONT'::text, 'ID_CARD_BACK'::text, 'BANK_BOOK'::text, 'COMPANY_DOCUMENT'::text]))
);
-- Create index "kyc_document_accesses_application" to table: "kyc_document_accesses"
CREATE INDEX "kyc_document_accesses_application" ON "seller"."kyc_document_accesses" ("application_id", "accessed_at" DESC, "access_id" DESC);
