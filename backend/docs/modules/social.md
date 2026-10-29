# social 模組行為規格

本文是單體 `social` 模組行為的唯一來源。範圍：貼文、留言、讚、店家討論區、hashtag、計數、瀏覽與分享，以及買家的社群處分與隱藏紀錄。

## 1. 範圍與公開介面

- 模組目前只有根套件 API 與 job，沒有對外入口（Connect handler）。
- 資料在 schema `social`（`db/schema.sql`）：`posts`、`post_hashtags`、`comments`、`comment_likes`、`post_likes`、`forum_entries`、`hides`、`sanctions`、`inactive_authors`（買家刪帳的複本）、`content_takedowns`（seller 內容下架措施的複本）、`post_stats`、`comment_stats`、`forum_entry_stats`、`post_views`、`view_salts`、`post_shares`，以及 media 引用的同步狀態 `media_ref_sync` 與退場資產 `retired_assets`。函式與 trigger 在 `db/objects.sql`，沒有 view。
- migration 是兩支基線：`social_baseline`（Atlas 從 `schema.sql` 生成，含 `CREATE SCHEMA`）與手寫的 `social_objects`（`objects.sql` 的函式與 trigger）；部署後不得改已套用的 migration。
- 模組事件 `PostPublished`、`CommentCreated`、`CommentReplied`、`PostLiked` 只帶領域鍵。
- 買家的社群處分（`sanctions`）與隱藏紀錄（`hides`）對 moderation 提供 `Hide`、`Restore`、`Warn`、`ContentOwner`，對作者提供 `MySanctions`、`MyHiddenContent`。
- 買家刪帳與店家退場各有實體清除 job；貼文的快取失效經 `platform/revalidate` 的 tag。
- 依賴：`buyer`、`seller`、`media`；依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 2. 貼文與 hashtag

- 貼文生命週期：DRAFT → PUBLISHED → UNPUBLISHED → PUBLISHED；刪除是軟刪。公開有要件，商品經 port 驗證（存在、上架中、屬於這家店），圖片引用照 content 的 `media.Ref`／`SyncMedia`。
- hashtag 隨貼文整批替換；標籤以值的集合 `post_hashtags` 存，沒有 `hashtags` 實體表。
- 熱門標籤的窗口最長 `model.MaxTrendingWindow`（30 天），超過是 InvalidInput；預設窗口是 `model.DefaultTrendingWindow`。
- 軟刪保留內容與圖片引用，直到隱私權政策訂出保留期；買家刪帳的清除 job 是例外（見 §6）。

## 3. 留言、討論區與互動

- 留言與店家討論區都是兩層。互動者 (kind, id) 由伺服器依資源歸屬推導；店家身分只用在自己的貼文與自己的討論區。
- 作者可編輯自己的內容（已被隱藏的內容不可編輯）；作者或該店可刪除，刪除冪等。
- 頂層項目不可見而底下有可見回覆時，列表回佔位項目（只有 ID、時間、狀態；state 為 DELETED 或 HIDDEN），回覆照常顯示並計入；沒有可見回覆時整則不出現。佔位只有刪除與隱藏，對所有觀看者一致。被內容下架的頂層留言或討論區留言與被隱藏相同：底下有可見回覆時回 state 為 HIDDEN 的佔位項目（不是 DELETED），回覆照常顯示並計入。
- 讚：貼文只有買家（`post_likes (post_id, buyer_id)`），留言與討論區帶 `actor_kind`。瀏覽每日換 salt 去重；分享同一位買家同一篇只算一次。
- 使用者看得到的錯誤句子都在 `internal/text`（`zh-TW.json`、`en.json`，`make uitext` 生成 `text_gen.go`，不得手改），語系取自 `uitext.LocaleFrom(ctx)`。`apperr.*` 的參數不得有字串字面值，只許 `"%s", text.X(locale)`；商品驗證的 port 回 `ErrProductNotAvailable`／`ErrProductNotOwned`，句子由 social 給。

## 4. 公開可讀與讀取規則

