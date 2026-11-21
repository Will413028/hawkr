# 10f-3 我的檢舉與補充說明（R7），設計

來源：`step10e-reports-brief.md` R7「信任與安全上線前那一刀做的」；計畫 10f 範圍確認（Will 2026-11-20）；`step10f-2-design.md` G1；盤點對照 dev `50d30a50`（2026-11-21）。決定 H1–H4（含 H1b、H1c、H2b）：Will 2026-11-21 全照建議。兩輪獨立審查已處理（第 1 輪 findings 11，blocking 1，全 fixed；第 2 輪 findings 5：fixed 4、rejected 1）。

使用者面名稱是「我的檢舉」（`docs/共通規範/使用者面用語術語表.md` L48–49）；「通報」是後台用語。前提：系統未上線，刪 RPC 不需相容窗口，回滾就是 revert。

## 現況（盤點）

- 兩支 `ListMyReports`：
  - moderation `ModerationAppService.ListMyReports`（`api/hawkr/moderation/v1alpha1/moderation.proto:23`）→ `ReportRecord`：`ReportTarget`（POST、COMMENT、FORUM_ENTRY、BUYER）、說明、`ReportOutcome`（PENDING／VIOLATION／NO_VIOLATION，`:38–47`）；無結果說明、無補充；`page_size` 上限 50。模組函式 `moderation.Module.MyReports(ctx, buyerID, after *ReportCursor, size)`。
  - seller `SellerReportAppService.ListMyReports`（`api/hawkr/seller/v1alpha1/report.proto:24`）→ `MyReport`：店家、商品、訂單編號、`resolution_note`、`ReportOutcome`（盤點時以 UNSPECIFIED 表示處理中，H6 修訂後改為 `optional`、沒設值＝處理中；ACTION_TAKEN／NO_VIOLATION／INSUFFICIENT／DUPLICATE／ABUSIVE，`:29–41`）；店名與 logo 由 client 呼叫 `BatchGetStores`（`:44–45`）；`page_size` 上限 100。handler `backend/internal/httpapi/seller/reports.go:143`。
- 補充：只有 seller 的 `report_supplements`，由 `CreateReport` 去重時寫入（上限 10 則），寫入時發 `ReportReceived`（帶 SupplementID，`backend/internal/modules/seller/events.go:33`）並進 admin 的「有新的補充」推導（`supplements_seen_seq`）。通報人沒有主動補充的 API；moderation 沒有補充。
- client：buyer-app `features/moderation/{data/moderation_repository.dart, presentation/my_reports_notifier.dart, my_reports_screen.dart}` 只呼叫 moderation 那支；seller 那支沒有 client；website 沒有頁面。
  ［量測：`rg -ln "ListMyReports|listMyReports" website buyer-app/lib admin/src --glob '!**/gen/**'`］

## 決定題

