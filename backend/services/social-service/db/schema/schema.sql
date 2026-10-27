-- cms_post: 賣家貼文
CREATE TABLE IF NOT EXISTS cms_post (
    id            BIGINT PRIMARY KEY,
    seller_id     BIGINT NOT NULL,
    content       TEXT,
    image         TEXT,                        -- 單張圖片 URL（nullable）
    product_id    BIGINT,                      -- 關聯商品（nullable）
    like_count    INT NOT NULL DEFAULT 0,
    comment_count INT NOT NULL DEFAULT 0,
    view_count    INT NOT NULL DEFAULT 0,
    share_count   INT NOT NULL DEFAULT 0,
    hidden_at     TIMESTAMPTZ,                 -- 平台強制下架時間（非 null = hidden）
    hidden_reason TEXT,                        -- 下架原因
    deleted_at    TIMESTAMPTZ,
    create_time   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    update_time   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- 樂觀鎖版本（56 §5.4）。Go 端無框架級樂觀鎖，寫入路徑逐句手寫 version = version + 1。
    version                   BIGINT      NOT NULL DEFAULT 0,
    -- 發布狀態（PostPublishStatus CodeMeta：4401 草稿 / 4402 已發布 / 4403 已下架）。
    -- DEFAULT 取已發布碼：變更前每一篇貼文語意上都是已發布，取草稿碼會讓全站既有貼文
    -- 在一次 migration 內集體變成未發布。賣家自建走 DEFAULT，代行建草稿由 Gateway 顯式傳 4401。
    publish_status            SMALLINT    NOT NULL DEFAULT 4402,
    scheduled_publish_at      TIMESTAMPTZ,
    published_at              TIMESTAMPTZ,
    -- 排程任務狀態；NULL = 從未被排程。三個 cancelled_* 分開記：hidden_at 可被 RestorePost 清成
    -- NULL，事後反查會得到「看起來沒被下架過」的錯誤答案，終態值必須凍結取消當下的原因。
    schedule_state            TEXT,
    -- 排程歸屬的稽核鏈結；跨庫（account-service）故不下 FK，僅存值。
    scheduled_by_session_id   BIGINT,
    -- 純觀測用；正確性由 schedule_lease_expires_at 的 CAS 保證。
    schedule_lease_owner      TEXT,
    -- 冷卻租約，不是「已處理旗標」：到期後該列自動回到候選集合。
    schedule_lease_expires_at TIMESTAMPTZ,
    schedule_next_attempt_at  TIMESTAMPTZ,
    schedule_retry_count      INT         NOT NULL DEFAULT 0,

    -- 媒體引用生命週期（#2772/#2773/#2774/#2775）。真相是 image_asset_id，media 的
    -- asset_reference 是它的投影，由寫入路徑同步維持：① Link（commit 前，失敗即中止）
    -- → ② 寫這一列 → ③ Release（commit 後，best-effort），見 pkg/media.LinkImages 的
    -- 順序契約與 PostUseCase.UpdatePost。
    --
    -- ⚠️ cms_post 不得硬刪，理由見本表的 COMMENT ON TABLE。
    --
    -- image_asset_id 為 NULL 而 image 非 NULL = migration 前就存在、尚未回填的 legacy 列。
    -- 那種列在 media 上可能有引用，而這裡讀不到 asset id ⇒ 任何「照這一欄推導 desired」
    -- 的批次流程都會把它讀成空集合、釋放掉它的既有引用，資產隨後被 GC 實體刪除且不可逆。
    -- 寫入路徑不受影響（它只在使用者自己動到圖時才送 ③，desired 是當下的真值）；要當心的
    -- 是維運腳本，見 scripts/audit_media_reference_precheck.py 對這類列的分類。
    image_asset_id            BIGINT,

    CONSTRAINT chk_cms_post_version_nonneg CHECK (version >= 0),
    CONSTRAINT chk_cms_post_schedule_state CHECK (
        schedule_state IS NULL
        OR schedule_state IN (
             'pending', 'done', 'expired',
             'cancelled_by_revocation', 'cancelled_by_deletion', 'cancelled_by_governance'
           )
    ),
    CONSTRAINT chk_cms_post_schedule_pending_at CHECK (
        schedule_state IS DISTINCT FROM 'pending' OR scheduled_publish_at IS NOT NULL
    )
);
CREATE INDEX IF NOT EXISTS idx_cms_post_seller ON cms_post (seller_id, deleted_at, create_time DESC);
-- 排程 worker 每個 tick 的候選 SELECT 與 claim 都走這支 partial index。
CREATE INDEX IF NOT EXISTS idx_cms_post_schedule_pending
  ON cms_post (scheduled_publish_at)
  WHERE schedule_state = 'pending';
