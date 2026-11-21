//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

const reporterBuyer, reporterAccount = int64(5), int64(1005)

func (f *fixture) reporterReport(t *testing.T, storeID int64) SubmitReportResult {
	t.Helper()
	f.everExposed(t, storeID)
	r, err := f.module.SubmitReport(context.Background(), SubmitReportInput{
		StoreID: storeID, ReporterBuyerID: reporterBuyer, AccountID: reporterAccount, Category: policy.Fraud, Description: "原本", EvidenceTokens: []string{"e1"},
	})
	must(t, err)
	return r
}

func (f *fixture) addSupplement(reportID uuid.UUID, description string, tokens ...string) (Report, error) {
	return f.module.AddReportSupplement(context.Background(), AddSupplementInput{
		ReportID: reportID, BuyerID: reporterBuyer, AccountID: reporterAccount, Description: description, EvidenceTokens: tokens,
	})
}

// 主動補充與去重併入的補充走同一條寫入路徑：同一串（依時間）、同樣發帶 SupplementID 的 ReportReceived、admin 的「有新的補充」成立、佐證位置接續。
func TestReporterSupplementUsesTheSamePathAsTheDedupeSupplement(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	received := record[ReportReceived](f)
	_, s := f.open(t)
	original := f.reporterReport(t, s.StoreID)
	f.mustTriage(t, original.ReportID, policy.Fraud, severityPtr(SeverityMedium))
	if f.reportOf(t, original.ReportID).HasUnseenSupplements() {
		t.Fatal("還沒有補充")
	}
	f.jobs.args = nil
	got, err := f.addSupplement(original.ReportID, "  主動補充  ", "e2")
	must(t, err)
	if len(got.Supplements) != 1 || got.Supplements[0].Description != "主動補充" || !slices.Equal(got.Supplements[0].EvidencePositions, []int{1}) || len(got.Evidence) != 2 {
		t.Fatalf("got=%+v", got)
	}
	if !slices.Contains(f.jobs.kinds(), "seller.media_sync") {
		t.Fatalf("補充的佐證要排入引用的同步：%v", f.jobs.kinds())
	}
	dedupe, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: reporterBuyer, AccountID: reporterAccount, Category: policy.Fraud, Description: "去重併入"})
	must(t, err)
	if !dedupe.Supplemented {
		t.Fatalf("dedupe=%+v", dedupe)
	}
	detail, err := f.module.BuyerReport(ctx, reporterBuyer, original.ReportID)
	must(t, err)
	if len(detail.Supplements) != 2 || detail.Supplements[0].Description != "主動補充" || detail.Supplements[1].Description != "去重併入" {
		t.Fatalf("兩種來源是同一串：%+v", detail.Supplements)
	}
	admin := f.reportOf(t, original.ReportID)
	if !admin.HasUnseenSupplements() || admin.SupplementCount != 2 {
		t.Fatalf("admin 看得到同一串補充且有新的補充：%+v", admin)
	}
	events := received.all()
	if len(events) != 3 || events[1].SupplementID == nil || *events[1].SupplementID != detail.Supplements[0].ID || events[1].ReportID != original.ReportID ||
		events[1].ReporterBuyerID != reporterBuyer || events[1].Category != policy.Fraud {
		t.Fatalf("主動補充要發帶 SupplementID 的 ReportReceived：%+v", events)
	}
}

// 兩種來源共用同一個 10 則上限：合計第 11 則（不論從哪條入口）回 failed_precondition，什麼都不存、不發事件。
func TestReporterAndDedupeSupplementsShareTheTenSupplementCap(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	received := record[ReportReceived](f)
	_, s := f.open(t)
	original := f.reporterReport(t, s.StoreID)
	for i := range 5 {
		if _, err := f.addSupplement(original.ReportID, fmt.Sprintf("主動 %d", i+1)); err != nil {
			t.Fatalf("主動第 %d 則：%v", i+1, err)
		}
		if r, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: reporterBuyer, AccountID: reporterAccount, Category: policy.Fraud, Description: fmt.Sprintf("去重 %d", i+1)}); err != nil || !r.Supplemented {
			t.Fatalf("去重第 %d 則：%+v %v", i+1, r, err)
		}
	}
	_, err := f.addSupplement(original.ReportID, "第 11 則", "over")
	wantBlocked(t, err, "這筆檢舉已經補充 10 次，不能再補充。")
	if n := f.count(t, "SELECT count(*) FROM seller.report_supplements WHERE report_id = $1", original.ReportID); n != MaxSupplements {
		t.Fatalf("supplements=%d", n)
	}
	if got := len(received.all()); got != 1+MaxSupplements {
		t.Fatalf("被擋下的不發事件：%d", got)
	}
}

// 檢舉已經有結果就不能再補充（failed_precondition），什麼都不存。
func TestReporterCannotSupplementAReportThatHasAnOutcome(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	original := f.reporterReport(t, s.StoreID)
	f.mustResolve(t, original.ReportID, OutcomeInsufficient)
	_, err := f.addSupplement(original.ReportID, "太晚了")
	wantKind(t, err, apperr.KindFailedPrecondition)
	if n := f.count(t, "SELECT count(*) FROM seller.report_supplements WHERE report_id = $1", original.ReportID); n != 0 {
		t.Fatalf("supplements=%d", n)
	}
}

// 別人的與不存在的檢舉 id 一律 not_found（讀與補充都是），不洩漏存在與否；別人的檢舉不會被轉正佐證、不會被寫入。
func TestReporterOnlySeesAndSupplementsTheirOwnReports(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	received := record[ReportReceived](f)
	_, s := f.open(t)
	original := f.reporterReport(t, s.StoreID)
	const stranger = int64(6)
	f.media.finalized = nil
	_, err := f.module.BuyerReport(ctx, stranger, original.ReportID)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.module.BuyerReport(ctx, reporterBuyer, uuid.NewV7())
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.module.AddReportSupplement(ctx, AddSupplementInput{ReportID: original.ReportID, BuyerID: stranger, AccountID: 1006, Description: "別人", EvidenceTokens: []string{"x"}})
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.module.AddReportSupplement(ctx, AddSupplementInput{ReportID: uuid.NewV7(), BuyerID: reporterBuyer, AccountID: reporterAccount, Description: "不存在"})
	wantKind(t, err, apperr.KindNotFound)
	if n := f.count(t, "SELECT count(*) FROM seller.report_supplements"); n != 0 || len(f.media.finalized) != 0 || len(received.all()) != 1 {
		t.Fatalf("別人的 id 不寫入、不轉正佐證、不發事件：supplements=%d finalized=%v events=%d", n, f.media.finalized, len(received.all()))
	}
	got, err := f.module.BuyerReport(ctx, reporterBuyer, original.ReportID)
	must(t, err)
	if got.ID != original.ReportID || len(got.Evidence) != 1 || got.Description != "原本" {
		t.Fatalf("got=%+v", got)
	}
}

// 補充的輸入驗證：內容 1 到 2000 字、佐證最多 5 張。
func TestReporterSupplementValidatesTheInput(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	original := f.reporterReport(t, s.StoreID)
	for name, in := range map[string]AddSupplementInput{
		"空內容":  {Description: "  "},
		"過長":   {Description: strings.Repeat("字", 2001)},
		"佐證過多": {Description: "x", EvidenceTokens: []string{"1", "2", "3", "4", "5", "6"}},
	} {
		in.ReportID, in.BuyerID, in.AccountID = original.ReportID, reporterBuyer, reporterAccount
		_, err := f.module.AddReportSupplement(context.Background(), in)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s：%v", name, err)
		}
	}
}
