# 10e 第 3d 刀：待辦與期限怎麼做——從零與業界比對 v3

> **狀態**：v3，已處理兩輪獨立審查（第一輪 1 個 blocker、10 個 major、7 個 minor；第二輪 1、9、4，對照見附錄 A）。v3 的建議 (g) 是第二輪審查者提出、我查證後採用的。**Will 2026-10-20 回「照建議」：Q1 採 (g)**；實作照 `step10e-3d-design.md` v3。起因：Will 2026-10-20 看了 `step10e-3d-design.md` v2 之後問「好的做法是什麼？業界怎麼做？…重構成最好的做法，不要留技術債」。

依據：業界研究 `step10e-3d-industry-research.md`（每條附 URL 與 VERIFIED／SEARCH／INFERENCE；下稱 ir）；兩輪審查 `step10e-3d-queue-review1.md`（含寫入點逐條表，下稱 rv1）、`step10e-3d-queue-review2.md`（下稱 rv2）；`step10e-3d-design.md` v2（下稱 3d v2）；3b D7、D8；3cd D′1；rb R3；A7 決策簡報 A7-D8 與 A7 計畫 A7-1e；ADR `2026-09-24-go-modular-monolith` 文末「修訂：跨模組讀取改用擁有者發布的 pub_*、D4 收窄」（下稱 D3 修訂、D4 (a)）；ADR `2026-10-18-seller-governance-consolidated`（下稱 gov ADR）。

---

## 結論

- **業界在同一個 database 裡的做法**：工作是存下的實體，佇列是對實體的查詢，成員資格在讀取時判斷（ServiceNow 的 task、Zendesk 的 ticket 與 view、JSM 的 issue 與 queue）［VERIFIED，ir A1–A3，rv1 重讀］。另存「中央工作項目」是多個系統要進同一個佇列、而且要路由與指派時才做的（Salesforce Omni-Channel、ROOST Coop）［ir A4–A5］。SLA 的紀錄（每段的開始、目標、有沒有準時）另存，是為了報表與可設定的暫停規則［ir B2］。
- **我們的情況**：11 種待辦都在同一個 database 的 seller schema；A7 的 moderation 之後也在同一個 database。專案自己的 ADR 已經為這種情況定了做法：擁有資料的模組發布帶時間參數的唯讀 SQL 函式（`pub_*`），上層模組在自己的 SQL 裡 join 它（D3 修訂第 4 點）；「能在讀取時推導的，改 join `pub_*`」（D4 (a)）。先例是 `seller.pub_exposable_stores(at)`，seller 自己的 `exposure.Classify` 也讀同一個函式。
- **建議 (g)**：seller 發布 `seller.pub_review_items(at)`（11 種待辦的成員、開始時間、期限，判定式與期限規則都在這一個函式裡）；新模組 `reviewqueue` 用一支 SQL 讀它（A7 時再 `UNION ALL` moderation 的函式），依期限排序、keyset 分頁。seller 的詳情頁期限也讀同一個函式。
  - 和 3d v2 比：**不需要** httpapi 的合併層、platform 的比較函式、哨兵、跨來源 seek、property test 的替身來源；一支 SQL 本身就是一致的快照。
  - 和「另存工作項目」比：**不改任何寫入路徑**（約 35 個指令函式、約 55 個效果，rv1），佇列的 bug 不會讓賣家或管理員的操作失敗。
- **終局**：另存的 SLA 紀錄（每段待辦一列，存當時的期限與結束原因），由 seller 在指令內以帳號或來函為單位重算維護（修正版 (e)，§2）。觸發：第一份要看「當時有沒有準時」的報表（達成率、給主管機關的報告）、第二位營運人員（認領）、或 `ListReviewQueue` 在 page_size 50 時 p95 超過 300 ms。那時 `pub_review_items` 就是重算與重建的來源，不會丟掉。
- **要你決定**：Q1 採 (g) 還是現在就做 (e)。(g) 會改寫 3cd D′1 (c)「httpapi 合併」與 A7-D8 (c)／A7-1e 的「接進 3d 的合併層」，改成 ADR 第 4 點的 `pub_*`。

---

## 0. 前提題：這題要改的東西之後會不會被重寫、搬走或刪除？

