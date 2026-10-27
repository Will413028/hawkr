package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// fakeCategoryMappingRepoForRollup is a minimal domain.CategoryMappingRepository
// stub — rollupGroupLayer calls ListResolvedCategoryGroupMappings plus
// ListResolvedCategoryCropVarietyMappings (the per-L3 variety narrowing).
type fakeCategoryMappingRepoForRollup struct {
	domain.CategoryMappingRepository // embed nil; panics if an unexpected method is called
	mappings                         []domain.CategoryGroupMapping
	varieties                        []domain.CategoryCropVarietyMapping
}

func (f *fakeCategoryMappingRepoForRollup) ListResolvedCategoryGroupMappings(ctx context.Context) ([]domain.CategoryGroupMapping, error) {
	return f.mappings, nil
}

func (f *fakeCategoryMappingRepoForRollup) ListResolvedCategoryCropVarietyMappings(ctx context.Context) ([]domain.CategoryCropVarietyMapping, error) {
	return f.varieties, nil
}

func TestRollupBatch_RollupGroupLayer_AggregatesAndFansOutToMultipleCategoryIDs(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	forecastRepo := &fakeForecastRepo{
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

	// Two different L3 categoryIds resolve to the SAME group_code — the
	// aggregation must be computed once and fanned out to both rows, not
	// recomputed (which fakeForecastRepo can't detect directly, but a call
	// counter below proves it).
	callCount := 0
	countingRepo := *forecastRepo
	countingRepo.observedPricesByGroupFn = func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
		callCount++
		return forecastRepo.observedPricesByGroupFn(ctx, groupCode, marketCode, from, to, cutoff)
	}
	forecastUC := NewForecast(&countingRepo, testConfig(defaultTestInference()), logger)

	mappingRepo := &fakeCategoryMappingRepoForRollup{mappings: []domain.CategoryGroupMapping{
		{CategoryID: "cat-a", GroupCode: "group1"},
		{CategoryID: "cat-b", GroupCode: "group1"},
	}}

	batch := NewRollupBatch(forecastUC, nil, forecast.ModelArtifact{}, mappingRepo, nil, nil, testConfig(defaultTestInference()), logger, func() time.Time { return d1.ToTime() })

	results, err := batch.rollupGroupLayer(context.Background(), d1.ToTime())
	if err != nil {
		t.Fatalf("rollupGroupLayer: unexpected error %v", err)
	}
	if callCount != 1 {
		t.Errorf("ObservedPricesByGroup called %d times, want 1 (group1 must be computed once and fanned out)", callCount)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for _, categoryID := range []string{"cat-a", "cat-b"} {
		r, ok := results[categoryID]
		if !ok {
			t.Fatalf("results missing categoryID %q", categoryID)
		}
		if r.Status != domain.RollupStatusReady {
			t.Errorf("%s: Status = %v, want ready", categoryID, r.Status)
		}
		if r.LatestPrice == nil || *r.LatestPrice != 50 {
			t.Errorf("%s: LatestPrice = %v, want 50", categoryID, r.LatestPrice)
		}
		if r.SourceGroupCount != 1 {
			t.Errorf("%s: SourceGroupCount = %d, want 1", categoryID, r.SourceGroupCount)
		}
	}
}

func TestRollupBatch_RollupGroupLayer_NoObservationsIsUnavailable(t *testing.T) {
	forecastRepo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return nil, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return nil, nil
		},
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(forecastRepo, testConfig(defaultTestInference()), logger)
	mappingRepo := &fakeCategoryMappingRepoForRollup{mappings: []domain.CategoryGroupMapping{{CategoryID: "cat-a", GroupCode: "group1"}}}
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	batch := NewRollupBatch(forecastUC, nil, forecast.ModelArtifact{}, mappingRepo, nil, nil, testConfig(defaultTestInference()), logger, func() time.Time { return now })

	results, err := batch.rollupGroupLayer(context.Background(), now)
	if err != nil {
		t.Fatalf("rollupGroupLayer: unexpected error %v", err)
	}
	r, ok := results["cat-a"]
	if !ok {
		t.Fatal("results missing cat-a")
	}
	if r.Status != domain.RollupStatusUnavailable {
		t.Errorf("Status = %v, want unavailable", r.Status)
	}
	if r.LatestPrice != nil {
		t.Errorf("LatestPrice = %v, want nil", r.LatestPrice)
	}
	if r.SourceGroupCount != 0 {
		t.Errorf("SourceGroupCount = %d, want 0", r.SourceGroupCount)
	}
}

