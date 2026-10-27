package model

import (
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"
)

// finding 是測試用的認定：只有挑主要依據會看的三個欄位。
func finding(n int, severity Severity, createdAt time.Time) Violation {
	return Violation{
		ID:       uuid.MustParse(fmt.Sprintf("0190a000-0000-7000-8000-%012x", n)),
		Severity: severity, CreatedAt: createdAt,
	}
}

func arrangements(in []Violation) [][]Violation {
	if len(in) <= 1 {
		return [][]Violation{slices.Clone(in)}
	}
	var out [][]Violation
	for i := range in {
		rest := slices.Concat(in[:i:i], in[i+1:])
		for _, p := range arrangements(rest) {
			out = append(out, append([]Violation{in[i]}, p...))
		}
	}
	return out
}

// leadOfEveryArrangement 在 cited 的每一種排列上挑主要依據，全部相同才回傳（不同就讓測試失敗）。
func leadOfEveryArrangement(t *testing.T, cited []Violation) uuid.UUID {
	t.Helper()
	var first uuid.UUID
	for i, a := range arrangements(cited) {
		lead, ok := LeadFindingOf(a)
		if !ok {
			t.Fatalf("排列 %d：沒有主要依據", i)
		}
		if i == 0 {
			first = lead.ID
		} else if lead.ID != first {
			t.Fatalf("排列 %d：主要依據 %v，第一種排列是 %v：結果不能取決於清單的順序", i, lead.ID, first)
		}
	}
	return first
}

func TestSeverityRankOrdersFromLowToCritical(t *testing.T) {
	for i, s := range SeverityValues() {
		if s.Rank() != i {
			t.Errorf("%s 的名次是 %d，應為 %d", s, s.Rank(), i)
		}
	}
	// 與 SeverityValues 的排列無關，明寫 LOW < MEDIUM < HIGH < CRITICAL。
	previous := -1
	for _, s := range []Severity{SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical} {
		if s.Rank() <= previous {
			t.Errorf("%s 的名次 %d 應高於前一個 %d", s, s.Rank(), previous)
		}
		previous = s.Rank()
	}
	if Severity("NOPE").Rank() != -1 {
		t.Error("不認得的嚴重度名次是 -1")
	}
}

// 主要依據是引用的認定中最嚴重的，不論清單怎麼排：CRITICAL > HIGH > MEDIUM > LOW；最嚴重的不是最新的也不是最舊的。
func TestLeadFindingOfIsTheMostSevereWhateverTheOrder(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	low := finding(1, SeverityLow, at.Add(4*time.Hour))
	medium := finding(2, SeverityMedium, at.Add(3*time.Hour))
	high := finding(3, SeverityHigh, at)
	critical := finding(4, SeverityCritical, at.Add(time.Hour))
	for name, c := range map[string]struct {
		cited []Violation
		want  Violation
	}{
		"四種嚴重度（CRITICAL 不是最新也不是最舊）": {[]Violation{low, medium, high, critical}, critical},
		"沒有 CRITICAL（HIGH 是最舊的）":    {[]Violation{low, medium, high}, high},
		"只有 MEDIUM 與 LOW":           {[]Violation{low, medium}, medium},
	} {
		if got := leadOfEveryArrangement(t, c.cited); got != c.want.ID {
			t.Errorf("%s：主要依據 %v，應為 %v", name, got, c.want.ID)
		}
	}
}

// 同嚴重度取最新建立的；嚴重度比建立時間優先（比較新但比較不嚴重的不會取代）。
func TestLeadFindingOfTakesTheNewestAmongTheSameSeverity(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	oldHigh := finding(1, SeverityHigh, at)
	newHigh := finding(2, SeverityHigh, at.Add(time.Hour))
	newestMedium := finding(3, SeverityMedium, at.Add(2*time.Hour))
	if got := leadOfEveryArrangement(t, []Violation{oldHigh, newHigh, newestMedium}); got != newHigh.ID {
		t.Errorf("主要依據 %v，應為最新的 HIGH %v", got, newHigh.ID)
	}
	// 兩筆同嚴重度的 CRITICAL，舊的 ID 較大：看建立時間，不看 ID。
	oldCritical := finding(9, SeverityCritical, at)
	newCritical := finding(5, SeverityCritical, at.Add(time.Minute))
	if got := leadOfEveryArrangement(t, []Violation{oldCritical, newCritical}); got != newCritical.ID {
		t.Errorf("主要依據 %v，應為較晚建立的 %v", got, newCritical.ID)
	}
}

// 嚴重度與建立時間都相同時取 ID 較大的：結果仍然只取決於這一組認定。
func TestLeadFindingOfBreaksAFullTieByTheLargerID(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	a, b, c := finding(1, SeverityHigh, at), finding(3, SeverityHigh, at), finding(2, SeverityHigh, at)
	if got := leadOfEveryArrangement(t, []Violation{a, b, c}); got != b.ID {
		t.Errorf("主要依據 %v，應為 ID 最大的 %v", got, b.ID)
	}
}

func TestLeadFindingOfOneOrNone(t *testing.T) {
	if _, ok := LeadFindingOf(nil); ok {
		t.Error("沒有認定就沒有主要依據")
	}
	only := finding(1, SeverityLow, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if lead, ok := LeadFindingOf([]Violation{only}); !ok || lead.ID != only.ID {
		t.Errorf("只有一筆就是它：%v %v", lead.ID, ok)
	}
}

// 不改動呼叫端的清單。
func TestLeadFindingOfDoesNotReorderTheList(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cited := []Violation{finding(1, SeverityLow, at), finding(2, SeverityCritical, at), finding(3, SeverityHigh, at)}
	before := slices.Clone(cited)
	LeadFindingOf(cited)
	for i := range cited {
		if cited[i].ID != before[i].ID {
			t.Fatalf("清單被改動：%v", cited)
		}
	}
}
