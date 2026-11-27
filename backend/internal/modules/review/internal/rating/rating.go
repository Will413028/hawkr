// Package rating 是商品評分的非同步投影（D3）：評分是 review 擁有的衍生統計，由週期 worker 重算後以絕對值寫進 catalog。
//   - 評價集合改變的 transaction（提交、隱藏、還原、刪帳清除）只在同 transaction 登記商品到 rating_recompute_queue，不碰 catalog：
//     review 鎖了自己的列之後不能再等 catalog 的列（platform/lockorder，catalog 排在 review 之前）。
//   - worker 以 SKIP LOCKED 認領到期列並寫租約（認領是獨立的 transaction），每個商品一個 transaction：先鎖 catalog 商品列，再無鎖讀統計，
//     與目前的評分不同才 SetRating 並排商品頁快取失效（失效在寫入之後，同 transaction，commit 才可見），最後只在 seq 沒變時刪列。
//     seq 變了表示處理中又有新請求（統計可能已過時）：重排而不是刪，所以處理中到的請求不會遺失。
//   - 失敗在同一列累計次數並指數退避，超過 MaxAttempts 設 failed_at 停手、列保留（含 last_error），之後的新請求會重置它。商品不存在不重試：刪列。
//   - 週期對帳把每個有評價的商品登記一次，補買家停用或復用等不經評價寫入的統計變動（統計 join buyer.pub_inactive_buyers()）。
package rating

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

const (
	// Interval 是 worker 的輪詢週期。
	Interval = 30 * time.Second
	// ReconcileInterval 是全量對帳的週期：不經評價寫入的變動（買家停用或復用）最久這麼久後被補正；
	// 對帳只登記，統計沒變的商品不寫 catalog、不排快取失效，所以一小時一次的成本是每個有評價的商品一次讀。
	ReconcileInterval = time.Hour
	// Lease 是認領的租約：必須長於一輪處理的時間（job 的 Timeout 較短），crash 的 worker 留下的列在它過後被重新認領。
	Lease = 5 * time.Minute
	// MaxAttempts 是失敗累計到幾次就停手。
	MaxAttempts = 5
	// BatchSize 是一輪最多認領幾列。
	BatchSize = 100
	// ReconcileBatch 是對帳每個 transaction 登記幾個商品。
	ReconcileBatch = 500

	backoffBase = time.Minute
	backoffMax  = time.Hour
	runTimeout  = 4 * time.Minute
)

// Backoff 是第 attempts 次失敗（從 1 起）之後到下一次嘗試的等待：一分鐘起倍增，上限一小時。
func Backoff(attempts int32) time.Duration {
	d := backoffBase
	for i := int32(1); i < attempts && d < backoffMax; i++ {
		d *= 2
	}
	return min(d, backoffMax)
}

// Deps 是評分投影的依賴。
type Deps struct {
	DB      *db.DB
	Jobs    jobs.Enqueuer
	Catalog model.Catalog
	Now     func() time.Time
	Logger  *slog.Logger
}

// Service 是評分投影的 use case。
type Service struct{ d Deps }

// New 建立評分投影。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

type claimed struct {
	productID uuid.UUID
	seq       int64
	attempts  int32
}

// Run 是 worker 一輪的內容：認領到期的列、逐商品處理。商品的失敗記在列上、不回傳；只有佇列表本身的資料庫錯誤才回 error。
func (s *Service) Run(ctx context.Context) error {
	now := s.d.Now()
	rows, err := s.q(ctx).ClaimRatingRecompute(ctx, store.ClaimRatingRecomputeParams{Now: now, LeaseUntil: now.Add(Lease), MaxRows: BatchSize})
	if err != nil {
		return fmt.Errorf("review：認領評分重算：%w", err)
	}
	var errs []error
	for _, r := range rows {
		c := claimed{productID: r.ProductID, seq: r.Seq, attempts: r.Attempts}
		if err := s.process(ctx, c); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// process 處理一個認領的商品：成功（含商品不存在）就在同 transaction 內收尾；失敗回滾後在同列記失敗。
func (s *Service) process(ctx context.Context, c claimed) error {
	procErr := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.project(ctx, c.productID); err != nil {
			if !errors.Is(err, catalog.ErrProductNotFound) {
				return err
			}
			s.d.Logger.WarnContext(ctx, "review：評分重算的商品不存在，略過", "product_id", c.productID)
		}
		return s.settle(ctx, c)
	})
	if procErr == nil {
		return nil
	}
	return s.fail(ctx, c, procErr)
}

