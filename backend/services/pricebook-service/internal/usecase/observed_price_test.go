package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
)

// fakeObservedPriceRepo only implements the 3 domain.ForecastRepository
// methods this usecase actually calls (ObservedPricesByCrop/ByGroup,
// CropGroupsByName). See observedPriceRepoAdapter below / wrapFakeRepo for
// how it's promoted to a full domain.ForecastRepository for the tests.
type fakeObservedPriceRepo struct {
	daily   func(context.Context, []string, string, time.Time, time.Time, time.Time) ([]domain.DailyObservedPrice, error)
	byCrop  func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error)
	byGroup func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error)
	byName  func(ctx context.Context, name string) ([]domain.CropGroupMatch, error)
}

func (f *fakeObservedPriceRepo) ObservedPricesByCrop(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	return f.byCrop(ctx, marketCode, cropCode, from, to, cutoff)
}
func (f *fakeObservedPriceRepo) ObservedPricesByGroup(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	return f.byGroup(ctx, groupCode, marketCode, from, to, cutoff)
}
func (f *fakeObservedPriceRepo) CropGroupsByName(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
	return f.byName(ctx, name)
}

// observedPriceRepoAdapter promotes fakeObservedPriceRepo's 3
// GetObservedPrice-related methods to a full domain.ForecastRepository by
// embedding the interface as nil for the six methods this usecase never
// calls (SeriesObservations/GroupObservations/RestDays/RestDayCoverage/
// CropsInGroup/RecentVolume). The 3 methods declared directly below shadow
// the (nil) promoted ones from the embedded interface; the other 6 stay
// promoted from the nil interface, so calling any of them here panics —
// which is the point, since ObservedPrice.Get must never reach them.
type observedPriceRepoAdapter struct {
	domain.ForecastRepository
	fake *fakeObservedPriceRepo
}

func (a *observedPriceRepoAdapter) DailyObservedPrices(ctx context.Context, crops []string, market string, from, to, cutoff time.Time) ([]domain.DailyObservedPrice, error) {
	return a.fake.daily(ctx, crops, market, from, to, cutoff)
}

func (a *observedPriceRepoAdapter) ObservedPricesByCrop(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	return a.fake.ObservedPricesByCrop(ctx, marketCode, cropCode, from, to, cutoff)
}
func (a *observedPriceRepoAdapter) ObservedPricesByGroup(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	return a.fake.ObservedPricesByGroup(ctx, groupCode, marketCode, from, to, cutoff)
}
func (a *observedPriceRepoAdapter) CropGroupsByName(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
	return a.fake.CropGroupsByName(ctx, name)
}

func wrapFakeRepo(f *fakeObservedPriceRepo) domain.ForecastRepository {
	return &observedPriceRepoAdapter{fake: f}
}

// testSeedWithOneCategoryEntry builds a mapping.Seed with a single row
// mapping alias -> cropGroupName (a crop_group/category match, crop_code
// left empty) via mapping.LoadSeed against a temp CSV/manifest pair — same
// construction pattern as internal/domain/mapping/seed_test.go's
// writeSeedFixture.
func testSeedWithOneCategoryEntry(t *testing.T, alias, cropGroupName string) mapping.Seed {
	t.Helper()
	return testSeedWithCategoryEntries(t, [2]string{alias, cropGroupName})
}

// testSeedWithCategoryEntries is the N-row form of the above: each pair is
// {alias, crop_group_name}, written in the given order (mapping.Seed keeps
// the CSV's row order, so the caller controls which alias the usecase's
// seed loop visits first).
func testSeedWithCategoryEntries(t *testing.T, pairs ...[2]string) mapping.Seed {
	t.Helper()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "seed.csv")
	manifestPath := filepath.Join(dir, "seed.manifest.json")

	content := "alias_name,match_kind,crop_group_name,crop_code,market_code,note\n"
	for _, p := range pairs {
		content += p[0] + ",consumer_alias," + p[1] + ",,,\n"
	}
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write seed csv: %v", err)
	}

	sum := sha256.Sum256([]byte(content))
	manifestBytes, err := json.Marshal(map[string]any{
		"schema":    "hawkr.product-crop-seed/v1",
		"file":      "seed.csv",
		"sha256":    hex.EncodeToString(sum[:]),
		"row_count": len(pairs),
		"sources":   []string{"test fixture"},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatalf("write seed manifest: %v", err)
	}

	seed, err := mapping.LoadSeed(csvPath, manifestPath)
	if err != nil {
		t.Fatalf("LoadSeed: %v", err)
	}
	return seed
}

// testSeedWithOneCropEntry builds a mapping.Seed with a single row mapping
// alias -> cropCode (a crop_code-specific match, crop_group_name left
// empty) — used by the latestPerSeries reduction / pinExact-passthrough
// tests below, which exercise the ObservedPricesByCrop path directly
// (simpler than routing through CropGroupsByName + ObservedPricesByGroup).
func testSeedWithOneCropEntry(t *testing.T, alias, cropCode string) mapping.Seed {
	t.Helper()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "seed.csv")
	manifestPath := filepath.Join(dir, "seed.manifest.json")

	content := "alias_name,match_kind,crop_group_name,crop_code,market_code,note\n" +
		alias + ",consumer_alias,," + cropCode + ",,\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write seed csv: %v", err)
	}

	sum := sha256.Sum256([]byte(content))
	manifestBytes, err := json.Marshal(map[string]any{
		"schema":    "hawkr.product-crop-seed/v1",
		"file":      "seed.csv",
		"sha256":    hex.EncodeToString(sum[:]),
		"row_count": 1,
		"sources":   []string{"test fixture"},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatalf("write seed manifest: %v", err)
	}

	seed, err := mapping.LoadSeed(csvPath, manifestPath)
	if err != nil {
		t.Fatalf("LoadSeed: %v", err)
	}
	return seed
}