| # | 題目 | 選項 | 建議 |
|---|---|---|---|
| H1 | 「讀取時組裝」放哪裡 | A：client 各叫兩支再合併（兩端各寫一次合併與分頁，違反「同一邏輯不在兩個 client 各寫」）；B：伺服器端 API composition：擴充 moderation 的 `ListMyReports`，handler 在 `httpapi` 層從兩個模組讀取並合併分頁，刪除 seller 的 `ListMyReports`；C：事件投影成讀取表（R7 已定讀取時組裝，且無效能需求） | **B**。模組之間不互相依賴，組裝只在 httpapi；A6 的聊天、評價檢舉加成新成員 |
| H1b | 合併後的項目形狀（兩套 `ReportOutcome` 值不同） | A：共用欄位（id、category、description、create_time、resolve_time）＋`oneof subject { CommunityReport community; StoreReport store; }`，兩套結果各留在成員裡（social 用 moderation 的、store 用 seller 的，含 `resolution_note`）；`resolve_time` 兩邊都有來源（moderation `ReportRecord.resolve_time` 是案件結案時間，`moderation.proto:93`）。`StoreReport` 是 seller 的 `MyReport` 改名，留在 `seller/v1alpha1/report.proto`，moderation proto import 它，`GetMyReport` 的回應也用它；B：統一成一套新 enum（要定兩套的對應，會丟掉 INSUFFICIENT／DUPLICATE 等區分或替社群硬造值） | **A**。兩套結果語意本來就不同（案件推導 vs. 每筆結果），H6 修訂：文案目錄只有 moderation 那套有文字 |
| H1c | 店名、logo | A：照現有契約，client 以 `store_id` 呼叫 `BatchGetStores`；B：伺服器在組裝時帶入 | **A**。與店家卡片其他頁一致；合併清單不新增對 seller 公開資料的耦合 |
| H2 | 通報人的補充說明 | A：只給店家、商品檢舉加 `AddReportSupplement`；B：A＋moderation 也加補充表 | **A**。社群檢舉是一鍵檢舉（說明 500 字選填），Instagram、YouTube 也不提供補充；觸發條件：A7 的申訴或案件需要補充時 |
| H2b | 主動補充的上限 | A：與去重補充共用同一串、同一個 10 則上限（術語表「補充」改成涵蓋兩種來源）；B：分開兩串、各自上限 | **A**。admin 看到的是同一串補充，`supplements_seen_seq` 與事件只有一條路徑 |
| H3 | 詳情 | A：清單直接帶補充；B：清單不帶，店家檢舉另有 `GetMyReport` | **B**。與 admin `GetReport` 同樣分法。欄位：`StoreReport` 全部欄位＋補充（內容、時間、佐證）＋原始佐證；佐證只回 `position`，畫面顯示「已附 N 張佐證」（H5） |
| H5 | 佐證縮圖、補充限流（10f-3a 實作時發現，Will 2026-11-21 照建議） | 縮圖：A 檢舉人讀自己佐證的路由；B 只顯示張數。限流：A `AddReportSupplement` 與 `CreateReport` 各自計數；B 共用額度 | 縮圖 **B**（延後，觸發條件：使用者回饋需要回看佐證，或 admin 機敏檔路由開放檢舉人讀取）；限流 **A**（補充本身上限 10 則） |
| H6 | seller `ReportOutcome` 的買家面文字（10f-3b 開工時發現目錄缺；Will 2026-11-21） | A：加 `REPORT_OUTCOME_PENDING`；B：照 API 設計規範（L102，L-brief Q4）把 `StoreReport.outcome` 改成 `optional`，沒設值＝還沒有結果 | **B**（初選 A，審查發現與規範衝突，Will 2026-11-21 改照規範）。目錄補 5 個結果值與一個非列舉字串「審核中」（client 在 outcome 沒設值時用）。文案：審核中／已處理（已採取措施）／已處理（不違規）／已處理（資料不足）／已處理（重複檢舉）／已處理（不受理）；en：Under review／Resolved (action taken)／Resolved (no violation)／Resolved (insufficient information)／Resolved (duplicate report)／Resolved (not accepted)。moderation `ReportOutcome` 的 PENDING 是既有偏離，延後（觸發條件：A7 再改 moderation 契約）。由 10f-3a2 做 |
| H4 | 成功句 | 10f-2b 留下的「處理結果可在「我的檢舉」查看」兩端加回 | 照做（Will 2026-11-21 已提） |

UI 版面（清單項目、詳情頁、補充表單）在 10f-3b 開工前給 Will 同意（共通規則「UI 改動」）。3a 的欄位由 H3 定死，不等 UI 同意；UI 同意若要新欄位，回到 3a 補。

## 合併分頁（H1 選 B 時）

- 排序鍵 `(create_time DESC, source, id DESC)`，`source` 固定順序 community < store。
- 游標是不透明 token，記雙方各自的最後位置（兩邊的 `ReportCursor` 都是 `(CreatedAt, ID)`）。一方只有在「回傳不到 `size+1` 筆，而且回傳的每一筆都已輸出」時才記 exhausted；本頁沒輸出到的一方保留原游標；`rpc.PageScope` 改用 moderation 的 procedure 名稱與 buyerID。
- 每頁：兩邊各取 `size+1`，合併後取前 `size`，各自游標推進到已輸出的最後一筆。`page_size` 預設 20、上限 50（沿用 moderation 現行）。

