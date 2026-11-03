//go:build integration

package seller_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	kindPaymentEnsure = "seller.payment_ensure"
	kindPaymentSync   = "seller.payment_sync"
)

var (
	enabledSnapshot = PaymentSnapshot{Transfers: PaymentCapabilityActive, PayoutsActive: true}
	// needsSellerSnapshot 是賣家要補資料（新帳戶的樣子）；rejectedSnapshot 是被金流商拒絕。
	needsSellerSnapshot = PaymentSnapshot{Transfers: PaymentCapabilityRestricted, Details: []PaymentStatusDetail{{Resolution: PaymentResolutionSellerAction, Reason: "routine_onboarding"}}}
	rejectedSnapshot    = PaymentSnapshot{Transfers: PaymentCapabilityRejected, Details: []PaymentStatusDetail{{Resolution: PaymentResolutionPlatformAction, Reason: "rejected.fraud"}}}
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

func (f *fixture) paymentAccount(t *testing.T, storeID int64) PaymentAccount {
	t.Helper()
	a, err := f.module.PaymentAccountOf(context.Background(), storeID)
	must(t, err)
	return a
}

// ensureAccount 執行建立 job 一輪，回傳外部帳戶 ID。
func (f *fixture) ensureAccount(t *testing.T, storeID int64) string {
	t.Helper()
	must(t, f.module.EnsurePaymentAccount(context.Background(), storeID, false))
	a := f.paymentAccount(t, storeID)
	if a.ExternalAccountID == nil {
		t.Fatalf("建立之後仍沒有外部帳戶：%+v", a)
	}
	return *a.ExternalAccountID
}

// setPayment 讓店家的外部帳戶回這個快照，並執行同步 job 一輪（還沒有外部帳戶時先建立）。
func (f *fixture) setPayment(t *testing.T, storeID int64, s PaymentSnapshot) {
	t.Helper()
	external := ""
	if a := f.paymentAccount(t, storeID); a.ExternalAccountID != nil {
		external = *a.ExternalAccountID
	} else {
		external = f.ensureAccount(t, storeID)
	}
	f.payment.Set(external, s)
	again, err := f.module.SyncPaymentAccount(context.Background(), storeID)
	must(t, err)
	if again {
		t.Fatalf("沒有事件卻要求再跑一輪")
	}
}

func (f *fixture) paymentEvent(id, external string) (http.Header, []byte) {
	h := http.Header{}
	h.Set(PaymentFakeEventHeader, id)
	return h, []byte(external)
}

// 申請上的收款聯絡信箱（D23）存的是正規化後的值，核准時複製到帳戶列，建立 job 從列上讀出來交給 provider；之後申請的草稿再怎麼改，列上的值不變。
// mutation：核准不複製信箱（寫入固定值或申請以外的來源）→ 列上的信箱與 provider 收到的信箱都不是申請上的值，失敗。
func TestApprovalCopiesThePayoutContactEmailToTheAccountRowAndTheCreateCallSendsIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.open(t)
	in := kycDraft(accountID, idA, bankAcct)
	in.PayoutContactEmail = ptr("  Pay.Out@Example.COM ")
	d := f.savedDraft(t, in)
	if d.PayoutContactEmail == nil || *d.PayoutContactEmail != "pay.out@example.com" {
		t.Fatalf("草稿的信箱=%v，預期去空白並轉小寫", d.PayoutContactEmail)
	}
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	must(t, approve(f, sub))
	var stored string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT contact_email FROM seller.payment_accounts WHERE store_id = $1", s.StoreID).Scan(&stored))
	if stored != "pay.out@example.com" {
		t.Fatalf("列上的信箱=%q", stored)
	}
	f.ensureAccount(t, s.StoreID)
	if got := f.payment.EnsureEmails(); !slices.Equal(got, []string{"pay.out@example.com"}) {
		t.Fatalf("provider 收到的信箱=%v", got)
	}
}

// reapprove 模擬同一家店再一次核准：舊的申請已取消（唯一索引只允許一筆沒取消的申請），賣家用新信箱送出新申請並核准。
func (f *fixture) reapprove(t *testing.T, accountID int64, email string) {
	t.Helper()
	ctx := context.Background()
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.kyc_applications SET status = 'CANCELLED', decided_at = NULL WHERE store_id IN (SELECT store_id FROM seller.stores WHERE account_id = $1)", accountID)
	must(t, err)
	in := kycDraft(accountID, idA, bankAcct)
	in.PayoutContactEmail = ptr(email)
	d := f.savedDraft(t, in)
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	must(t, approve(f, sub))
}

