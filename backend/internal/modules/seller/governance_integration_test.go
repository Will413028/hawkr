//go:build integration

package seller_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// recorded 記下某一種事件；concurrent 的測試也安全。
type recorded[E any] struct {
	mu    sync.Mutex
	items []E
}

func record[E any](f *fixture) *recorded[E] {
	r := &recorded[E]{}
	events.Subscribe(f.bus, func(_ context.Context, e E) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.items = append(r.items, e)
		return nil
	})
	return r
}

func (r *recorded[E]) all() []E {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.items)
}

// everExposed 記下店家曾經公開過（first_exposed_at），讓它可以被通報：通報的對象只能是曾經公開過的店。
func (f *fixture) everExposed(t *testing.T, storeID int64) {
	t.Helper()
	_, err := f.db.Conn(context.Background()).Exec(context.Background(),
		"UPDATE seller.stores SET first_exposed_at = COALESCE(first_exposed_at, $2) WHERE store_id = $1", storeID, f.now)
	must(t, err)
}

func (f *fixture) buyerReport(t *testing.T, storeID, buyerID int64, category policy.Category) SubmitReportResult {
	t.Helper()
	f.everExposed(t, storeID)
	r, err := f.module.SubmitReport(context.Background(), SubmitReportInput{
		StoreID: storeID, ReporterBuyerID: buyerID, Category: category, Description: "商品標示不實",
	})
	must(t, err)
	return r
}

func (f *fixture) anonymousReport(t *testing.T, storeID int64, phone string) SubmitReportResult {
	t.Helper()
	f.everExposed(t, storeID)
	r, err := f.module.SubmitReport(context.Background(), SubmitReportInput{
		StoreID: storeID, Phone: phone, Name: "王小明", Category: policy.FoodSafety, Description: "食用後腹瀉",
	})
	must(t, err)
	return r
}

// violate 以新的認定建立一個依據認定的措施：penalty 是 WARNING 或 SUSPENSION，停權帶結束時間。
func (f *fixture) violate(t *testing.T, storeID int64, severity Severity, kind MeasureKind, endsAt *time.Time) Measure {
	t.Helper()
	r, err := f.module.CreateMeasure(context.Background(), CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: kind, StatementToSeller: "商品頁宣稱有機但沒有認證", Note: "內部紀錄", EndsAt: endsAt,
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.MisleadingInfo, Severity: severity}},
	})
	must(t, err)
	return r.Measure
}

// riskSuspend 建立沒有固定期間的 RISK 停權（複審期限七天後）。
func (f *fixture) riskSuspend(t *testing.T, storeID int64) Measure {
	t.Helper()
	review := f.now.Add(7 * 24 * time.Hour)
	r, err := f.module.CreateMeasure(context.Background(), CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: MeasureSuspension, StatementToSeller: "疑似詐欺", Note: "內部紀錄", ReviewBy: &review,
		Risk: &RiskBasis{Ground: RiskFraud},
	})
	must(t, err)
	return r.Measure
}

// endRisk 結束 RISK 措施（風險已排除）。
func (f *fixture) endRisk(t *testing.T, m Measure) Measure {
	t.Helper()
	f.afterCreation(m)
	ended, err := f.module.EndMeasure(context.Background(), EndMeasureInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: m.Version})
	must(t, err)
	return ended
}

// afterCreation 讓 fixture 的時鐘晚於措施的建立時間：結束時間一定晚於建立時間（measures_ends_check），而凍結的時鐘會讓兩者相等。
func (f *fixture) afterCreation(m Measure) {
	if !f.now.After(m.CreatedAt) {
		f.now = m.CreatedAt.Add(time.Microsecond)
	}
}

// measureOf 讀 admin 看到的措施。
func (f *fixture) measureOf(t *testing.T, id uuid.UUID) Measure {
	t.Helper()
	m, err := f.module.Measure(context.Background(), id)
	must(t, err)
	return m
}

// suspend 建立沒有固定期間的 RISK 停權（riskSuspend 的簡寫，給只需要「帳號被停權」的測試）。
func (f *fixture) suspend(t *testing.T, storeID int64) Measure { return f.riskSuspend(t, storeID) }

// lift 結束 RISK 措施（以措施目前的後台版本）：帳號恢復。
func (f *fixture) lift(ctx context.Context, m Measure) (Measure, error) {
	current, err := f.module.Measure(ctx, m.ID)
	if err != nil {
		return Measure{}, err
	}
	f.afterCreation(current)
	return f.module.EndMeasure(ctx, EndMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: current.Version})
}

// activeRisk 回傳店家所屬帳號此刻生效中的那個措施。
func (f *fixture) activeRisk(t *testing.T, storeID int64) Measure {
	t.Helper()
	s, err := f.module.Store(context.Background(), storeID)
	must(t, err)
	if len(s.EffectiveMeasures()) != 1 {
		t.Fatalf("effective measures=%+v", s.EffectiveMeasures())
	}
	return f.measureOf(t, s.EffectiveMeasures()[0].MeasureID)
}

