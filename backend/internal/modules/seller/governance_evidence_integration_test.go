//go:build integration

package seller_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// reportWithEvidence 建立一筆附 n 張佐證的通報（登入的買家），佐證的內容放進 media 的替身。
func (f *fixture) reportWithEvidence(t *testing.T, storeID, ownerAccountID int64, tokens ...string) Report {
	t.Helper()
	f.everExposed(t, storeID)
	r, err := f.module.SubmitReport(context.Background(), SubmitReportInput{
		// 檢舉人（上傳者）的帳號不能是店家自己的帳號：不能檢舉自己的店。
		StoreID: storeID, ReporterBuyerID: 5, AccountID: ownerAccountID + 1000, Category: policy.FoodSafety,
		Description: "附圖", EvidenceTokens: tokens,
	})
	must(t, err)
	ref := media.Ref{Type: media.RefTypeGovernanceEvidence, ID: r.Report.ID.String()}
	for i, e := range r.Report.Evidence {
		f.media.PutSecure(e.ObjectKey, ref, media.SecureAsset{Data: []byte(tokens[i]), ContentType: "image/jpeg"})
	}
	return *r.Report
}

// appealable 建立一個帳號、店家與一個可以申覆的措施（依據認定的 30 天停權）。
func (f *fixture) appealable(t *testing.T) (int64, Store, Measure) {
	t.Helper()
	accountID, s := f.open(t)
	ends := f.now.Add(30 * 24 * time.Hour)
	return accountID, s, f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
}

// appeal 以賣家讀到的版本對措施提出申覆。
func (f *fixture) appeal(t *testing.T, accountID int64, m Measure, reason string, tokens ...string) Measure {
	t.Helper()
	got, err := f.module.SubmitAppeal(context.Background(), SubmitAppealInput{
		AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: reason, EvidenceTokens: tokens,
	})
	must(t, err)
	return got
}

// 佐證必須是機敏上傳（Will 2026-10-04，8g）：media 回報 is_secure=false 的轉正在寫入之前就被拒絕（invalid_argument，欄位是
// 那一張 EvidenceTokens[i]），通報與申覆都不寫入。
func TestEvidenceMustBeSecureAssets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m := f.appealable(t)
	f.everExposed(t, s.StoreID)
	f.media.NotSecure = true
	_, err := f.module.SubmitReport(ctx, SubmitReportInput{
		StoreID: s.StoreID, ReporterBuyerID: 5, AccountID: accountID + 1000, Category: policy.Fraud, Description: "附圖",
		EvidenceTokens: []string{"p1"},
	})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) == 0 || ae.FieldViolations()[0].Field != "EvidenceTokens[0]" {
		t.Fatalf("report：%v", err)
	}
	if _, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "已改善", EvidenceTokens: []string{"p2"}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("appeal：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.reports"); n != 0 {
		t.Fatalf("被拒絕的通報不能留下：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_requests"); n != 0 {
		t.Fatalf("被拒絕的申訴不能留下：%d", n)
	}
	f.media.NotSecure = false
	if _, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "已改善", EvidenceTokens: []string{"p2"}}); err != nil {
		t.Fatalf("機敏的佐證可以送：%v", err)
	}
}

