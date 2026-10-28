// Package views 是商品瀏覽數（`docs/modules/seller.md` §16）：寫入端只追加原始事件（product_view_events），背景的彙總 job 在請求的關鍵路徑之外
// 解析商品所屬的店家，並彙總成每家店每個台北日一列的 product_views_daily；儀表板（本週與上週，週日起算）只讀這張彙總表，
// 是顯示數字的唯一事實來源。總次數、不去重。來源是 seller-service 的 seller_product_view_log（遷移計畫 A3b 步驟 7c 階段 4）。
//
// 為什麼不在請求內累加：解析店家要呼叫商品服務（它掛了買家的瀏覽就失敗），而同一列每日彙總的 upsert 在熱門店家上是熱點列，
// 計數器出錯還會讓瀏覽回滾。業界的分析事件（Plausible、PostHog 的 ingestion；YouTube、Shopify 的延遲式分析）都是追加原始事件、
// 背景批次補充與彙總。彙總可以由保留期內的原始事件重算（Rebuild）。
//
// 鍵是 store_id（業務鍵）：舊實作的 App 路徑把帳號 ID 寫進同一個 seller_id 欄、儀表板以 store_id 讀，所以 App 來源的瀏覽從不
// 進儀表板。店家由彙總 job 經 ProductStores port 解析（商品在 catalog，seller 不依賴它）。
package views

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// ProductStores 是商品所屬店家的 port（舊服務期間由 internal/legacy/productservice 實作，catalog 搬進單體後改呼叫它的公開
// API）。一次批次呼叫：回傳的 map 只含存在的商品，值是店家的 store_id；缺席的商品表示不存在，值 ≤ 0 表示商品沒有店家。
// 暫時性的錯誤（服務不通、逾時）回 error，彙總 job 不標記任何事件、交給 River 重試。已下架的商品照樣有店家（直接連結仍可瀏覽）。
type ProductStores interface {
	StoreIDs(ctx context.Context, productIDs []int64) (map[int64]int64, error)
}

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB            *db.DB
	ProductStores ProductStores
	Now           func() time.Time
}

// Service 是瀏覽數的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// Record 追加一筆原始瀏覽事件（匿名的打點；步驟 8 的 beacon 呼叫）。只有一句 insert：不解析商品、不碰彙總，可以在
// transaction 內或外呼叫。
func (s *Service) Record(ctx context.Context, productID int64) error {
	return s.Ingest(ctx, uuid.NewV7(), productID, s.d.Now())
}

// Ingest 追加一筆已有 ID 與發生時間的原始瀏覽事件（登入買家的 ProductViewed，由組裝點轉進來）。event_id 重複的事件
// 不會多一列，所以事件重送不會重複計數。
func (s *Service) Ingest(ctx context.Context, eventID uuid.UUID, productID int64, occurredAt time.Time) error {
	if err := model.Positive(productID); err != nil {
		return err
	}
	return store.New(s.d.DB.Conn(ctx)).InsertProductViewEvent(ctx, store.InsertProductViewEventParams{
		EventID: eventID, ProductID: productID, OccurredAt: occurredAt.UTC(),
	})
}

// Between 回傳店家在期間 [start, end) 的瀏覽總數，只讀每日彙總；沒有彙總列的日子是 0。start 與 end 必須是台北的 00:00
// （model.ViewRange），否則回 InvalidInput。
func (s *Service) Between(ctx context.Context, storeID int64, start, end time.Time) (int64, error) {
	if err := model.Positive(storeID); err != nil {
		return 0, err
	}
	first, endDay, err := model.ViewRange(start, end)
	if err != nil {
		return 0, err
	}
	return store.New(s.d.DB.Conn(ctx)).ProductViewsBetween(ctx, store.ProductViewsBetweenParams{StoreID: storeID, FirstDay: first, EndDay: endDay})
}
