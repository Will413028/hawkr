-- Modify "accounts" table
ALTER TABLE "payment"."accounts" ADD COLUMN "seller_read_claimed_at" timestamptz NULL;
