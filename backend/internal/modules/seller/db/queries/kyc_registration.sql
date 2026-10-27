-- name: InsertKYCRegistrationCheck :one
-- 新增一列 PENDING 的查詢，綁定申請目前這一輪送件（submission_seq 取申請的值，所以呼叫方必須在申請已轉成 SUBMITTED 之後呼叫；
-- 還沒送出過的申請是 0，CHECK 擋下）；seq 是同一申請內的下一號。呼叫方已持有申請列鎖（送件與重新查詢都先鎖申請），所以同一申請的
-- 新增互相序列化，(application_id, seq) 的唯一約束是保底。
INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_by_admin_id, requested_at, deadline_at, submission_seq)
VALUES (
    sqlc.arg(check_id), sqlc.arg(application_id),
    (SELECT COALESCE(MAX(seq), 0) + 1 FROM seller.kyc_registration_checks WHERE application_id = sqlc.arg(application_id)),
    sqlc.arg(request_kind), sqlc.narg(requested_by_admin_id), sqlc.arg(now), sqlc.arg(deadline_at),
    (SELECT submission_seq FROM seller.kyc_applications WHERE application_id = sqlc.arg(application_id))
)
RETURNING *;

-- name: ClaimKYCRegistrationCheck :one
-- job 的認領：鎖住查詢列並一併讀申請的比對欄位（公司名稱、鄉鎮市區與地址明細、設立日期、統編）。呼叫方在
-- transaction 內讀完就結束它，之後才呼叫外部；結果由 CompleteKYCRegistrationCheck 在另一個 transaction 寫入。
-- 申請的個資已被清理時比對欄位是 NULL（呼叫方此時查詢列也已被刪除，所以通常讀不到列）。同時讀申請目前的狀態與輪次、查詢綁定的輪次、
-- 同一輪是否已有更新（seq 更大）的查詢：申請已不是這一輪審核中（退回、取消、重送），或這筆已被更新的取代時，呼叫方放棄它（CANCELLED）。
SELECT c.check_id, c.application_id, c.outcome, c.deadline_at, c.submission_seq AS check_submission_seq,
       a.status AS application_status, a.submission_seq AS application_submission_seq,
       EXISTS (
           SELECT 1 FROM seller.kyc_registration_checks newer
           WHERE newer.application_id = c.application_id AND newer.submission_seq = c.submission_seq AND newer.seq > c.seq
       ) AS superseded,
       a.seller_type, a.tax_id, a.company_name, a.company_town_code, a.company_address_detail,
       a.company_register_date
FROM seller.kyc_registration_checks c
JOIN seller.kyc_applications a ON a.application_id = c.application_id
WHERE c.check_id = $1
FOR UPDATE OF c;

-- name: CompleteKYCRegistrationCheck :one
-- 寫入查詢的結果。只在仍是 PENDING 時寫（條件是這裡的唯一把關：重複投遞或重試的 job 不覆寫已寫入的結果）；0 列表示已有
-- 結果。比對結果與登記資料在同一次寫入。
UPDATE seller.kyc_registration_checks
SET outcome = sqlc.arg(outcome),
    queried_at = sqlc.arg(queried_at),
    tax_id = sqlc.narg(tax_id),
    name = sqlc.narg(name),
    address_text = sqlc.narg(address_text),
    town_code = sqlc.narg(town_code),
    address_detail = sqlc.narg(address_detail),
    setup_date = sqlc.narg(setup_date),
    organization_type = sqlc.narg(organization_type),
    status = sqlc.narg(status),
    status_text = sqlc.narg(status_text),
    suspended_from = sqlc.narg(suspended_from),
    suspended_until = sqlc.narg(suspended_until),
    mismatched_fields = sqlc.arg(mismatched_fields)::text[],
    unverifiable_fields = sqlc.arg(unverifiable_fields)::text[]
WHERE check_id = sqlc.arg(check_id) AND outcome = 'PENDING'
RETURNING *;

-- name: LatestCompletedKYCRegistrationCheck :one
-- 後台顯示的查詢：申請目前這一輪送件（查詢的 submission_seq 等於申請的）seq 最大的已完成查詢（FOUND、NOT_FOUND、UNAVAILABLE；
-- CANCELLED 不顯示）；舊的查詢比新的晚完成時，顯示的仍是新的。申請在送出、核准、退回狀態才有快照（草稿與取消沒有）；沒有時無列。
-- 退回後重送沿用同一筆申請，輪次加一，所以上一輪的快照不會顯示成這一輪的依據。
SELECT c.*
FROM seller.kyc_registration_checks c
JOIN seller.kyc_applications a ON a.application_id = c.application_id
WHERE c.application_id = $1
  AND a.status IN ('SUBMITTED', 'APPROVED', 'REJECTED')
  AND c.submission_seq = a.submission_seq
  AND c.outcome NOT IN ('PENDING', 'CANCELLED')