func (f *fixture) accountEmail(t *testing.T, storeID int64) string {
	t.Helper()
	ctx := context.Background()
	var email string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT contact_email FROM seller.payment_accounts WHERE store_id = $1", storeID).Scan(&email))
	return email
}

// 再次核准時帳戶列已存在：還沒有外部帳戶（建立失敗或 job 沒跑）的列改用新申請的信箱，建立才會送最新的；已經有外部帳戶的列保留原信箱（D23 延後改信箱）。
// mutation：InsertPaymentAccount 改回 ON CONFLICT DO NOTHING → 第一段的信箱仍是舊的，失敗；拿掉 WHERE external_account_id IS NULL → 第二段的信箱被覆寫，失敗。
func TestReapprovalReplacesTheContactEmailOnlyWhileTheExternalAccountDoesNotExist(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.approvedWith(t, idA, bankAcct)
	if got := f.accountEmail(t, s.StoreID); got != "seller@example.com" {
		t.Fatalf("第一次核准的信箱=%q", got)
	}
	f.reapprove(t, accountID, "New.Owner@Example.com")
	if got := f.accountEmail(t, s.StoreID); got != "new.owner@example.com" {
		t.Fatalf("沒有外部帳戶時再次核准的信箱=%q，預期換成新申請的", got)
	}
	f.ensureAccount(t, s.StoreID)
	if got := f.payment.EnsureEmails(); !slices.Equal(got, []string{"new.owner@example.com"}) {
		t.Fatalf("provider 收到的信箱=%v", got)
	}
	f.reapprove(t, accountID, "third@example.com")
	if got := f.accountEmail(t, s.StoreID); got != "new.owner@example.com" {
		t.Fatalf("已有外部帳戶的列信箱=%q，預期保留原值", got)
	}
}

// 核准的 transaction 內寫「建立中」的列並排入建立 job，外部呼叫不在核准裡；建立 job 重送（外部帳戶已建立但本地沒記到）時，provider 以列上保存的
// UUID 為冪等鍵回同一個帳戶，不會有第二個；不同店家的鍵不同（兩個資料庫共用同一個 provider 帳號時不撞鍵）。
// mutation：Ensure 每次呼叫產生新的冪等鍵、不用列上保存的 → 重送建出第二個帳戶，且送出的鍵不等於列上的值，失敗。
func TestApprovalQueuesOneAccountCreationAndAReplayedCreateKeepsOneExternalAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s := f.approvedStore(t)
	if got := f.jobs.queuedStoreIDs(t, kindPaymentEnsure); !slices.Equal(got, []int64{s.StoreID}) {
		t.Fatalf("核准排入的建立 job=%v", got)
	}
	if ensures, _, _ := f.payment.Calls(); ensures != 0 {
		t.Fatalf("核准呼叫了 provider：%d 次", ensures)
	}
	a := f.paymentAccount(t, s.StoreID)
	if a.Status != PaymentNotCreated || a.ExternalAccountID != nil || a.Provider != "fake" || a.TransfersActive || a.RequirementCollection != CollectionProvider {
		t.Fatalf("建立中的列=%+v", a)
	}
	first := f.ensureAccount(t, s.StoreID)
	if got := f.paymentAccount(t, s.StoreID); got.Status != PaymentActionRequired {
		t.Fatalf("建立後的狀態=%s", got.Status)
	}
	if got := f.jobs.queuedStoreIDs(t, kindPaymentSync); !slices.Equal(got, []int64{s.StoreID}) {
		t.Fatalf("建立後排入的同步 job=%v", got)
	}
	// 重跑已建立的店家：什麼都不做。
	must(t, f.module.EnsurePaymentAccount(ctx, s.StoreID, false))
	// 模擬 provider 已建立、本地沒記到：把列退回「建立中」再跑一次，provider 以同一個冪等鍵回同一個帳戶。
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.payment_accounts SET external_account_id = NULL, status = 'NOT_CREATED' WHERE store_id = $1", s.StoreID)
	must(t, err)
	second := f.ensureAccount(t, s.StoreID)
	var stored string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT create_idempotency_key::text FROM seller.payment_accounts WHERE store_id = $1", s.StoreID).Scan(&stored))
	if keys := f.payment.EnsureKeys(); len(keys) != 2 || keys[0] != stored || keys[1] != stored || len(stored) != 36 || stored[14] != '4' {
		t.Fatalf("兩次建立送出的冪等鍵=%v，列上保存的 v4 UUID=%q", keys, stored)
	}
	if second != first {
		t.Fatalf("重送建出另一個帳戶：%s 與 %s", first, second)
	}
	if n := f.payment.Accounts(); n != 1 {
		t.Fatalf("外部帳戶數=%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.payment_accounts WHERE store_id = $1", s.StoreID); n != 1 {
		t.Fatalf("帳戶列=%d", n)
	}
	if v := f.payment.Violations(); len(v) != 0 {
		t.Fatalf("違反呼叫規則：%v", v)
	}
}

