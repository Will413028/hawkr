package request

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion/internal/store"
)

const pendingBatchSize int32 = 100

// Reconcile 按 account_id 掃描待辦；單筆失敗不阻斷同批或後續批次。
func (s *Service) Reconcile(ctx context.Context) error {
	var cursor int64
	var failures []error
	for {
		accounts, err := store.New(s.d.DB.Conn(ctx)).PendingBatch(ctx, store.PendingBatchParams{AccountID: cursor, Limit: pendingBatchSize})
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, accountID := range accounts {
			if _, err := s.CompletePending(ctx, accountID); err != nil {
				failures = append(failures, fmt.Errorf("帳號 %d：%w", accountID, err))
			}
			cursor = accountID
		}
		if len(accounts) < int(pendingBatchSize) {
			return errors.Join(failures...)
		}
	}
}

type ReconcileArgs struct{}

func (ReconcileArgs) Kind() string { return "accountdeletion.reconcile" }

func (ReconcileArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: time.Hour}}
}

type reconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	service *Service
}

func (worker *reconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	return worker.service.Reconcile(ctx)
}

func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &reconcileWorker{service: s})
}

func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
		&river.PeriodicJobOpts{ID: "accountdeletion.reconcile", RunOnStart: true},
	)}
}
