//go:build integration

package seller_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river/rivertype"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

// fakePayUni 是 PayUni 的替身：記下每次呼叫，依序回傳預先排好的錯誤（排完就成功），並確認每次呼叫都在 transaction 外、
// 而且有 deadline（規則 5）。
type fakePayUni struct {
	t         *testing.T
	mu        sync.Mutex
	binds     []pu.BindMerchantRequest
	updates   []pu.UpdateMerchantRequest
	statuses  []string
	bindErr   []error
	updateErr []error
	statusErr []error
	status    pu.MerchantStatus
	// onBind 在 bind 呼叫進行中執行，用來製造「呼叫期間有人改了資料」的交錯。
	onBind func()
}

func (p *fakePayUni) check(ctx context.Context) {
	p.t.Helper()
	if _, ok := db.TxFrom(ctx); ok {
		p.t.Error("PayUni 在 DB transaction 內被呼叫")
	}
	if _, ok := ctx.Deadline(); !ok {
		p.t.Error("PayUni 的呼叫沒有 deadline")
	}
}

func pop(errs *[]error) error {
	if len(*errs) == 0 {
		return nil
	}
	err := (*errs)[0]
	*errs = (*errs)[1:]
	return err
}

func (p *fakePayUni) BindMerchant(ctx context.Context, req pu.BindMerchantRequest) error {
	p.check(ctx)
	p.mu.Lock()
	p.binds = append(p.binds, req)
	err := pop(&p.bindErr)
	hook := p.onBind
	p.onBind = nil
	p.mu.Unlock()
	if hook != nil {
		hook()
	}
	return err
}

func (p *fakePayUni) UpdateMerchant(ctx context.Context, req pu.UpdateMerchantRequest) error {
	p.check(ctx)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.updates = append(p.updates, req)
	return pop(&p.updateErr)
}

func (p *fakePayUni) MerchantStatus(ctx context.Context, merID string) (pu.MerchantStatus, error) {
	p.check(ctx)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.statuses = append(p.statuses, merID)
	if err := pop(&p.statusErr); err != nil {
		return pu.MerchantStatus{}, err
	}
	st := p.status
	st.MerID = merID
	return st, nil
}

func (p *fakePayUni) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.binds) + len(p.updates) + len(p.statuses)
}

func apiErr(code string) error { return &pu.APIError{Code: code, Message: "測試用的說明"} }

var errNetwork = errors.New("connection reset by peer")

// lastObserve 取出最近排入的 webhook job 的參數（模擬 worker 取到它）。
func (q *queue) lastObserve(t *testing.T) PayUniObserveArgs {
	t.Helper()
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := len(q.args) - 1; i >= 0; i-- {
		if a, ok := q.args[i].(PayUniObserveArgs); ok {
			return a
		}
	}
	t.Fatal("沒有排入 webhook job")
	return PayUniObserveArgs{}
}

func (q *queue) count(kind string) int {
	n := 0
	for _, k := range q.kinds() {
		if k == kind {
			n++
		}
	}
	return n
}

const (
	kindSync    = "seller.payuni_sync"
	kindPoll    = "seller.payuni_poll"
	kindObserve = "seller.payuni_observe"
)

// 以台北時間表示的 ExecuteTime（f.now 是 2026-10-01 04:00 UTC，也就是台北 12:00）。
const (
	t1300 = "20261001130000"
	t1400 = "20261001140000"
	t1500 = "20261001150000"
)

func notification(merID, merStatus, execute string) PayUniNotification {
	return PayUniNotification{MerID: merID, EventKey: merID + ":" + merStatus + ":" + execute, MerStatus: merStatus, ExecuteTime: execute}
}

// deliver 走 consumer 的路徑：接收（排入 job）再由 worker 套用。
func (f *fixture) deliver(t *testing.T, n PayUniNotification) error {
	t.Helper()
	must(t, f.module.ReceivePayUniNotification(context.Background(), n))
	return f.module.ApplyPayUniNotification(context.Background(), f.jobs.lastObserve(t))
}

func (f *fixture) approvedStore(t *testing.T) (int64, Store) {
	t.Helper()
	accountID, s, sub := f.submitted(t, idA, bankAcct)
	must(t, approve(f, sub))
	return accountID, s
}

func (f *fixture) registration(t *testing.T, storeID int64) PayUniRegistration {
	t.Helper()
	r, err := f.module.PayUniRegistrationOf(context.Background(), storeID)
	must(t, err)
	return r.PayUniRegistration
}

func (f *fixture) sync(t *testing.T, storeID int64) bool {
	t.Helper()
	again, err := f.module.SyncPayUni(context.Background(), storeID, false)
	must(t, err)
	return again
}

// boundStore 核准並完成綁定（fake 回成功）。
func (f *fixture) boundStore(t *testing.T) (int64, Store) {
	t.Helper()
	accountID, s := f.approvedStore(t)
	f.sync(t, s.StoreID)
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusBound {
		t.Fatalf("registration=%+v", r)
	}
	return accountID, s
}

func setAddress(f *fixture, t *testing.T, storeID int64, detail string) Store {
	t.Helper()
	cur, err := f.module.Store(context.Background(), storeID)
	must(t, err)
	s, err := f.module.UpdateProfile(context.Background(), UpdateProfileInput{StoreID: storeID, ExpectedVersion: cur.Version, AddressDetail: ptr(detail)})
	must(t, err)
	return s
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func TestApproveWritesTheIntentAndEnqueuesWithoutCallingPayUni(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedStore(t)
	// D4、D5：核准在 transaction 內只寫意圖與 job，沒有任何對 PayUni 的呼叫。
	if f.payuni.calls() != 0 {
		t.Fatalf("approve 呼叫了 PayUni：%+v", f.payuni)
	}
	r := f.registration(t, s.StoreID)
	if r.BindStatus != BindStatusPending || r.MerID != FormatPayUniMerID(s.StoreCode) || r.ApplicationID.String() == "" || !r.SyncPending ||
		r.Status.Outcome != PayUniOutcomeNone || r.PaymentEnabled {
		t.Fatalf("registration=%+v", r)
	}
	if f.jobs.count(kindSync) != 1 {
		t.Fatalf("jobs=%v", f.jobs.kinds())
	}
	// job 以 store_id 為唯一鍵（含排隊中、執行中、重試中；不含已完成的）。
	for i, args := range f.jobs.args {
		if args.Kind() != kindSync {
			continue
		}
		if b, _ := json.Marshal(args); string(b) != fmt.Sprintf(`{"store_id":%d}`, s.StoreID) {
			t.Fatalf("args=%s", b)
		}
		opts := f.jobs.opts[i]
		if opts == nil || !opts.UniqueOpts.ByArgs || slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateCompleted) || !slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateRunning) || opts.MaxAttempts == 0 {
			t.Fatalf("opts=%+v", opts)
		}
	}
	// 意圖與核准在同一個 transaction：核准失敗（事件的訂閱者失敗）時，意圖也不存在。
	f2 := setup(t)
	_, s2, sub := f2.submitted(t, idB, bankAcct)
	boom := errors.New("boom")
	subscribeFail[KYCApproved](f2, boom)
	if err := approve(f2, sub); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if n := f2.count(t, "SELECT count(*) FROM seller.payuni_registrations WHERE store_id=$1", s2.StoreID); n != 0 {
		t.Fatalf("registrations=%d", n)
	}
}

// 不推斷重送：核准、退回生效後帶舊版本再送一次一律 VersionConflict（先於狀態檢查），不重複寫審核紀錄、不重發事件、不另排 job、不動意圖。
func TestStaleEtagAfterApproveAndRejectConflicts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	approved, rejected := record[KYCApproved](f), record[KYCRejected](f)
	_, _, sub := f.submitted(t, idA, bankAcct)
	first, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9})
	must(t, err)
	for _, v := range []int64{sub.Version, sub.Version - 1} {
		if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: v, AdminID: 9}); apperr.KindOf(err) != apperr.KindVersionConflict {
			t.Fatalf("approve version %d: %v", v, err)
		}
	}
	if _, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "資料不符"}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("reject after approve: %v", err)
	}
	// 帶目前的版本再核准：申請已不是審核中。
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: first.Version, AdminID: 9}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("approve at the current version: %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action='APPROVE'"); n != 1 || len(approved.all()) != 1 || f.jobs.count(kindSync) != 1 {
		t.Fatalf("reviews=%d events=%d jobs=%v", n, len(approved.all()), f.jobs.kinds())
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_applications WHERE application_id=$1 AND version=$2 AND status='APPROVED'", sub.ID, first.Version); n != 1 {
		t.Fatalf("application changed after the stale requests")
	}

	// 退回生效後帶退回之前的版本：衝突，審核紀錄與事件各只有一筆。
	_, _, sub2 := f.submitted(t, idB, "012345678902")
	done, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub2.ID, ExpectedVersion: sub2.Version, AdminID: 9, Issues: anIssue(), Reason: "資料不符"})
	must(t, err)
	if _, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub2.ID, ExpectedVersion: sub2.Version, AdminID: 9, Issues: anIssue(), Reason: "資料不符"}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("reject retry: %v", err)
	}
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub2.ID, ExpectedVersion: sub2.Version, AdminID: 9}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("approve after reject: %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action='REJECT'"); n != 1 || len(rejected.all()) != 1 || f.count(t, "SELECT count(*) FROM seller.kyc_applications WHERE application_id=$1 AND version=$2 AND status='REJECTED'", sub2.ID, done.Version) != 1 {
		t.Fatalf("reviews=%d events=%d", n, len(rejected.all()))
	}
}

func subscribeFail[E any](f *fixture, err error) {
	events.Subscribe(f.bus, func(context.Context, E) error { return err })
}

func TestBindSendsTheCurrentDataAndRecordsTheForwardedEvent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	forwarded := record[PayUniForwarded](f)
	accountID, s := f.approvedStore(t)
	if again := f.sync(t, s.StoreID); again {
		t.Fatal("綁定後不該再排一輪")
	}
	town, _ := twaddr.TownByCode("63000050")
	if len(f.payuni.binds) != 1 {
		t.Fatalf("binds=%d", len(f.payuni.binds))
	}
	req := f.payuni.binds[0]
	want := pu.BindMerchantRequest{
		Name: "王小明", UBN: idA, Email: "owner@example.com", MerID: FormatPayUniMerID(s.StoreCode), MerEmail: "owner@example.com",
		MerName: "範例小農", MerEngName: "Wang Farm", MerURL: fmt.Sprintf("https://store.example.invalid/sellers/%d", s.StoreCode),
		MerAddr: town.CountyName + town.Name + "測試路一號", MerAddrPost: town.PostalCode, MerDesc: "蔬果",
		ContactName: "王小明", ContactMobile: "0912345678", MerTel: "0912345678", GoodsRate: "100,0,0,0", Payment: "CREDIT,ApplePay,GooglePay,ATM",
		MerType: 2, MerMCC: 3, DeliverTime: 7, EscrowType: 1, MerUnitPrice: 1000, SellType: 1,
	}
	if req != want {
		t.Fatalf("req=%+v\nwant=%+v", req, want)
	}
	r := f.registration(t, s.StoreID)
	if r.BindStatus != BindStatusBound || r.BoundAt == nil || r.BindErrorCode != nil || r.BindAttempts != 1 || r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
	obs := f.observationPages(t, s.StoreID)
	if len(obs) != 1 || obs[0].Source != PayUniSourceBind || obs[0].MerStatus != nil {
		t.Fatalf("observations=%+v", obs)
	}
	// 事件帶穩定的 UUIDv7：等於寫進去的那筆觀測紀錄的 ID（relay 重送時不變）。
	ev := forwarded.all()
	if len(ev) != 1 || ev[0].EventID != obs[0].ID || ev[0].EventID[6]>>4 != 7 || ev[0].AccountID != accountID || ev[0].StoreID != s.StoreID {
		t.Fatalf("forwarded=%+v", ev)
	}
	// 沒有排到的 job 時重跑一輪不會再呼叫 PayUni。
	f.sync(t, s.StoreID)
	if len(f.payuni.binds) != 1 || len(forwarded.all()) != 1 {
		t.Fatal("已綁定的註冊又呼叫了 PayUni")
	}
}

