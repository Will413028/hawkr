
CREATE TABLE "invoice_issuers" (
  "id"              bigint NOT NULL,
  "issuer_type"     varchar(16) NOT NULL,
  "issuer_ref_id"   bigint NOT NULL,
  "display_name"    varchar(120) NOT NULL,
  "status"          varchar(16) NOT NULL DEFAULT 'active',
  "retired_at"      timestamptz,
  "version"         bigint NOT NULL DEFAULT 0,
  "created_at"      timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"      timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_issuers_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_invoice_issuers_type"   CHECK ("issuer_type" IN ('SELLER', 'PLATFORM')),
  CONSTRAINT "chk_invoice_issuers_status" CHECK ("status" IN ('active', 'retired')),
  CONSTRAINT "chk_invoice_issuers_retired" CHECK ("status" <> 'retired' OR "retired_at" IS NOT NULL),
  CONSTRAINT "chk_invoice_issuers_ref" CHECK (("issuer_type" = 'PLATFORM' AND "issuer_ref_id" = 0) OR ("issuer_type" = 'SELLER' AND "issuer_ref_id" > 0)),
  CONSTRAINT "chk_invoice_issuers_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invoice_issuers_ref" ON "invoice_issuers" ("issuer_type", "issuer_ref_id");

CREATE TABLE "seller_tax_profiles" (
  "id"                        bigint NOT NULL,
  "issuer_id"                 bigint NOT NULL,
  "review_status"             varchar(40) NOT NULL DEFAULT 'DRAFT',
  "claimed_obligation"        varchar(16) NOT NULL DEFAULT 'UNKNOWN',
  "entity_tax_type"           varchar(32),
  "legal_name"                varchar(120),
  "tax_id"                    varchar(8),
  "responsible_person"        varchar(60),
  "business_address"          varchar(255),
  "address_city"              varchar(32),
  "address_district"          varchar(32),
  "default_tax_category"      smallint NOT NULL DEFAULT 1,
  "evidence_object_keys"      jsonb NOT NULL DEFAULT '[]',
  "attestation_version"       varchar(32),
  "attested_at"               timestamptz,
  "attested_by_account_id"    bigint,
  "submitted_at"              timestamptz,
  "pending_valid_until"       timestamptz,
  "extension_count"           smallint NOT NULL DEFAULT 0,
  "pending_order_limit_count"  integer NOT NULL DEFAULT 50,
  "pending_order_limit_amount" bigint  NOT NULL DEFAULT 100000,
  "pending_order_count"        integer NOT NULL DEFAULT 0,
  "pending_order_amount"       bigint  NOT NULL DEFAULT 0,
  "review_note"               text,
  "review_note_at"            timestamptz,
  "approved_at"               timestamptz,
  "cooldown_until"            timestamptz,
  "approval_valid_until"      timestamptz,
  "recheck_reason"            text,
  "receipt_authorized_at"     timestamptz,
  "receipt_consent_version"   varchar(32),
  "receipt_approval_basis_key" text,
  "receipt_approval_basis_asset_id" bigint,
  "violation_window_days"     smallint NOT NULL DEFAULT 90,
  "violation_threshold"       smallint NOT NULL DEFAULT 3,
  "suspension_days"           smallint NOT NULL DEFAULT 14,
  "version"                   bigint NOT NULL DEFAULT 0,
  "created_at"                timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"                timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "seller_tax_profiles_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_seller_tax_profiles_issuer" FOREIGN KEY ("issuer_id") REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "chk_stp_review_status" CHECK ("review_status" IN ('DRAFT','SUBMITTED','UNDER_REVIEW','SUPPLEMENT_REQUIRED','SELF_ISSUE_PENDING','APPROVED_INVOICE_REQUIRED','APPROVED_INVOICE_EXEMPT','REJECTED','EXPIRED','REVOKED')),
  CONSTRAINT "chk_stp_obligation" CHECK ("claimed_obligation" IN ('REQUIRED','EXEMPT','UNKNOWN')),
  CONSTRAINT "chk_stp_tax_category" CHECK ("default_tax_category" IN (1, 2, 3)),
  CONSTRAINT "chk_stp_tax_id" CHECK ("tax_id" IS NULL OR "tax_id" ~ '^[0-9]{8}$'),
  CONSTRAINT "chk_stp_required_needs_taxid" CHECK ("review_status" <> 'APPROVED_INVOICE_REQUIRED' OR ("tax_id" IS NOT NULL AND "legal_name" IS NOT NULL)),
  CONSTRAINT "chk_stp_exempt_no_taxid" CHECK ("review_status" <> 'APPROVED_INVOICE_EXEMPT' OR "tax_id" IS NULL),
  CONSTRAINT "chk_stp_extension" CHECK ("extension_count" BETWEEN 0 AND 1),
  CONSTRAINT "chk_stp_extension_needs_deadline" CHECK ("extension_count" = 0 OR "pending_valid_until" IS NOT NULL),
  CONSTRAINT "chk_stp_pending_window" CHECK ("review_status" NOT IN ('SELF_ISSUE_PENDING', 'SUPPLEMENT_REQUIRED') OR ("submitted_at" IS NOT NULL AND "pending_valid_until" IS NOT NULL)),
  CONSTRAINT "chk_stp_valid_until_needs_approved" CHECK ("approval_valid_until" IS NULL OR ("approved_at" IS NOT NULL AND "approval_valid_until" > "approved_at")),
  CONSTRAINT "chk_stp_counters_nonneg" CHECK ("pending_order_count" >= 0 AND "pending_order_amount" >= 0 AND "version" >= 0),
  CONSTRAINT "chk_stp_receipt_auth_pair" CHECK (("receipt_authorized_at" IS NULL) = ("receipt_consent_version" IS NULL)),
  CONSTRAINT "chk_stp_receipt_basis_pair" CHECK (("receipt_approval_basis_key" IS NULL) = ("receipt_approval_basis_asset_id" IS NULL)),
  CONSTRAINT "chk_stp_receipt_consent_nonblank" CHECK ("receipt_consent_version" IS NULL OR length("receipt_consent_version") >= 1),
  CONSTRAINT "chk_stp_attestation_pair" CHECK (("attested_at" IS NULL) = ("attestation_version" IS NULL)),
  CONSTRAINT "chk_stp_attestation_version_nonblank" CHECK ("attestation_version" IS NULL OR length("attestation_version") >= 1),
  CONSTRAINT "chk_stp_attested_when_committed" CHECK ("review_status" NOT IN ('SUBMITTED', 'SELF_ISSUE_PENDING') OR ("attestation_version" IS NOT NULL AND "attested_at" IS NOT NULL))
);

CREATE UNIQUE INDEX "uq_seller_tax_profiles_issuer" ON "seller_tax_profiles" ("issuer_id");
CREATE INDEX "idx_stp_review_queue" ON "seller_tax_profiles" ("review_status", "created_at") WHERE ((review_status)::text = ANY ((ARRAY['SUBMITTED'::character varying, 'UNDER_REVIEW'::character varying, 'SUPPLEMENT_REQUIRED'::character varying, 'SELF_ISSUE_PENDING'::character varying])::text[]));
CREATE INDEX "idx_stp_pending_expiry" ON "seller_tax_profiles" ("pending_valid_until") WHERE ((review_status)::text = ANY ((ARRAY['SELF_ISSUE_PENDING'::character varying, 'SUPPLEMENT_REQUIRED'::character varying])::text[]));
CREATE INDEX "idx_stp_recheck_due" ON "seller_tax_profiles" ("approval_valid_until") WHERE ((review_status)::text = ANY ((ARRAY['APPROVED_INVOICE_REQUIRED'::character varying, 'APPROVED_INVOICE_EXEMPT'::character varying])::text[]));

CREATE TABLE "invoice_provider_accounts" (
  "id"                       bigint NOT NULL,
  "issuer_id"                bigint NOT NULL,
  "provider"                 varchar(16) NOT NULL DEFAULT 'AMEGO',
  "provider_tax_id"          varchar(8) NOT NULL,
  "credential_ciphertext"    text NOT NULL,
  "credential_key_version"   smallint NOT NULL DEFAULT 1,
  "credential_fingerprint"   varchar(64) NOT NULL,
  "credential_last4"         varchar(4) NOT NULL,
  "status"                   varchar(32) NOT NULL DEFAULT 'CREDENTIAL_SUBMITTED',
  "status_reason"            text,
  "verified_at"              timestamptz,
  "automation_cutover_at"    timestamptz,
  "last_health_checked_at"   timestamptz,
  "health_check_next_at"     timestamptz NOT NULL DEFAULT NOW(),
  "breaker_opened_at"        timestamptz,
  "breaker_probe_success"    smallint NOT NULL DEFAULT 0,
  "pending_ciphertext"       text,
  "pending_fingerprint"      varchar(64),
  "pending_last4"            varchar(4),
  "rotation_started_at"      timestamptz,
  "credential_updated_at"    timestamptz,
  "version"                  bigint NOT NULL DEFAULT 0,
  "created_at"               timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"               timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_provider_accounts_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_ipa_issuer" FOREIGN KEY ("issuer_id") REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "chk_ipa_provider" CHECK ("provider" IN ('AMEGO')),
  CONSTRAINT "chk_ipa_status" CHECK ("status" IN ('NOT_CONFIGURED','CREDENTIAL_SUBMITTED','CREDENTIAL_VERIFYING','ACTIVE','INVALID_CREDENTIAL','PROVIDER_REJECTED','PROVIDER_UNAVAILABLE','ROTATION_PENDING','DEGRADED','SUSPENDED','REVOKED')),
  CONSTRAINT "chk_ipa_tax_id" CHECK ("provider_tax_id" ~ '^[0-9]{8}$'),
  CONSTRAINT "chk_ipa_cutover_requires_verified" CHECK ("automation_cutover_at" IS NULL OR "verified_at" IS NOT NULL),
  CONSTRAINT "chk_ipa_active_verified" CHECK ("status" <> 'ACTIVE' OR "verified_at" IS NOT NULL),
  CONSTRAINT "chk_ipa_rotation" CHECK (("status" = 'ROTATION_PENDING') = ("pending_ciphertext" IS NOT NULL)),
  CONSTRAINT "chk_ipa_rotation_triplet" CHECK (("pending_ciphertext" IS NULL AND "pending_fingerprint" IS NULL AND "pending_last4" IS NULL) OR ("pending_ciphertext" IS NOT NULL AND "pending_fingerprint" IS NOT NULL AND "pending_last4" IS NOT NULL)),
  CONSTRAINT "chk_ipa_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_ipa_issuer_provider" ON "invoice_provider_accounts" ("issuer_id", "provider");
CREATE UNIQUE INDEX "uq_ipa_fingerprint" ON "invoice_provider_accounts" ("provider", "credential_fingerprint") WHERE ((status)::text = ANY ((ARRAY['CREDENTIAL_SUBMITTED'::character varying, 'CREDENTIAL_VERIFYING'::character varying, 'ACTIVE'::character varying, 'INVALID_CREDENTIAL'::character varying, 'PROVIDER_REJECTED'::character varying, 'PROVIDER_UNAVAILABLE'::character varying, 'ROTATION_PENDING'::character varying, 'DEGRADED'::character varying, 'SUSPENDED'::character varying])::text[]));
CREATE INDEX "idx_ipa_health" ON "invoice_provider_accounts" ("health_check_next_at") WHERE ((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'DEGRADED'::character varying, 'PROVIDER_UNAVAILABLE'::character varying])::text[]));

