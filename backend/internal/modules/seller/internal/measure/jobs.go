package measure

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// CloseEndedArgs 是措施的結案掃描：把已結束的措施上失去標的的待處理解除請求結案。
type CloseEndedArgs struct{}

// Kind 是 River 的 job 種類。
func (CloseEndedArgs) Kind() string { return "seller.measure_close_ended_requests" }

// InsertOpts 把同一個週期的排程與重啟合併。
func (CloseEndedArgs) InsertOpts() river.InsertOpts {
	return jobs.SweepOpts(closeEndedInterval)
}

const closeEndedInterval = 5 * time.Minute

// CloseEndedRequests 把已結束的措施上儲存為 PENDING 的解除請求以 CLOSED（ENDED）結案，發 MeasureRequestClosed，並讓後台的版本加一；
// 回傳結案的請求數；某個帳號失敗不影響其他帳號，錯誤合併回傳讓 river 重試。只掃已結束而仍有待處理解除請求的措施（「已結束」的定義只有 EffectiveMeasures，查詢用它的結果），每個帳號在自己的
// transaction 內先鎖帳號再重讀，所以與措施的其他寫入互斥、重跑是冪等的。這只決定通知與儲存值收斂的時效：請求對外的狀態由
// model.Measure.EffectiveRequest 推導，措施一結束輸出就是 CLOSED，SubmitAppeal 也自己先結案，job 遺失時正確性不受影響。
func (s *Service) CloseEndedRequests(ctx context.Context) (int, error) {
	accounts, err := s.accountsWithEndedRemediation(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	var errs []error
	for _, accountID := range accounts {
		closedHere := 0
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			if _, err := q.LockAccountStores(ctx, accountID); err != nil {
				return err
			}
			n, err := s.closeEndedOf(ctx, q, accountID, s.now())
			if err == nil {
				closedHere = n
			}
			return err
		})
		total += closedHere
		if err != nil {
			errs = append(errs, fmt.Errorf("account %d: %w", accountID, err))
		}
	}
	return total, errors.Join(errs...)
}

func (s *Service) accountsWithEndedRemediation(ctx context.Context) ([]int64, error) {
	var accounts []int64
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		active, err := stint.ActiveMeasureIDs(ctx, q, 0, s.now())
		if err != nil {
			return err
		}
		ended, err := q.MeasuresWithEndedRemediation(ctx, store.MeasuresWithEndedRemediationParams{ActiveIds: active})
		if err != nil {
			return err
		}
		for _, m := range ended {
			accounts = append(accounts, m.AccountID)
		}
		return nil
	})
	slices.Sort(accounts)
	return slices.Compact(accounts), err
}

func (s *Service) closeEndedOf(ctx context.Context, q *store.Queries, accountID int64, now time.Time) (int, error) {
	active, err := stint.ActiveMeasureIDs(ctx, q, accountID, now)
	if err != nil {
		return 0, err
	}
	ended, err := q.MeasuresWithEndedRemediation(ctx, store.MeasuresWithEndedRemediationParams{AccountID: &accountID, ActiveIds: active})
	if err != nil || len(ended) == 0 {
		return 0, err
	}
	closed, err := s.closeEndedRemediations(ctx, q, ended, now)
	if err != nil {
		return 0, err
	}
	for _, m := range ended {
		if !slices.ContainsFunc(closed, func(r store.SellerMeasureRequest) bool { return r.MeasureID == m.ID }) {
			continue
		}
		if _, err := q.TouchMeasure(ctx, store.TouchMeasureParams{ID: m.ID, ExpectedVersion: m.Version, Now: now}); err != nil {
			return 0, err
		}
	}
	return len(closed), nil
}

type closeEndedWorker struct {
	river.WorkerDefaults[CloseEndedArgs]
	service *Service
}

// Work 執行結案掃描。
func (w *closeEndedWorker) Work(ctx context.Context, _ *river.Job[CloseEndedArgs]) error {
	_, err := w.service.CloseEndedRequests(ctx)
	return err
}

// Timeout 是一次掃描的時間上限。
func (w *closeEndedWorker) Timeout(*river.Job[CloseEndedArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &closeEndedWorker{service: s})
	river.AddWorker(workers, &purgeContentSnapshotsWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每 5 分鐘一次的結案掃描，與啟動即執行、每日一次的過期內容快照清空。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(closeEndedInterval),
		func() (river.JobArgs, *river.InsertOpts) { return CloseEndedArgs{}, nil },
		&river.PeriodicJobOpts{ID: "seller.measure_close_ended_requests", RunOnStart: true},
	), river.NewPeriodicJob(
		river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return PurgeContentSnapshotsArgs{}, nil },
		&river.PeriodicJobOpts{ID: "seller.purge_content_snapshots", RunOnStart: true},
	)}
}

// PurgeContentSnapshotsArgs 是每日清空過期內容快照的 River job。
type PurgeContentSnapshotsArgs struct{}

// Kind 是 River 的 job 種類。
func (PurgeContentSnapshotsArgs) Kind() string { return "seller.purge_content_snapshots" }

// InsertOpts 讓同一個時段內的排程與重啟合併。
func (PurgeContentSnapshotsArgs) InsertOpts() river.InsertOpts {
	return jobs.SweepOpts(12 * time.Hour)
}

const contentSnapshotBatch = 500

// PurgeContentSnapshots 清空撤銷滿 model.ContentSnapshotRetention 的內容下架措施留著的內容快照（內容下架沒有結束時間，保留期只從 revoked_at 起算；
// 未撤銷的一直保留），回傳清空的列數；冪等。
func (s *Service) PurgeContentSnapshots(ctx context.Context) (int64, error) {
	cutoff := s.now().UTC().Add(-model.ContentSnapshotRetention)
	var total int64
	for {
		var n int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			n, err = store.New(s.d.DB.Conn(ctx)).ClearExpiredContentSnapshots(ctx, store.ClearExpiredContentSnapshotsParams{Cutoff: cutoff, MaxRows: contentSnapshotBatch})
			return err
		})
		if err != nil {
			return total, fmt.Errorf("seller：清空過期的內容快照：%w", err)
		}
		total += n
		if n < contentSnapshotBatch {
			return total, nil
		}
	}
}

type purgeContentSnapshotsWorker struct {
	river.WorkerDefaults[PurgeContentSnapshotsArgs]
	service *Service
}

// Work 執行清空。
func (w *purgeContentSnapshotsWorker) Work(ctx context.Context, _ *river.Job[PurgeContentSnapshotsArgs]) error {
	_, err := w.service.PurgeContentSnapshots(ctx)
	return err
}

// Timeout 是一次清空的時間上限。
func (w *purgeContentSnapshotsWorker) Timeout(*river.Job[PurgeContentSnapshotsArgs]) time.Duration {
	return 30 * time.Minute
}
