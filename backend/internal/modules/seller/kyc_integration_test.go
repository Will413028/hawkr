//go:build integration

package seller_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"regexp"
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
	"github.com/jackc/pgx/v5/pgconn"
)

// 測試用的統編（檢查碼正確，不是真實的公司）。
const (
	taxIDOne = "33330333"
	taxIDTwo = "04595257"
)

func individual() *KYCSellerType { return ptr(KYCIndividual) }

// kycDraft 是一份完整、合法的個人賣家草稿（個人賣家沒有必填的身分欄位，也沒有文件）。
func kycDraft(accountID int64) KYCDraftInput {
	return KYCDraftInput{AccountID: accountID, SellerType: individual(), ProductType: ptr("蔬果")}
}

// companyDraft 是一份完整、合法的企業賣家草稿（公司名稱與統編，沒有文件）。
func companyDraft(accountID int64, taxID string) KYCDraftInput {
	return KYCDraftInput{
		AccountID: accountID, SellerType: ptr(KYCCompany), CompanyName: ptr("範例小農有限公司"), TaxID: ptr(taxID), ProductType: ptr("蔬果"),
	}
}

// savedDraft 建立草稿；回傳申請。
func (f *fixture) savedDraft(t *testing.T, in KYCDraftInput) KYCApplication {
	t.Helper()
	a, err := f.module.SaveKYCDraft(context.Background(), in)
	must(t, err)
	return a
}

// submitted 開店、建立並送出一份個人賣家的申請。
func (f *fixture) submitted(t *testing.T) (int64, Store, KYCApplication) {
	t.Helper()
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID))
	a, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	return accountID, s, a
}

// submittedCompany 開店、建立並送出一份企業賣家的申請。
func (f *fixture) submittedCompany(t *testing.T, taxID string) (int64, Store, KYCApplication) {
	t.Helper()
	accountID, s := f.open(t)
	d := f.savedDraft(t, companyDraft(accountID, taxID))
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
	in := kycDraft(accountID)
	d := f.savedDraft(t, in)
	if d.Status != KYCStatusDraft || d.Version != 1 || d.StoreID != s.StoreID || d.AccountID != accountID || len(d.Documents) != 0 {
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
	if admin.Status != KYCStatusApproved || admin.DecidedAt == nil || len(admin.Reviews) != 2 {
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
	// 核准 KYC 不碰店家的 profile：店家的版本不動、沒有 profile 事件。
	after, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if after.Version != storeBefore.Version || len(profile.all()) != 0 {
		t.Fatalf("store=%+v profile events=%d", after, len(profile.all()))
	}
	// 已核准的不能再核准、退回、取消或修改。
	if err := approve(f, admin.KYCApplication); apperr.KindOf(err) != apperr.KindFailedPrecondition && apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("approve twice: %v", err)
	}
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: admin.Version, ProductType: ptr("水果")}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
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
	accountID, _, sub := f.submitted(t)
	for name, reason := range map[string]string{"none": "", "blank": " \t ", "overlong": strings.Repeat("理", 1001)} {
		if _, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: reason}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("reject with a %s reason: %v", name, err)
		}
	}
	if _, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, Issues: anIssue(), Reason: "x"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("reject without an admin: %v", err)
	}
	rej, err := f.module.RejectKYC(ctx, RejectKYCInput{
		ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Reason: "資料不完整",
		Issues: []KYCIssue{
			{Field: "product_type", Code: IssueMismatch, Message: "與營業項目不符"},
			{Field: "website_url", Code: IssueInvalid, Message: "網址打不開"},
		},
	})
	must(t, err)
	if rej.Status != KYCStatusRejected || len(rejected.all()) != 1 || rejected.all()[0].Reason != "資料不完整" || rejected.all()[0].AccountID != accountID {
		t.Fatalf("rejected=%+v events=%+v", rej, rejected.all())
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	wantIssues := []KYCIssue{
		{Field: "product_type", Code: IssueMismatch, Message: "與營業項目不符"},
		{Field: "website_url", Code: IssueInvalid, Message: "網址打不開"},
	}
	if mine.LastRejection == nil || mine.LastRejection.Reason != "資料不完整" || !slices.Equal(mine.LastRejection.Issues, wantIssues) {
		t.Fatalf("last rejection=%+v", mine.LastRejection)
	}
	// 被退回的申請編輯後回到 DRAFT，沿用同一筆（同一個 ID）；不需要重新建立。
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: rej.Version, ProductType: ptr("水果")})
	must(t, err)
	if edited.ID != sub.ID || edited.Status != KYCStatusDraft || edited.DecidedAt != nil || *edited.ProductType != "水果" {
		t.Fatalf("edited=%+v", edited)
	}
	// 改掉的商品類型算已處理，沒動的網址還在待處理。
	if !slices.Equal(edited.OutstandingIssues, wantIssues[1:]) {
		t.Fatalf("outstanding=%+v", edited.OutstandingIssues)
	}
	// 最近一次退回在回到草稿與再次送出之後的回應仍在（與 MyKYC 同一份檢視）。
	if edited.LastRejection == nil || edited.LastRejection.Reason != "資料不完整" || !slices.Equal(edited.LastRejection.Issues, wantIssues) {
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
	accountID, _, sub := f.submitted(t)
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: sub.Version}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("submit while under review: %v", err)
	}
	rej, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "資料不完整"})
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

// 平台不收身分證號、負責人身分證號、銀行代碼與帳號、存摺與身分證影像、本名、英文姓名、生日與證件發證資料（Stripe 計畫 D22、D37）：KYC 的表
// （information_schema.columns）與申請的輸入、檢視型別都沒有這些欄位，儲存這些資料的表（kyc_reveals、kyc_identity_hashes）也不存在，申請也不再有自己的統編 hash 欄位（D42，防詐保存只在 account_identity_hashes）。
// mutation：在 kyc_applications 加回任何一個被刪的欄位（例如 real_name）、或在 KYCDraftInput 加回欄位 → 本測試失敗（只在 CI 的 integration 驗證）。
func TestKYCHasNoPersonalIdentityOrBankColumnsOrFields(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	forbidden := regexp.MustCompile(`id_card|idcard|bank|real_name|realname|english_name|englishname|birth|issue_date|issue_type|issue_place|slot|reveal|tax_id_hash`)
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema = 'seller' AND table_name IN ('kyc_applications', 'kyc_documents', 'kyc_document_accesses', 'kyc_reviews', 'kyc_review_issues', 'kyc_registration_checks')`)
	must(t, err)
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var table, column string
		must(t, rows.Scan(&table, &column))
		seen++
		if forbidden.MatchString(strings.ToLower(column)) {
			t.Errorf("seller.%s.%s 是平台不再收集的資料", table, column)
		}
	}
	must(t, rows.Err())
	if seen == 0 {
		t.Fatal("information_schema 沒有讀到 KYC 的欄位，測試空轉")
	}
	var gone int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = 'seller' AND table_name IN ('kyc_reveals', 'kyc_identity_hashes')`).Scan(&gone))
	if gone != 0 {
		t.Errorf("kyc_reveals 與 kyc_identity_hashes 應該已刪除：%d", gone)
	}
	var walk func(path string, typ reflect.Type, seen map[reflect.Type]bool)
	walk = func(path string, typ reflect.Type, visited map[reflect.Type]bool) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || visited[typ] {
			return
		}
		visited[typ] = true
		for i := range typ.NumField() {
			field := typ.Field(i)
			if forbidden.MatchString(strings.ToLower(field.Name)) {
				t.Errorf("%s.%s 是平台不再收集的資料", path, field.Name)
			}
			walk(path+"."+field.Name, field.Type, visited)
		}
	}
	for name, v := range map[string]any{
		"KYCDraftInput": KYCDraftInput{}, "KYCApplication": KYCApplication{}, "KYCAdminApplication": KYCAdminApplication{}, "KYCDocumentAccess": KYCDocumentAccess{},
	} {
		walk(name, reflect.TypeOf(v), map[reflect.Type]bool{})
	}
}

