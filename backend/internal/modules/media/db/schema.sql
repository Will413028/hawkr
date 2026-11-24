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
-- state 只表示物理生命週期、只由 media 自己的 transaction 改（B1-D25）：STAGED：物件在、尚未開始刪除；
-- DELETING：media 的 job 已標記刪除、物件尚未刪掉；DELETED：物件已刪（保留列作為冪等的墓碑）。
-- 「已確認（ACTIVE）」與「擁有者已請求刪除」不寫在這一列，由 upload_claims 與 asset_deletion_requests 推導（internal/model.DeriveAssetState），
-- 因為擁有者的 transaction 內只能新增列、不得鎖 media 既有的列（單體規則 4）。
CREATE TABLE media.assets (
    id           UUID        PRIMARY KEY,
    visibility   TEXT        NOT NULL,
    object_key   TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL,
    state        TEXT        NOT NULL,
    -- 機敏資產的信封加密：包裹 DEK 的 KEK 代號與 wrapped DEK（nonce＋密文＋tag，AAD 是資產 ID 加 kek_id）；公開資產沒有。
    -- 機敏資產的 object_key 是前綴 secure/<資產 ID>/（不是單一物件），各段的 key 在 media.secure_chunks。
    kek_id       TEXT,
    wrapped_dek  BYTEA,
    -- 機敏資產的密文是 chunk_count 份獨立的 STREAM 密文（media.secure_chunks 逐段記錄 key）；公開資產沒有。
    chunk_count  INTEGER,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT assets_object_key_key UNIQUE (object_key),
    CONSTRAINT assets_visibility_check CHECK (visibility IN ('PUBLIC', 'SECURE')),
    CONSTRAINT assets_state_check CHECK (state IN ('STAGED', 'DELETING', 'DELETED')),
    CONSTRAINT assets_content_type_check CHECK (content_type <> ''),
    CONSTRAINT assets_size_check CHECK (size_bytes >= 0),
    CONSTRAINT assets_kek_id_check CHECK ((visibility = 'SECURE') = (kek_id IS NOT NULL)),
    CONSTRAINT assets_wrapped_dek_check CHECK ((visibility = 'SECURE') = (wrapped_dek IS NOT NULL)),
    CONSTRAINT assets_chunk_count_check CHECK ((visibility = 'SECURE') = (chunk_count IS NOT NULL) AND (chunk_count IS NULL OR chunk_count > 0)),
    CONSTRAINT assets_object_key_prefix_check CHECK (
        (visibility = 'PUBLIC' AND object_key LIKE 'public/%') OR (visibility = 'SECURE' AND object_key LIKE 'secure/%')
    )
);
-- 實體刪除 job 只掃 DELETING。
CREATE INDEX assets_deleting ON media.assets (id) WHERE state = 'DELETING';

