-- name: CreateProspect :one
INSERT INTO seller.prospects (
    display_name, vendor_type, outreach_status, tax_id,
    category, region, product_summary, positioning, research_summary, priority_letter, priority_stars,
    created_by_admin_id, updated_by_admin_id, created_at, updated_at
)
VALUES (
    sqlc.arg(display_name), sqlc.arg(vendor_type), sqlc.arg(outreach_status), sqlc.narg(tax_id),
    sqlc.narg(category), sqlc.narg(region), sqlc.narg(product_summary), sqlc.narg(positioning), sqlc.narg(research_summary),
    sqlc.narg(priority_letter), sqlc.arg(priority_stars),
    sqlc.arg(admin_id), sqlc.arg(admin_id), sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: ProspectByID :one
SELECT * FROM seller.prospects WHERE prospect_id = $1;

-- name: ProspectContactForShare :one
SELECT p.prospect_id, p.display_name, p.status, e.value AS primary_email
FROM seller.prospects p
LEFT JOIN seller.prospect_contacts e ON e.prospect_id = p.prospect_id AND e.kind = 'EMAIL' AND e.is_primary
WHERE p.prospect_id = $1
FOR SHARE OF p;

-- name: LockProspects :many
SELECT * FROM seller.prospects WHERE prospect_id = ANY(sqlc.arg(prospect_ids)::bigint[]) ORDER BY prospect_id FOR NO KEY UPDATE;

-- name: UpdateProspect :one
UPDATE seller.prospects
SET display_name = sqlc.arg(display_name),
    vendor_type = sqlc.arg(vendor_type),
    outreach_status = sqlc.arg(outreach_status),
    tax_id = sqlc.narg(tax_id),
    category = sqlc.narg(category),
    region = sqlc.narg(region),
    product_summary = sqlc.narg(product_summary),
    positioning = sqlc.narg(positioning),
    research_summary = sqlc.narg(research_summary),
    priority_letter = sqlc.narg(priority_letter),
    priority_stars = sqlc.arg(priority_stars),
    logo_key = CASE WHEN sqlc.arg(set_logo)::boolean THEN sqlc.narg(logo_key)::text ELSE logo_key END,
    logo_asset_id = CASE WHEN sqlc.arg(set_logo)::boolean THEN sqlc.narg(logo_asset_id)::uuid ELSE logo_asset_id END,
    logo_source_url = CASE WHEN sqlc.arg(set_logo)::boolean THEN sqlc.narg(logo_source_url)::text ELSE logo_source_url END,
    cover_key = CASE WHEN sqlc.arg(set_cover)::boolean THEN sqlc.narg(cover_key)::text ELSE cover_key END,
    cover_asset_id = CASE WHEN sqlc.arg(set_cover)::boolean THEN sqlc.narg(cover_asset_id)::uuid ELSE cover_asset_id END,
    cover_source_url = CASE WHEN sqlc.arg(set_cover)::boolean THEN sqlc.narg(cover_source_url)::text ELSE cover_source_url END,
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id) AND version = sqlc.arg(expected_version) AND status = 'ACTIVE'
RETURNING *;

-- name: ArchiveProspect :one
UPDATE seller.prospects
SET status = 'ARCHIVED',
    logo_key = NULL, logo_asset_id = NULL, logo_source_url = NULL,
    cover_key = NULL, cover_asset_id = NULL, cover_source_url = NULL,
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id) AND status = 'ACTIVE'
RETURNING *;

-- name: RestoreProspect :one
UPDATE seller.prospects
SET status = 'ACTIVE',
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id) AND status = 'ARCHIVED' AND merged_into IS NULL
RETURNING *;

-- name: ConvertProspect :one
UPDATE seller.prospects
SET status = 'CONVERTED',
    outreach_status = 'CONVERTED',
    converted_store_id = sqlc.arg(store_id),
    converted_at = sqlc.arg(now),
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id) AND status = 'ACTIVE'
RETURNING *;

-- name: ProspectContacts :many
SELECT kind, value, is_primary FROM seller.prospect_contacts WHERE prospect_id = $1 ORDER BY kind, is_primary DESC, value;

-- name: ClearProspectContacts :exec
DELETE FROM seller.prospect_contacts WHERE prospect_id = $1;

-- name: AddProspectContacts :exec
INSERT INTO seller.prospect_contacts (prospect_id, kind, value, match_key, is_primary)
SELECT sqlc.arg(prospect_id)::bigint, (sqlc.arg(kinds)::text[])[i], (sqlc.arg(values)::text[])[i],
       NULLIF((sqlc.arg(match_keys)::text[])[i], ''), (sqlc.arg(primaries)::boolean[])[i]
FROM generate_series(1, cardinality(sqlc.arg(kinds)::text[])) AS i;