| 東西 | 現況 | 之後的排程 | 判定 |
|---|---|---|---|
| 3d v2 的設計（合併層、比較函式、哨兵、`SeekFor`） | 只有設計稿［量測：`rg -n 'ReviewQueue\|reviewqueue' backend/internal api admin/src` 0 命中］ | 3d 本身 | 現在改，丟掉的是設計稿 |
| 3cd D′1 (c)、A7-D8 (c)、A7 計畫 A7-1e（`:321`、`:541`） | 已核准：「3d 做合併層，A7-1e 接進來」 | A7-1e | SELF-IMPOSED（我們的決定）；(g) 把 A7-1e 改成「發布 `moderation.pub_review_items` 加一個分支」 |
| 3b D8、設計書 32「期限讀取時推導，不另存」、gov ADR `:29` | 已核准 | 沒有 | (g) 照它們；(e) 才要推翻 |
| seller 的指令 | 已寫好 | 沒有 | (g) 不碰；(e) 約 35 個函式要改 |
| A7 moderation、A5 發票（`invoice` 已在依賴表 `backend/internal/arch/modules.go:25`） | 還不存在 | A7-1e、A5 | 我們的計畫分段（SELF-IMPOSED）；兩者都會在同一個 database |

結論：沒有要丟的程式；照從零的最佳做法選。

---

## 1. 選項與比較

| 選項 | 待辦頁怎麼讀 | 寫入路徑 | 跨來源（A7） | 歷史（當時的期限、有沒有準時） | 主要代價 |
|---|---|---|---|---|---|
| (a) 3d v2：各來源推導＋httpapi 的 keyset 合併（3cd D′1 (c)，已核准） | 每個來源一支查詢，合併層在記憶體合併 | 不改 | 合併層 | 沒有存 | 合併層、全序比較函式、哨兵、跨來源 seek、替身來源的 property test；現在只有一個來源 |
| (a′) 同 (a)，但先不做合併層 | seller 一支查詢 | 不改 | A7-1e 時再做合併層 | 沒有存 | 把合併層延到 A7 |
| **(g) `pub_*` 組合（建議）** | `reviewqueue` 一支 SQL 讀 `seller.pub_review_items(at)` | 不改 | 加一個 `UNION ALL` 分支 | 沒有存 | 一個函式、一個薄模組；期限規則寫在 SQL |
| (e) 另存的工作項目，指令內重算維護（v2 的建議，修正版見 §2） | 一張表、一個索引 | 約 35 個函式加呼叫 | A7 在自己的指令裡呼叫同一支 API | 有 | 寫入耦合：佇列的 bug 讓指令失敗；鎖順序；核對 job |
| (c) 另存，手寫每個轉移（v1 的建議） | 同 (e) | 約 55 個效果手寫 | 同 (e) | 有 | v1 第一次盤點就漏了約 20 個（rv1） |
| (f) 另存，outbox 非同步投影 | 同 (e) | 每個指令排一個 job | 同 (e) | 有 | 延遲；投影的 job、重建指令 |

### 1.1 業界基準（細節與 URL 在 ir）

- 同一個 database：佇列是對實體的查詢（ServiceNow 的 task 基礎表、Zendesk 的 view 依 SLA 排序、JSM 的 queue 是 JQL 篩選）［VERIFIED，ir A1–A3］。
- SLA 另存成紀錄：ServiceNow `task_sla`（階段、扣掉暫停的違約時間、原始違約時間，「Repair SLA」從歷史重算）、JSM 的週期（官方承認會缺失、提供重建）［VERIFIED］。原因是 SLA 的條件可以由管理員設定，而且要報表［ir B2，INFERENCE］。
- 多系統、要路由：中央工作項目指向來源紀錄（Salesforce Omni-Channel，SEARCH；ROOST Coop，VERIFIED）。
- 「推導當第一步，推導查詢之後當重建來源」是研究的推論［INFERENCE，ir C］。

### 1.2 為什麼現有的決定是那樣

- 3b D7 選推導：一致、期限不另存；另存要雙寫，還有 4 條不是指令的路徑［3b `:39`］。
- 3cd D′1 選 httpapi 合併：當時 D3 第 3 點寫「跨模組篩選與分頁要用讀取模型」，D′1 為它寫了「keyset composition」的例外；同一天（2026-10-18）D3 修訂另外加了第 4 點的 `pub_*`，D′1 沒有回頭比較［3cd `:54-136`；ADR `:304-318`］。
- A7-D8 跟著 D′1 改成 (c)［A7 決策簡報］。

