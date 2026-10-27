# 10e 第 3c、3d 刀盤點（文案一次核准用）

快照：worktree `hawkr-10e`，HEAD `65be53b77`，工作區上還有 3b 沒提交的改動（46 檔）。A7 計畫讀的是主 checkout（`fbdea5137`）。只讀，沒有跑任何 build 或測試。
標記：**[量測]** 這次用指令讀到；**[推論]**；**[猜測]**。

---

## 0. `maxRESTOperations` 的現值與預期值

| 量測點 | 值 | 指令 |
|---|---|---|
| HEAD 的常數 | 489 | `git show HEAD:backend/gateway/internal/router/openapi_coverage_test.go \| rg -n "const maxREST"` |
| 工作區的常數（3b 沒提交） | 488 | `rg -n "const maxRESTOperations" backend/gateway/internal/router/openapi_coverage_test.go` |
| 工作區 openapi 的實際 operation 數 | 488 | 用 python yaml 數 `backend/api/openapi/openapi.yaml` 的 paths×{get,post,put,patch,delete} |
| 剩下的治理路由 | admin 7 條、非 admin 4 條 | `backend/gateway/internal/router/routes_governance.go`；openapi 裡含 `violation\|food_safety` 的 11 個 operation |

- **簡報寫的 480 差了 1** [量測]：10d 的 3a-① social 刀（`36fb5d894`）把常數從 492 改成 493（為 `GET /app/sellers/posts/:id` 開了例外）。3a 刀是 492 → 488，合併之後 HEAD 是 489。
- **預期**：3b 提交後 488；3c 刪 admin 7 條 → **481**；3d 不刪路由，維持 **481**。如果 10d 的 K1（非 admin 4 條）先落地，就是 477 → 477，而且 3c 要刪整個 `routes_governance.go`。A7-4e 再扣 37 條。

---

## 1. 各刀的檔案清單

### 3c 通報（487→480 的那一段，實際是 488→481）

**契約**（`api/hawkr/admin/seller/v1alpha1/governance.proto`）
- `Report` 新增欄位，都是 OUTPUT_ONLY：
  - K4′ 的 8 個布林值：`can_acknowledge`、`can_start_investigation`、`can_verify`、`can_dismiss`、`can_export_fda`、`can_record_fda_referral`、`can_request_cooperation`、`can_cite_as_source`
  - `store_name`（K5′）
  - `assessed_category`（D9，型別 `seller.v1alpha1.ViolationCategory`，確認收件前是 UNSPECIFIED）
  - `repeated string resulting_measure_ids`（D10）
- `AcknowledgeReportRequest.assessed_category`，REQUIRED。
- 新增 `CreateReportNote`、`ListReportNotes`，連同 4 個 Request／Response 訊息（形狀照 `CreateMeasureNote*`／`ListMeasureNotes*`，`governance.proto:523-546`）。註記的訊息型別要決定（見 Q8）。
- `ListReportsRequest` 可能要改（見 Q2）。
- 註解要改的：`:22` ListReports 改成「瀏覽清單」；`:44-46` RecordFDAReferral／GetFDAExport 的「只限食安」改成看 `assessed_category`；`:201` 的 `acknowledge_due_time` 這刀不動（3d 才動）。
- 重新生成：`make connect`（`backend/internal/httpapi/gen/...`）、`pnpm --filter @hawkr/api gen:api`（`packages/hawkr-api-ts/src/gen/.../governance_pb.ts`）。`packages/hawkr_api` 不生成 `hawkr.admin.*`，所以 Dart 不用重新生成（L′ 設計:259）。

**權限**
- `backend/internal/platform/adminperm/adminperm.go`：新增 `SellerReportNotesCreate`、`SellerReportNotesList`（`seller.report_notes.*`），並加進 `Catalog()`。
- `backend/internal/httpapi/adminauth/adminauth.go:87-112`：加 2 列 procedure 對權限。

**migration**（只有 up，照 3a 修訂 3）：新的一支 `backend/internal/modules/seller/db/migrations/<時間戳>_seller_report_assessment_notes.sql`，同時更新 `atlas.sum` 與 `db/schema.sql`。
- `reports` 加 `assessed_category TEXT`。
  - CHECK 值域同 `reports_category_check`。
  - 併進 `reports_acknowledged_check`：`(acknowledged_at IS NULL) = (assessed_category IS NULL)`。
  - `reports_fda_check` 的 `category = 'FOOD_SAFETY'` 改成 `assessed_category = 'FOOD_SAFETY'`。
  - 回填：已確認收件的列設 `assessed_category = category`。
- `governance_notes` 加 `report_id UUID NULL REFERENCES seller.reports(id)`。
  - `measure_id` 改成 NULL。
  - CHECK `num_nonnulls(report_id, measure_id) = 1`。
  - 新索引 `governance_notes_report (report_id, created_at, note_id)`。
- 時間戳要晚於 3b 的 `20261018000223`。A7-1c 約定 seller migration 取 main 上最新一支之後的時間戳。

**backend（seller 模組）**
- `internal/modules/seller/internal/model/governance.go`
  - `Report` 加 `AssessedCategory`、`StoreName`、`ResultingMeasureIDs`。
  - 新增 `Report.Can*` 判定式，和 `governance/report.go` 的前置檢查共用（K4）。
  - `AcknowledgeReportInput` 加 `AssessedCategory`。
- `internal/modules/seller/internal/governance/report.go`
  - `:165` Acknowledge 要驗證 `assessed_category`。
  - 前置檢查改用 model 的判定式：`:180,208,243,284,319,383`。
  - FDA 的前置檢查改看 `assessed_category`。
  - `:350` Report、`:398` AdminReports 要帶店名與 resulting ids。
- 註記：`internal/modules/seller/internal/measure/notes.go` 的驗證（`model.Required(…,2000,"Body","內部註記")`）抽成共用的，再新增通報的版本（放在 governance 或共用的 notes 套件）。
- `internal/modules/seller/db/queries/reports.sql`
  - `AdminReports`、`ReportByID` 要 JOIN 店名、帶 `assessed_category`。
  - 新增 `AcknowledgeReport` 的參數。
  - 新增 `ReportResultingMeasures`：`finding_reports` → `measure_findings`。
