//go:build integration

package seller_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// admin 對店家做的動作的可用性與原因（step10e-5b-design.md）：可用性與方法的前置檢查是同一個函式，句子與錯誤是同一句。

func (f *fixture) beginDeletion(t *testing.T, accountID, storeID int64) {
	t.Helper()
	ctx := context.Background()
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		if err := f.identity.LockAccount(ctx, accountID); err != nil {
			return err
		}
		return f.module.BeginDeletion(ctx, storeID)
	}))
}

// kycReviewStarts 是 review_kyc_application(at) 的結果：店家 → 開始時間。
func (f *fixture) kycReviewStarts(t *testing.T, at time.Time) map[int64]time.Time {
	t.Helper()
	out := map[int64]time.Time{}
	for _, r := range f.reviewOf(t, "review_kyc_application", at) {
		out[*r.StoreID] = *r.Start
		if want := r.Start.Add(168 * time.Hour); !r.Due.Equal(want) {
			t.Fatalf("店家 %d 的期限 %v，應該是開始加 168 小時 %v", *r.StoreID, r.Due, want)
		}
	}
	return out
}

// deadlineOf 是後台詳情（StoreKYC）顯示的審核期限；暫停審核期間是 nil。
func (f *fixture) deadlineOf(t *testing.T, storeID int64) *Deadline {
	t.Helper()
	a, ok, err := f.module.StoreKYC(context.Background(), storeID)
	must(t, err)
	if !ok {
		t.Fatalf("店家 %d 沒有申請", storeID)
	}
	return a.DecideDeadline
}

func (f *fixture) reasonsOf(t *testing.T, a KYCApplication) (approve, reject, requery []StoreActionUnavailableReason) {
	t.Helper()
	v, err := f.module.AdminKYCApplication(context.Background(), a.ID)
	must(t, err)
	return v.ApproveUnavailable, v.RejectUnavailable, v.RequeryUnavailable
}

func (f *fixture) liftedAt(t *testing.T, accountID int64, at time.Time) *time.Time {
	t.Helper()
	var out *time.Time
	ctx := context.Background()
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT seller.account_sanction_lifted_at($1, $2::timestamptz)", accountID, at).Scan(&out))
	return out
}

func (f *fixture) effectiveAt(t *testing.T, accountID int64, at time.Time) bool {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM seller.effective_measures($2::timestamptz) WHERE account_id = $1", accountID, at) > 0
}

// 已關閉的店建立警告、停權與風險措施：GetStore 的原因、CreateMeasure 的錯誤、案件與通報的原因是同一句（StoreActionUnavailableText），
// 而且是 failed_precondition，不是 not_found。
// mutation：案件或通報改回自己的句子 → 句子斷言失敗；CreateMeasure 改回 NotFound → 類別斷言失敗。
func TestCreateMeasureOnAClosedStoreIsFailedPreconditionWithOneSentenceEverywhere(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	open, err := f.module.StoreWithActiveMeasures(ctx, s.StoreID)
	must(t, err)
	if len(open.CreateMeasure) != 0 {
		t.Fatalf("營運中的店可以建立措施：%v", open.CreateMeasure)
	}
	_, err = f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	closed, err := f.module.StoreWithActiveMeasures(ctx, s.StoreID)
	must(t, err)
	if !slices.Equal(closed.CreateMeasure, []StoreActionUnavailableReason{StoreReasonStoreClosed}) {
		t.Fatalf("create measure reasons=%v", closed.CreateMeasure)
	}
	want := StoreActionUnavailableText(StoreReasonStoreClosed, StoreActionCreateMeasure).Text
	if want != "店家已關閉，不能建立警告、停權或風險措施。" {
		t.Fatalf("核可的句子：%q", want)
	}
	_, err = f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID))
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), want) {
		t.Fatalf("CreateMeasure：%v", err)
	}
	if got := CaseBlockStoreClosed.Text(CaseActionCreateMeasure).Text; got != want {
		t.Fatalf("案件的原因句 %q", got)
	}
	if got := ReportBlockStoreClosed.Text().Text; got != want {
		t.Fatalf("通報的原因句 %q", got)
	}
}

