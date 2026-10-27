-- 主管機關來函（A3b 步驟 10e 第 3c-b-3 刀，rb R5）：來函、指名的對象（含告知）、保存期限的延長歷史、認定引用來函與措施的來源來函。新表，沒有轉換。
-- Create "authority_orders" table
CREATE TABLE "seller"."authority_orders" (
  "id" uuid NOT NULL,
  "agency" text NOT NULL,
  "document_no" text NOT NULL,
  "received_at" timestamptz NOT NULL,
  "kind" text NOT NULL,
  "summary" text NOT NULL,
  "due_at" timestamptz NOT NULL,
  "hold_until" timestamptz NOT NULL,
  "registered_at" timestamptz NOT NULL,
  "registered_by_admin_id" bigint NOT NULL,
  "completed_at" timestamptz NULL,
  "completed_by_admin_id" bigint NULL,
  "resolution_summary" text NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "request_id" uuid NULL,
  "request_hash" bytea NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "authority_orders_request_key" UNIQUE ("registered_by_admin_id", "request_id"),
  CONSTRAINT "authority_orders_admin_check" CHECK (registered_by_admin_id > 0),
  CONSTRAINT "authority_orders_complete_check" CHECK ((((completed_at IS NULL) AND (completed_by_admin_id IS NULL) AND (resolution_summary IS NULL)) OR ((completed_at >= registered_at) AND (completed_by_admin_id > 0) AND ((char_length(resolution_summary) >= 1) AND (char_length(resolution_summary) <= 4000)))) IS TRUE),
  CONSTRAINT "authority_orders_kind_check" CHECK (kind = ANY (ARRAY['RESTRICT_PRODUCT'::text, 'DATA_REQUEST'::text, 'SUSPEND_ACCOUNT'::text, 'OTHER'::text])),
  CONSTRAINT "authority_orders_request_check" CHECK ((request_id IS NULL) = (request_hash IS NULL)),
  CONSTRAINT "authority_orders_text_check" CHECK (char_length(agency) BETWEEN 1 AND 100 AND char_length(document_no) BETWEEN 1 AND 100 AND char_length(summary) BETWEEN 1 AND 4000),
  CONSTRAINT "authority_orders_times_check" CHECK ((registered_at >= received_at) AND (due_at > received_at) AND (hold_until > received_at)),
  CONSTRAINT "authority_orders_version_check" CHECK (version >= 1)
);
-- Create index "authority_orders_open_due" to table: "authority_orders"
CREATE INDEX "authority_orders_open_due" ON "seller"."authority_orders" ("due_at", "id") WHERE (completed_at IS NULL);
-- Create index "authority_orders_received" to table: "authority_orders"
CREATE INDEX "authority_orders_received" ON "seller"."authority_orders" ("received_at" DESC, "id" DESC);
-- Create "authority_order_hold_extensions" table
CREATE TABLE "seller"."authority_order_hold_extensions" (
  "order_id" uuid NOT NULL,
  "seq" smallint NOT NULL,
  "from_until" timestamptz NOT NULL,
  "to_until" timestamptz NOT NULL,
  "cause" text NOT NULL,
  "reason" text NULL,
  "admin_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("order_id", "seq"),
  CONSTRAINT "authority_order_hold_extensions_order_id_fkey" FOREIGN KEY ("order_id") REFERENCES "seller"."authority_orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "authority_order_hold_extensions_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "authority_order_hold_extensions_cause_check" CHECK ((((cause = 'ADMIN'::text) AND ((char_length(reason) >= 1) AND (char_length(reason) <= 2000))) OR ((cause = 'DATA_PROVIDED'::text) AND (reason IS NULL))) IS TRUE),
  CONSTRAINT "authority_order_hold_extensions_order_check" CHECK ((to_until > from_until) AND (seq >= 1))
);
-- Create "authority_order_targets" table
CREATE TABLE "seller"."authority_order_targets" (
  "order_id" uuid NOT NULL,
  "position" smallint NOT NULL,
  "account_id" bigint NOT NULL,
  "store_id" bigint NULL,
  "product_ids" text[] NOT NULL DEFAULT '{}',
  "notice" text NULL,
  "notified_at" timestamptz NULL,
  "notice_note" text NULL,
  "recheck_on" date NULL,
  "notice_recorded_at" timestamptz NULL,
  "notice_recorded_by_admin_id" bigint NULL,
  "prev_position" smallint NULL GENERATED ALWAYS AS (CASE WHEN ("position" > 0) THEN ("position" - 1) ELSE NULL::integer END) STORED,
  PRIMARY KEY ("order_id", "position"),
  CONSTRAINT "authority_order_targets_order_id_fkey" FOREIGN KEY ("order_id") REFERENCES "seller"."authority_orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "authority_order_targets_store_fkey" FOREIGN KEY ("store_id", "account_id") REFERENCES "seller"."stores" ("store_id", "account_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "authority_order_targets_account_check" CHECK (account_id > 0),
  CONSTRAINT "authority_order_targets_note_check" CHECK ((notice_note IS NULL) OR ((char_length(notice_note) >= 1) AND (char_length(notice_note) <= 2000))),
  CONSTRAINT "authority_order_targets_notice_check" CHECK ((((notice IS NULL) AND (notified_at IS NULL) AND (notice_note IS NULL) AND (recheck_on IS NULL) AND (notice_recorded_at IS NULL) AND (notice_recorded_by_admin_id IS NULL)) OR ((notice = 'NOTIFIED'::text) AND (notified_at IS NOT NULL) AND (recheck_on IS NULL) AND (notice_recorded_at IS NOT NULL) AND (notice_recorded_by_admin_id > 0)) OR ((notice = 'WITHHELD'::text) AND (notified_at IS NULL) AND (recheck_on IS NOT NULL) AND (notice_recorded_at IS NOT NULL) AND (notice_recorded_by_admin_id > 0)) OR ((notice = 'MEASURE_IS_NOTICE'::text) AND (notified_at IS NULL) AND (recheck_on IS NULL) AND (notice_recorded_at IS NOT NULL) AND (notice_recorded_by_admin_id > 0))) IS TRUE),
  CONSTRAINT "authority_order_targets_position_check" CHECK (("position" >= 0) AND ("position" <= 19)),
  CONSTRAINT "authority_order_targets_products_check" CHECK (cardinality(product_ids) <= 50),
  CONSTRAINT "authority_order_targets_prev_fkey" FOREIGN KEY ("order_id", "prev_position") REFERENCES "seller"."authority_order_targets" ("order_id", "position") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "authority_order_targets_account" to table: "authority_order_targets"
CREATE INDEX "authority_order_targets_account" ON "seller"."authority_order_targets" ("account_id", "order_id");
-- Create index "authority_order_targets_unique" to table: "authority_order_targets"
CREATE UNIQUE INDEX "authority_order_targets_unique" ON "seller"."authority_order_targets" ("order_id", "account_id", "store_id") NULLS NOT DISTINCT;
-- Create index "authority_order_targets_withheld" to table: "authority_order_targets"
CREATE INDEX "authority_order_targets_withheld" ON "seller"."authority_order_targets" ("recheck_on", "order_id", "position") WHERE (notice = 'WITHHELD'::text);
-- Create "authority_order_prepurged_stores" table
CREATE TABLE "seller"."authority_order_prepurged_stores" (
  "order_id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  PRIMARY KEY ("order_id", "store_id"),
  CONSTRAINT "authority_order_prepurged_stores_order_id_fkey" FOREIGN KEY ("order_id") REFERENCES "seller"."authority_orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "authority_order_prepurged_stores_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "authority_target_notice_history" table
CREATE TABLE "seller"."authority_target_notice_history" (
  "order_id" uuid NOT NULL,
  "position" smallint NOT NULL,
  "seq" smallint NOT NULL,
  "notice" text NOT NULL,
  "notice_note" text NULL,
  "recheck_on" date NOT NULL,
  "recorded_at" timestamptz NOT NULL,
  "recorded_by_admin_id" bigint NOT NULL,
  "replaced_at" timestamptz NOT NULL,
  "replaced_by_admin_id" bigint NOT NULL,
  PRIMARY KEY ("order_id", "position", "seq"),
  CONSTRAINT "authority_target_notice_history_target_fkey" FOREIGN KEY ("order_id", "position") REFERENCES "seller"."authority_order_targets" ("order_id", "position") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "authority_target_notice_history_admin_check" CHECK ((recorded_by_admin_id > 0) AND (replaced_by_admin_id > 0)),
  CONSTRAINT "authority_target_notice_history_notice_check" CHECK ((notice = 'WITHHELD'::text) AND (seq >= 1) AND (replaced_at >= recorded_at))
);
-- Create "finding_authority_orders" table
CREATE TABLE "seller"."finding_authority_orders" (
  "violation_id" uuid NOT NULL,
  "order_id" uuid NOT NULL,
  PRIMARY KEY ("violation_id", "order_id"),
  CONSTRAINT "finding_authority_orders_order_id_fkey" FOREIGN KEY ("order_id") REFERENCES "seller"."authority_orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "finding_authority_orders_violation_id_fkey" FOREIGN KEY ("violation_id") REFERENCES "seller"."violations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "finding_authority_orders_order" to table: "finding_authority_orders"
CREATE INDEX "finding_authority_orders_order" ON "seller"."finding_authority_orders" ("order_id");
-- Modify "measures" table
ALTER TABLE "seller"."measures" ADD COLUMN "source_authority_order_id" uuid NULL, ADD CONSTRAINT "measures_source_authority_order_id_fkey" FOREIGN KEY ("source_authority_order_id") REFERENCES "seller"."authority_orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "measures_source_authority_order" to table: "measures"
CREATE INDEX "measures_source_authority_order" ON "seller"."measures" ("source_authority_order_id") WHERE (source_authority_order_id IS NOT NULL);
