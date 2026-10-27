# 對外 API 的錯誤模型：Google 的標準 error details，使用者文字由伺服器提供

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-25 |
| 修訂 | 2026-09-26：D10 賣家停權（Will 同意；Connect 試點步驟 8） |
| 影響範圍 | `api/`（刪除自訂的 `ErrorInfo`，改用 `google.rpc`，新增 `ErrorReason`）、單體的 `platform/apperr` 與 `internal/httpapi/rpc`、gateway 的 Connect 錯誤處理、四個 client 的錯誤處理；〈API 設計規範（Connect）〉§八 |
| 上層決策 | Connect 試點計畫步驟 3、通過條件 3、4、7；`2026-09-25-optimistic-concurrency-etag.md` D2、D3（本檔修訂 reason 的來源） |
| 相關 | `2026-09-25-codemeta-retirement.md` |

## 脈絡

§八的草稿：Connect code 表示類別；需要區分業務情境時，details 加自訂的 `hawkr.common.v1alpha1.ErrorInfo{reason}`（reason 是 CodeMeta 的 Name）；`message` 給使用者看；不提供欄位錯誤。對照程式碼與程式庫原始碼，有五個缺口：

1. **`message` 不只單體會寫。** connect-go 在 interceptor 之前解碼請求，解碼失敗與超過大小上限的錯誤不經過轉換點，訊息是英文；gateway 把自己的英文訊息（`CSRF: origin not allowed`、`rate limit exceeded`）原樣放進去；client 程式庫會合成 `HTTP 403`、`HTTP/2 error: …`。client 分不出哪一則能顯示給使用者。
2. **gateway 的錯誤只寫 unary 格式。** 串流請求收到非 200 的回應時，connect-es 2.2.0 不讀內容，直接依 HTTP status 推 code（`protocol-connect/validate-response.js`），429 被推成 `unavailable`：被限流的串流看起來可以立刻重試。
3. **轉換點把下游的錯誤原樣轉出。** `rpc.toConnect` 遇到錯誤鏈上的 `*connect.Error` 就原樣回給 client；`legacy/mediaservice` 以 `%w` 包住 media-service 的錯誤，於是下游的 code 與訊息直接到了 client。下游如果回 `unauthenticated`，App 會把使用者登出。
4. **沒有欄位錯誤。** 唯一性、跨資源這類只有伺服器能判斷的規則，只能回一則訊息。
5. **client 沒有 reason 的常數。** 生成器的錯誤碼只輸出 Java 與 Go，通過條件 4 照現況做不到。

約束：

- 外部：Connect 協定的錯誤格式：16 個 code；details 是 type 加 binary protobuf；unary 以 HTTP status 表示，串流一律 200、錯誤放在 end-stream；回應不是 Connect 格式時，client 依 HTTP status 推 code。
- 外部：固定版本的程式庫行為（connect-go 1.21.0、connect-es 2.2.0、connectrpc Dart 2.0.0；Dart 把連線中斷回成 `unknown`，試點步驟 5 實測）。
- 外部：CD 前面有 Cloudflare。依 Cloudflare 文件，origin 的 502、504 會被換成 Cloudflare 的頁面，client 只能依 status 推 code（本專案尚未實測）。
- 外部（上線後）：舊版 App 會長期在線上，v1 App 內建的錯誤處理之後只能靠更新補。
- 繼承：過渡期由 gateway 負責認證、CSRF、限流，所以錯誤有兩個產生者。仍成立到 identity 搬進單體。
- 繼承：版本衝突只有一個原因 `VERSION_CONFLICT`，搭配 `aborted` 與 `already_exists`（etag ADR D2、D3）。仍成立。
- 繼承：越權存取的回應與「不存在」完全相同（〈API 設計規範〉§14.1）。仍成立：這是安全性質，與協定無關；本檔延伸到 details。
- 繼承：App 收到 401 時 refresh 一次、失敗就登出；賣家停權時導向停權畫面（通過條件 3、7）。仍成立。
- 繼承：只有 zh-TW 一個語系。仍成立。
- 拿掉：「錯誤文案集中在錯誤碼.md、程式碼禁止寫訊息」與「每個場景一個碼」，這是多個服務共用 `BusinessException` 時的規則。「使用 AIP-193 形狀的自訂 ErrorInfo」是同日的草稿決定，沒有留下選擇自訂型別的理由。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| **基準：Google 的 richer error model**（AIP-193、AIP-194、`google/rpc/error_details.proto`；connect-go 文件建議用 `errdetails`）。code 表示類別；details 用 `google.rpc` 的標準型別，每個錯誤都帶 `ErrorInfo{reason, domain, metadata}`；`message` 給開發者看；使用者文字放 `LocalizedMessage`；欄位錯誤用 `BadRequest`；重試延遲用 `RetryInfo` | 多一個 googleapis 依賴；每個錯誤多一到三個 detail；四個 client 都要內建分類器 | **採用**，偏離一處（D2） |
| A. 現行草稿 | 上述五個缺口；自訂型別在 `v1alpha1`，上線改名 `v1` 時 detail 的 type URL 會變，`LocalizedMessage`、`BadRequest` 都要自己再定義一次 | 否決 |
| B. `message` 維持給使用者看，把每個來源都改成中文 | connect-go 在 interceptor 之前的錯誤與 client 合成的訊息不在我們控制之內；每多一個錯誤來源都要記得再改 | 否決 |
| C. 伺服器只回 code 與 reason，文字全由 client 查表 | 舊版 App 遇到新的 reason 只能顯示通用文字；改一個字就要發版；四端各一份表 | 否決；保留「沒有 `LocalizedMessage` 時用 code 的預設文字」 |
| D. 數字業務碼 | 數字不自我描述，還與 HTTP status、CodeMeta 的值段混在一起（2026-09-25 已否決） | 否決 |
| E. RFC 9457 Problem Details | Connect 協定已經固定錯誤的格式 | 不適用；試點不通過、回到 OpenAPI 時才是選項 |

