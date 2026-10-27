-- name: GetApplicationByID :one
SELECT * FROM certification_applications WHERE id = $1;

-- name: GetApplicationByNo :one
SELECT * FROM certification_applications WHERE application_no = $1;

-- name: GetApplicationsBySeller :many
SELECT ca.* FROM certification_applications ca
JOIN sellers s ON s.account_id = ca.account_id AND s.is_current
WHERE ca.account_id = $1 AND ca.created_at >= s.created_at
ORDER BY ca.created_at DESC;

-- name: GetCurrentApplication :one
SELECT ca.* FROM certification_applications ca
INNER JOIN sellers s ON s.current_certification_id = ca.id
WHERE s.account_id = $1 AND s.is_current;

-- name: GetDraftApplication :one
SELECT ca.* FROM certification_applications ca
JOIN sellers s ON s.account_id = ca.account_id AND s.is_current
WHERE ca.account_id = $1 AND ca.status = 'draft' AND ca.created_at >= s.created_at
ORDER BY ca.created_at DESC
LIMIT 1;

-- name: CreateApplication :one
INSERT INTO certification_applications (
  id, account_id, application_no, status, seller_type,
  real_name, id_card_no, birth_date, id_card_issue_date, id_card_issue_type, id_card_issue_location, english_name, store_description,
  id_card_front_asset_id, id_card_back_asset_id, bank_book_asset_id,
  bank_name, bank_account,
  company_type, company_name, tax_id, company_register_date, company_city, company_district, company_address_detail, company_email, company_phone,
  product_type, is_processed_food, food_business_reg_no,
  document_asset_ids,
  current_step, total_steps, step_data,
  submitted_at, rejected_at, created_at, updated_at,
  bank_code
) VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8, $9, $10, $11, $12, $13,
  $14, $15, $16,
  $17, $18,
  $19, $20, $21, $22, $23, $24, $25, $26, $27,
  $28, $29, $30,
  $31,
  $32, $33, $34,
  $35, $36, $37, $38,
  $39
) RETURNING *;

-- name: UpdateApplication :execrows
UPDATE certification_applications SET
  status = $2,
  seller_type = $3,
  real_name = $4,
  id_card_no = $5,
  birth_date = $6,
  id_card_issue_date = $7,
  id_card_issue_type = $8,
  id_card_issue_location = $9,
  english_name = $10,
  id_card_front_asset_id = $11,
  id_card_back_asset_id = $12,
  bank_book_asset_id = $13,
  bank_name = $14,
  bank_account = $15,
  company_type = $16,
  company_name = $17,
  tax_id = $18,
  company_register_date = $19,
  company_city = $20,
  company_district = $21,
  company_address_detail = $22,
  company_email = $23,
  company_phone = $24,
  product_type = $25,
  is_processed_food = $26,
  food_business_reg_no = $27,
  document_asset_ids = $28,
  current_step = $29,
  total_steps = $30,
  step_data = $31,
  submitted_at = $32,
  forwarded_at = $33,
  rejected_at = $34,
  updated_at = $35,
  bank_code = $36,
  payuni_website_url = $37,
  cancel_reason = $38,
  version = version + 1
WHERE id = $1 AND version = $39;

-- name: ListCertificationsForAdmin :many
SELECT
  ca.id,
  ca.account_id,
  ca.application_no,
  ca.status,
  ca.seller_type,
  ca.real_name,
  ca.current_step,
  ca.total_steps,
  ca.submitted_at,
  ca.rejected_at,
  ca.created_at,
  ca.updated_at,
  m.store_name,
  ca.id_card_front_asset_id,
  ca.id_card_back_asset_id,
  ca.bank_book_asset_id,
  ca.document_asset_ids
FROM certification_applications ca
LEFT JOIN sellers m ON m.account_id = ca.account_id AND m.is_current
WHERE ($1 = '' OR ca.status = $1)
  AND ($2 = '' OR (
    ca.real_name ILIKE '%' || $2 || '%'
    OR ca.application_no ILIKE '%' || $2 || '%'
  ))
ORDER BY ca.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountCertificationsForAdmin :one
SELECT COUNT(*) FROM certification_applications ca
WHERE ($1 = '' OR ca.status = $1)
  AND ($2 = '' OR (
    ca.real_name ILIKE '%' || $2 || '%'
    OR ca.application_no ILIKE '%' || $2 || '%'
  ));

-- name: GetCertificationStatsForAdmin :one
SELECT
  COUNT(*) as total,
  COUNT(*) FILTER (WHERE status IN ('submitted', 'forward_failed')) as submitted,
  COUNT(*) FILTER (WHERE status = 'forwarded') as approved,
  COUNT(*) FILTER (WHERE status = 'rejected') as rejected
FROM certification_applications;

-- name: GetCertHolderNamesByStoreID :one
SELECT ca.real_name, ca.company_name
FROM certification_applications ca
INNER JOIN sellers s ON s.current_certification_id = ca.id
INNER JOIN payuni_registrations pr ON pr.application_id = ca.id
WHERE s.store_id = $1 AND s.is_current AND ca.status = 'forwarded'
  AND pr.registration_status = 'active';
