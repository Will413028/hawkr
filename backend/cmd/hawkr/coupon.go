package main

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// couponStores 是 coupon 的 Stores port 的正式 adapter（計畫 D8 修訂）：資格判定當下問 seller 店家是不是停權，不複製狀態。
// 讀不到店家（含店家不存在）原樣回錯誤，領券與鎖券整個失敗，不當成沒有停權。
type couponStores struct{ sellers storeReader }

// storeReader 是 couponStores 需要的 seller 讀取（*seller.Module）。
type storeReader interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

func (s couponStores) StoreSuspended(ctx context.Context, storeID int64) (bool, error) {
	st, err := s.sellers.Store(ctx, storeID)
	if err != nil {
		return false, err
	}
	return st.Suspended(), nil
}

// errMembersNotProjected 表示 MEMBER 資格的消費紀錄投影還沒有接上：賣家的 API 不能建立 MEMBER 活動，所以正常情形走不到這裡；萬一有 MEMBER 活動，領取與列表整個請求失敗，不降級成「不是會員」（BD28）。
var errMembersNotProjected = errors.New("coupon：MEMBER 資格的消費紀錄投影尚未接上")

type couponMembers struct{}

func (couponMembers) IsMember(context.Context, int64, int64) (bool, error) {
	return false, errMembersNotProjected
}

// newCoupon 組裝 coupon 模組。這裡只有 Connect 需要的部分：MEMBER 資格的投影、買家刪帳與到期提醒還沒有接上。
func newCoupon(d *db.DB, sellers *seller.Module) *coupon.Module {
	return coupon.New(coupon.Deps{DB: d, Stores: couponStores{sellers}, Members: couponMembers{}, NewID: uuid.NewV7, Now: time.Now})
}
