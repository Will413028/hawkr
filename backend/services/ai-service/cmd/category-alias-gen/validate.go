package main

import "strings"

// candidate 是一條待驗證的別名。
type candidate struct {
	Alias  string
	Target node
}

// rejection 記錄被丟棄的別名與理由，印到 stderr 供掃視系統性問題。
type rejection struct {
	Alias  string
	Target string
	Reason string
}

const (
	reasonTargetNotL2 = "target 不存在或非 ACTIVE L2"
	reasonAliasIsNode = "alias 撞既有節點名"
	reasonManualOwned = "撞人工覆寫層"
	reasonAmbiguous   = "alias 映到多個 L2"
)

// validateCandidates 是「不逐列人工覆核」前提下唯一能防住結構性汙染的東西。
// 三道檢查都是機械的，不需要人判斷：
//
//  1. target 必須是真實存在、ACTIVE、Level=2 的節點——模型編出樹上沒有的名字整列丟棄。
//  2. alias 不得等於任何既有節點名（L1／L2／L3 皆不可）。最重要的一道：樹上同時有
//     「香菇」與「濕香菇」，若收下 香菇→濕香菇，買家打「香菇」就再也命中不了那個節點。
//  3. 同一個 alias 不得映到多個 L2——衝突的整組丟棄，寧可 miss 也不要導向錯處。
func validateCandidates(cands []candidate, t tree, manual map[string]string) ([]candidate, []rejection) {
	nodeNames := make(map[string]bool, len(t.All))
	for _, n := range t.All {
		nodeNames[n.Name] = true
	}
	validL2 := make(map[string]node, len(t.L2))
	for _, n := range t.L2 {
		validL2[n.ID] = n
	}

	targetsByAlias := make(map[string]map[string]bool)
	for _, c := range cands {
		alias := strings.TrimSpace(c.Alias)
		if targetsByAlias[alias] == nil {
			targetsByAlias[alias] = make(map[string]bool)
		}
		targetsByAlias[alias][c.Target.ID] = true
	}

	var accepted []candidate
	var rejected []rejection
	seen := make(map[string]bool)
	for _, c := range cands {
		alias := strings.TrimSpace(c.Alias)
		reject := func(reason string) {
			rejected = append(rejected, rejection{Alias: alias, Target: c.Target.Name, Reason: reason})
		}
		switch {
		case alias == "":
			reject(reasonAliasIsNode)
		case validL2[c.Target.ID].Name != c.Target.Name || c.Target.Name == "":
			reject(reasonTargetNotL2)
		case nodeNames[alias]:
			reject(reasonAliasIsNode)
		case manual[alias] != "":
			reject(reasonManualOwned)
		case len(targetsByAlias[alias]) > 1:
			reject(reasonAmbiguous)
		case seen[alias]:
			// 同 alias 同 target 的重複樣本，靜默去重，不算拒絕。
		default:
			seen[alias] = true
			accepted = append(accepted, candidate{Alias: alias, Target: c.Target})
		}
	}
	return accepted, rejected
}
