//go:build integration

package seller_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// 測試用的身分證號（格式與檢查碼都正確，不是真實的人）與銀行帳號。
const (
	idA      = "A123456789"
	idB      = "F131104093"
	idC      = "V223456789"
	idRep    = "X223456789"
	bankAcct = "012345678901"
	taxIDOne = "33330333"
)

func individual() *KYCSellerType { return ptr(KYCIndividual) }

// kycDraft 是一份完整、合法的個人賣家草稿（身分證號與銀行帳號可換）。
func kycDraft(accountID int64, idCard, bank string) KYCDraftInput {
	return KYCDraftInput{
		AccountID: accountID, SellerType: individual(), RealName: ptr("王小明"), BirthDate: ptr("1990-05-06"), IDCardNo: ptr(idCard),
		EnglishName: ptr("Wang Farm"), BankName: ptr("臺灣銀行"), BankCode: ptr("004"), BankAccount: ptr(bank),
		ProductType: ptr("蔬果"),
		IDCardFront: &KYCDocumentRef{StagedToken: "front"}, IDCardBack: &KYCDocumentRef{StagedToken: "back"}, BankBook: &KYCDocumentRef{StagedToken: "book"},
	}
}

func companyDraft(accountID int64, repID, taxID, bank string) KYCDraftInput {
	in := kycDraft(accountID, "", bank)
	in.SellerType, in.IDCardNo = ptr(KYCCompany), nil
	in.RepIDCardNo, in.CompanyName, in.TaxID = ptr(repID), ptr("範例小農有限公司"), ptr(taxID)
	return in
}

// savedDraft 建立草稿；回傳申請。
func (f *fixture) savedDraft(t *testing.T, in KYCDraftInput) KYCApplication {
	t.Helper()
	a, err := f.module.SaveKYCDraft(context.Background(), in)
	must(t, err)
	return a
}

// submitted 開店、建立並送出一份個人賣家的申請。
func (f *fixture) submitted(t *testing.T, idCard, bank string) (int64, Store, KYCApplication) {
	t.Helper()
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idCard, bank))
	a, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	return accountID, s, a
}

func approve(f *fixture, a KYCApplication) error {
	_, err := f.module.ApproveKYC(context.Background(), ApproveKYCInput{ApplicationID: a.ID, ExpectedVersion: a.Version, AdminID: 9})
	return err
}

func TestKYCDraftSubmitApproveFlow(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	submitted, approved := record[KYCSubmitted](f), record[KYCApproved](f)
	profile := record[StoreProfileChanged](f)
	accountID, s := f.open(t)
	in := kycDraft(accountID, idA, bankAcct)
	d := f.savedDraft(t, in)
	if d.Status != KYCStatusDraft || d.Version != 1 || d.StoreID != s.StoreID || d.AccountID != accountID ||
		*d.IDCardNoMasked != "A1******89" || *d.BankAccountMasked != "********8901" || len(d.Documents) != 3 {
		t.Fatalf("draft=%+v", d)
	}
	// 沒有送出時管理員看不到草稿。
	if _, err := f.module.AdminKYCApplication(ctx, d.ID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("admin read a draft: %v", err)
	}
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	if sub.Status != KYCStatusSubmitted || sub.Version != 2 || sub.SubmittedAt == nil {
		t.Fatalf("submitted=%+v", sub)
	}
	storeBefore, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: d.ID, ExpectedVersion: sub.Version, AdminID: 9, Comment: "資料齊全"}); err != nil {
		t.Fatal(err)
	}
	admin, err := f.module.AdminKYCApplication(ctx, d.ID)
	must(t, err)
	if admin.Status != KYCStatusApproved || admin.DecidedAt == nil || len(admin.Reviews) != 2 || admin.SharesBankAccountWithSanctionedAccount {
		t.Fatalf("admin=%+v", admin)
	}
	submitReview, approveReview := admin.Reviews[0], admin.Reviews[1]
	if submitReview.ReviewerAdminID != nil || *approveReview.ReviewerAdminID != 9 || *approveReview.Reason != "資料齊全" {
		t.Fatalf("reviews=%+v", admin.Reviews)
	}
	// 事件帶穩定的 UUIDv7，等於審核紀錄的 ID（relay 重送時不變）。
	if len(submitted.all()) != 1 || submitted.all()[0].EventID != submitReview.ID || submitted.all()[0].EventID[6]>>4 != 7 {
		t.Fatalf("submitted=%+v", submitted.all())
	}
	if len(approved.all()) != 1 || approved.all()[0].EventID != approveReview.ID || approved.all()[0].AccountID != accountID || approved.all()[0].StoreID != s.StoreID {
		t.Fatalf("approved=%+v", approved.all())
	}
	// 核准 KYC 不碰店家的 profile 與食品業者登錄（登錄是獨立的審核資源）：店家的版本不動、沒有 profile 事件、沒有核准的登錄。
	after, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if after.ApprovedFoodRegistration != nil || after.Version != storeBefore.Version || len(profile.all()) != 0 {
		t.Fatalf("store=%+v profile events=%d", after, len(profile.all()))
	}
	// 已核准的不能再核准、退回、取消或修改。
	if err := approve(f, admin.KYCApplication); apperr.KindOf(err) != apperr.KindFailedPrecondition && apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("approve twice: %v", err)
	}
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: admin.Version, RealName: ptr("改名")}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("edit approved: %v", err)
	}
	if _, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: admin.Version}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("cancel approved: %v", err)
	}
}

func TestKYCRejectThenResubmitReusesTheApplication(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	rejected := record[KYCRejected](f)
	accountID, _, sub := f.submitted(t, idA, bankAcct)
	for name, reason := range map[string]string{"none": "", "blank": " \t ", "overlong": strings.Repeat("理", 1001)} {
		if _, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: reason}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("reject with a %s reason: %v", name, err)
		}
	}
	if _, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, Issues: anIssue(), Reason: "x"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("reject without an admin: %v", err)
	}
	rej, err := f.module.RejectKYC(ctx, RejectKYCInput{
		ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Reason: "存摺不清楚",
		Issues: []KYCIssue{
			{Field: "bank_book", DocumentAssetID: "asset-book", Code: IssueUnreadable, Message: "反光看不清楚"},
			{Field: "bank_account", Code: IssueMismatch, Message: "與存摺不符"},
		},
	})
	must(t, err)
	if rej.Status != KYCStatusRejected || len(rejected.all()) != 1 || rejected.all()[0].Reason != "存摺不清楚" || rejected.all()[0].AccountID != accountID {
		t.Fatalf("rejected=%+v events=%+v", rej, rejected.all())
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	wantIssues := []KYCIssue{
		{Field: "bank_book", DocumentAssetID: "asset-book", Code: IssueUnreadable, Message: "反光看不清楚"},
		{Field: "bank_account", Code: IssueMismatch, Message: "與存摺不符"},
	}
	if mine.LastRejection == nil || mine.LastRejection.Reason != "存摺不清楚" || !slices.Equal(mine.LastRejection.Issues, wantIssues) {
		t.Fatalf("last rejection=%+v", mine.LastRejection)
	}
	// 被退回的申請編輯後回到 DRAFT，沿用同一筆（同一個 ID）；不需要重新建立。
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: rej.Version, BankAccount: ptr("999888777666")})
	must(t, err)
	if edited.ID != sub.ID || edited.Status != KYCStatusDraft || edited.DecidedAt != nil || *edited.BankAccountMasked != "********7666" {
		t.Fatalf("edited=%+v", edited)
	}
	// 改掉的銀行帳號算已處理，沒動的存摺照片還在待處理。
	if !slices.Equal(edited.OutstandingIssues, wantIssues[:1]) {
		t.Fatalf("outstanding=%+v", edited.OutstandingIssues)
	}
	// 最近一次退回在回到草稿與再次送出之後的回應仍在（與 MyKYC 同一份檢視）。
	if edited.LastRejection == nil || edited.LastRejection.Reason != "存摺不清楚" || !slices.Equal(edited.LastRejection.Issues, wantIssues) {
		t.Fatalf("save response last rejection=%+v", edited.LastRejection)
	}
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: edited.Version})
	must(t, err)
	if again.Status != KYCStatusSubmitted || f.count(t, "SELECT count(*) FROM seller.kyc_applications") != 1 || f.count(t, "SELECT count(*) FROM seller.kyc_reviews") != 3 {
		t.Fatalf("again=%+v", again)
	}
	// 重新送出之後，退回不再是賣家要處理的：回應與 MyKYC 都沒有。
	if again.LastRejection != nil || len(again.OutstandingIssues) != 0 {
		t.Fatalf("submit response last rejection=%+v outstanding=%+v", again.LastRejection, again.OutstandingIssues)
	}
	if after, err := f.module.MyKYC(ctx, accountID); err != nil || after.LastRejection != nil {
		t.Fatalf("MyKYC after resubmit=%+v err=%v", after.LastRejection, err)
	}
}

func TestKYCSubmitsDirectlyFromRejectedWithoutASave(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submitted(t, idA, bankAcct)
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: sub.Version}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("submit while under review: %v", err)
	}
	rej, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "存摺不清楚"})
	must(t, err)
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: rej.Version})
	must(t, err)
	if again.ID != sub.ID || again.Status != KYCStatusSubmitted || again.DecidedAt != nil || again.LastRejection != nil ||
		f.count(t, "SELECT count(*) FROM seller.kyc_applications") != 1 || f.count(t, "SELECT count(*) FROM seller.kyc_reviews") != 3 {
		t.Fatalf("again=%+v", again)
	}
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: rej.Version}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale etag: %v", err)
	}
}

// 回歸（盤點 D2、D3）：舊實作的自查回應含解密後的 step_data，遮蔽被繞過，敏感鍵還是拼法黑名單。新的檢視型別沒有任何
// 欄位能放明文，草稿、自查、後台清單與詳情都只有遮蔽值；DB 裡也只有密文。
func TestKYCNeverReturnsOrStoresPlaintext(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	admin, err := f.module.AdminKYCApplication(ctx, d.ID)
	must(t, err)
	page, err := f.module.AdminKYCApplications(ctx, KYCListInput{})
	must(t, err)
	for name, v := range map[string]any{"draft": d, "mine": mine, "submitted": sub, "admin": admin, "list": page} {
		out := fmt.Sprintf("%+v", v)
		if strings.Contains(out, idA) || strings.Contains(out, bankAcct) || strings.Contains(out, "345678901") {
			t.Errorf("%s 含明文：%s", name, out)
		}
	}
	var idEnc, bankEnc []byte
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT id_card_no_enc, bank_account_enc FROM seller.kyc_applications").Scan(&idEnc, &bankEnc))
	if len(idEnc) == 0 || bytes.Contains(idEnc, []byte(idA)) || bytes.Contains(bankEnc, []byte(bankAcct)) {
		t.Fatal("DB 裡的個資不是密文")
	}
	var leaked int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM seller.kyc_applications WHERE row_to_json(kyc_applications)::text LIKE '%' || $1 || '%' OR row_to_json(kyc_applications)::text LIKE '%' || $2 || '%'", idA, bankAcct).Scan(&leaked))
	if leaked != 0 {
		t.Fatal("申請列的任何欄位都不該有明文")
	}
}

