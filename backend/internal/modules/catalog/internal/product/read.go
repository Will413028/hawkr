package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/description"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
)

// head 是商品列與它的子表（同一句 SQL 讀出，賣家與公開的讀取各有自己的 sqlc 型別）。
type head struct {
	id               uuid.UUID
	storeID          int64
	categoryID       uuid.UUID
	title            string
	description      []byte
	status           string
	firstPublishedAt *time.Time
	minPrice         int64
	maxPrice         int64
	salesCount       int32
	favoriteCount    int32
	ratingAverage    float64
	reviewCount      int32
	createdAt        time.Time
	updatedAt        time.Time
	images           []byte
	axes             []byte
	skus             []byte
	hashtags         []byte
}

type imageJSON struct {
	Kind      string    `json:"kind"`
	AssetID   uuid.UUID `json:"asset_id"`
	ObjectKey string    `json:"object_key"`
	Alt       string    `json:"alt"`
}

type axisJSON struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type skuJSON struct {
	ID             uuid.UUID `json:"id"`
	Option1        *string   `json:"option1"`
	Option2        *string   `json:"option2"`
	Price          int64     `json:"price"`
	CompareAtPrice *int64    `json:"compare_at_price"`
	StockQty       int32     `json:"stock_qty"`
}

type hashtagJSON struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// Get 讀店家自己的商品（任何狀態，沒刪除）。
func (s *Service) Get(ctx context.Context, storeID int64, productID uuid.UUID) (model.Product, error) {
	return s.load(ctx, s.q(ctx), productID, storeID, nil)
}

// GetPublic 讀公開可見的商品：ACTIVE、沒刪除、店家在 at 這一刻可曝光（at 是請求的 clock）；否則 ErrProductNotFound。
func (s *Service) GetPublic(ctx context.Context, productID uuid.UUID, at time.Time) (model.Product, error) {
	return s.load(ctx, s.q(ctx), productID, 0, &at)
}

// load 讀商品與它的選項、SKU、圖片與 hashtag。public 非 nil 時走公開的條件（storeID 不用），否則限定店家。
// 商品列與全部子表在同一句 SQL 讀出，所以是一份快照：並行的修改不會讓描述指到讀不到的圖片，也不會混到兩個版本。
// 公開讀取會被別的模組呼叫，規則 3（backend/AGENTS.md）不讓它開 InReadSnapshot，單一語句是合規的做法。
func (s *Service) load(ctx context.Context, q *store.Queries, id uuid.UUID, storeID int64, public *time.Time) (model.Product, error) {
	var h head
	if public != nil {
		r, err := q.GetPublicProduct(ctx, store.GetPublicProductParams{ID: id, At: *public})
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Product{}, model.ErrProductNotFound
		}
		if err != nil {
			return model.Product{}, err
		}
		h = head{r.ID, r.StoreID, r.CategoryID, r.Title, r.Description, r.Status, r.FirstPublishedAt, r.MinPrice, r.MaxPrice, r.SalesCount, r.FavoriteCount, r.RatingAverage, r.ReviewCount, r.CreatedAt, r.UpdatedAt, r.Images, r.Axes, r.Skus, r.Hashtags}
	} else {
		r, err := q.GetOwnProduct(ctx, store.GetOwnProductParams{ID: id, StoreID: storeID})
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Product{}, model.ErrProductNotFound
		}
		if err != nil {
			return model.Product{}, err
		}
		h = head{r.ID, r.StoreID, r.CategoryID, r.Title, r.Description, r.Status, r.FirstPublishedAt, r.MinPrice, r.MaxPrice, r.SalesCount, r.FavoriteCount, r.RatingAverage, r.ReviewCount, r.CreatedAt, r.UpdatedAt, r.Images, r.Axes, r.Skus, r.Hashtags}
	}
	p := model.Product{
		ID: h.id, StoreID: h.storeID, CategoryID: h.categoryID, Title: h.title, Status: model.ProductStatus(h.status),
		FirstPublishedAt: h.firstPublishedAt, MinPrice: h.minPrice, MaxPrice: h.maxPrice, SalesCount: h.salesCount,
		FavoriteCount: h.favoriteCount, RatingAverage: h.ratingAverage, ReviewCount: h.reviewCount, CreatedAt: h.createdAt, UpdatedAt: h.updatedAt,
	}
	if !p.Status.Valid() {
		return model.Product{}, errors.New("catalog: stored product status is not valid: " + h.status)
	}
	var images []imageJSON
	if err := json.Unmarshal(h.images, &images); err != nil {
		return model.Product{}, fmt.Errorf("catalog: decode product images: %w", err)
	}
	byAsset := make(map[uuid.UUID]model.Image, len(images))
	p.Gallery = []model.Image{}
	for _, im := range images {
		img := model.Image{AssetID: im.AssetID, ObjectKey: im.ObjectKey, Alt: im.Alt}
		byAsset[im.AssetID] = img
		if im.Kind == kindGallery {
			p.Gallery = append(p.Gallery, img)
		}
	}
	var err error
	if p.Description, err = description.Decode(h.description, byAsset); err != nil {
		return model.Product{}, err
	}
	var axes []axisJSON
	if err := json.Unmarshal(h.axes, &axes); err != nil {
		return model.Product{}, fmt.Errorf("catalog: decode product axes: %w", err)
	}
	p.Axes = make([]model.Axis, 0, len(axes))
	for _, a := range axes {
		p.Axes = append(p.Axes, model.Axis{Name: a.Name, Values: append([]string{}, a.Values...)})
	}
	var skus []skuJSON
	if err := json.Unmarshal(h.skus, &skus); err != nil {
		return model.Product{}, fmt.Errorf("catalog: decode product skus: %w", err)
	}
	p.Skus = make([]model.Sku, 0, len(skus))
	for _, k := range skus {
		p.Skus = append(p.Skus, model.Sku{ID: k.ID, Option1: deref(k.Option1), Option2: deref(k.Option2), Price: k.Price, CompareAtPrice: k.CompareAtPrice, StockQty: k.StockQty})
	}
	var tags []hashtagJSON
	if err := json.Unmarshal(h.hashtags, &tags); err != nil {
		return model.Product{}, fmt.Errorf("catalog: decode product hashtags: %w", err)
	}
	p.Hashtags = make([]model.Hashtag, 0, len(tags))
	for _, t := range tags {
		p.Hashtags = append(p.Hashtags, model.Hashtag{Name: t.Name, Display: t.DisplayName})
	}
	return p, nil
}