// 店家不是營運中時商家認證審核暫停（已關閉）：核准、退回、重新查詢都是 failed_precondition，訊息是原因清單的第一句（同一個函式）；申請維持 SUBMITTED、
// 不在待辦、沒有審核期限；撤銷終止合作恢復營業之後回到待辦，開始時間是恢復營業的時刻（期限從那時起算）。
// mutation：review_kyc_application 拿掉 s.status = 'ACTIVE' → 暫停期間仍在待辦（這個測試與下面兩個轉紅）。
// 注意：這個測試抓不到「開始時間不取 reopened_at」：現行唯一的恢復營業路徑是撤銷終止合作，撤銷同時結束停權（revoked_at 與 reopened_at 是同一個時刻），
// account_sanction_lifted_at 給出同一個值；reopened_at 項由 TestKYCReviewStartsFromTheReopeningWhenItIsTheLatest 單獨鎖住。
func TestKYCReviewPausesWhileTheStoreIsClosedAndResumesFromTheReopening(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s, sub := f.submitted(t, idA, bankAcct)
	submittedAt := f.now
	if got := f.kycReviewStarts(t, f.now); !got[s.StoreID].Equal(submittedAt) {
		t.Fatalf("營運中：開始是送出時間：%v", got)
	}
	if d := f.deadlineOf(t, s.StoreID); d == nil || !d.Due.Equal(submittedAt.Add(168*time.Hour)) {
		t.Fatalf("營運中：期限是送出加 168 小時（對照暫停期間的 nil）：%+v", d)
	}
	m := f.terminate(t, s.StoreID, GroundFraud)
	f.effect(t, m)
	if f.storeRow(t, s.StoreID).Status != Closed {
		t.Fatal("終止生效要關店")
	}
	closedText := "店家已關閉，這份商家認證申請暫停審核，店家恢復營業後才能審核或重新查詢。"
	approveR, rejectR, requeryR := f.reasonsOf(t, sub)
	closedOnly := []StoreActionUnavailableReason{StoreReasonStoreClosed}
	// 生效中的終止合作也是停權（帳號有生效中的措施）：核准的原因是 [已關閉, 停權]，第一個是已關閉；退回與重新查詢不看停權。
	if !slices.Equal(approveR, []StoreActionUnavailableReason{StoreReasonStoreClosed, StoreReasonAccountSuspended}) || !slices.Equal(rejectR, closedOnly) || !slices.Equal(requeryR, []StoreActionUnavailableReason{StoreReasonKYCNotBusiness, StoreReasonStoreClosed}) {
		t.Fatalf("approve=%v reject=%v requery=%v", approveR, rejectR, requeryR)
	}
	if v := f.adminView(t, sub.ID); v.Status != KYCStatusSubmitted {
		t.Fatalf("暫停：申請維持審核中：%+v", v)
	}
	if d := f.deadlineOf(t, s.StoreID); d != nil {
		t.Fatalf("暫停期間沒有審核期限：%+v", d)
	}
	// 方法與可用性是同一個函式：同一句。
	if err := approve(f, sub); apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), closedText) {
		t.Fatalf("approve：%v", err)
	}
	_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Reason: "x", Issues: anIssue()})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), closedText) {
		t.Fatalf("reject：%v", err)
	}
	if err := f.requery(ctx, RequeryKYCRegistrationInput{ApplicationID: sub.ID, AdminID: 9}); apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "只有公司型的申請有登記資料可以查詢。") {
		t.Fatalf("requery（個人型：結構性原因在前）：%v", err)
	}
	if got := f.kycReviewStarts(t, f.now); len(got) != 0 {
		t.Fatalf("暫停期間不在待辦：%v", got)
	}
	// 恢復營業：同一個 stint 回到營運中，申請回到待辦，開始時間是恢復的時刻。
	f.tick(48 * time.Hour)
	f.revoke(t, m)
	if f.storeRow(t, s.StoreID).Status != Active {
		t.Fatal("撤銷終止要恢復營業")
	}
	reopened := f.now
	if got := f.kycReviewStarts(t, f.now); !got[s.StoreID].Equal(reopened) || reopened.Equal(submittedAt) {
		t.Fatalf("恢復營業之後開始是恢復時間 %v：%v", reopened, got)
	}
	if d := f.deadlineOf(t, s.StoreID); d == nil || !d.Due.Equal(reopened.Add(168*time.Hour)) {
		t.Fatalf("期限從恢復營業起算：%+v", d)
	}
	if a, r, _ := f.reasonsOf(t, sub); len(a) != 0 || len(r) != 0 {
		t.Fatalf("恢復之後可以核准、退回：%v %v", a, r)
	}
}

