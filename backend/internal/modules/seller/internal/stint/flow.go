package stint

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// ToPayUniRegistration 把 PayUni 註冊列轉成公開型別。
func ToPayUniRegistration(row store.SellerPayuniRegistration) model.PayUniRegistration {
	outcome := model.PayUniOutcome(row.Outcome)
	bound := model.PayUniBindStatus(row.BindStatus) == model.BindBound
	mer := ""
	if row.MerStatus != nil {
		mer = *row.MerStatus
	}
	return model.PayUniRegistration{
		StoreID: row.StoreID, MerID: row.MerID, ApplicationID: row.ApplicationID,
		BindStatus: model.PayUniBindStatus(row.BindStatus), BindErrorCode: row.BindErrorCode, BindAttempts: int(row.BindAttempts),
		BoundAt:     row.BoundAt,
		SyncPending: row.SyncedSeq < row.IntentSeq, SyncErrorCode: row.SyncErrorCode,
		Status: model.PayUniStatus{
			MerStatus: row.MerStatus, MemberStatus: row.MemberStatus, CreditStatus: row.CreditStatus,
			ApplePayStatus: row.ApplepayStatus, GooglePayStatus: row.GooglepayStatus, ATMStatus: row.AtmStatus, Reason: row.Reason,
			Outcome: outcome, ObservedAt: row.StatusObservedAt, CheckedAt: row.StatusCheckedAt,
		},
		PaymentEnabled: bound && outcome == model.OutcomeApproved,
		// 商家層級的終止狀態（PayUni 回報的，或認領時查到的），或綁定因它而失敗。
		NeedsReonboarding: model.TerminalMerStatus(mer) || (row.BindErrorCode != nil && *row.BindErrorCode == model.ErrMerchantTerminal),
		CreatedAt:         row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// FlowStatusesOf 是各店家的流程狀態（model.KYCFlowStatus）：statuses 是各店家目前申請的 cert 狀態（沒有申請的店家不在裡面，回傳也沒有它），
// 固定一次查詢讀全部註冊，階段一律經 model.KYCFlowStatusOf。CanResubmit 要註冊帶賣家型態與店家缺的欄位，單一申請的流程走 payuni.SellerViewOf。
func FlowStatusesOf(ctx context.Context, q *store.Queries, statuses map[int64]model.KYCStatus) (map[int64]model.KYCFlowStatus, error) {
	ids := make([]int64, 0, len(statuses))
	for id := range statuses {
		ids = append(ids, id)
	}
	rows, err := q.PayUniRegistrationsByStores(ctx, ids)
	if err != nil {
		return nil, err
	}
	registrations := make(map[int64]model.PayUniRegistration, len(rows))
	for _, row := range rows {
		registrations[row.StoreID] = ToPayUniRegistration(row)
	}
	out := make(map[int64]model.KYCFlowStatus, len(statuses))
	for id, status := range statuses {
		if reg, ok := registrations[id]; ok {
			out[id] = model.KYCFlowStatusOf(status, &reg)
		} else {
			out[id] = model.KYCFlowStatusOf(status, nil)
		}
	}
	return out, nil
}
