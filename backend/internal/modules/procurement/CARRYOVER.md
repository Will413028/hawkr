# procurement 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

## 沿用盤點

ai-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| ID 方案（M1）：全 TSID BIGINT（Go 端產生），ADK 表用 TEXT | Java 需要認得 ID（舊服務互通） | 否：對 Java 只有 `order_no`、`prospect_id`、`store_id`、`buyer_id` 是外部鍵（`idrefs.csv:19`），run、request、candidate 等自己的實體沒有 Java 消費者【推】 | 自己的實體用 UUIDv7（row-identity ADR）；外部鍵維持 bigint 到最後一個 Java 消費者退場（同 A6 M1） | 實體 UUIDv7；buyer、store、prospect 引用維持 bigint |
| gRPC 內部服務、method policy、metadata 注入身分（M2） | 兩個進程之間要信任邊界 | 否：單體內 session 就是身分 | 入口層（`internal/httpapi`）從 session 取身分，模組公開 API 收 `buyer.ID`、`store.ID` 參數 | 刪 gRPC 與 metadata；method policy 改為 Connect 層 interceptor 宣告（同 `adminauth`） |
| ADK v2、GORM session service、pgx（M3），採購不使用 ADK session | 需要 ADK 的 agent loop 與 AG-UI 事件 | 部分：採購的狀態機由 run 帳本自持，中斷 payload 是接續的唯一狀態（A9-D24(2)），不用 ADK 圖的 checkpoint | 狀態機在業務表，agent loop 輕量；若留 ADK，用它的 pgx 原生儲存而非 GORM 第二套 ORM | A9-D5 決定保留 ADK、session 改 pgx 實作 `session.Service`，GORM 不進單體；採購的狀態仍在自己的業務表。採購不使用 ADK session（見下 S7 列） |
| LLM SDK（M4）：Gemini 走 `google.golang.org/genai`；DeepSeek、OpenRouter 走 `openai-go/v3` 的 `/responses`；修圖手寫 HTTP | 需要多 provider 切換 | 部分：只有 `EXTRACTOR_MODEL` 一個開關；登記表 4 個、其中 gemma 是 `:free`（無 SLA）【量】 | 一個 `LLM` port 加 provider adapter；明確的 timeout、retry、budget；修圖也走同一個 port | A9-D10：只搬 Gemini、DeepSeek，不搬 OpenRouter；單一建構點與架構測試（AD5）在 A9-1c 落地，本模組只依賴該 port，不自建 client |
| 金鑰與設定（M5）：env 讀取（反射加 strict env 警告）；`GEMINI_API_KEY`、`DEEPSEEK_API_KEY`、`OPENROUTER_API_KEY`、`LLM_API_KEY` 等 | 單一進程讀 env | 否：單體有統一設定載入（`cmd/hawkr`） | 設定集中在 composition root；secret 不進 repo | 併入 `cmd/hawkr` 設定；`LLM_API_URL`、`LLM_API_KEY`、`LLM_MODEL` 全 repo 無讀取者，不搬（AD13） |
| timeout 與 retry（M6）：extractor 單次 30s 加 `retryingExtractor` 一次、SDK 自帶重試關掉、realizer 4s、narrator 3s、run 預算 `runDispatchBudget`、`candidateSearchBudget`、assistant 30s | 外部 provider 要有 deadline | 成立：每個外部呼叫都有 deadline【量】 | 同；加 provider 層熔斷與併發上限 | 保留並集中到 LLM 單一建構點（A9-1c）；timeout 常數與 `observability` 的直方圖 bucket 鏡像要保持同步（`observability/metrics.go` 用測試守） |
| prompt 儲存（M7）：全部以 Go 字串常數寫在程式裡（extractor、realizer、narrator、crop model） | 需要不經部署修改 prompt | 否：沒有 prompt 版本欄位 | prompt 進程式碼並隨版本控管，加 eval gate；後台可改的文案走 content 模組 | 留在程式碼；prompt 內的農產文案（「生鮮」「農」「食安」）隨 scope cut 改寫（Q4） |
| observability（M8）：OTel meter（24 個 instrument）加 Prometheus `promauto` gauge；告警規則在 repo 外 | 有 Prometheus、OTel pipeline | 單體：待定（計畫 A7 延後項說「單體告警管道」尚未建立） | 單一 metrics 入口、標籤封閉基數 | 沿用指標名（告警規則依賴），指標命名照 A9-D11（沿用 `ai_procurement_llm_*`）；等單體告警管道（Q10） |
| 預約制配額與 advisory lock 字串雜湊命名空間（M9）：`procurement_usage_reservations` | 兩個進程併發，需要先佔再扣 | 部分：單體仍多 pod；但可用單列 counter 加條件 UPDATE，不需要預約表與雜湊鎖 | 一列 counter 加條件 UPDATE（`WHERE used < limit`），失敗即拒絕；預約只在長呼叫才有意義 | A9-D9：改成一列計數器加條件 UPDATE，外部呼叫失敗時退回，不建預約表；額度數字照現在 config 搬（A9-4）。A9-3a 已建 `procurement.usage` 與 `ConsumeUsage`／`RefundUsage`（`TestUsageCounterIsConditional`），接上送出路徑在 A9-3b |
| LISTEN/NOTIFY 作 run 事件喚醒（M10），每條 stream 一條專屬連線 | 多 pod 要跨進程喚醒 | 成立（單體多 pod），但連線數無上限（AD11） | 同；或 DB 輪詢（`last_event_seq`）加單連線 listener 扇出 | 保留；`run.Hub` 一條專用 LISTEN 連線（channel `procurement_run`，payload 是 run ID）扇出到 in-process channel，每程序上限 500 條訂閱（A9-D20 R5），連線數不隨訂閱數增加（`TestSubscriptionsShareOneListenConnection`，待 CI）；重連後喚醒全部訂閱者重查 |
| 兩支 outbox（`pkg/outbox` Scheduler 加 claim lease CAS）（M11）：rfq publication、order provisioning | 要可靠地外送 MQ、gRPC | 否：單體規則 4、5：tx 內 `events.Publish` 加 `jobs.Enqueue`（River），provider 在 tx 外由 worker 呼叫 | River job 帶冪等鍵，重試與退避由 River 管 | 兩支 outbox 改 River job；`procurement.events` exchange 的舊形狀在 notification 搬完之前由 relay 維持（前例 `legacy/sellerrelay`、`socialrelay`；A9-D11；Q8） |
| 用 gRPC client 取別的服務資料（M12）：6 個 client | 資料在別的服務 | 部分：buyer、seller、media 已在單體；ordering、catalog 還在 Java | 模組公開 API；未搬的走 `internal/legacy/*` | buyer、seller、media 換成模組 API；ordering、catalog 經 `internal/legacy` 補 client，模組只依賴 port，不等 B8（A9-D11；Q3） |
| 生成檔 `category_alias_gen.go`（M14）：851 筆別名，由 LLM 從農產分類樹生成 | 分類樹穩定、別名準 | 否：針對農產 L2 分類（id 4–222）；seed 分類樹是 id 4–33 的小樹，分類 ID 不對得上【量】 | 召回用 embedding 或搜尋索引加人工 synonym；別名表隨分類樹資料化 | A9-D7：刪別名層，分類樹加人工同義詞召回。A9-3a 已實作 `internal/resolver`：原詞有命中就不查同義詞（AD3，`TestSynonymNeverOverridesTheBuyersOwnTerm`），`synonyms.json` 空表起步；離線 gate（`category_alias_gate_test.go`）的設計留 |
| 簡繁轉換 `opencc`（M16）：LLM 抽取結果強制轉繁體 | LLM 輸出語域不穩 | 成立（2026-08-15 實測案例） | 同 | 不搬（A9-D28(5)）：抽取結果不做簡轉繁，不新增依賴 |
| fx 組裝、`pkglogger`（zap）、`syncutil.SafeGo`（M17） | 單進程 DI | 否：單體 composition root 手動接線 | 同單體規則 | 全改成 `cmd/hawkr` 接線；`go-pkg/{identity,tsid,outbox,mq,errors/bizcode,grpcclient}` 換成單體的 `platform` 對應 |
| bizcode 數字碼加 CodeMeta（M18）：例如 `9703` | 舊 REST 契約 | 否：新契約用 `ErrorReason` | `api/` 的 `ErrorReason` 加 Go 型別 | 全部轉 `ErrorReason`；`procurement_error_reason_registry_test.go` 已有註冊表概念，可沿用【量】（隨 A9-3b、3c 的 handler） |
| 雙軌欄位 `controller_kind`（DURABLE、GRAPH）（M19） | 兩條執行路徑並存 | 否：`ClaimAndDispatchTurn` 已刪，欄位成了單值 | 不留 | 刪欄位或固定 `GRAPH`（AD12） |
| lazy reaper：讀取時回收過期 candidate set（M20） | 沒有背景 sweeper | 否：單體有 River | River periodic job 掃 `lease_expires_at`（索引已有） | `run.Reconciler`（River 週期 job，每分鐘）對帳 run 與它的 River job，job 已死而 run 未收斂的轉 FAILED／`WORKER_LOST`（退額度）或 CANCELLED，並同交易寫 `finished` 事件，不重驅（A9-D6、A9-D22）；沒有時間門檻；candidate set 沒有 lease 與逾時回收（S1），一次搜尋一個 run，job 死亡由 run 的 reconciler 涵蓋 |
| 候選集同時是「一批候選」與「一次 run」，帶 lease、`fencing_token`、`policy_version`（S1） | 同一工作可能被第二個 owner 重新領走，舊 owner 的晚到寫入要被擋 | 否：A9-D6／A9-D22 reconciler 標失敗、買家重送，不重驅，沒有第二個 owner | run 狀態只放單一帳本，結果容器只是結果；所有狀態轉換經 `Ledger.Transition` 的守衛 UPDATE（`WHERE status IN (允許來源)`，A9-D22） | `candidate_sets` 一次搜尋一列，只有 `request_revision_id`、`buyer_id`；run 狀態在 `agent_runs`，`agent_runs.candidate_set_id` 是 typed 可空外鍵（ExecuteRfqPublication 不建 run，沒有 `publication_id`，A9-D31 Q3），取代 `domain_ref_*` 字串多型；刪 lease、`fencing_token`、`policy_version`、`platform_lane_status`、`failure_reason`；冪等鍵在 run（`agent_runs (buyer_id, client_request_id)`，A9-D29）；失敗後買家重送就是新的 run 與新的候選集 |
| `requests`、`request_revisions`、`answer_patches` 的模型：「現行 revision」三種表示法、`branch_id`、`slots_hash`、`requests.version`（S2） | 回上一題（Rewind）後舊分支不得再被當成現行，歷史要可稽核 | 成立（`Rewind` 在契約）；但三個表示法同一件事 | append-only 歷史加單一現行表示法 | 只留 `request_revisions_active`（partial unique `WHERE status = 'ACTIVE'`）；`requests` 不放指標與 version，轉移用條件式 UPDATE（`SetRequestStatus`）；`branch_id` 刪（`rg branch_id` 只有稽核用的寫入，沒有讀取者【量】）；`slots_hash` 刪，需要時由 Go 從欄位算；`answer_patches` 改名 `turns`：具名文字欄 `buyer_message`、`assistant_reply`，`steps`、`recorded_this_turn` 留 JSONB，`base_revision_id` 刪（用 `parent_revision_id`） |
| JSON 與 typed 欄位的界線、`schema_version`（S3） | 形狀不固定，或不值得為它拆表；已存 payload 的形狀會演進 | 部分：`Slots`、價格區間、`buyer_note` 在 proto 是封閉形狀；系統從未上線，舊資料不搬 | 封閉形狀用 typed 欄位並加 CHECK，JSONB 只留彈性結構；hash 要知道 preimage 版本才能重算 | slots 九欄、價格區間、`buyer_note`、賣家回價的金額與備註、`response_deadline` 全是具名欄位，文字欄用 `trimtext.Check` 並登記 `trimmedColumns`；JSONB 只留 `turns.steps`、`recorded_this_turn`、`seller_response_drafts.payload`、run 事件與中斷的 payload、artifact 的 data；事件 payload 存 proto JSON，刪 `schema_version`、`input_schema_version`、`response_schema_version`；`rfq_publication_approvals.schema_version` 與 `rfq_publications.schema_version` 保留（hash 的 preimage 版本） |
| 冪等儲存：6 張 `*_commands` 加 5 張實體的 `client_request_id`（S4） | 同一 `(呼叫者, client_request_id)` 重送回第一次結果，內容不同回 `INVALID_ARGUMENT`（`procurement.proto` 檔頭） | 成立 | 鍵放在被建立的實體上（同 transaction、不過期）；本身是目標狀態的方法天然冪等 | 刪 `conversation_commands`、`rfq_publication_commands`、`seller_response_commands`、`agent_run_commands`；**不建共用收據表**：逐支判定（下表）後沒有任何一支剩下，建一張空表違反必要條件 |
| 核准、發布、接受報價的 hash 與快照欄位重複（S5） | 被參照的列可變或跨服務，複製才有必要 | 否：同一個 schema，被參照的列不可變（版本只 INSERT） | 只存參照，hash 由 Go 從不可變的列重算 | `rfq_publication_approvals`、`rfq_publications` 只存參照與 `schema_version`；接受單只留接受當下的 `seller_response_payload_hash`、`seller_facing_hash`（賣家實際收到內容的 hash，買家 CAS 的對象，留作稽核）與凍結的成交價三金額。`PreviewRfqPriceResponse` 的 `version`、`ConfirmRfqSnapshotRequest.price_preview_version`（A9-D21）隨 A9-D35 連同整個訊息刪除 |
| 賣家回價版本的 `payload_hash` 欄（舊 `procurement_seller_response_versions.payload_hash`）（S5，A9-D36） | 被參照的列可變或跨服務，複製才有必要 | 否：版本只 INSERT、同一個 schema | 不存，由 Go 從版本的報價內容重算 | `rfqhash.SellerResponse`：`schema_version` "v1"、固定欄位順序的 JSON 的 SHA-256；原像是報價內容全部欄位加幣別，不含 ID、版本號、送出時間與冪等鍵（Will 2026-11-23）；列表回的 `payload_hash` 與接受單的 `seller_response_payload_hash` 同值，3c 的賣家送出重用它 |
| 沒有寫入者的狀態值、接受單鎖死（S6） | CHECK 把合法值釘在資料庫 | 成立，但舊寫法收「完整值域」而不是會寫的子集 | CHECK 只放有寫入者的值，新值隨它的寫入者加 | `rfq_publications.state` 只收 `PUBLISHED`；`seller_responses.write_closed_reason` 只收 `DECLINED`；接受單 `state` 只收 `ACTIVE`、`ORDER_FAILED`；proto `PublicationState` 刪 `SUPERSEDED`、`CANCELLED`（`reserved`）。A9-D19 修訂：建單的 River job 重試用盡時（A9-3c）acceptance 轉 `ORDER_FAILED` 並解除 live 鎖（`uq` 只擋 `state = 'ACTIVE'`），買家可再接受；`TestLiveLockOnlyBlocksActiveAcceptances` |
| 採購的 ADK session 存放處（S7） | 圖形的暫停與回答分屬兩個 run，session 要跨越兩者存活 | 不成立：A9-D24(2) 採購管線用一般 Go 程式，中斷 payload 是接續的唯一狀態，執行期沒有 session 讀寫者【量，A9-3b-2b 查 procurement 的 session 表只有刪除與一致性測試】 | 不為沒有讀寫者的表留 schema | procurement 不使用 ADK session：刪 `procurement.sessions`、`events`、`app_states`、`user_states` 與其 retention 刪除、`TestADKServiceConformanceOnProcurementSchema`；adksession 只剩 assistant 一個使用者，已搬進 assistant（A9-D26、A9-D27）；`conversations.adk_session_id` 早已刪 |
| 人工同義詞與分類樹的存放（S8）、分類樹的 TTL 快取（B8，原清單漏列） | 分類樹經遠端讀取，每次解析都讀太慢 | 成立（分類服務在舊服務，經 `TaxonomyReader` port） | 單一 in-process TTL 快取，讀取有 timeout；同義詞是變動慢的小資料 | 同義詞放 `internal/resolver/synonyms.json`（`go:embed`，格式由 `LoadSynonyms` 驗證，空表起步，不沿用農產內容）；分類樹 `resolver.TreeTTL`（30 秒）、`LoadTimeout`（30 秒），stale-while-revalidate：有快取樹時過期後仍立即回舊樹並在背景重讀（同時只一次讀取）；沒有快取樹時所有並行呼叫共用同一次讀取，等待者可因自己的 ctx 結束離開；讀取不持鎖、不隨呼叫者的 ctx 取消，panic 轉成錯誤；失敗後 `RetryBackoff`（10 秒）內不再發起讀取（mutex＋done channel 自寫，不增加依賴；沒有舊樹才回錯誤）。這份手寫快取在 catalog 搬進單體（B8）時刪除，改用模組 API；catalog 搬進單體後以模組 API 取代 legacy 呼叫並重評 |
| 候選的 `display_name` 與可見性（S9） | 賣家改名或換圖後，候選要維持當時的樣子 | 否：契約已用共用 `StoreCard`（讀取時組裝）；沒公開過的店名不能被讀出 | 名稱讀取時組裝，只快照業務判定依據 | 刪 `candidates.display_name`、`seller_name`、`logo_key`、`store_code`（`replicas.csv:50` 已填 `query`）與 proto `Candidate.display_name`（`reserved`）；候選只收 OPEN 的店（`candidate.KeepOpen`，經 `seller.ExposableStores` 分批，fail-closed）；`TestCandidatesHaveNoStoreNameColumns`；`ExecuteRfqPublication` 對 DIRECT_RECIPIENTS 在 transaction 內再檢一次店家狀態屬 A9-3c |
| 買家關閉後採購資料的去向（S10） | 個資在不再需要時刪除，交易紀錄依法保留 | 成立（買家自由文字會轉呈賣家） | 刪個資、匿名保留交易紀錄 | `retention.OnBuyerDeleted` 訂閱 `BuyerDeleted`（同 transaction，`cmd/hawkr` 在 serve 與 worker 訂閱）：刪對話、輪次、run、候選集、報價清單、額度、沒發布過的需求；已發布的需求與預覽、快照、核准、發布、接受單的 `buyer_id` 設 NULL，邀請與賣家回價原樣保留；保留期跟詢價的生命週期，不另訂（Will 2026-11-21）；`TestBuyerDeletedRemovesBuyerDataAndAnonymizesPublishedRecords`、原子性 `TestBuyerDeletedIsAtomicWhenALaterStepFails` |
| selection 與 conversation（S11） | selection 與 candidate 1:1 | 是 1:1 | 折入 candidate | selection 併入 `candidates.selected`、`selected_at`（一表少三個冗餘 id）；conversation 與 request 維持兩張表（契約有兩個獨立 ID，刪對話不刪需求） |

