# 社群計數存在獨立的 stats 表，由 trigger 在同一個 transaction 維護，外部事實改變時重算，每日對帳

| 項目 | 值 |
|---|---|
| 狀態 | accepted（2026-10-23；Will 要求「業界怎麼做、治本」，由沒看過計畫與草稿的 subagent 從零查業界基準後定案，見 `counts-brief.md` v4；回覆跟著頂層留言的去向是 Will 選的（佔位符）；「是否計入」的單一定義先由 trigger 與重算的 query 各表達一次，後於同日改為 Go 述詞加旗標，再改為買家刪帳的事實在 social 有複本、「計入」只依 social 自己的表，見文末兩則修訂） |
| 日期 | 2026-10-23 |
| 影響範圍 | `internal/modules/social`（stats 表、`db/objects.sql` 的 trigger 與生成的 `reevaluate_*`（原為 `social.comment_counted`，見文末修訂）、`internal/visibility` 與 `internal/reads` 的重算語句、`internal/stats`、清除 job）、`cmd/hawkr` 的 `social-recount`、`buyer.BuyerDeletionRequested`；舊 social-service 的 `counter.delta` 與 10 分鐘對帳隨它退場 |
| 取代 | `2026-09-18-mq-social-counter-async-delta.md`（本檔取代它，它標 `superseded by` 本檔） |
| 計畫 | `docs/plans/2026-10-18-a7-social.md` 步驟 A7-1a、決定 A7-D11；設計 `docs/plans/2026-10-18-a7-social/step-1a-design.md` §3.5；基準 `docs/plans/2026-10-18-a7-social/counts-brief.md`、`counts-baseline.md` |

## 脈絡

貼文與留言的讚數、留言數、回覆數顯示在每一列 feed 上，讀遠多於寫，之後還可能依互動數排序。舊 social-service 把計數存在貼文與留言列上，經 `counter.delta`（RabbitMQ）非同步加減，再用 10 分鐘一次的全表對帳收斂；留言數只算頂層，為了和列表 API 的總數一致；瀏覽與分享直接加一、沒有去重。

舊 ADR（`2026-09-18-mq-social-counter-async-delta`）選非同步的**理由是熱門內容的 row-lock 競爭**，不是跨服務；它同時承認 `counter.delta` 不冪等（at-least-once 重送會重複計數），所以必須留一條對帳路徑。社群搬進單體之後：

- 資料、事件的發布者與消費者都在同一個行程、同一個 database；計數自己發自己收，沒有外部消費者。
- 舊 ADR 否決「在主要寫入的同一個 transaction 加減」，因為它解決不了鎖競爭；而否決 DB trigger，因為「把業務邏輯藏進 DB、難測」。這兩條是當時為了跨服務與沒有單一 DB 的前提下的取捨。
- 系統從未上線，沒有正式流量資料，**熱點競爭是否真的出現沒有量測**。

要被取代的不是非同步本身的理由，而是它的前提：同一個 DB、可以把計數放在獨立的列上避開和編輯搶鎖，而且「計入與否」現在由多個寫入者改變（作者刪除、隱藏與還原、買家刪帳的清除 job、內容下架），寫在應用程式裡每個呼叫點都要記得加減。

## 業界在同樣條件下怎麼做（細節與來源：`counts-baseline.md`、`counts-brief.md` §1）

| 系統 | 存或算 | 維護與防漂移 |
|---|---|---|
| Lemmy | 存在獨立的 aggregates 表 | 資料庫 trigger 依「是否計入」的舊值與新值的差異加減，同一個 transaction |
| Mastodon | 存在獨立的 `status_stats`（不放在 `statuses` 列上，避免和編輯搶鎖） | 寫入時加減，讀取時負值當 0，手動 `tootctl cache recount` 全量重算 |
| Discourse | 存在主題列上 | 寫入時更新，`ensure_db_consistency` 每 12 小時對帳 |
| GitLab | 存；熱點先緩衝在 Redis 再批次寫回（`CounterAttribute`） | 有從來源重算的機制 |
| PostgreSQL | `count(*)` 要逐列判斷可見性，我們的可見性還要 join 店家、作者、封鎖，本來就做不成 index-only | — |

