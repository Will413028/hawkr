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
