package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const sampleTree = `{"code":0,"message":"ok","data":[
 {"id":"1","name":"蔬菜","parentId":"","status":"9004","children":[
   {"id":"117","name":"甘藍","parentId":"1","status":"9004","children":[
     {"id":"311","name":"初秋","parentId":"117","status":"9004"}]},
   {"id":"99","name":"停售菜","parentId":"1","status":"9099","children":[]}]},
 {"id":"9","name":"花卉","parentId":"","status":"9004","children":[
   {"id":"900","name":"文心蘭","parentId":"9","status":"9004","children":[]}]}]}`

func TestParseTree_ExcludesL1AndInactive(t *testing.T) {
	got, err := parseTree([]byte(sampleTree), []string{"花卉"})
	require.NoError(t, err)

	require.Len(t, got.L2, 1, "花卉底下的 L2 要被排除，停售的也要")
	require.Equal(t, "甘藍", got.L2[0].Name)
	require.Equal(t, "117", got.L2[0].ID)
	require.Equal(t, "蔬菜", got.L2[0].L1Name)
	require.Len(t, got.L2[0].Children, 1)
	require.Equal(t, "初秋", got.L2[0].Children[0].Name)
}

// All 是「所有層級、所有 L1」的完整節點清單，供撞名檢查與快照使用——
// 排除花卉只影響要產別名的對象，不影響「別名不得撞既有節點名」的比對範圍。
func TestParseTree_AllKeepsExcludedL1(t *testing.T) {
	got, err := parseTree([]byte(sampleTree), []string{"花卉"})
	require.NoError(t, err)

	names := map[string]bool{}
	for _, n := range got.All {
		names[n.Name] = true
	}
	require.True(t, names["文心蘭"], "被排除的 L1 底下的節點仍要進撞名集合")
	require.True(t, names["初秋"])
	require.False(t, names["停售菜"], "非 ACTIVE 的節點不算既有節點名")
}

func TestParseTree_RejectsMalformed(t *testing.T) {
	_, err := parseTree([]byte(`{"data":[]}`), nil)
	require.Error(t, err)
}
