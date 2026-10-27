package stats

import (
	"slices"
	"testing"
	"uuid"
)

// Sorted 是鎖多列 stats 的固定順序：去重、依 ID 由小到大，不依輸入的順序（清除 job 不能依 Go 的 map 迭代順序鎖）。
func TestSortedIsDeduplicatedAndIndependentOfInputOrder(t *testing.T) {
	a, b, c := uuid.MustParse("00000000-0000-7000-8000-000000000001"), uuid.MustParse("00000000-0000-7000-8000-000000000002"), uuid.MustParse("ffffffff-0000-7000-8000-000000000003")
	want := []uuid.UUID{a, b, c}
	for _, in := range [][]uuid.UUID{{c, b, a}, {b, a, c, a, b}, {a, b, c}, {c, c, c, b, a}} {
		orig := slices.Clone(in)
		if got := Sorted(in); !slices.Equal(got, want) {
			t.Errorf("Sorted(%v) = %v，要 %v", in, got, want)
		}
		if !slices.Equal(in, orig) {
			t.Errorf("Sorted 不得改動輸入：%v → %v", orig, in)
		}
	}
	if got := Sorted(nil); len(got) != 0 {
		t.Errorf("Sorted(nil) = %v", got)
	}
}
