package search

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// StartInput 是開始一次候選搜尋的輸入。
type StartInput struct {
	BuyerID         int64
	RequestID       uuid.UUID
	ClientRequestID uuid.UUID
}

// Started 是開始搜尋的結果：run 與它的候選集（兩個不同的 ID）以及 run 當下的狀態。
type Started struct {
	RunID          uuid.UUID
	CandidateSetID uuid.UUID
	Status         string
}

// Starter 是 StartCandidateSearch 的 use case：經 run.Submitter 的送出路徑，在同一個 transaction 內驗證前置、建候選集與 run 並排入 job。
// 不扣額度、不改需求狀態（成功時由 executor 轉 MATCHING）。
type Starter struct {
	Submitter run.Submitter
}

// Start 開始一次搜尋。需求不屬於買家回 NOT_FOUND；需求不在 REQUIREMENT_READY 或 MATCHING 回 FAILED_PRECONDITION／REQUIREMENT_NOT_READY；
// 同需求已有進行中的 run 或買家的上限，依 run.Submitter。同一個 client_request_id 重送回第一次的 run 與它的候選集。
func (s Starter) Start(ctx context.Context, in StartInput) (Started, error) {
	runID, err := s.Submitter.Submit(ctx, run.SubmitInput{
		BuyerID: in.BuyerID, RequestID: in.RequestID, RunType: run.RunTypeCandidateSearch, ClientRequestID: in.ClientRequestID,
		CheckRequest: func(ctx context.Context, q *store.Queries) error {
			return s.check(ctx, q, in)
		},
		PrepareCandidateSet: func(ctx context.Context, q *store.Queries) (uuid.UUID, error) {
			return s.prepare(ctx, q, in)
		},
	})
	if err != nil {
		return Started{}, err
	}
	r, err := store.New(s.Submitter.DB.Conn(ctx)).GetRunForBuyer(ctx, store.GetRunForBuyerParams{ID: runID, BuyerID: in.BuyerID})
	if err != nil {
		return Started{}, fmt.Errorf("search：讀取 run：%w", err)
	}
	if r.CandidateSetID == nil {
		return Started{}, fmt.Errorf("search：run %s 沒有 candidate_set_id", r.ID)
	}
	return Started{RunID: r.ID, CandidateSetID: *r.CandidateSetID, Status: r.Status}, nil
}

// check 在買家的送出鎖之內、上限檢查之前驗證需求屬於買家且狀態允許搜尋。
func (s Starter) check(ctx context.Context, q *store.Queries, in StartInput) error {
	buyer := in.BuyerID
	req, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: in.RequestID, BuyerID: &buyer})
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到這個需求")
	}
	if err != nil {
		return fmt.Errorf("search：讀取需求：%w", err)
	}
	if !slices.Contains(searchable, req.Status) {
		return notReady()
	}
	return nil
}

// prepare 在上限檢查之後讀需求的現行版本並建候選集，候選集指向需求當下的現行版本。
func (s Starter) prepare(ctx context.Context, q *store.Queries, in StartInput) (uuid.UUID, error) {
	buyer := in.BuyerID
	rev, err := q.GetActiveRevision(ctx, store.GetActiveRevisionParams{RequestID: in.RequestID, BuyerID: &buyer})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("search：讀取現行版本：%w", err)
	}
	if _, err := queryOf(rev); err != nil {
		return uuid.UUID{}, notReady()
	}
	id := uuid.NewV7()
	if err := q.InsertCandidateSet(ctx, store.InsertCandidateSetParams{ID: id, RequestID: in.RequestID, RequestRevisionID: rev.ID, BuyerID: in.BuyerID, Now: s.Submitter.Ledger.Now()}); err != nil {
		return uuid.UUID{}, fmt.Errorf("search：建立候選集：%w", err)
	}
	return id, nil
}

func notReady() error {
	return apperr.FailedPrecondition("需求的必填欄位還沒有填滿，不能搜尋候選").WithReason(apperr.ReasonRequirementNotReady)
}