// TestRollupBatch_RollupGroupLayer_SkipsFailingGroupAndLogsWarn covers final
// whole-branch review finding 2: computeGroupRollup failing for one group
// (group-bad) must not abort the whole batch — it should be logged and
// skipped, leaving cat-good (whose group succeeded) present in the result
// map and cat-bad absent (mirroring ForecastBatch.Handle's skip-and-continue
// precedent, forecast_batch.go:104-110).
func TestRollupBatch_RollupGroupLayer_SkipsFailingGroupAndLogsWarn(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	forecastRepo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			if groupCode == "group-bad" {
				return nil, fmt.Errorf("simulated transient DB failure")
			}
			return []domain.CropRef{{CropCode: "crop1", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "A", CropCode: "crop1", Volume: 1}}, nil
		},
	}
	logger, logs := testLogger()
	forecastUC := NewForecast(forecastRepo, testConfig(defaultTestInference()), logger)

	mappingRepo := &fakeCategoryMappingRepoForRollup{mappings: []domain.CategoryGroupMapping{
		{CategoryID: "cat-good", GroupCode: "group-good"},
		{CategoryID: "cat-bad", GroupCode: "group-bad"},
	}}

	batch := NewRollupBatch(forecastUC, nil, forecast.ModelArtifact{}, mappingRepo, nil, nil, testConfig(defaultTestInference()), logger, func() time.Time { return d1.ToTime() })

	results, err := batch.rollupGroupLayer(context.Background(), d1.ToTime())
	if err != nil {
		t.Fatalf("rollupGroupLayer: unexpected error %v (a single group's failure must not abort the whole batch)", err)
	}
	if _, ok := results["cat-good"]; !ok {
		t.Error("results missing cat-good — its group (group-good) succeeded and should be present")
	}
	if _, ok := results["cat-bad"]; ok {
		t.Error("results contains cat-bad — its group (group-bad) failed and must be absent, not fabricated")
	}

	found := false
	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, "略過無法計算彙總的大類") {
			found = true
			if got := entry.ContextMap()["group_code"]; got != "group-bad" {
				t.Errorf("group_code = %v, want group-bad", got)
			}
		}
	}
	if !found {
		t.Error("跳過失敗 group 時必須留下可觀察的 Warn log")
	}
}

// fakeCategoryTreeSource is a minimal categoryTreeSource stub returning a
// fixed tree, for rollupCategoryTree tests that don't need real gRPC.
type fakeCategoryTreeSource struct {
	tree *categorypb.GetCategoryTreeResponse
}

func (f *fakeCategoryTreeSource) GetCategoryTree(ctx context.Context, status string, includeDeleted bool) (*categorypb.GetCategoryTreeResponse, error) {
	return f.tree, nil
}

func node(id, parentID string, children ...*categorypb.CategoryNode) *categorypb.CategoryNode {
	return &categorypb.CategoryNode{Id: id, ParentId: parentID, Status: CategoryNodeStatusActive, Children: children}
}

