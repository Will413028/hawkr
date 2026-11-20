# assistant 模組行為規格

本文是單體 `assistant` 模組行為的唯一來源：買家助理的對話：買家與助理的跨輪對話、串流回應與續傳。目前實作到 session 儲存（A9-2a）；agent、tool、輪次執行與 handler 在 A9-2b、A9-2c。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：ordering、catalog、buyer。
- 公開介面與行為隨 A9-2b 以後的實作補上。
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
