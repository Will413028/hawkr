-- 排程發布 worker 的全部 SQL（56 §5.1.5 / §7.6）。
--
-- 獨立成檔而不併進 posts.sql：這一組的讀者是 worker，述詞彼此耦合（收斂／候選／
-- claim 三者共用同一組窗口與可見性條件），混在賣家與買家側的查詢之間會讓「改了其中
-- 一句忘了另外兩句」變得容易——而那正是 at-most-once 破掉的方式。
--
-- 窗口長度一律走參數，禁止寫 INTERVAL '24 hours' 字面值：硬寫之後 env 會被讀取、被
-- Validate()、進 k8s values，卻對行為零影響——一個「看起來可調、實際上不可調」的設定
-- 比沒有設定更危險（§5.1.5 L1815）。

-- ①-a 賣家自刪／整店退場。deleted_at 不可逆，優先於下架。
--
-- 三條收斂刻意不帶租約守衛：收斂語意是「這一列已經失效，誰持有它都不算數」。加上
-- `AND schedule_lease_owner IS NULL` 只會讓正被某個 pod 持有的失效列跳過收斂、停在
-- pending 等租約到期，而那個 pod 隨後照樣把它發出去——把偶發變成必然。
-- 與「失敗釋放語句必須帶守衛」不矛盾：釋放是還原自己的租約，動到別人的就是搶。
-- name: CancelSchedulesForDeletedPosts :execrows
UPDATE cms_post
   SET schedule_state = 'cancelled_by_deletion',
       schedule_lease_owner = NULL,
       schedule_lease_expires_at = NULL,
       update_time = sqlc.arg(now)::timestamptz
 WHERE schedule_state = 'pending'
   AND deleted_at IS NOT NULL;

-- ①-b 平台強制下架。與 ①-a 互斥（帶 deleted_at IS NULL），否則同一列會被兩條各記一次。
--
-- 與 ①-a 刻意拆成兩句而不是一句 CASE：各自的 RowsAffected 要餵 OBS10 各自的 reason
-- label，寫成一句就只剩一個計數，另一個原因永遠靜默。
-- name: CancelSchedulesForHiddenPosts :execrows
UPDATE cms_post
   SET schedule_state = 'cancelled_by_governance',
       schedule_lease_owner = NULL,
       schedule_lease_expires_at = NULL,
       update_time = sqlc.arg(now)::timestamptz
 WHERE schedule_state = 'pending'
   AND deleted_at IS NULL
   AND hidden_at IS NOT NULL;

-- ①-c 逾候選窗口。
--
-- 沒有窗口的話，worker 停機三天後恢復會一次發出三天前排定的所有貼文——對賣家（內容
-- 過時）與買家（時間軸被灌爆）都是傷害。
-- ①-a／①-b 刻意沒有這個時間下限：下個月才要發的貼文今天被刪除，排程就該今天取消。
-- name: ExpireSchedulesOutsideWindow :execrows
UPDATE cms_post
   SET schedule_state = 'expired',
       schedule_lease_owner = NULL,
       schedule_lease_expires_at = NULL,
       update_time = sqlc.arg(now)::timestamptz
 WHERE schedule_state = 'pending'
   AND scheduled_publish_at <= sqlc.arg(now)::timestamptz - make_interval(hours => sqlc.arg(window_hours)::int);

-- ② 候選 SELECT。
--
-- deleted_at／hidden_at 兩個述詞不是 ① 的重複防禦，是不同時點的兩個問題：① 處理
-- 「跑收斂的那一刻已經被刪／被下架」，②③ 處理「收斂跑完到 claim 之間才被刪／被下架」
-- 的那幾毫秒。
-- name: ListDueScheduledPosts :many
SELECT id
  FROM cms_post
 WHERE schedule_state = 'pending'
   AND scheduled_publish_at <= sqlc.arg(now)::timestamptz
   AND scheduled_publish_at > sqlc.arg(now)::timestamptz - make_interval(hours => sqlc.arg(window_hours)::int)
   AND (schedule_next_attempt_at IS NULL OR schedule_next_attempt_at <= sqlc.arg(now)::timestamptz)
   AND (schedule_lease_expires_at IS NULL OR schedule_lease_expires_at <= sqlc.arg(now)::timestamptz)
   AND deleted_at IS NULL
   AND hidden_at IS NULL
 ORDER BY scheduled_publish_at
 LIMIT sqlc.arg(row_limit)::int;

-- ③ 單列 CAS claim。租約基準（now 與 lease_expires_at）由呼叫端**逐筆**取，不可在批次
-- 迴圈外算一次——那會讓不變式從「單筆最壞耗時 < 租約」退化成「批次總耗時 < 租約」，
-- 隨 batch-size 一起破（#2294 的形狀）。
--
-- RETURNING * 而不是「先 UPDATE 再 SELECT」：後者的兩句之間，收斂語句或另一個 pod
-- 都可能動這一列，讀回來的欄位就不是被 claim 的那個版本，而冪等鍵正是由
-- scheduled_publish_at 導出的。pgx.ErrNoRows ＝ 沒搶到（或該列已不合資格）。
-- name: ClaimScheduledPost :one
UPDATE cms_post
   SET schedule_lease_owner = sqlc.arg(owner)::text,
       schedule_lease_expires_at = sqlc.arg(lease_expires_at)::timestamptz,
       update_time = sqlc.arg(now)::timestamptz
 WHERE id = sqlc.arg(id)
   AND schedule_state = 'pending'
   AND scheduled_publish_at <= sqlc.arg(now)::timestamptz
   AND scheduled_publish_at > sqlc.arg(now)::timestamptz - make_interval(hours => sqlc.arg(window_hours)::int)
   AND (schedule_next_attempt_at IS NULL OR schedule_next_attempt_at <= sqlc.arg(now)::timestamptz)
   AND (schedule_lease_expires_at IS NULL OR schedule_lease_expires_at <= sqlc.arg(now)::timestamptz)
   AND deleted_at IS NULL
   AND hidden_at IS NULL