func TestRollupBatch_RollupCategoryTree_AveragesL3IntoL2IntoL1(t *testing.T) {
	// L1 "root" -> L2 "mid" -> L3 "leaf-a" (price 40), L3 "leaf-b" (unavailable)
	tree := &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		node("root", "", node("mid", "root", node("leaf-a", "mid"), node("leaf-b", "mid"))),
	}}
	l3Results := map[string]RollupComputation{
		"leaf-a": readyChild(domain.PriceLayerWholesaleAuction, domain.UnitKilogram, func(c *RollupComputation) {
			c.LatestPrice = floatPtr(40)
			c.PrevPrice = floatPtr(36)
			c.Points = []domain.RollupPricePoint{{TradeDate: "2026-08-01", AvgPrice: 40}}
		}),
		"leaf-b": {Status: domain.RollupStatusUnavailable},
	}
	logger, _ := testLogger()
	batch := NewRollupBatch(nil, nil, forecast.ModelArtifact{}, nil, &fakeRollupRepo{}, &fakeCategoryTreeSource{tree: tree}, testConfig(defaultTestInference()), logger, func() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) })

	rollupRepo := &fakeRollupRepo{}
	batch.rollupRepo = rollupRepo

	written, err := batch.rollupCategoryTree(context.Background(), l3Results, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("rollupCategoryTree: unexpected error %v", err)
	}
	// root, mid, leaf-a, leaf-b — one Upsert per node in the tree.
	if written != 4 {
		t.Errorf("written = %d, want 4", written)
	}

	mid, ok := rollupRepo.written["mid"]
	if !ok {
		t.Fatal("mid (L2) was not written")
	}
	// leaf-b is unavailable and must be excluded, not averaged in as 0 —
	// mid should equal leaf-a's own price exactly (average of ONE ready child).
	if mid.Status != domain.RollupStatusReady || mid.LatestPrice == nil || *mid.LatestPrice != 40 {
		t.Errorf("mid = %+v, want ready/40", mid)
	}
	if mid.SourceGroupCount != 1 {
		t.Errorf("mid.SourceGroupCount = %d, want 1 (only leaf-a is ready)", mid.SourceGroupCount)
	}

	root, ok := rollupRepo.written["root"]
	if !ok {
		t.Fatal("root (L1) was not written")
	}
	if root.Status != domain.RollupStatusReady || root.LatestPrice == nil || *root.LatestPrice != 40 {
		t.Errorf("root = %+v, want ready/40", root)
	}

	leafA, ok := rollupRepo.written["leaf-a"]
	if !ok || leafA.Status != domain.RollupStatusReady || *leafA.LatestPrice != 40 {
		t.Errorf("leaf-a was not written back with its own L3 result: %+v", leafA)
	}
	leafB, ok := rollupRepo.written["leaf-b"]
	if !ok || leafB.Status != domain.RollupStatusUnavailable {
		t.Errorf("leaf-b was not written back as unavailable: %+v", leafB)
	}
}

func TestRollupBatch_RollupCategoryTree_AllChildrenUnavailableIsUnavailable(t *testing.T) {
	tree := &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		node("root", "", node("mid", "root", node("leaf-a", "mid"))),
	}}
	l3Results := map[string]RollupComputation{"leaf-a": {Status: domain.RollupStatusUnavailable}}
	logger, _ := testLogger()
	rollupRepo := &fakeRollupRepo{}
	batch := NewRollupBatch(nil, nil, forecast.ModelArtifact{}, nil, rollupRepo, &fakeCategoryTreeSource{tree: tree}, testConfig(defaultTestInference()), logger, func() time.Time { return time.Time{} })

	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	written, err := batch.rollupCategoryTree(context.Background(), l3Results, now)
	if err != nil {
		t.Fatalf("rollupCategoryTree: unexpected error %v", err)
	}
	// root, mid, leaf-a — one Upsert per node in the tree.
	if written != 3 {
		t.Errorf("written = %d, want 3", written)
	}

	for _, id := range []string{"mid", "root"} {
		got, ok := rollupRepo.written[id]
		if !ok || got.Status != domain.RollupStatusUnavailable || got.LatestPrice != nil || got.SourceGroupCount != 0 {
			t.Errorf("%s = %+v, want unavailable/nil/0", id, got)
		}
	}
}

