-- Modify "procurement_confirmed_rfq_snapshots" table
ALTER TABLE "procurement_confirmed_rfq_snapshots" DROP COLUMN "pricing_mode";
-- Modify "procurement_price_previews" table
ALTER TABLE "procurement_price_previews" DROP CONSTRAINT "chk_procurement_price_previews_pricing_mode", DROP CONSTRAINT "chk_procurement_price_previews_reference_availability", DROP COLUMN "pricing_mode", DROP COLUMN "reference_availability", DROP COLUMN "reference_facts_hash", DROP COLUMN "reference_payload_json", DROP COLUMN "unavailable_reason", DROP COLUMN "deal_temperature_json", DROP COLUMN "market_price_basis_json";
-- Modify "procurement_rfq_publication_approvals" table
ALTER TABLE "procurement_rfq_publication_approvals" DROP CONSTRAINT "chk_prpa_reference_availability", DROP COLUMN "pricing_mode", DROP COLUMN "reference_availability", DROP COLUMN "deal_temperature_status";
