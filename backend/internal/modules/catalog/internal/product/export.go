package product

import (
	"context"
	"strings"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
)

func checkBatch(ids []uuid.UUID) error {
	if len(ids) > model.MaxBatchIDs {
		return model.Invalid("IDs", model.ReasonTooMany)
	}
	for _, id := range ids {
		if id == uuid.Nil() {
			return model.Invalid("IDs", model.ReasonInvalid)
		}
	}
	return nil
}

// Cards 批次讀商品卡片：沒刪除的商品；includeInactive 為假時只回 ACTIVE 且店家在 at 可曝光的（公開可見），為真時任何狀態都回。
func (s *Service) Cards(ctx context.Context, at time.Time, ids []uuid.UUID, includeInactive bool) (map[uuid.UUID]model.ProductCard, error) {
	if err := checkBatch(ids); err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]model.ProductCard, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q(ctx).GetProductCards(ctx, store.GetProductCardsParams{Ids: ids, IncludeInactive: includeInactive, At: at})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = model.ProductCard{ProductSummary: cardSummary(r), Visible: r.Visible}
	}
	return out, nil
}

// StoreIDs 批次讀商品所屬店家：沒刪除的商品，任何狀態、不看店家曝光。
func (s *Service) StoreIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	if err := checkBatch(ids); err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q(ctx).GetProductStoreIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.StoreID
	}
	return out, nil
}

// StoresWithPublicProducts 回至少有一件公開商品（ACTIVE、沒刪除）且店家在 at 可曝光的店家 ID，升序。
func (s *Service) StoresWithPublicProducts(ctx context.Context, at time.Time) ([]int64, error) {
	ids, err := s.q(ctx).ListStoresWithPublicProducts(ctx, at)
	if err != nil {
		return nil, err
	}
	return nonNil(ids), nil
}

// SkuSnapshots 批次讀 SKU 與它的商品此刻的內容，不過濾狀態與刪除；Active 在這裡判定：商品在買家可見的商品之內（ACTIVE、店家可曝光、沒有生效中的下架措施、沒刪除）且 SKU 沒刪除。
func (s *Service) SkuSnapshots(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]model.SkuSnapshot, error) {
	if err := checkBatch(ids); err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]model.SkuSnapshot, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q(ctx).GetSkuSnapshots(ctx, store.GetSkuSnapshotsParams{At: s.d.Now(), Ids: ids})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		deleted := r.ProductDeleted || r.SkuDeleted
		status := model.ProductStatus(r.Status)
		out[r.SkuID] = model.SkuSnapshot{
			SkuID: r.SkuID, ProductID: r.ProductID, StoreID: r.StoreID, ProductTitle: r.Title,
			SkuName:  skuName(r.Option1Value, r.Option2Value),
			CoverKey: r.CoverKey, Price: r.Price, CompareAtPrice: r.CompareAtPrice, StockQty: r.StockQty,
			Status: status, Deleted: deleted,
			Active: r.ProductVisible && !r.SkuDeleted,
		}
	}
	return out, nil
}

func cardSummary(r store.GetProductCardsRow) model.ProductSummary {
	return summaryRow{
		ID: r.ID, StoreID: r.StoreID, CategoryID: r.CategoryID, Title: r.Title, Status: r.Status, FirstPublishedAt: r.FirstPublishedAt,
		MinPrice: r.MinPrice, MaxPrice: r.MaxPrice, SalesCount: r.SalesCount, FavoriteCount: r.FavoriteCount, RatingAverage: r.RatingAverage,
		ReviewCount: r.ReviewCount, LowStockThreshold: r.LowStockThreshold, ContentVersion: r.ContentVersion, CreatedAt: r.CreatedAt,
		CoverKey: r.CoverKey, TotalStock: r.TotalStock, SortKey: r.SortKey,
	}.summary()
}

func skuName(o1, o2 *string) string {
	var parts []string
	for _, o := range []*string{o1, o2} {
		if o != nil && *o != "" {
			parts = append(parts, *o)
		}
	}
	return strings.Join(parts, " / ")
}

// AddFavorite 把商品的收藏數加一（買家在收藏列同一個 transaction 內呼叫，D3）；商品不存在或已刪除回 model.ErrProductNotFound。
func (s *Service) AddFavorite(ctx context.Context, productID uuid.UUID) error {
	return s.favorite(ctx, productID, true)
}

// RemoveFavorite 把商品的收藏數減一，下限 0；商品不存在回 model.ErrProductNotFound，已刪除的商品照減（計數跟著收藏列走）。
func (s *Service) RemoveFavorite(ctx context.Context, productID uuid.UUID) error {
	return s.favorite(ctx, productID, false)
}

func (s *Service) favorite(ctx context.Context, productID uuid.UUID, add bool) error {
	if productID == uuid.Nil() {
		return model.Invalid("ProductID", model.ReasonRequired)
	}
	var n int64
	var err error
	if add {
		n, err = s.q(ctx).IncrementProductFavorites(ctx, store.IncrementProductFavoritesParams{ID: productID, Now: s.d.Now()})
	} else {
		n, err = s.q(ctx).DecrementProductFavorites(ctx, store.DecrementProductFavoritesParams{ID: productID, Now: s.d.Now()})
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrProductNotFound
	}
	return nil
}
