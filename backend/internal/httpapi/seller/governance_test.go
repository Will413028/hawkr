package seller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// governanceModule 記下每個指令收到的輸入，回傳設定好的通報、案件與措施。
type governanceModule struct {
	GovernanceModule
	report      model.AdminReport
	cases       model.AdminCase
	measure     model.Measure
	violation   model.Violation
	reportList  model.ReportListInput
	reportPage  model.AdminReportPage
	caseList    model.CaseListInput
	casePage    model.AdminCasePage
	measureList model.AdminMeasureListInput
	violList    model.ViolationListInput
	noteList    model.MeasureNoteListInput
	caseNotes   model.CaseNoteListInput
	notePage    model.NotePage
	triage      model.TriageReportInput
	resolve     model.ResolveReportInput
	markSeen    model.MarkSupplementsSeenInput
	move        model.MoveReportInput
	moveResult  model.MoveReportResult
	decideCase  model.DecideCaseInput
	severity    model.ChangeCaseSeverityInput
	caseNote    model.CreateCaseNoteInput
	create      model.CreateMeasureInput
	update      model.UpdateMeasureInput
	end         model.EndMeasureInput
	note        model.CreateMeasureNoteInput
	revoke      model.RevokeMeasureInput
	review      model.ScheduleReviewInput
	decide      model.DecideAppealInput
	remediate   model.DecideRemediationInput
	createErr   error
	calls       int
}

func (m *governanceModule) AdminReports(_ context.Context, in model.ReportListInput) (model.AdminReportPage, error) {
	m.reportList = in
	return m.reportPage, nil
}

func (m *governanceModule) Report(context.Context, uuid.UUID) (model.AdminReport, error) {
	return m.report, nil
}

func (m *governanceModule) TriageReport(_ context.Context, in model.TriageReportInput) (model.AdminReport, error) {
	m.triage, m.calls = in, m.calls+1
	return m.report, nil
}

func (m *governanceModule) MarkReportSupplementsSeen(_ context.Context, in model.MarkSupplementsSeenInput) (model.AdminReport, error) {
	m.markSeen, m.calls = in, m.calls+1
	return m.report, nil
}

func (m *governanceModule) ResolveReport(_ context.Context, in model.ResolveReportInput) (model.AdminReport, error) {
	m.resolve, m.calls = in, m.calls+1
	return m.report, nil
}

func (m *governanceModule) MoveReport(_ context.Context, in model.MoveReportInput) (model.MoveReportResult, error) {
	m.move, m.calls = in, m.calls+1
	return m.moveResult, nil
}

func (m *governanceModule) AdminCases(_ context.Context, in model.CaseListInput) (model.AdminCasePage, error) {
	m.caseList = in
	return m.casePage, nil
}

func (m *governanceModule) Case(context.Context, uuid.UUID) (model.AdminCase, error) {
	return m.cases, nil
}

func (m *governanceModule) DecideCase(_ context.Context, in model.DecideCaseInput) (model.AdminCase, error) {
	m.decideCase, m.calls = in, m.calls+1
	return m.cases, nil
}

func (m *governanceModule) ChangeCaseSeverity(_ context.Context, in model.ChangeCaseSeverityInput) (model.AdminCase, error) {
	m.severity, m.calls = in, m.calls+1
	return m.cases, nil
}

func (m *governanceModule) CreateCaseNote(_ context.Context, in model.CreateCaseNoteInput) (model.GovernanceNote, error) {
	m.caseNote, m.calls = in, m.calls+1
	return model.GovernanceNote{ID: uuid.NewV7(), AdminID: in.AdminID, Body: in.Body, CreatedAt: governanceNow}, nil
}

func (m *governanceModule) CaseNotes(_ context.Context, in model.CaseNoteListInput) (model.NotePage, error) {
	m.caseNotes = in
	return m.notePage, nil
}

func (m *governanceModule) CreateMeasure(_ context.Context, in model.CreateMeasureInput) (model.CreateMeasureResult, error) {
	m.create, m.calls = in, m.calls+1
	if m.createErr != nil {
		return model.CreateMeasureResult{}, m.createErr
	}
	return model.CreateMeasureResult{Measure: m.measure}, nil
}

func (m *governanceModule) CreateMeasureNote(_ context.Context, in model.CreateMeasureNoteInput) (model.GovernanceNote, error) {
	m.note, m.calls = in, m.calls+1
	return model.GovernanceNote{ID: uuid.NewV7(), AdminID: in.AdminID, Body: in.Body, CreatedAt: governanceNow}, nil
}

func (m *governanceModule) Measure(context.Context, uuid.UUID) (model.Measure, error) {
	return m.measure, nil
}

func (m *governanceModule) CurrentMeasure(context.Context, uuid.UUID) (model.Measure, error) {
	return m.measure, nil
}

func (m *governanceModule) MeasureNotes(_ context.Context, in model.MeasureNoteListInput) (model.NotePage, error) {
	m.noteList = in
	return m.notePage, nil
}

func (m *governanceModule) AdminMeasures(_ context.Context, in model.AdminMeasureListInput) (model.MeasurePage, error) {
	m.measureList = in
	return model.MeasurePage{}, nil
}

func (m *governanceModule) UpdateMeasure(_ context.Context, in model.UpdateMeasureInput) (model.Measure, error) {
	m.update, m.calls = in, m.calls+1
	return m.measure, nil
}

func (m *governanceModule) EndMeasure(_ context.Context, in model.EndMeasureInput) (model.Measure, error) {
	m.end, m.calls = in, m.calls+1
	return m.measure, nil
}

func (m *governanceModule) RevokeMeasure(_ context.Context, in model.RevokeMeasureInput) (model.Measure, error) {
	m.revoke, m.calls = in, m.calls+1
	return m.measure, nil
}

func (m *governanceModule) ScheduleMeasureReview(_ context.Context, in model.ScheduleReviewInput) (model.Measure, error) {
	m.review, m.calls = in, m.calls+1
	return m.measure, nil
}

func (m *governanceModule) DecideAppeal(_ context.Context, in model.DecideAppealInput) (model.Measure, error) {
	m.decide, m.calls = in, m.calls+1
	return m.measure, nil
}

func (m *governanceModule) DecideRemediation(_ context.Context, in model.DecideRemediationInput) (model.Measure, error) {
	m.remediate, m.calls = in, m.calls+1
	return m.measure, nil
}

func (m *governanceModule) Violation(context.Context, uuid.UUID) (model.Violation, error) {
	return m.violation, nil
}

func (m *governanceModule) AdminViolations(_ context.Context, in model.ViolationListInput) (model.ViolationPage, error) {
	m.violList = in
	return model.ViolationPage{}, nil
}

var governanceNow = time.Date(2026, 10, 4, 2, 0, 0, 0, time.UTC)

// anonymousFoodSafetyReport 是一筆待分案的匿名食安通報（沒有案件也沒有結果，所以此刻可以分案、設定除了已採取措施以外的結果）。
func anonymousFoodSafetyReport() model.AdminReport {
	phone := "0912345678"
	r := model.Report{
		ID: uuid.NewV7(), StoreID: 12, Category: policy.FoodSafety, Description: "吃了不舒服", Version: 4,
		Contact: &model.ReporterContact{Phone: &phone}, CreatedAt: governanceNow, UpdatedAt: governanceNow,
		Evidence: []model.Evidence{{AssetID: "asset-secret-1", ObjectKey: "secure/governance/k1"}, {AssetID: "asset-secret-2", ObjectKey: "secure/governance/k2"}},
		// 分案期限由模組的讀取路徑填入（seller.review_report_triage）；emitter 只轉換。
		TriageDeadline: &model.Deadline{Start: governanceNow, Due: governanceNow.Add(4 * time.Hour)},
	}
	return model.AdminReport{Report: r, Availability: model.ReportFacts{Report: r}.Availability()}
}

// caseFixture 是一個進行中的食安案件（嚴重度高、有 3 筆通報、有導出的措施），此刻可以選兩個決定的哪些由 facts 決定。
func caseFixture(facts model.CaseFacts) model.AdminCase {
	facts.Case.ID, facts.Case.StoreID, facts.Case.Category, facts.Case.Severity = uuid.NewV7(), 12, policy.FoodSafety, model.SeverityHigh
	facts.Case.Version, facts.Case.OpenedAt, facts.Case.OpenedByAdminID, facts.Case.UpdatedAt = 6, governanceNow, 7, governanceNow
	return model.AdminCase{Case: facts.Case, Availability: facts.Availability()}
}

func adminIDOf(id int64) *int64 { return &id }

func riskMeasureFixture() model.Measure {
	ends := governanceNow.Add(72 * time.Hour)
	review := governanceNow.Add(7 * 24 * time.Hour)
	ground := model.RiskFraud
	id := uuid.NewV7()
	return model.Measure{
		ID: id, AccountID: 3, StoreID: 12, Kind: model.MeasureSuspension, Basis: model.BasisRisk, RiskGround: &ground, ReviewBy: &review,
		Status: model.MeasureStatusEnforced, Version: 3, CurrentRevision: 2, CreatedByAdminID: 7, CreatedAt: governanceNow, StartsAt: governanceNow, UpdatedAt: governanceNow,
		StatementToSeller: "給賣家", EndsAt: &ends, State: model.StateActive,
		Findings: []model.Violation{{ID: uuid.NewV7(), StoreID: 12, AccountID: 3, Category: policy.Fraud, Severity: model.SeverityHigh, Status: model.ViolationStatusEnforced, OriginMeasureID: id, CreatedAt: governanceNow}},
		Revisions: []model.MeasureRevision{
			{Revision: 1, StatementToSeller: "原本的理由", AdminID: adminIDOf(7), CreatedAt: governanceNow},
			{Revision: 2, StatementToSeller: "給賣家", EndsAt: &ends, AdminID: adminIDOf(8), CreatedAt: governanceNow},
		},
		Requests: []model.MeasureRequest{{
			ID: uuid.NewV7(), Kind: model.RequestKindAppeal, State: model.RequestPending, Reason: "已改善", SubmittedAt: governanceNow, Revision: 1,
			Evidence: []model.Evidence{{AssetID: "asset-secret-3", ObjectKey: "secure/governance/k3"}},
		}},
	}
}

func TestGovernanceEnumsMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"PolicyCategory":                 rpc.EnumMismatches[commonpb.PolicyCategory](policy.Values()),
		"ReportOutcome":                  rpc.EnumMismatches[pb.ReportOutcome](model.OutcomeValues()),
		"ReportState":                    rpc.EnumMismatches[adminpb.ReportState](model.ReportStateValues()),
		"CaseState":                      rpc.EnumMismatches[adminpb.CaseState](model.CaseStateValues()),
		"CaseCloseReason":                rpc.EnumMismatches[adminpb.CaseCloseReason](model.CaseCloseReasonValues()),
		"CaseDecision":                   rpc.EnumMismatches[adminpb.CaseDecision](model.DecisionValues()),
		"SupplementMatch":                rpc.EnumMismatches[adminpb.SupplementMatch](model.SupplementMatchValues()),
		"ViolationSeverity":              rpc.EnumMismatches[pb.ViolationSeverity](model.SeverityValues()),
		"MeasureRequestKind":             rpc.EnumMismatches[pb.MeasureRequestKind](model.RequestKindValues()),
		"RemediationUnavailableReason":   rpc.EnumMismatches[pb.RemediationUnavailableReason](model.RemediationUnavailableReasonValues()),
		"AppealUnavailableReason":        rpc.EnumMismatches[pb.AppealUnavailableReason](model.AppealUnavailableReasonValues()),
		"TerminationGround":              rpc.EnumMismatches[pb.TerminationGround](model.TerminationGroundValues()),
		"MeasureKind":                    rpc.EnumMismatches[pb.MeasureKind](model.MeasureKindValues()),
		"ContentKind":                    rpc.EnumMismatches[pb.ContentKind](model.ContentKindValues()),
		"MeasureBasis":                   rpc.EnumMismatches[pb.MeasureBasis](model.BasisValues()),
		"MeasureState":                   rpc.EnumMismatches[pb.MeasureState](model.MeasureStateValues()),
		"RevokeReason":                   rpc.EnumMismatches[pb.RevokeReason](model.RevokeReasonValues()),
		"AdminRevokeReason":              rpc.EnumMismatches[adminpb.AdminRevokeReason](model.ManualRevokeReasonValues()),
		"LinkedIdentity":                 rpc.EnumMismatches[adminpb.LinkedIdentity](model.LinkedIdentityValues()),
		"MeasureActionUnavailableReason": rpc.EnumMismatches[adminpb.MeasureActionUnavailableReason](model.MeasureActionUnavailableReasonValues()),
		"MeasureRevisionSystemCause":     rpc.EnumMismatches[adminpb.MeasureRevisionSystemCause](model.RevisionSystemCauseValues()),
	} {
		if len(got) != 0 {
			t.Errorf("%s：%v", name, got)
		}
	}
}

