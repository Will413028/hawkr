// Package read 是評價的讀取路徑：公開列表、單筆、作者本人與統計。公開讀取的 SQL 都在 db/queries/public.sql，以同一段公開可讀述詞開頭。
package read

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Deps 是讀取路徑的依賴。
type Deps struct{ DB *db.DB }

// Service 是讀取路徑的 use case。
type Service struct{ d Deps }

// New 建立讀取路徑。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// pageSize 把呼叫端要的頁大小收斂到 [1, MaxPageSize]（AIP-158：0 或負數是預設，過大是上限）。
func pageSize(n int32) int32 {
	switch {
	case n <= 0:
		return model.DefaultPageSize
	case n > model.MaxPageSize:
		return model.MaxPageSize
	}
	return n
}

func cursorParams(after *model.Cursor) (*time.Time, *uuid.UUID) {
	if after == nil {
		return nil, nil
	}
	return &after.CreatedAt, &after.ID
}

// ratingArg 檢查評分篩選（0 是不篩，1 到 5）並轉成查詢的參數型別。
func ratingArg(ctx context.Context, f model.Filter) (int16, error) {
	if f.Rating < 0 || f.Rating > model.MaxRating {
		return 0, apperr.InvalidInputOn([]string{"Filter.Rating"}, "%s", text.ReviewFilterRatingInvalid(uitext.LocaleFrom(ctx)))
	}
	return int16(f.Rating), nil
}

