-- payment 發布的讀取介面（pub_*）的形狀，給依賴表允許的上層模組的 sqlc 讀（它們的 sqlc.yaml 列 `../../payment/db/pub.sql`）。
-- 這個檔只宣告形狀，不是定義：定義在 objects.sql 與手寫的 migration，契約測試（cmd/hawkr 的 TestPubContractsMatchMigrations）
-- 比對這裡的每個物件與 migration 套用後的真實物件。只能有 `CREATE SCHEMA payment` 與 `payment.pub_*`（internal/arch 的測試）。
-- 函式宣告成 RETURNS SETOF <pub 表>：sqlc 不型別化複合型別，宣告成表才會生成欄位；函式的 body 只是讓 sqlc 能解析，不是規則。

CREATE SCHEMA payment;

-- 後台待辦的一列：kind 是 UPPER_SNAKE 的種類（PAYMENT_ACCOUNT_CREATE_FAILED），item_id 是同一種類內唯一、位元組順序穩定的鍵（店家 ID 的文字），
-- store_id 是擁有者（店家），開始時間與期限一定有值。真實的 pub_review_item 是複合型別。
CREATE TABLE payment.pub_review_item (
    kind     TEXT        NOT NULL,
    item_id  TEXT        NOT NULL,
    store_id BIGINT      NOT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    due_at   TIMESTAMPTZ NOT NULL
);

-- at 這一刻營運要處理的 payment 待辦（時間由呼叫方以請求的 clock 傳入，不用 now()）；沒有排序，消費端自己排並分頁。
-- 用法：`SELECT * FROM payment.pub_review_items(sqlc.arg(now)::timestamptz) q WHERE q.kind = ANY(...)`。
CREATE FUNCTION payment.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF payment.pub_review_item
LANGUAGE sql STABLE
AS $$ SELECT * FROM payment.pub_review_item $$;