RETURNING *;

-- T2 終寫。第四件讀時防禦，也是五個不變式的第五條。
--
-- 前三件（收斂／候選／claim）都只防到 claim 當下，而 claim 到這裡還隔著 intent 與
-- 發布 tx 的預算；那個窗口只有這一句守得住，且它在單副本下同樣存在——賣家自刪與
-- worker 是兩個進程。
--
-- 不帶 version 述詞、也不 bump version：排程是「到點發布」不是「到點覆蓋內容」，
-- 賣家在等待期間繼續編輯完全合法，發出去的就是他最後編輯的版本。併發安全全部由
-- 租約 CAS 承擔，與資源樂觀鎖是兩套互不相干的機制。
-- name: PublishScheduledPost :execrows
UPDATE cms_post
   SET publish_status = sqlc.arg(publish_status)::smallint,
       published_at = sqlc.arg(now)::timestamptz,
       schedule_state = 'done',
       schedule_lease_owner = NULL,
       schedule_lease_expires_at = NULL,
       schedule_next_attempt_at = NULL,
       update_time = sqlc.arg(now)::timestamptz
 WHERE id = sqlc.arg(id)
   AND schedule_state = 'pending'
   AND schedule_lease_owner = sqlc.arg(owner)::text
   AND deleted_at IS NULL
   AND hidden_at IS NULL;

-- 失敗釋放：清租約 ＋ 累加 retry_count ＋ 推遲下次嘗試。
--
-- 述詞必須帶 schedule_lease_owner ＝ 自己：租約若已被別人接手，清它就是搶。
-- rows = 0 不是錯誤。
-- name: ReleaseScheduledPostLease :execrows
UPDATE cms_post
   SET schedule_lease_owner = NULL,
       schedule_lease_expires_at = NULL,
       schedule_retry_count = schedule_retry_count + 1,
       schedule_next_attempt_at = sqlc.arg(next_attempt_at)::timestamptz,
       update_time = sqlc.arg(now)::timestamptz
 WHERE id = sqlc.arg(id)
   AND schedule_state = 'pending'
   AND schedule_lease_owner = sqlc.arg(owner)::text;

-- 重試耗盡 → expired（OBS8 的 retry_exhausted）。與逾窗口是同一個終態值、兩個來源，
-- 兩者都必須累加指標，只記其中一條另一條就變成「貼文永遠不發布、賣家不知情、零告警」。
-- name: ExpireExhaustedScheduledPost :execrows
UPDATE cms_post
   SET schedule_state = 'expired',
       schedule_lease_owner = NULL,
       schedule_lease_expires_at = NULL,
       schedule_next_attempt_at = NULL,
       schedule_retry_count = schedule_retry_count + 1,
       update_time = sqlc.arg(now)::timestamptz
 WHERE id = sqlc.arg(id)
   AND schedule_state = 'pending'
   AND schedule_lease_owner = sqlc.arg(owner)::text;

-- 工作階段已被撤銷（RecordAuditIntent 回 9271）→ 取消排程。
-- 撤銷有三個來源（人工撤銷／ADM 登出連動／目標店家轉為不可代行），worker 端只有這
-- 一條分支；要分辨得讀 session 列的 revoke_reason。
-- name: CancelScheduledPostByRevocation :execrows
UPDATE cms_post
   SET schedule_state = 'cancelled_by_revocation',
       schedule_lease_owner = NULL,
       schedule_lease_expires_at = NULL,
       schedule_next_attempt_at = NULL,
       update_time = sqlc.arg(now)::timestamptz
 WHERE id = sqlc.arg(id)
   AND schedule_state = 'pending'
   AND schedule_lease_owner = sqlc.arg(owner)::text;

-- ⑦ 租約歸屬探針（唯讀）。
--
-- 終寫 CAS 落空（rows = 0）有兩個成因，而它們要的處置相反：
--   ⓐ 租約被別的 pod 接手 ⇒ 那個 pod 會用同一個冪等鍵拿到同一列稽核，本 pod 若搶著寫
--      FAILED 會擋掉它隨後的 SUCCEEDED（confirm 的述詞是 result='PENDING' 先到先贏），
--      稽核記成失敗、outbox 一列不建、賣家收不到通知，而貼文其實已經發出去了。
--   ⓑ 租約仍在我手上 ⇒ 沒有接手者，rows = 0 必然是另外三個述詞之一不成立（已刪除／
--      已下架／已收斂轉態），該列不會再回到候選集合。這時**必須**寫終態，否則那列稽核
--      永久停在 PENDING。
--
-- 不比 schedule_lease_expires_at：問的是「這一列現在的 owner 還是不是我」。租約過期但
-- 沒被接手時 owner 仍是我 —— 沒有接手者，寫終態依然安全。三條收斂與 Publish 成功都會
-- 把 owner 清成 NULL，那些情況一律回 false，退回「不寫」的保守側。
-- name: IsScheduledPostLeaseHeldBy :one
SELECT EXISTS (
  SELECT 1
    FROM cms_post
   WHERE id = sqlc.arg(id)
     AND schedule_lease_owner = sqlc.arg(owner)::text
)::boolean;
