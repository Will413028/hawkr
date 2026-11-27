package run

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// RetentionPeriod 是 run 事件在 run 結束後保留的時間（A9-D20 R1）。
const RetentionPeriod = 7 * 24 * time.Hour

// PurgeInterval 是事件清除的週期。
const PurgeInterval = time.Hour

// Purger 清除結束超過保留期的 run 的事件並推進 min_available_seq。
type Purger struct {
	DB     *db.DB
	Ledger Ledger
}

const purgeBatch = 100

// Purge 分批清除，直到沒有符合條件的 run，回傳刪除的事件數。
func (p Purger) Purge(ctx context.Context) (int64, error) {
	cutoff := p.Ledger.Now().Add(-RetentionPeriod)
	return drainExpired(ctx, func(ctx context.Context) (int64, int64, error) {
		r, err := store.New(p.DB.Conn(ctx)).ExpireRunEvents(ctx, store.ExpireRunEventsParams{Cutoff: cutoff, Batch: purgeBatch})
		if err != nil {
			return 0, 0, fmt.Errorf("run：清除過期事件：%w", err)
		}
		return r.Runs, r.Events, nil
	})
}

func drainExpired(ctx context.Context, step func(context.Context) (runs, events int64, err error)) (int64, error) {
	var total int64
	for {
		runs, events, err := step(ctx)
		if err != nil {
			return total, err
		}
		total += events
		if runs == 0 {
			return total, nil
		}
		if err := ctx.Err(); err != nil {
			return total, err
		}
	}
}

// PurgeArgs 是週期清除的 job。
type PurgeArgs struct{}

func (PurgeArgs) Kind() string { return "procurement.run_event_purge" }

type purgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	purger Purger
}

func (w *purgeWorker) Work(ctx context.Context, _ *river.Job[PurgeArgs]) error {
	_, err := w.purger.Purge(ctx)
	return err
}

func (w *purgeWorker) Timeout(*river.Job[PurgeArgs]) time.Duration { return 5 * time.Minute }