// noopForecastUC builds a *Forecast backed by a zero-value fakeForecastRepo
// (every Fn field nil, so every method returns nil/zero-value results per
// forecast_test.go's established guard pattern) — a safe stand-in for
// NewObservedPrice's forecastUC parameter in tests that don't exercise the
// group+history=true branch (which is the only branch that dereferences it;
// see Get's group branch in observed_price.go). A bare literal `nil` would
// work too as long as that branch is never reached, but it makes the failure
// mode of a future test accidentally reaching that branch (e.g. a copy-paste
// with `history` flipped to true) a nil-pointer panic deep inside
// GroupObservedSeries instead of an ordinary wrong-assertion test failure —
// this helper trades that panic for the latter.
func noopForecastUC(t *testing.T) *Forecast {
	t.Helper()
	logger, _ := testLogger()
	return NewForecast(&fakeForecastRepo{}, testConfig(defaultTestInference()), logger)
}

func TestObservedPrice_UnresolvedCropName_ReturnsNotResolved(t *testing.T) {
	seed := testSeedWithOneCategoryEntry(t, "甘藍菜", "甘藍") // alias -> category name
	repo := &fakeObservedPriceRepo{}
	u := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	resolved, obs, meta, err := u.Get(context.Background(), "火龍果", "", "", "", nil, false, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved {
		t.Fatalf("resolved = true, want false for a crop name absent from the seed table")
	}
	if obs != nil {
		t.Fatalf("observations = %+v, want nil when not resolved", obs)
	}

	// resolved=false 是合法業務結果（HTTP 200），不是錯誤——dataMeta 仍要有
	// 意義：freshnessStatus/usabilityStatus 誠實描述「沒東西可看」，不是零值
	// DataMeta{}（那樣 FreshnessStatus 會是空字串，不是任何一個合法值）。
	if meta.FreshnessStatus != forecast.FreshnessUnavailable {
		t.Errorf("DataMeta.FreshnessStatus = %q, want unavailable", meta.FreshnessStatus)
	}
	if meta.UsabilityStatus != forecast.UsabilityUnusable {
		t.Errorf("DataMeta.UsabilityStatus = %q, want unusable", meta.UsabilityStatus)
	}
	if meta.DatasetDigest != nil || meta.SnapshotID != nil || meta.SourceDataAsOf != nil || meta.MaxFeatureAvailableAt != nil {
		t.Errorf("DataMeta optional fields should all be nil when nothing resolved, got %+v", meta)
	}
}

// This is a fakeForecastRepo-backed fixture, not fakeObservedPriceRepo: since
// Task 5's fix, group + history=false + marketCode="" (this test's exact
// shape) aggregates via GroupObservedSeries rather than passing
// ObservedPricesByGroup's raw rows straight through, so exercising it needs
// the same wiring TestObservedPrice_Get_GroupLatestPriceAggregatesNotRawGrid
// uses. A single (market,crop) pair keeps the aggregated value equal to the
// raw AvgPrice, so this test's own concern — CropGroupsByName/
// ObservedPricesByGroup get called with the resolved group code — stays
// decoupled from the two-stage weighting math (covered separately by the
// GroupLatestPriceAggregatesNotRawGrid/GroupHistoryReturnsAggregatedSeriesNotRawGrid
// tests).
func TestObservedPrice_ResolvedCategory_UniqueGroup_ReturnsObservations(t *testing.T) {
	seed := testSeedWithOneCategoryEntry(t, "甘藍菜", "甘藍")
	tradeDate := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			if name != "甘藍" {
				t.Fatalf("CropGroupsByName called with %q, want 甘藍", name)
			}
			return []domain.CropGroupMatch{{GroupCode: "grp-1", Name: "甘藍"}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "A1", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			if groupCode != "grp-1" {
				t.Fatalf("ObservedPricesByGroup called with groupCode=%q, want grp-1", groupCode)
			}
			return []domain.ObservedPrice{{MarketCode: "109", CropCode: "A1", TradeDate: tradeDate, AvgPrice: floatPtr(25.5)}}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: "A1", Volume: 1}}, nil
		},
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	u := NewObservedPrice(repo, seed, testConfig(defaultTestInference()), forecastUC)

	resolved, obs, _, err := u.Get(context.Background(), "甘藍菜", "", "", "", nil, false, tradeDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved {
		t.Fatalf("resolved = false, want true")
	}
	if len(obs) != 1 || obs[0].AvgPrice == nil || *obs[0].AvgPrice != 25.5 {
		t.Fatalf("observations = %+v, want a single aggregated row with AvgPrice=25.5", obs)
	}
}

func TestObservedPrice_ResolvedCategory_AmbiguousGroup_ReturnsError(t *testing.T) {
	seed := testSeedWithOneCategoryEntry(t, "甘藍菜", "甘藍")
	repo := &fakeObservedPriceRepo{
		byName: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: "grp-v"}, {GroupCode: "grp-f"}}, nil
		},
	}
	u := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	_, _, _, err := u.Get(context.Background(), "甘藍菜", "", "", "", nil, false, time.Now())
	if !errors.Is(err, ErrAmbiguousCropGroup) {
		t.Fatalf("err = %v, want ErrAmbiguousCropGroup (ambiguity must surface as an error, never silently pick one — same principle as cmd/pricebook-seed-check)", err)
	}
}

