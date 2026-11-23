# turn 子領域

TURN 與 CLARIFICATION_RESUME 類型 run 的 executor（A9-D24(2)、A9-D28、A9-D30）：TURN 一次模型呼叫抽欄位，其餘是純函式，結果在單一 transaction 提交。行為規格見 `backend/docs/modules/procurement.md` 的 TURN 節。

## 不變量

- 流程固定：讀（run 輸入、需求、現行版本、對話、上一輪、價格預覽與確認快照）→ `extract`（transaction 外）→ `decide`（`slots.Apply`、品項變更或未解析時 `resolver.Resolve`、`planner`、`reply`，唯一的 I/O 是 resolver）→ `commit`。LLM 與分類樹讀取不得進 transaction（單體規則 5；`TestSuccessWrites…` 斷言）。
- `commit` 的 transaction 先 `Ledger.LockAskingOf`（回答 run 的鎖順序是提問的 run、中斷、自己；TURN 只讀不鎖），再 `LockRun` 自己的 run，再寫：取代現行版本、新版本、`InsertTurn`（0 列＝冪等衝突，失敗不繼續）、需求狀態、對話預覽、自動標題，最後 `Ledger.Append(turn_result)`＋`Transition(SUCCEEDED)` 或 `Ledger.Interrupt`。任何一步失敗整筆回滾，錯誤回給 Dispatcher 收斂；中斷寫入失敗不得退回成功。
- 失敗分類：`extract.Error.Unavailable`（逾時、429、5xx 重試用盡）→ `run.FailureError{MODEL_UNAVAILABLE}`，其餘 `TURN_PIPELINE_ERROR`；ctx 結束的錯誤原樣回傳（Dispatcher 依 job ctx 與 run 狀態收斂成 WORKER_LOST 或 CANCELLED）。沒有降級：模型沒有給出可用答案就失敗退額度，空抽取是成功。分類樹讀不到也是失敗。
- 每輪都寫新版本（`parent_revision_id` 指向被取代的），`turns.optional_streak`、`skipped_slots` 讀上一輪、寫這一輪（A9-D25(1)）。
- 品項解析：只有一個末層命中才確認（`CATEGORY_CONFIRMED`，存分類 ID）；多個命中停在中斷，0 個命中問 `DESCRIBE`（無選項、`Query` 保留）。
- 只有末層命中算數：大類、中間層節點與沒有子節點的大類自己都不能被確認，也不計入門檻與分組（`leavesOnly` 是唯一規則）。
- 出卡只有 `buildCard` 一套規則：命中 >5 且分散在 ≥2 個大類先問大類（`GROUP`，候選是大類節點，依最佳命中排序、最多 5），其餘列末層前 5（`ITEM`，label 帶大類路徑，加「其他」，開放自由輸入；候選同屬一個大類時 payload 記 `rootId`）。
- 中斷 payload（`InterruptPayload`）是接續的唯一狀態，內含當時渲染好的 `question`。回答用 `ApplyAnswer`（純函式，唯一 I/O 是 resolver）：只認 payload 卡上的選項；選大類只在該大類底下找末層（不以大類當結果）；「其他」或打字查無結果進 `DESCRIBE`，DESCRIBE 查無結果才是 `BUYER_DESCRIBED`；答案無效回 `OutcomeInvalid` 加原因代碼。
- ITEM 選項確認前以分類 ID 查現行分類樹（`Resolver.Leaf`，不看用詞），已不是末層就用現在的命中重新出卡（有 `rootId` 時只在該大類內，1 個確認、0 個 `DESCRIBE`）。
- 需求狀態：`searchAffectingFields` 只剩品項解析（A9-D29(2)：候選搜尋只用它當輸入），MATCHING 時改了才退回，數量、單位、縣市、交期、預算改了維持 MATCHING；有效的候選集＝需求在 MATCHING 時最新一筆 run 成功的候選集（`internal/search` 寫入時才轉 MATCHING），不另設過期欄位。品項未解析不算齊全。
- 自動標題 `品項｜縣市`（以 rune 截 40）只在 `title_source = 'AUTO'` 時寫（SQL 條件），品項未知不寫。
- 語系取自 `agent_run_inputs.locale`（儲存代碼 `ZH_TW`／`EN`，`run.LocaleCode`／`LocaleOf`），不讀 ctx。
- `turn_result` 與中斷 payload 的線格式是手工 proto JSON（`wire.go`）：`testdata/*.golden.json` 釘住產生器輸出，`internal/httpapi/procurement_wire_test.go` 以 protojson 解析同一批檔案；刻意改格式用 `UPDATE_GOLDEN=1`。
- `TurnResult.quota`、`steps`、`evidence_refs` 不填；`agent_runs.result_revision_id` 不寫（3b-2e 決定留或刪）。
- 回答中斷（`AnswerExecutor`，不呼叫模型、不扣額度）：`ApplyAnswer` 的結果經 `finish`（與 `decide` 共用的尾段）接成同一種 `decision`，`commit` 寫 `source = ANSWER` 的新版本與輪次（買家訊息是選項標籤或去空白的原文），提交點同交易解決中斷與提問的 run；`OutcomeInvalid` 是 `run.FailureError{ANSWER_REJECTED}`，中斷維持 PENDING、不還原。
- `ResumeRun`（`Resumer`）同步做 `CheckAnswer` 形狀檢查（A9-D30(1)）：非 PENDING 先交給 run.Submitter.Resume；PENDING 以 `CheckAnswer` 為準（文字先去空白，選項與非空白文字恰一個，選項在卡上）加不超過 2000 字元，不合回 INVALID_ARGUMENT 附欄位違規、不建 run；分類解析留給 executor。
