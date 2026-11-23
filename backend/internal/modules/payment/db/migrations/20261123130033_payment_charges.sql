-- Create "payments" table
CREATE TABLE "payment"."payments" (
  "id" uuid NOT NULL,
  "order_no" text NOT NULL,
  "store_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "amount" bigint NOT NULL,
  "provider" text NOT NULL,
  "status" text NOT NULL DEFAULT 'CREATING',
  "external_payment_id" text NULL,
  "external_charge_id" text NULL,
  "paid_at" timestamptz NULL,
  "last_error" text NULL,
  "failure_reason" text NULL,
  "fetched_at" timestamptz NULL,
  "create_idempotency_key" uuid NOT NULL DEFAULT gen_random_uuid(),
  "create_called_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "payments_amount_check" CHECK (amount > 0),
  CONSTRAINT "payments_buyer_id_check" CHECK (buyer_id > 0),
  CONSTRAINT "payments_called_check" CHECK ((external_payment_id IS NULL) OR (create_called_at IS NOT NULL)),
  CONSTRAINT "payments_charge_id_check" CHECK ((external_charge_id IS NULL) OR ((char_length(external_charge_id) >= 1) AND (char_length(external_charge_id) <= 255))),
  CONSTRAINT "payments_charge_only_when_paid_check" CHECK ((status = 'SUCCEEDED'::text) OR (external_charge_id IS NULL)),
  CONSTRAINT "payments_created_check" CHECK ((status <> ALL (ARRAY['AWAITING_PAYMENT'::text, 'PROCESSING'::text, 'SUCCEEDED'::text])) OR (external_payment_id IS NOT NULL)),
  CONSTRAINT "payments_creating_check" CHECK ((status <> 'CREATING'::text) OR (external_payment_id IS NULL)),
  CONSTRAINT "payments_external_id_check" CHECK ((external_payment_id IS NULL) OR ((char_length(external_payment_id) >= 1) AND (char_length(external_payment_id) <= 255))),
  CONSTRAINT "payments_failure_check" CHECK ((status = 'CREATE_FAILED'::text) = (failure_reason IS NOT NULL)),
  CONSTRAINT "payments_failure_reason_check" CHECK ((failure_reason IS NULL) OR ((char_length(failure_reason) >= 1) AND (char_length(failure_reason) <= 200))),
  CONSTRAINT "payments_last_error_check" CHECK ((last_error IS NULL) OR ((char_length(last_error) >= 1) AND (char_length(last_error) <= 200))),
  CONSTRAINT "payments_order_no_check" CHECK ((char_length(order_no) >= 1) AND (char_length(order_no) <= 64)),
  CONSTRAINT "payments_paid_check" CHECK (((status = 'SUCCEEDED'::text) = (paid_at IS NOT NULL)) AND ((status <> 'SUCCEEDED'::text) OR (external_charge_id IS NOT NULL))),
  CONSTRAINT "payments_provider_check" CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'::text),
  CONSTRAINT "payments_status_check" CHECK (status = ANY (ARRAY['CREATING'::text, 'AWAITING_PAYMENT'::text, 'PROCESSING'::text, 'SUCCEEDED'::text, 'CANCELED'::text, 'CREATE_FAILED'::text])),
  CONSTRAINT "payments_store_id_check" CHECK (store_id > 0)
);
-- Create index "payments_creating" to table: "payments"
CREATE INDEX "payments_creating" ON "payment"."payments" ("created_at") WHERE (status = 'CREATING'::text);
-- Create index "payments_external" to table: "payments"
CREATE UNIQUE INDEX "payments_external" ON "payment"."payments" ("provider", "external_payment_id") WHERE (external_payment_id IS NOT NULL);
-- Create index "payments_open" to table: "payments"
CREATE INDEX "payments_open" ON "payment"."payments" ("id") WHERE (status = ANY (ARRAY['AWAITING_PAYMENT'::text, 'PROCESSING'::text]));
-- Create index "payments_order" to table: "payments"
CREATE UNIQUE INDEX "payments_order" ON "payment"."payments" ("order_no");
-- Create "refunds" table
CREATE TABLE "payment"."refunds" (
  "id" uuid NOT NULL,
  "payment_id" uuid NOT NULL,
  "request_key" text NOT NULL,
  "amount" bigint NOT NULL,
  "auto" boolean NOT NULL DEFAULT false,
  "status" text NOT NULL DEFAULT 'PENDING',
  "reversal_amount" bigint NOT NULL DEFAULT 0,
  "reversal_idempotency_key" uuid NOT NULL DEFAULT gen_random_uuid(),
  "reversal_called_at" timestamptz NULL,
  "reversal_external_id" text NULL,
  "idempotency_key" uuid NOT NULL DEFAULT gen_random_uuid(),
  "refund_called_at" timestamptz NULL,
  "external_refund_id" text NULL,
  "failure_reason" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "refunds_payment_id_fkey" FOREIGN KEY ("payment_id") REFERENCES "payment"."payments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "refunds_amount_check" CHECK (amount > 0),
  CONSTRAINT "refunds_external_id_check" CHECK ((external_refund_id IS NULL) OR ((char_length(external_refund_id) >= 1) AND (char_length(external_refund_id) <= 255))),
  CONSTRAINT "refunds_failure_check" CHECK ((status = 'FAILED'::text) = (failure_reason IS NOT NULL)),
  CONSTRAINT "refunds_failure_reason_check" CHECK ((failure_reason IS NULL) OR ((char_length(failure_reason) >= 1) AND (char_length(failure_reason) <= 200))),
  CONSTRAINT "refunds_refund_called_check" CHECK ((status <> ALL (ARRAY['REFUNDING'::text, 'SUCCEEDED'::text])) OR (refund_called_at IS NOT NULL)),
  CONSTRAINT "refunds_request_key_check" CHECK ((char_length(request_key) >= 1) AND (char_length(request_key) <= 128)),
  CONSTRAINT "refunds_reversal_amount_check" CHECK ((reversal_amount >= 0) AND (reversal_amount <= amount)),
  CONSTRAINT "refunds_reversal_called_check" CHECK ((reversal_external_id IS NULL) OR (reversal_called_at IS NOT NULL)),
  CONSTRAINT "refunds_reversal_external_id_check" CHECK ((reversal_external_id IS NULL) OR ((char_length(reversal_external_id) >= 1) AND (char_length(reversal_external_id) <= 255))),
  CONSTRAINT "refunds_reversed_check" CHECK ((status <> ALL (ARRAY['REVERSED'::text, 'REFUNDING'::text, 'SUCCEEDED'::text])) OR (reversal_amount = 0) OR (reversal_external_id IS NOT NULL)),
  CONSTRAINT "refunds_reversing_check" CHECK ((status <> 'REVERSING'::text) OR (reversal_amount > 0)),
  CONSTRAINT "refunds_status_check" CHECK (status = ANY (ARRAY['PENDING'::text, 'REVERSING'::text, 'REVERSED'::text, 'REFUNDING'::text, 'SUCCEEDED'::text, 'FAILED'::text])),
  CONSTRAINT "refunds_succeeded_check" CHECK ((status = 'SUCCEEDED'::text) = (external_refund_id IS NOT NULL))
);
-- Create index "refunds_external" to table: "refunds"
CREATE UNIQUE INDEX "refunds_external" ON "payment"."refunds" ("external_refund_id") WHERE (external_refund_id IS NOT NULL);
-- Create index "refunds_payment" to table: "refunds"
CREATE INDEX "refunds_payment" ON "payment"."refunds" ("payment_id");
-- Create index "refunds_request" to table: "refunds"
CREATE UNIQUE INDEX "refunds_request" ON "payment"."refunds" ("payment_id", "request_key");
-- Create "transfers" table
CREATE TABLE "payment"."transfers" (
  "id" uuid NOT NULL,
  "payment_id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "amount" bigint NOT NULL,
  "reversed_amount" bigint NOT NULL DEFAULT 0,
  "destination_account_id" text NOT NULL,
  "status" text NOT NULL DEFAULT 'PENDING',
  "external_transfer_id" text NULL,
  "failure_reason" text NULL,
  "idempotency_key" uuid NOT NULL DEFAULT gen_random_uuid(),
  "called_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "transfers_payment_id_fkey" FOREIGN KEY ("payment_id") REFERENCES "payment"."payments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "transfers_amount_check" CHECK (amount > 0),
  CONSTRAINT "transfers_called_check" CHECK ((external_transfer_id IS NULL) OR (called_at IS NOT NULL)),
  CONSTRAINT "transfers_destination_check" CHECK ((char_length(destination_account_id) >= 1) AND (char_length(destination_account_id) <= 255)),
  CONSTRAINT "transfers_external_id_check" CHECK ((external_transfer_id IS NULL) OR ((char_length(external_transfer_id) >= 1) AND (char_length(external_transfer_id) <= 255))),
  CONSTRAINT "transfers_failure_check" CHECK ((status = 'FAILED'::text) = (failure_reason IS NOT NULL)),
  CONSTRAINT "transfers_failure_reason_check" CHECK ((failure_reason IS NULL) OR ((char_length(failure_reason) >= 1) AND (char_length(failure_reason) <= 200))),
  CONSTRAINT "transfers_reversal_only_when_succeeded_check" CHECK ((status = 'SUCCEEDED'::text) OR (reversed_amount = 0)),
  CONSTRAINT "transfers_reversed_check" CHECK ((reversed_amount >= 0) AND (reversed_amount <= amount)),
  CONSTRAINT "transfers_status_check" CHECK (status = ANY (ARRAY['PENDING'::text, 'SUCCEEDED'::text, 'FAILED'::text])),
  CONSTRAINT "transfers_store_id_check" CHECK (store_id > 0),
  CONSTRAINT "transfers_succeeded_check" CHECK ((status = 'SUCCEEDED'::text) = (external_transfer_id IS NOT NULL))
);
-- Create index "transfers_active_payment" to table: "transfers"
CREATE UNIQUE INDEX "transfers_active_payment" ON "payment"."transfers" ("payment_id") WHERE (status = ANY (ARRAY['PENDING'::text, 'SUCCEEDED'::text]));
-- Create index "transfers_external" to table: "transfers"
CREATE UNIQUE INDEX "transfers_external" ON "payment"."transfers" ("external_transfer_id") WHERE (external_transfer_id IS NOT NULL);
