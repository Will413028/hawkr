-- ============================================================================
-- V3: 將 product_import_item_sku.unit 與 shipping_methods.temperature /
--     certifications.name 從 CodeMeta NAME 轉為 enum value（CLAUDE.md §4.1 / §8.4）
--
-- 背景：
--   V1 schema 把 sku.unit / shipping_methods[].temperature / certifications[].name
--   存成 CodeMeta NAME（KG / AMBIENT / ORGANIC），但 product-service
--   CreateProductFromImportInternal 端 CodeMetaValidator 期望 enum value
--   （3301 / 3201 / 3701），導致 commit 階段 100% 失敗。
--   修正方向：DB 與 gRPC 契約一律存 value，CSV / 前端輸入用 NAME（人類可讀），
--   在 CsvParseService / ItemMutationService 寫入邊界統一轉成 value（V3 + code 修正）。
-- ============================================================================

-- 1) sku.unit ----------------------------------------------------------------
-- 放寬 CHECK 容許 value（避開 UPDATE 暫態違反舊 CHECK）
ALTER TABLE product_import_item_sku
    DROP CONSTRAINT IF EXISTS chk_piisku_unit;

UPDATE product_import_item_sku SET unit = '3301' WHERE unit = 'KG';
UPDATE product_import_item_sku SET unit = '3302' WHERE unit = 'CATTY';
UPDATE product_import_item_sku SET unit = '3303' WHERE unit = 'GRAM';

ALTER TABLE product_import_item_sku
    ADD CONSTRAINT chk_piisku_unit CHECK (unit IN ('3301','3302','3303'));

COMMENT ON COLUMN product_import_item_sku.unit IS 'WeightUnit value（3301=KG / 3302=CATTY / 3303=GRAM）';

-- 2) shipping_methods JSONB.temperature ---------------------------------------
-- jsonb_set 對陣列元素逐一改寫；先用 lateral 攤平再 aggregate 回來
UPDATE product_import_item_sku s
SET shipping_methods = sub.new_methods
FROM (
    SELECT s2.id,
           COALESCE(jsonb_agg(
               CASE elem->>'temperature'
                   WHEN 'AMBIENT' THEN jsonb_set(elem, '{temperature}', '"3201"'::jsonb)
                   WHEN 'CHILLED' THEN jsonb_set(elem, '{temperature}', '"3202"'::jsonb)
                   WHEN 'FROZEN'  THEN jsonb_set(elem, '{temperature}', '"3203"'::jsonb)
                   ELSE elem
               END
           ), '[]'::jsonb) AS new_methods
    FROM product_import_item_sku s2,
         LATERAL jsonb_array_elements(s2.shipping_methods) AS elem
    WHERE jsonb_typeof(s2.shipping_methods) = 'array'
    GROUP BY s2.id
) AS sub
WHERE s.id = sub.id;

-- 3) product_import_item.certifications JSONB.name ----------------------------
UPDATE product_import_item p
SET certifications = sub.new_certs
FROM (
    SELECT p2.id,
           COALESCE(jsonb_agg(
               CASE elem->>'name'
                   WHEN 'ORGANIC' THEN jsonb_set(elem, '{name}', '"3701"'::jsonb)
                   WHEN 'TAP'     THEN jsonb_set(elem, '{name}', '"3702"'::jsonb)
                   WHEN 'CAS'     THEN jsonb_set(elem, '{name}', '"3703"'::jsonb)
                   WHEN 'HACCP'   THEN jsonb_set(elem, '{name}', '"3704"'::jsonb)
                   WHEN 'ISO'     THEN jsonb_set(elem, '{name}', '"3705"'::jsonb)
                   WHEN 'GMP'     THEN jsonb_set(elem, '{name}', '"3706"'::jsonb)
                   ELSE elem
               END
           ), '[]'::jsonb) AS new_certs
    FROM product_import_item p2,
         LATERAL jsonb_array_elements(p2.certifications) AS elem
    WHERE jsonb_typeof(p2.certifications) = 'array'
    GROUP BY p2.id
) AS sub
WHERE p.id = sub.id;
