package app

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// App 是裝置 token 所屬的 App：BUYER 只收買家受眾的推播，SELLER 只收店家受眾的。
type App string

const (
	AppBuyer  App = "BUYER"
	AppSeller App = "SELLER"
)

// MaxTokenLen 是 token 的長度上限（db/schema.sql 的 CHECK：1 到 4096 個字元）。
const MaxTokenLen = 4096

func validToken(token string) bool { return token != "" && len(token) <= MaxTokenLen }

func (a App) valid() bool { return a == AppBuyer || a == AppSeller }

// lockAccount 在呼叫者的 transaction 內共享鎖住存活的帳號；帳號已刪除或不存在是競爭（呼叫者通過認證時帳號還活著），不是使用者的錯。
func (s *Service) lockAccount(ctx context.Context, accountID int64) error {
	deleted, err := s.d.Accounts.ShareLockAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("鎖定帳號：%w", err)
	}
	if deleted {
		return fmt.Errorf("帳號 %d 已刪除", accountID)
	}
	return nil
}

// RegisterDeviceToken 註冊帳號的推播 token：同一個 token 換帳號登入時改綁給呼叫者，並更新最後註冊時間（App 每次啟動、回到前景都呼叫）。
// 同一個 transaction 也把請求協商出的語系記成帳號的 preferred_locale（只在與目前的值不同時寫；多語系 ADR D1「最後一次生效的 UI 語系」）。
// 先寫語系再共享鎖帳號：identity 的帳號列最先，且改值的 UPDATE 先取得排他鎖，不會從共享鎖升級而與同帳號的並行請求互相等待。
func (s *Service) RegisterDeviceToken(ctx context.Context, accountID int64, app App, token string) error {
	l := uitext.LocaleFrom(ctx)
	switch {
	case !validToken(token):
		return apperr.InvalidInputOn([]string{"Token"}, "%s", text.DeviceTokenInvalid(l))
	case !app.valid():
		return apperr.InvalidInputOn([]string{"App"}, "%s", text.DeviceAppInvalid(l))
	}
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.d.Accounts.SetPreferredLocale(ctx, accountID, l); err != nil {
			return fmt.Errorf("記錄帳號語系：%w", err)
		}
		if err := s.lockAccount(ctx, accountID); err != nil {
			return err
		}
		return store.New(s.d.DB.Conn(ctx)).UpsertDeviceToken(ctx, store.UpsertDeviceTokenParams{Token: token, AccountID: accountID, App: string(app), Now: s.d.Now()})
	})
	if err != nil {
		return fmt.Errorf("註冊裝置 token：%w", err)
	}
	return nil
}

// DeleteDeviceToken 刪除帳號自己的一個 token；不存在或屬於別的帳號也成功，不洩漏它是否存在。
func (s *Service) DeleteDeviceToken(ctx context.Context, accountID int64, token string) error {
	if !validToken(token) {
		return apperr.InvalidInputOn([]string{"Token"}, "%s", text.DeviceTokenInvalid(uitext.LocaleFrom(ctx)))
	}
	if err := store.New(s.d.DB.Conn(ctx)).DeleteDeviceToken(ctx, store.DeleteDeviceTokenParams{Token: token, AccountID: accountID}); err != nil {
		return fmt.Errorf("刪除裝置 token：%w", err)
	}
	return nil
}
