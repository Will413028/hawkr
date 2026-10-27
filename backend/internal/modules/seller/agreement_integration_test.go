//go:build integration

package seller_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

type acceptanceRow struct {
	ID                 int64
	Kind, Version      string
	AccountID          int64
	AcceptedAt         time.Time
	SubmissionReviewID *string
	ClientIP           *string
	UserAgent          *string
	PurgedAt           *time.Time
}

func (f *fixture) acceptances(t *testing.T, storeID int64) []acceptanceRow {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT acceptance_id, kind, version, account_id, accepted_at, submission_review_id::text, host(client_ip), user_agent, pii_purged_at
		FROM seller.agreement_acceptances WHERE store_id = $1 ORDER BY acceptance_id`, storeID)
	must(t, err)
	defer rows.Close()
	var out []acceptanceRow
	for rows.Next() {
		var r acceptanceRow
		must(t, rows.Scan(&r.ID, &r.Kind, &r.Version, &r.AccountID, &r.AcceptedAt, &r.SubmissionReviewID, &r.ClientIP, &r.UserAgent, &r.PurgedAt))
		out = append(out, r)
	}
	must(t, rows.Err())
	return out
}

func (f *fixture) submitReviewIDs(t *testing.T, storeID int64) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT r.review_id::text FROM seller.kyc_reviews r JOIN seller.kyc_applications a USING (application_id)
		WHERE a.store_id = $1 AND r.action = 'SUBMIT' ORDER BY r.decided_at, r.review_id`, storeID)
	must(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		must(t, rows.Scan(&id))
		out = append(out, id)
	}
	must(t, rows.Err())
	return out
}

func (f *fixture) applyWith(accountID int64, agreements []AgreementAcceptance, ip, ua string) (Store, error) {
	in := validApply(accountID)
	in.AcceptedAgreements, in.ClientIP, in.UserAgent = agreements, ip, ua
	return f.module.Apply(context.Background(), in)
}

func apperrOf(t *testing.T, err error) *apperr.Error {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("不是 apperr：%v", err)
	}
	return ae
}

func equalText(got *string, want string) bool { return got != nil && *got == want }

// 開店記下同意：哪家店、哪份文件的哪個版本、誰、何時（模組的時鐘）、來源 IP 與 User-Agent。
func TestApplyRecordsTheAcceptanceWithItsSource(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID := f.account(t)
	s, err := f.applyWith(accountID, CurrentApplyAcceptances(), "203.0.113.9", "hk-test/1.0")
	must(t, err)
	got := f.acceptances(t, s.StoreID)
	want := CurrentAgreementsOrFail(t, AgreementSellerTerms)
	if len(got) != 1 {
		t.Fatalf("rows=%+v", got)
	}
	r := got[0]
	if r.Kind != "SELLER_TERMS" || r.Version != want.Version || r.AccountID != accountID || !r.AcceptedAt.Equal(f.now) ||
		!equalText(r.ClientIP, "203.0.113.9") || !equalText(r.UserAgent, "hk-test/1.0") {
		t.Fatalf("row=%+v", r)
	}
}

func CurrentAgreementsOrFail(t *testing.T, k AgreementKind) Agreement {
	t.Helper()
	a, ok := CurrentAgreement(k)
	if !ok {
		t.Fatalf("%s 沒有現行版本", k)
	}
	return a
}

// 取不到的來源是 NULL，不是空字串，也不是無法解析的文字。
func TestApplyStoresMissingSourceAsNull(t *testing.T) {
	t.Parallel()
	f := setup(t)
	for name, c := range map[string]struct{ ip, ua string }{
		"都沒有":      {"", ""},
		"IP 不是位址":  {"unknown", "   "},
		"帶埠號的不是位址": {"203.0.113.9:443", ""},
		"逗號連接的代理鏈": {"203.0.113.9, 10.0.0.1", ""},
	} {
		s, err := f.applyWith(f.account(t), CurrentApplyAcceptances(), c.ip, c.ua)
		must(t, err)
		got := f.acceptances(t, s.StoreID)
		if len(got) != 1 || got[0].ClientIP != nil || got[0].UserAgent != nil {
			t.Errorf("%s：rows=%+v", name, got)
		}
	}
	s, err := f.applyWith(f.account(t), CurrentApplyAcceptances(), "::ffff:203.0.113.9", strings.Repeat("a", 700))
	must(t, err)
	got := f.acceptances(t, s.StoreID)
	if len(got) != 1 || !equalText(got[0].ClientIP, "203.0.113.9") || got[0].UserAgent == nil || len(*got[0].UserAgent) != 512 {
		t.Fatalf("標準化：rows=%+v", got)
	}
}