// TestRollupBatch_Handle_EndToEnd covers final whole-branch review finding 5:
// rollupGroupLayer and rollupCategoryTree are each covered separately above
// (Tasks 7-8), but nothing wires them together through Handle, so the seam
// between them — rollupGroupLayer's output map is exactly what
// rollupCategoryTree/walkNode expects to look up by L3 category_id — was
// untested. This also asserts Level (a DB CHECK-constrained column, part of
// the read API's response) for all three written rows, which none of the
// existing tree-walk tests assert.
func TestRollupBatch_Handle_EndToEnd(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	forecastRepo := &fakeForecastRepo{
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
	forecastUC := NewForecast(forecastRepo, testConfig(defaultTestInference()), logger)

	mappingRepo := &fakeCategoryMappingRepoForRollup{mappings: []domain.CategoryGroupMapping{
		{CategoryID: "e2e-leaf", GroupCode: "e2e-group"},
	}}

	// e2e-root (L1) -> e2e-mid (L2) -> e2e-leaf (L3), a single-child chain so
	// the propagated price at every level must equal the L3 leaf's own price
	// exactly — proving the L3-map-by-category_id lookup wiring, not
	// re-deriving averageChildren's math (already covered by the
	// AveragesL3IntoL2IntoL1 test above).
	tree := &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		node("e2e-root", "", node("e2e-mid", "e2e-root", node("e2e-leaf", "e2e-mid"))),
	}}
	treeSource := &fakeCategoryTreeSource{tree: tree}
	rollupRepo := &fakeRollupRepo{}

	batch := NewRollupBatch(forecastUC, nil, forecast.ModelArtifact{}, mappingRepo, rollupRepo, treeSource, testConfig(defaultTestInference()), logger, func() time.Time { return d1.ToTime() })

	stats, err := batch.Handle(context.Background(), d1.ToTime())
	if err != nil {
		t.Fatalf("Handle: unexpected error %v", err)
	}
	// e2e-leaf, e2e-mid, e2e-root — one Upsert per node in the tree, the
	// real row count Handle must report (final whole-branch review finding 3).
	if stats.CategoriesWritten != 3 {
		t.Errorf("stats.CategoriesWritten = %d, want 3", stats.CategoriesWritten)
	}

	cases := []struct {
		categoryID string
		wantLevel  int16
	}{
		{"e2e-leaf", 3},
		{"e2e-mid", 2},
		{"e2e-root", 1},
	}
	for _, c := range cases {
		row, ok := rollupRepo.written[c.categoryID]
		if !ok {
			t.Fatalf("%s was not written", c.categoryID)
		}
		if row.Level != c.wantLevel {
			t.Errorf("%s: Level = %d, want %d", c.categoryID, row.Level, c.wantLevel)
		}
		if row.Status != domain.RollupStatusReady {
			t.Errorf("%s: Status = %v, want ready", c.categoryID, row.Status)
		}
		// The L3 leaf's observed price (avg of 30/70 = 50) must flow
		// unchanged through L2's single-child average up to L1.
		if row.LatestPrice == nil || *row.LatestPrice != 50 {
			t.Errorf("%s: LatestPrice = %v, want 50", c.categoryID, row.LatestPrice)
		}
	}
}

// fakeRollupRepo is a minimal domain.CategoryPriceRollupRepository stub
// recording every Upsert call by category_id.
type fakeRollupRepo struct {
	written map[string]domain.CategoryPriceRollup
}

func (f *fakeRollupRepo) Upsert(ctx context.Context, r domain.CategoryPriceRollup) error {
	if f.written == nil {
		f.written = map[string]domain.CategoryPriceRollup{}
	}
	f.written[r.CategoryID] = r
	return nil
}

func (f *fakeRollupRepo) Get(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error) {
	r, ok := f.written[categoryID]
	if !ok {
		return nil, nil
	}
	return &r, nil
}