## 決策

- **D1 組成**
  - code 依 gRPC 的判準選：重試這一次就可能成功，用 `unavailable`；要重讀再重做修改，用 `aborted`；要等狀態被修正才會成功，用 `failed_precondition`。
  - details 只用 `google/rpc/error_details.proto` 的標準型別，每種最多一個，不自訂 detail 型別。
  - `message` 給開發者看（log、`buf curl`），client 不顯示也不解析；內容可以與使用者文字相同，不另寫英文版（AIP-193 對英文是 should）。
  - trace id 維持在 `X-Trace-Id` header。
- **D2 `ErrorInfo`**
  - client 需要在同一個 code 底下分支時才帶。這是對 AIP-193「每個錯誤都要帶」的偏離，理由見下一節。
  - `reason` 取自 `hawkr.common.v1alpha1.ErrorReason` 的值名；`domain` 固定 `hawkr`；`metadata` 放 client 需要的動態值，key 依 AIP-193 的格式。
  - `ErrorReason` 的值名不加 `ERROR_REASON_` 前綴，值名就是 wire 上的 reason 字串，與 googleapis 的 `error_reason.proto` 相同。`api/buf.yaml` 只對這一個檔案排除 `ENUM_VALUE_PREFIX`；零值仍是 `ERROR_REASON_UNSPECIFIED`。
  - client 會依它做事的 reason，只能搭配 4xx 的 code，因為 Cloudflare 可能換掉 5xx 的內容。
- **D3 使用者文字**
  - 伺服器給使用者的文字只透過 `LocalizedMessage` 傳遞，`locale` 固定 `zh-TW`；欄位的文字放在 `BadRequest.FieldViolation.localized_message`。
  - 文字由產生錯誤的模組寫，經 `apperr` 傳到傳輸層；gateway 的通用錯誤由 gateway 寫。
  - client 的規則四端相同：有 `LocalizedMessage` 就顯示；沒有就顯示該 code 的預設文字（client 的呈現層）；永遠不顯示 `message`。
  - `internal`、`unknown`，以及屬於 client bug 的傳輸層錯誤（ID 格式錯、page token 無法解讀），不帶 `LocalizedMessage`。
- **D4 欄位錯誤**
  - `invalid_argument` 能歸到請求的某個欄位時，帶 `BadRequest.field_violations`：`field` 是請求訊息的 proto 路徑（例如 `recipe.ingredients[2].name`），每個違規都帶 `localized_message`。
  - 同時帶一個摘要的 `LocalizedMessage`，給沒有對應欄位的畫面顯示。
  - `apperr` 的 `InvalidInput` 可以附欄位清單：模組以自己的欄位名回報，由傳輸層對應到請求的路徑。