// insertEndedRisk 直接寫一筆已結束的 RISK 措施（結束時間在過去），withSnapshot 時帶一列身分快照：模擬結束 job 遺失、帳號沒有任何
// 期間可掃的情形。
func (f *fixture) insertEndedRisk(t *testing.T, storeID, accountID int64, withSnapshot bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewV7()
	created := f.now.Add(-time.Hour)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		conn := f.db.Conn(ctx)
		if _, err := conn.Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, ends_at, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
			VALUES ($1, $2, $3, 'SUSPENSION', 'RISK', 'FRAUD', $4, 'x', $6, 1, 1, $5, $5, $5)`, id, accountID, storeID, f.now.Add(7*24*time.Hour), created, f.now.Add(-time.Minute)); err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, `INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, created_at) VALUES ($1, 1, 'x', $2, 1, $3)`,
			id, f.now.Add(-time.Minute), created); err != nil {
			return err
		}
		if withSnapshot {
			_, err := conn.Exec(ctx, `INSERT INTO seller.measure_identities (measure_id, kind, key_id, hash) VALUES ($1, 'ID_CARD', 'x', decode(repeat('00', 32), 'hex'))`, id)
			return err
		}
		return nil
	}))
	return id
}

// isSuspended 回報店家所屬帳號此刻是否停權（讀店家時的推導）。
func (f *fixture) isSuspended(t *testing.T, storeID int64) bool {
	t.Helper()
	s, err := f.module.Store(context.Background(), storeID)
	must(t, err)
	return s.Suspended()
}

func TestSubmitReportValidatesTheReporter(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.everExposed(t, s.StoreID)
	for _, in := range []SubmitReportInput{
		{StoreID: s.StoreID, Category: policy.Fraud, Description: "沒有聯絡方式"},
		{StoreID: s.StoreID, ReporterBuyerID: 5, Phone: "0912345678", Category: policy.Fraud, Description: "兩種通報人"},
		{StoreID: s.StoreID, Phone: "12345", Category: policy.Fraud, Description: "電話錯誤"},
		{StoreID: s.StoreID, Email: "not-an-email", Category: policy.Fraud, Description: "email 錯誤"},
		{StoreID: s.StoreID, Phone: "0912345678", Category: policy.FoodSafety, Description: "匿名附圖", EvidenceTokens: []string{"t"}},
		{StoreID: s.StoreID, ReporterBuyerID: 5, Category: "SPAM", Description: "類別"},
		{StoreID: s.StoreID, ReporterBuyerID: 5, Category: policy.Fraud, Description: "  "},
		{StoreID: s.StoreID, ReporterBuyerID: 5, Category: policy.Fraud, Description: "太多佐證", AccountID: 1, EvidenceTokens: []string{"1", "2", "3", "4", "5", "6"}},
	} {
		if _, err := f.module.SubmitReport(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%+v: %v", in, err)
		}
	}
	if _, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID + 1000, ReporterBuyerID: 5, Category: policy.Fraud, Description: "x"}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("unknown store: %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.reports"); n != 0 {
		t.Fatalf("rejected reports were written: %d", n)
	}
	anonymous, err := f.module.SubmitReport(ctx, SubmitReportInput{
		StoreID: s.StoreID, Phone: "+886 912-345-678", Email: " Victim@Example.com ", Category: policy.FoodSafety, Description: "腹瀉",
	})
	must(t, err)
	c := anonymous.Report.Contact
	if anonymous.Supplemented || c == nil || *c.Phone != "0912345678" || *c.Email != "victim@example.com" || c.Name != nil ||
		anonymous.Report.ReporterBuyerID != nil || anonymous.Report.Outcome != nil || anonymous.Report.State() != ReportStateAwaitingTriage || anonymous.Report.Version != 1 {
		t.Fatalf("anonymous=%+v contact=%+v", anonymous, c)
	}
	// 分案期限來自 SQL 的審閱政策（seller.review_report_triage）：通報人選了食安 4 小時，其他類別 72 小時，開始是建立時間。
	read, err := f.module.Report(ctx, anonymous.Report.ID)
	must(t, err)
	if d := read.TriageDeadline; d == nil || !d.Start.Equal(f.now) || !d.Due.Equal(f.now.Add(4*time.Hour)) {
		t.Fatalf("food-safety triage deadline=%+v", d)
	}
	loggedIn := f.buyerReport(t, s.StoreID, 5, policy.Fraud)
	if loggedIn.Report.Contact != nil || *loggedIn.Report.ReporterBuyerID != 5 {
		t.Fatalf("logged-in=%+v", loggedIn.Report)
	}
	readLoggedIn, err := f.module.Report(ctx, loggedIn.Report.ID)
	must(t, err)
	if d := readLoggedIn.TriageDeadline; d == nil || !d.Start.Equal(f.now) || !d.Due.Equal(f.now.Add(72*time.Hour)) {
		t.Fatalf("other-category triage deadline=%+v", d)
	}
}

// 通報的類別只接受檢舉人可選的子集（platform/policy）：全部除了 PLATFORM_ABUSE。空字串、已經改名的舊值與不認得的值都是
// invalid_argument，而且什麼都不寫入。清單的類別篩選是「生效的類別」：可以是全部 13 個，不認得的值 invalid_argument。
func TestReportCategoryIsLimitedToWhatAReporterCanPick(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.everExposed(t, s.StoreID)
	rejected := []policy.Category{policy.PlatformAbuse, "", "UNAUTHORIZED_SALE", policy.Spam, policy.Harassment, policy.Hate}
	for _, c := range rejected {
		_, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: 5, Category: c, Description: "x"})
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("類別 %q：%v，預期 invalid_argument", c, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.reports"); n != 0 {
		t.Fatalf("被拒絕的通報寫入了 %d 筆", n)
	}
	accepted := 0
	for _, c := range policy.Values() {
		if slices.Contains(rejected, c) {
			continue
		}
		if _, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: 5, Category: c, Description: "x"}); err != nil {
			t.Errorf("類別 %q 檢舉人可以選：%v", c, err)
		}
		accepted++
	}
	if accepted != 12 {
		t.Fatalf("檢舉人可選的類別 %d 個，預期 15 個", accepted)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.reports"); n != 12 {
		t.Fatalf("寫入了 %d 筆通報，預期 12 筆", n)
	}
	for _, c := range []policy.Category{"", "UNAUTHORIZED_SALE", policy.Spam, policy.Harassment, policy.Hate} {
		if _, err := f.module.AdminReports(ctx, ReportListInput{Category: &c, PageSize: 10}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("清單篩選的類別 %q：%v，預期 invalid_argument", c, err)
		}
	}
	// 生效的類別：待分案的是檢舉人選的類別；分案之後是平台判定類別（可以是檢舉人不能選的 PLATFORM_ABUSE）。
	other := policy.Other
	page, err := f.module.AdminReports(ctx, ReportListInput{Category: &other, PageSize: 10})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].Category != policy.Other {
		t.Fatalf("篩選 OTHER 的結果：%+v", page.Items)
	}
	f.mustTriage(t, page.Items[0].ID, policy.PlatformAbuse, severityPtr(SeverityLow))
	abuse := policy.PlatformAbuse
	page, err = f.module.AdminReports(ctx, ReportListInput{Category: &abuse, PageSize: 10})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].Category != policy.Other || *page.Items[0].AssessedCategory != policy.PlatformAbuse {
		t.Fatalf("篩選生效的類別 PLATFORM_ABUSE：%+v", page.Items)
	}
	if page, err := f.module.AdminReports(ctx, ReportListInput{Category: &other, PageSize: 10}); err != nil || len(page.Items) != 0 {
		t.Fatalf("分案之後 OTHER 不再是生效的類別：%+v err=%v", page.Items, err)
	}
}

// 不能檢舉自己的店：登入的檢舉人與匿名入口的登入呼叫者都擋（invalid_argument 指到 StoreID）；從沒公開過的店仍是 not_found（不洩漏店家是不是那個帳號的）；
// 別的帳號可以檢舉。
func TestAReporterCannotReportTheirOwnStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	owner, s := f.open(t)
	_, never := f.open(t)
	f.everExposed(t, s.StoreID)
	loggedIn := SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: 5, AccountID: owner, Category: policy.Fraud, Description: "自己的店"}
	anonymousCaller := SubmitReportInput{StoreID: s.StoreID, Phone: "0912345678", AccountID: owner, Category: policy.FoodSafety, Description: "自己的店"}
	for name, in := range map[string]SubmitReportInput{"登入的檢舉": loggedIn, "匿名入口的登入呼叫者": anonymousCaller} {
		_, err := f.module.SubmitReport(ctx, in)
		if got := invalidFields(t, err); !slices.Equal(got, []string{"StoreID"}) || !strings.Contains(err.Error(), "不能檢舉自己的店。") {
			t.Errorf("%s：%v", name, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.reports"); n != 0 || len(f.media.finalized) != 0 {
		t.Fatalf("被擋下的檢舉寫入了 %d 筆、轉正了 %d 張圖", n, len(f.media.finalized))
	}
	neverOwn := loggedIn
	neverOwn.StoreID, neverOwn.AccountID = never.StoreID, f.mustAccountOf(t, never.StoreID)
	_, err := f.module.SubmitReport(ctx, neverOwn)
	wantKind(t, err, apperr.KindNotFound)
	other := loggedIn
	other.AccountID = owner + 1000
	if _, err := f.module.SubmitReport(ctx, other); err != nil {
		t.Fatalf("別的帳號可以檢舉：%v", err)
	}
	anonymous := anonymousCaller
	anonymous.AccountID = 0
	if _, err := f.module.SubmitReport(ctx, anonymous); err != nil {
		t.Fatalf("沒登入的匿名送出沒有帳號可比對：%v", err)
	}
}

func (f *fixture) mustAccountOf(t *testing.T, storeID int64) int64 {
	t.Helper()
	var id int64
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT account_id FROM seller.stores WHERE store_id = $1", storeID).Scan(&id))
	return id
}

// 去重的日與週以台北時間為界（盤點：舊的是 UTC）。台北 23:30 與隔天 00:30 在 UTC 是同一天；台北週日 23:30 與週一
// 00:30 在 UTC 是同一週。去重期間內、還沒有結果的那一筆存在時，這次的內容補充到它（Supplemented），不是新的通報。
func TestReportDedupUsesTaipeiDayAndWeek(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	f.everExposed(t, s.StoreID)
	f.now = time.Date(2026, 10, 1, 15, 30, 0, 0, time.UTC) // 台北 10/1（四）23:30
	first := f.buyerReport(t, s.StoreID, 5, policy.Fraud)
	again := f.buyerReport(t, s.StoreID, 5, policy.Fraud)
	if !again.Supplemented || again.ReportID != first.ReportID || again.Report == nil || again.Report.ID != first.ReportID {
		t.Fatalf("same Taipei day must be a supplement of the first report (logged-in callers get the original back): first=%v again=%+v", first.ReportID, again)
	}
	if other := f.buyerReport(t, s.StoreID, 5, policy.Counterfeit); other.Supplemented {
		t.Fatal("another category is not a supplement")
	}
	f.now = time.Date(2026, 10, 1, 16, 30, 0, 0, time.UTC) // 台北 10/2（五）00:30，UTC 仍是 10/1
	if next := f.buyerReport(t, s.StoreID, 5, policy.Fraud); next.Supplemented || next.ReportID == first.ReportID {
		t.Fatal("next Taipei day must be a new report")
	}

	f.now = time.Date(2026, 10, 4, 15, 30, 0, 0, time.UTC) // 台北 10/4（日）23:30
	weekly := f.anonymousReport(t, s.StoreID, "0911111111")
	f.now = time.Date(2026, 10, 4, 15, 45, 0, 0, time.UTC)
	if dup := f.anonymousReport(t, s.StoreID, "0911111111"); !dup.Supplemented || dup.ReportID != weekly.ReportID || dup.Report != nil {
		t.Fatalf("same Taipei week must be a supplement, and an anonymous caller gets nothing back: %+v", dup)
	}
	f.now = time.Date(2026, 10, 4, 16, 30, 0, 0, time.UTC) // 台北 10/5（一）00:30，UTC 仍是同一週
	if next := f.anonymousReport(t, s.StoreID, "0911111111"); next.Supplemented {
		t.Fatal("next Taipei ISO week must be a new report")
	}
	byEmail, err := f.module.SubmitReport(context.Background(), SubmitReportInput{
		StoreID: s.StoreID, Email: "a@example.com", Category: policy.Fraud, Description: "x",
	})
	must(t, err)
	dupEmail, err := f.module.SubmitReport(context.Background(), SubmitReportInput{
		StoreID: s.StoreID, Phone: "0922222222", Email: "A@example.com", Category: policy.Fraud, Description: "y",
	})
	must(t, err)
	if !dupEmail.Supplemented || dupEmail.ReportID != byEmail.ReportID {
		t.Fatalf("same email in the same week must be a supplement: %+v", dupEmail)
	}
}

// 去重只比對還沒有結果的通報：原本那一筆有了結果之後，同一天的下一次送出是新的通報（包含被判濫報的人）。
func TestAReportWithAnOutcomeIsNoLongerDeduplicated(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	f.everExposed(t, s.StoreID)
	first := f.buyerReport(t, s.StoreID, 5, policy.Fraud)
	phone := f.anonymousReport(t, s.StoreID, "0933333333")
	f.mustResolve(t, first.ReportID, OutcomeAbusive)
	f.mustResolve(t, phone.ReportID, OutcomeNoViolation)
	again := f.buyerReport(t, s.StoreID, 5, policy.Fraud)
	if again.Supplemented || again.ReportID == first.ReportID || again.Report == nil || again.Report.Outcome != nil {
		t.Fatalf("已有結果的通報不再比對，這次是新的通報：%+v", again)
	}
	anonymous := f.anonymousReport(t, s.StoreID, "0933333333")
	if anonymous.Supplemented || anonymous.ReportID == phone.ReportID {
		t.Fatalf("匿名也一樣：%+v", anonymous)
	}
	// 新的這一筆還沒有結果，再送一次就是補充到它。
	if third := f.buyerReport(t, s.StoreID, 5, policy.Fraud); !third.Supplemented || third.ReportID != again.ReportID {
		t.Fatalf("third=%+v", third)
	}
}

// 補充：重複的送出併入原通報（只新增），佐證接在後面編號、帶 supplement_id，引用仍登記在通報 ID；通報的版本前進、案件的版本不變；每次送出各發一個 ReportReceived。
func TestSupplementsAttachToTheOriginalReport(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	received := record[ReportReceived](f)
	accountID, s := f.open(t)
	f.everExposed(t, s.StoreID)
	buyerAccount := accountID + 1000
	first, err := f.module.SubmitReport(ctx, SubmitReportInput{
		StoreID: s.StoreID, ReporterBuyerID: 5, AccountID: buyerAccount, Category: policy.Fraud, Description: "原本的內容", EvidenceTokens: []string{"e1", "e2"},
	})
	must(t, err)
	c := f.mustTriage(t, first.ReportID, policy.Fraud, severityPtr(SeverityMedium))
	caseBefore := f.caseOf(t, *c.CaseID)
	f.jobs.args = nil
	f.now = f.now.Add(time.Hour)
	second, err := f.module.SubmitReport(ctx, SubmitReportInput{
		StoreID: s.StoreID, ReporterBuyerID: 5, AccountID: buyerAccount, Category: policy.Fraud, Description: "  補充：又看到一次  ", EvidenceTokens: []string{"e3"},
	})
	must(t, err)
	if !second.Supplemented || second.ReportID != first.ReportID || second.Report == nil || second.Report.Description != "原本的內容" || second.Report.Version != c.Version+1 {
		t.Fatalf("second=%+v", second)
	}
	if !slices.Contains(f.jobs.kinds(), "seller.media_sync") {
		t.Fatalf("補充的佐證要排入引用的同步：%v", f.jobs.kinds())
	}
	detail := f.reportOf(t, first.ReportID)
	if detail.SupplementCount != 1 || len(detail.Supplements) != 1 || detail.Supplements[0].Description != "補充：又看到一次" || !detail.Supplements[0].CreatedAt.Equal(f.now) ||
		detail.Supplements[0].MatchedBy != nil || !slices.Equal(detail.Supplements[0].EvidencePositions, []int{2}) || detail.LatestSupplementAt == nil || !detail.LatestSupplementAt.Equal(f.now) ||
		len(detail.Evidence) != 3 || detail.Evidence[2].Position != 2 || detail.Evidence[2].SupplementID == nil || *detail.Evidence[2].SupplementID != detail.Supplements[0].ID ||
		detail.Evidence[0].SupplementID != nil || detail.Evidence[2].AssetID != "asset-e3" {
		t.Fatalf("detail=%+v", detail)
	}
	ref := media.Ref{Type: media.RefTypeGovernanceEvidence, ID: first.ReportID.String()}
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-e1 asset-e2 asset-e3]" {
		t.Fatalf("引用仍登記在通報 ID，原通報與補充的佐證都在：%v", got)
	}
	// 補充改通報的版本，不改案件的版本（案件顯示「有新的補充」，直到下一次案件層級的處理）。
	caseAfter := f.caseOf(t, *c.CaseID)
	if caseAfter.Version != caseBefore.Version || !caseAfter.UpdatedAt.Equal(caseBefore.UpdatedAt) || caseAfter.UnseenSupplementReports != 1 || !f.reportOf(t, first.ReportID).HasUnseenSupplements() {
		t.Fatalf("補充不改案件的版本：before=%+v after=%+v", caseBefore.Case, caseAfter.Case)
	}
	// 通報的版本前進：之前讀到的版本分案、設定結果會撞版本。
	_, err = f.module.ResolveReport(ctx, ResolveReportInput{ReportID: first.ReportID, AdminID: 8, Outcome: OutcomeNoViolation, ExpectedVersion: c.Version})
	wantKind(t, err, apperr.KindVersionConflict)
	// 案件層級的處理之後，「有新的補充」消失。
	f.now = f.now.Add(time.Minute)
	f.mustResolve(t, first.ReportID, OutcomeInsufficient)
	if f.caseOf(t, *c.CaseID).UnseenSupplementReports != 0 {
		t.Fatal("處理之後不再有新的補充")
	}
	// 事件：每次送出各發一個；補充的事件帶 supplement_id，登入的檢舉人沒有確認地址。
	events := received.all()
	if len(events) != 2 || events[0].SupplementID != nil || events[0].Discarded || events[1].SupplementID == nil || *events[1].SupplementID != detail.Supplements[0].ID ||
		events[1].ReportID != first.ReportID || events[1].ConfirmationEmail != nil || *events[1].ReporterBuyerID != 5 || events[1].Category != policy.Fraud {
		t.Fatalf("events=%+v", events)
	}
	// 補充只新增：DB 的補充表沒有檢舉人的聯絡方式欄位（匿名補充只記比對方式）。
	if n := f.count(t, "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'report_supplements' AND column_name LIKE 'reporter%'"); n != 0 {
		t.Fatalf("report_supplements 不存檢舉人的聯絡方式：%d 欄", n)
	}
}

// 匿名的補充記下比對方式（電話、聯絡信箱或兩者）；補充者的聯絡方式不存、結果信只寄給原通報存下的地址；回應不帶任何內容。
func TestAnonymousSupplementsRecordHowTheyMatchedWithoutStoringContactDetails(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	received := record[ReportReceived](f)
	_, s := f.open(t)
	f.everExposed(t, s.StoreID)
	submit := func(phone, email, description string) SubmitReportResult {
		t.Helper()
		r, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, Phone: phone, Email: email, Category: policy.FoodSafety, Description: description})
		must(t, err)
		return r
	}
	first := submit("0911111111", "owner@example.com", "原本的內容")
	byPhone := submit("0911111111", "stranger@example.com", "電話相同")
	byEmail := submit("0922222222", "owner@example.com", "信箱相同")
	byBoth := submit("0911111111", "owner@example.com", "兩者相同")
	for _, r := range []SubmitReportResult{byPhone, byEmail, byBoth} {
		if !r.Supplemented || r.ReportID != first.ReportID || r.Report != nil {
			t.Fatalf("匿名的補充不帶內容：%+v", r)
		}
	}
	detail := f.reportOf(t, first.ReportID)
	var matched []SupplementMatch
	for _, s := range detail.Supplements {
		matched = append(matched, *s.MatchedBy)
	}
	if !slices.Equal(matched, []SupplementMatch{MatchedByPhone, MatchedByEmail, MatchedByBoth}) {
		t.Fatalf("matched_by=%v", matched)
	}
	// 通報存的聯絡方式仍是原通報的；補充者的電話與信箱沒有存下。
	if *detail.Contact.Phone != "0911111111" || *detail.Contact.Email != "owner@example.com" || f.count(t, "SELECT count(*) FROM seller.reports") != 1 {
		t.Fatalf("contact=%+v", detail.Contact)
	}
	// 每次送出都發 ReportReceived，確認地址是「這次送出留的」email（A8 用同一個範本寄到那個地址）。
	var emails []string
	for _, e := range received.all() {
		emails = append(emails, *e.ConfirmationEmail)
		if e.ReporterBuyerID != nil || e.Discarded {
			t.Fatalf("event=%+v", e)
		}
	}
	if !slices.Equal(emails, []string{"owner@example.com", "stranger@example.com", "owner@example.com", "owner@example.com"}) {
		t.Fatalf("confirmation emails=%v", emails)
	}
}

// 每筆通報最多 10 則補充：第 11 次送出，登入的回 failed_precondition（已轉正的佐證沒有引用），匿名的內容不存、discarded_submissions 加一、回應照舊、仍發 ReportReceived
// （Discarded，帶這次的 email）。三種情形（新的、補充、超過上限）的回應與事件的形狀相同，送出的人分辨不出這支電話用過。
func TestSupplementLimitIsTenAndAnonymousOverflowIsDiscardedQuietly(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	received := record[ReportReceived](f)
	accountID, s := f.open(t)
	f.everExposed(t, s.StoreID)

	// 登入的檢舉人：10 則之後的第 11 次回 failed_precondition，什麼都不存。
	submit := func(description string, tokens ...string) (SubmitReportResult, error) {
		return f.module.SubmitReport(ctx, SubmitReportInput{
			StoreID: s.StoreID, ReporterBuyerID: 5, AccountID: accountID + 1000, Category: policy.Fraud, Description: description, EvidenceTokens: tokens,
		})
	}
	original, err := submit("原本")
	must(t, err)
	for i := range MaxSupplements {
		if r, err := submit(fmt.Sprintf("補充 %d", i+1)); err != nil || !r.Supplemented {
			t.Fatalf("第 %d 則補充：%+v %v", i+1, r, err)
		}
	}
	f.jobs.args, f.media.finalized = nil, nil
	_, err = submit("第 11 則", "over")
	wantBlocked(t, err, "這筆檢舉已經補充 10 次，不能再補充。")
	if n := f.count(t, "SELECT count(*) FROM seller.report_supplements WHERE report_id = $1", original.ReportID); n != MaxSupplements {
		t.Fatalf("supplements=%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.report_evidence"); n != 0 || slices.Contains(f.jobs.kinds(), "seller.media_sync") {
		t.Fatalf("超過上限的佐證沒有存下也沒有排入同步：evidence=%d jobs=%v", n, f.jobs.kinds())
	}

	// 匿名：三種情形。
	f.now = f.now.Add(8 * 24 * time.Hour) // 下一個台北週，原本的登入通報不在去重範圍
	anon := func(email, description string) SubmitReportResult {
		t.Helper()
		r, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, Phone: "0944444444", Email: email, Category: policy.FoodSafety, Description: description})
		must(t, err)
		return r
	}
	before := len(received.all())
	newReport := anon("a@example.com", "新的")
	for i := range MaxSupplements {
		anon("b@example.com", fmt.Sprintf("補充 %d", i+1))
	}
	over := anon("c@example.com", "超過上限")
	if newReport.Supplemented || over.Supplemented || !over.Discarded || over.Report != nil || over.ReportID != newReport.ReportID {
		t.Fatalf("new=%+v over=%+v", newReport, over)
	}
	detail := f.reportOf(t, newReport.ReportID)
	if detail.SupplementCount != MaxSupplements || detail.DiscardedSubmissions != 1 || len(detail.Supplements) != MaxSupplements {
		t.Fatalf("超過上限的內容不存、計數加一：%+v", detail.Report)
	}
	events := received.all()[before:]
	last := events[len(events)-1]
	if len(events) != MaxSupplements+2 || !last.Discarded || last.SupplementID != nil || *last.ConfirmationEmail != "c@example.com" || events[0].SupplementID != nil || events[0].Discarded {
		t.Fatalf("events=%+v", events)
	}
	// 計數不動版本（沒有存下任何會影響後台判斷的內容）。
	if v := detail.Version; v != 1+MaxSupplements {
		t.Fatalf("版本只隨存下的補充前進：%d", v)
	}
}

// 事件失敗時整筆回滾：新的檢舉沒有寫入、補充沒有存下、結果沒有設定。
func TestEventFailuresRollBackTheWholeCommand(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.everExposed(t, s.StoreID)
	first := f.buyerReport(t, s.StoreID, 5, policy.Fraud)
	c, members := f.newCaseForRollback(t, s.StoreID)
	reportsBefore := f.count(t, "SELECT count(*) FROM seller.reports")
	events.Subscribe(f.bus, func(context.Context, ReportReceived) error { return errors.New("subscriber failed") })
	_, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: 6, Category: policy.Fraud, Description: "新的"})
	if err == nil {
		t.Fatal("訂閱者失敗要讓送出失敗")
	}
	_, err = f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: 5, Category: policy.Fraud, Description: "補充"})
	if err == nil {
		t.Fatal("訂閱者失敗要讓補充失敗")
	}
	if n := f.count(t, "SELECT count(*) FROM seller.reports"); n != reportsBefore || f.count(t, "SELECT count(*) FROM seller.report_supplements") != 0 || f.reportOf(t, first.ReportID).Version != 1 {
		t.Fatalf("回滾之後什麼都不能留下：reports=%d，預期 %d", n, reportsBefore)
	}
	events.Subscribe(f.bus, func(context.Context, ReportResolved) error { return errors.New("subscriber failed") })
	if _, err := f.resolve(t, first.ReportID, OutcomeNoViolation, ""); err == nil {
		t.Fatal("訂閱者失敗要讓設定結果失敗")
	}
	if r := f.reportOf(t, first.ReportID); r.Outcome != nil || r.Version != 1 {
		t.Fatalf("結果不得留下：%+v", r.Report)
	}
	if _, err := f.decide(t, c.ID, DecisionNoAction, ""); err == nil {
		t.Fatal("訂閱者失敗要讓結案失敗")
	}
	if after := f.caseOf(t, c.ID); after.State() != CaseStateOpen || after.Version != c.Version || f.reportOf(t, members[0].ID).Outcome != nil {
		t.Fatalf("結案要整筆回滾：%+v", after.Case)
	}
}

// newCaseForRollback 開一個案件（訂閱者已經會失敗，所以分案不發事件的路徑才走得通）。
func (f *fixture) newCaseForRollback(t *testing.T, storeID int64) (AdminCase, []AdminReport) {
	t.Helper()
	return f.newCase(t, storeID, policy.Counterfeit, SeverityLow, 1)
}

// 兩個 admin 同時對同一筆通報設定結果：只有一個成功，另一個因為版本已經前進而回版本衝突（以版本與目前的狀態為條件，不是讀了再覆寫）。
func TestConcurrentResolutionsTransitionOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	r := f.reportBy(t, s.StoreID, policy.Fraud)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, outcome := range []Outcome{OutcomeNoViolation, OutcomeAbusive} {
		wg.Go(func() {
			_, errs[i] = f.module.ResolveReport(ctx, ResolveReportInput{ReportID: r.ID, AdminID: int64(10 + i), Outcome: outcome, ExpectedVersion: r.Version})
		})
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case apperr.KindOf(err) != apperr.KindVersionConflict:
			t.Fatalf("unexpected: %v", err)
		}
	}
	if ok != 1 || f.reportOf(t, r.ID).Version != r.Version+1 {
		t.Fatalf("resolutions succeeded %d times: %v", ok, errs)
	}
}

// admin 清單是一張表一個 keyset：翻完每一筆剛好出現一次（盤點：舊的兩張表各自 limit／offset 再合併）。
func TestAdminInboxKeysetHasNoGapsOrDuplicates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, a := f.open(t)
	_, b := f.open(t)
	var all []uuid.UUID
	for i := range 9 {
		if i%3 == 0 {
			f.now = f.now.Add(time.Minute) // 其餘同一個時間，以 ID 決定順序
		}
		store := a.StoreID
		if i%2 == 1 {
			store = b.StoreID
		}
		var r SubmitReportResult
		if i%3 == 2 {
			r = f.anonymousReport(t, store, "09"+string(rune('1'+i))+"0000000")
		} else {
			r = f.buyerReport(t, store, int64(100+i), policy.Fraud)
		}
		all = append(all, r.Report.ID)
	}
	for _, oldestFirst := range []bool{false, true} {
		in := ReportListInput{PageSize: 4}
		if oldestFirst {
			awaiting := ReportStateAwaitingTriage
			in.State = &awaiting // 待分案依建立時間舊到新
		}
		var seen []uuid.UUID
		var last *Report
		var after *ReportCursor
		for {
			in.After = after
			page, err := f.module.AdminReports(ctx, in)
			must(t, err)
			for _, r := range page.Items {
				if last != nil {
					newer := r.CreatedAt.After(last.CreatedAt) || (r.CreatedAt.Equal(last.CreatedAt) && r.ID.Compare(last.ID) > 0)
					if newer != oldestFirst {
						t.Fatalf("out of order (oldest first=%v): %v after %v", oldestFirst, r.ID, last.ID)
					}
				}
				seen = append(seen, r.ID)
				rep := r.Report
				last = &rep
			}
			if page.Next == nil {
				break
			}
			after = page.Next
		}
		got, want := slices.Clone(seen), slices.Clone(all)
		slices.SortFunc(got, func(x, y uuid.UUID) int { return x.Compare(y) })
		slices.SortFunc(want, func(x, y uuid.UUID) int { return x.Compare(y) })
		if !slices.Equal(got, want) {
			t.Fatalf("pages returned %d of %d reports (or duplicates)", len(got), len(want))
		}
	}
	food := policy.FoodSafety
	page, err := f.module.AdminReports(ctx, ReportListInput{Category: &food, StoreID: a.StoreID, PageSize: 100})
	must(t, err)
	for _, r := range page.Items {
		if r.Category != policy.FoodSafety || r.StoreID != a.StoreID {
			t.Fatalf("filter leaked %+v", r)
		}
	}
	if len(page.Items) == 0 {
		t.Fatal("filter returned nothing")
	}
	bogus := ReportState("OPEN")
	if _, err := f.module.AdminReports(ctx, ReportListInput{State: &bogus}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("unknown state filter: %v", err)
	}
	mine, err := f.module.BuyerReports(ctx, BuyerReportListInput{BuyerID: 100, PageSize: 10})
	must(t, err)
	if len(mine.Items) != 1 || *mine.Items[0].ReporterBuyerID != 100 {
		t.Fatalf("buyer reports=%+v", mine.Items)
	}
}

// 匿名檢舉人的聯絡欄位：清除時間是 greatest(建立 + 90 天, 結果設定 + 30 天)；不論有沒有結果，建立滿 365 天一律清（絕對上限，Will 2026-10-18），
// 清理回報其中只靠上限清掉的筆數；登入的檢舉不受影響。結果信要寄到這個地址，所以案件拖得再久，有結果的也要等結果設定之後再留 30 天——除非先到了 365 天。
func TestAnonymousContactErasedByTheOutcomeRuleAndTheAbsoluteCap(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	end := f.now.Add(500 * 24 * time.Hour)
	day := 24 * time.Hour
	type scenario struct {
		name              string
		created, resolved time.Duration // 距離 end 的負數；resolved 為 0 表示沒有結果
		erased, capped    bool          // 第一次清理（現在 = end）是否清掉、是否只靠上限
	}
	scenarios := []scenario{
		{"建立 91 天前、結果 29 天前：不清", -91 * day, -29 * day, false, false},
		{"建立 60 天前、結果 31 天前：不清（不早於建立後 90 天）", -60 * day, -31 * day, false, false},
		{"建立 91 天前、結果 31 天前：依結果清", -91 * day, -31 * day, true, false},
		{"建立 91 天前、沒有結果：不清", -91 * day, 0, false, false},
		{"建立剛好 365 天前、沒有結果：不清（滿 365 天是嚴格大於）", -365 * day, 0, false, false},
		{"建立 365 天又 1 微秒前、沒有結果：上限清", -365*day - time.Microsecond, 0, true, true},
		{"建立 400 天前、結果 10 天前（結果還沒滿 30 天）：上限清", -400 * day, -10 * day, true, true},
		{"建立差 1 分鐘滿 365 天、結果 10 天前：不清", -365*day + time.Minute, -10 * day, false, false},
	}
	ids := make([]uuid.UUID, len(scenarios))
	for i, sc := range scenarios {
		f.now = end.Add(sc.created)
		ids[i] = f.anonymousReport(t, s.StoreID, fmt.Sprintf("09%08d", 55000000+i)).Report.ID
		if sc.resolved != 0 {
			f.now = end.Add(sc.resolved)
			f.mustResolve(t, ids[i], OutcomeNoViolation)
		}
	}
	f.now = end.Add(-400 * day)
	logged := f.buyerReport(t, s.StoreID, 5, policy.Fraud).ReportID
	f.mustResolve(t, logged, OutcomeNoViolation)
	f.now = end
	var wantErased, wantCapped int64
	for _, sc := range scenarios {
		if sc.erased {
			wantErased++
		}
		if sc.capped {
			wantCapped++
		}
	}
	got, err := f.module.EraseExpiredReportContacts(ctx)
	must(t, err)
	if got.Erased != wantErased || got.Capped != wantCapped {
		t.Fatalf("清理 %+v，預期清 %d 筆、其中 %d 筆只靠上限", got, wantErased, wantCapped)
	}
	for i, sc := range scenarios {
		r := f.reportOf(t, ids[i])
		if erased := r.ContactErasedAt != nil; erased != sc.erased || (erased && (r.Contact != nil || !r.ContactErasedAt.Equal(end))) || r.Description == "" {
			t.Errorf("%s：erased=%v %+v", sc.name, erased, r.Report)
		}
	}
	if r := f.reportOf(t, logged); r.ContactErasedAt != nil {
		t.Fatalf("登入的檢舉不受影響（即使超過 365 天）：%+v", r.Report)
	}
	if got, err := f.module.EraseExpiredReportContacts(ctx); err != nil || got != (ContactErasure{}) {
		t.Fatalf("second run: %+v err=%v", got, err)
	}
	// 之後：第 4 筆有了結果要等結果設定後 30 天才清；第 1、2 筆依結果規則先到期；第 5 筆（剛好 365 天、沒有結果）只靠上限清（Capped 1）；第 8 筆（差 1 分鐘滿 365 天）
	// 這時結果規則也到期了，不算只靠上限。
	f.now = end.Add(time.Hour)
	f.mustResolve(t, ids[3], OutcomeInsufficient)
	f.now = end.Add(time.Hour + 30*day - time.Minute)
	got, err = f.module.EraseExpiredReportContacts(ctx)
	must(t, err)
	if got != (ContactErasure{Erased: 4, Capped: 1}) || f.reportOf(t, ids[3]).ContactErasedAt != nil || f.reportOf(t, ids[4]).ContactErasedAt == nil || f.reportOf(t, ids[7]).ContactErasedAt == nil {
		t.Fatalf("結果設定後差一分鐘滿 30 天：%+v err=%v", got, err)
	}
	f.now = end.Add(time.Hour + 30*day + time.Minute)
	if got, err := f.module.EraseExpiredReportContacts(ctx); err != nil || got != (ContactErasure{Erased: 1}) || f.reportOf(t, ids[3]).ContactErasedAt == nil {
		t.Fatalf("結果設定後滿 30 天：%+v err=%v", got, err)
	}
}

// 佐證以通報 ID 為 ref：轉正不建立引用，寫入的 transaction 排入同步。轉正之後撞到重複時，這次的內容與佐證併入原通報成為補充（位置接在後面），一樣排入同步。
func TestReportEvidenceIsLinkedByTheSyncJobUnderTheReportID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	f.everExposed(t, s.StoreID)
	buyerAccount := accountID + 1000 // 檢舉人的帳號不是店家的擁有者
	r, err := f.module.SubmitReport(ctx, SubmitReportInput{
		StoreID: s.StoreID, ReporterBuyerID: 5, AccountID: buyerAccount, Category: policy.FoodSafety,
		Description: "附圖", EvidenceTokens: []string{"e1", "e2"},
	})
	must(t, err)
	if len(r.Report.Evidence) != 2 || len(f.media.finalized) != 2 || f.media.finalized[0] != media.RefTypeGovernanceEvidence {
		t.Fatalf("report=%+v finalized=%+v", r.Report, f.media.finalized)
	}
	ref := media.Ref{Type: media.RefTypeGovernanceEvidence, ID: r.Report.ID.String()}
	if got := f.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("寫入不得建立引用：%v", got)
	}
	detail := f.reportOf(t, r.Report.ID)
	if len(detail.Evidence) != 2 || detail.Evidence[1].AssetID != "asset-e2" {
		t.Fatalf("evidence=%+v", detail.Evidence)
	}
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-e1 asset-e2]" {
		t.Fatalf("同步之後的引用 = %v", got)
	}

	// 轉正之後、寫入之前，同一位檢舉人的另一次送出搶先寫入：這次的送出併入那一筆（補充），佐證接在後面。
	f.media.onFinalize = func() {
		if _, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: 6, Category: policy.Fraud, Description: "搶先"}); err != nil {
			t.Error(err)
		}
	}
	f.jobs.args = nil
	dup, err := f.module.SubmitReport(ctx, SubmitReportInput{
		StoreID: s.StoreID, ReporterBuyerID: 6, AccountID: buyerAccount + 1, Category: policy.Fraud, Description: "附圖", EvidenceTokens: []string{"e3"},
	})
	must(t, err)
	if !dup.Supplemented || !slices.Contains(f.jobs.kinds(), "seller.media_sync") {
		t.Fatalf("撞到重複的送出併入原通報，佐證要排入同步：dup=%+v jobs=%v", dup, f.jobs.kinds())
	}
	merged := f.reportOf(t, dup.ReportID)
	if len(merged.Evidence) != 1 || merged.Evidence[0].Position != 0 || merged.Evidence[0].SupplementID == nil || len(merged.Supplements) != 1 {
		t.Fatalf("merged=%+v", merged)
	}
}

// 來源通報的結果否定了它（未違規、內容不足、濫報），就不能成立認定；其餘（沒有結果、已採取措施、重複）可以。與 API 的 can_cite_as_source 同一個規則（Report.Supporting）。
func TestMeasureRejectsASourceReportWhoseOutcomeNegatesIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	for _, o := range []Outcome{OutcomeNoViolation, OutcomeInsufficient, OutcomeAbusive} {
		report := f.reportBy(t, s.StoreID, policy.Fraud)
		f.mustResolve(t, report.ID, o)
		_, err := f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID, report.ID))
		wantBlocked(t, err, "結果是未違規、內容不足或濫報的通報，不能當認定的來源。")
	}
	if f.count(t, "SELECT count(*) FROM seller.violations")+f.count(t, "SELECT count(*) FROM seller.measures") != 0 {
		t.Fatal("measure written for a negated report")
	}
	supporting := f.reportBy(t, s.StoreID, policy.Fraud)
	f.mustResolve(t, supporting.ID, OutcomeDuplicate)
	pending := f.reportBy(t, s.StoreID, policy.Fraud)
	taken, members := f.newCase(t, s.StoreID, policy.Counterfeit, SeverityLow, 1)
	f.riskFromCase(t, s.StoreID, taken.ID)
	f.mustResolve(t, members[0].ID, OutcomeActionTaken)
	for name, id := range map[string]uuid.UUID{"重複": supporting.ID, "還沒有結果": pending.ID, "已採取措施": members[0].ID} {
		if _, err := f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID, id)); err != nil {
			t.Errorf("%s 的通報可以當來源：%v", name, err)
		}
	}
}

// 建立措施鎖住來源通報（店家列之後、通報列）：另一個 admin 正在把通報結案為未違規（交易還沒 commit）時，建立要等它，等到之後看到的是未違規。
// 通報列沒鎖的話，建立讀到結案之前的狀態，認定的來源通報就是被否定的。
func TestMeasureLocksTheSourceReport(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	report := f.buyerReport(t, s.StoreID, 5, policy.Fraud).Report
	resolver, err := f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = resolver.Rollback(ctx) }()
	_, err = resolver.Exec(ctx, "UPDATE seller.reports SET outcome='NO_VIOLATION', resolved_at=$2, resolved_by_admin_id=1 WHERE id=$1", report.ID, f.now)
	must(t, err)
	done := make(chan error, 1)
	go func() {
		_, err := f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID, report.ID))
		done <- err
	}()
	f.waitForLockWaiters(t, 1)
	must(t, resolver.Commit(ctx))
	if err := <-done; apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("create after the report was resolved as no violation: %v", err)
	}
	if f.count(t, "SELECT count(*) FROM seller.violations")+f.count(t, "SELECT count(*) FROM seller.measures") != 0 {
		t.Fatal("measure written for a report resolved meanwhile")
	}
}

// maxSourceReports 是一個認定最多的來源通報數（API 契約 NewFinding.source_report_ids 的上限）。
const maxSourceReports = 20

// 來源通報有任何一筆不合格，整個建立都不寫入：別的店家的（invalid_argument，指到那一筆）、結果被否定的（failed_precondition）、
// 不存在的（not_found）；筆數超過上限與重複也是 invalid_argument。
func TestMeasureRejectsAnyInvalidSourceReportAndWritesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, other := f.open(t)
	own := f.buyerReport(t, s.StoreID, 5, policy.Fraud).Report
	foreign := f.buyerReport(t, other.StoreID, 5, policy.Fraud).Report
	negated := f.buyerReport(t, s.StoreID, 6, policy.Fraud).Report
	f.mustResolve(t, negated.ID, OutcomeNoViolation)
	tooMany := make([]uuid.UUID, maxSourceReports+1)
	for i := range tooMany {
		tooMany[i] = uuid.NewV7()
	}
	for name, tc := range map[string]struct {
		sources []uuid.UUID
		kind    apperr.Kind
	}{
		"其中一筆是別的店家的": {[]uuid.UUID{own.ID, foreign.ID}, apperr.KindInvalidInput},
		"只有別的店家的":    {[]uuid.UUID{foreign.ID}, apperr.KindInvalidInput},
		"其中一筆結果是未違規": {[]uuid.UUID{own.ID, negated.ID}, apperr.KindFailedPrecondition},
		"其中一筆不存在":    {[]uuid.UUID{own.ID, uuid.NewV7()}, apperr.KindNotFound},
		"重複":         {[]uuid.UUID{own.ID, own.ID}, apperr.KindInvalidInput},
		"空 ID":       {[]uuid.UUID{uuid.Nil()}, apperr.KindInvalidInput},
		"超過上限":       {tooMany, apperr.KindInvalidInput},
	} {
		if _, err := f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID, tc.sources...)); apperr.KindOf(err) != tc.kind {
			t.Errorf("%s：%v", name, err)
		}
	}
	for _, table := range []string{"violations", "finding_reports", "measures"} {
		if n := f.count(t, "SELECT count(*) FROM seller."+table); n != 0 {
			t.Errorf("%s=%d：被拒絕的建立不得留下任何列", table, n)
		}
	}
	atLimit := make([]uuid.UUID, 0, maxSourceReports)
	for range maxSourceReports {
		atLimit = append(atLimit, f.buyerReport(t, s.StoreID, int64(100+len(atLimit)), policy.Fraud).Report.ID)
	}
	if _, err := f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID, atLimit...)); err != nil {
		t.Fatalf("剛好 %d 筆要成功：%v", maxSourceReports, err)
	}
}

// 重複提交（補充）只回既有那一筆的 ID：匿名的不回它的內容、聯絡方式或任何檢舉人的資料（CR-5）。只知道電話號碼的人重送，不該因此
// 讀到原檢舉人留下的姓名、email 與描述；登入的檢舉人拿回他自己那一筆（去重的鍵含買家）。
func TestDuplicateSubmissionReturnsOnlyTheReportID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.everExposed(t, s.StoreID)
	first, err := f.module.SubmitReport(ctx, SubmitReportInput{
		StoreID: s.StoreID, Phone: "0912345678", Email: "victim@example.com", Name: "王小明", Category: policy.FoodSafety, Description: "食用後腹瀉",
	})
	must(t, err)
	if first.Report == nil || first.Report.ID != first.ReportID || first.Supplemented {
		t.Fatalf("first=%+v", first)
	}
	dup, err := f.module.SubmitReport(ctx, SubmitReportInput{
		StoreID: s.StoreID, Phone: "0912345678", Name: "路人", Category: policy.Fraud, Description: "只知道電話的人",
	})
	must(t, err)
	if !dup.Supplemented || dup.ReportID != first.ReportID || dup.Report != nil || dup.Notice.Text != "" {
		t.Fatalf("a duplicate must return only the id: %+v", dup)
	}
	buyer := f.buyerReport(t, s.StoreID, 5, policy.Fraud)
	again := f.buyerReport(t, s.StoreID, 5, policy.Fraud)
	if !again.Supplemented || again.ReportID != buyer.ReportID || again.Report == nil || again.Report.ID != buyer.ReportID || again.Notice.Text != "你今天已經檢舉過這家店的這個類別，這次的內容已經補充到原本的檢舉。" {
		t.Fatalf("logged-in supplement: %+v", again)
	}
}

// 認定的類別只接受平台判定與認定可用的子集：全部除了 OTHER（OTHER 只給檢舉人選，認定的理由要指到條款）。不接受的值是
// invalid_argument，而且不建立措施與認定。
func TestFindingCategoryIsLimitedToWhatThePlatformCanAssess(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	create := func(c policy.Category) error {
		_, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
			AdminID: 7, StoreID: s.StoreID, Kind: MeasureWarning, StatementToSeller: "警告", Note: "n",
			Violation: &ViolationBasis{Finding: &NewFinding{Category: c, Severity: SeverityLow}},
		})
		return err
	}
	rejected := []policy.Category{policy.Other, "", "UNAUTHORIZED_SALE"}
	for _, c := range rejected {
		if err := create(c); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("認定的類別 %q：%v，預期 invalid_argument", c, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.violations") + f.count(t, "SELECT count(*) FROM seller.measures"); n != 0 {
		t.Fatalf("被拒絕的請求寫入了 %d 列", n)
	}
	accepted := 0
	for _, c := range policy.Values() {
		if slices.Contains(rejected, c) {
			continue
		}
		if err := create(c); err != nil {
			t.Errorf("認定可以用類別 %q：%v", c, err)
		}
		accepted++
	}
	if accepted != 15 {
		t.Fatalf("認定可用的類別 %d 個，預期 15 個", accepted)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.violations"); n != 15 {
		t.Fatalf("寫入了 %d 筆認定，預期 15 筆", n)
	}
}

// 申覆的佐證以請求 ID 登記引用，請求 ID 在轉正之後才產生：轉正與 commit 之間不可能有人為這個 ref 同步；同步 job 在寫入之後登記
// 業務列指著的資產。
func TestAppealEvidenceIsLinkedByTheSyncJobUnderTheRequestID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, _, m := f.appealable(t)
	f.media.AfterFinalize = func(context.Context, media.Asset) {
		f.media.AfterFinalize = nil
		f.syncMedia(t, media.Ref{Type: media.RefTypeGovernanceEvidence, ID: m.ID.String()})
	}
	f.appeal(t, accountID, m, "已改善", "x1")
	request := f.measureOf(t, m.ID).Appeal().ID
	ref := media.Ref{Type: media.RefTypeGovernanceEvidence, ID: request.String()}
	if got := f.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("寫入不得建立引用：%v", got)
	}
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-x1]" {
		t.Fatalf("references=%v", got)
	}
	if got := f.media.Referenced(media.Ref{Type: media.RefTypeGovernanceEvidence, ID: m.ID.String()}); len(got) != 0 {
		t.Fatalf("措施 ID 不是佐證的 ref：%v", got)
	}
}

// warningFrom 是以新的認定建立警告的輸入，來源通報可選（可以多筆）。
func (f *fixture) warningFrom(storeID int64, sources ...uuid.UUID) CreateMeasureInput {
	return CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: MeasureWarning, StatementToSeller: "y", Note: "x",
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow, SourceReportIDs: sources}},
	}
}

// 一個認定可以有多筆來源通報（同一家店）：寫入 finding_reports，建立的回應、措施與認定的讀取都依通報 id 升序列出。
func TestMeasureStoresAndReadsMultipleSourceReports(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	first := f.buyerReport(t, s.StoreID, 5, policy.Fraud).Report
	second := f.buyerReport(t, s.StoreID, 6, policy.Fraud).Report
	want := []uuid.UUID{first.ID, second.ID}
	slices.SortFunc(want, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	created, err := f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID, want[1], want[0]))
	must(t, err)
	finding := created.Measure.Findings[0]
	if !slices.Equal(finding.SourceReportIDs, want) {
		t.Fatalf("created sources=%v want %v", finding.SourceReportIDs, want)
	}
	if got := f.measureOf(t, created.Measure.ID).Findings[0].SourceReportIDs; !slices.Equal(got, want) {
		t.Fatalf("measure sources=%v want %v", got, want)
	}
	violation, err := f.module.Violation(ctx, finding.ID)
	must(t, err)
	if !slices.Equal(violation.SourceReportIDs, want) {
		t.Fatalf("violation sources=%v want %v", violation.SourceReportIDs, want)
	}
	page, err := f.module.AdminViolations(ctx, ViolationListInput{StoreID: s.StoreID, PageSize: 10})
	must(t, err)
	if len(page.Items) != 1 || !slices.Equal(page.Items[0].SourceReportIDs, want) {
		t.Fatalf("listed=%+v", page.Items)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.finding_reports WHERE violation_id = $1", finding.ID); n != 2 {
		t.Fatalf("finding_reports=%d", n)
	}
	none, err := f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID))
	must(t, err)
	if len(none.Measure.Findings[0].SourceReportIDs) != 0 {
		t.Fatalf("no sources expected: %v", none.Measure.Findings[0].SourceReportIDs)
	}
}

// 建立措施的冪等：來源通報與引用的認定是集合，同一個 request_id 帶同一組 id 換了順序（管理員取消勾選再勾回）是同一個請求，
// 回傳原本的措施、不會再建一個；少一筆或換一筆就是不同的內容（invalid_argument，欄位 RequestID）。
func TestCreateMeasureRequestIDIgnoresTheOrderOfSourceReportsAndCitedFindings(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	reports := []uuid.UUID{
		f.buyerReport(t, s.StoreID, 5, policy.Fraud).Report.ID,
		f.buyerReport(t, s.StoreID, 6, policy.Fraud).Report.ID,
		f.buyerReport(t, s.StoreID, 7, policy.Fraud).Report.ID,
	}
	// 兩筆既有的認定供引用（各由一個警告建立）。
	var cited []uuid.UUID
	for range 2 {
		earlier, err := f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID))
		must(t, err)
		cited = append(cited, earlier.Measure.Findings[0].ID)
	}
	requestID := uuid.New()
	build := func(sources, citedIDs []uuid.UUID) CreateMeasureInput {
		in := f.warningFrom(s.StoreID, sources...)
		in.RequestID = &requestID
		in.Violation.CitedFindingIDs = citedIDs
		return in
	}
	reversed := func(ids []uuid.UUID) []uuid.UUID {
		out := slices.Clone(ids)
		slices.Reverse(out)
		return out
	}

	first, err := f.module.CreateMeasure(ctx, build(reports, cited))
	must(t, err)
	if first.Replayed {
		t.Fatal("第一次不是重送")
	}
	again, err := f.module.CreateMeasure(ctx, build(reversed(reports), reversed(cited)))
	must(t, err)
	if !again.Replayed || again.Measure.ID != first.Measure.ID {
		t.Fatalf("換了順序的同一組 id 要回傳原本的措施：%+v", again)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 3 {
		t.Fatalf("measures=%d，換順序重送不能再建一個", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.finding_reports"); n != int64(len(reports)) {
		t.Fatalf("finding_reports=%d", n)
	}

	for name, in := range map[string]CreateMeasureInput{
		"少一筆來源通報":  build(reports[:2], cited),
		"少一筆引用的認定": build(reports, cited[:1]),
	} {
		_, err := f.module.CreateMeasure(ctx, in)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "RequestID" {
			t.Fatalf("%s：同 id 不同內容要回 invalid_argument 指到 RequestID：%v", name, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 3 {
		t.Fatalf("measures=%d", n)
	}
}

// 同一個 request_id 帶不同的來源通報是不同的內容（invalid_argument）；同樣的來源通報重送回原本的措施。
func TestCreateMeasureRequestIDCoversTheSourceReports(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	a := f.buyerReport(t, s.StoreID, 5, policy.Fraud).Report
	b := f.buyerReport(t, s.StoreID, 6, policy.Fraud).Report
	requestID := uuid.New()
	in := f.warningFrom(s.StoreID, a.ID)
	in.RequestID = &requestID
	first, err := f.module.CreateMeasure(ctx, in)
	must(t, err)
	again, err := f.module.CreateMeasure(ctx, in)
	must(t, err)
	if again.Measure.ID != first.Measure.ID {
		t.Fatalf("replay returned another measure")
	}
	in.Violation = &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow, SourceReportIDs: []uuid.UUID{a.ID, b.ID}}}
	if _, err := f.module.CreateMeasure(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("same request id with other sources: %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.finding_reports"); n != 1 {
		t.Fatalf("finding_reports=%d", n)
	}
}

// finding_reports 的 migration：既有認定的單筆來源通報搬進關聯表，沒有來源的不產生列，violations.source_report_id 消失。
func TestFindingReportsMigrationMovesTheSingleSourceReport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_finding_reports.sql") })
	if at < 1 {
		t.Fatalf("migrations=%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		exec(body(name))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000003, 31, 'a', now(), now())")
	exec("INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, created_at, updated_at) SELECT uuidv7(), store_id, 'FRAUD', 'x', 5, now(), now() FROM seller.stores WHERE store_code = 2600000003")
	// 認定的建立措施（origin_measure）與這個 migration 無關：丟掉外鍵，免得為了兩列認定先造措施。
	exec("ALTER TABLE seller.violations DROP CONSTRAINT violations_origin_measure_fkey")
	insert := "INSERT INTO seller.violations (id, store_id, account_id, source_report_id, category, severity, origin_measure_id, created_by_admin_id, created_at, updated_at) " +
		"SELECT uuidv7(), s.store_id, s.account_id, (SELECT id FROM seller.reports LIMIT 1) IF_SOURCE, 'FRAUD', 'LOW', uuidv7(), 7, now(), now() FROM seller.stores s WHERE s.store_code = 2600000003"
	exec(strings.Replace(insert, "IF_SOURCE", "", 1))
	exec(strings.Replace(insert, "(SELECT id FROM seller.reports LIMIT 1) IF_SOURCE", "NULL", 1))
	exec(body(files[at]))
	var links, withReport, columns int64
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*), count(*) FILTER (WHERE report_id = (SELECT id FROM seller.reports)) FROM seller.finding_reports").Scan(&links, &withReport))
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='seller' AND table_name='violations' AND column_name='source_report_id'").Scan(&columns))
	if links != 1 || withReport != 1 || columns != 0 {
		t.Fatalf("links=%d matching=%d source_report_id columns=%d", links, withReport, columns)
	}
}

// 升到 7b 時 DB 可能已經有 7a 的處分列（開發用的 DB）：新的 migration 要能套用，回填 source = 'ADMIN'、把 suspension_id
// 換成 UUID，並丟掉 stores.suspension（CR-2、DR A-4）。
func TestGovernanceMigrationAppliesOnADatabaseWithSevenAData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	if len(files) < 2 || !strings.HasSuffix(files[0], "_seller_core.sql") || !strings.HasSuffix(files[1], "_seller_governance.sql") {
		t.Fatalf("migrations=%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	exec(body(files[0]))
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, suspension, created_at, updated_at) VALUES (2600000001, 11, 'a', 'ADMIN', now(), now())")
	exec("INSERT INTO seller.suspensions (store_id, reason, detail, suspended_by_admin_id, suspended_at, lifted_by_admin_id, lifted_at) VALUES (1, 'OTHER', '舊', 5, now() - interval '2 days', 6, now() - interval '1 day')")
	exec("INSERT INTO seller.suspensions (store_id, reason, suspended_by_admin_id, suspended_at) VALUES (1, 'POLICY_VIOLATION', 7, now())")
	exec("INSERT INTO seller.suspension_periods (store_id, started_at) VALUES (1, now())")
	exec(body(files[1]))
	var sources, distinctIDs, nullIDs int64
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FILTER (WHERE source = 'ADMIN'), count(DISTINCT suspension_id), count(*) FILTER (WHERE suspension_id IS NULL) FROM seller.suspensions").Scan(&sources, &distinctIDs, &nullIDs))
	if sources != 2 || distinctIDs != 2 || nullIDs != 0 {
		t.Fatalf("admin rows=%d distinct ids=%d null ids=%d", sources, distinctIDs, nullIDs)
	}
	var idType string
	var storeColumn, openPeriods, activeAdmin int64
	must(t, d.Pool().QueryRow(ctx, "SELECT data_type FROM information_schema.columns WHERE table_schema='seller' AND table_name='suspensions' AND column_name='suspension_id'").Scan(&idType))
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='seller' AND table_name='stores' AND column_name='suspension'").Scan(&storeColumn))
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM seller.suspension_periods WHERE ended_at IS NULL").Scan(&openPeriods))
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM seller.suspensions WHERE source = 'ADMIN' AND lifted_at IS NULL AND reason = 'POLICY_VIOLATION'").Scan(&activeAdmin))
	if idType != "uuid" || storeColumn != 0 || openPeriods != 1 || activeAdmin != 1 {
		t.Fatalf("id type=%s stores.suspension columns=%d open periods=%d active admin rows=%d", idType, storeColumn, openPeriods, activeAdmin)
	}
}

// 帳號欄位的 migration（suspension_account_id）：先套用它之前的全部，寫入一筆舊形狀的管理員停權，再套用它：既有的列由店家的
// account_id 回填，之後才設 NOT NULL（不能在 NULL 的列上失敗）。
func TestSuspensionAccountIDMigrationBackfillsFromTheStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_suspension_account_id.sql") })
	if at < 1 {
		t.Fatalf("migrations=%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		exec(body(name))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000002, 21, 'a', now(), now())")
	exec("INSERT INTO seller.store_exposures (store_id) SELECT store_id FROM seller.stores WHERE store_code = 2600000002")
	exec("INSERT INTO seller.suspensions (suspension_id, store_id, source, reason, suspended_by_admin_id, suspended_at) SELECT uuidv7(), store_id, 'ADMIN', 'POLICY_VIOLATION', 7, now() FROM seller.stores WHERE store_code = 2600000002")
	exec(body(files[at]))
	var accountID int64
	var nullable string
	must(t, d.Pool().QueryRow(ctx, "SELECT account_id FROM seller.suspensions").Scan(&accountID))
	must(t, d.Pool().QueryRow(ctx, "SELECT is_nullable FROM information_schema.columns WHERE table_schema='seller' AND table_name='suspensions' AND column_name='account_id'").Scan(&nullable))
	if accountID != 21 || nullable != "NO" {
		t.Fatalf("account_id=%d（要回填成店家的 21）、is_nullable=%s（要是 NO）", accountID, nullable)
	}
}