// 佐證只以位置與檢視網址出現：asset ID 與 object key 不在任何回應裡（佐證是機敏檔，只經擁有者的端點讀）。
func TestGovernanceResponsesCarryEvidenceOnlyAsViewURIs(t *testing.T) {
	report, measure := anonymousFoodSafetyReport(), riskMeasureFixture()
	m := &governanceModule{report: report, measure: measure}
	h := NewGovernance(m, rpc.Images{})
	got, err := h.GetReport(adminCtx(1), connect.NewRequest(&adminpb.GetReportRequest{Id: report.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	r := got.Msg.GetReport()
	if len(r.GetEvidence()) != 2 || r.GetEvidence()[1].GetPosition() != 1 ||
		r.GetEvidence()[1].GetViewUri() != "/api/files/admin/seller/reports/"+report.ID.String()+"/evidence/1" {
		t.Fatalf("evidence=%v", r.GetEvidence())
	}
	if r.GetReporterBuyerId() != "" || r.GetContact().GetPhone() != "0912345678" || r.GetState() != adminpb.ReportState_REPORT_STATE_AWAITING_TRIAGE ||
		!r.GetTriageDeadline().GetStartTime().AsTime().Equal(governanceNow) || !r.GetTriageDeadline().GetDueTime().AsTime().Equal(governanceNow.Add(4*time.Hour)) || r.GetTriageTime() != nil || r.GetEtag() != "4" {
		t.Fatalf("report=%v", r)
	}
	v, err := h.GetMeasure(adminCtx(1), connect.NewRequest(&adminpb.GetMeasureRequest{Id: measure.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	pm := v.Msg.GetMeasure()
	if e := pm.GetRequests()[0].GetEvidence(); len(e) != 1 || e[0].GetViewUri() != "/api/files/admin/seller/measure_requests/"+measure.Requests[0].ID.String()+"/evidence/0" {
		t.Fatalf("appeal evidence=%v", e)
	}
	if !pm.GetEndTime().AsTime().Equal(*measure.EndsAt) || !pm.GetReviewBy().AsTime().Equal(*measure.ReviewBy) || pm.GetEtag() != "3" ||
		pm.GetState() != pb.MeasureState_MEASURE_STATE_ACTIVE || pm.GetRiskGround() != pb.RiskGround_RISK_GROUND_FRAUD {
		t.Fatalf("measure=%v", pm)
	}
	if pm.GetCurrentRevision() != 2 || len(pm.GetRevisions()) != 2 || pm.GetRevisions()[0].GetStatement() != "原本的理由" ||
		pm.GetRevisions()[0].GetEndTime() != nil || !pm.GetRevisions()[1].GetEndTime().AsTime().Equal(*measure.EndsAt) || pm.GetRevisions()[1].GetCreatedByAdminId() != "8" ||
		pm.GetRequests()[0].GetRevision() != 1 || pm.GetRequests()[0].GetKind() != pb.MeasureRequestKind_MEASURE_REQUEST_KIND_APPEAL || len(pm.GetFindings()) != 1 || pm.GetFindings()[0].GetOriginMeasureId() != measure.ID.String() {
		t.Fatalf("revisions=%v", pm)
	}
	for _, raw := range []string{protojson.Format(got.Msg), protojson.Format(v.Msg)} {
		if strings.Contains(raw, "asset-secret") || strings.Contains(raw, "secure/governance") {
			t.Fatalf("回應不得帶佐證的 asset ID 或 object key：%s", raw)
		}
	}
}

// can_* 取自模組的共用判斷，後台的 etag 取自 version。
func TestAdminMeasureCarriesTheModulesCanFlags(t *testing.T) {
	m := riskMeasureFixture()
	out := toAdminMeasure(m)
	if out.GetCanUpdateStatement() != m.CanUpdateStatement() || out.GetCanExtend() != m.CanExtend() || out.GetCanShorten() != m.CanShorten() ||
		out.GetCanEnd() != m.CanEnd() || out.GetCanRevoke() != m.CanRevoke() || out.GetCanScheduleReview() != m.CanScheduleReview() ||
		out.GetCanDecideAppeal() != m.CanDecideAppeal() || out.GetCanDecideRemediation() != m.CanDecideRemediation() ||
		out.GetCanRevokeFindings() != m.CanRevokeFindings() {
		t.Fatalf("out=%v", out)
	}
	if out.GetCanUpdateStatement() || out.GetCanRevoke() || !out.GetCanDecideAppeal() || !out.GetCanEnd() || !out.GetCanShorten() {
		t.Fatalf("有待裁定的申訴的 RISK 停權：out=%v", out)
	}
	m.Version = 8
	if toAdminMeasure(m).GetEtag() != "8" {
		t.Fatal("後台的 etag 取自 version")
	}
}

// 不能操作的原因取自模組的同一份判斷：空 ⇔ can_*，順序照模組，每個原因帶模組的文字（同一個函式），店名照 StoreName。
func TestAdminMeasureCarriesTheActionUnavailabilitiesAndStoreName(t *testing.T) {
	m := riskMeasureFixture()
	m.StoreName = "小農直送"
	out := toAdminMeasure(m)
	frozen := adminpb.MeasureActionUnavailableReason_MEASURE_ACTION_UNAVAILABLE_REASON_REQUEST_PENDING_FROZEN
	appealPending := adminpb.MeasureActionUnavailableReason_MEASURE_ACTION_UNAVAILABLE_REASON_APPEAL_PENDING
	reasonsOf := func(us []*adminpb.MeasureActionUnavailability) []adminpb.MeasureActionUnavailableReason {
		var out []adminpb.MeasureActionUnavailableReason
		for _, u := range us {
			out = append(out, u.GetReason())
		}
		return out
	}
	if out.GetStoreName() != "小農直送" ||
		!slices.Equal(reasonsOf(out.GetUpdateStatementUnavailabilities()), []adminpb.MeasureActionUnavailableReason{frozen}) ||
		!slices.Equal(reasonsOf(out.GetExtendUnavailabilities()), []adminpb.MeasureActionUnavailableReason{frozen}) ||
		len(out.GetShortenUnavailabilities()) != 0 ||
		!slices.Equal(reasonsOf(out.GetRevokeUnavailabilities()), []adminpb.MeasureActionUnavailableReason{appealPending}) {
		t.Fatalf("out=%v", out)
	}
	for _, c := range []struct {
		action model.MeasureAction
		can    bool
		items  []*adminpb.MeasureActionUnavailability
	}{
		{model.MeasureActionUpdateStatement, out.GetCanUpdateStatement(), out.GetUpdateStatementUnavailabilities()},
		{model.MeasureActionExtend, out.GetCanExtend(), out.GetExtendUnavailabilities()},
		{model.MeasureActionShorten, out.GetCanShorten(), out.GetShortenUnavailabilities()},
		{model.MeasureActionRevoke, out.GetCanRevoke(), out.GetRevokeUnavailabilities()},
		{model.MeasureActionEnd, out.GetCanEnd(), out.GetEndUnavailabilities()},
	} {
		if c.can != (len(c.items) == 0) {
			t.Errorf("%s：can=%v items=%v", c.action, c.can, c.items)
		}
		want := m.ActionUnavailableReasons(c.action)
		if len(want) != len(c.items) {
			t.Fatalf("%s：items=%v want=%v", c.action, c.items, want)
		}
		for i, u := range c.items {
			if u.GetReason() == adminpb.MeasureActionUnavailableReason_MEASURE_ACTION_UNAVAILABLE_REASON_UNSPECIFIED {
				t.Errorf("%s：UNSPECIFIED 不得出現在原因裡", c.action)
			}
			text := model.ActionUnavailableText(want[i], c.action).Text
			if text == "" || u.GetMessage().GetMessage() != text || u.GetMessage().GetLocale() != "zh-TW" {
				t.Errorf("%s[%d]：message=%v 要 %q（zh-TW）", c.action, i, u.GetMessage(), text)
			}
		}
	}
}

// 解除請求：新到舊、帶佐證路徑與提出時的版本、結案的原因與（系統結案沒有的）管理員；冷卻的結束時間與可否裁定取自模組。
func TestAdminMeasureCarriesTheRemediationRequests(t *testing.T) {
	m := riskMeasureFixture()
	m.Requests = nil
	m.ReadAt = governanceNow
	ended, revoked := model.CloseCauseEnded, model.CloseCauseRevoked
	admin := int64(9)
	rejectedAt := governanceNow.Add(-time.Hour)
	reason := "佐證不足"
	m.Requests = []model.MeasureRequest{
		{ID: uuid.NewV7(), Kind: model.RequestKindRemediation, State: model.RequestPending, Reason: "第四次", SubmittedAt: governanceNow, Revision: 2,
			Evidence: []model.Evidence{{AssetID: "asset-secret-4", ObjectKey: "secure/governance/k4"}}},
		{ID: uuid.NewV7(), Kind: model.RequestKindRemediation, State: model.RequestClosed, Reason: "第三次", SubmittedAt: governanceNow.Add(-2 * time.Hour), CloseCause: &ended,
			DecidedAt: &rejectedAt},
		{ID: uuid.NewV7(), Kind: model.RequestKindRemediation, State: model.RequestClosed, Reason: "第二次", SubmittedAt: governanceNow.Add(-3 * time.Hour), CloseCause: &revoked,
			DecidedAt: &rejectedAt, DecidedByAdminID: &admin},
		{ID: uuid.NewV7(), Kind: model.RequestKindRemediation, State: model.RequestRejected, Reason: "第一次", SubmittedAt: governanceNow.Add(-4 * time.Hour), DecisionReason: &reason,
			DecidedAt: &rejectedAt, DecidedByAdminID: &admin},
		{ID: uuid.NewV7(), Kind: model.RequestKindAppeal, State: model.RequestRejected, Reason: "申訴", SubmittedAt: governanceNow.Add(-5 * time.Hour), DecisionReason: &reason,
			DecidedAt: &rejectedAt, DecidedByAdminID: &admin},
	}
	out := toAdminMeasure(m)
	got := out.GetRequests()
	if len(got) != 5 || got[0].GetReason() != "第四次" || got[3].GetReason() != "第一次" || got[4].GetReason() != "申訴" ||
		got[0].GetKind() != pb.MeasureRequestKind_MEASURE_REQUEST_KIND_REMEDIATION || got[4].GetKind() != pb.MeasureRequestKind_MEASURE_REQUEST_KIND_APPEAL {
		t.Fatalf("申訴與補正在同一個清單、新到舊、帶 kind：%v", out)
	}
	if e := got[0].GetEvidence(); len(e) != 1 || e[0].GetViewUri() != "/api/files/admin/seller/measure_requests/"+m.Requests[0].ID.String()+"/evidence/0" || got[0].GetRevision() != 2 {
		t.Fatalf("pending=%v", got[0])
	}
	if got[1].GetCloseCause() != pb.MeasureRequestCloseCause_MEASURE_REQUEST_CLOSE_CAUSE_ENDED || got[1].GetDecidedByAdminId() != "" ||
		got[2].GetCloseCause() != pb.MeasureRequestCloseCause_MEASURE_REQUEST_CLOSE_CAUSE_REVOKED || got[2].GetDecidedByAdminId() != "9" ||
		got[3].GetCloseCause() != pb.MeasureRequestCloseCause_MEASURE_REQUEST_CLOSE_CAUSE_UNSPECIFIED || got[3].GetDecisionReason() != "佐證不足" {
		t.Fatalf("close causes=%v", got)
	}
	if !out.GetCanDecideRemediation() || out.GetCanEnd() || out.GetNextRemediationTime() != nil {
		t.Fatalf("待處理的補正：可裁定、不能結束、沒有冷卻：%v", out)
	}
	for _, text := range []string{protojson.Format(out)} {
		if strings.Contains(text, "asset-secret") || strings.Contains(text, "secure/governance") {
			t.Fatalf("回應不得帶佐證的 asset ID 或 object key：%s", text)
		}
	}
	m.Requests = []model.MeasureRequest{
		{Kind: model.RequestKindRemediation, State: model.RequestRejected, DecidedAt: &rejectedAt},
		{Kind: model.RequestKindAppeal, State: model.RequestRejected, DecidedAt: &rejectedAt},
		{Kind: model.RequestKindRemediation, State: model.RequestRejected, DecidedAt: &rejectedAt},
	}
	cooled := toAdminMeasure(m)
	if want := rejectedAt.Add(7 * 24 * time.Hour); !cooled.GetNextRemediationTime().AsTime().Equal(want) {
		t.Fatalf("三次駁回：冷卻到最近一次駁回後 7 天：%v", cooled.GetNextRemediationTime())
	}
}

// 指令都以呼叫的管理員記錄，列舉照轉（DecideAppeal 的裁定是結果列舉，不是布林）。
func TestGovernanceCommandsUseTheCallingAdmin(t *testing.T) {
	report, measure := anonymousFoodSafetyReport(), riskMeasureFixture()
	m := &governanceModule{report: report, measure: measure, cases: caseFixture(model.CaseFacts{})}
	h := NewGovernance(m, rpc.Images{})
	ctx := adminCtx(42)
	id := report.ID.String()
	second := uuid.NewV7()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	caseID := uuid.NewV7()
	_, err := h.TriageReport(ctx, connect.NewRequest(&adminpb.TriageReportRequest{
		Id: id, AssessedCategory: commonpb.PolicyCategory_POLICY_CATEGORY_FOOD_SAFETY, Etag: "4",
		Target: &adminpb.CaseTarget{Target: &adminpb.CaseTarget_OpenNew{OpenNew: &adminpb.OpenNewCase{Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_CRITICAL}}},
	}))
	must(err)
	_, err = h.ResolveReport(ctx, connect.NewRequest(&adminpb.ResolveReportRequest{Id: id, Outcome: pb.ReportOutcome_REPORT_OUTCOME_NO_VIOLATION, Note: "查無", Etag: "5"}))
	must(err)
	moved, err := h.MoveReport(ctx, connect.NewRequest(&adminpb.MoveReportRequest{
		Id: id, AssessedCategory: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD, Etag: "6",
		Target: &adminpb.CaseTarget{Target: &adminpb.CaseTarget_Join{Join: &adminpb.JoinCase{CaseId: caseID.String(), Etag: "3"}}},
	}))
	must(err)
	_, err = h.DecideCase(ctx, connect.NewRequest(&adminpb.DecideCaseRequest{Id: caseID.String(), Decision: adminpb.CaseDecision_CASE_DECISION_NO_ACTION, Note: "已確認", Etag: "7"}))
	must(err)
	_, err = h.ChangeCaseSeverity(ctx, connect.NewRequest(&adminpb.ChangeCaseSeverityRequest{
		Id: caseID.String(), Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW, Note: "事實更清楚", Etag: "8",
	}))
	must(err)
	if m.triage.AdminID != 42 || m.triage.ReportID != report.ID || m.triage.AssessedCategory != policy.FoodSafety || m.triage.Target.OpenNew == nil || *m.triage.Target.OpenNew != model.SeverityCritical || m.triage.Target.Join != nil || m.triage.ExpectedVersion != 4 ||
		m.resolve.AdminID != 42 || m.resolve.Outcome != model.OutcomeNoViolation || m.resolve.Note != "查無" || m.resolve.ExpectedVersion != 5 ||
		m.move.AdminID != 42 || m.move.AssessedCategory != policy.Fraud || m.move.Target.Join == nil || m.move.Target.Join.CaseID != caseID || m.move.Target.Join.ExpectedVersion != 3 || m.move.Target.OpenNew != nil || m.move.ExpectedVersion != 6 || moved.Msg.GetSourceCaseClosed() ||
		m.decideCase.AdminID != 42 || m.decideCase.CaseID != caseID || m.decideCase.Decision != model.DecisionNoAction || m.decideCase.Note != "已確認" || m.decideCase.ExpectedVersion != 7 ||
		m.severity.AdminID != 42 || m.severity.Severity != model.SeverityLow || m.severity.Note != "事實更清楚" || m.severity.ExpectedVersion != 8 {
		t.Fatalf("triage=%+v resolve=%+v move=%+v decide=%+v severity=%+v", m.triage, m.resolve, m.move, m.decideCase, m.severity)
	}
	ends := governanceNow.Add(48 * time.Hour)
	reqID := uuid.New()
	_, err = h.CreateMeasure(ctx, connect.NewRequest(&adminpb.CreateMeasureRequest{
		RequestId: reqID.String(), StoreId: "12", Statement: "s", Note: "d",
		Measure: &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{
			EndTime: timestamppb.New(ends),
			Basis: &adminpb.SuspensionInput_Violation{Violation: &adminpb.ViolationBasis{
				Finding:         &adminpb.NewFinding{Category: commonpb.PolicyCategory_POLICY_CATEGORY_FOOD_SAFETY, Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_MEDIUM, SourceReportIds: []string{id, second.String()}},
				CitedFindingIds: []string{measure.Findings[0].ID.String()},
			}},
		}},
	}))
	must(err)
	if c := m.create; c.AdminID != 42 || c.StoreID != 12 || c.RequestID == nil || *c.RequestID != reqID || c.Kind != model.MeasureSuspension || c.StatementToSeller != "s" || c.Note != "d" ||
		c.EndsAt == nil || !c.EndsAt.Equal(ends) || c.ReviewBy != nil || c.Risk != nil || c.Violation == nil || c.Violation.Finding == nil ||
		c.Violation.Finding.Severity != model.SeverityMedium || len(c.Violation.Finding.SourceReportIDs) != 2 || c.Violation.Finding.SourceReportIDs[0] != report.ID || c.Violation.Finding.SourceReportIDs[1] != second ||
		len(c.Violation.CitedFindingIDs) != 1 || c.Violation.CitedFindingIDs[0] != measure.Findings[0].ID {
		t.Fatalf("create=%+v", c)
	}
	review := governanceNow.Add(5 * 24 * time.Hour)
	_, err = h.CreateMeasure(ctx, connect.NewRequest(&adminpb.CreateMeasureRequest{
		StoreId: "12", Statement: "s", Note: "d",
		Measure: &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{
			ReviewBy: timestamppb.New(review),
			Basis:    &adminpb.SuspensionInput_Risk{Risk: &adminpb.RiskBasis{Ground: pb.RiskGround_RISK_GROUND_SECURITY}},
		}},
	}))
	must(err)
	if c := m.create; c.RequestID != nil || c.Risk == nil || c.Risk.Ground != model.RiskSecurity || c.Violation != nil || c.ReviewBy == nil || !c.ReviewBy.Equal(review) || c.EndsAt != nil {
		t.Fatalf("risk create=%+v", c)
	}
	_, err = h.RevokeMeasure(ctx, connect.NewRequest(&adminpb.RevokeMeasureRequest{Id: measure.ID.String(), Etag: "4", Reason: adminpb.AdminRevokeReason_ADMIN_REVOKE_REASON_EXTERNAL_DECISION, RevokeFindings: true}))
	must(err)
	_, err = h.EndMeasure(ctx, connect.NewRequest(&adminpb.EndMeasureRequest{Id: measure.ID.String(), Etag: "7"}))
	must(err)
	_, err = h.ScheduleMeasureReview(ctx, connect.NewRequest(&adminpb.ScheduleMeasureReviewRequest{Id: measure.ID.String(), Etag: "8", ReviewBy: timestamppb.New(review), Note: "再觀察"}))
	must(err)
	_, err = h.DecideAppeal(ctx, connect.NewRequest(&adminpb.DecideAppealRequest{MeasureId: measure.ID.String(), Etag: "5", Decision: pb.MeasureRequestState_MEASURE_REQUEST_STATE_REJECTED, Reason: "證據不足"}))
	must(err)
	if m.revoke.AdminID != 42 || m.revoke.ExpectedVersion != 4 || m.revoke.Reason != model.RevokeExternalDecision || !m.revoke.RevokeFindings ||
		m.end.AdminID != 42 || m.end.ExpectedVersion != 7 || m.review.AdminID != 42 || m.review.ExpectedVersion != 8 || m.review.Note != "再觀察" || !m.review.ReviewBy.Equal(review) ||
		m.decide.AdminID != 42 || m.decide.ExpectedVersion != 5 || m.decide.Outcome != model.RequestRejected || m.decide.Reason != "證據不足" || m.decide.RevokeFindings {
		t.Fatalf("revoke=%+v end=%+v review=%+v decide=%+v", m.revoke, m.end, m.review, m.decide)
	}
	_, err = h.DecideAppeal(ctx, connect.NewRequest(&adminpb.DecideAppealRequest{MeasureId: measure.ID.String(), Etag: "6", Decision: pb.MeasureRequestState_MEASURE_REQUEST_STATE_ACCEPTED, Reason: "接受", RevokeFindings: true}))
	must(err)
	if m.decide.Outcome != model.RequestAccepted || m.decide.ExpectedVersion != 6 || !m.decide.RevokeFindings {
		t.Fatalf("decide=%+v", m.decide)
	}
	_, err = h.DecideRemediation(ctx, connect.NewRequest(&adminpb.DecideRemediationRequest{MeasureId: measure.ID.String(), Etag: "9", Decision: pb.MeasureRequestState_MEASURE_REQUEST_STATE_ACCEPTED, Reason: "已確認改善"}))
	must(err)
	if want := (model.DecideRemediationInput{MeasureID: measure.ID, AdminID: 42, ExpectedVersion: 9, Outcome: model.RequestAccepted, Reason: "已確認改善"}); m.remediate != want {
		t.Fatalf("remediate=%+v", m.remediate)
	}
	before := m.calls
	_, err = h.DecideRemediation(ctx, connect.NewRequest(&adminpb.DecideRemediationRequest{MeasureId: measure.ID.String(), Decision: pb.MeasureRequestState_MEASURE_REQUEST_STATE_REJECTED, Reason: "x"}))
	wantInvalidArgument(t, err)
	if m.calls != before {
		t.Fatal("沒有 etag 的裁定不能呼叫模組")
	}
}

// 建立措施的 request_id 必須是 UUIDv4；依據一定要指定；不合格的請求不呼叫模組。
func TestCreateMeasureRejectsBadRequestsBeforeTheModule(t *testing.T) {
	m := &governanceModule{measure: riskMeasureFixture()}
	h := NewGovernance(m, rpc.Images{})
	risk := &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{Basis: &adminpb.SuspensionInput_Risk{Risk: &adminpb.RiskBasis{Ground: pb.RiskGround_RISK_GROUND_FRAUD}}}}
	for name, req := range map[string]*adminpb.CreateMeasureRequest{
		"request_id 不是 UUID": {RequestId: "x", StoreId: "12", Measure: risk},
		"request_id 不是 v4":   {RequestId: uuid.NewV7().String(), StoreId: "12", Measure: risk},
		"沒有指定措施種類":           {StoreId: "12"},
		"停權沒有依據":             {StoreId: "12", Measure: &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{}}},
		"store_id":           {StoreId: "0", Measure: risk},
		"引用的認定不是 UUID": {StoreId: "12", Measure: &adminpb.CreateMeasureRequest_Warning{Warning: &adminpb.WarningInput{
			Violation: &adminpb.ViolationBasis{CitedFindingIds: []string{"x"}}}}},
		"下架的內容 ID 不是 UUID": {StoreId: "12", Measure: &adminpb.CreateMeasureRequest_ContentTakedown{ContentTakedown: &adminpb.ContentTakedownInput{
			Contents: []*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_POST, Id: "x"}}}}},
		"來源通報不是 UUID": {StoreId: "12", Measure: &adminpb.CreateMeasureRequest_Warning{Warning: &adminpb.WarningInput{
			Violation: &adminpb.ViolationBasis{Finding: &adminpb.NewFinding{SourceReportIds: []string{uuid.NewV7().String(), "x"}}}}}},
	} {
		_, err := h.CreateMeasure(adminCtx(1), connect.NewRequest(req))
		if err == nil {
			t.Errorf("%s：要被拒絕", name)
			continue
		}
		wantInvalidArgument(t, err)
	}
	if m.calls != 0 {
		t.Fatal("被拒絕的請求不能呼叫模組")
	}
}

