-- Add new schema named "crm"
CREATE SCHEMA "crm";
-- Create "customers" table
CREATE TABLE "crm"."customers" (
  "store_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "customer_type" text NOT NULL DEFAULT 'NORMAL',
  "total_spent" bigint NOT NULL DEFAULT 0,
  "order_count" integer NOT NULL DEFAULT 0,
  "first_order_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id", "buyer_id"),
  CONSTRAINT "customers_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "customers_order_count_check" CHECK (order_count >= 0),
  CONSTRAINT "customers_store_check" CHECK (store_id > 0),
  CONSTRAINT "customers_total_spent_check" CHECK (total_spent >= 0),
  CONSTRAINT "customers_type_check" CHECK (customer_type = ANY (ARRAY['NORMAL'::text, 'VIP'::text]))
);
-- Create index "customers_by_buyer" to table: "customers"
CREATE INDEX "customers_by_buyer" ON "crm"."customers" ("buyer_id");
-- Create index "customers_by_first_order" to table: "customers"
CREATE INDEX "customers_by_first_order" ON "crm"."customers" ("store_id", "first_order_at" DESC, "buyer_id" DESC);
-- Create index "customers_by_orders" to table: "customers"
CREATE INDEX "customers_by_orders" ON "crm"."customers" ("store_id", "order_count" DESC, "buyer_id" DESC);
-- Create index "customers_by_spent" to table: "customers"
CREATE INDEX "customers_by_spent" ON "crm"."customers" ("store_id", "total_spent" DESC, "buyer_id" DESC);
-- Create "inactive_parties" table
CREATE TABLE "crm"."inactive_parties" (
  "kind" text NOT NULL,
  "party_id" bigint NOT NULL,
  "since" timestamptz NOT NULL,
  PRIMARY KEY ("kind", "party_id"),
  CONSTRAINT "inactive_parties_kind_check" CHECK (kind = ANY (ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "inactive_parties_party_check" CHECK (party_id > 0)
);
-- Create "tags" table
CREATE TABLE "crm"."tags" (
  "id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "name" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "tags_store_id_key" UNIQUE ("store_id", "id"),
  CONSTRAINT "tags_store_name_key" UNIQUE ("store_id", "name"),
  CONSTRAINT "tags_name_check" CHECK ((char_length(name) >= 1) AND (char_length(name) <= 50) AND (name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "tags_store_check" CHECK (store_id > 0)
);
-- Create "customer_tags" table
CREATE TABLE "crm"."customer_tags" (
  "store_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "tag_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id", "buyer_id", "tag_id"),
  CONSTRAINT "customer_tags_customer_fkey" FOREIGN KEY ("store_id", "buyer_id") REFERENCES "crm"."customers" ("store_id", "buyer_id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "customer_tags_tag_fkey" FOREIGN KEY ("store_id", "tag_id") REFERENCES "crm"."tags" ("store_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "customer_tags_by_tag" to table: "customer_tags"
CREATE INDEX "customer_tags_by_tag" ON "crm"."customer_tags" ("store_id", "tag_id", "buyer_id");
-- Create "order_nets" table
CREATE TABLE "crm"."order_nets" (
  "order_no" text NOT NULL,
  "store_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "completed" boolean NOT NULL DEFAULT false,
  "net_amount" bigint NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("order_no"),
  CONSTRAINT "order_nets_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "order_nets_order_no_check" CHECK ((char_length(order_no) >= 1) AND (char_length(order_no) <= 128)),
  CONSTRAINT "order_nets_store_check" CHECK (store_id > 0)
);
-- Create index "order_nets_by_buyer" to table: "order_nets"
CREATE INDEX "order_nets_by_buyer" ON "crm"."order_nets" ("buyer_id");
-- Create index "order_nets_by_customer" to table: "order_nets"
CREATE INDEX "order_nets_by_customer" ON "crm"."order_nets" ("store_id", "buyer_id");
-- Create "ledger" table
CREATE TABLE "crm"."ledger" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "dedup_key" text NOT NULL,
  "order_no" text NOT NULL,
  "kind" text NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "ledger_dedup_key_key" UNIQUE ("dedup_key"),
  CONSTRAINT "ledger_order_no_fkey" FOREIGN KEY ("order_no") REFERENCES "crm"."order_nets" ("order_no") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "ledger_dedup_key_check" CHECK ((char_length(dedup_key) >= 1) AND (char_length(dedup_key) <= 128)),
  CONSTRAINT "ledger_kind_check" CHECK (kind = ANY (ARRAY['ORDER_COMPLETED'::text, 'REFUND_FULL'::text, 'REFUND_PARTIAL'::text]))
);
-- Create index "ledger_by_order" to table: "ledger"
CREATE INDEX "ledger_by_order" ON "crm"."ledger" ("order_no", "kind");
