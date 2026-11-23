# procurement 模組行為規格

本文是單體 `procurement` 模組行為的唯一來源。目前有資料層與 run 帳本（第 2 節）：買家採購（RFQ）與賣家報價；採購的另一半（賣家回報價）也在這裡。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：ordering、catalog、buyer、seller；不依賴 notification（notification 依賴 procurement，方向見 `internal/arch/modules.go`）。
- 公開介面與行為隨 A9-2 到 A9-4 的實作補上。

## 2. run 帳本與串流（A9-3b）

- 送出（`run.Submitter`）在同一個 transaction：以 `(buyer_id, client_request_id)` 冪等建 run（重送回第一次的 run，內容不同回 INVALID_ARGUMENT，不再扣額度）、`ConsumeUsage` 扣一次額度（用完回 RESOURCE_EXHAUSTED／`AGENT_MONTHLY_QUOTA_EXCEEDED`，run 一併回滾）、寫輸入與 `run_accepted` 事件、排入 River job（`MaxAttempts` 1）。只有 TURN 的送出取代同一個需求（`request_id`）裡等待輸入的 run（A9-D23(2)）：它們轉 CANCELLED（不退額度）、PENDING 中斷轉 CANCELLED，進行中的回答 run 一併要求取消；其他 run 類型不動它們。取消、取代、`settleFailure`、reconciler 依同一個鎖順序：提問的 run、中斷、回答的 run；回答 run 的提交順序於 CLARIFICATION_RESUME executor 接上時定。模組內的完整順序是「買家送出鎖→run→中斷→版本列→需求列」：送出之後鎖 run 的路徑一律先鎖 run，TURN 與 CANDIDATE_SEARCH 的提交在 run 之後依序鎖現行版本列（取代現行版本）、再由條件式 UPDATE 鎖需求列；兩者順序相同，才與彼此串行而不互鎖。額度的上限與期間由呼叫端依買家方案給。
- worker（`run.Dispatcher`）領走 QUEUED（條件式 UPDATE）、呼叫 `Executor`（LLM 與 provider 呼叫在 transaction 外）；執行失敗標 FAILED 並退回額度：job 的 ctx 被關機或 Timeout 取消（不是買家取消）是 `WORKER_LOST`，其餘候選搜尋 `SEARCH_FAILED`、其他 `TURN_PIPELINE_ERROR`（executor 回 `run.FailureError` 時用它帶的原因，例如 TURN 的 `MODEL_UNAVAILABLE`）；已要求取消的收斂到 CANCELLED。已終結或已被刪除（BuyerDeleted）的 run 再派一次不動作。送出時同一個對話裡等待輸入的舊 run 在同一個 transaction 轉 CANCELLED（不退額度，A9-D23）。
- 事件：序號由 run 列配發，run 是否還活著由 River job 的狀態決定，不看 `updated_at`；狀態轉終態與 `finished` 事件在同一個 transaction，所以看得到終態就讀得到結尾。串流只有一個結尾事件 `finished`（帶狀態，失敗時帶失敗原因；成功、失敗、取消都以它收尾），送出後結束；串流中途 run 被刪除回 NOT_FOUND。每條最長 `Window`（100 秒）。cursor 是事件序號的十進位字串。
- 保留：run 結束 7 天後（`RetentionPeriod`）由每小時的週期 job 刪事件並把 `min_available_seq` 推到 `last_event_seq + 1`。需要的第一筆事件（cursor + 1）小於 `min_available_seq` 回 `REPLAY_CURSOR_EXPIRED`（含從頭開始但事件已清掉）；cursor 大於 `last_event_seq` 回 `REPLAY_CURSOR_AHEAD`。
- 狀態轉換：唯一的守衛函式（公開入口 `Ledger.Transition`，`Ledger.Interrupt` 與回答的提交點共用同一個內部轉換），允許的邊 QUEUED→RUNNING、CANCEL_REQUESTED、FAILED；RUNNING→WAITING_FOR_INPUT、SUCCEEDED、FAILED、CANCEL_REQUESTED；CANCEL_REQUESTED→CANCELLED、FAILED；WAITING_FOR_INPUT→SUCCEEDED（只由回答的提交點寫）、CANCELLED（沒有逾時，interrupt 不設到期）；其餘是終態，終態吸收，之後的事件寫入回 `ErrRunTerminal`。公開的 `Transition` 拒絕轉入 WAITING_FOR_INPUT（只能經 `Interrupt`，要同時有中斷）與 WAITING_FOR_INPUT→SUCCEEDED（只在回答的提交點，要同時解決中斷）。
- job：每個 run 一個 River job（同交易排入，job id 存在 run，`MaxAttempts` 1，Timeout 2 分，River 的 `RescueStuckJobsAfter` 3 分）；ResumeRun 開普通的回答 run 與新 job。取消＝轉 CANCEL_REQUESTED 並在同交易取消 job，executor 收到 ctx 取消後轉 CANCELLED；River 回報 job 已是 cancelled（還沒被領走）的同交易直接轉 CANCELLED；等待輸入的 run 沒有 job，直接轉 CANCELLED；買家取消不退額度。
- reconciler：每分鐘，只看 QUEUED、RUNNING、CANCEL_REQUESTED 的 run，逐列交易、錯誤記錄後繼續，讀 River job 的狀態：job discarded 或不存在標 FAILED／`WORKER_LOST` 並退額度；job 已完成而 run 未終態（executor 違約）標 FAILED 並退額度；CANCEL_REQUESTED 且 job 已取消、完成、discarded 或不存在轉 CANCELLED（不退額度）；job 還在進行中略過；WAITING_FOR_INPUT 沒有 job，不對帳。每個終態轉換同交易寫 `finished`。沒有時間門檻。
- 共用 listener：一條專用連線 `LISTEN procurement_run`，通知只是提示，事件以資料庫為準；訂閱先於讀取，之後不漏；重連後喚醒全部訂閱者。**每程序上限 500 條訂閱**（`run.MaxSubscriptions`），超過回 RESOURCE_EXHAUSTED。500 是推論值：連線數與訂閱數無關，上限擋的是 goroutine、channel 與每次通知的事件查詢。
- run 類型與額度（A9-D24）：只有 TURN 扣額度；CANDIDATE_SEARCH 與回答中斷的子 run（CLARIFICATION_RESUME）不扣。`agent_runs.quota_period_start` 只在扣額度時有值（CHECK：`run_type = 'TURN'` 等價於它非 NULL），所有退款路徑（Dispatcher 失敗、reconciler 的 WORKER_LOST 與違約、`FailAndRefund`）都經 `Ledger.FailAndRefund`，只在它非 NULL 時退。不寫 run 產出（沒有 artifact 表）。
- 每買家上限（A9-D24(6)）：同時進行中的 run 最多 2、每分鐘送出最多 10（`run.Limits`，零值取預設），同一個需求同時只有一個進行中的 run（先檢查需求，再檢查買家的進行中數）。進行中指 QUEUED、RUNNING、CANCEL_REQUESTED，WAITING_FOR_INPUT 不算（沒有 job、不佔 LLM 與 worker，算進去會讓買家被自己放著的提問擋住新的送出）。檢查在送出的 transaction 內、同一位買家的 advisory lock（`LockBuyerSubmits`）之下，對資料庫計數，所以多 pod 並發也不超過；每分鐘數用 `agent_runs.created_at`（含子 run）而不是記憶體計數，避免重啟或多 pod 時各算各的。買家的進行中已滿或每分鐘已滿回 RESOURCE_EXHAUSTED（帶 retry-after），同需求已有進行中回 FAILED_PRECONDITION／`PROCUREMENT_RUN_IN_PROGRESS`。冪等重送在上限檢查之前回放，不受上限擋。
- 回答中斷（`Submitter.Resume`）：回答成功才解決中斷（A9-D26）。送出只建回答 run（`interrupt_id` 指向所答的中斷、CLARIFICATION_RESUME、不扣額度、新 job、以 `(buyer, client_request_id)` 冪等），不解決中斷也不動提問的 run，提問的 run 維持 WAITING_FOR_INPUT。提交點在 `Ledger.Transition`：回答 run 轉 SUCCEEDED 或寫入自己的新中斷（`Ledger.Interrupt`）的同一個 transaction 內，被回答的中斷轉 RESOLVED、提問的 run 經一般守衛轉換由 WAITING_FOR_INPUT 轉 SUCCEEDED（寫 `finished`）；鎖順序提問的 run、中斷、回答的 run（提交點與取消都照這個順序，回答 run 轉換前先鎖提問的 run 與中斷）。提交時中斷已不是 PENDING（提問的 run 在回答期間被取消）回 `ErrInterruptNotPending`，整個提交回滾，回答 run 留在原狀態由 executor 標失敗。回答 run FAILED 或 CANCELLED 不做任何還原：中斷仍 PENDING、提問的 run 仍等待，買家用新的 `client_request_id` 再答。重送：同 `client_request_id` 回放（不論回答 run 的狀態）；回答進行中新的 `client_request_id` 回 `PROCUREMENT_RUN_IN_PROGRESS`（同需求單一進行中 run）；成功後回 `APPROVAL_ALREADY_RESOLVED`；中斷已被取消或提問的 run 不在等待輸入回 `RUN_NOT_ACTIVE`；別人的中斷回 NOT_FOUND，不另設 `interrupt_id` 唯一索引。
- 回答中斷的入口（`Module.ResumeRun`，A9-D30）：輸入是 `ClarificationResponse` 的 `OptionID`、`FreeText` 加中斷與 `ClientRequestID`。同步檢查形狀（不讀分類樹）：`OptionID` 與 `FreeText` 恰一個、文字去空白後非空且不超過 2000 字元、選項在該中斷 payload 的卡上，不合回 INVALID_ARGUMENT 並指出欄位（`OptionID`、`FreeText`），不新增 ErrorReason、不建 run；之後依上一條建回答 run（輸入存 `agent_run_inputs`：文字回答存 `buyer_message`、選項回答存 `option_id` 且 `buyer_message` 為空字串，CHECK 要求恰一個），立即回傳 run ID 與當下狀態，不等結果（`ResumeRunResponse.result` 已 `reserved` 刪除（A9-D30），client 用 WatchRun 或 GetRuntimeSnapshot）。
- CLARIFICATION_RESUME executor（`turn.AnswerExecutor`，只需要分類樹，不呼叫模型）：`ApplyAnswer` 套用到中斷 payload；解析成功寫 `source = ANSWER` 的新版本（存 `item_resolution_category_id`）與輪次（買家訊息是選項標籤或文字原文），經 TURN 的 commit 到 SUCCEEDED 或新的中斷；答案被拒（`OutcomeInvalid`）run 以 `ANSWER_REJECTED` 失敗，中斷維持 PENDING、不還原；不扣額度。TURN 與回答 run 共用的 commit 在 transaction 開頭先 `Ledger.LockAskingOf`，回答 run 的鎖順序因此是提問的 run、中斷、回答的 run。
- 中斷（`Ledger.Interrupt`）：同一個 transaction 寫 `turn_result` 事件（可省）、把 RUNNING 的 run 轉 WAITING_FOR_INPUT（回答 run 在此提交所答的中斷）、再新增 PENDING 的 CLARIFICATION 中斷（payload 是接續的唯一狀態）。`agent_run_interrupts.request_id` 加部分唯一索引，每個需求最多一筆 PENDING。狀態 PENDING／RESOLVED（回答成功）／CANCELLED（提問的 run 被取消或被同需求的新一輪 TURN 取代，同交易轉，不佔索引；正在回答它的 run 在同交易要求取消，同 CancelRun 的路徑：CANCEL_REQUESTED 並取消 job，job 已死直接 CANCELLED）；`closed_at` 是離開 PENDING 的時間。讀取只認 WAITING_FOR_INPUT 的 run 的 PENDING 中斷（`OpenInterruptForRun`、`OpenInterruptForRequest`），並回 `Processing`：有回答它的 run 在 QUEUED、RUNNING 或 CANCEL_REQUESTED 時為真（讀取模型呈現「處理中」，否則「等回答」）。
- 執行者登記：`Dispatcher.Executors` 依 run 類型登記；領到沒有登記執行者的類型，記 log 並依該類型的 pipeline 失敗收斂（TURN 退額度，其餘不退），不留在 RUNNING 等 reconciler。接線：`cmd/hawkr` 的 `addProcurementWorkers`（worker 角色）登記執行 run 的 worker、reconciler 與清除；`startProcurementHub`（serve 角色）啟動共用 LISTEN；兩個角色的 `newProcurement` 都給 job client。
- BuyerDeleted：先刪 `agent_runs WHERE buyer_id`（輸入、事件、interrupt 隨 CASCADE），再把其他表的 `buyer_id` 設 NULL；`agent_runs.buyer_id` 維持 NOT NULL（A9-D31 Q3：發布不建 run，`agent_runs` 沒有 `publication_id`）。

