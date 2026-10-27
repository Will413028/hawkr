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
