package cropidentity

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity/recall"
)

// understandQuery 把買家原文展開成查詢變體：原文、正規化（繁簡／全半形／標點／臺→台）、
// 人工與生成別名改寫（高麗菜→甘藍）。去重、保序；空白輸入沒有變體。
// 這裡只負責把一句話變成幾種問法，讓每個 retriever 各自去比對。
func understandQuery(text string) recall.Query {
	q := recall.Query{Text: text}
	if strings.TrimSpace(text) == "" {
		return q
	}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || slices.Contains(q.Variants, s) {
			return
		}
		q.Variants = append(q.Variants, s)
	}
	add(text)
	normalized := NormalizeCategoryText(text)
	add(normalized)
	add(cache.CanonicalCategoryName(strings.TrimSpace(text)))
	add(cache.CanonicalCategoryName(normalized))
	for _, base := range []string{strings.TrimSpace(text), normalized} {
		relaxed := stripQualifiers(base)
		if relaxed == "" || relaxed == base {
			continue
		}
		add(relaxed)
		add(cache.CanonicalCategoryName(relaxed))
	}
	return q
}

var queryQualifiers = []string{
	"臺灣", "台灣", "本產", "進口", "國產", "新鮮", "有機", "冷凍", "冷藏", "特級", "頂級", "嚴選",
}

const minRelaxedRunes = 2

func stripQualifiers(text string) string {
	out := strings.TrimSpace(text)
	for changed := true; changed; {
		changed = false
		for _, qual := range queryQualifiers {
			trimmed := strings.TrimPrefix(out, qual)
			if trimmed == out {
				continue
			}
			trimmed = strings.TrimPrefix(strings.TrimSpace(trimmed), "的")
			trimmed = strings.TrimSpace(trimmed)
			if utf8.RuneCountInString(trimmed) < minRelaxedRunes {
				continue
			}
			out, changed = trimmed, true
			break
		}
	}
	return out
}