func pageSize(size int32) int32 {
	if size <= 0 || size > model.MaxListPageSize {
		return model.MaxListPageSize
	}
	return size
}

func cursorArgs(after *model.ListCursor) (*time.Time, *uuid.UUID) {
	if after == nil {
		return nil, nil
	}
	return &after.At, &after.ID
}

// ListSeller 是賣家清單：店家自己的所有沒刪除的商品（任何狀態，不看曝光），最新建立的在前。size 預設與上限是 model.MaxListPageSize。
func (s *Service) ListSeller(ctx context.Context, storeID int64, after *model.ListCursor, size int32) (model.ProductPage, error) {
	size = pageSize(size)
	at, aid := cursorArgs(after)
	rows, err := s.q(ctx).ListSellerProducts(ctx, store.ListSellerProductsParams{StoreID: storeID, AfterAt: at, AfterID: aid, RowLimit: size + 1})
	if err != nil {
		return model.ProductPage{}, err
	}
	items := make([]model.ProductSummary, 0, len(rows))
	for _, r := range rows {
		items = append(items, summary(r.ID, r.StoreID, r.CategoryID, r.Title, r.Status, r.FirstPublishedAt, r.MinPrice, r.MaxPrice, r.SalesCount, r.FavoriteCount, r.RatingAverage, r.ReviewCount, r.CoverKey, r.CreatedAt))
	}
	return finishPage(items, size, func(p model.ProductSummary) time.Time { return p.CreatedAt }), nil
}

// ListPublic 是公開列表：只含 ACTIVE、沒刪除、店家在 at 這一刻可曝光的商品，最新上架的在前；categoryID 非 nil 時只列該分類。
// at 是請求的 clock，不是 client 送的。
func (s *Service) ListPublic(ctx context.Context, at time.Time, categoryID *uuid.UUID, after *model.ListCursor, size int32) (model.ProductPage, error) {
	size = pageSize(size)
	aat, aid := cursorArgs(after)
	rows, err := s.q(ctx).ListPublicProducts(ctx, store.ListPublicProductsParams{At: at, CategoryID: categoryID, AfterAt: aat, AfterID: aid, RowLimit: size + 1})
	if err != nil {
		return model.ProductPage{}, err
	}
	items := make([]model.ProductSummary, 0, len(rows))
	for _, r := range rows {
		items = append(items, summary(r.ID, r.StoreID, r.CategoryID, r.Title, r.Status, r.FirstPublishedAt, r.MinPrice, r.MaxPrice, r.SalesCount, r.FavoriteCount, r.RatingAverage, r.ReviewCount, r.CoverKey, r.CreatedAt))
	}
	return finishPage(items, size, func(p model.ProductSummary) time.Time {
		if p.FirstPublishedAt == nil {
			return time.Time{}
		}
		return *p.FirstPublishedAt
	}), nil
}

func summary(id uuid.UUID, storeID int64, categoryID uuid.UUID, title, status string, firstPublishedAt *time.Time, minPrice, maxPrice int64, sales, favorites int32, rating float64, reviews int32, cover string, createdAt time.Time) model.ProductSummary {
	return model.ProductSummary{
		ID: id, StoreID: storeID, CategoryID: categoryID, Title: title, Status: model.ProductStatus(status), FirstPublishedAt: firstPublishedAt,
		MinPrice: minPrice, MaxPrice: maxPrice, SalesCount: sales, FavoriteCount: favorites, RatingAverage: rating, ReviewCount: reviews, CoverKey: cover, CreatedAt: createdAt,
	}
}

// finishPage 把多取的一列轉成下一頁的游標。
func finishPage(items []model.ProductSummary, size int32, at func(model.ProductSummary) time.Time) model.ProductPage {
	page := model.ProductPage{Items: items}
	if len(items) > int(size) {
		page.Items = items[:size]
		last := page.Items[size-1]
		page.Next = &model.ListCursor{At: at(last), ID: last.ID}
	}
	return page
}
