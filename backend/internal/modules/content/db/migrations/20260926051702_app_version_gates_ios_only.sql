-- App 只發 iOS：先刪掉 Android 的門檻，新的 CHECK 才加得上
DELETE FROM "content"."app_version_gates" WHERE "platform" = 'ANDROID';
-- Modify "app_version_gates" table
ALTER TABLE "content"."app_version_gates" DROP CONSTRAINT "app_version_gates_platform_check", ADD CONSTRAINT "app_version_gates_platform_check" CHECK (platform = 'IOS'::text);
-- Set comment to column: "platform" on table: "app_version_gates"
COMMENT ON COLUMN "content"."app_version_gates"."platform" IS 'App 平台 - AppPlatform（IOS）';
