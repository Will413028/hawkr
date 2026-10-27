# 對外 API 契約：Protobuf＋Connect

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-26 |
| 影響範圍 | 根目錄 `api/`、單體的 `internal/httpapi`、gateway 的 Connect 代理、website、admin、`packages/hawkr_api` 與兩支 App；`backend/api/openapi/openapi.yaml`（改為現有 API 清單）、`internal/httpapi/openapi`（刪除） |
| 取代 | `2026-09-24-go-modular-monolith.md` 的 D7、`2026-09-25-openapi-toolchain.md`；修訂 D8 的「階段 A 把 gateway 併進單體」（本檔 D6） |
| 證據 | `docs/plans/2026-09-25-connect-contract-pilot.md` 的〈驗證結果〉（content 模組的試點，Will 2026-09-26 判定通過） |

## 脈絡

D7 選了 OpenAPI spec-first，比較的對象只有 swag 與「proto 加 grpc-gateway」，前提是 v1 的 wire 不能改。系統從未上線，這個前提不成立，所以以 content 模組試點 Protobuf＋Connect：四個 client 都改用生成的 client，依九個通過條件驗證，並與 0.6 的 OpenAPI 工具鏈做同規模對照。

約束：

- 外部：系統未上線，四個 client 都在 monorepo，v1 的 wire 沒有外部使用者。
- 外部：website 用 TypeScript 7，admin 用 5.9；App 是 Flutter，只發 iOS（ADR `2026-09-26-app-ios-only.md`）。
- 外部：CD 前面有 Cloudflare，GET 的快取鍵與 5xx 的替換要在 CD 驗收（試點的「延後的驗證」）。
- 繼承：D8 的第一層安全網是同一份特性測試先對舊實作、再對新實作跑。仍成立：遷移期間要證明行為沒變。
- 繼承：過渡期由 gateway 負責認證、CSRF、CORS、限流。仍成立到 identity 搬進單體，之後入口在單體（D6）。
- 拿掉：「同一版本不改契約」與 App 舊版相容，這是公司時期為已上線系統訂的規則（根目錄 `AGENTS.md`「上線前不受相容性約束」）。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：schema-first，IDL 是唯一來源，server 介面與各端的 client 都由它生成，CI 擋住漂移與破壞性變更。多語言、第一方 client 的主流做法有兩種：Protobuf 加 RPC 協定（Connect、gRPC；Google AIP 的資源導向設計）與 OpenAPI（OpenAPI Initiative 的規格，搭配 oasdiff 這類 breaking 檢查）。來源見文末 | — | 兩個選項都符合基準，比較見下兩列 |
| **Protobuf＋Connect** | web 的 runtime 約 33 KB gzip（`@bufbuild/protobuf` 的反射與 JSON），openapi-fetch 約 2.8 KB；其他模組搬完前 App 同時帶 Dio 與 Connect；GET 把請求編進 query，CDN 的快取鍵要另外驗；D8 的情境要寫成 driver 介面；provider 決定格式的 webhook 與回呼仍是手寫 handler（兩案相同） | **採用** |
| OpenAPI spec-first（D7，0.6 的 oapi-codegen、openapi-typescript、dart-dio） | dart-dio 的 DTO 不是不可變型別，要 build_runner（content 子集 33–35 秒），在 Dart 3.11 還要改 SDK 下限才能生成；openapi-typescript 依賴 TS 5 的 compiler API，要在隔離環境執行；串流（SSE）與 multipart 不在生成的 client 裡，各端手寫；lint 與 breaking 要另外組合工具（Spectral、oasdiff）。swag 轉出的規格精度缺口（欄位全選填、ID 是整數）在逐模組重新設計時同樣會修掉，不列入判定 | 否決 |
| proto 加 grpc-gateway | 保留 gRPC server，另加一層 REST 轉碼 | 否決：Connect 直接以 HTTP 與 JSON 提供同一份 proto，不需要轉碼層 |
| 維持手寫 client | 契約與四端的型別各寫一份 | 否決：這正是 D7 要解決的漂移 |

## 決策

