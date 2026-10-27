package domain

import (
	"context"
	"errors"
)

const (
	StatusSent    = "sent"
	StatusFailed  = "failed"
	StatusPartial = "partial"
	StatusSkipped = "skipped"
)

type PushNotificationRecord struct {
	Data        map[string]string
	UserType    string
	EventType   string
	ReferenceID string
	Title       string
	Body        string
	Status      string
	ErrorLog    string
	ID          int64
	UserID      int64
}

type InboxItem struct {
	Data        map[string]string
	UserType    string
	EventType   string
	ReferenceID string
	Title       string
	Body        string
	ReadAt      *string
	CreatedAt   string
	ID          int64
	UserID      int64
	IsRead      bool
}

type PushNotificationRepository interface {
	Create(ctx context.Context, record *PushNotificationRecord) error
	// ExistsProcessed reports whether this logical notification has already been
	// processed for the recipient — keyed on identity (event_type, reference_id,
	// user), independent of delivery status. A redelivery of the same event must
	// not re-push even if the prior attempt was partial/skipped/failed.
	ExistsProcessed(ctx context.Context, eventType, referenceID string, userID int64, userType string) (bool, error)
}

type NotificationInboxRepository interface {
	// Create inserts an inbox item. Returns rows affected (0 when the insert was
	// deduped by ON CONFLICT DO NOTHING) so callers can skip realtime re-publish.
	Create(ctx context.Context, item *InboxItem) (int64, error)
	// userID is always the current domain surrogate: buyer_id for buyers and
	// store_id for sellers. Retired stints are handled by the existing lifecycle
	// cleanup instead of being made visible through an account-level fallback.
	ListByUser(ctx context.Context, userID int64, userType string, cursorID int64, limit int32, category string) ([]*InboxItem, error)
	CountByUser(ctx context.Context, userID int64, userType string) (int64, error)
	CountUnread(ctx context.Context, userID int64, userType string) (int64, error)
	MarkAsRead(ctx context.Context, id, userID int64, userType string) (int64, error)
	MarkAllAsRead(ctx context.Context, userID int64, userType string) (int64, error)
	SoftDeleteByIDs(ctx context.Context, ids []int64, userID int64, userType string) (int64, error)
	RedactChatBySender(ctx context.Context, senderIDs []int64) (int64, error)
	// SoftDeleteByHistoricalBuyerIDs soft-deletes every live inbox row addressed
	// to one of the given retired buyer stints. Only buyer rows are touched — the
	// same numeric id can be a still-active store_id on a dual-role account.
	SoftDeleteByHistoricalBuyerIDs(ctx context.Context, buyerIDs []int64) (int64, error)
	// SoftDeleteByHistoricalStoreIDs soft-deletes every live inbox row addressed
	// to one of the given retired seller incarnations. Only seller rows are
	// touched — the same numeric id can be a still-active buyer_id on a
	// dual-role account.
	SoftDeleteByHistoricalStoreIDs(ctx context.Context, storeIDs []int64) (int64, error)
}

// ErrPushDisabled is returned by a PushNotifier when push delivery is not
// configured. It is a configuration state, not a delivery failure.
var ErrPushDisabled = errors.New("push disabled: FIREBASE_CREDENTIALS_JSON is not set")

type PushNotifier interface {
	Send(ctx context.Context, tokens []string, title, body, imageURL string, data map[string]string) (map[string]error, error)
	SendDataOnly(ctx context.Context, tokens []string, data map[string]string) (map[string]error, error)
	SendToTopic(ctx context.Context, topic, title, body, imageURL string, data map[string]string) (string, error)
}

// TokenDeactivationPolicy decides whether a per-token FCM send error means the
// token is permanently undeliverable for us and should be deactivated. Injected
// into SendPushUseCase so the FCM-specific classification lives in infrastructure
// (and is swappable in tests), keeping the usecase free of infrastructure deps.
type TokenDeactivationPolicy func(error) bool
