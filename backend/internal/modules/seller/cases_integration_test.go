//go:build integration

package seller_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// 通報與案件的測試共用的小工具：每個指令都以「此刻讀到的版本」呼叫（過期版本的行為另有測試），回傳模組的後台檢視。

var nextBuyer atomic.Int64

// reportBy 以一位沒用過的買家檢舉 storeID（類別 category），回傳通報。
func (f *fixture) reportBy(t *testing.T, storeID int64, category policy.Category) Report {
	t.Helper()
	buyer := 1000 + nextBuyer.Add(1)
	return *f.buyerReport(t, storeID, buyer, category).Report
}

func (f *fixture) reportOf(t *testing.T, id uuid.UUID) AdminReport {
	t.Helper()
	r, err := f.module.Report(context.Background(), id)
	must(t, err)
	return r
}

func (f *fixture) caseOf(t *testing.T, id uuid.UUID) AdminCase {
	t.Helper()
	c, err := f.module.Case(context.Background(), id)
	must(t, err)
	return c
}

// targetFor 組出去處的意圖，像 client 照 Report.triage_targets 做的：severity 有值是開新案件，nil 是加入（照通報此刻讀到的 triage_targets 取該類別進行中的案件的 id 與版本；
// 該類別沒有進行中的案件時回空的去處，由指令以欄位錯誤擋下）。
func (f *fixture) targetFor(t *testing.T, reportID uuid.UUID, category policy.Category, severity *Severity) CaseTarget {
	t.Helper()
	if severity != nil {
		return CaseTarget{OpenNew: severity}
	}
	report := f.reportOf(t, reportID)
	for _, target := range report.Targets {
		if target.Category == category && target.Case != nil {
			return CaseTarget{Join: &JoinCase{CaseID: target.Case.ID, ExpectedVersion: target.Case.Version}}
		}
	}
	// 通報此刻不能分案也不能移動時沒有 triage_targets：改讀案件清單，讓指令走到狀態檢查（回不可用的原因）。
	open := CaseStateOpen
	page, err := f.module.AdminCases(context.Background(), CaseListInput{State: &open, Category: &category, StoreID: report.StoreID, PageSize: 1})
	must(t, err)
	if len(page.Items) == 1 {
		return CaseTarget{Join: &JoinCase{CaseID: page.Items[0].ID, ExpectedVersion: page.Items[0].Version}}
	}
	return CaseTarget{}
}

// triage 以通報此刻的版本分案（管理員 7）。
func (f *fixture) triage(t *testing.T, reportID uuid.UUID, category policy.Category, severity *Severity) (AdminReport, error) {
	t.Helper()
	return f.module.TriageReport(context.Background(), TriageReportInput{
		ReportID: reportID, AdminID: 7, AssessedCategory: category, Target: f.targetFor(t, reportID, category, severity), ExpectedVersion: f.reportOf(t, reportID).Version,
	})
}

func (f *fixture) mustTriage(t *testing.T, reportID uuid.UUID, category policy.Category, severity *Severity) AdminReport {
	t.Helper()
	r, err := f.triage(t, reportID, category, severity)
	must(t, err)
	return r
}

// resolve 以通報此刻的版本設定結果（管理員 8）。
func (f *fixture) resolve(t *testing.T, reportID uuid.UUID, outcome Outcome, note string) (AdminReport, error) {
	t.Helper()
	return f.module.ResolveReport(context.Background(), ResolveReportInput{
		ReportID: reportID, AdminID: 8, Outcome: outcome, Note: note, ExpectedVersion: f.reportOf(t, reportID).Version,
	})
}

func (f *fixture) mustResolve(t *testing.T, reportID uuid.UUID, outcome Outcome) AdminReport {
	t.Helper()
	r, err := f.resolve(t, reportID, outcome, "")
	must(t, err)
	return r
}

// move 以通報此刻的版本移到 category（管理員 9）。
func (f *fixture) move(t *testing.T, reportID uuid.UUID, category policy.Category, severity *Severity) (MoveReportResult, error) {
	t.Helper()
	return f.module.MoveReport(context.Background(), MoveReportInput{
		ReportID: reportID, AdminID: 9, AssessedCategory: category, Target: f.targetFor(t, reportID, category, severity), ExpectedVersion: f.reportOf(t, reportID).Version,
	})
}

// decide 以案件此刻的版本決定（管理員 10）。
func (f *fixture) decide(t *testing.T, caseID uuid.UUID, decision Decision, note string) (AdminCase, error) {
	t.Helper()
	return f.module.DecideCase(context.Background(), DecideCaseInput{
		CaseID: caseID, AdminID: 10, Decision: decision, Note: note, ExpectedVersion: f.caseOf(t, caseID).Version,
	})
}

// newCase 在 storeID 上開一個進行中的案件（平台判定類別 category，嚴重度 severity），成員是 n 筆不同買家的檢舉（檢舉人選 FRAUD）；回傳案件與成員（舊到新）。
func (f *fixture) newCase(t *testing.T, storeID int64, category policy.Category, severity Severity, n int) (AdminCase, []AdminReport) {
	t.Helper()
	var members []AdminReport
	var caseID uuid.UUID
	for i := range n {
		r := f.reportBy(t, storeID, policy.Fraud)
		var sev *Severity
		if i == 0 {
			sev = &severity
		}
		triaged := f.mustTriage(t, r.ID, category, sev)
		caseID = *triaged.CaseID
		members = append(members, triaged)
	}
	return f.caseOf(t, caseID), members
}

// riskFromCase 從案件建立一個 RISK 停權（任何依據都可以連到案件；RISK 沒有來源通報，只能這樣連）。
func (f *fixture) riskFromCase(t *testing.T, storeID int64, caseID uuid.UUID) Measure {
	t.Helper()
	review := f.now.Add(7 * 24 * time.Hour)
	r, err := f.module.CreateMeasure(context.Background(), CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: MeasureSuspension, StatementToSeller: "疑似食安事件", Note: "從案件建立", ReviewBy: &review,
		Risk: &RiskBasis{Ground: RiskFoodSafety}, SourceCaseID: &caseID,
	})
	must(t, err)
	return r.Measure
}

// warningCiting 建立一個警告，依據是以 sources 為來源通報的新認定（導出關係由認定推導）。
func (f *fixture) warningCiting(t *testing.T, storeID int64, sources ...uuid.UUID) Measure {
	t.Helper()
	r, err := f.module.CreateMeasure(context.Background(), f.warningFrom(storeID, sources...))
	must(t, err)
	return r.Measure
}

// invalidFields 回傳錯誤是 invalid_argument 時指到的欄位（模組輸入的欄位名）。
func invalidFields(t *testing.T, err error) []string {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("要是 invalid_argument：%v", err)
	}
	var out []string
	for _, v := range ae.FieldViolations() {
		out = append(out, v.Field)
	}
	return out
}

// wantBlocked 確認指令被前置條件擋下（failed_precondition，訊息是原因的句子）。
func wantBlocked(t *testing.T, err error, sentence string) {
	t.Helper()
	wantKind(t, err, apperr.KindFailedPrecondition)
	if !strings.Contains(err.Error(), sentence) {
		t.Fatalf("訊息要是 %q：%v", sentence, err)
	}
}

