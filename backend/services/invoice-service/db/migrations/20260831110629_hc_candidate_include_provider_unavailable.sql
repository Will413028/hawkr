-- Drop index "idx_ipa_health" from table: "invoice_provider_accounts"
DROP INDEX "idx_ipa_health";
-- Create index "idx_ipa_health" to table: "invoice_provider_accounts"
CREATE INDEX "idx_ipa_health" ON "invoice_provider_accounts" ("health_check_next_at") WHERE ((status)::text = ANY (ARRAY[('ACTIVE'::character varying)::text, ('DEGRADED'::character varying)::text, ('PROVIDER_UNAVAILABLE'::character varying)::text]));
