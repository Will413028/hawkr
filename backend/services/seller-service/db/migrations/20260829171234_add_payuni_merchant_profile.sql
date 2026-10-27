-- Modify "payuni_registrations" table
ALTER TABLE "payuni_registrations" ADD COLUMN "contact_name" character varying(100) NULL, ADD COLUMN "contact_mobile" character varying(20) NULL, ADD COLUMN "contact_tel" character varying(20) NULL, ADD COLUMN "mer_email" character varying(255) NULL, ADD COLUMN "mer_addr" character varying(255) NULL, ADD COLUMN "mer_addr_post" character varying(3) NULL, ADD COLUMN "mer_tel" character varying(20) NULL, ADD COLUMN "merchant_profile_version" bigint NOT NULL DEFAULT 0;
-- Create "payuni_merchant_profile_updates" table
CREATE TABLE "payuni_merchant_profile_updates" (
  "id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "idempotency_key" character varying(128) NOT NULL,
  "payload_hash" character varying(64) NOT NULL,
  "status" character varying(20) NOT NULL,
  "changed_fields" jsonb NOT NULL,
  "reason" text NULL,
  "trace_id" character varying(64) NULL,
  "provider_status" character varying(20) NULL,
  "provider_message" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "uq_payuni_merchant_profile_updates_store_key" to table: "payuni_merchant_profile_updates"
CREATE UNIQUE INDEX "uq_payuni_merchant_profile_updates_store_key" ON "payuni_merchant_profile_updates" ("store_id", "idempotency_key");
