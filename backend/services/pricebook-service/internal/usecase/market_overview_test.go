package usecase

import (
	"context"
	"errors"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"testing"
	"time"
)

func TestMarketOverviewQueryValidation(t *testing.T) {
	q, err := validateMarketOverviewInput(MarketOverviewInput{})
	if err != nil || q.WindowDays != 7 || q.Page != 1 || q.PageSize != 20 || q.Scope != "all" {
		t.Fatalf("defaults: %+v %v", q, err)
	}
	for _, v := range []string{"", "0", "-1", "+1", "1.5", " 7", "9223372036854775808"} {
		t.Run(v, func(t *testing.T) {
			if _, err := validateMarketOverviewInput(MarketOverviewInput{Page: &v}); err == nil {
				t.Fatal("invalid page accepted")
			}
		})
	}
	empty := ""
	badWindow := "8"
	bigSize := "101"
	for _, in := range []MarketOverviewInput{{Scope: "other"}, {Scope: "watchlist", MarketCode: &empty}, {WindowDays: &badWindow}, {PageSize: &bigSize}} {
		if _, err := validateMarketOverviewInput(in); err == nil {
			t.Fatalf("accepted %+v", in)
		}
	}
}

type overviewRepoFake struct {
	snapshot  domain.MarketOverviewSnapshot
	err       error
	calls     int
	accountID int64
	cutoff    time.Time
}

func (f *overviewRepoFake) LoadSnapshot(_ context.Context, accountID int64, _ domain.MarketOverviewQuery, cutoff time.Time) (domain.MarketOverviewSnapshot, error) {
	f.calls++
	f.accountID = accountID
	f.cutoff = cutoff
	return f.snapshot, f.err
}
func TestMarketOverviewSnapshotAndMetadata(t *testing.T) {
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	date := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	fake := &overviewRepoFake{snapshot: domain.MarketOverviewSnapshot{Cutoff: now, TotalItems: 21, Items: []domain.MarketOverviewRow{{CropCode: "A1", CropName: "crop", Active: true, LatestActual: &domain.MarketOverviewActual{TradeDate: date, AvgPrice: "0"}, AvailableAt: now.Add(-48 * time.Hour), VolumeWeightFallback: true, SourceDigests: []string{"digest"}}, {CropCode: "B1", Active: false, LatestActual: &domain.MarketOverviewActual{TradeDate: date, AvgPrice: "100"}}}}}
	uc := &MarketOverview{repo: fake, freshness: forecast.DefaultFreshnessConfig()}
	got, err := uc.Get(context.Background(), 123, MarketOverviewInput{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 || fake.accountID != 123 || !fake.cutoff.Equal(now) {
		t.Fatal("not one fixed account snapshot")
	}
	if !got.HasNext || got.Items[0].Status != forecast.PredictionStatusReady || got.Items[0].Meta.FreshnessStatus != forecast.FreshnessStale || got.Items[0].LatestActual.AvgPrice != "0" || len(got.Items[0].Meta.QualityReasonCodes) != 1 {
		t.Fatalf("metadata %+v", got)
	}
	if got.Items[1].Status != forecast.PredictionStatusUnavailable || got.Items[1].LatestActual != nil || got.Items[1].Points == nil {
		t.Fatal("inactive leaks data or missing array")
	}
	want := forecast.SourceDataAsOfUTC(forecast.DateFromTime(date))
	if got.SourceDataAsOf == nil || !got.SourceDataAsOf.Equal(want) {
		t.Fatalf("asof %v", got.SourceDataAsOf)
	}
	again, _ := uc.Get(context.Background(), 123, MarketOverviewInput{}, now)
	other, _ := uc.Get(context.Background(), 124, MarketOverviewInput{}, now)
	if again.SnapshotID != got.SnapshotID || other.SnapshotID == got.SnapshotID {
		t.Fatal("snapshot identity not deterministic/account scoped")
	}
	page := "3"
	empty := &overviewRepoFake{snapshot: domain.MarketOverviewSnapshot{TotalItems: 21}}
	emptyUC := &MarketOverview{repo: empty}
	out, err := emptyUC.Get(context.Background(), 123, MarketOverviewInput{Page: &page}, now)
	if err != nil || out.TotalItems != 21 || out.HasNext || out.Items == nil || out.MarketOptions == nil {
		t.Fatalf("empty out of range: %+v %v", out, err)
	}
}
func TestMarketOverviewFailureAndValidationBeforeIO(t *testing.T) {
	failure := errors.New("database offline")
	repo := &overviewRepoFake{err: failure}
	uc := &MarketOverview{repo: repo}
	if _, err := uc.Get(context.Background(), 123, MarketOverviewInput{Scope: "bad"}, time.Now()); err == nil || repo.calls != 0 {
		t.Fatal("invalid input touched database")
	}
	if _, err := uc.Get(context.Background(), 123, MarketOverviewInput{}, time.Now()); !errors.Is(err, failure) {
		t.Fatalf("lost database failure: %v", err)
	}
}

func TestMarketOverviewOffsetOverflowRejected(t *testing.T) {
	page := "9223372036854775807"
	if _, err := validateMarketOverviewInput(MarketOverviewInput{Page: &page}); err == nil {
		t.Fatal("offset overflow accepted")
	}
	size := "1"
	if _, err := validateMarketOverviewInput(MarketOverviewInput{Page: &page, PageSize: &size}); err != nil {
		t.Fatalf("valid maximum offset rejected: %v", err)
	}
}