// 建立與同步 job 都以 store_id 為唯一鍵（排隊中、執行中、等重試不重複排）：連續的核准後讀取、事件與管理員的重新整理都合併成一個。
// mutation：SyncArgs 拿掉 `river:"unique"`（或 ByArgs）→ 同一店家排了多個 job，失敗。
func TestPaymentJobsAreUniquePerStoreInRiver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	must(t, jobs.Migrate(ctx, f.db))
	jc, err := jobs.NewInsertOnly(f.db)
	must(t, err)
	f.module = f.newModule(t, f.payuni, jc)
	_, s := f.approvedStore(t)
	open := []string{"available", "scheduled", "running", "retryable", "pending"}
	jobCount := func(kind string, storeID int64) int64 {
		return f.count(t, "SELECT count(*) FROM river.river_job WHERE kind=$1 AND args->>'store_id'=$2 AND state = ANY($3)", kind, fmt.Sprint(storeID), open)
	}
	if n := jobCount(kindPaymentEnsure, s.StoreID); n != 1 {
		t.Fatalf("核准後的建立 job=%d", n)
	}
	external := f.ensureAccount(t, s.StoreID)
	for i := range 3 {
		must(t, f.module.RefreshPaymentAccount(ctx, s.StoreID))
		h, body := f.paymentEvent(fmt.Sprintf("evt_%d", i), external)
		must(t, f.module.ReceivePaymentEvent(ctx, h, body))
	}
	if n := jobCount(kindPaymentSync, s.StoreID); n != 1 {
		t.Fatalf("同一店家的同步 job=%d，應該合併成一個", n)
	}
	// 另一個店家有自己的 job。
	_, s2, sub2 := f.submitted(t, idB, "999888777666")
	must(t, approve(f, sub2))
	f.ensureAccount(t, s2.StoreID)
	if n := f.count(t, "SELECT count(*) FROM river.river_job WHERE kind=$1 AND state = ANY($2)", kindPaymentSync, open); n != 2 {
		t.Fatalf("兩個店家的同步 job=%d", n)
	}
	// 執行中的 job 也擋下新的 job（唯一的狀態集合含 running）。
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE river.river_job SET state='running', attempted_at=now() WHERE kind=$1 AND args->>'store_id'=$2", kindPaymentSync, fmt.Sprint(s.StoreID))
	must(t, err)
	must(t, f.module.RefreshPaymentAccount(ctx, s.StoreID))
	if n := jobCount(kindPaymentSync, s.StoreID); n != 1 {
		t.Fatalf("執行中又排了新的同步 job：%d", n)
	}
}

// 開通閘門看轉帳能力：平台審核核准、帳戶存在但轉帳能力沒有啟用時不可曝光，缺的條件是 ONBOARDING_COMPLETE；啟用後可曝光，再變成未啟用又不可曝光。
// mutation：store_requirement_flags 的 onboarding_complete 拿掉 `p.transfers_active` → 轉帳能力未啟用的店家仍可曝光，失敗。
func TestExposureRequiresTheTransfersCapabilityOfThePaymentAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.approvedStore(t)
	f.logoed(t, accountID, s)
	for _, snap := range []PaymentSnapshot{
		{},
		{Transfers: PaymentCapabilityPending},
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
	if a := f.paymentAccount(t, s.StoreID); a.Status != PaymentEnabled || !a.TransfersActive || !a.PayoutsActive {
		t.Fatalf("帳戶=%+v", a)
	}
	f.setPayment(t, s.StoreID, needsSellerSnapshot)
	if got := f.exposed(t, s.StoreID); len(got) != 0 {
		t.Fatalf("轉帳能力又失效後仍可曝光：%v", got)
	}
	// 沒有帳戶列的已核准店家不通過閘門。
	accountID2, s2, sub2 := f.submitted(t, idB, "999888777666")
	_ = accountID2
	must(t, approve(f, sub2))
	_, err := f.db.Conn(ctx).Exec(ctx, "DELETE FROM seller.payment_accounts WHERE store_id = $1", s2.StoreID)
	must(t, err)
	if got := f.exposed(t, s2.StoreID); len(got) != 0 {
		t.Fatalf("沒有收款帳戶的店家可曝光：%v", got)
	}
}

