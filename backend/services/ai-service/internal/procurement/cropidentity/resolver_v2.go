package cropidentity

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/fuse"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

func (uc *DisambiguationUseCase) Resolve(ctx context.Context, itemText string) (DisambiguationResult, error) {
	q := understandQuery(itemText)
	if len(q.Variants) == 0 {
		observability.RecordProcurementCropResolverV2(ctx, "escape")
		return categoryPrice(nil), nil
	}
	pool, err := uc.recallPool(ctx, q)
	if err != nil {
		observability.RecordProcurementCropResolverV2(ctx, "error")
		return DisambiguationResult{}, err
	}
	if !hasExactMatch(pool) {
		if parts := splitItems(itemText); len(parts) > 1 {
			return uc.resolveMultiItem(ctx, itemText, parts)
		}
	}
	return uc.decide(ctx, itemText, pool)
}

func hasExactMatch(pool []fuse.Fused) bool {
	for _, f := range pool {
		if f.Quality == recall.Strong {
			return true
		}
	}
	return false
}

func (uc *DisambiguationUseCase) recallPool(ctx context.Context, q recall.Query) ([]fuse.Fused, error) {
	retriever := taxonomyRetriever{uc: uc}
	cands, err := retriever.Recall(ctx, q)
	if err != nil {
		return nil, err
	}
	return fuse.RRF(map[string][]recall.Candidate{retriever.Name(): cands}), nil
}

