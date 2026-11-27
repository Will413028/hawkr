package catalog

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Public 實作 CatalogPublicService：不需要登入，回應不依觀看者（Cache-Control 與 Vary 由 httpapi 的 cachePolicy 與 interceptor 依 service 類別加上）。
// 可不可見、店家可不可曝光都由模組依 now 這個請求的 clock 判斷。
type Public struct {
	m   PublicModule
	p   *Presenter
	now func() time.Time
}

var _ catalogv1alpha1connect.CatalogPublicServiceHandler = (*Public)(nil)

// NewPublic 建立公開讀取的 handler；now 是請求的 clock。
func NewPublic(m PublicModule, p *Presenter, now func() time.Time) (*Public, error) {
	switch {
	case m == nil:
		return nil, fmt.Errorf("httpapi/catalog：需要 catalog 模組")
	case p == nil:
		return nil, fmt.Errorf("httpapi/catalog：需要 Presenter")
	case now == nil:
		return nil, fmt.Errorf("httpapi/catalog：需要 clock")
	}
	return &Public{m: m, p: p, now: now}, nil
}

func (h *Public) ListProducts(ctx context.Context, req *connect.Request[pb.ListProductsRequest]) (*connect.Response[pb.ListProductsResponse], error) {
	msg := req.Msg
	if err := checkPageSize(msg.GetPageSize()); err != nil {
		return nil, err
	}
	if msg.GetSort() == pb.ProductSort_PRODUCT_SORT_RELEVANCE {
		return nil, fieldError("sort", "商品列表不接受相關度排序")
	}
	f, parts, err := publicFilter(msg.GetFilter())
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(catalogv1alpha1connect.CatalogPublicServiceListProductsProcedure, append(parts, msg.GetSort().String())...)
	after, err := decodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	sort := rpc.EnumFromProto[catalog.ProductSort](msg.GetSort())
	page, err := h.m.ListPublicProducts(ctx, h.now(), f, sort, after, msg.GetPageSize())
	if err != nil {
		return nil, failure(err, publicListPaths)
	}
	products, err := h.p.Summaries(ctx, page.Items)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ListProductsResponse{Products: products, NextPageToken: nextToken(scope, page.Next)}), nil
}

func (h *Public) SearchProducts(ctx context.Context, req *connect.Request[pb.SearchProductsRequest]) (*connect.Response[pb.SearchProductsResponse], error) {
	msg := req.Msg
	if err := checkPageSize(msg.GetPageSize()); err != nil {
		return nil, err
	}
	query := normalizeQuery(msg.GetQuery())
	if query == "" {
		return nil, fieldError("query", "請輸入搜尋關鍵字")
	}
	if err := tooLong("query", query, maxSearchRunes); err != nil {
		return nil, err
	}
	f, parts, err := publicFilter(msg.GetFilter())
	if err != nil {
		return nil, err
	}
	f.Query = query
	scope := rpc.FilterScope(catalogv1alpha1connect.CatalogPublicServiceSearchProductsProcedure, append(parts, query, msg.GetSort().String())...)
	after, err := decodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	sort := rpc.EnumFromProto[catalog.ProductSort](msg.GetSort())
	if sort == "" {
		sort = catalog.SortRelevance
	}
	page, err := h.m.ListPublicProducts(ctx, h.now(), f, sort, after, msg.GetPageSize())
	if err != nil {
		return nil, failure(err, publicListPaths)
	}
	products, err := h.p.Summaries(ctx, page.Items)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SearchProductsResponse{Products: products, NextPageToken: nextToken(scope, page.Next)}), nil
}

