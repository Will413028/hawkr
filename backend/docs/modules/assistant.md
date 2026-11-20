# assistant 模組行為規格

本文是單體 `assistant` 模組行為的唯一來源：買家助理的對話：買家與助理的跨輪對話、串流回應與續傳。目前實作到 agent、tool、輪次執行（A9-2b）與 Connect handler、冪等、掛載（A9-2c）。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：ordering、catalog、buyer。
- 公開介面：`SubmitTurn`、`WatchTurn`、`LatestSession`、`Shutdown`；handler（`internal/httpapi/assistant`）呼叫它們，proto 到錯誤碼的對應見第 3 節。
- 舊行為的 characterization fixture 在 `docs/plans/2026-11-19-a9-agent/fixtures/assistant/`（A9-2a 以替身 LLM 錄自舊 ai-service 的買家路徑；每份含輸入、替身回應、假 tool 後端資料、實際送出的事件序列）。

## 2. Session 儲存（A9-D5）

`internal/adksession` 以 pgx 與 sqlc 實作 ADK v2 的 `session.Service`；不改 ADK 原始碼，GORM 不進單體。資料在 schema `assistant` 的四張表（`db/schema.sql`）。

| 語意 | 行為 |
|---|---|
| `Create` | `AppName`、`UserID` 必填；`SessionID` 空則產生 UUID。同一個 (app, user, session) 已存在回錯（`already exists`），不覆蓋、不寫任何 state。初始 state 依前綴分流：`app:` 進 `app_states`、`user:` 進 `user_states`、`temp:` 丟棄，其餘進 session。 |
| `Get` | 三個鍵必填；不存在或屬於別的使用者回包裝 `session.ErrNotFound` 的錯誤。state 是 session、app、user 三層合併（app、user 的 key 加回前綴）。events 依 `timestamp`、`seq`（寫入順序的 identity 欄）由舊到新；session 列與 events 在同一份 `db.InReadSnapshot` 讀取（所以不能在 transaction 內呼叫 `Get`）；`After` 先篩（`timestamp >= After`），`NumRecentEvents` 再取最近 N 筆。 |
| `List` | `AppName` 必填；`UserID` 為空字串列出該 app 全部使用者的 session，不含 events；每個 session 帶自己使用者的 user 層 state。 |
| `Delete` | 不存在或屬於別的使用者不是錯誤；events 由 FK 連帶刪除。 |
| `AppendEvent` | partial event 不寫；沒有 ID 補 UUID、時間截到微秒（都就地改呼叫端的 event）；`StateDelta` 的 `temp:` key 寫前剝除（呼叫端的 event 不被改，live session 的 state 仍保留 `temp:` key 到這一輪結束）；一個 transaction 內鎖 session 列、寫 event、合併三層 state delta。session 不存在回包裝 `session.ErrNotFound` 的錯誤。成功後就地更新傳入的 session（events、state、`LastUpdateTime`）。重複 event id 回包裝 `adksession.ErrEventExists` 的錯誤（`ON CONFLICT DO NOTHING` 看影響列數，為了不依賴解析驅動的 unique violation 錯誤）；呼叫端以 `errors.Is` 判斷。 |

- `Create`、`AppendEvent` 在提交後才更新 live session，所以拒絕在外層 transaction 內執行（`adksession.ErrInOuterTx`）；必須是最上層呼叫。
- `NumRecentEvents` 超過 int32 時夾到 `math.MaxInt32`。

**鎖順序**（`Create`、`AppendEvent` 同序，未來新增寫入路徑照此）：`assistant.sessions` 列、`assistant.app_states` 列、`assistant.user_states` 列。`events` 只插入不鎖舊列。跨模組的全域順序見 `backend/AGENTS.md` 規則 4。

**已知限制**：`app_states` 每個 `app_name` 只有一列，任何帶 `app:` 前綴 state delta 的 `AppendEvent` 都鎖它，會讓所有買家的這類寫入串行。盤點結果：`rg -n '"app:' backend/services/ai-service` 沒有命中，舊買家助理不寫 `app:` key，目前實際上不會競爭；若日後引入 `app:` state，需先量測再決定是否拆分。

