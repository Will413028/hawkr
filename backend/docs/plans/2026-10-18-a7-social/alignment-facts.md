# A7-0 單體側對齊事實（唯讀盤點，快照 main 172260d31）

路徑縮寫：B=backend，SL=B/internal/modules/seller，API=api/hawkr。【量】＝打開檔案或跑指令看過；【推】＝讀程式推論。

## 1. seller 措施模型（main）

- 【量】表 `seller.measures`：SL/db/schema.sql:380；`kind TEXT`（:384）；`measures_kind_check CHECK (kind IN ('WARNING','SUSPENSION','TERMINATION'))`（:406）。另有直接引用 kind 的 CHECK：termination（:412）、contract（:413）、starts（:417-419）、basis（:430-432）、period（:436-437）。
  - 相關表：`measure_findings`（:471）、`measure_revisions`（:482）、`measure_requests`（:529，kind APPEAL／REMEDIATION，:545）、`violations`（認定，:324，`source_report_id` REFERENCES seller.reports，category CHECK :341）、`governance_notes`。
  - 【量】`measures` 沒有「內容對象」欄位（沒有 post／comment 參照）；`store_id`、`account_id` 皆 NOT NULL（:385-386）。內容下架要記下架哪篇內容，需新欄位或新表。【推】
  - 部分索引與 query 直接寫死 kind：`measures_enforced_suspension` 等（schema.sql:456,461,462）；`db/queries/measures.sql:292,306,310,334,338,358,362,386,390,432`、`suspensions.sql:15,30,35,51,57`、`purge.sql:14-24`（rg 指令：`rg -n "'SUSPENSION'|'WARNING'|'TERMINATION'" SL/db/queries/*.sql`）。
- 【量】Go 型別：`model.MeasureKind`，SL/internal/model/measure.go:9-23（`MeasureKindValues()` 註解「與 DB 的 measures_kind_check 一致」）；`Restrictive()`（:26）= 停權或終止，決定「生效中」；`StateOf`（model.go 一帶）以 kind 推導狀態（WARNING→ISSUED，其他 ACTIVE/ENDED…）。對外再匯出：SL/governance.go:38-39,116-118,213-214。
- 【量】認定→措施：admin 呼叫 `CreateMeasure`（SL/governance.go:443）→ `measure.Service.Create`（internal/measure/create.go:205），輸入 `CreateMeasureInput{AdminID, StoreID, Kind, StatementToSeller, EndsAt, ReviewBy, Note, Violation|Risk|Termination}`（internal/model/measure.go:941）。VIOLATION 依據 = `ViolationBasis{Finding *NewFinding | CitedFindingIDs}`，`NewFinding{Category, Severity, SourceReportID *uuid}`（measure.go:924-934）；create.go:472-492 `LockReport` 驗證來源通報屬同一店家、不是 DISMISSED，並寫 violations.source_report_id。Admin RPC：`CreateMeasure` oneof warning／suspension／termination（API/admin/seller/v1alpha1/governance.proto:65；handler B/internal/httpapi/seller/governance_measures.go:244-270，每個 oneof 臂固定一個 kind）。
  - 【量】`created_by_admin_id > 0` 為 DB CHECK（schema.sql:449）、`measure.Create` 的輸入要 AdminID：目前措施一定由 admin 建立，沒有「系統／事件觸發建立措施」的路徑。A7 若要由 social 內容檢舉直接產生措施，這是要先決定的缺口（誰是 actor：admin 認定後呼叫，或新增系統 actor）。【推】
  - 【量】通報來源：`seller.reports`（見 Q2）→ admin Acknowledge/Resolve → admin 建立措施時帶 `SourceReportID`。
- 【量】seller 對措施發的事件（SL/events.go 別名；型別 internal/model/measure.go:1119-1190；發布點 `events.Publish`）：
  | 事件 | 欄位 | 發布點 |
  |---|---|---|
  | MeasureCreated | OccurredAt, EventID, MeasureID, StoreID, AccountID, Kind, Basis, RiskGround*, TerminationGround*, StartsAt, EndsAt* | internal/measure/create.go:338 |
  | MeasureRevised | …, Revision, EndsAt* | internal/measure/update.go:129 |
  | MeasureEnded | MeasureID, StoreID, AccountID | update.go:230 |
  | MeasureRevoked | …, Reason | internal/measure/revoke.go:335 |
  | MeasureRequestSubmitted／Decided／Closed | MeasureID, RequestID, Kind, (Outcome／Cause) | appeal.go:139／appeal.go:223,remediation.go:113／revoke.go:324 |
  | ReportAcknowledged／ReportResolved／SellerCooperationRequested | ReportID, StoreID, (ReporterBuyerID*, Outcome) | governance 內 |
  | StoreSuspended／Unsuspended／ExposureChanged… | StoreID, AccountID, PeriodID | model/model.go:303-330 |
  - 【量】MeasureCreated 沒有「內容對象」欄位；social 訂閱後無法知道要隱藏哪篇，必須在事件或查詢上補對象。【推】
  - 【量】組裝點只訂閱 StoreSuspended 等給 legacy relay（B/cmd/hawkr/seller_events.go:29-40）；治理事件（含 Measure*）目前沒有任何訂閱者（sellerrelay 套件說明：「治理事件（A8 的 notification 訂閱）」，B/internal/legacy/sellerrelay/relay.go:12-14）。
