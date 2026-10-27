//go:build integration

package seller_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// anIssue 是測試用的一個有效問題（個人與企業的申請都有 real_name）。
func anIssue() []KYCIssue {
	return []KYCIssue{{Field: "real_name", Code: IssueOther, Message: "請確認姓名"}}
}

func issueOn(field string, code KYCIssueCode) KYCIssue {
	return KYCIssue{Field: field, Code: code, Message: "請處理 " + field}
}

func documentIssue(field, assetID string, code KYCIssueCode) KYCIssue {
	i := issueOn(field, code)
	i.DocumentAssetID = assetID
	return i
}

func fields(issues []KYCIssue) []string {
	out := make([]string, len(issues))
	for i, issue := range issues {
		out[i] = issue.Field
		if issue.DocumentAssetID != "" {
			out[i] += "@" + issue.DocumentAssetID
		}
	}
	return out
}

func (f *fixture) outstanding(t *testing.T, accountID int64) []string {
	t.Helper()
	mine, err := f.module.MyKYC(context.Background(), accountID)
	must(t, err)
	return fields(mine.OutstandingIssues)
}

func (f *fixture) reject(t *testing.T, a KYCApplication, issues ...KYCIssue) KYCAdminApplication {
	t.Helper()
	rej, err := f.module.RejectKYC(context.Background(), RejectKYCInput{ApplicationID: a.ID, ExpectedVersion: a.Version, AdminID: 9, Reason: "請補件", Issues: issues})
	must(t, err)
	return rej
}

