-- Modify "invoice_document_lines" table
ALTER TABLE "invoice_document_lines" DROP COLUMN "sku_id", ADD COLUMN "sku_id" uuid NULL;
