-- name: GetPost :one
SELECT * FROM cms_post WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: ListPosts :many
-- 公開／買家側：只回已發布。少了這條述詞，代行者按「儲存草稿」建立的貼文會當場
-- 進入買家 feed，而 admin UI 同時顯示它是草稿。
-- 碼值由呼叫端帶入（codemeta.PostPublishStatusPublished），SQL 內不寫字面值。
SELECT * FROM cms_post
WHERE deleted_at IS NULL
  AND hidden_at IS NULL
  AND publish_status = sqlc.arg(published_status)::smallint
  AND ($1::bigint = 0 OR seller_id = $1)
ORDER BY create_time DESC
LIMIT $2 OFFSET $3;

-- name: CountPosts :one
-- 與 ListPosts 同一組述詞。
SELECT COUNT(*) FROM cms_post
WHERE deleted_at IS NULL
  AND hidden_at IS NULL
  AND publish_status = sqlc.arg(published_status)::smallint
  AND ($1::bigint = 0 OR seller_id = $1);

-- name: ListMyPosts :many
-- 賣家自己的清單：不過濾發布狀態（賣家必須看得到自己的草稿與已下架貼文），但
-- 'published' 這個篩選值必須真的問 publish_status。曾經只問 hidden_at IS NULL，而
-- 草稿與已下架的 hidden_at 本來就是 NULL（那一欄記的是平台強制下架）⇒ 賣家挑「已
-- 發布」會撈到自己從未發布的貼文，而畫面上沒有任何東西會說它不對。
-- 碼值由呼叫端帶入（codemeta.PostPublishStatusPublished），SQL 內不寫字面值。
SELECT * FROM cms_post
WHERE seller_id = $1
  AND deleted_at IS NULL
  AND (
    $2::text = ''
    OR ($2 = 'published' AND publish_status = sqlc.arg(published_status)::smallint AND hidden_at IS NULL)
    OR ($2 = 'hidden'    AND hidden_at IS NOT NULL)
  )
ORDER BY create_time DESC
LIMIT $3 OFFSET $4;

-- name: CountMyPosts :one
-- 與 ListMyPosts 同一組述詞（兩支必須逐字一致，否則分頁總數與內容對不上）。
SELECT COUNT(*) FROM cms_post
WHERE seller_id = $1
  AND deleted_at IS NULL
  AND (
    $2::text = ''
    OR ($2 = 'published' AND publish_status = sqlc.arg(published_status)::smallint AND hidden_at IS NULL)
    OR ($2 = 'hidden'    AND hidden_at IS NOT NULL)
  );

-- name: AdminListPosts :many
-- 'published' 的判準與 ListMyPosts 逐條相同，理由也相同：草稿與賣家自行下架的貼文，
-- hidden_at 本來就是 NULL（那一欄只記平台強制下架）⇒ 只問 hidden_at 會讓管理員的
-- 「上架中」分頁撈到從未發布與已被撤下的貼文，而畫面上沒有任何東西會說它不對。
-- 碼值由呼叫端帶入（domain.PostPublishStatusPublished），SQL 內不寫字面值。
--
-- 'hidden' 維持只問 hidden_at：平台強制下架與賣家自行下架（4403）是兩件事，申訴管道
-- 不同（見 CodeMeta PostPublishStatus），賣家側同一支述詞亦然。
SELECT
    p.id, p.seller_id, p.content, p.image, p.product_id,
    p.like_count, p.comment_count, p.view_count, p.share_count,
    p.hidden_at, p.hidden_reason, p.deleted_at, p.create_time, p.update_time,
    COALESCE(rc.cnt, 0)::int             AS report_count,
    COALESCE(rl.reason_code, '')         AS latest_report_reason,
    rl.create_time                       AS latest_report_at
