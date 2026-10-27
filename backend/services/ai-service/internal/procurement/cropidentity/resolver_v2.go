package cropidentity

import (
	"context"
	"sort"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/fuse"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

func (uc *DisambiguationUseCase) Resolve(ctx context.Context, itemText string) (DisambiguationResult, error) {
	q := understandQuery(itemText)
	if len(q.Variants) == 0 {
		observability.RecordProcurementCropResolverV2(ctx, "escape")
		return categoryPrice(nil), nil
	}
	results, err := uc.recallAll(ctx, q)
	if err != nil {
		observability.RecordProcurementCropResolverV2(ctx, "error")
		return DisambiguationResult{}, err
	}
	usable := usableOf(fuse.AbsorbCategories(fuse.RRF(results)))
	if !hasExactMatch(usable) {
		if parts := splitItems(itemText); len(parts) > 1 {
			return uc.resolveMultiItem(ctx, itemText, parts)
		}
	}
	return uc.decide(ctx, itemText, usable)
}

func hasExactMatch(pool []fuse.Fused) bool {
	for _, f := range pool {
		if f.Quality >= recall.Medium {
			return true
		}
	}
	return false
}

func (uc *DisambiguationUseCase) recallUsable(ctx context.Context, q recall.Query) ([]fuse.Fused, error) {
	results, err := uc.recallAll(ctx, q)
	if err != nil {
		return nil, err
	}
	return usableOf(fuse.AbsorbCategories(fuse.RRF(results))), nil
}

func (uc *DisambiguationUseCase) narrowToGroup(ctx context.Context, pool []fuse.Fused, groupID string) []fuse.Fused {
	out := make([]fuse.Fused, 0, len(pool))
	for _, f := range pool {
		ids := f.CategoryIDs
		if f.Kind == recall.KindCategory {
			ids = append([]string{f.Code}, ids...)
		}
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
		Label: g.Label, CategoryLevel: 2, HasForecast: true, MatchedBy: g.Label, Quality: recall.Weak,
	}}
}