// TestObservedPrice_Get_LatestPerSeries_ReducesPerMarketCropKey exercises
// latestPerSeries's reduction directly through Get (no pinned trade_date):
// the repo returns 3 rows spanning 2 distinct (market,crop) keys, one of
// which has 2 rows on different trade_dates. Get must keep exactly one row
// per key — the most recent trade_date — and must not collapse across
// different keys (proving the map key is (market,crop), not just crop or
// just market).
func TestObservedPrice_Get_LatestPerSeries_ReducesPerMarketCropKey(t *testing.T) {
	seed := testSeedWithOneCropEntry(t, "紅龍果", "A1")
	older := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	otherKey := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)

	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "A1", TradeDate: older, AvgPrice: floatPtr(20)},
				{MarketCode: "109", CropCode: "A1", TradeDate: newer, AvgPrice: floatPtr(22)},
				{MarketCode: "108", CropCode: "A1", TradeDate: otherKey, AvgPrice: floatPtr(18)},
			}, nil
		},
	}
	u := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	resolved, obs, _, err := u.Get(context.Background(), "紅龍果", "109", "", "", nil, false, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved {
		t.Fatalf("resolved = false, want true")
	}
	if len(obs) != 2 {
		t.Fatalf("observations = %+v, want 2 rows (1 per distinct (market,crop) key)", obs)
	}

	byKey := make(map[string]domain.ObservedPrice, len(obs))
	for _, o := range obs {
		byKey[o.MarketCode+"|"+o.CropCode] = o
	}

	got109, ok := byKey["109|A1"]
	if !ok {
		t.Fatalf("observations = %+v, missing market=109/crop=A1 key", obs)
	}
	if !got109.TradeDate.Equal(newer) {
		t.Fatalf("market=109/crop=A1 trade_date = %v, want %v (the more recent of the 2 rows for this key)", got109.TradeDate, newer)
	}
	if *got109.AvgPrice != 22 {
		t.Fatalf("market=109/crop=A1 avg_price = %v, want 22 (must come from the kept — newer — row)", *got109.AvgPrice)
	}

	got108, ok := byKey["108|A1"]
	if !ok {
		t.Fatalf("observations = %+v, missing market=108/crop=A1 key (a distinct key with only 1 row must survive unreduced)", obs)
	}
	if !got108.TradeDate.Equal(otherKey) {
		t.Fatalf("market=108/crop=A1 trade_date = %v, want %v", got108.TradeDate, otherKey)
	}
}

// TestObservedPrice_Get_PinnedTradeDate_SkipsReduction proves the pinExact
// guard actually skips latestPerSeries's reduction when the caller pinned
// an exact trade_date — not just that it happens to produce 1 row per key
// in the pinned case (which would be true even with a buggy always-reduce
// implementation). The fake deliberately returns 2 rows for the same
// (market,crop) key on different trade_dates (an anomaly a real query
// pinned to one day shouldn't produce, but the point is to distinguish
// "reduction skipped" from "reduction happened to be a no-op here") —
// Get must pass through both, unfiltered.
func TestObservedPrice_Get_PinnedTradeDate_SkipsReduction(t *testing.T) {
	seed := testSeedWithOneCropEntry(t, "紅龍果", "B2")
	pinned := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	rowA := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	rowB := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)

	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			if !from.Equal(pinned) || !to.Equal(pinned) {
				t.Fatalf("ObservedPricesByCrop called with from=%v to=%v, want both = pinned trade_date %v", from, to, pinned)
			}
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "B2", TradeDate: rowA, AvgPrice: floatPtr(30)},
				{MarketCode: "109", CropCode: "B2", TradeDate: rowB, AvgPrice: floatPtr(29)},
			}, nil
		},
	}
	u := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	resolved, obs, _, err := u.Get(context.Background(), "紅龍果", "109", "", "", &pinned, false, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved {
		t.Fatalf("resolved = false, want true")
	}
	if len(obs) != 2 {
		t.Fatalf("observations = %+v, want both rows passed through unreduced when trade_date is pinned", obs)
	}
}

// TestObservedPrice_Get_HistoryTrue_SkipsReduction proves the history=true
// flag skips latestPerSeries's reduction the same way a pinned trade_date
// does — same fixture shape as TestObservedPrice_Get_PinnedTradeDate_SkipsReduction
// (2 rows for the same (market,crop) key on different trade_dates), but with
// tradeDate=nil and history=true instead, so the reduction bypass must come
// from history alone.
func TestObservedPrice_Get_HistoryTrue_SkipsReduction(t *testing.T) {
	// 沿用既有 TestObservedPrice_Get_PinnedTradeDate_SkipsReduction 的 fixture
	// 形狀：同一個 (market, crop) key 兩筆不同 trade_date，驗證 history=true
	// 時兩筆都回來（不收斂到只剩最新一筆）。
	seed := testSeedWithOneCropEntry(t, "甘藍-初秋", "A1")
	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, now time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "A1", TradeDate: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)},
				{MarketCode: "109", CropCode: "A1", TradeDate: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)},
			}, nil
		},
	}
	uc := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	resolved, obs, _, err := uc.Get(context.Background(), "甘藍-初秋", "109", "", "", nil, true, time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved {
		t.Fatal("expected resolved=true")
	}
	if len(obs) != 2 {
		t.Fatalf("len(obs) = %d, want 2 (history=true must not collapse to latest-per-series)", len(obs))
	}
}

// TestObservedPrice_Get_HistoryFalse_StillCollapses proves the new history
// parameter is additive: with history=false (the pre-existing default
// behavior), the same fixture that would return 2 rows under history=true
// must still collapse to 1 — confirming this change didn't alter behavior
// for any existing caller that doesn't yet pass history=true.
func TestObservedPrice_Get_HistoryFalse_StillCollapses(t *testing.T) {
	// history=false（預設）必須完全維持既有行為——同一個 fixture，history=false
	// 只回最新一筆，證明這個新參數沒有動到既有呼叫端的行為。
	seed := testSeedWithOneCropEntry(t, "甘藍-初秋", "A1")
	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, now time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "A1", TradeDate: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)},
				{MarketCode: "109", CropCode: "A1", TradeDate: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)},
			}, nil
		},
	}
	uc := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	_, obs, _, err := uc.Get(context.Background(), "甘藍-初秋", "109", "", "", nil, false, time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("len(obs) = %d, want 1 (history=false must still collapse)", len(obs))
	}
}

