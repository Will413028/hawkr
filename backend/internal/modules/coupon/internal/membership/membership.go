// Package membership 是 MEMBER 資格的消費紀錄投影的寫入端：訂單完成事件餵入，(店家, 買家) 一列，重放冪等。
// 讀取端是 qualify.Check（資格的唯一判定）。
package membership

import (
	"context"
	"slices"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Buyers 是「這些買家存在」的 port：已刪帳的買家不再寫入投影。
type Buyers interface {
	Existing(ctx context.Context, ids []int64) ([]int64, error)
}

// Deps 是投影寫入的依賴。
type Deps struct {
	DB     *db.DB
	Buyers Buyers
	Now    func() time.Time
}

// Service 是投影寫入。
type Service struct{ d Deps }

// New 建立投影寫入。
func New(d Deps) *Service { return &Service{d: d} }

// Record 記下買家在該店有完成的訂單；recorded 為 false 表示已經記過（重放）或買家已不存在（不寫入）。寫入在 ctx 的 transaction 內，沒有時是單一語句。
func (s *Service) Record(ctx context.Context, storeID, buyerID int64) (recorded bool, err error) {
	if storeID <= 0 {
		return false, model.Invalid(ctx, "StoreID", model.InvalidStoreInvalid)
	}
	if buyerID <= 0 {
		return false, model.Invalid(ctx, "BuyerID", model.InvalidBuyerIDInvalid)
	}
	existing, err := s.d.Buyers.Existing(ctx, []int64{buyerID})
	if err != nil {
		return false, err
	}
	if !slices.Contains(existing, buyerID) {
		return false, nil
	}
	n, err := store.New(s.d.DB.Conn(ctx)).UpsertMembership(ctx, store.UpsertMembershipParams{StoreID: storeID, BuyerID: buyerID, Now: s.d.Now()})
	return n > 0, err
}
