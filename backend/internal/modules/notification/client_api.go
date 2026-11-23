package notification

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/app"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Accounts 是模組對 identity 的需求（*identity.Module 滿足它）：以共享鎖鎖住存活的帳號列，並回報帳號是否已刪除。
type Accounts = app.Accounts

// 登入帳號讀寫自己通知的型別（NotificationAppService 的 unary 方法用）。
type (
	// Audience 是通知的收件身分面：BUYER 或 STORE。
	Audience = model.Audience
	// Category 是偏好分類（D4、D18）。
	Category = model.PrefCategory
	// Channel 是通知的渠道：INBOX、PUSH、EMAIL。
	Channel = model.Channel
	// Presentation 是收件匣的呈現分類（D6）。
	Presentation = model.Presentation
	// Recipient 是收件身分：帳號與受眾；STORE 另帶店家 ID。
	Recipient = app.Recipient
	// InboxItem 是收件匣的一則通知，文字已依語系組好。
	InboxItem = app.Item
	// InboxPage 是收件匣的一頁，帶同一個篩選下的全部筆數。
	InboxPage = app.Page
	// InboxCursor 是收件匣列表的排序鍵。
	InboxCursor = app.Cursor
	// Preference 是一個 (分類, 渠道) 的偏好。
	Preference = app.Preference
	// DeviceApp 是裝置 token 所屬的 App。
	DeviceApp = app.App
)

const (
	AudienceBuyer = model.AudienceBuyer
	AudienceStore = model.AudienceStore

	DeviceAppBuyer  = app.AppBuyer
	DeviceAppSeller = app.AppSeller

	// MaxBatchDelete 是一次刪除通知的上限；MaxTokenLen 是裝置 token 的長度上限。
	MaxBatchDelete = app.MaxBatchDelete
	MaxTokenLen    = app.MaxTokenLen
)

// Categories、Channels、Presentations 回傳全部的代碼值（給 handler 檢查與契約 enum 一致）。
func Categories() []Category        { return model.Categories() }
func Channels() []Channel           { return model.Channels() }
func Presentations() []Presentation { return model.Presentations() }

// ListInbox 列出收件人的收件匣，新到舊；category 不為 nil 時只列該分類（不屬於受眾回 invalid_argument）。文字依 l 組。
func (m *Module) ListInbox(ctx context.Context, r Recipient, category *Category, l uitext.Locale, after *InboxCursor, size int32) (InboxPage, error) {
	return m.app.ListInbox(ctx, r, category, l, after, size)
}

// UnreadCount 回傳收件人的未讀數（不分分類）。
func (m *Module) UnreadCount(ctx context.Context, r Recipient) (int64, error) {
	return m.app.UnreadCount(ctx, r)
}

// MarkInboxItemRead 把帳號的一則通知標成已讀；不存在與不是帳號的回同一個 not_found。
func (m *Module) MarkInboxItemRead(ctx context.Context, accountID int64, id uuid.UUID) error {
	return m.app.MarkRead(ctx, accountID, id)
}

// MarkAllInboxItemsRead 把收件人的全部未讀通知標成已讀，回傳這次改成已讀的筆數。
func (m *Module) MarkAllInboxItemsRead(ctx context.Context, r Recipient) (int64, error) {
	return m.app.MarkAllRead(ctx, r)
}

// BatchDeleteInboxItems 刪除帳號的通知，全有或全無：空、重複或超過 MaxBatchDelete 個回 invalid_argument，任何一個不存在或不是帳號的整批回 not_found。
func (m *Module) BatchDeleteInboxItems(ctx context.Context, accountID int64, ids []uuid.UUID) error {
	return m.app.BatchDelete(ctx, accountID, ids)
}

// NotificationPreferences 列出收件人的偏好：只有註冊表實際會送達的 (分類, 渠道)。
func (m *Module) NotificationPreferences(ctx context.Context, r Recipient) ([]Preference, error) {
	return m.app.Preferences(ctx, r)
}

// UpdateNotificationPreference 設定一個 (分類, 渠道) 的開關並回傳更新後的那一項；不在送達組合內或要關閉不可關的組合回 invalid_argument。
func (m *Module) UpdateNotificationPreference(ctx context.Context, r Recipient, c Category, ch Channel, enabled bool) (Preference, error) {
	return m.app.UpdatePreference(ctx, r, c, ch, enabled)
}

// RegisterDeviceToken 註冊帳號的推播 token（同一個 token 換帳號登入時改綁，更新最後註冊時間）。
func (m *Module) RegisterDeviceToken(ctx context.Context, accountID int64, deviceApp DeviceApp, token string) error {
	return m.app.RegisterDeviceToken(ctx, accountID, deviceApp, token)
}

// DeleteDeviceToken 刪除帳號自己的一個 token；不存在或屬於別的帳號也成功。
func (m *Module) DeleteDeviceToken(ctx context.Context, accountID int64, token string) error {
	return m.app.DeleteDeviceToken(ctx, accountID, token)
}

// ValidateAudience 檢查請求的受眾：沒給或不認得回 invalid_argument。
func (m *Module) ValidateAudience(ctx context.Context, a Audience) error {
	return m.app.ValidateAudience(ctx, a)
}
