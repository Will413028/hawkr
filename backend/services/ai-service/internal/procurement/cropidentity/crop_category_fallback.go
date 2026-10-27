package cropidentity

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/fuse"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/zhtw"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
	"go.uber.org/zap"
)

const (
	cropCategoryLookupTimeout    = 3 * time.Second
	cropCategorySelectionTimeout = 12 * time.Second
	cropCategoryMaxOptions       = 8
)

type cropCategoryReader interface {
	GetCategoryTree(context.Context, string) (*categorypb.GetCategoryTreeResponse, error)
}

type cropCategory struct {
	ID    string `json:"id"`
	Label string `json:"label"`

	ParentID string         `json:"-"`
	L1Label  string         `json:"-"`
	Level    int            `json:"-"`
	Children []cropCategory `json:"children,omitempty"`
}

type cropCategorySuggester interface {
	Suggest(context.Context, string, string, string, []cropCategory) ([]string, error)
}

func (uc *DisambiguationUseCase) categoryGroups(ctx context.Context) ([]cropCategory, error) {
	if uc.categories == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, cropCategoryLookupTimeout)
	defer cancel()
	resp, err := uc.categories.GetCategoryTree(ctx, "active")
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.GetCode() != 0 {
		return nil, fmt.Errorf("active category tree unavailable")
	}
	groups := make([]cropCategory, 0)
	seen := make(map[string]bool)
	for _, root := range resp.GetCategories() {
		if !activeCropCategory(root) {
			continue
		}
		for _, node := range root.GetChildren() {
			if !activeCropCategory(node) || seen[node.GetId()] {
				continue
			}
			if node.GetParentId() != root.GetId() {
				continue
			}
			seen[node.GetId()] = true
			group := cropCategory{ID: node.GetId(), Label: node.GetName(), ParentID: root.GetId(), L1Label: root.GetName(), Level: 2}
			for _, child := range node.GetChildren() {
				if !activeCropCategory(child) || seen[child.GetId()] || child.GetParentId() != node.GetId() {
					continue
				}
				seen[child.GetId()] = true
				group.Children = append(group.Children, cropCategory{ID: child.GetId(), Label: child.GetName(), ParentID: node.GetId(), Level: 3})
			}
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func activeCropCategory(n *categorypb.CategoryNode) bool {
	return n != nil && n.GetId() != "" && strings.TrimSpace(n.GetName()) != "" && n.GetStatus() == codemeta.CommonStatusActive.Value && n.GetDeletedAt() == ""
}

func NormalizeCategoryText(text string) string {
	text = zhtw.ToTraditional(text)
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		if r >= 'Ａ' && r <= 'Ｚ' || r >= 'ａ' && r <= 'ｚ' || r >= '０' && r <= '９' {
			r -= 0xfee0
		}
		if r == '臺' {
			r = '台'
		}
		return unicode.ToLower(r)
	}, text)
}

func cropCategoryQueries(text string) []string {
	queries := make([]string, 0, 3)
	add := func(s string) {
		s = NormalizeCategoryText(s)
		if s == "" || slices.Contains(queries, s) {
			return
		}
		queries = append(queries, s)
	}
	add(text)
	add(cache.CanonicalCategoryName(text))
	if len(queries) > 0 {

		add(cache.CanonicalCategoryName(queries[0]))
	}
	return queries
}

type normalizedCropCategory struct {
	label    string
	children []string
}

func normalizeCropCategories(categories []cropCategory, includeChildren bool) []normalizedCropCategory {
	out := make([]normalizedCropCategory, len(categories))
	for i, c := range categories {
		out[i].label = NormalizeCategoryText(c.Label)
		if !includeChildren {
			continue
		}
		out[i].children = make([]string, 0, len(c.Children))
		for _, child := range c.Children {
			out[i].children = append(out[i].children, NormalizeCategoryText(child.Label))
		}
	}
	return out
}

func matchCropCategories(text string, categories []cropCategory, includeChildren bool) []cropCategory {
	queries := cropCategoryQueries(text)
	if len(queries) == 0 {
		return nil
	}
	normalized := normalizeCropCategories(categories, includeChildren)
	for _, exact := range []bool{true, false} {
		var matches []cropCategory
		for i, c := range categories {
			match := anyCategoryNameMatches(queries, normalized[i].label, exact)
			if !match {
				for _, child := range normalized[i].children {
					if anyCategoryNameMatches(queries, child, exact) {
						match = true
						break
					}
				}
			}
			if match {
				matches = append(matches, c)
			}
		}
		if len(matches) > 0 {
			return matches
		}
	}
	return nil
}

func anyCategoryNameMatches(queries []string, name string, exact bool) bool {
	for _, query := range queries {
		if categoryNameMatches(query, name, exact) {
			return true
		}
	}
	return false
}

func categoryNameMatches(query, name string, exact bool) bool {
	if name == "" {
		return false
	}
	if exact {
		return name == query
	}

	if len([]rune(query)) < 2 || len([]rune(name)) < 2 {
		return false
	}
	return strings.Contains(query, name) || strings.Contains(name, query)
}