- **D5 轉換點**
  - `toConnect` 只轉換 `apperr` 與傳輸層自己以 `rpc.Error` 建立的錯誤；其他錯誤一律回 `internal`（訊息固定、細節只進 log），包括下游 client 回的 `*connect.Error`。預期中的下游失敗，由 adapter 轉成 `apperr`。
  - connect-go 在 interceptor 之前產生的錯誤沒有 details，client 依 D3 顯示預設文字；這類錯誤的 log 在 HTTP 層補上。
- **D6 gateway**
  - Connect 路徑上的錯誤一律由 connect-go 的 `connect.NewErrorWriter` 寫出，它會依請求寫 unary、串流的 end-stream 或 gRPC 的格式，取代手寫的轉換。
  - gateway 只產生通用錯誤：`unauthenticated`、`permission_denied`、`resource_exhausted`、`unimplemented`、`unavailable`。這些錯誤帶 `LocalizedMessage`、不帶 `ErrorInfo`；限流另外帶 `RetryInfo`，值與 `Retry-After` 相同。
  - detail 用 `google.golang.org/genproto/googleapis/rpc/errdetails` 建立，不依賴單體的生成物。入口移到單體（identity 搬進單體，遷移計畫 A2）之後 Connect 不經過 gateway，本條移除。
- **D7 client 的分派與重試**（四端用同一份情境表測試）
  1. `canceled` → 不處理。
  2. 下列兩個條件同時成立才自動重試：
     - 方法可以安全重送：`idempotency_level` 是 `NO_SIDE_EFFECTS` 或 `IDEMPOTENT`，或請求帶 AIP-155 的 `request_id`。
     - 錯誤可以重試：`unavailable`；沒有收到伺服器回應的傳輸失敗（Dart 回 `unknown` 或 `internal` 也算）；帶 `RetryInfo` 的 `resource_exhausted`。

     重試以退避加 jitter；有 `RetryInfo` 就照它。
  3. `unauthenticated` → refresh 一次後重送，失敗就登出。
  4. reason 是認得的常數 → 執行該 reason 的流程，例如 `VERSION_CONFLICT`：保留輸入、顯示新舊版本對照。
  5. 有 `BadRequest` → 標在欄位上。
  6. 其他情況，包括不認得的 reason → 依 D3 顯示文字。

  每個 client 只在一層做重試：web 現在 query 層「401 以外都重試一次」的設定要拿掉。
- **D8 串流**：結束串流的錯誤用同一個模型，放在 end-stream；不結束串流的問題是串流中的一則訊息，不是 RPC 錯誤。
- **D9 相容**：上線（`v1`）之後，新增 reason、新增 metadata key、修改文字都不算破壞性變更；改 reason 的名稱或搭配的 code、刪除 reason 或 metadata key 算破壞性變更（AIP-193：同一個錯誤用同一組 reason 與 domain，出現過的 metadata key 要繼續出現）。
- **D10 賣家停權**（2026-09-26 修訂）
  - 停權的賣家呼叫被能力檢查擋下的 RPC（ADR `2026-09-18-seller-capability-authorization.md`）：code `permission_denied`，`ErrorInfo.reason` 是 `SELLER_SUSPENDED`，帶 `LocalizedMessage`。取代公司時期的 CodeMeta 9240（`SELLER_SUSPENDED_OPERATION_BLOCKED`）加 HTTP 409。
  - 被停權的是賣家（商店），不是帳號：同一個帳號的買家身分不受影響。帳號層級的封鎖是另一條路徑（認證失敗，`unauthenticated`）。
  - 產生者是單體裡的能力檢查。seller 在 A3 搬進單體時，入口已經在單體（遷移計畫 A0、A2），gateway 不產生這個錯誤，D6 不需要例外。能力檢查的做法（逐方法宣告、Redis 快取、fail-open）是 seller-service 與 gateway 分離時的設計，在 A3 依模組完成的定義重新評估該 ADR。
  - client：分類器認得這個 reason（D7 第 4 項），只把它轉成「賣家身分已停權」的狀態；導向停權公告頁由 router 依這個狀態決定，interceptor 不直接導頁。只影響賣家的畫面（seller-app、website 的賣家中心），不影響買家。App 的轉換在 `packages/hawkr_api` 的共用 interceptor。
  - 停權狀態的主要來源仍是登入與恢復 session 時讀取的賣家資料；這個錯誤是停權發生在 session 中途時的即時訊號。
  - 實作時機：seller 第一個 `*AppService` 搬進單體時，在同一個變更加入 `ErrorReason` 的值、產生者與四端的處理，並端到端驗證。在那之前不加：沒有產生者的 reason 與 client 分支只能驗單邊，而且會與之後才設計的授權漂移。
  - 買家看到停權賣家的商品（CodeMeta 9241）是資源的狀態、不是呼叫者的權限，不適用本條；由 product 或 ordering 搬遷時依「代碼值與錯誤碼的去向」決定。
  - 實作（2026-10-03，A3b 步驟 8a）：值、產生者與三端的分類器同一個變更；admin 不受影響；端到端驗證在步驟 10 client 改呼叫 seller 的 Connect API 時（尚無 client 呼叫產生它的 RPC）。

