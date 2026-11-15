package model

import (
	"time"
	"uuid"
)

// 模組事件只帶領域鍵（內容 ID、互動者 (kind, id)、store_id、以 rune 截斷的預覽），不帶名稱、頭像、account_id 或留言快照：A7-3 的
// relay 在自己的 job 裡經 buyer、seller、social 的 API 補 wire 需要的欄位。事件在寫入的 transaction 內發布，沒有訂閱者也照發；
// 自己通知自己的略過由消費端判斷。

// PostPublished 貼文轉為公開（建立時直接發布、發布、再發布）；建立草稿不發。已經是 PUBLISHED 時再發布不再發。
//
// Origin 是 DIRECT（呼叫發布）或 SCHEDULED（排程到點）；ScheduleID 與 ScheduledBy 只在 SCHEDULED 有值（DIRECT 是零值），稽核以排程 ID 當冪等鍵。
type PostPublished struct {
	EventID     uuid.UUID
	OccurredAt  time.Time
	PostID      uuid.UUID
	StoreID     int64
	Preview     string
	Origin      PostOrigin
	ScheduleID  uuid.UUID
	ScheduledBy ScheduledBy
}

// PostScheduleCanceled 一個待發布的排程被取消（店主取消、被取代、貼文刪除或下架、授權失效、發起者撤銷）；每個被取消的排程各發一次。
// 排程到點發布不發事件（PostPublished 的 Origin 是 SCHEDULED），建立排程也不發。
type PostScheduleCanceled struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	ScheduleID uuid.UUID
	PostID     uuid.UUID
	Reason     CancelReason
}

// CommentCreated 頂層留言建立。StoreID 是貼文所屬的店家。
type CommentCreated struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	CommentID  uuid.UUID
	PostID     uuid.UUID
	StoreID    int64
	Actor      Actor
	Preview    string
}

// CommentReplied 回覆建立。ParentID 是頂層留言，ReplyToID 是被回覆的那一則，ReplyToActor 是它的作者。
type CommentReplied struct {
	EventID      uuid.UUID
	OccurredAt   time.Time
	CommentID    uuid.UUID
	PostID       uuid.UUID
	StoreID      int64
	ParentID     uuid.UUID
	ReplyToID    uuid.UUID
	Actor        Actor
	ReplyToActor Actor
	Preview      string
}

// PostLiked 貼文被按讚（只在真的新增一列時發，並發雙擊與重複按讚不發）。
type PostLiked struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	PostID     uuid.UUID
	StoreID    int64
	BuyerID    int64
}
