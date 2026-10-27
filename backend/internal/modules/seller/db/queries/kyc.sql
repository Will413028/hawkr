-- name: KYCApplicationByID :one
SELECT *
FROM seller.kyc_applications
WHERE application_id = $1;

-- name: LockKYCApplication :one
-- 鎖定申請序列化它的寫入（鎖順序：店家列先於申請列）。
SELECT *
FROM seller.kyc_applications
WHERE application_id = $1
FOR UPDATE;

-- name: CurrentKYCApplication :one
-- 店家目前那一筆還沒取消的申請（kyc_applications_open 保證最多一筆）。
SELECT *
FROM seller.kyc_applications
WHERE store_id = $1 AND status <> 'CANCELLED';

-- name: InsertKYCApplication :one
-- 建立草稿；店家已有還沒取消的申請時違反 kyc_applications_open（呼叫方鎖住店家列後先檢查，所以只在不變量被破壞時發生）。
INSERT INTO seller.kyc_applications (
    application_id, store_id, seller_type, real_name, birth_date,
    id_card_no_enc, id_card_hash, id_card_hash_key, id_card_issue_date, id_card_issue_type, id_card_issue_location,
    english_name, bank_name, bank_code, bank_account_enc, bank_account_hash, bank_account_hash_key,
    product_type, website_url,
    company_name, tax_id, tax_id_hash, tax_id_hash_key, company_register_date,
    company_town_code, company_address_detail, company_phone,
    rep_id_card_no_enc, rep_id_card_hash, rep_id_card_hash_key,
    created_at, updated_at
)
VALUES (
    sqlc.arg(application_id), sqlc.arg(store_id), sqlc.arg(seller_type), sqlc.narg(real_name), sqlc.narg(birth_date),
    sqlc.narg(id_card_no_enc), sqlc.narg(id_card_hash), sqlc.narg(id_card_hash_key), sqlc.narg(id_card_issue_date), sqlc.narg(id_card_issue_type), sqlc.narg(id_card_issue_location),
    sqlc.narg(english_name), sqlc.narg(bank_name), sqlc.narg(bank_code), sqlc.narg(bank_account_enc), sqlc.narg(bank_account_hash), sqlc.narg(bank_account_hash_key),
    sqlc.narg(product_type), sqlc.narg(website_url),
    sqlc.narg(company_name), sqlc.narg(tax_id), sqlc.narg(tax_id_hash), sqlc.narg(tax_id_hash_key), sqlc.narg(company_register_date),
    sqlc.narg(company_town_code), sqlc.narg(company_address_detail), sqlc.narg(company_phone),
    sqlc.narg(rep_id_card_no_enc), sqlc.narg(rep_id_card_hash), sqlc.narg(rep_id_card_hash_key),
    sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: UpdateKYCDraft :one
-- 寫入已在列鎖內合併的草稿欄位，並讓被退回的申請回到 DRAFT；版本不符、或狀態不是 DRAFT／REJECTED／APPROVED 時 0 列
-- （APPROVED 只在 PayUni 拒絕、能重新綁定時由呼叫方在鎖內放行，見 kyc.editable）。
-- submitted_at 保留（歷史）。
UPDATE seller.kyc_applications
SET status = 'DRAFT',
    decided_at = NULL,
    seller_type = sqlc.arg(seller_type),
    real_name = sqlc.narg(real_name),
    birth_date = sqlc.narg(birth_date),
    id_card_no_enc = sqlc.narg(id_card_no_enc),
    id_card_hash = sqlc.narg(id_card_hash),
    id_card_hash_key = sqlc.narg(id_card_hash_key),
    id_card_issue_date = sqlc.narg(id_card_issue_date),
    id_card_issue_type = sqlc.narg(id_card_issue_type),
    id_card_issue_location = sqlc.narg(id_card_issue_location),
    english_name = sqlc.narg(english_name),
    bank_name = sqlc.narg(bank_name),
    bank_code = sqlc.narg(bank_code),
    bank_account_enc = sqlc.narg(bank_account_enc),
    bank_account_hash = sqlc.narg(bank_account_hash),
    bank_account_hash_key = sqlc.narg(bank_account_hash_key),
    product_type = sqlc.narg(product_type),
    website_url = sqlc.narg(website_url),
    company_name = sqlc.narg(company_name),
    tax_id = sqlc.narg(tax_id),
    tax_id_hash = sqlc.narg(tax_id_hash),
    tax_id_hash_key = sqlc.narg(tax_id_hash_key),
    company_register_date = sqlc.narg(company_register_date),
    company_town_code = sqlc.narg(company_town_code),
    company_address_detail = sqlc.narg(company_address_detail),
    company_phone = sqlc.narg(company_phone),
    rep_id_card_no_enc = sqlc.narg(rep_id_card_no_enc),
    rep_id_card_hash = sqlc.narg(rep_id_card_hash),
    rep_id_card_hash_key = sqlc.narg(rep_id_card_hash_key),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE application_id = sqlc.arg(application_id) AND version = sqlc.arg(expected_version) AND status IN ('DRAFT', 'REJECTED', 'APPROVED')
RETURNING *;

-- name: TransitionKYCApplication :one
-- 人工決定的狀態轉移：以目前的狀態與版本為條件（只有持有列鎖、確認過狀態的呼叫方會走到這裡，條件是保底）；0 列表示
-- 狀態或版本已變。轉成 SUBMITTED 時 submission_seq 加一（登記資料查詢以它辨識這一輪送件）。
UPDATE seller.kyc_applications
SET status = sqlc.arg(to_status),
    submitted_at = COALESCE(sqlc.narg(submitted_at)::timestamptz, submitted_at),
    submission_seq = submission_seq + CASE WHEN sqlc.arg(to_status)::text = 'SUBMITTED' THEN 1 ELSE 0 END,
    decided_at = sqlc.narg(decided_at)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE application_id = sqlc.arg(application_id) AND version = sqlc.arg(expected_version) AND status = sqlc.arg(from_status)
RETURNING *;

-- name: KYCDocuments :many
SELECT *
FROM seller.kyc_documents
WHERE application_id = $1
ORDER BY slot, position;

-- name: DeleteKYCDocuments :exec
DELETE FROM seller.kyc_documents WHERE application_id = $1;

-- name: InsertKYCDocument :exec
INSERT INTO seller.kyc_documents (application_id, slot, position, asset_id, object_key)
VALUES (sqlc.arg(application_id), sqlc.arg(slot), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(object_key));

-- name: InsertKYCReview :one
-- 審核紀錄只有這一支寫入（append-only）。registration_check_id 只在核准、退回時帶（kyc_reviews_registration_check_check）。
INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, reason, internal_note, decided_at, registration_check_id)
VALUES (sqlc.arg(review_id), sqlc.arg(application_id), sqlc.arg(action), sqlc.narg(reviewer_admin_id),
        sqlc.narg(reason), sqlc.narg(internal_note), sqlc.arg(now), sqlc.narg(registration_check_id))
