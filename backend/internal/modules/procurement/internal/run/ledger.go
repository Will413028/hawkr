// Package run 是採購的 run 帳本：送出（配額、冪等、排入 job）、受守衛的狀態轉換與事件的寫入與續傳、取消、
// reconciler 對帳 job 與 run、事件保留期的清除，以及 WatchRun 共用的 listener。LLM 與 provider 呼叫都不在這裡的 transaction 內（單體規則 5）。
package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// 事件類型：WatchRunResponse 的 oneof 欄位名。串流在送出 finished 之後結束；成功、失敗與取消都以它收尾。
const (
	EventRunAccepted = "run_accepted"
	EventFinished    = "finished"
	EventTurnResult  = "turn_result"
)

// Run 的失敗原因（與 schema 的 CHECK 相同）。
const (
	FailureWorkerLost       = "WORKER_LOST"
	FailureSearchFailed     = "SEARCH_FAILED"
	FailureTurnPipelineErr  = "TURN_PIPELINE_ERROR"
	notifyChannel           = "procurement_run"
	protoStatusPrefix       = "RUN_STATUS_"
	protoFailureReasonPrefx = "RUN_FAILURE_REASON_"
)

// Run 的類型（與 schema 的 CHECK 相同）。只有 TURN 扣額度。
const (
	RunTypeTurn            = "TURN"
	RunTypeCandidateSearch = "CANDIDATE_SEARCH"
	RunTypeResume          = "CLARIFICATION_RESUME"
)

// ChargesQuota 回報這個類型的 run 是否扣額度（A9-D24）：候選搜尋與回答中斷的子 run 不扣。
func ChargesQuota(runType string) bool { return runType == RunTypeTurn }

// IsTerminalEvent 回報事件類型是否是串流的結尾。
func IsTerminalEvent(eventType string) bool {
	return eventType == EventFinished
}

// Event 是帳本裡的一個事件；Payload 是 WatchRunResponse 的 oneof 欄位的 proto JSON（{"<type>": {...}}）。
type Event struct {
	Seq        int64
	Type       string
	Payload    []byte
	OccurredAt time.Time
}

// Cursor 是事件位置的不透明字串；0 表示還沒有事件。
func Cursor(seq int64) string {
	if seq <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", seq)
}

// Ledger 寫帳本。
type Ledger struct {
	DB  *db.DB
	Now func() time.Time
}

// Append 在 transaction 內寫一筆事件並在 commit 時喚醒訂閱者；序號由 run 列配發。
// 只在 run 非終態時寫：終態之後回 ErrRunTerminal（executor 收到就該停止），run 不存在回 NOT_FOUND。
// 串流的結尾事件 finished 不經這裡，它只隨終態轉換一起寫（Transition）。
func (l Ledger) Append(ctx context.Context, runID uuid.UUID, eventType string, payload []byte) (int64, error) {
	if IsTerminalEvent(eventType) {
		return 0, fmt.Errorf("run：%s 事件只能隨終態轉換寫入", eventType)
	}
	var seq int64
	err := l.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
		seq, err = l.append(ctx, runID, eventType, payload, activeStatuses())
		return err
	})
	return seq, err
}

func (l Ledger) append(ctx context.Context, runID uuid.UUID, eventType string, payload []byte, statuses []string) (int64, error) {
	q := store.New(l.DB.Conn(ctx))
	now := l.Now()
	seq, err := q.NextRunSeq(ctx, store.NextRunSeqParams{ID: runID, Now: now, Statuses: statuses})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, l.missingOrTerminal(ctx, q, runID)
	}
	if err != nil {
		return 0, fmt.Errorf("run：配發事件序號：%w", err)
	}
	if err := q.InsertRunEvent(ctx, store.InsertRunEventParams{RunID: runID, Seq: seq, EventType: eventType, Payload: payload, Now: now}); err != nil {
		return 0, fmt.Errorf("run：寫入事件：%w", err)
	}
	if err := q.NotifyRun(ctx, runID.String()); err != nil {
		return 0, fmt.Errorf("run：喚醒訂閱者：%w", err)
	}
	return seq, nil
}

func (l Ledger) missingOrTerminal(ctx context.Context, q *store.Queries, runID uuid.UUID) error {
	r, err := q.GetRun(ctx, runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到這個 run")
	}
	if err != nil {
		return fmt.Errorf("run：讀取 run：%w", err)
	}
	if IsTerminal(r.Status) {
		return ErrRunTerminal
	}
	return fmt.Errorf("run：狀態 %s 不接受寫入", r.Status)
}

