-- Create index "campaigns_by_end_at" to table: "coupon_campaigns"
CREATE INDEX "campaigns_by_end_at" ON "coupon"."coupon_campaigns" ("end_at");
-- Create "voucher_expiry_reminders" table
CREATE TABLE "coupon"."voucher_expiry_reminders" (
  "voucher_id" uuid NOT NULL,
  "reminded_at" timestamptz NOT NULL,
  PRIMARY KEY ("voucher_id")
);
