-- Add new schema named "coupon"
CREATE SCHEMA "coupon";
-- Create "coupon_campaigns" table
CREATE TABLE "coupon"."coupon_campaigns" (
  "id" uuid NOT NULL,
  "title" text NOT NULL,
  "discount_type" text NOT NULL,
  "discount_amount" bigint NULL,
  "discount_percent" integer NULL,
  "max_discount_amount" bigint NULL,
  "threshold_amount" bigint NOT NULL DEFAULT 0,
  "target_type" text NOT NULL,
  "store_id" bigint NULL,
  "issuer_type" text NOT NULL,
  "issuer_admin_id" bigint NULL,
  "issuer_store_id" bigint NULL,
  "start_at" timestamptz NOT NULL,
  "end_at" timestamptz NOT NULL,
  "total_limit" integer NULL,
  "issued_count" integer NOT NULL DEFAULT 0,
  "status" text NOT NULL,
  "source_type" text NULL,
  "source_id" text NULL,
  "ended_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "campaigns_source_key" UNIQUE ("source_type", "source_id"),
  CONSTRAINT "campaigns_discount_shape_check" CHECK (((discount_type = 'CASH'::text) AND (discount_amount IS NOT NULL) AND (discount_amount > 0) AND (discount_percent IS NULL) AND (max_discount_amount IS NULL)) OR ((discount_type = 'PERCENT'::text) AND (discount_percent IS NOT NULL) AND (discount_percent >= 1) AND (discount_percent <= 99) AND (max_discount_amount IS NOT NULL) AND (max_discount_amount > 0) AND (discount_amount IS NULL))),
  CONSTRAINT "campaigns_discount_type_check" CHECK (discount_type = ANY (ARRAY['CASH'::text, 'PERCENT'::text])),
  CONSTRAINT "campaigns_discount_within_threshold_check" CHECK ((discount_type <> 'CASH'::text) OR (threshold_amount = 0) OR (discount_amount <= threshold_amount)),
  CONSTRAINT "campaigns_ended_check" CHECK ((status = 'ENDED'::text) = (ended_at IS NOT NULL)),
  CONSTRAINT "campaigns_issued_count_check" CHECK ((issued_count >= 0) AND ((total_limit IS NULL) OR (issued_count <= total_limit))),
  CONSTRAINT "campaigns_issuer_check" CHECK (((issuer_type = 'ADMIN'::text) AND (issuer_admin_id IS NOT NULL) AND (issuer_admin_id > 0) AND (issuer_store_id IS NULL)) OR ((issuer_type = 'SELLER'::text) AND (issuer_store_id IS NOT NULL) AND (issuer_store_id > 0) AND (issuer_admin_id IS NULL))),
  CONSTRAINT "campaigns_seller_scope_check" CHECK ((issuer_type <> 'SELLER'::text) OR (NOT (store_id IS DISTINCT FROM issuer_store_id))),
  CONSTRAINT "campaigns_source_check" CHECK (((source_type IS NULL) AND (source_id IS NULL) AND (target_type <> 'COMPENSATION'::text)) OR ((source_type IS NOT NULL) AND (source_id IS NOT NULL) AND (source_type = ANY (ARRAY['AFTERSALES_CASE'::text, 'ADMIN_COMPENSATION'::text])) AND (char_length(source_id) >= 1) AND (char_length(source_id) <= 128) AND (target_type = 'COMPENSATION'::text))),
  CONSTRAINT "campaigns_status_check" CHECK (status = ANY (ARRAY['DRAFT'::text, 'ACTIVE'::text, 'ENDED'::text])),
  CONSTRAINT "campaigns_store_check" CHECK (store_id > 0),
  CONSTRAINT "campaigns_store_scope_check" CHECK ((target_type <> ALL (ARRAY['MEMBER'::text, 'COMPENSATION'::text])) OR (store_id IS NOT NULL)),
  CONSTRAINT "campaigns_target_type_check" CHECK (target_type = ANY (ARRAY['ALL'::text, 'MEMBER'::text, 'DESIGNATED'::text, 'COMPENSATION'::text])),
  CONSTRAINT "campaigns_threshold_check" CHECK (threshold_amount >= 0),
  CONSTRAINT "campaigns_title_check" CHECK ((char_length(title) >= 1) AND (char_length(title) <= 100) AND (title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "campaigns_total_limit_check" CHECK (total_limit >= 1),
  CONSTRAINT "campaigns_window_check" CHECK (start_at <= end_at)
);
-- Create index "campaigns_by_issuer_store" to table: "coupon_campaigns"
CREATE INDEX "campaigns_by_issuer_store" ON "coupon"."coupon_campaigns" ("issuer_store_id", "status") WHERE (issuer_store_id IS NOT NULL);
-- Create "coupon_redemption_logs" table
CREATE TABLE "coupon"."coupon_redemption_logs" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "campaign_id" uuid NOT NULL,
  "voucher_id" uuid NULL,
  "buyer_id" bigint NULL,
  "action" text NOT NULL,
  "order_no" text NULL,
  "operator_admin_id" bigint NULL,
  "operator_store_id" bigint NULL,
  "reason" text NULL,
  "voided_count" integer NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "redemption_logs_action_check" CHECK (action = ANY (ARRAY['CLAIM'::text, 'ISSUE'::text, 'LOCK'::text, 'REDEEM'::text, 'RELEASE'::text, 'VOID'::text, 'END_CAMPAIGN'::text])),
  CONSTRAINT "redemption_logs_reason_check" CHECK ((char_length(reason) >= 1) AND (char_length(reason) <= 255) AND (reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "redemption_logs_voided_count_check" CHECK (voided_count >= 0)
);
-- Create index "redemption_logs_by_campaign" to table: "coupon_redemption_logs"
CREATE INDEX "redemption_logs_by_campaign" ON "coupon"."coupon_redemption_logs" ("campaign_id", "created_at");
-- Create index "redemption_logs_by_voucher" to table: "coupon_redemption_logs"
CREATE INDEX "redemption_logs_by_voucher" ON "coupon"."coupon_redemption_logs" ("voucher_id", "created_at") WHERE (voucher_id IS NOT NULL);
-- Create "seller_coupon_quotas" table
CREATE TABLE "coupon"."seller_coupon_quotas" (
  "store_id" bigint NOT NULL,
  "max_campaign_count" integer NOT NULL DEFAULT 5,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "seller_quotas_max_check" CHECK (max_campaign_count >= 0),
  CONSTRAINT "seller_quotas_store_check" CHECK (store_id > 0)
);
-- Create "coupon_target_accounts" table
CREATE TABLE "coupon"."coupon_target_accounts" (
  "campaign_id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("campaign_id", "buyer_id"),
  CONSTRAINT "coupon_target_accounts_campaign_id_fkey" FOREIGN KEY ("campaign_id") REFERENCES "coupon"."coupon_campaigns" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "target_accounts_buyer_check" CHECK (buyer_id > 0)
);
-- Create index "target_accounts_by_buyer" to table: "coupon_target_accounts"
CREATE INDEX "target_accounts_by_buyer" ON "coupon"."coupon_target_accounts" ("buyer_id");
-- Create "coupon_vouchers" table
CREATE TABLE "coupon"."coupon_vouchers" (
  "id" uuid NOT NULL,
  "campaign_id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "origin" text NOT NULL,
  "source_type" text NULL,
  "source_id" text NULL,
  "status" text NOT NULL,
  "order_no" text NULL,
  "locked_at" timestamptz NULL,
  "redeemed_at" timestamptz NULL,
  "voided_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "vouchers_campaign_buyer_key" UNIQUE ("campaign_id", "buyer_id"),
  CONSTRAINT "vouchers_source_key" UNIQUE ("source_type", "source_id"),
  CONSTRAINT "coupon_vouchers_campaign_id_fkey" FOREIGN KEY ("campaign_id") REFERENCES "coupon"."coupon_campaigns" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "vouchers_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "vouchers_order_no_check" CHECK ((char_length(order_no) >= 1) AND (char_length(order_no) <= 128)),
  CONSTRAINT "vouchers_origin_check" CHECK (origin = ANY (ARRAY['CLAIM'::text, 'GRANT'::text, 'COMPENSATION'::text])),
  CONSTRAINT "vouchers_source_check" CHECK (((origin <> 'COMPENSATION'::text) AND (source_type IS NULL) AND (source_id IS NULL)) OR ((origin = 'COMPENSATION'::text) AND (source_type IS NOT NULL) AND (source_id IS NOT NULL) AND (source_type = ANY (ARRAY['AFTERSALES_CASE'::text, 'ADMIN_COMPENSATION'::text])) AND (char_length(source_id) >= 1) AND (char_length(source_id) <= 128))),
  CONSTRAINT "vouchers_state_check" CHECK (((status = 'AVAILABLE'::text) AND (order_no IS NULL) AND (locked_at IS NULL) AND (redeemed_at IS NULL) AND (voided_at IS NULL)) OR ((status = 'LOCKED'::text) AND (order_no IS NOT NULL) AND (locked_at IS NOT NULL) AND (redeemed_at IS NULL) AND (voided_at IS NULL)) OR ((status = 'REDEEMED'::text) AND (order_no IS NOT NULL) AND (locked_at IS NOT NULL) AND (redeemed_at IS NOT NULL) AND (voided_at IS NULL)) OR ((status = 'VOIDED'::text) AND (order_no IS NULL) AND (locked_at IS NULL) AND (redeemed_at IS NULL) AND (voided_at IS NOT NULL))),
  CONSTRAINT "vouchers_status_check" CHECK (status = ANY (ARRAY['AVAILABLE'::text, 'LOCKED'::text, 'REDEEMED'::text, 'VOIDED'::text]))
);
-- Create index "vouchers_by_buyer" to table: "coupon_vouchers"
CREATE INDEX "vouchers_by_buyer" ON "coupon"."coupon_vouchers" ("buyer_id", "status", "created_at" DESC, "id" DESC);
-- Create index "vouchers_by_campaign" to table: "coupon_vouchers"
CREATE INDEX "vouchers_by_campaign" ON "coupon"."coupon_vouchers" ("campaign_id", "status");
-- Create index "vouchers_order_no_key" to table: "coupon_vouchers"
CREATE UNIQUE INDEX "vouchers_order_no_key" ON "coupon"."coupon_vouchers" ("order_no") WHERE (order_no IS NOT NULL);
