# imageai 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

## 沿用盤點

ai-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| ID 方案（M1）：全 TSID BIGINT（Go 端產生），ADK 表用 TEXT | Java 需要認得 ID（舊服務互通） | 否：對 Java 只有 `order_no`、`prospect_id`、`store_id`、`buyer_id` 是外部鍵（`idrefs.csv:19`），run、request、candidate 等自己的實體沒有 Java 消費者【推】 | 自己的實體用 UUIDv7（row-identity ADR）；外部鍵維持 bigint 到最後一個 Java 消費者退場（同 A6 M1） | 實體 UUIDv7；buyer、store、prospect 引用維持 bigint |
| gRPC 內部服務、method policy、metadata 注入身分（M2） | 兩個進程之間要信任邊界 | 否：單體內 session 就是身分 | 入口層（`internal/httpapi`）從 session 取身分，模組公開 API 收 `buyer.ID`、`store.ID` 參數 | 刪 gRPC 與 metadata；method policy 改為 Connect 層 interceptor 宣告（同 `adminauth`） |
| LLM SDK（M4）：Gemini 走 `google.golang.org/genai`；修圖手寫 HTTP（`transform_usecase.go`） | 需要多 provider 切換 | 部分：只有 `EXTRACTOR_MODEL` 一個開關；登記表 4 個、其中 gemma 是 `:free`（無 SLA）【量】 | 一個 `LLM` port 加 provider adapter；明確的 timeout、retry、budget；修圖也走同一個 port | 修圖併入同一個 LLM port（M4 建議去向，Q1）；A9-D10：只搬 Gemini、DeepSeek；單一建構點在 A9-1c 落地（AD5） |
| 金鑰與設定（M5）與修圖繞過 config（AD17） | 單一進程讀 env | 否：單體有統一設定載入（`cmd/hawkr`）；AD17：修圖繞過 config | 設定集中在 composition root；secret 不進 repo | 併入 `cmd/hawkr` 設定；AD17 接受，搬時併入 config（A9-4a）；`LLM_API_URL`、`LLM_API_KEY`、`LLM_MODEL` 無讀取者，不搬（AD13） |
| timeout 與 retry（M6）：修圖 60s | 外部 provider 要有 deadline | 成立：每個外部呼叫都有 deadline【量】 | 同；加 provider 層熔斷與併發上限 | 保留並集中到 LLM 單一建構點（A9-1c）；串流穿透的前提與秒數依計畫的串流穿透 spike（A9-0）結論，記在 A9-5 的驗收 |
| prompt 儲存（M7）：唯一存在 DB 的 prompt 是修圖的 `ai_styles.prompt` | 需要不經部署修改 prompt | 否：沒有 prompt 版本欄位；風格 prompt 由後台管理（`CreateAiStyle`、`UpdateAiStyle`） | prompt 進程式碼並隨版本控管；後台可改的內容放資料表 | `ai_styles.prompt` 隨後台風格管理留在資料表（A9-D2 搬後台風格）；`style_usecase.go:31` 的農產文案隨 scope cut 改寫（Q4） |
| observability（M8）：OTel meter（24 個 instrument）加 Prometheus `promauto` gauge；告警規則在 repo 外 | 有 Prometheus、OTel pipeline | 單體：待定（計畫 A7 延後項說「單體告警管道」尚未建立） | 單一 metrics 入口、標籤封閉基數 | 沿用指標名（告警規則依賴），指標命名照 A9-D11（沿用 `ai_procurement_llm_*`）；等單體告警管道（Q10） |
| 預約制配額與 advisory lock 字串雜湊命名空間（M9）：`ai_usage_reservations` | 兩個進程併發，需要先佔再扣 | 部分：單體仍多 pod；但可用單列 counter 加條件 UPDATE，不需要預約表與雜湊鎖 | 一列 counter 加條件 UPDATE（`WHERE used < limit`），失敗即拒絕；預約只在長呼叫才有意義 | A9-D9（舊版額度是程式常數 `domain/usage.go`、`quota_usecase.go:75`，不是 config）：改成一列計數器加條件 UPDATE，外部呼叫失敗時退回，不建預約表（取代 M9 建議去向「修圖保留預約」）；額度數字（預設 10、上限 1000）是模組常數，寫在 `backend/docs/modules/imageai.md`，不是 config（舊版也是程式常數） |
| 用 gRPC client 取別的服務資料（M12）：media | 資料在別的服務 | 否：media 已在單體 | 模組公開 API；未搬的走 `internal/legacy/*` | 換成 media 模組 API（Q3） |
| fx 組裝、`pkglogger`（zap）、`syncutil.SafeGo`（M17） | 單進程 DI | 否：單體 composition root 手動接線 | 同單體規則 | 全改成 `cmd/hawkr` 接線；`go-pkg/{identity,tsid,outbox,mq,errors/bizcode,grpcclient}` 換成單體的 `platform` 對應 |
| bizcode 數字碼加 CodeMeta（M18）：例如 `9703` | 舊 REST 契約 | 否：新契約用 `ErrorReason` | `api/` 的 `ErrorReason` 加 Go 型別 | 全部轉 `ErrorReason`；`procurement_error_reason_registry_test.go` 已有註冊表概念，可沿用【量】 |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 請求大小：body_guard 10 MiB、gRPC 20 MB | 舊入口收原圖 | 否：單體 Connect 全域 2 MiB（`internal/httpapi/server.go`），規範 §十、§十一 | 檔案走 presigned 上傳；或暫態運算 inline 加上限 | A9-D37、A9-D42（A9-4b 落地）：inline `bytes`，原圖上限 2 MiB 由模組檢查、回 invalid_argument 帶 `image_data`；`ImageAiAppService` 單獨把讀取上限設 3 MiB 容納 base64；client 先縮圖；規範 §十、§十一 區分暫態運算與要保存的檔案 |
| 身分：gateway header 判斷 admin（AD8）、metadata 注入 store | 兩進程信任邊界 | 否：session 與 Cloudflare Access 在入口層 | 入口層宣告權限 | app 兩支（A9-4b）：handler 由 session 經 `seller.CurrentStore` 推導店家，沒有店家回 permission_denied，請求不帶店家 ID；後台（A9-4c 落地）：`adminauth` 宣告讀 2、寫 5 `audited`，同 tx `adminaudit.Record`（`internal/admin`；`TestStyleWritesRecordTheAuditInsideTheTransaction`、`TestAuditRowRollsBackWithTheBusinessWrite`） |
| 錯誤：bizcode 4291、9250–9253 | 舊 REST 契約 | 否 | `ErrorReason` 加 Connect code | 照 `error_reason.proto` 的四個 `IMAGE_*`（A9-4b 落地；文案進模組 `internal/text`，不再寫死在 handler） |
| 限流：byte 預算 semaphore（256 MiB、×6） | 防大圖同時解碼吃光記憶體 | 部分：入口固定上限後等於「同時 N 張」 | 併發數 semaphore；外部 provider 的專案層級速率限制另由 provider 回 429 處理 | 改併發數 semaphore，N 由 `platform/config` 讀；預設值依舊公式推得 21，未量測（`IMAGEAI_MAX_CONCURRENT`，4a 已落地，以記憶體量測校正前不當成容量保證）；跨 pod 不協調，Gemini 429 轉 `RESOURCE_EXHAUSTED`。先佔名額再扣額度（A9-4b 落地：`internal/transform`，滿了回 resource_exhausted 帶 RetryInfo、不扣額度；Gemini 429 同樣轉 resource_exhausted） |
| 後台 7 支（舊 `AIAdminService`）：無樂觀鎖、`is_default=false` 靜默忽略、縮圖 link 在 DB insert 之前且不同 tx、`DeleteAiQuota` 無列也成功、`GetAiQuota` 不驗店存在並吞 DB 錯誤 | 舊入口的 `x-account-type` 判斷與逐方法覆寫 | 否：契約已定 etag、update_mask、staged token、NOT_FOUND | etag 加條件式 UPDATE；縮圖引用由擁有者的同步 job 單一寫入 | 全改（A9-4c）：`internal/admin`（etag、預設風格規則 A9-D39、提示詞 A9-D40、店家存在 A9-D41）、`internal/mediaref`（照 content banner）；`default_style=false` 只在目前是預設時回 invalid_argument，停用目前的預設回 failed_precondition（契約沒寫，依 schema 的 `styles_default_enabled_check` 補） |
| 下游逾時：修圖 60s | 外部 provider 要有 deadline | 成立 | 同 | 保留，集中到 `platform/llm`（見 M6） |

## 與 ai-service 的差異

<!-- 資料沒有搬移（系統從未上線），schema 是新的 baseline。每一項對外看得到或 schema 上的差異一行。 -->