歸納：全部存起來；留言數含回覆（Lemmy 與主流消費產品）；隱藏後不留佔位符的系統不計被隱藏與刪除的；所有觀看者看到同一個數字。

## 約束

| 約束 | 分類 | 影響 |
|---|---|---|
| 沒有正式流量資料，熱點競爭沒有量測 | REAL（缺資料） | 不推離基準；只決定何時改成分片或緩衝（下方觸發） |
| 單體規則 2 與 `TestEveryModuleStaysInsideItsOwnSchema`：migration 單獨套用在空的 database 上必須成功，DB 物件不能參照別的模組的 `pub_*`（`buyer.pub_inactive_buyers`、之後的 `seller.pub_content_takedowns`） | REAL（我們自己的規則，但保證 migration 可獨立套用；改規則的成本高於這裡的代價） | 「是否計入」的組合規則在 `social.comment_counted`，social 之外的事實由呼叫方帶入；trigger（plpgsql，執行時才綁定，單體規則 6 的例外）與重算的 query 各讀一次，用資料集測試鎖住相等 |
| 公司時期規範「對帳 job 不寫進後端」（`後端服務責任邊界規範.md` §三） | SELF-IMPOSED，與基準（Discourse 定期對帳、Mastodon 手動重算）相反 | 不採：加每日對帳 |
| 舊 ADR 的「DB trigger 難測、難觀測」 | SELF-IMPOSED，且已被資料集測試處理 | trigger 是計數唯一的維護路徑，`TestSchemaFileMatchesMigrations` 涵蓋它，`TestStatsMatchTheDatasetAfterRandomWrites` 對任意一串寫入比對 |
| 刪帳申請不能撤回（`accountdeletion` 只有 `Request` 與 `CompletePending`） | 事實 | 申請起就可以把買家的內容標成刪除 |

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 沿用 `counter.delta` 非同步加每 10 分鐘全表對帳 | 計數落後；事件非冪等；單體裡自己發自己收，多一條 MQ 路徑只為避開沒有量測過的競爭；全表對帳隨資料量線性成長 | 否決 |
| 讀取時 `COUNT(*)` | 無漂移，但 feed 每一列都要算，熱門貼文讀遠多於寫；可見性要 join 店家與作者，做不成 index-only | 否決 |
| 計數放在實體列上，同 transaction 加減 | 按讚、瀏覽和貼文編輯的 `version` CAS 搶同一列的鎖 | 否決 |
| **獨立的 stats 表，trigger 同 transaction 加減，外部事實改變時重算，每日對帳** | trigger 是新機制（本 repo 之前沒有，`rg -n -i 'RETURNS trigger\|CREATE TRIGGER' backend/internal/modules --glob '*.sql'` 在 A7-1a 之前沒有結果）；熱點列有鎖等待的風險（見下方觸發） | **採用** |

trigger 的必要條件：計入與否由多個寫入者改變，寫在應用程式裡漏一個呼叫點就變成每日對帳才修的常態漂移；trigger 讓「每個寫入都加減」成為結構保證（Lemmy 型）。若計入與否只剩一個寫入者，這個必要條件就不成立，trigger 可以改回應用層加減。

## 決策