// 退回的驗證：至少一項、欄位與原因碼在值集合內、說明 1 到 500 字、非文件類的欄位不帶 asset、指到的文件是這份申請在這個欄位
// 目前的文件、欄位適用於申請的賣家型態；違反時整筆不寫入，欄位違規指到 Issues[i] 的欄位。
func TestRejectKYCValidatesIssuesAgainstTheApplication(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, sub := f.submitted(t, idA, bankAcct)
	long := strings.Repeat("字", 501)
	for name, c := range map[string]struct {
		issues []KYCIssue
		field  string
	}{
		"no issues":                    {nil, "Issues"},
		"unspecified field":            {[]KYCIssue{{Code: IssueOther, Message: "x"}}, "Issues[0].Field"},
		"unknown field":                {[]KYCIssue{{Field: "favourite_colour", Code: IssueOther, Message: "x"}}, "Issues[0].Field"},
		"unspecified code":             {[]KYCIssue{{Field: "real_name", Message: "x"}}, "Issues[0].Code"},
		"unknown code":                 {[]KYCIssue{{Field: "real_name", Code: "FINAL", Message: "x"}}, "Issues[0].Code"},
		"empty message":                {[]KYCIssue{{Field: "real_name", Code: IssueOther}}, "Issues[0].Message"},
		"blank message":                {[]KYCIssue{{Field: "real_name", Code: IssueOther, Message: " \n "}}, "Issues[0].Message"},
		"overlong message":             {[]KYCIssue{{Field: "real_name", Code: IssueOther, Message: long}}, "Issues[0].Message"},
		"asset on a non document":      {[]KYCIssue{{Field: "real_name", DocumentAssetID: "asset-front", Code: IssueOther, Message: "x"}}, "Issues[0].DocumentAssetID"},
		"asset not on the application": {[]KYCIssue{documentIssue("id_card_front", "asset-nope", IssueUnreadable)}, "Issues[0].DocumentAssetID"},
		"asset in another slot":        {[]KYCIssue{issueOn("real_name", IssueMismatch), documentIssue("bank_book", "asset-front", IssueUnreadable)}, "Issues[1].DocumentAssetID"},
		"business field on individual": {[]KYCIssue{issueOn("business.tax_id", IssueMismatch)}, "Issues[0].Field"},
		// 文件類欄位依原因碼：看不清楚、過期、不一致、格式不符必須指定是哪一張；缺少不可以指定。
		"unreadable without an asset": {[]KYCIssue{issueOn("id_card_front", IssueUnreadable)}, "Issues[0].DocumentAssetID"},
		"expired without an asset":    {[]KYCIssue{issueOn("id_card_back", IssueExpired)}, "Issues[0].DocumentAssetID"},
		"mismatch without an asset":   {[]KYCIssue{issueOn("bank_book", IssueMismatch)}, "Issues[0].DocumentAssetID"},
		"invalid without an asset":    {[]KYCIssue{issueOn("id_card_front", IssueInvalid)}, "Issues[0].DocumentAssetID"},
		"missing with an asset":       {[]KYCIssue{documentIssue("id_card_front", "asset-front", IssueMissing)}, "Issues[0].DocumentAssetID"},
	} {
		_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Reason: "x", Issues: c.issues})
		ae := apperrOf(t, err)
		if ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != c.field {
			t.Errorf("%s：err=%v violations=%+v，預期指到 %s", name, err, ae.FieldViolations(), c.field)
		}
	}
	// 內部備註的上限同理由（1000 字）。
	_, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Reason: "x", Issues: anIssue(), InternalNote: strings.Repeat("註", 1001)})
	if ae := apperrOf(t, err); ae.Kind() != apperr.KindInvalidInput || ae.FieldViolations()[0].Field != "InternalNote" {
		t.Errorf("overlong internal note: %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_review_issues"); n != 0 {
		t.Fatalf("被拒絕的退回寫入了 %d 項問題", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action = 'REJECT'"); n != 0 {
		t.Fatalf("被拒絕的退回寫入了審核紀錄：%d", n)
	}
	// 合法的問題：說明去掉前後空白；文件類的欄位可以不指定特定一張。
	rej := f.reject(t, sub,
		documentIssue("id_card_front", "asset-front", IssueUnreadable),
		KYCIssue{Field: "bank_book", Code: IssueMissing, Message: "  請重新上傳存摺  "},
		documentIssue("id_card_back", "asset-back", IssueOther),
		KYCIssue{Field: "real_name", Code: IssueUnreadable, Message: "非文件欄位的看不清楚不需要 asset"},
	)
	last := rej.Reviews[len(rej.Reviews)-1]
	if last.Action != ReviewReject || len(last.Issues) != 4 || last.Issues[1].Message != "請重新上傳存摺" || last.Issues[1].DocumentAssetID != "" {
		t.Fatalf("review=%+v", last)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.kyc_review_issues WHERE resolved_at IS NULL"); got != 4 {
		t.Fatalf("未處理的問題=%d", got)
	}
}

// 賣家只讀得到自己該看的：總理由與逐項問題；內部備註只在後台的審核紀錄，賣家的任何檢視型別都沒有它。
func TestInternalNoteIsOnlyOnTheAdminReviewRecord(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submitted(t, idA, bankAcct)
	const note = "疑似翻拍，下次先打電話確認"
	rej, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Reason: "存摺不清楚", Issues: anIssue(), InternalNote: "  " + note + " "})
	must(t, err)
	if got := rej.Reviews[len(rej.Reviews)-1].InternalNote; got == nil || *got != note {
		t.Fatalf("admin review note=%v", got)
	}
	adminView, err := f.module.AdminKYCApplication(ctx, sub.ID)
	must(t, err)
	if got := adminView.Reviews[len(adminView.Reviews)-1].InternalNote; got == nil || *got != note {
		t.Fatalf("admin read note=%v", got)
	}
	for _, r := range adminView.Reviews[:len(adminView.Reviews)-1] {
		if r.InternalNote != nil {
			t.Fatalf("非退回的紀錄不該有備註：%+v", r)
		}
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	// 序列化整份賣家的檢視（含巢狀的指標）：%+v 印不出指標指到的內容，會漏掉。
	raw, err := json.Marshal(mine)
	must(t, err)
	if strings.Contains(string(raw), note) || strings.Contains(strings.ToLower(string(raw)), "internalnote") {
		t.Fatalf("賣家的檢視含內部備註：%s", raw)
	}
	for _, view := range []KYCApplication{rej.KYCApplication, adminView.KYCApplication} {
		if raw, err := json.Marshal(view); err != nil || strings.Contains(string(raw), note) {
			t.Fatalf("申請本體含內部備註（備註只在審核紀錄）：%s err=%v", raw, err)
		}
	}
	// 沒寫備註時不存。
	_, _, other := f.submitted(t, idB, "999888777666")
	again := f.reject(t, other, anIssue()...)
	if got := again.Reviews[len(again.Reviews)-1].InternalNote; got != nil {
		t.Fatalf("note=%v", *got)
	}
}

