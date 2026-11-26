// Package restock 是補貨通知在 catalog 這一端的部分（D26）：偵測 SKU 由不可買變可買、排 job、job 執行時重新確認並呼叫通知 port。
//   - 可買是庫存大於 0 ∧ SKU 未刪 ∧ 商品 ACTIVE 且未刪 ∧ 店家在請求的 clock 可曝光（seller.pub_exposable_stores）；來源不限：賣家 SetStock 與 AdjustStock、歸還、
//     改回上架、店家重新曝光。
//   - 偵測與排入在寫入的 transaction 內（ctx 帶 transaction 時 job 與庫存一起 commit 或回滾）；呼叫通知 port 一律在 transaction 外，由 job 執行（單體規則 5）。
//   - SKU 的 job 延後 Delay 才執行，只合併還沒開始的 job（D26）：合併靠 catalog.restock_pending 的標記，不靠 River 的唯一鍵（River 不允許把 running 排除在唯一鍵之外，
//     執行中的 job 會吞掉之後的排入）。排入時占標記，新占到才排 job，已有標記就鎖住它並合併；job 開始時第一件事消耗標記，之後的轉換重新占標記、排自己的 job，所以沒有窗口。
//     店家曝光只排一個店家層的 job（同樣的標記，key 是店家），由它在 transaction 外展開成該店家可買 SKU 的 job。
//   - job 執行時重新確認可買並讀最新的名稱與主圖，不可買就結束、不呼叫通知；通知是一次性訂閱，重複呼叫是 no-op，所以 job 重試安全。
package restock

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	// Delay 是 SKU 與店家層 job 排入後延後執行的時間：合併短時間內反覆的庫存變動，也讓賣家來得及改正誤操作。
	Delay = 5 * time.Minute

	maxAttempts = 5
	storePage   = 500

	// requestAttempts 是占標記時遇到標記被並行消耗的重試上限。
	requestAttempts = 5
)

// Deps 是補貨的依賴。
type Deps struct {
	DB       *db.DB
	Jobs     jobs.Enqueuer
	Now      func() time.Time
	Notifier model.RestockNotifier
}

// Service 是補貨的 use case。
type Service struct{ d Deps }

// New 建立補貨 use case。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// SkuArgs 是單一 SKU 的補貨確認 job（種類 catalog.restock_sku）。
type SkuArgs struct {
	SkuID uuid.UUID `json:"sku_id"`
}

// Kind 是 River 的 job 種類。
func (SkuArgs) Kind() string { return "catalog.restock_sku" }

// StoreArgs 是店家重新曝光後展開成各 SKU 的 job（種類 catalog.restock_store）。
type StoreArgs struct {
	StoreID int64 `json:"store_id"`
}

// Kind 是 River 的 job 種類。
func (StoreArgs) Kind() string { return "catalog.restock_store" }

// PurgeArgs 是商品或 SKU 刪除後清補貨訂閱的 job（種類 catalog.restock_purge）。清除是冪等的，不設唯一鍵。
type PurgeArgs struct {
	SkuIDs []uuid.UUID `json:"sku_ids"`
}

// Kind 是 River 的 job 種類。
func (PurgeArgs) Kind() string { return "catalog.restock_purge" }

func (s *Service) opts(delay time.Duration) *river.InsertOpts {
	o := river.InsertOpts{MaxAttempts: maxAttempts}
	if delay > 0 {
		o.ScheduledAt = s.d.Now().Add(delay)
	}
	return &o
}

func skuKey(id uuid.UUID) string { return "sku:" + id.String() }

func storeKey(id int64) string { return "store:" + strconv.FormatInt(id, 10) }

// request 占 key 的待處理標記並在新占到時排 job。標記已存在就更新它（鎖住該列，見 TouchRestockPending）並合併進還沒開始的 job；
// 更新時標記剛好被消耗（job 開始了）就重新占。
func (s *Service) request(ctx context.Context, key string, args river.JobArgs, delay time.Duration) error {
	q := s.q(ctx)
	for range requestAttempts {
		at := s.d.Now()
		inserted, err := q.InsertRestockPending(ctx, store.InsertRestockPendingParams{JobKey: key, RequestedAt: at})
		if err != nil {
			return err
		}
		if inserted == 1 {
			return s.d.Jobs.Enqueue(ctx, args, s.opts(delay))
		}
		touched, err := q.TouchRestockPending(ctx, store.TouchRestockPendingParams{JobKey: key, RequestedAt: at})
		if err != nil {
			return err
		}
		if touched == 1 {
			return nil
		}
	}
	return fmt.Errorf("restock：%s 的待處理標記一直被並行消耗", key)
}

func (s *Service) enqueueSku(ctx context.Context, skuID uuid.UUID, delay time.Duration) error {
	return s.request(ctx, skuKey(skuID), SkuArgs{SkuID: skuID}, delay)
}