## 不變量

- 主動補充與去重補充走同一條寫入路徑（seller 模組的同一個函式）：同樣發 `ReportReceived`（帶 SupplementID）、同樣進 admin 的「有新的補充」推導、同一個 10 則上限。依賴者：`seller/events.go` `ReportReceived`、`cases.sql` 的 unseen 推導、admin `MarkReportSupplementsSeen`。
- 檢舉人只讀寫自己的檢舉：`GetMyReport`、`AddReportSupplement` 對別人的或不存在的 id 一律回 not_found（不洩漏存在與否）。

## 切分與檔案

- **10f-3a backend 與契約**
  - 修改：`api/hawkr/moderation/v1alpha1/moderation.proto`（項目形狀 H1b）、`api/hawkr/seller/v1alpha1/report.proto`（`MyReport` 改名 `StoreReport`，新增 `GetMyReport`、`AddReportSupplement`）、`backend/internal/httpapi/moderation/app.go`（組裝）、`backend/internal/httpapi/seller/reports.go`、`backend/internal/httpapi/effects.go`（加 `GetMyReport` 讀、`AddReportSupplement` 寫）、seller 模組的補充寫入與查詢、`docs/共通規範/使用者面用語術語表.md`「補充」列、`backend/internal/modules/seller/CARRYOVER.md:365`。
  - 刪除：seller `SellerReportAppService.ListMyReports` RPC 與其 request／response、handler `backend/internal/httpapi/seller/reports.go` 的 `ListMyReports`（L143 起）與測試、`effects.go:127` 條目。
  - 生成：`make connect`（backend）、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 的 `../../api/generate.sh buf.gen.yaml`。
- **10f-3b buyer-app**：修改 `features/moderation/` 三支檔與測試，新增店家檢舉詳情與補充畫面；店家檢舉成功句加回。
- **10f-3c website**：新增「我的檢舉」清單與詳情路由、補充表單；成功句加回；e2e 推給 CI。

## 消費端（重跑）

```
rg -l "ListMyReports|listMyReports|ReportRecord|MyReport\b|report_supplements|ReportReceived" --glob '!**/gen/**' --glob '!*.pb.go' --glob '!*.connect.go' --glob '!node_modules' --glob '!backend/services/**' .
```

## 驗收

- 10f-3a：`make lint-monolith`、`go test ./cmd/... ./internal/...`；`-tags integration` 的測試（起 DB 容器）推給 CI，回報為待 CI。integration：兩來源交錯的時間序合併、同一 `create_time` 的穩定順序、跨頁游標無重複無遺漏、一方取完後的翻頁、一方的尾端剛好跨過頁界、只看得到自己的檢舉；`GetMyReport`、`AddReportSupplement` 對別人的 id 回 not_found；有結果後補充回 failed_precondition；去重補充與主動補充合計第 11 則回 failed_precondition；主動補充後 admin 清單的「有新的補充」為真且發出 `ReportReceived`。`buf lint`、三條生成指令跑完 `git diff --exit-code`。
  - mutation：合併只取一個來源，交錯測試要失敗；拿掉「有結果不能補充」，測試要失敗；`GetMyReport` 拿掉擁有者條件，not_found 測試要失敗。
- 10f-3b：`fvm flutter analyze`、`fvm flutter test`、`dart run build_runner build` 無 drift；單元測試鎖兩種項目都顯示、已有結果不顯示補充入口、成功句含「我的檢舉」。mutation：補充入口改成永遠顯示；刪掉成功句後半；測試各要失敗。
- 10f-3c：`pnpm lint`（含 `tsc`）、`pnpm test`；同上三條單元測試與 mutation；e2e 推給 CI。

## 不在本步驟

- buyer-app 店面頁在自己的店顯示「⋯ → 檢舉」、website 選單在 `useSellerSelf` 載入前短暫顯示於自己的店：本步驟不動店面與選單，兩項延後維持（觸發條件：店家 API 加擁有者欄位）。
- 社群檢舉的補充：H2 的觸發條件。
- 聊天、評價的檢舉：A6。