FROM cms_post p
LEFT JOIN (
    -- reporter_id != 0 excludes synthetic admin-direct takedowns (reporter_id=0
    -- sentinel from CreateAdminReport) so 檢舉數 counts real user reports only.
    SELECT target_id AS post_id, COUNT(*)::int AS cnt
    FROM reports
    WHERE target_type = 'POST' AND status != 'DISMISSED' AND reporter_id != 0
    GROUP BY target_id
) rc ON rc.post_id = p.id
LEFT JOIN (
    SELECT DISTINCT ON (target_id) target_id AS post_id, reason_code, create_time
    FROM reports
    WHERE target_type = 'POST' AND reporter_id != 0
    ORDER BY target_id, create_time DESC
) rl ON rl.post_id = p.id
WHERE (
    $1::text = ''
    OR ($1 = 'published' AND p.deleted_at IS NULL AND p.hidden_at IS NULL
                         AND p.publish_status = sqlc.arg(published_status)::smallint)
    OR ($1 = 'hidden'    AND p.deleted_at IS NULL AND p.hidden_at IS NOT NULL)
    OR ($1 = 'deleted'   AND p.deleted_at IS NOT NULL)
)
ORDER BY COALESCE(rl.create_time, p.create_time) DESC
LIMIT $2 OFFSET $3;

-- name: AdminCountPosts :one
-- 與 AdminListPosts 同一組述詞（兩支必須逐字一致，否則分頁總數與內容對不上）。
SELECT COUNT(*) FROM cms_post
WHERE (
    $1::text = ''
    OR ($1 = 'published' AND deleted_at IS NULL AND hidden_at IS NULL
                         AND publish_status = sqlc.arg(published_status)::smallint)
    OR ($1 = 'hidden'    AND deleted_at IS NULL AND hidden_at IS NOT NULL)
    OR ($1 = 'deleted'   AND deleted_at IS NOT NULL)
);

-- name: HidePost :exec
UPDATE cms_post
SET hidden_at = NOW(), hidden_reason = $2, update_time = NOW(), version = version + 1
WHERE id = $1 AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: RestorePost :exec
UPDATE cms_post
SET hidden_at = NULL, hidden_reason = NULL, update_time = NOW(), version = version + 1
WHERE id = $1 AND deleted_at IS NULL AND hidden_at IS NOT NULL;

-- name: CreatePost :one
-- publish_status 顯式傳入，不吃 DB DEFAULT：DEFAULT 是 4402（已發布），因為 migration
-- 當下每一篇既有貼文語意上都是已發布。代行建立一律是草稿（§6.1.9），靠 DEFAULT 就會
-- 反過來——貼文當場出現在買家 feed，而 admin UI 同時顯示它是草稿，兩邊都不報錯。
--
-- published_at 由 publish_status 就地導出，不另開一個獨立參數：兩者各自傳入時，呼叫端
-- 就有機會寫出「已發布但沒有發布時點」——那正是 migration 回填要修掉的形狀，而回填只
-- 作用於既有列，新建的每一篇都會重新長出同一個破口。
-- 草稿（與任何非已發布狀態）留 NULL：它的發布時點要等排程 worker 或 publish 端點寫。
--
-- image_asset_id 是這一列宣告的媒體引用真相，寫入路徑只負責把它記下來，不必額外留
-- 待辦記號：引用由 media reconciler 每輪重放全部已納管列來收斂（level-triggered）。
-- Link 本來就已在 commit 前完成，所以這一列在 reconciler 第一次看到它之前就已經收斂，
-- 重放只是再確認一次。
INSERT INTO cms_post (id, seller_id, content, image, product_id, publish_status, published_at,
                      image_asset_id)
VALUES (
  sqlc.arg(id), sqlc.arg(seller_id), sqlc.arg(content), sqlc.arg(image), sqlc.arg(product_id),
  sqlc.arg(publish_status),
  CASE WHEN sqlc.arg(publish_status)::smallint = sqlc.arg(published_status)::smallint
       THEN NOW() END,
  sqlc.arg(image_asset_id)
)
RETURNING *;

