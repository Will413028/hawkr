//go:build integration

package seller_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 收款帳戶由 payment 模組擁有（Stripe 計畫 D28）：這裡測 seller 這一側——核准經 payment 的公開 API 寫帳戶列、PayoutAccountChanged 的訂閱者更新
// 開通閘門的副本。payment 自己的 job、inbox 與輪詢在 payment 模組的 integration 測試。

const kindPaymentEnsure = "payment.account_ensure"

var (
	enabledSnapshot = payment.Snapshot{Transfers: payment.CapabilityActive, PayoutsActive: true}
	// needsSellerSnapshot 是賣家要補資料（新帳戶的樣子）；rejectedSnapshot 是被金流商拒絕。
	needsSellerSnapshot = payment.Snapshot{Transfers: payment.CapabilityRestricted, Details: []payment.StatusDetail{{Resolution: payment.ResolutionSellerAction, Reason: "routine_onboarding"}}}
	rejectedSnapshot    = payment.Snapshot{Transfers: payment.CapabilityRejected, Details: []payment.StatusDetail{{Resolution: payment.ResolutionPlatformAction, Reason: "rejected.fraud"}}}
)

// queuedStoreIDs 是 fake queue 裡某個種類的 job 排入過的店家（依排入順序，不去重）。
func (q *queue) queuedStoreIDs(t *testing.T, kind string) []int64 {
	t.Helper()
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []int64
	for _, a := range q.args {
		if a.Kind() != kind {
			continue
		}
		raw, err := json.Marshal(a)
		must(t, err)
		var v struct {
			StoreID int64 `json:"store_id"`
		}
		must(t, json.Unmarshal(raw, &v))
		out = append(out, v.StoreID)
	}
	return out
}

// listingMissing 是店家 PUBLIC_LISTING 缺的條件。
func listingMissing(s Store) []Requirement {
	for _, st := range CapabilitiesOf(s) {
		if st.Capability == CapPublicListing {
			return st.Missing
		}
	}
	return nil
}

func (f *fixture) paymentAccount(t *testing.T, storeID int64) payment.Account {
	t.Helper()
	a, err := f.payments.Account(context.Background(), storeID)
	must(t, err)
	return a
}

// ensureAccount 執行 payment 的建立 job 一輪，回傳外部帳戶 ID。
func (f *fixture) ensureAccount(t *testing.T, storeID int64) string {
	t.Helper()
	must(t, f.payments.EnsureAccount(context.Background(), storeID, false))
	a := f.paymentAccount(t, storeID)
	if a.ExternalAccountID == nil {
		t.Fatalf("建立之後仍沒有外部帳戶：%+v", a)
	}
	return *a.ExternalAccountID
}

// setPayment 讓店家的外部帳戶回這個快照，並執行 payment 的同步 job 一輪（還沒有外部帳戶時先建立）；同步發的 PayoutAccountChanged 經 seller 的訂閱者
// 更新閘門的副本。
func (f *fixture) setPayment(t *testing.T, storeID int64, s payment.Snapshot) {
	t.Helper()
	external := ""
	if a := f.paymentAccount(t, storeID); a.ExternalAccountID != nil {
		external = *a.ExternalAccountID
	} else {
		external = f.ensureAccount(t, storeID)
	}
	f.payment.Set(external, s)
	again, err := f.payments.SyncAccount(context.Background(), storeID)
	must(t, err)
	if again {
		t.Fatalf("沒有事件卻要求再跑一輪")
	}
}

// setContactEmails 設定帳號的已驗證 email 與自填信箱（nil 是沒有）。
func (f *fixture) setContactEmails(t *testing.T, accountID int64, verified, selfEntered *string) {
	t.Helper()
	ctx := context.Background()
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE identity.accounts SET email = $2, self_entered_email = $3 WHERE id = $1", accountID, verified, selfEntered)
	must(t, err)
}