- **公開可讀**：一則內容公開可讀，若且唯若店家可曝光（`seller.pub_exposable_stores`）、貼文已發布且沒有被內容下架、內容自己的旗標 `counted` 為真；留言與討論區留言的旗標含「沒有被內容下架」，貼文底下的留言跟著父貼文。
- **內容下架**：社群內容被 seller 的內容下架措施（`CONTENT_TAKEDOWN`，規格 `seller.md` §5）下架後，所有公開讀取都看不到它，措施被撤銷後立即回來。
  - 複本：social 有 `content_takedowns(measure_id, post_id | comment_id | forum_entry_id, created_at)`，一列一個（措施，內容），恰好一個內容欄位非空（CHECK），內容欄位是外鍵，內容不存在時措施的建立回滾。同一則內容可被多個措施下架，內容只要有任一列就算下架。這是複本而不是讀取時 join `seller.pub_content_takedowns()`：旗標與 `reevaluate_*` 是資料庫函式，不能參照 seller 的表。
  - 訂閱者：`OnMeasureCreated`、`OnMeasureRevoked` 在 seller 建立與撤銷措施的同一個 transaction 執行，只處理帶 `Contents` 的內容下架。建立時插入並讓被下架的留言與討論區留言重新求值，排貼文與店面頁的快取失效；撤銷時只刪該措施的列並重新求值，另一個措施仍下架同一則內容時維持隱藏。呼叫方回滾時複本跟著回滾。**建立時核對歸屬**：訂閱者鎖住留言與討論區留言的列之後，核對每則內容存在、沒有刪除、作者是 `MeasureCreated.StoreID`（貼文的作者是所屬店家；留言與討論區留言的 `actor_kind = STORE` 且 `actor_id` 相同），不成立回 `TakedownContentError`（內容種類與 ID），整個措施回滾；每一條建立入口共用這一條，買家寫的內容走 `Hide`。貼文不加鎖：作者不可變，核對之後被店家軟刪只多一列複本。不認得的內容種類（值域在 `platform/contentkind`）同樣是錯誤。守門：`TestTakedownRejectsContentThatIsNotTheMeasureStoresOwn`、`TestTakedownOfUnknownContentKindFailsTheCallersTransaction`。
  - 讀取：貼文沒有計入旗標，`visibility.ReadablePosts` 對 `post_id` 加 NOT EXISTS（走 `content_takedowns_post`，`TestTakedownPredicateKeepsTheReadPlansIndexed` 守住 feed、店面頁、hashtag 與單篇讀取的計畫）。留言與討論區留言走旗標，計數同旗標。
  - 店主讀自己的店（`OwnerPosts`、`ListOwnPosts` 等不套公開條件的讀取）仍看得到被下架的貼文，沒有標記；賣家端的下架標記在 A7-4。
  - 對帳：每日對帳（`Reconcile` 與 `RecountAll`，在修旗標之前）雙向核對 `seller.pub_content_takedowns()`：複本缺的補上（`TakedownsAdded`）、多的刪除（`TakedownsRemoved`），都計入漂移。seller 有、但 social 找不到內容的列略過（插入會違反外鍵）。補漏不核對歸屬（措施已生效，內容之後可以被作者刪除），寫入的那一句自己再看一次 `seller.pub_content_takedowns()`，讀到「缺」的清單之後才被撤銷的措施不會被寫回（`TestReconcileInsertRechecksThatTheMeasureIsStillActive`）；撤銷的 transaction 與這一句同時進行且尚未提交時仍可能寫入一次，下一次對帳的「多」那一步刪除。
- 容器的規則（店家可曝光、貼文已發布）與觀看者的規則在 `internal/visibility` 寫一次（`ReadablePosts`、`ReadableForum`、`CommentsOf`／`ForumEntriesOf` 的 `Where`）；帶這些規則的讀取（`internal/reads`）用 go-jet 在上面組成；沒有 view。
- 讀取不 join `buyer.pub_inactive_buyers`：買家刪帳的事實是 social 自己的複本 `inactive_authors`，只有佔位項目的 state（DELETED 或 HIDDEN）才讀它。
- 公開表（posts、comments、forum_entries）只經 `visibility` 的入口取得；原始 SQL 的逃生口只許出現在 `pubsql` 的生成檔。
- 每支 jet query 登記在 `reads.Registry()`，有 golden SQL（`testdata/golden`）、邊界檢查（`platform/querycheck`）與對測試資料庫的 Describe 形狀測試。
- 固定代碼（`PUBLISHED`、`BUYER`）是 SQL 常數，不是 bind 參數：generic plan 無法用 `$1` 證明 partial index 的述詞。
- 時間一律是請求的 clock（`now` 參數），不用 `now()`。
- **keyset 的游標條件是單一列比較，沒有布林開關**：`(a, id) < (sqlc.arg(after_a), sqlc.arg(after_id))`；第一頁由 `model.*Cursor.Key()`（nil 回 `DescStart`／`AscStart` 的極值）給參數，所有頁同一個 prepared statement。`NOT has_cursor OR (…)` 在 generic plan 下是 Filter。
- **回覆的 partial index 的述詞是旗標**：`comments_replies`、`forum_entries_replies` 是 `WHERE counted`。讀取只看 `counted`，planner 無法從它證明 `deleted_at IS NULL`，述詞寫成後者索引就用不上。改讀取的述詞時重量效能（`docs/plans/2026-10-18-a7-social/step-1a-perf/README.md`）。