-- name: UpdatePost :execrows
-- #2772：CAS 在這支查詢上是強制的，沒有「停用比較」的逃生門——不像下面
-- SetPostPublishStatus／SchedulePostPublish 那樣接受負值哨兵當「不比較」。
-- expected_version 必須等於現行 version 才會有 rows 被改到；呼叫端（UpdatePost
-- usecase）保證一律傳進入時讀到的真實 version，理論上不會是負值，但即使有什麼
-- 東西不小心傳了負值進來，這裡也只會讓述詞恆假、rows = 0，fail closed 成版本
-- 衝突，而不是像那兩支一樣把它當成合法的「不比較」訊號悄悄放行。
-- rows = 0 由呼叫端轉 9266，一律視為版本衝突，不得退化成「再查一次然後覆蓋」。
--
-- image_asset_id 就地覆寫成新資產，這裡不必也不該留任何「待釋放舊資產」的記號：
-- 引用由 media reconciler 每輪重放全部已納管列來收斂（level-triggered），下一輪讀到
-- 這一欄的新值就會把換下來的舊引用釋放（#2772）。所以這支查詢唯一的責任是把
-- 真相寫進去；漏寫 image_asset_id 才是舊引用永遠釋放不掉的成因。
UPDATE cms_post
SET content = $2, image = $3, product_id = $4, image_asset_id = sqlc.arg(image_asset_id),
    update_time = NOW(), version = version + 1
WHERE id = $1 AND deleted_at IS NULL
  AND version = sqlc.arg(expected_version)::bigint;

-- name: SetPostPublishStatus :execrows
-- 互動式的發布／下架命令（56 §6.1.9 的 /publish 立即模式與 /unpublish）。
--
-- expected_version < 0 停用 CAS（賣家自己 publish/unpublish 時，delegation.
-- ParseExpectedVersion 對空字串回 nil，一路傳到這裡）；>= 0 必須與現行版本相符。
-- 負值哨兵安全的理由同 sellers：chk_cms_post_version_nonneg 保證 version >= 0。
-- rows = 0 由呼叫端轉 9266，一律視為版本衝突。#2772 之後這支的語意不再等於
-- UpdatePost——UpdatePost 已經拿掉停用分支，這支仍然合法接受 nil。
--
-- published_at 只在轉為「已發布」時重寫；下架不清它，那一欄記的是最後一次發布的時點，
-- 清掉之後「卡住的排程」偵測述詞（published_at IS NULL，§7.6.2）會把一則發布過又下架的
-- 貼文誤判成從未發布。
--
-- 四個 schedule 欄的 CASE 是把尚未執行的排程一併結束：明確的狀態命令覆蓋了排程要做的事，
-- 留著 pending 會讓 worker 到點再發一次——第二筆 POST_PUBLISH 稽核、第二則賣家通知，
-- 而 §5.1.5 的收斂三條都不會攔（貼文既沒被刪也沒被下架）。更關鍵的是 §5.1.5 對
-- 「pending 的列 publish_status 恆為 4401」有依賴（hashtag 計數的「減無可減」論證即此），
-- 4402 加上 pending 會讓 worker 的 +1 對一則已計數的貼文再加一次。
-- 終態取 done 而不是三個 cancelled_*：那三個記的是**外力**（撤銷／刪除／治理下架），
-- 而這裡是排程的擁有者親自下命令結束它，取消的原因由 account-service 那一列稽核承擔。
-- SET 讀的是更新前的列值，所以四個 CASE 的述詞一致。
UPDATE cms_post
   SET publish_status = sqlc.arg(publish_status)::smallint,
       published_at = CASE WHEN sqlc.arg(publish_status)::smallint = sqlc.arg(published_status)::smallint
                           THEN sqlc.arg(now)::timestamptz ELSE published_at END,
       schedule_state            = CASE WHEN schedule_state = 'pending' THEN 'done' ELSE schedule_state END,
       schedule_lease_owner      = CASE WHEN schedule_state = 'pending' THEN NULL ELSE schedule_lease_owner END,
       schedule_lease_expires_at = CASE WHEN schedule_state = 'pending' THEN NULL ELSE schedule_lease_expires_at END,
       schedule_next_attempt_at  = CASE WHEN schedule_state = 'pending' THEN NULL ELSE schedule_next_attempt_at END,
       update_time = sqlc.arg(now)::timestamptz,
       version = version + 1
 WHERE id = sqlc.arg(id)
   AND deleted_at IS NULL
   AND (sqlc.arg(expected_version)::bigint < 0 OR version = sqlc.arg(expected_version)::bigint);