## 候選搜尋的沿用盤點（A9-3b-2d，A9-D29）

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 只用關鍵字搜尋商品（`candidate_platform_lane.go`：把品項文字當關鍵字） | 沒有可靠的分類可用 | 否：TURN 已把品項確認到末層分類（A9-D28(1)），分類 ID 存在需求版本上 | 有結構化分類就以分類篩選，沒有才用文字搜尋 | 確認過的分類只帶分類 ID、不帶關鍵字；買家自述（`BUYER_DESCRIBED`）才用自述文字當關鍵字；已確認的分類不再是末層就失敗、清掉解析、需求退回 CLARIFYING（`ITEM_CATEGORY_UNAVAILABLE`） |
| 事實入庫（`candidates.matched_facts`、`unverifiable_facts`、`facts_as_of`） | 事實是搜尋當時的快照，之後不能被需求改動影響 | 否：事實只由需求現行版本決定（符合品項、需向賣家確認的數量、縣市、交期、預算），候選集的有效性已由「需求在 MATCHING 時最新一筆成功」表示 | 衍生資料讀取時組，不入庫 | 刪三欄；讀取時由 `reply.CandidateFacts` 依需求現行版本與語系組出，文字走模組 text 目錄；proto `facts_time` 指這次搜尋完成的時間 |
| 開始搜尋就把需求轉 MATCHING | 搜尋進行中需求要鎖在搜尋用的狀態 | 否：同需求同時只有一個進行中 run，搜尋可以失敗或被取消，失敗時需求不該停在 MATCHING | 狀態轉換跟著成功的提交走 | Start 不改需求狀態；搜尋成功的同一個 transaction 才轉 MATCHING（來源 REQUIREMENT_READY 或 MATCHING），零結果也是成功；提交時現行版本須仍是搜尋當時的版本，否則 run 失敗、狀態不動 |
| 相關性分數（`relevanceScore`）與依分數的一店一筆 | 同店多商品要挑最相關的 | 否：品項已由分類決定，沒有關鍵字可算相關性；分數無使用者 | 依可驗證的訊號排序；沒有的不編造 | 依銷量排序（`sort=SALES`，推論值），一店只留第一次出現的商品，`rank` 從 0 起依出現順序；刪 `candidates.score` 與 proto `Candidate.score`；翻頁到滿 20 家店或 5 頁（推論值，`candidate.MaxStores`、`MaxPages`），過濾（OPEN、不封鎖這位買家）後才計數，任一讀取失敗即搜尋失敗 |