// 回歸（盤點 D1）：舊實作的 raw_request 存明文身分證號（連同姓名、電話、地址）。新的註冊列與觀測紀錄沒有 raw 欄位，
// 任何一欄都找不到身分證號或銀行帳號。
func TestPayUniTablesNeverStoreTheRequestOrTheIdentity(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.boundStore(t)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourcePoll))
	for _, table := range []string{"payuni_registrations", "payuni_observations"} {
		if n := f.count(t, "SELECT count(*) FROM information_schema.columns WHERE table_schema='seller' AND table_name=$1 AND column_name ~* 'raw|request|body|payload'", table); n != 0 {
			t.Errorf("%s 有 raw 欄位", table)
		}
		if n := f.count(t, fmt.Sprintf("SELECT count(*) FROM seller.%s t WHERE to_jsonb(t)::text LIKE ANY($1)", table), []string{"%" + idA + "%", "%" + bankAcct + "%", "%345678901%"}); n != 0 {
			t.Errorf("%s 含身分證號或銀行帳號", table)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.payuni_observations") != 2 {
		t.Fatal("observations")
	}
}

func TestBindMapsACompanyApplication(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	in := companyDraft(accountID, idRep, taxIDOne, bankAcct)
	in.CompanyPhone = ptr("02-2901-0572")
	d := f.savedDraft(t, in)
	sub, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	must(t, approve(f, sub))
	f.sync(t, s.StoreID)
	req := f.payuni.binds[0]
	if req.Name != "範例小農有限公司" || req.UBN != taxIDOne || req.MCID != idRep || req.MCName != "王小明" || req.MBCName != "王小明" ||
		req.MBCTel != "0229010572" || req.MBCMail != "owner@example.com" || req.ContactTel != "0229010572" {
		t.Fatalf("req=%+v", req)
	}
}

// 回歸（盤點 D7）：舊實作失敗的狀態與審核紀錄的寫入失敗只 warn。綁定失敗的狀態與它的事件在同一個 transaction：
// 事件寫不出去時整筆回滾，狀態不會單獨變成 FAILED。
func TestBindFailureIsRecordedAtomicallyWithItsEvent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	failed := record[PayUniBindFailed](f)
	_, s := f.approvedStore(t)
	f.payuni.bindErr = []error{apiErr("AGENT03003")}
	f.sync(t, s.StoreID)
	r := f.registration(t, s.StoreID)
	if r.BindStatus != BindStatusFailed || deref(r.BindErrorCode) != "AGENT03003" || r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
	ev := failed.all()
	if len(ev) != 1 || ev[0].ErrorCode != "AGENT03003" || ev[0].StoreID != s.StoreID {
		t.Fatalf("events=%+v", ev)
	}
	// 失敗的註冊不會再被 job 碰（要管理員重新綁定）：不呼叫 PayUni。
	f.sync(t, s.StoreID)
	if len(f.payuni.binds) != 1 {
		t.Fatalf("binds=%d", len(f.payuni.binds))
	}

	// 事件的訂閱者失敗：整個結果回滾，註冊仍是 PENDING（沒有單獨的 FAILED），租約放掉讓重試可以接手。
	g := setup(t)
	_, s2 := g.approvedStore(t)
	boom := errors.New("subscriber down")
	subscribeFail[PayUniBindFailed](g, boom)
	g.payuni.bindErr = []error{apiErr("AGENT03003")}
	if _, err := g.module.SyncPayUni(context.Background(), s2.StoreID, false); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	r2 := g.registration(t, s2.StoreID)
	if r2.BindStatus != BindStatusPending || r2.BindErrorCode != nil {
		t.Fatalf("registration=%+v", r2)
	}
}

// 回歸（盤點 D16）：舊 client 把傳輸錯誤轉成 Success:false（err 為 nil、沒有錯誤碼），被當成業務錯誤。新的流程：
// 傳輸錯誤是錯誤，job 重試，註冊不會變成 FAILED；最後一次嘗試仍失敗才記為 FAILED（PAYUNI_UNAVAILABLE）。
func TestTransportErrorIsRetriedAndOnlyTheFinalAttemptFails(t *testing.T) {
	t.Parallel()
	f := setup(t)
	failed := record[PayUniBindFailed](f)
	_, s := f.approvedStore(t)
	f.payuni.bindErr = []error{errNetwork, errNetwork}
	if _, err := f.module.SyncPayUni(context.Background(), s.StoreID, false); !errors.Is(err, errNetwork) {
		t.Fatalf("err=%v", err)
	}
	r := f.registration(t, s.StoreID)
	if r.BindStatus != BindStatusPending || r.BindAttempts != 1 || len(failed.all()) != 0 {
		t.Fatalf("registration=%+v", r)
	}
	// PayUni 的暫時性錯誤碼同樣重試。
	f.payuni.bindErr = []error{apiErr("AGENT03021")}
	if _, err := f.module.SyncPayUni(context.Background(), s.StoreID, false); err == nil {
		t.Fatal("暫時性的錯誤碼必須重試")
	}
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusPending || r.BindAttempts != 2 {
		t.Fatalf("registration=%+v", r)
	}
	// 最後一次嘗試：記錄結果，不再重試。
	f.payuni.bindErr = []error{errNetwork}
	if _, err := f.module.SyncPayUni(context.Background(), s.StoreID, true); err != nil {
		t.Fatalf("最後一次嘗試應該記錄結果而不是再回錯誤：%v", err)
	}
	r = f.registration(t, s.StoreID)
	if r.BindStatus != BindStatusFailed || deref(r.BindErrorCode) != PayUniErrUnavailable || len(failed.all()) != 1 || r.BindAttempts != 3 {
		t.Fatalf("registration=%+v", r)
	}
	// 管理員重新綁定之後可以成功。
	if _, err := f.module.RebindPayUni(context.Background(), RebindPayUniInput{StoreID: s.StoreID, AdminID: 9}); err != nil {
		t.Fatal(err)
	}
	f.sync(t, s.StoreID)
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusBound || r.BindAttempts != 1 {
		t.Fatalf("registration=%+v", r)
	}
}

// 回歸（盤點 D15）：舊 client 在地址空白時送假資料（地址「台灣」、郵遞區號 100）。缺地址就記為資料不完整，不呼叫
// PayUni；店家補上地址之後自動回到 PENDING 重送，送的是真實的地址。
func TestMissingAddressFailsWithoutSendingFakeValues(t *testing.T) {
	t.Parallel()
	f := setup(t)
	failed := record[PayUniBindFailed](f)
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	sub, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{TownCode: ptr(""), AddressDetail: ptr("")}); err != nil {
		t.Fatal(err)
	}
	must(t, approve(f, sub))
	f.sync(t, s.StoreID)
	if len(f.payuni.binds) != 0 {
		t.Fatalf("缺地址仍然呼叫了 PayUni：%+v", f.payuni.binds)
	}
	r := f.registration(t, s.StoreID)
	if r.BindStatus != BindStatusFailed || deref(r.BindErrorCode) != PayUniErrIncompleteData || len(failed.all()) != 1 {
		t.Fatalf("registration=%+v", r)
	}
	mine, err := f.module.MyKYC(context.Background(), accountID)
	must(t, err)
	if mine.Flow == nil || mine.Flow.Status != FlowBindFailed || mine.Flow.CanResubmit || mine.PayUni == nil ||
		mine.PayUni.State != PayUniState("FIX_VIA_SUPPORT") || !slices.Equal(mine.PayUni.FixStoreFields, []string{"address"}) {
		t.Fatalf("flow=%+v payuni=%+v", mine.Flow, mine.PayUni)
	}
	// 問題只指向店家現在實際缺的欄位（只有地址，聯絡資料是齊的）；補上之後的註冊不再指向它。
	if p := PayUniProblemOf(r); p == nil || !slices.Equal(p.StoreFields, []string{"address"}) {
		t.Fatalf("problem=%+v", p)
	}
	// 補上地址：資料不完整造成的失敗自動回到 PENDING 並排入 job。
	before := f.jobs.count(kindSync)
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{TownCode: ptr("63000050"), AddressDetail: ptr("新路二號")}); err != nil {
		t.Fatal(err)
	}
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusPending || f.jobs.count(kindSync) != before+1 {
		t.Fatalf("registration=%+v jobs=%d", r, f.jobs.count(kindSync))
	}
	f.sync(t, s.StoreID)
	town, _ := twaddr.TownByCode("63000050")
	if len(f.payuni.binds) != 1 || f.payuni.binds[0].MerAddr != town.CountyName+town.Name+"新路二號" || f.payuni.binds[0].MerAddrPost != town.PostalCode {
		t.Fatalf("binds=%+v", f.payuni.binds)
	}
}

// 回歸（盤點 D18）：舊實作只在註冊 active 時排地址同步，PENDING 期間改的地址永遠送不出去（bind 用的是舊地址）。
// 新的 job 讀執行當下的資料：PENDING 期間改地址，bind 送的是新地址；綁定之後改地址走同一個 job 的 update。
func TestAddressChangeDuringPendingAndAfterBindingIsSynced(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedStore(t)
	before := f.jobs.count(kindSync)
	setAddress(f, t, s.StoreID, "改過的路三號")
	if f.jobs.count(kindSync) != before+1 {
		t.Fatalf("改地址沒有排入 sync job：%v", f.jobs.kinds())
	}
	if r := f.registration(t, s.StoreID); !r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
	f.sync(t, s.StoreID)
	if len(f.payuni.binds) != 1 || !strings.HasSuffix(f.payuni.binds[0].MerAddr, "改過的路三號") {
		t.Fatalf("bind 沒有送目前的地址：%+v", f.payuni.binds)
	}
	if r := f.registration(t, s.StoreID); r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
	// 綁定之後：連續兩次改地址合併成一次 update，送最後的值。
	setAddress(f, t, s.StoreID, "第四路四號")
	setAddress(f, t, s.StoreID, "第五路五號")
	f.sync(t, s.StoreID)
	if len(f.payuni.updates) != 1 || !strings.HasSuffix(f.payuni.updates[0].MerAddr, "第五路五號") || f.payuni.updates[0].MerID != FormatPayUniMerID(s.StoreCode) ||
		f.payuni.updates[0].ContactMobile != "0912345678" {
		t.Fatalf("updates=%+v", f.payuni.updates)
	}
	f.sync(t, s.StoreID)
	if len(f.payuni.updates) != 1 {
		t.Fatal("沒有新資料時不該再呼叫 PayUni")
	}
}