func TestRollupBatch_ComputeGroupForecastPoints_PopulatesFromGroupForecast(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, cropCode, category := "GRP-A", "A1", "N05"

	series109 := testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 0)

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: cropCode, Name: "測試作物", Active: true, CategoryCode: strPtr(category)}}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations("109", cropCode, strPtr(category), originDate.AddDays(-40), originDate, 100, 300, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: cropCode, Volume: 300}}, nil
		},
	}
	logger, _ := testLogger()
	batch := &RollupBatch{
		forecast: NewForecast(repo, testConfig(defaultTestInference()), logger),
		artifact: testArtifact(series109),
		logger:   logger,
	}

	points := batch.computeGroupForecastPoints(context.Background(), groupCode, nil, origin)

	// 這裡只斷言「GroupForecast 的輸出正確流進 RollupForecastPoint」（結構與
	// 欄位對映），不重複驗證 GroupForecast 內部的加權數學——那已經由
	// TestGroupForecast_GroupsObservationsByMarketAndCrop 自己的 P50=125
	// 斷言覆蓋，這裡的職責邊界不同，不重工。
	if len(points) != 7 {
		t.Fatalf("len(points) = %d, want 7 (one per horizon day)", len(points))
	}
	seenHorizons := map[int]bool{}
	for _, p := range points {
		seenHorizons[p.HorizonDays] = true
		if p.TargetDate == "" {
			t.Errorf("D+%d TargetDate 為空字串", p.HorizonDays)
		}
		if p.P50 == nil {
			t.Errorf("D+%d P50 為 nil，want 有值（單一乾淨市場，預期 ready）", p.HorizonDays)
		}
	}
	for h := 1; h <= 7; h++ {
		if !seenHorizons[h] {
			t.Errorf("缺少 horizon %d", h)
		}
	}
}

func TestRollupBatch_ComputeGroupForecastPoints_ReturnsNilOnGroupForecastError(t *testing.T) {
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return nil, errors.New("boom")
		},
	}
	logger, observed := testLogger()
	batch := &RollupBatch{
		forecast: NewForecast(repo, testConfig(defaultTestInference()), logger),
		artifact: testArtifact(),
		logger:   logger,
	}

	points := batch.computeGroupForecastPoints(context.Background(), "GRP-A", nil, time.Now())

	if points != nil {
		t.Errorf("points = %+v, want nil when GroupForecast fails", points)
	}
	if observed.FilterMessageSnippet("略過無法計算七日預測的大類").Len() != 1 {
		t.Error("want exactly one Warn log for the failed group forecast")
	}
}

func TestAverageChildren_ForecastPoints_AveragesOnlyChildrenWithThatHorizon(t *testing.T) {
	p50a, p50b := 40.0, 60.0
	children := []RollupComputation{
		readyChild(domain.PriceLayerWholesaleAuction, domain.UnitKilogram, func(c *RollupComputation) {
			c.ForecastPoints = []domain.RollupForecastPoint{
				{HorizonDays: 1, TargetDate: "2026-09-01", P50: &p50a},
			}
		}),
		readyChild(domain.PriceLayerWholesaleAuction, domain.UnitKilogram, func(c *RollupComputation) {
			c.ForecastPoints = []domain.RollupForecastPoint{
				{HorizonDays: 1, TargetDate: "2026-09-01", P50: &p50b},
				{HorizonDays: 2, TargetDate: "2026-09-02", P50: &p50b},
			}
		}),
	}

	got := averageChildren(children, false)

	if len(got.ForecastPoints) != 2 {
		t.Fatalf("ForecastPoints = %+v, want 2 horizons (1 and 2)", got.ForecastPoints)
	}
	for _, p := range got.ForecastPoints {
		switch p.HorizonDays {
		case 1:
			if p.P50 == nil || *p.P50 != 50.0 { // (40+60)/2
				t.Errorf("horizon 1 P50 = %v, want 50.0 (averaged over both children)", p.P50)
			}
		case 2:
			if p.P50 == nil || *p.P50 != 60.0 { // only child b has horizon 2
				t.Errorf("horizon 2 P50 = %v, want 60.0 (only child with this horizon)", p.P50)
			}
		default:
			t.Errorf("unexpected horizon %d", p.HorizonDays)
		}
	}
}

