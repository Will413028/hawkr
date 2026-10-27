-- Modify "procurement_quote_batches" table
ALTER TABLE "procurement_quote_batches" ADD COLUMN "delivery_city" text NULL, ADD COLUMN "delivery_date" date NULL, ADD COLUMN "budget_per_unit" bigint NULL, ADD COLUMN "quote_deadline" date NULL, ADD COLUMN "buyer_note" text NULL;
