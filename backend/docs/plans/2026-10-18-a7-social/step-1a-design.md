# A7-1a social 模組本體：實作設計 v3

- 對象：計畫 `../2026-10-18-a7-social.md` 的步驟 A7-1a。計畫的決定、預設與不變量是上位規則，本檔只把它們落成模組的形狀。
- 快照：main `109edd425`（2026-10-23）。
- 標記：【量】＝打開檔案或跑過指令；【推】＝讀程式後推論。舊行為的出處以 `domain-inventory.md` 的 `file:line` 為準。
- 路徑縮寫：`M=backend/internal`、`S=backend/services/social-service`、`SI=S/internal`。
- v2（2026-10-23）：開工的 design-review（21 項）處理後的版本，處理對照在 §10。
- v3（2026-10-23）：計數照 `counts-brief.md` v4 定案（Will 2026-10-23：照業界基準；頂層留言不可見時留佔位符、回覆照常顯示）。

## 0. 前提題：這一步的程式之後會不會被重寫、搬走或刪除？

不會。這是新模組的終態本體；之後的步驟只往上加（A7-1b 封鎖過濾、A7-1c 內容下架過濾、A7-1d 隱藏與警告的呼叫方、A7-1f 排程、A7-2 Connect）。唯一的過渡物是對 Java catalog 的商品驗證（`M/legacy/productservice`，到 B4）。

## 0.1 範圍

- 新模組 `M/modules/social`（`make new-module name=social from=social-service`，骨架照 content）。
- buyer 新增 `db/pub.sql`、`db/objects.sql` 與一個 `pub_*`（§2.2），以及進入刪除中時的 `BuyerDeletionRequested` 事件（§6.2）。
- `cmd/hawkr` 子命令 `social-recount`（§3.5）。
- `M/platform/textfilter`（新，§3.6）。
- `M/platform/revalidate` 加貼文的 tag（§6.4）。
- `M/platform/policy` 與 proto `PolicyCategory` 加社群的 SPAM、HARASSMENT、HATE（從 A7-1c 移到這一步，§4）。
- 依賴表：social 拿掉 im、review（`M/arch/modules.go`）。
- ADR：取代 `2026-09-18-mq-social-counter-async-delta`（§3.5）。
- `CARRYOVER.md`、`AGENTS.md`、特性測試骨架與 28 條情境、pg_trgm spike、效能量測。

## 0.2 不在範圍

- 封鎖（A7-1b）、內容下架的 join（A7-1c）、moderation（A7-1d）、排程（A7-1f）、Connect 與 handler（A7-2）、relay（A7-3）。
- 本步只建 social 的根套件 API 與 integration 測試；沒有對外入口，舊 social-service 照常服務。

## 0.3 決策總表（都已由計畫或規則決定，這裡只寫落點）

| # | 題 | 落點 | 出處 |
|---|---|---|---|
| E1 | ID | social 擁有的實體 UUIDv7；讚表複合自然鍵；`actor_id`、`store_id`、`buyer_id` bigint | 計畫「預設」；ADR `2026-09-25-row-identity-by-data-kind`、`2026-10-17-social-actor-identity` |
| E2 | 公開可讀 | 一個 SQL 述詞，所有公開讀取共用（§2.3） | 計畫不變量 |
| E3 | 計數 | 全部存在獨立的 stats 表；單一「是否計入」SQL 定義；trigger 同 transaction 加減；外部事實改變排重算；每日對帳與手動重算指令；hashtag 數量讀取時算 | `counts-brief.md` §4；A7-D11 |
| E4 | 瀏覽與分享 | `RecordView`、`RecordShare`，去重表；匿名用每日 salt 的 HMAC | A7-D11、Q4；計畫 A7-1a |
| E5 | 處分 | social 擁有買家的社群處分紀錄與隱藏紀錄 | A7-D9、10e R12 |
| E6 | 已刪帳與退場 | 讀取時經 `pub_*` 擋住，訂閱者只排清除 job | P2；單體規則 4 |
| E7 | 媒體 | 照 content 的 `media.Ref`／`SyncMedia` | 計畫「預設」；`M/modules/content` |
| E8 | 分頁 | AIP-158 keyset，不回總數 | 計畫「預設」；`M/platform/pagination` |
| E9 | 快取失效 | 寫入同 transaction 排 `revalidate` job | 計畫 A7-1a；`domain-inventory.md` M26 |

## 0.4 查證過的現況【量】

- `seller.pub_exposable_stores(at)` 在 `M/modules/seller/db/pub.sql:19`；`pub.sql` 的 arch 與契約測試已在（`M/arch/sqlc_test.go`、`cmd/hawkr` 的 `TestPubContractsMatchMigrations`）。
- buyer 沒有 `pub.sql`、`objects.sql`（`ls M/modules/buyer/db` → `atlas.hcl migrations queries schema.sql sqlc.yaml`）。`buyer.buyers.status ∈ {ACTIVE, PENDING_DELETION, CLOSED}`，`deleted_at` 與 CLOSED 同進退（`buyer/db/schema.sql:12-29`）；`PENDING_DELETION` 是「等待未結訂單完成的買家身分」（`buyer/internal/model/model.go:20`）。
- 事件 `buyer.BuyerDeleted`（`buyer/events.go:25`）、`seller.StoreRetired`（`seller/events.go:31`）存在。
- seller 的能力 API：`ActingStore(ctx, accountID, cap)`（`seller/seller.go:513`）、`CurrentStore`（`:436`）、`CapabilitiesOf(store)`（`:398`）；`CapMarketing`、`CapContentContract`（`:411-412`）。
- `M/platform` 沒有 `textfilter`；詞典在 `backend/pkg/moderation/dictionary.go`（14 個詞，子字串比對）。
- `M/platform/revalidate` 只有 `StoreTags`、`RecipeTag`（`revalidate.go:38-45`）。
- `M/legacy/productservice` 有 `Cards(ctx, ids, includeInactive)`（`productservice.go:41`）。
- httpapi 的 client IP 經 `rpc.WithClientIP` 放進 ctx（`M/httpapi/subject.go:61-78`）；website 的 SSR 請求帶 `X-HK-Origin-Auth`（`subject.go:21-36`）。

