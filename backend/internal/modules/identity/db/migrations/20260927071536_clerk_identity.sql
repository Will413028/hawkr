-- Drop index "uidx_accounts_email" from table: "accounts"
DROP INDEX "identity"."uidx_accounts_email";
-- Modify "accounts" table
-- 手改：既有的列（本機的 seed）沒有 Clerk 的使用者，填上不會撞到 Clerk ID 的值，再設 NOT NULL。
ALTER TABLE "identity"."accounts" DROP CONSTRAINT "accounts_live_check", DROP CONSTRAINT "accounts_phone_check", ADD CONSTRAINT "accounts_deleted_check" CHECK ((deleted_at IS NULL) OR ((email IS NULL) AND (suspended_at IS NULL))), DROP COLUMN "phone", ADD COLUMN "clerk_user_id" text;
UPDATE "identity"."accounts" SET "clerk_user_id" = 'unlinked:' || "id";
ALTER TABLE "identity"."accounts" ALTER COLUMN "clerk_user_id" SET NOT NULL;
-- Create index "uidx_accounts_clerk_user_id" to table: "accounts"
CREATE UNIQUE INDEX "uidx_accounts_clerk_user_id" ON "identity"."accounts" ("clerk_user_id");
-- Modify "admins" table
-- 手改：同 accounts。
ALTER TABLE "identity"."admins" DROP COLUMN "google_subject", ADD COLUMN "clerk_user_id" text;
UPDATE "identity"."admins" SET "clerk_user_id" = 'unlinked:' || "id";
ALTER TABLE "identity"."admins" ALTER COLUMN "clerk_user_id" SET NOT NULL;
-- Create index "uidx_admins_clerk_user_id" to table: "admins"
CREATE UNIQUE INDEX "uidx_admins_clerk_user_id" ON "identity"."admins" ("clerk_user_id");
-- Drop "apple_token_revocations" table
DROP TABLE "identity"."apple_token_revocations";
-- Drop "link_tokens" table
DROP TABLE "identity"."link_tokens";
-- Drop "oauth_links" table
DROP TABLE "identity"."oauth_links";
-- Drop "phone_codes" table
DROP TABLE "identity"."phone_codes";
-- Drop "sessions" table
DROP TABLE "identity"."sessions";