// 撤銷、結束、重排複審與裁定申覆的 etag 必填：沒帶回 invalid_input，不呼叫模組。
func TestMeasureCommandsRequireAnEtag(t *testing.T) {
	measure := riskMeasureFixture()
	m := &governanceModule{measure: measure}
	h := NewGovernance(m, rpc.Images{})
	ctx := adminCtx(42)
	id := measure.ID.String()
	_, err := h.RevokeMeasure(ctx, connect.NewRequest(&adminpb.RevokeMeasureRequest{Id: id, Reason: adminpb.AdminRevokeReason_ADMIN_REVOKE_REASON_OTHER}))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("revoke err=%v", err)
	}
	_, err = h.EndMeasure(ctx, connect.NewRequest(&adminpb.EndMeasureRequest{Id: id}))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("end err=%v", err)
	}
	_, err = h.ScheduleMeasureReview(ctx, connect.NewRequest(&adminpb.ScheduleMeasureReviewRequest{Id: id, ReviewBy: timestamppb.New(governanceNow), Note: "x"}))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("review err=%v", err)
	}
	_, err = h.DecideAppeal(ctx, connect.NewRequest(&adminpb.DecideAppealRequest{MeasureId: id, Decision: pb.MeasureRequestState_MEASURE_REQUEST_STATE_ACCEPTED, Reason: "x"}))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("decide err=%v", err)
	}
	if m.calls != 0 {
		t.Fatal("沒有 etag 的指令不能呼叫模組")
	}
}

