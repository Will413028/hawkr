UPDATE "sellers" SET "seller_account_id" = "account_id" WHERE "seller_account_id" IS NULL;
ALTER TABLE "sellers" ALTER COLUMN "seller_account_id" SET NOT NULL, ADD CONSTRAINT "sellers_seller_account_id_key" UNIQUE ("seller_account_id");
