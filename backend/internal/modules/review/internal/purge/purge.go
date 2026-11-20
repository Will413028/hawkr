// Package purge 是買家刪帳的評價清除。可見性在 job 跑之前已經改好：訂閱者在買家刪帳的 transaction 內把他記進 inactive_authors
// （公開可讀的述詞與統計都排除他），只排一個 job，transaction 內的工作量固定（單體規則 4）；
// 實際刪列由 job 依 id 升序分批做（刪帳者的評價筆數沒有上限）。job 冪等：述詞是「買家還有評價列」，重跑只處理剩下的。
package purge

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// batchSize 是每一批刪除的評價數。
const batchSize = 200

// sweepBatch 是保底掃描每次最多補排的買家數。
const sweepBatch = 100

// SweepInterval 是保底掃描的間隔。
const SweepInterval = 15 * time.Minute

// Stores 是讀店家（取 store_code 組快取失效的 tag）的 seller 公開 API。
type Stores interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// Deps 是清除的依賴。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Stores Stores
	NewID  func() uuid.UUID
	Now    func() time.Time
	Logger *slog.Logger
}

// Service 執行刪帳的清除。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// PurgeAuthorArgs 清除一位買家的評價。
type PurgeAuthorArgs struct {
	BuyerID int64 `json:"buyer_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (PurgeAuthorArgs) Kind() string { return "review.purge_author" }

// InsertOpts 讓同一位買家還沒開始的 job 合併（申請刪除與關閉各排一次）：唯一鍵是 BuyerID，不含執行中的狀態（job 可重跑），
// 執行中又排入的那一次排在它後面，不會被略過。買家進入刪除中之後沒有新評價：Submit 在 transaction 內持 LockAuthor 再查 inactive_authors，
// 與 Deactivate 串行，所以清除 job 看不到之後才提交的評價。
func (PurgeAuthorArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
}

// Deactivate 是買家刪帳時同 transaction 的原子狀態變更：記進 inactive_authors、排清除 job。
// 先持作者的 advisory lock，與 Submit 的 insert transaction 串行。冪等：重複呼叫（申請與關閉兩個事件）；清除 job 以買家為唯一鍵合併。必須在 transaction 內。
func (s *Service) Deactivate(ctx context.Context, buyerID int64, at time.Time) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return fmt.Errorf("review：Deactivate 必須在 transaction 內")
	}
	if err := s.q(ctx).LockAuthor(ctx, buyerID); err != nil {
		return err
	}
	if _, err := s.q(ctx).InsertInactiveAuthor(ctx, store.InsertInactiveAuthorParams{BuyerID: buyerID, Since: at.UTC()}); err != nil {
		return err
	}
	return s.enqueuePurge(ctx, buyerID)
}

func (s *Service) enqueuePurge(ctx context.Context, buyerID int64) error {
	opts := PurgeAuthorArgs{}.InsertOpts()
	return s.d.Jobs.Enqueue(ctx, PurgeAuthorArgs{BuyerID: buyerID}, &opts)
}

// Sweep 是清除的保底：job 用完 MaxAttempts 被丟棄後，買家仍在 inactive_authors 又還有評價列；
// 這裡每次最多補排 sweepBatch 位的清除 job，同一位買家還沒開始的 job 由唯一鍵合併。回傳這次檢查到的買家數。
func (s *Service) Sweep(ctx context.Context) (int, error) {
	var n int
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		ids, err := s.q(ctx).ListInactiveAuthorsWithReviews(ctx, sweepBatch)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.enqueuePurge(ctx, id); err != nil {
				return err
			}
		}
		n = len(ids)
		return nil
	})
	return n, err
}

// Purge 刪除買家的評價：依 id 升序分批，每批一個 transaction，內含刪列（附圖 CASCADE）、釋放的圖記成退場並排媒體同步、排快取失效。
// 作者此時已不可見也不計分，刪除不改任何統計，但 Java 的聚合要在刪帳當下重算（A6-D12）：每批在同 transaction 對該批每個不同的 (店, 商品)
// 發一個 ReviewAggregateChanged；重跑只處理剩下的，做完之後不再發。
func (s *Service) Purge(ctx context.Context, buyerID int64) error {
	for {
		n, err := s.purgeBatch(ctx, buyerID)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
	}
}

func (s *Service) purgeBatch(ctx context.Context, buyerID int64) (int, error) {
	n := 0
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		batch, err := q.LockBuyerReviewBatch(ctx, store.LockBuyerReviewBatchParams{BuyerID: buyerID, MaxRows: batchSize})
		if err != nil {
			return fmt.Errorf("review：鎖定買家的評價：%w", err)
		}
		if len(batch) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, len(batch))
		stores, products := make([]int64, 0, len(batch)), make([]int64, 0, len(batch))
		for i, r := range batch {
			ids[i] = r.ID
			stores, products = append(stores, r.StoreID), append(products, r.ProductID)
		}
		imageRows, err := q.ListReviewImages(ctx, ids)
		if err != nil {
			return fmt.Errorf("review：列出附圖：%w", err)
		}
		images := rows.Images(imageRows)
		if _, err := q.DeleteReviewsByID(ctx, ids); err != nil {
			return fmt.Errorf("review：刪除評價：%w", err)
		}
		now := s.d.Now().UTC()
		for _, id := range ids {
			list := images[id]
			if len(list) == 0 {
				continue
			}
			assets := make([]string, len(list))
			for i, im := range list {
				assets[i] = im.AssetID
			}
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.ReviewRef(id), assets...); err != nil {
				return err
			}
		}
		for _, t := range distinctTargets(stores, products) {
			if err := events.Publish(ctx, s.d.Events, model.ReviewAggregateChanged{EventID: s.d.NewID(), OccurredAt: now, StoreID: t.StoreID, ProductID: t.ProductID}); err != nil {
				return err
			}
		}
		if err := s.bust(ctx, stores, products); err != nil {
			return err
		}
		n = len(batch)
		return nil
	})
	return n, err
}

// distinctTargets 把一批評價的店與商品配成不重複、由小到大的 (店, 商品)。
func distinctTargets(storeIDs, productIDs []int64) []model.Target {
	out := make([]model.Target, len(storeIDs))
	for i := range storeIDs {
		out[i] = model.Target{StoreID: storeIDs[i], ProductID: productIDs[i]}
	}
	slices.SortFunc(out, func(a, b model.Target) int {
		if a.StoreID != b.StoreID {
			return cmp.Compare(a.StoreID, b.StoreID)
		}
		return cmp.Compare(a.ProductID, b.ProductID)
	})
	return slices.Compact(out)
}

// bust 排入這一批評價所在的商品頁與店面頁的快取失效（一個 job、去重、由小到大）。
func (s *Service) bust(ctx context.Context, storeIDs, productIDs []int64) error {
	slices.Sort(storeIDs)
	slices.Sort(productIDs)
	var tags []string
	for _, id := range slices.Compact(productIDs) {
		tags = append(tags, revalidate.ProductTag(id))
	}
	for _, id := range slices.Compact(storeIDs) {
		st, err := s.d.Stores.Store(ctx, id)
		switch {
		case err == nil:
			tags = append(tags, revalidate.StoreTags(st.StoreID, st.StoreCode)...)
		case apperr.KindOf(err) == apperr.KindNotFound:
		default:
			return err
		}
	}
	return s.d.Jobs.Enqueue(ctx, revalidate.Args{Tags: tags}, nil)
}

type worker struct {
	river.WorkerDefaults[PurgeAuthorArgs]
	svc *Service
}

// Work 清除買家的評價；失敗由 River 重試（冪等）。
func (w *worker) Work(ctx context.Context, job *river.Job[PurgeAuthorArgs]) error {
	return w.svc.Purge(ctx, job.Args.BuyerID)
}

// Timeout 是一次清除的時間上限；超過時 job 重試，從剩下的列接著做。
func (w *worker) Timeout(*river.Job[PurgeAuthorArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記清除的 River worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &worker{svc: s})
	river.AddWorker(workers, &sweepWorker{svc: s})
}

// SweepArgs 是清除的保底掃描。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "review.purge_sweep" }

// InsertOpts 讓同時只有一個掃描排隊。
func (SweepArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	svc *Service
}

// Work 執行保底掃描。
func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	_, err := w.svc.Sweep(ctx)
	return err
}

// Timeout 是一次掃描的時間上限。
func (w *sweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return time.Minute }

// PeriodicJobs 回傳啟動即執行、每 SweepInterval 的清除保底掃描。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(SweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "review.purge_sweep", RunOnStart: true},
	)}
}
