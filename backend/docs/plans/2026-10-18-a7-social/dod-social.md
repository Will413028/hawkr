# social 模組完成定義核對（A7-5）

依據：主計畫 `backend/docs/plans/2026-09-24-go-modular-monolith.md` 的「模組完成的定義」18 項，順序同該清單。核對日 2026-11-19，分支 `a7-5`。路徑相對 `backend/`。計畫進度表在 `backend/docs/plans/2026-10-18-a7-social.md`，下稱「計畫」。

結果：成立 16、不適用 2、未成立 0。

## 未成立

無。第 3 項與第 8 項已於 2026-11-19 補齊（見逐項）；第 3 項裡屬於檢舉面的 SLA 告警沒有處置，記在 `dod-moderation.md`。

## 逐項

| # | 項目 | 狀態 | 證據 |
|---|---|---|---|
| 1 | 目錄與 `AGENTS.md` | 成立 | `internal/modules/social/AGENTS.md` 有職責、擁有的資料、發布的讀取介面、不變量、不負責、決策紀錄（`rg -n '^## ' internal/modules/social/AGENTS.md` 7 段）。是否以 `make new-module` 建立，沒有可重跑的證據，只確認結果的形狀 |
| 2 | 沿用盤點 | 成立 | `internal/modules/social/CARRYOVER.md` 含「沿用盤點」與第 104 行起的「傳輸層的沿用盤點」，M1–M27 逐項；`GOWORK=off go test -count=1 ./cmd/hawkr -run TestEveryMovedModuleHasACarryOverAudit` ok |
| 3 | 公司時期未完成項 | 成立 | `internal/modules/social/CARRYOVER.md`「公司時期未完成項」：#2692 修掉（`TestSD1DraftIsNotPublic`、`TestSD1SD2PublicReadabilityPredicate`）；#1632、#2779、#3513 屬檢舉面，結論在 `internal/modules/moderation/CARRYOVER.md` 同名段落（#1632、#2779 不再成立，#3513 的 SLA 告警未處置，見 `dod-moderation.md`） |
| 4 | 獨立設計審查 | 成立 | 子步驟審查在計畫進度表：A7-1a（開工 21 項、收尾兩輪）、A7-1f（5 項、4 項）等，各列「設計審查」欄；A7-5 完成前審查 2026-11-19：5 項，1 項 should 交 Will 成為 A7-D29、實作在 `a7fa358e`；1 項 should 是計畫收尾項（進行中，即本核對與計畫收尾，不是未處理）；3 項 nit 改在 `f4ddb6ed`、`90dbbf85` 與本步驟 |
| 5 | schema 在模組 `db/`、arch 與 SQL 檢查 | 成立 | `internal/modules/social/db/`（`schema.sql`、`objects.sql`、`migrations`、`queries`）；`GOWORK=off go test -count=1 ./internal/arch/...` ok；`schema_integration_test.go`（`TestChecksMatchGo`、`TestSamePostIsEnforcedByTheDatabase` 等）。integration 由 CI 執行（A7-1a 到 A7-4e 各列的 backend monolith run） |
| 6 | 公開 API 依賴不超出 D2 表 | 成立 | `internal/arch/modules.go:32` `"social": {"buyer", "seller", "media"}`，`arch` 測試 ok；`content`、`delegation`、`moderation` 對 social 的依賴分別在第 20、34、37 行 |
| 7 | RPC 去向 | 成立 | `backend/docs/plans/2026-09-24-go-modular-monolith/rpc-servers.csv` 中 `implementing_deployable=social-service` 34 列，空白 0（csv 讀取計數）；`python3 scripts/check-migration-inventory.py --assume-gone social-service` exit 0。刪除：`ReportPost`、`AdminReopenReport` 等標 `removed` 並註明理由 |
| 8 | 事件去向 | 成立 | `events.csv` 中 social-service 為 producer 或 consumer 的 14 列都有 `disposition`（`python3 scripts/check-migration-inventory.py --assume-gone social-service` exit 0）：relay 6 與 `buyer.deleted`、`seller.deleted` 為 `in-tx-event`，gateway 的 5 列 SSE 與 `counter.delta` 為 `removed`（`comment.updated`、`comment.deleted` 的 pkg/mq 建構子已於 82369b78 移除） |
| 9 | 副本去向 | 成立 | `replicas.csv` 中 `holder_deployable=social-service` 2 列皆有 disposition（`reports.target_snapshot` 改由 moderation 快照；作者顯示名副本）；腳本 exit 0 |
| 10 | 外部 provider（D5） | 不適用 | social 沒有外部 provider 呼叫：`rg -n 'net/http"|http\.Client|stripe|payuni' internal/modules/social internal/modules/moderation -g '*.go' -l` 無結果；網站快取失效走 `platform/revalidate`（transaction 內排 River job，CARRYOVER M26），是自家 Next.js |
| 11 | 代碼值與錯誤碼去向 | 成立 | CARRYOVER 的 CodeMeta 列（例：`publish_status` 4401–4403 改 `status` TEXT 加 CHECK）；`apperr` 與 `ErrorReason`；`TestChecksMatchGo` 守 CHECK 與 Go 型別一致 |
| 12 | 測試 | 成立 | 模組內 185 個 `Test` 函式，25 個 integration 檔（`rg -l 'go:build integration' internal/modules/social`），unit 另有 `schedule_authorizers_test.go`、`keyset_sqlc_test.go` 等；不是 Java 改寫，無對照表。integration 本機不跑，證據是計畫各列的 CI run（例 A7-4e：`cb9ad1ca` 的 backend monolith run 44514697332） |
| 13 | 特性測試 | 成立（偏離清單字面） | 情境在 `internal/httpapi/social/characterization/`（`scenarios_test.go`、`block_scenarios_test.go`、`connect_test.go`、`monolith_integration_test.go`），只由單體的 Connect driver 跑。REST driver、`legacy.sh`、`TestLegacy`、`skipOnLegacy` 與 `backend-characterization.yml` 已在 `cb9ad1ca` 刪除：A7-4e 刪了 gateway 路由，舊端不可能再執行（計畫 A7-4e 列）。所以「REST driver 對舊 gateway 也通過」這一半沒有、也不可能再有 |
| 14 | 對外 API | 成立 | proto 在 `api/hawkr/social/`，handler 註冊在 `internal/httpapi/server.go:310-322`（`SocialPublicService`、`SocialAppService`、`SocialAdminService` 等）；`buf lint`、生成物 drift 由 `api-contract.yml` 守 |
| 15 | client | 成立 | A7-4a（website）、4b（admin）、4c（buyer-app）、4d（seller-app）、4e 在計畫進度表皆完成；gateway 與清單：`rg -n -i 'social|/posts|/comments' backend/api/openapi/openapi.yaml` 無社群端點，`maxRESTOperations` 減 40（`gateway/internal/router/openapi_coverage_test.go:28`）。允許殘留：聊天與評價入口的封鎖、檢舉仍打舊 REST，列為 A6 的殘留（計畫 S3、Q1 (i)） |
| 16 | 刪除舊服務 | 成立 | `ls services/social-service` 不存在；`3415e293`（目錄、build、CI、compose）、`196abed6`、`875714aa`（MQ queue 與 proto allowlist）、`20a8db4b`（legacy proto 與生成碼）、`8bebeea0`、`f73429c8`。殘留字串：`pkg/server/http_test.go:169` 的 `ServiceName: "social-service"` 與 `external-systems.csv:24`（盤點歷史列），都不是服務目錄或接線 |
| 17 | D9 gate | 不適用 | gate 腳本 `scripts/check-legacy-endpoints.py` 與其 workflow 已在 `7fb87b1d`（作品集轉 public 步驟 9）整個移除，現行沒有可跑的 gate；social 的新碼沒有引入舊公司端點的證據是第 10 項的 `rg` |
| 18 | 相關 ADR | 成立 | 取代：`2026-10-23-social-counts-in-stats-tables`（取代 `2026-09-18-mq-social-counter-async-delta`，後者已標 superseded）；`2026-10-17-social-actor-identity`；A7-5 的 `52678b02` 在 social、connect、SSE、delegation、suspension 五份 ADR 補終態修訂 |
