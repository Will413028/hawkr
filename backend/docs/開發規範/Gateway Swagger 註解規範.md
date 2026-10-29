# Gateway Swagger 註解規範

> **適用範圍（2026-09-26 起）**：§1 到 §5 不再適用。對外契約改為根目錄 `api/` 的 Protobuf＋Connect（ADR `docs/adr/2026-09-26-external-contract-protobuf-connect.md`）；舊 gateway 的 REST 記錄在現有 API 清單 `api/openapi/openapi.yaml`，改 gateway 的輸入輸出或刪路由時改清單，swag 註解不再維護，模組搬進單體時隨路由移除。§6 的 gateway 測試 mock 同步，在 gateway 還存在時仍然適用。


新增或修改 Gateway handler 函數時必須同步維護 Swagger 註解。**Swagger 是與前端溝通的唯一管道**，註解的參數名稱、型別、預設值、Enum 合法值必須與實際程式碼輸入輸出完全一致。

## 1. 必要註解欄位

| 欄位 | 說明 | 範例 |
|------|------|------|
| `@Summary` | 簡短標題（繁體中文） | `取得訂單列表` |
| `@Description` | 詳細描述含業務邏輯（繁體中文） | `依帳號取得訂單列表，支援分頁` |
| `@Tags` | 分組標籤，格式 `中文 (English)` | `訂單 (Order)` |
| `@Accept` | 請求格式 | `json` |
| `@Produce` | 回應格式 | `json` |
| `@Security` | 需驗證的端點必填 | `BearerAuth` |
| `@Param` | 請求參數（path / query / body） | 見 §5 |
| `@Success` | 成功回應結構 | `200 {object} object{code=int,data=...}` |
| `@Failure` | 錯誤回應結構 | `400 {object} object{code=int,message=string}` |
| `@Router` | 路由路徑與 HTTP 方法 | `/app/orders [get]` |

## 2. 同步時機

| 變更類型 | 必須更新 |
|---------|---------|
| 新增 handler 函數 | 完整 Swagger 註解 |
| 修改路由路徑 | `@Router` |
| 修改 request / response 結構 | `@Param`、`@Success` |
| 變更權限需求 | `@Security` |
| 變更 Namespace | `@Router` 路徑前綴 |

## 3. 重新生成文件

**契約的唯一真實來源是 handler 註解**（`@Summary` / `@Param` / `@Success` / `@Router`）。
`swagger.json` / `swagger.yaml` 是 `swag init` 的**終端產物**，**不在版控**（`.gitignore` 的 `gateway/docs/`），
唯一消費者是 Apidog——由 `sync-dev-apidog.yml`（push `dev`）與 `sync-apidog.yml`（push `main`）
在 runner 內先跑 `make swagger` 再上傳剛生成的檔。

| 動作 | 做法 |
|---|---|
| 改契約 | **只改 handler 註解**，push 後 CI 自動重生成並同步 Apidog |
| 本地想查某端點的 wire 形狀 | `cd gateway && make swagger`，再 grep `gateway/docs/swagger.yaml`（輸出被 gitignore，不會進 diff） |
| 手寫編輯生成檔 | **禁止**——下次生成即覆蓋 |
| 把 `gateway/docs/` 加回版控 | **禁止**（見下） |

> ⚠️ **為什麼生成檔不入版控**：沒有任何程式讀它（repo 內零 import、Gateway 未註冊 Swagger UI 路由、
> 部署流程不讀），而版控副本要保持新鮮就得有人 regenerate 後 commit——本規範刻意不設這道義務，
> 因為**沒有任何地方釘死 `swag` 版本**：CI 用 `go install …@latest`，各人本機裝到哪版算哪版
> （盤點當下實測本機 `v1.16.4`、`go.mod` 記的是 `v1.16.6`、CI 又是 `@latest`，三處互不相干；
> 生成物退出版控後 `go.mod` 那一處已隨 tidy 移除，剩下兩處仍不對齊）。
> 強制對拍只會製造與程式碼無關的紅燈。
> 沒有義務就必然腐化：2026-08-05 實測，當時版控裡那份比源碼少 24 個 definition、含 2 個已刪除的幽靈端點，
> `evidenceType` 的描述還停在早就改掉的舊值——**一份會騙人的契約比沒有契約更糟**。
>
> 註解與 handler 實際輸出對不對得上，由 `gateway-json-contract.yml` 的 Check B 守（源碼對源碼，不依賴生成檔）。

## 4. DTO 規範

- Request / Response **必須定義 exported Go struct**（PascalCase），**禁止** `object{...}` 行內定義
- 命名規則：`{Action}{Resource}Request` / `{Resource}Response`
- 定義在 handler 檔案頂部 DTO 區塊

## 5. 註解品質規則

| 規則 | 說明 |
|------|------|
| `@Param` 數字參數必須 `string` | 禁止 `int`，避免前端判斷錯誤 |
| DTO Enum 欄位必須附註合法值 | 例：`// 合法值：ORGANIC, TAP, CAS` |
| `@Param body` 必須引用 exported struct | 禁止匿名 object |
| 分頁回應用 `records` | 禁止 `list`、`items` 等其他命名 |

## 6. Gateway Test Mock 同步

新增或修改 gRPC RPC 方法時，**必須同步更新 Gateway handler test 的 mock struct**，否則 CI 編譯失敗。

- 每個 gRPC service 的 mock 定義在對應的 `*_test.go`（如 `grpc_mocks_test.go` 定義 `mockProductServiceClient`）
- 新增 RPC → mock struct 必須新增對應 stub 方法（回傳 `nil, nil` 即可）
- 修改後用 `go test -count=1 ./internal/handler/` 驗證（`-count=1` 強制不用 cache）

**為何強制 `-count=1`**：本地 `go test` 可能用 cache 結果顯示通過，但 CI 是 clean build 會直接失敗。
