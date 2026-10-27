-- name: UpsertSurfaceForm :exec
-- 同一實體、同一正規化 form 已存在時不覆蓋 form_kind——先登記的較權威
-- （CANONICAL 先於 ALIAS，由 RegisterSurfaceForms 的寫入順序保證）。
INSERT INTO priceable_surface_form (entity_kind, entity_code, form_raw, form_kind)
VALUES (sqlc.arg(entity_kind), sqlc.arg(entity_code), sqlc.arg(form_raw), sqlc.arg(form_kind))
ON CONFLICT (entity_kind, entity_code, form_norm) DO NOTHING;

-- name: DeleteSurfaceFormsByKind :exec
-- 實體改名時先清掉舊的同 kind 列再重登記；ALIAS 不經此路（那是 seed／人工資料）。
DELETE FROM priceable_surface_form
WHERE entity_kind = sqlc.arg(entity_kind) AND entity_code = sqlc.arg(entity_code)
  AND form_kind = sqlc.arg(form_kind);

-- name: SearchSurfaceForms :many
-- tier：0 精確、1 前綴、2 子串。同 tier 內依 form_kind 權威度，再依 form_norm 定序。
-- 查詢參數的正規化與 schema.sql 的 generated column 用同一段運算式（見標記行，
-- surface_form_expr_test.go 斷言兩份逐字相同）。
-- 空查詢回零列：空字串由 usecase 走 ListAliasSurfaceForms，不在這裡特判。
WITH q AS (
    SELECT
        -- norm-expr-begin
        lower(translate(
            regexp_replace(sqlc.arg(query)::text, '[[:space:][:punct:]、。，！？：；（）「」『』【】《》〈〉—…·～]', '', 'g'),
            'ＡＢＣＤＥＦＧＨＩＪＫＬＭＮＯＰＱＲＳＴＵＶＷＸＹＺａｂｃｄｅｆｇｈｉｊｋｌｍｎｏｐｑｒｓｔｕｖｗｘｙｚ０１２３４５６７８９臺',
            'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789台'
        ))
        -- norm-expr-end
        AS norm
)
SELECT
    f.entity_kind, f.entity_code, f.form_raw, f.form_kind,
    (CASE WHEN f.form_norm = q.norm THEN 0
          WHEN f.form_norm LIKE q.norm || '%' THEN 1
          ELSE 2 END)::int AS tier,
    COALESCE(g.group_code, cg.group_code, '')::text AS group_code,
    COALESCE(g.name, cg.name, '')::text             AS group_name,
    COALESCE(c.crop_code, '')::text                 AS crop_code,
    COALESCE(c.name, '')::text                      AS crop_name,
    COALESCE(c.variant_name, '')::text              AS variant_name,
    COALESCE(m.name, '')::text                      AS commodity_name
FROM priceable_surface_form f
CROSS JOIN q
LEFT JOIN crop_group g  ON f.entity_kind = 'CROP_GROUP' AND g.group_code = f.entity_code
LEFT JOIN crop c        ON f.entity_kind = 'CROP'       AND c.crop_code  = f.entity_code
LEFT JOIN crop_group cg ON c.group_id = cg.id
LEFT JOIN commodity m   ON f.entity_kind = 'COMMODITY'  AND m.id::text   = f.entity_code
WHERE q.norm <> ''
  AND f.form_norm LIKE '%' || q.norm || '%'
  AND (g.id IS NULL OR g.active)
  AND (c.id IS NULL OR c.active)
  AND (m.id IS NULL OR m.active)
ORDER BY tier,
         CASE f.form_kind WHEN 'CANONICAL' THEN 0 WHEN 'SOURCE_TERM' THEN 1 WHEN 'VARIANT' THEN 2 ELSE 3 END,
         f.form_norm, f.entity_kind, f.entity_code
LIMIT sqlc.arg(row_limit);

-- name: ListAliasSurfaceForms :many
-- SearchCropCatalog 空查詢的「預設常見品項」：原本回 mapping.Seed 的 27 條，現在
-- 回 ALIAS 列（backfill 把那 27 條遷進來）。順序照 form_raw；proto 契約從未承諾順序。
SELECT f.entity_kind, f.entity_code, f.form_raw,
       COALESCE(g.group_code, cg.group_code, '')::text AS group_code,
       COALESCE(g.name, cg.name, '')::text             AS group_name,
       COALESCE(c.crop_code, '')::text                 AS crop_code,
       COALESCE(c.name, '')::text                      AS crop_name,
       COALESCE(c.variant_name, '')::text              AS variant_name
FROM priceable_surface_form f
LEFT JOIN crop_group g  ON f.entity_kind = 'CROP_GROUP' AND g.group_code = f.entity_code
LEFT JOIN crop c        ON f.entity_kind = 'CROP'       AND c.crop_code  = f.entity_code
LEFT JOIN crop_group cg ON c.group_id = cg.id
WHERE f.form_kind = 'ALIAS' AND f.entity_kind IN ('CROP_GROUP', 'CROP')
  AND (g.id IS NULL OR g.active)
  AND (c.id IS NULL OR c.active)
ORDER BY f.form_raw
LIMIT sqlc.arg(row_limit);

-- name: CategoryIDsByGroupCodes :many
SELECT group_code::text AS group_code, category_id FROM category_crop_mapping
WHERE status = 'resolved' AND group_code = ANY(sqlc.arg(group_codes)::text[]);

-- name: CategoryIDsByCropCodes :many
-- crop 的分類 = 它自己的 L3 收窄（category_crop_mapping_crop）∪ 所屬大類的分類。
SELECT c.crop_code, x.category_id
FROM crop c
JOIN category_crop_mapping_crop x ON x.crop_id = c.id
WHERE c.crop_code = ANY(sqlc.arg(crop_codes)::text[])
UNION
SELECT c.crop_code, m.category_id
FROM crop c
JOIN crop_group g ON g.id = c.group_id
JOIN category_crop_mapping m ON m.group_code = g.group_code AND m.status = 'resolved'
WHERE c.crop_code = ANY(sqlc.arg(crop_codes)::text[]);

-- name: CategoryIDsByCommodityIDs :many
SELECT commodity_id::bigint AS commodity_id, category_id FROM category_crop_mapping
WHERE status = 'resolved' AND commodity_id = ANY(sqlc.arg(commodity_ids)::bigint[]);
