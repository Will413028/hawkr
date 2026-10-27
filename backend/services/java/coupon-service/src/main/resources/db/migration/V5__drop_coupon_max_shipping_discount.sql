-- 物理移除免運券券種（FREE_SHIPPING, type=3）遺留欄位（#1367）。
-- 平台政策不提供免運券，prod 存量為 0；券種 enum / proto 欄位 / caller 已同波移除。
-- 破壞性 DDL：DROP COLUMN。coupon_templates.max_shipping_discount 僅供 FREE_SHIPPING 使用，
-- CASH / PERCENT 券從未寫入此欄，移除無資料損失。
ALTER TABLE coupon_templates
    DROP COLUMN max_shipping_discount;