### 1.3 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| 只有一位營運人員 | **REAL**：不需要認領、指派 | 3cd D′1 |
| 從未上線、沒有期限的資料、沒有報表的讀者 | **REAL**：所以另存的 SLA 紀錄現在沒有讀者（necessity 不成立） | rb R3 |
| D3 修訂第 3、4 點、D4 (a) | SELF-IMPOSED，符合基準（同一個 database 用一支查詢），保留：(g) 照第 4 點與 D4 (a) | ADR `:314-316`、`:336` |
| 「加來源不改合併層」（A7 的要求） | SELF-IMPOSED；(g) 下加來源是加一個分支，沒有合併層 | 3cd `:110` |
| 單體規則 2：SQL 只碰自己的 schema 加依賴表允許的 `pub.sql` | SELF-IMPOSED，符合基準，保留：`reviewqueue` 依賴 seller（之後加 moderation） | `backend/AGENTS.md:28`；`modules.go` |
| 期限推導不另存（3b D8、設計書 32、gov ADR `:29`） | SELF-IMPOSED；(g) 照它，終局 (e) 的觸發寫明 | — |

### 1.4 建議 (g) 的理由

- 對 (a)／(a′)：同樣推導、同樣不改寫入路徑；但同一個 database 裡一支 SQL 就能排序與分頁，合併層、比較函式與哨兵是在 application 裡重做 database 的事。ADR 自己的第 4 點與 D4 (a) 就是這個規則。
- 對 (e)：(e) 的好處（歷史、單一索引、認領的位置）現在都沒有讀者；代價（約 35 個指令要改、佇列的 bug 讓指令失敗、鎖順序、核對 job、週期判斷的邊界）現在就要付。第二輪審查還找到 (e) 的五個設計缺口（§2），表示它要做就要做對，不是加一行呼叫那麼簡單。
- (g) 不留要丟的東西：終局 (e) 的「從事實算該有哪些待辦」就是 `pub_review_items`。

---

## 2. 終局：修正版 (e)（觸發時才做，先寫下來）

第二輪審查指出 v2 的 (e) 有五個缺口，觸發時照修正版做：

1. **先鎖再算**：重算要在 seller 的領域鎖之下（帳號的店家列，或來函列），而且要開新週期時那一列還不存在、鎖不到；所以串行化靠領域鎖，佇列只信任呼叫方已經鎖住。
2. **重算的單位是帳號或來函**，不是單一通報或措施：關店、商家認證退回草稿、刪帳號會影響同一個帳號的多種待辦；以帳號為單位，呼叫方本來就鎖著它；或由 seller 的 transaction helper 在 commit 前對碰過的帳號自動重算，不手加 35 個呼叫。
3. **週期的身分**：不能只用 `(種類, 對象, 開始時間)`：恢復營業後會開出開始時間相同、一開就逾期的新週期；複審或告知在出現之前改期，會留下從沒出現過的週期。要另定（例如以「這一段可以處理的期間」的事實為鍵）。
4. **核對的漂移定義**：時間造成的出現與離開（複審在措施自然結束時離開）不算漂移，不記錯誤。
5. **存結束原因**：沒有原因就算不出達成率，歷史的好處不成立。

另外寫明：佇列的列在全域鎖順序的位置（D4 (c)：上層模組的列在 seller 之後；seller 的公告列也自稱最後一個鎖，要一起定）。

---

## 3. 對 3d v2 的影響（選 (g) 時）

| 3d v2 | (g) 之下 |
|---|---|
| E1 種類、§5 的種類標籤 | 不變 |
| E2 `Deadline` 訊息（v2 的 Q1） | 不變；資源的期限由 seller 讀同一個 `pub_review_items`（以對象篩選），不另寫一份規則 |
| E3 有無期限的旗標 | 保留（A7 的種類可能沒有期限），是 SQL 的一欄 |
| E4–E8、E14 的全序、`SeekFor`、哨兵、合併層、`InReadSnapshot`、property test | 刪除；換成 `reviewqueue` 一支 SQL 的 row-value keyset：`(NOT has_due, due_at, start_at, kind, item_id COLLATE "C")`；一個語句就是一個快照 |
| E9 鍵會變（v2 的 Q2） | 不變（改嚴重度、延後複審、換確認日期會改期限或開始時間；同 Zendesk 的 view 重新排序）；不需要決定題，寫進設計 |
| E10 `ReviewPolicy` | 期限的時長只寫在 `pub_review_items` 裡（規則一個家，ADR 第 4 點：擁有者的 Go 程式讀同一份 SQL）；Go 的判定式（指令用的 `AwaitingTriage` 等）與函式的成員由資料集測試鎖住一致；每種的期限以表格測試逐種斷言 |
| E11 食品業者登錄、E13 權限細到種類、E15–E17 | 不變（權限：`reviewqueue` 的查詢收呼叫者有權的 kinds） |
| E12 ADR | 3cd D′1、A7-D8、A7 計畫 A7-1e 寫修訂段：改用 D3 修訂第 4 點；ADR 2026-09-24 不必加 composition 的不變式 |
| 新的 | `reviewqueue` 模組（依賴表加 `"reviewqueue": {"seller"}`）、`seller.pub_review_items(at)`（手寫 migration，`db/pub.sql` 宣告形狀，契約測試）、函式內聯與計畫的量測（先例 `TestExposableStoresJoinProbesMeasuresByAccount`） |

