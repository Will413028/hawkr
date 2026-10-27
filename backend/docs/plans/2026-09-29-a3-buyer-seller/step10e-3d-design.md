# 10e 第 3d 刀：審閱政策、期限與待辦，實作設計 v3.1

> **狀態**：v3。Will 2026-10-20 對 `step10e-3d-queue-brief.md` v3 與本稿 v2 的決定題回「照建議」：待辦改用 `pub_*` 組合（queue brief Q1 (g)，取代 v2 的 httpapi 合併層，E3–E8、E14 與 §2、§3.2、§3.3、§6.3 依此改寫）；期限改成 `Deadline{start_time, due_time}` 一組並改名（v2 Q1 (a)）；§5 的文案全部照提案核准；v2 的 Q2（鍵在待辦期間會變）在 (g) 之下照 E9 的語意寫進設計，不再是決定題。v2 處理過一輪獨立審查（對照見附錄 A）；v3.1 處理開工前的 design-review（5 項，附錄 B），其中兩項交 Will，2026-10-20 照建議（文末「決定紀錄（v3.1）」）。

讀取的是 `local/10e` 的 `4cc2ec079`（與 main 相同），worktree 沒有未提交的改動［量測：`git status --short`］。盤點：`step10e-3d-inventory.md`（唯讀 subagent，逐條標量測／推論；本文引用時寫「盤點 §n」）。
SSOT：`step10e-3cd-brief.md`（下稱 3cd：D′1、D′5、D′6、D′11、§2 第 12、16、17 項、§4.4–§4.6；Will 2026-10-18 照建議）；`step10e-3-brief.md`（下稱 3b：D7 的形狀、D8 的值；Will 2026-10-16 照建議）；`step10e-reports-brief.md`（下稱 rb：R3 的種類與期限、§3「3cd §2 第 1 項：種類從 8 個變成 10 個」）；`step10e-3ca-design.md` §6 與附錄 D #2（下稱 3ca）；`step10e-3cb-design.md` §6、Q1 (a)、Q6 與各「實作時的修訂」（下稱 3cb）；ADR `2026-09-24-go-modular-monolith` 文末「修訂：跨模組讀取改用擁有者發布的 pub_*、D4 收窄」第 3 點（`:314`，下稱 D3 修訂）；〈API 設計規範（Connect）〉六、分頁；術語表。
路徑縮寫：`W` 是 worktree 根目錄；`S`＝`W/backend/internal/modules/seller`；`H`＝`W/backend/internal/httpapi`；`A`＝`W/admin/src`；`P`＝`W/api/hawkr`。
標記：**[量測]** 這次讀到原始碼或跑了指令；**[推論]** 由讀到的內容推得；**[外部]** 讀了外部來源。
判定原則同 3cb：上線前沒有相容性約束；每個取捨照業界從零的做法；只有基準定不了的產品選擇、或要改寫已核准決定的，列給 Will。

---

## 0. 前提題：這刀要改的程式之後會不會被重寫、搬走或刪除？

| 程式 | 之後的排程 | 判定 |
|---|---|---|
| `seller.pub_review_items(at)` 與每種的 `seller.review_<種類>(at)`、`reviewqueue` 模組 | 沒有。A7-1e 發布 `moderation.pub_review_items(at)`，`reviewqueue` 的查詢加一個 `UNION ALL` 分支（E12 改寫 A7-1e）。「有無期限」欄不做，第一個沒有期限的種類出現時再加（E3 v3.1） | 會留下 |
| 推導、不另存（另存 SLA 紀錄是終局） | 觸發（第一份要看當時是否準時的報表、第二位營運人員、page_size 50 的 p95 超過 300 ms）都沒有排程；觸發時 `pub_review_items` 就是重算與重建的來源，不丟 | 會留下（queue brief 結論） |
| 各資源的期限輸出（`Report`、`Case`、`MeasureRequest`、`FoodRegistration`、`KYCApplication`、`Measure`） | 沒有。10f 會把詢問的期限改成「從出現在賣家中心起算」（3cb §0.2），那是詢問自己的 `due_at`，不碰本刀的政策 | 會留下 |
| 發票申訴（invoice-service 的工作天 SLA，`SlaCountdown`） | 在 invoice-service，搬進單體的步驟沒有排定；本刀不碰。搬進來的那一步加一個來源 | 範圍外；邊界外的服務，不是本刀的重做 |
| admin 待辦頁的側欄位置 | A7-4b 加內容審核時移到「總覽」並改頁面說明（3cd D′1「跟著改的」） | 只動一行設定與一句說明，不是重做 |
| D′6 的用語 | 沒有 | 會留下 |

結論：本刀改的都是會留下的程式，照從零的最佳做法做（業界比對與選項在 `step10e-3d-queue-brief.md` v3，Will 2026-10-20 照建議）。

---

## 0.1 範圍

| 部分 | 內容 |
|---|---|
| 審閱政策 | seller 每種待辦一支 SQL 函式 `seller.review_<種類>(at)`，規則只寫這一份；待辦、資源的 `*_deadline`、兩個收件匣都讀它（E10） |
| 期限輸出 | 共用訊息 `Deadline{start_time, due_time}`；資源輸出各自的期限，只在義務還在時（3cd §2 第 12 項） |
| seller 的發布函式 | `seller.pub_review_items(at)`：11 種待辦（`UNION ALL` 11 個分支）的種類、對象、店家、開始與期限；成員 ⇔ Go 判定式的資料集測試；`pub.sql` 的形狀與契約測試 |
| `reviewqueue` 模組 | 新模組，依賴 seller；一支 SQL 讀 `pub_review_items`、依期限排序、keyset 分頁；沒有自己的表 |
| handler | `H/reviewqueue`：權限交集、page token（`rpc.FilterScope` 綁 kinds）、一次請求一個 `now`、N＋1 判斷下一頁 |
| 契約 | `P/admin/reviewqueue/v1alpha1`：`ReviewQueueAdminService.ListReviewQueue`、`ReviewQueueItem`、`ReviewQueueItemKind`（11 個值）；權限 `reviewqueue.items.list`（admin 方法 92 → 93）；只列呼叫者有讀取權的種類（E13） |
| admin | `/admin/queue` 待辦頁、側欄；既有頁面的期限（3cd §4.5 #120–126）；案件詳情的詢問錨點與來函詳情的對象錨點（待辦列要連得過去） |
| 用語 | D′6：「實名認證」→「商家認證」（獨立 commit） |
| 小修 | 3cd §2 第 17 項：措施清單沒有店名時退回措施 ID 的 bug |
| 文件 | runbook K18 與兩份 runbook 的期限數字；新 ADR（待辦以 `pub_*` 組合、另存 SLA 紀錄的觸發，E12）；3cd D′1、A7-D8、A7 計畫 A7-1e 的修訂段；3cd D′1 的舊引用改指 ADR 修訂段（G12）；設計書 32、seller `AGENTS.md`、CARRYOVER（食安 SLA 那一列由審閱政策取代，3ca 附錄 D #3）、術語表 |

## 0.2 不在範圍

- 發票申訴的工作天 SLA（invoice-service；見前提題）。
- 指派、認領、暫停時鐘、升級（3b D11；觸發條件在 E12）。
- 期限逾期的通知（推播、email）：上線後有營運人員時才加（violation runbook `:139`）。
- 轉報的追蹤日期：rb R5「只顯示在案件上，不進待辦」（3cb B14）。
- 期限值的校準：照 rb R3、3b D8 的觸發（上線後累積 20 筆結案，取 P90）。

---

## 0.3 決策總表

