package purge

import (
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
)

func TestDistinctTargetsDedupesAndSorts(t *testing.T) {
	t.Parallel()
	got := distinctTargets([]int64{2, 1, 2, 1, 2}, []int64{9, 5, 9, 6, 8})
	want := []model.Target{{StoreID: 1, ProductID: 5}, {StoreID: 1, ProductID: 6}, {StoreID: 2, ProductID: 8}, {StoreID: 2, ProductID: 9}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
