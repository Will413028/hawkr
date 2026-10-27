package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"

	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// RollupComputation is one category_id's computed rollup, shared in-memory
// between rollupGroupLayer (L3) and rollupCategoryTree (L2/L1, Task 8) so
// the latter reads L3 results straight from this map instead of re-querying
// the row rollupGroupLayer already wrote in the same batch run.
type RollupComputation struct {
	Status           domain.RollupStatus
	LatestPrice      *float64
	PrevPrice        *float64
	Points           []domain.RollupPricePoint
	ForecastPoints   []domain.RollupForecastPoint
	SourceGroupCount int16
	PriceLayer       *domain.PriceLayer
	Unit             *string
}

// RollupBatch is the daily category_price_rollup cron batch: L3 first
// (rollupGroupLayer, this task), then a bottom-up L2/L1 walk of the
// product-service category tree (rollupCategoryTree, Task 8).
type RollupBatch struct {
	forecast           *Forecast
	commodityForecast  *CommodityForecast
	artifact           forecast.ModelArtifact
	mappingRepo        domain.CategoryMappingRepository
	rollupRepo         domain.CategoryPriceRollupRepository
	categorySource     categoryTreeSource
	observedWindowDays int
	logger             *zap.Logger
	now                func() time.Time
}

func NewRollupBatch(
	forecastUC *Forecast,
	commodityForecastUC *CommodityForecast,
	artifact forecast.ModelArtifact,
	mappingRepo domain.CategoryMappingRepository,
	rollupRepo domain.CategoryPriceRollupRepository,
	categorySource categoryTreeSource,
	cfg *config.Config,
	logger *zap.Logger,
	now func() time.Time,
) *RollupBatch {
	return &RollupBatch{
		forecast: forecastUC, commodityForecast: commodityForecastUC,
		artifact: artifact, mappingRepo: mappingRepo, rollupRepo: rollupRepo,
		categorySource: categorySource, observedWindowDays: cfg.Inference.ObservedWindowDays,
		logger: logger, now: now,
	}
}

// rollupGroupLayer computes one RollupComputation per resolved L3
// category_id, narrowed to that L3's own crop varieties where
// category_crop_mapping_crop says which ones they are.
//
// A per-group failure from computeGroupRollup is logged and skipped, not
// treated as batch-fatal: unlike ListResolvedCategoryGroupMappings itself
// failing (nothing to iterate over, still returned as an error below), a
// single group's aggregation failing here (e.g. a transient DB error on one
// GroupObservedSeries query) shouldn't take down every other group's rollup
// for the day. This mirrors ForecastBatch.Handle's skip-and-continue
// precedent (forecast_batch.go, "單一 group 呼叫 GroupForecast 失敗...只跳過
// 該 group、記 Warn log，不中止整個 Handle") rather than the original
// abort-on-first-error behavior. A category_id whose group failed this run
// is simply absent from the returned map — walkNode already treats an
// absent L3 result as unavailable, so this composes correctly with the
// existing fail-closed behavior.
func (b *RollupBatch) rollupGroupLayer(ctx context.Context, origin time.Time) (map[string]RollupComputation, error) {
	mappings, err := b.mappingRepo.ListResolvedCategoryGroupMappings(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出 resolved category-group mapping 失敗: %w", err)
	}

	varieties, err := b.mappingRepo.ListResolvedCategoryCropVarietyMappings(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出 resolved category-crop 品種收窄失敗: %w", err)
	}
	narrowed := make(map[string][]string, len(varieties))
	for _, v := range varieties {
		narrowed[v.CategoryID] = v.CropCodes
	}

	type rollupKey struct {
		groupCode string
		cropSig   string // 收窄的 crop 集合簽章；空字串＝整個大類
	}
	byKey := map[rollupKey][]string{}   // key -> []category_id
	cropsOf := map[rollupKey][]string{} // key -> 該 key 的 crop 集合
	for _, m := range mappings {
		codes := narrowed[m.CategoryID]
		k := rollupKey{groupCode: m.GroupCode, cropSig: strings.Join(codes, "\x00")}
		byKey[k] = append(byKey[k], m.CategoryID)
		cropsOf[k] = codes
	}

	from := origin.AddDate(0, 0, -b.observedWindowDays)
	results := make(map[string]RollupComputation, len(mappings))
	for k, categoryIDs := range byKey {
		computation, err := b.computeGroupRollup(ctx, k.groupCode, cropsOf[k], from, origin)
		if err != nil {
			b.logger.Warn("rollup_batch: 略過無法計算彙總的大類",
				zap.String("group_code", k.groupCode),
				zap.Strings("crop_codes", cropsOf[k]), zap.Error(err))
			continue
		}
		for _, categoryID := range categoryIDs {
			results[categoryID] = computation
		}
	}
	return results, nil
}

