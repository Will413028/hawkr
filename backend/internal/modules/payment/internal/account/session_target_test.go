package account

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 開通 session 的前置條件（App 的內嵌路徑與一般路徑共用同一個檢查）：關閉中、沒有外部帳戶、provider 不符、被停用都是 failed_precondition。
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

func TestSurfaceOfFollowsTheRequirementCollection(t *testing.T) {
	if model.SurfaceOf(model.CollectionPlatform) != model.SurfaceEmbedded || model.SurfaceOf(model.CollectionProvider) != model.SurfaceHosted {
		t.Fatal("收件方式與介面的對應不符")
	}
}

// App（canEmbed 為 false）加內嵌列：前置條件照常檢查，通過才回「到網站完成」（不呼叫 provider）；介面取自列的收件方式。
func TestPlanSessionKeepsThePreconditionsOnTheCannotEmbedPath(t *testing.T) {
	ext := "acct_1"
	now := time.Now()
	row := func(collection model.RequirementCollection) store.PaymentAccount {
		return store.PaymentAccount{StoreID: 1, Provider: FakeName, ExternalAccountID: &ext, RequirementCollection: string(collection), Status: string(model.PaymentUnderReview)}
	}
	for name, mutate := range map[string]func(*store.PaymentAccount){
		"關閉中":         func(r *store.PaymentAccount) { r.PurgePendingAt = &now },
		"沒有外部帳戶":      func(r *store.PaymentAccount) { r.ExternalAccountID = nil },
		"provider 不符": func(r *store.PaymentAccount) { r.Provider = "other" },
		"被停用":         func(r *store.PaymentAccount) { r.Status = string(model.PaymentDisabled) },
	} {
		r := row(model.CollectionPlatform)
		mutate(&r)
		if _, complete, err := planSession(r, FakeName, false); apperr.KindOf(err) != apperr.KindFailedPrecondition || complete {
			t.Errorf("%s：complete=%v err=%v", name, complete, err)
		}
	}
	if _, complete, err := planSession(row(model.CollectionPlatform), FakeName, false); err != nil || !complete {
		t.Errorf("內嵌列、App：complete=%v err=%v", complete, err)
	}
	if _, complete, err := planSession(row(model.CollectionPlatform), FakeName, true); err != nil || complete {
		t.Errorf("內嵌列、網站：complete=%v err=%v", complete, err)
	}
	if _, complete, err := planSession(row(model.CollectionProvider), FakeName, false); err != nil || complete {
		t.Errorf("託管列、App 仍拿 provider 連結：complete=%v err=%v", complete, err)
	}
}