## 1. 資料模型（`social/db/schema.sql`，Atlas）

全部表在 `social` schema。文字欄的頭尾空白 CHECK 用 `trimtext.Check`，登記在 `cmd/hawkr` 的 `trimmedColumns`（單體規則 9）。長度上限、圖片數、標籤數照舊 social 的驗證（實作者第一步讀出每條規則的 `SI/...:line`，列在 `CARRYOVER.md`，§8）。

| 表 | 鍵 | 欄位重點 | 約束 |
|---|---|---|---|
| `posts` | `id uuid` | `store_id bigint`、`body text`、`image_asset_id uuid NULL`、`product_id bigint NULL`、`status text`（`DRAFT`／`PUBLISHED`／`UNPUBLISHED`）、`published_at`、`version bigint`、`created_at`、`updated_at`、`deleted_at` | status CHECK；`status = 'PUBLISHED' ⇒ published_at IS NOT NULL`（重新發布時 `published_at` 的語意照舊 social，實作者讀出出處寫進 `CARRYOVER.md`）；公開要件（有圖、有內文）在 Go 驗證；文字長度上限同時寫成 `char_length` CHECK |
| `comments` | `id uuid` | `post_id uuid FK`、`parent_id uuid NULL`、`reply_to_id uuid NULL`、`actor_kind text`、`actor_id bigint`、`body text`、`created_at`、`updated_at`、`deleted_at` | `actor_kind IN ('BUYER','STORE')`；同篇由 DB 保證：`UNIQUE (post_id, id)`，`(post_id, parent_id)`、`(post_id, reply_to_id)` 複合 FK 指回它；兩層（父必須是頂層）由 Go 保證；`char_length` CHECK；索引照 keyset 的排序鍵：頂層 `(post_id, created_at, id) WHERE parent_id IS NULL AND deleted_at IS NULL`、回覆 `(parent_id, created_at, id) WHERE deleted_at IS NULL`，排序方向由 A7-2 的列表定，§9.3 的 EXPLAIN 驗證 |
| `comment_likes` | `(comment_id, actor_kind, actor_id)` | `created_at` | FK comments |
| `post_likes` | `(post_id, buyer_id)` | `created_at` | FK posts；不存 kind（M2） |
| `forum_entries` | `id uuid` | `store_id bigint`、`parent_id`、`reply_to_id`、`actor_kind`、`actor_id`、`body`、時間欄、`deleted_at` | 同 `comments`（容器是 `store_id`：`UNIQUE (store_id, id)` 與複合 FK）；不和 `comments` 合成一張（容器不同、留言有讚，合併要「容器恰好一個」的 CHECK 與兩套讀取，§8 記） |
| `post_hashtags` | `(post_id, position)` | `name text`（正規化：NFC、ASCII 小寫）、`display_name text` | 值的集合（ADR `2026-09-25-row-identity-by-data-kind` D1：隨貼文整批讀寫、不被以 ID 指稱），整批替換；`UNIQUE (post_id, name)`；索引 `(name, post_id)`；不建 `hashtags` 實體表、不存 `post_count`（SD16） |
| `hides` | `id uuid` | `comment_id uuid NULL FK comments`、`forum_entry_id uuid NULL FK forum_entries`、`reason_category text`（`PolicyCategory`）、`reason_text`、`source_ref text`（不透明，moderation 的案件）、`admin_id bigint`、`hidden_at`、`restored_at`、`restored_by`、`restore_reason` | `num_nonnulls(comment_id, forum_entry_id) = 1`；部分唯一索引：同一則內容未還原的隱藏最多一筆（`WHERE restored_at IS NULL`，兩欄各一個）；還原留列不刪（稽核），之後可以再隱藏。只收買家寫的內容（`Hide` 檢查作者是 BUYER）：貼文與店家身分的內容走 seller 的內容下架措施（計畫不變量「措施是店家處分效果的唯一來源」） |
| `sanctions` | `id uuid` | `buyer_id bigint`、`kind text`（初始 `WARNING`）、`reason_category`、`reason_text`、`source_ref`、`admin_id`、`created_at`、`ends_at NULL` | kind CHECK；之後加限制種類只加值與規則（Q2 (iii) 延後） |
| `post_stats` | `post_id` FK | `like_count`、`comment_count`（含回覆）、`view_count`、`share_count` | 全部 `>= 0`；建立貼文時同 transaction 建列 |
| `comment_stats` | `comment_id` FK | `like_count`、`reply_count` | `>= 0` |
| `forum_entry_stats` | `forum_entry_id` FK | `reply_count` | `>= 0` |
| `post_views` | `(post_id, view_day date, viewer_key bytea)` | — | 主鍵即去重；`view_day` 是台北日 |
| `view_salts` | `view_day date` | `salt bytea` | 每日一列，跨實例共用 |
| `post_shares` | `(post_id, buyer_id)` | `created_at` | 主鍵即去重 |

