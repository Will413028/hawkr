# social 模組

社群：貼文、留言、讚、店家討論區、hashtag、計數、瀏覽與分享，以及買家的社群處分與隱藏紀錄。來源是 social-service（遷移計畫 A7，計畫 `docs/plans/2026-10-18-a7-social.md`，設計 `docs/plans/2026-10-18-a7-social/step-1a-design.md`）。這一步只有根套件 API 與 job，沒有對外入口（Connect 在 A7-2）。

## 職責

- 貼文的生命週期（DRAFT → PUBLISHED → UNPUBLISHED → PUBLISHED，刪除是軟刪）、公開要件、hashtag 的整批替換、商品驗證（port）、圖片引用（照 content 的 `media.Ref`／`SyncMedia`）。
- 貼文的留言與店家討論區：兩層、互動者 (kind, id) 由伺服器依資源歸屬推導、作者編輯與作者或該店刪除（冪等）、頂層不可見而底下有可見回覆時回佔位項目。
- 讚（貼文只有買家，留言與討論區帶 kind）、瀏覽（每日換 salt 的去重）、分享。
- 計數：stats 表、「是否計入」的單一定義（資料庫的 `reevaluate_*`）與每則內容的旗標、trigger、重新求值、重算與每日對帳、`social-recount`。
- 買家的社群處分（`sanctions`）與隱藏紀錄（`hides`）：給 moderation 的 `Hide`、`Restore`、`Warn`、`ContentOwner`，給作者的 `MySanctions`、`MyHiddenContent`。
- 買家刪帳與店家退場的實體清除 job；快取失效（`platform/revalidate` 的貼文 tag）。
- 模組事件 `PostPublished`、`CommentCreated`、`CommentReplied`、`PostLiked`（只帶領域鍵，給 A7-3 的 relay）。

## 擁有的資料

schema `social`（`db/schema.sql`）：`posts`、`post_hashtags`、`comments`、`comment_likes`、`post_likes`、`forum_entries`、`hides`、`sanctions`、`inactive_authors`（買家刪帳的複本）、`post_stats`、`comment_stats`、`forum_entry_stats`、`post_views`、`view_salts`、`post_shares`，以及 media 引用的同步狀態 `media_ref_sync` 與退場資產 `retired_assets`。函式與 trigger 在 `db/objects.sql`（沒有 view）。
- migration：模組沒有合併或部署過，所以分支上的開發過程壓成兩支基線——`social_baseline`（Atlas 從 `schema.sql` 生成，含 `CREATE SCHEMA`）與手寫的 `social_objects`（`objects.sql` 的函式與 trigger）；之後的改動才是新的 migration（部署後不得改已套用的）。本機在壓平前套用過舊 migration 的資料庫要重建。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與事件。
- `internal/model` 型別與共用規則；`post`、`comment`、`forum`、`engage`（讚、瀏覽、分享）、`sanction`（處分與隱藏）、`stats`（重算與對帳）、`purge`（清除 job）各管自己的 use case；`actor` 推導互動者並檢查前提；`hashtag`、`textcheck` 是輸入的正規化與檢查；`text` 是使用者面句子的目錄（`zh-TW.json`、`en.json`，`make uitext` 生成 `text_gen.go`，不得手改；Go 裡不寫使用者面的字面常數，語系取自 `uitext.LocaleFrom(ctx)`）；`mediaref` 是圖片引用的同步；`readcache` 排快取失效；`store` 是 sqlc 的查詢套件；`visibility` 是「公開可讀」「可見」唯一的定義（go-jet 述詞與公開表的入口；「計入」只在資料庫的 `reevaluate_*`，`visibility.Counted` 只讀旗標）；`reads` 是帶這些規則的讀取（go-jet，26 支 query，登記表加守門測試）；`jet`（表型別）與 `pubsql`（`pub_*` 的包裝）是 `make jet` 的生成物，不得手改。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。依賴表：`buyer`、`seller`、`media`。

## 不變量