// Transition 是 run 狀態的公開寫入口：同一個 transaction 內以 UPDATE … WHERE status IN (允許的來源) 轉換，終態同時寫串流的結尾事件
// （狀態與結尾事件一起可見，訂閱者看得到終態就一定讀得到結尾）。回傳轉換後的列。
// run 已在終態回 ErrRunTerminal，非終態但沒有這條邊回 *InvalidTransitionError，不存在回 NOT_FOUND。
// 轉入 WAITING_FOR_INPUT 只能經 Interrupt（要同時有中斷），WAITING_FOR_INPUT 轉 SUCCEEDED 只在回答的提交點（要同時解決中斷）；這兩條邊公開的 Transition 都拒絕。
// 回答中斷的 run（interrupt_id 非空）轉 SUCCEEDED 是回答的提交點：被回答的中斷轉 RESOLVED、提問的 run 轉 SUCCEEDED；
// 等待輸入的 run 轉 CANCELLED 時它的 PENDING 中斷轉 CANCELLED。回答的 run 失敗或被取消不動中斷與提問的 run。
func (l Ledger) Transition(ctx context.Context, runID uuid.UUID, to, failureReason string) (store.ProcurementAgentRun, error) {
	if to == StatusWaitingForInput {
		return store.ProcurementAgentRun{}, fmt.Errorf("run：%s 只能經 Ledger.Interrupt 轉入", to)
	}
	return l.commitTransition(ctx, runID, to, failureReason, publicFrom(to))
}

// commitTransition 是 Transition 與 Interrupt 共用的轉換，from 是允許的來源狀態。
// 回答中斷的 run 轉 SUCCEEDED 或 WAITING_FOR_INPUT 時同一個 transaction 內提交回答（commitAnswer）。
func (l Ledger) commitTransition(ctx context.Context, runID uuid.UUID, to, failureReason string, from []string) (store.ProcurementAgentRun, error) {
	var out store.ProcurementAgentRun
	err := l.DB.InTx(ctx, func(ctx context.Context) error {
		if err := l.lockAskingOf(ctx, runID); err != nil {
			return err
		}
		r, err := l.transition(ctx, runID, to, failureReason, from)
		if err != nil {
			return err
		}
		out = r
		if r.InterruptID != nil && (to == StatusSucceeded || to == StatusWaitingForInput) {
			return l.commitAnswer(ctx, *r.InterruptID)
		}
		return nil
	})
	return out, err
}

// lockAskingOf 在 run 是回答 run 時先鎖提問的 run 與被回答的中斷：全域鎖順序是提問的 run、中斷、回答的 run，
// 每條會鎖回答 run 的路徑（回答的提交點、取消、取代、settleFailure、reconciler）都先經這裡或 lockRun，否則兩條路徑互相等待。
// run、提問的 run 或中斷已被刪除回 NOT_FOUND（與刪除後的轉換同一個結果）；不是回答 run 時只讀不鎖。
func (l Ledger) lockAskingOf(ctx context.Context, runID uuid.UUID) error {
	q := store.New(l.DB.Conn(ctx))
	r, err := q.GetRun(ctx, runID)
	if err != nil {
		return lockErr(err, "讀取 run")
	}
	if r.InterruptID == nil {
		return nil
	}
	irq, err := q.GetInterrupt(ctx, *r.InterruptID)
	if err != nil {
		return lockErr(err, "讀取被回答的中斷")
	}
	if _, err := q.LockRun(ctx, irq.RunID); err != nil {
		return lockErr(err, "鎖提問的 run")
	}
	if _, err := q.LockInterrupt(ctx, irq.ID); err != nil {
		return lockErr(err, "鎖被回答的中斷")
	}
	return nil
}

// lockRun 依全域鎖順序鎖住 run（先 lockAskingOf 再鎖它自己）並回傳鎖住的列；不存在回 NOT_FOUND。
func (l Ledger) lockRun(ctx context.Context, runID uuid.UUID) (store.ProcurementAgentRun, error) {
	if err := l.lockAskingOf(ctx, runID); err != nil {
		return store.ProcurementAgentRun{}, err
	}
	r, err := store.New(l.DB.Conn(ctx)).LockRun(ctx, runID)
	if err != nil {
		return store.ProcurementAgentRun{}, lockErr(err, "鎖 run")
	}
	return r, nil
}

