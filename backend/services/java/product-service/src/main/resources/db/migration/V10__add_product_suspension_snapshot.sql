-- 44 號 §5.10a / 32 號 §13.5：賣家停權觸發下架的可逆快照表。
-- SellerSuspendedConsumer 消費 seller.suspended 時，把本次因停權下架（ACTIVE→INACTIVE）
-- 的精確 product ids 寫入本表（scope=suspension_id）；解除停權時 SellerUnsuspendedConsumer
-- 依快照精準還原，不誤還原賣家本來就自行下架的商品。
CREATE TABLE product_suspension_snapshot (
  suspension_id  BIGINT      NOT NULL,
  product_id     BIGINT      NOT NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (suspension_id, product_id)
);

COMMENT ON TABLE product_suspension_snapshot IS
  '賣家停權觸發下架的可逆快照（PK 天然冪等 ON CONFLICT DO NOTHING）；scope 綁定 suspension_id，解除時只還原本次停權下架的商品 - 設計書 02 號 §5.10a';
