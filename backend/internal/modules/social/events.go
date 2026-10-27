package social

import "github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"

// 模組事件只帶領域鍵（內容 ID、互動者 (kind, id)、store_id、以 rune 截斷的預覽）；A7-3 的 relay 在自己的 job 裡經 buyer、seller、
// social 的 API 補 wire 需要的欄位。事件在寫入的 transaction 內發布，沒有訂閱者也照發。

// PostPublished 貼文轉為公開（建立時直接發布、發布、下架後再發布）；建立草稿不發。
type PostPublished = model.PostPublished

// CommentCreated 頂層留言建立。
type CommentCreated = model.CommentCreated

// CommentReplied 回覆建立。
type CommentReplied = model.CommentReplied

// PostLiked 貼文被按讚（只在真的新增一列時發）。
type PostLiked = model.PostLiked
