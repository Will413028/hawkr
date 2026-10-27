-- Create "procurement_order_provisioning_outbox" table
CREATE TABLE "procurement_order_provisioning_outbox" (
  "id" bigint NOT NULL,
  "acceptance_id" bigint NOT NULL,
  "event_type" text NOT NULL,
  "payload" jsonb NOT NULL,
  "status" text NOT NULL DEFAULT 'pending',
  "attempt_count" integer NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "last_error" text NULL,
  "product_sku_id" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_propo_attempt_count_non_negative" CHECK (attempt_count >= 0),
  CONSTRAINT "chk_propo_completed_at" CHECK ((status = 'done'::text) = (completed_at IS NOT NULL)),
  CONSTRAINT "chk_propo_event_type" CHECK (event_type = 'RFQ_ORDER_SHELL_REQUESTED'::text),
  CONSTRAINT "chk_propo_status" CHECK (status = ANY (ARRAY['pending'::text, 'done'::text, 'failed'::text]))
);
-- Create index "idx_propo_pending" to table: "procurement_order_provisioning_outbox"
CREATE INDEX "idx_propo_pending" ON "procurement_order_provisioning_outbox" ("status", "next_attempt_at") WHERE (status = 'pending'::text);
-- Create index "uq_propo_acceptance_event" to table: "procurement_order_provisioning_outbox"
CREATE UNIQUE INDEX "uq_propo_acceptance_event" ON "procurement_order_provisioning_outbox" ("acceptance_id", "event_type");
