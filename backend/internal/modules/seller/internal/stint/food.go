package stint

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// AttachFood 把店家目前核准的食品業者登錄、賣家看得到的未結束登錄（最近送出的那一筆，狀態是待審或被退回才算；之後被核准、撤回或
// 結束的不顯示）與最近一次被平台撤銷的登錄載入到 stores，一批店家兩次查詢。公開、後台與 legacy 的字號都只讀這裡的核准那一筆
// （ApprovedFoodRegistration）。撤銷的那一筆（RevokedFoodRegistration）只在沒有核准、撤銷之後也沒有更新的待審或核准時帶（判斷在
// StoreRevokedFoodRegistrations：更新的核准即使之後又結束也算）：賣家看到它就知道字號被撤銷、要重新送審。個資清理過的列沒有字號，
// 不算核准、不算未結束、不顯示。
func AttachFood(ctx context.Context, q *store.Queries, stores ...*model.Store) error {
	if len(stores) == 0 {
		return nil
	}
	ids := make([]int64, len(stores))
	for i, s := range stores {
		ids[i] = s.StoreID
	}
	rows, err := q.StoreFoodRegistrations(ctx, ids)
	if err != nil {
		return err
	}
	revokedRows, err := q.StoreRevokedFoodRegistrations(ctx, ids)
	if err != nil {
		return err
	}
	approved := make(map[int64]*model.FoodRegistration, len(rows))
	latest := make(map[int64]store.SellerFoodRegistration, len(rows))
	for _, row := range rows {
		if row.State == string(model.FoodRegApproved) {
			approved[row.StoreID] = FoodRegistrationOf(row)
		}
		if prev, ok := latest[row.StoreID]; !ok || row.SubmittedAt.After(prev.SubmittedAt) || (row.SubmittedAt.Equal(prev.SubmittedAt) && row.ID.String() > prev.ID.String()) {
			latest[row.StoreID] = row
		}
	}
	revoked := make(map[int64]*model.FoodRegistration, len(revokedRows))
	for _, row := range revokedRows {
		if row.RegistrationNumber != nil {
			revoked[row.StoreID] = FoodRegistrationOf(row)
		}
	}
	for _, s := range stores {
		s.ApprovedFoodRegistration = approved[s.StoreID]
		s.OpenFoodRegistration = nil
		s.RevokedFoodRegistration = nil
		if last, ok := latest[s.StoreID]; ok {
			s.OpenFoodRegistration = OpenOf(last)
		}
		if s.ApprovedFoodRegistration == nil {
			s.RevokedFoodRegistration = revoked[s.StoreID]
		}
	}
	return nil
}

// FoodRegistrationOf 是登錄列的檢視；被平台撤銷的列帶撤銷時間與理由，其他的結束原因不帶。
func FoodRegistrationOf(row store.SellerFoodRegistration) *model.FoodRegistration {
	out := &model.FoodRegistration{
		ID: row.ID, RegistrationNumber: model.Deref(row.RegistrationNumber), State: model.FoodRegState(row.State),
		SubmittedAt: row.SubmittedAt, DecidedAt: row.DecidedAt, RejectReason: model.Deref(row.RejectReason),
	}
	if row.EndCause != nil && *row.EndCause == string(model.FoodRegRevoked) {
		out.EndedAt = row.EndedAt
		out.EndReason = model.Deref(row.EndReason)
	}
	return out
}

// OpenOf 是登錄列對賣家的未結束檢視：只有待審與被退回而且字號還在的算，其他狀態回 nil。
func OpenOf(row store.SellerFoodRegistration) *model.FoodRegistration {
	state := model.FoodRegState(row.State)
	if (state != model.FoodRegPending && state != model.FoodRegRejected) || row.RegistrationNumber == nil {
		return nil
	}
	return FoodRegistrationOf(row)
}
