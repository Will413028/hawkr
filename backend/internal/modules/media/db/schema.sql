-- media 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（media.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。
-- 儲存的代碼是 UPPER_SNAKE，值與 internal/model 的 Go 常數一致（ADR 2026-09-25-codemeta-retirement D3）。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA media;

-- 資產：只有可見性與物件位置，不認識任何業務類型（B1-D3）；引用只存在擁有者的資料表。
-- STAGED：處理完成、尚未被擁有者確認（暫存列 READY）；ACTIVE：已確認，只由擁有者的 MarkDeleted 刪除；
-- DELETING：已標記刪除、物件尚未刪掉；DELETED：物件已刪（保留列作為冪等的墓碑）。
CREATE TABLE media.assets (
    id           UUID        PRIMARY KEY,
    visibility   TEXT        NOT NULL,
    object_key   TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL,
    state        TEXT        NOT NULL,
    -- 機敏資產的信封加密：包裹 DEK 的 KEK 代號與 wrapped DEK（nonce＋密文＋tag，AAD 是資產 ID 加 kek_id）；公開資產沒有。
    kek_id       TEXT,
    wrapped_dek  BYTEA,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT assets_object_key_key UNIQUE (object_key),
    CONSTRAINT assets_visibility_check CHECK (visibility IN ('PUBLIC', 'SECURE')),
    CONSTRAINT assets_state_check CHECK (state IN ('STAGED', 'ACTIVE', 'DELETING', 'DELETED')),
    CONSTRAINT assets_content_type_check CHECK (content_type <> ''),
    CONSTRAINT assets_size_check CHECK (size_bytes >= 0),
    CONSTRAINT assets_kek_id_check CHECK ((visibility = 'SECURE') = (kek_id IS NOT NULL)),
    CONSTRAINT assets_wrapped_dek_check CHECK ((visibility = 'SECURE') = (wrapped_dek IS NOT NULL)),
    CONSTRAINT assets_object_key_prefix_check CHECK (
        (visibility = 'PUBLIC' AND object_key LIKE 'public/%') OR (visibility = 'SECURE' AND object_key LIKE 'secure/%')
    )
);
-- 實體刪除 job 只掃 DELETING。
CREATE INDEX assets_deleting ON media.assets (id) WHERE state = 'DELETING';

-- 暫存上傳：token 與暫存列一對一（B1-D2），只存 token 的雜湊，不存明文。
-- 狀態機（每個轉換都是條件式 UPDATE，WHERE state=<預期>）：
--   PENDING → UPLOADED → READY | REJECTED → CONFIRMED（secure 的上傳沒有 PENDING：以 UPLOADED 建列，見 internal/upload/secure_upload.go）
--   PENDING、READY、REJECTED 且過期，或 UPLOADED 且過期超過寬限（處理 job 卡住）→ DELETING → DELETED（GC）
-- final_key 在 PENDING → UPLOADED 時產生並記下，處理 job 重試寫同一個 key；asset_id 在 READY 時才有（READY 之前資產列不存在）。
CREATE TABLE media.staged_uploads (
    id                    UUID        PRIMARY KEY,
    token_hash            BYTEA       NOT NULL,
    uploader_id           BIGINT      NOT NULL,
    uploader_admin        BOOLEAN     NOT NULL,
    visibility            TEXT        NOT NULL,
    declared_size         BIGINT      NOT NULL,
    declared_content_type TEXT        NOT NULL,
    -- 宣告的內容 MD5，標準 base64（預簽 PUT 的 Content-MD5）；只有 public 走預簽，secure 是 NULL。
    declared_md5          TEXT,
    state                 TEXT        NOT NULL,
    reject_reason         TEXT,
    -- 預簽直傳的暫存 key；secure 經單體串流上傳，沒有暫存物件，是 NULL。
    tmp_key               TEXT,
    final_key             TEXT,
    asset_id              UUID,
    expires_at            TIMESTAMPTZ NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL,
    updated_at            TIMESTAMPTZ NOT NULL,
    CONSTRAINT staged_uploads_token_hash_key UNIQUE (token_hash),
    CONSTRAINT staged_uploads_tmp_key_key UNIQUE (tmp_key),
    CONSTRAINT staged_uploads_final_key_key UNIQUE (final_key),
    CONSTRAINT staged_uploads_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id),
    CONSTRAINT staged_uploads_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT staged_uploads_uploader_check CHECK (uploader_id > 0),
    CONSTRAINT staged_uploads_visibility_check CHECK (visibility IN ('PUBLIC', 'SECURE')),
    CONSTRAINT staged_uploads_declared_size_check CHECK (declared_size > 0),
    CONSTRAINT staged_uploads_declared_content_type_check CHECK (declared_content_type <> ''),
    CONSTRAINT staged_uploads_declared_md5_check CHECK (declared_md5 IS NULL OR declared_md5 <> ''),
    CONSTRAINT staged_uploads_tmp_key_check CHECK ((visibility = 'PUBLIC') = (tmp_key IS NOT NULL)),
    CONSTRAINT staged_uploads_md5_visibility_check CHECK ((visibility = 'PUBLIC') = (declared_md5 IS NOT NULL)),
    CONSTRAINT staged_uploads_state_check CHECK (
        state IN ('PENDING', 'UPLOADED', 'READY', 'REJECTED', 'CONFIRMED', 'DELETING', 'DELETED')
    ),
    CONSTRAINT staged_uploads_reject_reason_check CHECK (
        reject_reason IS NULL OR reject_reason IN (
            'UNRECOGNIZED_FORMAT', 'SIZE_MISMATCH', 'CHECKSUM_MISMATCH', 'PROCESSING_FAILED', 'EXPIRED'
        )
    ),
    -- 只有被拒絕的暫存列（以及它之後進 GC 的狀態）有原因；REJECTED 一定有。
    CONSTRAINT staged_uploads_reject_state_check CHECK (
        (state <> 'REJECTED' OR reject_reason IS NOT NULL)
        AND (reject_reason IS NULL OR state IN ('REJECTED', 'DELETING', 'DELETED'))
    ),
    CONSTRAINT staged_uploads_final_key_state_check CHECK (
        state NOT IN ('UPLOADED', 'READY', 'CONFIRMED') OR final_key IS NOT NULL
    ),
    CONSTRAINT staged_uploads_asset_state_check CHECK (
        (state NOT IN ('READY', 'CONFIRMED') OR asset_id IS NOT NULL)
        AND (asset_id IS NULL OR state IN ('READY', 'CONFIRMED', 'DELETING', 'DELETED'))
    )
);
-- GC 只掃還沒確認的過期列。
CREATE INDEX staged_uploads_gc ON media.staged_uploads (expires_at) WHERE state IN ('PENDING', 'READY', 'REJECTED');
-- 實體刪除只掃 DELETING。
CREATE INDEX staged_uploads_deleting ON media.staged_uploads (id) WHERE state = 'DELETING';
