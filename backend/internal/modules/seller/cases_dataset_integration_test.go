//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// allReports 讀出全部通報（後台清單的所有頁）。
func (f *fixture) allReports(t *testing.T, in ReportListInput) []AdminReport {
	t.Helper()
	in.PageSize = 3
	var all []AdminReport
	for {
		page, err := f.module.AdminReports(context.Background(), in)
		must(t, err)
		all = append(all, page.Items...)
		if page.Next == nil {
			return all
		}
		in.After = page.Next
	}
}

// allCases 讀出全部案件（後台清單的所有頁）。
func (f *fixture) allCases(t *testing.T, in CaseListInput) []AdminCase {
	t.Helper()
	in.PageSize = 3
	var all []AdminCase
	for {
		page, err := f.module.AdminCases(context.Background(), in)
		must(t, err)
		all = append(all, page.Items...)
		if page.Next == nil {
			return all
		}
		in.After = page.Next
	}
}

func idsOfReports(reports []AdminReport) []uuid.UUID {
	out := make([]uuid.UUID, len(reports))
	for i, r := range reports {
		out[i] = r.ID
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

// 清單的篩選與判定式是同一個定義：ListReports(state) 的集合等於 Report.State() 的集合，ListCases(state) 同理，生效的類別、店家、案件篩選也一致；
// 翻頁不漏不重。資料集含每種通報狀態：待分案、在進行中的案件、沒有案件而有結果、在已結案的案件、移動過、原案件被清空。
func TestListMembershipEqualsTheStatePredicates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, a := f.open(t)
	_, b := f.open(t)
	for range 3 {
		f.reportBy(t, a.StoreID, policy.Fraud)
	}
	f.anonymousReport(t, b.StoreID, "0912345678")
	openCase, _ := f.newCase(t, a.StoreID, policy.Fraud, SeverityHigh, 3)
	f.newCase(t, b.StoreID, policy.Counterfeit, SeverityLow, 2)
	f.mustResolve(t, f.reportBy(t, a.StoreID, policy.Fraud).ID, OutcomeNoViolation)
	f.mustResolve(t, f.reportBy(t, b.StoreID, policy.Privacy).ID, OutcomeAbusive)
	decided, _ := f.newCase(t, a.StoreID, policy.Privacy, SeverityLow, 2)
	_, err := f.decide(t, decided.ID, DecisionNoAction, "")
	must(t, err)
	emptied, solo := f.newCase(t, b.StoreID, policy.IPInfringement, SeverityLow, 1)
	moved, err := f.move(t, solo[0].ID, policy.Fraud, severityPtr(SeverityMedium))
	must(t, err)
	if !moved.SourceCaseClosed {
		t.Fatal("原案件應被清空")
	}

	everything := f.allReports(t, ReportListInput{})
	if len(everything) != 3+1+3+2+2+2+1 {
		t.Fatalf("資料集的通報數 %d", len(everything))
	}
	byID := map[uuid.UUID]AdminReport{}
	for _, r := range everything {
		byID[r.ID] = f.reportOf(t, r.ID)
	}
	// 狀態：清單的集合 = 判定式的集合，三種狀態都有成員。
	seen := map[ReportState]int{}
	for _, state := range ReportStateValues() {
		var want []uuid.UUID
		for id, r := range byID {
			if r.State() == state {
				want = append(want, id)
			}
		}
		slices.SortFunc(want, uuid.UUID.Compare)
		got := idsOfReports(f.allReports(t, ReportListInput{State: &state}))
		if !slices.Equal(got, want) || len(want) == 0 {
			t.Errorf("ListReports(state=%s)：%d 筆，Report.State()：%d 筆", state, len(got), len(want))
		}
		seen[state] = len(want)
	}
	if len(seen) != 3 {
		t.Fatalf("seen=%v", seen)
	}
	// 待分案（Report.State()）與列表的集合一致，且依建立時間舊到新（等最久的在最前面）。
	awaiting := ReportStateAwaitingTriage
	list := f.allReports(t, ReportListInput{State: &awaiting})
	for i, r := range list {
		if r.State() != ReportStateAwaitingTriage || (i > 0 && (r.CreatedAt.Before(list[i-1].CreatedAt) || r.CreatedAt.Equal(list[i-1].CreatedAt) && r.ID.Compare(list[i-1].ID) < 0)) {
			t.Errorf("待分案的順序或判定式不一致：%+v", r.Report)
		}
	}
	// 生效的類別、店家、案件篩選。
	for _, category := range policy.ReporterSelectable() {
		var want []uuid.UUID
		for id, r := range byID {
			effective := r.Category
			if r.AssessedCategory != nil {
				effective = *r.AssessedCategory
			}
			if effective == category {
				want = append(want, id)
			}
		}
		slices.SortFunc(want, uuid.UUID.Compare)
		if got := idsOfReports(f.allReports(t, ReportListInput{Category: &category})); !slices.Equal(got, want) {
			t.Errorf("ListReports(category=%s)：%d 筆，預期 %d 筆", category, len(got), len(want))
		}
	}
	for _, storeID := range []int64{a.StoreID, b.StoreID} {
		var want []uuid.UUID
		for id, r := range byID {
			if r.StoreID == storeID {
				want = append(want, id)
			}
		}
		slices.SortFunc(want, uuid.UUID.Compare)
		if got := idsOfReports(f.allReports(t, ReportListInput{StoreID: storeID})); !slices.Equal(got, want) {
			t.Errorf("ListReports(store=%d)：%d 筆，預期 %d 筆", storeID, len(got), len(want))
		}
	}
	members := f.allReports(t, ReportListInput{CaseID: &openCase.ID})
	if len(members) != 3 {
		t.Fatalf("案件的成員 %d 筆", len(members))
	}
	for i, r := range members {
		if r.CaseID == nil || *r.CaseID != openCase.ID || (i > 0 && r.ID.Compare(members[i-1].ID) < 0) {
			t.Errorf("案件成員（舊到新）：%+v", r.Report)
		}
	}
	// 案件：狀態篩選與判定式一致；待決定的判定式 = 進行中。
	cases := f.allCases(t, CaseListInput{})
	if len(cases) != 5 {
		t.Fatalf("cases=%d", len(cases))
	}
	for _, state := range CaseStateValues() {
		var want []uuid.UUID
		for _, c := range cases {
			if c.State() == state {
				want = append(want, c.ID)
			}
		}
		var got []uuid.UUID
		for _, c := range f.allCases(t, CaseListInput{State: &state}) {
			got = append(got, c.ID)
			if (state == CaseStateOpen) != (c.Open() && !c.AwaitingSeller()) || (state == CaseStateOpen) != c.Open() {
				t.Errorf("待決定的判定式：%+v", c.Case)
			}
		}
		slices.SortFunc(want, uuid.UUID.Compare)
		slices.SortFunc(got, uuid.UUID.Compare)
		if !slices.Equal(got, want) || len(want) == 0 {
			t.Errorf("ListCases(state=%s)：%d 筆，Case.State()：%d 筆", state, len(got), len(want))
		}
	}
	if emptied := f.caseOf(t, emptied.ID); emptied.State() != CaseStateClosed || emptied.CloseReason == nil || *emptied.CloseReason != CaseClosedNoReports {
		t.Fatalf("emptied=%+v", emptied.Case)
	}
	counterfeit := policy.Counterfeit
	if got := f.allCases(t, CaseListInput{Category: &counterfeit, StoreID: b.StoreID}); len(got) != 1 || got[0].Category != policy.Counterfeit {
		t.Fatalf("案件的類別與店家篩選：%+v", got)
	}
}

// 案件導出的措施的定義在 SQL 有四處寫法：CaseMeasureCounts（可用性用），與措施清單三支查詢（AdminMeasures、AdminMeasuresPendingRequests、AdminMeasuresReviewDue）的
// source_case_id 篩選；source_report_id 的篩選在三支清單查詢也各一份。四處都要等於預期的集合，所以資料集同時要讓三條清單路徑各自有成員、也各自有「導出但不屬於這條路徑」
// 與「屬於這條路徑但沒有導出」的措施：只有明確連結的（RISK 停權沒有來源通報）、認定以案件的通報為來源的、引用既有認定的、已撤銷的、兩個案件共用一個認定（來源通報在不同案件）、
// 與案件無關的；待處理的請求（申覆與解除請求，收件匣）；複審到期與還沒到期的 RISK 措施。
// RISK 措施沒有認定，所以複審到期這條路徑只有明確連結的那一邊（source_case_id = 案件）有成員，source_report_id 的篩選在這條路徑上一定是空的。
func TestDerivedMeasuresHaveOneDefinitionInAllFourQueries(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	start := f.now
	accountID, s := f.open(t)
	c1, m1 := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 3)
	c2, m2 := f.newCase(t, s.StoreID, policy.Counterfeit, SeverityLow, 2)

	linked := f.riskFromCase(t, s.StoreID, c1.ID)              // 明確連結，沒有來源通報
	viaFinding := f.warningCiting(t, s.StoreID, m1[0].ID)      // 認定的來源通報在 c1
	cited := f.warningCitingExisting(t, s.StoreID, viaFinding) // 引用既有的認定（沒有新的認定）
	revoked := f.warningCiting(t, s.StoreID, m1[1].ID)         // 來源通報在 c1，已撤銷
	f.revoke(t, revoked)
	shared := f.warningCiting(t, s.StoreID, m1[2].ID, m2[0].ID) // 一個認定的來源通報在兩個案件
	linked2 := f.riskFromCase(t, s.StoreID, c2.ID)              // c2 的明確連結，之後撤銷
	f.revoke(t, linked2)
	unrelated := f.warningCiting(t, s.StoreID) // 沒有來源通報、沒有連結
	unrelatedRisk := f.riskSuspend(t, s.StoreID)
	f.now = start.Add(3 * 24 * time.Hour)
	notDue := f.riskFromCase(t, s.StoreID, c1.ID) // 連到 c1，複審期限（十天後）比 linked 的（七天後）晚
	// 待處理的請求：每個請求提出的時間依序遞增（收件匣依提出時間舊到新）；notDue 是解除請求，其餘是申覆。
	for _, m := range []Measure{linked, viaFinding, shared, unrelated} {
		f.now = f.now.Add(time.Minute)
		f.appeal(t, accountID, m, "申覆")
	}
	f.now = f.now.Add(time.Minute)
	f.remediate(t, accountID, notDue.ID, "已改善")
	f.now = start.Add(8 * 24 * time.Hour) // linked 與 unrelatedRisk 的複審期限（七天）已到，notDue 的（十天）還沒有

	ids := func(ms []Measure) []uuid.UUID {
		out := make([]uuid.UUID, len(ms))
		for i, m := range ms {
			out[i] = m.ID
		}
		return out
	}
	sorted := func(v []uuid.UUID) []uuid.UUID {
		out := slices.Clone(v)
		slices.SortFunc(out, uuid.UUID.Compare)
		return out
	}
	// list 讀一條路徑的全部頁（每頁 2 筆，翻頁不漏不重）。
	list := func(in AdminMeasureListInput) []uuid.UUID {
		t.Helper()
		in.PageSize = 2
		var out []uuid.UUID
		for {
			page, err := f.module.AdminMeasures(ctx, in)
			must(t, err)
			for _, m := range page.Items {
				out = append(out, m.ID)
			}
			if page.Next == nil {
				return out
			}
			in.After = page.Next
		}
	}
	pending := RequestPending
	paths := []struct {
		name  string
		apply func(*AdminMeasureListInput)
	}{
		{"清單", func(*AdminMeasureListInput) {}},
		{"待處理請求的收件匣", func(in *AdminMeasureListInput) { in.RequestState = &pending }},
		{"複審到期", func(in *AdminMeasureListInput) { in.ReviewDue = true }},
	}
	// 沒有來源篩選時，三條路徑各自的成員：確認路徑本身有效，下面的篩選才有意義。
	baseline := [][]Measure{
		{linked, viaFinding, cited, revoked, shared, linked2, unrelated, unrelatedRisk, notDue},
		{linked, viaFinding, shared, unrelated, notDue},
		{linked, unrelatedRisk},
	}
	for i, p := range paths {
		var in AdminMeasureListInput
		p.apply(&in)
		if got, want := sorted(list(in)), sorted(ids(baseline[i])); !slices.Equal(got, want) {
			t.Errorf("%s（沒有來源篩選）：%d 筆，預期 %d 筆", p.name, len(got), len(want))
		}
	}
	// 每個篩選在每條路徑上的預期：清單與複審到期比集合，收件匣依請求提出的時間（linked、viaFinding、shared、notDue）。
	for _, c := range []struct {
		name                  string
		in                    AdminMeasureListInput
		list, inbox, reviewed []Measure
	}{
		{"案件 c1", AdminMeasureListInput{SourceCaseID: &c1.ID}, []Measure{linked, viaFinding, cited, revoked, shared, notDue}, []Measure{linked, viaFinding, shared, notDue}, []Measure{linked}},
		{"案件 c2", AdminMeasureListInput{SourceCaseID: &c2.ID}, []Measure{shared, linked2}, []Measure{shared}, nil},
		{"通報 c1 的第一筆", AdminMeasureListInput{SourceReportID: &m1[0].ID}, []Measure{viaFinding, cited}, []Measure{viaFinding}, nil},
		{"通報 c1 的第二筆", AdminMeasureListInput{SourceReportID: &m1[1].ID}, []Measure{revoked}, nil, nil},
		{"通報 c1 的第三筆", AdminMeasureListInput{SourceReportID: &m1[2].ID}, []Measure{shared}, []Measure{shared}, nil},
		{"通報 c2 的第一筆", AdminMeasureListInput{SourceReportID: &m2[0].ID}, []Measure{shared}, []Measure{shared}, nil},
		{"通報 c2 的第二筆（沒有措施）", AdminMeasureListInput{SourceReportID: &m2[1].ID}, nil, nil, nil},
		{"案件 c2 且通報 c1 的第三筆（shared 兩個條件都符合）", AdminMeasureListInput{SourceCaseID: &c2.ID, SourceReportID: &m1[2].ID}, []Measure{shared}, []Measure{shared}, nil},
		{"案件 c2 且通報 c1 的第一筆（沒有措施同時符合）", AdminMeasureListInput{SourceCaseID: &c2.ID, SourceReportID: &m1[0].ID}, nil, nil, nil},
	} {
		for i, want := range [][]Measure{c.list, c.inbox, c.reviewed} {
			in := c.in
			paths[i].apply(&in)
			got, wantIDs := list(in), ids(want)
			if i != 1 { // 收件匣的順序是路徑自己的語意（請求提出時間舊到新），要逐筆一致；其餘比集合。
				got, wantIDs = sorted(got), sorted(wantIDs)
			}
			if !slices.Equal(got, wantIDs) {
				t.Errorf("%s／%s：%d 筆，預期 %d 筆", paths[i].name, c.name, len(got), len(wantIDs))
			}
		}
	}

	// 第四處：CaseMeasureCounts 的導出措施數與其中未撤銷的數目；沒有導出措施的案件不在結果裡（呼叫方當成 0）。
	for caseID, want := range map[uuid.UUID][]Measure{
		c1.ID: {linked, viaFinding, cited, revoked, shared, notDue},
		c2.ID: {shared, linked2},
	} {
		unrevoked := 0
		for _, m := range want {
			if f.measureOf(t, m.ID).Status == MeasureStatusEnforced {
				unrevoked++
			}
		}
		rows, err := store.New(f.db.Conn(ctx)).CaseMeasureCounts(ctx, []uuid.UUID{caseID})
		must(t, err)
		if len(rows) != 1 || int(rows[0].ResultingMeasures) != len(want) || int(rows[0].UnrevokedMeasures) != unrevoked {
			t.Errorf("CaseMeasureCounts(%s)：%+v，預期導出 %d 筆、未撤銷 %d 筆", caseID, rows, len(want), unrevoked)
		}
	}
	other, _ := f.newCase(t, f.anotherStore(t).StoreID, policy.Fraud, SeverityLow, 1)
	if rows, err := store.New(f.db.Conn(ctx)).CaseMeasureCounts(ctx, []uuid.UUID{other.ID}); err != nil || len(rows) != 0 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	// 兩個篩選可以和其他篩選組合。
	state := StateRevoked
	page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{SourceCaseID: &c1.ID, State: &state, PageSize: 100})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != revoked.ID {
		t.Fatalf("source_case_id 加 state=REVOKED：%+v", page.Items)
	}
}

