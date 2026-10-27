CREATE TABLE "sellers" (
  "store_id" bigint PRIMARY KEY,
  "store_code" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "is_current" boolean NOT NULL,
  "store_name" varchar(255) NOT NULL,
  "description" text,
  "logo" varchar(255),
  "cover_key" varchar(255),
  "email" varchar(255),
  "contact_phone" varchar(50),
  "city" varchar(50),
  "district" varchar(50),
  "address_detail" varchar(255),
  "postal_code" varchar(10),
  "birthday" date,
  "gender" varchar(10),
  "prize" varchar(255),
  "rating" double precision DEFAULT 0,
  "current_certification_id" bigint,
  "seller_type" varchar(20),
  "onboarding_completed" boolean DEFAULT false,
  "certified_at" timestamptz,
  "created_at" timestamptz DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz,
  "status" varchar(20) NOT NULL DEFAULT 'active',
  "pii_cleared_at" timestamptz,
  "payuni_account_unlinked_at" timestamptz,
  "seo_title" varchar(70),
  "seo_description" varchar(180),
  "website_url" varchar(500),
  "is_processed_food" boolean NOT NULL DEFAULT false,
  "food_business_reg_no" varchar(50) NOT NULL DEFAULT '',
  "version" bigint NOT NULL DEFAULT 0,
  CONSTRAINT "chk_sellers_status" CHECK (status IN ('active', 'suspended', 'suspended_governance', 'closed')),
  CONSTRAINT "chk_sellers_version_nonneg" CHECK (version >= 0),
  CONSTRAINT "chk_sellers_active_is_current" CHECK (status <> 'active' OR is_current),
  CONSTRAINT "chk_sellers_city" CHECK (city IS NULL OR city = '' OR city IN (
    '基隆市', '台北市', '新北市', '桃園市', '新竹市', '新竹縣', '苗栗縣',
    '台中市', '彰化縣', '南投縣', '雲林縣', '嘉義市', '嘉義縣', '台南市',
    '高雄市', '屏東縣', '宜蘭縣', '花蓮縣', '台東縣', '澎湖縣', '金門縣', '連江縣'
  )),
  CONSTRAINT "chk_sellers_deleted_requires_closed" CHECK (status = 'closed' OR deleted_at IS NULL),
  CONSTRAINT "chk_sellers_food_reg" CHECK (
    (is_processed_food = false AND food_business_reg_no = '')
    OR
    (is_processed_food = true  AND food_business_reg_no <> '')
  )
);

CREATE UNIQUE INDEX "uq_sellers_account_current" ON "sellers" ("account_id") WHERE "is_current";

CREATE UNIQUE INDEX "uq_sellers_store_code" ON "sellers" ("store_code");

CREATE INDEX "idx_sellers_current_certification_id" ON "sellers" ("current_certification_id");

CREATE INDEX "idx_sellers_public_city" ON "sellers" ("city", "store_id")
  WHERE "is_current" AND "deleted_at" IS NULL AND "status" = 'active';

CREATE TABLE "payuni_registrations" (
  "id" bigint PRIMARY KEY,
  "account_id" bigint NOT NULL,
  "store_id" bigint NOT NULL UNIQUE,
  "application_id" bigint NOT NULL,
  "payuni_mer_id" varchar(50),
  "registration_status" varchar(20) DEFAULT 'unregistered',
  "payment_enabled" boolean DEFAULT false,
  "mer_status" varchar(10),
  "member_status" varchar(10),
  "credit_status" varchar(10),
  "applepay_status" varchar(10),
  "googlepay_status" varchar(10),
  "atm_status" varchar(10),
  "reason" text,
  "error_message" text,
  "error_code" varchar(20),
  "raw_request" jsonb,
  "raw_response" jsonb,
  "registered_at" timestamptz,
  "approved_at" timestamptz,
  "last_synced_at" timestamptz,
  "last_claimed_at" timestamptz,
  "updated_at" timestamptz DEFAULT CURRENT_TIMESTAMP,
  "contact_name" varchar(100),
  "contact_mobile" varchar(20),
  "contact_tel" varchar(20),
  "mer_email" varchar(255),
  "mer_addr" varchar(255),
  "mer_addr_post" varchar(10),
  "mer_tel" varchar(20),
  "merchant_profile_version" bigint NOT NULL DEFAULT 0
);