func (uc *DisambiguationUseCase) narrowToGroup(ctx context.Context, pool []fuse.Fused, groupID string) []fuse.Fused {
	out := make([]fuse.Fused, 0, len(pool))
	for _, f := range pool {
		ids := append([]string{f.Code}, f.CategoryIDs...)
		for _, id := range ids {
			node, ok := uc.leafByID(ctx, id)
			if ok && node.ParentID == groupID {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

func unexpandedGroup(g cropCategory) fuse.Fused {
	return fuse.Fused{Candidate: recall.Candidate{
		Key: recall.CandidateKey(recall.KindCategory, g.ID), Kind: recall.KindCategory, Code: g.ID,
		Label: g.Label, CategoryLevel: 2, MatchedBy: g.Label, Quality: recall.Weak,
	}}
}

func (uc *DisambiguationUseCase) decide(ctx context.Context, itemText string, pool []fuse.Fused) (DisambiguationResult, error) {
	if len(pool) == 0 {
		observability.RecordProcurementCropResolverV2(ctx, "escape")
		return categoryPrice(nil), nil
	}
	var strong []fuse.Fused
	for _, f := range pool {
		if f.Quality == recall.Strong {
			strong = append(strong, f)
		}
	}
	switch {
	case len(strong) == 1:
		return uc.resolveOne(ctx, itemText, strong[0])
	case len(strong) == 0 && len(pool) == 1 && pool[0].CategoryLevel != 2:
		return uc.resolveOne(ctx, itemText, pool[0])
	}
	cands := strong
	if len(cands) < 2 {
		cands = pool
	}
	return uc.categoryCard(ctx, itemText, cands)
}

func (uc *DisambiguationUseCase) resolveOne(ctx context.Context, itemText string, c fuse.Fused) (DisambiguationResult, error) {
	return uc.resolveCategory(ctx, itemText, c.Code, c.Label, c.ParentID, c.ParentLabel)
}

func (uc *DisambiguationUseCase) resolveCategory(ctx context.Context, itemText, categoryID, label, parentID, parentLabel string) (DisambiguationResult, error) {
	if label == "" || parentID == "" {
		if node, ok := uc.leafByID(ctx, categoryID); ok {
			label, parentID, parentLabel = node.Label, node.ParentID, node.ParentLabel
		}
	}
	sel := &domain.CropSelection{Stage: domain.CropSelectionPrice, QueryText: itemText, CategoryID: categoryID, Label: label, ParentCategoryID: parentID, ParentLabel: parentLabel}
	observability.RecordProcurementCropResolverV2(ctx, "resolved")
	return categoryPrice(sel), nil
}

func (uc *DisambiguationUseCase) categoryCard(ctx context.Context, itemText string, categoryPool []fuse.Fused) (DisambiguationResult, error) {
	nodes := uc.categoryNodes(ctx, categoryPool)
	if len(nodes) == 0 {
		observability.RecordProcurementCropResolverV2(ctx, "escape")
		return categoryPrice(nil), nil
	}
	groups := facetGroups(nodes)
	if facetAsksAtGroup(groups) {
		options := make([]domain.CropDisambiguationOption, 0, min(len(groups), cropCategoryMaxOptions))
		for _, g := range groups {
			if len(options) >= cropCategoryMaxOptions {
				break
			}
			options = append(options, domain.CropDisambiguationOption{Label: g.label, CategoryID: g.id, CategoryLevel: 2})
		}
		observability.RecordProcurementCropResolverV2(ctx, "ask")
		sel := domain.CropSelection{Stage: domain.CropSelectionGroup, QueryText: itemText}
		return DisambiguationResult{NeedsDisambiguation: true, Candidates: options, Selection: &sel}, nil
	}
	g := dominantGroup(groups)
	leaves := g.leaves
	switch {
	case len(leaves) == 0:
		leaves = uc.expandGroup(ctx, itemText, g)
		if len(leaves) == 0 {

			observability.RecordProcurementCropResolverV2(ctx, "escape")
			return categoryPrice(nil), nil
		}
	case len(leaves) == 1:
		return uc.resolveCategory(ctx, itemText, leaves[0].id, leaves[0].label, g.id, g.label)
	}
	options := make([]domain.CropDisambiguationOption, 0, min(len(leaves), cropCategoryMaxOptions))
	for _, l := range leaves {
		if len(options) >= cropCategoryMaxOptions {
			break
		}
		options = append(options, domain.CropDisambiguationOption{Label: l.label, CategoryID: l.id, CategoryLevel: 3})
	}
	observability.RecordProcurementCropResolverV2(ctx, "ask")
	sel := domain.CropSelection{Stage: domain.CropSelectionItem, QueryText: itemText, ParentCategoryID: g.id, ParentLabel: g.label}
	return DisambiguationResult{NeedsDisambiguation: true, Candidates: options, Selection: &sel}, nil
}

func (uc *DisambiguationUseCase) categoryNodes(ctx context.Context, pool []fuse.Fused) []categoryNode {
	var nodes []categoryNode
	seen := map[string]bool{}
	for _, f := range pool {
		var id string
		switch {
		case f.CategoryLevel == 2:
			if !seen[f.Code] {
				seen[f.Code] = true
				nodes = append(nodes, categoryNode{id: f.Code, label: f.Label, level: 2})
			}
			continue
		default:
			id = f.Code
		}
		if seen[id] {
			continue
		}
		node, ok := uc.leafByID(ctx, id)
		if !ok {
			continue
		}
		seen[id] = true
		nodes = append(nodes, categoryNode{id: id, label: node.Label, parentID: node.ParentID, parentLabel: node.ParentLabel, level: 3})
	}
	return nodes
}

func (uc *DisambiguationUseCase) expandGroup(ctx context.Context, itemText string, g facetGroup) []categoryNode {
	group, ok := uc.groupByID(ctx, g.id)
	if !ok || len(group.Children) == 0 {
		return nil
	}
	children := group.Children
	if len(children) > 1 {
		children = uc.suggestCategories(ctx, itemText, domain.CropSelectionItem, group.Label, children)
	}
	out := make([]categoryNode, 0, len(children))
	for _, c := range children {
		out = append(out, categoryNode{id: c.ID, label: c.Label, parentID: group.ID, parentLabel: group.Label, level: 3})
	}
	return out
}

func (uc *DisambiguationUseCase) groupByID(ctx context.Context, id string) (cropCategory, bool) {
	groups, err := uc.categoryGroups(ctx)
	if err != nil {
		return cropCategory{}, false
	}
	for _, g := range groups {
		if g.ID == id {
			return g, true
		}
	}
	return cropCategory{}, false
}

type leafNode struct {
	Label, ParentID, ParentLabel string
}

func (uc *DisambiguationUseCase) leafByID(ctx context.Context, id string) (leafNode, bool) {
	groups, err := uc.categoryGroups(ctx)
	if err != nil {
		return leafNode{}, false
	}
	for _, g := range groups {
		for _, child := range g.Children {
			if child.ID == id {
				return leafNode{Label: child.Label, ParentID: g.ID, ParentLabel: g.Label}, true
			}
		}
	}
	return leafNode{}, false
}