- **D1 IDL 與協定**：對外 API 以根目錄 `api/` 的 Protobuf 定義，協定是 Connect。server 用 connect-go；TS 用 protoc-gen-es 與 connect-es，讀取用 connect-query；Dart 用 protoc_plugin 與 connectrpc，生成物在兩支 App 共用的 `packages/hawkr_api`。慣例寫在〈API 設計規範（Connect）〉。
- **D2 工具鏈**：buf 的 lint 用 STANDARD；breaking 用 `WIRE_JSON`，上線前的 package 一律 `v1alpha1` 並設 `ignore_unstable_packages`，上線時一次改名 `v1`。外掛鎖在各消費端自己的 lockfile、在本機執行；四端的生成物由各自的 CI 檢查 drift。
- **D3 傳輸**：web 用 JSON；App 用 JSON 並接受 gzip（試點 6.7 的量測）。讀取標 `NO_SIDE_EFFECTS`、以 GET 呼叫，公開的讀取回 `public, no-cache` 加弱 ETag。串流用 server streaming RPC；檔案由 client 以 presigned URL 直傳 R2，API 只處理 metadata。
- **D4 D8 的安全網**：特性測試的情境寫一次、透過 driver 介面操作：REST driver 打舊 gateway，Connect driver 打單體。
- **D5 OpenAPI 規格的去留**：`backend/api/openapi/openapi.yaml` 改為「現有 API 清單」，模組搬遷、重新設計 API 時對照；gateway 的路由覆蓋與 JSON contract 檢查讓它與 gateway 一致，清單的端點數只准變少，最後一個模組搬完時隨 gateway 刪除（D6）。`internal/httpapi/openapi` 與 oapi-codegen 刪除。
- **D6 gateway 的退場**（修訂 D8，Will 2026-09-26 決定）：gateway 不在階段 A 整個併進單體，而是當過渡入口（strangler facade）、隨模組搬遷縮小：每個模組搬遷時刪掉它的 REST 路由，改由 gateway 依 service 名稱代理 Connect；identity 搬進單體時入口移到單體，還沒搬的 REST 由單體轉給 gateway；最後一個模組搬完時刪除 gateway。
- **D7 即時通道**：現有 API 清單裡有 16 支 SSE（買賣家的 AI 助理、procurement、通知、公開留言），IM 是 gateway 上的 WebSocket。預設在所屬模組搬遷時改成 server streaming RPC，續傳改用請求裡的游標欄位（取代 `Last-Event-ID`），送出用 unary。例外在該模組搬遷時決定，候選是保留原本的 SSE 或 WebSocket、明列為第二套契約：IM 需要瀏覽器上的雙向即時（connect-web 在瀏覽器不支援雙向串流），在 A6 決定；AI 助理的 AG-UI 是外部的開放格式，在 A9 決定。

## 理由

- **兩種 IDL 在重新設計後都能精確描述 API，差別在生成器與協定。** 每個模組搬遷時都會重新設計 API（遷移計畫的模組完成定義），所以 swag 轉出的規格缺口不是判定的理由；以下各點才是。
- **Dart 端差距最大。** protoc_plugin 生成的訊息有 `freeze()`、`rebuild()`，不需要 build_runner；dart-dio 要 build_runner，而且生成的 pubspec 在目前的 Dart 版本就生成失敗。
- **lint、breaking、生成是同一套工具。** buf 一個 CLI 涵蓋三者，`buf breaking` 以 `v1` fixture 驗證會擋下改型別、刪欄位、欄位改名；OpenAPI 方案也做得到，但要另外組合 Spectral、oasdiff 與各語言的生成器，版本各自維護。
- **錯誤與串流有標準答案。** 錯誤用 `google.rpc` 的標準 details（ADR `2026-09-25-api-error-model.md`），串流是協定的一部分；OpenAPI 方案的 SSE 要各端手寫。
- **TS 7 不受影響。** protoc-gen-es 自帶 TypeScript 5 當直接依賴，`target=ts` 不經過專案的編譯器。
- **gateway 不整批搬進單體（D6）。** D8 原本在階段 A 把 gateway 併進單體，前提是對外的 REST handler 會沿用、再由 OpenAPI 生成的介面接手。改成 Connect 之後，每個模組搬遷時 API 都要重新設計，整批搬進來的約 600 支 REST handler 與測試都會再刪一次；讓 gateway 當入口、逐模組縮小，是 strangler 模式的標準做法（Martin Fowler，StranglerFigApplication）。入口在 identity 搬進單體時移轉，gateway 就不必回頭呼叫單體驗證 session。代價是 gateway 這個 Go 服務要活到最後一個模組搬完。
- **反論點都有答案。** 試點計畫列的四個反論點：D8 的安全網以 driver 介面保住（兩邊 8 個情境通過、突變會失敗）；公開 GET 的 `Cache-Control` 與 304 在本機驗過，Cloudflare 列入 CD 驗收；第二套契約：provider 決定格式的 webhook 與回呼兩案都要手寫，即時通道的去向見 D7；錯誤處理已在四端改寫並以同一份情境表測試。
- **代價可以接受。** web 多出的約 33 KB gzip 是一次性的 runtime，不隨 RPC 數量線性增加；App 同時帶兩套 HTTP 只在遷移期間，每個模組搬完就刪掉它的手寫呼叫。

## 後果

