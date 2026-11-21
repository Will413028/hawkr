# procurement 模組行為規格

本文是單體 `procurement` 模組行為的唯一來源。目前有資料層與 run 帳本（第 2 節）：買家採購（RFQ）與賣家報價；採購的另一半（賣家回報價）也在這裡。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：ordering、catalog、buyer、seller；不依賴 notification（notification 依賴 procurement，方向見 `internal/arch/modules.go`）。
- 公開介面與行為隨 A9-2 到 A9-4 的實作補上。

## 2. run 帳本與串流（A9-3b）

- 送出（`run.Submitter`）在同一個 transaction：以 `(buyer_id, client_request_id)` 冪等建 run（重送回第一次的 run，內容不同回 INVALID_ARGUMENT，不再扣額度）、`ConsumeUsage` 扣一次額度（用完回 RESOURCE_EXHAUSTED／`AGENT_MONTHLY_QUOTA_EXCEEDED`，run 一併回滾）、寫輸入與 `run_accepted` 事件、排入 River job（`MaxAttempts` 1）。額度的上限與期間由呼叫端依買家方案給。
- worker（`run.Dispatcher`）領走 QUEUED（條件式 UPDATE）、呼叫 `Executor`（LLM 與 provider 呼叫在 transaction 外）；執行失敗標 FAILED（候選搜尋 `SEARCH_FAILED`，其餘 `TURN_PIPELINE_ERROR`）並退回額度。已終結的 run 再派一次不動作。
- 事件：序號由 run 列配發，`updated_at` 同時是執行中 run 的心跳；狀態轉終態與 `finished` 事件在同一個 transaction，所以看得到終態就讀得到結尾。串流在送出 `finished` 或 `error` 之後結束，每條最長 `Window`（100 秒）。cursor 是事件序號的十進位字串。
- 保留：run 結束 7 天後（`RetentionPeriod`）由每小時的週期 job 刪事件並把 `min_available_seq` 推到 `last_event_seq + 1`。需要的第一筆事件（cursor + 1）小於 `min_available_seq` 回 `REPLAY_CURSOR_EXPIRED`（含從頭開始但事件已清掉）；cursor 大於 `last_event_seq` 回 `REPLAY_CURSOR_AHEAD`。
- 狀態轉換：唯一的守衛函式 `Ledger.Transition`，允許的邊 QUEUED→RUNNING、CANCEL_REQUESTED、FAILED；RUNNING→WAITING_FOR_INPUT、SUCCEEDED、FAILED、CANCEL_REQUESTED；CANCEL_REQUESTED→CANCELLED、FAILED；WAITING_FOR_INPUT→RESUMED、EXPIRED；其餘是終態，終態吸收，之後的事件寫入回 `ErrRunTerminal`。
- job：每個 run 一個 River job（同交易排入，job id 存在 run，`MaxAttempts` 1，Timeout 2 分，River 的 `RescueStuckJobsAfter` 3 分）；ResumeRun 開新的子 run 與新 job。取消＝轉 CANCEL_REQUESTED 並在同交易取消 job，executor 收到 ctx 取消後轉 CANCELLED；買家取消不退額度。
- reconciler：每分鐘，只看 QUEUED、RUNNING、CANCEL_REQUESTED 的 run，逐列交易、錯誤記錄後繼續，讀 River job 的狀態：job discarded 或不存在標 FAILED／`WORKER_LOST` 並退額度；job 已完成而 run 未終態（executor 違約）標 FAILED 並退額度；CANCEL_REQUESTED 且 job 已取消或完成轉 CANCELLED（不退額度）；job 還在進行中略過；WAITING_FOR_INPUT 沒有 job，不對帳。每個終態轉換同交易寫 `finished`。沒有時間門檻。
- 共用 listener：一條專用連線 `LISTEN procurement_run`，通知只是提示，事件以資料庫為準；訂閱先於讀取，之後不漏；重連後喚醒全部訂閱者。**每程序上限 500 條訂閱**（`run.MaxSubscriptions`），超過回 RESOURCE_EXHAUSTED。500 是推論值：連線數與訂閱數無關，上限擋的是 goroutine、channel 與每次通知的事件查詢。
- artifact：每個 `(run_id, artifact_type)` 一列就地更新，`version` 只當 `ARTIFACT_UPSERTED` 的去重序號。interrupt 不設到期，下一輪送出即取代。
- BuyerDeleted：先刪 `agent_runs WHERE buyer_id`（輸入、事件、產出、interrupt 隨 CASCADE），再把其他表的 `buyer_id` 設 NULL；`agent_runs.buyer_id` 維持 NOT NULL，`publication_id` 為 `ON DELETE SET NULL`。
