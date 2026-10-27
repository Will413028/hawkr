# 對外契約改用 Protobuf＋Connect：content 試點驗證計畫

**Goal：** 用 content 模組把「Protobuf 定義、Connect 協定、四端都用生成的 client」完整跑一次，量到足夠的證據，決定是否以它取代 OpenAPI spec-first（ADR `2026-09-24-go-modular-monolith.md` D7，與 `2026-09-25-openapi-toolchain.md`）。

**狀態：** 通過（Will 2026-09-26）。ADR `2026-09-26-external-contract-protobuf-connect.md` 取代上述兩份；收尾見步驟 8。

## 為什麼要重新評估

- 系統從未上線，四個 client 都在這個 monorepo：v1 的 wire 格式沒有外部使用者，「同一版本不改契約」這條公司時期的規則在上線前不適用。D7 在這個前提下只比較了「proto 加 grpc-gateway」，沒有評估「以 protobuf 當對外 IDL」。
- OpenAPI 方案選定後仍留下三個代價：Dart 的 DTO 不是 freezed；openapi-typescript 以 peer dependency 使用專案的 TypeScript，在 website 的 TS 7 下 crash；SSE 與 multipart 各端都要自己處理。
- Protobuf＋Connect 的預期解法（本計畫要驗證的就是這些）：
  - Dart：訊息類別由官方 protobuf 插件生成，`freeze()`／`rebuild()` 涵蓋不可變與 copyWith；client 用 connectrpc（pub.dev，2.0.0）。
  - TS：protoc-gen-es 的 `@bufbuild/protoplugin` 自帶 `typescript@5.4.5` 為直接依賴，`target=ts` 不需要轉譯，不碰專案的 TS 7。
  - 串流改成 server streaming RPC；檔案改成 presigned URL 直接上傳 R2，API 只處理 metadata。
  - 另外得到 `buf breaking` 的破壞性變更偵測，以及 proto3 JSON 對應（camelCase、int64 轉字串、RFC 3339 時間）直接滿足現行命名與型別規則。

### 必須正面回應的反論點

2026-09-25 以四個獨立 agent 重做 D7 的決策（兩個用舊的 ADR 規則、兩個用新的），四個都評估了 Protobuf＋Connect，也都選回 OpenAPI。它們提出的理由裡，以下幾項是真的，驗證必須逐一給出答案：

1. **D8 的第一層安全網**：HTTP 特性測試要「同一套情境先對舊實作、再對新實作跑」，改成 Connect 後兩邊的 wire 不同，同一份測試跑不了。本計畫的答案是把情境寫成透過 driver 介面操作：情境只寫一次，舊 gateway 用 REST driver、單體用 Connect driver（見步驟 4 與通過條件 8）。如果這個做法在 content 上成本過高，或保不住安全網的鑑別力，就是不通過的理由。
2. **公開讀取的 CDN 快取**：Connect 的 GET 把整個 request 編進 query 參數，快取鍵、`Cache-Control` 與 ETag／304 要另外處理。本機驗證 header 與條件請求的行為；Cloudflare 上的行為列在延後的驗證。
3. **第二套契約**：webhook、表單回呼、Range／206、上傳、SSE 若都在 Connect 之外，就是兩套契約並存。本計畫的答案：上傳改 presigned URL，媒體的 Range 由 R2 或 CDN 直接提供，SSE 改 server streaming；剩下的只有 provider 決定格式的 webhook 與回呼，這一類在 OpenAPI 方案下同樣是手寫 handler，不因 Connect 而增加。
4. **錯誤處理要和遷移同時重寫**：envelope 與 CodeMeta 錯誤碼改成 Connect 的 error details，四端的錯誤處理都要改（見步驟 3 與通過條件 4）。

## 範圍

**要做：**
- content 模組的全部對外 API：公開的 banner、食譜的隨機推薦、詳情與搜尋建議、App 版本檢查；admin 的 banner、食譜、版本門檻的維護。現行 gateway 上有 20 支 operation；站點文案、前台食譜的 popular／recent 排序與瀏覽數沒有讀取端，拿掉（Will 2026-09-25），Connect 上是前台 5 支、後台 13 支。
- 四個 client 各自把 content 的呼叫全部換成生成的 client：
  - website：公開頁的 SSR 與搜尋補全
  - admin：後台 CRUD
  - buyer-app：banner、食譜、版本檢查
  - seller-app：版本檢查
- 契約工具鏈：buf 的 lint、breaking、generate，各消費端的生成物 drift 檢查。
- 一支串流的傳輸驗證（content 沒有串流，見步驟 5）。

**不做：**
- CD 與 Cloudflare 上的實測：部署環境要在第 4、5 步用個人帳號重建，這裡只驗 HTTP 語意，邊緣行為列為 CD 驗收條件（見「延後的驗證」）。
- identity 搬遷：認證沿用 gateway，見「過渡期架構」。
- 其他模組的 API。

## 過渡期架構

> 2026-09-26：試點通過後，遷移期間的架構改以 ADR `2026-09-26-external-contract-protobuf-connect.md` D6 為準（gateway 當過渡入口、隨模組搬遷縮小，identity 搬進單體時入口移到單體）。以下是驗證期間的做法。

- gateway 繼續當入口：認證（cookie session 與 Bearer）、CSRF、CORS、限流不變。新增一組反向代理路由，把 Connect 的路徑轉給單體：`/hawkr.content.v1alpha1.ContentPublicService/*` 走公開或選擇性認證，`/hawkr.content.v1alpha1.ContentAdminService/*` 走 admin 認證。身分以 gateway 既有的 `X-Account-Id`、`X-Login-Role` 傳入。
- 單體只信任經由 gateway 進來的身分 header，驗證期間不直接對外。
- 依使用者分成不同 service（Public／App／Admin），授權策略跟著 service 走，對應現行的 `public`、`app`、`admin` 三個 namespace。
- identity 模組搬進單體後，認證移進單體，gateway 退場；這不在本計畫。

## 步驟

- [x] **1. 前置條件**（2026-09-25）
  - [x] ~~本機要跑 gateway 與 account-service，需要 `backend/.env`~~：不需要。經 gateway 的端到端測試以 `scripts/characterize-legacy-content.sh` 起 gateway、舊的 content-service 與單體，Bearer token 以腳本產生的本機 `JWT_SECRET` 簽，不經 account-service，也不讀 `.env`（步驟 4）。
  - [x] content 模組依遷移計畫 A1 搬進單體（schema、migration、repository、use case、測試）。這部分不論驗證結論都保留，所以與契約無關的做法照 A1 與模組完成的定義。2026-09-25 完成，見遷移計畫 A1；公開 API 只用 Go 型別（`time.Time`、`*int32`、結構化的 banner 點擊行為），字串與數字的解析留給傳輸層。
- [x] **2. 契約的位置與工具鏈**（2026-09-25）
  - proto 放在 monorepo 根目錄 `api/`（四個專案共用的契約，根目錄 AGENTS.md 的跨專案契約表同步改），例如 `api/hawkr/content/v1alpha1/content.proto`；與 `backend/api/proto`（內部 gRPC，階段 C 刪除）分開。
  - `buf.yaml` 與 `buf.gen.yaml`：lint 用 STANDARD 規則加上本專案的例外，`buf breaking` 對 `main` 比對。
  - 生成：Go（protoc-gen-go、protoc-gen-connect-go）進 `backend/internal/httpapi`；TS（protoc-gen-es，`target=ts`）進 website 與 admin；Dart（官方 protoc_plugin 與 connect 的 Dart 插件）進兩支 App 共用的套件或各自目錄。插件一律在本機固定版本執行，確認 Dart 的 connect 插件能在本機跑，不依賴 BSR 遠端插件。
  - 各消費端的 CI 加生成物 drift 檢查。
  - 做法：
    - 第一份 proto 是步驟 5 要用的 `hawkr.pilot.v1alpha1`（server streaming 加 heartbeat），content 的 proto 等步驟 3 的慣例定了再寫。`buf.yaml` 目前是 STANDARD，沒有例外。
    - 版本：buf CLI 固定在 `api/.buf-version`（1.50.0）。外掛鎖在各消費端自己的 lockfile，以 `local:` 執行：Go 是 `backend/go.mod` 的 tool（protoc-gen-go 1.36.11、protoc-gen-connect-go 1.21.0）；TS 是 website、admin 的 `package.json`（protoc-gen-es 與 `@bufbuild/protobuf` 2.15.0，精確版本）；Dart 是 `dart run`，版本在共用套件的 `pubspec.yaml`（protoc_plugin 25.1.0、protobuf 6.1.0、connectrpc 2.0.0）。
    - Dart 生成物放在共用套件 `packages/hawkr_api`（Will 2026-09-25 決定），兩支 App 在步驟 6 以 path 依賴；步驟 2 還沒有改 App。
    - 生成物：`backend/internal/httpapi/gen`、`website/lib/gen`、`admin/src/lib/gen`、`packages/hawkr_api/lib/gen`。檢查 buf 版本後生成的流程共用 `api/generate.sh`，清空輸出由各 `buf.gen.yaml` 的 `clean: true` 負責；Go 用 buf 的 managed mode 設 `go_package`，proto 裡不寫語言相關的選項。
    - CI：`api-contract.yml` 跑 `buf lint`、`buf breaking`（PR 對 base、push 對推上來之前的 main）與 Dart 套件的 drift 和 `dart analyze`；`backend-monolith`、`website CI`、`admin CI` 各自加上生成物 drift，觸發路徑加上 `api/**`。
    - 驗證：四端生成兩次都逐位元相同；手改生成物後重新生成都會覆寫（CI 的 `git status` 會抓到）；website 在 TS 7 下 lint、型別檢查、測試與 build 通過，admin 在 TS 5.9 下 lint、測試與 build 通過；`dart analyze` 無問題。
    - website 的 lockfile 除了新套件，還多了兩個與這次無關的變動：`styled-jsx` 不再解析 optional peer `@babel/core`、`third-party-web` 0.29.2 → 0.30.0。這是 HEAD 的 lockfile 與 pnpm 11.2.2 之間既有的差異，任何依賴變更都會觸發（只加一個沒有依賴的套件也一樣，已驗證）。
    - **上線前的 breaking 政策**（Will 2026-09-25 決定）：`buf breaking` 原本會擋住刻意的破壞性變更（包括步驟 8 刪掉 pilot），與「上線前不受相容性約束」衝突。上線前的 package 一律用 `v1alpha1`（Google AIP-185 的 alpha），`api/buf.yaml` 設 `ignore_unstable_packages`；上線時一次改名成 `v1`，之後 breaking 才生效。沒選「上線前不跑 breaking」：上線時才第一次接上檢查，要到那時才發現它是否有效；也沒選「保留 v1、逐次放行」：只有一條 main、直接 push，沒有 label 可用。
      - CI 的 `buf breaking` 加 `--exclude-imports`：WKT 這類 import 不再被引用時會從 image 消失，不是我們的契約。
      - 驗證（2026-09-25，以目錄當基準）：`v1alpha1` 改型別、刪掉整個 pilot package 都放行；另建一個 `v1` 的 fixture 改型別會擋下（exit 100）。通過條件 6 的 breaking 部分，上線前以這個 `v1` fixture 驗證檢查本身有效；`v1alpha1` 的破壞性變更刻意不擋。
