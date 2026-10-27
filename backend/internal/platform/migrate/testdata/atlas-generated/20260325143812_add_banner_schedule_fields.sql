-- Modify "banners" table
ALTER TABLE "banners" ADD COLUMN "click_count" bigint NOT NULL DEFAULT 0, ADD COLUMN "start_date" timestamptz NULL, ADD COLUMN "end_date" timestamptz NULL;