-- 暫存上傳：token 與暫存列一對一（B1-D2），只存 token 的雜湊，不存明文。
-- 狀態機（每個轉換都是條件式 UPDATE，WHERE state=<預期>）：
--   PENDING → UPLOADED → READY | REJECTED → CONFIRMED
--   secure 的分段上傳：UPLOADING（逐段推進 next_index）→ READY（最後一段，同一個 transaction 建資產列）| REJECTED（第 0 段辨認不出）→ CONFIRMED，見 internal/upload/secure_upload.go
--   PENDING、UPLOADING、READY、REJECTED 且過期，或 UPLOADED 且過期超過寬限（處理 job 卡住）→ DELETING → DELETED（GC，先贏得 RECLAIMED 認領）
--   CONFIRMED 是 media 的 settle 步驟事後物化的值（權威來源是 upload_claims 的 CONFIRMED 認領）：擁有者確認時不更新這一列。
-- final_key 在 PENDING → UPLOADED 時產生並記下，處理 job 重試寫同一個 key；asset_id 在 READY 時才有（READY 之前資產列不存在）。
CREATE TABLE media.staged_uploads (
    id                    UUID        PRIMARY KEY,
    token_hash            BYTEA       NOT NULL,
    uploader_id           BIGINT      NOT NULL,
    uploader_admin        BOOLEAN     NOT NULL,
    -- 代行營運開的會話（B1-D16）綁定被代行的店家：只有同一家店的代行確認或續傳；非代行是 NULL。
    delegated_store_id    BIGINT,
    visibility            TEXT        NOT NULL,
    declared_size         BIGINT      NOT NULL,
    -- public：宣告的型別；secure：第 0 段依 magic bytes 辨認後才有，之前是 NULL（不用空字串或假值佔位）。
    declared_content_type TEXT,
    -- 宣告的內容 MD5，標準 base64（預簽 PUT 的 Content-MD5）；只有 public 走預簽，secure 是 NULL。
    declared_md5          TEXT,
    state                 TEXT        NOT NULL,
    reject_reason         TEXT,
    -- 預簽直傳的暫存 key；secure 經單體分段上傳，沒有暫存物件，是 NULL。
    tmp_key               TEXT,
    final_key             TEXT,
    asset_id              UUID,
    -- secure 的分段上傳：預配的資產 ID（最後一段推進時才建資產列，所以不是 FK）、段總數、下一個要收的段序號、
    -- 包好的 DEK 與 kek_id（資產列建立時從這裡搬過去）；public 全是 NULL。
    planned_asset_id      UUID,
    chunk_count           INTEGER,
    next_index            INTEGER,
    kek_id                TEXT,
    wrapped_dek           BYTEA,
    expires_at            TIMESTAMPTZ NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL,
    updated_at            TIMESTAMPTZ NOT NULL,
    CONSTRAINT staged_uploads_token_hash_key UNIQUE (token_hash),
    CONSTRAINT staged_uploads_tmp_key_key UNIQUE (tmp_key),
    CONSTRAINT staged_uploads_final_key_key UNIQUE (final_key),
    CONSTRAINT staged_uploads_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id),
    CONSTRAINT staged_uploads_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT staged_uploads_uploader_check CHECK (uploader_id > 0),
    CONSTRAINT staged_uploads_delegated_store_check CHECK (delegated_store_id IS NULL OR (delegated_store_id > 0 AND uploader_admin)),
    CONSTRAINT staged_uploads_visibility_check CHECK (visibility IN ('PUBLIC', 'SECURE')),
    CONSTRAINT staged_uploads_declared_size_check CHECK (declared_size > 0),
    CONSTRAINT staged_uploads_declared_content_type_check CHECK (
        (declared_content_type IS NULL OR declared_content_type <> '')
        AND (visibility = 'SECURE' OR declared_content_type IS NOT NULL)
        AND (state NOT IN ('READY', 'CONFIRMED') OR declared_content_type IS NOT NULL)
    ),
    CONSTRAINT staged_uploads_planned_asset_id_key UNIQUE (planned_asset_id),
    CONSTRAINT staged_uploads_chunks_check CHECK (
        (visibility = 'SECURE') = (planned_asset_id IS NOT NULL)
        AND (visibility = 'SECURE') = (chunk_count IS NOT NULL)
        AND (visibility = 'SECURE') = (next_index IS NOT NULL)
        AND (visibility = 'SECURE') = (kek_id IS NOT NULL)
        AND (visibility = 'SECURE') = (wrapped_dek IS NOT NULL)
        AND (chunk_count IS NULL OR chunk_count > 0)
        AND (next_index IS NULL OR (next_index >= 0 AND next_index <= chunk_count))
    ),
    CONSTRAINT staged_uploads_ready_chunks_check CHECK (
        visibility <> 'SECURE' OR state NOT IN ('READY', 'CONFIRMED') OR next_index = chunk_count
    ),
    CONSTRAINT staged_uploads_declared_md5_check CHECK (declared_md5 IS NULL OR declared_md5 <> ''),
    CONSTRAINT staged_uploads_tmp_key_check CHECK ((visibility = 'PUBLIC') = (tmp_key IS NOT NULL)),
    CONSTRAINT staged_uploads_md5_visibility_check CHECK ((visibility = 'PUBLIC') = (declared_md5 IS NOT NULL)),
    CONSTRAINT staged_uploads_state_check CHECK (
        state IN ('PENDING', 'UPLOADING', 'UPLOADED', 'READY', 'REJECTED', 'CONFIRMED', 'DELETING', 'DELETED')
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
-- GC 只掃還沒確認的過期列（settle 步驟把已確認的 READY 物化成 CONFIRMED，離開這個索引）。
CREATE INDEX staged_uploads_gc ON media.staged_uploads (expires_at) WHERE state IN ('PENDING', 'UPLOADING', 'READY', 'REJECTED');
-- 實體刪除只掃 DELETING。
CREATE INDEX staged_uploads_deleting ON media.staged_uploads (id) WHERE state = 'DELETING';
-- 金鑰輪替只掃還在上傳中的機敏暫存列。
CREATE INDEX staged_uploads_uploading_rewrap ON media.staged_uploads (id) WHERE state = 'UPLOADING';

-- 機敏資產的各段：每段是推進成功那次請求寫的物件（B1-D17）。asset_id 是預配的資產 ID，上傳中資產列還不存在，所以不設 FK；
-- 每次嘗試寫新 key（secure/<資產 ID>/<段序號>/<嘗試 uuid>），輸給並行請求的嘗試沒有列，它的物件由輸家自己刪、或由 GC 依前綴清掉。
CREATE TABLE media.secure_chunks (
    asset_id   UUID    NOT NULL,
    idx        INTEGER NOT NULL,
    object_key TEXT    NOT NULL,
    PRIMARY KEY (asset_id, idx),
    CONSTRAINT secure_chunks_object_key_key UNIQUE (object_key),
    CONSTRAINT secure_chunks_idx_check CHECK (idx >= 0),
    CONSTRAINT secure_chunks_object_key_check CHECK (object_key LIKE 'secure/%')
);

-- token 的認領：Confirm 與 GC 搶同一個唯一鍵（upload_id），先 INSERT 的一方擁有這筆暫存（B1-D25）。
-- 擁有者的 transaction 只 INSERT 這張表；media 自己只在實體刪除完成時才刪（與暫存或資產列同生命週期，沒有獨立保留期）。
CREATE TABLE media.upload_claims (
    upload_id   UUID        PRIMARY KEY,
    -- CONFIRMED：擁有者確認；RECLAIMED：GC 回收。
    kind        TEXT        NOT NULL,
    -- CONFIRMED 一定有；RECLAIMED 在暫存還沒有資產時是 NULL。
    asset_id    UUID,
    -- 寫入這筆認領的 transaction（DEFAULT 保證是它自己）：Confirm 重入只在同一個 transaction 內冪等（B1-D23）。
    claimed_xid XID8        NOT NULL DEFAULT pg_current_xact_id(),
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT upload_claims_upload_fkey FOREIGN KEY (upload_id) REFERENCES media.staged_uploads (id),
    CONSTRAINT upload_claims_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id),
    CONSTRAINT upload_claims_asset_key UNIQUE (asset_id),
    CONSTRAINT upload_claims_kind_check CHECK (kind IN ('CONFIRMED', 'RECLAIMED')),
    CONSTRAINT upload_claims_asset_check CHECK (kind <> 'CONFIRMED' OR asset_id IS NOT NULL)
);

-- 擁有者的刪除請求：一個資產最多一筆（ON CONFLICT DO NOTHING 冪等）；applied_at 只由 media 的套用 job 寫。
-- 資產物理刪除完成時，media 一併刪掉這筆。
CREATE TABLE media.asset_deletion_requests (
    asset_id     UUID        PRIMARY KEY,
    requested_at TIMESTAMPTZ NOT NULL,
    applied_at   TIMESTAMPTZ,
    CONSTRAINT asset_deletion_requests_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id)
);
-- 套用 job 只掃還沒套用的請求。
CREATE INDEX asset_deletion_requests_pending ON media.asset_deletion_requests (requested_at) WHERE applied_at IS NULL;
