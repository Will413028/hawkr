// Package purge 是買家刪帳的評價清除。可見性在 job 跑之前已經改好：訂閱者在買家刪帳的 transaction 內把他記進 inactive_authors
// （公開可讀的述詞與統計都排除他），只排一個 job 並發一個 ReviewAuthorDeactivated，transaction 內的工作量固定（單體規則 4）；
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

// InsertOpts 讓同一位買家還沒結束的 job 合併（申請刪除與關閉各排一次）：唯一鍵是 BuyerID，含執行中與重試中的狀態。
// 執行中被略過的那一次不會漏：買家進入刪除中之後沒有新評價（Submit 查 inactive_authors），執行中的 job 會處理完剩下的。
func (PurgeAuthorArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
}

// Deactivate 是買家刪帳時同 transaction 的原子狀態變更：記進 inactive_authors、排清除 job，第一次寫入時發一個 ReviewAuthorDeactivated。
// 冪等：重複呼叫（申請與關閉兩個事件）不重發事件；清除 job 以買家為唯一鍵合併。必須在 transaction 內。
func (s *Service) Deactivate(ctx context.Context, buyerID int64, at time.Time) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return fmt.Errorf("review：Deactivate 必須在 transaction 內")
	}
	n, err := s.q(ctx).InsertInactiveAuthor(ctx, store.InsertInactiveAuthorParams{BuyerID: buyerID, Since: at.UTC()})
	if err != nil {
		return err
	}
	opts := PurgeAuthorArgs{}.InsertOpts()
	if err := s.d.Jobs.Enqueue(ctx, PurgeAuthorArgs{BuyerID: buyerID}, &opts); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	return events.Publish(ctx, s.d.Events, model.ReviewAuthorDeactivated{EventID: s.d.NewID(), OccurredAt: at.UTC(), BuyerID: buyerID})
}

// Targets 列出買家評過的 (店, 商品)（含隱藏的），給 relay 在 transaction 外逐組重送聚合。
func (s *Service) Targets(ctx context.Context, buyerID int64) ([]model.Target, error) {
	list, err := s.q(ctx).ListBuyerReviewTargets(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Target, len(list))
	for i, r := range list {
		out[i] = model.Target{StoreID: r.StoreID, ProductID: r.ProductID}
	}
	return out, nil
}

// Purge 刪除買家的評價：依 id 升序分批，每批一個 transaction，內含刪列（附圖 CASCADE）、釋放的圖記成退場並排媒體同步、排快取失效。
// 作者此時已不可見也不計分，所以刪除不改任何統計；重跑只處理剩下的。
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
		if err := s.bust(ctx, stores, products); err != nil {
			return err
		}
		n = len(batch)
		return nil
	})
	return n, err
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
func AddWorkers(workers *river.Workers, s *Service) { river.AddWorker(workers, &worker{svc: s}) }
