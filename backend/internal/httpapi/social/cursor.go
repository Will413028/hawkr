package social

import (
	"time"
	"uuid"
)

// postCursor 是貼文頁的 page token 內容：上一頁最後一篇的 (publish_time, id)。
type postCursor struct {
	PublishedAt time.Time `json:"p"`
	ID          uuid.UUID `json:"i"`
}

func (c postCursor) Valid() bool { return !c.PublishedAt.IsZero() && c.ID != uuid.UUID{} }

// commentCursor 是留言頁的 page token 內容：上一頁最後一則的 (create_time, id)。
type commentCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        uuid.UUID `json:"i"`
}

func (c commentCursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

// createdPostCursor 是店主與後台貼文清單的 page token 內容：(CreatedAt, ID)。
type createdPostCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (c createdPostCursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

// scheduleCursor 是待發布排程清單的 page token 內容：(PublishAt, ID)。
type scheduleCursor struct {
	PublishAt time.Time
	ID        uuid.UUID
}

func (c scheduleCursor) Valid() bool { return !c.PublishAt.IsZero() && c.ID != uuid.UUID{} }