// assertCaseInvariants 在每個指令之後對全部案件檢查跨列不變量（DB 保證不了，由模組保證）：
// I1 進行中的案件至少有一筆通報；I2 已結案（DECIDED）的案件每筆通報都有結果；I3 決定是不採取措施的案件沒有結果是已採取措施的通報；
// I4 決定是採取措施的案件有導出的措施（不論狀態）或結果是已採取措施的成員；I5 每個進行中的案件至少有一個可以選的決定；I6 有導出措施的案件不會因移動而變空。
func (f *fixture) assertCaseInvariants(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	for name, query := range map[string]string{
		"I1 進行中的案件沒有通報":        `SELECT count(*) FROM seller.cases c WHERE c.closed_at IS NULL AND NOT EXISTS (SELECT 1 FROM seller.reports r WHERE r.case_id = c.id)`,
		"I2 已結案的案件有沒有結果的通報":    `SELECT count(*) FROM seller.reports r JOIN seller.cases c ON c.id = r.case_id WHERE c.closed_at IS NOT NULL AND r.outcome IS NULL`,
		"I3 不採取措施的案件有已採取措施的通報": `SELECT count(*) FROM seller.cases c WHERE c.decision = 'NO_ACTION' AND EXISTS (SELECT 1 FROM seller.reports r WHERE r.case_id = c.id AND r.outcome = 'ACTION_TAKEN')`,
		"I4 採取措施的案件沒有措施也沒有已採取措施的成員": `SELECT count(*) FROM seller.cases c WHERE c.decision = 'ACTION_TAKEN'
			AND NOT EXISTS (SELECT 1 FROM seller.reports r WHERE r.case_id = c.id AND r.outcome = 'ACTION_TAKEN')
			AND NOT EXISTS (SELECT 1 FROM seller.measures m WHERE m.source_case_id = c.id)
			AND NOT EXISTS (SELECT 1 FROM seller.reports r JOIN seller.finding_reports fr ON fr.report_id = r.id JOIN seller.measure_findings mf ON mf.violation_id = fr.violation_id WHERE r.case_id = c.id)`,
		"I6 有連結措施的案件被清空":               `SELECT count(*) FROM seller.cases c JOIN seller.measures m ON m.source_case_id = c.id WHERE c.close_reason = 'NO_REPORTS'`,
		"J1 已結案的案件還有沒結束的詢問":            `SELECT count(*) FROM seller.case_inquiries i JOIN seller.cases c ON c.id = i.case_id WHERE c.closed_at IS NOT NULL AND i.reply_received_at IS NULL AND i.withdrawn_at IS NULL`,
		"closed_at 與 close_reason 不一致": `SELECT count(*) FROM seller.cases WHERE (closed_at IS NULL) <> (close_reason IS NULL)`,
		"一家店一個類別有兩個進行中的案件":             `SELECT count(*) FROM (SELECT store_id, category FROM seller.cases WHERE closed_at IS NULL GROUP BY 1, 2 HAVING count(*) > 1) x`,
	} {
		if n := f.count(t, query); n != 0 {
			t.Errorf("不變量 %s：%d 筆", name, n)
		}
	}
	// I5：每個進行中的案件至少有一個可以選的決定。
	open := CaseStateOpen
	var after *CaseCursor
	for {
		page, err := f.module.AdminCases(ctx, CaseListInput{State: &open, PageSize: 50, After: after})
		must(t, err)
		for _, c := range page.Items {
			if len(c.Availability.DecisionOptions()) == 0 {
				t.Errorf("不變量 I5：進行中的案件 %s 沒有可以選的決定：%+v", c.ID, c.Availability.Decide)
			}
		}
		if page.Next == nil {
			return
		}
		after = page.Next
	}
}

func severityPtr(s Severity) *Severity { return &s }

// wantViolates 確認語句被資料庫以 constraint 這一條約束（或唯一索引）擋下，而且只違反它：拿掉那一條之後，同一個語句就成功。
// 兩個條件缺一不可：只看「有錯誤」，參數數量不符之類不是資料庫擋下的錯誤也會算通過；一個語句同時違反兩條約束時，資料庫只回報其中一條，
// 另一條被拿掉測試仍然通過。拿掉約束在 transaction 裡做，結束就還原。
func (f *fixture) wantViolates(t *testing.T, name, constraint, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	_, err := f.db.Conn(ctx).Exec(ctx, sql, args...)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		t.Errorf("%s：要被約束 %s 擋下，得到的不是資料庫的錯誤：%v：%s", name, constraint, err, sql)
		return
	}
	if pg.ConstraintName != constraint {
		t.Errorf("%s：被約束 %s 擋下（%s），要是 %s：%s", name, pg.ConstraintName, pg.Message, constraint, sql)
		return
	}
	tx, err := f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	var isConstraint bool
	must(t, tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1 AND conrelid = $2::regclass)", constraint, pgx.Identifier{pg.SchemaName, pg.TableName}.Sanitize()).Scan(&isConstraint))
	drop := "DROP INDEX " + pgx.Identifier{pg.SchemaName, constraint}.Sanitize() // 唯一索引沒有約束
	if isConstraint {
		drop = "ALTER TABLE " + pgx.Identifier{pg.SchemaName, pg.TableName}.Sanitize() + " DROP CONSTRAINT " + pgx.Identifier{constraint}.Sanitize()
	}
	_, err = tx.Exec(ctx, drop)
	must(t, err)
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Errorf("%s：拿掉 %s 之後語句仍被擋下（同時違反另一條約束，那一條被遮住了）：%v：%s", name, constraint, err, sql)
	}
}

