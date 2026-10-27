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

func c(code string, rank int) recall.Candidate {
	return recall.Candidate{Key: recall.CandidateKey(recall.KindCategory, code), Kind: recall.KindCategory, Code: code, Rank: rank}
}

func TestRRF_ScoresAndOrders(t *testing.T) {
	got := RRF(map[string][]recall.Candidate{
		"a": {c("A", 1), c("B", 2)},
		"b": {c("B", 1), c("C", 2)},
	})
	require.Equal(t, []string{"CATEGORY|B", "CATEGORY|A", "CATEGORY|C"}, keys(got), "B 兩路都中，分數最高")
	require.InDelta(t, 1.0/61+1.0/62, got[0].Score, 1e-9)
	require.ElementsMatch(t, []string{"a", "b"}, got[0].Sources)
}

func TestRRF_SingleSourceKeepsRetrieverOrder(t *testing.T) {
	got := RRF(map[string][]recall.Candidate{"taxonomy": {c("A", 1), c("B", 2), c("C", 3)}})
	require.Equal(t, []string{"CATEGORY|A", "CATEGORY|B", "CATEGORY|C"}, keys(got))
}

func TestRRF_PinnedComesFirstRegardlessOfScore(t *testing.T) {
	pinned := c("P", 5)
	pinned.Pinned = true
	got := RRF(map[string][]recall.Candidate{
		"a": {c("A", 1), c("B", 2)},
		"b": {pinned},
	})
	require.Equal(t, "CATEGORY|P", got[0].Key)
}

func TestRRF_MergesQualityAndCategoryIDs(t *testing.T) {
	weak := c("7", 1)
	weak.Quality, weak.CategoryIDs = recall.Weak, []string{"2011"}
	strong := c("7", 1)
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
			"x": {c("B", 1)},
			"y": {c("A", 1)},
		})
		require.Equal(t, []string{"CATEGORY|A", "CATEGORY|B"}, keys(got))
	}
}
