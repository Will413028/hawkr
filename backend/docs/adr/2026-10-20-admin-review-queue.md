# 後台待辦：擁有者發布的 pub_* 組合，期限推導不另存

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-20） |
| 日期 | 2026-10-20 |
| 影響範圍 | `internal/modules/seller`（審閱政策函式、`pub_review_items`）、`internal/modules/reviewqueue`（新）、`internal/httpapi/reviewqueue`、`api/hawkr/admin/reviewqueue/v1alpha1`、`api/hawkr/common/v1alpha1/deadline.proto`、admin |
| 取代 | `docs/plans/2026-09-29-a3-buyer-seller/step10e-3cd-brief.md` D′1 (c)「各模組推導＋httpapi 的全域 keyset 合併」、A7-D8 (c) 與 A7 計畫 A7-1e 的「接進 10e 3d 的合併層」 |
| 依據 | `docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-queue-brief.md` v3（業界比對、選項、約束帳與兩輪審查）；設計 `step10e-3d-design.md` v3.1 |

## 脈絡

後台要有一個地方回答「現在最急的是哪一件」：11 種待辦（待分案、待決定、詢問逾期、待轉報、主管機關來函、告知被指名者、裁定申覆、裁定解除請求、複審停權、審核食品業者登錄、審核商家認證）都在 seller；A7 的 moderation、A5 的發票申訴之後也會進同一頁，三者都在同一個 database。每一種都有營運的處理期限（日曆時間），期限值是上線前的起始值，上線後要校準。只有一位營運人員，沒有指派、認領。

## 決策

1. **待辦是對擁有者事實的查詢，成員資格與期限在讀取時推導，不另存**：每種待辦一支 seller 內部函式 `seller.review_<種類>(at)`（單一 SELECT、`BEGIN ATOMIC`），期限的時長只寫在這裡；`seller.pub_review_items(at)` 是它們的 `UNION ALL`，形狀宣告在 seller 的 `db/pub.sql`。資源詳情的期限（`*_deadline`）與兩個收件匣（複審到期、食品業者登錄待審核）讀同一支函式，Go 不另寫時長。
2. **組合用 ADR `2026-09-24-go-modular-monolith` 文末修訂的第 4 點（`pub_*`），不用第 3 點（httpapi 的 keyset composition）**：同一個 database 裡，一支 SQL 就能排序與分頁，也天生是一個快照；新模組 `reviewqueue` 依賴 seller，查詢依 `(due_at, start_at, kind, item_id)` 做 keyset。加來源是發布另一個 `pub_*` 函式、查詢多一個 `UNION ALL` 分支：每個來源的列形狀由擁有者自己定、只放自己的對象鍵，`reviewqueue` 的查詢把各來源逐欄投影成共同的欄位（沒有的鍵填 NULL），擁有者不必認識別的來源的對象（2026-10-20 完成前的 design-review #2）。
3. **`reviewqueue` 沒有自己的資料，不建 schema 與 migration**：sqlc 只讀依賴表允許的 `pub.sql`（單體規則 6 的例外，2026-10-20）。
4. **期限的形狀是一組 `Deadline{start_time, due_time}`**：畫面要用開始時間決定何時轉黃，而待決定、食品業者登錄的開始時間只有伺服器算得出來。只在義務還在時輸出。
5. **只列呼叫者有讀取權的種類**：每種待辦對應既有的讀取權限，請求的種類與呼叫者有權的取交集。

## 考慮過的方案

| 方案 | 為什麼不選 |
|---|---|
| 各模組推導＋httpapi 的全域 keyset 合併（3cd D′1 (c)） | 在 application 裡重做 database 已經會做的排序與分頁：全序比較函式、跨來源的 seek 與哨兵、N＋1 合併、替身來源的 property test；現在只有一個來源 |
| 另存工作項目，由每個指令手寫開啟、關閉、重新起算 | 實際的轉移約 55 個效果、約 35 個函式，第一次盤點就漏了約 20 個；漏寫的待辦默默出錯 |
| 另存工作項目，指令內「從事實重算」維護（seller 的 `exposure.Announce`、`suspension.Recompute` 的形狀） | 好處（歷史、單一索引、認領的位置）現在沒有讀者；代價現在就要付：約 35 個指令要改、佇列的 bug 讓賣家與管理員的操作失敗、鎖順序、核對 job、週期判斷的邊界。留作終局 |
| 另存，outbox 非同步投影 | 同上，另有延遲（剛處理的事還留在待辦頁） |
| 期限存在 seller，成員照舊推導 | A7 來了還是要合併或搬到中央，中途多一次搬家 |

業界的基準：同一個系統裡，工作是存下的實體，佇列是對實體的查詢（ServiceNow 的 task、Zendesk 的 view、JSM 的 queue）；SLA 另存成紀錄（ServiceNow `task_sla`、JSM 的週期），是為了報表與可設定的暫停規則；多個系統要進同一個佇列、而且要路由與指派時，才另存中央的工作項目（Salesforce Omni-Channel、ROOST Coop）。出處與證據等級見 queue brief 與 `step10e-3d-industry-research.md`。

## 後果

