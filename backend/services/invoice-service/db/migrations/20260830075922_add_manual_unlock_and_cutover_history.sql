-- Create "invoice_manual_unlocks" table
CREATE TABLE "invoice_manual_unlocks" (
  "id" bigint NOT NULL,
  "operation_id" bigint NOT NULL,
  "status" character varying(24) NOT NULL DEFAULT 'AWAITING_SECOND_APPROVAL',
  "requested_by_account_id" bigint NOT NULL,
  "request_reason" text NOT NULL,
  "provider_check_evidence" text NOT NULL,
  "provider_checked_at" timestamptz NOT NULL,
  "requested_at" timestamptz NOT NULL DEFAULT now(),
  "expires_at" timestamptz NOT NULL,
  "approved_by_account_id" bigint NULL,
  "decision_reason" text NULL,
  "decided_at" timestamptz NULL,
  "version" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invmu_operation" FOREIGN KEY ("operation_id") REFERENCES "invoice_operations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_invmu_decided_pair" CHECK (((status)::text = ANY ((ARRAY['APPROVED'::character varying, 'REJECTED'::character varying])::text[])) = ((approved_by_account_id IS NOT NULL) AND (decided_at IS NOT NULL))),
  CONSTRAINT "chk_invmu_evidence_len" CHECK ((length(provider_check_evidence) >= 1) AND (length(provider_check_evidence) <= 1000)),
  CONSTRAINT "chk_invmu_expiry_window" CHECK (expires_at > requested_at),
  CONSTRAINT "chk_invmu_reason_len" CHECK (length(request_reason) >= 1),
  CONSTRAINT "chk_invmu_reject_reason" CHECK (((status)::text <> 'REJECTED'::text) OR ((decision_reason IS NOT NULL) AND (length(decision_reason) >= 1))),
  CONSTRAINT "chk_invmu_status" CHECK ((status)::text = ANY ((ARRAY['AWAITING_SECOND_APPROVAL'::character varying, 'APPROVED'::character varying, 'REJECTED'::character varying, 'EXPIRED'::character varying])::text[])),
  CONSTRAINT "chk_invmu_two_person" CHECK ((approved_by_account_id IS NULL) OR (approved_by_account_id <> requested_by_account_id)),
  CONSTRAINT "chk_invmu_version_nonneg" CHECK (version >= 0)
);
-- Create index "idx_invmu_operation" to table: "invoice_manual_unlocks"
CREATE INDEX "idx_invmu_operation" ON "invoice_manual_unlocks" ("operation_id", "created_at" DESC);
-- Create index "uq_invmu_open_per_operation" to table: "invoice_manual_unlocks"
CREATE UNIQUE INDEX "uq_invmu_open_per_operation" ON "invoice_manual_unlocks" ("operation_id") WHERE ((status)::text = 'AWAITING_SECOND_APPROVAL'::text);
-- Create "invoice_provider_cutovers" table
CREATE TABLE "invoice_provider_cutovers" (
  "id" bigserial NOT NULL,
  "provider_account_id" bigint NOT NULL,
  "issuer_id" bigint NOT NULL,
  "account_version" bigint NOT NULL,
  "cutover_at" timestamptz NOT NULL,
  "deactivated_at" timestamptz NULL,
  "mode" character varying(24) NOT NULL DEFAULT 'AMEGO_AUTO',
  "created_by_account_id" bigint NOT NULL,
  "reason" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invpc_account" FOREIGN KEY ("provider_account_id") REFERENCES "invoice_provider_accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_invpc_issuer" FOREIGN KEY ("issuer_id") REFERENCES "invoice_issuers" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_invpc_deactivated_after" CHECK ((deactivated_at IS NULL) OR (deactivated_at >= created_at)),
  CONSTRAINT "chk_invpc_mode" CHECK ((mode)::text = 'AMEGO_AUTO'::text),
  CONSTRAINT "chk_invpc_version_nonneg" CHECK (account_version >= 0)
);
-- Create index "idx_invpc_account_time" to table: "invoice_provider_cutovers"
CREATE INDEX "idx_invpc_account_time" ON "invoice_provider_cutovers" ("provider_account_id", "created_at" DESC);
-- Create index "uq_invpc_open_per_account" to table: "invoice_provider_cutovers"
CREATE UNIQUE INDEX "uq_invpc_open_per_account" ON "invoice_provider_cutovers" ("provider_account_id") WHERE (deactivated_at IS NULL);
