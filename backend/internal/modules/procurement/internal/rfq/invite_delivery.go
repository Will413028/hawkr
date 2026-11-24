package rfq

import (
	"context"

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