// TestObservedPrice_Get_PinnedTradeDate_CutoffIsNowNotTo covers review
// Finding I1 (2026-07-31): when trade_date is pinned to a past date, `to`
// (the from/to window bound) collapses to that past date — but cutoff
// (which bounds which revisions are visible "as of now") must stay `now`,
// distinct from `to`. Before this fix, the repository had no cutoff
// parameter at all and derived visibility from to_date, which silently
// excluded rows whose available_at legitimately lags trade_date (see
// db/queries/observation.sql's ObservedPriceByCrop comment, and the
// repository-level TestObservedPricesByCropExcludesRevisionsNotYetVisibleAtCutoff
// for the DB-level proof). This test pins the contract at the usecase
// boundary: Get must call the repository with cutoff=now even when
// from=to=a pinned date far in the past, and a row that only becomes
// visible under cutoff=now (simulating available_at landing after the
// pinned trade_date but at/before now) must come through.
func TestObservedPrice_Get_PinnedTradeDate_CutoffIsNowNotTo(t *testing.T) {
	seed := testSeedWithOneCropEntry(t, "紅龍果", "B2")
	pinned := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) // far in the past
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)   // "today"
	// lateRow simulates a row whose available_at (not modeled in
	// domain.ObservedPrice — filtered server-side by the SQL cutoff) landed
	// after the pinned trade_date but before now — exactly the shape the
	// daily cron's rolling re-ingest window produces for late-arriving data.
	lateRow := domain.ObservedPrice{MarketCode: "109", CropCode: "B2", TradeDate: pinned, AvgPrice: floatPtr(21)}

	var gotCutoff time.Time
	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			if !from.Equal(pinned) || !to.Equal(pinned) {
				t.Fatalf("ObservedPricesByCrop called with from=%v to=%v, want both = pinned trade_date %v", from, to, pinned)
			}
			gotCutoff = cutoff
			return []domain.ObservedPrice{lateRow}, nil
		},
	}
	u := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	resolved, obs, _, err := u.Get(context.Background(), "紅龍果", "109", "", "", &pinned, false, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved {
		t.Fatalf("resolved = false, want true")
	}
	if !gotCutoff.Equal(now) {
		t.Fatalf("cutoff passed to repository = %v, want %v (= now, NOT the pinned trade_date %v — collapsing cutoff to `to` would make this query blind to legitimately-late-arriving data)", gotCutoff, now, pinned)
	}
	if len(obs) != 1 || obs[0].MarketCode != "109" {
		t.Fatalf("observations = %+v, want the late-arriving row to be returned (proves cutoff=now, not to, was actually used)", obs)
	}
}

// TestObservedPrice_Get_LatestPerSeries_SortsDeterministically covers review
// Finding I3 (2026-07-31): latestPerSeries reduces through a Go map keyed by
// MarketCode+"|"+CropCode, and ranges over it to build the result slice — Go
// deliberately randomizes map iteration order, so identical inputs could
// produce differently-ordered output across calls. The consumer (ai-service,
// an LLM prompt builder) needs reproducible ordering for incident-triage
// response diffs. Repeats the call many times: a flaky/random order would
// eventually surface as a mismatch, whereas a real sort is deterministic
// every time.
// This resolves cropName -> a direct crop_code (testSeedWithOneCropEntry),
// not a crop_group name: since Task 5's fix, group + history=false +
// marketCode="" no longer reaches latestPerSeries's map-reduction at all (it
// aggregates via GroupObservedSeries instead — see
// TestObservedPrice_Get_GroupLatestPriceAggregatesNotRawGrid) — so the crop
// path with an explicit market still exercises the raw-series property.
// The synthetic multi-key fixture tests the helper ordering defensively;
// omitted-market requests now have separate aggregate contract tests.
func TestObservedPrice_Get_LatestPerSeries_SortsDeterministically(t *testing.T) {
	seed := testSeedWithOneCropEntry(t, "甘藍菜", "A1")
	date := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			// Deliberately unsorted across 3 distinct (market,crop) keys.
			return []domain.ObservedPrice{
				{MarketCode: "110", CropCode: "B2", TradeDate: date, AvgPrice: floatPtr(1)},
				{MarketCode: "108", CropCode: "C3", TradeDate: date, AvgPrice: floatPtr(2)},
				{MarketCode: "109", CropCode: "A1", TradeDate: date, AvgPrice: floatPtr(3)},
			}, nil
		},
	}
	u := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	wantOrder := []string{"108|C3", "109|A1", "110|B2"}
	for i := 0; i < 20; i++ {
		_, obs, _, err := u.Get(context.Background(), "甘藍菜", "109", "", "", nil, false, time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(obs) != 3 {
			t.Fatalf("iteration %d: observations = %+v, want 3 rows", i, obs)
		}
		for j, want := range wantOrder {
			got := obs[j].MarketCode + "|" + obs[j].CropCode
			if got != want {
				t.Fatalf("iteration %d: order[%d] = %q, want %q (must sort deterministically by (MarketCode, CropCode), not rely on Go map iteration order): full order %v",
					i, j, got, want, obs)
			}
		}
	}
}

// TestObservedPrice_Get_PinnedTradeDate_AlsoSortsDeterministically covers the
// other half of Finding I3: the pinned-trade_date path skips latestPerSeries's
// map-reduction entirely (see TestObservedPrice_Get_PinnedTradeDate_SkipsReduction)
// and passes SQL row order straight through — which must also be sorted for
// consistency with the unpinned path, not left dependent on incidental SQL
// ORDER BY.
func TestObservedPrice_Get_PinnedTradeDate_AlsoSortsDeterministically(t *testing.T) {
	seed := testSeedWithOneCropEntry(t, "紅龍果", "B2")
	pinned := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)

	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			// Deliberately unsorted market order.
			return []domain.ObservedPrice{
				{MarketCode: "110", CropCode: "B2", TradeDate: pinned, AvgPrice: floatPtr(1)},
				{MarketCode: "108", CropCode: "B2", TradeDate: pinned, AvgPrice: floatPtr(2)},
				{MarketCode: "109", CropCode: "B2", TradeDate: pinned, AvgPrice: floatPtr(3)},
			}, nil
		},
	}
	u := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	_, obs, _, err := u.Get(context.Background(), "紅龍果", "109", "", "", &pinned, false, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantOrder := []string{"108|B2", "109|B2", "110|B2"}
	for j, want := range wantOrder {
		got := obs[j].MarketCode + "|" + obs[j].CropCode
		if got != want {
			t.Fatalf("order[%d] = %q, want %q (pinned-date passthrough must also be sorted): full order %v", j, got, want, obs)
		}
	}
}

