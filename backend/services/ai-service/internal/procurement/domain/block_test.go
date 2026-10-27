package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlockKind_AllSixValuesAreDistinctNonEmptyStrings(t *testing.T) {
	kinds := []BlockKind{BlockUnderstood, BlockContext, BlockRationale, BlockWarning, BlockNextStep, BlockComplete}
	seen := map[BlockKind]bool{}
	for _, k := range kinds {
		require.NotEmpty(t, string(k))
		require.False(t, seen[k], "duplicate BlockKind value: %s", k)
		seen[k] = true
	}
	require.Len(t, seen, 6)
}

func TestBlock_StructHoldsKindAndText(t *testing.T) {
	b := Block{Kind: BlockWarning, Text: "有幾個內容我沒辦法直接採用。"}
	require.Equal(t, BlockWarning, b.Kind)
	require.Equal(t, "有幾個內容我沒辦法直接採用。", b.Text)
}
