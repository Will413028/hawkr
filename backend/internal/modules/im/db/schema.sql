
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA im;

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

CREATE TABLE im.messages (
    id               UUID        PRIMARY KEY,
    conversation_id  UUID        NOT NULL REFERENCES im.conversations (id) ON DELETE CASCADE,
    seq              BIGINT      NOT NULL,
    sender_kind      TEXT        NOT NULL,
    request_id       UUID        NOT NULL,
    message_type     TEXT        NOT NULL,
    content          TEXT,
    image_asset_id   UUID,
    image_object_key TEXT,
    created_at       TIMESTAMPTZ NOT NULL,
    erased_at        TIMESTAMPTZ,
    CONSTRAINT messages_seq_key UNIQUE (conversation_id, seq),
    CONSTRAINT messages_request_key UNIQUE (conversation_id, sender_kind, request_id),
    CONSTRAINT messages_seq_check CHECK (seq > 0),
    CONSTRAINT messages_sender_kind_check CHECK (sender_kind IN ('BUYER', 'STORE')),
    CONSTRAINT messages_type_check CHECK (message_type IN ('TEXT', 'IMAGE')),
    CONSTRAINT messages_content_check CHECK (content IS NULL OR (char_length(content) >= 1 AND char_length(content) <= 4096 AND content !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT messages_image_check CHECK (image_object_key IS NULL OR image_object_key <> ''),
    CONSTRAINT messages_shape_check CHECK (
        (erased_at IS NULL AND (
            (message_type = 'TEXT' AND content IS NOT NULL AND image_asset_id IS NULL AND image_object_key IS NULL)
            OR (message_type = 'IMAGE' AND content IS NULL AND image_asset_id IS NOT NULL AND image_object_key IS NOT NULL)
        ))
        OR (erased_at IS NOT NULL AND content IS NULL AND image_asset_id IS NULL AND image_object_key IS NULL)
    )
);

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

CREATE UNLOGGED TABLE im.presence_streams (
    stream_id  UUID        PRIMARY KEY,
    kind       TEXT        NOT NULL,
    party_id   BIGINT      NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT presence_streams_kind_check CHECK (kind IN ('BUYER', 'STORE')),
    CONSTRAINT presence_streams_party_check CHECK (party_id > 0)
);
CREATE INDEX presence_streams_party ON im.presence_streams (kind, party_id, expires_at);