// TestObservedPrice_Get_HistoryTrue_SortsChronologically covers whole-branch
// review Finding I1 (2026-08-02): before the history=true field existed,
// sortObservedPrices's (MarketCode, CropCode) sort key was effectively
// unique per row (latestPerSeries had already reduced to one row per key, or
// a pinned trade_date meant every row shared the same date). history=true
// makes a third case reachable where MANY rows share the same
// (MarketCode, CropCode) — the full trailing observed_window_days window,
// unreduced — so sort.Slice (not stable) leaves their relative order
// unspecified, discarding the repository's own `ORDER BY ..., trade_date,
// revision_no DESC`. This test feeds 4 same-(market,crop) rows in
// deliberately non-chronological order and asserts the returned slice comes
// back in ascending TradeDate order, proving TradeDate is now a tiebreaker
// after (MarketCode, CropCode).
func TestObservedPrice_Get_HistoryTrue_SortsChronologically(t *testing.T) {
	seed := testSeedWithOneCropEntry(t, "甘藍-初秋", "A1")
	d := func(day int) time.Time { return time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC) }
	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, now time.Time) ([]domain.ObservedPrice, error) {
			// Deliberately non-chronological: neither ascending nor descending.
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "A1", TradeDate: d(22), AvgPrice: floatPtr(4)},
				{MarketCode: "109", CropCode: "A1", TradeDate: d(18), AvgPrice: floatPtr(1)},
				{MarketCode: "109", CropCode: "A1", TradeDate: d(25), AvgPrice: floatPtr(5)},
				{MarketCode: "109", CropCode: "A1", TradeDate: d(20), AvgPrice: floatPtr(2)},
			}, nil
		},
	}
	uc := NewObservedPrice(wrapFakeRepo(repo), seed, testConfig(defaultTestInference()), noopForecastUC(t))

	_, obs, _, err := uc.Get(context.Background(), "甘藍-初秋", "109", "", "", nil, true, d(30))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs) != 4 {
		t.Fatalf("len(obs) = %d, want 4 (history=true must not collapse)", len(obs))
	}
	wantOrder := []time.Time{d(18), d(20), d(22), d(25)}
	for j, want := range wantOrder {
		if !obs[j].TradeDate.Equal(want) {
			t.Fatalf("order[%d].TradeDate = %v, want %v (history=true rows sharing the same (market,crop) key must come back in ascending TradeDate order): full order %+v", j, obs[j].TradeDate, want, obs)
		}
	}
}

// TestObservedPrice_Get_GroupHistoryReturnsAggregatedSeriesNotRawGrid is the
// bug-fix regression test: a crop_group's raw observed-price grid has one row
// per (market, crop, date), and returning that unaggregated for history=true
// zigzags when charted (multiple y-values at the same x/date). Get's group
// branch must instead run the same two-stage volume weighting
// GroupObservedSeries/GroupForecast use, collapsing 2 raw rows (markets A and
// B, same crop, same date) into 1 aggregated point.
//
// Unlike the other tests in this file, this one needs GroupObservedSeries to
// actually run — which additionally calls CropsInGroup/RecentVolume, two
// methods fakeObservedPriceRepo/observedPriceRepoAdapter deliberately don't
// implement (they panic if called, by design — see observedPriceRepoAdapter's
// doc comment). Rather than widen that narrow shadow set, this test reuses
// fakeForecastRepo from forecast_test.go, which already implements every
// domain.ForecastRepository method, and builds a real *Forecast from it to
// pass as NewObservedPrice's new forecastUC parameter.
func TestObservedPrice_Get_GroupHistoryReturnsAggregatedSeriesNotRawGrid(t *testing.T) {
	seed := testSeedWithOneCategoryEntry(t, "小黃瓜", "花胡瓜")
	d1 := forecast.NewDate(2026, 8, 1)
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: "group1", Name: name}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			// Two markets reporting the SAME date — this is exactly the
			// shape that used to zigzag when returned raw.
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30)},
				{MarketCode: "B", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(70)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{
				{MarketCode: "A", CropCode: "crop1", Volume: 1},
				{MarketCode: "B", CropCode: "crop1", Volume: 1},
			}, nil
		},
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	u := NewObservedPrice(repo, seed, testConfig(defaultTestInference()), forecastUC)

	resolved, observations, _, err := u.Get(context.Background(), "小黃瓜", "", "", "", nil, true, d1.ToTime())

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if !resolved {
		t.Fatal("resolved = false, want true")
	}
	if len(observations) != 1 {
		t.Fatalf("len(observations) = %d, want 1 (aggregated to a single point, not the 2 raw rows)", len(observations))
	}
	if observations[0].AvgPrice == nil || *observations[0].AvgPrice != 50 { // (30*1+70*1)/2
		t.Errorf("observations[0].AvgPrice = %v, want 50", observations[0].AvgPrice)
	}
	if observations[0].MarketCode != "" || observations[0].CropCode != "" {
		t.Errorf("observations[0] = %+v, want empty MarketCode/CropCode (an aggregated group row has no single market/crop)", observations[0])
	}
}

