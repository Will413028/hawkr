// Package schedule 是排程發布的資料與取消：post_schedules 的列轉換、取消事件與快取失效。到點發布與排程的建立在 post（它們要用貼文的發布路徑）；
// 取消的入口在這裡，讓貼文的刪除、清除與內容下架不必依賴 post。
//
// 鎖順序：貼文列 → 排程列。每個取消都以呼叫方已鎖貼文列為前提（FromLockedPosts），或自己先依 ID 由小到大鎖貼文列再取消
// （ByScheduler、OfStoreByKind）；別的模組的呼叫要在鎖自己的列之前呼叫，且必須在 transaction 內。
package schedule

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// Canceler 取消待發布的排程：同一個 transaction 內改列、發 PostScheduleCanceled。有待發布排程的貼文不是公開的，取消排程不改變公開狀態，所以不排快取失效；
// 讓貼文公開狀態改變的呼叫方（發布、刪除、內容下架、店家清除）自己排。
type Canceler struct {
	DB     *db.DB
	Events *events.Bus
}

// Of 把 store 的列轉成模組型別。
func Of(r store.SocialPostSchedule) model.Schedule {
	s := model.Schedule{
		ID: r.ID, PostID: r.PostID, StoreID: r.StoreID, PublishAt: r.PublishAt,
		By:    model.ScheduledBy{Kind: model.ScheduleKind(r.ScheduledByKind), Ref: r.ScheduledByRef},
		State: model.ScheduleState(r.State), CreatedAt: r.CreatedAt, ResolvedAt: r.ResolvedAt,
	}
	if r.CancelReason != nil {
		s.CancelReason = model.CancelReason(*r.CancelReason)
	}
	return s
}

// After 是排程已被寫成 CANCELED 之後的副作用：每筆發事件。
func (c Canceler) After(ctx context.Context, rows []store.SocialPostSchedule, now time.Time) error {
	for _, r := range rows {
		reason := model.CancelReason(*r.CancelReason)
		if err := events.Publish(ctx, c.Events, model.PostScheduleCanceled{
			EventID: uuid.NewV7(), OccurredAt: now, ScheduleID: r.ID, PostID: r.PostID, Reason: reason,
		}); err != nil {
			return err
		}
	}
	return nil
}

func reasonArg(r model.CancelReason) *string {
	s := string(r)
	return &s
}

func ids(rows []store.SocialPostSchedule) []uuid.UUID {
	out := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

// FromLockedPosts 取消這些貼文的待發布排程。呼叫方已鎖這些貼文列。回傳被取消的排程 ID；沒有待發布的排程是 no-op。
func (c Canceler) FromLockedPosts(ctx context.Context, postIDs []uuid.UUID, reason model.CancelReason, now time.Time) ([]uuid.UUID, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}
	q := store.New(c.DB.Conn(ctx))
	rows, err := q.CancelPendingSchedulesOfPosts(ctx, store.CancelPendingSchedulesOfPostsParams{Reason: reasonArg(reason), Now: &now, PostIds: postIDs})
	if err != nil {
		return nil, fmt.Errorf("social：取消貼文的排程：%w", err)
	}
	if err := c.After(ctx, rows, now); err != nil {
		return nil, err
	}
	return ids(rows), nil
}

// ByScheduler 取消某個發起者（種類加一批 ref）所有待發布的排程。空的 refs 是 no-op。先以不鎖的讀取找出貼文，依 ID 由小到大鎖貼文列，
// 再取消仍是 PENDING 的；不鎖讀取與加鎖之間才新排的不在這一輪（由到點的授權重驗擋下）。必須在 transaction 內。
func (c Canceler) ByScheduler(ctx context.Context, kind model.ScheduleKind, refs []string, reason model.CancelReason, now time.Time) ([]uuid.UUID, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	var out []uuid.UUID
	err := c.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(c.DB.Conn(ctx))
		postIDs, err := q.PendingPostIDsOfScheduler(ctx, store.PendingPostIDsOfSchedulerParams{Kind: string(kind), Refs: refs})
		if err != nil {
			return fmt.Errorf("social：找發起者的待發布排程：%w", err)
		}
		if len(postIDs) == 0 {
			return nil
		}
		if _, err := q.LockPostsByID(ctx, postIDs); err != nil {
			return fmt.Errorf("social：鎖定排程的貼文：%w", err)
		}
		rows, err := q.CancelPendingSchedulesOfScheduler(ctx, store.CancelPendingSchedulesOfSchedulerParams{
			Reason: reasonArg(reason), Now: &now, PostIds: postIDs, Kind: string(kind), Refs: refs,
		})
		if err != nil {
			return fmt.Errorf("social：取消發起者的排程：%w", err)
		}
		if err := c.After(ctx, rows, now); err != nil {
			return err
		}
		out = ids(rows)
		return nil
	})
	return out, err
}

// OfStoreByKind 取消某家店某種發起者所有待發布的排程（撤回店家授權時取消該店所有代行的排程）。
func (c Canceler) OfStoreByKind(ctx context.Context, storeID int64, kind model.ScheduleKind, reason model.CancelReason, now time.Time) ([]uuid.UUID, error) {
	var out []uuid.UUID
	err := c.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(c.DB.Conn(ctx))
		postIDs, err := q.PendingPostIDsOfStoreByKind(ctx, store.PendingPostIDsOfStoreByKindParams{StoreID: storeID, Kind: string(kind)})
		if err != nil {
			return fmt.Errorf("social：找店家的待發布排程：%w", err)
		}
		if len(postIDs) == 0 {
			return nil
		}
		if _, err := q.LockPostsByID(ctx, postIDs); err != nil {
			return fmt.Errorf("social：鎖定排程的貼文：%w", err)
		}
		rows, err := q.CancelPendingSchedulesOfStoreByKind(ctx, store.CancelPendingSchedulesOfStoreByKindParams{
			Reason: reasonArg(reason), Now: &now, PostIds: postIDs, StoreID: storeID, Kind: string(kind),
		})
		if err != nil {
			return fmt.Errorf("social：取消店家的排程：%w", err)
		}
		if err := c.After(ctx, rows, now); err != nil {
			return err
		}
		out = ids(rows)
		return nil
	})
	return out, err
}
