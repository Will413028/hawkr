package social

import "github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"

// 模組事件只帶領域鍵（內容 ID、互動者 (kind, id)、store_id、以 rune 截斷的預覽）；A7-3 的 relay 在自己的 job 裡經 buyer、seller、
// social 的 API 補 wire 需要的欄位。事件在寫入的 transaction 內發布，沒有訂閱者也照發。

// PostPublished 貼文轉為公開（建立時直接發布、發布、下架後再發布、排程到點發布）；建立草稿不發。Origin 區分 DIRECT 與 SCHEDULED，排程的帶排程 ID 與發起者。
type PostPublished = model.PostPublished

// CommentCreated 頂層留言建立。
type CommentCreated = model.CommentCreated

// CommentReplied 回覆建立。
type CommentReplied = model.CommentReplied

// PostLiked 貼文被按讚（只在真的新增一列時發）。
type PostLiked = model.PostLiked

// PostScheduleCanceled 一個待發布的排程被取消（店主取消、被取代、貼文刪除或下架、授權失效、發起者撤銷）；每個被取消的排程各發一次，帶凍結的原因。
type PostScheduleCanceled = model.PostScheduleCanceled

// PostWriteKind 是代行者對貼文做的寫入種類。
type PostWriteKind = model.PostWriteKind

const (
	PostWriteCreated     = model.PostWriteCreated
	PostWriteUpdated     = model.PostWriteUpdated
	PostWritePublished   = model.PostWritePublished
	PostWriteUnpublished = model.PostWriteUnpublished
	PostWriteScheduled   = model.PostWriteScheduled
)

// PostWrittenByActor 在 ctx 帶著代行者（platform/delegatedactor）時，貼文的建立、修改、發布、下架、排程在寫入的 transaction 內各發一次（含冪等的 no-op），
// 訂閱者同 transaction 寫稽核；店主自己的寫入不發。沒有訂閱者時代行寫入失敗關閉（rollback），不會有沒稽核的代行寫入。
type PostWrittenByActor = model.PostWrittenByActor
