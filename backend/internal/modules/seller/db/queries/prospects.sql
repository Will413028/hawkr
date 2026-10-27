-- name: CreateProspect :one
-- 建立業者；聯絡方式與分類在同一個 transaction 內另外寫入。
INSERT INTO seller.prospects (
    display_name, vendor_type, outreach_status, tax_id,
    category, region, product_summary, positioning, research_summary, priority_letter, priority_stars,
    created_by_admin_id, updated_by_admin_id, created_at, updated_at
)
VALUES (
    sqlc.arg(display_name), sqlc.arg(vendor_type), sqlc.arg(outreach_status), sqlc.narg(tax_id),
    sqlc.narg(category), sqlc.narg(region), sqlc.narg(product_summary), sqlc.narg(positioning), sqlc.narg(research_summary),
    sqlc.narg(priority_letter), sqlc.arg(priority_stars),
    sqlc.arg(admin_id), sqlc.arg(admin_id), sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: ProspectByID :one
SELECT * FROM seller.prospects WHERE prospect_id = $1;

-- name: ProspectContactForShare :one
-- 主要 email 與狀態，帶共享鎖：與封存、合併的 FOR NO KEY UPDATE 衝突（呼叫方在自己的 transaction 內讀時序列化）。
SELECT p.prospect_id, p.display_name, p.status, e.value AS primary_email
FROM seller.prospects p
LEFT JOIN seller.prospect_contacts e ON e.prospect_id = p.prospect_id AND e.kind = 'EMAIL' AND e.is_primary
WHERE p.prospect_id = $1
FOR SHARE OF p;

-- name: LockProspects :many
-- 依 prospect_id 的順序鎖定，兩個管理員對同一對做反向合併時不會互鎖。FOR NO KEY UPDATE：寫入 prospect_contacts 與 prospect_distinct_pairs 的外鍵檢查
-- 對被指的業者列取 KEY SHARE，FOR UPDATE 會讓兩個互相確認的請求互鎖；任何寫入都不改 prospect_id（沒有別的外鍵目標的唯一鍵），所以不需要 FOR UPDATE。
SELECT * FROM seller.prospects WHERE prospect_id = ANY(sqlc.arg(prospect_ids)::bigint[]) ORDER BY prospect_id FOR NO KEY UPDATE;

-- name: UpdateProspect :one
-- 寫入已在列鎖內合併的欄位；set_logo、set_cover 為 true 時連同那張圖片（key、asset ID、來源網址）一起換，另一張原樣保留；
-- 只有 ACTIVE 的列可以改，版本不符時 0 列。
UPDATE seller.prospects
SET display_name = sqlc.arg(display_name),
    vendor_type = sqlc.arg(vendor_type),
    outreach_status = sqlc.arg(outreach_status),
    tax_id = sqlc.narg(tax_id),
    category = sqlc.narg(category),
    region = sqlc.narg(region),
    product_summary = sqlc.narg(product_summary),
    positioning = sqlc.narg(positioning),
    research_summary = sqlc.narg(research_summary),
    priority_letter = sqlc.narg(priority_letter),
    priority_stars = sqlc.arg(priority_stars),
    logo_key = CASE WHEN sqlc.arg(set_logo)::boolean THEN sqlc.narg(logo_key)::text ELSE logo_key END,
    logo_asset_id = CASE WHEN sqlc.arg(set_logo)::boolean THEN sqlc.narg(logo_asset_id)::text ELSE logo_asset_id END,
    logo_source_url = CASE WHEN sqlc.arg(set_logo)::boolean THEN sqlc.narg(logo_source_url)::text ELSE logo_source_url END,
    cover_key = CASE WHEN sqlc.arg(set_cover)::boolean THEN sqlc.narg(cover_key)::text ELSE cover_key END,
    cover_asset_id = CASE WHEN sqlc.arg(set_cover)::boolean THEN sqlc.narg(cover_asset_id)::text ELSE cover_asset_id END,
    cover_source_url = CASE WHEN sqlc.arg(set_cover)::boolean THEN sqlc.narg(cover_source_url)::text ELSE cover_source_url END,
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id) AND version = sqlc.arg(expected_version) AND status = 'ACTIVE'
RETURNING *;

