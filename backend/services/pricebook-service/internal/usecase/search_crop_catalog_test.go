package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// fakeSurfaceFormRepo 是 domain.SurfaceFormRepository 的替身；nil 的 fn 回零值。
type fakeSurfaceFormRepo struct {
	searchFn      func(ctx context.Context, query string, limit int32) ([]domain.SurfaceFormHit, error)
	listAliasesFn func(ctx context.Context, limit int32) ([]domain.SurfaceFormHit, error)
	categoryFn    func(ctx context.Context, hits []domain.SurfaceFormHit) (map[string][]string, error)
}

func (f *fakeSurfaceFormRepo) Search(ctx context.Context, query string, limit int32) ([]domain.SurfaceFormHit, error) {
	if f.searchFn == nil {
		return nil, nil
	}
	return f.searchFn(ctx, query, limit)
}

func (f *fakeSurfaceFormRepo) ListAliases(ctx context.Context, limit int32) ([]domain.SurfaceFormHit, error) {
	if f.listAliasesFn == nil {
		return nil, nil
	}
	return f.listAliasesFn(ctx, limit)
}

func (f *fakeSurfaceFormRepo) CategoryIDs(ctx context.Context, hits []domain.SurfaceFormHit) (map[string][]string, error) {
	if f.categoryFn == nil {
		return map[string][]string{}, nil
	}
	return f.categoryFn(ctx, hits)
}

func groupHit(code, name string, kind domain.FormKind, form string, tier int32) domain.SurfaceFormHit {
	return domain.SurfaceFormHit{EntityKind: domain.EntityKindCropGroup, EntityCode: code, FormRaw: form, FormKind: kind, Tier: tier, GroupCode: code, GroupName: name}
}

func cropHit(groupCode, groupName, cropCode, cropName, variant string, tier int32) domain.SurfaceFormHit {
	return domain.SurfaceFormHit{EntityKind: domain.EntityKindCrop, EntityCode: cropCode, FormRaw: cropName, FormKind: domain.FormKindCanonical, Tier: tier,
		GroupCode: groupCode, GroupName: groupName, CropCode: cropCode, CropName: cropName, VariantName: variant}
}

func TestSearchCropCatalog_EmptyQueryReturnsAliasesOnly(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			t.Fatal("Search must not be called for an empty query")
			return nil, nil
		},
		listAliasesFn: func(context.Context, int32) ([]domain.SurfaceFormHit, error) {
			return []domain.SurfaceFormHit{groupHit("G1", "花胡瓜", domain.FormKindAlias, "小黃瓜", 0)}, nil
		},
	}
	u := NewSearchCropCatalog(&fakeForecastRepo{}, surface, forecast.ModelArtifact{})

	matches, err := u.Get(context.Background(), "", 20)

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(matches) != 1 || matches[0].MatchedAlias == nil || *matches[0].MatchedAlias != "小黃瓜" {
		t.Fatalf("matches = %+v, want 1 alias-derived match aliased 小黃瓜", matches)
	}
	if matches[0].MatchType != "group" || matches[0].GroupName != "花胡瓜" || matches[0].GroupCode != "G1" {
		t.Errorf("matches[0] = %+v, want match_type=group, group_name=花胡瓜, group_code=G1", matches[0])
	}
}

// 同一個大類同時被 CANONICAL 與 ALIAS 命中：只回一列，且保留 matched_alias
// （契約：a group surfaced via an alias keeps its matched_alias）。
func TestSearchCropCatalog_AliasAndCanonicalHitsAreDedupedKeepingAlias(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			return []domain.SurfaceFormHit{
				groupHit("G1", "花胡瓜", domain.FormKindCanonical, "花胡瓜", 0),
				groupHit("G1", "花胡瓜", domain.FormKindAlias, "小黃瓜", 0),
				cropHit("G1", "花胡瓜", "C1", "花胡瓜-青皮", "青皮", 1),
			}, nil
		},
	}
	u := NewSearchCropCatalog(&fakeForecastRepo{}, surface, forecast.ModelArtifact{})

	matches, err := u.Get(context.Background(), "花胡瓜", 20)

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %+v, want group + crop = 2 (same group_code must dedupe)", matches)
	}
	if matches[0].MatchType != "group" || matches[0].MatchedAlias == nil || *matches[0].MatchedAlias != "小黃瓜" {
		t.Errorf("matches[0] = %+v, want the group match to keep its alias", matches[0])
	}
	if matches[1].MatchType != "crop" || matches[1].CropCode == nil || *matches[1].CropCode != "C1" || matches[1].VariantName == nil || *matches[1].VariantName != "青皮" {
		t.Errorf("matches[1] = %+v, want crop C1 / variant 青皮", matches[1])
	}
}

