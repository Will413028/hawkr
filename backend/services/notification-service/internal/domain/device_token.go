package domain

import "context"

const (
	UserTypeBuyer  = "buyer"
	UserTypeSeller = "seller"

	// App 只發 iOS。
	PlatformIOS = "ios"
)

type DeviceToken struct {
	UserType string
	Token    string
	Platform string
	ID       int64
	UserID   int64
	IsActive bool
}

type DeviceTokenRepository interface {
	Upsert(ctx context.Context, userID int64, userType, token, platform string) error
	Deactivate(ctx context.Context, userID int64, userType, token string) error
	DeactivateByToken(ctx context.Context, token string) error
	// DeactivateByUser deactivates every device_tokens row registered under
	// (userID, userType), regardless of token value — unlike Deactivate above,
	// which targets exactly one known token. Used when a seller incarnation
	// retires (issue #2707 follow-up, worker/seller_deleted.go) so a device
	// that re-registered under a NEW store_id no longer keeps receiving pushes
	// through the OLD store_id's row. Returns the number of rows deactivated.
	DeactivateByUser(ctx context.Context, userID int64, userType string) (int64, error)
	GetActiveByUser(ctx context.Context, userID int64, userType string) ([]*DeviceToken, error)
	GetActiveByUserType(ctx context.Context, userType string, limit, offset int64) ([]*DeviceToken, error)
}
