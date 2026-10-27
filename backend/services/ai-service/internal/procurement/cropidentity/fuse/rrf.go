// Package fuse 把多個 retriever 的名次序列融合成一份排序（Reciprocal Rank Fusion），
// 再把 taxonomy 的分類候選併進「已映射到該分類」的實體候選。
package fuse

import (
	"sort"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
)

// K 是 RRF 的平滑常數；60 是原論文與 Elasticsearch／Vespa 的預設。
const K = 60

// Fused 是融合後的一列：同 Key 的候選合成一列，Quality 取最高、CategoryIDs 取聯集、
// Pinned 取或、其餘欄位取第一次出現的。Score 只用來排序，不外露、不設門檻——
// 它依賴 retriever 數量與候選數，跨查詢不可比較。
type Fused struct {
	recall.Candidate
	Score   float64
	Sources []string
}

// RRF：score = Σ_r 1/(K + rank_r)。pinned 永遠排在非 pinned 之前；其餘依分數降冪、
// 同分依 Key，讓輸出不受 map 迭代順序影響。
func RRF(results map[string][]recall.Candidate) []Fused {
	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}
	sort.Strings(names)

	byKey := map[string]*Fused{}
	order := []string{}
	for _, name := range names {
		for _, c := range results[name] {
			f, ok := byKey[c.Key]
			if !ok {
				f = &Fused{Candidate: c}
				f.CategoryIDs = append([]string(nil), c.CategoryIDs...)
				byKey[c.Key] = f
				order = append(order, c.Key)
			} else {
				merge(f, c)
			}
			f.Score += 1.0 / float64(K+c.Rank)
			f.Sources = append(f.Sources, name)
		}
	}
	out := make([]Fused, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
	}
	sortFused(out)
	return out
}

// AbsorbCategories 把 CATEGORY 候選併進 CategoryIDs 含該分類的實體候選：分數相加、
// Sources 合併、Quality／Pinned 取強者。沒有實體承接的 CATEGORY 保留為獨立候選。
// 一個分類被多個實體承接時每一個都得到加成（它們對這個分類是等價的答案）。
func AbsorbCategories(in []Fused) []Fused {
	var categories, entities []Fused
	for _, f := range in {
		if f.Kind == recall.KindCategory {
			categories = append(categories, f)
		} else {
			entities = append(entities, f)
		}
	}
	out := make([]Fused, 0, len(in))
	claimed := map[string]bool{}
	for _, e := range entities {
		for _, cat := range categories {
			if !containsID(e.CategoryIDs, cat.Code) {
				continue
			}
			claimed[cat.Key] = true
			e.Score += cat.Score
			e.Sources = appendUnique(e.Sources, cat.Sources...)
			if cat.Quality > e.Quality {
				e.Quality = cat.Quality
			}
			e.Pinned = e.Pinned || cat.Pinned
		}
		out = append(out, e)
	}
	for _, cat := range categories {
		if !claimed[cat.Key] {
			out = append(out, cat)
		}
	}
	sortFused(out)
	return out
}

func merge(f *Fused, c recall.Candidate) {
	if c.Quality > f.Quality {
		f.Quality = c.Quality
	}
	f.Pinned = f.Pinned || c.Pinned
	for _, id := range c.CategoryIDs {
		if !containsID(f.CategoryIDs, id) {
			f.CategoryIDs = append(f.CategoryIDs, id)
		}
	}
	if f.MatchedBy == "" {
		f.MatchedBy = c.MatchedBy
	}
	if !f.HasForecast && c.HasForecast {
		f.HasForecast = c.HasForecast
	}
}

func sortFused(out []Fused) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Key < out[j].Key
	})
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func appendUnique(dst []string, src ...string) []string {
	for _, s := range src {
		if !containsID(dst, s) {
			dst = append(dst, s)
		}
	}
	return dst
}