func lockErr(err error, what string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到這個 run")
	}
	return fmt.Errorf("run：%s：%w", what, err)
}

func (l Ledger) transition(ctx context.Context, runID uuid.UUID, to, failureReason string, from []string) (store.ProcurementAgentRun, error) {
	if err := checkTarget(to, failureReason); err != nil {
		return store.ProcurementAgentRun{}, err
	}
	var out store.ProcurementAgentRun
	err := l.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(l.DB.Conn(ctx))
		var reason *string
		if failureReason != "" {
			reason = &failureReason
		}
		terminal := IsTerminal(to)
		now := l.Now()
		r, err := q.TransitionRun(ctx, store.TransitionRunParams{
			ID: runID, ToStatus: to, FailureReason: reason, Terminal: terminal, FromStatuses: from, Now: now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return l.rejectedTransition(ctx, q, runID, to)
		}
		if err != nil {
			return fmt.Errorf("run：轉換狀態：%w", err)
		}
		out = r
		if to == StatusCancelled {
			if _, err := q.CancelInterruptsOfRun(ctx, store.CancelInterruptsOfRunParams{RunID: runID, Now: &now}); err != nil {
				return fmt.Errorf("run：取消中斷：%w", err)
			}
		}
		if !terminal {
			return nil
		}
		payload, err := finishedPayload(to, failureReason)
		if err != nil {
			return err
		}
		_, err = l.append(ctx, runID, EventFinished, payload, []string{to})
		return err
	})
	return out, err
}

// ErrInterruptNotPending 表示回答的提交點發現被回答的中斷已不是 PENDING（提問的 run 在回答期間被取消或取代）；提交整個回滾，回答的 run 留在原狀態，由呼叫端（executor）把它標失敗。
var ErrInterruptNotPending = errors.New("run：被回答的中斷已不在等待回答")

// commitAnswer 是回答成功的提交點（與回答 run 的轉換同一個 transaction，提問的 run 與中斷已由 lockAskingOf 鎖住）：
// 中斷轉 RESOLVED，提問的 run 由 WAITING_FOR_INPUT 轉到 SUCCEEDED。
func (l Ledger) commitAnswer(ctx context.Context, interruptID uuid.UUID) error {
	q := store.New(l.DB.Conn(ctx))
	irq, err := q.GetInterrupt(ctx, interruptID)
	if err != nil {
		return fmt.Errorf("run：讀取被回答的中斷：%w", err)
	}
	if irq.Status != "PENDING" {
		return ErrInterruptNotPending
	}
	if _, err := l.transition(ctx, irq.RunID, StatusSucceeded, "", allowedFrom(StatusSucceeded)); err != nil {
		if errors.Is(err, ErrRunTerminal) {
			return ErrInterruptNotPending
		}
		return err
	}
	if _, err := q.ResolveInterrupt(ctx, store.ResolveInterruptParams{ID: interruptID, Now: ptr(l.Now())}); err != nil {
		return fmt.Errorf("run：中斷轉已解決：%w", err)
	}
	return nil
}

func (l Ledger) rejectedTransition(ctx context.Context, q *store.Queries, runID uuid.UUID, to string) error {
	r, err := q.LockRun(ctx, runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到這個 run")
	}
	if err != nil {
		return fmt.Errorf("run：讀取 run：%w", err)
	}
	if IsTerminal(r.Status) {
		return ErrRunTerminal
	}
	return &InvalidTransitionError{From: r.Status, To: to}
}

func checkTarget(to, failureReason string) error {
	if _, known := edges[to]; !known || len(allowedFrom(to)) == 0 {
		return fmt.Errorf("run：%q 不是可轉入的狀態", to)
	}
	switch {
	case to == StatusFailed && failureReason != FailureWorkerLost && failureReason != FailureSearchFailed && failureReason != FailureTurnPipelineErr:
		return fmt.Errorf("run：FAILED 需要失敗原因，收到 %q", failureReason)
	case to != StatusFailed && failureReason != "":
		return fmt.Errorf("run：%s 不帶失敗原因", to)
	}
	return nil
}

func finishedPayload(status, failureReason string) ([]byte, error) {
	body := map[string]string{"status": protoStatusPrefix + status}
	if failureReason != "" {
		body["failureReason"] = protoFailureReasonPrefx + failureReason
	}
	return json.Marshal(map[string]any{EventFinished: body})
}

func ptr[T any](v T) *T { return &v }