// 等待刪除：與已關閉同一條（審核暫停），句子是 N6；不在待辦；等待刪除沒有反向，所以不會回來。
// 審核待辦的開始時間取這個 stint 最近一次恢復營業的時間（store_closures.reopened_at），即使它晚於送出時間與停權結束的時間。
// 現行唯一的恢復營業路徑（撤銷終止合作）讓 reopened_at 與停權結束是同一個時刻，所以這裡走完撤銷之後直接以 SQL 把 reopened_at 往後挪，
// 造出 reopened_at 嚴格晚於另外兩個候選時刻的狀態。
// mutation：review_kyc_application 的 greatest 拿掉 reopened_at 那一項 → 開始時間變成停權結束（撤銷）的時刻，這個測試轉紅。
func TestKYCReviewStartsFromTheReopeningWhenItIsTheLatest(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, _ := f.submitted(t, idA, bankAcct)
	submittedAt := f.now
	m := f.terminate(t, s.StoreID, GroundFraud)
	f.effect(t, m)
	f.tick(48 * time.Hour)
	f.revoke(t, m)
	liftedAt := f.now
	if l := f.liftedAt(t, accountID, liftedAt); l == nil || !l.Equal(liftedAt) {
		t.Fatalf("撤銷的那一刻停權結束：%v", l)
	}
	reopened := f.tick(2 * time.Hour)
	tag, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.store_closures SET reopened_at = $2 WHERE store_id = $1 AND reopened_at IS NOT NULL", s.StoreID, reopened)
	must(t, err)
	if tag.RowsAffected() != 1 {
		t.Fatalf("應該只有一筆已恢復的關店紀錄：%d", tag.RowsAffected())
	}
	if !reopened.After(liftedAt) || !reopened.After(submittedAt) {
		t.Fatal("前提：reopened_at 晚於送出與停權結束")
	}
	if got := f.kycReviewStarts(t, f.now); !got[s.StoreID].Equal(reopened) {
		t.Fatalf("開始是恢復營業時間 %v（不是停權結束 %v）：%v", reopened, liftedAt, got)
	}
	if d := f.deadlineOf(t, s.StoreID); d == nil || !d.Due.Equal(reopened.Add(168*time.Hour)) {
		t.Fatalf("期限從恢復營業起算：%+v", d)
	}
}
func TestKYCReviewPausesWhileTheStoreIsPendingDeletion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s, sub := f.submitted(t, idA, bankAcct)
	f.beginDeletion(t, accountID, s.StoreID)
	text := "店家等待刪除，這份商家認證申請不再審核。"
	pending := []StoreActionUnavailableReason{StoreReasonStorePendingDeletion}
	a, r, _ := f.reasonsOf(t, sub)
	if !slices.Equal(a, pending) || !slices.Equal(r, pending) {
		t.Fatalf("approve=%v reject=%v", a, r)
	}
	if err := approve(f, sub); apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), text) {
		t.Fatalf("approve：%v", err)
	}
	_, err := f.module.RejectKYC(context.Background(), RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Reason: "x", Issues: anIssue()})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), text) {
		t.Fatalf("reject：%v", err)
	}
	if got := f.kycReviewStarts(t, f.now); len(got) != 0 {
		t.Fatalf("等待刪除不在待辦：%v", got)
	}
}