// warningCitingExisting 建立一個警告，依據是引用 from 的既有認定（沒有新的認定）。
func (f *fixture) warningCitingExisting(t *testing.T, storeID int64, from Measure) Measure {
	t.Helper()
	r, err := f.module.CreateMeasure(context.Background(), CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: MeasureWarning, StatementToSeller: "y", Note: "x",
		Violation: &ViolationBasis{CitedFindingIDs: []uuid.UUID{from.Findings[0].ID}},
	})
	must(t, err)
	return r.Measure
}

// 從案件建立措施：任何種類與依據都可以連到案件；別家店的案件是 invalid_argument（SourceCaseID）、不存在的也是、已結案的是 failed_precondition；
// 連結之後的措施算案件導出的措施；同一個 request_id 換了案件是不同的請求。
func TestCreateMeasureCanLinkAnyKindToAnOpenCaseOfTheSameStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, other := f.open(t)
	c, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
	otherCase, _ := f.newCase(t, other.StoreID, policy.Fraud, SeverityLow, 1)
	review := f.now.Add(7 * 24 * time.Hour)
	ends := f.now.Add(72 * time.Hour)
	inputs := map[string]CreateMeasureInput{
		"警告（違規）": {Kind: MeasureWarning, Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow}}},
		"停權（違規）": {Kind: MeasureSuspension, EndsAt: &ends, Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow}}},
		"停權（風險）": {Kind: MeasureSuspension, ReviewBy: &review, Risk: &RiskBasis{Ground: RiskFoodSafety}},
		"終止（事由）": {Kind: MeasureTermination, Termination: &TerminationBasis{Ground: GroundFoodSafetyIncident}},
	}
	var linked []uuid.UUID
	for name, in := range inputs {
		in.AdminID, in.StoreID, in.StatementToSeller, in.Note, in.SourceCaseID = 7, s.StoreID, "理由", "註記", &c.ID
		if name == "終止（事由）" {
			continue // 終止會讓店家停權與關店，最後單獨測
		}
		r, err := f.module.CreateMeasure(ctx, in)
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if f.count(t, "SELECT count(*) FROM seller.measures WHERE id = $1 AND source_case_id = $2", r.Measure.ID, c.ID) != 1 {
			t.Fatalf("%s：沒有記下來源案件", name)
		}
		linked = append(linked, r.Measure.ID)
	}
	page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{SourceCaseID: &c.ID, PageSize: 100})
	must(t, err)
	if len(page.Items) != len(linked) {
		t.Fatalf("導出的措施 %d 筆，預期 %d", len(page.Items), len(linked))
	}
	// 別家店的案件、不存在的案件、已結案的案件。
	base := inputs["警告（違規）"]
	base.AdminID, base.StoreID, base.StatementToSeller, base.Note = 7, s.StoreID, "理由", "註記"
	wrong := base
	wrong.SourceCaseID = &otherCase.ID
	_, err = f.module.CreateMeasure(ctx, wrong)
	if got := invalidFields(t, err); !slices.Equal(got, []string{"SourceCaseID"}) || !strings.Contains(err.Error(), "案件不是這家店的。") {
		t.Fatalf("別家店的案件：%v", err)
	}
	missing := base
	missing.SourceCaseID = ptr(uuid.NewV7())
	_, err = f.module.CreateMeasure(ctx, missing)
	if got := invalidFields(t, err); !slices.Equal(got, []string{"SourceCaseID"}) || !strings.Contains(err.Error(), "找不到這個案件。") {
		t.Fatalf("不存在的案件：%v", err)
	}
	closedCase, _ := f.newCase(t, s.StoreID, policy.Privacy, SeverityLow, 1)
	f.riskFromCase(t, s.StoreID, closedCase.ID)
	_, err = f.decide(t, closedCase.ID, DecisionActionTaken, "")
	must(t, err)
	closed := base
	closed.SourceCaseID = &closedCase.ID
	_, err = f.module.CreateMeasure(ctx, closed)
	wantBlocked(t, err, "案件已結案，不能再從它建立措施。")
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != int64(len(linked)+1) {
		t.Fatalf("被拒絕的建立不得留下任何列：measures=%d", n)
	}
	// 同一個 request_id 換了案件是不同的請求；同一個案件重送回原本的措施。
	requestID := uuid.New()
	first := base
	first.RequestID, first.SourceCaseID = &requestID, &c.ID
	created, err := f.module.CreateMeasure(ctx, first)
	must(t, err)
	again, err := f.module.CreateMeasure(ctx, first)
	must(t, err)
	if !again.Replayed || again.Measure.ID != created.Measure.ID {
		t.Fatalf("同一個案件重送：%+v", again)
	}
	third, _ := f.newCase(t, s.StoreID, policy.Impersonation, SeverityLow, 1)
	changed := first
	changed.SourceCaseID = &third.ID
	_, err = f.module.CreateMeasure(ctx, changed)
	if got := invalidFields(t, err); !slices.Equal(got, []string{"RequestID"}) {
		t.Fatalf("同一個 request_id 換了案件要回「內容不同」：%v", err)
	}
	none := first
	none.SourceCaseID = nil
	_, err = f.module.CreateMeasure(ctx, none)
	if got := invalidFields(t, err); !slices.Equal(got, []string{"RequestID"}) {
		t.Fatalf("同一個 request_id 拿掉案件要回「內容不同」：%v", err)
	}
	// 終止合作也可以連到案件（任何依據）；案件因此有導出的措施。
	ter := inputs["終止（事由）"]
	ter.AdminID, ter.StoreID, ter.StatementToSeller, ter.Note, ter.SourceCaseID = 7, s.StoreID, "理由", "註記", &third.ID
	if _, err := f.module.CreateMeasure(ctx, ter); err != nil {
		t.Fatalf("終止合作連到案件：%v", err)
	}
	if got := f.caseOf(t, third.ID).Availability.DecisionOptions(); !slices.Equal(got, []Decision{DecisionActionTaken}) {
		t.Fatalf("連結了未撤銷的終止合作：%v", got)
	}
}