func (b *RollupBatch) rollupCommodityLayer(ctx context.Context, origin time.Time) (map[string]RollupComputation, error) {
	if b.commodityForecast == nil {
		return map[string]RollupComputation{}, nil
	}
	mappings, err := b.mappingRepo.ListResolvedCategoryCommodityMappings(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出 resolved category-commodity mapping 失敗: %w", err)
	}

	byCommodity := map[int64][]string{} // commodity_id -> []category_id
	for _, m := range mappings {
		byCommodity[m.CommodityID] = append(byCommodity[m.CommodityID], m.CategoryID)
	}

	from := origin.AddDate(0, 0, -b.observedWindowDays)
	results := make(map[string]RollupComputation, len(mappings))
	for commodityID, categoryIDs := range byCommodity {
		computation, cerr := b.computeCommodityRollup(ctx, commodityID, from, origin)
		if cerr != nil {
			b.logger.Warn("rollup_batch: 略過無法計算彙總的多來源品項",
				zap.Int64("commodity_id", commodityID), zap.Error(cerr))
			continue
		}
		for _, categoryID := range categoryIDs {
			results[categoryID] = computation
		}
	}
	return results, nil
}

func (b *RollupBatch) computeCommodityRollup(ctx context.Context, commodityID int64, from, origin time.Time) (RollupComputation, error) {
	series, err := b.commodityForecast.ObservedSeries(ctx, commodityID, from, origin, origin)
	if err != nil {
		return RollupComputation{}, err
	}
	if len(series.Points) == 0 {
		return RollupComputation{Status: domain.RollupStatusUnavailable, SourceGroupCount: 0}, nil
	}

	points := make([]domain.RollupPricePoint, 0, len(series.Points))
	for _, p := range series.Points {
		points = append(points, domain.RollupPricePoint{
			TradeDate: p.TradeDate.Format("2006-01-02"), AvgPrice: p.Price,
		})
	}

	latest := series.Points[len(series.Points)-1]
	latestPrice := latest.Price
	var prev *float64
	if len(series.Points) >= 2 {
		p := series.Points[len(series.Points)-2].Price
		prev = &p
	}

	p50 := latestPrice
	forecastPoints := []domain.RollupForecastPoint{{
		HorizonDays: 1,
		TargetDate:  latest.TradeDate.AddDate(0, 0, 1).Format("2006-01-02"),
		P50:         &p50,
	}}

	layer := series.PriceLayer
	unit := series.Unit
	return RollupComputation{
		Status: domain.RollupStatusReady, LatestPrice: &latestPrice, PrevPrice: prev,
		Points: points, SourceGroupCount: 1, ForecastPoints: forecastPoints,
		PriceLayer: &layer, Unit: &unit,
	}, nil
}