// 帳號停權：核准被擋（ACCOUNT_SUSPENDED，N7）、退回與重新查詢照舊可用；件不在待辦、沒有期限；停權結束（到期或撤銷）時回到待辦，
// 開始時間是結束的時刻；停權在送件前就結束的，開始仍是送出時間。
// mutation：review_kyc_application 不取 account_sanction_lifted_at → 停權結束後開始仍是送出時間。
func TestKYCReviewPausesDuringSuspensionAndResumesWhenItEnds(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, sub := f.submitted(t, idA, bankAcct)
	submittedAt := f.now
	f.tick(time.Hour)
	ends := f.now.Add(24 * time.Hour)
	_, err := f.module.CreateMeasure(ctx, CreateMeasureInput{AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "y", Note: "x", EndsAt: &ends,
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityHigh}}})
	must(t, err)
	a, r, _ := f.reasonsOf(t, sub)
	if !slices.Equal(a, []StoreActionUnavailableReason{StoreReasonAccountSuspended}) || len(r) != 0 {
		t.Fatalf("停權：approve=%v reject=%v", a, r)
	}
	if got := f.kycReviewStarts(t, f.now); len(got) != 0 {
		t.Fatalf("停權期間不在待辦：%v", got)
	}
	if d := f.deadlineOf(t, s.StoreID); d != nil {
		t.Fatalf("停權期間沒有期限：%+v", d)
	}
	if err := approve(f, sub); apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "帳號停權中，停權解除之後才能核准商家認證。") {
		t.Fatalf("approve：%v", err)
	}
	// 邊界：到期前 1µs 仍停權，到期的那一刻回到待辦、開始時間是到期時間；兩個定義（effective_measures 與 account_sanction_lifted_at）在邊界一致。
	before := ends.Add(-time.Microsecond)
	if !f.effectiveAt(t, accountID, before) || f.liftedAt(t, accountID, before) != nil || len(f.kycReviewStarts(t, before)) != 0 {
		t.Fatalf("到期前 1µs：effective=%v lifted=%v", f.effectiveAt(t, accountID, before), f.liftedAt(t, accountID, before))
	}
	if f.effectiveAt(t, accountID, ends) || f.liftedAt(t, accountID, ends) == nil || !f.liftedAt(t, accountID, ends).Equal(ends) {
		t.Fatalf("到期的那一刻：effective=%v lifted=%v", f.effectiveAt(t, accountID, ends), f.liftedAt(t, accountID, ends))
	}
	if got := f.kycReviewStarts(t, ends); !got[s.StoreID].Equal(ends) || ends.Equal(submittedAt) {
		t.Fatalf("到期之後開始是停權結束的時間 %v：%v", ends, got)
	}
	// 撤銷也是一種結束：另一家店，停權再撤銷。
	account2, s2, _ := f.submitted(t, idB, "012345678902")
	f.tick(time.Hour)
	risk := f.suspend(t, s2.StoreID)
	if len(f.kycReviewStarts(t, f.now)) != 0 || f.liftedAt(t, account2, f.now) != nil {
		t.Fatal("停權中")
	}
	f.tick(time.Hour)
	f.revoke(t, risk)
	revokedAt := f.now
	if f.effectiveAt(t, account2, revokedAt) || f.liftedAt(t, account2, revokedAt) == nil || !f.liftedAt(t, account2, revokedAt).Equal(revokedAt) {
		t.Fatalf("撤銷的那一刻：effective=%v lifted=%v", f.effectiveAt(t, account2, revokedAt), f.liftedAt(t, account2, revokedAt))
	}
	if got := f.kycReviewStarts(t, revokedAt); !got[s2.StoreID].Equal(revokedAt) {
		t.Fatalf("撤銷之後開始是撤銷時間 %v：%v", revokedAt, got)
	}
}

// 停權時退回照舊可用（員工確定要退的件不必等停權結束）。
func TestRejectingDuringSuspensionStillWorks(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s, sub := f.submitted(t, idA, bankAcct)
	f.suspend(t, s.StoreID)
	rej := f.reject(t, sub, anIssue()...)
	if rej.Status != KYCStatusRejected {
		t.Fatalf("status=%v", rej.Status)
	}
}

// 停權在送件前就結束的：開始仍是送出時間（只算真的生效過、結束時間晚於 starts_at 的，而且是在送出之前的那一次不影響較晚的送出）。
func TestASuspensionThatEndedBeforeSubmittingDoesNotMoveTheStart(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	risk := f.suspend(t, s.StoreID)
	f.tick(time.Hour)
	f.revoke(t, risk)
	f.tick(time.Hour)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	_, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	// 較早結束的停權讓 greatest 取它，但送出時間更晚，所以開始是送出時間。
	if got := f.kycReviewStarts(t, f.now); !got[s.StoreID].Equal(f.now) {
		t.Fatalf("開始是送出時間 %v：%v", f.now, got)
	}
}

