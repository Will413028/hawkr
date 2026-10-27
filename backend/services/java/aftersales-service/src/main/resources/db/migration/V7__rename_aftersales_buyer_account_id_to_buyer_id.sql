-- buyer_id 8-service rekey, Phase 7 (aftersales-service): rename the buyer identity
-- columns from buyer_account_id to buyer_id — name now matches what the value has
-- always actually been (a stint-scoped buyer_id resolved by the gateway resolver /
-- inherited from order-service's already-renamed buyer_id), not a raw login
-- account_id. Pure rename, no backfill (RENAME COLUMN is metadata-only in Postgres;
-- no FK/PK dependency on either column).
--
-- Deliberately NOT touched (see ADR 2026-07-17-buyer-id-8-services-rename-design.md,
-- Phase 7 section, and Global Constraints "audit-trail carve-outs"):
--   - aftersale_evidence.uploaded_by            (PHOTO=buyer_id, INSPECTION_PHOTO=
--     unresolved seller/admin account_id; classification pending Will's sign-off)
--   - aftersale_appeal_events.actor_id           (BUYER/ADMIN/SYSTEM identity space;
--     classification pending Will's sign-off)
--   - aftersale_audit_logs.actor_account_id      (audit-trail carve-out; known gap —
--     for BUYER-type rows this already silently holds gateway-resolved buyer_id, not
--     raw account_id, because ownership and audit-actor share one caller-supplied
--     accountId parameter; fix timing deferred to Will pending Phase 0 divergence
--     baseline query)
--   - aftersale_platform_decisions.decided_by_admin_id (always ADMIN, unambiguous)
--
-- Index idx_aftersale_cases_buyer keeps its name unchanged — it already reads
-- "buyer", not "buyer_account_id"; only the column it indexes is renamed
-- (Postgres updates the index definition automatically, no ALTER INDEX needed).
-- seller_cancel_requests.buyer_account_id has no index dependency
-- (uk_seller_cancel_order_no / idx_seller_cancel_store don't reference it —
-- the latter was idx_seller_cancel_seller before V6's store_id rename).

ALTER TABLE aftersale_cases RENAME COLUMN buyer_account_id TO buyer_id;

COMMENT ON COLUMN aftersale_cases.buyer_id IS
  '售後案件買家識別（stint-scoped buyer_id，由 gateway resolver 解析 / 承接 order-service 已改名的 buyer_id）；buyer_id 8-service rekey Phase 7 更名，原 buyer_account_id';

ALTER TABLE seller_cancel_requests RENAME COLUMN buyer_account_id TO buyer_id;

COMMENT ON COLUMN seller_cancel_requests.buyer_id IS
  '取消出貨請求買家識別（stint-scoped buyer_id）；buyer_id 8-service rekey Phase 7 更名，原 buyer_account_id';

COMMENT ON COLUMN aftersale_evidence.uploaded_by IS
  '證據上傳者：evidence_type=PHOTO 時為買家 buyer_id，evidence_type=INSPECTION_PHOTO 時為未解析的賣家/admin account_id — 雙身份空間欄位，一個名字無法誠實代表兩者，是否拆欄位/改名待 Will 裁決，buyer_id 8-service rekey Phase 7 不動（分類待決）';

COMMENT ON COLUMN aftersale_appeal_events.actor_id IS
  '申訴事件操作者：actor_type=BUYER/ADMIN/SYSTEM 對應不同身份空間，SYSTEM 時為 NULL；BUYER 時可證明等於該 case 的 buyer_id 但是否視為獨立身份欄位改名待 Will 裁決，buyer_id 8-service rekey Phase 7 不動（分類待決）';

COMMENT ON COLUMN aftersale_audit_logs.actor_account_id IS
  '稽核操作者帳號：audit-trail carve-out，設計上應維持跨買家重新開業穩定不變 — 但已知有缺口尚未修復：對 actor_type=BUYER 的 row，因為 ownership 印記與 audit-trail 印記共用同一個呼叫端 accountId 參數（未分開兩條路徑），此欄位實際上已經偷偷存的是 gateway 解析過的 buyer_id 而非 raw account_id，削弱了原始「audit-trail 撐過重新開業」的假設。是否本次修復或記錄後延後，待 Will 依 Phase 0 買家分岔基準值查詢結果決定，buyer_id 8-service rekey Phase 7 不動（KEEP，但非穩定 carve-out）';
