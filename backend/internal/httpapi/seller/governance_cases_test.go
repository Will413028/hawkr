package seller

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func outcomesOf(r *adminpb.Report) []pb.ReportOutcome {
	var out []pb.ReportOutcome
	for _, o := range r.GetAvailableOutcomes() {
		out = append(out, o.GetOutcome())
	}
	return out
}

// 通報的 can_*、available_outcomes 與 can_cite_as_source 取自模組的 Availability（不輸出原因）：待分案的可以分案、不能移動、不能設為已採取措施；
// 案件裡有導出措施的可以設為已採取措施；被否定的結果與已關閉的店家不能當認定的來源。
func TestReportCarriesTheModulesAvailabilityWithoutReasons(t *testing.T) {
	awaiting := anonymousFoodSafetyReport()
	got := toReport(awaiting)
	if !got.GetCanTriage() || got.GetCanMove() || !got.GetCanResolve() || !got.GetCanCiteAsSource() || got.GetState() != adminpb.ReportState_REPORT_STATE_AWAITING_TRIAGE ||
		!slices.Equal(outcomesOf(got), []pb.ReportOutcome{
			pb.ReportOutcome_REPORT_OUTCOME_NO_VIOLATION, pb.ReportOutcome_REPORT_OUTCOME_INSUFFICIENT, pb.ReportOutcome_REPORT_OUTCOME_DUPLICATE, pb.ReportOutcome_REPORT_OUTCOME_ABUSIVE,
		}) {
		t.Fatalf("待分案：%v", got)
	}
	caseID := uuid.NewV7()
	assessed := policy.FoodSafety
	inCase := awaiting.Report
	inCase.CaseID, inCase.AssessedCategory = &caseID, &assessed
	facts := model.CaseFacts{Case: model.Case{ID: caseID, Category: policy.FoodSafety, ReportCount: 2}, ResultingMeasures: 1}
	view := model.AdminReport{Report: inCase, Availability: model.ReportFacts{Report: inCase, Case: &facts}.Availability()}
	got = toReport(view)
	if got.GetCanTriage() || !got.GetCanMove() || !got.GetCanResolve() || got.GetState() != adminpb.ReportState_REPORT_STATE_IN_CASE || got.GetCaseId() != caseID.String() ||
		got.GetAssessedCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_FOOD_SAFETY ||
		!slices.Equal(outcomesOf(got), []pb.ReportOutcome{
			pb.ReportOutcome_REPORT_OUTCOME_ACTION_TAKEN, pb.ReportOutcome_REPORT_OUTCOME_NO_VIOLATION, pb.ReportOutcome_REPORT_OUTCOME_INSUFFICIENT,
			pb.ReportOutcome_REPORT_OUTCOME_DUPLICATE, pb.ReportOutcome_REPORT_OUTCOME_ABUSIVE,
		}) {
		t.Fatalf("案件裡、案件有導出的措施：%v", got)
	}
	abusive := model.OutcomeAbusive
	resolved := awaiting.Report
	resolved.Outcome = &abusive
	view = model.AdminReport{Report: resolved, Availability: model.ReportFacts{Report: resolved, StoreClosed: true}.Availability()}
	got = toReport(view)
	if got.GetCanTriage() || got.GetCanMove() || got.GetCanResolve() || got.GetCanCiteAsSource() || len(got.GetAvailableOutcomes()) != 0 ||
		got.GetState() != adminpb.ReportState_REPORT_STATE_RESOLVED || got.GetOutcome() != pb.ReportOutcome_REPORT_OUTCOME_ABUSIVE {
		t.Fatalf("已有結果（濫報）、店家已關閉：%v", got)
	}
}