// 回歸（盤點 D12）：舊實作的日期解析失敗只 log、欄位沿用舊值或留空。壞日期整筆拒絕、不寫入，原本的值與版本不變。
func TestKYCInvalidDatesAreRejectedNotKept(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	bad := companyDraft(accountID, taxIDOne)
	bad.CompanyRegisterDate = ptr("2026-02-30")
	if _, err := f.module.SaveKYCDraft(ctx, bad); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("create with a bad date: %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_applications"); n != 0 {
		t.Fatalf("a rejected draft was written: %d", n)
	}
	good := companyDraft(accountID, taxIDOne)
	good.CompanyRegisterDate = ptr("2015-03-04")
	d := f.savedDraft(t, good)
	for name, value := range map[string]string{
		"not a date": "yesterday", "wrong order": "06-05-1990", "ROC year": "079-05-06", "nonexistent day": "2025-02-29",
		"future": "2026-10-02", "before 1900": "1899-12-31",
	} {
		in := KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, CompanyRegisterDate: ptr(value), CompanyName: ptr("被誤寫的名字")}
		if _, err := f.module.SaveKYCDraft(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("CompanyRegisterDate %s = %q 應該被拒絕：%v", name, value, err)
		}
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if mine.Version != d.Version || *mine.CompanyRegisterDate != "2015-03-04" || *mine.CompanyName != "範例小農有限公司" {
		t.Fatalf("a rejected update changed the draft: %+v", mine)
	}
	// 今天（台北）本身是有效的日期。
	ok, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, CompanyRegisterDate: ptr("2026-10-01")})
	must(t, err)
	if *ok.CompanyRegisterDate != "2026-10-01" {
		t.Fatalf("register date=%v", ok.CompanyRegisterDate)
	}
}

// 回歸（盤點 D20）：舊實作靠 sellers.current_certification_id 找目前的申請，寫入失敗只 log，之後會再建一筆。新的以 store_id 為鍵，
// 一個店家同時最多一筆還沒取消的申請由 DB 保證；取消後可以重新建立。
func TestOneOpenKYCApplicationPerStoreIsEnforcedByTheDatabase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID))
	insert := "INSERT INTO seller.kyc_applications (application_id, store_id, seller_type, status, submission_seq, created_at, updated_at) VALUES (uuidv7(), $1, 'INDIVIDUAL', $2::text, CASE WHEN $2::text = 'DRAFT' THEN 0 ELSE 1 END, now(), now())"
	for _, status := range []string{"DRAFT", "SUBMITTED"} {
		if _, err := f.db.Conn(ctx).Exec(ctx, insert, s.StoreID, status); err == nil {
			t.Fatalf("a second %s application for the same store was accepted", status)
		}
	}
	// 經模組：已有申請時再「建立」是 AlreadyExists（帶 VERSION_CONFLICT），不會多一筆。
	if _, err := f.module.SaveKYCDraft(ctx, kycDraft(accountID)); apperr.KindOf(err) != apperr.KindAlreadyExists {
		t.Fatalf("create twice: %v", err)
	}
	cancelled, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: d.Version, Reason: "改天再填"})
	must(t, err)
	if cancelled.Status != KYCStatusCancelled {
		t.Fatalf("cancelled=%+v", cancelled)
	}
	again := f.savedDraft(t, kycDraft(accountID))
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
	_, s, sub := f.submitted(t)
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
	accountID, _, sub := f.submitted(t)
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
	_, s, sub := f.submitted(t)
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

// 身分比對：別的帳號有生效中處分的統編，不能通過認證；送出時先查，核准時在鎖內再查一次（以後者為準）；錯誤是泛用的，不透露命中哪個欄位。
// 個人賣家沒有身分欄位，不被比對。
func TestKYCIdentityMatchBlocksAcrossAccounts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	// 帳號 B 以統編入駐過、之後被停權。
	bAccount, bStore := f.open(t)
	f.savedDraft(t, companyDraft(bAccount, taxIDOne))
	f.suspend(t, bStore.StoreID)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 1 {
		t.Fatalf("snapshot rows=%d（統編一列）", n)
	}
	accountID, _ := f.open(t)
	d := f.savedDraft(t, companyDraft(accountID, taxIDOne))
	_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("同一個統編：err=%v", err)
	}
	if m := err.Error(); strings.Contains(m, "統") || strings.Contains(m, "停權") {
		t.Fatalf("錯誤不能透露命中的欄位或原因：%q", m)
	}
	// 不相干的統編、沒有身分欄位的個人賣家可以送出。
	for name, draft := range map[string]func(int64) KYCDraftInput{
		"不相干的統編": func(a int64) KYCDraftInput { return companyDraft(a, taxIDTwo) },
		"個人賣家":   func(a int64) KYCDraftInput { return kycDraft(a) },
	} {
		accountID, _ := f.open(t)
		d := f.savedDraft(t, draft(accountID))
		if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version}); err != nil {
			t.Fatalf("%s：%v", name, err)
		}
	}
}

// 核准時在鎖內再查一次，以後者為準：送出時還沒有處分，核准前另一個帳號被停權，核准就要失敗。
func TestKYCApproveRechecksTheIdentityInsideTheTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	bAccount, bStore := f.open(t)
	f.savedDraft(t, companyDraft(bAccount, taxIDOne))
	_, _, sub := f.submittedCompany(t, taxIDOne) // 送出時 B 還沒被處分
	f.suspend(t, bStore.StoreID)
	err := approve(f, sub)
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || strings.Contains(err.Error(), "統編") {
		t.Fatalf("approve: %v", err)
	}
	if got, err := f.module.AdminKYCApplication(ctx, sub.ID); err != nil || got.Status != KYCStatusSubmitted {
		t.Fatalf("status=%v err=%v", got.Status, err)
	}
}

