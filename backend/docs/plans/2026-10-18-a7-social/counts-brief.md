# A7 社群計數：業界基準與定案（v4，Will 2026-10-23 定案）

- 對象：A7-1a 設計書 `step-1a-design.md` §10 的 Q-A；計畫的 A7-D11 與「預設」的計數項。
- 快照：main `109edd425`（2026-10-23）。標記：【量】＝讀過原始碼、論文、官方文件或跑過指令；未查證的另外標明。
- 經過：v1–v3 由主 session 起草、兩輪 subagent 批判，推薦一直錨定在草稿自己的框架（「依可見性改不改變來分存或算」）。Will 2026-10-23 四題都回「業界怎麼做、治本」之後，另派一個沒看過計畫與草稿的 subagent，只拿需求從零查業界（報告：`counts-baseline.md`）。v4 照它的基準定案，v1–v3 的推薦撤回。

## 0. 前提題

要改的程式之後不會被重寫、搬走或刪除：social 模組是 A7-1a 新建的終態本體，還沒有程式。

## 1. 業界基準（細節與來源見 `counts-baseline.md`）

| 系統 | 存或算 | 語意 | 維護與防漂移 | 證據 |
|---|---|---|---|---|
| Lemmy | 存（獨立的 aggregates 表） | 被移除、刪除的不計，還原後恢復 | 資料庫 trigger 依「是否計入」的舊值與新值差異加減，同一個 transaction；批次寫入自動涵蓋 | 原始碼【量】 |
| Mastodon | 存（獨立的 `status_stats`，不放在 `statuses` 列上，避免和編輯搶鎖） | 刪除不計 | 寫入時加減；讀取時負值當 0；手動 `tootctl cache recount` 全量重算 | 原始碼、文件【量】 |
| Discourse | 存（主題列上） | 被隱藏的照算（串裡留佔位符） | 寫入時更新；`ensure_db_consistency` 每 12 小時對帳 | 原始碼【量】 |
| Facebook TAO | 存（關聯的計數） | 計數是整串的大小 | 依觀看者過濾的是內容項目，不是計數 | 論文（USENIX ATC 2013）【量】 |
| GitLab | 存；熱點用 `CounterAttribute` 先緩衝在 Redis 再批次寫回 | — | 有從來源重算的機制 | 原始碼【量】 |
| Reddit、Instagram | 存；被移除的留言仍計（留佔位符）；Instagram 隱藏讚數只是顯示開關 | — | — | 第三方轉述（未查證） |
| PostgreSQL | `count(*)` 要逐列判斷可見性；最近改過的資料頁讓 index-only scan 回到讀表；我們的可見性還要 join 店家、作者、封鎖，本來就做不成 index-only | — | — | 官方文件【量】 |

歸納：

- **存或算**：全部存。逼出這個做法的條件是 feed 的每一列都要顯示計數、讀遠多於寫，而且之後可能要依互動數排序；這些條件我們也有。
- **語意**：計數是「一般人在父項可見時看得到的則數」。隱藏後**不留佔位符**的系統（Lemmy、Mastodon）不計被隱藏與刪除的；留佔位符的（Reddit、Discourse）才照算。
- **留言數含回覆**：主流消費產品與 Lemmy 都含；Mastodon 只計直接回覆，是聯邦架構拿不到完整討論串逼出來的。
- **所有觀看者看到同一個數字**：封鎖者看到的計數比列表多，業界接受這個落差。

## 2. 既有機制為什麼存在

- 舊 social：計數存在貼文與留言列上，經 `counter.delta`（RabbitMQ）非同步加減、10 分鐘全表對帳（`domain-inventory.md` M3）。非同步是為了避開熱門內容的 row-lock 競爭（ADR `2026-09-18-mq-social-counter-async-delta`），對帳是因為 `counter.delta` 不冪等、會丟。舊的留言數只算頂層，為了和列表 API 的 total 一致（新 API 不回 total，理由已不在）。
- 計畫 A7-D11（Will 2026-10-18）：在單體內改成同 transaction 更新。基準正是這樣，保留。

## 3. 約束帳