// 分案：判定平台判定類別，掛到這家店那個類別進行中的案件，沒有就開一個（必須給嚴重度）；已有進行中的案件時不可以再給嚴重度。
func TestTriageOpensTheCaseForTheStoreAndCategoryAndLaterReportsJoinIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, other := f.open(t)
	r1 := f.reportBy(t, s.StoreID, policy.Fraud)
	r2 := f.reportBy(t, s.StoreID, policy.Fraud)
	r3 := f.reportBy(t, s.StoreID, policy.Counterfeit)
	foreign := f.reportBy(t, other.StoreID, policy.Fraud)
	start := f.reportOf(t, r1.ID)

	// 先做無狀態的驗證：去處沒給或兩個都給、開新案件沒給嚴重度、類別是 OTHER（只給檢舉人選）或不在平台判定的子集、嚴重度不合法；被拒絕的什麼都不寫。
	openHigh := CaseTarget{OpenNew: severityPtr(SeverityHigh)}
	for name, c := range map[string]struct {
		in    TriageReportInput
		field string
	}{
		"沒有去處":      {TriageReportInput{AssessedCategory: policy.Fraud}, "Target"},
		"兩個去處都給":    {TriageReportInput{AssessedCategory: policy.Fraud, Target: CaseTarget{OpenNew: severityPtr(SeverityHigh), Join: &JoinCase{CaseID: uuid.NewV7(), ExpectedVersion: 1}}}, "Target"},
		"開新案件沒給嚴重度": {TriageReportInput{AssessedCategory: policy.Fraud, Target: CaseTarget{OpenNew: severityPtr("")}}, "Target"},
		"類別 OTHER":  {TriageReportInput{AssessedCategory: policy.Other, Target: openHigh}, "AssessedCategory"},
		"沒選類別":      {TriageReportInput{Target: openHigh}, "AssessedCategory"},
		"已經改名的舊值":   {TriageReportInput{AssessedCategory: "UNAUTHORIZED_SALE", Target: openHigh}, "AssessedCategory"},
		"嚴重度不合法":    {TriageReportInput{AssessedCategory: policy.Fraud, Target: CaseTarget{OpenNew: severityPtr("EXTREME")}}, "Target"},
	} {
		in := c.in
		in.ReportID, in.AdminID, in.ExpectedVersion = r1.ID, 7, start.Version
		_, err := f.module.TriageReport(ctx, in)
		if got := invalidFields(t, err); !slices.Equal(got, []string{c.field}) {
			t.Errorf("%s：欄位 %v，預期 %s", name, got, c.field)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.cases"); n != 0 || f.reportOf(t, r1.ID).Version != start.Version {
		t.Fatalf("被拒絕的分案寫入了：cases=%d", n)
	}

	opened := f.mustTriage(t, r1.ID, policy.Fraud, severityPtr(SeverityHigh))
	if opened.CaseID == nil || opened.AssessedCategory == nil || *opened.AssessedCategory != policy.Fraud || opened.Category != policy.Fraud ||
		opened.TriagedByAdminID == nil || *opened.TriagedByAdminID != 7 || opened.TriagedAt == nil || !opened.TriagedAt.Equal(f.now) ||
		opened.Version != start.Version+1 || opened.State() != ReportStateInCase || opened.Outcome != nil {
		t.Fatalf("opened=%+v", opened)
	}
	c := f.caseOf(t, *opened.CaseID)
	if c.Severity != SeverityHigh || c.Category != policy.Fraud || c.StoreID != s.StoreID || c.OpenedByAdminID != 7 || !c.OpenedAt.Equal(f.now) || c.Version != 1 ||
		c.ReportCount != 1 || c.UnresolvedReportCount != 1 || c.State() != CaseStateOpen || c.CloseReason != nil || c.Decision != nil {
		t.Fatalf("case=%+v", c.Case)
	}

	// 已有進行中的案件：去處是意圖，要開新案件的意圖不成立（版本衝突，conflictTarget 是 category_case_opened，什麼都不寫），要加入就加入它（案件的版本加一，嚴重度以案件為準、不變）。
	// 加入時案件的版本前進了（別人先處理過），意圖同樣不成立。
	before := f.reportOf(t, r2.ID)
	_, err := f.triage(t, r2.ID, policy.Fraud, severityPtr(SeverityLow))
	wantKind(t, err, apperr.KindVersionConflict)
	wantConflictTarget(t, err, "category_case_opened")
	_, err = f.module.TriageReport(ctx, TriageReportInput{
		ReportID: r2.ID, AdminID: 7, AssessedCategory: policy.Fraud, Target: CaseTarget{Join: &JoinCase{CaseID: c.ID, ExpectedVersion: c.Version + 1}}, ExpectedVersion: before.Version,
	})
	wantKind(t, err, apperr.KindVersionConflict)
	wantConflictTarget(t, err, "target_case")
	if after := f.reportOf(t, r2.ID); after.Version != before.Version || after.CaseID != nil || f.caseOf(t, c.ID).ReportCount != 1 {
		t.Fatalf("被拒絕的分案寫入了：report=%+v", after)
	}
	f.now = f.now.Add(time.Minute)
	joined := f.mustTriage(t, r2.ID, policy.Fraud, nil)
	if joined.CaseID == nil || *joined.CaseID != c.ID {
		t.Fatalf("joined=%+v", joined)
	}
	if after := f.caseOf(t, c.ID); after.Severity != SeverityHigh || after.ReportCount != 2 || after.Version != c.Version+1 || !after.UpdatedAt.Equal(f.now) {
		t.Fatalf("after=%+v", after.Case)
	}

	// 平台判定類別不是檢舉人選的類別：檢舉人選仿冒的，也可以判成詐欺加入同一個案件；檢舉人選的類別不會被改掉。
	counterfeit := f.mustTriage(t, r3.ID, policy.Fraud, nil)
	if counterfeit.Category != policy.Counterfeit || *counterfeit.AssessedCategory != policy.Fraud || *counterfeit.CaseID != c.ID {
		t.Fatalf("counterfeit=%+v", counterfeit)
	}
	// 另一家店的同類別是另一個案件。
	foreignCase := f.mustTriage(t, foreign.ID, policy.Fraud, severityPtr(SeverityLow))
	if *foreignCase.CaseID == c.ID || f.caseOf(t, *foreignCase.CaseID).StoreID != other.StoreID {
		t.Fatalf("foreign=%+v", foreignCase)
	}

	// 狀態不允許：已經分案、已有結果（不開案件直接結掉的）；過期的版本先回版本衝突。
	if _, err := f.triage(t, r1.ID, policy.Fraud, nil); err == nil {
		t.Fatal("已分案的通報不能再分案")
	} else {
		wantBlocked(t, err, "這筆通報已經分案。")
	}
	direct := f.reportBy(t, s.StoreID, policy.Fraud)
	f.mustResolve(t, direct.ID, OutcomeNoViolation)
	if _, err := f.triage(t, direct.ID, policy.Fraud, nil); err == nil {
		t.Fatal("已有結果的通報不能分案")
	} else {
		wantBlocked(t, err, "這筆通報已經有結果。")
	}
	stale := f.reportBy(t, s.StoreID, policy.Fraud)
	_, err = f.module.TriageReport(ctx, TriageReportInput{ReportID: stale.ID, AdminID: 7, AssessedCategory: policy.Fraud, Target: openHigh, ExpectedVersion: stale.Version + 5})
	wantKind(t, err, apperr.KindVersionConflict)
	wantConflictTarget(t, err, "report")
	_, err = f.module.TriageReport(ctx, TriageReportInput{ReportID: uuid.NewV7(), AdminID: 7, AssessedCategory: policy.Fraud, Target: openHigh, ExpectedVersion: 1})
	wantKind(t, err, apperr.KindNotFound)

	if n := f.count(t, "SELECT count(*) FROM seller.cases WHERE closed_at IS NULL"); n != 2 {
		t.Fatalf("進行中的案件 %d 個，預期 2（兩家店各一個）", n)
	}
	f.assertCaseInvariants(t)
}

// 設定結果：不開案件直接結掉只能設定除了已採取措施以外的四個；已採取措施只能設在案件裡的通報，而且案件要有導出的措施或已有成員的結果是已採取措施；
// 結果設定後不能修改；每筆通報各發一次 ReportResolved。
func TestResolveRulesFollowTheCaseAndItsMeasures(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	resolved := record[ReportResolved](f)
	_, s := f.open(t)

	// 不開案件直接結掉。
	var direct []AdminReport
	for _, o := range []Outcome{OutcomeNoViolation, OutcomeInsufficient, OutcomeDuplicate, OutcomeAbusive} {
		r := f.reportBy(t, s.StoreID, policy.Fraud)
		f.now = f.now.Add(time.Minute)
		got, err := f.resolve(t, r.ID, o, "  感謝你的檢舉  ")
		must(t, err)
		if got.Outcome == nil || *got.Outcome != o || got.ResolvedByAdminID == nil || *got.ResolvedByAdminID != 8 || got.ResolvedAt == nil || !got.ResolvedAt.Equal(f.now) ||
			got.ResolutionNote == nil || *got.ResolutionNote != "感謝你的檢舉" || got.State() != ReportStateResolved || got.CaseID != nil {
			t.Fatalf("%s：%+v", o, got)
		}
		direct = append(direct, got)
	}
	awaiting := f.reportBy(t, s.StoreID, policy.Fraud)
	_, err := f.resolve(t, awaiting.ID, OutcomeActionTaken, "")
	wantBlocked(t, err, "還沒分案的通報不能設為已採取措施；請先分案。")

	// 結果設定後不能修改；其他驗證。
	_, err = f.resolve(t, direct[0].ID, OutcomeAbusive, "")
	wantBlocked(t, err, "這筆通報已經有結果。")
	for name, in := range map[string]ResolveReportInput{
		"沒選結果": {Outcome: ""}, "結果不認得": {Outcome: "WITHDRAWN"}, "說明太長": {Outcome: OutcomeNoViolation, Note: strings.Repeat("字", 2001)},
	} {
		in.ReportID, in.AdminID, in.ExpectedVersion = awaiting.ID, 8, f.reportOf(t, awaiting.ID).Version
		_, err := f.module.ResolveReport(ctx, in)
		if len(invalidFields(t, err)) != 1 {
			t.Errorf("%s：%v", name, err)
		}
	}
	_, err = f.module.ResolveReport(ctx, ResolveReportInput{ReportID: awaiting.ID, AdminID: 8, Outcome: OutcomeNoViolation, ExpectedVersion: 99})
	wantKind(t, err, apperr.KindVersionConflict)
	_, err = f.module.ResolveReport(ctx, ResolveReportInput{ReportID: uuid.NewV7(), AdminID: 8, Outcome: OutcomeNoViolation, ExpectedVersion: 1})
	wantKind(t, err, apperr.KindNotFound)

	// 案件裡：沒有導出的措施、也沒有已採取措施的成員時，已採取措施被擋下，其他結果可以；案件的版本與 updated_at 隨之前進。
	c, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityMedium, 3)
	_, err = f.resolve(t, members[0].ID, OutcomeActionTaken, "")
	wantBlocked(t, err, "案件還沒有從它建立的措施，不能設為已採取措施。")
	before := f.caseOf(t, c.ID)
	f.now = f.now.Add(time.Minute)
	got, err := f.resolve(t, members[0].ID, OutcomeInsufficient, "")
	must(t, err)
	after := f.caseOf(t, c.ID)
	if got.CaseID == nil || *got.CaseID != c.ID || after.Version != before.Version+1 || !after.UpdatedAt.Equal(f.now) || after.UnresolvedReportCount != 2 || after.ReportCount != 3 {
		t.Fatalf("got=%+v after=%+v", got, after.Case)
	}
	// 有從這個案件建立的措施（連結）之後，已採取措施可以設；再有成員是已採取措施，另一筆也可以。
	f.riskFromCase(t, s.StoreID, c.ID)
	got, err = f.resolve(t, members[1].ID, OutcomeActionTaken, "")
	must(t, err)
	if *got.Outcome != OutcomeActionTaken {
		t.Fatalf("got=%+v", got)
	}
	f.assertCaseInvariants(t)

	events := resolved.all()
	if len(events) != 6 {
		t.Fatalf("ReportResolved 每筆通報各發一次：%d", len(events))
	}
	for i, e := range events[:4] {
		if e.Outcome != *direct[i].Outcome || e.CaseID != nil || e.ReporterBuyerID == nil || e.StoreID != s.StoreID {
			t.Errorf("直接結掉的事件 %d：%+v", i, e)
		}
	}
	if last := events[5]; last.Outcome != OutcomeActionTaken || last.CaseID == nil || *last.CaseID != c.ID || last.ReportID != members[1].ID {
		t.Fatalf("案件裡的事件：%+v", last)
	}
}

