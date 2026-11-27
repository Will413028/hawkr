package turn

import (
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"uuid"
)

type pendingCard struct {
	payload InterruptPayload
	card    *planner.Question
}

func buildCard(query string, ms []resolver.Match) *pendingCard {
	ms = leavesOnly(ms)
	if len(ms) == 0 {
		return describeCard(query)
	}
	if len(ms) > planner.MaxItemOptions {
		if groups := groupsOf(ms); len(groups) >= 2 {
			return cardOf(StageGroup, query, groups, planner.BuildGroupDisambiguation(groups))
		}
	}
	cands := make([]planner.ItemCandidate, 0, len(ms))
	for _, m := range ms {
		cands = append(cands, planner.ItemCandidate{CategoryID: m.CategoryID, Label: m.Name, Path: m.Path})
	}
	c := cardOf(StageItem, query, cands, planner.BuildItemDisambiguation(cands))
	if root := commonRoot(ms); root != uuid.Nil() {
		c.payload.Root = &root
	}
	return c
}

func commonRoot(ms []resolver.Match) uuid.UUID {
	var root uuid.UUID
	for _, m := range ms {
		if m.RootID == uuid.Nil() || (root != uuid.Nil() && m.RootID != root) {
			return uuid.Nil()
		}
		root = m.RootID
	}
	return root
}

func leavesOnly(ms []resolver.Match) []resolver.Match {
	var out []resolver.Match
	for _, m := range ms {
		if m.Leaf && m.CategoryID != m.RootID {
			out = append(out, m)
		}
	}
	return out
}

func describeCard(query string) *pendingCard {
	return cardOf(StageDescribe, query, nil, planner.BuildItemDescribe())
}

func cardOf(stage, query string, cands []planner.ItemCandidate, card *planner.Question) *pendingCard {
	shown := cands[:min(len(cands), planner.MaxItemOptions)]
	p := InterruptPayload{Stage: stage, Query: query, Candidates: make([]PayloadCandidate, 0, len(shown))}
	for _, c := range shown {
		p.Candidates = append(p.Candidates, PayloadCandidate{CategoryID: c.CategoryID, Label: c.Label, Path: c.Path})
	}
	return &pendingCard{payload: p, card: card}
}

func groupsOf(ms []resolver.Match) []planner.ItemCandidate {
	var out []planner.ItemCandidate
	seen := map[uuid.UUID]bool{}
	for _, m := range ms {
		if m.RootID == uuid.Nil() || m.RootName == "" {
			return nil
		}
		if seen[m.RootID] {
			continue
		}
		seen[m.RootID] = true
		out = append(out, planner.ItemCandidate{CategoryID: m.RootID, Label: m.RootName, Path: m.RootName})
	}
	return out
}
