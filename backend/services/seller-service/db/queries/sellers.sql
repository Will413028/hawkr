-- name: GetSellerByAccountID :one
SELECT * FROM sellers WHERE account_id = $1 AND is_current AND deleted_at IS NULL;

-- name: GetCurrentSellerByStoreID :one
SELECT * FROM sellers WHERE store_id = $1 AND is_current AND deleted_at IS NULL;

-- name: GetSellerProfileByAccountID :one
SELECT m.account_id, m.store_id, m.store_code, m.store_name, m.description, m.logo, m.cover_key,
       m.email, m.contact_phone, m.city, m.district, m.address_detail, m.postal_code,
       m.birthday, m.gender, m.prize, m.rating,
       m.current_certification_id, m.seller_type, m.onboarding_completed, m.certified_at,
       m.created_at, m.updated_at, m.deleted_at, m.status,
       m.pii_cleared_at, m.payuni_account_unlinked_at, m.seo_title, m.seo_description,
       m.website_url,
       m.is_processed_food, m.food_business_reg_no, m.version,
       COALESCE(c.status, 'not_started') AS cert_status,
       pr.approved_at AS payuni_approved_at,
       COALESCE(c.seller_type, '') AS cert_seller_type,
       COALESCE(pr.mer_status, '') AS payuni_mer_status,
       COALESCE(pr.registration_status, '') AS payuni_registration_status,
       COALESCE(pr.member_status, '') AS payuni_member_status
FROM sellers m
LEFT JOIN certification_applications c ON c.id = m.current_certification_id
LEFT JOIN payuni_registrations pr ON pr.store_id = m.store_id
WHERE m.account_id = $1 AND m.is_current AND m.deleted_at IS NULL;

-- name: GetPublicSellerProfileByStoreIDOrCode :one
SELECT m.account_id, m.store_id, m.store_code, m.store_name, m.description, m.logo, m.cover_key,
       m.email, m.contact_phone, m.city, m.district, m.address_detail, m.postal_code,
       m.birthday, m.gender, m.prize, m.rating,
       m.current_certification_id, m.seller_type, m.onboarding_completed, m.certified_at,
       m.created_at, m.updated_at, m.deleted_at, m.status,
       m.pii_cleared_at, m.payuni_account_unlinked_at, m.seo_title, m.seo_description,
       m.website_url,
       m.is_processed_food, m.food_business_reg_no, m.version,
       COALESCE(c.status, 'not_started') AS cert_status,
       pr.approved_at AS payuni_approved_at,
       COALESCE(c.seller_type, '') AS cert_seller_type,
       COALESCE(pr.mer_status, '') AS payuni_mer_status,
       COALESCE(pr.registration_status, '') AS payuni_registration_status,
       COALESCE(pr.member_status, '') AS payuni_member_status
FROM sellers m
LEFT JOIN certification_applications c ON c.id = m.current_certification_id
LEFT JOIN payuni_registrations pr ON pr.store_id = m.store_id
WHERE (m.store_id = $1 OR m.store_code = $1) AND m.deleted_at IS NULL;

-- name: GetSellerAccountLinkage :one
SELECT account_id FROM sellers WHERE store_id = $1;

-- name: GetSellerProfileByStoreID :one
SELECT m.account_id, m.store_id, m.store_code, m.store_name, m.description, m.logo, m.cover_key,
       m.email, m.contact_phone, m.city, m.district, m.address_detail, m.postal_code,
       m.birthday, m.gender, m.prize, m.rating,
       m.current_certification_id, m.seller_type, m.onboarding_completed, m.certified_at,
       m.created_at, m.updated_at, m.deleted_at, m.status,
       m.pii_cleared_at, m.payuni_account_unlinked_at, m.seo_title, m.seo_description,
       m.website_url,
       m.is_processed_food, m.food_business_reg_no, m.version,
       COALESCE(c.status, 'not_started') AS cert_status,
       pr.approved_at AS payuni_approved_at,
       COALESCE(c.seller_type, '') AS cert_seller_type,
       COALESCE(pr.mer_status, '') AS payuni_mer_status,
       COALESCE(pr.registration_status, '') AS payuni_registration_status,
       COALESCE(pr.member_status, '') AS payuni_member_status
FROM sellers m
LEFT JOIN certification_applications c ON c.id = m.current_certification_id
LEFT JOIN payuni_registrations pr ON pr.store_id = m.store_id
WHERE m.store_id = $1;

-- name: CreateSeller :one
INSERT INTO sellers (
  account_id, store_name, description, logo, cover_key,
  email, contact_phone, city, district, address_detail, postal_code,
  birthday, gender, prize, rating,
  seller_type,
  created_at, updated_at, store_id, store_code, is_current
) VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8, $9, $10, $11,
  $12, $13, $14, $15,
  $16,
  $17, $18, $19, $20, true
) RETURNING *;

-- name: UpdateSeller :execrows
UPDATE sellers SET
  store_name = $2,
  description = $3,
  logo = $4,
  cover_key = $5,
  contact_phone = $6,
  city = $7,
  district = $8,
  address_detail = $9,
  postal_code = $10,
  birthday = $11,
  gender = $12,
  prize = $13,
  rating = $14,
  website_url = $15,
  updated_at = $16,
  version = version + 1
WHERE store_id = $1 AND is_current AND deleted_at IS NULL
  AND (sqlc.arg(expected_version)::bigint < 0 OR version = sqlc.arg(expected_version)::bigint);