- state 在 SQL 內以 jsonb `||` 合併（淺層、後寫的 key 覆蓋），並行 append 不丟 key。沒有 stale session 檢查（理由見 `CARRYOVER.md`）。
- state 經 JSONB 往返，數字讀回是 float64；大整數 ID 一律以字串寫入。
- 驗收：ADK 的 `session/sessiontestsuite.RunServiceTests`（`session_integration_test.go`，需要 Postgres，只在 CI 跑，本機不起容器）加上本模組補的並行、回滾、跨使用者與 cascade 測試。

## 3. 輪次執行（A9-D12、A9-D14）

`SubmitTurn(ctx, SubmitTurnInput{BuyerID, Topic, SessionID, Message, ImageURL})` 驗證後回 `TurnRef{TurnID, SessionID}`，agent 在背景執行；`WatchTurn(ctx, buyerID, turnID, cursor)` 回事件迭代器。語系取自 ctx（`uitext.LocaleFrom`）。

**主題與 agent**：`TopicOrders` 直接用訂單 specialist（空訊息時用預設訊息「查詢我的訂單」），`TopicTracking` 用物流 specialist（預設訊息「查詢包裹狀態」），`TopicChat` 用 coordinator（兩個 specialist 以 agenttool 掛上，不掛 function tool；空訊息回 `ErrMessageRequired`），`TopicReturnPolicy` 回固定文案、不呼叫 LLM、不建 session。tool 三支：`get_my_orders`、`get_buyer_order_detail`、`track_my_package`，都只經 `OrderReader` port 讀訂單。

**送出流程**（回傳前同步完成）：拒絕在 transaction 內呼叫（`ErrOuterTx`）→ 主題與訊息檢查（`ErrInvalidTopic`、`ErrMessageRequired`）→ 在同一把鎖內依序檢查已關閉（`ErrShuttingDown`）、佔用對話（`ErrTurnInProgress`）、`wg.Add`；新對話的 ID 由伺服器先產生（UUID，用 `session.CreateRequest.SessionID` 傳入）→ 寫或讀 session：`SessionID` 空則以產生的 ID 建立（state 只有 `buyer_id`，十進位字串；失敗回 `ErrSessionCreate`），非空則必須已存在且屬於該買家（否則 `ErrSessionNotFound`，不自動建立；別的買家的 session 同樣是 NotFound）→ 登記輪次並起背景 goroutine。佔用與 `wg` 在 session 寫入之前取得，所以 `Shutdown` 之後不會有沒有輪次的 session；任何一步失敗都釋放佔用並 `wg.Done`（`TestFailedSessionWriteReleasesTheGuardAndTheWaitGroup`）。

**單一進行中輪次（A9-D14）**：記憶體裡以 `buyerID/sessionID` 為鍵的佔用標記。釋放發生在終止事件寫入之前：client 一收到 finished 或 error 就能重送。正常結束、LLM 錯誤、panic、輪次逾時（`DefaultTimeout` 2 分鐘）、`Shutdown` 取消都會釋放。退貨政策不建 session，不佔用。

**事件**：`text`（文字增量）、`tool_result`（`Tool` 列舉＝三支 tool 之一、`ToolSucceeded`；對外是 `AssistantToolResult{tool, succeeded}`，顯示文字在共用文案目錄 `assistant.AssistantTool.*`；不帶參數與回傳內容，A9-D13 修訂）、`finished`（對話 ID）、`error`（`FailureNoOutput`＝還沒有任何文字、`FailureInterrupted`＝已送出部分文字，加上依語系的訊息）。`finished` 與 `error` 是終止事件，之後沒有事件。tool 失敗不終止這一輪：模型只收到錯誤代碼，助理仍回覆。文字規則：partial 的 chunk 逐段送出；非 partial 到達時只補送沒送過的尾巴；tool 回應之後重新計算；thought part 不外流。

