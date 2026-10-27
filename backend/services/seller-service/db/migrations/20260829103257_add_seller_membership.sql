-- Create "seller_membership_plans" table
CREATE TABLE "seller_membership_plans" (
  "id" bigint NOT NULL,
  "plan_code" character varying(30) NOT NULL,
  "display_name" character varying(100) NOT NULL,
  "tier_rank" integer NOT NULL DEFAULT 0,
  "monthly_price_twd" integer NOT NULL DEFAULT 0,
  "monthly_order_cap" integer NOT NULL,
  "is_purchasable" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_seller_membership_plans_cap_positive" CHECK (monthly_order_cap > 0),
  CONSTRAINT "chk_seller_membership_plans_price_nonneg" CHECK (monthly_price_twd >= 0)
);
-- Create index "uq_seller_membership_plans_plan_code" to table: "seller_membership_plans"
CREATE UNIQUE INDEX "uq_seller_membership_plans_plan_code" ON "seller_membership_plans" ("plan_code");
-- Create "seller_membership_subscriptions" table
CREATE TABLE "seller_membership_subscriptions" (
  "id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "plan_code" character varying(30) NOT NULL,
  "status" character varying(20) NOT NULL DEFAULT 'active',
  "effective_at" timestamptz NOT NULL,
  "source" character varying(20) NOT NULL,
  "granted_by" bigint NULL,
  "reason" text NOT NULL DEFAULT '',
  "request_id" character varying(100) NOT NULL DEFAULT '',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "seller_membership_subscriptions_plan_code_fkey" FOREIGN KEY ("plan_code") REFERENCES "seller_membership_plans" ("plan_code") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_seller_membership_subscriptions_source" CHECK ((source)::text = ANY ((ARRAY['admin'::character varying, 'system_default'::character varying])::text[])),
  CONSTRAINT "chk_seller_membership_subscriptions_status" CHECK ((status)::text = ANY ((ARRAY['active'::character varying, 'superseded'::character varying])::text[]))
);
-- Create index "idx_seller_membership_subscriptions_store_created" to table: "seller_membership_subscriptions"
CREATE INDEX "idx_seller_membership_subscriptions_store_created" ON "seller_membership_subscriptions" ("store_id", "created_at" DESC);
-- Create index "uq_seller_membership_subscriptions_active" to table: "seller_membership_subscriptions"
CREATE UNIQUE INDEX "uq_seller_membership_subscriptions_active" ON "seller_membership_subscriptions" ("store_id") WHERE ((status)::text = 'active'::text);
-- Create index "uq_seller_membership_subscriptions_request_id" to table: "seller_membership_subscriptions"
CREATE UNIQUE INDEX "uq_seller_membership_subscriptions_request_id" ON "seller_membership_subscriptions" ("request_id") WHERE ((request_id)::text <> ''::text);
-- Seed seller_membership_plans (free/plus/pro). Hand-appended: Atlas's declarative
-- schema diff does not emit DML for row data (structural DDL diff only), so the
-- INSERTs from schema.sql are not auto-generated here even though the rows exist
-- in the ephemeral dev-diff database. Values are PLACEHOLDER pending business
-- sign-off (see schema.sql for the same rationale).
INSERT INTO "seller_membership_plans" ("id", "plan_code", "display_name", "tier_rank", "monthly_price_twd", "monthly_order_cap", "is_purchasable") VALUES
  (2001, 'free', '免費方案', 0, 0, 50,   true),
  (2002, 'plus', '進階方案', 1, 0, 200,  true),
  (2003, 'pro',  '旗艦方案', 2, 0, 1000, true);
