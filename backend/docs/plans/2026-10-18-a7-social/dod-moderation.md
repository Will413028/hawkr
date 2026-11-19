# moderation 模組完成定義核對（A7-5），含 reviewqueue 新增來源

依據：主計畫 `backend/docs/plans/2026-09-24-go-modular-monolith.md` 的「模組完成的定義」18 項，順序同該清單。核對日 2026-11-19，分支 `a7-5`。路徑相對 `backend/`。moderation 是新模組：沒有自己的舊服務，來源是 social-service 的檢舉部分，加 seller 通報與 `reviewqueue`。

結果：成立 12、不適用 4、未成立 2。

## 未成立

- 第 3 項（公司時期未完成項）：`internal/modules/moderation/CARRYOVER.md` 已有「公司時期未完成項」表（2026-11-19）：#1632、#2779 不再成立，#3513 的稽核與整合測試修掉、字典詞庫延後有觸發。剩 #3513 的 SLA 告警未處置：待辦列有 72 小時期限，沒有主動告警的程式也沒有決定（A7-D14 只延後優先度提升）。缺：Will 決定做、延後（給觸發）或刪除。
- 第 13 項（特性測試）：moderation 的對外 endpoint（`ModerationAppService`、`ModerationAdminService`）沒有經 driver 介面的情境：`internal/httpapi/social/characterization/` 只有 `monolith_integration_test.go` 裝配 `Moderation` 模組，沒有檢舉情境（`rg -n -i 'SubmitReport' internal/httpapi/social/characterization` 無結果）。現有覆蓋是 handler 測試 `internal/httpapi/moderation/{app_test,admin_test,admin_integration_test}.go`（24 個 `Test`）加模組 integration。缺：driver 情境，或 Will 確認新模組不需要（舊 REST 側已不存在，雙跑已不可能）。

## 逐項

| # | 項目 | 狀態 | 證據 |
|---|---|---|---|
| 1 | 目錄與 `AGENTS.md` | 成立 | `internal/modules/moderation/AGENTS.md` 有職責、擁有的資料、發布的讀取介面、不變量、不負責、決策紀錄 7 段（`rg -n '^## '`）。`make new-module` 是否使用無可重跑證據 |
| 2 | 沿用盤點 | 成立 | `internal/modules/moderation/CARRYOVER.md` 含「沿用盤點」與第 32 行起「傳輸層的沿用盤點」；`TestEveryMovedModuleHasACarryOverAudit` ok（`cmd/hawkr`） |
| 3 | 公司時期未完成項 | 未成立 | 見上；只剩 SLA 告警未處置 |
| 4 | 獨立設計審查 | 成立 | 計畫 A7-1d moderation 列：設計審查 8 項（should 4、nit 4，改 6、交 Will 2 → A7-D20）與 5 項（should 3、nit 2，改 3、交 Will 2 → A7-D21）；A7-1e 列獨立 diff 審查 1 項 should；A7-5 完成前審查 2026-11-19（見 `dod-social.md` 第 4 項） |
| 5 | schema 在 `db/`、arch 與 SQL 檢查 | 成立 | `internal/modules/moderation/db/`（`schema.sql`、`objects.sql`、`pub.sql`、`migrations/20261115033604_moderation_schema.sql`、`20261115120000_moderation_objects.sql`）；`internal/arch` ok；`schema_integration_test.go` 的 `TestChecksMatchGo`、`TestCaseChecksRejectIllegalStates`。案件、檢舉、快照不跨 schema FK（對象以不透明鍵引用） |
| 6 | 公開 API 依賴不超出 D2 表 | 成立 | `internal/arch/modules.go:37` `"moderation": {"social","seller","buyer","accountsuspension","media"}`；`arch` 測試 ok；`reviewqueue` 的列在第 38 行 |
| 7 | RPC 去向 | 成立 | 接手 SocialService 的檢舉 RPC，`rpc-servers.csv` 皆已填：`SubmitReport`→`ModerationAppService.SubmitReport`、`ListMyReports`→`ModerationAppService.ListMyReports`、`AdminListReports`→`ModerationAdminService.ListCases`、`AdminGetReport`→`GetCase`、`AdminResolveReport` 與 `AdminModeratePost`→`DecideCase`；`ReportPost`、`AdminReopenReport` 標 `removed` 並註明理由。腳本 exit 0 |
| 8 | 事件去向 | 不適用 | `events.csv` 沒有 producer 或 consumer 是 moderation 的列（核對用 `rg -n 'moderation' backend/docs/plans/2026-09-24-go-modular-monolith/events.csv`，無結果）；social-service 的事件列歸 social（其 CSV 缺口記在 `dod-social.md` 第 8 項）。未逐一核對舊 social-service 是否有檢舉相關事件列：events.csv 的 14 列中沒有以 report 命名者 |
| 9 | 副本去向 | 成立 | `replicas.csv` 的 `reports.target_snapshot` 列：「案件的內容快照由 moderation 在提交時凍結（A7-D14，SD21）」；快照圖片引用 `internal/mediaref` 與 `retired_assets`（A7-D21） |
| 10 | 外部 provider（D5） | 不適用 | 沒有外部 provider 呼叫（`dod-social.md` 第 10 項的 `rg` 同時掃 moderation，無結果） |
| 11 | 代碼值與錯誤碼去向 | 成立 | CARRYOVER 第 19 行（檢舉原因 8 種的 CodeMeta `ReportReasonCode`）與第 39 行（錯誤碼改 `apperr`）；CHECK 與 Go 一致由 `TestChecksMatchGo` 守 |
| 12 | 測試 | 成立 | 模組內 73 個 `Test` 函式，8 個 integration 檔（`decide_`、`lifecycle_`、`list_`、`open_report_counts_`、`request_id_`、`schema_`、`submit_`、`fixture_`）；CI 證據：計畫 A7-1d、A7-2f 列（A7-2f PR #392 HEAD `4abda157` 16 個 workflow success）。非 Java 改寫 |
| 13 | 特性測試 | 未成立 | 見上 |
| 14 | 對外 API | 成立 | `api/hawkr/moderation/`；`internal/httpapi/server.go:316-317` 註冊 `ModerationAppService`、`ModerationAdminService`；`api-contract.yml` 守 |
| 15 | client | 成立 | buyer-app `lib/features/moderation/`（`moderation_repository.dart`、`report_sheet.dart`、`my_reports_*`）、admin 檢舉頁（A7-4b）、seller-app 的 4d-5 移除店主檢舉入口（moderation 只收買家檢舉）；舊 REST 路由與清單端點已於 A7-4e 刪除（`dod-social.md` 第 15 項） |
| 16 | 刪除舊服務 | 不適用 | moderation 沒有自己的舊服務；來源 social-service 的刪除記在 `dod-social.md` 第 16 項（`3415e293`） |
| 17 | D9 gate | 不適用 | gate 已於 `7fb87b1d` 移除（見 `dod-social.md` 第 17 項） |
| 18 | 相關 ADR | 成立 | `2026-10-29-account-suspension-and-moderation` 取代 `2026-09-18-content-moderation-consolidation` 的 D1、D3（後者已標「被取代」，D2、D4 不變）；`2026-10-20-admin-review-queue` 是待辦來源的 ADR |

