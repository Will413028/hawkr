-- name: CreateSellerProspect :one
-- facts_as_of（sqlc.narg）只給一次性遷移用：候選卡把 updated_at 呈現成
-- facts_as_of（事實何時成立，不是何時複製），沒有這個覆寫值，634 筆 8 月做的
-- 研究會全部謊報成遷移執行日。NULL（階段二 admin 建檔的一般情況）就落回
-- now()，行為與加這欄之前完全相同。
INSERT INTO seller_prospects (
  id, display_name, vendor_type, status, outreach_status, tax_id,
  match_email, match_phone, match_domain, match_social,
  public_email, public_phone, public_website, public_social,
  category, region, product_summary, positioning, research_summary,
  priority_letter, priority_stars, created_by_account_id, updated_by_account_id,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, $9, $10,
  $11, $12, $13, $14,
  $15, $16, $17, $18, $19,
  $20, $21, $22, $23,
  COALESCE(sqlc.narg(facts_as_of)::timestamptz, now())
) RETURNING *;

-- name: FindSellerProspectByTaxID :one
-- Closing-fix review item 2: uq_seller_prospects_tax_id 已 narrow 成不含
-- ARCHIVED 列，一個統編現在可以同時被一筆 ARCHIVED 與一筆 ACTIVE 列持有
-- （見 seller_prospect_tax_id_index_integration_test.go）。這條查詢是
-- usecase.Match 的 T1，T1 命中即短路、不會再往下看 T2——若這裡撈到的是
-- 已封存的舊列，一筆合併走或封存的舊業者會冒充成比對結果，而真正的
-- ACTIVE 列永遠不會被看到。加上 status <> 'ARCHIVED'，與下面
-- FindSellerProspectsByMatchKeys（T2）的既有述詞一致：CONVERTED 刻意不排除
-- ——一個已入駐的業者仍是「這個統編已經有對應紀錄」的有效比對結果，值得
-- 呈現給操作者知道，不是雜訊。
SELECT * FROM seller_prospects WHERE tax_id = $1 AND status <> 'ARCHIVED';

-- name: FindSellerProspectsByMatchKeys :many
-- T2 比對。每個鍵都必須先「非空」才參與——空字串是佔位值正規化後的樣子，
-- 拿它進等值比對會一次撈回整份名單並把它們全部判成同一家業者。
SELECT * FROM seller_prospects
WHERE status <> 'ARCHIVED'
  AND (
       (sqlc.arg(match_email)::text  <> '' AND match_email  = sqlc.arg(match_email)::text)
    OR (sqlc.arg(match_phone)::text  <> '' AND match_phone  = sqlc.arg(match_phone)::text)
    OR (sqlc.arg(match_domain)::text <> '' AND match_domain = sqlc.arg(match_domain)::text)
    OR (sqlc.arg(match_social)::text <> '' AND match_social = sqlc.arg(match_social)::text)
  );

-- name: SearchSellerProspects :many
-- 加權 OR 而非嚴格 AND：沿用 candidate_lead.sql 的既有語意（該支註解記載
-- 2026-08-07 live 驗證——嚴格 AND 會讓「分類與地區都對、但描述文字沒有字面
-- 提到該關鍵字」的候選整筆消失，且回空時說不出原因）。
--
-- category/region 用 ILIKE 部分比對而非精確比對：CRM1 的 產地 有 195 種細粒度值
-- （台南東山、嘉義竹崎…），精確比對會漏掉「台南」這種泛稱查詢；CRM 與 CRM1 的分類
-- 命名習慣也不一致（CRM 用複合詞如「雞肉／蛋品」，CRM1 用單詞如「水果」）。這條理由
-- 原本記在 ai-service 已刪除的 candidate_lead.sql，隨資料源遷移到這裡——不要為了
-- 「看起來更精確」把這兩個 ILIKE 簡化成 =，那會讓 CRM1 來源的列全部搜不到。
--
-- status = 'ACTIVE' 這個述詞就是跨 lane 去重本身：已入駐（CONVERTED）的業者
-- 出現在「找平台外的業者」本來就是錯的，不是去重問題（spec D4）。
--
-- LIMIT 是服務端安全上限，不是相關性排序的截斷點——這條查詢刻意不排序
-- （沒有 ORDER BY）：算分與排序是呼叫端（ai-service CandidateSearchUseCase.Search）
-- 的責任，交給 DB 排序會撞上 sqlc/PostgreSQL 對輸出別名的 ORDER BY scoping 限制
-- （見該檔同段落註解）。若這裡的 LIMIT 被呼叫端當成「回傳筆數」誤用（例如塞 10），
-- 排序會變成先對任意 10 列算分再排序，而不是對全部命中列排序後取前 10 名——
-- 這正是 C1 修過的那個回歸類別，之所以現在預設 500 不是 10，見 repository 的
-- Search 方法。
SELECT
  seller_prospects.*,
  (sqlc.arg(category)::text != '' AND category ILIKE '%' || sqlc.arg(category)::text || '%') AS hit_category,
  (sqlc.arg(region)::text != '' AND region ILIKE '%' || sqlc.arg(region)::text || '%') AS hit_region,
  (sqlc.arg(keywords)::text != '' AND (
    product_summary ILIKE '%' || sqlc.arg(keywords)::text || '%'
    OR positioning ILIKE '%' || sqlc.arg(keywords)::text || '%'
    OR research_summary ILIKE '%' || sqlc.arg(keywords)::text || '%'
  )) AS hit_keywords,
  (cardinality(sqlc.arg(category_ids)::text[]) > 0 AND EXISTS (
    SELECT 1 FROM seller_prospect_categories spc
    WHERE spc.prospect_id = seller_prospects.id
      AND spc.category_id = ANY(sqlc.arg(category_ids)::text[])
  )) AS hit_category_ids