// 案件的 can_*、decision_options 取自模組的 Availability；decision_options 帶結案時會帶入沒有結果的通報的結果（client 不自己對應）。
func TestCaseCarriesTheModulesAvailabilityAndDecisionOptions(t *testing.T) {
	for name, c := range map[string]struct {
		facts   model.CaseFacts
		options []adminpb.CaseDecision
		want    []pb.ReportOutcome
	}{
		"沒有導出的措施也沒有已採取措施的成員": {model.CaseFacts{}, []adminpb.CaseDecision{adminpb.CaseDecision_CASE_DECISION_NO_ACTION}, []pb.ReportOutcome{pb.ReportOutcome_REPORT_OUTCOME_NO_VIOLATION}},
		"有未撤銷的導出措施":          {model.CaseFacts{ResultingMeasures: 1, UnrevokedMeasures: 1}, []adminpb.CaseDecision{adminpb.CaseDecision_CASE_DECISION_ACTION_TAKEN}, []pb.ReportOutcome{pb.ReportOutcome_REPORT_OUTCOME_ACTION_TAKEN}},
		"導出的措施都撤銷了":          {model.CaseFacts{ResultingMeasures: 1}, []adminpb.CaseDecision{adminpb.CaseDecision_CASE_DECISION_ACTION_TAKEN, adminpb.CaseDecision_CASE_DECISION_NO_ACTION}, []pb.ReportOutcome{pb.ReportOutcome_REPORT_OUTCOME_ACTION_TAKEN, pb.ReportOutcome_REPORT_OUTCOME_NO_VIOLATION}},
		"已有成員的結果是已採取措施":      {model.CaseFacts{ActionTakenReports: 1}, []adminpb.CaseDecision{adminpb.CaseDecision_CASE_DECISION_ACTION_TAKEN}, []pb.ReportOutcome{pb.ReportOutcome_REPORT_OUTCOME_ACTION_TAKEN}},
	} {
		got := toCase(caseFixture(c.facts))
		var options []adminpb.CaseDecision
		var defaults []pb.ReportOutcome
		for _, o := range got.GetDecisionOptions() {
			options, defaults = append(options, o.GetDecision()), append(defaults, o.GetDefaultOutcome())
		}
		if !slices.Equal(options, c.options) || !slices.Equal(defaults, c.want) || !got.GetCanDecide() {
			t.Errorf("%s：options=%v defaults=%v can_decide=%v", name, options, defaults, got.GetCanDecide())
		}
		if got.GetState() != adminpb.CaseState_CASE_STATE_OPEN || !got.GetCanChangeSeverity() || !got.GetCanLinkMeasure() || !got.GetCanCreateMeasure() ||
			got.GetEtag() != "6" {
			t.Errorf("%s：進行中的案件 %v", name, got)
		}
	}
	// 已結案：什麼決定都不能做、不能改嚴重度與連結措施。
	closedAt, reason, decision := governanceNow, model.CaseClosedDecided, model.DecisionNoAction
	closed := caseFixture(model.CaseFacts{Case: model.Case{ClosedAt: &closedAt, CloseReason: &reason, Decision: &decision}, StoreClosed: true})
	got := toCase(closed)
	if got.GetCanDecide() || len(got.GetDecisionOptions()) != 0 || got.GetCanChangeSeverity() || got.GetCanLinkMeasure() || got.GetCanCreateMeasure() ||
		got.GetState() != adminpb.CaseState_CASE_STATE_CLOSED ||
		got.GetCloseReason() != adminpb.CaseCloseReason_CASE_CLOSE_REASON_DECIDED || got.GetDecision() != adminpb.CaseDecision_CASE_DECISION_NO_ACTION {
		t.Fatalf("已結案：%v", got)
	}
	// 以沒有通報關閉的案件沒有決定；店家已關閉時進行中的案件不能建立警告、停權與風險措施，仍可連結終止合作。
	noReports := model.CaseClosedNoReports
	got = toCase(caseFixture(model.CaseFacts{Case: model.Case{ClosedAt: &closedAt, CloseReason: &noReports}}))
	if got.GetDecision() != adminpb.CaseDecision_CASE_DECISION_UNSPECIFIED {
		t.Fatalf("以沒有通報關閉：%v", got)
	}
	got = toCase(caseFixture(model.CaseFacts{StoreClosed: true}))
	if got.GetCanCreateMeasure() || !got.GetCanLinkMeasure() {
		t.Fatalf("店家已關閉：%v", got)
	}
}

// 回應從不帶原因：沒有任何欄位說「為什麼不能」（K4′：按鈕依 can_* 隱藏，不顯示原因）。
func TestAvailabilityIsNeverSerializedAsReasons(t *testing.T) {
	raw := protojson.Format(toCase(caseFixture(model.CaseFacts{}))) + protojson.Format(toReport(anonymousFoodSafetyReport()))
	var reasons []string
	for _, b := range model.ReportBlockValues() {
		reasons = append(reasons, string(b))
	}
	for _, b := range model.CaseBlockValues() {
		reasons = append(reasons, string(b))
	}
	for _, reason := range reasons {
		// 原因的代碼（NO_ACTION_BASIS、NEEDS_CASE…）與句子都不在回應裡；ACTION_TAKEN 是結果與決定的值，不是原因。
		if reason != "ACTION_TAKEN" && strings.Contains(raw, reason) {
			t.Errorf("回應不得帶原因 %s：%s", reason, raw)
		}
	}
}

