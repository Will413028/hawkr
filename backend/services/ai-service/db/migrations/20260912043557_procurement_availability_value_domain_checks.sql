-- Modify "procurement_price_previews" table
ALTER TABLE "procurement_price_previews" ADD CONSTRAINT "chk_procurement_price_previews_reference_availability" CHECK (reference_availability = ANY (ARRAY['AVAILABLE'::text, 'UNAVAILABLE'::text]));
-- Modify "procurement_quote_batches" table
ALTER TABLE "procurement_quote_batches" ADD CONSTRAINT "chk_procurement_quote_batches_price_availability" CHECK (price_availability = ANY (ARRAY['MARKET'::text, 'MARKET_STALE'::text, 'UNAVAILABLE'::text, 'BUYER_ENTERED'::text]));
-- Modify "procurement_rfq_publication_approvals" table
ALTER TABLE "procurement_rfq_publication_approvals" ADD CONSTRAINT "chk_prpa_reference_availability" CHECK (reference_availability = ANY (ARRAY['AVAILABLE'::text, 'UNAVAILABLE'::text]));
