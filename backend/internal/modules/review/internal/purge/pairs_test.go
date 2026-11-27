package purge

import (
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
)

func TestDistinctTargetsDedupesAndSorts(t *testing.T) {
	t.Parallel()
	got := distinctTargets([]int64{2, 1, 2, 1, 2}, []uuid.UUID{{15: 9}, {15: 5}, {15: 9}, {15: 6}, {15: 8}})
	want := []model.Target{{StoreID: 1, ProductID: uuid.UUID{15: 5}}, {StoreID: 1, ProductID: uuid.UUID{15: 6}}, {StoreID: 2, ProductID: uuid.UUID{15: 8}}, {StoreID: 2, ProductID: uuid.UUID{15: 9}}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