- [x] **3. API 設計慣例（寫成規範草稿，驗證通過後取代 API 設計規範的 HTTP 部分）**（2026-09-25）
  - 做法：
    - 草稿：`docs/開發規範/API 設計規範（Connect）.md`；契約：`api/hawkr/content/v1alpha1/content.proto`（前台）、`api/hawkr/admin/content/v1alpha1/content.proto`（後台）、`api/hawkr/common/v1alpha1/error.proto`（`ErrorInfo`）。
    - 後台獨立成 `hawkr.admin.*`：同名資源在前後台的欄位不同；website 與兩支 App 的 `buf.gen.yaml` 以 `exclude_paths` 不生成後台 API。授權依 service 名稱（`*PublicService`、`*AppService`、`hawkr.admin.*` 的 `*AdminService`），gateway 與單體各檢查一次。
    - 方法照 buf STANDARD：每支 RPC 有自己的 Request／Response，`Get`、`Create`、`Update` 回 `{資源}`，與 AIP 回資源本身不同。集合讀取分成 `List`（分頁）、`Suggest`（前 N 筆）、`Sample`（隨機）。
    - 分頁是 keyset 的不透明 token（含請求參數的指紋）；部分更新用 `update_mask`（AIP-134，沒帶時是非零值的欄位）；以 key 識別的資源 `Create` 與 `Update` 分開（ADR D3）。
    - 前台食譜只有 `SampleRecipes`（隨機）、`SuggestRecipes`（搜尋建議）與 `GetRecipe`；站點文案、瀏覽數、popular／recent 排序、分類篩選都沒有讀取端，拿掉（Will 2026-09-25；分類篩選是設計審查後拿掉的）。
    - 設計審查（`design-review`，2026-09-25）後改掉的：banner 的點擊行為從 JSONB 改成欄位加 CHECK、url 只收 http、https（設計書 25）、`store_url` 不再用 NULL、單體只生成通用錯誤碼、trace 改成 W3C `traceparent`。待 Will 決定的見步驟 4 最後一項。
  - 方法命名與分頁依 Google AIP：`Get`／`List`／`Create`／`Update`／`Delete`，分頁用 `page_size`、`page_token`，部分更新用 field mask。現行的 `records`、`page`、`pageSize` 不沿用。
  - 版本：上線前的 package 一律是 `v1alpha1`，上線時一次改成 `v1`（見步驟 2 的 breaking 政策）。
  - breaking 的類別是 `WIRE_JSON`（Will 2026-09-25 決定）：上線後要保護的是已發布的 App 與 web bundle 的 wire／JSON 相容，Buf 文件建議 Connect 用它；沒選沿用內部 gRPC 的 `FILE`：它保護的是「拿生成碼的 client 不重新生成」，而四個消費端都在 monorepo、生成物由 drift CI 在同一個 commit 更新。`WIRE_JSON` 不擋 message 改名，而 error detail 的 type URL 含 message 全名，`ErrorInfo` 這類 detail 訊息不得改名。驗證（`v1` fixture）：改型別、刪欄位、欄位改名都擋下，message 改名放行。
  - 並發修改用 etag（樂觀鎖；Will 2026-09-25 決定）：規則與理由在 ADR `2026-09-25-optimistic-concurrency-etag.md`，與協定無關，試點不論結論都適用。Connect 的對應：資源訊息帶 `string etag`；Update 必須帶，沒帶回 `INVALID_ARGUMENT`，不符回 `ABORTED` 加上通用的版本衝突業務碼；以 key 建立或修改的資源，沒帶 etag 表示只建立，已經存在回 `ALREADY_EXISTS`、業務碼相同（ADR D3 修訂，Will 2026-09-25）。資源的 etag 與公開 GET 的 HTTP 快取 `ETag`（通過條件 9）是兩件事。
  - 錯誤：Connect 的標準錯誤碼；需要區分業務情境時 details 加 AIP-193 形狀的 `hawkr.common.v1alpha1.ErrorInfo`，`reason` 是 CodeMeta 錯誤碼的 Name（Will 2026-09-25 決定，取代原本數字碼的 `BusinessError`）。traceId 放在 response header。前端依 CodeMeta 生成的常數對應。（2026-09-25 由 ADR `2026-09-25-api-error-model.md` 取代：改用 `google.rpc` 的標準 details，reason 來自 proto 的 `ErrorReason`，使用者文字走 `LocalizedMessage`）
  - 讀取方法標 `idempotency_level = NO_SIDE_EFFECTS`，client 以 GET 呼叫，回應帶 Cache-Control。
    - 現行的食譜詳情在讀取時順便加瀏覽數，與 NO_SIDE_EFFECTS 和 CDN 快取都衝突（命中快取的瀏覽不會被計入）。瀏覽數沒有讀取端，整個拿掉（Will 2026-09-25）；讀取沒有副作用。
  - 需要區分「沒值」與零值的欄位用 `optional`。
  - ID 是 uuid 格式的字串，各端當成不透明字串，不解析成數字（ADR `2026-09-25-row-identity-by-data-kind.md` D5）。
  - 圖片欄位只放 object key，顯示 URL 的組法沿用多媒體與圖片 URL 規範。（2026-09-25 由 ADR `2026-09-25-api-image-uri-template.md` 取代：回伺服器組好的 `Image.uri_template`）
  - wire 格式：web 用 JSON（除錯方便）；App 用 JSON 或 binary 由步驟 6 的量測決定。