func (h *Public) SuggestProductNames(ctx context.Context, req *connect.Request[pb.SuggestProductNamesRequest]) (*connect.Response[pb.SuggestProductNamesResponse], error) {
	msg := req.Msg
	if err := checkMaxResults(msg.GetMaxResults()); err != nil {
		return nil, err
	}
	query := normalizeQuery(msg.GetQuery())
	if err := tooLong("query", query, maxSuggestRunes); err != nil {
		return nil, err
	}
	names, err := h.m.SuggestProductNames(ctx, h.now(), query, msg.GetMaxResults())
	if err != nil {
		return nil, failure(err, noPaths)
	}
	return connect.NewResponse(&pb.SuggestProductNamesResponse{Names: names}), nil
}

// GetProduct：ACTIVE 與 ARCHIVED 讀得到（ARCHIVED 的 availability 是不可購買，由 Presenter 依狀態算出）；草稿、已刪除、店家不可曝光都是模組回的不存在。
func (h *Public) GetProduct(ctx context.Context, req *connect.Request[pb.GetProductRequest]) (*connect.Response[pb.GetProductResponse], error) {
	id, err := rpc.ParseUUID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	p, err := h.m.PublicProduct(ctx, id, h.now())
	if err != nil {
		return nil, failure(err, noPaths)
	}
	out, err := h.p.Product(ctx, p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetProductResponse{Product: out}), nil
}

// ListCategories 回整棵分類樹，名稱依請求語系（沒有該語言的名稱時是 zh-TW）。
func (h *Public) ListCategories(ctx context.Context, _ *connect.Request[pb.ListCategoriesRequest]) (*connect.Response[pb.ListCategoriesResponse], error) {
	cats, err := h.m.Categories(ctx)
	if err != nil {
		return nil, failure(err, noPaths)
	}
	return connect.NewResponse(&pb.ListCategoriesResponse{Categories: categoryTree(cats, uitext.LocaleFrom(ctx))}), nil
}

func categoryTree(cats []catalog.Category, loc uitext.Locale) []*pb.Category {
	byParent := map[string][]*pb.Category{}
	var top []*pb.Category
	for _, c := range cats {
		name := c.NameZhTW
		if loc == uitext.EN && c.NameEn != "" {
			name = c.NameEn
		}
		node := &pb.Category{Id: c.ID.String(), Name: name}
		if c.ParentID == nil {
			top = append(top, node)
			continue
		}
		byParent[c.ParentID.String()] = append(byParent[c.ParentID.String()], node)
	}
	for _, t := range top {
		t.Children = byParent[t.Id]
	}
	return top
}

func (h *Public) SuggestHashtags(ctx context.Context, req *connect.Request[pb.SuggestHashtagsRequest]) (*connect.Response[pb.SuggestHashtagsResponse], error) {
	msg := req.Msg
	if err := checkMaxResults(msg.GetMaxResults()); err != nil {
		return nil, err
	}
	tags, err := h.m.SuggestHashtags(ctx, h.now(), msg.GetQuery(), msg.GetContains(), msg.GetMaxResults())
	if err != nil {
		return nil, failure(err, noPaths)
	}
	return connect.NewResponse(&pb.SuggestHashtagsResponse{Hashtags: hashtagCounts(tags)}), nil
}

func (h *Public) SuggestTrendingHashtags(ctx context.Context, req *connect.Request[pb.SuggestTrendingHashtagsRequest]) (*connect.Response[pb.SuggestTrendingHashtagsResponse], error) {
	if err := checkMaxResults(req.Msg.GetMaxResults()); err != nil {
		return nil, err
	}
	tags, err := h.m.TrendingHashtags(ctx, h.now(), req.Msg.GetMaxResults())
	if err != nil {
		return nil, failure(err, noPaths)
	}
	return connect.NewResponse(&pb.SuggestTrendingHashtagsResponse{Hashtags: hashtagCounts(tags)}), nil
}

func hashtagCounts(in []catalog.HashtagCount) []*pb.HashtagCount {
	out := make([]*pb.HashtagCount, len(in))
	for i, t := range in {
		out[i] = &pb.HashtagCount{Name: t.Name, Display: t.Display, ProductCount: t.ProductCount}
	}
	return out
}
