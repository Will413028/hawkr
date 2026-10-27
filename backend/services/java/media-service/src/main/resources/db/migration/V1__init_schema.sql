-- ========================================
-- Media Service Database Schema
-- 依據設計書建立，嚴格遵守欄位定義
-- ========================================

-- 上傳會話表：記錄前端請求上傳的臨時狀態
CREATE TABLE IF NOT EXISTS upload_session (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      BIGINT          NOT NULL,
    object_key      VARCHAR(512)    NOT NULL,
    is_secure       BOOLEAN         NOT NULL DEFAULT FALSE,
    content_type    VARCHAR(64),                -- MIME 類型（建立時由前端提供）
    file_size       BIGINT,                     -- 檔案大小（位元組，建立時由前端提供）
    encryption_key  VARCHAR(256)    NULL,       -- 機敏檔案專用：AES-GCM 加密金鑰（以主密鑰加密存儲）
    asset_id        BIGINT          NULL,       -- Finalize 成功後回填，用於重入查詢
    status          VARCHAR(4)      NOT NULL,  -- CodeMeta: INIT=7001, COMPLETED=7002, FINALIZED=7003, EXPIRED=7004
    expires_at      TIMESTAMPTZ     NOT NULL,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 索引：支援依狀態與過期時間查詢（用於 GC）
CREATE INDEX IF NOT EXISTS idx_upload_session_status_expires ON upload_session(status, expires_at);
-- 索引：支援依帳號查詢
CREATE INDEX IF NOT EXISTS idx_upload_session_account ON upload_session(account_id);

-- 資源資產表：正式的檔案紀錄
CREATE TABLE IF NOT EXISTS media_asset (
    id              BIGINT          PRIMARY KEY,        -- TSID (時序 ID)
    account_id      BIGINT          NOT NULL,
    object_key      VARCHAR(512)    NOT NULL,
    encryption_key  VARCHAR(256)    NULL,       -- 機敏檔案專用：流式加密所使用的金鑰（加密存儲）
    ref_type        VARCHAR(64)     NULL,       -- 資源分類（Finalize 時設定，如 PRODUCT, SHIPPING）
    url             TEXT            NULL,       -- CDN 完整路徑，機敏檔案時為空
    mime_type       VARCHAR(64)     NOT NULL,
    size_bytes      BIGINT          NOT NULL,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    is_secure       BOOLEAN         NOT NULL DEFAULT FALSE
);

-- 索引：支援依帳號查詢
CREATE INDEX IF NOT EXISTS idx_media_asset_account ON media_asset(account_id);
-- 索引：支援依建立時間查詢（用於 GC）
CREATE INDEX IF NOT EXISTS idx_media_asset_created ON media_asset(created_at);

-- 引用關聯表：記錄誰在用這張圖
CREATE TABLE IF NOT EXISTS asset_reference (
    id              BIGSERIAL       PRIMARY KEY,
    asset_id        BIGINT          NOT NULL REFERENCES media_asset(id),
    ref_type        VARCHAR(64)     NOT NULL,   -- 業務類型 e.g., PRODUCT, USER_AVATAR
    ref_id          VARCHAR(64)     NOT NULL,   -- 業務 ID e.g., product_id
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT asset_reference_asset_id_ref_type_ref_id_key UNIQUE (asset_id, ref_type, ref_id)
);

-- 索引：支援依資產 ID 查詢引用
CREATE INDEX IF NOT EXISTS idx_asset_reference_asset ON asset_reference(asset_id);
-- 索引：支援依業務類型與 ID 查詢
CREATE INDEX IF NOT EXISTS idx_asset_reference_ref ON asset_reference(ref_type, ref_id);

-- ========================================
-- 註解說明
-- ========================================
COMMENT ON TABLE upload_session IS '上傳會話表：記錄前端請求上傳的臨時狀態';
COMMENT ON COLUMN upload_session.id IS 'Session ID (Token 內含此 ID)';
COMMENT ON COLUMN upload_session.account_id IS '帳號 ID（上傳者）';
COMMENT ON COLUMN upload_session.object_key IS 'OSS 上的路徑，例如 tmp/20260129/uuid.jpg';
COMMENT ON COLUMN upload_session.is_secure IS '是否為機敏檔案（採流式加密上傳，禁止直傳 OSS）';
COMMENT ON COLUMN upload_session.status IS '狀態碼：INIT=7001, COMPLETED=7002, FINALIZED=7003, EXPIRED=7004';
COMMENT ON COLUMN upload_session.content_type IS 'MIME 類型，例如 image/jpeg';
COMMENT ON COLUMN upload_session.file_size IS '檔案大小（位元組）';
COMMENT ON COLUMN upload_session.encryption_key IS '機敏檔案專用：AES-GCM 加密金鑰（以主密鑰加密存儲）';
COMMENT ON COLUMN upload_session.asset_id IS 'Finalize 成功後回填的 media_asset ID，用於重入查詢';
COMMENT ON COLUMN upload_session.expires_at IS 'Token 有效期';

COMMENT ON TABLE media_asset IS '資源資產表：正式的檔案紀錄';
COMMENT ON COLUMN media_asset.id IS 'Asset ID（對外流通的 ID）';
COMMENT ON COLUMN media_asset.account_id IS '帳號 ID（上傳者）';
COMMENT ON COLUMN media_asset.object_key IS 'OSS 最終路徑（於 Finalize 後固定，不可變更）';
COMMENT ON COLUMN media_asset.ref_type IS '資源分類（Finalize 時設定），對應 AssetReferenceType';
COMMENT ON COLUMN media_asset.is_secure IS '是否為機敏檔案';
COMMENT ON COLUMN media_asset.encryption_key IS '機敏檔案專用：流式加密所使用的金鑰（加密存儲）';
COMMENT ON COLUMN media_asset.url IS 'CDN 完整路徑，機敏檔案時為空字串';
COMMENT ON COLUMN media_asset.mime_type IS 'MIME 類型，例如 image/jpeg';
COMMENT ON COLUMN media_asset.size_bytes IS '檔案大小（位元組）';

COMMENT ON TABLE asset_reference IS '引用關聯表：記錄誰在用這張圖';
COMMENT ON COLUMN asset_reference.asset_id IS '資產 ID，外鍵關聯 media_asset';
COMMENT ON COLUMN asset_reference.ref_type IS '業務類型，例如 PRODUCT、USER_AVATAR';
COMMENT ON COLUMN asset_reference.ref_id IS '業務 ID，例如 product_id';

-- ========================================
-- updated_at 自動更新觸發器
-- ========================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS update_upload_session_updated_at ON upload_session;
CREATE TRIGGER update_upload_session_updated_at
    BEFORE UPDATE ON upload_session
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