// webhook inbox：同一個 event id 第二次不再排同步 job；對不到本地帳戶的事件只留紀錄；簽章不合的事件被拒絕、不留紀錄。
// mutation：payment_provider_events 拿掉 (provider, event_id) 的唯一約束並讓 insert 不去重 → 第二次也排 job，失敗。
func TestDuplicateProviderEventIsRecordedOnceAndQueuesNoSecondSync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s := f.approvedStore(t)
	external := f.ensureAccount(t, s.StoreID)
	before := len(f.jobs.queuedStoreIDs(t, kindPaymentSync))
	h, body := f.paymentEvent("evt_1", external)
	must(t, f.module.ReceivePaymentEvent(ctx, h, body))
	must(t, f.module.ReceivePaymentEvent(ctx, h, body))
	if got := len(f.jobs.queuedStoreIDs(t, kindPaymentSync)) - before; got != 1 {
		t.Fatalf("同一個事件排了 %d 個同步 job", got)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.payment_provider_events WHERE event_id = 'evt_1'"); n != 1 {
		t.Fatalf("inbox 列=%d", n)
	}
	h2, body2 := f.paymentEvent("evt_2", external)
	must(t, f.module.ReceivePaymentEvent(ctx, h2, body2))
	if got := len(f.jobs.queuedStoreIDs(t, kindPaymentSync)) - before; got != 2 {
		t.Fatalf("另一個事件共排了 %d 個同步 job", got)
	}
	h3, body3 := f.paymentEvent("evt_3", "fake_acct_unknown")
	must(t, f.module.ReceivePaymentEvent(ctx, h3, body3))
	if got := len(f.jobs.queuedStoreIDs(t, kindPaymentSync)) - before; got != 2 {
		t.Fatalf("對不到帳戶的事件排了 job：%d", got)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.payment_provider_events WHERE event_id = 'evt_3'"); n != 1 {
		t.Fatalf("對不到帳戶的事件沒有留紀錄")
	}
	err := f.module.ReceivePaymentEvent(ctx, http.Header{}, []byte(external))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("簽章不合的事件 err=%v", err)
	}
}

// 驗過簽章但不處理的事件（例如連線帳戶的 v1 事件、v2.core.account_person.*）：回 nil，不寫 inbox、不排 job。
// mutation：ReceiveEvent 把 ErrIgnoredEvent 當成 ErrInvalidEvent → 回 InvalidInput，失敗。
func TestIgnoredProviderEventWritesNoInboxRowAndQueuesNoSync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s := f.approvedStore(t)
	external := f.ensureAccount(t, s.StoreID)
	before := len(f.jobs.queuedStoreIDs(t, kindPaymentSync))
	h, body := f.paymentEvent("evt_ignored", external)
	h.Set(PaymentFakeIgnoredEventHeader, "1")
	must(t, f.module.ReceivePaymentEvent(ctx, h, body))
	if got := len(f.jobs.queuedStoreIDs(t, kindPaymentSync)) - before; got != 0 {
		t.Fatalf("不處理的事件排了 %d 個同步 job", got)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.payment_provider_events WHERE event_id = 'evt_ignored'"); n != 0 {
		t.Fatalf("不處理的事件寫了 inbox：%d", n)
	}
}