- 隱藏從留言、討論區的欄位改成 `hides` 表：理由、來源、還原與稽核在同一處，可以多次隱藏與還原（取代舊的 `hidden_at`／`hidden_reason` 欄，Q6）。公開述詞以 `NOT EXISTS (… hides … restored_at IS NULL)` 判定。
- 留言與討論區不存作者名稱、頭像快照（M19、Q9）；讀取時由 buyer、seller 的 API 批次補。
- 舊的排程欄（`schedule_*`、`scheduled_by_session_id`）不建，A7-1f 再加（它的形狀由 A7-D6 決定）。

## 2. 跨模組讀取（`pub_*`）

### 2.1 social 的 `sqlc.yaml`

`schema: ["schema.sql", "objects.sql", "../../seller/db/pub.sql", "../../buyer/db/pub.sql"]`。arch 測試會擋清單裡不在依賴表的模組（social 依賴 buyer、seller，已在表內）。

### 2.2 buyer 發布 `buyer.pub_inactive_buyers()`

- 形狀（`buyer/db/pub.sql`）：`buyer.pub_inactive_buyer (buyer_id bigint NOT NULL)`，函式 `RETURNS SETOF buyer.pub_inactive_buyer`。
- 定義（`buyer/db/objects.sql` 與手寫 migration，`BEGIN ATOMIC`、`STABLE`）：`status <> 'ACTIVE'` 的買家，也就是刪除中與已關閉（預設，可推翻：和 seller 的 `not_pending_deletion` 一樣，申請刪除起內容就不公開；P2 只要求刪帳後立即不可見）。
- 消費端只呼叫函式。**用途（Will 2026-10-23，D-sync）**：social 的讀取與 `reevaluate_*` 不呼叫它（它們只讀 social 自己的 `inactive_authors`，見 §3.5）；只有 social 每日對帳的一支 sqlc 查詢（`MissingInactiveAuthors`）用它核對複本是否漏了。
- buyer 的 `AGENTS.md` 加「發布的讀取介面」段（照 seller 的格式）；backend `AGENTS.md` 規則 2 的「現有的 `pub_*`」清單同步。

### 2.3 公開可讀述詞

一則內容公開可讀，若且唯若：

1. 貼文：`deleted_at IS NULL`、`status = 'PUBLISHED'`、店家在 `seller.pub_exposable_stores($now)` 裡（A7-1c 加：不在 `seller.pub_content_takedowns($now)` 裡）。
2. 留言：所屬貼文公開可讀；自己 `deleted_at IS NULL`、沒有未還原的 `hides`；作者是買家時不在 `social.inactive_authors` 裡（買家刪帳的複本，§3.5；A7-1c 加：不在內容下架的複本裡）。這三項合成旗標 `counted`，讀取只看旗標。回覆不跟著頂層留言（Will 2026-10-23）：頂層留言不可見而底下有可見的回覆時，列表回一個佔位項目（只有 ID、建立時間、狀態「已刪除」或「已隱藏」，沒有內文與作者），回覆照常顯示；沒有可見回覆時整則不出現。佔位符的文字由共用文案目錄提供（狀態是 enum，規則 E），在 A7-4 的 UI 稿核可。
3. 討論區：店家可曝光；自己 `deleted_at IS NULL`、沒有未還原的 `hides`；買家作者未停用；回覆與佔位項目的規則同留言。
4. hashtag 計數與趨勢只算公開可讀的貼文；趨勢以 `published_at` 分窗（M6）。

- 述詞寫在 Go（Will 2026-10-23，取代先前「view 加每支 query 固定 join」的做法）：`social/internal/visibility` 是「公開可讀」「可見」的入口（「計入」是旗標，定義在資料庫的 `reevaluate_*`，§3.5），讀取的 query（24 支加 `CommentIsHidden`、`ForumEntryIsHidden`）用 go-jet 在上面組成，在 `db.Conn(ctx)` 上執行（`social/internal/reads`）。原因：DB 物件不能參照別的模組的 `pub_*`（migration 單獨套用在空的 database 上要成功），所以 view 版只能把規則切成「view 加每支 query 自己 join」，靠數次數的測試保證不漏；sqlc 沒有片段可以組合。業界做法與基準見 `step-1a-builder-baseline.md`，決策與重新評估條件見 ADR `2026-09-27-module-internal-structure.md` 的修訂。已刪除 view `published_posts`、`visible_comments`、`visible_forum_entries` 與函式 `comment_is_hidden`、`comment_counted`。
- 不能漏的保證改由守門測試給：入口型別（公開表只經 `visibility` 取得，AST 測試）、登記表加 golden SQL 快照（改規則看得到每支 query 的 SQL 差異）、渲染出來的 SQL 的邊界檢查（pg_query）、對測試資料庫的 Describe 形狀測試、資料集與獨立 SQL 的對照。A7-1b（封鎖）、A7-1c（內容下架）改 `visibility` 的述詞（`Viewable` 加封鎖；下架進 `reevaluate_*` 的輸入），golden 檔的差異就是影響範圍。
- 容器的規則（貼文公開可讀、店家可曝光）對父鍵只檢查一次（頂層留言列表一個 `EXISTS`，不是每列一次）；固定代碼（`PUBLISHED`、`BUYER`）是 SQL 常數，不是 bind 參數（generic plan 無法用 `$1` 證明 partial index 的述詞）。
- `$now` 一律是請求的 clock（`twtime` 注入），不用 `now()`。
- 作者本人與店主的讀取另走「自己的內容」查詢：作者看得到被隱藏的自己的內容與理由（A7-D9），店主看得到不公開貼文下的留言（C8）；這兩條不經公開述詞，但仍不含已刪除的列。