-- name: UnionProspectContacts :exec
INSERT INTO seller.prospect_contacts (prospect_id, kind, value, match_key, is_primary)
SELECT sqlc.arg(winner_id)::bigint, c.kind, c.value, c.match_key,
       c.is_primary AND NOT EXISTS (SELECT 1 FROM seller.prospect_contacts w WHERE w.prospect_id = sqlc.arg(winner_id)::bigint AND w.kind = c.kind)
FROM seller.prospect_contacts c
WHERE c.prospect_id = sqlc.arg(loser_id)::bigint
  AND NOT EXISTS (
      SELECT 1 FROM seller.prospect_contacts w
      WHERE w.prospect_id = sqlc.arg(winner_id)::bigint
        AND ((c.match_key IS NOT NULL AND w.match_key = c.match_key) OR (w.kind = c.kind AND w.value = c.value)));

-- name: EnsureProspectPrimaries :exec
UPDATE seller.prospect_contacts c
SET is_primary = true
FROM (
    SELECT DISTINCT ON (p.kind) p.kind, p.value
    FROM seller.prospect_contacts p
    WHERE p.prospect_id = $1
      AND NOT EXISTS (SELECT 1 FROM seller.prospect_contacts q WHERE q.prospect_id = $1 AND q.kind = p.kind AND q.is_primary)
    ORDER BY p.kind, p.value
) pick
WHERE c.prospect_id = $1 AND c.kind = pick.kind AND c.value = pick.value;

-- name: SharedKeyHolders :many
SELECT DISTINCT c.prospect_id, c.match_key::text AS match_key
FROM seller.prospect_contacts c
JOIN seller.prospects p ON p.prospect_id = c.prospect_id AND p.live
WHERE c.prospect_id <> sqlc.arg(self_id)::bigint
  AND c.match_key = ANY(sqlc.arg(keys)::text[])
  AND NOT EXISTS (
      SELECT 1 FROM seller.prospect_distinct_pairs d
      WHERE d.low_prospect_id = LEAST(c.prospect_id, sqlc.arg(self_id)::bigint) AND d.high_prospect_id = GREATEST(c.prospect_id, sqlc.arg(self_id)::bigint))
ORDER BY c.prospect_id, match_key;

-- name: ConfirmDistinctProspects :exec
INSERT INTO seller.prospect_distinct_pairs (low_prospect_id, high_prospect_id, confirmed_by_admin_id, confirmed_at)
SELECT LEAST(sqlc.arg(prospect_id)::bigint, o.id), GREATEST(sqlc.arg(prospect_id)::bigint, o.id), sqlc.arg(admin_id), sqlc.arg(now)
FROM unnest(sqlc.arg(others)::bigint[]) AS o (id)
ON CONFLICT DO NOTHING;

-- name: MarkDistinctPair :exec
INSERT INTO seller.prospect_distinct_pairs (low_prospect_id, high_prospect_id, confirmed_by_admin_id, confirmed_at)
VALUES (LEAST(sqlc.arg(first_id)::bigint, sqlc.arg(second_id)::bigint), GREATEST(sqlc.arg(first_id)::bigint, sqlc.arg(second_id)::bigint), sqlc.arg(admin_id), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: RepointDistinctPairs :exec
INSERT INTO seller.prospect_distinct_pairs (low_prospect_id, high_prospect_id, confirmed_by_admin_id, confirmed_at)
SELECT LEAST(sqlc.arg(winner_id)::bigint, m.other_id), GREATEST(sqlc.arg(winner_id)::bigint, m.other_id), m.confirmed_by_admin_id, m.confirmed_at
FROM (
    SELECT CASE WHEN low_prospect_id = sqlc.arg(loser_id)::bigint THEN high_prospect_id ELSE low_prospect_id END AS other_id,
           confirmed_by_admin_id, confirmed_at
    FROM seller.prospect_distinct_pairs
    WHERE low_prospect_id = sqlc.arg(loser_id)::bigint OR high_prospect_id = sqlc.arg(loser_id)::bigint
) m
WHERE m.other_id <> sqlc.arg(winner_id)::bigint
ON CONFLICT DO NOTHING;

-- name: DeleteDistinctPairsOf :exec
DELETE FROM seller.prospect_distinct_pairs WHERE low_prospect_id = $1 OR high_prospect_id = $1;

-- name: ProspectCategories :many
SELECT category_id FROM seller.prospect_categories WHERE prospect_id = $1 ORDER BY category_id;

-- name: ClearProspectCategories :exec
DELETE FROM seller.prospect_categories WHERE prospect_id = $1;

-- name: AddProspectCategories :exec
INSERT INTO seller.prospect_categories (prospect_id, category_id)
SELECT sqlc.arg(prospect_id), unnest(sqlc.arg(category_ids)::text[])
ON CONFLICT DO NOTHING;

-- name: CopyProspectCategories :exec
INSERT INTO seller.prospect_categories (prospect_id, category_id)
SELECT sqlc.arg(winner_id), c.category_id FROM seller.prospect_categories c WHERE c.prospect_id = sqlc.arg(loser_id)
ON CONFLICT DO NOTHING;

-- name: RepointMergedProspects :exec
UPDATE seller.prospects SET merged_into = sqlc.arg(winner_id) WHERE merged_into = sqlc.arg(loser_id);

-- name: ArchiveMergedLoser :one
UPDATE seller.prospects
SET status = 'ARCHIVED',
    merged_into = sqlc.arg(winner_id),
    tax_id = NULL,
    logo_key = NULL, logo_asset_id = NULL, logo_source_url = NULL,
    cover_key = NULL, cover_asset_id = NULL, cover_source_url = NULL,
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(loser_id) AND status = 'ACTIVE'
RETURNING *;

-- name: CarryProspectTaxID :one
UPDATE seller.prospects
SET tax_id = sqlc.arg(tax_id),
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(winner_id) AND tax_id IS NULL
RETURNING *;

-- name: TouchProspect :one
UPDATE seller.prospects
SET updated_by_admin_id = sqlc.arg(admin_id), updated_at = sqlc.arg(now), version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id)
RETURNING *;