| # | 題目 | 決定 | 依據 |
|---|---|---|---|
| E1 | 種類 | 11 種（rb §3 的 10 種＋3cb Q1 (a) 的「告知被指名者」），表見 §1.3；篩選、標籤、審閱政策的函式一對一 | rb R3、§3；3cb §6、Q1 (a) |
| E2 | 期限的形狀與欄位名（**改寫已核准，Q1**） | 共用訊息 `hawkr.common.v1alpha1.Deadline{start_time, due_time}`，兩個都是 REQUIRED；資源以 `<義務>_deadline` 欄位輸出，沒有義務時不設。既有的三個 `*_due_time` 一起換（§1.2）。**現有機制為什麼是那樣**：3b D8 核准的是平的 `*_due_time`（`MeasureRequest.decide_due_time` 等），當時期限的開始時間 client 都拿得到；3ca 附錄 D #2 之後待決定的開始時間只有伺服器知道（`DecisionClockStart`），食品業者登錄的開始時間也要 JOIN 商家認證的核准時間，所以必須多給開始時間。選項：(a) 訊息、全部換名（建議：一種形狀，client 一支 helper）；(b) 平的 `*_due_time` 加平的 `*_start_time`（欄位數加倍，兩者一起有值的規則要逐欄寫）；(c) 只在開始時間是伺服器推導的資源用訊息，其他維持 `*_due_time`（兩種形狀並存） | 3ca 附錄 D #2（「例如期限訊息 `{due_time, start_time}`」）；JSM 的 SLA 是 `ongoingCycle{startTime, breachTime, …}` 一組［外部，`breachTime` 3b D8 已查證，`startTime` 沒有重新查證］；約束帳：3b D8 的欄位名是 SELF-IMPOSED（我們的設計），上線前沒有相容性約束 |
| E3 | 「有無期限」欄（**v3.1：不做**，Will 2026-10-20 Q-B） | 11 種都有期限，`ReviewQueueItem.deadline` REQUIRED、`due_at` NOT NULL。觸發：A7-1e 出現第一個沒有期限的種類時，函式加一欄、`reviewqueue` 的排序與游標各改一處、`deadline` 改選填 | necessity；開工前 design-review #4 |
| E4 | 全序與讀法（**v3 改寫**） | seller 發布 `seller.pub_review_items(at)`（11 種待辦一個函式，§2.1）；新模組 `reviewqueue` 在自己的 SQL 讀它並排序：`ORDER BY due_at, start_at, kind COLLATE "C", item_id COLLATE "C"`，keyset 以同樣的 row-value 比較（§2.2）。時間由請求的 clock 傳入，截到 µs | D3 修訂第 4 點、D4 (a)；queue brief Q1 (g)；先例 `seller.pub_exposable_stores(at)` |
| E5–E7 | （v2 的 rank、`SeekFor`、N＋1 合併） | **刪除**：一支 SQL 的 `ORDER BY … LIMIT N+1` 取代 | queue brief §3 |
| E8 | 一致性 | 一支 SQL 語句就是一個快照；不需要 `InReadSnapshot`、不需要在 httpapi 注入 `*db.DB`。整個請求一個 `now` | PG 的語句快照（READ COMMITTED 下每個語句一個快照） |
| E9 | 留在待辦期間 key 會變的情形 | 有 3 種（盤點更正了 3cd 的「8 種只有 KYC」；KYC 反而不會：送出後不能編輯，重送是離開再進入）：改案件嚴重度、延後複審、暫不告知換成仍已到期的確認日期。都是營運自己的指令。**接受**，語意同 Elasticsearch 不配 PIT 的 `search_after`：翻頁途中被自己改過 key 的那一列，可能在下一頁重複出現或不出現，重新整理就正確。待辦頁在任何對話框或詳情頁回來時回到第一頁。**終局與觸發**：有第二位營運人員（別人改的 key 會在你翻頁時移動），或出現要凍結 key 的需求時，改成實體化（E12 的觸發之一） | 盤點 §1 的 2、6、9、11；Q2 |
| E10 | 審閱政策的家（**v3.1 改寫**，開工前 design-review #1、#2、#3） | **規則只有一個家，在 SQL**（ADR 2026-09-24 D3 修訂第 4 點：「規則只有一個家，擁有者的 Go 程式讀同一份 SQL」；queue brief §3 E2）：seller 內部每種待辦一支函式 `seller.review_<種類>(at)`（`db/objects.sql`，不是 `pub_*`），回對象的鍵、店家、開始與期限；`pub_review_items(at)` 是它們的 `UNION ALL`。Go 不寫時長：資源的 `*_deadline` 由 seller 以對象 ID 批次讀同一支函式（`WHERE id = ANY($ids)`，函式是單一 SELECT、會被內聯，條件推得進去，同 `effective_measures` 的做法）；措施清單「複審到期」分頁與食品業者登錄「待審核」分頁的成員也改讀同一支函式，不再各寫一份條件。`FoodSafetyTriageSLA`、`InquiryOverdueGrace`、`AuthorityNoticeFollowUpGrace`（含 `seller/governance.go:266`、`:293` 的重新匯出）刪除。測試：每種一張表格，逐種斷言函式算出的開始與期限等於 §1.3 的值（**mutation**：任一分支的時長改掉要失敗）。指令用的 Go 判定式（`AwaitingTriage` 等）保留（`Availability` 要用），成員與函式一致由資料集測試鎖住（3c-a 起的既有做法） | ADR 2026-09-24 `:315`；先例 `seller.pub_exposable_stores` 與 `exposure.Classify`（Go 讀 SQL）、`effective_measures` 的內聯 |
| E11 | 食品業者登錄的成員與起算 | 成員只有「可以裁定」的（`DECIDABLE`：待審、店家營業中、商家認證已核准）；開始時間 `max(送出時間, 目前核准的商家認證的 decided_at)`；等商家認證的不進待辦 | 3b D8「從可以裁定時起算」；盤點 §1.10。KYC 被退回草稿時登錄回到等商家認證、離開待辦，再核准時重新進入，不是留在待辦期間換 key |
| E12 | 文件與 ADR（**v3 改寫**） | 不改 ADR 2026-09-24（照第 4 點做，不需要例外）；寫一份 ADR「後台待辦以擁有者發布的 `pub_review_items` 組合，期限推導不另存；另存 SLA 紀錄的觸發」（終局的五點與觸發照 queue brief §2、結論）；3cd D′1、A7-D8、A7 計畫 A7-1e（`:321`、`:541`）寫修訂段：「A7-1e 發布 `moderation.pub_review_items(at)`，`reviewqueue` 加一個分支」 | queue brief；ADR gate |
| E13 | 權限 | `reviewqueue.items.list`，`adminperm` 檔頭補「跨模組的讀取組裝以組裝的名稱為前綴（`legacy` 是先例）」。照 3cd D′1「只給呼叫者有讀取權的來源」做，粒度細到種類：`reviewqueue` 的 Go 有一張種類 → 既有讀取權限的表（通報與案件、來函、措施、食品業者登錄、商家認證各自的 list 權限；實作時以 `adminperm` 的常數為準），handler 把請求的 kinds 與呼叫者有權的 kinds 取交集後傳進 SQL（`kind = ANY(@kinds)`）。現在只有 `OWNER`，交集等於全部 | 3cd D′1 `:127`；審查 MAJOR 8 |
| E14 | page token | `rpc.FilterScope("ListReviewQueue", 排序並去重後的請求 kinds)` 綁住篩選；key 是 `(due_at, start_at, kind, item_id)`。page_size 預設 50、上限 100。kinds 是空的時候展開成「呼叫者有權的全部種類」 | API 規範六；3cd D′1 |
| E15 | 待辦列的對象 | `oneof subject`：`report_id`｜`case_id`｜`CaseInquiryRef{case_id, inquiry_id}`｜`authority_order_id`｜`AuthorityTargetRef{order_id, position, account_id}`｜`MeasureRequestRef{measure_id, request_id}`｜`measure_id`（複審）｜`food_registration_id`｜`KYCApplicationRef{store_id, application_id}`。店家是 `string store_id`（空字串表示沒有，同 `AuthorityOrderTarget`）＋`store_name`（來函沒有單一店家；A7 的項目沒有店家）。`account_id` 給 #113 的「帳號 {id}」 | rb R3「契約」；3cd D′1「店家欄位是選填」；API 規範「ID 是 string」；`authority_order.proto:140`；審查 MAJOR 5、6 |
| E16 | 詢問逾期列的期限 | 待辦列的期限是 `due_at + 24h`（營運要處理逾期的時間），開始時間是 `due_at`；詢問本身的 `reply_deadline` 是賣家的期限（`create_time` 到 `due_at`）。兩個是不同的義務 | 3cb §6 第 2 點 |
| E17 | D′6 | 3d-0a 獨立 commit；範圍照 3cd §4.6 逐行表，以 3cd 的量法 `rg -n "實名認證\|身分認證"`（不只「實名認證」：admin 另有 15 行「身分認證」，含 #139–141）逐行分類：UI 字串照表改、測試跟著改、法律文件與個資說明的「身分認證」不動 | 3cd D′6 `:214`、`:227`；盤點 §13；審查 minor |

---

## 0.4 查證過的現況

