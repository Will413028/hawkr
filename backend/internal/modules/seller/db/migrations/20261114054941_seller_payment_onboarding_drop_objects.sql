-- Stripe 收款開通：先拿掉依賴舊表（payuni_registrations）或定義有變的 view 與函式，之後的 migration 再改表、再依 objects.sql 重建（單體規則 6）。
-- 依賴順序：pub_review_items 讀 review_payuni_bind_retry；pub_exposable_stores 讀 store_requirement_flags。
DROP FUNCTION seller.pub_review_items(timestamptz);
DROP FUNCTION seller.pub_exposable_stores(timestamptz);
DROP FUNCTION seller.review_payuni_bind_retry(timestamptz);
DROP VIEW seller.store_requirement_flags;

-- 後面收緊約束、改主鍵與加 NOT NULL 欄位的表，先刪掉會讓它們失敗的退役列（依外鍵順序：問題與同意先於審核紀錄）。
DELETE FROM "seller"."kyc_review_issues" WHERE field NOT IN ('product_type', 'website_url', 'business.company_name', 'business.tax_id', 'business.company_register_date', 'business.company_address', 'business.company_phone', 'business.company_documents');
DELETE FROM "seller"."kyc_review_issues" WHERE review_id IN (SELECT review_id FROM "seller"."kyc_reviews" WHERE action = 'REBIND');
DELETE FROM "seller"."agreement_acceptances" WHERE kind = 'PAYUNI_TERMS' OR submission_review_id IN (SELECT review_id FROM "seller"."kyc_reviews" WHERE action = 'REBIND');
DELETE FROM "seller"."kyc_reviews" WHERE action = 'REBIND';
DELETE FROM "seller"."account_identity_hashes" WHERE field NOT IN ('TAX_ID', 'BANK_FINGERPRINT');
DELETE FROM "seller"."measure_identities" WHERE kind NOT IN ('TAX_ID', 'BANK_ACCOUNT');
DELETE FROM "seller"."kyc_documents" WHERE slot <> 'COMPANY_DOCUMENT';
DELETE FROM "seller"."kyc_document_accesses";
