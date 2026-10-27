-- ============================================================================
-- Product Import Service 初始化 schema（40 號設計書 §3）
--
-- 涵蓋 7 張表：
--   1. product_import_job              — 匯入任務主表
--   2. product_import_item             — 暫存商品主表
--   3. product_import_item_sku         — 暫存 SKU 表
--   4. product_import_item_asset       — 暫存圖片資產表
--   5. product_import_item_error       — Row-level 解析/校驗錯誤明細
--   6. product_import_commit_attempt   — 每次 commit 嘗試結果
--   7. product_import_media_callback_inbox — Media callback event 去重 inbox
--
-- 跨服務 schema（不在本 migration）：
--   - import_asset（屬 Media Service，由 media-service V{N+1} migration 建立）
--   - product.import_job_id / product.import_item_id（屬 Product Service）
-- ============================================================================

-- ----------------------------------------------------------------------------
-- 1. product_import_job — 匯入任務主表
-- ----------------------------------------------------------------------------
CREATE TABLE product_import_job (
    id                                BIGINT       PRIMARY KEY,                       -- TSID
    account_id                        BIGINT       NOT NULL,
    status                            VARCHAR(20)  NOT NULL,                          -- ImportJobStatus value (9101~9108, 9111)
    source_filename                   VARCHAR(255) NOT NULL,
    source_object_key                 VARCHAR(500) NOT NULL,                          -- OSS tmp 區路徑
    source_file_size                  BIGINT       NOT NULL,
    source_file_hash                  VARCHAR(64)  NOT NULL,                          -- SHA256 hex，dedup 用
    total_rows                        INTEGER,
    total_items                       INTEGER,
    parse_error                       TEXT,
    target_status                     VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE',
    expires_at                        TIMESTAMPTZ  NOT NULL,
    committed_at                      TIMESTAMPTZ,
    cancelled_at                      TIMESTAMPTZ,
    parsed_event_published_at         TIMESTAMPTZ,                                    -- 一次性 flag
    commit_finished_event_published_at TIMESTAMPTZ,
    commit_finished_event_id          VARCHAR(64),
    validation_errors                 JSONB,                                          -- L2 失敗時的明細
    version                           INTEGER      NOT NULL DEFAULT 0,
    created_at                        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_pij_status         CHECK (status IN ('9101','9102','9103','9104','9105','9106','9107','9108','9111')),
    CONSTRAINT chk_pij_target_status  CHECK (target_status IN ('ACTIVE','DRAFT','PENDING_REVIEW'))
);

CREATE INDEX idx_pij_account_status          ON product_import_job (account_id, status, created_at DESC);
CREATE INDEX idx_pij_account_hash            ON product_import_job (account_id, source_file_hash);
CREATE INDEX idx_pij_expires_at_for_gc       ON product_import_job (expires_at)
    WHERE status NOT IN ('9106','9107','9108','9103');
CREATE INDEX idx_pij_account_filename_active ON product_import_job (account_id, source_filename)
    WHERE status IN ('9101','9102','9104','9105','9111');

COMMENT ON TABLE product_import_job IS '商品匯入任務主表（40 號設計書 §3.1）';
COMMENT ON COLUMN product_import_job.source_file_hash IS 'SHA256(file_content) hex；24h 內同 account + hash 觸發 dedup';
COMMENT ON COLUMN product_import_job.parsed_event_published_at IS '一次性 flag — IS NULL 才 publish ImportJobParsedEvent';