// 回歸（盤點 D12）：舊實作的日期解析失敗只 log、欄位沿用舊值或留空。壞日期整筆拒絕、不寫入，原本的值與版本不變。
func TestKYCInvalidDatesAreRejectedNotKept(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	bad := kycDraft(accountID, idA, bankAcct)
	bad.BirthDate = ptr("2026-02-30")
	if _, err := f.module.SaveKYCDraft(ctx, bad); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("create with a bad date: %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_applications"); n != 0 {
		t.Fatalf("a rejected draft was written: %d", n)
	}
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	for name, value := range map[string]string{
		"not a date": "yesterday", "wrong order": "06-05-1990", "ROC year": "079-05-06", "nonexistent day": "2025-02-29",
		"future": "2026-10-02", "before 1900": "1899-12-31",
	} {
		for field, in := range map[string]KYCDraftInput{
			"BirthDate":       {BirthDate: ptr(value)},
			"IDCardIssueDate": {IDCardIssueDate: ptr(value)},
		} {
			in.AccountID, in.ExpectedVersion, in.RealName = accountID, d.Version, ptr("被誤寫的名字")
			if _, err := f.module.SaveKYCDraft(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
				t.Errorf("%s %s = %q 應該被拒絕：%v", field, name, value, err)
			}
		}
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if mine.Version != d.Version || *mine.BirthDate != "1990-05-06" || *mine.RealName != "王小明" || mine.IDCardIssueDate != nil {
		t.Fatalf("a rejected update changed the draft: %+v", mine)
	}
	// 今天（台北）本身是有效的日期。
	ok, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, BirthDate: ptr("2026-10-01")})
	must(t, err)
	if *ok.BirthDate != "2026-10-01" {
		t.Fatalf("birth date=%v", ok.BirthDate)
	}
}

// 回歸（盤點 D20）：舊實作靠 sellers.current_certification_id 找目前的申請，寫入失敗只 log，之後會再建一筆。新的以 store_id 為鍵，
// 一個店家同時最多一筆還沒取消的申請由 DB 保證；取消後可以重新建立。
func TestOneOpenKYCApplicationPerStoreIsEnforcedByTheDatabase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	insert := "INSERT INTO seller.kyc_applications (application_id, store_id, seller_type, status, submission_seq, created_at, updated_at) VALUES (uuidv7(), $1, 'INDIVIDUAL', $2::text, CASE WHEN $2::text = 'DRAFT' THEN 0 ELSE 1 END, now(), now())"
	for _, status := range []string{"DRAFT", "SUBMITTED"} {
		if _, err := f.db.Conn(ctx).Exec(ctx, insert, s.StoreID, status); err == nil {
			t.Fatalf("a second %s application for the same store was accepted", status)
		}
	}
	// 經模組：已有申請時再「建立」是 AlreadyExists（帶 VERSION_CONFLICT），不會多一筆。
	if _, err := f.module.SaveKYCDraft(ctx, kycDraft(accountID, idA, bankAcct)); apperr.KindOf(err) != apperr.KindAlreadyExists {
		t.Fatalf("create twice: %v", err)
	}
	cancelled, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: d.Version, Reason: "改天再填"})
	must(t, err)
	if cancelled.Status != KYCStatusCancelled {
		t.Fatalf("cancelled=%+v", cancelled)
	}
	again := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	if again.ID == d.ID || again.Version != 1 {
		t.Fatalf("again=%+v", again)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_applications WHERE status <> 'CANCELLED'"); n != 1 {
		t.Fatalf("open applications=%d", n)
	}
}

// 回歸（盤點 D6）：舊實作的 FOR UPDATE 不在 transaction 內，鎖立刻釋放，同時核准與退回各自成功。新的在 transaction 內先鎖
// 帳號的 stint 再鎖申請列，鎖內重新確認；兩個決定同時進來只有一個成立。
func TestKYCApproveAndRejectAreSerialisedUnderTheLock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s, sub := f.submitted(t, idA, bankAcct)
	holder, err := f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = holder.Rollback(ctx) }()
	_, err = holder.Exec(ctx, "SELECT 1 FROM seller.stores WHERE store_id=$1 FOR UPDATE", s.StoreID)
	must(t, err)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, decide := range []func() error{
		func() error { return approve(f, sub) },
		func() error {
			_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 10, Issues: anIssue(), Reason: "資料不符"})
			return err
		},
	} {
		wg.Add(1)
		go func() { defer wg.Done(); results <- decide() }()
	}
	f.waitForLockWaiters(t, 2)
	must(t, holder.Commit(ctx))
	wg.Wait()
	close(results)
	var won, lost int
	for err := range results {
		switch {
		case err == nil:
			won++
		case apperr.KindOf(err) == apperr.KindFailedPrecondition || apperr.KindOf(err) == apperr.KindVersionConflict:
			lost++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if won != 1 || lost != 1 {
		t.Fatalf("won=%d lost=%d", won, lost)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action IN ('APPROVE', 'REJECT')"); n != 1 {
		t.Fatalf("decisions=%d", n)
	}
}

func TestKYCStaleVersionsConflict(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submitted(t, idA, bankAcct)
	for name, err := range map[string]error{
		"approve": func() error {
			_, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version - 1, AdminID: 9})
			return err
		}(),
		"reject": func() error {
			_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: 0, AdminID: 9, Issues: anIssue(), Reason: "x"})
			return err
		}(),
		"cancel": func() error {
			_, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: sub.Version + 1})
			return err
		}(),
	} {
		if apperr.KindOf(err) != apperr.KindVersionConflict {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// 回歸（盤點 D21）：舊實作核准前不看申請人的帳號有沒有被停權。
func TestKYCApproveRejectsASuspendedApplicant(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s, sub := f.submitted(t, idA, bankAcct)
	sanction := f.suspend(t, s.StoreID)
	err := approve(f, sub)
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("approve while suspended: %v", err)
	}
	if got, err := f.module.AdminKYCApplication(ctx, sub.ID); err != nil || got.Status != KYCStatusSubmitted {
		t.Fatalf("status=%v err=%v", got.Status, err)
	}
	_, err = f.lift(ctx, sanction)
	must(t, err)
	if err := approve(f, sub); err != nil {
		t.Fatalf("approve after the sanction was lifted: %v", err)
	}
}

// 身分比對：別的帳號有生效中處分的身分證號（個人與企業負責人是同一種）或統編，不能通過認證；送出時先查，核准時在鎖內
// 再查一次（以後者為準）；錯誤是泛用的，不透露命中哪個欄位。
func TestKYCIdentityMatchBlocksAcrossAccounts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	// 帳號 B 以身分證號 idA 與統編入駐過、之後被停權。
	bAccount, bStore := f.open(t)
	b := companyDraft(bAccount, idA, taxIDOne, "111222333444")
	f.savedDraft(t, b)
	f.suspend(t, bStore.StoreID)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 3 {
		t.Fatalf("snapshot rows=%d（身分證號、統編、銀行帳號各一）", n)
	}
	var messages []string
	for name, draft := range map[string]func(int64) KYCDraftInput{
		"同一個身分證號（個人）":    func(a int64) KYCDraftInput { return kycDraft(a, idA, bankAcct) },
		"同一個身分證號（企業負責人）": func(a int64) KYCDraftInput { return companyDraft(a, idA, "71777777", bankAcct) },
		"同一個統編":          func(a int64) KYCDraftInput { return companyDraft(a, idRep, taxIDOne, bankAcct) },
	} {
		accountID, _ := f.open(t)
		d := f.savedDraft(t, draft(accountID))
		_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
		if apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("%s：err=%v", name, err)
		}
		messages = append(messages, err.Error())
	}
	for _, m := range messages {
		if m != messages[0] || strings.Contains(m, "身分證") || strings.Contains(m, "統") || strings.Contains(m, "停權") {
			t.Fatalf("錯誤不能透露命中的欄位或原因：%q", messages)
		}
	}
	// 不相干的身分證號可以送出。
	accountID, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idC, "555666777888"))
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version}); err != nil {
		t.Fatalf("unrelated identity: %v", err)
	}
}

// 核准時在鎖內再查一次，以後者為準：送出時還沒有處分，核准前另一個帳號被停權，核准就要失敗。
func TestKYCApproveRechecksTheIdentityInsideTheTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	bAccount, bStore := f.open(t)
	f.savedDraft(t, kycDraft(bAccount, idA, bankAcct))
	_, _, sub := f.submitted(t, idA, "999000111222") // 送出時 B 還沒被處分
	f.suspend(t, bStore.StoreID)
	err := approve(f, sub)
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || strings.Contains(err.Error(), "身分證") {
		t.Fatalf("approve: %v", err)
	}
	if got, err := f.module.AdminKYCApplication(ctx, sub.ID); err != nil || got.Status != KYCStatusSubmitted {
		t.Fatalf("status=%v err=%v", got.Status, err)
	}
}

// 銀行帳號命中只標記、不擋（家人或配偶共用帳戶是常態）。
func TestKYCBankAccountMatchFlagsButDoesNotBlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	bAccount, bStore := f.open(t)
	f.savedDraft(t, kycDraft(bAccount, idA, bankAcct))
	f.suspend(t, bStore.StoreID)
	aAccount, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(aAccount, idC, "012-345-678901")) // 同一個帳號，分隔符不同
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: aAccount, ExpectedVersion: d.Version})
	must(t, err)
	admin, err := f.module.AdminKYCApplication(ctx, sub.ID)
	must(t, err)
	if !admin.SharesBankAccountWithSanctionedAccount {
		t.Fatal("同一個銀行帳號應該標記")
	}
	if err := approve(f, sub); err != nil {
		t.Fatalf("approve should not be blocked by a shared bank account: %v", err)
	}
	// 不同銀行代碼的同一串數字不算同一個帳戶。
	other, _ := f.open(t)
	in := kycDraft(other, idB, bankAcct)
	in.BankCode = ptr("812")
	od := f.savedDraft(t, in)
	osub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: other, ExpectedVersion: od.Version})
	must(t, err)
	if got, err := f.module.AdminKYCApplication(ctx, osub.ID); err != nil || got.SharesBankAccountWithSanctionedAccount {
		t.Fatalf("different bank code flagged: %+v %v", got.SharesBankAccountWithSanctionedAccount, err)
	}
	// 處分結束後標記消失。
	_, err = f.lift(ctx, f.activeRisk(t, bStore.StoreID))
	must(t, err)
	if got, err := f.module.AdminKYCApplication(ctx, sub.ID); err != nil || got.SharesBankAccountWithSanctionedAccount {
		t.Fatalf("flag after the sanction was lifted: %v %v", got.SharesBankAccountWithSanctionedAccount, err)
	}
}

