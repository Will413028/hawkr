-- category_crop_mapping：L3 一律對 group，不再區分 crop（2026-08-30 推翻同日
-- 稍早的「crop 優先於 group」ADR，見 docs/superpowers/specs/2026-08-30-
-- pricebook-category-price-rollup-foundation-design.md §1/§6）。
--
-- 這支 migration 只搬資料，不動 schema——下一支 migration（simplify_
-- category_crop_mapping_group_only）才會真正 DROP COLUMN crop_code/
-- selection_kind。順序不可顛倒：DROP COLUMN 之前必須先把既有 276 筆 crop
-- 型別的列換成等價的 group_code，否則資料直接消失，無法回復。
--
-- 換算邏輯：crop_code → crop.group_id → crop_group.group_code —— 一個
-- crop 恰好屬於一個 group（crop.group_id 是 NOT NULL FK），所以這個 JOIN
-- 對每一筆合法的 crop_code 恰好命中一列，不會產生扇出。
--
-- 同時更新 selection_kind 從 'crop' → 'group'、清除 crop_code，使得轉換後
-- 的列滿足 CHECK constraint 的第二個分支（resolved + group + 非空 group_code）：
-- (status = 'resolved' AND selection_kind = 'group' AND group_code IS NOT NULL AND crop_code IS NULL)
UPDATE category_crop_mapping ccm
SET group_code = cg.group_code,
    selection_kind = 'group',
    crop_code = NULL
FROM crop c
JOIN crop_group cg ON cg.id = c.group_id
WHERE ccm.selection_kind = 'crop'
  AND ccm.crop_code = c.crop_code
  AND ccm.group_code IS NULL;

-- Fail-closed 防呆：276 筆 bootstrap 資料的 crop_code 全部存在於 crop 表
-- （來自同一批精確比對），理論上不會有列在上面那步之後仍是 resolved 卻沒有
-- group_code。但若這支 migration 套用之前，有人手動停用/刪除了某個
-- crop_code，上面的 JOIN 就不會命中——這裡寧可把該筆降級成 unmapped 交回
-- 人工判斷，也不留一筆會撞下一支 migration 新 CHECK constraint 的孤兒列
-- （沿用 issue #2786 一貫的「不猜測」原則，同 bootstrap migration 的說明）。
UPDATE category_crop_mapping
SET status = 'unmapped', selection_kind = NULL, crop_code = NULL, group_code = NULL
WHERE status = 'resolved' AND group_code IS NULL;