// 核准時從 identity 讀帳號的聯絡信箱（已驗證的 email 優先於自填的），正規化後傳給 payment 快照到帳戶列，建立 job 從列上讀出來交給 provider；之後帳號的信箱
// 再改，列上的值不變（provider 收到的仍是核准當下的）。
// mutation：改成自填優先 → provider 收到 self@example.com，失敗；核准不傳帳號的信箱（固定值）→ provider 收到的不是帳號的，失敗。
func TestApprovalSnapshotsTheAccountContactEmailVerifiedFirst(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.open(t)
	f.setContactEmails(t, accountID, ptr("Pay.Out@Example.COM"), ptr("self@example.com"))
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	must(t, approve(f, sub))
	f.setContactEmails(t, accountID, ptr("changed@example.com"), nil)
	f.ensureAccount(t, s.StoreID)
	if got := f.payment.EnsureEmails(); !slices.Equal(got, []string{"pay.out@example.com"}) {
		t.Fatalf("provider 收到的信箱=%v，預期核准當下已驗證的 email（小寫）", got)
	}
}

// 沒有已驗證的 email 時用自填的（identity 只存小寫的自填信箱，accounts_self_entered_email_check）。
// mutation：核准只讀已驗證的 email → 核准因沒有聯絡信箱被拒，失敗。
func TestApprovalSnapshotsTheSelfEnteredEmailWhenThereIsNoVerifiedOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.open(t)
	f.setContactEmails(t, accountID, nil, ptr("self@example.com"))
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	must(t, approve(f, sub))
	f.ensureAccount(t, s.StoreID)
	if got := f.payment.EnsureEmails(); !slices.Equal(got, []string{"self@example.com"}) {
		t.Fatalf("provider 收到的信箱=%v", got)
	}
}

// reapprove 模擬同一家店再一次核准：舊的申請已取消（唯一索引只允許一筆沒取消的申請），帳號換了（已驗證的）聯絡信箱後賣家送出新申請並核准。
func (f *fixture) reapprove(t *testing.T, accountID int64, email string) {
	t.Helper()
	ctx := context.Background()
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.kyc_applications SET status = 'CANCELLED', decided_at = NULL WHERE store_id IN (SELECT store_id FROM seller.stores WHERE account_id = $1)", accountID)
	must(t, err)
	f.setContactEmails(t, accountID, &email, nil)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	must(t, approve(f, sub))
}

// 帳號沒有聯絡信箱（沒有已驗證的 email，也沒有自填的）時送出被拒：failed_precondition 與 CONTACT_EMAIL_REQUIRED，申請維持草稿；補上之後可以送出。
// mutation：SubmitKYC 拿掉聯絡信箱的檢查 → 送出成功，失敗。
func TestSubmitKYCRequiresAContactEmail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, _ := f.open(t)
	f.setContactEmails(t, accountID, nil, nil)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonContactEmailRequired {
		t.Fatalf("err=%v，預期 FailedPrecondition 與 CONTACT_EMAIL_REQUIRED", err)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.kyc_applications WHERE status = 'DRAFT'"); got != 1 {
		t.Fatalf("申請應仍是草稿，草稿數=%d", got)
	}
	f.setContactEmails(t, accountID, nil, ptr("seller@example.com"))
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
}

// 再次核准時帳戶列已存在：核准的 transaction 只 INSERT 帳戶列（ON CONFLICT DO NOTHING），不鎖、不改 payment 的既有列（全域鎖順序 payment 先於
// seller，Stripe 計畫 7d′），所以聯絡信箱維持第一次核准的快照，建立外部帳戶送的也是它；還沒有外部帳戶的列重新排入建立 job。
// mutation：RequestAccount 改回 upsert 信箱 → provider 收到新的信箱，失敗（integration 的 lockorder 檢查也會報核准鎖了下層的既有列）。
func TestReapprovalKeepsTheFirstContactEmailSnapshot(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.approvedWith(t, idA, bankAcct)
	f.reapprove(t, accountID, "New.Owner@Example.com")
	if got := f.jobs.queuedStoreIDs(t, kindPaymentEnsure); !slices.Equal(got, []int64{s.StoreID, s.StoreID}) {
		t.Fatalf("兩次核准排入的建立 job=%v", got)
	}
	f.ensureAccount(t, s.StoreID)
	if got := f.payment.EnsureEmails(); !slices.Equal(got, []string{"seller@example.com"}) {
		t.Fatalf("provider 收到的信箱=%v，預期第一次核准的快照", got)
	}
}

