package search

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/reply"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Reader 是 GetCandidateSearch 的讀取模型。
type Reader struct{ DB *db.DB }

// Get 回傳買家自己的候選搜尋 run 的狀態與結果（A9-D29(3)）。run 不存在、不是這位買家的、不是 CANDIDATE_SEARCH，或不屬於這個需求，一律回 NOT_FOUND。
// 只有 SUCCEEDED 的 run 有候選；FAILED 帶失敗原因；其餘狀態（排隊、執行中、取消）只回狀態。
// 候選的事實依需求的現行版本與語系在讀取時組出，FactsAsOf 是 run 完成的時間。呼叫端的 ctx 不可已在 transaction 內。
func (g Reader) Get(ctx context.Context, buyerID int64, requestID, runID uuid.UUID, locale uitext.Locale) (model.CandidateSearch, error) {
	var out model.CandidateSearch
	err := g.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		out, err = g.read(ctx, buyerID, requestID, runID, locale)
		return err
	})
	return out, err
}

func (g Reader) read(ctx context.Context, buyerID int64, requestID, runID uuid.UUID, locale uitext.Locale) (model.CandidateSearch, error) {
	q := store.New(g.DB.Conn(ctx))
	r, err := q.GetRunForBuyer(ctx, store.GetRunForBuyerParams{ID: runID, BuyerID: buyerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.CandidateSearch{}, notFound()
	}
	if err != nil {
		return model.CandidateSearch{}, fmt.Errorf("search：讀取 run：%w", err)
	}
	if r.RunType != run.RunTypeCandidateSearch || r.RequestID != requestID {
		return model.CandidateSearch{}, notFound()
	}
	if r.CandidateSetID == nil {
		return model.CandidateSearch{}, fmt.Errorf("search：搜尋 run %s 沒有 candidate_set_id", r.ID)
	}
	out := model.CandidateSearch{RequestID: requestID, RunID: r.ID, CandidateSetID: *r.CandidateSetID, Status: r.Status}
	switch r.Status {
	case run.StatusFailed:
		if r.FailureReason == nil {
			return model.CandidateSearch{}, fmt.Errorf("search：失敗的 run %s 沒有失敗原因", r.ID)
		}
		out.FailureReason = *r.FailureReason
		return out, nil
	case run.StatusSucceeded:
	default:
		return out, nil
	}
	if r.CompletedAt == nil {
		return model.CandidateSearch{}, fmt.Errorf("search：成功的 run %s 沒有完成時間", r.ID)
	}
	owner := buyerID
	rev, err := q.GetActiveRevision(ctx, store.GetActiveRevisionParams{RequestID: requestID, BuyerID: &owner})
	if err != nil {
		return model.CandidateSearch{}, fmt.Errorf("search：讀取現行版本：%w", err)
	}
	matched, unverifiable := reply.CandidateFacts(locale, slots.FromRevision(rev))
	rows, err := q.ListCandidates(ctx, store.ListCandidatesParams{CandidateSetID: *r.CandidateSetID, BuyerID: buyerID})
	if err != nil {
		return model.CandidateSearch{}, fmt.Errorf("search：讀取候選：%w", err)
	}
	for _, c := range rows {
		out.Candidates = append(out.Candidates, model.Candidate{
			ID: c.ID, StoreID: c.StoreID, Rank: int(c.Rank), MatchedFacts: matched, UnverifiableFacts: unverifiable,
			FactsAsOf: *r.CompletedAt, Selected: c.Selected,
		})
	}
	return out, nil
}

func notFound() error { return apperr.NotFound("找不到這次候選搜尋") }
