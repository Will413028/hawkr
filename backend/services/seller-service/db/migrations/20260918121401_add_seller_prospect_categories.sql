-- Create "seller_prospect_categories" table
CREATE TABLE "seller_prospect_categories" (
  "prospect_id" bigint NOT NULL,
  "category_id" character varying(64) NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("prospect_id", "category_id"),
  CONSTRAINT "seller_prospect_categories_prospect_id_fkey" FOREIGN KEY ("prospect_id") REFERENCES "seller_prospects" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "chk_spc_category_id" CHECK ((category_id)::text <> ''::text)
);
-- Create index "idx_spc_category" to table: "seller_prospect_categories"
CREATE INDEX "idx_spc_category" ON "seller_prospect_categories" ("category_id");
