-- name: UpsertVendorPortalIdentity :one
INSERT INTO vendor_portal_identities (
  id, prospect_id, invited_email
) VALUES ($1, $2, $3)
ON CONFLICT (prospect_id) DO UPDATE SET
  invited_email = EXCLUDED.invited_email,
  updated_at    = now()
RETURNING *;

-- name: GetSellerProspectPublicEmail :one
SELECT public_email FROM seller_prospects WHERE id = $1;

-- name: GetVendorPortalInviteByRFQRecipient :one
SELECT * FROM vendor_portal_invites WHERE rfq_recipient_id = $1;

-- name: InsertVendorPortalInvite :one
INSERT INTO vendor_portal_invites (
  id, identity_id, rfq_recipient_id, token_hash, expires_at
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: RotateVendorPortalInviteSecret :one
UPDATE vendor_portal_invites
   SET token_hash = $2, expires_at = $3
 WHERE id = $1
   AND secret_delivered_at IS NULL
   AND consumed_at IS NULL
   AND revoked_at IS NULL
RETURNING *;

-- name: MarkVendorPortalInviteSecretDelivered :execrows
UPDATE vendor_portal_invites
   SET secret_delivered_at = $2
 WHERE id = $1 AND secret_delivered_at IS NULL;
