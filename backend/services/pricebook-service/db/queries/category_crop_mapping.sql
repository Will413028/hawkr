-- name: GetCategoryCropMapping :one
SELECT * FROM category_crop_mapping WHERE category_id = sqlc.arg(category_id);

-- name: UpsertCategoryCropMapping :one
-- 人工治理入口（admin API）。這支的語意是「把這個分類指向某個 crop_group，
-- 或標成 unmapped/ambiguous」——它不設定 commodity 目標。
--
-- 🔴 DO UPDATE 必須把 commodity_id 一起清成 NULL，否則碰到 adapter 寫的
-- commodity 映射列時**任何** upsert 都會違反
-- category_crop_mapping_resolved_oneof_check：
--   改成 resolved+group_code → 兩個目標同時非空
--   改成 unmapped/ambiguous  → 非 resolved 卻留著 commodity_id
-- 兩種都是 500，admin 完全治理不了豬肉、雞蛋、米這些分類。
--
-- 清成 NULL 也是正確的語意而不只是為了過 CHECK：人工判斷本來就該壓過
-- adapter（declareCommodityCategories 用 ON CONFLICT DO NOTHING 正是為了
-- 不覆蓋人工決定）。人工把某個分類改指向 group 之後，adapter 下一輪也不會
-- 再把 commodity 填回去。
INSERT INTO category_crop_mapping (
    category_id, status, group_code, mapping_version, source_note, updated_by, updated_at
) VALUES (
    sqlc.arg(category_id), sqlc.arg(status), sqlc.narg(group_code),
    sqlc.arg(mapping_version), sqlc.narg(source_note), sqlc.arg(updated_by), now()
)
ON CONFLICT (category_id) DO UPDATE SET
    status          = EXCLUDED.status,
    group_code      = EXCLUDED.group_code,
    commodity_id    = NULL,
    mapping_version = EXCLUDED.mapping_version,
    source_note     = EXCLUDED.source_note,
    updated_by      = EXCLUDED.updated_by,
    updated_at      = now()
RETURNING *;

-- name: ListCategoryCropMappingsByStatuses :many
-- 待處理清單用（Task 9，issue #2786）：只回 status 落在指定清單裡的既有列。
SELECT * FROM category_crop_mapping
WHERE status = ANY(sqlc.arg(statuses)::text[])
ORDER BY category_id;

-- name: ListAllCategoryCropMappings :many
-- 待處理清單用：跟 category service 的分類樹做差集，找出完全查無列的 categoryId。
SELECT category_id FROM category_crop_mapping;

-- name: ListResolvedCategoryGroupMappings :many
-- category_price_rollup 每日批次用：列出每個 resolved L3 categoryId 對應的
-- group_code，只取目前仍 active 的 group——已停用的 group 應該讓對應
-- categoryId 的 rollup 落到 unavailable（同 ResolveCategoryMapping usecase
-- 的 fail-closed 邏輯，兩個端點對同一個 categoryId 的判斷不該互相矛盾）。
SELECT ccm.category_id, ccm.group_code FROM category_crop_mapping ccm
JOIN crop_group cg ON cg.group_code = ccm.group_code AND cg.active
WHERE ccm.status = 'resolved'
ORDER BY ccm.group_code, ccm.category_id;

-- name: FindActiveCropGroupByCode :one
-- resolve-time fail-closed 驗證用：只回目前 active 的 crop_group，查無代表
-- 已停用或不存在，呼叫端要當 unmapped 處理，不是回錯誤。
SELECT group_code, name FROM crop_group WHERE group_code = sqlc.arg(group_code) AND active;

-- name: ListResolvedCategoryCommodityMappings :many
-- category_price_rollup 每日批次用：列出每個 resolved L3 categoryId 對應的
-- commodity_id，只取目前仍 active 的 commodity。
--
-- 與 ListResolvedCategoryGroupMappings 是**互補**的兩半而不是二選一：
-- resolved 的目標恰好是 group_code 或 commodity_id 其中之一（DB CHECK 強制），
-- 那支 INNER JOIN crop_group 天然濾掉 commodity 列，這支反之。rollup 必須兩支
-- 都跑，否則毛豬、家禽、蛋品、糧價對應的 L3 永遠算不出價格——而且不會有任何
-- 錯誤訊息，walkNode 把「缺席的 L3」當成 unavailable 處理。
--
-- 已停用的 commodity 讓對應 categoryId 的 rollup 落到 unavailable，
-- 與 group 那支、與 ResolveCategoryMapping usecase 的 fail-closed 一致。
SELECT ccm.category_id, ccm.commodity_id
FROM category_crop_mapping ccm
JOIN commodity c ON c.id = ccm.commodity_id AND c.active
WHERE ccm.status = 'resolved'
ORDER BY ccm.commodity_id, ccm.category_id;

-- name: ListResolvedCategoryCropMappings :many
SELECT ccm.category_id, c.crop_code
FROM category_crop_mapping ccm
JOIN category_crop_mapping_crop m ON m.category_id = ccm.category_id
JOIN crop c ON c.id = m.crop_id AND c.active
WHERE ccm.status = 'resolved'
ORDER BY ccm.category_id, c.crop_code;

-- name: ListCategoryCropMappingCrops :many
SELECT c.crop_code, c.name, c.variant_name
FROM category_crop_mapping_crop m
JOIN crop c ON c.id = m.crop_id AND c.active
WHERE m.category_id = sqlc.arg(category_id)
ORDER BY c.crop_code;

-- name: DeleteCategoryCropMappingCrops :exec
DELETE FROM category_crop_mapping_crop WHERE category_id = sqlc.arg(category_id);

-- name: InsertCategoryCropMappingCrop :exec
INSERT INTO category_crop_mapping_crop (category_id, crop_id)
SELECT sqlc.arg(category_id), c.id FROM crop c WHERE c.crop_code = sqlc.arg(crop_code)
ON CONFLICT (category_id, crop_id) DO NOTHING;
