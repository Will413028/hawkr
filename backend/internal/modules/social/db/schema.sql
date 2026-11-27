
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA social;

CREATE TABLE social.posts (
    id             UUID        PRIMARY KEY,
    store_id       BIGINT      NOT NULL,
    body           TEXT        NOT NULL DEFAULT '',
    image_key      TEXT,
    image_asset_id UUID,
    product_id     UUID,
    status         TEXT        NOT NULL,
    published_at   TIMESTAMPTZ,
    version        BIGINT      NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    deleted_at     TIMESTAMPTZ,
    request_id     UUID,
    request_hash   BYTEA,
    CONSTRAINT posts_request_key UNIQUE (store_id, request_id),
    CONSTRAINT posts_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT posts_store_check CHECK (store_id > 0),
    CONSTRAINT posts_body_check CHECK (char_length(body) <= 2200 AND body !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT posts_image_check CHECK ((image_key IS NULL) = (image_asset_id IS NULL)),
    CONSTRAINT posts_product_check CHECK (product_id <> '00000000-0000-0000-0000-000000000000'),
    CONSTRAINT posts_status_check CHECK (status IN ('DRAFT', 'PUBLISHED', 'UNPUBLISHED')),
    CONSTRAINT posts_published_at_check CHECK ((status = 'DRAFT') = (published_at IS NULL)),
    CONSTRAINT posts_version_check CHECK (version >= 1)
);
CREATE INDEX posts_feed ON social.posts (published_at DESC, id DESC) WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX posts_store_feed ON social.posts (store_id, published_at DESC, id DESC) WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX posts_store_mine ON social.posts (store_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX posts_admin_list ON social.posts (created_at DESC, id DESC) WHERE deleted_at IS NULL AND status <> 'DRAFT';

COMMENT ON COLUMN social.posts.status IS '貼文狀態：DRAFT、PUBLISHED、UNPUBLISHED（儲存的代碼是 UPPER_SNAKE，舊的 publish_status 4401、4402、4403 依序對應）';
COMMENT ON COLUMN social.posts.published_at IS '第一次發布的時間，之後不變（下架不清、再發布沿用）；DRAFT 沒有，發布過的一定有';
COMMENT ON COLUMN social.posts.version IS '樂觀鎖的版本，對外是 etag；每次修改加一';
COMMENT ON COLUMN social.posts.request_id IS '建立時 client 帶的 request_id（AIP-155，選填）；範圍是 (店家, request_id)，同一家店重送回這一列；貼文刪除後它仍占著，重送回原本那一列（刪除的貼文讀不到，回 not_found）';
COMMENT ON COLUMN social.posts.request_hash IS '建立輸入（內文、圖片 token、商品、標籤、是否發布）的 SHA-256；同一個 request_id 重送時比對內容是否相同，與 request_id 同時有或同時沒有';

CREATE TABLE social.post_schedules (
    id                UUID        PRIMARY KEY,
    post_id           UUID        NOT NULL REFERENCES social.posts (id),
    store_id          BIGINT      NOT NULL,
    publish_at        TIMESTAMPTZ NOT NULL,
    scheduled_by_kind TEXT        NOT NULL,
    scheduled_by_ref  TEXT        NOT NULL,
    state             TEXT        NOT NULL,
    cancel_reason     TEXT,
    created_at        TIMESTAMPTZ NOT NULL,
    resolved_at       TIMESTAMPTZ,
    CONSTRAINT post_schedules_store_check CHECK (store_id > 0),
    CONSTRAINT post_schedules_by_kind_check CHECK (scheduled_by_kind IN ('OWNER', 'DELEGATION')),
    CONSTRAINT post_schedules_by_ref_check CHECK (char_length(scheduled_by_ref) >= 1 AND char_length(scheduled_by_ref) <= 200 AND scheduled_by_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT post_schedules_state_check CHECK (state IN ('PENDING', 'PUBLISHED', 'CANCELED')),
    CONSTRAINT post_schedules_reason_check CHECK (cancel_reason IN ('AUTHORITY_LOST', 'DELETED', 'TAKEN_DOWN', 'REPLACED', 'OWNER_CANCELED', 'NOT_PUBLISHABLE', 'RETRY_EXHAUSTED', 'EXPIRED')),
    CONSTRAINT post_schedules_reason_state_check CHECK ((state = 'CANCELED') = (cancel_reason IS NOT NULL)),
    CONSTRAINT post_schedules_resolved_check CHECK ((state = 'PENDING') = (resolved_at IS NULL))
);
CREATE UNIQUE INDEX post_schedules_post_pending ON social.post_schedules (post_id) WHERE state = 'PENDING';
CREATE INDEX post_schedules_by_pending ON social.post_schedules (scheduled_by_kind, scheduled_by_ref) WHERE state = 'PENDING';
CREATE INDEX post_schedules_store_pending ON social.post_schedules (store_id) WHERE state = 'PENDING';

COMMENT ON COLUMN social.post_schedules.scheduled_by_ref IS '發起者擁有格式的不透明字串；DELEGATION 放代行 session 的 ID，OWNER 放店家帳號 ID 的十進位字串';
COMMENT ON COLUMN social.post_schedules.cancel_reason IS '取消原因，凍結在列上：AUTHORITY_LOST 到點時授權已失效、DELETED 貼文被刪、TAKEN_DOWN 貼文被下架措施、REPLACED 被立即發布或新排程取代、OWNER_CANCELED 店主取消、NOT_PUBLISHABLE 到點時貼文不再滿足公開要件、RETRY_EXHAUSTED 到點發布重試用盡、EXPIRED 到點時已超過可補發的期限';

CREATE TABLE social.comments (
    id          UUID        PRIMARY KEY,
    post_id     UUID        NOT NULL REFERENCES social.posts (id),
    parent_id   UUID,
    reply_to_id UUID,
    actor_kind  TEXT        NOT NULL,
    actor_id    BIGINT      NOT NULL,
    body        TEXT,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    deleted_at  TIMESTAMPTZ,
    thread_id   UUID        NOT NULL GENERATED ALWAYS AS (COALESCE(parent_id, id)) STORED,
    counted     BOOLEAN     NOT NULL DEFAULT FALSE,
    version     BIGINT      NOT NULL DEFAULT 1,
    request_id   UUID,
    request_hash BYTEA,
    CONSTRAINT comments_request_key UNIQUE (actor_kind, actor_id, request_id),
    CONSTRAINT comments_version_check CHECK (version >= 1),
    CONSTRAINT comments_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT comments_post_id_key UNIQUE (post_id, id),
    CONSTRAINT comments_thread_key UNIQUE (post_id, thread_id, id),
    CONSTRAINT comments_parent_fk FOREIGN KEY (post_id, parent_id) REFERENCES social.comments (post_id, id),
    CONSTRAINT comments_reply_to_fk FOREIGN KEY (post_id, parent_id, reply_to_id) REFERENCES social.comments (post_id, thread_id, id),
    CONSTRAINT comments_actor_kind_check CHECK (actor_kind IN ('BUYER', 'STORE')),
    CONSTRAINT comments_actor_id_check CHECK (actor_id > 0),
    CONSTRAINT comments_body_check CHECK (char_length(body) >= 1 AND char_length(body) <= 2200 AND body !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT comments_scrubbed_check CHECK (body IS NOT NULL OR deleted_at IS NOT NULL),
    CONSTRAINT comments_parent_check CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT comments_reply_to_check CHECK (reply_to_id IS NULL OR (parent_id IS NOT NULL AND reply_to_id <> id))
);
CREATE INDEX comments_top_level ON social.comments (post_id, created_at, id) WHERE parent_id IS NULL;
CREATE INDEX comments_store_replies ON social.comments (parent_id) WHERE counted AND actor_kind = 'STORE';
CREATE INDEX comments_replies ON social.comments (parent_id, created_at, id) WHERE counted;
CREATE INDEX comments_actor ON social.comments (actor_kind, actor_id, id);

COMMENT ON COLUMN social.comments.counted IS '此刻是否計入貼文的留言數與父留言的回覆數（沒有刪除、沒有未還原的隱藏、沒有內容下架、作者不在 inactive_authors，在 social.reevaluate_comments 求值）；只由 social.reevaluate_comments 維護，讀取直接用它，貼文的 comment_count 與留言的 reply_count 是這個旗標的加總';
COMMENT ON COLUMN social.comments.body IS '內文；作者自己刪除時保留，買家刪帳的清除 job 之後設為 NULL（保留列讓討論串結構與佔位項目還在），所以 NULL 一定是已刪除的列（comments_scrubbed_check）';
COMMENT ON COLUMN social.comments.version IS '樂觀鎖的版本，對外是 etag；內文修改與刪除各加一';
COMMENT ON COLUMN social.comments.request_id IS '建立時 client 帶的 request_id（AIP-155，選填）；範圍是 (互動者, request_id)，同一個互動者重送回這一列';
COMMENT ON COLUMN social.comments.request_hash IS '建立輸入（貼文、父留言、被回覆的留言、內文）的 SHA-256；同一個 request_id 重送時比對內容是否相同，與 request_id 同時有或同時沒有';
COMMENT ON COLUMN social.comments.actor_kind IS '互動者的種類：BUYER（actor_id 是 buyer_id）、STORE（actor_id 是 store_id）；買家與店家各自連號，同一個數值在兩邊是不同的人';

CREATE TABLE social.comment_likes (
    comment_id UUID        NOT NULL REFERENCES social.comments (id),
    actor_kind TEXT        NOT NULL,
    actor_id   BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (comment_id, actor_kind, actor_id),
    CONSTRAINT comment_likes_actor_kind_check CHECK (actor_kind IN ('BUYER', 'STORE')),
    CONSTRAINT comment_likes_actor_id_check CHECK (actor_id > 0)
);
CREATE INDEX comment_likes_actor ON social.comment_likes (actor_kind, actor_id, comment_id);

CREATE TABLE social.post_likes (
    post_id    UUID        NOT NULL REFERENCES social.posts (id),
    buyer_id   BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (post_id, buyer_id),
    CONSTRAINT post_likes_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX post_likes_buyer ON social.post_likes (buyer_id, post_id);

CREATE TABLE social.forum_entries (
    id          UUID        PRIMARY KEY,
    store_id    BIGINT      NOT NULL,
    parent_id   UUID,
    reply_to_id UUID,
    actor_kind  TEXT        NOT NULL,
    actor_id    BIGINT      NOT NULL,
    body        TEXT,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    deleted_at  TIMESTAMPTZ,
    thread_id   UUID        NOT NULL GENERATED ALWAYS AS (COALESCE(parent_id, id)) STORED,
    counted     BOOLEAN     NOT NULL DEFAULT FALSE,
    version     BIGINT      NOT NULL DEFAULT 1,
    request_id   UUID,
    request_hash BYTEA,
    CONSTRAINT forum_entries_request_key UNIQUE (actor_kind, actor_id, request_id),
    CONSTRAINT forum_entries_version_check CHECK (version >= 1),
    CONSTRAINT forum_entries_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT forum_entries_store_id_key UNIQUE (store_id, id),
    CONSTRAINT forum_entries_thread_key UNIQUE (store_id, thread_id, id),
    CONSTRAINT forum_entries_parent_fk FOREIGN KEY (store_id, parent_id) REFERENCES social.forum_entries (store_id, id),
    CONSTRAINT forum_entries_reply_to_fk FOREIGN KEY (store_id, parent_id, reply_to_id) REFERENCES social.forum_entries (store_id, thread_id, id),
    CONSTRAINT forum_entries_store_check CHECK (store_id > 0),
    CONSTRAINT forum_entries_actor_kind_check CHECK (actor_kind IN ('BUYER', 'STORE')),
    CONSTRAINT forum_entries_actor_id_check CHECK (actor_id > 0),
    CONSTRAINT forum_entries_body_check CHECK (char_length(body) >= 1 AND char_length(body) <= 2200 AND body !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT forum_entries_scrubbed_check CHECK (body IS NOT NULL OR deleted_at IS NOT NULL),
    CONSTRAINT forum_entries_parent_check CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT forum_entries_reply_to_check CHECK (reply_to_id IS NULL OR (parent_id IS NOT NULL AND reply_to_id <> id))
);
COMMENT ON COLUMN social.forum_entries.counted IS '此刻是否計入父討論區留言的回覆數（沒有刪除、沒有未還原的隱藏、沒有內容下架、作者不在 inactive_authors，在 social.reevaluate_forum_entries 求值）；只由 social.reevaluate_forum_entries 維護，讀取直接用它，父項的 reply_count 是這個旗標的加總';
COMMENT ON COLUMN social.forum_entries.body IS '內文；作者自己刪除時保留，買家刪帳的清除 job 之後設為 NULL（保留列讓討論串結構與佔位項目還在），所以 NULL 一定是已刪除的列（forum_entries_scrubbed_check）';
COMMENT ON COLUMN social.forum_entries.version IS '樂觀鎖的版本，對外是 etag；內文修改與刪除各加一';
COMMENT ON COLUMN social.forum_entries.request_id IS '建立時 client 帶的 request_id（AIP-155，選填）；範圍是 (互動者, request_id)，同一個互動者重送回這一列';
COMMENT ON COLUMN social.forum_entries.request_hash IS '建立輸入（店家、父項、被回覆的項目、內文）的 SHA-256；同一個 request_id 重送時比對內容是否相同，與 request_id 同時有或同時沒有';
CREATE INDEX forum_entries_top_level ON social.forum_entries (store_id, created_at, id) WHERE parent_id IS NULL;
CREATE INDEX forum_entries_replies ON social.forum_entries (parent_id, created_at, id) WHERE counted;
CREATE INDEX forum_entries_actor ON social.forum_entries (actor_kind, actor_id, id);

CREATE TABLE social.post_hashtags (
    post_id      UUID     NOT NULL REFERENCES social.posts (id),
    position     SMALLINT NOT NULL,
    name         TEXT     NOT NULL,
    display_name TEXT     NOT NULL,
    published_at TIMESTAMPTZ,
    PRIMARY KEY (post_id, position),
    CONSTRAINT post_hashtags_name_key UNIQUE (post_id, name),
    CONSTRAINT post_hashtags_position_check CHECK (position BETWEEN 0 AND 9),
    CONSTRAINT post_hashtags_name_check CHECK (char_length(name) BETWEEN 1 AND 40),
    CONSTRAINT post_hashtags_display_check CHECK (char_length(display_name) BETWEEN 1 AND 40)
);
CREATE INDEX post_hashtags_name ON social.post_hashtags (name, post_id);
CREATE INDEX post_hashtags_feed ON social.post_hashtags (name, published_at DESC, post_id DESC) WHERE published_at IS NOT NULL;
COMMENT ON COLUMN social.post_hashtags.published_at IS '貼文 posts.published_at 的複本（trigger 維護，貼文發布後不變）；草稿的標籤列是 NULL';

CREATE TABLE social.hides (
    id              UUID        PRIMARY KEY,
    comment_id      UUID        REFERENCES social.comments (id),
    forum_entry_id  UUID        REFERENCES social.forum_entries (id),
    reason_category TEXT        NOT NULL,
    reason_text     TEXT        NOT NULL DEFAULT '',
    source_ref      TEXT,
    admin_id        BIGINT      NOT NULL,
    hidden_at       TIMESTAMPTZ NOT NULL,
    restored_at     TIMESTAMPTZ,
    restored_by     BIGINT,
    restore_reason  TEXT,
    CONSTRAINT hides_target_check CHECK (num_nonnulls(comment_id, forum_entry_id) = 1),
    CONSTRAINT hides_reason_category_check CHECK (reason_category IN ('MISLEADING_INFO', 'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT')),
    CONSTRAINT hides_reason_text_check CHECK (char_length(reason_text) <= 500 AND reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT hides_source_ref_check CHECK (source_ref IS NULL OR (char_length(source_ref) >= 1 AND char_length(source_ref) <= 200 AND source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT hides_admin_check CHECK (admin_id > 0),
    CONSTRAINT hides_restore_check CHECK (
        (restored_at IS NULL AND restored_by IS NULL AND restore_reason IS NULL)
        OR (restored_at IS NOT NULL AND restored_by > 0 AND restore_reason IS NOT NULL AND restored_at >= hidden_at)
    ),
    CONSTRAINT hides_restore_reason_check CHECK (restore_reason IS NULL OR (char_length(restore_reason) >= 1 AND char_length(restore_reason) <= 500 AND restore_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'))
);
CREATE UNIQUE INDEX hides_active_comment ON social.hides (comment_id) WHERE restored_at IS NULL AND comment_id IS NOT NULL;
CREATE UNIQUE INDEX hides_active_forum_entry ON social.hides (forum_entry_id) WHERE restored_at IS NULL AND forum_entry_id IS NOT NULL;
CREATE INDEX hides_comment ON social.hides (comment_id, hidden_at) WHERE comment_id IS NOT NULL;
CREATE INDEX hides_forum_entry ON social.hides (forum_entry_id, hidden_at) WHERE forum_entry_id IS NOT NULL;

CREATE TABLE social.content_takedowns (
    measure_id     UUID        NOT NULL,
    post_id        UUID        REFERENCES social.posts (id),
    comment_id     UUID        REFERENCES social.comments (id),
    forum_entry_id UUID        REFERENCES social.forum_entries (id),
    CONSTRAINT content_takedowns_target_check CHECK (num_nonnulls(post_id, comment_id, forum_entry_id) = 1)
);
CREATE UNIQUE INDEX content_takedowns_measure_post ON social.content_takedowns (measure_id, post_id) WHERE post_id IS NOT NULL;
CREATE UNIQUE INDEX content_takedowns_measure_comment ON social.content_takedowns (measure_id, comment_id) WHERE comment_id IS NOT NULL;
CREATE UNIQUE INDEX content_takedowns_measure_forum_entry ON social.content_takedowns (measure_id, forum_entry_id) WHERE forum_entry_id IS NOT NULL;
CREATE UNIQUE INDEX content_takedowns_post ON social.content_takedowns (post_id) WHERE post_id IS NOT NULL;
CREATE UNIQUE INDEX content_takedowns_comment ON social.content_takedowns (comment_id) WHERE comment_id IS NOT NULL;
CREATE UNIQUE INDEX content_takedowns_forum_entry ON social.content_takedowns (forum_entry_id) WHERE forum_entry_id IS NOT NULL;

CREATE TABLE social.sanctions (
    id              UUID        PRIMARY KEY,
    buyer_id        BIGINT      NOT NULL,
    kind            TEXT        NOT NULL,
    reason_category TEXT        NOT NULL,
    reason_text     TEXT        NOT NULL DEFAULT '',
    source_ref      TEXT,
    admin_id        BIGINT      NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    CONSTRAINT sanctions_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT sanctions_kind_check CHECK (kind IN ('WARNING')),
    CONSTRAINT sanctions_reason_category_check CHECK (reason_category IN ('MISLEADING_INFO', 'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT')),
    CONSTRAINT sanctions_reason_text_check CHECK (char_length(reason_text) <= 500 AND reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT sanctions_source_ref_check CHECK (source_ref IS NULL OR (char_length(source_ref) >= 1 AND char_length(source_ref) <= 200 AND source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT sanctions_admin_check CHECK (admin_id > 0)
);
CREATE INDEX sanctions_buyer ON social.sanctions (buyer_id, created_at DESC, id DESC);

CREATE TABLE social.inactive_authors (
    buyer_id BIGINT      PRIMARY KEY,
    since    TIMESTAMPTZ NOT NULL,
    CONSTRAINT inactive_authors_buyer_check CHECK (buyer_id > 0)
);

CREATE TABLE social.post_stats (
    post_id       UUID   PRIMARY KEY REFERENCES social.posts (id),
    like_count    BIGINT NOT NULL DEFAULT 0,
    comment_count BIGINT NOT NULL DEFAULT 0,
    view_count    BIGINT NOT NULL DEFAULT 0,
    share_count   BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT post_stats_counts_check CHECK (like_count >= 0 AND comment_count >= 0 AND view_count >= 0 AND share_count >= 0)
);

CREATE TABLE social.comment_stats (
    comment_id  UUID   PRIMARY KEY REFERENCES social.comments (id),
    like_count  BIGINT NOT NULL DEFAULT 0,
    reply_count BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT comment_stats_counts_check CHECK (like_count >= 0 AND reply_count >= 0)
);

CREATE TABLE social.forum_entry_stats (
    forum_entry_id UUID   PRIMARY KEY REFERENCES social.forum_entries (id),
    reply_count    BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT forum_entry_stats_counts_check CHECK (reply_count >= 0)
);

COMMENT ON COLUMN social.post_stats.comment_count IS '留言數含回覆：旗標 comments.counted 為真的列數；計入的定義在 social.reevaluate_comments（只讀 social 自己的表）';
COMMENT ON COLUMN social.post_stats.view_count IS '只增的事件計數（去重後的瀏覽），沒有可重算的來源，不進每日對帳';

CREATE TABLE social.post_views (
    post_id    UUID  NOT NULL REFERENCES social.posts (id),
    view_day   DATE  NOT NULL,
    viewer_key BYTEA NOT NULL,
    PRIMARY KEY (post_id, view_day, viewer_key),
    CONSTRAINT post_views_viewer_key_check CHECK (octet_length(viewer_key) = 32)
);
CREATE INDEX post_views_day ON social.post_views (view_day);

CREATE TABLE social.view_salts (
    view_day DATE  PRIMARY KEY,
    salt     BYTEA NOT NULL,
    CONSTRAINT view_salts_salt_check CHECK (octet_length(salt) = 32)
);

CREATE TABLE social.post_shares (
    post_id    UUID        NOT NULL REFERENCES social.posts (id),
    buyer_id   BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (post_id, buyer_id),
    CONSTRAINT post_shares_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX post_shares_buyer ON social.post_shares (buyer_id, post_id);