-- name: ListProspects :many
SELECT *
FROM seller.prospects
WHERE (CASE WHEN sqlc.narg(status)::text IS NULL THEN status = 'ACTIVE' ELSE status = sqlc.narg(status)::text END)
  AND (sqlc.narg(outreach_status)::text IS NULL OR outreach_status = sqlc.narg(outreach_status)::text)
  AND (sqlc.narg(vendor_type)::text IS NULL OR vendor_type = sqlc.narg(vendor_type)::text)
  AND (sqlc.narg(category)::text IS NULL OR category = sqlc.narg(category)::text)
  AND (sqlc.narg(region)::text IS NULL OR region = sqlc.narg(region)::text)
  AND (sqlc.narg(keyword_pattern)::text IS NULL OR
       display_name ILIKE sqlc.narg(keyword_pattern)::text ESCAPE '\' OR
       product_summary ILIKE sqlc.narg(keyword_pattern)::text ESCAPE '\' OR
       positioning ILIKE sqlc.narg(keyword_pattern)::text ESCAPE '\' OR
       research_summary ILIKE sqlc.narg(keyword_pattern)::text ESCAPE '\')
  AND (
    sqlc.narg(after_updated_at)::timestamptz IS NULL OR
    (updated_at, prospect_id) < (sqlc.narg(after_updated_at)::timestamptz, sqlc.narg(after_id)::bigint)
  )
ORDER BY updated_at DESC, prospect_id DESC
LIMIT sqlc.arg(page_size);

-- name: ProspectsByName :many
SELECT * FROM seller.prospects
WHERE status = 'ACTIVE' AND lower(display_name) = lower(sqlc.arg(display_name)::text)
ORDER BY prospect_id
LIMIT 20;

-- name: DuplicateProspectPairs :many
WITH reasons AS (
    SELECT a.prospect_id AS left_id, b.prospect_id AS right_id, 'NAME'::text AS reason
    FROM seller.prospects a
    JOIN seller.prospects b ON a.prospect_id < b.prospect_id AND lower(a.display_name) = lower(b.display_name)
    WHERE a.status = 'ACTIVE' AND b.status = 'ACTIVE'
    UNION
    SELECT ca.prospect_id, cb.prospect_id, split_part(ca.match_key, ':', 1)
    FROM seller.prospect_contacts ca
    JOIN seller.prospect_contacts cb ON cb.match_key = ca.match_key AND cb.prospect_id > ca.prospect_id
    JOIN seller.prospects pa ON pa.prospect_id = ca.prospect_id AND pa.status = 'ACTIVE'
    JOIN seller.prospects pb ON pb.prospect_id = cb.prospect_id AND pb.status = 'ACTIVE'
    WHERE ca.match_key IS NOT NULL
), candidates AS (
    SELECT left_id, right_id, array_agg(reason ORDER BY reason)::text[] AS matched_on
    FROM reasons
    GROUP BY left_id, right_id
)
SELECT sqlc.embed(a), sqlc.embed(b), c.matched_on
FROM candidates c
JOIN seller.prospects a ON a.prospect_id = c.left_id
JOIN seller.prospects b ON b.prospect_id = c.right_id
WHERE NOT EXISTS (
    SELECT 1 FROM seller.prospect_distinct_pairs d WHERE d.low_prospect_id = c.left_id AND d.high_prospect_id = c.right_id)
  AND (
    sqlc.narg(after_left_id)::bigint IS NULL OR
    (c.left_id, c.right_id) > (sqlc.narg(after_left_id)::bigint, sqlc.narg(after_right_id)::bigint)
  )
ORDER BY c.left_id, c.right_id
LIMIT sqlc.arg(page_size);

-- name: SearchProspects :many
SELECT
    sqlc.embed(p),
    COALESCE(sqlc.narg(category)::text IS NOT NULL AND p.category ILIKE sqlc.narg(category)::text ESCAPE '\', false)::boolean AS hit_category,
    COALESCE(sqlc.narg(region)::text IS NOT NULL AND p.region ILIKE sqlc.narg(region)::text ESCAPE '\', false)::boolean AS hit_region,
    COALESCE(sqlc.narg(keywords)::text IS NOT NULL AND (
        p.product_summary ILIKE sqlc.narg(keywords)::text ESCAPE '\' OR
        p.positioning ILIKE sqlc.narg(keywords)::text ESCAPE '\' OR
        p.research_summary ILIKE sqlc.narg(keywords)::text ESCAPE '\'
    ), false)::boolean AS hit_keywords,
    (cardinality(sqlc.arg(category_ids)::text[]) > 0 AND EXISTS (
        SELECT 1 FROM seller.prospect_categories c
        WHERE c.prospect_id = p.prospect_id AND c.category_id = ANY(sqlc.arg(category_ids)::text[])
    ))::boolean AS hit_category_ids,
    w.value AS primary_website
FROM seller.prospects p
LEFT JOIN seller.prospect_contacts w ON w.prospect_id = p.prospect_id AND w.kind = 'WEBSITE' AND w.is_primary
WHERE p.status = 'ACTIVE'
  AND (
    (sqlc.narg(category)::text IS NULL AND sqlc.narg(region)::text IS NULL AND sqlc.narg(keywords)::text IS NULL AND
     cardinality(sqlc.arg(category_ids)::text[]) = 0)
    OR (sqlc.narg(category)::text IS NOT NULL AND p.category ILIKE sqlc.narg(category)::text ESCAPE '\')
    OR (sqlc.narg(region)::text IS NOT NULL AND p.region ILIKE sqlc.narg(region)::text ESCAPE '\')
    OR (sqlc.narg(keywords)::text IS NOT NULL AND (
        p.product_summary ILIKE sqlc.narg(keywords)::text ESCAPE '\' OR
        p.positioning ILIKE sqlc.narg(keywords)::text ESCAPE '\' OR
        p.research_summary ILIKE sqlc.narg(keywords)::text ESCAPE '\'
    ))
    OR (cardinality(sqlc.arg(category_ids)::text[]) > 0 AND EXISTS (
        SELECT 1 FROM seller.prospect_categories c
        WHERE c.prospect_id = p.prospect_id AND c.category_id = ANY(sqlc.arg(category_ids)::text[])
    ))
  )
LIMIT sqlc.arg(row_limit);

-- name: CurrentStoreExists :one
SELECT EXISTS (SELECT 1 FROM seller.stores WHERE store_id = $1 AND status <> 'CLOSED');

-- name: ProspectTaxIDMatches :many
SELECT * FROM (
    SELECT DISTINCT ON (p.prospect_id, s.store_id)
           p.prospect_id, s.store_id, s.store_name, k.company_name, k.status AS kyc_status
    FROM seller.prospects p
    JOIN seller.kyc_applications k ON k.tax_id = p.tax_id
    JOIN seller.stores s ON s.store_id = k.store_id AND s.status <> 'CLOSED'
    WHERE p.tax_id IS NOT NULL AND p.status = 'ACTIVE'
      AND NOT EXISTS (SELECT 1 FROM seller.prospects claimed WHERE claimed.converted_store_id = s.store_id)
    ORDER BY p.prospect_id, s.store_id, k.created_at DESC
) m
WHERE sqlc.narg(after_prospect_id)::bigint IS NULL OR (m.prospect_id, m.store_id) > (sqlc.narg(after_prospect_id)::bigint, sqlc.narg(after_store_id)::bigint)
ORDER BY m.prospect_id, m.store_id
LIMIT sqlc.arg(page_size);

-- name: LiveProspectByTaxID :one
SELECT prospect_id, status FROM seller.prospects WHERE tax_id = $1 AND live;

-- name: LockProspectTaxID :exec
SELECT pg_advisory_xact_lock(hashtextextended('seller.prospect_tax_id:' || sqlc.arg(tax_id)::text, 0));

-- name: ProspectsByIDs :many
SELECT * FROM seller.prospects WHERE prospect_id = ANY(sqlc.arg(prospect_ids)::bigint[]) ORDER BY prospect_id;