-- ----------------------------------------------------------------------------
-- 2. product_import_item — 暫存商品主表
-- ----------------------------------------------------------------------------
CREATE TABLE product_import_item (
    id                            BIGINT       PRIMARY KEY,                           -- TSID
    import_job_id                 BIGINT       NOT NULL REFERENCES product_import_job(id),
    account_id                    BIGINT       NOT NULL,
    product_key                   VARCHAR(64)  NOT NULL,
    status                        VARCHAR(20)  NOT NULL,                              -- ImportItemStatus value (9201/9202/9203/9205/9206/9207/9208)
    commit_revision               INTEGER      NOT NULL DEFAULT 0,                    -- 賣家每次有效編輯 +1
    -- 商品本體
    name                          VARCHAR(80)  NOT NULL,
    category_id                   BIGINT,
    origin                        VARCHAR(100),
    target_status                 VARCHAR(20),                                        -- NULL = 繼承 job
    description_json              JSONB,
    hashtags                      JSONB,
    certifications                JSONB,
    -- 反正規化欄位（reflow）
    cat_l1_id                     BIGINT,
    cat_l1_name                   VARCHAR(50),
    cat_l2_id                     BIGINT,
    cat_l2_name                   VARCHAR(50),
    cat_l3_name                   VARCHAR(50),
    -- 解析來源追溯
    first_csv_line                INTEGER      NOT NULL,
    sku_count                     INTEGER      NOT NULL DEFAULT 0,
    warning_count                 INTEGER      NOT NULL DEFAULT 0,
    blocking_error_count          INTEGER      NOT NULL DEFAULT 0,
    -- 重名偵測
    allow_duplicate_name          BOOLEAN      NOT NULL DEFAULT FALSE,
    duplicate_existing_product_id BIGINT,
    -- 終態欄位
    formal_product_id             BIGINT,
    commit_failed_reason          TEXT,
    removed_at                    TIMESTAMPTZ,
    committed_at                  TIMESTAMPTZ,
    version                       INTEGER      NOT NULL DEFAULT 0,
    created_at                    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_pii_job_key     UNIQUE (import_job_id, product_key),
    CONSTRAINT chk_pii_status     CHECK (status IN ('9201','9202','9203','9205','9206','9207','9208'))
);

CREATE INDEX idx_pii_job_status              ON product_import_item (import_job_id, status);
CREATE INDEX idx_pii_account_created         ON product_import_item (account_id, created_at DESC);
CREATE INDEX idx_pii_formal_product          ON product_import_item (formal_product_id)
    WHERE formal_product_id IS NOT NULL;
CREATE INDEX idx_pii_job_needs_fix           ON product_import_item (import_job_id, blocking_error_count)
    WHERE blocking_error_count > 0;
CREATE INDEX idx_pii_job_duplicate_existing  ON product_import_item (import_job_id, duplicate_existing_product_id)
    WHERE duplicate_existing_product_id IS NOT NULL;

COMMENT ON TABLE product_import_item IS '暫存商品主表（40 號設計書 §3.2）';
COMMENT ON COLUMN product_import_item.blocking_error_count IS '反正規化 = COUNT(*) FROM product_import_item_error WHERE severity=ERROR';

-- ----------------------------------------------------------------------------
-- 3. product_import_item_sku — 暫存 SKU 表
-- ----------------------------------------------------------------------------
CREATE TABLE product_import_item_sku (
    id              BIGINT       PRIMARY KEY,                                         -- TSID
    import_item_id  BIGINT       NOT NULL REFERENCES product_import_item(id) ON DELETE CASCADE,
    csv_line        INTEGER      NOT NULL,
    sku_name        VARCHAR(80),
    price           BIGINT       NOT NULL,                                            -- 元
    sale_price      BIGINT,
    stock_qty       INTEGER      NOT NULL,
    weight          NUMERIC(10,2),
    unit            VARCHAR(20)  NOT NULL,                                            -- WeightUnit value
    length_cm       NUMERIC(8,2),
    width_cm        NUMERIC(8,2),
    height_cm       NUMERIC(8,2),
    shipping_methods JSONB       NOT NULL,
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_piisku_unit  CHECK (unit IN ('KG','CATTY','GRAM')),
    CONSTRAINT chk_piisku_price CHECK (price > 0),
    CONSTRAINT chk_piisku_stock CHECK (stock_qty >= 0),
    CONSTRAINT chk_piisku_sale  CHECK (sale_price IS NULL OR (sale_price > 0 AND sale_price < price))
);

CREATE INDEX idx_piisku_item_sort ON product_import_item_sku (import_item_id, sort_order);

COMMENT ON TABLE product_import_item_sku IS '暫存 SKU 表（40 號設計書 §3.3）';

