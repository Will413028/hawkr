package domain

import (
	"encoding/json"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// runFailedPayload 是 RUN_FAILED ledger event 的 public payload 形狀。
type runFailedPayload struct {
	FailureReason string `json:"failureReason"`
}

// RunFailedPayload 組 RUN_FAILED ledger event 的 public payload。
//
// 這一型的 payload **必須自描述**：gateway 靠它把事件投影成原生 AG-UI RUN_ERROR
// 的 `reason`（doc57 §4.2 Run 那一列的 carrier 定義）。舊版四個 producer 各自寫
// 字面 `{}`，於是失敗原因只存在 procurement_agent_runs.failure_reason 這個
// **不在 wire 上**的欄位裡——消費端只知道「失敗了」，不知道為什麼。
//
// reason 為空時回字面 `{}`：不憑空補值，CodeMeta 是這個值域的 SSOT。
func RunFailedPayload(reason string) []byte {
	if reason == "" {
		return []byte(`{}`)
	}
	b, err := json.Marshal(runFailedPayload{FailureReason: reason})
	if err != nil {
		return []byte(`{}`)
	}
	return b
}

// RunType 是 doc57 identity contract 的 run 種類值域（開放值域，client 需
// default 分支，同 ProcurementRunStatus 的既有慣例）。
//
// ⛔ QUOTE_APPROVAL 已於 2026-09-20 從這個型別值域移除。它是 #2450 商務線
// 預留的值，而 #2450 已關票、WISH_POOL 定為不做，該值自始至終零 producer
// （dev 實測 run_type 只有 TURN／CLARIFICATION_RESUME／CANDIDATE_SEARCH／
// PUBLICATION 四種）。`chk_procurement_agent_runs_type` 裡的字面值留給
// contract 波一併收緊——收緊型 CHECK 屬破壞性變更，按 expand-contract 拆波。
type RunType string

const (
	RunTypeTurn                RunType = "TURN"
	RunTypeCandidateSearch     RunType = "CANDIDATE_SEARCH"
	RunTypeClarificationResume RunType = "CLARIFICATION_RESUME"
	// RunTypePublication 是 RFQ execute 自己的 run（#2450 Slice 2a，設計決定 1-A）。
	//
	// ⛔ 它與其餘四型的生命週期不同：run 在建立它的**同一個交易**內就走完
	// QUEUED → SUCCEEDED，沒有 worker、沒有 step 事件、也沒有 lease——它存在的唯一
	// 理由是 procurement_agent_run_artifacts.run_id 是 NOT NULL，
	// RFQ_PUBLICATION_RESULT artifact 必須掛在某個 run 上（doc 57 §4.5）。
	//
	// 這裡刻意仍是字面值常數而不是 CodeMeta 生成的值：RunType 這一組整體都還沒有
	// 登記成 CodeMeta 值域（pkg/codemeta 沒有 ProcurementRunType 群組），為單一個新值
	// 破例會把一個既有的未登記慣例升格成契約，屬獨立決定。
	RunTypePublication RunType = "PUBLICATION"
)

// ControllerKind records which non-legacy execution path claimed a run
// (Sub-project 3b, spec §0.3/§4.1) — Handle() never writes
// procurement_agent_runs at all, so there is no LEGACY value; every row
// in that table comes from either the durable-run path or the graph path.
type ControllerKind string

const (
	ControllerKindDurable ControllerKind = "DURABLE"
	ControllerKindGraph   ControllerKind = "GRAPH"
)

// 綁到 CodeMeta 生成的 var（同 steps.go／artifact.go 的既有手法：編譯期綁死，
// 避免第二份字面值）。
var (
	runStatusQueued    = codemeta.ProcurementRunStatusQueued.Value
	runStatusRunning   = codemeta.ProcurementRunStatusRunning.Value
	runStatusWaiting   = codemeta.ProcurementRunStatusWaitingForInput.Value
	runStatusCancelReq = codemeta.ProcurementRunStatusCancelRequested.Value
	runStatusSucceeded = codemeta.ProcurementRunStatusSucceeded.Value
	runStatusFailed    = codemeta.ProcurementRunStatusFailed.Value
	runStatusCancelled = codemeta.ProcurementRunStatusCancelled.Value
	runStatusResumed   = codemeta.ProcurementRunStatusResumed.Value
	runStatusExpired   = codemeta.ProcurementRunStatusExpired.Value
)

func RunStatusQueued() string    { return runStatusQueued }
func RunStatusRunning() string   { return runStatusRunning }
func RunStatusWaiting() string   { return runStatusWaiting }
func RunStatusCancelReq() string { return runStatusCancelReq }
func RunStatusSucceeded() string { return runStatusSucceeded }
func RunStatusFailed() string    { return runStatusFailed }
func RunStatusCancelled() string { return runStatusCancelled }
func RunStatusResumed() string   { return runStatusResumed }
func RunStatusExpired() string   { return runStatusExpired }

// IsTerminalRunStatus 判斷一個 run status 是否為終態（doc57 §6.4-3：終態不可
// reopen）。集中成一個函式，避免各呼叫端各自維護一份終態清單而漂移。
func IsTerminalRunStatus(status string) bool {
	switch status {
	case runStatusSucceeded, runStatusFailed, runStatusCancelled, runStatusResumed, runStatusExpired:
		return true
	default:
		return false
	}
}

// RunFailureRetryable 依失敗原因判斷「重新跑一次同一個命令是否值得」，是
// ErrorRecoveryData.Retryable 的唯一來源——該欄位明文規定由伺服器依 Reason 判斷、
// 不得由前端猜測。
//
// 目前只有兩個值回 true，兩者都是**基礎設施層的暫時失敗**，而且候選搜尋本來就有
// 一條重試路徑（RetryCandidateSet）：
//   - WORKER_LOST：執行程序消失（lease 過期／pod 重啟），與輸入無關。
//   - LANE_ALL_FAILED：兩條 lane 都失敗，成因是上游查詢逾時一類。
//
// 其餘一律 false，包含未知值。這是刻意保守：告訴買家「再試一次」而實際上重試必然
// 再失敗，比不給建議更糟——它把一個確定的失敗包裝成看似可恢復的狀態。
func RunFailureRetryable(reason string) bool {
	switch reason {
	case codemeta.ProcurementRunFailureReasonWorkerLost.Value,
		codemeta.ProcurementRunFailureReasonLaneAllFailed.Value:
		return true
	default:
		return false
	}
}
