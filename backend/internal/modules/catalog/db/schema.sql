-- catalog 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（catalog.<表>），不建跨 schema 的 FK；實體（分類、商品、SKU）的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；值的集合（選項值、hashtag、圖片引用）沒有自己的 ID。
-- 時間一律 timestamptz（UTC）；店家用業務鍵 store_id（沒有 FK：它屬於 seller）。金額是新台幣元的整數。

CREATE SCHEMA IF NOT EXISTS public;

-- pg_trgm 給商品搜尋（D15）：searchable_text 的 GIN trigram 索引支援 ILIKE，相關度排序用 word_similarity。擴充裝在 public，不屬於本模組。
-- Atlas 社群版讀到 CREATE EXTENSION 就報錯：改這個檔之後跑 `atlas migrate diff` 時，先暫時拿掉下一行與 products_search_idx，產生 migration 後把這兩者
-- 以手寫補回 migration（product_api_support 是範例），再還原這個檔、`atlas migrate hash`。
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

CREATE SCHEMA catalog;

-- 平台分類樹：兩層（頂層分組與葉節點），由 migration 的版本化種子定義，沒有執行期寫入。商品只能掛葉節點（寫入端以
-- 「沒有子節點」檢查；分類只在 migration 變動，所以沒有並行新增子節點的競爭）。名稱是 LocalizedText（多語 ADR D5）：
-- JSONB 物件，必有非空的 zh-TW，en 缺時顯示退回 zh-TW。shopify_taxonomy_id 是對外對照（Shopify Standard Product Taxonomy），
-- 葉節點必填、頂層為空；升版只更新對照欄，不動內部 ID。
CREATE TABLE catalog.categories (
    id                  UUID    PRIMARY KEY,
    parent_id           UUID    REFERENCES catalog.categories (id),
    name                JSONB   NOT NULL,
    shopify_taxonomy_id TEXT,
    position            INTEGER NOT NULL,
    CONSTRAINT categories_name_check CHECK (
        jsonb_typeof(name) = 'object'
        AND COALESCE(jsonb_typeof(name -> 'zh-TW'), '') = 'string' AND char_length(name ->> 'zh-TW') >= 1
        AND (NOT name ? 'en' OR (COALESCE(jsonb_typeof(name -> 'en'), '') = 'string' AND char_length(name ->> 'en') >= 1))),
    CONSTRAINT categories_position_check CHECK (position >= 0),
    CONSTRAINT categories_taxonomy_shape_check CHECK (parent_id IS NOT NULL OR shopify_taxonomy_id IS NULL),
    CONSTRAINT categories_not_self_parent_check CHECK (parent_id IS DISTINCT FROM id)
);

CREATE UNIQUE INDEX categories_shopify_taxonomy_key ON catalog.categories (shopify_taxonomy_id) WHERE shopify_taxonomy_id IS NOT NULL;
CREATE UNIQUE INDEX categories_sibling_name_key ON catalog.categories (COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), (name ->> 'zh-TW'));
CREATE INDEX categories_parent_idx ON catalog.categories (parent_id, position);