func (uc *DisambiguationUseCase) recallAll(ctx context.Context, q recall.Query) (map[string][]recall.Candidate, error) {
	entity := recall.NewEntityRetriever(uc.pricebook)
	taxonomy := taxonomyRetriever{uc: uc}
	results := map[string][]recall.Candidate{}
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		cands, err := entity.Recall(gctx, q)
		if err != nil {
			return err
		}
		mu.Lock()
		results[entity.Name()] = cands
		mu.Unlock()
		return nil
	})
	g.Go(func() error {
		cands, err := taxonomy.Recall(gctx, q)
		if err != nil {
			pkglogger.Warn(ctx, "procurement crop resolver v2: taxonomy retriever degraded", zap.Error(err))
			return nil
		}
		mu.Lock()
		results[taxonomy.Name()] = cands
		mu.Unlock()
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func usableOf(fused []fuse.Fused) []fuse.Fused {
	usable := make([]fuse.Fused, 0, len(fused))
	for _, f := range fused {
		switch f.Kind {
		case recall.KindCommodity:
			if len(f.CategoryIDs) == 0 {
				continue
			}
		case recall.KindCrop, recall.KindCropGroup:
			if !f.HasForecast {
				continue
			}
		}
		usable = append(usable, f)
	}
	return usable
}

func (uc *DisambiguationUseCase) decide(ctx context.Context, itemText string, usable []fuse.Fused) (DisambiguationResult, error) {
	if len(usable) == 0 {
		observability.RecordProcurementCropResolverV2(ctx, "escape")
		return categoryPrice(nil), nil
	}
	var strong []fuse.Fused
	for _, f := range usable {
		if f.Quality == recall.Strong {
			strong = append(strong, f)
		}
	}
	switch {
	case len(strong) == 1:
		res, err := uc.resolveOne(ctx, itemText, strong[0])
		if err != nil || !res.Unavailable || len(usable) < 2 {
			return res, err
		}
		return uc.ask(ctx, itemText, without(usable, strong[0].Key))
	case len(strong) == 0 && len(usable) == 1 && usable[0].CategoryLevel != 2:
		return uc.resolveOne(ctx, itemText, usable[0])
	}
	pool := strong
	if len(pool) < 2 {
		pool = usable
	}
	return uc.ask(ctx, itemText, pool)
}

func without(pool []fuse.Fused, key string) []fuse.Fused {
	out := make([]fuse.Fused, 0, len(pool))
	for _, f := range pool {
		if f.Key != key {
			out = append(out, f)
		}
	}
	return out
}

func (uc *DisambiguationUseCase) resolveOne(ctx context.Context, itemText string, c fuse.Fused) (DisambiguationResult, error) {
	switch c.Kind {
	case recall.KindCrop:
		code, group := c.Code, c.GroupCode
		res := DisambiguationResult{CropCode: &code, Label: c.Label,
			CategoryOrigin: domain.CategoryFromEntity(entityCategoryOf(c))}
		if group != "" {
			res.FallbackGroupCode = &group
		}
		observability.RecordProcurementCropResolverV2(ctx, "resolved")
		return res, nil
	case recall.KindCropGroup:
		candidates, err := uc.drillDownGroup(ctx, c.Code, c.Label, entityCategoryOf(c))
		if err != nil {
			observability.RecordProcurementCropResolverV2(ctx, "error")
			return DisambiguationResult{}, err
		}
		if len(candidates) > 0 {
			observability.RecordProcurementCropResolverV2(ctx, "ask")
			return DisambiguationResult{NeedsDisambiguation: true, Candidates: candidates}, nil
		}
		code := c.Code
		observability.RecordProcurementCropResolverV2(ctx, "resolved")
		return DisambiguationResult{GroupCode: &code, FallbackGroupCode: &code, Label: c.Label,
			CategoryOrigin: domain.CategoryFromEntity(entityCategoryOf(c))}, nil
	case recall.KindCommodity:
		return uc.resolveCategory(ctx, itemText, c.CategoryIDs[0], "", "", "")
	default:
		return uc.resolveCategory(ctx, itemText, c.Code, c.Label, c.ParentID, c.ParentLabel)
	}
}

func (uc *DisambiguationUseCase) resolveCategory(ctx context.Context, itemText, categoryID, label, parentID, parentLabel string) (DisambiguationResult, error) {
	if label == "" || parentID == "" {
		if node, ok := uc.leafByID(ctx, categoryID); ok {
			label, parentID, parentLabel = node.Label, node.ParentID, node.ParentLabel
		}
	}
	sel := &domain.CropSelection{Stage: domain.CropSelectionPrice, QueryText: itemText, CategoryID: categoryID, Label: label, ParentCategoryID: parentID, ParentLabel: parentLabel}
	res, err := uc.priceSelectedCategory(ctx, sel)
	res.CategoryOrigin = domain.CategoryFromSelection(sel)
	switch {
	case err != nil:
		observability.RecordProcurementCropResolverV2(ctx, "error")
	case res.Unavailable:
		observability.RecordProcurementCropResolverV2(ctx, "escape")
	default:
		observability.RecordProcurementCropResolverV2(ctx, "resolved")
	}
	return res, err
}

func (uc *DisambiguationUseCase) ask(ctx context.Context, itemText string, pool []fuse.Fused) (DisambiguationResult, error) {
	var pricebookPool, categoryPool []fuse.Fused
	for _, f := range pool {
		if f.Kind == recall.KindCrop || f.Kind == recall.KindCropGroup {
			pricebookPool = append(pricebookPool, f)
		} else {
			categoryPool = append(categoryPool, f)
		}
	}

	sortByQuality(pricebookPool)
	sortByQuality(categoryPool)
	if len(categoryPool) == 0 || (len(pricebookPool) > 0 && maxQuality(pricebookPool) >= maxQuality(categoryPool)) {
		return uc.pricebookCard(ctx, pricebookPool), nil
	}
	res, err := uc.categoryCard(ctx, itemText, categoryPool)
	if err == nil && res.Unavailable && len(pricebookPool) > 0 {

		return uc.pricebookCard(ctx, pricebookPool), nil
	}
	return res, err
}

func (uc *DisambiguationUseCase) pricebookCard(ctx context.Context, pricebookPool []fuse.Fused) DisambiguationResult {
	options := make([]domain.CropDisambiguationOption, 0, len(pricebookPool))
	for _, f := range pricebookPool {
		if len(options) >= searchCropCatalogLimit {
			break
		}
		code := f.Code
		if f.Kind == recall.KindCrop {
			opt := domain.CropDisambiguationOption{Label: f.Label, CropCode: &code, EntityCategoryID: entityCategoryOf(f)}
			if f.GroupCode != "" {
				group := f.GroupCode
				opt.FallbackGroupCode = &group
			}
			options = append(options, opt)
			continue
		}
		options = append(options, domain.CropDisambiguationOption{
			Label: "整個" + f.Label + "都可以", ResolvedLabel: f.Label, GroupCode: &code, FallbackGroupCode: &code,
			EntityCategoryID: entityCategoryOf(f),
		})
	}
	observability.RecordProcurementCropResolverV2(ctx, "ask")
	return DisambiguationResult{NeedsDisambiguation: true, Candidates: options}
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
		case f.Kind == recall.KindCategory && f.CategoryLevel == 2:
			if !seen[f.Code] {
				seen[f.Code] = true
				nodes = append(nodes, categoryNode{id: f.Code, label: f.Label, level: 2})
			}
			continue
		case f.Kind == recall.KindCategory:
			id = f.Code
		default:
			if len(f.CategoryIDs) == 0 {
				continue
			}
			id = f.CategoryIDs[0]
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

func sortByQuality(pool []fuse.Fused) {
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].Quality > pool[j].Quality })
}

func maxQuality(pool []fuse.Fused) recall.Quality {
	best := recall.Weak
	for _, f := range pool {
		if f.Quality > best {
			best = f.Quality
		}
	}
	return best
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

func entityCategoryOf(f fuse.Fused) string {
	if len(f.CategoryIDs) == 0 {
		return ""
	}
	return f.CategoryIDs[0]
}
