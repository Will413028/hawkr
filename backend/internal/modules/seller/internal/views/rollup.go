package views

import (
	"context"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

const (
	// batchSize 是一批處理的事件數，也是一次批次呼叫商品 port 的商品數上限。
	batchSize int32 = 100
	// maxBatches 是一次執行處理的批數上限：到了就停，下一次排程接著做（單次執行有界，積壓時不會一直占住 worker）。
	maxBatches = 50
	// portTimeout 是呼叫商品 port 的 deadline（port 的實作自己也可以更短）。
	portTimeout = 10 * time.Second
	// purgeBatch 是一個 transaction 刪除的事件數。
	purgeBatch int32 = 1000
)

// Rollup 處理還沒處理的瀏覽事件並累加每日彙總，回傳處理的事件數。一批的流程：
//  1. 讀一批還沒處理的事件（依發生順序），沒有就結束；
//  2. 在 transaction 之外呼叫商品 port，一次解析這批的商品所屬的店家（規則 5 的精神：外部呼叫不在 DB transaction 內）；
//  3. 一個 transaction：取共享鎖（排除重算與清理）、鎖住這批仍然沒處理的列（FOR UPDATE SKIP LOCKED），一句話內標記並累加
//     彙總（日期取自 occurred_at 的台北日曆日，不是處理時間）。
//
// 商品 port 的暫時性錯誤直接回 error，這批沒有任何事件被標記，River 重試時會重新處理（只計一次）。商品不存在或沒有店家
// 的事件標為 UNRESOLVED，不計入也不重試。重跑是 no-op：已處理的列不會再被選中。
func (s *Service) Rollup(ctx context.Context) (int, error) {
	total := 0
	for range maxBatches {
		read, applied, err := s.rollupBatch(ctx)
		total += applied
		if err != nil {
			return total, err
		}
		if read < int(batchSize) {
			return total, nil
		}
	}
	return total, nil
}

// rollupBatch 處理一批，回傳讀到的事件數（沒讀滿代表取完了）與實際標記的事件數（別的 job 鎖住或已處理的不算）。
func (s *Service) rollupBatch(ctx context.Context) (int, int, error) {
	pending, err := store.New(s.d.DB.Conn(ctx)).PendingProductViewEvents(ctx, batchSize)
	if err != nil || len(pending) == 0 {
		return 0, 0, err
	}
	productIDs := make([]int64, 0, len(pending))
	for _, e := range pending {
		productIDs = append(productIDs, e.ProductID)
	}
	slices.Sort(productIDs)
	productIDs = slices.Compact(productIDs)
	portCtx, cancel := context.WithTimeout(ctx, portTimeout)
	stores, err := s.d.ProductStores.StoreIDs(portCtx, productIDs)
	cancel()
	if err != nil {
		return 0, 0, fmt.Errorf("views：解析商品所屬的店家：%w", err)
	}
	applied := 0
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if err := q.LockProductViewMaintenance(ctx); err != nil {
			return err
		}
		locked, err := q.LockPendingProductViewEvents(ctx, idsOf(pending))
		if err != nil {
			return err
		}
		eventIDs := make([]uuid.UUID, 0, len(locked))
		storeIDs := make([]int64, 0, len(locked))
		for _, e := range locked {
			eventIDs = append(eventIDs, e.EventID)
			storeIDs = append(storeIDs, max(stores[e.ProductID], 0))
		}
		if len(eventIDs) == 0 {
			return nil
		}
		marked, err := q.ApplyProductViewResolutions(ctx, store.ApplyProductViewResolutionsParams{
			EventIds: eventIDs, StoreIds: storeIDs, ProcessedAt: s.d.Now().UTC(),
		})
		applied = int(marked)
		return err
	})
	return len(pending), applied, err
}

func idsOf(events []store.PendingProductViewEventsRow) []uuid.UUID {
	ids := make([]uuid.UUID, len(events))
	for i, e := range events {
		ids[i] = e.EventID
	}
	return ids
}

// Rebuild 以已處理事件儲存的 store_id 重算 [from, to]（台北日曆日，含兩端）的每日彙總，一個 transaction 內先刪後建，沒有
// 彙總缺漏的空窗。只涵蓋完整落在原始事件保留期內的日子（model.ViewRebuildRange）：更早的事件可能已被清理。重算與彙總 job、
// 清理以 advisory lock 互斥，所以不會在重建途中被插入或刪除。還沒處理的事件不在其中（它們也還沒進彙總，之後處理時才累加）。
func (s *Service) Rebuild(ctx context.Context, from, to time.Time) error {
	from, to, err := model.ViewRebuildRange(s.d.Now(), from, to)
	if err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if err := q.LockProductViewRebuild(ctx); err != nil {
			return err
		}
		if err := q.DeleteProductViewsDaily(ctx, store.DeleteProductViewsDailyParams{FromDay: from, ToDay: to}); err != nil {
			return err
		}
		return q.InsertRebuiltProductViewsDaily(ctx, store.InsertRebuiltProductViewsDailyParams{FromDay: from, ToDay: to})
	})
}

// Purge 刪除發生時間早於 now − 7 天、已處理的事件，分批到取不滿為止，回傳刪除的事件數。還沒處理的事件不刪（不論多舊）。
// 每批一個 transaction，取獨占鎖與重算互斥。
func (s *Service) Purge(ctx context.Context) (int, error) {
	total := 0
	for {
		var n int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			if err := q.LockProductViewRebuild(ctx); err != nil {
				return err
			}
			var err error
			n, err = q.PurgeProcessedProductViewEvents(ctx, store.PurgeProcessedProductViewEventsParams{
				Cutoff: s.d.Now().Add(-model.ViewEventRetention).UTC(), BatchLimit: purgeBatch,
			})
			return err
		})
		total += int(n)
		if err != nil || n < int64(purgeBatch) {
			return total, err
		}
	}
}