## 3. 寫入規則

### 3.1 身分與能力

- 互動者 (kind, id) 由伺服器依資源歸屬推導：呼叫者此刻的店擁有該貼文或討論區 → `STORE`，否則 `BUYER`（ADR 10-17 S2；舊實作 `SI/usecase/actor.go:39-57`）。
- 店家身分的寫入只有一個核心入口，收已經解析好的店家（`seller.Store`），在 social 內以 `seller.CapabilitiesOf(store)` 檢查能力：建立、編輯、發布貼文與店家身分留言要 `CapMarketing`；下架、刪除自己的內容要 `CapContentContract`（A7-D3）。帳號路徑由 `seller.CurrentStore` 取得店家，代行路徑（A7-2d）由 delegation 驗證 session 後給 store_id、social 讀店家；兩條路徑走同一個能力判斷，停權的店代行也寫不了（不另開繞過能力的入口）。能力宣告表的「依資源判斷」豁免照停權 ADR 的修訂（A7-2 加修訂段，本步在 `CARRYOVER.md` 記下每個方法用哪個能力）。
- 買家身分寫入要求買家是 ACTIVE（經 buyer 公開 API）。

### 3.2 貼文

- 狀態：`DRAFT → PUBLISHED → UNPUBLISHED → PUBLISHED`；刪除是軟刪。公開要件（恰好一張圖、內文必填）在轉為 `PUBLISHED` 時驗證（舊 `SI/usecase/post.go:111-125`）。
- 寫入一律帶 `version` CAS（etag 由 `version` 推導，A7-2 用）。
- `product_id`：寫入時驗證存在、上架中且屬於該店（SD24）；讀取時不重驗。social 定義 port（`ProductVerifier`），`cmd/hawkr` 注入 `M/legacy/productservice` 的實作（模組不得 import legacy，`M/arch`）；它是跨行程呼叫，在開 transaction 之前呼叫（單體規則 5 的精神），B4 catalog 搬進單體後換成 in-process。
- `PostPublished` 事件在轉為公開時發（互動式；排程在 A7-1f），建立草稿不發（SD8）。

### 3.3 留言與討論區

- 兩層：回覆的父一律壓成頂層（舊 `comment.go:178-186`）；`reply_to` 只接受同篇、未刪、公開可讀的目標（SD4）。
- 編輯只限作者；刪除限作者或貼文所屬店（舊 `comment.go:294-329`）；被隱藏的內容作者不能編輯（SD7）。
- 刪除冪等：`UPDATE … WHERE deleted_at IS NULL`，0 列不做副作用（SD5）。
- 推播預覽以 rune 截斷（SD9）；預覽給 A7-3 的 relay，放在事件裡。

### 3.4 讚

- `INSERT … ON CONFLICT DO NOTHING`、取消讚 `DELETE`；讚數由 trigger 依實際新增或刪除的列加減（SD17：並發雙擊只有一列，只加一）。
- 只對公開可讀的內容按讚。

### 3.5 計數（`counts-brief.md` §4，Will 2026-10-23）

