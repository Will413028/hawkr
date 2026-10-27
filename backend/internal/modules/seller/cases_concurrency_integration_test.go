//go:build integration

package seller_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func isDeadlock(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "40P01") || strings.Contains(err.Error(), "deadlock"))
}

// race 同時執行兩個函式並回傳各自的錯誤。
func race(a, b func() error) (error, error) {
	var ea, eb error
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Go(func() { <-start; ea = a() })
	wg.Go(func() { <-start; eb = b() })
	close(start)
	wg.Wait()
	return ea, eb
}

// 同一家店、同一個類別同時分案：店家列的鎖讓它們串行，只有一個開新案件（其餘看到進行中的案件，開新案件的意圖不成立所以是案件版本衝突，不是 23505）；
// cases_one_open 只是保底。
func TestConcurrentTriageOfTheSameStoreAndCategoryOpensExactlyOneCase(t *testing.T) {
	t.Parallel()
	const workers = 6
	f := setup(t)
	_, s := f.open(t)
	reports := make([]uuid.UUID, workers)
	for i := range reports {
		reports[i] = f.reportBy(t, s.StoreID, policy.Fraud).ID
	}
	ctx := context.Background()
	errs := make([]error, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range workers {
		wg.Go(func() {
			<-start
			sev := SeverityHigh
			_, errs[i] = f.module.TriageReport(ctx, TriageReportInput{
				ReportID: reports[i], AdminID: int64(20 + i), AssessedCategory: policy.Fraud, Target: CaseTarget{OpenNew: &sev}, ExpectedVersion: 1,
			})
		})
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case apperr.KindOf(err) != apperr.KindVersionConflict:
			t.Fatalf("其餘的要看到進行中的案件而被拒絕（案件版本衝突），不是資料庫錯誤：%v", err)
		}
	}
	if ok != 1 || f.count(t, "SELECT count(*) FROM seller.cases WHERE store_id = $1 AND closed_at IS NULL", s.StoreID) != 1 {
		t.Fatalf("開了 %d 個案件，errs=%v", ok, errs)
	}
	f.assertCaseInvariants(t)
}

// 結案對同時加入案件的分案：結案前已加入的成員被帶入結果；結案之後才到的，要加入的案件已經關閉（案件版本衝突）。
// 任何一輪都不能留下「已結案卻有成員沒有結果」（I2）。
func TestDecideAndAConcurrentTriageNeverLeaveAnUnresolvedMemberInAClosedCase(t *testing.T) {
	t.Parallel()
	const rounds = 200
	f := setup(t)
	ctx := context.Background()
	for i := range rounds {
		_, s := f.open(t)
		c, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
		late := f.reportBy(t, s.StoreID, policy.Fraud)
		version := f.caseOf(t, c.ID).Version
		lateVersion := f.reportOf(t, late.ID).Version
		a, b := race(func() error {
			_, err := f.module.DecideCase(ctx, DecideCaseInput{CaseID: c.ID, AdminID: 10, Decision: DecisionNoAction, ExpectedVersion: version})
			return err
		}, func() error {
			_, err := f.module.TriageReport(ctx, TriageReportInput{
				ReportID: late.ID, AdminID: 7, AssessedCategory: policy.Fraud, Target: CaseTarget{Join: &JoinCase{CaseID: c.ID, ExpectedVersion: version}}, ExpectedVersion: lateVersion,
			})
			return err
		})
		if isDeadlock(a) || isDeadlock(b) {
			t.Fatalf("round %d: deadlock %v / %v", i, a, b)
		}
		// 結案可能因為 triage 先加入而版本衝突（案件的版本前進）；重讀再結一次也行。要緊的是不變量。
		if a != nil && apperr.KindOf(a) != apperr.KindVersionConflict {
			t.Fatalf("round %d: decide %v", i, a)
		}
		if b != nil && apperr.KindOf(b) != apperr.KindVersionConflict {
			t.Fatalf("round %d: triage %v", i, b)
		}
		if n := f.count(t, "SELECT count(*) FROM seller.reports r JOIN seller.cases c ON c.id = r.case_id WHERE c.closed_at IS NOT NULL AND r.outcome IS NULL"); n != 0 {
			t.Fatalf("round %d: 已結案的案件有 %d 筆沒有結果的成員", i, n)
		}
	}
	f.assertCaseInvariants(t)
}