## 3. TURN 執行（A9-3b-2c）

- executor（`internal/turn`）由 `procurement.New` 在給了 `Deps.LLM` 與 `Deps.Taxonomy` 時登記（只給其中一個 panic；`Executors` 已有 TURN 的項目則保留）。worker 角色的 `cmd/hawkr` 給採購用的 `llm.Procurement` 與經 `internal/legacy/categoryservice` 轉接的分類樹；serve 角色不執行 run，不給。
- 一輪：一次模型呼叫（`internal/extract`：Gemini 走 response schema、DeepSeek 走強制 tool call；買家訊息截 2000 rune；單次 30 秒、含退避總預算 70 秒，呼叫內最多重試一次，只對逾時、429、5xx、連線層錯誤、格式錯誤與 ErrorCode 為 OTHER／MALFORMED_FUNCTION_CALL（輸出不明或壞掉）；4xx、停用與安全類 ErrorCode（SAFETY、PROHIBITED_CONTENT、RECITATION、BLOCKLIST、SPII、MAX_TOKENS、content_filter）不重試；抽取結果不做簡轉繁）→ 套用欄位 → 品項變更或未解析時查分類樹 → 規劃與渲染 → 單一 transaction 提交。模型沒有給出可用答案（含重試用盡）一律失敗並退額度：逾時、429、5xx 用盡是 `MODEL_UNAVAILABLE`，其餘（含帶 ErrorCode 的回應用盡後）`TURN_PIPELINE_ERROR`；空抽取是成功。沒有降級。
- 提交：新版本（`request_revisions`，品項解析存 `item_resolution_category_id`，BUYER_DESCRIBED 為 NULL）、`turns`、需求狀態、對話預覽與自動標題（`品項｜縣市`，40 rune，只在 `title_source = 'AUTO'` 時寫）、`turn_result` 事件與 run 的終態轉換同一個 transaction；等待輸入時改寫中斷（`InterruptInput.ID` 由呼叫端預先產生，事件內的 `interrupt.interrupt_id` 等於它）。
- 需求狀態：依齊全與否在 CLARIFYING 與 REQUIREMENT_READY 之間；需求在 MATCHING 時，只有品項解析改了才退回，其餘（數量、單位、縣市、交期、預算等）維持 MATCHING，因為候選搜尋只用品項解析當輸入（A9-D29(2)）。
- 語系：送出時存進 `agent_run_inputs.locale`（`ZH_TW`／`EN`），executor 用它渲染伺服器句子。
- 事件：TURN 只有 `run_accepted`、一個 `turn_result`、`finished`；不發 step 與 activity。