- 存在 `post_stats`、`comment_stats`、`forum_entry_stats`，不放在實體列上（Mastodon 型），按讚、瀏覽不和貼文的 `version` CAS 搶鎖。
- **計入的單一定義**（Will 2026-10-23；同日 D-sync 修訂，主 session 依 Will 的選擇改為此形）：`objects.sql` 的 `reevaluate_comments`、`reevaluate_forum_entries`，`LANGUAGE sql BEGIN ATOMIC`，輸入只有 social 自己的表：自己沒有被刪除（`deleted_at`）、沒有未還原的 `hides`、作者是買家時不在 `social.inactive_authors`（A7-1c 加內容下架的複本表，同樣的形狀）。貼文與店家的狀態不影響計數。讚與分享的集合是存在的列。Go 沒有第二份述詞：`visibility.Counted` 只是旗標 `counted`。不再有生成器、`internal/counted`、`migrate.Module.Generated` 與規則 6 的第二個例外。
- **複本表 `social.inactive_authors(buyer_id PK, since)`**：buyer 擁有「買家不再營運中」的事實；social 的同 transaction 訂閱者（`BuyerDeletionRequested` 與 `BuyerDeleted`，直接從營運中關閉的路徑只發後者）在買家刪帳的 transaction 內插入（`ON CONFLICT DO NOTHING`，買家不會回到營運中所以只增），並讓該作者的內容集合式重新求值（`stats.Service.MarkAuthorInactive`：鎖內容列、再以一次集合呼叫 reevaluate，函式自己依序鎖整批 stats，不經 `stats.Service.Lock`）。資料庫與 API 讀取的可見性與計數在申請提交的同一刻改好，沒有 job 的延遲（P2、SD13 不變）；網站快取頁由同 transaction 排入、立即可跑的清除 job 失效，晚秒級，在 P2「秒到分鐘級」內，快取 TTL 保底。必要條件：資料庫端維護的旗標依賴別的模組擁有的事實，而函式不能參照別的模組；單體規則 4 與 2 同步寫明。
- **每則內容存旗標 `counted`**（`comments`、`forum_entries`）：此刻是否計入，貼文的留言數與父項的回覆數是旗標的加總。reevaluate 對一批內容鎖內容列、貼文 stats、父項 stats（固定順序，各自依 ID）之後求值（body 的每個語句各取新 snapshot，先鎖再求值成立），與旗標不同才改旗標並加減計數；冪等。
- **trigger**（`objects.sql`，`RETURNS trigger`）：`comments`、`forum_entries` 的建立與 `deleted_at` 改變、`hides` 的新增與還原讓目標內容 reevaluate；`comment_likes`、`post_likes`、`post_shares` 的新增與刪除加減對應的 stats；同一個 transaction。trigger 是計數唯一的維護路徑，寫進 social 的 `AGENTS.md`。
- **重算**：`reads` 的三支重算語句（go-jet）把計數覆寫成旗標的加總（stats = 旗標之和，旗標只由 reevaluate 改）；先 `SELECT … FOR UPDATE` 鎖 stats 列、再計數覆寫。
- **外部事實改變時，訂閱者在同一個 transaction 寫複本並重新求值**：買家刪帳（§6.2）、內容下架（A7-1c：`MeasureCreated` 插入、`MeasureRevoked` 刪除複本，同樣重新求值）。
- **每日對帳**：River 週期 job 先核對複本與 `buyer.pub_inactive_buyers`（補漏，記為漂移 `AuthorsAdded`），再讓全部內容重新求值（修旗標），再對全部可重算的計數跑重算、覆寫，記下旗標修正與漂移的數量與對象；不為 0 時 warn log。瀏覽數不能重算，不在範圍。
- **手動全量重算**：`cmd/hawkr` 子命令 `social-recount`；改了計數定義的 migration 在同一步跑一次。
- 新 ADR 取代 `2026-09-18-mq-social-counter-async-delta`：照實寫舊前提（熱門內容的鎖競爭）仍可能成立、目前沒有量測；依基準（Lemmy、Mastodon 存起來加 trigger 或寫入時加減，Discourse 定期對帳）同 transaction 維護；觸發：負載測試量到單一 stats 列持續鎖等待 → 分片計數列或緩衝批次寫入（GitLab 型）。

### 3.6 `platform/textfilter`

- 新 platform 套件，詞典與比對規則照 `backend/pkg/moderation/dictionary.go`（14 詞、子字串比對）搬一份；`pkg/moderation` 到 A6 才刪（im、review 還在用）。
- 所有寫入路徑（建立與編輯貼文、留言、討論區）呼叫同一個入口（SD6）。
- 「從零設計」：詞庫應有擁有者與後台維護（M17），這一步只搬現行為；後台維護的觸發寫進 `CARRYOVER.md`（觸發：營運要求新增詞）。
- 子字串比對已知會誤擋正常文案（`pkg/moderation/dictionary.go:10-14` 記載三個詞因此移除），而這一步把它套到編輯與討論區，範圍更大。`CARRYOVER.md` 記：下一次要加詞時，先把比對改成 token-boundary 或更精準的比對。

### 3.7 hashtag

- client 明確送標籤；正規化與驗證照舊（`SI/domain/hashtag/normalizer.go:20-32`、`validator.go:20-81`：最多 10 個、每個 ≤ 40 字、`a-z0-9` 加漢字）。
- 寫入是整批替換一篇貼文的 `post_hashtags`（值的集合），沒有共用的標籤列要鎖，舊的「照 id 升冪」鎖順序（`hashtag.go:128-147`，為了更新 `post_count`）不再需要。
- 建議與趨勢從 `post_hashtags` join 公開述詞算（`SELECT name, count(*) … GROUP BY name`）。前綴搜尋用 `LIKE name || '%'`，查詢前驗證字元集（SD19）；「包含」搜尋看 spike（§9.4）。

### 3.8 媒體

- 照 content：`media/ref_type.go` 加 social 的 ref type，換圖在 transaction 內 `SyncMedia`；舊引用由 media 的 job 釋放（E7）。
- 軟刪：保留。舊表的理由（跨服務時 media 無法列舉引用，`S/db/schema/schema.sql:71`）在單體不成立；今天的必要條件是保留期還沒訂（計畫「延後」的軟刪保留期與清理 job，觸發：隱私權政策訂出保留期），所以內容與它的媒體引用先留著，清理 job 在那時一起釋放。店家退場的清除 job 照樣釋放（§6.2）。`CARRYOVER.md` 的 M15 列寫這個新理由與觸發。

## 4. 處分與隱藏（A7-D9）