// 措施結束或到期之後不再擋，快照也刪掉：解除在同一個 transaction 刪除；到期的處分，時間一過就不擋（stint.Derive，不等
// job），到期 job 與對帳再把快照刪掉。違規產生的處分一樣有快照。
func TestKYCLiftedOrExpiredSanctionNoLongerBlocksAndItsSnapshotIsDeleted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	blockedSubmit := func(idCard string) bool {
		accountID, _ := f.open(t)
		d := f.savedDraft(t, kycDraft(accountID, idCard, fmt.Sprintf("%012d", time.Now().UnixNano()%1_000_000_000_000)))
		_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
		return err != nil
	}
	// 管理員停權：解除後刪除。
	aAccount, aStore := f.open(t)
	f.savedDraft(t, kycDraft(aAccount, idA, bankAcct))
	sanction := f.suspend(t, aStore.StoreID)
	if !blockedSubmit(idA) || f.count(t, "SELECT count(*) FROM seller.measure_identities") == 0 {
		t.Fatal("停權中的身分證號應該被擋，快照應該存在")
	}
	_, err := f.lift(ctx, sanction)
	must(t, err)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 0 {
		t.Fatalf("解除後快照還在：%d", n)
	}
	if blockedSubmit(idA) {
		t.Fatal("解除後不該再擋")
	}
	// 違規的停權：有快照；到期後時間一過就不擋（job 還沒跑）；job 之後快照刪除。
	bAccount, bStore := f.open(t)
	f.savedDraft(t, kycDraft(bAccount, idB, "123123123123"))
	endsAt := f.now.Add(48 * time.Hour)
	f.violate(t, bStore.StoreID, SeverityHigh, MeasureSuspension, &endsAt)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 2 {
		t.Fatalf("違規的處分也要有快照（身分證號與銀行帳號）：%d", n)
	}
	if !blockedSubmit(idB) {
		t.Fatal("違規停權中的身分證號應該被擋")
	}
	f.now = endsAt.Add(time.Minute)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 2 {
		t.Fatalf("job 還沒跑，快照還在：%d", n)
	}
	if blockedSubmit(idB) {
		t.Fatal("處分到期之後不該再擋（不能等 job）")
	}
	var id uuid.UUID
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT id FROM seller.measures WHERE basis = 'VIOLATION'").Scan(&id))
	must(t, f.module.ExpireMeasure(ctx, id, endsAt))
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 0 {
		t.Fatalf("到期 job 之後快照還在：%d", n)
	}
}

// 對帳：到期 job 遺失、帳號沒有任何期間可掃時，措施結束後留下的快照由對帳刪除。
func TestReconcileDeletesSnapshotsOfEndedSanctions(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	// 沒有目前 stint 的帳號：直接寫一筆已結束的措施與快照（模擬結束 job 遺失）。
	var storeID int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE account_id=$1", accountID).Scan(&storeID))
	f.insertEndedRisk(t, storeID, accountID, true)
	if _, err := f.module.ReconcileSuspensions(ctx); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 0 {
		t.Fatalf("snapshots left: %d", n)
	}
}

func TestRevealWritesAnAuditRowAndNeverTheValue(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, companyDraft(accountID, idRep, taxIDOne, bankAcct))
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	// 理由的規則是 platform/auditreason：空白、超過 1000 字都拒絕，也不寫稽核；管理員 ID 要是正數。
	for name, in := range map[string]RevealKYCInput{
		"no reason": {AdminID: 9, ApplicationID: sub.ID}, "blank reason": {AdminID: 9, ApplicationID: sub.ID, Reason: " \n "},
		"overlong reason": {AdminID: 9, ApplicationID: sub.ID, Reason: strings.Repeat("理", 1001)}, "no admin": {ApplicationID: sub.ID, Reason: "核對證件"},
	} {
		if _, err := f.module.RevealKYCSensitive(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("reveal with %s: %v", name, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reveals"); n != 0 {
		t.Fatal("a refused reveal wrote an audit row")
	}
	got, err := f.module.RevealKYCSensitive(ctx, RevealKYCInput{AdminID: 9, ApplicationID: sub.ID, Reason: "核對證件"})
	must(t, err)
	if got.IDCardNo != nil || *got.RepIDCardNo != idRep || *got.BankAccount != bankAcct {
		t.Fatalf("revealed=%+v", got)
	}
	var adminID int64
	var reason string
	var at time.Time
	var rowText string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT admin_id, reason, revealed_at, row_to_json(kyc_reveals)::text FROM seller.kyc_reveals WHERE application_id=$1", sub.ID).Scan(&adminID, &reason, &at, &rowText))
	if adminID != 9 || reason != "核對證件" || !at.Equal(f.now) || strings.Contains(rowText, idRep) || strings.Contains(rowText, bankAcct) {
		t.Fatalf("audit row=%s", rowText)
	}
	if reveals := f.revealPages(t, sub.ID); len(reveals) != 1 || reveals[0].AdminID != 9 {
		t.Fatalf("reveals=%+v", reveals)
	}
	// 草稿不能顯示。
	other, _ := f.open(t)
	draft := f.savedDraft(t, kycDraft(other, idB, "123123123123"))
	if _, err := f.module.RevealKYCSensitive(ctx, RevealKYCInput{AdminID: 9, ApplicationID: draft.ID, Reason: "看看"}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("reveal a draft: %v", err)
	}
}

// AAD（申請 ID 加欄位名）防止密文在列與欄之間搬動：把別的申請、別的欄位的密文換進來，解不開（不會回傳別人的值）。
func TestKYCCiphertextCannotBeSwappedBetweenRowsOrColumns(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub1 := f.submitted(t, idA, bankAcct)
	_, _, sub2 := f.submitted(t, idB, "999000111222")
	reveal := func(id uuid.UUID) error {
		_, err := f.module.RevealKYCSensitive(ctx, RevealKYCInput{AdminID: 9, ApplicationID: id, Reason: "核對"})
		return err
	}
	if err := reveal(sub1.ID); err != nil {
		t.Fatal(err)
	}
	// 申請 2 的身分證號密文換成申請 1 的。
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.kyc_applications SET id_card_no_enc = (SELECT id_card_no_enc FROM seller.kyc_applications WHERE application_id = $1) WHERE application_id = $2", sub1.ID, sub2.ID)
	must(t, err)
	if err := reveal(sub2.ID); err == nil {
		t.Fatal("別的申請的密文不該解得開")
	}
	// 同一個申請內，銀行帳號的密文換成身分證號的。
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE seller.kyc_applications SET bank_account_enc = id_card_no_enc WHERE application_id = $1", sub1.ID)
	must(t, err)
	if err := reveal(sub1.ID); err == nil {
		t.Fatal("別的欄位的密文不該解得開")
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reveals WHERE application_id = $1", sub2.ID); n != 0 {
		t.Fatalf("解密失敗時整筆回滾，不該留下稽核紀錄：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reveals WHERE application_id = $1", sub1.ID); n != 1 {
		t.Fatalf("只有成功的那一次有稽核紀錄：%d", n)
	}
}

func TestKYCSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	for name, query := range map[string]string{
		"submitted without required fields":  "UPDATE seller.kyc_applications SET status='SUBMITTED', submitted_at=now(), submission_seq=1, real_name=NULL",
		"submitted without a submission seq": "UPDATE seller.kyc_applications SET status='SUBMITTED', submitted_at=now()",
		"a negative submission seq":          "UPDATE seller.kyc_applications SET submission_seq=-1",
		"individual with company fields":     "UPDATE seller.kyc_applications SET company_name='x'",
		"company with an individual id":      "UPDATE seller.kyc_applications SET seller_type='COMPANY'",
		"ciphertext without hash":            "UPDATE seller.kyc_applications SET id_card_hash=NULL",
		"short hash":                         "UPDATE seller.kyc_applications SET id_card_hash='\\x00'",
		"bad bank code":                      "UPDATE seller.kyc_applications SET bank_code='12'",
		"approved without a decision time":   "UPDATE seller.kyc_applications SET status='APPROVED', submission_seq=1",
		"landline as mobile":                 "UPDATE seller.kyc_applications SET contact_phone='0229010572'",
		"unknown status":                     "UPDATE seller.kyc_applications SET status='FORWARDED'",
		"unknown slot":                       "INSERT INTO seller.kyc_documents (application_id, slot, asset_id, object_key) SELECT application_id, 'SELFIE', 'a', 'k' FROM seller.kyc_applications",
		"review without a reviewer":          "INSERT INTO seller.kyc_reviews (review_id, application_id, action, decided_at) SELECT uuidv7(), application_id, 'APPROVE', now() FROM seller.kyc_applications",
		"reject without a reason":            "INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, decided_at) SELECT uuidv7(), application_id, 'REJECT', 1, now() FROM seller.kyc_applications",
		"rebind without a reviewer":          "INSERT INTO seller.kyc_reviews (review_id, application_id, action, decided_at) SELECT uuidv7(), application_id, 'REBIND', now() FROM seller.kyc_applications",
		"approve by a non positive admin":    "INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, decided_at) SELECT uuidv7(), application_id, 'APPROVE', 0, now() FROM seller.kyc_applications",
		"seller action with a reviewer":      "INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, decided_at) SELECT uuidv7(), application_id, 'SUBMIT', 1, now() FROM seller.kyc_applications",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, query); err == nil {
			t.Errorf("%s 應該被 CHECK 擋下", name)
		}
	}
}

// DB 的值集合等於 Go 的 Values()（同 TestChecksMatchGo）。
func TestKYCChecksMatchGo(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	strs := func(n int, at func(int) string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = at(i)
		}
		return out
	}
	statuses, types, slots, actions, kinds := KYCStatusValues(), model.KYCSellerTypeValues(), model.KYCDocumentSlotValues(), model.ReviewActionValues(), model.SanctionIdentityKindValues()
	for constraint, want := range map[string][]string{
		"kyc_applications_status_check":      strs(len(statuses), func(i int) string { return string(statuses[i]) }),
		"kyc_applications_seller_type_check": strs(len(types), func(i int) string { return string(types[i]) }),
		"kyc_documents_slot_check":           strs(len(slots), func(i int) string { return string(slots[i]) }),
		"kyc_reviews_action_check":           strs(len(actions), func(i int) string { return string(actions[i]) }),
		"measure_identities_kind_check":      strs(len(kinds), func(i int) string { return string(kinds[i]) }),
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

// 證件圖片的 media 引用以申請 ID 為 ref，由同步 job 登記：換圖後只有被換掉的那一張退場，取消後全部退場。
func TestKYCDocumentsLinkByApplicationIDAndSync(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	if len(f.media.finalized) != 3 {
		t.Fatalf("finalized=%v", f.media.finalized)
	}
	for _, refType := range f.media.finalized {
		if refType != "7111" {
			t.Fatalf("ref type=%q", refType)
		}
	}
	ref := media.Ref{Type: media.RefTypeCertification, ID: d.ID.String()}
	if !contains(f.jobs.kinds(), "seller.media_sync") || len(f.media.Referenced(ref)) != 0 {
		t.Fatalf("jobs=%v references=%v", f.jobs.kinds(), f.media.Referenced(ref))
	}
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); len(got) != 3 {
		t.Fatalf("references=%v", got)
	}
	// 換正面一張。
	swapped, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, IDCardFront: &KYCDocumentRef{StagedToken: "front2"}})
	must(t, err)
	var front string
	for _, doc := range swapped.Documents {
		if doc.Slot == SlotIDCardFront {
			front = doc.AssetID
		}
	}
	if front != "asset-front2" || len(swapped.Documents) != 3 {
		t.Fatalf("documents=%+v", swapped.Documents)
	}
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); len(got) != 3 || contains(got, "asset-front") || !contains(got, "asset-front2") {
		t.Fatalf("references=%v", got)
	}
	// 被換掉的證件不能寫回：同一個 staged token 重新轉正會回到同一個資產。
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: swapped.Version, IDCardFront: &KYCDocumentRef{StagedToken: "front"}})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("retired document written back: %v", err)
	}
	// 公司文件只有企業賣家。
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: swapped.Version, CompanyDocuments: &[]KYCDocumentRef{{StagedToken: "co"}}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("company document for an individual: %v", err)
	}
	_, err = f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: swapped.Version})
	must(t, err)
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("取消後應該全部解除：%v", got)
	}
}