// 版本落後：failed_precondition（AGREEMENT_OUTDATED），沒有建店、沒有 SELLER 角色、沒有同意紀錄。
func TestApplyWithAnOutdatedVersionCreatesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID := f.account(t)
	_, err := f.applyWith(accountID, []AgreementAcceptance{{Kind: AgreementSellerTerms, Version: "2026-01-01"}}, "203.0.113.9", "ua")
	ae := apperrOf(t, err)
	if ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonAgreementOutdated {
		t.Fatalf("err=%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.stores WHERE account_id = $1", accountID); n != 0 {
		t.Fatalf("建了 %d 家店", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.agreement_acceptances"); n != 0 {
		t.Fatalf("寫了 %d 筆同意紀錄", n)
	}
	a, err := f.identity.Account(context.Background(), accountID)
	must(t, err)
	if len(a.Roles) != 0 {
		t.Fatalf("roles=%v", a.Roles)
	}
}

// 種類錯誤：invalid_argument，欄位是 AcceptedAgreements，不建店。
func TestApplyWithWrongAgreementKindsIsInvalidInput(t *testing.T) {
	t.Parallel()
	f := setup(t)
	current := func(k AgreementKind) AgreementAcceptance {
		return AgreementAcceptance{Kind: k, Version: CurrentAgreementsOrFail(t, k).Version}
	}
	for name, in := range map[string][]AgreementAcceptance{
		"沒有":          nil,
		"KYC 的種類":     CurrentKYCAcceptances(),
		"多出一種":        append(CurrentApplyAcceptances(), current(AgreementPayUniTerms)),
		"同一種兩次":       {current(AgreementSellerTerms), current(AgreementSellerTerms)},
		"UNSPECIFIED": {{Kind: "", Version: CurrentAgreementsOrFail(t, AgreementSellerTerms).Version}},
	} {
		accountID := f.account(t)
		_, err := f.applyWith(accountID, in, "", "")
		ae := apperrOf(t, err)
		if ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "AcceptedAgreements" {
			t.Errorf("%s：err=%v", name, err)
		}
		if n := f.count(t, "SELECT count(*) FROM seller.stores WHERE account_id = $1", accountID); n != 0 {
			t.Errorf("%s：建了 %d 家店", name, n)
		}
	}
}

// 同意紀錄與開店在同一個 transaction：開店在寫完紀錄之後失敗（帳號等待刪除時授予角色被拒），店與紀錄一起回滾。
func TestApplyRollsTheAcceptanceBackWithTheStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID := f.account(t)
	must(t, f.identity.BeginDeletion(context.Background(), accountID))
	_, err := f.applyWith(accountID, CurrentApplyAcceptances(), "203.0.113.9", "ua")
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("err=%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.agreement_acceptances"); n != 0 {
		t.Fatalf("回滾之後還有 %d 筆同意紀錄", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.stores WHERE account_id = $1", accountID); n != 0 {
		t.Fatalf("回滾之後還有 %d 家店", n)
	}
}

func (f *fixture) kycSubmit(accountID, version int64, agreements []AgreementAcceptance, ip, ua string) (KYCApplication, error) {
	return f.module.SubmitKYC(context.Background(), SubmitKYCInput{
		AccountID: accountID, ExpectedVersion: version, AcceptedAgreements: agreements, ClientIP: ip, UserAgent: ua,
	})
}

// 送出 KYC 記下兩份同意，與送出同一個 transaction。
func TestSubmitKYCRecordsBothAcceptances(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	before := f.acceptances(t, s.StoreID)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	f.now = f.now.Add(time.Hour)
	_, err := f.kycSubmit(accountID, d.Version, CurrentKYCAcceptances(), "2001:db8::7", "hk-app/2.0")
	must(t, err)
	got := f.acceptances(t, s.StoreID)
	if len(before) != 1 || len(got) != 3 {
		t.Fatalf("開店後 %d 筆，送出後 %d 筆", len(before), len(got))
	}
	byKind := map[string]acceptanceRow{}
	for _, r := range got {
		byKind[r.Kind] = r
	}
	for _, k := range []AgreementKind{AgreementKYCPrivacyNotice, AgreementPayUniTerms} {
		r, ok := byKind[string(k)]
		if !ok || r.Version != CurrentAgreementsOrFail(t, k).Version || r.AccountID != accountID || !r.AcceptedAt.Equal(f.now) ||
			!equalText(r.ClientIP, "2001:db8::7") || !equalText(r.UserAgent, "hk-app/2.0") {
			t.Errorf("%s：row=%+v", k, r)
		}
	}
	if r := byKind["SELLER_TERMS"]; !r.AcceptedAt.Before(f.now) || r.SubmissionReviewID != nil {
		t.Errorf("開店當時的紀錄被改動或帶了送出：%+v", r)
	}
	reviews := f.submitReviewIDs(t, s.StoreID)
	for _, k := range []string{"KYC_PRIVACY_NOTICE", "PAYUNI_TERMS"} {
		if r := byKind[k]; len(reviews) != 1 || r.SubmissionReviewID == nil || *r.SubmissionReviewID != reviews[0] {
			t.Errorf("%s 沒有綁定這次送出的審核紀錄 %v：%+v", k, reviews, r)
		}
	}
}

// 版本落後與種類錯誤都不送出：申請維持草稿、版本不變，沒有新的同意紀錄。
func TestSubmitKYCWithBadAgreementsDoesNotSubmit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	outdated := []AgreementAcceptance{{Kind: AgreementKYCPrivacyNotice, Version: "2026-01-01"}, {Kind: AgreementPayUniTerms, Version: CurrentAgreementsOrFail(t, AgreementPayUniTerms).Version}}
	_, err := f.kycSubmit(accountID, d.Version, outdated, "", "")
	if ae := apperrOf(t, err); ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonAgreementOutdated {
		t.Fatalf("outdated err=%v", err)
	}
	for name, in := range map[string][]AgreementAcceptance{
		"沒有":      nil,
		"缺一種":     CurrentKYCAcceptances()[:1],
		"開店的種類":   CurrentApplyAcceptances(),
		"同一種兩次":   {CurrentKYCAcceptances()[0], CurrentKYCAcceptances()[0]},
		"多出開店的種類": append(CurrentKYCAcceptances(), CurrentApplyAcceptances()...),
	} {
		_, err := f.kycSubmit(accountID, d.Version, in, "", "")
		if ae := apperrOf(t, err); ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "AcceptedAgreements" {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	mine, err := f.module.MyKYC(context.Background(), accountID)
	must(t, err)
	if mine.Status != KYCStatusDraft || mine.Version != d.Version {
		t.Fatalf("申請被送出：%+v", mine)
	}
	if got := f.acceptances(t, s.StoreID); len(got) != 1 {
		t.Fatalf("rows=%+v", got)
	}
}

// 被退回後重送：每次送出各寫兩列新的紀錄，各自綁定那一次的送出（換了申請主體也留下新的同意）；第一次的紀錄不動。
func TestResubmittingKYCAddsNewAcceptancesBoundToTheNewSubmission(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s, sub := f.submitted(t, idA, bankAcct)
	first := f.acceptances(t, s.StoreID)
	if len(first) != 3 {
		t.Fatalf("rows=%+v", first)
	}
	_, err := f.module.RejectKYC(context.Background(), RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "照片不清楚"})
	must(t, err)
	rejected, err := f.module.MyKYC(context.Background(), accountID)
	must(t, err)
	f.now = f.now.Add(48 * time.Hour)
	_, err = f.kycSubmit(accountID, rejected.Version, CurrentKYCAcceptances(), "203.0.113.50", "other-ua")
	must(t, err)
	second := f.acceptances(t, s.StoreID)
	if len(second) != 5 {
		t.Fatalf("重送之後有 %d 筆，預期 5 筆", len(second))
	}
	for i := range first {
		if !reflect.DeepEqual(first[i], second[i]) {
			t.Errorf("第一次的紀錄被改動：%+v → %+v", first[i], second[i])
		}
	}
	reviews := f.submitReviewIDs(t, s.StoreID)
	if len(reviews) != 2 {
		t.Fatalf("送出紀錄=%v", reviews)
	}
	for i, r := range second[3:] {
		if r.SubmissionReviewID == nil || *r.SubmissionReviewID != reviews[1] || !equalText(r.ClientIP, "203.0.113.50") || !equalText(r.UserAgent, "other-ua") {
			t.Errorf("新的紀錄[%d]=%+v，預期綁定 %s", i, r, reviews[1])
		}
	}
	for _, r := range first[1:] {
		if r.SubmissionReviewID == nil || *r.SubmissionReviewID != reviews[0] {
			t.Errorf("第一次的 KYC 紀錄 %+v 沒有綁定第一次的送出 %s", r, reviews[0])
		}
	}
}

// 資料表自己把關：開店的同意每家店一列，KYC 的同意一定帶送出，開店的同意一定不帶，kind 與版本與 UA 的限制。
func TestAgreementAcceptanceTableConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s, _ := f.submitted(t, idA, bankAcct)
	ctx := context.Background()
	reviews := f.submitReviewIDs(t, s.StoreID)
	insert := func(kind, version string, review *string, ua *string) error {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.agreement_acceptances (store_id, kind, version, account_id, accepted_at, submission_review_id, user_agent)
			VALUES ($1, $2, $3, $4, now(), $5::uuid, $6)`, s.StoreID, kind, version, accountID, review, ua)
		return err
	}
	empty := ""
	for name, err := range map[string]error{
		"開店的同意重複":      insert("SELLER_TERMS", "2025-01-01", nil, nil),
		"開店的同意帶了送出":    insert("SELLER_TERMS", "2025-01-01", &reviews[0], nil),
		"KYC 的同意沒有帶送出": insert("PAYUNI_TERMS", "2025-01-01", nil, nil),
		"不認得的 kind":    insert("NO_SUCH", "v", &reviews[0], nil),
		"空版本":          insert("PAYUNI_TERMS", "", &reviews[0], nil),
		"空的 UA":        insert("PAYUNI_TERMS", "v", &reviews[0], &empty),
	} {
		if err == nil {
			t.Errorf("%s：CHECK 或索引沒有擋下", name)
		}
	}
	if err := insert("PAYUNI_TERMS", "2025-01-01", &reviews[0], nil); err != nil {
		t.Fatalf("同一次送出可以有不同版本的列：%v", err)
	}
}

// 關店滿保留期清理：來源 IP 與 User-Agent 清成 NULL 並記下 pii_purged_at，種類、版本、時間、帳號與送出都還在。
func TestPurgeClearsAgreementSourcesAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID := f.account(t)
	s, err := f.applyWith(accountID, CurrentApplyAcceptances(), "203.0.113.9", "hk-test/1.0")
	must(t, err)
	f.setContact(t, s.StoreID)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	_, err = f.kycSubmit(accountID, d.Version, CurrentKYCAcceptances(), "2001:db8::7", "hk-app/2.0")
	must(t, err)
	before := f.acceptances(t, s.StoreID)
	if len(before) != 3 {
		t.Fatalf("rows=%+v", before)
	}
	for _, r := range before {
		if r.ClientIP == nil || r.UserAgent == nil || r.PurgedAt != nil {
			t.Fatalf("清理之前來源欄位應該有值：%+v", r)
		}
	}
	_, err = f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 {
		t.Fatalf("purged=%d", n)
	}
	after := f.acceptances(t, s.StoreID)
	if len(after) != len(before) {
		t.Fatalf("清理刪了紀錄：%d → %d", len(before), len(after))
	}
	for i, r := range after {
		b := before[i]
		if r.ClientIP != nil || r.UserAgent != nil || r.PurgedAt == nil {
			t.Errorf("來源欄位沒清：%+v", r)
		}
		if r.ID != b.ID || r.Kind != b.Kind || r.Version != b.Version || r.AccountID != b.AccountID || !r.AcceptedAt.Equal(b.AcceptedAt) ||
			(r.SubmissionReviewID == nil) != (b.SubmissionReviewID == nil) || (r.SubmissionReviewID != nil && *r.SubmissionReviewID != *b.SubmissionReviewID) {
			t.Errorf("清理改動了要保留的欄位：%+v → %+v", b, r)
		}
	}
}

// 同意紀錄與送出在同一個 transaction：紀錄寫完之後，同一個 tx 內發布 KYCSubmitted 的訂閱者失敗，紀錄與送出一起回滾。
func TestSubmitKYCRollsTheAcceptanceBackWithTheSubmission(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	before := f.acceptances(t, s.StoreID)
	boom := errors.New("subscriber failed")
	events.Subscribe(f.bus, func(context.Context, KYCSubmitted) error { return boom })
	_, err := f.kycSubmit(accountID, d.Version, CurrentKYCAcceptances(), "203.0.113.9", "ua")
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if got := f.acceptances(t, s.StoreID); len(got) != len(before) {
		t.Fatalf("回滾之後多了同意紀錄：%+v", got)
	}
	mine, err := f.module.MyKYC(context.Background(), accountID)
	must(t, err)
	if mine.Status != KYCStatusDraft || mine.Version != d.Version {
		t.Fatalf("申請被送出：%+v", mine)
	}
}

// User-Agent 含非 UTF-8 位元組時照樣開店，紀錄裡是清理後的值。
func TestApplyWithInvalidUTF8UserAgentSucceeds(t *testing.T) {
	t.Parallel()
	f := setup(t)
	s, err := f.applyWith(f.account(t), CurrentApplyAcceptances(), "203.0.113.9", "hk\xff/1.0")
	must(t, err)
	if got := f.acceptances(t, s.StoreID); len(got) != 1 || !equalText(got[0].UserAgent, "hk/1.0") {
		t.Fatalf("rows=%+v", got)
	}
}

// 版本是空字串：invalid_argument，不是版本過時。
func TestApplyWithAnEmptyVersionIsInvalidInput(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, err := f.applyWith(f.account(t), []AgreementAcceptance{{Kind: AgreementSellerTerms}}, "", "")
	if ae := apperrOf(t, err); ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "AcceptedAgreements" {
		t.Fatalf("err=%v", err)
	}
}

// agreement_acceptances_source_check：每種 kind 只綁定剛好一種來源。開店的同意（SELLER_TERMS）不帶送出，KYC 的兩份同意
// （KYC_PRIVACY_NOTICE、PAYUNI_TERMS）一定帶送出。被拒的語句以 wantViolates 斷言是這一條擋下，而且拿掉它就成功（沒有被別的約束遮住）。
// 開店時已寫下的那列 SELLER_TERMS 先刪掉，否則每店一列的唯一索引會遮住「開店的同意帶了送出」。
// mutation：在測試資料庫 ALTER TABLE seller.agreement_acceptances DROP CONSTRAINT agreement_acceptances_source_check，
// 三個被拒的情況都改成寫入成功，本測試失敗。
func TestAgreementAcceptancesBindExactlyOneSourceByKind(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s, _ := f.submitted(t, idA, bankAcct)
	ctx := context.Background()
	review := f.submitReviewIDs(t, s.StoreID)[0]
	_, err := f.db.Conn(ctx).Exec(ctx, "DELETE FROM seller.agreement_acceptances WHERE store_id = $1 AND kind = 'SELLER_TERMS'", s.StoreID)
	must(t, err)
	const insert = `INSERT INTO seller.agreement_acceptances (store_id, kind, version, account_id, accepted_at, submission_review_id)
		VALUES ($1, $2, 'v', $3, now(), $4::uuid)`
	for _, c := range []struct {
		name, kind string
		review     *string
	}{
		{"開店的同意帶了送出", "SELLER_TERMS", &review},
		{"KYC 告知沒有送出", "KYC_PRIVACY_NOTICE", nil},
		{"PayUni 條款沒有送出", "PAYUNI_TERMS", nil},
	} {
		f.wantViolates(t, c.name, "agreement_acceptances_source_check", insert, s.StoreID, c.kind, accountID, c.review)
	}
	for _, c := range []struct {
		kind   string
		review *string
	}{
		{"SELLER_TERMS", nil},
		{"KYC_PRIVACY_NOTICE", &review},
		{"PAYUNI_TERMS", &review},
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, insert, s.StoreID, c.kind, accountID, c.review); err != nil {
			t.Errorf("%s 綁正確來源卻寫不進去：%v", c.kind, err)
		}
	}
}
