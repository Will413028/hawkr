-- ② seller/store aggregate re-key（design 2026-07-11 §3.2）：商品擁有權 key 從
-- created_by（登入 account_id）改為 seller_account_id（store 代理鍵）。
-- created_by / updated_by 自此降級為純 audit 欄位。
--
-- 部署機制：Flyway 於 pod 啟動執行；本檔必須在 dev/stage write-freeze
-- （product-service scale to 0）視窗內與新 code 同 deploy，backfill + NOT NULL 原子完成。
-- 前置 gate：docs/runbooks/product-rekey-backfill-gate.md（G1/G2 先跑；
-- revived seller 存在時，repair UPDATE 在本 migration 之後、解除 freeze 之前執行）。
ALTER TABLE product ADD COLUMN IF NOT EXISTS seller_account_id BIGINT;

-- Backfill 簡單規則 := created_by。從未 revive 的賣家（surrogate == account_id）此規則精確；
-- revived seller 的例外由 gate runbook 的 R1/R2 修（stated, not assumed）。
UPDATE product SET seller_account_id = created_by WHERE seller_account_id IS NULL;

ALTER TABLE product ALTER COLUMN seller_account_id SET NOT NULL;

-- 鏡射既有 owner 索引（idx_product_created_by / idx_product_status_created_by，V1:93,101）
CREATE INDEX IF NOT EXISTS idx_product_seller_account_id ON product(seller_account_id);
CREATE INDEX IF NOT EXISTS idx_product_status_seller_account_id ON product(status, seller_account_id);

COMMENT ON COLUMN product.seller_account_id IS
  '商品擁有權 key（store 代理鍵，非登入 account_id）；建立當下由請求 accountId snapshot（resolver 上線後為 current surrogate），永不改寫（entity updateStrategy=NEVER）';