// page 把多取一筆的結果切成一頁：多出來的那筆只用來判斷有沒有下一頁，Next 是這頁最後一筆的位置。
func (s *Service) page(ctx context.Context, list []store.ReviewReview, size int32) (model.Page, error) {
	more := len(list) > int(size)
	if more {
		list = list[:size]
	}
	ids := make([]uuid.UUID, len(list))
	for i, r := range list {
		ids[i] = r.ID
	}
	imgs, err := s.q(ctx).ListReviewImages(ctx, ids)
	if err != nil {
		return model.Page{}, err
	}
	byReview := rows.Images(imgs)
	down, err := s.takenDownReplies(ctx, ids)
	if err != nil {
		return model.Page{}, err
	}
	out := model.Page{Reviews: make([]model.Review, len(list))}
	for i, r := range list {
		out.Reviews[i] = rows.Review(r, byReview[r.ID])
		if down[r.ID] {
			out.Reviews[i].Reply = nil
		}
	}
	if more {
		last := list[len(list)-1]
		out.Next = &model.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out, nil
}

// ListProductReviews 是商品的公開評價；viewerBuyerID 大於 0 時套用觀看者的封鎖（買家之間任一方向、觀看者封鎖店），在 keyset 之前過濾。
func (s *Service) ListProductReviews(ctx context.Context, productID uuid.UUID, viewerBuyerID int64, f model.Filter, after *model.Cursor, size int32) (model.Page, error) {
	rating, err := ratingArg(ctx, f)
	if err != nil {
		return model.Page{}, err
	}
	size = pageSize(size)
	at, id := cursorParams(after)
	var list []store.ReviewReview
	if viewerBuyerID > 0 {
		list, err = s.q(ctx).ListProductReviewsForViewer(ctx, store.ListProductReviewsForViewerParams{
			ProductID: productID, ViewerBuyerID: viewerBuyerID, Rating: rating, WithImages: f.WithImages, AfterCreatedAt: at, AfterID: id, PageSize: size + 1,
		})
	} else {
		list, err = s.q(ctx).ListProductReviewsAnonymous(ctx, store.ListProductReviewsAnonymousParams{
			ProductID: productID, Rating: rating, WithImages: f.WithImages, AfterCreatedAt: at, AfterID: id, PageSize: size + 1,
		})
	}
	if err != nil {
		return model.Page{}, err
	}
	return s.page(ctx, list, size)
}

// ListStoreReviews 是店家的公開評價（買家看的賣家評價頁），觀看者規則同 ListProductReviews。
func (s *Service) ListStoreReviews(ctx context.Context, storeID, viewerBuyerID int64, f model.Filter, after *model.Cursor, size int32) (model.Page, error) {
	rating, err := ratingArg(ctx, f)
	if err != nil {
		return model.Page{}, err
	}
	size = pageSize(size)
	at, id := cursorParams(after)
	var list []store.ReviewReview
	if viewerBuyerID > 0 {
		list, err = s.q(ctx).ListStoreReviewsForViewer(ctx, store.ListStoreReviewsForViewerParams{
			StoreID: storeID, ViewerBuyerID: viewerBuyerID, Rating: rating, WithImages: f.WithImages, AfterCreatedAt: at, AfterID: id, PageSize: size + 1,
		})
	} else {
		list, err = s.q(ctx).ListStoreReviewsAnonymous(ctx, store.ListStoreReviewsAnonymousParams{
			StoreID: storeID, Rating: rating, WithImages: f.WithImages, AfterCreatedAt: at, AfterID: id, PageSize: size + 1,
		})
	}
	if err != nil {
		return model.Page{}, err
	}
	return s.page(ctx, list, size)
}

// ListOwnStoreReviews 是店家自己的後台看到的評價：公開可讀，不套觀看者（店家沒有買家身分）。
func (s *Service) ListOwnStoreReviews(ctx context.Context, storeID int64, f model.Filter, after *model.Cursor, size int32) (model.Page, error) {
	return s.ListStoreReviews(ctx, storeID, 0, f, after, size)
}

// ListMyReviews 是作者本人的評價歷史：被隱藏的不給作者（舊行為）；orderItemID 大於 0 時只列那個訂單項目的評價。
func (s *Service) ListMyReviews(ctx context.Context, buyerID, orderItemID int64, after *model.Cursor, size int32) (model.Page, error) {
	size = pageSize(size)
	at, id := cursorParams(after)
	list, err := s.q(ctx).ListMyReviews(ctx, store.ListMyReviewsParams{BuyerID: buyerID, OrderItemID: orderItemID, AfterCreatedAt: at, AfterID: id, PageSize: size + 1})
	if err != nil {
		return model.Page{}, err
	}
	return s.page(ctx, list, size)
}

// GetReview 是單筆讀取：與列表同一個公開可讀述詞，觀看者規則同列表；不可讀的與不存在的同一個 NotFound，不洩漏存在性。
func (s *Service) GetReview(ctx context.Context, id uuid.UUID, viewerBuyerID int64) (model.Review, error) {
	var r store.ReviewReview
	var err error
	if viewerBuyerID > 0 {
		r, err = s.q(ctx).GetPublicReviewForViewer(ctx, store.GetPublicReviewForViewerParams{ID: id, ViewerBuyerID: viewerBuyerID})
	} else {
		r, err = s.q(ctx).GetPublicReviewAnonymous(ctx, id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Review{}, apperr.NotFound("%s", text.ReviewNotFound(uitext.LocaleFrom(ctx)))
	}
	if err != nil {
		return model.Review{}, err
	}
	imgs, err := s.q(ctx).ListReviewImages(ctx, []uuid.UUID{id})
	if err != nil {
		return model.Review{}, err
	}
	out := rows.Review(r, rows.Images(imgs)[id])
	down, err := s.takenDownReplies(ctx, []uuid.UUID{id})
	if err != nil {
		return model.Review{}, err
	}
	if down[id] {
		out.Reply = nil
	}
	return out, nil
}

// takenDownReplies 回傳店家回覆被內容下架措施下架的評價 ID；讀取時回覆換成沒有回覆。
func (s *Service) takenDownReplies(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	got, err := s.q(ctx).TakenDownReplyIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]bool, len(got))
	for _, id := range got {
		out[id] = true
	}
	return out, nil
}

// ReviewForReport 是檢舉用的單筆讀取：可讀規則與 GetReview 共用同一組公開可讀述詞與觀看者封鎖，不可讀與不存在同為 NotFound；不鎖列、加入呼叫方的 transaction。
func (s *Service) ReviewForReport(ctx context.Context, id uuid.UUID, viewerBuyerID int64) (model.ReportedReview, error) {
	r, err := s.GetReview(ctx, id, viewerBuyerID)
	if err != nil {
		return model.ReportedReview{}, err
	}
	out := model.ReportedReview{
		ID: r.ID, BuyerID: r.BuyerID, StoreID: r.StoreID, ProductID: r.ProductID, Rating: r.Rating, Content: r.Content,
		AssetIDs: make([]string, len(r.Images)), CreatedAt: r.CreatedAt, Reply: r.Reply,
	}
	for i, im := range r.Images {
		out.AssetIDs[i] = im.AssetID.String()
	}
	return out, nil
}

