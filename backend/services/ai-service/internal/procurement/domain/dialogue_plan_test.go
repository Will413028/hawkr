package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllActCodes_Has18UniqueValues(t *testing.T) {
	require.Len(t, AllActCodes, 18)
	seen := map[ActCode]bool{}
	for _, c := range AllActCodes {
		require.False(t, seen[c], "duplicate ActCode: %s", c)
		seen[c] = true
	}
}

func TestPlannableActCodes_Has10UniqueValuesAllInAllActCodes(t *testing.T) {
	require.Len(t, PlannableActCodes, 10)
	all := map[ActCode]bool{}
	for _, c := range AllActCodes {
		all[c] = true
	}
	seen := map[ActCode]bool{}
	for _, c := range PlannableActCodes {
		require.True(t, all[c], "PlannableActCodes contains a code not in AllActCodes: %s", c)
		require.False(t, seen[c], "duplicate PlannableActCode: %s", c)
		seen[c] = true
	}
}
