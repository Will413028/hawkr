
-- name: ListInvoiceDocumentLines :many
SELECT * FROM invoice_document_lines
WHERE document_id = sqlc.arg(document_id)
ORDER BY line_seq;

-- name: GetInvoiceDocumentLineByID :one
SELECT * FROM invoice_document_lines WHERE id = sqlc.arg(id);

-- name: InsertInvoiceDocumentLine :one
INSERT INTO invoice_document_lines (
  document_id, line_seq, line_type, order_item_id, sku_id, description, unit,
  quantity, unit_price, net_amount, tax_category, tax_rate_bps, remark,
  allocated_discount, allowanceable_base, created_at)
VALUES (
  sqlc.arg(document_id), sqlc.arg(line_seq), sqlc.arg(line_type),
  sqlc.narg(order_item_id), sqlc.narg(sku_id), sqlc.arg(description), sqlc.narg(unit),
  sqlc.arg(quantity), sqlc.arg(unit_price), sqlc.arg(net_amount),
  sqlc.arg(tax_category), sqlc.arg(tax_rate_bps), sqlc.narg(remark),
  sqlc.arg(allocated_discount), sqlc.arg(allowanceable_base), sqlc.arg(now))
RETURNING *;

-- name: BulkInsertInvoiceDocumentLines :copyfrom
INSERT INTO invoice_document_lines (
  document_id, line_seq, line_type, order_item_id, sku_id, description, unit,
  quantity, unit_price, net_amount, tax_category, tax_rate_bps, remark,
  allocated_discount, allowanceable_base, created_at)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16);

-- name: DeleteInvoiceDocumentLines :execrows
DELETE FROM invoice_document_lines
WHERE document_id = sqlc.arg(document_id)
  AND EXISTS (
    SELECT 1 FROM invoice_documents d
    WHERE d.id = sqlc.arg(document_id)
      AND d.status IN ('PENDING', 'AWAITING_MANUAL'));

-- name: AddInvoiceDocumentLineAllowancedAmount :execrows
UPDATE invoice_document_lines
SET allowanced_amount = allowanced_amount + sqlc.arg(delta_amount)
WHERE id = sqlc.arg(id)
  AND allowanced_amount + sqlc.arg(delta_amount) <= allowanceable_base
  AND allowanced_amount + sqlc.arg(delta_amount) >= 0;
