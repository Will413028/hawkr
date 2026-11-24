package notification

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/deliver"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/reach"
)

// ResendInput 是重送永久失敗投遞的條件（Actor.Action 是後台 RPC 的 procedure）。
type ResendInput = reach.ResendInput

// ResendResult 是重送的結果：重排的筆數與是否還有因批次上限沒處理的投遞。
type ResendResult = reach.ResendResult

// UnreachedInput 是未觸達查詢的條件。
type UnreachedInput = reach.UnreachedInput

// UnreachedCursor 是未觸達列表的排序鍵。
type UnreachedCursor = reach.Cursor

// UnreachedPage 是一頁未觸達的通知，帶窗內的總筆數。
type UnreachedPage = reach.Page

// UnreachedNotification 是一個收件人沒有觸達的一則通知（只有參考 ID）。
type UnreachedNotification = reach.Notification

// UnreachedChannel 是一個渠道的失敗結果。
type UnreachedChannel = reach.Channel

// 重送與未觸達查詢的上限與預設。
const (
	ResendDefaultMaxCount = reach.DefaultMaxCount
	ResendMaxMaxCount     = reach.MaxMaxCount
	ResendMaxWindow       = reach.MaxResendWindow
	UnreachedDefaultDays  = reach.DefaultDays
	UnreachedMaxDays      = reach.MaxDays
)

// ResendableReasons 回傳可重送的失敗原因。
func ResendableReasons() []string { return reach.Resendable() }

// DeliveryFailureReasons 回傳投遞紀錄全部的失敗原因（deliveries.failure_reason 的值域）。
func DeliveryFailureReasons() []string { return deliver.Reasons() }

// ResendDeliveries 把符合條件、永久失敗且原因可重送的投遞改回待送並排入渠道 job，寫稽核紀錄；不重送、冪等與批次的規則見 internal/reach。
func (m *Module) ResendDeliveries(ctx context.Context, in ResendInput) (ResendResult, error) {
	return m.reach.Resend(ctx, in)
}

// ListUnreachedNotifications 列出近期交易必要型別的通知中沒有觸達（推播與 email 都沒送出）的收件人，唯讀。
func (m *Module) ListUnreachedNotifications(ctx context.Context, in UnreachedInput) (UnreachedPage, error) {
	return m.reach.Unreached(ctx, in)
}