func (uc *DisambiguationUseCase) suggestCategories(ctx context.Context, text, stage, parentLabel string, categories []cropCategory) []cropCategory {
	if len(categories) == 0 {
		return nil
	}
	matches := matchCropCategories(text, categories, stage == domain.CropSelectionGroup)
	if len(matches) > 0 {
		return limitCropCategories(matches)
	}
	if uc.categoryModel == nil {
		return nil
	}
	if stage == domain.CropSelectionGroup {
		return uc.suggestGroupsViaL1(ctx, text, categories)
	}
	return uc.adoptSuggestions(ctx, text, stage, parentLabel, categories)
}

func (uc *DisambiguationUseCase) suggestGroupsViaL1(ctx context.Context, text string, groups []cropCategory) []cropCategory {
	l1s := cropCategoryL1s(groups)
	if len(l1s) == 0 {
		return nil
	}
	picked := uc.adoptSuggestions(ctx, text, cropCategoryStageL1, "", l1s)
	if len(picked) == 0 {
		return nil
	}
	if len(picked) > cropCategoryMaxL1Picks {
		picked = picked[:cropCategoryMaxL1Picks]
	}
	ids := make([]string, 0, len(picked))
	for _, c := range picked {
		ids = append(ids, c.ID)
	}
	allowed := groupsUnderL1s(groups, ids)
	if len(allowed) == 0 {
		return nil
	}
	return uc.pickGroupsInChunks(ctx, text, allowed)
}

func (uc *DisambiguationUseCase) pickGroupsInChunks(ctx context.Context, text string, allowed []cropCategory) []cropCategory {
	if len(allowed) <= cropCategoryModelMaxCandidates {
		return uc.adoptSuggestions(ctx, text, domain.CropSelectionGroup, "", allowed)
	}
	var merged []cropCategory
	for start := 0; start < len(allowed); start += cropCategoryModelMaxCandidates {
		end := min(start+cropCategoryModelMaxCandidates, len(allowed))
		merged = append(merged, uc.adoptSuggestions(ctx, text, domain.CropSelectionGroup, "", allowed[start:end])...)
	}
	if len(merged) <= cropCategoryMaxOptions {
		return merged
	}
	return uc.adoptSuggestions(ctx, text, domain.CropSelectionGroup, "", merged)
}

func (uc *DisambiguationUseCase) adoptSuggestions(ctx context.Context, text, stage, parentLabel string, supplied []cropCategory) []cropCategory {
	ids, err := uc.categoryModel.Suggest(ctx, text, stage, parentLabel, supplied)
	if err != nil {
		logCropCategorySuggestFailure(ctx, stage, len(supplied), err)
		return nil
	}
	if len(ids) > cropCategoryMaxOptions {
		pkglogger.Warn(ctx, "procurement taxonomy suggestion rejected: 回傳數量超過上限",
			zap.String("stage", stage), zap.Int("returned", len(ids)), zap.Int("max", cropCategoryMaxOptions))
		return nil
	}
	allowed := make(map[string]cropCategory, len(supplied))
	for _, c := range supplied {
		allowed[c.ID] = c
	}
	var out []cropCategory
	seen := make(map[string]bool)
	for _, id := range ids {
		c, ok := allowed[id]
		if !ok {
			pkglogger.Warn(ctx, "procurement taxonomy suggestion rejected: ID 不在白名單",
				zap.String("stage", stage), zap.String("rejected_id", id))
			return nil
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, c)
		}
	}
	return out
}

func cropCategoryL1s(groups []cropCategory) []cropCategory {
	var out []cropCategory
	seen := make(map[string]bool)
	for _, g := range groups {
		if g.ParentID == "" || g.L1Label == "" || seen[g.ParentID] {
			continue
		}
		seen[g.ParentID] = true
		out = append(out, cropCategory{ID: g.ParentID, Label: g.L1Label, Level: 1})
	}
	return out
}

func groupsUnderL1s(groups []cropCategory, l1IDs []string) []cropCategory {
	want := make(map[string]bool, len(l1IDs))
	for _, id := range l1IDs {
		want[id] = true
	}
	var out []cropCategory
	for _, g := range groups {
		if want[g.ParentID] {
			out = append(out, g)
		}
	}
	return out
}

func logCropCategorySuggestFailure(ctx context.Context, stage string, supplied int, err error) {
	switch {
	case errors.Is(err, errCropCategoryBoundExceeded), errors.Is(err, errCropCategoryPromptTooLarge):
		pkglogger.Error(ctx, "procurement taxonomy suggestion 觸及硬上限——收斂邏輯有漏，這不是偶發錯誤",
			zap.Error(err), zap.String("stage", stage), zap.Int("supplied", supplied))
	case errors.Is(err, context.DeadlineExceeded):
		pkglogger.Warn(ctx, "procurement taxonomy suggestion 逾時",
			zap.String("stage", stage), zap.Int("supplied", supplied))
	default:
		pkglogger.Warn(ctx, "procurement taxonomy suggestion unavailable",
			zap.Error(err), zap.String("stage", stage), zap.Int("supplied", supplied))
	}
}

