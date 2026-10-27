package buyer

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1/buyerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Ports 是買家清單顯示資料的來源。
type Ports struct {
	Products Products
	Stores   Stores
	Orders   Orders
	CRM      CRM
}

type listModule interface {
	ListFavorites(context.Context, model.ListInput) (model.Page, error)
	FavoriteCount(context.Context, int64) (int64, error)
	CheckFavorite(context.Context, int64, int64) (bool, error)
	BatchCheckFavorites(context.Context, int64, []int64) (map[int64]bool, error)
	AddFavorite(context.Context, int64, int64) error
	RemoveFavorite(context.Context, int64, int64) error
	ListFollows(context.Context, model.ListInput) (model.Page, error)
	FollowCount(context.Context, int64) (int64, error)
	CheckFollow(context.Context, int64, int64) (bool, error)
	BatchCheckFollows(context.Context, int64, []int64) (map[int64]bool, error)
	Follow(context.Context, int64, int64) error
	Unfollow(context.Context, int64, int64) error
	ListVisits(context.Context, model.ListInput) (model.Page, error)
	RecordVisit(context.Context, int64, int64) error
	ClearVisits(context.Context, int64) error
	ListRestock(context.Context, model.RestockListInput) (model.RestockPage, error)
	SubscribeRestock(context.Context, int64, int64) error
	UnsubscribeRestock(context.Context, int64, int64) error
	BatchCheckRestock(context.Context, int64, []int64) (map[int64]bool, error)
}

func (h *App) lists() listModule { return h.m }

func encodeCursor(scope string, c *model.Cursor) string {
	if c == nil {
		return ""
	}
	return rpc.TokenFor(scope, c)
}

// storeCards 讀取店家卡片，key 是 store_id；沒有卡片（從沒公開過或不存在）的店家不在 map 裡，client 看到 store 不設。
// 評分由 httpseller.StatsSource 對整批卡片批次讀取一次，讀不到時只有評分不設：不影響卡片，也不轉成 Unavailable。
func (h *App) storeCards(ctx context.Context, storeIDs []int64) (map[int64]*commonpb.StoreCard, error) {
	cards, err := h.ports.Stores.StoreCards(ctx, storeIDs)
	if err != nil {
		return nil, apperr.Unavailable("暫時無法讀取店家")
	}
	out := make(map[int64]*commonpb.StoreCard, len(cards))
	cardIDs := make([]int64, len(cards))
	rated := make([]*commonpb.StoreCard, len(cards))
	for i, c := range cards {
		if rated[i], err = httpseller.ToStoreCard(h.images, c); err != nil {
			return nil, err
		}
		out[c.StoreID], cardIDs[i] = rated[i], c.StoreID
	}
	h.stats.RateCards(ctx, cardIDs, rated)
	return out, nil
}

func ids(items []model.Item) []int64 {
	out := make([]int64, len(items))
	for i, item := range items {
		out[i] = item.ID
	}
	return out
}

