-- Stripe 收款開通：先拿掉依賴舊表（payuni_registrations）或定義有變的 view 與函式，之後的 migration 再改表、再依 objects.sql 重建（單體規則 6）。
-- 依賴順序：pub_review_items 讀 review_payuni_bind_retry；pub_exposable_stores 讀 store_requirement_flags。
DROP FUNCTION seller.pub_review_items(timestamptz);
DROP FUNCTION seller.pub_exposable_stores(timestamptz);
DROP FUNCTION seller.review_payuni_bind_retry(timestamptz);
DROP VIEW seller.store_requirement_flags;

-- dev 的種子資料會寫入的列，與後面收緊的約束衝突，所以先刪：
-- 種子的入駐同意含 PAYUNI_TERMS（SubmitKYCAgreements），agreement_acceptances_kind_check 不再收它。
DELETE FROM "seller"."agreement_acceptances" WHERE kind = 'PAYUNI_TERMS';
-- 種子的 KYC 識別欄位經 syncHashes 寫入 ID_CARD_NO 與 BANK_ACCOUNT 的 hash，account_identity_hashes_field_check 只收 TAX_ID 與 BANK_FINGERPRINT。
DELETE FROM "seller"."account_identity_hashes" WHERE field NOT IN ('TAX_ID', 'BANK_FINGERPRINT');