func limitCropCategories(matches []cropCategory) []cropCategory {
	if len(matches) > cropCategoryMaxOptions {
		return matches[:cropCategoryMaxOptions]
	}
	return matches
}

func categoryPending(selection domain.CropSelection, categories []cropCategory) DisambiguationResult {
	options := make([]domain.CropDisambiguationOption, 0, len(categories))
	for _, c := range categories {
		options = append(options, domain.CropDisambiguationOption{Label: c.Label, CategoryID: c.ID, CategoryLevel: c.Level})
	}
	return DisambiguationResult{NeedsDisambiguation: true, Candidates: options, Selection: &selection}
}

func categoryPrice(selection *domain.CropSelection) DisambiguationResult {
	out := domain.CropSelection{Stage: domain.CropSelectionPrice}
	if selection != nil {
		out = *selection
		out.Stage = domain.CropSelectionPrice
	}
	return DisambiguationResult{Unavailable: true, Selection: &out}
}

func (uc *DisambiguationUseCase) ResolveSelection(ctx context.Context, itemText string, selection *domain.CropSelection, candidates []domain.CropDisambiguationOption, optionID, freeText string) (DisambiguationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, cropCategorySelectionTimeout)
	defer cancel()
	if selection == nil || (selection.Stage != domain.CropSelectionGroup && selection.Stage != domain.CropSelectionItem) {
		return categoryPrice(selection), nil
	}
	if optionID == domain.CropOtherOptionID && strings.TrimSpace(freeText) == "" {
		return categoryPrice(selection), nil
	}
	groups, err := uc.categoryGroups(ctx)
	if err != nil {
		pkglogger.Warn(ctx, "procurement active taxonomy unavailable on resume", zap.Error(err))
		return categoryPrice(selection), nil
	}
	allowed := groups
	if selection.Stage == domain.CropSelectionItem {
		allowed = nil
		for _, group := range groups {
			if group.ID == selection.ParentCategoryID {
				allowed = group.Children
				break
			}
		}
	}
	var chosen *cropCategory
	if optionID != "" && optionID != domain.CropOtherOptionID {

		for _, option := range candidates {
			if option.CategoryID == "" || optionID != "category-"+option.CategoryID {
				continue
			}
			for _, c := range allowed {
				if c.ID == option.CategoryID && c.Level == option.CategoryLevel {
					chosen = &c
					break
				}
			}
			break
		}
	} else if strings.TrimSpace(freeText) != "" {
		text := strings.TrimSpace(freeText)
		matches, err := uc.suggestWithinStage(ctx, text, selection, allowed)
		if err != nil {
			return DisambiguationResult{}, err
		}
		if len(matches) > 0 {
			next := *selection
			next.QueryText = text
			return categoryPending(next, matches), nil
		}
	}
	if chosen == nil {
		return categoryPrice(selection), nil
	}
	if selection.Stage == domain.CropSelectionGroup {
		query := itemText
		if selection.QueryText != "" {
			query = selection.QueryText
		}
		return uc.resolveWithinGroup(ctx, query, *chosen)
	}
	next := *selection
	next.CategoryID, next.Label = chosen.ID, chosen.Label
	return categoryPrice(&next), nil
}

func (uc *DisambiguationUseCase) resolveWithinGroup(ctx context.Context, query string, group cropCategory) (DisambiguationResult, error) {
	pool, err := uc.recallPool(ctx, understandQuery(query))
	if err != nil {
		return DisambiguationResult{}, err
	}
	pool = uc.narrowToGroup(ctx, pool, group.ID)
	if len(pool) == 0 {
		pool = []fuse.Fused{unexpandedGroup(group)}
	}
	return uc.categoryCard(ctx, query, pool)
}

func (uc *DisambiguationUseCase) suggestWithinStage(ctx context.Context, text string, selection *domain.CropSelection, allowed []cropCategory) ([]cropCategory, error) {
	pool, err := uc.recallPool(ctx, understandQuery(text))
	if err != nil {
		return nil, err
	}
	if selection.Stage == domain.CropSelectionItem {
		pool = uc.narrowToGroup(ctx, pool, selection.ParentCategoryID)
	}
	if matches := uc.stageCategories(ctx, pool, selection.Stage); len(matches) > 0 {
		return limitCropCategories(matches), nil
	}
	return uc.suggestCategories(ctx, text, selection.Stage, selection.ParentLabel, allowed), nil
}

func (uc *DisambiguationUseCase) stageCategories(ctx context.Context, pool []fuse.Fused, stage string) []cropCategory {
	var out []cropCategory
	seen := map[string]bool{}
	for _, n := range uc.categoryNodes(ctx, pool) {
		id, label, level := n.id, n.label, 3
		if stage == domain.CropSelectionGroup {
			level = 2
			if n.level != 2 {
				id, label = n.parentID, n.parentLabel
			}
		} else if n.level != 3 {
			continue
		}
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, cropCategory{ID: id, Label: label, Level: level})
	}
	return out
}
