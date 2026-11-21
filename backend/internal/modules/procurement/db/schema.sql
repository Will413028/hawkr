-- procurement 模組的 desired state（Atlas 與 sqlc 共用）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（procurement.<表>），不建跨 schema 的 FK。buyer_id、store_id 是外部鍵（BIGINT，不建 FK）；
-- 自己的實體用 UUIDv7（應用層產生、不設 DEFAULT，ADR 2026-09-25-row-identity-by-data-kind）。
--
-- 31 張舊表依 A9-D19（table-audit.md 的 S1–S11）收斂：沒有 candidate_leads（AD4）、controller_kind（AD12）、external lane 與 prospect（A9-D1）、
-- 預約表（A9-D9）、outbox 與 *_commands（M11、S4）。頭尾不得有空白的文字欄用 `trimtext.Check` 展開（單體規則 9），登記在 cmd/hawkr 的 trimmedColumns。
--
-- 買家刪除（S10）：requests、price_previews、confirmed_rfq_snapshots、approvals、publications、acceptances 的 buyer_id 可為 NULL：已發布的詢價與賣家
-- 看得到的紀錄保留（匿名，保留期跟詢價的生命週期），其餘買家資料（對話、輪次、候選、run、額度）刪除。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA procurement;

-- 一張採購需求。「現行 revision」只有一種表示法：request_revisions 的 partial unique（S2）。
CREATE TABLE procurement.requests (
    id UUID PRIMARY KEY,
    buyer_id BIGINT,
    status TEXT NOT NULL DEFAULT 'CLARIFYING',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT requests_status_check CHECK (status IN ('CLARIFYING', 'REQUIREMENT_READY', 'MATCHING')),
    CONSTRAINT requests_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX requests_buyer ON procurement.requests (buyer_id, created_at DESC) WHERE buyer_id IS NOT NULL;

-- 需求的一個版本，append-only。slots 是具名欄位（S3）；欄位 NULL 表示買家還沒填。
CREATE TABLE procurement.request_revisions (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    revision_no INT NOT NULL,
    parent_revision_id UUID REFERENCES procurement.request_revisions (id),
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    item TEXT,
    item_resolution_kind TEXT,
    item_resolution_label TEXT,
    quantity INT,
    unit TEXT,
    packaging TEXT,
    budget_per_unit BIGINT,
    delivery_city TEXT,
    delivery_date DATE,
    exact_match_required BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT request_revisions_no_check CHECK (revision_no >= 1),
    CONSTRAINT request_revisions_status_check CHECK (status IN ('ACTIVE', 'SUPERSEDED')),
    CONSTRAINT request_revisions_resolution_check CHECK (
        (item_resolution_kind IS NULL) = (item_resolution_label IS NULL)
        AND (item_resolution_kind IS NULL OR item_resolution_kind IN ('CATEGORY_CONFIRMED', 'BUYER_DESCRIBED'))
    ),
    CONSTRAINT request_revisions_item_check CHECK (char_length(item) BETWEEN 1 AND 200 AND item !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_resolution_label_check CHECK (char_length(item_resolution_label) BETWEEN 1 AND 200 AND item_resolution_label !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_unit_check CHECK (char_length(unit) BETWEEN 1 AND 40 AND unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_packaging_check CHECK (char_length(packaging) BETWEEN 1 AND 200 AND packaging !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_delivery_city_check CHECK (char_length(delivery_city) BETWEEN 1 AND 100 AND delivery_city !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_quantity_check CHECK (quantity > 0),
    CONSTRAINT request_revisions_budget_check CHECK (budget_per_unit >= 0)
);
CREATE UNIQUE INDEX request_revisions_no ON procurement.request_revisions (request_id, revision_no);
-- 一張需求同時只有一個現行版本，由資料庫擋雙頭。
CREATE UNIQUE INDEX request_revisions_active ON procurement.request_revisions (request_id) WHERE status = 'ACTIVE';

-- 一個買家的對話，與 requests 1:1。ADK session ID 是 request ID 的字串（跨掛起與回答兩個 run 存活，S7）。軟刪除只在這裡：買家刪對話後需求與已發布的詢價保留。
CREATE TABLE procurement.conversations (
    id UUID PRIMARY KEY,
    buyer_id BIGINT NOT NULL,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    title_source TEXT NOT NULL DEFAULT 'AUTO',
    last_message_preview TEXT NOT NULL DEFAULT '',
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT conversations_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT conversations_title_source_check CHECK (title_source IN ('AUTO', 'USER')),
    CONSTRAINT conversations_title_check CHECK (char_length(title) <= 200 AND title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT conversations_preview_check CHECK (char_length(last_message_preview) <= 200 AND last_message_preview !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);
CREATE UNIQUE INDEX conversations_request ON procurement.conversations (request_id);
CREATE INDEX conversations_buyer_active ON procurement.conversations (buyer_id, updated_at DESC, id DESC) WHERE deleted_at IS NULL;

-- 一輪釐清（舊 answer_patches，S2）：冪等鍵、稽核紀錄與對話訊息的唯一來源。
CREATE TABLE procurement.turns (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES procurement.conversations (id) ON DELETE CASCADE,
    buyer_id BIGINT NOT NULL,
    result_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    buyer_message TEXT NOT NULL DEFAULT '',
    assistant_reply TEXT NOT NULL DEFAULT '',
    steps JSONB NOT NULL DEFAULT '[]',
    recorded_this_turn JSONB NOT NULL DEFAULT '[]',
    client_request_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT turns_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT turns_source_check CHECK (source IN ('LLM_EXTRACT', 'ANSWER', 'REWIND')),
    CONSTRAINT turns_buyer_message_check CHECK (char_length(buyer_message) <= 2000 AND buyer_message !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT turns_assistant_reply_check CHECK (char_length(assistant_reply) <= 4000 AND assistant_reply !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT turns_steps_check CHECK (jsonb_typeof(steps) = 'array'),
    CONSTRAINT turns_recorded_check CHECK (jsonb_typeof(recorded_this_turn) = 'array')
);
CREATE UNIQUE INDEX turns_idempotency ON procurement.turns (buyer_id, client_request_id);
CREATE INDEX turns_request ON procurement.turns (request_id, created_at DESC);

-- 一次候選搜尋的結果（S1）：不承載 run 的生命週期（在 runs），沒有 lease、fencing_token、policy_version。失敗後買家重送就是新的一列。
CREATE TABLE procurement.candidate_sets (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    request_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    buyer_id BIGINT NOT NULL,
    client_request_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT candidate_sets_buyer_check CHECK (buyer_id > 0)
);
CREATE UNIQUE INDEX candidate_sets_idempotency ON procurement.candidate_sets (request_revision_id, client_request_id);
CREATE INDEX candidate_sets_request ON procurement.candidate_sets (request_id, created_at DESC);

-- 候選賣家：沒有店名快照（S9，名稱讀取時由 StoreCard 組裝），只收 OPEN 的店；selected 併入（S11）。自由文字（研究摘要等）不入庫（A9-D8）。
CREATE TABLE procurement.candidates (
    id UUID PRIMARY KEY,
    candidate_set_id UUID NOT NULL REFERENCES procurement.candidate_sets (id) ON DELETE CASCADE,
    store_id BIGINT NOT NULL,
    source_id TEXT NOT NULL,
    score INT NOT NULL DEFAULT 0,
    rank INT NOT NULL DEFAULT 0,
    matched_facts JSONB NOT NULL DEFAULT '[]',
    unverifiable_facts JSONB NOT NULL DEFAULT '[]',
    facts_as_of TIMESTAMPTZ NOT NULL,
    selected BOOLEAN NOT NULL DEFAULT FALSE,
    selected_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT candidates_store_check CHECK (store_id > 0),
    CONSTRAINT candidates_source_check CHECK (char_length(source_id) >= 1),
    CONSTRAINT candidates_matched_check CHECK (jsonb_typeof(matched_facts) = 'array'),
    CONSTRAINT candidates_unverifiable_check CHECK (jsonb_typeof(unverifiable_facts) = 'array'),
    CONSTRAINT candidates_selected_check CHECK (selected = (selected_at IS NOT NULL))
);
CREATE UNIQUE INDEX candidates_set_store ON procurement.candidates (candidate_set_id, store_id);
CREATE INDEX candidates_set_rank ON procurement.candidates (candidate_set_id, rank);

-- 詢價價格預覽，只 INSERT：沒有 version（S5），hash 由 Go 從欄位重算。
CREATE TABLE procurement.price_previews (
    id UUID PRIMARY KEY,
    buyer_id BIGINT,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    request_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    range_lower_amount BIGINT NOT NULL,
    range_upper_amount BIGINT NOT NULL,
    basis_quantity INT NOT NULL,
    basis_unit TEXT NOT NULL,
    total_lower_amount BIGINT,
    total_upper_amount BIGINT,
    total_quantity INT,
    total_unit TEXT,
    client_request_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT price_previews_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT price_previews_range_check CHECK (range_lower_amount >= 0 AND range_upper_amount >= range_lower_amount),
    CONSTRAINT price_previews_basis_check CHECK (basis_quantity > 0 AND char_length(basis_unit) >= 1),
    CONSTRAINT price_previews_total_check CHECK (
        (total_lower_amount IS NULL) = (total_upper_amount IS NULL)
        AND (total_lower_amount IS NULL) = (total_quantity IS NULL)
        AND (total_lower_amount IS NULL) = (total_unit IS NULL)
    ),
    CONSTRAINT price_previews_basis_unit_check CHECK (char_length(basis_unit) <= 40 AND basis_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT price_previews_total_unit_check CHECK (char_length(total_unit) <= 40 AND total_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);
CREATE UNIQUE INDEX price_previews_idempotency ON procurement.price_previews (buyer_id, client_request_id);
CREATE INDEX price_previews_request ON procurement.price_previews (request_id);

-- 買家確認的詢價快照，只 INSERT。buyer_note 是買家原文、轉呈賣家（S3：具名文字欄，登記 trimmedColumns）。
CREATE TABLE procurement.confirmed_rfq_snapshots (
    id UUID PRIMARY KEY,
    buyer_id BIGINT,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    request_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    price_preview_id UUID NOT NULL REFERENCES procurement.price_previews (id) ON DELETE CASCADE,
    buyer_note TEXT NOT NULL DEFAULT '',
    client_request_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT confirmed_rfq_snapshots_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT confirmed_rfq_snapshots_note_check CHECK (char_length(buyer_note) <= 500 AND buyer_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);
CREATE UNIQUE INDEX confirmed_rfq_snapshots_idempotency ON procurement.confirmed_rfq_snapshots (buyer_id, client_request_id);
CREATE INDEX confirmed_rfq_snapshots_request ON procurement.confirmed_rfq_snapshots (request_id, created_at DESC);

-- 發布核准（預覽發布時建立）。只存參照，不複製 hash 與快照欄位（S5）；hash 由 Go 從參照的不可變列重算。
-- schema_version 保留：hash 要知道 preimage 的版本才能重算（S3）。
CREATE TABLE procurement.rfq_publication_approvals (
    id UUID PRIMARY KEY,
    version INT NOT NULL DEFAULT 1,
    buyer_id BIGINT,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    request_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    confirmed_rfq_snapshot_id UUID NOT NULL REFERENCES procurement.confirmed_rfq_snapshots (id) ON DELETE CASCADE,
    response_deadline DATE,
    audience_mode TEXT NOT NULL,
    audience_policy_id TEXT,
    audience_policy_version INT,
    state TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    client_request_id UUID NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT rfq_publication_approvals_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT rfq_publication_approvals_version_check CHECK (version >= 1),
    CONSTRAINT rfq_publication_approvals_mode_check CHECK (audience_mode IN ('WISH_POOL', 'DIRECT_RECIPIENTS')),
    CONSTRAINT rfq_publication_approvals_state_check CHECK (state IN ('PENDING', 'EXECUTING', 'SUCCEEDED')),
    CONSTRAINT rfq_publication_approvals_schema_check CHECK (char_length(schema_version) >= 1),
    CONSTRAINT rfq_publication_approvals_policy_check CHECK (
        (audience_mode = 'WISH_POOL') = (audience_policy_id IS NOT NULL AND audience_policy_version IS NOT NULL)
    )
);
CREATE UNIQUE INDEX rfq_publication_approvals_idempotency ON procurement.rfq_publication_approvals (buyer_id, client_request_id);
CREATE INDEX rfq_publication_approvals_request ON procurement.rfq_publication_approvals (request_id, created_at DESC);

-- 核准當下凍結的受眾（指定對象時的店家，順序有意義）。值的集合，主鍵 (approval_id, position)。
CREATE TABLE procurement.rfq_publication_approval_recipients (
    approval_id UUID NOT NULL REFERENCES procurement.rfq_publication_approvals (id) ON DELETE CASCADE,
    position INT NOT NULL,
    store_id BIGINT NOT NULL,
    PRIMARY KEY (approval_id, position),
    CONSTRAINT rfq_publication_approval_recipients_position_check CHECK (position >= 0),
    CONSTRAINT rfq_publication_approval_recipients_store_check CHECK (store_id > 0)
);
CREATE UNIQUE INDEX rfq_publication_approval_recipients_store ON procurement.rfq_publication_approval_recipients (approval_id, store_id);

-- 一次發布。state 只收有寫入者的值 PUBLISHED（S6）；一個核准最多一次發布；冪等鍵放在這裡（S4，舊 rfq_publication_commands 刪除）。
CREATE TABLE procurement.rfq_publications (
    id UUID PRIMARY KEY,
    version INT NOT NULL DEFAULT 1,
    buyer_id BIGINT,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    request_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    approval_id UUID NOT NULL REFERENCES procurement.rfq_publication_approvals (id) ON DELETE CASCADE,
    state TEXT NOT NULL DEFAULT 'PUBLISHED',
    schema_version TEXT NOT NULL,
    client_request_id UUID NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT rfq_publications_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT rfq_publications_version_check CHECK (version >= 1),
    CONSTRAINT rfq_publications_state_check CHECK (state IN ('PUBLISHED')),
    CONSTRAINT rfq_publications_schema_check CHECK (char_length(schema_version) >= 1)
);
CREATE UNIQUE INDEX rfq_publications_approval ON procurement.rfq_publications (approval_id);
CREATE UNIQUE INDEX rfq_publications_idempotency ON procurement.rfq_publications (buyer_id, client_request_id);
CREATE INDEX rfq_publications_request ON procurement.rfq_publications (request_id, created_at DESC);

-- 發給一家店的邀請。與 River job 同 transaction 建立，一出生就是 QUEUED（沒有 NOT_SENT）。
CREATE TABLE procurement.rfq_publication_invites (
    id UUID PRIMARY KEY,
    publication_id UUID NOT NULL REFERENCES procurement.rfq_publications (id) ON DELETE CASCADE,
    position INT NOT NULL,
    store_id BIGINT NOT NULL,
    delivery_state TEXT NOT NULL DEFAULT 'QUEUED',
    delivery_state_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT rfq_publication_invites_position_check CHECK (position >= 0),
    CONSTRAINT rfq_publication_invites_store_check CHECK (store_id > 0),
    CONSTRAINT rfq_publication_invites_state_check CHECK (delivery_state IN ('QUEUED', 'SENT', 'FAILED'))
);
CREATE UNIQUE INDEX rfq_publication_invites_store ON procurement.rfq_publication_invites (publication_id, store_id);
CREATE UNIQUE INDEX rfq_publication_invites_position ON procurement.rfq_publication_invites (publication_id, position);

-- 一家店對一次發布的回價（身分列）。沒有 buyer_id（由 request 推出）、沒有列版本（回價版次是 latest_version）。
-- write_closed_reason 只收有寫入者的 DECLINED（S6）。
CREATE TABLE procurement.seller_responses (
    id UUID PRIMARY KEY,
    publication_id UUID NOT NULL REFERENCES procurement.rfq_publications (id) ON DELETE CASCADE,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    store_id BIGINT NOT NULL,
    latest_version INT NOT NULL DEFAULT 0,
    withdrawn_version INT,
    withdrawn_at TIMESTAMPTZ,
    write_closed_at TIMESTAMPTZ,
    write_closed_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT seller_responses_store_check CHECK (store_id > 0),
    CONSTRAINT seller_responses_latest_check CHECK (latest_version >= 0),
    CONSTRAINT seller_responses_withdrawn_pair CHECK ((withdrawn_version IS NULL) = (withdrawn_at IS NULL)),
    CONSTRAINT seller_responses_write_closed_pair CHECK ((write_closed_at IS NULL) = (write_closed_reason IS NULL)),
    CONSTRAINT seller_responses_write_closed_reason CHECK (write_closed_reason IS NULL OR write_closed_reason IN ('DECLINED'))
);
CREATE UNIQUE INDEX seller_responses_publication_store ON procurement.seller_responses (publication_id, store_id);
CREATE INDEX seller_responses_request_submitted ON procurement.seller_responses (request_id, created_at DESC) WHERE latest_version > 0;

-- 賣家回價的一個版本，不可變。沒有驗證欄位（A9-D1）、沒有 hash 複本（S5，由 Go 重算）。
-- 品相欄位與 conversion_basis 的去留是 A9-3c 開工要交 Will 的題目（計畫 A9-3 停止條件），這裡只保留契約必填的形狀。
CREATE TABLE procurement.seller_response_versions (
    id UUID PRIMARY KEY,
    seller_response_id UUID NOT NULL REFERENCES procurement.seller_responses (id) ON DELETE CASCADE,
    version INT NOT NULL,
    response_type TEXT NOT NULL,
    offered_quality_grade TEXT NOT NULL,
    offered_quality_inspection_notes TEXT NOT NULL,
    unit_basis_quantity NUMERIC(14, 3) NOT NULL,
    unit_basis_unit TEXT NOT NULL,
    offered_quantity NUMERIC(14, 3) NOT NULL,
    offered_quantity_unit TEXT NOT NULL,
    conversion_basis TEXT NOT NULL,
    currency TEXT NOT NULL,
    goods_amount BIGINT NOT NULL,
    shipping_amount BIGINT NOT NULL,
    tax_amount BIGINT NOT NULL,
    tax_inclusion_basis TEXT NOT NULL,
    total_amount BIGINT NOT NULL,
    lead_time_days INT,
    promised_delivery_at TIMESTAMPTZ,
    valid_until TIMESTAMPTZ NOT NULL,
    terms_note TEXT NOT NULL DEFAULT '',
    deviation_disclosure TEXT NOT NULL DEFAULT '',
    outside_buyer_requested_range BOOLEAN NOT NULL,
    publication_version INT NOT NULL,
    client_request_id UUID NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT seller_response_versions_version_check CHECK (version >= 1),
    CONSTRAINT seller_response_versions_type_check CHECK (response_type IN ('MANUAL_QUOTE', 'ACCEPT_NON_PRICE_TERMS_WITH_QUOTE')),
    CONSTRAINT seller_response_versions_tax_basis_check CHECK (tax_inclusion_basis IN ('TAX_INCLUDED', 'TAX_EXCLUDED')),
    CONSTRAINT seller_response_versions_amounts_check CHECK (goods_amount >= 0 AND shipping_amount >= 0 AND tax_amount >= 0),
    CONSTRAINT seller_response_versions_total_check CHECK (total_amount = goods_amount + shipping_amount + tax_amount),
    CONSTRAINT seller_response_versions_quantities_check CHECK (unit_basis_quantity > 0 AND offered_quantity > 0),
    CONSTRAINT seller_response_versions_publication_version_check CHECK (publication_version >= 1),
    CONSTRAINT seller_response_versions_notes_check CHECK (char_length(offered_quality_inspection_notes) <= 1000 AND offered_quality_inspection_notes !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_terms_check CHECK (char_length(terms_note) <= 1000 AND terms_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_deviation_check CHECK (char_length(deviation_disclosure) <= 1000 AND deviation_disclosure !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_unit_basis_unit_check CHECK (char_length(unit_basis_unit) BETWEEN 1 AND 40 AND unit_basis_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_offered_unit_check CHECK (char_length(offered_quantity_unit) BETWEEN 1 AND 40 AND offered_quantity_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_quality_grade_check CHECK (char_length(offered_quality_grade) BETWEEN 1 AND 100 AND offered_quality_grade !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_conversion_basis_check CHECK (char_length(conversion_basis) <= 500 AND conversion_basis !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);
CREATE UNIQUE INDEX seller_response_versions_version ON procurement.seller_response_versions (seller_response_id, version);
CREATE UNIQUE INDEX seller_response_versions_idempotency ON procurement.seller_response_versions (seller_response_id, client_request_id);

-- 賣家的回價草稿：一家店一份、整份覆寫；payload 是部分填寫的 SellerQuote（JSONB 合理：形狀不固定）。
CREATE TABLE procurement.seller_response_drafts (
    seller_response_id UUID PRIMARY KEY REFERENCES procurement.seller_responses (id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT seller_response_drafts_payload_check CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT seller_response_drafts_version_check CHECK (version >= 1)
);

-- 買家的候選報價清單（買家資料，買家刪除時清掉）。
CREATE TABLE procurement.buyer_shortlist_entries (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    seller_response_id UUID NOT NULL REFERENCES procurement.seller_responses (id) ON DELETE CASCADE,
    seller_response_version INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT buyer_shortlist_entries_version_check CHECK (seller_response_version >= 1)
);
CREATE UNIQUE INDEX buyer_shortlist_entries_response ON procurement.buyer_shortlist_entries (request_id, seller_response_id);

-- 買家接受一份報價。state 只收有寫入者的值（S6，A9-D19 修訂）：ACTIVE，與建單重試用盡後的 ORDER_FAILED；live 鎖只擋 ACTIVE，
-- 所以建單失敗後買家可以再接受（同家或別家）。成交價（三金額）凍結在這裡；接受當下的快照 hash 與回價 hash 是買家 CAS 的對象，留作稽核（S5 的 (c)）。
CREATE TABLE procurement.buyer_quote_acceptances (
    id UUID PRIMARY KEY,
    buyer_id BIGINT,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    publication_id UUID NOT NULL REFERENCES procurement.rfq_publications (id) ON DELETE CASCADE,
    seller_response_id UUID NOT NULL REFERENCES procurement.seller_responses (id) ON DELETE CASCADE,
    seller_response_version INT NOT NULL,
    seller_response_payload_hash TEXT NOT NULL,
    confirmed_rfq_payload_hash TEXT NOT NULL,
    store_id BIGINT NOT NULL,
    currency TEXT NOT NULL,
    goods_amount BIGINT NOT NULL,
    shipping_amount BIGINT NOT NULL,
    tax_amount BIGINT NOT NULL,
    accepted_deal_price BIGINT NOT NULL,
    state TEXT NOT NULL DEFAULT 'ACTIVE',
    order_provisioning_state TEXT NOT NULL DEFAULT 'CREATING_ORDER',
    order_failure_reason TEXT,
    order_no TEXT,
    payment_deadline TIMESTAMPTZ,
    client_request_id UUID NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT buyer_quote_acceptances_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT buyer_quote_acceptances_version_check CHECK (seller_response_version >= 1),
    CONSTRAINT buyer_quote_acceptances_store_check CHECK (store_id > 0),
    CONSTRAINT buyer_quote_acceptances_state_check CHECK (state IN ('ACTIVE', 'ORDER_FAILED')),
    CONSTRAINT buyer_quote_acceptances_provisioning_check CHECK (order_provisioning_state IN ('CREATING_ORDER', 'ORDER_CREATED', 'ORDER_FAILED')),
    CONSTRAINT buyer_quote_acceptances_failed_check CHECK ((state = 'ORDER_FAILED') = (order_provisioning_state = 'ORDER_FAILED')),
    CONSTRAINT buyer_quote_acceptances_order_no_check CHECK ((order_provisioning_state = 'ORDER_CREATED') = (order_no IS NOT NULL)),
    CONSTRAINT buyer_quote_acceptances_amounts_check CHECK (goods_amount >= 0 AND shipping_amount >= 0 AND tax_amount >= 0),
    CONSTRAINT buyer_quote_acceptances_price_check CHECK (accepted_deal_price = goods_amount + shipping_amount + tax_amount)
);
CREATE UNIQUE INDEX buyer_quote_acceptances_idempotency ON procurement.buyer_quote_acceptances (buyer_id, client_request_id);
CREATE UNIQUE INDEX buyer_quote_acceptances_live ON procurement.buyer_quote_acceptances (request_id) WHERE state = 'ACTIVE';
CREATE UNIQUE INDEX buyer_quote_acceptances_order_no ON procurement.buyer_quote_acceptances (order_no) WHERE order_no IS NOT NULL;
CREATE INDEX buyer_quote_acceptances_response ON procurement.buyer_quote_acceptances (seller_response_id, seller_response_version);

-- run 帳本：沒有 controller_kind（AD12）、列版本（契約沒有 run_version）、domain_ref 字串多型（S1：typed 可空外鍵）。
-- 建 run 的方法（SubmitTurn、StartCandidateSearch、ResumeRun）的冪等鍵放在這裡（S4，舊 agent_run_commands 刪除）。
CREATE TABLE procurement.agent_runs (
    id UUID PRIMARY KEY,
    buyer_id BIGINT NOT NULL,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    conversation_id UUID REFERENCES procurement.conversations (id) ON DELETE CASCADE,
    parent_run_id UUID REFERENCES procurement.agent_runs (id) ON DELETE CASCADE,
    candidate_set_id UUID REFERENCES procurement.candidate_sets (id) ON DELETE CASCADE,
    publication_id UUID REFERENCES procurement.rfq_publications (id) ON DELETE SET NULL,
    run_type TEXT NOT NULL,
    status TEXT NOT NULL,
    failure_reason TEXT,
    resumed_into_run_id UUID,
    result_revision_id UUID REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    last_event_seq BIGINT NOT NULL DEFAULT 0,
    min_available_seq BIGINT NOT NULL DEFAULT 1,
    client_request_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT agent_runs_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT agent_runs_type_check CHECK (run_type IN ('TURN', 'CANDIDATE_SEARCH', 'CLARIFICATION_RESUME', 'PUBLICATION')),
    CONSTRAINT agent_runs_status_check CHECK (status IN ('QUEUED', 'RUNNING', 'WAITING_FOR_INPUT', 'CANCEL_REQUESTED', 'SUCCEEDED', 'FAILED', 'CANCELLED', 'RESUMED', 'EXPIRED')),
    CONSTRAINT agent_runs_failure_check CHECK (failure_reason IS NULL OR failure_reason IN ('WORKER_LOST', 'SEARCH_FAILED', 'TURN_PIPELINE_ERROR', 'EXPIRED')),
    CONSTRAINT agent_runs_seq_check CHECK (last_event_seq >= 0 AND min_available_seq >= 1)
);
CREATE UNIQUE INDEX agent_runs_idempotency ON procurement.agent_runs (buyer_id, client_request_id);
CREATE INDEX agent_runs_request ON procurement.agent_runs (request_id, created_at DESC);
CREATE INDEX agent_runs_unfinished ON procurement.agent_runs (status, created_at) WHERE status IN ('QUEUED', 'RUNNING');
CREATE INDEX agent_runs_event_expiry ON procurement.agent_runs (completed_at) WHERE completed_at IS NOT NULL AND min_available_seq <= last_event_seq;

-- run 的輸入（買家原文，具名文字欄）。沒有 retention_class（只有一個字面值、沒有讀取者）。
CREATE TABLE procurement.agent_run_inputs (
    run_id UUID PRIMARY KEY REFERENCES procurement.agent_runs (id) ON DELETE CASCADE,
    buyer_message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT agent_run_inputs_message_check CHECK (char_length(buyer_message) BETWEEN 1 AND 2000 AND buyer_message !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);

-- run 事件：(run_id, seq) 識別；payload 是 proto JSON（WatchRunResponse 的 oneof），形狀靠 proto 規則演進（S3），沒有 schema_version。run 結束 7 天後由週期 job 清除並把 min_available_seq 推到 last_event_seq + 1（A9-D20 R1）。
CREATE TABLE procurement.agent_run_events (
    run_id UUID NOT NULL REFERENCES procurement.agent_runs (id) ON DELETE CASCADE,
    seq BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (run_id, seq),
    CONSTRAINT agent_run_events_seq_check CHECK (seq >= 1),
    CONSTRAINT agent_run_events_type_check CHECK (char_length(event_type) >= 1),
    CONSTRAINT agent_run_events_payload_check CHECK (jsonb_typeof(payload) = 'object')
);

-- run 的產出：每個 (run_id, artifact_type) 一列就地更新（A9-D20 R3）；version 只當 ARTIFACT_UPSERTED 的去重序號。
CREATE TABLE procurement.agent_run_artifacts (
    run_id UUID NOT NULL REFERENCES procurement.agent_runs (id) ON DELETE CASCADE,
    artifact_type TEXT NOT NULL,
    version INT NOT NULL,
    status TEXT NOT NULL,
    data JSONB NOT NULL,
    checksum TEXT NOT NULL,
    as_of TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (run_id, artifact_type),
    CONSTRAINT agent_run_artifacts_version_check CHECK (version >= 1),
    CONSTRAINT agent_run_artifacts_data_check CHECK (jsonb_typeof(data) = 'object')
);

-- run 停下來等買家回答的中斷。interrupt_kind 只有 CLARIFICATION 一值；不設到期，下一輪送出即取代（A9-D20 R2）。
CREATE TABLE procurement.agent_run_interrupts (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES procurement.agent_runs (id) ON DELETE CASCADE,
    interrupt_kind TEXT NOT NULL,
    status TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    facts_as_of TIMESTAMPTZ,
    resolved_by_child_run_id UUID REFERENCES procurement.agent_runs (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    CONSTRAINT agent_run_interrupts_kind_check CHECK (interrupt_kind IN ('CLARIFICATION')),
    CONSTRAINT agent_run_interrupts_status_check CHECK (status IN ('PENDING', 'RESOLVED')),
    CONSTRAINT agent_run_interrupts_payload_check CHECK (jsonb_typeof(payload) = 'object')
);
CREATE INDEX agent_run_interrupts_run ON procurement.agent_run_interrupts (run_id);

-- 買家每期的送出次數：一列計數器加條件 UPDATE（A9-D9），沒有預約表。
CREATE TABLE procurement.usage (
    buyer_id BIGINT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    used INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, period_start),
    CONSTRAINT usage_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT usage_used_check CHECK (used >= 0)
);

-- ADK session.Service 的四張表（S7）：由 internal/platform/adksession 讀寫（schema 傳 procurement），形狀與 assistant 的同名表相同。
-- 鍵是 ADK 的字串；session ID 是 request ID 的字串，user_id 是買家 ID 的十進位字串。session 跨越掛起與回答兩個 run（graph 的 checkpoint）。
CREATE TABLE procurement.sessions (
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    id TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (app_name, user_id, id),
    CONSTRAINT sessions_key_check CHECK (app_name <> '' AND user_id <> '' AND id <> ''),
    CONSTRAINT sessions_state_check CHECK (jsonb_typeof(state) = 'object')
);

CREATE TABLE procurement.events (
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    seq BIGINT NOT NULL GENERATED ALWAYS AS IDENTITY,
    event JSONB NOT NULL,
    PRIMARY KEY (app_name, user_id, session_id, id),
    FOREIGN KEY (app_name, user_id, session_id) REFERENCES procurement.sessions (app_name, user_id, id) ON DELETE CASCADE,
    CONSTRAINT events_id_check CHECK (id <> ''),
    CONSTRAINT events_event_check CHECK (jsonb_typeof(event) = 'object')
);
CREATE INDEX events_session_time ON procurement.events (app_name, user_id, session_id, timestamp, seq);

CREATE TABLE procurement.app_states (
    app_name TEXT PRIMARY KEY,
    state JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT app_states_state_check CHECK (jsonb_typeof(state) = 'object')
);

CREATE TABLE procurement.user_states (
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (app_name, user_id),
    CONSTRAINT user_states_state_check CHECK (jsonb_typeof(state) = 'object')
);