// TestObservedPrice_Get_GroupLatestPriceAggregatesNotRawGrid is the
// history=false twin of TestObservedPrice_Get_GroupHistoryReturnsAggregatedSeriesNotRawGrid
// (design spec §2): the DEFAULT "latest price" path for a group must also
// return the two-stage-weighted aggregate, not the raw per-(market,crop)
// grid — this is the path 趨勢總覽's headline price actually calls.
func TestObservedPrice_Get_GroupLatestPriceAggregatesNotRawGrid(t *testing.T) {
	seed := testSeedWithOneCategoryEntry(t, "小黃瓜", "花胡瓜")
	d1 := forecast.NewDate(2026, 8, 1)
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: "group1", Name: name}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30)},
				{MarketCode: "B", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(70)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{
				{MarketCode: "A", CropCode: "crop1", Volume: 1},
				{MarketCode: "B", CropCode: "crop1", Volume: 1},
			}, nil
		},
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	u := NewObservedPrice(repo, seed, testConfig(defaultTestInference()), forecastUC)

	// history=false, no tradeDate pin — this is the "latest price" default.
	resolved, observations, _, err := u.Get(context.Background(), "小黃瓜", "", "", "", nil, false, d1.ToTime())

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if !resolved {
		t.Fatal("resolved = false, want true")
	}
	if len(observations) != 1 {
		t.Fatalf("len(observations) = %d, want 1 (aggregated to a single point, not the 2 raw rows)", len(observations))
	}
	if observations[0].AvgPrice == nil || *observations[0].AvgPrice != 50 { // (30*1+70*1)/2
		t.Errorf("observations[0].AvgPrice = %v, want 50", observations[0].AvgPrice)
	}
}

// --- crop_code / group_code direct-lookup bypass (Task 7) ---
//
// Every bypass test below constructs the usecase with a ZERO-VALUE
// mapping.Seed and passes an empty cropName, so any regression that kept
// routing through seed.Resolve would surface as resolved=false / a panic
// rather than quietly still passing.

func TestObservedPrice_Get_CropCodeBypassSkipsSeedResolution(t *testing.T) {
	var gotMarketCode, gotCropCode string
	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			gotMarketCode, gotCropCode = marketCode, cropCode
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "crop1", TradeDate: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), AvgPrice: floatPtr(12)},
			}, nil
		},
	}
	u := NewObservedPrice(wrapFakeRepo(repo), mapping.Seed{}, testConfig(defaultTestInference()), noopForecastUC(t))

	resolved, obs, _, err := u.Get(context.Background(), "", "109", "crop1", "", nil, false, time.Now())
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if !resolved {
		t.Fatal("resolved = false, want true (a crop_code bypass is resolved by definition)")
	}
	if gotMarketCode != "109" || gotCropCode != "crop1" {
		t.Errorf("ObservedPricesByCrop(market=%q, crop=%q), want (109, crop1)", gotMarketCode, gotCropCode)
	}
	if len(obs) != 1 {
		t.Fatalf("len(obs) = %d, want 1", len(obs))
	}
}

// TestObservedPrice_Get_GroupCodeBypassAggregatesHistory is the group-code
// twin of TestObservedPrice_Get_GroupHistoryReturnsAggregatedSeriesNotRawGrid
// (Task 5): it proves the group_code bypass reaches the SAME aggregation
// path via the shared groupObserved helper, rather than re-implementing a
// raw-grid passthrough that would reintroduce the zigzag bug on the new
// entry point.
func TestObservedPrice_Get_GroupCodeBypassAggregatesHistory(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			t.Fatalf("CropGroupsByName must not be called on the group_code bypass path (name=%q)", name)
			return nil, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30)},
				{MarketCode: "B", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(70)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{
				{MarketCode: "A", CropCode: "crop1", Volume: 1},
				{MarketCode: "B", CropCode: "crop1", Volume: 1},
			}, nil
		},
	}
	logger, _ := testLogger()
	u := NewObservedPrice(repo, mapping.Seed{}, testConfig(defaultTestInference()), NewForecast(repo, testConfig(defaultTestInference()), logger))

	resolved, observations, _, err := u.Get(context.Background(), "", "", "", "group1", nil, true, d1.ToTime())
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if !resolved {
		t.Fatal("resolved = false, want true")
	}
	if len(observations) != 1 {
		t.Fatalf("len(observations) = %d, want 1 (aggregated to a single point, not the 2 raw rows)", len(observations))
	}
	if observations[0].AvgPrice == nil || *observations[0].AvgPrice != 50 { // (30*1+70*1)/2
		t.Errorf("observations[0].AvgPrice = %v, want 50", observations[0].AvgPrice)
	}
}

// TestObservedPrice_Get_GroupCodeBypassWithMarketCode proves the group_code
// bypass entry point now accepts a non-empty marketCode (previously forbidden
// by the ErrMarketCodeNotAllowedForGroup guard in the groupCode != "" branch).
// The aggregation for a single market must still succeed (Task 2 collapsed
// groupObserved to always use GroupObservedSeries regardless of marketCode).
func TestObservedPrice_Get_GroupCodeBypassWithMarketCode(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			t.Fatalf("CropGroupsByName must not be called on the group_code bypass path (name=%q)", name)
			return nil, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}, {CropCode: "crop2", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			if marketCode != "109" {
				t.Fatalf("ObservedPricesByGroup called with marketCode=%q, want \"109\"", marketCode)
			}
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30)},
				{MarketCode: "109", CropCode: "crop2", TradeDate: d1.ToTime(), AvgPrice: floatPtr(70)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{
				{MarketCode: "109", CropCode: "crop1", Volume: 1},
				{MarketCode: "109", CropCode: "crop2", Volume: 1},
			}, nil
		},
	}
	logger, _ := testLogger()
	u := NewObservedPrice(repo, mapping.Seed{}, testConfig(defaultTestInference()), NewForecast(repo, testConfig(defaultTestInference()), logger))

	// Pass marketCode via the group_code bypass: empty cropName, empty cropCode, non-empty groupCode, non-empty marketCode.
	resolved, observations, _, err := u.Get(context.Background(), "", "109", "", "group1", nil, false, d1.ToTime())

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if !resolved {
		t.Fatal("resolved = false, want true (group_code bypass is resolved by definition)")
	}
	if len(observations) != 1 {
		t.Fatalf("len(observations) = %d, want 1 (aggregated to a single point for market 109, not the 2 raw crop rows)", len(observations))
	}
	if observations[0].AvgPrice == nil || *observations[0].AvgPrice != 50 {
		t.Errorf("observations[0].AvgPrice = %v, want 50", observations[0].AvgPrice)
	}
}

