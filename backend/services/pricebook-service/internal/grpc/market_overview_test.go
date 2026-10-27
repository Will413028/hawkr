package grpc

import (
	"context"
	"errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

type overviewHandlerFake struct {
	calls     int
	accountID int64
	input     usecase.MarketOverviewInput
	result    usecase.MarketOverviewResult
	err       error
}

func (f *overviewHandlerFake) Get(_ context.Context, id int64, in usecase.MarketOverviewInput, _ time.Time) (usecase.MarketOverviewResult, error) {
	f.calls++
	f.accountID = id
	f.input = in
	return f.result, f.err
}

type watchHandlerFake struct {
	calls         int
	accountID     int64
	crop, deleted string
	market        *string
	err           error
}

func (f *watchHandlerFake) Create(_ context.Context, id int64, crop string, market *string) (domain.MarketWatchTargetResult, error) {
	f.calls++
	f.accountID = id
	f.crop = crop
	f.market = market
	return domain.MarketWatchTargetResult{WatchTargetID: 42, Created: false}, f.err
}
func (f *watchHandlerFake) Delete(_ context.Context, id int64, watch string) error {
	f.calls++
	f.accountID = id
	f.deleted = watch
	return f.err
}
func TestMarketRPCMetadataRejectsAmbiguousOrSpoofedIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		md   metadata.MD
		code codes.Code
	}{
		{"absent", nil, codes.Unauthenticated},
		{"missing_role", metadata.Pairs("x-account-id", "123"), codes.Unauthenticated},
		{"duplicate_account", metadata.Pairs("x-account-id", "123", "x-account-id", "456", "x-account-type", "2"), codes.Unauthenticated},
		{"duplicate_role", metadata.Pairs("x-account-id", "123", "x-account-type", "2", "x-account-type", "1"), codes.Unauthenticated},
		{"negative", metadata.Pairs("x-account-id", "-123", "x-account-type", "2"), codes.Unauthenticated},
		{"plus", metadata.Pairs("x-account-id", "+123", "x-account-type", "2"), codes.Unauthenticated},
		{"overflow", metadata.Pairs("x-account-id", "9223372036854775808", "x-account-type", "2"), codes.Unauthenticated},
		{"bad_role", metadata.Pairs("x-account-id", "123", "x-account-type", "seller"), codes.Unauthenticated},
		{"buyer", metadata.Pairs("x-account-id", "123", "x-account-type", "1"), codes.PermissionDenied},
		{"unknown_role", metadata.Pairs("x-account-id", "123", "x-account-type", "99"), codes.PermissionDenied},
		{"admin", metadata.Pairs("x-account-id", "123", "x-account-type", "4"), codes.PermissionDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := &overviewHandlerFake{}
			w := &watchHandlerFake{}
			s := &PricebookServer{overviewUC: o, watchUC: w}
			ctx := metadata.NewIncomingContext(context.Background(), tc.md)
			_, e1 := s.GetMarketOverview(ctx, &pb.GetMarketOverviewRequest{})
			_, e2 := s.CreateMarketWatchTarget(ctx, &pb.CreateMarketWatchTargetRequest{CropCode: "A1"})
			_, e3 := s.DeleteMarketWatchTarget(ctx, &pb.DeleteMarketWatchTargetRequest{WatchId: "42"})
			for _, err := range []error{e1, e2, e3} {
				if status.Code(err) != tc.code {
					t.Fatalf("got %v want %v", err, tc.code)
				}
			}
			if o.calls != 0 || w.calls != 0 {
				t.Fatal("unauthorized actor reached usecase")
			}
		})
	}
	outgoing := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-account-id", "123", "x-account-type", "2"))
	if _, err := marketSellerAccount(outgoing); status.Code(err) != codes.Unauthenticated {
		t.Fatal("outgoing metadata trusted")
	}
}
func TestMarketRPCPreservesIdentityPresenceAndNumericStrings(t *testing.T) {
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	id := int64(9007199254740993)
	volume := "12345678901234567890.123456789"
	o := &overviewHandlerFake{result: usecase.MarketOverviewResult{SnapshotID: "sha256:test", GeneratedAt: now, Query: domain.MarketOverviewQuery{Scope: "all", WindowDays: 7, Page: 1, PageSize: 20}, TotalItems: 0, Items: []usecase.MarketOverviewItem{{MarketOverviewRow: domain.MarketOverviewRow{WatchTargetID: &id, CropCode: "A1", LatestActual: &domain.MarketOverviewActual{TradeDate: now, AvgPrice: "0.000000000000000001", VolumeKg: &volume}, Change: &domain.MarketOverviewChange{Absolute: "1", Direction: "up"}}, Status: forecast.PredictionStatusReady, Meta: usecase.DataMeta{FreshnessStatus: forecast.FreshnessFresh}}}}}
	w := &watchHandlerFake{}
	s := &PricebookServer{overviewUC: o, watchUC: w}
	for _, role := range []string{"2", "3"} {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-account-id", "123", "x-account-type", role))
		empty := ""
		res, err := s.GetMarketOverview(ctx, &pb.GetMarketOverviewRequest{Page: &empty, MarketCode: &empty})
		if err != nil || o.accountID != 123 || o.input.Page == nil || *o.input.Page != "" || o.input.MarketCode == nil {
			t.Fatalf("presence/identity: %v", err)
		}
		if res.TotalItems != "0" || res.MarketOptions == nil || res.Items[0].Points == nil || res.Items[0].QualityReasonCodes == nil || res.Items[0].GetWatchTargetId() != "9007199254740993" || res.Items[0].LatestActual.AvgPrice != "0.000000000000000001" || res.Items[0].LatestActual.GetVolumeKg() != volume || res.Items[0].Change.Percent != nil {
			t.Fatalf("lossy mapping %+v", res)
		}
		create, err := s.CreateMarketWatchTarget(ctx, &pb.CreateMarketWatchTargetRequest{CropCode: "A1", MarketCode: &empty})
		if err != nil || create.WatchTargetId != "42" || create.Created || w.accountID != 123 || w.market == nil {
			t.Fatal("create mapping")
		}
		if _, err := s.DeleteMarketWatchTarget(ctx, &pb.DeleteMarketWatchTargetRequest{WatchId: "42"}); err != nil || w.accountID != 123 || w.deleted != "42" {
			t.Fatal("delete mapping")
		}
	}
}
func TestMarketRPCErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{{domain.ErrInvalidWatchTarget, codes.InvalidArgument}, {usecase.ErrInvalidMarketOverviewInput, codes.InvalidArgument}, {domain.ErrWatchLimit, codes.ResourceExhausted}, {errors.New("db failed"), codes.Internal}} {
		if got := status.Code(marketOverviewError(tc.err)); got != tc.code {
			t.Fatalf("%v: %v", tc.err, got)
		}
	}
}