func (h *App) productCards(ctx context.Context, buyerID int64, items []model.Item, includeInactive bool) (map[int64]*pb.ProductCard, error) {
	productIDs := ids(items)
	products, err := h.ports.Products.Cards(ctx, productIDs, includeInactive)
	if err != nil {
		return nil, apperr.Unavailable("暫時無法讀取商品")
	}
	storeSet := map[int64]bool{}
	for _, p := range products {
		if p.StoreID <= 0 {
			return nil, apperr.Unavailable("商品的店家資料不完整")
		}
		storeSet[p.StoreID] = true
	}
	storeIDs := make([]int64, 0, len(storeSet))
	for id := range storeSet {
		storeIDs = append(storeIDs, id)
	}
	stores, err := h.storeCards(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	restock, err := h.lists().BatchCheckRestock(ctx, buyerID, productIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*pb.ProductCard, len(productIDs))
	for _, id := range productIDs {
		card := &pb.ProductCard{Id: rpc.FormatID(id)}
		p, exists := products[id]
		if !exists {
			card.Deleted = true
			out[id] = card
			continue
		}
		store := stores[p.StoreID]
		if err := rpc.ObjectKey(p.ImageKey); err != nil {
			return nil, err
		}
		card.Name, card.Image, card.StoreId, card.Store = p.Name, h.images.Image(p.ImageKey), rpc.FormatID(p.StoreID), store
		card.MinPrice, card.MaxPrice, card.Rating = p.MinPrice, p.MaxPrice, p.Rating
		card.Available = p.Available && store.GetStatus() == commonpb.StoreCardStatus_STORE_CARD_STATUS_OPEN
		card.RestockSubscribed = restock[id]
		out[id] = card
	}
	return out, nil
}

func size32(n int64) (int32, error) {
	if n < 0 || n > int64(^uint32(0)>>1) {
		return 0, apperr.FailedPrecondition("清單總數超過對外契約的範圍")
	}
	return int32(n), nil
}

func (h *App) ListFavorites(ctx context.Context, req *connect.Request[pb.ListFavoritesRequest]) (*connect.Response[pb.ListFavoritesResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	s := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListFavoritesProcedure, buyerID)
	after, err := rpc.ValidCursor[model.Cursor](s, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.lists().ListFavorites(ctx, model.ListInput{BuyerID: buyerID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	cards, err := h.productCards(ctx, buyerID, page.Items, true)
	if err != nil {
		return nil, err
	}
	count, err := h.lists().FavoriteCount(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	total, err := size32(count)
	if err != nil {
		return nil, err
	}
	out := &pb.ListFavoritesResponse{Favorites: make([]*pb.Favorite, len(page.Items)), NextPageToken: encodeCursor(s, page.Next), TotalSize: total}
	for i, item := range page.Items {
		out.Favorites[i] = &pb.Favorite{Product: cards[item.ID], CreateTime: timestamppb.New(item.Time)}
	}
	return connect.NewResponse(out), nil
}

func (h *App) CheckFavorite(ctx context.Context, req *connect.Request[pb.CheckFavoriteRequest]) (*connect.Response[pb.CheckFavoriteResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	yes, err := h.lists().CheckFavorite(ctx, buyerID, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CheckFavoriteResponse{Favorited: yes}), nil
}

// batchIDs 只把字串 ID 解析成數字；批次上限是模組的規則（model.BatchIDs），在這裡不重複檢查。
func batchIDs(values []string, field string) ([]int64, error) {
	ids := make([]int64, len(values))
	for i, value := range values {
		id, err := rpc.ParseID(field, value)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

func (h *App) BatchCheckFavorites(ctx context.Context, req *connect.Request[pb.BatchCheckFavoritesRequest]) (*connect.Response[pb.BatchCheckFavoritesResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := batchIDs(req.Msg.GetProductIds(), "product_ids")
	if err != nil {
		return nil, err
	}
	checked, err := h.lists().BatchCheckFavorites(ctx, buyerID, ids)
	if err != nil {
		return nil, err
	}
	out := &pb.BatchCheckFavoritesResponse{}
	for i, id := range ids {
		if checked[id] {
			out.FavoritedProductIds = append(out.FavoritedProductIds, req.Msg.ProductIds[i])
		}
	}
	return connect.NewResponse(out), nil
}

func (h *App) AddFavorite(ctx context.Context, req *connect.Request[pb.AddFavoriteRequest]) (*connect.Response[pb.AddFavoriteResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	if err := h.lists().AddFavorite(ctx, buyerID, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.AddFavoriteResponse{}), nil
}

func (h *App) RemoveFavorite(ctx context.Context, req *connect.Request[pb.RemoveFavoriteRequest]) (*connect.Response[pb.RemoveFavoriteResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	if err := h.lists().RemoveFavorite(ctx, buyerID, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RemoveFavoriteResponse{}), nil
}

func (h *App) ListFollows(ctx context.Context, req *connect.Request[pb.ListFollowsRequest]) (*connect.Response[pb.ListFollowsResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	s := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListFollowsProcedure, buyerID)
	after, err := rpc.ValidCursor[model.Cursor](s, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.lists().ListFollows(ctx, model.ListInput{BuyerID: buyerID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	stores, err := h.storeCards(ctx, ids(page.Items))
	if err != nil {
		return nil, err
	}
	count, err := h.lists().FollowCount(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	total, err := size32(count)
	if err != nil {
		return nil, err
	}
	out := &pb.ListFollowsResponse{Follows: make([]*pb.Follow, len(page.Items)), NextPageToken: encodeCursor(s, page.Next), TotalSize: total}
	for i, item := range page.Items {
		out.Follows[i] = &pb.Follow{StoreId: rpc.FormatID(item.ID), Store: stores[item.ID], CreateTime: timestamppb.New(item.Time)}
	}
	return connect.NewResponse(out), nil
}

func (h *App) CheckFollow(ctx context.Context, req *connect.Request[pb.CheckFollowRequest]) (*connect.Response[pb.CheckFollowResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	yes, err := h.lists().CheckFollow(ctx, buyerID, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CheckFollowResponse{Following: yes}), nil
}

func (h *App) BatchCheckFollows(ctx context.Context, req *connect.Request[pb.BatchCheckFollowsRequest]) (*connect.Response[pb.BatchCheckFollowsResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := batchIDs(req.Msg.GetStoreIds(), "store_ids")
	if err != nil {
		return nil, err
	}
	checked, err := h.lists().BatchCheckFollows(ctx, buyerID, ids)
	if err != nil {
		return nil, err
	}
	out := &pb.BatchCheckFollowsResponse{}
	for i, id := range ids {
		if checked[id] {
			out.FollowedStoreIds = append(out.FollowedStoreIds, req.Msg.StoreIds[i])
		}
	}
	return connect.NewResponse(out), nil
}

func (h *App) FollowStore(ctx context.Context, req *connect.Request[pb.FollowStoreRequest]) (*connect.Response[pb.FollowStoreResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	public, err := h.ports.Stores.PublicStores(ctx, []int64{id})
	if err != nil {
		return nil, apperr.Unavailable("暫時無法查詢店家")
	}
	if len(public) == 0 {
		return nil, apperr.NotFound("找不到公開店家")
	}
	accountID, err := accountCaller(ctx)
	if err != nil {
		return nil, err
	}
	if public[0].AccountID == accountID {
		return nil, apperr.InvalidInput("不能追蹤自己的店家")
	}
	if err := h.lists().Follow(ctx, buyerID, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.FollowStoreResponse{}), nil
}

func (h *App) UnfollowStore(ctx context.Context, req *connect.Request[pb.UnfollowStoreRequest]) (*connect.Response[pb.UnfollowStoreResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	if err := h.lists().Unfollow(ctx, buyerID, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnfollowStoreResponse{}), nil
}

func (h *App) ListVisits(ctx context.Context, req *connect.Request[pb.ListVisitsRequest]) (*connect.Response[pb.ListVisitsResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	s := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListVisitsProcedure, buyerID)
	after, err := rpc.ValidCursor[model.Cursor](s, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.lists().ListVisits(ctx, model.ListInput{BuyerID: buyerID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	cards, err := h.productCards(ctx, buyerID, page.Items, true)
	if err != nil {
		return nil, err
	}
	out := &pb.ListVisitsResponse{Visits: make([]*pb.Visit, len(page.Items)), NextPageToken: encodeCursor(s, page.Next)}
	for i, item := range page.Items {
		out.Visits[i] = &pb.Visit{Product: cards[item.ID], VisitTime: timestamppb.New(item.Time)}
	}
	return connect.NewResponse(out), nil
}

func (h *App) RecordVisit(ctx context.Context, req *connect.Request[pb.RecordVisitRequest]) (*connect.Response[pb.RecordVisitResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	if err := h.lists().RecordVisit(ctx, buyerID, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RecordVisitResponse{}), nil
}

func (h *App) ClearVisits(ctx context.Context, _ *connect.Request[pb.ClearVisitsRequest]) (*connect.Response[pb.ClearVisitsResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.lists().ClearVisits(ctx, buyerID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ClearVisitsResponse{}), nil
}

func (h *App) ListRestockSubscriptions(ctx context.Context, req *connect.Request[pb.ListRestockSubscriptionsRequest]) (*connect.Response[pb.ListRestockSubscriptionsResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	s := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListRestockSubscriptionsProcedure, buyerID)
	after, err := rpc.ValidCursor[model.Cursor](s, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.lists().ListRestock(ctx, model.RestockListInput{BuyerID: buyerID, PageSize: req.Msg.GetPageSize(), After: (*model.RestockCursor)(after)})
	if err != nil {
		return nil, err
	}
	items := make([]model.Item, len(page.Items))
	for i, item := range page.Items {
		items[i] = model.Item{ID: item.ProductID, Time: item.CreatedAt}
	}
	cards, err := h.productCards(ctx, buyerID, items, true)
	if err != nil {
		return nil, err
	}
	var next *model.Cursor
	if page.Next != nil {
		next = &model.Cursor{Time: page.Next.Time, ID: page.Next.ID}
	}
	out := &pb.ListRestockSubscriptionsResponse{RestockSubscriptions: make([]*pb.RestockSubscription, len(items)), NextPageToken: encodeCursor(s, next)}
	for i, item := range items {
		out.RestockSubscriptions[i] = &pb.RestockSubscription{Product: cards[item.ID], CreateTime: timestamppb.New(item.Time)}
	}
	return connect.NewResponse(out), nil
}

func (h *App) SubscribeRestock(ctx context.Context, req *connect.Request[pb.SubscribeRestockRequest]) (*connect.Response[pb.SubscribeRestockResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	if err := h.lists().SubscribeRestock(ctx, buyerID, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SubscribeRestockResponse{}), nil
}

func (h *App) UnsubscribeRestock(ctx context.Context, req *connect.Request[pb.UnsubscribeRestockRequest]) (*connect.Response[pb.UnsubscribeRestockResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	if err := h.lists().UnsubscribeRestock(ctx, buyerID, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnsubscribeRestockResponse{}), nil
}