- [x] **4. server 端**（2026-09-25）
  - connect-go 的 handler 掛在 `internal/httpapi` 的 mux 上，呼叫 content 模組的公開 API。
  - `internal/legacy` 加上呼叫 Java media-service 的 `media.Media` 實作（`FinalizeImages`、`LinkImages`、`ReleaseReferences`），在 `cmd/hawkr` 接線；gRPC stub 用步驟 2 的 buf 工具鏈從 `api/proto/media` 生成到 `internal/legacy`，單體不依賴舊的 `go-pkg` module。`FinalizeImages` 需要 `x-account-id`、`x-account-type`（CodeMeta AccountType 的 Value），由 `media.Uploader` 帶入。
  - interceptor：從 gateway 的 header 取身分、統一錯誤轉換（模組錯誤 → Connect 錯誤加業務碼 detail）、deadline、log 與 trace。
  - request 大小上限（connect-go 的 `WithReadMaxBytes`）：模組的文字欄位是 `text`、不設長度上限（只有產品規格訂的上限另外檢查），大小由這裡擋。
  - gateway 加上步驟「過渡期架構」的代理路由，並確認 CSRF guard 放行 Connect 的 POST（Origin 在允許清單、需要的話由 client 帶 `X-Requested-With`）。
  - 測試：每支 RPC 的 handler 測試，加上經 gateway 代理的端到端測試。
  - content 的 etag（ADR `2026-09-25-optimistic-concurrency-etag.md`）：banner、食譜、版本門檻的表加版本欄，版本門檻照 D3 拆成只建立與帶版本的修改；在轉正圖片之前先以讀到的版本比對一次，最後以條件式 UPDATE 為準；回應的 etag 取自寫入時的 `RETURNING`，不要 commit 之後另外讀（可能已是別人的版本）；integration 測試涵蓋兩個人同時修改與同時建立同一個 key。沿用盤點與「已知限制」照實作更新。
  - D8 的特性測試：content 的情境（公開列表與詳情、搜尋補全、版本門檻、admin 的新增、修改、刪除、未登入與權限不足）寫成透過 driver 介面操作，REST driver 打舊 gateway、Connect driver 打單體，兩邊跑同一份情境。etag 由 Connect driver 在內部取得（先讀再寫），情境本身不帶；「並發修改被拒」只存在於單體，列為刻意的差異，只對單體跑。
  - 做法與結果：
    - 模組：拿掉站點文案、瀏覽數與沒有讀取端的排序、篩選；`version` 欄與條件式 UPDATE，回應取自 `RETURNING`；banner 的點擊行為改成欄位加 CHECK。migration `drop_unread_features`、`etag_versions`、`banner_action_columns`。沿用盤點（含新增的「傳輸層的沿用盤點」）在模組的 `CARRYOVER.md`。
    - `internal/httpapi`：`rpc`（依 service 名稱授權、`apperr` → Connect 錯誤與 `ErrorInfo`、請求的預設與上限 deadline、log、page token、field mask）、`content`（前台與後台的 handler）、`WithReadMaxBytes` 2 MiB、公開 GET 的 `public, no-cache` 加弱 ETag 與 304、其他 `private, no-store`、`otelhttp` 接續 gateway 的 `traceparent`。CodeMeta 新增通用的版本衝突碼 `VERSION_CONFLICT`（412），單體由 `generate.sh --go` 生成錯誤原因的常數（通用碼與 `GO_MONOLITH_BIZ_ERRORS` 列的）。
    - `internal/legacy/mediaservice`：以 connect-go 的 gRPC 協定（h2c）呼叫 Java media-service，stub 由 `buf.gen.legacy.yaml` 從 `api/proto/media` 生成，簽章照設計書 52；`cmd/hawkr serve` 接線（`MEDIA_SERVICE_ADDR`）。
    - gateway：`routes_connect.go` 依 service 名稱掛 Public（不認證、刪掉 client 帶的身分 header）與 Admin（`JWTAuth`＋`requireAdmin`）的代理；gateway 自己寫出的錯誤在 Connect 路徑上改成 Connect 格式（`middleware.ConnectErrors`）；CORS 加上 Connect 的 header、expose `X-Trace-Id`；公開 service 的限流與 `/api/v1/public` 同級。CSRF 不另開例外：帶 cookie 的 POST 仍要 Origin 在允許清單加 `X-Requested-With`（client 在步驟 6 帶）。
    - 測試：模組與 handler 的 integration 測試（兩人同版本同時修改、同時建立同一組門檻、比對後被搶先寫入）；gateway 的 router 測試（身分 header、認證、CSRF、預檢、單體連不上、請求太大、代理清單與 `api/` 一致）。
    - D8：情境在 `internal/httpapi/content/characterization`。CI 以 Connect driver 直接打單體，並以三種突變（食譜少一項食材、前台少一則 banner、版本檢查被改成不必更新）確認情境會失敗。`scripts/characterize-legacy-content.sh` 在本機起 gateway、舊的 content-service 與單體（只用 infra 的 Postgres、Redis 與測試自己的假 media-service，`env -i` 啟動、不讀 `.env`），同一份情境以 REST 打舊的、以 Connect 經同一個 gateway 打單體：2026-09-25 兩邊 8 個情境全過，單體另外通過並發修改的情境。
  - [x] 設計審查交給 Will 的四項（2026-09-25 決定）：錯誤 detail 改成 AIP-193 形狀的 `ErrorInfo`；以 key 建立已存在時回 `ALREADY_EXISTS`（ADR D3 修訂）；CodeMeta 生成器改用 `gofmt`、上線前把各模組的 migration 壓成一個 baseline，兩項列進遷移計畫。
- [x] **5. 串流的傳輸驗證**（2026-09-25）
  - content 沒有串流。以一支只存在於本計畫、在 `hawkr.pilot.v1alpha1` 底下的 server streaming RPC（定時送出遞增序號，含 heartbeat），驗證各 client 的串流在本機的行為；驗完刪除，列在步驟 8 的清理。
  - 檢查：website 的瀏覽器端（Playwright）能逐筆收到、能中止；App 的 http2 client 在 iOS 模擬器上能逐筆收到，App 退到背景再回前景時的重連行為；經 gateway 代理時沒有被緩衝成一次送出。App 以暫時的宿主 App 驗、只驗 iOS（Will 2026-09-25；原本是兩支 App 在 iOS 與 Android 模擬器上，理由見下方做法）。
  - AI 助理的串流用 AG-UI 事件格式，是外部的開放協定：是否改成 proto 串流，另列為決策項，不在本計畫決定。
  - 做法與結果：
    - 契約：service 改名為 `PilotPublicService`（命名規則的 Public，不需要登入）；`Count` 立刻送出序號 1、之後每個 interval 一個，中間送 heartbeat；`limit` 1–1000、`interval` 50 毫秒–10 秒、整個串流最多 10 分鐘（串流不受 interceptor 的逾時限制，長度由 handler 擋）。
    - 單體：`internal/httpapi/pilot`，掛進 `connectServices`；gateway 的 `connectServices` 加上它，兩邊的契約測試不再排除 `hawkr.pilot`。
    - 測試：單體的 `internal/httpapi/pilot_test.go` 經整條 handler（tracing、快取規則、interceptor）確認逐筆送出（兩個序號至少相隔 2/3 個 interval）、client 中止後 handler 以 canceled 結束、參數超出範圍回 `INVALID_ARGUMENT`；把 POST 的回應也緩衝起來的突變會失敗。gateway 的 `TestConnectStreamIsNotBuffered` 以假的單體逐筆等待確認，中介層加上會包住 writer 的 `Logger` 等（與 `newEngine` 相同的順序）；讓 `Logger` 的 writer 吞掉 `Flush` 的突變會在 3 秒內失敗。
    - gateway 的 `Logger` 不再把 Connect 串流（`application/connect+json`、`+proto`）的內容擷取進 log，與 SSE 相同。
    - 經真的 gateway：`scripts/verify-connect-streaming.sh` 起單體與 TLS 的 gateway（這次產生的 CA），HTTP/2 與 HTTP/1.1 各跑一次。2026-09-25：兩種連線都逐筆送出，client 中止後單體 50 毫秒內記下 canceled。`--keep` 讓服務繼續跑，給 App 的檢查用。
    - 瀏覽器：`website/tests/connect-pilot`（vite 服務一個只載入 connect-web client 的頁面，Playwright 用系統的 Chrome 打本機的 TLS gateway；`verify-connect-streaming.sh --web`）。website 新增 `@connectrpc/connect`、`@connectrpc/connect-web` 2.2.0（Will 2026-09-25 同意下載）。2026-09-25：走 HTTP/2、CORS 預檢通過，逐筆收到（序號間隔約 500 毫秒），`AbortController` 中止後迴圈以 `Canceled` 結束、單體記下 canceled。
    - App：兩支 App 在本機 build 不起來（iOS 的 build 會複製舊公司 Firebase 的 `GoogleService-Info.plist`，檔案不在 repo，AppDelegate 啟動就呼叫 `FirebaseApp.configure()`；seller 的 Android 套了 `google-services` plugin；另外接了舊的 Sentry 與 Google Sign-In）。改用暫時的宿主 App `packages/pilot_streaming_check`（Will 2026-09-25 決定），驗的是傳輸本身：connectrpc 的 http2 client（`Http2ClientTransport`，只信任這次的 CA）、JSON。只進版控 Dart 原始碼與腳本，iOS 的專案殼由 `run.sh` 在暫存目錄以 `flutter create` 產生；App 經 `run.sh` 起的 TCP relay 連 gateway，停掉 relay 就是切斷連線。兩支 App 在本機跑不起來的問題另列待辦（App CD）。
    - iOS（2026-09-25，iOS 26.0 模擬器，debug build，跑兩次結果一致）：
      - 前景：逐筆收到，序號間隔 466–535 毫秒。
      - 背景 15 秒：`paused` 後 0.6–0.8 秒 App 被暫停（之後沒有任何事件），TCP 連線撐過 10–13 秒的暫停；醒來後 70–180 毫秒內補上 kernel 緩衝的序號，串流完整結束、不漏不亂序。
      - 背景時切斷連線：暫停中收不到連線關閉，醒來後 110–115 毫秒串流以錯誤結束；同一個 transport 第一次重試就重新連上。
      - connectrpc 的 Dart http2 client 把連線中斷回成 `Code.unknown`（`HTTP/2 error: Connection error ... errorCode: 10`，即 CONNECT_ERROR），不是 `unavailable`：步驟 6 的重試判斷不能只看 `unavailable`。
      - 沒有驗到的：沒有 FIN 的斷線（例如離開 Wi-Fi）。`Http2ClientTransport` 預設不送 ping（`pingInterval` 為 null），這種半開的連線要等 TCP 逾時才會發現；步驟 6 建 App 的 transport 時設 `pingInterval`。
    - Android 不驗（Will 2026-09-25）。
    - 之後的串流 RPC 要處理：gateway 的 Connect 代理與單體都沒有逐次寫入的 deadline（gateway 的 SSE 有），client 不再讀取但連線還在時，串流佔著連線直到 handler 自己結束。pilot 以 10 分鐘上限擋住；正式的串流 RPC 與 AI 助理串流的決策一起處理。