## 4. 候選搜尋（A9-3b-2d，A9-D29）

- 開始：`Module.StartCandidateSearch`（`internal/search` 的 `Starter`），經 `run.Submitter` 的送出路徑，在買家的送出鎖之內同一個 transaction 先驗證需求屬於買家且狀態允許（別人的需求不因對方有進行中的 run 而洩漏），再檢查同需求的進行中 run 與買家上限，最後讀現行版本並建候選集（不鎖需求列）（指向需求當下的現行版本）與 `CANDIDATE_SEARCH` run 並排入 job。需求不屬於買家回 `NOT_FOUND`；需求不在 `REQUIREMENT_READY` 或 `MATCHING`（或現行版本沒有品項解析）回 `FAILED_PRECONDITION`／`REQUIREMENT_NOT_READY`；同需求已有進行中 run 回 `PROCUREMENT_RUN_IN_PROGRESS`；每買家上限同 TURN；不扣額度；Start 不改需求狀態；CANDIDATE_SEARCH 沒有買家訊息，不寫 `agent_run_inputs`（executor 不讀它），重送只比對需求與 run 類型。冪等以 run 的 `(buyer_id, client_request_id)` 為準，重送回同一個 run 與它的 `candidate_set_id`，內容不同回 `INVALID_ARGUMENT`。
- executor（`internal/search`）由 `procurement.New` 在給了 `Deps.Search`（商品搜尋 port）時登記，必須同時有 `Deps.Sellers` 與 `Deps.Taxonomy`（否則 panic）；worker 角色的 `cmd/hawkr` 以 `internal/legacy/productservice` 轉接商品搜尋。
- 輸入只取自候選集所指需求版本的品項解析：`CATEGORY_CONFIRMED` 用分類 ID、不帶關鍵字；`BUYER_DESCRIBED` 用解析的 `Label` 當關鍵字。搜尋前以分類樹確認已確認的分類仍是末層：一律用 `Resolver.LeafAsOf(分類 ID, notBefore)`，`notBefore` 是候選集所指版本的 `created_at`：快取樹開始讀取的時間早於 `notBefore` 或已超過 TTL 時同步重讀（不論快取答案是肯定或否定），只有夠新的樹才能判定分類失效；拿不到這樣的樹回錯誤，run 以 `SEARCH_FAILED` 收斂，不清解析。
- 翻頁：只取上架商品、依銷量（`SALES`）排序、每頁 50 筆、page 從 1 起；每頁處理後累計不重複且通過過濾的店，滿 20 家（`candidate.MaxStores`）、最後一頁（回傳少於 50 筆）或翻滿 5 頁（`candidate.MaxPages`）即停；任一頁失敗整次搜尋失敗。一店只留第一次出現的商品（`source_id` 是商品 ID），`rank` 從 0 起依出現順序。20 與 5 是推論值。
- 過濾（transaction 外，fail-closed）：先 `candidate.KeepOpen`（只收 seller 判定可公開曝光的店），再排除封鎖這位買家的店（`seller.StoresBlockingBuyer`，依 `seller.MaxBatchSize` 分批）；「滿 20 家」以過濾後計。任一讀取失敗即搜尋失敗。
- 提交（單一 transaction）：`LockRun`（取消搶先時整筆回滾）→ 鎖現行版本列（`LockActiveRevision`）並確認它的 ID 仍等於候選集的 `request_revision_id`（不等回錯誤，run 以 `SEARCH_FAILED` 收斂、需求狀態不動）→ 寫各候選 → 需求轉 `MATCHING`（條件式 UPDATE 鎖需求列，順序在版本列之後）（來源 `REQUIREMENT_READY` 或 `MATCHING`）→ run 轉 `SUCCEEDED`。零結果也是成功。外部呼叫（搜尋、店家狀態、封鎖、分類樹）都在 transaction 外。
- 失敗原因：搜尋或過濾失敗、版本已變、提交失敗是 `SEARCH_FAILED`（CANDIDATE_SEARCH 不扣額度，沒有額度可退）；已確認的分類不再是末層是 `ITEM_CATEGORY_UNAVAILABLE`：同一個 transaction 寫一個清掉品項解析的新版本（品項文字保留）、需求轉 `CLARIFYING`、run 轉 `FAILED`，executor 回傳 nil。
- 事件：只有 `run_accepted` 與終態轉換寫的 `finished`，不發 step 與 activity。
- 有效的候選集：需求在 `MATCHING` 時最新一筆 run 成功的候選集；重搜後新候選集從未勾選開始。候選不存事實：符合與需向賣家確認的事實讀取時由需求現行版本組出（`reply.CandidateFacts`：有品項→符合「品項」；預算、數量、縣市、交期有值→各一筆需向賣家確認），`facts_time` 是這次搜尋完成的時間。讀取端與 `ToggleSelection` 由 A9-3b-3 接。

