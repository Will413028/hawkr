-- Create "expiry_reminder_cursor" table
CREATE TABLE "coupon"."expiry_reminder_cursor" (
  "singleton" boolean NOT NULL DEFAULT true,
  "end_at" timestamptz NOT NULL,
  "voucher_id" uuid NOT NULL,
  PRIMARY KEY ("singleton"),
  CONSTRAINT "expiry_reminder_cursor_singleton_check" CHECK (singleton)
);
