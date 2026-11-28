package hold

import (
	"context"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
)

func Place(ctx context.Context, q *store.Queries, assetIDs []uuid.UUID, source string, sourceID uuid.UUID, until, now time.Time) error {
	if source == "" {
		return fmt.Errorf("media：保全的來源不得為空")
	}
	if !until.After(now) {
		return fmt.Errorf("media：保全期限 %s 不在現在 %s 之後", until, now)
	}
	ids := slices.Clone(assetIDs)
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	ids = slices.Compact(ids)
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.LockAssetsForHold(ctx, ids)
	if err != nil {
		return fmt.Errorf("media：鎖定要保全的資產：%w", err)
	}
	if len(rows) != len(ids) {
		return model.ErrAssetNotFound
	}
	for _, r := range rows {
		if model.AssetState(r.State) != model.AssetStaged {
			return model.ErrAssetGone
		}
	}
	if err := q.UpsertHolds(ctx, store.UpsertHoldsParams{Ids: ids, Source: source, SourceID: sourceID, Until: until, Now: now}); err != nil {
		return fmt.Errorf("media：登記保全：%w", err)
	}
	return nil
}
