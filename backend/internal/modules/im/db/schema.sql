-- im 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（im.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA im;

-- 買家與店家之間的一對一對話。id 是 UUIDv7；buyer_id、store_id 是外部業務鍵（不可互換）。UNIQUE (buyer_id, store_id) 是「一對雙方只有一個對話」的最終保證。
-- 對話只由 AllocateSeq 建立，建立時就有首則訊息。last_seq 是已配發的最後一個 seq；兩個已讀游標在這一列上（聊天只有兩方，沒有 participants 表），
-- 未讀 = last_seq - 自己那一方的游標，送出時寄件人自己那一方的游標與 last_seq 一起前進。
CREATE TABLE im.conversations (
    id                  UUID        PRIMARY KEY,
    buyer_id            BIGINT      NOT NULL,
    store_id            BIGINT      NOT NULL,
    last_seq            BIGINT      NOT NULL DEFAULT 0,
    buyer_last_read_seq BIGINT      NOT NULL DEFAULT 0,
    store_last_read_seq BIGINT      NOT NULL DEFAULT 0,
    last_message_at     TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT conversations_pair_key UNIQUE (buyer_id, store_id),
    CONSTRAINT conversations_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT conversations_store_check CHECK (store_id > 0),
    CONSTRAINT conversations_last_seq_check CHECK (last_seq >= 0),
    CONSTRAINT conversations_buyer_read_check CHECK (buyer_last_read_seq >= 0 AND buyer_last_read_seq <= last_seq),
    CONSTRAINT conversations_store_read_check CHECK (store_last_read_seq >= 0 AND store_last_read_seq <= last_seq)
);
CREATE INDEX conversations_buyer ON im.conversations (buyer_id, last_message_at DESC, id DESC);
CREATE INDEX conversations_store ON im.conversations (store_id, last_message_at DESC, id DESC);

-- 訊息。寄件人以 (sender_kind, 對話的 buyer_id 或 store_id) 表示，不另存 ID。抹除（刪帳、退場）後保留列（seq 連續），內容與圖片欄位清空，
-- 由 messages_shape_check 保證抹除後內容真的不在。content 只放文字訊息。
CREATE TABLE im.messages (
    id               UUID        PRIMARY KEY,
    conversation_id  UUID        NOT NULL REFERENCES im.conversations (id) ON DELETE CASCADE,
    seq              BIGINT      NOT NULL,
    sender_kind      TEXT        NOT NULL,
    message_type     TEXT        NOT NULL,
    content          TEXT,
    image_asset_id   TEXT,
    image_object_key TEXT,
    created_at       TIMESTAMPTZ NOT NULL,
    erased_at        TIMESTAMPTZ,
    CONSTRAINT messages_seq_key UNIQUE (conversation_id, seq),
    CONSTRAINT messages_seq_check CHECK (seq > 0),
    CONSTRAINT messages_sender_kind_check CHECK (sender_kind IN ('BUYER', 'STORE')),
    CONSTRAINT messages_type_check CHECK (message_type IN ('TEXT', 'IMAGE')),
    CONSTRAINT messages_content_check CHECK (content IS NULL OR (char_length(content) >= 1 AND char_length(content) <= 4096 AND content !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT messages_image_check CHECK ((image_asset_id IS NULL OR image_asset_id <> '') AND (image_object_key IS NULL OR image_object_key <> '')),
    CONSTRAINT messages_shape_check CHECK (
        (erased_at IS NULL AND (
            (message_type = 'TEXT' AND content IS NOT NULL AND image_asset_id IS NULL AND image_object_key IS NULL)
            OR (message_type = 'IMAGE' AND content IS NULL AND image_asset_id IS NOT NULL AND image_object_key IS NOT NULL)
        ))
        OR (erased_at IS NOT NULL AND content IS NULL AND image_asset_id IS NULL AND image_object_key IS NULL)
    )
);

-- 訊息的隱藏紀錄（moderation 用），形狀同 review.hides、social.hides：可多次隱藏與還原（還原留列），同一則訊息未還原的隱藏至多一筆。
CREATE TABLE im.hides (
    id              UUID        PRIMARY KEY,
    message_id      UUID        NOT NULL REFERENCES im.messages (id) ON DELETE CASCADE,
    reason_category TEXT        NOT NULL,
    reason_text     TEXT        NOT NULL DEFAULT '',
    source_ref      TEXT,
    admin_id        BIGINT      NOT NULL,
    hidden_at       TIMESTAMPTZ NOT NULL,
    restored_at     TIMESTAMPTZ,
    restored_by     BIGINT,
    restore_reason  TEXT,
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
CREATE UNIQUE INDEX hides_active_message ON im.hides (message_id) WHERE restored_at IS NULL;
CREATE INDEX hides_message ON im.hides (message_id, hidden_at);

-- 刪帳者與退場店家的複本（kind 是 BUYER 或 STORE，party_id 是 buyer_id 或 store_id）：送出以它擋寄件人與收件人。只插入不刪除，冪等。
CREATE TABLE im.inactive_parties (
    kind     TEXT        NOT NULL,
    party_id BIGINT      NOT NULL,
    since    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (kind, party_id),
    CONSTRAINT inactive_parties_kind_check CHECK (kind IN ('BUYER', 'STORE')),
    CONSTRAINT inactive_parties_party_check CHECK (party_id > 0)
);

-- media 引用的同步版本與退場資產：形狀與語意同 review、social（internal/mediaref）。ref 的列不刪除。
CREATE TABLE im.media_ref_sync (
    ref_type   TEXT        NOT NULL,
    ref_id     TEXT        NOT NULL,
    seq        BIGINT      NOT NULL DEFAULT 1,
    synced_seq BIGINT      NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ref_type, ref_id)
);
CREATE INDEX media_ref_sync_unsynced ON im.media_ref_sync (updated_at) WHERE synced_seq < seq;

CREATE TABLE im.retired_assets (
    ref_type    TEXT        NOT NULL,
    ref_id      TEXT        NOT NULL,
    asset_id    TEXT        NOT NULL,
    retired_at  TIMESTAMPTZ NOT NULL,
    unlinked_at TIMESTAMPTZ,
    PRIMARY KEY (ref_type, ref_id, asset_id)
);
CREATE INDEX retired_assets_retired_at ON im.retired_assets (retired_at);
