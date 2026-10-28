// Package recall 是品項解析的召回結果型別：分類樹召回產出依優先序排好的 Candidate，
// 決策只看匹配品質層級。
package recall

// Quality 是匹配品質分層：決策只看層級。
type Quality int

const (
	Weak   Quality = iota // 前綴／子串
	Strong                // 分類名精確
)

// Query 是一次解析的輸入：原文與 query understanding 展開的變體（至少含原文）。
type Query struct {
	Text     string
	Variants []string
}

// Candidate 是分類樹召回的一列；Code 是分類 id。
type Candidate struct {
	Code  string
	Label string
	// CategoryIDs：CATEGORY 候選是 {Code}。
	CategoryIDs []string
	// CategoryLevel／ParentID／ParentLabel：L3 的 L2 父節點決定 ITEM 卡的 ParentCategoryID。
	CategoryLevel int
	ParentID      string
	ParentLabel   string
	// MatchedBy 是命中的分類名，供消歧卡顯示理由。
	MatchedBy string
	Quality   Quality
	// Pinned：人工覆寫的命中，永遠排在最前（不移除其他候選）。
	Pinned bool
}
