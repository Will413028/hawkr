-- name: InsertAgreementAcceptance :exec
INSERT INTO seller.agreement_acceptances (store_id, kind, version, account_id, accepted_at, submission_review_id, client_ip, user_agent)
VALUES (sqlc.arg(store_id), sqlc.arg(kind), sqlc.arg(version), sqlc.arg(account_id), sqlc.arg(accepted_at), sqlc.narg(submission_review_id),
        sqlc.narg(client_ip), sqlc.narg(user_agent));

-- name: PurgeAgreementSources :execrows
UPDATE seller.agreement_acceptances a
SET client_ip = NULL,
    user_agent = NULL,
    pii_purged_at = sqlc.arg(now)
FROM seller.stores s
WHERE a.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND s.store_id = a.store_id AND s.pii_purged_at IS NOT NULL AND a.pii_purged_at IS NULL;