1. **全部存**，放在 `social.post_stats`（讚、留言、瀏覽、分享）、`social.comment_stats`（讚、回覆）、`social.forum_entry_stats`（回覆），不放在實體列上。計數 `CHECK (>= 0)`。
2. **語意**：一則留言計入，若且唯若它自己沒有被刪除、沒有未還原的隱藏、作者是買家時不是已停用的買家（A7-1c 加上生效中的內容下架）。貼文與店家的狀態（草稿、下架、停權、關閉）不影響計數：那時整篇連同計數都不顯示，恢復後計數照舊。讚與分享的集合是存在的列。留言數含回覆，每則頂層留言另外有自己的回覆數。所有觀看者看到同一個數字，不依封鎖、作者或店主而變。頂層留言不可見而底下有可見的回覆時，列表回佔位項目，回覆照常顯示並計入。
3. **單一定義**：組合規則只在 `social.comment_counted(deleted, hidden, author_inactive)` 一處；hides 與作者是否停用兩個事實由 trigger 與 `db/queries/recount.sql` 各讀一次（DB 物件不能參照別的模組的 `pub_*`）。外部事實改變後、重算之前，trigger 看到的新舊值都是「不計入」，不會多扣，兩條路徑不會互相蓋掉。
4. **維護**：`db/objects.sql` 的 trigger（`RETURNS trigger`）依每一列「是否計入」的舊值與新值差異，在同一個 transaction 加減 stats，涵蓋 social 自己的所有寫入，包含批次的清除 job；固定順序更新（先貼文的 stats、再留言的 stats、再討論區留言的 stats，各自依 ID 由小到大）避免死結。隱藏與還原先鎖目標內容列，與刪除相同。寫入端不直接改 stats，重算除外。
5. **在 social 之外改變計入與否的事實，改變當下排重算 job**（trigger 只在 social 的列被寫入時觸發）：
   - 買家刪帳：buyer 在 ACTIVE 轉成 `PENDING_DELETION` 的同一個 transaction 發 `BuyerDeletionRequested`（申請不能撤回），social 的訂閱者只排清除 job；`BuyerDeleted` 排同一個 job，冪等。job 跑之前的秒級窗口，可見性由 `buyer.pub_inactive_buyers()` 擋住。買家的寫入與刪帳申請以 `buyer.LockActive` 序列化，清除 job 因此看得到申請之前成立的所有寫入，最後重算買家寫過的全部內容所在的對象。
   - 內容下架（A7-1c）：訂閱 seller 的措施事件，排受影響貼文的重算 job。
   - 店家停權或關閉：不改計數，整店本來就不公開。
6. **防漂移**：每日一個 River 週期 job 對全部可重算的計數重算、覆寫，記下漂移的數量與對象，漂移不為 0 時記 warn log（等於 trigger 有 bug 的警報）；手動全量重算 `hawkr social-recount`，改了計數定義的 migration 在同一步跑一次；重算先 `SELECT … FOR UPDATE` 鎖住 stats 列，再用另一個語句計數覆寫，不蓋掉並行的加減。
7. **瀏覽與分享的去重**（Discourse 型）：先寫去重列，真的新增才加一。瀏覽：`(貼文, 台北日, 觀看者鍵)` 的主鍵，登入者的觀看者鍵是帳號，匿名的是 `HMAC-SHA256(當日 salt, 貼文 ‖ client IP ‖ User-Agent)`（Plausible 的做法），表裡沒有 IP 與 UA，拿不到 client IP 時不計數；salt 每個台北日一列、跨實例共用，去重表與 salt 只留當日與前一日，之後由週期 job 刪。分享：`(貼文, 買家)` 的主鍵，同一位買家同一篇只算一次。瀏覽數沒有可重算的來源，是只增的事件計數，不進每日對帳。
8. **`counter.delta` 與 10 分鐘全表對帳退場**，留言與貼文 SSE 的樂觀計數（`likeCount` 取 `existing±1`）隨之消失。

## 換做法的觸發

上線前的負載測試量到**單一 stats 列持續的鎖等待**（熱門貼文的讚、瀏覽、留言同時寫同一列）→ 改成分片計數列（讀取時加總）或緩衝批次寫入（GitLab 型：先累積在記憶體或 Redis，批次寫回）。觸發條件涵蓋瀏覽，不只讚：瀏覽每次都更新貼文的 stats 列，在熱點上比讚更頻繁。A7-1a 在 10⁵ 貼文的資料量上量過讀取延遲（`docs/plans/2026-10-18-a7-social/step-1a-perf/`），寫入端的鎖等待要等有並發負載才量得到，現在沒有資料。

## 後果