- `Hide(ctx, HideInput{Target, Category, Text, SourceRef, AdminID})`：寫 `hides`；同一則已有未還原的隱藏時回既有的（冪等）；排快取失效。
- `Restore(ctx, RestoreInput{Target, AdminID, Reason})`：管理員更正（SD13），手動原因；寫 `restored_*`；排快取失效。
- `Warn(ctx, WarnInput{BuyerID, Category, Text, SourceRef, AdminID})`：寫 `sanctions`（`WARNING`，`ends_at` NULL）。
- 讀：`MySanctions(buyerID)`、`MyHiddenContent(actor)`（作者自己的內容與理由）；給 A7-2 的 Connect。
- 這些都是根套件的方法，會被 moderation 在它的 transaction 內呼叫：只用 `db.Conn(ctx)`，不用 `InReadSnapshot`；撞唯一鍵用 `ON CONFLICT`，不在同一個 transaction 繼續下 SQL（單體規則 3）。
- 理由的類別是 `PolicyCategory` 的 `Assessable` 子集（平台判定可用的類別，`policy.go:59-60`），不另定子集；買家條款寫好時再收窄（計畫「延後」）。社群的 SPAM、HARASSMENT、HATE 在這一步加進 proto 與 `platform/policy`（`policy.go` 的套件說明已預告由 A7 加），A7-1c 只做 seller 那邊的子集與 CHECK。`hides`、`sanctions` 的 CHECK 登記在模組的 `TestChecksMatchGo`。`make uitext` 補三個值的文字（zh-TW、en）。

## 5. 瀏覽與分享（A7-D11、Q4）

- `RecordView(ctx, RecordViewInput{PostID, Viewer})`，`Viewer` 是「已登入的帳號」或「匿名的 IP 與 UA」二選一；店主與代行的讀取由呼叫方不呼叫（A7-2 的 handler 判斷）。
- 只計公開可讀的貼文；`(post_id, view_day, viewer_key)` 插入成功才 `post_stats.view_count + 1`（只增的事件計數，不進對帳）。
- `viewer_key`：登入者 `HMAC-SHA256(當日 salt, "a:" ‖ account_id)`；匿名 `HMAC-SHA256(當日 salt, "v:" ‖ post_id ‖ ip ‖ ua)`。兩者都用 salt，表裡就沒有可回推的帳號或 IP。
- salt：`view_salts` 當日一列，第一次需要時 `INSERT … ON CONFLICT DO NOTHING` 後讀回（跨實例一致）；River 週期 job 刪 `view_day < 今天` 的 salt 與兩天前以前的 `post_views` 列（去重只需要當日）。
- 沒有 IP 時不計數（A7-2 的 handler 從 `rpc` 的 ctx 取；website SSR 代表誰的 IP 在 A7-2 依 `subject.go` 的 origin-auth 定）。
- `RecordShare(ctx, postID, buyerID)`：寫 `post_shares`，分享數由 trigger 加減。

## 6. 事件、訂閱與 job

### 6.1 social 發的模組事件（`social/events.go`）

`PostPublished`、`CommentCreated`、`CommentReplied`、`PostLiked`：只帶領域鍵（內容 ID、互動者 (kind, id)、`store_id`、以 rune 截斷的預覽），不帶名稱、頭像、account_id 或留言快照（M18、M19）。A7-3 的 relay 在自己的 job 裡經 buyer、seller、social 的 API 補 wire 需要的欄位（前例 `M/legacy/sellerrelay/relay.go` 在 relay job 裡讀 seller）。在 transaction 內 `events.Publish`，沒有訂閱者也照發。只給 SSE 的 4 列與 `counter.delta` 不建（計畫「規模」）。
- wire 的 `postId`、`commentId` 從 int64 變成 UUID 字串：notification 把它們當字串用（`notification-service/internal/worker/template_handler.go` 的 `DecodePayload`／`toStringVal`，`refId` 與深連結），收件人的 ID 才解成 int64（`extractUserID`），所以 notification 不用改；A7-3 以 backend-e2e-verify 實測一次。計畫的不變量「A8 之前 wire 不變」改寫成「欄位與語意不變，social 實體 ID 改成字串」。

### 6.2 social 訂閱的事件

- 買家刪帳：buyer 進入 `PENDING_DELETION` 時新發 `buyer.BuyerDeletionRequested`（buyer 的改動在本步範圍；刪帳申請不能撤回，`accountdeletion/accountdeletion.go:88-91`）；social 的同 transaction 訂閱者對它與 `BuyerDeleted` 都先寫複本並重新求值（`MarkAuthorInactive`，冪等），再 `jobs.Enqueue(PurgeBuyerContent{BuyerID})`（冪等）。job 軟刪該買家的留言與討論區、刪讚與分享、清內文；也排受影響貼文的快取失效。可見性與計數在申請提交時已改好，job 的軟刪差額是 0。
- `seller.StoreRetired`：同 transaction 只 `Enqueue(PurgeStoreContent{StoreID})`。job 軟刪該店的貼文與討論區（含店家身分的留言與讚）、釋放媒體引用；可見性由 `pub_exposable_stores` 擋住。
- job 冪等（以 `deleted_at IS NULL` 述詞、批次處理），重跑結果相同。

### 6.3 依賴表

`social` 的允許清單拿掉 im、review，只留用得到的：`buyer`（ACTIVE 檢查、名稱頭像、`pub_inactive_buyers`（對帳核對複本）、`BuyerDeleted`）、`seller`（店家與能力、`pub_*`、`StoreRetired`）、`media`。`catalog` 現在經 port 與 legacy productservice（§3.2），B4 之後才 import，所以先拿掉；`identity` 實作時確認沒有用到就拿掉（帳號 ID 從請求來，店家從 `seller.CurrentStore` 來）。

