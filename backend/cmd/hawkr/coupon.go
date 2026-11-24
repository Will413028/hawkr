package main

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
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

// StoreState 是 coupon 的 StoreDirectory port 的 adapter：店家不存在（NotFound）回 Exists=false，其他讀取失敗原樣回錯誤；停權不看。
func (s couponStores) StoreState(ctx context.Context, storeID int64) (coupon.StoreState, error) {
	st, err := s.sellers.Store(ctx, storeID)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return coupon.StoreState{}, nil
	}
	if err != nil {
		return coupon.StoreState{}, err
	}
	return coupon.StoreState{Exists: true, Closed: st.Status == seller.Closed}, nil
}

// couponBuyers 是 coupon 的 Buyers port 的 adapter：分批問 buyer 模組存活的 profile。
type couponBuyers struct{ buyers buyerReader }

// buyerReader 是 couponBuyers 需要的 buyer 讀取（*buyer.Module）。
type buyerReader interface {
	BatchMeta(ctx context.Context, ids []int64) ([]buyer.Profile, error)
}

func (b couponBuyers) Existing(ctx context.Context, ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	for chunk := range slices.Chunk(ids, buyer.MaxBatchSize) {
		ps, err := b.buyers.BatchMeta(ctx, chunk)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			out = append(out, p.BuyerID)
		}
	}
	return out, nil
}

// errMembersNotProjected 表示 MEMBER 資格的消費紀錄投影還沒有接上：賣家的 API 不能建立 MEMBER 活動，所以正常情形走不到這裡；萬一有 MEMBER 活動，領取與列表整個請求失敗，不降級成「不是會員」（BD28）。
var errMembersNotProjected = errors.New("coupon：MEMBER 資格的消費紀錄投影尚未接上")

type couponMembers struct{}

func (couponMembers) IsMember(context.Context, int64, int64) (bool, error) {
	return false, errMembersNotProjected
}

// newCoupon 組裝 coupon 模組。這裡只有 Connect 需要的部分：MEMBER 資格的投影、買家刪帳與到期提醒還沒有接上。
func newCoupon(d *db.DB, sellers *seller.Module, buyers *buyer.Module) *coupon.Module {
	stores := couponStores{sellers}
	return coupon.New(coupon.Deps{DB: d, Stores: stores, Members: couponMembers{}, Buyers: couponBuyers{buyers}, StoreDirectory: stores, NewID: uuid.NewV7, Now: time.Now})
}
