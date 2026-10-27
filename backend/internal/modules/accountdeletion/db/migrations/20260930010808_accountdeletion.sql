-- Add new schema named "accountdeletion"
CREATE SCHEMA "accountdeletion";
-- Create "deletion_requests" table
CREATE TABLE "accountdeletion"."deletion_requests" (
  "id" uuid NOT NULL,
  "account_id" bigint NOT NULL,
  "status" text NOT NULL,
  "requested_at" timestamptz NOT NULL,
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "deletion_requests_account_check" CHECK (account_id > 0),
  CONSTRAINT "deletion_requests_completed_check" CHECK ((status = 'COMPLETED'::text) = (completed_at IS NOT NULL)),
  CONSTRAINT "deletion_requests_status_check" CHECK (status = ANY (ARRAY['PENDING_ORDERS'::text, 'COMPLETED'::text]))
);
-- Create index "deletion_requests_one_pending" to table: "deletion_requests"
CREATE UNIQUE INDEX "deletion_requests_one_pending" ON "accountdeletion"."deletion_requests" ("account_id") WHERE (status = 'PENDING_ORDERS'::text);
-- Create "request_attempts" table
CREATE TABLE "accountdeletion"."request_attempts" (
  "id" uuid NOT NULL,
  "request_id" uuid NOT NULL,
  "requester" text NOT NULL,
  "operator_admin_id" bigint NULL,
  "reason" text NOT NULL,
  "attempted_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "request_attempts_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "accountdeletion"."deletion_requests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "request_attempts_operator_check" CHECK (((requester = 'ADMIN'::text) = (operator_admin_id IS NOT NULL)) AND ((operator_admin_id IS NULL) OR (operator_admin_id > 0))),
  CONSTRAINT "request_attempts_reason_check" CHECK ((char_length(reason) <= 1000) AND ((requester <> 'ADMIN'::text) OR (char_length(btrim(reason)) > 0))),
  CONSTRAINT "request_attempts_requester_check" CHECK (requester = ANY (ARRAY['ACCOUNT_OWNER'::text, 'ADMIN'::text]))
);
-- Create index "request_attempts_request_time" to table: "request_attempts"
CREATE INDEX "request_attempts_request_time" ON "accountdeletion"."request_attempts" ("request_id", "attempted_at", "id");
