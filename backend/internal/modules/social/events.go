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

// CommentLiked 留言被按讚（只在真的新增一列時發，取消讚與重複按讚不發）。
type CommentLiked = model.CommentLiked

// PostScheduleCanceled 一個待發布的排程被取消（店主取消、被取代、貼文刪除或下架、授權失效、發起者撤銷）；每個被取消的排程各發一次，帶凍結的原因。
type PostScheduleCanceled = model.PostScheduleCanceled

// PostCreated 貼文建立；每個成功的建立呼叫發一次（店主與代行都發），Actor 是明確傳入的代行者（沒有則 nil）。訂閱者與發布在同一個 transaction。
type PostCreated = model.PostCreated

// PostUpdated 貼文修改，發布規則同 PostCreated。
type PostUpdated = model.PostUpdated

// PostUnpublished 貼文下架，發布規則同 PostCreated。
type PostUnpublished = model.PostUnpublished

// PostScheduled 貼文排定發布，發布規則同 PostCreated。
type PostScheduled = model.PostScheduled

// PostPublishRepeated 對已發布的貼文再發布（冪等的 no-op，不發 PostPublished）；請求本身要能被稽核。
type PostPublishRepeated = model.PostPublishRepeated

// PostUnpublishRepeated 對已下架的貼文再下架（冪等的 no-op）。
type PostUnpublishRepeated = model.PostUnpublishRepeated

// PostWritten 是上面這幾個事件共用的欄位。
type PostWritten = model.PostWritten