// 移動：改平台判定類別，通報移到這家店那個類別的案件；留紀錄；移走最後一筆時原案件以沒有通報關閉（不算決定、不發事件）。
func TestMoveReportsBetweenCasesKeepsARecordAndClosesAnEmptiedCase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	resolved := record[ReportResolved](f)
	_, s := f.open(t)
	c1, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityHigh, 3)
	a, b, c := members[0], members[1], members[2]

	// 沒有指定去處（這個類別沒有進行中的案件可加入、也沒有選開新案件）；類別和目前相同、類別不合法是 invalid_argument；什麼都不寫。
	_, err := f.move(t, a.ID, policy.MisleadingInfo, nil)
	if got := invalidFields(t, err); !slices.Equal(got, []string{"Target"}) {
		t.Fatalf("沒有去處：%v", got)
	}
	_, err = f.move(t, a.ID, policy.Fraud, nil)
	if got := invalidFields(t, err); !slices.Equal(got, []string{"AssessedCategory"}) {
		t.Fatalf("移動到相同的類別：%v", got)
	}
	_, err = f.move(t, a.ID, policy.Other, severityPtr(SeverityLow))
	invalidFields(t, err)
	if f.count(t, "SELECT count(*) FROM seller.report_case_moves") != 0 || f.caseOf(t, c1.ID).Version != c1.Version {
		t.Fatal("被拒絕的移動寫入了")
	}

	triagedAt := f.reportOf(t, a.ID).TriagedAt
	f.now = f.now.Add(time.Hour)
	moved, err := f.move(t, a.ID, policy.MisleadingInfo, severityPtr(SeverityLow))
	must(t, err)
	if moved.SourceCaseClosed || moved.Report.CaseID == nil || *moved.Report.CaseID == c1.ID || *moved.Report.AssessedCategory != policy.MisleadingInfo ||
		!moved.Report.TriagedAt.Equal(*triagedAt) || *moved.Report.TriagedByAdminID != 7 || moved.Report.Version != a.Version+1 {
		t.Fatalf("moved=%+v", moved)
	}
	c2 := f.caseOf(t, *moved.Report.CaseID)
	after1 := f.caseOf(t, c1.ID)
	if c2.Severity != SeverityLow || c2.OpenedByAdminID != 9 || c2.ReportCount != 1 || after1.ReportCount != 2 || after1.Version != c1.Version+1 || !after1.UpdatedAt.Equal(f.now) {
		t.Fatalf("c2=%+v after1=%+v", c2.Case, after1.Case)
	}
	if after := f.caseOf(t, c1.ID); len(after.Moves) != 1 || after.Moves[0].ReportID != a.ID || after.Moves[0].FromCaseID != c1.ID || after.Moves[0].ToCaseID != c2.ID || after.Moves[0].AdminID != 9 {
		t.Fatalf("移動紀錄：%+v", after.Moves)
	}
	if got := f.caseOf(t, c2.ID).Moves; len(got) != 1 {
		t.Fatalf("目標案件也看得到移入的紀錄：%+v", got)
	}

	// 目標類別已有進行中的案件：要開新案件的意圖不成立（版本衝突，conflictTarget 是 category_case_opened，什麼都不寫），要加入就加入它。
	_, err = f.move(t, b.ID, policy.MisleadingInfo, severityPtr(SeverityHigh))
	wantKind(t, err, apperr.KindVersionConflict)
	wantConflictTarget(t, err, "category_case_opened")
	joined, err := f.move(t, b.ID, policy.MisleadingInfo, nil)
	must(t, err)
	if *joined.Report.CaseID != c2.ID || joined.SourceCaseClosed || f.caseOf(t, c2.ID).ReportCount != 2 {
		t.Fatalf("joined=%+v", joined)
	}

	// 移走最後一筆：原案件以沒有通報自動關閉，不算決定，不發事件；結果、說明與決定都是空的。
	events := len(resolved.all())
	f.now = f.now.Add(time.Hour)
	last, err := f.move(t, c.ID, policy.MisleadingInfo, nil)
	must(t, err)
	closed := f.caseOf(t, c1.ID)
	if !last.SourceCaseClosed || closed.State() != CaseStateClosed || closed.CloseReason == nil || *closed.CloseReason != CaseClosedNoReports || closed.Decision != nil ||
		closed.DecisionNote != nil || closed.ClosedByAdminID == nil || *closed.ClosedByAdminID != 9 || closed.ClosedAt == nil || !closed.ClosedAt.Equal(f.now) ||
		closed.ReportCount != 0 || len(closed.Availability.DecisionOptions()) != 0 || len(resolved.all()) != events {
		t.Fatalf("closed=%+v events=%d→%d", closed.Case, events, len(resolved.all()))
	}
	// 以沒有通報關閉的案件不是終局的決定：同一家店、同一個類別的新通報開新案件。
	again := f.reportBy(t, s.StoreID, policy.Fraud)
	reopened := f.mustTriage(t, again.ID, policy.Fraud, severityPtr(SeverityCritical))
	if *reopened.CaseID == c1.ID || f.caseOf(t, *reopened.CaseID).State() != CaseStateOpen {
		t.Fatalf("reopened=%+v", reopened)
	}
	f.assertCaseInvariants(t)
	_ = ctx
}

// 移動的前置條件：已有結果、不在進行中的案件、是認定的來源、案件有導出的措施而它是最後一筆，都是 failed_precondition，而且什麼都不寫。
func TestMoveIsBlockedByTheReportsStateItsFindingsAndTheCasesMeasures(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	assertUnchanged := func(name string, before AdminReport, err error, sentence string) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s：應被擋下", name)
		}
		wantBlocked(t, err, sentence)
		if after := f.reportOf(t, before.ID); after.Version != before.Version || !sameUUIDPtr(after.CaseID, before.CaseID) {
			t.Fatalf("%s：被擋下的移動不得寫入：%+v → %+v", name, before.Report, after.Report)
		}
	}
	// 還沒分案（沒有案件）。
	awaiting := f.reportOf(t, f.reportBy(t, s.StoreID, policy.Fraud).ID)
	_, err := f.move(t, awaiting.ID, policy.MisleadingInfo, severityPtr(SeverityLow))
	assertUnchanged("待分案", awaiting, err, "只有在進行中的案件裡、還沒有結果的通報可以移動。")

	// 已有結果（案件裡先個別設定結果）。
	_, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
	f.mustResolve(t, members[0].ID, OutcomeInsufficient)
	_, err = f.move(t, members[0].ID, policy.MisleadingInfo, severityPtr(SeverityLow))
	assertUnchanged("已有結果", f.reportOf(t, members[0].ID), err, "這筆通報已經有結果。")

	// 是認定的來源：認定來源的通報不能移到其他案件。
	_, sourced := f.newCase(t, s.StoreID, policy.Counterfeit, SeverityLow, 2)
	f.warningCiting(t, s.StoreID, sourced[0].ID)
	_, err = f.move(t, sourced[0].ID, policy.Fraud, nil)
	assertUnchanged("認定的來源", f.reportOf(t, sourced[0].ID), err, "這筆通報已是認定的依據來源，不能移到其他案件。")

	// 案件有導出的措施，而它是最後一筆；案件還有別的成員時可以移走（措施已撤銷也一樣：做過就是導出的）。
	c, solo := f.newCase(t, s.StoreID, policy.IPInfringement, SeverityLow, 1)
	m := f.riskFromCase(t, s.StoreID, c.ID)
	_, err = f.move(t, solo[0].ID, policy.Fraud, nil)
	assertUnchanged("最後一筆而案件有導出的措施", f.reportOf(t, solo[0].ID), err, "這個案件已有導出的措施，不能把最後一筆通報移走。")
	f.revoke(t, m)
	_, err = f.move(t, solo[0].ID, policy.Fraud, nil)
	assertUnchanged("措施已撤銷也算", f.reportOf(t, solo[0].ID), err, "這個案件已有導出的措施，不能把最後一筆通報移走。")

	f.assertCaseInvariants(t)
}