- `internal/modules/seller/db/queries/measures.sql:215-222`：新增 `InsertReportNote`、`ReportNotes`，或者把 note 的查詢搬到 reports.sql。
- 跑 sqlc 生成 `internal/modules/seller/internal/store/*.sql.go`。
- `internal/modules/seller/governance.go`：module facade 新增 `CreateReportNote`、`ReportNotes`。
- `internal/modules/seller/internal/measure/create.go:484`：來源通報的檢查改用 `Report.CanCiteAsSource`，和 K4 同一份。
- 測試：
  - `governance_integration_test.go`、`migration_integration_test.go`（新 migration）
  - `internal/model/*_test.go`：`can_*` 與前置檢查一致的資料集測試
  - `internal/purge`：沒有引用 reports／notes，不用改（D4′ [量測] 引用自簡報）

**httpapi**
- `backend/internal/httpapi/seller/governance.go`：`toReport` 的新欄位；`AcknowledgeReport` 接 `assessed_category`；新增 2 個 note handler。
- `backend/internal/httpapi/seller/governance_test.go`、`admin_integration_test.go`。

**gateway（刪除）**
- `backend/gateway/internal/router/routes_governance.go`：刪 admin 的 3 個 group（7 條路由，`:63-77`）與檔頭註解的 admin 那幾行。非 admin 的 4 條留著。
- `backend/gateway/internal/handler/governance.go`
  - 刪 7 個 handler：`AdminListViolationReports:372`、`AdminPatchSellerViolationReportStatus:410`、`AdminPatchFoodSafetyReportStatus:449`、`AdminAcknowledgeFoodSafetyReport:485`、`AdminGenerateFDAReport:520`、`AdminMarkFDAReported:557`、`AdminNotifySeller:600`。
  - 刪只有它們在用的 `toFoodSafetyReportDTO:103`、`toUnionReportRowDTOs:171`，以及 `FoodSafetyReportDTO`、`UnionReportRowDTO` 型別。
  - `toViolationReportDTO` 留著：`ListMyViolationReports` 還在用。
- `backend/gateway/internal/grpc/governance_client.go:46-87`：刪 7 個 `Admin*` wrapper。
- `backend/gateway/internal/handler/governance_test.go`：刪 `:52` 的 union DTO 測試。
- `mockSellerServiceClient`（`testutil_test.go`）留著（K13）。
- `backend/api/openapi/openapi.yaml`：刪 7 個 operation，以及只有它們在用的 schema。
- `backend/gateway/internal/router/openapi_coverage_test.go:28`：改成 481。
- `admin/tests/governance-wire-contract.test.ts`：刪除。
- `scripts/legacy-endpoints-baseline.json:38-40`：刪 `governance-wire-contract.test.ts` 那一筆，或跑 `--update`。

**admin**
- `admin/src/app/admin/violation-reports/page.tsx`（450 行）：整頁改寫成 Connect `ListReports`。
- `admin/src/app/admin/violation-reports/[source]/[id]/page.tsx`（618 行）：刪除，新建 `admin/src/app/admin/violation-reports/[id]/page.tsx`（K1）。
- 新增的元件（建議）：
  - `violation-reports/report-list.ts`：狀態 label、篩選、期限文字
  - `violation-reports/[id]/report-actions.ts`：build request、錯誤句
  - 對話框 `AcknowledgeReportDialog.tsx`、`ResolveReportDialog.tsx`、`RecordFDAReferralDialog.tsx`、`RequestCooperationDialog.tsx`
  - `fda-export.ts`：組 `.md`，K2
  - `ResultingMeasuresSection.tsx`
- 共用註記元件：`admin/src/app/admin/measures/[id]/MeasureNotesSection.tsx` 改成參數化的 `NotesSection`（list／create 的 mutation 由呼叫端給），移到共用的位置（例如 `admin/src/app/admin/governance/NotesSection.tsx`）。標題 id `measure-notes-title` 要改成通用的。
- 通報詳情掛 `CreateMeasureDialog`（3b 的 `admin/src/app/admin/merchants/[id]/CreateMeasureDialog.tsx`，`initialSourceReportIds`）。
- **簡報「實作時的修訂（3b）」第 6 點（Will 2026-10-18 核准，排進 3c）**
  - 6-2 `admin/src/app/admin/merchants/[id]/SourceReportsField.tsx:45`：client 過濾 `report.status !== ReportStatus.DISMISSED` 改成 `report.canCiteAsSource`（K4，伺服器推導）。
    - `:25-26` 的註解跟著改。
    - `ReportStatus` 的 import（`:6`）不再需要。
    - 依賴 3c 契約的 `Report.can_cite_as_source`，所以要和 K4 同一個 commit。
  - 6-1 `admin/src/app/admin/merchants/[id]/CreateMeasureDialog.tsx:38-79`：加初始值的 props，機制和 `initialSourceReportIds` 相同，都是 `useState` 的初始值。
    - 例如 `initialKind?: MeasureKind`、`initialEndTime?: Date`、`initialStatement?: string`。
    - 類別不預填（`category` 維持 UNSPECIFIED）。
    - 結束時間轉成 `datetime-local` 用既有的 `toLocalInput`（`merchants/[id]/measure.ts:295`）。
    - request_id 照舊在開啟時產生；預填的內容等同使用者輸入，改過就換新的。
  - 6-1 `admin/src/app/admin/merchants/[id]/violation-measure.ts`：建議的轉換函式，例如 `measureInitialFromSuggestion(recommendation, now)`。
    - 種類 = 停權。
    - 結束時間 = 現在 + `suggestedSuspendDays`×24h。`types.ts:871` 是字串；空字串、0 或不是正整數時不預填結束時間，由既有的「請填寫結束時間。」擋下 [推論：簡報沒寫這個邊界]。
    - 理由 = `suggestedReason`；空字串時不預填。
  - 6-1 `admin/src/app/admin/invoices/complaints/[complaintId]/page.tsx`
    - `:534` 欄標題改成「建議理由」。
    - `:540-548`「依建議建立措施」開啟時帶上述初始值。
    - `:147` 的 `measureStoreId` 改成 `{ storeId, initial? }`：右欄 `:560-565` 的常駐「建立措施」照舊不預填。
    - `:575-579` 的掛載傳入初始值。
    - 檔頭註解 `:97-102`「帶著剛拿到的建議理由」改成預填三項。
  - 測試
    - `admin/tests/merchant-create-measure-dialog.test.ts`：`:192`、`:296-302` 的 fixture 改設 `canCiteAsSource`。要有一筆 `status=PENDING` 但 `canCiteAsSource=false`，斷言它不會列出，用來證明 client 不再看狀態。另加一個測試：初始值預填之後可以修改，送出的請求帶修改後的值，類別仍要選。
    - `admin/tests/invoice-adm-contract.test.ts:1116-1127`：斷言「依建議建立措施」帶預填、常駐鍵不帶，欄標題是「建議理由」。
