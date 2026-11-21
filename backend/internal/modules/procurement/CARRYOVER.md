# procurement 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

## 沿用盤點

ai-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| ID 方案（M1）：全 TSID BIGINT（Go 端產生），ADK 表用 TEXT | Java 需要認得 ID（舊服務互通） | 否：對 Java 只有 `order_no`、`prospect_id`、`store_id`、`buyer_id` 是外部鍵（`idrefs.csv:19`），run、request、candidate 等自己的實體沒有 Java 消費者【推】 | 自己的實體用 UUIDv7（row-identity ADR）；外部鍵維持 bigint 到最後一個 Java 消費者退場（同 A6 M1） | 實體 UUIDv7；buyer、store、prospect 引用維持 bigint |
| gRPC 內部服務、method policy、metadata 注入身分（M2） | 兩個進程之間要信任邊界 | 否：單體內 session 就是身分 | 入口層（`internal/httpapi`）從 session 取身分，模組公開 API 收 `buyer.ID`、`store.ID` 參數 | 刪 gRPC 與 metadata；method policy 改為 Connect 層 interceptor 宣告（同 `adminauth`） |
| ADK v2、GORM session service、pgx（M3），採購只把 ADK session 當 graph 的 checkpoint | 需要 ADK 的 agent loop 與 AG-UI 事件 | 部分：採購的狀態機由 run 帳本自持，ADK session 只當 graph 的 checkpoint【推】`graph_session.go` | 狀態機在業務表，agent loop 輕量；若留 ADK，用它的 pgx 原生儲存而非 GORM 第二套 ORM | A9-D5 決定保留 ADK、session 改 pgx 實作 `session.Service`，GORM 不進單體；採購的狀態仍在自己的業務表。A9-3a 已依 S7 抽出 `internal/platform/adksession`，採購在自己的 schema 建四張表（見下 S7 列） |
| LLM SDK（M4）：Gemini 走 `google.golang.org/genai`；DeepSeek、OpenRouter 走 `openai-go/v3` 的 `/responses`；修圖手寫 HTTP | 需要多 provider 切換 | 部分：只有 `EXTRACTOR_MODEL` 一個開關；登記表 4 個、其中 gemma 是 `:free`（無 SLA）【量】 | 一個 `LLM` port 加 provider adapter；明確的 timeout、retry、budget；修圖也走同一個 port | A9-D10：只搬 Gemini、DeepSeek，不搬 OpenRouter；單一建構點與架構測試（AD5）在 A9-1c 落地，本模組只依賴該 port，不自建 client |
| 金鑰與設定（M5）：env 讀取（反射加 strict env 警告）；`GEMINI_API_KEY`、`DEEPSEEK_API_KEY`、`OPENROUTER_API_KEY`、`LLM_API_KEY` 等 | 單一進程讀 env | 否：單體有統一設定載入（`cmd/hawkr`） | 設定集中在 composition root；secret 不進 repo | 併入 `cmd/hawkr` 設定；`LLM_API_URL`、`LLM_API_KEY`、`LLM_MODEL` 全 repo 無讀取者，不搬（AD13） |
| timeout 與 retry（M6）：extractor 單次 30s 加 `retryingExtractor` 一次、SDK 自帶重試關掉、realizer 4s、narrator 3s、run 預算 `runDispatchBudget`、`candidateSearchBudget`、assistant 30s | 外部 provider 要有 deadline | 成立：每個外部呼叫都有 deadline【量】 | 同；加 provider 層熔斷與併發上限 | 保留並集中到 LLM 單一建構點（A9-1c）；timeout 常數與 `observability` 的直方圖 bucket 鏡像要保持同步（`observability/metrics.go` 用測試守） |
| prompt 儲存（M7）：全部以 Go 字串常數寫在程式裡（extractor、realizer、narrator、crop model） | 需要不經部署修改 prompt | 否：沒有 prompt 版本欄位 | prompt 進程式碼並隨版本控管，加 eval gate；後台可改的文案走 content 模組 | 留在程式碼；prompt 內的農產文案（「生鮮」「農」「食安」）隨 scope cut 改寫（Q4） |
| observability（M8）：OTel meter（24 個 instrument）加 Prometheus `promauto` gauge；告警規則在 repo 外 | 有 Prometheus、OTel pipeline | 單體：待定（計畫 A7 延後項說「單體告警管道」尚未建立） | 單一 metrics 入口、標籤封閉基數 | 沿用指標名（告警規則依賴），指標命名照 A9-D11（沿用 `ai_procurement_llm_*`）；等單體告警管道（Q10） |
| 預約制配額與 advisory lock 字串雜湊命名空間（M9）：`procurement_usage_reservations` | 兩個進程併發，需要先佔再扣 | 部分：單體仍多 pod；但可用單列 counter 加條件 UPDATE，不需要預約表與雜湊鎖 | 一列 counter 加條件 UPDATE（`WHERE used < limit`），失敗即拒絕；預約只在長呼叫才有意義 | A9-D9：改成一列計數器加條件 UPDATE，外部呼叫失敗時退回，不建預約表；額度數字照現在 config 搬（A9-4）。A9-3a 已建 `procurement.usage` 與 `ConsumeUsage`／`RefundUsage`（`TestUsageCounterIsConditional`），接上送出路徑在 A9-3b |
| LISTEN/NOTIFY 作 run 事件喚醒（M10），每條 stream 一條專屬連線 | 多 pod 要跨進程喚醒 | 成立（單體多 pod），但連線數無上限（AD11） | 同；或 DB 輪詢（`last_event_seq`）加單連線 listener 扇出 | 保留；單體用一條共用 listener 扇出到 in-process channel 並設訂閱上限，測試守 DB 連線數不隨訂閱數增加（AD11；A9-3b；Q6） |
| 兩支 outbox（`pkg/outbox` Scheduler 加 claim lease CAS）（M11）：rfq publication、order provisioning | 要可靠地外送 MQ、gRPC | 否：單體規則 4、5：tx 內 `events.Publish` 加 `jobs.Enqueue`（River），provider 在 tx 外由 worker 呼叫 | River job 帶冪等鍵，重試與退避由 River 管 | 兩支 outbox 改 River job；`procurement.events` exchange 的舊形狀在 notification 搬完之前由 relay 維持（前例 `legacy/sellerrelay`、`socialrelay`；A9-D11；Q8） |
| 用 gRPC client 取別的服務資料（M12）：6 個 client | 資料在別的服務 | 部分：buyer、seller、media 已在單體；ordering、catalog 還在 Java | 模組公開 API；未搬的走 `internal/legacy/*` | buyer、seller、media 換成模組 API；ordering、catalog 經 `internal/legacy` 補 client，模組只依賴 port，不等 B8（A9-D11；Q3） |
| 生成檔 `category_alias_gen.go`（M14）：851 筆別名，由 LLM 從農產分類樹生成 | 分類樹穩定、別名準 | 否：針對農產 L2 分類（id 4–222）；seed 分類樹是 id 4–33 的小樹，分類 ID 不對得上【量】 | 召回用 embedding 或搜尋索引加人工 synonym；別名表隨分類樹資料化 | A9-D7：刪別名層，分類樹加人工同義詞召回。A9-3a 已實作 `internal/resolver`：原詞有命中就不查同義詞（AD3，`TestSynonymNeverOverridesTheBuyersOwnTerm`），`synonyms.json` 空表起步；離線 gate（`category_alias_gate_test.go`）的設計留 |
| 簡繁轉換 `opencc`（M16）：LLM 抽取結果強制轉繁體 | LLM 輸出語域不穩 | 成立（2026-08-15 實測案例） | 同 | 保留，併入採購 |
| fx 組裝、`pkglogger`（zap）、`syncutil.SafeGo`（M17） | 單進程 DI | 否：單體 composition root 手動接線 | 同單體規則 | 全改成 `cmd/hawkr` 接線；`go-pkg/{identity,tsid,outbox,mq,errors/bizcode,grpcclient}` 換成單體的 `platform` 對應 |
| bizcode 數字碼加 CodeMeta（M18）：例如 `9703` | 舊 REST 契約 | 否：新契約用 `ErrorReason` | `api/` 的 `ErrorReason` 加 Go 型別 | 全部轉 `ErrorReason`；`procurement_error_reason_registry_test.go` 已有註冊表概念，可沿用【量】（隨 A9-3b、3c 的 handler） |
| 雙軌欄位 `controller_kind`（DURABLE、GRAPH）（M19） | 兩條執行路徑並存 | 否：`ClaimAndDispatchTurn` 已刪，欄位成了單值 | 不留 | 刪欄位或固定 `GRAPH`（AD12） |
| lazy reaper：讀取時回收過期 candidate set（M20） | 沒有背景 sweeper | 否：單體有 River | River periodic job 掃 `lease_expires_at`（索引已有） | 併入 sweeper（A9-3b；Q6）；A9-D6：sweeper 把逾時的 run 標失敗、買家重送，不重驅 |
| 候選集同時是「一批候選」與「一次 run」，帶 lease、`fencing_token`、`policy_version`（S1） | 同一工作可能被第二個 owner 重新領走，舊 owner 的晚到寫入要被擋 | 否：A9-D6 sweeper 標失敗、買家重送，不重驅，沒有第二個 owner | run 狀態只放單一帳本，結果容器只是結果；終態寫入用條件式 UPDATE | `candidate_sets` 一次搜尋一列，只有 `request_revision_id`、`buyer_id`、`client_request_id`；run 狀態在 `agent_runs`，`agent_runs.candidate_set_id`／`publication_id` 是 typed 可空外鍵，取代 `domain_ref_*` 字串多型；刪 lease、`fencing_token`、`policy_version`、`platform_lane_status`、`failure_reason`；冪等鍵 `(request_revision_id, client_request_id)`；失敗後買家重送就是新的 run 與新的候選集 |
| `requests`、`request_revisions`、`answer_patches` 的模型：「現行 revision」三種表示法、`branch_id`、`slots_hash`、`requests.version`（S2） | 回上一題（Rewind）後舊分支不得再被當成現行，歷史要可稽核 | 成立（`Rewind` 在契約）；但三個表示法同一件事 | append-only 歷史加單一現行表示法 | 只留 `request_revisions_active`（partial unique `WHERE status = 'ACTIVE'`）；`requests` 不放指標與 version，轉移用條件式 UPDATE（`SetRequestStatus`）；`branch_id` 刪（`rg branch_id` 只有稽核用的寫入，沒有讀取者【量】）；`slots_hash` 刪，需要時由 Go 從欄位算；`answer_patches` 改名 `turns`：具名文字欄 `buyer_message`、`assistant_reply`，`steps`、`recorded_this_turn` 留 JSONB，`base_revision_id` 刪（用 `parent_revision_id`） |
| JSON 與 typed 欄位的界線、`schema_version`（S3） | 形狀不固定，或不值得為它拆表；已存 payload 的形狀會演進 | 部分：`Slots`、價格區間、`buyer_note` 在 proto 是封閉形狀；系統從未上線，舊資料不搬 | 封閉形狀用 typed 欄位並加 CHECK，JSONB 只留彈性結構；hash 要知道 preimage 版本才能重算 | slots 九欄、價格區間、`buyer_note`、賣家回價的金額與備註、`response_deadline` 全是具名欄位，文字欄用 `trimtext.Check` 並登記 `trimmedColumns`；JSONB 只留 `turns.steps`、`recorded_this_turn`、`candidates.matched_facts`、`unverifiable_facts`、`seller_response_drafts.payload`、run 事件與中斷的 payload、artifact 的 data；事件 payload 存 proto JSON，刪 `schema_version`、`input_schema_version`、`response_schema_version`；`rfq_publication_approvals.schema_version` 與 `rfq_publications.schema_version` 保留（hash 的 preimage 版本） |
| 冪等儲存：6 張 `*_commands` 加 5 張實體的 `client_request_id`（S4） | 同一 `(呼叫者, client_request_id)` 重送回第一次結果，內容不同回 `INVALID_ARGUMENT`（`procurement.proto` 檔頭） | 成立 | 鍵放在被建立的實體上（同 transaction、不過期）；本身是目標狀態的方法天然冪等 | 刪 `conversation_commands`、`rfq_publication_commands`、`seller_response_commands`、`agent_run_commands`；**不建共用收據表**：逐支判定（下表）後沒有任何一支剩下，建一張空表違反必要條件 |
| 核准、發布、接受報價的 hash 與快照欄位重複（S5） | 被參照的列可變或跨服務，複製才有必要 | 否：同一個 schema，被參照的列不可變（預覽、快照、版本只 INSERT） | 只存參照，hash 由 Go 從不可變的列重算 | `rfq_publication_approvals`、`rfq_publications` 只存參照與 `schema_version`；`price_previews.version`、`price_preview_version` 欄位刪；接受單只留接受當下的 `seller_response_payload_hash`、`confirmed_rfq_payload_hash`（買家 CAS 的對象，留作稽核）與凍結的成交價三金額。proto 的 `PreviewRfqPriceResponse.version`、`ConfirmRfqSnapshotRequest.price_preview_version` 沒改（A9-3a 範圍列的 proto 變更不含它們）：由伺服器填 `price_preview_id` 衍生值，A9-3b 開工時由 Will 決定是否從 proto 刪 |
| 沒有寫入者的狀態值、接受單鎖死（S6） | CHECK 把合法值釘在資料庫 | 成立，但舊寫法收「完整值域」而不是會寫的子集 | CHECK 只放有寫入者的值，新值隨它的寫入者加 | `rfq_publications.state` 只收 `PUBLISHED`；`seller_responses.write_closed_reason` 只收 `DECLINED`；接受單 `state` 只收 `ACTIVE`、`ORDER_FAILED`；proto `PublicationState` 刪 `SUPERSEDED`、`CANCELLED`（`reserved`）。A9-D19 修訂：建單的 River job 重試用盡時（A9-3c）acceptance 轉 `ORDER_FAILED` 並解除 live 鎖（`uq` 只擋 `state = 'ACTIVE'`），買家可再接受；`TestLiveLockOnlyBlocksActiveAcceptances` |
| 採購的 ADK session 存放處（S7） | 圖形的暫停與回答分屬兩個 run，session 要跨越兩者存活 | 成立：`graph_session.go` 以 request ID 當 session ID，掛起狀態靠 ADK workflow 在 session events 內保存的 pause 資訊【量，A9-3a 讀 `graph_session.go`、`graph_dispatch.go`：`resumeGraphTurn` 以 `FunctionResponse` 加 workflow 的 pause ID 重入，`ResumeRunner` 關掉 AutoCreateSession，checkpoint 被清掉就回錯】；不只是 interrupt payload，需要 `session.Service` 的完整語意 | 無領域知識的基礎設施放 platform，表放各模組自己的 schema | 抽出 `internal/platform/adksession`，`New(db, schema)`；assistant 傳 `assistant`、procurement 傳 `procurement`，各自建 `sessions`、`events`、`app_states`、`user_states`；`TestADKServiceConformanceOnProcurementSchema` 在 procurement 的 schema 上跑 ADK 的 `RunServiceTests`；`conversations.adk_session_id` 刪（session ID 就是 request ID 的字串） |
| 人工同義詞與分類樹的存放（S8）、分類樹的 TTL 快取（B8，原清單漏列） | 分類樹經遠端讀取，每次解析都讀太慢 | 成立（分類服務在舊服務，經 `TaxonomyReader` port） | 單一 in-process TTL 快取，讀取有 timeout；同義詞是變動慢的小資料 | 同義詞放 `internal/resolver/synonyms.json`（`go:embed`，格式由 `LoadSynonyms` 驗證，空表起步，不沿用農產內容）；分類樹 `resolver.TreeTTL`（30 秒）、`LoadTimeout`（30 秒），stale-while-revalidate：有快取樹時過期後仍立即回舊樹並在背景重讀（同時只一次讀取）；沒有快取樹時所有並行呼叫共用同一次讀取，等待者可因自己的 ctx 結束離開；讀取不持鎖、不隨呼叫者的 ctx 取消，panic 轉成錯誤；失敗後 `RetryBackoff`（10 秒）內不再發起讀取（mutex＋done channel 自寫，不增加依賴；沒有舊樹才回錯誤）。這份手寫快取在 catalog 搬進單體（B8）時刪除，改用模組 API；catalog 搬進單體後以模組 API 取代 legacy 呼叫並重評 |
| 候選的 `display_name` 與可見性（S9） | 賣家改名或換圖後，候選要維持當時的樣子 | 否：契約已用共用 `StoreCard`（讀取時組裝）；沒公開過的店名不能被讀出 | 名稱讀取時組裝，只快照業務判定依據 | 刪 `candidates.display_name`、`seller_name`、`logo_key`、`store_code`（`replicas.csv:50` 已填 `query`）與 proto `Candidate.display_name`（`reserved`）；候選只收 OPEN 的店（`candidate.KeepOpen`，經 `seller.ExposableStores` 分批，fail-closed）；`TestCandidatesHaveNoStoreNameColumns`；`ExecuteRfqPublication` 對 DIRECT_RECIPIENTS 在 transaction 內再檢一次店家狀態屬 A9-3c |
| 買家關閉後採購資料的去向（S10） | 個資在不再需要時刪除，交易紀錄依法保留 | 成立（買家自由文字會轉呈賣家） | 刪個資、匿名保留交易紀錄 | `retention.OnBuyerDeleted` 訂閱 `BuyerDeleted`（同 transaction，`cmd/hawkr` 在 serve 與 worker 訂閱）：刪對話、輪次、run、候選集、報價清單、額度、ADK session、沒發布過的需求；已發布的需求與預覽、快照、核准、發布、接受單的 `buyer_id` 設 NULL，邀請與賣家回價原樣保留；保留期跟詢價的生命週期，不另訂（Will 2026-11-21）；`TestBuyerDeletedRemovesBuyerDataAndAnonymizesPublishedRecords`、原子性 `TestBuyerDeletedIsAtomicWhenALaterStepFails` |
| selection 與 conversation（S11） | selection 與 candidate 1:1 | 是 1:1 | 折入 candidate | selection 併入 `candidates.selected`、`selected_at`（一表少三個冗餘 id）；conversation 與 request 維持兩張表（契約有兩個獨立 ID，刪對話不刪需求） |

