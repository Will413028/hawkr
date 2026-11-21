-- moderation 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（moderation.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA moderation;

-- 案件：一個對象同時至多一個進行中案件（partial UNIQUE，A7-D14），同對象的檢舉都掛進它；結案是終態，不重開也不改判，
-- 結案後對同一對象的新檢舉開新案件。對象是貼文、留言、討論區留言（content_id，social 的 UUID）或買家（buyer_id），恰好填一欄；
-- 兩種 ID 型別各有一個 partial UNIQUE（cases_one_open_content、cases_one_open_buyer）。
-- 決定只存在案件（A7-D14、A7-D20）：decision 是 VIOLATION（帶認定的政策分類 decision_category 與處置種類 decision_action）或 NO_VIOLATION，
-- decided_by_admin_id、closed_at 與 decision 同為 NULL 或同不為 NULL；這就是 moderation 的處置決定紀錄，處分本身的紀錄在 social（隱藏、警告）、
-- seller（內容下架措施）與 identity（帳號停權），不另建處置紀錄表。處置種類與對象種類的搭配由 cases_action_target_check 擋；作者是買家或店家由寫入端
-- 向 social 核對。version 是樂觀鎖：結案以 (id, version, closed_at IS NULL) 轉移，同時結案只有一個成功；新檢舉掛進既有案件也加一（同一個 transaction；開案的那筆不加，版本從 1 起），
-- 所以管理員讀到的檢舉集合不是結案時的集合就會版本衝突。
-- measure_id 是 TAKEDOWN 決定建立的 seller 措施 ID（跨 schema 不建 FK），結案時同一個 UPDATE 寫入；內容下架一定有、其他處置與進行中的案件一定是 NULL（cases_measure_check）。
-- reversed_at、reversed_by_admin_id、reverse_reason 是 HIDE 決定的撤銷（ReverseHide）：案件維持結案、決定不改，撤銷記在原決定旁；三欄同有同無，只允許處置是 HIDE 的案件（cases_reversed_*）。
-- decision_category 是 platform/policy.Assessable 的值（模組的 TestChecksMatchGo）；買家分支另外限於 CommunityAssessable，由寫入端檢查。
CREATE TABLE moderation.cases (
    id                  UUID        PRIMARY KEY,
    target_kind         TEXT        NOT NULL,
    content_id          UUID,
    buyer_id            BIGINT,
    version             BIGINT      NOT NULL DEFAULT 1,
    opened_at           TIMESTAMPTZ NOT NULL,
    decision            TEXT,
    decision_category   TEXT,
    decision_action     TEXT,
    decided_by_admin_id BIGINT,
    closed_at           TIMESTAMPTZ,
    measure_id          UUID,
    reversed_at          TIMESTAMPTZ,
    reversed_by_admin_id BIGINT,
    reverse_reason       TEXT,
    CONSTRAINT cases_target_kind_check CHECK (target_kind IN ('POST', 'COMMENT', 'FORUM_ENTRY', 'BUYER', 'REVIEW', 'CHAT_MESSAGE')),
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
        OR (decision_action = 'HIDE' AND target_kind IN ('COMMENT', 'FORUM_ENTRY', 'REVIEW', 'CHAT_MESSAGE'))
        OR (decision_action IN ('WARN', 'BAN') AND target_kind IN ('COMMENT', 'FORUM_ENTRY', 'BUYER', 'REVIEW', 'CHAT_MESSAGE'))
        OR (decision_action = 'TAKEDOWN' AND target_kind IN ('POST', 'COMMENT', 'FORUM_ENTRY'))
    ),
    CONSTRAINT cases_measure_check CHECK ((decision_action IS NOT DISTINCT FROM 'TAKEDOWN') = (measure_id IS NOT NULL)),
    CONSTRAINT cases_reversed_by_check CHECK (reversed_by_admin_id > 0),
    CONSTRAINT cases_reversed_together_check CHECK (
        (reversed_at IS NULL AND reversed_by_admin_id IS NULL AND reverse_reason IS NULL)
        OR (reversed_at IS NOT NULL AND reversed_by_admin_id IS NOT NULL AND reverse_reason IS NOT NULL AND closed_at IS NOT NULL AND reversed_at >= closed_at)
    ),
    CONSTRAINT cases_reversed_hide_check CHECK (reversed_at IS NULL OR decision_action = 'HIDE'),
    CONSTRAINT cases_reverse_reason_check CHECK (reverse_reason IS NULL OR (char_length(reverse_reason) >= 1 AND char_length(reverse_reason) <= 500 AND reverse_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'))
);
-- 一個對象至多一個進行中案件。
CREATE UNIQUE INDEX cases_one_open_content ON moderation.cases (target_kind, content_id) WHERE closed_at IS NULL AND content_id IS NOT NULL;
CREATE UNIQUE INDEX cases_one_open_buyer ON moderation.cases (buyer_id) WHERE closed_at IS NULL AND buyer_id IS NOT NULL;
-- 待辦（A7-1e）與後台列進行中案件依開案時間讀進行中的案件。
CREATE INDEX cases_open_by_time ON moderation.cases (opened_at, id) WHERE closed_at IS NULL;
-- 後台列案件不分狀態或只看已結案：全表索引，開案時間新的在前。
CREATE INDEX cases_by_time ON moderation.cases (opened_at DESC, id DESC);
-- 快照的保留 job 依結案時間找可以清的案件。
CREATE INDEX cases_closed_by_time ON moderation.cases (closed_at) WHERE closed_at IS NOT NULL;

-- 內容快照（A7-D20）：每案每份內容一列，UNIQUE (case_id, content_sha256)；content 是擁有者模組提供、經正規化的快照（canonical JSON 文字），
-- content_sha256 是它的 SHA-256（不用各表的 version：留言、討論區留言沒有 version，buyer.version 改任何欄位都加一）。30 筆檢舉同一份內容只存一份，
-- 內容改過才多一份。結案後 184 天由排程 job 刪除（reports.snapshot_id 是 ON DELETE SET NULL），案件與檢舉列留著、不含內容。
-- asset_ids 是快照引用的圖片資產（A7-D21 證據保全）：快照在 media 以 (GOVERNANCE_EVIDENCE, 快照 ID) 登記引用，快照刪除時才解除。
CREATE TABLE moderation.case_snapshots (
    id             UUID        PRIMARY KEY,
    case_id        UUID        NOT NULL REFERENCES moderation.cases (id),
    content_sha256 BYTEA       NOT NULL,
    content        TEXT        NOT NULL,
    asset_ids      TEXT[]      NOT NULL DEFAULT '{}',
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
CREATE INDEX reports_reporter_time ON moderation.reports (reporter_buyer_id, created_at DESC, id DESC);
CREATE INDEX reports_snapshot ON moderation.reports (snapshot_id) WHERE snapshot_id IS NOT NULL;

-- 檢舉的冪等紀錄（AIP-155，A7-D28 第 3 點）：(檢舉人, request_id) 對應第一次回傳的那一筆檢舉。檢舉可能是新建的，也可能是併入檢舉人在進行中案件裡已有的那一筆；
-- 兩條路徑都記，所以案件結案後同一個 request_id 重送仍回第一次的結果。request_hash 是送出輸入（對象、類別、說明；已正規化）的 SHA-256，重送時比對內容是否相同。
CREATE TABLE moderation.report_requests (
    reporter_buyer_id BIGINT NOT NULL,
    request_id        UUID   NOT NULL,
    report_id         UUID   NOT NULL REFERENCES moderation.reports (id),
    request_hash      BYTEA  NOT NULL,
    PRIMARY KEY (reporter_buyer_id, request_id),
    CONSTRAINT report_requests_reporter_check CHECK (reporter_buyer_id > 0),
    CONSTRAINT report_requests_hash_check CHECK (octet_length(request_hash) = 32)
);
CREATE INDEX report_requests_report ON moderation.report_requests (report_id);

-- 每位檢舉人每個台北日最多 20 筆：一人一列，day 是台北日（platform/twtime.Day）。送出時單一 upsert 原子地「換日重算、未達上限才加一」，
-- 沒有先 count 再比較的空窗；upsert 鎖住這一列，同一檢舉人的並行送出自然序列化。
CREATE TABLE moderation.reporter_daily_counts (
    reporter_buyer_id BIGINT PRIMARY KEY,
    day               DATE   NOT NULL,
    submitted         INT    NOT NULL,
    CONSTRAINT reporter_daily_counts_reporter_check CHECK (reporter_buyer_id > 0),
    CONSTRAINT reporter_daily_counts_submitted_check CHECK (submitted BETWEEN 1 AND 20)
);

-- 快照圖片在 media 的引用同步（internal/mediaref，演算法在 media 模組）：引用只由同步 job 寫入。media_ref_sync 的 seq 先於 job 遞增，synced_seq 結算
-- 後才記下；列不刪除（刪掉再建會讓 seq 從頭算）。ref 是 (ref_type, ref_id)：快照的 ID。
CREATE TABLE moderation.media_ref_sync (
    ref_type   TEXT        NOT NULL,
    ref_id     TEXT        NOT NULL,
    seq        BIGINT      NOT NULL DEFAULT 1,
    synced_seq BIGINT      NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ref_type, ref_id)
);
CREATE INDEX media_ref_sync_unsynced ON moderation.media_ref_sync (updated_at) WHERE synced_seq < seq;

-- 快照刪除時釋放的資產：同步 job 精確解除它們的引用。unlinked_at 是第一次解除成功的時間。
CREATE TABLE moderation.retired_assets (
    ref_type    TEXT        NOT NULL,
    ref_id      TEXT        NOT NULL,
    asset_id    TEXT        NOT NULL,
    retired_at  TIMESTAMPTZ NOT NULL,
    unlinked_at TIMESTAMPTZ,
    PRIMARY KEY (ref_type, ref_id, asset_id)
);
CREATE INDEX retired_assets_retired_at ON moderation.retired_assets (retired_at);
