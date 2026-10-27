//go:build integration

package seller_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/gcis"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// fakeRegistry 是登記資料來源的替身：回應照順序消耗（沒有排定的回應是查無），每次呼叫都確認不在 DB transaction 內、有 deadline。
type fakeRegistry struct {
	t       *testing.T
	mu      sync.Mutex
	replies []registryReply
	calls   []string
	// onLookup 在呼叫進行中執行，用來製造「呼叫期間別的查詢先寫入」的交錯。
	onLookup func()
}

type registryReply struct {
	reg   gcis.Registration
	found bool
	err   error
}

func (r *fakeRegistry) Lookup(ctx context.Context, taxID string) (gcis.Registration, bool, error) {
	r.t.Helper()
	if _, ok := db.TxFrom(ctx); ok {
		r.t.Error("登記資料來源在 DB transaction 內被呼叫")
	}
	if _, ok := ctx.Deadline(); !ok {
		r.t.Error("登記資料來源的呼叫沒有 deadline")
	}
	r.mu.Lock()
	r.calls = append(r.calls, taxID)
	var reply registryReply
	if len(r.replies) > 0 {
		reply, r.replies = r.replies[0], r.replies[1:]
	}
	hook := r.onLookup
	r.onLookup = nil
	r.mu.Unlock()
	if hook != nil {
		hook()
	}
	return reply.reg, reply.found, reply.err
}

func (r *fakeRegistry) reply(reg gcis.Registration, found bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replies = append(r.replies, registryReply{reg, found, err})
}

func (r *fakeRegistry) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

var errRegistryDown = errors.New("連線被重設")

// matchingRegistration 是與 companySubmission 填的申請完全一致的登記資料（名稱、地址、日期都用另一種寫法：臺、全形數字）。
func matchingRegistration() gcis.Registration {
	return gcis.Registration{
		Source: gcis.SourceCompany, TaxID: taxIDOne, Name: "範例小農有限公司", Address: "臺北市中正區中山路２號", SetupDate: time.Date(2015, 3, 4, 0, 0, 0, 0, time.UTC),
		OrganizationType: "有限公司", StatusText: "核准設立",
	}
}

// companySubmission 開店、建立並送出一份公司型申請（名稱、型態、設立日期、地址都填，對得上 matchingRegistration）。
func (f *fixture) companySubmission(t *testing.T, mutate func(*KYCDraftInput)) (int64, Store, KYCApplication) {
	t.Helper()
	accountID, s := f.open(t)
	in := companyDraft(accountID, idRep, taxIDOne, bankAcct)
	in.CompanyRegisterDate, in.CompanyTownCode, in.CompanyAddressDetail = ptr("2015-03-04"), ptr("63000050"), ptr("中山路 2 號")
	if mutate != nil {
		mutate(&in)
	}
	d := f.savedDraft(t, in)
	sub, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	return accountID, s, sub
}

// checkIDs 回傳申請的全部登記資料查詢，依 seq。
func (f *fixture) checkIDs(t *testing.T, applicationID uuid.UUID) []uuid.UUID {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT check_id::text FROM seller.kyc_registration_checks WHERE application_id = $1 ORDER BY seq", applicationID)
	must(t, err)
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var s string
		must(t, rows.Scan(&s))
		id, err := uuid.Parse(s)
		must(t, err)
		out = append(out, id)
	}
	must(t, rows.Err())
	return out
}

// requery 是後台重新查詢，只看有沒有錯誤（回傳的進行中的查詢另有測試看）。
func (f *fixture) requery(ctx context.Context, in RequeryKYCRegistrationInput) error {
	_, err := f.module.RequeryKYCRegistration(ctx, in)
	return err
}

func (f *fixture) runCheck(t *testing.T, id uuid.UUID) {
	t.Helper()
	must(t, f.module.RunKYCRegistryCheck(context.Background(), id, false))
}

func (f *fixture) adminView(t *testing.T, id uuid.UUID) KYCAdminApplication {
	t.Helper()
	a, err := f.module.AdminKYCApplication(context.Background(), id)
	must(t, err)
	return a
}

// 公司型送件在同一個 transaction 內建立 PENDING 的查詢並排入 job；個人型不建。
func TestCompanySubmitCreatesAPendingCheckAndEnqueuesTheJob(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, _, sub := f.companySubmission(t, nil)
	ids := f.checkIDs(t, sub.ID)
	if len(ids) != 1 {
		t.Fatalf("checks=%v", ids)
	}
	var outcome, kind string
	var seq int64
	var admin *int64
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT outcome, request_kind, seq, requested_by_admin_id FROM seller.kyc_registration_checks WHERE check_id=$1", ids[0]).Scan(&outcome, &kind, &seq, &admin))
	if outcome != "PENDING" || kind != "SUBMIT" || seq != 1 || admin != nil {
		t.Fatalf("outcome=%s kind=%s seq=%d admin=%v", outcome, kind, seq, admin)
	}
	if f.jobs.count("seller.kyc_registry_check") != 1 {
		t.Fatalf("jobs=%v", f.jobs.kinds())
	}
	if f.registry.callCount() != 0 {
		t.Fatal("送件的 transaction 內不該呼叫資料來源")
	}
	_, _, individual := f.submitted(t, idA, bankAcct)
	if got := f.checkIDs(t, individual.ID); len(got) != 0 {
		t.Fatalf("個人型建立了查詢：%v", got)
	}
	if f.jobs.count("seller.kyc_registry_check") != 1 {
		t.Fatal("個人型排入了查詢 job")
	}
	// 查詢還沒完成：後台沒有結果可顯示，但標示查詢中；etag 的查詢序號是 0。
	view := f.adminView(t, sub.ID)
	if view.RegistrationCheck != nil || view.PendingRegistrationCheck == nil {
		t.Fatalf("view=%+v pending=%v", view.RegistrationCheck, (view.PendingRegistrationCheck != nil))
	}
}