## 冪等鍵逐支判定（S4）

`api/hawkr/procurement/v1alpha1/procurement.proto` 帶 `client_request_id` 的 15 支 RPC。「建立型」的鍵放在被建立的實體上（`(buyer_id, client_request_id)` 唯一，重送時 `ON CONFLICT DO NOTHING` 回 0 列、改讀既有的實體並比對內容）；「目標狀態」的方法本身重送結果相同，不建收據。

| RPC | 判定 | 鍵放哪 |
|---|---|---|
| `SubmitTurn` | 建立型（run） | `agent_runs (buyer_id, client_request_id)` |
| `StartCandidateSearch` | 建立型（候選集與 run） | `candidate_sets (request_revision_id, client_request_id)`、`agent_runs` |
| `SubmitAnswer`、`Rewind` | 建立型（輪次與新 revision） | `turns (buyer_id, client_request_id)` |
| `PreviewRfqPrice` | 建立型 | `price_previews (buyer_id, client_request_id)` |
| `ConfirmRfqSnapshot` | 建立型 | `confirmed_rfq_snapshots (buyer_id, client_request_id)` |
| `PreviewRfqPublication` | 建立型（核准） | `rfq_publication_approvals (buyer_id, client_request_id)` |
| `ExecuteRfqPublication` | 建立型（發布） | `rfq_publications (buyer_id, client_request_id)`；一個核准最多一次發布 |
| `AcceptSellerQuote` | 建立型 | `buyer_quote_acceptances (buyer_id, client_request_id)` |
| `ResumeRun` | 建立型（接續的 run） | `agent_runs` |
| `SubmitSellerResponse` | 建立型（回價版本） | `seller_response_versions (seller_response_id, client_request_id)` |
| `RenameConversation` | 目標狀態（標題設為指定值） | 不建收據 |
| `DeleteConversation` | 目標狀態（已刪除再刪不再更新，`deleted_at IS NULL` 條件） | 不建收據 |
| `WithdrawSellerResponse` | 目標狀態（撤回的版本是指定值，已撤回再撤回結果相同） | 不建收據 |
| `DeclineRfqPublication` | 目標狀態（`write_closed_reason = 'DECLINED'`） | 不建收據 |