// can_cite_as_source 與 can_* 的表格：每個情境各嘗試一次每個動作，動作成功 ⇔ 可用性說可以。每個情境、每個動作都在新建立的店家與案件上做（動作會改狀態）。
// 涵蓋通報的每個原因（已有結果、已分案、還沒分案、不在進行中的案件、認定的來源、案件有導出措施而它是最後一筆、結果被否定、店家已關閉）與案件的每個原因。
func TestReportAvailabilityIsExactlyWhatTheCommandsAccept(t *testing.T) {
	t.Parallel()
	type scenario struct {
		name  string
		build func(t *testing.T, f *fixture) uuid.UUID
	}
	closeStore := func(t *testing.T, f *fixture, accountID int64) {
		t.Helper()
		_, err := f.module.CloseCurrent(context.Background(), accountID)
		must(t, err)
	}
	scenarios := []scenario{
		{"待分案", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			return f.reportBy(t, s.StoreID, policy.Fraud).ID
		}},
		{"待分案、店家已關閉", func(t *testing.T, f *fixture) uuid.UUID {
			a, s := f.open(t)
			id := f.reportBy(t, s.StoreID, policy.Fraud).ID
			closeStore(t, f, a)
			return id
		}},
		{"案件裡、單獨一筆、沒有措施", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			_, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
			return m[0].ID
		}},
		{"案件裡、兩筆、沒有措施", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			_, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
			return m[0].ID
		}},
		{"案件裡、單獨一筆、有未撤銷的措施", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			c, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
			f.riskFromCase(t, s.StoreID, c.ID)
			return m[0].ID
		}},
		{"案件裡、兩筆、有未撤銷的措施", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			c, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
			f.riskFromCase(t, s.StoreID, c.ID)
			return m[0].ID
		}},
		{"案件裡、單獨一筆、措施已撤銷", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			c, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
			f.revoke(t, f.riskFromCase(t, s.StoreID, c.ID))
			return m[0].ID
		}},
		{"案件裡、另一筆是已採取措施（措施已撤銷）", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			c, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 3)
			risk := f.riskFromCase(t, s.StoreID, c.ID)
			f.mustResolve(t, m[1].ID, OutcomeActionTaken)
			f.revoke(t, risk)
			return m[0].ID
		}},
		{"案件裡、是認定的來源", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			_, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
			f.warningCiting(t, s.StoreID, m[0].ID)
			return m[0].ID
		}},
		{"案件裡、店家已關閉", func(t *testing.T, f *fixture) uuid.UUID {
			a, s := f.open(t)
			_, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
			closeStore(t, f, a)
			return m[0].ID
		}},
		{"不開案件、結果是未違規", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			return f.mustResolve(t, f.reportBy(t, s.StoreID, policy.Fraud).ID, OutcomeNoViolation).ID
		}},
		{"不開案件、結果是重複（仍可當來源）", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			return f.mustResolve(t, f.reportBy(t, s.StoreID, policy.Fraud).ID, OutcomeDuplicate).ID
		}},
		{"不開案件、結果是濫報、店家已關閉", func(t *testing.T, f *fixture) uuid.UUID {
			a, s := f.open(t)
			id := f.mustResolve(t, f.reportBy(t, s.StoreID, policy.Fraud).ID, OutcomeAbusive).ID
			closeStore(t, f, a)
			return id
		}},
		{"案件裡、結果是內容不足", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			_, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
			return f.mustResolve(t, m[0].ID, OutcomeInsufficient).ID
		}},
		{"在已結案的案件裡（採取措施）", func(t *testing.T, f *fixture) uuid.UUID {
			_, s := f.open(t)
			c, m := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
			f.riskFromCase(t, s.StoreID, c.ID)
			_, err := f.decide(t, c.ID, DecisionActionTaken, "")
			must(t, err)
			return m[0].ID
		}},
	}
	reasons := map[ReportBlock]bool{}
	for _, sc := range scenarios {
		// 在同一個資料庫裡，每個動作各建一份新的情境。
		f := setup(t)
		probe := func(action string, available func(AdminReport) bool, run func(id uuid.UUID) error) {
			id := sc.build(t, f)
			r := f.reportOf(t, id)
			all := [][]ReportBlock{r.Availability.Triage, r.Availability.Move, r.Availability.CiteAsSource}
			for _, blocks := range r.Availability.Resolve {
				all = append(all, blocks)
			}
			for _, blocks := range all {
				for _, b := range blocks {
					reasons[b] = true
				}
			}
			err := run(id)
			if want := available(r); (err == nil) != want {
				t.Errorf("%s／%s：可用性 %v，指令 %v", sc.name, action, want, err)
			}
			// 被前置條件擋下的一律是 failed_precondition（店家已關閉時建立措施也是，句子同 CreateMeasure 的 STORE_CLOSED）。
			if !blockedProperly(err) {
				t.Errorf("%s／%s：被擋下要是 failed_precondition：%v", sc.name, action, err)
			}
			f.assertCaseInvariants(t)
		}
		probe("分案", func(r AdminReport) bool { return len(r.Availability.Triage) == 0 }, func(id uuid.UUID) error {
			_, err := f.triage(t, id, policy.Privacy, severityPtr(SeverityLow))
			return err
		})
		for _, o := range OutcomeValues() {
			probe("設定結果 "+string(o), func(r AdminReport) bool { return slices.Contains(r.Availability.AvailableOutcomes(), o) }, func(id uuid.UUID) error {
				_, err := f.resolve(t, id, o, "")
				return err
			})
		}
		probe("移動", func(r AdminReport) bool { return len(r.Availability.Move) == 0 }, func(id uuid.UUID) error {
			_, err := f.move(t, id, policy.OffPlatformTrade, severityPtr(SeverityLow))
			return err
		})
		probe("當認定的來源", func(r AdminReport) bool { return len(r.Availability.CiteAsSource) == 0 }, func(id uuid.UUID) error {
			r := f.reportOf(t, id)
			_, err := f.module.CreateMeasure(context.Background(), f.warningFrom(r.StoreID, id))
			return err
		})
		// 回傳的 can_* 與清單裡同一筆的相同。
		id := sc.build(t, f)
		single := f.reportOf(t, id)
		var listed AdminReport
		for _, r := range f.allReports(t, ReportListInput{StoreID: single.StoreID}) {
			if r.ID == id {
				listed = r
			}
		}
		if fmt.Sprint(listed.Availability) != fmt.Sprint(single.Availability) {
			t.Errorf("%s：清單與單筆的可用性不同：%v / %v", sc.name, listed.Availability, single.Availability)
		}
		// 「可以當認定的來源」篩選（SQL）與 Availability 的 CiteAsSource（Go）是同一個定義：這家店的清單帶上篩選，剛好留下沒有原因的通報。
		citable := idsOfReports(f.allReports(t, ReportListInput{StoreID: single.StoreID, CitableAsSource: true}))
		if in, can := slices.Contains(citable, id), len(single.Availability.CiteAsSource) == 0; in != can {
			t.Errorf("%s：篩選留下=%v，Availability 可以當來源=%v（原因 %v）", sc.name, in, can, single.Availability.CiteAsSource)
		}
		for _, r := range f.allReports(t, ReportListInput{StoreID: single.StoreID, CitableAsSource: true}) {
			if len(r.Availability.CiteAsSource) != 0 {
				t.Errorf("%s：篩選留下了不能當來源的通報 %s：%v", sc.name, r.ID, r.Availability.CiteAsSource)
			}
		}
	}
	for _, b := range ReportBlockValues() {
		if !reasons[b] {
			t.Errorf("沒有情境碰到原因 %s", b)
		}
	}
}

