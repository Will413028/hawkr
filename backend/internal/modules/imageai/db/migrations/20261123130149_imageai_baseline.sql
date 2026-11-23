-- Add new schema named "imageai"
CREATE SCHEMA "imageai";
-- Create "quotas" table
CREATE TABLE "imageai"."quotas" (
  "store_id" bigint NOT NULL,
  "weekly_limit" integer NOT NULL,
  "memo" text NOT NULL DEFAULT '',
  "version" bigint NOT NULL DEFAULT 1,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "quotas_memo_check" CHECK (memo !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text),
  CONSTRAINT "quotas_store_check" CHECK (store_id > 0),
  CONSTRAINT "quotas_weekly_limit_check" CHECK (weekly_limit >= 0)
);
-- Set comment to column: "version" on table: "quotas"
COMMENT ON COLUMN "imageai"."quotas"."version" IS '樂觀鎖的版本，對外是 etag；每次修改加一';
-- Create "styles" table
CREATE TABLE "imageai"."styles" (
  "id" uuid NOT NULL,
  "name" text NOT NULL,
  "label" text NOT NULL,
  "prompt" text NOT NULL,
  "category" text NOT NULL,
  "thumbnail_key" text NOT NULL DEFAULT '',
  "thumbnail_asset_id" text NOT NULL DEFAULT '',
  "display_order" integer NOT NULL DEFAULT 0,
  "is_default" boolean NOT NULL DEFAULT false,
  "enabled" boolean NOT NULL DEFAULT true,
  "version" bigint NOT NULL DEFAULT 1,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "styles_category_name_key" UNIQUE ("category", "name"),
  CONSTRAINT "styles_category_check" CHECK (category = ANY (ARRAY['PRODUCT'::text, 'POST'::text])),
  CONSTRAINT "styles_label_check" CHECK ((char_length(label) >= 1) AND (label !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "styles_name_check" CHECK ((char_length(name) >= 1) AND (name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "styles_prompt_check" CHECK ((char_length(prompt) >= 1) AND (char_length(prompt) <= 500) AND (prompt !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))
);
-- Create index "styles_default_per_category" to table: "styles"
CREATE UNIQUE INDEX "styles_default_per_category" ON "imageai"."styles" ("category") WHERE is_default;
-- Set comment to column: "prompt" on table: "styles"
COMMENT ON COLUMN "imageai"."styles"."prompt" IS '提示詞，最多 500 個字元（A9-D40，以 Unicode 字元計）';
-- Set comment to column: "version" on table: "styles"
COMMENT ON COLUMN "imageai"."styles"."version" IS '樂觀鎖的版本，對外是 etag；每次修改加一（ADR 2026-09-25-optimistic-concurrency-etag）';
-- Create "usage" table
CREATE TABLE "imageai"."usage" (
  "store_id" bigint NOT NULL,
  "period_start" timestamptz NOT NULL,
  "used" integer NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id", "period_start"),
  CONSTRAINT "usage_store_check" CHECK (store_id > 0),
  CONSTRAINT "usage_used_check" CHECK (used >= 0)
);

-- 每個用途一筆中性的預設風格（A9-D39）：沒有預設風格的用途不存在，伺服器不另寫後備提示詞。
INSERT INTO imageai.styles (id, name, label, prompt, category, is_default, created_at, updated_at) VALUES
    ('019e9a00-0000-7000-8000-000000000001', 'default', '預設', '調整光線、色彩與清晰度，讓主體更突出，保持畫面內容與主體外觀的真實，不新增或移除物件。', 'PRODUCT', TRUE, NOW(), NOW()),
    ('019e9a00-0000-7000-8000-000000000002', 'default', '預設', '調整光線、色彩與清晰度，讓畫面更美觀，保持畫面內容與主體外觀的真實，不新增或移除物件。', 'POST', TRUE, NOW(), NOW());
