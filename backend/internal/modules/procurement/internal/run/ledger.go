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
)

// Run 的失敗原因（與 schema 的 CHECK 相同）。
const (
	FailureWorkerLost       = "WORKER_LOST"
	FailureSearchFailed     = "SEARCH_FAILED"
	FailureTurnPipelineErr  = "TURN_PIPELINE_ERROR"
	runTypeCandidateSearch  = "CANDIDATE_SEARCH"
	notifyChannel           = "procurement_run"
	protoStatusPrefix       = "RUN_STATUS_"
	protoFailureReasonPrefx = "RUN_FAILURE_REASON_"
)

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

// Transition 是 run 狀態的唯一寫入口：同一個 transaction 內以 UPDATE … WHERE status IN (允許的來源) 轉換，終態同時寫串流的結尾事件
// （狀態與結尾事件一起可見，訂閱者看得到終態就一定讀得到結尾）。回傳轉換後的列。
// run 已在終態回 ErrRunTerminal，非終態但沒有這條邊回 *InvalidTransitionError，不存在回 NOT_FOUND。
func (l Ledger) Transition(ctx context.Context, runID uuid.UUID, to, failureReason string) (store.ProcurementAgentRun, error) {
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
		r, err := q.TransitionRun(ctx, store.TransitionRunParams{
			ID: runID, ToStatus: to, FailureReason: reason, Terminal: terminal, FromStatuses: allowedFrom(to), Now: l.Now(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return l.rejectedTransition(ctx, q, runID, to)
		}
		if err != nil {
			return fmt.Errorf("run：轉換狀態：%w", err)
		}
		out = r
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

// UpsertArtifact 就地更新 (run, artifact_type) 的產出並回傳新的 version（ARTIFACT_UPSERTED 的去重序號）。
func (l Ledger) UpsertArtifact(ctx context.Context, runID uuid.UUID, artifactType, status string, data []byte, checksum string, asOf time.Time) (int32, error) {
	v, err := store.New(l.DB.Conn(ctx)).UpsertRunArtifact(ctx, store.UpsertRunArtifactParams{
		RunID: runID, ArtifactType: artifactType, Status: status, Data: data, Checksum: checksum, AsOf: asOf, Now: l.Now(),
	})
	if err != nil {
		return 0, fmt.Errorf("run：寫入產出：%w", err)
	}
	return v, nil
}