| 約束 | 分類 | 對結論的影響 | 來源 |
|---|---|---|---|
| 沒有正式流量資料 | REAL（缺資料） | 不推離基準（基準本來就是存起來）；只影響何時改成分片或緩衝 | 系統從未上線 |
| 可見性在讀取時以 `pub_*` 判斷（店家可曝光、買家已停用、內容下架） | SELF-IMPOSED（我們的決定，符合基準） | 只決定「誰會在 social 之外改變計入與否」，由 §4 第 6 點的重算路徑涵蓋 | ADR 2026-09-24 修訂段 |
| 單體規則 2「讀取模型等量測觸發才建」 | SELF-IMPOSED；它講跨模組讀取，計數是模組自己的資料，不適用 | 無 | backend `AGENTS.md` |
| 公司時期規範「對帳 job 不寫進後端」 | SELF-IMPOSED（我們的規範）；和基準（Discourse 定期對帳、Mastodon 手動重算）相反 | 不採，加每日對帳 | `backend/docs/開發規範/後端服務責任邊界規範.md` §三 |
| 刪帳申請不能撤回 | 事實【量】：`accountdeletion` 只有 `Request` 與 `CompletePending` | 申請起就可以把買家的內容標成刪除 | `accountdeletion/accountdeletion.go:88-91` |

沒有 REAL 約束推離基準。

## 4. 定案

1. **全部存**，放在獨立的 stats 表（Mastodon 型）：`social.post_stats`（讚、留言、瀏覽、分享）、`social.comment_stats`（讚、回覆）、`social.forum_entry_stats`（回覆）。按讚、瀏覽不和貼文編輯（`version` CAS）搶同一列的鎖。
2. **語意**：一則留言計入，若且唯若它自己沒有被刪除、沒有未還原的隱藏、沒有生效中的內容下架、作者沒有停用。貼文與店家的狀態（草稿、下架、店家停權或關閉）不影響計數：那時整篇連同計數都不顯示，恢復後計數照舊，不需要重算。回覆**不**跟著頂層留言（§5 的決定：頂層不可見時留佔位符，回覆照常顯示並計入）。草稿不能留言。讚與分享的集合是「存在的列」。
3. **留言數含回覆**；每則頂層留言另外顯示自己的回覆數。
4. **所有觀看者看到同一個數字**，不依封鎖、作者或店主而變。
5. **單一定義**：「是否計入」只有一個定義，由 trigger 與重算的 query 各自表達，用資料集測試鎖住兩者相等（實作時的修訂，主 session 2026-10-23 依真約束決定）：單體規則 2 與 `TestEveryModuleStaysInsideItsOwnSchema` 要求 migration 單獨套用在空的 database 上也成功，所以 DB 物件不能參照別的模組的 `pub_*`（`buyer.pub_inactive_buyers`、`seller.pub_content_takedowns`）；組合規則寫在 `objects.sql` 的 `social.comment_counted(deleted, hidden, author_inactive)`，social 之外的事實由呼叫方帶入：plpgsql 的 trigger（執行時才綁定，單體規則 6 的例外）與 `db/queries/recount.sql` 的重算 query 各讀一次。重算 job、每日對帳、整合測試的判定與 trigger 因此是同一個組合規則加同樣的事實來源，`TestStatsMatchTheDatasetAfterRandomWrites` 以獨立寫的 SQL 對照任意一串寫入之後兩者的結果。原本的「一支 SQL 函式」做不到，因為 BEGIN ATOMIC 函式與 view 在建立時就綁定它參照的物件。兩條路徑不會互相蓋掉：外部事實改變後、重算之前，trigger 看到的新舊值都是「不計入」，不會多扣。

   > 2026-10-23 再更新（Will 選 D-sync，ADR `2026-10-23-social-counts-in-stats-tables.md` 修訂二）：下面這則「Go 述詞加生成 plpgsql」又被取代——買家刪帳的事實在 social 有複本 `inactive_authors`，「計入」只在資料庫的 `BEGIN ATOMIC` 函式 `reevaluate_*`，沒有生成器。以下為歷史紀錄。
   > 2026-10-23 更新（Will，ADR `2026-09-27-module-internal-structure.md` 的修訂）：上面「各自表達、用資料集測試鎖相等」改成：「計入」的唯一定義在 Go（`social/internal/visibility` 的 `Counted`），讀取與重算內聯它，資料庫端的求值由 `internal/counted` 生成 plpgsql 函式 `reevaluate_*`（golden 測試比對），每則內容存旗標 `counted`、reevaluate 依旗標差額加減計數。`comment_counted` 與 view 已刪除；`TestStatsMatchTheDatasetAfterRandomWrites` 的獨立 SQL 仍是對照。
