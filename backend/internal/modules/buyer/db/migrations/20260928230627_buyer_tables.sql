-- Create "buyers" table
CREATE TABLE "buyer"."buyers" (
  "buyer_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "account_id" bigint NOT NULL,
  "status" text NOT NULL DEFAULT 'ACTIVE',
  "display_name" text NULL,
  "phone" text NULL,
  "gender" text NULL,
  "avatar" text NULL,
  "birth_date" date NULL,
  "city" text NULL,
  "job" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "deleted_at" timestamptz NULL,
  "pii_cleared_at" timestamptz NULL,
  PRIMARY KEY ("buyer_id"),
  CONSTRAINT "buyers_deleted_check" CHECK ((status = 'CLOSED'::text) = (deleted_at IS NOT NULL)),
  CONSTRAINT "buyers_display_name_check" CHECK (((pii_cleared_at IS NOT NULL) AND (display_name IS NULL)) OR ((pii_cleared_at IS NULL) AND (display_name IS NOT NULL) AND ((char_length(display_name) >= 1) AND (char_length(display_name) <= 100)))),
  CONSTRAINT "buyers_gender_check" CHECK (gender = ANY (ARRAY['MALE'::text, 'FEMALE'::text, 'OTHER'::text])),
  CONSTRAINT "buyers_pii_cleared_check" CHECK ((status = 'CLOSED'::text) OR (pii_cleared_at IS NULL)),
  CONSTRAINT "buyers_status_check" CHECK (status = ANY (ARRAY['ACTIVE'::text, 'PENDING_DELETION'::text, 'CLOSED'::text]))
);
-- Create index "buyers_current" to table: "buyers"
CREATE UNIQUE INDEX "buyers_current" ON "buyer"."buyers" ("account_id") WHERE (status <> 'CLOSED'::text);
-- Create "deletion_audits" table
CREATE TABLE "buyer"."deletion_audits" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "trigger" text NOT NULL,
  "operator_admin_id" bigint NULL,
  "reason" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "deletion_audits_buyer_key" UNIQUE ("buyer_id"),
  CONSTRAINT "deletion_audits_buyer_id_fkey" FOREIGN KEY ("buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "deletion_audits_trigger_check" CHECK (trigger = ANY (ARRAY['BUYER_REQUEST'::text, 'ADMIN'::text, 'ACCOUNT_DELETED'::text]))
);
-- Create "favorites" table
CREATE TABLE "buyer"."favorites" (
  "buyer_id" bigint NOT NULL,
  "product_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("buyer_id", "product_id"),
  CONSTRAINT "favorites_buyer_id_fkey" FOREIGN KEY ("buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "membership_plans" table
CREATE TABLE "buyer"."membership_plans" (
  "plan_code" text NOT NULL,
  "display_name" text NOT NULL,
  "monthly_answer_limit" integer NOT NULL,
  "monthly_price_twd" integer NOT NULL,
  "is_purchasable" boolean NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("plan_code"),
  CONSTRAINT "membership_plans_limit_check" CHECK (monthly_answer_limit > 0),
  CONSTRAINT "membership_plans_price_check" CHECK (monthly_price_twd >= 0)
);
-- Create "memberships" table
CREATE TABLE "buyer"."memberships" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "plan_code" text NOT NULL,
  "period_start" timestamptz NOT NULL,
  "period_end" timestamptz NOT NULL,
  "source" text NOT NULL,
  "status" text NOT NULL,
  "request_id" text NOT NULL,
  "granted_by" bigint NULL,
  "note" text NOT NULL DEFAULT '',
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "memberships_buyer_id_fkey" FOREIGN KEY ("buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "memberships_plan_code_fkey" FOREIGN KEY ("plan_code") REFERENCES "buyer"."membership_plans" ("plan_code") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "memberships_period_check" CHECK (period_end > period_start),
  CONSTRAINT "memberships_request_check" CHECK ((char_length(request_id) >= 1) AND (char_length(request_id) <= 100)),
  CONSTRAINT "memberships_source_check" CHECK (source = 'ADMIN'::text),
  CONSTRAINT "memberships_status_check" CHECK (status = ANY (ARRAY['ACTIVE'::text, 'SUPERSEDED'::text]))
);
-- Create index "memberships_active" to table: "memberships"
CREATE UNIQUE INDEX "memberships_active" ON "buyer"."memberships" ("buyer_id") WHERE (status = 'ACTIVE'::text);
-- Create index "memberships_request" to table: "memberships"
CREATE UNIQUE INDEX "memberships_request" ON "buyer"."memberships" ("request_id");
-- Create "restock_subscriptions" table
CREATE TABLE "buyer"."restock_subscriptions" (
  "buyer_id" bigint NOT NULL,
  "product_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("buyer_id", "product_id"),
  CONSTRAINT "restock_subscriptions_buyer_id_fkey" FOREIGN KEY ("buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "restock_product" to table: "restock_subscriptions"
CREATE INDEX "restock_product" ON "buyer"."restock_subscriptions" ("product_id", "buyer_id");
-- Create "seller_subscriptions" table
CREATE TABLE "buyer"."seller_subscriptions" (
  "buyer_id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("buyer_id", "store_id"),
  CONSTRAINT "seller_subscriptions_buyer_id_fkey" FOREIGN KEY ("buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "subscriptions_store" to table: "seller_subscriptions"
CREATE INDEX "subscriptions_store" ON "buyer"."seller_subscriptions" ("store_id");
-- Create "shipping_addresses" table
CREATE TABLE "buyer"."shipping_addresses" (
  "address_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "buyer_id" bigint NOT NULL,
  "label" text NOT NULL,
  "receiver_name" text NOT NULL,
  "receiver_phone" text NOT NULL,
  "city" text NOT NULL,
  "district" text NOT NULL,
  "detail_address" text NOT NULL,
  "is_default" boolean NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("address_id"),
  CONSTRAINT "shipping_addresses_buyer_id_fkey" FOREIGN KEY ("buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "shipping_addresses_label_check" CHECK (label = ANY (ARRAY['HOME'::text, 'COMPANY'::text, 'OTHER'::text]))
);
-- Create index "addresses_default" to table: "shipping_addresses"
CREATE UNIQUE INDEX "addresses_default" ON "buyer"."shipping_addresses" ("buyer_id") WHERE is_default;
-- Create "visit_history" table
CREATE TABLE "buyer"."visit_history" (
  "buyer_id" bigint NOT NULL,
  "product_id" bigint NOT NULL,
  "visited_at" timestamptz NOT NULL,
  PRIMARY KEY ("buyer_id", "product_id"),
  CONSTRAINT "visit_history_buyer_id_fkey" FOREIGN KEY ("buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
