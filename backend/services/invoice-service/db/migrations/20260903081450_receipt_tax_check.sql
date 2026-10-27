-- Modify "invoice_documents" table
ALTER TABLE "invoice_documents" ADD CONSTRAINT "chk_invdoc_receipt_tax" CHECK (((document_kind)::text <> 'RECEIPT'::text) OR (tax_amount = 0));