-- name: ListSellersForAdmin :many
SELECT m.account_id, m.store_id, m.store_name, m.store_code, m.description, m.logo, m.cover_key,
       m.contact_phone, m.email, m.city, m.district, m.address_detail, m.postal_code,
       m.rating, m.seller_type, m.current_certification_id, m.certified_at,
       m.created_at, m.updated_at, m.seo_title, m.seo_description, m.status,
       COALESCE(c.status, 'not_started') AS cert_status,
       pr.approved_at AS payuni_approved_at,
       COALESCE(c.seller_type, '') AS cert_seller_type,
       COALESCE(pr.mer_status, '') AS payuni_mer_status,
       COALESCE(pr.member_status, '') AS payuni_member_status
FROM sellers m
LEFT JOIN certification_applications c ON c.id = m.current_certification_id
LEFT JOIN payuni_registrations pr
  ON pr.store_id = m.store_id AND pr.application_id = m.current_certification_id
WHERE (cardinality($1::text[]) = 0 OR COALESCE(c.status, 'not_started') = ANY($1::text[]))
  AND ($2 = '' OR (
    m.store_name ILIKE '%' || $2 || '%'
    OR m.contact_phone ILIKE '%' || $2 || '%'
    OR m.email ILIKE '%' || $2 || '%'
  ))
  AND ($5 = '' OR m.status = $5)
  AND m.is_current AND m.deleted_at IS NULL
ORDER BY m.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountSellersForAdmin :one
SELECT COUNT(*)
FROM sellers m
LEFT JOIN certification_applications c ON c.id = m.current_certification_id
WHERE (cardinality($1::text[]) = 0 OR COALESCE(c.status, 'not_started') = ANY($1::text[]))
  AND ($2 = '' OR (
    m.store_name ILIKE '%' || $2 || '%'
    OR m.contact_phone ILIKE '%' || $2 || '%'
    OR m.email ILIKE '%' || $2 || '%'
  ))
  AND ($3 = '' OR m.status = $3)
  AND m.is_current AND m.deleted_at IS NULL;

-- name: GetSellerStatsForAdmin :one
SELECT
  COUNT(*)                                                                          AS total,
  COUNT(*) FILTER (WHERE c.status IN ('submitted', 'forward_failed'))               AS pending,
  COUNT(*) FILTER (WHERE c.status = 'forwarded')                                    AS approved,
  COUNT(*) FILTER (WHERE c.status = 'rejected')                                     AS rejected,
  COUNT(*) FILTER (WHERE DATE_TRUNC('month', m.created_at) = DATE_TRUNC('month', CURRENT_TIMESTAMP)) AS monthly_new
FROM sellers m
LEFT JOIN certification_applications c ON c.id = m.current_certification_id
WHERE m.is_current AND m.deleted_at IS NULL;

-- name: UpdateCurrentCertificationID :exec
UPDATE sellers SET
  current_certification_id = $2,
  updated_at = $3,
  version = version + 1
WHERE account_id = $1 AND is_current AND deleted_at IS NULL;

-- name: UpdateSellerLogo :execrows
UPDATE sellers SET
  logo = $2,
  updated_at = $3,
  version = version + 1
WHERE store_id = $1 AND is_current AND deleted_at IS NULL
  AND (sqlc.arg(expected_version)::bigint < 0 OR version = sqlc.arg(expected_version)::bigint);

-- name: UpdateSellerCover :execrows
UPDATE sellers SET
  cover_key = $2,
  updated_at = $3,
  version = version + 1
WHERE store_id = $1 AND is_current AND deleted_at IS NULL
  AND (sqlc.arg(expected_version)::bigint < 0 OR version = sqlc.arg(expected_version)::bigint);

-- name: CompleteOnboarding :exec
UPDATE sellers SET onboarding_completed = true, updated_at = $2, version = version + 1
WHERE account_id = $1 AND is_current AND deleted_at IS NULL;

-- name: UpdateSellerSEO :exec
UPDATE sellers SET
  seo_title = $2,
  seo_description = $3,
  updated_at = $4,
  version = version + 1
WHERE account_id = $1 AND is_current AND deleted_at IS NULL;

-- name: UpdateSellerRatingByStoreID :exec
UPDATE sellers SET
  rating = $2,
  updated_at = $3
WHERE store_id = $1 AND is_current AND deleted_at IS NULL;

-- name: UpdateSellerFoodBusinessRegistration :exec
UPDATE sellers SET
  is_processed_food = $2,
  food_business_reg_no = $3,
  updated_at = $4,
  version = version + 1
WHERE account_id = $1 AND is_current AND deleted_at IS NULL;

-- name: GetSellerByAccountIDForUpdate :one
SELECT * FROM sellers WHERE account_id = $1 AND is_current FOR UPDATE;

-- name: RetireCurrentSellerStint :exec
UPDATE sellers SET is_current = false, updated_at = $2
WHERE account_id = $1 AND is_current;

-- name: UpdateSellerStatusByStoreID :exec
UPDATE sellers SET
  status = $2,
  payuni_account_unlinked_at = $3,
  deleted_at = $4,
  updated_at = $5,
  version = version + 1
WHERE store_id = $1;

-- name: UpdateSellerStatusCurrentByAccountID :exec
UPDATE sellers SET status = $2, updated_at = $3, version = version + 1
WHERE account_id = $1 AND is_current;

-- name: SetSellerPIICleared :exec
UPDATE sellers SET
  store_name    = '已刪除帳號',
  contact_phone = '',
  email         = '',
  logo          = '',
  pii_cleared_at = $2,
  updated_at     = $2
WHERE store_id = $1 AND pii_cleared_at IS NULL;

-- name: ListSellersPendingPIICleanup :many
SELECT store_id FROM sellers
WHERE deleted_at < $1 AND pii_cleared_at IS NULL
LIMIT $2;