| # | 事實 | 證據 |
|---|---|---|
| G1 | 程式裡沒有 `ReviewPolicy`；期限常數散在：`FoodSafetyTriageSLA` 4h（`S/internal/model/report_case.go:131`）、`InquiryDefaultDue` 72h、`InquiryOverdueGrace` 24h（`case_inquiry.go:79`、`:84`）、`DefaultAuthorityDue`（`authority_order.go:36`）、`AuthorityNoticeFollowUpGrace` 72h（`:73`）；一般通報 72h、待決定、轉報、申覆、解除請求、食品業者登錄、商家認證的值只在文件 | [量測] `rg -n "ReviewPolicy" backend -g '!*.pb.go'` 只命中 docs；盤點 §3 |
| G2 | 既有的期限輸出只有三個：`Report.triage_due_time`（只食安、只待分案，`P/admin/seller/v1alpha1/governance.proto:390`）、`CaseInquiry.due_time`（只 OPEN，`case_inquiry.proto:61`）、`AuthorityOrder.due_time`（只 OPEN，`authority_order.proto:109`）；`Measure.review_by` 是事實欄位，撤銷後仍輸出 | [量測] 盤點 §2、§1.9 |
| G3 | 判定式：`Report.AwaitingTriage()`（`report_case.go:208`）、`Case.AwaitingDecision()`／`DecisionClockStart()`（`:348`、`:351-356`）、`CaseFacts.AwaitingReferral()`（`:540`）、`CaseInquiry.Overdue(now)`（`case_inquiry.go:136`）、`AuthorityOrder.Open()`（`authority_order.go:241`）、`AuthorityOrderTarget.NoticeFollowUpDue(now)`／`RecheckStart()`（`:202`、`:192`）、`Measure.Pending()`／`PendingRemediation()`（`measure.go:582`、`:592`）、`FoodRegistrationReview.CanApprove()`（`foodreg.go:195`）、KYC `status = SUBMITTED`（`kyc.go:26`）。複審沒有 Go 判定式，只有 SQL `AdminMeasuresReviewDue`（`measures.sql:421-459`，`review_by <= now`） | [量測] 盤點 §1 |
| G4 | 篩選用的部分索引（多數註解寫「3d 來源」）：`reports_awaiting_triage`、`cases_open`、`case_inquiries_one_open`、`case_inquiries_open_due`、`cases_decided_food_safety`、`authority_orders_open_due`、`authority_order_targets_withheld`、`measure_requests_pending`、`measures_review`、`food_registrations_inbox`、`kyc_applications_status_submitted`；`case_referrals` 的 `case_id` 是主鍵。排序用的是算出來的期限與 `item_id::text`，**沒有一個分支能靠索引排序**：索引只幫篩選，每個分支取出未結的成員後排序（未結的項目量小，§6.5 量測） | [量測] `S/db/schema.sql`，行號見盤點 §1；`:421-422`；審查 minor |
| G5 | 措施的「生效中」現在在 Go 算（`stint.ActiveMeasureIDs`，`S/internal/measure/list.go:56-65`）再 `= ANY`；SQL 函式 `seller.effective_measures(at)`（`objects.sql:44`）已被 `EffectiveMeasuresAll` 用 | [量測] 盤點 §15.4 |
| G6 | 分頁工具在 `H/rpc/paging.go`：`EncodePageToken`、`ValidCursor[T]`、`TokenFor`、`FilterScope(procedure, filters...)`（`:20-89`）；platform 沒有 keyset 工具；`InReadSnapshot`（`W/backend/internal/platform/db/db.go:149`）在 httpapi 沒有呼叫者 | [量測] 盤點 §7 |
| G7 | admin service 的掛載要求名稱 `hawkr.admin.*` 且以 `AdminService` 結尾（`H/rpc/rpc.go:46`），宣告與掛載不符無法啟動（`H/server.go:294`、`:326-332`）；admin 方法數 92 斷言在 `H/identity/admin_roles_integration_test.go:80` 與 `H/adminauth/adminauth_test.go:65`；`maxRESTOperations` 481 | [量測] 盤點 §8 |
| G8 | admin：`A/lib/deadline.ts` 的 `deadlineDisplay({due, start, now})` 與 `DeadlineText`、`useNow`（3c-a 建立）；`keyset-pages.ts` 與 `authority-orders/page.tsx` 的分頁、30 秒更新（`LIST_REFRESH_MS`）可照抄；`/admin/queue` 不存在；側欄「賣家 / 商品」在 `A/components/Layout/Sidebar.tsx:47` | [量測] 盤點 §9 |
| G9 | 3cd §4.5 的位置現在都沒有期限：複審到期分頁是「複審期限 {日期}」（`A/app/admin/measures/measure-inbox.ts:137-143`）、措施詳情只有提出時間（`MeasureSections.tsx:77`）、食品業者登錄清單與詳情只有送出時間、商家詳情的 KYC 只有提交時間（`merchants/[id]/page.tsx:931-937`） | [量測] 盤點 §10 |
| G10 | 案件詳情的詢問只有 `data-inquiry-id`，沒有 `#inquiry-{id}` 錨點（`CaseInquiries.tsx:68`）；來函詳情的對象沒有錨點 | [量測] 盤點 §1.3、§1.6 |
| G11 | runbook：食安 `:160`「申訴 14 天…待 3d 校準（K18）」、`:5` 分案「一般 72 小時」、`:395`「案件的決定期限與待辦頁在 3d」；violation runbook `:119`「申訴…**7 個工作日內**」、`:153-155`「若 7 工作日內未 decide」、`:139` 補正「7 個日曆天」、`:20`「之後補在本文」；食品業者登錄 runbook `:24`（「比照措施補正」，要補「從可以裁定時起算」） | [量測] `rg -n '工作天\|工作日\|14 天' backend/docs/runbooks/*.md`（v1 只搜「工作天」，漏了 `:119`、`:155`，審查 minor） |
| G12 | ADR 2026-09-24 `:304` 的修訂段第 3 點（`:314`）已是「各來源都擁有全部篩選與排序鍵：在 httpapi 依同一個全序做 keyset composition」，`backend/AGENTS.md:28` 同步；3cd 的 `:19`、`:61`、`:75`、`:92`、`:99-100`、`:135`、`:344`、`:580` 還寫成「偏離、待決定、ADR 用這裡的文字」 | [量測] 盤點 §12 |
| G13 | D′6 還沒做：`rg -n "實名認證"` 在 admin 11、website 41、seller-app 6、目錄 2 行 | [量測] 盤點 §13 |
| G14 | 措施清單店家欄 `{m.storeName \|\| m.id}`（`A/app/admin/measures/page.tsx:151`）退回措施 ID；`Measure.store_id` 有值 | [量測] 盤點 §14 |
| G15 | id 的型別：通報、案件、詢問、來函、措施、措施請求、食品業者登錄、商家認證申請都是 UUID；來函對象是 `(order_id UUID, position SMALLINT)`；店家是 BIGINT | [量測] `S/db/schema.sql:631-635`、`:904-906`、`:1016-1018`；盤點 §15.2 |

## 0.5 約束帳（本刀新出現的）

| 約束 | 判定 | 來源 |
|---|---|---|
| 只有一位營運人員，沒有指派、認領 | **REAL**（缺第二方）：所以不實體化 | 3cd D′1 |
| 期限值沒有資料 | **REAL**（從未上線）：用 rb R3、3b D8 的起始值，觸發後校準 | rb R3 |
| 工作天行事曆不在單體 | SELF-IMPOSED，但日曆天的決定是 Will 2026-10-15 的理由（「免去假日曆」），照用 | 3b D8 (d) |
| A7 的種類有沒有期限 | 未定（A7 的設計還沒寫到）：所以照 D′1 保留旗標（E3），不賭它 | A7 計畫 `:321-326` |
| 3b D8 的欄位名 `*_due_time` | SELF-IMPOSED（我們的設計）；改名交 Will（Q1） | 3b D8 |
| 3cd D′1 的「8 種只有 KYC 會換 key」 | SELF-IMPOSED，而且不準（E9） | 盤點 §1 |
| 單體規則 2：SQL 只碰自己的 schema；規則 3：被別的模組呼叫的方法不開快照 | SELF-IMPOSED，符合基準，保留：`reviewqueue` 只讀依賴表允許的 `seller/db/pub.sql`，一支語句就是一個快照 | `backend/AGENTS.md` |
| 「先重用既有元件與 pattern」 | SELF-IMPOSED，符合基準，保留：`DeadlineText`、`keyset-pages`、清單頁骨架 | 根目錄 UI 規則 |
| 發票申訴在 invoice-service | **REAL**（邊界外的服務，搬遷步驟沒有排定，現在改它是拋棄式的） | 3b D7 約束帳 |

---

## 1. 契約

### 1.1 `Deadline`（`P/common/v1alpha1/deadline.proto`，新）

```proto
// 一個有期限的義務：從 start_time 起算，到 due_time 到期。兩者都由伺服器給：依審閱政策算出，
// 或是存下的期限（例如管理員建立詢問時指定的）。client 只拿現在和它們比較（剩多少、是否逾期、
// 色階），不重建起算規則。
message Deadline {
  google.protobuf.Timestamp start_time = 1 [(google.api.field_behavior) = REQUIRED];
  google.protobuf.Timestamp due_time = 2 [(google.api.field_behavior) = REQUIRED];
}
```

放在 common：admin 的 seller 與 reviewqueue 兩個 package 都用，10f 的賣家端詢問期限也會用（賣家看得到自己的回覆期限）。common 不在 Dart 生成端的排除清單裡，所以 `packages/hawkr_api` 也會生成它（三個生成端的 drift 都要跑）；API 規範 §一列舉 common 訊息的那一句加上 `Deadline`。建立請求的輸入欄位（`CreateCaseInquiryRequest.due_time`、來函的 `due_time`）不改名：它們是一個時間點，不是一組期限。