## reviewqueue：A7 新增的來源

A7 只新增一個來源：moderation 的進行中案件，種類 `MODERATION_CASE`（A7-1e，計畫第 388 行；其餘來源 seller、payment 不是 A7 加的）。

| 檢查點 | 證據 |
|---|---|
| 來源函式 | `moderation.pub_review_items(at)`（`internal/modules/moderation/db/pub.sql:21`、`objects.sql:15,25`）：一列一個 `closed_at IS NULL` 的案件，期限只寫在 `review_moderation_case(at)` |
| 查詢分支 | `internal/modules/reviewqueue/internal/model/model.go:32` 有 `ModerationCase` 種類；`reviewqueue/db` 的 `UNION ALL` 分支與 `sqlc.yaml` 讀 moderation 的 `pub.sql`；依賴 `internal/arch/modules.go:38` 含 moderation |
| 契約 | `api/hawkr/admin/reviewqueue/v1alpha1/review_queue.proto:51` `REVIEW_QUEUE_ITEM_KIND_MODERATION_CASE = 13` |
| 權限 | `internal/httpapi/reviewqueue/review_queue_test.go:192` `rq.ModerationCase: adminperm.ModerationCasesList`；空 kinds 是否含該種類的測試（第 600 行附近）；`GOWORK=off go test -count=1 ./internal/httpapi/reviewqueue/` ok |
| 資料集與分頁測試 | `reviewqueue_integration_test.go`：`TestModerationCasesAreInTheQueueExactlyWhenOpen`、`TestModerationCaseDeadlineDoesNotDependOnAt`、`TestCrossSourceKeysetPagingWithModeration`、`TestKindsMatchTheSource`；CI：A7-1e 列 dev `54f32273` backend monolith run 43687010634，mutation「漏掉進行中的買家案件」兩個資料集測試紅（run 43686107572） |
| admin 窮舉 | `rg -n 'MODERATION_CASE' admin/src` 命中 `admin/src/app/admin/queue/queue-item.ts` |
| `reviewqueue` 自身完成定義 | 沒有 schema、沒有舊服務；`reviewqueue/CARRYOVER.md` 有兩張盤點表；第 7–9、15、16 項不適用（新模組、無舊 RPC、事件、副本、client 之外的舊碼） |

結論：新來源有 SQL、查詢分支、契約、權限、admin 呈現與測試，無缺口。