## 領域層的沿用盤點（A9-3b-2a）

舊 `ai-service/internal/procurement/domain` 的 slots、planner、templates 搬成 `internal/slots`、`internal/planner`、`internal/reply`、`internal/text`，不搬 LLM narrator 與 realizer（A9-D24 第 4 項）。對照測試的來源在本節最後的「對照測試的來源」。A9-D25（Will 2026-11-21）已決定下表原本待決的項目。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 拒絕原因與模板文字寫死在 domain 的中文字串（`slots.go`、`dialogue_templates.go`、`SlotLabels`、`SlotPrompts`） | 單一語系的前端直接顯示伺服器字串 | 否：買家介面是 zh-TW／en（ADR 2026-10-17），使用者面文字走目錄 | domain 回代碼，呈現層依語系查文字 | 改：原因、act、問句、選項都是代碼，文字在 `internal/text`（`make uitext`，zh-TW 與 en）；zh-TW 文字與舊版逐字相同（`reply_test.go` 對照） |
| `Apply` 以 UTC 判斷到貨日的「今天」 | 伺服器與買家同時區 | 否：日曆日一律台北（`platform/twtime`），UTC 在台北凌晨會收下昨天 | 以使用者所在日曆日為界 | 改：以 `twtime.Day(now)` 為界，上限是今天加 365 天；差異有測試（`TestApplyDateBoundariesUseTaipeiCalendarDay`） |
| 文字截斷後不再去尾端空白 | 資料表沒有空白 CHECK | 否：`request_revisions` 的文字欄有「頭尾不得有空白」CHECK | 寫入端保證 CHECK 成立 | 改：截斷後再 `trimtext.Trim`（`TestApplyTruncationLeavesNoTrailingSpace`） |
| `ItemResolution` 帶 `CategoryID`，驗證「已確認分類必須有 ID、自行描述不可有 ID」 | 分類 ID 要存在需求版本裡 | 否：A9-3a 的 schema 只存 `kind` 與 `label` | 只存有消費者的欄位 | 刪 `CategoryID` 與兩條 ID 規則；標籤超過 200 字照一般文字截斷（資料表 CHECK） |
| `Slots.Hash()`（artifact 的 input_hash） | artifact 要比對輸入版本 | 否：A9-D24 第 5 項不寫 run 產出 | — | 刪 |
| `DialoguePlanV1` 的信封：schemaVersion、stateVersion、planHash、policyVersion、templateVersion、toneHints、maxSentences、artifactRefs、salientFactRefs、nextQuestionRef、allowed／forbidden claim codes、readinessGate | 給 LLM realizer 與 semantic validator 當輸入與稽核 | 否：不搬 realizer 與 validator（A9-D24 第 4 項），proto 沒有這些欄位 | 沒有消費者的欄位不建 | 刪；`planner.Plan` 只有 act、最多 2 個本輪事實、下一題 |
| 17 個 act 常數（其中 7 個「Phase 1 不可達」）與找不到模板時的通用句 | 之後各步驟解除封鎖時不必再命名 | 否：沒有寫入者的值不留（同 S6） | 只定義會被選中的值，封閉集合沒有找不到模板的退路 | 刪 7 個與通用句，留 10 個；每個 act 在 zh-TW 與 en 都有測試窮舉 |
| `PlanDialogue` 輸入的冗餘旗標（`Ready` 與 `Missing`、`HasRejected` 與 `Rejected`、`CropDisambiguationPending` 與 `Question`） | 呼叫端各自算好餵入 | 否：它們由同一份資料決定，分開傳是兩個真相來源 | 由單一來源推導 | 改：由 `Next.MissingRequired()`、`len(Rejected)`、`Question.Slot` 推導 |
| 問題卡的 `Source`、選項的 `Value` 在 wire 上 | 前端 types.ts 已宣告 | 否：新 proto 的 `QuestionOption` 只有 `option_id`、`label`，`ClarifyQuestion` 沒有 source | 契約是唯一來源 | 刪 `Source`；`Value` 留在伺服器內部，回答時以選項 ID 反查 |
| 欄位代碼的小寫字串（`item`、`delivery_city`）與 `crop_disambiguation` | 前端 JSON 的 key | 否：proto `SlotKey` 值名（`ITEM`、`ITEM_DISAMBIGUATION`） | 契約是唯一來源 | 改：用 `model.SlotKey`，值是 proto 值名；「crop」是農產內容的舊稱，改叫 item（選項 ID `item-other`、`category-<id>`） |
| 必填欄位＝品項、配送城市、單位；選填順序＝數量（含單位）、到貨日、包裝、單價預算；連續選填上限 3；數量與到貨日不可跳過；單位折進數量的複合問題卡 | 產品決定（舊註解：2026-08-09、2026-08-23 Will；報價單前提） | 是（產品規則，計畫沒有改它） | — | 沿用，原樣搬，有對照測試 |
| `AllowedUnits` 只收重量單位、`AllowedCities` 手寫 26 個字串（臺／台兩種寫法）、別名表、六都選項、到貨日 ≤365 天、數量 ≤1,000,000、單價預算 ≤10,000,000 | 需要可比對的固定值 | 部分：縣市在 `platform/twaddr` 有正規資料 | 縣市取自代碼表而不是第二份手寫清單 | 改（A9-D25 第 2 項）：刪手寫清單，縣市層級取自 `platform/twaddr.Counties()`，輸入的「台」統一成代碼表的「臺」，存與回傳只有一種拼法；簡稱（去掉市／縣）只在唯一時收，新竹、嘉義有市有縣不收；升格前的舊縣名（臺北縣等 5 個）無法由代碼表推導，留一張小表。單位、數字上限、六都選項照搬（`TestEveryTwaddrCountyIsAccepted`、`TestNormalizeCity`） |
| `RecordedThisTurn` 的「前後是否不同」規則與 `computeHasCorrection`（recorded 的欄位在 Current 已有值就是更正） | 讓更正與第一次填分開回應 | 是 | — | 沿用；`ExactMatchRequired` 改（A9-D25 第 6 項）：`Apply` 不再補預設值，`nil` 是系統預設、`Set.ExactMatch()` 取生效值，只有改掉買家設過的值才算更正（`TestExactMatchDefaultIsNotBuyerSet`、`TestFirstExplicitExactMatchIsNotACorrection`） |
| 模板文字 `REQUEST_BASICS_READY` 的「我現在就去幫你找找看合適的候選」 | 舊流程需求齊備後自動搜尋 | 否：A9-D24 第 3 項把候選搜尋拆成買家觸發的獨立 run | 只說已發生或買家下一步要做的事 | 改（A9-D25 第 4 項）：zh-TW「需求已齊，可以開始找候選」，en 同義句 |

