package cropidentity

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
)

type taxonomyRetriever struct {
	uc *DisambiguationUseCase
}

func (taxonomyRetriever) Name() string { return "taxonomy" }

func (t taxonomyRetriever) Recall(ctx context.Context, q recall.Query) ([]recall.Candidate, error) {
	groups, err := t.uc.categoryGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("taxonomy retriever: %w", err)
	}
	queries := make([]string, 0, len(q.Variants))
	for _, v := range q.Variants {
		if n := NormalizeCategoryText(v); n != "" && !slices.Contains(queries, n) {
			queries = append(queries, n)
		}
	}
	if len(queries) == 0 {
		return nil, nil
	}
	pinnedNorm := ""
	if canonical, ok := cache.ManualSynonym(strings.TrimSpace(q.Text)); ok {
		pinnedNorm = NormalizeCategoryText(canonical)
	}

	var out []recall.Candidate
	seen := map[string]bool{}
	add := func(c recall.Candidate) {
		if seen[c.Key] {
			return
		}
		seen[c.Key] = true
		c.Rank = len(out) + 1
		out = append(out, c)
	}
	for _, g := range groups {
		if g.L1Label == "" || !anyCategoryNameMatches(queries, NormalizeCategoryText(g.L1Label), true) {
			continue
		}
		add(recall.Candidate{
			Key: recall.CandidateKey(recall.KindCategory, g.ID), Kind: recall.KindCategory, Code: g.ID,
			Label: g.Label, CategoryLevel: 2, ParentID: g.ParentID, ParentLabel: g.L1Label,
			MatchedBy: g.L1Label, Quality: recall.Strong,
		})
	}

	for _, exact := range []bool{true, false} {
		for _, g := range groups {
			gNorm := NormalizeCategoryText(g.Label)
			groupExact := exact && anyCategoryNameMatches(queries, gNorm, true)
			for _, child := range g.Children {
				cNorm := NormalizeCategoryText(child.Label)
				cand := recall.Candidate{
					Key: recall.CandidateKey(recall.KindCategory, child.ID), Kind: recall.KindCategory, Code: child.ID,
					Label: child.Label, CategoryIDs: []string{child.ID}, CategoryLevel: 3,
					ParentID: g.ID, ParentLabel: g.Label, MatchedBy: child.Label,
				}
				switch {
				case exact && anyCategoryNameMatches(queries, cNorm, true):
					cand.Quality = recall.Strong
					cand.Pinned = pinnedNorm != "" && cNorm == pinnedNorm
					add(cand)
				case groupExact:
					cand.Quality, cand.MatchedBy = recall.Weak, g.Label
					add(cand)
				case !exact && (anyCategoryNameMatches(queries, cNorm, false) || anyCategoryNameMatches(queries, gNorm, false)):
					cand.Quality = recall.Weak
					add(cand)
				}
			}
		}
	}
	return out, nil
}