### 1.2 資源的期限輸出（全部 `OUTPUT_ONLY`，只在義務還在時設）

| 資源 | 欄位 | 何時有值 | 開始 | 期限 | 取代 |
|---|---|---|---|---|---|
| `Report` | `triage_deadline` | 待分案 | `create_time` | ＋4h（通報人選了食安）／＋72h | `triage_due_time`（改成所有類別都有） |
| `Case` | `decide_deadline` | `AwaitingDecision()` | `DecisionClockStart()` | ＋依嚴重度：嚴重 24h、高 72h、中 7d、低 14d | 新 |
| `Case` | `fda_referral_deadline` | `AwaitingReferral()` | `ClosedAt` | ＋24h | 新 |
| `CaseInquiry` | `reply_deadline` | OPEN | `create_time` | `due_at`（賣家的期限，存的） | `due_time` |
| `AuthorityOrder` | `deadline` | OPEN | `receive_time` | `due_at`（存的） | `due_time` |
| `MeasureRequest` | `decide_deadline` | 推導的 state 是 PENDING | `submit_time` | ＋7d（申覆與解除請求一樣） | 新 |
| `Measure` | `review_deadline` | RISK、生效中、有 `review_by` | `review_by − 24h` | `review_by` | 新（`review_by` 照舊是事實欄位） |
| `FoodRegistration` | `decide_deadline` | 可以裁定（E11） | `max(送出, 商家認證核准)` | ＋7d | 新 |
| `KYCApplicationSummary` | `decide_deadline` | `SUBMITTED` | `submit_time` | ＋7d | 新（3b D8 寫在 `KYCApplication`；商家詳情的 KYC 區讀的是摘要 `kycSummary`，`merchants/[id]/page.tsx:931-937`，完整的 `KYCApplication` 沒有要顯示期限的消費者，所以只放摘要） |

- 「告知被指名者」與「詢問逾期」不在資源上輸出期限：只有待辦用到（necessity：沒有詳情頁要顯示）。觸發：詳情頁要顯示「重新確認的期限」或「跟進逾期的期限」。
- 欄位名去掉 `_time`：值是訊息，不是時間（AIP-142 的 `_time` 只用在 `Timestamp`）。
- 引用舊欄位名的文件：A7 計畫 `:271`、`:547`（`*_due_time`），3d-4 跟著改。
- 既有三個欄位的消費者：admin 的 `report-list.ts:200-209`、`violation-reports/[id]/page.tsx:353`、`authority-orders/page.tsx:158`、`[id]/page.tsx:355`、`CaseInquiries.tsx:97`［量測，盤點 §10］；實作時以 `rg -n 'triageDueTime|dueTime' admin/src website/src`（後者預期 0）重查一次。website 與兩支 App 沒有呼叫這三個 admin message［推論：它們在 `admin/seller` package］，實作時 `rg` 確認。

### 1.3 `ReviewQueueAdminService`（`P/admin/reviewqueue/v1alpha1/review_queue.proto`，新）

```proto
service ReviewQueueAdminService {
  // 後台待辦：所有模組中等待營運處理的事項，依期限由急到緩。項目由各模組的領域狀態推導，
  // 不是儲存的資源，所以沒有 Get。
  rpc ListReviewQueue(ListReviewQueueRequest) returns (ListReviewQueueResponse);
}

message ListReviewQueueRequest {
  int32 page_size = 1;   // 0 用預設 50；超過 100 以 100 計
  string page_token = 2; // token 綁住 kinds
  repeated ReviewQueueItemKind kinds = 3; // 任一相符就列出，空的不篩（API 規範的 repeated 篩選慣例，3cd D′2）
}

message ListReviewQueueResponse {
  repeated ReviewQueueItem items = 1;
  string next_page_token = 2;
}

message ReviewQueueItem {
  ReviewQueueItemKind kind = 1 [(google.api.field_behavior) = REQUIRED];
  oneof subject { ... }  // E15
  string store_id = 20;   // 空字串表示這個項目沒有單一店家
  string store_name = 21; // 沒有店家或店名已清理時空字串
  hawkr.common.v1alpha1.Deadline deadline = 22 [(google.api.field_behavior) = REQUIRED];
}
```

`ReviewQueueItemKind`（排序是 proto 的編號，不是 rank；rank 在來源裡）：

| 值 | 種類（rb R3） | 對象 | 成員（Go 判定式） | 開始 | 期限 |
|---|---|---|---|---|---|
| `REPORT_TRIAGE` | 待分案 | 通報 | `Report.AwaitingTriage()` | 建立 | ＋4h（食安）／72h |
| `CASE_DECISION` | 待決定 | 案件 | `Case.AwaitingDecision()` | `DecisionClockStart()` | ＋依嚴重度 |
| `CASE_INQUIRY_OVERDUE` | 詢問逾期 | 詢問 | `CaseInquiry.Overdue(now)` | `due_at` | ＋24h |
| `CASE_FDA_REFERRAL` | 待轉報 | 案件 | `CaseFacts.AwaitingReferral()` | 決定（`ClosedAt`） | ＋24h |
| `AUTHORITY_ORDER` | 主管機關來函 | 來函 | `AuthorityOrder.Open()` | 收文 | `due_at` |
| `AUTHORITY_TARGET_NOTICE` | 告知被指名者 | 來函的對象 | `NoticeFollowUpDue(now)` | `RecheckStart()` | ＋72h |
| `MEASURE_APPEAL` | 裁定申覆 | 措施請求 | `Measure.Pending()` 且種類是申覆 | 提出 | ＋7d |
| `MEASURE_LIFT_REQUEST` | 裁定解除請求 | 措施請求 | `Measure.PendingRemediation()` | 提出 | ＋7d |
| `MEASURE_REVIEW` | 複審停權 | 措施 | `review_measure_review(at)`：RISK、生效中、`at >= review_by − 24h`（沒有 Go 判定式，§3.1） | `review_by − 24h` | `review_by` |
| `FOOD_REGISTRATION` | 審核食品業者登錄 | 登錄 | `FoodRegistrationReview.CanApprove()` | E11 | ＋7d |
| `KYC_APPLICATION` | 審核商家認證 | 申請 | `status = SUBMITTED` | 送出 | ＋7d |

- `MEASURE_REVIEW` 的判定式從 `review_by <= now` 改成提前 24 小時（3cd D′11），措施清單的「複審到期」分頁用同一個判定式（`AdminMeasuresReviewDue` 跟著改，資料集測試鎖住清單與待辦一致）。
- 權限 `reviewqueue.items.list`；`H/adminauth` 的宣告表加一列；admin 方法 93（兩處斷言，G7）。

### 1.4 錯誤

不新增 `ErrorReason`。`page_size` 負數、token 解不開或 kinds 與 token 不符回 `INVALID_ARGUMENT`（API 規範六，句子沿用 `rpc` 既有的分頁錯誤）。`kinds` 含不是已定義的值（`UNSPECIFIED`，或 proto3 開放 enum 收到的未知數字）回 `INVALID_ARGUMENT`，句子用 httpapi 既有的通用欄位驗證句（實作時以 `rg -n 'InvalidInputOn' backend/internal/httpapi` 找到現行寫法；不在 handler 寫中文字面值，3cd §2 第 15 項）。呼叫者沒有任何種類的讀取權時回空清單，不回 `PERMISSION_DENIED`（`reviewqueue.items.list` 已經過關，種類的權限只決定看得到哪些）。

---

## 2. Schema 與查詢

不新增表與欄位。新增的 DB 物件只有 seller 的一個複合型別與一個函式（手寫 migration，`atlas migrate new`；`db/objects.sql` 放定義、`db/pub.sql` 宣告形狀，同 `pub_exposable_stores`）。可能新增的索引要量測之後才加（§6.5）。

### 2.1 `seller.pub_review_items(at)`（`S/db/objects.sql`、`S/db/pub.sql`、migration）

