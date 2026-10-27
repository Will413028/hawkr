# 10e 第 4b 刀：疑似重複、合併、統編對帳改走 Connect，與修改時的合併出口，實作設計 v1.2

> **狀態**：v1.1。Will 2026-10-21 對 v1 的 F1 與 §5 文案回「都按照建議」。v1 經一輪獨立審查（1 HIGH、8 MED、12 LOW，全部處理，對照見附錄 A；全文在 session scratchpad `4b-design-review.md`，不入庫）；審查改到的文案與 F14 Will 已再確認（文末決定紀錄）。v1.2 處理開工前的 design-review（6 項，附錄 B）：5 項改、1 項記。

讀取的是 `local/10e` 的 `a6d814bb7`（與 main 相同），worktree 沒有未提交的改動［量測：`git status --short`、`git worktree list`］。
盤點：2026-10-18 的唯讀盤點（下稱「盤點」）、2026-10-20 對 `c6af4ed88` 的複核（下稱「複核」）、2026-10-21 對 `a6d814bb7` 的 4b 複核（下稱「4b 複核」）；三份都是 session scratchpad，結論收進本文 §0.4，不入庫。
SSOT：計畫 10e 段（`2026-09-29-a3-buyer-seller.md:346-387`）；`step10e-decision-brief.md`（下稱 db：D3、D4 第 9–10 點、:200 切分表、:203）；`step10e-4a-design.md`（下稱 4a：E1、E3、E8、E9、§0.2、附錄 C）；ADR `2026-10-04-prospect-contact-points-and-matching.md`（下稱 prospect ADR：D1、D3、D5、文末修訂）；錯誤模型 ADR `2026-09-25-api-error-model`；`prospect.proto`、`error_reason.proto`；admin `AGENTS.md`；術語表（4a-2 已補商家名單一節）。
路徑縮寫同 4a：`W` worktree 根、`A`＝`W/admin/src`、`SP`＝`A/app/admin/seller-prospects`、`G`＝`W/backend/gateway/internal`、`M`＝`W/backend/internal/modules/seller`。
標記：**[量測]**、**[推論]**、**[外部]** 同 4a。判定原則同 4a：上線前沒有相容性約束；照業界從零的做法；只有基準定不了的產品選擇、UI 改動、改契約或改寫已核准決定的列給 Will。

---

## 0. 前提題：這刀要改的程式之後會不會被重寫、搬走或刪除？

| 程式 | 之後的排程 | 判定 |
|---|---|---|
| `SP/duplicates/**`、`SP/reconciliation/**`、`MergeDialog`、`ProspectCompareTable`、`ConvertConfirmDialog` | 沒有。Connect 版的終點 | 會留下，照從零的做法做 |
| 4a 的 `ProspectMatchDialog`、`useProspectMatchRetry`、`ProspectEditForm` | 沒有 | 會留下；本刀在上面加合併出口 |
| gateway 剩下的 4 條路由、`admin_seller_prospect.go`、`routes_admin_seller_prospect.go`、`SellerProspectsPath` | 本刀刪 | 刪除，不在裡面最佳化 |
| `A/lib/seller-prospects.ts` 的 REST 段（:636-748）、`A/services/seller-prospects.ts` | 本刀刪 | 刪除 |
| seller 模組的 prospect 比對錯誤（`M/internal/prospect/match.go:105-111`） | 沒有 | 會留下；F1 改它 |
| seller-service 的 prospect proto、mock stub、`rpc-servers.csv` | 步驟 11（db K24） | 範圍外 |

結論：admin 與模組的改動都會留下；gateway 這一側是刪除。

---

## 0.1 範圍

| 部分 | 內容 |
|---|---|
| 疑似重複 `SP/duplicates/page.tsx` | `ListProspectDuplicates` keyset 分頁；`matched_on` 多個原因；每列「比對並合併」與「不是同一家」 |
| 標記不是同一家（新 `SP/duplicates/MarkDistinctDialog.tsx`） | `MarkProspectsDistinct` 的確認框 |
| 合併 `MergeDialog.tsx`、`ProspectCompareTable.tsx`（搬到 `SP/` 根目錄，F4） | 開啟時兩邊 `GetProspect`；`MergeProspects` 帶兩個 etag；比對表的聯絡方式合成一列；後果清單照模組的實際行為（F5） |
| 統編對帳 `SP/reconciliation/page.tsx`、`ConvertConfirmDialog.tsx` | `ListProspectTaxIDMatches` keyset；`ConvertProspect` 帶 etag；KYC 狀態文字（F7）；失敗後重讀清單、刪本機鎖列（F6） |
| 修改模式的合併出口（4a E9） | `ProspectMatchDialog` 每列「與這家合併」（對方是進行中才顯示，F2） |
| 修改撞到統編的合併出口（4a §0.2） | 契約新增 reason（F1），編輯頁的錯誤提示加「與這家合併」（F3） |
| seller 模組 | F1 的 reason 與 metadata；統編持有者已入駐、或還原時撞到時的伺服器句子（F1）；契約註解「進行中」→ 未封存（F8） |
| 刪除 | `A/services/seller-prospects.ts` 整檔、`A/lib/seller-prospects.ts` 的 REST 段 6 個符號、`seller-prospects-contract.test.ts:9,23-27` 的字串版 `matchedOnLabel` 測試 |
| gateway | 刪 4 條路由、`admin_seller_prospect.go` 整檔、`routes_admin_seller_prospect.go`、`routes.go:83`、`SellerProspectsPath`、4 支 wrapper、openapi 4 operation 6 schema 與 tag、routes test 與 media test 整檔、testutil 4 個 `*Fn` 改 stub；`maxRESTOperations` 469 → 465 |
| 文件 | 計畫 10e 的 4b 完成紀錄、seller `CARRYOVER.md:181,306`、prospect ADR 文末修訂（F1、F8） |

## 0.2 不在範圍

- 建立時撞到統編的 client 出口：伺服器會帶 reason（F1），但建立還沒有寫入、沒有東西可以合併，client 照舊顯示伺服器句子；觸發是 BD 要求從建立直接跳到持有者。
- 詳情頁的一般「合併到其他業者」（盤點 Q2 選項 c）：觸發是 BD 要合併兩筆從不相撞的業者。
- `rpc-servers.csv` 與 seller-service 的 prospect stub（db K24，步驟 11）。

---

## 0.3 決策總表

