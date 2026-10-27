// Package stint 提供買家 stint 鎖定與狀態檢查。
package stint

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Require 鎖定存活 stint；active 為 true 時拒絕 pending。
func Require(ctx context.Context, queries *store.Queries, id int64, active bool) (store.BuyerBuyer, error) {
	if err := model.Positive(id); err != nil {
		return store.BuyerBuyer{}, err
	}
	buyerRow, err := queries.LockBuyer(ctx, id)
	if err != nil {
		return buyerRow, model.Error(err)
	}
	if buyerRow.Status == string(model.Closed) {
		return buyerRow, apperr.NotFound("買家已關閉")
	}
	if active && buyerRow.Status != string(model.Active) {
		return buyerRow, apperr.FailedPrecondition("買家正在刪除中")
	}
	return buyerRow, nil
}

// RequireActiveShared 以共享鎖確認 stint 是營運中：BeginDeletion 的排他鎖要等持有共享鎖的 transaction 提交才拿得到，
// 拿到之後才開始的寫入會看到買家已不是營運中。已關閉回 NotFound，刪除中回 FailedPrecondition。
func RequireActiveShared(ctx context.Context, queries *store.Queries, id int64) error {
	if err := model.Positive(id); err != nil {
		return err
	}
	buyerRow, err := queries.LockBuyerShared(ctx, id)
	if err != nil {
		return model.Error(err)
	}
	if buyerRow.Status == string(model.Closed) || buyerRow.PiiClearedAt != nil {
		return apperr.NotFound("買家已關閉")
	}
	if buyerRow.Status != string(model.Active) {
		return apperr.FailedPrecondition("買家正在刪除中")
	}
	return nil
}

// Profile 將儲存的 stint 轉成公開 profile。
func Profile(buyerRow store.BuyerBuyer) model.Profile {
	profile := model.Profile{
		BuyerID:      buyerRow.BuyerID,
		AccountID:    buyerRow.AccountID,
		Version:      buyerRow.Version,
		Status:       model.Status(buyerRow.Status),
		Phone:        buyerRow.Phone,
		Avatar:       buyerRow.Avatar,
		City:         buyerRow.City,
		Job:          buyerRow.Job,
		CreatedAt:    buyerRow.CreatedAt,
		UpdatedAt:    buyerRow.UpdatedAt,
		DeletedAt:    buyerRow.DeletedAt,
		PIIClearedAt: buyerRow.PiiClearedAt,
	}
	if buyerRow.DisplayName != nil {
		profile.DisplayName = *buyerRow.DisplayName
	}
	if buyerRow.Gender != nil {
		gender := model.Gender(*buyerRow.Gender)
		profile.Gender = &gender
	}
	if buyerRow.BirthDate.Valid {
		profile.BirthDate = &buyerRow.BirthDate.Time
	}
	return profile
}