func TestObservedPrice_Get_ConflictingResolutionInputsRejected(t *testing.T) {
	u := NewObservedPrice(wrapFakeRepo(&fakeObservedPriceRepo{}), mapping.Seed{}, testConfig(defaultTestInference()), noopForecastUC(t))

	_, _, _, err := u.Get(context.Background(), "", "", "crop1", "group1", nil, false, time.Now())
	if !errors.Is(err, ErrConflictingResolutionInputs) {
		t.Errorf("err = %v, want ErrConflictingResolutionInputs", err)
	}
}

func TestObservedPrice_Get_NoResolutionInputRejected(t *testing.T) {
	u := NewObservedPrice(wrapFakeRepo(&fakeObservedPriceRepo{}), mapping.Seed{}, testConfig(defaultTestInference()), noopForecastUC(t))

	_, _, _, err := u.Get(context.Background(), "", "109", "", "", nil, false, time.Now())
	if !errors.Is(err, ErrNoResolutionInput) {
		t.Errorf("err = %v, want ErrNoResolutionInput", err)
	}
}

// ==================== dataMeta（issue #2249 Slice A） ====================

// TestObservedPrice_DataMeta_CropCodeBypass_SingleRowUsesRowSourceFields
// pins the non-aggregated path's dataMeta wiring: a single returned row's
// own Source/SourceDigest/AvailableAt (repository-level fields, see
// domain.ObservedPrice) drive dataMeta directly — Rule 3's cross-row policy
// collapses to that one digest unchanged, freshnessStatus/usabilityStatus
// come from forecast.FreshnessConfig.Classify fed with that row's
// AvailableAt, and modelVersion stays nil (observed responses never borrow
// a forecast model version).
func TestObservedPrice_DataMeta_CropCodeBypass_SingleRowUsesRowSourceFields(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	availableAt := now.Add(-2 * time.Hour) // well within "fresh" (36h default)
	tradeDate := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	digest := repeatHexTest('7')

	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "crop1", TradeDate: tradeDate, AvgPrice: floatPtr(25.5), Source: "moa", SourceDigest: digest, AvailableAt: availableAt, RevisionNo: 2},
			}, nil
		},
	}
	u := NewObservedPrice(wrapFakeRepo(repo), mapping.Seed{}, testConfig(defaultTestInference()), noopForecastUC(t))

	resolved, obs, meta, err := u.Get(context.Background(), "", "109", "crop1", "", nil, false, now)
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if !resolved || len(obs) != 1 {
		t.Fatalf("resolved=%v obs=%+v, want resolved=true with 1 row", resolved, obs)
	}

	if meta.FreshnessStatus != forecast.FreshnessFresh {
		t.Errorf("DataMeta.FreshnessStatus = %q, want fresh", meta.FreshnessStatus)
	}
	if meta.UsabilityStatus != forecast.UsabilityUsable {
		t.Errorf("DataMeta.UsabilityStatus = %q, want usable", meta.UsabilityStatus)
	}
	wantDigest := "sha256:" + digest
	if meta.DatasetDigest == nil || *meta.DatasetDigest != wantDigest {
		t.Errorf("DataMeta.DatasetDigest = %v, want %q (single row collapses to its own digest, unhashed)", meta.DatasetDigest, wantDigest)
	}
	if meta.SnapshotID == nil || *meta.SnapshotID != wantDigest {
		t.Errorf("DataMeta.SnapshotID = %v, want %q", meta.SnapshotID, wantDigest)
	}
	if meta.MaxFeatureAvailableAt == nil || !meta.MaxFeatureAvailableAt.Equal(availableAt) {
		t.Errorf("DataMeta.MaxFeatureAvailableAt = %v, want %v", meta.MaxFeatureAvailableAt, availableAt)
	}
	wantSourceDataAsOf := forecast.SourceDataAsOfUTC(forecast.DateFromTime(tradeDate))
	if meta.SourceDataAsOf == nil || !meta.SourceDataAsOf.Equal(wantSourceDataAsOf) {
		t.Errorf("DataMeta.SourceDataAsOf = %v, want %v", meta.SourceDataAsOf, wantSourceDataAsOf)
	}
	if meta.ModelVersion != nil {
		t.Errorf("DataMeta.ModelVersion = %v, want nil (observed responses never carry a model version)", meta.ModelVersion)
	}
	if len(meta.QualityReasonCodes) != 0 {
		t.Errorf("DataMeta.QualityReasonCodes = %v, want empty", meta.QualityReasonCodes)
	}
}

