package turn

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// MaxMessageRunes 是買家一則訊息的長度上限，與 agent_run_inputs.buyer_message 的 CHECK 一致。
const MaxMessageRunes = 2000

// Quota 是買家本期的採購額度：上限與期間（TURN 才扣）。
type Quota struct {
	Limit                  int32
	PeriodStart, PeriodEnd time.Time
}

// QuotaSource 回買家現在有效的額度（買家模組的有效權益）。
type QuotaSource interface {
	TurnQuota(ctx context.Context, buyerID int64) (Quota, error)
}

// SubmitInput 是 SubmitTurn 的輸入。RequestID 為 nil 表示新建需求（連同它的對話）。
type SubmitInput struct {
	BuyerID         int64
	RequestID       *uuid.UUID
	Message         string
	ClientRequestID uuid.UUID
	Locale          uitext.Locale
}

// Submitted 是 SubmitTurn 的結果：需求、對話與這一輪的 run。
type Submitted struct {
	RequestID      uuid.UUID
	ConversationID uuid.UUID
	RunID          uuid.UUID
}

// Submitter 是 SubmitTurn 的 use case：在同一個 transaction 內解析（或新建）需求與對話，再經 run.Submitter 的送出路徑冪等建 run、扣額度、排入 job。
// 新建的需求與對話隨送出失敗（額度、上限、進行中）一起回滾。
type Submitter struct {
	Submitter run.Submitter
	Quota     QuotaSource
}

// Submit 送出買家的一則訊息。訊息去頭尾空白後不得為空、不得超過 MaxMessageRunes 個字元（INVALID_ARGUMENT，欄位 Message）；
// 指定的需求不屬於買家、已無對話或不存在回 NOT_FOUND；其餘（重送、同需求進行中、上限、額度）依 run.Submitter.Submit。
// 重送同一個 ClientRequestID 回第一次的需求、對話與 run（RequestID 為 nil 時不再新建需求），內容或需求不同回 INVALID_ARGUMENT。
func (s Submitter) Submit(ctx context.Context, in SubmitInput) (Submitted, error) {
	msg := trimtext.Trim(in.Message)
	switch {
	case msg == "":
		return Submitted{}, apperr.InvalidInputOn([]string{"Message"}, "訊息不能是空的")
	case utf8.RuneCountInString(msg) > MaxMessageRunes:
		return Submitted{}, apperr.InvalidInputOn([]string{"Message"}, "訊息不能超過 %d 個字元", MaxMessageRunes)
	}
	var out Submitted
	err := s.Submitter.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.Submitter.DB.Conn(ctx))
		if err := q.LockBuyerSubmits(ctx, in.BuyerID); err != nil {
			return fmt.Errorf("turn：鎖買家的送出：%w", err)
		}
		requestID, conversationID, replayed, err := s.resolve(ctx, q, in)
		if err != nil {
			return err
		}
		var quota Quota
		if !replayed {
			if quota, err = s.Quota.TurnQuota(ctx, in.BuyerID); err != nil {
				return fmt.Errorf("turn：讀取買家額度：%w", err)
			}
		}
		runID, err := s.Submitter.Submit(ctx, run.SubmitInput{
			BuyerID: in.BuyerID, RequestID: requestID, ConversationID: &conversationID, RunType: run.RunTypeTurn,
			ClientRequestID: in.ClientRequestID, Message: msg, Locale: in.Locale,
			QuotaLimit: quota.Limit, PeriodStart: quota.PeriodStart, PeriodEnd: quota.PeriodEnd,
		})
		if err != nil {
			return err
		}
		out = Submitted{RequestID: requestID, ConversationID: conversationID, RunID: runID}
		return nil
	})
	if err != nil {
		return Submitted{}, err
	}
	return out, nil
}

// resolve 回這次送出的需求與對話，以及是否為重送（重送回放不讀額度、不扣額度）：重送沿用第一次的（沒指定需求時不新建），指定的需求要屬於買家且有對話，沒指定就新建。
// 重送但指定了別的需求時回傳指定的，由 run.Submitter 的重送檢查回 INVALID_ARGUMENT。
func (s Submitter) resolve(ctx context.Context, q *store.Queries, in SubmitInput) (uuid.UUID, uuid.UUID, bool, error) {
	first, err := q.GetRunByIdempotencyKey(ctx, store.GetRunByIdempotencyKeyParams{BuyerID: in.BuyerID, ClientRequestID: in.ClientRequestID})
	switch {
	case err == nil && first.ConversationID != nil && (in.RequestID == nil || *in.RequestID == first.RequestID):
		if _, err := q.GetConversationForBuyer(ctx, store.GetConversationForBuyerParams{ID: *first.ConversationID, BuyerID: in.BuyerID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return uuid.UUID{}, uuid.UUID{}, false, apperr.NotFound("找不到這張需求的對話")
			}
			return uuid.UUID{}, uuid.UUID{}, false, fmt.Errorf("turn：讀取重送的對話：%w", err)
		}
		return first.RequestID, *first.ConversationID, true, nil
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return uuid.UUID{}, uuid.UUID{}, false, fmt.Errorf("turn：讀取重送的 run：%w", err)
	}
	if in.RequestID == nil {
		rid, cid, err := s.create(ctx, q, in.BuyerID)
		return rid, cid, false, err
	}
	if _, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: *in.RequestID, BuyerID: &in.BuyerID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, uuid.UUID{}, false, apperr.NotFound("找不到這張需求")
		}
		return uuid.UUID{}, uuid.UUID{}, false, fmt.Errorf("turn：讀取需求：%w", err)
	}
	conv, err := q.GetConversationByRequest(ctx, store.GetConversationByRequestParams{RequestID: *in.RequestID, BuyerID: in.BuyerID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, uuid.UUID{}, false, apperr.NotFound("找不到這張需求的對話")
		}
		return uuid.UUID{}, uuid.UUID{}, false, fmt.Errorf("turn：讀取對話：%w", err)
	}
	return *in.RequestID, conv.ID, false, nil
}

func (s Submitter) create(ctx context.Context, q *store.Queries, buyerID int64) (uuid.UUID, uuid.UUID, error) {
	now := s.Submitter.Ledger.Now()
	requestID, conversationID := uuid.NewV7(), uuid.NewV7()
	if err := q.InsertRequest(ctx, store.InsertRequestParams{ID: requestID, BuyerID: &buyerID, Now: now}); err != nil {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("turn：建立需求：%w", err)
	}
	if err := q.InsertConversation(ctx, store.InsertConversationParams{ID: conversationID, BuyerID: buyerID, RequestID: requestID, Now: now}); err != nil {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("turn：建立對話：%w", err)
	}
	return requestID, conversationID, nil
}
