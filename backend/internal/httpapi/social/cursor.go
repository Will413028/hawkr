package social

import (
	"time"
	"uuid"
)

type postCursor struct {
	PublishedAt time.Time `json:"p"`
	ID          uuid.UUID `json:"i"`
}

func (c postCursor) Valid() bool { return !c.PublishedAt.IsZero() && c.ID != uuid.UUID{} }

type commentCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        uuid.UUID `json:"i"`
}

func (c commentCursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

type createdPostCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (c createdPostCursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

type scheduleCursor struct {
	PublishAt time.Time
	ID        uuid.UUID
}

func (c scheduleCursor) Valid() bool { return !c.PublishAt.IsZero() && c.ID != uuid.UUID{} }