A9-D25 對開放問題的決定（Will 2026-11-21）：

1. 連續選填計數與跳過的欄位：`turns.optional_streak`、`turns.skipped_slots`（具名欄位，CHECK 限選填欄位），每輪寫入（`InsertTurn`）；`planner.Input` 帶上一輪的值，`planner.Plan` 回這一輪要存的值。讀上一輪、寫這一輪的接線在 3b-2c 的 turn 用例。
2. 縣市：見上表。
3. 品項消歧：已由 A9-D28(1) 取代（一層為主，狀態只在中斷 payload，確認後存 `item_resolution_category_id`）；wave 1 的實作與 stage 欄位見 `internal/turn/AGENTS.md`。
4. `REQUEST_BASICS_READY`：見上表。
5. `RejectedField.reason`：proto 加列舉 `RejectionReason`（`REJECTION_REASON_*`，UNSPECIFIED=0），client 依值查共用文案目錄（`procurement.RejectionReason.*`）；敘述段落裡帶上限數字的原因句仍在模組 `internal/text`（`reject.*`）。
6. `ExactMatchRequired`：見上表。

## 對照測試的來源

下列測試的輸入與預期取自舊 `backend/services/ai-service/internal/procurement/`（行號見 commit `76de5c02` 的測試註解）：