func (b *RollupBatch) computeGroupRollup(ctx context.Context, groupCode string, cropCodes []string, from, origin time.Time) (RollupComputation, error) {
	out, err := b.forecast.GroupObservedSeries(ctx, GroupObservedSeriesInput{
		GroupCode: groupCode, CropCodes: cropCodes, From: from, To: origin, Now: origin,
	})
	if err != nil {
		return RollupComputation{}, err
	}

	points := make([]domain.RollupPricePoint, 0, len(out.Points))
	for _, p := range out.Points {
		if p.Value == nil {
			continue
		}
		points = append(points, domain.RollupPricePoint{TradeDate: p.TradeDate.String(), AvgPrice: *p.Value})
	}

	forecastPoints := b.computeGroupForecastPoints(ctx, groupCode, cropCodes, origin)

	if len(points) == 0 {
		return RollupComputation{Status: domain.RollupStatusUnavailable, SourceGroupCount: 0, ForecastPoints: forecastPoints}, nil
	}

	latest := points[len(points)-1].AvgPrice
	var prev *float64
	if len(points) >= 2 {
		p := points[len(points)-2].AvgPrice
		prev = &p
	}
	layer := domain.PriceLayerWholesaleAuction
	unit := domain.UnitKilogram
	return RollupComputation{
		Status: domain.RollupStatusReady, LatestPrice: &latest, PrevPrice: prev,
		Points: points, SourceGroupCount: 1, ForecastPoints: forecastPoints,
		PriceLayer: &layer, Unit: &unit,
	}, nil
}

// computeGroupForecastPoints 呼叫既有 GroupForecast 取這個 group 的七日全市場
// 曲線。呼叫失敗只記 Warn、回傳空陣列——預測是價格 rollup 以外的獨立一塊資料，
// 失敗不該讓整個 group 的價格彙總也失敗（同檔案 rollupGroupLayer 對
// computeGroupRollup 本身失敗的既有 skip-and-continue 精神，這裡是同一支
// group 內部再分兩塊獨立成功/失敗）。
func (b *RollupBatch) computeGroupForecastPoints(ctx context.Context, groupCode string, cropCodes []string, origin time.Time) []domain.RollupForecastPoint {
	out, err := b.forecast.GroupForecast(ctx, GroupForecastInput{Artifact: b.artifact, GroupCode: groupCode, CropCodes: cropCodes, Origin: origin})
	if err != nil {
		b.logger.Warn("rollup_batch: 略過無法計算七日預測的大類",
			zap.String("group_code", groupCode), zap.Error(err))
		return nil
	}
	forecastPoints := make([]domain.RollupForecastPoint, 0, len(out.Forecast))
	for _, p := range out.Forecast {
		if p.P50 == nil {
			continue
		}
		forecastPoints = append(forecastPoints, domain.RollupForecastPoint{
			HorizonDays: p.HorizonDays, TargetDate: p.TargetDate.String(),
			P10: p.P10, P50: p.P50, P90: p.P90,
		})
	}
	return forecastPoints
}

// RollupBatchStats is one Handle call's summary, for the cron job's log
// line (mirrors ForecastBatchStats' shape/purpose). CategoriesWritten is the
// actual total number of category_price_rollup rows written this run —
// every successful rollupRepo.Upsert call across all three levels (L1+L2+
// L3), not just the L3/group-layer mapping count (final whole-branch review
// finding: this is the batch's only operational log signal, so it must
// reflect what was actually written, not a proxy that happens to correlate
// with it most of the time).
type RollupBatchStats struct {
	CategoriesWritten int
}

// Handle runs the full daily rollup: L3/group layer first, then the
// bottom-up L2/L1 tree walk that consumes it.
func (b *RollupBatch) Handle(ctx context.Context, origin time.Time) (RollupBatchStats, error) {
	l3Results, err := b.rollupGroupLayer(ctx, origin)
	if err != nil {
		return RollupBatchStats{}, err
	}
	commodityResults, err := b.rollupCommodityLayer(ctx, origin)
	if err != nil {
		return RollupBatchStats{}, err
	}
	for categoryID, computation := range commodityResults {
		if _, dup := l3Results[categoryID]; dup {
			b.logger.Warn("rollup_batch: 同一個 categoryId 同時有 group 與 commodity 結果，已保留 group 的",
				zap.String("category_id", categoryID))
			continue
		}
		l3Results[categoryID] = computation
	}
	written, err := b.rollupCategoryTree(ctx, l3Results, origin)
	if err != nil {
		return RollupBatchStats{}, err
	}
	return RollupBatchStats{CategoriesWritten: written}, nil
}