// 呼叫 PayUni 期間店家改了地址：結果寫入後 intent_seq 比讀到的大，job 再跑一輪（again），送新的地址；不靠重新排入
// （唯一鍵會把它跟執行中的 job 合併）。
func TestAddressChangeDuringTheCallRunsAnotherRound(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedStore(t)
	f.payuni.onBind = func() { setAddress(f, t, s.StoreID, "呼叫中改的路") }
	if again := f.sync(t, s.StoreID); !again {
		t.Fatal("呼叫期間有新資料，job 應該再跑一輪")
	}
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusBound || !r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
	if again := f.sync(t, s.StoreID); again {
		t.Fatal("第二輪之後不該再跑")
	}
	if len(f.payuni.updates) != 1 || !strings.HasSuffix(f.payuni.updates[0].MerAddr, "呼叫中改的路") {
		t.Fatalf("updates=%+v", f.payuni.updates)
	}
}

// 呼叫 PayUni 期間註冊列被重建（開發種子：intent_seq 延續加一、synced_seq 記到新值）：舊 intent 的在途 job 結束時不寫入，
// 重建的列不會被記成綁定失敗、也不會退回舊的結果；job 也不再跑一輪。
func TestStaleJobDoesNotOverwriteARebuiltRegistration(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedStore(t)
	f.payuni.bindErr = []error{apiErr("AGENT03003")} // 沒有 compare-and-set 時，這會把重建的列寫成 FAILED
	f.payuni.onBind = func() {
		_, err := f.db.Conn(context.Background()).Exec(context.Background(),
			"UPDATE seller.payuni_registrations SET bind_status='BOUND', bound_at=now(), intent_seq=intent_seq+1, synced_seq=intent_seq+1 WHERE store_id=$1", s.StoreID)
		must(t, err)
	}
	failed := record[PayUniBindFailed](f)
	if again := f.sync(t, s.StoreID); again {
		t.Fatal("舊 intent 的 job 不該再跑一輪")
	}
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusBound || r.BindErrorCode != nil || r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
	if n := len(failed.all()); n != 0 {
		t.Fatalf("PayUniBindFailed events=%d", n)
	}
}

// AGENT04017（MerID 已存在）：先查詢狀態認領，結果不明（逾時）之後重送一律安全。
func TestAlreadyBoundMerchantIsClaimedBySelfHealing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	forwarded, approvedEvents := record[PayUniForwarded](f), record[PayUniApproved](f)
	_, s := f.approvedStore(t)
	f.payuni.bindErr = []error{apiErr("AGENT04017")}
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1", Credit: "1"}
	f.sync(t, s.StoreID)
	r := f.registration(t, s.StoreID)
	if r.BindStatus != BindStatusBound || r.Status.Outcome != PayUniOutcomeApproved || !r.PaymentEnabled || len(f.payuni.statuses) != 1 {
		t.Fatalf("registration=%+v statuses=%v", r, f.payuni.statuses)
	}
	if len(forwarded.all()) != 1 || len(approvedEvents.all()) != 1 {
		t.Fatalf("forwarded=%d approved=%d", len(forwarded.all()), len(approvedEvents.all()))
	}
	obs := f.observationPages(t, s.StoreID)
	if len(obs) != 2 || approvedEvents.all()[0].EventID == forwarded.all()[0].EventID {
		t.Fatalf("observations=%+v", obs)
	}
}

// 商家層級的終止狀態（退件、關閉）：同一個 MerID 無法再綁定，記為失敗（MERCHANT_TERMINAL）、不能重新綁定，要關店重新入駐。
func TestAlreadyBoundTerminalMerchantNeedsReonboarding(t *testing.T) {
	t.Parallel()
	f := setup(t)
	rejected := record[PayUniRejected](f)
	accountID, s := f.approvedStore(t)
	f.payuni.bindErr = []error{apiErr("AGENT04017")}
	f.payuni.status = pu.MerchantStatus{MerStatus: "92", Reason: "文件不符"}
	f.sync(t, s.StoreID)
	r := f.registration(t, s.StoreID)
	if r.BindStatus != BindStatusFailed || deref(r.BindErrorCode) != PayUniErrMerchantTerminal || !r.NeedsReonboarding || r.Status.Outcome != PayUniOutcomeRejected {
		t.Fatalf("registration=%+v", r)
	}
	if ev := rejected.all(); len(ev) != 1 || ev[0].Reason != "文件不符" {
		t.Fatalf("rejected=%+v", ev)
	}
	if _, err := f.module.RebindPayUni(context.Background(), RebindPayUniInput{StoreID: s.StoreID, AdminID: 9}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("rebind: %v", err)
	}
	mine, err := f.module.MyKYC(context.Background(), accountID)
	must(t, err)
	if mine.Flow.Status != FlowBindFailed || mine.Flow.CanResubmit || mine.PayUni == nil || mine.PayUni.State != PayUniState("TERMINATED") {
		t.Fatalf("flow=%+v payuni=%+v", mine.Flow, mine.PayUni)
	}
}

func TestRebindOnlyFromFailed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedStore(t)
	rebind := func() (AdminPayUniRegistration, error) {
		return f.module.RebindPayUni(context.Background(), RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
	}
	// PENDING：已經在（重新）綁定，重送安全、不另排 job。
	jobs := f.jobs.count(kindSync)
	if r, err := rebind(); err != nil || r.BindStatus != BindStatusPending || f.jobs.count(kindSync) != jobs {
		t.Fatalf("pending rebind: %+v %v", r, err)
	} else if len(r.RebindUnavailable) != 1 || r.RebindUnavailable[0] != StoreReasonPayUniBindInProgress {
		// 綁定中：可用性的第一個原因是 PAYUNI_BIND_IN_PROGRESS（idempotent），與方法回目前註冊的分支是同一個判斷。
		t.Fatalf("pending reasons: %v", r.RebindUnavailable)
	}
	f.payuni.bindErr = []error{apiErr("AGENT04003")}
	f.sync(t, s.StoreID)
	if f.registration(t, s.StoreID).BindStatus != BindStatusFailed {
		t.Fatal("not failed")
	}
	if _, err := f.module.RebindPayUni(context.Background(), RebindPayUniInput{StoreID: 0, AdminID: 9}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("invalid: %v", err)
	}
	r, err := rebind()
	must(t, err)
	if r.BindStatus != BindStatusPending || r.BindErrorCode != nil || r.BindAttempts != 0 || f.jobs.count(kindSync) != jobs+1 {
		t.Fatalf("rebind: %+v jobs=%d", r, f.jobs.count(kindSync))
	}
	f.sync(t, s.StoreID)
	if _, err := rebind(); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("rebind on a bound registration: %v", err)
	}
	if _, err := f.module.RebindPayUni(context.Background(), RebindPayUniInput{StoreID: s.StoreID + 100, AdminID: 9}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("unknown store: %v", err)
	}
}

func TestNotConfiguredPayUniFailsWithAClearCodeAndDoesNotPoll(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.module = f.newModule(t, nil, f.jobs)
	failed := record[PayUniBindFailed](f)
	_, s := f.approvedStore(t)
	f.sync(t, s.StoreID)
	r := f.registration(t, s.StoreID)
	if r.BindStatus != BindStatusFailed || deref(r.BindErrorCode) != PayUniErrNotConfigured || len(failed.all()) != 1 || f.payuni.calls() != 0 {
		t.Fatalf("registration=%+v", r)
	}
	if err := f.module.RefreshPayUniStatus(context.Background(), s.StoreID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("refresh: %v", err)
	}
	if _, polled, err := f.module.SweepPayUni(context.Background()); err != nil || polled != 0 {
		t.Fatalf("sweep: %d %v", polled, err)
	}
	// 設定好之後由管理員重新綁定。
	f.module = f.newModule(t, f.payuni, f.jobs)
	_, err := f.module.RebindPayUni(context.Background(), RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
	must(t, err)
	f.sync(t, s.StoreID)
	if f.registration(t, s.StoreID).BindStatus != BindStatusBound {
		t.Fatal("not bound")
	}
}

func TestClosedStoreIsNotSynced(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.approvedStore(t)
	_, err := f.module.CloseCurrent(context.Background(), accountID)
	must(t, err)
	f.sync(t, s.StoreID)
	if f.payuni.calls() != 0 {
		t.Fatalf("已關閉的店家呼叫了 PayUni")
	}
}

func TestDuplicateWebhookIsAppliedOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	pending := record[PayUniPendingInfo](f)
	_, s := f.boundStore(t)
	n := notification(FormatPayUniMerID(s.StoreCode), "91", t1300)
	n.MerStatusNote = "營業登記證影本模糊"
	must(t, f.deliver(t, n))
	must(t, f.deliver(t, n)) // PayUni 重送同一則
	if ev := pending.all(); len(ev) != 1 || ev[0].Reason != "營業登記證影本模糊" {
		t.Fatalf("pending_info=%+v", ev)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.payuni_observations WHERE source='WEBHOOK'"); got != 1 {
		t.Fatalf("webhook observations=%d", got)
	}
	r := f.registration(t, s.StoreID)
	if r.Status.Outcome != PayUniOutcomePendingInfo || deref(r.Status.MerStatus) != "91" || deref(r.Status.Reason) != "營業登記證影本模糊" {
		t.Fatalf("registration=%+v", r)
	}
	// Receive 本身也是冪等的：job 以 event_key 為唯一鍵（真正的去重在 river，見 TestSyncJobIsUniquePerStoreInRiver 的同型檢查）。
	if f.jobs.count(kindObserve) != 2 {
		t.Fatalf("jobs=%v", f.jobs.kinds())
	}
}

// 回歸（盤點 D11、D13）：舊實作以收到的時間為觀測時間、以記憶體快照全列覆寫，亂序到達的舊通知會蓋掉新狀態。
// 新的套用以 PayUni 的 ExecuteTime 排序，只有比目前新的才更新；舊的仍追加在觀測紀錄（歷史）。
func TestOutOfOrderObservationsDoNotRegressTheStatus(t *testing.T) {
	t.Parallel()
	f := setup(t)
	rejected, pending, review := record[PayUniRejected](f), record[PayUniPendingInfo](f), record[PayUniUnderReview](f)
	_, s := f.boundStore(t)
	mer := FormatPayUniMerID(s.StoreCode)
	must(t, f.deliver(t, notification(mer, "2", t1400)))
	// 較早發生、較晚到達的通知：不能把已拒絕的狀態倒退成待補件。
	must(t, f.deliver(t, notification(mer, "91", t1300)))
	r := f.registration(t, s.StoreID)
	if deref(r.Status.MerStatus) != "2" || r.Status.Outcome != PayUniOutcomeRejected || r.NeedsReonboarding != true {
		t.Fatalf("registration=%+v", r)
	}
	if len(rejected.all()) != 1 || len(pending.all()) != 0 {
		t.Fatalf("rejected=%d pending=%d", len(rejected.all()), len(pending.all()))
	}
	if got := f.count(t, "SELECT count(*) FROM seller.payuni_observations WHERE source='WEBHOOK'"); got != 2 {
		t.Fatalf("observations=%d：舊的通知也要留在歷史", got)
	}
	// 之後更新的通知照常套用。
	must(t, f.deliver(t, notification(mer, "9", t1500)))
	r = f.registration(t, s.StoreID)
	if deref(r.Status.MerStatus) != "9" || r.Status.Outcome != PayUniOutcomeUnderReview || len(review.all()) != 0 {
		t.Fatalf("registration=%+v under_review events=%d", r, len(review.all()))
	}
}

