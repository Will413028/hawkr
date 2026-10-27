package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

var ErrNoQueryVariants = errors.New("search priceable entities: no non-blank query variants")

const defaultSearchPriceableEntitiesLimit = 20

// PriceableEntityMatch 是 SearchPriceableEntities 的一列：crop_group / crop /
// commodity 同場，帶分類 id 與名次，給 ai-service 的 rank fusion 用。
type PriceableEntityMatch struct {
	Kind        domain.EntityKind
	Code        string
	Name        string
	CategoryIDs []string
	MatchedForm string
	FormKind    domain.FormKind
	Tier        int32
	Rank        int32
	// HasForecast：CROP 在任一市場有訓練好的序列；CROP_GROUP 任一子品項有；
	// COMMODITY 恆為 true（行情來自觀測值，不經模型）。呼叫端據此決定能不能定案。
	HasForecast bool
	// GroupCode：CROP 的所屬大類；CROP_GROUP 等於 Code；COMMODITY 空。
	GroupCode string
}

// SearchPriceableEntities 是品項解析的檢索半邊：每個查詢變體各查一次索引，
// 依 (kind, code) 去重取最佳 tier，再補分類 id。名次由「最佳 tier → form_kind
// 權威度 → 第一次出現的順序」決定；RRF 只需要名次。
type SearchPriceableEntities struct {
	surface domain.SurfaceFormRepository
	avail   forecastAvailability
}

func NewSearchPriceableEntities(surface domain.SurfaceFormRepository, repo domain.ForecastRepository, artifact forecast.ModelArtifact) *SearchPriceableEntities {
	return &SearchPriceableEntities{surface: surface, avail: forecastAvailability{repo: repo, artifact: artifact}}
}

func (u *SearchPriceableEntities) Get(ctx context.Context, variants []string, limit int32) ([]PriceableEntityMatch, error) {
	if limit <= 0 {
		limit = defaultSearchPriceableEntitiesLimit
	}
	type candidate struct {
		hit   domain.SurfaceFormHit
		order int
	}
	best := map[string]*candidate{}
	seen := 0
	anyVariant := false
	for _, v := range variants {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		anyVariant = true
		hits, err := u.surface.Search(ctx, v, limit*indexFetchMultiplier)
		if err != nil {
			return nil, err
		}
		for _, h := range hits {
			key := h.EntityKey()
			cur, ok := best[key]
			if !ok {
				best[key] = &candidate{hit: h, order: seen}
				seen++
				continue
			}
			if betterHit(h, cur.hit) {
				cur.hit = h
			}
		}
	}
	if !anyVariant {
		return nil, ErrNoQueryVariants
	}

	ranked := make([]*candidate, 0, len(best))
	for _, c := range best {
		ranked = append(ranked, c)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].hit.Tier != ranked[j].hit.Tier {
			return ranked[i].hit.Tier < ranked[j].hit.Tier
		}
		if ri, rj := domain.FormKindRank(ranked[i].hit.FormKind), domain.FormKindRank(ranked[j].hit.FormKind); ri != rj {
			return ri < rj
		}
		return ranked[i].order < ranked[j].order
	})
	if int32(len(ranked)) > limit {
		ranked = ranked[:limit]
	}

	hits := make([]domain.SurfaceFormHit, 0, len(ranked))
	for _, c := range ranked {
		hits = append(hits, c.hit)
	}
	categories, err := u.surface.CategoryIDs(ctx, hits)
	if err != nil {
		return nil, err
	}
	out := make([]PriceableEntityMatch, 0, len(hits))
	for i, h := range hits {
		m := PriceableEntityMatch{
			Kind: h.EntityKind, Code: h.EntityCode, Name: displayNameOf(h),
			CategoryIDs: categories[h.EntityKey()],
			MatchedForm: h.FormRaw, FormKind: h.FormKind, Tier: h.Tier, Rank: int32(i + 1),
		}
		switch h.EntityKind {
		case domain.EntityKindCrop:
			m.HasForecast, m.GroupCode = u.avail.cropHasForecast(h.EntityCode), h.GroupCode
		case domain.EntityKindCropGroup:
			m.HasForecast, m.GroupCode = u.avail.groupHasForecast(ctx, h.EntityCode), h.EntityCode
		case domain.EntityKindCommodity:
			m.HasForecast = true
		}
		out = append(out, m)
	}
	return out, nil
}

// betterHit：tier 小者勝；同 tier 依 form_kind 權威度。
func betterHit(a, b domain.SurfaceFormHit) bool {
	if a.Tier != b.Tier {
		return a.Tier < b.Tier
	}
	return domain.FormKindRank(a.FormKind) < domain.FormKindRank(b.FormKind)
}

func displayNameOf(h domain.SurfaceFormHit) string {
	switch h.EntityKind {
	case domain.EntityKindCropGroup:
		return h.GroupName
	case domain.EntityKindCrop:
		return h.CropName
	default:
		return h.CommodityName
	}
}
