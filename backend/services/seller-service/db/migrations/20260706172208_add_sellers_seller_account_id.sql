ALTER TABLE "sellers" ADD COLUMN "seller_account_id" bigint NULL;
UPDATE "sellers" SET "seller_account_id" = "account_id" WHERE "seller_account_id" IS NULL;