- `admin/src/components/Layout/Sidebar.tsx:39`：把「商家違規 / 調查」從「內容治理」分組移到「賣家 / 商品」，改名「商家通報」（v1 D6-1／D6′-1）。
- 註解要改的路徑：`admin/src/components/UI/SlaCountdown.tsx:23`、`admin/src/styles/pages/Invoice.css:178`。
- 測試
  - `admin/tests/measure-pages-wiring.test.ts:83-84`：改成斷言 `[id]` 新路由存在、`[source]/[id]` 已刪。
  - 新增 `admin/tests/report-pages-wiring.test.ts`（或類似名稱）：期限文字、`can_*` 對按鈕、`.md` 的組裝。
  - `admin/tests/admin-governance-modal-a11y.test.ts`：把新增的 4 個對話框加進清單。
- lint：`admin/package.json:10` 的 check-enum-text 本來就涵蓋整個 admin，不用改。

**文件**
- `backend/docs/詳細設計書/32-…md`
  - `:130`：governance_notes 也掛通報
  - `:261`：「通報的後台 API 不變（`ListViolationReports` 等…）」已過時
  - `:300-330`：舊 UI 流程圖，每秒倒數、閃動
  - `:374`：路由改成 `[id]`
  - §2 的 `reports` 欄位表加 `assessed_category`
- runbook
  - `backend/docs/runbooks/seller-violation-management-runbook.md`：K16 改成單人營運；K19 的 `:170` 改成 D9 流程；§2 的「每日 09:00、依 severity 排序」改成新清單。
  - `backend/docs/runbooks/food-safety-incident-runbook.md`：K16，`:10-45`、`:63-290` 整段重寫。v1 列在 3c，K18 的 SLA 數字留到 3d。
- `backend/internal/modules/seller/CARRYOVER.md`：沿用盤點加 `assessed_category`、通報註記、`can_*`。
- `backend/internal/modules/seller/AGENTS.md`：權限表。
- ADR `backend/docs/adr/2026-10-18-seller-governance-consolidated.md`：D9 的修訂段 [推論：要不要寫照慣例由實作者判斷]。
- 計畫 `backend/docs/plans/2026-09-29-a3-buyer-seller.md` 的 10e 完成紀錄。
- 簡報加「實作時的修訂（3c）」。

### 3d 待辦與 SLA（481 不變）

**是否包含佇列取決於 Q1**（A7-D8）。下表把兩種範圍分開列。

**一定在 3d 做的（SLA）**
- `backend/internal/modules/seller/internal/model/`：新的 `review_policy.go`。
  - `ReviewPolicy(kind, category) → window`。
  - 取代 `governance.go:155-156` 的 `FoodSafetyAckSLA`，以及 `:241-253` 的 `AckDueAt`／`AckOverdue`（改成所有類別：食安 4h、其他 72h）。
  - 新增 `FDAReferralDueAt`（resolved_at + 24h）。
  - `MeasureRequest` 的 DecideDueAt：申覆與解除請求都是 7d。
  - `FoodRegistration` 的 DecideDueAt：7d，從可裁定時起算。
  - `KYCApplication` 的 DecideDueAt：7d。
  - 附資料集測試。
- 讀 KYC 核准時間的地方：`internal/foodreg/review.go`、`db/queries/food_registrations.sql`。起算點是 max(submitted, KYC 核准)，欄位來源簡報附錄 B 標了沒有驗證。
- 契約
  - `governance.proto`：`Report.acknowledge_due_time` 的註解改成所有類別；新增 `Report.fda_referral_due_time`。
  - `measure.proto:70-90`：`MeasureRequest.decide_due_time`。
  - `seller.proto:810`：`FoodRegistration.decide_due_time`。
  - `seller.proto:488`：`KYCApplication.decide_due_time`。
  - 全部 OUTPUT_ONLY。
- httpapi：`backend/internal/httpapi/seller/governance.go`、`governance_measures.go`、`foodreg.go`、`admin_kyc.go` 輸出新欄位。
- admin
  - `admin/src/app/admin/measures/measure-inbox.ts`、`page.tsx`：加 danger 徽章「已逾期」，依 `decideDueTime` 判斷（D6′-4）。
  - `measures/[id]/MeasureSections.tsx`：待處理請求的裁定期限。
  - `violation-reports/` 清單與詳情：一般通報開始有確認期限，FDA 轉報期限。
  - `admin/src/app/admin/food-registrations/page.tsx`、`[id]/page.tsx`、`admin/src/lib/food-registration.ts`：期限欄。
  - `admin/src/app/admin/merchants/[id]/page.tsx`（KYC 審核區）、`kyc-application.ts`：審核期限。
- 文件（K18）
  - 設計書 32 `:35`、`:75`、`:105`、`:396`。簡報寫 `:383`，現在已經移到 `:396`，「high 24h / medium 72h / low 7d」在 `:396`。
  - `seller-violation-management-runbook.md:14-17,29-31,115,149-153,164`。
  - `food-safety-incident-runbook.md:196,559-571`。
  - `seller-food-registration-runbook.md:24`：寫明起算點。

**佇列（D7、K17）：選項 B 時在 3d 做；選項 A 時移到 A7-1e 與 A7-4b**
- 契約
  - B：`ListReviewQueue` 放在 `seller.proto` 的 `SellerAdminService`。
  - A：放在 `api/hawkr/admin/reviewqueue/v1alpha1`。
  - 新的 enum `ReviewQueueItemKind`（見 Q3）、`ReviewQueueItem`、Request／Response。
- 查詢：`backend/internal/modules/seller/db/queries/review_queue.sql`（UNION ALL 7 個分支）。可能要為 keyset 加索引。
- 權限：`adminperm.SellerReviewQueueList`、`adminauth` 加一列。
- admin
  - 新頁 `admin/src/app/admin/queue/page.tsx` 與 `queue.ts`。
  - `Sidebar.tsx`：「待辦」放在「賣家 / 商品」分組最上面（D6′-1；選項 A 時見 Q1）。
- 測試：資料集測試「成員資格 ⇔ 判定式」、跨種類的 keyset 不漏不重（簡報切分:350）。

---

## 2. 文案表

