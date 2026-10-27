package seller

import (
	"testing"
	"time"
	"uuid"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// wantWire 斷言 emitter 把模組給的期限原樣轉成 Deadline（開始與期限各自對應，不對調、不自己算時長）；want 為 nil 時欄位不設。
func wantWire(t *testing.T, label string, got *commonpb.Deadline, want *model.Deadline) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("%s：義務不在，欄位不該設：%v", label, got)
		}
		return
	}
	if got == nil {
		t.Errorf("%s：義務還在，欄位要有值", label)
		return
	}
	if !got.GetStartTime().AsTime().Equal(want.Start) || !got.GetDueTime().AsTime().Equal(want.Due) {
		t.Errorf("%s：開始 %v、期限 %v，預期 %v、%v", label, got.GetStartTime().AsTime(), got.GetDueTime().AsTime(), want.Start, want.Due)
	}
}

// 每個 *_deadline 在義務還在時有值（開始與期限互不相同，對調會被抓到），義務結束後（模組沒有給）沒有。
func TestEveryDeadlineFieldIsEmittedOnlyWhileTheObligationExists(t *testing.T) {
	start := governanceNow
	// 每一個欄位用不同的偏移，其中一個欄位的值放到另一個欄位會被發現。
	dl := func(startOffset, dueOffset time.Duration) *model.Deadline {
		return &model.Deadline{Start: start.Add(startOffset), Due: start.Add(dueOffset)}
	}

	// 通報
	report := anonymousFoodSafetyReport()
	report.TriageDeadline = dl(time.Minute, 4*time.Hour)
	wantWire(t, "Report.triage_deadline", toReport(report).GetTriageDeadline(), report.TriageDeadline)
	report.TriageDeadline = nil
	wantWire(t, "Report.triage_deadline（分案後）", toReport(report).GetTriageDeadline(), nil)

	// 案件：決定期限。
	c := caseFixture(model.CaseFacts{})
	c.DecideDeadline = dl(2*time.Minute, 72*time.Hour)
	wantWire(t, "Case.decide_deadline", toCase(c).GetDecideDeadline(), c.DecideDeadline)
	c.DecideDeadline = nil
	wantWire(t, "Case.decide_deadline（結案後）", toCase(c).GetDecideDeadline(), nil)

	// 措施的複審期限與請求的裁定期限。
	m := riskMeasureFixture()
	m.ReviewDeadline = dl(4*time.Minute, 7*24*time.Hour)
	wantWire(t, "Measure.review_deadline", toAdminMeasure(m).GetReviewDeadline(), m.ReviewDeadline)
	m.ReviewDeadline = nil
	wantWire(t, "Measure.review_deadline（結束後）", toAdminMeasure(m).GetReviewDeadline(), nil)
	request := model.MeasureRequest{ID: uuid.NewV7(), Kind: model.RequestKindAppeal, State: model.RequestPending, SubmittedAt: start, DecideDeadline: dl(0, 7*24*time.Hour)}
	wantWire(t, "MeasureRequest.decide_deadline", toAdminRequest(request).GetDecideDeadline(), request.DecideDeadline)
	request.DecideDeadline = nil
	wantWire(t, "MeasureRequest.decide_deadline（裁定後）", toAdminRequest(request).GetDecideDeadline(), nil)
}