FROM seller_prospects
WHERE status = 'ACTIVE'
  AND (
       (sqlc.arg(category)::text = '' AND sqlc.arg(region)::text = '' AND sqlc.arg(keywords)::text = ''
        AND cardinality(sqlc.arg(category_ids)::text[]) = 0)
    OR (sqlc.arg(category)::text != '' AND category ILIKE '%' || sqlc.arg(category)::text || '%')
    OR (sqlc.arg(region)::text != '' AND region ILIKE '%' || sqlc.arg(region)::text || '%')
    OR (sqlc.arg(keywords)::text != '' AND (
         product_summary ILIKE '%' || sqlc.arg(keywords)::text || '%'
      OR positioning ILIKE '%' || sqlc.arg(keywords)::text || '%'
      OR research_summary ILIKE '%' || sqlc.arg(keywords)::text || '%'
    ))
    OR (cardinality(sqlc.arg(category_ids)::text[]) > 0 AND EXISTS (
         SELECT 1 FROM seller_prospect_categories spc
         WHERE spc.prospect_id = seller_prospects.id
           AND spc.category_id = ANY(sqlc.arg(category_ids)::text[])
       ))
  )
LIMIT sqlc.arg(row_limit)::int;

-- name: MarkSellerProspectConverted :execrows
-- WHERE status = 'ACTIVE' 是二次轉換與誤歸因的真正防線。
-- uq_seller_prospects_converted_store 只擋「同一個 store 被兩筆 prospect 認領」，
-- 擋不住「同一筆 prospect 的 converted_store_id 被後來的寫入覆蓋掉」。
--
-- 述詞是 = 'ACTIVE' 而不是 <> 'CONVERTED'：後者放行 ARCHIVED，而被合併掉的
-- loser 正是 ARCHIVED ＋ merged_into 非 NULL，UPDATE 會撞上 chk_sp_merged_into
-- 回 23514——對呼叫端是 500，且把 DB 約束名洩漏到 service 邊界外。
--
-- updated_by_account_id 記這次歸因是誰做的：admin 手動綁定傳操作者的登入
-- principal，CreateSeller 的 token 路徑傳 NULL（見
-- internal/usecase/seller_prospect.go markProspectConverted 的呼叫端）——
-- 這一欄不分岔二次轉換防護的單一寫入點，只是多記一個事實。
UPDATE seller_prospects
SET converted_store_id     = $2,
    converted_at           = now(),
    status                 = 'CONVERTED',
    outreach_status        = 'CONVERTED',
    updated_by_account_id  = $3,
    updated_at             = now()
WHERE id = $1 AND status = 'ACTIVE';

-- name: FindUsableProspectInviteByToken :one
-- 「可用」＝未撤銷、未消費、未過期。三個條件都寫在 SQL 裡，不交給呼叫端
-- 各自判斷——這是 token 的有效性定義，只能有一份。
SELECT * FROM seller_prospect_invites
WHERE token = $1
  AND revoked_at IS NULL
  AND consumed_at IS NULL
  AND expires_at > now();

-- name: ConsumeProspectInvite :execrows
UPDATE seller_prospect_invites
SET consumed_at = now()
WHERE id = $1 AND consumed_at IS NULL;