// 改狀態的指令一律要帶 etag（決定依據通報或案件的內容，補充會在狀態不變時改變內容）；沒帶的請求不呼叫模組。
func TestStateChangingReportAndCaseCommandsRequireAnEtag(t *testing.T) {
	m := &governanceModule{report: anonymousFoodSafetyReport(), cases: caseFixture(model.CaseFacts{})}
	h := NewGovernance(m, nil, rpc.Images{})
	ctx, id, caseID := adminCtx(7), uuid.NewV7().String(), uuid.NewV7().String()
	for name, call := range map[string]func() error{
		"triage": func() error {
			_, err := h.TriageReport(ctx, connect.NewRequest(&adminpb.TriageReportRequest{Id: id, AssessedCategory: commonpb.PolicyCategory_POLICY_CATEGORY_FOOD_SAFETY}))
			return err
		},
		"resolve": func() error {
			_, err := h.ResolveReport(ctx, connect.NewRequest(&adminpb.ResolveReportRequest{Id: id, Outcome: pb.ReportOutcome_REPORT_OUTCOME_ABUSIVE}))
			return err
		},
		"move": func() error {
			_, err := h.MoveReport(ctx, connect.NewRequest(&adminpb.MoveReportRequest{Id: id, AssessedCategory: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD}))
			return err
		},
		"decide": func() error {
			_, err := h.DecideCase(ctx, connect.NewRequest(&adminpb.DecideCaseRequest{Id: caseID, Decision: adminpb.CaseDecision_CASE_DECISION_NO_ACTION}))
			return err
		},
		"severity": func() error {
			_, err := h.ChangeCaseSeverity(ctx, connect.NewRequest(&adminpb.ChangeCaseSeverityRequest{Id: caseID, Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW, Note: "原因"}))
			return err
		},
	} {
		wantInvalidArgument(t, call())
		if m.calls != 0 {
			t.Fatalf("%s：沒有 etag 的指令不能呼叫模組", name)
		}
	}
	// 不帶 etag 的是寫註記（只新增）。
	if _, err := h.CreateCaseNote(ctx, connect.NewRequest(&adminpb.CreateCaseNoteRequest{CaseId: caseID, Note: &adminpb.GovernanceNote{Body: "x"}})); err != nil {
		t.Fatal(err)
	}
}

