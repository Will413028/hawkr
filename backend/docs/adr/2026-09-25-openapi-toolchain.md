# 對外 HTTP 契約的生成工具

| 項目 | 值 |
|---|---|
| 狀態 | superseded by `2026-09-26-external-contract-protobuf-connect.md` |
| 日期 | 2026-09-25 |
| 影響範圍 | `api/openapi/openapi.yaml`、`internal/httpapi/openapi`，以及 website、admin、buyer-app、seller-app 的 API client |
| 上層決策 | `2026-09-24-go-modular-monolith.md` D7 |

## 脈絡

- D7：OpenAPI 規格是對外 HTTP 契約的唯一來源，server 介面與三種 client 都由規格生成，工具在遷移計畫階段 0 選定並驗證。
- 規格由 gateway 的 swag 註解轉出，OpenAPI 3.0.3，612 個 operation，含 16 支 SSE、multipart 上傳、表單回呼、遞迴型別。
- 不可動的約束：
  - Go：接在 `internal/httpapi` 的 `net/http` ServeMux 上；模組逐一搬進單體，所以要能只接一部分 operation。
  - Web：website（TypeScript 7.0）與 admin（TypeScript 5.9）都有自己的 transport：依呼叫決定 credentials、CSRF header、`X-Active-Role`、401 事件、envelope 拆包。生成物不能換掉它。
  - App：dio 5 加 json_serializable，兩支 App 共用自己的 Dio 與 interceptor。
- 評估方式：每個候選都對完整規格生成，接著編譯、型別檢查或 analyze；生成兩次比對是否逐位元相同；再用本機假 server 跑 wire test（帶 path 與 query 的 GET、帶 JSON body 的 POST、回 4xx `ErrorEnvelope`）。

## 考慮過的方案

**Go server**

| 方案 | 結果 | 判定 |
|---|---|---|
| oapi-codegen v2.8.0，strict server、std-http | 生成 8 秒，約 10 萬行，build 5 秒，兩次生成相同。SSE 回應是 `io.Reader`，而 response object 是介面，可以自訂型別逐筆 flush。每個 operation 有自己的 `http.HandlerFunc`，可以只註冊已搬進來的 operation | 採用 |
| ogen v1.24.0 | 生成約 87 萬行，build 50 秒。不支援 SSE 的 server 端，16 支 SSE operation 被跳過 | 否決 |

**TypeScript client（website、admin）**

| 方案 | 結果 | 判定 |
|---|---|---|
| openapi-typescript 7.13.0 加 openapi-fetch 0.17.0 | 只生成型別（一個 `.d.ts`），runtime 約 2.8 KB gzip，不隨 operation 數量增加。`fetch` 參數的型別就是 `(Request) => Promise<Response>`，現有 transport 原封不動當 `fetch` 或 middleware 用。`--check` 可以直接當 drift gate。TS 5.9 與 7.0 型別檢查都是 0 錯，wire test 通過 | 採用 |
| @hey-api/openapi-ts 0.99.0 | 型別檢查與 wire test 也都通過，multipart 型別較好，另有 SSE client。但它在 app 裡另放一套 HTTP runtime，錯誤模型與 JSON 解析都是自己的。仍是 0.x，一年 67 次 release。schema 名稱與 operation 衍生名稱衝突時，產生 `…Response2` 這類依生成順序決定的名字（現有規格有 24 組） | 否決 |

**Dart client（buyer-app、seller-app）**

| 方案 | 結果 | 判定 |
|---|---|---|
| openapi-generator 7.25.0 的 dart-dio，json_serializable、`skipCopyWith=true` | 名稱由 operationId 導出：規格多一個 operation 只影響 5 個檔。現有規格直接可生成，analyze 0 error。回傳 `Response<T>`，每次呼叫可以帶 extra、headers。直接用 App 現有的 Dio | 採用 |
| swagger_parser 1.44.3，retrofit 加 freezed | DTO 是 freezed，與 App 慣例一致。但 inline 物件依走訪順序編號（`Data` 到 `Data158`），規格多一個 operation 就改動 307 個檔。中文 tag 生成不合法的 identifier。inline 的 list 被命名成 `List`，蓋掉 `dart:core` | 否決 |

## 決策

- Go：oapi-codegen 的 strict server（std-http）。生成到 `internal/httpapi/openapi`，進版控，CI 重新生成並比對。版本由 `go.mod` 的 tool 依賴固定。
- TypeScript：openapi-typescript 生成型別，openapi-fetch 當 runtime，各前端的既有 transport 以 `fetch` 或 middleware 接上。
- Dart：openapi-generator 的 dart-dio，json_serializable、`skipCopyWith=true`，生成成獨立 package，App 以現有的 Dio 建立 API 物件。
- TypeScript 與 Dart 的生成物在遷移計畫階段 D 接進各前端，版本與 drift 檢查放在各前端。上表的版本是這次驗證時的版本。
- 規格維持 OpenAPI 3.0.3 單一 YAML 檔。暫不採用 3.1：oapi-codegen 依賴的 kin-openapi 只支援 3.0。

## 後果

**接受的代價**

- openapi-typescript 使用 typescript 套件的 JS compiler API，而 TS 7（原生編譯器）沒有這套 API。生成器必須在隔離環境執行，例如 `pnpm dlx --package=openapi-typescript@7.13.0 --package=typescript@5.9.3`。生成的是純型別，TS 7 可以直接使用。
- openapi-typescript 把 multipart 的 binary 生成成 string，也不處理 SSE：multipart 要自己組 `FormData`，SSE 沿用各前端既有的 client。
- dart-dio 的 DTO 不是 freezed，而且 json_serializable 模式官方仍標為 beta。需要 copyWith 或 union 的地方，在 data layer 轉成 App 自己的 freezed model。錯誤以 `DioException` 丟出，body 要自己轉成 `ErrorEnvelope`。
- 三端共同：SSE 端點與非瀏覽器呼叫的端點（PayUni 回跳、Apple 網域驗證），client 不使用生成的方法。server 端的 SSE 要自訂 response object 逐筆 flush。

**應該重新評估的條件**

- 規格把 inline 物件都抽成具名 component 之後，swagger_parser 的命名不穩定就不存在，freezed DTO 反而是優勢。屆時重跑 Dart 的評估，包含「規格多一個 operation 會改動幾個檔」。
- kin-openapi 與 oapi-codegen 支援 OpenAPI 3.1 之後，評估是否升級。
- 前端全面改用 TS 7，而 openapi-typescript 仍停在 TS 5 的 compiler API 時。