// 回歸（CR2-4）：同步 job 在「轉正之後、寫入 commit 之前」執行，不得留下業務列指著卻沒有引用的證件。
func TestSyncBetweenFinalizeAndCommitKeepsKYCDocuments(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	ref := media.Ref{Type: media.RefTypeCertification, ID: d.ID.String()}
	f.syncMedia(t, ref)
	f.media.AfterFinalize = func(context.Context, media.Asset) {
		f.media.AfterFinalize = nil
		f.syncMedia(t, ref)
	}
	swapped, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, IDCardFront: &KYCDocumentRef{StagedToken: "front2"}})
	must(t, err)
	f.syncMedia(t, ref)
	got := f.media.Referenced(ref)
	if len(got) != 3 || !contains(got, "asset-front2") || contains(got, "asset-front") {
		t.Fatalf("references=%v，swapped=%+v", got, swapped.Documents)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// 授權由模組做（盤點 D22）：申請以帳號目前的店家為準；停權的賣家不能改申請，已關閉的店家沒有申請。
func TestKYCAuthorizationFollowsTheAccountsCurrentStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	stranger := f.account(t)
	if _, err := f.module.SaveKYCDraft(ctx, kycDraft(stranger, idA, bankAcct)); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("an account without a store: %v", err)
	}
	if _, err := f.module.MyKYC(ctx, stranger); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("my kyc without a store: %v", err)
	}
	owner, s := f.open(t)
	d := f.savedDraft(t, kycDraft(owner, idA, bankAcct))
	if _, err := f.module.MyKYC(ctx, owner); err != nil {
		t.Fatal(err)
	}
	// 另一個帳號只看得到自己店家的（沒有申請）。
	other, _ := f.open(t)
	if _, err := f.module.MyKYC(ctx, other); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("another store's application: %v", err)
	}
	sanction := f.suspend(t, s.StoreID)
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: owner, ExpectedVersion: d.Version, RealName: ptr("改名")}); apperr.KindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("suspended seller edits: %v", err)
	}
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: owner, ExpectedVersion: d.Version}); apperr.KindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("suspended seller submits: %v", err)
	}
	if _, err := f.module.MyKYC(ctx, owner); err != nil {
		t.Fatalf("a suspended seller can still read: %v", err)
	}
	_, err := f.lift(ctx, sanction)
	must(t, err)
	_, err = f.module.CloseCurrent(ctx, owner)
	must(t, err)
	if _, err := f.module.MyKYC(ctx, owner); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("a closed stint's application is not reachable: %v", err)
	}
	// 重新入駐的新 stint 沒有舊申請。
	_, err = f.module.Apply(ctx, validApply(owner))
	must(t, err)
	if _, err := f.module.MyKYC(ctx, owner); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("the new stint inherits the old application: %v", err)
	}
}

func TestKYCSwitchingSellerTypeClearsTheOtherTypesFields(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, companyDraft(accountID, idRep, taxIDOne, bankAcct))
	if *d.TaxID != taxIDOne || *d.RepIDCardNoMasked != "H2******10" || d.IDCardNoMasked != nil {
		t.Fatalf("company draft=%+v", d)
	}
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, IDCardNo: ptr(idA)}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("individual id on a company: %v", err)
	}
	ind, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, SellerType: individual(), IDCardNo: ptr(idA)})
	must(t, err)
	if ind.TaxID != nil || ind.CompanyName != nil || ind.RepIDCardNoMasked != nil || *ind.IDCardNoMasked != "A1******89" {
		t.Fatalf("individual draft=%+v", ind)
	}
	// 銀行代碼改了要以原本的帳號重算 blind index（比對才不會失準）；清掉代碼不能留下帳號。
	var before []byte
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT bank_account_hash FROM seller.kyc_applications").Scan(&before))
	changed, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: ind.Version, BankCode: ptr("812")})
	must(t, err)
	var after []byte
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT bank_account_hash FROM seller.kyc_applications").Scan(&after))
	if bytes.Equal(before, after) || *changed.BankAccountMasked != "********8901" {
		t.Fatalf("bank hash not recomputed or account lost: %+v", changed)
	}
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: changed.Version, BankCode: ptr("")}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("clearing the bank code but keeping the account: %v", err)
	}
}

func TestKYCInputValidation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	for name, in := range map[string]KYCDraftInput{
		"bad id checksum":        {IDCardNo: ptr("A123456780")},
		"short id":               {IDCardNo: ptr("a12345")},
		"bad bank account":       {BankAccount: ptr("12ab")},
		"bad bank code":          {BankCode: ptr("12")},
		"english name unicode":   {EnglishName: ptr("範例小農")},
		"english name too long":  {EnglishName: ptr(strings.Repeat("a", 51))},
		"bad website":            {WebsiteURL: ptr("ftp://x")},
		"unknown seller type":    {SellerType: ptr(KYCSellerType("GOV"))},
		"company field on indiv": {CompanyName: ptr("x")},
	} {
		in.AccountID, in.ExpectedVersion = accountID, d.Version
		if _, err := f.module.SaveKYCDraft(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	// 送出時一次列出全部缺的欄位。
	blank, _ := f.open(t)
	minimal := f.savedDraft(t, KYCDraftInput{AccountID: blank, SellerType: individual()})
	_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: blank, ExpectedVersion: minimal.Version})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) < 8 {
		t.Fatalf("submit an empty draft: %v", err)
	}
}

func TestAdminKYCListFiltersAndPaginates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	// 同一個身分證號可以出現在不同帳號的申請：沒有生效中的處分就不擋。
	names := []string{"王大明", "王小明", "李100%", "李1000", "陳_x", "陳ax"}
	for i, name := range names {
		accountID, _ := f.open(t)
		in := kycDraft(accountID, idA, fmt.Sprintf("%012d", i+1))
		in.RealName = ptr(name)
		d := f.savedDraft(t, in)
		f.now = f.now.Add(time.Minute)
		_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
		must(t, err)
	}
	// 一份草稿不在清單。
	draftAccount, _ := f.open(t)
	f.savedDraft(t, kycDraft(draftAccount, idC, "000000000099"))
	// 沒有篩選：送出時間新到舊。
	all := collectKYC(t, f, KYCListInput{PageSize: 2})
	if len(all) != 6 {
		t.Fatalf("all=%d（草稿不該在清單）", len(all))
	}
	for i := 1; i < len(all); i++ {
		if !all[i-1].SubmittedAt.After(*all[i].SubmittedAt) {
			t.Fatalf("not newest first: %v %v", all[i-1].SubmittedAt, all[i].SubmittedAt)
		}
	}
	if got := collectKYC(t, f, KYCListInput{Query: "王"}); len(got) != 2 {
		t.Fatalf("王 = %d", len(got))
	}
	// 關鍵字裡的 % 與 _ 是字面，不是萬用字元。
	if got := collectKYC(t, f, KYCListInput{Query: "100%"}); len(got) != 1 || *got[0].RealName != "李100%" {
		t.Fatalf("100%% = %+v", got)
	}
	if got := collectKYC(t, f, KYCListInput{Query: "_x"}); len(got) != 1 || *got[0].RealName != "陳_x" {
		t.Fatalf("_x = %+v", got)
	}
	if got := collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusApproved)}); len(got) != 0 {
		t.Fatalf("approved = %d", len(got))
	}
	if got := collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusSubmitted)}); len(got) != 6 {
		t.Fatalf("submitted = %d", len(got))
	}
	for _, bad := range []KYCStatus{KYCStatusDraft, "FORWARDED", ""} {
		if _, err := f.module.AdminKYCApplications(ctx, KYCListInput{Status: &bad}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("status %q: %v", bad, err)
		}
	}
}

// 審核佇列（篩選審核中）依送出時間舊到新，等最久的在最前面；其他狀態與沒有篩選新到舊。排序鍵是送出時間，不是草稿的建立
// 時間：先建立草稿、後送出的申請，排在後建立草稿、先送出的後面。翻頁不漏不重，同一個送出時間以申請 ID 定序。
func TestAdminKYCQueueIsOrderedBySubmitTime(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var drafts []KYCApplication
	var accounts []int64
	for i := range 4 {
		accountID, _ := f.open(t)
		in := kycDraft(accountID, idA, fmt.Sprintf("%012d", i+1))
		in.RealName = ptr(fmt.Sprintf("申請人%d", i))
		drafts = append(drafts, f.savedDraft(t, in))
		accounts = append(accounts, accountID)
		f.now = f.now.Add(time.Minute) // 草稿的建立時間依 0、1、2、3
	}
	// 送出的順序與建立草稿的順序相反：3、1、0，其中 2 與 0 同一個時間點送出（同刻以申請 ID 定序）。
	submitAt := func(i int) KYCApplication {
		a, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accounts[i], ExpectedVersion: drafts[i].Version})
		must(t, err)
		return a
	}
	base := f.now
	f.now = base
	third := submitAt(3)
	f.now = base.Add(time.Hour)
	second := submitAt(1)
	f.now = base.Add(2 * time.Hour)
	tieA, tieB := submitAt(0), submitAt(2)
	oldest := collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusSubmitted), PageSize: 1})
	ids := func(items []KYCApplication) []uuid.UUID {
		out := make([]uuid.UUID, len(items))
		for i, item := range items {
			out[i] = item.ID
		}
		return out
	}
	tie := []KYCApplication{tieA, tieB}
	slices.SortFunc(tie, func(a, b KYCApplication) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	want := ids(append([]KYCApplication{third, second}, tie...))
	if got := ids(oldest); !slices.Equal(got, want) {
		t.Fatalf("審核中的佇列要舊到新（同刻以申請 ID 升冪）：got=%v want=%v", got, want)
	}
	// 沒有篩選與其他狀態：新到舊（同刻以申請 ID 降冪）。
	newest := ids(collectKYC(t, f, KYCListInput{PageSize: 3}))
	slices.Reverse(want)
	if !slices.Equal(newest, want) {
		t.Fatalf("沒有篩選要新到舊：got=%v want=%v", newest, want)
	}
	// 核准一筆之後，它離開審核中的佇列、出現在已核准的清單；已核准的清單新到舊。
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: third.ID, ExpectedVersion: third.Version, AdminID: 9}); err != nil {
		t.Fatal(err)
	}
	f.now = base.Add(3 * time.Hour)
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: second.ID, ExpectedVersion: second.Version, AdminID: 9}); err != nil {
		t.Fatal(err)
	}
	if got := ids(collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusSubmitted)})); len(got) != 2 {
		t.Fatalf("approved ones must leave the queue: %v", got)
	}
	approved := collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusApproved), PageSize: 1})
	if len(approved) != 2 || approved[0].ID != second.ID || approved[1].ID != third.ID {
		t.Fatalf("approved list = %v", ids(approved))
	}
	// 沒送出就取消的申請不在佇列（它沒有進過審核）；送出之後才取消的在，狀態是 CANCELLED。
	accountID, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idB, "999888777666"))
	if _, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: d.Version}); err != nil {
		t.Fatal(err)
	}
	if got := collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusCancelled)}); len(got) != 0 {
		t.Fatalf("a cancelled draft never entered the queue: %d", len(got))
	}
	if got := collectKYC(t, f, KYCListInput{}); len(got) != 4 {
		t.Fatalf("all = %d", len(got))
	}
	cancelAccount, _ := f.open(t)
	cd := f.savedDraft(t, kycDraft(cancelAccount, idB, "111222333444"))
	cs, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: cancelAccount, ExpectedVersion: cd.Version})
	must(t, err)
	if _, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: cancelAccount, ExpectedVersion: cs.Version, Reason: "改天再申請"}); err != nil {
		t.Fatal(err)
	}
	if got := collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusCancelled)}); len(got) != 1 {
		t.Fatalf("a submitted-then-cancelled application is listed: %d", len(got))
	}
}