func sameUUIDPtr(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// 決定即結案：結案時還沒有結果的通報帶入決定的預設結果與說明，每筆各發一個 ReportResolved；已經有結果的不受影響；結案是終態。
func TestDecideClosesTheCaseAndCarriesTheDefaultOutcomeIntoUnresolvedReports(t *testing.T) {
	t.Parallel()
	f := setup(t)
	resolved := record[ReportResolved](f)
	_, s := f.open(t)
	c, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityHigh, 3)
	f.riskFromCase(t, s.StoreID, c.ID) // 採取措施要有導出的措施
	early := f.resolveWithNote(t, members[0].ID, OutcomeInsufficient, "資料不足")
	events := len(resolved.all())

	if _, err := f.module.DecideCase(context.Background(), DecideCaseInput{CaseID: c.ID, AdminID: 10, Decision: DecisionActionTaken, ExpectedVersion: 1}); err == nil {
		t.Fatal("過期的版本")
	} else {
		wantKind(t, err, apperr.KindVersionConflict)
	}
	for name, in := range map[string]DecideCaseInput{
		"沒選決定": {Decision: ""}, "決定不認得": {Decision: "REOPEN"}, "說明太長": {Decision: DecisionActionTaken, Note: strings.Repeat("字", 2001)},
	} {
		in.CaseID, in.AdminID, in.ExpectedVersion = c.ID, 10, f.caseOf(t, c.ID).Version
		_, err := f.module.DecideCase(context.Background(), in)
		if len(invalidFields(t, err)) != 1 {
			t.Errorf("%s：%v", name, err)
		}
	}
	f.now = f.now.Add(time.Hour)
	before := f.caseOf(t, c.ID)
	decided, err := f.decide(t, c.ID, DecisionActionTaken, "  已請賣家處理  ")
	must(t, err)
	if decided.State() != CaseStateClosed || decided.CloseReason == nil || *decided.CloseReason != CaseClosedDecided || decided.Decision == nil || *decided.Decision != DecisionActionTaken ||
		decided.DecisionNote == nil || *decided.DecisionNote != "已請賣家處理" || *decided.ClosedByAdminID != 10 || !decided.ClosedAt.Equal(f.now) ||
		decided.Version != before.Version+1 || decided.UnresolvedReportCount != 0 || len(decided.Availability.DecisionOptions()) != 0 {
		t.Fatalf("decided=%+v", decided.Case)
	}
	// 沒有結果的兩筆帶入已採取措施與說明（版本各加一）；個別先設定的那筆不變。
	for _, m := range members[1:] {
		r := f.reportOf(t, m.ID)
		if r.Outcome == nil || *r.Outcome != OutcomeActionTaken || r.ResolutionNote == nil || *r.ResolutionNote != "已請賣家處理" || *r.ResolvedByAdminID != 10 ||
			!r.ResolvedAt.Equal(f.now) || r.Version != m.Version+1 {
			t.Fatalf("帶入的通報：%+v", r.Report)
		}
	}
	if got := f.reportOf(t, members[0].ID); *got.Outcome != OutcomeInsufficient || *got.ResolutionNote != "資料不足" || got.Version != early.Version {
		t.Fatalf("先個別設定結果的通報不受影響：%+v", got.Report)
	}
	got := resolved.all()[events:]
	if len(got) != 2 || got[0].Outcome != OutcomeActionTaken || got[0].CaseID == nil || *got[0].CaseID != c.ID {
		t.Fatalf("每筆帶入結果的通報各發一個 ReportResolved：%+v", got)
	}
	// 結案是終態：不能再決定、不能個別設定結果、不能分案或移動。
	_, err = f.decide(t, c.ID, DecisionNoAction, "")
	wantBlocked(t, err, "案件已結案。")
	_, err = f.resolve(t, members[1].ID, OutcomeAbusive, "")
	wantBlocked(t, err, "這筆通報已經有結果。")
	f.assertCaseInvariants(t)
}

func (f *fixture) resolveWithNote(t *testing.T, reportID uuid.UUID, outcome Outcome, note string) AdminReport {
	t.Helper()
	r, err := f.resolve(t, reportID, outcome, note)
	must(t, err)
	return r
}

// 決定與導出的措施一致（Q1）：「採取措施」要有導出的措施（不論狀態）或已有成員的結果是已採取措施；「不採取措施」要沒有未撤銷的導出措施，也沒有成員的結果是已採取措施。
// 審查 B1 的情境（措施 → 成員設為已採取措施 → 措施被撤銷）不會讓案件卡住：每個進行中的案件至少有一個可以選的決定。
func TestDecideFollowsTheDerivedMeasuresAndNeverLeavesACaseWithoutAChoice(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	noBasis := "還沒有從這個案件建立的措施，也沒有通報的結果是已採取措施，不能決定採取措施。"
	stillEnforced := "從這個案件建立的措施還沒有撤銷，不能決定不採取措施；請先撤銷措施。"
	actionTakenMember := "案件裡有通報的結果是已採取措施，不能決定不採取措施。"
	options := func(c uuid.UUID) []Decision { return f.caseOf(t, c).Availability.DecisionOptions() }

	// 沒有導出的措施、沒有已採取措施的成員：只能不採取措施。
	c1, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
	if got := options(c1.ID); !slices.Equal(got, []Decision{DecisionNoAction}) {
		t.Fatalf("沒有措施：%v", got)
	}
	_, err := f.decide(t, c1.ID, DecisionActionTaken, "")
	wantBlocked(t, err, noBasis)

	// 有未撤銷的導出措施：只能採取措施；撤銷之後兩個都可以（做過就是採取過措施，撤銷是之後的更正）。
	c2, _ := f.newCase(t, s.StoreID, policy.Counterfeit, SeverityLow, 2)
	m := f.riskFromCase(t, s.StoreID, c2.ID)
	if got := options(c2.ID); !slices.Equal(got, []Decision{DecisionActionTaken}) {
		t.Fatalf("有未撤銷的措施：%v", got)
	}
	_, err = f.decide(t, c2.ID, DecisionNoAction, "")
	wantBlocked(t, err, stillEnforced)
	f.revoke(t, m)
	if got := options(c2.ID); !slices.Equal(got, []Decision{DecisionActionTaken, DecisionNoAction}) {
		t.Fatalf("措施撤銷之後：%v", got)
	}

	// B1：措施 → 成員設為已採取措施 → 措施被撤銷：仍可以採取措施（措施做過、成員是已採取措施），不採取措施被成員擋下。
	c3, members := f.newCase(t, s.StoreID, policy.Privacy, SeverityLow, 2)
	m3 := f.riskFromCase(t, s.StoreID, c3.ID)
	f.mustResolve(t, members[0].ID, OutcomeActionTaken)
	f.revoke(t, m3)
	if got := options(c3.ID); !slices.Equal(got, []Decision{DecisionActionTaken}) {
		t.Fatalf("B1：%v", got)
	}
	_, err = f.decide(t, c3.ID, DecisionNoAction, "")
	wantBlocked(t, err, actionTakenMember)
	f.assertCaseInvariants(t)
	done, err := f.decide(t, c3.ID, DecisionActionTaken, "")
	must(t, err)
	if *done.Decision != DecisionActionTaken || f.reportOf(t, members[1].ID).Outcome == nil || *f.reportOf(t, members[1].ID).Outcome != OutcomeActionTaken {
		t.Fatalf("done=%+v", done.Case)
	}

	// 不採取措施：沒有結果的通報帶入未違規；案件結案。
	nobasis, err := f.decide(t, c1.ID, DecisionNoAction, "")
	must(t, err)
	if *nobasis.Decision != DecisionNoAction || nobasis.DecisionNote != nil {
		t.Fatalf("nobasis=%+v", nobasis.Case)
	}
	if r := f.reportOf(t, f.firstReportOf(t, c1.ID)); *r.Outcome != OutcomeNoViolation || r.ResolutionNote != nil {
		t.Fatalf("帶入的結果要是未違規：%+v", r.Report)
	}
	f.assertCaseInvariants(t)
}