// rollupCategoryTree walks the product-service category tree bottom-up,
// writing one category_price_rollup row per node (all three levels — L3
// nodes are written here too, from l3Results, so every category_id in the
// tree has a row after this runs, even one category-crop-mapping never
// resolved: an unmapped/missing L3 leaf becomes an `unavailable` row here,
// not a silently absent one, so a reader never has to distinguish "no row"
// from "no price" — see design spec §4). Returns the total number of rows
// actually written (every successful walkNode Upsert, across all levels).
func (b *RollupBatch) rollupCategoryTree(ctx context.Context, l3Results map[string]RollupComputation, now time.Time) (int, error) {
	tree, err := b.categorySource.GetCategoryTree(ctx, "", false) // active-only
	if err != nil {
		return 0, fmt.Errorf("取得分類樹失敗: %w", err)
	}
	written := 0
	for _, root := range tree.GetCategories() {
		if _, err := b.walkNode(ctx, root, 1, l3Results, now, &written); err != nil {
			return written, err
		}
	}
	return written, nil
}

// walkNode computes and writes the rollup for node at the given level
// (1=L1, 2=L2, 3=L3), returning the same computation so a parent call can
// fold it into its own average. L3 nodes are leaves: their computation
// comes from l3Results (Task 7's output, already computed — not
// recomputed here), defaulting to unavailable when the node has no
// resolved category_crop_mapping row at all. written is incremented once
// per successful Upsert (by this node and, via recursion, every descendant)
// so the caller can report the real row count written this run.
func (b *RollupBatch) walkNode(ctx context.Context, n *categorypb.CategoryNode, level int16, l3Results map[string]RollupComputation, now time.Time, written *int) (RollupComputation, error) {
	var computation RollupComputation
	if level == 3 {
		computation = l3Results[n.GetId()] // zero value (unavailable status "") if absent — normalized below
		if computation.Status == "" {
			computation.Status = domain.RollupStatusUnavailable
		}
	} else {
		var children []RollupComputation
		hasMixedChild := false
		for _, child := range n.GetChildren() {
			c, err := b.walkNode(ctx, child, level+1, l3Results, now, written)
			if err != nil {
				return RollupComputation{}, err
			}
			switch c.Status {
			case domain.RollupStatusReady:
				children = append(children, c)
			case domain.RollupStatusMixedLayer:
				hasMixedChild = true
			}
		}
		computation = averageChildren(children, hasMixedChild)
	}

	if err := b.rollupRepo.Upsert(ctx, domain.CategoryPriceRollup{
		CategoryID: n.GetId(), Level: level, Status: computation.Status,
		LatestPrice: computation.LatestPrice, PrevPrice: computation.PrevPrice,
		Points: computation.Points, ForecastPoints: computation.ForecastPoints,
		SourceGroupCount: computation.SourceGroupCount,
		PriceLayer:       computation.PriceLayer,
		Unit:             computation.Unit,
		ComputedAt:       now,
	}); err != nil {
		return RollupComputation{}, fmt.Errorf("寫入 category_price_rollup 失敗（category_id=%s）: %w", n.GetId(), err)
	}
	*written++
	return computation, nil
}

