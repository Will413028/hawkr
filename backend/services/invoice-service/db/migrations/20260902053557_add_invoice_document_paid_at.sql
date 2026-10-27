-- Modify "invoice_documents" table
ALTER TABLE "invoice_documents" ADD COLUMN "paid_at" timestamptz NULL;
