# 商家治理與食安通報（Seller Governance & Food Safety）詳細設計書

> 本檔是 hawkr-admin（Next.js）商家治理畫面的現行規格。畫面背後的行為（通報與案件、措施與認定、停權推導、終止合作、主管機關來函、保存期限與排程）由單體的 seller 模組擁有，以 `docs/modules/seller.md` §5–§8、§11 為準，通報與案件的契約是 `api/hawkr/seller/v1alpha1/report.proto` 與 `api/hawkr/admin/seller/v1alpha1/`；資料表與 CHECK 的唯一來源是 `internal/modules/seller/db/schema.sql`。決策與理由見 ADR [`docs/adr/2026-10-18-seller-governance-consolidated.md`](../adr/2026-10-18-seller-governance-consolidated.md)。

---

## 1. Admin UI（hawkr-admin / Next.js）

全部頁面走單體的 Connect。

| 路徑 | 對應功能 |
|---|---|
| `/admin/violation-reports` | 「商家通報」：待分案｜案件｜所有通報三個分頁（類別篩選，案件分頁另有狀態篩選，含「等賣家」），keyset 分頁，每 30 秒自動更新；待分案分頁有分案期限（日曆時間，剩 {時長}／已逾期 {時長}） |
| `/admin/violation-reports/[id]` | 通報詳情：通報內容、補充（匿名的補充標示比對方式、丟掉的次數）、檢舉人、佐證、處理；分案、設定結果、移到其他案件三個對話框，查看案件 |
| `/admin/violation-reports/cases/[id]` | 案件詳情：案件、案件裡的通報（各有設定結果與移到其他案件）、移動紀錄、導出的措施（`ListMeasures(source_case_id)`）、內部註記；結案、改嚴重度、建立措施、建立風險措施、終止合作；「詢問賣家」區塊（`ListCaseInquiries`，舊到新，每筆一張卡：種類與狀態、建立、期限、給賣家的說明、回覆〔管道、收到的時間、記下、內容、機敏附件的連結〕或撤回〔時間、原因〕）與頁首的「詢問賣家」「等賣家」標記；對話框：詢問賣家、記下賣家的回覆、撤回詢問；結案對話框在等賣家時提示會撤回詢問 |
| `/admin/authority-orders` | 「主管機關來函」：處理中（期限舊到新）｜已完成｜已撤銷登記（收文時間新到舊）三個分頁，keyset 分頁，每 30 秒自動更新；欄位機關、文號、種類、指名的店家，處理中有期限（開始時間是收文時間）與收文時間，已完成有完成時間與保存到 |
| `/admin/authority-orders/new` | 登記來函（表單頁）：機關、文號、收文時間（預設現在）、種類、期限（不填就是伺服器依種類的預設，說明句寫出預設值）、內容摘要、指名的對象（多列：賣家帳號 ID、店家〔選填，讀這個帳號營運中與已關閉的店家〕、商品 ID）；`request_id` 在頁面開啟時產生，成功導向詳情頁 |
| `/admin/authority-orders/[id]` | 來函詳情：來函、指名的對象（逐家的涵蓋狀態，沒有店家資料時明示）、處理（完成、每個對象的告知）、保存（保存到、延長歷史）、導出的措施（`ListMeasures(source_authority_order_id)`）；完成、延長保存、記下告知（暫不告知的後續，依對象的 `can_record_notice`）、撤銷登記（`can_revoke`）四個對話框；建立措施／建立風險措施／終止合作三個入口帶 `source_authority_order_id`（違規依據另在新認定帶來源來函），每家店能不能建立由伺服器給（`AuthorityTargetStore.can_create_measure`、`can_create_termination`），可以的店有多家時先選（不預設第一家）。沒有內部註記 |
| `/admin/measures` | 「措施與申覆」：措施收件匣，待裁定｜複審到期｜生效中｜全部；篩選與排序由伺服器決定，不在 client 重排 |
| `/admin/merchants/[id]` | 店家詳情的治理區：生效中的措施（`GetStore.active_measures`）、該帳號的措施歷史（`ListMeasures(account_id)`，顯示 `state`）、建立 RISK 停權措施（風險類型、給賣家的理由、選填結束時間、必填複審期限與第一則內部註記）、結束措施（依 `can_end`）。建立對話框開啟時產生 `request_id`，同一對話框重送沿用 |

側欄在「賣家 / 商品」分組，順序是「待辦」、「賣家管理 / 審核」、「商家通報」、「主管機關來函」、「措施與申覆」。

### 1.1 畫面規則

- 按鈕只看伺服器的 `can_*`：不能操作時隱藏、不顯示原因；選項也來自伺服器（設定結果只列 `available_outcomes`，結案只列 `decision_options`，終止合作的事由來自 `GetStore`），前端不複製前置條件。可用性與原因都來自伺服器（`can_*`、`awaiting_seller`、`CaseInquiry.can_*`、`AuthorityOrder.can_*`、錯誤訊息），欄位的長度與範圍由伺服器判斷並回已核准的句子。
- 分案與移動的對話框開啟時讀這家店進行中的案件，說明會加入哪個案件或開新案件（沒有進行中的案件時要選嚴重度）；結案的對話框開啟時重讀案件（補充不改案件的版本，有新的補充時提醒結案前先看）；設定結果的對話框並排列出這筆通報當來源的措施，提醒設定結果不會撤銷措施。
- 版本衝突（`aborted`、`VERSION_CONFLICT`）：重讀、對話框留在原地並保留輸入、顯示衝突句；其他失敗顯示伺服器的句子，網路層失敗用該操作的固定句。成功時不另外提示，只有「移動讓原案件關閉」顯示橫幅。
- 從案件開三種建立措施的對話框都帶 `source_case_id`（頂端說明這個措施會記為案件導出的措施）；建立措施（違規依據）預填案件的類別與案件裡最早的 20 筆可以當來源的通報。
- 給賣家的理由（`statement`）必填，欄位旁 link 到 `docs/runbooks/seller-governance-shared.md` 的撰寫指引；理由只給受影響的賣家看，不公開。措施表單的複審期限預設建立後 7 天（表單預設，伺服器沒有預設值），上限 30 天（`docs/modules/seller.md` §5）。
- 分案期限的顯示：檢舉人選了食安的通報有分案期限（建立後 4 小時，`Report.triage_due_time`），「剩 {時長}」，剩下不到整段期限的四分之一（1 小時）轉 warning，逾期「已逾期 {時長}」danger；絕對時間在 title；時長無條件捨去；每 30 秒重讀，相對時間跟著走。期限只顯示、不強制，不推播。
- 詢問回覆的「收到的時間」預設現在且精確到秒；附件是機敏檔，在送出時才上傳成機敏檔（`isSecure` 的 secure_upload，失敗後重送不重傳），份數上限取自 `CaseInquiry.reply_evidence_max`、超過在上傳之前擋下，上傳失敗顯示網路失敗句，不顯示內部錯誤；附件連結在新分頁開啟（每次讀取伺服器都記錄）。

---

## 2. Runbook

| 檔案 | 用途 |
|---|---|
| `docs/runbooks/food-safety-incident-runbook.md` | 食安事件 runbook |
| `docs/runbooks/seller-violation-management-runbook.md` | 一般通報處理（分案、案件、移到其他案件、措施決策、申訴與補正） |
| `docs/runbooks/seller-governance-shared.md` | 共用內容（措施種類與依據選用、給賣家的理由撰寫指引、申訴決議標準、升級規則） |