// job 查到登記資料：存快照、逐欄比對。寫法不同（臺與台、全形數字與空白）不算不一致；不一致與無法比對的欄位各自記錄。
func TestRegistryCheckFoundStoresTheSnapshotAndComparesField(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.now = time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	_, _, sub := f.companySubmission(t, nil)
	id := f.checkIDs(t, sub.ID)[0]
	reg := matchingRegistration()
	reg.Name = "範例小農有限公司"
	reg.SuspendedFrom, reg.SuspendedUntil = time.Date(1993, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(1994, 3, 31, 0, 0, 0, 0, time.UTC)
	f.registry.reply(reg, true, nil)
	f.runCheck(t, id)
	view := f.adminView(t, sub.ID)
	c := view.RegistrationCheck
	if c == nil || c.Outcome != RegistrationFound || c.Seq != 1 || c.QueriedAt == nil || !c.QueriedAt.Equal(f.now) || view.PendingRegistrationCheck != nil {
		t.Fatalf("check=%+v pending=%v", c, (view.PendingRegistrationCheck != nil))
	}
	r := c.Registration
	if r == nil || r.TaxID != taxIDOne || r.Address != "臺北市中正區中山路２號" || r.TownCode != "63000050" || r.AddressDetail != "中山路２號" ||
		r.Status != RegistrationStatusActive || r.StatusText != "核准設立" || r.OrganizationType != "有限公司" ||
		!r.SetupDate.Equal(reg.SetupDate) || !r.SuspendedFrom.Equal(reg.SuspendedFrom) || !r.SuspendedUntil.Equal(reg.SuspendedUntil) {
		t.Fatalf("registration=%+v", r)
	}
	if len(c.MismatchedFields) != 0 || len(c.UnverifiableFields) != 0 {
		t.Fatalf("寫法不同不該算不一致：mismatched=%v unverifiable=%v", c.MismatchedFields, c.UnverifiableFields)
	}
	if got := f.registry.callCount(); got != 1 {
		t.Fatalf("calls=%d", got)
	}
}

func TestRegistryCheckRecordsMismatchedAndUnverifiableFields(t *testing.T) {
	t.Parallel()
	f := setup(t)
	// 申請：名稱與登記不同、設立日期與登記不同、地址的鄉鎮市區與登記不同。
	_, _, sub := f.companySubmission(t, func(in *KYCDraftInput) {
		in.CompanyName = ptr("範例農產有限公司")
		in.CompanyRegisterDate = ptr("2015-03-05")
		in.CompanyTownCode = ptr("63000040")
	})
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[0])
	c := f.adminView(t, sub.ID).RegistrationCheck
	want := []string{"business.company_name", "business.company_register_date", "business.company_address"}
	if c == nil || len(c.MismatchedFields) != 3 || len(c.UnverifiableFields) != 0 {
		t.Fatalf("check=%+v", c)
	}
	for i, field := range want {
		if c.MismatchedFields[i] != field {
			t.Fatalf("mismatched=%v，要 %v", c.MismatchedFields, want)
		}
	}
	if c.Registration.OrganizationType != matchingRegistration().OrganizationType {
		t.Fatalf("快照保留登記的組織型態：%+v", c.Registration)
	}
	// 無法比對：地址拆不開（科學園區）、來源沒有設立日期；來源沒有組織型態也不產生比對結果（申請不填組織型態）。
	_, _, sub2 := f.companySubmission(t, nil)
	reg := matchingRegistration()
	reg.Address, reg.SetupDate, reg.OrganizationType = "新竹科學園區新竹市力行六路8號", time.Time{}, ""
	f.registry.reply(reg, true, nil)
	f.runCheck(t, f.checkIDs(t, sub2.ID)[0])
	c = f.adminView(t, sub2.ID).RegistrationCheck
	if c == nil || len(c.MismatchedFields) != 0 || len(c.UnverifiableFields) != 2 {
		t.Fatalf("check=%+v", c)
	}
	if c.Registration.TownCode != "" || c.Registration.AddressDetail != "" || c.Registration.Address != "新竹科學園區新竹市力行六路8號" {
		t.Fatalf("拆不開時只有原文：%+v", c.Registration)
	}
	// 申請沒有填的欄位（日期、地址）也是無法比對，不是不一致。
	_, _, sub3 := f.companySubmission(t, func(in *KYCDraftInput) {
		in.CompanyRegisterDate, in.CompanyTownCode, in.CompanyAddressDetail = nil, nil, nil
	})
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, f.checkIDs(t, sub3.ID)[0])
	c = f.adminView(t, sub3.ID).RegistrationCheck
	if c == nil || len(c.MismatchedFields) != 0 || len(c.UnverifiableFields) != 2 {
		t.Fatalf("check=%+v", c)
	}
}

// 未知的現況落在 UNKNOWN 並保留原文；不一致只記錄，申請的狀態不變。
func TestRegistryCheckUnknownStatusKeepsTheTextAndNeverChangesTheApplication(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, _, sub := f.companySubmission(t, func(in *KYCDraftInput) { in.CompanyName = ptr("完全不同的公司") })
	reg := matchingRegistration()
	reg.StatusText = "未來新增的現況"
	f.registry.reply(reg, true, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[0])
	view := f.adminView(t, sub.ID)
	if view.RegistrationCheck.Registration.Status != RegistrationStatusUnknown || view.RegistrationCheck.Registration.StatusText != "未來新增的現況" {
		t.Fatalf("registration=%+v", view.RegistrationCheck.Registration)
	}
	if view.Status != KYCStatusSubmitted || view.Version != sub.Version {
		t.Fatalf("比對改動了申請：status=%s version=%d", view.Status, view.Version)
	}
	// 對照以外的值（空字串）不是有效的分類：落在 UNKNOWN。
	_, _, sub2 := f.companySubmission(t, nil)
	reg.StatusText = ""
	f.registry.reply(reg, true, nil)
	f.runCheck(t, f.checkIDs(t, sub2.ID)[0])
	if got := f.adminView(t, sub2.ID).RegistrationCheck.Registration.Status; got != RegistrationStatusUnknown {
		t.Fatalf("status=%q", got)
	}
}

func TestRegistryCheckNotFound(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, _, sub := f.companySubmission(t, nil)
	f.registry.reply(gcis.Registration{}, false, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[0])
	c := f.adminView(t, sub.ID).RegistrationCheck
	if c == nil || c.Outcome != RegistrationNotFound || c.Registration != nil || len(c.MismatchedFields) != 0 || c.QueriedAt == nil {
		t.Fatalf("check=%+v", c)
	}
}

// 暫時失敗：回錯誤讓 River 重試，查詢仍是 PENDING；最後一次嘗試才寫 UNAVAILABLE。
func TestRegistryCheckRetriesTransientFailuresAndWritesUnavailableOnTheFinalAttempt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	id := f.checkIDs(t, sub.ID)[0]
	f.registry.reply(gcis.Registration{}, false, gcis.ErrUnavailable)
	if err := f.module.RunKYCRegistryCheck(ctx, id, false); !errors.Is(err, gcis.ErrUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if view := f.adminView(t, sub.ID); view.RegistrationCheck != nil || view.PendingRegistrationCheck == nil {
		t.Fatalf("重試之前不該寫入結果：%+v", view.RegistrationCheck)
	}
	f.registry.reply(gcis.Registration{}, false, gcis.ErrUnavailable)
	must(t, f.module.RunKYCRegistryCheck(ctx, id, true))
	view := f.adminView(t, sub.ID)
	if c := view.RegistrationCheck; c == nil || c.Outcome != RegistrationUnavailable || c.Registration != nil {
		t.Fatalf("check=%+v", c)
	}
	// 沒有分類的錯誤與呼叫方取消：不是最後一次時不寫入（由 River 重試），最後一次一律寫 UNAVAILABLE，不留 PENDING。
	for name, cause := range map[string]error{"未分類的錯誤": errRegistryDown, "呼叫方取消": context.Canceled} {
		_, _, sub2 := f.companySubmission(t, nil)
		id2 := f.checkIDs(t, sub2.ID)[0]
		f.registry.reply(gcis.Registration{}, false, cause)
		if err := f.module.RunKYCRegistryCheck(ctx, id2, false); !errors.Is(err, cause) {
			t.Fatalf("%s：err=%v", name, err)
		}
		if f.adminView(t, sub2.ID).RegistrationCheck != nil {
			t.Fatalf("%s：不是最後一次就寫入了結果", name)
		}
		f.registry.reply(gcis.Registration{}, false, cause)
		must(t, f.module.RunKYCRegistryCheck(ctx, id2, true))
		if c := f.adminView(t, sub2.ID).RegistrationCheck; c == nil || c.Outcome != RegistrationUnavailable {
			t.Fatalf("%s：最後一次沒有寫 UNAVAILABLE：%+v", name, c)
		}
		if f.adminView(t, sub2.ID).PendingRegistrationCheck != nil {
			t.Fatalf("%s：查詢仍是 PENDING", name)
		}
	}
}

