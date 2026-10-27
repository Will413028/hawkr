-- Create "procurement_rfq_publication_outbox" table
CREATE TABLE "procurement_rfq_publication_outbox" (
  "id" bigint NOT NULL,
  "invite_id" bigint NOT NULL,
  "event_type" text NOT NULL,
  "payload" jsonb NOT NULL,
  "status" text NOT NULL DEFAULT 'pending',
  "attempt_count" integer NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "last_error" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_prpo_event_type" CHECK (event_type = 'RFQ_PUBLICATION_INVITE_CREATED'::text),
  CONSTRAINT "chk_prpo_status" CHECK (status = ANY (ARRAY['pending'::text, 'done'::text, 'failed'::text]))
);
-- Create index "idx_prpo_pending" to table: "procurement_rfq_publication_outbox"
CREATE INDEX "idx_prpo_pending" ON "procurement_rfq_publication_outbox" ("status", "next_attempt_at") WHERE (status = 'pending'::text);
-- Create index "uq_prpo_invite_event" to table: "procurement_rfq_publication_outbox"
CREATE UNIQUE INDEX "uq_prpo_invite_event" ON "procurement_rfq_publication_outbox" ("invite_id", "event_type");