來源欄的意思：**已核准** = 簡報原文已經定案（v2 D6′ 或 v1 D6 沒有被取代的那幾點；Will 2026-10-16「全照建議」）。**沿用** = 3a／3b 已經核准、已上線的原文。**新提案** = 這次提出、要 Will 核准。**目錄** = 共用文案目錄 `backend/internal/platform/uitext/catalog/zh-TW.json` 的既有 entry，不是 admin 字串。**伺服器** = Go 的 apperr 句子，admin 照原樣顯示。

### 2.0 enum 名稱的歸屬
- 目錄（已存在，直接用 `@hawkr/ui-text`）
  - `seller.ViolationCategory`（7 個：食安、假認證、誤導資訊、仿冒、未授權販售、詐騙、其他）
  - `seller.ViolationSeverity`（嚴重、高、中、低）
  - `seller.MeasureKind`、`MeasureRequestKind`（申覆、解除請求）
  - `seller.MeasureRequestState`（staff：待裁定、成立、駁回、已結案）
  - `seller.FoodRegistrationState`、`seller.KYCFlowStatus`（staff）
  - [量測] 以 python 讀 zh-TW.json 列出。
- 只有 admin 一端用的 enum，不在目錄（manifest `inCatalog:false, serverOwned:false` [量測]）
  - `seller.ReportStatus`、`admin.FDAChannel`、`admin.CooperationTemplate`，以及新的 `ReviewQueueItemKind`。
  - 照 L′ 設計 §4.2 Q3 留在 admin，寫成 `Record<Exclude<E, UNSPECIFIED>, string>`，不認得的值走 `common.unknown`。
  - 守門只擋目錄內與伺服器擁有的 enum，所以這幾個的文字在下表以 admin 字串送審。
  - 例外：`ReportStatus` 如果之後 website 的「我的通報」也要顯示，就成為兩端，要進目錄（見 Q9）。

### 2.1 3c：側欄與通報清單 `/admin/violation-reports`

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| 1 | Sidebar 項目（移到「賣家 / 商品」） | 商家通報 | 已核准（v1 D6-1） |
| 2 | 頁面標題 | 商家通報 | 新提案（同側欄；舊的是「商家違規 / 調查」） |
| 3 | 頁面說明 | 買家與公眾對店家的通報。在這裡確認收件、調查與結案；食安通報要在 4 小時內確認收件。 | 新提案（3d 要改成不寫數字，見 2.6 #3） |
| 4 | 狀態篩選 | 全部｜待處理｜調查中｜已結案 | 已核准（D6′-2），但「已結案」在契約做不到，見 Q2 |
| 5 | 類別篩選的第一個選項 | 所有類別 | 新提案 |
| 5a | 類別選項 | ViolationCategory 名稱 | 目錄 |
| 6 | 欄位 | 店家｜類別｜嚴重度｜狀態｜確認期限｜建立時間 | 已核准（v1 D6-2） |
| 7 | 嚴重度：還沒確認收件 | 未分級 | 已核准（v1 D6-2） |
| 7a | 嚴重度的值 | ViolationSeverity 名稱 | 目錄 |
| 8 | 狀態 badge | 待處理／調查中／查證屬實／不成立 | 新提案（照 proto 註解 `report.proto:37-44`；舊頁是「已成立／已駁回」） |
| 9 | 確認期限：未逾時 | {HH:mm} 前確認 | 已核准（D6′-2）；跨日要帶日期，見 Q4 |
| 10 | 確認期限：已逾時（danger） | 已逾時 {h} 小時 {m} 分 | 已核准（D6′-2）；和待辦頁的「已逾期」不一致，見 Q4 |
| 11 | 確認期限：已確認收件 | 已確認 | 新提案 |
| 12 | 確認期限：沒有期限（3c 期間的一般通報） | - | 新提案（沿用措施清單的「-」） |
| 13 | 載入 | 載入中... | 沿用（`measures/page.tsx:64`） |
| 14 | 讀取失敗 | `connectErrorText(err, "通報讀取失敗")` ＋「重試」 | 新提案（句型沿用「註記讀取失敗」） |
| 15 | 空狀態：全部 | 尚無通報 | 新提案（句型沿用「尚無措施紀錄」） |
| 16 | 空狀態：有篩選 | 沒有符合條件的通報 | 新提案（句型沿用 food-registration 的「沒有符合的…」） |
| 17 | 分頁 | 上一頁｜第 {n} 頁｜下一頁 | 沿用 |