-- name: ArchiveProspect :one
-- 封存：圖片欄位清成 NULL（引用由呼叫方記成退場、同步 job 解除）；聯絡方式留著，live 讓出統編的唯一性、不再被比對。
UPDATE seller.prospects
SET status = 'ARCHIVED',
    logo_key = NULL, logo_asset_id = NULL, logo_source_url = NULL,
    cover_key = NULL, cover_asset_id = NULL, cover_source_url = NULL,
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id) AND status = 'ACTIVE'
RETURNING *;

-- name: RestoreProspect :one
-- 還原：被合併的列（merged_into 有值）不可還原。統編已被別的業者拿走時，這個 UPDATE 違反唯一性（呼叫方先查，這是並行的保底）。
UPDATE seller.prospects
SET status = 'ACTIVE',
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id) AND status = 'ARCHIVED' AND merged_into IS NULL
RETURNING *;

-- name: ConvertProspect :one
-- 轉成店家：只有 ACTIVE 的列可以轉（二次轉換的防線，0 列）；同一個店家只能被一個業者認領（prospects_converted_store）。
UPDATE seller.prospects
SET status = 'CONVERTED',
    outreach_status = 'CONVERTED',
    converted_store_id = sqlc.arg(store_id),
    converted_at = sqlc.arg(now),
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id) AND status = 'ACTIVE'
RETURNING *;

-- name: ProspectImageAssets :one
-- 同步 job 讀的列：目前那一張 logo、封面的 asset ID；封存的列沒有圖片（CHECK），所以沒有目前的資產。
SELECT prospect_id, logo_asset_id, cover_asset_id FROM seller.prospects WHERE prospect_id = $1;

-- name: ProspectContacts :many
-- 業者的聯絡方式（原值）；同類型主要的在前。
SELECT kind, value, is_primary FROM seller.prospect_contacts WHERE prospect_id = $1 ORDER BY kind, is_primary DESC, value;

-- name: ClearProspectContacts :exec
DELETE FROM seller.prospect_contacts WHERE prospect_id = $1;

-- name: AddProspectContacts :exec
-- 四個陣列逐項對應；match_keys 的空字串是沒有比對鍵（NULL）。每個類型恰好一個主要的由 deferred 外鍵在 commit 時檢查。
INSERT INTO seller.prospect_contacts (prospect_id, kind, value, match_key, is_primary)
SELECT sqlc.arg(prospect_id)::bigint, (sqlc.arg(kinds)::text[])[i], (sqlc.arg(values)::text[])[i],
       NULLIF((sqlc.arg(match_keys)::text[])[i], ''), (sqlc.arg(primaries)::boolean[])[i]
FROM generate_series(1, cardinality(sqlc.arg(kinds)::text[])) AS i;

-- name: UnionProspectContacts :exec
-- 合併：輸家的聯絡方式併入贏家；同一個比對鍵、或同類型同一個值的只留贏家那一列。贏家在這個類型還沒有聯絡方式時，輸家那個類型的
-- 主要的成為主要的；其餘留給 EnsureProspectPrimaries 補齊。
INSERT INTO seller.prospect_contacts (prospect_id, kind, value, match_key, is_primary)
SELECT sqlc.arg(winner_id)::bigint, c.kind, c.value, c.match_key,
       c.is_primary AND NOT EXISTS (SELECT 1 FROM seller.prospect_contacts w WHERE w.prospect_id = sqlc.arg(winner_id)::bigint AND w.kind = c.kind)
FROM seller.prospect_contacts c
WHERE c.prospect_id = sqlc.arg(loser_id)::bigint
  AND NOT EXISTS (
      SELECT 1 FROM seller.prospect_contacts w
      WHERE w.prospect_id = sqlc.arg(winner_id)::bigint
        AND ((c.match_key IS NOT NULL AND w.match_key = c.match_key) OR (w.kind = c.kind AND w.value = c.value)));

-- name: EnsureProspectPrimaries :exec
-- 有聯絡方式卻沒有主要的類型，把值排序最前的那一列補成主要的（合併帶入的類型）。
UPDATE seller.prospect_contacts c
SET is_primary = true
FROM (
    SELECT DISTINCT ON (p.kind) p.kind, p.value
    FROM seller.prospect_contacts p
    WHERE p.prospect_id = $1
      AND NOT EXISTS (SELECT 1 FROM seller.prospect_contacts q WHERE q.prospect_id = $1 AND q.kind = p.kind AND q.is_primary)
    ORDER BY p.kind, p.value
) pick
WHERE c.prospect_id = $1 AND c.kind = pick.kind AND c.value = pick.value;