func TestAverageChildren_ForecastPoints_EmptyWhenNoChildHasAnyForecast(t *testing.T) {
	children := []RollupComputation{
		readyChild(domain.PriceLayerWholesaleAuction, domain.UnitKilogram, nil), // 價格 ready 但沒有 forecast_points
	}

	got := averageChildren(children, false)

	if len(got.ForecastPoints) != 0 {
		t.Errorf("ForecastPoints = %+v, want empty when no child has forecast data", got.ForecastPoints)
	}
}

func readyChild(layer domain.PriceLayer, unit string, mutate func(*RollupComputation)) RollupComputation {
	l, u := layer, unit
	c := RollupComputation{Status: domain.RollupStatusReady, PriceLayer: &l, Unit: &u}
	if mutate != nil {
		mutate(&c)
	}
	return c
}

func TestAverageChildren_SameLayerAndUnit_AveragesAndCarriesLayer(t *testing.T) {
	a, b := 100.0, 110.0
	children := []RollupComputation{
		readyChild(domain.PriceLayerLiveAnimal, domain.UnitKilogram, func(c *RollupComputation) { c.LatestPrice = &a }),
		readyChild(domain.PriceLayerLiveAnimal, domain.UnitKilogram, func(c *RollupComputation) { c.LatestPrice = &b }),
	}

	got := averageChildren(children, false)

	if got.Status != domain.RollupStatusReady {
		t.Fatalf("Status = %q, want ready", got.Status)
	}
	if got.LatestPrice == nil || *got.LatestPrice != 105.0 {
		t.Errorf("LatestPrice = %v, want 105.0", got.LatestPrice)
	}
	if got.PriceLayer == nil || *got.PriceLayer != domain.PriceLayerLiveAnimal {
		t.Errorf("PriceLayer = %v, want live_animal（同層級應原樣往上帶）", got.PriceLayer)
	}
	if got.Unit == nil || *got.Unit != domain.UnitKilogram {
		t.Errorf("Unit = %v, want kg", got.Unit)
	}
}

func TestAverageChildren_DifferentLayers_ReturnsMixedLayerWithoutValues(t *testing.T) {
	pig, chicken := 105.0, 38.0
	children := []RollupComputation{
		readyChild(domain.PriceLayerLiveAnimal, domain.UnitKilogram, func(c *RollupComputation) { c.LatestPrice = &pig }),
		readyChild(domain.PriceLayerFarmgate, domain.UnitKilogram, func(c *RollupComputation) { c.LatestPrice = &chicken }),
	}

	got := averageChildren(children, false)

	if got.Status != domain.RollupStatusMixedLayer {
		t.Fatalf("Status = %q, want mixed_layer", got.Status)
	}
	if got.LatestPrice != nil || got.PrevPrice != nil || len(got.Points) != 0 {
		t.Errorf("mixed_layer 不得帶任何彙總值，got LatestPrice=%v PrevPrice=%v Points=%d",
			got.LatestPrice, got.PrevPrice, len(got.Points))
	}
	if got.PriceLayer != nil || got.Unit != nil {
		t.Errorf("mixed_layer 不得宣稱單一層級／單位，got %v %v", got.PriceLayer, got.Unit)
	}
}

func TestAverageChildren_SameLayerDifferentUnit_ReturnsMixedLayer(t *testing.T) {
	a, b := 38.0, 42.0
	children := []RollupComputation{
		readyChild(domain.PriceLayerFarmgate, domain.UnitKilogram, func(c *RollupComputation) { c.LatestPrice = &a }),
		readyChild(domain.PriceLayerFarmgate, "catty", func(c *RollupComputation) { c.LatestPrice = &b }),
	}

	got := averageChildren(children, false)

	if got.Status != domain.RollupStatusMixedLayer {
		t.Fatalf("Status = %q, want mixed_layer（單位不同不可相加）", got.Status)
	}
}

