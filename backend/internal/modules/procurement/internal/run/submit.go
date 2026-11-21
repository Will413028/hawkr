package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// RunArgs 是執行一個 run 的 job。BuyerID 與 PeriodStart 讓失敗時能退回額度，不必回頭查。
// 不重試（MaxAttempts 1）：LLM 呼叫不冪等，重驅會產生不同結果並重複扣額度（A9-D6）；逾時沒完成的由 sweeper 標失敗。
type RunArgs struct {
	RunID       uuid.UUID `json:"runId"`
	BuyerID     int64     `json:"buyerId"`
	PeriodStart time.Time `json:"periodStart"`
}

func (RunArgs) Kind() string { return "procurement.run" }

func (RunArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// SubmitInput 是送出一個 run 的輸入。額度的上限與期間由呼叫端依買家的方案給。
type SubmitInput struct {
	BuyerID         int64
	RequestID       uuid.UUID
	ConversationID  *uuid.UUID
	RunType         string
	ClientRequestID uuid.UUID
	Message         string
	QuotaLimit      int32
	PeriodStart     time.Time
	PeriodEnd       time.Time
}

// Submitter 是送出路徑：同一個 transaction 內冪等建 run、扣一次額度、寫輸入與第一個事件、排入 job。
type Submitter struct {
	DB     *db.DB
	Ledger Ledger
	Jobs   jobs.Enqueuer
}

// Submit 回傳 run ID；同一位買家以同一個 client_request_id 重送回第一次的 run（內容不同回 INVALID_ARGUMENT），不再扣額度。
// 額度用完回 RESOURCE_EXHAUSTED（AGENT_MONTHLY_QUOTA_EXCEEDED），run 與扣款一起回滾。
func (s Submitter) Submit(ctx context.Context, in SubmitInput) (uuid.UUID, error) {
	id := uuid.NewV7()
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		now := s.Ledger.Now()
		n, err := q.InsertRun(ctx, store.InsertRunParams{
			ID: id, BuyerID: in.BuyerID, RequestID: in.RequestID, ConversationID: in.ConversationID,
			RunType: in.RunType, ClientRequestID: in.ClientRequestID, Now: now,
		})
		if err != nil {
			return fmt.Errorf("run：建立 run：%w", err)
		}
		if n == 0 {
			id, err = replay(ctx, q, in)
			return err
		}
		if _, err := q.ConsumeUsage(ctx, store.ConsumeUsageParams{BuyerID: in.BuyerID, PeriodStart: in.PeriodStart, Now: now, QuotaLimit: in.QuotaLimit}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.ResourceExhausted(0, "本期的採購額度已用完").
					WithReason(apperr.ReasonAgentMonthlyQuotaExceeded).
					WithMetadata("remaining", "0").
					WithMetadata("resetTime", in.PeriodEnd.UTC().Format(time.RFC3339))
			}
			return fmt.Errorf("run：扣額度：%w", err)
		}
		if err := q.InsertRunInput(ctx, store.InsertRunInputParams{RunID: id, BuyerMessage: in.Message, Now: now}); err != nil {
			return fmt.Errorf("run：寫入輸入：%w", err)
		}
		accepted, err := acceptedPayload(id, in.ConversationID)
		if err != nil {
			return err
		}
		if _, err := s.Ledger.Append(ctx, id, EventRunAccepted, accepted); err != nil {
			return err
		}
		return s.Jobs.Enqueue(ctx, RunArgs{RunID: id, BuyerID: in.BuyerID, PeriodStart: in.PeriodStart}, nil)
	})
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

func acceptedPayload(runID uuid.UUID, conversationID *uuid.UUID) ([]byte, error) {
	body := map[string]string{"runId": runID.String()}
	if conversationID != nil {
		body["conversationId"] = conversationID.String()
	}
	b, err := json.Marshal(map[string]any{EventRunAccepted: body})
	if err != nil {
		return nil, fmt.Errorf("run：組 run_accepted 事件：%w", err)
	}
	return b, nil
}

func replay(ctx context.Context, q *store.Queries, in SubmitInput) (uuid.UUID, error) {
	r, err := q.GetRunByIdempotencyKey(ctx, store.GetRunByIdempotencyKeyParams{BuyerID: in.BuyerID, ClientRequestID: in.ClientRequestID})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("run：讀取重送的 run：%w", err)
	}
	input, err := q.GetRunInput(ctx, store.GetRunInputParams{RunID: r.ID, BuyerID: in.BuyerID})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("run：讀取重送的輸入：%w", err)
	}
	if r.RequestID != in.RequestID || r.RunType != in.RunType || input.BuyerMessage != in.Message {
		return uuid.UUID{}, apperr.InvalidInputOn([]string{"client_request_id"}, "同一個 client_request_id 的內容不同")
	}
	return r.ID, nil
}