### 2.2 3c：通報詳情 `/admin/violation-reports/[id]`

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| 20 | 返回鈕 aria-label | 返回列表 | 沿用（`measures/[id]/page.tsx:145`） |
| 21 | 載入 | 載入通報中... | 新提案（句型沿用「載入措施中...」） |
| 22 | 讀取失敗 | `connectErrorText(err, "通報讀取失敗")`、「重試」 | 新提案 |
| 23 | 標題 | 通報詳情 | 新提案（句型沿用「措施詳情」） |
| 24 | 副標 | 查看店家・通報 {id}・建立於 {時間} | 新提案（沿用措施詳情的「查看店家」「措施 {id}」） |
| 25 | 區塊標題 | 通報內容 | 新提案（舊頁同名） |
| 26 | 欄位：店家 | 店家 | 新提案 |
| 26a | 欄位：類別 | 通報人選的類別 | 新提案 |
| 26b | 欄位：平台判定 | 平台判定類別 | 已核准（D6′-3 的用詞） |
| 26c | 欄位：其他 | 嚴重度｜狀態｜商品 ID｜訂單編號｜說明 | 新提案 |
| 27 | 沒填的欄位 | -（商品 ID、訂單編號沒有提供） | 新提案 |
| 28 | 區塊標題 | 通報人 | 新提案 |
| 29 | 登入的通報人 | 買家 ID {id} | 新提案 |
| 30 | 匿名通報人 | 匿名｜電話｜email｜姓名 | 新提案 |
| 31 | 聯絡方式已清除 | 聯絡方式已於 {時間} 依保留期限清除 | 新提案 |
| 32 | 區塊標題 | 佐證 | 新提案（沿用措施詳情「佐證 {n}」：`measure-actions.ts:530`） |
| 32a | 佐證：空 | 沒有佐證 | 新提案 |
| 33 | 區塊標題 | 處理紀錄 | 新提案（用「紀錄」，不用舊頁的「既有處理摘要」） |
| 34 | 處理紀錄的列 | 確認收件：{admin ID}・{時間}｜開始調查：{時間}｜結案：{admin ID}・{時間} | 新提案（「{admin ID}・{時間}」沿用註記的格式，v1 D6-9） |
| 35 | 欄位 | 給通報人的結果說明 | 已核准（v1 D6-3 的欄名去掉括號） |
| 35a | 結果說明：空 | 沒有說明 | 新提案 |
| 36 | 按鈕 | 確認收件｜開始調查｜結案｜建立措施 | 已核准（v1 D6-3） |
| 37 | 按鈕（食安） | 下載轉報資料（.md）｜記錄已轉報主管機關 | 已核准（v1 D6-3；「記錄」在這裡是動詞，術語表只限名詞，見 Q10） |
| 38 | 按鈕 | 要求賣家配合 | 已核准（v1 D6-3） |
| 39 | 已轉報 | 已於 {時間} 記錄轉報（{管道}，案號 {no}） | 已核准（v1 D6-3） |
| 40 | 已要求配合 | 已於 {時間} 要求 | 已核准（v1 D6-3），每一種範本一列：「{範本}：已於 {時間} 要求」 |
| 41 | 區塊標題 | 導出的措施 | 已核准（D6′-7） |
| 42 | 導出措施的列 | 措施 {id}（連到 `/admin/measures/{id}`） | 新提案；種類與狀態要不要一起顯示見 Q7 |
| 43 | 導出措施：空 | 還沒有導出的措施 | 新提案 |
| 44 | 註記區（共用元件） | 內部註記／只有後台看得到。送出後不能修改或刪除，要更正請再寫一則。不要寫通報人的聯絡方式。／寫下調查經過或判斷依據（最多 2000 字）／新增註記／還沒有註記／{admin ID}・{時間}／註記讀取失敗／新增註記失敗，註記沒有送出；請確認網路狀態後再試一次。 | 已核准（v1 D6-9），沿用（`MeasureNotesSection.tsx`） |
| 45 | 建立措施對話框 | 照 3b 修訂 2 | 沿用（Will 2026-10-18） |
| 46 | 不能操作時 | 按鈕依 `can_*` 隱藏，不顯示原因（K4′ 沒有 reasons） | 已核准（K4′）；要不要加原因見 Q6 |

**對話框**

| # | 對話框 | zh-TW | 來源 |
|---|---|---|---|
| 50 | 確認收件 | 標題「確認收件」；欄位「平台判定類別」「嚴重度」；提示「通報人選的是 {類別}」 | 已核准（D6′-3） |
| 51 | 確認收件 | 說明：「確認收件後開始分案；平台判定類別決定是否走食安的轉報流程，不會改掉通報人選的類別。」 | 新提案 |
| 52 | 確認收件 | 驗證：「請選擇平台判定類別。」「請選擇嚴重度。」 | 新提案（句型沿用 3b） |
| 53 | 確認收件 | 送出「確認收件」／送出中「送出中…」／「取消」 | 新提案（沿用 3a 的「送出中…」「取消」） |
| 54 | 確認收件 | 失敗：「確認收件失敗，通報沒有變動；請確認網路狀態後再試一次。」 | 新提案（照 3a 的句型） |
| 55 | 開始調查 | 不開對話框，按下就送出。失敗：「開始調查失敗，通報沒有變動；請確認網路狀態後再試一次。」 | 新提案 |
| 56 | 結案 | 標題「結案」；選項「查證屬實」「不成立」；欄位「給通報人的結果說明（選填，通報人看得到）」 | 已核准（v1 D6-3） |
| 57 | 結案 | 選「不成立」時的提示：「不成立的通報不能當措施的來源，也不能再要求賣家配合。」 | 新提案（照伺服器規則 `report.go:319`、3b 的來源規則） |
| 58 | 結案 | 驗證「請選擇結案結果。」「結果說明最多 2000 字。」 | 新提案 |
| 59 | 結案 | 送出「確認結案」；失敗「結案失敗，通報沒有變動；請確認網路狀態後再試一次。」 | 新提案 |
| 60 | 記錄轉報 | 標題「記錄已轉報主管機關」；欄位「管道」「主管機關的案號」；說明「轉報本身在系統外完成；這裡只記一次，記過之後不能修改。」 | 新提案（照 proto `:40-41`） |
| 61 | 記錄轉報 | 管道選項：1919 食安專線｜線上表單｜傳真衛生局｜親自送件｜其他 | 新提案（照 proto `:141-149`；舊頁有錯字「食藝專線」，又寫「當面」） |
| 62 | 記錄轉報 | 驗證「請選擇管道。」「請填寫案號。」「案號最多 100 字。」 | 新提案（舊頁要求「無則填 n/a」，新契約 REQUIRED，見 Q11） |
| 63 | 記錄轉報 | `already_recorded` 時：「這筆通報先前已記錄轉報，以下是原本的紀錄，這次輸入的內容沒有寫入。」 | 新提案 |
| 64 | 記錄轉報 | 失敗「記錄轉報失敗，通報沒有變動；請確認網路狀態後再試一次。」 | 新提案 |
| 65 | 要求配合 | 標題「要求賣家配合」；欄位「範本」 | 新提案 |
| 66 | 要求配合 | 範本：協助調查（提供批次、進貨與製程資料）｜暫時下架並自我檢查 | 新提案（照 proto `:154-158`；舊頁「下架自查」「7 日內提供」沒有契約依據） |
| 67 | 要求配合 | 說明「送出後平台會通知賣家。同一種範本只會通知一次。調查初期先不要通知，以免證據被湮滅。」 | 新提案（後半沿用舊頁的理由） |
| 68 | 要求配合 | `already_requested` 時：「先前已要求過這一種配合，這次沒有再通知賣家。」 | 新提案 |
| 69 | 要求配合 | 送出「通知賣家」；失敗「要求配合失敗，賣家沒有收到通知；請確認網路狀態後再試一次。」 | 新提案 |
| 70 | 下載轉報資料 | 失敗「轉報資料讀取失敗，請再試一次。」 | 新提案 |
| 71 | 成功提示 | 不另外顯示（3a 的慣例：對話框關閉、畫面更新）；只有 #63、#68 這兩種「沒有寫入」的結果顯示橫幅 | 新提案 |