- [x] **5b. 落實 2026-09-25 的三份 ADR（在步驟 6 之前，client 只需要改一次）**
  - ADR：`2026-09-25-api-image-uri-template.md`（圖片）、`2026-09-25-codemeta-retirement.md`（CodeMeta 退場）、`2026-09-25-api-error-model.md`（錯誤模型）。Will 2026-09-25 同意；錯誤模型的 `ErrorInfo` 只在 client 需要分支時帶（Will 選擇），googleapis 從 BSR 下載（Will 同意）。
  - [x] 契約：`common/v1alpha1` 新增 `Image`、`ErrorReason`（值名不加前綴，`buf.yaml` 只對該檔排除 `ENUM_VALUE_PREFIX`），刪除自訂的 `ErrorInfo`；content 的圖片欄位改成 `Image`，`app_type`、`platform` 改成 `AppType`、`AppPlatform`；四端重新生成。TS、Dart 的 `google/rpc/error_details.proto` 由各自的 `buf.gen.yaml` 從 BSR 生成（`api/` 的 proto 沒有 import 它，不進 `api/buf.yaml`；三處版本由 API contract CI 檢查相同），Go 用 genproto 的 `errdetails`。
  - [x] 單體：`internal/httpapi` 組 `Image`（`MEDIA_ORIGIN`）；寫入只帶新圖的 staged token，沿用的 key 由模組取自現有資料。content 的 `AppType`、`AppPlatform` 改成模組的 Go 型別，值＝proto 值名去前綴，轉換用 `rpc.EnumFromProto`／`EnumToProto`，CHECK 與 Go 的整合測試、proto 與 Go 的雙向測試（`rpc.EnumMismatches`）。`apperr` 加欄位違規（模組自己的欄位名，handler 以 `rpc.WithFieldPaths` 對應到請求路徑）；metadata 等第一個需要動態值的 reason 出現才加。`apperr.Reason` 只能在 apperr 宣告，與 proto 的 `ErrorReason` 雙向比對。`rpc.toConnect` 只轉換自己建立的錯誤、下游錯誤變 `internal`，下游的訊息只進 log；details 改成 `ErrorInfo`＋`LocalizedMessage`＋`BadRequest`；interceptor 之前的錯誤由 `rpc.LogUnhandled` 補 log。`VERSION_CONFLICT` 移到 `ErrorReason`，刪除 `internal/platform/codemeta` 的錯誤原因生成與 CodeMeta `錯誤碼.md` 的通用 `VERSION_CONFLICT`（412）。
  - [x] 欄位行為改用 AIP-203 的 `google.api.field_behavior`（design-review 提出，Will 2026-09-26 決定；不用 protovalidate）：`api/buf.yaml` 加 googleapis 的 deps（`buf.lock`），`rpc.ApplyUpdateMask` 依標註略過 `OUTPUT_ONLY`、`IDENTIFIER` 並擋下 `IMMUTABLE` 的修改，拿掉手寫的名單；`TestRequiredFieldsAreEnforced` 從 descriptor 確認每個 `REQUIRED` 欄位模組都會擋。TS 以 `include_imports` 生成 `google/api/field_behavior`，Dart 的生成物不引用它，Go 用 genproto 的 `annotations`。
  - [x] gateway：Connect 路徑的錯誤改用 connect-go 的 `ErrorWriter`，帶 `LocalizedMessage`，限流加 `RetryInfo`；unary 與串流各自測試，並以突變確認。攔截式轉換的代價記在 content 的傳輸層沿用盤點。
  - [x] CodeMeta 過渡第 0 步：`--web`／`--admin`／`--flutter` 直接寫進 monorepo 的四個檔（Flutter 檔頭 `// dart format off`，與生成器輸出逐位元相同），backend monolith 的 drift 檢查涵蓋它們；刪除 `check-copies.sh`、`_repo-paths.sh`、`test-check-copies.sh`、`scripts/output/`、lefthook 的 `codemeta-copies`、四支 pin digest 的 sync test（monorepo 裡與 CI drift 重複）與已棄用的 `website/scripts/codemeta_to_ts.py`。漂移對帳：client 拷貝比 Markdown 多出的 `ShippingStatus` 994、`OrderCancelReason` 9105、`ShipmentAnomalyType.UBER_CANCEL_RECONCILIATION_REQUIRED`、整個 `DeliveryMethodUnavailableReason`，以及成員層級比對才找到的 `FulfillmentAction.REPORT_READY`，都來自設計書 65、66 的 Uber 即時配送（公司時期未合併的分支），沒有手寫程式使用，重新生成時從拷貝移除；9105 在 65 還有撞號未決。Uber 配送搬進單體時，這些值依 ADR 進 proto 或模組。macOS 的 awk 與 Linux 的 mawk 生成結果逐位元相同。同類的 AG-UI fixture 拷貝也改成 website 直接讀 gateway 的 golden fixture。