## 5. 讀取模型 GetRuntimeSnapshot（A9-3b-2e，A9-D30）

- `Module.GetRuntimeSnapshot(buyerID, conversationID)`（`internal/snapshot`）在一個唯讀 REPEATABLE READ 快照內讀齊；對話不存在、已刪除或屬於別的買家回 NOT_FOUND。回傳 `model.RuntimeSnapshot`（對應 `GetRuntimeSnapshotResponse`，狀態、角色、類型是 proto enum 值名去掉前綴的代碼）。
- 訊息：每一輪（`turns`，依時間）拆成買家與助理兩則，空的不出；買家訊息的 ID 是該輪的 `client_request_id`，助理訊息的 ID 是輪次 ID（等於 `turn_result.message_id`）；回答中斷的一輪，買家訊息是答案（選項標籤或文字）。`steps` 不填。
- `runs`：目前需求的 run，新到舊，最多 20 筆，不分頁；`last_cursor` 是 `last_event_seq` 的 cursor；`terminal` 依狀態。`active_run_id`：需求目前 QUEUED、RUNNING 或 CANCEL_REQUESTED 的最新一個 run（等待輸入的 run 不算），沒有時不填。
- `open_confirmation`：`OpenInterruptForRequest` 的 PENDING 中斷，問題取自 payload 的 `question`，`processing` 為真代表有回答它的 run 進行中。
- `published_rfq`：需求最新一次發布（`published_at` 新到舊）的 ID、版本、時間、受眾數（核准凍結的受眾列數，公開池 0）、邀請數與確認快照 ID；`rfq_snapshot_hash` 由發布的 `schema_version`、確認快照、它確認的需求版本與價格預覽以 `rfq.SnapshotHash` 重算，與 `ConfirmRfqSnapshotResponse.payload_hash` 同值。

