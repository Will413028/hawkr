-- 關店後的兩段清理（ADR 2026-10-19-seller-kyc-payuni-consolidated 的 2026-10-20 修訂）、來函的撤銷登記、主管機關回覆的冪等鍵、追蹤逾期的設定時間。
-- Drop index "authority_orders_open_due" from table: "authority_orders"
DROP INDEX "seller"."authority_orders_open_due";
-- Modify "authority_orders" table
ALTER TABLE "seller"."authority_orders" ADD COLUMN "revoked_at" timestamptz NULL, ADD COLUMN "revoked_by_admin_id" bigint NULL, ADD COLUMN "revoke_reason" text NULL, ADD CONSTRAINT "authority_orders_revoke_check" CHECK ((((revoked_at IS NULL) AND (revoked_by_admin_id IS NULL) AND (revoke_reason IS NULL)) OR ((revoked_at >= registered_at) AND (revoked_by_admin_id > 0) AND ((char_length(revoke_reason) >= 1) AND (char_length(revoke_reason) <= 2000)))) IS TRUE), ADD CONSTRAINT "authority_orders_terminal_check" CHECK ((completed_at IS NULL) OR (revoked_at IS NULL));
-- Create index "authority_orders_open_due" to table: "authority_orders"
CREATE INDEX "authority_orders_open_due" ON "seller"."authority_orders" ("due_at", "id") WHERE ((completed_at IS NULL) AND (revoked_at IS NULL));
-- Modify "case_referral_replies" table
ALTER TABLE "seller"."case_referral_replies" ADD COLUMN "request_id" uuid NULL, ADD COLUMN "request_hash" bytea NULL, ADD CONSTRAINT "case_referral_replies_request_check" CHECK ((request_id IS NULL) = (request_hash IS NULL)), ADD CONSTRAINT "case_referral_replies_request_key" UNIQUE ("recorded_by_admin_id", "request_id");
-- Modify "case_referrals" table：既有的追蹤日期是記錄轉報時設的（之後記下回覆換成下一個時，那一則回覆的時間不在舊列上：保守地取記錄轉報的時間，
-- 只會讓之後記下的回覆都算「設定之後的回覆」，不會把已回覆的誤判成逾期）。
ALTER TABLE "seller"."case_referrals" ADD COLUMN "follow_up_set_at" timestamptz NULL;
UPDATE "seller"."case_referrals" SET "follow_up_set_at" = "recorded_at" WHERE "follow_up_on" IS NOT NULL;
ALTER TABLE "seller"."case_referrals" ADD CONSTRAINT "case_referrals_follow_up_check" CHECK ((follow_up_on IS NULL) = (follow_up_set_at IS NULL));
-- Drop index "stores_retire_pending" from table: "stores"
DROP INDEX "seller"."stores_retire_pending";
-- Modify "stores" table：已清理個資的店家在當時已一併退場（清理是一段的時候，圖片與公開內容在同一個 transaction 清掉），所以 retired_at 取 pii_purged_at。
ALTER TABLE "seller"."stores" ADD COLUMN "retired_at" timestamptz NULL;
UPDATE "seller"."stores" SET "retired_at" = "pii_purged_at" WHERE "pii_purged_at" IS NOT NULL;
ALTER TABLE "seller"."stores" DROP CONSTRAINT "stores_closed_images_check", ADD CONSTRAINT "stores_closed_images_check" CHECK ((retired_at IS NULL) OR ((logo_key IS NULL) AND (cover_key IS NULL))), DROP CONSTRAINT "stores_pii_purged_check", ADD CONSTRAINT "stores_pii_purged_check" CHECK (((pii_purged_at IS NULL) AND (store_name IS NOT NULL)) OR ((pii_purged_at IS NOT NULL) AND (retired_at IS NOT NULL) AND (status = 'CLOSED'::text) AND (store_name IS NULL) AND (contact_phone IS NULL) AND (contact_name IS NULL) AND (contact_mobile IS NULL) AND (support_email IS NULL) AND (town_code IS NULL) AND (address_detail IS NULL))), ADD CONSTRAINT "stores_retired_check" CHECK ((retired_at IS NULL) OR ((status = 'CLOSED'::text) AND (description IS NULL) AND (prize IS NULL) AND (website_url IS NULL)));
-- Create index "stores_retire_pending" to table: "stores"
CREATE INDEX "stores_retire_pending" ON "seller"."stores" ("store_id") WHERE ((status = 'CLOSED'::text) AND (retired_at IS NULL));
-- Create index "stores_purge_pending" to table: "stores"
CREATE INDEX "stores_purge_pending" ON "seller"."stores" ("store_id") WHERE ((retired_at IS NOT NULL) AND (pii_purged_at IS NULL));