| # | 題目 | 決定 | 依據 |
|---|---|---|---|
| F1 | 修改撞到統編時，client 怎麼知道持有者（**改契約，要 Will 決定**） | `ErrorReason` 新增 `PROSPECT_TAX_ID_HELD`（搭配 failed_precondition；metadata `prospectIds` 是持有者的 ID，格式同 `PROSPECT_MATCH_FOUND`，只有一筆）。模組 `errTaxIDHeld` 在建立、修改與還原都加上 reason 與 metadata；錯誤碼照舊（建立 `already_exists`，修改與還原 `failed_precondition`），同一個 reason 搭兩個錯誤碼有先例（`VERSION_CONFLICT` 搭 aborted 與 already_exists）。建立與還原沒有 client 分支，仍帶同一個 reason：同一個錯誤條件只有一個形狀（AIP-193），client 不認得時照錯誤碼處理（附錄 B #2）。`LiveProspectByTaxID` 多回 `status`，句子依情況分三種（§5 #45–47）：修改且持有者進行中照舊「…該做的是合併」；持有者已入駐時不說合併（合併要兩邊都進行中）；還原時說「是同一家的話不需要還原」（封存的業者不能合併）。並行保底（unique 違反，在 transaction rollback 之後才處理，`lifecycle.go:114-115`、`prospect.go:496-497`，建立的對應處）改成以新的查詢重查持有者、回同一個 `errTaxIDHeld`；重查不到（持有者在這段時間離開）時伺服器把整個操作重跑一次，仍失敗才回不帶 reason 的 `aborted`（不借用 `VERSION_CONFLICT`：它只表示 etag 不符，還原也不帶 etag；附錄 A M-3、附錄 B #1） | 業界：錯誤附機器可讀的原因讓 client 分支（AIP-193 `ErrorInfo.reason`）［外部］；錯誤模型 ADR D1 的 details 只用標準型別，ID 放 `ErrorInfo.metadata` 正是這個形狀；`error_reason.proto` 檔頭「只為 client 會分支的情境新增」：本刀的合併出口就是第一個分支，必要條件成立。契約檔頭 :9-10「其他衝突只在錯誤訊息裡說明，不帶 reason」是因為當時沒有 client 分支，條件改變了。**替代**：不改契約，client 遇到沒有 reason 的 failed_precondition、而且草稿改了統編時，以只帶 `tax_id` 的 `PreviewProspect` 找持有者（`prospect.go:216-224` 會回 T1）。不採用：靠「錯誤碼＋表單狀態」猜錯誤種類，`只有進行中的業者可以修改` 也是沒有 reason 的 failed_precondition，猜錯會開出錯誤的合併框 |
| F2 | 修改模式每列「與這家合併」（4a E9） | `ProspectMatchDialog` 加選填 prop `onMerge(other: Prospect)`，只有修改的呼叫端傳；元件只看有沒有 `onMerge` 與該列 `status === ACTIVE`，不看 `mode`（附錄 B #5）；相符的業者包含已入駐的（F8），合併要兩邊都進行中（REAL）。按下：取消重送（hook 的 `cancel`），開 `MergeDialog`，預填「目前這筆」與「這家」。已入駐的列不顯示按鈕，改顯示「已入駐，不能合併」；三個模式的說明改成「未封存」的說法並寫出已入駐時該怎麼做（§5 #43、#51、#52，附錄 A M-2）。`useProspectMatchRetry` 的 `confirm`（帶 ID 重送）撞到 `PROSPECT_TAX_ID_HELD` 時關閉對話框、交給 `onFailure`，與第一次送出同一個出口（附錄 A L-12） | prospect ADR D3「讓管理員選擇合併」；4a E9（Will 2026-10-20 核可）；`prospect.proto` MergeProspects 註解；`M/internal/prospect/lifecycle.go:184-187` |
| F3 | 修改撞到統編的合併出口 | `useProspectMatchRetry` 的 `onFailure` 改收整個失敗結果（含 `reason`、`metadata`），不只文字。`ProspectEditForm` 收到 `PROSPECT_TAX_ID_HELD` 時，頁內錯誤提示顯示伺服器句子，並 `GetProspect` 持有者：進行中就在提示裡加「與「{持有者}」合併」按鈕，開預填的 `MergeDialog`；不是進行中（已入駐）只顯示句子 | 4a §0.2；prospect ADR D1「該做的是合併」；F1 |
| F4 | `MergeDialog` 的形狀 | props 改成 `{leftId, rightId, matchedOn?, draftWillBeLost?, onMerged(winner: Prospect), onClose}`；開啟時兩邊各 `GetProspect`（疑似重複清單不帶 contacts，也要拿最新 etag；4a 附錄 C #3 同一個代價判斷）。讀取中、讀取失敗可重試。送出 `MergeProspects{winner_id, winner_etag, loser_id, loser_etag}`，走 `runAction`：`VERSION_CONFLICT` 重讀兩邊、清掉勾選、顯示 §5 #41；其他失敗顯示伺服器句子並重讀。檔案搬到 `SP/MergeDialog.tsx` 與 `SP/ProspectCompareTable.tsx`（兩個頁面在用），`admin/tests/modal-layout.test.ts:60-67` 的路徑跟著改。預設勝方依進入點：從統編出口（F3）進來時是統編持有者（統編是權威識別碼，prospect ADR D1；雙方統編不同時輸家的統編會被清掉，預設保留持有者的才符合管理員的意圖，附錄 A M-5）；疑似重複頁與修改模式的相符列沿用「業務欄位填得多的一邊」，聯絡方式以「有幾種類型」計入（取代 4 個 public 欄位），平手取 ID 小的（以數值比較，ID 是十進位字串）。勝方可切換、切換時清掉勾選與錯誤（現行）。讀到任一邊不是進行中時停用送出並說明（§5 #53） | 契約 MergeProspects（兩個 etag 必填）；`runAction`（4a E3）。預設勝方：業界做法不一（HubSpot 以目前所在的紀錄為主、Salesforce 讓使用者選主紀錄）［外部，本次未查證］，現行規則是我們的，沒有新理由改，保留；重新評估的觸發：BD 反映預設方向常常要換 |
| F5 | 合併的後果清單 | 照契約重寫（§5 #37–40）；`prospect.proto` MergeProspects 註解（`:70-74`）漏了詢價連結改由勝方承接、兩家之間的標記撤銷兩項，4b-0 補進契約，文案以契約為準、模組行為是證據（附錄 B #4）：輸家封存並指向勝方；詢價連結改由勝方承接（`agent.go:154-158`）；聯絡方式併入勝方、相同的只留一筆、勝方原本的主要不變（`prospects.sql:112-136`）；分類樹的歸類（`category_ids`，admin 沒有畫面）也併入（`:192-196`），文案不提，畫面上的「分類」（純量 `category`）屬於「其他欄位不動」（附錄 A H-1）；輸家的統編一律清空，勝方沒有統編時搬過去（`prospects.sql:202-223`）；輸家的 logo 與封面刪除、不搬（`lifecycle.go:220`）；輸家標記過的「不是同一家」改掛勝方，兩家之間的標記撤銷（`prospects.sql:164-179`）；勝方其他欄位不動 | 4b 複核 Q5［量測］ |
| F6 | 統編對帳的轉成店家失敗 | 刪本機的 `blockedMatches` 鎖列。`ConvertProspect` 走 `runAction`（reload＝確認框那一筆的 `GetProspect` 重讀，加上讓對帳清單失效；附錄 A M-4）：`already_exists`（店家已被認領）、`failed_precondition`（業者不再是進行中）、`not_found`（店家已關閉）關閉確認框，頁面錯誤提示顯示伺服器句子，清單重讀後那一列由伺服器決定是否還在（對帳查詢排除已認領的店與非進行中的業者，`prospects.sql:337-353`）；`VERSION_CONFLICT` 留在框內、顯示 4a §5 #20b；沒收到回應留在框內可重試 | 鎖列的必要條件是「REST 清單有 5 分鐘快取、會再列出同一列」；改成失敗後重讀，條件不成立。舊碼以 HTTP 409 判斷，Connect 下改看錯誤碼 |
| F7 | KYC 狀態文字 | `kycStatusText` 從 `A/lib/food-registration.ts:136-148` 搬到 `A/lib/kyc-status.ts`（第二個使用者出現），food-registration 改 import。對帳表格用 `kycStatusText`；`ConvertConfirmDialog` 的業者側 props 改成直接收 `Prospect`，刪 `ConvertProspectSummary` 轉接型別（它只為 REST 與 Connect 並存而存在，附錄 B #3）；`certificationStatus` 改成選填的 `KYCStatus`，框內轉文字（`ConvertPanel` 不傳；附錄 A L-7）。兩個呼叫端的失敗處理不同是刻意的：詳情頁的店家可以在同一個面板換一家再送，所以留在框內；對帳頁的店家是那一列固定的，永久性失敗後那一列由伺服器決定是否還在，所以關框重讀（F6） | 盤點落差 8；`prospect.proto:265` 的 `kyc_status` 與 `kycStatusText` 是同一個 `KYCStatus` enum［量測］ |
| F8 | 「進行中」與「未封存」的落差 | 模組的 `live`＝`status <> 'ARCHIVED'`（`M/db/migrations/20261002211956_seller_prospects_views.sql:17`），統編唯一與比對都以它為準，與 prospect ADR D1、D3 的「live」一致；契約檔頭 :6、UpdateProspect :54、`error_reason.proto` 的 `PROSPECT_MATCH_FOUND` 與 `errTaxIDHeld` 的註解寫「進行中」，是註解錯。改註解成「未封存（進行中或已入駐）」，三個生成端重新生成（只有註解變）。client 依列的 `status` 決定能不能合併 | 程式與 ADR 一致，註解是落差［量測，4b 複核 Q5］ |
| F9 | 疑似重複頁 | keyset（`A/lib/keyset-pages.ts`），「上一頁／第 N 頁／下一頁」，刪 500 截斷橫幅（db D4 第 10 點）；`matched_on` 以 `matchReasonsText`（4a 的 `A/lib/seller-prospects.ts:95-114`）；每列兩邊顯示名稱連結與統編，不顯示聯絡方式（清單不帶 contacts，盤點落差 4）；操作「比對並合併」、「不是同一家」 | db D4 第 10 點、K2 |
| F10 | 標記不是同一家 | 確認框（共用 `Dialog`）；`MarkProspectsDistinct{first_prospect_id, second_prospect_id}`，不帶 etag、冪等；成功 toast、清單失效；`failed_precondition`（有一筆不再是進行中）與 `not_found` 顯示伺服器句子並讓清單失效 | 契約 MarkProspectsDistinct；prospect ADR 文末修訂（直接標記）；db D3 |
| F11 | 寫入後的快取 | 沿用 4a 的 `useProspectCache().written`（`SP/prospect-queries.ts:11-53`，已讓三個清單類查詢與統計卡失效）：合併以回應的勝方呼叫 `written(winner)`，輸家的 `getProspect` 失效由 `useProspectCache` 新增的 `retired(id)` 負責，呼叫端不自己寫失效（附錄 B #6）；標記不是同一家沒有回應，只讓清單類查詢失效；轉成店家 `written(prospect)` | 4a 附錄 C #5、#6（附錄 A L-2） |
| F12 | 從編輯頁合併時的草稿與導向 | 從 F2、F3 開的 `MergeDialog` 一定是儲存被擋下的時候（草稿必定有改動），框內固定加一句「這筆業者尚未儲存的修改不會保留」（§5 #44）。合併成功：目前這筆是輸家 → 導到勝方的詳情頁；是勝方 → 以回應的 `prospect` 取代頁面持有的那筆，表單照 `send` 成功時的同一組步驟重設（`ownEtag`、基準、草稿，`ProspectEditForm.tsx:130-137`），不出現衝突提示（附錄 A L-6）。兩種都 toast | 合併讓兩邊的版本都前進，草稿依的版本必定過時（4a E5）；留著草稿只會在下次儲存時撞 `VERSION_CONFLICT`。把草稿自動套到勝方是另一個寫入、而且輸家的草稿套到勝方語意不明，不做［推論］ |
| F14 | 標記「不是同一家」之後發現是同一家（**再確認**） | 標記過的一對不會再出現在疑似重複清單與相符對話框（`prospects.sql:146-148,285-286`），本刀沒有其他合併入口，所以畫面上無法合併它們。建議：文案說實話（§5 #35），一般的「合併到其他業者」維持 §0.2 的觸發（BD 要合併兩筆不相撞的業者，或第一次有人標錯）。替代：本刀在詳情頁加「合併到其他業者」（以 `ListProspects` 的關鍵字搜尋選對方，再開 `MergeDialog`），是新的 UI 流程 | 附錄 A M-1；業界 CRM 都有從紀錄頁手動合併（HubSpot、Salesforce）［外部，本次未查證］ |
| F13 | 刪 REST 殘留 | `A/lib/seller-prospects.ts` 的 `SellerProspect`、`DuplicatePair`、`TaxIDMatch`、`PROSPECT_STATUSES`、`OUTREACH_STATUSES`、字串版 `matchedOnLabel` 與 `A/services/seller-prospects.ts` 整檔刪；舊表的「營運中」隨之消失。實作者先 `rg -nw` 每個符號確認只剩 4b 的使用者再刪 | 4b 複核 Q2［量測］ |

