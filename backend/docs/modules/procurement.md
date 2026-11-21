# procurement 模組行為規格

本文是單體 `procurement` 模組行為的唯一來源。目前有資料層與 run 帳本（第 2 節）：買家採購（RFQ）與賣家報價；採購的另一半（賣家回報價）也在這裡。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：ordering、catalog、buyer、seller；不依賴 notification（notification 依賴 procurement，方向見 `internal/arch/modules.go`）。
- 公開介面與行為隨 A9-2 到 A9-4 的實作補上。

## 2. run 帳本與串流（A9-3b）

- 送出（`run.Submitter`）在同一個 transaction：以 `(buyer_id, client_request_id)` 冪等建 run（重送回第一次的 run，內容不同回 INVALID_ARGUMENT，不再扣額度）、`ConsumeUsage` 扣一次額度（用完回 RESOURCE_EXHAUSTED／`AGENT_MONTHLY_QUOTA_EXCEEDED`，run 一併回滾）、寫輸入與 `run_accepted` 事件、排入 River job（`MaxAttempts` 1）。額度的上限與期間由呼叫端依買家方案給。
- worker（`run.Dispatcher`）領走 QUEUED（條件式 UPDATE）、呼叫 `Executor`（LLM 與 provider 呼叫在 transaction 外）；執行失敗標 FAILED 並退回額度：job 的 ctx 被關機或 Timeout 取消（不是買家取消）是 `WORKER_LOST`，其餘候選搜尋 `SEARCH_FAILED`、其他 `TURN_PIPELINE_ERROR`；已要求取消的收斂到 CANCELLED。已終結或已被刪除（BuyerDeleted）的 run 再派一次不動作。送出時同一個對話裡等待輸入的舊 run 在同一個 transaction 轉 CANCELLED（不退額度，A9-D23）。
- 事件：序號由 run 列配發，run 是否還活著由 River job 的狀態決定，不看 `updated_at`；狀態轉終態與 `finished` 事件在同一個 transaction，所以看得到終態就讀得到結尾。串流只有一個結尾事件 `finished`（帶狀態，失敗時帶失敗原因；成功、失敗、取消都以它收尾），送出後結束；串流中途 run 被刪除回 NOT_FOUND。每條最長 `Window`（100 秒）。cursor 是事件序號的十進位字串。
- 保留：run 結束 7 天後（`RetentionPeriod`）由每小時的週期 job 刪事件並把 `min_available_seq` 推到 `last_event_seq + 1`。需要的第一筆事件（cursor + 1）小於 `min_available_seq` 回 `REPLAY_CURSOR_EXPIRED`（含從頭開始但事件已清掉）；cursor 大於 `last_event_seq` 回 `REPLAY_CURSOR_AHEAD`。
- 狀態轉換：唯一的守衛函式 `Ledger.Transition`，允許的邊 QUEUED→RUNNING、CANCEL_REQUESTED、FAILED；RUNNING→WAITING_FOR_INPUT、SUCCEEDED、FAILED、CANCEL_REQUESTED；CANCEL_REQUESTED→CANCELLED、FAILED；WAITING_FOR_INPUT→RESUMED、CANCELLED（沒有逾時，interrupt 不設到期）；其餘是終態，終態吸收，之後的事件寫入回 `ErrRunTerminal`。
- job：每個 run 一個 River job（同交易排入，job id 存在 run，`MaxAttempts` 1，Timeout 2 分，River 的 `RescueStuckJobsAfter` 3 分）；ResumeRun 開新的子 run 與新 job。取消＝轉 CANCEL_REQUESTED 並在同交易取消 job，executor 收到 ctx 取消後轉 CANCELLED；River 回報 job 已是 cancelled（還沒被領走）的同交易直接轉 CANCELLED；等待輸入的 run 沒有 job，直接轉 CANCELLED；買家取消不退額度。
- reconciler：每分鐘，只看 QUEUED、RUNNING、CANCEL_REQUESTED 的 run，逐列交易、錯誤記錄後繼續，讀 River job 的狀態：job discarded 或不存在標 FAILED／`WORKER_LOST` 並退額度；job 已完成而 run 未終態（executor 違約）標 FAILED 並退額度；CANCEL_REQUESTED 且 job 已取消、完成、discarded 或不存在轉 CANCELLED（不退額度）；job 還在進行中略過；WAITING_FOR_INPUT 沒有 job，不對帳。每個終態轉換同交易寫 `finished`。沒有時間門檻。
- 共用 listener：一條專用連線 `LISTEN procurement_run`，通知只是提示，事件以資料庫為準；訂閱先於讀取，之後不漏；重連後喚醒全部訂閱者。**每程序上限 500 條訂閱**（`run.MaxSubscriptions`），超過回 RESOURCE_EXHAUSTED。500 是推論值：連線數與訂閱數無關，上限擋的是 goroutine、channel 與每次通知的事件查詢。
- run 類型與額度（A9-D24）：只有 TURN 扣額度；CANDIDATE_SEARCH 與回答中斷的子 run（CLARIFICATION_RESUME）不扣。`agent_runs.quota_period_start` 只在扣額度時有值，所有退款路徑（Dispatcher 失敗、reconciler 的 WORKER_LOST 與違約、`FailAndRefund`）都經 `Ledger.FailAndRefund`，只在它非 NULL 時退。不寫 run 產出（沒有 artifact 表）。
- 每買家上限（A9-D24(6)）：同時進行中的 run 最多 2、每分鐘送出最多 10（`run.Limits`，零值取預設），同一個需求同時只有一個進行中的 run。進行中指 QUEUED、RUNNING、CANCEL_REQUESTED，WAITING_FOR_INPUT 不算（沒有 job、不佔 LLM 與 worker，算進去會讓買家被自己放著的提問擋住新的送出）。檢查在送出的 transaction 內、同一位買家的 advisory lock（`LockBuyerSubmits`）之下，對資料庫計數，所以多 pod 並發也不超過；每分鐘數用 `agent_runs.created_at`（含子 run）而不是記憶體計數，避免重啟或多 pod 時各算各的。買家的進行中已滿或每分鐘已滿回 RESOURCE_EXHAUSTED（帶 retry-after），同需求已有進行中回 FAILED_PRECONDITION／`PROCUREMENT_RUN_IN_PROGRESS`。冪等重送在上限檢查之前回放，不受上限擋。
- 回答中斷（`Submitter.Resume`）：同一個 transaction 鎖父 run（必須是 WAITING_FOR_INPUT 且中斷 PENDING）、建子 run（`parent_run_id`、CLARIFICATION_RESUME、不扣額度、以 `(buyer, client_request_id)` 冪等）、中斷轉 RESOLVED（`resolved_by_child_run_id`）、父 run 轉 RESUMED（寫 `finished`、記 `resumed_into_run_id`）、排 job。中斷已被回答回 `APPROVAL_ALREADY_RESOLVED`，父 run 不在等待輸入回 `RUN_NOT_ACTIVE`，別人的中斷回 NOT_FOUND；防重送靠中斷 ID，不看 revision。子 run 變 FAILED 時（任何路徑，都經 `Ledger.Transition`）同交易把中斷退回 PENDING、父 run 退回 WAITING_FOR_INPUT（`resumed_into_run_id` 清空，不寫事件，父 run 的串流已在 RESUMED 時結尾），買家可再答。這是唯一讓 RESUMED 離開的路徑，不在 `edges` 表內。子 run 被取消目前不重新開放（計畫沒有決定，見 CARRYOVER）。
- 中斷（`Ledger.Interrupt`）：同一個 transaction 寫 `turn_result` 事件（可省）、把 RUNNING 的 run 轉 WAITING_FOR_INPUT、新增 PENDING 的 CLARIFICATION 中斷（payload 是接續的唯一狀態）。查詢只認狀態為 WAITING_FOR_INPUT 的 run 的 PENDING 中斷（`OpenInterruptForRun`、`OpenInterruptForRequest`）；等待中的 run 被取消或被新一輪取代時中斷維持 PENDING（schema 的 CHECK：RESOLVED 一定有 `resolved_by_child_run_id`，沒有子 run 回答就不是 RESOLVED），但查不到。
- 執行者登記：`Dispatcher.Executors` 依 run 類型登記；領到沒有登記執行者的類型，記 log 並依該類型的 pipeline 失敗收斂（TURN 退額度，其餘不退），不留在 RUNNING 等 reconciler。接線：`cmd/hawkr` 的 `addProcurementWorkers`（worker 角色）登記執行 run 的 worker、reconciler 與清除；`startProcurementHub`（serve 角色）啟動共用 LISTEN；兩個角色的 `newProcurement` 都給 job client。
- BuyerDeleted：先刪 `agent_runs WHERE buyer_id`（輸入、事件、interrupt 隨 CASCADE），再把其他表的 `buyer_id` 設 NULL；`agent_runs.buyer_id` 維持 NOT NULL，`publication_id` 為 `ON DELETE SET NULL`。
