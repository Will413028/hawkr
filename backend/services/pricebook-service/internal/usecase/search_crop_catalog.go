package usecase

import (
	"context"
	"sort"
	"strings"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

const defaultSearchCropCatalogLimit = 20

// CatalogMarketOption is one market a specific crop trades in — a display
// name paired with the code the caller sends back as market_code.
type CatalogMarketOption struct {
	MarketCode string
	MarketName string
}

// CatalogMatch is one hit from SearchCropCatalog.Get — either a crop_group
// (MatchType=="group") or a specific crop (MatchType=="crop").
type CatalogMatch struct {
	MatchType    string
	GroupCode    string
	GroupName    string
	CropCode     *string
	CropName     *string
	VariantName  *string
	MatchedAlias *string
	HasForecast  bool
	Markets      []CatalogMarketOption
}

// SearchCropCatalog 讀 priceable_surface_form 索引。空查詢回 ALIAS 列（原本的
// 「27 條常見品項」）；非空查詢由索引依 tier（精確／前綴／子串）→ form_kind 權威度
// → 正規化名排序，這裡再依 (kind, code) 去重、保留 ALIAS 命中的 matched_alias、
// 只露出 group / crop（commodity 走 SearchPriceableEntities）。
// proto 的 doc comment 說的是同一件事——兩邊不得漂移。
type SearchCropCatalog struct {
	repo    domain.ForecastRepository
	surface domain.SurfaceFormRepository
	avail   forecastAvailability
}

func NewSearchCropCatalog(repo domain.ForecastRepository, surface domain.SurfaceFormRepository, artifact forecast.ModelArtifact) *SearchCropCatalog {
	return &SearchCropCatalog{repo: repo, surface: surface, avail: forecastAvailability{repo: repo, artifact: artifact}}
}

// indexFetchMultiplier：索引多取幾倍再去重、再濾掉 commodity，避免 limit 被同一
// 實體的多個 form（正式名＋別名＋品種名）吃掉。表很小，多取幾列不痛。
const indexFetchMultiplier = 4

func (u *SearchCropCatalog) Get(ctx context.Context, query string, limit int32) ([]CatalogMatch, error) {
	if limit <= 0 {
		limit = defaultSearchCropCatalogLimit
	}

	var hits []domain.SurfaceFormHit
	var err error
	if strings.TrimSpace(query) == "" {
		hits, err = u.surface.ListAliases(ctx, limit)
	} else {
		hits, err = u.surface.Search(ctx, query, limit*indexFetchMultiplier)
	}
	if err != nil {
		return nil, err
	}

	indexByKey := map[string]int{}
	out := make([]CatalogMatch, 0, len(hits))
	for _, h := range hits {
		if h.EntityKind == domain.EntityKindCommodity {
			continue
		}
		key := h.EntityKey()
		if idx, seen := indexByKey[key]; seen {
			// 同一實體再次命中：只有「這次是別名、之前沒有別名」才補 matched_alias。
			if h.FormKind == domain.FormKindAlias && out[idx].MatchedAlias == nil {
				alias := h.FormRaw
				out[idx].MatchedAlias = &alias
			}
			continue
		}
		if int32(len(out)) >= limit {
			continue // 名額已滿，但仍要讓已入選實體的後續別名命中補上 matched_alias
		}
		indexByKey[key] = len(out)
		out = append(out, u.hitToMatch(ctx, h))
	}

	// After the limit cut, so nothing is looked up for a match that got
	// dropped anyway.
	if err := u.resolveMarketNames(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// hitToMatch converts one index hit into a picker row. Market *names*
// are deliberately not filled in here — see resolveMarketNames.
func (u *SearchCropCatalog) hitToMatch(ctx context.Context, h domain.SurfaceFormHit) CatalogMatch {
	var match CatalogMatch
	if h.EntityKind == domain.EntityKindCropGroup {
		match = u.groupMatchRow(ctx, h.GroupCode, h.GroupName)
	} else {
		match = u.cropMatchRow(h.GroupCode, h.GroupName, h.CropCode, h.CropName, h.VariantName)
	}
	if h.FormKind == domain.FormKindAlias {
		alias := h.FormRaw
		match.MatchedAlias = &alias
	}
	return match
}

func (u *SearchCropCatalog) groupMatchRow(ctx context.Context, groupCode, groupName string) CatalogMatch {
	return CatalogMatch{MatchType: "group", GroupCode: groupCode, GroupName: groupName, HasForecast: u.avail.groupHasForecast(ctx, groupCode)}
}

func (u *SearchCropCatalog) cropMatchRow(groupCode, groupName, cropCode, cropName, variantName string) CatalogMatch {
	code, name := cropCode, cropName
	match := CatalogMatch{MatchType: "crop", GroupCode: groupCode, GroupName: groupName, CropCode: &code, CropName: &name}
	if variantName != "" {
		variant := variantName
		match.VariantName = &variant
	}
	match.Markets = u.avail.marketsForCrop(code)
	match.HasForecast = len(match.Markets) > 0
	return match
}

// resolveMarketNames fills every match's market display names using ONE
// repo round-trip for the whole result set.
//
// 🔴 This used to be a MarketByCode call per (crop, market) pair inside
// marketsForCrop — i.e. up to limit x markets-per-crop queries on every
// keystroke of an autocomplete box (review Finding I3). The market table is
// tiny and static, but the fan-out is driven by user typing speed, so it is
// the request rate that hurts, not the row count. Codes are deduplicated
// across the whole result set first (crops routinely share markets), and
// sorted, so the query is deterministic and cacheable.
//
// A code the batch doesn't return (unknown/retired market) keeps the
// original fallback: show the code itself, never an empty label.
func (u *SearchCropCatalog) resolveMarketNames(ctx context.Context, matches []CatalogMatch) error {
	codeSet := map[string]bool{}
	for _, m := range matches {
		for _, opt := range m.Markets {
			codeSet[opt.MarketCode] = true
		}
	}
	if len(codeSet) == 0 {
		return nil // group-only result set: nothing to label, no query
	}
	codes := make([]string, 0, len(codeSet))
	for code := range codeSet {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	refs, err := u.repo.MarketsByCode(ctx, codes)
	if err != nil {
		return err
	}
	names := make(map[string]string, len(refs))
	for _, ref := range refs {
		if ref.Name != "" {
			names[ref.MarketCode] = ref.Name
		}
	}
	for i := range matches {
		for j := range matches[i].Markets {
			code := matches[i].Markets[j].MarketCode
			if name, ok := names[code]; ok {
				matches[i].Markets[j].MarketName = name
			} else {
				matches[i].Markets[j].MarketName = code
			}
		}
	}
	return nil
}