## 6. SubmitTurn（A9-3b-2f，A9-D13、D24、D26）

- 入口 `Module.SubmitTurn`（`internal/turn.Submitter`）：輸入是買家 ID、選填的 `RequestID`、訊息、`ClientRequestID`（冪等鍵）與語系；回需求 ID、對話 ID 與這一輪 TURN run 的 ID，立即回傳，結果從 WatchRun 取得。handler 的 `request_id` 接受任何有效 UUID（伺服器以 UUIDv7 建需求），空字串表示新建；`client_request_id` 必須是 UUIDv4。
- 訊息去頭尾空白後不得為空、不得超過 2000 字元（`MaxMessageRunes`，與 `agent_run_inputs.buyer_message` 的 CHECK 一致），不合回 INVALID_ARGUMENT，欄位 `Message`。
- 沒有 `RequestID`：在同一個 transaction 新建需求與對話（兩者 ID 都是 UUIDv7）；之後送出若失敗（額度、買家上限、同需求進行中），需求與對話一起回滾，不留空需求。
- 有 `RequestID`：需求不存在、屬於別的買家或需求沒有（未刪除的）對話都回 NOT_FOUND。
- 額度：transaction 內先取買家的送出鎖（`LockBuyerSubmits`）、解析需求與對話（含重送偵測），非重送才讀買家模組的有效權益（`Deps.Entitlements`，沒給時 SubmitTurn 回內部錯誤，不當成不限額；重送不讀，所以買家關閉或權益讀取失敗時重送仍回放），然後經 `run.Submitter` 建 run、扣一次額度、排入 job；額度用完、買家上限、同需求進行中依 `run.Submitter.Submit`。
- 重送同一個 `ClientRequestID`：回第一次的需求、對話與 run，不再建需求、不再扣額度；沒指定 `RequestID` 時沿用第一次的，指定了別的需求或內容不同回 INVALID_ARGUMENT（指定的需求不存在、不屬於買家或沒有對話則回 NOT_FOUND，先於內容比對）；重送時對話已被刪除回 NOT_FOUND（與非重送路徑一致）。