目標狀態那四支，契約仍要求「內容不同回 `INVALID_ARGUMENT`」：請求除 ID 之外只有 `client_request_id`，沒有可以不同的內容；若 A9-3c 的請求加了內容欄位，要回頭為它補鍵。

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位。新的對外 API 照〈API 設計規範（Connect）〉；handler 在 A9-3b、3c 掛載，下表的「決定」是 3a 已能從 SSOT 定的部分，「3b／3c 定」的由該步開工時補。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 認證與身分（method policy、metadata 注入身分，M2） | 兩個進程之間要信任邊界 | 否：單體內 session 就是身分 | 入口層從 session 取身分，模組公開 API 收 `buyer.ID`、`store.ID` | Connect interceptor 宣告買家與賣家方法的權限（買家 service 帳號沒有買家角色回 `PERMISSION_DENIED`，不屬於呼叫者的需求、對話、run 回 `NOT_FOUND`，見 proto 檔頭）；買家面的 store 查詢一律帶 `buyer_id` 條件（`GetRequestForBuyer`、`GetRunForBuyer`、`GetPublication`、`ListRevisions` 等；沒有 `buyer_id` 欄的子表經父表 join 或 EXISTS，`ClaimApprovalForExecute` 另檢查 `expires_at`）。免 `buyer_id` 的只有系統內部與賣家面：`MarkOrderCreated`、`MarkOrderFailed`（建單 worker 以 job 引數的接受單 id 定位，買家關閉後接受單的 `buyer_id` 會是 NULL）、`MarkInviteSent`、`MarkInviteFailed`（發送 worker 以邀請 id 定位）、`MarkApprovalSucceeded`（`(id, version, state)` 已是 CAS，擁有者在 `ClaimApprovalForExecute` 驗過；帶 `buyer_id` 的話，BuyerDeleted 把它設 NULL 後核准會卡在 EXECUTING）、`InsertInvite`、`InsertSellerResponse`、`GetSellerResponse`（賣家面，以 `store_id` 定位；A9-3c 起加賣家身分條件）。BuyerDeleted 之後 `buyer_id` 為 NULL，現有的 `GetPublication` 等買家面查詢讀不到已發布的詢價，賣家與系統目前沒有可讀 publication 的查詢：A9-3c 加賣家面查詢（以 invite 的 `store_id` 限定） |
| 錯誤（bizcode 加 CodeMeta，M18） | 舊 REST 契約 | 否 | `ErrorReason` 加 Go 型別 | 全部轉 `ErrorReason`；3b、3c 補註冊表測試 |
| 分頁 | 清單有大小上限 | 成立：`ListConversations` 現有 `page_size` 上限、賣家與買家清單依契約 | keyset 分頁 | `ListConversations` 以 `(updated_at, id)` 排序、`LIMIT`（3b 定 keyset 游標）；其餘清單依 proto |
| 串流與逾時（舊 SSE 加 120 秒單輪上限） | 單體不限串流總時長 | 部分：website 經 Vercel rewrite 上限 120 秒 | server streaming 在 100 秒正常結束，client 以游標續看 | A9-D4、A9-D12：`SubmitTurn` unary、`WatchRun` 單一訂閱；3b 實作，單輪上限 120 秒由單體自己設 |
| 請求大小 | 防止超大輸入 | 成立 | 欄位長度上限寫在契約與 DB CHECK | 買家訊息 2000 字元、`buyer_note` 500 字元；DB 以 `char_length` CHECK 守第二道（`turns.buyer_message`、`confirmed_rfq_snapshots.buyer_note`、`agent_run_inputs.buyer_message`），入口驗證 3b、3c |
| 限流與配額 | 保護 LLM 成本 | 成立 | 計數器加條件 UPDATE | A9-D9：`usage` 一列計數器；每位買家同時進行的輪次等記憶體上限（A9-D18 是助理的）採購的另訂，3b 定 |
| 快取 | 分類樹讀取成本 | 成立 | TTL | 見 B8 列（`resolver.TreeTTL`） |
| trace | 跨程序追蹤 | 否：單體 | 單一 trace | `agent_run_events.trace_id` 不建（沒有讀取者，3b 若要再補） |
| 下游呼叫的逾時與重試 | 外部 provider 要有 deadline | 成立 | 明確 timeout、retry | LLM 走 `internal/platform/llm`（M6）；分類樹讀取 `LoadTimeout`；ordering 建單的 River job 退避與重試用盡（轉 `ORDER_FAILED`）在 3c |

