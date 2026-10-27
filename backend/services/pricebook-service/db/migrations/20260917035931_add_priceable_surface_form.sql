-- pg_trgm 供 priceable_surface_form 的 gin_trgm_ops 索引使用。
-- Atlas community 版不管理 extension（diff 不會產出這一行），沿用 content-service
-- 20260308000001_create_recipes.sql 與 social-service 20260516091316_add_post_hashtag.sql 的先例：
-- 手加在 migration 開頭，再 atlas migrate hash。pg_trgm 自 PG13 起是 trusted extension，
-- 執行 migration 的 role 只要對該 database 有 CREATE 權限即可。
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Create "priceable_surface_form" table
CREATE TABLE "priceable_surface_form" (
  "entity_kind" text NOT NULL,
  "entity_code" text NOT NULL,
  "form_raw" text NOT NULL,
  "form_kind" text NOT NULL,
  "form_norm" text NOT NULL GENERATED ALWAYS AS (lower(translate(regexp_replace(form_raw, '[[:space:][:punct:]、。，！？：；（）「」『』【】《》〈〉—…·～]'::text, ''::text, 'g'::text), 'ＡＢＣＤＥＦＧＨＩＪＫＬＭＮＯＰＱＲＳＴＵＶＷＸＹＺａｂｃｄｅｆｇｈｉｊｋｌｍｎｏｐｑｒｓｔｕｖｗｘｙｚ０１２３４５６７８９臺'::text, 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789台'::text))) STORED,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("entity_kind", "entity_code", "form_norm"),
  CONSTRAINT "priceable_surface_form_form_kind_check" CHECK (form_kind = ANY (ARRAY['CANONICAL'::text, 'SOURCE_TERM'::text, 'VARIANT'::text, 'ALIAS'::text])),
  CONSTRAINT "priceable_surface_form_kind_check" CHECK (entity_kind = ANY (ARRAY['CROP_GROUP'::text, 'CROP'::text, 'COMMODITY'::text])),
  CONSTRAINT "priceable_surface_form_raw_not_blank" CHECK (length(btrim(form_raw)) > 0)
);
-- Create index "priceable_surface_form_entity_idx" to table: "priceable_surface_form"
CREATE INDEX "priceable_surface_form_entity_idx" ON "priceable_surface_form" ("entity_kind", "entity_code");
-- Create index "priceable_surface_form_norm_trgm_idx" to table: "priceable_surface_form"
CREATE INDEX "priceable_surface_form_norm_trgm_idx" ON "priceable_surface_form" USING GIN ("form_norm" gin_trgm_ops);
