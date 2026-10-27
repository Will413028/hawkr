-- media_asset.object_key 唯一化
--
-- 背景：FinalizeImages 舊實作是「先 INSERT media_asset，再對 upload_session 做狀態 CAS」。
-- 同一個 session 被並發轉正時（前端重複送出），輸掉 CAS 的那一方已經 INSERT 完成，
-- 而它拿到的 finalObjectKey 由來源 key 決定、同 session 恆等 —— 於是留下兩列指向
-- 同一個 object_key 的 media_asset（dev 實測到相隔 117ms 的一對）。
--
-- 危害：MediaGarbageCollectionJob 回收「無引用」的 media_asset 時是照 object_key 刪 R2 物件
-- （MediaAssetRepository.selectOrphanedAssets → storageService.deleteObject）。共用 key 的兩列
-- 只要有一列失去引用被回收，R2 上的物件就被刪掉，另一列（仍被業務引用）當場變成死指標。
--
-- code 端已改成 CAS 先行（MediaAssetService.commitFinalize），本檔補上 DB 層的機械保證：
-- 先把既有重複列併回保留列，再加唯一索引。
--
-- 實作註記：保留列的判定刻意寫成每個語句各自重算的 CTE（不用 TEMP TABLE），
-- 好讓本檔不依賴「Flyway 是否把整個 migration 包在單一交易內」這個預設值。
-- 判定結果在下列語句序中是穩定的：保留列的排序鍵（有無 upload_session/import_asset 指向、
-- asset_reference 數）只會往「更像保留列」的方向變動，待併列只會往反方向變動，故名次不會翻轉；
-- 唯一會改動 media_asset 本身的語句（DELETE）排在最後。
--
-- 保留優先序刻意對齊「並發轉正的贏家」——upload_session.asset_id /
-- import_asset.finalized_asset_id 回填的是贏家，而業務表（跨庫，本 migration 看不到）
-- 拿到的通常也是它；其次比引用數，最後才比建立時間。這樣即使跨庫欄位指著某一列，
-- 被刪掉的機率最低。
--
-- blast radius（2026-08-06 實查）：dev 1 組（刪 1 列）、prod 0 組（只建索引，不刪任何列）。

-- 1) asset_reference 改指保留列。
--    asset_reference 有 UNIQUE(asset_id, ref_type, ref_id)，故只搬「搬過去不會撞」的那些；
--    同一組 (keep_id, ref_type, ref_id) 也只留一列，避免多重重複時一次 UPDATE 自撞。
WITH keep AS (
    SELECT DISTINCT ON (m.object_key) m.object_key, m.id AS keep_id
    FROM media_asset m
    ORDER BY m.object_key,
             (EXISTS (SELECT 1 FROM upload_session us WHERE us.asset_id = m.id)
              OR EXISTS (SELECT 1 FROM import_asset ia WHERE ia.finalized_asset_id = m.id)) DESC,
             (SELECT count(*) FROM asset_reference ar WHERE ar.asset_id = m.id) DESC,
             m.created_at,
             m.id
), dup AS (
    SELECT ma.id AS dup_id, k.keep_id
    FROM media_asset ma
    JOIN keep k ON k.object_key = ma.object_key
    WHERE ma.id <> k.keep_id
), movable AS (
    SELECT DISTINCT ON (d.keep_id, ar.ref_type, ar.ref_id) ar.id AS ref_row_id, d.keep_id
    FROM asset_reference ar
    JOIN dup d ON ar.asset_id = d.dup_id
    WHERE NOT EXISTS (
        SELECT 1 FROM asset_reference ex
        WHERE ex.asset_id = d.keep_id
          AND ex.ref_type = ar.ref_type
          AND ex.ref_id = ar.ref_id
    )
    ORDER BY d.keep_id, ar.ref_type, ar.ref_id, ar.id
)
UPDATE asset_reference ar
SET asset_id = m.keep_id
FROM movable m
WHERE ar.id = m.ref_row_id;