RETURNING *;

-- name: InsertKYCReviewIssue :exec
-- 退回的逐項問題，與退回的審核紀錄在同一個 transaction 寫入；resolved_at 一開始是 NULL。
INSERT INTO seller.kyc_review_issues (issue_id, review_id, position, field, document_asset_id, code, message)
VALUES (sqlc.arg(issue_id), sqlc.arg(review_id), sqlc.arg(position), sqlc.arg(field), sqlc.narg(document_asset_id), sqlc.arg(code), sqlc.arg(message));

-- name: KYCReviewIssues :many
-- 一筆申請全部審核紀錄的逐項問題，依審核紀錄的順序（同 KYCReviews）再依 position。
SELECT i.*
FROM seller.kyc_review_issues i
JOIN seller.kyc_reviews r ON r.review_id = i.review_id
WHERE r.application_id = $1
ORDER BY r.decided_at, r.review_id, i.position;

-- name: UnresolvedKYCReviewIssues :many
-- 一筆申請還沒標成已處理的問題：寫入申請的 transaction 用它和新舊值比對（呼叫方已鎖住申請列）。
SELECT i.issue_id, i.field, i.document_asset_id
FROM seller.kyc_review_issues i
JOIN seller.kyc_reviews r ON r.review_id = i.review_id
WHERE r.application_id = $1 AND i.resolved_at IS NULL;

-- name: ResolveKYCReviewIssues :exec
-- 標成已處理（只填還沒有時間的，重複標不改變第一次的時間）。
UPDATE seller.kyc_review_issues
SET resolved_at = sqlc.arg(now)::timestamptz
WHERE issue_id = ANY(sqlc.arg(issue_ids)::uuid[]) AND resolved_at IS NULL;

-- name: KYCReviews :many
SELECT *
FROM seller.kyc_reviews
WHERE application_id = $1
ORDER BY decided_at, review_id;

