-- Rename "ai_styles" column thumbnail_url -> thumbnail_key（保留既有值；顯示 URL 改由 Gateway 組，DB 只存 objectKey）
ALTER TABLE "ai_styles" RENAME COLUMN "thumbnail_url" TO "thumbnail_key";
