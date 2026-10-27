-- 一次性資料處理（與 schema migration 分檔）：把既有實體的名字與 P4 seed 的 27 條
-- 別名遷進 priceable_surface_form。generated column 會自己算 form_norm，本檔不含
-- 任何正規化邏輯。每段都 ON CONFLICT DO NOTHING，可重跑。
--
-- 之後的實體由 ingest 的 upsert 同交易登記（internal/usecase/surface_form_register.go），
-- 本檔只補「這一刀之前」就存在的列。

-- 大類正式名
INSERT INTO priceable_surface_form (entity_kind, entity_code, form_raw, form_kind)
SELECT 'CROP_GROUP', group_code, name, 'CANONICAL'
FROM crop_group WHERE active AND length(btrim(name)) > 0
ON CONFLICT DO NOTHING;

-- 子類正式名（完整名，如「甘藍-初秋」）
INSERT INTO priceable_surface_form (entity_kind, entity_code, form_raw, form_kind)
SELECT 'CROP', crop_code, name, 'CANONICAL'
FROM crop WHERE active AND length(btrim(name)) > 0
ON CONFLICT DO NOTHING;

-- 子類品種名（「初秋」）
INSERT INTO priceable_surface_form (entity_kind, entity_code, form_raw, form_kind)
SELECT 'CROP', crop_code, variant_name, 'VARIANT'
FROM crop WHERE active AND variant_name IS NOT NULL AND length(btrim(variant_name)) > 0
ON CONFLICT DO NOTHING;

-- 商品（commodity）正式名。SOURCE_TERM（毛豬／白肉雞／稉種…）不在這裡猜，
-- 由各 adapter 的規格表宣告、在 ingest 時登記。
INSERT INTO priceable_surface_form (entity_kind, entity_code, form_raw, form_kind)
SELECT 'COMMODITY', id::text, name, 'CANONICAL'
FROM commodity WHERE active AND length(btrim(name)) > 0
ON CONFLICT DO NOTHING;

-- P4 seed 的 27 條別名（全部對到大類；CSV 的 crop_code 欄全空）。
-- 大類名在 crop_group 裡不唯一時跳過——與退場前 SearchCropCatalog.groupMatch 的行為
-- 一致（同名大類是 ingest 殘留，挑 groups[0] 會給出「看起來完整、其實只有一半子類」
-- 的序列）。alias 與正式名相同的 5 條（category_l3）會撞 PK、由上面的 CANONICAL 勝出。
-- 來源 reference/product_crop_seed.csv sha256=6289fab60394a14f8af2130b48ac84f4042b2828ecc8a3365941329d3426e657（27 列）
INSERT INTO priceable_surface_form (entity_kind, entity_code, form_raw, form_kind)
SELECT 'CROP_GROUP', g.group_code, s.alias_name, 'ALIAS'
FROM (VALUES
    ('高麗菜', '甘藍', ''),
    ('蕃茄', '番茄', ''),
    ('紅蘿蔔', '胡蘿蔔', ''),
    ('地瓜', '甘薯', ''),
    ('番薯', '甘薯', ''),
    ('蕃薯', '甘薯', ''),
    ('甘藷', '甘薯', ''),
    ('地瓜葉', '甘薯葉', ''),
    ('大黃瓜', '胡瓜', ''),
    ('小黃瓜', '花胡瓜', ''),
    ('大白菜', '包心白菜', ''),
    ('四季豆', '敏豆', ''),
    ('空心菜', '蕹菜', ''),
    ('香菜', '芫荽', ''),
    ('青江菜', '青江白菜', ''),
    ('大陸妹', '萵苣菜', ''),
    ('A菜', '萵苣菜', ''),
    ('a菜', '萵苣菜', ''),
    ('白蘿蔔', '蘿蔔', ''),
    ('香菇', '濕香菇', ''),
    ('青花菜', '青花苔', ''),
    ('蔥', '青蔥', ''),
    ('甘藍', '甘藍', ''),
    ('番茄', '番茄', ''),
    ('甘薯', '甘薯', ''),
    ('胡蘿蔔', '胡蘿蔔', ''),
    ('敏豆', '敏豆', '')
) AS s(alias_name, crop_group_name, crop_code)
JOIN crop_group g ON g.name = s.crop_group_name AND g.active
WHERE (SELECT count(*) FROM crop_group g2 WHERE g2.name = s.crop_group_name AND g2.active) = 1
ON CONFLICT DO NOTHING;