// 措施結束或到期之後不再擋，快照也刪掉：解除在同一個 transaction 刪除；到期的處分，時間一過就不擋（stint.Derive，不等
// job），到期 job 與對帳再把快照刪掉。違規產生的處分一樣有快照。
func TestKYCLiftedOrExpiredSanctionNoLongerBlocksAndItsSnapshotIsDeleted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	blockedSubmit := func(taxID string) bool {
		accountID, _ := f.open(t)
		d := f.savedDraft(t, companyDraft(accountID, taxID))
		_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
		return err != nil
	}
	// 管理員停權：解除後刪除。
	aAccount, aStore := f.open(t)
	f.savedDraft(t, companyDraft(aAccount, taxIDOne))
	sanction := f.suspend(t, aStore.StoreID)
	if !blockedSubmit(taxIDOne) || f.count(t, "SELECT count(*) FROM seller.measure_identities") == 0 {
		t.Fatal("停權中的統編應該被擋，快照應該存在")
	}
	_, err := f.lift(ctx, sanction)
	must(t, err)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 0 {
		t.Fatalf("解除後快照還在：%d", n)
	}
	if blockedSubmit(taxIDOne) {
		t.Fatal("解除後不該再擋")
	}
	// 違規的停權：有快照；到期後時間一過就不擋（job 還沒跑）；job 之後快照刪除。
	bAccount, bStore := f.open(t)
	f.savedDraft(t, companyDraft(bAccount, taxIDTwo))
	endsAt := f.now.Add(48 * time.Hour)
	f.violate(t, bStore.StoreID, SeverityHigh, MeasureSuspension, &endsAt)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 1 {
		t.Fatalf("違規的處分也要有快照（統編）：%d", n)
	}
	if !blockedSubmit(taxIDTwo) {
		t.Fatal("違規停權中的統編應該被擋")
	}
	f.now = endsAt.Add(time.Minute)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 1 {
		t.Fatalf("job 還沒跑，快照還在：%d", n)
	}
	if blockedSubmit(taxIDTwo) {
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
	f.savedDraft(t, kycDraft(accountID))
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

func TestKYCSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	f.savedDraft(t, kycDraft(accountID))
	for name, query := range map[string]string{
		"submitted company without its registration data": "UPDATE seller.kyc_applications SET status='SUBMITTED', submitted_at=now(), submission_seq=1, seller_type='COMPANY', company_name=NULL",
		"submitted without a submission seq":              "UPDATE seller.kyc_applications SET status='SUBMITTED', submitted_at=now()",
		"a negative submission seq":                       "UPDATE seller.kyc_applications SET submission_seq=-1",
		"individual with company fields":                  "UPDATE seller.kyc_applications SET company_name='x'",
		"individual with a tax id":                        "UPDATE seller.kyc_applications SET tax_id='33330333'",
		"website without a scheme":                        "UPDATE seller.kyc_applications SET website_url='farm.example.com'",
		"approved without a decision time":                "UPDATE seller.kyc_applications SET status='APPROVED', submission_seq=1",
		"landline as mobile":                              "UPDATE seller.stores SET contact_mobile='0229010572'",
		"unknown status":                                  "UPDATE seller.kyc_applications SET status='FORWARDED'",
		"document position out of range":                  "INSERT INTO seller.kyc_documents (application_id, position, asset_id, object_key) SELECT application_id, 10, 'a', 'k' FROM seller.kyc_applications",
		"review without a reviewer":                       "INSERT INTO seller.kyc_reviews (review_id, application_id, action, decided_at) SELECT uuidv7(), application_id, 'APPROVE', now() FROM seller.kyc_applications",
		"reject without a reason":                         "INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, decided_at) SELECT uuidv7(), application_id, 'REJECT', 1, now() FROM seller.kyc_applications",
		"retired rebind action":                           "INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, decided_at) SELECT uuidv7(), application_id, 'REBIND', 1, now() FROM seller.kyc_applications",
		"approve by a non positive admin":                 "INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, decided_at) SELECT uuidv7(), application_id, 'APPROVE', 0, now() FROM seller.kyc_applications",
		"seller action with a reviewer":                   "INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, decided_at) SELECT uuidv7(), application_id, 'SUBMIT', 1, now() FROM seller.kyc_applications",
	} {
		_, err := f.db.Conn(ctx).Exec(ctx, query)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "23514" {
			t.Errorf("%s 應該被 CHECK 擋下（23514）：%v", name, err)
		}
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.kyc_applications SET product_type=NULL"); err != nil {
		t.Errorf("草稿不受送出時的必填限制：%v", err)
	}
	companyAccount, companyStore := f.open(t)
	f.savedDraft(t, companyDraft(companyAccount, taxIDOne))
	const submit = "UPDATE seller.kyc_applications SET status='SUBMITTED', submitted_at=now(), submission_seq=1%s WHERE store_id=$1"
	for name, extra := range map[string]string{
		"company submitted without a tax id":       ", tax_id=NULL",
		"company with a malformed tax id":          ", tax_id='1234'",
		"company submitted without a company name": ", company_name=NULL",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, fmt.Sprintf(submit, extra), companyStore.StoreID); err == nil {
			t.Errorf("%s 應該被 CHECK 擋下", name)
		}
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, fmt.Sprintf(submit, ""), companyStore.StoreID); err != nil {
		t.Errorf("公司賣家有公司名稱、統編與收款聯絡信箱就能送出：%v", err)
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
	statuses, types, actions, kinds := KYCStatusValues(), model.KYCSellerTypeValues(), model.ReviewActionValues(), model.SanctionIdentityKindValues()
	for constraint, want := range map[string][]string{
		"kyc_applications_status_check":      strs(len(statuses), func(i int) string { return string(statuses[i]) }),
		"kyc_applications_seller_type_check": strs(len(types), func(i int) string { return string(types[i]) }),
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

// 公司文件的 media 引用以申請 ID 為 ref，由同步 job 登記：換圖後只有被換掉的那一張退場，取消後全部退場。
func TestKYCDocumentsLinkByApplicationIDAndSync(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := companyDraft(accountID, taxIDOne)
	in.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "co1"}, {StagedToken: "co2"}}
	d := f.savedDraft(t, in)
	if len(f.media.finalized) != 2 {
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
	if got := f.media.Referenced(ref); len(got) != 2 {
		t.Fatalf("references=%v", got)
	}
	// 換第二張。
	swapped, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, CompanyDocuments: &[]KYCDocumentRef{{AssetID: "asset-co1"}, {StagedToken: "co3"}}})
	must(t, err)
	var assets []string
	for _, doc := range swapped.Documents {
		assets = append(assets, doc.AssetID)
	}
	if !slices.Equal(assets, []string{"asset-co1", "asset-co3"}) {
		t.Fatalf("documents=%+v", swapped.Documents)
	}
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); len(got) != 2 || contains(got, "asset-co2") || !contains(got, "asset-co3") {
		t.Fatalf("references=%v", got)
	}
	// 被換掉的文件不能寫回：同一個 staged token 重新轉正會回到同一個資產。
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: swapped.Version, CompanyDocuments: &[]KYCDocumentRef{{AssetID: "asset-co1"}, {StagedToken: "co2"}}})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("retired document written back: %v", err)
	}
	_, err = f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: swapped.Version})
	must(t, err)
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("取消後應該全部解除：%v", got)
	}
	// 公司文件只有企業賣家。
	individualAccount, _ := f.open(t)
	individualDraft := f.savedDraft(t, kycDraft(individualAccount))
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: individualAccount, ExpectedVersion: individualDraft.Version, CompanyDocuments: &[]KYCDocumentRef{{StagedToken: "co"}}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("company document for an individual: %v", err)
	}
}