## 理由

- **採用標準型別。** 草稿的五個缺口，standard details 都有現成的答案（`LocalizedMessage`、`BadRequest`、`RetryInfo`）。type URL 不會隨 `v1alpha1` 改成 `v1` 而改變；gateway 不必生成我們的 proto，也能帶 detail。代價是一個 googleapis 依賴，每個錯誤多幾十 bytes。
- **文字由伺服器提供，用 `LocalizedMessage` 標示。** `message` 有四個寫入者，其中兩個不在控制之內；`LocalizedMessage` 出現，本身就代表「這是寫給使用者的」，沒有它就退回預設文字，預設行為是安全的。由伺服器提供，也讓舊版 App 對上線後才新增的錯誤有正確的說明，改錯字不必發版。
- **`ErrorInfo` 不是每個錯誤都帶（唯一的偏離）。** AIP-193 要求必帶，是為了讓 client 不必依 `message` 判斷，`message` 才能自由修改。本專案四個 client 都在 monorepo，D3 禁止解析 `message`，這個前提不存在。其餘錯誤如果也帶 reason，只會與 code 一一對應（例如 `INVALID_ARGUMENT` 對 `invalid_argument`），成為沒有讀取端的登記。之後要補上，只是新增 detail，上線後也不算破壞性。代價是只有帶 reason 的錯誤能依 reason 統計。
- **重試要看方法的冪等性，不能只看 code。** `unavailable` 不代表請求沒有執行；經過 Cloudflare 的 504 會被推成 `unavailable`，Dart 的斷線回 `unknown`。只看 code 的話，可能已經寫入的修改會被重送。
- **欄位錯誤現在就做。** 只有伺服器知道的規則，如果 v1 App 沒有顯示欄位錯誤的能力，之後在舊 App 上都只能是一則訊息。
- **gateway 用 `ErrorWriter`。** 這正是 connect-go 為 handler 前面的中介層提供的 API，涵蓋串流；手寫的版本只處理 unary。
- **domain 用 `hawkr`。** 錯誤只有我們自己的產生者；正式網域要到上線時才確定。
- **停權用 `permission_denied`，reason 指出被停權的主體（D10）。** 停權的賣家不能執行某些操作，是呼叫者的授權狀態：Google 對停權的專案回 `PERMISSION_DENIED` 加 `ErrorInfo.reason` `CONSUMER_SUSPENDED`（`google/api/error_reason.proto`），reason 指的是被停權的 consumer，形狀與本 ADR 相同。所以 reason 是 `SELLER_SUSPENDED` 而不是帳號層級的名稱，client 的狀態也只限賣家身分。409 是狀態衝突（並發修改），不是授權。D1 的 `failed_precondition` 指的是被操作資源的狀態，買家遇到停權賣家的商品才屬於這一類。導頁由狀態驅動，同一個狀態也來自登入時的賣家資料，兩個來源走同一條路，不必在 interceptor 裡處理畫面。

## 後果

- **接受的代價**
  - TS 與 Dart 的消費端從 BSR 的 googleapis 生成 `google/rpc/error_details.proto`（Will 同意下載）。`api/` 的 proto 沒有 import 它，由三個 `buf.gen.yaml` 各自釘 commit；與 `api/buf.yaml` 為了 `field_behavior` 鎖在 `buf.lock` 的版本相同，CI 檢查一致（〈API 設計規範（Connect）〉§十二）。
  - Go 端的 managed mode 不覆寫 googleapis 的 `go_package`，單體與 gateway 都用 genproto 的 `errdetails`，因為同一個 full name 在 Go 的 registry 註冊兩次會衝突。
  - gateway 新增 `connectrpc.com/connect` 直接依賴。
  - `apperr` 新增欄位違規；模組以自己輸入的欄位名回報，handler 以明確的對應表轉成請求的路徑（D4）。metadata 在第一個需要動態值的 reason 出現時才加：目前唯一的 `VERSION_CONFLICT` 沒有動態值。
  - 四個 client 的 v1 要內建完整的分類器。
  - `buf.yaml` 多一條只作用在 `error_reason.proto` 的 lint 例外。