// 分案與移動：去處（加入進行中的案件或開新案件）原樣轉給模組，由模組驗證意圖此刻仍然成立；被移動的原案件關閉會帶出來。
func TestMoveReportReportsTheClosedSourceCase(t *testing.T) {
	report := anonymousFoodSafetyReport()
	m := &governanceModule{report: report, moveResult: model.MoveReportResult{Report: report, SourceCaseClosed: true}}
	h := NewGovernance(m, nil, rpc.Images{})
	got, err := h.MoveReport(adminCtx(3), connect.NewRequest(&adminpb.MoveReportRequest{
		Id: report.ID.String(), AssessedCategory: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD, Etag: "9",
		Target: &adminpb.CaseTarget{Target: &adminpb.CaseTarget_OpenNew{OpenNew: &adminpb.OpenNewCase{Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_MEDIUM}}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Msg.GetSourceCaseClosed() || m.move.Target.OpenNew == nil || *m.move.Target.OpenNew != model.SeverityMedium || m.move.Target.Join != nil || m.move.AssessedCategory != policy.Fraud {
		t.Fatalf("got=%v move=%+v", got.Msg, m.move)
	}
}

// 案件清單與詳情：篩選照轉給模組，page token 綁在篩選上；案件的移動與「有新的補充」依模組的值轉出。
func TestListCasesBindsThePageTokenAndGetCaseCarriesMoves(t *testing.T) {
	cursor := &model.CaseCursor{OpenedAt: governanceNow, ID: uuid.NewV7()}
	c := caseFixture(model.CaseFacts{})
	c.UnseenSupplementReports = 1
	c.Moves = []model.ReportMove{{ReportID: uuid.NewV7(), FromCaseID: uuid.NewV7(), ToCaseID: c.ID, AdminID: 5, MovedAt: governanceNow}}
	c.Availability = model.CaseFacts{Case: c.Case}.Availability()
	m := &governanceModule{cases: c, casePage: model.AdminCasePage{Items: []model.AdminCase{c}, Next: cursor}}
	h := NewGovernance(m, nil, rpc.Images{})
	req := &adminpb.ListCasesRequest{State: adminpb.CaseState_CASE_STATE_OPEN, Category: commonpb.PolicyCategory_POLICY_CATEGORY_FOOD_SAFETY, StoreId: "12", PageSize: 2}
	got, err := h.ListCases(adminCtx(1), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if in := m.caseList; *in.State != model.CaseStateOpen || *in.Category != policy.FoodSafety || in.StoreID != 12 || in.PageSize != 2 || len(got.Msg.GetCases()) != 1 {
		t.Fatalf("in=%+v", in)
	}
	req.PageToken = got.Msg.GetNextPageToken()
	if _, err := h.ListCases(adminCtx(1), connect.NewRequest(req)); err != nil || m.caseList.After == nil || *m.caseList.After != *cursor {
		t.Fatalf("after=%+v err=%v", m.caseList.After, err)
	}
	for name, change := range map[string]func(*adminpb.ListCasesRequest){
		"state":    func(r *adminpb.ListCasesRequest) { r.State = adminpb.CaseState_CASE_STATE_CLOSED },
		"category": func(r *adminpb.ListCasesRequest) { r.Category = commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD },
		"store":    func(r *adminpb.ListCasesRequest) { r.StoreId = "13" },
	} {
		changed := proto.Clone(req).(*adminpb.ListCasesRequest)
		change(changed)
		_, err := h.ListCases(adminCtx(1), connect.NewRequest(changed))
		t.Run(name, func(t *testing.T) { wantInvalidArgument(t, err) })
	}
	one, err := h.GetCase(adminCtx(1), connect.NewRequest(&adminpb.GetCaseRequest{Id: c.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	pc := one.Msg.GetCase()
	if len(pc.GetMoves()) != 1 || pc.GetMoves()[0].GetMovedByAdminId() != "5" || pc.GetUnseenSupplementReportCount() != 1 {
		t.Fatalf("case=%v", pc)
	}
}

// 案件註記：管理員取自呼叫者、不帶 etag；page token 綁在案件上；改嚴重度的註記帶改前與改後，措施的註記不帶。
func TestCaseNotesUseTheCallingAdminAndCarryTheSeverityChange(t *testing.T) {
	caseID := uuid.NewV7()
	cursor := &model.NoteCursor{CreatedAt: governanceNow, ID: uuid.NewV7()}
	m := &governanceModule{notePage: model.NotePage{Items: []model.GovernanceNote{
		{ID: uuid.NewV7(), AdminID: 7, Body: "改嚴重度", CreatedAt: governanceNow, SeverityChange: &model.SeverityChange{From: model.SeverityMedium, To: model.SeverityHigh}},
		{ID: uuid.NewV7(), AdminID: 8, Body: "一般註記", CreatedAt: governanceNow},
	}, Next: cursor}}
	h := NewGovernance(m, nil, rpc.Images{})
	created, err := h.CreateCaseNote(adminCtx(42), connect.NewRequest(&adminpb.CreateCaseNoteRequest{
		CaseId: caseID.String(), Note: &adminpb.GovernanceNote{
			Body: "補充", CreatedByAdminId: "1", SeverityChange: &adminpb.SeverityChange{FromSeverity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW, ToSeverity: pb.ViolationSeverity_VIOLATION_SEVERITY_HIGH},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if m.caseNote.CaseID != caseID || m.caseNote.AdminID != 42 || m.caseNote.Body != "補充" || created.Msg.GetNote().GetCreatedByAdminId() != "42" || created.Msg.GetNote().GetSeverityChange() != nil {
		t.Fatalf("note=%+v got=%v（severity_change 只由 ChangeCaseSeverity 寫，這裡不採用）", m.caseNote, created.Msg)
	}
	req := &adminpb.ListCaseNotesRequest{CaseId: caseID.String(), PageSize: 2}
	got, err := h.ListCaseNotes(adminCtx(1), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	notes := got.Msg.GetNotes()
	if len(notes) != 2 || notes[0].GetSeverityChange().GetFromSeverity() != pb.ViolationSeverity_VIOLATION_SEVERITY_MEDIUM ||
		notes[0].GetSeverityChange().GetToSeverity() != pb.ViolationSeverity_VIOLATION_SEVERITY_HIGH || notes[1].GetSeverityChange() != nil || m.caseNotes.CaseID != caseID {
		t.Fatalf("notes=%v in=%+v", notes, m.caseNotes)
	}
	req.PageToken = got.Msg.GetNextPageToken()
	if _, err := h.ListCaseNotes(adminCtx(1), connect.NewRequest(req)); err != nil || m.caseNotes.After == nil || *m.caseNotes.After != *cursor {
		t.Fatalf("after=%+v err=%v", m.caseNotes.After, err)
	}
	req.CaseId = uuid.NewV7().String()
	_, err = h.ListCaseNotes(adminCtx(1), connect.NewRequest(req))
	wantInvalidArgument(t, err)
}

// 建立措施與措施清單的新欄位：source_case_id 進模組輸入、格式不對不呼叫模組；兩個來源篩選綁在 page token 上。
func TestSourceCaseAndReportFiltersReachTheModule(t *testing.T) {
	m := &governanceModule{measure: riskMeasureFixture()}
	h := NewGovernance(m, nil, rpc.Images{})
	caseID, reportID := uuid.NewV7(), uuid.NewV7()
	if _, err := h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{SourceCaseId: caseID.String(), SourceReportId: reportID.String()})); err != nil {
		t.Fatal(err)
	}
	if in := m.measureList; in.SourceCaseID == nil || *in.SourceCaseID != caseID || in.SourceReportID == nil || *in.SourceReportID != reportID {
		t.Fatalf("in=%+v", in)
	}
	_, err := h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{SourceCaseId: "not-a-uuid"}))
	wantInvalidArgument(t, err)
	_, err = h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{SourceReportId: "x"}))
	wantInvalidArgument(t, err)
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListMeasuresProcedure, "", "", "MEASURE_STATE_UNSPECIFIED",
		"MEASURE_BASIS_UNSPECIFIED", "MEASURE_REQUEST_STATE_UNSPECIFIED", "MEASURE_REQUEST_KIND_UNSPECIFIED", "false", caseID.String(), "", "")
	token := rpc.TokenFor(scope, &model.MeasureCursor{SortAt: governanceNow, ID: uuid.NewV7()})
	if _, err := h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token, SourceCaseId: caseID.String()})); err != nil {
		t.Fatalf("同一組篩選可以沿用：%v", err)
	}
	_, err = h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token, SourceCaseId: caseID.String(), SourceReportId: reportID.String()}))
	wantInvalidArgument(t, err)
	_, err = h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token}))
	wantInvalidArgument(t, err)

	_, err = h.CreateMeasure(adminCtx(1), connect.NewRequest(&adminpb.CreateMeasureRequest{
		StoreId: "12", Statement: "s", Note: "n", SourceCaseId: "oops",
		Measure: &adminpb.CreateMeasureRequest_Warning{Warning: &adminpb.WarningInput{Violation: &adminpb.ViolationBasis{Finding: &adminpb.NewFinding{
			Category: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD, Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW,
		}}}},
	}))
	wantInvalidArgument(t, err)
	calls := m.calls
	if _, err := h.CreateMeasure(adminCtx(1), connect.NewRequest(&adminpb.CreateMeasureRequest{
		StoreId: "12", Statement: "s", Note: "n", SourceCaseId: caseID.String(),
		Measure: &adminpb.CreateMeasureRequest_Warning{Warning: &adminpb.WarningInput{Violation: &adminpb.ViolationBasis{Finding: &adminpb.NewFinding{
			Category: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD, Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW,
		}}}},
	})); err != nil || m.calls != calls+1 || m.create.SourceCaseID == nil || *m.create.SourceCaseID != caseID {
		t.Fatalf("create=%+v err=%v", m.create, err)
	}
}

