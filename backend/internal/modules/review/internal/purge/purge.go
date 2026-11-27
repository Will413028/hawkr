// Package purge 是買家刪帳的評價清除。可見性在 job 跑之前已經改好：公開可讀的述詞與統計 join buyer.pub_inactive_buyers()，
// 買家刪帳的 transaction 一提交評價就不可讀；訂閱者只排一個 job，transaction 內的工作量固定（單體規則 4）；
// 實際刪列由 job 依 id 升序分批做（刪帳者的評價筆數沒有上限）。job 冪等：述詞是「買家還有評價列」，重跑只處理剩下的。
package purge

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
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
	Jobs   jobs.Enqueuer
	Media  model.Media
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

// InsertOpts 讓同一位買家還沒開始的 job 合併（申請刪除與關閉各排一次）：唯一鍵是 BuyerID，含執行中的狀態（River 要求 available、pending、running、scheduled 都在內）：執行中又排入的那一次併入它：
// 該身分進入刪除後不再有新資料，執行中的 job 已涵蓋全部。買家進入刪除中之後沒有新評價：Submit 在 transaction 內先取 buyer.LockActive（買家列的 FOR KEY SHARE），
// 與刪帳轉移的 FOR UPDATE 互斥，所以清除 job 看不到之後才提交的評價。
func (PurgeAuthorArgs) InsertOpts() river.InsertOpts {
	return jobs.UniqueByArgs(10)
}

// EnqueuePurge 在買家刪帳（申請與關閉）時排清除 job：只排 job，不要求 transaction（訂閱者在刪帳的 transaction 內呼叫時併入它）。
// 冪等：清除 job 以買家為唯一鍵合併。可見性不在這裡改，由公開述詞的 pub_inactive_buyers join 提供。
func (s *Service) EnqueuePurge(ctx context.Context, buyerID int64) error {
	if buyerID <= 0 {
		return fmt.Errorf("review：停用的買家 %d 不合法", buyerID)
	}
	return s.enqueuePurge(ctx, buyerID)
}

func (s *Service) enqueuePurge(ctx context.Context, buyerID int64) error {
	opts := PurgeAuthorArgs{}.InsertOpts()
	return s.d.Jobs.Enqueue(ctx, PurgeAuthorArgs{BuyerID: buyerID}, &opts)
}

// Sweep 是清除的保底：job 用完 MaxAttempts 被丟棄後，買家已不營運（pub_inactive_buyers）又還有評價列；
// 以 buyer_id 的游標逐頁（每頁 sweepBatch 位、每頁一個 transaction）走完全部，同一位買家還沒開始的 job 由唯一鍵合併；
// 每頁固定上限，前面的買家一直失敗也不會餓死後面的。回傳這次檢查到的買家數。
func (s *Service) Sweep(ctx context.Context) (int, error) {
	total := 0
	after := int64(0)
	for {
		var ids []int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			ids, err = s.q(ctx).ListInactiveBuyersWithReviews(ctx, store.ListInactiveBuyersWithReviewsParams{AfterID: after, MaxRows: sweepBatch})
			if err != nil {
				return err
			}
			for _, id := range ids {
				if err := s.enqueuePurge(ctx, id); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return total, err
		}
		total += len(ids)
		if len(ids) < sweepBatch {
			return total, nil
		}
		after = ids[len(ids)-1]
	}
}

// Purge 刪除買家的評價：依 id 升序分批，每批一個 transaction，內含刪列（附圖 CASCADE）、釋放的圖記成退場並排媒體同步、排快取失效。
// 作者此時已不可見也不計分，刪除前已不計入公開統計，但 catalog 的評分同步仍在此依商品 ID 升序寫入最新絕對值（可重跑）。
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
		stores, products := make([]int64, 0, len(batch)), make([]uuid.UUID, 0, len(batch))
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
		var assets []uuid.UUID
		for _, id := range ids {
			for _, im := range images[id] {
				assets = append(assets, im.AssetID)
			}
		}
		if err := s.d.Media.MarkDeleted(ctx, assets); err != nil {
			return err
		}
		if err := rows.RequestRatingRecompute(ctx, q, s.d.Now().UTC(), distinctProductIDs(products)...); err != nil {
			return err
		}
		if err := s.bust(ctx, stores, products); err != nil {
			return err
		}
		n = len(batch)
		return nil
	})
	return n, err
}

// distinctProductIDs 把一批評價的商品去重並依 UUID 升序排列（鎖順序：同層多列依 ID 升序）。
func distinctProductIDs(productIDs []uuid.UUID) []uuid.UUID {
	out := slices.Clone(productIDs)
	slices.SortFunc(out, uuid.UUID.Compare)
	return slices.Compact(out)
}

// bust 排入這一批評價所在的商品頁與店面頁的快取失效（一個 job、去重、由小到大）。
func (s *Service) bust(ctx context.Context, storeIDs []int64, productIDs []uuid.UUID) error {
	slices.Sort(storeIDs)
	slices.SortFunc(productIDs, uuid.UUID.Compare)
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

// InsertOpts 以 SweepInterval 為唯一性的時間窗：同一個週期內的排程與重啟合併，窗過了就能再排。
func (SweepArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(SweepInterval)
	o.MaxAttempts = 1
	return o
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
