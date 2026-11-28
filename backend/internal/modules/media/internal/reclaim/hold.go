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
	return hold.Place(ctx, s.q(ctx), assetIDs, source, sourceID, until, s.d.Now())
}

func (s *Service) ExtendHold(ctx context.Context, source string, sourceID uuid.UUID, until time.Time) error {
	if err := model.RequireTx(ctx); err != nil {
		return err
	}
	n, err := s.q(ctx).ExtendHolds(ctx, store.ExtendHoldsParams{Source: source, SourceID: sourceID, Until: until})
	if err != nil {
		return fmt.Errorf("media：延長保全：%w", err)
	}
	if n == 0 {
		return model.ErrHoldNotFound
	}
	return nil
}

func (s *Service) ReleaseHold(ctx context.Context, source string, sourceID uuid.UUID) error {
	if err := model.RequireTx(ctx); err != nil {
		return err
	}
	if _, err := s.q(ctx).ReleaseHolds(ctx, store.ReleaseHoldsParams{Source: source, SourceID: sourceID}); err != nil {
		return fmt.Errorf("media：解除保全：%w", err)
	}
	return nil
}

func (s *Service) ExpireHolds(ctx context.Context) (int, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return 0, err
	}
	var n int64
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
		n, err = s.q(ctx).DeleteExpiredHolds(ctx, s.d.Now())
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("media：整理到期的保全：%w", err)
	}
	return int(n), nil
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