---

## 0.4 查證過的現況（`a6d814bb7`）

| # | 事實 | 證據 |
|---|---|---|
| H1 | `duplicates/page.tsx`、`MergeDialog.tsx`、`ProspectCompareTable.tsx` 自 `c6af4ed88` 未變；`reconciliation/page.tsx:173-174` 把 REST 的 `publicEmail/Phone` 餵給 4a 改名後的 `primaryEmail/Phone` | [量測] `git diff --stat c6af4ed88 a6d814bb7` |
| H2 | REST 殘留只剩 `A/lib/seller-prospects.ts:636-748` 的 6 個符號與 `A/services/seller-prospects.ts`（74 行，4 個方法），使用者只有 4b 三頁與 `seller-prospects-contract.test.ts:9,23-27` | [量測] `rg -nw <符號> admin/src admin/tests` |
| H3 | `ProspectMatchDialog`（289 行）沒有每列動作的插槽，每列已有完整 `Prospect`（含 `status`、`etag`）；`useProspectMatchRetry` 的 `onFailure` 只收文字 | [量測] `SP/ProspectMatchDialog.tsx:80-126,221-288`、`SP/use-prospect-match-retry.ts:16-58` |
| H4 | 修改撞統編：`apperr.FailedPrecondition("統一編號已由業者 %d 使用，該做的是合併")`，不帶 reason；傳輸層只在有 reason 時附 `ErrorInfo` | [量測] `M/internal/prospect/match.go:105-111`、`prospect.go:377-383`、`lifecycle.go:93`、`backend/internal/httpapi/rpc/errors.go:145-148` |
| H5 | 統編檢查在聯絡方式比對之前（兩個都撞時只看到統編） | [量測] `prospect.go:377` 先於 `:384` |
| H6 | `live`＝未封存（含已入駐）；疑似重複與統編對帳只列進行中 | [量測] F8；`prospects.sql:260-292,337-353` |
| H7 | 合併的實際行為（F5）；`MarkDistinct` 非進行中回 failed_precondition | [量測] `lifecycle.go:120-245`、`prospects.sql`、`agent.go:154-158` |
| H8 | gateway：4 條路由（`routes_admin_seller_prospect.go:15-18`）、`admin_seller_prospect.go` 297 行、wrapper `seller_client.go:131-163`、`SellerProspectsPath`（`constants.go:538`，唯一使用者是路由檔）、testutil 4 個 `*Fn`（`:382-385`，其他地方沒人賦值）；openapi 469 個 operation（prospect 4、schema 6、tag 1）＝`maxRESTOperations` | [量測] 4b 複核 Q7 |
| H9 | legacy baseline 只剩 `scripts/legacy-endpoints-baseline.json:74-76`（`admin_seller_prospect_media_test.go`）；json-contract check_a 的 prospect 位點 #6、#16、#23、#24 都在本刀刪的檔或 schema；gate 目前通過 | [量測] `python3 backend/scripts/gateway-json-contract-check.py` |
| H10 | `modal-layout.test.ts:60-67` 寫死 `seller-prospects/duplicates/MergeDialog.tsx` | [量測] |
| H11 | `ProspectCompareTable` 的欄位（:55-126）在 `Prospect` 都有對應（contacts 以外 1:1） | [量測] `prospect.proto` message Prospect |

