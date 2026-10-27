-- category_price_rollup：新增 price_layer／unit，並把 status 擴為三態。
--
-- 為什麼不能讓 Atlas 產出的單一 ALTER 直接上：
--   atlas migrate diff 會把 ADD COLUMN 與 ADD CONSTRAINT 併進同一個 ALTER TABLE。
--   PostgreSQL 會在欄位加完之後才驗 CHECK，而既有列全部是 status='ready' 且新欄位
--   為 NULL，layer_presence_check 當場整批違反，migration 直接失敗。
--   所以必須拆成三步：先加欄位、回填既有列、最後才加約束。
--
-- 回填值的依據：
--   這張表在本次擴充之前，資料只可能來自農業部果菜花卉批發行情
--   （price_ingest.go 只打 /AgriProductsTransType/，TcType 僅 N04/N05/N06），
--   因此所有既有 ready 列的層級與單位必然是 wholesale_auction／kg。
--   這不是猜測，是當時唯一存在的組合。
--
-- 為什麼回填而不是讓讀取端把 NULL 當預設：
--   讀取端確實也要容忍 NULL（contract 階段之前），但把既有列補成明確值可以讓
--   layer_presence_check 從第一天就成立，不必等 contract。兩者不衝突。
--
-- 冪等：ADD COLUMN IF NOT EXISTS 與帶條件的 UPDATE 都可重跑；約束以 DROP ... IF
--   EXISTS 前置，避免重放時撞名。

-- 1. 加欄位
ALTER TABLE "category_price_rollup"
    ADD COLUMN IF NOT EXISTS "price_layer" text NULL,
    ADD COLUMN IF NOT EXISTS "unit"        text NULL;

-- 2. 回填既有列（見上方依據）。只動 ready 列；unavailable 列沒有彙總值，
--    兩欄維持 NULL 才是正確狀態。
UPDATE "category_price_rollup"
SET price_layer = 'wholesale_auction',
    unit        = 'kg'
WHERE status = 'ready'
  AND (price_layer IS NULL OR unit IS NULL);

-- 3. 加／換約束
ALTER TABLE "category_price_rollup"
    DROP CONSTRAINT IF EXISTS "category_price_rollup_status_check",
    DROP CONSTRAINT IF EXISTS "category_price_rollup_layer_presence_check",
    DROP CONSTRAINT IF EXISTS "category_price_rollup_price_layer_check";

ALTER TABLE "category_price_rollup"
    ADD CONSTRAINT "category_price_rollup_status_check"
        CHECK (status = ANY (ARRAY['ready'::text, 'mixed_layer'::text, 'unavailable'::text])),
    ADD CONSTRAINT "category_price_rollup_price_layer_check"
        CHECK ((price_layer IS NULL) OR (price_layer = ANY (ARRAY['wholesale_auction'::text, 'farmgate'::text, 'live_animal'::text, 'retail'::text, 'wholesale_bulk'::text]))),
    ADD CONSTRAINT "category_price_rollup_layer_presence_check"
        CHECK ((status <> 'ready'::text) OR ((price_layer IS NOT NULL) AND (unit IS NOT NULL)));