// commodity 命中在本 RPC 被過濾掉：SearchCropCatalog 的契約只有 group / crop 兩種
// match_type。commodity 由 SearchPriceableEntities 供應。
func TestSearchCropCatalog_CommodityHitsAreNotExposedHere(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			return []domain.SurfaceFormHit{{EntityKind: domain.EntityKindCommodity, EntityCode: "7", FormRaw: "毛豬", FormKind: domain.FormKindCanonical, CommodityName: "毛豬（規格豬）"}}, nil
		},
	}
	u := NewSearchCropCatalog(&fakeForecastRepo{}, surface, forecast.ModelArtifact{})

	matches, err := u.Get(context.Background(), "毛豬", 20)

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %+v, want none (commodity must not surface through SearchCropCatalog)", matches)
	}
}

func TestSearchCropCatalog_CropMatchIncludesMarketsFromArtifactSeries(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			return []domain.SurfaceFormHit{cropHit("group1", "香蕉", "crop1", "香蕉", "", 0)}, nil
		},
	}
	repo := &fakeForecastRepo{
		marketsByCodeFn: func(ctx context.Context, marketCodes []string) ([]domain.MarketRef, error) {
			out := make([]domain.MarketRef, 0, len(marketCodes))
			for _, c := range marketCodes {
				out = append(out, domain.MarketRef{MarketCode: c, Name: "市場-" + c})
			}
			return out, nil
		},
	}
	artifact := testArtifact(testSeriesModel("moa:109:crop1", "moa:109", "moa:crop1", "", 50, 10))
	u := NewSearchCropCatalog(repo, surface, artifact)

	matches, err := u.Get(context.Background(), "香蕉", 20)

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(matches) != 1 || !matches[0].HasForecast {
		t.Fatalf("matches = %+v, want 1 match with HasForecast=true", matches)
	}
	if len(matches[0].Markets) != 1 || matches[0].Markets[0].MarketCode != "109" || matches[0].Markets[0].MarketName != "市場-109" {
		t.Errorf("matches[0].Markets = %+v, want [{109 市場-109}]", matches[0].Markets)
	}
}

// TestSearchCropCatalog_MarketNamesUseOneBatchedLookup pins the fix for the
// per-keystroke N+1: market display names for the WHOLE result set must be
// resolved in a single repo round-trip, not one MarketByCode per (crop,
// market) pair. The singular method is wired to fail the test outright, so
// a regression back to the loop can't pass by accident.
func TestSearchCropCatalog_MarketNamesUseOneBatchedLookup(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			return []domain.SurfaceFormHit{
				cropHit("g1", "香蕉", "crop1", "香蕉", "", 0),
				cropHit("g2", "芭蕉", "crop2", "芭蕉", "", 2),
			}, nil
		},
	}
	batchCalls := 0
	var batchedCodes []string
	repo := &fakeForecastRepo{
		marketByCodeFn: func(ctx context.Context, marketCode string) (domain.MarketRef, error) {
			t.Errorf("MarketByCode(%q) called — SearchCropCatalog must batch market-name lookups for the whole result set", marketCode)
			return domain.MarketRef{}, nil
		},
		marketsByCodeFn: func(ctx context.Context, marketCodes []string) ([]domain.MarketRef, error) {
			batchCalls++
			batchedCodes = append([]string(nil), marketCodes...)
			out := make([]domain.MarketRef, 0, len(marketCodes))
			for _, c := range marketCodes {
				out = append(out, domain.MarketRef{MarketCode: c, Name: "市場-" + c})
			}
			return out, nil
		},
	}
	// 109 is shared by both crops — it must be asked for once, not twice.
	artifact := testArtifact(
		testSeriesModel("moa:104:crop1", "moa:104", "moa:crop1", "", 50, 10),
		testSeriesModel("moa:109:crop1", "moa:109", "moa:crop1", "", 50, 10),
		testSeriesModel("moa:109:crop2", "moa:109", "moa:crop2", "", 50, 10),
		testSeriesModel("moa:220:crop2", "moa:220", "moa:crop2", "", 50, 10),
	)
	u := NewSearchCropCatalog(repo, surface, artifact)

	matches, err := u.Get(context.Background(), "蕉", 20)

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if batchCalls != 1 {
		t.Fatalf("MarketsByCode called %d times, want exactly 1 for the whole result set", batchCalls)
	}
	if got, want := strings.Join(batchedCodes, ","), "104,109,220"; got != want {
		t.Errorf("batched market codes = %q, want %q (deduped across crops, deterministic order)", got, want)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %+v, want 2 crop matches", matches)
	}
	wantMarkets := map[string][]CatalogMarketOption{
		"crop1": {{MarketCode: "104", MarketName: "市場-104"}, {MarketCode: "109", MarketName: "市場-109"}},
		"crop2": {{MarketCode: "109", MarketName: "市場-109"}, {MarketCode: "220", MarketName: "市場-220"}},
	}
	for _, m := range matches {
		if m.CropCode == nil {
			t.Fatalf("match %+v has no crop_code", m)
		}
		want := wantMarkets[*m.CropCode]
		if len(m.Markets) != len(want) {
			t.Fatalf("markets for %s = %+v, want %+v", *m.CropCode, m.Markets, want)
		}
		for i := range want {
			if m.Markets[i] != want[i] {
				t.Errorf("markets for %s = %+v, want %+v (batched results must map back to the right crop)", *m.CropCode, m.Markets, want)
			}
		}
	}
}