// 統編格式被拒絕不可能發生（草稿時已驗證）：記 error 並寫 UNAVAILABLE（不留 PENDING，也不重試）。
func TestRegistryCheckInvalidTaxIDWritesUnavailableAndLogsAnError(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, _, sub := f.companySubmission(t, nil)
	id := f.checkIDs(t, sub.ID)[0]
	f.registry.reply(gcis.Registration{}, false, gcis.ErrInvalidTaxID)
	must(t, f.module.RunKYCRegistryCheck(context.Background(), id, false))
	view := f.adminView(t, sub.ID)
	if view.RegistrationCheck == nil || view.RegistrationCheck.Outcome != RegistrationUnavailable || view.PendingRegistrationCheck != nil {
		t.Fatalf("check=%+v pending=%v", view.RegistrationCheck, (view.PendingRegistrationCheck != nil))
	}
	if !strings.Contains(f.logs.String(), "拒絕申請上的統編格式") {
		t.Fatalf("沒有記 error：%s", f.logs.String())
	}
}

// 結果只在查詢仍是 PENDING 時寫入：先寫入的結果不被後來的（重複投遞、重試）覆寫。
func TestRegistryCheckResultIsNeverOverwritten(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, _, sub := f.companySubmission(t, nil)
	id := f.checkIDs(t, sub.ID)[0]
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, id)
	late := matchingRegistration()
	late.Name = "後來才到的另一個名稱有限公司"
	f.registry.reply(late, true, nil)
	f.runCheck(t, id) // 已經不是 PENDING：認領就結束，不呼叫資料來源
	if got := f.adminView(t, sub.ID).RegistrationCheck.Registration.Name; got != "範例小農有限公司" {
		t.Fatalf("結果被覆寫：%q", got)
	}
	if f.registry.callCount() != 1 {
		t.Fatalf("已完成的查詢又呼叫了資料來源：%d", f.registry.callCount())
	}
	// 呼叫期間（認領之後、寫入之前）別人先寫入了結果：這次的結果擋下。
	_, _, sub2 := f.companySubmission(t, nil)
	id2 := f.checkIDs(t, sub2.ID)[0]
	slow := matchingRegistration()
	slow.Name = "慢的那一個有限公司"
	f.registry.reply(slow, true, nil)
	f.registry.onLookup = func() {
		ctx := context.Background()
		if _, err := f.db.Conn(ctx).Exec(ctx, `UPDATE seller.kyc_registration_checks SET outcome='NOT_FOUND', queried_at=now() WHERE check_id=$1`, id2); err != nil {
			t.Error(err)
		}
	}
	f.runCheck(t, id2)
	c := f.adminView(t, sub2.ID).RegistrationCheck
	if c == nil || c.Outcome != RegistrationNotFound || c.Registration != nil {
		t.Fatalf("先寫入的結果被覆寫：%+v", c)
	}
}

// 後台顯示 seq 最大的已完成查詢；重新查詢把這一輪還沒查完的查詢取代（CANCELLED），所以被取代的查詢不會晚於新的完成。
func TestAdminViewShowsTheLatestCompletedCheckAndRequerySupersedesAPendingOne(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	first := f.checkIDs(t, sub.ID)[0]
	before := f.jobs.count("seller.kyc_registry_check")
	must(t, f.requery(ctx, RequeryKYCRegistrationInput{ApplicationID: sub.ID, AdminID: 7}))
	ids := f.checkIDs(t, sub.ID)
	if len(ids) != 2 || f.jobs.count("seller.kyc_registry_check") != before+1 {
		t.Fatalf("checks=%v jobs=%d", ids, f.jobs.count("seller.kyc_registry_check"))
	}
	if got := f.checkOutcome(t, first); got != "CANCELLED" {
		t.Fatalf("被取代的查詢：%s", got)
	}
	second := ids[1]
	view := f.adminView(t, sub.ID)
	if view.RegistrationCheck != nil || view.PendingRegistrationCheck == nil {
		t.Fatalf("第二個查詢中：check=%+v pending=%v", view.RegistrationCheck, (view.PendingRegistrationCheck != nil))
	}
	// 被取代的舊 job 之後認領：不是 PENDING，直接結束，不呼叫資料來源，也不覆寫。
	calls := f.registry.callCount()
	f.runCheck(t, first)
	if f.registry.callCount() != calls || f.checkOutcome(t, first) != "CANCELLED" {
		t.Fatalf("被取代的舊 job 動了資料：calls=%d→%d outcome=%s", calls, f.registry.callCount(), f.checkOutcome(t, first))
	}
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, second)
	view = f.adminView(t, sub.ID)
	if view.RegistrationCheck.Seq != 2 || view.RegistrationCheck.Outcome != RegistrationFound || view.PendingRegistrationCheck != nil {
		t.Fatalf("check=%+v pending=%v", view.RegistrationCheck, (view.PendingRegistrationCheck != nil))
	}
}

func (f *fixture) checkOutcome(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var outcome string
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT outcome FROM seller.kyc_registration_checks WHERE check_id=$1", id).Scan(&outcome))
	return outcome
}

// 被取代的舊查詢在呼叫期間（認領之後、寫入之前）被重新查詢取代：它的結果寫不進去（不是 PENDING），顯示的是新的。
func TestASupersededCheckCannotWriteItsResult(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	old := f.checkIDs(t, sub.ID)[0]
	slow := matchingRegistration()
	slow.Name = "舊查詢的名稱有限公司"
	f.registry.reply(slow, true, nil)
	f.registry.onLookup = func() {
		if err := f.requery(ctx, RequeryKYCRegistrationInput{ApplicationID: sub.ID, AdminID: 7}); err != nil {
			t.Error(err)
		}
	}
	f.runCheck(t, old)
	if got := f.checkOutcome(t, old); got != "CANCELLED" {
		t.Fatalf("舊查詢的結果寫進去了：%s", got)
	}
	if v := f.adminView(t, sub.ID); v.RegistrationCheck != nil || v.PendingRegistrationCheck == nil {
		t.Fatalf("view=%+v", v.RegistrationCheck)
	}
}

