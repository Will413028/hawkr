-- CRM Service: Initial Schema
-- Create: crm_customers, crm_tags, crm_customer_tags, marketing_tasks

CREATE TABLE "crm_customers" (
  "id"                  bigserial PRIMARY KEY,
  "account_id"          bigint NOT NULL,
  "customer_account_id" bigint NOT NULL,
  "status"              smallint NOT NULL DEFAULT 0,
  "status_reason"       text NOT NULL DEFAULT '',
  "total_spent"         bigint NOT NULL DEFAULT 0,
  "order_count"         int NOT NULL DEFAULT 0,
  "first_order_at"      timestamptz,
  "last_order_at"       timestamptz,
  "remark"              text NOT NULL DEFAULT '',
  "created_at"          timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"          timestamptz NOT NULL DEFAULT NOW(),
  UNIQUE ("account_id", "customer_account_id")
);

CREATE INDEX "idx_crm_customers_account_id" ON "crm_customers" ("account_id");
CREATE INDEX "idx_crm_customers_status" ON "crm_customers" ("account_id", "status");

CREATE TABLE "crm_tags" (
  "id"             bigserial PRIMARY KEY,
  "account_id"     bigint NOT NULL,
  "name"           varchar(50) NOT NULL,
  "color"          varchar(20) NOT NULL DEFAULT '#000000',
  "description"    text NOT NULL DEFAULT '',
  "customer_count" int NOT NULL DEFAULT 0,
  "sort_order"     int NOT NULL DEFAULT 0,
  "created_at"     timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"     timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX "idx_crm_tags_account_id" ON "crm_tags" ("account_id");

CREATE TABLE "crm_customer_tags" (
  "id"                  bigserial PRIMARY KEY,
  "account_id"          bigint NOT NULL,
  "customer_account_id" bigint NOT NULL,
  "tag_id"              bigint NOT NULL REFERENCES "crm_tags" ("id") ON DELETE CASCADE,
  "created_at"          timestamptz NOT NULL DEFAULT NOW(),
  UNIQUE ("account_id", "customer_account_id", "tag_id")
);

CREATE INDEX "idx_crm_customer_tags_customer" ON "crm_customer_tags" ("account_id", "customer_account_id");
CREATE INDEX "idx_crm_customer_tags_tag" ON "crm_customer_tags" ("tag_id");

CREATE TABLE "marketing_tasks" (
  "id"            bigserial PRIMARY KEY,
  "task_uuid"     uuid NOT NULL DEFAULT gen_random_uuid(),
  "account_id"    bigint NOT NULL,
  "type"          varchar(50) NOT NULL,
  "status"        smallint NOT NULL DEFAULT 0,
  "params_json"   text NOT NULL DEFAULT '{}',
  "target_count"  int NOT NULL DEFAULT 0,
  "success_count" int NOT NULL DEFAULT 0,
  "failed_count"  int NOT NULL DEFAULT 0,
  "error_message" text NOT NULL DEFAULT '',
  "created_at"    timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"    timestamptz NOT NULL DEFAULT NOW(),
  "completed_at"  timestamptz
);

CREATE INDEX "idx_marketing_tasks_account_id" ON "marketing_tasks" ("account_id");
CREATE UNIQUE INDEX "idx_marketing_tasks_uuid" ON "marketing_tasks" ("task_uuid");