-- ⚠️ 這段 COMMENT ON TABLE 是硬刪禁令的**唯一**投放點，不要把它退化成 `--` 註解：
-- `--` 註解不會進資料庫，而會打出 DELETE FROM 的人正坐在 psql 前面看不到它。
COMMENT ON TABLE cms_post IS '本表不得硬刪（DELETE FROM）。image_asset_id 是這一列宣告的媒體引用真相，media 的 asset_reference 由寫入路徑同步維持：換圖／清圖時在 commit 之後送 ReleaseReferences 收斂（PostUseCase.UpdatePost）。硬刪之後就沒有任何一列能推導出「這個 ref_id 該掛哪些資產」，而 media 也沒有任何 RPC 能列舉某 ref_type 的全部引用——它在 media 上已建立的引用因此永久留存、事後無跡可循。軟刪保住那一列因而保住這個能力：資產欄位仍指著原資產，引用刻意保留不釋放；代價是軟刪列的資產永遠不會被回收，這是刻意接受的存儲成本，不是待修的洩漏。真正需要清除時由該情境自己的流程先釋放引用，帳號終止走 seller.deleted handler（internal/worker/seller_deleted.go）。見 #2772/#2773/#2774/#2775。';

-- reports: 通用 UGC 檢舉表（B-135 從 cms_post_report 擴充而來；2026-07 內容檢舉重構統一 A/B/D 三條路徑）
-- target_type: REVIEW | POST | POST_COMMENT | SELLER_FORUM | CHAT_MESSAGE | USER
-- reason_code: SPAM | SEXUAL | FRAUD | HARASSMENT | FOOD_SAFETY | POLITICAL | FAKE_NEWS | OTHER
-- status: PENDING | RESOLVED_HIDDEN | RESOLVED_WARNED | RESOLVED_BANNED | DISMISSED | APPEALED
-- resolved_action: HIDE_TARGET | WARN_USER | BAN_USER | DISMISS
CREATE TABLE IF NOT EXISTS reports (
    id              BIGINT PRIMARY KEY,
    reporter_id     BIGINT NOT NULL,
    target_type     VARCHAR(30) NOT NULL,
    target_id       BIGINT NOT NULL,
    target_owner_id BIGINT,
    reason_code     VARCHAR(30) NOT NULL,
    description     TEXT,
    evidence        JSONB NOT NULL DEFAULT '[]'::jsonb,
    target_snapshot JSONB,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    resolved_action VARCHAR(20),
    resolved_by     BIGINT,
    resolved_at     TIMESTAMPTZ,
    create_time     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    update_time     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_reports_target_type CHECK (target_type IN ('REVIEW','POST','POST_COMMENT','SELLER_FORUM','CHAT_MESSAGE','USER')),
    CONSTRAINT chk_reports_reason_code CHECK (reason_code IN ('SPAM','SEXUAL','FRAUD','HARASSMENT','FOOD_SAFETY','POLITICAL','FAKE_NEWS','OTHER')),
    CONSTRAINT chk_reports_status CHECK (status IN ('PENDING','RESOLVED_HIDDEN','RESOLVED_WARNED','RESOLVED_BANNED','DISMISSED','APPEALED')),
    -- 值域是 CodeMeta `ReportResolveAction`（四個終態裁決），刻意不是稽核 log 的
    -- `ModerationActionType`：後者是嚴格超集，多出的 AUTO_REJECT / REOPEN 不對應
    -- 任何 chk_reports_status 終態，寫進本欄會讓 actionToStatus 找不到映射。
    CONSTRAINT chk_reports_resolved_action CHECK (resolved_action IS NULL OR resolved_action IN ('HIDE_TARGET','WARN_USER','BAN_USER','DISMISS'))
);
CREATE INDEX IF NOT EXISTS idx_reports_status_create ON reports (status, create_time DESC);
CREATE INDEX IF NOT EXISTS idx_reports_reporter_target ON reports (reporter_id, target_type, target_id, create_time DESC);
CREATE INDEX IF NOT EXISTS idx_reports_target ON reports (target_type, target_id, create_time DESC);