**finished 的條件（A9-D15 第 4 點）**：事件迭代正常結束，而且看過根 agent 的最終回應（`IsFinalResponse`、作者是根 agent、沒有 `ErrorCode`）；否則一律是 `error`。取消、逾時、`Shutdown`、模型安靜停下、緩衝寫滿都落在這一條，迴圈之後不再看 `ctx.Err()`（`TestStreamEndingNormallyWithoutAnyFinalResponseIsNotFinished`、`TestStreamThatEndsWithoutTheFinalResponseIsNotFinished`、`TestCancelledTurnWhoseModelStopsQuietlyEndsAsAnError`）。`Config.ToolObserver` 是觀察點：每次 tool 呼叫收到 tool、名稱、參數的深拷貝與是否失敗，只給測試與日後的指標，不進事件；characterization 靠它比對 tool 名稱與參數。

**tool 包裝層（A9-D15 第 3 點）**：所有 function tool 都是 `buyeragent.guardedTool`，包在 `tool.Tool.Run` 外層，`specialist()` 只接受這個型別。它 recover panic（stack 只進 slog）、把任何錯誤換成固定代碼回給模型（`tool_failed`、`upstream_unavailable`、`order_not_found`、`forbidden`；下游的錯誤文字只進 log），並且每次呼叫恰好回報一次——參數驗證失敗與 panic 都算。每個有 tool 的 agent（含 coordinator 的 agenttool）以 Gemini 函式呼叫模式 `VALIDATED`＋`AllowedFunctionNames`（名稱與該 agent 掛的 tool 同一份清單，`GenerateContentConfig.ToolConfig`）在解碼端禁止未知名稱（A9-D15 第 3 點修訂）；事後仍保留防線：模型叫了不存在的 tool 時，ADK 的預設錯誤文字會列出所有 tool 名稱，所以每個 agent 掛 `OnToolErrorCallback`：模型只看到 `unknown_tool`（coordinator 委派 specialist 失敗是 `delegate_failed`）；沒有對應的 `AssistantTool`，不能表示成 `tool_result`（UNSPECIFIED 不得當合法值），只記 log。

**語言與代碼（A9-D15 第 2 點）**：agent 以買家這則訊息的語言回覆，判斷不了才用請求語系（系統提示的 `replyRule`）；退貨政策固定文案依請求語系（`text.ReturnPolicy*`）。tool 回給模型的 `status` 是訂單狀態代碼，不是標籤；`track_my_package` 沒有包裹時回 `note=no_packages_in_transit`。狀態碼表在系統提示（`statusGuide`），來源是 CodeMeta〈業務狀態碼〉的 OrderStatus（0、10、20、30、32、35、40、50、54、60、70、80），`TestStatusGuideListsExactlyTheOrderStatusCodesOfTheAuthoritativeSource` 比對。追蹤 tool 查「20,30,32,35,40,54」（待出貨、配送中、待取貨、物流異常、已送達、配送失敗未退款；order-service 的 status filter 以逗號分隔；集合由 ordering 擁有，常數 `trackingStatuses`，A9-D16 第 1 點），每個包裹帶自己的 `status` 代碼由模型回答；舊服務只查 20 是缺陷，已更正。單筆詳情查詢失敗不再靜默略過：全部失敗一律回失敗的 tool 結果 `upstream_unavailable`（不看哪一筆最後失敗；原因合併後由 guardedTool 記一次 log）；部分失敗成功並回 `note=some_packages_unavailable`（清單不完整，模型要告知買家），逐筆記 log（含訂單號與 tool 名稱）。詳情查詢回 `ErrOrderNotFound`（訂單已不存在）的包裹靜默略過，不觸發 `some_packages_unavailable`。追蹤 tool 只查第一頁（10 筆）：舊 order-service 的列表只有 status filter 與分頁（`legacy/orderservice/buyerorders.go` `MyOrders`），沒有可排除已送達的條件，翻頁會讓詳情查詢乘上頁數，所以滿頁時加 `note=more_packages_not_shown`（追蹤中的包裹不只這些，模型要告知買家）；`note` 同時有多個代碼時以逗號分隔。