// 佐證的讀取：先寫讀取紀錄（誰、哪一張、實際送出的 asset ID），commit 之後才讀 media；通報以通報 ID、申覆以請求 ID 定址，位置
// 沒有佐證、紀錄不存在、種類與 ID 對不上都是 NotFound，而且不留紀錄、不問 media；紀錄寫不進去就不讀；media 失敗時紀錄仍在。
func TestEvidenceReadIsRecordedBeforeTheBytesAreRead(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m := f.appealable(t)
	report := f.reportWithEvidence(t, s.StoreID, accountID, "r0", "r1")
	f.appeal(t, accountID, m, "已改善", "a0")
	appealed := f.measureOf(t, m.ID)
	request := appealed.Appeal().ID
	f.media.PutSecure(appealed.Appeal().Evidence[0].ObjectKey, media.Ref{Type: media.RefTypeGovernanceEvidence, ID: request.String()},
		media.SecureAsset{Data: []byte("a0"), ContentType: "application/pdf"})

	for name, in := range map[string]ReadEvidenceInput{
		"位置沒有佐證":     {AdminID: 9, Kind: EvidenceReport, RefID: report.ID, Position: 2},
		"負的位置":       {AdminID: 9, Kind: EvidenceReport, RefID: report.ID, Position: -1},
		"位置超出上限":     {AdminID: 9, Kind: EvidenceReport, RefID: report.ID, Position: 5},
		"不存在的通報":     {AdminID: 9, Kind: EvidenceReport, RefID: uuid.NewV7(), Position: 0},
		"以請求 ID 讀通報": {AdminID: 9, Kind: EvidenceReport, RefID: request, Position: 0},
		"以措施 ID 讀申訴": {AdminID: 9, Kind: EvidenceRequest, RefID: m.ID, Position: 0},
		"以通報 ID 讀申訴": {AdminID: 9, Kind: EvidenceRequest, RefID: report.ID, Position: 0},
		"不認得的種類":     {AdminID: 9, Kind: "OTHER", RefID: report.ID, Position: 0},
	} {
		if _, err := f.module.ReadEvidence(ctx, in); apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s：%v", name, err)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.evidence_accesses") != 0 || f.readSecureCalls() != 0 {
		t.Fatal("找不到的佐證不能留下紀錄或問 media")
	}

	got, err := f.module.ReadEvidence(ctx, ReadEvidenceInput{AdminID: 9, Kind: EvidenceReport, RefID: report.ID, Position: 1})
	must(t, err)
	if string(got.Data) != "r1" || got.ContentType != "image/jpeg" {
		t.Fatalf("report evidence=%q %s", got.Data, got.ContentType)
	}
	got, err = f.module.ReadEvidence(ctx, ReadEvidenceInput{AdminID: 11, Kind: EvidenceRequest, RefID: request, Position: 0})
	must(t, err)
	if string(got.Data) != "a0" || got.ContentType != "application/pdf" {
		t.Fatalf("appeal evidence=%q %s", got.Data, got.ContentType)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.evidence_accesses WHERE report_id = $1 AND request_id IS NULL AND position = 1 AND admin_id = 9 AND accessed_at = $2",
		report.ID, f.now); n != 1 {
		t.Fatalf("report access rows=%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.evidence_accesses WHERE request_id = $1 AND report_id IS NULL AND position = 0 AND admin_id = 11", request); n != 1 {
		t.Fatalf("appeal access rows=%d", n)
	}

	// 讀取紀錄寫不進去：不讀、不回傳。
	_, err = f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.evidence_accesses ADD CONSTRAINT test_no_writes CHECK (false) NOT VALID")
	must(t, err)
	calls := f.readSecureCalls()
	got, err = f.module.ReadEvidence(ctx, ReadEvidenceInput{AdminID: 9, Kind: EvidenceReport, RefID: report.ID, Position: 0})
	if err == nil || len(got.Data) != 0 || f.readSecureCalls() != calls {
		t.Fatalf("紀錄寫不進去就不能讀：err=%v data=%q", err, got.Data)
	}
	_, err = f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.evidence_accesses DROP CONSTRAINT test_no_writes")
	must(t, err)

	// media 讀失敗：紀錄還在，錯誤原樣交回。
	f.media.FailReadSecure = errors.New("media down")
	if _, err := f.module.ReadEvidence(ctx, ReadEvidenceInput{AdminID: 9, Kind: EvidenceReport, RefID: report.ID, Position: 0}); err == nil {
		t.Fatal("media 失敗要回錯誤")
	}
	f.media.FailReadSecure = nil
	if n := f.count(t, "SELECT count(*) FROM seller.evidence_accesses"); n != 3 {
		t.Fatalf("accesses=%d", n)
	}
}

// media 讀不到佐證的三種結果（與 KYC 證件相同）：引用的同步還沒結算（ErrFileNotSynced）、同步過了卻讀不到（資料不一致，
// ErrFileMissing）、不是機敏資產（media.ErrNotSecure 原樣）。三種都先寫了讀取紀錄。
func TestEvidenceReadFailuresAreDistinguished(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	f.everExposed(t, s.StoreID)
	r, err := f.module.SubmitReport(ctx, SubmitReportInput{
		StoreID: s.StoreID, ReporterBuyerID: 5, AccountID: accountID + 1000, Category: policy.Fraud, Description: "附圖",
		EvidenceTokens: []string{"x0"},
	})
	must(t, err)
	read := func() error {
		_, err := f.module.ReadEvidence(ctx, ReadEvidenceInput{AdminID: 9, Kind: EvidenceReport, RefID: r.ReportID, Position: 0})
		return err
	}
	if err := read(); !errors.Is(err, ErrFileNotSynced) || errors.Is(err, ErrFileMissing) {
		t.Fatalf("同步還沒結算：%v", err)
	}
	f.syncMedia(t, media.Ref{Type: media.RefTypeGovernanceEvidence, ID: r.ReportID.String()})
	if err := read(); !errors.Is(err, ErrFileMissing) || errors.Is(err, ErrFileNotSynced) {
		t.Fatalf("同步過了還讀不到：%v", err)
	}
	f.media.FailReadSecure = media.ErrNotSecure
	if err := read(); !errors.Is(err, media.ErrNotSecure) || errors.Is(err, ErrFileMissing) || apperr.KindOf(err) == apperr.KindNotFound {
		t.Fatalf("不是機敏資產：%v", err)
	}
	f.media.FailReadSecure = nil
	if n := f.count(t, "SELECT count(*) FROM seller.evidence_accesses"); n != 3 {
		t.Fatalf("三次嘗試都要有紀錄：%d", n)
	}
}

// 措施的版本（Will 2026-10-11）：建立時寫第 1 版；每次實際改了賣家看得到的內容（理由、結束時間）就在同一個 transaction 寫下一版，
// 記完整的內容、誰、何時；沒有改變、版本衝突、被拒絕的修改都不產生版本。version 與 current_revision 各加一。
func TestMeasureRevisionsRecordEachSellerVisibleVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	ends := f.now.Add(24 * time.Hour)
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	if v.CurrentRevision != 1 {
		t.Fatalf("current revision=%d", v.CurrentRevision)
	}
	if n := f.count(t, `SELECT count(*) FROM seller.measure_revisions WHERE measure_id = $1 AND revision = 1 AND admin_id = 7 AND created_at = $2
		AND statement_to_seller = $3 AND ends_at = $4`, v.ID, f.now, v.StatementToSeller, ends); n != 1 {
		t.Fatalf("第 1 版：%d", n)
	}
	longer := f.now.Add(72 * time.Hour)
	if _, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 0, ExpectedVersion: v.Version, ChangeEndsAt: true, EndsAt: &longer}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("沒有管理員：%v", err)
	}
	if _, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 8, ExpectedVersion: v.Version + 1, ChangeEndsAt: true, EndsAt: &longer}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale：%v", err)
	}
	f.now = f.now.Add(time.Hour)
	updated, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 8, ExpectedVersion: v.Version, ChangeEndsAt: true, EndsAt: &longer})
	must(t, err)
	if updated.Version != v.Version+1 || updated.CurrentRevision != 2 {
		t.Fatalf("updated version=%d revision=%d", updated.Version, updated.CurrentRevision)
	}
	if n := f.count(t, `SELECT count(*) FROM seller.measure_revisions WHERE measure_id = $1 AND revision = 2 AND admin_id = 8 AND created_at = $2
		AND statement_to_seller = $3 AND ends_at = $4`, v.ID, f.now, v.StatementToSeller, longer); n != 1 {
		t.Fatalf("結束時間的版本：%d", n)
	}
	statement := "新的理由"
	f.now = f.now.Add(time.Hour)
	again, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 9, ExpectedVersion: updated.Version, StatementToSeller: &statement})
	must(t, err)
	if again.CurrentRevision != 3 {
		t.Fatalf("revision=%d", again.CurrentRevision)
	}
	if n := f.count(t, `SELECT count(*) FROM seller.measure_revisions WHERE measure_id = $1 AND revision = 3 AND admin_id = 9
		AND statement_to_seller = $2 AND ends_at = $3`, v.ID, statement, longer); n != 1 {
		t.Fatalf("理由的版本（結束時間沿用目前的）：%d", n)
	}
	same, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 9, ExpectedVersion: again.Version, StatementToSeller: &statement})
	must(t, err)
	if same.Version != again.Version || same.CurrentRevision != 3 {
		t.Fatalf("沒有改變不能產生版本：%+v", same)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_revisions WHERE measure_id = $1", v.ID); n != 3 {
		t.Fatalf("版本數：%d", n)
	}
	got := f.measureOf(t, v.ID)
	if len(got.Revisions) != 3 || got.Revisions[0].StatementToSeller != v.StatementToSeller || got.Revisions[2].StatementToSeller != statement ||
		!got.Revisions[0].EndsAt.Equal(ends) || !got.Revisions[1].EndsAt.Equal(longer) || got.Revisions[1].AdminID == nil || *got.Revisions[1].AdminID != 8 {
		t.Fatalf("revisions=%+v", got.Revisions)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures WHERE id = $1 AND statement_to_seller = $2 AND ends_at = $3 AND current_revision = 3", v.ID, statement, longer); n != 1 {
		t.Fatalf("measures 存的是目前值（第 3 版的理由與結束時間）：%d", n)
	}
	warning := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_revisions WHERE measure_id = $1 AND ends_at IS NULL", warning.ID); n != 1 {
		t.Fatalf("警告的版本沒有結束時間：%d", n)
	}
	f.assertCurrentValuesMatchHistory(t)
	for name, sql := range map[string]string{
		"第 0 版":    "INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ($1, 0, 'x', 1, now())",
		"空的理由":     "INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ($1, 9, '', 1, now())",
		"重複的版本":    "INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ($1, 1, 'x', 1, now())",
		"目前的版本不存在": "UPDATE seller.measures SET current_revision = 9 WHERE id = $1",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, sql, v.ID); err == nil {
			t.Fatalf("%s 應該被資料庫拒絕", name)
		}
	}
}