// webhook 只回報 MerStatus：沒有回報的欄位保留目前的值，不被清掉（舊實作的 Apply 整組覆寫）。
func TestWebhookKeepsTheFieldsItDoesNotReport(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.boundStore(t)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "5", Reason: "補件原因", Credit: "1"}
	must(t, f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourcePoll))
	must(t, f.deliver(t, notification(FormatPayUniMerID(s.StoreCode), "9", t1300)))
	r := f.registration(t, s.StoreID)
	if deref(r.Status.MerStatus) != "9" || deref(r.Status.MemberStatus) != "5" || deref(r.Status.CreditStatus) != "1" || deref(r.Status.Reason) != "補件原因" ||
		r.Status.Outcome != PayUniOutcomePendingInfo {
		t.Fatalf("status=%+v", r.Status)
	}
	// 完整的輪詢結果整組取代：沒有回報的欄位就是沒有值。
	f.now = f.now.Add(3 * time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourcePoll))
	r = f.registration(t, s.StoreID)
	if r.Status.Reason != nil || r.Status.CreditStatus != nil || r.Status.Outcome != PayUniOutcomeApproved || !r.PaymentEnabled {
		t.Fatalf("status=%+v", r.Status)
	}
}

// 回歸（盤點 D9）：舊實作套用失敗只 warn、訊息照樣 ack，狀態遺失。新的套用失敗回錯誤（River 重試），觀測與狀態整筆
// 回滾，修好之後重試只套用一次。
func TestApplyFailureIsReturnedNotSwallowed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.boundStore(t)
	boom := errors.New("subscriber down")
	fail := true
	events.Subscribe(f.bus, func(context.Context, PayUniRejected) error {
		if fail {
			return boom
		}
		return nil
	})
	n := notification(FormatPayUniMerID(s.StoreCode), "2", t1300)
	must(t, f.module.ReceivePayUniNotification(context.Background(), n))
	args := f.jobs.lastObserve(t)
	if err := f.module.ApplyPayUniNotification(context.Background(), args); !errors.Is(err, boom) || errors.Is(err, ErrPayUniPermanent) {
		t.Fatalf("err=%v", err)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.payuni_observations WHERE source='WEBHOOK'"); got != 0 {
		t.Fatalf("失敗的套用留下了觀測：%d", got)
	}
	if r := f.registration(t, s.StoreID); r.Status.Outcome != PayUniOutcomeNone {
		t.Fatalf("registration=%+v", r)
	}
	fail = false
	must(t, f.module.ApplyPayUniNotification(context.Background(), args))
	must(t, f.module.ApplyPayUniNotification(context.Background(), args))
	if r := f.registration(t, s.StoreID); r.Status.Outcome != PayUniOutcomeRejected || f.count(t, "SELECT count(*) FROM seller.payuni_observations WHERE source='WEBHOOK'") != 1 {
		t.Fatalf("registration=%+v", r)
	}
}

// 回歸（盤點 D14）：未知店號、格式不合的 MerID 是永久錯誤：不重試、不留下資料（舊實作回錯誤給 RabbitMQ，永遠重送）。
func TestMalformedOrUnknownNotificationsArePermanent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.boundStore(t)
	mer := FormatPayUniMerID(s.StoreCode)
	for name, n := range map[string]PayUniNotification{
		"MerID 不是 HAWK 加店號": {MerID: "UTF8290000", EventKey: "k", MerStatus: "1", ExecuteTime: t1300},
		"沒有 event_key":      {MerID: mer, MerStatus: "1", ExecuteTime: t1300},
		"沒有 MerStatus":      {MerID: mer, EventKey: "k", ExecuteTime: t1300},
		"MerStatus 太長":      {MerID: mer, EventKey: "k", MerStatus: strings.Repeat("9", 21), ExecuteTime: t1300},
		"MerID 的店號超過 10 位數": {MerID: "HAWK12345678901", EventKey: "k", MerStatus: "1", ExecuteTime: t1300},
	} {
		if err := f.module.ReceivePayUniNotification(context.Background(), n); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	if f.jobs.count(kindObserve) != 0 {
		t.Fatal("形狀不合的通知不該排入 job")
	}
	// 形狀合法但沒有這個店家：job 回永久錯誤（worker 取消，不重試），不寫任何資料。
	unknown := notification("HAWK2600000009", "1", t1300)
	must(t, f.module.ReceivePayUniNotification(context.Background(), unknown))
	if err := f.module.ApplyPayUniNotification(context.Background(), f.jobs.lastObserve(t)); !errors.Is(err, ErrPayUniPermanent) {
		t.Fatalf("unknown store: %v", err)
	}
	if err := f.module.ApplyPayUniNotification(context.Background(), PayUniObserveArgs{EventKey: "k", MerID: "garbage", MerStatus: "1", ExecutedAt: f.now}); !errors.Is(err, ErrPayUniPermanent) {
		t.Fatalf("malformed MerID: %v", err)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.payuni_observations WHERE source='WEBHOOK'"); got != 0 {
		t.Fatalf("observations=%d", got)
	}
}

// 沒有（或格式不合）ExecuteTime 的通知無從排序：不套用它的內容，改排店家的狀態刷新，由查到的權威狀態更新。
func TestNotificationWithoutAUsableExecuteTimeTriggersARefresh(t *testing.T) {
	t.Parallel()
	f := setup(t)
	approvedEvents := record[PayUniApproved](f)
	_, s := f.boundStore(t)
	mer := FormatPayUniMerID(s.StoreCode)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	for _, execute := range []string{"", "2026-10-01 13:00:00"} {
		// 通知聲稱被拒絕，但內容不能信：只有刷新查到的狀態會被套用。
		n := PayUniNotification{MerID: mer, EventKey: "k:" + execute, MerStatus: "2", ExecuteTime: execute}
		observes, polls := f.jobs.count(kindObserve), f.jobs.count(kindPoll)
		must(t, f.module.ReceivePayUniNotification(context.Background(), n))
		if f.jobs.count(kindObserve) != observes || f.jobs.count(kindPoll) != polls+1 {
			t.Fatalf("execute=%q jobs=%v", execute, f.jobs.kinds())
		}
	}
	if r := f.registration(t, s.StoreID); r.Status.Outcome != PayUniOutcomeNone {
		t.Fatalf("通知的內容被套用了：%+v", r.Status)
	}
	must(t, f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourceRefresh))
	r := f.registration(t, s.StoreID)
	if r.Status.Outcome != PayUniOutcomeApproved || len(approvedEvents.all()) != 1 || deref(r.Status.MerStatus) != "1" {
		t.Fatalf("registration=%+v", r)
	}
	obs := f.observationPages(t, s.StoreID)
	if len(obs) != 2 || obs[0].Source != PayUniSourceRefresh {
		t.Fatalf("observations=%+v", obs)
	}
	// 沒有這個店家仍是永久錯誤；MerID 格式不合仍是 InvalidInput。
	if err := f.module.ReceivePayUniNotification(context.Background(), PayUniNotification{MerID: "HAWK2600000009", EventKey: "k", MerStatus: "1"}); !errors.Is(err, ErrPayUniPermanent) {
		t.Fatalf("unknown store: %v", err)
	}
	if err := f.module.ReceivePayUniNotification(context.Background(), PayUniNotification{MerID: "bad", EventKey: "k", MerStatus: "1"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("malformed MerID: %v", err)
	}
}

func TestPollAppliesTheStatusAndSweepSchedulesPolls(t *testing.T) {
	t.Parallel()
	f := setup(t)
	pending, approvedEvents := record[PayUniPendingInfo](f), record[PayUniApproved](f)
	_, s := f.boundStore(t)
	// 還沒有任何觀測的已綁定註冊要排輪詢；剛問過的不排。
	if _, polled, err := f.module.SweepPayUni(context.Background()); err != nil || polled != 1 || f.jobs.count(kindPoll) != 1 {
		t.Fatalf("polled=%d err=%v jobs=%v", polled, err, f.jobs.kinds())
	}
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "5", Reason: "請補存摺"}
	must(t, f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourcePoll))
	r := f.registration(t, s.StoreID)
	if r.Status.Outcome != PayUniOutcomePendingInfo || r.Status.CheckedAt == nil || len(pending.all()) != 1 || pending.all()[0].Reason != "請補存摺" {
		t.Fatalf("registration=%+v", r)
	}
	if _, polled, _ := f.module.SweepPayUni(context.Background()); polled != 0 {
		t.Fatalf("剛問過又排了輪詢：%d", polled)
	}
	// 狀態沒變：不重複通知。
	f.now = f.now.Add(3 * time.Hour)
	must(t, f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourcePoll))
	if len(pending.all()) != 1 {
		t.Fatalf("pending_info=%d", len(pending.all()))
	}
	f.now = f.now.Add(3 * time.Hour)
	if _, polled, _ := f.module.SweepPayUni(context.Background()); polled != 1 {
		t.Fatalf("待補件的註冊要繼續輪詢：%d", polled)
	}
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	f.now = f.now.Add(3 * time.Hour)
	must(t, f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourcePoll))
	if len(approvedEvents.all()) != 1 || !f.registration(t, s.StoreID).PaymentEnabled {
		t.Fatalf("approved=%d", len(approvedEvents.all()))
	}
	f.now = f.now.Add(3 * time.Hour)
	if _, polled, _ := f.module.SweepPayUni(context.Background()); polled != 0 {
		t.Fatalf("已核准的註冊不再輪詢：%d", polled)
	}
	// 傳輸錯誤回傳（River 重試）；PayUni 的業務失敗記錄後不重試。
	f.payuni.statusErr = []error{errNetwork}
	if err := f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourcePoll); !errors.Is(err, errNetwork) {
		t.Fatalf("err=%v", err)
	}
	f.payuni.statusErr = []error{apiErr("AGENT04000")}
	if err := f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourcePoll); err != nil {
		t.Fatalf("API error: %v", err)
	}
}