// 「待審核」只有一個定義：待辦（review_kyc_application）的店家集合 ＝ 清單的「待審核」分頁 ＝ 統計卡 ＝ Go 對每筆審核中的申請算核准原因為空的集合。
// 資料集：營運中、已關閉、等待刪除、停權、恢復營業、身分命中（核准會失敗但仍在待辦，讓員工退回）、已核准。
// mutation：統計改回 k.status = 'SUBMITTED' → 數量與集合不一致。
func TestAwaitingReviewIsOneDefinitionAcrossTheQueueTheListAndTheStats(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	sub := func(serial int) (int64, Store, KYCApplication) {
		accountID, store := f.open(t)
		return accountID, store, f.submitKYCFor(t, accountID, serial)
	}
	_, operating, _ := sub(1)
	_, closedStore, _ := sub(2)
	closing := f.terminate(t, closedStore.StoreID, GroundFraud)
	f.effect(t, closing)
	pendAccount, pendStore, _ := sub(3)
	f.beginDeletion(t, pendAccount, pendStore.StoreID)
	_, suspendedStore, _ := sub(4)
	f.suspend(t, suspendedStore.StoreID)
	_, reopenedStore, _ := sub(5)
	reopenTerm := f.terminate(t, reopenedStore.StoreID, GroundFraud)
	f.effect(t, reopenTerm)
	f.tick(time.Hour)
	f.revoke(t, reopenTerm)
	_, _, toApprove := sub(6)
	must(t, approve(f, toApprove))
	// 身分命中：別的帳號以同一個身分證號入駐過、之後被停權，這一份在送出時還沒有處分；核准會失敗，但仍留在待辦讓員工退回。
	otherAccount, otherStore := f.open(t)
	f.savedDraft(t, kycDraft(otherAccount, taiwanID(7), "999888777666"))
	_, hitStore, hitSub := sub(7)
	f.suspend(t, otherStore.StoreID)
	if err := approve(f, hitSub); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("身分命中核准要失敗：%v", err)
	}
	if a, _, _ := f.reasonsOf(t, hitSub); len(a) != 0 {
		t.Fatalf("身分命中不列在原因裡（不透露）：%v", a)
	}

	queue := map[int64]bool{}
	for id := range f.kycReviewStarts(t, f.now) {
		queue[id] = true
	}
	list := map[int64]bool{}
	page, err := f.module.AdminStores(ctx, StoreListInput{KYCAwaitingReview: true, Statuses: []Status{Active, PendingDeletion, Closed}})
	must(t, err)
	for _, item := range page.Items {
		list[item.Store.StoreID] = true
	}
	goSet := map[int64]bool{}
	all, err := f.module.AdminKYCApplications(ctx, KYCListInput{Status: ptr(KYCStatusSubmitted)})
	must(t, err)
	for _, item := range all.Items {
		a, r, _ := f.reasonsOf(t, KYCApplication{ID: item.ID})
		_ = r
		if len(a) == 0 {
			goSet[item.StoreID] = true
		}
	}
	stats, err := f.module.AdminStoreStats(ctx)
	must(t, err)
	want := map[int64]bool{operating.StoreID: true, reopenedStore.StoreID: true, hitStore.StoreID: true}
	for name, got := range map[string]map[int64]bool{"待辦": queue, "清單": list, "Go（核准原因為空）": goSet} {
		if len(got) != len(want) {
			t.Errorf("%s：%v，預期 %v", name, got, want)
			continue
		}
		for id := range want {
			if !got[id] {
				t.Errorf("%s：缺 %d（%v）", name, id, got)
			}
		}
	}
	if int(stats.KYCAwaitingReview) != len(want) {
		t.Errorf("統計卡 %d，預期 %d", stats.KYCAwaitingReview, len(want))
	}
}

