-- invoice_issuers｜開票主體
-- SSOT：設計書 58 號 §5.1.1
-- 本波只放骨架：§5 有 DDL，但開票主體的建立／退休流程（端點）在本波範圍外，
-- 待實作對應 usecase 時補 Insert／Retire。

-- name: GetInvoiceIssuerByID :one
SELECT * FROM invoice_issuers WHERE id = sqlc.arg(id);

-- name: GetInvoiceIssuerByRef :one
-- uq_invoice_issuers_ref ⇒ 一個 store_id 只會有一列 issuer。
-- issuer_ref_id：SELLER ⇒ sellers.store_id（**非 account_id**）；PLATFORM ⇒ 0。
SELECT * FROM invoice_issuers
WHERE issuer_type = sqlc.arg(issuer_type) AND issuer_ref_id = sqlc.arg(issuer_ref_id);

-- name: InsertInvoiceIssuer :one
-- 開票主體建立。uq_invoice_issuers_ref（issuer_type, issuer_ref_id）⇒ 一個 store_id 只會有一列。
-- ⚠️ ON CONFLICT DO NOTHING **不指名 conflict target**（§7.6 X05 的通則）：本表目前只有一個
--    唯一索引，但不指名一樣正確，且未來加第二個唯一約束時不會突然開始拋 23505——而在
--    PostgreSQL 上交易內任一句失敗即整個交易 aborted（25P02），catch 了也救不回來。
-- 回 0 列 ⇒ 該 (issuer_type, issuer_ref_id) 已有 issuer，呼叫端改讀 GetInvoiceIssuerByRef。
INSERT INTO invoice_issuers (
  id, issuer_type, issuer_ref_id, display_name, status, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(issuer_type), sqlc.arg(issuer_ref_id), sqlc.arg(display_name),
  'active', sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: RetireInvoiceIssuer :execrows
-- 開票主體退休。chk_invoice_issuers_retired 要求 retired 必有 retired_at ⇒ 兩欄必須同一句寫，
-- 少寫 retired_at 當場 23514。
-- 'active' 正面列出（禁 <>）：已 retired 的列再退休一次 rows=0，呼叫端當成 no-op 或衝突自行決定。
UPDATE invoice_issuers
SET status = 'retired',
    retired_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'active'
  AND version = sqlc.arg(expected_version);

-- name: UpdateInvoiceIssuerDisplayName :execrows
-- display_name ＝ **法定名稱**（§5.1.1 欄位註解），而它的來源只有 seller_tax_profiles.legal_name
-- ——賣家在端點 18 填的那一欄。issuer 列必須在賣家填完法定名稱**之前**就存在（否則
-- seller_tax_profiles 沒有 FK 對象），所以建立當下只能先放一個佔位字串。
--
-- 🔴 少了本句，那個佔位字串會一路留到開票：invoice_documents.issuer_name_snapshot 取的就是
--    display_name，而發票上的銷售人名稱寫錯**只能作廢重開**（§6.1.8 端點 24 的 9334 同一個理由）。
-- `display_name <> $2` 讓重複存檔不空轉 version：rows=0 在本句是 no-op 不是失敗，
-- 呼叫端不得把它當守衛不成立。
UPDATE invoice_issuers
SET display_name = sqlc.arg(display_name),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'active'
  AND display_name <> sqlc.arg(display_name);
