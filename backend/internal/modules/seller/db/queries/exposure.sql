-- name: InsertStoreExposure :exec
-- 開店時與店家列在同一個 transaction 建立公告列（version 0：還沒公告過）。
INSERT INTO seller.store_exposures (store_id)
VALUES ($1);

-- name: LockStoreExposures :many
-- 鎖住這些店家最後公告的列，依 store_id；鎖住之後才算現在的可曝光（先算再鎖，兩個同時進行的寫入會各拿著對方還沒 commit
-- 之前的輸入）。
SELECT *
FROM seller.store_exposures
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[])
ORDER BY store_id
FOR UPDATE;

-- name: AnnounceStoreExposure :one
-- 改成新的公告值，version 加一。exposable_changed_at 只在公告值翻轉（或第一次公告）時前進：全量重新公告重發同一個值不動它。
UPDATE seller.store_exposures
SET exposable = sqlc.arg(exposable)::boolean,
    version = version + 1,
    exposable_changed_at = CASE WHEN exposable_changed_at IS NULL OR exposable <> sqlc.arg(exposable)::boolean THEN sqlc.arg(now) ELSE exposable_changed_at END
WHERE store_id = sqlc.arg(store_id)
RETURNING *;

-- name: StampFirstExposed :execrows
-- 記下第一次可公開曝光的時間。只寫還沒有值的列：已經有的時間不覆寫。唯一的寫入者是 exposure 套件的公告函式。
UPDATE seller.stores
SET first_exposed_at = sqlc.arg(now)
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND first_exposed_at IS NULL;

-- name: ExposureReconcileCandidates :many
-- 對帳的候選：還沒關閉的店，加上最後公告為可曝光的店（關店之後要補發 false），依 store_id 取 after 之後的一批。
SELECT s.store_id
FROM seller.stores s
LEFT JOIN seller.store_exposures e ON e.store_id = s.store_id
WHERE (s.status <> 'CLOSED' OR COALESCE(e.exposable, false))
  AND s.store_id > sqlc.arg(after_store_id)::bigint
ORDER BY s.store_id
LIMIT sqlc.arg(batch_size)::int;

-- name: ExposureStoreIDs :many
-- 公告列的全部店家（含已關閉的），依 store_id 取 after 之後的一批；全量重新公告的候選。
SELECT store_id
FROM seller.store_exposures
WHERE store_id > sqlc.arg(after_store_id)::bigint
ORDER BY store_id
LIMIT sqlc.arg(batch_size)::int;

-- name: ExposableStoresByCounty :many
-- 最後公告為可曝光、而且鄉鎮市區代碼的前 5 碼（縣市代碼）在 county_codes 裡的店，依 store_id。公告的值與 Java 的
-- seller.exposure.changed 消費者看到的是同一份。
SELECT s.store_id, left(s.town_code, 5)::text AS county_code
FROM seller.store_exposures e
JOIN seller.stores s ON s.store_id = e.store_id
WHERE e.exposable AND left(s.town_code, 5) = ANY(sqlc.arg(county_codes)::text[])
ORDER BY s.store_id;

-- name: StoreRequirementFlags :many
-- 這些店家與時間無關的條件（seller.store_requirement_flags：曝光與能力的條件唯一的家，Go 只把布林對應到 model.Requirement，不重算）；
-- 不存在的 ID 沒有列。stint.WithConditions 讀店家時帶上。
SELECT f.store_id, f.active, f.not_pending_deletion, f.kyc_approved, f.has_store_name, f.has_logo, f.has_town_code
FROM seller.store_requirement_flags f
WHERE f.store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: ExposableStoreIDs :many
-- 這些店家裡 at 這一刻可以公開曝光的（seller.pub_exposable_stores(at)，上層模組 join 的同一個函式），依 store_id。exposure.Classify 用它，
-- 所以 seller 自己的公開讀取與上層模組的 SQL 在同一刻得到同一個答案。sqlc 不型別化複合型別的欄位，所以函式放在 IN 子查詢裡。
SELECT s.store_id
FROM seller.stores s
WHERE s.store_id = ANY(sqlc.arg(store_ids)::bigint[])
  AND s.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
ORDER BY s.store_id;