-- name: ListSellerProspectsForAdmin :many
-- status 空字串 ⇒ 只回 ACTIVE。keyword 用 ILIKE 而非全文檢索：634 筆量級下
-- 順序掃描比維護一個 tsvector 欄位便宜，且 BD 的關鍵字是片段而非詞彙
-- （既有 SearchSellerProspects 的同一個判斷，見 6d47c97ca）。
SELECT *, COUNT(*) OVER() AS total_count
FROM seller_prospects
WHERE (CASE WHEN @status::text = '' THEN status = 'ACTIVE' ELSE status = @status::text END)
  AND (@outreach_status::text = '' OR outreach_status = @outreach_status::text)
  AND (@vendor_type::text     = '' OR vendor_type     = @vendor_type::text)
  AND (@category::text        = '' OR category        = @category::text)
  AND (@region::text          = '' OR region          = @region::text)
  AND (@keyword::text         = '' OR
       display_name     ILIKE '%' || @keyword::text || '%' OR
       product_summary  ILIKE '%' || @keyword::text || '%' OR
       positioning      ILIKE '%' || @keyword::text || '%' OR
       research_summary ILIKE '%' || @keyword::text || '%')
ORDER BY updated_at DESC, id DESC
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: CountSellerProspectsForAdmin :one
-- ListSellerProspectsForAdmin 用 COUNT(*) OVER() 把總筆數跟本頁結果綁在
-- 一起：翻到超出範圍的頁碼時 LIMIT／OFFSET 讓查詢回零列，窗口函式沒有
-- 任何一列可附著，total 因此變成 0——前端的 pager 會把它讀成「其實沒有
-- 資料」而收合，即使真正符合條件的筆數大於 0。這裡用同一份 WHERE 條件
-- （刻意逐字重複，不抽共用片段：sqlc 對兩份查詢各自產生獨立的 Params，
-- 硬抽共用巨集只會讓兩者的欄位對應更難核對）單獨查一次真正的相符筆數，
-- 只在 ListForAdmin 看到零列時才呼叫，不影響有資料的一般路徑多付一趟
-- 往返。
SELECT COUNT(*) FROM seller_prospects
WHERE (CASE WHEN @status::text = '' THEN status = 'ACTIVE' ELSE status = @status::text END)
  AND (@outreach_status::text = '' OR outreach_status = @outreach_status::text)
  AND (@vendor_type::text     = '' OR vendor_type     = @vendor_type::text)
  AND (@category::text        = '' OR category        = @category::text)
  AND (@region::text          = '' OR region          = @region::text)
  AND (@keyword::text         = '' OR
       display_name     ILIKE '%' || @keyword::text || '%' OR
       product_summary  ILIKE '%' || @keyword::text || '%' OR
       positioning      ILIKE '%' || @keyword::text || '%' OR
       research_summary ILIKE '%' || @keyword::text || '%');

-- name: GetSellerProspectByID :one
SELECT * FROM seller_prospects WHERE id = $1;

-- name: UpdateSellerProspect :one
-- match_* 一律由呼叫端以 BuildProspectMatchKeys 算好帶進來——與 Create 同一條
-- 規則。改了聯絡方式卻沿用舊 match keys，等於讓比對鍵與顯示值長期分岔。
UPDATE seller_prospects
SET display_name          = $2,
    vendor_type           = $3,
    outreach_status       = $4,
    tax_id                = $5,
    match_email           = $6,
    match_phone           = $7,
    match_domain          = $8,
    match_social          = $9,
    public_email          = $10,
    public_phone          = $11,
    public_website        = $12,
    public_social         = $13,
    category              = $14,
    region                = $15,
    product_summary       = $16,
    positioning           = $17,
    research_summary      = $18,
    priority_letter       = $19,
    priority_stars        = $20,
    updated_by_account_id = $21,
    updated_at            = now()
WHERE id = $1 AND status = 'ACTIVE'
RETURNING *;

-- name: SetSellerProspectArchived :execrows
-- 還原（archived = false）刻意加上 merged_into_prospect_id IS NULL：被合併的
-- 列要還原就必須同時撤銷那次合併（含把邀請搬回去），本輪不提供反向操作。
-- 封存（archived = true）只允許從 ACTIVE 出發——CONVERTED 由 chk_sp_converted
-- 擋著，但那會是 500，所以這裡先擋成 rows = 0。
UPDATE seller_prospects
SET status                = CASE WHEN @archived::boolean THEN 'ARCHIVED' ELSE 'ACTIVE' END,
    updated_by_account_id = @operator_account_id,
    updated_at            = now()