6. **維護**（Lemmy 型）：資料庫 trigger 依每一列「是否計入」的舊值與新值差異，在同一個 transaction 加減 stats（`objects.sql`，單體規則 6 允許 `RETURNS trigger`），涵蓋 social 自己的所有寫入，包含批次的清除 job。計數 `CHECK (>= 0)`；trigger 照固定順序更新（先貼文的 stats，再留言的 stats），避免死結。
7. **在 social 之外改變計入與否的事實**（改變當下排重算 job，因為 trigger 只在 social 的列被寫入時觸發）：
   - 買家刪帳：buyer 在進入 `PENDING_DELETION` 時發 `BuyerDeletionRequested`（申請不能撤回），social 的同 transaction 訂閱者排清除 job：job 把他的內容標成刪除、刪掉他的讚與分享，最後重算受影響的貼文；`BuyerDeleted`（關閉）排同一個 job，冪等。job 跑之前的秒級窗口，可見性由 `buyer.pub_inactive_buyers()` 擋住，計數稍後跟上。
   - 內容下架（A7-1c）：social 的 `MeasureCreated`／`MeasureRevoked` 訂閱者（已經要排快取失效）同時排受影響貼文的重算 job。
   - 店家停權或關閉：不改計數，整店本來就不公開。
8. **防漂移**：
   - 每日一個 River 週期 job 對全部 stats 跑重算，覆寫並記下漂移的數量與對象（Discourse 型）；漂移不為 0 時記 warn log，等於 trigger 有 bug 的警報。
   - 手動全量重算指令 `hawkr social-recount`（Mastodon 型）；改了計數定義的 migration 在同一步跑一次。
   - 重算先 `SELECT … FOR UPDATE` 鎖住 stats 列，再用另一個語句計數，不蓋掉並行的加減。
9. **瀏覽與分享**（Discourse 型）：先寫「每人每貼文每天一次」的去重列，真的新增才 +1；去重表只留 1–2 天。瀏覽數沒有可重算的來源，是只增的事件計數，不進每日對帳（對帳只覆寫可重算的欄）。
10. **換做法的觸發**：上線前負載測試量到單一 stats 列持續鎖等待 → 分片計數列或緩衝批次寫入（GitLab 型）。寫進取代 `2026-09-18-mq-social-counter-async-delta` 的新 ADR。

## 5. 要 Will 決定的（產品題）

頂層留言被作者刪除、被平台隱藏或作者刪帳時，底下別人的回覆怎麼辦：

- (i) 回覆一起不公開，留言數連帶扣掉（舊 social 的行為，由實作者核對）；
- (ii) 頂層留言顯示「此留言已刪除／已隱藏」佔位符，回覆照常顯示並計入（Reddit、Discourse 型；畫面改動）。

兩種都能用 trigger 做，差別在扣法與畫面。

**結論（Will 2026-10-23）：(ii) 留佔位符、回覆照常顯示並計入。** 頂層留言不可見而底下有可見的回覆時，列表回一個沒有內文與作者的佔位項目（狀態是已刪除或已隱藏）；沒有可見回覆時整則不出現。佔位符的文字在 A7-4 的 UI 稿一起核可，文字由共用文案目錄提供（狀態是 enum，規則 E）。

## 6. 還沒驗證的

- trigger 在本 repo 沒有前例（`rg -n -i 'RETURNS trigger|CREATE TRIGGER' backend/internal/modules --glob '*.sql'` 沒有結果【量】），是新機制。必要條件：計入與否由多個寫入者改變（作者刪除、`hides` 的新增與還原、清除 job 的批次、重算），寫在應用程式裡要每個呼叫點都記得加減，漏一個就變成每日對帳才修的常態漂移；trigger 讓「每個寫入都加減」成為結構保證（Lemmy 型）。單體規則 6 已允許 `RETURNS trigger` 的觸發函式；social 的 `AGENTS.md` 寫明這是計數唯一的維護路徑，`TestSchemaFileMatchesMigrations` 涵蓋它。
- 10⁵ 規模下讀取時算的延遲沒有量；選「存」的依據是基準與之後的熱門排序，不是效能壓力。
- 瀏覽的定義（feed 曝光算不算、bot 排除）沿用設計書 §5 的「打開單篇、每人每天一次」。
