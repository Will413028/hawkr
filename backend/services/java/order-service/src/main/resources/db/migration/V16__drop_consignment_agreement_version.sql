-- 受託代銷附約版本退場（2026-09-20 決策）
--
-- chk_orders_consignment_agreement 要求 PLATFORM_CONSIGNMENT 的訂單必帶
-- consignment_agreement_version，而附約的簽署機制不會建立 ⇒ 該欄永遠無來源，
-- EXTERNAL lane 的建單會恆在 fail-closed 上停住。
--
-- ⚠️ 移除的是法律存證欄，不是帳務形狀：PLATFORM_CONSIGNMENT 與政策其餘五欄原樣保留，
--    chk_orders_policy_matrix／chk_orders_invoice_collection 不動，撥款軸的部分索引
--    idx_orders_consignment_settlement 也不動（它索引 commerce_model，不碰本欄）。

-- 🔴 DROP 前先斷言零資料。dev 實查 0 列；prod 結構上也不可能有——EXTERNAL lane 建單
--    對 agentMerId 空值 fail-closed，而兩環境的 PAYUNI_AGENT_MER_ID 都是空字串
--    ⇒ PLATFORM_CONSIGNMENT 訂單從未建得出來。斷言失敗代表這個推論錯了：
--    停下來看資料，不要把斷言拿掉。
DO $$
DECLARE
    stale_count bigint;
BEGIN
    SELECT count(*) INTO stale_count
    FROM orders
    WHERE consignment_agreement_version IS NOT NULL;

    IF stale_count > 0 THEN
        RAISE EXCEPTION
            'consignment_agreement_version 有 % 列非空，與「從未建出代銷訂單」的前提矛盾',
            stale_count;
    END IF;
END $$;

ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_consignment_agreement;
ALTER TABLE orders DROP COLUMN IF EXISTS consignment_agreement_version;
