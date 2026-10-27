-- invoice_adjustment_lines｜折讓明細行
-- SSOT：設計書 58 號 §5.1.7
-- 本波只放骨架：g0401 逐行回填原發票資訊的組裝在本波範圍外。
-- ⚠️ 金額語意與 invoice_document_lines **相反**（本表是未稅：unit_price_ex_tax／net_amount_ex_tax），
--    補 query 時不要照抄另一張表的欄名。

-- name: ListInvoiceAdjustmentLines :many
SELECT * FROM invoice_adjustment_lines
WHERE adjustment_id = sqlc.arg(adjustment_id)
ORDER BY line_seq;

-- name: GetInvoiceAdjustmentLineByID :one
SELECT * FROM invoice_adjustment_lines WHERE id = sqlc.arg(id);

-- name: InsertInvoiceAdjustmentLine :one
-- g0401 逐行回填原發票資訊。**金額語意與 invoice_document_lines 相反**：本表是未稅
-- （unit_price_ex_tax／net_amount_ex_tax），開立端 DetailVat=1 是含稅 ——
-- 照抄另一張表的欄名會得到一張金額全錯但每一條 CHECK 都通過的折讓單。
-- chk_invadjline_amount：net_amount_ex_tax = unit_price_ex_tax × quantity（呼叫端先算好）。
-- tax_amount 必須是整數（AMEGO 4040139），拆分尾差由 §5A.9.3 的最大餘數法吸收；
--   I-9 要求 Σ lines.tax_amount = header.tax_amount，那條只有測試守得住（跨列）。
-- original_invoice_no／original_invoice_date／original_description 三欄 NOT NULL：
--   AMEGO g0401 要求**逐行**回填原發票資訊，不是表頭帶一次就好。
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
-- 整批折讓明細（COPY 協定）。欄位與 CHECK 同上——COPY 不繞過 CHECK／UNIQUE／FK。
-- 沒有 ON CONFLICT（copyfrom 不支援）：重跑會撞 uq_invadjline_seq，這是刻意的。
INSERT INTO invoice_adjustment_lines (
  adjustment_id, line_seq, source_line_id,
  original_invoice_no, original_invoice_date, original_description,
  quantity, unit_price_ex_tax, net_amount_ex_tax, tax_amount, tax_category, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);
