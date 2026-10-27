-- name: CurrentStore :one
-- 讀取帳號目前的 stint（status 不是 CLOSED）。
SELECT *
FROM seller.stores
WHERE account_id = $1 AND status <> 'CLOSED';

-- name: StoreByID :one
-- 讀取指定 stint，包含已關閉的。
SELECT *
FROM seller.stores
WHERE store_id = $1;

-- name: StoresByIDs :many
-- 批次讀取指定 stint（含已關閉的），不存在的 ID 略過；是否可公開曝光由 exposure.Classify（seller.pub_exposable_stores）判斷，這裡不過濾。
SELECT *
FROM seller.stores
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: StoreIDsByAccount :many
-- 帳號全部的 stint（含已關閉的），依 store_id；唯讀，不鎖（LockAccountStores 是寫入流程的鎖）。只給 legacy 的個資清除 consumer
-- 列舉歷史 store_id。
SELECT store_id
FROM seller.stores
WHERE account_id = $1
ORDER BY store_id;

-- name: StoreByCode :one
-- 以 store_code 讀取指定 stint，包含已關閉的；是否可公開曝光由 exposure.Classify（seller.pub_exposable_stores）判斷，這裡不過濾。
SELECT *
FROM seller.stores
WHERE store_code = $1;

-- name: ExposedStoresAfter :many
-- 最後公告為可曝光的 stint，依 store_id 取 after 之後的一批，連同公告時間（exposure_changed_at，公開資料的更新時間之一）。
-- 只是候選：公告落後現況時，呼叫方仍以 exposure.Classify（seller.pub_exposable_stores）的判定為準，這裡不套規則。給公開清單（ListPublicStores）。
SELECT sqlc.embed(s), e.exposable_changed_at AS exposure_changed_at
FROM seller.store_exposures e
JOIN seller.stores s ON s.store_id = e.store_id
WHERE e.exposable AND e.store_id > sqlc.arg(after_store_id)::bigint
ORDER BY e.store_id
LIMIT sqlc.arg(batch_size)::int;

-- name: StoreExposureChangedAt :many
-- 這些店家最後公告的時間（還沒公告過的是 NULL），不鎖；唯讀。
SELECT store_id, exposable_changed_at
FROM seller.store_exposures
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: LockStore :one
-- 鎖定指定 stint，序列化它的寫入。
SELECT *
FROM seller.stores
WHERE store_id = $1
FOR UPDATE;

-- name: CreateStore :one
-- 建立新的 stint；store_code 撞號時不寫入、回 0 列，由呼叫方換一個代碼重試。帳號已有目前的 stint 時違反
-- stores_current_account（呼叫方先鎖帳號再檢查，所以只在不變量被破壞時發生）。
INSERT INTO seller.stores (
    store_code, account_id, store_name, description, contact_phone, town_code, address_detail, prize,
    created_at, updated_at
)
VALUES (
    sqlc.arg(store_code), sqlc.arg(account_id), sqlc.arg(store_name)::text, sqlc.narg(description), sqlc.narg(contact_phone),
    sqlc.narg(town_code), sqlc.narg(address_detail), sqlc.narg(prize), sqlc.arg(now), sqlc.arg(now)
)
ON CONFLICT (store_code) DO NOTHING
RETURNING *;

-- name: UpdateProfile :one
-- 寫入已在列鎖內合併的 profile 欄位；版本不符時 0 列。
UPDATE seller.stores
SET store_name = sqlc.arg(store_name)::text,
    description = sqlc.narg(description),
    contact_phone = sqlc.narg(contact_phone),
    contact_name = sqlc.narg(contact_name),
    contact_mobile = sqlc.narg(contact_mobile),
    support_email = sqlc.narg(support_email),
    town_code = sqlc.narg(town_code),
    address_detail = sqlc.narg(address_detail),
    prize = sqlc.narg(prize),
    website_url = sqlc.narg(website_url),
    onboarding_completed = sqlc.arg(onboarding_completed),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND version = sqlc.arg(expected_version) AND status <> 'CLOSED'