---

## 4. 沒有驗證的部分

- `pub_review_items` 的 11 個分支以 `UNION ALL` 寫在一個函式裡時，外層的 `ORDER BY … LIMIT` 與詳情頁「以對象篩選」能不能推進各分支：沒有跑 `EXPLAIN`；先例的註解記了 `UNION ALL` 讓關聯條件推不進去（`objects.sql` 的 `effective_measures` 改成單一 SELECT 的原因）。詳情頁若推不進去，改成每種一個 `pub_*` 函式、`reviewqueue` 自己 `UNION ALL`。3d 設計 v3 先量再定。
- (e) 的五個缺口的修法只寫了方向，觸發時另做設計。
- Salesforce 與 Atlassian 的部分頁面是 SEARCH；台灣法定「3 日」的算法沒查條文（來函期限可由管理員改，不受影響）。

---

## 給 Will 的決定題

**Q1 待辦頁用哪一種做法？**
- 建議 (g)：seller 發布一個唯讀的 SQL 函式，算出「此刻所有待辦的種類、開始時間與期限」；待辦頁用一支 SQL 讀它排序。這是我們 ADR 已經定好的跨模組讀法（店家能不能曝光就是這樣做），也是業界在同一個 database 裡的做法（佇列是對資料的查詢）。不需要原設計的合併層與跨來源翻頁的特殊處理，也不改任何操作的寫入。A7 來的時候加一個分支。
- 會改寫：3cd D′1 與 A7-D8 的「httpapi 合併層」，改成這個做法；A7-1e 的範圍跟著改。
- 終局：要看「當時有沒有準時」的報表、有第二位營運人員、或待辦頁變慢時，改成另存每段待辦的紀錄（§2 已寫下要注意的五點），這個函式就是重算的來源。
- 另一個選項 (e)：現在就另存。好處是歷史；代價是約 35 個操作要改、佇列出錯會讓操作失敗，而且歷史現在沒有人讀。

3d v2 的 Q1（期限改成開始＋到期一組並改名）與 Q3（文案）不受影響，一起回覆即可；3d v2 的 Q2 不需要決定。

---

## 附錄 A：審查意見的處理

### 第一輪（v1 → v2）

v1 建議手寫轉移的另存表 (c)；第一輪的 blocker（寫入點約 55 個，v1 寫 25）與 10 個 major 讓 v2 改成 (e)。逐條的處理記在 v2（本檔的 git 歷史沒有 v2，摘要如下）：寫入點照 rv1 的表；店家關閉與恢復營業、措施的 `Update`／`End`、延後複審由重算處理；業界分兩層；重開的原因改寫；漏的 (d)、(e) 加入；發票改成 SELF-IMPOSED；gov ADR `:29`、`:113` 列入。

### 第二輪（v2 → v3）

| 審查 | 處理 |
|---|---|
| BLOCKER 漏了現行 ADR 的 `pub_*`（D3 修訂第 4 點、D4 (a)） | 加入 (g)，成為建議 |
| MAJOR 先鎖再算是假的（新週期的列鎖不到；串行化靠領域鎖） | §2 第 1 點（終局才做） |
| MAJOR 重算單位太細 | §2 第 2 點 |
| MAJOR 週期身分有洞（恢復營業、出現前改期） | §2 第 3 點 |
| MAJOR 核對會把時間造成的離開誤報成漂移 | §2 第 4 點 |
| MAJOR 漏了 A7-D8 (c)、A7-1e、D4 (a)、D3 第 3 點；rb R3 否決過另存 | 前提題、§1.2、Q1 列出；(g) 照 rb R3 與 D4 (a) |
| MAJOR 歷史的好處說過頭（沒存原因） | §2 第 5 點；§1.4 改寫 |
| MAJOR 把 (a′) 說成債不成立 | §1.4 改寫：(a′) 與 (g) 都不留債，(g) 比 (a′) 少一個合併層 |
| MAJOR 同步的理由弱、Q1 沒寫「佇列出錯讓操作失敗」 | Q1 寫明 (e) 的這個代價 |
| MAJOR 兩個「最後一個鎖」 | §2 最後一段 |
| minor（4 項：引用、白話、未驗證的效能、devseed） | 引用改正；Q1 改白話；效能列在 §4；devseed 只在 (e) 相關，留在 §2 |
