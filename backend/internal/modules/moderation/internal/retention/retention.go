// Package retention 是檢舉資料的保留期（A7-D20、A7-D21）：結案滿 model.SnapshotRetention 之後由排程 job 刪除內容快照（釋放快照在 media 的圖片引用）並清除檢舉的說明文字；
// 檢舉人刪帳時（買家的刪除事件，同 transaction）清掉他全部檢舉的說明與每日計數列。檢舉的 snapshot_id 由 ON DELETE SET NULL 清空，案件與檢舉列
// （含 reporter_buyer_id，假名鍵）留著。進行中的案件（沒有結案時間）的快照與說明不清。
package retention

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// batchSize 是每個 transaction 處理的快照與說明數：job 分批做，不在一個 statement 持有大量列鎖。
const batchSize = 500

// Deps 是保留期清理的依賴，全部必填（Logger 可為空）。
type Deps struct {
	DB     *db.DB
	Buyer  *buyer.Module
	Media  media.Media
	Jobs   jobs.Enqueuer
	Now    func() time.Time
	Logger *slog.Logger
}

// Service 是保留期清理與檢舉人資料清除的 use case。
type Service struct {
	d      Deps
	holds  mediaref.Holds
	syncer *media.RefSyncer
}

// New 建立服務。
func New(d Deps) *Service {
	return &Service{d: d, holds: mediaref.Holds{DB: d.DB, Jobs: d.Jobs}, syncer: mediaref.NewSyncer(d.DB, d.Media, d.Jobs, d.Now, d.Logger)}
}

// Holds 是快照的圖片引用登記，給送出檢舉的 use case 在它的 transaction 內呼叫。
func (s *Service) Holds() mediaref.Holds { return s.holds }

// Syncer 是快照圖片引用的同步（測試與營運直接跑一輪用）。
func (s *Service) Syncer() *media.RefSyncer { return s.syncer }

// PurgeSnapshots 刪除結案超過保留期的案件的快照（釋放圖片引用）並清掉檢舉的說明，回傳刪除的快照數；冪等。
func (s *Service) PurgeSnapshots(ctx context.Context) (int64, error) {
	cutoff := s.d.Now().UTC().Add(-model.SnapshotRetention)
	var total int64
	for {
		n, err := s.purgeBatch(ctx, cutoff)
		if err != nil {
			return total, err
		}
		total += n
		if n < batchSize {
			break
		}
	}
	for {
		var n int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			n, err = store.New(s.d.DB.Conn(ctx)).ClearExpiredDescriptionsBatch(ctx, store.ClearExpiredDescriptionsBatchParams{Cutoff: &cutoff, MaxRows: batchSize})
			return err
		})
		if err != nil {
			return total, fmt.Errorf("moderation：清除過期的檢舉說明：%w", err)
		}
		if n < batchSize {
			return total, nil
		}
	}
}

// purgeBatch 在一個 transaction 內鎖住一批過期的快照、把它們引用的資產記成退場並排入同步 job（解除 media 的引用），再刪除快照。
func (s *Service) purgeBatch(ctx context.Context, cutoff time.Time) (int64, error) {
	var deleted int64
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		rows, err := q.ExpiredSnapshotsBatch(ctx, store.ExpiredSnapshotsBatchParams{Cutoff: &cutoff, MaxRows: batchSize})
		if err != nil {
			return err
		}
		now := s.d.Now().UTC()
		ids := make([]uuid.UUID, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
			if err := s.holds.Release(ctx, q, r.ID, now, r.AssetIds); err != nil {
				return err
			}
		}
		deleted, err = q.DeleteSnapshots(ctx, ids)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("moderation：清除過期的快照：%w", err)
	}
	return deleted, nil
}

// OnBuyerDeletionRequested 是買家進入刪除中時同 transaction 的訂閱者（單體規則 4 的原子狀態變更）：清掉這位檢舉人（含同一帳號的歷史身分）全部檢舉的說明與每日計數列；
// 檢舉列與 reporter_buyer_id 留著。冪等。
func (s *Service) OnBuyerDeletionRequested(ctx context.Context, e buyer.BuyerDeletionRequested) error {
	return s.clearReporter(ctx, e.BuyerID, e.AccountID)
}

// OnBuyerDeleted 是買家關閉時同 transaction 的訂閱者：同 OnBuyerDeletionRequested（沒有開啟中的訂單時買家直接關閉，只發這個事件）。
func (s *Service) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	return s.clearReporter(ctx, e.BuyerID, e.AccountID)
}

func (s *Service) clearReporter(ctx context.Context, buyerID, accountID int64) error {
	ids, err := s.d.Buyer.BuyerIDsByAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if !slices.Contains(ids, buyerID) {
		ids = append(ids, buyerID)
	}
	q := store.New(s.d.DB.Conn(ctx))
	for _, id := range ids {
		if _, err := q.ClearReporterDescriptions(ctx, id); err != nil {
			return fmt.Errorf("moderation：清除檢舉人 %d 的說明：%w", id, err)
		}
		if _, err := q.DeleteReporterDailyCount(ctx, id); err != nil {
			return fmt.Errorf("moderation：刪除檢舉人 %d 的每日計數：%w", id, err)
		}
	}
	return nil
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

// AddWorkers 登記清除與快照圖片引用同步的 worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &purgeWorker{svc: s})
	mediaref.AddWorkers(workers, s.syncer)
}

// PeriodicJobs 是每日清除過期快照（啟動時也跑一次）與快照圖片引用的保底掃描。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return append([]*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return PurgeSnapshotsArgs{}, nil },
		&river.PeriodicJobOpts{ID: "moderation.purge_snapshots", RunOnStart: true},
	)}, mediaref.PeriodicJobs()...)
}