// 申覆的版本必須是措施真的有的那一版（外鍵）；申覆提出時複製當下的 current_revision，之後 admin 再改也不變。
func TestAppealRecordsTheRevisionItWasSubmittedAgainst(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, v := f.appealable(t)
	statement := "第二版的理由"
	f.now = f.now.Add(time.Hour)
	second, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 8, ExpectedVersion: v.Version, StatementToSeller: &statement})
	must(t, err)
	appealed := f.appeal(t, accountID, second, "誤判")
	if appealed.Appeal().Revision != 2 || appealed.CurrentRevision != 2 {
		t.Fatalf("appeal revision=%d current=%d", appealed.Appeal().Revision, appealed.CurrentRevision)
	}
	// 提出申覆起措施凍結，申覆之後只能縮短結束時間，這也產生新的一版。
	shorter := appealed.EndsAt.Add(-time.Hour)
	admin := f.measureOf(t, v.ID)
	third, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 8, ExpectedVersion: admin.Version, ChangeEndsAt: true, EndsAt: &shorter})
	must(t, err)
	if third.CurrentRevision != 3 || third.Appeal().Revision != 2 {
		t.Fatalf("申訴之後的修改不能改申訴的版本：current=%d appeal=%d", third.CurrentRevision, third.Appeal().Revision)
	}
	got := f.measureOf(t, v.ID)
	if got.Appeal().Revision != 2 || got.CurrentRevision != 3 || got.Revisions[got.Appeal().Revision-1].StatementToSeller != statement {
		t.Fatalf("got=%+v", got)
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.measure_requests SET revision = 9 WHERE measure_id = $1", v.ID); err == nil {
		t.Fatal("申訴的版本必須是措施真的有的那一版")
	}
	account, err := f.module.AccountMeasure(ctx, accountID, v.ID)
	must(t, err)
	if len(account.Revisions) != 0 {
		t.Fatalf("賣家讀到的措施不載入版本：%+v", account)
	}
}

