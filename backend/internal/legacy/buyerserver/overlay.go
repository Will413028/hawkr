package buyerserver

import (
	"context"
	"strconv"
	"uuid"

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

type parsedID struct {
	raw string
	id  uuid.UUID
}

func parseUUIDs(raw []string) []parsedID {
	parsed := make([]parsedID, 0, len(raw))
	for _, value := range raw {
		if id, err := uuid.Parse(value); err == nil && len(value) == 36 {
			parsed = append(parsed, parsedID{raw: value, id: id})
		}
	}
	return parsed
}

func distinct(parsed []parsedID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(parsed))
	ids := make([]uuid.UUID, 0, len(parsed))
	for _, p := range parsed {
		if !seen[p.id] {
			seen[p.id] = true
			ids = append(ids, p.id)
		}
	}
	return ids
}

func (s *Server) CheckFavorite(ctx context.Context, req *connect.Request[pb.CheckFavoriteRequest]) (*connect.Response[pb.CheckFavoriteResponse], error) {
	parsed := parseUUIDs([]string{req.Msg.GetProductId()})
	if len(parsed) == 0 {
		return connect.NewResponse(&pb.CheckFavoriteResponse{}), nil
	}
	yes, err := s.buyer.CheckFavorite(ctx, req.Msg.GetBuyerId(), parsed[0].id)
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.CheckFavoriteResponse{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.CheckFavoriteResponse{Favorited: yes}), nil
}

func (s *Server) BatchCheckFavorites(ctx context.Context, req *connect.Request[pb.BatchCheckFavoritesRequest]) (*connect.Response[pb.BatchCheckFavoritesResponse], error) {
	parsed := parseUUIDs(req.Msg.GetProductIds())
	flags, err := s.buyer.BatchCheckFavorites(ctx, req.Msg.GetBuyerId(), distinct(parsed))
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.BatchCheckFavoritesResponse{Code: code, Message: msg}), nil
	}
	out := make([]string, 0, len(parsed))
	for _, p := range parsed {
		if flags[p.id] {
			out = append(out, p.raw)
		}
	}
	return connect.NewResponse(&pb.BatchCheckFavoritesResponse{FavoritedProductIds: out}), nil
}

func (s *Server) BatchCheckRestockSubscriptions(ctx context.Context, req *connect.Request[pb.BatchCheckRestockSubscriptionsRequest]) (*connect.Response[pb.BatchCheckRestockSubscriptionsResponse], error) {
	raw := req.Msg.GetProductIds()
	parsed := parseUUIDs(raw)
	flags, err := s.buyer.BatchCheckRestock(ctx, req.Msg.GetBuyerId(), distinct(parsed))
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.BatchCheckRestockSubscriptionsResponse{Code: code, Message: msg}), nil
	}
	out := make(map[string]bool, len(raw))
	for _, value := range raw {
		out[value] = false
	}
	for _, p := range parsed {
		out[p.raw] = flags[p.id]
	}
	return connect.NewResponse(&pb.BatchCheckRestockSubscriptionsResponse{Subscribed: out}), nil
}