- **公開可讀**：一則內容公開可讀，若且唯若店家可曝光（`seller.pub_exposable_stores`）、貼文已發布、內容自己的旗標 `counted` 為真（沒有刪除、沒有未還原的隱藏、作者是買家時不在 `social.inactive_authors`，定義在資料庫的 `reevaluate_*`，見下一條）。容器的規則（店家可曝光、貼文已發布）和觀看者的規則在 Go 的 `internal/visibility` 寫一次（`ReadablePosts`、`ReadableForum`、`CommentsOf`／`ForumEntriesOf` 的 `Where`），帶這些規則的讀取（`internal/reads`）用 go-jet 在上面組成；沒有 view。讀取不 join `buyer.pub_inactive_buyers`：買家刪帳的事實是 social 自己的複本（`inactive_authors`），佔位項目的 state（DELETED 或 HIDDEN）才讀它。公開表（posts、comments、forum_entries）只經 `visibility` 的入口取得，`reads` 的 AST 測試擋直接引用 jet 的 `table.Posts` 等；原始 SQL 的逃生口只許出現在 `pubsql` 的生成檔；每支 jet query 都登記在 `reads.Registry()`，有 golden SQL（`testdata/golden`）、邊界檢查（`platform/querycheck`）與對測試資料庫的 Describe 形狀測試。固定代碼（`PUBLISHED`、`BUYER`）是 SQL 常數，不是 bind 參數（generic plan 無法用 `$1` 證明 partial index 的述詞）。A7-1c（內容下架）改 `visibility` 的述詞，golden 檔的差異就是影響範圍。時間一律是請求的 clock（`now` 參數），不用 `now()`。
- **個人封鎖是依觀看者的獨立條件，不併進 `counted`、不影響計數**：觀看者是買家（`visibility.Viewer{BuyerID}`，零值是未登入；公開讀取帶呼叫者的 `Caller.BuyerID`）。條件由入口型別必帶（`Posts`、`Comments`、`ForumEntries` 的 `Where`），不是呼叫端另外 AND 的第二個條件；匿名讀取的語句文字沒有封鎖子句（不用 `$v IS NULL OR …`），有觀看者的語句另存 `.viewer` golden。觀看者封鎖的店（`buyer.pub_blocked_stores()`），它的貼文、留言與討論區對觀看者整個不出現；與觀看者任一方向有個人封鎖的買家作者（`buyer.pub_blocked_buyers()`），他的頂層項目連同底下的回覆整則不出現，不變成佔位項目（佔位只有刪除與隱藏，對所有人一致）；被排除的列不進「有可見回覆」的判斷，回覆的讀取要求父列沒被排除（`Admits`）。店主以店家身分讀自己的店（`OwnerPosts`、`ListOwner*`、`actor.ViewerFor` 對自己的店回零值）、治理讀取（`GetCommentOwner` 等）與 hashtag 的全域彙總不套個人封鎖。寫入端：讀取類的前置檢查（貼文公開可讀、回覆目標可讀、讚的對象可讀、討論區可讀）帶呼叫者本人的觀看者，讀不到就不能寫；店家封鎖（`seller.StoreBlocksBuyer`，單向，不發布 `seller.pub_*`）併在寫入的身分解析裡：買家身分的寫入一律經 `actor.Resolver` 的 `ForWrite`（留言、討論區發言、留言讚）、`ForAuthor`（編輯自己的留言與發言）與 `ForBuyerWrite`（貼文讚），這三個入口先確認買家營運中、再檢查這家店有沒有封鎖他（PermissionDenied），所以新入口拿不到互動者就繞不過檢查；不檢查店家封鎖的路徑是 `ForSelfService`（分享）、刪除自己的內容與取消讚（不解析買家身分），店家封鎖攔不住自清的手；瀏覽不經身分解析。封鎖檢查的來源是 `Deps.StoreBlocks`（必要依賴，`New` 缺它就 panic；`TestNewSocialWiresStoreBlocks` 驗證正式組裝有接；只含 `StoreBlocksBuyer` 的介面，正式組裝傳 `seller.Module`；`seller.Reader` 不提供它）；被封鎖的買家仍讀得到店家的公開內容。封鎖查詢的量測在 `docs/plans/2026-10-18-a7-social/step-1b-perf/README.md`。
- **計數的維護路徑只有 trigger 與 reevaluate**（`db/objects.sql`）：寫入端不直接改 stats（重算除外：依 ID 由小到大 `SELECT … FOR UPDATE` 鎖一批 stats 列，再用另一個語句分組計數、只覆寫不同的列）。「是否計入」唯一的定義是 `social.reevaluate_comments`／`reevaluate_forum_entries`（`LANGUAGE sql BEGIN ATOMIC`，輸入只有 social 自己的表：`deleted_at`、`hides`、`inactive_authors`；Go 沒有第二份述詞，`visibility.Counted` 只是旗標）；每則留言與討論區留言存旗標 `counted`，reevaluate 對一批內容鎖定後求值，旗標不同才改旗標並加減貼文的留言數與父項的回覆數；冪等。trigger（留言與討論區留言的建立與 `deleted_at` 改變、隱藏的新增與還原）、買家刪帳的訂閱者（`stats.MarkAuthorInactive`）與每日對帳都只呼叫它。stats = 旗標之和，旗標只由 reevaluate 改，所以讀取與重算（旗標的加總）不重新推導。鎖的順序固定：內容列 → 貼文的 stats → 父項的 stats，各自依 ID 由小到大（reevaluate 自己取，不依呼叫方）。stats 有 `CHECK (>= 0)`，維護出錯時寫入失敗而不是漂移成負數。每日對帳先核對複本與 `buyer.pub_inactive_buyers`（缺的補上，`AuthorsAdded`），再讓全部內容重新求值（修旗標，`FlagsFixed`），再用 `reads` 的批次重算語句（500 列一批，一批一個 transaction）覆寫計數。瀏覽數是只增的事件計數，不重算。`TestStatsMatchTheDatasetAfterRandomWrites`、`TestConcurrentReevaluatesDoNotCountTwice`、`TestDirectHideRowRacingADeleteCountsOnce`、`TestReevaluateHasNoDeadlocksAndEndsConsistent`、`TestRecountDoesNotOverwriteConcurrentLikes`、`TestRecountSumsTheStoredFlagsWhenABuyerLeavesBetweenTheTwoSteps`、`TestReconcileOverwritesDriftAndLogsIt`、`TestReconcileRepairsFlagsBeforeCounts`、`TestReconcileAddsTheBuyersTheSubscriberMissed` 驗證。
- **回覆的 partial index 的述詞是旗標**（`comments_replies`、`forum_entries_replies` 是 `WHERE counted`）：讀取只看 `counted`，planner 無法從它證明 `deleted_at IS NULL`，述詞寫成後者索引就用不上（量測：熱門貼文頂層留言 1.7 ms 變 196 ms）。`TestReplyIndexesAreOnTheCountedFlag` 擋；改讀取的述詞時重量 `step-1a-perf`。
- **買家刪帳的事實在 social 有複本，由訂閱者同步寫**：`OnBuyerDeletionRequested` 與 `OnBuyerDeleted`（沒有開啟中訂單的買家直接從營運中關閉，只發後者，`TestDirectCloseStillHidesUncountsAndPurges`）在買家刪帳的同一個 transaction 呼叫 `stats.MarkAuthorInactive`：插入 `inactive_authors`（`ON CONFLICT DO NOTHING`，只增不刪），並讓該作者的全部內容（含已刪除的）集合式重新求值，先依 id 鎖內容列，求值是一次集合呼叫，`reevaluate_*` 在寫入前依同一個順序鎖整批 stats（貼文 stats → 父留言 stats → 討論區 stats，各依 id；不需要 `stats.Service.Lock`，`TestMarkAuthorInactiveLocksStatsInTheSameOrderAsTheTriggers` 釘住順序），再排清除 job。資料庫與 API 的讀取、計數在申請提交的同一刻改好，沒有 job 的延遲（`TestBuyerDeletionRequestUncountsTheContentInTheSameTransaction`）；網站的快取頁不在其內：清除 job 在同 transaction 排入、立即可跑，由它的 `finish` 失效快取，晚秒級（P2「秒到分鐘級」），快取 TTL 保底。這是單體規則 4 的原子狀態變更；`buyer.pub_inactive_buyers` 只給每日對帳的核對查詢（`MissingInactiveAuthors`）。A7-1c 的內容下架用同一個形狀（`MeasureCreated` 插入、`MeasureRevoked` 刪除）。
- **買家的寫入與刪帳序列化**：讚、分享、留言、編輯在寫入的 transaction 內先 `buyer.LockActive`（買家列的共享鎖，持有到提交；編輯用不加鎖的讀取確認作者，再鎖買家列，最後才鎖內容列，`TestEditLocksTheBuyerBeforeTheContentRow`）；刪帳申請（`BeginDeletion` 的排他鎖）要等它們提交，之後才開始的寫入被擋。訂閱者因此看得到申請之前成立的所有寫入並一起重新求值。鎖的順序：identity → buyer → seller → social。
- **隱藏與還原先鎖目標內容列**（`FOR UPDATE`，與刪除相同的鎖，服務層的 `lockTarget`），讓歸屬與可讀的檢查和寫入看到同一個已提交的狀態；trigger 的 reevaluate 自己也先鎖內容列，所以繞過服務層直接寫 `hides` 也與刪除序列化。
- **清除 job 鎖 stats 列的順序固定**：貼文的 stats、留言的 stats、討論區留言的 stats，各自依 ID 由小到大（`stats.Service.Lock`，在這一批的第一個寫入之前），與 trigger 的順序相同。買家刪帳的內容在申請的 transaction 已經不計入（複本與重新求值），job 的軟刪差額是 0，由 trigger 照常維護；軟刪做完之後把他全部已刪除的留言與討論區留言的內文設為 NULL（`scrubBodies`，墓碑列保留；`comments_scrubbed_check`：NULL 只出現在已刪除的列）。
- 隱藏、還原、警告的理由文字與來源參照先 `trimtext.Trim`，DB 以 `trimtext.Check` 的 CHECK 為後備（`trimmedColumns` 登記）。
- **keyset 的游標條件是單一列比較，沒有布林開關**：`(a, id) < (sqlc.arg(after_a), sqlc.arg(after_id))`；第一頁由 `model.*Cursor.Key()`（nil 回 `DescStart`／`AscStart` 的極值）給參數，所有頁同一個 prepared statement。`NOT has_cursor OR (…)` 在 generic plan 下是 Filter，`TestKeysetQueriesCompareOneRowAndHaveNoBooleanSwitch` 擋。
- 熱門標籤的窗口最長 `model.MaxTrendingWindow`（30 天），超過是 InvalidInput；A7-2 的預設窗口是 `model.DefaultTrendingWindow`。