// TestObservedPrice_DataMeta_CropCodeBypass_TwoDistinctDigestsUsesComposite
// covers Rule 3's cross-row policy on the crop path's history=true window:
// when the returned rows span more than one distinct source_digest, the
// digest must be the sorted-join-sha256 composite, not an arbitrary row's
// digest.
func TestObservedPrice_DataMeta_CropCodeBypass_TwoDistinctDigestsUsesComposite(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	digestA, digestB := repeatHexTest('a'), repeatHexTest('b')

	repo := &fakeObservedPriceRepo{
		byCrop: func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "crop1", TradeDate: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), AvgPrice: floatPtr(20), SourceDigest: digestA, AvailableAt: now.Add(-3 * time.Hour)},
				{MarketCode: "109", CropCode: "crop1", TradeDate: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC), AvgPrice: floatPtr(21), SourceDigest: digestB, AvailableAt: now.Add(-1 * time.Hour)},
			}, nil
		},
	}
	u := NewObservedPrice(wrapFakeRepo(repo), mapping.Seed{}, testConfig(defaultTestInference()), noopForecastUC(t))

	_, obs, meta, err := u.Get(context.Background(), "", "109", "crop1", "", nil, true, now)
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("len(obs) = %d, want 2 (history=true must not collapse)", len(obs))
	}

	wantDigest := forecast.DatasetDigestFromSourceDigests([]string{digestA, digestB})
	if wantDigest == nil {
		t.Fatal("test oracle DatasetDigestFromSourceDigests unexpectedly returned nil")
	}
	if meta.DatasetDigest == nil || *meta.DatasetDigest != *wantDigest {
		t.Errorf("DataMeta.DatasetDigest = %v, want %v (Rule 3 composite of the 2 distinct digests actually returned)", meta.DatasetDigest, wantDigest)
	}
	// MaxFeatureAvailableAt must be the LATER of the two rows' AvailableAt.
	wantMaxAvailableAt := now.Add(-1 * time.Hour)
	if meta.MaxFeatureAvailableAt == nil || !meta.MaxFeatureAvailableAt.Equal(wantMaxAvailableAt) {
		t.Errorf("DataMeta.MaxFeatureAvailableAt = %v, want %v", meta.MaxFeatureAvailableAt, wantMaxAvailableAt)
	}
}

// TestObservedPrice_DataMeta_GroupHistoryAggregated_UsesContributingRowsSignals
// covers the group-aggregated (history=true) path: the returned rows are
// synthesized aggregate points with no Source/SourceDigest/AvailableAt of
// their own (see groupObserved's own comment), so dataMeta must instead come
// from GroupObservedSeriesOutput's MaxAvailableAt/SourceDigests — derived
// from the raw rows that actually fed the aggregation, not the 9999-priced
// inactive-crop row this fixture deliberately also seeds (mirrors
// TestGroupObservedSeries_InactiveCropExcluded's fixture at the usecase
// layer this method actually calls through).
func TestObservedPrice_DataMeta_GroupHistoryAggregated_UsesContributingRowsSignals(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	activeDigest := repeatHexTest('4')
	activeAvailableAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	inactiveAvailableAt := time.Date(2026, 8, 1, 23, 0, 0, 0, time.UTC) // later, but must not leak in

	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: "group1", Name: name}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}, {CropCode: "crop2", Active: false}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30), SourceDigest: activeDigest, AvailableAt: activeAvailableAt},
				{MarketCode: "A", CropCode: "crop2", TradeDate: d1.ToTime(), AvgPrice: floatPtr(9999), SourceDigest: repeatHexTest('9'), AvailableAt: inactiveAvailableAt},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "A", CropCode: "crop1", Volume: 1}}, nil
		},
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	u := NewObservedPrice(repo, mapping.Seed{}, testConfig(defaultTestInference()), forecastUC)

	_, obs, meta, err := u.Get(context.Background(), "", "", "", "group1", nil, true, d1.ToTime())
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(obs) != 1 || obs[0].AvgPrice == nil || *obs[0].AvgPrice != 30 {
		t.Fatalf("obs = %+v, want a single aggregated point with AvgPrice=30 (inactive crop2 excluded)", obs)
	}

	wantDigest := "sha256:" + activeDigest
	if meta.DatasetDigest == nil || *meta.DatasetDigest != wantDigest {
		t.Errorf("DataMeta.DatasetDigest = %v, want %q (only the active crop's contributing row, inactive crop2 must not leak in)", meta.DatasetDigest, wantDigest)
	}
	if meta.MaxFeatureAvailableAt == nil || !meta.MaxFeatureAvailableAt.Equal(activeAvailableAt) {
		t.Errorf("DataMeta.MaxFeatureAvailableAt = %v, want %v (inactive crop2's later available_at must not leak in)", meta.MaxFeatureAvailableAt, activeAvailableAt)
	}
}

// TestObservedPrice_Get_GroupMarketScoped_AggregatesNotRawGrid proves the
// group + specific-market path (previously the raw-grid fallback, see
// design spec §1a) now blends the group's crops within that one market
// into a single number, the same way the all-markets path already does.
func TestObservedPrice_Get_GroupMarketScoped_AggregatesNotRawGrid(t *testing.T) {
	seed := testSeedWithOneCategoryEntry(t, "小黃瓜", "花胡瓜")
	d1 := forecast.NewDate(2026, 8, 1)
	repo := &fakeForecastRepo{
		cropGroupsByNameFn: func(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
			return []domain.CropGroupMatch{{GroupCode: "group1", Name: name}}, nil
		},
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}, {CropCode: "crop2", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			if marketCode != "109" {
				t.Fatalf("ObservedPricesByGroup called with marketCode=%q, want \"109\"", marketCode)
			}
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30)},
				{MarketCode: "109", CropCode: "crop2", TradeDate: d1.ToTime(), AvgPrice: floatPtr(70)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{
				{MarketCode: "109", CropCode: "crop1", Volume: 1},
				{MarketCode: "109", CropCode: "crop2", Volume: 1},
			}, nil
		},
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(repo, testConfig(defaultTestInference()), logger)
	u := NewObservedPrice(repo, seed, testConfig(defaultTestInference()), forecastUC)

	resolved, observations, _, err := u.Get(context.Background(), "小黃瓜", "109", "", "", nil, false, d1.ToTime())

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if !resolved {
		t.Fatal("resolved = false, want true")
	}
	if len(observations) != 1 {
		t.Fatalf("len(observations) = %d, want 1 (aggregated to a single point for market 109, not the 2 raw crop rows)", len(observations))
	}
	if observations[0].AvgPrice == nil || *observations[0].AvgPrice != 50 {
		t.Errorf("observations[0].AvgPrice = %v, want 50", observations[0].AvgPrice)
	}
}
