# A9-6 殘留清單（預期）

產生指令（在 repo 根目錄，dev `53555604`）：

```bash
rg -l "ai-service|aiservice" . | grep -vE "^\./(docs|backend/docs)/|^\./backend/services/ai-service/" | sort
rg -ln "modules/agent|httpapi/agent" backend | grep -v "^backend/internal/modules/agent/" | grep -v "^backend/docs/" | sort
```

A9-6 的驗收：第一條指令的輸出等於下方「允許殘留」清單；第二條輸出為空（A9-D1 刪除 `agent` 模組）。下表以檔案群組標處置，執行時逐檔核對，新增命中要先分類再收尾。

## 刪除

| 群組 | 步驟 |
|---|---|
| `backend/api/proto/ai/`、`backend/pkg/proto/ai/`、`backend/services/java/shared/java-proto/src/main/proto/ai/` | A9-6 |
| `backend/gateway/internal/{router/routes_vendor_portal.go,middleware/vendor_portal_session*,handler/vendor_portal*,grpc/ai_*,grpc/circuit_breaker_stream_interceptor_test.go,handler/ai*,handler/procurement_errors*,procurement/,proxy/agui_golden_test.go,proxy/ai_*}` | A9-6 |
| `backend/internal/modules/agent/`、`backend/internal/httpapi/agent/`、`backend/internal/httpapi/vendor_portal_test.go`、`backend/internal/legacy/gatewayproxy/vendor_portal_test.go`、`backend/internal/legacy/sellerserver/invites*`、`backend/cmd/hawkr/agent*.go`、agent 模組規格（`modules/agent.md`） | A9-1（A9-D1） |
| `api/hawkr/agent/v1alpha1/` 與三端生成的 `agent` 目錄、website 的 vendor-portal client 與頁面、`identity.VerifyGoogle` 與 `rpc` 的 invitee 受眾（只有供應商入口使用） | A9-1（A9-D1；盤點時漏列，A9-1a 執行時補） |
| `seller-app/lib/features/ai_assistant/`、`seller-app/lib/features/product/data/product_listing_chat_repository.dart`、`website/lib/buyer/seller-match-chat*`、`website/lib/seller/ai-listing-chat.ts` | A9-5（A9-D2） |

## 修改

| 群組 | 改什麼 | 步驟 |
|---|---|---|
| `backend/cmd/hawkr/{main,main_test,modules}.go`、`backend/internal/httpapi/server.go`、`backend/internal/httpapi/public_services_test.go`、`backend/internal/legacy/gatewayproxy/proxy*.go`、`backend/internal/legacy/sellerserver/{errors,server,server_integration_test}.go`、`backend/internal/arch/modules.go`（第 39、42、97 行）、`backend/cmd/new-module/templates/characterization/monolith_integration_test.go.tmpl`、`httpapi/{buyer,content,identity,moderation,seller,social}` 的 integration test | 拿掉 `agent` 模組與供應商入口的接線 | A9-1 |
| `backend/gateway/internal/{router/constants.go,router/proxy_failure_status_test.go,middleware/ratelimit.go,handler/testutil_test.go}`、`backend/api/openapi/openapi.yaml`、`website/lib/api/openapi-types.ts` | 刪 AI 路由與 operation | A9-6 |
| `backend/{go.work,Makefile,dev.sh,.env.template,docker-compose-local.yml}`、`backend/scripts/{README.md,expected-skips.json}`、`scripts/govulncheck-allowlist.json`、`.github/workflows/{backend-go-checks,backend-integration-tests,backend-sqlc-generated-diff}.yml`、`.github/dependabot.yml`、`backend/services/im-service/Makefile` | 拿掉 ai-service | A9-6 |
| `backend/monitoring/prometheus/{rules/service-alerts.yml,tests/ai-llm-budget-alerts.test.yml}` | job 名改成單體；指標名沿用（A9-D11） | A9-6 |
| `backend/monitoring/prometheus/{rules/business-alerts.yml（vendor-portal-alerts 群組）,tests/vendor-portal-alerts.test.yml}` | 刪供應商入口告警（A9-D1） | A9-6 |
| `backend/tests/contract/golden/event_payload_keys.json` | 依 A9-D1 刪 `external_created`；保留的事件改由 relay 產生 | A9-3c |
| `website/features/procurement-assistant/*`、`website/lib/seller/ai-image.ts` | 改用生成的 Connect client | A9-5 |
| `backend/internal/httpapi/rpc/session.go` | 註解提到 ai-service 的地方改寫 | A9-6 |

## 允許殘留（不在本階段改）

| 檔 | 理由 |
|---|---|
| `admin/src/lib/codemeta/generated.ts`、`website/lib/codemeta/generated.ts`、`buyer-app/lib/core/codemeta/code_meta.dart`、`seller-app/lib/core/codemeta/code_meta.dart`、`backend/internal/platform/codemeta/code_meta.go`、`backend/pkg/codemeta/*` | CodeMeta 凍結、生成檔不手改，隨模組搬遷退場 |
| `backend/api/proto/{order,seller}/*.proto`、`backend/pkg/proto/{order,seller}/*`、`backend/internal/legacy/gen/{order,seller}/**`、`backend/services/java/**` | 其他舊服務的契約與程式，隨 B8、A3 退場 |
| `backend/services/{notification,seller}-service/**` | 舊服務，隨 A8、A3 退場 |
| `backend/internal/modules/{seller,content}/**`、`api/hawkr/admin/seller/v1alpha1/prospect.proto` 與其生成檔 | A3 的範圍（本階段不改 seller）；seller 的 `prospect` 註解在 A3 或 A9 收尾時另提 |
| `backend/pkg/{server,media,outbox,middleware}/**` | 舊服務共用套件，隨最後一個舊服務退場 |