CREATE TABLE "payuni_status_history" (
  "id" bigint PRIMARY KEY,
  "account_id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "payuni_mer_id" varchar(50) NOT NULL,
  "source" varchar(20) NOT NULL,
  "mer_status" varchar(10),
  "member_status" varchar(10),
  "credit_status" varchar(10),
  "applepay_status" varchar(10),
  "googlepay_status" varchar(10),
  "atm_status" varchar(10),
  "reason" text,
  "raw_response" jsonb,
  "recorded_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX "idx_payuni_status_history_account_recorded"
  ON "payuni_status_history" ("account_id", "recorded_at" DESC);

CREATE TABLE "payuni_merchant_profile_updates" (
  "id" bigint PRIMARY KEY,
  "store_id" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "idempotency_key" varchar(128) NOT NULL,
  "payload_hash" varchar(64) NOT NULL,
  "status" varchar(20) NOT NULL,
  "changed_fields" jsonb NOT NULL,
  "reason" text,
  "trace_id" varchar(64),
  "provider_status" varchar(20),
  "provider_message" text,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "completed_at" timestamptz
);

CREATE UNIQUE INDEX "uq_payuni_merchant_profile_updates_store_key"
  ON "payuni_merchant_profile_updates" ("store_id", "idempotency_key");

CREATE TABLE "certification_applications" (
  "id" bigint PRIMARY KEY,
  "account_id" bigint NOT NULL,
  "application_no" varchar(50) UNIQUE NOT NULL,
  "status" varchar(20) NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','submitted','forwarded','forward_failed','rejected','cancelled')),
  "seller_type" varchar(20) NOT NULL
    CHECK (seller_type IN ('individual','business')),
  "real_name" varchar(100),
  "id_card_no" varchar(255),
  "birth_date" date,
  "id_card_issue_date" date,
  "id_card_issue_type" varchar(20),
  "id_card_issue_location" varchar(100),
  "english_name" varchar(100),
  "store_description" text,
  "id_card_front_asset_id" text,
  "id_card_back_asset_id" text,
  "bank_book_asset_id" text,
  "bank_name" varchar(100),
  "bank_code" varchar(8),
  "bank_account" varchar(255),
  "company_type" varchar(50),
  "company_name" varchar(255),
  "tax_id" varchar(20),
  "company_register_date" date,
  "company_city" varchar(50),
  "company_district" varchar(50),
  "company_address_detail" varchar(255),
  "company_email" varchar(100),
  "company_phone" varchar(50),
  "product_type" varchar(100),
  "is_processed_food" boolean DEFAULT false,
  "food_business_reg_no" varchar(50) NOT NULL DEFAULT '',
  "payuni_website_url" varchar(500),
  "document_asset_ids" jsonb,
  "current_step" int DEFAULT 0,
  "total_steps" int DEFAULT 8,
  "step_data" jsonb,
  "version" int NOT NULL DEFAULT 0,
  "submitted_at" timestamptz,
  "forwarded_at" timestamptz,
  "rejected_at" timestamptz,
  "cancel_reason" varchar(500),
  "created_at" timestamptz DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX "idx_cert_app_account_created" ON "certification_applications" ("account_id", "created_at" DESC);
CREATE INDEX "idx_cert_app_status" ON "certification_applications" ("status");

CREATE TABLE "seller_store_visitors" (
  "store_id"         bigint NOT NULL,
  "visitor_buyer_id" bigint NOT NULL,
  "visit_count"      int NOT NULL DEFAULT 1,
  "last_visited_at"  timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("store_id", "visitor_buyer_id")
);
CREATE INDEX idx_seller_store_visitors_seller
  ON seller_store_visitors (store_id, last_visited_at DESC);

CREATE TABLE "certification_reviews" (
  "id" bigserial PRIMARY KEY,
  "application_id" bigint NOT NULL,
  "reviewer_id" bigint,
  "reviewer_name" varchar(100),
  "action" varchar(20) NOT NULL,
  "result" varchar(20),
  "comment" text,
  "rejection_reason" varchar(255),
  "required_fields" jsonb,
  "created_at" timestamptz DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "seller_product_view_log" (
  "id"         bigint PRIMARY KEY,
  "seller_id"  bigint NOT NULL,
  "product_id" bigint NOT NULL,
  "source"     varchar(10) NOT NULL,
  "viewed_at"  timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_spvl_seller_viewed ON seller_product_view_log (seller_id, viewed_at DESC);

CREATE TABLE "seller_violation_reports" (
  "id"                     bigint PRIMARY KEY,
  "target_seller_id"       bigint NOT NULL,
  "target_product_id"      bigint,
  "target_post_id"         bigint,
  "target_order_no"        varchar(50),
  "reporter_account_id"    bigint NOT NULL,
  "category"               varchar(30) NOT NULL,
  "severity"               varchar(10) NOT NULL,
  "description"            text NOT NULL,
  "evidence_asset_ids"     jsonb,
  "status"                 varchar(20) NOT NULL DEFAULT 'pending',
  "investigating_at"       timestamptz,
  "resolved_at"            timestamptz,
  "admin_resolution_summary" text,
  "created_at"             timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"             timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_seller_violation_reports_dedup_calendar_day
  ON seller_violation_reports (reporter_account_id, target_seller_id, category, ((created_at AT TIME ZONE 'UTC')::date));
CREATE INDEX idx_svr_target_seller_status ON seller_violation_reports (target_seller_id, status);
CREATE INDEX idx_svr_reporter_created ON seller_violation_reports (reporter_account_id, created_at DESC);
CREATE INDEX idx_svr_status_severity_created ON seller_violation_reports (status, severity, created_at);

CREATE TABLE "food_safety_reports" (
  "id"                     bigint PRIMARY KEY,
  "target_seller_id"       bigint,
  "target_product_id"      bigint,
  "target_order_no"        varchar(50),
  "reporter_phone"         varchar(20) NOT NULL,
  "reporter_email"         varchar(255),
  "reporter_name"          varchar(100),
  "description"            text NOT NULL,
  "evidence_asset_ids"     jsonb,
  "status"                 varchar(20) NOT NULL DEFAULT 'pending',
  "acknowledged_at"        timestamptz,
  "acknowledged_by_admin_id" bigint,
  "investigating_at"       timestamptz,
  "resolved_at"            timestamptz,
  "admin_resolution_summary" text,
  "fda_reported_at"        timestamptz,
  "fda_report_channel"     varchar(30),
  "fda_reference_no"       text,
  "created_at"             timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"             timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_food_safety_reports_phone_seller_week
  ON food_safety_reports (reporter_phone, COALESCE(target_seller_id, 0), date_trunc('week', (created_at AT TIME ZONE 'UTC')));
CREATE INDEX idx_fsr_phone_seller_created ON food_safety_reports (reporter_phone, target_seller_id, created_at DESC);
CREATE INDEX idx_fsr_status_created ON food_safety_reports (status, created_at);
CREATE INDEX idx_fsr_acknowledged_at ON food_safety_reports (acknowledged_at) WHERE acknowledged_at IS NULL;

CREATE TABLE "seller_violations" (
  "id"                            bigint PRIMARY KEY,
  "seller_id"                     bigint NOT NULL,
  "source_seller_report_id"       bigint REFERENCES seller_violation_reports(id),
  "source_food_safety_report_id"  bigint REFERENCES food_safety_reports(id),
  "category"                      varchar(30) NOT NULL,
  "severity"                      varchar(10) NOT NULL,
  "penalty_type"                  varchar(30) NOT NULL,
  "penalty_amount_cents"          bigint,
  "description"                   text NOT NULL,
  "public_summary"                text,
  "effective_at"                  timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expires_at"                    timestamptz,
  "status"                        varchar(20) NOT NULL DEFAULT 'enforced',
  "revoked_at"                    timestamptz,
  "revoked_reason"                varchar(50),
  "appealed"                      boolean NOT NULL DEFAULT false,
  "appeal_reason"                 text,
  "appeal_evidence_asset_ids"     jsonb,
  "appeal_status"                 varchar(20),
  "appeal_submitted_at"           timestamptz,
  "appeal_decided_at"             timestamptz,
  "appeal_decision_reason"        text,
  "appeal_decided_by_admin_id"    bigint,
  "created_by_admin_id"           bigint NOT NULL,
  "created_at"                    timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"                    timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT chk_sv_source_mutex CHECK (
    ((source_seller_report_id IS NOT NULL)::int + (source_food_safety_report_id IS NOT NULL)::int) <= 1
  ),
  CONSTRAINT chk_sv_penalty_fields CHECK (
    (penalty_type = 'warning'
       AND penalty_amount_cents IS NULL
       AND expires_at IS NULL)
    OR (penalty_type = 'full_suspension'
       AND penalty_amount_cents IS NULL
       AND expires_at IS NOT NULL)
    OR (penalty_type = 'fine'
       AND penalty_amount_cents IS NOT NULL
       AND expires_at IS NULL)
    OR (penalty_type = 'partial_suspension'
       AND penalty_amount_cents IS NULL
       AND expires_at IS NOT NULL)
    OR (penalty_type IN ('permanent_ban', 'mandatory_training'))
  ),
  CONSTRAINT chk_sv_status CHECK (status IN ('enforced', 'revoked'))
);
CREATE INDEX idx_sv_seller_severity_effective ON seller_violations (seller_id, severity, effective_at DESC);
CREATE INDEX idx_sv_seller_created ON seller_violations (seller_id, created_at DESC);
CREATE INDEX idx_sv_seller_severity_revoked ON seller_violations (seller_id, severity, revoked_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_sv_expires_at_full_suspension ON seller_violations (expires_at)
  WHERE penalty_type = 'full_suspension' AND revoked_at IS NULL;
CREATE INDEX idx_sv_appeal_pending ON seller_violations (appeal_submitted_at)
  WHERE appeal_status = 'pending';

CREATE TABLE "seller_notification_log" (
  "id"                  bigint PRIMARY KEY,
  "source_report_id"    bigint,
  "source_report_table" varchar(30),
  "template_key"        varchar(50) NOT NULL,
  "seller_id"           bigint NOT NULL,
  "sent_by_admin_id"    bigint,
  "sent_at"             timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "message_payload"     jsonb NOT NULL
);
CREATE INDEX idx_snl_seller_sent ON seller_notification_log (seller_id, sent_at DESC);
CREATE INDEX idx_snl_source ON seller_notification_log (source_report_table, source_report_id) WHERE source_report_id IS NOT NULL;

CREATE TABLE "seller_deletion_audit" (
  "id"                 bigint PRIMARY KEY,
  "account_id"         bigint NOT NULL,
  "operator_id"        bigint NOT NULL,
  "active_order_count" int    NOT NULL,
  "reason"             varchar(255),
  "created_at"         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX "idx_sda_account" ON "seller_deletion_audit" ("account_id", "created_at" DESC);

CREATE TABLE seller_deleted_outbox (
  id                  bigint PRIMARY KEY,
  account_id          bigint NOT NULL,
  event_type          text NOT NULL DEFAULT 'seller.deleted',
  payload             jsonb,
  status              varchar(20) NOT NULL DEFAULT 'pending',
  attempt_count       int NOT NULL DEFAULT 0,
  next_attempt_at     timestamptz NOT NULL DEFAULT NOW(),
  last_error          text,
  created_at          timestamptz NOT NULL DEFAULT NOW(),
  completed_at        timestamptz,
  event_published_at  timestamptz,
  role_removed_at     timestamptz
);
CREATE INDEX idx_seller_deleted_outbox_pending
  ON seller_deleted_outbox (status, next_attempt_at) WHERE status = 'pending';

CREATE TABLE "merchant_verification_outbox" (
  "id" bigint PRIMARY KEY,
  "account_id" bigint NOT NULL,
  "event_type" varchar(30) NOT NULL,
  "reason" text,
  "payload" jsonb NOT NULL,
  "status" varchar(20) NOT NULL DEFAULT 'pending',
  "attempt_count" int NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL,
  "last_error" text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz
);
CREATE INDEX "merchant_verification_outbox_pending_idx"
  ON "merchant_verification_outbox" ("status", "next_attempt_at")
  WHERE "status" = 'pending';

CREATE TABLE "payuni_merchant_profile_outbox" (
  "id"              bigint PRIMARY KEY,
  "store_id"        bigint NOT NULL,
  "account_id"      bigint NOT NULL,
  "payload"         jsonb NOT NULL,
  "status"          varchar(20) NOT NULL DEFAULT 'pending',
  "attempt_count"   int NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL,
  "last_error"      text,
  "created_at"      timestamptz NOT NULL DEFAULT now(),
  "completed_at"    timestamptz
);
CREATE INDEX "payuni_merchant_profile_outbox_pending_idx"
  ON "payuni_merchant_profile_outbox" ("status", "next_attempt_at")
  WHERE "status" = 'pending';

CREATE TABLE "seller_suspensions" (
  "id"                     bigint PRIMARY KEY,
  "seller_id"              bigint NOT NULL,
  "reason_code"            varchar(30) NOT NULL,
  "reason_detail"          text,
  "status"                 varchar(20) NOT NULL DEFAULT 'active',
  "suspended_by_admin_id"  bigint NOT NULL,
  "suspended_at"           timestamptz NOT NULL DEFAULT now(),
  "lifted_by_admin_id"     bigint,
  "lifted_at"              timestamptz,
  "created_at"             timestamptz NOT NULL DEFAULT now(),
  "updated_at"             timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX "uq_seller_suspensions_active"
  ON "seller_suspensions" ("seller_id") WHERE "status" = 'active';

CREATE TABLE "seller_membership_plans" (
  "id"                 bigint NOT NULL,
  "plan_code"          varchar(30) NOT NULL,
  "display_name"       varchar(100) NOT NULL,
  "tier_rank"          int NOT NULL DEFAULT 0,
  "monthly_price_twd"  int NOT NULL DEFAULT 0,
  "monthly_order_cap"  int NOT NULL,
  "is_purchasable"     boolean NOT NULL DEFAULT true,
  "created_at"         timestamptz NOT NULL DEFAULT now(),
  "updated_at"         timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_seller_membership_plans_price_nonneg" CHECK (monthly_price_twd >= 0),
  CONSTRAINT "chk_seller_membership_plans_cap_positive" CHECK (monthly_order_cap > 0)
);
CREATE UNIQUE INDEX "uq_seller_membership_plans_plan_code" ON "seller_membership_plans" ("plan_code");

CREATE TABLE "seller_membership_subscriptions" (
  "id"           bigint NOT NULL,
  "store_id"     bigint NOT NULL,
  "plan_code"    varchar(30) NOT NULL REFERENCES "seller_membership_plans" ("plan_code"),
  "status"       varchar(20) NOT NULL DEFAULT 'active',
  "effective_at" timestamptz NOT NULL,
  "source"       varchar(20) NOT NULL,
  "granted_by"   bigint,
  "reason"       text NOT NULL DEFAULT '',
  "request_id"   varchar(100) NOT NULL DEFAULT '',
  "created_at"   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_seller_membership_subscriptions_source" CHECK (source IN ('admin', 'system_default')),
  CONSTRAINT "chk_seller_membership_subscriptions_status" CHECK (status IN ('active', 'superseded'))
);

CREATE UNIQUE INDEX "uq_seller_membership_subscriptions_active"
  ON "seller_membership_subscriptions" ("store_id")
  WHERE "status" = 'active';

CREATE UNIQUE INDEX "uq_seller_membership_subscriptions_request_id"
  ON "seller_membership_subscriptions" ("request_id")
  WHERE "request_id" != '';

CREATE INDEX "idx_seller_membership_subscriptions_store_created"
  ON "seller_membership_subscriptions" ("store_id", "created_at" DESC);

INSERT INTO "seller_membership_plans" ("id", "plan_code", "display_name", "tier_rank", "monthly_price_twd", "monthly_order_cap", "is_purchasable") VALUES
  (2001, 'free', '免費方案', 0, 0, 50,   true),
  (2002, 'plus', '進階方案', 1, 0, 200,  true),
  (2003, 'pro',  '旗艦方案', 2, 0, 1000, true);

CREATE TABLE "seller_prospects" (
  "id"                 bigint PRIMARY KEY,
  "display_name"       varchar(255) NOT NULL,
  "vendor_type"        varchar(20)  NOT NULL,
  "status"             varchar(20)  NOT NULL DEFAULT 'ACTIVE',
  "outreach_status"    varchar(30)  NOT NULL DEFAULT 'NOT_CONTACTED',

  "tax_id"             varchar(8),
  "converted_store_id" bigint,
  "converted_at"       timestamptz,

  "match_email"        varchar(255) NOT NULL DEFAULT '',
  "match_phone"        varchar(50)  NOT NULL DEFAULT '',
  "match_domain"       varchar(255) NOT NULL DEFAULT '',
  "match_social"       varchar(255) NOT NULL DEFAULT '',

  "public_email"       varchar(255) NOT NULL DEFAULT '',
  "public_phone"       varchar(50)  NOT NULL DEFAULT '',
  "public_website"     varchar(500) NOT NULL DEFAULT '',
  "public_social"      varchar(500) NOT NULL DEFAULT '',

  "category"           varchar(100) NOT NULL DEFAULT '',
  "region"             varchar(100) NOT NULL DEFAULT '',
  "product_summary"    text NOT NULL DEFAULT '',
  "positioning"        text NOT NULL DEFAULT '',
  "research_summary"   text NOT NULL DEFAULT '',
  "priority_letter"    varchar(4)   NOT NULL DEFAULT '',
  "priority_stars"     int          NOT NULL DEFAULT 0,

  -- 品牌圖片。key 是 media-service 轉正後的 objectKey，顯示 URL 一律由 Gateway 組；
  -- source_url 是「這張圖是從哪個公開頁面找到的」的稽核欄位，後端不會去下載它。
  "logo_key"           varchar(500)  NOT NULL DEFAULT '',
  "cover_key"          varchar(500)  NOT NULL DEFAULT '',
  "logo_source_url"    varchar(2048) NOT NULL DEFAULT '',
  "cover_source_url"   varchar(2048) NOT NULL DEFAULT '',
  -- 圖片專屬的樂觀鎖序號，與文字主檔的編輯分開計。Admin 的一般文字表單刻意不送
  -- 媒體欄位，兩邊共用一個版本會讓換圖把並行的文字編輯撞成 409。
  "media_version"      bigint        NOT NULL DEFAULT 0,

  "created_by_account_id" bigint,
  "updated_by_account_id" bigint,
  "created_at"         timestamptz NOT NULL DEFAULT now(),
  "updated_at"         timestamptz NOT NULL DEFAULT now(),
  "merged_into_prospect_id" bigint REFERENCES "seller_prospects"("id"),

  CONSTRAINT "chk_sp_vendor_type" CHECK (vendor_type IN ('individual','business')),
  CONSTRAINT "chk_sp_status"      CHECK (status IN ('ACTIVE','ARCHIVED','CONVERTED')),
  CONSTRAINT "chk_sp_outreach"    CHECK (outreach_status IN (
      'NOT_CONTACTED','INVITED','REPLIED','DECLINED','DO_NOT_CONTACT','CONVERTED')),
  CONSTRAINT "chk_sp_tax_id"      CHECK (tax_id IS NULL OR tax_id ~ '^[0-9]{8}$'),
  CONSTRAINT "chk_sp_converted"   CHECK (status <> 'CONVERTED' OR converted_store_id IS NOT NULL),
  CONSTRAINT "chk_sp_merged_into"  CHECK (
    merged_into_prospect_id IS NULL
    OR (status = 'ARCHIVED' AND merged_into_prospect_id <> id)
  )
);

CREATE UNIQUE INDEX "uq_seller_prospects_tax_id"
  ON "seller_prospects" ("tax_id") WHERE "tax_id" IS NOT NULL AND "status" <> 'ARCHIVED';

CREATE UNIQUE INDEX "uq_seller_prospects_converted_store"
  ON "seller_prospects" ("converted_store_id") WHERE "converted_store_id" IS NOT NULL;

CREATE INDEX "idx_seller_prospects_active"
  ON "seller_prospects" ("status") WHERE "status" = 'ACTIVE';

CREATE INDEX "idx_seller_prospects_match_email"  ON "seller_prospects" ("match_email")  WHERE "match_email"  <> '';
CREATE INDEX "idx_seller_prospects_match_phone"  ON "seller_prospects" ("match_phone")  WHERE "match_phone"  <> '';
CREATE INDEX "idx_seller_prospects_match_domain" ON "seller_prospects" ("match_domain") WHERE "match_domain" <> '';
CREATE INDEX "idx_seller_prospects_match_social" ON "seller_prospects" ("match_social") WHERE "match_social" <> '';

CREATE TABLE "seller_prospect_categories" (
  "prospect_id" bigint      NOT NULL REFERENCES "seller_prospects"("id") ON DELETE CASCADE,
  "category_id" varchar(64) NOT NULL,
  "created_at"  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("prospect_id", "category_id"),
  CONSTRAINT "chk_spc_category_id" CHECK ("category_id" <> '')
);

CREATE INDEX "idx_spc_category" ON "seller_prospect_categories" ("category_id");

CREATE INDEX "idx_seller_prospects_merged_into" ON "seller_prospects" ("merged_into_prospect_id")
  WHERE merged_into_prospect_id IS NOT NULL;

CREATE TABLE "seller_prospect_invites" (
  "id"          bigint PRIMARY KEY,
  "prospect_id" bigint NOT NULL REFERENCES "seller_prospects" ("id"),
  "token"       varchar(64) NOT NULL,
  "sent_at"     timestamptz,
  "expires_at"  timestamptz NOT NULL,
  "clicked_at"  timestamptz,
  "consumed_at" timestamptz,
  "revoked_at"  timestamptz,
  "created_at"  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX "uq_spi_token" ON "seller_prospect_invites" ("token");
CREATE INDEX "idx_spi_prospect" ON "seller_prospect_invites" ("prospect_id", "created_at" DESC);

CREATE TABLE "vendor_portal_identities" (
  "id"                       bigint PRIMARY KEY,
  "prospect_id"              bigint NOT NULL REFERENCES "seller_prospects" ("id"),
  "invited_email"            varchar(255) NOT NULL,
  "status"                   varchar(20) NOT NULL DEFAULT 'ACTIVE',
  "created_at"               timestamptz NOT NULL DEFAULT now(),
  "updated_at"               timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "chk_vpi_status" CHECK (status IN ('ACTIVE','REVOKED'))
);

CREATE UNIQUE INDEX "uq_vpi_prospect" ON "vendor_portal_identities" ("prospect_id");

CREATE TABLE "vendor_portal_invites" (
  "id"                  bigint PRIMARY KEY,
  "identity_id"         bigint NOT NULL REFERENCES "vendor_portal_identities" ("id"),
  "rfq_recipient_id"    bigint      NOT NULL,
  "token_hash"          varchar(64) NOT NULL,
  "secret_delivered_at" timestamptz,
  "expires_at"          timestamptz NOT NULL,
  "consumed_at"         timestamptz,
  "revoked_at"          timestamptz,
  "created_at"          timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX "uq_vpinv_token_hash" ON "vendor_portal_invites" ("token_hash");
CREATE UNIQUE INDEX "uq_vpinv_rfq_recipient" ON "vendor_portal_invites" ("rfq_recipient_id");

CREATE TABLE "vendor_portal_sessions" (
  "id"                  bigint PRIMARY KEY,
  "session_hash"        varchar(64) NOT NULL,
  "identity_id"         bigint NOT NULL REFERENCES "vendor_portal_identities" ("id"),
  "invite_id"           bigint NOT NULL REFERENCES "vendor_portal_invites" ("id"),
  "assurance"           varchar(20) NOT NULL,
  "idle_expires_at"     timestamptz NOT NULL,
  "absolute_expires_at" timestamptz NOT NULL,
  "revoked_at"          timestamptz,
  "last_seen_at"        timestamptz NOT NULL DEFAULT now(),
  "created_at"          timestamptz NOT NULL DEFAULT now(),
  -- INVITE_ONLY ＝ 只憑邀請信那條一次性連結（bearer），GOOGLE_VERIFIED ＝ 另外以
  -- Google ID Token 證明當下持有 vendor_portal_identities.invited_email 那個信箱。
  --
  -- 🔴 兩級存在的理由是「可覆寫」與「不可撤回」的差別：報價**草稿**可以覆寫，用
  -- INVITE_ONLY 低摩擦；**正式送出**產生的 sellerResponseVersion 是 immutable，
  -- 買家接受 exact version 即商務 award ⇒ 不可撤回的商務承諾不能只靠一條七天有效的
  -- 連結（拿到信的任何人都能代該業者承諾價格）。
  --
  -- ⚠️ 本次只放寬值域，**尚無寫入者**（expand-only）：升級路徑與要求它的寫入端在
  -- 下一刀。先放寬是為了讓舊 pod 與新 pod 都寫得進去，避免相容窗口。
  CONSTRAINT "chk_vps_assurance" CHECK (assurance IN ('INVITE_ONLY', 'GOOGLE_VERIFIED'))
);

CREATE UNIQUE INDEX "uq_vps_session_hash" ON "vendor_portal_sessions" ("session_hash");
CREATE INDEX "idx_vps_identity_active"
  ON "vendor_portal_sessions" ("identity_id") WHERE revoked_at IS NULL;
