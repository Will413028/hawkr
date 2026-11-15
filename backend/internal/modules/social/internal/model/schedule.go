package model

import (
	"context"
	"errors"
	"time"
	"uuid"
)

// 排程發布的時點區間：publish_at 距現在要在 [MinScheduleLead, MaxScheduleLead]。兩端都擋：太近的排程與立即發布沒有可觀察的差別卻要多付一整條路徑，
// 太遠的排程會佔著待發布列很久。
const (
	MinScheduleLead = 5 * time.Minute
	MaxScheduleLead = 90 * 24 * time.Hour
)

// MaxScheduledByRefRunes 是 scheduled_by_ref 的上限（schema 的 post_schedules_by_ref_check）。
const MaxScheduledByRefRunes = 200

// ScheduleKind 是排程發起者的種類，決定到點時用哪一個授權實作。
type ScheduleKind string

const (
	// ScheduleByOwner 是店主自己排的：到點只做 social 自己的檢查。
	ScheduleByOwner ScheduleKind = "OWNER"
	// ScheduleByDelegation 是管理員代行排的：到點由注入的授權實作重驗當下的代行授權。
	ScheduleByDelegation ScheduleKind = "DELEGATION"
)

// ScheduleKindValues 回傳全部發起者種類；DB 的 CHECK 與它一致。
func ScheduleKindValues() []ScheduleKind {
	return []ScheduleKind{ScheduleByOwner, ScheduleByDelegation}
}

// ScheduleState 是排程的狀態：PENDING 之後只會轉成 PUBLISHED 或 CANCELED。
type ScheduleState string

const (
	SchedulePending   ScheduleState = "PENDING"
	SchedulePublished ScheduleState = "PUBLISHED"
	ScheduleCanceled  ScheduleState = "CANCELED"
)

// ScheduleStateValues 回傳全部排程狀態。
func ScheduleStateValues() []ScheduleState {
	return []ScheduleState{SchedulePending, SchedulePublished, ScheduleCanceled}
}

// CancelReason 是排程被取消的原因，凍結在排程列上。
type CancelReason string

const (
	CancelAuthorityLost CancelReason = "AUTHORITY_LOST"
	CancelDeleted       CancelReason = "DELETED"
	CancelTakenDown     CancelReason = "TAKEN_DOWN"
	CancelReplaced      CancelReason = "REPLACED"
	CancelOwnerCanceled CancelReason = "OWNER_CANCELED"
)

// CancelReasonValues 回傳全部取消原因；DB 的 CHECK 與它一致。
func CancelReasonValues() []CancelReason {
	return []CancelReason{CancelAuthorityLost, CancelDeleted, CancelTakenDown, CancelReplaced, CancelOwnerCanceled}
}

// ScheduledBy 是排程的發起者：Ref 是發起者擁有格式的不透明字串（代行放 session ID）。
type ScheduledBy struct {
	Kind ScheduleKind
	Ref  string
}

// Schedule 是貼文的一次排程。
type Schedule struct {
	ID           uuid.UUID
	PostID       uuid.UUID
	StoreID      int64
	PublishAt    time.Time
	By           ScheduledBy
	State        ScheduleState
	CancelReason CancelReason
	CreatedAt    time.Time
	ResolvedAt   *time.Time
}

// SchedulePostInput 是排程發布：貼文要是 DRAFT 或 UNPUBLISHED，Version 是讀取時的版本。
type SchedulePostInput struct {
	PostID    uuid.UUID
	Version   int64
	PublishAt time.Time
	By        ScheduledBy
}

// ScheduleCursor 是店家待發布排程清單的排序鍵：(PublishAt, ID) 由小到大。
type ScheduleCursor struct {
	PublishAt time.Time
	ID        uuid.UUID
}

// Key 回 SQL 的游標參數；nil（第一頁）回由小到大的極值。
func (c *ScheduleCursor) Key() (time.Time, uuid.UUID) {
	if c == nil {
		return AscStart()
	}
	return c.PublishAt, c.ID
}

// SchedulePage 是店家的一頁待發布排程。
type SchedulePage struct {
	Schedules []Schedule
	Next      *ScheduleCursor
}

// ScheduledPublish 是到點發布前交給授權實作的事實。
type ScheduledPublish struct {
	ScheduleID uuid.UUID
	PostID     uuid.UUID
	StoreID    int64
	By         ScheduledBy
	PublishAt  time.Time
}

// ErrScheduleDenied 是授權實作回報「現在不能發布」：排程轉 CANCELED（AUTHORITY_LOST）、不重試。其他錯誤是暫時的失敗，job 重試。
var ErrScheduleDenied = errors.New("social：排程發布的授權已失效")

// ScheduleAuthorizer 在到點、貼文列已被鎖住時重驗發起者此刻的授權。回 nil 表示允許，ErrScheduleDenied（可包裝）表示拒絕，其他錯誤讓 job 重試。
// 它在 social 的 transaction 內被呼叫，實作不得鎖比 social 更上層的列（單體規則 4）：用不鎖的讀取。
type ScheduleAuthorizer interface {
	AuthorizeScheduledPublish(ctx context.Context, p ScheduledPublish) error
}

// PostOrigin 是貼文公開的來源。
type PostOrigin string

const (
	// OriginDirect 是店家（或代行）當下呼叫發布，包含建立時直接發布。
	OriginDirect PostOrigin = "DIRECT"
	// OriginScheduled 是排程到點發布。
	OriginScheduled PostOrigin = "SCHEDULED"
)
