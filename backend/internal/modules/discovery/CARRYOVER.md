# discovery 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

## 沿用盤點

seller-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。決策來源是 ADR `docs/adr/2026-10-03-recommended-sellers-scoring.md`（下稱 ADR）。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 排序只看近 30 天營收（營收榜） | 排行要反映店家的好壞 | 不成立：大店壟斷、可自買刷高、新店永遠上不了榜 | 先以門檻決定資格，再以品質訊號排序並給新店曝光（Etsy、蝦皮優選賣家） | 不沿用：綜合分數（公式 v1，`internal/model/score.go`），需求用不重複買家數取 log 而非營收（ADR） |
| 營收口徑：狀態集合、`created_at` 窗口、PARTIALLY_REFUNDED 整筆計入金額、單筆大單可洗榜 | 要有「近期成立訂單」的訊號 | 狀態集合成立（已付款到完成、部分退款；與營收排名、CRM 的集合一致，是照搬的）；`created_at` 窗口不成立（下單早、付款晚的訂單會落在錯的窗口，賣家取消也不是在下單那天發生）；金額不再使用 | 口徑由資料擁有者在契約中定義，消費端不重猜；時間窗以事件發生的時間計 | 狀態集合沿用；窗口改成成立訂單以付款時間 `pay_time`、賣家取消以取消時間 `cancelled_at`（Java `findStoreOrderStats`，`ListStoreOrderStatsIT`）；不再用營收，只計不重複買家數與筆數，所以部分退款的金額不影響分數（部分退款仍是成立的購買）；口徑由 order 端定義、寫在 proto 註解與 port 文件，搬進單體時寫進 ordering 的契約 |
| 補位路徑：不足 limit 時以 `store_id DESC` 補「最新核准店家」 | 榜單不夠長時要有人補 | 不成立：補位是第二條排序語意，排序鍵是 surrogate id、沒有品質訊號 | 同一個排序鍵下排序，不另設補位 | 刪除：每家合格店都有分數，沒有資料時新店加分與 `published_at DESC, store_id DESC` 決定順序（`TestZeroDataRanksByNewnessThenPublishedAt`、`TestEqualScoresOrderByPublishedAtThenStoreIDDescending`） |
| 過度抓取 ×3，order 端夾 50（`limit>16` 時損耗悄悄流向補位） | 過濾發生在取排名之後 | 不成立：過濾下推到候選集合，快照包含全部合格店 | 資格先於排名；兩階段推薦（離線分數、線上套用規則） | 刪除：重算時就只算合格店；讀取分頁補滿，最多讀到快照盡頭（`TestReadPagesPastManyFilteredTopRows`） |
| 讀取時 fan-out：每次 cache miss 呼叫 order、product、本地兩查詢、buyer | 排行的新鮮度夠用即可 | 不成立：每個請求都依賴 Java 是否可用 | 近線批次預計算，讀取只查一張表 | River 每 15 分鐘重算 `discovery.seller_scores`，讀取只查快照加 seller 的批次資格（ADR） |
| product 失敗＝fail-open（空店再度曝光），product 全集整包每次拉取 | 要濾掉沒有公開商品的店 | 不成立：失敗不該放行；全集只在重算時拉一次 | fail-closed 或沿用上一份結果 | 失敗讓重算失敗、保留上一份快照並以 error 記錄上游；product port 文件寫明不可降級為空集合；全集每 15 分鐘一次（`TestUpstreamFailureKeepsThePreviousSnapshot/product`） |
| order 失敗＝整個請求失敗，其他上游軟處理（行為不一致） | 上游失敗時使用者仍有榜單 | 不成立：榜單與上游解耦後，失敗只影響重算 | 讀取不依賴批次上游；上游失敗時 stale-while-error | 四個上游一致處理：任一失敗，整次重算失敗、上一份快照繼續服務、River 重試；讀取只依賴 DB 與 seller 資格 |
| 追蹤數失敗＝0，與真實 0 無法區分 | 顯示欄位要有值 | 不成立 | 未知與 0 要區分，或省略欄位 | 追蹤數是顯示欄位，不屬 discovery；`internal/httpapi` 在步驟 8 組裝時要區分失敗與 0（見「不負責」） |
| 評分取自 `sellers.rating`（MQ 覆寫的副本，亂序會倒退） | 評分要即時且單調 | 不成立：seller 已刪除副本（seller `CARRYOVER.md`） | 從權威來源取，不維護副本 | 重算時經 review port 批次取評價數與平均（權威來源；review-service 新增 `ListStoreRatingStats`，定義與單店版相同，不逐店呼叫）；評分參與排序（貝氏平均）；失敗不當成 0（`TestFailedRatingIsNeverTurnedIntoZero`） |
| 「可公開」條件在 `GetByStoreIDs` 與 `GetApprovedExcluding` 各寫一份 SQL（只看 status 與 PayUni） | 單一事實來源 | 不成立：兩份會漂移，也沒有停權與 KYC | 單一規則函式，所有曝光端共用 | 不重寫：用 seller 的 `ExposableStores`（規則唯一的家是 seller 的 SQL `seller.pub_exposable_stores(at)`，Go 只讀它帶上的條件）；重算與讀取都經它，只有一個定義 |
| 過濾位置不一致（營收路徑在 Go 端、補位路徑在 SQL） | 同一規則同一位置 | 不成立 | 同上 | 只有一條路徑：候選在重算時過濾，讀取時再套用同一個資格 port |
| 沒有排行的存續資料與可觀測性 | 能說明某店為何在榜上 | 不成立 | 存分數與成分、公式版本 | `seller_scores` 存分數、四個成分、原始輸入、公式版本與算出時間；失敗以 error log 標明上游（新鮮度見下方各列） |
| 沒有後端快取，只靠 website ISR（`sellers:list` tag，900 秒）與 `seller:<id>` 展開失效 | 榜單變動要讓前端快取失效 | 部分成立：榜單改成每 15 分鐘整批更新 | 快照更新時發失效；或接受 TTL 的延遲 | 不新增失效鏈：分數最多落後 15 分鐘，與 TTL 同一個量級。步驟 8 注意：停權即時生效靠 seller 事件展開成 `sellers:list` 失效，這條鏈要保住（見傳輸層） |
| 失敗的粒度：整次重算失敗 vs 排除單一上游有問題的店 | 重算要在上游有狀況時仍有榜單 | 整次失敗成立：批次的上游沒有「某一家失敗」的情況（一次呼叫要嘛成功要嘛失敗），排除單一店要靠猜；上游回了違反契約的資料（沒問的店、不合法的數值）代表上游有 bug，靜默排除會讓榜單悄悄缺店 | stale-if-error（RFC 5861）：失敗時繼續服務上一份，錯誤要看得見 | 整次失敗、不寫入、上一份快照繼續服務、River 重試並把失敗記進 `recompute_state` 與 error log（`TestUpstreamFailureKeepsThePreviousSnapshot`、`TestReviewReturningAnUnaskedStoreFailsTheRun`）。批次 RPC 讓逐店呼叫與「某店失敗」不再存在 |
| 重算的新鮮度與可觀測性：舊服務沒有 | 能分辨「還沒算過」「算過了沒有店」「很久沒成功」 | 不成立（舊的每次現算） | 快照要有 as-of 時間與最近成功／失敗；告警由排程監控負責 | `recompute_state` 存最近一次嘗試、成功（開始與結束）、失敗（時間與有界錯誤）；`RecommendedSellers` 回傳 `ComputedAt`（nil＝從沒算過）。告警在單體對 River job 的監控（CD 待辦），不在模組內（`TestFreshnessDistinguishesNeverComputedFromNoEligibleStores`） |
| 重疊或重試的重算：River unique 當互斥 | 兩個重算不能讓舊結果蓋掉新的 | 不成立：`UniqueOpts.ByPeriod` 只去除重複的排入，重試、下一個週期、多節點都可能交錯 | 以版本或時間戳做條件寫入（樂觀鎖） | 每次記下開始時間；取代快照的 transaction 鎖住狀態列，只有開始時間晚於目前快照那次的才寫，否則丟棄並 log（`TestAnOlderRunFinishingLaterDoesNotOverwriteANewerSnapshot`）；取代掉原本的表鎖 |
| 讀取跨頁：每頁各自一個 statement | 同一次讀取要看見一致的快照 | 不成立：重算在兩頁之間提交，游標之後的排序已經是另一份快照，會漏或重複 | 分頁讀取用快照隔離 | 整個讀取在一個唯讀 `REPEATABLE READ` transaction（seller 的資格查詢走同一個 transaction，不另佔連線，避免連線池滿時互等）；`ComputedAt` 與列來自同一份快照（`TestReadSeesOneSnapshotEvenWhenARecomputeCommitsBetweenPages`，改回預設的 `READ COMMITTED` 會失敗；快照由 `db.InReadSnapshot` 的 `RepeatableRead` 提供） |
| 回傳 `RecentOrders`、評價數與平均 | 顯示（步驟 8）要與排名一致 | 訂單數成立；評價不成立（步驟 10a 第 3 刀修訂）：推薦清單的評價與其他清單上同一家店的卡片顯示同一個值，要走同一條批次讀取才一致；排名用的評價留在快照內 | 訂單數用同一份快照的事實；評價由 httpapi 與卡片一起批次讀取（`StoreCard.review_count`、`rating_average`） | 隨排行回傳快照裡的近 30 天成立訂單數（`TestRankedSellersCarryTheFactsUsedForRanking`）；評價數與平均不再從模組輸出 |
| 新店加分從 `stores.created_at`（開店）起算 | 新店要在第一次曝光時有曝光機會 | 不成立：審核拖得久的店在第一次曝光前就用掉新店期 | 以進入可售狀態的時間起算（上架、通過審核） | seller 的 `ExposableStore.PublishedAt`：全站唯一的公開時間 `stores.first_exposed_at`（`seller.stamp_first_exposed` 記下、永久保留；還沒記下的可曝光店是讀取當下，視為剛公開，這個規則只在 seller 寫一次）；不是核准時間：一月核准、三月才補齊 logo 的店從三月起算（seller 的 `TestPublishedAtStartsWhenTheStoreFirstBecomesExposable`）。newness 與同分排序都用它（`TestNewnessCountsFromTheFirstExposableDay`）；快照欄位 `published_at` 取代原本的 `exposable_since` |
| `limit<=0` 視為預設 10（buyer-app 傳 0 想關閉榜單卻照跑） | 契約對 limit 的語意明確 | 不成立 | 0 表示空，預設值只在傳輸層 | 模組只接受 1 到 50（其他回 `InvalidInput`，`TestLimitMustBeBetweenOneAndFifty`）；`0＝空` 與預設值由步驟 8 的契約與 handler 決定 |
| 會員方案是排行因子 | 排行用到方案 | 不成立：舊排行從未使用方案（盤點 §0） | 不適用 | 不適用；方案的刪除與 discovery 無關 |
| store_id 的 BIGINT 與 TSID 風格 | 跨 Java 服務的店家鍵 | 成立（過渡） | row-identity ADR：實體 UUIDv7，Java 期間沿用業務鍵 | `seller_scores.store_id BIGINT` 主鍵、不建 FK；階段 C 與 seller 一起換 |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位。新的對外 API 在計畫 A3b 步驟 8，照〈API 設計規範（Connect）〉；模組本身不含傳輸層。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| `GET /public/sellers/recommended`、marketplace 的 `recommendedSellers`（兩個形狀不同的出口）、`certified` 寫死 true、`followers` 失敗為 0 | 顯示欄位組裝 | 不成立 | 一個契約，欄位由曝光規則推導 | **8c 完成**：`DiscoveryPublicService.SuggestRecommendedSellers`（`internal/httpapi/discovery`）：店名、logo 與店家網址碼由 seller 的 `StoreCards` 一次批次讀取，評價數與平均由卡片提供（步驟 10a 第 3 刀修訂：原本取快照裡的值、handler 不依賴 review；現在 handler 對 review 做一次批次讀取，見 `StoreCard.review_count`，推薦清單不再有評價欄位），粉絲數取 buyer 的 `SubscriberCounts`、讀取失敗時欄位不設（不是 0）；`certified` 刪除（排行上的店本來就可曝光）；排行上的店沒有卡片、或卡片不是 OPEN 時略過並記 warn（不是整個請求 unavailable），所以清單上的每家店都有營業中的卡片、家數可能少於 max_results，不補預設；評價數放不進 int32 時不設；快照從沒算過時回空清單並記 warn |
| 錯誤：業務碼包在 200 | 錯誤分類 | 不成立 | Connect error code | 模組回 `apperr`（limit 不合法是 `InvalidInput`），轉換在 `internal/httpapi/rpc` |
| 分頁：只有 limit | 榜單的長度 | 成立 | 小榜單只用 limit | **8c 完成**：不分頁，依〈API 設計規範（Connect）〉§三 這是「依相關度排序的前幾筆」，用 `Suggest` 與 `max_results`（預設 10、最多 50，預設值寫在契約與 `httpapi/discovery.DefaultMaxResults`，超過 50 以 50 計（同 AIP-158 的 page_size）、負數回 invalid_argument）；方法名不是計畫寫的 `ListRecommendedSellers`：`List` 在規範裡一律分頁 |
| 逾時與重試：order 無逾時、product 3 秒 | 上游呼叫有界 | 部分成立 | 每個呼叫有 deadline，批次 job 有總時限 | 各 legacy client 自帶逐呼叫的 deadline；單次嘗試上限 10 分鐘；失敗由 River 重試 3 次，下個週期再算（重疊由開始時間檢查處理，不靠逾時小於間隔） |
| 限流、認證、trace | 公開讀取 | 成立 | 傳輸層處理 | 不在本模組，步驟 8 沿用 `internal/httpapi` |

## 與 seller-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline。

- 新表 `discovery.seller_scores`；舊服務沒有排行的存續資料（欄位 `published_at` 取代 `opened_at`）。
- 排序由營收改為公式 v1 的綜合分數；沒有補位路徑。
- 候選從「營收前 3×limit」改為「全部合格店家」；曝光資格是 seller 的 `ExposableStores`（含 KYC、PayUni、停權、店家資料完整度），不再只看 status 與 PayUni。
- 回傳是店家 ID、近 30 天成立訂單數，以及快照的計算時間；評價與其餘顯示欄位由 httpapi 組裝。
- limit 必須是 1 到 50，沒有預設值。
- 新的上游：order 的 `ListStoreOrderStats`（Java 新增）、review 的 `ListStoreRatingStats`（review-service 新增的批次 RPC）；`REVIEW_SERVICE_ADDR` 是新的 worker 設定。
- 新表 `discovery.recompute_state`（新鮮度與重算的序列化）。
- 回傳多了快照的計算時間（評價數與平均不再對外輸出）。