func TestAdminRefreshUsesTheSamePollJob(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedStore(t)
	if err := f.module.RefreshPayUniStatus(context.Background(), s.StoreID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("尚未綁定：%v", err)
	}
	f.sync(t, s.StoreID)
	must(t, f.module.RefreshPayUniStatus(context.Background(), s.StoreID))
	if f.jobs.count(kindPoll) != 1 {
		t.Fatalf("jobs=%v", f.jobs.kinds())
	}
	for i, a := range f.jobs.args {
		if a.Kind() == kindPoll {
			if b, _ := json.Marshal(a); !strings.Contains(string(b), `"REFRESH"`) || f.jobs.opts[i] == nil || !f.jobs.opts[i].UniqueOpts.ByArgs {
				t.Fatalf("args=%s opts=%+v", b, f.jobs.opts[i])
			}
		}
	}
	if err := f.module.RefreshPayUniStatus(context.Background(), s.StoreID+1000); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("unknown store: %v", err)
	}
}

func TestSweepReschedulesSyncsThatWereLost(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedStore(t)
	if resynced, _, err := f.module.SweepPayUni(context.Background()); err != nil || resynced != 0 {
		t.Fatalf("剛寫入的意圖還在等 job：%d %v", resynced, err)
	}
	f.now = f.now.Add(11 * time.Minute)
	before := f.jobs.count(kindSync)
	if resynced, _, err := f.module.SweepPayUni(context.Background()); err != nil || resynced != 1 || f.jobs.count(kindSync) != before+1 {
		t.Fatalf("resynced=%d err=%v", resynced, err)
	}
	// 失敗的註冊要管理員重新綁定，掃描不碰。
	f.payuni.bindErr = []error{apiErr("AGENT03003")}
	f.sync(t, s.StoreID)
	f.now = f.now.Add(time.Hour)
	if resynced, _, _ := f.module.SweepPayUni(context.Background()); resynced != 0 {
		t.Fatalf("resynced=%d", resynced)
	}
}

// editStore 以店家目前的版本寫入店家資料。
func editStore(f *fixture, t *testing.T, storeID int64, in UpdateProfileInput) (Store, error) {
	t.Helper()
	cur, err := f.module.Store(context.Background(), storeID)
	must(t, err)
	in.StoreID, in.ExpectedVersion = storeID, cur.Version
	return f.module.UpdateProfile(context.Background(), in)
}

// 店家的聯絡資料是唯一來源：改了就和地址一樣登記同步，同一個 job 送到 PayUni；格式不對整筆不寫入、不登記。
func TestStoreContactEditsRegisterTheSyncAndAreSentByTheSameJob(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.boundStore(t)
	for name, in := range map[string]UpdateProfileInput{
		"超過 20 字的聯絡人": {ContactName: ptr(strings.Repeat("字", 21))},
		"市話當手機":       {ContactMobile: ptr("0229010572")},
		"壞的 email":    {SupportEmail: ptr("not-an-email")},
		"壞的電話":        {ContactPhone: ptr("abc")},
	} {
		if _, err := editStore(f, t, s.StoreID, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	jobs := f.jobs.count(kindSync)
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr("李大華"), SupportEmail: ptr(" New@Example.com "), ContactPhone: ptr("02-1234-5678")}); err != nil {
		t.Fatal(err)
	}
	if r := f.registration(t, s.StoreID); !r.SyncPending || f.jobs.count(kindSync) != jobs+1 {
		t.Fatalf("registration=%+v jobs=%d", r, f.jobs.count(kindSync))
	}
	// 沒有任何改變：不寫入、不排 job。
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr("李大華")}); err != nil || f.jobs.count(kindSync) != jobs+1 {
		t.Fatalf("same edit: err=%v jobs=%d", err, f.jobs.count(kindSync))
	}
	f.sync(t, s.StoreID)
	if len(f.payuni.updates) != 1 {
		t.Fatalf("updates=%+v", f.payuni.updates)
	}
	got := f.payuni.updates[0]
	if got.ContactName != "李大華" || got.MerEmail != "new@example.com" || got.MerTel != "0212345678" || got.ContactMobile != "0912345678" || got.MerAddr == "" || got.MerAddrPost == "" {
		t.Fatalf("update=%+v", got)
	}
	if r := f.registration(t, s.StoreID); r.SyncPending || r.SyncErrorCode != nil {
		t.Fatalf("registration=%+v", r)
	}
	// PayUni 以驗證類的碼拒絕：記下碼，不重試、不迴圈；賣家再改一次才重送，成功後清掉。
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{SupportEmail: ptr("bad@example.com")}); err != nil {
		t.Fatal(err)
	}
	f.payuni.updateErr = []error{apiErr("AGENT03009")}
	f.sync(t, s.StoreID)
	r := f.registration(t, s.StoreID)
	if deref(r.SyncErrorCode) != "AGENT03009" || r.SyncPending || len(f.payuni.updates) != 2 {
		t.Fatalf("registration=%+v", r)
	}
	f.sync(t, s.StoreID)
	if len(f.payuni.updates) != 2 {
		t.Fatal("被拒絕的同步不該自己重送")
	}
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{SupportEmail: ptr("good@example.com")}); err != nil {
		t.Fatal(err)
	}
	f.sync(t, s.StoreID)
	if len(f.payuni.updates) != 3 || f.payuni.updates[2].MerEmail != "good@example.com" {
		t.Fatalf("updates=%+v", f.payuni.updates)
	}
	if r := f.registration(t, s.StoreID); r.SyncErrorCode != nil {
		t.Fatalf("成功之後要清掉同步錯誤：%+v", r)
	}
	// 停權的賣家不能改（能力檢查）。
	f.suspend(t, s.StoreID)
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr("停權後")}); apperr.KindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("suspended: %v", err)
	}
}

// 綁定之前改的聯絡資料，就是 bind 送出的值（PENDING 期間改的也一樣）；綁定時會員的 Email 與 MerEmail 都是店家的客服信箱。
func TestContactEditBeforeTheBindIsWhatGetsSent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedStore(t)
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactMobile: ptr("0987654321"), SupportEmail: ptr("support@example.com")}); err != nil {
		t.Fatal(err)
	}
	f.sync(t, s.StoreID)
	if len(f.payuni.binds) != 1 || f.payuni.binds[0].ContactMobile != "0987654321" || len(f.payuni.updates) != 0 ||
		f.payuni.binds[0].Email != "support@example.com" || f.payuni.binds[0].MerEmail != "support@example.com" {
		t.Fatalf("binds=%+v updates=%+v", f.payuni.binds, f.payuni.updates)
	}
	if r := f.registration(t, s.StoreID); r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
}

// 兩條軸合成的流程狀態（舊 cert_status_derive 的語意）：PayUni 拒絕但還能重新綁定時，賣家可以把已核准的申請改回草稿重新
// 送出，再核准之後註冊回到 PENDING 重綁。
func TestFlowStatusAndResubmissionAfterAPayUniRejection(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, sub := f.submitted(t, idA, bankAcct)
	flowOf := func() KYCFlow {
		t.Helper()
		a, err := f.module.MyKYC(ctx, accountID)
		must(t, err)
		return *a.Flow
	}
	if fl := flowOf(); fl.Status != FlowSubmitted || fl.CanResubmit {
		t.Fatalf("flow=%+v", fl)
	}
	must(t, approve(f, sub))
	if fl, r := flowOf(), f.registration(t, s.StoreID); fl.Status != FlowBinding || fl.CanResubmit || r.PaymentEnabled || r.BindStatus != BindStatusPending {
		t.Fatalf("flow=%+v registration=%+v", fl, r)
	}
	f.sync(t, s.StoreID)
	if fl := flowOf(); fl.Status != FlowForwarded {
		t.Fatalf("flow=%+v", fl)
	}
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8", Reason: "會員審核未通過"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	fl := flowOf()
	// PayUni 的事實（終止與否、問題）在註冊上，流程只有階段與能不能重送。
	if r, p := f.registration(t, s.StoreID), PayUniProblemOf(f.registration(t, s.StoreID)); fl.Status != FlowPayUniRejected || !fl.CanResubmit || r.NeedsReonboarding || p == nil || p.Code != "MEMBER_8" ||
		p.Remedy != "SELLER_FIX" || deref(p.Reason) != "會員審核未通過" {
		t.Fatalf("flow=%+v problem=%+v", fl, p)
	}
	// 賣家看到的 PayUni 狀態：整份表單的碼是 FIXABLE，不列欄位，審核意見原樣帶上。
	if a, err := f.module.MyKYC(ctx, accountID); err != nil || a.PayUni == nil || a.PayUni.State != PayUniState("FIXABLE") || a.PayUni.Reason != "會員審核未通過" ||
		len(a.PayUni.FixFields) != 0 || len(a.PayUni.FixStoreFields) != 0 || len(a.ResubmitReasons) != 0 {
		t.Fatalf("payuni=%+v reasons=%v err=%v", a.PayUni, a.ResubmitReasons, err)
	}
	// 賣家改了店家的聯絡手機；回到草稿、重新送出、再核准：核准不會把店家的聯絡資料重設成別的值。
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactMobile: ptr("0955555555")}); err != nil {
		t.Fatal(err)
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王小明")})
	must(t, err)
	if edited.Status != KYCStatusDraft || edited.Flow.Status != FlowDraft || !edited.Flow.CanResubmit {
		t.Fatalf("edited=%+v flow=%+v", edited, edited.Flow)
	}
	// 回到草稿之後註冊仍是這份申請的 PayUni 拒絕：問題要留著，賣家正在修正它。
	if edited.PayUni == nil || edited.PayUni.State != PayUniState("FIXABLE") || edited.PayUni.Reason != "會員審核未通過" {
		t.Fatalf("草稿的 payuni=%+v", edited.PayUni)
	}
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: edited.Version})
	must(t, err)
	// 送出之後回到平台審核：沒有 PayUni 狀態，舊的拒絕不再出現。
	if again.PayUni != nil || again.Flow.Status != FlowSubmitted {
		t.Fatalf("送出之後 payuni=%+v flow=%+v", again.PayUni, again.Flow)
	}
	must(t, approve(f, again))
	// 已知缺口（seller CARRYOVER.md：重新核准之後 PayUni 還沒重審的期間，註冊仍是舊的拒絕）：流程回到 PAYUNI_REJECTED、仍是 FIXABLE。
	if a, err := f.module.MyKYC(ctx, accountID); err != nil || a.PayUni == nil || a.PayUni.State != PayUniState("FIXABLE") {
		t.Fatalf("重新核准之後 payuni=%+v err=%v", a.PayUni, err)
	}
	r := f.registration(t, s.StoreID)
	// 已綁定的註冊重新核准：PayUni 端已有這個商店（再 bind 只會得到 AGENT04017），維持 BOUND，新的資料以 update_merchant 送。
	if r.BindStatus != BindStatusBound || !r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
	if st, err := f.module.Store(ctx, s.StoreID); err != nil || deref(st.ContactMobile) != "0955555555" {
		t.Fatalf("重新核准改動了店家的聯絡資料：%+v err=%v", st, err)
	}
	// PayUni 的狀態與歷史保留（外部事實）。
	if r.Status.Outcome != PayUniOutcomeRejected || f.count(t, "SELECT count(*) FROM seller.payuni_observations") < 2 {
		t.Fatalf("registration=%+v", r)
	}
	f.sync(t, s.StoreID)
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusBound || r.SyncPending || len(f.payuni.binds) != 1 || len(f.payuni.updates) != 1 ||
		f.payuni.updates[0].ContactMobile != "0955555555" {
		t.Fatalf("registration=%+v binds=%d updates=%+v", r, len(f.payuni.binds), f.payuni.updates)
	}
}