// 重新綁定：可用性與方法是同一個函式。已關閉、等待刪除 → failed_precondition 與各自的句子（不再是 not_found）；終止（needs_reonboarding）而且已關閉 →
// 第一句是終止；BOUND → failed_precondition；PENDING → 回目前的註冊、不排 job，原因第一個是 PAYUNI_BIND_IN_PROGRESS、can_rebind 為 false。
// mutation：RebindUnavailableReasons 把 STORE_CLOSED 拿掉 → 已關閉的店可以重新綁定。
func TestRebindReasonsAndErrorsAreTheSameFunction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	failed := func(t *testing.T) (*fixture, int64, Store) {
		f := setup(t)
		accountID, s := f.approvedStore(t)
		f.payuni.bindErr = []error{apiErr("AGENT04003")}
		f.sync(t, s.StoreID)
		if f.registration(t, s.StoreID).BindStatus != BindStatusFailed {
			t.Fatal("not failed")
		}
		return f, accountID, s
	}
	t.Run("已關閉", func(t *testing.T) {
		t.Parallel()
		f, accountID, s := failed(t)
		_, err := f.module.CloseCurrent(ctx, accountID)
		must(t, err)
		view, err := f.module.PayUniRegistrationOf(ctx, s.StoreID)
		must(t, err)
		if !slices.Equal(view.RebindUnavailable, []StoreActionUnavailableReason{StoreReasonStoreClosed}) {
			t.Fatalf("reasons=%v", view.RebindUnavailable)
		}
		_, err = f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
		if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "店家已關閉，不能重新綁定 PayUni。") {
			t.Fatalf("rebind：%v", err)
		}
	})
	t.Run("等待刪除", func(t *testing.T) {
		t.Parallel()
		f, accountID, s := failed(t)
		f.beginDeletion(t, accountID, s.StoreID)
		view, err := f.module.PayUniRegistrationOf(ctx, s.StoreID)
		must(t, err)
		if !slices.Equal(view.RebindUnavailable, []StoreActionUnavailableReason{StoreReasonStorePendingDeletion}) {
			t.Fatalf("reasons=%v", view.RebindUnavailable)
		}
		_, err = f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
		if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "店家等待刪除，不能重新綁定 PayUni。") {
			t.Fatalf("rebind：%v", err)
		}
	})
	t.Run("PayUni 端已終止而且已關閉", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		accountID, s := f.approvedStore(t)
		f.payuni.bindErr = []error{apiErr("AGENT04017")}
		f.payuni.status = pu.MerchantStatus{MerStatus: "92", Reason: "文件不符"}
		f.sync(t, s.StoreID)
		_, err := f.module.CloseCurrent(ctx, accountID)
		must(t, err)
		view, err := f.module.PayUniRegistrationOf(ctx, s.StoreID)
		must(t, err)
		if len(view.RebindUnavailable) < 2 || view.RebindUnavailable[0] != StoreReasonPayUniMerchantTerminal || view.RebindUnavailable[1] != StoreReasonStoreClosed {
			t.Fatalf("reasons=%v", view.RebindUnavailable)
		}
		_, err = f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
		if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "PayUni 端的商店已是終止狀態，這家店無法再綁定；賣家要重新入駐才能收款。") {
			t.Fatalf("rebind：%v", err)
		}
	})
	t.Run("已綁定", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		_, s := f.boundStore(t)
		view, err := f.module.PayUniRegistrationOf(ctx, s.StoreID)
		must(t, err)
		if len(view.RebindUnavailable) == 0 || view.RebindUnavailable[0] != StoreReasonPayUniBound {
			t.Fatalf("reasons=%v", view.RebindUnavailable)
		}
		if _, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 9}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("rebind：%v", err)
		}
	})
	t.Run("綁定失敗而賣家改回草稿", func(t *testing.T) {
		t.Parallel()
		f, accountID, s := failed(t)
		mine, err := f.module.MyKYC(ctx, accountID)
		must(t, err)
		_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王大明")})
		must(t, err)
		view, err := f.module.PayUniRegistrationOf(ctx, s.StoreID)
		must(t, err)
		if !slices.Equal(view.RebindUnavailable, []StoreActionUnavailableReason{StoreReasonKYCNotApproved}) {
			t.Fatalf("reasons=%v", view.RebindUnavailable)
		}
		// 同一句：卡片上的原因句與 RebindPayUni 的錯誤（5a #38）。
		want := StoreActionUnavailableText(StoreReasonKYCNotApproved, StoreActionRebindPayUni).Text
		_, err = f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
		if want != "賣家正在修改商家認證，重新核准後會自動重新綁定。" || apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), want) {
			t.Fatalf("rebind：%q %v", want, err)
		}
	})
	t.Run("綁定失敗：可以重新綁定", func(t *testing.T) {
		t.Parallel()
		f, _, s := failed(t)
		view, err := f.module.PayUniRegistrationOf(ctx, s.StoreID)
		must(t, err)
		if len(view.RebindUnavailable) != 0 || view.FlowStatus != FlowBindFailed {
			t.Fatalf("reasons=%v flow=%v", view.RebindUnavailable, view.FlowStatus)
		}
		jobs := f.jobs.count(kindSync)
		after, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
		must(t, err)
		if after.BindStatus != BindStatusPending || after.FlowStatus != FlowBinding || f.jobs.count(kindSync) != jobs+1 ||
			!slices.Equal(after.RebindUnavailable, []StoreActionUnavailableReason{StoreReasonPayUniBindInProgress}) {
			t.Fatalf("after=%+v", after)
		}
		// 綁定中：不是失敗原因、沒有句子；再送一次回目前的註冊、不另排 job。
		if StoreActionUnavailableText(StoreReasonPayUniBindInProgress, StoreActionRebindPayUni).Text != "" {
			t.Fatal("PAYUNI_BIND_IN_PROGRESS 沒有句子")
		}
		again, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
		if err != nil || again.BindStatus != BindStatusPending || f.jobs.count(kindSync) != jobs+1 {
			t.Fatalf("again=%+v err=%v", again, err)
		}
	})
}
