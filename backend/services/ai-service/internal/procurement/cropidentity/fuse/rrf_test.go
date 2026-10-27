package fuse

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
)

func keys(in []Fused) []string {
	out := make([]string, 0, len(in))
	for _, f := range in {
		out = append(out, f.Key)
	}
	return out
}

func c(kind recall.Kind, code string, rank int) recall.Candidate {
	return recall.Candidate{Key: recall.CandidateKey(kind, code), Kind: kind, Code: code, Rank: rank}
}

func TestRRF_ScoresAndOrders(t *testing.T) {
	got := RRF(map[string][]recall.Candidate{
		"entity":   {c(recall.KindCrop, "A", 1), c(recall.KindCrop, "B", 2)},
		"taxonomy": {c(recall.KindCrop, "B", 1), c(recall.KindCategory, "C", 2)},
	})
	require.Equal(t, []string{"CROP|B", "CROP|A", "CATEGORY|C"}, keys(got), "B 兩路都中，分數最高")
	require.InDelta(t, 1.0/61+1.0/62, got[0].Score, 1e-9)
	require.ElementsMatch(t, []string{"entity", "taxonomy"}, got[0].Sources)
}

func TestRRF_PinnedComesFirstRegardlessOfScore(t *testing.T) {
	pinned := c(recall.KindCategory, "P", 5)
	pinned.Pinned = true
	got := RRF(map[string][]recall.Candidate{
		"entity":   {c(recall.KindCrop, "A", 1), c(recall.KindCrop, "B", 2)},
		"taxonomy": {pinned},
	})
	require.Equal(t, "CATEGORY|P", got[0].Key)
}

func TestRRF_MergesQualityAndCategoryIDs(t *testing.T) {
	weak := c(recall.KindCommodity, "7", 1)
	weak.Quality, weak.CategoryIDs = recall.Weak, []string{"2011"}
	strong := c(recall.KindCommodity, "7", 1)
	strong.Quality, strong.CategoryIDs = recall.Strong, []string{"2258"}
	got := RRF(map[string][]recall.Candidate{"a": {weak}, "b": {strong}})
	require.Len(t, got, 1)
	require.Equal(t, recall.Strong, got[0].Quality)
	require.ElementsMatch(t, []string{"2011", "2258"}, got[0].CategoryIDs)
}

func TestRRF_IsDeterministicAcrossMapOrder(t *testing.T) {
	// 同分時依 Key 定序，不依 map 迭代順序。
	for i := 0; i < 20; i++ {
		got := RRF(map[string][]recall.Candidate{
			"x": {c(recall.KindCrop, "B", 1)},
			"y": {c(recall.KindCrop, "A", 1)},
		})
		require.Equal(t, []string{"CROP|A", "CROP|B"}, keys(got))
	}
}

func TestAbsorbCategories_CategoryJoinsEntityCarryingIt(t *testing.T) {
	entity := c(recall.KindCommodity, "7", 1)
	entity.Quality, entity.CategoryIDs = recall.Weak, []string{"2011"}
	cat := c(recall.KindCategory, "2011", 1)
	cat.Quality, cat.Pinned, cat.CategoryIDs = recall.Strong, true, []string{"2011"}
	in := RRF(map[string][]recall.Candidate{"entity": {entity}, "taxonomy": {cat}})

	got := AbsorbCategories(in)

	require.Len(t, got, 1, "分類被實體承接，不再是獨立候選")
	require.Equal(t, "COMMODITY|7", got[0].Key)
	require.Equal(t, recall.Strong, got[0].Quality, "承接後品質取強者")
	require.True(t, got[0].Pinned)
	require.ElementsMatch(t, []string{"entity", "taxonomy"}, got[0].Sources)
	require.InDelta(t, 2.0/61, got[0].Score, 1e-9, "分數相加")
}

func TestAbsorbCategories_UnclaimedCategoryStays(t *testing.T) {
	cat := c(recall.KindCategory, "2008", 1)
	cat.Quality, cat.CategoryIDs = recall.Strong, []string{"2008"}
	got := AbsorbCategories(RRF(map[string][]recall.Candidate{"taxonomy": {cat}}))
	require.Len(t, got, 1)
	require.Equal(t, "CATEGORY|2008", got[0].Key)
}

func TestAbsorbCategories_CategoryClaimedByTwoEntitiesBoostsBoth(t *testing.T) {
	a := c(recall.KindCommodity, "7", 1)
	a.CategoryIDs = []string{"2011"}
	b := c(recall.KindCommodity, "9", 2)
	b.CategoryIDs = []string{"2011"}
	cat := c(recall.KindCategory, "2011", 1)
	cat.Quality, cat.CategoryIDs = recall.Strong, []string{"2011"}
	got := AbsorbCategories(RRF(map[string][]recall.Candidate{"entity": {a, b}, "taxonomy": {cat}}))
	require.Equal(t, []string{"COMMODITY|7", "COMMODITY|9"}, keys(got))
	require.Equal(t, recall.Strong, got[1].Quality)
}
