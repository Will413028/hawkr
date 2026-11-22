# turn 子領域

TURN 類型 run 的 executor（A9-D24(2)、A9-D28）：一次模型呼叫抽欄位，其餘是純函式，結果在單一 transaction 提交。行為規格見 `backend/docs/modules/procurement.md` 的 TURN 節。

## 不變量

- 流程固定：讀（run 輸入、需求、現行版本、對話、上一輪、價格預覽與確認快照）→ `extract`（transaction 外）→ `decide`（`slots.Apply`、品項變更或未解析時 `resolver.Resolve`、`planner`、`reply`，唯一的 I/O 是 resolver）→ `commit`。LLM 與分類樹讀取不得進 transaction（單體規則 5；`TestSuccessWrites…` 斷言）。
- `commit` 的 transaction 先 `LockRun` 自己的 run，再寫：取代現行版本、新版本、`InsertTurn`（0 列＝冪等衝突，失敗不繼續）、需求狀態、對話預覽、自動標題，最後 `Ledger.Append(turn_result)`＋`Transition(SUCCEEDED)` 或 `Ledger.Interrupt`。任何一步失敗整筆回滾，錯誤回給 Dispatcher 收斂；中斷寫入失敗不得退回成功。
- 失敗分類：`extract.Error.Unavailable`（逾時、429、5xx 重試用盡）→ `run.FailureError{MODEL_UNAVAILABLE}`，其餘 `TURN_PIPELINE_ERROR`；ctx 結束的錯誤原樣回傳（Dispatcher 依 job ctx 與 run 狀態收斂成 WORKER_LOST 或 CANCELLED）。沒有降級：模型沒有給出可用答案就失敗退額度，空抽取是成功。分類樹讀不到也是失敗。
- 每輪都寫新版本（`parent_revision_id` 指向被取代的），`turns.optional_streak`、`skipped_slots` 讀上一輪、寫這一輪（A9-D25(1)）。
- 品項解析：只有一個命中才確認（`CATEGORY_CONFIRMED`，存分類 ID）；0 個或多個命中停在中斷，卡片最多 `planner.MaxItemOptions` 個末層品項（label 帶大類路徑）加「其他」，開放自由輸入。出卡只有 `buildCard` 一套規則：命中 >5 且分散在 ≥2 個大類先問大類（stage `GROUP`，候選是大類節點，依最佳命中排序、最多 5），其餘列末層前 5（`ITEM`）；「其他」或打字查無結果進 `DESCRIBE`（無選項、`Query` 保留）。中斷 payload（`InterruptPayload`）是接續的唯一狀態，內含當時渲染好的 `question`。回答用 `ApplyAnswer`（純函式，唯一 I/O 是 resolver）：只認 payload 卡上的選項、選大類只在該大類底下找末層（不以大類當結果）、DESCRIBE 查無結果才是 `BUYER_DESCRIBED`；答案無效回 `OutcomeInvalid` 加原因代碼。executor 與 transaction 收尾由 3b-2e 接。
- 需求狀態：`searchAffectingFields`（品項解析、數量、單位、縣市、交期）是 MATCHING 時改了就退回的集合，3b-2d 依搜尋實際輸入修正；候選集的過期由「候選集的 `request_revision_id` 不是現行版本」表示，不另設欄位。品項未解析不算齊全。
- 自動標題 `品項｜縣市`（以 rune 截 40）只在 `title_source = 'AUTO'` 時寫（SQL 條件），品項未知不寫。
- 語系取自 `agent_run_inputs.locale`（儲存代碼 `ZH_TW`／`EN`，`run.LocaleCode`／`LocaleOf`），不讀 ctx。
- `turn_result` 與中斷 payload 的線格式是手工 proto JSON（`wire.go`）：`testdata/*.golden.json` 釘住產生器輸出，`internal/httpapi/procurement_wire_test.go` 以 protojson 解析同一批檔案；刻意改格式用 `UPDATE_GOLDEN=1`。
- `TurnResult.quota`、`steps`、`evidence_refs` 不填；`agent_runs.result_revision_id` 不寫（3b-2e 決定留或刪）。
