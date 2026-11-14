package account

import (
	"log/slog"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// create_unresolved：結果不明而且冪等鍵已過保存期，列停在等管理員處理的終局（建立失敗，或清理的 unresolved_creation 終局）才為真；其他情況為假。
// mutation：createUnresolved 不含清理終局 → 第二列為假，失敗；不看 blockedAsUnresolved → 其他原因的終局為真，失敗；不看 keyExpired → 保存期內為真，失敗。
func TestCreateUnresolvedCoversTheCreateFailedAndThePurgeTerminalStatesPastTheRetention(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	old := now.Add(-IdempotencyRetention - time.Hour)
	recent := now.Add(-UnresolvedDeadline - time.Hour)
	reason := func(s string) *string { return &s }
	failed := &now
	for name, tc := range map[string]struct {
		row  store.PaymentAccount
		want bool
	}{
		"建立失敗、超過保存期":            {store.PaymentAccount{CreateCalledAt: &old, CreateFailedAt: failed}, true},
		"建立失敗、保存期內":             {store.PaymentAccount{CreateCalledAt: &recent, CreateFailedAt: failed}, false},
		"清理終局 unresolved、超過保存期": {store.PaymentAccount{CreateCalledAt: &old, PurgePendingAt: &old, PurgeBlockedAt: &old, PurgeBlockedReason: reason("unresolved_creation：x")}, true},
		"清理終局 unresolved、保存期內":  {store.PaymentAccount{CreateCalledAt: &recent, PurgePendingAt: &old, PurgeBlockedAt: &old, PurgeBlockedReason: reason("unresolved_creation：x")}, false},
		"清理終局其他原因、超過保存期":        {store.PaymentAccount{CreateCalledAt: &old, PurgePendingAt: &old, PurgeBlockedAt: &old, PurgeBlockedReason: reason("provider_refused：x")}, false},
		"清理中沒有終局、超過保存期":         {store.PaymentAccount{CreateCalledAt: &old, PurgePendingAt: &old}, false},
	} {
		if got := createUnresolved(tc.row, now); got != tc.want {
			t.Errorf("%s：createUnresolved=%v，預期 %v", name, got, tc.want)
		}
	}
}

// 後台寫入沒有紀錄就不成立：缺 Audit 在建構時 panic，不等到第一次後台寫入。
// mutation：New 不檢查 Audit → 沒有 panic，失敗。
func TestNewRequiresTheAuditLog(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("缺 Audit 沒有 panic")
		}
	}()
	New(Deps{DB: &db.DB{}, Events: events.NewBus(), Jobs: noopJobs{}, Now: time.Now, Logger: slog.Default()})
}