### 6.4 快取失效（M26）

- `platform/revalidate` 加 `PostTag(postID uuid.UUID) string`（與 `RecipeTag` 同形）。不送 `hashtag:` tag：website 把它展開成商品 hashtag 的 megamenu 快取（`website/lib/api/revalidate-request.ts:139-140`、`lib/api/cached.ts:605-618`），貼文的熱門話題用 `posts:list`，`post:` 與 `seller:` 都會展開到它（`revalidate-request.ts:79-83,135-136`）。
- 發布、下架、編輯、刪除、隱藏、還原在同一個 transaction 內 `jobs.Enqueue(revalidate.Args{Tags: …})`：貼文 tag 加店家 `StoreTags`。
- 店家曝光翻轉沿用 seller 既有的 `StoreTags` 失效；開工時核對店面貼文頁用的是哪個 tag（`website/app/[locale]/(public)/sellers/[id]/(storefront)/posts/page.tsx`）。

## 7. 根套件 API（`social/social.go`）

- 給 httpapi（A7-2）：貼文的建立、更新、發布、下架、刪除、讀取（公開、店主、作者）、列表（動態牆、店家、hashtag、時間窗熱度排序）；留言與討論區的建立、編輯、刪除、列表；讚與取消讚；`RecordView`、`RecordShare`；hashtag 的建議與趨勢；`MySanctions`、`MyHiddenContent`。
- 給 moderation（A7-1d）：`ContentOwner(target)`（存在、作者 (kind, id)、所屬店、是否公開可讀）、`Hide`、`Restore`、`Warn`、內容快照。
- 給 delegation（A7-2d）：同一組寫入方法，店家由呼叫方給 store_id，能力照 §3.1 檢查（不另開入口）。
- 列表都用 `pagination` 的 keyset。
- 熱門（a8，「本週主題」）：`TopPosts(n)` 回最近發布的 30 篇公開貼文中 `view_count` 最高的 n 篇（n ≤ 30，同分以 `published_at`、`id` 排），不分頁。這是把 website 現在的算法（`website/lib/api/posts.ts:104-121` 的 `fetchTopWeeklyPosts`：取最新 30 篇、依 `viewCount` 重排、取前 5）原樣搬到伺服器，畫面不變；改算法（加權、時間窗）是產品題，不在這一步。

## 8. 沿用盤點（`CARRYOVER.md`）

逐項寫 `domain-inventory.md` §2 的 M1–M27 中屬於 social 的項目（M1–M3、M5–M7 的不搬部分、M9、M11 的讀取面、M13–M23、M25–M27），四欄：現況（`SI/...:line`）、從零設計、去向、重新評估的條件。另外列：

- 舊驗證規則逐條（長度、圖片、標籤、每頁上限），每條附 `SI/...:line`。
- `publish_status` 4401／4402／4403 對到 `DRAFT`／`PUBLISHED`／`UNPUBLISHED`（CodeMeta 凍結，不新增值域）。
- 回覆不跟著頂層留言、頂層留佔位符（§2.3 第 2 點，Will 2026-10-23；改變舊行為）。
- 匿名瀏覽去重、salt 輪替。
- pg_trgm spike 的結果（§9.4）。
- 軟刪的新理由與觸發（§3.8）、textfilter 的誤擋與觸發（§3.6）、`hashtags` 實體表不建（§1）、`comments` 與 `forum_entries` 不合併（§1）、`sanctions.ends_at` 目前沒有讀者（a7 已核可，觸發：有期限的參與限制，Q2 (iii)）、熱門算法照搬（§7）。

## 9. 測試與量測

### 9.1 integration（`social` 與 `buyer`，`pg.NewDB(t, testdb.Jobs(), testdb.Migrations(…))`，`t.Parallel()`）

- 計畫 A7-1a 驗收列的 SD1、SD2、SD4–SD9、SD11（紀錄面）、SD13（還原）、SD16–SD19、SD24 各一個測試。
- 公開述詞：草稿、下架、刪除、隱藏、店家不可曝光、買家刪除中與已關閉，各自讓貼文、留言、討論區不可見；還原後可見。
- 作者讀得到自己被隱藏的內容與理由，別人讀不到；作者不能編輯。
- `BuyerDeletionRequested`、`BuyerDeleted`、`StoreRetired` 排入清除 job；job 重跑結果相同。
- 計數：任意一串寫入（留言、回覆、刪除、隱藏、還原、讚、取消讚、清除 job）之後，stats 等於重算函式的結果（資料集測試）；並發雙擊讚只加一；重算與並行按讚不互蓋；佔位項目：頂層刪除或隱藏後，有可見回覆時出現佔位、沒有時不出現，回覆照常計入。
- 每日對帳覆寫漂移並記錄（故意改壞一列 stats，job 修回並記下）。
- 匿名瀏覽同日同 IP、UA 只計一次、換日再計；沒有 IP 不計；表沒有 IP、UA 欄。
- 快取：發布、隱藏、還原各排入一個 `revalidate` job。
- `TestPubContractsMatchMigrations`、`TestSchemaFileMatchesMigrations`、`trimmedColumns`。

### 9.2 mutation