| 新測試 | 舊來源 |
|---|---|
| `slots`：`TestApplyParity`、`TestNormalize*`、`TestRecordedThisTurn*`、`TestMissingRequired` | `domain/slots_test.go`、`domain/testdata/slots_apply/*.json`、`domain/normalize_test.go`、`domain/recorded_test.go` |
| `planner`：`TestPickNextParity`、`TestCanSkip`、`TestBuildForShapes`、`TestNextOptionalStreakParity`、`TestDecideParity` | `domain/pick_next_question_test.go`、`usecase/question_history_test.go`、`domain/dialogue_planner_test.go` |
| `reply`：`TestRenderParity`、`TestTextFallbackJoinsWithSpace`、`TestRejectionReasonsInZhTW`、`TestQuestionTextParity` | `domain/dialogue_templates_test.go`、`domain/testdata/dialogue_templates/*.json`；zh-TW 逐字相同，只有 `REQUEST_BASICS_READY` 的下一步句與單位清單（改用本地化單位名）依 A9-D25 與審查修正而不同 |

## 冪等鍵逐支判定（S4）

`api/hawkr/procurement/v1alpha1/procurement.proto` 帶 `client_request_id` 的 15 支 RPC。「建立型」的鍵放在被建立的實體上（`(buyer_id, client_request_id)` 唯一，重送時 `ON CONFLICT DO NOTHING` 回 0 列、改讀既有的實體並比對內容）；「目標狀態」的方法本身重送結果相同，不建收據。

| RPC | 判定 | 鍵放哪 |
|---|---|---|
| `SubmitTurn` | 建立型（run） | `agent_runs (buyer_id, client_request_id)` |
| `StartCandidateSearch` | 建立型（候選集與 run） | `agent_runs (buyer_id, client_request_id)`；候選集由 run 的 `candidate_set_id` 指到，重送回同一個 run 與候選集（A9-D29） |
| `SubmitAnswer`、`Rewind` | 建立型（輪次與新 revision） | `turns (buyer_id, client_request_id)`；重送回放 `turns.turn_result`（A9-D33(2)），比對含 `turns.command_slot`（A9-D34）；同一把鍵已被 run 用掉也算內容不同 |
| `PreviewRfqPrice` | 純計算（A9-D35），不寫入、沒有冪等鍵 | 無 |
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
| 請求大小 | 防止超大輸入 | 成立 | 欄位長度上限寫在契約與 DB CHECK | 買家訊息 2000 字元、`buyer_note` 500 字元；DB 以 `char_length` CHECK 守第二道（`turns.buyer_message`、`rfq_publication_approvals.buyer_note`、`agent_run_inputs.buyer_message`），入口驗證 3b、3c |
| 限流與配額 | 保護 LLM 成本 | 成立 | 計數器加條件 UPDATE | A9-D9：`usage` 一列計數器；每位買家同時進行的輪次等記憶體上限（A9-D18 是助理的）採購的另訂，3b 定 |
| 快取 | 分類樹讀取成本 | 成立 | TTL | 見 B8 列（`resolver.TreeTTL`） |
| trace | 跨程序追蹤 | 否：單體 | 單一 trace | `agent_run_events.trace_id` 不建（沒有讀取者，3b 若要再補） |
| 下游呼叫的逾時與重試 | 外部 provider 要有 deadline | 成立 | 明確 timeout、retry | LLM 走 `internal/platform/llm`（M6）；分類樹讀取 `LoadTimeout`；ordering 建單的 River job 退避與重試用盡（轉 `ORDER_FAILED`）在 3c |

## 詢價發布鏈的沿用盤點（A9-3b-3c，A9-D31 Q3–Q6）