// 內部註記（Will 2026-10-11）：建立措施時第一則必填；之後只新增，不能改也不能刪，舊到新；寫註記不動措施的 version 與
// current_revision，賣家與後台依舊 etag 的寫入不受影響；已撤銷的措施也可以寫，不存在的措施 NotFound。
func TestMeasureNotesAreAppendOnlyAndDoNotBumpAnyVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	ends := f.now.Add(24 * time.Hour)
	blank := f.warningFrom(s.StoreID)
	blank.Note = "  "
	if _, err := f.module.CreateMeasure(ctx, blank); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("第一則註記必填：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.violations") + f.count(t, "SELECT count(*) FROM seller.measures") + f.count(t, "SELECT count(*) FROM seller.governance_notes"); n != 0 {
		t.Fatalf("被拒絕的建立不能寫入：%d", n)
	}
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	notes := func() []GovernanceNote {
		t.Helper()
		var all []GovernanceNote
		var after *NoteCursor
		for {
			page, err := f.module.MeasureNotes(ctx, MeasureNoteListInput{MeasureID: v.ID, PageSize: 2, After: after})
			must(t, err)
			all = append(all, page.Items...)
			if page.Next == nil {
				return all
			}
			after = page.Next
		}
	}
	if first := notes(); len(first) != 1 || first[0].Body != "內部紀錄" || first[0].AdminID != 7 || !first[0].CreatedAt.Equal(f.now) {
		t.Fatalf("第一則註記：%+v", first)
	}
	f.now = f.now.Add(time.Hour)
	note, err := f.module.CreateMeasureNote(ctx, CreateMeasureNoteInput{MeasureID: v.ID, AdminID: 9, Body: "  補充調查  "})
	must(t, err)
	if note.Body != "補充調查" || note.AdminID != 9 || !note.CreatedAt.Equal(f.now) {
		t.Fatalf("note=%+v", note)
	}
	for _, in := range []CreateMeasureNoteInput{
		{MeasureID: v.ID, AdminID: 9, Body: " "},
		{MeasureID: v.ID, AdminID: 0, Body: "x"},
		{MeasureID: v.ID, AdminID: 9, Body: strings.Repeat("字", 2001)},
	} {
		if _, err := f.module.CreateMeasureNote(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%+v：%v", in, err)
		}
	}
	if _, err := f.module.CreateMeasureNote(ctx, CreateMeasureNoteInput{MeasureID: uuid.NewV7(), AdminID: 9, Body: "x"}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的措施：%v", err)
	}
	if _, err := f.module.MeasureNotes(ctx, MeasureNoteListInput{MeasureID: uuid.NewV7()}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的措施：%v", err)
	}
	after := f.measureOf(t, v.ID)
	if got := notes(); after.Version != v.Version || after.CurrentRevision != v.CurrentRevision || !after.UpdatedAt.Equal(v.UpdatedAt) || len(got) != 2 || got[0].Body != "內部紀錄" || got[1].ID != note.ID {
		t.Fatalf("寫註記不能改 version、current_revision 與 updated_at，舊到新：version %d → %d notes=%+v", v.Version, after.Version, got)
	}
	// 賣家讀到註記之前的 etag 照樣可以申覆；後台讀到註記之前的 etag 照樣可以裁定。
	appealed := f.appeal(t, accountID, v, "誤判")
	adminView := f.measureOf(t, v.ID)
	if adminView.Version != v.Version+1 || appealed.Version != v.Version+1 {
		t.Fatalf("appealed.Version=%d", appealed.Version)
	}
	// 申覆中、撤銷之後都可以繼續寫。
	decided, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: v.ID, AdminID: 7, ExpectedVersion: adminView.Version, Outcome: RequestAccepted, Reason: "接受"})
	must(t, err)
	if _, err := f.module.CreateMeasureNote(ctx, CreateMeasureNoteInput{MeasureID: v.ID, AdminID: 9, Body: "已撤銷之後的說明"}); err != nil {
		t.Fatal(err)
	}
	if final := f.measureOf(t, v.ID); final.Version != decided.Version || len(notes()) != 3 {
		t.Fatalf("final=%+v notes=%d", final, len(notes()))
	}
	// 只新增：資料庫沒有更新與刪除的 API，模組也沒有；這裡只確認 id 之外沒有可以重複的鍵（同一則不會寫兩次）。
	if n := f.count(t, "SELECT count(DISTINCT note_id) FROM seller.governance_notes WHERE measure_id = $1", v.ID); n != 3 {
		t.Fatalf("notes=%d", n)
	}
}