-- name: InsertKYCReveal :one
-- 顯示全文的稽核紀錄（同時是限時授權）只有這一支寫入（append-only），不含值；expires_at 由呼叫方以注入的時鐘算。
INSERT INTO seller.kyc_reveals (reveal_id, application_id, admin_id, reason, revealed_at, expires_at)
VALUES (sqlc.arg(reveal_id), sqlc.arg(application_id), sqlc.arg(admin_id), sqlc.arg(reason), sqlc.arg(now), sqlc.arg(expires_at))
RETURNING *;

-- name: ActiveKYCGrant :one
-- 這位管理員對這筆申請目前有效的授權（顯示全文的紀錄，到期時間晚於 now）；有多筆時取最晚到期的。沒有時 0 列。
SELECT *
FROM seller.kyc_reveals
WHERE application_id = sqlc.arg(application_id) AND admin_id = sqlc.arg(admin_id) AND expires_at > sqlc.arg(now)
ORDER BY expires_at DESC, reveal_id DESC
LIMIT 1;

-- name: InsertKYCDocumentAccess :one
-- 證件圖片的讀取紀錄只有這一支寫入（append-only）：記實際送出的 asset_id，不含內容。application_id 與授權的 application_id
-- 由複合外鍵保證一致。
INSERT INTO seller.kyc_document_accesses (access_id, application_id, reveal_id, slot, position, asset_id, accessed_at)
VALUES (sqlc.arg(access_id), sqlc.arg(application_id), sqlc.arg(reveal_id), sqlc.arg(slot), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(now))
RETURNING *;