| 機制 | 當時為什麼 | 現在成立嗎 | 從零設計 | 結論 |
|---|---|---|---|---|
| 發布建一個 `PUBLICATION` run 與 `RFQ_PUBLICATION_RESULT` artifact | 買家的 replay 要看到這次發布 | 否：artifact 已刪（A9-D24(5)），快照改讀 `GetLatestPublicationByRequest` | 發布是同步命令，不是 agent run | 不建 run；刪 `run_type` 的 PUBLICATION 與 `agent_runs.publication_id`（Q3） |
| 發布 outbox（INVITE_CREATED／EXTERNAL）與 `rfq_publication_commands` | 跨服務可靠外送；冪等收據 | 否：單體規則 4（M11、S4） | 同 transaction 寫發布、邀請與一則模組事件，投遞由訂閱者做 | `model.RfqPublished` 在 `ExecuteRfqPublication` 的 transaction 內 `events.Publish`；relay 與通知在 A9-3c（Q4）。冪等鍵放在 `rfq_publications` |
| 核准、確認快照、預覽各存 hash 與 canonical payload 複本，執行時比對存下的 hash | 被參照的列可變 | 否：參照的列不可變（S5） | hash 由不可變的列重算，比對「呼叫端帶來的」與「重算的」 | `internal/rfqhash`（SHA-256 對固定欄位順序的 JSON，原像含 `schemaVersion`）；預覽、執行與讀取端都用 `rfq.compile`／`rfq.SellerFacingHash` 同一個對照（Q5、A9-D35） |
| 受眾與賣家可見內容兩種 hash 各自的原像（預覽範圍與確認快照的 hash 隨 A9-D35 刪除） | 各自獨立演進 | 成立 | 同演算法、各自一個型別，欄位不共用 | `rfqhash` 的 `Audience`、`SellerFacing`；`TestEvery*FieldChangesTheHash`、`TestAssembledSellerHashCoversEveryApprovalAndRevisionField` 守欄位覆蓋 |
| 核准存 `candidate_set_id` 並在執行時重驗（A9-D35 推翻 A9-D29(4) 的「候選集不存進核准」） | 當初的理由是「受眾已凍結在 `_recipients`，有效性在預覽時檢查過」 | 否：這只防到預覽當下，擋不住預覽之後同一版本重新搜尋、產生新的有效候選集 | 同意物件存它同意當時依據的候選集，執行時問它現在還是不是有效的 | 指定對象的核准存 `rfq_publication_approvals.candidate_set_id`（`ON DELETE SET NULL`：買家關閉刪候選集時，保留的核准不連帶刪除）；`ExecuteRfqPublication` 以 `search.IsEffective` 重驗，失效回 `VERSION_CONFLICT`（`candidate_set`）。受眾 hash 仍只含模式、政策與有序店家 ID（候選集 ID 不進 hash：它不是賣家看到的內容） |
| EXTERNAL 站外收件人（prospect lane） | vendor portal | 否（A9-D1 刪） | 只有平台店家 | 邀請只有 `store_id` |
| 執行的 `canonical_request_hash` 判斷重送是否同一次請求 | 命令表存雜湊 | 否：沒有命令表 | 重送比對冪等鍵所在實體的欄位 | 比對需求、核准、核准版本、讀到的版本與 payload hash；核准版本存在 `rfq_publications.approval_version`（核准本身成功後版本已前進，不能從核准讀回）。發布預覽的重送另比對區間、補充說明、模式、期限、候選集與受眾 |
| 單一仲裁者：`ClaimApprovalForExecute` 的 CAS（state、version、buyer、`expires_at`） | exactly-one 與零寫入 | 成立 | 同 | 過期、已執行、版本不符都只由這個 UPDATE 判定，Go 不重複檢查；拒絕路徑整筆 rollback，核准維持 PENDING |
| 預覽 15 分、快照與核准各 24 小時、`MaxBuyerNoteLen` 200 | 沿用 | 預覽與快照隨 A9-D35 刪除；核准的 24 小時是業界做法（單一同意物件加單一到期時間），不是限制；補充說明上限依 proto 與 DB CHECK 是 500（proto 勝） | 同 | `rfq.ApprovalTTL`、`MaxBuyerNoteRunes`；`expires_at` 之前有效、等於 `expires_at` 已過期（procurement.md §9） |
| 確認快照與價格預覽（A9-D35 刪除） | 快照原本是「被參照的列可以改，所以要凍結一份複本」；預覽是「伺服器先算總額給買家看」 | 否：參照的列已不可變（S5）；總額只是買家的單價區間乘版本數量，沒有外部價格，沒有會變舊的東西；快照的 `expires_at` 舊碼與新碼都沒人讀 | 可改的草稿（需求現行版本）加一個不可變、會到期、只能執行一次的同意；試算是不寫入的計算 | 刪 `ConfirmRfqSnapshot` RPC、`confirmed_rfq_snapshots` 與 `price_previews` 表；核准存單價區間與 `buyer_note`，賣家可見內容由核准組出；`AcceptSellerQuote` 改比對賣家實際收到內容的 hash（ADR `2026-11-23-seller-facing-hash-supersedes-snapshot-hash`） |

### 舊分支到新契約的對照（A9-3b-3c，A9-D35 後）