**FDA 轉報 `.md`**（K2，client 組；新提案，取代舊 seller-service 的範本 `services/seller-service/internal/usecase/food_safety_report.go:224-284`）
- `# 食安通報轉報資料（通報 {id}）`
- `## 1. 時間`：通報時間｜確認收件時間｜結案時間（都用台北時間）
- `## 2. 被通報的店家`：店名｜店號｜聯絡人｜聯絡電話｜地址｜食品業者登錄字號（沒有時寫「無」）
- `## 3. 商品與訂單`：商品 ID｜訂單編號｜（沒有提供時寫「通報人沒有提供」）
- `## 4. 通報人聯絡方式（內部使用，不要轉寄）`：電話｜email｜姓名｜（已清除時寫「已依保留期限清除」）
- `## 5. 通報內容`：全文
- `## 6. 佐證`：佐證 {n} 的連結；或「無」
- 結尾：`本檔由後台於 {時間} 產生，僅供轉報參考，請以實際送出的內容為準。`

**伺服器句子**（`backend/internal/modules/seller/internal/governance/report.go`，admin 照原樣顯示）

| # | 位置 | 現在 | 提案 | 來源 |
|---|---|---|---|---|
| 80 | `:284` | 只有食安通報可以轉報主管機關 | 平台判定類別是食安的通報才能轉報主管機關 | 新提案（D9） |
| 81 | `:383` | 只有食安通報可以匯出轉報資料 | 平台判定類別是食安的通報才能匯出轉報資料 | 新提案（D9） |
| 82 | Acknowledge 新的驗證 | — | 請選擇平台判定類別 | 新提案 |
| 83 | `:180,208,243,250,319`、`:23` | 照舊 | 不變 | 沿用 |

### 2.2b 3c：發票申訴頁與建立措施對話框（簡報 3b 修訂第 6 點）

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| 72 | `complaints/[complaintId]/page.tsx:534` 建議卡的欄標題 | 建議理由 | 已核准（3b 修訂 6，Will 2026-10-18；取代 3b 修訂 3 的「建議理由（可直接複製到措施的理由）」） |
| 73 | 對話框預填的種類 | 停權（目錄 `seller.MeasureKind`） | 目錄 |
| 74 | 對話框預填的給賣家的理由 | 建議的 `suggestedReason` 原文（invoice-service 產生，不是 admin 字串） | 伺服器 |
| 75 | 沒有建議天數時的結束時間 | 不預填；送出時沿用「請填寫結束時間。」 | 沿用（3b 修訂 2） |
| 76 | （選用）對話框頂端的預填提示 | 已依停權建議預先填入種類、結束時間與給賣家的理由，送出前請確認並選擇類別。 | 新提案；可以不加，核准的做法沒有要求 |
| 77 | 來源通報清單的說明 | 選填；最多 20 筆，只列這家店還沒判定不成立的通報。 | 沿用（3b 修訂 2）。改看 `can_cite_as_source` 之後語意不變：K4 的規則仍是「同一家店、不是不成立」 |

### 2.3 3c：舊頁要被取代的過時字串（全部刪除）

`complaints/[complaintId]/page.tsx:534`：「建議理由（可直接複製到措施的理由）」改成「建議理由」（#72）。

`violation-reports/page.tsx`：
- `:160` 商家違規 / 調查
- `:162` 商家違規 / 食安通報合併處理（B-169）。每 30 秒自動更新。
- `:165-166` 🚨 紅色警示為 critical 食安通報，需 4h 內 ack；3h 未 ack 將閃爍提示。
- `:181` 🚨 unacked critical: {n} — 請立即處理
- `:89-92`、`:206-210` 危急（食安）／高／中／低、所有嚴重度
- `:96-99`、`:195-199` 待處理／調查中／已成立／已駁回、所有狀態
- `:103-109` 類別表：改用目錄
- `:217-219` 所有來源／登入端檢舉／食安通報
- `:222` 手動更新
- `:230,233` 欄位「來源」「SLA」
- `:243` 載入中…
- `:249` 暫無資料
- `:330` 詳情
- `:362` 🚨 食安
- `:375` 檢舉
- `:405` 已逾時

`violation-reports/[source]/[id]/page.tsx`：
- `:134` confirm「確認 ack 此 critical 食安通報？此動作不可撤銷。」
- `:138,148,169,203,218` alert 失敗句
- `:143` 確認推進 status 至 …？
- `:196` FDA 受理編號必填（如無請填 n/a）
- `:199` 確認標記已通報 FDA？此動作不可撤銷。
- `:209-216` 協助調查／下架自查、通知模板 confirm、「通知已發送」
- `:223,227` 載入中…／找不到此 report
- `:236` ← 返回列表
- `:240,243` 🚨 食安通報／違規檢舉 #{id}、建立於
- `:248-289` 基本資訊、當前狀態、檢舉者 account_id、危急（食安通道一律 critical）、通報者 phone／email／姓名、涉案店家 Store ID（未指定）、涉案 product_id、涉案訂單號、涉案 post_id
- `:294-315` 通報內容、證據 asset_ids、（無）
- `:324-333` ✅ 已 ack 於…／尚未 ack — 請於 4h 內處理
- `:345-375` 推進處理狀態、下一狀態、investigating（調查中）等選項、處理摘要（可選）、placeholder、推進至 …
- `:382-427` FDA 通報、✅ 已通報 FDA 於…、管道／受理編號、說明段、標記已通報 FDA、「1919 食藝專線」（錯字）、當面、受理編號（無則填 n/a）、placeholder
- `:445-475` 通知賣家（模板化）、說明「停權通知由建立 violation 自動發」、模板選項「要求 7 日內提供批次…」、通知賣家
- `:482-499` 📝 Postmortem 提示（7 天內提交 postmortem、8 sections、開 git PR），見 Q12
- `:506` 既有處理摘要
- `:585-602` 狀態、類別的第二份表

