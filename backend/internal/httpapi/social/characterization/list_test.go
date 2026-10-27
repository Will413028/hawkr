package characterization

import "testing"

// 情境的清單是計畫 A7-1a 約定的 28 條，名字不重複；舊的一端跳過的情境要在名字或說明裡帶 SD 編號。
func TestScenarioList(t *testing.T) {
	if len(scenarios) != 28 {
		t.Fatalf("情境 = %d 條，計畫約定 28 條", len(scenarios))
	}
	seen := map[string]bool{}
	for _, s := range scenarios {
		if seen[s.Name] {
			t.Errorf("情境名稱重複：%s", s.Name)
		}
		seen[s.Name] = true
	}
}
