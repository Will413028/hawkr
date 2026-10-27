-- name: InsertFoodRegistration :one
-- 送出一筆待審；店家已有待審時違反 food_registrations_one_pending（呼叫方鎖住店家列、先撤回舊的，所以只在不變量被破壞時發生）。
INSERT INTO seller.food_registrations (id, store_id, account_id, registration_number, state, submitted_at)
VALUES (sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(account_id), sqlc.arg(registration_number), 'PENDING', sqlc.arg(now))
RETURNING *;

-- name: PendingFoodRegistration :one
SELECT *
FROM seller.food_registrations
WHERE store_id = $1 AND state = 'PENDING';

-- name: ApprovedFoodRegistration :one
SELECT *
FROM seller.food_registrations
WHERE store_id = $1 AND state = 'APPROVED';

-- name: FoodRegistrationByID :one
SELECT *
FROM seller.food_registrations
WHERE id = $1;

-- name: LockFoodRegistration :one
-- 鎖順序：店家列先於登錄列。
SELECT *
FROM seller.food_registrations
WHERE id = $1
FOR UPDATE;

-- name: WithdrawPendingFoodRegistration :one
-- 撤回店家待審的那一筆（賣家撤回、被新送出的取代、清除時一併撤回）；沒有待審時 0 列。
UPDATE seller.food_registrations
SET state = 'WITHDRAWN', ended_at = sqlc.arg(now)::timestamptz
WHERE store_id = sqlc.arg(store_id) AND state = 'PENDING'
RETURNING *;

-- name: ApproveFoodRegistration :one
-- 核准待審的一筆；已不是待審時 0 列。呼叫方先結束舊的核准（food_registrations_one_approved）。
UPDATE seller.food_registrations
SET state = 'APPROVED', decided_by_admin_id = sqlc.arg(admin_id)::bigint, decided_at = sqlc.arg(now)::timestamptz, internal_note = sqlc.narg(internal_note)
WHERE id = sqlc.arg(id) AND state = 'PENDING'
RETURNING *;

-- name: RejectFoodRegistration :one
UPDATE seller.food_registrations
SET state = 'REJECTED', decided_by_admin_id = sqlc.arg(admin_id)::bigint, decided_at = sqlc.arg(now)::timestamptz,
    reject_reason = sqlc.arg(reject_reason)::text, internal_note = sqlc.narg(internal_note)
WHERE id = sqlc.arg(id) AND state = 'PENDING'
RETURNING *;

-- name: EndApprovedFoodRegistration :one
-- 結束店家目前核准的那一筆（被新核准的取代，或賣家清除）；沒有核准的時 0 列。
UPDATE seller.food_registrations
SET state = 'ENDED', ended_at = sqlc.arg(now)::timestamptz, end_cause = sqlc.arg(end_cause)::text
WHERE store_id = sqlc.arg(store_id) AND state = 'APPROVED'
RETURNING *;

-- name: RevokeApprovedFoodRegistration :one
-- 平台撤銷核准中的那一筆（資源層級，給賣家看的理由必填；核准時的內部註記留著）；已不是核准中時 0 列。
UPDATE seller.food_registrations
SET state = 'ENDED', ended_at = sqlc.arg(now)::timestamptz, end_cause = 'REVOKED',
    end_reason = sqlc.arg(end_reason)::text, ended_by_admin_id = sqlc.arg(admin_id)::bigint
WHERE id = sqlc.arg(id) AND state = 'APPROVED'
RETURNING *;

-- name: StoreFoodRegistrations :many
-- 一批店家各自核准的那一筆與賣家看得到的登錄（最近送出的那一筆，狀態是 PENDING 或 REJECTED 才算；判斷在呼叫方），每店最多兩列。
SELECT f.*
FROM seller.food_registrations f
WHERE f.store_id = ANY(sqlc.arg(store_ids)::bigint[])
  AND (f.state = 'APPROVED'
       OR f.id = (
        SELECT l.id FROM seller.food_registrations l
        WHERE l.store_id = f.store_id
        ORDER BY l.submitted_at DESC, l.id DESC
        LIMIT 1))
ORDER BY f.store_id, f.submitted_at, f.id;

-- name: StoreRevokedFoodRegistrations :many
-- 一批店家各自最近一次被平台撤銷的那一筆（撤銷時間最晚的），而且撤銷之後沒有更新的待審或核准：更新的待審是撤銷之後送出而且還在待審，
-- 更新的核准是撤銷之後核准的（不論之後是否又結束）。店家目前有核准時由呼叫方不帶。沒有的店家沒有列。
SELECT r.*
FROM (
    SELECT DISTINCT ON (x.store_id) x.*
    FROM seller.food_registrations x
    WHERE x.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND x.end_cause = 'REVOKED'
    ORDER BY x.store_id, x.ended_at DESC, x.id DESC
) r
WHERE NOT EXISTS (
    SELECT 1 FROM seller.food_registrations g
    WHERE g.store_id = r.store_id AND g.id <> r.id
      AND ((g.state = 'PENDING' AND g.submitted_at > r.ended_at)
           OR (g.state IN ('APPROVED', 'ENDED') AND g.decided_at > r.ended_at)))
ORDER BY r.store_id;

-- name: FoodRegistrationReview :one
-- 一筆登錄的後台檢視：登錄與店家。KYC 的狀態與主體另查（CurrentKYCStatuses、ApprovedKYCSubjects），後台只看得到已核准的主體。
SELECT sqlc.embed(f), s.store_name, s.status AS store_status
FROM seller.food_registrations f
JOIN seller.stores s ON s.store_id = f.store_id
WHERE f.id = $1;