// 回歸（CR2-4）：同步 job 在「轉正之後、寫入 commit 之前」執行，不得留下業務列指著卻沒有引用的公司文件。
func TestSyncBetweenFinalizeAndCommitKeepsKYCDocuments(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := companyDraft(accountID, taxIDOne)
	in.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "co1"}, {StagedToken: "co2"}}
	d := f.savedDraft(t, in)
	ref := media.Ref{Type: media.RefTypeCertification, ID: d.ID.String()}
	f.syncMedia(t, ref)
	f.media.AfterFinalize = func(context.Context, media.Asset) {
		f.media.AfterFinalize = nil
		f.syncMedia(t, ref)
	}
	swapped, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, CompanyDocuments: &[]KYCDocumentRef{{AssetID: "asset-co1"}, {StagedToken: "co3"}}})
	must(t, err)
	f.syncMedia(t, ref)
	got := f.media.Referenced(ref)
	if len(got) != 2 || !contains(got, "asset-co3") || contains(got, "asset-co2") {
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
	if _, err := f.module.SaveKYCDraft(ctx, kycDraft(stranger)); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("an account without a store: %v", err)
	}
	if _, err := f.module.MyKYC(ctx, stranger); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("my kyc without a store: %v", err)
	}
	owner, s := f.open(t)
	d := f.savedDraft(t, kycDraft(owner))
	if _, err := f.module.MyKYC(ctx, owner); err != nil {
		t.Fatal(err)
	}
	// 另一個帳號只看得到自己店家的（沒有申請）。
	other, _ := f.open(t)
	if _, err := f.module.MyKYC(ctx, other); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("another store's application: %v", err)
	}
	sanction := f.suspend(t, s.StoreID)
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: owner, ExpectedVersion: d.Version, ProductType: ptr("水果")}); apperr.KindOf(err) != apperr.KindPermissionDenied {
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
	d := f.savedDraft(t, companyDraft(accountID, taxIDOne))
	if *d.TaxID != taxIDOne || *d.CompanyName == "" {
		t.Fatalf("company draft=%+v", d)
	}
	ind, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, SellerType: individual()})
	must(t, err)
	if ind.TaxID != nil || ind.CompanyName != nil || ind.CompanyRegisterDate != nil || ind.CompanyPhone != nil || ind.ProductType == nil {
		t.Fatalf("individual draft=%+v", ind)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE account_id = $1 AND field = 'TAX_ID'", accountID); n != 0 {
		t.Fatalf("換成個人之後統編的防詐列要修剪：%d", n)
	}
}

func TestKYCInputValidation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID))
	for name, in := range map[string]KYCDraftInput{
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
	minimal := f.savedDraft(t, KYCDraftInput{AccountID: blank, SellerType: ptr(KYCCompany)})
	_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: blank, ExpectedVersion: minimal.Version})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 2 {
		t.Fatalf("submit an empty draft: %v", err)
	}
}

func TestAdminKYCListFiltersAndPaginates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	// 清單以店名搜尋（D37）。
	names := []string{"王大明農場", "王小明農場", "李100%農場", "李1000農場", "陳_x農場", "陳ax農場"}
	for _, name := range names {
		accountID, s := f.open(t)
		_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET store_name = $1 WHERE store_id = $2", name, s.StoreID)
		must(t, err)
		d := f.savedDraft(t, kycDraft(accountID))
		f.now = f.now.Add(time.Minute)
		_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
		must(t, err)
	}
	// 一份草稿不在清單。
	draftAccount, _ := f.open(t)
	f.savedDraft(t, kycDraft(draftAccount))
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
	if got := collectKYC(t, f, KYCListInput{Query: "100%"}); len(got) != 1 || *got[0].StoreName != "李100%農場" {
		t.Fatalf("100%% = %+v", got)
	}
	if got := collectKYC(t, f, KYCListInput{Query: "_x"}); len(got) != 1 || *got[0].StoreName != "陳_x農場" {
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
	for range 4 {
		accountID, _ := f.open(t)
		drafts = append(drafts, f.savedDraft(t, kycDraft(accountID)))
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
	d := f.savedDraft(t, kycDraft(accountID))
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
	cd := f.savedDraft(t, kycDraft(cancelAccount))
	cs, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: cancelAccount, ExpectedVersion: cd.Version})
	must(t, err)
	if _, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: cancelAccount, ExpectedVersion: cs.Version, Reason: "改天再申請"}); err != nil {
		t.Fatal(err)
	}
	if got := collectKYC(t, f, KYCListInput{Status: ptr(KYCStatusCancelled)}); len(got) != 1 {
		t.Fatalf("a submitted-then-cancelled application is listed: %d", len(got))
	}
}

// 佇列用店名、公司名稱與統編找申請（D37：不再用負責人的本名）。
func TestAdminKYCListSearchesCompanyNameAndTaxID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, companyDraft(accountID, taxIDOne))
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	otherAccount, otherStore := f.open(t)
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET store_name = '個人農場' WHERE store_id = $1", otherStore.StoreID)
	must(t, err)
	other := f.savedDraft(t, kycDraft(otherAccount))
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: otherAccount, ExpectedVersion: other.Version})
	must(t, err)
	for query, want := range map[string]int{"範例小農": 1, "範例小農有限公司": 1, taxIDOne: 1, taxIDOne[:4]: 1, "沒有這個": 0, "": 2, "個人農場": 1, "王小明": 0} {
		got := collectKYC(t, f, KYCListInput{Query: query})
		if len(got) != want {
			t.Errorf("%q → %d，要 %d", query, len(got), want)
		}
		if want == 1 && query != "個人農場" && got[0].ID != sub.ID {
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
			draft := f.savedDraft(t, kycDraft(accountID))
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

// 啟動檢查：資料庫參照的金鑰（blind index 的 ID、措施身分快照的 ID）都要在設定的金鑰環裡。輪替時新的金鑰
// 放前面、舊的留著就通過；把舊的拿掉就失敗，並列出缺的 ID。
func TestStartupChecksThatEveryKeyReferencedByTheDatabaseIsConfigured(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if err := f.module.CheckKYCKeys(ctx); err != nil {
		t.Fatalf("空的資料庫：%v", err)
	}
	_, s, _ := f.submittedCompany(t, taxIDOne)
	f.suspend(t, s.StoreID) // 處分的身分快照也參照金鑰
	if err := f.module.CheckKYCKeys(ctx); err != nil {
		t.Fatalf("同一組金鑰：%v", err)
	}
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	modWith := func(macSpec string) *Module {
		mac, err := seal.ParseMAC(macSpec)
		must(t, err)
		return New(Deps{DB: f.db, Events: f.bus, Jobs: f.jobs, Identity: f.identity, Media: f.media, Orders: f.orders, KYCMAC: mac, ProductStores: f.products, Registry: f.registry, Payment: f.payments,
			Now: func() time.Time { return f.now }, StoreCode: func(time.Time) (int64, error) { return 0, errors.New("unused") }})
	}
	oldMAC := "test1:" + b64("test-kyc-hash-hmac-key-000000001")
	newMAC := "rot2:" + b64("rotated-kyc-hash-hmac-key-0000002")
	if err := modWith(newMAC + "," + oldMAC).CheckKYCKeys(ctx); err != nil {
		t.Fatalf("輪替後舊的金鑰還在：%v", err)
	}
	err := modWith(newMAC).CheckKYCKeys(ctx)
	if err == nil || !strings.Contains(err.Error(), "HASH:test1") {
		t.Fatalf("金鑰被拿掉時要失敗並列出缺的 ID：%v", err)
	}
}

// 證件是狀態：公司文件整個換成請求的清單（現有的以 asset ID 保留、新的以 staged token）；
// 指定不是這份申請的證件、重複、超過上限都是 InvalidInput；被換掉的圖片退場並由同步 job 解除；在位置之間移動（順序反過來）的證件不退場。
func TestKYCDocumentsAreReplacedAsState(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := companyDraft(accountID, taxIDOne)
	in.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "co1"}, {StagedToken: "co2"}}
	d := f.savedDraft(t, in)
	kycRef := media.Ref{Type: media.RefTypeCertification, ID: d.ID.String()}
	f.syncMedia(t, kycRef)
	company := func(a KYCApplication) (ids []string) {
		for _, doc := range a.Documents {
			ids = append(ids, doc.AssetID)
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
	// 空清單清掉全部公司文件。
	cleared, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: next.Version, CompanyDocuments: &[]KYCDocumentRef{}})
	must(t, err)
	if len(cleared.Documents) != 0 {
		t.Fatalf("documents=%+v", cleared.Documents)
	}
}

