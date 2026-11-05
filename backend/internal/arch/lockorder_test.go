package arch

import (
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/lockorder"
)

// TestLockOrderIsBottomUpOverTheDependencyTable 守住 platform/lockorder.Modules 和依賴表是同一個圖：每個模組
// 都在表裡、恰好一次，而且排在它依賴的每個模組之後（規則 4「由下往上」）。platform 不能 import arch，所以順序表
// 放在 platform，由這裡對齊。
func TestLockOrderIsBottomUpOverTheDependencyTable(t *testing.T) {
	t.Parallel()
	for _, problem := range lockOrderProblems(Dependencies, lockorder.Modules) {
		t.Error(problem)
	}
}

func TestLockOrderProblemsFlagsMissingExtraAndTopDownEntries(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{"identity": {}, "buyer": {"identity"}, "social": {"buyer"}}
	got := lockOrderProblems(deps, []string{"buyer", "identity", "ghost"})
	want := []string{
		"lockorder.Modules 有依賴表沒有的模組 ghost",
		"lockorder.Modules 缺少模組 social",
		"lockorder.Modules 把 buyer 排在它依賴的 identity 之前",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("problems = %q，預期 %q", got, want)
	}
}

func lockOrderProblems(deps map[string][]string, order []string) []string {
	var out []string
	pos := map[string]int{}
	for i, m := range order {
		if _, dup := pos[m]; dup {
			out = append(out, "lockorder.Modules 重複列出 "+m)
		}
		pos[m] = i
		if _, ok := deps[m]; !ok {
			out = append(out, "lockorder.Modules 有依賴表沒有的模組 "+m)
		}
	}
	for m, ds := range deps {
		p, ok := pos[m]
		if !ok {
			out = append(out, "lockorder.Modules 缺少模組 "+m)
			continue
		}
		for _, d := range ds {
			if q, ok := pos[d]; ok && q > p {
				out = append(out, "lockorder.Modules 把 "+m+" 排在它依賴的 "+d+" 之前")
			}
		}
	}
	return out
}