照計畫 A7-1a 的 mutation 清單；另加：把公開述詞的 `buyer.pub_inactive_buyers` 條件拿掉，「刪除中買家的留言不可見」要失敗；把 trigger 對 `hides` 的分支拿掉，計數的資料集測試要失敗；讓重算不先鎖 stats 列，並行測試要失敗。

### 9.3 效能（量測，不是測試）

照計畫 A7-1a：播種腳本與量測指令放附件 `step-1a-perf/`；EXPLAIN 看內聯與 Index Cond，200 次重複量 p95 ≤ 300 ms @ page_size 50。結果貼進計畫的進度表。

### 9.4 spike：pg_trgm 與 CJK

需求是「包含」，所以量子字串比對 `name LIKE '%' || q || '%'`：在單體 Postgres 18 的 locale 下，對 1–4 個漢字的查詢量正確性，以及在 `post_hashtags` 的預估量下不用索引時的耗時；只有不用索引太慢時才看 pg_trgm 的 GIN 索引（它支援 `LIKE`，但查詢少於 3 個字元時用不上）。舊的 `%`（相似度）不是子字串比對，不量；結果與指令寫進 `CARRYOVER.md`。不可用而「包含」搜尋又必要時停下回報（計畫停止條件）。extension 的加法照主計畫已實測的做法（`2026-09-24-go-modular-monolith.md` 第 79 行附近）。

### 9.5 特性測試

`make new-characterization name=social legacy=social-service`，貼文、留言、討論區、標籤 28 條情境；SD 修正的刻意差異標成 legacy 跳過並附 SD 編號。本步只跑 legacy 端（單體還沒有入口）；Connect driver 在 A7-2。

## 10. 開工 design-review 的處理（2026-10-23，21 項）

審查者：沒參與撰寫的 subagent，只看設計。改 15、記 3、提 2、駁回 0；另 1 項（UUID 與 wire）以查到的事實解決，改寫計畫的不變量。

| # | 發現 | 處理 |
|---|---|---|
| 1 | 取代計數 ADR 的理由寫錯（舊前提是鎖競爭，不是跨服務） | 改：§3.5 |
| 2 | `hashtags` 實體表與 id 升冪的鎖順序已無理由 | 改：`post_hashtags` 改成值的集合，§1、§3.7 |
| 3 | `hides` 含 POST，形成第二條隱藏店家內容的路徑 | 改：只收買家寫的留言與討論區，§1 |
| 4 | `hides` 主鍵讓同一則內容無法再隱藏 | 改：`id uuid` 加部分唯一索引，§1 |
| 5 | 模組事件照 legacy wire 定欄位，逼 social 解析名稱與 account_id | 改：只帶領域鍵，relay 補欄位，§6.1 |
| 6 | UUIDv7 與「A8 前 wire 不變」衝突 | 以事實解決：notification 把貼文、留言 ID 當字串用（`template_handler.go` 的 `toStringVal`），不變量改寫成「欄位與語意不變，ID 改成字串」，A7-3 實測，§6.1 |
| 7 | 存起來的計數和讀取時的可見性對不上 | 提：交 Will；Will 要業界基準，另派從零研究，定案在 `counts-brief.md` v4（§3.5） |
| 8 | `hashtag:<id>` 快取 tag 清錯快取 | 改：不送，§6.4 |
| 9 | 瀏覽每次更新貼文列，熱點觸發只看讚 | 改：ADR 觸發條件涵蓋瀏覽，§3.5 |
| 10 | spike 量相似度而不是子字串 | 改：量 `LIKE '%q%'`，§9.4 |
| 11 | textfilter 子字串比對的已知誤擋沒有重新評估條件 | 記：§3.6、§8 |
| 12 | 軟刪與保留媒體引用沒有新的必要條件 | 記：新理由是保留期未訂，§3.8、§8 |
| 13 | 留言索引照頁碼分頁的舊形狀 | 改：照 keyset 排序鍵，§1 |
| 14 | `hides` 一個 id 欄指多張表、沒有 FK | 改：型別正確的 FK 欄加 `num_nonnulls`，§1 |
| 15 | 代行用的指定店家版寫入繞過能力檢查 | 改：單一核心入口收店家、以 `CapabilitiesOf` 檢查，§3.1、§7 |
| 16 | 處分類別用賣家條款的值，沒有社群類別與子集 | 改：社群三值移到這一步加，處分用 `Assessable` 子集，§4 |
| 17 | 熱度排序沒有定義，和 keyset 衝突 | 改：照搬 website 現行算法成 `TopPosts`，不分頁，§7 |
| 18 | 父留言不可見時回覆跟著不可見是推論 | 提：Will 2026-10-23 選留佔位符、回覆照常顯示（§2.3） |
| 19 | `comments`、`forum_entries` 重複；同篇只在 Go 檢查 | 改：同篇用複合 FK；不合併記進盤點，§1、§8 |
| 20 | 長度上限只在 Go；`ends_at` 沒有讀者 | 改：`char_length` CHECK；`ends_at` 記（a7 已核可），§1、§8 |
| 21 | productservice 的呼叫位置與注入；依賴 identity 的必要性 | 改：port 注入、transaction 之前呼叫；依賴表只留用得到的，§3.2、§6.3 |

### Q-A 計數（已定案）

見 `counts-brief.md` v4：Will 2026-10-23 要求照業界基準、治本；四個子題照基準定案，回覆去向由 Will 選留佔位符。
