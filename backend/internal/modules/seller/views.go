package seller

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/views"
)

// ProductStores 是商品所屬店家的 port（見 views.ProductStores）：彙總 job 在 transaction 之外一次批次解析；舊服務期間由
// internal/legacy/productservice 實作，由組裝點接上。
type ProductStores = views.ProductStores

// ProductView 是登入買家的一次商品瀏覽（buyer 的 ProductViewed，由 cmd/hawkr 轉進來）。
type ProductView struct {
	EventID    uuid.UUID
	ProductID  int64
	OccurredAt time.Time
}

// RecordProductView 追加一筆原始瀏覽事件（匿名的打點入口用，步驟 8；總次數、不去重，設計書 28）。只有一句 insert，不解析
// 商品所屬的店家（由彙總 job 在背景做），可以在 transaction 內或外呼叫。
func (m *Module) RecordProductView(ctx context.Context, productID int64) error {
	return m.views.Record(ctx, productID)
}

// IngestProductView 追加登入買家的一筆瀏覽事件（insert only）：event_id 重複的事件不會多一列。呼叫方通常在 buyer 的
// transaction 內（事件同步發布），所以只能是一句 insert，不能呼叫外部服務。
func (m *Module) IngestProductView(ctx context.Context, v ProductView) error {
	return m.views.Ingest(ctx, v.EventID, v.ProductID, v.OccurredAt)
}

// ProductViewsBetween 回傳店家在期間 [start, end)（左閉右開）的商品瀏覽總數（儀表板用）；只讀每日彙總，所以最近一次彙總之後
// 的瀏覽還沒計入。start 與 end 必須是台北的 00:00（期間以整個日曆日為單位）且 start 早於 end，storeID 必須是正數，否則回
// InvalidInput。
func (m *Module) ProductViewsBetween(ctx context.Context, storeID int64, start, end time.Time) (int64, error) {
	return m.views.Between(ctx, storeID, start, end)
}

// RollupProductViews 處理還沒處理的瀏覽事件並累加每日彙總（River 排程每 5 分鐘執行，這裡給測試與營運手動呼叫），回傳處理的
// 事件數。商品 port 失敗回 error，沒有事件被標記。
func (m *Module) RollupProductViews(ctx context.Context) (int, error) {
	return m.views.Rollup(ctx)
}

// RebuildProductViews 以已處理事件的 store_id 重算 [fromDay, toDay]（台北日曆日，含兩端）的每日彙總，一個 transaction 內完成。
// 原始事件只留 7 天：起日早於保留期會回 InvalidInput，重算只涵蓋保留期內完整的台北日。
func (m *Module) RebuildProductViews(ctx context.Context, fromDay, toDay time.Time) error {
	return m.views.Rebuild(ctx, fromDay, toDay)
}

// PurgeProductViewEvents 刪除發生超過 7 天、已處理的原始瀏覽事件（River 排程每小時執行），回傳刪除的事件數。
func (m *Module) PurgeProductViewEvents(ctx context.Context) (int, error) {
	return m.views.Purge(ctx)
}