// project 在一個 transaction 內重算一個商品：先鎖 catalog 商品列（下層，之後讀 review 不再往下層等鎖），無鎖讀統計，與目前的不同才寫入並清快取。
func (s *Service) project(ctx context.Context, productID uuid.UUID) error {
	cur, err := s.d.Catalog.LockProduct(ctx, productID)
	if err != nil {
		return err
	}
	st, err := s.q(ctx).ProductStats(ctx, productID)
	if err != nil {
		return err
	}
	count := int32(st.TotalCount) //nolint:gosec // 單一商品的評價則數不會超過 int32 上限
	if cur.Average == st.AvgRating && cur.Count == count {
		return nil
	}
	if err := s.d.Catalog.SetRating(ctx, productID, st.AvgRating, count); err != nil {
		return err
	}
	return s.d.Jobs.Enqueue(ctx, revalidate.Args{Tags: []string{revalidate.ProductTag(productID)}}, nil)
}

// settle 在處理的 transaction 內收尾：seq 沒變就刪列；刪不到（處理中又有新請求）就重排。
func (s *Service) settle(ctx context.Context, c claimed) error {
	n, err := s.q(ctx).FinishRatingRecompute(ctx, store.FinishRatingRecomputeParams{ProductID: c.productID, Seq: c.seq})
	if err != nil {
		return err
	}
	if n == 0 {
		return s.q(ctx).RearmRatingRecompute(ctx, store.RearmRatingRecomputeParams{ProductID: c.productID, DueAt: s.d.Now()})
	}
	return nil
}

// fail 在處理的 transaction 之外記失敗：同列次數加一、退避，達上限停手；seq 變了（刪不到）改為重排。
func (s *Service) fail(ctx context.Context, c claimed, procErr error) error {
	now := s.d.Now()
	var failedAt *time.Time
	if c.attempts+1 >= MaxAttempts {
		failedAt = &now
	}
	msg := procErr.Error()
	q := s.q(ctx)
	n, err := q.FailRatingRecompute(ctx, store.FailRatingRecomputeParams{
		ProductID: c.productID, Seq: c.seq, DueAt: now.Add(Backoff(c.attempts + 1)), LastError: &msg, FailedAt: failedAt,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return q.RearmRatingRecompute(ctx, store.RearmRatingRecomputeParams{ProductID: c.productID, DueAt: now})
	}
	s.d.Logger.WarnContext(ctx, "review：評分重算失敗", "product_id", c.productID, "attempts", c.attempts+1, "error", procErr)
	return nil
}

// Reconcile 把每個有評價的商品登記一次（product_id 游標分批，每批一個 transaction）；回傳登記的商品數。
func (s *Service) Reconcile(ctx context.Context) (int, error) {
	total := 0
	after := uuid.Nil()
	for {
		var ids []uuid.UUID
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			ids, err = s.q(ctx).RequestRatingReconcileBatch(ctx, store.RequestRatingReconcileBatchParams{AfterID: after, MaxRows: ReconcileBatch, Now: s.d.Now()})
			return err
		})
		if err != nil {
			return total, fmt.Errorf("review：評分對帳：%w", err)
		}
		total += len(ids)
		if len(ids) < ReconcileBatch {
			return total, nil
		}
		after = ids[len(ids)-1]
	}
}

// RunArgs 是週期 job 的參數（種類 review.rating_run）。
type RunArgs struct{}

// Kind 是 River 的 job 種類。
func (RunArgs) Kind() string { return "review.rating_run" }

type runWorker struct {
	river.WorkerDefaults[RunArgs]
	svc *Service
}

func (w *runWorker) Work(ctx context.Context, _ *river.Job[RunArgs]) error { return w.svc.Run(ctx) }

func (w *runWorker) Timeout(*river.Job[RunArgs]) time.Duration { return runTimeout }

// ReconcileArgs 是全量對帳的參數（種類 review.rating_reconcile）。
type ReconcileArgs struct{}

// Kind 是 River 的 job 種類。
func (ReconcileArgs) Kind() string { return "review.rating_reconcile" }

// InsertOpts 以 ReconcileInterval 為唯一性的時間窗：同一個週期內的排程與重啟合併。
func (ReconcileArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(ReconcileInterval)
	o.MaxAttempts = 1
	return o
}

type reconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	svc *Service
}

func (w *reconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	_, err := w.svc.Reconcile(ctx)
	return err
}

func (w *reconcileWorker) Timeout(*river.Job[ReconcileArgs]) time.Duration { return 10 * time.Minute }

// AddWorkers 登記週期 job 的 River worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &runWorker{svc: s})
	river.AddWorker(workers, &reconcileWorker{svc: s})
}

// PeriodicJobs 回傳每 Interval 處理一輪待重算列、每 ReconcileInterval 全量對帳的排程。處理只試一次：下一個週期就是重試，失敗的列在表上。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(Interval),
			func() (river.JobArgs, *river.InsertOpts) { return RunArgs{}, &river.InsertOpts{MaxAttempts: 1} },
			&river.PeriodicJobOpts{ID: "review.rating_run", RunOnStart: true},
		),
		river.NewPeriodicJob(
			river.PeriodicInterval(ReconcileInterval),
			func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
			&river.PeriodicJobOpts{ID: "review.rating_reconcile", RunOnStart: true},
		),
	}
}