-- name: SharedKeyHolders :many
-- 這些比對鍵（種類:正規化值）目前被哪些其他業者持有：live 的列（ACTIVE 與 CONVERTED）、而且與 self_id 沒有標記為不是同一家。
-- self_id 為 0 是建立之前的查詢（沒有標記可言）。一列一個（業者, 命中的鍵）。
SELECT DISTINCT c.prospect_id, c.match_key::text AS match_key
FROM seller.prospect_contacts c
JOIN seller.prospects p ON p.prospect_id = c.prospect_id AND p.live
WHERE c.prospect_id <> sqlc.arg(self_id)::bigint
  AND c.match_key = ANY(sqlc.arg(keys)::text[])
  AND NOT EXISTS (
      SELECT 1 FROM seller.prospect_distinct_pairs d
      WHERE d.low_prospect_id = LEAST(c.prospect_id, sqlc.arg(self_id)::bigint) AND d.high_prospect_id = GREATEST(c.prospect_id, sqlc.arg(self_id)::bigint))
ORDER BY c.prospect_id, match_key;

-- name: ConfirmDistinctProspects :exec
-- 管理員確認 prospect_id 與 others 不是同一家：每一對較小的 ID 在前，已標記的略過（保留第一次的管理員與時間）。
INSERT INTO seller.prospect_distinct_pairs (low_prospect_id, high_prospect_id, confirmed_by_admin_id, confirmed_at)
SELECT LEAST(sqlc.arg(prospect_id)::bigint, o.id), GREATEST(sqlc.arg(prospect_id)::bigint, o.id), sqlc.arg(admin_id), sqlc.arg(now)
FROM unnest(sqlc.arg(others)::bigint[]) AS o (id)
ON CONFLICT DO NOTHING;

