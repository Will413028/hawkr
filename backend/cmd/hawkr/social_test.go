package main

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

type fakeSocialSellers struct{}

func (fakeSocialSellers) Store(context.Context, int64) (seller.Store, error) {
	return seller.Store{}, nil
}

func (fakeSocialSellers) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) {
	return false, nil
}

// 正式組裝路徑把 seller 同時接成 social 的店家讀取與店家封鎖檢查：social.New 缺任何一個必要依賴就 panic，
// 所以拿掉 StoreBlocks 的接線，這個測試失敗。
func TestNewSocialWiresStoreBlocks(t *testing.T) {
	if newSocial(nil, events.NewBus(), nil, nil, fakeSocialSellers{}, nil, socialProducts{}, nil) == nil {
		t.Fatal("newSocial 要回傳模組")
	}
}