// 同步讀取期間到的事件不被這次重讀涵蓋：同步結果寫入後回 again（唯一鍵擋下了新 job，事件不能遺失）；沒有新事件時事件標記為已處理。
// mutation：MarkPaymentEventsProcessed 拿掉 received_at 條件 → 讀取期間到的事件被標成已處理，again 為 false，失敗。
func TestAnEventArrivingWhileTheAccountIsBeingReadMakesTheSyncRunAgain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s := f.approvedStore(t)
	external := f.ensureAccount(t, s.StoreID)
	f.payment.Set(external, enabledSnapshot)
	early, body := f.paymentEvent("evt_early", external)
	must(t, f.module.ReceivePaymentEvent(ctx, early, body))
	f.payment.OnFetch = func() {
		f.now = f.now.Add(time.Second)
		late, body := f.paymentEvent("evt_late", external)
		must(t, f.module.ReceivePaymentEvent(ctx, late, body))
	}
	again, err := f.module.SyncPaymentAccount(ctx, s.StoreID)
	must(t, err)
	if !again {
		t.Fatal("讀取期間到的事件沒有讓同步再跑一輪")
	}
	if n := f.count(t, "SELECT count(*) FROM seller.payment_provider_events WHERE processed_at IS NOT NULL"); n != 1 {
		t.Fatalf("已處理的事件=%d，只有讀取之前到的一筆", n)
	}
	f.now = f.now.Add(time.Second)
	again, err = f.module.SyncPaymentAccount(ctx, s.StoreID)
	must(t, err)
	if again {
		t.Fatal("第二輪涵蓋了全部事件，不該再跑")
	}
	if n := f.count(t, "SELECT count(*) FROM seller.payment_provider_events WHERE processed_at IS NULL"); n != 0 {
		t.Fatalf("未處理的事件=%d", n)
	}
}