func TestCaseAvailabilityIsExactlyWhatTheCommandsAccept(t *testing.T) {
	t.Parallel()
	type built struct {
		caseID    uuid.UUID
		storeID   int64
		accountID int64
	}
	type scenario struct {
		name  string
		build func(t *testing.T, f *fixture) built
	}
	scenarios := []scenario{
		{"進行中、沒有措施（食安）", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, _ := f.newCase(t, s.StoreID, policy.FoodSafety, SeverityLow, 2)
			return built{c.ID, s.StoreID, a}
		}},
		{"進行中、有未撤銷的措施（詐欺）", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
			f.riskFromCase(t, s.StoreID, c.ID)
			return built{c.ID, s.StoreID, a}
		}},
		{"進行中、措施已撤銷（食安）", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, _ := f.newCase(t, s.StoreID, policy.FoodSafety, SeverityLow, 2)
			f.revoke(t, f.riskFromCase(t, s.StoreID, c.ID))
			return built{c.ID, s.StoreID, a}
		}},
		{"進行中、成員是已採取措施（措施已撤銷）", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, m := f.newCase(t, s.StoreID, policy.Counterfeit, SeverityLow, 2)
			risk := f.riskFromCase(t, s.StoreID, c.ID)
			f.mustResolve(t, m[0].ID, OutcomeActionTaken)
			f.revoke(t, risk)
			return built{c.ID, s.StoreID, a}
		}},
		{"進行中、店家已關閉", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, _ := f.newCase(t, s.StoreID, policy.FoodSafety, SeverityLow, 1)
			_, err := f.module.CloseCurrent(context.Background(), a)
			must(t, err)
			return built{c.ID, s.StoreID, a}
		}},
		{"已結案：決定採取措施（食安）", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, _ := f.newCase(t, s.StoreID, policy.FoodSafety, SeverityLow, 1)
			f.riskFromCase(t, s.StoreID, c.ID)
			_, err := f.decide(t, c.ID, DecisionActionTaken, "")
			must(t, err)
			return built{c.ID, s.StoreID, a}
		}},
		{"已結案：決定不採取措施（詐欺）", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
			_, err := f.decide(t, c.ID, DecisionNoAction, "")
			must(t, err)
			return built{c.ID, s.StoreID, a}
		}},
		{"已結案：以沒有通報關閉（食安）", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, m := f.newCase(t, s.StoreID, policy.FoodSafety, SeverityLow, 1)
			_, err := f.move(t, m[0].ID, policy.Fraud, severityPtr(SeverityLow))
			must(t, err)
			return built{c.ID, s.StoreID, a}
		}},
		{"進行中、有還沒結束的詢問", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, _ := f.newCase(t, s.StoreID, policy.FoodSafety, SeverityLow, 1)
			f.mustInquire(t, c.ID)
			return built{c.ID, s.StoreID, a}
		}},
		{"進行中、店家個資已清除", func(t *testing.T, f *fixture) built {
			a, s := f.open(t)
			c, _ := f.newCase(t, s.StoreID, policy.FoodSafety, SeverityLow, 1)
			_, err := f.module.CloseCurrent(context.Background(), a)
			must(t, err)
			f.now = f.now.Add(StoreRetention + time.Second)
			_, err = f.module.PurgePII(context.Background())
			must(t, err)
			return built{c.ID, s.StoreID, a}
		}},
	}
	reasons := map[CaseBlock]bool{}
	for _, sc := range scenarios {
		f := setup(t)
		ctx := context.Background()
		probe := func(action string, available func(AdminCase) bool, run func(b built) error) {
			b := sc.build(t, f)
			c := f.caseOf(t, b.caseID)
			av := c.Availability
			for _, blocks := range [][]CaseBlock{av.ChangeSeverity, av.LinkMeasure, av.CreateMeasure, av.CreateInquiry, av.Decide[DecisionActionTaken], av.Decide[DecisionNoAction]} {
				for _, blk := range blocks {
					reasons[blk] = true
				}
			}
			err := run(b)
			if want := available(c); (err == nil) != want {
				t.Errorf("%s／%s：可用性 %v，指令 %v", sc.name, action, want, err)
			}
			if !blockedProperly(err) {
				t.Errorf("%s／%s：被擋下要是 failed_precondition：%v", sc.name, action, err)
			}
			f.assertCaseInvariants(t)
		}
		for _, d := range DecisionValues() {
			probe("決定 "+string(d), func(c AdminCase) bool { return len(c.Availability.Decide[d]) == 0 }, func(b built) error {
				_, err := f.decide(t, b.caseID, d, "")
				return err
			})
		}
		probe("改嚴重度", func(c AdminCase) bool { return len(c.Availability.ChangeSeverity) == 0 }, func(b built) error {
			_, err := f.module.ChangeCaseSeverity(ctx, ChangeCaseSeverityInput{CaseID: b.caseID, AdminID: 11, Severity: SeverityCritical, Note: "原因", ExpectedVersion: f.caseOf(t, b.caseID).Version})
			return err
		})
		probe("建立警告（連結案件）", func(c AdminCase) bool { return len(c.Availability.CreateMeasure) == 0 }, func(b built) error {
			in := f.warningFrom(b.storeID)
			in.SourceCaseID = &b.caseID
			_, err := f.module.CreateMeasure(ctx, in)
			return err
		})
		probe("連結終止合作", func(c AdminCase) bool { return len(c.Availability.LinkMeasure) == 0 }, func(b built) error {
			_, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
				AdminID: 7, StoreID: b.storeID, Kind: MeasureTermination, StatementToSeller: "理由", Note: "註記", SourceCaseID: &b.caseID,
				Termination: &TerminationBasis{Ground: GroundFoodSafetyIncident},
			})
			return err
		})
		probe("發詢問", func(c AdminCase) bool { return len(c.Availability.CreateInquiry) == 0 }, func(b built) error {
			_, err := f.inquire(t, b.caseID, InquiryKindRecords, "請提供紀錄")
			return err
		})
	}
	for _, b := range CaseBlockValues() {
		if !reasons[b] {
			t.Errorf("沒有情境碰到原因 %s", b)
		}
	}
}