WHERE id = @id::bigint
  AND (
    (@archived::boolean       AND status = 'ACTIVE')
    OR (NOT @archived::boolean AND status = 'ARCHIVED' AND merged_into_prospect_id IS NULL)
  );

-- name: ListProspectInvites :many
SELECT * FROM seller_prospect_invites
WHERE prospect_id = $1
ORDER BY created_at DESC;

-- name: CreateProspectInvite :one
INSERT INTO seller_prospect_invites (id, prospect_id, token, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RevokeProspectInvite :execrows
-- 同時比對 prospect_id：只憑 invite_id 會讓 admin 從 A 的詳情頁撤銷掉 B 的邀請。
-- 已消費的不再撤銷——那筆歸因已經發生，撤銷它只會讓歷史說謊。
UPDATE seller_prospect_invites
SET revoked_at = now()
WHERE id = $2 AND prospect_id = $1 AND revoked_at IS NULL AND consumed_at IS NULL;

-- name: ListDuplicateProspectPairs :many
-- 每個 OR 分支的守衛是這條查詢的存亡關鍵：541 筆「待確認」email 正規化後
-- 就是空字串，少了守衛會得到 O(n²) 的假配對爆炸——與遷移時佔位值黑名單
-- 同一個地雷，在 SQL 層再踩一次。守衛本身（Go 的文件註解格式化會把行內
-- 相鄰的兩個單引號吃成一個排版引號，所以獨立成縮排區塊）：
--
--	<> ''
--
-- T1（tax_id）刻意不列入：uq_seller_prospects_tax_id 已結構性禁止 tax_id
-- 重複，掃出來恆為空。列進去只會讓讀的人以為那是可能發生的事。
--
-- a.id < b.id 讓一對只出現一次，順帶排除自我配對。
-- LIMIT 501：多取一筆用來判斷是否截斷（呼叫端只回前 500 對）。
SELECT
    sqlc.embed(a),
    sqlc.embed(b)
FROM seller_prospects a
JOIN seller_prospects b ON a.id < b.id AND (
     (a.match_email  <> '' AND a.match_email  = b.match_email)
  OR (a.match_phone  <> '' AND a.match_phone  = b.match_phone)
  OR (a.match_domain <> '' AND a.match_domain = b.match_domain)
  OR (a.match_social <> '' AND a.match_social = b.match_social))
WHERE a.status = 'ACTIVE' AND b.status = 'ACTIVE'
ORDER BY a.id, b.id
LIMIT 501;

-- name: LockSellerProspectsForMerge :many
-- ORDER BY id 讓兩個 admin 對同一對做反向合併時不會死鎖。
SELECT * FROM seller_prospects WHERE id = ANY(@ids::bigint[]) ORDER BY id FOR UPDATE;

-- name: MoveProspectInvitesToWinner :execrows
UPDATE seller_prospect_invites SET prospect_id = @winner_id::bigint
WHERE prospect_id = @loser_id::bigint;

-- name: RepointMergedProspects :execrows
-- 路徑壓縮：loser 底下已掛的被合併列一併改指 winner，讓 merged_into 鏈
-- 永遠只有一層。A→B→C 的鏈會讓「這家業者是哪一列」變成要走圖。
UPDATE seller_prospects SET merged_into_prospect_id = @winner_id::bigint
WHERE merged_into_prospect_id = @loser_id::bigint;

-- name: ArchiveLoserProspect :execrows
UPDATE seller_prospects
SET status                  = 'ARCHIVED',
    merged_into_prospect_id = @winner_id::bigint,
    updated_by_account_id   = @operator_account_id,
    updated_at              = now()
WHERE id = @loser_id::bigint AND status = 'ACTIVE';

-- name: ClearLoserProspectTaxID :execrows
-- tax_id 是「winner 欄位不合併」規則的唯一例外（Merge 的 repository 文件
-- 註解、spec §5.2）。這條只管 loser 那一半：無論它的統編有沒有被搬去
-- winner，loser 合併之後都不再是業者，繼續讓一筆非業者的列佔著統編
-- 只會製造日後無法復原的狀態（封存列扣著號碼 → 新業者合法拿走 → 這筆
-- 舊列既無法還原、統編也已經無處可清）。呼叫端只在 loser 原本非 NULL
-- 時才呼叫這條，但即使命中 0 列也不是錯誤——單純沒有統編可清。
UPDATE seller_prospects SET tax_id = NULL
WHERE id = @loser_id::bigint;

-- name: CarryProspectTaxIDToWinner :execrows
-- 只在呼叫端已確認 winner 現在是 NULL 時才會被呼叫；WHERE 的
-- tax_id IS NULL 是第二道防線，不是判斷依據——真正的「winner 有沒有
-- 統編」判斷在 Go 那一層用同一次上鎖讀到的列做，這裡重複一次只是不讓
-- 這條 UPDATE 在呼叫端邏輯出錯時意外覆寫 winner 已有的統編。
--
-- 呼叫順序是硬性的：必須先執行 ClearLoserProspectTaxID 再執行這一條。
-- 合併當下兩列都還是 ACTIVE，uq_seller_prospects_tax_id 兩者都管——先寫
-- 這條會讓 winner 跟還沒清空的 loser 在同一個號碼上短暫並存，直接撞
-- 23505。
--
-- updated_at／updated_by_account_id 一併寫：這張表其他每一條寫入路徑
-- （Create／Update／SetArchived／MarkConverted／ArchiveLoserProspect）
-- 都會記這兩欄，統編被合併搬進 winner 不該是唯一的例外——少了它，
-- admin console 看不出這筆統編是誰在什麼時候透過合併放進來的。
UPDATE seller_prospects
SET tax_id                = @tax_id::text,
    updated_by_account_id = @operator_account_id,
    updated_at            = now()
WHERE id = @winner_id::bigint AND tax_id IS NULL;

-- name: CurrentStoreExists :one
-- 歸因前的存在性驗證。uq_seller_prospects_converted_store 只擋重複認領，
-- 擋不住「指向一個不存在的 store」——admin 打錯一個字就會產生那種列。
SELECT EXISTS (
  SELECT 1 FROM sellers
  WHERE store_id = $1 AND is_current = true AND deleted_at IS NULL
);

-- name: ListProspectTaxIDMatches :many
-- 階段一 spec §5.2 的第三條歸因路徑：statutory id 對得上、但從來沒有人把
-- 這兩列連起來。
--
-- btrim 不是防禦性寫法：certification_applications.tax_id 是 varchar(20)
-- 且沒有任何格式約束（spec §11 的「統編存在兩處」落差），
-- seller_prospects.tax_id 有 ^[0-9]{8}$。不 trim 會漏掉尾隨空白的列。
--
-- certification_applications 只有 account_id 沒有 store_id，故必須 join
-- sellers 才能取得歸因所需的 store stint。
-- 不限 ca.status——未核准的申請一樣代表「這家已經來了」——但把 status 回給
-- admin 判斷。DISTINCT ON 處理同帳號多次申請，取最新一筆。
SELECT DISTINCT ON (p.id, s.store_id)
       sqlc.embed(p),
       s.store_id,
       s.store_name,
       ca.company_name,
       ca.status AS certification_status
FROM seller_prospects p
JOIN certification_applications ca ON btrim(ca.tax_id) = p.tax_id
JOIN sellers s ON s.account_id = ca.account_id AND s.is_current = true AND s.deleted_at IS NULL
WHERE p.tax_id IS NOT NULL AND p.status = 'ACTIVE'
ORDER BY p.id, s.store_id, ca.created_at DESC;

-- name: UpdateSellerProspectMedia :one
-- 換一張品牌圖片。kind 只有 logo 與 cover 兩種，未命中的那一組欄位原樣保留——
-- 換 logo 不可以把 cover 的 key 或 source_url 洗掉。
--
-- media_version 是圖片專屬的樂觀鎖，與文字主檔的 updated_at 分開：Admin 的文字表單
-- 刻意不送媒體欄位，共用一個版本會讓兩邊的並行編輯互相撞成 409。
--
-- WHERE 同時擋 status 與 merged_into_prospect_id：被合併或已轉入駐的列只供檢視。
-- 版本不符時回 0 列（:one 於是回 ErrNoRows），由呼叫端翻成衝突而不是「找不到」。
UPDATE seller_prospects
SET logo_key         = CASE WHEN @kind::text = 'logo'  THEN @object_key::text ELSE logo_key END,
    cover_key        = CASE WHEN @kind::text = 'cover' THEN @object_key::text ELSE cover_key END,
    logo_source_url  = CASE WHEN @kind::text = 'logo'  THEN @source_url::text ELSE logo_source_url END,
    cover_source_url = CASE WHEN @kind::text = 'cover' THEN @source_url::text ELSE cover_source_url END,
    media_version         = media_version + 1,
    updated_by_account_id = @operator_account_id,
    updated_at            = now()
WHERE id = @id::bigint
  AND status = 'ACTIVE'
  AND merged_into_prospect_id IS NULL
  AND media_version = @expected_media_version::bigint
RETURNING *;
