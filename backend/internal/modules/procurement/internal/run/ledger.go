// Package run 是採購的 run 帳本：送出（配額、冪等、排入 job）、事件的寫入與續傳、逾時的 run 標失敗、事件保留期的清除，
// 以及 WatchRun 共用的 listener。LLM 與 provider 呼叫都不在這裡的 transaction 內（單體規則 5）。
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

// 事件類型：WatchRunResponse 的 oneof 欄位名。串流在送出 finished 或 error 之後結束。
const (
	EventRunAccepted = "run_accepted"
	EventFinished    = "finished"
	EventError       = "error"
)

// Run 的狀態與失敗原因（與 schema 的 CHECK 相同）。
const (
	StatusFailed = "FAILED"

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
	return eventType == EventFinished || eventType == EventError
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

// Append 在呼叫者的 transaction 內寫一筆事件並在 commit 時喚醒訂閱者；序號由 run 列配發。
func (l Ledger) Append(ctx context.Context, runID uuid.UUID, eventType string, payload []byte) (int64, error) {
	q := store.New(l.DB.Conn(ctx))
	now := l.Now()
	seq, err := q.NextRunSeq(ctx, store.NextRunSeqParams{ID: runID, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.NotFound("找不到這個 run")
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

// Finish 在同一個 transaction 內把 run 轉成終態並寫串流的結尾事件：狀態與結尾事件一起可見，訂閱者看得到終態就一定讀得到結尾。
// run 已經是終態（或不存在）時回 false、不寫事件。
func (l Ledger) Finish(ctx context.Context, runID uuid.UUID, status, failureReason string) (bool, error) {
	var done bool
	err := l.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(l.DB.Conn(ctx))
		var reason *string
		if failureReason != "" {
			reason = &failureReason
		}
		n, err := q.FinishRun(ctx, store.FinishRunParams{ID: runID, Status: status, FailureReason: reason, Now: l.Now()})
		if err != nil {
			return fmt.Errorf("run：終結：%w", err)
		}
		if n == 0 {
			return nil
		}
		payload, err := finishedPayload(status, failureReason)
		if err != nil {
			return err
		}
		if _, err := l.Append(ctx, runID, EventFinished, payload); err != nil {
			return err
		}
		done = true
		return nil
	})
	return done, err
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