ORDER BY c.seq DESC
LIMIT 1;

-- name: PendingKYCRegistrationCheck :one
-- 申請目前這一輪還沒查完的查詢（最新的一列）；沒有時無列。只在申請是 SUBMITTED 時才算「查詢中」（已退回、核准、取消的申請不顯示
-- 查詢中）。顯示端以它標示「查詢中」。
SELECT c.*
FROM seller.kyc_registration_checks c
JOIN seller.kyc_applications a ON a.application_id = c.application_id
WHERE c.application_id = $1 AND a.status = 'SUBMITTED' AND c.outcome = 'PENDING' AND c.submission_seq = a.submission_seq
ORDER BY c.seq DESC
LIMIT 1;

-- name: SupersedePendingKYCRegistrationChecks :execrows
-- 後台重新查詢：把申請目前這一輪還沒查完的查詢全部收掉（CANCELLED，被取代），再由呼叫方在同一個 transaction 內新增一筆。任何原因留下
-- 的 PENDING（程序當掉、job 被丟棄）都由此收掉；被取代的 job 之後認領時看到它已不是 PENDING，直接結束。
UPDATE seller.kyc_registration_checks c
SET outcome = 'CANCELLED', queried_at = sqlc.arg(now)
FROM seller.kyc_applications a
WHERE c.application_id = sqlc.arg(application_id) AND a.application_id = c.application_id
  AND c.outcome = 'PENDING' AND c.submission_seq = a.submission_seq;

-- name: ExpireKYCRegistrationChecks :execrows
-- 掃描 job 的保證終態：deadline_at 不晚於 deadline_before 還在 PENDING 的查詢收成終態（呼叫方給的是「現在加一個掃描間隔」，
-- 所以在期限之前就收掉：掃描最晚在期限前的最後一次執行收成終態）。
-- 仍屬於審核中這一輪、而且沒有被更新的取代的記成 UNAVAILABLE（來源在期限內沒有給出答案）；其餘（申請已退回、核准、取消、重送過，或被
-- 取代）記成 CANCELLED，與認領時放棄的語意一致。之後才跑的 job 認領時看到它已不是 PENDING，直接結束；寫入結果只在仍是 PENDING 時發生，
-- 所以不會覆寫這裡的終態。
UPDATE seller.kyc_registration_checks c
SET outcome = CASE
        WHEN a.status = 'SUBMITTED' AND a.submission_seq = c.submission_seq AND NOT EXISTS (
            SELECT 1 FROM seller.kyc_registration_checks newer
            WHERE newer.application_id = c.application_id AND newer.submission_seq = c.submission_seq AND newer.seq > c.seq
        ) THEN 'UNAVAILABLE'
        ELSE 'CANCELLED'
    END,
    queried_at = sqlc.arg(now)
FROM seller.kyc_applications a
WHERE a.application_id = c.application_id
  AND c.outcome = 'PENDING' AND c.deadline_at <= sqlc.arg(deadline_before)::timestamptz;

-- name: KYCRegistrationChecksToResync :many
-- 閒置超過一段時間還在 PENDING 的查詢（job 遺失、當掉或被丟棄時的保底；先收掉過期的，所以這裡的都還在期限內）；呼叫方重新排入
-- job，River 的唯一鍵讓還存在的 job 不會重複。
SELECT c.check_id
FROM seller.kyc_registration_checks c
WHERE c.outcome = 'PENDING' AND c.requested_at <= sqlc.arg(idle_before)::timestamptz
ORDER BY c.deadline_at, c.check_id
LIMIT sqlc.arg(batch_size);

-- name: ListKYCRegistrationChecks :many
SELECT *
FROM seller.kyc_registration_checks
WHERE application_id = $1
ORDER BY seq;

-- name: KYCReviewRegistrationChecks :many
-- 審核紀錄引用的查詢（只取 seq 與查詢時間，讓審核紀錄看得到依據）；沒有引用的審核不在結果內。
SELECT r.review_id, c.check_id, c.seq, c.queried_at
FROM seller.kyc_reviews r
JOIN seller.kyc_registration_checks c ON c.check_id = r.registration_check_id
WHERE r.application_id = $1;

-- name: PurgeKYCRegistrationChecks :execrows
-- 刪掉已清理的申請的全部查詢列（登記資料含公司名稱與地址，與申請的個資同時清除）；kyc_reviews 的引用由外鍵置 NULL。
DELETE FROM seller.kyc_registration_checks
WHERE application_id = ANY(sqlc.arg(application_ids)::uuid[]);
