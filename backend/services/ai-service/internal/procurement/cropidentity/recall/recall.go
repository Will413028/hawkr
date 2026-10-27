// Package recall 是品項解析 v2 的召回層：每個來源實作 Retriever，各自產出帶名次的
// Candidate；融合（fuse）只看名次，不看分數，所以來源之間不必「分數可比較」。
package recall

import "context"

type Kind string

const (
	// KindCategory 是 taxonomy 命中：Code 是分類 id。
	KindCategory Kind = "CATEGORY"
)

// Quality 是匹配品質分層（spec §排序與決策）：決策只看層級，不看 RRF 分數。
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

// Candidate 是任一 retriever 的一列。Key 是融合用的身分（kind|code）。
type Candidate struct {
	Key   string
	Kind  Kind
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
	// Rank 是該 retriever 內的名次（1-based）。
	Rank int
	// Pinned：人工覆寫的命中，融合時永遠排在最前（不移除其他候選）。
	Pinned bool
}

type Retriever interface {
	Name() string
	Recall(ctx context.Context, q Query) ([]Candidate, error)
}

func CandidateKey(kind Kind, code string) string { return string(kind) + "|" + code }
