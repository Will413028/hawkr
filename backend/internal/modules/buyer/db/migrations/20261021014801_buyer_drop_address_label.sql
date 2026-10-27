-- Modify "shipping_addresses" table
ALTER TABLE "buyer"."shipping_addresses" DROP CONSTRAINT "shipping_addresses_label_check", DROP COLUMN "label";