// StockIncreased 在庫存增加的寫入之後、同一個 transaction 內呼叫：added 是這次增加的量，呼叫方保證 SKU 列已被它的 UPDATE 鎖住，
// 所以此刻庫存等於 added 就表示增加前是 0；而且此刻可買才排 job（>0 到 >0、店家不可曝光、商品不是 ACTIVE 都不排）。
func (s *Service) StockIncreased(ctx context.Context, skuID uuid.UUID, added int32) error {
	if added <= 0 {
		return nil
	}
	st, err := s.q(ctx).GetSkuRestockState(ctx, store.GetSkuRestockStateParams{ID: skuID, At: s.d.Now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.StockQty != added || !st.Purchasable {
		return nil
	}
	return s.enqueueSku(ctx, skuID, Delay)
}

// ProductActivated 在商品由 DRAFT 或 ARCHIVED 切成 ACTIVE 的 transaction 內呼叫：切換前它的 SKU 都不可買，所以現在可買的每個 SKU 都是新的轉換。
func (s *Service) ProductActivated(ctx context.Context, productID uuid.UUID) error {
	ids, err := s.q(ctx).ListPurchasableSkusOfProduct(ctx, store.ListPurchasableSkusOfProductParams{ProductID: productID, At: s.d.Now()})
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.enqueueSku(ctx, id, Delay); err != nil {
			return err
		}
	}
	return nil
}

// StoreExposed 在店家變成可曝光（seller.StoreExposureChanged）的 transaction 內呼叫：只排一個店家層的 job，不在 transaction 內展開（店家的 SKU 可能很多）。
func (s *Service) StoreExposed(ctx context.Context, storeID int64) error {
	return s.request(ctx, storeKey(storeID), StoreArgs{StoreID: storeID}, Delay)
}

// SkusDeleted 在商品或 SKU 刪除的 transaction 內呼叫：排一個 job 非同步清這些 SKU 的訂閱。
func (s *Service) SkusDeleted(ctx context.Context, skuIDs []uuid.UUID) error {
	if len(skuIDs) == 0 {
		return nil
	}
	return s.d.Jobs.Enqueue(ctx, PurgeArgs{SkuIDs: skuIDs}, &river.InsertOpts{MaxAttempts: maxAttempts})
}

// RunSku 是 SKU job 的內容：重新確認可買，可買才以最新的名稱與主圖呼叫通知 port；不可買（庫存又歸零、商品下架或刪除、店家不可曝光）直接結束。
func (s *Service) RunSku(ctx context.Context, skuID uuid.UUID) error {
	// 先消耗標記再讀狀態：此後 commit 的轉換排自己的 job；標記上有尚未 commit 的合併時，這裡等它 commit，下面讀到的狀態就含它。
	if err := s.q(ctx).ConsumeRestockPending(ctx, skuKey(skuID)); err != nil {
		return err
	}
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

// RunStore 是店家層 job 的內容：把店家目前可買的每個 SKU 展開成立即執行的 SKU job（和延後中的同 SKU job 合併）。
func (s *Service) RunStore(ctx context.Context, storeID int64) error {
	if err := s.q(ctx).ConsumeRestockPending(ctx, storeKey(storeID)); err != nil {
		return err
	}
	after := uuid.Nil()
	for {
		ids, err := s.q(ctx).ListPurchasableSkusOfStore(ctx, store.ListPurchasableSkusOfStoreParams{StoreID: storeID, After: after, At: s.d.Now(), PageSize: storePage})
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.enqueueSku(ctx, id, 0); err != nil {
				return err
			}
		}
		if len(ids) < storePage {
			return nil
		}
		after = ids[len(ids)-1]
	}
}

// RunPurge 是清訂閱 job 的內容。
func (s *Service) RunPurge(ctx context.Context, skuIDs []uuid.UUID) error {
	return s.d.Notifier.PurgeSKUSubscriptions(ctx, skuIDs)
}

type skuWorker struct {
	river.WorkerDefaults[SkuArgs]
	svc *Service
}

func (w *skuWorker) Work(ctx context.Context, j *river.Job[SkuArgs]) error {
	return w.svc.RunSku(ctx, j.Args.SkuID)
}

func (w *skuWorker) Timeout(*river.Job[SkuArgs]) time.Duration { return time.Minute }

type storeWorker struct {
	river.WorkerDefaults[StoreArgs]
	svc *Service
}

func (w *storeWorker) Work(ctx context.Context, j *river.Job[StoreArgs]) error {
	return w.svc.RunStore(ctx, j.Args.StoreID)
}

func (w *storeWorker) Timeout(*river.Job[StoreArgs]) time.Duration { return 5 * time.Minute }

type purgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	svc *Service
}

func (w *purgeWorker) Work(ctx context.Context, j *river.Job[PurgeArgs]) error {
	return w.svc.RunPurge(ctx, j.Args.SkuIDs)
}

func (w *purgeWorker) Timeout(*river.Job[PurgeArgs]) time.Duration { return time.Minute }

// AddWorkers 登記三種 job 的 River worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &skuWorker{svc: s})
	river.AddWorker(workers, &storeWorker{svc: s})
	river.AddWorker(workers, &purgeWorker{svc: s})
}
