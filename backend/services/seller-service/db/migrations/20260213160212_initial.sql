-- Create "certification_applications" table
CREATE TABLE "certification_applications" (
  "id" bigserial NOT NULL,
  "account_id" bigint NOT NULL,
  "application_no" character varying(50) NOT NULL,
  "status" character varying(20) NOT NULL DEFAULT 'draft',
  "seller_type" character varying(20) NOT NULL,
  "real_name" character varying(100) NULL,
  "id_card_no" character varying(20) NULL,
  "birth_date" date NULL,
  "id_card_issue_date" date NULL,
  "id_card_issue_type" character varying(20) NULL,
  "id_card_issue_location" character varying(100) NULL,
  "english_name" character varying(100) NULL,
  "id_card_front_asset_id" text NULL,
  "id_card_back_asset_id" text NULL,
  "bank_book_asset_id" text NULL,
  "bank_name" character varying(100) NULL,
  "bank_account" character varying(50) NULL,
  "company_type" character varying(50) NULL,
  "company_name" character varying(255) NULL,
  "tax_id" character varying(20) NULL,
  "company_register_date" date NULL,
  "company_city" character varying(50) NULL,
  "company_district" character varying(50) NULL,
  "company_address_detail" character varying(255) NULL,
  "company_email" character varying(100) NULL,
  "company_phone" character varying(50) NULL,
  "product_type" character varying(100) NULL,
  "is_processed_food" boolean NULL DEFAULT false,
  "document_asset_ids" jsonb NULL,
  "current_step" integer NULL DEFAULT 0,
  "total_steps" integer NULL DEFAULT 8,
  "step_data" jsonb NULL,
  "submitted_at" timestamptz NULL,
  "approved_at" timestamptz NULL,
  "rejected_at" timestamptz NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "certification_applications_application_no_key" UNIQUE ("application_no")
);
-- Create "certification_reviews" table
CREATE TABLE "certification_reviews" (
  "id" bigserial NOT NULL,
  "application_id" bigint NOT NULL,
  "reviewer_id" bigint NULL,
  "reviewer_name" character varying(100) NULL,
  "action" character varying(20) NOT NULL,
  "result" character varying(20) NULL,
  "comment" text NULL,
  "rejection_reason" character varying(255) NULL,
  "required_fields" jsonb NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create "crm_customer_tags" table
CREATE TABLE "crm_customer_tags" (
  "id" bigserial NOT NULL,
  "account_id" bigint NOT NULL,
  "customer_account_id" bigint NOT NULL,
  "tag_id" bigint NOT NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create "crm_customers" table
CREATE TABLE "crm_customers" (
  "id" bigserial NOT NULL,
  "account_id" bigint NOT NULL,
  "customer_account_id" bigint NOT NULL,
  "status" integer NULL DEFAULT 0,
  "status_reason" character varying(255) NULL,
  "total_spent" bigint NULL DEFAULT 0,
  "order_count" integer NULL DEFAULT 0,
  "first_order_at" timestamptz NULL,
  "last_order_at" timestamptz NULL,
  "remark" text NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create "crm_tags" table
CREATE TABLE "crm_tags" (
  "id" bigserial NOT NULL,
  "account_id" bigint NOT NULL,
  "name" character varying(64) NOT NULL,
  "color" character varying(20) NULL,
  "description" character varying(255) NULL,
  "customer_count" integer NULL DEFAULT 0,
  "sort_order" integer NULL DEFAULT 0,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create "marketing_tasks" table
CREATE TABLE "marketing_tasks" (
  "id" bigserial NOT NULL,
  "task_uuid" character varying(64) NOT NULL,
  "account_id" bigint NOT NULL,
  "type" character varying(32) NOT NULL,
  "status" integer NULL DEFAULT 0,
  "params" jsonb NULL,
  "target_count" integer NULL DEFAULT 0,
  "success_count" integer NULL DEFAULT 0,
  "failed_count" integer NULL DEFAULT 0,
  "error_message" text NULL,
  "started_at" timestamptz NULL,
  "completed_at" timestamptz NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "marketing_tasks_task_uuid_key" UNIQUE ("task_uuid")
);
-- Create "payuni_registrations" table
CREATE TABLE "payuni_registrations" (
  "id" bigserial NOT NULL,
  "account_id" bigint NOT NULL,
  "application_id" bigint NOT NULL,
  "payuni_mer_id" character varying(50) NULL,
  "registration_status" character varying(20) NULL DEFAULT 'unregistered',
  "payment_enabled" boolean NULL DEFAULT false,
  "mer_status" character varying(10) NULL,
  "member_status" character varying(10) NULL,
  "credit_status" character varying(10) NULL,
  "applepay_status" character varying(10) NULL,
  "googlepay_status" character varying(10) NULL,
  "atm_status" character varying(10) NULL,
  "error_message" text NULL,
  "raw_request" jsonb NULL,
  "raw_response" jsonb NULL,
  "registered_at" timestamptz NULL,
  "approved_at" timestamptz NULL,
  "last_synced_at" timestamptz NULL,
  "updated_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "payuni_registrations_application_id_key" UNIQUE ("application_id")
);
-- Create "sellers" table
CREATE TABLE "sellers" (
  "id" bigserial NOT NULL,
  "account_id" bigint NOT NULL,
  "store_name" character varying(255) NOT NULL,
  "description" text NULL,
  "logo" character varying(255) NULL,
  "cover_url" character varying(255) NULL,
  "contact_phone" character varying(50) NULL,
  "address" character varying(255) NULL,
  "prize" character varying(255) NULL,
  "status" integer NULL DEFAULT 0,
  "rating" double precision NULL DEFAULT 0,
  "current_certification_id" bigint NULL,
  "seller_type" character varying(20) NULL,
  "payuni_status" integer NULL DEFAULT 0,
  "approval_status" character varying(20) NULL DEFAULT 'pending',
  "certified_at" timestamptz NULL,
  "payuni_mer_id" character varying(50) NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "sellers_account_id_key" UNIQUE ("account_id")
);