// 結案對補充、移動對補充：不互鎖。結案先鎖全部成員通報，補充只鎖原通報；補充在原通報被結案之後到的，找到的那一筆已有結果，這次是新的檢舉。
func TestDecideAndMoveNeverDeadlockWithASupplementToTheSameReport(t *testing.T) {
	t.Parallel()
	const rounds = 200
	f := setup(t)
	ctx := context.Background()
	for i := range rounds {
		_, s := f.open(t)
		c, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
		buyer := *f.reportOf(t, members[0].ID).ReporterBuyerID
		version := f.caseOf(t, c.ID).Version
		supplement := func() error {
			_, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: buyer, Category: policy.Fraud, Description: "補充"})
			return err
		}
		a, b := race(func() error {
			_, err := f.module.DecideCase(ctx, DecideCaseInput{CaseID: c.ID, AdminID: 10, Decision: DecisionNoAction, ExpectedVersion: version})
			return err
		}, supplement)
		if isDeadlock(a) || isDeadlock(b) {
			t.Fatalf("round %d: deadlock %v / %v", i, a, b)
		}
		if a != nil || b != nil {
			t.Fatalf("round %d: 結案與補充都要成功（補充不改案件的版本）：decide=%v supplement=%v", i, a, b)
		}
		// 補充要嘛併進原通報（結案帶入的結果之前），要嘛因為原通報已有結果而成為新的檢舉：不能不見。
		if n := f.count(t, "SELECT count(*) FROM seller.report_supplements WHERE report_id = $1", members[0].ID) + f.count(t, "SELECT count(*) FROM seller.reports WHERE reporter_buyer_id = $1 AND id <> $2", buyer, members[0].ID); n != 1 {
			t.Fatalf("round %d: 補充不見了或重複了：%d", i, n)
		}

		// 移動對補充。
		_, s2 := f.open(t)
		_, m2 := f.newCase(t, s2.StoreID, policy.Counterfeit, SeverityLow, 2)
		buyer2 := *f.reportOf(t, m2[0].ID).ReporterBuyerID
		moveVersion := f.reportOf(t, m2[0].ID).Version
		a, b = race(func() error {
			sev := SeverityLow
			_, err := f.module.MoveReport(ctx, MoveReportInput{ReportID: m2[0].ID, AdminID: 9, AssessedCategory: policy.Privacy, Target: CaseTarget{OpenNew: &sev}, ExpectedVersion: moveVersion})
			return err
		}, func() error {
			_, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s2.StoreID, ReporterBuyerID: buyer2, Category: policy.Fraud, Description: "補充"})
			return err
		})
		if isDeadlock(a) || isDeadlock(b) {
			t.Fatalf("round %d: deadlock %v / %v", i, a, b)
		}
		// 補充先 commit 時通報的版本前進，移動撞版本；移動先時兩個都成功。
		if a != nil && apperr.KindOf(a) != apperr.KindVersionConflict || b != nil {
			t.Fatalf("round %d: move=%v supplement=%v", i, a, b)
		}
	}
	f.assertCaseInvariants(t)
}

// 分案對建立措施、結案對帶 source_case_id 的建立措施：不互鎖。後者互斥：建立措施連到案件與決定不採取措施只有一個能成功
// （不採取措施的案件不會有未撤銷的導出措施），否則「告訴檢舉人未違規，賣家卻有生效中的措施」。
func TestTriageAndDecideNeverDeadlockWithCreatingMeasures(t *testing.T) {
	t.Parallel()
	const rounds = 200
	f := setup(t)
	ctx := context.Background()
	for i := range rounds {
		_, s := f.open(t)
		report := f.reportBy(t, s.StoreID, policy.Fraud)
		sev := SeverityLow
		a, b := race(func() error {
			_, err := f.module.TriageReport(ctx, TriageReportInput{ReportID: report.ID, AdminID: 7, AssessedCategory: policy.Fraud, Target: CaseTarget{OpenNew: &sev}, ExpectedVersion: report.Version})
			return err
		}, func() error {
			_, err := f.module.CreateMeasure(ctx, f.warningFrom(s.StoreID, report.ID))
			return err
		})
		if isDeadlock(a) || isDeadlock(b) || a != nil || b != nil {
			t.Fatalf("round %d: triage=%v create=%v", i, a, b)
		}

		_, s2 := f.open(t)
		c, _ := f.newCase(t, s2.StoreID, policy.Counterfeit, SeverityLow, 1)
		version := f.caseOf(t, c.ID).Version
		a, b = race(func() error {
			_, err := f.module.DecideCase(ctx, DecideCaseInput{CaseID: c.ID, AdminID: 10, Decision: DecisionNoAction, ExpectedVersion: version})
			return err
		}, func() error {
			in := f.warningFrom(s2.StoreID)
			in.SourceCaseID = &c.ID
			_, err := f.module.CreateMeasure(ctx, in)
			return err
		})
		if isDeadlock(a) || isDeadlock(b) {
			t.Fatalf("round %d: deadlock %v / %v", i, a, b)
		}
		if (a == nil) == (b == nil) {
			t.Fatalf("round %d: 決定不採取措施與連到案件的建立措施只有一個能成功：decide=%v create=%v", i, a, b)
		}
		if n := f.count(t, "SELECT count(*) FROM seller.cases c JOIN seller.measures m ON m.source_case_id = c.id WHERE c.decision = 'NO_ACTION' AND m.status = 'ENFORCED'"); n != 0 {
			t.Fatalf("round %d: 不採取措施的案件有未撤銷的導出措施", i)
		}
	}
	f.assertCaseInvariants(t)
}

// 補充不鎖案件、不改案件的版本：結案的版本檢查不會被補充打斷（補充只讓案件顯示「有新的補充」）。
func TestASupplementNeverInvalidatesTheCaseVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	c, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
	buyer := *f.reportOf(t, members[0].ID).ReporterBuyerID
	for range 3 {
		f.now = f.now.Add(time.Minute)
		if r, err := f.module.SubmitReport(context.Background(), SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: buyer, Category: policy.Fraud, Description: "補充"}); err != nil || !r.Supplemented {
			t.Fatalf("r=%+v err=%v", r, err)
		}
	}
	after := f.caseOf(t, c.ID)
	if after.Version != c.Version || after.UnseenSupplementReports == 0 {
		t.Fatalf("案件的版本不變、顯示有新的補充：%+v", after.Case)
	}
	if _, err := f.module.DecideCase(context.Background(), DecideCaseInput{CaseID: c.ID, AdminID: 10, Decision: DecisionNoAction, ExpectedVersion: c.Version}); err != nil {
		t.Fatalf("以補充之前讀到的案件版本結案仍成立（補充由 unseen_supplement_report_count 提醒，不擋結案）：%v", err)
	}
}
