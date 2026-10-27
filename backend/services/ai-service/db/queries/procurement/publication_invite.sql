-- procurement_rfq_publication_invites 的持久層（#2450 Slice 2b）。

-- name: InsertProcurementRfqPublicationInvite :one
INSERT INTO procurement_rfq_publication_invites (
    id, publication_id, buyer_id, position, lane, store_id, prospect_id,
    delivery_state, delivery_state_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListProcurementRfqPublicationInvites :many
SELECT * FROM procurement_rfq_publication_invites
WHERE publication_id = $1
ORDER BY position;
