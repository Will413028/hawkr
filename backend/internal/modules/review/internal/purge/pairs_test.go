package purge

import (
	"slices"
	"testing"
	"uuid"
)

func TestDistinctProductIDsDedupesAndSorts(t *testing.T) {
	t.Parallel()
	got := distinctProductIDs([]uuid.UUID{{15: 9}, {15: 5}, {15: 9}, {15: 6}, {15: 8}})
	want := []uuid.UUID{{15: 5}, {15: 6}, {15: 8}, {15: 9}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
