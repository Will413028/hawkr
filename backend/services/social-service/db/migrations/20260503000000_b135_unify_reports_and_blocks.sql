-- B-135: 內容檢舉 / 使用者封鎖 / Admin 審核基礎 schema
-- 本 migration 將既有 cms_post_report 擴充為通用 reports 表，
-- 並新增 user_blocks、moderation_actions、cms_comment/cms_seller_forum 的 hidden_at 欄位。

-- ===========================================================================
-- 1. cms_post_report → reports
-- ===========================================================================

-- 1.1 加新欄位（既有列）
ALTER TABLE cms_post_report
    ADD COLUMN target_type     VARCHAR(30) NOT NULL DEFAULT 'POST',
    ADD COLUMN target_id       BIGINT,
    ADD COLUMN evidence        JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN target_snapshot JSONB,
    ADD COLUMN status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    ADD COLUMN resolved_action VARCHAR(20),
    ADD COLUMN resolved_by     BIGINT,
    ADD COLUMN resolved_at     TIMESTAMPTZ,
    ADD COLUMN update_time     TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- 1.2 Backfill：既有 row 的 target_id = post_id；status = DISMISSED（B-135 之前未經正式 admin 流程）
UPDATE cms_post_report SET
    target_id       = post_id,
    status          = 'DISMISSED',
    resolved_action = 'DISMISS',
    resolved_at     = create_time;

-- 1.3 鎖定 target_id NOT NULL
ALTER TABLE cms_post_report ALTER COLUMN target_id SET NOT NULL;

-- 1.4 移除舊 UNIQUE 與舊 column post_id（24h idempotency 改 application 層）
ALTER TABLE cms_post_report DROP CONSTRAINT cms_post_report_post_id_reporter_id_key;
DROP INDEX IF EXISTS idx_cms_post_report_post;
ALTER TABLE cms_post_report DROP COLUMN post_id;

-- 1.5 加 target_type CHECK
ALTER TABLE cms_post_report
    ADD CONSTRAINT chk_reports_target_type
    CHECK (target_type IN ('REVIEW','POST','POST_COMMENT','SELLER_FORUM','CHAT_MESSAGE','USER'));

-- 1.6 RENAME 表 + 同步 rename PK 名稱
ALTER TABLE cms_post_report RENAME TO reports;
ALTER TABLE reports RENAME CONSTRAINT cms_post_report_pkey TO reports_pkey;

-- 1.6.1 backfill 完成後 drop target_type 的 DEFAULT（schema.sql 不宣告 default）
ALTER TABLE reports ALTER COLUMN target_type DROP DEFAULT;

-- 1.7 新 indexes
CREATE INDEX idx_reports_status_create   ON reports (status, create_time DESC);
CREATE INDEX idx_reports_reporter_target ON reports (reporter_id, target_type, target_id, create_time DESC);
CREATE INDEX idx_reports_target          ON reports (target_type, target_id, create_time DESC);

-- ===========================================================================
-- 2. user_blocks
-- ===========================================================================

CREATE TABLE user_blocks (
    id          BIGINT PRIMARY KEY,
    blocker_id  BIGINT NOT NULL,
    blocked_id  BIGINT NOT NULL,
    create_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (blocker_id, blocked_id),
    CONSTRAINT chk_user_blocks_not_self CHECK (blocker_id <> blocked_id)
);
CREATE INDEX idx_user_blocks_blocked ON user_blocks (blocked_id);

-- ===========================================================================
-- 3. moderation_actions
-- ===========================================================================

CREATE TABLE moderation_actions (
    id                BIGINT PRIMARY KEY,
    report_id         BIGINT,
    admin_id          BIGINT,
    user_id           BIGINT,
    action            VARCHAR(30) NOT NULL,
    note              TEXT,
    attempted_content TEXT,
    create_time       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_moderation_actions_report ON moderation_actions (report_id);
CREATE INDEX idx_moderation_actions_admin  ON moderation_actions (admin_id, create_time DESC);
CREATE INDEX idx_moderation_actions_user   ON moderation_actions (user_id, create_time DESC);

-- ===========================================================================
-- 4. cms_comment / cms_seller_forum 加 hidden_at + hidden_reason
-- ===========================================================================

ALTER TABLE cms_comment
    ADD COLUMN hidden_at     TIMESTAMPTZ,
    ADD COLUMN hidden_reason TEXT;

ALTER TABLE cms_seller_forum
    ADD COLUMN hidden_at     TIMESTAMPTZ,
    ADD COLUMN hidden_reason TEXT;

-- ===========================================================================
-- 5. Pre-existing schema cleanup（atlas migrate diff 偵測到的舊 SERIAL 殘留）
-- 既有初始 migration 用 BIGSERIAL 後改 BIGINT，但 sequences 與 DEFAULT 仍掛在 dev
-- ===========================================================================

ALTER TABLE cms_comment      ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS cms_comment_id_seq;
ALTER TABLE cms_post         ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS cms_post_id_seq;
ALTER TABLE cms_seller_forum ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS cms_seller_forum_id_seq;
