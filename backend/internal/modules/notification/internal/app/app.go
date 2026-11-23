// Package app 是登入帳號讀寫自己通知的用例（NotificationAppService 的 unary 方法）：收件匣列表與未讀數、已讀、刪除、偏好、裝置 token。
// 收件人是 (帳號, 受眾, 店)；已讀與刪除只看帳號擁有（通知 ID 不帶受眾）。顯示哪些列、偏好列出哪些組合都由型別註冊表推導。
package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
}

// Registry 是型別註冊表；正式組裝用 registry.Registry，測試可以換成帶測試型別的版本。
type Registry interface {
	All() []registry.Entry
	Lookup(name string) (registry.Entry, bool)
}

// Deps 是 Service 的依賴。
type Deps struct {
	DB       *db.DB
	Accounts Accounts
	Registry Registry
	Now      func() time.Time
	Logger   *slog.Logger
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

// validate 檢查收件人的形狀：呼叫端（handler）依登入帳號與受眾組出來，不合法是呼叫端的程式錯誤，不是使用者的錯。
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

// storeID 是查詢用的店家 ID：BUYER 是 nil。
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
