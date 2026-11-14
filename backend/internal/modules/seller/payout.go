package seller

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/payout"
)

// OnPayoutAccountChanged 是 payment 的 PayoutAccountChanged 的同 transaction 訂閱者（cmd/hawkr 接線；Stripe 計畫 D28）：把轉帳能力複製到開通閘門
// 讀的副本、把外部銀行帳號 fingerprint 寫成防詐比對 hash（D12），轉帳能力有變時重新公告可曝光。
func (m *Module) OnPayoutAccountChanged(ctx context.Context, e payment.PayoutAccountChanged) error {
	return m.payout.Apply(ctx, payout.Change{StoreID: e.StoreID, Provider: e.Provider, TransfersActive: e.TransfersActive, Fingerprints: e.BankFingerprints, At: e.OccurredAt})
}