```sql
-- 發布的讀取介面：at 這一刻營運要處理的事項（後台待辦的 seller 部分）。每一列是一件待辦：種類、對象、店家、開始時間與期限。
-- 每種一支 seller.review_<種類>(at) 的 UNION ALL（§3.1）；成員的條件與 model 的判定式同義，由資料集測試鎖住（E10）。時間由呼叫方傳入，不用 now()。
CREATE TYPE seller.pub_review_item AS (
    kind TEXT,                    -- REPORT_TRIAGE、CASE_DECISION、…（§1.3 的 11 個值，UPPER_SNAKE）
    item_id TEXT,                 -- 同一種類內唯一、位元組順序穩定：UUID 的 ::text；來函對象是 order_id::text || '/' || lpad(position::text, 5, '0')
    report_id UUID, case_id UUID, inquiry_id UUID, authority_order_id UUID, target_position SMALLINT, account_id BIGINT,
    measure_id UUID, request_id UUID, food_registration_id UUID, kyc_application_id UUID,
    store_id BIGINT, store_name TEXT, -- 沒有單一店家時 NULL；店名已清理時空字串
    start_at TIMESTAMPTZ, due_at TIMESTAMPTZ
);
CREATE FUNCTION seller.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF seller.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
  -- REPORT_TRIAGE：r.case_id IS NULL AND r.outcome IS NULL；due = created_at + (食安 4h｜其他 72h)
  SELECT ... FROM seller.reports r JOIN seller.stores s ON ...
  UNION ALL
  -- CASE_DECISION：進行中、沒有還沒結束的詢問；start = greatest(opened_at, 最後一筆回覆收到時間或 ADMIN 撤回時間)（同 CaseDecisionClockStarts）；
  --                due = start + CASE severity（嚴重 24h、高 72h、中 7d、低 14d）
  ...（其餘 9 種照 §1.3 的表；措施的「生效中」用 seller.effective_measures(at)）
END;
```

- 11 種都有期限，`due_at` 一律有值（E3）。
- 店名：店家個資已清理（第二段）時是空字串，client 顯示「店家 {id}」（§5 #113）。

### 2.2 `reviewqueue` 的查詢（`backend/internal/modules/reviewqueue/db/queries/items.sql`）

```sql
-- name: ListItems :many
-- 後台待辦：各來源發布的 pub_*_review_items 依期限由急到緩。現在只有 seller；A7-1e 加 moderation 的分支。
SELECT * FROM (
  SELECT * FROM seller.pub_review_items(sqlc.arg(now)::timestamptz)
) q
WHERE q.kind = ANY(sqlc.arg(kinds)::text[])
  AND (q.due_at, q.start_at, q.kind COLLATE "C", q.item_id COLLATE "C")
    > (sqlc.arg(after_due)::timestamptz, sqlc.arg(after_start)::timestamptz,
       sqlc.arg(after_kind)::text COLLATE "C", sqlc.arg(after_id)::text COLLATE "C")
ORDER BY q.due_at, q.start_at, q.kind COLLATE "C", q.item_id COLLATE "C"
LIMIT sqlc.arg(lim);
```

- 第一頁沒有游標（四個 `after_*` 都是 NULL），照 repo 慣例用 `sqlc.narg` 加 `IS NULL OR`（終審 3d-5 改：這支查詢本來就沒有索引能用，哨兵值的理由不成立）。
- 模組：`make new-module NAME=reviewqueue`（`DOCKER_CONTEXT=desktop-linux`）；依賴表 `"reviewqueue": {"seller"}`；`sqlc.yaml` 的 schema 是自己的 `schema.sql` 加 `../../seller/db/pub.sql`（先例：讀 `pub_exposable_stores` 的模組，實作時以 `rg -l 'seller/db/pub.sql' backend/internal/modules` 找）。`reviewqueue` 自己沒有表，`schema.sql` 只有 `CREATE SCHEMA`（實作時確認 `make new-module` 與 Atlas 對空 schema 的處理）。
- 公開方法 `reviewqueue.Service.List(ctx, now, kinds, after, limit)` 跑在 `db.Conn(ctx)`。

### 2.3 不另存

期限與成員都在讀取時推導（設計書 32、3b D8，queue brief Q1 (g)）；另存 SLA 紀錄的觸發見 E12 的 ADR。

---

## 3. 模組

### 3.1 審閱政策：每種一支 SQL 函式（`S/db/objects.sql`，新；v3.1 取代 Go 的 `ReviewPolicy`）

```sql
-- 營運處理「待分案」的期限：通報人選了食安 4 小時，其他 72 小時；一律日曆時間、不暫停（3b D8）。值是上線前的起始值，
-- 上線後依 rb R3 的觸發校準。待辦（pub_review_items）與資源的 triage_deadline 都讀這一支，規則只有這一份。
CREATE FUNCTION seller.review_report_triage(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT ... FROM seller.reports r WHERE r.case_id IS NULL AND r.outcome IS NULL;
END;
-- 其餘 10 種同形：review_case_decision、review_case_inquiry_overdue、review_case_fda_referral、review_authority_order、
-- review_authority_target_notice、review_measure_appeal、review_measure_lift_request、review_measure_review、
-- review_food_registration、review_kyc_application。
```

- `seller.review_item` 是 seller 內部的複合型別（同 `pub_review_item` 的欄位；`pub_review_items` 只是它們的 `UNION ALL`，形狀宣告在 `pub.sql`）。
- 每支都寫成**單一 SELECT**（不在函式內 `UNION ALL`），讓呼叫方的 `WHERE id = ANY($ids)` 推得進去（`effective_measures` 改成單一 SELECT 的同一個理由，`objects.sql:40-43`）；以 `EXPLAIN` 鎖住（先例 `TestExposableStoresJoinProbesMeasuresByAccount`）。
- seller 的 Go：`internal/governance`、`measure`、`foodreg`、`kyc` 各加一支「依 ID 批次讀期限」的 sqlc 查詢（`SELECT … FROM seller.review_<種類>(@at) WHERE … = ANY(@ids)`），emitter 把結果填進 `*_deadline`。清單（`ListReports`、`ListCases`、`ListMeasures` 的待處理請求、`ListFoodRegistrations`、`ListKYCApplications`）一頁一次批次讀；詳情一次讀一筆。
- Go 不留成員判定式：`Report.AwaitingTriage`、`Case.AwaitingDecision`、`CaseFacts.AwaitingReferral`、`CaseInquiry.Overdue`、`AuthorityOrderTarget.NoticeFollowUpDue` 在正式程式沒有呼叫者（終審 3d-5 以 `rg` 量測），已刪除；指令用的是 `Report.State()`、`Case.Open()`、`CaseFacts.Availability()`、`CaseInquiry.Open()`、`Measure.Pending` 等（有呼叫者，照舊）。成員與函式一致由資料集測試拿函式的成員比對這些指令用的定義。新的 `Measure.ReviewDue` **不做**：複審沒有指令要用它，措施清單的「複審到期」分頁直接讀 `review_measure_review(at)`。
- 刪除：`FoodSafetyTriageSLA`、`Report.TriageDueAt`、`InquiryOverdueGrace`、`AuthorityNoticeFollowUpGrace`、`RecheckStart` 與它們在 `seller/governance.go:266`、`:293` 的重新匯出（實作時以 `rg` 找全部消費者）。`InquiryDefaultDue`、`DefaultAuthorityDue` 不動：它們是賣家或機關期限的預設值，不是營運的審閱政策。
- 收件匣的成員也只有一份：`AdminMeasuresReviewDue` 改成讀 `review_measure_review(@now)`；`AdminFoodRegistrationInbox` 的 `DECIDABLE` 分頁讀 `review_food_registration(@now)`（`AWAITING_KYC` 分頁照舊）。

### 3.2 `reviewqueue` 模組與 handler（`backend/internal/modules/reviewqueue`、`H/reviewqueue`，新）

- 模組：`Item`（種類、對象、店家、`Start`、`Due`）、`Cursor{Due, Start, Kind, ID}`（`Valid()`，時間截到 µs）、`List(ctx, now, kinds []Kind, after *Cursor, limit int32)`。種類是模組的 Go 型別（UPPER_SNAKE 字串，與 `pub_review_items.kind` 相同）；模組的測試斷言 SQL 回的每個 `kind` 都是已知的值。
- handler（`H/reviewqueue/review_queue.go`）：驗 page_size 與 kinds → 與呼叫者有權的種類取交集（空的請求展開成全部有權的）→ `ValidCursor` 解 token（scope 是 `FilterScope` 綁**請求的** kinds）→ 注入的 clock 取 `now` → `List(limit = N+1)` → 前 N 筆轉成 proto（`subject` 依種類組、`deadline`）→ 有第 N＋1 筆時 `TokenFor(scope, 第 N 筆的 cursor)`。
- 呼叫者的權限：用既有的 `rpc.PrincipalFrom(ctx)` 與 `Principal.AdminAllows(perm)`（interceptor 與 `adminauth.GuardFile` 用的同一組，`H/adminauth/adminauth.go:196-210`、`files.go:47-55`），不另加 API（開工前 design-review #5）。
- `server.go` 掛上 `ReviewQueueAdminService`；`Deps` 加 `ReviewQueue`。

---

## 4. admin

### 4.1 `/admin/queue`（新）

