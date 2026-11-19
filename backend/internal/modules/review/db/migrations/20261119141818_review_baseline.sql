-- Add new schema named "review"
CREATE SCHEMA "review";
-- Create "inactive_authors" table
CREATE TABLE "review"."inactive_authors" (
  "buyer_id" bigint NOT NULL,
  "since" timestamptz NOT NULL,
  PRIMARY KEY ("buyer_id"),
  CONSTRAINT "inactive_authors_buyer_check" CHECK (buyer_id > 0)
);
-- Create "media_ref_sync" table
CREATE TABLE "review"."media_ref_sync" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "seq" bigint NOT NULL DEFAULT 1,
  "synced_seq" bigint NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("ref_type", "ref_id")
);
-- Create index "media_ref_sync_unsynced" to table: "media_ref_sync"
CREATE INDEX "media_ref_sync_unsynced" ON "review"."media_ref_sync" ("updated_at") WHERE (synced_seq < seq);
-- Create "retired_assets" table
CREATE TABLE "review"."retired_assets" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "asset_id" text NOT NULL,
  "retired_at" timestamptz NOT NULL,
  "unlinked_at" timestamptz NULL,
  PRIMARY KEY ("ref_type", "ref_id", "asset_id")
);
-- Create index "retired_assets_retired_at" to table: "retired_assets"
CREATE INDEX "retired_assets_retired_at" ON "review"."retired_assets" ("retired_at");
-- Create "reviews" table
CREATE TABLE "review"."reviews" (
  "id" uuid NOT NULL,
  "order_no" text NOT NULL,
  "order_item_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "product_id" bigint NOT NULL,
  "rating" smallint NOT NULL,
  "content" text NULL,
  "sku_name" text NOT NULL,
  "item_image_key" text NULL,
  "price" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  "reply_content" text NULL,
  "replied_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "reviews_order_item_key" UNIQUE ("order_item_id"),
  CONSTRAINT "reviews_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "reviews_content_check" CHECK ((content IS NULL) OR (((char_length(content) >= 1) AND (char_length(content) <= 1000)) AND (content !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))),
  CONSTRAINT "reviews_item_image_key_check" CHECK ((item_image_key IS NULL) OR (item_image_key <> ''::text)),
  CONSTRAINT "reviews_order_item_check" CHECK (order_item_id > 0),
  CONSTRAINT "reviews_order_no_check" CHECK ((char_length(order_no) >= 1) AND (char_length(order_no) <= 64)),
  CONSTRAINT "reviews_price_check" CHECK (price >= 0),
  CONSTRAINT "reviews_product_check" CHECK (product_id > 0),
  CONSTRAINT "reviews_rating_check" CHECK ((rating >= 1) AND (rating <= 5)),
  CONSTRAINT "reviews_reply_content_check" CHECK ((reply_content IS NULL) OR (((char_length(reply_content) >= 1) AND (char_length(reply_content) <= 1000)) AND (reply_content !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))),
  CONSTRAINT "reviews_reply_pair_check" CHECK ((reply_content IS NULL) = (replied_at IS NULL)),
  CONSTRAINT "reviews_sku_name_check" CHECK (char_length(sku_name) <= 255),
  CONSTRAINT "reviews_store_check" CHECK (store_id > 0)
);
-- Create index "reviews_buyer" to table: "reviews"
CREATE INDEX "reviews_buyer" ON "review"."reviews" ("buyer_id", "created_at" DESC, "id" DESC);
-- Create index "reviews_product" to table: "reviews"
CREATE INDEX "reviews_product" ON "review"."reviews" ("product_id", "created_at" DESC, "id" DESC);
-- Create index "reviews_store" to table: "reviews"
CREATE INDEX "reviews_store" ON "review"."reviews" ("store_id", "created_at" DESC, "id" DESC);
-- Create "hides" table
CREATE TABLE "review"."hides" (
  "id" uuid NOT NULL,
  "review_id" uuid NOT NULL,
  "reason_category" text NOT NULL,
  "reason_text" text NOT NULL DEFAULT '',
  "source_ref" text NULL,
  "admin_id" bigint NOT NULL,
  "hidden_at" timestamptz NOT NULL,
  "restored_at" timestamptz NULL,
  "restored_by" bigint NULL,
  "restore_reason" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "hides_review_id_fkey" FOREIGN KEY ("review_id") REFERENCES "review"."reviews" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "hides_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "hides_reason_category_check" CHECK (reason_category = ANY (ARRAY['MISLEADING_INFO'::text, 'IP_INFRINGEMENT'::text, 'FRAUD'::text, 'OFF_PLATFORM_TRADE'::text, 'PRIVACY'::text, 'IMPERSONATION'::text, 'PLATFORM_ABUSE'::text, 'SPAM'::text, 'HARASSMENT'::text, 'HATE'::text, 'SEXUAL_CONTENT'::text])),
  CONSTRAINT "hides_reason_text_check" CHECK ((char_length(reason_text) <= 500) AND (reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "hides_restore_check" CHECK (((restored_at IS NULL) AND (restored_by IS NULL) AND (restore_reason IS NULL)) OR ((restored_at IS NOT NULL) AND (restored_by > 0) AND (restore_reason IS NOT NULL) AND (restored_at >= hidden_at))),
  CONSTRAINT "hides_restore_reason_check" CHECK ((restore_reason IS NULL) OR (((char_length(restore_reason) >= 1) AND (char_length(restore_reason) <= 500)) AND (restore_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))),
  CONSTRAINT "hides_source_ref_check" CHECK ((source_ref IS NULL) OR (((char_length(source_ref) >= 1) AND (char_length(source_ref) <= 200)) AND (source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)))
);
-- Create index "hides_active_review" to table: "hides"
CREATE UNIQUE INDEX "hides_active_review" ON "review"."hides" ("review_id") WHERE (restored_at IS NULL);
-- Create index "hides_review" to table: "hides"
CREATE INDEX "hides_review" ON "review"."hides" ("review_id", "hidden_at");
-- Create "review_images" table
CREATE TABLE "review"."review_images" (
  "review_id" uuid NOT NULL,
  "position" smallint NOT NULL,
  "asset_id" text NOT NULL,
  "object_key" text NOT NULL,
  PRIMARY KEY ("review_id", "position"),
  CONSTRAINT "review_images_review_id_fkey" FOREIGN KEY ("review_id") REFERENCES "review"."reviews" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "review_images_asset_check" CHECK (asset_id <> ''::text),
  CONSTRAINT "review_images_key_check" CHECK (object_key <> ''::text),
  CONSTRAINT "review_images_position_check" CHECK (("position" >= 0) AND ("position" <= 3))
);