// UpdateMeasure 依 update_mask 合併：只改理由時不帶結束時間；結束時間有變才交給模組（清除是 ChangeEndsAt 加 nil）；etag 必填。
func TestUpdateMeasureAppliesTheMaskAndSendsOnlyAChangedEndTime(t *testing.T) {
	measure := riskMeasureFixture()
	m := &governanceModule{measure: measure}
	h := NewGovernance(m, rpc.Images{})
	update := func(v *adminpb.Measure, paths ...string) error {
		_, err := h.UpdateMeasure(adminCtx(9), connect.NewRequest(&adminpb.UpdateMeasureRequest{Measure: v, UpdateMask: &fieldmaskpb.FieldMask{Paths: paths}}))
		return err
	}
	id := measure.ID.String()
	if err := update(&adminpb.Measure{Id: id, Etag: "3", Statement: "新的理由", State: pb.MeasureState_MEASURE_STATE_REVOKED}, "statement", "state"); err != nil {
		t.Fatal(err)
	}
	if u := m.update; u.AdminID != 9 || u.ExpectedVersion != 3 || *u.StatementToSeller != "新的理由" || u.ChangeEndsAt || u.EndsAt != nil {
		t.Fatalf("update=%+v", u)
	}
	later := measure.EndsAt.Add(24 * time.Hour)
	if err := update(&adminpb.Measure{Id: id, Etag: "3", EndTime: timestamppb.New(later)}, "end_time"); err != nil {
		t.Fatal(err)
	}
	if !m.update.ChangeEndsAt || m.update.EndsAt == nil || !m.update.EndsAt.Equal(later) {
		t.Fatalf("end time=%+v", m.update)
	}
	if err := update(&adminpb.Measure{Id: id, Etag: "3"}, "end_time"); err != nil {
		t.Fatal(err)
	}
	if !m.update.ChangeEndsAt || m.update.EndsAt != nil {
		t.Fatalf("mask 列了 end_time 卻沒有值是清除：%+v", m.update)
	}
	calls := m.calls
	wantInvalidArgument(t, update(&adminpb.Measure{Id: id, Statement: "x"}, "statement"))
	wantInvalidArgument(t, update(&adminpb.Measure{Id: id, Etag: "3"}, "unknown_field"))
	wantInvalidArgument(t, update(&adminpb.Measure{Id: id, Etag: "3"}, "description"))
	if m.calls != calls {
		t.Fatal("被拒絕的修改不能呼叫模組")
	}
}

// 新增註記只帶措施 ID 與 body，管理員取自呼叫者。
func TestCreateMeasureNoteUsesTheCallingAdmin(t *testing.T) {
	measure := riskMeasureFixture()
	m := &governanceModule{measure: measure}
	h := NewGovernance(m, rpc.Images{})
	got, err := h.CreateMeasureNote(adminCtx(42), connect.NewRequest(&adminpb.CreateMeasureNoteRequest{
		MeasureId: measure.ID.String(), Note: &adminpb.GovernanceNote{Body: "補充", CreatedByAdminId: "1"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if m.note.MeasureID != measure.ID || m.note.AdminID != 42 || m.note.Body != "補充" || got.Msg.GetNote().GetCreatedByAdminId() != "42" || got.Msg.GetNote().GetBody() != "補充" {
		t.Fatalf("note=%+v got=%v", m.note, got.Msg)
	}
}

// 註記是子集合：獨立分頁讀（GetMeasure 不內嵌），page token 綁在措施上，換一筆措施不能沿用。
func TestListMeasureNotesPagesTheSubCollectionPerMeasure(t *testing.T) {
	measure := riskMeasureFixture()
	cursor := &model.NoteCursor{CreatedAt: governanceNow, ID: uuid.NewV7()}
	m := &governanceModule{measure: measure, notePage: model.NotePage{
		Items: []model.GovernanceNote{{ID: uuid.NewV7(), AdminID: 7, Body: "內部", CreatedAt: governanceNow}}, Next: cursor,
	}}
	h := NewGovernance(m, rpc.Images{})
	req := &adminpb.ListMeasureNotesRequest{MeasureId: measure.ID.String(), PageSize: 1}
	got, err := h.ListMeasureNotes(adminCtx(1), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if n := got.Msg.GetNotes(); len(n) != 1 || n[0].GetBody() != "內部" || n[0].GetCreatedByAdminId() != "7" || m.noteList.MeasureID != measure.ID || m.noteList.PageSize != 1 {
		t.Fatalf("notes=%v in=%+v", got.Msg.GetNotes(), m.noteList)
	}
	req.PageToken = got.Msg.GetNextPageToken()
	if _, err := h.ListMeasureNotes(adminCtx(1), connect.NewRequest(req)); err != nil || m.noteList.After == nil || *m.noteList.After != *cursor {
		t.Fatalf("after=%+v err=%v", m.noteList.After, err)
	}
	req.MeasureId = uuid.NewV7().String()
	_, err = h.ListMeasureNotes(adminCtx(1), connect.NewRequest(req))
	wantInvalidArgument(t, err)
}

// 措施清單的篩選照轉給模組，page token 綁在篩選上。
func TestListMeasuresMapsFiltersAndBindsThePageToken(t *testing.T) {
	m := &governanceModule{}
	h := NewGovernance(m, rpc.Images{})
	if _, err := h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{})); err != nil {
		t.Fatal(err)
	}
	if in := m.measureList; in.State != nil || in.Basis != nil || in.RequestState != nil || in.RequestKind != nil || in.ReviewDue || in.StoreID != 0 || in.AccountID != 0 {
		t.Fatalf("UNSPECIFIED 不是篩選值：%+v", in)
	}
	if _, err := h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{
		StoreId: "12", AccountId: "3", State: pb.MeasureState_MEASURE_STATE_ACTIVE, Basis: pb.MeasureBasis_MEASURE_BASIS_RISK,
		RequestState: pb.MeasureRequestState_MEASURE_REQUEST_STATE_PENDING, RequestKind: pb.MeasureRequestKind_MEASURE_REQUEST_KIND_REMEDIATION, ReviewDue: true,
	})); err != nil {
		t.Fatal(err)
	}
	if in := m.measureList; in.StoreID != 12 || in.AccountID != 3 || *in.State != model.StateActive || *in.Basis != model.BasisRisk || *in.RequestState != model.RequestPending ||
		*in.RequestKind != model.RequestKindRemediation || !in.ReviewDue {
		t.Fatalf("in=%+v", in)
	}
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListMeasuresProcedure, "", "", pb.MeasureState_MEASURE_STATE_UNSPECIFIED.String(),
		pb.MeasureBasis_MEASURE_BASIS_UNSPECIFIED.String(), pb.MeasureRequestState_MEASURE_REQUEST_STATE_UNSPECIFIED.String(),
		pb.MeasureRequestKind_MEASURE_REQUEST_KIND_UNSPECIFIED.String(), "false", "", "", "")
	token := rpc.TokenFor(scope, &model.MeasureCursor{SortAt: governanceNow, ID: uuid.NewV7()})
	if _, err := h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token})); err != nil || m.measureList.After == nil {
		t.Fatalf("after=%+v err=%v", m.measureList.After, err)
	}
	_, err := h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token, ReviewDue: true}))
	wantInvalidArgument(t, err)
	_, err = h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token, State: pb.MeasureState_MEASURE_STATE_ENDED}))
	wantInvalidArgument(t, err)
	_, err = h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token, RequestState: pb.MeasureRequestState_MEASURE_REQUEST_STATE_PENDING}))
	wantInvalidArgument(t, err)
	_, err = h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token, RequestKind: pb.MeasureRequestKind_MEASURE_REQUEST_KIND_APPEAL}))
	wantInvalidArgument(t, err)
}

