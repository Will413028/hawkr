-- Plan C Phase 1 (coupon-service): seller_account_id → store_id rekey.
-- docs/superpowers/specs/2026-07-15-coupon-order-aftersales-store-id-rekey-design.md §Phase 1.
--
-- issuer_account_id → issuer_store_id: this column is written by BOTH admin-issued
-- templates (createTemplate/createCompensationTemplate — value is a genuine admin
-- account_id, never a store surrogate) and seller-issued templates
-- (createSellerTemplate/createSellerCompensationTemplate — value is the store_id
-- surrogate once CouponGrpcService's semantic fix lands, Task 4 of this plan). The
-- column stays a single generic BIGINT holding either an account_id (issuer_type=
-- ADMIN rows) or a store_id (issuer_type=SELLER rows) — a deliberate, documented
-- design choice (spec's own wording), not an oversight. Most queries that read this
-- column for identity-comparison purposes are already scoped
-- WHERE issuer_type = 'SELLER' (see disableActiveTemplatesBySuspension /
-- restoreTemplatesFromSuspension), so the mixed semantics don't leak into a wrong
-- comparison there. However, two call sites compare issuer_store_id for identity
-- WITHOUT an issuer_type filter — a pre-existing gap that predates this rename
-- (the same unscoped WHERE/comparison shape already existed under the old
-- issuer_account_id column name) and is NOT introduced by this migration:
--   - CouponTemplateMapper#revokeTemplate: `WHERE template_id = #{templateId}
--     AND issuer_store_id = #{storeId} AND status = '1'` — no issuer_type filter.
--   - CouponGrpcService#sellerGetTemplateDetail: `template.getIssuerStoreId().equals(
--     sellerAccountId)` — no issuer_type check before the comparison.
-- Whichever later task in this plan owns fixing coupon-service's identity-comparison
-- logic should treat these two as in scope.
--
-- Postgres RENAME COLUMN is metadata-only and auto-updates dependent index
-- definitions (idx_tpl_scope_status, idx_tpl_issuer, idx_inst_seller_owner) without
-- needing separate ALTER INDEX statements — verified: index names are unchanged,
-- only the column references inside them follow the rename automatically.

ALTER TABLE coupon_templates RENAME COLUMN applicable_seller_account_id TO applicable_store_id;
ALTER TABLE coupon_templates RENAME COLUMN issuer_account_id TO issuer_store_id;

ALTER TABLE coupon_instances RENAME COLUMN applicable_seller_account_id TO applicable_store_id;

-- seller_coupon_quotas PK rename: itself the fix for the re-onboard collision risk —
-- a re-onboarded store mints a new store_id, so it naturally gets a fresh quota row
-- instead of silently inheriting a stale row keyed by the old account_id. No backfill
-- logic needed beyond the column rename itself (existing rows keep their PK value,
-- just under the new column name).
ALTER TABLE seller_coupon_quotas RENAME COLUMN seller_account_id TO store_id;

COMMENT ON COLUMN coupon_templates.applicable_store_id IS
  '適用商店 store_id（NULL = 全平台券）- Plan C Phase 1 rename from applicable_seller_account_id';
COMMENT ON COLUMN coupon_templates.issuer_store_id IS
  '建立者 id：issuer_type=ADMIN 時為 admin accountId，issuer_type=SELLER 時為 store_id（欄位保持通用，語意由 issuer_type 決定）- Plan C Phase 1 rename from issuer_account_id';
COMMENT ON COLUMN coupon_instances.applicable_store_id IS
  '冗餘自 Template 的適用商店 store_id - Plan C Phase 1 rename from applicable_seller_account_id';
COMMENT ON COLUMN seller_coupon_quotas.store_id IS
  '商店 store_id（PK）- Plan C Phase 1 rename from seller_account_id；重開店會拿到新 store_id，天然獲得全新配額列';
