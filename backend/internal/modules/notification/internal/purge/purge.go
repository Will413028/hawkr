// Package purge 是 notification 的兩種清除：保留期清除（收件匣、投遞紀錄、意圖到期硬刪，D2、D15）與個資清除（身分刪除或退場後，
// 該收件人的收件匣、偏好、送出文字與意圖參數，D16）。兩者都由 job 分批做、每批一個 transaction、可重跑；同步訂閱者只排 job。
package purge

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// retentionDays 是收件匣、投遞紀錄與意圖的保留天數。
const retentionDays = 180

// batchSize 是每一批刪除或清洗的列數。
const batchSize = 500

// SweepInterval 是保留期清除的間隔。
const SweepInterval = 24 * time.Hour

// Deps 是清除的依賴。
type Deps struct {
	DB     *db.DB
	Jobs   jobs.Enqueuer
	Now    func() time.Time
	Logger *slog.Logger
}

// Service 執行保留期清除與個資清除。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service { return &Service{d: d} }

// Swept 是一次保留期清除各表刪掉的列數。
type Swept struct{ Deliveries, Intents, InboxItems int64 }

// Sweep 硬刪超過保留期的資料：先刪 created_at 過期的投遞紀錄，再刪過期且已沒有投遞紀錄的意圖（投遞紀錄還在保留期內的意圖留著，
// deliveries.intent_id 是 RESTRICT），收件匣列以 updated_at 起算。每批一個 transaction，做到沒有剩下為止。
func (s *Service) Sweep(ctx context.Context) (Swept, error) {
	cutoff := s.d.Now().UTC().Add(-retentionDays * 24 * time.Hour)
	var out Swept
	steps := []struct {
		name string
		n    *int64
		run  func(q *store.Queries) (int64, error)
	}{
		{"投遞紀錄", &out.Deliveries, func(q *store.Queries) (int64, error) {
			return q.DeleteExpiredDeliveries(ctx, store.DeleteExpiredDeliveriesParams{Cutoff: cutoff, MaxRows: batchSize})
		}},
		{"意圖", &out.Intents, func(q *store.Queries) (int64, error) {
			return q.DeleteExpiredIntents(ctx, store.DeleteExpiredIntentsParams{Cutoff: cutoff, MaxRows: batchSize})
		}},
		{"收件匣", &out.InboxItems, func(q *store.Queries) (int64, error) {
			return q.DeleteExpiredInboxItems(ctx, store.DeleteExpiredInboxItemsParams{Cutoff: cutoff, MaxRows: batchSize})
		}},
	}
	for _, step := range steps {
		n, err := s.batches(ctx, step.run)
		*step.n = n
		if err != nil {
			return out, fmt.Errorf("清除過期的%s：%w", step.name, err)
		}
	}
	return out, nil
}

// batches 反覆執行 run（每次一個 transaction）直到一批不滿 batchSize，回傳總列數。
func (s *Service) batches(ctx context.Context, run func(q *store.Queries) (int64, error)) (int64, error) {
	var total int64
	for {
		var n int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			n, err = run(store.New(s.d.DB.Conn(ctx)))
			return err
		})
		if err != nil {
			return total, err
		}
		total += n
		if n < batchSize {
			return total, nil
		}
	}
}

// SweepArgs 是保留期清除 job。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "notification.retention_sweep" }

// InsertOpts 以 SweepInterval 為唯一性的時間窗：同一個週期內的排程與重啟合併，窗過了就能再排。
func (SweepArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(SweepInterval)
	o.MaxAttempts = 3
	return o
}

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	svc *Service
}

// Work 執行保留期清除；失敗由 River 重試（冪等）。
func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	swept, err := w.svc.Sweep(ctx)
	if err != nil {
		return err
	}
	w.svc.log().InfoContext(ctx, "通知保留期清除完成", "deliveries", swept.Deliveries, "intents", swept.Intents, "inbox_items", swept.InboxItems)
	return nil
}

// Timeout 是一次清除的時間上限；超過時 job 重試，從剩下的列接著做。
func (w *sweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return 5 * time.Minute }

// PeriodicJobs 回傳啟動即執行、每 SweepInterval 的保留期清除。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(SweepInterval),
			func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
			&river.PeriodicJobOpts{ID: "notification.retention_sweep", RunOnStart: true},
		),
	}
}

func (s *Service) log() *slog.Logger {
	if s.d.Logger != nil {
		return s.d.Logger
	}
	return slog.Default()
}

// AddWorkers 登記清除的 River worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &sweepWorker{svc: s})
	river.AddWorker(workers, &recipientWorker{svc: s})
}
