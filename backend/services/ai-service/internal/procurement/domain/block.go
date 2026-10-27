package domain

// BlockKind 是 PublicAgentMessageV1.blocks[] 的語意分類（issue #2549 §F）。
// 6 個值窮盡 §F 定義的集合，不是每個 act 都會用滿全部 6 種。
type BlockKind string

const (
	BlockUnderstood BlockKind = "UNDERSTOOD"
	BlockContext    BlockKind = "CONTEXT"
	BlockRationale  BlockKind = "RATIONALE"
	BlockWarning    BlockKind = "WARNING"
	BlockNextStep   BlockKind = "NEXT_STEP"
	BlockComplete   BlockKind = "COMPLETE"
)

// Block 是一個型別化的敘述段落，取代 Phase 1 的扁平 Reply string。
type Block struct {
	Kind BlockKind `json:"kind"`
	Text string    `json:"text"`
}
