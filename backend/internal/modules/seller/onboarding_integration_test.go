//go:build integration

package seller_test

import (
	"context"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// onboardingOf 以真實的來源讀店家的開通任務：條件來自讀店家（view）、KYC 與收款帳戶各自讀。
func (f *fixture) onboardingOf(t *testing.T, storeID int64) (Store, Onboarding) {
	t.Helper()
	ctx := context.Background()
	cur, err := f.module.Store(ctx, storeID)
	must(t, err)
	in := OnboardingInput{Store: cur}
	app, found, err := f.module.StoreKYC(ctx, storeID)
	must(t, err)
	in.KYC = OnboardingKYCRead{Found: found, Status: app.Status}
	acct, err := f.payments.Account(ctx, storeID)
	switch {
	case err == nil:
		in.Payment = OnboardingPaymentRead{Found: true, Status: string(acct.Status)}
	case apperr.KindOf(err) != apperr.KindNotFound:
		t.Fatal(err)
	}
	return cur, OnboardingTasksOf(uitext.ZhTW, in)
}

// 開通閘門、任務與曝光同源：「平台審核與收款帳戶兩項任務都 COMPLETED」⇔ view 的 onboarding_complete ⇔ OnboardingTasksOf 的 Complete ⇔ CapabilitiesOf
// 沒有缺 ONBOARDING_COMPLETE ⇔ 商店資料齊全的店家可曝光。逐一走過：申請審核中、已核准但沒有收款帳戶、轉帳能力未啟用、啟用、又失效。
// mutation 1：Conditions 的閘門改成只看 KYCApproved → 轉帳能力未啟用的階段 CapabilitiesOf 與 view 不一致，失敗。
// mutation 2：任務的收款帳戶完成改看帳戶狀態而不是閘門副本 → 帳戶狀態與副本不同步時任務與 view 不一致（單元測試也擋）。
func TestOnboardingTasksAgreeWithTheGateAndTheView(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.approvedStore(t)
	f.logoed(t, accountID, s)
	_, pending, _ := f.submitted(t)
	stages := []struct {
		name  string
		store int64
		apply func()
		want  bool
	}{
		{"申請審核中", pending.StoreID, func() {}, false},
		{"已核准、還沒有收款帳戶", s.StoreID, func() {}, false},
		{"帳戶要賣家補資料", s.StoreID, func() { f.setPayment(t, s.StoreID, needsSellerSnapshot) }, false},
		{"帳戶被拒絕", s.StoreID, func() { f.setPayment(t, s.StoreID, rejectedSnapshot) }, false},
		{"轉帳能力啟用", s.StoreID, func() { f.setPayment(t, s.StoreID, enabledSnapshot) }, true},
		{"轉帳能力又失效", s.StoreID, func() { f.setPayment(t, s.StoreID, needsSellerSnapshot) }, false},
	}
	for _, st := range stages {
		st.apply()
		cur, o := f.onboardingOf(t, st.store)
		var viewComplete bool
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT onboarding_complete FROM seller.store_requirement_flags WHERE store_id = $1", st.store).Scan(&viewComplete))
		gateTasks := true
		for _, task := range o.Tasks {
			if (task.ID == OnboardingPlatformReview || task.ID == OnboardingPaymentAccount) && task.State != OnboardingCompleted {
				gateTasks = false
			}
		}
		missingGate := false
		for _, c := range CapabilitiesOf(cur) {
			if c.Capability == CapPublicListing {
				for _, r := range c.Missing {
					missingGate = missingGate || r == RequirementOnboardingComplete
				}
			}
		}
		if gateTasks != st.want || viewComplete != st.want || o.Complete != st.want || missingGate == st.want {
			t.Errorf("%s：兩項閘門任務完成=%v、view=%v、Complete=%v、CapabilitiesOf 缺閘門=%v，預期完成=%v", st.name, gateTasks, viewComplete, o.Complete, missingGate, st.want)
		}
		if st.store == s.StoreID {
			if exposed := len(f.exposed(t, s.StoreID)) == 1; exposed != st.want {
				t.Errorf("%s：可曝光=%v，預期 %v", st.name, exposed, st.want)
			}
		}
	}
}
