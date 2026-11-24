// Package purge 處理買家刪帳（D8、BD19）：同一個 transaction 內作廢該買家 AVAILABLE 的券、刪除指定名單與消費紀錄投影。
package purge

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Deps 是刪帳處理的依賴。
type Deps struct {
	DB  *db.DB
	Now func() time.Time
}

// Service 是刪帳處理。
type Service struct{ d Deps }

// New 建立刪帳處理。
func New(d Deps) *Service { return &Service{d: d} }

const logReason = "BUYER_DELETED"

// EraseBuyer 清掉買家在 coupon 的狀態；冪等（重跑沒有東西可動）。必須由呼叫方的 transaction 提供原子性：所有寫入走 db.Conn(ctx)。
// LOCKED 的券不動（持鎖訂單的 Release 或 Redeem 處理），已核銷與已作廢的券與稽核紀錄留作紀錄。
func (s *Service) EraseBuyer(ctx context.Context, buyerID int64) error {
	if buyerID <= 0 {
		return model.Invalid(ctx, "BuyerID", model.InvalidBuyerIDInvalid)
	}
	q := store.New(s.d.DB.Conn(ctx))
	now := s.d.Now()
	voided, err := q.VoidAvailableVouchersByBuyer(ctx, store.VoidAvailableVouchersByBuyerParams{BuyerID: buyerID, Now: &now})
	if err != nil {
		return err
	}
	reason := logReason
	for _, v := range voided {
		b := buyerID
		if err := q.InsertLog(ctx, store.InsertLogParams{CampaignID: v.CampaignID, VoucherID: pgtype.UUID{Bytes: v.ID, Valid: true}, BuyerID: &b, Action: "VOID", Reason: &reason, Now: now}); err != nil {
			return err
		}
	}
	if _, err := q.DeleteTargetAccountsByBuyer(ctx, buyerID); err != nil {
		return err
	}
	_, err = q.DeleteMembershipsByBuyer(ctx, buyerID)
	return err
}
