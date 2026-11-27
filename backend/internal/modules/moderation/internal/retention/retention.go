// Package retention 是檢舉資料的保留期（A7-D20、A7-D21）：結案滿 model.SnapshotRetention 之後由排程 job 刪除內容快照（快照的圖片複本是 moderation 自己 Confirm 的資產，由擁有者的刪除不會 MarkDeleted，只有這裡在快照過保留期時 MarkDeleted，B1-D20）並清除檢舉的說明文字與冪等紀錄（request_hash 是說明的雜湊）；
// 檢舉人刪帳時（買家的刪除事件）或店家退場時（seller.StoreRetired），同 transaction 清掉他（該店）全部檢舉的說明、冪等紀錄與每日計數列。檢舉的 snapshot_id 由 ON DELETE SET NULL 清空，案件與檢舉列
// （含 reporter_buyer_id、reporter_store_id，業務鍵）留著。進行中的案件（沒有結案時間）的快照與說明不清。
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
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const batchSize = 500

// Deps 是保留期清理的依賴，全部必填（Logger 可為空）。
type Deps struct {
	DB     *db.DB
	Buyer  *buyer.Module
	Media  Media
	Now    func() time.Time
	Logger *slog.Logger
}

// Media 是保留期清理對 media 的依賴（*media.Module）：刪除過期快照時把它擁有的圖片複本 MarkDeleted，ctx 必須帶 transaction。
type Media interface {
	MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error
}

// Service 是保留期清理與檢舉人資料清除的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// PurgeSnapshots 刪除結案超過保留期的案件的快照並清掉檢舉的說明與冪等紀錄，回傳刪除的快照數；冪等。
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
	if err := s.drain(ctx, "清除過期的檢舉說明", func(q *store.Queries) (int64, error) {
		return q.ClearExpiredDescriptionsBatch(ctx, store.ClearExpiredDescriptionsBatchParams{Cutoff: &cutoff, MaxRows: batchSize})
	}); err != nil {
		return total, err
	}
	return total, s.drain(ctx, "刪除過期的檢舉冪等紀錄", func(q *store.Queries) (int64, error) {
		return q.DeleteExpiredReportRequestsBatch(ctx, store.DeleteExpiredReportRequestsBatchParams{Cutoff: &cutoff, MaxRows: batchSize})
	})
}

func (s *Service) drain(ctx context.Context, what string, batch func(q *store.Queries) (int64, error)) error {
	for {
		var n int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			n, err = batch(store.New(s.d.DB.Conn(ctx)))
			return err
		})
		if err != nil {
			return fmt.Errorf("moderation：%s：%w", what, err)
		}
		if n < batchSize {
			return nil
		}
	}
}

func (s *Service) purgeBatch(ctx context.Context, cutoff time.Time) (int64, error) {
	var deleted int64
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		rows, err := q.ExpiredSnapshotsBatch(ctx, store.ExpiredSnapshotsBatchParams{Cutoff: &cutoff, MaxRows: batchSize})
		if err != nil {
			return err
		}
		owned, err := q.DeleteSnapshots(ctx, rows)
		if err != nil {
			return err
		}
		deleted = int64(len(owned))
		return s.retireAssets(ctx, owned)
	})
	if err != nil {
		return 0, fmt.Errorf("moderation：清除過期的快照：%w", err)
	}
	return deleted, nil
}

func (s *Service) retireAssets(ctx context.Context, owned [][]uuid.UUID) error {
	var assets []uuid.UUID
	for _, ids := range owned {
		for _, id := range ids {
			if !slices.Contains(assets, id) {
				assets = append(assets, id)
			}
		}
	}
	if len(assets) == 0 {
		return nil
	}
	if err := s.d.Media.MarkDeleted(ctx, assets); err != nil {
		return fmt.Errorf("moderation：標記快照圖片刪除：%w", err)
	}
	return nil
}

// OnBuyerDeletionRequested 是買家進入刪除中時同 transaction 的訂閱者（單體規則 4 的原子狀態變更）：清掉這位檢舉人（含同一帳號的歷史身分）全部檢舉的說明、冪等紀錄與每日計數列；
// 檢舉列與 reporter_buyer_id 留著。冪等。
func (s *Service) OnBuyerDeletionRequested(ctx context.Context, e buyer.BuyerDeletionRequested) error {
	return s.clearReporter(ctx, e.BuyerID, e.AccountID)
}

// OnBuyerDeleted 是買家關閉時同 transaction 的訂閱者：同 OnBuyerDeletionRequested（沒有開啟中的訂單時買家直接關閉，只發這個事件）。
func (s *Service) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	return s.clearReporter(ctx, e.BuyerID, e.AccountID)
}

// OnStoreRetired 是店家退場時同 transaction 的訂閱者：清掉該店以檢舉人身分送出的全部檢舉的說明、冪等紀錄與每日計數列（店家額度與買家分開計）；
// 檢舉列與 reporter_store_id 留著。冪等。
func (s *Service) OnStoreRetired(ctx context.Context, e seller.StoreRetired) error {
	q := store.New(s.d.DB.Conn(ctx))
	if _, err := q.ClearStoreReporterDescriptions(ctx, &e.StoreID); err != nil {
		return fmt.Errorf("moderation：清除店家檢舉人 %d 的說明：%w", e.StoreID, err)
	}
	if _, err := q.DeleteStoreReporterRequests(ctx, &e.StoreID); err != nil {
		return fmt.Errorf("moderation：刪除店家檢舉人 %d 的冪等紀錄：%w", e.StoreID, err)
	}
	if _, err := q.DeleteStoreReporterDailyCount(ctx, &e.StoreID); err != nil {
		return fmt.Errorf("moderation：刪除店家檢舉人 %d 的每日計數：%w", e.StoreID, err)
	}
	return nil
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
		if _, err := q.ClearReporterDescriptions(ctx, &id); err != nil {
			return fmt.Errorf("moderation：清除檢舉人 %d 的說明：%w", id, err)
		}
		if _, err := q.DeleteReporterRequests(ctx, &id); err != nil {
			return fmt.Errorf("moderation：刪除檢舉人 %d 的冪等紀錄：%w", id, err)
		}
		if _, err := q.DeleteReporterDailyCount(ctx, &id); err != nil {
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
	return jobs.SweepOpts(12 * time.Hour)
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
func (s *Service) AddWorkers(workers *river.Workers) { river.AddWorker(workers, &purgeWorker{svc: s}) }

// PeriodicJobs 是每日清除過期快照（啟動時也跑一次）。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return PurgeSnapshotsArgs{}, nil },
		&river.PeriodicJobOpts{ID: "moderation.purge_snapshots", RunOnStart: true},
	)}
}