- 計數與讀取一致，不再有落後的視窗與重複計數；feed 每一列直接讀 stats。
- 多一個新機制（trigger）：改了「是否計入」的定義必須同時改 trigger 與重算的 query，資料集測試會在兩者分歧時失敗；改了定義的 migration 要在同一步跑 `social-recount`。
- 每日對帳漂移不為 0 是警報，不是常態；若它長期不為 0，先查 trigger 而不是放寬對帳。
- 撤銷條件：若熱點量測證明單一列無法承受，依「換做法的觸發」改成分片或緩衝，那時另寫 ADR 取代本檔的第 1、4 點；第 2、3、5、6 點的語意不變。

## 修訂：「計入」的單一定義改在 Go，每則內容存旗標（2026-10-23，Will；A7-1a 收尾審查）

第 3、4、5、6 點與約束表第二列中「`social.comment_counted`、trigger 與重算的 query 各表達一次、用資料集測試鎖相等」被取代；第 1、2 點與語意不變。原因：把規則推進資料庫物件被「migration 單獨套用」擋住，結果是同一個規則寫在三處（view、`comment_counted`、各 query 的 join）靠測試數次數；共用可見性述詞的讀取改用 go-jet（`2026-09-27-module-internal-structure.md` 的修訂）之後，規則可以只有一個定義。

- **單一定義**：Go 的 `social/internal/visibility` 的 `Counted`（沒有刪除、沒有未還原的隱藏、買家作者沒有停用；A7-1c 加內容下架）。讀取與重算的 query 直接內聯它。
- **旗標**：`comments.counted`、`forum_entries.counted` 存「此刻是否計入」，貼文的留言數與父項的回覆數是旗標的加總。外部事實（買家停用）改變時沒有寫入觸發 trigger，舊設計只能整批重算；有旗標之後，重新求值並以旗標差額加減就是精確的。
- **資料庫端的求值**：`internal/counted` 把 `Counted` 生成成 `db/objects.sql` 裡 `GENERATED` 區塊的 plpgsql 函式 `reevaluate_comments(ids)`、`reevaluate_forum_entries(ids)`（單體規則 6 的第二個例外，`migrate.Module.Generated`）；golden 測試比對，`go test … -update` 重新生成。它先鎖（內容列、貼文 stats、父項 stats，各自依 ID 由小到大）再求值、與旗標不同才改旗標並加減；冪等；VOLATILE（STABLE 的函式沿用呼叫者的 snapshot，看不到鎖等待之後別人提交的改動）。
- **維護**：trigger 只呼叫 reevaluate：留言與討論區留言的建立與 `deleted_at` 改變、隱藏的新增與還原。讚、分享、瀏覽照舊（單列加減）。讀取不得逐列呼叫 reevaluate（量測：27k 列 8,676 ms 對內聯 53 ms）。
- **外部事實改變的 job**：買家刪帳的清除 job 先讓買家寫過的內容（含已刪除的）reevaluate，再軟刪；原本最後一步的整批重算不再需要（必要條件「trigger 看不到外部事實」由旗標與 reevaluate 取代）。
- **每日對帳**：先讓全部內容 reevaluate（修旗標，回報 `FlagsFixed`），再用 `reads` 的重算語句（同一個 Go 述詞）覆寫計數；旗標先於計數修，因為 reevaluate 依旗標差額加減。
- **鎖**：內容列 → 貼文 stats → 父項 stats，與寫入端、清除 job 相同；清除 job 的批次仍在第一個寫入之前依序鎖 stats。隱藏與還原不再需要在 trigger 內 `FOR UPDATE` 內容列（reevaluate 先鎖內容列）；服務層的 `lockTarget` 保留，讓 `Owner` 的檢查與寫入看到同一個已提交的狀態。

## 修訂二：買家刪帳的事實在 social 有複本，「計入」只依 social 自己的表（2026-10-23，Will 選 D-sync；A7-1a 最終審查）