// 認領時同一輪已有 seq 更大的查詢：寫 CANCELLED 結束（沒有被重新查詢取代、卻留下的舊 PENDING 也會被收掉）。
func TestClaimingAQuerySupersededByANewerOneCancelsIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	old := f.checkIDs(t, sub.ID)[0]
	if _, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, deadline_at, submission_seq)
		SELECT uuidv7(), application_id, 2, 'SUBMIT', now(), now() + interval '15 minutes', submission_seq FROM seller.kyc_registration_checks WHERE check_id = $1`, old); err != nil {
		t.Fatal(err)
	}
	calls := f.registry.callCount()
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, old)
	if got := f.checkOutcome(t, old); got != "CANCELLED" || f.registry.callCount() != calls {
		t.Fatalf("outcome=%s calls=%d→%d", got, calls, f.registry.callCount())
	}
}

// 已取消的申請不顯示「查詢中」：只有審核中（SUBMITTED）才算。
func TestACancelledApplicationShowsNoPendingCheck(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.companySubmission(t, nil)
	if v := f.adminView(t, sub.ID); v.PendingRegistrationCheck == nil {
		t.Fatal("審核中的申請應該顯示查詢中")
	}
	_, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: sub.Version})
	must(t, err)
	if v := f.adminView(t, sub.ID); v.Status != KYCStatusCancelled || v.PendingRegistrationCheck != nil {
		t.Fatalf("status=%s pending=%v", v.Status, (v.PendingRegistrationCheck != nil))
	}
}

// 輪次：同一輪的多次重新查詢綁定同一個 submission_seq；退回後重送是新的一輪（加一）。
func TestChecksAreBoundToTheSubmissionRound(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.companySubmission(t, nil)
	for range 2 {
		must(t, f.requery(ctx, RequeryKYCRegistrationInput{ApplicationID: sub.ID, AdminID: 7}))
	}
	rounds := func() []int64 {
		rows, err := f.db.Conn(ctx).Query(ctx, "SELECT submission_seq FROM seller.kyc_registration_checks WHERE application_id=$1 ORDER BY seq", sub.ID)
		must(t, err)
		defer rows.Close()
		var out []int64
		for rows.Next() {
			var n int64
			must(t, rows.Scan(&n))
			out = append(out, n)
		}
		return out
	}
	if got := rounds(); !slices.Equal(got, []int64{1, 1, 1}) {
		t.Fatalf("同一輪的查詢：%v", got)
	}
	f.resubmit(t, accountID, sub)
	if got := rounds(); !slices.Equal(got, []int64{1, 1, 1, 2}) {
		t.Fatalf("重送之後：%v", got)
	}
	var seq int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT submission_seq FROM seller.kyc_applications WHERE application_id=$1", sub.ID).Scan(&seq))
	if seq != 2 {
		t.Fatalf("申請的輪次=%d", seq)
	}
}

// 重新查詢只在 SUBMITTED、公司型時；個人型與已決定的回 FailedPrecondition，草稿與不存在是 NotFound。
func TestRequeryRegistrationRules(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	requery := func(id uuid.UUID) error {
		return f.requery(ctx, RequeryKYCRegistrationInput{ApplicationID: id, AdminID: 7})
	}
	_, _, individual := f.submitted(t, idA, bankAcct)
	if err := requery(individual.ID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("個人型：%v", err)
	}
	accountID, _ := f.open(t)
	draft := f.savedDraft(t, companyDraft(accountID, idRep, taxIDOne, "999888777666"))
	if err := requery(draft.ID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("草稿：%v", err)
	}
	if err := requery(uuid.NewV7()); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在：%v", err)
	}
	_, _, sub := f.companySubmission(t, nil)
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[0])
	before := f.jobs.count("seller.kyc_registry_check")
	must(t, requery(sub.ID)) // 沒有查詢中的 → 新增第二個（沒有東西可取代）
	if got := f.checkIDs(t, sub.ID); len(got) != 2 || f.jobs.count("seller.kyc_registry_check") != before+1 {
		t.Fatalf("checks=%v jobs=%d", got, f.jobs.count("seller.kyc_registry_check"))
	}
	var kind string
	var admin int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT request_kind, requested_by_admin_id FROM seller.kyc_registration_checks WHERE seq=2 AND application_id=$1", sub.ID).Scan(&kind, &admin))
	if kind != "ADMIN_REQUERY" || admin != 7 {
		t.Fatalf("kind=%s admin=%d", kind, admin)
	}
	must(t, requery(sub.ID)) // 這一輪已有查詢中的 → 取代它（CANCELLED）再新增第三個
	got := f.checkIDs(t, sub.ID)
	if len(got) != 3 || f.checkOutcome(t, got[1]) != "CANCELLED" || f.checkOutcome(t, got[2]) != "PENDING" {
		t.Fatalf("取代失敗：%v", got)
	}
	if err := f.requery(ctx, RequeryKYCRegistrationInput{ApplicationID: sub.ID}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("沒有管理員：%v", err)
	}
	// 核准之後不能再查。
	view := f.adminView(t, sub.ID)
	_, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: view.Version, ExpectedCheckSeq: view.RegistrationCheckSeq(), AdminID: 9})
	must(t, err)
	if err := requery(sub.ID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("已核准：%v", err)
	}
}

// 組合 etag：顯示的查詢變了（新的已完成結果）而 etag 是舊的，核准與退回回 aborted；用新的 etag 成功，審核紀錄記下依據的查詢。
func TestApproveWithAStaleCheckSeqConflictsAndTheNewOneRecordsTheCheck(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[0])
	seen := f.adminView(t, sub.ID) // 審核員看到 seq 1
	if seen.RegistrationCheckSeq() != 1 {
		t.Fatalf("seq=%d", seen.RegistrationCheckSeq())
	}
	// 之後重新查詢完成了 seq 2。
	must(t, f.requery(ctx, RequeryKYCRegistrationInput{ApplicationID: sub.ID, AdminID: 7}))
	f.registry.reply(gcis.Registration{}, false, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[1])
	_, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: seen.Version, ExpectedCheckSeq: 1, AdminID: 9})
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("approve with the stale seq: %v", err)
	}
	_, err = f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: seen.Version, ExpectedCheckSeq: 1, AdminID: 9, Issues: anIssue(), Reason: "查無登記"})
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("reject with the stale seq: %v", err)
	}
	if got := f.adminView(t, sub.ID).Status; got != KYCStatusSubmitted {
		t.Fatalf("衝突時不該寫入：%s", got)
	}
	if f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action IN ('APPROVE','REJECT')") != 0 {
		t.Fatal("衝突時寫了審核紀錄")
	}
	fresh := f.adminView(t, sub.ID)
	if fresh.RegistrationCheckSeq() != 2 || fresh.RegistrationCheck.Outcome != RegistrationNotFound {
		t.Fatalf("fresh=%+v", fresh.RegistrationCheck)
	}
	approved, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: fresh.Version, ExpectedCheckSeq: 2, AdminID: 9})
	must(t, err)
	var refSeq int64
	must(t, f.db.Conn(ctx).QueryRow(ctx,
		"SELECT c.seq FROM seller.kyc_reviews r JOIN seller.kyc_registration_checks c ON c.check_id = r.registration_check_id WHERE r.application_id=$1 AND r.action='APPROVE'", sub.ID).Scan(&refSeq))
	if refSeq != 2 {
		t.Fatalf("核准依據的查詢 seq=%d", refSeq)
	}
	// 其他審核紀錄（送件）沒有引用；核准的那一筆帶出依據的序號與查詢時間。
	var approve KYCReview
	for _, r := range approved.Reviews {
		if r.Action == ReviewApprove {
			approve = r
		} else if r.RegistrationCheckSeq != 0 || r.RegistrationCheckQueriedAt != nil {
			t.Fatalf("%s 不該有依據：%+v", r.Action, r)
		}
	}
	if approve.RegistrationCheckSeq != 2 || approve.RegistrationCheckQueriedAt == nil {
		t.Fatalf("approve review=%+v", approve)
	}
	if approved.RegistrationCheckSeq() != 2 {
		t.Fatalf("回應顯示的查詢 seq=%d", approved.RegistrationCheckSeq())
	}
}

// 沒有查詢、查詢中或不可用時照樣能核准與退回，引用是 NULL；個人型 etag 的查詢序號永遠是 0。
func TestApproveAndRejectWithoutAFinishedCheckRecordNoReference(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil) // 查詢還在 PENDING
	_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, ExpectedCheckSeq: 0, AdminID: 9, Issues: anIssue(), Reason: "資料不齊"})
	must(t, err)
	_, _, individual := f.submitted(t, idB, "999888777666")
	_, err = f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: individual.ID, ExpectedVersion: individual.Version, AdminID: 9})
	must(t, err)
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action IN ('APPROVE','REJECT') AND registration_check_id IS NULL"); n != 2 {
		t.Fatalf("沒有引用的決定=%d", n)
	}
	_, _, unavailable := f.companySubmission(t, nil)
	f.registry.reply(gcis.Registration{}, false, gcis.ErrUnavailable)
	must(t, f.module.RunKYCRegistryCheck(ctx, f.checkIDs(t, unavailable.ID)[0], true))
	view := f.adminView(t, unavailable.ID)
	_, err = f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: unavailable.ID, ExpectedVersion: view.Version, ExpectedCheckSeq: view.RegistrationCheckSeq(), AdminID: 9})
	must(t, err)
	var outcome string
	must(t, f.db.Conn(ctx).QueryRow(ctx,
		"SELECT c.outcome FROM seller.kyc_reviews r JOIN seller.kyc_registration_checks c ON c.check_id = r.registration_check_id WHERE r.application_id=$1 AND r.action='APPROVE'", unavailable.ID).Scan(&outcome))
	if outcome != "UNAVAILABLE" {
		t.Fatalf("outcome=%s", outcome)
	}
}

// 個資清理：全部查詢列與申請的個資同時刪除，審核紀錄保留、引用置 NULL。
func TestPurgeDeletesRegistrationChecksAndKeepsTheReviews(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.companySubmission(t, nil)
	checkID := f.checkIDs(t, sub.ID)[0]
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, checkID)
	view := f.adminView(t, sub.ID)
	_, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: view.Version, ExpectedCheckSeq: view.RegistrationCheckSeq(), AdminID: 9})
	must(t, err)
	if f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE registration_check_id IS NOT NULL") != 1 {
		t.Fatal("核准沒有引用查詢")
	}
	if f.count(t, "SELECT count(*) FROM seller.kyc_registration_checks WHERE name IS NOT NULL") != 1 {
		t.Fatal("測試沒有留下登記資料")
	}
	_, err = f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 {
		t.Fatalf("purged=%d", n)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.kyc_registration_checks"); got != 0 {
		t.Fatalf("清理之後還有 %d 列登記資料查詢", got)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE application_id=$1 AND registration_check_id IS NULL", sub.ID); got < 2 {
		t.Fatalf("審核紀錄被刪或引用沒有置 NULL：%d", got)
	}
	// 已清理的申請上還在途的 job 找不到查詢列：結束，不呼叫資料來源、不是錯誤。
	calls := f.registry.callCount()
	must(t, f.module.RunKYCRegistryCheck(ctx, checkID, false))
	if f.registry.callCount() != calls {
		t.Fatal("清理之後呼叫了資料來源")
	}
}

// 查詢的 job 以 check_id 為 River 的唯一鍵（以真的 River 排入）：每筆查詢（送件、每次重新查詢）各有一個 job；掃描對還在排隊的 job
// 不會重複排入，job 被丟棄之後才補排新的；同一個 check_id 的重複投遞由「仍是 PENDING 才寫」擋下，不會覆寫已寫入的結果。
func TestRegistryCheckJobsAreUniqueByCheckIDAndDuplicateDeliveryIsHarmless(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	must(t, jobs.Migrate(ctx, f.db))
	jc, err := jobs.NewInsertOnly(f.db)
	must(t, err)
	f.module = f.newModule(t, f.payuni, jc)
	_, _, sub := f.companySubmission(t, nil)
	must(t, f.requery(ctx, RequeryKYCRegistrationInput{ApplicationID: sub.ID, AdminID: 7}))
	const registryJobs = "SELECT count(*) FROM river.river_job WHERE kind='seller.kyc_registry_check'"
	if n := f.count(t, registryJobs); n != 2 {
		t.Fatalf("送件與重新查詢各一個 job：%d", n)
	}
	if args := (KYCRegistryCheckArgs{}).InsertOpts(); !args.UniqueOpts.ByArgs || slices.Contains(args.UniqueOpts.ByState, "discarded") {
		t.Fatalf("唯一鍵：%+v", args.UniqueOpts)
	}
	ids := f.checkIDs(t, sub.ID)
	// 掃描：第二筆閒置超過門檻，但它的 job 還在排隊，River 的唯一鍵讓補排不重複。
	f.now = f.now.Add(4 * time.Minute)
	_, resynced, err := f.module.SweepKYCRegistryChecks(ctx)
	must(t, err)
	if resynced != 0 || f.count(t, registryJobs) != 2 {
		t.Fatalf("job 還在，補排被唯一鍵略過、不該新增也不該計入：resynced=%d jobs=%d", resynced, f.count(t, registryJobs))
	}
	// job 被丟棄（River 用盡重試或 rescuer 丟棄）：PENDING 還在，補排會新增一個 job。
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE river.river_job SET state='discarded', finalized_at=now() WHERE kind='seller.kyc_registry_check' AND args->>'check_id' = $1", ids[1].String()); err != nil {
		t.Fatal(err)
	}
	_, resynced, err = f.module.SweepKYCRegistryChecks(ctx)
	must(t, err)
	if resynced != 1 {
		t.Fatalf("被丟棄的 job 補排一個：resynced=%d", resynced)
	}
	if n := f.count(t, "SELECT count(*) FROM river.river_job WHERE kind='seller.kyc_registry_check' AND args->>'check_id' = '"+ids[1].String()+"' AND state='available'"); n != 1 {
		t.Fatalf("被丟棄的 job 沒有補排：%d", n)
	}
	// 同一個 check_id 重複投遞（兩次執行）：第二次已不是 PENDING，結果不變。
	id := ids[1]
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, id)
	other := matchingRegistration()
	other.Name = "重複投遞的另一個名稱有限公司"
	f.registry.reply(other, true, nil)
	f.runCheck(t, id)
	if got := f.adminView(t, sub.ID).RegistrationCheck.Registration.Name; got != "範例小農有限公司" {
		t.Fatalf("重複投遞覆寫了結果：%q", got)
	}
}

// schema 的 CHECK：FOUND 一定有登記資料、其他結果一定沒有；比對的欄位只能是四個路徑且兩個清單不重疊；
// 審核紀錄只有核准與退回能引用查詢。
func TestRegistryCheckSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	other, _, individual := f.submitted(t, idB, "999888777666")
	_ = other
	id := f.checkIDs(t, sub.ID)[0]
	for name, query := range map[string]string{
		"found with a bad status":                      "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), tax_id='33330333', name='x', address_text='x', status='SUSPENDED', status_text='停業'",
		"found with an empty status":                   "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), tax_id='33330333', name='x', address_text='x', status='', status_text=''",
		"not found with a registration":                "UPDATE seller.kyc_registration_checks SET outcome='NOT_FOUND', queried_at=now(), name='x'",
		"pending with a query time":                    "UPDATE seller.kyc_registration_checks SET queried_at=now()",
		"completed without a query time":               "UPDATE seller.kyc_registration_checks SET outcome='UNAVAILABLE'",
		"unknown outcome":                              "UPDATE seller.kyc_registration_checks SET outcome='DONE', queried_at=now()",
		"unknown field":                                "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), tax_id='33330333', name='x', address_text='x', status='ACTIVE', status_text='', mismatched_fields='{business.tax_id}'",
		"overlapping fields":                           "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), tax_id='33330333', name='x', address_text='x', status='ACTIVE', status_text='', mismatched_fields='{business.company_name}', unverifiable_fields='{business.company_name}'",
		"town without a detail":                        "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), tax_id='33330333', name='x', address_text='x', status='ACTIVE', status_text='', town_code='63000050'",
		"suspension ending before it began":            "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), tax_id='33330333', name='x', address_text='x', status='ACTIVE', status_text='', suspended_from='2020-02-01', suspended_until='2020-01-01'",
		"admin kind without an admin":                  "UPDATE seller.kyc_registration_checks SET request_kind='ADMIN_REQUERY'",
		"submit kind with an admin":                    "UPDATE seller.kyc_registration_checks SET requested_by_admin_id=1",
		"duplicate seq":                                "INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, deadline_at, submission_seq) SELECT gen_random_uuid(), application_id, 1, 'SUBMIT', now(), now() + interval '15 minutes', submission_seq FROM seller.kyc_registration_checks",
		"a deadline not after the request":             "UPDATE seller.kyc_registration_checks SET deadline_at = requested_at",
		"a check without a deadline":                   "UPDATE seller.kyc_registration_checks SET deadline_at = NULL",
		"a review of another action refers to a check": "UPDATE seller.kyc_reviews SET registration_check_id='" + id.String() + "' WHERE action='SUBMIT'",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, query); err == nil {
			t.Errorf("%s：資料庫沒有擋下", name)
		}
	}
	_ = individual
	// Go 的每個現況分類都是 DB 接受的值（FOUND 的 status 沒有第二份清單）。
	for _, status := range RegistrationStatusValues() {
		query := "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), tax_id='33330333', name='x', address_text='x', status_text='', status='" + string(status) + "'"
		if _, err := f.db.Conn(ctx).Exec(ctx, query); err != nil {
			t.Errorf("DB 不接受現況分類 %s：%v", status, err)
		}
	}
}

// FOUND 一定有統編、名稱與現況分類：欄位是 NULL 時 CHECK 也要擋（SQL 的 CHECK 對 NULL 的比較結果是 NULL，會放行，所以條件要明寫
// IS NOT NULL）。
func TestFoundCheckRequiresTheTaxIDNameAndStatus(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.companySubmission(t, nil)
	for name, query := range map[string]string{
		"found without a tax id": "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), name='x', address_text='x', status='ACTIVE', status_text=''",
		"found without a name":   "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), tax_id='33330333', address_text='x', status='ACTIVE', status_text=''",
		"found without a status": "UPDATE seller.kyc_registration_checks SET outcome='FOUND', queried_at=now(), tax_id='33330333', name='x', address_text='x', status_text=''",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, query); err == nil {
			t.Errorf("%s：資料庫沒有擋下", name)
		}
	}
}

// resubmit 讓申請被退回、賣家改一個欄位後重送（時鐘往前走：送出時間是這次送件的身分）。
func (f *fixture) resubmit(t *testing.T, accountID int64, sub KYCApplication) KYCApplication {
	t.Helper()
	ctx := context.Background()
	view := f.adminView(t, sub.ID)
	_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: view.Version, ExpectedCheckSeq: view.RegistrationCheckSeq(), AdminID: 9, Issues: anIssue(), Reason: "請補件"})
	must(t, err)
	f.now = f.now.Add(time.Hour)
	rejected, err := f.module.AdminKYCApplication(ctx, sub.ID)
	must(t, err)
	draft, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: rejected.Version, ProductType: ptr("水果")})
	must(t, err)
	f.now = f.now.Add(time.Hour)
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: draft.Version})
	must(t, err)
	return again
}

// 退回後重送沿用同一筆申請：新一輪的查詢完成前，後台沒有快照（上一輪的不顯示成這一輪的依據），核准引用 NULL；新一輪完成後顯示新的。
func TestResubmittedApplicationDoesNotShowThePreviousRoundsSnapshot(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.companySubmission(t, nil)
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[0])
	if f.adminView(t, sub.ID).RegistrationCheckSeq() != 1 {
		t.Fatal("第一輪沒有快照")
	}
	again := f.resubmit(t, accountID, sub)
	ids := f.checkIDs(t, sub.ID)
	if len(ids) != 2 {
		t.Fatalf("checks=%v", ids)
	}
	view := f.adminView(t, again.ID)
	if view.RegistrationCheck != nil || view.RegistrationCheckSeq() != 0 || view.PendingRegistrationCheck == nil {
		t.Fatalf("第二輪完成前顯示了舊的快照：%+v pending=%v", view.RegistrationCheck, (view.PendingRegistrationCheck != nil))
	}
	// 第二輪完成前核准：etag 的查詢序號是 0，引用是 NULL。
	approved, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: again.ID, ExpectedVersion: view.Version, ExpectedCheckSeq: 0, AdminID: 9})
	must(t, err)
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE application_id=$1 AND action='APPROVE' AND registration_check_id IS NULL", again.ID); n != 1 {
		t.Fatalf("核准的引用不是 NULL（%d）", n)
	}
	if approved.RegistrationCheck != nil {
		t.Fatalf("approved=%+v", approved.RegistrationCheck)
	}
}

func TestResubmittedApplicationShowsTheNewRoundOnceItCompletes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, _, sub := f.companySubmission(t, nil)
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[0])
	f.resubmit(t, accountID, sub)
	newer := matchingRegistration()
	newer.Name = "第二輪的名稱有限公司"
	f.registry.reply(newer, true, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[1])
	view := f.adminView(t, sub.ID)
	if view.RegistrationCheckSeq() != 2 || view.RegistrationCheck.Registration.Name != "第二輪的名稱有限公司" || view.PendingRegistrationCheck != nil {
		t.Fatalf("check=%+v pending=%v", view.RegistrationCheck, (view.PendingRegistrationCheck != nil))
	}
}

// 退回（或重送）之後才被認領的舊查詢：寫成 CANCELLED，不呼叫資料來源，不出現在檢視，也不成為下一輪的快照。
func TestClaimingAQueryAfterTheRoundEndedCancelsIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	// 退回之後（申請是 REJECTED）才認領。
	_, _, sub := f.companySubmission(t, nil)
	first := f.checkIDs(t, sub.ID)[0]
	view := f.adminView(t, sub.ID)
	_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: view.Version, AdminID: 9, Issues: anIssue(), Reason: "資料不齊"})
	must(t, err)
	calls := f.registry.callCount()
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, first)
	var outcome string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT outcome FROM seller.kyc_registration_checks WHERE check_id=$1", first).Scan(&outcome))
	if outcome != "CANCELLED" || f.registry.callCount() != calls {
		t.Fatalf("outcome=%s calls=%d→%d", outcome, calls, f.registry.callCount())
	}
	if v := f.adminView(t, sub.ID); v.RegistrationCheck != nil {
		t.Fatalf("CANCELLED 的查詢出現在檢視：%+v", v.RegistrationCheck)
	}
	// 退回、重送之後才認領上一輪的查詢（送出時間與查詢的不同）：同樣 CANCELLED，新一輪的查詢不受影響。
	account2, _, sub2 := f.companySubmission(t, nil)
	stale := f.checkIDs(t, sub2.ID)[0]
	f.resubmit(t, account2, sub2)
	calls = f.registry.callCount()
	f.runCheck(t, stale)
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT outcome FROM seller.kyc_registration_checks WHERE check_id=$1", stale).Scan(&outcome))
	if outcome != "CANCELLED" || f.registry.callCount() != calls {
		t.Fatalf("上一輪的查詢：outcome=%s", outcome)
	}
	var current string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT outcome FROM seller.kyc_registration_checks WHERE application_id=$1 AND seq=2", sub2.ID).Scan(&current))
	if current != "PENDING" {
		t.Fatalf("新一輪的查詢被動到：%s", current)
	}
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, f.checkIDs(t, sub2.ID)[1])
	if v := f.adminView(t, sub2.ID); v.RegistrationCheckSeq() != 2 {
		t.Fatalf("新一輪的快照：%+v", v.RegistrationCheck)
	}
}

// 重送之後可以重新查詢：上一輪還在 PENDING 的查詢不擋也不被動到（它只在認領時被放棄）；這一輪有查詢中的會被取代。
func TestRequeryAfterResubmitIsNotBlockedByThePreviousRoundsPendingCheck(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.companySubmission(t, nil)
	f.resubmit(t, accountID, sub) // 第一輪的查詢仍是 PENDING；第二輪排了自己的
	ids := f.checkIDs(t, sub.ID)
	if len(ids) != 2 {
		t.Fatalf("checks=%v", ids)
	}
	must(t, f.requery(ctx, RequeryKYCRegistrationInput{ApplicationID: sub.ID, AdminID: 7}))
	got := f.checkIDs(t, sub.ID)
	if len(got) != 3 {
		t.Fatalf("checks=%v", got)
	}
	if f.checkOutcome(t, got[0]) != "PENDING" || f.checkOutcome(t, got[1]) != "CANCELLED" || f.checkOutcome(t, got[2]) != "PENDING" {
		t.Fatalf("上一輪應原封不動、這一輪被取代：%s %s %s", f.checkOutcome(t, got[0]), f.checkOutcome(t, got[1]), f.checkOutcome(t, got[2]))
	}
}

// 來源回的停業期間顛倒時不存日期、寫入成功（儲存端的 CHECK 不接受迄日早於起日）。
func TestRegistryCheckDropsAnInvertedSuspensionPeriod(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, _, sub := f.companySubmission(t, nil)
	reg := matchingRegistration()
	reg.SuspendedFrom, reg.SuspendedUntil = time.Date(1994, 3, 31, 0, 0, 0, 0, time.UTC), time.Date(1993, 11, 1, 0, 0, 0, 0, time.UTC)
	f.registry.reply(reg, true, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[0])
	c := f.adminView(t, sub.ID).RegistrationCheck
	if c == nil || c.Outcome != RegistrationFound || !c.Registration.SuspendedFrom.IsZero() || !c.Registration.SuspendedUntil.IsZero() {
		t.Fatalf("check=%+v", c)
	}
}

// 互動的統編查詢經模組：與快照同一個轉換入口（現況分類、地址拆分、停業期間的合理性）；查無是 found 為 false，來源不可用是
// Unavailable，沒有分類的錯誤原樣交回；在 transaction 內呼叫被拒絕（外部呼叫）。
func TestLookupBusinessRegistrationThroughTheModule(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	reg := matchingRegistration()
	reg.StatusText = "核准設立，但已命令解散"
	reg.SuspendedFrom, reg.SuspendedUntil = time.Date(1994, 3, 31, 0, 0, 0, 0, time.UTC), time.Date(1993, 11, 1, 0, 0, 0, 0, time.UTC)
	f.registry.reply(reg, true, nil)
	got, found, err := f.module.LookupBusinessRegistration(ctx, taxIDOne)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if got.Status != RegistrationStatusOrderedToDissolve || got.StatusText != "核准設立，但已命令解散" || got.TownCode != "63000050" || got.AddressDetail != "中山路２號" ||
		!got.SuspendedFrom.IsZero() || !got.SuspendedUntil.IsZero() {
		t.Fatalf("registration=%+v", got)
	}
	f.registry.reply(gcis.Registration{}, false, nil)
	if _, found, err := f.module.LookupBusinessRegistration(ctx, taxIDOne); err != nil || found {
		t.Fatalf("查無：found=%v err=%v", found, err)
	}
	f.registry.reply(gcis.Registration{}, false, gcis.ErrUnavailable)
	if _, _, err := f.module.LookupBusinessRegistration(ctx, taxIDOne); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("不可用：%v", err)
	}
	f.registry.reply(gcis.Registration{}, false, errRegistryDown)
	if _, _, err := f.module.LookupBusinessRegistration(ctx, taxIDOne); !errors.Is(err, errRegistryDown) {
		t.Fatalf("沒有分類的錯誤：%v", err)
	}
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		if _, _, err := f.module.LookupBusinessRegistration(ctx, taxIDOne); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("transaction 內呼叫：%v", err)
		}
		return nil
	}))
}

// 「查詢中」只看目前這一輪：上一輪還停在 PENDING 的查詢（job 遺失）不讓這一輪顯示查詢中。
func TestPendingFlagIgnoresThePreviousRoundsPendingCheck(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.companySubmission(t, nil) // 第一輪的查詢永遠停在 PENDING
	f.resubmit(t, accountID, sub)
	ids := f.checkIDs(t, sub.ID)
	f.registry.reply(gcis.Registration{}, false, gcis.ErrUnavailable)
	must(t, f.module.RunKYCRegistryCheck(ctx, ids[1], true)) // 第二輪完成（UNAVAILABLE）
	if f.checkOutcome(t, ids[0]) != "PENDING" {
		t.Fatal("測試前提：上一輪的查詢仍是 PENDING")
	}
	v := f.adminView(t, sub.ID)
	if v.RegistrationCheck == nil || v.RegistrationCheck.Seq != 2 || v.PendingRegistrationCheck != nil {
		t.Fatalf("check=%+v pending=%v", v.RegistrationCheck, (v.PendingRegistrationCheck != nil))
	}
	// 這一輪沒有完成的查詢、也沒有查詢中的（第二輪的查詢被放棄）：上一輪停在 PENDING 的不算。
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.kyc_registration_checks SET outcome='CANCELLED' WHERE check_id = $1", ids[1]); err != nil {
		t.Fatal(err)
	}
	if v := f.adminView(t, sub.ID); v.RegistrationCheck != nil || v.PendingRegistrationCheck != nil {
		t.Fatalf("上一輪的 PENDING 讓這一輪顯示查詢中：check=%+v pending=%v", v.RegistrationCheck, (v.PendingRegistrationCheck != nil))
	}
}

// ---------------------------------------------------------------------------
// 查詢是有期限的資源，伺服器保證在期限前到終態（掃描 job seller.kyc_registry_sweep）。

// 進行中的查詢帶要求時間與期限；期限在重試的最壞總長（約 94 分鐘）之後。
func TestAPendingCheckCarriesItsRequestTimeAndDeadline(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, _, sub := f.companySubmission(t, nil)
	p := f.adminView(t, sub.ID).PendingRegistrationCheck
	if p == nil || p.Outcome != RegistrationPending || p.Seq != 1 || !p.RequestedAt.Equal(f.now) || p.Deadline == nil {
		t.Fatalf("pending=%+v", p)
	}
	if got := p.Deadline.Sub(p.RequestedAt); got != 15*time.Minute {
		t.Fatalf("期限離要求時間 %v，預期 15 分鐘（建立查詢時寫下的 deadline_at）", got)
	}
	var stored time.Time
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT deadline_at FROM seller.kyc_registration_checks WHERE check_id = $1", p.ID).Scan(&stored))
	if !stored.Equal(*p.Deadline) {
		t.Fatalf("後台的期限不是資料列的 deadline_at：%v ≠ %v", p.Deadline, stored)
	}
	// 已完成的查詢不帶期限。
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, f.checkIDs(t, sub.ID)[0])
	if c := f.adminView(t, sub.ID).RegistrationCheck; c == nil || c.Deadline != nil || !c.RequestedAt.Equal(p.RequestedAt) {
		t.Fatalf("check=%+v", c)
	}
}

// 重新查詢回傳新增的這筆進行中的查詢（帶期限），並取代進行中的那一筆（CANCELLED）。
func TestRequeryReturnsTheNewPendingCheckAndSupersedesTheRunningOne(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	first := f.checkIDs(t, sub.ID)[0]
	f.now = f.now.Add(7 * time.Minute)
	got, err := f.module.RequeryKYCRegistration(ctx, RequeryKYCRegistrationInput{ApplicationID: sub.ID, AdminID: 7})
	must(t, err)
	if got.Seq != 2 || got.Outcome != RegistrationPending || !got.RequestedAt.Equal(f.now) || got.Deadline == nil || !got.Deadline.After(got.RequestedAt) {
		t.Fatalf("requery=%+v", got)
	}
	ids := f.checkIDs(t, sub.ID)
	if len(ids) != 2 || got.ID != ids[1] {
		t.Fatalf("ids=%v requery=%v", ids, got.ID)
	}
	if f.checkOutcome(t, first) != "CANCELLED" || f.checkOutcome(t, ids[1]) != "PENDING" {
		t.Fatalf("outcomes=%s %s", f.checkOutcome(t, first), f.checkOutcome(t, ids[1]))
	}
	if p := f.adminView(t, sub.ID).PendingRegistrationCheck; p == nil || p.Seq != 2 || !p.Deadline.Equal(*got.Deadline) {
		t.Fatalf("pending=%+v", p)
	}
}

// job 遺失（程序當掉、River 丟棄）的 PENDING：期限之前掃描不改它的結果，到了期限收成 UNAVAILABLE，不需要任何人按重新查詢。
func TestSweepClosesALostPendingCheckAsUnavailableOnlyAfterItsDeadline(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	id := f.checkIDs(t, sub.ID)[0]
	p := f.adminView(t, sub.ID).PendingRegistrationCheck
	requested, deadline := p.RequestedAt, *p.Deadline

	// 期限之前（掃描提前一個掃描間隔收，所以離期限還有兩分鐘以上）的 PENDING 可能還在正常重試，掃描不能動它。
	f.now = requested.Add(10 * time.Minute)
	expired, _, err := f.module.SweepKYCRegistryChecks(ctx)
	must(t, err)
	if expired != 0 || f.checkOutcome(t, id) != "PENDING" {
		t.Fatalf("期限之前被收掉了：expired=%d outcome=%s", expired, f.checkOutcome(t, id))
	}

	// 掃描最晚在期限之前的最後一次執行收掉它（終態在期限之前）：離期限不到一個掃描間隔時就收成終態。
	f.now = deadline.Add(-30 * time.Second)
	expired, _, err = f.module.SweepKYCRegistryChecks(ctx)
	must(t, err)
	if expired != 1 || f.checkOutcome(t, id) != "UNAVAILABLE" {
		t.Fatalf("期限到了仍不是終態：expired=%d outcome=%s", expired, f.checkOutcome(t, id))
	}
	view := f.adminView(t, sub.ID)
	if view.PendingRegistrationCheck != nil || view.RegistrationCheck == nil || view.RegistrationCheck.Outcome != RegistrationUnavailable ||
		view.RegistrationCheck.QueriedAt == nil || view.RegistrationCheck.Registration != nil {
		t.Fatalf("view check=%+v pending=%+v", view.RegistrationCheck, view.PendingRegistrationCheck)
	}
	// 再掃一次不重複處理。
	expired, _, err = f.module.SweepKYCRegistryChecks(ctx)
	must(t, err)
	if expired != 0 {
		t.Fatalf("expired=%d", expired)
	}
}

// 期限已過才醒來的 job（或重複投遞）不覆寫已收成的終態，也不呼叫資料來源。
func TestALateJobDoesNotOverwriteTheSweptTerminalState(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	id := f.checkIDs(t, sub.ID)[0]
	f.now = f.adminView(t, sub.ID).PendingRegistrationCheck.Deadline.Add(time.Minute)
	_, _, err := f.module.SweepKYCRegistryChecks(ctx)
	must(t, err)
	calls := f.registry.callCount()
	f.registry.reply(matchingRegistration(), true, nil)
	f.runCheck(t, id)
	if f.checkOutcome(t, id) != "UNAVAILABLE" || f.registry.callCount() != calls {
		t.Fatalf("outcome=%s calls=%d→%d", f.checkOutcome(t, id), calls, f.registry.callCount())
	}
}

// 閒置的 PENDING 由掃描補排 job（唯一鍵是 check_id，River 讓還在排隊或重試的 job 不重複）；剛排入的不補排。
func TestSweepRequeuesAnIdlePendingCheckWithTheSameUniqueKey(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.companySubmission(t, nil)
	id := f.checkIDs(t, sub.ID)[0]
	before := f.jobs.count("seller.kyc_registry_check")

	f.now = f.now.Add(2 * time.Minute)
	_, resynced, err := f.module.SweepKYCRegistryChecks(ctx)
	must(t, err)
	if resynced != 0 || f.jobs.count("seller.kyc_registry_check") != before {
		t.Fatalf("剛排入的查詢被補排：resynced=%d", resynced)
	}

	f.now = f.now.Add(2 * time.Minute)
	_, resynced, err = f.module.SweepKYCRegistryChecks(ctx)
	must(t, err)
	if resynced != 1 || f.jobs.count("seller.kyc_registry_check") != before+1 {
		t.Fatalf("閒置的 PENDING 沒有補排：resynced=%d", resynced)
	}
	f.jobs.mu.Lock()
	defer f.jobs.mu.Unlock()
	last := f.jobs.args[len(f.jobs.args)-1]
	opts := f.jobs.opts[len(f.jobs.opts)-1]
	if args, ok := last.(interface{ Kind() string }); !ok || args.Kind() != "seller.kyc_registry_check" || !strings.Contains(fmt.Sprintf("%v", last), id.String()) {
		t.Fatalf("補排的不是同一筆查詢：%v", last)
	}
	if !opts.UniqueOpts.ByArgs {
		t.Fatalf("補排沒有唯一鍵：%+v", opts)
	}
}

// 已經不屬於審核中這一輪的過期 PENDING（申請取消、被更新的取代）收成 CANCELLED，與認領時放棄的語意一致，不假稱「來源不可用」。
func TestSweepCancelsAnExpiredPendingCheckThatIsNoLongerCurrent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.companySubmission(t, nil)
	id := f.checkIDs(t, sub.ID)[0]
	deadline := *f.adminView(t, sub.ID).PendingRegistrationCheck.Deadline
	_, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: sub.Version})
	must(t, err)
	f.now = deadline
	expired, _, err := f.module.SweepKYCRegistryChecks(ctx)
	must(t, err)
	if expired != 1 || f.checkOutcome(t, id) != "CANCELLED" {
		t.Fatalf("expired=%d outcome=%s", expired, f.checkOutcome(t, id))
	}
}

// 過期的 job 不再呼叫資料來源：重試拖過了期限（或 job 很晚才醒來）時，下一次執行直接把查詢記成 UNAVAILABLE，不論是不是最後一次嘗試；
// 期限之前的 job 照常呼叫。
func TestAJobThatRunsAfterTheDeadlineWritesUnavailableWithoutCallingTheRegistry(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, _, sub := f.companySubmission(t, nil)
	id := f.checkIDs(t, sub.ID)[0]
	deadline := *f.adminView(t, sub.ID).PendingRegistrationCheck.Deadline
	f.now = deadline.Add(-time.Second)
	f.registry.reply(matchingRegistration(), true, nil)
	// 期限之前：照常呼叫（這裡讓它失敗以便留下 PENDING，由重試接手）。
	f.registry.replies = nil
	f.registry.reply(gcis.Registration{}, false, gcis.ErrUnavailable)
	if err := f.module.RunKYCRegistryCheck(context.Background(), id, false); !errors.Is(err, gcis.ErrUnavailable) {
		t.Fatalf("期限之前應該呼叫並回錯誤讓 River 重試：%v", err)
	}
	calls := f.registry.callCount()
	if calls != 1 || f.checkOutcome(t, id) != "PENDING" {
		t.Fatalf("calls=%d outcome=%s", calls, f.checkOutcome(t, id))
	}
	// 期限到了：重試的下一次執行（不是最後一次嘗試）直接收成終態，不呼叫資料來源。
	f.now = deadline
	f.registry.reply(matchingRegistration(), true, nil)
	must(t, f.module.RunKYCRegistryCheck(context.Background(), id, false))
	if f.checkOutcome(t, id) != "UNAVAILABLE" || f.registry.callCount() != calls {
		t.Fatalf("outcome=%s calls=%d→%d", f.checkOutcome(t, id), calls, f.registry.callCount())
	}
	if v := f.adminView(t, sub.ID); v.PendingRegistrationCheck != nil || v.RegistrationCheck == nil || v.RegistrationCheck.Outcome != RegistrationUnavailable {
		t.Fatalf("view=%+v", v)
	}
}