// 企業賣家的 real_name 是負責人，所以佇列也要能用公司名稱與統編找到它。
func TestAdminKYCListSearchesCompanyNameAndTaxID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := companyDraft(accountID, idRep, taxIDOne, bankAcct)
	in.RealName = ptr("負責人王")
	d := f.savedDraft(t, in)
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	otherAccount, _ := f.open(t)
	other := f.savedDraft(t, kycDraft(otherAccount, idB, "999888777666"))
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: otherAccount, ExpectedVersion: other.Version})
	must(t, err)
	for query, want := range map[string]int{"負責人王": 1, "範例小農": 1, "範例小農有限公司": 1, taxIDOne: 1, taxIDOne[:4]: 1, "沒有這個": 0, "": 2, "王小明": 1} {
		got := collectKYC(t, f, KYCListInput{Query: query})
		if len(got) != want {
			t.Errorf("%q → %d，要 %d", query, len(got), want)
		}
		if want == 1 && query != "王小明" && got[0].ID != sub.ID {
			t.Errorf("%q → %v", query, got[0].ID)
		}
	}
}

func collectKYC(t *testing.T, f *fixture, in KYCListInput) []KYCApplication {
	t.Helper()
	var out []KYCApplication
	seen := map[uuid.UUID]bool{}
	for {
		page, err := f.module.AdminKYCApplications(context.Background(), in)
		must(t, err)
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("duplicate %v", item.ID)
			}
			seen[item.ID] = true
			out = append(out, item)
		}
		if page.Next == nil {
			return out
		}
		in.After = page.Next
	}
}