// 清單的 page token 綁在篩選上：換了篩選就不能沿用；篩選照轉給模組；清單不帶匿名檢舉人的聯絡方式。
func TestListReportsBindsThePageTokenToTheFilters(t *testing.T) {
	cursor := &model.ReportCursor{CreatedAt: governanceNow, ID: uuid.NewV7()}
	m := &governanceModule{reportPage: model.AdminReportPage{Items: []model.AdminReport{anonymousFoodSafetyReport()}, Next: cursor}}
	h := NewGovernance(m, rpc.Images{})
	caseID := uuid.NewV7()
	req := &adminpb.ListReportsRequest{
		State: adminpb.ReportState_REPORT_STATE_IN_CASE, Category: commonpb.PolicyCategory_POLICY_CATEGORY_OTHER, StoreId: "12", CaseId: caseID.String(), CitableAsSource: true, PageSize: 1,
	}
	got, err := h.ListReports(adminCtx(1), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.GetReports()[0].GetContact() != nil {
		t.Fatal("清單不帶匿名檢舉人的聯絡方式")
	}
	if in := m.reportList; *in.State != model.ReportStateInCase || *in.Category != policy.Other || in.StoreID != 12 || in.CaseID == nil || *in.CaseID != caseID || !in.CitableAsSource || in.PageSize != 1 {
		t.Fatalf("in=%+v", in)
	}
	token := got.Msg.GetNextPageToken()
	req.PageToken = token
	if _, err := h.ListReports(adminCtx(1), connect.NewRequest(req)); err != nil || m.reportList.After == nil || *m.reportList.After != *cursor {
		t.Fatalf("after=%+v err=%v", m.reportList.After, err)
	}
	for name, change := range map[string]func(*adminpb.ListReportsRequest){
		"state":    func(r *adminpb.ListReportsRequest) { r.State = adminpb.ReportState_REPORT_STATE_RESOLVED },
		"category": func(r *adminpb.ListReportsRequest) { r.Category = commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD },
		"store":    func(r *adminpb.ListReportsRequest) { r.StoreId = "13" },
		"case":     func(r *adminpb.ListReportsRequest) { r.CaseId = uuid.NewV7().String() },
		"citable":  func(r *adminpb.ListReportsRequest) { r.CitableAsSource = false },
	} {
		changed := proto.Clone(req).(*adminpb.ListReportsRequest)
		change(changed)
		_, err := h.ListReports(adminCtx(1), connect.NewRequest(changed))
		t.Run(name, func(t *testing.T) { wantInvalidArgument(t, err) })
	}
	if _, err := h.ListViolations(adminCtx(1), connect.NewRequest(&adminpb.ListViolationsRequest{StoreId: "12", AccountId: "3"})); err != nil ||
		m.violList.StoreID != 12 || m.violList.AccountID != 3 || m.violList.Status != nil {
		t.Fatalf("violations in=%+v err=%v", m.violList, err)
	}
}

type evidenceModule struct {
	in      model.ReadEvidenceInput
	content model.FileContent
	err     error
	calls   int
}

func (e *evidenceModule) ReadEvidence(_ context.Context, in model.ReadEvidenceInput) (model.FileContent, error) {
	e.in, e.calls = in, e.calls+1
	return e.content, e.err
}

func serveEvidence(m *evidenceModule, log *slog.Logger, ctx context.Context, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.Handle(EvidenceReportRoute, NewAdminEvidence(m, model.EvidenceReport, log))
	mux.Handle(EvidenceRequestRoute, NewAdminEvidence(m, model.EvidenceRequest, log))
	mux.Handle(EvidenceInquiryRoute, NewAdminEvidence(m, model.EvidenceInquiry, log))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil))
	return rec
}

// 佐證端點：只給管理員；兩條路由各自對應通報與措施上的請求；格式不對的 ID 與位置不呼叫模組；錯誤只有 HTTP 狀態，與證件端點同一套對應
// （同步還沒結算 503 加 Retry-After，資料不一致 404 加 error log）。
func TestEvidenceEndpointsAuthorizeAdminsAndMapErrorsWithoutLeaking(t *testing.T) {
	report, request := uuid.NewV7(), uuid.NewV7()
	reportPath := "/api/files/admin/seller/reports/" + report.String() + "/evidence/1"
	appealPath := "/api/files/admin/seller/measure_requests/" + request.String() + "/evidence/0"
	customer := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 5})
	for name, c := range map[string]struct {
		ctx  context.Context
		want int
	}{"visitor": {context.Background(), 401}, "customer": {customer, 403}} {
		m := &evidenceModule{}
		if rec := serveEvidence(m, nil, c.ctx, reportPath); rec.Code != c.want || m.calls != 0 || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s：code=%d calls=%d", name, rec.Code, m.calls)
		}
	}
	m := &evidenceModule{content: model.FileContent{Data: []byte("PDF"), ContentType: "application/pdf"}}
	rec := serveEvidence(m, nil, adminCtx(42), reportPath)
	if rec.Code != 200 || rec.Body.String() != "PDF" || rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
		m.in != (model.ReadEvidenceInput{AdminID: 42, Kind: model.EvidenceReport, RefID: report, Position: 1}) {
		t.Fatalf("report：code=%d in=%+v", rec.Code, m.in)
	}
	if rec := serveEvidence(m, nil, adminCtx(42), appealPath); rec.Code != 200 || m.in != (model.ReadEvidenceInput{AdminID: 42, Kind: model.EvidenceRequest, RefID: request, Position: 0}) {
		t.Fatalf("appeal：code=%d in=%+v", rec.Code, m.in)
	}
	for _, bad := range []string{
		"/api/files/admin/seller/reports/not-a-uuid/evidence/0", "/api/files/admin/seller/reports/" + report.String() + "/evidence/x",
		"/api/files/admin/seller/reports/" + report.String() + "/evidence/-1",
	} {
		m := &evidenceModule{}
		if rec := serveEvidence(m, nil, adminCtx(42), bad); rec.Code != 404 || m.calls != 0 {
			t.Fatalf("%s：code=%d calls=%d", bad, rec.Code, m.calls)
		}
	}
	for name, c := range map[string]struct {
		err        error
		code       int
		retryAfter bool
		level      string
	}{
		"not synced yet":     {fmt.Errorf("x：%w", model.ErrFileNotSynced), 503, true, "WARN"},
		"synced but missing": {fmt.Errorf("x：%w", model.ErrFileMissing), 404, false, "ERROR"},
		"not secure":         {fmt.Errorf("x：%w", media.ErrNotSecure), 404, false, "ERROR"},
		"not found":          {apperr.NotFound("找不到這張佐證"), 404, false, ""},
		"media down":         {errors.New("secret internal detail"), 500, false, "ERROR"},
	} {
		var logs strings.Builder
		rec := serveEvidence(&evidenceModule{err: c.err}, slog.New(slog.NewTextHandler(&logs, nil)), adminCtx(42), appealPath)
		if rec.Code != c.code || (rec.Header().Get("Retry-After") != "") != c.retryAfter || strings.Contains(rec.Body.String(), "secret") {
			t.Fatalf("%s：code=%d retry-after=%q body=%q", name, rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
		}
		if c.level == "" && logs.Len() != 0 || c.level != "" && !strings.Contains(logs.String(), "level="+c.level) {
			t.Fatalf("%s：log=%q，要 %q", name, logs.String(), c.level)
		}
	}
}

// handler 的欄位對應表指向請求真的有的欄位：proto 改名時對應表要跟著改。
func TestMeasureFieldPathsExistInTheRequests(t *testing.T) {
	for name, c := range map[string]struct {
		paths rpc.FieldPaths
		req   interface{ ProtoReflect() protoreflect.Message }
	}{
		"createWarning":     {createWarningPaths, &adminpb.CreateMeasureRequest{}},
		"createSuspension":  {createSuspensionPaths, &adminpb.CreateMeasureRequest{}},
		"createTermination": {createTerminationPaths, &adminpb.CreateMeasureRequest{}},
		"createTakedown":    {createTakedownPaths, &adminpb.CreateMeasureRequest{}},
		"note":              {createMeasureNotePaths, &adminpb.CreateMeasureNoteRequest{}},
		"caseNote":          {createNotePaths, &adminpb.CreateCaseNoteRequest{}},
		"listReports":       {listReportsPaths, &adminpb.ListReportsRequest{}},
		"triage":            {triagePaths, &adminpb.TriageReportRequest{}},
		"move":              {triagePaths, &adminpb.MoveReportRequest{}},
		"resolveReport":     {resolvePaths, &adminpb.ResolveReportRequest{}},
		"markSeen":          {markSeenPaths, &adminpb.MarkReportSupplementsSeenRequest{}},
		"listCases":         {listCasesPaths, &adminpb.ListCasesRequest{}},
		"decideCase":        {decideCasePaths, &adminpb.DecideCaseRequest{}},
		"severity":          {changeSeverityPaths, &adminpb.ChangeCaseSeverityRequest{}},
		"update":            {updateMeasurePaths, &adminpb.UpdateMeasureRequest{}},
		"end":               {endMeasurePaths, &adminpb.EndMeasureRequest{}},
		"revoke":            {revokeMeasurePaths, &adminpb.RevokeMeasureRequest{}},
		"review":            {scheduleReviewPaths, &adminpb.ScheduleMeasureReviewRequest{}},
		"decide":            {decideAppealPaths, &adminpb.DecideAppealRequest{}},
		"remed":             {decideRemediationPaths, &adminpb.DecideRemediationRequest{}},
		"list":              {listMeasuresPaths, &adminpb.ListMeasuresRequest{}},
		"listV":             {listViolationsPaths, &adminpb.ListViolationsRequest{}},
		"appeal":            {appealPaths, &pb.SubmitAppealRequest{}},
		"remedy":            {appealPaths, &pb.RequestRemediationRequest{}},
		"myList":            {measurePaths, &pb.ListMyMeasuresRequest{}},
	} {
		if missing := rpc.CheckFieldPaths(c.req.ProtoReflect().Descriptor(), c.paths); len(missing) > 0 {
			t.Errorf("%s：不存在的路徑 %v", name, missing)
		}
	}
}

func terminationFixture(ground model.TerminationGround, basis model.MeasureBasis) model.Measure {
	m := riskMeasureFixture()
	m.Kind, m.Basis, m.RiskGround, m.ReviewBy, m.EndsAt = model.MeasureTermination, basis, nil, nil, nil
	m.TerminationGround = &ground
	m.StartsAt = governanceNow.Add(720 * time.Hour)
	m.State = model.StateScheduled
	m.Requests = nil
	m.ReadAt = governanceNow
	if basis == model.BasisContract {
		m.Findings = nil
	}
	return m
}

