package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func fixtureTree() tree {
	cabbage := node{ID: "117", Name: "甘藍", Level: 2, L1Name: "蔬菜",
		Children: []node{{ID: "311", Name: "初秋", Level: 3, L1Name: "蔬菜"}}}
	mushroom := node{ID: "70", Name: "濕香菇", Level: 2, L1Name: "蔬菜"}
	return tree{
		All: []node{
			{ID: "1", Name: "蔬菜", Level: 1, L1Name: "蔬菜"},
			cabbage, {ID: "311", Name: "初秋", Level: 3, L1Name: "蔬菜"},
			mushroom, {ID: "71", Name: "香菇", Level: 2, L1Name: "蔬菜"},
		},
		L2: []node{cabbage, mushroom},
	}
}

func reasonsFor(rejected []rejection, alias string) []string {
	var out []string
	for _, r := range rejected {
		if r.Alias == alias {
			out = append(out, r.Reason)
		}
	}
	return out
}

func TestValidateCandidates(t *testing.T) {
	tr := fixtureTree()
	manual := map[string]string{"高麗菜": "甘藍"}

	cands := []candidate{
		{Alias: "捲心菜", Target: tr.L2[0]},
		{Alias: "不存在菜", Target: node{ID: "999", Name: "幽靈", Level: 2}},
		{Alias: "初秋", Target: tr.L2[0]},
		{Alias: "蔬菜", Target: tr.L2[0]},
		{Alias: "香菇", Target: tr.L2[1]},
		{Alias: "高麗菜", Target: tr.L2[0]},
		{Alias: "兩面討好", Target: tr.L2[0]},
		{Alias: "兩面討好", Target: tr.L2[1]},
	}

	accepted, rejected := validateCandidates(cands, tr, manual)

	require.Len(t, accepted, 1)
	require.Equal(t, "捲心菜", accepted[0].Alias)
	require.Equal(t, "甘藍", accepted[0].Target.Name)

	require.Equal(t, []string{reasonTargetNotL2}, reasonsFor(rejected, "不存在菜"))
	require.Equal(t, []string{reasonAliasIsNode}, reasonsFor(rejected, "初秋"))
	require.Equal(t, []string{reasonAliasIsNode}, reasonsFor(rejected, "蔬菜"))
	require.Equal(t, []string{reasonAliasIsNode}, reasonsFor(rejected, "香菇"),
		"樹上同時有香菇與濕香菇時，收下 香菇→濕香菇 會讓買家再也命中不了香菇本身")
	require.Equal(t, []string{reasonManualOwned}, reasonsFor(rejected, "高麗菜"))
	require.Len(t, reasonsFor(rejected, "兩面討好"), 2, "衝突的整組都要丟掉，不是留一個")
}

func TestValidateCandidates_TargetMustBeL2NotL3(t *testing.T) {
	tr := fixtureTree()
	l3 := node{ID: "311", Name: "初秋", Level: 3, L1Name: "蔬菜"}

	accepted, rejected := validateCandidates([]candidate{{Alias: "早秋", Target: l3}}, tr, nil)

	require.Empty(t, accepted)
	require.Equal(t, []string{reasonTargetNotL2}, reasonsFor(rejected, "早秋"))
}
