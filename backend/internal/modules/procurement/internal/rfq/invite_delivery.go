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

// StaleQueuedInvites 回傳建立時間早於 before、仍是 QUEUED 的邀請 ID（最舊的先，至多 limit 個）。
func (s Service) StaleQueuedInvites(ctx context.Context, before time.Time, limit int32) ([]uuid.UUID, error) {
	return store.New(s.DB.Conn(ctx)).ListStaleQueuedInvites(ctx, store.ListStaleQueuedInvitesParams{Before: before, MaxRows: limit})
}
