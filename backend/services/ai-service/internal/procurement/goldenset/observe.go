package goldenset

import "github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"

// ObserveFrom 把現行 Resolve 的三態攤平成名次序列，供指標計算。
//
// 現行架構沒有 ranked list 的概念（見 spec 的根因段），所以這裡是一層轉譯：
// 已定案＝唯一一名、需消歧＝候選順序即名次、Unavailable＝NIL。重構之後
// recall/fuse 會直接產出名次，這個轉譯層就退場。
//
// ⚠️ 只有帶 CategoryID 的結果映射得到分類路徑。crop 路徑（pricebook 直接命中）
// 帶的是 crop code、不經分類樹，一律標 Unmappable——那是工具的限制不是解析錯誤，
// 兩者混在一起會讓 baseline 虛低。
func ObserveFrom(q Query, res cropidentity.DisambiguationResult, pathOf func(string) string) Observation {
	o := Observation{Query: q, ResolvedLabel: res.Label}

	if res.Unavailable {
		o.IsNIL = true
		return o
	}

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

	// 已定案：分類來自買家確認過的最深節點，不是 CommodityCode（後者是 pricebook
	// 的 commodity code，不是分類樹節點 id）。
	if id, _ := res.Selection.ConfirmedCategory(); id != "" {
		if p := pathOf(id); p != "" {
			o.RankedCategories = []string{p}
			return o
		}
	}
	o.Unmappable = true
	return o
}