// 讀取紀錄必須屬於一筆通報或一筆請求，並指向存在的佐證。
func TestEvidenceAccessesRequireAnOwnerAndExistingEvidence(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, v := f.appealable(t)
	f.appeal(t, accountID, v, "誤判")
	request := f.measureOf(t, v.ID).Appeal().ID
	if _, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO seller.evidence_accesses (access_id, report_id, request_id, position, admin_id, accessed_at) VALUES (uuidv7(), NULL, NULL, 0, 1, now())"); err == nil {
		t.Fatal("讀取紀錄必須屬於一筆通報或一筆請求")
	}
	// 讀取紀錄以外鍵指向佐證列本身：沒有那一張佐證就寫不進去（這筆申覆沒有佐證）。
	if _, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO seller.evidence_accesses (access_id, request_id, position, admin_id, accessed_at) VALUES (uuidv7(), $1, 0, 1, now())", request); err == nil {
		t.Fatal("讀取紀錄必須指向存在的佐證")
	}
}

// 通報的對象只能是曾經公開過的店：從沒公開過的店與不存在的店同一個 NotFound（匿名入口不能探測店家 ID），什麼都不寫；曾經公開
// 過、之後關閉的店照樣可以通報。
func TestOnlyStoresThatWereEverPublicCanBeReported(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	submit := func(storeID int64) error {
		_, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: storeID, Phone: "0912345678", Category: policy.FoodSafety, Description: "x"})
		return err
	}
	never, missing := submit(s.StoreID), submit(s.StoreID+1000)
	var a, b *apperr.Error
	if !errors.As(never, &a) || !errors.As(missing, &b) || a.Kind() != apperr.KindNotFound || a.Message != b.Message {
		t.Fatalf("從沒公開過的店要與不存在的店相同：%v / %v", never, missing)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.reports"); n != 0 {
		t.Fatalf("reports=%d", n)
	}
	f.everExposed(t, s.StoreID)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if err := submit(s.StoreID); err != nil {
		t.Fatalf("曾經公開過、已關閉的店可以通報：%v", err)
	}
}