## 7. 讀取與對話管理（A9-3b-3a，A9-D31 Q7–Q9）

- `GetSnapshot(buyerID, requestID)`（`snapshot.Reader.GetByRequest`）以需求為鍵：現行版本的欄位（`slots.FromRevision`）、`MissingRequired`、run、進行中的 run、待回答的中斷與最新的發布結果。需求不存在、不屬於買家，或它的對話已被買家刪除回 NOT_FOUND（契約沒寫刪除對話後需求能否由 request_id 讀取，這裡取較保守的 NOT_FOUND）。`PublishedRfq.RfqSnapshotHash` 與 `GetRuntimeSnapshot` 相同，由 `internal/rfqhash` 對確認快照、它確認的需求版本與價格預覽重算（SHA-256 對固定欄位順序的 JSON）；3c 的 `ConfirmRfqSnapshot` 必須以同一個函式算 `payload_hash`。
- `GetConversationMessages` 與 `GetRuntimeSnapshot` 的訊息同源（`snapshot.messagesOf`）。
- `ListConversations`：keyset `(updated_at DESC, id DESC)`，走索引 `conversations_buyer_active`；`page_size` 0 是 20、超過 50 以 50 計、負數與壞 `page_token` 回 INVALID_ARGUMENT（欄位 `page_size`、`page_token`）；token 是 `(updated_at 微秒, id)` 的 base64url JSON，不透明。
- `RenameConversation`：去頭尾空白後 1 到 40 字元、不含 `\n`、`\r`，否則 INVALID_ARGUMENT（欄位 `title`）；DB 的 `conversations_title_check` 同為 40。目標狀態的方法，不建收據。
- `DeleteConversation`：同一個 transaction 依序取買家送出鎖、`Canceller.CancelAllOfRequest`（需求還沒結束的 run，含等待輸入的，逐個走 `CancelRun` 的路徑，買家取消不退額度）、軟刪除對話；已發布的詢價與接受單不動。已刪除的重送回成功，不存在或不是本人回 NOT_FOUND。
- `GetCandidateSearch(buyerID, requestID, runID, locale)`（`search.Reader`）：run 不存在、不是本人、不是 CANDIDATE_SEARCH 或不屬於該需求回 NOT_FOUND。候選集在建 run 時就建好，所以每個狀態都有 `CandidateSetID`；只有 SUCCEEDED 有候選（事實由需求現行版本讀取時組出，`FactsAsOf` 是 run 完成的時間），FAILED 帶失敗原因，其餘狀態只回狀態。店家卡片在 httpapi 組（`seller.Module.StoreCards`）；沒公開過的店沒有卡片，候選照列、不設 `store`。