| 舊分支 | 新契約 | 守門 |
|---|---|---|
| 價格預覽：clientRequestId 必填、冪等命中 | 刪：試算不寫入，沒有冪等鍵（A9-D35） | `TestPreviewPriceComputesTheTotalAndWritesNothing` |
| 價格預覽：revision ≠ expected（STALE_REVISION） | ABORTED／VERSION_CONFLICT，conflictTarget `request_revision` | `TestPreviewPriceBranches` |
| 價格預覽：range 不合法（FAILED_PRECONDITION BUYER_REQUESTED_RANGE_REQUIRED） | INVALID_ARGUMENT，欄位 `buyer_requested_range.lower_bound_amount`／`upper_bound_amount`，先於版本檢查（新契約以 proto 註解為準） | `TestPreviewPriceBranches`、`TestCheckInputReportsEachInvalidInputOnItsField` |
| 價格預覽：有單位數量卻算不出（QUANTITY_UNIT_UNCONVERTIBLE） | FAILED_PRECONDITION，無 reason（只剩溢位） | `TestPreviewPriceOverflowIsAFailedPreconditionWithoutAReason`、`TestTotalReportsOverflow` |
| 價格預覽：`referenceBasis`（GRAM 每 1000） | 同 | `TestPreviewPriceBasisAndTotalByRequirement`、`TestBasis` |
| 價格預覽：沒有單位（舊程式留空的計價基準） | FAILED_PRECONDITION／REQUIREMENT_NOT_READY（SSOT 沒寫，取最窄的擋法） | `TestPreviewPriceRequirementWithoutAUnitIsNotReady` |
| 價格預覽：`delivery_city` 欄位 | 刪（Q6）；賣家看到的交貨縣市取需求版本 | proto |
| 價格預覽：不屬於買家的需求 | NOT_FOUND（含已刪除的對話） | `TestPreviewPriceBranches` |
| 需求狀態不是 REQUIREMENT_READY 或 MATCHING（舊程式不查） | FAILED_PRECONDITION／REQUIREMENT_NOT_READY；試算、發布預覽與執行都查（A9-D35(5)） | `TestPreviewPriceRequiresAReadyOrMatchingRequest`、`TestPreviewPublicationRequiresAReadyOrMatchingRequestAndAUnit`、`TestExecuteRequiresAReadyOrMatchingRequest` |
| 確認快照：buyer_note 過長（200）、冪等、過期、預覽不符 | 刪（A9-D35）；補充說明移到發布預覽，INVALID_ARGUMENT 欄位 `buyer_note`，上限 500 字、先去頭尾空白，先於版本檢查 | `TestPreviewPublicationStoresTheRangeAndTheTrimmedNote`、`TestPreviewPublicationValidatesInputsBeforeTheRevisionCheck` |
| 發布預覽：revision ≠ expected | VERSION_CONFLICT `request_revision` | `TestPreviewPublicationStaleBranches` |
| 發布預覽：快照不屬於買家、snapshot.request／revision 不符（APPROVAL_STALE） | 刪：沒有快照（A9-D35） | — |
| 發布預覽：沒帶候選集或候選、候選重複、未知或零值模式 | INVALID_ARGUMENT，指向該欄位（含重複的索引） | `TestAudienceShape`、`TestPreviewPublicationShapeErrorsPointAtTheField` |
| 發布預覽：候選集不屬於買家或這個需求（舊：INVALID_ARGUMENT） | NOT_FOUND（與 ToggleSelection 一致） | `TestPreviewPublicationDirectBranches` |
| 發布預覽：候選集不是需求現行版本的（舊：`set.RequestRevisionID` ≠ active） | 改成有效候選集檢查（A9-D29(4)）：VERSION_CONFLICT `candidate_set`；核准存候選集，執行時重驗（A9-D35(4)） | `TestPreviewPublicationDirectOnlyAcceptsTheEffectiveCandidateSet`、`TestExecuteDirectRechecksTheEffectiveCandidateSet` |
| 發布預覽：候選不在候選集裡 | INVALID_ARGUMENT，欄位 `selected_candidate_ids[i]` | `TestPreviewPublicationDirectBranches` |
| 發布預覽：候選身分解析失敗（Internal） | 不存在：候選只有 `store_id`（NOT NULL、CHECK > 0） | schema |
| 發布預覽：回價期限格式 | handler 以 `google.type.Date` 驗證日曆日期，INVALID_ARGUMENT | `TestDateFromProto` |
| 發布預覽：回價期限早於今天（舊程式不查） | INVALID_ARGUMENT，欄位 `response_deadline`；今天以營業時區（台北）算，執行時再查一次（回 `publication_approval` 衝突，因為 Execute 請求沒有這個欄位） | `TestPreviewPublicationResponseDeadlineIsBoundedByTodayInTaipei`、`TestDeadlinePassedUsesTheTaipeiCalendarDay`、`TestExecuteRechecksTheResponseDeadlineInTaipei` |
| 發布預覽：冪等鍵撞到不同核准（IDEMPOTENCY_KEY_CONFLICT） | INVALID_ARGUMENT，欄位 `client_request_id`；內容相同回同一份核准；比對含區間、補充說明與候選集本身，期限在首次呼叫後過了也照回放 | `TestPreviewPublicationRejectsTheSameKeyWithDifferentContent`、`TestPreviewPublicationReplayIsNotBlockedByADeadlineThatPassedSinceTheFirstCall` |
| 執行：命令重送回原發布 | `Replayed` 為 true，不重複建邀請與事件（先於其餘檢查，過期核准也回放） | `TestExecuteReplaysTheFirstPublicationWithoutNewInvitesOrEvents` |
| 執行：同鍵不同內容（ErrPublicationCommandConflict） | INVALID_ARGUMENT，欄位 `client_request_id`；比對含 `approval_version` | `TestExecuteRejectsTheSameKeyWithDifferentContent`、`TestExecuteReplayComparesTheApprovalVersion` |
| 執行：revision ≠ expected | VERSION_CONFLICT `request_revision` | `TestExecuteStaleBranchesLeaveTheApprovalPending` |
| 執行：核准 CAS 失敗（版本、狀態、已執行）、核准不是這個需求的、payload hash 不符 | VERSION_CONFLICT `publication_approval` | `TestExecuteRejectsAnApprovalWhoseVersionMatchesButIsNotPending`、`TestExecuteRejectsAnApprovalVersionThatIsNotTheOnePreviewReturned`、`TestExecuteAnApprovalThatWasAlreadyExecutedIsAVersionConflict`、`TestExecuteStaleBranchesLeaveTheApprovalPending` |
| 執行：approval.revision ≠ active（舊：STALE_REVISION） | VERSION_CONFLICT `publication_approval`（核准凍的版本已過時） | `TestExecuteStaleBranchesLeaveTheApprovalPending` |
| 執行：核准過期（APPROVAL_STALE） | VERSION_CONFLICT `publication_approval`，核准維持 PENDING；`expires_at` 當下已過期 | `TestExecuteAnExpiredApprovalIsAVersionConflictAndLeavesItPending`、`TestExecuteApprovalExpiryBoundary` |
| 執行：DIRECT 受眾名單是空的／hash 不符 | 名單空：VERSION_CONFLICT `publication_approval`；hash 不再存，名單即來源，沒有「存下的 hash 不符」 | `Service.ExecutePublication` |
| 執行：核准不屬於買家 | NOT_FOUND（舊：CAS 失敗統稱 stale；新契約 not-owned → NOT_FOUND） | `TestExecuteStaleBranchesLeaveTheApprovalPending` |
| 執行：UnknownAudienceMode、PublicationNotFound（Internal） | 不存在：模式有 CHECK，發布與核准在同 transaction 建立 | schema |
| 執行：建 ledger run 與 artifact | 不建（Q3） | `TestExecuteCreatesThePublicationOneEventAndNoRun` |
| 執行：invite 與 outbox 同 transaction | invite 與 `RfqPublished` 事件同 transaction；任何一步失敗全部還原 | `TestExecuteIsAtomicWhenTheEventSubscriberFails`、`TestExecuteIsAtomicWhenAnInviteInsertFails`、`TestExecuteConcurrentRetriesCreateOnePublicationAndOneEvent` |


整合測試（`-tags integration`）只寫、沒有在本機跑，待 CI。

## 與 ai-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline。每一項對外看得到或 schema 上的差異一行。

