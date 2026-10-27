-- name: InsertAgreementAcceptance :exec
-- 每次同意事件寫一列（沒有 ON CONFLICT）：開店的 SELLER_TERMS 每家店一列（agreement_acceptances_seller_terms），KYC 的兩份帶
-- 那次送出的 SUBMIT 審核紀錄，食品業者登錄的告知帶那次送出的登錄（agreement_acceptances_source_check：依種類恰好綁一個來源）。
INSERT INTO seller.agreement_acceptances (store_id, kind, version, account_id, accepted_at, submission_review_id, food_registration_id, client_ip, user_agent)
VALUES (sqlc.arg(store_id), sqlc.arg(kind), sqlc.arg(version), sqlc.arg(account_id), sqlc.arg(accepted_at), sqlc.narg(submission_review_id), sqlc.narg(food_registration_id),
        sqlc.narg(client_ip), sqlc.narg(user_agent));

-- name: PurgeAgreementSources :execrows
-- 個資清理：把這些店家的同意紀錄的來源 IP 與 User-Agent 清成 NULL 並記下 pii_purged_at；種類、版本、時間、帳號與送出保留。只作用於
-- 店家列已清理的店家（保留期限的判斷只有 PurgeStores 一處）。這是這張表唯一的 UPDATE。
UPDATE seller.agreement_acceptances a
SET client_ip = NULL,
    user_agent = NULL,
    pii_purged_at = sqlc.arg(now)
FROM seller.stores s
WHERE a.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND s.store_id = a.store_id AND s.pii_purged_at IS NOT NULL AND a.pii_purged_at IS NULL;