## 0.5 約束帳（本刀新出現的）

| 約束 | 判定 | 來源 |
|---|---|---|
| 合併要兩邊都進行中 | **REAL**（契約與模組） | `prospect.proto` MergeProspects；`lifecycle.go:184-187` |
| 合併兩個 etag 必填 | **REAL**（契約） | `prospect.proto:353-362` |
| 統編在未封存的業者之間唯一 | **REAL**（DB partial unique index） | prospect ADR D1 |
| 「其他衝突不帶 reason」 | SELF-IMPOSED（契約註解，當時沒有 client 分支）；F1 改 | `prospect.proto:9-10` |
| `ErrorInfo` 只用標準型別、不自訂 detail | **REAL**（已核准的 ADR，本刀不重開） | 錯誤模型 ADR D1 |
| 4a 的 `ConvertConfirmDialog` 要同時服務 REST 與 Connect | 本刀消失 | 4a E1 |
| 舊統計卡 5 分鐘快取 | SELF-IMPOSED；F11 加寫入後失效 | 4a 附錄 C #6 |

---

## 1. 契約（F1、F8）

- `error_reason.proto`：新增 `PROSPECT_TAX_ID_HELD = <下一個值>`，註解：「商家名單修改或還原時，統一編號已由另一筆未封存的業者持有（搭配 failed_precondition）。ErrorInfo.metadata 的 prospectIds 是持有者的 ID（十進位）。持有者是進行中時 client 可以讓管理員與它合併（修改時）；還原時是同一家就不需要還原（ADR 2026-10-04-prospect-contact-points-and-matching）。」`PROSPECT_MATCH_FOUND` 的「進行中」改「未封存」。
- `prospect.proto`：檔頭 :6 與 :9-10、UpdateProspect :54-55、RestoreProspect 對應句改成 F1、F8 的說法；另外 `:30,67,345` 的「其他／別的進行中業者」一起改（附錄 A M-6）；MergeProspects 註解補詢價連結改由勝方承接、兩家之間的「不是同一家」撤銷（F5）；CreateProspect 的統編衝突註解補 reason（F1）。
- 三個生成端重新生成：backend `make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 的 `../../api/generate.sh buf.gen.yaml`（PATH 先放 `$HOME/.local/bin`；Dart 先暖機，見 `AGENTS.local.md`）。`buf breaking`：新增 enum 值不是破壞性變更。

## 2. 單體（seller 模組與 apperr）

- `backend/internal/platform/apperr/apperr.go`：`ReasonProspectTaxIDHeld = newReason("PROSPECT_TAX_ID_HELD")`；`apperr_test.go:69-73` 的清單是寫死的，手動加；與 proto 的一致性由 `backend/internal/httpapi/rpc/rpc_test.go:299-305` 檢查。
- `LiveProspectByTaxID` 改回 `prospect_id, status`（`make sqlc`，Docker 先確認）；呼叫端有三個：`match.go`、還原（`lifecycle.go:93` 一帶）與 `PreviewProspect`（`prospect.go:217`）。
- `errTaxIDHeld(holder, holderStatus, op)`：建立照舊；建立、修改與還原都 `.WithReason(ReasonProspectTaxIDHeld).WithMetadata("prospectIds", <id>)`，句子依 §5 #45–47。
- 並行保底（建立、修改、還原三處）改成 rollback 後重查持有者、回 `errTaxIDHeld`；查不到時整個操作重跑一次，再失敗回不帶 reason 的 `aborted`（F1）。
- 「進行中」實指未封存的註解一起改（附錄 A M-6 的清單：`internal/model/prospect.go:240,423,444`、`internal/prospect/match.go:52,74,90`、`internal/prospect/prospect.go:6,128-129,394-395`、`lifecycle.go:64`、`apperr.go:100`、`seller/prospect.go:172,195,208`；實作者以 `rg -n "其他進行中|別的進行中|進行中業者|進行中的業者相同"` 再掃一次）。
- seller `AGENTS.md:128`（live 的定義、統編衝突的回法）與 `CARRYOVER.md:173,374`（「其他衝突不帶 reason」）同一個 commit 更新。
- 測試：`M/prospect_integration_test.go` 補修改撞統編（持有者進行中、已入駐）與還原撞統編，斷言 reason、metadata、句子，並行保底以兩個 transaction 交錯重現一次；`backend/internal/httpapi/seller/prospect_integration_test.go` 補一條 wire 層斷言 `ErrorInfo.reason` 與 metadata。

## 3. admin

### 3.1 疑似重複 `SP/duplicates/page.tsx`（F9）

- `useQuery(listProspectDuplicates, {pageSize: 50, pageToken})`，`keyset-pages`；表格「比對依據（多個徽章或以「、」連接）／業者 A／業者 B／操作」。
- 操作：「比對並合併」開 `MergeDialog{leftId, rightId, matchedOn}`；「不是同一家」開 `MarkDistinctDialog`。

### 3.2 `MarkDistinctDialog.tsx`（新，F10）

- 共用 `Dialog`；顯示兩邊名稱與比對依據、§5 #35 的說明；按鈕「確認不是同一家」「取消」；送出走 `runAction`。

### 3.3 `SP/MergeDialog.tsx`、`SP/ProspectCompareTable.tsx`（搬移，F4、F5）

- 欄位改讀 `Prospect`；聯絡方式合成一列（§5 #36）；enum 文字用 4a 的新表（`prospectStatusText`、`outreachStatusText`）。
- 身分卡的電子郵件、電話取主要的（`primaryContactValue`）。
- 比對表補「業者型態」與「logo／封面」（有／沒有）兩列：後果清單說輸家的圖片會刪除，管理員要看得到（附錄 A L-9）。
- 成功：toast、F11 的失效、呼叫 `onMerged(winner)`。

### 3.4 統編對帳 `SP/reconciliation/page.tsx`（F6、F7）

- `useQuery(listProspectTaxIDMatches, {pageSize: 50, pageToken})`，`keyset-pages`；認證狀態欄 `kycStatusText(match.kycStatus)`。
- 確認框的 reload 是這一筆的 `GetProspect` 加清單失效（F6）。
- 確認框摘要：業者沒有聯絡方式（清單不帶 contacts），`primaryEmail/Phone` 以 `GetProspect` 補，或不顯示這兩行——**選 `GetProspect`**：確認框是不可逆動作前的核對，需要聯絡方式，也順便取最新 etag；讀取中停用確認鍵。
- 送出 `ConvertProspect{id, store_id, etag}`，失敗處理照 F6；成功 toast（4a 的「已轉成店家」）、F11。

### 3.5 合併出口（F2、F3、F12）

- `ProspectMatchDialog`：選填 `onMerge`；列上「與這家合併」按鈕（`mode === "update"` 且 `status === ACTIVE`）。修改模式說明改 §5 #43。
- `useProspectMatchRetry`：`onFailure(result)` 收整個失敗結果；三個呼叫端（建立、修改、還原）改用 `result.text`，修改另處理 F3。
- `ProspectEditForm`：持有 `mergeTarget: Prospect | null`；開 `MergeDialog{leftId, rightId（以數值排序）, defaultWinnerId（F3 是持有者）, draftWillBeLost: true}`；`onMerged` 照 F12。

### 3.6 `A/lib/`

- `kyc-status.ts`（新，F7，搬移）。
- `seller-prospects.ts`：刪 REST 段（F13）；預設勝方的完整度純函式（F4）放這裡並測試。

## 4. gateway（刪除）

- 路由 4 條、`routes_admin_seller_prospect.go` 整檔、`routes.go:83` 的註冊、`constants.go:535-539` 的 `SellerProspectsPath` 區塊。
- `G/handler/admin_seller_prospect.go` 整檔（6 DTO、`toAdminProspectDTO`、`operatorAccountID`、4 handler）；wrapper `G/grpc/seller_client.go:131-163` 的 4 支；testutil 4 個 `*Fn` 改 stub（照 4a 在 `:711-713` 的先例）。
- 測試：`G/router/admin_seller_prospect_routes_test.go`、`G/handler/admin_seller_prospect_media_test.go` 整檔刪。
- openapi：4 operation、6 schema、tag `SellerProspects`；`maxRESTOperations` 469 → 465。
- `python3 backend/scripts/gateway-json-contract-check.py`：check_a 的 #6（count 2 → ?）、#16、#23、#24 會變，預期要 `--update`，以輸出為準。
- 最後一步 `scripts/check-legacy-endpoints.py --update`（baseline :74-76 那筆消失），再跑一次不帶 `--update`。
- 實作者先逐一 `rg` 刪除清單上的每個符號，確認沒有其他使用者再刪。

---

## 5. 文案表（4b 的部分；A＝已核准、C＝跟隨契約或 SSOT、E＝沿用、**N＝新寫，要 Will 核可**）

行號是 `a6d814bb7`。用語同 4a：標題與側欄「站外業者」，頁內句子「業者」。

| # | 位置 | 改前 → 改後 | 類 |
|---|---|---|---|
| 30 | `MergeDialog.tsx:147-148`「信箱 ·」「電話 ·」 | 取主要的電子郵件、電話；沒有時「未填寫」 | E |
| 31 | `duplicates/page.tsx:66` 副標 | →「進行中的業者之間名稱相同，或聯絡方式（電子郵件、電話、網站網域、社群帳號）相同、而且還沒標記為不是同一家的配對。可以合併（不可逆），或標記為不是同一家。」 | N |
| 32 | `duplicates/page.tsx:80-98` 截斷橫幅 | 刪，改「上一頁／第 N 頁／下一頁」 | A（db D4 第 10 點） |
| 33 | `duplicates/page.tsx:39-41` 聯絡方式行與「未留聯絡方式」 | 刪 | C（清單不帶 contacts） |
| 34 | 疑似重複每列 | 按鈕「不是同一家」；確認框標題「確定這兩筆不是同一家嗎？」 | A（按鈕，db D3）＋N（標題） |
| 35 | 標記確認框 | 說明「標記後這一對不會再出現在疑似重複清單，之後修改或還原也不會再因為它們相符而提醒。標記無法在畫面上取消，也不會再提示這兩筆可以合併，請確認後再送出。」（**再確認**，F14、附錄 A M-1、L-10）；按鈕「確認不是同一家」；成功 toast「已標記為不是同一家」 | N（後果量測自 `lifecycle.go:120-147`、`prospects.sql:164-179,285-286`） |
| 36 | `ProspectCompareTable.tsx:60-79` 四列聯絡欄位 | 合成一列「聯絡方式」，每筆「{類型}：{值}」，主要的加「（主要）」；沒有時「未填寫」 | N |
| 37 | `MergeDialog.tsx:220` 邀請搬移 | →「寄給「{敗方}」的詢價連結改由「{勝方}」承接」 | N |
| 38 | `MergeDialog.tsx:226` 其餘欄位 | →「「{敗方}」的聯絡方式會併入「{勝方}」（相同的只留一筆，「{勝方}」的主要聯絡方式不變）；「{勝方}」其他欄位（含分類、地區）不變，要保留「{敗方}」這些欄位的值，請先編輯「{勝方}」再回來合併」（**再確認**，附錄 A H-1）；統編那條（:223）照舊 | N（:226）＋E（:223） |
| 39 | `MergeDialog` 後果清單新增 | 「「{敗方}」的 logo 與封面會刪除，不會搬給「{勝方}」」、「「{敗方}」標記過的「不是同一家」改掛到「{勝方}」；兩家之間的標記會撤銷」 | N |
| 40 | `MergeDialog.tsx:209` | 「本次上線未提供合併的復原功能」→「合併沒有復原功能」 | N |
| 41 | `MergeDialog` 讀取與衝突 | 讀取中「正在讀取兩筆業者的完整資料…」；讀取失敗「無法讀取業者資料。」＋「重試」；衝突「其中一筆已被其他操作更新。畫面已重新讀取，請重新核對後再合併。」；沒收到回應時：自動重讀兩邊，顯示「無法確認合併是否完成。畫面已重新讀取，請確認兩筆的狀態後再決定。」（取代 `:122-125`「…兩筆站外業者的狀態都沒有變動…」：沒收到回應時合併可能已經完成；**再確認**，附錄 A M-8） | N |
| 42 | `reconciliation/page.tsx:265`、`ConvertConfirmDialog.tsx:136` 認證狀態 | 原樣印 enum → 填寫中／待審核／已核准／已退回／已取消／未知 | C（`kycStatusText`） |
| 43 | `ProspectMatchDialog` 修改模式說明（`:51-52`）與列按鈕 | →「下列業者的聯絡方式與這次修改後的相同。是同一家的話：進行中的可以與它合併，已入駐的請封存這筆；確認都不是同一家，逐一勾選後再儲存。」（**再確認**，附錄 A M-2）；列按鈕「與這家合併」（只在對方進行中時出現） | N（說明，4a §5 #16 預告的補句）＋A（按鈕，4a E9） |
| 44 | 從編輯頁開的 `MergeDialog`，表單有未儲存的修改時 | 「這筆業者尚未儲存的修改不會保留。」 | N（F12） |
| 45 | 編輯頁撞到統編的提示 | 伺服器句子（修改、持有者進行中）「統一編號已由業者 {ID} 使用，該做的是合併」照舊，後面加按鈕「與「{持有者名稱}」合併」 | E（句子）＋N（按鈕） |
| 46 | 伺服器句子：修改、持有者已入駐 | 「統一編號已由已入駐的業者 {ID} 使用；是同一家的話請封存這筆」（**再確認**，附錄 A L-11） | N（`match.go`） |
| 47 | 伺服器句子：還原 | 「統一編號已由業者 {ID} 使用，是同一家的話不需要還原」 | N（`match.go`；與 4a §5 #16 還原模式的「是同一家的話不需要還原」同句型） |
| 48 | 合併成功 toast | 「已合併到「{勝方}」」 | N（同 4a §5 #31 的短句） |
| 49 | `reconciliation/page.tsx` 失敗 | 刪「已處理」鎖列（F6）；失敗時頁面錯誤提示顯示伺服器句子；後備句 `:164`「歸因綁定失敗，請確認網路狀態後再試一次。」保留（4a E2） | E |
| 50 | `reconciliation/page.tsx` 確認框讀取 | 補讀業者時「正在讀取業者資料…」；失敗「無法讀取業者資料。」＋「重試」 | N（同 #41 句型） |
| 51 | `ProspectMatchDialog` 已入駐的列 | 不顯示「與這家合併」，改顯示「已入駐，不能合併」 | N（**再確認**，附錄 A M-2） |
| 52 | `ProspectMatchDialog` 建立、還原模式說明（`:50`、`:54`，4a 已核可的句子） | 建立：「下列業者的聯絡方式與這次填寫的相同。是同一家的話請取消：進行中的請改去編輯它，已入駐的不需要再建立；確認都不是同一家，逐一勾選後再送出。」；還原：「下列業者的聯絡方式與這筆相同。是同一家的話不需要還原；確認都不是同一家，逐一勾選後再還原。」（拿掉「進行中的」） | N（**再確認**，附錄 A M-2；相符的業者包含已入駐的，F8） |
| 53 | `MergeDialog` 讀到有一邊不是進行中 | 停用送出，說明「「{名稱}」目前不是進行中，不能合併。」 | N（**再確認**，附錄 A L-12） |
| 54 | 比對表新增兩列 | 「業者型態」；「logo／封面」值「有／沒有」（例如「logo 有、封面沒有」） | N（**再確認**，附錄 A L-9） |

---

## 6. 測試

### 6.1 unit（`admin/tests`）

- 預設勝方：聯絡方式以類型數計入；平手取 ID 數值小的（F4）。
- `kycStatusText` 搬家後原測試照跑。
- 刪 `seller-prospects-contract.test.ts:9,23-27`。

### 6.2 行為（`dom-harness`）

- 疑似重複：翻頁帶 `page_token`；「不是同一家」送出的兩個 ID 等於該列；成功後清單重讀。
- 合併：開啟時兩邊各 `GetProspect` 一次；送出的 `winner_etag`、`loser_etag` 來自讀到的版本，切換勝方後兩者對調。**突變**：把兩個 etag 對調送出時測試要失敗。`VERSION_CONFLICT` → 重讀、勾選清掉、顯示 #41。
- 修改模式：相符列中進行中的有「與這家合併」、已入駐的沒有；按下後重送取消、合併框以兩個 ID 開啟。
- 修改撞統編：`failed_precondition`＋`PROSPECT_TAX_ID_HELD`＋`prospectIds` → 提示有合併按鈕；持有者已入駐 → 沒有。沒有 reason 的 failed_precondition → 只有句子（**突變**：client 改成只看錯誤碼時要失敗）。
- 從編輯頁合併成功：目前這筆是輸家 → 導到勝方；是勝方 → 表單以新版本重設。
- 統編對帳：`already_exists` → 框關閉、提示顯示伺服器句子、清單重讀；`VERSION_CONFLICT` → 留在框內、重讀那一筆，**重送帶的是新的 etag**（附錄 A M-4）。
- 從統編出口開的合併框，預設勝方是持有者；從修改模式相符列開的，照完整度規則。
- 合併沒收到回應 → 兩邊各重讀一次、顯示 #41 的新句子。
- 相符對話框重送撞到 `PROSPECT_TAX_ID_HELD` → 對話框關閉、編輯頁出現統編出口。
- 已入駐的相符列沒有合併鍵、有 #51 的說明。

### 6.3 backend

- §2 的 integration 測試；`apperr` 的 reason 清單測試；`TestEveryRouteIsInOpenAPISpec`、`openapi_coverage_test`（465）。

## 7. Commit 計畫

| # | 內容 | 審查 |
|---|---|---|
| 4b-0 | 契約（F1、F8）與三個生成端、`apperr`、模組 `errTaxIDHeld` 與 `LiveProspectByTaxID`、測試 | code-review |
| 4b-1 | admin 全部（§3）；同一個 commit 刪 gateway 4 條路由與整檔、`maxRESTOperations` 465、json-contract 與 legacy baseline 的 `--update` | code-review |
| 4b-2 | 文件：計畫 10e 的 4b 完成紀錄；seller `CARRYOVER.md:181`（邀請面板已刪）、`:306`（client 路由已全刪）；prospect ADR 文末修訂（D1：修改與還原撞統編帶 `PROSPECT_TAX_ID_HELD`；live 的定義）；db :200 的 469 改 465 | 併入完成前的 design-review |

design-review 兩次：Will 決定後、派 4b-0 之前一次；4b-2 之後看整刀。實作派 Sonnet subagent，主 session 寫派工、驗證與審查。

## 8. 驗證命令（派工前照當時的 `.github/workflows/` 再核對）

- api（`api-contract.yml`）：`buf lint`、`buf breaking`、三個生成端的 drift。
- 單體（`backend-monolith*.yml`）：gofmt、`go vet`（含 `-tags integration`）、`go test ./internal/...`（integration 帶 `DOCKER_HOST`）、`make lint-monolith`（只看新增）、`GOWORK=off go mod tidy` 無 diff。
- admin（`admin-ci.yml`）：`pnpm --filter admin lint`（含 `tsc`、`check-enum-text`）、`test`、`build`。
- gateway：gofmt、`go vet ./...`（含 `-tags integration`）、`go test ./...`；`gateway-json-contract-check.py`。
- 跨專案：`scripts/check-enum-text-cross.sh`、`scripts/check-migration-inventory.py`（`backend-migration-inventory.yml`）；最後 `scripts/check-legacy-endpoints.py --update` 再跑一次不帶。
- 改 `api/**` 與 `packages/hawkr-api-ts/**` 觸發的：`ui-text-generated-diff.yml`（`make -C backend uitext` 後 `git diff --exit-code`）、`ui-text-guard.yml`、`ts-packages.yml`（`@hawkr/api` 的測試）、`website-ci.yml`（website 的 lint、test、build，build 帶 `API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值的 `NEXT_PUBLIC_API_BASE_URL`）。
- 改 `packages/hawkr_api/**` 觸發的：`buyer-ci.yml`、`seller-ci.yml`（`AGENTS.local.md` 的 commit 前清單：兩支 App 的 analyze、custom_lint、完整測試、`scripts/deadcode.sh`）與 `packages/hawkr_api` 的測試。
- 改 `backend/gateway/**` 觸發的 `backend-go-checks.yml`。
- 派工前逐一打開上面的 workflow 核對實際命令（附錄 A M-7）。

## 決定紀錄

- Will 2026-10-21「都按照建議」：F1 改契約加 `PROSPECT_TAX_ID_HELD`；§5 的 N 類文案照提案（含伺服器句子 #46、#47）。審查若改到已核可的句子，再交 Will 確認改過的那幾句。
- Will 2026-10-21「好」：v1.1 標「再確認」的文案（#35、#38、#41、#43、#46、#51–54）照提案；F14 照建議（#35 說實話，一般的「合併到其他業者」維持 §0.2 的觸發）。

## 實作時的修訂

- 4b-0：並行寫入同一個統編的保底，先做成「交易外重查持有者、查不到重跑一次、再撞回新的 `apperr.Aborted`」，完成前 design-review（附錄 C #4）後改成交易級 advisory lock：建立、修改（統編有改）、還原在檢查持有者之前以統編取 `pg_advisory_xact_lock`，同一個統編的寫入排隊，應用層的檢查就是權威；重跑、`KindAborted` 與那句待核可的新句子一併刪除。合併搬統編不取鎖（輸家清除與勝方取得在同一個交易，統編始終有持有者）。
- 4b-1：`MarkDistinctDialog` 沒收到回應時的後備句用目錄的預設錯誤文字（`common.error.UNAVAILABLE`），不新增句子；關閉鈕用 `Dialog` 的預設標籤。對帳頁確認框的業者讀取狀態放在 `ConvertConfirmDialog`（props `prospectLoading`、`prospectLoadFailed`、`onRetryProspect`）。統編出口的合併按鈕放在錯誤提示下方。合併沒收到回應或其他非衝突失敗時，除了重讀兩邊也讓清單類查詢失效（code-review 4b-1 LOW）。

## 9. 沒有驗證的部分

- 管理員 token 經通用上傳的 live 驗證（4a §9）仍未做，與本刀無關。
- 業界的預設勝方做法（F4）沒有重新查證。
- json-contract check_a 的確切變化以實作時的輸出為準。

---

## 附錄 A：審查意見的處理（v1 → v1.1；審查全文在 session scratchpad `4b-design-review.md`）

| # | 意見（嚴重度） | 處理 |
|---|---|---|
| H-1 | #38「分類會併入」：合併併入的是沒有畫面的 `category_ids`，畫面上的「分類」不動（HIGH） | **改**：F5、#38；**再確認** |
| M-1 | #35 說合併會撤銷標記，但畫面沒有路徑合併已標記的一對（MED） | **改**：#35 說實話；一般合併入口列 F14 **再確認** |
| M-2 | 相符的業者包含已入駐的，文案與出口沒處理（MED） | **改**：F2、#43、#51、#52；**再確認** |
| M-3 | 並行保底可以重查持有者，兩種錯誤形狀是特例（MED） | **改**：F1、§2；§0.2 刪這條 |
| M-4 | 對帳衝突時重讀的對象錯了（MED） | **改**：F6、§3.4、§6.2 |
| M-5 | 統編出口的預設勝方可能清掉管理員要的統編（MED） | **改**：F4 依進入點，統編出口預設持有者 |
| M-6 | seller `AGENTS.md:128`、`CARRYOVER.md:173,374` 與約 15 處註解漏列（MED） | **改**：§1、§2 |
| M-7 | 驗證命令漏了會觸發的 CI（MED） | **改**：§8 |
| M-8 | 沒收到回應時說「都沒有變動」可能是假的（MED） | **改**：#41；**再確認** |
| L-1 | 還原也帶 reason，沒有 client 分支 | **記**：F1 寫明理由（同一個錯誤條件一個形狀） |
| L-2 | F11 與 4a 的 `useProspectCache` 重複、與附錄 C #5 牴觸 | **改**：F11 |
| L-3 | reason 清單測試是寫死的 | **改**：§2 |
| L-4 | `PreviewProspect` 也用 `LiveProspectByTaxID` | **改**：§2 |
| L-5 | ID 用字串比大小 | **改**：F4、§3.5 以數值排序 |
| L-6 | `draftWillBeLost` 恆真；勝方重設要照 `send` 的步驟 | **改**：F12、§3.5 |
| L-7 | `ConvertPanel` 描述錯；`certificationStatus` 應是 enum | **改**：F7 |
| L-8 | 行號 | **改**：H8、#49 |
| L-9 | 比對表缺業者型態與圖片 | **改**：§3.3、#54；**再確認** |
| L-10 | #35 的「建立」不成立 | **改**：#35 |
| L-11 | #46 沒有下一步 | **改**：#46；**再確認** |
| L-12 | 合併框沒檢查狀態；重送撞到統編沒有出口 | **改**：F2、F4、#53 |

## 附錄 B：開工前的 design-review（v1.1 → v1.2，6 項；全文在 session scratchpad `4b-design-review-start.md`）

| # | 發現 | 處理 |
|---|---|---|
| 1 | 並行保底查不到持有者時借用 `VERSION_CONFLICT`，語意不符、還原也不帶 etag（B，高） | **改**：伺服器重跑一次，再失敗回不帶 reason 的 `aborted` |
| 2 | 建立的統編衝突沒有 reason，與 F1 的「一個條件一個形狀」矛盾（A，中） | **改**：建立也帶 `PROSPECT_TAX_ID_HELD`，錯誤碼照舊（先例 `VERSION_CONFLICT`）；client 出口仍在 §0.2 的觸發後 |
| 3 | `ConvertProspectSummary` 轉接型別的必要條件已消失；兩個呼叫端失敗處理不同沒寫理由（A，中） | **改**：直接收 `Prospect`；**記**：失敗處理不同的理由寫進 F7 |
| 4 | 合併後果文案取自實作，契約註解漏兩項（B，中） | **改**：4b-0 補契約，F5 改引用契約 |
| 5 | 合併按鈕同時看 `onMerge` 與 `mode`（B，低） | **改**：只看 `onMerge` |
| 6 | 輸家的詳情失效沒有歸屬，可能繞過 `useProspectCache`（B，低） | **改**：`useProspectCache` 加 `retired(id)` |

指令檔對帳：本刀範圍內的 seller `AGENTS.md:128` 與 `CARRYOVER.md:173,181,306,374` 已列進 4b-0、4b-2；admin `AGENTS.md` 沒有過時的句子（Dialog、toast、design token 規則本刀照用）。

## 附錄 C：完成前的 design-review（7 項；全文在 session scratchpad `4b-design-review-done.md`）

| # | 發現 | 處理 |
|---|---|---|
| 1 | `runAction` 的結果沒有錯誤碼與「沒收到回應」，兩個呼叫端各自包 try/catch 繞過（B，高） | **改**：`ActionResult` 的失敗分支帶 `code`、`noResponse`，刪兩處繞法 |
| 2 | 並行的新句子待 Will 核可（C，高） | **改**：隨 #4 刪除，不再需要核可 |
| 3 | `conflictError` 仍把統編唯一違反轉成不帶 reason 的 `AlreadyExists`（B，中） | **改**：刪該分支；有鎖之後撞到唯一約束代表程式有錯 |
| 4 | 為罕見的並行路徑加了重查、重跑、`KindAborted` 一整套（B，中低） | **改**：交易級 advisory lock（PostgreSQL Explicit Locking 的 Advisory Locks），刪重跑與 `KindAborted` |
| 5 | `MergeDialog` 用自訂查詢 key 與手動 client，繞過共用快取（B，低） | **改**：兩個 `useQuery(getProspect)`，與詳情頁同一個 key |
| 6 | 重送流程寫死 `PROSPECT_TAX_ID_HELD` 的特例（B，低） | **改**：帶 reason 的其他失敗一律關框交給 `onFailure`，沒 reason 的留在框內 |
| 7 | 合併時勝方整筆保留，不能逐欄挑值（A，低） | **記**：逐欄挑值要改契約（MergeProspects 帶要保留的欄位值）；主流 CRM 的合併畫面可以逐欄選值［外部，本次未查證］。重新評估的觸發：BD 反映合併前常要先手動抄欄位 |

