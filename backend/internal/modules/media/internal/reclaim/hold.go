package reclaim

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/hold"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const ExpireHoldsInterval = SweepInterval

func (s *Service) Hold(ctx context.Context, assetIDs []uuid.UUID, source string, sourceID uuid.UUID, until time.Time) error {
	if err := model.RequireTx(ctx); err != nil {
		return err
	}
	return hold.Place(ctx, s.q(ctx), assetIDs, source, sourceID, until, s.d.Now(), true)
}

func (s *Service) ExtendHold(ctx context.Context, source string, sourceID uuid.UUID, until time.Time) error {
	if err := model.RequireTx(ctx); err != nil {
		return err
	}
	return hold.Extend(ctx, s.q(ctx), source, sourceID, until, s.d.Now())
}

func (s *Service) ReleaseHold(ctx context.Context, source string, sourceID uuid.UUID) error {
	if err := model.RequireTx(ctx); err != nil {
		return err
	}
	return hold.Release(ctx, s.q(ctx), source, sourceID, s.d.Now())
}

func (s *Service) ExpireHolds(ctx context.Context) (int, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return 0, err
	}
	total := 0
	after := uuid.UUID{}
	for {
		now := s.d.Now()
		ids, err := s.q(ctx).ListAssetsWithDeadHolds(ctx, store.ListAssetsWithDeadHoldsParams{AfterID: after, Now: now, MaxRows: batchSize})
		if err != nil {
			return total, fmt.Errorf("media：列出有到期保全的資產：%w", err)
		}
		for _, id := range ids {
			n, err := s.settleHolds(ctx, id, now)
			if err != nil {
				return total, err
			}
			total += n
		}
		if len(ids) < batchSize {
			return total, nil
		}
		after = ids[len(ids)-1]
	}
}

func (s *Service) settleHolds(ctx context.Context, assetID uuid.UUID, now time.Time) (int, error) {
	n := 0
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := hold.SetLockTimeout(ctx, s.d.DB.Conn(ctx)); err != nil {
			return err
		}
		var err error
		n, err = hold.Settle(ctx, s.q(ctx), assetID, now)
		return err
	})
	if hold.IsContention(err) {
		s.d.Logger.Info("media：整理保全撞上擁有者未提交的保全，跳過", "asset_id", assetID)
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("media：整理資產 %s 的保全：%w", assetID, err)
	}
	return n, nil
}

type ExpireHoldsArgs struct{}

func (ExpireHoldsArgs) Kind() string { return "media.expire_holds" }

func (ExpireHoldsArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(ExpireHoldsInterval)
	o.MaxAttempts = MaxAttempts
	return o
}

type expireHoldsWorker struct {
	river.WorkerDefaults[ExpireHoldsArgs]
	svc *Service
}

func (w *expireHoldsWorker) Work(ctx context.Context, _ *river.Job[ExpireHoldsArgs]) error {
	_, err := w.svc.ExpireHolds(ctx)
	return err
}

func (w *expireHoldsWorker) Timeout(*river.Job[ExpireHoldsArgs]) time.Duration { return time.Minute }