- **要改的地方**
  - 刪除 `api/hawkr/common/v1alpha1/error.proto`，新增 `error_reason.proto`。
  - `rpc/errors.go` 的 `WithReason` 與 `toConnect`；`rpc_test.go` 的「handler 自己的 Connect 錯誤照原樣」改成「下游錯誤變成 internal」。
  - `apperr`、`legacy/mediaservice` 的錯誤轉換。
  - gateway 的 `middleware/connect.go` 與 `proxy/connect.go`。
  - 〈API 設計規範（Connect）〉§八；etag ADR D2 的 reason 來源；試點計畫的通過條件 4 與步驟 6（分類器與情境表）。
  - D10：seller 第一個 `*AppService` 搬進單體時實作（遷移計畫「模組完成的定義」）。
- **驗證**
  - 情境表在四端都通過；「非冪等方法遇到 `unavailable` 也重試」的突變要讓測試失敗。
  - gateway 的每一條錯誤路徑，unary 與串流都取回同樣的 code 與 details；「串流錯誤回非 200」的突變要讓測試失敗。
  - 每個 client 都以生成的常數取出 `VERSION_CONFLICT`。
- **重新評估的條件**
  - 需要第二個語系：locale 改由請求決定。
  - API 開放給第三方：`ErrorInfo` 改成每個錯誤都帶，`domain` 改用網域。
  - 需要依 reason 統計所有錯誤。
  - Cloudflare 實測 4xx 的內容也會被換掉。
  - identity 搬進單體：移除 D6。

## 來源

- 本 ADR 是與 Will 討論後的決定（2026-09-25），另以兩份獨立草稿對照；無其他外部來源文件。
- AIP-193：https://google.aip.dev/193 ；AIP-194：https://google.aip.dev/194 ；AIP-155：https://google.aip.dev/155
- `google/rpc/error_details.proto`、`google/rpc/code.proto`：https://github.com/googleapis/googleapis/tree/master/google/rpc ；`google/api/error_reason.proto`：https://github.com/googleapis/googleapis/blob/master/google/api/error_reason.proto
- Connect 協定：https://connectrpc.com/docs/protocol/ ；connect-go 的錯誤：https://connectrpc.com/docs/go/errors/
- gRPC status codes 的判準：https://grpc.github.io/grpc/core/md_doc_statuscodes.html
- Cloudflare 502／504：https://developers.cloudflare.com/support/troubleshooting/http-status-codes/cloudflare-5xx-errors/error-502-504/
- 本機查證：connect-es 2.2.0 `dist/esm/protocol-connect/validate-response.js`（非 200 的串流回應依 status 推 code、不讀內容）；`backend/internal/httpapi/rpc/errors.go` 的 `toConnect`；`backend/gateway/internal/middleware/connect.go` 的註解（只轉換 unary）

## 修訂：使用者文字依請求語系，開發者訊息自動產生（2026-10-17，Will 決定）

原決策段保留歷史。「需要第二個語系」的條件已成立，決定見 `2026-10-17-multi-locale-architecture.md`（D1、D3）。

- 脈絡的約束「繼承：只有 zh-TW 一個語系。仍成立」不再成立（買家與賣家介面 zh-TW／en，admin 只有 zh-TW）。
- 決策「`message` 給開發者看……內容可以與使用者文字相同，不另寫英文版」：改為 `Status.message` 與 `FieldViolation.description` 由 `ErrorInfo.reason` 加 metadata 自動產生，給開發者看，不隨請求語系變，也不顯示給使用者。TS 與 Dart 的 client 同一刀拿掉在 `FieldViolation` 沒有 `localized_message` 時退回 `description` 的做法；每個要給使用者看的 `FieldViolation` 必須帶 `localized_message`。
- 決策「`LocalizedMessage` 的 `locale` 固定 `zh-TW`」：`locale` 是實際使用的語系。同步回應依 client transport 送的單一值 `Accept-Language`（伺服器協商，信心值 No 時用 zh-TW，回應帶 `Vary: Accept-Language`）；非同步訊息依收件人 `preferred_locale`；admin RPC 固定 zh-TW。
- 後果「需要第二個語系：locale 改由請求決定」：已執行，依上條。