func TestSubmitsDirectlyFromAResubmittablePayUniRejection(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.boundStore(t)
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: mine.Version}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("submit while approved and not rejected: %v", err)
	}
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8", Reason: "會員審核未通過"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	mine, err = f.module.MyKYC(ctx, accountID)
	must(t, err)
	if !mine.Flow.CanResubmit || mine.Status != KYCStatusApproved {
		t.Fatalf("mine=%+v flow=%+v", mine, mine.Flow)
	}
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: mine.Version})
	must(t, err)
	if again.Status != KYCStatusSubmitted || again.DecidedAt != nil {
		t.Fatalf("again=%+v", again)
	}
	must(t, approve(f, again))
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusBound || !r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
}

// 回歸（CR-1）：PayUni 會員層級拒絕之後重新送出、再核准，修正的資料要送到 PayUni（update_merchant；舊的實作再 bind 得到
// AGENT04017，認領之後標成已綁定、什麼都沒送），而且輪詢要繼續問新的結果（拒絕是舊的狀態，輪詢原本排除 REJECTED，結果
// 永遠停在 REJECTED）。
func TestReapprovalAfterAMemberRejectionSendsTheCorrectedDataAndResumesPolling(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	approvedEvents := record[PayUniApproved](f)
	accountID, s := f.boundStore(t)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8", Reason: "會員審核未通過"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	// 已拒絕的註冊平常不輪詢。
	f.now = f.now.Add(3 * time.Hour)
	if _, polled, _ := f.module.SweepPayUni(ctx); polled != 0 {
		t.Fatalf("已拒絕的註冊不該輪詢：%d", polled)
	}
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactMobile: ptr("0966666666")}); err != nil {
		t.Fatal(err)
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王小明")})
	must(t, err)
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: edited.Version})
	must(t, err)
	must(t, approve(f, again))
	f.payuni.bindErr = []error{apiErr("AGENT04017")} // 若 job 又去 bind，PayUni 會這樣回
	f.sync(t, s.StoreID)
	if len(f.payuni.binds) != 1 || len(f.payuni.updates) != 1 || f.payuni.updates[0].ContactMobile != "0966666666" {
		t.Fatalf("修正的資料沒有送到 PayUni：binds=%d updates=%+v", len(f.payuni.binds), f.payuni.updates)
	}
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusBound || r.SyncPending {
		t.Fatalf("registration=%+v", r)
	}
	// 輪詢繼續問：PayUni 重審通過之後結果才會更新。
	f.now = f.now.Add(3 * time.Hour)
	if _, polled, _ := f.module.SweepPayUni(ctx); polled != 1 {
		t.Fatalf("重新核准之後的輪詢沒有恢復：%d", polled)
	}
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	if r := f.registration(t, s.StoreID); r.Status.Outcome != PayUniOutcomeApproved || !r.PaymentEnabled || len(approvedEvents.all()) != 1 {
		t.Fatalf("registration=%+v approved=%d", r, len(approvedEvents.all()))
	}
	// 有了新的結果就停止輪詢；沒有新結果的註冊只在一段期間內輪詢（之後只剩 webhook）。
	f.now = f.now.Add(3 * time.Hour)
	if _, polled, _ := f.module.SweepPayUni(ctx); polled != 0 {
		t.Fatalf("已核准的註冊不該輪詢：%d", polled)
	}
}

func TestResubmittedRegistrationStopsPollingAfterTheWindow(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.boundStore(t)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王小明")})
	must(t, err)
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: edited.Version})
	must(t, err)
	must(t, approve(f, again))
	f.sync(t, s.StoreID)
	f.now = f.now.Add(8 * 24 * time.Hour)
	if _, polled, _ := f.module.SweepPayUni(ctx); polled != 0 {
		t.Fatalf("超過期間的註冊不該再輪詢：%d", polled)
	}
}

// 商家層級的終止狀態：同一個 MerID 不會再回到可用，不能把申請改回草稿重送，要關店重新入駐。
func TestTerminalPayUniStatusCannotBeResubmitted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.boundStore(t)
	f.payuni.status = pu.MerchantStatus{MerStatus: "92", Reason: "不符資格"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	fl := mine.Flow
	if r, p := f.registration(t, s.StoreID), PayUniProblemOf(f.registration(t, s.StoreID)); fl.Status != FlowPayUniRejected || fl.CanResubmit || !r.NeedsReonboarding || p == nil || p.Code != "MER_92" ||
		mine.PayUni == nil || mine.PayUni.State != PayUniState("TERMINATED") {
		t.Fatalf("flow=%+v problem=%+v payuni=%+v", fl, p, mine.PayUni)
	}
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王小明")}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("edit after a terminal status: %v", err)
	}
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: mine.Version}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("submit after a terminal status: %v", err)
	}
	// 重新入駐：關店之後開新的 stint，帶新的 store_code（MerID 是一次性的），舊的註冊列留在舊 stint。
	_, err = f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	next, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	if next.StoreCode == s.StoreCode {
		t.Fatal("新的 stint 沿用了舊的 store_code")
	}
	if _, err := f.module.MyKYC(ctx, accountID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("新的 stint 不該看到舊的申請與註冊：%v", err)
	}
}

// 回歸（盤點 D17）：舊實作只在 cert 是 submitted／forwarded 時鎖店名與介紹，讀取失敗時放行，PayUni 待補件、綁定失敗都不鎖。
// 新的鎖由 KYC 審核與 PayUni 綁定狀態推導：PayUni 持有的期間（PENDING、BOUND）鎖住，沒有收下（FAILED）時解鎖。
func TestStoreNameAndDescriptionAreLockedWhilePayUniHoldsThem(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, sub := f.submitted(t, idA, bankAcct)
	rename := func(name string) error {
		cur, err := f.module.Store(ctx, s.StoreID)
		must(t, err)
		_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, StoreName: ptr(name)})
		return err
	}
	describe := func(text string) error {
		cur, err := f.module.Store(ctx, s.StoreID)
		must(t, err)
		_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, Description: ptr(text)})
		return err
	}
	other := func() error {
		cur, err := f.module.Store(ctx, s.StoreID)
		must(t, err)
		_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, Prize: ptr("金牌")})
		return err
	}
	locked := func(stage string) {
		t.Helper()
		if err := rename("新店名"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("%s：改店名 %v", stage, err)
		}
		if err := describe("新介紹"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("%s：改介紹 %v", stage, err)
		}
		if err := other(); err != nil {
			t.Errorf("%s：改其他欄位 %v", stage, err)
		}
	}
	locked("審核中")
	must(t, approve(f, sub))
	locked("綁定中")
	f.payuni.bindErr = []error{apiErr("AGENT04003")}
	f.sync(t, s.StoreID)
	if err := rename("修正後的店名"); err != nil {
		t.Fatalf("綁定失敗時要能修正店名：%v", err)
	}
	_, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
	must(t, err)
	f.sync(t, s.StoreID)
	locked("已綁定")
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "5"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	locked("PayUni 待補件")
	_ = accountID
}

func TestPayUniStateIsAvailableOnlyForTheOwnStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, _ = f.approvedStore(t)
	other := f.account(t)
	if _, err := f.module.MyKYC(context.Background(), other); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("other account: %v", err)
	}
}

// 以真的 River 驗證唯一鍵：同一個店家同時最多一個還沒有結果的 sync job（連續的改動合併）；完成之後的新改動要能排進去
// （唯一鍵不含 completed，否則 River 清掉完成的 job 之前，之後的改動都被擋下）。
func TestSyncJobIsUniquePerStoreInRiver(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	must(t, jobs.Migrate(ctx, f.db))
	jc, err := jobs.NewInsertOnly(f.db)
	must(t, err)
	f.module = f.newModule(t, f.payuni, jc)
	_, s := f.approvedStore(t)
	syncJobs := func(states ...string) int64 {
		return f.count(t, "SELECT count(*) FROM river.river_job WHERE kind=$1 AND args->>'store_id'=$2 AND state = ANY($3)", kindSync, fmt.Sprint(s.StoreID), states)
	}
	if n := syncJobs("available"); n != 1 {
		t.Fatalf("核准後的 sync job = %d", n)
	}
	setAddress(f, t, s.StoreID, "第一次改")
	setAddress(f, t, s.StoreID, "第二次改")
	if n := syncJobs("available", "scheduled", "running", "retryable", "pending"); n != 1 {
		t.Fatalf("連續的改動排了 %d 個 job，應該合併成一個", n)
	}
	// 另一個店家有自己的 job。
	_, s2, sub2 := f.submitted(t, idB, "999888777666")
	must(t, approve(f, sub2))
	if n := f.count(t, "SELECT count(*) FROM river.river_job WHERE kind=$1 AND state='available'", kindSync); n != 2 {
		t.Fatalf("兩個店家的 sync job = %d（store %d 與 %d）", n, s.StoreID, s2.StoreID)
	}
	// 執行中的 job 也擋下新的 job（唯一的狀態集合含 running）：同一個店家同時只有一個 job 呼叫 PayUni，不需要另外的租約。
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE river.river_job SET state='running', attempted_at=now() WHERE kind=$1 AND args->>'store_id'=$2", kindSync, fmt.Sprint(s.StoreID)); err != nil {
		t.Fatal(err)
	}
	setAddress(f, t, s.StoreID, "執行中改")
	if n := syncJobs("available", "scheduled", "running", "retryable", "pending"); n != 1 {
		t.Fatalf("執行中的 job 沒有擋下新的 job：%d", n)
	}
	// job 完成之後，新的改動要排得進去。
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE river.river_job SET state='completed', finalized_at=now() WHERE kind=$1", kindSync); err != nil {
		t.Fatal(err)
	}
	setAddress(f, t, s.StoreID, "完成之後改")
	if n := syncJobs("available"); n != 1 {
		t.Fatalf("job 完成之後的改動沒有排進去：%d", n)
	}
	// webhook job 以 event_key 去重（重送的通知只排一個）。
	n := notification(FormatPayUniMerID(s.StoreCode), "91", t1300)
	must(t, f.module.ReceivePayUniNotification(ctx, n))
	must(t, f.module.ReceivePayUniNotification(ctx, n))
	if got := f.count(t, "SELECT count(*) FROM river.river_job WHERE kind=$1", kindObserve); got != 1 {
		t.Fatalf("重送的通知排了 %d 個 job", got)
	}
}

func TestPayUniSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.boundStore(t)
	for name, query := range map[string]string{
		"bound without a time":            "UPDATE seller.payuni_registrations SET bound_at=NULL",
		"failed without a code":           "UPDATE seller.payuni_registrations SET bind_status='FAILED', bound_at=NULL",
		"failed time without failure":     "UPDATE seller.payuni_registrations SET bind_failed_at=now()",
		"failed without a failed time":    "UPDATE seller.payuni_registrations SET bind_status='FAILED', bound_at=NULL, bind_error_code='AGENT04003'",
		"a code on a bound registration":  "UPDATE seller.payuni_registrations SET bind_error_code='AGENT04003'",
		"lowercase code":                  "UPDATE seller.payuni_registrations SET bind_status='FAILED', bound_at=NULL, bind_error_code='agent'",
		"synced beyond the intent":        "UPDATE seller.payuni_registrations SET synced_seq=intent_seq+1",
		"malformed mer id":                "UPDATE seller.payuni_registrations SET mer_id='X1'",
		"landline as the mobile":          "UPDATE seller.payuni_registrations SET contact_mobile='0229010572'",
		"uppercase email":                 "UPDATE seller.payuni_registrations SET mer_email='A@B.COM'",
		"empty contact name":              "UPDATE seller.payuni_registrations SET contact_name=''",
		"outcome without an observation":  "UPDATE seller.payuni_registrations SET outcome='APPROVED'",
		"observation without an outcome":  "UPDATE seller.payuni_registrations SET status_observed_at=now()",
		"unknown outcome":                 "UPDATE seller.payuni_registrations SET outcome='PAID', status_observed_at=now()",
		"unknown bind status":             "UPDATE seller.payuni_registrations SET bind_status='DONE'",
		"empty status code":               "UPDATE seller.payuni_registrations SET mer_status=''",
		"zero version":                    "UPDATE seller.payuni_registrations SET version=0",
		"second registration for a store": "INSERT INTO seller.payuni_registrations (store_id, mer_id, application_id, contact_name, contact_mobile, mer_email, created_at, updated_at) SELECT store_id, 'HAWK9', application_id, 'x', '0912345678', 'a@b.co', now(), now() FROM seller.payuni_registrations",
		"duplicate mer id":                "INSERT INTO seller.payuni_registrations (store_id, mer_id, application_id, contact_name, contact_mobile, mer_email, created_at, updated_at) SELECT store_id + 1, mer_id, application_id, 'x', '0912345678', 'a@b.co', now(), now() FROM seller.payuni_registrations",
		"webhook without an event key":    fmt.Sprintf("INSERT INTO seller.payuni_observations (observation_id, store_id, source, executed_at, received_at) VALUES (uuidv7(), %d, 'WEBHOOK', now(), now())", s.StoreID),
		"unknown source":                  fmt.Sprintf("INSERT INTO seller.payuni_observations (observation_id, store_id, source, executed_at, received_at) VALUES (uuidv7(), %d, 'EMAIL', now(), now())", s.StoreID),
		"observation for a missing store": "INSERT INTO seller.payuni_observations (observation_id, store_id, source, executed_at, received_at) VALUES (uuidv7(), 999999, 'POLL', now(), now())",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, query); err == nil {
			t.Errorf("%s 應該被 DB 擋下", name)
		}
	}
	insert := fmt.Sprintf("INSERT INTO seller.payuni_observations (observation_id, store_id, source, event_key, executed_at, received_at) VALUES (uuidv7(), %d, 'WEBHOOK', 'k', now(), now())", s.StoreID)
	_, err := f.db.Conn(ctx).Exec(ctx, insert)
	must(t, err)
	if _, err := f.db.Conn(ctx).Exec(ctx, insert); err == nil {
		t.Error("同一個店家的 event_key 重複")
	}
}

func TestPayUniChecksMatchGo(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for constraint, want := range map[string][]string{
		"payuni_registrations_bind_status_check": stringsOf(PayUniBindStatusValues()),
		"payuni_registrations_outcome_check":     stringsOf(PayUniOutcomeValues()),
		"payuni_observations_source_check":       stringsOf(PayUniSourceValues()),
	} {
		var def string
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname=$1", constraint).Scan(&def))
		for _, v := range want {
			if !strings.Contains(def, "'"+v+"'") {
				t.Errorf("%s 缺 %s：%s", constraint, v, def)
			}
		}
		if got := strings.Count(def, "'") / 2; got != len(want) {
			t.Errorf("%s 的值數 %d 與 Go 的 %d 不同：%s", constraint, got, len(want), def)
		}
	}
}

// outcome 欄位與原碼由同一個 UPDATE 寫入：它永遠等於原碼的推導。
func TestStoredOutcomeAlwaysMatchesTheDerivation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.boundStore(t)
	tick := time.Hour
	for _, tc := range []struct{ mer, member string }{{"1", "1"}, {"1", "5"}, {"9", ""}, {"91", "1"}, {"92", "1"}, {"1", "8"}, {"7", "1"}, {"2", ""}} {
		f.now = f.now.Add(tick)
		f.payuni.status = pu.MerchantStatus{MerStatus: tc.mer, MemberStatus: tc.member}
		must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
		r := f.registration(t, s.StoreID)
		if want := PayUniOutcomeOf(tc.mer, tc.member); r.Status.Outcome != want || deref(r.Status.MerStatus) != tc.mer {
			t.Errorf("(%q, %q)：存的是 %s，推導是 %s", tc.mer, tc.member, r.Status.Outcome, want)
		}
	}
}

// 回歸（CR-2）：結果的 transaction 先鎖店家列、再鎖註冊列，與核准、重新綁定、賣家改資料相同。舊的順序是先鎖註冊列、再在
// 寫觀測列時（外鍵對店家列取 KEY SHARE）碰店家列，會和「先鎖店家列、再鎖註冊列」的寫入者互鎖。測試在 bind 呼叫進行中（job
// 還沒寫結果）拿住店家列的鎖、模擬另一個寫入者，等結果的 transaction 卡在店家列上之後，再去鎖註冊列：結果的 transaction
// 還沒碰註冊列時這一步成功，舊的順序會死結（PostgreSQL 會中止其中一邊）。
func TestSyncFinishLocksTheStoreBeforeTheRegistration(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.approvedStore(t)
	holderDone := make(chan error, 1)
	f.payuni.onBind = func() {
		holder, err := f.db.Pool().Begin(ctx)
		must(t, err)
		_, err = holder.Exec(ctx, "SELECT 1 FROM seller.stores WHERE store_id=$1 FOR UPDATE", s.StoreID)
		must(t, err)
		go func() {
			holderDone <- func() error {
				deadline := time.Now().Add(15 * time.Second)
				for {
					var waiting int64
					if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(&waiting); err != nil {
						_ = holder.Rollback(ctx)
						return err
					}
					if waiting >= 1 {
						break
					}
					if time.Now().After(deadline) {
						_ = holder.Rollback(ctx)
						return errors.New("結果的 transaction 沒有等店家列的鎖")
					}
					time.Sleep(10 * time.Millisecond)
				}
				if _, err := holder.Exec(ctx, "SELECT 1 FROM seller.payuni_registrations WHERE store_id=$1 FOR UPDATE", s.StoreID); err != nil {
					_ = holder.Rollback(ctx)
					return err
				}
				return holder.Commit(ctx)
			}()
		}()
	}
	f.sync(t, s.StoreID)
	if err := <-holderDone; err != nil {
		t.Fatalf("另一個寫入者鎖註冊列失敗（死結？）：%v", err)
	}
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusBound {
		t.Fatalf("registration=%+v", r)
	}
}

// 回歸（CR-8）：資料不完整造成的失敗只在資料補齊時自動回到 PENDING；不相干的修改（賣家改聯絡欄位）不重跑一次註定失敗的
// job，不發新的失敗事件，也不重置嘗試次數。
func TestIncompleteDataFailureReturnsToPendingOnlyWhenTheDataIsComplete(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	failed := record[PayUniBindFailed](f)
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{TownCode: ptr(""), AddressDetail: ptr("")}); err != nil {
		t.Fatal(err)
	}
	must(t, approve(f, sub))
	f.sync(t, s.StoreID)
	before := f.registration(t, s.StoreID)
	if before.BindStatus != BindStatusFailed || deref(before.BindErrorCode) != PayUniErrIncompleteData || before.BindAttempts != 1 {
		t.Fatalf("registration=%+v", before)
	}
	jobs := f.jobs.count(kindSync)
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr("李大華")}); err != nil {
		t.Fatal(err)
	}
	after := f.registration(t, s.StoreID)
	if after.BindStatus != BindStatusFailed || after.BindAttempts != 1 || f.jobs.count(kindSync) != jobs || len(failed.all()) != 1 {
		t.Fatalf("不相干的修改把失敗的註冊拉回去了：registration=%+v jobs=%d failed=%d", after, f.jobs.count(kindSync)-jobs, len(failed.all()))
	}
	// 地址補齊之後才自動回到 PENDING。
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{TownCode: ptr("63000050"), AddressDetail: ptr("新路二號")}); err != nil {
		t.Fatal(err)
	}
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusPending || f.jobs.count(kindSync) != jobs+1 {
		t.Fatalf("registration=%+v jobs=%d", r, f.jobs.count(kindSync)-jobs)
	}
}

// 回歸（CR-10）：管理員重新綁定留下 append-only 的紀錄（誰、何時、哪一份申請），而不是丟掉 AdminID。
func TestAdminRebindIsRecorded(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.approvedStore(t)
	f.payuni.bindErr = []error{apiErr("AGENT04003")}
	f.sync(t, s.StoreID)
	_, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 4242})
	must(t, err)
	reg := f.registration(t, s.StoreID)
	var n int64
	must(t, f.db.Conn(ctx).QueryRow(ctx,
		"SELECT count(*) FROM seller.kyc_reviews WHERE action='REBIND' AND reviewer_admin_id=4242 AND application_id=$1 AND decided_at=$2",
		reg.ApplicationID, f.now).Scan(&n))
	if n != 1 {
		t.Fatalf("重新綁定的紀錄 = %d", n)
	}
	// 重送（已經在重新綁定）不重複記錄。
	if _, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 4242}); err != nil {
		t.Fatal(err)
	}
	if f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action='REBIND'") != 1 {
		t.Fatal("重送又記了一筆")
	}
}

// 回歸：重新綁定回傳的註冊與一般讀取（PayUniRegistrationOf）是同一個形狀，帶申請的賣家型態（問題要補的欄位依它過濾）；
// 第一次重新綁定與已經在重新綁定時的重送都一樣。
func TestRebindReturnsTheRegistrationWithTheSellerType(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.approvedStore(t)
	f.payuni.bindErr = []error{apiErr("AGENT04003")}
	f.sync(t, s.StoreID)
	first, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 4242})
	must(t, err)
	if first.SellerType != KYCIndividual || first.BindStatus != BindStatusPending || !reflect.DeepEqual(first.PayUniRegistration, f.registration(t, s.StoreID)) {
		t.Fatalf("rebind=%+v read=%+v", first, f.registration(t, s.StoreID))
	}
	again, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 4242})
	must(t, err)
	if !reflect.DeepEqual(again, first) {
		t.Fatalf("replay=%+v first=%+v", again, first)
	}
}

