-- Create "seller_prospects" table
CREATE TABLE "seller_prospects" (
  "id" bigint NOT NULL,
  "display_name" character varying(255) NOT NULL,
  "vendor_type" character varying(20) NOT NULL,
  "status" character varying(20) NOT NULL DEFAULT 'ACTIVE',
  "outreach_status" character varying(30) NOT NULL DEFAULT 'NOT_CONTACTED',
  "tax_id" character varying(8) NULL,
  "converted_store_id" bigint NULL,
  "converted_at" timestamptz NULL,
  "match_email" character varying(255) NOT NULL DEFAULT '',
  "match_phone" character varying(50) NOT NULL DEFAULT '',
  "match_domain" character varying(255) NOT NULL DEFAULT '',
  "match_social" character varying(255) NOT NULL DEFAULT '',
  "public_email" character varying(255) NOT NULL DEFAULT '',
  "public_phone" character varying(50) NOT NULL DEFAULT '',
  "public_website" character varying(500) NOT NULL DEFAULT '',
  "public_social" character varying(500) NOT NULL DEFAULT '',
  "category" character varying(100) NOT NULL DEFAULT '',
  "region" character varying(100) NOT NULL DEFAULT '',
  "product_summary" text NOT NULL DEFAULT '',
  "positioning" text NOT NULL DEFAULT '',
  "research_summary" text NOT NULL DEFAULT '',
  "priority_letter" character varying(4) NOT NULL DEFAULT '',
  "priority_stars" integer NOT NULL DEFAULT 0,
  "created_by_account_id" bigint NULL,
  "updated_by_account_id" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_sp_converted" CHECK (((status)::text <> 'CONVERTED'::text) OR (converted_store_id IS NOT NULL)),
  CONSTRAINT "chk_sp_outreach" CHECK ((outreach_status)::text = ANY ((ARRAY['NOT_CONTACTED'::character varying, 'INVITED'::character varying, 'REPLIED'::character varying, 'DECLINED'::character varying, 'DO_NOT_CONTACT'::character varying, 'CONVERTED'::character varying])::text[])),
  CONSTRAINT "chk_sp_status" CHECK ((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'ARCHIVED'::character varying, 'CONVERTED'::character varying])::text[])),
  CONSTRAINT "chk_sp_tax_id" CHECK ((tax_id IS NULL) OR ((tax_id)::text ~ '^[0-9]{8}$'::text)),
  CONSTRAINT "chk_sp_vendor_type" CHECK ((vendor_type)::text = ANY ((ARRAY['individual'::character varying, 'business'::character varying])::text[]))
);
-- Create index "idx_seller_prospects_active" to table: "seller_prospects"
CREATE INDEX "idx_seller_prospects_active" ON "seller_prospects" ("status") WHERE ((status)::text = 'ACTIVE'::text);
-- Create index "idx_seller_prospects_match_domain" to table: "seller_prospects"
CREATE INDEX "idx_seller_prospects_match_domain" ON "seller_prospects" ("match_domain") WHERE ((match_domain)::text <> ''::text);
-- Create index "idx_seller_prospects_match_email" to table: "seller_prospects"
CREATE INDEX "idx_seller_prospects_match_email" ON "seller_prospects" ("match_email") WHERE ((match_email)::text <> ''::text);
-- Create index "idx_seller_prospects_match_phone" to table: "seller_prospects"
CREATE INDEX "idx_seller_prospects_match_phone" ON "seller_prospects" ("match_phone") WHERE ((match_phone)::text <> ''::text);
-- Create index "idx_seller_prospects_match_social" to table: "seller_prospects"
CREATE INDEX "idx_seller_prospects_match_social" ON "seller_prospects" ("match_social") WHERE ((match_social)::text <> ''::text);
-- Create index "uq_seller_prospects_converted_store" to table: "seller_prospects"
CREATE UNIQUE INDEX "uq_seller_prospects_converted_store" ON "seller_prospects" ("converted_store_id") WHERE (converted_store_id IS NOT NULL);
-- Create index "uq_seller_prospects_tax_id" to table: "seller_prospects"
CREATE UNIQUE INDEX "uq_seller_prospects_tax_id" ON "seller_prospects" ("tax_id") WHERE (tax_id IS NOT NULL);
-- Create "seller_prospect_invites" table
CREATE TABLE "seller_prospect_invites" (
  "id" bigint NOT NULL,
  "prospect_id" bigint NOT NULL,
  "token" character varying(64) NOT NULL,
  "sent_at" timestamptz NULL,
  "expires_at" timestamptz NOT NULL,
  "clicked_at" timestamptz NULL,
  "consumed_at" timestamptz NULL,
  "revoked_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "seller_prospect_invites_prospect_id_fkey" FOREIGN KEY ("prospect_id") REFERENCES "seller_prospects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_spi_prospect" to table: "seller_prospect_invites"
CREATE INDEX "idx_spi_prospect" ON "seller_prospect_invites" ("prospect_id", "created_at" DESC);
-- Create index "uq_spi_token" to table: "seller_prospect_invites"
CREATE UNIQUE INDEX "uq_spi_token" ON "seller_prospect_invites" ("token");
