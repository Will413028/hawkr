CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE market (
    id            BIGINT PRIMARY KEY,
    market_code   TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT market_code_not_blank CHECK (length(btrim(market_code)) > 0),
    CONSTRAINT market_name_not_blank CHECK (length(btrim(name)) > 0)
);

CREATE TABLE crop_group (
    id            BIGINT PRIMARY KEY,
    group_code    TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    category_code TEXT,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT crop_group_code_not_blank CHECK (length(btrim(group_code)) > 0),
    CONSTRAINT crop_group_name_not_blank CHECK (length(btrim(name)) > 0)
);

CREATE TABLE crop_group_forecast_result (
    id                      BIGINT PRIMARY KEY,
    group_code              TEXT NOT NULL REFERENCES crop_group (group_code),
    generated_at            TIMESTAMPTZ NOT NULL,
    target_date             DATE NOT NULL,
    horizon_days            INTEGER NOT NULL,
    status                  TEXT NOT NULL,
    missing_reason          TEXT,
    aggregation_method      TEXT NOT NULL,
    effective_variant_count INTEGER NOT NULL,
    total_variant_count     INTEGER NOT NULL,
    market_support_count    INTEGER NOT NULL,
    p10                     NUMERIC(12,4),
    p50                     NUMERIC(12,4),
    p90                     NUMERIC(12,4),
    freshness               TEXT NOT NULL,
    model_version           TEXT NOT NULL,
    dataset_digest          TEXT NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX crop_group_forecast_result_lookup_idx
    ON crop_group_forecast_result (group_code, target_date DESC);
CREATE UNIQUE INDEX crop_group_forecast_result_dedup_idx
    ON crop_group_forecast_result (group_code, target_date, ((generated_at AT TIME ZONE 'UTC')::date));

CREATE TABLE crop (
    id            BIGINT PRIMARY KEY,
    crop_code     TEXT NOT NULL UNIQUE,
    group_id      BIGINT NOT NULL REFERENCES crop_group (id),
    name          TEXT NOT NULL,
    variant_name  TEXT,
    category_code TEXT,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT crop_code_not_blank CHECK (length(btrim(crop_code)) > 0),
    CONSTRAINT crop_name_not_blank CHECK (length(btrim(name)) > 0)
);
CREATE INDEX crop_group_id_idx ON crop (group_id);

CREATE TABLE market_watch_target (
    id BIGINT PRIMARY KEY,
    account_id BIGINT NOT NULL,
    crop_id BIGINT NOT NULL REFERENCES crop (id),
    market_id BIGINT REFERENCES market (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uidx_market_watch_target_all_market
    ON market_watch_target (account_id, crop_id) WHERE market_id IS NULL;
CREATE UNIQUE INDEX uidx_market_watch_target_market
    ON market_watch_target (account_id, crop_id, market_id) WHERE market_id IS NOT NULL;
CREATE INDEX idx_market_watch_target_account_created
    ON market_watch_target (account_id, created_at DESC, id DESC);

CREATE TABLE category_crop_mapping (
    category_id     TEXT PRIMARY KEY,
    status          TEXT NOT NULL,
    group_code      TEXT,
    commodity_id    BIGINT,
    mapping_version TEXT NOT NULL,
    source_note     TEXT,
    updated_by      BIGINT NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT category_crop_mapping_status_check
        CHECK (status IN ('resolved', 'unmapped', 'ambiguous')),
    CONSTRAINT category_crop_mapping_resolved_oneof_check
        CHECK (
            (status = 'resolved' AND (group_code IS NULL) <> (commodity_id IS NULL)) OR
            (status <> 'resolved' AND group_code IS NULL AND commodity_id IS NULL)
        )
);
CREATE INDEX category_crop_mapping_commodity_idx
    ON category_crop_mapping (commodity_id) WHERE commodity_id IS NOT NULL;

CREATE TABLE category_crop_mapping_crop (
    category_id TEXT   NOT NULL REFERENCES category_crop_mapping (category_id) ON DELETE CASCADE,
    crop_id     BIGINT NOT NULL REFERENCES crop (id),
    PRIMARY KEY (category_id, crop_id)
);
CREATE INDEX category_crop_mapping_crop_crop_idx
    ON category_crop_mapping_crop (crop_id);

CREATE TABLE price_observation (
    id            BIGINT PRIMARY KEY,
    market_id     BIGINT NOT NULL REFERENCES market (id),
    crop_id       BIGINT NOT NULL REFERENCES crop (id),
    trade_date    DATE NOT NULL,
    revision_no   INTEGER NOT NULL,
    available_at  TIMESTAMPTZ NOT NULL,
    avg_price     NUMERIC(12,4),
    upper_price   NUMERIC(12,4),
    middle_price  NUMERIC(12,4),
    lower_price   NUMERIC(12,4),
    volume_kg     NUMERIC(14,3),
    source        TEXT NOT NULL,
    source_digest TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (market_id, crop_id, trade_date, revision_no)
);
CREATE INDEX price_observation_series_idx ON price_observation (market_id, crop_id, trade_date DESC);
CREATE INDEX price_observation_available_at_idx ON price_observation (available_at);

CREATE TABLE market_rest_day (
    id          BIGINT PRIMARY KEY,
    market_id   BIGINT NOT NULL REFERENCES market (id),
    market_type TEXT,
    rest_date   DATE NOT NULL,
    reason      TEXT,
    fetched_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (market_id, market_type, rest_date)
);

CREATE TABLE ingest_partition (
    market_id       BIGINT NOT NULL REFERENCES market (id),
    trade_date      DATE NOT NULL,
    status          SMALLINT NOT NULL,
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    declared_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_attempt_at TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    last_error      TEXT,
    accepted_at     TIMESTAMPTZ,
    accepted_by     TEXT,
    accepted_reason TEXT,
    PRIMARY KEY (market_id, trade_date),
    CONSTRAINT ingest_partition_accepted_evidence CHECK (
        status <> 5
        OR (accepted_at IS NOT NULL
            AND accepted_by IS NOT NULL
            AND accepted_reason IS NOT NULL)
    )
);

CREATE INDEX ingest_partition_pending_idx
    ON ingest_partition (trade_date)
    WHERE status = 1;

CREATE TABLE ingest_run (
    id             BIGINT PRIMARY KEY,
    kind           SMALLINT NOT NULL,
    range_start    DATE NOT NULL,
    range_end      DATE NOT NULL,
    status         SMALLINT NOT NULL,
    fetched_count  INTEGER NOT NULL DEFAULT 0,
    inserted_count INTEGER NOT NULL DEFAULT 0,
    revised_count  INTEGER NOT NULL DEFAULT 0,
    error_message  TEXT,
    started_at     TIMESTAMPTZ NOT NULL,
    finished_at    TIMESTAMPTZ
);

CREATE TABLE ingest_rejected_identity (
    source           TEXT NOT NULL,
    crop_code        TEXT NOT NULL,
    market_code      TEXT NOT NULL,
    reason           TEXT NOT NULL,
    occurrence_count BIGINT NOT NULL DEFAULT 1,
    first_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    first_trade_date TEXT NOT NULL,
    last_trade_date  TEXT NOT NULL,
    PRIMARY KEY (source, crop_code, market_code, reason)
);
CREATE INDEX ingest_rejected_identity_last_seen_idx
    ON ingest_rejected_identity (last_seen_at DESC);

CREATE TABLE category_price_rollup (
    category_id         TEXT PRIMARY KEY,
    level                SMALLINT NOT NULL,   -- 1｜2｜3，對齊 product-service 分類樹深度
    status               TEXT NOT NULL,
    latest_price         NUMERIC(12,4),
    prev_price           NUMERIC(12,4),
    points               JSONB,               -- 近 N 日 [{tradeDate, avgPrice}]，供小型走勢圖
    forecast_points      JSONB,               -- 未來預測點 [{tradeDate, forecastPrice}]，L1/L2 專用
    source_group_count   SMALLINT NOT NULL,   -- 參與這次彙總的底層 group／子分類數，資料品質判讀用
    price_layer          TEXT,
    unit                 TEXT,
    computed_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT category_price_rollup_level_check CHECK (level IN (1, 2, 3)),
    CONSTRAINT category_price_rollup_status_check
        CHECK (status IN ('ready', 'mixed_layer', 'unavailable')),
    CONSTRAINT category_price_rollup_price_layer_check
        CHECK (price_layer IS NULL OR price_layer IN (
            'wholesale_auction', 'farmgate', 'live_animal', 'retail', 'wholesale_bulk'
        )),
    CONSTRAINT category_price_rollup_layer_presence_check
        CHECK (status <> 'ready' OR (price_layer IS NOT NULL AND unit IS NOT NULL))
);
CREATE INDEX category_price_rollup_level_idx ON category_price_rollup (level);

CREATE TABLE commodity (
    id              BIGINT PRIMARY KEY,
    source          TEXT NOT NULL,
    commodity_code  TEXT NOT NULL,
    name            TEXT NOT NULL,
    price_layer     TEXT NOT NULL,
    unit            TEXT NOT NULL,
    canonical_unit  TEXT,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT commodity_price_layer_check CHECK (price_layer IN (
        'wholesale_auction', 'farmgate', 'live_animal', 'retail', 'wholesale_bulk'
    )),
    UNIQUE (source, commodity_code, price_layer)
);
CREATE INDEX commodity_source_idx ON commodity (source);

-- 可定價實體的表面形式索引：crop / crop_group / commodity 的正式名、品種名、
-- 來源術語、別名，全部住這一張表，只有一個寫入點（ingest 的 upsert 同交易寫入）。
--
-- form_norm 是 GENERATED 欄位：寫入端只寫 form_raw，正規化不可能被跳過或漂移，
-- 不論寫入的是 Go 還是手打 SQL。改正規化規則＝改下面的運算式，Atlas 會產生
-- drop/re-add 把每一列重算，所以不需要 norm_version 欄位——版本就是這段 schema。
--
-- 同一段運算式也套在查詢參數上（db/queries/surface_form.sql），兩份必須逐字
-- 相同；internal/repository/postgres/surface_form_expr_test.go 讀兩個檔案比對。
-- 標記行 norm-expr-begin / norm-expr-end 是那支測試的錨點，不要動。
--
-- 為什麼不用 SQL function：Atlas OSS 不管理 function/trigger（Pro 功能），
-- 本 repo 零先例；generated column 是核心欄位 DDL，OSS 可用。
CREATE TABLE priceable_surface_form (
    entity_kind TEXT NOT NULL,
    entity_code TEXT NOT NULL,
    form_raw    TEXT NOT NULL,
    form_kind   TEXT NOT NULL,
    form_norm   TEXT NOT NULL GENERATED ALWAYS AS (
        -- norm-expr-begin
        lower(translate(
            regexp_replace(form_raw, '[[:space:][:punct:]、。，！？：；（）「」『』【】《》〈〉—…·～]', '', 'g'),
            'ＡＢＣＤＥＦＧＨＩＪＫＬＭＮＯＰＱＲＳＴＵＶＷＸＹＺａｂｃｄｅｆｇｈｉｊｋｌｍｎｏｐｑｒｓｔｕｖｗｘｙｚ０１２３４５６７８９臺',
            'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789台'
        ))
        -- norm-expr-end
    ) STORED,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_kind, entity_code, form_norm),
    CONSTRAINT priceable_surface_form_kind_check
        CHECK (entity_kind IN ('CROP_GROUP', 'CROP', 'COMMODITY')),
    CONSTRAINT priceable_surface_form_form_kind_check
        CHECK (form_kind IN ('CANONICAL', 'SOURCE_TERM', 'VARIANT', 'ALIAS')),
    CONSTRAINT priceable_surface_form_raw_not_blank CHECK (length(btrim(form_raw)) > 0)
);
CREATE INDEX priceable_surface_form_norm_trgm_idx
    ON priceable_surface_form USING gin (form_norm gin_trgm_ops);
CREATE INDEX priceable_surface_form_entity_idx
    ON priceable_surface_form (entity_kind, entity_code);

CREATE TABLE commodity_observation (
    id             BIGINT PRIMARY KEY,
    commodity_id   BIGINT NOT NULL REFERENCES commodity (id),
    market_code    TEXT,
    trade_date     DATE NOT NULL,
    revision_no    INTEGER NOT NULL,
    available_at   TIMESTAMPTZ NOT NULL,
    avg_price      NUMERIC(12,4),
    canonical_price NUMERIC(12,4),
    quantity       NUMERIC(14,3),
    quantity_unit  TEXT,
    source         TEXT NOT NULL,
    source_digest  TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uidx_commodity_observation_national
    ON commodity_observation (commodity_id, trade_date, revision_no)
    WHERE market_code IS NULL;
CREATE UNIQUE INDEX uidx_commodity_observation_market
    ON commodity_observation (commodity_id, market_code, trade_date, revision_no)
    WHERE market_code IS NOT NULL;
CREATE INDEX commodity_observation_series_idx
    ON commodity_observation (commodity_id, trade_date DESC);
CREATE INDEX commodity_observation_available_at_idx
    ON commodity_observation (available_at);

CREATE TABLE commodity_ingest_partition (
    source          TEXT NOT NULL,
    trade_date      DATE NOT NULL,
    status          SMALLINT NOT NULL,
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    declared_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_attempt_at TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    last_error      TEXT,
    PRIMARY KEY (source, trade_date)
);
CREATE INDEX commodity_ingest_partition_status_idx
    ON commodity_ingest_partition (status, trade_date DESC);

ALTER TABLE category_crop_mapping
    ADD CONSTRAINT category_crop_mapping_commodity_id_fkey
    FOREIGN KEY (commodity_id) REFERENCES commodity (id);
