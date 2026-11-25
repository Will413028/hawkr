-- Add new schema named "catalog"
CREATE SCHEMA "catalog";
-- Create "categories" table
CREATE TABLE "catalog"."categories" (
  "id" uuid NOT NULL,
  "parent_id" uuid NULL,
  "name" jsonb NOT NULL,
  "shopify_taxonomy_id" text NULL,
  "position" integer NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "categories_parent_id_fkey" FOREIGN KEY ("parent_id") REFERENCES "catalog"."categories" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "categories_name_check" CHECK ((jsonb_typeof(name) = 'object'::text) AND (COALESCE(jsonb_typeof((name -> 'zh-TW'::text)), ''::text) = 'string'::text) AND (char_length((name ->> 'zh-TW'::text)) >= 1) AND ((NOT (name ? 'en'::text)) OR ((COALESCE(jsonb_typeof((name -> 'en'::text)), ''::text) = 'string'::text) AND (char_length((name ->> 'en'::text)) >= 1)))),
  CONSTRAINT "categories_not_self_parent_check" CHECK (parent_id IS DISTINCT FROM id),
  CONSTRAINT "categories_position_check" CHECK ("position" >= 0),
  CONSTRAINT "categories_taxonomy_shape_check" CHECK ((parent_id IS NOT NULL) OR (shopify_taxonomy_id IS NULL))
);
-- Create index "categories_parent_idx" to table: "categories"
CREATE INDEX "categories_parent_idx" ON "catalog"."categories" ("parent_id", "position");
-- Create index "categories_shopify_taxonomy_key" to table: "categories"
CREATE UNIQUE INDEX "categories_shopify_taxonomy_key" ON "catalog"."categories" ("shopify_taxonomy_id") WHERE (shopify_taxonomy_id IS NOT NULL);
-- Create index "categories_sibling_name_key" to table: "categories"
CREATE UNIQUE INDEX "categories_sibling_name_key" ON "catalog"."categories" ((COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid)), ((name ->> 'zh-TW'::text)));
-- Create "products" table
CREATE TABLE "catalog"."products" (
  "id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "category_id" uuid NOT NULL,
  "title" text NOT NULL,
  "description" jsonb NOT NULL DEFAULT '[]',
  "status" text NOT NULL,
  "first_published_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "min_price" bigint NOT NULL,
  "max_price" bigint NOT NULL,
  "sales_count" integer NOT NULL DEFAULT 0,
  "favorite_count" integer NOT NULL DEFAULT 0,
  "rating_average" double precision NOT NULL DEFAULT 0,
  "review_count" integer NOT NULL DEFAULT 0,
  "searchable_text" text NOT NULL DEFAULT '',
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "products_category_id_fkey" FOREIGN KEY ("category_id") REFERENCES "catalog"."categories" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "products_counts_check" CHECK ((sales_count >= 0) AND (favorite_count >= 0) AND (review_count >= 0)),
  CONSTRAINT "products_description_check" CHECK (jsonb_typeof(description) = 'array'::text),
  CONSTRAINT "products_first_published_check" CHECK ((status <> 'ACTIVE'::text) OR (first_published_at IS NOT NULL)),
  CONSTRAINT "products_price_range_check" CHECK ((min_price >= 0) AND (max_price >= min_price)),
  CONSTRAINT "products_rating_check" CHECK ((rating_average >= (0)::double precision) AND (rating_average <= (5)::double precision) AND ((review_count > 0) OR (rating_average = (0)::double precision))),
  CONSTRAINT "products_status_check" CHECK (status = ANY (ARRAY['DRAFT'::text, 'ACTIVE'::text, 'ARCHIVED'::text])),
  CONSTRAINT "products_store_check" CHECK (store_id > 0),
  CONSTRAINT "products_title_check" CHECK ((char_length(title) >= 1) AND (char_length(title) <= 100) AND (title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))
);
-- Create index "products_category_idx" to table: "products"
CREATE INDEX "products_category_idx" ON "catalog"."products" ("category_id", "id") WHERE ((deleted_at IS NULL) AND (status = 'ACTIVE'::text));
-- Create index "products_public_new_idx" to table: "products"
CREATE INDEX "products_public_new_idx" ON "catalog"."products" ("first_published_at" DESC, "id" DESC) WHERE ((deleted_at IS NULL) AND (status = 'ACTIVE'::text));
-- Create index "products_public_sales_idx" to table: "products"
CREATE INDEX "products_public_sales_idx" ON "catalog"."products" ("sales_count" DESC, "id" DESC) WHERE ((deleted_at IS NULL) AND (status = 'ACTIVE'::text));
-- Create index "products_store_idx" to table: "products"
CREATE INDEX "products_store_idx" ON "catalog"."products" ("store_id", "created_at" DESC, "id" DESC) WHERE (deleted_at IS NULL);
-- Create "product_hashtags" table
CREATE TABLE "catalog"."product_hashtags" (
  "product_id" uuid NOT NULL,
  "name" text NOT NULL,
  "display_name" text NOT NULL,
  "position" integer NOT NULL,
  PRIMARY KEY ("product_id", "name"),
  CONSTRAINT "product_hashtags_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "catalog"."products" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "product_hashtags_name_check" CHECK ((char_length(name) >= 1) AND (char_length(name) <= 40)),
  CONSTRAINT "product_hashtags_position_check" CHECK ("position" >= 0)
);
-- Create "product_images" table
CREATE TABLE "catalog"."product_images" (
  "product_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "position" integer NOT NULL,
  "asset_id" uuid NOT NULL,
  "object_key" text NOT NULL,
  "alt" text NOT NULL DEFAULT '',
  PRIMARY KEY ("product_id", "kind", "position"),
  CONSTRAINT "product_images_product_id_asset_id_key" UNIQUE ("product_id", "asset_id"),
  CONSTRAINT "product_images_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "catalog"."products" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "product_images_alt_check" CHECK (char_length(alt) <= 200),
  CONSTRAINT "product_images_kind_check" CHECK (kind = ANY (ARRAY['GALLERY'::text, 'DESCRIPTION'::text])),
  CONSTRAINT "product_images_position_check" CHECK ("position" >= 0)
);
-- Create "product_option_axes" table
CREATE TABLE "catalog"."product_option_axes" (
  "product_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "name" text NOT NULL,
  PRIMARY KEY ("product_id", "position"),
  CONSTRAINT "product_option_axes_product_id_name_key" UNIQUE ("product_id", "name"),
  CONSTRAINT "product_option_axes_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "catalog"."products" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "option_axes_name_check" CHECK ((char_length(name) >= 1) AND (char_length(name) <= 20) AND (name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "option_axes_position_check" CHECK ("position" = ANY (ARRAY[1, 2]))
);
-- Create "product_option_values" table
CREATE TABLE "catalog"."product_option_values" (
  "product_id" uuid NOT NULL,
  "axis_position" integer NOT NULL,
  "position" integer NOT NULL,
  "value" text NOT NULL,
  PRIMARY KEY ("product_id", "axis_position", "value"),
  CONSTRAINT "product_option_values_product_id_axis_position_position_key" UNIQUE ("product_id", "axis_position", "position") DEFERRABLE INITIALLY DEFERRED,
  CONSTRAINT "product_option_values_product_id_axis_position_fkey" FOREIGN KEY ("product_id", "axis_position") REFERENCES "catalog"."product_option_axes" ("product_id", "position") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "option_values_position_check" CHECK ("position" >= 0),
  CONSTRAINT "option_values_value_check" CHECK ((char_length(value) >= 1) AND (char_length(value) <= 40) AND (value !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))
);
-- Create "product_skus" table
CREATE TABLE "catalog"."product_skus" (
  "id" uuid NOT NULL,
  "product_id" uuid NOT NULL,
  "option1_value" text NULL,
  "option2_value" text NULL,
  "price" bigint NOT NULL,
  "compare_at_price" bigint NULL,
  "stock_qty" integer NOT NULL DEFAULT 0,
  "deleted_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "product_skus_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "catalog"."products" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "skus_compare_at_check" CHECK ((compare_at_price IS NULL) OR (compare_at_price > price)),
  CONSTRAINT "skus_option_order_check" CHECK ((option2_value IS NULL) OR (option1_value IS NOT NULL)),
  CONSTRAINT "skus_price_check" CHECK (price > 0),
  CONSTRAINT "skus_stock_check" CHECK (stock_qty >= 0)
);
-- Create index "skus_combination_key" to table: "product_skus"
CREATE UNIQUE INDEX "skus_combination_key" ON "catalog"."product_skus" ("product_id", (COALESCE(option1_value, ''::text)), (COALESCE(option2_value, ''::text))) WHERE (deleted_at IS NULL);
-- Create index "skus_product_idx" to table: "product_skus"
CREATE INDEX "skus_product_idx" ON "catalog"."product_skus" ("product_id") WHERE (deleted_at IS NULL);
-- Create "stock_operations" table
CREATE TABLE "catalog"."stock_operations" (
  "id" uuid NOT NULL,
  "idempotency_key" text NOT NULL,
  "kind" text NOT NULL,
  "deduct_operation_id" uuid NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "stock_operations_idempotency_key_key" UNIQUE ("idempotency_key"),
  CONSTRAINT "stock_operations_deduct_operation_id_fkey" FOREIGN KEY ("deduct_operation_id") REFERENCES "catalog"."stock_operations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "stock_operations_key_check" CHECK ((char_length(idempotency_key) >= 1) AND (char_length(idempotency_key) <= 200)),
  CONSTRAINT "stock_operations_kind_check" CHECK (kind = ANY (ARRAY['DEDUCT'::text, 'RESTORE'::text, 'ADJUST'::text])),
  CONSTRAINT "stock_operations_restore_ref_check" CHECK ((kind = 'RESTORE'::text) = (deduct_operation_id IS NOT NULL))
);
-- Create index "stock_operations_deduct_idx" to table: "stock_operations"
CREATE INDEX "stock_operations_deduct_idx" ON "catalog"."stock_operations" ("deduct_operation_id") WHERE (deduct_operation_id IS NOT NULL);
-- Create "stock_movements" table
CREATE TABLE "catalog"."stock_movements" (
  "operation_id" uuid NOT NULL,
  "sku_id" uuid NOT NULL,
  "delta" integer NOT NULL,
  PRIMARY KEY ("operation_id", "sku_id"),
  CONSTRAINT "stock_movements_operation_id_fkey" FOREIGN KEY ("operation_id") REFERENCES "catalog"."stock_operations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "stock_movements_sku_id_fkey" FOREIGN KEY ("sku_id") REFERENCES "catalog"."product_skus" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "stock_movements_delta_check" CHECK (delta <> 0)
);
-- Create index "stock_movements_sku_idx" to table: "stock_movements"
CREATE INDEX "stock_movements_sku_idx" ON "catalog"."stock_movements" ("sku_id");
