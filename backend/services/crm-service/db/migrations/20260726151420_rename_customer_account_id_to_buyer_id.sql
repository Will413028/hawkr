-- Rename crm's misnamed buyer identifier: the column named customer_account_id
-- has always held buyer-service's buyer_id (PK), never an account_id. The value
-- arrives from order.completed's buyer_account_id field, whose own javadoc
-- (OrderCompletedPayload.java:24) documents it as carrying buyer_id.
-- Rename only — no data changes.

-- crm_customers
ALTER TABLE "crm_customers" RENAME COLUMN "customer_account_id" TO "buyer_id";
ALTER TABLE "crm_customers" RENAME CONSTRAINT "crm_customers_store_id_customer_account_id_key" TO "crm_customers_store_id_buyer_id_key";
COMMENT ON COLUMN "crm_customers"."buyer_id" IS 'buyers.buyer_id (per-stint PK, never equals account_id). Renamed from customer_account_id 2026-07-26.';

-- crm_customer_tags
ALTER TABLE "crm_customer_tags" RENAME COLUMN "customer_account_id" TO "buyer_id";
ALTER TABLE "crm_customer_tags" RENAME CONSTRAINT "crm_customer_tags_store_id_customer_account_id_tag_id_key" TO "crm_customer_tags_store_id_buyer_id_tag_id_key";
COMMENT ON COLUMN "crm_customer_tags"."buyer_id" IS 'buyers.buyer_id. Renamed from customer_account_id 2026-07-26.';

-- crm_customer_ledger
ALTER TABLE "crm_customer_ledger" RENAME COLUMN "customer_account_id" TO "buyer_id";
COMMENT ON COLUMN "crm_customer_ledger"."buyer_id" IS 'buyers.buyer_id. Renamed from customer_account_id 2026-07-26.';

-- Named NOT NULL constraints are deliberately NOT renamed here. Postgres only
-- started recording them in pg_constraint in PG 18, so `ALTER TABLE ... RENAME
-- CONSTRAINT ..._not_null` errors with 42704 on anything older. dev is PG 18.1,
-- but prod runs PlanetScale Postgres (k8s/values/overlays/prod/common.yaml) of
-- unknown version, and Atlas's default tx-mode=file would roll back this whole
-- file on that error — taking the RENAME COLUMNs above down with it. The stale
-- names are purely cosmetic, so they stay stale. Every RENAME CONSTRAINT in
-- this repo targets a _key/_pkey, which exists on all supported versions.

-- Pre-existing stale index name left over from the 2026-07-12 store_id rename.
ALTER INDEX "idx_crm_tags_account_id" RENAME TO "idx_crm_tags_store_id";
