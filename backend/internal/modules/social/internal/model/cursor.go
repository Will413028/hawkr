package model

import (
	"time"

	"uuid"
)

var (
	descStartTime = time.Date(9999, 12, 31, 23, 59, 59, 999999000, time.UTC)
	descStartID   = uuid.UUID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	ascStartTime  = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	ascStartID    = uuid.UUID{}
)

// DescStart 是由大到小排序的第一頁游標；AscStart 是由小到大排序的第一頁游標。測試用它們驗證「第一頁等於帶極值游標的一頁」。
func DescStart() (time.Time, uuid.UUID) { return descStartTime, descStartID }
func AscStart() (time.Time, uuid.UUID)  { return ascStartTime, ascStartID }

// Key 回 SQL 的游標參數；nil（第一頁）回方向的極值。
func (c *PostCursor) Key() (time.Time, uuid.UUID) {
	if c == nil {
		return DescStart()
	}
	return c.PublishedAt, c.ID
}

func (c *CreatedPostCursor) Key() (time.Time, uuid.UUID) {
	if c == nil {
		return DescStart()
	}
	return c.CreatedAt, c.ID
}

func (c *SanctionCursor) Key() (time.Time, uuid.UUID) {
	if c == nil {
		return DescStart()
	}
	return c.CreatedAt, c.ID
}

func (c *HiddenCursor) Key() (time.Time, uuid.UUID) {
	if c == nil {
		return DescStart()
	}
	return c.HiddenAt, c.HideID
}

func (c *CommentCursor) Key() (time.Time, uuid.UUID) {
	if c == nil {
		return AscStart()
	}
	return c.CreatedAt, c.ID
}