// 能力在鎖內重新檢查（同 SaveDraft）：賣家的送出與取消讀到「沒有停權」之後、拿到店家列的鎖之前，帳號被停權時要被拒絕。
func TestKYCSubmitAndCancelRecheckTheCapabilityInsideTheLock(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		run  func(f *fixture, accountID int64, a KYCApplication) error
	}{
		{"submit", func(f *fixture, accountID int64, a KYCApplication) error {
			_, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: a.Version})
			return err
		}},
		{"cancel", func(f *fixture, accountID int64, a KYCApplication) error {
			_, err := f.module.CancelKYC(context.Background(), CancelKYCInput{AccountID: accountID, ExpectedVersion: a.Version})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			accountID, s := f.open(t)
			draft := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
			holder, err := f.db.Pool().Begin(ctx)
			must(t, err)
			defer func() { _ = holder.Rollback(ctx) }()
			_, err = holder.Exec(ctx, "SELECT 1 FROM seller.stores WHERE store_id=$1 FOR UPDATE", s.StoreID)
			must(t, err)
			done := make(chan error, 1)
			go func() { done <- tc.run(f, accountID, draft) }()
			f.waitForLockWaiters(t, 1)
			// 請求已經讀過「沒有停權」、卡在店家列的鎖上；這時另一個交易停權並 commit。
			measureID := uuid.NewV7()
			_, err = holder.Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
				VALUES ($1, $2, $3, 'SUSPENSION', 'RISK', 'FRAUD', $4, 'x', 1, 7, $5, $5, $5)`, measureID, accountID, s.StoreID, f.now.Add(7*24*time.Hour), f.now)
			must(t, err)
			_, err = holder.Exec(ctx, `INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ($1, 1, 'x', 7, $2)`, measureID, f.now)
			must(t, err)
			must(t, holder.Commit(ctx))
			if err := <-done; apperr.KindOf(err) != apperr.KindPermissionDenied {
				t.Fatalf("鎖內沒有重新檢查能力：%v", err)
			}
			if n := f.count(t, "SELECT count(*) FROM seller.kyc_applications WHERE application_id=$1 AND status='DRAFT'", draft.ID); n != 1 {
				t.Fatal("申請不該被改動")
			}
		})
	}
}

// 啟動檢查：資料庫參照的金鑰（密文開頭的 ID、blind index 的 ID、措施身分快照的 ID）都要在設定的金鑰環裡。輪替時新的金鑰
// 放前面、舊的留著就通過；把舊的拿掉就失敗，並列出缺的 ID。
func TestStartupChecksThatEveryKeyReferencedByTheDatabaseIsConfigured(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if err := f.module.CheckKYCKeys(ctx); err != nil {
		t.Fatalf("空的資料庫：%v", err)
	}
	_, s, _ := f.submitted(t, idA, bankAcct)
	f.suspend(t, s.StoreID) // 處分的身分快照也參照金鑰
	if err := f.module.CheckKYCKeys(ctx); err != nil {
		t.Fatalf("同一組金鑰：%v", err)
	}
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	modWith := func(encSpec, macSpec string) *Module {
		enc, err := seal.Parse(encSpec)
		must(t, err)
		mac, err := seal.ParseMAC(macSpec)
		must(t, err)
		return New(Deps{DB: f.db, Events: f.bus, Jobs: f.jobs, Identity: f.identity, Media: f.media, Orders: f.orders, KYCKeys: enc, KYCMAC: mac, ProductStores: f.products, Registry: f.registry,
			Now: func() time.Time { return f.now }, StoreCode: func(time.Time) (int64, error) { return 0, errors.New("unused") }})
	}
	oldEnc, oldMAC := "test1:"+b64("test-kyc-encryption-key-00000001"), "test1:"+b64("test-kyc-hash-hmac-key-000000001")
	newEnc, newMAC := "rot2:"+b64("rotated-kyc-encryption-key-00002"), "rot2:"+b64("rotated-kyc-hash-hmac-key-0000002")
	if err := modWith(newEnc+","+oldEnc, newMAC+","+oldMAC).CheckKYCKeys(ctx); err != nil {
		t.Fatalf("輪替後舊的金鑰還在：%v", err)
	}
	err := modWith(newEnc, newMAC).CheckKYCKeys(ctx)
	if err == nil || !strings.Contains(err.Error(), "ENCRYPTION:test1") || !strings.Contains(err.Error(), "HASH:test1") {
		t.Fatalf("金鑰被拿掉時要失敗並列出缺的 ID：%v", err)
	}
	if err := modWith(newEnc+","+oldEnc, newMAC).CheckKYCKeys(ctx); err == nil || !strings.Contains(err.Error(), "HASH:test1") || strings.Contains(err.Error(), "ENCRYPTION:") {
		t.Fatalf("只缺 HMAC 金鑰：%v", err)
	}
}

// 證件是狀態：公司文件整個換成請求的清單（現有的以 asset ID 保留、新的以 staged token），單張的位置可以清掉；
// 指定不是這份申請的證件、重複、超過上限都是 InvalidInput；被換掉的圖片退場並由同步 job 解除；在位置之間移動（順序反過來）的證件不退場。
func TestKYCDocumentsAreReplacedAsState(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := companyDraft(accountID, idRep, taxIDOne, bankAcct)
	in.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "co1"}, {StagedToken: "co2"}}
	d := f.savedDraft(t, in)
	kycRef := media.Ref{Type: media.RefTypeCertification, ID: d.ID.String()}
	f.syncMedia(t, kycRef)
	company := func(a KYCApplication) (ids []string) {
		for _, doc := range a.Documents {
			if doc.Slot == SlotCompanyDocument {
				ids = append(ids, doc.AssetID)
			}
		}
		return ids
	}
	if got := company(d); len(got) != 2 || got[0] != "asset-co1" || got[1] != "asset-co2" {
		t.Fatalf("documents=%+v", d.Documents)
	}
	// 保留第二張、換上第三張、順序反過來。
	next, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version,
		CompanyDocuments: &[]KYCDocumentRef{{StagedToken: "co3"}, {AssetID: "asset-co2"}}})
	must(t, err)
	if got := company(next); len(got) != 2 || got[0] != "asset-co3" || got[1] != "asset-co2" {
		t.Fatalf("documents=%+v", next.Documents)
	}
	f.syncMedia(t, kycRef)
	if got := f.media.Referenced(kycRef); contains(got, "asset-co1") || !contains(got, "asset-co2") || !contains(got, "asset-co3") {
		t.Fatalf("references=%v", got)
	}
	for name, refs := range map[string][]KYCDocumentRef{
		"不是這份申請的證件": {{AssetID: "asset-nope"}},
		"重複":        {{AssetID: "asset-co2"}, {AssetID: "asset-co2"}},
		"兩者都給":      {{AssetID: "asset-co2", StagedToken: "x"}},
		"兩者都空":      {{}},
		"超過上限":      make([]KYCDocumentRef, 11),
	} {
		if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: next.Version, CompanyDocuments: &refs}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	// 空清單清掉全部公司文件；單張的位置以空的指定清掉，別的位置的 asset ID 不能指定到這個位置。
	cleared, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: next.Version, CompanyDocuments: &[]KYCDocumentRef{}, BankBook: &KYCDocumentRef{}})
	must(t, err)
	if len(company(cleared)) != 0 || len(cleared.Documents) != 2 {
		t.Fatalf("documents=%+v", cleared.Documents)
	}
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: cleared.Version, BankBook: &KYCDocumentRef{AssetID: "asset-front"}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("別的位置的圖片：err=%v", err)
	}
}

// 統編走和登記資料查詢同一個規則（格式加 2023 年起的檢查碼）：格式對、檢查碼不對的統編草稿也收不下，整筆不寫入。
func TestKYCRejectsATaxIDWithABadChecksum(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := companyDraft(accountID, idRep, "12345678", bankAcct) // 檢查碼不符
	if _, err := f.module.SaveKYCDraft(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("檢查碼不符的統編：err=%v", err)
	}
	in = companyDraft(accountID, idRep, "10000073", bankAcct) // 第 7 位是 7 的特例
	if _, err := f.module.SaveKYCDraft(ctx, in); err != nil {
		t.Fatalf("合法的統編被拒絕：%v", err)
	}
}

// 同一個 staged token（同一張圖片）不能放進兩個證件位置：轉正之前就擋下（單張的位置之間、單張與公司文件之間都一樣）。
func TestKYCSameStagedTokenCannotFillTwoDocumentSlots(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := kycDraft(accountID, idA, bankAcct)
	in.IDCardBack = &KYCDocumentRef{StagedToken: "front"}
	if _, err := f.module.SaveKYCDraft(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("正反面同一張：err=%v", err)
	}
	if len(f.media.finalized) != 0 {
		t.Fatalf("擋下之前不該轉正：%v", f.media.finalized)
	}
	company := companyDraft(accountID, idRep, taxIDOne, bankAcct)
	company.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "front"}}
	if _, err := f.module.SaveKYCDraft(ctx, company); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("身分證正面與公司文件同一張：err=%v", err)
	}
	// 現有的圖片也不能同時放在兩個位置（保留路徑）。
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, IDCardBack: &KYCDocumentRef{AssetID: "asset-front"}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("保留的圖片放到別的位置：err=%v", err)
	}
}

// missing_fields 與送出檢查同一份規則：缺的欄位依 KYCFieldNames 的順序，個人與企業各自專屬的項目依型態，補齊之後是空的，
// 而且送出缺項時回的違規正好是這份清單。
func TestKYCMissingFieldsFollowTheSubmissionRule(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	empty := f.savedDraft(t, KYCDraftInput{AccountID: accountID, SellerType: individual()})
	wantIndividual := []string{"real_name", "english_name", "bank_name", "bank_code", "bank_account", "id_card_front", "id_card_back", "bank_book", "individual.id_card_no"}
	if !slices.Equal(empty.MissingFields, wantIndividual) {
		t.Fatalf("個人賣家缺：%v", empty.MissingFields)
	}
	_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: empty.Version})
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err=%v", err)
	}
	var violations []string
	for _, v := range ae.FieldViolations() {
		violations = append(violations, v.Field)
	}
	if !slices.Equal(violations, wantIndividual) {
		t.Fatalf("送出的違規與 missing_fields 不一致：%v", violations)
	}
	switched, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: empty.Version, SellerType: ptr(KYCCompany)})
	must(t, err)
	for _, want := range []string{"business.company_name", "business.tax_id", "business.rep_id_card_no"} {
		if !slices.Contains(switched.MissingFields, want) {
			t.Fatalf("企業賣家缺：%v", switched.MissingFields)
		}
	}
	if slices.Contains(switched.MissingFields, "individual.id_card_no") {
		t.Fatalf("企業賣家不缺個人的身分證號：%v", switched.MissingFields)
	}
	other, _ := f.open(t)
	if full := f.savedDraft(t, kycDraft(other, idB, bankAcct)); len(full.MissingFields) != 0 {
		t.Fatalf("齊全時應該是空的：%v", full.MissingFields)
	}
}

// 回歸：送出 KYC 要店家的 PayUni 綁定資料（地址、聯絡人姓名與手機、客服信箱）齊全；店家的狀態不在請求裡，缺的不是欄位違規而是
// failed_precondition，缺哪些看申請檢視的 missing_store_fields（與送出的檢查同一份規則）；申請自己的缺項先回 invalid_argument；
// 補齊之後才能送出。
func TestSubmitKYCRequiresTheStoreContactData(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	if len(d.MissingStoreFields) != 0 {
		t.Fatalf("店家資料齊全：%v", d.MissingStoreFields)
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET contact_name = NULL, support_email = NULL, town_code = NULL, address_detail = NULL WHERE store_id = $1", s.StoreID); err != nil {
		t.Fatal(err)
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if want := []string{"address", "contact_name", "support_email"}; !slices.Equal(mine.MissingStoreFields, want) || len(mine.MissingFields) != 0 {
		t.Fatalf("missing_store_fields=%v missing_fields=%v", mine.MissingStoreFields, mine.MissingFields)
	}
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("缺店家資料仍然送出了，或錯誤種類不對：%v", err)
	}
	var ae *apperr.Error
	if errors.As(err, &ae) && len(ae.FieldViolations()) != 0 {
		t.Fatalf("店家資料的缺項不是欄位違規：%v", ae.FieldViolations())
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action='SUBMIT'"); n != 0 {
		t.Fatalf("被拒絕的送出不該留審核紀錄：%d", n)
	}
	// 申請自己的缺項先檢查：回 invalid_argument 與欄位違規，不是店家的 failed_precondition。
	other, _ := f.open(t)
	blank := f.savedDraft(t, KYCDraftInput{AccountID: other, SellerType: individual()})
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET contact_name = NULL WHERE account_id = $1", other); err != nil {
		t.Fatal(err)
	}
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: other, ExpectedVersion: blank.Version})
	if apperr.KindOf(err) != apperr.KindInvalidInput || !errors.As(err, &ae) || len(ae.FieldViolations()) == 0 {
		t.Fatalf("申請缺項要先回欄位違規：%v", err)
	}
	fixed, err := f.module.UpdateProfile(ctx, UpdateProfileInput{
		StoreID: s.StoreID, ExpectedVersion: s.Version, ContactName: ptr("王小明"), SupportEmail: ptr("owner@example.com"),
		TownCode: ptr("63000050"), AddressDetail: ptr("測試路一號"),
	})
	must(t, err)
	if fixed.Version != s.Version+1 {
		t.Fatalf("version=%d", fixed.Version)
	}
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version}); err != nil {
		t.Fatalf("補齊之後應該能送出：%v", err)
	}
}

// 核准的 comment 是選填，但有長度上限（與理由同一條規則）：超過上限在寫入之前就回 InvalidInput，不是讓 DB 的 CHECK 擋成
// internal；被拒絕的核准什麼都不寫（申請仍在審核中、沒有審核紀錄）。
func TestApproveCommentIsValidatedBeforeTheDatabaseSeesIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submitted(t, idA, bankAcct)
	_, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Comment: strings.Repeat("理", 1001)})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("overlong comment: %v", err)
	}
	admin, err := f.module.AdminKYCApplication(ctx, sub.ID)
	must(t, err)
	if admin.Status != KYCStatusSubmitted || len(admin.Reviews) != 1 {
		t.Fatalf("a rejected approval must write nothing: %+v", admin)
	}
	// 去掉前後空白之後剛好 1000 字可以；空白的 comment 等於沒有。
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Comment: " " + strings.Repeat("理", 1000) + " "}); err != nil {
		t.Fatalf("1000 runes: %v", err)
	}
	approved, err := f.module.AdminKYCApplication(ctx, sub.ID)
	must(t, err)
	if r := approved.Reviews[len(approved.Reviews)-1]; r.Reason == nil || len([]rune(*r.Reason)) != 1000 {
		t.Fatalf("review=%+v", r)
	}
	_, _, blank := f.submitted(t, idB, "999888777666")
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: blank.ID, ExpectedVersion: blank.Version, AdminID: 9, Comment: " \n "}); err != nil {
		t.Fatalf("blank comment: %v", err)
	}
	got, err := f.module.AdminKYCApplication(ctx, blank.ID)
	must(t, err)
	if r := got.Reviews[len(got.Reviews)-1]; r.Action != ReviewApprove || r.Reason != nil {
		t.Fatalf("a blank comment must store no reason: %+v", r)
	}
}

// secureDocuments 把申請的每張證件放進 media 的替身（只有 (object_key, 證件的 ref_type, ref_id = 申請 ID) 全等讀得到），
// 內容是 "bytes:<位置>"。
func (f *fixture) secureDocuments(a KYCApplication) {
	for _, d := range a.Documents {
		f.media.PutSecure(d.ObjectKey, media.Ref{Type: media.RefTypeCertification, ID: a.ID.String()},
			media.SecureAsset{Data: []byte(fmt.Sprintf("bytes:%s:%d", d.Slot, d.Position)), ContentType: "image/jpeg"})
	}
}

func (f *fixture) reveal(t *testing.T, adminID int64, a KYCApplication) KYCSensitive {
	t.Helper()
	got, err := f.module.RevealKYCSensitive(context.Background(), RevealKYCInput{AdminID: adminID, ApplicationID: a.ID, Reason: "核對證件"})
	must(t, err)
	return got
}

func (f *fixture) readSecureCalls() int {
	n := 0
	for _, c := range f.media.Calls() {
		if strings.HasPrefix(c, "read-secure") {
			n++
		}
	}
	return n
}

// 證件圖片的存取是限時授權（JIT）：顯示全文的紀錄就是授權，15 分鐘（注入的時鐘），只有授權的那位管理員、只有這筆申請；
// 沒有授權、別人的授權、過期的授權都是 PermissionDenied（連申請存不存在都不透露），而且沒有讀取紀錄、媒體沒有被問。
func TestKYCDocumentAccessNeedsAnUnexpiredGrantOfTheSameAdmin(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submitted(t, idA, bankAcct)
	f.secureDocuments(sub)
	read := func(adminID int64, id uuid.UUID, slot KYCDocumentSlot, pos int) (FileContent, error) {
		return f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: adminID, ApplicationID: id, Slot: slot, Position: pos})
	}
	denied := func(name string, err error) {
		t.Helper()
		if apperr.KindOf(err) != apperr.KindPermissionDenied {
			t.Fatalf("%s：err=%v，要 PermissionDenied", name, err)
		}
		if n := f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses"); n != 0 || f.readSecureCalls() != 0 {
			t.Fatalf("%s：被拒絕的讀取不能留下讀取紀錄（%d）或問媒體（%d）", name, n, f.readSecureCalls())
		}
	}
	_, err := read(9, sub.ID, SlotIDCardFront, 0)
	denied("沒有授權", err)
	_, err = read(9, uuid.NewV7(), SlotIDCardFront, 0)
	denied("不存在的申請（不洩漏存在與否）", err)

	grant := f.reveal(t, 9, sub)
	if grant.GrantID == uuid.Nil() || !grant.ExpiresAt.Equal(f.now.Add(RevealGrantTTL)) || RevealGrantTTL != 15*time.Minute || len(grant.Documents) != 3 {
		t.Fatalf("grant=%+v", grant)
	}
	_, err = read(10, sub.ID, SlotIDCardFront, 0)
	denied("別的管理員", err)
	// 授權是這一筆申請的。
	_, _, other := f.submitted(t, idB, "999888777666")
	f.secureDocuments(other)
	_, err = read(9, other.ID, SlotIDCardFront, 0)
	denied("別筆申請", err)

	// 到期前一秒可以讀；到期那一刻（expires_at > now 不成立）不行。
	f.now = f.now.Add(RevealGrantTTL - time.Second)
	got, err := read(9, sub.ID, SlotIDCardFront, 0)
	must(t, err)
	if string(got.Data) != "bytes:ID_CARD_FRONT:0" || got.ContentType != "image/jpeg" {
		t.Fatalf("content=%+v", got)
	}
	var revealID uuid.UUID
	var slot string
	var position int
	var at time.Time
	var rowText string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT reveal_id, slot, position, accessed_at, row_to_json(kyc_document_accesses)::text FROM seller.kyc_document_accesses").Scan(&revealID, &slot, &position, &at, &rowText))
	if revealID != grant.GrantID || slot != "ID_CARD_FRONT" || position != 0 || !at.Equal(f.now) || !strings.Contains(rowText, `"asset_id":"asset-front"`) || strings.Contains(rowText, "7111/") || strings.Contains(rowText, "bytes:") {
		t.Fatalf("access row=%s", rowText)
	}
	f.now = f.now.Add(time.Second)
	before := f.readSecureCalls()
	_, err = read(9, sub.ID, SlotIDCardFront, 0)
	if apperr.KindOf(err) != apperr.KindPermissionDenied || f.readSecureCalls() != before || f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses") != 1 {
		t.Fatalf("過期的授權：err=%v", err)
	}
	// 再顯示一次就是新的授權（再寫一列稽核）。
	f.reveal(t, 9, sub)
	if _, err := read(9, sub.ID, SlotBankBook, 0); err != nil {
		t.Fatal(err)
	}

	// 稽核紀錄各自分頁（新到舊），列出誰、依據哪次授權、讀了哪張；每次讀取記實際送出的 asset ID。
	reveals := f.revealPages(t, sub.ID)
	if len(reveals) != 2 || !reveals[1].ExpiresAt.Equal(reveals[1].RevealedAt.Add(RevealGrantTTL)) || reveals[1].ID != grant.GrantID || !reveals[0].RevealedAt.After(reveals[1].RevealedAt) {
		t.Fatalf("reveals=%+v", reveals)
	}
	accesses := f.accessPages(t, sub.ID)
	if len(accesses) != 2 {
		t.Fatalf("accesses=%+v", accesses)
	}
	if a := accesses[1]; a.AdminID != 9 || a.Slot != SlotIDCardFront || a.RevealID != grant.GrantID {
		t.Fatalf("access=%+v", a)
	}
	if a := accesses[0]; a.AdminID != 9 || a.Slot != SlotBankBook || a.RevealID == grant.GrantID {
		t.Fatalf("access=%+v", a)
	}
	var frontAsset, bookAsset string
	for _, d := range sub.Documents {
		if d.Slot == SlotIDCardFront {
			frontAsset = d.AssetID
		}
		if d.Slot == SlotBankBook {
			bookAsset = d.AssetID
		}
	}
	for slot, want := range map[string]string{"ID_CARD_FRONT": frontAsset, "BANK_BOOK": bookAsset} {
		var got string
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT asset_id FROM seller.kyc_document_accesses WHERE slot=$1", slot).Scan(&got))
		if want == "" || got != want {
			t.Fatalf("%s 的讀取紀錄記的 asset_id = %q，要實際送出的 %q", slot, got, want)
		}
	}
}

// revealPages、accessPages 以每頁一筆走完一筆申請的稽核紀錄（新到舊），確認頁與頁之間不漏不重。
func (f *fixture) revealPages(t *testing.T, applicationID uuid.UUID) []KYCReveal {
	t.Helper()
	var out []KYCReveal
	in := KYCAuditListInput{ApplicationID: applicationID, PageSize: 1}
	for {
		page, err := f.module.ListKYCReveals(context.Background(), in)
		must(t, err)
		out = append(out, page.Items...)
		if page.Next == nil {
			return out
		}
		in.After = page.Next
	}
}

func (f *fixture) accessPages(t *testing.T, applicationID uuid.UUID) []KYCDocumentAccess {
	t.Helper()
	var out []KYCDocumentAccess
	in := KYCAuditListInput{ApplicationID: applicationID, PageSize: 1}
	for {
		page, err := f.module.ListKYCDocumentAccesses(context.Background(), in)
		must(t, err)
		out = append(out, page.Items...)
		if page.Next == nil {
			return out
		}
		in.After = page.Next
	}
}

// 有授權之後：位置沒有圖片是 NotFound（不寫讀取紀錄）；先寫讀取紀錄、再問媒體——紀錄寫不進去就不讀（fail closed）；
// 媒體讀失敗時紀錄仍在（嘗試也是紀錄）。
func TestKYCDocumentAccessIsRecordedBeforeTheBytesAreRead(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submitted(t, idA, bankAcct)
	f.secureDocuments(sub)
	f.reveal(t, 9, sub)
	read := func(slot KYCDocumentSlot, pos int) error {
		_, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Slot: slot, Position: pos})
		return err
	}
	for name, c := range map[string]struct {
		slot KYCDocumentSlot
		pos  int
	}{"個人賣家沒有公司文件": {SlotCompanyDocument, 0}, "單張的位置只有 0": {SlotIDCardFront, 1}, "位置超出範圍": {SlotCompanyDocument, 10}, "不認得的位置": {"SELFIE", 0}, "負的順序": {SlotBankBook, -1}} {
		if err := read(c.slot, c.pos); apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s：%v", name, err)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses") != 0 || f.readSecureCalls() != 0 {
		t.Fatal("找不到的位置不能留下紀錄或問媒體")
	}

	// 讀取紀錄寫不進去：不讀、不回傳。
	_, err := f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.kyc_document_accesses ADD CONSTRAINT test_no_writes CHECK (false) NOT VALID")
	must(t, err)
	got, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Slot: SlotBankBook, Position: 0})
	if err == nil || len(got.Data) != 0 || f.readSecureCalls() != 0 {
		t.Fatalf("紀錄寫不進去就不能讀：err=%v data=%q mediaCalls=%d", err, got.Data, f.readSecureCalls())
	}
	_, err = f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.kyc_document_accesses DROP CONSTRAINT test_no_writes")
	must(t, err)

	// 媒體讀失敗：紀錄還在，錯誤原樣交回。
	f.media.FailReadSecure = errors.New("media down")
	if err := read(SlotBankBook, 0); err == nil {
		t.Fatal("media 失敗要回錯誤")
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses"); n != 1 || f.readSecureCalls() != 1 {
		t.Fatalf("accesses=%d mediaCalls=%d", n, f.readSecureCalls())
	}
	f.media.FailReadSecure = nil
	if err := read(SlotBankBook, 0); err != nil {
		t.Fatal(err)
	}
}

// 個資清除之後（關店滿保留期）圖片不能再讀：NotFound，即使有一筆還沒到期的授權（直接寫入資料庫模擬）。
func TestKYCDocumentAccessOfAPurgedApplicationIsNotFound(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submitted(t, idA, bankAcct)
	f.secureDocuments(sub)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 {
		t.Fatalf("purged=%d", n)
	}
	// 清除之後不能再顯示全文（沒有新的授權）。
	if _, err := f.module.RevealKYCSensitive(ctx, RevealKYCInput{AdminID: 9, ApplicationID: sub.ID, Reason: "核對"}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("reveal after purge: %v", err)
	}
	_, err = f.db.Conn(ctx).Exec(ctx, "INSERT INTO seller.kyc_reveals (reveal_id, application_id, admin_id, reason, revealed_at, expires_at) VALUES (uuidv7(), $1, 9, '清除之前的授權', $2, $3)",
		sub.ID, f.now.Add(-time.Minute), f.now.Add(time.Hour))
	must(t, err)
	_, err = f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Slot: SlotIDCardFront, Position: 0})
	if apperr.KindOf(err) != apperr.KindNotFound || f.readSecureCalls() != 0 || f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses") != 0 {
		t.Fatalf("purged：err=%v", err)
	}
}

// 顯示全文與證件讀取的稽核紀錄不內嵌在申請檢視裡，各自分頁；申請不存在或是草稿回 NotFound（不是空的一頁）；頁大小的規則同其他清單。
func TestKYCAuditListsPageAndCheckTheParent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submitted(t, idA, bankAcct)
	f.secureDocuments(sub)
	for i := range 3 {
		f.now = f.now.Add(time.Minute)
		f.reveal(t, int64(9+i), sub)
	}
	for _, slot := range []KYCDocumentSlot{SlotIDCardFront, SlotIDCardBack, SlotBankBook} {
		if _, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 11, ApplicationID: sub.ID, Slot: slot}); err != nil {
			t.Fatal(err)
		}
	}
	reveals, accesses := f.revealPages(t, sub.ID), f.accessPages(t, sub.ID)
	if len(reveals) != 3 || len(accesses) != 3 || reveals[0].AdminID != 11 || reveals[2].AdminID != 9 {
		t.Fatalf("reveals=%+v accesses=%+v", reveals, accesses)
	}
	for i := 1; i < len(reveals); i++ {
		if !reveals[i-1].RevealedAt.After(reveals[i].RevealedAt) {
			t.Fatalf("不是新到舊：%+v", reveals)
		}
	}
	// 申請檢視只內嵌審核紀錄（筆數由人工動作限制），沒有稽核紀錄的欄位：型別上就放不進去。
	admin, err := f.module.AdminKYCApplication(ctx, sub.ID)
	must(t, err)
	if len(admin.Reviews) != 1 {
		t.Fatalf("reviews=%+v", admin.Reviews)
	}
	big, err := f.module.ListKYCReveals(ctx, KYCAuditListInput{ApplicationID: sub.ID})
	if err != nil || len(big.Items) != 3 || big.Next != nil {
		t.Fatalf("預設頁大小：%+v err=%v", big, err)
	}
	if _, err := f.module.ListKYCReveals(ctx, KYCAuditListInput{ApplicationID: sub.ID, PageSize: -1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("負的頁大小：%v", err)
	}
	accountID, _ := f.open(t)
	draft := f.savedDraft(t, kycDraft(accountID, idB, "123123123123"))
	for name, id := range map[string]uuid.UUID{"unknown": uuid.NewV7(), "draft": draft.ID} {
		if _, err := f.module.ListKYCReveals(ctx, KYCAuditListInput{ApplicationID: id}); apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s reveals：%v", name, err)
		}
		if _, err := f.module.ListKYCDocumentAccesses(ctx, KYCAuditListInput{ApplicationID: id}); apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s accesses：%v", name, err)
		}
	}
}

// 核准與退回回傳的後台檢視在同一個 transaction 內讀：含剛寫入的審核紀錄與新的狀態、版本（呼叫方不必事後再讀一次）。
func TestApproveAndRejectReturnTheAdminViewIncludingTheirOwnReview(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, a := f.submitted(t, idA, bankAcct)
	_, _, b := f.submitted(t, idB, "999888777666")
	approved, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: a.ID, ExpectedVersion: a.Version, AdminID: 9, Comment: "齊全"})
	must(t, err)
	if approved.Status != KYCStatusApproved || approved.Version != a.Version+1 || len(approved.Reviews) != 2 || approved.Reviews[1].Action != ReviewApprove ||
		*approved.Reviews[1].ReviewerAdminID != 9 || approved.Flow == nil || approved.Flow.Status != FlowBinding {
		t.Fatalf("approved=%+v", approved)
	}
	rejected, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: b.ID, ExpectedVersion: b.Version, AdminID: 9, Reason: "存摺不清楚", Issues: []KYCIssue{{Field: "bank_book", DocumentAssetID: "asset-book", Code: IssueUnreadable, Message: "反光"}}})
	must(t, err)
	if rejected.Status != KYCStatusRejected || len(rejected.Reviews) != 2 || rejected.Reviews[1].Action != ReviewReject || rejected.LastRejection == nil {
		t.Fatalf("rejected=%+v", rejected)
	}
}

// 證件必須是機敏上傳：media 回報 is_secure=false 的轉正在寫入之前就被拒絕（invalid_argument，欄位違規指向那個證件欄位），
// 什麼都不寫。
func TestKYCDocumentsMustBeSecureAssets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.media.NotSecure = true
	accountID, _ := f.open(t)
	_, err := f.module.SaveKYCDraft(context.Background(), kycDraft(accountID, idA, bankAcct))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) == 0 || ae.FieldViolations()[0].Field != "IDCardFront" {
		t.Fatalf("err=%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_applications"); n != 0 {
		t.Fatalf("被拒絕的草稿不能留下申請：%d", n)
	}
	// 公司文件同樣。
	in := companyDraft(accountID, idRep, taxIDOne, bankAcct)
	in.IDCardFront, in.IDCardBack, in.BankBook = nil, nil, nil
	in.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "doc1"}}
	if _, err := f.module.SaveKYCDraft(context.Background(), in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("company document：%v", err)
	}
	f.media.NotSecure = false
	if _, err := f.module.SaveKYCDraft(context.Background(), kycDraft(accountID, idA, bankAcct)); err != nil {
		t.Fatalf("機敏的證件可以存：%v", err)
	}
}

// media 讀不到證件時的三種結果：引用的同步還沒結算（暫時，ErrFileNotSynced）、同步過了卻讀不到（資料不一致，ErrFileMissing）、
// 不是機敏資產（media.ErrNotSecure 原樣）。三種都先寫了讀取紀錄（嘗試也是紀錄）。
func TestKYCDocumentReadFailuresAreDistinguished(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submitted(t, idA, bankAcct)
	f.reveal(t, 9, sub)
	read := func() error {
		_, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Slot: SlotIDCardFront})
		return err
	}
	// 圖片沒有放進 media 的替身（讀不到），同步還沒做。
	if err := read(); !errors.Is(err, ErrFileNotSynced) || errors.Is(err, ErrFileMissing) {
		t.Fatalf("同步還沒結算：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses"); n != 1 {
		t.Fatalf("accesses=%d", n)
	}
	f.syncMedia(t, media.Ref{Type: media.RefTypeCertification, ID: sub.ID.String()})
	if err := read(); !errors.Is(err, ErrFileMissing) || errors.Is(err, ErrFileNotSynced) {
		t.Fatalf("同步過了還讀不到：%v", err)
	}
	f.media.FailReadSecure = media.ErrNotSecure
	if err := read(); !errors.Is(err, media.ErrNotSecure) || errors.Is(err, ErrFileMissing) || apperr.KindOf(err) == apperr.KindNotFound {
		t.Fatalf("不是機敏資產：%v", err)
	}
	f.media.FailReadSecure = nil
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses"); n != 3 {
		t.Fatalf("三次嘗試都要有紀錄：%d", n)
	}
}

// 佇列的兩支 SQL（舊到新與新到舊）篩選條件必須一致：同一組篩選，審核中的結果集合相同，只有順序相反。沒有篩選（新到舊）的結果
// 裡取出審核中的，要等於篩選審核中（舊到新）的結果反過來。
func TestAdminKYCQueueVariantsFilterIdentically(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	type seed struct {
		name   string
		action string // approve、reject、cancel、none
		bank   string
	}
	seeds := []seed{{"甲", "none", "000000000001"}, {"乙", "approve", "000000000002"}, {"丙", "none", "000000000003"}, {"丁", "reject", "000000000004"}, {"戊", "cancel", "000000000005"}, {"己", "none", "000000000006"}}
	for _, sd := range seeds {
		accountID, _ := f.open(t)
		in := kycDraft(accountID, idA, sd.bank)
		in.RealName = ptr(sd.name + "小明")
		if sd.name == "丙" {
			in = companyDraft(accountID, idRep, taxIDOne, sd.bank)
			in.RealName = ptr("負責人丙")
		}
		d := f.savedDraft(t, in)
		f.now = f.now.Add(time.Minute)
		sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
		must(t, err)
		switch sd.action {
		case "approve":
			_, err = f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9})
		case "reject":
			_, err = f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "x"})
		case "cancel":
			_, err = f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: sub.Version})
		}
		must(t, err)
	}
	// 沒有送出的草稿與沒送出就取消的，兩支都不列。
	draftAccount, _ := f.open(t)
	f.savedDraft(t, kycDraft(draftAccount, idB, "000000000099"))
	ids := func(items []KYCApplication) []uuid.UUID {
		out := make([]uuid.UUID, len(items))
		for i, item := range items {
			out[i] = item.ID
		}
		return out
	}
	for _, query := range []string{"", "小明", "負責人", "丙", taxIDOne, "範例小農", "不存在", "%", "_"} {
		oldest := ids(collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusSubmitted), Query: query, PageSize: 2}))
		var fromAll []uuid.UUID
		for _, item := range collectKYC(t, f, KYCListInput{Query: query, PageSize: 2}) {
			if item.Status == KYCStatusSubmitted {
				fromAll = append(fromAll, item.ID)
			}
		}
		slices.Reverse(fromAll)
		if !slices.Equal(oldest, fromAll) {
			t.Errorf("query %q：舊到新 %v 與新到舊反過來 %v 不一致", query, oldest, fromAll)
		}
	}
	if got := collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusSubmitted)}); len(got) != 3 {
		t.Fatalf("審核中 = %d，要 3（甲、丙、己）", len(got))
	}
}

// 取消的申請沒有可讀的證件：取消時證件已記成退場並解除引用（kyc_documents 的列還在），所以授權之後讀證件是 NotFound，不寫讀取紀錄、
// 不問 media；取消之後再顯示全文，欄位照舊可顯示，但沒有證件的檢視網址。
func TestCancelledApplicationHasNoReadableDocuments(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submitted(t, idA, bankAcct)
	f.secureDocuments(sub)
	granted := f.reveal(t, 9, sub)
	if len(granted.Documents) != 3 {
		t.Fatalf("取消之前有證件：%+v", granted.Documents)
	}
	_, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: sub.Version, Reason: "改天再申請"})
	must(t, err)
	if f.count(t, "SELECT count(*) FROM seller.kyc_documents WHERE application_id=$1", sub.ID) == 0 {
		t.Fatal("前提：取消之後 kyc_documents 的列還在")
	}
	before := f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses")
	_, err = f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Slot: SlotIDCardFront})
	if apperr.KindOf(err) != apperr.KindNotFound || f.readSecureCalls() != 0 || f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses") != before {
		t.Fatalf("取消之後：err=%v mediaCalls=%d", err, f.readSecureCalls())
	}
	again := f.reveal(t, 9, sub)
	if again.IDCardNo == nil || *again.IDCardNo != idA || again.BankAccount == nil || len(again.Documents) != 0 {
		t.Fatalf("取消之後欄位照舊可顯示、沒有證件網址：%+v", again)
	}
}

// media 說沒有時，用「呼叫 media 之前」讀到的同步狀態分類：呼叫期間才結算的同步，不會被誤判成資料不一致。
func TestKYCDocumentClassificationUsesTheSyncStateFromBeforeTheMediaCall(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submitted(t, idA, bankAcct)
	f.reveal(t, 9, sub)
	ref := media.Ref{Type: media.RefTypeCertification, ID: sub.ID.String()}
	f.media.BeforeReadSecure = func() { f.syncMedia(t, ref) } // 同步在 media 呼叫進行中結算
	_, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Slot: SlotIDCardFront})
	if !errors.Is(err, ErrFileNotSynced) || errors.Is(err, ErrFileMissing) {
		t.Fatalf("呼叫之前還沒結算：%v", err)
	}
	f.media.BeforeReadSecure = nil
	// 已經結算之後，同樣的讀不到才是不一致。
	if _, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Slot: SlotIDCardFront}); !errors.Is(err, ErrFileMissing) {
		t.Fatalf("呼叫之前已結算：%v", err)
	}
}

// media 拒絕的 staged token（無效、過期或不屬於這個帳號）是那個證件位置的欄位錯誤：單張的位置指到位置，公司文件指到請求清單的
// 第幾張（索引是請求裡的位置，不是只算有新 token 的），media 的原因留在錯誤裡給 log。
func TestKYCRejectedUploadPointsAtTheDocumentPosition(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	f.media.rejected = map[string]error{"gone": fmt.Errorf("mediaservice：轉正（code=404）：%w：%w", media.ErrStagedTokenRejected, apperr.InvalidInput("圖片無法使用，請重新上傳"))}
	d := f.savedDraft(t, companyDraft(accountID, idRep, taxIDOne, bankAcct))
	fields := func(err error) []string {
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
			return nil
		}
		var out []string
		for _, fv := range ae.FieldViolations() {
			out = append(out, fv.Field)
		}
		return out
	}
	company := &[]KYCDocumentRef{{AssetID: "keep"}, {StagedToken: "ok"}, {StagedToken: "gone"}}
	_, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, CompanyDocuments: company})
	if got := fields(err); !slices.Equal(got, []string{"CompanyDocuments[2]"}) {
		t.Fatalf("公司文件的欄位 = %v（%v）", got, err)
	}
	if err == nil || !strings.Contains(err.Error(), "code=404") {
		t.Fatalf("缺少 media 的原因：%v", err)
	}
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, BankBook: &KYCDocumentRef{StagedToken: "gone"}})
	if got := fields(err); !slices.Equal(got, []string{"BankBook"}) {
		t.Fatalf("單張位置的欄位 = %v（%v）", got, err)
	}
}

// 所有被 media 拒絕的 token 一次回報（單張的位置與公司文件一起，各指到自己的位置）；轉正失敗的請求不寫入任何證件。
func TestKYCReportsEveryRejectedUploadAtOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	gone := func() error {
		return fmt.Errorf("mediaservice：轉正（code=404）：%w：%w", media.ErrStagedTokenRejected, apperr.InvalidInput("圖片無法使用，請重新上傳"))
	}
	f.media.rejected = map[string]error{"gone1": gone(), "gone2": gone(), "gone3": gone()}
	d := f.savedDraft(t, companyDraft(accountID, idRep, taxIDOne, bankAcct))
	company := &[]KYCDocumentRef{{StagedToken: "gone1"}, {StagedToken: "ok"}, {StagedToken: "gone2"}}
	_, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, BankBook: &KYCDocumentRef{StagedToken: "gone3"}, CompanyDocuments: company})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("err = %v", err)
	}
	var got []string
	for _, v := range ae.FieldViolations() {
		got = append(got, v.Field)
	}
	if want := []string{"BankBook", "CompanyDocuments[0]", "CompanyDocuments[2]"}; !slices.Equal(got, want) {
		t.Fatalf("欄位 = %v，預期 %v", got, want)
	}
}

// 單一元素的問題（資產與 token 同時指定、空白、重複的 token、不是目前證件的現有圖片）指到那一張（CompanyDocuments[i]）；只有整個清單
// 的問題（張數上限）不帶索引。
func TestKYCCompanyDocumentElementProblemsAreIndexed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, companyDraft(accountID, idRep, taxIDOne, bankAcct))
	fields := func(docs []KYCDocumentRef) []string {
		_, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, CompanyDocuments: &docs})
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
			t.Fatalf("err = %v", err)
		}
		var out []string
		for _, v := range ae.FieldViolations() {
			out = append(out, v.Field)
		}
		return out
	}
	for name, c := range map[string]struct {
		docs []KYCDocumentRef
		want string
	}{
		"資產與 token 同時指定": {[]KYCDocumentRef{{StagedToken: "a"}, {AssetID: "x", StagedToken: "b"}}, "CompanyDocuments[1]"},
		"空白":             {[]KYCDocumentRef{{StagedToken: "a"}, {}}, "CompanyDocuments[1]"},
		"重複的 token":      {[]KYCDocumentRef{{StagedToken: "a"}, {StagedToken: "b"}, {StagedToken: "a"}}, "CompanyDocuments[2]"},
		"不是目前證件的圖片":      {[]KYCDocumentRef{{AssetID: "nope"}}, "CompanyDocuments[0]"},
		"張數上限":           {make([]KYCDocumentRef, 50), "CompanyDocuments"},
	} {
		if got := fields(c.docs); !slices.Equal(got, []string{c.want}) {
			t.Errorf("%s：欄位 = %v，預期 %s", name, got, c.want)
		}
	}
}
