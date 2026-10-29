-- seller 發布的讀取介面（pub_*）的形狀，給依賴表允許的上層模組的 sqlc 讀（它們的 sqlc.yaml 列 `../../seller/db/pub.sql`）。
-- 這個檔只宣告形狀，不是定義：定義在 objects.sql 與手寫的 migration，契約測試（cmd/hawkr 的 TestPubContractsMatchMigrations）
-- 比對這裡的每個物件與 migration 套用後的真實物件（欄位、型別、參數與回傳，雙向）。只能有 `CREATE SCHEMA seller` 與 `seller.pub_*`
-- （internal/arch 的測試）。
--
-- 函式宣告成 RETURNS SETOF <pub 表>：sqlc 不型別化 RETURNS TABLE 與複合型別，宣告成表才會檢查與生成欄位。真實的 pub_exposable_store
-- 是複合型別；store_id 來自 stores 的主鍵，一定有值，所以這裡宣告 NOT NULL。函式的 body 只是讓 sqlc 能解析，不是規則。
-- 消費端的 query 只能呼叫函式，不得直接參照 seller.pub_exposable_store（真實物件不是表，執行期才失敗；internal/arch 的測試擋）。

CREATE SCHEMA seller;

CREATE TABLE seller.pub_exposable_store (
    store_id BIGINT NOT NULL
);

-- at 這一刻可以公開曝光的店家（時間由呼叫方以請求的 clock 傳入，不用 now()）：營運中、沒有正在隨帳號刪除、帳號在 at 沒有生效中的停權
-- 或終止合作、KYC 與 PayUni 都核准、店名、logo 與鄉鎮市區代碼齊全。「至少一個上架商品」不在這裡，由曝光端檢查。
-- 用法：`EXISTS (SELECT 1 FROM seller.pub_exposable_stores(sqlc.arg(now)::timestamptz) e WHERE e.store_id = p.store_id)`。
CREATE FUNCTION seller.pub_exposable_stores(at TIMESTAMPTZ) RETURNS SETOF seller.pub_exposable_store
LANGUAGE sql STABLE
AS $$ SELECT * FROM seller.pub_exposable_store $$;

-- 後台待辦的一列（step10e-3d-design.md §2.1）。真實的 pub_review_item 是複合型別，宣告成表只為了讓 sqlc 型別化欄位。每一列是一件待辦：
-- kind 是 UPPER_SNAKE 的種類（REPORT_TRIAGE、CASE_DECISION、CASE_INQUIRY_OVERDUE、AUTHORITY_ORDER、AUTHORITY_TARGET_NOTICE、
-- MEASURE_APPEAL、MEASURE_LIFT_REQUEST、MEASURE_REVIEW、KYC_APPLICATION、PAYUNI_BIND_RETRY）；item_id 是同一種類內唯一、位元組順序穩定的鍵
-- （UUID 的 ::text；來函的對象是 order_id::text || '/' || lpad(position::text, 5, '0')）；其餘是對象的鍵（依種類只有相關的有值，其餘 NULL）、
-- 店家（沒有單一店家時 NULL，店名已清理時 store_name 是空字串）、開始時間與期限（10 種都有期限，E3）。kind、item_id、start_at、due_at 一定有值。
CREATE TABLE seller.pub_review_item (
    kind TEXT NOT NULL,
    item_id TEXT NOT NULL,
    report_id UUID,
    case_id UUID,
    inquiry_id UUID,
    authority_order_id UUID,
    target_position SMALLINT,
    account_id BIGINT,
    measure_id UUID,
    request_id UUID,
    kyc_application_id UUID,
    store_id BIGINT,
    store_name TEXT,
    start_at TIMESTAMPTZ NOT NULL,
    due_at TIMESTAMPTZ NOT NULL
);

-- at 這一刻營運要處理的全部待辦（seller 的 10 種；時間由呼叫方以請求的 clock 傳入，不用 now()）。成員、開始與期限的規則在 seller 的
-- review_<種類>(at)，這裡只是它們的 UNION ALL。沒有排序，消費端自己排並分頁。
-- 用法：`SELECT * FROM seller.pub_review_items(sqlc.arg(now)::timestamptz) q WHERE q.kind = ANY(...) ORDER BY q.due_at, q.start_at, q.kind COLLATE "C", q.item_id COLLATE "C"`。
CREATE FUNCTION seller.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF seller.pub_review_item
LANGUAGE sql STABLE
AS $$ SELECT * FROM seller.pub_review_item $$;

-- 此刻生效中的內容下架措施的對象，一列是一個（措施，內容）；真實的 pub_content_takedown 是複合型別，宣告成表只為了讓 sqlc 型別化欄位。
-- 內容下架建立即生效、沒有結束時間，所以不收 at。給 social 每日對帳核對自己的複本（雙向：漏的補、多的刪）；social 的讀取不 join 它。
-- 用法：`SELECT measure_id, content_kind, content_id FROM seller.pub_content_takedowns()`。
CREATE TABLE seller.pub_content_takedown (
    measure_id UUID NOT NULL,
    content_kind TEXT NOT NULL,
    content_id UUID NOT NULL
);

CREATE FUNCTION seller.pub_content_takedowns() RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql STABLE
AS $$ SELECT * FROM seller.pub_content_takedown $$;