// 統編走和登記資料查詢同一個規則（格式加 2023 年起的檢查碼）：格式對、檢查碼不對的統編草稿也收不下，整筆不寫入。
func TestKYCRejectsATaxIDWithABadChecksum(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := companyDraft(accountID, "12345678") // 檢查碼不符
	if _, err := f.module.SaveKYCDraft(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("檢查碼不符的統編：err=%v", err)
	}
	in = companyDraft(accountID, "10000073") // 第 7 位是 7 的特例
	if _, err := f.module.SaveKYCDraft(ctx, in); err != nil {
		t.Fatalf("合法的統編被拒絕：%v", err)
	}
}

// 同一個 staged token（同一張圖片）不能放進兩個位置：轉正之前就擋下；現有的圖片也不能在同一份清單出現兩次（保留路徑）。
func TestKYCSameStagedTokenCannotFillTwoDocumentPositions(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	company := companyDraft(accountID, taxIDOne)
	company.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "front"}, {StagedToken: "front"}}
	if _, err := f.module.SaveKYCDraft(ctx, company); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("兩個位置同一張：err=%v", err)
	}
	if len(f.media.finalized) != 0 {
		t.Fatalf("擋下之前不該轉正：%v", f.media.finalized)
	}
	company.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "co1"}}
	d := f.savedDraft(t, company)
	if _, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, CompanyDocuments: &[]KYCDocumentRef{{AssetID: "asset-co1"}, {AssetID: "asset-co1"}}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("保留的圖片放到兩個位置：err=%v", err)
	}
}

// missing_fields 與送出檢查同一份規則：缺的欄位依 KYCFieldNames 的順序，企業專屬的項目依型態，補齊之後是空的，
// 而且送出缺項時回的違規正好是這份清單。身分、銀行與證件不在必填之內。
func TestKYCMissingFieldsFollowTheSubmissionRule(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	empty := f.savedDraft(t, KYCDraftInput{AccountID: accountID, SellerType: individual()})
	if len(empty.MissingFields) != 0 {
		t.Fatalf("個人賣家缺：%v", empty.MissingFields)
	}
	switched, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: empty.Version, SellerType: ptr(KYCCompany)})
	must(t, err)
	wantCompany := []string{"business.company_name", "business.tax_id"}
	if !slices.Equal(switched.MissingFields, wantCompany) {
		t.Fatalf("企業賣家缺：%v", switched.MissingFields)
	}
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: switched.Version})
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err=%v", err)
	}
	var violations []string
	for _, v := range ae.FieldViolations() {
		violations = append(violations, v.Field)
	}
	if !slices.Equal(violations, wantCompany) {
		t.Fatalf("送出的違規與 missing_fields 不一致：%v", violations)
	}
	other, _ := f.open(t)
	if full := f.savedDraft(t, kycDraft(other)); len(full.MissingFields) != 0 {
		t.Fatalf("齊全時應該是空的：%v", full.MissingFields)
	}
}

// 送出只要求市集登記資料與收款聯絡信箱：不帶身分、銀行、證件的個人與企業申請都能送出並核准。
// mutation：把身分欄位加回 requirements → 本測試與 TestRequirementsAreMarketplaceDataOnly 失敗；
// kyc_applications_complete_check 改回含身分與銀行欄位 → 送出被資料庫的約束擋下，本測試失敗（只在 CI 的 integration 驗證）。
func TestSubmitAndApproveWithoutIdentityBankOrDocuments(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for name, draft := range map[string]func(accountID int64) KYCDraftInput{
		"個人": func(accountID int64) KYCDraftInput {
			return KYCDraftInput{AccountID: accountID, SellerType: individual(), ProductType: ptr("蔬果")}
		},
		"企業": func(accountID int64) KYCDraftInput {
			return KYCDraftInput{
				AccountID: accountID, SellerType: ptr(KYCCompany), ProductType: ptr("蔬果"),
				CompanyName: ptr("範例小農有限公司"), TaxID: ptr(taxIDOne),
			}
		},
	} {
		accountID, _ := f.open(t)
		d := f.savedDraft(t, draft(accountID))
		if len(d.MissingFields) != 0 {
			t.Fatalf("%s：missing_fields=%v", name, d.MissingFields)
		}
		sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
		if err != nil || sub.Status != KYCStatusSubmitted {
			t.Fatalf("%s：送出 err=%v status=%v", name, err, sub.Status)
		}
		must(t, approve(f, sub))
		mine, err := f.module.MyKYC(ctx, accountID)
		must(t, err)
		if mine.Status != KYCStatusApproved {
			t.Fatalf("%s：核准後 status=%v", name, mine.Status)
		}
	}
}

// 回歸：送出 KYC 要店家的必填資料（地址、客服信箱）齊全，聯絡人姓名與手機是選填；店家的狀態不在請求裡，缺的不是欄位違規而是
// failed_precondition，缺哪些看申請檢視的 missing_store_fields（與送出的檢查同一份規則）；申請自己的缺項先回 invalid_argument；
// 補齊之後才能送出。
func TestSubmitKYCRequiresTheStoreContactData(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID))
	if len(d.MissingStoreFields) != 0 {
		t.Fatalf("店家資料齊全：%v", d.MissingStoreFields)
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET contact_name = NULL, contact_mobile = NULL, support_email = NULL, town_code = NULL, address_detail = NULL WHERE store_id = $1", s.StoreID); err != nil {
		t.Fatal(err)
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if want := []string{"address", "support_email"}; !slices.Equal(mine.MissingStoreFields, want) || len(mine.MissingFields) != 0 {
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
	blank := f.savedDraft(t, KYCDraftInput{AccountID: other, SellerType: ptr(KYCCompany)})
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET support_email = NULL WHERE account_id = $1", other); err != nil {
		t.Fatal(err)
	}
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: other, ExpectedVersion: blank.Version})
	if apperr.KindOf(err) != apperr.KindInvalidInput || !errors.As(err, &ae) || len(ae.FieldViolations()) == 0 {
		t.Fatalf("申請缺項要先回欄位違規：%v", err)
	}
	fixed, err := f.module.UpdateProfile(ctx, UpdateProfileInput{
		StoreID: s.StoreID, ExpectedVersion: s.Version, SupportEmail: ptr("owner@example.com"),
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
	_, _, sub := f.submitted(t)
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
	_, _, blank := f.submitted(t)
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: blank.ID, ExpectedVersion: blank.Version, AdminID: 9, Comment: " \n "}); err != nil {
		t.Fatalf("blank comment: %v", err)
	}
	got, err := f.module.AdminKYCApplication(ctx, blank.ID)
	must(t, err)
	if r := got.Reviews[len(got.Reviews)-1]; r.Action != ReviewApprove || r.Reason != nil {
		t.Fatalf("a blank comment must store no reason: %+v", r)
	}
}

