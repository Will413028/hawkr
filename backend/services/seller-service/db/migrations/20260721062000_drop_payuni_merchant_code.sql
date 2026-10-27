-- Drop sequence "payuni_merchant_code_seq"
DROP SEQUENCE "payuni_merchant_code_seq";
-- Modify "payuni_registrations" table
ALTER TABLE "payuni_registrations" DROP COLUMN "merchant_code";