// 建立措施帶的來源案件有問題（不存在或不是這家店的）時，模組以 SourceCaseID 回 invalid_argument；每一種措施的欄位對應都要把它指到請求的 source_case_id。
// 少了對應，client 拿到的 invalid_argument 沒有 BadRequest，interceptor 還會以 error 等級記錄「欄位沒有附給 client」（handler 的 bug）。
// 錯誤經過和線上相同的攔截器才轉成 connect 錯誤（欄位路徑在那裡對應）。模組端的欄位名由整合測試 TestCreateMeasureCanLinkAnyKindToAnOpenCaseOfTheSameStore 鎖住。
func TestCreateMeasureSourceCaseErrorsPointAtSourceCaseIDForEveryKind(t *testing.T) {
	finding := &adminpb.ViolationBasis{Finding: &adminpb.NewFinding{
		Category: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD, Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW,
	}}
	later := timestamppb.New(governanceNow.Add(7 * 24 * time.Hour))
	withSource := func(req *adminpb.CreateMeasureRequest) *adminpb.CreateMeasureRequest {
		req.StoreId, req.Statement, req.Note, req.SourceCaseId = "12", "s", "n", uuid.NewV7().String()
		return req
	}
	for name, req := range map[string]*adminpb.CreateMeasureRequest{
		"警告": withSource(&adminpb.CreateMeasureRequest{Measure: &adminpb.CreateMeasureRequest_Warning{Warning: &adminpb.WarningInput{Violation: finding}}}),
		"依據認定的停權": withSource(&adminpb.CreateMeasureRequest{Measure: &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{
			Basis: &adminpb.SuspensionInput_Violation{Violation: finding}, EndTime: later,
		}}}),
		"風險停權": withSource(&adminpb.CreateMeasureRequest{Measure: &adminpb.CreateMeasureRequest_Suspension{Suspension: &adminpb.SuspensionInput{
			Basis: &adminpb.SuspensionInput_Risk{Risk: &adminpb.RiskBasis{Ground: pb.RiskGround_RISK_GROUND_FOOD_SAFETY}}, ReviewBy: later,
		}}}),
		"終止合作": withSource(&adminpb.CreateMeasureRequest{Measure: &adminpb.CreateMeasureRequest_Termination{Termination: &adminpb.TerminationInput{
			Ground: pb.TerminationGround_TERMINATION_GROUND_FOOD_SAFETY_INCIDENT,
		}}}),
	} {
		m := &governanceModule{measure: riskMeasureFixture(), createErr: apperr.InvalidInputOn([]string{"SourceCaseID"}, "找不到這個案件。")}
		h := NewGovernance(m, nil, rpc.Images{})
		var logs strings.Builder
		call := rpc.NewInterceptor(slog.New(slog.NewTextHandler(&logs, nil))).WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			return h.CreateMeasure(ctx, connect.NewRequest(req))
		})
		_, err := call(adminCtx(1), fakeRequest{procedure: sellerv1alpha1connect.SellerGovernanceAdminServiceCreateMeasureProcedure})
		if m.calls != 1 || connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：calls=%d code=%v err=%v", name, m.calls, connect.CodeOf(err), err)
			continue
		}
		if got := reportErrFields(t, err); !slices.Equal(got, []string{"source_case_id"}) {
			t.Errorf("%s：BadRequest 的欄位 %v，要 [source_case_id]", name, got)
		}
		if strings.Contains(logs.String(), "欄位沒有附給 client") {
			t.Errorf("%s：欄位沒有對應到請求的路徑：%s", name, logs.String())
		}
	}
}