- 期限規則只有一個家；規則一改，所有還開著的待辦的期限跟著改，已經結束的事項沒有「當時的期限」紀錄（現在沒有讀者）。
- 留在待辦期間，改案件嚴重度、延後複審、換告知的確認日期會改期限或開始時間；翻頁途中那一列可能重複或少一列，重新整理就正確（Elasticsearch 不配 PIT 的 `search_after` 同樣的語意）。
- 期限是日曆時間；工作天要等行事曆搬進單體。

## 終局與觸發

改成另存 SLA 紀錄（每段待辦一列，存當時的期限與結束原因），由 seller 在指令內以帳號或來函為單位、在領域鎖之下從事實重算維護，`pub_review_items` 當重算與重建的來源。觸發（任一）：

- 第一份要看「當時有沒有準時」的報表（達成率、給主管機關的報告）；
- 第二位營運人員（認領、「取下一筆」）；
- `ListReviewQueue` 在 page_size 50 時 p95 超過 300 ms（2026-10-20 量測：5 千筆開著的待辦，p95 19.7 ms）。

做的時候要先解決 queue brief §2 列的五點：先鎖再算靠領域鎖、重算的單位、週期的身分、核對的漂移定義、存結束原因；以及佇列的列在全域鎖順序的位置。

第一個沒有期限的種類出現時（例如 A7 的某一種），`pub_review_item` 加「有無期限」一欄，排序與游標各改一處、`deadline` 改選填。

## 修訂：刪除兩種待辦（2026-10-28，ADR `2026-10-27-general-ecommerce-scope-cut` D3；計畫 `plans/2026-10-27-ecommerce-scope-cut.md` 步驟 4）

FDA 轉報與食品業者登錄刪除，待辦少了 `CASE_FDA_REFERRAL`（待轉報）與 `FOOD_REGISTRATION`（審核食品業者登錄），seller 發布的種類剩 10 種；`seller.review_item` 與 `seller.pub_review_item` 少了 `food_registration_id`，`ReviewQueueItem.subject` 少了 `food_registration_id`（欄號 reserved）。排序、游標與「每一項都有期限」的決策不變。

## 修訂：待辦種類的增減與第一個存下來的處置（2026-11-13，Will 2026-11-02、2026-11-13 決定；收款開通見 `2026-11-13-payment-onboarding-provider-port-stripe.md`，銀行帳號提醒見 `2026-10-18-seller-governance-consolidated.md` 2026-11-13 修訂）

原決策段與 2026-10-28 修訂保留歷史。

**脈絡。** 收款開通改走 payment port 後，PayUni 的重新綁定不存在了，收款帳戶的建立失敗成為新的待辦來源；防詐用的銀行帳號 fingerprint 讓「處分之後才共用同一個銀行帳號」成為需要人看的事。同一頁要回答「現在最急的是哪一件」，所以兩者都進待辦。

**決策。**

- 種類現況（`ReviewQueueItemKind`）：刪除 PayUni 重新綁定的種類（值 12 `reserved`）；新增 `PAYMENT_ACCOUNT_CREATE_FAILED`（13，由 payment 發布，subject 是 `PaymentAccountRef`）與 `SHARED_BANK_ACCOUNT`（14，由 seller 發布，subject 是 `SharedBankAccountRef`）。seller 發布 10 種（含 `SHARED_BANK_ACCOUNT`）、payment 發布 1 種。第二個來源 payment 依決策 2 的方式加入：`payment.pub_review_items(at)` 與 seller 的 `pub_review_items` 在同一支查詢 `UNION ALL`，`reviewqueue` 依賴 payment，不 import 它的 Go 套件；payment 的列只有店家 ID，店名由 `internal/httpapi/reviewqueue` 經 seller 的店家卡片補上。
- `PAYMENT_ACCOUNT_CREATE_FAILED`：`payment.accounts` 的 `create_failed_at` 有值的帳戶，建立 job 的重試用完才進來；期限 24 小時（上線前的起始值）；管理員只有「重試建立」。
- `SHARED_BANK_ACCOUNT` 的成員仍是推導的，由 `seller.review_shared_bank_account(at)` 從處分與命中的雜湊算出，沒有待辦列。唯一存下來的是處置：表 `seller.measure_linked_reviews`（處分、帳號、時間、管理員），管理員把處分頁的一個關聯帳號標為「已檢視」，這一對就離開待辦。
- 這不違反決策 1「待辦不儲存」：存的是管理員做過的處置，不是待辦列；成員資格、開始與期限仍在讀取時推導，處置只是推導條件的一項（「這一對還沒有已檢視」）。沒有處置，項目永遠離不開待辦，所以處置不能省。
- 未採：另存提醒列（業界做法是保留歷史，但歷史現在沒有讀者）：要由同步的 transaction 寫入，手寫開啟與關閉的開關正是本 ADR 拒絕的形狀；純推導並限時消失：營運有沒有看過沒有紀錄（推測）。
- 未採：為 PayUni 重新綁定保留種類：缺件由賣家經新的開通連結補，管理員沒有可補的資料。

期限規則仍只有一個家（擁有者的 SQL）。