// 輪詢補漏排 UNDER_REVIEW、ACTION_REQUIRED 的帳戶，以及 DISABLED 但原因碼不是終局（rejected*、unsupported*）的帳戶（Stripe 可以解除 restricted，掉了 webhook 時只有輪詢補得到）；
// 已啟用、終局的 DISABLED 與還沒建立的帳戶不排同步，閒置的「建立中」補排建立。
// mutation 1：PollablePaymentAccountStoreIDs 的條件改成排除 'DISABLED' 而不是列出狀態 → ENABLED 的帳戶也被排，失敗。
// mutation 2：拿掉 DISABLED 的子句 → 可解除的 DISABLED 帳戶沒被排，失敗。
func TestSweepQueuesSyncForOpenAndLiftableStatusesOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	byStatus := map[PaymentAccountStatus]int64{}
	for _, st := range PaymentAccountStatusValues() {
		_, s := f.open(t)
		byStatus[st] = s.StoreID
		external := "NULL"
		if st != PaymentNotCreated {
			external = fmt.Sprintf("'acct_%d'", s.StoreID)
		}
		enabled := st == PaymentEnabled
		_, err := f.db.Conn(ctx).Exec(ctx, fmt.Sprintf(`INSERT INTO seller.payment_accounts
			(store_id, provider, external_account_id, contact_email, requirement_collection, status, transfers_active, created_at, updated_at)
			VALUES ($1, 'fake', %s, 'seller@example.com', 'PROVIDER', $2, $3, $4, $4)`, external), s.StoreID, string(st), enabled, f.now.Add(-time.Hour))
		must(t, err)
	}
	// byStatus 的 DISABLED 沒有原因碼（可被輪詢）；再加三個 DISABLED：可解除的原因碼被輪詢，rejected、unsupported 開頭的終局原因碼不輪詢。
	disabledWith := map[string]int64{}
	for _, reason := range []string{"risk_under_review", "rejected_fraud", "unsupported_country"} {
		_, s := f.open(t)
		disabledWith[reason] = s.StoreID
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.payment_accounts
			(store_id, provider, external_account_id, contact_email, requirement_collection, status, status_reason, created_at, updated_at)
			VALUES ($1, 'fake', $2, 'seller@example.com', 'PROVIDER', 'DISABLED', $3, $4, $4)`, s.StoreID, fmt.Sprintf("acct_%d", s.StoreID), reason, f.now.Add(-time.Hour))
		must(t, err)
	}
	synced, ensured, err := f.module.SweepPaymentAccounts(ctx)
	must(t, err)
	want := []int64{byStatus[PaymentUnderReview], byStatus[PaymentActionRequired], byStatus[PaymentDisabled], disabledWith["risk_under_review"]}
	got := f.jobs.queuedStoreIDs(t, kindPaymentSync)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) || synced != 4 {
		t.Fatalf("輪詢排入的店家=%v（回報 %d），應該是 %v", got, synced, want)
	}
	if ensured != 1 || !slices.Equal(f.jobs.queuedStoreIDs(t, kindPaymentEnsure), []int64{byStatus[PaymentNotCreated]}) {
		t.Fatalf("補排的建立=%v（回報 %d）", f.jobs.queuedStoreIDs(t, kindPaymentEnsure), ensured)
	}
}

// 輪詢補漏也看 inbox：任何狀態（包含 ENABLED）的帳戶，有超過 1 分鐘還沒處理的事件就排同步；剛到的事件與已處理的事件不排。
// mutation：PollablePaymentAccountStoreIDs 拿掉 payment_provider_events 的 EXISTS 子句 → ENABLED 帳戶的舊事件沒有同步，失敗。
func TestSweepQueuesSyncForAnOldUnprocessedEventOnAnEnabledAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, old := f.approvedStore(t)
	_, fresh := f.approvedStore(t)
	_, done := f.approvedStore(t)
	externals := map[int64]string{}
	for _, s := range []Store{old, fresh, done} {
		externals[s.StoreID] = f.ensureAccount(t, s.StoreID)
		f.setPayment(t, s.StoreID, enabledSnapshot)
	}
	h, body := f.paymentEvent("evt_done", externals[done.StoreID])
	must(t, f.module.ReceivePaymentEvent(ctx, h, body))
	_, err := f.module.SyncPaymentAccount(ctx, done.StoreID)
	must(t, err)
	h, body = f.paymentEvent("evt_old", externals[old.StoreID])
	must(t, f.module.ReceivePaymentEvent(ctx, h, body))
	f.now = f.now.Add(2 * time.Minute)
	h, body = f.paymentEvent("evt_fresh", externals[fresh.StoreID])
	must(t, f.module.ReceivePaymentEvent(ctx, h, body))
	before := len(f.jobs.queuedStoreIDs(t, kindPaymentSync))
	synced, _, err := f.module.SweepPaymentAccounts(ctx)
	must(t, err)
	got := f.jobs.queuedStoreIDs(t, kindPaymentSync)[before:]
	if !slices.Equal(got, []int64{old.StoreID}) || synced != 1 {
		t.Fatalf("輪詢排入的店家=%v（回報 %d），應該只有舊事件的 %d", got, synced, old.StoreID)
	}
}

// 每家店的「建立中」列寫入時各自產生一個 UUID 冪等鍵：兩家店的鍵不同，各自得到自己的外部帳戶。
// mutation：create_idempotency_key 的預設值改成常數 → 兩家店共用一把鍵，外部帳戶只有一個，失敗。
func TestEachStoreGetsItsOwnCreateIdempotencyKey(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, a := f.approvedStore(t)
	_, b := f.approvedStore(t)
	ea, eb := f.ensureAccount(t, a.StoreID), f.ensureAccount(t, b.StoreID)
	keys := f.payment.EnsureKeys()
	if len(keys) != 2 || keys[0] == keys[1] || ea == eb {
		t.Fatalf("keys=%v accounts=%s %s", keys, ea, eb)
	}
}

// 建立重試用完（最後一次嘗試）或 provider 永久拒絕：記下建立失敗的時刻並進後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED，期限 24 小時；中途的暫時性失敗
// 只回傳錯誤、不記失敗。管理員重試建立清空失敗並重新排入，待辦消失；不是建立失敗的狀態不能重試。
// mutation：Ensure 在最後一次嘗試仍只回傳錯誤、不記 create_failed_at → 待辦沒有這一列，失敗。
func TestCreateFailureAfterTheLastAttemptQueuesAReviewItemAndRetryClearsIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s := f.approvedStore(t)
	boom := errors.New("provider 暫時無法使用")
	f.payment.FailEnsure(boom, boom)
	if err := f.module.EnsurePaymentAccount(ctx, s.StoreID, false); !errors.Is(err, boom) {
		t.Fatalf("中途的失敗 err=%v", err)
	}
	if a := f.paymentAccount(t, s.StoreID); a.CreateFailedAt != nil {
		t.Fatal("中途的暫時性失敗就記成建立失敗")
	}
	if rows := f.reviewOf(t, "review_payment_account_create_failed", f.now); len(rows) != 0 {
		t.Fatalf("待辦=%v", rows)
	}
	must(t, f.module.EnsurePaymentAccount(ctx, s.StoreID, true))
	if keys := f.payment.EnsureKeys(); len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("重試沒有沿用同一把冪等鍵：%v", keys)
	}
	a := f.paymentAccount(t, s.StoreID)
	if a.CreateFailedAt == nil || !a.CreateFailedAt.Equal(f.now) || a.ExternalAccountID != nil || a.Status != PaymentNotCreated {
		t.Fatalf("最後一次嘗試後=%+v", a)
	}
	rows := f.reviewOf(t, "review_payment_account_create_failed", f.now)
	if len(rows) != 1 || rows[0].Kind != "PAYMENT_ACCOUNT_CREATE_FAILED" || rows[0].ItemID != fmt.Sprint(s.StoreID) || rows[0].StoreID == nil || *rows[0].StoreID != s.StoreID ||
		rows[0].Start == nil || !rows[0].Start.Equal(f.now) || rows[0].Due == nil || !rows[0].Due.Equal(f.now.Add(24*time.Hour)) {
		t.Fatalf("待辦=%+v", rows)
	}
	var inQueue int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM seller.pub_review_items($1) WHERE kind = 'PAYMENT_ACCOUNT_CREATE_FAILED'", f.now).Scan(&inQueue))
	if inQueue != 1 {
		t.Fatalf("pub_review_items 的列=%d", inQueue)
	}
	// 已進入失敗的列，建立 job 不再自動重跑。
	must(t, f.module.EnsurePaymentAccount(ctx, s.StoreID, false))
	if ensures, _, _ := f.payment.Calls(); ensures != 2 {
		t.Fatalf("失敗之後 job 又呼叫了 provider：%d 次", ensures)
	}
	retried, err := f.module.RetryPaymentAccountCreate(ctx, s.StoreID)
	must(t, err)
	if retried.CreateFailedAt != nil {
		t.Fatalf("重試後仍標示失敗：%+v", retried)
	}
	if got := f.jobs.queuedStoreIDs(t, kindPaymentEnsure); len(got) != 2 {
		t.Fatalf("重試排入的建立 job=%v", got)
	}
	if rows := f.reviewOf(t, "review_payment_account_create_failed", f.now); len(rows) != 0 {
		t.Fatalf("重試後待辦仍在：%v", rows)
	}
	if _, err := f.module.RetryPaymentAccountCreate(ctx, s.StoreID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("不是建立失敗的狀態重試 err=%v", err)
	}
	if _, err := f.module.RetryPaymentAccountCreate(ctx, s.StoreID+1000); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有帳戶的店家重試 err=%v", err)
	}
	// provider 永久拒絕：不等最後一次嘗試，立刻記成建立失敗；管理員再重試一次就建立成功。
	f.payment.FailEnsure(ErrPaymentPermanent)
	must(t, f.module.EnsurePaymentAccount(ctx, s.StoreID, false))
	if a := f.paymentAccount(t, s.StoreID); a.CreateFailedAt == nil {
		t.Fatalf("永久失敗沒有記成建立失敗：%+v", a)
	}
	_, err = f.module.RetryPaymentAccountCreate(ctx, s.StoreID)
	must(t, err)
	f.ensureAccount(t, s.StoreID)
}

// 開通 session：外部帳戶建立之前回 failed_precondition，建立之後回 provider 的 session；被停用的帳戶不給 session。
func TestOnboardingSessionNeedsAnExternalAccountThatIsNotDisabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s := f.approvedStore(t)
	if _, err := f.module.CreatePaymentOnboardingSession(ctx, s.StoreID, "https://web.example.test/ok", "https://web.example.test/again"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("還沒有外部帳戶 err=%v", err)
	}
	external := f.ensureAccount(t, s.StoreID)
	session, err := f.module.CreatePaymentOnboardingSession(ctx, s.StoreID, "https://web.example.test/ok", "https://web.example.test/again")
	must(t, err)
	if session.Surface != SurfaceHosted || session.URL == "" || session.ClientSecret != "" {
		t.Fatalf("session=%+v", session)
	}
	f.payment.Set(external, rejectedSnapshot)
	_, err = f.module.SyncPaymentAccount(ctx, s.StoreID)
	must(t, err)
	if a := f.paymentAccount(t, s.StoreID); a.Status != PaymentDisabled || a.StatusReason == nil || *a.StatusReason != "rejected.fraud" {
		t.Fatalf("帳戶=%+v", a)
	}
	if _, err := f.module.CreatePaymentOnboardingSession(ctx, s.StoreID, "https://web.example.test/ok", "https://web.example.test/again"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被停用的帳戶 err=%v", err)
	}
	if v := f.payment.Violations(); len(v) != 0 {
		t.Fatalf("違反呼叫規則：%v", v)
	}
}