// 建立措施與寫註記的回應取資料庫存下的值：時鐘有奈秒時，timestamptz 只存到微秒，回應若用時鐘的值，之後的讀取就會不一樣。
func TestMeasureCreateAndNoteResponsesEqualTheStoredRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.now = f.now.Add(123456789 * time.Nanosecond)
	ends := f.now.Add(24 * time.Hour)
	created := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	read := f.measureOf(t, created.ID)
	if len(created.Revisions) != 1 || len(read.Revisions) != 1 {
		t.Fatalf("revisions created=%+v read=%+v", created.Revisions, read.Revisions)
	}
	cr, rr := created.Revisions[0], read.Revisions[0]
	if cr.Revision != rr.Revision || cr.StatementToSeller != rr.StatementToSeller || !sameAdmin(cr.AdminID, rr.AdminID) ||
		!cr.CreatedAt.Equal(rr.CreatedAt) || (cr.EndsAt == nil) != (rr.EndsAt == nil) || (cr.EndsAt != nil && !cr.EndsAt.Equal(*rr.EndsAt)) {
		t.Fatalf("建立的第 1 版與之後讀到的不同：\n created %+v\n read    %+v", cr, rr)
	}
	if !created.CreatedAt.Equal(read.CreatedAt) || !created.UpdatedAt.Equal(read.UpdatedAt) || !created.EndsAt.Equal(*read.EndsAt) {
		t.Fatalf("建立時間：created %v/%v read %v/%v", created.CreatedAt, created.UpdatedAt, read.CreatedAt, read.UpdatedAt)
	}
	f.now = f.now.Add(time.Hour + 987654321*time.Nanosecond)
	note, err := f.module.CreateMeasureNote(ctx, CreateMeasureNoteInput{MeasureID: created.ID, AdminID: 9, Body: "補充"})
	must(t, err)
	page, err := f.module.MeasureNotes(ctx, MeasureNoteListInput{MeasureID: created.ID, PageSize: 10})
	must(t, err)
	if len(page.Items) != 2 {
		t.Fatalf("notes=%+v", page.Items)
	}
	got := page.Items[1]
	if got.ID != note.ID || got.AdminID != note.AdminID || got.Body != note.Body || !got.CreatedAt.Equal(note.CreatedAt) {
		t.Fatalf("註記回應與之後讀到的不同：\n created %+v\n read    %+v", note, got)
	}
}

func sameAdmin(a, b *int64) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