- [x] **6. 四個 client 換成生成的 client**（2026-09-26）
  - 進度（依序做，每項一個 commit；2026-09-26 盤點四端的呼叫點與風險後排定）：
    - [x] 6.1 錯誤分派的共用情境表（`api/testdata/error-dispatch.json`）與 Dart 的分類器、重試 interceptor（`packages/hawkr_api`）。
    - [x] 6.2 preset 常數由 `backend/workers/image-gateway/src/presets.ts` 生成到 website、admin、Dart 套件，CI 檢查 drift；三端的 `Image` helper（只把 `{preset}` 換掉）。舊 REST 的 `buildImageUrl` 與媒體網域設定留給還沒搬的模組，最後一個模組搬完時刪除。
    - [x] 6.3 website：Connect transport 以 interceptor 補齊現有 client 的保證（server 端的 origin-auth 簽章與 visitorIp、逾時、5xx 上報、`AUTH_EXPIRED_EVENT`；base 是 gateway 根目錄，不是 `/api/v1`，本機代理也要涵蓋）；TS 分類器以共用情境表測試；Query 層拿掉「401 以外都重試一次」並認得 `ConnectError`；`check-transport.ts` 攔下自建的 Connect transport。server 端的重試不排 timer（Workers 的 B-199），只立即重試一次；登入後的 service（`*AppService`）第一個搬進單體時再補 credentials、`X-Active-Role`、`AUTH_EXPIRED_EVENT` 與代行的護欄。
    - [x] 6.4 website 的 content：megamenu 的 banner、隨機食譜（SSR 的 `unstable_cache` 與 tag、tab-bootstrap 的序列化）、搜尋建議、食譜詳情（維持不重試）；拿掉食譜瀏覽的 beacon 與 BFF 路由（瀏覽數已拿掉，gateway 也沒有這條路由）；拿掉手寫的型別與圖片的二次組裝；fallback 圖改成站內的靜態檔。實際做法：`AppImage` 加 `image` 入口（只換 `{preset}`、不經過 `buildImageUrl`）；食譜用生成的 `RecipeCard`／`RecipeSuggestion`／`Recipe`，隨機食譜經 `unstable_cache` 讀 `SampleRecipes`，搜尋建議、詳情與搜尋彈窗的輪播用 connect-query（彈窗的 BFF 拿掉）；詳情大圖改用 `detail` preset（原本下載原圖）；banner 保留畫面用的 union，資料改讀 `ListLiveBanners`。食譜一定有圖（模組要求），沒有圖時用 `AppImage` 既有的空狀態，不再引用 R2 上的 fallback key。e2e 的 mock 改成 Connect 的路徑。
    - [x] 6.5 admin：安裝 connect、connect-web、connect-query；transport 走同源 BFF，BFF 轉送 Connect 的 header（`connect-protocol-version`、`connect-timeout-ms`）並補 CSRF 需要的 `X-Requested-With`；banner、食譜、版本門檻改用 Connect（etag、`update_mask` 含零值、分頁、`Timestamp`、`optional int32`、沿用圖片不送 key、門檻拆成 Create 與 Update）；版本衝突保留輸入並顯示最新版本。
    - [x] 6.6 App：兩支 App 以 path 依賴 `packages/hawkr_api`；transport 是 `Http2ClientTransport` 加 `pingInterval`，base 是 gateway 根目錄；interceptor 補上 Bearer、unauthenticated 時 refresh 一次後重送、`X-App-*` header 與 Sentry；buyer 的 banner、食譜（ID 改成字串）、版本檢查，seller 的版本檢查（維持 3 秒逾時、不帶身分、失敗放行）；hermetic 測試改成 fake transport。實際做法：
      - 套件：`transport.dart`（`apiHttpClient`、`apiTransport` 以 GET 呼叫讀取、`headersInterceptor`、`timeoutInterceptor`、`reportInterceptor`、`authInterceptor`；逾時用自己的可取消計時器，connectrpc 的 `TimeoutSignal` 計時器取消不了，widget 測試會因為留下 timer 失敗）、`testing.dart`（轉出 `FakeTransportBuilder`，App 不直接依賴 connectrpc，版本只固定在套件）、`userMessage`；重試的退避等待中被取消或到 deadline 就結束（原本會等滿退避）。JSON 或 binary 仍留給步驟 7 的量測，預設 JSON。
      - 兩支 App 共用一條 HTTP/2 連線；公開的 service 不帶身分，一般呼叫的逾時照 flavor，版本檢查以 `withTimeout` 設單次呼叫 3 秒的上限（涵蓋重試）；interceptor 由套件的 `appInterceptors` 組，順序只有一處。登入後的 service 用 `connectAuthTransport`：帶 Bearer，unauthenticated 時經 App 既有的 `refreshSession`（單飛，與 Dio 共用）refresh 一次後重送；暫時失敗保留憑證；憑證無效，或剛換到的 token 也被拒絕時登出（seller 的 Dio 在後者只記 log，Connect 兩支 App 一致）。還沒有 App 的 service 搬進單體，通過條件 3 的 App 部分以邊界兩側的測試驗證：gateway 的 `TestConnectAdminRequiresAnAdmin`（沒有登入與 token 過期回 unauthenticated、非管理員回 permission_denied、管理員帶身分轉給單體）與兩支 App 的 `connect_auth_test.dart`（接 App 真的 session provider）。停權導向要等契約有停權的 `ErrorReason` 才能接。
      - 圖片：`AppCachedImage.api(image:, scene:)` 只換 `{preset}`，不經過 `buildImageUrl`（它會把 `/i/{preset}/` 的網址改寫成 App 設定的媒體網域）；四端的 preset 都改用 `presets.ts` 的生成物（App 的 `MediaPreset` 字串常數換成 `ImagePreset` enum）。
      - buyer：banner 讀 `ListLiveBanners`，點擊行為由 oneof 轉換（`BannerAction.fromProto`）；食譜清單讀 `SampleRecipes`，刪掉 B-188 的種子池 fallback；搜尋建議讀 `SuggestRecipes`；詳情讀 `GetRecipe`，錯誤轉成 App 既有的 `AppException` 與文案；刪掉 JSON 的防呆解析與不在契約上的 `productKeyword`、`description`（舊 API 的詳情也沒有 `description`，畫面不變）。呼叫都在 repository（`BannerRepository`、`RecipeRepository`），`tool/architecture_check.dart` 加上 `presentation-direct-connect`：presentation 不能 import Connect 的 client 或 transport，不給例外。
      - seller：版本檢查讀 `CheckAppVersion`，平台直接用 proto 的 `AppPlatform`；它不再經過 Dio，`kSkipAuthInterceptorExtra` 的白名單拿掉版本檢查。
      - 兩支 App 的 CI 在 `packages/hawkr_api/**` 變更時也會跑。
    - [x] 6.7 刪除四端 content 的手寫 wire 型別與呼叫，更新 e2e 的 mock；量測 App 的 JSON 與 binary（步驟 7）。實際做法：四端的 content 手寫型別與 JSON 解析已在 6.4–6.6 隨各自的遷移刪除，e2e 的 mock 在 6.4 改成 Connect 的路徑；這一項刪除 admin 的文案頁（`site-contents`，content 模組已拿掉站點文案，沿用盤點寫明在步驟 6 刪除）與側欄入口，修正仍寫著舊路徑的註解。量測見〈驗證結果〉的「App 的編碼」：App 維持 JSON，改成接受 gzip。
  - website：
    - transport 以 connect-es 的 `createConnectTransport` 建立，沿用既有 client 的行為：公開呼叫不帶 cookie、登入後的呼叫帶 cookie、角色 header、401 事件、錯誤轉換。
    - SSR 與 Next.js 快取：要確認 `revalidate` 與 tags 能透過 Connect 的呼叫生效（預期做法是 context value 加自訂 fetch）；做不到的話列為不通過的原因。
    - 讀取以 connect-query（TanStack Query 整合）呼叫。
    - `check-transport.ts` 的「只走一個 transport」規則同步更新。
    - 以 TS 7 型別檢查、建置、跑既有測試。
  - admin：同 website，但只有 CSR 的後台 CRUD；TS 5.9。修改收到版本衝突時，保留使用者的輸入，並顯示最新版本讓使用者對照後再送出（Will 2026-09-25 決定；不直接重新載入，以免丟掉填好的表單）。重新送出時帶最新版本的 etag；新圖沿用同一個 staged token（契約沒有 key，圖片 ADR D5；media 的轉正可以重入，預檢與條件式 UPDATE 之間被搶先時已轉正的 token 回傳同一個資產）。
  - buyer-app、seller-app：
    - 以 connectrpc 建 client。transport 用 `Http2ClientTransport` 並設 `pingInterval`；連線中斷回的是 `Code.unknown`，重試判斷要涵蓋它（步驟 5 的 iOS 結果）。
    - 現有 Dio interceptor 的行為改寫成 Connect 的 interceptor：token 刷新與 401 重試在 6.6 完成；停權導向等契約有停權的 `ErrorReason` 再接。其他模組仍走 Dio，驗證期間兩套 HTTP 並存。
    - 量測 JSON 與 binary 的差異。
    - buyer-app 的 `recipe_model.dart` 把食譜 ID 解析成 `int`，改成字串。
  - 刪除各端 content 相關的手寫 wire 型別與呼叫。
  - client 的沿用盤點（design-review 2026-09-26 審查 `be5a70786..5d60aadb8` 的 18 項發現中，列在這裡的是決定沿用的與駁回的；改掉的在 commit 裡）。交給 Will 的 9 項，Will 2026-09-26 全部照建議：後台清單改分頁 UI；banner 排程以台北時間解讀、改成 [開始, 結束)（模組的 ListLiveBanners 一起改）；admin 拿掉重複的伺服器規則；兩支 App 的 `AppVersionGateResult` 改用生成的 `CheckAppVersionResponse`、`RagRecipe` 保留（合併三種來源）；後端的使用者文字不帶 ID，buyer 改顯示伺服器的文字；啟用中的 banner 最多 20 則、寫進 proto；TS 共用套件與通過條件 3、7 見下表與〈驗證結果〉：

    | 機制 | 當初為什麼必要 | 今天是否成立 | 業界做法 | 決定 |
    | --- | --- | --- | --- | --- |
    | website 的 Connect base URL 由 REST 的 base 去掉 `/api/v1` 推出 | 兩者指向同一個 gateway，只設一個環境變數 | 成立到最後一個模組搬完：REST base 刪除時依賴方向就反了 | 設定 gateway 的 origin，REST base 由它推出 | 沿用；REST 的 base 刪除時改成設定 gateway origin |
    | 版本檢查兩支 App 各一份，失敗的契約不同（buyer 丟錯由 provider 放行；seller 回 null） | 兩支 App 的 force-update provider 原本就不同（seller 以 null 區分「失敗不更新節流錨點」） | 成立：改的是 provider 的契約，不在試點範圍 | 一份共用的檢查 | 沿用；任一支的 force-update provider 下次改動時合成一份放進 `packages/hawkr_api` |
    | website 與 admin 的 Connect 錯誤分類器、重試逐字重複（Dart 已集中在 `packages/hawkr_api`） | 兩個 app 各自獨立，過去是分開的 repo | 成立到第二個模組搬遷：content 只有這一組 | pnpm workspace 的共用套件（Turborepo 的 Internal Packages 加 Next.js 的 `transpilePackages`） | 沿用（Will 2026-09-26）；下一個模組搬進單體、兩端要改用 Connect 時做成共用套件 |
    | buyer 的食譜步驟過濾空字串、banner 的圖片以 `has…()` 判斷存在 | 鬆散 JSON 的防呆 | 成立：模組只檢查步驟的數量不檢查每一步是否空白（`recipe.go`）；公開契約的 message 欄位沒有表達一定存在 | — | 保留（審查建議刪除，駁回）；標題的 fallback（「最新消息」「未命名」）已刪除 |
- [x] **7. 量測與比較**（2026-09-26；結果在〈驗證結果〉的「步驟 7」。App 大小未量：iOS build 缺舊公司 Firebase 的 plist，以 Dart 的 AOT 程式大小代替）
  - 生成：時間、產物大小、生成兩次是否逐位元相同；規格多一個 RPC 時改動幾個檔。
  - 建置：website 與 admin 的 bundle 增量、建置時間；App 的 build_runner 或生成時間、App 大小增量。
  - 開發體驗：新增一支 RPC 從 proto 到四端可用要改哪些檔；錯誤訊息與除錯（`buf curl`、瀏覽器 devtools 看得懂的程度）。
  - 與 OpenAPI 方案（0.6 的評估數字）逐項對照。
  - App 改用原生 Swift 時的工具鏈（Will 2026-09-26 考慮中，另案決策）：兩條路各比一次，讓結論不綁 Dart。
    - Connect：`swift-protobuf`（Apple）加 `connect-swift`（Buf）；確認 server streaming、`ErrorInfo`／`LocalizedMessage` 的取出、連線中斷時回的 code、以 GET 呼叫讀取。
    - OpenAPI：`swift-openapi-generator`（Apple）；確認生成型別、錯誤回應的型別化，以及串流（SSE）要手寫的範圍。
    - 以 content 的一支 unary 與步驟 5 的串流 RPC 在一個最小的 Swift package 實測，不改現有兩支 Flutter App；通過條件 3、7 的 App 驗證仍以 Flutter 為準，App 改寫定案後在 Swift 上重驗。
