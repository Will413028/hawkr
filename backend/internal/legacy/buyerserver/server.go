// Package buyerserver exposes the BuyerService methods still used by legacy services.
package buyerserver

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/buyer"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/buyer/buyerconnect"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

type Server struct {
	buyerconnect.UnimplementedBuyerServiceHandler
	buyer *buyer.Module
	now   func() time.Time
	log   *slog.Logger
}

var _ buyerconnect.BuyerServiceHandler = (*Server)(nil)

func New(b *buyer.Module) *Server {
	return &Server{buyer: b, now: time.Now, log: slog.Default()}
}

func (s *Server) logger() *slog.Logger {
	if s.log != nil {
		return s.log
	}
	return slog.Default()
}

func failure(err error) (int32, string, error) {
	switch apperr.KindOf(err) {
	case apperr.KindNotFound:
		return 404, err.Error(), nil
	case apperr.KindInvalidInput:
		return 400, err.Error(), nil
	case apperr.KindFailedPrecondition:
		return 9203, err.Error(), nil
	case apperr.KindInternal, apperr.KindVersionConflict, apperr.KindAlreadyExists,
		apperr.KindUnauthenticated, apperr.KindPermissionDenied, apperr.KindUnavailable,
		apperr.KindResourceExhausted:
		return 0, "", err
	default:
		return 0, "", err
	}
}

func format(id int64) string { return strconv.FormatInt(id, 10) }
func optional(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func profile(p buyer.Profile) *pb.BuyerInfo {
	info := &pb.BuyerInfo{Id: format(p.BuyerID), AccountId: format(p.AccountID), Nickname: p.DisplayName,
		AvatarKey: optional(p.Avatar), City: optional(p.City), Job: optional(p.Job), Phone: optional(p.Phone),
		CreatedAt: format(p.CreatedAt.Unix()), UpdatedAt: format(p.UpdatedAt.Unix())}
	if p.BirthDate != nil {
		info.BirthDate = p.BirthDate.Format("2006-01-02")
	}
	if p.Gender != nil {
		info.Gender = string(*p.Gender)
	}
	return info
}
func (s *Server) address(ctx context.Context, a buyer.Address) (*pb.AddressInfo, error) {
	town, ok := twaddr.TownByCode(a.TownCode)
	if !ok {
		s.logger().ErrorContext(ctx, "buyerserver：未知鄉鎮市區代碼", "town_code", a.TownCode, "address_id", a.AddressID)
		return nil, connect.NewError(connect.CodeInternal, errors.New("未知鄉鎮市區代碼"))
	}
	return &pb.AddressInfo{
		Id:            format(a.AddressID),
		BuyerId:       format(a.BuyerID),
		ReceiverName:  a.ReceiverName,
		ReceiverPhone: a.ReceiverPhone,
		City:          town.CountyName,
		District:      town.Name,
		DetailAddress: a.AddressDetail,
		IsDefault:     a.IsDefault,
	}, nil
}

func (s *Server) GetBuyerProfile(ctx context.Context, req *connect.Request[pb.GetBuyerProfileRequest]) (*connect.Response[pb.BuyerProfileResponse], error) {
	p, err := s.buyer.Profile(ctx, req.Msg.GetAccountId())
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.BuyerProfileResponse{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.BuyerProfileResponse{Data: profile(p)}), nil
}

func (s *Server) GetAddress(ctx context.Context, req *connect.Request[pb.GetAddressRequest]) (*connect.Response[pb.GetAddressResponse], error) {
	a, err := s.buyer.GetAddress(ctx, req.Msg.GetBuyerId(), req.Msg.GetAddressId())
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.GetAddressResponse{Code: code, Message: msg}), nil
	}
	info, err := s.address(ctx, a)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetAddressResponse{Data: info}), nil
}

// ValidateAddress 回業務碼 9700（地址查詢停用，決定 9a 移除 TGOS）。Java shipping 的 UberQuoteService 只把
// code != 0 轉成「Uber 不可用」；不實作會變成 gRPC UNIMPLEMENTED，整個報價失敗。shipping 搬進單體或接上地理編碼時刪除。
func (s *Server) ValidateAddress(context.Context, *connect.Request[pb.ValidateAddressRequest]) (*connect.Response[pb.ValidateAddressResponse], error) {
	return connect.NewResponse(&pb.ValidateAddressResponse{Code: 9700, Message: "地址查詢已停用"}), nil
}

