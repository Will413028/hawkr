-- Create "product_views_daily" table
CREATE TABLE "seller"."product_views_daily" (
  "store_id" bigint NOT NULL,
  "day" date NOT NULL,
  "views" bigint NOT NULL,
  PRIMARY KEY ("store_id", "day"),
  CONSTRAINT "product_views_daily_views_check" CHECK (views >= 1)
);
-- Create "prospects" table
CREATE TABLE "seller"."prospects" (
  "prospect_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "version" bigint NOT NULL DEFAULT 1,
  "display_name" text NOT NULL,
  "vendor_type" text NOT NULL,
  "status" text NOT NULL DEFAULT 'ACTIVE',
  "outreach_status" text NOT NULL DEFAULT 'NOT_CONTACTED',
  "live" boolean NULL GENERATED ALWAYS AS (status <> 'ARCHIVED'::text) STORED,
  "tax_id" text NULL,
  "converted_store_id" bigint NULL,
  "converted_at" timestamptz NULL,
  "merged_into" bigint NULL,
  "public_email" text NULL,
  "public_phone" text NULL,
  "public_website" text NULL,
  "public_social" text NULL,
  "category" text NULL,
  "region" text NULL,
  "product_summary" text NULL,
  "positioning" text NULL,
  "research_summary" text NULL,
  "priority_letter" text NULL,
  "priority_stars" integer NOT NULL DEFAULT 0,
  "logo_key" text NULL,
  "logo_asset_id" text NULL,
  "logo_source_url" text NULL,
  "cover_key" text NULL,
  "cover_asset_id" text NULL,
  "cover_source_url" text NULL,
  "created_by_account_id" bigint NULL,
  "updated_by_account_id" bigint NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("prospect_id"),
  CONSTRAINT "prospects_live_key" UNIQUE ("prospect_id", "live"),
  CONSTRAINT "prospects_converted_store_id_fkey" FOREIGN KEY ("converted_store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "prospects_merged_into_fkey" FOREIGN KEY ("merged_into") REFERENCES "seller"."prospects" ("prospect_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "prospects_converted_check" CHECK (((status = 'CONVERTED'::text) = (converted_store_id IS NOT NULL)) AND ((converted_store_id IS NULL) = (converted_at IS NULL)) AND ((status = 'CONVERTED'::text) = (outreach_status = 'CONVERTED'::text))),
  CONSTRAINT "prospects_display_name_check" CHECK ((char_length(display_name) >= 1) AND (char_length(display_name) <= 255) AND (display_name = btrim(display_name))),
  CONSTRAINT "prospects_images_check" CHECK (((logo_key IS NULL) = (logo_asset_id IS NULL)) AND ((cover_key IS NULL) = (cover_asset_id IS NULL)) AND ((status <> 'ARCHIVED'::text) OR ((logo_key IS NULL) AND (cover_key IS NULL)))),
  CONSTRAINT "prospects_merged_check" CHECK ((merged_into IS NULL) OR ((status = 'ARCHIVED'::text) AND (merged_into <> prospect_id))),
  CONSTRAINT "prospects_outreach_check" CHECK (outreach_status = ANY (ARRAY['NOT_CONTACTED'::text, 'INVITED'::text, 'REPLIED'::text, 'DECLINED'::text, 'DO_NOT_CONTACT'::text, 'CONVERTED'::text])),
  CONSTRAINT "prospects_priority_check" CHECK ((priority_stars >= 0) AND (char_length(priority_letter) >= 1) AND (char_length(priority_letter) <= 4)),
  CONSTRAINT "prospects_status_check" CHECK (status = ANY (ARRAY['ACTIVE'::text, 'ARCHIVED'::text, 'CONVERTED'::text])),
  CONSTRAINT "prospects_tax_id_check" CHECK (tax_id ~ '^[0-9]{8}$'::text),
  CONSTRAINT "prospects_text_check" CHECK ((char_length(public_email) >= 1) AND (char_length(public_email) <= 255) AND (char_length(public_phone) >= 1) AND (char_length(public_phone) <= 50) AND (char_length(public_website) >= 1) AND (char_length(public_website) <= 500) AND (char_length(public_social) >= 1) AND (char_length(public_social) <= 500) AND (char_length(category) >= 1) AND (char_length(category) <= 100) AND (char_length(region) >= 1) AND (char_length(region) <= 100) AND (char_length(product_summary) >= 1) AND (char_length(positioning) >= 1) AND (char_length(research_summary) >= 1)),
  CONSTRAINT "prospects_vendor_type_check" CHECK (vendor_type = ANY (ARRAY['INDIVIDUAL'::text, 'BUSINESS'::text])),
  CONSTRAINT "prospects_version_check" CHECK (version >= 1)
);
-- Create index "prospects_active_name" to table: "prospects"
CREATE INDEX "prospects_active_name" ON "seller"."prospects" ((lower(btrim(display_name))), "prospect_id") WHERE (status = 'ACTIVE'::text);
-- Create index "prospects_converted_store" to table: "prospects"
CREATE UNIQUE INDEX "prospects_converted_store" ON "seller"."prospects" ("converted_store_id") WHERE (converted_store_id IS NOT NULL);
-- Create index "prospects_list" to table: "prospects"
CREATE INDEX "prospects_list" ON "seller"."prospects" ("updated_at" DESC, "prospect_id" DESC);
-- Create index "prospects_merged_into" to table: "prospects"
CREATE INDEX "prospects_merged_into" ON "seller"."prospects" ("merged_into") WHERE (merged_into IS NOT NULL);
-- Create index "prospects_tax_id" to table: "prospects"
CREATE UNIQUE INDEX "prospects_tax_id" ON "seller"."prospects" ("tax_id") WHERE ((tax_id IS NOT NULL) AND live);
-- Create "prospect_categories" table
CREATE TABLE "seller"."prospect_categories" (
  "prospect_id" bigint NOT NULL,
  "category_id" text NOT NULL,
  PRIMARY KEY ("prospect_id", "category_id"),
  CONSTRAINT "prospect_categories_prospect_id_fkey" FOREIGN KEY ("prospect_id") REFERENCES "seller"."prospects" ("prospect_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "prospect_categories_category_check" CHECK ((char_length(category_id) >= 1) AND (char_length(category_id) <= 64))
);
-- Create index "prospect_categories_category" to table: "prospect_categories"
CREATE INDEX "prospect_categories_category" ON "seller"."prospect_categories" ("category_id");
-- Create "prospect_identifiers" table
CREATE TABLE "seller"."prospect_identifiers" (
  "prospect_id" bigint NOT NULL,
  "live" boolean NOT NULL,
  "kind" text NOT NULL,
  "value_normalized" text NOT NULL,
  PRIMARY KEY ("prospect_id", "kind", "value_normalized"),
  CONSTRAINT "prospect_identifiers_prospect_id_live_fkey" FOREIGN KEY ("prospect_id", "live") REFERENCES "seller"."prospects" ("prospect_id", "live") ON UPDATE CASCADE ON DELETE NO ACTION,
  CONSTRAINT "prospect_identifiers_kind_check" CHECK (kind = ANY (ARRAY['EMAIL'::text, 'PHONE'::text, 'DOMAIN'::text, 'SOCIAL'::text])),
  CONSTRAINT "prospect_identifiers_value_check" CHECK ((char_length(value_normalized) >= 1) AND (char_length(value_normalized) <= 500))
);
-- Create index "prospect_identifiers_live" to table: "prospect_identifiers"
CREATE UNIQUE INDEX "prospect_identifiers_live" ON "seller"."prospect_identifiers" ("kind", "value_normalized") WHERE live;