- [x] **8. 決策與收尾**（2026-09-26）
  - 依下方的通過條件寫驗證結果。（2026-09-26 寫完，見〈驗證結果〉的「步驟 8」；Will 判定通過）
  - 通過：新增 ADR 取代 D7 與 `2026-09-25-openapi-toolchain.md`（兩份標 superseded）；改寫遷移計畫（A0 重新定義、階段 D 併進各模組的完成定義、階段 C 保留 buf）；API 設計規範改寫；0.6 的 OpenAPI 規格保留為「現有 API 清單」供後續模組重新設計時對照，`internal/httpapi/openapi` 與 oapi-codegen 依賴刪除。
  - 不通過：寫明原因，回到 OpenAPI 路線；content 模組的搬遷成果保留，契約層改以 OpenAPI 實作。etag 的 ADR 照樣適用，改以 `ETag`／`If-Match` 與 412 實作（該 ADR 的 D5）。
  - 不論結論，刪除 `hawkr.pilot.v1alpha1` 與它的測試：proto、`internal/httpapi/pilot`、`internal/httpapi/pilot_test.go`、`scripts/verify-connect-streaming.sh`、單體與 gateway 的 `connectServices` 各一列、`website/tests/connect-pilot`、`packages/pilot_streaming_check`。gateway 的 `TestConnectStreamIsNotBuffered` 改打正式的串流 service，還沒有的話一起刪。
  - 做法與結果（2026-09-26）：
    - ADR `2026-09-26-external-contract-protobuf-connect.md` 取代 D7 與 `2026-09-25-openapi-toolchain.md`；錯誤模型 ADR 加 D10（停權）。
    - 遷移計畫：模組完成的定義改成 Connect 的契約與 client（含 D4 的 driver 特性測試）；階段 D 取消、收尾移到階段 C（刪 Dio 與手寫 client、D9）；階段 C 只刪內部 gRPC 的 buf；A3 加上 seller 的停權與登入後 service 的 web 待辦。A0 改為 gateway 當過渡入口、隨模組搬遷縮小，identity 搬進單體時入口移到單體（Will 決定）。
    - 〈API 設計規範（Connect）〉轉正，順手修掉 iOS-only 之後殘留的版本門檻 `platform`；〈API 設計規範〉標為只適用舊 gateway 的 REST。
    - 刪除 `internal/httpapi/openapi`、`api/openapi/lint-baseline.json`、`make openapi`、oapi-codegen 與它帶進來的依賴、CI 的 OpenAPI drift；`api/openapi/openapi.yaml` 保留為現有 API 清單，gateway 的路由覆蓋（雙向）與 Check B 繼續讓它與 gateway 一致。
    - pilot 照上一項刪除，四端重新生成。gateway 的 `TestConnectStreamIsNotBuffered` 沒有刪：它用假的上游，驗的是代理與中介層不緩衝串流，與 service 無關，改借用已登記的 `ContentPublicService` 路徑；讓 `Logger` 的 writer 吞掉 `Flush` 的突變仍在 3 秒內失敗。單體端的串流測試需要真的串流 handler，隨 pilot 刪除，第一個正式的串流 RPC 自帶測試。

## 通過條件

全部成立才算通過：

1. 四個 client 的 content 功能全部改用生成的 client，既有測試與建置全綠；website 在 TS 7 下型別檢查通過。
2. website 公開頁的 SSR 快取（revalidate 與 tags）行為與改動前相同。
3. 認證：web 的 cookie 與 App 的 Bearer 都能經 gateway 代理到單體；未登入、權限不足、token 過期三種情況，各端都得到正確的錯誤並觸發既有流程（登出事件、token 刷新）。
4. 錯誤：業務原因能從 `google.rpc.ErrorInfo` 取出，並對應到契約生成的 `ErrorReason` 常數；使用者文字取自 `LocalizedMessage`，欄位錯誤標在欄位上（ADR `2026-09-25-api-error-model.md`）。
5. 串流：步驟 5 的各項檢查通過。
6. 契約工具鏈：`buf lint`、`buf breaking`、各端 drift 檢查在 CI 生效，而且都以故意違規確認會失敗（刪欄位、改型別、生成物手改）。
7. 伺服器的錯誤能經 Dart 端的 interceptor 觸發全域的 session 流程，與現行 Dio 版本一致（token 刷新後重送、失敗時登出）。（2026-09-26 修正，Will 同意：原本也列「停權導向」，但它是 seller 模組的業務流程，content 的範圍內沒有會被停權擋下的呼叫；協定要證明的能力由 token 刷新與條件 4 涵蓋。停權的錯誤設計定在錯誤模型 ADR D10，seller 第一個 `*AppService` 搬進單體時實作並驗證。）
8. D8 的安全網仍然有效：content 的特性測試情境只寫一次，分別經 REST driver 對舊 gateway、經 Connect driver 對單體執行，都通過（etag 造成的刻意差異除外，見步驟 4）；故意讓單體的一支 RPC 回錯資料時，測試會失敗。
9. 公開讀取的 GET RPC 在本機回出正確的 `Cache-Control`，條件請求（ETag／If-None-Match）的行為與現行 gateway 相同，或有明確的替代做法。

任何一項不成立就寫明原因；可以用「換做法」補上的，補上後重驗，不以放寬條件通過。

## 延後的驗證（CD 第 4、5 步驗收前必須補上）

- Cloudflare 對 GET RPC 的快取行為（快取鍵含 query 的編碼參數）。
- 串流經過 Cloudflare 時不被緩衝、長連線的逾時設定。
- website 部署在 Cloudflare Workers 時，SSR 端呼叫 Connect 的行為。
- Cloudflare 是否替換 origin 的 4xx、5xx 回應內容：錯誤 ADR 的前提是 client 會依它做事的 reason 只搭配 4xx、504 會被推成 `unavailable`（`2026-09-25-api-error-model.md` D2、D7）。
- backend CD 對單體設定的 `MEDIA_ORIGIN` 探測 preset：未知的 preset 回 400、已知的 preset 配不存在的 key 回 404（`2026-09-25-api-image-uri-template.md` 的後果）。

## 驗證結果

**結論：通過**（Will 2026-09-26）。九個條件都成立（條件 7 依上方的修正），證據見下方「步驟 8」。

### 通過條件 3 的驗證範圍（Will 2026-09-26）

- App 的 Bearer：content 沒有登入後的 App service，以邊界兩側的測試驗證。gateway 的 `TestConnectAdminRequiresAnAdmin` 驗沒有登入與 token 過期回 unauthenticated、非管理員回 permission_denied、管理員帶身分轉給單體；兩支 App 的 `connect_auth_test.dart` 驗 token 過期時經 App 的 session refresh 一次後重送、憑證失效時登出、訪客不彈「登入已過期」。
- web 的 cookie：只由 admin 的同源 BFF 驗到（cookie 在 BFF 換成 gateway 的身分）。website 跨來源的 cookie、CSRF 與 `X-Active-Role` 延到第一個 `*AppService` 搬進單體時驗；這一項不在試點的判定範圍。

### 通過條件 7：停權導向（Will 2026-09-26）

- 現行 Dio 版的停權導向：gateway 的能力檢查回 CodeMeta 9240（HTTP 409），seller 的 interceptor 把 session 標成停權，router 依狀態導到公告頁。content 的範圍內，seller 只呼叫公開的版本檢查，沒有停權錯誤的產生者，所以這一項在試點裡驗不到。
- 條件修正為協定要證明的能力（見通過條件 7），停權的錯誤設計依業界做法定案：`permission_denied` 加 `ErrorInfo.reason` `SELLER_SUSPENDED`，client 只轉成賣家身分的停權狀態、導頁由 router 決定（錯誤模型 ADR D10）。實作與端到端驗證在 seller 第一個 `*AppService` 搬進單體時，列在遷移計畫的「模組完成的定義」。

### App 的編碼：JSON 或 binary（步驟 6.7，2026-09-26）

依 content 模組的欄位與實際長度組出代表性的回應，以 connectrpc 2.0.0 的 `JsonCodec`／`ProtoCodec` 編碼，解碼時間是 Dart 3.11 VM（Apple Silicon）上 2 萬次的中位數，兩種編碼解回來逐欄相同。量測程式在試點的 scratchpad，未進版控。

| 回應 | JSON | JSON gzip | binary | binary gzip | JSON 解碼 µs | binary 解碼 µs |
| --- | --: | --: | --: | --: | --: | --: |
| ListLiveBanners（5 則） | 2303 | 376 | 1780 | 319 | 24.6 | 5.4 |
| SampleRecipes（6 份） | 1525 | 313 | 1020 | 250 | 19.0 | 3.5 |
| SuggestRecipes（5 份，各 8 項食材） | 2543 | 353 | 1545 | 294 | 45.3 | 12.7 |
| GetRecipe（10 項食材、8 步） | 1614 | 464 | 1308 | 377 | 16.7 | 6.7 |
| CheckAppVersion | 229 | 232 | 148 | 158 | 2.4 | 0.6 |