-- name: SchedulePostPublish :execrows
-- 建立（或改期）排程發布（56 §7.6.1 的 T0）。publish_status 刻意不動：排程期間貼文仍是
-- 草稿，到點由 worker 的終寫轉成已發布。
--
-- 述詞帶 publish_status <> 已發布：已發布的貼文再排一次發布，到點時 worker 會對一則
-- 已計數的貼文再 +1 一次 hashtag 計數（終寫的 sameTx 是無條件 +1，它靠的正是
-- 「pending ⇒ 草稿」這個不變式）。呼叫端先讀一次給出可讀的錯誤，這一句是防兩者之間
-- 那個窗口——worker 的終寫不 bump version，所以樂觀鎖擋不住它。
--
-- retry/lease/next_attempt 一律歸零：改期是一次新的排程，沿用上一次的重試預算會讓
-- 新排程帶著舊的失敗次數進場，離耗盡更近一步而沒有任何地方說得出為什麼。
UPDATE cms_post
   SET scheduled_publish_at      = sqlc.arg(scheduled_publish_at)::timestamptz,
       scheduled_by_session_id   = sqlc.arg(scheduled_by_session_id)::bigint,
       schedule_state            = 'pending',
       schedule_lease_owner      = NULL,
       schedule_lease_expires_at = NULL,
       schedule_next_attempt_at  = NULL,
       schedule_retry_count      = 0,
       update_time = sqlc.arg(now)::timestamptz,
       version = version + 1
 WHERE id = sqlc.arg(id)
   AND deleted_at IS NULL
   AND publish_status <> sqlc.arg(published_status)::smallint
   AND (sqlc.arg(expected_version)::bigint < 0 OR version = sqlc.arg(expected_version)::bigint);

-- name: SoftDeletePost :execrows
-- affected rows 是授權判定的一部分，不是統計值：述詞含 seller_id，rows = 0 表示
-- 「這篇貼文不屬於這個賣家」或「它已經被刪掉了」。兩者對呼叫端是同一個結論——
-- 這次刪除沒有發生，counter/cache/media 副作用一個都不能做（#2757）。
-- 改成 :exec 等於把授權結果丟掉，那正是本票的成因。
UPDATE cms_post SET deleted_at = NOW(), update_time = NOW()
WHERE id = $1 AND seller_id = $2 AND deleted_at IS NULL;

-- name: IncrPostViewCount :exec
UPDATE cms_post SET view_count = view_count + 1 WHERE id = $1;

-- name: IncrPostShareCount :exec
UPDATE cms_post SET share_count = share_count + 1 WHERE id = $1;

-- name: IncrPostLikeCount :exec
UPDATE cms_post SET like_count = like_count + 1, update_time = NOW() WHERE id = $1;

-- name: DecrPostLikeCount :exec
UPDATE cms_post SET like_count = GREATEST(0, like_count - 1), update_time = NOW() WHERE id = $1;

-- name: IncrPostCommentCount :exec
UPDATE cms_post SET comment_count = comment_count + 1, update_time = NOW() WHERE id = $1;

-- name: DecrPostCommentCount :exec
UPDATE cms_post SET comment_count = GREATEST(0, comment_count - 1), update_time = NOW() WHERE id = $1;

-- name: GetPostLike :one
SELECT * FROM cms_post_like WHERE post_id = $1 AND actor_kind = $2 AND user_id = $3 LIMIT 1;

-- name: CreatePostLike :exec
INSERT INTO cms_post_like (post_id, actor_kind, user_id) VALUES ($1, $2, $3)
ON CONFLICT (post_id, actor_kind, user_id) DO NOTHING;

-- name: DeletePostLike :exec
DELETE FROM cms_post_like WHERE post_id = $1 AND actor_kind = $2 AND user_id = $3;

-- name: BatchGetPostLikes :many
SELECT post_id FROM cms_post_like
WHERE post_id = ANY($1::bigint[]) AND actor_kind = $2 AND user_id = $3;

-- name: DeletePostLikesByUserID :exec
DELETE FROM cms_post_like WHERE actor_kind = $1 AND user_id = $2;