- **接受的代價**：上表 Protobuf＋Connect 那一列；App 大小在試點中沒量到（iOS build 缺舊公司 Firebase 的設定），以 Dart 的 AOT 程式大小代替（+481 KiB，連同 connectrpc、http2、protobuf）。
- **要改的地方**
  - 遷移計畫：模組完成的定義改成 Connect 的契約與 client；階段 D 併進各模組的完成定義，收尾移到階段 C；階段 C 只刪內部 gRPC 的 buf 設定，對外契約的 buf 保留；A0 依 D6 重新定義，A2 移轉入口，階段 A 的驗收不再要求刪除 gateway。
  - 〈API 設計規範（Connect）〉從草稿轉正，〈API 設計規範〉的 HTTP 部分標為只適用舊 gateway。
  - 刪除 `internal/httpapi/openapi`、oapi-codegen 的依賴、`make openapi` 與 CI 的 OpenAPI drift；刪除試點專用的 `hawkr.pilot.v1alpha1` 與它的測試。
- **後續的義務**寫在遷移計畫（唯一清單）：第一個 `*AppService` 的 web 部分（A2）、seller 的停權（A3）、web 的共用套件（模組完成的定義）、即時通道（A6、A8、A9 與 D7）、上線前 `v1alpha1` 改名 `v1`（階段 C）。
- **第一個正式的串流 RPC 要一起處理**：gateway 的 Connect 代理與單體都沒有逐次寫入的 deadline（client 不再讀取時，串流佔著連線直到 handler 結束）；續傳的游標；經 gateway 與單體的不緩衝測試（兩邊現在各有一個以假 handler 驗的測試）。App 的半開連線以 `pingInterval` 偵測（試點 6.6）。
- **重新評估的條件**
  - API 要開放給第三方，而對方需要 REST 或 OpenAPI：先評估由 proto 生成 OpenAPI 文件，而不是換掉 IDL。
  - Cloudflare 實測 GET RPC 的快取無法正確運作，而且沒有替代做法。
  - connect-es、connectrpc Dart 或 connect-go 停止維護。
  - App 大小：試點沒量到。App CD（第 5 步）換掉舊公司的 Firebase 設定後，以 Xcode 的 App Thinning Size Report 補量，並與 iOS 的發佈大小預算一起設門檻；超過預算時先評估把 Dio 提早換掉，而不是換協定。
  - App 改寫成原生 Swift（另案）：connect-swift 在 URLSession 上會靜默截斷串流、無法信任自訂 CA（試點步驟 7），要以 NIO transport 或修正後的版本解決。

## 補充：原文沒有寫下的否決理由與重新評估條件（2026-10-03）

決策不變。以下是 2026-09-26 討論時寫在另一份紀錄、本檔原文沒有寫下的內容，併回這裡，讓本檔是唯一的一份。

**約束**（補）

- 外部：開發者一人加 coding agent，每多一個生成器，就多一組獨立的故障模式。

**考慮過的方案**（補）

| 方案 | 代價 | 判定 |
|---|---|---|
| Protobuf 加 HTTP transcoding：proto 標 `google.api.http`，由 grpc-gateway 或 vanguard-go 提供 REST 路徑（AIP-127） | 每支 RPC 多一份路徑對應要維護；REST 面現在沒有消費者 | 否決，保留為退路（只給公開讀取或第三方） |
| GraphQL schema-first：gqlgen、graphql-codegen、ferry／Apollo iOS | 消費端是與 API 一起設計的自家畫面，用不到任意查詢；CDN 快取要 persisted query；錯誤放在 HTTP 200 的 `errors`，與 ADR `2026-09-25-api-error-model.md` 依 code 分派衝突；串流要另一種傳輸（graphql-ws） | 否決 |

**理由**（補）

