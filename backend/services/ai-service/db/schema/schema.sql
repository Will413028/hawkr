-- ADK Session 儲存（對應 google.golang.org/adk/v2/session/database 的 GORM model）

CREATE TABLE sessions (
  app_name    TEXT NOT NULL,
  user_id     TEXT NOT NULL,
  id          TEXT NOT NULL,
  state       JSONB NOT NULL DEFAULT '{}',
  create_time TIMESTAMPTZ(6) NOT NULL DEFAULT NOW(),
  update_time TIMESTAMPTZ(6) NOT NULL DEFAULT NOW(),
  PRIMARY KEY (app_name, user_id, id)
);

CREATE TABLE events (
  id                          TEXT NOT NULL,
  app_name                    TEXT NOT NULL,
  user_id                     TEXT NOT NULL,
  session_id                  TEXT NOT NULL,
  invocation_id               TEXT NOT NULL DEFAULT '',
  author                      TEXT NOT NULL DEFAULT '',
  actions                     BYTEA,
  long_running_tool_ids_json  JSONB,
  routes_json                 JSONB,
  output_json                 JSONB,
  node_info_json              JSONB,
  requested_input_json        JSONB,
  branch                      TEXT,
  isolation_scope             TEXT,
  "timestamp"                 TIMESTAMPTZ(6),
  content                     JSONB,
  grounding_metadata          JSONB,
  custom_metadata             JSONB,
  usage_metadata              JSONB,
  citation_metadata           JSONB,
  partial                     BOOLEAN,
  turn_complete               BOOLEAN,
  error_code                  TEXT,
  error_message               TEXT,
  interrupted                 BOOLEAN,
  PRIMARY KEY (id, app_name, user_id, session_id),
  CONSTRAINT fk_sessions_events
    FOREIGN KEY (app_name, user_id, session_id)
    REFERENCES sessions (app_name, user_id, id)
    ON DELETE CASCADE
);

CREATE TABLE app_states (
  app_name    TEXT NOT NULL PRIMARY KEY,
  state       JSONB NOT NULL DEFAULT '{}',
  update_time TIMESTAMPTZ(6) NOT NULL DEFAULT NOW()
);

CREATE TABLE user_states (
  app_name    TEXT NOT NULL,
  user_id     TEXT NOT NULL,
  state       JSONB NOT NULL DEFAULT '{}',
  update_time TIMESTAMPTZ(6) NOT NULL DEFAULT NOW(),
  PRIMARY KEY (app_name, user_id)
);

-- 加速 24h 清理查詢
CREATE INDEX idx_sessions_update_time ON sessions (update_time);
CREATE INDEX idx_events_timestamp ON events ("timestamp");

-- AI 修圖風格管理
CREATE TABLE ai_styles (
    id            BIGINT NOT NULL PRIMARY KEY,
    name          TEXT NOT NULL,
    label         TEXT NOT NULL,
    prompt        TEXT NOT NULL,
    thumbnail_key TEXT NOT NULL DEFAULT '',
    sort_order    INTEGER NOT NULL DEFAULT 0,
    is_default    BOOLEAN NOT NULL DEFAULT FALSE,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    category      TEXT NOT NULL DEFAULT 'product' CHECK (category IN ('product', 'post')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_ai_styles_category_name ON ai_styles (category, name);
CREATE UNIQUE INDEX idx_ai_styles_default ON ai_styles (category) WHERE is_default = TRUE;
CREATE INDEX idx_ai_styles_sort ON ai_styles (enabled, sort_order);

-- AI 修圖每店每週使用紀錄。綁 store_id（② re-key store-stint surrogate）不是 login
-- account_id：賣家刪店重開視為新店，該週用量重新計算（業務決定，非漏改）。
CREATE TABLE ai_usage (
    store_id    BIGINT NOT NULL,
    week_start  DATE NOT NULL,
    usage_count INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (store_id, week_start)
);

-- Admin 個別 store 額度上限覆寫。綁 store_id，同 ai_usage —— account_id 只用於登入
-- 身份查詢，不當任何業務表的 key。
CREATE TABLE ai_quotas (
    store_id     BIGINT PRIMARY KEY,
    weekly_limit INTEGER NOT NULL,
    memo         TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 修圖名額預約（呼叫 Gemini 期間持有，成功則轉入 ai_usage，失敗則釋放；expires_at 到期
-- 未確認即視為失效，查詢時濾掉，不需清理 job；避免 process 被殺導致額度永久洩漏）
CREATE TABLE ai_usage_reservations (
    id          BIGINT NOT NULL PRIMARY KEY,
    store_id    BIGINT NOT NULL,
    week_start  DATE NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_usage_reservations_store_week ON ai_usage_reservations (store_id, week_start, expires_at);

-- 開發名單候選廠商的歷史來源表（原來源：開發名單.xlsx，經 scripts/import_candidate_leads.go
-- 一次性匯入；該腳本已退場）。「AI 找貨」agent 的站外業者主檔已遷移進 seller-service 的
-- seller_prospects（一次性遷移腳本：scripts/migrate_candidate_leads_to_prospects.go），
-- 現在經 gRPC SearchSellerProspects 查詢，不再讀這張表——見
-- services/ai-service/internal/client/seller/adapter.go 與
-- services/ai-service/internal/usecase/candidate_search_usecase.go。本表保留但不再被
-- 讀取（expand-contract 的 expand 段尚未走到 drop，見 spec §7）。
CREATE TABLE candidate_leads (
    id               BIGINT NOT NULL PRIMARY KEY,
    source_sheet     TEXT NOT NULL,
    brand_name       TEXT NOT NULL DEFAULT '',
    category         TEXT NOT NULL DEFAULT '',
    product_summary  TEXT NOT NULL DEFAULT '',
    region           TEXT NOT NULL DEFAULT '',
    positioning      TEXT NOT NULL DEFAULT '',
    research_summary TEXT NOT NULL DEFAULT '',
    public_website   TEXT NOT NULL DEFAULT '',
    public_email     TEXT NOT NULL DEFAULT '',
    public_phone     TEXT NOT NULL DEFAULT '',
    public_social    TEXT NOT NULL DEFAULT '',
    priority_letter  TEXT NOT NULL DEFAULT '',
    priority_stars   INTEGER NOT NULL DEFAULT 0,
    imported_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_candidate_leads_category ON candidate_leads (category);

