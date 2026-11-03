-- Modify "account_identity_hashes" table
ALTER TABLE "seller"."account_identity_hashes" DROP CONSTRAINT "account_identity_hashes_field_check", ADD CONSTRAINT "account_identity_hashes_field_check" CHECK (field = ANY (ARRAY['ID_CARD_NO'::text, 'REP_ID_CARD_NO'::text, 'TAX_ID'::text, 'BANK_ACCOUNT'::text, 'BANK_FINGERPRINT'::text]));
