-- social 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式、trigger 與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（social.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；值的集合與關聯表用複合自然鍵。店家（store_id）、買家（buyer_id）與管理員（admin_id）
-- 是別的模組擁有的業務鍵，只存 BIGINT。慣例見 PostgreSQL 設計規範。
--
-- 文字欄位頭尾沒有空白的 CHECK 是 platform/trimtext.Check 的展開（單體規則 9，登記在 cmd/hawkr 的 trimmedColumns）；長度上限
-- 同時是寫入端的第一道檢查（internal/model 的 MaxPostBodyRunes、MaxCommentBodyRunes）與這裡的後備。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA social;

-- 貼文。status 是 DRAFT → PUBLISHED → UNPUBLISHED → PUBLISHED，刪除是軟刪（deleted_at）；DRAFT 還沒發布過所以沒有 published_at，
-- 發布過的（PUBLISHED、UNPUBLISHED）一定有：published_at 是第一次發布的時間，之後不變（下架不清它，再發布沿用）（動態牆與熱門的時間窗都以它為準）。
-- 公開要件（恰好一張圖、內文必填）在轉為 PUBLISHED 時由寫入端檢查，草稿可以不完整，所以內文可以是空字串。version 是樂觀鎖，
-- 對外是 etag（ADR 2026-09-25-optimistic-concurrency-etag）；按讚、瀏覽等計數不在這張表，放在 post_stats，不和編輯搶鎖。
-- image_key 是顯示用的 object key，image_asset_id 是 media 的資產 ID：同步 job 以它為業務列目前的資產（internal/mediaref）。
-- product_id 是商品（Java catalog，B4 前的 BIGINT 業務鍵），寫入時驗證存在、上架中且屬於該店，讀取時不重驗。
CREATE TABLE social.posts (
    id             UUID        PRIMARY KEY,
    store_id       BIGINT      NOT NULL,
    body           TEXT        NOT NULL DEFAULT '',
    image_key      TEXT,
    image_asset_id TEXT,
    product_id     BIGINT,
    status         TEXT        NOT NULL,
    published_at   TIMESTAMPTZ,
    version        BIGINT      NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    deleted_at     TIMESTAMPTZ,
    CONSTRAINT posts_store_check CHECK (store_id > 0),
    CONSTRAINT posts_body_check CHECK (char_length(body) <= 2200 AND body !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT posts_image_check CHECK ((image_key IS NULL) = (image_asset_id IS NULL)),
    CONSTRAINT posts_product_check CHECK (product_id > 0),
    CONSTRAINT posts_status_check CHECK (status IN ('DRAFT', 'PUBLISHED', 'UNPUBLISHED')),
    CONSTRAINT posts_published_at_check CHECK ((status = 'DRAFT') = (published_at IS NULL)),
    CONSTRAINT posts_version_check CHECK (version >= 1)
);
-- 動態牆與熱門：最新發布的在前（keyset：published_at DESC, id DESC）。
CREATE INDEX posts_feed ON social.posts (published_at DESC, id DESC) WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
-- 店家頁：該店已發布的貼文。
CREATE INDEX posts_store_feed ON social.posts (store_id, published_at DESC, id DESC) WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
-- 店主看自己的貼文（含草稿與下架）：最新建立的在前。
CREATE INDEX posts_store_mine ON social.posts (store_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

COMMENT ON COLUMN social.posts.status IS '貼文狀態：DRAFT、PUBLISHED、UNPUBLISHED（儲存的代碼是 UPPER_SNAKE，舊的 publish_status 4401、4402、4403 依序對應）';
COMMENT ON COLUMN social.posts.published_at IS '第一次發布的時間，之後不變（下架不清、再發布沿用）；DRAFT 沒有，發布過的一定有';
COMMENT ON COLUMN social.posts.version IS '樂觀鎖的版本，對外是 etag；每次修改加一';

-- 貼文的留言：兩層（回覆的父一律是頂層，由寫入端保證，DB 保證同篇與 reply_to 在同一串）。actor_kind、actor_id 是互動者 (kind, id)，由伺服器依資源歸屬推導
-- （ADR 2026-10-17-social-actor-identity）：BUYER 的 actor_id 是 buyer_id，STORE 的是 store_id。不存作者名稱與頭像快照，讀取時由
-- buyer、seller 批次補。同篇靠 UNIQUE (post_id, id) 與複合 FK：parent_id 指到別篇的留言時 DB 就拒絕；reply_to 在同一串靠產生欄位
-- thread_id（頂層留言是自己，回覆是它的父）與 UNIQUE (post_id, thread_id, id)：reply_to_fk 要求被回覆的留言的串等於這則回覆的父，
-- 所以 reply_to 只能是父本身，或父底下的另一則回覆。
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
-- 頂層留言的列表（keyset：created_at, id）：已刪除與被隱藏的頂層留言底下還有可見的回覆時要出現佔位項目，所以不排除 deleted_at。
CREATE INDEX comments_top_level ON social.comments (post_id, created_at, id) WHERE parent_id IS NULL;
-- 回覆的列表與「底下有沒有可見的回覆」的探測。述詞是旗標：讀取只看 counted（visibility.Counted），planner 無法從 counted 推出 deleted_at IS NULL，
-- 索引的述詞若是 deleted_at IS NULL 就用不上（量測：熱門貼文頂層留言 p50 1.7 ms 變 196 ms，Seq Scan comments）。
CREATE INDEX comments_replies ON social.comments (parent_id, created_at, id) WHERE counted;
-- 買家的清除 job 與「我的留言」：以作者找。
CREATE INDEX comments_actor ON social.comments (actor_kind, actor_id, id);

COMMENT ON COLUMN social.comments.counted IS '此刻是否計入貼文的留言數與父留言的回覆數（沒有刪除、沒有未還原的隱藏、沒有內容下架、作者不在 inactive_authors，在 social.reevaluate_comments 求值）；只由 social.reevaluate_comments 維護，讀取直接用它，貼文的 comment_count 與留言的 reply_count 是這個旗標的加總';
COMMENT ON COLUMN social.comments.body IS '內文；作者自己刪除時保留，買家刪帳的清除 job 之後設為 NULL（保留列讓討論串結構與佔位項目還在），所以 NULL 一定是已刪除的列（comments_scrubbed_check）';
COMMENT ON COLUMN social.comments.actor_kind IS '互動者的種類：BUYER（actor_id 是 buyer_id）、STORE（actor_id 是 store_id）；買家與店家各自連號，同一個數值在兩邊是不同的人';

-- 留言的讚：互動者是 (kind, id)，複合鍵保證重複按讚冪等（INSERT ... ON CONFLICT DO NOTHING）。
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

-- 貼文的讚：只有買家按得到（店家本人的讚不在貼文上，不存 kind）。
CREATE TABLE social.post_likes (
    post_id    UUID        NOT NULL REFERENCES social.posts (id),
    buyer_id   BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (post_id, buyer_id),
    CONSTRAINT post_likes_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX post_likes_buyer ON social.post_likes (buyer_id, post_id);

-- 店家討論區的留言：容器是店家（store_id），其餘同 comments；不和 comments 合成一張（容器不同、討論區留言沒有讚，合併要「容器恰好
-- 一個」的 CHECK 與兩套讀取）。
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
CREATE INDEX forum_entries_top_level ON social.forum_entries (store_id, created_at, id) WHERE parent_id IS NULL;
CREATE INDEX forum_entries_replies ON social.forum_entries (parent_id, created_at, id) WHERE counted;
CREATE INDEX forum_entries_actor ON social.forum_entries (actor_kind, actor_id, id);

-- 貼文的 hashtag：貼文的值的集合（隨貼文整批讀寫、不被以 ID 指稱，ADR 2026-09-25-row-identity-by-data-kind D1），整批替換。
-- name 是正規化後的名稱（NFC、ASCII 小寫；字元集 a-z、0-9 與漢字由寫入端的 internal/hashtag 檢查，DB 不重寫一份 Unicode 的 Han 範圍），
-- display_name 是使用者輸入的樣子；position 是顯示順序。沒有 hashtags 實體表，也不存
-- post_count：數量在讀取時依公開可讀的貼文計算（SD16）。
-- published_at 是貼文 published_at 的複本，讓某個標籤的貼文列表能以 (name, published_at, post_id) 的索引直接走 keyset：貼文第一次發布時
-- 由 trigger 寫進它所有的標籤列，標籤列在貼文已發布後才寫入時由 trigger 帶入；貼文的 published_at 發布後不變（trigger 擋改動），所以複本之後不需要再同步。
-- 草稿的標籤列是 NULL（貼文還沒有 published_at）。
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

-- 買家的內容的隱藏紀錄：理由、來源、還原與稽核在同一處，可以多次隱藏與還原（還原留列不刪，之後可以再隱藏）。只收買家寫的留言與討論區
-- 留言：貼文與店家身分的內容走 seller 的內容下架措施（措施是店家處分效果的唯一來源）。reason_category 是 PolicyCategory 的社群子集
-- （platform/policy.CommunityAssessable，模組的 TestChecksMatchGo）；source_ref 是 moderation 的案件的不透明參照（social 不讀 moderation）。
-- 同一則內容未還原的隱藏最多一筆（兩個部分唯一索引）。
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
    CONSTRAINT hides_reason_category_check CHECK (reason_category IN ('MISLEADING_INFO', 'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE')),
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
-- 還原後可以再隱藏，歷史的列要能依內容找：作者讀「我被隱藏的內容」、管理員看紀錄。
CREATE INDEX hides_comment ON social.hides (comment_id, hidden_at) WHERE comment_id IS NOT NULL;
CREATE INDEX hides_forum_entry ON social.hides (forum_entry_id, hidden_at) WHERE forum_entry_id IS NOT NULL;

-- 內容下架的複本：seller 擁有措施（seller.measures、measure_contents），social 在同一個 transaction 的訂閱者（MeasureCreated、MeasureRevoked 中的
-- 內容下架）裡寫一列一個（措施，內容），所以貼文的公開讀取與留言、討論區留言的 counted 旗標只依 social 自己的表求值。與 inactive_authors 同一個形狀，
-- 差別是措施可以撤銷：建立插入，撤銷刪除，並讓受影響的內容重新求值。同一則內容可以同時被多個措施下架，所以內容不是主鍵，(措施, 內容) 才是（三個部分唯一索引）；
-- 每日對帳拿 seller.pub_content_takedowns() 雙向核對。貼文沒有計入旗標，公開讀取的述詞對 post_id 做 NOT EXISTS（visibility.ReadablePosts）；
-- 貼文底下的留言跟著父貼文，不另寫列。
CREATE TABLE social.content_takedowns (
    measure_id     UUID        NOT NULL,
    post_id        UUID        REFERENCES social.posts (id),
    comment_id     UUID        REFERENCES social.comments (id),
    forum_entry_id UUID        REFERENCES social.forum_entries (id),
    created_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT content_takedowns_target_check CHECK (num_nonnulls(post_id, comment_id, forum_entry_id) = 1)
);
CREATE UNIQUE INDEX content_takedowns_measure_post ON social.content_takedowns (measure_id, post_id) WHERE post_id IS NOT NULL;
CREATE UNIQUE INDEX content_takedowns_measure_comment ON social.content_takedowns (measure_id, comment_id) WHERE comment_id IS NOT NULL;
CREATE UNIQUE INDEX content_takedowns_measure_forum_entry ON social.content_takedowns (measure_id, forum_entry_id) WHERE forum_entry_id IS NOT NULL;
-- 讀取與重新求值依內容探測「有沒有下架」，撤銷依措施找列。
CREATE INDEX content_takedowns_post ON social.content_takedowns (post_id) WHERE post_id IS NOT NULL;
CREATE INDEX content_takedowns_comment ON social.content_takedowns (comment_id) WHERE comment_id IS NOT NULL;
CREATE INDEX content_takedowns_forum_entry ON social.content_takedowns (forum_entry_id) WHERE forum_entry_id IS NOT NULL;

-- 買家的社群處分紀錄（處分紀錄與效果在同一個模組、同一個 transaction，只有一個寫入者，10e R12）。kind 初始只有 WARNING；之後加限制
-- 種類只加值與規則（買家條款訂出後）。ends_at 目前沒有讀者（警告不限制參與），為有期限的參與限制保留。
CREATE TABLE social.sanctions (
    id              UUID        PRIMARY KEY,
    buyer_id        BIGINT      NOT NULL,
    kind            TEXT        NOT NULL,
    reason_category TEXT        NOT NULL,
    reason_text     TEXT        NOT NULL DEFAULT '',
    source_ref      TEXT,
    admin_id        BIGINT      NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ,
    CONSTRAINT sanctions_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT sanctions_kind_check CHECK (kind IN ('WARNING')),
    CONSTRAINT sanctions_reason_category_check CHECK (reason_category IN ('MISLEADING_INFO', 'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE')),
    CONSTRAINT sanctions_reason_text_check CHECK (char_length(reason_text) <= 500 AND reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT sanctions_source_ref_check CHECK (source_ref IS NULL OR (char_length(source_ref) >= 1 AND char_length(source_ref) <= 200 AND source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT sanctions_admin_check CHECK (admin_id > 0),
    CONSTRAINT sanctions_ends_at_check CHECK (ends_at IS NULL OR ends_at > created_at)
);
CREATE INDEX sanctions_buyer ON social.sanctions (buyer_id, created_at DESC, id DESC);

-- 不再營運中的買家（刪除中與已關閉）作者的複本：buyer 擁有這個事實（buyer.pub_inactive_buyers），social 在同一個 transaction 的訂閱者裡
-- （BuyerDeletionRequested、BuyerDeleted；直接從營運中關閉的路徑只發 BuyerDeleted，所以兩個都訂）寫一列，並讓該作者的內容重新求值，
-- 所以「是否計入」的旗標只依 social 自己的表求值。買家的狀態不會回到營運中（刪帳申請不能撤回），因此只插入不刪除，冪等
-- （ON CONFLICT DO NOTHING）。每日對帳拿 buyer.pub_inactive_buyers 核對，缺的補上並記漂移。since 是事件發生的時間。
CREATE TABLE social.inactive_authors (
    buyer_id BIGINT      PRIMARY KEY,
    since    TIMESTAMPTZ NOT NULL,
    CONSTRAINT inactive_authors_buyer_check CHECK (buyer_id > 0)
);

-- 計數：全部存在獨立的 stats 表（不放在實體列上，按讚與瀏覽不和貼文編輯的 version CAS 搶鎖）。「是否計入」的單一定義與維護它的 trigger 在
-- objects.sql（counts-brief.md §4）；trigger 是計數唯一的維護路徑，寫入端不直接改這些表（重算除外：先鎖列再覆寫）。列隨實體建立（trigger）。
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

-- 瀏覽的去重：(貼文, 台北日, 觀看者鍵) 的主鍵就是去重，插入成功才把 post_stats.view_count 加一。viewer_key 是以當日 salt 做的
-- HMAC-SHA256（登入者是帳號、匿名是貼文 + IP + User-Agent），表裡沒有可回推的帳號或 IP。只留當日與前一日，之後由週期 job 刪。
CREATE TABLE social.post_views (
    post_id    UUID  NOT NULL REFERENCES social.posts (id),
    view_day   DATE  NOT NULL,
    viewer_key BYTEA NOT NULL,
    PRIMARY KEY (post_id, view_day, viewer_key),
    CONSTRAINT post_views_viewer_key_check CHECK (octet_length(viewer_key) = 32)
);
CREATE INDEX post_views_day ON social.post_views (view_day);

-- 每個台北日一把 salt，跨實例共用（第一次需要時 INSERT ... ON CONFLICT DO NOTHING 後讀回）；前一日以前的列由週期 job 刪。
CREATE TABLE social.view_salts (
    view_day DATE  PRIMARY KEY,
    salt     BYTEA NOT NULL,
    CONSTRAINT view_salts_salt_check CHECK (octet_length(salt) = 32)
);

-- 貼文的分享：同一位買家同一篇只算一次，主鍵就是去重；分享數由 trigger 加減。
CREATE TABLE social.post_shares (
    post_id    UUID        NOT NULL REFERENCES social.posts (id),
    buyer_id   BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (post_id, buyer_id),
    CONSTRAINT post_shares_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX post_shares_buyer ON social.post_shares (buyer_id, post_id);

-- media 引用的同步版本：業務列改變 ref 的資產時在同一個 transaction 遞增 seq，同步 job 讀到 seq 之後做事，結算時只在 seq 沒變
-- 才記下 synced_seq，updated_at 是最後一次變動；保底掃描補排 synced_seq < seq 的 ref（internal/mediaref）。ref 的列不刪除：
-- 刪掉再建會讓 seq 從頭算，同步讀到的舊 seq 可能剛好相等而誤判為已同步。
CREATE TABLE social.media_ref_sync (
    ref_type   TEXT   NOT NULL,
    ref_id     TEXT   NOT NULL,
    seq        BIGINT NOT NULL DEFAULT 1,
    synced_seq BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ref_type, ref_id)
);
-- 保底掃描只看還沒同步完的 ref。
CREATE INDEX media_ref_sync_unsynced ON social.media_ref_sync (updated_at) WHERE synced_seq < seq;

-- 被業務列換掉或終態釋放的資產：同步 job 精確解除它們的引用，寫入端拒絕把它們寫回同一個 ref。unlinked_at 是第一次解除成功的時間。
CREATE TABLE social.retired_assets (
    ref_type    TEXT        NOT NULL,
    ref_id      TEXT        NOT NULL,
    asset_id    TEXT        NOT NULL,
    retired_at  TIMESTAMPTZ NOT NULL,
    unlinked_at TIMESTAMPTZ,
    PRIMARY KEY (ref_type, ref_id, asset_id)
);
CREATE INDEX retired_assets_retired_at ON social.retired_assets (retired_at);
