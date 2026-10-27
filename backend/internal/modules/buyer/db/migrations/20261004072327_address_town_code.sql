-- Existing addresses used free-text city/district names; clear them so the town_code NOT NULL constraint holds (ADR 2026-10-04-address-town-code).
DELETE FROM "buyer"."shipping_addresses";
-- Modify "shipping_addresses" table
ALTER TABLE "buyer"."shipping_addresses" DROP COLUMN "city", DROP COLUMN "district", DROP COLUMN "detail_address", ADD COLUMN "town_code" text NOT NULL, ADD COLUMN "address_detail" text NOT NULL, ADD CONSTRAINT "shipping_addresses_town_code_check" CHECK (town_code ~ '^[0-9]{8}$'::text), ADD CONSTRAINT "shipping_addresses_address_detail_check" CHECK ((char_length(address_detail) >= 1) AND (char_length(address_detail) <= 255));
