package usecase

import (
	"context"
	"math"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

type ProductViewUseCase struct {
	repo          domain.ProductViewRepository
	productClient ProductClient
	tsid          func() int64
}

func NewProductViewUseCase(repo domain.ProductViewRepository, productClient ProductClient, tsid func() int64) *ProductViewUseCase {
	return &ProductViewUseCase{repo: repo, productClient: productClient, tsid: tsid}
}

// RecordView 寫入一筆商品瀏覽事件（source: "app" 或 "web"）
func (u *ProductViewUseCase) RecordView(ctx context.Context, sellerID, productID int64, source string) error {
	return u.repo.Insert(ctx, u.tsid(), sellerID, productID, source)
}

// RecordViewByProductID 從 product_id 查出 seller_id 後寫入（網頁端路徑，50ms timeout）
func (u *ProductViewUseCase) RecordViewByProductID(ctx context.Context, productID int64) error {
	lookupCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	products, err := u.productClient.GetProductsByIDs(lookupCtx, []int64{productID})
	if err != nil || len(products) == 0 {
		return nil // best-effort，靜默忽略
	}
	p := products[0]
	if p.SellerID == 0 {
		return nil
	}
	return u.repo.Insert(ctx, u.tsid(), p.SellerID, productID, "web")
}

// GetStats 回傳賣家本期商品總瀏覽數與週環比成長率（依日曆週，週日起算）
func (u *ProductViewUseCase) GetStats(ctx context.Context, storeID int64) (viewCount int64, growthPct int32, err error) {
	now := time.Now().UTC()
	thisWeekStart, lastWeekStart := calendarWeekBounds(now)

	viewCount, err = u.repo.CountByDateRange(ctx, storeID, thisWeekStart, now)
	if err != nil {
		return 0, 0, err
	}

	previousCount, err := u.repo.CountByDateRange(ctx, storeID, lastWeekStart, thisWeekStart)
	if err != nil {
		return 0, 0, err
	}

	denom := previousCount
	if denom < 1 {
		denom = 1
	}
	growthPct = int32(math.Round(float64(viewCount-previousCount) / float64(denom) * 100))

	return viewCount, growthPct, nil
}
