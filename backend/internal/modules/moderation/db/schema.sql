-- moderation 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（moderation.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA moderation;

-- 案件：一個對象（target_kind, target_key）同時至多一個進行中案件（partial UNIQUE，A7-D14），同對象的檢舉都掛進它；結案是終態，不重開也不改判，
-- 結案後對同一對象的新檢舉開新案件。對象是貼文、留言、討論區留言（content_id，social 的 UUID）或買家（buyer_id），恰好填一欄，target_key 是
-- 它的文字形式，讓兩種 ID 共用同一個唯一鍵（NULL 在唯一索引裡互不相等）。
-- 決定只存在案件（A7-D14、A7-D20）：decision 是 VIOLATION（帶認定的政策分類 decision_category 與處置種類 decision_action）或 NO_VIOLATION，
-- decided_by_admin_id、closed_at 與 decision 同為 NULL 或同不為 NULL；這就是 moderation 的處置決定紀錄，處分本身的紀錄在 social（隱藏、警告）、
-- seller（內容下架措施）與 identity（帳號停權），不另建處置紀錄表。處置種類與對象種類的搭配由 cases_action_target_check 擋；作者是買家或店家由寫入端
-- 向 social 核對。version 是樂觀鎖：結案以 (id, version, closed_at IS NULL) 轉移，同時結案只有一個成功。
-- decision_category 是 platform/policy.Assessable 的值（模組的 TestChecksMatchGo）；買家分支另外限於 CommunityAssessable，由寫入端檢查。
CREATE TABLE moderation.cases (
    id                  UUID        PRIMARY KEY,
    target_kind         TEXT        NOT NULL,
    content_id          UUID,
    buyer_id            BIGINT,
    target_key          TEXT        NOT NULL GENERATED ALWAYS AS (COALESCE(content_id::text, buyer_id::text)) STORED,
    version             BIGINT      NOT NULL DEFAULT 1,
    opened_at           TIMESTAMPTZ NOT NULL,
    decision            TEXT,
    decision_category   TEXT,
    decision_action     TEXT,
    decided_by_admin_id BIGINT,
    closed_at           TIMESTAMPTZ,
    CONSTRAINT cases_target_kind_check CHECK (target_kind IN ('POST', 'COMMENT', 'FORUM_ENTRY', 'BUYER')),
    CONSTRAINT cases_target_check CHECK (
        (target_kind = 'BUYER' AND buyer_id > 0 AND content_id IS NULL)
        OR (target_kind <> 'BUYER' AND content_id IS NOT NULL AND buyer_id IS NULL)
    ),
    CONSTRAINT cases_version_check CHECK (version >= 1),
    CONSTRAINT cases_decision_check CHECK (decision IN ('VIOLATION', 'NO_VIOLATION')),
    CONSTRAINT cases_decision_category_check CHECK (decision_category IN ('FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT', 'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT')),
    CONSTRAINT cases_decision_action_check CHECK (decision_action IN ('HIDE', 'WARN', 'BAN', 'TAKEDOWN')),
    CONSTRAINT cases_decided_by_check CHECK (decided_by_admin_id > 0),
    CONSTRAINT cases_closed_together_check CHECK (
        (decision IS NULL AND decided_by_admin_id IS NULL AND closed_at IS NULL)
        OR (decision IS NOT NULL AND decided_by_admin_id IS NOT NULL AND closed_at IS NOT NULL AND closed_at >= opened_at)
    ),
    CONSTRAINT cases_decision_shape_check CHECK (
        decision IS NULL
        OR (decision = 'VIOLATION' AND decision_category IS NOT NULL AND decision_action IS NOT NULL)
        OR (decision = 'NO_VIOLATION' AND decision_category IS NULL AND decision_action IS NULL)
    ),
    CONSTRAINT cases_action_target_check CHECK (
        decision_action IS NULL
        OR (decision_action = 'HIDE' AND target_kind IN ('COMMENT', 'FORUM_ENTRY'))
        OR (decision_action IN ('WARN', 'BAN') AND target_kind IN ('COMMENT', 'FORUM_ENTRY', 'BUYER'))
        OR (decision_action = 'TAKEDOWN' AND target_kind IN ('POST', 'COMMENT', 'FORUM_ENTRY'))
    )
);
-- 一個對象至多一個進行中案件。
CREATE UNIQUE INDEX cases_one_open ON moderation.cases (target_kind, target_key) WHERE closed_at IS NULL;
-- 待辦（A7-1e）依開案時間讀進行中的案件。
CREATE INDEX cases_open_by_time ON moderation.cases (opened_at, id) WHERE closed_at IS NULL;
-- 快照的保留 job 依結案時間找可以清的案件。
CREATE INDEX cases_closed_by_time ON moderation.cases (closed_at) WHERE closed_at IS NOT NULL;

