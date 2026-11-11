package stint

import (
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
