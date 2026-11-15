// Package retention 是內容快照的保留期（A7-D20）：結案滿 model.SnapshotRetention 之後由排程 job 刪除快照；檢舉的 snapshot_id 由 ON DELETE SET NULL 清空，
// 案件與檢舉列留著、不含內容。進行中的案件（沒有結案時間）的快照不刪。
package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// batchSize 是每個 statement 刪除的快照數：job 分批刪，不在一個 statement 持有大量列鎖。
const batchSize = 500

// Deps 是保留期清理的依賴。
type Deps struct {
	DB  *db.DB
	Now func() time.Time
}

// Service 是保留期清理的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// PurgeSnapshots 刪除結案超過保留期的案件的快照，回傳刪除的快照數；冪等。
func (s *Service) PurgeSnapshots(ctx context.Context) (int64, error) {
	cutoff := s.d.Now().UTC().Add(-model.SnapshotRetention)
	q := store.New(s.d.DB.Conn(ctx))
	var total int64
	for {
		n, err := q.PurgeSnapshotsBatch(ctx, store.PurgeSnapshotsBatchParams{Cutoff: &cutoff, MaxRows: batchSize})
		if err != nil {
			return total, fmt.Errorf("moderation：清除過期的快照：%w", err)
		}
		total += n
		if n < batchSize {
			return total, nil
		}
	}
}

// PurgeSnapshotsArgs 是每日清除過期快照的 River job。
type PurgeSnapshotsArgs struct{}

// Kind 是 River 的 job 種類。
func (PurgeSnapshotsArgs) Kind() string { return "moderation.purge_snapshots" }

// InsertOpts 讓同一個時段內的排程與重啟合併。
func (PurgeSnapshotsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 12 * time.Hour}}
}

type purgeWorker struct {
	river.WorkerDefaults[PurgeSnapshotsArgs]
	svc *Service
}

func (w *purgeWorker) Work(ctx context.Context, _ *river.Job[PurgeSnapshotsArgs]) error {
	_, err := w.svc.PurgeSnapshots(ctx)
	return err
}

func (w *purgeWorker) Timeout(*river.Job[PurgeSnapshotsArgs]) time.Duration { return 30 * time.Minute }

// AddWorkers 登記清除的 worker。
func AddWorkers(workers *river.Workers, s *Service) { river.AddWorker(workers, &purgeWorker{svc: s}) }

// PeriodicJobs 是每日清除過期快照（啟動時也跑一次）。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return PurgeSnapshotsArgs{}, nil },
		&river.PeriodicJobOpts{ID: "moderation.purge_snapshots", RunOnStart: true},
	)}
}
