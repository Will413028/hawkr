# assistant 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

## 沿用盤點

ai-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| ID 方案（M1）：全 TSID BIGINT（Go 端產生），ADK 表用 TEXT | Java 需要認得 ID（舊服務互通） | 否：對 Java 只有 `order_no`、`prospect_id`、`store_id`、`buyer_id` 是外部鍵（`idrefs.csv:19`），run、request、candidate 等自己的實體沒有 Java 消費者【推】 | 自己的實體用 UUIDv7（row-identity ADR）；外部鍵維持 bigint 到最後一個 Java 消費者退場（同 A6 M1） | 實體 UUIDv7；buyer、store、prospect 引用維持 bigint |
| gRPC 內部服務、method policy、metadata 注入身分（M2） | 兩個進程之間要信任邊界 | 否：單體內 session 就是身分 | 入口層（`internal/httpapi`）從 session 取身分，模組公開 API 收 `buyer.ID`、`store.ID` 參數 | 刪 gRPC 與 metadata；method policy 改為 Connect 層 interceptor 宣告（同 `adminauth`） |
| ADK v2、GORM session service、pgx（M3）：session、event 持久化在自己 4 張表 | 需要 ADK 的 agent loop 與 AG-UI 事件 | 部分：助理 coordinator 靠 ADK | 狀態機在業務表，agent loop 輕量；若留 ADK，用它的 pgx 原生儲存而非 GORM 第二套 ORM | A9-D5：保留 ADK、session 改 pgx 實作 `session.Service`，以 ADK 的 `RunServiceTests` 當驗收（`spikes.md` §1；A9-2）；GORM 不進單體。A9-2a 已實作（`internal/adksession`，4 張表，細節見下列各列） |
| LLM SDK（M4）：Gemini 走 `google.golang.org/genai`；DeepSeek、OpenRouter 走 `openai-go/v3` 的 `/responses`；修圖手寫 HTTP | 需要多 provider 切換 | 部分：只有 `EXTRACTOR_MODEL` 一個開關；登記表 4 個、其中 gemma 是 `:free`（無 SLA）【量】 | 一個 `LLM` port 加 provider adapter；明確的 timeout、retry、budget；修圖也走同一個 port | A9-D10：只搬 Gemini、DeepSeek，不搬 OpenRouter；單一建構點與架構測試（AD5）在 A9-1c 落地，本模組只依賴該 port，不自建 client |
| 金鑰與設定（M5）：env 讀取（反射加 strict env 警告）；`GEMINI_API_KEY`、`DEEPSEEK_API_KEY`、`OPENROUTER_API_KEY`、`LLM_API_KEY` 等 | 單一進程讀 env | 否：單體有統一設定載入（`cmd/hawkr`） | 設定集中在 composition root；secret 不進 repo | 併入 `cmd/hawkr` 設定；`LLM_API_URL`、`LLM_API_KEY`、`LLM_MODEL` 全 repo 無讀取者，不搬（AD13） |
| timeout 與 retry（M6）：assistant 30s；SDK 自帶重試關掉 | 外部 provider 要有 deadline | 成立：每個外部呼叫都有 deadline【量】 | 同；加 provider 層熔斷與併發上限 | 保留並集中到 LLM 單一建構點（A9-1c）；timeout 常數與 `observability` 的直方圖 bucket 鏡像要保持同步 |
| prompt 儲存（M7）：coordinator、specialist 的 prompt 與 `return_policy.go` 以 Go 字串常數寫在程式裡 | 需要不經部署修改 prompt | 否：沒有 prompt 版本欄位；`return_policy` 有「gateway 先讀 key `return-policy`、沒有值才退 ai-service 固定文案」的過渡 | prompt 進程式碼並隨版本控管，加 eval gate；後台可改的文案走 content 模組 | 留在程式碼；`return_policy.go` 與 content 的關係在 A9 結案（Q8）；coordinator prompt 內「生鮮賣家小幫手」「食安責任」「農產品圖片」等農產文案隨 scope cut 改寫（Q4） |
| observability（M8）：OTel meter（24 個 instrument）加 Prometheus `promauto` gauge；告警規則在 repo 外 | 有 Prometheus、OTel pipeline | 單體：待定（計畫 A7 延後項說「單體告警管道」尚未建立） | 單一 metrics 入口、標籤封閉基數 | 沿用指標名（告警規則依賴），指標命名照 A9-D11（沿用 `ai_procurement_llm_*`）；等單體告警管道（Q10） |
| 用 gRPC client 取別的服務資料（M12）：tool 呼叫 buyer 與 ordering | 資料在別的服務 | 部分：buyer 已在單體；ordering 還在 Java | 模組公開 API；未搬的走 `internal/legacy/*` | buyer 換成模組 API；ordering 經 `internal/legacy` 補 client，模組只依賴 port（A9-D11；Q3） |
| fx 組裝、`pkglogger`（zap）、`syncutil.SafeGo`（M17） | 單進程 DI | 否：單體 composition root 手動接線 | 同單體規則 | 全改成 `cmd/hawkr` 接線；`go-pkg/{identity,tsid,outbox,mq,errors/bizcode,grpcclient}` 換成單體的 `platform` 對應 |
| bizcode 數字碼加 CodeMeta（M18）：例如 `9703` | 舊 REST 契約 | 否：新契約用 `ErrorReason` | `api/` 的 `ErrorReason` 加 Go 型別 | 全部轉 `ErrorReason`；`procurement_error_reason_registry_test.go` 已有註冊表概念，可沿用【量】 |
| session 表的 event 欄位（M3 的細部）：舊 GORM 版把 event 拆成約 20 個欄位 | 欄位要能被 SQL 查詢或索引 | 否：assistant 只依 `timestamp` 與 `seq` 排序與篩選，沒有任何查詢讀 event 內容欄位【量，A9-2a 的 query 清單】 | event 整份存 JSON，查詢用的欄位另存 | 整個 `session.Event` 存 `events.event`（JSONB，ADK 公開並測試過的往返編碼），`timestamp`、`seq` 另存（`id` 只是主鍵）；ADK 新增 event 欄位不需改 schema |
| 並行寫入防護（M3 的細部）：GORM 版 `AppendEvent` 比對 session 更新時間，過期就回 stale 錯誤 | 兩個 append 可能同時改同一份 state | 部分：同一對話可能同時有兩輪（重送、雙裝置）；這個防護擋「讀到舊 session 再寫」，但不阻止兩輪交錯寫 event | 寫入端用列鎖與原子合併保證不丟更新；輪次層序列化同一對話的輪次 | 不搬 stale 檢查：`AppendEvent` 鎖 session 列、state 以 jsonb `\|\|` 在 SQL 內合併，並行 append 不丟 key（CI 測試 `TestConcurrentAppendsKeepEveryStateKey`）；同一對話兩輪交錯的 event 序列由輪次執行器（A9-2b）決定是否序列化，不在儲存層擋 |
| state 的 JSON 往返（M3 的細部）：19 位 TSID 要先轉字串才不被 float64 改寫（舊 `getInt64State` 的註解） | state 經 JSON 存取 | 是：新儲存也是 JSONB，讀回的數字是 float64 | 以字串存大整數 ID | 維持：寫入 state 的 ID 一律字串（`user_id` 本身就是字串）；數字型 state 值只保證 float64 精度 |
| agent 拓撲（A9-2b-1）：buyer coordinator 加 3 個 specialist（agenttool 委派），按鈕端點直接跑 specialist | 要把「訂單、物流、問答」分給不同 prompt 與 tool 集合 | 部分：只剩訂單與物流兩個 specialist（A9-D2 刪 seller_match）；舊 ADR 實測 coordinator 27/28、all-tools 20/28【引】 | 一個 agent 掛少量 tool，或 router 加單一職責 agent | 沿用 coordinator 加 2 specialist：每輪重新建構（tool 回呼綁這一輪）；主題決定入口（訂單、物流直接進 specialist，問答進 coordinator）；不在這步重新設計 prompt（Q4 的農產文案已不在買家路徑） |
| tool 的買家身分（A9-2b-2）：從 session state 讀 `buyer_id`，值型別接受 string、int64、float64 | state 經 JSONB 往返，ID 以字串存 | 否：新儲存一律寫字串；float64 會改寫 19 位 ID | 身分取自驗證過的呼叫者，tool 不信 model 與可變 state | tool 讀 `ctx.State()`，只接受字串，且必須等於這一輪的買家（輪次由 session 登入身分決定），不符回錯、不呼叫訂單服務；int64、float64 兩個舊 session 相容分支刪除（系統未上線） |
| 事件翻譯（A9-2b-3）：`RunAssistantStream` 的 sentText 補尾巴規則、thought 過濾、tool 回應重置 | ADK SSE 模式先送 partial 再送完整回應 | 是 | 同；事件由 runner 回呼推到緩衝 | 原樣搬（characterization 鎖行為）；改變：error 之後不再送 done（proto：error 是終止事件）、tool 完成時另送 `tool_result`（A9-D13）、client 斷線不再取消輪次（送出是 unary，A9-D12）、輪次逾時與取消以 error 結束 |
| 空訊息與預設訊息（A9-2b-4）：按鈕端點空訊息用預設文字（訂單、物流），其餘回 InvalidArgument | 有按鈕不讓使用者打字 | 疑似否：proto 把 `message` 標為必填，預設訊息不會被用到【推】 | 入口層驗證必填，沒有隱含預設 | 照搬進輪次執行器並移到 `text` 目錄（會出現在對話紀錄，是使用者面文字）；是否刪除交 A9-2c 決定（待決 1） |
| 退貨政策固定文案（M7 的細部） | 退貨政策不需要 LLM | 是 | 靜態內容放目錄 | 進 `text/*.json`（zh-TW、en）；固定回覆與 coordinator prompt 共用同一個 zh-TW key（`return_policy.facts`）；不建 session，finished 帶請求的 session_id（可為空）；與 content 模組的 `return-policy` key 的關係仍在 A9 結案（Q8） |
| 輪次事件暫存（A9-D12） | 單體是單一進程 | 是 | 小量、短命的串流事件放記憶體環形緩衝 | 每輪 1024 個事件、結束後保留 10 分鐘（常數 `DefaultBufferCapacity`、`DefaultRetention`，計畫沒有規定數字，待 Will 確認，待決 2）；輪次 ID 是隨機 UUID 且須買家相符 |
| 同對話單一輪次（A9-D14） | ADK `session.Service` 沒有條件寫入 | 是 | 輪次層序列化同一對話 | 記憶體佔用標記，結束時先釋放再發終止事件；`runner` 的 `AutoCreateSession` 關掉：帶了不存在的 session ID 回 `ErrSessionNotFound`（舊行為會用該 ID 自動建立一個沒有 `buyer_id` 的 session，tool 之後全部失敗；疑似缺陷） |
| 輪次逾時 | 外部呼叫要有 deadline（規則：沒有 deadline 的外部呼叫不允許） | 是 | 每輪有上限 | `DefaultTimeout` 2 分鐘，計畫沒有規定數字（待決 3）；舊服務只靠 LLM 呼叫自身的 30 秒 |

**待決**（計畫與 proto 沒有規定，已先以保守值實作，不阻擋 A9-2b）：1. 預設訊息是否刪除（A9-2c）；2. 緩衝容量與保留時間；3. 輪次逾時；4. `error` 事件的 `ErrorReason`：現有 enum 沒有「助理輪次失敗」的值，模組只給 `FailureKind`，handler 要新增 reason 或決定 UNSPECIFIED 的語意（A9-2c）；5. 疑似缺陷照搬未修，程式註解標「疑似缺陷」：狀態標籤表與 prompt 的狀態碼不一致、`track_my_package` 以狀態 20 當配送中、單筆物流詳情失敗靜默略過、整句空白的訊息被當成有內容。

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
|  |  |  |  |  |

## 與 ai-service 的差異

<!-- 資料沒有搬移（系統從未上線），schema 是新的 baseline。每一項對外看得到或 schema 上的差異一行。 -->
