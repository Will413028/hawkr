
-- name: ListCategories :many
SELECT id, parent_id, name, shopify_taxonomy_id, position FROM catalog.categories ORDER BY parent_id NULLS FIRST, position, id;

-- name: GetCategory :one
SELECT id, parent_id, name, shopify_taxonomy_id, position FROM catalog.categories WHERE id = sqlc.arg(id);

-- name: IsLeafCategory :one
SELECT EXISTS (
    SELECT 1 FROM catalog.categories c
    WHERE c.id = sqlc.arg(id) AND c.parent_id IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM catalog.categories ch WHERE ch.parent_id = c.id));
