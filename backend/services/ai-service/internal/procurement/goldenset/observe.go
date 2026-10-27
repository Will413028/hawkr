package goldenset

import "github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"

// ObserveFrom 把 Resolve 的結果攤平成名次序列，供指標計算：需消歧＝候選順序即名次；
// 沒有更多東西要問（Unavailable）時，Selection 帶著買家確認的最深分類就是唯一一名，
// 沒有確認分類就是 NIL（逃生艙）。
func ObserveFrom(q Query, res cropidentity.DisambiguationResult, pathOf func(string) string) Observation {
	o := Observation{Query: q}

	if res.NeedsDisambiguation {
		sawCandidate := false
		for _, c := range res.Candidates {
			sawCandidate = true
			if c.CategoryID == "" {
				continue
			}
			if p := pathOf(c.CategoryID); p != "" {
				o.RankedCategories = append(o.RankedCategories, p)
			}
		}
		if sawCandidate && len(o.RankedCategories) == 0 {
			o.Unmappable = true
		}
		return o
	}

	if id, _ := res.Selection.ConfirmedCategory(); id != "" {
		if p := pathOf(id); p != "" {
			o.RankedCategories = []string{p}
			return o
		}
		o.Unmappable = true
		return o
	}
	o.IsNIL = true
	return o
}
