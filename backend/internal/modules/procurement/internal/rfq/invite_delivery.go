package rfq

import (
	"context"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

// MarkInviteSent 把邀請的投遞狀態從 QUEUED 轉為 SENT；邀請已經是終態（重試重跑）或不存在時不做事，回 nil。
func (s Service) MarkInviteSent(ctx context.Context, inviteID uuid.UUID) error {
	_, err := store.New(s.DB.Conn(ctx)).MarkInviteSent(ctx, store.MarkInviteSentParams{ID: inviteID, Now: s.Now()})
	return err
}

// MarkInviteFailed 把邀請的投遞狀態從 QUEUED 轉為 FAILED；邀請已經是終態或不存在時不做事，回 nil。
func (s Service) MarkInviteFailed(ctx context.Context, inviteID uuid.UUID) error {
	_, err := store.New(s.DB.Conn(ctx)).MarkInviteFailed(ctx, store.MarkInviteFailedParams{ID: inviteID, Now: s.Now()})
	return err
}

// StaleInvite 是補掃游標的一格：邀請 ID 與建立時間（排序鍵是 (created_at, id)）。
type StaleInvite struct {
	ID        uuid.UUID
	CreatedAt time.Time
}

// StaleQueuedInvites 回傳建立時間早於 before、仍是 QUEUED、排序鍵 (created_at, id) 大於 after 的邀請（最舊的先，至多 limit 個）。
// after 為零值 StaleInvite 時從頭開始；呼叫端以最後一筆當下一頁的游標。
func (s Service) StaleQueuedInvites(ctx context.Context, before time.Time, after StaleInvite, limit int32) ([]StaleInvite, error) {
	rows, err := store.New(s.DB.Conn(ctx)).ListStaleQueuedInvites(ctx, store.ListStaleQueuedInvitesParams{
		Before: before, AfterCreatedAt: after.CreatedAt, AfterID: after.ID, MaxRows: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]StaleInvite, len(rows))
	for i, r := range rows {
		out[i] = StaleInvite{ID: r.ID, CreatedAt: r.CreatedAt}
	}
	return out, nil
}