// submittedWithDocuments 開店、建立並送出一份帶兩張公司補充文件的企業賣家申請。
func (f *fixture) submittedWithDocuments(t *testing.T) (int64, Store, KYCApplication) {
	t.Helper()
	return f.companySubmission(t, func(in *KYCDraftInput) {
		in.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "co1"}, {StagedToken: "co2"}}
	})
}

// secureDocuments 把申請的每張補充文件放進 media 的替身（只有 (object_key, 證件的 ref_type, ref_id = 申請 ID) 全等讀得到），
// 內容是 "bytes:<位置>"。
func (f *fixture) secureDocuments(a KYCApplication) {
	for _, d := range a.Documents {
		f.media.PutSecure(d.ObjectKey, media.Ref{Type: media.RefTypeCertification, ID: a.ID.String()},
			media.SecureAsset{Data: []byte(fmt.Sprintf("bytes:%d", d.Position)), ContentType: "image/jpeg"})
	}
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

// accessPages 以每頁一筆走完一筆申請的補充文件讀取紀錄（新到舊），確認頁與頁之間不漏不重。
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

// 後台的 KYC 讀取權限由路由檢查（httpapi 的測試），模組只管讀取紀錄：位置沒有文件是 NotFound（不寫讀取紀錄）；先寫讀取紀錄、再問媒體——
// 紀錄寫不進去就不讀（fail closed）；媒體讀失敗時紀錄仍在（嘗試也是紀錄）；每次成功的讀取各寫一列。
// mutation：讀取之後才寫紀錄、或成功時不寫紀錄 → 本測試失敗（只在 CI 的 integration 驗證）。
func TestKYCDocumentAccessIsRecordedBeforeTheBytesAreRead(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submittedWithDocuments(t)
	f.secureDocuments(sub)
	_, _, individualSub := f.submitted(t)
	read := func(a KYCApplication, pos int) error {
		_, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: a.ID, Position: pos})
		return err
	}
	for name, c := range map[string]struct {
		app KYCApplication
		pos int
	}{"個人賣家沒有公司文件": {individualSub, 0}, "沒有這個位置": {sub, 2}, "位置超出範圍": {sub, 10}, "負的順序": {sub, -1}} {
		if err := read(c.app, c.pos); apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s：%v", name, err)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses") != 0 || f.readSecureCalls() != 0 {
		t.Fatal("找不到的位置不能留下紀錄或問媒體")
	}

	// 讀取紀錄寫不進去：不讀、不回傳。
	_, err := f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.kyc_document_accesses ADD CONSTRAINT test_no_writes CHECK (false) NOT VALID")
	must(t, err)
	got, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Position: 0})
	if err == nil || len(got.Data) != 0 || f.readSecureCalls() != 0 {
		t.Fatalf("紀錄寫不進去就不能讀：err=%v data=%q mediaCalls=%d", err, got.Data, f.readSecureCalls())
	}
	_, err = f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.kyc_document_accesses DROP CONSTRAINT test_no_writes")
	must(t, err)

	// 媒體讀失敗：紀錄還在，錯誤原樣交回。
	f.media.FailReadSecure = errors.New("media down")
	if err := read(sub, 0); err == nil {
		t.Fatal("media 失敗要回錯誤")
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses"); n != 1 || f.readSecureCalls() != 1 {
		t.Fatalf("accesses=%d mediaCalls=%d", n, f.readSecureCalls())
	}
	f.media.FailReadSecure = nil
	// 成功的讀取：每一次各寫一列（誰、哪一張、何時）。
	if err := read(sub, 0); err != nil {
		t.Fatal(err)
	}
	if err := read(sub, 1); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses WHERE application_id = $1 AND admin_id = 9", sub.ID); n != 3 {
		t.Fatalf("三次嘗試各一列：%d", n)
	}
}

// 個資清除之後（關店滿保留期）文件不能再讀：NotFound，不寫讀取紀錄、不問媒體。
func TestKYCDocumentAccessOfAPurgedApplicationIsNotFound(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submittedWithDocuments(t)
	f.secureDocuments(sub)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 {
		t.Fatalf("purged=%d", n)
	}
	_, err = f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Position: 0})
	if apperr.KindOf(err) != apperr.KindNotFound || f.readSecureCalls() != 0 || f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses") != 0 {
		t.Fatalf("purged：err=%v", err)
	}
}

// 補充文件的讀取紀錄不內嵌在申請檢視裡，分頁（新到舊）；申請不存在或是草稿回 NotFound（不是空的一頁）；頁大小的規則同其他清單。
func TestKYCAuditListsPageAndCheckTheParent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submittedWithDocuments(t)
	f.secureDocuments(sub)
	for i := range 3 {
		f.now = f.now.Add(time.Minute)
		if _, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: int64(9 + i), ApplicationID: sub.ID, Position: i % 2}); err != nil {
			t.Fatal(err)
		}
	}
	accesses := f.accessPages(t, sub.ID)
	if len(accesses) != 3 || accesses[0].AdminID != 11 || accesses[2].AdminID != 9 {
		t.Fatalf("accesses=%+v", accesses)
	}
	for i := 1; i < len(accesses); i++ {
		if !accesses[i-1].AccessedAt.After(accesses[i].AccessedAt) {
			t.Fatalf("不是新到舊：%+v", accesses)
		}
	}
	// 申請檢視只內嵌審核紀錄（筆數由人工動作限制），沒有讀取紀錄的欄位：型別上就放不進去。
	admin, err := f.module.AdminKYCApplication(ctx, sub.ID)
	must(t, err)
	if len(admin.Reviews) != 1 {
		t.Fatalf("reviews=%+v", admin.Reviews)
	}
	big, err := f.module.ListKYCDocumentAccesses(ctx, KYCAuditListInput{ApplicationID: sub.ID})
	if err != nil || len(big.Items) != 3 || big.Next != nil {
		t.Fatalf("預設頁大小：%+v err=%v", big, err)
	}
	if _, err := f.module.ListKYCDocumentAccesses(ctx, KYCAuditListInput{ApplicationID: sub.ID, PageSize: -1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("負的頁大小：%v", err)
	}
	accountID, _ := f.open(t)
	draft := f.savedDraft(t, kycDraft(accountID))
	for name, id := range map[string]uuid.UUID{"unknown": uuid.NewV7(), "draft": draft.ID} {
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
	_, _, a := f.submitted(t)
	_, _, b := f.submitted(t)
	approved, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: a.ID, ExpectedVersion: a.Version, AdminID: 9, Comment: "齊全"})
	must(t, err)
	if approved.Status != KYCStatusApproved || approved.Version != a.Version+1 || len(approved.Reviews) != 2 || approved.Reviews[1].Action != ReviewApprove ||
		*approved.Reviews[1].ReviewerAdminID != 9 {
		t.Fatalf("approved=%+v", approved)
	}
	rejected, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: b.ID, ExpectedVersion: b.Version, AdminID: 9, Reason: "資料不完整", Issues: []KYCIssue{{Field: "product_type", Code: IssueMismatch, Message: "類別不符"}}})
	must(t, err)
	if rejected.Status != KYCStatusRejected || len(rejected.Reviews) != 2 || rejected.Reviews[1].Action != ReviewReject || rejected.LastRejection == nil {
		t.Fatalf("rejected=%+v", rejected)
	}
}