## 與 ai-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline。每一項對外看得到或 schema 上的差異一行。

- 表：31 張收斂為 23 張業務表加 4 張 ADK session 表；刪 7 張（`conversation_commands`、`rfq_publication_commands`、`rfq_publication_outbox`、`seller_response_commands`、`order_provisioning_outbox`、`agent_run_commands`、`usage_reservations`），`candidate_selections` 併入 `candidates`，`answer_patches` 改為 `turns`。
- ID：自己的實體 UUIDv7（`uuid`），`buyer_id`、`store_id`、`order_no` 維持舊值；原本的 TSID BIGINT 不保留。
- 表名：加 `procurement.` schema 前綴、去掉 `procurement_` 前綴。
- 沒有 EXTERNAL lane、`prospect_id`、站外業者驗證四欄（A9-D1）；`candidate_leads`（AD4）、`controller_kind`（AD12）不建；`run_type` 刪 `QUOTE_APPROVAL`。
- 候選：無 `display_name`、`seller_name`、`logo_key`、`store_code`、`brand_*`（S9）；`selected`、`selected_at` 在 `candidates`（S11）；無 `policy_version`、lease、`fencing_token`（S1）。
- 需求：無 `active_revision_id`、`version`、`branch_id`、`slots_hash`、`slots_json`（slots 改九個具名欄位，S2、S3）；`revision.status` 只有 `ACTIVE`、`SUPERSEDED`。
- 對話：無 `adk_session_id`；`turns` 取代 `answer_patches`，訊息原文是具名欄位。
- 詢價：核准、發布無 hash 與快照複本、無 `candidate_set_id`、`candidate_set_policy_version`、`price_preview_version`；預覽與快照無 `version`、`payload_hash`、`canonical_payload_json`（hash 由 Go 重算，S3、S5）；接受單無 `version`、`request_hash`、`lane`。
- 狀態值收斂（S6）：發布 `PUBLISHED`；`write_closed_reason` 只有 `DECLINED`；接受單 `state` 為 `ACTIVE`、`ORDER_FAILED`（新增，A9-D19 修訂：建單失敗解除 live 鎖）；邀請 `delivery_state` 無 `NOT_SENT`。proto `PublicationState` 刪 `SUPERSEDED`、`CANCELLED`。
- 回價：版本無驗證欄位與 hash 複本；賣家回價身分列無 `buyer_id`、`version`；品相欄位（`offered_quality_*`、`conversion_basis`）仍是契約必填，是否屬 scope cut 殘留由 A9-3c 開工時交 Will 決定。
- run：無 `controller_kind`、`version`、`domain_ref_type`、`domain_ref_id`（改 typed 外鍵）；`agent_run_inputs` 無 `retention_class`、`input_schema_version`，買家原文是具名欄位；`agent_run_events` 主鍵改 `(run_id, seq)`、無 `schema_version`、`trace_id`；`agent_run_interrupts` 無 `expires_at`、`response_schema_version`（3b 確認是否有寫入者）、`run_artifacts` 無 `domain_ref_*`、`updated_at`。
- 額度：`usage.usage_count` 改名 `used`，條件 UPDATE，無預約表（A9-D9）。
- 買家關閉：新增 `BuyerDeleted` 訂閱者，刪買家資料、匿名保留賣家可見的紀錄（S10）；ai-service 沒有任何刪帳處理。
- 候選同義詞：不沿用 851 筆農產別名與 `synonyms` map，空表起步（S8、AD3）。
- 變動：`procurement.proto` 的 `Candidate.display_name` 與沒有寫入者的 `PublicationState` 值刪除（上線前不留 `reserved`）。
- 待補（run 帳本，A9-D6）：`agent_runs`、`agent_run_inputs`、`agent_run_events` 等表在 A9-3a 建好，但讀寫它們的 query 到 A9-3b 才有（只有 `InsertRun`、`InsertRunInput`、`ConsumeUsage` 等額度與建立）。觸發條件：A9-3b 開工時補 run 表的 integration test。
- 待決（FK 的 ON DELETE）：run 帳本相關表指向 `requests`、`conversations` 的外鍵，ON DELETE 行為待 A9-3b 開工時決定（BuyerDeleted 的刪除順序依賴它），並補測試。
- 待刪（手寫 TTL 快取）：`resolver` 的手寫快取在 catalog 搬進單體（B8）時刪除，改用模組 API，見 B8 列。
- 守門：`retention` 的清單由 `TestRetentionCoversEveryBuyerIdTable` 對 schema 檢查（含 `buyer_id` 的表必須登記或明列豁免與理由）；`adksession` 兩模組的表由 `TestAdkTablesHaveTheSameShapeInEveryModuleSchema` 比對形狀。