- 差最多的是有沒有壓縮：gzip 後約剩 1/5。connectrpc 預設不接受壓縮，App 原本收到未壓縮的回應；`apiTransport` 改成接受 gzip（單體對 1 KB 以上的回應壓縮，gateway 原樣轉送，`TestConnectProxyPassesCompressedResponsesThrough`）。
- gzip 之後 binary 只比 JSON 小 15–20%（60–90 bytes）；解碼 binary 快 2.5–5 倍，但 JSON 最多 45µs，與網路的延遲相比可以忽略。
- 結論：App 維持 JSON。log 與 `buf curl` 看得懂，與 website 相同；改成 binary 只要換 `apiTransport` 的 codec。重新評估的條件：出現大量或頻繁的回應（例如清單分頁、串流），或裝置上的解碼時間成為可見的延遲。

### 步驟 7：量測與比較（2026-09-26）

條件：`304275c6d` 的 detached worktree，Apple Silicon Mac；量測期間另有 Swift 的建置在背景跑，時間只當量級看。OpenAPI 的同規模對照是從 `api/openapi/openapi.yaml` 抽出 content 的 17 支 operation（與 Connect 的 18 支 RPC 同範圍，REST 的版本門檻是一支 upsert），連同引用到的 schema，用 0.6 選定的三個生成器生成。量測程式與產物在試點的 scratchpad，未進版控。

**生成**

| | Connect（buf 1.50.0） | OpenAPI（content 子集） | 0.6 的全規格（612 operation） |
| --- | --- | --- | --- |
| Go | 0.7–1.6 秒；6 檔 4,328 行（前後台） | oapi-codegen 2.8.0：0.3–0.6 秒；1 檔 2,460 行 | 8 秒、約 10 萬行 |
| TS | website 2.4–2.8 秒、5 檔 1,549 行；admin 2.6–3.4 秒、6 檔 2,492 行 | openapi-typescript 7.13.0（隔離環境的 TS 5.9.3）：0.8–3.9 秒；1 個 `.d.ts` 1,214 行 | 一個 `.d.ts` |
| Dart | 2.6–7.6 秒（第一次含 `dart run` 的編譯）；15 檔 4,128 行；不需要 build_runner | openapi-generator 7.25.0 dart-dio：2.9 秒，再加 build_runner 33–35 秒；65 檔 6,152 行（其中 27 個 `.g.dart`） | 規格多一個 operation 影響 5 個檔 |
| 生成兩次逐位元相同 | 四端各生成 3 次，與 commit 的生成物相同 | 三者都相同 | 相同 |
| 規格多一支 | 生成物改 9 檔：Go 2、website 1、admin 1、Dart 5 | Go 1、TS 1、Dart 的 `lib` 5（再加 2 個 `.g.dart`） | Dart 5 |

- Connect 的檔數與行數不含 pilot；website 與 Dart 不生成後台，所以比 Go、admin 少。
- 多一支 RPC 的做法：在 `ContentPublicService` 加 `GetBanner`（重用 `Banner`）。重新生成後單體編譯失敗：`*Public does not implement ContentPublicServiceHandler (missing method GetBanner)`，漏實作在編譯期擋下。
- dart-dio 在現在的 Dart 3.11 下不能直接用：生成的 `pubspec.yaml` 寫 `sdk: '>=3.5.0'`，json_serializable 6.14 卻輸出 null-aware elements（3.8 的語法），build_runner 全部 `FormatterException`，要把 SDK 下限改成 3.8。套件名與 tag 撞名時（例如 `content_api` 對上 tag `Content` 的 `ContentApi`）analyze 出 `ambiguous_export` 錯誤；換名字後 0 error、3 個 unused import 警告。規格本身的精度缺口也直接出現在生成物：`getRecipeDetail` 的 `id` 是 `int`（ID ADR 規定是不透明字串）、回傳型別是 `CreateRecipeForAdmin200Response`。

**建置與大小**

| | 改用 Connect 前（`716fff621`） | HEAD | 差 |
| --- | --: | --: | --: |
| website 建置時間（`next build`） | 63.1 秒 | 65.9 秒 | 雜訊範圍 |
| website 所有 client JS（gzip） | 3,887,087 | 3,941,665 | +54.6 KB |
| admin 建置時間 | 30.8 秒 | 31.3 秒 | 雜訊範圍 |
| admin 所有 client JS（gzip） | 984,604 | 1,033,674 | +49.1 KB |

- 上表的差包含同期的其他改動（admin 的清單分頁等）。含 Connect runtime 的 chunk 在兩個 app 都是 2 個、合計約 37 KB gzip。
- 以 esbuild 單獨打包（minify、gzip，react 與 TanStack Query 不計）：connect＋connect-web 的 runtime 32.6 KB；加上 content 的 client 與訊息 34.8 KB；再加 `google.rpc` 的 details 35.9 KB；再加 connect-query 38.1 KB。大部分是 `@bufbuild/protobuf` 的反射與 JSON runtime，一次性的成本，每多一個 message 只增加它的 descriptor。對照 0.6 的 openapi-fetch 約 2.8 KB gzip：web 端多出約 33 KB gzip。
- App 大小：**未量**。buyer-app 的 iOS build 要複製 `ios/Runner/config/{dev,stage}/GoogleService-Info.plist`（舊公司的 Firebase，不在 repo），`AppDelegate` 啟動就呼叫 `FirebaseApp.configure()`；本機也沒有 CocoaPods 的快取，`pod install` 要下載 Firebase 等 SDK。依 Will 的指示不拆 Firebase，先回報。
- 代替的指標（Dart 的 AOT 程式大小，macOS arm64 的 `dart compile exe`，不是 App 大小）：只用 `dart:io` 的最小程式 6,244,400 bytes，加上 Connect（`apiTransport`、content 的 client、錯誤分類）後 6,736,896，**+481 KiB**；dart-dio 的對照：只用 Dio 的最小程式 6,753,296，加上生成的 content API 後 7,032,368，**+273 KiB**。App 已經帶著 Dio，所以 OpenAPI 的增量大約只有生成碼；Connect 另外帶進 connectrpc、http2、protobuf。其他模組還走 Dio 的期間，App 兩套都帶。
- App 的生成時間：Connect 的 Dart 由 buf 直接生成（見上表），不經 build_runner；dart-dio 要多跑 build_runner（content 子集 33–35 秒）。

**開發體驗**

- 在既有的 service 加一支 RPC：手改 proto、單體的 handler（`internal/httpapi/content/public.go`，有新型別時加 `convert.go`）與測試，四端各自重新生成。client 不用接線：生成的 service client 直接有新方法；gateway 以 service 為單位代理，不用改。新增 service 時，單體與 gateway 的 `connectServices` 各加一列（契約測試檢查兩邊與 `api/` 一致）。
- `buf curl --schema api/`：送出前依 schema 檢查請求，欄位打錯直接在本機報 `unknown field "platform"`，不會送出；錯誤回應印出每個 detail 解碼後的內容。
- 瀏覽器 devtools：讀取是 GET，請求放在 query 的 `message=`（URL 編碼的 JSON），Payload 分頁看得懂；回應是 JSON。錯誤的 body 除了 base64 的 `value`，connect-go 還附上每個 detail 的 `debug`（JSON 形式），例如 `{"type":"google.rpc.BadRequest", …, "debug":{"fieldViolations":[{"field":"app_type", …}]}}`，不用工具就看得懂；經 gateway 時 `X-Trace-Id` 保留。只有改成 binary 編碼時才需要工具解碼。
- 附帶發現（已修）：單體的公開 GET 回兩個相同的 `Vary: Accept-Encoding`：connect-go 對 GET 已經加了一次，`cachePolicy` 又 `Add` 一次。改成只在還沒有時才加（不依賴 connect-go 的行為），`TestHTTPCaching` 與 `TestCachePolicyVary` 檢查只有一個。

**App 改用原生 Swift 時的工具鏈**

最小的 Swift package（macOS 執行檔，Swift 6.2、`swift-tools-version:6.0`），不改兩支 Flutter App。Connect 生成 content 前台、pilot、common 與 `google/rpc/error_details`，打本機的單體與 gateway；OpenAPI 從規格抽出 content 前台 5 支，加一支公開的 SSE（`streamPostComments`）當串流的對照，打舊 gateway 與舊的 content-service。SwiftPM 的快取指到 scratchpad，所以 resolve 是冷的。

| | Connect | OpenAPI |
| --- | --- | --- |
| 套件 | swift-protobuf 1.38.1、connect-swift 1.2.3（生成用 buf 1.50.0） | swift-openapi-generator 1.13.1、swift-openapi-runtime 1.12.1、swift-openapi-urlsession 1.3.1 |
| 下載的相依 | 8 個，其中 6 個是 swift-nio 系列；只用 URLSession 時也會下載，但不連進執行檔 | 10 個，連進執行檔的 4 個 |
| 冷 resolve | 52.8 秒 | 46.7 秒 |
| 建生成器（release） | protoc-gen-swift 70.8 秒＋protoc-gen-connect-swift 52.7 秒 | 80.4 秒 |
| 乾淨 build 最小程式（debug／release） | 32.9／53.2 秒 | 17.3／31.5 秒 |
| 增量 build（改 main.swift） | 1.2–1.5 秒 | 1.3 秒 |
| 生成 | 0.08–0.25 秒；7 檔 2,731 行；兩次逐位元相同 | 0.15–0.17 秒；9 檔 3,027 行；兩次逐位元相同 |
| 規格多一支 | 改 2 檔，+81 行 | 改 3 檔，+284 行 |
| Swift 6 模式下生成碼的警告 | 0 | 31（public import 沒用到） |
| 最小程式的大小（release、strip） | 2,664,232 B；連 NIO transport 9.8 MB | 2,270,520 B |