func (f *fixture) firstReportOf(t *testing.T, caseID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT id FROM seller.reports WHERE case_id = $1 ORDER BY created_at, id LIMIT 1", caseID).Scan(&id))
	return id
}

// 改嚴重度必須附原因：原因寫成案件的一則內部註記，改前改後記在那則註記上；只限進行中、新的嚴重度必須不同。
func TestChangeCaseSeverityWritesANoteAndOnlyWhileOpen(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityMedium, 2)
	change := func(sev Severity, note string, version int64) (AdminCase, error) {
		return f.module.ChangeCaseSeverity(ctx, ChangeCaseSeverityInput{CaseID: c.ID, AdminID: 11, Severity: sev, Note: note, ExpectedVersion: version})
	}
	for name, in := range map[string]struct {
		sev  Severity
		note string
	}{
		"嚴重度不合法": {"EXTREME", "原因"}, "沒有原因": {SeverityHigh, "  "}, "原因太長": {SeverityHigh, strings.Repeat("字", 2001)}, "和目前相同": {SeverityMedium, "原因"},
	} {
		if _, err := change(in.sev, in.note, c.Version); len(invalidFields(t, err)) != 1 {
			t.Errorf("%s：%v", name, err)
		}
	}
	_, err := change(SeverityHigh, "原因", c.Version+4)
	wantKind(t, err, apperr.KindVersionConflict)
	if after := f.caseOf(t, c.ID); after.Version != c.Version || f.count(t, "SELECT count(*) FROM seller.governance_notes WHERE case_id = $1", c.ID) != 0 {
		t.Fatalf("被拒絕的修改寫入了：%+v", after.Case)
	}
	f.now = f.now.Add(time.Hour)
	changed, err := change(SeverityCritical, "  發現更多受害者  ", c.Version)
	must(t, err)
	if changed.Severity != SeverityCritical || changed.Version != c.Version+1 || !changed.UpdatedAt.Equal(f.now) {
		t.Fatalf("changed=%+v", changed.Case)
	}
	notes, err := f.module.CaseNotes(ctx, CaseNoteListInput{CaseID: c.ID, PageSize: 10})
	must(t, err)
	if len(notes.Items) != 1 || notes.Items[0].Body != "發現更多受害者" || notes.Items[0].AdminID != 11 || notes.Items[0].SeverityChange == nil ||
		*notes.Items[0].SeverityChange != (SeverityChange{From: SeverityMedium, To: SeverityCritical}) {
		t.Fatalf("notes=%+v", notes.Items)
	}
	// 註記上的改前改後只由這個指令寫：一般的註記沒有，DB 也擋下措施的註記或相同的改前改後。
	plain, err := f.module.CreateCaseNote(ctx, CreateCaseNoteInput{CaseID: c.ID, AdminID: 12, Body: "電話聯絡過賣家"})
	must(t, err)
	if plain.SeverityChange != nil {
		t.Fatalf("plain=%+v", plain)
	}
	for _, sql := range []string{
		"INSERT INTO seller.governance_notes (note_id, case_id, admin_id, body, severity_from, severity_to, created_at) VALUES (uuidv7(), $1, 1, 'x', 'LOW', 'LOW', now())",
		"INSERT INTO seller.governance_notes (note_id, case_id, admin_id, body, severity_from, created_at) VALUES (uuidv7(), $1, 1, 'x', 'LOW', now())",
		"INSERT INTO seller.governance_notes (note_id, case_id, admin_id, body, severity_from, severity_to, created_at) VALUES (uuidv7(), $1, 1, 'x', 'LOW', 'EXTREME', now())",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, sql, c.ID); err == nil {
			t.Fatalf("DB accepted: %s", sql)
		}
	}
	// 已結案的案件不能改嚴重度，仍可寫註記。
	f.riskFromCase(t, s.StoreID, c.ID)
	_, err = f.decide(t, c.ID, DecisionActionTaken, "")
	must(t, err)
	_, err = change(SeverityLow, "原因", f.caseOf(t, c.ID).Version)
	wantBlocked(t, err, "案件已結案。")
	if _, err := f.module.CreateCaseNote(ctx, CreateCaseNoteInput{CaseID: c.ID, AdminID: 12, Body: "結案之後的補記"}); err != nil {
		t.Fatalf("已結案的案件也可以寫註記：%v", err)
	}
	f.assertCaseInvariants(t)
}

// 案件註記：只新增、不帶 etag、不改案件的版本與 updated_at；案件不存在 NotFound；驗證；舊到新分頁。
func TestCaseNotesAreAppendOnlyAndDoNotBumpTheCase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
	var ids []uuid.UUID
	for i, body := range []string{"第一則", "  第二則  ", "第三則"} {
		f.now = f.now.Add(time.Minute)
		n, err := f.module.CreateCaseNote(ctx, CreateCaseNoteInput{CaseID: c.ID, AdminID: int64(20 + i), Body: body})
		must(t, err)
		if n.AdminID != int64(20+i) || n.Body != strings.TrimSpace(body) || !n.CreatedAt.Equal(f.now) {
			t.Fatalf("note=%+v", n)
		}
		ids = append(ids, n.ID)
	}
	for _, in := range []CreateCaseNoteInput{{CaseID: c.ID, AdminID: 1, Body: " "}, {CaseID: c.ID, AdminID: 0, Body: "x"}, {CaseID: c.ID, AdminID: 1, Body: strings.Repeat("字", 2001)}} {
		if _, err := f.module.CreateCaseNote(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%+v：%v", in, err)
		}
	}
	_, err := f.module.CreateCaseNote(ctx, CreateCaseNoteInput{CaseID: uuid.NewV7(), AdminID: 1, Body: "x"})
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.module.CaseNotes(ctx, CaseNoteListInput{CaseID: uuid.NewV7()})
	wantKind(t, err, apperr.KindNotFound)
	after := f.caseOf(t, c.ID)
	if after.Version != c.Version || !after.UpdatedAt.Equal(c.UpdatedAt) {
		t.Fatalf("寫註記不改案件的版本與 updated_at：%d → %d", c.Version, after.Version)
	}
	var got []uuid.UUID
	var cursor *NoteCursor
	for {
		page, err := f.module.CaseNotes(ctx, CaseNoteListInput{CaseID: c.ID, PageSize: 2, After: cursor})
		must(t, err)
		for _, n := range page.Items {
			got = append(got, n.ID)
		}
		if page.Next == nil {
			break
		}
		cursor = page.Next
	}
	if !slices.Equal(got, ids) {
		t.Fatalf("舊到新、不漏不重：%v，預期 %v", got, ids)
	}
	// 註記只能掛在案件或措施的其中一個。
	for _, sql := range []string{
		"INSERT INTO seller.governance_notes (note_id, admin_id, body, created_at) VALUES (uuidv7(), 1, 'x', now())",
		"INSERT INTO seller.governance_notes (note_id, case_id, measure_id, admin_id, body, created_at) SELECT uuidv7(), $1, id, 1, 'x', now() FROM seller.measures LIMIT 1",
	} {
		if tag, err := f.db.Conn(ctx).Exec(ctx, sql, c.ID); err == nil && tag.RowsAffected() > 0 {
			t.Fatalf("DB accepted: %s", sql)
		}
	}
}

// anotherStore 開另一個帳號的店。
func (f *fixture) anotherStore(t *testing.T) Store {
	t.Helper()
	_, s := f.open(t)
	return s
}