RETURNING *;

-- name: UpdateLogo :one
-- 寫入已轉正的 logo object key；版本不符時 0 列。
UPDATE seller.stores
SET logo_key = sqlc.arg(logo_key),
    logo_asset_id = sqlc.arg(logo_asset_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND version = sqlc.arg(expected_version) AND status <> 'CLOSED'
RETURNING *;

-- name: UpdateCover :one
-- 寫入已轉正的封面 object key；版本不符時 0 列。
UPDATE seller.stores
SET cover_key = sqlc.arg(cover_key),
    cover_asset_id = sqlc.arg(cover_asset_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND version = sqlc.arg(expected_version) AND status <> 'CLOSED'
RETURNING *;

-- name: MarkPendingDeletion :exec
-- 店家隨帳號刪除進入等待；只從 ACTIVE 轉移。
UPDATE seller.stores
SET status = 'PENDING_DELETION',
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND status = 'ACTIVE';

-- name: CloseStore :exec
-- 關閉 stint：釋放「目前」的位置。關店可逆，logo 與封面留著（連同 media 的引用），到個資清理才釋放（PurgeStores）。關店的紀錄由呼叫方同一個
-- transaction 以 OpenStoreClosure 寫入 store_closures（關店紀錄只有那一處），now 與它相同：current_closed_at 指向那一列。
UPDATE seller.stores
SET status = 'CLOSED',
    current_closed_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND status <> 'CLOSED';

-- name: OpenStoreClosure :exec
-- 記下這一次關店（目前的紀錄：reopened_at 為 NULL；一個 stint 同時最多一筆）。closed_by 是關閉的原因，終止合作另記造成關店的措施
-- （store_closures_measure_check）。
INSERT INTO seller.store_closures (store_id, closed_at, closed_by, closed_by_measure_id)
VALUES (sqlc.arg(store_id), sqlc.arg(now), sqlc.arg(closed_by), sqlc.narg(closed_by_measure_id));

-- name: ReopenStore :one
-- 恢復營業：CLOSED → ACTIVE。只作用於還沒退場的列（退場之後下游已刪除，不能恢復）；是否在保留期內由呼叫方以 RetireDueStores 與 StoreHolds 判斷
-- （清理與恢復共用同一個判斷式）。
-- 目前那筆關店紀錄由呼叫方同一個 transaction 以 EndStoreClosure 補上 reopened_at。
UPDATE seller.stores
SET status = 'ACTIVE',
    current_closed_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND status = 'CLOSED' AND retired_at IS NULL
RETURNING *;

-- name: EndStoreClosure :execrows
-- 結束目前的關店紀錄（恢復營業）；沒有目前的紀錄時 0 列。
UPDATE seller.store_closures
SET reopened_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND reopened_at IS NULL;

-- name: CurrentStoreClosures :many
-- 這些 stint 目前的關店紀錄（已關閉的才有）。
SELECT *
FROM seller.store_closures
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND reopened_at IS NULL;

-- name: StoreClosures :many
-- 一個 stint 的關店歷史（含目前這一次），舊到新。
SELECT *
FROM seller.store_closures
WHERE store_id = $1
ORDER BY closed_at;

-- name: LatestStoreOfAccount :one
-- 帳號最近的 stint（store_id 最大的；含已關閉的）。
SELECT *
FROM seller.stores
WHERE account_id = $1
ORDER BY store_id DESC
LIMIT 1;

-- name: ImageAssets :one
-- 同步 job 讀的：店家目前的 logo、封面 asset（沒有圖時是 NULL；關店可逆，已關閉的 stint 到個資清理才釋放，欄位就是引用的來源）。
SELECT logo_asset_id, cover_asset_id
FROM seller.stores
WHERE store_id = $1;

-- name: LockAccountStores :many
-- 鎖住帳號全部的 stint（含已關閉的），固定依 store_id 的順序，避免兩個重算互鎖。停權的推導與期間的寫入都在這個鎖
-- 之後才讀處分。
SELECT *
FROM seller.stores
WHERE account_id = $1
ORDER BY store_id
FOR UPDATE;

-- name: AdminStores :many
-- 後台的店家清單：生命週期（呼叫方一定給至少一個）、名稱或電話（ILIKE，呼叫方已跳脫 % _ \）或 store_code 相同、帳號、
-- 目前那一筆還沒取消的 KYC 申請的狀態、PayUni 註冊的綁定狀態（儲存的事實），新到舊的 keyset。停權不在這裡判斷：它的定義
-- 只有 EffectiveMeasures 一處，呼叫方先以它取得生效中停權措施的帳號（suspended_accounts），suspended 為 true 只列這些帳號的
-- 店家、false 只列不在其中的。
SELECT st.*
FROM seller.stores st
WHERE st.status = ANY(sqlc.arg(statuses)::text[])
  AND (sqlc.narg(account_id)::bigint IS NULL OR st.account_id = sqlc.narg(account_id)::bigint)
  AND (
    sqlc.narg(query_pattern)::text IS NULL OR
    st.store_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    st.contact_phone ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    st.store_code = sqlc.narg(store_code)::bigint
  )
  AND (
    sqlc.narg(kyc_status)::text IS NULL OR
    EXISTS (
      SELECT 1 FROM seller.kyc_applications k
      WHERE k.store_id = st.store_id AND k.status <> 'CANCELLED' AND k.status = sqlc.narg(kyc_status)::text
    )
  )
  AND (
    NOT sqlc.arg(kyc_awaiting_review)::bool OR
    EXISTS (SELECT 1 FROM seller.review_kyc_application(sqlc.arg(now)::timestamptz) r WHERE r.store_id = st.store_id)
  )
  AND (
    sqlc.narg(bind_status)::text IS NULL OR
    EXISTS (
      SELECT 1 FROM seller.payuni_registrations r
      WHERE r.store_id = st.store_id AND r.bind_status = sqlc.narg(bind_status)::text
    )
  )
  AND (
    sqlc.narg(suspended)::bool IS NULL OR
    (st.account_id = ANY(sqlc.arg(suspended_accounts)::bigint[])) = sqlc.narg(suspended)::bool
  )
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (st.created_at, st.store_id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::bigint)
  )
ORDER BY st.created_at DESC, st.store_id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminStoreStats :one
-- 後台的店家統計，一句查詢、同一個 snapshot：目前的店家（營運中與等待刪除）的總數、依目前那一筆還沒取消的 KYC 申請狀態的核准與退回家數、
-- 商家認證申請在後台待辦裡的家數（kyc_awaiting_review：與待辦、商家清單的「待審核」分頁同一個成員 seller.review_kyc_application），
-- 以及台北本月（[month_start, month_end)）新開的家數。
SELECT
  count(*)::int AS total,
  (count(*) FILTER (WHERE EXISTS (SELECT 1 FROM seller.review_kyc_application(sqlc.arg(now)::timestamptz) r WHERE r.store_id = st.store_id)))::int AS kyc_awaiting_review,
  (count(*) FILTER (WHERE k.status = 'APPROVED'))::int AS kyc_approved,
  (count(*) FILTER (WHERE k.status = 'REJECTED'))::int AS kyc_rejected,
  (count(*) FILTER (WHERE st.created_at >= sqlc.arg(month_start)::timestamptz AND st.created_at < sqlc.arg(month_end)::timestamptz))::int AS new_this_month
FROM seller.stores st
LEFT JOIN seller.kyc_applications k ON k.store_id = st.store_id AND k.status <> 'CANCELLED'
WHERE st.status <> 'CLOSED';
