ALTER TABLE "sellers" RENAME COLUMN "seller_account_id" TO "store_id";
ALTER TABLE "sellers" RENAME CONSTRAINT "sellers_seller_account_id_key" TO "sellers_store_id_key";
ALTER TABLE "payuni_registrations" RENAME COLUMN "seller_account_id" TO "store_id";
ALTER TABLE "payuni_registrations" RENAME CONSTRAINT "payuni_registrations_seller_account_id_key" TO "payuni_registrations_store_id_key";