// 觀測紀錄的 keyset：新到舊，每頁不漏不重，只有這個店家的，page size 的預設與上限由模組定。
func TestPayUniObservationsPageNewestFirstPerStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, a := f.boundStore(t)
	_, b := f.boundStore(t)
	for i := range 4 {
		must(t, f.deliver(t, notification(FormatPayUniMerID(a.StoreCode), "9", fmt.Sprintf("202610011%d0000", 3+i))))
	}
	all := f.observationPages(t, a.StoreID)
	if len(all) != 5 {
		t.Fatalf("a 的觀測 = %d，要 1 筆綁定加 4 筆通知", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].ExecutedAt.Before(all[i].ExecutedAt) {
			t.Fatalf("不是新到舊：%v 在 %v 之前", all[i-1].ExecutedAt, all[i].ExecutedAt)
		}
	}
	if only := f.observationPages(t, b.StoreID); len(only) != 1 {
		t.Fatalf("b 只有自己的綁定觀測：%d", len(only))
	}
	big, err := f.module.PayUniObservations(ctx, PayUniObservationListInput{StoreID: a.StoreID})
	must(t, err)
	if len(big.Items) != 5 || big.Next != nil {
		t.Fatalf("預設的頁大小要容得下 5 筆：%d next=%v", len(big.Items), big.Next)
	}
	if _, err := f.module.PayUniObservations(ctx, PayUniObservationListInput{StoreID: a.StoreID, PageSize: -1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("負的頁大小：%v", err)
	}
	if _, err := f.module.PayUniObservations(ctx, PayUniObservationListInput{StoreID: 0}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("store 0：%v", err)
	}
	// 沒有這家店是 NotFound，不是空的一頁（店家存在但還沒有觀測才是空的）。
	if _, err := f.module.PayUniObservations(ctx, PayUniObservationListInput{StoreID: b.StoreID + a.StoreID + 1000}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的店家：%v", err)
	}
	_, fresh := f.open(t)
	if page, err := f.module.PayUniObservations(ctx, PayUniObservationListInput{StoreID: fresh.StoreID}); err != nil || len(page.Items) != 0 || page.Next != nil {
		t.Fatalf("還沒有觀測的店家是空的一頁：%+v err=%v", page, err)
	}
}

// 外部限制（PayUni 代理商專用 API 總覽 §8）：update_merchant 不能改 bind_merchant 建立的核心身分欄位，所以註冊有 MerID 之後，
// PayUni 拒絕、申請改回草稿修正的期間，店名與介紹、KYC 的身分欄位仍鎖住；只有聯絡資料與地址能改，而且只有它們會送出。
func TestIdentityStaysLockedWhileThePayUniRejectedApplicationIsBackInDraft(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.boundStore(t)
	rename := func(name string) error {
		cur, err := f.module.Store(ctx, s.StoreID)
		must(t, err)
		_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, StoreName: ptr(name)})
		return err
	}
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8", Reason: "店名不符"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	if err := rename("被拒絕後還沒改回草稿"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("已核准的申請還沒改回草稿：%v", err)
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if !mine.Flow.IdentityLocked || !mine.Flow.CanResubmit {
		t.Fatalf("flow=%+v", mine.Flow)
	}
	// 聯絡資料與地址能改，地址排入同步。
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactMobile: ptr("0966666666"), SupportEmail: ptr("new@example.com")}); err != nil {
		t.Fatal(err)
	}
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王小明")})
	must(t, err)
	if !edited.Flow.IdentityLocked || edited.Flow.Status != FlowDraft {
		t.Fatalf("flow=%+v", edited.Flow)
	}
	setAddress(f, t, s.StoreID, "新的路二號")
	// 店名、介紹與身分欄位仍鎖住：回 FailedPrecondition，版本不動。
	if err := rename("修正後的店名"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("草稿修正期間店名仍鎖住：%v", err)
	}
	cur, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, Description: ptr("新的介紹")}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("草稿修正期間介紹仍鎖住：%v", err)
	}
	for name, in := range map[string]KYCDraftInput{
		"RealName": {RealName: ptr("王大明")}, "IDCardNo": {IDCardNo: ptr(idB)}, "EnglishName": {EnglishName: ptr("New Farm")},
		"WebsiteURL": {WebsiteURL: ptr("https://new.example.com")}, "ProductType": {ProductType: ptr("水果")},
		"SellerType": {SellerType: ptr(KYCCompany)},
	} {
		in.AccountID, in.ExpectedVersion = accountID, edited.Version
		if _, err := f.module.SaveKYCDraft(ctx, in); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("%s 綁定之後不能改：%v", name, err)
		}
	}
	// 身分已鎖定時取消會讓新申請繞過身分鎖：拒絕，申請與版本不動。
	_, err = f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: edited.Version})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "申請不能取消") {
		t.Fatalf("身分鎖定時取消草稿：%v", err)
	}
	if kept, err := f.module.MyKYC(ctx, accountID); err != nil || kept.Status != KYCStatusDraft || kept.Version != edited.Version {
		t.Fatalf("取消被拒後申請不變：%+v err=%v", kept, err)
	}
	// 沒有改變的身分欄位重送不算修改。
	same, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: edited.Version, RealName: ptr("王小明"), IDCardNo: ptr(idA), EnglishName: ptr("Wang Farm")})
	must(t, err)
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: same.Version})
	must(t, err)
	if err := rename("審核中改"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("審核中：%v", err)
	}
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: again.Version, RealName: ptr("王小明")})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "申請審核中，請等待審核結果。") {
		t.Fatalf("身分鎖定且審核中修改：%v", err)
	}
	_, err = f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: again.Version})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "申請不能取消") {
		t.Fatalf("身分鎖定且審核中取消：%v", err)
	}
	must(t, approve(f, again))
	// 只送出聯絡欄位與地址（update_merchant），身分欄位不重送。
	f.sync(t, s.StoreID)
	if len(f.payuni.binds) != 1 || len(f.payuni.updates) == 0 {
		t.Fatalf("binds=%d updates=%d", len(f.payuni.binds), len(f.payuni.updates))
	}
	last := f.payuni.updates[len(f.payuni.updates)-1]
	if last.ContactMobile != "0966666666" || last.MerEmail != "new@example.com" || !strings.Contains(last.MerAddr, "新的路二號") {
		t.Fatalf("update=%+v", last)
	}
}

// 還沒有 MerID（沒有註冊列）時，草稿與被退回的申請能改身分欄位與店名；綁定失敗（PayUni 沒有收下）時也解鎖。
func TestIdentityIsEditableBeforeAnyMerIDExists(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, sub := f.submitted(t, idA, bankAcct)
	_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "資料不符"})
	must(t, err)
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if mine.Flow.IdentityLocked || mine.Flow.Status != FlowRejected {
		t.Fatalf("flow=%+v", mine.Flow)
	}
	fixed, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version,
		RealName: ptr("王大明"), IDCardNo: ptr(idB), EnglishName: ptr("New Farm"), WebsiteURL: ptr("https://new.example.com")})
	must(t, err)
	if fixed.Flow.IdentityLocked {
		t.Fatalf("flow=%+v", fixed.Flow)
	}
	cur, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, StoreName: ptr("新店名")})
	must(t, err)
}

// observationPages 讀店家全部的觀測紀錄（新到舊）。每頁一筆，所以每一處讀取都走過 keyset 翻頁，頁與頁之間不漏不重。
func (f *fixture) observationPages(t *testing.T, storeID int64) []PayUniObservation {
	t.Helper()
	var out []PayUniObservation
	seen := map[uuid.UUID]bool{}
	in := PayUniObservationListInput{StoreID: storeID, PageSize: 1}
	for {
		page, err := f.module.PayUniObservations(context.Background(), in)
		must(t, err)
		for _, o := range page.Items {
			if seen[o.ID] {
				t.Fatalf("duplicate observation %v", o.ID)
			}
			seen[o.ID] = true
			out = append(out, o)
		}
		if page.Next == nil {
			return out
		}
		in.After = page.Next
	}
}

// 回歸：有 PayUni 註冊之後，PayUni 必填的店家欄位（聯絡人姓名與手機、客服信箱、地址）不能清掉；清掉會讓 PayUni 留著舊值、同步卻記成成功。
// 沒有註冊（還沒核准）時可以清；原本就缺的欄位不相干的修改照常通過。
func TestRequiredStoreFieldsCannotBeClearedOnceARegistrationExists(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	// 還沒有註冊：清掉是允許的（送出 KYC 之前會再檢查完整性）。
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr("")}); err != nil {
		t.Fatalf("沒有註冊時應該可以清：%v", err)
	}
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr("王小明")}); err != nil {
		t.Fatal(err)
	}
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	sub, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	must(t, approve(f, sub))
	f.sync(t, s.StoreID)
	jobs := f.jobs.count(kindSync)
	for name, tc := range map[string]struct {
		in    UpdateProfileInput
		field string
	}{
		"聯絡人姓名": {UpdateProfileInput{ContactName: ptr("")}, "ContactName"},
		"聯絡人手機": {UpdateProfileInput{ContactMobile: ptr(" ")}, "ContactMobile"},
		"客服信箱":  {UpdateProfileInput{SupportEmail: ptr("")}, "SupportEmail"},
		"整個地址":  {UpdateProfileInput{TownCode: ptr(""), AddressDetail: ptr("")}, "TownCode"},
		"地址明細":  {UpdateProfileInput{AddressDetail: ptr("")}, "AddressDetail"},
	} {
		_, err := editStore(f, t, s.StoreID, tc.in)
		var ae *apperr.Error
		if apperr.KindOf(err) != apperr.KindInvalidInput || !errors.As(err, &ae) || !hasField(ae, tc.field) {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	cur, err := f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	if deref(cur.ContactName) != "王小明" || deref(cur.SupportEmail) != "owner@example.com" || cur.AddressDetail == nil || f.jobs.count(kindSync) != jobs {
		t.Fatalf("被拒絕的修改不該寫入或排 job：%+v", cur)
	}
	// 換成別的值是允許的，商店電話不是必填、可以清。
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr("李大華"), ContactPhone: ptr("")}); err != nil {
		t.Fatal(err)
	}
}

// 同步時店家的必填資料是空的（例如繞過寫入檢查的資料問題）：不呼叫 PayUni、不記成成功，記下 INCOMPLETE_DATA 讓賣家看見。
func TestSyncFailsLoudlyWhenARequiredStoreValueIsEmpty(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.boundStore(t)
	if _, err := f.db.Conn(context.Background()).Exec(context.Background(), "UPDATE seller.stores SET support_email = NULL WHERE store_id = $1", s.StoreID); err != nil {
		t.Fatal(err)
	}
	// 以地址改動登記同步（聯絡信箱在直接寫入時是空的）。
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{AddressDetail: ptr("新路九號")}); err != nil {
		t.Fatal(err)
	}
	f.sync(t, s.StoreID)
	if len(f.payuni.updates) != 0 {
		t.Fatalf("必填資料是空的仍然呼叫了 PayUni：%+v", f.payuni.updates)
	}
	if r := f.registration(t, s.StoreID); deref(r.SyncErrorCode) != PayUniErrIncompleteData {
		t.Fatalf("同步失敗要留下碼：%+v", r)
	}
}

func hasField(e *apperr.Error, field string) bool {
	for _, v := range e.FieldViolations() {
		if v.Field == field {
			return true
		}
	}
	return false
}
