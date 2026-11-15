-- moderation 的 view、函式與複合型別（單體規則 6）：改這裡要以 `atlas migrate new <名稱> --env dev` 手寫 migration 再 `atlas migrate hash --env dev`。
-- 函式一律 BEGIN ATOMIC、只參照自己的表。

-- 發布的讀取介面：後台待辦的一列（reviewqueue 讀，形狀在 db/pub.sql）。只放 moderation 的對象鍵：case_id 是案件，target_kind 是被檢舉的對象種類，report_count 只供顯示。
CREATE TYPE moderation.pub_review_item AS (
    kind TEXT,
    item_id TEXT,
    case_id UUID,
    target_kind TEXT,
    report_count BIGINT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

-- 內部：at 這一刻等待決定的案件（A7-D14：一個進行中案件一列，進行中就是 decision IS NULL）。MODERATION_CASE 的期限：案件從開案（首筆檢舉提交）起 72 小時，
-- 開始轉黃在到期前 24 小時（對齊 seller 通報的分案期限；上線前的起始值）。時長只寫在這裡，at 只是呼叫方傳入的時間（不用 now()），期限與 at 無關。
CREATE FUNCTION moderation.review_moderation_case(at TIMESTAMPTZ) RETURNS SETOF moderation.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'MODERATION_CASE'::text, c.id::text, c.id, c.target_kind,
           (SELECT count(*) FROM moderation.reports r WHERE r.case_id = c.id),
           c.opened_at + INTERVAL '72 hours' - INTERVAL '24 hours', c.opened_at + INTERVAL '72 hours'
    FROM moderation.cases c
    WHERE c.decision IS NULL;
END;

-- 發布的讀取介面：at 這一刻營運要處理的 moderation 待辦，沒有排序，消費端排序與分頁。
CREATE FUNCTION moderation.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF moderation.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT * FROM moderation.review_moderation_case(at);
END;