// 伺服器輸出 client 本來要自己推導的東西（設計審查 #1、#4、#8）：每個可設定的結果帶 negates_source（同 Outcome.Negates）、生效的類別、分案與移動的去處
// （有進行中的案件帶 id／嚴重度／etag，沒有就只有類別）、有沒有沒看過的補充（最新一則的編號大於看過的編號）。
func TestReportCarriesTheDerivedFactsTheClientUsedToCompute(t *testing.T) {
	awaiting := anonymousFoodSafetyReport()
	caseID := uuid.NewV7()
	awaiting.SupplementCount, awaiting.LatestSupplementSeq, awaiting.SupplementsSeenSeq = 3, 3, 2
	awaiting.Targets = []model.TriageTarget{
		{Category: policy.Fraud, Case: &model.Case{ID: caseID, Severity: model.SeverityHigh, Version: 9, ReportCount: 3}},
		{Category: policy.Privacy},
	}
	got := toReport(awaiting)
	wantNegates := map[pb.ReportOutcome]bool{
		pb.ReportOutcome_REPORT_OUTCOME_NO_VIOLATION: true, pb.ReportOutcome_REPORT_OUTCOME_INSUFFICIENT: true,
		pb.ReportOutcome_REPORT_OUTCOME_DUPLICATE: false, pb.ReportOutcome_REPORT_OUTCOME_ABUSIVE: true,
	}
	if len(got.GetAvailableOutcomes()) != len(wantNegates) {
		t.Fatalf("outcomes=%v", got.GetAvailableOutcomes())
	}
	for _, o := range got.GetAvailableOutcomes() {
		if want, ok := wantNegates[o.GetOutcome()]; !ok || o.GetNegatesSource() != want {
			t.Errorf("%v：negates_source=%v，預期 %v", o.GetOutcome(), o.GetNegatesSource(), want)
		}
	}
	if got.GetEffectiveCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_FOOD_SAFETY || !got.GetHasUnseenSupplements() {
		t.Fatalf("effective=%v unseen=%v", got.GetEffectiveCategory(), got.GetHasUnseenSupplements())
	}
	targets := got.GetTriageTargets()
	if len(targets) != 2 || targets[0].GetCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD || targets[0].GetCaseId() != caseID.String() ||
		targets[0].GetSeverity() != pb.ViolationSeverity_VIOLATION_SEVERITY_HIGH || targets[0].GetCaseEtag() != "9" || targets[0].GetCaseReportCount() != 3 ||
		targets[1].GetCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_PRIVACY || targets[1].GetCaseId() != "" || targets[1].GetCaseEtag() != "" ||
		targets[1].GetSeverity() != pb.ViolationSeverity_VIOLATION_SEVERITY_UNSPECIFIED {
		t.Fatalf("targets=%v", targets)
	}
	// 在案件裡：生效的類別是平台判定類別（不是檢舉人選的）；補充看過之後沒有提醒。
	assessed := policy.Fraud
	inCase := awaiting.Report
	inCase.AssessedCategory, inCase.SupplementsSeenSeq = &assessed, 3
	view := toReport(model.AdminReport{Report: inCase, Availability: awaiting.Availability})
	if view.GetEffectiveCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD || view.GetHasUnseenSupplements() {
		t.Fatalf("effective=%v unseen=%v", view.GetEffectiveCategory(), view.GetHasUnseenSupplements())
	}
}