// 分案與移動的去處由伺服器輸出（設計審查 #1）：只有此刻能分案或能移動的通報帶 triage_targets；每個可判定的類別一筆（順序同政策分類），
// 這家店該類別有進行中的案件就帶它的 id、嚴重度與版本（加入），沒有就是開新案件；移動時不含目前的類別；別家店的案件不會出現。
func TestReportDetailListsTheTriageTargetsOfTheStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	_, other := f.open(t)
	c, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityHigh, 2)
	f.newCase(t, other.StoreID, policy.Privacy, SeverityLow, 1)
	awaiting := f.reportBy(t, s.StoreID, policy.Counterfeit)

	categories := policy.Assessable()
	targets := f.reportOf(t, awaiting.ID).Targets
	if len(targets) != len(categories) {
		t.Fatalf("待分案的通報：%d 個去處，預期 %d", len(targets), len(categories))
	}
	for i, target := range targets {
		if target.Category != categories[i] {
			t.Errorf("去處 %d 的類別 %s，預期 %s（同政策分類的順序）", i, target.Category, categories[i])
		}
		if joins := target.Category == policy.Fraud; joins != (target.Case != nil) {
			t.Errorf("%s：加入=%v，預期 %v（只有詐欺有進行中的案件；別家店的隱私案件不算）", target.Category, target.Case != nil, joins)
		}
		if target.Case != nil && (target.Case.ID != c.ID || target.Case.Severity != SeverityHigh || target.Case.Version != c.Version || target.Case.ReportCount != 2) {
			t.Errorf("加入的案件 %+v，預期 %s／%s／版本 %d／2 筆通報", target.Case, c.ID, SeverityHigh, c.Version)
		}
	}
	// 移動：案件裡的成員，去處不含目前的類別。
	moveTargets := f.reportOf(t, members[0].ID).Targets
	if len(moveTargets) != len(categories)-1 {
		t.Fatalf("案件成員：%d 個去處，預期 %d", len(moveTargets), len(categories)-1)
	}
	for _, target := range moveTargets {
		if target.Category == policy.Fraud || target.Case != nil {
			t.Errorf("移動的去處不含目前的類別、而且這家店沒有其他進行中的案件：%+v", target)
		}
	}
	// 不能分案也不能移動的（已有結果）沒有去處。
	f.mustResolve(t, members[1].ID, OutcomeNoViolation)
	if got := f.reportOf(t, members[1].ID).Targets; len(got) != 0 {
		t.Errorf("已有結果的通報不帶去處：%+v", got)
	}
	// 去處是意圖：照讀到的 triage_targets 加入就成功。
	fresh := f.reportOf(t, awaiting.ID)
	fraud := fresh.Targets[slices.Index(categories, policy.Fraud)].Case
	joined, err := f.module.TriageReport(context.Background(), TriageReportInput{
		ReportID: awaiting.ID, AdminID: 7, AssessedCategory: policy.Fraud, ExpectedVersion: fresh.Version,
		Target: CaseTarget{Join: &JoinCase{CaseID: fraud.ID, ExpectedVersion: fraud.Version}},
	})
	if err != nil || joined.CaseID == nil || *joined.CaseID != c.ID {
		t.Fatalf("joined=%+v err=%v", joined.Report, err)
	}
}

// 「有新的補充」記在通報上（設計審查 #8）：分案、移動記下「管理員看過這筆通報的補充」；設定結果的通報有了結果就不再提醒；對別的成員設定結果、改案件嚴重度不算；
// 案件的標示是「有成員的補充沒被看過」，所以對成員 A 設定結果不會把成員 B 還沒看過的補充標成看過；有了結果的通報不再提醒。
func TestSupplementsAreSeenOnlyWhenTheAdminActsOnThatReport(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 3)
	supplement := func(r AdminReport) {
		t.Helper()
		f.now = f.now.Add(time.Minute)
		got, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: *r.ReporterBuyerID, Category: r.Category, Description: "補充"})
		must(t, err)
		if !got.Supplemented {
			t.Fatalf("要併成補充：%+v", got)
		}
	}
	unseen := func(id uuid.UUID) bool { return f.reportOf(t, id).HasUnseenSupplements() }
	for _, m := range members {
		supplement(f.reportOf(t, m.ID))
	}
	if after := f.caseOf(t, c.ID); after.UnseenSupplementReports != 3 || !unseen(members[0].ID) || !unseen(members[1].ID) || !unseen(members[2].ID) {
		t.Fatalf("三個成員都有沒看過的補充：%+v", after.Case)
	}
	// 對成員 A 設定結果：A 的補充看過了（而且有了結果不再提醒），B、C 的還沒；案件仍提醒。這是舊做法（拿案件的 updated_at）會標錯的地方。
	f.resolveNextMinute(t, members[0].ID)
	if unseen(members[0].ID) || !unseen(members[1].ID) || !unseen(members[2].ID) {
		t.Fatalf("只有 A 看過：A=%v B=%v C=%v", unseen(members[0].ID), unseen(members[1].ID), unseen(members[2].ID))
	}
	if after := f.caseOf(t, c.ID); after.UnseenSupplementReports != 2 {
		t.Fatalf("案件還有 2 個成員的補充沒看過：%+v", after.Case)
	}
	// 改案件嚴重度之類的案件層級處理不記任何成員的補充：B、C 仍然沒看過。
	f.now = f.now.Add(time.Minute)
	_, err := f.module.ChangeCaseSeverity(ctx, ChangeCaseSeverityInput{CaseID: c.ID, AdminID: 10, Severity: SeverityMedium, Note: "改", ExpectedVersion: f.caseOf(t, c.ID).Version})
	must(t, err)
	if !unseen(members[1].ID) || !unseen(members[2].ID) || f.caseOf(t, c.ID).UnseenSupplementReports != 2 {
		t.Fatal("改嚴重度不記成員的補充已看過")
	}
	// 移動 B（帶著 B 的 etag）：B 的補充看過了；原案件還有 C。
	f.now = f.now.Add(time.Minute)
	_, err = f.move(t, members[1].ID, policy.Counterfeit, severityPtr(SeverityLow))
	must(t, err)
	if unseen(members[1].ID) || !unseen(members[2].ID) || f.caseOf(t, c.ID).UnseenSupplementReports != 1 {
		t.Fatal("移動記下補充已看過")
	}
	// 補充又來了（B 在新案件裡再補充一次）：晚於看過的時間，又是沒看過。
	supplement(f.reportOf(t, members[1].ID))
	if !unseen(members[1].ID) {
		t.Fatal("看過之後新的補充又是沒看過")
	}
	// 結案帶入預設結果的成員（C）不再提醒；案件結案後沒有未處理的成員。
	_, err = f.decide(t, c.ID, DecisionNoAction, "")
	must(t, err)
	if unseen(members[2].ID) || f.caseOf(t, c.ID).UnseenSupplementReports != 0 {
		t.Fatalf("結案之後不再提醒：C=%v case=%+v", unseen(members[2].ID), f.caseOf(t, c.ID).Case)
	}
	// 待分案的通報有補充：分案記下已看過。
	awaiting := f.reportBy(t, s.StoreID, policy.Privacy)
	supplement(f.reportOf(t, awaiting.ID))
	if !unseen(awaiting.ID) {
		t.Fatal("待分案的通報有沒看過的補充")
	}
	f.now = f.now.Add(time.Minute)
	f.mustTriage(t, awaiting.ID, policy.Privacy, severityPtr(SeverityLow))
	if unseen(awaiting.ID) {
		t.Fatal("分案記下補充已看過")
	}
}