-- 2) 搬不過去的（保留列已有同一組引用）＝重複引用，刪除。
WITH keep AS (
    SELECT DISTINCT ON (m.object_key) m.object_key, m.id AS keep_id
    FROM media_asset m
    ORDER BY m.object_key,
             (EXISTS (SELECT 1 FROM upload_session us WHERE us.asset_id = m.id)
              OR EXISTS (SELECT 1 FROM import_asset ia WHERE ia.finalized_asset_id = m.id)) DESC,
             (SELECT count(*) FROM asset_reference ar WHERE ar.asset_id = m.id) DESC,
             m.created_at,
             m.id
), dup AS (
    SELECT ma.id AS dup_id
    FROM media_asset ma
    JOIN keep k ON k.object_key = ma.object_key
    WHERE ma.id <> k.keep_id
)
DELETE FROM asset_reference ar
USING dup d
WHERE ar.asset_id = d.dup_id;

-- 3) upload_session.asset_id 改指保留列。
WITH keep AS (
    SELECT DISTINCT ON (m.object_key) m.object_key, m.id AS keep_id
    FROM media_asset m
    ORDER BY m.object_key,
             (EXISTS (SELECT 1 FROM upload_session us WHERE us.asset_id = m.id)
              OR EXISTS (SELECT 1 FROM import_asset ia WHERE ia.finalized_asset_id = m.id)) DESC,
             (SELECT count(*) FROM asset_reference ar WHERE ar.asset_id = m.id) DESC,
             m.created_at,
             m.id
), dup AS (
    SELECT ma.id AS dup_id, k.keep_id
    FROM media_asset ma
    JOIN keep k ON k.object_key = ma.object_key
    WHERE ma.id <> k.keep_id
)
UPDATE upload_session us
SET asset_id = d.keep_id
FROM dup d
WHERE us.asset_id = d.dup_id;

-- 4) import_asset.finalized_asset_id 改指保留列。
WITH keep AS (
    SELECT DISTINCT ON (m.object_key) m.object_key, m.id AS keep_id
    FROM media_asset m
    ORDER BY m.object_key,
             (EXISTS (SELECT 1 FROM upload_session us WHERE us.asset_id = m.id)
              OR EXISTS (SELECT 1 FROM import_asset ia WHERE ia.finalized_asset_id = m.id)) DESC,
             (SELECT count(*) FROM asset_reference ar WHERE ar.asset_id = m.id) DESC,
             m.created_at,
             m.id
), dup AS (
    SELECT ma.id AS dup_id, k.keep_id
    FROM media_asset ma
    JOIN keep k ON k.object_key = ma.object_key
    WHERE ma.id <> k.keep_id
)
UPDATE import_asset ia
SET finalized_asset_id = d.keep_id
FROM dup d
WHERE ia.finalized_asset_id = d.dup_id;

-- 5) 刪除待併列。R2 物件不動 —— 它與保留列共用同一個 object_key，刪了會讓保留列變死指標。
WITH keep AS (
    SELECT DISTINCT ON (m.object_key) m.object_key, m.id AS keep_id
    FROM media_asset m
    ORDER BY m.object_key,
             (EXISTS (SELECT 1 FROM upload_session us WHERE us.asset_id = m.id)
              OR EXISTS (SELECT 1 FROM import_asset ia WHERE ia.finalized_asset_id = m.id)) DESC,
             (SELECT count(*) FROM asset_reference ar WHERE ar.asset_id = m.id) DESC,
             m.created_at,
             m.id
), dup AS (
    SELECT ma.id AS dup_id
    FROM media_asset ma
    JOIN keep k ON k.object_key = ma.object_key
    WHERE ma.id <> k.keep_id
)
DELETE FROM media_asset ma
USING dup d
WHERE ma.id = d.dup_id;

-- 6) 唯一索引：往後任何「兩列一物件」都會在 INSERT 當下被擋下。
CREATE UNIQUE INDEX uk_media_asset_object_key ON media_asset (object_key);