- `A/app/admin/queue/page.tsx`：仿 `authority-orders/page.tsx`（`keyset-pages.ts`、`useQuery(...ListReviewQueue, req, {refetchInterval: LIST_REFRESH_MS})`、分頁列）；篩選是單選下拉（「所有種類」＋11 種），送成 `kinds: [選的那個]`；換篩選回第一頁。
- 欄位：種類｜店家｜開始時間｜期限。期限用 `DeadlineText`（`deadlineDisplay({due, start, now})`，`useNow(LIST_REFRESH_MS)`），`title` 是絕對時間；開始時間 `formatDateTimeShort`。
- 種類標籤是 admin 的單端文字表（3cd §2 第 4 項：只有 admin 一端用的 enum），以 `Record<ReviewQueueItemKind, string>` 寫，`UNSPECIFIED` 不顯示。
- 列的連結（`A/app/admin/queue/queue-item.ts` 的 `itemHref`，單元測試逐種斷言）：通報 `/admin/violation-reports/{id}`；案件與待轉報 `/admin/violation-reports/cases/{id}`；詢問逾期 `…/cases/{case_id}#inquiry-{inquiry_id}`；來函 `/admin/authority-orders/{id}`；告知 `…/{order_id}#target-{position}`；申覆、解除請求、複審 `/admin/measures/{measure_id}`；食品業者登錄 `/admin/food-registrations/{id}`；商家認證 `/admin/merchants/{store_id}#kyc`。
- 錨點：`CaseInquiries.tsx` 的詢問列加 `id="inquiry-{id}"`；來函詳情的對象列加 `id="target-{position}"`；商家詳情的 KYC 區加 `id="kyc"`（G10）。
- 側欄：「賣家 / 商品」最上面加「待辦」（3cd #100）。

### 4.2 既有頁面的期限（3cd §4.5）

| # | 位置 | 改動 |
|---|---|---|
| 122 | 措施清單「複審到期」分頁 | `reviewDueText` 換成「複審期限：{日期時間}（{DeadlineText}）」，用 `Measure.review_deadline` |
| 123 | 措施詳情的待處理請求 | 「裁定期限：{日期時間}（{DeadlineText}）」，用 `MeasureRequest.decide_deadline` |
| 124 | 食品業者登錄清單 | 新欄「期限」；「待審核」分頁依期限由近到遠排序（Q-A）；「等商家認證」分頁顯示「-」（伺服器不給），排序照舊 |
| 125 | 食品業者登錄詳情 | 「裁定期限：…」 |
| 126 | 商家詳情的 KYC 審核區 | 「審核期限：…」 |
| — | 案件詳情 | 「決定期限：…」（`decide_deadline`）、「轉報期限：…」（`fda_referral_deadline`），放在既有的狀態區 |
| — | 通報清單與詳情、詢問、來函 | 改讀 `*_deadline` 的 `start_time`，不再自己拿 `create_time`／`receive_time` 當開始時間 |

逾期時括號內一律是 danger「已逾期 {時長}」（`deadline.ts` 現有行為）。

### 4.3 小修

- 措施清單店家欄：沒有店名時顯示 `店家 {store_id}`（3cd §4.4 #113 的寫法），測試斷言不會出現措施 ID（G14）。

---

## 5. 文案表（Will 2026-10-20 照提案核准）

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| 100 | 側欄項目 | 待辦 | 已核准（D6′-1） |
| 101 | 頁面標題 | 待辦 | 已核准（Will 2026-10-20；3cd 提過，未核准） |
| 102 | 頁面說明 | 通報與案件、主管機關來函、措施、食品業者登錄與商家認證中等待處理的事項，依期限由急到緩。每 30 秒自動更新。 | 已核准（Will 2026-10-20；改寫 3cd #102：種類變了，而且全部都有期限） |
| 103 | 欄位 | 種類｜店家｜開始時間｜期限 | 已核准（D6′-5） |
| 104a | 種類：待分案 | 分案通報 | 已核准（Will 2026-10-20；取代「確認收件」：3c-a 起沒有確認收件） |
| 104b | 種類：待決定 | 決定案件 | 已核准（Will 2026-10-20） |
| 104c | 種類：詢問逾期 | 跟進逾期的詢問 | 已核准（Will 2026-10-20） |
| 104d | 種類：待轉報 | 轉報主管機關 | 已核准（D6′-5） |
| 104e | 種類：主管機關來函 | 處理主管機關來函 | 已核准（Will 2026-10-20） |
| 104f | 種類：告知被指名者 | 確認是否告知被指名者 | 已核准（Will 2026-10-20） |
| 104g | 種類：申覆 | 裁定申覆 | 已核准（3cd #104） |
| 104h | 種類：解除請求 | 裁定解除請求 | 已核准（3cd #104） |
| 104i | 種類：複審 | 複審停權 | 已核准（D6′-5） |
| 104j | 種類：食品業者登錄 | 審核食品業者登錄 | 已核准（D6′-5） |
| 104k | 種類：商家認證 | 審核商家認證 | 已核准（3cd #104） |
| — | 「調查通報」 | 刪除（rb R3：「調查中」拿掉，案件的待決定取代它） | — |
| 105 | 篩選 | 所有種類＋上面 11 種 | 已核准（Will 2026-10-20） |
| 106 | 期限 | 剩 {時長}｜已逾期 {時長}（danger）；剩不到四分之一 warning | 已核准（3cd D′5，3c-a 實作） |
| 108 | 開始時間 | {日期時間}（`formatDateTimeShort`） | 已核准（Will 2026-10-20） |
| 109 | 空狀態 | 目前沒有待辦 | 已核准（D6′-5） |
| 110 | 空狀態：有篩選 | 沒有這個種類的待辦 | 已核准（Will 2026-10-20） |
| 111 | 載入／讀取失敗 | 載入中...｜網路層失敗時「待辦讀取失敗」，Connect 錯誤照伺服器或目錄的句子；加「重試」 | 已核准（Will 2026-10-20；句型同 3c-a 已核准的「通報讀取失敗」） |
| 113 | 店家欄 | 店名；沒有店名時「店家 {id}」；沒有店家的項目（來函、A7 之後的項目）「-」；告知被指名者的對象沒有指定店家時「帳號 {id}」 | 已核准（Will 2026-10-20；改寫 3cd #113：來函沒有單一店家） |
| 114 | 分頁 | 上一頁｜第 {n} 頁｜下一頁 | 沿用 |
| 120 | 措施清單徽章 | 已逾期 | 已核准（D6′-4） |
| 122 | 複審到期分頁 | 複審期限：{日期時間}（剩 {時長}） | 已核准（Will 2026-10-20；3cd 提過） |
| 123 | 措施詳情 | 裁定期限：{日期時間}（剩 {時長}） | 已核准（Will 2026-10-20；3cd 提過） |
| 124 | 食品業者登錄清單 | 欄名「期限」；值照 #106；等商家認證「-」 | 已核准（Will 2026-10-20；3cd 提過） |
| 125 | 食品業者登錄詳情 | 裁定期限：{日期時間}（剩 {時長}） | 已核准（Will 2026-10-20；3cd 提過） |
| 126 | 商家詳情 KYC | 審核期限：{日期時間}（剩 {時長}） | 已核准（Will 2026-10-20；3cd 提過） |
| 127 | 案件詳情 | 決定期限：{日期時間}（剩 {時長}）｜轉報期限：{日期時間}（剩 {時長}） | 已核准（Will 2026-10-20；新） |
| 128 | 措施清單店家欄（沒有店名） | 店家 {id} | 已核准（Will 2026-10-20；同 #113） |
| 130–152 | D′6 用語 | 照 3cd §4.6 逐行表 | 已核准（3cd D′6，Will 2026-10-18） |

---

## 6. 測試

### 6.1 model（unit）
- 每個 `*Deadline()` 方法：有義務時開始與期限、沒有義務時 nil；邊界 ±1µs（`Overdue`、`ReviewDue`、`NoticeFollowUpDue` 的進入時間）。
- 每種函式的表格測試：開始與期限等於 §1.3 的值，待決定涵蓋每個 `Severity`（新增嚴重度沒有對應時長時，函式回 NULL 期限，測試要失敗）。
- `review_measure_review(at)`（integration）：`review_by − 24h` 前後 ±1µs、撤銷、到期、非 RISK。

