-- name: GetEnabledTemplates :many
SELECT id, exchange, routing_key, event_type, target_user_type, target_user_field,
       category, title_template, body_template, body_fallback, data_type, data_fields, schema_url,
       reference_id_template, image_key_template, payload_filter, enabled, push_mode, identity_migrated, updated_by, created_at, updated_at
FROM notification_templates
WHERE enabled = true
ORDER BY id;

-- name: GetAllTemplates :many
SELECT id, exchange, routing_key, event_type, target_user_type, target_user_field,
       category, title_template, body_template, body_fallback, data_type, data_fields, schema_url,
       reference_id_template, image_key_template, payload_filter, enabled, push_mode, identity_migrated, updated_by, created_at, updated_at
FROM notification_templates
ORDER BY id;

-- name: GetTemplateByID :one
SELECT id, exchange, routing_key, event_type, target_user_type, target_user_field,
       category, title_template, body_template, body_fallback, data_type, data_fields, schema_url,
       reference_id_template, image_key_template, payload_filter, enabled, push_mode, identity_migrated, updated_by, created_at, updated_at
FROM notification_templates
WHERE id = $1;

-- name: InsertTemplate :one
INSERT INTO notification_templates (
  id, exchange, routing_key, event_type, target_user_type, target_user_field,
  category, title_template, body_template, body_fallback, data_type, data_fields,
  schema_url, reference_id_template, image_key_template, payload_filter, enabled, push_mode, updated_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING id, created_at, updated_at;

-- name: UpdateTemplate :exec
UPDATE notification_templates
SET exchange = $2,
    routing_key = $3,
    event_type = $4,
    target_user_type = $5,
    target_user_field = $6,
    category = $7,
    title_template = $8,
    body_template = $9,
    body_fallback = $10,
    data_type = $11,
    data_fields = $12,
    schema_url = $13,
    reference_id_template = $14,
    image_key_template = $15,
    payload_filter = $16,
    enabled = $17,
    push_mode = $18,
    updated_by = $19,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: UpdateTemplateEnabled :exec
UPDATE notification_templates
SET enabled = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;