## 5. 個人封鎖與店家封鎖

- 個人封鎖是依觀看者的獨立條件，不併進 `counted`、不影響計數。觀看者是買家（`visibility.Viewer{BuyerID}`，零值是未登入；公開讀取帶呼叫者的 `Caller.BuyerID`）。
- 條件由入口型別必帶（`Posts`、`Comments`、`ForumEntries` 的 `Where`），不是呼叫端另外 AND 的第二個條件。匿名讀取的語句文字沒有封鎖子句（不用 `$v IS NULL OR …`）；有觀看者的語句另存 `.viewer` golden。
- 觀看者封鎖的店（`buyer.pub_blocked_stores()`）：它的貼文、留言與討論區對觀看者整個不出現。
- 與觀看者任一方向有個人封鎖的買家作者（`buyer.pub_blocked_buyers()`）：他的頂層項目連同底下的回覆整則不出現，不變成佔位項目。被排除的列不進「有可見回覆」的判斷；回覆的讀取要求父列沒被排除（`Admits`）。
- 不套個人封鎖的讀取：店主以店家身分讀自己的店（`OwnerPosts`、`ListOwner*`、`actor.ViewerFor` 對自己的店回零值）、治理讀取（`GetCommentOwner` 等）、hashtag 的全域彙總。
- 寫入端的讀取類前置檢查（貼文公開可讀、回覆目標可讀、讚的對象可讀、討論區可讀）帶呼叫者本人的觀看者，讀不到就不能寫。
- 店家封鎖（`seller.StoreBlocksBuyer`，單向，不發布 `seller.pub_*`）併在寫入的身分解析：買家身分的寫入一律經 `actor.Resolver` 的 `ForWrite`（留言、討論區發言、留言讚）、`ForAuthor`（編輯自己的留言與發言）、`ForBuyerWrite`（貼文讚）。這三個入口先確認買家營運中，再檢查這家店有沒有封鎖他（PermissionDenied）。
- 不檢查店家封鎖的路徑：`ForSelfService`（分享）、刪除自己的內容、取消讚（不解析買家身分）；瀏覽不經身分解析。被店家封鎖的買家仍讀得到店家的公開內容。
- 封鎖檢查的來源是 `Deps.StoreBlocks`：必要依賴，`New` 缺它就 panic；型別是只含 `StoreBlocksBuyer` 的介面，正式組裝傳 `seller.Module`（`seller.Reader` 不提供它）。封鎖查詢的量測在 `docs/plans/2026-10-18-a7-social/step-1b-perf/README.md`。

## 6. 計數

- 計數存在獨立的 stats 表（`post_stats`、`comment_stats`、`forum_entry_stats`），`CHECK (>= 0)`：維護出錯時寫入失敗，不漂移成負數。留言數含回覆，每則頂層留言另有回覆數；所有觀看者看到同一個數字。
- **維護路徑只有 trigger 與 reevaluate**（`db/objects.sql`）。寫入端不直接改 stats；重算例外：依 ID 由小到大 `SELECT … FOR UPDATE` 鎖一批 stats 列，再用另一個語句分組計數、只覆寫不同的列。
- 「是否計入」唯一的定義是 `social.reevaluate_comments`／`reevaluate_forum_entries`（`LANGUAGE sql BEGIN ATOMIC`）。輸入只有 social 自己的表：`deleted_at`、`hides`、`content_takedowns`、`inactive_authors`。Go 沒有第二份述詞，`visibility.Counted` 只讀旗標。
- 每則留言與討論區留言存旗標 `counted`。reevaluate 對一批內容鎖定後求值，旗標不同才改旗標並加減貼文的留言數與父項的回覆數；冪等。
- trigger（留言與討論區留言的建立與 `deleted_at` 改變、隱藏的新增與還原）、買家刪帳的訂閱者（`stats.MarkAuthorInactive`）與每日對帳都只呼叫 reevaluate。stats 是旗標之和，旗標只由 reevaluate 改，所以讀取與重算不重新推導。
- 鎖的順序固定：內容列 → 貼文的 stats → 父項的 stats，各自依 ID 由小到大（reevaluate 自己取，不依呼叫方）。
- 每日對帳依序：核對複本與 `buyer.pub_inactive_buyers`（缺的補上，`AuthorsAdded`）；讓全部內容重新求值（修旗標，`FlagsFixed`）；用 `reads` 的批次重算語句（500 列一批，一批一個 transaction）覆寫計數。手動全量重算是 `hawkr social-recount`。
- 瀏覽數是只增的事件計數，不重算。
- 讚、分享、瀏覽是單列加減，不經 reevaluate。

