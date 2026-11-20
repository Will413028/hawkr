# assistant 模組行為規格

本文是單體 `assistant` 模組行為的唯一來源：買家助理的對話：買家與助理的跨輪對話、串流回應與續傳。目前實作到 agent、tool 與輪次執行（A9-2b）；Connect handler 與掛載在 A9-2c。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：ordering、catalog、buyer。
- 公開介面（A9-2b）：`SubmitTurn`、`WatchTurn`、`Shutdown`；handler（A9-2c）呼叫這三個方法，proto 到錯誤碼的對應見第 3 節。
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

**送出流程**（回傳前同步完成）：拒絕在 transaction 內呼叫（`ErrOuterTx`）→ 主題與訊息檢查（`ErrInvalidTopic`、`ErrMessageRequired`）→ 解析對話：`SessionID` 空則建立（state 只有 `buyer_id`，十進位字串；失敗回 `ErrSessionCreate`），非空則必須已存在且屬於該買家（否則 `ErrSessionNotFound`，不自動建立）→ 佔用對話（`ErrTurnInProgress`）→ 登記輪次並起背景 goroutine。

**單一進行中輪次（A9-D14）**：記憶體裡以 `buyerID/sessionID` 為鍵的佔用標記。釋放發生在終止事件寫入之前：client 一收到 finished 或 error 就能重送。正常結束、LLM 錯誤、panic、輪次逾時（`DefaultTimeout` 2 分鐘）、`Shutdown` 取消都會釋放。退貨政策不建 session，不佔用。

**事件**：`text`（文字增量）、`tool_result`（tool 名稱、是否失敗、tool 解析後的參數）、`finished`（對話 ID）、`error`（`FailureNoOutput`＝還沒有任何文字、`FailureInterrupted`＝已送出部分文字，加上依語系的訊息）。`finished` 與 `error` 是終止事件，之後沒有事件。tool 失敗不終止這一輪：錯誤文字回給 LLM，助理仍回覆。文字規則：partial 的 chunk 逐段送出；非 partial 到達時只補送沒送過的尾巴；tool 回應之後重新計算；thought part 不外流。

**環形緩衝**：每輪 `DefaultBufferCapacity`（1024）個事件，序號從 1 起，cursor 是序號的十進位字串。輪次結束後保留 `DefaultRetention`（10 分鐘）供續看，超過時下一次送出或訂閱時清掉。`WatchTurn`：輪次不存在、已清掉或不屬於該買家回 `ErrTurnNotFound`；cursor 早於緩衝起點回 `ErrCursorExpired`；cursor 超前最新事件或不是數字回 `ErrCursorAhead`。迭代器在終止事件後結束；ctx 結束（handler 的 100 秒）時正常結束；消費太慢被覆蓋時以 `ErrCursorExpired` 結束。

**對外對應（A9-2c 實作）**：`ErrTurnInProgress` → `FAILED_PRECONDITION` + `ASSISTANT_TURN_IN_PROGRESS`；`ErrCursorExpired`／`ErrCursorAhead` → `FAILED_PRECONDITION` + `REPLAY_CURSOR_EXPIRED`／`REPLAY_CURSOR_AHEAD`；`ErrTurnNotFound`、`ErrSessionNotFound` → `NOT_FOUND`；`ErrMessageRequired`、`ErrInvalidTopic` → `INVALID_ARGUMENT`；`ErrSessionCreate` → `INTERNAL`。`error` 事件的 `ErrorReason` 未決（見 CARRYOVER 的待決）。

**身分範圍**：每支 tool 自己驗：session state 的 `buyer_id` 必須等於這一輪的買家，否則 tool 回錯（`ErrBuyerScope`），不呼叫訂單服務。agent 與 tool 為每一輪重新建構，tool 完成時經回呼寫 `tool_result` 事件。

**不在 transaction 內**：輪次執行不開 transaction，也不接受在 transaction 內的 ctx；session 寫入由 `adksession` 在最上層開。

**驗收**：`internal/turn` 的 characterization 測試鎖舊 ai-service 的行為（比對 fixture 的事件種類與順序、文字串接、tool 名稱與參數、結束或錯誤、LLM 請求的路由、使用者文字、回給 LLM 的 tool 結果）；跳過 `latest_session_*`（A9-2c）。
