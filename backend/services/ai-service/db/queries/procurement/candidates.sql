-- name: FindProcurementCandidateSet :one
SELECT * FROM procurement_candidate_sets
WHERE request_revision_id = $1 AND policy_version = $2;

-- name: CreateProcurementCandidateSet :one
-- fencing_token 顯式帶入（呼叫端固定傳 1），不依賴 schema 的 DEFAULT 0——
-- 見 repository/candidate.go CreateCandidateSet 的呼叫處註解。
-- run_id 與 id 各配一次 TSID，刻意不同值（設計書 57 §1.2-3）。
INSERT INTO procurement_candidate_sets (
  id, run_id, request_id, request_revision_id, buyer_id, policy_version,
  lease_owner, lease_expires_at, fencing_token
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (request_revision_id, policy_version)
DO UPDATE SET policy_version = EXCLUDED.policy_version
RETURNING *;

-- name: GetProcurementCandidateSet :one
SELECT * FROM procurement_candidate_sets
WHERE id = $1 AND request_id = $2 AND buyer_id = $3;

-- name: GetProcurementCandidateSetByRunID :one
-- 對外 GET .../candidate-searches/{runId} 的解析查詢。ownership 條件與
-- GetProcurementCandidateSet 逐字相同——換一把查詢鍵不是放寬 ownership 的理由。
--
-- 這一句只認得**當前**那次嘗試：run_id 欄位在每次 RetryProcurementCandidateSet 都會換
-- 新值，被取代的舊 run 身分在這張表上查不到，改由
-- GetProcurementCandidateSetBySupersededRunID 解析。
SELECT * FROM procurement_candidate_sets
WHERE run_id = $1 AND request_id = $2 AND buyer_id = $3;

-- name: GetProcurementCandidateSetBySupersededRunID :one
-- 用一個**被取代的**（前一次嘗試的）run 身分反查它所屬的 candidate set。
--
-- 重試改成「一次嘗試配一個新 run」之後，procurement_candidate_sets.run_id 只留最新一次
-- 嘗試，舊值在那張表上永遠 miss。少了這一句，一個在重試發生前就開始輪詢的 browser 會從
-- 某一刻起固定收到 404，而它手上那個舊值不會隨部署恢復——問題出在 client 記著的值，不是
-- 服務端的程式版本（同 grpc/procurement_handler.go GetCandidateSearch echo-back 註解描述
-- 的那一族靜默失效）。
--
-- 解析方向是 1:N 的多對一那一邊：每一列 agent_runs 都用 domain_ref 記著自己屬於哪一批
-- 候選（CreateLedgerRunForCandidateSearch 寫入，doc57 §1.2-3）。domain_ref_id 是 TEXT，
-- 兩邊都轉成 TEXT 比對，不把 candidate set id 反向 parse 成整數——那會讓一列髒資料
-- （非數字的 domain_ref_id）在查詢層直接爆 22P02，而不是安靜地不匹配。
--
-- ownership 條件與另外兩支解析查詢逐字相同：換一把查詢鍵不是放寬 ownership 的理由。
-- run_type 一併框住 CANDIDATE_SEARCH——TURN／CLARIFICATION_RESUME 的 run 不寫
-- candidateSetId 這個 domain_ref_type，但把它寫出來，這句查詢自己就說得完整。
SELECT cs.* FROM procurement_candidate_sets cs
JOIN procurement_agent_runs r
  ON r.domain_ref_type = 'candidateSetId' AND r.domain_ref_id = cs.id::text
WHERE r.id = $1 AND r.run_type = 'CANDIDATE_SEARCH'
  AND cs.request_id = $2 AND cs.buyer_id = $3;

-- name: GetProcurementCandidateSetByID :one
SELECT * FROM procurement_candidate_sets
WHERE id = $1;

-- name: ReapExpiredProcurementCandidateSet :one
-- fencing_token 一併遞增：只把 status 標成 FAILED 不足以擋住 zombie worker——它手上
-- 拿的 token（例如首次認領的 1）在 reap 前後沒有變化的話，之後仍然能通過
-- MarkProcurementCandidateSetRunning／FinalizeProcurementCandidateSet 的
-- `fencing_token = $N` 檢查（那兩句原本就沒檢查 status='FAILED' 以外的什麼），
-- 讓 fencing_token 形同虛設。遞增後，zombie worker 手上的舊 token 保證從此再也對不上。
--
-- `lease_expires_at < now()` 必須寫在 SQL 裡，不能只靠 Go 端（reapCandidateSet 呼叫端讀完
-- row 之後在記憶體裡比對 LeaseExpiresAt）——那是一個 TOCTOU：讀取與這句 UPDATE 不在同一
-- 個交易，中間 row 可能已經被 RetryProcurementCandidateSet 重新配發一個有效的 lease。
-- 真實可達的交錯：兩個併發的 GET 輪詢都讀到同一列過期的 lease → 讀者 A 的 reap 先 commit
-- 把它標成 FAILED → 一次 POST candidate-searches 重試把同一列救回 QUEUED（新 lease、
-- fencing_token 再 +1、新 worker 已派出）→ 讀者 B 這時才進來 reap，只比對
-- `status IN ('QUEUED','RUNNING')` 的話會直接殺掉這個 lease 還有 30 秒的活 run。
-- 把過期條件放進 WHERE，等於在交易內對當下的 DB 狀態重新驗證一次，而不是相信呼叫端
-- 那份可能已經過時的快照。lease_expires_at 為 NULL 時比較結果是 NULL、不匹配——與 Go 端
-- 「LeaseExpiresAt == nil 就不 reap」的判斷一致。
--
-- RETURNING *（而非 :execrows）讓呼叫端拿到**交易內**那一列的真實內容，不用在 Go 端手抄
-- 一份 SET 子句的鏡像（status／failure_reason／fencing_token 原本各複製一次，任何一次
-- SQL 改動都要記得同步）。更重要的是 run_id：reap 現在要順帶終態化這一列**當前掛著的那次
-- 嘗試**，而呼叫端手上的快照是開交易之前讀的。同一列在那之間可能已經被重試換過 run_id
-- （新 lease 讓 WHERE 不匹配、於是這句影響 0 列——但若那次新嘗試的 lease 也已經過期就會
-- 匹配），拿舊快照的 run_id 去終態化會寫錯 attempt。0 列時 sqlc 回 pgx.ErrNoRows。
UPDATE procurement_candidate_sets
SET status = 'FAILED', failure_reason = 'WORKER_LOST',
    fencing_token = fencing_token + 1, updated_at = now()
WHERE id = $1 AND status IN ('QUEUED', 'RUNNING') AND lease_expires_at < now()
RETURNING *;

-- name: SetProcurementRequestStatusIfActiveRevision :execrows
-- Reaper 專用的保守寫入：只有在這個 candidate_set 所屬的 revision 仍然是 request 目前
-- 的 active_revision_id 時，才把狀態退回 REQUIREMENT_READY。買家可能已經在這個 run
-- 過期之前就送出了下一輪 turn（active_revision_id 已經前進），這時候一個遲來的 reap
-- 不該用舊 run 的失敗結果蓋掉 request 更新的狀態——見 repository/candidate.go
-- GetCandidateSetWithCandidates 的呼叫處註解。
UPDATE procurement_requests
SET status = $2, updated_at = now()
WHERE id = $1 AND active_revision_id = $3;

-- name: ListProcurementCandidatesBySet :many
SELECT * FROM procurement_candidates
WHERE candidate_set_id = $1
ORDER BY lane, rank;

-- name: MarkProcurementCandidateSetRunning :execrows
UPDATE procurement_candidate_sets
SET status = 'RUNNING', updated_at = now()
WHERE id = $1 AND fencing_token = $2 AND status = 'QUEUED';

-- name: RetryProcurementCandidateSet :one
-- 把一個已終止的 FAILED run 轉回 QUEUED 並遞增 fencing_token，讓 StartSearch 對「reap 過的
-- 死 run」可以重新派工，而不是照 idempotency key 永遠回傳同一筆死掉的紀錄——見計畫
-- Design Decision #7「買家重新呼叫即可拿到新的 run」。guard 在 status='FAILED'：對成功終態
-- SUCCEEDED 一律不動作；併發下輸掉 race 的呼叫端會拿到 pgx.ErrNoRows（0 rows matched），
-- 由 caller 判斷退回舊行為。
--
-- platform_lane_status／external_lane_status 一併歸零回 'PENDING'：走 reap 進 FAILED 的
-- run 這兩欄還停在 'PENDING'，但「兩條 lane 都真的失敗」而 FinalizeProcurementCandidateSet
-- 判定 FAILED 的 run，這兩欄是 'FAILED'。少了這一句，重試之後的整段新嘗試期間，輪詢的
-- client 會讀到 {"status":"RUNNING","platformLaneStatus":"FAILED","externalLaneStatus":"FAILED"}
-- ——上一次嘗試的失敗資訊，被當成這一次的進度顯示，直到新的 finalize 才被蓋掉。
-- 重試＝重新開始一次 run，run 層級的欄位（status／failure_reason／lease／fencing_token）既然
-- 全部重置，lane 層級的也必須一起重置，否則兩者會描述不同的兩次嘗試。
--
-- run_id 換新值（$4）：一次重試是一次**新的執行**，不是同一個 run 重新開機。沿用舊 run_id
-- 的話，那個 ledger run 在第一次嘗試就已經被寫成終態，第二次嘗試的終態會被
-- TerminalizeProcurementAgentRunStatus 的「現在還不是終態」守衛擋掉——**第二次執行永遠沒有
-- 終態 ledger event**，串流端沒有收線訊號，連線開到 gateway 的 deadline 為止。
-- candidate_set 仍是同一列（冪等鍵 (request_revision_id, policy_version) 本來就擋掉第二列），
-- 所以 candidate_sets : agent_runs 由 1:1 變成 1:N，本欄是**當前**嘗試的 run 身分。
UPDATE procurement_candidate_sets
SET status = 'QUEUED', failure_reason = '', lease_owner = $2, lease_expires_at = $3,
    run_id = $4,
    platform_lane_status = 'PENDING', external_lane_status = 'PENDING',
    fencing_token = fencing_token + 1, updated_at = now()
WHERE id = $1 AND status = 'FAILED'
RETURNING *;

-- name: CreateProcurementCandidate :one
-- store_id 以下四欄是 #2377 的賣家身分快照，EXTERNAL lane 一律留空（store_id 為 NULL，
-- 三個字串欄為 ''）。store_id **不是授權判準**，理由見 schema/procurement.sql 的
-- procurement_candidates 註解。
-- prospect_id 與 store_id 互斥：EXTERNAL lane 恆有值、PLATFORM lane 恆為 NULL。
-- brand_* 兩欄相反：EXTERNAL lane 的品牌素材快照，PLATFORM lane 一律 ''。它們與
-- 上面的 logo_key 不是同一件事——那一欄是平台商家的 logo，站外業者填進去等於
-- 偽造平台身分（見 ai.proto 14–15）。
INSERT INTO procurement_candidates (
  id, candidate_set_id, lane, source_id, display_name, score,
  matched_facts_json, unverifiable_facts_json, facts_as_of, rank,
  store_id, seller_name, logo_key, store_code, prospect_id,
  brand_logo_key, brand_cover_key
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
  $11, $12, $13, $14, $15, $16, $17
)
RETURNING *;

-- name: FinalizeProcurementCandidateSet :execrows
-- failure_reason 是本次新增的 SET 欄位。在此之前「兩條 lane 都失敗」這條路徑只寫
-- status='FAILED'，reason 留在空字串——於是 CodeMeta 登記的 LANE_ALL_FAILED 是一個沒有任何
-- 寫入路徑的死值（已生成進 Go/Java/Dart/TS 四份輸出），而契約明寫「FAILED 時 reason 必非空」。
-- 實際上只有 reap 的 WORKER_LOST 會寫。呼叫端負責在 status='FAILED' 時傳 LANE_ALL_FAILED。
--
-- ⚠️ 終態守衛是 finalize 的冪等防護（不得覆寫已終態的 run）。清單少列任何一個終態值，
-- 那些剛寫完終態的 run 會被判定成「還沒終態」，一次遲到或重送的 finalize 就能覆寫它，
-- 候選資料被寫入第二次——那正是這道守衛要擋的真實 bug（見 repository/candidate.go
-- 對應呼叫處的註解）。舊值 COMPLETED／PARTIAL 已由 #2446 Wave C 的 backfill migration 收斂掉，
-- 且沒有任何寫入端還會產生它們，故清單收斂為 ('SUCCEEDED','FAILED')。
UPDATE procurement_candidate_sets
SET status = sqlc.arg(status),
    platform_lane_status = sqlc.arg(platform_lane_status),
    external_lane_status = sqlc.arg(external_lane_status),
    failure_reason = sqlc.arg(failure_reason),
    updated_at = now()
WHERE id = sqlc.arg(id) AND fencing_token = sqlc.arg(fencing_token)
  AND status NOT IN ('SUCCEEDED', 'FAILED');

-- name: UpsertProcurementCandidateSelection :one
INSERT INTO procurement_candidate_selections (
  id, request_id, request_revision_id, buyer_id, candidate_set_id, lane, candidate_id, selected
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (request_revision_id, lane, candidate_id) DO UPDATE
SET selected = EXCLUDED.selected, updated_at = now()
RETURNING *;

-- name: GetProcurementCandidate :one
SELECT * FROM procurement_candidates
WHERE id = $1 AND candidate_set_id = $2;

-- name: ListSelectedProcurementCandidateIDs :many
-- 只回傳目前 selected=true 的 candidate_id——UpsertProcurementCandidateSelection 是
-- upsert，取消勾選會把既有列的 selected 改成 false 而不是刪除，所以「這個 candidate_set
-- 下有列」不等於「目前有效勾選」，一定要帶 selected=true 這個條件。
SELECT candidate_id FROM procurement_candidate_selections
WHERE candidate_set_id = $1 AND selected = true;
