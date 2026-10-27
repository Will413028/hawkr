package profile

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

const (
	reconcileJobID    = "seller.reconcile_exposure"
	reconcileInterval = 15 * time.Minute
	reconcileBatch    = 100
)

// ReconcileArgs 是對帳可曝光的工作。
type ReconcileArgs struct{}

// Kind 回傳 River 工作種類。
func (ReconcileArgs) Kind() string { return reconcileJobID }

// InsertOpts 將同一個週期的排程與重啟合併。
func (ReconcileArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: reconcileInterval}}
}

// ReconcileExposure 讓每家店最後公告的可曝光與現在一致，回傳掃描的店家數。它補上寫入路徑接不到的變化：時間造成的（處分
// 到期與 job 執行之間的空窗）、以及任何漏接的寫入；公告函式（exposure.Announce）與寫入路徑是同一支，所以規則只有一處。
// first_exposed_at 在公告為可曝光時一併記下，沒有別的寫入路徑。
//
// 候選是所有還沒關閉的店加上最後公告為可曝光的店（關店之後要補發 false），分批依 store_id 掃描，每批一個 transaction；任何一批
// 失敗就回錯誤，由 River 重試（已提交的批次不會重發：公告以最後公告的值為準）。
func (s *Service) ReconcileExposure(ctx context.Context) (int, error) {
	scanned := 0
	var after int64
	for {
		var ids []int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			var err error
			ids, err = q.ExposureReconcileCandidates(ctx, store.ExposureReconcileCandidatesParams{AfterStoreID: after, BatchSize: reconcileBatch})
			if err != nil || len(ids) == 0 {
				return err
			}
			return s.d.Exposure.Announce(ctx, q, s.d.Now().UTC(), ids...)
		})
		if err != nil {
			return scanned, err
		}
		scanned += len(ids)
		if len(ids) < reconcileBatch {
			return scanned, nil
		}
		after = ids[len(ids)-1]
	}
}

// ReannounceExposures 全量重新公告：每一列公告列（含已關閉的店）都以現在算出來的可曝光更新並發 StoreExposureChanged，不比對最後公告的值，
// 版本照常加一。公告函式與寫入路徑、對帳是同一支（exposure.Reannounce 與 Announce 共用計算）。分批依 store_id，每批一個 transaction；
// 任何一批失敗就回錯誤（已提交的批次已經公告，重跑會再公告一次，版本再加一，下游照常丟棄舊版本）。回傳公告的店家數。
func (s *Service) ReannounceExposures(ctx context.Context) (int, error) {
	announced := 0
	var after int64
	for {
		var ids []int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			var err error
			ids, err = q.ExposureStoreIDs(ctx, store.ExposureStoreIDsParams{AfterStoreID: after, BatchSize: reconcileBatch})
			if err != nil || len(ids) == 0 {
				return err
			}
			return s.d.Exposure.Reannounce(ctx, q, s.d.Now().UTC(), ids...)
		})
		if err != nil {
			return announced, err
		}
		announced += len(ids)
		if len(ids) < reconcileBatch {
			return announced, nil
		}
		after = ids[len(ids)-1]
	}
}

type reconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	service *Service
}

func (w *reconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	_, err := w.service.ReconcileExposure(ctx)
	return err
}

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &reconcileWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每 15 分鐘的排程。放在 profile 而不是處分的對帳：可曝光的輸入不只處分（KYC、PayUni、店名、logo
// 都是），對帳要掃全部未關閉的店；15 分鐘讓處分到期與下游隱藏商品之間的空窗很短。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(reconcileInterval),
			func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
			&river.PeriodicJobOpts{ID: reconcileJobID, RunOnStart: true},
		),
	}
}
