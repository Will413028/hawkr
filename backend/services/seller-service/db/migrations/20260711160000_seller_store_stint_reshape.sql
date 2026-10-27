UPDATE "sellers" SET "status" = 'closed' WHERE "status" = 'deleted';
ALTER TABLE "sellers" ADD COLUMN "is_current" boolean NOT NULL DEFAULT true;
ALTER TABLE "sellers" ALTER COLUMN "is_current" DROP DEFAULT;
ALTER TABLE "sellers" DROP CONSTRAINT "sellers_pkey";
ALTER TABLE "sellers" DROP CONSTRAINT "sellers_store_id_key";
ALTER TABLE "sellers" ADD PRIMARY KEY ("store_id");
ALTER TABLE "sellers"
  ADD CONSTRAINT "chk_sellers_status" CHECK (status IN ('active', 'suspended_governance', 'closed')),
  ADD CONSTRAINT "chk_sellers_active_is_current" CHECK (status <> 'active' OR is_current);
CREATE UNIQUE INDEX "uq_sellers_account_current" ON "sellers" ("account_id") WHERE "is_current";
ALTER TABLE "seller_deleted_outbox"
  ADD COLUMN "event_type" text NOT NULL DEFAULT 'seller.deleted',
  ADD COLUMN "payload" jsonb;
