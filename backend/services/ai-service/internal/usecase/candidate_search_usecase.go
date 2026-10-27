package usecase

import (
	"context"
	"sort"
	"strings"
	"time"
)

type CandidateSearchParams struct {
	Category string
	Keywords string
	Region   string

	CategoryIDs []string
}

const (
	FacetCategory = "category"
	FacetRegion   = "region"
	FacetKeywords = "keywords"

	FacetCategoryIDs = "category_ids"
)

const (
	weightCategory = 2
	weightRegion   = 2

	weightCategoryIDs = 4
)

type ProspectVendor struct {
	ID              int64
	DisplayName     string
	Category        string
	Region          string
	ProductSummary  string
	Positioning     string
	ResearchSummary string

	PublicWebsite  string
	PriorityLetter string
	PriorityStars  int

	// 品牌素材 objectKey。刻意不進 keywordRelevanceScore——圖片不是可比對的
	// 文字事實，讓它影響排序等於用「這家有沒有圖」當相關性訊號。
	LogoKey  string
	CoverKey string

	FactsAsOf time.Time
}

type CandidateMatch struct {
	Vendor        ProspectVendor
	Score         int
	MatchedFacets []string
	MissedFacets  []string
}

type CandidateLeadSearcher interface {
	Search(ctx context.Context, params CandidateSearchParams) ([]CandidateMatch, error)
}

type CandidateSearchUseCase struct {
	seller SellerClient
}

func NewCandidateSearchUseCase(seller SellerClient) *CandidateSearchUseCase {
	return &CandidateSearchUseCase{seller: seller}
}

func keywordRelevanceScore(keywords string, v ProspectVendor) int {
	score := 0
	fields := []string{v.ProductSummary, v.Positioning, v.ResearchSummary}
	for _, tok := range strings.Fields(strings.ToLower(keywords)) {
		if tok == "" {
			continue
		}
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), tok) {
				score++
			}
		}
	}
	return score
}

func (uc *CandidateSearchUseCase) Search(ctx context.Context, params CandidateSearchParams) ([]CandidateMatch, error) {
	category := strings.TrimSpace(params.Category)
	region := strings.TrimSpace(params.Region)
	keywords := strings.TrimSpace(params.Keywords)

	hits, err := uc.seller.SearchProspects(ctx, ProspectSearchParams{
		Category: category, Region: region, Keywords: keywords,
		CategoryIDs: params.CategoryIDs, Limit: 0,
	})
	if err != nil {
		return nil, err
	}

	out := make([]CandidateMatch, 0, len(hits))
	for _, h := range hits {
		v := ProspectVendor{
			ID: h.ID, DisplayName: h.DisplayName, Category: h.Category, Region: h.Region,
			ProductSummary: h.ProductSummary, Positioning: h.Positioning,
			ResearchSummary: h.ResearchSummary, PublicWebsite: h.PublicWebsite,
			PriorityLetter: h.PriorityLetter,
			PriorityStars:  h.PriorityStars, FactsAsOf: h.UpdatedAt,
			LogoKey: h.LogoKey, CoverKey: h.CoverKey,
		}
		m := CandidateMatch{Vendor: v}
		addFacet := func(specified, hit bool, name string, weight int) {
			if !specified {
				return
			}
			if hit {
				m.MatchedFacets = append(m.MatchedFacets, name)
				m.Score += weight
				return
			}
			m.MissedFacets = append(m.MissedFacets, name)
		}
		addFacet(category != "", h.HitCategory, FacetCategory, weightCategory)
		addFacet(region != "", h.HitRegion, FacetRegion, weightRegion)
		addFacet(keywords != "", h.HitKeywords, FacetKeywords, keywordRelevanceScore(keywords, v))
		addFacet(len(params.CategoryIDs) > 0, h.HitCategoryIDs, FacetCategoryIDs, weightCategoryIDs)
		out = append(out, m)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		vi, vj := out[i].Vendor, out[j].Vendor
		iEmpty, jEmpty := vi.PriorityLetter == "", vj.PriorityLetter == ""
		if iEmpty != jEmpty {
			return !iEmpty
		}
		if vi.PriorityLetter != vj.PriorityLetter {
			return vi.PriorityLetter < vj.PriorityLetter
		}
		if vi.PriorityStars != vj.PriorityStars {
			return vi.PriorityStars > vj.PriorityStars
		}
		return vi.ID > vj.ID
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out, nil
}