### 6.2 資料集測試（integration，seller）
- **成員 ⇔ 判定式**：一份資料集含每一種的成員與「差一點不是」的反例（已分案、等賣家、已轉報或不轉報、已回覆或撤回的詢問、還沒逾期的詢問、已完成或撤銷的來函、已告知或確認日期在明天的對象、已裁定或措施已結束的解除請求、複審在 25 小時後、店家關閉或等商家認證的食品業者登錄、已核准的商家認證）。`pub_review_items(at)` 的集合等於逐一套用 Go 判定式的集合；另以函式內聯後的 `EXPLAIN` 記錄計畫（§6.5）。
- **期限 ⇔ 單一定義**：每一種的資源 `*_deadline` 等於 `pub_review_items` 裡同一個對象的開始與期限（兩者讀同一支函式，這條鎖住 emitter 沒有另外算）；待決定的開始時間含 ADMIN 撤回、不含系統撤回，食品業者登錄的開始時間含商家認證核准（**mutation**：少掉任一項要失敗）。
- **pub 契約**：`TestPubContractsMatchMigrations` 涵蓋 `pub_review_item`（既有的契約測試自動比對，不另寫）；`internal/arch` 的 sqlc schema 清單檢查涵蓋 `reviewqueue`。
- **資源輸出 ⇔ 待辦**：`*_deadline` 有值的集合等於待辦裡對應種類的集合（`Measure.review_deadline` 例外：它從生效起就有值，只斷言待辦是 `now >= start` 的子集；`CaseInquiry.reply_deadline` 與 `AuthorityOrder.deadline` 是賣家／機關的期限，只斷言後者與待辦列相同）。
- **複審到期分頁與待辦一致**：`ListMeasures(review_due)` 的集合等於 `MEASURE_REVIEW` 的集合。
- **翻頁不漏不重**（照 `TestAdminInboxKeysetHasNoGapsOrDuplicates`，`S/governance_integration_test.go:644`）：每頁 1、2、3、7 筆，含期限相同、開始時間也相同的列。
- **kinds 篩選**：每一種單獨篩選的集合等於全部裡那一種的集合。

### 6.3 `reviewqueue`（integration 與 unit）
- **翻頁不漏不重**（integration，真實 PG，資料由 seller 的指令或 fixture 建）：每頁 1、2、3、7 筆，含期限相同、開始時間也相同、跨種類的列，逐頁讀完的序列等於整個 `pub_review_items` 依全序排序的序列；**mutation**：`ORDER BY` 與 `WHERE` 的 row-value 少掉 `COLLATE "C"`、或少掉 `item_id`、或只取 N 筆而不是 N＋1，各自要失敗。
- `kind` 的值：SQL 回的每個 `kind` 都是模組已知的值；模組的種類與 proto 的 `ReviewQueueItemKind` 一對一（handler 的對照表測試）。
- token：kinds 不同、順序不同但集合相同（要通過）、壞 token 回 `INVALID_ARGUMENT`。
- 權限：呼叫者沒有某種類的讀取權時，那一種不出現；請求的 kinds 全部沒有權限時回空清單。**mutation**：拿掉交集，測試要失敗。

### 6.4 httpapi、admin
- handler：權限宣告、`kinds` 含 UNSPECIFIED、page_size 負數；admin 方法 93。
- admin：`itemHref` 逐種、店家欄的退回、種類標籤涵蓋 enum 的每個值（`satisfies Record<...>`）、篩選換頁回第一頁、30 秒更新、既有頁面的期限（#122–127）、措施清單店家欄；a11y 沿用既有守門。

### 6.5 效能（量測，不是測試）
- 開發資料量與 5 千筆合成的待辦（各種平均分配），`EXPLAIN (ANALYZE, BUFFERS)` 與 100 次 `ListReviewQueue(page_size=50)` 的 p50／p95，記在計畫書。超過 300 ms 是另存 SLA 紀錄的觸發之一（queue brief 結論）；接近時才考慮加索引（例如待決定的開始時間）。

---

## 7. Commit 計畫

| # | 內容 | 審查 |
|---|---|---|
| 3d-0a | D′6 用語（admin、目錄＋`make uitext`、website、seller-app、術語表），獨立 commit（3cd D′6） | code-review |
| 3d-0b | 措施清單店家欄；3cd D′1 改指 ADR 修訂段（G12 的八處） | 併入 3d-1 的 code-review |
| 3d-1 | 每種的 `seller.review_<種類>(at)`（手寫 migration）、`Deadline` 訊息、各資源的 `*_deadline`（換掉三個 `*_due_time`，emitter 改讀函式）、兩個收件匣改讀函式（複審到期提前 24h）；admin 改讀新欄位、既有頁面的期限（#122–127） | code-review |
| 3d-2 | `seller.pub_review_items`（`pub.sql`、契約測試）與成員資料集測試、`reviewqueue` 模組與查詢、`H/reviewqueue` handler、契約與權限 | code-review |
| 3d-3 | admin `/admin/queue`、側欄、三個錨點 | code-review |
| 3d-4 | 文件：新 ADR 與 3cd D′1、A7-D8、A7 計畫 A7-1e 的修訂段（E12）；runbook K18（食安 `:160` 申訴 14 天 → 7 天）、食安 `:5`、`:395`，violation runbook `:119`、`:153-155`（7 個工作日 → 7 天）與 `:20`，食品業者登錄 runbook `:24`（從可以裁定時起算）；API 規範 §一的 common 清單加 `Deadline`；A7 計畫 `:271`、`:547` 的欄位名；設計書 32、seller `AGENTS.md`、CARRYOVER（食安 SLA 列、沿用盤點）、計畫書的完成紀錄與效能量測 | design-review（整刀） |

design-review 跑兩次（全域規則：大範圍改動開工時與完成前各一次；3cd §3）：Will 決定之後、派 3d-1 之前跑一次，只看本設計；3d-4 之後跑整刀。

實作分派：主 session 寫派工、驗證與審查，實作派 Sonnet subagent（`AGENTS.local.md`）；migration 沒有，生成物（`buf generate`、`make sqlc`、`make uitext`）由主 session 產生。

## 8. 驗證命令（與 CI 相同）

每一刀派工前，照當時 `.github/workflows/` 的內容逐條核對一次（下列是 2026-10-20 讀到的）。

