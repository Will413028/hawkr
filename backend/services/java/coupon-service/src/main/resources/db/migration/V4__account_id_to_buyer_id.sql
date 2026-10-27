-- buyer_id 8-service rekey, Phase 5 (coupon-service): account_id -> buyer_id rekey.
-- docs/superpowers/specs/2026-07-17-buyer-id-8-services-rename-design.md §Phase 5.
--
-- Naming convention (Will confirmed 2026-07-18, mirrors store_id Plan C's asymmetric
-- treatment of seller_coupon_quotas vs coupon_templates): owner_ prefix kept only when
-- a store/template-scoping column coexists in the same table.
--   - coupon_instances.owner_account_id / coupon_redemption_logs.owner_account_id
--     -> owner_buyer_id (prefix kept: both tables also carry template/store-scoped
--        columns in the same row -- applicable_store_id / template_id).
--   - coupon_target_accounts.account_id / coupon_claims.buyer_account_id -> buyer_id
--     (bare: single-identity tables, no store/template-scope column to disambiguate
--     against).
--
-- Two pre-existing gaps discovered during this phase's re-verification, NOT introduced
-- by this migration and NOT fixed by it (out of this phase's confirmed scope; flagged
-- here for whoever picks them up next, same convention as V3's own carve-out notes):
--   - CouponGrpcService#sellerGetTemplateDetail (grpc/CouponGrpcService.java:574-587)
--     passes the buyer_id values read from coupon_target_accounts.buyer_id straight
--     into AccountGrpcClient.batchGetAccountInfo(), which queries account-service's
--     `accounts` table by raw account_id PK. If a targeted buyer has since diverged
--     (buyer_id != account_id), that seller-facing nickname/phone lookup will silently
--     return nothing for that entry. Same bug class as the listMyCoupons/
--     buyerClaimCoupon GrpcAuthContext misread this phase's Semantic fixes do address,
--     but this specific call site was not in the ADR's confirmed scope for Phase 5.
--   - gateway/internal/handler/coupon.go's AdminCreateCouponTemplate handler passes
--     req.BuyerAccountIds straight through to AdminCreateTemplateRequest without ever
--     calling resolveTargetBuyerAccountIDs (unlike its CreateCouponTemplate/
--     AdminIssueCouponsHandler siblings), so admin-created DESIGNATED/COMPENSATION
--     templates can seed coupon_target_accounts.buyer_id with unresolved raw
--     account_id values. Pre-existing, independent of this rename.
--
-- Postgres RENAME COLUMN is metadata-only and auto-updates dependent index/constraint
-- definitions (idx_inst_owner_status, idx_inst_seller_owner, uk_inst_template_owner,
-- coupon_target_accounts' composite PK + idx_target_account, coupon_claims' composite
-- PK + idx_claim_buyer) without needing separate ALTER INDEX/ALTER TABLE ... DROP
-- CONSTRAINT statements -- same pattern already validated by V3 on coupon_templates/
-- coupon_instances/seller_coupon_quotas in this same service. Index *names* are left
-- unchanged (same precedent as V3, which kept idx_inst_seller_owner's name after
-- renaming applicable_seller_account_id -> applicable_store_id in the same table).

ALTER TABLE coupon_instances RENAME COLUMN owner_account_id TO owner_buyer_id;
ALTER TABLE coupon_redemption_logs RENAME COLUMN owner_account_id TO owner_buyer_id;
ALTER TABLE coupon_target_accounts RENAME COLUMN account_id TO buyer_id;
ALTER TABLE coupon_claims RENAME COLUMN buyer_account_id TO buyer_id;

COMMENT ON COLUMN coupon_instances.owner_buyer_id IS
  '持有者買家 buyer_id（stint-scoped）- buyer_id rekey Phase 5 rename from owner_account_id';
COMMENT ON COLUMN coupon_redemption_logs.owner_buyer_id IS
  '持有者買家 buyer_id（stint-scoped）- buyer_id rekey Phase 5 rename from owner_account_id';
COMMENT ON COLUMN coupon_target_accounts.buyer_id IS
  '指定買家 buyer_id（stint-scoped）- buyer_id rekey Phase 5 rename from account_id';
COMMENT ON COLUMN coupon_claims.buyer_id IS
  '領券買家 buyer_id（stint-scoped）- buyer_id rekey Phase 5 rename from buyer_account_id';
