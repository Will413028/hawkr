package repository

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
)

// PublicationOutboxEventInviteCreated 是 PLATFORM lane 的 event_type，值域同時由
// chk_prpo_event_type 表達。新增值時兩邊一起改。
const PublicationOutboxEventInviteCreated = "RFQ_PUBLICATION_INVITE_CREATED"

// PublicationOutboxEventInviteExternalCreated 是 EXTERNAL lane 的 event_type。
const PublicationOutboxEventInviteExternalCreated = "RFQ_PUBLICATION_INVITE_EXTERNAL_CREATED"

// InsertPublicationOutbox 為一張 invite 排一個投遞任務。
func (r *Repo) InsertPublicationOutbox(ctx context.Context, inviteID int64, eventType string, payload []byte, now time.Time) error {
	return r.q(ctx).InsertProcurementRfqPublicationOutbox(ctx, gen.InsertProcurementRfqPublicationOutboxParams{
		ID:            r.newID(),
		InviteID:      inviteID,
		EventType:     eventType,
		Payload:       payload,
		NextAttemptAt: timestamptzParam(now),
	})
}

// MarkPublicationInviteSent 把 invite 從 QUEUED 轉 SENT。
//
// ⛔ SENT 是「已交給投遞管道」，不是已送達、更不是已看到（doc 64 §4.2）。
// CAS 影響 0 列不是錯誤：一列有 outbox 列的 invite 在 ProcessFunc 執行當下的可達狀態
// 只有 QUEUED（第一次）與 SENT（MarkDone 失敗後的重試）兩種，所以 0 列必然是後者。
func (r *Repo) MarkPublicationInviteSent(ctx context.Context, inviteID int64, now time.Time) error {
	return r.q(ctx).MarkProcurementRfqPublicationInviteSent(ctx, gen.MarkProcurementRfqPublicationInviteSentParams{
		ID:              inviteID,
		DeliveryStateAt: timestamptzParam(now),
	})
}
