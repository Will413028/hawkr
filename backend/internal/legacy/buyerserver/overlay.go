package buyerserver

import (
	"context"
	"strconv"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/buyer"
)

func positiveID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func (s *Server) CheckFavorite(ctx context.Context, req *connect.Request[pb.CheckFavoriteRequest]) (*connect.Response[pb.CheckFavoriteResponse], error) {
	id, valid := positiveID(req.Msg.GetProductId())
	if !valid {
		return connect.NewResponse(&pb.CheckFavoriteResponse{Code: 400, Message: "product_id 不合法"}), nil
	}
	yes, err := s.buyer.CheckFavorite(ctx, req.Msg.GetBuyerId(), id)
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.CheckFavoriteResponse{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.CheckFavoriteResponse{Favorited: yes}), nil
}

func parseIDs(raw []string) ([]int64, bool) {
	ids := make([]int64, 0, len(raw))
	for _, value := range raw {
		id, valid := positiveID(value)
		if !valid {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func (s *Server) BatchCheckFavorites(ctx context.Context, req *connect.Request[pb.BatchCheckFavoritesRequest]) (*connect.Response[pb.BatchCheckFavoritesResponse], error) {
	ids, valid := parseIDs(req.Msg.GetProductIds())
	if !valid {
		return connect.NewResponse(&pb.BatchCheckFavoritesResponse{Code: 400, Message: "product_id 不合法"}), nil
	}
	flags, err := s.buyer.BatchCheckFavorites(ctx, req.Msg.GetBuyerId(), ids)
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.BatchCheckFavoritesResponse{Code: code, Message: msg}), nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if flags[id] {
			out = append(out, format(id))
		}
	}
	return connect.NewResponse(&pb.BatchCheckFavoritesResponse{FavoritedProductIds: out}), nil
}

func (s *Server) BatchCheckRestockSubscriptions(ctx context.Context, req *connect.Request[pb.BatchCheckRestockSubscriptionsRequest]) (*connect.Response[pb.BatchCheckRestockSubscriptionsResponse], error) {
	ids, valid := parseIDs(req.Msg.GetProductIds())
	if !valid {
		return connect.NewResponse(&pb.BatchCheckRestockSubscriptionsResponse{Code: 400, Message: "product_id 不合法"}), nil
	}
	flags, err := s.buyer.BatchCheckRestock(ctx, req.Msg.GetBuyerId(), ids)
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.BatchCheckRestockSubscriptionsResponse{Code: code, Message: msg}), nil
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[format(id)] = flags[id]
	}
	return connect.NewResponse(&pb.BatchCheckRestockSubscriptionsResponse{Subscribed: out}), nil
}

func (s *Server) GetSubscribersList(ctx context.Context, req *connect.Request[pb.GetSubscribersListRequest]) (*connect.Response[pb.SubscribersListResponse], error) {
	page := req.Msg.GetPage()
	if page < 1 {
		page = 1
	}
	size := req.Msg.GetPageSize()
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	counts, err := s.buyer.SubscriberCounts(ctx, []int64{req.Msg.GetStoreId()})
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.SubscribersListResponse{Code: code, Message: msg}), nil
	}
	if page-1 > 100000/size {
		return connect.NewResponse(&pb.SubscribersListResponse{Code: 400, Message: "粉絲分頁超出範圍"}), nil
	}
	items, err := s.buyer.SubscribersOffset(ctx, req.Msg.GetStoreId(), (page-1)*size, size)
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.SubscribersListResponse{Code: code, Message: msg}), nil
	}
	// 舊 buyer-service 的 SubscriberInfo.id 是 seller_subscriptions 的列 ID。模組的追蹤是複合鍵 (buyer_id, store_id)，
	// 沒有列 ID，所以 id 不填（0），不拿 buyer_id 冒充；
	// 唯一的呼叫方 social-service 只讀 buyer_id（services/social-service/internal/client/buyer/client.go GetSubscriberIDs）。
	out := make([]*pb.SubscriberInfo, 0, len(items))
	for _, item := range items {
		out = append(out, &pb.SubscriberInfo{BuyerId: item.ID, SubscribedAt: item.Time.Unix()})
	}
	return connect.NewResponse(&pb.SubscribersListResponse{Data: out, Total: counts[req.Msg.GetStoreId()], Page: format(int64(page)), PageSize: format(int64(size))}), nil
}