// 補充文件必須是機敏上傳：media 回報 is_secure=false 的轉正在寫入之前就被拒絕（invalid_argument，欄位違規指向那個文件），
// 什麼都不寫。
func TestKYCDocumentsMustBeSecureAssets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.media.NotSecure = true
	accountID, _ := f.open(t)
	in := companyDraft(accountID, taxIDOne)
	in.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "doc1"}}
	_, err := f.module.SaveKYCDraft(context.Background(), in)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) == 0 || !strings.HasPrefix(ae.FieldViolations()[0].Field, "CompanyDocuments") {
		t.Fatalf("err=%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_applications"); n != 0 {
		t.Fatalf("被拒絕的草稿不能留下申請：%d", n)
	}
	f.media.NotSecure = false
	if _, err := f.module.SaveKYCDraft(context.Background(), in); err != nil {
		t.Fatalf("機敏的文件可以存：%v", err)
	}
}

// media 讀不到補充文件時的三種結果：引用的同步還沒結算（暫時，ErrFileNotSynced）、同步過了卻讀不到（資料不一致，ErrFileMissing）、
// 不是機敏資產（media.ErrNotSecure 原樣）。三種都先寫了讀取紀錄（嘗試也是紀錄）。
func TestKYCDocumentReadFailuresAreDistinguished(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submittedWithDocuments(t)
	read := func() error {
		_, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Position: 0})
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
	}
	seeds := []seed{{"甲", "none"}, {"乙", "approve"}, {"丙", "none"}, {"丁", "reject"}, {"戊", "cancel"}, {"己", "none"}}
	for _, sd := range seeds {
		accountID, s := f.open(t)
		_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET store_name = $1 WHERE store_id = $2", sd.name+"小明", s.StoreID)
		must(t, err)
		in := kycDraft(accountID)
		if sd.name == "丙" {
			in = companyDraft(accountID, taxIDOne)
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
	f.savedDraft(t, kycDraft(draftAccount))
	ids := func(items []KYCApplication) []uuid.UUID {
		out := make([]uuid.UUID, len(items))
		for i, item := range items {
			out[i] = item.ID
		}
		return out
	}
	for _, query := range []string{"", "小明", "丙", taxIDOne, "範例小農", "不存在", "%", "_"} {
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

// 取消的申請沒有可讀的文件：取消時文件已記成退場並解除引用（kyc_documents 的列還在），所以讀文件是 NotFound，不寫讀取紀錄、不問 media。
func TestCancelledApplicationHasNoReadableDocuments(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submittedWithDocuments(t)
	f.secureDocuments(sub)
	_, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: sub.Version, Reason: "改天再申請"})
	must(t, err)
	if f.count(t, "SELECT count(*) FROM seller.kyc_documents WHERE application_id=$1", sub.ID) == 0 {
		t.Fatal("前提：取消之後 kyc_documents 的列還在")
	}
	before := f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses")
	_, err = f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Position: 0})
	if apperr.KindOf(err) != apperr.KindNotFound || f.readSecureCalls() != 0 || f.count(t, "SELECT count(*) FROM seller.kyc_document_accesses") != before {
		t.Fatalf("取消之後：err=%v mediaCalls=%d", err, f.readSecureCalls())
	}
}

// media 說沒有時，用「呼叫 media 之前」讀到的同步狀態分類：呼叫期間才結算的同步，不會被誤判成資料不一致。
func TestKYCDocumentClassificationUsesTheSyncStateFromBeforeTheMediaCall(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submittedWithDocuments(t)
	ref := media.Ref{Type: media.RefTypeCertification, ID: sub.ID.String()}
	f.media.BeforeReadSecure = func() { f.syncMedia(t, ref) } // 同步在 media 呼叫進行中結算
	_, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Position: 0})
	if !errors.Is(err, ErrFileNotSynced) || errors.Is(err, ErrFileMissing) {
		t.Fatalf("呼叫之前還沒結算：%v", err)
	}
	f.media.BeforeReadSecure = nil
	// 已經結算之後，同樣的讀不到才是不一致。
	if _, err := f.module.ReadKYCDocument(ctx, ReadKYCDocumentInput{AdminID: 9, ApplicationID: sub.ID, Position: 0}); !errors.Is(err, ErrFileMissing) {
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
	d := f.savedDraft(t, companyDraft(accountID, taxIDOne))
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
	only := &[]KYCDocumentRef{{StagedToken: "gone"}}
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, CompanyDocuments: only})
	if got := fields(err); !slices.Equal(got, []string{"CompanyDocuments[0]"}) {
		t.Fatalf("單張清單的欄位 = %v（%v）", got, err)
	}
}

// 所有被 media 拒絕的 token 一次回報（各指到請求清單的位置）；轉正失敗的請求不寫入任何文件。
func TestKYCReportsEveryRejectedUploadAtOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	gone := func() error {
		return fmt.Errorf("mediaservice：轉正（code=404）：%w：%w", media.ErrStagedTokenRejected, apperr.InvalidInput("圖片無法使用，請重新上傳"))
	}
	f.media.rejected = map[string]error{"gone1": gone(), "gone2": gone()}
	d := f.savedDraft(t, companyDraft(accountID, taxIDOne))
	company := &[]KYCDocumentRef{{StagedToken: "gone1"}, {StagedToken: "ok"}, {StagedToken: "gone2"}}
	_, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: d.Version, CompanyDocuments: company})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("err = %v", err)
	}
	var got []string
	for _, v := range ae.FieldViolations() {
		got = append(got, v.Field)
	}
	if want := []string{"CompanyDocuments[0]", "CompanyDocuments[2]"}; !slices.Equal(got, want) {
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
	d := f.savedDraft(t, companyDraft(accountID, taxIDOne))
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

// editStore 以店家目前的版本寫入店家資料。
func editStore(f *fixture, t *testing.T, storeID int64, in UpdateProfileInput) (Store, error) {
	t.Helper()
	cur, err := f.module.Store(context.Background(), storeID)
	must(t, err)
	in.StoreID, in.ExpectedVersion = storeID, cur.Version
	return f.module.UpdateProfile(context.Background(), in)
}

func hasField(e *apperr.Error, field string) bool {
	for _, v := range e.FieldViolations() {
		if v.Field == field {
			return true
		}
	}
	return false
}

// 回歸：店家目前的申請在審核中或已核准時，地址與客服信箱不能清掉；草稿、已退回與沒有申請時可以清；聯絡人姓名與手機任何時候都能清掉。
func TestRequiredStoreFieldsCannotBeClearedWhileTheApplicationIsSubmittedOrApproved(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	restore := UpdateProfileInput{SupportEmail: ptr("owner@example.com"), TownCode: ptr("63000050"), AddressDetail: ptr("測試路一號")}
	clearRequired := func(label string) {
		t.Helper()
		for name, tc := range map[string]struct {
			in    UpdateProfileInput
			field string
		}{
			"客服信箱": {UpdateProfileInput{SupportEmail: ptr("")}, "SupportEmail"},
			"整個地址": {UpdateProfileInput{TownCode: ptr(""), AddressDetail: ptr("")}, "TownCode"},
			"地址明細": {UpdateProfileInput{AddressDetail: ptr("")}, "AddressDetail"},
		} {
			_, err := editStore(f, t, s.StoreID, tc.in)
			var ae *apperr.Error
			if apperr.KindOf(err) != apperr.KindInvalidInput || !errors.As(err, &ae) || !hasField(ae, tc.field) {
				t.Errorf("%s %s：err=%v", label, name, err)
			}
		}
		cur, err := f.module.Store(ctx, s.StoreID)
		must(t, err)
		if deref(cur.SupportEmail) != "owner@example.com" || cur.AddressDetail == nil {
			t.Fatalf("%s：被拒絕的修改不該寫入：%+v", label, cur)
		}
	}
	clearOptional := func(label string) {
		t.Helper()
		if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr(""), ContactMobile: ptr(" ")}); err != nil {
			t.Fatalf("%s：聯絡人姓名與手機應該可以清：%v", label, err)
		}
		if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr("王小明"), ContactMobile: ptr("0912345678")}); err != nil {
			t.Fatal(err)
		}
	}
	// 沒有申請與草稿：地址與信箱可以清。
	clearOptional("沒有申請")
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{SupportEmail: ptr(""), TownCode: ptr(""), AddressDetail: ptr("")}); err != nil {
		t.Fatalf("沒有申請時應該可以清：%v", err)
	}
	if _, err := editStore(f, t, s.StoreID, restore); err != nil {
		t.Fatal(err)
	}
	d := f.savedDraft(t, kycDraft(accountID))
	clearOptional("草稿")
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	clearRequired("審核中")
	clearOptional("審核中")
	must(t, approve(f, sub))
	clearRequired("已核准")
	clearOptional("已核准")
	// 換成別的值是允許的，商店電話不是必填、可以清。
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{SupportEmail: ptr("new@example.com"), ContactPhone: ptr("")}); err != nil {
		t.Fatal(err)
	}
}

