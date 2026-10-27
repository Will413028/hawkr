package grpc

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type marketOverviewUC interface {
	Get(context.Context, int64, usecase.MarketOverviewInput, time.Time) (usecase.MarketOverviewResult, error)
}
type marketWatchUC interface {
	Create(context.Context, int64, string, *string) (domain.MarketWatchTargetResult, error)
	Delete(context.Context, int64, string) error
}

// These USER_SCOPED RPCs accept only trusted, unambiguous seller metadata.
func marketSellerAccount(ctx context.Context) (int64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md.Get("x-account-id")) != 1 || len(md.Get("x-account-type")) != 1 {
		return 0, status.Error(codes.Unauthenticated, "seller identity required")
	}
	parse := func(s string) (int64, error) {
		for _, c := range s {
			if c < '0' || c > '9' {
				return 0, errors.New("invalid identity")
			}
		}
		v, e := strconv.ParseInt(s, 10, 64)
		if e != nil || v <= 0 {
			return 0, errors.New("invalid identity")
		}
		return v, nil
	}
	id, err := parse(md.Get("x-account-id")[0])
	if err != nil {
		return 0, status.Error(codes.Unauthenticated, "invalid account identity")
	}
	role, err := parse(md.Get("x-account-type")[0])
	if err != nil {
		return 0, status.Error(codes.Unauthenticated, "invalid account type")
	}
	if role != int64(accountpb.Role_ROLE_SELLER) && role != int64(accountpb.Role_ROLE_SELLER|accountpb.Role_ROLE_BUYER) {
		return 0, status.Error(codes.PermissionDenied, "seller role required")
	}
	return id, nil
}
func marketOverviewError(err error) error {
	switch {
	case errors.Is(err, usecase.ErrInvalidMarketOverviewInput), errors.Is(err, domain.ErrInvalidWatchTarget):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrWatchLimit):
		return status.Error(codes.ResourceExhausted, err.Error())
	default:
		return status.Errorf(codes.Internal, "market overview: %v", err)
	}
}
func (s *PricebookServer) GetMarketOverview(ctx context.Context, req *pb.GetMarketOverviewRequest) (*pb.GetMarketOverviewResponse, error) {
	accountID, err := marketSellerAccount(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	result, err := s.overviewUC.Get(ctx, accountID, usecase.MarketOverviewInput{Scope: req.Scope, Query: req.Query, WindowDays: req.WindowDays, Page: req.Page, PageSize: req.PageSize, MarketCode: req.MarketCode}, time.Now())
	if err != nil {
		return nil, marketOverviewError(err)
	}
	out := &pb.GetMarketOverviewResponse{SnapshotId: result.SnapshotID, Source: result.Source, GeneratedAt: timeutil.FormatISO8601(result.GeneratedAt), Scope: result.Query.Scope, WindowDays: strconv.Itoa(result.Query.WindowDays), Currency: result.Currency, Unit: result.Unit, Page: strconv.Itoa(result.Query.Page), PageSize: strconv.Itoa(result.Query.PageSize), TotalItems: strconv.FormatInt(result.TotalItems, 10), HasNext: result.HasNext, MarketOptions: make([]*pb.MarketOverviewMarketOption, 0, len(result.MarketOptions)), Items: make([]*pb.MarketOverviewItem, 0, len(result.Items))}
	if result.SourceDataAsOf != nil {
		v := timeutil.FormatISO8601(*result.SourceDataAsOf)
		out.SourceDataAsOf = &v
	}
	if result.Query.MarketCode != "" {
		v := result.Query.MarketCode
		out.MarketCode = &v
	}
	for _, market := range result.MarketOptions {
		out.MarketOptions = append(out.MarketOptions, &pb.MarketOverviewMarketOption{MarketCode: market.MarketCode, MarketName: market.Name})
	}
	for _, item := range result.Items {
		row := &pb.MarketOverviewItem{CropCode: item.CropCode, CropName: item.CropName, VariantName: item.VariantName, MarketCode: item.MarketCode, MarketName: item.MarketName, IsWatched: item.IsWatched, Status: string(item.Status), FreshnessStatus: string(item.Meta.FreshnessStatus), Currency: result.Currency, Unit: result.Unit, QualityReasonCodes: make([]string, 0, len(item.Meta.QualityReasonCodes)), Points: make([]*pb.MarketOverviewPoint, 0, len(item.Points)), LatestActual: marketActualProto(item.LatestActual), PreviousActual: marketActualProto(item.PreviousActual)}
		if item.WatchTargetID != nil {
			v := strconv.FormatInt(*item.WatchTargetID, 10)
			row.WatchTargetId = &v
		}
		if item.Meta.SourceDataAsOf != nil {
			v := timeutil.FormatISO8601(*item.Meta.SourceDataAsOf)
			row.SourceDataAsOf = &v
		}
		for _, reason := range item.Meta.QualityReasonCodes {
			row.QualityReasonCodes = append(row.QualityReasonCodes, string(reason))
		}
		for _, point := range item.Points {
			row.Points = append(row.Points, &pb.MarketOverviewPoint{TradeDate: point.TradeDate.Format(time.DateOnly), AvgPrice: point.AvgPrice})
		}
		if item.Change != nil {
			row.Change = &pb.MarketOverviewChange{Absolute: item.Change.Absolute, Percent: item.Change.Percent, Direction: item.Change.Direction, Basis: codemeta.PricebookChangeBasisLatestVsPreviousObservedTradeDate.Value}
		}
		out.Items = append(out.Items, row)
	}
	return out, nil
}
func marketActualProto(actual *domain.MarketOverviewActual) *pb.MarketOverviewActual {
	if actual == nil {
		return nil
	}
	return &pb.MarketOverviewActual{TradeDate: actual.TradeDate.Format(time.DateOnly), AvgPrice: actual.AvgPrice, VolumeKg: actual.VolumeKg}
}
func (s *PricebookServer) CreateMarketWatchTarget(ctx context.Context, req *pb.CreateMarketWatchTargetRequest) (*pb.CreateMarketWatchTargetResponse, error) {
	accountID, err := marketSellerAccount(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	result, err := s.watchUC.Create(ctx, accountID, req.CropCode, req.MarketCode)
	if err != nil {
		return nil, marketOverviewError(err)
	}
	return &pb.CreateMarketWatchTargetResponse{WatchTargetId: strconv.FormatInt(result.WatchTargetID, 10), Created: result.Created}, nil
}
func (s *PricebookServer) DeleteMarketWatchTarget(ctx context.Context, req *pb.DeleteMarketWatchTargetRequest) (*pb.DeleteMarketWatchTargetResponse, error) {
	accountID, err := marketSellerAccount(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	if err = s.watchUC.Delete(ctx, accountID, req.WatchId); err != nil {
		return nil, marketOverviewError(err)
	}
	return &pb.DeleteMarketWatchTargetResponse{}, nil
}