-- ----------------------------------------------------------------------------
-- 4. product_import_item_asset — 暫存圖片資產表
-- ----------------------------------------------------------------------------
CREATE TABLE product_import_item_asset (
    id                    BIGINT        PRIMARY KEY,                                  -- TSID
    import_item_id        BIGINT        NOT NULL REFERENCES product_import_item(id) ON DELETE CASCADE,
    import_job_id         BIGINT        NOT NULL,
    usage                 VARCHAR(20)   NOT NULL,                                     -- GALLERY | DESCRIPTION
    sort_order            INTEGER       NOT NULL DEFAULT 0,
    block_index           INTEGER,
    source_url            VARCHAR(2000) NOT NULL,
    source_url_hash       VARCHAR(64)   NOT NULL,
    status                VARCHAR(20)   NOT NULL,                                     -- ImportAssetLocalStatus value (9301~9306)
    media_import_asset_id VARCHAR(64),
    object_key            VARCHAR(500),
    preview_url           VARCHAR(1000),
    failure_reason        VARCHAR(80),
    failure_detail        TEXT,
    retry_count           INTEGER       NOT NULL DEFAULT 0,
    finalized_asset_id    BIGINT,
    version               INTEGER       NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_piia_usage  CHECK (usage IN ('GALLERY','DESCRIPTION')),
    CONSTRAINT chk_piia_status CHECK (status IN ('9301','9302','9303','9304','9305','9306'))
);

CREATE INDEX idx_piia_item       ON product_import_item_asset (import_item_id, usage, sort_order);
CREATE INDEX idx_piia_job_status ON product_import_item_asset (import_job_id, status);
CREATE INDEX idx_piia_dedup      ON product_import_item_asset (import_job_id, source_url_hash);
CREATE INDEX idx_piia_orphan_gc  ON product_import_item_asset (status, updated_at) WHERE status = '9306';

COMMENT ON TABLE product_import_item_asset IS '暫存圖片資產表（40 號設計書 §3.4）';

-- ----------------------------------------------------------------------------
-- 5. product_import_item_error — Row-level 解析/校驗錯誤明細
-- ----------------------------------------------------------------------------
CREATE TABLE product_import_item_error (
    id              BIGSERIAL    PRIMARY KEY,
    import_item_id  BIGINT       REFERENCES product_import_item(id) ON DELETE CASCADE,
    import_job_id   BIGINT       NOT NULL,
    csv_line        INTEGER,
    error_type      VARCHAR(40)  NOT NULL,                                            -- ImportItemErrorType value
    severity        VARCHAR(10)  NOT NULL,                                            -- ERROR | WARNING
    field           VARCHAR(80),
    message         VARCHAR(500) NOT NULL,
    raw_value       TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_piie_severity CHECK (severity IN ('ERROR','WARNING'))
);

CREATE INDEX idx_piie_item     ON product_import_item_error (import_item_id, severity);
CREATE INDEX idx_piie_job_type ON product_import_item_error (import_job_id, error_type);

COMMENT ON TABLE product_import_item_error IS 'Row-level 錯誤明細（40 號設計書 §3.5）';

-- ----------------------------------------------------------------------------
-- 6. product_import_commit_attempt — 每次 commit 嘗試結果
-- ----------------------------------------------------------------------------
CREATE TABLE product_import_commit_attempt (
    id              BIGSERIAL    PRIMARY KEY,
    import_item_id  BIGINT       NOT NULL REFERENCES product_import_item(id) ON DELETE CASCADE,
    import_job_id   BIGINT       NOT NULL,
    revision        INTEGER      NOT NULL,
    idempotency_key VARCHAR(120) NOT NULL,
    started_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ,
    result          VARCHAR(20)  NOT NULL,                                            -- IN_PROGRESS | SUCCEEDED | FAILED
    formal_product_id BIGINT,
    failure_reason  TEXT,
    grpc_code       INTEGER,
    CONSTRAINT chk_pica_result CHECK (result IN ('IN_PROGRESS','SUCCEEDED','FAILED')),
    CONSTRAINT uk_pica_idempotency UNIQUE (idempotency_key)
);

CREATE INDEX idx_pica_item ON product_import_commit_attempt (import_item_id, started_at DESC);
CREATE INDEX idx_pica_job  ON product_import_commit_attempt (import_job_id, result);

COMMENT ON TABLE product_import_commit_attempt IS '每次 commit 嘗試結果（40 號設計書 §3.6）';

-- ----------------------------------------------------------------------------
-- 7. product_import_media_callback_inbox — Media callback 去重 inbox
-- ----------------------------------------------------------------------------
CREATE TABLE product_import_media_callback_inbox (
    event_id        VARCHAR(64)  PRIMARY KEY,
    import_asset_id VARCHAR(64)  NOT NULL,
    result_status   VARCHAR(20)  NOT NULL,                                            -- UPLOADED | FAILED
    processed_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pimci_processed ON product_import_media_callback_inbox (processed_at DESC);

COMMENT ON TABLE product_import_media_callback_inbox IS 'Media callback event 去重 inbox（40 號設計書 §3.6a）';
