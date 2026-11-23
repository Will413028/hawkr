# turn 子領域

TURN 與 CLARIFICATION_RESUME run 的 executor（A9-D24(2)、D28、D30）：TURN 一次模型呼叫抽欄位，其餘是純函式，結果在單一 transaction 提交。規格見 `backend/docs/modules/procurement.md` 的 TURN 節。

## 不變量

- 流程固定：讀（`loadPrior`：需求、現行版本、上一輪、價格預覽與確認快照）→ `extract`（transaction 外）→ `decide`（`slots.Apply`、品項變更或未解析時 `resolver.Resolve`、`planner`、`reply`，唯一的 I/O 是 resolver）→ `commit`。LLM 與分類樹讀取不得進 transaction（單體規則 5；`TestSuccessWrites…` 斷言）。
- `commit` 的 transaction 先 `Ledger.LockAskingOf`（回答 run 的鎖順序是提問的 run、中斷、自己；TURN 只讀不鎖），再 `LockRun` 自己的 run，再寫：取代現行版本、新版本、`InsertTurn`（0 列＝冪等衝突；`turns.turn_result` 是 TurnResult 的 wire JSON，任何來源都寫，與事件同一份，A9-D33(2)；共用寫入 `writeTurn`）、需求狀態、對話預覽、自動標題，最後 `Ledger.Append(turn_result)`＋`Transition(SUCCEEDED)` 或 `Ledger.Interrupt`。任何一步失敗整筆回滾，由 Dispatcher 收斂；中斷寫入失敗不得退回成功。
- 失敗分類：`extract.Error.Unavailable`（逾時、429、5xx 重試用盡）→ `MODEL_UNAVAILABLE`，其餘 `TURN_PIPELINE_ERROR`；ctx 結束的錯誤原樣回傳（Dispatcher 依 job ctx 與 run 狀態收斂）。沒有降級：模型沒給出可用答案就失敗退額度，空抽取是成功；分類樹讀不到也是失敗。
- 每輪都寫新版本。`parent_revision_id` 是內容祖先，不是被取代者：新版指向它讀到的現行版，倒回的新版指向目標版（取代順序是 `revision_no` 與現行，A9-D33(3)）。`turns.optional_streak`、`skipped_slots` 讀上一輪、寫這一輪（A9-D25(1)）。
- 品項解析：只有一個末層命中才確認（`CATEGORY_CONFIRMED`，存分類 ID）；多個命中停在中斷，0 個命中問 `DESCRIBE`（無選項、`Query` 保留）。大類、中間層節點與無子節點的大類都不算命中（`leavesOnly`）。
- 出卡只有 `buildCard`：命中 >5 且分散在 ≥2 個大類先問大類（`GROUP`，最多 5），其餘列末層前 5（`ITEM`，label 帶大類路徑，加「其他」，開放自由輸入；同屬一個大類時 payload 記 `rootId`）。
- 中斷 payload（`InterruptPayload`）是接續的唯一狀態，內含渲染好的 `question`。`ApplyAnswer`（純函式，唯一 I/O 是 resolver）只認卡上的選項；選大類只在其底下找末層；「其他」或打字查無結果進 `DESCRIBE`，DESCRIBE 查無結果才是 `BUYER_DESCRIBED`；無效回 `OutcomeInvalid` 加原因代碼。
- ITEM 選項確認前以分類 ID 查現行分類樹（`Resolver.Leaf`），已不是末層就用現在的命中重新出卡（有 `rootId` 時只在該大類內，1 個確認、0 個 `DESCRIBE`）。
- 需求狀態：`searchAffectingFields` 只剩品項解析（A9-D29(2)），MATCHING 時改了才退回，其餘欄位改了維持 MATCHING；有效的候選集＝需求在 MATCHING 時最新一筆成功搜尋的候選集，不另設過期欄位。品項未解析不算齊全。
- 自動標題 `品項｜縣市`（rune 截 40）只在 `title_source = 'AUTO'` 時寫，品項未知不寫；語系取自 `agent_run_inputs.locale`（`run.LocaleOf`），不讀 ctx。
- `turn_result` 與中斷 payload 的線格式是手工 proto JSON（`wire.go`）：`testdata/*.golden.json` 釘住輸出，`internal/httpapi/procurement_wire_test.go` 以 protojson 解析；刻意改格式用 `UPDATE_GOLDEN=1`。
- `TurnResult.quota`、`steps`、`evidence_refs` 不填；`agent_runs.result_revision_id` 不寫。
- 回答中斷（`AnswerExecutor`，不呼叫模型、不扣額度）：`ApplyAnswer` 的結果經 `finish`（與 `decide` 共用）接成 `decision`，`commit` 寫 `source = ANSWER` 的版本與輪次，同交易解決中斷與提問的 run；`OutcomeInvalid` 是 `ANSWER_REJECTED`，中斷維持 PENDING、不還原。
- `ResumeRun`（`Resumer`）同步做 `CheckAnswer` 形狀檢查（A9-D30(1)）：非 PENDING 先交給 run.Submitter.Resume；PENDING 以 `CheckAnswer` 為準加不超過 2000 字元，不合回 INVALID_ARGUMENT 附欄位違規、不建 run；分類解析留給 executor。
- 不經 run 的 `Direct`（SubmitAnswer、Rewind，A9-D31）：同一 transaction 鎖買家的送出、現行版本、需求列，守衛（待回覆中斷、進行中 run、讀到的版本），純函式 `answerDecision`／`rewindDecision`，與 run 共用 `writeTurn`。ITEM 文字須唯一對到一個末層分類，否則 INVALID_ARGUMENT（free_text），不出卡（D33(1)）。重送回放 `turn_result`（同需求、來源、欄位 `command_slot`、回答、版本，不重算，A9-D34）。倒回目標沿祖先鏈算（`rewindTarget`），沒有可倒回回 FAILED_PRECONDITION、無 reason（D33(4)）。
