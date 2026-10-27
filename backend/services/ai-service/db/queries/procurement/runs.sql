-- name: FindProcurementAgentRunCommandByIdempotencyKey :one
SELECT * FROM procurement_agent_run_commands
WHERE buyer_id = $1 AND client_request_id = $2;

-- name: CreateProcurementAgentRunCommand :one
INSERT INTO procurement_agent_run_commands (
  id, buyer_id, command_kind, client_request_id, payload_hash, run_id, status
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
)
-- 撞冪等鍵不拋錯（PostgreSQL 規範第十二章）。DO UPDATE 把冪等鍵寫回自己：不改內容，
-- 但保證一定 RETURNING 一列，呼叫端用「回傳的 id 是不是這次產的」區分 claim 與既有列。
ON CONFLICT (buyer_id, client_request_id)
DO UPDATE SET client_request_id = EXCLUDED.client_request_id
RETURNING *;

-- name: CreateProcurementAgentRun :one
INSERT INTO procurement_agent_runs (
  id, buyer_id, conversation_id, parent_run_id, run_type,
  domain_ref_type, domain_ref_id, status, last_event_seq, controller_kind
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- ⛔ DEPRECATED（2026-09-20）：呼叫端已移除，procurement_agent_run_inputs
-- 零讀取端，重驅 sweeper 已由 ADR 判定不做。本 query 與該表留給 contract
-- 波一併刪除，期間不得新增呼叫端。
-- name: CreateProcurementAgentRunInput :exec
INSERT INTO procurement_agent_run_inputs (
  run_id, message_id, input_ref, input_schema_version, retention_class
) VALUES (
  $1, $2, $3, $4, $5
);

-- name: GetProcurementAgentRunForUpdate :one
-- row lock：seq 分配與 status transition 都要先鎖這一列，理由同
-- AppendRevision 對 procurement_requests 做 FOR UPDATE 的註解——序列化同一個
-- run 上的併發寫入，避免兩個 event 算出同一個 seq。
SELECT * FROM procurement_agent_runs
WHERE id = $1 AND buyer_id = $2
FOR UPDATE;

-- name: GetProcurementAgentRun :one
SELECT * FROM procurement_agent_runs
WHERE id = $1 AND buyer_id = $2;

-- name: AppendProcurementAgentRunEvent :one
INSERT INTO procurement_agent_run_events (
  id, run_id, seq, event_type, schema_version, public_payload_json, occurred_at, trace_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: BumpProcurementAgentRunLastEventSeq :exec
UPDATE procurement_agent_runs SET last_event_seq = $2, updated_at = now() WHERE id = $1;

-- name: TerminalizeProcurementAgentRunStatus :execrows
-- terminalize 專用的終態轉移：守衛只有 `id` ＋「現在還不是終態」，**刻意不比對
-- version**（2026-09-07 決策：terminalize 不對 version 做 CAS，改用狀態機 guard）。
--
-- 為什麼不共用下面那支：worker 宣告自己的執行結果**不是** read-modify-write——它
-- 沒讀過快照、也不基於快照改值，它在陳述一件已經發生的事實。optimistic CAS on
-- version 是給 cancel／resume 這種「拿著一份快照要改它」的呼叫端用的。兩者共用同
-- 一支帶 version 述詞的 query 會讓 cancel（或 retry）bump 過版本之後，終態整個寫
-- 不進去、而且無聲：TerminalizeRun 回 wrote=false 不是錯誤，呼叫端只記一行 Warn，
-- 該 run 於是**永遠沒有終態 ledger event**，串流消費端等不到收線訊號，連線開到
-- gateway 的 120 秒 deadline。
--
-- 真正要防的是「不要覆寫一個已經終態的 run」，而那個守衛本來就在 WHERE 裡、不依賴
-- version。started_at 不在這裡處理：terminalize 不會把 run 轉成 RUNNING。
UPDATE procurement_agent_runs
SET status = sqlc.arg(status),
    failure_reason = sqlc.arg(failure_reason),
    result_revision_id = COALESCE(sqlc.narg(result_revision_id), result_revision_id),
    completed_at = now(),
    version = version + 1, updated_at = now()
WHERE id = sqlc.arg(id)
  AND status NOT IN ('SUCCEEDED','FAILED','CANCELLED','RESUMED','EXPIRED')
  AND sqlc.arg(status) IN ('SUCCEEDED','FAILED','CANCELLED','RESUMED','EXPIRED');

-- name: UpdateProcurementAgentRunStatus :execrows
-- version 是 v1 唯一的 CAS 欄位（doc57 §6.2 v1 不含 lease/fencing）。呼叫端帶
-- 目前讀到的 version，UPDATE 命中才代表沒有被別的呼叫搶先轉移狀態。
--
-- result_revision_id 用 COALESCE 保留原值：這支 query 服務三種轉移
-- （terminalize／cancel／resume），只有 terminalize 帶得出真正的結果
-- revision，另外兩種不該碰它。傳 NULL 代表「這次轉移不改動
-- result_revision_id」，**不是**「把它清成 0」——後者會抹掉
-- replayExistingRun 的就緒判準（answer.go/turn.go 的 `run.ResultRevisionID
-- == 0` 判斷），也會讓 ledger 指向一個不存在的 revision id（Task 14b C1）。
UPDATE procurement_agent_runs
SET status = sqlc.arg(status),
    failure_reason = sqlc.arg(failure_reason),
    result_revision_id = COALESCE(sqlc.narg(result_revision_id), result_revision_id),
    started_at = COALESCE(started_at, CASE WHEN sqlc.arg(status) = 'RUNNING' THEN now() END),
    completed_at = CASE WHEN sqlc.arg(status) IN ('SUCCEEDED','FAILED','CANCELLED','RESUMED','EXPIRED')
                        THEN now() ELSE completed_at END,
    version = version + 1, updated_at = now()
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
  AND status NOT IN ('SUCCEEDED','FAILED','CANCELLED','RESUMED','EXPIRED');

-- name: ListProcurementAgentRunEventsAfterSeq :many
SELECT * FROM procurement_agent_run_events
WHERE run_id = $1 AND seq > $2
ORDER BY seq ASC;

-- name: SetProcurementAgentRunResumedInto :exec
UPDATE procurement_agent_runs
SET resumed_into_run_id = $2, updated_at = now()
WHERE id = $1;

-- name: CreateProcurementAgentRunInterrupt :one
-- payload_json 是這個中斷要問買家的內容，形狀由 (interrupt_kind,
-- response_schema_version) 決定。它跟中斷本體同一次 INSERT——待答狀態住
-- procurement_requests 的時代，「候選落庫」與「建立中斷」是兩次各自 log-and-swallow
-- 的寫入，因此有兩個不對稱的中間狀態（有卡片沒中斷／有中斷沒白名單）。合成一次
-- INSERT 之後這兩個狀態結構上不存在。
INSERT INTO procurement_agent_run_interrupts (
  id, run_id, interrupt_kind, response_schema_version, status, payload_json, facts_as_of, expires_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetProcurementAgentRunInterruptForUpdate :one
SELECT * FROM procurement_agent_run_interrupts
WHERE id = $1
FOR UPDATE;

-- name: ResolveProcurementAgentRunInterrupt :execrows
UPDATE procurement_agent_run_interrupts
SET status = $2, resolved_by_child_run_id = $3, resolved_at = now()
WHERE id = $1 AND resolved_by_child_run_id IS NULL;

-- name: FindOpenProcurementInterruptForRequest :one
-- 該 request 目前是否被一個「已 park 且尚未回覆」的 run 佔用（Task 14b B3）。
-- run → request 的連結只有 result_revision_id 這一條（domain_ref_type／
-- domain_ref_id 在 TURN/ANSWER/REWIND 路徑上從未被填），所以必須經由
-- revision 表 join。
--
-- ⚠️ Task 15 起 conversation_id 已在 ClaimRun／CreateLedgerRunForCandidateSearch
-- 補上，但這支既有查詢維持原本經由 revision 表 join 的寫法不變：它同時要驗證
-- 「run 目前仍是 WAITING_FOR_INPUT」，這個時效性判準跟 conversation_id 是兩件
-- 事，join result_revision_id 一次拿到兩者，沒有理由拆成兩支查詢。
SELECT i.* FROM procurement_agent_run_interrupts i
JOIN procurement_agent_runs r          ON r.id  = i.run_id
JOIN procurement_request_revisions rev ON rev.id = r.result_revision_id
WHERE rev.request_id = sqlc.arg(request_id)
  AND r.buyer_id     = sqlc.arg(buyer_id)
  AND r.status       = 'WAITING_FOR_INPUT'
  AND i.status        = 'PENDING'
  AND i.resolved_by_child_run_id IS NULL
ORDER BY i.created_at DESC
LIMIT 1;

-- name: SetProcurementAgentRunConversationID :exec
-- 回填用（Task 15 write-path 修復）：ClaimRun 對「這是這個買家第一次開新對話」
-- 的 TURN run（claim 當下 request 尚未存在）無法在 claim transaction 內解出
-- conversation_id——conversation row 要等 executeTurnPipeline 之後呼叫
-- CreateRequest 才誕生。`conversation_id IS NULL` guard 讓這支查詢對「claim
-- 時就已經解出來」的 run（繼續既有對話的 TURN／ANSWER／REWIND）是天然的
-- no-op，不會覆寫一個已經正確的值。
UPDATE procurement_agent_runs
SET conversation_id = $2, updated_at = now()
WHERE id = $1 AND conversation_id IS NULL;

-- name: ListProcurementAgentRunsByConversation :many
-- runtime-snapshot 端點擴充（Task 15，doc57 §5.6）：這個 conversation 底下
-- 全部的 run，最新的排最前面——GetSnapshot handler 用它組 runs[] 並掃出目前
-- 的 active run（v1 一個 conversation 同時只有一個 active run，doc57 §5.6）。
SELECT * FROM procurement_agent_runs
WHERE conversation_id = sqlc.arg(conversation_id) AND buyer_id = sqlc.arg(buyer_id)
ORDER BY created_at DESC;

-- name: GetOpenProcurementAgentRunInterruptByRunID :one
-- runtime-snapshot 端點擴充（Task 15）：GetSnapshot handler 只在 active run
-- 真的是 WAITING_FOR_INPUT 時才呼叫這支，查出它對應的未解 interrupt 組
-- openConfirmation。`i.status = 'PENDING'` 這個條件跟上面
-- FindOpenProcurementInterruptForRequest 的既有「open interrupt」定義（Task
-- 14b B3）保持一致，不只看 resolved_by_child_run_id IS NULL——兩個判準理論上
-- 同時成立（CreateInterrupt 只在 status='PENDING' 時建立列，且 status 只有
-- ResolveProcurementAgentRunInterrupt 一個寫入點會改），這裡是防禦深度，同
-- FindOpenProcurementInterruptForRequest 文件記載的既有慣例。
--
-- r.buyer_id 這個 join 條件是 review round 2 Minor #4 補上的租戶邊界：
-- procurement_agent_run_interrupts 本身沒有 buyer_id 欄位，修復前這支查詢只
-- 靠呼叫端「傳進來的 runID 已經是從一次買家自己的 ListRunsByConversation 撈
-- 出來的」這個沒有被強制執行、也沒有文件記載的前提維持安全——跟既有姊妹查詢
-- FindOpenProcurementInterruptForRequest（同一個 buyer_id 邊界寫法）補齊一致，
-- defense-in-depth。
SELECT i.* FROM procurement_agent_run_interrupts i
JOIN procurement_agent_runs r ON r.id = i.run_id
WHERE i.run_id = sqlc.arg(run_id) AND r.buyer_id = sqlc.arg(buyer_id)
  AND i.status = 'PENDING' AND i.resolved_by_child_run_id IS NULL
ORDER BY i.created_at DESC
LIMIT 1;

-- name: FindLatestProcurementAgentRunArtifact :one
-- 「同一個 run 的同一型 artifact」是 artifact 的身分鍵——與
-- idx_procurement_agent_run_artifacts_run (run_id, artifact_type) 對齊。artifact_id
-- 在版本之間穩定（doc 57 §4.5：version 是「這個 artifact_id 的第幾次產製」，不是
-- data schema 版本），所以重新產製要先在這裡找回既有身分，不是每次都鑄一個新的。
SELECT * FROM procurement_agent_run_artifacts
WHERE run_id = $1 AND artifact_type = $2
ORDER BY version DESC
LIMIT 1;

-- name: InsertProcurementAgentRunArtifact :one
-- 每次產製寫一列新版本，不 UPDATE 既有列：artifact 是不可變快照，舊版本要留著給
-- replay 與 owner restore 讀（PRIMARY KEY (artifact_id, version) 是最終防線，
-- 版本配號在 run row lock 內做，同 AppendProcurementAgentRunEvent 的 seq 配號）。
INSERT INTO procurement_agent_run_artifacts (
  artifact_id, run_id, artifact_type, version, status,
  public_data_json, checksum, domain_ref_type, domain_ref_id, as_of
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: RestoreProcurementAgentRunInterruptToPending :execrows
UPDATE procurement_agent_run_interrupts
SET status = 'PENDING', resolved_by_child_run_id = NULL, resolved_at = NULL
WHERE id = sqlc.arg(id) AND resolved_by_child_run_id = sqlc.arg(child_run_id);

-- name: RestoreProcurementAgentRunToWaitingForInput :execrows
UPDATE procurement_agent_runs
SET status = 'WAITING_FOR_INPUT',
    resumed_into_run_id = NULL,
    completed_at = NULL,
    version = version + 1,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND buyer_id = sqlc.arg(buyer_id)
  AND status = 'RESUMED'
  AND resumed_into_run_id = sqlc.arg(child_run_id);

-- name: ListLatestProcurementAgentRunArtifactsByConversation :many
-- 去重鍵是 artifact_id 不是 (run_id, artifact_type)：同一型 artifact 在不同 run
-- 各自鑄一個 artifact_id，以型別去重會讓前一個 run 的 artifact 從 snapshot 消失。
SELECT a.* FROM procurement_agent_run_artifacts a
JOIN procurement_agent_runs r ON r.id = a.run_id
WHERE r.conversation_id = sqlc.arg(conversation_id)
  AND r.buyer_id = sqlc.arg(buyer_id)
  AND a.version = (
    SELECT MAX(a2.version) FROM procurement_agent_run_artifacts a2
    WHERE a2.artifact_id = a.artifact_id
  )
ORDER BY a.created_at DESC, a.artifact_id DESC;
