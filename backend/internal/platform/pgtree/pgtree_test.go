package pgtree

import (
	"slices"
	"testing"
)

func TestWalkVisitsKeysInSortedOrderEveryTime(t *testing.T) {
	t.Parallel()
	tree := map[string]any{"b": map[string]any{"x": 1}, "a": map[string]any{"y": 2}, "c": []any{map[string]any{"z": 3}}}
	var first []string
	for i := 0; i < 50; i++ {
		var got []string
		Walk(tree, func(m map[string]any) {
			for k := range m {
				if len(m) == 1 {
					got = append(got, k)
				}
			}
		})
		if first == nil {
			first = got
		} else if !slices.Equal(first, got) {
			t.Fatalf("order changed: %v vs %v", first, got)
		}
	}
	if want := []string{"y", "x", "z"}; !slices.Equal(first, want) {
		t.Fatalf("order = %v, want %v", first, want)
	}
}

func TestParseRejectsInvalidSQL(t *testing.T) {
	t.Parallel()
	if _, err := Parse("SELEC 1"); err == nil {
		t.Fatal("want error")
	}
}