// 核准的 transaction 內經 payment 寫「建立中」的列並排入建立 job，外部呼叫不在核准裡；沒有設定收款開通（payment 沒有 provider）時核准照常、不寫帳戶列。
// mutation：核准不呼叫 RequestAccount → 沒有建立 job，失敗。
func TestApprovalQueuesOneAccountCreationThroughPayment(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedStore(t)
	if got := f.jobs.queuedStoreIDs(t, kindPaymentEnsure); !slices.Equal(got, []int64{s.StoreID}) {
		t.Fatalf("核准排入的建立 job=%v", got)
	}
	if ensures, _, _ := f.payment.Calls(); ensures != 0 {
		t.Fatalf("核准呼叫了 provider：%d 次", ensures)
	}
	a := f.paymentAccount(t, s.StoreID)
	if a.Status != payment.StatusNotCreated || a.ExternalAccountID != nil || a.Provider != payment.FakeName || a.TransfersActive {
		t.Fatalf("建立中的列=%+v", a)
	}
	if v := f.payment.Violations(); len(v) != 0 {
		t.Fatalf("違反呼叫規則：%v", v)
	}
}

// 開通閘門看轉帳能力：平台審核核准、帳戶存在但轉帳能力沒有啟用時不可曝光，缺的條件是 ONBOARDING_COMPLETE；啟用後可曝光，再變成未啟用又不可曝光。
// 副本由 payment 同步的 transaction 內的訂閱者（OnPayoutAccountChanged）寫入，seller 不讀 payment 的表。
// mutation 1：store_requirement_flags 的 onboarding_complete 拿掉 `p.transfers_active` → 轉帳能力未啟用的店家仍可曝光，失敗。
// mutation 2：payout.Apply 不寫副本（拿掉 UpsertPayoutCapability）→ 啟用後仍不可曝光、副本沒有列，失敗。
func TestExposureRequiresTheTransfersCapabilityOfThePaymentAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.approvedStore(t)
	f.logoed(t, accountID, s)
	for _, snap := range []payment.Snapshot{
		{},
		{Transfers: payment.CapabilityPending},
		needsSellerSnapshot,
		rejectedSnapshot,
	} {
		f.setPayment(t, s.StoreID, snap)
		if got := f.exposed(t, s.StoreID); len(got) != 0 {
			t.Fatalf("快照 %+v 下店家可曝光", snap)
		}
		cur, err := f.module.Store(ctx, s.StoreID)
		must(t, err)
		if missing := listingMissing(cur); !slices.Equal(missing, []Requirement{RequirementOnboardingComplete}) {
			t.Fatalf("快照 %+v 的缺項=%v", snap, missing)
		}
	}
	f.setPayment(t, s.StoreID, enabledSnapshot)
	if got := f.exposed(t, s.StoreID); !slices.Equal(got, []int64{s.StoreID}) {
		t.Fatalf("轉帳能力啟用後 exposed=%v", got)
	}
	if a := f.paymentAccount(t, s.StoreID); a.Status != payment.StatusEnabled || !a.TransfersActive || !a.PayoutsActive {
		t.Fatalf("帳戶=%+v", a)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.payout_capabilities WHERE store_id = $1 AND transfers_active AND provider = 'fake'", s.StoreID); n != 1 {
		t.Fatalf("閘門副本沒有跟著轉帳能力啟用：%d", n)
	}
	f.setPayment(t, s.StoreID, needsSellerSnapshot)
	if got := f.exposed(t, s.StoreID); len(got) != 0 {
		t.Fatalf("轉帳能力又失效後仍可曝光：%v", got)
	}
	// 沒有帳戶列的已核准店家不通過閘門。
	accountID2, s2, sub2 := f.submitted(t, idB, "999888777666")
	_ = accountID2
	must(t, approve(f, sub2))
	if got := f.exposed(t, s2.StoreID); len(got) != 0 {
		t.Fatalf("還沒有同步過（沒有閘門副本）的店家可曝光：%v", got)
	}
}
