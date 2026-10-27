-- buyer_invoice_preferences｜買家常用發票資料（append-only）
-- SSOT：設計書 58 號 §5.1.4
-- 本波只放骨架：B08「我的發票資料」的增刪改查（端點）在本波範圍外。
-- ⚠️ 補 query 時三件事：① 全表唯一有 soft delete 的表 ⇒ 每支查詢都要帶
--    `deleted_at IS NULL AND superseded_at IS NULL`（三個 partial unique 索引的述詞都含這兩條）
--    ② Tier A：buyer_id 一律寫進 WHERE，不是查回來再比對（§8.1.4(b)）
--    ③ carrier_value_ciphertext 是密文，明文只在 mapper.go 出入

-- name: GetBuyerInvoicePreferenceByID :one
-- buyer_id 是述詞的一部分（Tier A 防枚舉）：查不到與不屬於你刻意不可分辨。
SELECT * FROM buyer_invoice_preferences
WHERE id = sqlc.arg(id)
  AND buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL;

-- name: GetDefaultBuyerInvoicePreference :one
-- USE_BUYER_DEFAULT 的第一步（58 §6.1.2(b)：`買家指定的預設 → 無載具發票`，**只有兩步**）。
--
-- 🔴 **不可用 ListBuyerInvoicePreferences ＋ Go 端過濾代替**：那支帶 LIMIT，預設那一列
--    排在 limit 之外時會**靜默**回「這個買家沒有預設」⇒ 該張發票退成無載具，而買家設過。
--    沒有任何東西會報錯，只有買家自己發現發票沒歸戶。
-- 述詞四條與 GetBuyerInvoicePreferenceByID 對齊（buyer_id 是授權述詞的一部分）。
-- is_default 全表最多一列為真（由 ClearDefault ＋ SetDefault 成對維護），仍加 LIMIT 1
--    讓 :one 在資料異常時不炸而是取一列——異常本身由 uq 索引與那對寫入路徑守。
SELECT * FROM buyer_invoice_preferences
WHERE buyer_id = sqlc.arg(buyer_id)
  AND is_default = true
  AND deleted_at IS NULL
  AND superseded_at IS NULL
LIMIT 1;

-- name: ListBuyerInvoicePreferences :many
-- B08「我的發票資料」列表。吃 idx_bip_buyer（partial 述詞含 deleted_at／superseded_at 兩條）。
SELECT * FROM buyer_invoice_preferences
WHERE buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: InsertBuyerInvoicePreference :one
-- I-14「值欄位一旦寫入不再變更」的落實方式：本表**沒有** UPDATE 值欄位的 query，
-- 「編輯」＝ 新增一列 ＋ SupersedeBuyerInvoicePreference 把舊列指過去。
-- ⚠️ ON CONFLICT DO NOTHING 不指名 target：本表有**三個** partial unique
--    （uq_bip_default／uq_bip_carrier_dedup／uq_bip_taxid_dedup），指名其一 ⇒ 另兩個衝突時
--    直接 23505 並讓整個交易 aborted（25P02）。rows=0 ⇒ 呼叫端回「這筆資料已存在」。
-- carrier_value_ciphertext 是密文，明文只在 mapper.go 出入；carrier_value_hash 由同一層算。
INSERT INTO buyer_invoice_preferences (
  id, buyer_id, preference_kind, carrier_type,
  carrier_value_ciphertext, carrier_value_hash, carrier_value_mask,
  tax_id, tax_title, donation_code, label, is_default, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(buyer_id), sqlc.arg(preference_kind), sqlc.narg(carrier_type),
  sqlc.narg(carrier_value_ciphertext), sqlc.narg(carrier_value_hash), sqlc.narg(carrier_value_mask),
  sqlc.narg(tax_id), sqlc.narg(tax_title), sqlc.narg(donation_code), sqlc.narg(label),
  sqlc.arg(is_default), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ClearDefaultBuyerInvoicePreferences :execrows
-- 設定預設的前半（必須與 SetDefaultBuyerInvoicePreference 在同一交易內）。
-- uq_bip_default 是 partial UNIQUE(buyer_id) WHERE is_default AND 兩個 NULL ⇒ 先清後設，
-- 否則第二筆設定會撞唯一鍵。述詞逐字含 deleted_at／superseded_at 兩條（與該索引同源）。
UPDATE buyer_invoice_preferences
SET is_default = false, updated_at = sqlc.arg(now)
WHERE buyer_id = sqlc.arg(buyer_id)
  AND is_default
  AND deleted_at IS NULL
  AND superseded_at IS NULL;

-- name: SetDefaultBuyerInvoicePreference :execrows
-- 設定預設的後半。chk_bip_default_alive 要求 is_default 的列必須未刪除且未被取代 ⇒
-- 述詞帶那兩條不只是防越權，少了會 23514。
-- buyer_id 在述詞內（Tier A 防枚舉，§8.1.4(b)）：查不到與不屬於你刻意不可分辨。
UPDATE buyer_invoice_preferences
SET is_default = true, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL;

-- name: SupersedeBuyerInvoicePreference :execrows
-- 「編輯」的後半：舊列指向新列。chk_bip_superseded_pair 是雙向等式
-- (superseded_at IS NULL) = (superseded_by_id IS NULL) ⇒ 兩欄必須同一句寫。
-- 被取代的列同時離開三個 partial unique 與 idx_bip_buyer 的述詞 ⇒ 新列不會撞去重鍵。
-- ⚠️ 若舊列是預設，is_default 必須一併清掉：chk_bip_default_alive 禁止「已取代但仍是預設」。
UPDATE buyer_invoice_preferences
SET superseded_at = sqlc.arg(now),
    superseded_by_id = sqlc.arg(superseded_by_id),
    is_default = false,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL;

-- name: SoftDeleteBuyerInvoicePreference :execrows
-- 全表唯一有 soft delete 的表。同上：清 is_default 是 chk_bip_default_alive 的要求，不是禮貌。
UPDATE buyer_invoice_preferences
SET deleted_at = sqlc.arg(now),
    is_default = false,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND buyer_id = sqlc.arg(buyer_id)
  AND deleted_at IS NULL
  AND superseded_at IS NULL;