- 另外：swift-openapi-generator 能吃完整的 612 operation 規格，生成 12.7 秒、34 萬行，debug 編譯 224.8 秒、0 error。

Connect（connect-swift）：

- GET：`ProtocolClientConfig(unaryGET: .alwaysEnabled)`，預設關閉。它把 JSON 也做 base64（`base64=1`），connect-es 與 Dart 是 URL 編碼的 JSON：同一個請求的 URL 不同，CDN 的快取鍵分開。URLSession 的 URLCache 會自己帶 `If-None-Match`，wire 上是 304，程式拿到 200 的內容。
- 串流：NIO（HTTP/2、TLS 經 gateway）與 URLSession（HTTP/1.1）都逐筆收到，間隔 496–507 毫秒；`cancel()` 後 2–3 毫秒以 `canceled` 結束，單體記下 canceled。
- 錯誤：`LocalizedMessage`、`BadRequest`、`ErrorInfo` 都取得出來（`error.unpackedDetails()`）。兩個缺口：
  - swift-protobuf 沒有公開「以值名取得 enum」的 API（`init?(name:)` 是 `internal`），`ErrorInfo.reason` 對應到生成的 `ErrorReason` 要自己寫一層（實測以一個 wrapper message 經 JSON 解碼可行，不認得的值得到 `.unspecified`）。
  - unary 失敗時 `ResponseMessage.code` 由 HTTP status 推出（400 變 `internal`、404 變 `unimplemented`），body 裡真正的 code 只在 `response.error?.code`。
- 連線中斷（串流收到第 5 筆後切斷）：NIO 在 FIN 與 RST 都回 `unavailable`；URLSession 在 RST 回 `unavailable`，**在 FIN 時以 `ok` 正常結束，資料被靜默截斷**（跑 3 次，含直連單體）。原因是 macOS 的 URLSession 在 body 中途收到 FIN 不報錯，而 connect-swift 1.2.3 不檢查有沒有收到 end-stream 訊息。連線被拒兩種 transport 都是 `unavailable`（Dart 是 `unknown`）。
- transport：URLSession 版支援 Connect 與 gRPC-Web，但 delegate 是私有的包裝，無法信任自訂 CA 或做 certificate pinning；NIO 版只有 HTTP/2，可以換 trust roots，沒有 HTTP 快取。JSON 與 binary 都支援，預設接受 gzip；7 種 transport × 協定的組合都通過。

OpenAPI（swift-openapi-generator）：

- 生成型別：每支 operation 的回應是 enum，每個宣告過的 status 一個 case，另有 `.undocumented`；envelope 不拆，要自己取 `ok.body.json.data`。規格的精度缺口直接變成型別：欄位全是 optional、`app_type` 與 `platform` 是 `String`、banner 的 `action` 是沒有型別的容器、日期是 `String`、食譜的 `id` 是 `Int64`。
- 錯誤：宣告過的 400、404、503 的 body 是型別化的 `ErrorEnvelope`；沒宣告的 status 只有原始 body；業務錯誤碼沒有型別。
- 5 支 REST 經舊 gateway 全部成功。舊的 REST 沒有 ETag，沒有 304。自訂的 URLSession 可以交給 `URLSessionTransport`，信任自訂 CA 可行。
- SSE：runtime 1.2.0 起有 `asDecodedServerSentEvents`。以假的 SSE 服務測（舊 gateway 的 SSE 要整套舊服務），逐筆收到。要手寫的：依 `event` 分派、每種事件的 data 型別（規格只寫 string）、重連與 `Last-Event-ID`、心跳逾時、分辨串流結束與連線被截斷（FIN 斷線同樣是靜默結束）。

沒驗到的：connect-swift 的 URLSession 走 HTTP/2（自訂 CA 注入不進去，以原生 URLSession 推論 FIN 時同樣不報錯）；`ErrorInfo` 經過 gateway（後台要 JWT，直接打單體並帶 gateway 會加的身分 header）；iOS 模擬器與實機（背景切換、沒有 FIN 的半開連線）；真的 gateway 的 SSE；swift-openapi-generator 的 build plugin 模式。

### 步驟 8：各通過條件（2026-09-26）

| 條件 | 結果 | 證據 |
| --- | --- | --- |
| 1. 生成的 client、測試與建置 | 成立（修掉 2 個與 Connect 無關的失敗後） | 四個 client 的 content 都改用生成的 client（步驟 6）。還沒推的 7 個 commit 沒有 CI，以本機照 workflow 的指令跑觸發到的 14 支 workflow（2026-09-26，HEAD `304275c6d`，不連網）：website 的 lint（含 TS 7 的 `tsc --noEmit`）、2,398＋2,264 個測試與 build；admin 543 個測試與 build；buyer 1,370、seller 1,540、`packages/hawkr_api` 54 個測試；單體的 unit 與 integration；go-checks 13,084 個測試；Java 2,588 個測試，都通過。失敗的兩項是移除 Android 的 commit 漏掉的：`backend/pkg/authcontract` 的 gofmt、seller 的 `force_update_provider.g.dart` 沒重新生成，已修。沒照原指令跑的：三端 buf 生成物的 drift 要連 BSR，改以本機快取裡同一個 googleapis commit 生成，結果與 commit 相同；`pnpm audit` 要連網；Java 的 integration 只跑了不需要下載 image 的部分（164 個通過，aftersales-service 與 ryuk 沒跑）。推上去之後以 CI 各 job 的結論為準 |
| 2. SSR 快取 | 成立 | 隨機食譜的 `unstable_cache`（`website/lib/api/cached.ts` 的 `getCachedSeasonalRecipes`）key、tag 與 `revalidate` 與改動前逐字相同，只換掉 fetcher（`20a0ef920`）；banner 在改動前後都沒有 server 端快取。Workers 上的行為在延後的驗證 |
| 3. 認證 | 依 Will 決定的範圍成立 | 見上方「通過條件 3 的驗證範圍」：App 的 Bearer 以 gateway 與 App 兩側的測試驗，web 的 cookie 只驗 admin BFF；website 跨來源的 cookie 不在判定範圍 |
| 4. 錯誤 | 成立 | 三端的分類器以同一份 `api/testdata/error-dispatch.json` 測試（website、admin、`packages/hawkr_api`），reason 對應到生成的 `ErrorReason`、使用者文字取自 `LocalizedMessage`；欄位錯誤由單體的 `rpc.WithFieldPaths` 對應到請求路徑，admin 以 `fieldMessage` 顯示在欄位下方（`connect-submit.test.ts`）。wire 上的形狀見步驟 7 的 `buf curl` |
| 5. 串流 | 成立 | 步驟 5：瀏覽器、iOS、經 gateway 都逐筆送達、能中止。App 只發 iOS，Android 不適用。沒有 FIN 的半開連線、串流的逐次寫入 deadline 是已知的待辦，不在條件內 |
| 6. 契約工具鏈 | 成立 | `buf lint`、`buf breaking`、四端 drift 都在 CI（`69df0ee4f` 各 job 通過）。以 CI 的同一行指令確認會失敗（2026-09-26，本機）：`v1` fixture 改型別、刪欄位、欄位改名都 exit 100；手改 Go 與 Dart 的生成物後重新生成，drift 檢查失敗（TS 在步驟 2 驗過）。`v1alpha1` 的破壞性變更依上線前的政策放行，所以 CI 的 breaking 在上線前不會擋任何實際的改動 |
| 7. Dart 的 interceptor | 成立（條件於 2026-09-26 修正） | token 過期時經 App 的 session refresh 一次後重送、憑證失效時登出，與 Dio 版一致（6.6，兩支 App 的 `connect_auth_test.dart`）；停權導向見上方「通過條件 7」 |
| 8. D8 安全網 | 成立 | `scripts/characterize-legacy-content.sh` 在 HEAD 重跑（2026-09-26）：同一份 8 個情境經 REST 打舊 gateway、經 Connect 打單體都通過，單體另外通過並發修改；三種突變由 `TestScenariosCatchWrongData`（integration）確認情境會失敗 |
| 9. 公開 GET 的快取 | 成立（明確的替代做法） | 單體：公開 GET 回 `public, no-cache` 加弱 ETag，沒變時 304；其他一律 `private, no-store`（`TestHTTPCaching`）。舊 gateway 的 content 路由（`routes_content.go`）不設 `Cache-Control`、沒有 ETag，Connect 的做法比現行明確 |

Will 的判定（2026-09-26）：

- 通過。條件 7 原本把 seller 模組的停權流程綁進協定的驗證，改成協定要證明的能力；停權的設計定在錯誤模型 ADR D10。
- 接受的代價（步驟 7，不屬於通過條件）：web 多約 33 KB gzip 的 runtime（openapi-fetch 約 2.8 KB）；其他模組搬完前 App 同時帶 Dio 與 Connect（Dart AOT 代理指標 +481 KiB）；App 大小未量。Swift 的實測是另案決策的參考，不列入判定。
