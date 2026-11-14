package account

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 開通 session 的前置條件（託管連結與內嵌共用同一個檢查）：關閉中、沒有外部帳戶、provider 不符、被停用都是 failed_precondition。
func TestSessionTargetRequiresAnOpenExistingAccountOfTheConfiguredProvider(t *testing.T) {
	ext := "acct_1"
	now := time.Now()
	ok := store.PaymentAccount{StoreID: 1, Provider: FakeName, ExternalAccountID: &ext, RequirementCollection: string(model.CollectionPlatform), Status: string(model.PaymentUnderReview)}
	if got, err := sessionTarget(ok, FakeName); err != nil || got.ExternalAccountID == nil || *got.ExternalAccountID != ext {
		t.Fatalf("可開通的列：%+v %v", got, err)
	}
	for name, mutate := range map[string]func(*store.PaymentAccount){
		"關閉中":         func(r *store.PaymentAccount) { r.PurgePendingAt = &now },
		"沒有外部帳戶":      func(r *store.PaymentAccount) { r.ExternalAccountID = nil },
		"provider 不符": func(r *store.PaymentAccount) { r.Provider = "other" },
		"被停用":         func(r *store.PaymentAccount) { r.Status = string(model.PaymentDisabled) },
	} {
		row := ok
		mutate(&row)
		if _, err := sessionTarget(row, FakeName); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("%s：err=%v", name, err)
		}
	}
}