## 8. 需求編輯（A9-3b-3b，A9-D31 Q1–Q2、A9-D33）

- `ToggleSelection`（`search.Selector`）：目標狀態的方法，重送結果相同。同一個 transaction 鎖需求列，需求不屬於本人、對話已刪除、候選集不屬於這個需求或候選不在候選集裡回 NOT_FOUND；候選集不是有效的候選集（需求不在 MATCHING，或不是最新一筆成功搜尋的候選集）回 ABORTED／`VERSION_CONFLICT`，`conflictTarget` 是 `candidate_set`。
- `SubmitAnswer`、`Rewind`（`turn.Direct`）同步、不建 run、不扣額度。transaction 內依序鎖買家的送出、現行版本、需求列；先看重送（輪次存在才回放），再守衛：待回覆的中斷 FAILED_PRECONDITION／`APPROVAL_PENDING`、同需求進行中的 run `PROCUREMENT_RUN_IN_PROGRESS`、讀到的版本不是現行版本 ABORTED／`VERSION_CONFLICT`（`conflictTarget` 是 `request_revision`）；需求不屬於本人或沒有對話 NOT_FOUND。分類樹讀取在 transaction 外。
- 重送回放 `turns.turn_result`，位元組與第一次相同；同一個 `client_request_id` 內容不同（需求、來源、回答、欄位（`turns.command_slot`）、讀到的版本任一不同，或鍵已被 run 用掉）回 INVALID_ARGUMENT（`client_request_id`）。
- SubmitAnswer 的欄位：ITEM（文字，須唯一對到一個末層分類，否則 INVALID_ARGUMENT `free_text`）、QUANTITY（數字文字與單位選項至少一個；缺的一邊現況要已有）、PACKAGING、BUDGET_PER_UNIT（整數文字）、DELIVERY_CITY（選項或文字）、DELIVERY_DATE（文字）。選項只認 `planner.BuildFor` 的白名單；略過只給可略過的欄位（PACKAGING、BUDGET_PER_UNIT），記進 `turns.skipped_slots`；值被 `slots.Apply` 拒絕不是錯誤，記在 `TurnResult.rejected`。
- Rewind 的欄位同上（UNIT 視同 QUANTITY）。新版本內容同目標版，問題是倒回那一格（`planner.BuildFor`），連續選填計數歸零；需求狀態依 `nextStatus`：MATCHING 時品項解析變了才退回。