func (s *Server) ListFavoritedProductIds(ctx context.Context, req *connect.Request[pb.ListFavoritedProductIdsRequest]) (*connect.Response[pb.ListFavoritedProductIdsResponse], error) {
	ids, err := s.buyer.FavoritedProductIDs(ctx, req.Msg.GetBuyerId())
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.ListFavoritedProductIdsResponse{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.ListFavoritedProductIdsResponse{ProductIds: ids}), nil
}

func (s *Server) ListSubscribedSellerIds(ctx context.Context, req *connect.Request[pb.ListSubscribedSellerIdsRequest]) (*connect.Response[pb.ListSubscribedSellerIdsResponse], error) {
	ids, err := s.buyer.FollowedStoreIDs(ctx, req.Msg.GetBuyerId())
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.ListSubscribedSellerIdsResponse{Code: code, Message: msg}), nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = format(id)
	}
	return connect.NewResponse(&pb.ListSubscribedSellerIdsResponse{StoreIds: out}), nil
}

func (s *Server) BatchGetBuyerDisplayByBuyerIDs(ctx context.Context, req *connect.Request[pb.BatchGetBuyerDisplayByBuyerIDsRequest]) (*connect.Response[pb.BatchGetBuyerDisplayByBuyerIDsResponse], error) {
	rows, err := s.buyer.BatchDisplay(ctx, req.Msg.GetBuyerIds())
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.BatchGetBuyerDisplayByBuyerIDsResponse{Code: code, Message: msg}), nil
	}
	out := make(map[int64]*pb.BuyerDisplay, len(rows))
	for _, row := range rows {
		name := optional(row.DisplayName)
		if row.Deleted {
			name = "已刪除帳號"
		}
		out[row.BuyerID] = &pb.BuyerDisplay{DisplayName: name, AvatarKey: optional(row.Avatar), Deleted: row.Deleted}
	}
	return connect.NewResponse(&pb.BatchGetBuyerDisplayByBuyerIDsResponse{Buyers: out}), nil
}

func (s *Server) ResolvePrincipals(ctx context.Context, req *connect.Request[pb.ResolvePrincipalsRequest]) (*connect.Response[pb.ResolvePrincipalsResponse], error) {
	resolved, err := s.buyer.ResolvePrincipals(ctx, req.Msg.GetIds())
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.ResolvePrincipalsResponse{Code: code, Message: msg}), nil
	}
	unresolved := make([]int64, 0)
	for _, id := range req.Msg.GetIds() {
		if _, ok := resolved[id]; !ok {
			unresolved = append(unresolved, id)
		}
	}
	return connect.NewResponse(&pb.ResolvePrincipalsResponse{Principals: resolved, UnresolvedIds: unresolved}), nil
}

func (s *Server) ListBuyerIDsByAccount(ctx context.Context, req *connect.Request[pb.ListBuyerIDsByAccountRequest]) (*connect.Response[pb.ListBuyerIDsByAccountResponse], error) {
	ids, err := s.buyer.BuyerIDsByAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.ListBuyerIDsByAccountResponse{Code: code, Message: msg}), nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = format(id)
	}
	return connect.NewResponse(&pb.ListBuyerIDsByAccountResponse{BuyerIds: out}), nil
}

func (s *Server) GetBuyerEntitlement(ctx context.Context, req *connect.Request[pb.GetBuyerEntitlementRequest]) (*connect.Response[pb.BuyerEntitlement], error) {
	id, valid := positiveID(req.Msg.GetBuyerId())
	if !valid {
		return connect.NewResponse(&pb.BuyerEntitlement{Code: 400, Message: "buyer_id 不合法"}), nil
	}
	e, err := s.buyer.Entitlement(ctx, id)
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, fatal
		}
		return connect.NewResponse(&pb.BuyerEntitlement{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.BuyerEntitlement{PlanCode: e.PlanCode, MonthlyAnswerLimit: format(int64(e.MonthlyAnswerLimit)), PeriodStart: e.PeriodStart.UTC().Format(time.RFC3339), PeriodEnd: e.PeriodEnd.UTC().Format(time.RFC3339)}), nil
}