// ReviewForRelay 讀單則評價的原始列與它現在是否公開可讀（relay 組事件 payload 用，不套公開可讀述詞）；列不存在（已清除）是 NotFound。
func (s *Service) ReviewForRelay(ctx context.Context, id uuid.UUID) (model.RelayReview, error) {
	r, err := s.q(ctx).GetReviewByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RelayReview{}, apperr.NotFound("%s", text.ReviewNotFound(uitext.LocaleFrom(ctx)))
	}
	if err != nil {
		return model.RelayReview{}, err
	}
	imgs, err := s.q(ctx).ListReviewImages(ctx, []uuid.UUID{id})
	if err != nil {
		return model.RelayReview{}, err
	}
	public, err := s.q(ctx).IsReviewPublic(ctx, id)
	if err != nil {
		return model.RelayReview{}, err
	}
	return model.RelayReview{Review: rows.Review(r, rows.Images(imgs)[id]), Public: public}, nil
}

// ReviewStore 回評價所屬的店與現在是否公開可讀（不套公開可讀述詞）；不存在是 NotFound。
func (s *Service) ReviewStore(ctx context.Context, id uuid.UUID) (model.ReviewStore, error) {
	row, err := s.q(ctx).GetReviewStore(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ReviewStore{}, apperr.NotFound("%s", text.ReviewNotFound(uitext.LocaleFrom(ctx)))
	}
	if err != nil {
		return model.ReviewStore{}, err
	}
	return model.ReviewStore{StoreID: row.StoreID, Public: row.Public}, nil
}

// ProductStats 是商品的評價統計（公開可讀的評價，不套觀看者）。
func (s *Service) ProductStats(ctx context.Context, productID uuid.UUID) (model.ProductStats, error) {
	r, err := s.q(ctx).ProductStats(ctx, productID)
	return model.ProductStats{
		Count: r.TotalCount, Average: r.AvgRating, WithImagesCount: r.WithImagesCount,
		Distribution: [5]int64{r.Rating1Count, r.Rating2Count, r.Rating3Count, r.Rating4Count, r.Rating5Count},
	}, err
}

// StoreStats 是店家的評價統計（含各星等的數量）。
func (s *Service) StoreStats(ctx context.Context, storeID int64) (model.StoreStats, error) {
	r, err := s.q(ctx).StoreStats(ctx, storeID)
	return model.StoreStats{
		Count: r.TotalCount, Average: r.AvgRating,
		Distribution: [5]int64{r.Rating1Count, r.Rating2Count, r.Rating3Count, r.Rating4Count, r.Rating5Count},
	}, err
}

// StoreRatingStats 批次回傳多家店的評分：storeIDs 必須是 1 到 MaxStoreRatingStatsBatch 個正整數，重複的只算一次；只回有公開評價的店。
// 失敗回錯誤（fail-closed）。
func (s *Service) StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]model.StoreRating, error) {
	if len(storeIDs) == 0 || len(storeIDs) > model.MaxStoreRatingStatsBatch {
		return nil, apperr.InvalidInputOn([]string{"StoreIDs"}, "%s", text.ReviewStoreIdsInvalid(uitext.LocaleFrom(ctx), text.ReviewStoreIdsInvalidArgs{Max: model.MaxStoreRatingStatsBatch}))
	}
	for _, id := range storeIDs {
		if id <= 0 {
			return nil, apperr.InvalidInputOn([]string{"StoreIDs"}, "%s", text.ReviewStoreIdsInvalid(uitext.LocaleFrom(ctx), text.ReviewStoreIdsInvalidArgs{Max: model.MaxStoreRatingStatsBatch}))
		}
	}
	list, err := s.q(ctx).StoreRatingStats(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]model.StoreRating, len(list))
	for _, r := range list {
		out[r.StoreID] = model.StoreRating{Count: r.TotalCount, Average: r.AvgRating}
	}
	return out, nil
}
