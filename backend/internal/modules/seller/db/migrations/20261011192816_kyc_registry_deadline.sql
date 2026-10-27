-- Modify "kyc_registration_checks" table：期限在建立時寫入。既有的列回填 requested_at 加 15 分鐘（還在 PENDING 的舊列因此已過期，
-- 由掃描 job 收成 UNAVAILABLE 或 CANCELLED），再設 NOT NULL 與檢查。
ALTER TABLE "seller"."kyc_registration_checks" ADD COLUMN "deadline_at" timestamptz NULL;
UPDATE "seller"."kyc_registration_checks" SET "deadline_at" = "requested_at" + interval '15 minutes';
ALTER TABLE "seller"."kyc_registration_checks" ALTER COLUMN "deadline_at" SET NOT NULL, ADD CONSTRAINT "kyc_registration_checks_deadline_check" CHECK (deadline_at > requested_at);
-- Create index "kyc_registration_checks_pending" to table: "kyc_registration_checks"
CREATE INDEX "kyc_registration_checks_pending" ON "seller"."kyc_registration_checks" ("deadline_at") WHERE (outcome = 'PENDING'::text);