-- name: MarkDistinctPair :exec
-- 管理員直接標記一對候選不是同一家（ADR 2026-10-04-prospect-contact-points-and-matching 的修訂）：較小的 ID 在前，已標記的略過（保留第一次的管理員與時間）。
INSERT INTO seller.prospect_distinct_pairs (low_prospect_id, high_prospect_id, confirmed_by_admin_id, confirmed_at)
VALUES (LEAST(sqlc.arg(first_id)::bigint, sqlc.arg(second_id)::bigint), GREATEST(sqlc.arg(first_id)::bigint, sqlc.arg(second_id)::bigint), sqlc.arg(admin_id), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: RepointDistinctPairs :exec
-- 合併：輸家的「不是同一家」配對改掛贏家；與贏家自己的那一對（變成自己對自己）不搬，已有的略過。
INSERT INTO seller.prospect_distinct_pairs (low_prospect_id, high_prospect_id, confirmed_by_admin_id, confirmed_at)
SELECT LEAST(sqlc.arg(winner_id)::bigint, m.other_id), GREATEST(sqlc.arg(winner_id)::bigint, m.other_id), m.confirmed_by_admin_id, m.confirmed_at
FROM (
    SELECT CASE WHEN low_prospect_id = sqlc.arg(loser_id)::bigint THEN high_prospect_id ELSE low_prospect_id END AS other_id,
           confirmed_by_admin_id, confirmed_at
    FROM seller.prospect_distinct_pairs
    WHERE low_prospect_id = sqlc.arg(loser_id)::bigint OR high_prospect_id = sqlc.arg(loser_id)::bigint
) m
WHERE m.other_id <> sqlc.arg(winner_id)::bigint
ON CONFLICT DO NOTHING;

-- name: DeleteDistinctPairsOf :exec
-- 合併：輸家的配對（含與贏家的那一對：明確合併一對標記為不是同一家的業者，就是撤銷那個標記）全部刪除。
DELETE FROM seller.prospect_distinct_pairs WHERE low_prospect_id = $1 OR high_prospect_id = $1;

-- name: ProspectCategories :many
SELECT category_id FROM seller.prospect_categories WHERE prospect_id = $1 ORDER BY category_id;

-- name: ClearProspectCategories :exec
DELETE FROM seller.prospect_categories WHERE prospect_id = $1;

-- name: AddProspectCategories :exec
INSERT INTO seller.prospect_categories (prospect_id, category_id)
SELECT sqlc.arg(prospect_id), unnest(sqlc.arg(category_ids)::text[])
ON CONFLICT DO NOTHING;

-- name: CopyProspectCategories :exec
-- 合併：輸家的分類補進贏家（已有的略過）；輸家的分類由呼叫方接著清掉。
INSERT INTO seller.prospect_categories (prospect_id, category_id)
SELECT sqlc.arg(winner_id), c.category_id FROM seller.prospect_categories c WHERE c.prospect_id = sqlc.arg(loser_id)
ON CONFLICT DO NOTHING;

-- name: RepointMergedProspects :exec
-- 路徑壓縮：已被輸家吸收的列改指贏家，merged_into 的鏈永遠只有一層。
UPDATE seller.prospects SET merged_into = sqlc.arg(winner_id) WHERE merged_into = sqlc.arg(loser_id);

-- name: ArchiveMergedLoser :one
UPDATE seller.prospects
SET status = 'ARCHIVED',
    merged_into = sqlc.arg(winner_id),
    tax_id = NULL,
    logo_key = NULL, logo_asset_id = NULL, logo_source_url = NULL,
    cover_key = NULL, cover_asset_id = NULL, cover_source_url = NULL,
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(loser_id) AND status = 'ACTIVE'
RETURNING *;

-- name: CarryProspectTaxID :one
-- 贏家沒有統編時收下輸家的；呼叫方已先清掉輸家的統編（兩列都 live，同一個號碼不能短暫並存）。
UPDATE seller.prospects
SET tax_id = sqlc.arg(tax_id),
    updated_by_admin_id = sqlc.arg(admin_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE prospect_id = sqlc.arg(winner_id) AND tax_id IS NULL
RETURNING *;

-- name: TouchProspect :one
-- 贏家吸收了輸家的鍵與分類：版本加一，讓還開著的編輯畫面衝突、重新讀取。
UPDATE seller.prospects
SET updated_by_admin_id = sqlc.arg(admin_id), updated_at = sqlc.arg(now), version = version + 1
WHERE prospect_id = sqlc.arg(prospect_id)
RETURNING *;

-- name: ListProspects :many
-- admin 清單：status 沒指定時只看 ACTIVE；關鍵字（ILIKE，呼叫方已跳脫 % _ \）找名稱與三段文字；新到舊的 keyset。
SELECT *
FROM seller.prospects
WHERE (CASE WHEN sqlc.narg(status)::text IS NULL THEN status = 'ACTIVE' ELSE status = sqlc.narg(status)::text END)
  AND (sqlc.narg(outreach_status)::text IS NULL OR outreach_status = sqlc.narg(outreach_status)::text)
  AND (sqlc.narg(vendor_type)::text IS NULL OR vendor_type = sqlc.narg(vendor_type)::text)
  AND (sqlc.narg(category)::text IS NULL OR category = sqlc.narg(category)::text)
  AND (sqlc.narg(region)::text IS NULL OR region = sqlc.narg(region)::text)
  AND (sqlc.narg(keyword_pattern)::text IS NULL OR
       display_name ILIKE sqlc.narg(keyword_pattern)::text ESCAPE '\' OR
       product_summary ILIKE sqlc.narg(keyword_pattern)::text ESCAPE '\' OR
       positioning ILIKE sqlc.narg(keyword_pattern)::text ESCAPE '\' OR
       research_summary ILIKE sqlc.narg(keyword_pattern)::text ESCAPE '\')
  AND (
    sqlc.narg(after_updated_at)::timestamptz IS NULL OR
    (updated_at, prospect_id) < (sqlc.narg(after_updated_at)::timestamptz, sqlc.narg(after_id)::bigint)
  )
ORDER BY updated_at DESC, prospect_id DESC
LIMIT sqlc.arg(page_size);

-- name: ProspectsByName :many
-- T3：名稱（忽略大小寫；頭尾空白由 CHECK 排除，參數由呼叫端以 platform/trimtext 去掉）相同的 ACTIVE 業者，只供人判斷，不自動合併。
SELECT * FROM seller.prospects
WHERE status = 'ACTIVE' AND lower(display_name) = lower(sqlc.arg(display_name)::text)
ORDER BY prospect_id
LIMIT 20;

-- name: DuplicateProspectPairs :many
-- 疑似重複的清單（ADR 2026-10-04-prospect-contact-points-and-matching D4）：尚未處理的候選配對，來源是名稱相同（忽略大小寫；頭尾空白由 CHECK 排除）
-- 與比對鍵相同（並行建立漏網的），兩邊都是 ACTIVE，排除已標記不是同一家的。左邊的 ID 較小讓一對只出現一次；以 (左, 右) 的 ID 做 keyset。
WITH reasons AS (
    SELECT a.prospect_id AS left_id, b.prospect_id AS right_id, 'NAME'::text AS reason
    FROM seller.prospects a
    JOIN seller.prospects b ON a.prospect_id < b.prospect_id AND lower(a.display_name) = lower(b.display_name)
    WHERE a.status = 'ACTIVE' AND b.status = 'ACTIVE'
    UNION
    SELECT ca.prospect_id, cb.prospect_id, split_part(ca.match_key, ':', 1)
    FROM seller.prospect_contacts ca
    JOIN seller.prospect_contacts cb ON cb.match_key = ca.match_key AND cb.prospect_id > ca.prospect_id
    JOIN seller.prospects pa ON pa.prospect_id = ca.prospect_id AND pa.status = 'ACTIVE'
    JOIN seller.prospects pb ON pb.prospect_id = cb.prospect_id AND pb.status = 'ACTIVE'
    WHERE ca.match_key IS NOT NULL
), candidates AS (
    -- matched_on：這一對命中的原因（NAME 或比對鍵的種類 EMAIL、PHONE、DOMAIN、SOCIAL，值同 MatchReason），排序固定。
    SELECT left_id, right_id, array_agg(reason ORDER BY reason)::text[] AS matched_on
    FROM reasons
    GROUP BY left_id, right_id
)
SELECT sqlc.embed(a), sqlc.embed(b), c.matched_on
FROM candidates c
JOIN seller.prospects a ON a.prospect_id = c.left_id
JOIN seller.prospects b ON b.prospect_id = c.right_id
WHERE NOT EXISTS (
    SELECT 1 FROM seller.prospect_distinct_pairs d WHERE d.low_prospect_id = c.left_id AND d.high_prospect_id = c.right_id)
  AND (
    sqlc.narg(after_left_id)::bigint IS NULL OR
    (c.left_id, c.right_id) > (sqlc.narg(after_left_id)::bigint, sqlc.narg(after_right_id)::bigint)
  )
ORDER BY c.left_id, c.right_id
LIMIT sqlc.arg(page_size);

-- name: SearchProspects :many
-- 採購候選搜尋（P5）：加權 OR，不是 AND——分類與地區都對、描述沒有字面提到關鍵字的候選不能消失。category、region 與
-- keywords 是 ILIKE 的部分比對（呼叫方已跳脫 % _ \）：匯入來源的產地有 195 種細粒度值，精確比對會漏掉「台南」這種泛稱；
-- 分類樹 ID 走子表。ACTIVE 就是跨 lane 去重（已入駐的業者不該出現在「找平台外的業者」）。LIMIT 只是服務端的安全上限，
-- 不排序：算分與排序是呼叫端的事，用它當「回傳筆數」會變成先對任意幾列算分再排序。
SELECT
    sqlc.embed(p),
    COALESCE(sqlc.narg(category)::text IS NOT NULL AND p.category ILIKE sqlc.narg(category)::text ESCAPE '\', false)::boolean AS hit_category,
    COALESCE(sqlc.narg(region)::text IS NOT NULL AND p.region ILIKE sqlc.narg(region)::text ESCAPE '\', false)::boolean AS hit_region,
    COALESCE(sqlc.narg(keywords)::text IS NOT NULL AND (
        p.product_summary ILIKE sqlc.narg(keywords)::text ESCAPE '\' OR
        p.positioning ILIKE sqlc.narg(keywords)::text ESCAPE '\' OR
        p.research_summary ILIKE sqlc.narg(keywords)::text ESCAPE '\'
    ), false)::boolean AS hit_keywords,
    (cardinality(sqlc.arg(category_ids)::text[]) > 0 AND EXISTS (
        SELECT 1 FROM seller.prospect_categories c
        WHERE c.prospect_id = p.prospect_id AND c.category_id = ANY(sqlc.arg(category_ids)::text[])
    ))::boolean AS hit_category_ids,
    w.value AS primary_website
FROM seller.prospects p
LEFT JOIN seller.prospect_contacts w ON w.prospect_id = p.prospect_id AND w.kind = 'WEBSITE' AND w.is_primary
WHERE p.status = 'ACTIVE'
  AND (
    (sqlc.narg(category)::text IS NULL AND sqlc.narg(region)::text IS NULL AND sqlc.narg(keywords)::text IS NULL AND
     cardinality(sqlc.arg(category_ids)::text[]) = 0)
    OR (sqlc.narg(category)::text IS NOT NULL AND p.category ILIKE sqlc.narg(category)::text ESCAPE '\')
    OR (sqlc.narg(region)::text IS NOT NULL AND p.region ILIKE sqlc.narg(region)::text ESCAPE '\')
    OR (sqlc.narg(keywords)::text IS NOT NULL AND (
        p.product_summary ILIKE sqlc.narg(keywords)::text ESCAPE '\' OR
        p.positioning ILIKE sqlc.narg(keywords)::text ESCAPE '\' OR
        p.research_summary ILIKE sqlc.narg(keywords)::text ESCAPE '\'
    ))
    OR (cardinality(sqlc.arg(category_ids)::text[]) > 0 AND EXISTS (
        SELECT 1 FROM seller.prospect_categories c
        WHERE c.prospect_id = p.prospect_id AND c.category_id = ANY(sqlc.arg(category_ids)::text[])
    ))
  )
LIMIT sqlc.arg(row_limit);

-- name: CurrentStoreExists :one
-- 轉成店家前的存在性驗證：必須是目前的 stint（status 不是 CLOSED）。
SELECT EXISTS (SELECT 1 FROM seller.stores WHERE store_id = $1 AND status <> 'CLOSED');

-- name: ProspectTaxIDMatches :many
-- 統編對帳：業者的統編對得上某個店家的 KYC 申請、但還沒有連起來；已被其他業者認領（converted_store_id）的店家不列。申請以 store_id 為鍵（目前的 stint）；不限申請狀態
-- （草稿與未核准的申請一樣代表「這家已經來了」），把 KYC 狀態一併回給管理員判斷；同一個店家有多次申請取最新一筆。
-- 以 (業者, 店家) 的 ID 做 keyset。
SELECT * FROM (
    SELECT DISTINCT ON (p.prospect_id, s.store_id)
           p.prospect_id, s.store_id, s.store_name, k.company_name, k.status AS kyc_status
    FROM seller.prospects p
    JOIN seller.kyc_applications k ON k.tax_id = p.tax_id
    JOIN seller.stores s ON s.store_id = k.store_id AND s.status <> 'CLOSED'
    WHERE p.tax_id IS NOT NULL AND p.status = 'ACTIVE'
      AND NOT EXISTS (SELECT 1 FROM seller.prospects claimed WHERE claimed.converted_store_id = s.store_id)
    ORDER BY p.prospect_id, s.store_id, k.created_at DESC
) m
WHERE sqlc.narg(after_prospect_id)::bigint IS NULL OR (m.prospect_id, m.store_id) > (sqlc.narg(after_prospect_id)::bigint, sqlc.narg(after_store_id)::bigint)
ORDER BY m.prospect_id, m.store_id
LIMIT sqlc.arg(page_size);

-- name: LiveProspectByTaxID :one
-- 統編目前的持有者與它的狀態（live 的列；T1）。
SELECT prospect_id, status FROM seller.prospects WHERE tax_id = $1 AND live;

-- name: LockProspectTaxID :exec
-- 同一個統編的寫入排隊（交易級 advisory lock，交易結束時釋放）；取得之後的 LiveProspectByTaxID 看得到前一個交易已提交的持有者。
SELECT pg_advisory_xact_lock(hashtextextended('seller.prospect_tax_id:' || sqlc.arg(tax_id)::text, 0));

-- name: ProspectsByIDs :many
SELECT * FROM seller.prospects WHERE prospect_id = ANY(sqlc.arg(prospect_ids)::bigint[]) ORDER BY prospect_id;