-- 內容快照（A7-D20）：每案每份內容一列，UNIQUE (case_id, content_sha256)；content 是擁有者模組提供、經正規化的快照（canonical JSON 文字），
-- content_sha256 是它的 SHA-256（不用各表的 version：留言、討論區留言沒有 version，buyer.version 改任何欄位都加一）。30 筆檢舉同一份內容只存一份，
-- 內容改過才多一份。結案後 184 天由排程 job 刪除（reports.snapshot_id 是 ON DELETE SET NULL），案件與檢舉列留著、不含內容。
CREATE TABLE moderation.case_snapshots (
    id             UUID        PRIMARY KEY,
    case_id        UUID        NOT NULL REFERENCES moderation.cases (id),
    content_sha256 BYTEA       NOT NULL,
    content        TEXT        NOT NULL,
    captured_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT case_snapshots_case_content_key UNIQUE (case_id, content_sha256),
    CONSTRAINT case_snapshots_sha256_check CHECK (octet_length(content_sha256) = 32),
    CONSTRAINT case_snapshots_content_check CHECK (char_length(content) >= 2)
);

-- 檢舉：一案一人一筆（UNIQUE (case_id, reporter_buyer_id)），結案後新案件自然可以再檢舉。檢舉不存結果，各檢舉的結果由所屬案件的決定推導（查詢時 join）。
-- reporter_buyer_id 是檢舉的買家（buyer 擁有的業務鍵，不建跨 schema 的 FK）；category 是 platform/policy.CommunityReporterSelectable 的值
-- （模組的 TestChecksMatchGo）；snapshot_id 是送出時凍結的那一份，保留期後為 NULL。
CREATE TABLE moderation.reports (
    id                UUID        PRIMARY KEY,
    case_id           UUID        NOT NULL REFERENCES moderation.cases (id),
    reporter_buyer_id BIGINT      NOT NULL,
    category          TEXT        NOT NULL,
    description       TEXT,
    snapshot_id       UUID        REFERENCES moderation.case_snapshots (id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL,
    CONSTRAINT reports_case_reporter_key UNIQUE (case_id, reporter_buyer_id),
    CONSTRAINT reports_reporter_check CHECK (reporter_buyer_id > 0),
    CONSTRAINT reports_category_check CHECK (category IN ('SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT', 'MISLEADING_INFO', 'FOOD_SAFETY', 'FRAUD', 'OTHER')),
    CONSTRAINT reports_description_check CHECK (description IS NULL OR (char_length(description) >= 1 AND char_length(description) <= 500 AND description !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'))
);
CREATE INDEX reports_case_time ON moderation.reports (case_id, created_at, id);
CREATE INDEX reports_snapshot ON moderation.reports (snapshot_id) WHERE snapshot_id IS NOT NULL;

-- 每位檢舉人每個台北日最多 20 筆：一人一列，day 是台北日（platform/twtime.Day）。送出時單一 upsert 原子地「換日重算、未達上限才加一」，
-- 沒有先 count 再比較的空窗；upsert 鎖住這一列，同一檢舉人的並行送出自然序列化。
CREATE TABLE moderation.reporter_daily_counts (
    reporter_buyer_id BIGINT PRIMARY KEY,
    day               DATE   NOT NULL,
    submitted         INT    NOT NULL,
    CONSTRAINT reporter_daily_counts_reporter_check CHECK (reporter_buyer_id > 0),
    CONSTRAINT reporter_daily_counts_submitted_check CHECK (submitted BETWEEN 1 AND 20)
);