- 表：31 張收斂為 21 張業務表（A9-3a 曾建 4 張 ADK session 表，A9-D26 刪除）；刪 7 張（`conversation_commands`、`rfq_publication_commands`、`rfq_publication_outbox`、`seller_response_commands`、`order_provisioning_outbox`、`agent_run_commands`、`usage_reservations`），`candidate_selections` 併入 `candidates`，`answer_patches` 改為 `turns`。
- ID：自己的實體 UUIDv7（`uuid`），`buyer_id`、`store_id`、`order_no` 維持舊值；原本的 TSID BIGINT 不保留。
- 表名：加 `procurement.` schema 前綴、去掉 `procurement_` 前綴。
- 沒有 EXTERNAL lane、`prospect_id`、站外業者驗證四欄（A9-D1）；`candidate_leads`（AD4）、`controller_kind`（AD12）不建；`run_type` 刪 `QUOTE_APPROVAL`。
- 候選：無 `display_name`、`seller_name`、`logo_key`、`store_code`、`brand_*`（S9）；`selected`、`selected_at` 在 `candidates`（S11）；無 `policy_version`、lease、`fencing_token`（S1）。
- 需求：無 `active_revision_id`、`version`、`branch_id`、`slots_hash`、`slots_json`（slots 改九個具名欄位，S2、S3）；`revision.status` 只有 `ACTIVE`、`SUPERSEDED`。
- 對話：無 `adk_session_id`；`turns` 取代 `answer_patches`，訊息原文是具名欄位。
- 輪次（A9-D33）：`turns.turn_result`（TurnResult 的 wire JSON，NOT NULL）。必要條件：SubmitAnswer、Rewind 不經 run（A9-D31），重送沒有 run 事件可回放，重算會因 planner 或文案改動而與第一次不同；所以任何來源的 commit 都寫它，重送回放它。業界：Stripe idempotency key、AIP-155 存第一次回應。重送的比對是同需求、同來源、同欄位（`turns.command_slot`）、同買家回答、同讀到的版本（結果版本的前一號）。`command_slot`（A9-D34）存 SubmitAnswer 回答或 Rewind 倒回的欄位，其他來源 NULL；必要條件：不經 run 的指令重送要比對欄位，略過的回答沒有 `buyer_message`、`turn_result.question` 是下一題，兩者都帶不住欄位。CHECK 只能要求 REWIND 必填、LLM_EXTRACT 必空，因為 `source = 'ANSWER'` 也含 ResumeRun 的回答（欄位在中斷上）。SubmitAnswer 的輪次必帶 `command_slot` 是程式的不變量（DB 無法強制，由 integration 測試守）；ResumeRun 的回答 NULL，SubmitAnswer 不回放它。
- 略過已有值的選填欄（A9-3b-3b）：SubmitAnswer 對已填的 PACKAGING、BUDGET_PER_UNIT 送 `skip` 仍把它記進 `skipped_slots` 並寫一個內容不變的新版本；舊 ai-service 同（`services/ai-service/internal/procurement/usecase/answer.go:98-103` 的略過只擋必填與報價必填欄，不看現有值），照搬、不加拒絕。
- 版本祖先（A9-D33(3)）：`parent_revision_id` 是內容祖先而不是被取代者；Rewind 的新版（來源 REWIND）內容同目標版、祖先是目標版（git 在舊 commit 上開分支、LangGraph time travel），不重新啟用舊版；取代順序由 `revision_no` 與現行表示。目標沿祖先鏈計算：最近一個改動該格的版本的祖先，所以再倒回同一格會繼續往前退。沒有可倒回的內容回 FAILED_PRECONDITION，不加 ErrorReason。
- SubmitAnswer 只收確定性回答（A9-D33(1)）：選項、數量、略過、對到唯一末層的品項文字；提問卡與中斷永遠由 run 擁有（業界：LangGraph interrupt、OpenAI `required_action` 屬於 run／thread）。UNIT、ITEM_DISAMBIGUATION、ITEM_RESOLUTION 不是回答的欄位。
- 詢價：核准、發布無 hash 與快照複本、無 `candidate_set_policy_version`、`price_preview_version`；核准存 `candidate_set_id`（A9-D35，指定對象時，`ON DELETE SET NULL`）；沒有確認快照與價格預覽兩張表（A9-D35）；hash 由 Go 重算，S3、S5；接受單無 `version`、`request_hash`、`lane`。
- 狀態值收斂（S6）：發布 `PUBLISHED`；`write_closed_reason` 只有 `DECLINED`；接受單 `state` 為 `ACTIVE`、`ORDER_FAILED`（新增，A9-D19 修訂：建單失敗解除 live 鎖）；邀請 `delivery_state` 無 `NOT_SENT`。proto `PublicationState` 刪 `SUPERSEDED`、`CANCELLED`。
- 回價：版本無驗證欄位與 hash 複本；賣家回價身分列無 `buyer_id`、`version`；品相欄位（品級、檢驗備註）已換成選填 `spec_note`（A9-D44）；`conversion_basis` 仍是契約欄位。
- run：無 `controller_kind`、`version`、`domain_ref_type`、`domain_ref_id`（改 typed 外鍵）；`agent_run_inputs` 無 `retention_class`、`input_schema_version`，買家原文是具名欄位；`agent_run_events` 主鍵改 `(run_id, seq)`、無 `schema_version`、`trace_id`；`agent_run_interrupts` 無 `expires_at`、`response_schema_version`（3b 確認是否有寫入者）、`run_artifacts` 無 `domain_ref_*`、`updated_at`。
- 額度：`usage.usage_count` 改名 `used`，條件 UPDATE，無預約表（A9-D9）。只有 TURN 扣額度，`agent_runs.quota_period_start` 可為 NULL，退款只在非 NULL 時做（A9-D24(3)）。
- 每買家上限（A9-D24(6)）：進行中數與每分鐘送出數都對 `agent_runs` 計數，在買家的 advisory lock 內；舊 ai-service 的記憶體計數不搬（多 pod 與重啟會各算各的）。沒有「同需求一個進行中」的唯一索引：既有的 integration fixture 會對同一個需求種多個進行中的 run，而買家鎖已序列化同買家的送出（需求只屬於一位買家）。
- 回答中斷改回答成功才解決（A9-D26，取代 A9-D24(7) 的重新開放）：回答 run 失敗或取消不做還原，中斷與提問的 run 維持原狀；`RESUMED` 狀態、`resumed_into_run_id`、`resolved_by_child_run_id`、`reopenParent` 已刪。每個需求最多一筆 PENDING 中斷（部分唯一索引），提問的 run 被取消或取代時中斷轉 CANCELLED。
- RESOLVED 的中斷不記解決它的 run：重送回放看回答 run 的 `interrupt_id`，沒有讀取者需要反查，所以不留 `resolved_by_run_id`。
- 買家關閉：新增 `BuyerDeleted` 訂閱者，刪買家資料、匿名保留賣家可見的紀錄（S10）；ai-service 沒有任何刪帳處理。
- 候選同義詞：不沿用 851 筆農產別名與 `synonyms` map，空表起步（S8、AD3）。
- 變動：`procurement.proto` 的 `Candidate.display_name` 與沒有寫入者的 `PublicationState` 值刪除（上線前不留 `reserved`）。
- run 帳本的 integration test（A9-3b）：`internal/run/run_integration_test.go` 與 `procurement_integration_test.go` 的 R4 兩支，要 Postgres，待推後 CI。
- run 帳本外鍵（A9-D20 R4）：維持 CASCADE（A9-D31 Q3 刪了 `publication_id`）；BuyerDeleted 的 `run` 步驟排在第一（`TestRunsAreDeletedBeforeEveryOtherStep`）。
- 已決（A9-3b-2f）：`RunFinished.localized_message` 已刪除改為 `reserved`（A9-D30：失敗文字由 client 依 `failure_reason` 以共用文案目錄呈現，伺服器不填）。
- 待決（A9-3b 留下）：`Executor` 與 handler 接上後登記 `run.Dispatcher`、`run.Submitter`、`run.Canceller`（A9-3b-2）；`JobQueue` 的 River 實作（`CancelJob` 回傳取消後的 job 狀態）在 A9-3b-2 一併做；job 被 River 取消而 run 不在 CANCEL_REQUESTED 時一律標 `WORKER_LOST`（A9-D22 沒寫）。
- 待刪（手寫 TTL 快取）：`resolver` 的手寫快取在 catalog 搬進單體（B8）時刪除，改用模組 API，見 B8 列。
- 守門：`retention` 的清單由 `TestRetentionCoversEveryBuyerIdTable` 對 schema 檢查（含 `buyer_id` 的表必須登記或明列豁免與理由）。