// outstanding 由伺服器在寫入申請時判定：值被改過才算處理（重填同一個值不算）、只改銀行代碼不算處理銀行帳號、文件被換掉才算處理
// 那一張（保留的不算）；重新送出之後是空的，再次退回時只算新的那一批。
func TestOutstandingIssuesAreResolvedByTheSellersEdits(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submitted(t, idA, bankAcct)
	f.reject(t, sub,
		issueOn("real_name", IssueMismatch), issueOn("bank_account", IssueInvalid), issueOn("birth_date", IssueMismatch),
		documentIssue("id_card_front", "asset-front", IssueUnreadable), documentIssue("id_card_back", "asset-back", IssueExpired),
		KYCIssue{Field: "bank_book", Code: IssueMissing, Message: "存摺封面缺少"},
	)
	all := []string{"real_name", "bank_account", "birth_date", "id_card_front@asset-front", "id_card_back@asset-back", "bank_book"}
	if got := f.outstanding(t, accountID); !slices.Equal(got, all) {
		t.Fatalf("outstanding=%v", got)
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	save := func(in KYCDraftInput) KYCApplication {
		t.Helper()
		in.AccountID = accountID
		in.ExpectedVersion = mine.Version
		out, err := f.module.SaveKYCDraft(ctx, in)
		must(t, err)
		mine = out
		return out
	}
	want := func(step string, remaining ...string) {
		t.Helper()
		if got := fields(mine.OutstandingIssues); !slices.Equal(got, remaining) {
			t.Fatalf("%s：save 回應的 outstanding=%v，預期 %v", step, got, remaining)
		}
		if got := f.outstanding(t, accountID); !slices.Equal(got, remaining) {
			t.Fatalf("%s：MyKYC 的 outstanding=%v，預期 %v", step, got, remaining)
		}
	}
	// 沒改任何東西、重填同樣的值、保留同一張圖：都不算處理。
	save(KYCDraftInput{})
	want("empty save", all...)
	save(KYCDraftInput{RealName: ptr("王小明"), BirthDate: ptr("1990-05-06"), BankAccount: ptr(bankAcct), IDCardBack: &KYCDocumentRef{AssetID: "asset-back"}})
	want("same values", all...)
	// 只改銀行代碼：銀行帳號的問題還在。
	save(KYCDraftInput{BankCode: ptr("005")})
	want("bank code only", all...)
	// 同時改銀行代碼並重填同一個帳號：銀行帳號的值沒變，不算處理（blind index 會隨銀行代碼變，不能拿來比）。
	save(KYCDraftInput{BankCode: ptr("006"), BankAccount: ptr(bankAcct)})
	want("bank code with the same account", all...)
	// 改了值才算：姓名、銀行帳號。
	save(KYCDraftInput{RealName: ptr("王大明")})
	want("real name", "bank_account", "birth_date", "id_card_front@asset-front", "id_card_back@asset-back", "bank_book")
	save(KYCDraftInput{BankAccount: ptr("999888777666")})
	want("bank account", "birth_date", "id_card_front@asset-front", "id_card_back@asset-back", "bank_book")
	// 換掉被指出的那張證件；另一張保留的不算。
	save(KYCDraftInput{IDCardFront: &KYCDocumentRef{StagedToken: "front2"}})
	want("front replaced", "birth_date", "id_card_back@asset-back", "bank_book")
	// 沒有指定特定一張的文件問題：那個位置的文件換掉才算。
	save(KYCDraftInput{BankBook: &KYCDocumentRef{StagedToken: "book2"}})
	want("book replaced", "birth_date", "id_card_back@asset-back")
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_review_issues WHERE resolved_at IS NOT NULL"); n != 4 {
		t.Fatalf("已處理的問題=%d", n)
	}
	// 重新送出：outstanding 與 last_rejection 都清空；沒處理的問題不被標記（已處理與否只由寫入申請判定）。
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: mine.Version})
	must(t, err)
	if len(again.OutstandingIssues) != 0 || again.LastRejection != nil {
		t.Fatalf("submitted=%+v", again)
	}
	if got := f.outstanding(t, accountID); len(got) != 0 {
		t.Fatalf("after submit outstanding=%v", got)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_review_issues WHERE resolved_at IS NULL"); n != 2 {
		t.Fatalf("重送不該標記問題：未處理=%d", n)
	}
	// 再次退回：只算新的一批，上一批沒處理的不會回來。
	second := f.reject(t, again, issueOn("english_name", IssueInvalid))
	if got := fields(second.OutstandingIssues); !slices.Equal(got, []string{"english_name"}) {
		t.Fatalf("second outstanding=%v", got)
	}
	// 重新送出後取消：沒有待處理的問題。
	resubmitted, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: second.Version})
	must(t, err)
	cancelled, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: resubmitted.Version})
	must(t, err)
	if len(cancelled.OutstandingIssues) != 0 || cancelled.LastRejection != nil {
		t.Fatalf("cancelled=%+v %+v", cancelled.OutstandingIssues, cancelled.LastRejection)
	}
}

