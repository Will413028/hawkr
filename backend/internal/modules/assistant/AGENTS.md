# assistant 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與已知限制寫在 `backend/docs/modules/assistant.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

買家助理的對話：買家與助理的跨輪對話、串流回應與續傳。服務 buyer-app 的買家。來源是 ai-service，搬遷計畫 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）；依賴：ordering、catalog、buyer。已有 session 儲存、買家 agent 與 3 支 tool、輪次執行器（A9-2b）；Connect handler 在 A9-2c。行為規格的唯一來源是 `backend/docs/modules/assistant.md`。

## 擁有的資料

schema `assistant`（`db/schema.sql`），登記在 `cmd/hawkr/modules.go` 的 `modules()`。四張表是 ADK v2 `session.Service` 的儲存（A9-D5）：`sessions`（session 層 state）、`events`（整個 `session.Event` 的 JSON，partial 不寫、`temp:` key 寫前剝除）、`app_states`、`user_states`；鍵是 ADK 的字串，`user_id` 是買家 ID 的十進位字串（外部鍵、不建 FK）。助理輪次事件不建表，放記憶體的每輪緩衝（只增不減、有事件數上限，A9-D12、A9-D15）。讀取模型：無。

## 發布的讀取介面

無。<!-- 每個 `pub_*` 一行：形狀與用途（契約在 `db/pub.sql`）。 -->

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名、事件與錯誤。
- `internal/model` 放對外型別；`internal/feature` 是第一個子領域，實作時改成業務名稱；`internal/store` 是 sqlc 的查詢套件；`internal/adksession` 以 pgx 實作 ADK 的 `session.Service`。
- `internal/buyeragent`：ADK agent 與 3 支 tool，為每一輪建構；`OrderReader` port 由 `cmd/hawkr` 以訂單服務 client 實作。`internal/turn`：輪次執行器、每輪事件緩衝、同對話單一輪次。`internal/text`：模組句子（`make uitext`，zh-TW、en）。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加 use case 或子領域時同步修改圖。各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。

## 不變量

模組只 import 依賴表允許的模組根套件：`internal/arch` 的依賴方向測試守門。

- session 的寫入都在 `db.InTx` 內，transaction 內不呼叫 LLM（規則 5）。state delta 以 SQL 的 jsonb `||` 合併，不讀出再寫回；鎖順序 session 列、app 列、user 列，詳見 `backend/docs/modules/assistant.md` §2。
- session 的寫入（`Create`、`AppendEvent`）必須在最上層呼叫；在外層 transaction 內回 `ErrInOuterTx`。
- `adksession` 的驗收是 ADK 的 `RunServiceTests`（`session_integration_test.go`，CI 的 Postgres）；`AppendEvent` 寫入內容由純函式 `encodeAppend` 算出，`temp:` 剝除有本機單元測試守。
- 輪次執行器不開 transaction、不接受 transaction 內的 ctx（`ErrOuterTx`）；LLM 只經 `Deps.LLM`（`internal/platform/llm` 建構）。
- 同一個對話同時只有一輪（A9-D14）：佔用在終止事件寫入之前釋放，所以釋放路徑（結束、錯誤、panic、逾時、取消）都要先 release 再發終止事件。
- tool 的身分範圍：session state 的 `buyer_id` 必須等於這一輪的買家；新增 tool 一律用 `newTool`（它先驗範圍），在 callback 與 tool 裡用 `ctx.State()`，不用 `ctx.Session().State()`。
- tool 只能經 `guardedTool` 掛上 agent（`specialist()` 只收這個型別）：它 recover panic、把錯誤換成固定代碼、每次呼叫恰好回報一次。回給模型的內容不帶語言（狀態用 CodeMeta OrderStatus 的代碼，錯誤用代碼），下游錯誤文字與 stack 只進 log；新增代碼同步改系統提示的 `toolErrorGuide`。
- `finished` 只在事件迭代正常結束且看過根 agent 的最終回應時發；其他一律 `error`，不看迴圈後的 `ctx.Err()`。`Submit` 的順序是 closed 檢查、佔用、`wg.Add`（同一把鎖）、產生 session ID、寫 session，任何失敗路徑都要釋放。
- 每輪事件緩衝只增不減，上限含終止事件，最後一格留給終止事件；寫滿就中止這一輪，沒有 cursor 過期。`tool_result` 只帶 tool 列舉與成功與否，參數只經 `Config.ToolObserver` 給測試。
- 對外 enum 與錯誤原因不得用 UNSPECIFIED 當合法值；`error` 事件的 `ErrorReason` 由 A9-2c 的 handler 決定。

## 不負責

採購（`procurement`）、修圖（`imageai`）；賣家助理與刊登助理已依 A9-D2 刪除。

## 決策紀錄

從 ai-service 沿用的每個機制、當時的決定與理由，以及與 ai-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。<!-- 相關 ADR 列在這一行後面。 -->
