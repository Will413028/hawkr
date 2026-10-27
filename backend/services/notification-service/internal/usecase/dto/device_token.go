package dto

// RegisterDeviceTokenRequest is the input for registering a device token.
type RegisterDeviceTokenRequest struct {
	UserType string
	Token    string
	Platform string
	UserID   int64
}

// RemoveDeviceTokenRequest is the input for removing a device token.
type RemoveDeviceTokenRequest struct {
	UserType string
	Token    string
	UserID   int64
}

// SendPushRequest is the input for sending a push notification.
//
// When Silent is true, the use case sends a data-only FCM message (no notification
// block). The receiving App handles it in background without showing any UI banner
// — typical use case is triggering list / detail page refresh after the user's own
// action. Silent pushes still respect notification_preferences and idempotency.
type SendPushRequest struct {
	Data        map[string]string
	UserType    string
	EventType   string
	ReferenceID string
	Title       string
	Body        string
	ImageURL    string // FCM Notification.ImageUrl + data.imageUrl 共用
	Category    string // 由 worker 從 template.Category 帶入；空字串 = fail-open（不檢查 preference）
	UserID      int64
	Silent      bool
}