// 被退回、編輯回到草稿後直接取消，以及退回、重送、核准之後：待處理的問題與 last_rejection 都是空的。
func TestRejectionIsClearedByCancelAndApprove(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submitted(t, idA, bankAcct)
	rej := f.reject(t, sub, issueOn("real_name", IssueMismatch))
	draft, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: rej.Version, EnglishName: ptr("Wang Farm Two")})
	must(t, err)
	if draft.LastRejection == nil || len(draft.OutstandingIssues) != 1 {
		t.Fatalf("取消之前仍是要處理的退回：%+v %+v", draft.LastRejection, draft.OutstandingIssues)
	}
	cancelled, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: accountID, ExpectedVersion: draft.Version})
	must(t, err)
	if cancelled.LastRejection != nil || len(cancelled.OutstandingIssues) != 0 {
		t.Fatalf("cancelled=%+v %+v", cancelled.LastRejection, cancelled.OutstandingIssues)
	}

	_, _, other := f.submitted(t, idB, "999888777666")
	rejected := f.reject(t, other, issueOn("real_name", IssueMismatch))
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: rejected.AccountID, ExpectedVersion: rejected.Version})
	must(t, err)
	approved, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: again.ID, ExpectedVersion: again.Version, AdminID: 9})
	must(t, err)
	if approved.LastRejection != nil || len(approved.OutstandingIssues) != 0 {
		t.Fatalf("approved=%+v %+v", approved.LastRejection, approved.OutstandingIssues)
	}
}

// 賣家把每一項問題都處理完、還沒重新送出時，退回的總理由仍在（last_rejection 由伺服器依「最近一次審核紀錄是退回」決定，
// client 不推導）；重新送出之後消失。
func TestLastRejectionStaysUntilResubmit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, sub := f.submitted(t, idA, bankAcct)
	f.reject(t, sub, issueOn("real_name", IssueMismatch), issueOn("birth_date", IssueMismatch))
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王大明"), BirthDate: ptr("1991-01-02")})
	must(t, err)
	for name, got := range map[string]KYCApplication{"save response": edited, "MyKYC": func() KYCApplication { a, err := f.module.MyKYC(ctx, accountID); must(t, err); return a }()} {
		if len(got.OutstandingIssues) != 0 || got.LastRejection == nil || got.LastRejection.Reason != "請補件" || len(got.LastRejection.Issues) != 2 {
			t.Fatalf("%s：全部處理完、還沒重送，outstanding=%v last_rejection=%+v", name, got.OutstandingIssues, got.LastRejection)
		}
	}
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: edited.Version})
	must(t, err)
	if again.LastRejection != nil || len(again.OutstandingIssues) != 0 {
		t.Fatalf("重送之後：%+v", again.LastRejection)
	}
	// 再次退回：新的一次才是要處理的。
	second := f.reject(t, again, issueOn("english_name", IssueInvalid))
	if second.LastRejection == nil || len(second.LastRejection.Issues) != 1 || second.LastRejection.Issues[0].Field != "english_name" {
		t.Fatalf("second=%+v", second.LastRejection)
	}
}