CREATE TABLE "buyer_invoice_preferences" (
  "id"                 bigint NOT NULL,
  "buyer_id"           bigint NOT NULL,
  "preference_kind"    varchar(20) NOT NULL,
  "carrier_type"       varchar(6),
  "carrier_value_ciphertext" text,
  "carrier_value_hash" varchar(64),
  "carrier_value_mask" varchar(16),
  "tax_id"             varchar(8),
  "tax_title"          varchar(60),
  "donation_code"      varchar(7),
  "label"              varchar(30),
  "is_default"         boolean NOT NULL DEFAULT false,
  "superseded_at"      timestamptz,
  "superseded_by_id"   bigint,
  "deleted_at"         timestamptz,
  "created_at"         timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"         timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "buyer_invoice_preferences_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_bip_superseded" FOREIGN KEY ("superseded_by_id") REFERENCES "buyer_invoice_preferences" ("id"),
  CONSTRAINT "chk_bip_kind" CHECK ("preference_kind" IN ('MOBILE_BARCODE','CITIZEN_CERT','TAX_ID','DONATION')),
  CONSTRAINT "chk_bip_shape" CHECK (
    ("preference_kind" = 'MOBILE_BARCODE' AND COALESCE("carrier_type", '') = '3J0002' AND "carrier_value_ciphertext" IS NOT NULL AND "tax_id" IS NULL AND "donation_code" IS NULL)
 OR ("preference_kind" = 'CITIZEN_CERT'   AND COALESCE("carrier_type", '') = 'CQ0001' AND "carrier_value_ciphertext" IS NOT NULL AND "tax_id" IS NULL AND "donation_code" IS NULL)
 OR ("preference_kind" = 'TAX_ID'         AND "carrier_type" IS NULL AND "carrier_value_ciphertext" IS NULL AND "tax_id" IS NOT NULL AND "tax_title" IS NOT NULL AND "donation_code" IS NULL)
 OR ("preference_kind" = 'DONATION'       AND "carrier_type" IS NULL AND "carrier_value_ciphertext" IS NULL AND "tax_id" IS NULL AND "donation_code" IS NOT NULL)),
  CONSTRAINT "chk_bip_tax_id"          CHECK ("tax_id" IS NULL OR "tax_id" ~ '^[0-9]{8}$'),
  CONSTRAINT "chk_bip_donation_code"   CHECK ("donation_code" IS NULL OR "donation_code" ~ '^[0-9]{3,7}$'),
  CONSTRAINT "chk_bip_hash_pair"       CHECK (("carrier_value_ciphertext" IS NULL) = ("carrier_value_hash" IS NULL)),
  CONSTRAINT "chk_bip_superseded_pair" CHECK (("superseded_at" IS NULL) = ("superseded_by_id" IS NULL)),
  CONSTRAINT "chk_bip_default_alive"   CHECK (NOT "is_default" OR ("superseded_at" IS NULL AND "deleted_at" IS NULL))
);

CREATE UNIQUE INDEX "uq_bip_default" ON "buyer_invoice_preferences" ("buyer_id")
  WHERE "is_default" AND "deleted_at" IS NULL AND "superseded_at" IS NULL;
CREATE UNIQUE INDEX "uq_bip_carrier_dedup" ON "buyer_invoice_preferences" ("buyer_id", "carrier_type", "carrier_value_hash")
  WHERE "carrier_value_hash" IS NOT NULL AND "deleted_at" IS NULL AND "superseded_at" IS NULL;
CREATE UNIQUE INDEX "uq_bip_taxid_dedup" ON "buyer_invoice_preferences" ("buyer_id", "tax_id")
  WHERE "tax_id" IS NOT NULL AND "deleted_at" IS NULL AND "superseded_at" IS NULL;
CREATE INDEX "idx_bip_buyer" ON "buyer_invoice_preferences" ("buyer_id", "created_at" DESC)
  WHERE "deleted_at" IS NULL AND "superseded_at" IS NULL;

