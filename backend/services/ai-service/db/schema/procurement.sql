-- Hawkr 幫找 Agent — procurement 域（MVP-A Slice 1）
-- 設計依據：docs/superpowers/specs/2026-08-07-procurement-assistant-mvp-design.md
-- 刻意不與既有 AI 表（sessions/events/ai_*）建外鍵：未來抽成獨立服務時整檔搬走。
-- 主鍵一律 TSID（BIGINT），時間一律 TIMESTAMPTZ（UTC）。
-- 擁有者欄位一律用 buyer_id（現任 buyer stint surrogate），而非登入 principal 的帳號 ID：
-- 見 pkg/identity/identity.go 的型別註解與 spec-gate 的 identity.semantic-naming 規則。

CREATE TABLE procurement_requests (
    id                 BIGINT      PRIMARY KEY,
    buyer_id           BIGINT      NOT NULL,
    status             TEXT        NOT NULL DEFAULT 'CLARIFYING',
    active_revision_id BIGINT,
    version            INT         NOT NULL DEFAULT 1,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_procurement_requests_buyer
    ON procurement_requests (buyer_id, created_at DESC);

-- append-only；同一 request 同時只有一個 ACTIVE revision（rewind 建新 branch 時
-- 舊分支整條標 SUPERSEDED）。
CREATE TABLE procurement_request_revisions (
    id                 BIGINT      PRIMARY KEY,
    request_id         BIGINT      NOT NULL,
    revision_no        INT         NOT NULL,
    parent_revision_id BIGINT,
    branch_id          BIGINT      NOT NULL,
    slots_json         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    slots_hash         TEXT        NOT NULL,
    status             TEXT        NOT NULL DEFAULT 'ACTIVE',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_procurement_request_revisions_no
    ON procurement_request_revisions (request_id, revision_no);

CREATE UNIQUE INDEX uq_procurement_request_revisions_active
    ON procurement_request_revisions (request_id)
    WHERE status = 'ACTIVE';

CREATE INDEX idx_procurement_request_revisions_branch
    ON procurement_request_revisions (request_id, branch_id, revision_no DESC);

-- 每次「需求被改動」的稽核紀錄與冪等鍵。
-- source：LLM_EXTRACT（模型從自然語言抽出）/ ANSWER（買家答題）/ REWIND（回上一題）
CREATE TABLE procurement_answer_patches (
    id                 BIGINT      PRIMARY KEY,
    request_id         BIGINT      NOT NULL,
    buyer_id           BIGINT      NOT NULL,
    base_revision_id   BIGINT,
    result_revision_id BIGINT      NOT NULL,
    source             TEXT        NOT NULL,
    payload_json       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    client_request_id  TEXT        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 冪等：同一買家同一 client_request_id 只會產生一次副作用。
CREATE UNIQUE INDEX uq_procurement_answer_patches_idempotency
    ON procurement_answer_patches (buyer_id, client_request_id);

CREATE INDEX idx_procurement_answer_patches_request
    ON procurement_answer_patches (request_id, created_at DESC);

-- Slice 1 只建表、程式尚未使用；Slice 2 的問答卡與 Slice 4 的
-- /account/messages/assistant/{conversationId} 路由才會寫入。
-- 現在一起建是為了讓 procurement schema 一次成形、避免第二次 migration 動到同一批表。
-- adk_session_id 只是可丟棄的模型上下文，不是交易 SSOT（真實狀態在 revision）。
CREATE TABLE procurement_conversations (
    id             BIGINT      PRIMARY KEY,
    buyer_id       BIGINT      NOT NULL,
    request_id     BIGINT      NOT NULL,
    adk_session_id TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_procurement_conversations_buyer
    ON procurement_conversations (buyer_id, created_at DESC);

CREATE UNIQUE INDEX uq_procurement_conversations_request
    ON procurement_conversations (request_id);

ALTER TABLE procurement_conversations
    ADD COLUMN title                TEXT        NOT NULL DEFAULT '',
    ADD COLUMN title_source         TEXT        NOT NULL DEFAULT 'AUTO',
    ADD COLUMN last_message_preview TEXT        NOT NULL DEFAULT '',
    ADD COLUMN deleted_at           TIMESTAMPTZ;

-- 列表分頁用：排序鍵改成 (updated_at, id)，跟 ListProcurementConversations 的
-- ORDER BY updated_at DESC, id DESC 對齊，讓查詢能吃到 index order 不必額外
-- in-memory sort（原本只有 id DESC，跟現在的排序鍵不匹配）。只看未刪除的。
-- 與既有 idx_procurement_conversations_buyer (buyer_id, created_at DESC) 並存，
-- 不改動既有索引。
CREATE INDEX idx_procurement_conversations_buyer_active
    ON procurement_conversations (buyer_id, updated_at DESC, id DESC)
    WHERE deleted_at IS NULL;

-- PATCH／DELETE 共用的冪等表，跟 procurement_answer_patches 同一套
-- 「先查再寫、DB unique index 當最終防線」模式。result_json 依操作種類存
-- {"op":"rename","title":"..."} 或 {"op":"delete"}。
CREATE TABLE procurement_conversation_commands (
    id                 BIGINT      PRIMARY KEY,
    buyer_id           BIGINT      NOT NULL,
    client_request_id  TEXT        NOT NULL,
    conversation_id    BIGINT      NOT NULL,
    result_json        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_procurement_conversation_commands
    ON procurement_conversation_commands (buyer_id, client_request_id);

-- A3：候選搜尋 run 的容器與終態（見 candidate-matching-a3 ADR D2：PLATFORM／EXTERNAL 兩條 lane 都做）。
-- 這張表同時扮演「run」的生命週期紀錄（QUEUED/RUNNING/.../lease/fencing）與候選結果的容器——
-- 理由見計畫 docs/superpowers/plans/2026-08-08-procurement-candidate-matching-a3.md Design Decisions #1。
--
-- ⚠️ 上面那個「一張表兩個角色」的決定本身沒有被推翻，但它原本連**識別碼**也共用（對外
-- runId 與 candidateSetId 回同一個 id），那一半已於 #2446 契約凍結前撤銷：詳細設計書 57
-- §1.2-3「runId 不得同時代表 candidateSetId、requestId 或 Temporal Workflow ID」。
-- 現在 run 身分是獨立欄位 run_id，見它自己的註解。
CREATE TABLE procurement_candidate_sets (
    id                    BIGINT      PRIMARY KEY,
    -- run 身分。與 id（＝candidate set 身分）**刻意是不同的值**：對外契約上 runId 是
    -- 「這次執行」、candidateSetId 是「這批候選」，設計書 57 §1.2-3 禁止一個 ID 同時代表
    -- 兩者。凍結 golden fixtures 前必須先分開，否則相等會變成消費端可以依賴的契約。
    --
    -- NULL 只出現在這支 migration 之前建立的舊列（expand 階段）；新列一律由 Go 端的 TSID
    -- 產生器指派，與 id 各配一次號。讀取端對 NULL 列退回舊行為（runId 回 id），contract
    -- 階段連同 GetProcurementCandidateSetByRunID 的 legacy 分支一起收掉，那時改 NOT NULL。
    --
    -- 本欄是**當前那一次嘗試**的 run 身分，不是「這批候選唯一的 run」：一次重試
    -- （RetryProcurementCandidateSet）會就地換上一個新配號的 run_id，被取代的舊值只留在
    -- procurement_agent_runs（由 domain_ref 指回同一個 candidate set）。`candidate_sets :
    -- agent_runs` 因此是 **1:N**——doc 57 §6.2 預告的 additive 基數變化，已落地。
    --
    -- 為什麼不能沿用同一個 run_id 重新開機：procurement_agent_runs 的終態守衛是「現在還
    -- 不是終態」，那一列在第一次嘗試就已經終態化，第二次嘗試的終態於是被整段丟掉——那個
    -- 執行永遠沒有終態 ledger event，串流端沒有收線訊號。
    --
    -- 唯一索引仍然成立：一個 run_id 至多被一列 candidate_set 持有（歷史嘗試不留在本欄）。
    run_id                BIGINT,
    request_id            BIGINT      NOT NULL,
    request_revision_id   BIGINT      NOT NULL,
    buyer_id              BIGINT      NOT NULL,
    policy_version        INT         NOT NULL DEFAULT 1,
    -- 值域 SSOT 是 docs/代碼管理（CodeMeta）/業務狀態碼.md 的 ProcurementRunStatus／
    -- ProcurementLaneStatus（issue #2446 契約凍結）。改名過渡期已結束：舊值
    -- COMPLETED／PARTIAL／OK 已由 backfill migration 收斂，且沒有任何寫入端會再產生它們。
    --
    -- CHECK 列的是 **registry 的完整值域**，不是「目前這版 code 實際會寫的子集」
    -- （run 實際只寫 QUEUED/RUNNING/SUCCEEDED/FAILED、lane 只寫 PENDING/SUCCEEDED/FAILED）。
    -- 收窄到子集的話，之後任何一次讓 code 開始寫既有 registry 值的改動，都會在沒有 schema
    -- 變更的情況下 23514——那是一個「明明照著 SSOT 寫卻被擋」的失敗，很難查。
    -- 反過來說，registry **新增**值時這裡必須同步加，否則同樣 23514；registry 的修改流程
    -- 是先改 docs/代碼管理（CodeMeta）/業務狀態碼.md 再跑 generate.sh，那時請一併看這裡。
    --
    -- CHECK 寫成表層具名 constraint（同 seller-service 的 chk_sellers_status 等）：
    -- 具名的話，日後值域變更 Atlas 會生出同名 constraint 的 DROP+ADD，diff 讀得懂；
    -- 匿名的欄位層 CHECK 由 Postgres 自動命名，改動時 diff 只會出現一串亂碼名字。
    status                TEXT        NOT NULL DEFAULT 'QUEUED',
    platform_lane_status  TEXT        NOT NULL DEFAULT 'PENDING',
    external_lane_status  TEXT        NOT NULL DEFAULT 'PENDING',
    -- failure_reason 承載 run 為何進入 FAILED 的機器可讀原因（ProcurementRunFailureReason），
    -- 不是本 repo「錯誤碼」那套 HTTP／gRPC 數字碼。
    failure_reason        TEXT        NOT NULL DEFAULT '',
    lease_owner           TEXT        NOT NULL DEFAULT '',
    lease_expires_at      TIMESTAMPTZ,
    fencing_token         BIGINT      NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT "chk_pcs_status" CHECK (status IN (
        'QUEUED', 'RUNNING', 'WAITING_FOR_INPUT', 'CANCEL_REQUESTED',
        'SUCCEEDED', 'FAILED', 'CANCELLED', 'RESUMED', 'EXPIRED')),
    CONSTRAINT "chk_pcs_platform_lane_status" CHECK (platform_lane_status IN (
        'PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'SKIPPED', 'CANCELLED')),
    CONSTRAINT "chk_pcs_external_lane_status" CHECK (external_lane_status IN (
        'PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'SKIPPED', 'CANCELLED')),
    -- 空字串＝未失敗，是 DEFAULT 也是 Retry 寫回的值，必須放行。
    -- ⚠️ Postgres 對 UPDATE 會重驗整列的所有 CHECK，所以漏了 '' 的話，連只改 status 的
    -- MarkProcurementCandidateSetRunning 都會 23514——CHECK 會打到根本沒引用該欄位的查詢。
    -- 同 status 那組：列的是 ProcurementRunFailureReason **registry 的完整值域**，
    -- 不是 candidate search 這條路徑實際會寫的子集。TURN_PIPELINE_ERROR 只由
    -- procurement_agent_runs 那張表的 TURN pipeline 寫入（doc 57 Slice F），這張表
    -- 目前不會出現它——仍然列進來，理由與 status 那段註解完全相同：registry 新增值
    -- 時同步加，才不會在某次「讓 code 開始寫某個既有 registry 值」的改動上撞到
    -- 一個沒有 schema 變更提示的 23514。
    CONSTRAINT "chk_pcs_failure_reason" CHECK (
        failure_reason = '' OR failure_reason IN ('WORKER_LOST', 'LANE_ALL_FAILED', 'TURN_PIPELINE_ERROR'))
);

-- 冪等鍵：同一需求版本、同一比對策略版本只會有一個 candidate_set（doc 55 §8.4）。
CREATE UNIQUE INDEX uq_procurement_candidate_sets_idempotency
    ON procurement_candidate_sets (request_revision_id, policy_version);

-- run 身分查詢鍵。partial index：expand 階段的舊列 run_id 為 NULL，普通 UNIQUE 在
-- Postgres 雖然容許多個 NULL，但寫成 partial 讓「NULL 是刻意排除的舊列」這件事在
-- schema 上看得出來，contract 階段拿掉 WHERE 子句即可。
CREATE UNIQUE INDEX uq_procurement_candidate_sets_run_id
    ON procurement_candidate_sets (run_id)
    WHERE run_id IS NOT NULL;

CREATE INDEX idx_procurement_candidate_sets_request
    ON procurement_candidate_sets (request_id, created_at DESC);

-- reaper 掃描用：只需要看還沒結束、且 lease 已過期的列。
CREATE INDEX idx_procurement_candidate_sets_lease
    ON procurement_candidate_sets (status, lease_expires_at)
    WHERE status IN ('QUEUED', 'RUNNING');

-- 一個 candidate_set 底下的個別候選；PLATFORM 來自 product-service，EXTERNAL 來自
-- seller-service 的 seller_prospects（經 gRPC SearchSellerProspects 查詢，見
-- internal/client/seller/adapter.go 與 internal/usecase/candidate_search_usecase.go）。
-- candidate_leads（本 DB 內）是遷移前的歷史來源，遷移完成後不再被讀取——見
-- schema.sql 該表上方的註解與 scripts/migrate_candidate_leads_to_prospects.go。
-- 刻意不存 public_email/public_phone：候選聯絡方式不得進入本模組任何一層（doc 55 §8.1）。
--
-- store_id 以下四欄是 issue #2377 新增的**賣家身分快照**（facts_as_of 當下的事實），
-- 讓網頁把候選呈現成「找到哪幾家商家」，不必逐候選反查 product-service。
--
-- 刻意不存商品層的圖片與標價：候選的單位是**收件人**不是品項（#2377 是 #2450
-- DIRECT_RECIPIENTS 的前置），而且 EXTERNAL lane 的候選來自站外名單、根本沒有平台商品。
-- 價格更不能放這裡——doc 57 §1.1-2 只認四種價格事實，商品標價不在其中；買家該看到的是
-- Pricebook 對該品項的參考價區間（referencePriceRange，屬 #2403）。
--
-- ⛔ store_id **不是授權判準**。「這個商品現在屬於哪個賣家」必須在需要授權的當下重新
--    問 product-service（見 usecase/quote_confirm.go 的 ConfirmByID 與
--    client/product.GetProductStoreID）。商品可以移店，快照會過期；拿這一欄去省掉那次
--    查詢，等於把擁有權檢查退化成讀舊資料。
--
-- 四欄全部 nullable／帶 DEFAULT：EXTERNAL lane 沒有平台賣家（來源是 seller-service
-- 的 seller_prospects），一律留空而不是偽造平台 seller identity（#2377 驗收條件 3）；
-- 同時滿足 expand-contract 對加欄位的要求（見 docs/開發規範/DB Migration 相容性規範
-- （expand-contract）.md §二）。
CREATE TABLE procurement_candidates (
    id                        BIGINT      PRIMARY KEY,
    candidate_set_id          BIGINT      NOT NULL,
    lane                      TEXT        NOT NULL, -- PLATFORM|EXTERNAL
    source_id                 TEXT        NOT NULL, -- product_id 或 seller_prospects.id（遷移後），皆存字串避免型別分歧；typed 約束掛在下面的 prospect_id
    display_name              TEXT        NOT NULL,
    score                     INT         NOT NULL DEFAULT 0,
    matched_facts_json        JSONB       NOT NULL DEFAULT '[]'::jsonb,
    unverifiable_facts_json   JSONB       NOT NULL DEFAULT '[]'::jsonb,
    facts_as_of               TIMESTAMPTZ NOT NULL,
    rank                      INT         NOT NULL DEFAULT 0,
    -- 商品當下擁有者的 store-stint surrogate（sellers.store_id）。DB／proto／對外 JSON
    -- 三層一律叫 store_id／storeId，不叫 seller_id：它是 store 代理鍵、不是登入 principal，
    -- 而 ai.proto 既有的 6 個 store_id 欄位就是上一輪「account_id 命名債正名」的結果
    -- （見檔頭 identity.semantic-naming 那段）。EXTERNAL lane 為 NULL。
    store_id                  BIGINT,
    seller_name               TEXT        NOT NULL DEFAULT '',
    logo_key                  TEXT        NOT NULL DEFAULT '', -- 商家 logo objectKey（49 §6.6：Gateway 組 URL）
    store_code                TEXT        NOT NULL DEFAULT '', -- 賣家頁 canonical 短碼；空字串時前端不得退回 store_id
    -- EXTERNAL lane 的業者身分（seller-service.seller_prospects.id）。
    -- 與 store_id 互斥：PLATFORM 填 store_id、EXTERNAL 填 prospect_id。
    -- source_id 維持字串以相容既有讀取端；約束掛在這個 typed 欄位上。
    prospect_id               BIGINT,
    -- EXTERNAL lane 的品牌素材快照（seller_prospects.{logo,cover}_key）。
    -- 與上面 logo_key 分開一組，理由同 ai.proto 14–15 的註解：logo_key 的語意是
    -- 「平台商家的 logo」，站外業者填進去就是偽造平台身分。本欄不表示平台已驗證。
    brand_logo_key            TEXT        NOT NULL DEFAULT '',
    brand_cover_key           TEXT        NOT NULL DEFAULT '',
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_procurement_candidates_set
    ON procurement_candidates (candidate_set_id, lane, rank);

-- 一家一筆。應用層在各 lane 內已經去重（PLATFORM 依 store 選代表、EXTERNAL
-- 靠上游一列即一業者），這兩條是防線不是機制——機制失效時擋下，而不是靠它
-- 來實現去重。補上 doc 57 §563 記載的缺口。
CREATE UNIQUE INDEX uq_procurement_candidates_set_store
    ON procurement_candidates (candidate_set_id, store_id) WHERE store_id IS NOT NULL;
CREATE UNIQUE INDEX uq_procurement_candidates_set_prospect
    ON procurement_candidates (candidate_set_id, prospect_id) WHERE prospect_id IS NOT NULL;

-- 買家對候選的勾選狀態。UNIQUE 依 doc 55 §8.4：同一需求版本、同一 lane、同一候選只有一列，
-- 重複勾選（同 selected 值）為 no-op，取消勾選（selected=false）是實際的狀態轉換。
CREATE TABLE procurement_candidate_selections (
    id                   BIGINT      PRIMARY KEY,
    request_id           BIGINT      NOT NULL,
    request_revision_id  BIGINT      NOT NULL,
    buyer_id             BIGINT      NOT NULL,
    candidate_set_id     BIGINT      NOT NULL,
    lane                 TEXT        NOT NULL,
    candidate_id         BIGINT      NOT NULL,
    selected             BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_procurement_candidate_selections
    ON procurement_candidate_selections (request_revision_id, lane, candidate_id);

CREATE INDEX idx_procurement_candidate_selections_request
    ON procurement_candidate_selections (request_id, created_at DESC);

-- 品項解析的**結果**住 procurement_request_revisions.slots_json 的 item_resolution
-- （ADR 2026-09-04-crop-resolution-into-slots，刀 1）：解析結果與 item 原話同生命
-- 週期，同一個 revision 內一起寫、一起回退。
--
-- 「現在正在問什麼」（待消歧候選清單與待處理原因）曾經也在這張表上，刀 2 起搬進
-- procurement_agent_run_interrupts.payload_json——它描述的是一個中斷的內容，生命
-- 週期就是那個中斷的生命週期。理由見該表的欄位註解。
--
-- 候選聯絡方式不進本表任何欄位（doc 55 §8.1）——email 只在 outbox worker 寄信當下
-- 經 gRPC 向 seller-service 動態查 seller_prospects 一次（不再是本 DB 內的
-- candidate_leads；遷移完成後該表不再是 EXTERNAL 候選的資料源）。寄信本體屬
-- T17 尚待實作，這裡先記錄查詢對象已經改變。

-- 一次「報價前試算」的持久化結果（Slice 2c）。與 procurement_answer_patches／
-- procurement_conversation_commands 同一套冪等慣例：client_request_id 唯一鍵是
-- 「有沒有處理過這個請求」唯一仲裁者，不用 application-level payload hash 比對。
--
-- reference_facts_hash／reference_payload_json／unavailable_reason 彼此依
-- reference_availability 的值互斥：AVAILABLE 時前兩者有值、unavailable_reason 為
-- NULL；UNAVAILABLE 時相反——由寫入端（Task 6 usecase）保證，這裡不加 CHECK
-- constraint 表達（同一組互斥欄位在 procurement_requests 的品項解析三態也是
-- 應用層保證，見該表註解）。
--
-- market_price_basis_json／quote_total_range_json（v6 設計稿 03 市場價格參考
-- 區塊）是兩個純附加的 nullable 欄位。它們必須持久化、不能在讀取時重算：
-- FindPricePreviewByIdempotencyKey 的冪等回放路徑不會再打 Pricebook、也不會
-- 再讀 slots，重算不出來；沒有它們的話同一個 client_request_id 重送就會拿到
-- 一份少了「資料截至／計價基準／詢價總額區間」的殘缺結果。
--
-- 兩者各自為 NULL 的條件不同，刻意不合併成一欄：
--   market_price_basis_json 與 reference_payload_json 同生同滅（只有
--     PRICEBOOK_GUIDED 且 mapping 可靠時才有市場資料可揭露）；
--   quote_total_range_json 兩個 pricing_mode 都可能有值，但需求數量是選填
--     slot，沒填數量就算不出總額，這時為 NULL。
CREATE TABLE procurement_price_previews (
    id                         BIGINT      PRIMARY KEY,
    buyer_id                   BIGINT      NOT NULL,
    request_id                 BIGINT      NOT NULL,
    request_revision_id        BIGINT      NOT NULL,
    pricing_mode               TEXT        NOT NULL,
    quality_knowledge_state    TEXT        NOT NULL,
    reference_availability     TEXT        NOT NULL,
    reference_facts_hash       TEXT,
    reference_payload_json     JSONB,
    unavailable_reason         TEXT,
    buyer_requested_range_json JSONB       NOT NULL,
    deal_temperature_json      JSONB       NOT NULL,
    market_price_basis_json    JSONB,
    quote_total_range_json     JSONB,
    client_request_id          TEXT        NOT NULL,
    preview_input_hash         TEXT        NOT NULL,
    payload_hash               TEXT        NOT NULL,
    version                    BIGINT      NOT NULL DEFAULT 1,
    expires_at                 TIMESTAMPTZ NOT NULL,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_procurement_price_previews_pricing_mode
        CHECK (pricing_mode IN ('PRICEBOOK_GUIDED','BUYER_ENTERED_NO_REFERENCE')),
    CONSTRAINT chk_procurement_price_previews_reference_availability
        CHECK (reference_availability IN ('AVAILABLE','UNAVAILABLE'))
);

CREATE UNIQUE INDEX uq_procurement_price_previews_idempotency
    ON procurement_price_previews (buyer_id, client_request_id);

-- CountPricePreviewsByRequestID（db/queries/procurement/price_preview.sql）撐起
-- HasPricePreview／NextAction 提示，在 request 進入 REQUIREMENT_READY 之後，
-- 每一次 /turns、/answers、/rewinds 都會跑一次這條查詢——沒有這個索引就是全表
-- sequential scan。純加值、expand-only，不需要資料搬遷（Task 18／Fix 6）。
CREATE INDEX idx_procurement_price_previews_request
    ON procurement_price_previews (request_id);

-- 買家對某次 price preview 的確認快照（Slice 3a）。price_preview_id／
-- price_preview_version 記錄「確認的是哪一版試算」——試算表目前只 INSERT、沒有
-- 更新版本的路徑（見 procurement_price_previews 表註解），但這裡提前存 version
-- 欄位，避免未來試算表真的開始覆寫版本時，既有快照的「當初確認的是哪一版」
-- 這件事變成不可回溯。canonical_payload_json 是確認當下的完整定案內容快照
-- （報價單建立與後續顯示的唯一依據），與 price_preview 的
-- reference_payload_json／buyer_requested_range_json 等試算期間的中間結果分開存。
--
-- 冪等慣例與 procurement_price_previews 完全一致：client_request_id 唯一鍵是
-- 「有沒有處理過這次確認」的唯一仲裁者。
CREATE TABLE procurement_confirmed_rfq_snapshots (
    id                      BIGINT      PRIMARY KEY,
    buyer_id                BIGINT      NOT NULL,
    request_id              BIGINT      NOT NULL,
    request_revision_id     BIGINT      NOT NULL,
    price_preview_id        BIGINT      NOT NULL,
    price_preview_version   BIGINT      NOT NULL,
    quality_knowledge_state TEXT        NOT NULL,
    pricing_mode            TEXT        NOT NULL,
    canonical_payload_json  JSONB       NOT NULL,
    payload_hash            TEXT        NOT NULL,
    client_request_id       TEXT        NOT NULL,
    expires_at              TIMESTAMPTZ NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_procurement_confirmed_rfq_snapshots_idempotency
    ON procurement_confirmed_rfq_snapshots (buyer_id, client_request_id);

CREATE INDEX idx_procurement_confirmed_rfq_snapshots_request
    ON procurement_confirmed_rfq_snapshots (request_id, buyer_id, created_at DESC);

-- publication approval（#2450 Scope 3）：買家「我要把這張詢價發出去」的不可變意圖。
-- approval 與 execute 分離是刻意的（doc 64 §4.1）——doc 57 §1.1-1 的「不得在無人
-- 確認下觸發對外通知」需要一個可稽核的「人確認過」的物件，合成一支 API 就沒有它。
--
-- 內容不可變、狀態可變。version 是**狀態轉移的 CAS token**（同
-- procurement_agent_runs.version 的模型），不是 data schema 版本——後者是
-- schema_version 欄。execute 帶 (id, version) 來 CAS，兩者都由 preview 的回應吐給
-- 呼叫端；缺任一個就會重演 runs/{runId}/cancel 那個「必填 CAS 欄位在 wire 上無來源」
-- 的坑。
--
-- 核心的 version／hash／state／audience 全部是實欄位，不藏進 JSON（#2450 Scope 8）。
-- seller_facing_payload_json 是唯一的 JSONB，它是編譯結果的快照本身。
CREATE TABLE procurement_rfq_publication_approvals (
    id                           BIGINT      PRIMARY KEY,
    version                      INT         NOT NULL DEFAULT 1,
    buyer_id                     BIGINT      NOT NULL,
    request_id                   BIGINT      NOT NULL,
    request_revision_id          BIGINT      NOT NULL,
    -- issue Scope 3 要「confirmedRfqSnapshotId／version／hash」三元組，但
    -- procurement_confirmed_rfq_snapshots **沒有 version 欄**（它是不可變的，
    -- 一個冪等鍵一列，重新確認會產生新的 id 而不是新版本）。id ＋ hash 已完整
    -- 決定內容，不為了對齊票文憑空加一個恆為 1 的欄位。
    confirmed_rfq_snapshot_id    BIGINT      NOT NULL,
    confirmed_rfq_payload_hash   TEXT        NOT NULL,
    price_preview_id             BIGINT      NOT NULL,
    price_preview_version        BIGINT      NOT NULL,
    -- 定價事實的兩個真實 hash。issue Scope 3 另外要求的 pricingFacts／policy／
    -- unit-conversion 三個 hash 在生產路徑沒有來源，刻意不建欄位——綁一個恆為
    -- NULL 的值等於假裝有防線。reference_facts_hash 在 procurement_price_previews
    -- 是宣告了但生產路徑從未填過的欄位（usecase/price_preview.go 建
    -- InsertPricePreviewInput 時沒帶它），因此不引用。
    price_preview_payload_hash   TEXT        NOT NULL,
    price_preview_input_hash     TEXT        NOT NULL,
    quality_knowledge_state      TEXT        NOT NULL,
    pricing_mode                 TEXT        NOT NULL,
    -- reference/no-reference truth（#2450 Scope 3、Scope 5 的 hard block 判準）。
    -- 值域同 procurement_price_previews.reference_availability。
    -- ⛔ PRICE_BASIS_UNAVAILABLE 在 execute 必須 hard block；legacy 的 UNAVAILABLE
    -- 不得自動視為 BUYER_ENTERED_NO_REFERENCE 人工模式。
    reference_availability       TEXT        NOT NULL,
    buyer_requested_range_hash   TEXT        NOT NULL,
    -- 報價截止：publication 自己的屬性，不在 confirmed snapshot 裡（doc 64 §3.1）。
    -- 它進 seller_facing_payload，所以已被 seller_facing_payload_hash 涵蓋。
    response_deadline            DATE,
    audience_mode                TEXT        NOT NULL,
    audience_policy_id           TEXT        NOT NULL,
    audience_policy_version      INT         NOT NULL,
    audience_hash                TEXT        NOT NULL,
    -- 只有 DIRECT_RECIPIENTS 用；WISH_POOL 恆為 NULL（空 recipient list 不得冒充
    -- Wish Pool，所以用 NULL 而不是 0 或空陣列來表達「本模式沒有這個概念」）。
    candidate_set_id             BIGINT,
    candidate_set_policy_version INT,
    seller_facing_payload_json   JSONB       NOT NULL,
    seller_facing_payload_hash   TEXT        NOT NULL,
    -- #2537 完成前恆為 UNKNOWN（doc 64 §2-5）；不得由 client 用 candidate score 補值。
    deal_temperature_status      TEXT        NOT NULL DEFAULT 'UNKNOWN',
    state                        TEXT        NOT NULL,
    schema_version               TEXT        NOT NULL,
    preview_client_request_id    TEXT        NOT NULL,
    generated_at                 TIMESTAMPTZ NOT NULL,
    expires_at                   TIMESTAMPTZ NOT NULL,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_prpa_direct_requires_candidate_set CHECK (
        (audience_mode <> 'DIRECT_RECIPIENTS')
        OR (candidate_set_id IS NOT NULL AND candidate_set_policy_version IS NOT NULL)
    ),
    CONSTRAINT chk_prpa_wish_pool_has_no_candidate_set CHECK (
        (audience_mode <> 'WISH_POOL')
        OR (candidate_set_id IS NULL AND candidate_set_policy_version IS NULL)
    ),
    CONSTRAINT chk_prpa_reference_availability
        CHECK (reference_availability IN ('AVAILABLE','UNAVAILABLE'))
);

CREATE UNIQUE INDEX uq_procurement_rfq_publication_approvals_idempotency
    ON procurement_rfq_publication_approvals (buyer_id, preview_client_request_id);

CREATE INDEX idx_procurement_rfq_publication_approvals_request
    ON procurement_rfq_publication_approvals (request_id, buyer_id, created_at DESC);

-- approval 的**有序**收件人名單（#2450 Scope 2／Scope 5）。
--
-- ⛔ 存在的理由：approval 只存 audience_hash，而 hash 不可逆——execute 要重驗
-- 「ordered IDs／seller identities」就必須有名單本身，Slice 2b 要建 invite 更需要。
-- Slice 1 漏了這張表，補在這裡。
--
-- ⛔ 不得改從 procurement_candidate_selections 反推：那張表是 upsert、selected 可
-- 反覆切換，買家核准後還能改；從可變來源反推「當初核准的是誰」正是 approval 這個
-- 不可變物件要防的事（doc 64 §4.1）。
--
-- position 是契約的一部分：換順序就是換受眾，audienceHash 依序計算。
-- 兩條 lane 的身分住不同欄位（PLATFORM→store_id、EXTERNAL→prospect_id），互斥，
-- 理由見 domain.PublicationRecipient 與 ADR 2026-09-08-publication-recipient-identity-in-audience-preimage。
CREATE TABLE procurement_rfq_publication_approval_recipients (
    approval_id BIGINT NOT NULL,
    position    INT    NOT NULL,
    lane        TEXT   NOT NULL,
    store_id    BIGINT,
    prospect_id BIGINT,
    PRIMARY KEY (approval_id, position),
    CONSTRAINT chk_prpar_identity_matches_lane CHECK (
        (lane = 'PLATFORM'  AND store_id IS NOT NULL AND prospect_id IS NULL)
     OR (lane = 'EXTERNAL'  AND prospect_id IS NOT NULL AND store_id IS NULL)
    )
);

-- execute 的冪等 claim（#2450 Scope 6 步驟 1、Scope 7）。
-- ⛔ 與 approval 的 preview_client_request_id 是**兩把不同的鍵**：Scope 7 明文
-- 「preview、confirmation、publication preview、execute 各用不同 stable key；
-- 不可跨 phase 重用」。混用會讓「重送 preview」與「重送 execute」互相冒充。
--
-- canonical_request_hash 用來分辨「同 key 同內容」（回放原結果）與「同 key 不同
-- 內容」（409，零 mutation）。只存 hash 不存原文：原文是 approval 的內容，
-- 已經在 approval 列上，重存一份就是第二個 SSOT。
CREATE TABLE procurement_rfq_publication_commands (
    id                     BIGINT      PRIMARY KEY,
    buyer_id               BIGINT      NOT NULL,
    request_id             BIGINT      NOT NULL,
    approval_id            BIGINT      NOT NULL,
    approval_version       INT         NOT NULL,
    client_request_id      TEXT        NOT NULL,
    canonical_request_hash TEXT        NOT NULL,
    -- 回放用：成功後指回建出來的 publication；失敗或仍在執行時為 NULL。
    publication_id         BIGINT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_procurement_rfq_publication_commands_idempotency
    ON procurement_rfq_publication_commands (buyer_id, client_request_id);

-- 一次 RFQ 發布（#2450 Scope 6 步驟 4）。
--
-- ⛔ state = PUBLISHED **只代表 server transaction 完成**，不代表 Wish Pool 已
-- 建索引、也不代表賣家已收到（Scope 9、doc 64 §4.2）。投遞與索引的真相由 #2539
-- 的 typed evidence 更新，不得由本表的 state 推導。
--
-- 本 slice（2a）刻意**不建** invite／projection／outbox：那是 2b／2c。所以一張
-- PUBLISHED 的 publication 在本 slice 落地後是「已建立、零對外投遞」的狀態，
-- 這與 Slice 1 的 preview 零 side effect 是同一種刻意邊界。
--
-- 核心的 version／hash／state／audience 全部是實欄位，不藏進 JSON（Scope 8）。
CREATE TABLE procurement_rfq_publications (
    id                         BIGINT      PRIMARY KEY,
    version                    INT         NOT NULL DEFAULT 1,
    buyer_id                   BIGINT      NOT NULL,
    request_id                 BIGINT      NOT NULL,
    request_revision_id        BIGINT      NOT NULL,
    -- 產生本次發布的核准。approval 的內容不可變，所以這裡只存參照＋當時的 version。
    approval_id                BIGINT      NOT NULL,
    approval_version           INT         NOT NULL,
    command_id                 BIGINT      NOT NULL,
    confirmed_rfq_snapshot_id  BIGINT      NOT NULL,
    -- 從 approval 複製下來的三個 CAS 對象，讓 publication 自己就能回答
    -- 「發出去的是什麼、發給誰」，不必回頭 join approval。
    seller_facing_payload_hash TEXT        NOT NULL,
    audience_mode              TEXT        NOT NULL,
    audience_hash              TEXT        NOT NULL,
    state                      TEXT        NOT NULL,
    schema_version             TEXT        NOT NULL,
    published_at               TIMESTAMPTZ NOT NULL,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 一個 command 只能產生一張 publication（Scope 6「exactly-one」的機械保證）。
CREATE UNIQUE INDEX uq_procurement_rfq_publications_command
    ON procurement_rfq_publications (command_id);

CREATE INDEX idx_procurement_rfq_publications_request
    ON procurement_rfq_publications (request_id, buyer_id, created_at DESC);

-- 一次 DIRECT 發布的收件人快照（#2450 Scope 6 步驟 6）。
--
-- ⛔ 它**就是**那份不可變的 recipient 快照，不另建 procurement_rfq_recipient_snapshots
-- （Scope 8 表清單有列，設計書刻意不建）。一個收件人在 publication 生命週期內只會有
-- 一張 invite、狀態在同一列流轉，此時獨立的 snapshot 表與本表是 1:1，等於把一張表垂直
-- 切兩半，並讓收件人身分同時存在三個地方（approval recipients／snapshot／invite）。
-- 日後真的長出「同一收件人多次投遞嘗試」時，拆法是把狀態欄搬進新表、身分欄留在原地。
--
-- ⛔ 刻意不含 confirm_token 與任何 confirm 語意：legacy procurement_quote_invites
-- （已隨 #2543 C3 退場）的「token confirm、first-wins、sibling LOST」與 V6 語意
-- 相反（選擇權在買家），doc 64 §5.1 明文 #2538 是新建鏈路不是既有表擴充。
-- 這段留著是因為它回答的是「為什麼這張表長這樣」，不是「那張表在哪裡」。
-- ⛔ 刻意不含任何價格欄：doc 64 §2-1，平台算的價不是賣家報價。
CREATE TABLE procurement_rfq_publication_invites (
    id                BIGINT      PRIMARY KEY,
    publication_id    BIGINT      NOT NULL,
    buyer_id          BIGINT      NOT NULL,
    -- 承 approval recipients 的 position。順序是 audienceHash preimage 的一部分，
    -- 不是顯示偏好。
    position          INT         NOT NULL,
    lane              TEXT        NOT NULL,
    store_id          BIGINT,
    prospect_id       BIGINT,
    -- 值域見 CodeMeta ProcurementRfqInviteDeliveryState。SENT 是「已交給投遞管道」，
    -- 不是已送達、更不是已看到（doc 64 §4.2）。
    delivery_state    TEXT        NOT NULL,
    delivery_state_at TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- 照抄 chk_prpar_identity_matches_lane：兩條 lane 的身分住不同欄位。EXTERNAL 若
    -- 塌成 store_id=0，兩個不同的站外業者會算出同一個身分。
    CONSTRAINT chk_prpi_identity_matches_lane CHECK (
        (lane = 'PLATFORM' AND store_id IS NOT NULL AND prospect_id IS NULL)
     OR (lane = 'EXTERNAL' AND prospect_id IS NOT NULL AND store_id IS NULL)
    ),
    CONSTRAINT chk_prpi_delivery_state CHECK (
        delivery_state IN ('NOT_SENT', 'QUEUED', 'SENT', 'FAILED')
    )
);

-- Scope 6「每 recipient 至多一 invite」的機械保證，兩個身分維度各一條。
-- 這是防線不是去重機制：真正的去重在上游（uq_procurement_candidates_set_store ＋
-- compileAudience 的 candidate ID 去重）。
CREATE UNIQUE INDEX uq_prpi_publication_store
    ON procurement_rfq_publication_invites (publication_id, store_id)
    WHERE store_id IS NOT NULL;
CREATE UNIQUE INDEX uq_prpi_publication_prospect
    ON procurement_rfq_publication_invites (publication_id, prospect_id)
    WHERE prospect_id IS NOT NULL;
CREATE UNIQUE INDEX uq_prpi_publication_position
    ON procurement_rfq_publication_invites (publication_id, position);

CREATE INDEX idx_prpi_publication
    ON procurement_rfq_publication_invites (publication_id);

-- DIRECT lane 的 typed notification outbox（#2450 Scope 6 步驟 6、Scope 8）。
--
-- ⛔ 新表而不是複用 procurement_quote_outbox：那張表與 quote_batches／quote_invites 同屬
-- legacy 三表，已隨 #2543 C3 一起 DROP（doc 64 §7）。當初不複用正是為了讓那一刀能
-- 直接 drop 而不是「先搬 V6 資料再刪」；且 doc 64 §7 的隔離判準是 legacy 與 V6 不得
-- 同時對同一張 RFQ 生效，混在同一張表時「哪些列屬於哪個入口」只能靠 event_type 推斷。
-- 這個決定的回報就是本刀：三張表無條件 DROP，零資料遷移。完整取捨見私人封存的 ADR
-- 2026-09-09-publication-notification-outbox-new-typed-table。
--
-- 欄位形狀對齊 pkg/outbox.Repository[T] 的五個方法；claim SQL 照
-- pkg/outbox/query_template.sql.md。
CREATE TABLE procurement_rfq_publication_outbox (
    id              BIGINT      PRIMARY KEY,
    invite_id       BIGINT      NOT NULL,
    event_type      TEXT        NOT NULL,
    payload         JSONB       NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending',
    attempt_count   INT         NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    CONSTRAINT chk_prpo_event_type CHECK (
        event_type IN ('RFQ_PUBLICATION_INVITE_CREATED', 'RFQ_PUBLICATION_INVITE_EXTERNAL_CREATED')
    ),
    CONSTRAINT chk_prpo_status CHECK (status IN ('pending', 'done', 'failed'))
);

-- #2450 驗收條件的 deterministic dedupe：業務鍵直接擋在 DB，不靠應用層。
CREATE UNIQUE INDEX uq_prpo_invite_event
    ON procurement_rfq_publication_outbox (invite_id, event_type);

CREATE INDEX idx_prpo_pending
    ON procurement_rfq_publication_outbox (status, next_attempt_at)
    WHERE status = 'pending';


CREATE TABLE procurement_seller_responses (
    id                  BIGINT      PRIMARY KEY,
    version             INT         NOT NULL DEFAULT 1,
    publication_id      BIGINT      NOT NULL,
    buyer_id            BIGINT      NOT NULL,
    request_id          BIGINT      NOT NULL,
    lane                TEXT        NOT NULL,
    store_id            BIGINT,
    prospect_id         BIGINT,
    latest_version      INT         NOT NULL DEFAULT 0,
    withdrawn_version   INT,
    withdrawn_at        TIMESTAMPTZ,
    write_closed_at     TIMESTAMPTZ,
    write_closed_reason TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_prsr_identity_matches_lane CHECK (
        (lane = 'PLATFORM' AND store_id IS NOT NULL AND prospect_id IS NULL)
     OR (lane = 'EXTERNAL' AND prospect_id IS NOT NULL AND store_id IS NULL)
    ),
    CONSTRAINT chk_prsr_withdrawn_pair CHECK (
        (withdrawn_version IS NULL) = (withdrawn_at IS NULL)
    ),
    CONSTRAINT chk_prsr_write_closed_pair CHECK (
        (write_closed_at IS NULL) = (write_closed_reason IS NULL)
    ),
    CONSTRAINT chk_prsr_write_closed_reason CHECK (
        write_closed_reason IS NULL
     OR write_closed_reason IN ('DECLINED', 'PUBLICATION_CLOSED', 'OTHER_WON')
    )
);

CREATE UNIQUE INDEX uq_prsr_publication_store
    ON procurement_seller_responses (publication_id, store_id)
    WHERE store_id IS NOT NULL;
CREATE UNIQUE INDEX uq_prsr_publication_prospect
    ON procurement_seller_responses (publication_id, prospect_id)
    WHERE prospect_id IS NOT NULL;

CREATE INDEX idx_prsr_request_with_submission
    ON procurement_seller_responses (request_id, buyer_id, created_at DESC)
    WHERE latest_version > 0;

CREATE TABLE procurement_seller_response_versions (
    id                               BIGINT        PRIMARY KEY,
    seller_response_id               BIGINT        NOT NULL,
    version                          INT           NOT NULL,
    response_type                    TEXT          NOT NULL,
    offered_quality_grade            TEXT          NOT NULL,
    offered_quality_inspection_notes TEXT          NOT NULL,
    unit_basis_quantity              NUMERIC(14,3) NOT NULL,
    unit_basis_unit                  TEXT          NOT NULL,
    offered_quantity                 NUMERIC(14,3) NOT NULL,
    offered_quantity_unit            TEXT          NOT NULL,
    conversion_basis                 TEXT          NOT NULL,
    currency                         TEXT          NOT NULL,
    goods_amount                     BIGINT        NOT NULL,
    shipping_amount                  BIGINT        NOT NULL,
    tax_amount                       BIGINT        NOT NULL,
    tax_inclusion_basis              TEXT          NOT NULL,
    total_amount                     BIGINT        NOT NULL,
    lead_time_days                   INT,
    promised_delivery_at             TIMESTAMPTZ,
    valid_until                      TIMESTAMPTZ   NOT NULL,
    terms_note                       TEXT          NOT NULL DEFAULT '',
    deviation_disclosure             TEXT          NOT NULL DEFAULT '',
    outside_buyer_requested_range    BOOLEAN       NOT NULL,
    payload_hash                     TEXT          NOT NULL,
    publication_version              INT           NOT NULL,
    seller_facing_payload_hash       TEXT          NOT NULL,
    client_request_id                TEXT          NOT NULL,
    submitted_at                     TIMESTAMPTZ   NOT NULL,
    -- 站外業者（EXTERNAL lane）送出時入口帶來的驗證紀錄；平台賣家為 NULL。
    verification_method              TEXT,
    verification_email               TEXT,
    verified_at                      TIMESTAMPTZ,
    verification_session_id          TEXT,
    created_at                       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT chk_prsrv_verification_all_or_none CHECK (
        (verification_method IS NULL AND verification_email IS NULL
         AND verified_at IS NULL AND verification_session_id IS NULL)
     OR (verification_method = 'GOOGLE' AND verification_email <> ''
         AND verified_at IS NOT NULL AND verification_session_id <> '')
    ),
    CONSTRAINT chk_prsrv_response_type CHECK (
        response_type IN ('MANUAL_QUOTE', 'ACCEPT_NON_PRICE_TERMS_WITH_QUOTE')
    ),
    CONSTRAINT chk_prsrv_tax_inclusion_basis CHECK (
        tax_inclusion_basis IN ('TAX_INCLUDED', 'TAX_EXCLUDED')
    ),
    CONSTRAINT chk_prsrv_amounts_non_negative CHECK (
        goods_amount >= 0 AND shipping_amount >= 0 AND tax_amount >= 0
    ),
    CONSTRAINT chk_prsrv_total_is_sum CHECK (
        total_amount = goods_amount + shipping_amount + tax_amount
    ),
    CONSTRAINT chk_prsrv_quantities_positive CHECK (
        unit_basis_quantity > 0 AND offered_quantity > 0
    ),
    CONSTRAINT chk_prsrv_version_positive CHECK (version >= 1)
);

CREATE UNIQUE INDEX uq_prsrv_response_version
    ON procurement_seller_response_versions (seller_response_id, version);

CREATE UNIQUE INDEX uq_prsrv_idempotency
    ON procurement_seller_response_versions (seller_response_id, client_request_id);

-- 撤回與婉拒的只追加指令紀錄（ADR 2026-10-03 vendor portal 修訂）：不 UPDATE、不 DELETE，
-- 與版本表一樣只靠沒有任何改寫它的查詢保證。撤回的事實來源是這張表，身分列的
-- withdrawn_version／withdrawn_at 在同一個 transaction 由它推出。
CREATE TABLE procurement_seller_response_commands (
    id                      BIGINT      PRIMARY KEY,
    seller_response_id      BIGINT      NOT NULL,
    kind                    TEXT        NOT NULL,
    withdrawn_version       INT,
    client_request_id       TEXT        NOT NULL,
    reason_code             TEXT        NOT NULL DEFAULT '',
    verification_method     TEXT,
    verification_email      TEXT,
    verified_at             TIMESTAMPTZ,
    verification_session_id TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_prsrc_kind CHECK (kind IN ('WITHDRAW', 'DECLINE')),
    CONSTRAINT chk_prsrc_withdrawn_version CHECK (
        (kind = 'WITHDRAW') = (withdrawn_version IS NOT NULL)
    ),
    CONSTRAINT chk_prsrc_withdrawn_version_positive CHECK (
        withdrawn_version IS NULL OR withdrawn_version >= 1
    ),
    CONSTRAINT chk_prsrc_verification_all_or_none CHECK (
        (verification_method IS NULL AND verification_email IS NULL
         AND verified_at IS NULL AND verification_session_id IS NULL)
     OR (verification_method = 'GOOGLE' AND verification_email <> ''
         AND verified_at IS NOT NULL AND verification_session_id <> '')
    )
);

CREATE UNIQUE INDEX uq_prsrc_idempotency
    ON procurement_seller_response_commands (seller_response_id, client_request_id);

CREATE TABLE procurement_seller_response_drafts (
    seller_response_id BIGINT      PRIMARY KEY,
    payload            JSONB       NOT NULL,
    version            INT         NOT NULL DEFAULT 1,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE procurement_buyer_shortlist_entries (
    id                     BIGINT      PRIMARY KEY,
    buyer_id               BIGINT      NOT NULL,
    request_id             BIGINT      NOT NULL,
    seller_response_id     BIGINT      NOT NULL,
    seller_response_version INT        NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_prbse_request_response
    ON procurement_buyer_shortlist_entries (request_id, seller_response_id);

CREATE INDEX idx_prbse_request
    ON procurement_buyer_shortlist_entries (request_id, created_at);

CREATE TABLE procurement_buyer_quote_acceptances (
    id                       BIGINT      PRIMARY KEY,
    version                  INT         NOT NULL DEFAULT 1,
    buyer_id                 BIGINT      NOT NULL,
    request_id               BIGINT      NOT NULL,
    publication_id           BIGINT      NOT NULL,
    seller_response_id       BIGINT      NOT NULL,
    seller_response_version  INT         NOT NULL,
    seller_response_payload_hash TEXT    NOT NULL,
    confirmed_rfq_snapshot_id    BIGINT  NOT NULL,
    confirmed_rfq_payload_hash   TEXT    NOT NULL,
    lane                     TEXT        NOT NULL,
    store_id                 BIGINT,
    prospect_id              BIGINT,
    accepted_deal_price      BIGINT      NOT NULL,
    currency                 TEXT        NOT NULL,
    goods_amount             BIGINT      NOT NULL,
    shipping_amount          BIGINT      NOT NULL,
    tax_amount               BIGINT      NOT NULL,
    state                    TEXT        NOT NULL,
    order_provisioning_state TEXT        NOT NULL,
    -- ORDER_FAILED 的成因（doc 64 §6.4.14）。沒有它就分不出「order-service 拒絕」與
    -- 「worker 自己出錯」——outbox 的 last_error 是另一張表的欄位，acceptance 讀不到。
    -- 值域走 CodeMeta（ProcurementOrderProvisioningFailureReason），⛔ 不新增第二套數字碼。
    -- ⚠️ CodeMeta 的 producer 目前被物流線的四組缺失值域卡著（碰 CodeMeta 的 commit 一律
    --    過不了 codemeta-copies gate），所以本欄先上、值域對照後補；期間寫入端填的是
    --    §6.4.14 表列的那五個字面值，接上 CodeMeta 時是一對一映射、不需改寫既有列。
    -- ⚠️ 刻意**不加 CHECK 綁定 state**：ORDER_FAILED 之外恆為 NULL 是寫入端的責任，
    --    加 CHECK 會讓未來新增 reason 值時動到既有列的約束。
    order_failure_reason     TEXT,
    order_no                 TEXT,
    payment_deadline         TIMESTAMPTZ,
    client_request_id        TEXT        NOT NULL,
    request_hash             TEXT        NOT NULL,
    accepted_at              TIMESTAMPTZ NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_prbqa_identity_matches_lane CHECK (
        (lane = 'PLATFORM' AND store_id IS NOT NULL AND prospect_id IS NULL)
     OR (lane = 'EXTERNAL' AND prospect_id IS NOT NULL AND store_id IS NULL)
    ),
    CONSTRAINT chk_prbqa_state CHECK (
        state IN ('ACTIVE', 'PAYMENT_FINALIZED', 'CANCELLED', 'EXPIRED')
    ),
    CONSTRAINT chk_prbqa_order_provisioning_state CHECK (
        order_provisioning_state IN ('CREATING_ORDER', 'ORDER_CREATED', 'ORDER_FAILED')
    ),
    CONSTRAINT chk_prbqa_order_no_matches_state CHECK (
        (order_provisioning_state = 'ORDER_CREATED') = (order_no IS NOT NULL)
    ),
    CONSTRAINT chk_prbqa_amounts_non_negative CHECK (
        goods_amount >= 0 AND shipping_amount >= 0 AND tax_amount >= 0
    ),
    CONSTRAINT chk_prbqa_price_is_sum CHECK (
        accepted_deal_price = goods_amount + shipping_amount + tax_amount
    )
);

CREATE UNIQUE INDEX uq_prbqa_idempotency
    ON procurement_buyer_quote_acceptances (buyer_id, client_request_id);

CREATE UNIQUE INDEX uq_prbqa_request_live
    ON procurement_buyer_quote_acceptances (request_id)
    WHERE state IN ('ACTIVE', 'PAYMENT_FINALIZED');

CREATE UNIQUE INDEX uq_prbqa_order_no
    ON procurement_buyer_quote_acceptances (order_no)
    WHERE order_no IS NOT NULL;

CREATE INDEX idx_prbqa_seller_response
    ON procurement_buyer_quote_acceptances (seller_response_id, seller_response_version);

-- acceptance → order shell 的投遞 outbox（doc 64 §6.2 選項 B、§6.4）。
--
-- 為什麼是 outbox 而不是同步呼叫 order-service：acceptance 在 Go 的 DB、order
-- shell 在 Java 的 DB，兩者跨語言跨資料庫，§6.2 已否決 2PC 與「由 order-service
-- 主導」。這張表與 `procurement_rfq_publication_outbox` 是**同一個慣例的第二個
-- 實例**（status／attempt_count／next_attempt_at／last_error／completed_at ＋
-- 業務鍵 unique index ＋ pending 偏索引），刻意不另立第二套機制。
--
-- ⚰️ `product_sku_id` 已於 2026-09-22 DROP（Wave C-3 的 contract 階段）。它原本是為
-- 「每筆成交先建一次性 SKU」準備的冪等回寫欄位，而 ADR 2026-09-15 推翻了那個做法：
-- RFQ 明細不指向任何 product_sku（doc 64 §6.4.9），`order_items.sku_id` 已放寬為
-- nullable ⇒ 建單前不需要 SKU。
-- ⛔ **不要把它加回來。** 要復活「一次性 SKU」這條路徑得先推翻 ADR 2026-09-15，
--    而那份 ADR 的理由（不得把 quote 偽裝成 cart SKU）與這一欄無關地仍然成立。
CREATE TABLE procurement_order_provisioning_outbox (
    id              BIGINT      PRIMARY KEY,
    acceptance_id   BIGINT      NOT NULL,
    event_type      TEXT        NOT NULL,
    payload         JSONB       NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending',
    attempt_count   INT         NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    CONSTRAINT chk_propo_event_type CHECK (
        event_type IN ('RFQ_ORDER_SHELL_REQUESTED')
    ),
    CONSTRAINT chk_propo_status CHECK (status IN ('pending', 'done', 'failed')),
    -- 終態（done／failed）必然帶完成時點；還在重試的 pending 必然沒有。兩份真相不得漂移。
    -- ⚠️ 2026-09-21 訂正述詞：原本寫 `(status = 'done') = (completed_at IS NOT NULL)`，
    --    那把「pending 不得有完成時點」誤擴成「failed 也不得有」，使 #3418 一起生成的
    --    MarkProcurementOrderProvisioningOutboxFailed（它會寫 completed_at）**結構上
    --    不可能成功**——Wave B 的 worker 第一次落終態就撞上，而在 worker 存在之前
    --    這條路徑沒有任何寫入者，所以沒人發現。
    --    判準取自同一慣例的第一個實例 procurement_rfq_publication_outbox：它的
    --    MarkFailed 同樣寫 completed_at，且它沒有這條 CHECK。⇒ 失敗也是完成。
    CONSTRAINT chk_propo_completed_at CHECK (
        (status = 'pending') = (completed_at IS NULL)
    ),
    CONSTRAINT chk_propo_attempt_count_non_negative CHECK (attempt_count >= 0)
);

-- 業務鍵直接擋在 DB，不靠應用層：一筆 acceptance 對一個 event_type 只能有一列。
-- acceptance 的 insert 走 ON CONFLICT 回放既有列，outbox 這一側因此也必須冪等。
CREATE UNIQUE INDEX uq_propo_acceptance_event
    ON procurement_order_provisioning_outbox (acceptance_id, event_type);

CREATE INDEX idx_propo_pending
    ON procurement_order_provisioning_outbox (status, next_attempt_at)
    WHERE status = 'pending';

-- Agent run ledger（doc 57 §6.2，Slice F）。v1 故意不含 lease_owner／
-- lease_expires_at／fencing_token／attempt——那四欄留給 Track 2（gated on
-- #2452）的 expand migration，本表只留 version 做 CAS。
CREATE TABLE procurement_agent_runs (
    id                  BIGINT      PRIMARY KEY,
    buyer_id            BIGINT      NOT NULL,
    conversation_id     BIGINT,
    parent_run_id       BIGINT,
    run_type            TEXT        NOT NULL,
    domain_ref_type     TEXT        NOT NULL DEFAULT '',
    domain_ref_id       TEXT        NOT NULL DEFAULT '',
    status              TEXT        NOT NULL,
    failure_reason      TEXT        NOT NULL DEFAULT '',
    resumed_into_run_id BIGINT,
    version             BIGINT      NOT NULL DEFAULT 1,
    last_event_seq      BIGINT      NOT NULL DEFAULT 0,
    min_available_seq   BIGINT      NOT NULL DEFAULT 1,
    result_revision_id  BIGINT,
    -- controller_kind (Sub-project 3b, spec §4.1): which non-legacy path
    -- claimed this run — DURABLE (ClaimAndDispatchTurn) or GRAPH
    -- (ClaimAndDispatchTurnViaGraph). No LEGACY value: Handle() never
    -- writes this table. DEFAULT 'DURABLE' is a safe backfill for every
    -- row that exists before this migration — PROCUREMENT_LLM_CONTROLLER_ENABLED
    -- has never been true for live traffic in any environment. Every
    -- application INSERT (ClaimRun, CreateLedgerRunForCandidateSearch,
    -- ResumeRun's child creation) sets this explicitly going forward.
    controller_kind     TEXT        NOT NULL DEFAULT 'DURABLE',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- PUBLICATION（#2450 Slice 2a，設計決定 1-A）：RFQ execute 建一個自己的 run 承載
    -- RFQ_PUBLICATION_RESULT artifact——procurement_agent_run_artifacts.run_id 是
    -- NOT NULL，artifact 必須掛在某個 run 上。與 CANDIDATE_SEARCH 同形（domain 動作
    -- 有自己的 run、用 domain_ref_* 指回領域物件），只是它在建立的同一個交易內就走完
    -- QUEUED → SUCCEEDED，不會有 step 事件。
    CONSTRAINT chk_procurement_agent_runs_type
        CHECK (run_type IN ('TURN', 'CANDIDATE_SEARCH', 'CLARIFICATION_RESUME', 'QUOTE_APPROVAL', 'PUBLICATION')),
    CONSTRAINT chk_procurement_agent_runs_status
        CHECK (status IN ('QUEUED','RUNNING','WAITING_FOR_INPUT','CANCEL_REQUESTED',
                           'SUCCEEDED','FAILED','CANCELLED','RESUMED','EXPIRED')),
    CONSTRAINT chk_procurement_agent_runs_controller_kind
        CHECK (controller_kind IN ('DURABLE', 'GRAPH'))
);

CREATE INDEX idx_procurement_agent_runs_buyer ON procurement_agent_runs (buyer_id, created_at DESC);
CREATE INDEX idx_procurement_agent_runs_conversation ON procurement_agent_runs (conversation_id, created_at DESC)
    WHERE conversation_id IS NOT NULL;
CREATE INDEX idx_procurement_agent_runs_parent ON procurement_agent_runs (parent_run_id)
    WHERE parent_run_id IS NOT NULL;

-- FindOpenProcurementInterruptForRequest（Task 14b B3）的必要索引：
-- result_revision_id 原本沒有索引，少了它每次 /answers、/crop-disambiguation
-- 的 open-interrupt guard 都要對 procurement_agent_runs 做一次 seq scan。
CREATE INDEX idx_procurement_agent_runs_result_revision
    ON procurement_agent_runs (result_revision_id)
    WHERE result_revision_id IS NOT NULL;

CREATE TABLE procurement_agent_run_inputs (
    run_id                BIGINT      PRIMARY KEY,
    message_id            BIGINT,
    input_ref             JSONB       NOT NULL,
    input_schema_version  TEXT        NOT NULL,
    retention_class       TEXT        NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE procurement_agent_run_events (
    id                  BIGINT      PRIMARY KEY,
    run_id              BIGINT      NOT NULL,
    seq                 BIGINT      NOT NULL,
    event_type          TEXT        NOT NULL,
    schema_version      TEXT        NOT NULL,
    public_payload_json JSONB       NOT NULL DEFAULT '{}'::jsonb,
    occurred_at         TIMESTAMPTZ NOT NULL,
    trace_id            TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_procurement_agent_run_events ON procurement_agent_run_events (run_id, seq);

CREATE TABLE procurement_agent_run_artifacts (
    artifact_id      BIGINT      NOT NULL,
    run_id           BIGINT      NOT NULL,
    artifact_type    TEXT        NOT NULL,
    version          INT         NOT NULL,
    status           TEXT        NOT NULL,
    public_data_json JSONB       NOT NULL,
    checksum         TEXT        NOT NULL,
    domain_ref_type  TEXT        NOT NULL DEFAULT '',
    domain_ref_id    TEXT        NOT NULL DEFAULT '',
    as_of            TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (artifact_id, version)
);
CREATE INDEX idx_procurement_agent_run_artifacts_run ON procurement_agent_run_artifacts (run_id, artifact_type);

CREATE TABLE procurement_agent_run_commands (
    id                BIGINT      PRIMARY KEY,
    buyer_id          BIGINT      NOT NULL,
    command_kind      TEXT        NOT NULL,
    client_request_id TEXT        NOT NULL,
    payload_hash      TEXT        NOT NULL,
    run_id            BIGINT      NOT NULL,
    status            TEXT        NOT NULL,
    result_json       JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_procurement_agent_run_commands ON procurement_agent_run_commands (buyer_id, client_request_id);

-- payload_json 是「這個中斷正在問買家什麼」的 kind-specific 內容，形狀由
-- (interrupt_kind, response_schema_version) 這組既有欄位決定——CLARIFICATION/v1
-- 是 {"pendingReason":"AMBIGUOUS|UNAVAILABLE","options":[…]}（見
-- usecase/graph_interrupt.go 的 CropDisambiguationInterruptPayload）。
--
-- 為什麼在這裡而不是 procurement_requests 上的兩個欄位（ADR
-- 2026-09-04-crop-resolution-into-slots Updates 第 2 點的後續刀）：
--
--  1. 它描述的是「現在正在問什麼」（執行狀態），生命週期就是這個 interrupt 的
--     生命週期——建立時寫、resolve 時連同整列一起變成歷史。放在 request 單列上
--     時，「清除待處理狀態」是一次獨立的 UPDATE，跟 interrupt 的建立／解決各自
--     發生，兩邊都是 log-and-swallow，於是有兩個不對稱的中間狀態：候選落庫但
--     interrupt 沒建成（買家看得到卡片、resume 卻沒有 interrupt 可回覆），以及
--     interrupt 建成但候選沒落庫（有卡片、白名單卻是空的）。收進同一列之後這
--     兩個狀態結構上不存在——候選就是 interrupt 的 INSERT 內容。
--  2. 它**不能**放 slots_json：slots_hash 是買家輸入的內容指紋，候選清單是從
--     pricebook 現況推導出來的，同一份買家輸入會因為 pricebook 更新而算出不同
--     hash。
--
-- kind-specific 內容一律進 payload_json，不為 crop 開專屬欄位：下一個
-- interrupt kind 不該需要動 schema。
--
-- facts_as_of 是組出這份 payload 時查詢 pricebook 的時刻（比照
-- procurement_candidates.facts_as_of 的語意：**記錄**這份事實的時間，不是守門
-- 條件）。守門由既有的兩道負責——resume 路徑比對
-- ExpectedRevisionID（需求本身變了），以及下游 priceAvailability 三態（選中的
-- code 查無行情就回 UNAVAILABLE，不會算出錯價）。這裡不再加第三道 TTL：門檻值
-- 沒有非任意的定法，而它會在買家正常思考的時間內把人踢掉。
CREATE TABLE procurement_agent_run_interrupts (
    id                       TEXT        PRIMARY KEY,
    run_id                   BIGINT      NOT NULL,
    interrupt_kind           TEXT        NOT NULL,
    response_schema_version  TEXT        NOT NULL,
    status                   TEXT        NOT NULL,
    payload_json             JSONB       NOT NULL DEFAULT '{}'::jsonb,
    facts_as_of              TIMESTAMPTZ,
    expires_at               TIMESTAMPTZ,
    resolved_by_child_run_id BIGINT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at              TIMESTAMPTZ
);
CREATE INDEX idx_procurement_agent_run_interrupts_run ON procurement_agent_run_interrupts (run_id);

-- ========================================================================
-- Agent 月配額（doc 57 §7、doc 59 子系統 1）— 2026-08-28
-- ========================================================================

CREATE TABLE procurement_usage (
    buyer_id     BIGINT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    usage_count  INT NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (buyer_id, period_start)
);

-- 配額預約（呼叫 Handle/HandleStream 期間持有，AppendRevision 成功則轉入
-- procurement_usage，失敗則釋放；expires_at 到期未確認即視為失效，查詢時濾掉，
-- 不需清理 job——完全比照 ai-service 既有 ai_usage_reservations 的既定模式
-- （services/ai-service/db/schema/schema.sql），只是換一組計量軸。
CREATE TABLE procurement_usage_reservations (
    id           BIGINT NOT NULL PRIMARY KEY,
    buyer_id     BIGINT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_procurement_usage_reservations_buyer_period
  ON procurement_usage_reservations (buyer_id, period_start, expires_at);