-- name: ListKYCDocumentAccesses :many
-- 一筆申請的證件讀取紀錄（誰從授權來），新到舊的 keyset（accessed_at, access_id）。
SELECT a.access_id, a.reveal_id, r.admin_id, a.slot, a.position, a.accessed_at
FROM seller.kyc_document_accesses a
JOIN seller.kyc_reveals r ON r.reveal_id = a.reveal_id
WHERE a.application_id = sqlc.arg(application_id)
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (a.accessed_at, a.access_id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY a.accessed_at DESC, a.access_id DESC
LIMIT sqlc.arg(page_size);

-- name: ListKYCReveals :many
-- 一筆申請的顯示全文紀錄，新到舊的 keyset（revealed_at, reveal_id）。
SELECT *
FROM seller.kyc_reveals
WHERE application_id = sqlc.arg(application_id)
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (revealed_at, reveal_id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY revealed_at DESC, reveal_id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminKYCApplications :many
-- admin 審核佇列的「新到舊」：依送出時間（不是草稿的建立時間）。只列送出過的申請（不含草稿，也不含沒送出就取消的），
-- 狀態篩選；query_pattern 是姓名、公司名稱或統編的子字串（ILIKE，呼叫方已跳脫 % _ \）。篩選審核中的佇列走
-- AdminKYCApplicationsOldestFirst（兩支的條件必須一致）。
SELECT *
FROM seller.kyc_applications
WHERE status <> 'DRAFT' AND submitted_at IS NOT NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (
    sqlc.narg(query_pattern)::text IS NULL OR
    real_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    company_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    tax_id ILIKE sqlc.narg(query_pattern)::text ESCAPE '\'
  )
  AND (
    sqlc.narg(after_submitted_at)::timestamptz IS NULL OR
    (submitted_at, application_id) < (sqlc.narg(after_submitted_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY submitted_at DESC, application_id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminKYCApplicationsOldestFirst :many
-- admin 審核佇列的「舊到新」（篩選審核中時：等最久的在最前面），條件同 AdminKYCApplications，只有排序與 keyset 方向相反。
SELECT *
FROM seller.kyc_applications
WHERE status <> 'DRAFT' AND submitted_at IS NOT NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (
    sqlc.narg(query_pattern)::text IS NULL OR
    real_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    company_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    tax_id ILIKE sqlc.narg(query_pattern)::text ESCAPE '\'
  )
  AND (
    sqlc.narg(after_submitted_at)::timestamptz IS NULL OR
    (submitted_at, application_id) > (sqlc.narg(after_submitted_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY submitted_at, application_id
LIMIT sqlc.arg(page_size);

-- name: KYCDocumentAssets :many
-- 同步 job 讀的：申請目前指著的證件 asset（申請已取消時沒有）。
SELECT d.asset_id
FROM seller.kyc_documents d
JOIN seller.kyc_applications a ON a.application_id = d.application_id
WHERE d.application_id = $1 AND a.status <> 'CANCELLED';

-- name: SnapshotMeasureIdentities :exec
-- 停權措施建立時，複製被處分帳號全部 KYC 申請（跨 stint、含已取消的）的身分 blind index 到措施擁有的快照：每個識別欄位金鑰環裡每一把金鑰算的 hash 各一筆
-- （kyc_identity_hashes），之後換鑰也查得到；同一個 hash 只留一筆。身分證號與企業負責人身分證號同一種（ID_CARD）。
INSERT INTO seller.measure_identities (measure_id, kind, key_id, hash)
SELECT sqlc.arg(measure_id)::uuid,
       CASE h.field WHEN 'TAX_ID' THEN 'TAX_ID' WHEN 'BANK_ACCOUNT' THEN 'BANK_ACCOUNT' ELSE 'ID_CARD' END,
       h.key_id, h.hash
FROM seller.kyc_identity_hashes h
JOIN seller.kyc_applications a ON a.application_id = h.application_id
JOIN seller.stores st ON st.store_id = a.store_id
WHERE st.account_id = sqlc.arg(account_id)
ON CONFLICT DO NOTHING;

-- name: MeasureIdentityMatches :many
-- 快照裡與這些 hash 相同、而且屬於別的帳號的列。只讀快照，不判斷措施是否生效：呼叫方對每個帳號以 stint.Derive 判斷
-- （生效中的定義只有 EffectiveMeasures 那一份）。
SELECT DISTINCT m.account_id, mi.kind
FROM seller.measure_identities mi
JOIN seller.measures m ON m.id = mi.measure_id
WHERE mi.hash = ANY(sqlc.arg(hashes)::bytea[]) AND m.account_id <> sqlc.arg(account_id)
ORDER BY m.account_id, mi.kind;

-- name: LinkedAccountHits :many
-- 與這筆措施的身分快照相同的其他帳號（W9）：別的帳號的申請（跨 stint、含已取消的）裡，身分證號（本人與企業負責人）對快照的 ID_CARD、統一編號對
-- TAX_ID，銀行帳號不算。快照與申請都存金鑰環每一把金鑰算的 (key_id, hash)，以索引等值連接，不解密；個資清理過的申請沒有這些列，不命中。
-- account_id 不為 NULL 時只看那個帳號。
SELECT DISTINCT st.account_id, h.field
FROM seller.measure_identities mi
JOIN seller.kyc_identity_hashes h ON h.hash = mi.hash AND h.key_id = mi.key_id
JOIN seller.kyc_applications a ON a.application_id = h.application_id
JOIN seller.stores st ON st.store_id = a.store_id
WHERE mi.measure_id = sqlc.arg(measure_id)
  AND mi.kind IN ('ID_CARD', 'TAX_ID')
  AND mi.kind = CASE h.field WHEN 'TAX_ID' THEN 'TAX_ID' ELSE 'ID_CARD' END
  AND h.field <> 'BANK_ACCOUNT'
  AND st.account_id <> sqlc.arg(except_account_id)
  AND (sqlc.narg(account_id)::bigint IS NULL OR st.account_id = sqlc.narg(account_id)::bigint)
ORDER BY st.account_id, h.field;

-- name: DeleteKYCIdentityHashes :exec
-- 清掉這筆申請某個識別欄位的比對 hash（欄位被改掉或清空時，寫入同一個 transaction）。
DELETE FROM seller.kyc_identity_hashes WHERE application_id = $1 AND field = $2;

-- name: InsertKYCIdentityHash :exec
-- 寫入一筆比對 hash；已存在的略過（補金鑰可重跑）。
INSERT INTO seller.kyc_identity_hashes (application_id, field, key_id, hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: KYCApplicationsMissingHashKey :many
-- 還有識別欄位、但缺這把金鑰的比對 hash 的申請（個資清理過的沒有欄位，不在其中）：換鑰之後補算用，依 application_id 一批一批取（after_id 是上一批最後一筆），
-- 鎖住申請列，與草稿寫入互斥。
SELECT a.*
FROM seller.kyc_applications a
WHERE a.pii_purged_at IS NULL
  AND (sqlc.narg(after_id)::uuid IS NULL OR a.application_id > sqlc.narg(after_id)::uuid)
  AND (
    (a.id_card_no_enc IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM seller.kyc_identity_hashes h WHERE h.application_id = a.application_id AND h.field = 'ID_CARD_NO' AND h.key_id = sqlc.arg(key_id)::text)) OR
    (a.rep_id_card_no_enc IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM seller.kyc_identity_hashes h WHERE h.application_id = a.application_id AND h.field = 'REP_ID_CARD_NO' AND h.key_id = sqlc.arg(key_id)::text)) OR
    (a.tax_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM seller.kyc_identity_hashes h WHERE h.application_id = a.application_id AND h.field = 'TAX_ID' AND h.key_id = sqlc.arg(key_id)::text)) OR
    (a.bank_account_enc IS NOT NULL AND a.bank_code IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM seller.kyc_identity_hashes h WHERE h.application_id = a.application_id AND h.field = 'BANK_ACCOUNT' AND h.key_id = sqlc.arg(key_id)::text))
)
ORDER BY a.application_id
LIMIT sqlc.arg(batch_limit)
FOR UPDATE OF a;

-- name: DeleteIdentitiesOutside :exec
-- 刪除帳號的措施快照中，不屬於「生效中的措施」的列。keep 是 EffectiveMeasures 的結果（呼叫方已鎖住帳號全部的 stint，所以
-- 這之間不會有新的措施建立）；生效中的定義只有那一支查詢，這裡不再寫第二份。
DELETE FROM seller.measure_identities mi
USING seller.measures m
WHERE m.id = mi.measure_id AND m.account_id = sqlc.arg(account_id)
  AND mi.measure_id <> ALL(sqlc.arg(keep)::uuid[]);

-- name: AccountsWithMeasureSnapshots :many
-- 還有身分快照的帳號（對帳時逐一清理；措施結束就刪快照，所以只是生效中的措施，加上到期 job 遺失的少數）。
SELECT DISTINCT m.account_id
FROM seller.measure_identities mi
JOIN seller.measures m ON m.id = mi.measure_id
ORDER BY m.account_id;

-- name: StoreAccounts :many
-- 一批店家所屬的帳號（admin 清單補顯示欄位；申請列只存 store_id）。
SELECT store_id, account_id
FROM seller.stores
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: ReferencedKeyIDs :many
-- 資料庫裡被參照的每一把金鑰 ID（啟動檢查用）：ENCRYPTION 是密文開頭的金鑰 ID（1 byte 的長度、ID），HASH 是 blind index
-- 的金鑰 ID（申請的 hash、比對用的 hash 與處分的身分快照）。已清理（NULL）的欄位不參照任何金鑰。
SELECT 'ENCRYPTION'::text AS kind, convert_from(substring(c FROM 2 FOR get_byte(c, 0)), 'UTF8')::text AS key_id
FROM (
    SELECT id_card_no_enc AS c FROM seller.kyc_applications
    UNION ALL SELECT rep_id_card_no_enc FROM seller.kyc_applications
    UNION ALL SELECT bank_account_enc FROM seller.kyc_applications
) e
WHERE c IS NOT NULL
GROUP BY 2
UNION
SELECT 'HASH'::text, k.key_id::text
FROM (
    SELECT id_card_hash_key AS key_id FROM seller.kyc_applications
    UNION SELECT rep_id_card_hash_key FROM seller.kyc_applications
    UNION SELECT tax_id_hash_key FROM seller.kyc_applications
    UNION SELECT bank_account_hash_key FROM seller.kyc_applications
    UNION SELECT key_id FROM seller.kyc_identity_hashes
    UNION SELECT key_id FROM seller.measure_identities
) k
WHERE k.key_id IS NOT NULL
ORDER BY 1, 2;

-- name: CurrentKYCStatuses :many
-- 批次讀取各店家目前那一筆還沒取消的申請的 cert 狀態（kyc_applications_open 保證每店最多一筆）；沒有申請的店家沒有列。
SELECT store_id, status
FROM seller.kyc_applications
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND status <> 'CANCELLED';

-- name: ReviewKYCApplicationByIDs :many
-- 這些商家認證申請的審核開始時間與期限（資源的 decide_deadline）。規則只有 seller.review_kyc_application(at)（objects.sql）；不是審核中的申請沒有列。
-- 輸出欄位的轉型是給 sqlc 的（它不型別化複合型別函式的輸出欄位，沒有轉型會在生成時報 column does not exist）。
SELECT r.kyc_application_id::uuid AS kyc_application_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_kyc_application(sqlc.arg(at)::timestamptz) r
WHERE r.kyc_application_id = ANY(sqlc.arg(ids)::uuid[]);