// A market code the batch didn't return (unknown/deleted market) keeps the
// pre-batching fallback: display the code itself, never an empty label.
func TestSearchCropCatalog_UnknownMarketCodeFallsBackToTheCode(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			return []domain.SurfaceFormHit{cropHit("g1", "香蕉", "crop1", "香蕉", "", 0)}, nil
		},
	}
	repo := &fakeForecastRepo{
		marketsByCodeFn: func(ctx context.Context, marketCodes []string) ([]domain.MarketRef, error) {
			return nil, nil // market row missing for every requested code
		},
	}
	artifact := testArtifact(testSeriesModel("moa:109:crop1", "moa:109", "moa:crop1", "", 50, 10))
	u := NewSearchCropCatalog(repo, surface, artifact)

	matches, err := u.Get(context.Background(), "香蕉", 20)

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(matches) != 1 || len(matches[0].Markets) != 1 {
		t.Fatalf("matches = %+v, want 1 match with 1 market", matches)
	}
	if got := matches[0].Markets[0]; got.MarketCode != "109" || got.MarketName != "109" {
		t.Errorf("market = %+v, want {109 109} (name falls back to the code)", got)
	}
}

// limit 算的是去重後的實體數，不是索引命中列數：同一實體的多個 form 不能把名額吃掉。
func TestSearchCropCatalog_LimitCountsDistinctEntities(t *testing.T) {
	var askedLimit int32
	surface := &fakeSurfaceFormRepo{
		searchFn: func(_ context.Context, _ string, limit int32) ([]domain.SurfaceFormHit, error) {
			askedLimit = limit
			return []domain.SurfaceFormHit{
				groupHit("g1", "A", domain.FormKindCanonical, "A", 0),
				groupHit("g1", "A", domain.FormKindAlias, "A別名", 0),
				groupHit("g2", "B", domain.FormKindCanonical, "B", 1),
				groupHit("g3", "C", domain.FormKindCanonical, "C", 2),
			}, nil
		},
	}
	u := NewSearchCropCatalog(&fakeForecastRepo{}, surface, forecast.ModelArtifact{})

	matches, err := u.Get(context.Background(), "x", 2)

	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if len(matches) != 2 || matches[0].GroupCode != "g1" || matches[1].GroupCode != "g2" {
		t.Fatalf("matches = %+v, want g1, g2 (limit=2 over distinct entities)", matches)
	}
	if askedLimit <= 2 {
		t.Errorf("index asked with limit %d, want more than the caller's limit so dedup/filtering can't starve the result", askedLimit)
	}
}

// 索引失敗往上拋：它是主要來源，吞掉錯誤會讓結果系統性偏向其他路徑。
func TestSearchCropCatalog_IndexErrorIsReturned(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			return nil, context.DeadlineExceeded
		},
	}
	u := NewSearchCropCatalog(&fakeForecastRepo{}, surface, forecast.ModelArtifact{})

	if _, err := u.Get(context.Background(), "x", 5); err == nil {
		t.Fatal("Get: want the index error to be returned, got nil")
	}
}