// 明確的已讀（MarkReportSupplementsSeen）：帶讀到補充時的 etag，只記下這筆通報的補充已看過，**不讓通報或案件的版本前進**；冪等；etag 過時（讀到之後又有新的補充）
// 回版本衝突、不記；有了結果的通報不寫入。版本衝突的 metadata 說是通報還是案件。
func TestMarkReportSupplementsSeenRecordsOnlyTheMarkerAndNeverAdvancesTheVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
	supplement := func(r AdminReport) {
		t.Helper()
		f.now = f.now.Add(time.Minute)
		got, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: *r.ReporterBuyerID, Category: r.Category, Description: "補充"})
		must(t, err)
		if !got.Supplemented {
			t.Fatalf("要併成補充：%+v", got)
		}
	}
	mark := func(r AdminReport) (AdminReport, error) {
		return f.module.MarkReportSupplementsSeen(ctx, MarkSupplementsSeenInput{ReportID: r.ID, ExpectedVersion: r.Version})
	}
	supplement(f.reportOf(t, members[0].ID))
	supplement(f.reportOf(t, members[1].ID))
	a, caseBefore := f.reportOf(t, members[0].ID), f.caseOf(t, c.ID)
	if !a.HasUnseenSupplements() || caseBefore.UnseenSupplementReports != 2 {
		t.Fatalf("前提：兩筆都有沒看過的補充：%+v", caseBefore.Case)
	}
	f.now = f.now.Add(time.Minute)
	marked, err := mark(a)
	must(t, err)
	if marked.HasUnseenSupplements() || marked.Version != a.Version || !marked.UpdatedAt.Equal(a.UpdatedAt) {
		t.Fatalf("記下已看過，版本與更新時間不變：before=%+v after=%+v", a.Report, marked.Report)
	}
	if got := f.caseOf(t, c.ID); got.Version != caseBefore.Version || got.UnseenSupplementReports != 1 || !f.reportOf(t, members[1].ID).HasUnseenSupplements() {
		t.Fatalf("案件的版本不變、只少一筆沒看過：%+v", got.Case)
	}
	// 冪等：再記一次成功，已看過的編號不動。
	f.now = f.now.Add(time.Minute)
	again, err := mark(f.reportOf(t, a.ID))
	must(t, err)
	if again.SupplementsSeenSeq != marked.SupplementsSeenSeq || marked.SupplementsSeenSeq != 1 || again.Version != a.Version {
		t.Fatalf("重複記下不改任何東西：%+v", again.Report)
	}
	// 讀到之後又有新的補充：舊 etag 的已讀被拒（那則補充還沒被看到），標記不動；用新 etag 才記得下。
	b := f.reportOf(t, members[1].ID)
	supplement(b)
	_, err = mark(b)
	wantKind(t, err, apperr.KindVersionConflict)
	wantConflictTarget(t, err, "report")
	if !f.reportOf(t, b.ID).HasUnseenSupplements() {
		t.Fatal("etag 過時時不記已看過")
	}
	fresh, err := mark(f.reportOf(t, b.ID))
	must(t, err)
	if fresh.HasUnseenSupplements() || f.caseOf(t, c.ID).UnseenSupplementReports != 0 {
		t.Fatalf("新 etag 記得下：%+v", fresh.Report)
	}
	// 有了結果的通報不寫入（也不報錯）。
	resolved := f.mustResolve(t, b.ID, OutcomeNoViolation)
	got, err := mark(resolved)
	must(t, err)
	if got.Version != resolved.Version || got.SupplementsSeenSeq != fresh.SupplementsSeenSeq || fresh.SupplementsSeenSeq != 2 {
		t.Fatalf("有結果的通報不寫入：%+v", got.Report)
	}
	// 通報的版本衝突與案件的版本衝突在 metadata 裡分得出來。
	_, err = f.module.ResolveReport(ctx, ResolveReportInput{ReportID: a.ID, AdminID: 8, Outcome: OutcomeInsufficient, ExpectedVersion: a.Version - 1})
	wantConflictTarget(t, err, "report")
	_, err = f.module.DecideCase(ctx, DecideCaseInput{CaseID: c.ID, AdminID: 8, Decision: DecisionNoAction, ExpectedVersion: caseBefore.Version})
	wantConflictTarget(t, err, "case")
	_, err = f.module.MarkReportSupplementsSeen(ctx, MarkSupplementsSeenInput{ReportID: uuid.NewV7(), ExpectedVersion: 1})
	wantKind(t, err, apperr.KindNotFound)
}

// wantConflictTarget 確認錯誤是版本衝突，而且 ErrorInfo.metadata 的 conflictTarget 是 target。
func wantConflictTarget(t *testing.T, err error, target string) {
	t.Helper()
	wantKind(t, err, apperr.KindVersionConflict)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Metadata["conflictTarget"] != target {
		t.Fatalf("conflictTarget=%v，預期 %s", ae, target)
	}
}

// resolveNextMinute 在下一分鐘對通報設定結果（未違規）。
func (f *fixture) resolveNextMinute(t *testing.T, id uuid.UUID) {
	t.Helper()
	f.now = f.now.Add(time.Minute)
	f.mustResolve(t, id, OutcomeNoViolation)
}

// 「有新的補充」不依賴時鐘（設計審查 #8 的最後一輪）：已看過的標記是補充的編號，不是時間。補充晚到的時鐘比標記記下時還早（多實例的時鐘不同步）、
// 或與標記同一個時間，都仍然是沒看過；看過之後記下的編號等於當時最新一則。
func TestUnseenSupplementsDoNotDependOnTheClock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
	supplement := func() {
		t.Helper()
		r := f.reportOf(t, members[0].ID)
		got, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: *r.ReporterBuyerID, Category: r.Category, Description: "補充"})
		must(t, err)
		if !got.Supplemented {
			t.Fatalf("要併成補充：%+v", got)
		}
	}
	mark := func() AdminReport {
		t.Helper()
		r := f.reportOf(t, members[0].ID)
		got, err := f.module.MarkReportSupplementsSeen(ctx, MarkSupplementsSeenInput{ReportID: r.ID, ExpectedVersion: r.Version})
		must(t, err)
		return got
	}
	f.now = f.now.Add(time.Hour)
	supplement()
	if got := mark(); got.HasUnseenSupplements() || got.SupplementsSeenSeq != 1 {
		t.Fatalf("看過第 1 則：%+v", got.Report)
	}
	// 第 2 則補充的時鐘比標記記下時早 2 小時：時間比較會判成「早於看過的時間」而永遠不提醒。
	f.now = f.now.Add(-2 * time.Hour)
	supplement()
	if !f.reportOf(t, members[0].ID).HasUnseenSupplements() {
		t.Fatal("時鐘倒退的補充仍是沒看過")
	}
	if got := mark(); got.HasUnseenSupplements() || got.SupplementsSeenSeq != 2 {
		t.Fatalf("看過第 2 則：%+v", got.Report)
	}
	// 與標記同一個時間（微秒相同）的補充也是沒看過。
	supplement()
	if got := f.reportOf(t, members[0].ID); !got.HasUnseenSupplements() || got.SupplementsSeenSeq != 2 || got.SupplementCount != 3 {
		t.Fatalf("同一時間的補充仍是沒看過：%+v", got.Report)
	}
}

// Go 的「有新的補充」（Report.HasUnseenSupplements）與 SQL（案件的 unseen_supplement_reports、MarkReportSupplementsSeen）是同一個定義：比最新一則的編號，
// 不比則數。DB 只保證編號 1..10、每筆通報內唯一，不保證連號，所以直接寫入不連號的補充（2、5）：則數 2 小於看過的編號 3，比則數會判成「看過了」。
func TestUnseenSupplementsCompareTheLatestSeqNotTheCount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
	id := members[0].ID
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := f.db.Conn(ctx).Exec(ctx, sql, args...)
		must(t, err)
	}
	supplement := func(seq int) {
		t.Helper()
		exec(`INSERT INTO seller.report_supplements (id, report_id, description, seq, created_at) VALUES ($1, $2, '補充', $3, $4)`, uuid.NewV7(), id, seq, f.now)
	}
	agree := func(name string, want bool) {
		t.Helper()
		r := f.reportOf(t, id)
		n := f.caseOf(t, c.ID).UnseenSupplementReports
		if r.HasUnseenSupplements() != want || (n == 1) != want {
			t.Fatalf("%s：Go=%v SQL=%d，預期 %v（則數 %d、最新編號 %d、看過 %d）", name, r.HasUnseenSupplements(), n, want, r.SupplementCount, r.LatestSupplementSeq, r.SupplementsSeenSeq)
		}
	}
	supplement(2)
	supplement(5)
	exec(`UPDATE seller.reports SET supplements_seen_seq = 3 WHERE id = $1`, id)
	agree("最新編號 5 大於看過的 3（則數只有 2）", true)
	r := f.reportOf(t, id)
	got, err := f.module.MarkReportSupplementsSeen(ctx, MarkSupplementsSeenInput{ReportID: id, ExpectedVersion: r.Version})
	must(t, err)
	if got.SupplementsSeenSeq != 5 || got.LatestSupplementSeq != 5 || got.SupplementCount != 2 {
		t.Fatalf("看過之後記下最新一則的編號：%+v", got.Report)
	}
	agree("看過編號 5", false)
	supplement(7)
	agree("編號 7 還沒看過", true)
}