- **型別在四端一致。** proto3 JSON 的 int64 轉字串、Timestamp、enum、oneof 由同一份規格定義，各官方 runtime 的行為一致；OpenAPI 的 oneOf、nullable、int64 由各生成器各自解讀，oapi-codegen 依賴的 kin-openapi 只支援 3.0。Connect 四種語言的 runtime 由同一家維護，錯誤、逾時、interceptor 的語意一致。
- **可逆性不對稱。** 選 proto 之後仍可加 `google.api.http`、由 vanguard-go 提供 REST，或由 proto 轉出 OpenAPI 給第三方，handler 都不動；反過來從 OpenAPI 換到 proto 是重寫。
- **遷移期間的 wire：搬遷時就重新設計（D4 的前提）。** 另一案是對外介面不變（strangler 的常規做法：照 swag 轉出的規格實作，上線前再統一重新設計）。不選它：保留介面是因為有線上消費者，這裡沒有；它會讓每個模組的 handler 寫兩次，並把舊規格的缺口（沒有 required、110 支自由物件、數字 ID）帶過整個遷移。試點列的反論點（同一份 HTTP 測試跑不了、錯誤處理要重寫）其實是「搬遷時重新設計」的代價，不是 protobuf 的；以 driver 介面解決，代價是每個模組多一個 REST driver。
- **gateway 退場（D6）的另外兩案。** 邊緣分流（ingress 依 path 分給 gateway 與單體，兩邊各自認證）：現在沒有可放 ingress 的部署環境；兩個 process 各自認證、讀同一份 Redis session，多一份跨 process 的資料契約，行為一分歧就是安全漏洞，否決。gateway 當入口直到最後一個模組（identity 搬進單體後由單體替 gateway 驗 session）：identity 得在整個階段 B 為 gateway 保留舊的 Redis key 格式與驗證用的 gRPC，否決。也不在 A2 之前移轉入口：得先寫一個經 legacy 呼叫 account-service 的 session 轉接，只活到下一步。D6 讓 session 的擁有者搬進來的同一步接手認證，session 的儲存格式從此只有一個讀者。遷移在 CD 之前完成、gateway 不會再被部署，所以這裡比的是白做的程式碼與開發是否中斷，不是線上流量的風險。

**接受的代價**（補）

- web 多出的約 33 KB gzip 約占 website client JS（3.9 MB）的 1%；App 端 dart-dio 的增量是 +273 KiB，可與 Connect 的 +481 KiB 對照。
- GET 的快取鍵：同一個請求在 connect-swift（base64）與 connect-es（URL 編碼）編進 query 的方式不同，CDN 的快取鍵因此不同（試點計畫〈驗證結果〉）。
- connectrpc Dart 在斷線時回 `unknown`。

**重新評估的條件**（補）

- A6、A9 都保留非 Connect 的串流 → D1 的串流理由只剩少數幾支，重新權衡（生成器與型別的理由仍在）。
- Cloudflare 驗證顯示 GET RPC 無法有效快取 → 公開讀取改走 vanguard-go 的 REST 路徑（上表 transcoding 的退路），IDL 不動。

## 修訂：D5 的例外，店主讀單篇貼文（2026-10-16，Will 決定）

原決策段保留歷史。D5 的「端點數只准變少」開一類有條件的例外：legacy 服務在它的搬遷步驟之前，為修正可見性缺口而必須補的店主讀取，可以新增一條 REST，並與同一族的路由一起刪除。每一個例外都要以文末修訂登記，不是開放式的例外；目前登記的只有本條 `GET /app/sellers/posts/:id`。

- **起因**：social 的曝光過濾（A3b 步驟 10d 第 3a-② 刀）讓公開的貼文詳情不再回草稿、已下架與被平台下架的貼文，而 website 的貼文編輯頁與賣家中心的深連結一直借用公開詳情。店主沒有別的路徑讀自己的這些貼文，編輯就不可能；沒有可接受的退化狀態（留言與迴響在店家不能公開時可以顯示空狀態，編輯不行）。
- **例外的內容**：新增 `GET /app/sellers/posts/:id`（`CapMarketing`，與 `ListMyPosts` 同一組）。`maxRESTOperations` 492 → 493。
- **不選的方案**：店主的 Connect 方法要先設計 social 的 Post 資源，A7 搬遷時整個資源（發布狀態、排程、下架原因、媒體）會重新設計，現在做是丟棄的工作；`ListMyPosts` 加 id 篩選是以 List 冒充 Get；編輯頁從列表快取取資料在重新整理與深連結時失效。
- **終點與觸發**：A7 的 social 模組提供店主的 Connect Get，這條路由與本例外同時刪除。

## 來源

- 試點計畫與〈驗證結果〉：`docs/plans/2026-09-25-connect-contract-pilot.md`
- Strangler 模式：https://martinfowler.com/bliki/StranglerFigApplication.html
- OpenAPI 規格：https://spec.openapis.org/oas/latest.html ；oasdiff：https://github.com/oasdiff/oasdiff ；Spectral：https://github.com/stoplightio/spectral
- connect-web 支援的串流類型：https://connectrpc.com/docs/web/getting-started/
- Connect 協定：https://connectrpc.com/docs/protocol/ ；Buf 的 breaking 規則：https://buf.build/docs/breaking/rules/
- Google AIP-185（版本）：https://google.aip.dev/185 ；AIP-193（錯誤）：https://google.aip.dev/193 ；AIP-127（HTTP transcoding）：https://google.aip.dev/127
- vanguard-go：https://github.com/connectrpc/vanguard-go
- 0.6 的 OpenAPI 工具鏈評估：`2026-09-25-openapi-toolchain.md`
