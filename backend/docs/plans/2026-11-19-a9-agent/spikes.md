# A9-0 spike 結論（dev `3f04d39b`，2026-11-19）

spike 程式用完已刪；結論與重跑方式記在這裡。

## 1. ADK session 改 pgx（A9-D5）

**結論：可換，不改 ADK 原始碼。**【量】

- `google.golang.org/adk/v2` v2.6.0 的 `session.Service` 是公開 interface，5 個方法：`Create`、`Get`、`List`、`Delete`、`AppendEvent`；`Session`、`State`、`Events` 也是公開 interface，`Event` 是帶 json tag 的公開 struct，外部套件可自行實作與序列化（`session/service.go`、`session/session.go`）。
- `runner.Config.SessionService` 的型別就是 `session.Service`（`runner/runner.go:54`）；`runner/`、`agent/`、`internal/` 的非測試程式沒有對 ADK 自己實作的 type assertion【量，grep】；`internal/compactioninternal/apply.go:695-715` 的 `sessionUnwrapper` 是選用的。未以 runner 實跑一輪 agent【未驗】。
- `Service` 沒有條件寫入或版本號（`runner/runner.go:406`），ADK 自己的 gorm 實作也一樣；不是換 pgx 才有的限制。
- 判準測試：以 pgx 形狀的 `querier` 介面實作 `session.Service`，用記憶體替身（本機不起 Postgres）跑「存兩輪、讀回」：`--- PASS: TestTwoTurnsRoundTrip`【量；指令 `GOWORK=off GOFLAGS=-mod=mod GOPROXY=off go test ./...`】。
- ADK 附公開的合格測試 `session/sessiontestsuite.RunServiceTests`：spike 刻意沒做的 5 項語意失敗（重複 `Create` 要回錯、`List` 的空 UserID 列全部、`app:`／`user:` 前綴 state 共用、寫入 event 時剝掉 `temp:` key），其餘 21 項通過。正式實作要補齊，並以 `RunServiceTests` 在 CI 的 Postgres 上當驗收。
- 現況接點：`backend/services/ai-service/internal/agent/di.go:47` 的 `provideSessionService`（`gorm.Open` 加 `database.NewSessionService`）。

## 2. 串流穿透

| 層 | 限制 | 證據 |
|---|---|---|
| 採購串流第一個事件 | `RunAccepted` 在 `ClaimRun` 之後、LLM 之前送出；之前只有 1–3 次同步 DB 查詢 | `turn_run_lifecycle.go:117-131`、`graph_dispatch.go:173-183`【推，讀碼；需 Postgres 才量得到實數，本機不起】 |
| 一條串流最長 | gateway 單輪上限 `streamTurnDeadline` 120s；背景預算 `runDispatchBudget` 150s；實測成功的輪次 16–46s | `ai_assistant_stream.go:82,207`、`run_dispatch.go:112`、`interface-inventory.md` §4.3 |
| 單體 gateway proxy | `ResponseHeaderTimeout` 60s 只管 header；`FlushInterval: -1` 逐 frame 送出；不限總時長 | `internal/legacy/gatewayproxy/proxy.go:71,260,276`【讀碼】 |
| 單體 `http.Server` | 只設 `ReadHeaderTimeout` 10s，沒有 Read／Write／Idle timeout | `cmd/hawkr/main.go:511-512`【讀碼】 |
| website 同源反代 | Next `rewrites().beforeFiles` 的外部 rewrite，不經 function，沒有 `maxDuration` | `website/next.config.ts:374-377`、`website/lib/api/local-proxy.ts:120-140` |
| Vercel 外部 rewrite | **Proxied request timeout 最長 120 秒**，逾時回 `ROUTER_EXTERNAL_TARGET_ERROR`；文件沒寫明是首位元組還是總時長，也沒寫串流是否緩衝 | [Vercel Limits](https://vercel.com/docs/limits#proxied-request-timeout)【文件，2026-11-19 讀】；未在 Vercel 上實量 |
| admin BFF | `runtime = "nodejs"`，`new NextResponse(upstream.body)` 直接透傳；無 `maxDuration`；admin 不用這 12 條串流 | `admin/src/app/api/bff/[...path]/route.ts:11,91-95` |
| App | Dio `receiveTimeout` 3 分鐘（兩次收資料的間隔，非總時長）；`agui_sse.dart` 無 timeout | `buyer-app/.../ai_assistant_repository.dart:90`、`seller-app/.../ai_assistant_repository.dart:84`【讀碼】 |

**結論：** 單體與 App 不會砍長串流；唯一的上限是 Vercel 外部 rewrite 的 120 秒，文件無法排除它算總時長，而現行單輪上限也是 120 秒，貼著邊。做法見計畫前提「串流時長」。