// 隨機的指令序列（固定種子）：每個動作都先看可用性，指令成功 ⇔ 可用性說可以；每個指令之後全部案件的跨列不變量都成立（I1–I6）。
func TestRandomCommandSequencesKeepTheInvariantsAndAvailabilityHonest(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	rng := rand.New(rand.NewPCG(20261018, 3)) //nolint:gosec // 固定種子的確定性隨機指令序列，不是安全用途
	var stores []Store
	var accounts []int64
	for range 3 {
		a, s := f.open(t)
		accounts, stores = append(accounts, a), append(stores, s)
	}
	closed := map[int64]bool{}
	categories := slices.Clone(policy.Assessable())
	reporterCategories := policy.ReporterSelectable()
	pick := func(n int) int { return rng.IntN(n) }
	reports := func() []AdminReport { return f.allReports(t, ReportListInput{}) }
	cases := func() []AdminCase { return f.allCases(t, CaseListInput{}) }
	mismatches := 0
	check := func(step int, action string, available bool, err error) {
		t.Helper()
		if (err == nil) != available {
			mismatches++
			t.Errorf("步驟 %d／%s：可用性 %v，指令 %v", step, action, available, err)
		}
		// 被前置條件擋下的一律是 failed_precondition，店家已關閉時建立措施也是。
		if !blockedProperly(err) {
			t.Errorf("步驟 %d／%s：被擋下要是 failed_precondition：%v", step, action, err)
		}
	}
	openCaseOf := func(storeID int64, category policy.Category) bool {
		for _, c := range cases() {
			if c.StoreID == storeID && c.Category == category && c.Open() {
				return true
			}
		}
		return false
	}
	for step := range 260 {
		s := stores[pick(len(stores))]
		switch pick(11) {
		case 0, 1:
			f.reportBy(t, s.StoreID, reporterCategories[pick(len(reporterCategories))])
		case 2:
			all := reports()
			if len(all) == 0 {
				continue
			}
			r := all[pick(len(all))]
			category := categories[pick(len(categories))]
			var severity *Severity
			if !openCaseOf(r.StoreID, category) {
				severity = severityPtr(SeverityValues()[pick(4)])
			}
			_, err := f.triage(t, r.ID, category, severity)
			check(step, "分案", len(r.Availability.Triage) == 0, err)
		case 3, 4:
			all := reports()
			if len(all) == 0 {
				continue
			}
			r := all[pick(len(all))]
			o := OutcomeValues()[pick(5)]
			_, err := f.resolve(t, r.ID, o, "")
			check(step, "設定結果 "+string(o), slices.Contains(r.Availability.AvailableOutcomes(), o), err)
		case 5:
			all := reports()
			if len(all) == 0 {
				continue
			}
			r := all[pick(len(all))]
			category := categories[pick(len(categories))]
			if r.AssessedCategory != nil && category == *r.AssessedCategory {
				continue
			}
			var severity *Severity
			if !openCaseOf(r.StoreID, category) {
				severity = severityPtr(SeverityValues()[pick(4)])
			}
			_, err := f.move(t, r.ID, category, severity)
			check(step, "移動", len(r.Availability.Move) == 0, err)
		case 6:
			all := cases()
			if len(all) == 0 {
				continue
			}
			c := all[pick(len(all))]
			d := DecisionValues()[pick(2)]
			_, err := f.decide(t, c.ID, d, "")
			check(step, "決定 "+string(d), slices.Contains(c.Availability.DecisionOptions(), d), err)
		case 7:
			all := cases()
			if len(all) == 0 {
				continue
			}
			c := all[pick(len(all))]
			in := f.warningFrom(c.StoreID)
			in.SourceCaseID = &c.ID
			_, err := f.module.CreateMeasure(ctx, in)
			check(step, "建立警告（連結案件）", len(c.Availability.CreateMeasure) == 0, err)
		case 8:
			all := reports()
			if len(all) == 0 {
				continue
			}
			r := all[pick(len(all))]
			_, err := f.module.CreateMeasure(ctx, f.warningFrom(r.StoreID, r.ID))
			check(step, "當認定的來源", len(r.Availability.CiteAsSource) == 0, err)
		case 9:
			page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{StoreID: s.StoreID, PageSize: 100})
			must(t, err)
			for _, m := range page.Items {
				if m.Status == MeasureStatusEnforced && pick(2) == 0 {
					f.revoke(t, m)
					break
				}
			}
		case 10:
			if i := slices.IndexFunc(stores, func(x Store) bool { return x.StoreID == s.StoreID }); !closed[s.StoreID] && pick(8) == 0 {
				_, err := f.module.CloseCurrent(ctx, accounts[i])
				must(t, err)
				closed[s.StoreID] = true
			}
		}
		f.assertCaseInvariants(t)
	}
	if mismatches != 0 {
		t.Fatalf("可用性與指令不一致 %d 次", mismatches)
	}
	if n := len(cases()); n < 5 {
		t.Fatalf("隨機序列只開了 %d 個案件，覆蓋太薄", n)
	}
}

// blockedProperly 回報指令的結果是否合乎「被擋下」的約定：沒有錯誤（做成了），或前置條件擋下的 failed_precondition（店家已關閉時建立措施也是）。
func blockedProperly(err error) bool {
	return err == nil || apperr.KindOf(err) == apperr.KindFailedPrecondition
}