// averageChildren folds a node's ready children into one simple arithmetic
// average — no volume weighting at this layer (design spec §4: by the time
// values reach here they're already "someone else's computed number", with
// no natural per-category trade-volume weight to apply). An empty input
// means every child was unavailable, and this node is unavailable too
// (fail-closed — never fabricate a price from zero real contributors).
func averageChildren(children []RollupComputation, hasMixedChild bool) RollupComputation {
	if len(children) == 0 {
		if hasMixedChild {
			return RollupComputation{Status: domain.RollupStatusMixedLayer, SourceGroupCount: 0}
		}
		return RollupComputation{Status: domain.RollupStatusUnavailable, SourceGroupCount: 0}
	}
	layer, unit, uniform := uniformLayerUnit(children)
	if hasMixedChild || !uniform {
		return RollupComputation{Status: domain.RollupStatusMixedLayer, SourceGroupCount: int16(len(children))}
	}

	var latestSum float64
	latestCount := 0
	var prevSum float64
	prevCount := 0
	byDate := map[string]float64{}
	byDateCount := map[string]int{}
	for _, c := range children {
		if c.LatestPrice != nil {
			latestSum += *c.LatestPrice
			latestCount++
		}
		if c.PrevPrice != nil {
			prevSum += *c.PrevPrice
			prevCount++
		}
		for _, p := range c.Points {
			byDate[p.TradeDate] += p.AvgPrice
			byDateCount[p.TradeDate]++
		}
	}

	out := RollupComputation{
		Status: domain.RollupStatusReady, SourceGroupCount: int16(len(children)),
		PriceLayer: layer, Unit: unit,
	}
	if latestCount > 0 {
		v := latestSum / float64(latestCount)
		out.LatestPrice = &v
	}
	if prevCount > 0 {
		v := prevSum / float64(prevCount)
		out.PrevPrice = &v
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates) // ISO 8601 date strings sort lexically == chronologically
	out.Points = make([]domain.RollupPricePoint, 0, len(dates))
	for _, d := range dates {
		out.Points = append(out.Points, domain.RollupPricePoint{TradeDate: d, AvgPrice: byDate[d] / float64(byDateCount[d])})
	}

	byHorizon := map[int]struct {
		sumP10, sumP50, sumP90       float64
		countP10, countP50, countP90 int
		targetDate                   string
	}{}
	for _, c := range children {
		for _, p := range c.ForecastPoints {
			entry := byHorizon[p.HorizonDays]
			entry.targetDate = p.TargetDate
			if p.P10 != nil {
				entry.sumP10 += *p.P10
				entry.countP10++
			}
			if p.P50 != nil {
				entry.sumP50 += *p.P50
				entry.countP50++
			}
			if p.P90 != nil {
				entry.sumP90 += *p.P90
				entry.countP90++
			}
			byHorizon[p.HorizonDays] = entry
		}
	}
	horizons := make([]int, 0, len(byHorizon))
	for h := range byHorizon {
		horizons = append(horizons, h)
	}
	sort.Ints(horizons)
	out.ForecastPoints = make([]domain.RollupForecastPoint, 0, len(horizons))
	for _, h := range horizons {
		entry := byHorizon[h]
		point := domain.RollupForecastPoint{HorizonDays: h, TargetDate: entry.targetDate}
		if entry.countP10 > 0 {
			v := entry.sumP10 / float64(entry.countP10)
			point.P10 = &v
		}
		if entry.countP50 > 0 {
			v := entry.sumP50 / float64(entry.countP50)
			point.P50 = &v
		}
		if entry.countP90 > 0 {
			v := entry.sumP90 / float64(entry.countP90)
			point.P90 = &v
		}
		out.ForecastPoints = append(out.ForecastPoints, point)
	}
	return out
}

func uniformLayerUnit(children []RollupComputation) (*domain.PriceLayer, *string, bool) {
	if len(children) == 0 {
		return nil, nil, false
	}
	first := children[0]
	if first.PriceLayer == nil || first.Unit == nil {
		return nil, nil, false
	}
	for _, c := range children[1:] {
		if c.PriceLayer == nil || c.Unit == nil {
			return nil, nil, false
		}
		if *c.PriceLayer != *first.PriceLayer || *c.Unit != *first.Unit {
			return nil, nil, false
		}
	}
	return first.PriceLayer, first.Unit, true
}