## 9. 詢價發布鏈（A9-3b-3c，A9-D31 Q3–Q6）

- 四支方法（`internal/rfq`）各自一個 transaction，依序鎖現行版本與需求列（順序同 `turn.Direct`）：需求不屬於本人或對話已刪除 NOT_FOUND；讀到的需求版本不是現行版本 ABORTED／`VERSION_CONFLICT`，`conflictTarget` 是 `request_revision`。每支先看冪等鍵（建立的實體上，`(buyer_id, client_request_id)`）：命中就回第一次的結果，先於版本檢查；內容不同回 INVALID_ARGUMENT（`client_request_id`）。`ON CONFLICT DO NOTHING` 寫入 0 列（同鍵的並行請求搶先提交）時重讀以它為準。
- `PreviewRfqPrice`：區間下界與上界是正整數且下界不大於上界（INVALID_ARGUMENT）；需求沒有單位（`basis_unit` NOT NULL，算不出計價基準）FAILED_PRECONDITION／`REQUIREMENT_NOT_READY`；GRAM 以每 1000 計價，其餘每 1；有數量時總額 = 單價 × 數量 ÷ 計價基準四捨五入到元，溢位 FAILED_PRECONDITION（無 reason）；沒有數量不設總額。有效 15 分鐘。回的 `payload_hash` 是預覽的 hash。
- `ConfirmRfqSnapshot`：補充說明去頭尾空白，上限 500 字元；預覽不屬於本人 NOT_FOUND；預覽屬於別的需求、對應的不是現行版本、已過期、`payload_hash` 不等於由預覽列重算的值回 `VERSION_CONFLICT`（`conflictTarget` 是 `price_preview`）。快照有效 24 小時（目前沒有讀取者擋它，proto 沒寫到期後的行為）；回的 `payload_hash` 是快照的 hash。
- `PreviewRfqPublication`：先驗欄位形狀（公開池不帶候選集與候選；指定對象要有候選集與至少一個候選，不重複；回價期限是合法日期），再讀庫。快照不屬於本人 NOT_FOUND；快照屬於別的需求或確認的不是現行版本 `VERSION_CONFLICT`（`rfq_snapshot`）。指定對象：候選集不屬於本人的這張需求 NOT_FOUND；不是有效的候選集（同 `ToggleSelection`，`search.IsEffective`）`VERSION_CONFLICT`（`candidate_set`）；候選不在候選集裡 INVALID_ARGUMENT（`selected_candidate_ids[i]`）；受眾依選擇順序凍結店家 ID。公開池政策是 `WISH_POOL_ALL_ACTIVE_SELLERS_V1`／1，預覽沒有收件人（`recipient_count` 0），收件人與封鎖檢查在 A9-3c（A9-D29(5)）。核准有效 24 小時，只存參照、模式、政策與期限；賣家可見內容與兩個 hash 由核准參照的列重算。
- `ExecuteRfqPublication`：核准不屬於本人 NOT_FOUND；核准是別的需求的、`ClaimApprovalForExecute` 的 CAS（PENDING、版本、有效期）沒命中、核准凍的需求版本已不是現行版本、`payload_hash` 不等於重算的賣家可見內容 hash、指定對象的受眾名單是空的，都回 `VERSION_CONFLICT`（`publication_approval`），整筆 rollback，核准維持 PENDING。成功時同一個 transaction 寫發布（不建 run）、依受眾順序寫 QUEUED 的邀請、把核准標 SUCCEEDED、發 `RfqPublished` 事件（含每個邀請）；任何一步失敗全部還原。重送（鍵已有發布）先於其餘檢查，回第一次的發布（`replayed`），不再建邀請與事件。需要 `Deps.Events`，沒給是組裝錯誤。
- hash（`internal/rfqhash`）：SHA-256 對固定欄位順序的 JSON，原像含 `schemaVersion`（`rfq_publication_approvals.schema_version`、`rfq_publications.schema_version`），欄位集合依版本分派，不認得的版本回錯誤；預覽、快照、受眾、賣家可見內容各一個原像型別。
