-- invoice_document_lines｜品項行
-- SSOT：設計書 58 號 §5.1.6
-- 本波只放骨架：開票時的行組裝與折讓分攤（allowanced_amount 累加）在本波範圍外。
-- ⚠️ 補 query 時：line_seq 決定送 AMEGO 的 ProductItem 順序，任何列表一律 ORDER BY line_seq。

-- name: ListInvoiceDocumentLines :many
SELECT * FROM invoice_document_lines
WHERE document_id = sqlc.arg(document_id)
ORDER BY line_seq;

-- name: GetInvoiceDocumentLineByID :one
SELECT * FROM invoice_document_lines WHERE id = sqlc.arg(id);

-- name: InsertInvoiceDocumentLine :one
-- 開票時的行組裝落點（§5A.3）。三個 CHECK 決定呼叫端必須先算好才送：
--   chk_invline_amount          net_amount = unit_price × quantity
--   chk_invline_sign            DISCOUNT 行 net_amount < 0，其餘 >= 0
--   chk_invline_allowance_shape DISCOUNT 行三個折讓欄恆 0；其餘
--                               allowanceable_base = net_amount − allocated_discount
-- ⇒ **allowanceable_base 不是可有可無的冗餘欄**：算錯當場 23514，不會靜默寫進去。
--   它同時是 I-65（Σ allowanceable_base 只計 PRODUCT ＋ SHIPPING 行 = document.total_amount）
--   的被加數。
-- line_seq 決定送 AMEGO 的 ProductItem 順序（1-based，上限 9999）；uq_invline_doc_seq
--   讓同一份文件的同一個序號只可能有一列。
-- 單價語意是**含稅**（與 invoice_adjustment_lines 的未稅相反，不要照抄另一張表的欄名）。
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
-- 同一份文件的整批明細行（COPY 協定，一次來回）。與上面那一支是同一組欄位、同一組 CHECK：
-- COPY **不繞過** CHECK／UNIQUE／FK，所以「批次寫入時約束比較鬆」這個常見誤解在這裡不成立。
-- ⚠️ 沒有 ON CONFLICT：copyfrom 不支援。重跑同一份文件的行組裝會撞 uq_invline_doc_seq，
--    這是刻意的——重建明細行的正確做法是同交易內先刪後建（fk_invline_document 是
--    ON DELETE CASCADE，但刪 document 不是選項），不是靠衝突靜默跳過。
-- ⚠️ 本句要求 database.DBTX 具備 CopyFrom（pkg/database 的 DBTX 已含），
--    交易感知照舊由 baseRepo.q(ctx) 提供。
INSERT INTO invoice_document_lines (
  document_id, line_seq, line_type, order_item_id, sku_id, description, unit,
  quantity, unit_price, net_amount, tax_category, tax_rate_bps, remark,
  allocated_discount, allowanceable_base, created_at)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16);

-- name: DeleteInvoiceDocumentLines :execrows
-- 買家付款後修改（M6 R03 UPDATED_BEFORE_ISSUE）要「重建 lines」⇒ 先刪後建，同一交易內。
-- 只允許在文件尚未送出時重建：述詞正面列出兩個狀態（子查詢讀 invoice_documents 的權威 status），
-- 少了它，一份已 ISSUED 的發票會被就地換掉明細而號碼不變 —— 稅務上那是偽造。
-- ⚠️ 本檔沒有、也不得新增任何 DELETE FROM invoice_documents（I-40）；刪明細行不等於刪文件。
DELETE FROM invoice_document_lines
WHERE document_id = sqlc.arg(document_id)
  AND EXISTS (
    SELECT 1 FROM invoice_documents d
    WHERE d.id = sqlc.arg(document_id)
      AND d.status IN ('PENDING', 'AWAITING_MANUAL'));

-- name: AddInvoiceDocumentLineAllowancedAmount :execrows
-- 部分折讓時逐行累加已折讓額（含稅）。**單句 CAS**，理由同 AddInvoiceDocumentAllowanceTotal：
-- 先查再寫在兩筆折讓併發時會雙雙讀到舊值、雙雙通過上限。
-- chk_invline_allowanced_cap（0 ≤ allowanced_amount ≤ allowanceable_base）是 DB 側兜底；
-- 述詞裡的同一條比較是為了讓超額回 **rows=0**（可翻成業務碼）而不是 23514
-- （PostgreSQL 交易內任一句失敗即整個交易 aborted，之後每一句都被拒）。兩層都要。
-- DISCOUNT 行的 allowanceable_base 恆 0 ⇒ 對它累加任何正數都 rows=0，天然不需要另加述詞。
UPDATE invoice_document_lines
SET allowanced_amount = allowanced_amount + sqlc.arg(delta_amount)
WHERE id = sqlc.arg(id)
  AND allowanced_amount + sqlc.arg(delta_amount) <= allowanceable_base
  AND allowanced_amount + sqlc.arg(delta_amount) >= 0;