// 回歸：DB 的 CHECK 接受 D39 的新上限（姓名 50 字、信箱 254），並拒絕超過的值；已退回的申請可以清掉地址與客服信箱；審核中可以清聯絡人姓名與手機。
func TestStoreContactLimitsAndRejectedApplicationAreEnforcedByTheDatabase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	emailOfLen := func(n int) string {
		label := strings.Repeat("b", 60)
		e := strings.Repeat("a", 64) + "@" + label + "." + label + "." + label + "." + strings.Repeat("c", n-(64+1+3*61))
		if len(e) != n {
			t.Fatalf("test email length = %d, want %d", len(e), n)
		}
		return e
	}
	name50, name51 := strings.Repeat("王", 50), strings.Repeat("王", 51)
	email254, email255 := emailOfLen(254), emailOfLen(255)
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr(name50), SupportEmail: ptr(email254)}); err != nil {
		t.Fatalf("50 字姓名與 254 長度信箱應該可以寫入：%v", err)
	}
	cur, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if deref(cur.ContactName) != name50 || deref(cur.SupportEmail) != email254 {
		t.Fatalf("寫入後讀不回來：%+v", cur)
	}
	for name, in := range map[string]UpdateProfileInput{
		"51 字姓名":   {ContactName: ptr(name51)},
		"255 長度信箱": {SupportEmail: ptr(email255)},
	} {
		if _, err := editStore(f, t, s.StoreID, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s 應該被拒絕：%v", name, err)
		}
	}
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{TownCode: ptr("63000050"), AddressDetail: ptr("測試路一號")}); err != nil {
		t.Fatal(err)
	}
	d := f.savedDraft(t, kycDraft(accountID))
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{ContactName: ptr(""), ContactMobile: ptr("")}); err != nil {
		t.Fatalf("審核中應該可以清聯絡人姓名與手機：%v", err)
	}
	if _, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "請確認"}); err != nil {
		t.Fatal(err)
	}
	if _, err := editStore(f, t, s.StoreID, UpdateProfileInput{SupportEmail: ptr(""), TownCode: ptr(""), AddressDetail: ptr("")}); err != nil {
		t.Fatalf("已退回時應該可以清地址與客服信箱：%v", err)
	}
	cur, err = f.module.Store(ctx, s.StoreID)
	must(t, err)
	if cur.SupportEmail != nil || cur.AddressDetail != nil {
		t.Fatalf("清除後仍有值：%+v", cur)
	}
}

// 儲存草稿與送出只看申請的狀態：草稿與已退回的可以；審核中只能取消；已核准的不能編輯也不能重送，而且被擋下時沒有寫入。
// mutation：model.CanEditKYC 把 APPROVED 算進可編輯 → 已核准的申請被改成草稿，這個測試失敗。
func TestSavingAndSubmittingDependOnlyOnTheApplicationStatus(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, sub := f.submitted(t)
	save := func(a KYCApplication) error {
		_, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: a.Version, ProductType: ptr("水果")})
		return err
	}
	submit := func(a KYCApplication) error {
		_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: a.Version})
		return err
	}
	statusAndVersion := func() (string, int64) {
		var status string
		var version int64
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT status, version FROM seller.kyc_applications WHERE store_id=$1 AND status <> 'CANCELLED'", s.StoreID).Scan(&status, &version))
		return status, version
	}
	blocked := func(label string, err error) {
		t.Helper()
		if apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("%s：要 failed_precondition，得到 %v", label, err)
		}
	}
	// 審核中：兩個都擋下，狀態與版本不變。
	blocked("審核中儲存", save(sub))
	blocked("審核中送出", submit(sub))
	if status, version := statusAndVersion(); status != "SUBMITTED" || version != sub.Version {
		t.Fatalf("審核中被擋下之後：%s v%d，要 SUBMITTED v%d", status, version, sub.Version)
	}
	// 已核准：兩個都擋下，狀態與版本不變。
	must(t, approve(f, sub))
	_, approvedVersion := statusAndVersion()
	approved, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	blocked("已核准儲存", save(approved))
	blocked("已核准送出", submit(approved))
	if status, version := statusAndVersion(); status != "APPROVED" || version != approvedVersion {
		t.Fatalf("已核准被擋下之後：%s v%d，要 APPROVED v%d", status, version, approvedVersion)
	}
	// 已退回：可以編輯、可以直接重送。
	otherAccount, _, other := f.submitted(t)
	_, err = f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: other.ID, ExpectedVersion: other.Version, AdminID: 9, Issues: anIssue(), Reason: "請確認"})
	must(t, err)
	rejected, err := f.module.MyKYC(ctx, otherAccount)
	must(t, err)
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: otherAccount, ExpectedVersion: rejected.Version, ProductType: ptr("水果")})
	must(t, err)
	if edited.Status != KYCStatusDraft {
		t.Fatalf("已退回編輯之後 status=%s", edited.Status)
	}
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: otherAccount, ExpectedVersion: edited.Version}); err != nil {
		t.Fatalf("草稿重送：%v", err)
	}
}

// 店名與介紹沒有鎖：審核中與核准之後都能改。
func TestStoreNameAndDescriptionStayEditableWhileSubmittedAndAfterApproval(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s, sub := f.submitted(t)
	if got, err := editStore(f, t, s.StoreID, UpdateProfileInput{StoreName: ptr("審核中的新店名"), Description: ptr("審核中的介紹")}); err != nil || got.StoreName != "審核中的新店名" {
		t.Fatalf("審核中改店名：%+v err=%v", got, err)
	}
	must(t, approve(f, sub))
	got, err := editStore(f, t, s.StoreID, UpdateProfileInput{StoreName: ptr("核准後的新店名"), Description: ptr("核准後的介紹")})
	if err != nil || got.StoreName != "核准後的新店名" || deref(got.Description) != "核准後的介紹" {
		t.Fatalf("核准後改店名：%+v err=%v", got, err)
	}
	if cur, err := f.module.Store(ctx, s.StoreID); err != nil || cur.StoreName != "核准後的新店名" {
		t.Fatalf("讀回：%+v err=%v", cur, err)
	}
}