// 終止合作建立請求的對應：事由、依據的認定與來源措施都交給模組，依據由事由推導（不在傳輸層決定）；種類就是設定的是哪一支輸入。
func TestCreateMeasureMapsTheTerminationInput(t *testing.T) {
	m := &governanceModule{measure: terminationFixture(model.GroundNotice, model.BasisContract)}
	h := NewGovernance(m, rpc.Images{})
	source := uuid.NewV7()
	cited := uuid.NewV7()
	_, err := h.CreateMeasure(adminCtx(42), connect.NewRequest(&adminpb.CreateMeasureRequest{
		StoreId: "12", Statement: "s", Note: "n",
		Measure: &adminpb.CreateMeasureRequest_Termination{Termination: &adminpb.TerminationInput{
			Ground: pb.TerminationGround_TERMINATION_GROUND_LINKED_ACCOUNT, SourceMeasureId: source.String(),
		}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c := m.create; c.Kind != model.MeasureTermination || c.Termination == nil || c.Termination.Ground != model.GroundLinkedAccount ||
		c.Termination.SourceMeasureID == nil || *c.Termination.SourceMeasureID != source || c.Termination.Violation != nil || c.Violation != nil || c.Risk != nil {
		t.Fatalf("create=%+v", c)
	}
	_, err = h.CreateMeasure(adminCtx(42), connect.NewRequest(&adminpb.CreateMeasureRequest{
		StoreId: "12", Statement: "s", Note: "n",
		Measure: &adminpb.CreateMeasureRequest_Termination{Termination: &adminpb.TerminationInput{
			Ground:    pb.TerminationGround_TERMINATION_GROUND_MATERIAL_BREACH,
			Violation: &adminpb.ViolationBasis{CitedFindingIds: []string{cited.String()}},
		}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c := m.create; c.Termination == nil || c.Termination.Ground != model.GroundMaterialBreach || c.Termination.Violation == nil ||
		len(c.Termination.Violation.CitedFindingIDs) != 1 || c.Termination.Violation.CitedFindingIDs[0] != cited {
		t.Fatalf("create=%+v", c)
	}
	_, err = h.CreateMeasure(adminCtx(42), connect.NewRequest(&adminpb.CreateMeasureRequest{
		StoreId: "12", Statement: "s", Note: "n",
		Measure: &adminpb.CreateMeasureRequest_Termination{Termination: &adminpb.TerminationInput{
			Ground: pb.TerminationGround_TERMINATION_GROUND_LINKED_ACCOUNT, SourceMeasureId: "not-a-uuid",
		}},
	}))
	wantInvalidArgument(t, err)
}

// 後台看到的終止合作：事由、申覆期限、來源措施、期間的開始時間是 starts_at（不是建立時間）、關聯帳號區塊，以及系統寫入的版本沒有管理員。
func TestAdminTerminationCarriesGroundDeadlineSourceAndLinkedAccounts(t *testing.T) {
	m := terminationFixture(model.GroundLinkedAccount, model.BasisContract)
	source, linked := uuid.NewV7(), uuid.NewV7()
	m.SourceMeasureID = &source
	m.LinkedAccounts = []model.LinkedAccount{{AccountID: 5, StoreID: 21, StoreCode: 2600000021, Matched: model.LinkedUBN, LinkedMeasureID: &linked, CanCreateLinkedTermination: true}, {AccountID: 6, StoreID: 22, StoreCode: 2600000022, Matched: model.LinkedUBN}}
	cause := model.RevisionStoreClosed
	m.Revisions = []model.MeasureRevision{{Revision: 1, StatementToSeller: "s", AdminID: adminIDOf(7), CreatedAt: governanceNow}, {Revision: 2, StatementToSeller: "s", SystemCause: &cause, CreatedAt: governanceNow}}
	out := toAdminMeasure(m)
	if out.GetKind() != pb.MeasureKind_MEASURE_KIND_TERMINATION || out.GetBasis() != pb.MeasureBasis_MEASURE_BASIS_CONTRACT ||
		out.GetTerminationGround() != pb.TerminationGround_TERMINATION_GROUND_LINKED_ACCOUNT || out.GetSourceMeasureId() != source.String() ||
		out.GetState() != pb.MeasureState_MEASURE_STATE_SCHEDULED {
		t.Fatalf("out=%v", out)
	}
	if !out.GetPeriod().GetStartTime().AsTime().Equal(m.StartsAt) || out.GetPeriod().GetEndTime() != nil ||
		!out.GetAppealDeadlineTime().AsTime().Equal(m.StartsAt.Add(model.AppealWindow)) {
		t.Fatalf("period=%v deadline=%v", out.GetPeriod(), out.GetAppealDeadlineTime())
	}
	if len(out.GetLinkedAccounts()) != 2 || out.GetLinkedAccounts()[0].GetCanCreateLinkedTermination() != true || out.GetLinkedAccounts()[1].GetCanCreateLinkedTermination() {
		t.Fatalf("linked=%v", out.GetLinkedAccounts())
	}
	l := out.GetLinkedAccounts()[0]
	if l.GetAccountId() != "5" || l.GetStoreId() != "21" || l.GetStoreCode() != "2600000021" || l.GetMatchedIdentity() != adminpb.LinkedIdentity_LINKED_IDENTITY_UBN || l.GetLinkedMeasureId() != linked.String() {
		t.Fatalf("linked=%v", l)
	}
	if r := out.GetRevisions(); r[0].GetCreatedByAdminId() != "7" || r[0].GetSystemCause() != adminpb.MeasureRevisionSystemCause_MEASURE_REVISION_SYSTEM_CAUSE_UNSPECIFIED ||
		r[1].GetCreatedByAdminId() != "" || r[1].GetSystemCause() != adminpb.MeasureRevisionSystemCause_MEASURE_REVISION_SYSTEM_CAUSE_STORE_CLOSED {
		t.Fatalf("revisions=%v", r)
	}
	if out.GetCanExtend() || out.GetCanShorten() || out.GetCanEnd() || out.GetCanScheduleReview() || !out.GetCanRevoke() || !out.GetCanUpdateStatement() {
		t.Fatalf("終止合作的操作：%v", out)
	}
	if plain := toAdminMeasure(riskMeasureFixture()); plain.GetTerminationGround() != pb.TerminationGround_TERMINATION_GROUND_UNSPECIFIED || plain.GetAppealDeadlineTime() != nil ||
		plain.GetSourceMeasureId() != "" || len(plain.GetLinkedAccounts()) != 0 {
		t.Fatalf("停權沒有終止的欄位：%v", plain)
	}
}

// 建立措施的種類就是設定的哪一支輸入：每一支轉成模組的種類與只屬於它的欄位（沒有種類欄位可以和依據對不上）；模組的欄位錯誤依設定的那一支
// 對應到請求裡的路徑（同一個模組欄位在不同種類的輸入下路徑不同）。
// mutation：停權的輸入轉成 Kind = 警告 → 種類斷言失敗；停權的 EndsAt 路徑指到 warning.violation 之類的別支 → 路徑斷言失敗。
func TestCreateMeasureKindIsTheInputThatIsSet(t *testing.T) {
	violation := &adminpb.ViolationBasis{CitedFindingIds: []string{uuid.NewV7().String()}}
	risk := &adminpb.SuspensionInput_Risk{Risk: &adminpb.RiskBasis{Ground: pb.RiskGround_RISK_GROUND_FRAUD}}
	ends := timestamppb.New(governanceNow.Add(time.Hour))
	for name, tc := range map[string]struct {
		input    *adminpb.CreateMeasureRequest
		kind     model.MeasureKind
		violated bool
		risk     bool
		termin   bool
		endsAt   bool
	}{
		"警告": {&adminpb.CreateMeasureRequest{Measure: &adminpb.CreateMeasureRequest_Warning{Warning: &adminpb.WarningInput{Violation: violation}}}, model.MeasureWarning, true, false, false, false},
		"依據認定的停權": {&adminpb.CreateMeasureRequest{Measure: &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{
			Basis: &adminpb.SuspensionInput_Violation{Violation: violation}, EndTime: ends}}}, model.MeasureSuspension, true, false, false, true},
		"依據風險的停權": {&adminpb.CreateMeasureRequest{Measure: &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{Basis: risk, ReviewBy: ends}}},
			model.MeasureSuspension, false, true, false, false},
		"終止合作": {&adminpb.CreateMeasureRequest{Measure: &adminpb.CreateMeasureRequest_Termination{Termination: &adminpb.TerminationInput{
			Ground: pb.TerminationGround_TERMINATION_GROUND_NOTICE}}}, model.MeasureTermination, false, false, true, false},
	} {
		m := &governanceModule{measure: riskMeasureFixture()}
		h := NewGovernance(m, rpc.Images{})
		tc.input.StoreId, tc.input.Statement, tc.input.Note = "12", "s", "n"
		if _, err := h.CreateMeasure(adminCtx(42), connect.NewRequest(tc.input)); err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		c := m.create
		if c.Kind != tc.kind || (c.Violation != nil) != tc.violated || (c.Risk != nil) != tc.risk || (c.Termination != nil) != tc.termin || (c.EndsAt != nil) != tc.endsAt ||
			(c.ReviewBy != nil) != tc.risk {
			t.Errorf("%s：%+v", name, c)
		}
	}

	call := func(t *testing.T, req *adminpb.CreateMeasureRequest, moduleErr error) []string {
		t.Helper()
		m := &governanceModule{measure: riskMeasureFixture(), createErr: moduleErr}
		h := NewGovernance(m, rpc.Images{})
		wrapped := rpc.NewInterceptor(slog.New(slog.DiscardHandler)).WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			return h.CreateMeasure(ctx, connect.NewRequest(req))
		})
		_, err := wrapped(adminCtx(42), fakeRequest{procedure: sellerv1alpha1connect.SellerGovernanceAdminServiceCreateMeasureProcedure})
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("code = %v：%v", connect.CodeOf(err), err)
		}
		return reportErrFields(t, err)
	}
	warning := &adminpb.CreateMeasureRequest{StoreId: "12", Measure: &adminpb.CreateMeasureRequest_Warning{Warning: &adminpb.WarningInput{Violation: violation}}}
	suspension := &adminpb.CreateMeasureRequest{StoreId: "12", Measure: &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{Basis: risk}}}
	violationSuspension := &adminpb.CreateMeasureRequest{StoreId: "12", Measure: &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{
		Basis: &adminpb.SuspensionInput_Violation{Violation: violation}}}}
	termination := &adminpb.CreateMeasureRequest{StoreId: "12", Measure: &adminpb.CreateMeasureRequest_Termination{Termination: &adminpb.TerminationInput{Ground: pb.TerminationGround_TERMINATION_GROUND_NOTICE}}}
	for name, tc := range map[string]struct {
		req   *adminpb.CreateMeasureRequest
		field string
		want  string
	}{
		"警告的依據":          {warning, "Violation", "warning.violation"},
		"警告引用的第二筆認定":     {warning, "Violation.CitedFindingIDs[1]", "warning.violation.cited_finding_ids[1]"},
		"停權依據認定的結束時間":    {violationSuspension, "EndsAt", "suspension.end_time"},
		"停權依據認定的新認定類別":   {violationSuspension, "Violation.Finding.Category", "suspension.violation.finding.category"},
		"新認定的第二筆來源通報":    {violationSuspension, "Violation.Finding.SourceReportIDs[1]", "suspension.violation.finding.source_report_ids[1]"},
		"新認定的來源通報":       {violationSuspension, "Violation.Finding.SourceReportIDs", "suspension.violation.finding.source_report_ids"},
		"停權依據風險的複審期限":    {suspension, "ReviewBy", "suspension.review_by"},
		"停權依據風險的類型":      {suspension, "Risk.Ground", "suspension.risk.ground"},
		"終止的事由":          {termination, "Termination.Ground", "termination.ground"},
		"終止引用的認定":        {termination, "Termination.Violation.CitedFindingIDs", "termination.violation.cited_finding_ids"},
		"終止的來源措施":        {termination, "Termination.SourceMeasureID", "termination.source_measure_id"},
		"給賣家的理由（每一種都一樣）": {termination, "StatementToSeller", "statement"},
	} {
		if got := call(t, tc.req, apperr.InvalidInputOn([]string{tc.field}, "x")); !slices.Equal(got, []string{tc.want}) {
			t.Errorf("%s：%v，要 [%s]", name, got, tc.want)
		}
	}
}

// 已讀補充：帶通報 id 與讀到的 etag 照轉（不需要管理員 id 以外的資料）；沒帶 etag 不呼叫模組；回應是模組回的通報。
func TestMarkReportSupplementsSeenForwardsTheEtag(t *testing.T) {
	report := anonymousFoodSafetyReport()
	m := &governanceModule{report: report}
	h := NewGovernance(m, rpc.Images{})
	got, err := h.MarkReportSupplementsSeen(adminCtx(42), connect.NewRequest(&adminpb.MarkReportSupplementsSeenRequest{Id: report.ID.String(), Etag: "9"}))
	if err != nil || m.markSeen.ReportID != report.ID || m.markSeen.ExpectedVersion != 9 || got.Msg.GetReport().GetId() != report.ID.String() {
		t.Fatalf("in=%+v err=%v", m.markSeen, err)
	}
	m.calls = 0
	if _, err := h.MarkReportSupplementsSeen(adminCtx(42), connect.NewRequest(&adminpb.MarkReportSupplementsSeenRequest{Id: report.ID.String()})); err == nil || m.calls != 0 {
		t.Fatalf("沒有 etag 不能呼叫模組：calls=%d err=%v", m.calls, err)
	}
}

// 內容下架的分支：種類與對象交給模組，依據一定是認定；模組的欄位錯誤對應到 content_takedown 的路徑；回應帶對象。
func TestCreateContentTakedownMapsTheBranchAndTheTargets(t *testing.T) {
	measure := riskMeasureFixture()
	measure.Kind, measure.Basis, measure.RiskGround, measure.EndsAt, measure.ReviewBy, measure.State = model.MeasureContentTakedown, model.BasisViolation, nil, nil, nil, model.StateIssued
	measure.Requests = nil
	measure.Contents = []model.ContentRef{{Kind: model.ContentPost, ID: uuid.NewV7()}, {Kind: model.ContentForumEntry, ID: uuid.NewV7()}}
	m := &governanceModule{measure: measure}
	post, forum := uuid.NewV7(), uuid.NewV7()
	h := NewGovernance(m, rpc.Images{})
	got, err := h.CreateMeasure(adminCtx(42), connect.NewRequest(&adminpb.CreateMeasureRequest{
		StoreId: "12", Statement: "s", Note: "d",
		Measure: &adminpb.CreateMeasureRequest_ContentTakedown{ContentTakedown: &adminpb.ContentTakedownInput{
			Contents: []*pb.ContentRef{
				{Kind: pb.ContentKind_CONTENT_KIND_POST, Id: post.String()},
				{Kind: pb.ContentKind_CONTENT_KIND_FORUM_ENTRY, Id: forum.String()},
			},
			Violation: &adminpb.ViolationBasis{Finding: &adminpb.NewFinding{
				Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM, Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW}},
		}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	c := m.create
	if c.Kind != model.MeasureContentTakedown || c.Takedown == nil || c.Termination != nil || c.Risk != nil || c.EndsAt != nil ||
		!slices.Equal(c.Takedown.Contents, []model.ContentRef{{Kind: model.ContentPost, ID: post}, {Kind: model.ContentForumEntry, ID: forum}}) ||
		c.Violation == nil || c.Violation.Finding == nil || c.Violation.Finding.Category != policy.Spam {
		t.Fatalf("create=%+v takedown=%+v", c, c.Takedown)
	}
	out := got.Msg.GetMeasure()
	if out.GetKind() != pb.MeasureKind_MEASURE_KIND_CONTENT_TAKEDOWN || out.GetPeriod() != nil || len(out.GetContents()) != 2 ||
		out.GetContents()[1].GetKind() != pb.ContentKind_CONTENT_KIND_FORUM_ENTRY || out.GetContents()[1].GetId() != measure.Contents[1].ID.String() {
		t.Fatalf("measure=%v", out)
	}
	for _, end := range [][]*adminpb.MeasureActionUnavailability{out.GetExtendUnavailabilities(), out.GetShortenUnavailabilities(), out.GetEndUnavailabilities()} {
		if len(end) != 1 || end[0].GetReason() != adminpb.MeasureActionUnavailableReason_MEASURE_ACTION_UNAVAILABLE_REASON_CONTENT_TAKEDOWN_REVOKE_ONLY ||
			end[0].GetMessage().GetMessage() != "內容下架沒有結束時間，要讓內容恢復請撤銷措施，或接受賣家的申覆" {
			t.Fatalf("unavailabilities=%v", end)
		}
	}
	if out.GetCanEnd() || out.GetCanExtend() || out.GetCanShorten() || !out.GetCanRevoke() {
		t.Fatalf("can_*=%v", out)
	}
}

// 內容下架：social 的訂閱者回歸屬錯誤時，欄位路徑指到請求裡那一則（不依賴排序後的順序）；其他錯誤不被改寫。
func TestCreateContentTakedownMapsTheOwnershipErrorToTheOffendingItem(t *testing.T) {
	own, bad := uuid.NewV7(), uuid.NewV7()
	req := &adminpb.CreateMeasureRequest{
		StoreId: "12", Statement: "s", Note: "d",
		Measure: &adminpb.CreateMeasureRequest_ContentTakedown{ContentTakedown: &adminpb.ContentTakedownInput{
			Contents: []*pb.ContentRef{
				{Kind: pb.ContentKind_CONTENT_KIND_POST, Id: own.String()},
				{Kind: pb.ContentKind_CONTENT_KIND_COMMENT, Id: bad.String()},
			},
			Violation: &adminpb.ViolationBasis{Finding: &adminpb.NewFinding{
				Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM, Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW}},
		}},
	}
	m := &governanceModule{createErr: fmt.Errorf("events：處理 MeasureCreated：%w", &social.TakedownContentError{Kind: model.ContentComment, ID: bad})}
	h := NewGovernance(m, rpc.Images{})
	wrapped := rpc.NewInterceptor(slog.New(slog.DiscardHandler)).WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return h.CreateMeasure(ctx, connect.NewRequest(req))
	})
	_, err := wrapped(adminCtx(42), fakeRequest{procedure: sellerv1alpha1connect.SellerGovernanceAdminServiceCreateMeasureProcedure})
	if got := reportErrFields(t, err); connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(got, []string{"content_takedown.contents[1]"}) {
		t.Fatalf("err=%v fields=%v", err, got)
	}
}
