# procurement 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

## 沿用盤點

ai-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| ID 方案（M1）：全 TSID BIGINT（Go 端產生），ADK 表用 TEXT | Java 需要認得 ID（舊服務互通） | 否：對 Java 只有 `order_no`、`prospect_id`、`store_id`、`buyer_id` 是外部鍵（`idrefs.csv:19`），run、request、candidate 等自己的實體沒有 Java 消費者【推】 | 自己的實體用 UUIDv7（row-identity ADR）；外部鍵維持 bigint 到最後一個 Java 消費者退場（同 A6 M1） | 實體 UUIDv7；buyer、store、prospect 引用維持 bigint |
| gRPC 內部服務、method policy、metadata 注入身分（M2） | 兩個進程之間要信任邊界 | 否：單體內 session 就是身分 | 入口層（`internal/httpapi`）從 session 取身分，模組公開 API 收 `buyer.ID`、`store.ID` 參數 | 刪 gRPC 與 metadata；method policy 改為 Connect 層 interceptor 宣告（同 `adminauth`） |
| ADK v2、GORM session service、pgx（M3），採購只把 ADK session 當 graph 的 checkpoint | 需要 ADK 的 agent loop 與 AG-UI 事件 | 部分：採購的狀態機由 run 帳本自持，ADK session 只當 graph 的 checkpoint【推】`graph_session.go` | 狀態機在業務表，agent loop 輕量；若留 ADK，用它的 pgx 原生儲存而非 GORM 第二套 ORM | A9-D5 決定保留 ADK、session 改 pgx 實作 `session.Service`，GORM 不進單體；採購的狀態仍在自己的業務表（A9-2 實作） |
| LLM SDK（M4）：Gemini 走 `google.golang.org/genai`；DeepSeek、OpenRouter 走 `openai-go/v3` 的 `/responses`；修圖手寫 HTTP | 需要多 provider 切換 | 部分：只有 `EXTRACTOR_MODEL` 一個開關；登記表 4 個、其中 gemma 是 `:free`（無 SLA）【量】 | 一個 `LLM` port 加 provider adapter；明確的 timeout、retry、budget；修圖也走同一個 port | A9-D10：只搬 Gemini、DeepSeek，不搬 OpenRouter；單一建構點與架構測試（AD5）在 A9-1c 落地，本模組只依賴該 port，不自建 client |
| 金鑰與設定（M5）：env 讀取（反射加 strict env 警告）；`GEMINI_API_KEY`、`DEEPSEEK_API_KEY`、`OPENROUTER_API_KEY`、`LLM_API_KEY` 等 | 單一進程讀 env | 否：單體有統一設定載入（`cmd/hawkr`） | 設定集中在 composition root；secret 不進 repo | 併入 `cmd/hawkr` 設定；`LLM_API_URL`、`LLM_API_KEY`、`LLM_MODEL` 全 repo 無讀取者，不搬（AD13） |
| timeout 與 retry（M6）：extractor 單次 30s 加 `retryingExtractor` 一次、SDK 自帶重試關掉、realizer 4s、narrator 3s、run 預算 `runDispatchBudget`、`candidateSearchBudget`、assistant 30s | 外部 provider 要有 deadline | 成立：每個外部呼叫都有 deadline【量】 | 同；加 provider 層熔斷與併發上限 | 保留並集中到 LLM 單一建構點（A9-1c）；timeout 常數與 `observability` 的直方圖 bucket 鏡像要保持同步（`observability/metrics.go` 用測試守） |
| prompt 儲存（M7）：全部以 Go 字串常數寫在程式裡（extractor、realizer、narrator、crop model） | 需要不經部署修改 prompt | 否：沒有 prompt 版本欄位 | prompt 進程式碼並隨版本控管，加 eval gate；後台可改的文案走 content 模組 | 留在程式碼；prompt 內的農產文案（「生鮮」「農」「食安」）隨 scope cut 改寫（Q4） |
| observability（M8）：OTel meter（24 個 instrument）加 Prometheus `promauto` gauge；告警規則在 repo 外 | 有 Prometheus、OTel pipeline | 單體：待定（計畫 A7 延後項說「單體告警管道」尚未建立） | 單一 metrics 入口、標籤封閉基數 | 沿用指標名（告警規則依賴），指標命名照 A9-D11（沿用 `ai_procurement_llm_*`）；等單體告警管道（Q10） |
| 預約制配額與 advisory lock 字串雜湊命名空間（M9）：`procurement_usage_reservations` | 兩個進程併發，需要先佔再扣 | 部分：單體仍多 pod；但可用單列 counter 加條件 UPDATE，不需要預約表與雜湊鎖 | 一列 counter 加條件 UPDATE（`WHERE used < limit`），失敗即拒絕；預約只在長呼叫才有意義 | A9-D9：改成一列計數器加條件 UPDATE，外部呼叫失敗時退回，不建預約表；額度數字照現在 config 搬（A9-4） |
| LISTEN/NOTIFY 作 run 事件喚醒（M10），每條 stream 一條專屬連線 | 多 pod 要跨進程喚醒 | 成立（單體多 pod），但連線數無上限（AD11） | 同；或 DB 輪詢（`last_event_seq`）加單連線 listener 扇出 | 保留；單體用一條共用 listener 扇出到 in-process channel 並設訂閱上限，測試守 DB 連線數不隨訂閱數增加（AD11；A9-3b；Q6） |
| 兩支 outbox（`pkg/outbox` Scheduler 加 claim lease CAS）（M11）：rfq publication、order provisioning | 要可靠地外送 MQ、gRPC | 否：單體規則 4、5：tx 內 `events.Publish` 加 `jobs.Enqueue`（River），provider 在 tx 外由 worker 呼叫 | River job 帶冪等鍵，重試與退避由 River 管 | 兩支 outbox 改 River job；`procurement.events` exchange 的舊形狀在 notification 搬完之前由 relay 維持（前例 `legacy/sellerrelay`、`socialrelay`；A9-D11；Q8） |
| 用 gRPC client 取別的服務資料（M12）：6 個 client | 資料在別的服務 | 部分：buyer、seller、media 已在單體；ordering、catalog 還在 Java | 模組公開 API；未搬的走 `internal/legacy/*` | buyer、seller、media 換成模組 API；ordering、catalog 經 `internal/legacy` 補 client，模組只依賴 port，不等 B8（A9-D11；Q3） |
| 生成檔 `category_alias_gen.go`（M14）：851 筆別名，由 LLM 從農產分類樹生成 | 分類樹穩定、別名準 | 否：針對農產 L2 分類（id 4–222）；seed 分類樹是 id 4–33 的小樹，分類 ID 不對得上【量】 | 召回用 embedding 或搜尋索引加人工 synonym；別名表隨分類樹資料化 | A9-D7：刪別名層，分類樹加人工同義詞召回（A9-3a）；離線 gate（`category_alias_gate_test.go`）的設計留 |
| 簡繁轉換 `opencc`（M16）：LLM 抽取結果強制轉繁體 | LLM 輸出語域不穩 | 成立（2026-08-15 實測案例） | 同 | 保留，併入採購 |
| fx 組裝、`pkglogger`（zap）、`syncutil.SafeGo`（M17） | 單進程 DI | 否：單體 composition root 手動接線 | 同單體規則 | 全改成 `cmd/hawkr` 接線；`go-pkg/{identity,tsid,outbox,mq,errors/bizcode,grpcclient}` 換成單體的 `platform` 對應 |
| bizcode 數字碼加 CodeMeta（M18）：例如 `9703` | 舊 REST 契約 | 否：新契約用 `ErrorReason` | `api/` 的 `ErrorReason` 加 Go 型別 | 全部轉 `ErrorReason`；`procurement_error_reason_registry_test.go` 已有註冊表概念，可沿用【量】 |
| 雙軌欄位 `controller_kind`（DURABLE、GRAPH）（M19） | 兩條執行路徑並存 | 否：`ClaimAndDispatchTurn` 已刪，欄位成了單值 | 不留 | 刪欄位或固定 `GRAPH`（AD12） |
| lazy reaper：讀取時回收過期 candidate set（M20） | 沒有背景 sweeper | 否：單體有 River | River periodic job 掃 `lease_expires_at`（索引已有） | 併入 sweeper（A9-3b；Q6）；A9-D6：sweeper 把逾時的 run 標失敗、買家重送，不重驅 |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
|  |  |  |  |  |

## 與 ai-service 的差異

<!-- 資料沒有搬移（系統從未上線），schema 是新的 baseline。每一項對外看得到或 schema 上的差異一行。 -->
