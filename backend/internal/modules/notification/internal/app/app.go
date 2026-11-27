// Package app 是登入帳號讀寫自己通知的用例（NotificationAppService 的 unary 方法）：收件匣列表與未讀數、已讀、刪除、偏好、裝置 token。
// 收件人是 (帳號, 受眾, 店)；已讀與刪除只看帳號擁有（通知 ID 不帶受眾）。顯示哪些列、偏好列出哪些組合都由型別註冊表推導。
package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Accounts 是對 identity 的需求：以共享鎖鎖住存活的帳號列（commit 前不會被刪），並回報帳號是否已刪除。identity.Module 實作它。
type Accounts interface {
	ShareLockAccount(ctx context.Context, id int64) (deleted bool, err error)
	// SetPreferredLocale 在呼叫者的 transaction 內把帳號的 preferred_locale 改成 locale（只在不同時寫）。
	SetPreferredLocale(ctx context.Context, id int64, locale uitext.Locale) (bool, error)
}

// Registry 是型別註冊表；正式組裝用 registry.Registry，測試可以換成帶測試型別的版本。
type Registry interface {
	All() []registry.Entry
	Lookup(name string) (registry.Entry, bool)
}

// Signals 是已讀與刪除通知串流訂閱者的出口（根套件以 stream.NotifyInbox 實作）。
// 在改動收件匣的同一個 transaction 內呼叫（pg_notify 在 commit 後才送達，rollback 不送）；nil 表示不通知（沒有串流的組裝）。
type Signals interface {
	InboxChanged(ctx context.Context, accountID int64, audience model.Audience, storeID int64, hint model.StateHint, itemID uuid.UUID) error
}

// Deps 是 Service 的依賴。
type Deps struct {
	DB       *db.DB
	Accounts Accounts
	Registry Registry
	Signals  Signals
	// StoreCodes 讀取店家的 store_code（深連結要用）；沒有內容型別的組裝可以不給。
	StoreCodes StoreCodes
	Now        func() time.Time
	Logger     *slog.Logger
}

// StoreCodes 讀取多家店的 store_code（store_id 到 store_code）；沒有公開頁的店不在結果裡。
type StoreCodes interface {
	StoreCodes(ctx context.Context, storeIDs []int64) (map[int64]int64, error)
}

// Service 是登入帳號的通知用例。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}
}

// Recipient 是通知的收件身分：帳號與受眾；STORE 受眾另帶店家 ID，BUYER 的 StoreID 一定是 0。
type Recipient struct {
	AccountID int64
	Audience  model.Audience
	StoreID   int64
}

func (r Recipient) validate() error {
	switch {
	case r.AccountID <= 0:
		return fmt.Errorf("收件人的帳號 %d 不合法", r.AccountID)
	case r.Audience == model.AudienceBuyer && r.StoreID == 0:
		return nil
	case r.Audience == model.AudienceStore && r.StoreID > 0:
		return nil
	}
	return fmt.Errorf("收件人的受眾 %q 與店家 %d 不合法", r.Audience, r.StoreID)
}

func (r Recipient) storeID() *int64 {
	if r.StoreID > 0 {
		return &r.StoreID
	}
	return nil
}

func notFound(ctx context.Context) error {
	return apperr.NotFound("%s", text.InboxItemNotFound(uitext.LocaleFrom(ctx)))
}

// ValidateAudience 檢查請求的受眾：沒給或不認得回 invalid_argument（指向 Audience 欄位）。
func (s *Service) ValidateAudience(ctx context.Context, a model.Audience) error {
	if a != model.AudienceBuyer && a != model.AudienceStore {
		return apperr.InvalidInputOn([]string{"Audience"}, "%s", text.AudienceInvalid(uitext.LocaleFrom(ctx)))
	}
	return nil
}

func (s *Service) inboxChanged(ctx context.Context, accountID int64, audience string, storeID *int64) error {
	if s.d.Signals == nil {
		return nil
	}
	var sid int64
	if storeID != nil {
		sid = *storeID
	}
	return s.d.Signals.InboxChanged(ctx, accountID, model.Audience(audience), sid, model.StateHintNone, uuid.Nil())
}