// 分案與移動的去處（oneof）：加入帶案件 id 與 etag、開新案件帶嚴重度；格式不對的案件 id 與 etag 是 invalid_argument（指到 target 底下的欄位）；
// 沒有設去處的原樣交給模組，由它以 target 欄位錯誤回應（去處是否成立由模組在鎖內驗證）。
func TestTriageAndMoveForwardTheExplicitTarget(t *testing.T) {
	report := anonymousFoodSafetyReport()
	m := &governanceModule{report: report, moveResult: model.MoveReportResult{Report: report}}
	h := NewGovernance(m, nil, rpc.Images{})
	caseID := uuid.NewV7()
	join := func(id, etag string) *adminpb.CaseTarget {
		return &adminpb.CaseTarget{Target: &adminpb.CaseTarget_Join{Join: &adminpb.JoinCase{CaseId: id, Etag: etag}}}
	}
	triage := func(target *adminpb.CaseTarget) error {
		_, err := h.TriageReport(adminCtx(3), connect.NewRequest(&adminpb.TriageReportRequest{
			Id: report.ID.String(), AssessedCategory: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD, Etag: "4", Target: target,
		}))
		return err
	}
	if err := triage(join(caseID.String(), "7")); err != nil || m.triage.Target.Join == nil || m.triage.Target.Join.CaseID != caseID || m.triage.Target.Join.ExpectedVersion != 7 || m.triage.Target.OpenNew != nil {
		t.Fatalf("join：in=%+v err=%v", m.triage.Target, err)
	}
	for name, target := range map[string]*adminpb.CaseTarget{
		"案件 id 格式不對": join("not-a-uuid", "7"),
		"沒有 etag":    join(caseID.String(), ""),
		"etag 不是數字":  join(caseID.String(), "x"),
	} {
		t.Run(name, func(t *testing.T) { wantInvalidArgument(t, triage(target)) })
	}
	m.triage = model.TriageReportInput{}
	if err := triage(nil); err != nil || m.triage.Target.Join != nil || m.triage.Target.OpenNew != nil || m.triage.ReportID != report.ID {
		t.Fatalf("沒有去處要原樣交給模組：in=%+v err=%v", m.triage, err)
	}
	_, err := h.MoveReport(adminCtx(3), connect.NewRequest(&adminpb.MoveReportRequest{
		Id: report.ID.String(), AssessedCategory: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD, Etag: "4", Target: join("x", "1"),
	}))
	wantInvalidArgument(t, err)
}
