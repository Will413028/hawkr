-- Add new schema named "seller"
CREATE SCHEMA "seller";
-- Create "stores" table
CREATE TABLE "seller"."stores" (
  "store_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "store_code" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "status" text NOT NULL DEFAULT 'ACTIVE',
  "suspension" text NOT NULL DEFAULT 'NONE',
  "version" bigint NOT NULL DEFAULT 1,
  "store_name" text NOT NULL,
  "description" text NULL,
  "logo_key" text NULL,
  "logo_asset_id" text NULL,
  "cover_key" text NULL,
  "cover_asset_id" text NULL,
  "contact_phone" text NULL,
  "town_code" text NULL,
  "address_detail" text NULL,
  "prize" text NULL,
  "website_url" text NULL,
  "seo_title" text NULL,
  "seo_description" text NULL,
  "is_processed_food" boolean NOT NULL DEFAULT false,
  "food_business_reg_no" text NULL,
  "onboarding_completed" boolean NOT NULL DEFAULT false,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "deleted_at" timestamptz NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "stores_address_detail_check" CHECK ((address_detail IS NULL) OR ((town_code IS NOT NULL) AND ((char_length(address_detail) >= 1) AND (char_length(address_detail) <= 255)))),
  CONSTRAINT "stores_closed_images_check" CHECK ((status <> 'CLOSED'::text) OR ((logo_key IS NULL) AND (cover_key IS NULL))),
  CONSTRAINT "stores_contact_phone_check" CHECK (contact_phone ~ '^0[0-9]{8,9}$'::text),
  CONSTRAINT "stores_deleted_check" CHECK ((status = 'CLOSED'::text) = (deleted_at IS NOT NULL)),
  CONSTRAINT "stores_description_check" CHECK ((char_length(description) >= 1) AND (char_length(description) <= 2000)),
  CONSTRAINT "stores_food_registration_check" CHECK ((is_processed_food AND (food_business_reg_no IS NOT NULL) AND ((char_length(food_business_reg_no) >= 1) AND (char_length(food_business_reg_no) <= 50))) OR ((NOT is_processed_food) AND (food_business_reg_no IS NULL))),
  CONSTRAINT "stores_image_assets_check" CHECK (((logo_key IS NULL) = (logo_asset_id IS NULL)) AND ((cover_key IS NULL) = (cover_asset_id IS NULL))),
  CONSTRAINT "stores_images_check" CHECK ((logo_key <> ''::text) AND (cover_key <> ''::text) AND (logo_asset_id <> ''::text) AND (cover_asset_id <> ''::text)),
  CONSTRAINT "stores_prize_check" CHECK ((char_length(prize) >= 1) AND (char_length(prize) <= 255)),
  CONSTRAINT "stores_seo_description_check" CHECK ((char_length(seo_description) >= 1) AND (char_length(seo_description) <= 180)),
  CONSTRAINT "stores_seo_title_check" CHECK ((char_length(seo_title) >= 1) AND (char_length(seo_title) <= 70)),
  CONSTRAINT "stores_status_check" CHECK (status = ANY (ARRAY['ACTIVE'::text, 'PENDING_DELETION'::text, 'CLOSED'::text])),
  CONSTRAINT "stores_store_code_check" CHECK ((store_code >= 1) AND (store_code <= '9999999999'::bigint)),
  CONSTRAINT "stores_store_name_check" CHECK ((char_length(store_name) >= 1) AND (char_length(store_name) <= 100) AND (store_name = btrim(store_name))),
  CONSTRAINT "stores_suspension_check" CHECK (suspension = ANY (ARRAY['NONE'::text, 'ADMIN'::text])),
  CONSTRAINT "stores_town_code_check" CHECK (town_code ~ '^[0-9]{8}$'::text),
  CONSTRAINT "stores_version_check" CHECK (version >= 1),
  CONSTRAINT "stores_website_url_check" CHECK ((website_url ~ '^https?://[^/?#]+'::text) AND (char_length(website_url) <= 500))
);
-- Create index "stores_account" to table: "stores"
CREATE INDEX "stores_account" ON "seller"."stores" ("account_id");
-- Create index "stores_current_account" to table: "stores"
CREATE UNIQUE INDEX "stores_current_account" ON "seller"."stores" ("account_id") WHERE (status <> 'CLOSED'::text);
-- Create index "stores_store_code" to table: "stores"
CREATE UNIQUE INDEX "stores_store_code" ON "seller"."stores" ("store_code");
-- Create "suspension_periods" table
CREATE TABLE "seller"."suspension_periods" (
  "period_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "store_id" bigint NOT NULL,
  "started_at" timestamptz NOT NULL,
  "ended_at" timestamptz NULL,
  PRIMARY KEY ("period_id"),
  CONSTRAINT "suspension_periods_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "suspension_periods_ended_check" CHECK ((ended_at IS NULL) OR (ended_at >= started_at))
);
-- Create index "suspension_periods_open" to table: "suspension_periods"
CREATE UNIQUE INDEX "suspension_periods_open" ON "seller"."suspension_periods" ("store_id") WHERE (ended_at IS NULL);
-- Create "suspensions" table
CREATE TABLE "seller"."suspensions" (
  "suspension_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "store_id" bigint NOT NULL,
  "reason" text NOT NULL,
  "detail" text NULL,
  "suspended_by_admin_id" bigint NOT NULL,
  "suspended_at" timestamptz NOT NULL,
  "lifted_by_admin_id" bigint NULL,
  "lifted_at" timestamptz NULL,
  PRIMARY KEY ("suspension_id"),
  CONSTRAINT "suspensions_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "suspensions_admin_check" CHECK ((suspended_by_admin_id > 0) AND ((lifted_by_admin_id IS NULL) OR (lifted_by_admin_id > 0))),
  CONSTRAINT "suspensions_detail_check" CHECK ((char_length(detail) >= 1) AND (char_length(detail) <= 1000)),
  CONSTRAINT "suspensions_lifted_check" CHECK (((lifted_at IS NULL) = (lifted_by_admin_id IS NULL)) AND ((lifted_at IS NULL) OR (lifted_at >= suspended_at))),
  CONSTRAINT "suspensions_other_detail_check" CHECK ((reason <> 'OTHER'::text) OR (detail IS NOT NULL)),
  CONSTRAINT "suspensions_reason_check" CHECK (reason = ANY (ARRAY['POLICY_VIOLATION'::text, 'REPEATED_WARNING_IGNORED'::text, 'OTHER'::text]))
);
-- Create index "suspensions_active" to table: "suspensions"
CREATE UNIQUE INDEX "suspensions_active" ON "seller"."suspensions" ("store_id") WHERE (lifted_at IS NULL);
-- Create index "suspensions_store" to table: "suspensions"
CREATE INDEX "suspensions_store" ON "seller"."suspensions" ("store_id", "suspended_at" DESC, "suspension_id" DESC);
