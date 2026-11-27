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

// buildCard 是「依命中出卡」的唯一規則（A9-D28(1)）：命中多於 MaxItemOptions 個而且分散在兩個以上大類時先問大類（GROUP，依各大類的最佳命中排序）；
// 其餘列末層品項（ITEM，照 resolver 的排序，最多 MaxItemOptions 個，同一大類超過也只取前幾個）。呼叫端給的命中已排除無效的。
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

// commonRoot 是命中共同的大類 ID；沒有大類資訊或分屬多個大類回 uuid.Nil()。
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

// leavesOnly 只留末層命中：大類、中間層節點與沒有子節點的大類自己都不能被確認，也不計入出卡的門檻與分組。
func leavesOnly(ms []resolver.Match) []resolver.Match {
	var out []resolver.Match
	for _, m := range ms {
		if m.Leaf && m.CategoryID != m.RootID {
			out = append(out, m)
		}
	}
	return out
}

// describeCard 是自由描述卡：沒有選項。
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

// groupsOf 依命中順序（resolver 的排序）取不重複的大類；任何命中沒有大類資訊時回 nil，不分組。
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