-- name: ReconcilePostLikeCounts :exec
-- LEFT JOIN from cms_post so posts whose likes all dropped to 0 still get a row
-- (cnt=0) and get reconciled. A GROUP BY on cms_post_like alone omits zero-like
-- posts entirely, leaving their stale count stuck forever (#1171).
-- ponytail: full-table scan every tick; add "AND p.update_time > last_run" if it hurts.
UPDATE cms_post p SET like_count = sub.cnt
FROM (
    SELECT p2.id AS post_id, COUNT(l.post_id)::int AS cnt
    FROM cms_post p2
    LEFT JOIN cms_post_like l ON l.post_id = p2.id
    WHERE p2.deleted_at IS NULL
    GROUP BY p2.id
) sub
WHERE p.id = sub.post_id AND p.like_count != sub.cnt;

-- name: ReconcilePostCommentCounts :exec
-- comment_count = top-level comments only (parent_id = 0), matching the comment
-- list API's total (#1171). LEFT JOIN so posts whose top-level comments all got
-- deleted (e.g. via account.deleted bulk soft-delete) reconcile back to 0 instead
-- of staying stuck — a GROUP BY on cms_comment alone can never emit a 0 row.
UPDATE cms_post p SET comment_count = sub.cnt
FROM (
    SELECT p2.id AS post_id, COUNT(c.id)::int AS cnt
    FROM cms_post p2
    LEFT JOIN cms_comment c ON c.post_id = p2.id AND c.parent_id = 0 AND c.deleted_at IS NULL
    WHERE p2.deleted_at IS NULL
    GROUP BY p2.id
) sub
WHERE p.id = sub.post_id AND p.comment_count != sub.cnt;

-- name: SoftDeletePostsBySellerID :exec
UPDATE cms_post SET deleted_at = NOW(), update_time = NOW()
WHERE seller_id = $1 AND deleted_at IS NULL;

-- name: ListPostIDsBySellerID :many
-- 帳號終止時要釋放媒體引用的對象（seller.deleted handler，Will 2026-09-05：
-- 賣家刪除後貼文圖片不用留）。
--
-- 為什麼不是在 SoftDeletePostsBySellerID 上加 RETURNING id（少一次往返）：那支的
-- 述詞是 deleted_at IS NULL，MQ 重投時第二次執行影響 0 列 ⇒ 回傳空集合 ⇒ 釋放被
-- 整個跳過。handler 的四個操作之所以能「失敗就 return err、交給既有 retry／DLQ」，
-- 前提正是整支重跑無害；RETURNING 會讓「軟刪成功、後面某一步失敗」的重投永遠補
-- 不上釋放，引用永久殘留。這支的述詞不看 deleted_at，重跑幾次都回同一個集合。
--
-- 刻意不過濾 image_asset_id IS NOT NULL：ReleaseReferences 是依 (ref_type, ref_id)
-- 刪引用，不需要 asset id。未納管的 legacy 列（image 有值、image_asset_id 尚未回填）
-- 在 media 上有既有引用，卻正好是這裡讀不到 asset id 的那些——濾掉它們就是把唯一
-- 能釋放它們的路徑也關掉。
SELECT id FROM cms_post WHERE seller_id = $1 ORDER BY id;

-- name: ListPostsByHashtagName :many
-- p.* 而非明列欄位：明列清單是資料表的嚴格子集 ⇒ sqlc 會為這一支生一個獨立的
-- ListPostsByHashtagNameRow，呼叫端（post_repository.ListByHashtag）當場編譯失敗。
-- p.* 讓回傳型別與其餘四支整列查詢一致，同為 CmsPost。
SELECT p.*
FROM cms_post p
JOIN cms_post_hashtag ph ON ph.post_id = p.id
JOIN cms_hashtag h       ON h.id = ph.hashtag_id
WHERE h.name = $1
  AND p.deleted_at IS NULL AND p.hidden_at IS NULL
  AND p.publish_status = sqlc.arg(published_status)::smallint
ORDER BY p.create_time DESC
LIMIT $2 OFFSET $3;

-- name: CountPostsByHashtagName :one
SELECT COUNT(*)
FROM cms_post p
JOIN cms_post_hashtag ph ON ph.post_id = p.id
JOIN cms_hashtag h       ON h.id = ph.hashtag_id
WHERE h.name = $1
  AND p.deleted_at IS NULL AND p.hidden_at IS NULL
  AND p.publish_status = sqlc.arg(published_status)::smallint;