func TestAverageChildren_MixedChildPropagatesUpwards(t *testing.T) {
	a := 100.0
	children := []RollupComputation{
		readyChild(domain.PriceLayerWholesaleAuction, domain.UnitKilogram, func(c *RollupComputation) { c.LatestPrice = &a }),
	}

	got := averageChildren(children, true)

	if got.Status != domain.RollupStatusMixedLayer {
		t.Fatalf("Status = %q, want mixed_layer（子樹不可彙總則父節點亦然）", got.Status)
	}
	if got.LatestPrice != nil {
		t.Errorf("LatestPrice = %v, want nil", got.LatestPrice)
	}
}

func TestAverageChildren_NoChildren_StaysUnavailable(t *testing.T) {
	got := averageChildren(nil, false)
	if got.Status != domain.RollupStatusUnavailable {
		t.Fatalf("Status = %q, want unavailable", got.Status)
	}
}

func TestAverageChildren_OnlyMixedChildren_IsMixedNotUnavailable(t *testing.T) {
	got := averageChildren(nil, true)
	if got.Status != domain.RollupStatusMixedLayer {
		t.Fatalf("Status = %q, want mixed_layer", got.Status)
	}
}

func TestRollupBatch_RollupGroupLayer_NarrowedCategoryGetsItsOwnVarietyPrice(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	forecastRepo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{
				{CropCode: "vintage", Active: true},
				{CropCode: "premium", Active: true},
			}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "vintage", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30)},
				{MarketCode: "A", CropCode: "premium", TradeDate: d1.ToTime(), AvgPrice: floatPtr(70)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{
				{MarketCode: "A", CropCode: "vintage", Volume: 1},
				{MarketCode: "A", CropCode: "premium", Volume: 1},
			}, nil
		},
	}
	logger, _ := testLogger()
	forecastUC := NewForecast(forecastRepo, testConfig(defaultTestInference()), logger)

	mappingRepo := &fakeCategoryMappingRepoForRollup{
		mappings: []domain.CategoryGroupMapping{
			{CategoryID: "cat-whole", GroupCode: "group1"},
			{CategoryID: "cat-premium", GroupCode: "group1"},
		},
		varieties: []domain.CategoryCropVarietyMapping{
			{CategoryID: "cat-premium", CropCodes: []string{"premium"}},
		},
	}

	batch := NewRollupBatch(forecastUC, nil, forecast.ModelArtifact{}, mappingRepo, nil, nil, testConfig(defaultTestInference()), logger, func() time.Time { return d1.ToTime() })

	results, err := batch.rollupGroupLayer(context.Background(), d1.ToTime())
	if err != nil {
		t.Fatalf("rollupGroupLayer: unexpected error %v", err)
	}
	whole, ok := results["cat-whole"]
	if !ok {
		t.Fatal("results missing cat-whole")
	}
	if whole.LatestPrice == nil || *whole.LatestPrice != 50 {
		t.Errorf("cat-whole LatestPrice = %v, want 50（未收窄＝大類混合）", whole.LatestPrice)
	}
	narrowed, ok := results["cat-premium"]
	if !ok {
		t.Fatal("results missing cat-premium")
	}
	if narrowed.LatestPrice == nil || *narrowed.LatestPrice != 70 {
		t.Errorf("cat-premium LatestPrice = %v, want 70（收窄到 premium 這一個品種）", narrowed.LatestPrice)
	}
}

func TestNarrowToCropCodes_EmptySubsetKeepsWholeGroup(t *testing.T) {
	active := map[string]bool{"a": true, "b": true}
	narrowToCropCodes(active, nil)
	if len(active) != 2 {
		t.Errorf("空 subset 後 len = %d, want 2（空＝整個大類，不是零個）", len(active))
	}
	narrowToCropCodes(active, []string{"b"})
	if len(active) != 1 || !active["b"] {
		t.Errorf("收窄後 = %v, want 只剩 b", active)
	}
	narrowToCropCodes(active, []string{"zzz"})
	if len(active) != 0 {
		t.Errorf("收窄到不存在的 crop 後 = %v, want 空（收窄只能變窄，不能擴大）", active)
	}
}
