package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeAliasSource struct {
	byTarget map[string][][]string
	calls    map[string]int
	siblings map[string][]string
}

func (f *fakeAliasSource) Aliases(_ context.Context, target node, siblings []string) ([]string, error) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	if f.siblings == nil {
		f.siblings = map[string][]string{}
	}
	f.siblings[target.ID] = siblings
	i := f.calls[target.ID]
	f.calls[target.ID]++
	rounds := f.byTarget[target.ID]
	if i >= len(rounds) {
		return nil, nil
	}
	return rounds[i], nil
}

// 共識過濾是「不逐列人工覆核」的替代機制：穩定的俗名三次都會出現，模型半猜
// 的邊緣詞很難連中兩次。
func TestConsensus(t *testing.T) {
	got := consensus([][]string{
		{"捲心菜", "包心菜", "玻璃菜"},
		{"捲心菜", "包心菜"},
		{"捲心菜", "結球甘藍"},
	}, 2)

	require.ElementsMatch(t, []string{"捲心菜", "包心菜"}, got)
	require.NotContains(t, got, "玻璃菜", "只出現一次的要丟掉")
	require.NotContains(t, got, "結球甘藍")
}

func TestConsensus_DeduplicatesWithinOneSample(t *testing.T) {
	got := consensus([][]string{{"捲心菜", "捲心菜"}, {"包心菜"}}, 2)
	require.Empty(t, got, "同一次取樣裡重複不算兩票")
}

func TestGenerateAll_AppliesConsensusPerNode(t *testing.T) {
	cabbage := node{ID: "117", Name: "甘藍", Level: 2, L1Name: "蔬菜"}
	burdock := node{ID: "64", Name: "牛蒡", Level: 2, L1Name: "蔬菜"}
	src := &fakeAliasSource{byTarget: map[string][][]string{
		"117": {{"捲心菜", "玻璃菜"}, {"捲心菜"}, {"捲心菜"}},
		"64":  {{"牛蒡根"}, {"牛旁"}, {"牛蒡根"}},
	}}

	got, err := generateAll(context.Background(), src, tree{L2: []node{cabbage, burdock}}, 3, 2)

	require.NoError(t, err)
	require.Equal(t, 3, src.calls["117"], "每個節點都要跑滿 samples 次")
	require.Len(t, got, 2)
	require.Equal(t, "捲心菜", got[0].Alias)
	require.Equal(t, "117", got[0].Target.ID)
	require.Equal(t, "牛蒡根", got[1].Alias)
}

// 兄弟清單是排除清單：形近節點並列，模型才不會把「芥藍」丟給「甘藍」。
func TestGenerateAll_PassesSameL1SiblingsAsExclusionList(t *testing.T) {
	cabbage := node{ID: "117", Name: "甘藍", Level: 2, L1Name: "蔬菜"}
	kale := node{ID: "118", Name: "芥藍菜", Level: 2, L1Name: "蔬菜"}
	orchid := node{ID: "900", Name: "文心蘭", Level: 2, L1Name: "花卉"}
	src := &fakeAliasSource{byTarget: map[string][][]string{}}

	_, err := generateAll(context.Background(), src, tree{L2: []node{cabbage, kale, orchid}}, 1, 1)

	require.NoError(t, err)
	require.Equal(t, []string{"芥藍菜"}, src.siblings["117"], "只帶同 L1 的兄弟，且不含自己")
	require.Equal(t, []string{"甘藍"}, src.siblings["118"])
	require.Empty(t, src.siblings["900"], "花卉只有一個節點時沒有兄弟")
}