-- name: AdminFoodRegistrationDecidable :many
-- 收件匣的「現在可以裁定」隊列：成員只讀 seller.review_food_registration(now)（objects.sql，後台待辦的 FOOD_REGISTRATION 也讀它）：待審、店家營運中、
-- 店家目前的商家認證已核准。依期限由近到遠（Will 2026-10-20 Q-A）；sort_at 是期限（due_at），keyset 的位置是 (期限, id)。
-- 商家認證被退回又核准時開始時間會換，所以位置在翻頁途中可能移動（待辦頁的同一個語意，step10e-3d-design.md E9），重新整理就正確。
SELECT sqlc.embed(f), s.store_name, s.status AS store_status, d.due_at::timestamptz AS sort_at
FROM seller.review_food_registration(sqlc.arg(now)::timestamptz) d
JOIN seller.food_registrations f ON f.id = d.food_registration_id
JOIN seller.stores s ON s.store_id = f.store_id
WHERE (sqlc.narg(query_pattern)::text IS NULL OR f.registration_number ILIKE sqlc.narg(query_pattern) OR s.store_name ILIKE sqlc.narg(query_pattern))
  AND (sqlc.narg(after_id)::uuid IS NULL OR (d.due_at, f.id) > (sqlc.narg(after_sort_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY sort_at, f.id
LIMIT sqlc.arg(page_size);

-- name: AdminFoodRegistrationAwaitingKYC :many
-- 收件匣的「等 KYC」隊列：營運中的店家的待審件、店家沒有已核准的 KYC 申請（沒申請、草稿、審核中、已退回），送出時間舊到新（這一隊沒有期限：
-- 還不能裁定）；keyset 的位置是 (送出時間, id)，兩者都不會變。
SELECT sqlc.embed(f), s.store_name, s.status AS store_status
FROM seller.food_registrations f
JOIN seller.stores s ON s.store_id = f.store_id
WHERE f.state = 'PENDING' AND s.status = 'ACTIVE'
  AND NOT EXISTS (SELECT 1 FROM seller.kyc_applications k WHERE k.store_id = f.store_id AND k.status = 'APPROVED')
  AND (sqlc.narg(query_pattern)::text IS NULL OR f.registration_number ILIKE sqlc.narg(query_pattern) OR s.store_name ILIKE sqlc.narg(query_pattern))
  AND (sqlc.narg(after_id)::uuid IS NULL OR (f.submitted_at, f.id) > (sqlc.narg(after_submitted_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY f.submitted_at, f.id
LIMIT sqlc.arg(page_size);

-- name: AdminFoodRegistrationsByState :many
-- 已裁定或已結束的登錄（查歷史）：新到舊；店家的狀態不限。
SELECT sqlc.embed(f), s.store_name, s.status AS store_status
FROM seller.food_registrations f
JOIN seller.stores s ON s.store_id = f.store_id
WHERE f.state = sqlc.arg(state)
  AND (sqlc.narg(query_pattern)::text IS NULL OR f.registration_number ILIKE sqlc.narg(query_pattern) OR s.store_name ILIKE sqlc.narg(query_pattern))
  AND (sqlc.narg(after_id)::uuid IS NULL OR (f.submitted_at, f.id) < (sqlc.narg(after_submitted_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY f.submitted_at DESC, f.id DESC
LIMIT sqlc.arg(page_size);

-- name: ApprovedKYCSubjects :many
-- 這些店家已核准的 KYC 申請的主體（核對登錄要比對的對象）。只回 APPROVED 的：賣家未送出或還在審的草稿沒有被驗證過，後台看不到
-- （同 ListKYCApplications 只列送出過的）；沒有已核准申請的店家沒有列。
SELECT store_id, seller_type, real_name, company_name, tax_id
FROM seller.kyc_applications
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND status = 'APPROVED';

-- name: StoreFoodChangedAt :many
-- 這些店家的公開字號最後一次改變的時間：目前核准那一筆的核准時間，與最近一次賣家清除或平台撤銷的結束時間，取較晚的；從來沒有核准、清除或撤銷過的
-- 店家沒有列。給公開資料的 update_time（model.PublicStoreUpdateTime）。被取代的舊核准結束時，新的核准時間就是同一刻。
SELECT store_id,
       MAX(CASE WHEN state = 'APPROVED' THEN decided_at WHEN end_cause IN ('CLEARED', 'REVOKED') THEN ended_at END)::timestamptz AS food_changed_at
FROM seller.food_registrations
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND (state = 'APPROVED' OR end_cause IN ('CLEARED', 'REVOKED'))
GROUP BY store_id;

-- name: WithdrawPendingFoodRegistrations :many
-- 店家退場（第一段清理，之後不會再重開）時撤回這些店家待審的登錄；沒有待審的店家沒有列。只作用於店家列已退場的店家。
UPDATE seller.food_registrations f
SET state = 'WITHDRAWN', ended_at = sqlc.arg(now)::timestamptz
FROM seller.stores s
WHERE f.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND s.store_id = f.store_id AND s.retired_at IS NOT NULL AND f.state = 'PENDING'
RETURNING f.store_id;

-- name: ReviewFoodRegistrationByIDs :many
-- 這些登錄的裁定開始時間與期限（資源的 decide_deadline）。規則只有 seller.review_food_registration(at)（objects.sql）；現在不能裁定的登錄
-- （等商家認證、店家關閉、已裁定）沒有列。
-- 輸出欄位的轉型是給 sqlc 的（它不型別化複合型別函式的輸出欄位，沒有轉型會在生成時報 column does not exist）。
SELECT r.food_registration_id::uuid AS food_registration_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_food_registration(sqlc.arg(at)::timestamptz) r
WHERE r.food_registration_id = ANY(sqlc.arg(ids)::uuid[]);
