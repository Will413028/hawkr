-- Modify "buyer_quote_acceptances" table
ALTER TABLE "procurement"."buyer_quote_acceptances" ADD COLUMN "order_ref" bigint NOT NULL GENERATED ALWAYS AS IDENTITY, ADD COLUMN "order_job_id" bigint NULL, ADD CONSTRAINT "buyer_quote_acceptances_order_ref_key" UNIQUE ("order_ref");
