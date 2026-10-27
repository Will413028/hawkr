-- buyer_id 8-service rekey, Phase 2 (cart-service): account_id -> buyer_id.
-- Mirrors V2's treatment (seller_account_id -> store_id) on this same table.
-- Metadata-only rename; account_id is a partial-unique-index member
-- (uk_cart_account_sku), a non-unique composite-index member
-- (idx_cart_account_store), and a CHECK member (chk_owner_xor) — all three
-- are RENAME COLUMN-safe (Postgres updates dependent index/constraint
-- definitions automatically; verified by V2 on this exact table).
ALTER TABLE cart_items RENAME COLUMN account_id TO buyer_id;

-- 鏡射 V1 建立的索引名稱
ALTER INDEX uk_cart_account_sku RENAME TO uk_cart_buyer_sku;
ALTER INDEX idx_cart_account_store RENAME TO idx_cart_buyer_store;

COMMENT ON COLUMN cart_items.buyer_id IS
  '買家身份（stint-scoped buyer_id，gateway 已解析）。例外：純賣家帳號（無 buyer
   profile）逛購物車時，gateway resolveCartAccountID 刻意 fallback 成 raw 登入
   account_id（見 gateway/internal/handler/cart.go），此欄位不保證 100% 是
   buyer_id，讀取時需留意（buyer_id 8-service rekey Phase 2 semantic-fix 文件化項）。';