### 2.4 3d：待辦頁 `/admin/queue`（選項 B 在 3d 做；選項 A 時文案照樣先定，給 A7-4b 用）

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| 100 | Sidebar 項目 | 待辦 | 已核准（D6′-1） |
| 101 | 頁面標題 | 待辦 | 新提案（同側欄） |
| 102 | 頁面說明 | 所有等後台處理的事項，有期限的在前、由急到緩。每 30 秒自動更新。 | 新提案（後半沿用舊頁的句型） |
| 103 | 欄位 | 種類｜店家｜開始時間｜期限 | 已核准（D6′-5） |
| 104 | 種類 | 確認收件｜調查通報｜轉報主管機關｜複審停權｜審核食品業者登錄 | 已核准（D6′-5） |
| 105 | 種類（術語表修正） | 簡報原文「裁定申訴」「裁定補正」→ **裁定申覆**、**裁定解除請求** | 新提案（術語表退場用語:48-49；同 3a 對話框的標題 `measure-actions.ts:327`） |
| 106 | 種類（術語表修正） | 簡報原文「審核身分認證」→ **審核商家認證** | 新提案（術語表:38；見 Q5） |
| 107 | 篩選 | 所有種類＋上面 8 個種類 | 新提案（「篩選只有種類」已核准） |
| 108 | 期限：未逾期 | {日期 HH:mm} 前 | 已核准（D6′-5） |
| 109 | 期限：已逾期（danger） | 已逾期 {時間} | 已核准（D6′-5）；「{時間}」是時長還是時刻沒寫，見 Q4 |
| 110 | 期限：沒有期限 | — | 已核准（D6′-5） |
| 111 | 開始時間 | {日期 HH:mm}（`formatDateTimeShort`） | 新提案 |
| 112 | 空狀態 | 目前沒有待辦 | 已核准（D6′-5） |
| 112a | 空狀態：有篩選 | 沒有這個種類的待辦 | 新提案 |
| 113 | 載入／失敗 | 載入中...／`connectErrorText(err, "待辦讀取失敗")`＋「重試」 | 新提案 |
| 114 | 列的連結 | 通報 → `/admin/violation-reports/{id}`；措施 → `/admin/measures/{id}`；食品業者登錄 → `/admin/food-registrations/{id}`；KYC → `/admin/merchants/{store_id}` | 已核准（D6′-5「進各自的詳情頁」） |
| 115 | 店家欄：沒有店名 | 顯示 store_id（照措施清單 `m.storeName \|\| m.id`） | 沿用 |

### 2.5 3d：既有頁面的期限顯示

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| 120 | 措施清單的徽章（danger） | 已逾期 | 已核准（D6′-4） |
| 121 | 措施清單的「已等候」 | 已等候 {N} 天／今天提出 | 沿用（`measure-inbox.ts:122`） |
| 122 | 措施詳情：待處理的請求 | 裁定期限：{日期 HH:mm} 前；逾期時加 danger「已逾期」 | 新提案 |
| 123 | 通報清單：一般通報開始有期限 | 照 #9、#10 | 已核准 |
| 124 | 通報詳情 | 確認期限：{日期 HH:mm} 前（已確認收件時不顯示） | 新提案 |
| 125 | 通報詳情（食安、查證屬實、還沒轉報） | 轉報期限：{日期 HH:mm} 前；逾期 danger「已逾期」 | 新提案 |
| 126 | 食品業者登錄清單 | 新的欄「期限」，值「{日期 HH:mm} 前」或 danger「已逾期」；「等實名認證」分頁顯示「—」 | 新提案 |
| 127 | 食品業者登錄詳情 | 欄位「裁定期限」 | 新提案 |
| 128 | 商家詳情的 KYC 審核區 | 「審核期限：{日期 HH:mm} 前」；逾期 danger「已逾期」 | 新提案 |

### 2.6 3d：要跟著改的既有字串

| # | 位置 | 現在 | 提案 |
|---|---|---|---|
| 130 | `food-registrations/page.tsx:102`、`lib/food-registration.ts:21,71,74` | 實名認證 | 依 Q5 決定（商家認證或維持） |
| 131 | `admin/src/app/admin/merchants/store-row.ts:18,22`、`use-store-detail.ts:45,48` | 身分認證 | 同上 |
| 132 | `admin/src/lib/food-registration.ts:141` | KYC SUBMITTED「待審核」 | 不變 |
| 133 | #3 的通報頁說明「4 小時內」 | — | 3d 改成「依期限處理；逾時的會標示」，不寫數字 |

**數字**（只計 2.1、2.2、2.2b、2.4、2.5 的列，對話框的子項不另外拆）
- 已核准 20 列（含 2.2b 的「建議理由」）
- 沿用 11 列
- 新提案約 61 列：清單 11、詳情 18、對話框 20、2.2b 選用提示 1、伺服器 3、`.md` 1 組、待辦 7、期限顯示 7
- 目錄 3 組（類別、嚴重度、措施種類），沒有新增目錄 entry
- 過時要刪的字串：2 檔約 60 處

---

## 3. 待決題與衝突

