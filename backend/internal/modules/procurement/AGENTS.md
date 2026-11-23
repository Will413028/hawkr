# procurement 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與已知限制寫在 `backend/docs/modules/procurement.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

買家採購（RFQ）與賣家報價，服務買家與賣家。事件 `procurement.*` 由 notification 訂閱。來源是 ai-service，搬遷計畫 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）；依賴：ordering、catalog、buyer、seller，不依賴 notification（見 `internal/arch/modules.go`）。行為規格的唯一來源是 `backend/docs/modules/procurement.md`。

## 擁有的資料

schema `procurement`（`db/schema.sql`），登記在 `cmd/hawkr/modules.go` 的 `modules()`。23 張業務表：

- 需求：`requests`、`request_revisions`、`conversations`、`turns`。
- 候選：`candidate_sets`、`candidates`。
- 詢價：`price_previews`、`confirmed_rfq_snapshots`、`rfq_publication_approvals` 與 `_recipients`、`rfq_publications`、`_invites`。
- 賣家回價與接受：`seller_responses`、`seller_response_versions`、`_drafts`、`buyer_shortlist_entries`、`buyer_quote_acceptances`。
- run：`agent_runs`、`_inputs`、`_events`、`_interrupts`。額度：`usage`。

無 outbox、`*_commands`、預約表、`controller_kind`、`candidate_leads`、lane 與 prospect。讀取模型：`snapshot`。

## 發布的讀取介面

無。<!-- 每個 `pub_*` 一行：形狀與用途（契約在 `db/pub.sql`）。 -->

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名、事件與錯誤。
- `internal/model` 放對外型別；`internal/store` 是 sqlc 查詢套件（repository 一律 `store.New(db.Conn(ctx))`）；`internal/resolver` 把用詞對到分類樹；`internal/candidate` 決定誰能成為候選、翻頁收集；`internal/retention` 處理買家關閉；`internal/run` 是 run 帳本（送出與配額、守衛的狀態轉換與事件、取消、對帳、保留期清除、共用 LISTEN）；`slots`（欄位驗證）、`planner`（問題與 act 優先序）、`reply` 加 `text`（代碼換語系文字，無 LLM 敘述）、`extract`、`turn`（TURN、回答、`ResumeRun`、不經 run 的回答與倒回，見 `internal/turn/AGENTS.md`）、`search`（CANDIDATE_SEARCH executor 與 `StartCandidateSearch`，§4）、`snapshot`（`GetRuntimeSnapshot`）。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加子領域時同步修改。各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。

## 不變量

- 模組只 import 依賴表允許的模組根套件：`internal/arch` 的依賴方向測試守門。
- 候選沒有店名快照，只收 OPEN 的店，名稱讀取時由 StoreCard 組裝（S9：`candidate.KeepOpen`）。
- 買家回應的候選型別 `model.Candidate` 沒有自由文字（A9-D8、AD7：`TestCandidateCarriesNoFreeText`）。
- 買家的用詞不被同義詞覆寫：原詞有命中分類就不查同義詞（AD3）；人工同義詞在 `internal/resolver/synonyms.json`。
- 「現行 revision」只有 `request_revisions_active` 一種表示法；requests 與 acceptances 的轉移用條件式 UPDATE，不設 version。
- 接受單的 live 鎖只擋 `state = 'ACTIVE'`；建單重試用盡轉 `ORDER_FAILED` 解除鎖。
- 建立型方法的冪等鍵放在被建立的實體上；rename、delete、withdraw、decline 是目標狀態的方法，不建收據（S4，逐支判定見 CARRYOVER）。
- `BuyerDeleted` 同 transaction：買家資料刪除，已發布的需求與賣家看得到的紀錄保留、`buyer_id` 設 NULL。
- 頭尾不得有空白的文字欄登記在 `cmd/hawkr` 的 `trimmedColumns`；新增文字欄一併登記。
- run 事件在 run 結束 7 天後由週期 job 清除並推進 `min_available_seq`；cursor 過舊回 `REPLAY_CURSOR_EXPIRED`，超前回 `REPLAY_CURSOR_AHEAD`。interrupt 不到期；只有 TURN 扣額度。
- run 狀態只經 `Ledger.Transition`（條件式 UPDATE，邊表在 `run/transition.go`，終態吸收），事件只在非終態寫入，終態後回 `run.ErrRunTerminal`，executor 收到就停止。
- 每個 run 一個 River job（建 run 的同交易排入，id 存 `agent_runs.job_id`，`MaxAttempts` 1、Timeout 2 分；`RescueStuckJobsAfter` 3 分），不重驅。
- job 死了的 run 由 `run.Reconciler`（每分鐘，逐列交易）標失敗：job discarded 或不存在標 `WORKER_LOST` 並退額度，job 已完成而 run 未終態標 executor 的失敗原因並退額度，CANCEL_REQUESTED 且 job 已死收斂到 CANCELLED（不退額度）；關機或逾時取消 job 標 `WORKER_LOST` 並退額度；等待輸入的 run 可直接取消（不退額度、中斷轉 CANCELLED），同對話新一輪自動取消它（A9-D23）；買家取消不退額度（A9-D6）。`WatchRun` 共用一條 LISTEN 連線。
- 中斷回答成功才解決（A9-D26、A9-D30）：回答 run 成功或寫新中斷的同交易，中斷 RESOLVED、提問的 run SUCCEEDED；失敗（答案被拒是 `ANSWER_REJECTED`）或取消不還原；每需求最多一筆 PENDING；回答 run 的鎖順序是提問的 run、中斷、自己（`Ledger.LockAskingOf`）。
- 配額是一列計數器加條件 UPDATE（`ConsumeUsage`），沒有預約表（A9-D9）。
- `turns.turn_result` 存第一次的完整回應：不經 run 的 SubmitAnswer、Rewind 重送沒有事件可回放，重算會與第一次不同（A9-D33(2)）。

## 不負責

買家助理（`assistant`）、AI 修圖（`imageai`）、站外業者入口（A9-D1 已刪，不重做）。

## 決策紀錄

從 ai-service 沿用的每個機制、當時的決定與理由，以及與 ai-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。<!-- 相關 ADR 列在這一行後面。 -->
