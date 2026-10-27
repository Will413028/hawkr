ALTER TABLE "payuni_registrations" ADD COLUMN "seller_account_id" bigint NULL;
UPDATE "payuni_registrations" SET "seller_account_id" = "account_id" WHERE "seller_account_id" IS NULL;