- 【量】申覆／解除請求：`SubmitAppeal`（governance.go:484）、`DecideAppeal`（:489）、`RequestRemediation`（:495，只限 basis=RISK，`measure_requests_remediation_basis_check` schema.sql:548）、`DecideRemediation`（:500）。每措施至多一次申覆（索引 schema.sql:559），同時至多一筆待處理（:560）。接受申覆 → 撤銷措施（同 tx 發 MeasureRevoked）。賣家端 `SellerAppService.SubmitAppeal`（API/seller/v1alpha1/seller.proto:138 附近）。【推】內容下架要「補正後解除」須放寬該 CHECK（目前補正只掛 RISK）。
- 【量】術語表 `B/docs/開發規範/使用者面用語術語表.md:19`（措施：「種類有警告、停權、終止合作」）、:20-23（措施與申覆、警告、停權、終止合作）、:25-28（事由、認定、申覆、解除請求）。「內容下架」尚無條目（brief C9）。
- 新增 kind「內容下架」會動到的地方（rg 與讀檔得出，【推】為判斷）：
  1. DB：`measures_kind_check`（schema.sql:406）＋新 migration（目前最新 `20261016120000_seller_governance_notes.sql`）；kind 相關 CHECK（:412,413,417-419,430-437）要逐條評估；內容對象欄位／表；索引與 queries（上列檔）；`sqlc` 重新生成 internal/store。
  2. Go：`model.MeasureKind` 常數與 `MeasureKindValues`（measure.go:9-23）、`Restrictive()`、`StateOf`、`measure/create.go:39,59,74,86-88,348-362` 的 kind 分支、`revoke.go:117,187,200,211`（TERMINATION 特例，內容下架撤銷需新分支）、`closing/*`、`stint/stint.go:108`、`devseed`、SL/governance.go:38-39,116-118,213-214、`B/internal/legacy/sellerreason/reason.go:37-70`（`m.Kind != seller.MeasureSuspension` 之類）。
  3. 事件：MeasureCreated 等需帶內容對象（或另立事件）；sellerrelay 不轉送治理事件。
  4. Proto：`API/seller/v1alpha1/seller.proto:1040-1048` enum `MeasureKind`（+ Measure.kind :1225）、`API/admin/seller/v1alpha1/measure.proto:97`、admin `CreateMeasureRequest` oneof（governance.proto，新增臂）；以及 `MeasureState`／可用性原因 enum 對新 kind 的語意。
  5. uitext 目錄：`B/internal/platform/uitext/catalog/zh-TW.json:626-634`（`seller.MeasureKind.MEASURE_KIND_{SUSPENSION,TERMINATION,WARNING}` 名詞）、:707（`MEASURE_KIND_TERMINATION {合作已終止} other {停權中}` 的 select，other 會把新 kind 誤顯示為「停權中」，需改），en.json 缺字退回 zh-TW；生成物 `packages/hawkr-ui-text-ts`（keys.gen.ts、zh-tw.gen.ts、messages/*.gen.json、enum-manifest.gen.json、tests/golden*）與 `packages/hawkr_ui_text`（ui_text.g.dart、enum_manifest.g.json、golden*）。
  6. Client：admin `admin/src/app/admin/measures/[id]/MeasureSections.tsx`、`admin/src/app/admin/merchants/[id]/measure.ts`、`ActiveMeasuresCard.tsx`；website `website/features/seller/measures/measureDisplay.ts`、`website/features/seller-center/components/SellerSuspendedNotice.tsx`；seller-app `lib/features/seller/data/measure_display.dart`、`presentation/suspension_notice_screen.dart`；生成：`packages/hawkr-api-ts/src/gen`、`packages/hawkr_api/lib/gen`、`backend/internal/httpapi/gen`。
  7. 文件：術語表 :19-23（10e 擁有）、seller 的 AGENTS.md／CARRYOVER.md（措施說明）、ADR 2026-10-18-seller-governance-consolidated 修訂。
  指令：`rg -ln 'MeasureKind|MEASURE_KIND' website admin seller-app/lib --glob '!node_modules' --glob '!**/gen/**'`（6 個非測試檔）。

## 2. seller.reports（通報）

- 【量】schema：SL/db/schema.sql:211-（註解 :203-210）。欄位：id UUID、store_id（FK stores）、product_id、order_no、category、description（1–2000 字）、reporter_buyer_id 或 reporter_phone/email/name（二選一，`reports_reporter_check`）、contact_erased_at、status（PENDING→INVESTIGATING→VERIFIED｜DISMISSED）、severity（admin 確認收件時決定）、FDA 轉報欄位。
- 【量】類別 `reports_category_check`（:237）：FOOD_SAFETY, FAKE_CERTIFICATION, MISLEADING_INFO, COUNTERFEIT, UNAUTHORIZED_SALE, FRAUD, OTHER（7 種；認定的 category 同組，:341）。沒有「騷擾／垃圾訊息／不當內容」之類社群類別；social 的檢舉原因要對映到這 7 類或擴充。【推】
- 【量】建立路徑：Connect `SellerReportAppService.CreateReport`（登入買家，`B/internal/httpapi/seller/reports.go:128-152`）→ `Module.SubmitReport`（SL/governance.go:378）→ `governance.Service.Submit`（internal/governance/report.go:89）。匿名食安通報 `SubmitFoodSafetyReport`（reports.go:180-190）。
- 【量】非 seller 模組可否直接呼叫：`SubmitReport` 是 seller 根套件的公開方法，輸入 `SubmitReportInput{StoreID, ProductID, OrderNo, Category, Description, ReporterBuyerID, UploaderAccountID, Phone/Email/Name, EvidenceTokens}`（internal/model/governance.go:259-268）。限制：(a) 必須在 transaction 外呼叫（證據檔先轉正，governance.go:375 註解）；(b) 店家必須是「曾公開過的店」（`StoreCards` 規則，governance.go:379-387，否則 NotFound）；(c) 去重：登入通報人同店家、同類別、同一個台北日回 Duplicate（reports.go:144-146；report.go:86-88,152）；(d) 通報人只能是 buyer（reporter_buyer_id）或匿名聯絡方式，沒有「店家檢舉買家」的通報人身分。(e) 依賴表：moderation 要在 B/internal/arch/modules.go 加 seller（brief 已列），目前沒有 moderation 項（rg `"moderation"` 無結果）。
- 【量】另一個去向：CreateReport 的 category 對外是 `ViolationCategory` enum（reports.go:111）。

## 3. 待辦佇列 ListReviewQueue

- 【量】main 上不存在：`rg -n 'ListReviewQueue|ReviewQueue|review_queue' api backend/internal admin/src` 只命中 plan／brief 文件與無關的 invoice 稅務佇列（admin/src/app/admin/invoices/tax-reviews/page.tsx:43）；adminperm 沒有 `seller.review_queue.list`。
- 【量】核准設計（`B/docs/plans/2026-09-29-a3-buyer-seller/step10e-3-brief.md`，Will 2026-10-16 全照建議）：
  - D7（:105-180）：一支 `ListReviewQueue`，推導的讀取模型（一個 SQL `UNION ALL`，不另寫入），keyset `(due_time 有無, due_time, open_time, kind, id)`；項目種類 REPORT_ACKNOWLEDGE／REPORT_INVESTIGATE／FDA_REFERRAL／MEASURE_REQUEST／MEASURE_REVIEW／FOOD_REGISTRATION／KYC_APPLICATION（表 :149-157）；契約 `ListReviewQueueRequest{page_size,page_token,kind}`、`ReviewQueueItem{kind, oneof subject{report_id|measure_id(+request_id)|food_registration_id|kyc_application_id}, store_id, store_name, open_time, due_time}`（:169-176），放 `SellerAdminService`。**終局與觸發（:178-180）：出現第一個推導不出的工作狀態，或 social／invoice 搬進單體時，實體化成事件餵入的讀取模型，「佇列移到平台層，由各模組的事件投影，API 形狀不變」。** 約束帳 :約 133-139：social 不在單體是 REAL（過期，A7 就是觸發）。
  - D8（:181-250）：期限由 model 的 `ReviewPolicy` 擁有、日曆時間、讀取時推導：一般通報確認收件 72 小時、申訴與補正 7 天、KYC 7 天、食品登錄 7 天自可裁定起算、轉報 24 小時、食安確認 4 小時；:247「出現暫停規則時 breach_at 不再等於 open_time+固定值，那時 D7 要實體化，期限要另存」。
  - K17（:57）：新權限 `seller.review_queue.list`，只給同時擁有 5 種事項讀取權的角色；列只帶種類／ID／店名／時間，不帶內容；觸發條件：角色分工時改成伺服器依權限過濾。
- 【量】切分（:337-346）：3a 措施、3b 建立（對話框、D10 `finding_reports`／`source_report_ids`）、3c 通報（`assessed_category`、`resulting_measure_ids`、通報註記）、3d 待辦與 SLA（`ReviewPolicy`、`*_due_time`、`ListReviewQueue`、K17、K18 runbook）。
- 【量】進度（B/docs/plans/2026-09-29-a3-buyer-seller.md）：設計記在 :372（「切分 3a 措施（492→488）、3b 建立（→487）、3c 通報（→480）、3d 待辦與 SLA（480）」）；git log 顯示 3a 已落地 `946ed2d80 feat(seller): admin 措施清單與詳情頁（…第 3a 刀…）`，`maxRESTOperations` 現為 489（B/gateway/internal/router/openapi_coverage_test.go:28）；3b／3c／3d 沒有 commit，且 `rg 'source_report_ids|finding_reports|assessed_category|resulting_measure_ids|decide_due_time' api` 為空 → 三刀都未開工。plan 的 10e 勾選框仍未勾（:346），無分刀進度表。plan :373-374 記錄 L′（共用文案）與 i18n 決定，10e 目前優先序是 L′（見 Q4）。
- 【量】舊 REST 現況：admin 仍有 `/api/v1/admin/reports`（社群檢舉，admin/src/app/admin/reports/page.tsx:146,155）、`/api/v1/admin/posts`（posts/page.tsx:75,100）、`/api/v1/admin/violation_reports`（seller 通報，3c 要換掉）。
- 【推】A7 要不要做佇列取決於 3d：若 3d 先落地則 A7 把實作從「seller 內 UNION」換成「事件投影的平台層表」；若 A7 先，直接做平台層（brief D5′ 已寫）。兩者的 API 形狀 `ListReviewQueue` 必須同一份 proto（A7-0 要定誰擁有 proto：admin 的 `SellerAdminService` 還是新的平台 admin service）。

## 4. 10e 進行中（worktree <10e worktree>）

- 【量】HEAD `759cb81f8`（main 的 L′-1 commit，比 main 少 6b1cf36af 與 172260d31 兩個 social commit），`git status --short` 69 個檔變動 + 未追蹤 5 項；`git diff --stat`：69 files, +2967/−1722。這是 **L′-2（賣家端三個原因 enum 改成 `*Unavailability` 形狀，契約＋四端）並已進 L′-3 起手**（Lprime-design §8；`step10e-Lprime-design.md:757`）。
- 變動檔分群（`git -C … status --short`）：api 2（seller.proto、error_reason.proto）；backend：cmd/uitext-gen/main.go、arch/modules.go、httpapi/{gen（pb.go、connect.go）、rpc/errors.go+test、seller/{app,governance_measures,measures}.go+tests}、modules/seller/{AGENTS.md,CARRYOVER.md,governance.go,seller.go,internal/{measure/appeal.go,measure/remediation.go,model/model.go,model/measure_test.go,onboarding/onboarding.go},測試}、platform/twtime、runbook 與設計書 32；新檔 `seller/internal/model/unavailability.go(+_test)`、`seller/internal/text/`、`.github/workflows/ui-text-guard.yml`；packages：api-ts gen、hawkr_api gen、ui-text-ts/enum-manifest.gen.json、hawkr_ui_text/tool/enum_manifest.g.json；seller-app measure_display.dart／measures.dart／seller_onboarding.dart／seller_info_screen.dart＋測試；website seller/measures、seller/onboarding、lib/seller/*、e2e mock。
- 逐項（你指定的檔案）：
  - `API/seller/v1alpha1/seller.proto`（91 行變動）：新增 import `google/rpc/error_details.proto`；`GetMyStoreOpeningResponse` 欄位 2 `unavailable_reasons` 改 reserved，新欄位 3 `repeated StoreOpeningUnavailability unavailabilities`；新增 messages `StoreOpeningUnavailability`、`RemediationUnavailability`、`AppealUnavailability`；`Measure` 欄位 19、22 reserved，改 23 `remediation_unavailabilities`、24 `appeal_unavailabilities`。**沒有動 `MeasureKind` enum（:1040-1048）。**
  - `API/common/v1alpha1/error_reason.proto`：只改 `STORE_OPENING_SUSPENDED`／`STORE_OPENING_TERMINATED` 的註解（拿掉寫死的標籤、導向「措施與申覆」）。
  - `B/internal/arch/modules.go`：1 行，seller 內部圖 `"model": {"text"}, "store": {}, "text": {}`（新增 seller 內部 package `text`）；模組依賴表（Dependencies）沒動。
  - seller schema／measures：`db/schema.sql` 與 migrations **未改**（`git diff --stat -- backend/internal/modules/seller/db` 為空）；Go 的 measure/appeal.go、remediation.go、model/model.go（OpeningBlocker 改 Text(locale)）有改。
  - 術語表：**未改**（diff 為空；main 上 L0 已改 :19-23）。
  - uitext 目錄 `catalog/*.json`：**未改**；改的是 `cmd/uitext-gen/main.go` 與兩個 enum-manifest 生成檔；新檔 `modules/seller/internal/text/`（伺服器句子 JSON 與 Go）屬 seller 模組。
- 衝突風險（A7-1/A7-2/A7-4 會碰的檔，與 10e 在飛／後續刀重疊）：
  | 檔 | 10e 現況 | A7 何時碰 | 風險 |
  |---|---|---|---|
  | `API/seller/v1alpha1/seller.proto` | 在飛（Measure、Unavailability） | A7-1/2：`MeasureKind` 加值、店家封鎖方法 | 高：同檔；且 10e 的 reserved 編號（Measure 19/22→23/24）要先合；生成物（Go/TS/Dart pb）三份全衝突，需誰後落地誰重生（brief「和 10e 配合」） |
  | `seller/internal/model/model.go`、`governance.go`、`seller.go`、`internal/measure/*` | 在飛 | A7-1 加 kind／事件欄位／系統建立措施 | 中高 |
  | `B/internal/arch/modules.go` | 只動 seller 內部圖（一行） | A7-1：social 拿掉 im/review、新增 moderation、notification 加 moderation | 低（不同行），但 rebase 時同檔 |
  | `uitext/catalog/*.json` 與生成物 | 本批沒動；後續 L′-3…L′-8 會動 | A7-4 新文案、C9「內容下架」 | 中：L′-3 起 Go 改用目錄，之後 catalog 變動頻繁 |
  | `B/docs/開發規範/使用者面用語術語表.md` | 本批沒動 | A7-1 前要加「內容下架」 | 低，但歸 10e 擁有 |
  | `seller-app/.../measure_display.dart`、website `measureDisplay.ts`、admin measures 頁 | seller-app、website 在飛 | A7-4 加 kind 顯示 | 中 |
  | `seller/db/schema.sql`、migrations | 本批不動；10e 3b–3d 尚未開工，3c 會加 `assessed_category`、`governance_notes`（已存在）等 | A7-1 改 `measures_kind_check`、加內容對象 | 中：3c／3d 的 schema 變更與 A7-1 同表（reports、measures）；migration 時間戳排序與 atlas.sum 會衝突 |
  | `B/gateway/internal/router/openapi_coverage_test.go` `maxRESTOperations`（現 489） | 10e 各刀會再降 | A7-4 刪 42 個 social operation | 誰後落地誰重算 |
  | `API/admin/seller/v1alpha1/governance.proto`、`measure.proto` | 3b／3c／3d 預定要改（`source_report_ids`、`assessed_category`、`ListReviewQueue`） | A7-2 內容下架臂、平台佇列 | 高（同一組 proto） |
- 備註【量】worktree 基底落後 main 兩個 commit（`git -C hawkr-10e log -1` = 759cb81f8；merge-base 與其相同），main 多的 `6b1cf36af`、`172260d31` 是 social 身分第 1 刀與 A7 文件，與上列檔案不重疊（【推】未逐檔比對）。

## 5. delegation 稽核與事件接線

- 【量】delegation 稽核表（B/internal/modules/delegation/db/schema.sql）：`admin_operation_audit_logs`（:222）— 欄位 id、session_id（FK delegated_operation_sessions）、actor_admin_id、actor_display_name、target_store_id／target_seller_account_id／target_store_name、origin（interactive｜scheduled）、capability、action、resource_type（storefront｜product｜post｜order）、resource_id、resource_name、result（PENDING→SUCCEEDED｜FAILED）、biz_code、operation_reason（1–100）、case_reference、request_id、correlation_id、idempotency_key（(session_id,key) 唯一，:294）、payload_hash、before／after_snapshot、changed_fields、resource_version_before／after、user_agent、risk_level、notify_seller、notified_at、created_at、confirmed_at；action 封閉 19 值含 `POST_PUBLISH`（:270-276）。授權歷史另有 `delegation_authorization_audit_logs`（:305）。
- 【量】函式（delegation 根套件）：`RecordDelegationAuditIntent(s)`（delegation.go:324,328）、`ConfirmDelegationAudit(s)`（:332,336）、`DelegationAuditLogs`（:320）。intent：internal/audit/audit.go:88-135，session 由 `SessionForAudit(ID, Origin, Now)` 解析並填 actor／target 欄位；scheduled origin 允許 session 已結束（撤銷仍擋）；scheduled 的 operation_reason 由 `inheritedReason`（audit.go:210-224）從「同 session、同資源的 POST_SCHEDULE 那一列」繼承，找不到就拒絕（DenialAuditIntentFailed）。confirm 成功且 notify_seller 時在同 tx 發 `DelegationOperationCompleted`（model/events.go:11-33）；AGENTS.md：「稽核 intent 是寫入前置；confirm 只從 PENDING 轉移一次」。
  - intent 輸入（internal/model/delegation_audit_types.go:191-213）：SessionID、Capability、Action、ResourceType、ResourceID、ResourceName、OperationReason、ResourceVersionBefore、BeforeSnapshot、IdempotencyKey、RequestID、CorrelationID、UserAgent、PayloadHash、NotifySeller、Origin。confirm 輸入（:233-244）：AuditLogID、Result、BizCode、ResourceID*、ResourceVersionAfter、AfterSnapshot、ChangedFields。
- 【量】事件接線：`B/internal/platform/events/bus.go`——同步、同 transaction、in-process；`Subscribe[E]` 在組裝點註冊，handler 失敗則發布者 tx 回滾，`Publish` 必須在 `db.InTx` 內（:41-77）；事件型別必須是具體型別。delegation 目前只訂閱 identity 的 `AdminRoleRevoked`（B/cmd/hawkr/delegation.go:13 → `Module.OnAdminRoleRevoked` delegation.go:347）；沒有訂閱任何 seller／social 事件；delegation 發 `DelegationOperationCompleted`，由 `delegationservice.OnDelegationOperationCompleted` 轉 RabbitMQ（B/cmd/hawkr/main.go:203，B/internal/legacy/delegationservice/events.go）。
- 【量】舊 `RecordAuditIntent` server：`B/internal/legacy/delegationservice/audit.go:48`（`RecordAuditIntent`）、:62（Batch）、:128/:144（`ConfirmAuditResult(Batch)`）；掛載 `legacyMux.Handle(accountconnect.NewDelegationServiceHandler(delegationservice.New(del, log)))`（B/cmd/hawkr/main.go:293）。social 的呼叫端：`services/social-service/internal/worker/post_schedule_worker.go:260-282`（intent：Capability posts、Action POST_PUBLISH、ResourceType post、ResourceID=貼文 id、OperationReason 刻意留空、ResourceVersionBefore=row.Version、BeforeSnapshot、IdempotencyKey、CorrelationID=冪等鍵、NotifySeller=true、Origin=scheduled）；confirm :289-（AfterSnapshot `{publishStatus, publishedAt}`，帶總預算重試）。
- 【量】依賴表 B/internal/arch/modules.go：`"social": {"im","review","catalog","buyer","seller","identity","media"}`（:34）；`"delegation": {"identity","seller","ordering","catalog","social","media"}`（:36，delegation → social，所以 social 不能 import delegation，會成環）；`"notification"` 含 social、delegation（:39-41）；沒有 moderation 項。
- 【推】「social 發事件、delegation 訂閱並寫稽核」delegation 需要的欄位（事件須帶，其餘 delegation 自己查）：SessionID（排程當時的代行 session，social 要存 `scheduled_by_session_id`）、Capability(posts.publish) 與 Action(POST_PUBLISH)＝由 delegation 固定、ResourceType=post、ResourceID＝貼文 id（字串）、ResourceName（可選）、ResourceVersionBefore（發布當時版本，不 bump）、BeforeSnapshot、AfterSnapshot＝`{publishStatus, publishedAt}`、IdempotencyKey／CorrelationID（同一貼文每次重試相同）、Result（SUCCEEDED｜FAILED＋BizCode）、OccurredAt。actor、target store、reason 由 delegation 從 session 與 POST_SCHEDULE 列取得（audit.go:108,213）。結構性差異需 A7-0 定案：現行是 intent→（寫入）→confirm 兩段，event 在寫入同 tx 內發布只能表達單段「已完成」；若要保留「intent 前置」，delegation 需新增 `RecordCompleted`-類單段 API，或 social 發 `PostPublishIntent`／`PostPublished` 兩個事件（同 tx，順序保證）。事件在 social tx 內同步執行，handler 失敗會讓貼文發布回滾（嚴格稽核，但稽核表故障會擋發布）。

## 6. legacy relay 前例

- 【量】sellerrelay（B/internal/legacy/sellerrelay/relay.go）：模組事件 → 同 tx 內由 `OnX(q jobs.Enqueuer) func(ctx, seller.X) error` 排入 River job（Kind `legacy.seller_<事件>`，:112-122；例 `OnStoreSuspended` :139-143，transactional outbox）→ worker（`Worker.AddWorkers` :228-240，`typedWorker` 25s timeout :221）在 tx 外讀 DB 組 payload → `enqueuePublish`（:309-315）排 `rabbitmq.PublishArgs{EventID,Exchange,RoutingKey,Body}`（`rabbitmq.InsertOpts()` unique by event_id）→ `B/internal/legacy/rabbitmq` 的 publisher worker 送出（B/cmd/hawkr/main.go:417 `river.AddWorker(workers, rabbitmq.NewWorker(pub))`）。wire 與舊 pkg/mq 的 BaseEvent 逐欄位相同（`baseEvent` :243-257；exchange `hawkr.seller`／`hawkr.merchant`，source `seller-service`）。組裝：`subscribeSellerRelay`（B/cmd/hawkr/seller_events.go:29-40）；worker 側 `sellerrelay.NewWorker(sellerModule, jc, log).AddWorkers(workers)`（main.go:419）。測試：relay_test.go、wire_test.go、relay_integration_test.go。另有 buyerrelay（B/cmd/hawkr/buyer_events.go:16-23）可對照。
- 【量】legacy gRPC（Connect）server：`B/internal/legacy/sellerserver/server.go`（Server 內嵌 `UnimplementedSellerServiceHandler`，只實作仍被舊服務用的方法；其餘 Unimplemented）；掛載在 `legacyMux`（B/cmd/hawkr/main.go:289-293：`legacyMux.Handle(sellerconnect.NewSellerServiceHandler(sellerserver.New(...)))`，delegation 同行 :293），`legacyLn` 監聽 `legacyAddr`（預設 :9941，h2c，main.go:~290-297），`serveAll(...)`。生成碼 `B/internal/legacy/gen/<service>/<service>connect`（`make connect`，buf.gen.connect.yaml）。
- 【量】沒有 `legacy/socialservice`、`legacy/socialrelay`：`ls internal/legacy` 無此二目錄。social 目前會消費的 seller 事件走 RabbitMQ（sellerrelay 套件說明「下游（product、social、im、notification、pricebook）」，relay.go:56-57；`seller.deleted`／`seller.suspended`／`seller.exposure.changed` 等）。A7 後 social 變模組，這些改成 in-process 訂閱（見 Q8），RabbitMQ 側只剩 A8 前 notification 要消費的 social 事件（單體要加 socialrelay）。

## 7. 能力檢查

- 【量】seller 根 API（B/internal/modules/seller/seller.go）：`ActingStore(ctx, accountID, c Capability) (Store, error)`（:513）— 帳號「目前店家」有能力 c 才成功；停權 → PermissionDenied（SELLER_SUSPENDED）、其他前置 → FailedPrecondition（SELLER_REQUIREMENT_MISSING）、無店 → NotFound，fail-closed。`CapabilitiesOf(ctx, Store)`（:507）、`CurrentStore(ctx, accountID)`（:429）、`Store(ctx, storeID)`（:434，含已關閉 stint）。常數 `CapMarketing`、`CapContentContract`（seller.go:404-405；定義 internal/model/capability.go:26,28）。
- 【量】能力語意（capability.go 的 `needs` 表）：`CapMarketing` = Active ＋ NotSuspended ＋ NotPendingDeletion；`CapContentContract` = 只要 Active（停權也放行）。註解已寫 CapMarketing 含「貼文發佈」、CapContentContract 含「刪除、下架自己的貼文」。這正是 brief D3 的分法，A7 不需新增 seller 能力。
- 【推】ActingStore 只認「帳號目前店家」；social 在貼文屬於已關閉 stint 或要驗證「貼文所屬店 == 帳號的店」時，要先 `CurrentStore(accountID)` 比對 store_id（或 `Store(storeID)` + `CapabilitiesOf`）。
- 【量】Connect 方法宣告前例：`B/internal/httpapi/seller/declaration.go:20-60`——`declarations map[procedure]declaration{capability|exemptReason, limit}`，`requires(model.CapX)`／`exempt("理由")`；啟動時 `checkDeclarations` 雙向對照（缺漏或多餘不能啟動，declaration.go:~100-115）；interceptor 呼叫 `ActingStore` 並把店放進 ctx，handler 不自己檢查；測試 `declaration_test.go:51`（每個宣告恰好一個能力或豁免）、`kyc_integration_test.go:865`（宣告與模組強制一致）。ADR `2026-09-29-seller-suspension-and-business-identity-in-monolith.md` 文末修訂（:85-91，2026-10-17）已寫：宣告可掛在其他模組的 AppService（A7 的 `SocialAppService`），由擁有資料的模組先判斷是否店家身分，再呼叫 `seller.ActingStore`／`CapabilitiesOf`；各 service 各一張宣告表。
- 【量】目前 interceptor 只掛在 `SellerAppService`（ADR :54：「只掛在 SellerAppService、位在 rpc.NewInterceptor 之內」）；social 方法「依資源動態決定是否店家身分」不是靜態宣告能表達的 → A7 需要擴充：宣告表 + 在 social use case 內依資源推導後呼叫（ADR 修訂句：handler 仍不自己檢查，需決定放 interceptor 的 hook 還是 use case 的 port）。【推】

## 8. 曝光（exposure）

- 【量】seller 公開 API：`ExposableStores(ctx, ids)`（seller.go:441，即時計算，最多 MaxBatchSize，不可曝光略過，fail-closed）、`PublicStores`（:447）、`StoreCards`（:465，曾公開過的店，含停權與關閉）、`StoresWithExposure`（:480，legacy 退場用）、`ExposableStoreIDsByCounty`（:500，讀公告表、落後一個對帳週期）、`ListPublicStores`。單體內現行消費者：discovery 排行用 `ExposableStores`（B/internal/modules/discovery/internal/ranking/ports.go:41、recompute.go:132、ranking.go:124）；buyer httpapi 用 `StoreCards`／`PublicStores`（httpapi/buyer/ports.go:53-54）。**規則唯一來源（A′(i) 之後）：seller 的 SQL `seller.pub_exposable_stores(at)`；`model.Exposable` 已刪除，`MissingExposure` 只說明缺哪些條件。**
- 【量】事件：`StoreExposureChanged{OccurredAt, EventID, StoreID, Exposable, Version}`（model/model.go:316-327；SL/events.go）；可曝光翻轉在寫入 tx 內發，時間造成的變化（處分到期）由對帳 job 補發（internal/profile/reconcile.go）；Version 為每店公告序號，消費端依序丟棄舊版（ADR 2026-10-05-store-exposure-to-legacy 決策 1，ADR:33）。目前唯一訂閱者是 sellerrelay（seller_events.go:34）→ RabbitMQ `seller.exposure.changed`（舊 social-service 消費）。
- 【量】`reannounce-exposures`（B/cmd/hawkr/main.go:66,99；seller_commands.go:96）：seller 全量重新公告（不比對差異，版本照常加一）；用於種子重寫、下游表重建；事件由 `make worker` 送出（B/AGENTS.md 指令段）。
- 【推】A7 可選：(a) social 自己維護讀取模型（backend/AGENTS.md 規則 2：跨模組篩選／分頁時，在自己 schema 維護讀取模型，由對方事件在同 tx 更新）——訂閱 `StoreExposureChanged`（同步 in-process），依 Version 丟棄舊版，`reannounce-exposures` 即是重建手段；或 (b) 讀取時呼叫 `ExposableStores`（簡單，但貼文列表分頁／過濾要先拿到 store id 再過濾，會有少回資料的分頁問題）。brief 的 3a-②（曝光過濾）原先設計為事件餵入；A7 沿用。
- 【量】注意 seller 的 `StoreSuspended`／`StoreUnsuspended`／`StoreClosed`／`StoreRetired` 也是 in-process 事件，social 需要「店家個資清理後刪貼文」（StoreRetired；relay.go:56-57 說下游此時才刪資料）。

## 9. identity SuspendAccount 與「帳號有未關閉 stint」

- 【量】legacy server：`B/internal/legacy/accountservice/server.go:139-149`（`SuspendAccount`，註解「social 的 BAN_USER」；NotFound → connect NotFound，其他錯誤原樣回 gRPC 錯誤，沒有業務碼）→ `identity.Module.SuspendAccount(ctx, accountID, reason)`（B/internal/modules/identity/identity.go:218，「停權帳號並在同一個 transaction 排入 Clerk 停用 job；已停權時成功。reason 只寫入 log」）→ `internal/account/account.go`。呼叫端：`services/social-service/internal/usecase/dispatcher.go:118-147`（brief 引用）。
- 【量】identity 對 seller 沒有依賴（`Dependencies["identity"] = {}`，B/internal/arch/modules.go:15），SuspendAccount 內**沒有任何 store／stint 檢查**（`rg -n 'seller' internal/modules/identity/internal/account/account.go` 無依賴）。`rg -n 'SuspendAccount' --glob '*.go' -l` 命中：identity.go、accounts.sql.go、account.go、identity_integration_test.go、legacy/accountservice/server.go(+test)、social-service client、gateway 測試 mock。
- 【量】能不能查「帳號有未關閉 stint」：可以，經 seller 根 API：`CurrentStore(ctx, accountID)`（seller.go:429：「status 不是 CLOSED 的 stint，沒有時回 NotFound；停權的店也回傳」）；或 `StoreIDsByAccount`（:493，含已關閉）、`StoreOpening(ctx, accountID)`（:424）。因此「帳號底下有店（stint 未關閉）時拒絕 BAN」要由呼叫端（moderation，依賴表要含 identity、seller）做：`CurrentStore` 回成功 → 拒絕；NotFound → 呼叫 `identity.SuspendAccount`。【推】兩者之間有競態（開店發生在檢查與停權之間）：identity 有 `LockAccount`（identity.go:211，「與授予、移除角色及刪帳號序列化」）可供在同 tx 鎖帳號後再查，但 seller 的開店是否也持同一把鎖未查證（【推】accountdeletion 是前例，可對照 B/internal/modules/accountdeletion）。
- 【量】舊路徑 `SuspendAccount` 被 legacy server 保留供 social-service 呼叫；A7-3 的 legacy 處置（若 moderation 在單體內，social-service 的 BAN 呼叫消失，該 legacy RPC 可刪）。

## 驗證指令（可重跑）
- `rg -n 'ListReviewQueue|ReviewQueue' api backend/internal admin/src`
- `rg -n "measures_kind_check" backend/internal/modules/seller/db/schema.sql`
- `git -C <10e worktree> status --short | wc -l`（69 修改 + 5 未追蹤）
- `git -C <10e worktree> diff --stat -- backend/internal/modules/seller/db backend/internal/platform/uitext "backend/docs/開發規範/使用者面用語術語表.md"`（空）
- `rg -n 'moderation' backend/internal/arch/modules.go`（無）
- `ls backend/internal/legacy`（無 socialservice／socialrelay）