COMMENT ON COLUMN reports.reporter_id IS '依當下 active role 存現任 buyer_id 或現任 store_id，皆非 raw account_id；僅查無現任 stint（ErrNoCurrentBuyer/ErrNoCurrentStore）時 fallback 用 account_id，見 2026-08-01 ADR（seller-content-moderation-identity-store-id-rekey）';

-- user_blocks: 使用者封鎖表（B-135）
CREATE TABLE IF NOT EXISTS user_blocks (
    id          BIGINT PRIMARY KEY,
    blocker_id  BIGINT NOT NULL,
    blocked_id  BIGINT NOT NULL,
    create_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (blocker_id, blocked_id),
    CONSTRAINT chk_user_blocks_not_self CHECK (blocker_id <> blocked_id)
);
CREATE INDEX IF NOT EXISTS idx_user_blocks_blocked ON user_blocks (blocked_id);

COMMENT ON COLUMN user_blocks.blocker_id IS '依當下 active role 存現任 buyer_id 或現任 store_id，皆非 raw account_id；僅查無現任 stint（ErrNoCurrentBuyer/ErrNoCurrentStore）時 fallback 用 account_id，見 2026-08-01 ADR（seller-content-moderation-identity-store-id-rekey）';

-- moderation_actions: 審核動作 audit log（B-135；append-only，每次下架/處置都必須有一筆，含系統 D 直接下架）
-- action: AUTO_REJECT | HIDE_TARGET | WARN_USER | BAN_USER | DISMISS | REOPEN
CREATE TABLE IF NOT EXISTS moderation_actions (
    id                BIGINT PRIMARY KEY,
    report_id         BIGINT,
    admin_id          BIGINT,
    affected_user_id  BIGINT,
    action            VARCHAR(30) NOT NULL,
    note              TEXT,
    attempted_content TEXT,
    previous_state    JSONB,
    new_state         JSONB,
    create_time       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_moderation_actions_report ON moderation_actions (report_id);
CREATE INDEX IF NOT EXISTS idx_moderation_actions_admin ON moderation_actions (admin_id, create_time DESC);
CREATE INDEX IF NOT EXISTS idx_moderation_actions_user ON moderation_actions (affected_user_id, create_time DESC);

-- cms_comment: 貼文留言（兩層：parent_id=0 為頂層）
CREATE TABLE IF NOT EXISTS cms_comment (
    id            BIGINT PRIMARY KEY,
    post_id       BIGINT NOT NULL,
    user_id       BIGINT NOT NULL,
    user_name     VARCHAR(100),
    user_avatar   VARCHAR(500),
    content       TEXT,
    parent_id     BIGINT NOT NULL DEFAULT 0,   -- 0=頂層留言
    reply_to_id   BIGINT NOT NULL DEFAULT 0,
    reply_to_name VARCHAR(100),
    role          SMALLINT NOT NULL, -- actor_kind：1=buyer, 2=store
    like_count    INT NOT NULL DEFAULT 0,
    reply_count   INT NOT NULL DEFAULT 0,
    deleted_at    TIMESTAMPTZ,
    hidden_at     TIMESTAMPTZ,                  -- B-135: 平台強制下架（與 user 自刪 deleted_at 區分）
    hidden_reason TEXT,
    create_time   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    update_time   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_cms_comment_role CHECK (role IN (1, 2))
);
CREATE INDEX IF NOT EXISTS idx_cms_comment_post   ON cms_comment (post_id, parent_id, deleted_at, create_time);
CREATE INDEX IF NOT EXISTS idx_cms_comment_parent ON cms_comment (parent_id, deleted_at, create_time DESC);

COMMENT ON COLUMN cms_comment.user_id IS '留言者的業務鍵，命名空間由 role 決定：role=1 是 buyer_id、role=2 是 store_id（只會是該貼文所屬店家自己），不是 account_id。';
COMMENT ON COLUMN cms_comment.role IS '留言者的身分種類（actor_kind）：1=buyer、2=store；與 user_id 合成互動者 (kind, id)。身分由伺服器依貼文歸屬推導，見 ADR 2026-10-17-social-actor-identity。';

-- cms_comment_like
CREATE TABLE IF NOT EXISTS cms_comment_like (
    id          BIGSERIAL PRIMARY KEY,
    comment_id  BIGINT NOT NULL,
    user_id     BIGINT NOT NULL,
    actor_kind  SMALLINT NOT NULL,
    create_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (comment_id, actor_kind, user_id),
    CONSTRAINT chk_cms_comment_like_actor_kind CHECK (actor_kind IN (1, 2))
);

COMMENT ON COLUMN cms_comment_like.actor_kind IS '按讚者的身分種類：1=buyer、2=store（只會是該留言所在貼文所屬的店家自己）。';
COMMENT ON COLUMN cms_comment_like.user_id IS '按讚者的業務鍵，命名空間由 actor_kind 決定：1 是 buyer_id、2 是 store_id，不是 account_id。';

-- cms_post_like
CREATE TABLE IF NOT EXISTS cms_post_like (
    id          BIGSERIAL PRIMARY KEY,
    post_id     BIGINT NOT NULL,
    user_id     BIGINT NOT NULL,
    actor_kind  SMALLINT NOT NULL,
    create_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (post_id, actor_kind, user_id),
    CONSTRAINT chk_cms_post_like_actor_kind CHECK (actor_kind = 1)
);

COMMENT ON COLUMN cms_post_like.actor_kind IS '按讚者的身分種類：貼文讚永遠是本人身分，所以寫入的值恆為 1（buyer）；欄位與唯一鍵照 cms_comment_like 的形狀，讓兩張讚表同一種讀寫。';
COMMENT ON COLUMN cms_post_like.user_id IS '按讚者的業務鍵，命名空間由 actor_kind 決定（恆為 buyer_id），不是 account_id。';

-- cms_seller_forum: 賣家討論區（兩層：parent_id=0 為頂層）
CREATE TABLE IF NOT EXISTS cms_seller_forum (
    id            BIGINT PRIMARY KEY,
    seller_id     BIGINT NOT NULL,
    user_id       BIGINT NOT NULL,
    user_name     VARCHAR(100),
    user_avatar   VARCHAR(500),
    content       TEXT NOT NULL,
    parent_id     BIGINT NOT NULL DEFAULT 0,   -- 0=頂層，>0=回覆
    reply_to_id   BIGINT NOT NULL DEFAULT 0,
    reply_to_name VARCHAR(100),
    role          SMALLINT NOT NULL, -- actor_kind：1=buyer, 2=store
    reply_count   INT NOT NULL DEFAULT 0,
    deleted_at    TIMESTAMPTZ,
    hidden_at     TIMESTAMPTZ,                  -- B-135: 平台強制下架
    hidden_reason TEXT,
    create_time   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    update_time   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_cms_seller_forum_role CHECK (role IN (1, 2))
);
CREATE INDEX IF NOT EXISTS idx_cms_seller_forum_seller ON cms_seller_forum (seller_id, parent_id, deleted_at, create_time DESC);
CREATE INDEX IF NOT EXISTS idx_cms_seller_forum_parent ON cms_seller_forum (parent_id, deleted_at, create_time DESC);

COMMENT ON COLUMN cms_seller_forum.user_id IS '留言者的業務鍵，命名空間由 role 決定：role=1 是 buyer_id、role=2 是 store_id（只會是該討論區所屬店家自己），不是 account_id。';
COMMENT ON COLUMN cms_seller_forum.role IS '留言者的身分種類（actor_kind）：1=buyer、2=store；與 user_id 合成互動者 (kind, id)，決定名稱解析、擁有權比對與刪除處理器的範圍。';

-- ============================================================
-- Hashtag tables (B-153, spec 34)
-- ============================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS cms_hashtag (
    id          BIGINT PRIMARY KEY,
    name        VARCHAR(40) NOT NULL,
    post_count  INT NOT NULL DEFAULT 0,
    create_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    update_time TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_cms_hashtag_name        ON cms_hashtag (name);
CREATE INDEX        IF NOT EXISTS idx_cms_hashtag_name_pattern ON cms_hashtag (name varchar_pattern_ops);
CREATE INDEX        IF NOT EXISTS idx_cms_hashtag_name_trgm    ON cms_hashtag USING gin (name gin_trgm_ops);

CREATE TABLE IF NOT EXISTS cms_post_hashtag (
    post_id      BIGINT NOT NULL,
    hashtag_id   BIGINT NOT NULL,
    display_name VARCHAR(40) NOT NULL,
    position     SMALLINT NOT NULL DEFAULT 0,
    create_time  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, hashtag_id)
);
CREATE INDEX IF NOT EXISTS idx_cms_post_hashtag_lookup  ON cms_post_hashtag (hashtag_id, post_id);
CREATE INDEX IF NOT EXISTS idx_cms_post_hashtag_created ON cms_post_hashtag (create_time, hashtag_id);