CREATE TABLE "invoice_documents" (
  "id"                    bigint NOT NULL,
  "issuer_id"             bigint NOT NULL,
  "issuer_name_snapshot"  varchar(120) NOT NULL,
  "issuer_tax_id_snapshot" varchar(8),
  "source_type"           varchar(16) NOT NULL,
  "source_ref"            varchar(64),
  "trigger_type"          varchar(16) NOT NULL,
  "recipient_type"        varchar(16) NOT NULL,
  "recipient_ref_id"      bigint,
  "recipient_name"        varchar(60) NOT NULL,
  "recipient_tax_id"      varchar(10) NOT NULL DEFAULT '0000000000',
  "recipient_email_ciphertext" text,
  "recipient_email_hash"  varchar(64),
  "recipient_phone_ciphertext" text,
  "carrier_type"          varchar(6),
  "carrier_id1_ciphertext" text,
  "carrier_id2_ciphertext" text,
  "carrier_value_hash"    varchar(64),
  "donation_code"         varchar(7),
  "document_kind"         varchar(16) NOT NULL,
  "fulfillment_mode"      varchar(24) NOT NULL,
  "status"                varchar(24) NOT NULL DEFAULT 'PENDING',
  "not_applicable_reason" varchar(32),
  "manual_reason_code"    varchar(32),
  "cancel_reason_code"    varchar(48),
  "cancel_reason"         text,
  "snapshot_version"      integer NOT NULL DEFAULT 1,
  "issue_attempt_seq"     integer NOT NULL DEFAULT 1,
  "provider_account_version" bigint,
  "superseded_by_id"      bigint,
  "original_document_id"  bigint,
  "sales_amount"          bigint NOT NULL,
  "free_tax_sales_amount" bigint NOT NULL DEFAULT 0,
  "zero_tax_sales_amount" bigint NOT NULL DEFAULT 0,
  "tax_amount"            bigint NOT NULL DEFAULT 0,
  "total_amount"          bigint NOT NULL,
  "tax_type"              smallint NOT NULL DEFAULT 1,
  "tax_rate_bps"          integer NOT NULL DEFAULT 500,
  "allowance_total_amount" bigint NOT NULL DEFAULT 0,
  "voided_at"             timestamptz,
  "provider"              varchar(16),
  "provider_request_no"   varchar(40),
  "provider_invoice_no"   varchar(10),
  "provider_random_code"  varchar(4),
  "provider_issued_at"    timestamptz,
  "external_invoice_no"   varchar(10),
  "external_invoice_date" date,
  "external_reported_at"  timestamptz,
  "provider_upload_status" smallint,
  "provider_upload_checked_at" timestamptz,
  "upload_recheck_next_at" timestamptz,
  "upload_recheck_attempts" smallint NOT NULL DEFAULT 0,
  "receipt_no"            varchar(20),
  "receipt_authorization_version" varchar(32),
  "pdf_object_key"        varchar(512),
  "pdf_fetch_next_at"     timestamptz,
  "pdf_fetch_attempts"    smallint NOT NULL DEFAULT 0,
  "invoice_date_taipei"   date,
  "tax_period_code"       char(5),
  "issue_not_before"      timestamptz NOT NULL,
  "paid_at"               timestamptz,
  "statutory_deadline_at" timestamptz,
  "source_snapshot"       jsonb,
  "notification_resent_at"    timestamptz,
  "notification_resend_count" integer NOT NULL DEFAULT 0,
  "version"               bigint NOT NULL DEFAULT 0,
  "created_at"            timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"            timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_documents_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invdoc_issuer"     FOREIGN KEY ("issuer_id")            REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "fk_invdoc_superseded" FOREIGN KEY ("superseded_by_id")     REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "fk_invdoc_original"   FOREIGN KEY ("original_document_id") REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "chk_invdoc_source"    CHECK ("source_type" IN ('ORDER','MANUAL')),
  CONSTRAINT "chk_invdoc_trigger"   CHECK ("trigger_type" IN ('AUTO','ADM_MANUAL')),
  CONSTRAINT "chk_invdoc_recipient" CHECK ("recipient_type" IN ('BUYER','SELLER','EXTERNAL')),
  CONSTRAINT "chk_invdoc_kind"      CHECK ("document_kind" IN ('B2C','B2B','RECEIPT')),
  CONSTRAINT "chk_invdoc_mode"      CHECK ("fulfillment_mode" IN ('AMEGO_AUTO','SELLER_SELF_ISSUE','PLATFORM_RECEIPT','LEGACY_MANUAL')),
  CONSTRAINT "chk_invdoc_status"    CHECK ("status" IN ('PENDING','ISSUING','ISSUED','ISSUE_FAILED','AWAITING_MANUAL','VOIDING','VOIDED','SUPERSEDED','CANCELLED_BEFORE_ISSUE','NOT_APPLICABLE','RESOLVED_EXTERNALLY')),
  CONSTRAINT "chk_invdoc_tax_type"  CHECK ("tax_type" IN (1,2,3,9)),
  CONSTRAINT "chk_invdoc_source_ref" CHECK (("source_type" = 'ORDER') = ("source_ref" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_total" CHECK ("total_amount" = "sales_amount" + "free_tax_sales_amount" + "zero_tax_sales_amount" + "tax_amount"),
  CONSTRAINT "chk_invdoc_amounts_nonneg" CHECK ("sales_amount" >= 0 AND "free_tax_sales_amount" >= 0 AND "zero_tax_sales_amount" >= 0 AND "tax_amount" >= 0 AND "total_amount" >= 0),
  CONSTRAINT "chk_invdoc_b2c" CHECK ("document_kind" <> 'B2C' OR ("tax_amount" = 0 AND "recipient_tax_id" = '0000000000')),
  CONSTRAINT "chk_invdoc_b2b" CHECK ("document_kind" <> 'B2B' OR "recipient_tax_id" ~ '^[0-9]{8}$'),
  CONSTRAINT "chk_invdoc_taxid_shape" CHECK ("recipient_tax_id" ~ '^([0-9]{8}|0000000000)$'),
  CONSTRAINT "chk_invdoc_exclusive" CHECK (
    (CASE WHEN "recipient_tax_id" <> '0000000000' THEN 1 ELSE 0 END)
  + (CASE WHEN "carrier_type" IS NOT NULL THEN 1 ELSE 0 END)
  + (CASE WHEN "donation_code" IS NOT NULL THEN 1 ELSE 0 END) <= 1),
  CONSTRAINT "chk_invdoc_carrier_pair" CHECK (("carrier_type" IS NULL) = ("carrier_id1_ciphertext" IS NULL)),
  CONSTRAINT "chk_invdoc_donation" CHECK ("donation_code" IS NULL OR "donation_code" ~ '^[0-9]{3,7}$'),
  CONSTRAINT "chk_invdoc_issued_has_no" CHECK ("fulfillment_mode" <> 'AMEGO_AUTO' OR "status" <> 'ISSUED' OR "provider_invoice_no" IS NOT NULL),
  CONSTRAINT "chk_invdoc_receipt" CHECK (("document_kind" = 'RECEIPT') = ("receipt_no" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_receipt_no_provider" CHECK ("document_kind" <> 'RECEIPT' OR ("provider_invoice_no" IS NULL AND "provider" IS NULL)),
  CONSTRAINT "chk_invdoc_receipt_tax" CHECK ("document_kind" <> 'RECEIPT' OR "tax_amount" = 0),
  CONSTRAINT "chk_invdoc_self_issue" CHECK ("fulfillment_mode" NOT IN ('SELLER_SELF_ISSUE', 'LEGACY_MANUAL') OR "status" IN ('NOT_APPLICABLE', 'RESOLVED_EXTERNALLY', 'CANCELLED_BEFORE_ISSUE')),
  CONSTRAINT "chk_invdoc_external_no" CHECK (
    ("status" = 'RESOLVED_EXTERNALLY') = ("external_invoice_no" IS NOT NULL)
    AND ("status" = 'RESOLVED_EXTERNALLY') = ("external_invoice_date" IS NOT NULL)
    AND ("status" = 'RESOLVED_EXTERNALLY') = ("external_reported_at" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_external_reported_after" CHECK ("external_reported_at" IS NULL OR "external_reported_at" >= "issue_not_before"),
  CONSTRAINT "chk_invdoc_external_no_shape" CHECK ("external_invoice_no" IS NULL OR "external_invoice_no" ~ '^[A-Z]{2}[0-9]{8}$'),
  CONSTRAINT "chk_invdoc_external_excl" CHECK ("external_invoice_no" IS NULL OR "provider_invoice_no" IS NULL),
  CONSTRAINT "chk_invdoc_kind_mode" CHECK (("document_kind" = 'RECEIPT') = ("fulfillment_mode" = 'PLATFORM_RECEIPT')),
  CONSTRAINT "chk_invdoc_na_reason" CHECK (("status" = 'NOT_APPLICABLE') = ("not_applicable_reason" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_manual_reason" CHECK ("status" <> 'AWAITING_MANUAL' OR "manual_reason_code" IS NOT NULL),
  CONSTRAINT "chk_invdoc_voided_pair" CHECK (("status" = 'VOIDED') = ("voided_at" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_resend_pair" CHECK (("notification_resent_at" IS NULL) = ("notification_resend_count" = 0)),
  CONSTRAINT "chk_invdoc_resend_count_nonneg" CHECK ("notification_resend_count" >= 0),
  CONSTRAINT "chk_invdoc_allowance_cap" CHECK ("allowance_total_amount" >= 0 AND "allowance_total_amount" <= "total_amount"),
  CONSTRAINT "chk_invdoc_request_no_len" CHECK ("provider_request_no" IS NULL OR length("provider_request_no") <= 40),
  CONSTRAINT "chk_invdoc_period" CHECK ("tax_period_code" IS NULL OR "tax_period_code" ~ '^[0-9]{5}$'),
  CONSTRAINT "chk_invdoc_versions" CHECK ("snapshot_version" >= 1 AND "issue_attempt_seq" >= 1 AND "version" >= 0),
  CONSTRAINT "chk_invdoc_deadline" CHECK ("statutory_deadline_at" IS NULL OR "statutory_deadline_at" > "issue_not_before"),
  CONSTRAINT "chk_invdoc_deadline_presence" CHECK (("document_kind" IN ('B2C','B2B')) = ("statutory_deadline_at" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_pdf_attempts" CHECK ("pdf_fetch_attempts" >= 0),
  CONSTRAINT "chk_invdoc_upload_recheck" CHECK ("upload_recheck_attempts" >= 0 AND ("upload_recheck_next_at" IS NULL OR "provider_upload_status" IS NULL OR "provider_upload_status" IN (1, 2, 3, 31, 32)))
);

CREATE UNIQUE INDEX "uq_invdoc_order_identity" ON "invoice_documents" ("issuer_id", "source_ref", "document_kind", "snapshot_version", "issue_attempt_seq") WHERE "source_type" = 'ORDER';
CREATE UNIQUE INDEX "uq_invdoc_provider_request_no" ON "invoice_documents" ("provider_request_no") WHERE "provider_request_no" IS NOT NULL;
CREATE UNIQUE INDEX "uq_invdoc_provider_invoice_no" ON "invoice_documents" ("provider", "provider_invoice_no") WHERE "provider_invoice_no" IS NOT NULL;
CREATE UNIQUE INDEX "uq_invdoc_receipt_no" ON "invoice_documents" ("receipt_no") WHERE "receipt_no" IS NOT NULL;

CREATE INDEX "idx_invdoc_recipient" ON "invoice_documents" ("recipient_type", "recipient_ref_id", "status", "created_at" DESC);
CREATE INDEX "idx_invdoc_issuer_status" ON "invoice_documents" ("issuer_id", "status", "created_at" DESC);
CREATE INDEX "idx_invdoc_source_ref" ON "invoice_documents" ("source_ref") WHERE "source_type" = 'ORDER';

COMMENT ON TABLE  invoice_documents IS
  '稅務文件（發票／收據）。法定保存 5 年，列為 PII 清理例外：帳號刪除不得連帶刪除本表任何一列，只關閉對外查詢（授權層）。本服務不存在任何 DELETE FROM invoice_documents 的路徑（不變式 I-40）。';
COMMENT ON COLUMN invoice_documents.recipient_name IS
  'AMEGO BuyerName。永不寫入自然人真實姓名：B2C 恆為常數「消費者」、B2B 為公司抬頭、RECEIPT／MANUAL 為 ADM 或賣家手填抬頭。因此本欄是明文且不進遮罩清單。';
COMMENT ON COLUMN invoice_documents.source_snapshot IS
  '開票判定依據的白名單快照（已遮罩）。禁存收件人姓名／電話／地址／買家 email／載具值。';
COMMENT ON COLUMN invoice_documents.statutory_deadline_at IS
  '法定存證期限（自然日）：自 invoice_date_taipei 之翌日起算，B2C 2 日、B2B 7 日，末日 23:59:59 Asia/Taipei，不因假日延長；RECEIPT 無法定期限故為 NULL。禁止以 +48h 計算。本欄是權威值，invoice_operations 上的同名欄是它的反正規化副本（I-52）。';
COMMENT ON COLUMN invoice_documents.external_invoice_no IS
  '賣家自行開立並回報的發票號碼（status=RESOLVED_EXTERNALLY 專用）。與 provider_invoice_no 互斥且永不互相 fallback：平台不冒充開立人。';

CREATE TABLE "invoice_document_lines" (
  "id"              bigserial NOT NULL,
  "document_id"     bigint NOT NULL,
  "line_seq"        integer NOT NULL,
  "line_type"       varchar(20) NOT NULL,
  "order_item_id"   bigint,
  "sku_id"          uuid,
  "description"     varchar(256) NOT NULL,
  "unit"            varchar(6),
  "quantity"        integer NOT NULL,
  "unit_price"      bigint NOT NULL,
  "net_amount"      bigint NOT NULL,
  "tax_category"    smallint NOT NULL DEFAULT 1,
  "tax_rate_bps"    integer NOT NULL DEFAULT 500,
  "remark"          varchar(40),
  "allocated_discount"  bigint NOT NULL DEFAULT 0,
  "allowanceable_base"  bigint NOT NULL DEFAULT 0,
  "allowanced_amount"   bigint NOT NULL DEFAULT 0,
  "created_at"      timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_document_lines_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invline_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id") ON DELETE CASCADE,
  CONSTRAINT "chk_invline_type" CHECK ("line_type" IN ('PRODUCT','SHIPPING','DISCOUNT')),
  CONSTRAINT "chk_invline_tax_category" CHECK ("tax_category" IN (1,2,3)),
  CONSTRAINT "chk_invline_qty" CHECK ("quantity" > 0),
  CONSTRAINT "chk_invline_amount" CHECK ("net_amount" = "unit_price" * "quantity"),
  CONSTRAINT "chk_invline_sign" CHECK (("line_type" = 'DISCOUNT' AND "net_amount" < 0) OR ("line_type" <> 'DISCOUNT' AND "net_amount" >= 0)),
  CONSTRAINT "chk_invline_seq" CHECK ("line_seq" >= 1),
  CONSTRAINT "chk_invline_seq_max" CHECK ("line_seq" <= 9999),
  CONSTRAINT "chk_invline_allowance_shape" CHECK (
    ("line_type" = 'DISCOUNT' AND "allocated_discount" = 0 AND "allowanceable_base" = 0 AND "allowanced_amount" = 0)
    OR ("line_type" <> 'DISCOUNT' AND "allocated_discount" >= 0 AND "allowanceable_base" = "net_amount" - "allocated_discount")),
  CONSTRAINT "chk_invline_allowanced_cap" CHECK ("allowanced_amount" >= 0 AND "allowanced_amount" <= "allowanceable_base")
);

CREATE UNIQUE INDEX "uq_invline_doc_seq" ON "invoice_document_lines" ("document_id", "line_seq");

CREATE TABLE "invoice_adjustments" (
  "id"                  bigint NOT NULL,
  "document_id"         bigint NOT NULL,
  "adjustment_type"     varchar(16) NOT NULL,
  "status"              varchar(32) NOT NULL DEFAULT 'PENDING',
  "trigger_kind"        varchar(24) NOT NULL,
  "refund_no"           varchar(32),
  "correction_request_id" bigint,
  "allowance_number"    varchar(16),
  "allowance_date_taipei" date,
  "net_amount_ex_tax"   bigint,
  "tax_amount"          bigint,
  "gross_amount"        bigint,
  "void_reason_code"    varchar(32),
  "void_reason_detail"  text,
  "blocked_reason_code" varchar(32),
  "cancel_reason_code"  varchar(48),
  "provider_confirmed_at"    timestamptz,
  "provider_upload_status"   smallint,
  "provider_upload_checked_at" timestamptz,
  "pdf_object_key"      varchar(512),
  "version"             bigint NOT NULL DEFAULT 0,
  "created_at"          timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"          timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_adjustments_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invadj_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "chk_invadj_type"   CHECK ("adjustment_type" IN ('VOID','ALLOWANCE')),
  CONSTRAINT "chk_invadj_status" CHECK ("status" IN ('PENDING','BLOCKED_UNKNOWN_DOCUMENT','BLOCKED_PENDING_CORRECTION','SENDING','WAITING_CONFIRMATION','COMPLETED','FAILED_PERMANENT','FAILED_UNKNOWN','CANCELLED')),
  CONSTRAINT "chk_invadj_blocked_reason" CHECK ("status" <> 'FAILED_PERMANENT' OR "blocked_reason_code" IS NOT NULL),
  CONSTRAINT "chk_invadj_trigger" CHECK ("trigger_kind" IN ('ORDER_REFUND','BUYER_CORRECTION','ADM_MANUAL')),
  CONSTRAINT "chk_invadj_refund_pair" CHECK (("trigger_kind" = 'ORDER_REFUND') = ("refund_no" IS NOT NULL)),
  CONSTRAINT "chk_invadj_correction_pair" CHECK (("trigger_kind" = 'BUYER_CORRECTION') = ("correction_request_id" IS NOT NULL)),
  CONSTRAINT "chk_invadj_allowance_shape" CHECK (
    ("adjustment_type" = 'ALLOWANCE' AND "allowance_number" IS NOT NULL AND "allowance_date_taipei" IS NOT NULL AND "net_amount_ex_tax" IS NOT NULL AND "tax_amount" IS NOT NULL AND "gross_amount" IS NOT NULL)
    OR ("adjustment_type" = 'VOID' AND "allowance_number" IS NULL AND "net_amount_ex_tax" IS NULL AND "tax_amount" IS NULL AND "gross_amount" IS NULL)),
  CONSTRAINT "chk_invadj_allowance_sum" CHECK ("gross_amount" IS NULL OR "gross_amount" = "net_amount_ex_tax" + "tax_amount"),
  CONSTRAINT "chk_invadj_allowance_positive" CHECK ("gross_amount" IS NULL OR ("gross_amount" > 0 AND "net_amount_ex_tax" >= 0 AND "tax_amount" >= 0)),
  CONSTRAINT "chk_invadj_allowance_no_len" CHECK ("allowance_number" IS NULL OR length("allowance_number") <= 16),
  CONSTRAINT "chk_invadj_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invadj_allowance_refund" ON "invoice_adjustments" ("document_id", "refund_no")
  WHERE "adjustment_type" = 'ALLOWANCE' AND "refund_no" IS NOT NULL AND "status" <> 'CANCELLED';
CREATE UNIQUE INDEX "uq_invadj_void_once" ON "invoice_adjustments" ("document_id")
  WHERE "adjustment_type" = 'VOID' AND "status" <> 'CANCELLED';
CREATE UNIQUE INDEX "uq_invadj_allowance_number" ON "invoice_adjustments" ("allowance_number") WHERE "allowance_number" IS NOT NULL;

CREATE TABLE "invoice_adjustment_lines" (
  "id"                 bigserial NOT NULL,
  "adjustment_id"      bigint NOT NULL,
  "line_seq"           integer NOT NULL,
  "source_line_id"     bigint,
  "original_invoice_no"     varchar(10) NOT NULL,
  "original_invoice_date"   date NOT NULL,
  "original_description"    varchar(256) NOT NULL,
  "quantity"           integer NOT NULL,
  "unit_price_ex_tax"  bigint NOT NULL,
  "net_amount_ex_tax"  bigint NOT NULL,
  "tax_amount"         bigint NOT NULL,
  "tax_category"       smallint NOT NULL DEFAULT 1,
  "created_at"         timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_adjustment_lines_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invadjline_adjustment" FOREIGN KEY ("adjustment_id") REFERENCES "invoice_adjustments" ("id") ON DELETE CASCADE,
  CONSTRAINT "fk_invadjline_source" FOREIGN KEY ("source_line_id") REFERENCES "invoice_document_lines" ("id"),
  CONSTRAINT "chk_invadjline_amount" CHECK ("net_amount_ex_tax" = "unit_price_ex_tax" * "quantity"),
  CONSTRAINT "chk_invadjline_qty" CHECK ("quantity" > 0),
  CONSTRAINT "chk_invadjline_tax_category" CHECK ("tax_category" IN (1,2,3)),
  CONSTRAINT "chk_invadjline_nonneg" CHECK ("net_amount_ex_tax" >= 0 AND "tax_amount" >= 0),
  CONSTRAINT "chk_invadjline_seq" CHECK ("line_seq" BETWEEN 1 AND 9999)
);

CREATE UNIQUE INDEX "uq_invadjline_seq" ON "invoice_adjustment_lines" ("adjustment_id", "line_seq");
CREATE INDEX "idx_invadj_document" ON "invoice_adjustments" ("document_id", "created_at");
CREATE INDEX "idx_invadj_status" ON "invoice_adjustments" ("status", "created_at")
  WHERE ((status)::text = ANY ((ARRAY['PENDING'::character varying, 'BLOCKED_UNKNOWN_DOCUMENT'::character varying, 'BLOCKED_PENDING_CORRECTION'::character varying, 'SENDING'::character varying, 'WAITING_CONFIRMATION'::character varying, 'FAILED_UNKNOWN'::character varying])::text[]));
CREATE INDEX "idx_invadjline_source" ON "invoice_adjustment_lines" ("source_line_id") WHERE "source_line_id" IS NOT NULL;

CREATE TABLE "invoice_operations" (
  "id"                  bigint NOT NULL,
  "document_id"         bigint NOT NULL,
  "adjustment_id"       bigint,
  "operation_type"      varchar(16) NOT NULL,
  "operation_key"       varchar(96) NOT NULL,
  "provider_request_no" varchar(40) NOT NULL,
  "status"              varchar(28) NOT NULL DEFAULT 'READY',
  "request_snapshot_hash" varchar(64) NOT NULL,
  "attempt_count"       integer NOT NULL DEFAULT 0,
  "max_attempts"        integer NOT NULL DEFAULT 13,
  "next_attempt_at"     timestamptz NOT NULL DEFAULT NOW(),
  "held_until"          timestamptz,
  "recheck_list_page"   smallint NOT NULL DEFAULT 0,
  "statutory_deadline_at" timestamptz,
  "last_http_status"    integer,
  "provider_code"       varchar(16),
  "provider_message"    text,
  "unknown_result"      boolean NOT NULL DEFAULT false,
  "unknown_since"       timestamptz,
  "cancel_requested_by"     bigint,
  "cancel_requested_at"     timestamptz,
  "cancel_confirmed_by"     bigint,
  "cancel_confirmed_at"     timestamptz,
  "cancel_evidence_kind"       varchar(48),
  "cancel_evidence_object_key" text,
  "cancel_reason_code"      varchar(48),
  "cancel_reason"           text,
  "started_at"          timestamptz,
  "completed_at"        timestamptz,
  "last_error"          text,
  "version"             bigint NOT NULL DEFAULT 0,
  "created_at"          timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"          timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_operations_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invop_document"   FOREIGN KEY ("document_id")   REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "fk_invop_adjustment" FOREIGN KEY ("adjustment_id") REFERENCES "invoice_adjustments" ("id"),
  CONSTRAINT "chk_invop_type" CHECK ("operation_type" IN ('ISSUE','QUERY','VOID','ALLOWANCE','REISSUE','VOID_ALLOWANCE')),
  CONSTRAINT "chk_invop_status" CHECK ("status" IN ('CREATED','AWAITING_TRIGGER','READY','HELD','SENDING','RETRY_SCHEDULED','WAITING_CONFIRMATION','PROVIDER_STATUS_RECHECK','SUCCEEDED','FAILED_PERMANENT','FAILED_RETRY_EXHAUSTED','FAILED_UNKNOWN','SELLER_ACTION_REQUIRED','CANCEL_REQUESTED','CANCEL_BLOCKED_UNKNOWN','CANCELLED_BY_ADMIN','MANUAL_PROCESS_ALLOWED','RESOLVED_EXTERNALLY','NOT_APPLICABLE','CANCELLED_BEFORE_ISSUE')),
  CONSTRAINT "chk_invop_adjustment_pair" CHECK (("operation_type" IN ('VOID','ALLOWANCE','VOID_ALLOWANCE')) = ("adjustment_id" IS NOT NULL)),
  CONSTRAINT "chk_invop_held_pair" CHECK (("status" = 'HELD') = ("held_until" IS NOT NULL)),
  CONSTRAINT "chk_invop_recheck_page" CHECK ("recheck_list_page" >= 0),
  CONSTRAINT "chk_invop_attempts" CHECK ("attempt_count" >= 0 AND "max_attempts" >= 1),
  CONSTRAINT "chk_invop_unknown_pair" CHECK (("unknown_result" = false) = ("unknown_since" IS NULL)),
  CONSTRAINT "chk_invop_two_person" CHECK ("cancel_confirmed_by" IS NULL OR ("cancel_requested_by" IS NOT NULL AND "cancel_confirmed_by" <> "cancel_requested_by")),
  CONSTRAINT "chk_invop_cancel_pair" CHECK (("cancel_requested_by" IS NULL) = ("cancel_requested_at" IS NULL)),
  CONSTRAINT "chk_invop_admin_cancel_needs_confirm" CHECK ("status" <> 'CANCELLED_BY_ADMIN' OR "cancel_confirmed_by" IS NOT NULL OR COALESCE("cancel_evidence_kind", '') IN ('PROVIDER_QUERY_71', 'PROVIDER_LIST_ABSENT')),
  CONSTRAINT "chk_invop_evidence_object_key" CHECK ("cancel_evidence_object_key" IS NULL OR "cancel_evidence_kind" IN ('MOF_PLATFORM_SCREENSHOT', 'AMEGO_RECONCILE_FILE')),
  CONSTRAINT "chk_invop_request_no_len" CHECK (length("provider_request_no") <= 40),
  CONSTRAINT "chk_invop_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invop_operation_key" ON "invoice_operations" ("operation_key");
CREATE UNIQUE INDEX "uq_invop_open_adjustment_per_document" ON "invoice_operations" ("document_id") WHERE (((operation_type)::text = ANY ((ARRAY['VOID'::character varying, 'REISSUE'::character varying, 'ALLOWANCE'::character varying, 'VOID_ALLOWANCE'::character varying])::text[])) AND ((status)::text <> ALL ((ARRAY['SUCCEEDED'::character varying, 'NOT_APPLICABLE'::character varying, 'CANCELLED_BEFORE_ISSUE'::character varying, 'RESOLVED_EXTERNALLY'::character varying, 'CANCELLED_BY_ADMIN'::character varying])::text[])));
CREATE INDEX "idx_invop_document" ON "invoice_operations" ("document_id", "created_at" DESC);
CREATE INDEX "idx_invop_adjustment" ON "invoice_operations" ("adjustment_id", "created_at" DESC) WHERE "adjustment_id" IS NOT NULL;

CREATE TABLE "invoice_inbox_events" (
  "id"               bigserial NOT NULL,
  "event_id"         varchar(64) NOT NULL,
  "event_type"       varchar(64) NOT NULL,
  "routing_key"      varchar(128) NOT NULL,
  "exchange"         varchar(64) NOT NULL,
  "business_dedup_key" varchar(160) NOT NULL,
  "aggregate_ref"    varchar(64),
  "payload"          jsonb NOT NULL,
  "status"           varchar(16) NOT NULL DEFAULT 'RECEIVED',
  "disposition"      varchar(40),
  "disposition_reason" text,
  "attempt_count"    integer NOT NULL DEFAULT 0,
  "next_attempt_at"  timestamptz NOT NULL DEFAULT NOW(),
  "deferred_until"   timestamptz,
  "last_error"       text,
  "occurred_at"      timestamptz NOT NULL,
  "received_at"      timestamptz NOT NULL DEFAULT NOW(),
  "processed_at"     timestamptz,
  "payload_purged_at" timestamptz,
  CONSTRAINT "invoice_inbox_events_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_invinbox_status" CHECK ("status" IN ('RECEIVED','DEFERRED','PROCESSED','SKIPPED','FAILED')),
  CONSTRAINT "chk_invinbox_skip_reason" CHECK ("status" <> 'SKIPPED' OR "disposition" IS NOT NULL),
  CONSTRAINT "chk_invinbox_attempts" CHECK ("attempt_count" >= 0)
);

CREATE UNIQUE INDEX "uq_invinbox_event_id" ON "invoice_inbox_events" ("event_id");
CREATE UNIQUE INDEX "uq_invinbox_business_key" ON "invoice_inbox_events" ("event_type", "business_dedup_key");

CREATE TABLE "invoice_cancelled_orders" (
  "order_no"      varchar(64) NOT NULL,
  "buyer_id"      bigint NOT NULL,
  "cancel_reason" varchar(32),
  "occurred_at"   timestamptz NOT NULL,
  "created_at"    timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_cancelled_orders_pkey" PRIMARY KEY ("order_no"),
  CONSTRAINT "chk_invcanord_buyer" CHECK ("buyer_id" > 0)
);

CREATE INDEX "idx_invcanord_order_buyer" ON "invoice_cancelled_orders" ("order_no", "buyer_id");

CREATE TABLE "invoice_outbox_events" (
  "id"                 bigserial NOT NULL,
  "aggregate_type"     varchar(24) NOT NULL,
  "aggregate_id"       bigint NOT NULL,
  "event_type"         varchar(64) NOT NULL,
  "exchange"           varchar(64) NOT NULL DEFAULT 'hawkr.invoice',
  "routing_key"        varchar(128) NOT NULL,
  "payload"            jsonb NOT NULL,
  "status"             varchar(16) NOT NULL DEFAULT 'pending',
  "attempt_count"      integer NOT NULL DEFAULT 0,
  "next_attempt_at"    timestamptz NOT NULL DEFAULT NOW(),
  "last_error"         text,
  "created_at"         timestamptz NOT NULL DEFAULT NOW(),
  "completed_at"       timestamptz,
  CONSTRAINT "invoice_outbox_events_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_invoutbox_status" CHECK ("status" IN ('pending','done','failed')),
  CONSTRAINT "chk_invoutbox_attempts" CHECK ("attempt_count" >= 0)
);

CREATE TABLE "invoice_correction_requests" (
  "id"                 bigint NOT NULL,
  "document_id"        bigint NOT NULL,
  "requested_by_buyer_id" bigint NOT NULL,
  "expected_snapshot_version" integer NOT NULL,
  "before_payload"     jsonb NOT NULL,
  "after_payload"      jsonb NOT NULL,
  "status"             varchar(32) NOT NULL DEFAULT 'REQUESTED',
  "handling_mode"      varchar(24),
  "reject_reason_code" varchar(32),
  "reject_reason_detail" text,
  "void_adjustment_id" bigint,
  "reissued_document_id" bigint,
  "version"            bigint NOT NULL DEFAULT 0,
  "created_at"         timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"         timestamptz NOT NULL DEFAULT NOW(),
  "completed_at"       timestamptz,
  CONSTRAINT "invoice_correction_requests_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invcorr_document"  FOREIGN KEY ("document_id")          REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "fk_invcorr_void"      FOREIGN KEY ("void_adjustment_id")   REFERENCES "invoice_adjustments" ("id"),
  CONSTRAINT "fk_invcorr_reissued"  FOREIGN KEY ("reissued_document_id") REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "chk_invcorr_status" CHECK ("status" IN ('REQUESTED','REJECTED','UPDATED_BEFORE_ISSUE','VOID_REISSUE_IN_PROGRESS','COMPLETED','REISSUE_FAILED')),
  CONSTRAINT "chk_invcorr_mode" CHECK ("handling_mode" IS NULL OR "handling_mode" IN ('UPDATE_SNAPSHOT','VOID_AND_REISSUE')),
  CONSTRAINT "chk_invcorr_reject_reason" CHECK ("status" <> 'REJECTED' OR "reject_reason_code" IS NOT NULL),
  CONSTRAINT "chk_invcorr_reject_reason_code" CHECK ("reject_reason_code" IS NULL OR "reject_reason_code" IN ('ALREADY_REPORTED','PAST_VOID_DEADLINE','ALLOWANCE_EXISTS','TRACK_INSUFFICIENT','CARRIER_INVALID','PROVIDER_UNAVAILABLE','PROVIDER_STATE_CONFLICT')),
  CONSTRAINT "chk_invcorr_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invcorr_open_per_document" ON "invoice_correction_requests" ("document_id") WHERE ((status)::text = ANY ((ARRAY['REQUESTED'::character varying, 'VOID_REISSUE_IN_PROGRESS'::character varying])::text[]));

CREATE TABLE "invoice_complaints" (
  "id"                  bigint NOT NULL,
  "document_id"         bigint NOT NULL,
  "issuer_id"           bigint NOT NULL,
  "order_no"            varchar(64),
  "complainant_buyer_id" bigint NOT NULL,
  "reason_code"         varchar(32) NOT NULL DEFAULT 'NOT_RECEIVED',
  "description"         text,
  "status"              varchar(32) NOT NULL DEFAULT 'OPEN',
  "sla_due_at"          timestamptz NOT NULL,
  "overdue_at"          timestamptz,
  "buyer_review_due_at" timestamptz,
  "first_responded_at"  timestamptz,
  "resolved_at"         timestamptz,
  "resolution_code"     varchar(32),
  "resolution_note"     text,
  "reopen_count"        smallint NOT NULL DEFAULT 0,
  "reopen_allowed_after" timestamptz,
  "decided_by_account_id" bigint,
  "decided_at"          timestamptz,
  "version"             bigint NOT NULL DEFAULT 0,
  "created_at"          timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"          timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_complaints_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invcomp_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "fk_invcomp_issuer"   FOREIGN KEY ("issuer_id")   REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "chk_invcomp_status" CHECK ("status" IN ('OPEN','SELLER_RESPONDED','OVERDUE','ESCALATED_TO_ADM','RESOLVED','VIOLATION_CONFIRMED','WITHDRAWN')),
  CONSTRAINT "chk_invcomp_overdue_pair" CHECK ("status" <> 'OVERDUE' OR "overdue_at" IS NOT NULL),
  CONSTRAINT "chk_invcomp_reopen" CHECK ("reopen_count" BETWEEN 0 AND 2),
  CONSTRAINT "chk_invcomp_resolved_pair" CHECK (("status" IN ('RESOLVED','VIOLATION_CONFIRMED')) = ("resolved_at" IS NOT NULL)),
  CONSTRAINT "chk_invcomp_violation_needs_decider" CHECK ("status" <> 'VIOLATION_CONFIRMED' OR "decided_by_account_id" IS NOT NULL),
  CONSTRAINT "chk_invcomp_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invcomp_open_per_document" ON "invoice_complaints" ("document_id") WHERE ((status)::text = ANY ((ARRAY['OPEN'::character varying, 'OVERDUE'::character varying, 'SELLER_RESPONDED'::character varying, 'ESCALATED_TO_ADM'::character varying, 'RESOLVED'::character varying])::text[]));

CREATE TABLE "invoice_complaint_events" (
  "id"              bigserial NOT NULL,
  "complaint_id"    bigint NOT NULL,
  "event_type"      varchar(32) NOT NULL,
  "action"          varchar(24),
  "actor_type"      varchar(16) NOT NULL,
  "actor_account_id" bigint,
  "message"         text,
  "attachment_object_keys" jsonb NOT NULL DEFAULT '[]',
  "occurred_at"     timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_complaint_events_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invcompev_complaint" FOREIGN KEY ("complaint_id") REFERENCES "invoice_complaints" ("id") ON DELETE CASCADE,
  CONSTRAINT "chk_invcompev_actor" CHECK ("actor_type" IN ('BUYER','SELLER','ADMIN','SYSTEM')),
  CONSTRAINT "chk_invcompev_actor_id" CHECK ("actor_type" = 'SYSTEM' OR "actor_account_id" IS NOT NULL)
);

CREATE INDEX "idx_invcompev_complaint" ON "invoice_complaint_events" ("complaint_id", "occurred_at");
CREATE INDEX "idx_invcomp_issuer_status" ON "invoice_complaints" ("issuer_id", "status", "created_at" DESC);
CREATE INDEX "idx_invcomp_violation" ON "invoice_complaints" ("issuer_id", "resolved_at") WHERE "status" = 'VIOLATION_CONFIRMED';

CREATE TABLE "invoice_status_history" (
  "id"              bigserial NOT NULL,
  "subject_type"    varchar(24) NOT NULL,
  "subject_id"      bigint NOT NULL,
  "from_status"     varchar(32),
  "to_status"       varchar(32) NOT NULL,
  "reason_code"     varchar(48),
  "reason_detail"   text,
  "actor_type"      varchar(16) NOT NULL,
  "actor_account_id" bigint,
  "correlation_id"  varchar(64),
  "occurred_at"     timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_status_history_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_invhist_subject" CHECK ("subject_type" IN ('DOCUMENT','ADJUSTMENT','OPERATION','CORRECTION_REQUEST','TAX_PROFILE','PROVIDER_ACCOUNT','COMPLAINT')),
  CONSTRAINT "chk_invhist_actor" CHECK ("actor_type" IN ('SYSTEM','ADMIN','SELLER','BUYER','PROVIDER'))
);

CREATE INDEX "idx_invhist_subject" ON "invoice_status_history" ("subject_type", "subject_id", "occurred_at");

CREATE TABLE "invoice_provider_audit_logs" (
  "id"                bigserial NOT NULL,
  "operation_id"      bigint,
  "issuer_id"         bigint NOT NULL,
  "provider"          varchar(16) NOT NULL DEFAULT 'AMEGO',
  "endpoint"          varchar(64) NOT NULL,
  "attempt_seq"       integer NOT NULL,
  "request_masked"    jsonb NOT NULL,
  "response_masked"   jsonb,
  "raw_body_hash"     varchar(64) NOT NULL,
  "http_status"       integer,
  "provider_code"     varchar(16),
  "provider_message"  text,
  "latency_ms"        integer,
  "error_kind"        varchar(32),
  "correlation_id"    varchar(64),
  "occurred_at"       timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_provider_audit_logs_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invpaudit_operation" FOREIGN KEY ("operation_id") REFERENCES "invoice_operations" ("id"),
  CONSTRAINT "chk_invpaudit_attempt" CHECK ("attempt_seq" >= 1),
  CONSTRAINT "chk_invpaudit_latency" CHECK ("latency_ms" IS NULL OR "latency_ms" >= 0)
);

CREATE INDEX "idx_invpaudit_operation" ON "invoice_provider_audit_logs" ("operation_id", "attempt_seq");
CREATE INDEX "idx_invpaudit_issuer_time" ON "invoice_provider_audit_logs" ("issuer_id", "occurred_at" DESC);

CREATE TABLE "invoice_admin_audit_logs" (
  "id"                bigserial NOT NULL,
  "actor_account_id"  bigint NOT NULL,
  "action"            varchar(64) NOT NULL,
  "resource_type"     varchar(32) NOT NULL,
  "resource_id"       bigint NOT NULL,
  "before_state"      jsonb,
  "after_state"       jsonb,
  "reason_code"       varchar(48) NOT NULL,
  "reason_detail"     text NOT NULL,
  "approval_actor_account_id" bigint,
  "trace_id"          varchar(64),
  "request_id"        varchar(64),
  "correlation_id"    varchar(64),
  "client_ip"         varchar(45),
  "user_agent"        varchar(512),
  "occurred_at"       timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_admin_audit_logs_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_invadmaudit_reason" CHECK (length("reason_detail") >= 1),
  CONSTRAINT "chk_invadmaudit_two_person" CHECK ("approval_actor_account_id" IS NULL OR "approval_actor_account_id" <> "actor_account_id")
);

CREATE INDEX "idx_invadmaudit_resource" ON "invoice_admin_audit_logs" ("resource_type", "resource_id", "occurred_at" DESC);
CREATE INDEX "idx_invadmaudit_actor" ON "invoice_admin_audit_logs" ("actor_account_id", "occurred_at" DESC);
CREATE UNIQUE INDEX "uq_invadmaudit_client_request" ON "invoice_admin_audit_logs" ("actor_account_id", "action", "request_id")
  WHERE "request_id" IS NOT NULL;

CREATE TABLE "invoice_business_day_mirror" (
  "holiday_date"  date NOT NULL,
  "synced_at"     timestamptz NOT NULL,
  CONSTRAINT "invoice_business_day_mirror_pkey" PRIMARY KEY ("holiday_date")
);

COMMENT ON TABLE invoice_business_day_mirror IS
  '台灣國定假日唯讀鏡像。owner 是單體的 platform/bizcalendar；本表由單體呼叫 ReplaceBusinessDayMirror 整段覆蓋。禁止人工寫入、禁止 ADM 編輯、禁止在本服務加 seed migration。';

CREATE TABLE "invoice_business_day_coverage" (
  "singleton"    boolean NOT NULL DEFAULT true,
  "covered_from" date NOT NULL,
  "covered_to"   date NOT NULL,
  "synced_at"    timestamptz NOT NULL,
  CONSTRAINT "invoice_business_day_coverage_pkey" PRIMARY KEY ("singleton"),
  CONSTRAINT "chk_invbizcov_singleton" CHECK ("singleton"),
  CONSTRAINT "chk_invbizcov_range" CHECK ("covered_from" <= "covered_to")
);

COMMENT ON TABLE invoice_business_day_coverage IS
  '行事曆鏡像的涵蓋區間（單列）。由 ReplaceBusinessDayMirror 與假日清單同一交易覆蓋；沒有列代表鏡像從未被推送過。';

CREATE TABLE "invoice_provider_cutovers" (
  "id"                    bigserial NOT NULL,
  "provider_account_id"   bigint NOT NULL,
  "issuer_id"             bigint NOT NULL,
  "account_version"       bigint NOT NULL,
  "cutover_at"            timestamptz NOT NULL,
  "deactivated_at"        timestamptz,
  "mode"                  varchar(24) NOT NULL DEFAULT 'AMEGO_AUTO',
  "created_by_account_id" bigint NOT NULL,
  "reason"                text,
  "created_at"            timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_provider_cutovers_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invpc_account" FOREIGN KEY ("provider_account_id") REFERENCES "invoice_provider_accounts" ("id"),
  CONSTRAINT "fk_invpc_issuer"  FOREIGN KEY ("issuer_id")          REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "chk_invpc_mode" CHECK ("mode" IN ('AMEGO_AUTO')),
  CONSTRAINT "chk_invpc_version_nonneg" CHECK ("account_version" >= 0),
  CONSTRAINT "chk_invpc_deactivated_after" CHECK ("deactivated_at" IS NULL OR "deactivated_at" >= "created_at")
);

CREATE UNIQUE INDEX "uq_invpc_open_per_account" ON "invoice_provider_cutovers" ("provider_account_id") WHERE "deactivated_at" IS NULL;
CREATE INDEX "idx_invpc_account_time" ON "invoice_provider_cutovers" ("provider_account_id", "created_at" DESC);

CREATE TABLE "invoice_manual_unlocks" (
  "id"                      bigint NOT NULL,
  "operation_id"            bigint NOT NULL,
  "status"                  varchar(24) NOT NULL DEFAULT 'AWAITING_SECOND_APPROVAL',
  "requested_by_account_id" bigint NOT NULL,
  "request_reason"          text NOT NULL,
  "provider_check_evidence" text NOT NULL,
  "provider_checked_at"     timestamptz NOT NULL,
  "requested_at"            timestamptz NOT NULL DEFAULT NOW(),
  "expires_at"              timestamptz NOT NULL,
  "cancel_evidence_kind"       varchar(48),
  "cancel_evidence_object_key" text,
  "approved_by_account_id"  bigint,
  "decision_reason"         text,
  "decided_at"              timestamptz,
  "version"                 bigint NOT NULL DEFAULT 0,
  "created_at"              timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"              timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_manual_unlocks_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invmu_operation" FOREIGN KEY ("operation_id") REFERENCES "invoice_operations" ("id"),
  CONSTRAINT "chk_invmu_status" CHECK ("status" IN ('AWAITING_SECOND_APPROVAL','APPROVED','REJECTED','EXPIRED')),
  CONSTRAINT "chk_invmu_evidence_len" CHECK (length("provider_check_evidence") BETWEEN 1 AND 1000),
  CONSTRAINT "chk_invmu_reason_len" CHECK (length("request_reason") >= 1),
  CONSTRAINT "chk_invmu_expiry_window" CHECK ("expires_at" > "requested_at"),
  CONSTRAINT "chk_invmu_two_person" CHECK ("approved_by_account_id" IS NULL OR "approved_by_account_id" <> "requested_by_account_id"),
  CONSTRAINT "chk_invmu_decided_pair" CHECK (("status" IN ('APPROVED','REJECTED')) = ("approved_by_account_id" IS NOT NULL AND "decided_at" IS NOT NULL)),
  CONSTRAINT "chk_invmu_reject_reason" CHECK ("status" <> 'REJECTED' OR ("decision_reason" IS NOT NULL AND length("decision_reason") >= 1)),
  CONSTRAINT "chk_invmu_version_nonneg" CHECK ("version" >= 0),
  CONSTRAINT "chk_invmu_evidence_kind" CHECK ("cancel_evidence_kind" IS NULL OR "cancel_evidence_kind" IN ('MOF_PLATFORM_SCREENSHOT', 'AMEGO_RECONCILE_FILE')),
  CONSTRAINT "chk_invmu_evidence_object_key" CHECK ("cancel_evidence_object_key" IS NULL OR "cancel_evidence_kind" IS NOT NULL)
);

CREATE UNIQUE INDEX "uq_invmu_open_per_operation" ON "invoice_manual_unlocks" ("operation_id") WHERE "status" = 'AWAITING_SECOND_APPROVAL';
CREATE INDEX "idx_invmu_operation" ON "invoice_manual_unlocks" ("operation_id", "created_at" DESC);

CREATE INDEX "idx_invop_due" ON "invoice_operations" ("next_attempt_at") WHERE ((status)::text = ANY ((ARRAY['READY'::character varying, 'RETRY_SCHEDULED'::character varying, 'SENDING'::character varying])::text[]));
CREATE INDEX "idx_invop_recheck_due" ON "invoice_operations" ("next_attempt_at") WHERE ((status)::text = ANY ((ARRAY['WAITING_CONFIRMATION'::character varying, 'PROVIDER_STATUS_RECHECK'::character varying, 'FAILED_UNKNOWN'::character varying, 'CANCEL_REQUESTED'::character varying, 'CANCEL_BLOCKED_UNKNOWN'::character varying])::text[]));
CREATE INDEX "idx_invinbox_due" ON "invoice_inbox_events" ("next_attempt_at") WHERE ((status)::text = ANY ((ARRAY['RECEIVED'::character varying, 'DEFERRED'::character varying, 'FAILED'::character varying])::text[]));
CREATE INDEX "idx_invinbox_deferred_aggregate" ON "invoice_inbox_events" ("aggregate_ref") WHERE "status" = 'DEFERRED';
CREATE INDEX "idx_invoutbox_pending" ON "invoice_outbox_events" ("next_attempt_at") WHERE "status" = 'pending';
CREATE INDEX "idx_invcomp_sla_overdue" ON "invoice_complaints" ("sla_due_at") WHERE "status" = 'OPEN';
CREATE INDEX "idx_invcomp_overdue_escalate" ON "invoice_complaints" ("overdue_at") WHERE "status" = 'OVERDUE';
CREATE INDEX "idx_invcomp_buyer_review_due" ON "invoice_complaints" ("buyer_review_due_at") WHERE "status" = 'SELLER_RESPONDED';
CREATE INDEX "idx_invdoc_deadline" ON "invoice_documents" ("statutory_deadline_at") WHERE ((status)::text = ANY ((ARRAY['PENDING'::character varying, 'ISSUING'::character varying, 'ISSUE_FAILED'::character varying, 'AWAITING_MANUAL'::character varying])::text[]));
CREATE INDEX "idx_invdoc_upload_recheck" ON "invoice_documents" ("upload_recheck_next_at")
  WHERE "status" = 'ISSUED' AND "upload_recheck_next_at" IS NOT NULL
    AND ("provider_upload_status" IS NULL OR "provider_upload_status" IN (1,2,3,31,32));
CREATE INDEX "idx_invdoc_pdf_fetch" ON "invoice_documents" ("pdf_fetch_next_at")
  WHERE "pdf_fetch_next_at" IS NOT NULL AND "pdf_object_key" IS NULL;

CREATE SEQUENCE "invoice_allowance_number_seq" AS bigint START WITH 1 INCREMENT BY 1 NO CYCLE;
CREATE SEQUENCE "invoice_receipt_number_seq"   AS bigint START WITH 1 INCREMENT BY 1 NO CYCLE;

CREATE TABLE "invoice_idempotency_keys" (
  "scope"        varchar(64)  NOT NULL,
  "actor_id"     bigint       NOT NULL,
  "idem_key"     varchar(128) NOT NULL,
  "request_hash" char(64)     NOT NULL,
  "result_ref"   bigint,
  "created_at"   timestamptz  NOT NULL DEFAULT now(),
  CONSTRAINT "pk_invidem" PRIMARY KEY ("scope", "actor_id", "idem_key"),
  CONSTRAINT "chk_invidem_hash" CHECK ("request_hash" ~ '^[0-9a-f]{64}$'),
  CONSTRAINT "chk_invidem_key_nonblank" CHECK (length("idem_key") >= 1)
);

CREATE INDEX "idx_invidem_created" ON "invoice_idempotency_keys" ("created_at");

COMMENT ON TABLE invoice_idempotency_keys IS
  '冪等鍵（#3015）。與業務寫入同一個 transaction ⇒ 同一把鍵絕不產生第二筆。保留期由維運 SQL 依 created_at 清理，非永久帳。';
