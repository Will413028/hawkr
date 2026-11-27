package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/description"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/hashtag"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

type head struct {
	ID                uuid.UUID
	StoreID           int64
	CategoryID        uuid.UUID
	Title             string
	Description       []byte
	Status            string
	FirstPublishedAt  *time.Time
	MinPrice          int64
	MaxPrice          int64
	SalesCount        int32
	FavoriteCount     int32
	RatingAverage     float64
	ReviewCount       int32
	LowStockThreshold *int32
	ContentVersion    int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Images            []byte
	Axes              []byte
	Skus              []byte
	Hashtags          []byte
}

type scope struct {
	storeID int64
	public  *time.Time
	admin   bool
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

// OwnSku 讀店家自己的 SKU 此刻的內容與可購買性，含已刪除的 SKU 與商品；SKU 不存在或不是這家店的回 model.ErrSkuNotFound。
func (s *Service) OwnSku(ctx context.Context, storeID int64, skuID uuid.UUID) (model.SkuState, error) {
	r, err := s.q(ctx).GetOwnSkuState(ctx, store.GetOwnSkuStateParams{ID: skuID, StoreID: storeID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SkuState{}, model.ErrSkuNotFound
	}
	if err != nil {
		return model.SkuState{}, err
	}
	return model.SkuState{
		Sku:      model.Sku{ID: r.ID, Option1: deref(r.Option1Value), Option2: deref(r.Option2Value), Price: r.Price, CompareAtPrice: r.CompareAtPrice, StockQty: r.StockQty},
		Sellable: r.Sellable,
	}, nil
}

// GetPublic 讀公開可見的商品：ACTIVE 或 ARCHIVED（下架的商品頁保留，呼叫端以狀態決定可不可以買）、沒刪除、店家在 at 這一刻可曝光（at 是請求的 clock）；
// 草稿與其他情況回 ErrProductNotFound。
func (s *Service) GetPublic(ctx context.Context, productID uuid.UUID, at time.Time) (model.Product, error) {
	return s.loadScoped(ctx, s.q(ctx), productID, scope{public: &at})
}

// GetAdmin 讀任何店家的商品（任何狀態，沒刪除）；後台唯讀用。
func (s *Service) GetAdmin(ctx context.Context, productID uuid.UUID) (model.Product, error) {
	return s.loadScoped(ctx, s.q(ctx), productID, scope{admin: true})
}

func (s *Service) load(ctx context.Context, q *store.Queries, id uuid.UUID, storeID int64, public *time.Time) (model.Product, error) {
	return s.loadScoped(ctx, q, id, scope{storeID: storeID, public: public})
}

func (s *Service) loadScoped(ctx context.Context, q *store.Queries, id uuid.UUID, sc scope) (model.Product, error) {
	var h head
	var err error
	switch {
	case sc.public != nil:
		var r store.GetPublicProductRow
		r, err = q.GetPublicProduct(ctx, store.GetPublicProductParams{ID: id, At: *sc.public})
		h = head(r)
	case sc.admin:
		var r store.GetAdminProductRow
		r, err = q.GetAdminProduct(ctx, id)
		h = head(r)
	default:
		var r store.GetOwnProductRow
		r, err = q.GetOwnProduct(ctx, store.GetOwnProductParams{ID: id, StoreID: sc.storeID})
		h = head(r)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Product{}, model.ErrProductNotFound
	}
	if err != nil {
		return model.Product{}, err
	}
	p := model.Product{
		ID: h.ID, StoreID: h.StoreID, CategoryID: h.CategoryID, Title: h.Title, Status: model.ProductStatus(h.Status),
		FirstPublishedAt: h.FirstPublishedAt, MinPrice: h.MinPrice, MaxPrice: h.MaxPrice, SalesCount: h.SalesCount,
		FavoriteCount: h.FavoriteCount, RatingAverage: h.RatingAverage, ReviewCount: h.ReviewCount, CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt,
		LowStockThreshold: h.LowStockThreshold, Version: h.ContentVersion,
	}
	if !p.Status.Valid() {
		return model.Product{}, errors.New("catalog: stored product status is not valid: " + h.Status)
	}
	var images []imageJSON
	if err := json.Unmarshal(h.Images, &images); err != nil {
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
	if p.Description, err = description.Decode(h.Description, byAsset); err != nil {
		return model.Product{}, err
	}
	var axes []axisJSON
	if err := json.Unmarshal(h.Axes, &axes); err != nil {
		return model.Product{}, fmt.Errorf("catalog: decode product axes: %w", err)
	}
	p.Axes = make([]model.Axis, 0, len(axes))
	for _, a := range axes {
		p.Axes = append(p.Axes, model.Axis{Name: a.Name, Values: append([]string{}, a.Values...)})
	}
	var skus []skuJSON
	if err := json.Unmarshal(h.Skus, &skus); err != nil {
		return model.Product{}, fmt.Errorf("catalog: decode product skus: %w", err)
	}
	p.Skus = make([]model.Sku, 0, len(skus))
	for _, k := range skus {
		p.Skus = append(p.Skus, model.Sku{ID: k.ID, Option1: deref(k.Option1), Option2: deref(k.Option2), Price: k.Price, CompareAtPrice: k.CompareAtPrice, StockQty: k.StockQty})
	}
	var tags []hashtagJSON
	if err := json.Unmarshal(h.Hashtags, &tags); err != nil {
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

type summaryRow struct {
	ID                uuid.UUID
	StoreID           int64
	CategoryID        uuid.UUID
	Title             string
	Status            string
	FirstPublishedAt  *time.Time
	MinPrice          int64
	MaxPrice          int64
	SalesCount        int32
	FavoriteCount     int32
	RatingAverage     float64
	ReviewCount       int32
	LowStockThreshold *int32
	ContentVersion    int64
	CreatedAt         time.Time
	CoverKey          string
	TotalStock        int32
	SortKey           int64
}

func (r summaryRow) summary() model.ProductSummary {
	return model.ProductSummary{
		ID: r.ID, StoreID: r.StoreID, CategoryID: r.CategoryID, Title: r.Title, Status: model.ProductStatus(r.Status), FirstPublishedAt: r.FirstPublishedAt,
		MinPrice: r.MinPrice, MaxPrice: r.MaxPrice, SalesCount: r.SalesCount, FavoriteCount: r.FavoriteCount, RatingAverage: r.RatingAverage,
		ReviewCount: r.ReviewCount, CoverKey: r.CoverKey, CreatedAt: r.CreatedAt, TotalStock: r.TotalStock, LowStockThreshold: r.LowStockThreshold, Version: r.ContentVersion,
	}
}

func finishPage(rows []summaryRow, size int32) model.ProductPage {
	items := make([]model.ProductSummary, 0, len(rows))
	for _, r := range rows {
		items = append(items, r.summary())
	}
	page := model.ProductPage{Items: items}
	if len(rows) > int(size) {
		page.Items = items[:size]
		last := rows[size-1]
		page.Next = &model.ListCursor{Key: last.SortKey, ID: last.ID}
	}
	return page
}

func timeCursor(after *model.ListCursor) (*time.Time, *uuid.UUID) {
	if after == nil {
		return nil, nil
	}
	at := time.UnixMicro(after.Key).UTC()
	return &at, &after.ID
}

func likePattern(q string) string {
	q = trimtext.Trim(q)
	if q == "" {
		return ""
	}
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
}

// ListSeller 是賣家清單：店家自己的所有沒刪除的商品（任何狀態，不看曝光），最新建立的在前，可依狀態、庫存與關鍵字篩選。size 預設與上限是 model.MaxListPageSize。
func (s *Service) ListSeller(ctx context.Context, storeID int64, f model.SellerFilter, after *model.ListCursor, size int32) (model.ProductPage, error) {
	if err := checkSellerFilter(f); err != nil {
		return model.ProductPage{}, err
	}
	size = pageSize(size)
	at, aid := timeCursor(after)
	rows, err := s.q(ctx).ListSellerProducts(ctx, store.ListSellerProductsParams{
		StoreID: storeID, Status: string(f.Status), StockStatus: string(f.StockStatus), DefaultThreshold: model.DefaultLowStockThreshold,
		QueryPattern: likePattern(f.Query), AfterAt: at, AfterID: aid, RowLimit: size + 1,
	})
	if err != nil {
		return model.ProductPage{}, err
	}
	out := make([]summaryRow, len(rows))
	for i, r := range rows {
		out[i] = summaryRow(r)
	}
	return finishPage(out, size), nil
}

// CountSeller 回店家自己符合篩選的商品數（篩選同 ListSeller）。
func (s *Service) CountSeller(ctx context.Context, storeID int64, f model.SellerFilter) (int32, error) {
	if err := checkSellerFilter(f); err != nil {
		return 0, err
	}
	return s.q(ctx).CountSellerProducts(ctx, store.CountSellerProductsParams{
		StoreID: storeID, Status: string(f.Status), StockStatus: string(f.StockStatus), DefaultThreshold: model.DefaultLowStockThreshold, QueryPattern: likePattern(f.Query),
	})
}

func checkSellerFilter(f model.SellerFilter) error {
	if f.Status != "" && !f.Status.Valid() {
		return model.Invalid("Status", model.ReasonUnknown)
	}
	if f.StockStatus != "" && f.StockStatus != model.StockLow && f.StockStatus != model.StockOutOfStock {
		return model.Invalid("StockStatus", model.ReasonUnknown)
	}
	return nil
}

// ListAdmin 是後台列表：全站沒刪除的商品（任何狀態），最新建立的在前；total 是符合篩選的總數（不隨游標變）。
func (s *Service) ListAdmin(ctx context.Context, f model.AdminFilter, after *model.ListCursor, size int32) (model.ProductPage, int32, error) {
	size = pageSize(size)
	at, aid := timeCursor(after)
	q := s.q(ctx)
	rows, err := q.ListAdminProducts(ctx, store.ListAdminProductsParams{StoreID: f.StoreID, QueryPattern: likePattern(f.Query), AfterAt: at, AfterID: aid, RowLimit: size + 1})
	if err != nil {
		return model.ProductPage{}, 0, err
	}
	total, err := q.CountAdminProducts(ctx, store.CountAdminProductsParams{StoreID: f.StoreID, QueryPattern: likePattern(f.Query)})
	if err != nil {
		return model.ProductPage{}, 0, err
	}
	out := make([]summaryRow, len(rows))
	for i, r := range rows {
		out[i] = summaryRow(r)
	}
	return finishPage(out, size), total, nil
}

// ListPublic 是公開列表與搜尋：只含 ACTIVE、沒刪除、店家在 at 這一刻可曝光的商品。at 是請求的 clock，不是 client 送的。
// sort 空字串是 LATEST（有關鍵字時 RELEVANCE 要自己指定）；RELEVANCE 沒有關鍵字回 model.Invalid。
func (s *Service) ListPublic(ctx context.Context, at time.Time, f model.PublicFilter, sort model.ProductSort, after *model.ListCursor, size int32) (model.ProductPage, error) {
	if sort == "" {
		sort = model.SortLatest
	}
	switch sort {
	case model.SortLatest, model.SortPriceAsc, model.SortPriceDesc, model.SortSales, model.SortRating:
	case model.SortRelevance:
		if likePattern(f.Query) == "" {
			return model.ProductPage{}, model.Invalid("Sort", model.ReasonInvalid)
		}
	default:
		return model.ProductPage{}, model.Invalid("Sort", model.ReasonUnknown)
	}
	names, err := filterHashtags(f.Hashtags)
	if err != nil {
		return model.ProductPage{}, err
	}
	size = pageSize(size)
	var akey *int64
	var aid *uuid.UUID
	if after != nil {
		akey, aid = &after.Key, &after.ID
	}
	rows, err := s.q(ctx).ListPublicProducts(ctx, store.ListPublicProductsParams{
		Sort: string(sort), Query: trimtext.Trim(f.Query), At: at, CategoryIds: nonNil(f.CategoryIDs), PriceMin: f.PriceMin, PriceMax: f.PriceMax,
		Hashtags: nonNil(names), HashtagAll: f.HashtagAll, StoreIds: nonNil(f.StoreIDs), QueryPattern: likePattern(f.Query),
		AfterKey: akey, AfterID: aid, RowLimit: size + 1,
	})
	if err != nil {
		return model.ProductPage{}, err
	}
	out := make([]summaryRow, len(rows))
	for i, r := range rows {
		out[i] = summaryRow(r)
	}
	return finishPage(out, size), nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// SuggestNames 是搜尋框的商品名稱建議：可見商品的標題含 query，相關度高的在前，已去重，最多 limit 筆（1 到 20，其餘夾在範圍內）。
func (s *Service) SuggestNames(ctx context.Context, at time.Time, query string, limit int32) ([]string, error) {
	pat := likePattern(query)
	if pat == "" {
		return []string{}, nil
	}
	out, err := s.q(ctx).SuggestProductNames(ctx, store.SuggestProductNamesParams{Query: trimtext.Trim(query), At: at, QueryPattern: pat, RowLimit: clamp(limit, 10, 20)})
	return nonNil(out), err
}

func filterHashtags(raw []string) ([]string, error) {
	tags, err := hashtag.RequireSet(raw)
	if err != nil {
		return nil, model.Invalid("Hashtags", model.ReasonInvalid)
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names, nil
}

// SuggestHashtags 是標籤建議：query 可帶開頭的 #；contains 為假是以 query 開頭。回可見商品數多的在前，最多 limit 筆（預設 10，上限 20）。
func (s *Service) SuggestHashtags(ctx context.Context, at time.Time, query string, contains bool, limit int32) ([]model.HashtagCount, error) {
	name, _, ok := hashtag.Normalize(query)
	if !ok {
		return []model.HashtagCount{}, nil
	}
	pat := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(name) + "%"
	if contains {
		pat = "%" + pat
	}
	rows, err := s.q(ctx).SuggestHashtags(ctx, store.SuggestHashtagsParams{At: at, NamePattern: pat, RowLimit: clamp(limit, 10, 20)})
	if err != nil {
		return nil, err
	}
	out := make([]model.HashtagCount, len(rows))
	for i, r := range rows {
		out[i] = model.HashtagCount{Name: r.Name, Display: r.Display, ProductCount: r.ProductCount}
	}
	return out, nil
}

// TrendingHashtags 是熱門標籤：各標籤的可見商品數，多的在前，最多 limit 名（預設 20，上限 50）。
func (s *Service) TrendingHashtags(ctx context.Context, at time.Time, limit int32) ([]model.HashtagCount, error) {
	rows, err := s.q(ctx).TrendingHashtags(ctx, store.TrendingHashtagsParams{At: at, RowLimit: clamp(limit, 20, 50)})
	if err != nil {
		return nil, err
	}
	out := make([]model.HashtagCount, len(rows))
	for i, r := range rows {
		out[i] = model.HashtagCount{Name: r.Name, Display: r.Display, ProductCount: r.ProductCount}
	}
	return out, nil
}

func clamp(n, def, upper int32) int32 {
	switch {
	case n <= 0:
		return def
	case n > upper:
		return upper
	}
	return n
}
