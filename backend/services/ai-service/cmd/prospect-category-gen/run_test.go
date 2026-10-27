package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testNodes() map[string]node {
	return map[string]node{
		"4":   {ID: "4", Name: "肉類", Level: 1, L1Name: "肉類"},
		"198": {ID: "198", Name: "豬肉", Level: 2, L1Name: "肉類", L2Name: "豬肉"},
		"201": {ID: "201", Name: "雞肉", Level: 2, L1Name: "肉類", L2Name: "雞肉"},
	}
}

func TestConsensus_KeepsOnlyIDsSeenEnoughTimes(t *testing.T) {
	agreed, dropped := consensus([][]string{{"198", "201"}, {"198"}}, 2, testNodes())

	require.Equal(t, []string{"198"}, agreed)
	require.Len(t, dropped, 1)
	require.Contains(t, dropped[0], "雞肉")
	require.Contains(t, dropped[0], "只出現 1 次")
}

func TestConsensus_DropsIDsOutsideTheTree(t *testing.T) {
	agreed, dropped := consensus([][]string{{"9999"}, {"9999"}}, 2, testNodes())

	require.Empty(t, agreed, "模型捏造的 id 不得進入標註")
	require.Len(t, dropped, 1)
	require.Contains(t, dropped[0], "不在分類樹")
}

func TestConsensus_DedupesWithinOneRound(t *testing.T) {
	agreed, _ := consensus([][]string{{"198", "198"}, {"201"}}, 2, testNodes())

	require.Empty(t, agreed, "同一輪內重複不重複計票")
}

func TestRenderMigration_EmitsOneRowPerTag(t *testing.T) {
	sql := renderMigration([]annotation{
		{ProspectID: 700000001, DisplayName: "究好豬", CategoryIDs: []string{"198"}},
		{ProspectID: 700000002, DisplayName: "台畜", CategoryIDs: []string{"4", "198"}},
		{ProspectID: 700000003, DisplayName: "無法判斷"},
	})

	require.Contains(t, sql, "(700000001::bigint, '198'::varchar)")
	require.Contains(t, sql, "(700000002::bigint, '4'::varchar)")
	require.Contains(t, sql, "(700000002::bigint, '198'::varchar)")
	require.Contains(t, sql, "JOIN seller_prospects sp ON sp.id = v.prospect_id",
		"乾淨環境沒有這些業者時要安全地插入 0 列，不是讓 migration 失敗")
	require.NotContains(t, sql, "700000003", "沒有標籤的業者不產生列")
	require.Contains(t, sql, "ON CONFLICT DO NOTHING")
	require.Equal(t, 3, strings.Count(sql, "\n  ("), "三筆標籤各一列")
}

func TestRenderMigration_NoTagsProducesNoInsert(t *testing.T) {
	sql := renderMigration([]annotation{{ProspectID: 1, DisplayName: "無法判斷"}})

	require.NotContains(t, sql, "INSERT")
}