上一則修訂中「Go 述詞 `visibility.Counted` 是唯一定義、`internal/counted` 生成 plpgsql 函式、`migrate.Module.Generated` 例外、買家停用由清除 job 重新求值、讀取與重算內聯述詞並讀 `buyer.pub_inactive_buyers`」被取代；第 1、2 點與語意不變，約束表第二列（migration 單獨套用）仍成立，但改成靠資料模型滿足，不靠例外。原因：生成器、規則 6 的第二個例外與「述詞與旗標分開」是同一個問題的三個症狀——「計入」依賴 buyer 擁有的事實，而 DB 函式不能參照別的模組。讓所有輸入都屬於 social，三個症狀一起消失。審查找到的 R1（重算用讀取時的買家現況推導，旗標卻要等清除 job 才跟上，兩者在空窗不一致，之後清除 job 把計數扣成負數）也因此結構性消失。

- **複本**：`social.inactive_authors(buyer_id PK, since)`。買家狀態不會回到營運中（刪帳申請不能撤回），所以只插入不刪除，冪等（`ON CONFLICT DO NOTHING`）。
- **同步寫入**：social 訂閱 `BuyerDeletionRequested` 與 `BuyerDeleted` 的訂閱者，在買家刪帳的同一個 transaction 內（單體規則 4 的「原子狀態變更」，已把「複製擁有者的事實並讓依賴它的旗標一起改」明寫進規則）呼叫 `stats.MarkAuthorInactive`：插入複本，並讓該作者的內容集合式重新求值。兩個事件都要訂，因為沒有開啟中訂單的買家直接從營運中關閉，只發 `BuyerDeleted`。可見性與計數在申請提交的同一刻改好，不等 job（決定 P2「可見性立即消失」與 SD13「還原後立即可見」不變；這是 Discourse、Lemmy 的形狀：旗標由發動的請求同步寫入）。
- **單一定義**：「計入」只在 `db/objects.sql` 的 `reevaluate_comments`、`reevaluate_forum_entries`，用 `LANGUAGE sql BEGIN ATOMIC` 寫成（PG 18.6 實測可行，body 的每個語句各取新 snapshot，「先鎖再求值」仍成立），輸入只有 social 自己的表：`deleted_at`、`hides`、`inactive_authors`。plpgsql 只剩 `RETURNS trigger` 的函式（規則 6 唯一的例外）。`internal/counted`、`migrate.Module.Generated` 與規則 6 的第二個例外刪除。
- **讀取與重算**：讀取直接用 `comments.counted`、`forum_entries.counted`（`visibility.Counted` 只是這個旗標），不再重新推導，也不再 join `buyer.pub_inactive_buyers`；重算是旗標的加總（stats = 旗標之和，旗標只由 reevaluate 改）。佔位項目的 state（DELETED 或 HIDDEN）才讀 `inactive_authors`，只對頁內的列求值。
- **批次寫入者與鎖**：row-level trigger 保留（單列寫入各自求值最簡單）。逐列寫入的批次寫入者（清除 job）走 `stats.Service.Lock` 依固定順序（內容列、貼文 stats、父留言 stats、討論區 stats，各依 id）鎖好再寫，因為每列的 trigger 各自交錯取 stats 鎖，批次無法靠 trigger 排序；`MarkAuthorInactive` 以一次集合呼叫 `reevaluate_*` 求值，函式在寫入前自己依同一順序鎖整批，不需要 `Lock`。換成 statement-level trigger 的觸發：出現不能用 `Lock` 的批次寫入者。
- **對帳**：每日對帳多一步：`buyer.pub_inactive_buyers` 有而複本沒有的買家補進來並重新求值，記為漂移（`AuthorsAdded`）。`buyer.pub_inactive_buyers` 因此保留，只給這個 Go 查詢（單體規則 2 允許的跨模組讀取），不給 trigger 或旗標的求值。
- **A7-1c 的內容下架**用同一個形狀：seller 的 `MeasureCreated` 在同 transaction 插入 social 的複本表、`MeasureRevoked` 刪除，並讓受影響內容重新求值；下架可撤銷，所以這張表不是只增（買家那張是）。見計畫 A7-1c。
- **代價與觸發**：複本是新機制，必要條件是「資料庫端維護的旗標依賴別的模組擁有的事實，而函式不能參照別的模組」；若 `reevaluate_*` 改成不存旗標（讀取時推導），這張表與同步寫入就不再需要。每日對帳多一個核對查詢。
