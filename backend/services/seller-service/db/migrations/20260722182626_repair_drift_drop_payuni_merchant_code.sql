-- Repair migration: 20260721062000_drop_payuni_merchant_code recorded a
-- clean apply in dev's atlas_schema_revisions ledger, but the target dev DB
-- never actually had the column/sequence dropped (ledger-vs-reality drift,
-- plausibly a side effect of the store_code epic's manual ledger-repair
-- workaround, 2026-07-21). Confirmed live 2026-07-23: dev's payuni_registrations
-- still has merchant_code NOT NULL, blocking every fresh insert; prod already
-- has neither. IF EXISTS makes this a safe no-op wherever the original drop
-- genuinely already happened (prod, and dev once this actually runs).
ALTER TABLE "payuni_registrations" DROP COLUMN IF EXISTS "merchant_code";
-- Drop sequence "payuni_merchant_code_seq"
DROP SEQUENCE IF EXISTS "payuni_merchant_code_seq";