- backend（`backend-monolith.yml`）：`gofmt`（`cmd`、`internal`）、`go vet ./...`（含 `-tags integration`）、`go test -race ./...`（integration 帶 `DOCKER_HOST`，負載高時 `-p 2`）、`make lint-monolith`（只看新增問題）、`make connect` 與 `make sqlc` 的 drift（`make sqlc` 前 `docker info`）、`GOWORK=off go mod tidy` 無 diff。
- api（`api-contract.yml`）：`buf lint`；`buf breaking` 對 v1alpha1 不檢查（`api/buf.yaml` 的 `ignore_unstable_packages`，所以三個欄位改名不會報）；**三個**生成端的 drift（backend、`@hawkr/api`、`packages/hawkr_api`；`Deadline` 在 common，Dart 也生成；PATH 先放 `$HOME/.local/bin`）。
- 文案目錄：3d-0a 與 3d-2 都跑 `make uitext` 的 drift（新的 enum 會改 `enum-manifest.gen.json`）與 `scripts/check-enum-text-cross.sh`。
- admin（`admin-ci.yml`）：`pnpm --filter admin lint`（含 `tsc`）、`test`、`build`。
- website（3d-0a，`website-ci.yml`）：lint、`test:unit`、`test:build-output`、`design:check --apps-root ..`、build（`API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值的 `NEXT_PUBLIC_API_BASE_URL`）。
- seller-app（3d-0a）：analyze、custom_lint、改到的測試、commit 前完整測試一次、`scripts/deadcode.sh`。
- `scripts/check-legacy-endpoints.py`（最後一步）、`maxRESTOperations` 481 不變。

## 9. 沒有驗證的部分

- website 與 admin 的「實名認證｜身分認證」逐行分類（盤點 §13、審查 minor）：3d-0a 實作時做。
- `pub_review_items` 的 11 個分支以 `UNION ALL` 寫在一個函式裡時的計畫（內聯、`kind = ANY` 的篩選能否推進分支）：沒有跑 `EXPLAIN`，§6.5 量。待辦清單本來就要算出所有開著的項目再排序，推不進去只影響篩選單一種類時的成本；超過門檻時改成每種一個函式、`reviewqueue` 自己 `UNION ALL`（形狀與契約不變）。
- 外部基準：JSM 的 `breachTime`、Zendesk 的 `breach_at` 沿用 3b 的查證；`ongoingCycle` 的 `startTime` 欄位名沒有重新查證。

---

## 決定紀錄（Will 2026-10-20「照建議」）

- 期限的形狀：`Deadline{start_time, due_time}` 一組，欄位 `<義務>_deadline`，3c 已有的三個 `*_due_time` 一起換（v2 Q1 (a)，改寫 3b D8 的欄位名）。
- 待辦的讀法：`pub_*` 組合（`step10e-3d-queue-brief.md` v3 Q1 (g)），取代 3cd D′1 (c) 與 A7-D8 (c) 的 httpapi 合併層。
- 鍵在待辦期間會變的 3 種（E9）：接受，重新整理就正確；有第二位營運人員時重新評估。
- §5 的文案全部照提案。

---

## 實作時的修訂（3d-1）

實作與上文不同的地方，以這份為準。

1. `Measure.review_deadline` 只在複審進入待辦（`review_by` 前 24 小時）之後才有值，不是「生效中、有 `review_by` 就有」（§1.2）：期限規則只有一個家（E10），措施的期限讀的就是 `review_measure_review(at)`；在那之前措施詳情照舊顯示事實欄位「複審期限」（`review_by`）。（3d-1 code-review D1）
2. `KYCSummary.decide_deadline`，不是 `KYCApplicationSummary`（§1.2 寫錯了訊息：商家詳情讀的是 `KYCSummary`，`seller.proto:213`；`KYCApplicationSummary` 是列表列，admin 沒有消費者）。
3. 「依 ID 批次讀期限」的 sqlc 查詢只寫有消費者的 9 種；詢問逾期與告知被指名者不在資源上輸出期限（§1.2 第一點），沒有呼叫端。
4. `Case.DecisionClockStart`、`LastClockRestart`、`CaseDecisionClockStarts`、`CaseInquiry.ClockRestart` 刪除：它們是待決定開始時間的第二份定義，正式程式已沒有讀者；測試改以 fixture 的事實斷言函式的開始時間。
5. sqlc 讀不出複合型別函式輸出欄位的型別，查詢的輸出欄位以 `::` 轉型；食品業者登錄收件匣的 `ORDER BY` 用別名 `sort_at`。
6. 案件詳情的「決定期限」「轉報期限」有值才顯示（同通報詳情的「分案期限」）；詢問卡片的期限在回覆或撤回之後顯示「-」（伺服器不輸出，3cd §2 第 12 項）。
7. seller 的 integration 測試整包要 `-timeout 45m`（預設 10 分鐘跑不完）。

## 實作時的修訂（3d-2）

1. **沒有自己資料的模組不建 schema 與 migration**（Will 2026-10-20 照建議選 (d)；取代 §2.2「`schema.sql` 只有 `CREATE SCHEMA`」）：`reviewqueue` 沒有 `db/schema.sql`、migration 與 `atlas.hcl`，sqlc 只讀 `../../seller/db/pub.sql`；`cmd/hawkr/modules.go` 的 migration 清單不含它，另以 `withoutData` 登記（`TestEveryModuleIsRegistered` 涵蓋，並檢查這些模組沒有 `schema.sql`）；arch 的 `CheckSQLCConfig` 改成「有自己的 `schema.sql` 時剛好列一次，沒有時不列、至少一份允許的 `pub.sql`」；backend `AGENTS.md` 規則 6 加一句。觸發：第一次要存自己的資料（例如另存 SLA 紀錄）時以 `make new-module` 的骨架建 schema。
2. 種類與讀取權限的對照（E13）：通報 `seller.reports.list`；待決定、待轉報 `seller.cases.list`；詢問逾期 `seller.case_inquiries.list`（它揭露的是詢問）；來函兩種 `seller.authority_orders.list`；措施三種 `seller.measures.list`；食品業者登錄 `seller.food_registrations.list`；商家認證 `seller.kyc_applications.list`。handler 的權限判斷可注入（預設 `Principal.AdminAllows`），因為現在只有 `OWNER` 一個角色，不注入寫不出「只有部分讀取權」的測試。
3. 第一頁沒有游標，`after_*` 為 NULL（終審 3d-5 取代原先的 `-infinity` 文字哨兵）。
4. 種類不合法時回 `kinds[i]` 的欄位錯誤（`apperr.InvalidInputOn` 加 `rpc.WithFieldPaths`），句子「kinds[{i}] 必須是有效的待辦種類」照 `rpc/fields.go` 的「{欄位} 必須是有效的 ID」句型（傳輸層的格式錯誤，admin 畫面不會送出）。**待 Will 核准**。
5. `kind` 的 `COLLATE "C"` 保留，但現在 11 個值在預設與 C collation 下順序相同，測試抓不到拿掉它；觸發：新增的 kind 值與既有值在 collation 下順序不同時，ICU 測試加入跨種類的列。
6. 效能量測（§6.5，量測不是測試）：Apple M1 Pro、postgres:18-alpine、單連線、快取全熱；5018 件開著的待辦（11 種不均勻）；`ListItems(limit 51)` 的 `EXPLAIN ANALYZE` 執行 18.7 ms、計畫 6.9 ms，函式內聯、條件推進各分支（表小，Seq Scan）；100 次 p50 13.1 ms、p95 19.7 ms（單一種類 p50 4.2 ms、p95 4.7 ms）。遠低於 300 ms 的觸發線；沒量並行、冷快取與深頁，不能外推到大一個數量級。

## 附錄 A：審查意見的處理（v1 → v2）

| 審查 | 處理 |
|---|---|
| MAJOR 1 拿掉「有無期限」旗標與「加來源不改合併層」衝突 | 改照 D′1 保留旗標（E3、E4、§2.1、§3.2）；原 Q1 撤回 |
| MAJOR 2 `MaxKind` 沒定義、int32 溢位 | `MinKind = −1`、`MaxKind = math.MaxInt32`、rank 用 int32（E6、§3.2） |
| MAJOR 3 跨來源 seek 沒對真實 PG 跑 | §6.2 加真實 PG 的哨兵測試與 mutation |
| MAJOR 4 時長同值，mutation 測不出對調 | 政策改成注入的值型別，測試用互不相同的時長（E10、§3.1、§6.2） |
| MAJOR 5 `optional int64 store_id` | 改 `string store_id`（E15、§1.3） |
| MAJOR 6 「帳號 {id}」沒有欄位 | `AuthorityTargetRef` 加 `account_id`（E15） |
| MAJOR 7 靜默改寫 3b D8 的欄位名 | 列成 Q1，E2 補現有機制的理由與選項；A7 計畫的引用列進 3d-4；KYC 欄位改叫 `decide_deadline`、只放摘要 |
| MAJOR 8 靜默拿掉「只呼叫有讀取權的來源」 | 照 D′1 做，細到種類（E13、§3.3、§6.3）；刪掉 §0.2 的不實引用 |
| minor：`buf breaking` 不檢查 v1alpha1 | §8 改寫，§9 刪除 |
| minor：驗證命令與 CI 不符 | §8 照 workflow 重列（三個生成端、`go vet`、`-race`、`make connect`、`make uitext`、admin build、website 的檢查） |
| minor：runbook 漏「工作日」 | G11 與 3d-4 補四處 |
| minor：D′6 只量「實名認證」 | E17 改照 3cd 的量法 |
| minor：第一頁的 seek、空的 kinds | E6、E14、§2.1 |
| minor：G4 暗示索引能排序、`case_referrals` 的 case_id | G4 改寫，§9 刪除 |
| minor：`Deadline` 註解、輸入欄位名 | §1.1 |
| minor：D′6 要獨立 commit | 3d-0 拆成 0a、0b |
| minor：開放 enum、句子的位置 | §1.4 |
| minor：httpapi 沒有 `*db.DB` | E8 |
| minor：`ReviewPolicy` 匯出的 var 與 map | §3.1 改成值型別與欄位 |
| minor：`KYCApplication` 的期限沒有消費者 | 只放 `KYCApplicationSummary` |
| minor：文件清單少 API 規範 §一、A7 計畫 | 3d-4 |
| minor：格式（Q1 沒基準、Q2 沒觸發、E2／E13 沒有現有機制的理由） | Q1 換題並補基準，Q2 補觸發，E2、E13 補理由 |
| minor：只跑一次 design-review | §7 改成開工前與完成前各一次 |

---

## 附錄 B：開工前的 design-review（v3 → v3.1，5 項；`step10e-3d-design-review-start.md`）

| # | 發現 | 分流 | 處理 |
|---|---|---|---|
| 1 | [A] 期限規則寫成 Go 與 SQL 兩份，違反 ADR D3 修訂第 4 點「規則只有一個家」與 queue brief §3 | 改 | E10、§3.1：每種一支 SQL 函式，Go 只讀結果 |
| 2 | [B] v2 的可注入政策與 v3 的字面值互相矛盾 | 改 | 隨 #1 刪除注入；§6.1、§6.2 改寫 |
| 3 | [B] 兩個收件匣的成員是第三份條件；食品業者登錄清單依送出時間排、期限依 `max(送出, 核准)` | 改（成員）＋提（排序）→ Will 2026-10-20 照建議：待審核分頁依期限排序 | §3.1、§4.2 #124 |
| 4 | [A] `has_due` 在 (g) 之下沒有必要條件 | 提 → Will 2026-10-20 照建議：拿掉 | E3、§1.3、§2.1、§2.2、§3.2 |
| 5 | [C] 權限 API 留到實作時查 | 改 | 查到既有的 `rpc.PrincipalFrom` 與 `AdminAllows`，§3.2 寫明，不加新 API |

## 決定紀錄（v3.1，Will 2026-10-20「按照建議」）

- Q-A：食品業者登錄「待審核」分頁改成依期限由近到遠（`(due_at, id)` keyset，讀 `review_food_registration(@now)`）；「等商家認證」分頁照舊依送出時間。
- Q-B：拿掉「有無期限」欄（E3）。
