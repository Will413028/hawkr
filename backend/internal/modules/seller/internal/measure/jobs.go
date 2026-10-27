package measure

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// CloseEndedArgs 是措施的結案掃描：把已結束的措施上失去標的的待處理解除請求結案。
type CloseEndedArgs struct{}

// Kind 是 River 的 job 種類。
func (CloseEndedArgs) Kind() string { return "seller.measure_close_ended_requests" }

// InsertOpts 把同一個週期的排程與重啟合併。
func (CloseEndedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: closeEndedInterval}}
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

// closeEndedOf 在帳號已鎖住的 transaction 內，把帳號已結束的措施上待處理的解除請求結案並讓那些措施的後台版本加一。
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
}

// PeriodicJobs 回傳啟動即執行、每 5 分鐘一次的結案掃描。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(closeEndedInterval),
		func() (river.JobArgs, *river.InsertOpts) { return CloseEndedArgs{}, nil },
		&river.PeriodicJobOpts{ID: "seller.measure_close_ended_requests", RunOnStart: true},
	)}
}