## 已知限制

- 標籤的「包含」搜尋沒有索引（子字串 `LIKE`）：10⁵ 貼文下完整的建議 query p95 ≤ 146 ms，spike 結果在 `CARRYOVER.md`「pg_trgm spike 與效能量測」。觸發：包含搜尋的 p95 超過 300 ms。
- `ListHiddenContentOf` 的游標條件是 join 上的 Filter（作者自己被隱藏的內容，有界集合，成本與游標深度無關；其餘 keyset query 都是 Index Cond，包含 `ListHashtagPosts`：排序鍵複製在 `post_hashtags.published_at`，由 trigger 維護）。觸發：`ListHiddenContentOf` 實測 p95 超過 300 ms。量測在 `docs/plans/2026-10-18-a7-social/step-1a-perf/README.md`。
- 軟刪保留內容與圖片引用，直到隱私權政策訂出保留期（觸發見 `CARRYOVER.md`）；買家刪帳的清除 job 是例外：他的留言與討論區留言內文設為 NULL。
- 使用者看得到的錯誤句子都在 `internal/text`（`zh-TW.json`、`en.json`），語系取自 context（`uitext.LocaleFrom`）；A7-2 定義 handler 時負責把請求的語系放進 context。 `apperr.*` 的參數不得有字串字面值（只許 `"%s", text.X(locale)`；商品驗證的 port 回 `ErrProductNotAvailable`／`ErrProductNotOwned`，句子由 social 給）：`internal/arch` 的 `TestSocialApperrCallsCarryNoSentenceLiterals` 以 go/ast 掃 social 的非生成檔與 `cmd/hawkr/social.go`。
- 商品驗證經 port 呼叫舊的 product-service（跨行程），B4 catalog 搬進單體後換成 in-process。

## 讀取模型

無。

## 不負責

- 檢舉、案件與處置：moderation（A7-1d）；social 只存隱藏與警告的紀錄，案件以不透明的 `source_ref` 參照。
- 內容下架的過濾（A7-1c）、排程發布（A7-1f）、Connect 與 handler（A7-2）、推播 relay（A7-3）。
- 店家與買家的名稱、頭像：讀取時由 seller、buyer 批次補，social 不存快照。

## 決策紀錄

從 social-service 沿用的每個機制、當時的決定與理由，以及與 social-service 的差異在同目錄的 `CARRYOVER.md`。改動或移除沿用的機制前，先讀那一列。
