-- PayUni Webhook Service 初始化
-- webhook_inbox（去重 + 稽核）+ outbox_events（可靠投遞）

CREATE TABLE webhook_inbox (
    id                  BIGSERIAL PRIMARY KEY,
    provider            VARCHAR(20)  NOT NULL DEFAULT '8241',  -- WebhookProvider.PAYUNI
    mer_id              VARCHAR(32)  NOT NULL,
    webhook_type        VARCHAR(50)  NOT NULL,                 -- PayUniEventType value（8251-8254）
    event_key           VARCHAR(255) NOT NULL,
    status              VARCHAR(20)  NOT NULL DEFAULT '8211',  -- WebhookInboxStatus.RECEIVED
    verify_status       VARCHAR(20)  NOT NULL DEFAULT '8231',  -- WebhookVerifyStatus.PASSED
    raw_envelope        JSONB,
    decrypted_payload   JSONB,
    received_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    processed_at        TIMESTAMPTZ,
    -- §10.5 PayUni 端事件時間（從 decrypted_payload 抽出，可 NULL）
    provider_event_at   TIMESTAMPTZ,
    last_error          TEXT,
    CONSTRAINT uq_inbox_event UNIQUE (provider, webhook_type, event_key)
);

CREATE TABLE outbox_events (
    id              BIGSERIAL    PRIMARY KEY,
    aggregate_type  VARCHAR(50)  NOT NULL DEFAULT 'PAYUNI_WEBHOOK',
    aggregate_id    VARCHAR(255) NOT NULL,
    exchange        VARCHAR(100) NOT NULL,
    topic           VARCHAR(100) NOT NULL,
    message_key     VARCHAR(255) NOT NULL,
    payload         JSONB        NOT NULL,
    headers         JSONB,
    status          VARCHAR(20)  NOT NULL DEFAULT '8221',  -- OutboxEventStatus.PENDING
    retry_count     SMALLINT     NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    sent_at         TIMESTAMPTZ,
    last_error      TEXT,
    CONSTRAINT chk_outbox_status CHECK (status IN ('8221', '8222', '8223', '8224')),
    -- §4.5.3 replay-safe：T2 重放時 INSERT 命中 UNIQUE 視為已建立、跳過
    -- AGENT_NOTIFY 雙通道（payuni.events / seller.events）因 exchange/topic 不同仍各自允許
    CONSTRAINT uq_outbox_aggregate_route UNIQUE (aggregate_id, exchange, topic)
);

CREATE INDEX idx_outbox_status_retry ON outbox_events(status, next_retry_at)
    WHERE status IN ('8221', '8223');  -- PENDING / FAILED（待重送）