| # | 題目 | 衝突 | 建議 | 約束 |
|---|---|---|---|---|
| Q1 | **佇列在 3d 做，還是交給 A7-1e**（A7-D8 待 Will） | 簡報 D7：seller 內的 UNION，放在 `SellerAdminService`，3d 做。A7 計畫 A7-D8 選項 A（建議）：佇列只做一次，在 A7-1e 直接做成平台層的 `reviewqueue` 模組，proto 放 `admin/reviewqueue/v1alpha1`；3d 只留 ReviewPolicy、`*_due_time` 與 K18 | **選 A**。D7 自己寫的觸發條件（「social 搬進單體 = 實體化，佇列移到平台層」，簡報:178-179）正是 A7 現在在做的事；先做 UNION 再改成投影，同一件事要做兩次。3d 改成：ReviewPolicy、所有 `*_due_time`、清單的逾期徽章、K18；佇列的 kind 與文案在 3d 先定好，交給 A7-1e 與 A7-4b。「待辦」放在側欄的「總覽」分組，因為它會包含內容審核 | 「模組不能 UNION 對方的表」是 **REAL**（單體規則 2）。「佇列頁早一點有」是 **SELF-IMPOSED**：沒有上線、只有一位營運人員。代價是 seller 要補發開啟事件，這是 A 的成本，不是約束 |
| Q2 | 通報狀態篩選的「已結案」 | D6′-2 的篩選是「全部｜待處理｜調查中｜已結案」，但 `ListReportsRequest.status` 是單一值（`governance.proto:303`），表示不出 VERIFIED ∪ DISMISSED | 篩選改成 5 個選項「全部｜待處理｜調查中｜查證屬實｜不成立」，契約不動。另一個做法是加 `repeated statuses`，代價是一支新的查詢 | 單值篩選是 **SELF-IMPOSED**（8g 的形狀） |
| Q3 | 佇列 kind 的數量 | D7 是 7 個 kind（`MEASURE_REQUEST` 一個）；D6′-5 有 8 個標籤（申覆、解除請求分開） | enum 用 8 個值，`MEASURE_APPEAL`、`MEASURE_REMEDIATION` 分開：篩選、標籤、ReviewPolicy 一對一，client 不用看第二個欄位組標籤 | **SELF-IMPOSED** |
| Q4 | 期限的格式 | 通報清單「{HH:mm} 前確認」只有時刻，在 3d 有 72 小時期限之後會跨日；通報用「已逾時 {h} 小時 {m} 分」，待辦用「已逾期 {時間}」，措施用「已逾期」 | 統一：未逾期「{M/D HH:mm} 前」（通報清單的列加「確認」兩個字）；逾期一律 danger「已逾期 {時長}」，時長滿 1 天時是「{d} 天 {h} 小時」，未滿 1 天是「{h} 小時 {m} 分」。措施清單的徽章只寫「已逾期」（D6′-4 原文），旁邊另有「已等候 N 天」。發票頁已有先例「已逾期 2 天」（`Invoice.css:71`） | 4h 時期的格式是 **SELF-IMPOSED** |
| Q5 | KYC 的用語 | 術語表:38「商家認證」；賣家面目錄用「商家認證」；admin 混用「實名認證」（food-registrations、目錄的 `kyc_pending_note`、staff 說明共 47 處）與「身分認證」（store-row）；簡報 D6′-5 寫「審核身分認證」 | 待辦 kind 用「審核商家認證」；admin 其他地方統一成術語表，另開一刀處理，或在 3d 一併處理（範圍要 Will 定）。目錄 `seller.FoodRegistration.kyc_pending_note` 是賣家面的句子，用「實名認證」，和術語表不一致，要回報給 L′ 的負責人 | 術語表是 SSOT（**REAL**，本專案的規則） |
| Q6 | 通報的 `can_*` 要不要附原因 | K4′ 只有布林值；3a 的措施有 `*_unavailabilities`（D5） | 照 K4′，不附。通報的不能操作都來自狀態，而狀態就在畫面上（同 D5 對 end、schedule 的判斷）。觸發條件比照 D5 | **SELF-IMPOSED** |
| Q7 | 「導出的措施」顯示什麼 | D10 只輸出 `resulting_measure_ids`；顯示種類與狀態的話，client 要逐筆 GetMeasure | 只顯示「措施 {id}」與連結（N+1 不划算）；要種類的話，改輸出 `repeated MeasureSummary`，要 Will 定 | **SELF-IMPOSED** |
| Q8 | 註記的訊息型別 | `MeasureNote`（`governance.proto:515`）名稱綁死措施 | 改名成 `GovernanceNote`（還沒上線，可以直接改名），兩組 RPC 共用；或另開同形的 `ReportNote` | 相容性：**SELF-IMPOSED**（上線前） |
| Q9 | `ReportStatus` 進不進目錄 | L′ 設計:209 寫「ReportStatus（CodeMeta 凍結）」不收。但 `seller.v1alpha1.ReportStatus` 是 proto enum，不是 CodeMeta；目前只有 admin 用 | 3c 先在 admin 寫單端表（`Record<Exclude…>`）；website 做「我的通報」時移進目錄。順手更正 L′ 設計:209 的理由 | 單端表是 **REAL**（L′ 守門規則） |
| Q10 | 「記錄已轉報主管機關」的「記錄」 | 術語表:39 只限名詞要寫「紀錄」 | 動詞維持「記錄」，已核准，不改 | 無 |
| Q11 | 案號沒有時 | 舊頁「無則填 n/a」；新契約 `reference_no` REQUIRED、1–100 字（CHECK） | 照契約必填；說明加「沒有案號時填管道給的受理編號或日期」。要不要改成選填，要 Will 定 | CHECK 是 **SELF-IMPOSED**（我們擁有的程式） |
| Q12 | 食安查證屬實後的 postmortem 提示 | 舊頁有，新設計（D6）沒有；runbook 要求 7 天內寫 | 不放在 UI，留在 K16 改寫後的 runbook。單人營運下 git PR 的 postmortem 是不是還適用，在 K16 一起決定 | **SELF-IMPOSED** |
| Q13 | FDA 匯出要先確認收件 | K4′：`can_export_fda` 看 `assessed_category`，確認收件之前是 NULL，所以食安通報要先確認收件才能匯出 | 接受。4 小時內本來就要確認收件；運作上是先確認、再匯出 | 無 |
| Q14 | 和 A7-D7「內容下架」的衝突 | (a) A7-D7 第 7 點寫「`violations.moderation_report_id`，用 CHECK 限制它和 `source_report_id` 最多填一個」，但 3b 已經刪掉 `violations.source_report_id`、改成 `finding_reports`（migration `20261018000223`），跨表沒辦法用 CHECK；A7-1c:262 只寫了「接在 finding_reports 的形狀上」。(b) A7-D7 第 2 點要在 `ViolationCategory` 加社群類別，但這個 enum 也是通報類別與 `assessed_category` 的值域（`reports_category_check`） | (a) 通知 A7 改寫第 7 點，例如另開 `finding_moderation_reports` 關聯表，或在 `violations` 加欄位，在應用層檢查互斥。(b) 3c 的 `assessed_category` CHECK 只列通報的 7 類；A7 加類別時不放進 reports 的 CHECK，CreateReport 也不接受，確認收件對話框的選項要排除它 | (a) 跨表沒有 CHECK 是 **REAL**（Postgres）。(b) 值域共用是 **SELF-IMPOSED** |
| Q15 | 簡報內部的行號已經過時 | 簡報寫設計書 32:383，現在是 :396；`openapi_coverage_test.go:27` 現在是 :28；480 應該是 481 | 3c、3d 的修訂段照實際值更正 | 無 |
| Q16 | 側欄的排序 | D6′-1：「待辦」放在「賣家 / 商品」最上面，「商家通報」「措施與申訴」放在「食品業者登錄審核」上面。現在這組第一項是「賣家管理 / 審核」，「措施與申訴」已經改名成「措施與申覆」 | 「賣家 / 商品」：待辦（選項 B 時）、賣家管理 / 審核、商家通報、措施與申覆、食品業者登錄審核……；選項 A 時，待辦移到「總覽」 | **SELF-IMPOSED** |

### 沒有驗證的部分
- 食品業者登錄的起算點（KYC 核准時間在哪個欄位）沒有逐欄對過，簡報附錄 B 也是同樣的狀態。
- `GetFDAExport` 的 `Store` 欄位是否足夠組 `.md`：只讀了 message 的欄位（店名、電話、地址、`approved_food_registration`），沒有對 handler。
- 新提案的文字沒有和 `website/docs/legal/seller-terms.md` 逐字對過；只有用詞照術語表。
