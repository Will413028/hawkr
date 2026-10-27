-- Part A / doc 44 §11：雙角色賣家刪賣家 role 後復活，舊商品仍出現在自己下架分頁。
-- seller_deleted_at 於 seller.deleted 消費時（stampSellerDeletedByMerchant）蓋章；
-- IS NOT NULL = 此商品所屬賣家帳號已刪除，賣家端查詢過濾；不影響 public（靠 status）/ order。
ALTER TABLE product ADD COLUMN IF NOT EXISTS seller_deleted_at TIMESTAMPTZ;

COMMENT ON COLUMN product.seller_deleted_at IS
  '賣家帳號刪除時間戳（seller.deleted 消費後由 stampSellerDeletedByMerchant 寫入）；IS NOT NULL = 賣家已刪除，賣家端不顯示；不影響 public（靠 status）/ order';
