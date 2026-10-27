-- Create "invoice_status_history" table
CREATE TABLE "invoice_status_history" (
  "id" bigserial NOT NULL,
  "subject_type" character varying(24) NOT NULL,
  "subject_id" bigint NOT NULL,
  "from_status" character varying(32) NULL,
  "to_status" character varying(32) NOT NULL,
  "reason_code" character varying(48) NULL,
  "reason_detail" text NULL,
  "actor_type" character varying(16) NOT NULL,
  "actor_account_id" bigint NULL,
  "correlation_id" character varying(64) NULL,
  "occurred_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_invhist_actor" CHECK ((actor_type)::text = ANY ((ARRAY['SYSTEM'::character varying, 'ADMIN'::character varying, 'SELLER'::character varying, 'BUYER'::character varying, 'PROVIDER'::character varying])::text[])),
  CONSTRAINT "chk_invhist_subject" CHECK ((subject_type)::text = ANY ((ARRAY['DOCUMENT'::character varying, 'ADJUSTMENT'::character varying, 'OPERATION'::character varying, 'CORRECTION_REQUEST'::character varying, 'TAX_PROFILE'::character varying, 'PROVIDER_ACCOUNT'::character varying, 'COMPLAINT'::character varying])::text[]))
);
-- Create index "idx_invhist_subject" to table: "invoice_status_history"
CREATE INDEX "idx_invhist_subject" ON "invoice_status_history" ("subject_type", "subject_id", "occurred_at");
-- Create "invoice_admin_audit_logs" table
CREATE TABLE "invoice_admin_audit_logs" (
  "id" bigserial NOT NULL,
  "actor_account_id" bigint NOT NULL,
  "action" character varying(64) NOT NULL,
  "resource_type" character varying(32) NOT NULL,
  "resource_id" bigint NOT NULL,
  "before_state" jsonb NULL,
  "after_state" jsonb NULL,
  "reason_code" character varying(48) NOT NULL,
  "reason_detail" text NOT NULL,
  "approval_actor_account_id" bigint NULL,
  "trace_id" character varying(64) NULL,
  "request_id" character varying(64) NULL,
  "correlation_id" character varying(64) NULL,
  "client_ip" character varying(45) NULL,
  "user_agent" character varying(512) NULL,
  "occurred_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_invadmaudit_reason" CHECK (length(reason_detail) >= 1),
  CONSTRAINT "chk_invadmaudit_two_person" CHECK ((approval_actor_account_id IS NULL) OR (approval_actor_account_id <> actor_account_id))
);
-- Create index "idx_invadmaudit_actor" to table: "invoice_admin_audit_logs"
CREATE INDEX "idx_invadmaudit_actor" ON "invoice_admin_audit_logs" ("actor_account_id", "occurred_at" DESC);
-- Create index "idx_invadmaudit_resource" to table: "invoice_admin_audit_logs"
CREATE INDEX "idx_invadmaudit_resource" ON "invoice_admin_audit_logs" ("resource_type", "resource_id", "occurred_at" DESC);
-- Create index "uq_invadmaudit_client_request" to table: "invoice_admin_audit_logs"
CREATE UNIQUE INDEX "uq_invadmaudit_client_request" ON "invoice_admin_audit_logs" ("actor_account_id", "action", "request_id") WHERE (request_id IS NOT NULL);
-- Create "invoice_business_day_mirror" table
CREATE TABLE "invoice_business_day_mirror" (
  "holiday_date" date NOT NULL,
  "synced_at" timestamptz NOT NULL,
  PRIMARY KEY ("holiday_date")
);
-- Set comment to table: "invoice_business_day_mirror"
COMMENT ON TABLE "invoice_business_day_mirror" IS '台灣國定假日唯讀鏡像。owner 是 shipping-service 的 holidays 表；本表每日由 INV.gov 整段覆蓋。禁止人工寫入、禁止 ADM 編輯、禁止在本服務加 seed migration。';
-- Create "invoice_outbox_events" table
CREATE TABLE "invoice_outbox_events" (
  "id" bigserial NOT NULL,
  "aggregate_type" character varying(24) NOT NULL,
  "aggregate_id" bigint NOT NULL,
  "event_type" character varying(64) NOT NULL,
  "exchange" character varying(64) NOT NULL DEFAULT 'hawkr.invoice',
  "routing_key" character varying(128) NOT NULL,
  "payload" jsonb NOT NULL,
  "status" character varying(16) NOT NULL DEFAULT 'pending',
  "attempt_count" integer NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "last_error" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_invoutbox_attempts" CHECK (attempt_count >= 0),
  CONSTRAINT "chk_invoutbox_status" CHECK ((status)::text = ANY ((ARRAY['pending'::character varying, 'done'::character varying, 'failed'::character varying])::text[]))
);
-- Create index "idx_invoutbox_pending" to table: "invoice_outbox_events"
CREATE INDEX "idx_invoutbox_pending" ON "invoice_outbox_events" ("next_attempt_at") WHERE ((status)::text = 'pending'::text);
-- Create "invoice_issuers" table
CREATE TABLE "invoice_issuers" (
  "id" bigint NOT NULL,
  "issuer_type" character varying(16) NOT NULL,
  "issuer_ref_id" bigint NOT NULL,
  "display_name" character varying(120) NOT NULL,
  "status" character varying(16) NOT NULL DEFAULT 'active',
  "retired_at" timestamptz NULL,
  "version" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_invoice_issuers_ref" CHECK ((((issuer_type)::text = 'PLATFORM'::text) AND (issuer_ref_id = 0)) OR (((issuer_type)::text = 'SELLER'::text) AND (issuer_ref_id > 0))),
  CONSTRAINT "chk_invoice_issuers_retired" CHECK (((status)::text <> 'retired'::text) OR (retired_at IS NOT NULL)),
  CONSTRAINT "chk_invoice_issuers_status" CHECK ((status)::text = ANY ((ARRAY['active'::character varying, 'retired'::character varying])::text[])),
  CONSTRAINT "chk_invoice_issuers_type" CHECK ((issuer_type)::text = ANY ((ARRAY['SELLER'::character varying, 'PLATFORM'::character varying])::text[])),
  CONSTRAINT "chk_invoice_issuers_version_nonneg" CHECK (version >= 0)
);
-- Create index "uq_invoice_issuers_ref" to table: "invoice_issuers"
CREATE UNIQUE INDEX "uq_invoice_issuers_ref" ON "invoice_issuers" ("issuer_type", "issuer_ref_id");
-- Create "invoice_inbox_events" table
CREATE TABLE "invoice_inbox_events" (
  "id" bigserial NOT NULL,
  "event_id" character varying(64) NOT NULL,
  "event_type" character varying(64) NOT NULL,
  "routing_key" character varying(128) NOT NULL,
  "exchange" character varying(64) NOT NULL,
  "business_dedup_key" character varying(160) NOT NULL,
  "aggregate_ref" character varying(64) NULL,
  "payload" jsonb NOT NULL,
  "status" character varying(16) NOT NULL DEFAULT 'RECEIVED',
  "disposition" character varying(40) NULL,
  "disposition_reason" text NULL,
  "attempt_count" integer NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "deferred_until" timestamptz NULL,
  "last_error" text NULL,
  "occurred_at" timestamptz NOT NULL,
  "received_at" timestamptz NOT NULL DEFAULT now(),
  "processed_at" timestamptz NULL,
  "payload_purged_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_invinbox_attempts" CHECK (attempt_count >= 0),
  CONSTRAINT "chk_invinbox_skip_reason" CHECK (((status)::text <> 'SKIPPED'::text) OR (disposition IS NOT NULL)),
  CONSTRAINT "chk_invinbox_status" CHECK ((status)::text = ANY ((ARRAY['RECEIVED'::character varying, 'DEFERRED'::character varying, 'PROCESSED'::character varying, 'SKIPPED'::character varying, 'FAILED'::character varying])::text[]))
);
-- Create index "idx_invinbox_deferred_aggregate" to table: "invoice_inbox_events"
CREATE INDEX "idx_invinbox_deferred_aggregate" ON "invoice_inbox_events" ("aggregate_ref") WHERE ((status)::text = 'DEFERRED'::text);
-- Create index "idx_invinbox_due" to table: "invoice_inbox_events"
CREATE INDEX "idx_invinbox_due" ON "invoice_inbox_events" ("next_attempt_at") WHERE ((status)::text = ANY ((ARRAY['RECEIVED'::character varying, 'DEFERRED'::character varying, 'FAILED'::character varying])::text[]));
-- Create index "uq_invinbox_business_key" to table: "invoice_inbox_events"
CREATE UNIQUE INDEX "uq_invinbox_business_key" ON "invoice_inbox_events" ("event_type", "business_dedup_key");
-- Create index "uq_invinbox_event_id" to table: "invoice_inbox_events"
CREATE UNIQUE INDEX "uq_invinbox_event_id" ON "invoice_inbox_events" ("event_id");
-- Create "buyer_invoice_preferences" table
CREATE TABLE "buyer_invoice_preferences" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "preference_kind" character varying(20) NOT NULL,
  "carrier_type" character varying(6) NULL,
  "carrier_value_ciphertext" text NULL,
  "carrier_value_hash" character varying(64) NULL,
  "carrier_value_mask" character varying(16) NULL,
  "tax_id" character varying(8) NULL,
  "tax_title" character varying(60) NULL,
  "donation_code" character varying(7) NULL,
  "label" character varying(30) NULL,
  "is_default" boolean NOT NULL DEFAULT false,
  "superseded_at" timestamptz NULL,
  "superseded_by_id" bigint NULL,
  "deleted_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_bip_superseded" FOREIGN KEY ("superseded_by_id") REFERENCES "buyer_invoice_preferences" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_bip_default_alive" CHECK ((NOT is_default) OR ((superseded_at IS NULL) AND (deleted_at IS NULL))),
  CONSTRAINT "chk_bip_donation_code" CHECK ((donation_code IS NULL) OR ((donation_code)::text ~ '^[0-9]{3,7}$'::text)),
  CONSTRAINT "chk_bip_hash_pair" CHECK ((carrier_value_ciphertext IS NULL) = (carrier_value_hash IS NULL)),
  CONSTRAINT "chk_bip_kind" CHECK ((preference_kind)::text = ANY ((ARRAY['MOBILE_BARCODE'::character varying, 'CITIZEN_CERT'::character varying, 'TAX_ID'::character varying, 'DONATION'::character varying])::text[])),
  CONSTRAINT "chk_bip_shape" CHECK ((((preference_kind)::text = 'MOBILE_BARCODE'::text) AND ((COALESCE(carrier_type, ''::character varying))::text = '3J0002'::text) AND (carrier_value_ciphertext IS NOT NULL) AND (tax_id IS NULL) AND (donation_code IS NULL)) OR (((preference_kind)::text = 'CITIZEN_CERT'::text) AND ((COALESCE(carrier_type, ''::character varying))::text = 'CQ0001'::text) AND (carrier_value_ciphertext IS NOT NULL) AND (tax_id IS NULL) AND (donation_code IS NULL)) OR (((preference_kind)::text = 'TAX_ID'::text) AND (carrier_type IS NULL) AND (carrier_value_ciphertext IS NULL) AND (tax_id IS NOT NULL) AND (tax_title IS NOT NULL) AND (donation_code IS NULL)) OR (((preference_kind)::text = 'DONATION'::text) AND (carrier_type IS NULL) AND (carrier_value_ciphertext IS NULL) AND (tax_id IS NULL) AND (donation_code IS NOT NULL))),
  CONSTRAINT "chk_bip_superseded_pair" CHECK ((superseded_at IS NULL) = (superseded_by_id IS NULL)),
  CONSTRAINT "chk_bip_tax_id" CHECK ((tax_id IS NULL) OR ((tax_id)::text ~ '^[0-9]{8}$'::text))
);
-- Create index "idx_bip_buyer" to table: "buyer_invoice_preferences"
CREATE INDEX "idx_bip_buyer" ON "buyer_invoice_preferences" ("buyer_id", "created_at" DESC) WHERE ((deleted_at IS NULL) AND (superseded_at IS NULL));
-- Create index "uq_bip_carrier_dedup" to table: "buyer_invoice_preferences"
CREATE UNIQUE INDEX "uq_bip_carrier_dedup" ON "buyer_invoice_preferences" ("buyer_id", "carrier_type", "carrier_value_hash") WHERE ((carrier_value_hash IS NOT NULL) AND (deleted_at IS NULL) AND (superseded_at IS NULL));
-- Create index "uq_bip_default" to table: "buyer_invoice_preferences"
CREATE UNIQUE INDEX "uq_bip_default" ON "buyer_invoice_preferences" ("buyer_id") WHERE (is_default AND (deleted_at IS NULL) AND (superseded_at IS NULL));
-- Create index "uq_bip_taxid_dedup" to table: "buyer_invoice_preferences"
CREATE UNIQUE INDEX "uq_bip_taxid_dedup" ON "buyer_invoice_preferences" ("buyer_id", "tax_id") WHERE ((tax_id IS NOT NULL) AND (deleted_at IS NULL) AND (superseded_at IS NULL));
-- Create "invoice_documents" table
CREATE TABLE "invoice_documents" (
  "id" bigint NOT NULL,
  "issuer_id" bigint NOT NULL,
  "issuer_name_snapshot" character varying(120) NOT NULL,
  "issuer_tax_id_snapshot" character varying(8) NULL,
  "source_type" character varying(16) NOT NULL,
  "source_ref" character varying(64) NULL,
  "trigger_type" character varying(16) NOT NULL,
  "recipient_type" character varying(16) NOT NULL,
  "recipient_ref_id" bigint NULL,
  "recipient_name" character varying(60) NOT NULL,
  "recipient_tax_id" character varying(10) NOT NULL DEFAULT '0000000000',
  "recipient_email_ciphertext" text NULL,
  "recipient_email_hash" character varying(64) NULL,
  "recipient_phone_ciphertext" text NULL,
  "carrier_type" character varying(6) NULL,
  "carrier_id1_ciphertext" text NULL,
  "carrier_id2_ciphertext" text NULL,
  "carrier_value_hash" character varying(64) NULL,
  "donation_code" character varying(7) NULL,
  "document_kind" character varying(16) NOT NULL,
  "fulfillment_mode" character varying(24) NOT NULL,
  "status" character varying(24) NOT NULL DEFAULT 'PENDING',
  "not_applicable_reason" character varying(32) NULL,
  "manual_reason_code" character varying(32) NULL,
  "cancel_reason_code" character varying(48) NULL,
  "cancel_reason" text NULL,
  "snapshot_version" integer NOT NULL DEFAULT 1,
  "issue_attempt_seq" integer NOT NULL DEFAULT 1,
  "provider_account_version" bigint NULL,
  "superseded_by_id" bigint NULL,
  "original_document_id" bigint NULL,
  "sales_amount" bigint NOT NULL,
  "free_tax_sales_amount" bigint NOT NULL DEFAULT 0,
  "zero_tax_sales_amount" bigint NOT NULL DEFAULT 0,
  "tax_amount" bigint NOT NULL DEFAULT 0,
  "total_amount" bigint NOT NULL,
  "tax_type" smallint NOT NULL DEFAULT 1,
  "tax_rate_bps" integer NOT NULL DEFAULT 500,
  "allowance_total_amount" bigint NOT NULL DEFAULT 0,
  "voided_at" timestamptz NULL,
  "provider" character varying(16) NULL,
  "provider_request_no" character varying(40) NULL,
  "provider_invoice_no" character varying(10) NULL,
  "provider_random_code" character varying(4) NULL,
  "provider_issued_at" timestamptz NULL,
  "external_invoice_no" character varying(10) NULL,
  "external_invoice_date" date NULL,
  "external_reported_at" timestamptz NULL,
  "provider_upload_status" smallint NULL,
  "provider_upload_checked_at" timestamptz NULL,
  "upload_recheck_next_at" timestamptz NULL,
  "upload_recheck_attempts" smallint NOT NULL DEFAULT 0,
  "receipt_no" character varying(20) NULL,
  "receipt_authorization_version" character varying(32) NULL,
  "pdf_object_key" character varying(512) NULL,
  "pdf_fetch_next_at" timestamptz NULL,
  "pdf_fetch_attempts" smallint NOT NULL DEFAULT 0,
  "invoice_date_taipei" date NULL,
  "tax_period_code" character(5) NULL,
  "issue_not_before" timestamptz NOT NULL,
  "statutory_deadline_at" timestamptz NULL,
  "source_snapshot" jsonb NULL,
  "version" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invdoc_issuer" FOREIGN KEY ("issuer_id") REFERENCES "invoice_issuers" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_invdoc_original" FOREIGN KEY ("original_document_id") REFERENCES "invoice_documents" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_invdoc_superseded" FOREIGN KEY ("superseded_by_id") REFERENCES "invoice_documents" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_invdoc_allowance_cap" CHECK ((allowance_total_amount >= 0) AND (allowance_total_amount <= total_amount)),
  CONSTRAINT "chk_invdoc_amounts_nonneg" CHECK ((sales_amount >= 0) AND (free_tax_sales_amount >= 0) AND (zero_tax_sales_amount >= 0) AND (tax_amount >= 0) AND (total_amount >= 0)),
  CONSTRAINT "chk_invdoc_b2b" CHECK (((document_kind)::text <> 'B2B'::text) OR ((recipient_tax_id)::text ~ '^[0-9]{8}$'::text)),
  CONSTRAINT "chk_invdoc_b2c" CHECK (((document_kind)::text <> 'B2C'::text) OR ((tax_amount = 0) AND ((recipient_tax_id)::text = '0000000000'::text))),
  CONSTRAINT "chk_invdoc_carrier_pair" CHECK ((carrier_type IS NULL) = (carrier_id1_ciphertext IS NULL)),
  CONSTRAINT "chk_invdoc_deadline" CHECK ((statutory_deadline_at IS NULL) OR (statutory_deadline_at > issue_not_before)),
  CONSTRAINT "chk_invdoc_deadline_presence" CHECK (((document_kind)::text = ANY ((ARRAY['B2C'::character varying, 'B2B'::character varying])::text[])) = (statutory_deadline_at IS NOT NULL)),
  CONSTRAINT "chk_invdoc_donation" CHECK ((donation_code IS NULL) OR ((donation_code)::text ~ '^[0-9]{3,7}$'::text)),
  CONSTRAINT "chk_invdoc_exclusive" CHECK (((
CASE
    WHEN ((recipient_tax_id)::text <> '0000000000'::text) THEN 1
    ELSE 0
END +
CASE
    WHEN (carrier_type IS NOT NULL) THEN 1
    ELSE 0
END) +
CASE
    WHEN (donation_code IS NOT NULL) THEN 1
    ELSE 0
END) <= 1),
  CONSTRAINT "chk_invdoc_external_excl" CHECK ((external_invoice_no IS NULL) OR (provider_invoice_no IS NULL)),
  CONSTRAINT "chk_invdoc_external_no" CHECK ((((status)::text = 'RESOLVED_EXTERNALLY'::text) = (external_invoice_no IS NOT NULL)) AND (((status)::text = 'RESOLVED_EXTERNALLY'::text) = (external_invoice_date IS NOT NULL)) AND (((status)::text = 'RESOLVED_EXTERNALLY'::text) = (external_reported_at IS NOT NULL))),
  CONSTRAINT "chk_invdoc_external_no_shape" CHECK ((external_invoice_no IS NULL) OR ((external_invoice_no)::text ~ '^[A-Z]{2}[0-9]{8}$'::text)),
  CONSTRAINT "chk_invdoc_external_reported_after" CHECK ((external_reported_at IS NULL) OR (external_reported_at >= issue_not_before)),
  CONSTRAINT "chk_invdoc_issued_has_no" CHECK (((fulfillment_mode)::text <> 'AMEGO_AUTO'::text) OR ((status)::text <> 'ISSUED'::text) OR (provider_invoice_no IS NOT NULL)),
  CONSTRAINT "chk_invdoc_kind" CHECK ((document_kind)::text = ANY ((ARRAY['B2C'::character varying, 'B2B'::character varying, 'RECEIPT'::character varying])::text[])),
  CONSTRAINT "chk_invdoc_kind_mode" CHECK (((document_kind)::text = 'RECEIPT'::text) = ((fulfillment_mode)::text = 'PLATFORM_RECEIPT'::text)),
  CONSTRAINT "chk_invdoc_manual_reason" CHECK (((status)::text <> 'AWAITING_MANUAL'::text) OR (manual_reason_code IS NOT NULL)),
  CONSTRAINT "chk_invdoc_mode" CHECK ((fulfillment_mode)::text = ANY ((ARRAY['AMEGO_AUTO'::character varying, 'SELLER_SELF_ISSUE'::character varying, 'PLATFORM_RECEIPT'::character varying, 'LEGACY_MANUAL'::character varying])::text[])),
  CONSTRAINT "chk_invdoc_na_reason" CHECK (((status)::text = 'NOT_APPLICABLE'::text) = (not_applicable_reason IS NOT NULL)),
  CONSTRAINT "chk_invdoc_pdf_attempts" CHECK (pdf_fetch_attempts >= 0),
  CONSTRAINT "chk_invdoc_period" CHECK ((tax_period_code IS NULL) OR (tax_period_code ~ '^[0-9]{5}$'::text)),
  CONSTRAINT "chk_invdoc_receipt" CHECK (((document_kind)::text = 'RECEIPT'::text) = (receipt_no IS NOT NULL)),
  CONSTRAINT "chk_invdoc_receipt_no_provider" CHECK (((document_kind)::text <> 'RECEIPT'::text) OR ((provider_invoice_no IS NULL) AND (provider IS NULL))),
  CONSTRAINT "chk_invdoc_recipient" CHECK ((recipient_type)::text = ANY ((ARRAY['BUYER'::character varying, 'SELLER'::character varying, 'EXTERNAL'::character varying])::text[])),
  CONSTRAINT "chk_invdoc_request_no_len" CHECK ((provider_request_no IS NULL) OR (length((provider_request_no)::text) <= 40)),
  CONSTRAINT "chk_invdoc_self_issue" CHECK (((fulfillment_mode)::text <> ALL ((ARRAY['SELLER_SELF_ISSUE'::character varying, 'LEGACY_MANUAL'::character varying])::text[])) OR ((status)::text = ANY ((ARRAY['NOT_APPLICABLE'::character varying, 'RESOLVED_EXTERNALLY'::character varying, 'CANCELLED_BEFORE_ISSUE'::character varying])::text[]))),
  CONSTRAINT "chk_invdoc_source" CHECK ((source_type)::text = ANY ((ARRAY['ORDER'::character varying, 'MANUAL'::character varying])::text[])),
  CONSTRAINT "chk_invdoc_source_ref" CHECK (((source_type)::text = 'ORDER'::text) = (source_ref IS NOT NULL)),
  CONSTRAINT "chk_invdoc_status" CHECK ((status)::text = ANY ((ARRAY['PENDING'::character varying, 'ISSUING'::character varying, 'ISSUED'::character varying, 'ISSUE_FAILED'::character varying, 'AWAITING_MANUAL'::character varying, 'VOIDING'::character varying, 'VOIDED'::character varying, 'SUPERSEDED'::character varying, 'CANCELLED_BEFORE_ISSUE'::character varying, 'NOT_APPLICABLE'::character varying, 'RESOLVED_EXTERNALLY'::character varying])::text[])),
  CONSTRAINT "chk_invdoc_tax_type" CHECK (tax_type = ANY (ARRAY[1, 2, 3, 9])),
  CONSTRAINT "chk_invdoc_taxid_shape" CHECK ((recipient_tax_id)::text ~ '^([0-9]{8}|0000000000)$'::text),
  CONSTRAINT "chk_invdoc_total" CHECK (total_amount = (((sales_amount + free_tax_sales_amount) + zero_tax_sales_amount) + tax_amount)),
  CONSTRAINT "chk_invdoc_trigger" CHECK ((trigger_type)::text = ANY ((ARRAY['AUTO'::character varying, 'ADM_MANUAL'::character varying])::text[])),
  CONSTRAINT "chk_invdoc_upload_recheck" CHECK ((upload_recheck_attempts >= 0) AND ((upload_recheck_next_at IS NULL) OR (provider_upload_status IS NULL) OR (provider_upload_status = ANY (ARRAY[1, 2, 3, 31, 32])))),
  CONSTRAINT "chk_invdoc_versions" CHECK ((snapshot_version >= 1) AND (issue_attempt_seq >= 1) AND (version >= 0)),
  CONSTRAINT "chk_invdoc_voided_pair" CHECK (((status)::text = 'VOIDED'::text) = (voided_at IS NOT NULL))
);
-- Create index "idx_invdoc_deadline" to table: "invoice_documents"
CREATE INDEX "idx_invdoc_deadline" ON "invoice_documents" ("statutory_deadline_at") WHERE ((status)::text = ANY ((ARRAY['PENDING'::character varying, 'ISSUING'::character varying, 'ISSUE_FAILED'::character varying, 'AWAITING_MANUAL'::character varying])::text[]));
-- Create index "idx_invdoc_issuer_status" to table: "invoice_documents"
CREATE INDEX "idx_invdoc_issuer_status" ON "invoice_documents" ("issuer_id", "status", "created_at" DESC);
-- Create index "idx_invdoc_pdf_fetch" to table: "invoice_documents"
CREATE INDEX "idx_invdoc_pdf_fetch" ON "invoice_documents" ("pdf_fetch_next_at") WHERE ((pdf_fetch_next_at IS NOT NULL) AND (pdf_object_key IS NULL));
-- Create index "idx_invdoc_recipient" to table: "invoice_documents"
CREATE INDEX "idx_invdoc_recipient" ON "invoice_documents" ("recipient_type", "recipient_ref_id", "status", "created_at" DESC);
-- Create index "idx_invdoc_source_ref" to table: "invoice_documents"
CREATE INDEX "idx_invdoc_source_ref" ON "invoice_documents" ("source_ref") WHERE ((source_type)::text = 'ORDER'::text);
-- Create index "idx_invdoc_upload_recheck" to table: "invoice_documents"
CREATE INDEX "idx_invdoc_upload_recheck" ON "invoice_documents" ("upload_recheck_next_at") WHERE (((status)::text = 'ISSUED'::text) AND (upload_recheck_next_at IS NOT NULL) AND ((provider_upload_status IS NULL) OR (provider_upload_status = ANY (ARRAY[1, 2, 3, 31, 32]))));
-- Create index "uq_invdoc_order_identity" to table: "invoice_documents"
CREATE UNIQUE INDEX "uq_invdoc_order_identity" ON "invoice_documents" ("issuer_id", "source_ref", "document_kind", "snapshot_version", "issue_attempt_seq") WHERE ((source_type)::text = 'ORDER'::text);
-- Create index "uq_invdoc_provider_invoice_no" to table: "invoice_documents"
CREATE UNIQUE INDEX "uq_invdoc_provider_invoice_no" ON "invoice_documents" ("provider", "provider_invoice_no") WHERE (provider_invoice_no IS NOT NULL);
-- Create index "uq_invdoc_provider_request_no" to table: "invoice_documents"
CREATE UNIQUE INDEX "uq_invdoc_provider_request_no" ON "invoice_documents" ("provider_request_no") WHERE (provider_request_no IS NOT NULL);
-- Create index "uq_invdoc_receipt_no" to table: "invoice_documents"
CREATE UNIQUE INDEX "uq_invdoc_receipt_no" ON "invoice_documents" ("receipt_no") WHERE (receipt_no IS NOT NULL);
-- Set comment to table: "invoice_documents"
COMMENT ON TABLE "invoice_documents" IS '稅務文件（發票／收據）。法定保存 5 年，列為 PII 清理例外：帳號刪除不得連帶刪除本表任何一列，只關閉對外查詢（授權層）。本服務不存在任何 DELETE FROM invoice_documents 的路徑（不變式 I-40）。';
-- Set comment to column: "recipient_name" on table: "invoice_documents"
COMMENT ON COLUMN "invoice_documents"."recipient_name" IS 'AMEGO BuyerName。永不寫入自然人真實姓名：B2C 恆為常數「消費者」、B2B 為公司抬頭、RECEIPT／MANUAL 為 ADM 或賣家手填抬頭。因此本欄是明文且不進遮罩清單。';
-- Set comment to column: "external_invoice_no" on table: "invoice_documents"
COMMENT ON COLUMN "invoice_documents"."external_invoice_no" IS '賣家自行開立並回報的發票號碼（status=RESOLVED_EXTERNALLY 專用）。與 provider_invoice_no 互斥且永不互相 fallback：平台不冒充開立人。';
-- Set comment to column: "statutory_deadline_at" on table: "invoice_documents"
COMMENT ON COLUMN "invoice_documents"."statutory_deadline_at" IS '法定存證期限（自然日）：自 invoice_date_taipei 之翌日起算，B2C 2 日、B2B 7 日，末日 23:59:59 Asia/Taipei，不因假日延長；RECEIPT 無法定期限故為 NULL。禁止以 +48h 計算。本欄是權威值，invoice_operations 上的同名欄是它的反正規化副本（I-52）。';
-- Set comment to column: "source_snapshot" on table: "invoice_documents"
COMMENT ON COLUMN "invoice_documents"."source_snapshot" IS '開票判定依據的白名單快照（已遮罩）。禁存收件人姓名／電話／地址／買家 email／載具值。';
-- Create "invoice_adjustments" table
CREATE TABLE "invoice_adjustments" (
  "id" bigint NOT NULL,
  "document_id" bigint NOT NULL,
  "adjustment_type" character varying(16) NOT NULL,
  "status" character varying(32) NOT NULL DEFAULT 'PENDING',
  "trigger_kind" character varying(24) NOT NULL,
  "refund_no" character varying(32) NULL,
  "correction_request_id" bigint NULL,
  "allowance_number" character varying(16) NULL,
  "allowance_date_taipei" date NULL,
  "net_amount_ex_tax" bigint NULL,
  "tax_amount" bigint NULL,
  "gross_amount" bigint NULL,
  "void_reason_code" character varying(32) NULL,
  "void_reason_detail" text NULL,
  "blocked_reason_code" character varying(32) NULL,
  "cancel_reason_code" character varying(48) NULL,
  "provider_confirmed_at" timestamptz NULL,
  "provider_upload_status" smallint NULL,
  "provider_upload_checked_at" timestamptz NULL,
  "pdf_object_key" character varying(512) NULL,
  "version" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invadj_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_invadj_allowance_no_len" CHECK ((allowance_number IS NULL) OR (length((allowance_number)::text) <= 16)),
  CONSTRAINT "chk_invadj_allowance_positive" CHECK ((gross_amount IS NULL) OR ((gross_amount > 0) AND (net_amount_ex_tax >= 0) AND (tax_amount >= 0))),
  CONSTRAINT "chk_invadj_allowance_shape" CHECK ((((adjustment_type)::text = 'ALLOWANCE'::text) AND (allowance_number IS NOT NULL) AND (allowance_date_taipei IS NOT NULL) AND (net_amount_ex_tax IS NOT NULL) AND (tax_amount IS NOT NULL) AND (gross_amount IS NOT NULL)) OR (((adjustment_type)::text = 'VOID'::text) AND (allowance_number IS NULL) AND (net_amount_ex_tax IS NULL) AND (tax_amount IS NULL) AND (gross_amount IS NULL))),
  CONSTRAINT "chk_invadj_allowance_sum" CHECK ((gross_amount IS NULL) OR (gross_amount = (net_amount_ex_tax + tax_amount))),
  CONSTRAINT "chk_invadj_blocked_reason" CHECK (((status)::text <> 'FAILED_PERMANENT'::text) OR (blocked_reason_code IS NOT NULL)),
  CONSTRAINT "chk_invadj_correction_pair" CHECK (((trigger_kind)::text = 'BUYER_CORRECTION'::text) = (correction_request_id IS NOT NULL)),
  CONSTRAINT "chk_invadj_refund_pair" CHECK (((trigger_kind)::text = 'ORDER_REFUND'::text) = (refund_no IS NOT NULL)),
  CONSTRAINT "chk_invadj_status" CHECK ((status)::text = ANY ((ARRAY['PENDING'::character varying, 'BLOCKED_UNKNOWN_DOCUMENT'::character varying, 'BLOCKED_PENDING_CORRECTION'::character varying, 'SENDING'::character varying, 'WAITING_CONFIRMATION'::character varying, 'COMPLETED'::character varying, 'FAILED_PERMANENT'::character varying, 'FAILED_UNKNOWN'::character varying, 'CANCELLED'::character varying])::text[])),
  CONSTRAINT "chk_invadj_trigger" CHECK ((trigger_kind)::text = ANY ((ARRAY['ORDER_REFUND'::character varying, 'BUYER_CORRECTION'::character varying, 'ADM_MANUAL'::character varying])::text[])),
  CONSTRAINT "chk_invadj_type" CHECK ((adjustment_type)::text = ANY ((ARRAY['VOID'::character varying, 'ALLOWANCE'::character varying])::text[])),
  CONSTRAINT "chk_invadj_version_nonneg" CHECK (version >= 0)
);
-- Create index "idx_invadj_document" to table: "invoice_adjustments"
CREATE INDEX "idx_invadj_document" ON "invoice_adjustments" ("document_id", "created_at");
-- Create index "idx_invadj_status" to table: "invoice_adjustments"
CREATE INDEX "idx_invadj_status" ON "invoice_adjustments" ("status", "created_at") WHERE ((status)::text = ANY ((ARRAY['PENDING'::character varying, 'BLOCKED_UNKNOWN_DOCUMENT'::character varying, 'BLOCKED_PENDING_CORRECTION'::character varying, 'SENDING'::character varying, 'WAITING_CONFIRMATION'::character varying, 'FAILED_UNKNOWN'::character varying])::text[]));
-- Create index "uq_invadj_allowance_number" to table: "invoice_adjustments"
CREATE UNIQUE INDEX "uq_invadj_allowance_number" ON "invoice_adjustments" ("allowance_number") WHERE (allowance_number IS NOT NULL);
-- Create index "uq_invadj_allowance_refund" to table: "invoice_adjustments"
CREATE UNIQUE INDEX "uq_invadj_allowance_refund" ON "invoice_adjustments" ("document_id", "refund_no") WHERE (((adjustment_type)::text = 'ALLOWANCE'::text) AND (refund_no IS NOT NULL) AND ((status)::text <> 'CANCELLED'::text));
-- Create index "uq_invadj_void_once" to table: "invoice_adjustments"
CREATE UNIQUE INDEX "uq_invadj_void_once" ON "invoice_adjustments" ("document_id") WHERE (((adjustment_type)::text = 'VOID'::text) AND ((status)::text <> 'CANCELLED'::text));
-- Create "invoice_document_lines" table
CREATE TABLE "invoice_document_lines" (
  "id" bigserial NOT NULL,
  "document_id" bigint NOT NULL,
  "line_seq" integer NOT NULL,
  "line_type" character varying(20) NOT NULL,
  "order_item_id" bigint NULL,
  "sku_id" bigint NULL,
  "description" character varying(256) NOT NULL,
  "unit" character varying(6) NULL,
  "quantity" integer NOT NULL,
  "unit_price" bigint NOT NULL,
  "net_amount" bigint NOT NULL,
  "tax_category" smallint NOT NULL DEFAULT 1,
  "tax_rate_bps" integer NOT NULL DEFAULT 500,
  "remark" character varying(40) NULL,
  "allocated_discount" bigint NOT NULL DEFAULT 0,
  "allowanceable_base" bigint NOT NULL DEFAULT 0,
  "allowanced_amount" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invline_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "chk_invline_allowance_shape" CHECK ((((line_type)::text = 'DISCOUNT'::text) AND (allocated_discount = 0) AND (allowanceable_base = 0) AND (allowanced_amount = 0)) OR (((line_type)::text <> 'DISCOUNT'::text) AND (allocated_discount >= 0) AND (allowanceable_base = (net_amount - allocated_discount)))),
  CONSTRAINT "chk_invline_allowanced_cap" CHECK ((allowanced_amount >= 0) AND (allowanced_amount <= allowanceable_base)),
  CONSTRAINT "chk_invline_amount" CHECK (net_amount = (unit_price * quantity)),
  CONSTRAINT "chk_invline_qty" CHECK (quantity > 0),
  CONSTRAINT "chk_invline_seq" CHECK (line_seq >= 1),
  CONSTRAINT "chk_invline_seq_max" CHECK (line_seq <= 9999),
  CONSTRAINT "chk_invline_sign" CHECK ((((line_type)::text = 'DISCOUNT'::text) AND (net_amount < 0)) OR (((line_type)::text <> 'DISCOUNT'::text) AND (net_amount >= 0))),
  CONSTRAINT "chk_invline_tax_category" CHECK (tax_category = ANY (ARRAY[1, 2, 3])),
  CONSTRAINT "chk_invline_type" CHECK ((line_type)::text = ANY ((ARRAY['PRODUCT'::character varying, 'SHIPPING'::character varying, 'DISCOUNT'::character varying])::text[]))
);
-- Create index "uq_invline_doc_seq" to table: "invoice_document_lines"
CREATE UNIQUE INDEX "uq_invline_doc_seq" ON "invoice_document_lines" ("document_id", "line_seq");
-- Create "invoice_adjustment_lines" table
CREATE TABLE "invoice_adjustment_lines" (
  "id" bigserial NOT NULL,
  "adjustment_id" bigint NOT NULL,
  "line_seq" integer NOT NULL,
  "source_line_id" bigint NULL,
  "original_invoice_no" character varying(10) NOT NULL,
  "original_invoice_date" date NOT NULL,
  "original_description" character varying(256) NOT NULL,
  "quantity" integer NOT NULL,
  "unit_price_ex_tax" bigint NOT NULL,
  "net_amount_ex_tax" bigint NOT NULL,
  "tax_amount" bigint NOT NULL,
  "tax_category" smallint NOT NULL DEFAULT 1,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invadjline_adjustment" FOREIGN KEY ("adjustment_id") REFERENCES "invoice_adjustments" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_invadjline_source" FOREIGN KEY ("source_line_id") REFERENCES "invoice_document_lines" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_invadjline_amount" CHECK (net_amount_ex_tax = (unit_price_ex_tax * quantity)),
  CONSTRAINT "chk_invadjline_nonneg" CHECK ((net_amount_ex_tax >= 0) AND (tax_amount >= 0)),
  CONSTRAINT "chk_invadjline_qty" CHECK (quantity > 0),
  CONSTRAINT "chk_invadjline_seq" CHECK ((line_seq >= 1) AND (line_seq <= 9999)),
  CONSTRAINT "chk_invadjline_tax_category" CHECK (tax_category = ANY (ARRAY[1, 2, 3]))
);
-- Create index "idx_invadjline_source" to table: "invoice_adjustment_lines"
CREATE INDEX "idx_invadjline_source" ON "invoice_adjustment_lines" ("source_line_id") WHERE (source_line_id IS NOT NULL);
-- Create index "uq_invadjline_seq" to table: "invoice_adjustment_lines"
CREATE UNIQUE INDEX "uq_invadjline_seq" ON "invoice_adjustment_lines" ("adjustment_id", "line_seq");
-- Create "invoice_complaints" table
CREATE TABLE "invoice_complaints" (
  "id" bigint NOT NULL,
  "document_id" bigint NOT NULL,
  "issuer_id" bigint NOT NULL,
  "order_no" character varying(64) NULL,
  "complainant_buyer_id" bigint NOT NULL,
  "reason_code" character varying(32) NOT NULL DEFAULT 'NOT_RECEIVED',
  "description" text NULL,
  "status" character varying(32) NOT NULL DEFAULT 'OPEN',
  "sla_due_at" timestamptz NOT NULL,
  "overdue_at" timestamptz NULL,
  "buyer_review_due_at" timestamptz NULL,
  "first_responded_at" timestamptz NULL,
  "resolved_at" timestamptz NULL,
  "resolution_code" character varying(32) NULL,
  "resolution_note" text NULL,
  "reopen_count" smallint NOT NULL DEFAULT 0,
  "reopen_allowed_after" timestamptz NULL,
  "decided_by_account_id" bigint NULL,
  "decided_at" timestamptz NULL,
  "version" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invcomp_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_invcomp_issuer" FOREIGN KEY ("issuer_id") REFERENCES "invoice_issuers" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_invcomp_overdue_pair" CHECK (((status)::text <> 'OVERDUE'::text) OR (overdue_at IS NOT NULL)),
  CONSTRAINT "chk_invcomp_reopen" CHECK ((reopen_count >= 0) AND (reopen_count <= 2)),
  CONSTRAINT "chk_invcomp_resolved_pair" CHECK (((status)::text = ANY ((ARRAY['RESOLVED'::character varying, 'VIOLATION_CONFIRMED'::character varying])::text[])) = (resolved_at IS NOT NULL)),
  CONSTRAINT "chk_invcomp_status" CHECK ((status)::text = ANY ((ARRAY['OPEN'::character varying, 'SELLER_RESPONDED'::character varying, 'OVERDUE'::character varying, 'ESCALATED_TO_ADM'::character varying, 'RESOLVED'::character varying, 'VIOLATION_CONFIRMED'::character varying, 'WITHDRAWN'::character varying])::text[])),
  CONSTRAINT "chk_invcomp_version_nonneg" CHECK (version >= 0),
  CONSTRAINT "chk_invcomp_violation_needs_decider" CHECK (((status)::text <> 'VIOLATION_CONFIRMED'::text) OR (decided_by_account_id IS NOT NULL))
);
-- Create index "idx_invcomp_buyer_review_due" to table: "invoice_complaints"
CREATE INDEX "idx_invcomp_buyer_review_due" ON "invoice_complaints" ("buyer_review_due_at") WHERE ((status)::text = 'SELLER_RESPONDED'::text);
-- Create index "idx_invcomp_issuer_status" to table: "invoice_complaints"
CREATE INDEX "idx_invcomp_issuer_status" ON "invoice_complaints" ("issuer_id", "status", "created_at" DESC);
-- Create index "idx_invcomp_overdue_escalate" to table: "invoice_complaints"
CREATE INDEX "idx_invcomp_overdue_escalate" ON "invoice_complaints" ("overdue_at") WHERE ((status)::text = 'OVERDUE'::text);
-- Create index "idx_invcomp_sla_overdue" to table: "invoice_complaints"
CREATE INDEX "idx_invcomp_sla_overdue" ON "invoice_complaints" ("sla_due_at") WHERE ((status)::text = 'OPEN'::text);
-- Create index "idx_invcomp_violation" to table: "invoice_complaints"
CREATE INDEX "idx_invcomp_violation" ON "invoice_complaints" ("issuer_id", "resolved_at") WHERE ((status)::text = 'VIOLATION_CONFIRMED'::text);
-- Create index "uq_invcomp_open_per_document" to table: "invoice_complaints"
CREATE UNIQUE INDEX "uq_invcomp_open_per_document" ON "invoice_complaints" ("document_id") WHERE ((status)::text = ANY ((ARRAY['OPEN'::character varying, 'OVERDUE'::character varying, 'SELLER_RESPONDED'::character varying, 'ESCALATED_TO_ADM'::character varying, 'RESOLVED'::character varying])::text[]));
-- Create "invoice_complaint_events" table
CREATE TABLE "invoice_complaint_events" (
  "id" bigserial NOT NULL,
  "complaint_id" bigint NOT NULL,
  "event_type" character varying(32) NOT NULL,
  "action" character varying(24) NULL,
  "actor_type" character varying(16) NOT NULL,
  "actor_account_id" bigint NULL,
  "message" text NULL,
  "attachment_object_keys" jsonb NOT NULL DEFAULT '[]',
  "occurred_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invcompev_complaint" FOREIGN KEY ("complaint_id") REFERENCES "invoice_complaints" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "chk_invcompev_actor" CHECK ((actor_type)::text = ANY ((ARRAY['BUYER'::character varying, 'SELLER'::character varying, 'ADMIN'::character varying, 'SYSTEM'::character varying])::text[])),
  CONSTRAINT "chk_invcompev_actor_id" CHECK (((actor_type)::text = 'SYSTEM'::text) OR (actor_account_id IS NOT NULL))
);
-- Create index "idx_invcompev_complaint" to table: "invoice_complaint_events"
CREATE INDEX "idx_invcompev_complaint" ON "invoice_complaint_events" ("complaint_id", "occurred_at");
-- Create "invoice_correction_requests" table
CREATE TABLE "invoice_correction_requests" (
  "id" bigint NOT NULL,
  "document_id" bigint NOT NULL,
  "requested_by_buyer_id" bigint NOT NULL,
  "expected_snapshot_version" integer NOT NULL,
  "before_payload" jsonb NOT NULL,
  "after_payload" jsonb NOT NULL,
  "status" character varying(32) NOT NULL DEFAULT 'REQUESTED',
  "handling_mode" character varying(24) NULL,
  "reject_reason_code" character varying(32) NULL,
  "reject_reason_detail" text NULL,
  "void_adjustment_id" bigint NULL,
  "reissued_document_id" bigint NULL,
  "version" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invcorr_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_invcorr_reissued" FOREIGN KEY ("reissued_document_id") REFERENCES "invoice_documents" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_invcorr_void" FOREIGN KEY ("void_adjustment_id") REFERENCES "invoice_adjustments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_invcorr_mode" CHECK ((handling_mode IS NULL) OR ((handling_mode)::text = ANY ((ARRAY['UPDATE_SNAPSHOT'::character varying, 'VOID_AND_REISSUE'::character varying])::text[]))),
  CONSTRAINT "chk_invcorr_reject_reason" CHECK (((status)::text <> 'REJECTED'::text) OR (reject_reason_code IS NOT NULL)),
  CONSTRAINT "chk_invcorr_reject_reason_code" CHECK ((reject_reason_code IS NULL) OR ((reject_reason_code)::text = ANY ((ARRAY['ALREADY_REPORTED'::character varying, 'PAST_VOID_DEADLINE'::character varying, 'ALLOWANCE_EXISTS'::character varying, 'TRACK_INSUFFICIENT'::character varying, 'CARRIER_INVALID'::character varying, 'PROVIDER_UNAVAILABLE'::character varying])::text[]))),
  CONSTRAINT "chk_invcorr_status" CHECK ((status)::text = ANY ((ARRAY['REQUESTED'::character varying, 'REJECTED'::character varying, 'UPDATED_BEFORE_ISSUE'::character varying, 'VOID_REISSUE_IN_PROGRESS'::character varying, 'COMPLETED'::character varying, 'REISSUE_FAILED'::character varying])::text[])),
  CONSTRAINT "chk_invcorr_version_nonneg" CHECK (version >= 0)
);
-- Create index "uq_invcorr_open_per_document" to table: "invoice_correction_requests"
CREATE UNIQUE INDEX "uq_invcorr_open_per_document" ON "invoice_correction_requests" ("document_id") WHERE ((status)::text = ANY ((ARRAY['REQUESTED'::character varying, 'VOID_REISSUE_IN_PROGRESS'::character varying])::text[]));
-- Create "invoice_operations" table
CREATE TABLE "invoice_operations" (
  "id" bigint NOT NULL,
  "document_id" bigint NOT NULL,
  "adjustment_id" bigint NULL,
  "operation_type" character varying(16) NOT NULL,
  "operation_key" character varying(96) NOT NULL,
  "provider_request_no" character varying(40) NOT NULL,
  "status" character varying(28) NOT NULL DEFAULT 'READY',
  "request_snapshot_hash" character varying(64) NOT NULL,
  "attempt_count" integer NOT NULL DEFAULT 0,
  "max_attempts" integer NOT NULL DEFAULT 13,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "held_until" timestamptz NULL,
  "recheck_list_page" smallint NOT NULL DEFAULT 0,
  "statutory_deadline_at" timestamptz NULL,
  "last_http_status" integer NULL,
  "provider_code" character varying(16) NULL,
  "provider_message" text NULL,
  "unknown_result" boolean NOT NULL DEFAULT false,
  "unknown_since" timestamptz NULL,
  "cancel_requested_by" bigint NULL,
  "cancel_requested_at" timestamptz NULL,
  "cancel_confirmed_by" bigint NULL,
  "cancel_confirmed_at" timestamptz NULL,
  "cancel_reason_code" character varying(48) NULL,
  "cancel_reason" text NULL,
  "started_at" timestamptz NULL,
  "completed_at" timestamptz NULL,
  "last_error" text NULL,
  "version" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invop_adjustment" FOREIGN KEY ("adjustment_id") REFERENCES "invoice_adjustments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_invop_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_invop_adjustment_pair" CHECK (((operation_type)::text = ANY ((ARRAY['VOID'::character varying, 'ALLOWANCE'::character varying, 'VOID_ALLOWANCE'::character varying])::text[])) = (adjustment_id IS NOT NULL)),
  CONSTRAINT "chk_invop_admin_cancel_needs_confirm" CHECK (((status)::text <> 'CANCELLED_BY_ADMIN'::text) OR (cancel_confirmed_by IS NOT NULL)),
  CONSTRAINT "chk_invop_attempts" CHECK ((attempt_count >= 0) AND (max_attempts >= 1)),
  CONSTRAINT "chk_invop_cancel_pair" CHECK ((cancel_requested_by IS NULL) = (cancel_requested_at IS NULL)),
  CONSTRAINT "chk_invop_held_pair" CHECK (((status)::text = 'HELD'::text) = (held_until IS NOT NULL)),
  CONSTRAINT "chk_invop_recheck_page" CHECK (recheck_list_page >= 0),
  CONSTRAINT "chk_invop_request_no_len" CHECK (length((provider_request_no)::text) <= 40),
  CONSTRAINT "chk_invop_status" CHECK ((status)::text = ANY ((ARRAY['CREATED'::character varying, 'AWAITING_TRIGGER'::character varying, 'READY'::character varying, 'HELD'::character varying, 'SENDING'::character varying, 'RETRY_SCHEDULED'::character varying, 'WAITING_CONFIRMATION'::character varying, 'PROVIDER_STATUS_RECHECK'::character varying, 'SUCCEEDED'::character varying, 'FAILED_PERMANENT'::character varying, 'FAILED_RETRY_EXHAUSTED'::character varying, 'FAILED_UNKNOWN'::character varying, 'SELLER_ACTION_REQUIRED'::character varying, 'CANCEL_REQUESTED'::character varying, 'CANCEL_BLOCKED_UNKNOWN'::character varying, 'CANCELLED_BY_ADMIN'::character varying, 'MANUAL_PROCESS_ALLOWED'::character varying, 'RESOLVED_EXTERNALLY'::character varying, 'NOT_APPLICABLE'::character varying, 'CANCELLED_BEFORE_ISSUE'::character varying])::text[])),
  CONSTRAINT "chk_invop_two_person" CHECK ((cancel_confirmed_by IS NULL) OR ((cancel_requested_by IS NOT NULL) AND (cancel_confirmed_by <> cancel_requested_by))),
  CONSTRAINT "chk_invop_type" CHECK ((operation_type)::text = ANY ((ARRAY['ISSUE'::character varying, 'QUERY'::character varying, 'VOID'::character varying, 'ALLOWANCE'::character varying, 'REISSUE'::character varying, 'VOID_ALLOWANCE'::character varying])::text[])),
  CONSTRAINT "chk_invop_unknown_pair" CHECK ((unknown_result = false) = (unknown_since IS NULL)),
  CONSTRAINT "chk_invop_version_nonneg" CHECK (version >= 0)
);
-- Create index "idx_invop_adjustment" to table: "invoice_operations"
CREATE INDEX "idx_invop_adjustment" ON "invoice_operations" ("adjustment_id", "created_at" DESC) WHERE (adjustment_id IS NOT NULL);
-- Create index "idx_invop_document" to table: "invoice_operations"
CREATE INDEX "idx_invop_document" ON "invoice_operations" ("document_id", "created_at" DESC);
-- Create index "idx_invop_due" to table: "invoice_operations"
CREATE INDEX "idx_invop_due" ON "invoice_operations" ("next_attempt_at") WHERE ((status)::text = ANY ((ARRAY['READY'::character varying, 'RETRY_SCHEDULED'::character varying, 'SENDING'::character varying])::text[]));
-- Create index "idx_invop_recheck_due" to table: "invoice_operations"
CREATE INDEX "idx_invop_recheck_due" ON "invoice_operations" ("next_attempt_at") WHERE ((status)::text = ANY ((ARRAY['WAITING_CONFIRMATION'::character varying, 'PROVIDER_STATUS_RECHECK'::character varying, 'FAILED_UNKNOWN'::character varying, 'CANCEL_REQUESTED'::character varying, 'CANCEL_BLOCKED_UNKNOWN'::character varying])::text[]));
-- Create index "uq_invop_open_adjustment_per_document" to table: "invoice_operations"
CREATE UNIQUE INDEX "uq_invop_open_adjustment_per_document" ON "invoice_operations" ("document_id") WHERE (((operation_type)::text = ANY ((ARRAY['VOID'::character varying, 'REISSUE'::character varying, 'ALLOWANCE'::character varying, 'VOID_ALLOWANCE'::character varying])::text[])) AND ((status)::text <> ALL ((ARRAY['SUCCEEDED'::character varying, 'NOT_APPLICABLE'::character varying, 'CANCELLED_BEFORE_ISSUE'::character varying, 'RESOLVED_EXTERNALLY'::character varying, 'CANCELLED_BY_ADMIN'::character varying])::text[])));
-- Create index "uq_invop_operation_key" to table: "invoice_operations"
CREATE UNIQUE INDEX "uq_invop_operation_key" ON "invoice_operations" ("operation_key");
-- Create "invoice_provider_accounts" table
CREATE TABLE "invoice_provider_accounts" (
  "id" bigint NOT NULL,
  "issuer_id" bigint NOT NULL,
  "provider" character varying(16) NOT NULL DEFAULT 'AMEGO',
  "provider_tax_id" character varying(8) NOT NULL,
  "credential_ciphertext" text NOT NULL,
  "credential_key_version" smallint NOT NULL DEFAULT 1,
  "credential_fingerprint" character varying(64) NOT NULL,
  "credential_last4" character varying(4) NOT NULL,
  "status" character varying(32) NOT NULL DEFAULT 'CREDENTIAL_SUBMITTED',
  "status_reason" text NULL,
  "verified_at" timestamptz NULL,
  "automation_cutover_at" timestamptz NULL,
  "last_health_checked_at" timestamptz NULL,
  "health_check_next_at" timestamptz NOT NULL DEFAULT now(),
  "breaker_opened_at" timestamptz NULL,
  "breaker_probe_success" smallint NOT NULL DEFAULT 0,
  "pending_ciphertext" text NULL,
  "pending_fingerprint" character varying(64) NULL,
  "pending_last4" character varying(4) NULL,
  "rotation_started_at" timestamptz NULL,
  "version" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_ipa_issuer" FOREIGN KEY ("issuer_id") REFERENCES "invoice_issuers" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_ipa_active_verified" CHECK (((status)::text <> 'ACTIVE'::text) OR (verified_at IS NOT NULL)),
  CONSTRAINT "chk_ipa_cutover_requires_verified" CHECK ((automation_cutover_at IS NULL) OR (verified_at IS NOT NULL)),
  CONSTRAINT "chk_ipa_provider" CHECK ((provider)::text = 'AMEGO'::text),
  CONSTRAINT "chk_ipa_rotation" CHECK (((status)::text = 'ROTATION_PENDING'::text) = (pending_ciphertext IS NOT NULL)),
  CONSTRAINT "chk_ipa_rotation_triplet" CHECK (((pending_ciphertext IS NULL) AND (pending_fingerprint IS NULL) AND (pending_last4 IS NULL)) OR ((pending_ciphertext IS NOT NULL) AND (pending_fingerprint IS NOT NULL) AND (pending_last4 IS NOT NULL))),
  CONSTRAINT "chk_ipa_status" CHECK ((status)::text = ANY ((ARRAY['NOT_CONFIGURED'::character varying, 'CREDENTIAL_SUBMITTED'::character varying, 'CREDENTIAL_VERIFYING'::character varying, 'ACTIVE'::character varying, 'INVALID_CREDENTIAL'::character varying, 'PROVIDER_REJECTED'::character varying, 'PROVIDER_UNAVAILABLE'::character varying, 'ROTATION_PENDING'::character varying, 'DEGRADED'::character varying, 'SUSPENDED'::character varying, 'REVOKED'::character varying])::text[])),
  CONSTRAINT "chk_ipa_tax_id" CHECK ((provider_tax_id)::text ~ '^[0-9]{8}$'::text),
  CONSTRAINT "chk_ipa_version_nonneg" CHECK (version >= 0)
);
-- Create index "idx_ipa_health" to table: "invoice_provider_accounts"
CREATE INDEX "idx_ipa_health" ON "invoice_provider_accounts" ("health_check_next_at") WHERE ((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'DEGRADED'::character varying])::text[]));
-- Create index "uq_ipa_fingerprint" to table: "invoice_provider_accounts"
CREATE UNIQUE INDEX "uq_ipa_fingerprint" ON "invoice_provider_accounts" ("provider", "credential_fingerprint") WHERE ((status)::text = ANY ((ARRAY['CREDENTIAL_SUBMITTED'::character varying, 'CREDENTIAL_VERIFYING'::character varying, 'ACTIVE'::character varying, 'INVALID_CREDENTIAL'::character varying, 'PROVIDER_REJECTED'::character varying, 'PROVIDER_UNAVAILABLE'::character varying, 'ROTATION_PENDING'::character varying, 'DEGRADED'::character varying, 'SUSPENDED'::character varying])::text[]));
-- Create index "uq_ipa_issuer_provider" to table: "invoice_provider_accounts"
CREATE UNIQUE INDEX "uq_ipa_issuer_provider" ON "invoice_provider_accounts" ("issuer_id", "provider");
-- Create "invoice_provider_audit_logs" table
CREATE TABLE "invoice_provider_audit_logs" (
  "id" bigserial NOT NULL,
  "operation_id" bigint NULL,
  "issuer_id" bigint NOT NULL,
  "provider" character varying(16) NOT NULL DEFAULT 'AMEGO',
  "endpoint" character varying(64) NOT NULL,
  "attempt_seq" integer NOT NULL,
  "request_masked" jsonb NOT NULL,
  "response_masked" jsonb NULL,
  "raw_body_hash" character varying(64) NOT NULL,
  "http_status" integer NULL,
  "provider_code" character varying(16) NULL,
  "latency_ms" integer NULL,
  "error_kind" character varying(32) NULL,
  "correlation_id" character varying(64) NULL,
  "occurred_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_invpaudit_operation" FOREIGN KEY ("operation_id") REFERENCES "invoice_operations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_invpaudit_attempt" CHECK (attempt_seq >= 1),
  CONSTRAINT "chk_invpaudit_latency" CHECK ((latency_ms IS NULL) OR (latency_ms >= 0))
);
-- Create index "idx_invpaudit_issuer_time" to table: "invoice_provider_audit_logs"
CREATE INDEX "idx_invpaudit_issuer_time" ON "invoice_provider_audit_logs" ("issuer_id", "occurred_at" DESC);
-- Create index "idx_invpaudit_operation" to table: "invoice_provider_audit_logs"
CREATE INDEX "idx_invpaudit_operation" ON "invoice_provider_audit_logs" ("operation_id", "attempt_seq");
-- Create "seller_tax_profiles" table
CREATE TABLE "seller_tax_profiles" (
  "id" bigint NOT NULL,
  "issuer_id" bigint NOT NULL,
  "review_status" character varying(40) NOT NULL DEFAULT 'DRAFT',
  "claimed_obligation" character varying(16) NOT NULL DEFAULT 'UNKNOWN',
  "entity_tax_type" character varying(32) NULL,
  "legal_name" character varying(120) NULL,
  "tax_id" character varying(8) NULL,
  "responsible_person" character varying(60) NULL,
  "business_address" character varying(255) NULL,
  "default_tax_category" smallint NOT NULL DEFAULT 1,
  "evidence_object_keys" jsonb NOT NULL DEFAULT '[]',
  "attestation_version" character varying(32) NULL,
  "attested_at" timestamptz NULL,
  "submitted_at" timestamptz NULL,
  "pending_valid_until" timestamptz NULL,
  "extension_count" smallint NOT NULL DEFAULT 0,
  "pending_order_limit_count" integer NOT NULL DEFAULT 50,
  "pending_order_limit_amount" bigint NOT NULL DEFAULT 100000,
  "pending_order_count" integer NOT NULL DEFAULT 0,
  "pending_order_amount" bigint NOT NULL DEFAULT 0,
  "review_note" text NULL,
  "approved_at" timestamptz NULL,
  "cooldown_until" timestamptz NULL,
  "approval_valid_until" timestamptz NULL,
  "recheck_reason" text NULL,
  "violation_window_days" smallint NOT NULL DEFAULT 90,
  "violation_threshold" smallint NOT NULL DEFAULT 3,
  "suspension_days" smallint NOT NULL DEFAULT 14,
  "version" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_seller_tax_profiles_issuer" FOREIGN KEY ("issuer_id") REFERENCES "invoice_issuers" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_stp_counters_nonneg" CHECK ((pending_order_count >= 0) AND (pending_order_amount >= 0) AND (version >= 0)),
  CONSTRAINT "chk_stp_exempt_no_taxid" CHECK (((review_status)::text <> 'APPROVED_INVOICE_EXEMPT'::text) OR (tax_id IS NULL)),
  CONSTRAINT "chk_stp_extension" CHECK ((extension_count >= 0) AND (extension_count <= 1)),
  CONSTRAINT "chk_stp_extension_needs_deadline" CHECK ((extension_count = 0) OR (pending_valid_until IS NOT NULL)),
  CONSTRAINT "chk_stp_obligation" CHECK ((claimed_obligation)::text = ANY ((ARRAY['REQUIRED'::character varying, 'EXEMPT'::character varying, 'UNKNOWN'::character varying])::text[])),
  CONSTRAINT "chk_stp_pending_window" CHECK (((review_status)::text <> ALL ((ARRAY['SELF_ISSUE_PENDING'::character varying, 'SUPPLEMENT_REQUIRED'::character varying])::text[])) OR ((submitted_at IS NOT NULL) AND (pending_valid_until IS NOT NULL))),
  CONSTRAINT "chk_stp_required_needs_taxid" CHECK (((review_status)::text <> 'APPROVED_INVOICE_REQUIRED'::text) OR ((tax_id IS NOT NULL) AND (legal_name IS NOT NULL))),
  CONSTRAINT "chk_stp_review_status" CHECK ((review_status)::text = ANY ((ARRAY['DRAFT'::character varying, 'SUBMITTED'::character varying, 'UNDER_REVIEW'::character varying, 'SUPPLEMENT_REQUIRED'::character varying, 'SELF_ISSUE_PENDING'::character varying, 'APPROVED_INVOICE_REQUIRED'::character varying, 'APPROVED_INVOICE_EXEMPT'::character varying, 'REJECTED'::character varying, 'EXPIRED'::character varying, 'REVOKED'::character varying])::text[])),
  CONSTRAINT "chk_stp_tax_category" CHECK (default_tax_category = ANY (ARRAY[1, 2, 3])),
  CONSTRAINT "chk_stp_tax_id" CHECK ((tax_id IS NULL) OR ((tax_id)::text ~ '^[0-9]{8}$'::text)),
  CONSTRAINT "chk_stp_valid_until_needs_approved" CHECK ((approval_valid_until IS NULL) OR ((approved_at IS NOT NULL) AND (approval_valid_until > approved_at)))
);
-- Create index "idx_stp_pending_expiry" to table: "seller_tax_profiles"
CREATE INDEX "idx_stp_pending_expiry" ON "seller_tax_profiles" ("pending_valid_until") WHERE ((review_status)::text = ANY ((ARRAY['SELF_ISSUE_PENDING'::character varying, 'SUPPLEMENT_REQUIRED'::character varying])::text[]));
-- Create index "idx_stp_recheck_due" to table: "seller_tax_profiles"
CREATE INDEX "idx_stp_recheck_due" ON "seller_tax_profiles" ("approval_valid_until") WHERE ((review_status)::text = ANY ((ARRAY['APPROVED_INVOICE_REQUIRED'::character varying, 'APPROVED_INVOICE_EXEMPT'::character varying])::text[]));
-- Create index "idx_stp_review_queue" to table: "seller_tax_profiles"
CREATE INDEX "idx_stp_review_queue" ON "seller_tax_profiles" ("review_status", "created_at") WHERE ((review_status)::text = ANY ((ARRAY['SUBMITTED'::character varying, 'UNDER_REVIEW'::character varying, 'SUPPLEMENT_REQUIRED'::character varying, 'SELF_ISSUE_PENDING'::character varying])::text[]));
-- Create index "uq_seller_tax_profiles_issuer" to table: "seller_tax_profiles"
CREATE UNIQUE INDEX "uq_seller_tax_profiles_issuer" ON "seller_tax_profiles" ("issuer_id");
