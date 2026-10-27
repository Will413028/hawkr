package vendorportal

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/agent/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/agent/internal/store"
)

// 清理（舊實作的 D8：三張表沒有清理、email 永久保存）：絕對期限過了超過 7 天的 session；到期或撤銷超過 90 天的邀請，連同
// invited_email（個資）一起刪，底下的 session 由 ON DELETE CASCADE 帶走。分批刪，每批一個 transaction，刪到取不滿為止；
// 重跑是 no-op。
const (
	purgeBatch int32 = 500
	jobID            = "agent.purge_vendor_portal"
)

// Purge 執行清理，回傳刪除的列數。
func (s *Service) Purge(ctx context.Context) (model.PurgeResult, error) {
	var out model.PurgeResult
	now := s.d.Now().UTC()
	for {
		n, err := s.batch(ctx, func(q *store.Queries) (int64, error) {
			return q.PurgeSessions(ctx, store.PurgeSessionsParams{Cutoff: now.Add(-model.SessionRetention), BatchLimit: purgeBatch})
		})
		out.Sessions += n
		if err != nil {
			return out, err
		}
		if n < int64(purgeBatch) {
			break
		}
	}
	for {
		n, err := s.batch(ctx, func(q *store.Queries) (int64, error) {
			return q.PurgeInvites(ctx, store.PurgeInvitesParams{Cutoff: now.Add(-model.InviteRetention), BatchLimit: purgeBatch})
		})
		out.Invites += n
		if err != nil {
			return out, err
		}
		if n < int64(purgeBatch) {
			return out, nil
		}
	}
}

func (s *Service) batch(ctx context.Context, fn func(*store.Queries) (int64, error)) (int64, error) {
	var n int64
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
		n, err = fn(store.New(s.d.DB.Conn(ctx)))
		return err
	})
	return n, err
}

// Args 是清理的工作。
type Args struct{}

// Kind 回傳 River 工作種類。
func (Args) Kind() string { return jobID }

// InsertOpts 將同一天的排程與重啟合併。
func (Args) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 24 * time.Hour}}
}

type worker struct {
	river.WorkerDefaults[Args]
	service *Service
}

// Work 執行清理；每批各一個 transaction，失敗由 River 重試（已刪除的列不會再被選中）。
func (w *worker) Work(ctx context.Context, _ *river.Job[Args]) error {
	_, err := w.service.Purge(ctx)
	return err
}

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &worker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每天補跑的清理排程。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(24*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return Args{}, nil },
			&river.PeriodicJobOpts{ID: jobID, RunOnStart: true},
		),
	}
}
