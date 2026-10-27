UPDATE "payuni_registrations" SET "seller_account_id" = "account_id" WHERE "seller_account_id" IS NULL;
ALTER TABLE "payuni_registrations" DROP CONSTRAINT "payuni_registrations_account_id_key", ALTER COLUMN "seller_account_id" SET NOT NULL, ADD CONSTRAINT "payuni_registrations_seller_account_id_key" UNIQUE ("seller_account_id");