**事件緩衝（A9-D15 第 5 點）**：每輪一個只增不減的緩衝，上限 `DefaultBufferCapacity`（4096，含終止事件）個事件，序號從 1 起，cursor 是序號的十進位字串；不覆蓋舊事件，所以沒有「cursor 過期」。最後一格保留給終止事件：一般事件寫滿時，這一輪被中止（取消 runner、記 log），以 `error` 終止（已有文字是 `FailureInterrupted`，否則 `FailureNoOutput`），佔用照常在終止事件之前釋放（`TestATurnThatOverflowsTheBufferEndsWithAnErrorAndReleasesTheSession`、`TestBufferNeverOverwritesAndKeepsTheLastSlotForTheTerminalEvent`）。切片隨事件增長，不預先配置。輪次結束後保留 `DefaultRetention`（10 分鐘）供續看，超過時下一次送出或訂閱時清掉。`WatchTurn`：輪次不存在、已清掉或不屬於該買家回 `ErrTurnNotFound`；cursor 超前最新事件或不是非負整數回 `ErrCursorAhead`。迭代器在終止事件後結束；ctx 結束（handler 的 100 秒）時正常結束。

**對外對應（A9-2c）**：`ErrTurnInProgress` → `FAILED_PRECONDITION` + `ASSISTANT_TURN_IN_PROGRESS`；`ErrCursorAhead` → `FAILED_PRECONDITION` + `REPLAY_CURSOR_AHEAD`；`ErrShuttingDown` → `UNAVAILABLE`；`ErrTurnNotFound`（含已超過保留期被清除，A9-D17 修訂）、`ErrSessionNotFound` → `NOT_FOUND`；`ErrMessageRequired`、`ErrInvalidTopic`、`ErrIdempotencyMismatch` → `INVALID_ARGUMENT`；`ErrSessionCreate`、`ErrOuterTx` → `INTERNAL`。`error` 事件的 reason 一律 `ASSISTANT_TURN_FAILED`（`FailureKind` 只進 log）。

**冪等（`client_request_id`，A9-D17 修訂）**：鍵是（買家，`client_request_id`），存請求內容（主題、對話、訊息、圖片網址）的雜湊與 `(turn_id, session_id)`，與該輪的保留期（10 分鐘）一起清除；同鍵同內容回放第一次的結果，不看對話是否進行中、不開新的一輪；同鍵不同內容 `INVALID_ARGUMENT`；第一次受理失敗不留紀錄；同時送出同一個鍵的請求等第一個的結果。存在記憶體，重啟後遺失。

**串流窗口**：`WatchAssistantTurn` 每條最長 100 秒就正常結束（無錯誤、無終止事件），client 以最後的 cursor 接續；收到 `finished` 或 `error` 事件才是這一輪結束。

**最近對話**：`GetLatestAssistantSession` 列呼叫者（`user_id` = 買家 ID）的 session，取最後更新時間最新的一筆，回整段歷史中有文字的事件（tool 呼叫與回應、thought 不外流）。

**身分範圍**：每支 tool 自己驗：session state 的 `buyer_id` 必須等於這一輪的買家，否則 tool 回 `forbidden`（內部 `ErrBuyerScope`），不呼叫訂單服務。agent 與 tool 為每一輪重新建構，包裝層在 tool 完成時經回呼寫 `tool_result` 事件。

**不在 transaction 內**：輪次執行不開 transaction，也不接受在 transaction 內的 ctx；session 寫入由 `adksession` 在最上層開。

**驗收**：`internal/turn` 的 characterization 測試鎖舊 ai-service 的行為（比對 fixture 的事件種類與順序、文字串接、經觀察點取得的 tool 名稱與參數、結束或錯誤、LLM 請求的路由、使用者文字、回給 LLM 的 tool 結果）；回給 LLM 的 tool 結果有 6 份 fixture 是 A9-D15 的刻意偏離（狀態標籤改代碼、下游錯誤文字改代碼、追蹤查詢狀態），每份在測試的 `fedBackDeviations` 與 `statusRemap` 寫理由，其餘必須逐字相同；`latest_session_*` 兩份由 `internal/history` 的測試比對。

**訂單 port 的錯誤轉換（A9-2c 已完成）**：`buyeragent.Orders` 的實作在 composition root（`cmd/hawkr/assistant.go`）以訂單服務 client 組成，把 `orderservice.ErrOrderNotFound` 轉成 `assistant.ErrOrderNotFound`（以 `%w` 包住原錯誤），`cmd/hawkr/assistant_test.go` 守住——否則「查無訂單」會被當成 `upstream_unavailable` 給模型。
