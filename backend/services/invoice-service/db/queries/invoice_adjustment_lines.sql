
-- name: ListInvoiceAdjustmentLines :many
SELECT * FROM invoice_adjustment_lines
WHERE adjustment_id = sqlc.arg(adjustment_id)
ORDER BY line_seq;

-- name: GetInvoiceAdjustmentLineByID :one
SELECT * FROM invoice_adjustment_lines WHERE id = sqlc.arg(id);

-- name: InsertInvoiceAdjustmentLine :one
INSERT INTO invoice_adjustment_lines (
  adjustment_id, line_seq, source_line_id,
  original_invoice_no, original_invoice_date, original_description,
  quantity, unit_price_ex_tax, net_amount_ex_tax, tax_amount, tax_category, created_at)
VALUES (
  sqlc.arg(adjustment_id), sqlc.arg(line_seq), sqlc.narg(source_line_id),
  sqlc.arg(original_invoice_no), sqlc.arg(original_invoice_date), sqlc.arg(original_description),
  sqlc.arg(quantity), sqlc.arg(unit_price_ex_tax), sqlc.arg(net_amount_ex_tax),
  sqlc.arg(tax_amount), sqlc.arg(tax_category), sqlc.arg(now))
RETURNING *;

-- name: BulkInsertInvoiceAdjustmentLines :copyfrom
INSERT INTO invoice_adjustment_lines (
  adjustment_id, line_seq, source_line_id,
  original_invoice_no, original_invoice_date, original_description,
  quantity, unit_price_ex_tax, net_amount_ex_tax, tax_amount, tax_category, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);