## 7. 買家刪帳

- 買家刪帳的事實在 social 有複本 `inactive_authors(buyer_id PK, since)`，只增不刪（`ON CONFLICT DO NOTHING`）。
- `OnBuyerDeletionRequested` 與 `OnBuyerDeleted` 在買家刪帳的同一個 transaction 呼叫 `stats.MarkAuthorInactive`：插入複本，並讓該作者的全部內容（含已刪除的）集合式重新求值，再排清除 job。沒有開啟中訂單的買家直接從營運中關閉，只發 `BuyerDeleted`。
- `MarkAuthorInactive` 先依 id 鎖內容列；求值是一次集合呼叫，`reevaluate_*` 在寫入前依同一個順序鎖整批 stats（貼文 stats → 父留言 stats → 討論區 stats，各依 id），不需要 `stats.Service.Lock`。
- 資料庫與 API 的讀取、計數在申請提交的同一刻改好，沒有 job 的延遲。網站的快取頁不在其內：清除 job 在同 transaction 排入、立即可跑，由它的 `finish` 失效快取，晚秒級，快取 TTL 保底。
- `buyer.pub_inactive_buyers` 只給每日對帳的核對查詢（`MissingInactiveAuthors`）。
- **買家的寫入與刪帳序列化**：讚、分享、留言、編輯在寫入的 transaction 內先 `buyer.LockActive`（買家列的共享鎖，持有到提交）；編輯用不加鎖的讀取確認作者，再鎖買家列，最後才鎖內容列。刪帳申請（`BeginDeletion` 的排他鎖）要等它們提交，之後才開始的寫入被擋。鎖的順序：identity → buyer → seller → social。
- **清除 job**：鎖 stats 列的順序固定為貼文的 stats、留言的 stats、討論區留言的 stats，各自依 ID 由小到大（`stats.Service.Lock`，在這一批的第一個寫入之前），與 trigger 的順序相同。買家刪帳的內容在申請的 transaction 已經不計入，job 的軟刪差額是 0。軟刪做完之後，把他全部已刪除的留言與討論區留言的內文設為 NULL（`scrubBodies`，墓碑列保留；`comments_scrubbed_check`：NULL 只出現在已刪除的列）。

## 8. 隱藏、還原與處分

- 隱藏與還原先鎖目標內容列（`FOR UPDATE`，與刪除相同的鎖，服務層的 `lockTarget`），讓歸屬與可讀的檢查和寫入看到同一個已提交的狀態；trigger 的 reevaluate 自己也先鎖內容列，所以繞過服務層直接寫 `hides` 也與刪除序列化。
- 隱藏、還原、警告的理由文字與來源參照先 `trimtext.Trim`；DB 以 `trimtext.Check` 的 CHECK 為後備（`trimmedColumns` 登記）。
- 處分 `sanctions` 的 `kind` 目前只有 `WARNING`，`ends_at` 可為空。警告是紀錄，作者讀得到，不限制參與。
- 案件以不透明的 `source_ref` 參照；social 只存隱藏與警告的紀錄。
- 店家與買家的名稱、頭像在讀取時由 seller、buyer 批次補，social 不存快照。

## 9. 已知限制

- 標籤的「包含」搜尋沒有索引（子字串 `LIKE`）：10⁵ 貼文下完整的建議 query p95 ≤ 146 ms。量測結果在 `CARRYOVER.md`「pg_trgm spike 與效能量測」。包含搜尋的 p95 超過 300 ms 時要重新評估。
- `ListHiddenContentOf` 的游標條件是 join 上的 Filter：作者自己被隱藏的內容是有界集合，成本與游標深度無關。其餘 keyset query 都是 Index Cond，包含 `ListHashtagPosts`（排序鍵複製在 `post_hashtags.published_at`，由 trigger 維護）。實測 p95 超過 300 ms 時要重新評估；量測在 `docs/plans/2026-10-18-a7-social/step-1a-perf/README.md`。
- 商品驗證經 port 呼叫舊的 product-service（跨行程），在 transaction 之前呼叫。