// 公司文件以 asset 定址：調整順序不算處理，被指出的那一張換掉才算，旁邊保留的那一張仍待處理；沒有指定特定一張的問題在
// 公司文件的組成改變時算處理。
func TestOutstandingCompanyDocumentIssuesFollowTheAssetNotThePosition(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	in := companyDraft(accountID, idRep, taxIDOne, bankAcct)
	in.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "co1"}, {StagedToken: "co2"}}
	d := f.savedDraft(t, in)
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	rej := f.reject(t, sub,
		documentIssue("business.company_documents", "asset-co1", IssueExpired), documentIssue("business.company_documents", "asset-co2", IssueUnreadable),
		issueOn("business.tax_id", IssueMismatch),
	)
	mine := rej.KYCApplication
	save := func(in KYCDraftInput) {
		t.Helper()
		in.AccountID, in.ExpectedVersion = accountID, mine.Version
		out, err := f.module.SaveKYCDraft(ctx, in)
		must(t, err)
		mine = out
	}
	// 順序反過來：兩張都還在。
	save(KYCDraftInput{CompanyDocuments: &[]KYCDocumentRef{{AssetID: "asset-co2"}, {AssetID: "asset-co1"}}})
	if got := f.outstanding(t, accountID); !slices.Equal(got, []string{"business.company_documents@asset-co1", "business.company_documents@asset-co2", "business.tax_id"}) {
		t.Fatalf("reordered outstanding=%v", got)
	}
	// 換掉 co2：只有 co2 的問題算處理。
	save(KYCDraftInput{CompanyDocuments: &[]KYCDocumentRef{{AssetID: "asset-co1"}, {StagedToken: "co3"}}})
	if got := f.outstanding(t, accountID); !slices.Equal(got, []string{"business.company_documents@asset-co1", "business.tax_id"}) {
		t.Fatalf("replaced outstanding=%v", got)
	}
	// 統編改掉才算；整份公司文件清空時 co1 也算處理。
	save(KYCDraftInput{TaxID: ptr("71777777"), CompanyDocuments: &[]KYCDocumentRef{}})
	if got := f.outstanding(t, accountID); len(got) != 0 {
		t.Fatalf("cleared outstanding=%v", got)
	}
}

// 新表的 CHECK 綁 Go 的值集合，並擋下 Go 層之外的壞資料。
func TestKYCReviewIssueChecksMatchGoAndRejectBadRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for constraint, want := range map[string][]string{
		"kyc_review_issues_field_check": KYCFieldNames(),
		"kyc_review_issues_code_check":  codeStrings(KYCIssueCodeValues()),
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
	// 文件類欄位的清單綁同一份 Go 的來源：只有文件類的欄位能帶 asset ID。
	var assetDef string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname='kyc_review_issues_asset_check'").Scan(&assetDef))
	documentFields := KYCDocumentFieldNames()
	for _, field := range KYCFieldNames() {
		if listed := strings.Contains(assetDef, "'"+field+"'"); listed != slices.Contains(documentFields, field) {
			t.Errorf("kyc_review_issues_asset_check 對 %s 的設定與 Go 不同：%s", field, assetDef)
		}
	}
	_, _, sub := f.submitted(t, idA, bankAcct)
	rej := f.reject(t, sub, anIssue()...)
	review := rej.Reviews[len(rej.Reviews)-1].ID.String()
	insert := func(field, asset, code, message string, position int) error {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.kyc_review_issues (issue_id, review_id, position, field, document_asset_id, code, message)
			VALUES (uuidv7(), $1, $2, $3, NULLIF($4, ''), $5, $6)`, review, position, field, asset, code, message)
		return err
	}
	if err := insert("bank_book", "asset-book", "OTHER", "x", 5); err != nil {
		t.Fatalf("合法的列被擋下：%v", err)
	}
	for name, err := range map[string]error{
		"asset on a non document": insert("real_name", "asset-x", "OTHER", "x", 6),
		"unknown field":           insert("favourite_colour", "", "OTHER", "x", 7),
		"unknown code":            insert("real_name", "", "FINAL", "x", 8),
		"empty message":           insert("real_name", "", "OTHER", "", 9),
		"overlong message":        insert("real_name", "", "OTHER", strings.Repeat("字", 501), 10),
		"negative position":       insert("real_name", "", "OTHER", "x", -1),
		"duplicate position":      insert("real_name", "", "OTHER", "x", 0),
	} {
		if err == nil {
			t.Errorf("%s 應該被 CHECK 擋下", name)
		}
	}
	// 內部備註只有退回有，1 到 1000 字。
	for name, query := range map[string]string{
		"note on a submit": "UPDATE seller.kyc_reviews SET internal_note = 'x' WHERE action = 'SUBMIT'",
		"empty note":       "UPDATE seller.kyc_reviews SET internal_note = '' WHERE action = 'REJECT'",
		"overlong note":    "UPDATE seller.kyc_reviews SET internal_note = repeat('註', 1001) WHERE action = 'REJECT'",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, query); err == nil {
			t.Errorf("%s 應該被 CHECK 擋下", name)
		}
	}
}

func codeStrings(codes []KYCIssueCode) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = string(c)
	}
	return out
}
