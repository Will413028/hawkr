package domain

import "github.com/Will413028/hawkr/go-pkg/codemeta"

const (
	DefaultPageSize int32 = 20
	// MaxPageSize bounds consumer-facing paginated lists (comments, posts,
	// forum threads): without a cap, page_size is an unbounded full-table
	// read.
	MaxPageSize int32 = 50
	// MaxModerationPageSize is the wider cap for admin/moderation queues,
	// which legitimately need to pull bigger batches than a consumer feed.
	MaxModerationPageSize int32 = 100
	PreviewMaxLength      int   = 100
)

// 互動者的身分種類（actor_kind）：與 id 合成 (kind, id)。buyer_id 與 store_id 是兩個
// 各自連號的命名空間，同一個數值在兩邊是不同的人，所以身分一律成對比較。
const (
	ActorKindBuyer int16 = 1
	ActorKindStore int16 = 2
)

// cms_post.publish_status 的合法值。碼值不在這裡：一律取自 CodeMeta 生成的
// pkg/codemeta（來源 docs/代碼管理（CodeMeta）/業務狀態碼.md 的 PostPublishStatus），
// 本檔只把它們釘成 int16 —— sqlc 生成的 gen.CmsPost.PublishStatus 是 SMALLINT → int16，
// 有了型別，比較與賦值處就不必逐處轉型，也不會有人順手寫回 4402 字面值。
const (
	PostPublishStatusDraft       int16 = codemeta.PostPublishStatusDraft
	PostPublishStatusPublished   int16 = codemeta.PostPublishStatusPublished
	PostPublishStatusUnpublished int16 = codemeta.PostPublishStatusUnpublished
)