-- 商品：draft／active／archived 由賣家切換，不預審；刪除是另一個動作（deleted_at，列保留給訂單與評價引用）。
-- 可見性還要 AND 店家可曝光（join seller.pub_exposable_stores）與沒有有效限制措施，不寫進這裡。
-- min_price／max_price 是 SKU 價格區間的反正規化，在 SKU 寫入的同 transaction 重算。
-- 銷量、收藏數、評分與則數是反正規化的計數：由事實擁有者在同 transaction 更新或傳絕對值，不用 ±1。
-- description 是結構化區塊的 JSON 陣列，結構與長度由寫入端驗證。searchable_text 由寫入端組（名稱、描述文字、hashtag、
-- 分類英文名、選項軸與值），給 pg_trgm 搜尋（公開搜尋、賣家清單與後台的關鍵字共用）。
-- content_version 是樂觀鎖的版本，對外是 etag（ADR 2026-09-25-optimistic-concurrency-etag D4）：只在賣家可編輯的內容與狀態改變時加一
-- （整體修改、狀態切換）；庫存、銷量、收藏數、評分與價格區間的重算不改它，所以調整庫存或下單不會讓編輯中的表單衝突。
-- low_stock_threshold 是賣家設的低庫存提醒門檻（每個 SKU 庫存大於 0 且小於或等於它就偏低），空表示用平台預設值（程式常數 5）。
-- request_id 與 request_hash 是建立的冪等：同一家店以同一個 request_id 重送只建立一次，request_hash 是請求內容的雜湊，不同內容回錯；
-- 兩者永久保存（跟著商品列），一起有值或一起為空。
CREATE TABLE catalog.products (
    id                 UUID             PRIMARY KEY,
    store_id           BIGINT           NOT NULL,
    category_id        UUID             NOT NULL REFERENCES catalog.categories (id),
    title              TEXT             NOT NULL,
    description        JSONB            NOT NULL DEFAULT '[]',
    status             TEXT             NOT NULL,
    first_published_at TIMESTAMPTZ,
    deleted_at         TIMESTAMPTZ,
    min_price          BIGINT           NOT NULL,
    max_price          BIGINT           NOT NULL,
    sales_count        INTEGER          NOT NULL DEFAULT 0,
    favorite_count     INTEGER          NOT NULL DEFAULT 0,
    rating_average     DOUBLE PRECISION NOT NULL DEFAULT 0,
    review_count       INTEGER          NOT NULL DEFAULT 0,
    searchable_text    TEXT             NOT NULL DEFAULT '',
    content_version    BIGINT           NOT NULL DEFAULT 1,
    low_stock_threshold INTEGER,
    request_id         UUID,
    request_hash       BYTEA,
    created_at         TIMESTAMPTZ      NOT NULL,
    updated_at         TIMESTAMPTZ      NOT NULL,
    CONSTRAINT products_store_check CHECK (store_id > 0),
    CONSTRAINT products_version_check CHECK (content_version >= 1),
    CONSTRAINT products_low_stock_check CHECK (low_stock_threshold IS NULL OR (low_stock_threshold >= 0 AND low_stock_threshold <= 9999)),
    CONSTRAINT products_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT products_title_check CHECK (char_length(title) >= 1 AND char_length(title) <= 100 AND title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT products_description_check CHECK (jsonb_typeof(description) = 'array'),
    CONSTRAINT products_status_check CHECK (status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')),
    CONSTRAINT products_first_published_check CHECK (status <> 'ACTIVE' OR first_published_at IS NOT NULL),
    CONSTRAINT products_price_range_check CHECK (min_price >= 0 AND max_price >= min_price),
    CONSTRAINT products_counts_check CHECK (sales_count >= 0 AND favorite_count >= 0 AND review_count >= 0),
    CONSTRAINT products_rating_check CHECK (rating_average >= 0 AND rating_average <= 5 AND (review_count > 0 OR rating_average = 0))
);

CREATE INDEX products_store_idx ON catalog.products (store_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX products_category_idx ON catalog.products (category_id, id) WHERE deleted_at IS NULL AND status = 'ACTIVE';
CREATE INDEX products_public_new_idx ON catalog.products (first_published_at DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';
CREATE UNIQUE INDEX products_request_key ON catalog.products (store_id, request_id) WHERE request_id IS NOT NULL;
CREATE INDEX products_search_idx ON catalog.products USING gin (searchable_text public.gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX products_public_sales_idx ON catalog.products (sales_count DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';

-- 選項軸：每個商品 0–2 個，position 是軸的順序。軸的名稱與值都是賣家的字，不翻譯。
CREATE TABLE catalog.product_option_axes (
    product_id UUID    NOT NULL REFERENCES catalog.products (id),
    position   INTEGER NOT NULL,
    name       TEXT    NOT NULL,
    PRIMARY KEY (product_id, position),
    CONSTRAINT option_axes_position_check CHECK (position IN (1, 2)),
    CONSTRAINT option_axes_name_check CHECK (char_length(name) >= 1 AND char_length(name) <= 20 AND name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    UNIQUE (product_id, name)
);

-- 軸上的有序值；同一軸內值不重複（主鍵），順序是 position。
CREATE TABLE catalog.product_option_values (
    product_id    UUID    NOT NULL,
    axis_position INTEGER NOT NULL,
    position      INTEGER NOT NULL,
    value         TEXT    NOT NULL,
    PRIMARY KEY (product_id, axis_position, value),
    FOREIGN KEY (product_id, axis_position) REFERENCES catalog.product_option_axes (product_id, position),
    CONSTRAINT option_values_position_check CHECK (position >= 0),
    CONSTRAINT option_values_value_check CHECK (char_length(value) >= 1 AND char_length(value) <= 40 AND value !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    UNIQUE (product_id, axis_position, position) DEFERRABLE INITIALLY DEFERRED
);

-- SKU：選項值的組合；沒有選項軸的商品只有一個 option1 與 option2 都為空的預設 SKU。價格是新台幣元；
-- compare_at_price 是原價，要大於售價。stock_qty 只由 stock 的條件式扣減與歸還改，不為負。刪除是 deleted_at（訂單的快照仍指向它）。
-- 組合是否落在商品的選項值內由寫入端在鎖住商品列的 transaction 內驗證。
CREATE TABLE catalog.product_skus (
    id               UUID        PRIMARY KEY,
    product_id       UUID        NOT NULL REFERENCES catalog.products (id),
    option1_value    TEXT,
    option2_value    TEXT,
    price            BIGINT      NOT NULL,
    compare_at_price BIGINT,
    stock_qty        INTEGER     NOT NULL DEFAULT 0,
    deleted_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL,
    CONSTRAINT skus_price_check CHECK (price > 0),
    CONSTRAINT skus_compare_at_check CHECK (compare_at_price IS NULL OR compare_at_price > price),
    CONSTRAINT skus_stock_check CHECK (stock_qty >= 0),
    CONSTRAINT skus_option_order_check CHECK (option2_value IS NULL OR option1_value IS NOT NULL)
);

CREATE UNIQUE INDEX skus_combination_key ON catalog.product_skus (product_id, COALESCE(option1_value, ''), COALESCE(option2_value, '')) WHERE deleted_at IS NULL;
CREATE INDEX skus_product_idx ON catalog.product_skus (product_id) WHERE deleted_at IS NULL;

-- 庫存操作：一次扣減、歸還、賣家調整或賣家設定是一列。冪等鍵在各自的範圍內唯一：結帳的扣減與歸還沒有店家（store_id 為空），鍵全域唯一；
-- 賣家調整記下店家，鍵只在該店家內唯一，兩家店可以用同一個鍵，也探不到彼此的鍵。同一個範圍內併發或重送同一個鍵只生效一次。
-- 賣家的調整與設定（ADJUST、SET）記下請求內容的雜湊（request_hash），同一個鍵以不同內容重送時比對它；SET 的 movement 是設定當下的差額。
-- 歸還指向它歸還的那次扣減（deduct_operation_id），每個 SKU 的累計歸還量不得超過原扣減量（寫入端在鎖住原扣減列後檢查）。
CREATE TABLE catalog.stock_operations (
    id                  UUID        PRIMARY KEY,
    idempotency_key     TEXT        NOT NULL,
    kind                TEXT        NOT NULL,
    store_id            BIGINT,
    deduct_operation_id UUID        REFERENCES catalog.stock_operations (id),
    request_hash        BYTEA,
    created_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT stock_operations_key_check CHECK (char_length(idempotency_key) >= 1 AND char_length(idempotency_key) <= 200),
    CONSTRAINT stock_operations_kind_check CHECK (kind IN ('DEDUCT', 'RESTORE', 'ADJUST', 'SET')),
    CONSTRAINT stock_operations_restore_ref_check CHECK ((kind = 'RESTORE') = (deduct_operation_id IS NOT NULL)),
    CONSTRAINT stock_operations_store_check CHECK ((kind IN ('ADJUST', 'SET')) = (store_id IS NOT NULL)),
    CONSTRAINT stock_operations_hash_check CHECK ((kind IN ('ADJUST', 'SET')) = (request_hash IS NOT NULL))
);

CREATE UNIQUE INDEX stock_operations_global_key ON catalog.stock_operations (idempotency_key) WHERE store_id IS NULL;
CREATE UNIQUE INDEX stock_operations_store_key ON catalog.stock_operations (store_id, idempotency_key) WHERE store_id IS NOT NULL;
CREATE INDEX stock_operations_deduct_idx ON catalog.stock_operations (deduct_operation_id) WHERE deduct_operation_id IS NOT NULL;

-- 庫存異動明細：delta 是有號的變動量（扣減為負、歸還為正）；帳本只增不改，是庫存稽核的依據。
CREATE TABLE catalog.stock_movements (
    operation_id UUID    NOT NULL REFERENCES catalog.stock_operations (id),
    sku_id       UUID    NOT NULL REFERENCES catalog.product_skus (id),
    delta        INTEGER NOT NULL,
    PRIMARY KEY (operation_id, sku_id),
    CONSTRAINT stock_movements_delta_check CHECK (delta <> 0)
);

CREATE INDEX stock_movements_sku_idx ON catalog.stock_movements (sku_id);

-- 商品與描述的圖片引用：只存在擁有者的表（媒體 ADR 2026-10-03）；儲存時 Confirm、移除時同 transaction MarkDeleted。
-- kind 區分主圖庫（gallery）與描述裡的圖片區塊（description，描述 JSON 以 asset_id 指向這裡）。
-- DB 只存 key（API 回 Image，ADR 2026-09-25-api-image-uri-template）。
CREATE TABLE catalog.product_images (
    product_id UUID    NOT NULL REFERENCES catalog.products (id),
    kind       TEXT    NOT NULL,
    position   INTEGER NOT NULL,
    asset_id   UUID    NOT NULL,
    object_key TEXT    NOT NULL,
    alt        TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (product_id, kind, position),
    CONSTRAINT product_images_kind_check CHECK (kind IN ('GALLERY', 'DESCRIPTION')),
    CONSTRAINT product_images_position_check CHECK (position >= 0),
    CONSTRAINT product_images_alt_check CHECK (char_length(alt) <= 200),
    UNIQUE (product_id, asset_id)
);

-- hashtag是商品上的值集合，沒有實體表：name 是正規化名稱（比對與去重），display_name 是顯示樣子。
-- 熱門與自動完成由查詢聚合；搜尋與自動完成的索引（pg_trgm）隨搜尋的查詢一起加。
CREATE TABLE catalog.product_hashtags (
    product_id   UUID    NOT NULL REFERENCES catalog.products (id),
    name         TEXT    NOT NULL,
    display_name TEXT    NOT NULL,
    position     INTEGER NOT NULL,
    PRIMARY KEY (product_id, name),
    CONSTRAINT product_hashtags_name_check CHECK (char_length(name) >= 1 AND char_length(name) <= 40),
    CONSTRAINT product_hashtags_position_check CHECK (position >= 0)
);

