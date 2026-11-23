package search

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// SelectInput 是 ToggleSelection 的輸入：Selected 是目標狀態。
type SelectInput struct {
	BuyerID                                int64
	RequestID, CandidateSetID, CandidateID uuid.UUID
	Selected                               bool
}

// Selector 是 ToggleSelection 的 use case。
type Selector struct {
	DB  *db.DB
	Now func() time.Time
}

// Select 把一個候選設成指定的勾選狀態（目標狀態的方法，重送結果相同）。需求不屬於買家、對話已刪除、候選集不屬於這個需求或候選不在候選集裡回 NOT_FOUND；
// 候選集不是有效的候選集（需求不在 MATCHING，或不是最新一筆成功搜尋的候選集，A9-D29(4)）回 ABORTED／VERSION_CONFLICT，conflictTarget 是 candidate_set。
// 有效性檢查與寫入在同一個 transaction、同一把需求列鎖之下，與搜尋的提交串行。
func (s Selector) Select(ctx context.Context, in SelectInput) error {
	return s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		buyer := in.BuyerID
		req, err := q.LockRequestForBuyer(ctx, store.LockRequestForBuyerParams{ID: in.RequestID, BuyerID: &buyer})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這張需求")
		}
		if err != nil {
			return fmt.Errorf("search：鎖需求：%w", err)
		}
		if _, err := q.GetConversationByRequest(ctx, store.GetConversationByRequestParams{RequestID: in.RequestID, BuyerID: in.BuyerID}); errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這張需求的對話")
		} else if err != nil {
			return fmt.Errorf("search：讀取對話：%w", err)
		}
		if _, err := q.GetCandidateSetForBuyer(ctx, store.GetCandidateSetForBuyerParams{ID: in.CandidateSetID, RequestID: in.RequestID, BuyerID: in.BuyerID}); errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這個候選集")
		} else if err != nil {
			return fmt.Errorf("search：讀取候選集：%w", err)
		}
		latest, err := q.GetLatestSucceededSearchSet(ctx, store.GetLatestSucceededSearchSetParams{RequestID: in.RequestID, BuyerID: in.BuyerID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("search：讀取最新的候選集：%w", err)
		}
		if req.Status != statusMatching || err != nil || *latest != in.CandidateSetID {
			return apperr.VersionConflict("這個候選集已不是有效的候選集，請重新載入").WithMetadata("conflictTarget", "candidate_set")
		}
		n, err := q.SetCandidateSelected(ctx, store.SetCandidateSelectedParams{
			ID: in.CandidateID, Selected: in.Selected, Now: s.Now(), CandidateSetID: in.CandidateSetID, RequestID: in.RequestID, BuyerID: in.BuyerID,
		})
		if err != nil {
			return fmt.Errorf("search：設定勾選：%w", err)
		}
		if n == 0 {
			return apperr.NotFound("找不到這個候選")
		}
		return nil
	})
}
