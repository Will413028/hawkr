// Package restock 是補貨通知在 catalog 這一端的部分（D26）：偵測 SKU 由不可買變可買、寫進佇列表、由週期 worker 重新確認後呼叫通知 port。
//   - 可買是庫存大於 0 ∧ SKU 未刪 ∧ 商品 ACTIVE 且未刪 ∧ 店家在請求的 clock 可曝光（seller.pub_exposable_stores）；來源不限：賣家 SetStock 與 AdjustStock、歸還、改回上架。
//     店家重新曝光不觸發（Will 2026-11-27）。
//   - 偵測與排入在寫入的 transaction 內（ctx 帶 transaction 時佇列列與庫存一起 commit 或回滾）；呼叫通知 port 一律在 transaction 外，由 worker 執行（單體規則 5）。
//     這是 transactional outbox 的 polling publisher，前例是 media 的刪除請求表。
//   - 佇列表 restock_queue 一個 SKU 最多一列：沒被認領就合併（保留較早的 due_at），被認領處理中就只更新 requested_at；worker 完成時只在 requested_at 沒變才刪列，
//     變了就重新排（Rearm），所以處理中的轉換不會遺失，也不需要 River 唯一鍵或標記。
//   - worker 認領時寫租約（claimed_until），crash 後租約到期即重新處理；通知是一次性訂閱，重複呼叫是 no-op，所以重處理安全。失敗在同一列累計次數並指數退避，
//     超過 MaxAttempts 設 failed_at 停手、列保留（含 last_error），之後的新請求會重置它。
//   - 商品或 SKU 刪除經另一張同形的 restock_purge_queue 非同步清訂閱，同一套認領、完成與失敗。
package restock

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	// Delay 是請求排入後到可處理的延後時間：合併短時間內反覆的庫存變動，也讓賣家來得及改正誤操作。
	Delay = 5 * time.Minute
	// Interval 是 worker 的輪詢週期。
	Interval = 30 * time.Second
	// Lease 是認領的租約：必須長於一輪處理的時間（job 的 Timeout 較短），crash 的 worker 留下的列在它過後被重新認領。
	Lease = 5 * time.Minute
	// MaxAttempts 是失敗累計到幾次就停手。
	MaxAttempts = 5
	// BatchSize 是一輪最多認領幾列（補貨與清訂閱各自）。
	BatchSize = 100

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

// Deps 是補貨的依賴。
type Deps struct {
	DB       *db.DB
	Now      func() time.Time
	Notifier model.RestockNotifier
}

// Service 是補貨的 use case。
type Service struct{ d Deps }

// New 建立補貨 use case。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// StockIncreased 在庫存增加的寫入之後、同一個 transaction 內呼叫：added 是這次增加的量，呼叫方保證 SKU 列已被它的 UPDATE 鎖住，
// 所以此刻庫存等於 added 就表示增加前是 0；而且此刻可買才排（>0 到 >0、店家不可曝光、商品不是 ACTIVE 都不排）。
// 商品狀態與這次庫存變更並行時，兩邊 transaction 互相看不到對方；漏的那一邊由 ProductActivated 對每個 SKU 都排補上。
func (s *Service) StockIncreased(ctx context.Context, skuID uuid.UUID, added int32) error {
	if added <= 0 {
		return nil
	}
	now := s.d.Now()
	st, err := s.q(ctx).GetSkuRestockState(ctx, store.GetSkuRestockStateParams{ID: skuID, At: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.StockQty != added || !st.Purchasable {
		return nil
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		return s.q(ctx).EnqueueRestock(ctx, store.EnqueueRestockParams{SkuID: skuID, DueAt: now.Add(Delay), Now: now})
	})
}

// ProductActivated 在商品由 DRAFT 或 ARCHIVED 切成 ACTIVE 的 transaction 內呼叫：商品每個沒刪除的 SKU 都排，不看庫存。
// 不讀庫存就不會與並行的 0 到正庫存變更互相看不到而漏通知；庫存是 0 的 SKU 由 worker 執行時重新確認而不通知。
func (s *Service) ProductActivated(ctx context.Context, productID uuid.UUID) error {
	now := s.d.Now()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		return s.q(ctx).EnqueueRestockOfProduct(ctx, store.EnqueueRestockOfProductParams{ProductID: productID, DueAt: now.Add(Delay), Now: now})
	})
}

// SkusDeleted 在商品或 SKU 刪除的 transaction 內呼叫：排這些 SKU 的訂閱清除，由 worker 非同步呼叫 port。
func (s *Service) SkusDeleted(ctx context.Context, skuIDs []uuid.UUID) error {
	if len(skuIDs) == 0 {
		return nil
	}
	now := s.d.Now()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		for _, id := range skuIDs {
			if err := q.EnqueuePurge(ctx, store.EnqueuePurgeParams{SkuID: id, DueAt: now, Now: now}); err != nil {
				return err
			}
		}
		return nil
	})
}

// Run 是 worker 一輪的內容：處理到期的補貨列，再處理清訂閱列。通知 port 的失敗記在列上、不回傳；只有佇列表本身的資料庫錯誤才回 error。
func (s *Service) Run(ctx context.Context) error {
	return errors.Join(s.runRestock(ctx), s.runPurge(ctx))
}

type claimed struct {
	skuID       uuid.UUID
	requestedAt time.Time
	attempts    int32
}

func (s *Service) runRestock(ctx context.Context) error {
	now := s.d.Now()
	rows, err := s.q(ctx).ClaimRestock(ctx, store.ClaimRestockParams{Now: now, LeaseUntil: now.Add(Lease), MaxRows: BatchSize})
	if err != nil {
		return fmt.Errorf("restock：認領：%w", err)
	}
	var errs []error
	for _, r := range rows {
		c := claimed{skuID: r.SkuID, requestedAt: r.RequestedAt, attempts: r.Attempts}
		if err := s.settle(ctx, c, s.notify(ctx, c.skuID), s.finishRestock, s.failRestock, s.rearmRestock); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// notify 重新確認可買、讀最新的名稱與主圖，可買才呼叫通知 port；不可買（庫存又歸零、商品下架或刪除、店家不可曝光）不通知，視為處理完。
func (s *Service) notify(ctx context.Context, skuID uuid.UUID) error {
	now := s.d.Now()
	st, err := s.q(ctx).GetSkuRestockState(ctx, store.GetSkuRestockStateParams{ID: skuID, At: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !st.Purchasable {
		return nil
	}
	return s.d.Notifier.NotifyRestocked(ctx, model.RestockedSKU{
		ProductID: st.ProductID, SKUID: skuID, StoreID: st.StoreID, Name: st.Title, ImageKey: st.CoverKey, OccurredAt: now,
	})
}

func (s *Service) runPurge(ctx context.Context) error {
	now := s.d.Now()
	rows, err := s.q(ctx).ClaimPurge(ctx, store.ClaimPurgeParams{Now: now, LeaseUntil: now.Add(Lease), MaxRows: BatchSize})
	if err != nil {
		return fmt.Errorf("restock：認領清訂閱：%w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.SkuID
	}
	callErr := s.d.Notifier.PurgeSKUSubscriptions(ctx, ids)
	var errs []error
	for _, r := range rows {
		c := claimed{skuID: r.SkuID, requestedAt: r.RequestedAt, attempts: r.Attempts}
		if err := s.settle(ctx, c, callErr, s.finishPurge, s.failPurge, s.rearmPurge); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// settle 收尾一個認領的列：成功就刪（requested_at 沒變）；處理中有新請求（刪不到）就重新排；失敗就同列退避或停手。
func (s *Service) settle(ctx context.Context, c claimed, procErr error,
	finish func(context.Context, claimed) (int64, error),
	fail func(context.Context, claimed, time.Time, string, *time.Time) (int64, error),
	rearm func(context.Context, claimed, time.Time) error,
) error {
	now := s.d.Now()
	if procErr == nil {
		n, err := finish(ctx, c)
		if err != nil {
			return err
		}
		if n == 0 {
			return rearm(ctx, c, now.Add(Delay))
		}
		return nil
	}
	var failedAt *time.Time
	if c.attempts+1 >= MaxAttempts {
		failedAt = &now
	}
	n, err := fail(ctx, c, now.Add(Backoff(c.attempts+1)), procErr.Error(), failedAt)
	if err != nil {
		return err
	}
	if n == 0 {
		return rearm(ctx, c, now.Add(Delay))
	}
	return nil
}

func (s *Service) finishRestock(ctx context.Context, c claimed) (int64, error) {
	return s.q(ctx).FinishRestock(ctx, store.FinishRestockParams{SkuID: c.skuID, RequestedAt: c.requestedAt})
}

func (s *Service) failRestock(ctx context.Context, c claimed, due time.Time, msg string, failedAt *time.Time) (int64, error) {
	return s.q(ctx).FailRestock(ctx, store.FailRestockParams{SkuID: c.skuID, RequestedAt: c.requestedAt, DueAt: due, LastError: &msg, FailedAt: failedAt})
}

func (s *Service) rearmRestock(ctx context.Context, c claimed, due time.Time) error {
	return s.q(ctx).RearmRestock(ctx, store.RearmRestockParams{SkuID: c.skuID, DueAt: due})
}

func (s *Service) finishPurge(ctx context.Context, c claimed) (int64, error) {
	return s.q(ctx).FinishPurge(ctx, store.FinishPurgeParams{SkuID: c.skuID, RequestedAt: c.requestedAt})
}

func (s *Service) failPurge(ctx context.Context, c claimed, due time.Time, msg string, failedAt *time.Time) (int64, error) {
	return s.q(ctx).FailPurge(ctx, store.FailPurgeParams{SkuID: c.skuID, RequestedAt: c.requestedAt, DueAt: due, LastError: &msg, FailedAt: failedAt})
}

func (s *Service) rearmPurge(ctx context.Context, c claimed, due time.Time) error {
	return s.q(ctx).RearmPurge(ctx, store.RearmPurgeParams{SkuID: c.skuID, DueAt: due})
}

// RunArgs 是週期 job 的參數（種類 catalog.restock_run）。
type RunArgs struct{}

// Kind 是 River 的 job 種類。
func (RunArgs) Kind() string { return "catalog.restock_run" }

type runWorker struct {
	river.WorkerDefaults[RunArgs]
	svc *Service
}

func (w *runWorker) Work(ctx context.Context, _ *river.Job[RunArgs]) error { return w.svc.Run(ctx) }

func (w *runWorker) Timeout(*river.Job[RunArgs]) time.Duration { return runTimeout }

// AddWorkers 登記週期 job 的 River worker。
func AddWorkers(workers *river.Workers, s *Service) { river.AddWorker(workers, &runWorker{svc: s}) }

// PeriodicJobs 回傳每 Interval 執行一輪的排程。只試一次：下一個週期就是重試，失敗的列在表上。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(Interval),
			func() (river.JobArgs, *river.InsertOpts) { return RunArgs{}, &river.InsertOpts{MaxAttempts: 1} },
			&river.PeriodicJobOpts{ID: "catalog.restock_run", RunOnStart: true},
		),
	}
}
