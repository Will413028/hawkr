package seller

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func measureFixture() model.Measure {
	decision := "理由不足"
	at := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	end := at.Add(72 * time.Hour)
	ground := model.RiskFraud
	return model.Measure{
		ID: uuid.MustParse("0192f1c0-0000-7000-8000-000000000002"), AccountID: 7, StoreID: 11, Kind: model.MeasureSuspension, Basis: model.BasisRisk,
		RiskGround: &ground, ReviewBy: &end, Status: model.MeasureStatusEnforced, Version: 9, CurrentRevision: 2, CreatedByAdminID: 7771, CreatedAt: at, StartsAt: at,
		StatementToSeller: "商品說明不實", EndsAt: &end, State: model.StateActive,
		Findings: []model.Violation{{ID: uuid.MustParse("0192f1c0-0000-7000-8000-000000000003"), Category: policy.MisleadingInfo, Severity: model.SeverityMedium, CreatedByAdminID: 7772, CreatedAt: at}},
		Requests: []model.MeasureRequest{{
			ID: uuid.MustParse("0192f1c0-0000-7000-8000-000000000004"), Kind: model.RequestKindAppeal, State: model.RequestRejected, Reason: "我沒有",
			SubmittedAt: at, DecisionReason: &decision, DecidedAt: &at, DecidedByAdminID: int64Ptr(7773),
		}},
		Revisions: []model.MeasureRevision{{Revision: 1, StatementToSeller: "舊的理由不可外流", AdminID: int64Ptr(7774), CreatedAt: at}},
	}
}

// 賣家只看到給賣家的理由、依據的認定與期間；內部註記、複審期限與管理員的識別不對外，etag 由賣家看得到的版本產生。
func TestMeasureResponseOmitsInternalFields(t *testing.T) {
	m := measureFixture()
	out := toMeasure(uitext.ZhTW, m)
	if out.GetStatement() != "商品說明不實" || out.GetKind() != pb.MeasureKind_MEASURE_KIND_SUSPENSION || out.GetBasis() != pb.MeasureBasis_MEASURE_BASIS_RISK ||
		out.GetRiskGround() != pb.RiskGround_RISK_GROUND_FRAUD || out.GetState() != pb.MeasureState_MEASURE_STATE_ACTIVE ||
		out.GetRequests()[0].GetKind() != pb.MeasureRequestKind_MEASURE_REQUEST_KIND_APPEAL ||
		out.GetRequests()[0].GetState() != pb.MeasureRequestState_MEASURE_REQUEST_STATE_REJECTED || out.GetRequests()[0].GetDecisionReason() != "理由不足" ||
		out.GetRequests()[0].GetDecideTime() == nil || len(out.GetFindings()) != 1 || out.GetEtag() != "2" {
		t.Fatalf("out=%v", out)
	}
	if text := out.String(); strings.Contains(text, "舊的理由") || containsAdminID(text) {
		t.Fatalf("內部欄位出現在回應：%s", text)
	}
	m.Version = 10
	if toMeasure(uitext.ZhTW, m).GetEtag() != "2" {
		t.Fatal("賣家的 etag 不隨後台的版本改變")
	}
}

// can_appeal 取自模組的共用判斷（與 SubmitAppeal 的前置檢查同一個規則），不由 state 與 appeal 在傳輸層推導。
func TestMeasureCanAppealIsTheModulesSharedRule(t *testing.T) {
	base := measureFixture()
	base.Requests = nil
	appealed := measureFixture()
	revoked := base
	revoked.Status, revoked.State = model.MeasureStatusRevoked, model.StateRevoked
	ended := base
	ended.State = model.StateEnded
	for name, tc := range map[string]struct {
		m    model.Measure
		want bool
	}{"active": {base, true}, "ended": {ended, true}, "appealed": {appealed, false}, "revoked": {revoked, false}} {
		if got := toMeasure(uitext.ZhTW, tc.m).GetCanAppeal(); got != tc.want {
			t.Errorf("%s: can_appeal=%v want %v", name, got, tc.want)
		}
	}
}

// 期間：警告沒有；停權帶開始與結束，沒有固定期間時不設結束。
func TestMeasureResponseCarriesThePeriod(t *testing.T) {
	m := measureFixture()
	period := toMeasure(uitext.ZhTW, m).GetPeriod()
	if !period.GetStartTime().AsTime().Equal(m.CreatedAt) || !period.GetEndTime().AsTime().Equal(*m.EndsAt) {
		t.Fatalf("period=%v", period)
	}
	m.EndsAt = nil
	if period = toMeasure(uitext.ZhTW, m).GetPeriod(); period.GetStartTime() == nil || period.GetEndTime() != nil {
		t.Fatalf("沒有固定期間：%v", period)
	}
	m.Kind = model.MeasureWarning
	if toMeasure(uitext.ZhTW, m).GetPeriod() != nil {
		t.Fatal("警告沒有期間")
	}
}

// 措施與申覆以登入的帳號為範圍：模組收到的是呼叫者的帳號 ID（不是店家的）；page token 只對同一個帳號與同一組篩選有效。
func TestMeasureMethodsScopeByTheCallingAccount(t *testing.T) {
	v := measureFixture()
	m := &kycModule{measure: v, listOut: model.MeasurePage{Items: []model.Measure{v}, Next: &model.MeasureCursor{SortAt: v.CreatedAt, ID: v.ID}}}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	ctx := accountContext(7)
	list, err := h.ListMyMeasures(ctx, connect.NewRequest(&pb.ListMyMeasuresRequest{PageSize: 5, State: pb.MeasureState_MEASURE_STATE_ACTIVE}))
	if err != nil || len(list.Msg.GetMeasures()) != 1 || m.listIn.AccountID != 7 || m.listIn.PageSize != 5 || list.Msg.GetNextPageToken() == "" ||
		m.listIn.State == nil || *m.listIn.State != model.StateActive {
		t.Fatalf("resp=%v err=%v input=%+v", list, err, m.listIn)
	}
	next := &pb.ListMyMeasuresRequest{PageToken: list.Msg.GetNextPageToken(), State: pb.MeasureState_MEASURE_STATE_ACTIVE}
	if _, err := h.ListMyMeasures(ctx, connect.NewRequest(next)); err != nil || m.listIn.After == nil || m.listIn.After.ID != v.ID {
		t.Fatalf("err=%v input=%+v", err, m.listIn)
	}
	if _, err := h.ListMyMeasures(accountContext(8), connect.NewRequest(next)); err == nil {
		t.Fatal("別的帳號的 page token 被接受")
	}
	if _, err := h.ListMyMeasures(ctx, connect.NewRequest(&pb.ListMyMeasuresRequest{PageToken: list.Msg.GetNextPageToken()})); err == nil {
		t.Fatal("換了篩選仍接受上一組的 page token")
	}
	scope := rpc.PageScope(rpc.FilterScope(sellerv1alpha1connect.SellerAppServiceListMyMeasuresProcedure, pb.MeasureState_MEASURE_STATE_UNSPECIFIED.String()), 7)
	if _, err := h.ListMyMeasures(ctx, connect.NewRequest(&pb.ListMyMeasuresRequest{PageToken: rpc.TokenFor(scope, model.MeasureCursor{})})); err == nil || !strings.HasPrefix(err.Error(), "invalid_argument") {
		t.Fatalf("內容不合法的 page token：err=%v", err)
	}
	if _, err := h.GetMyMeasure(ctx, connect.NewRequest(&pb.GetMyMeasureRequest{MeasureId: v.ID.String()})); err != nil || m.getAccount != 7 || m.getID != v.ID {
		t.Fatalf("err=%v account=%d id=%v", err, m.getAccount, m.getID)
	}
	if _, err := h.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: v.ID.String(), Etag: "3", Reason: "我沒有", EvidenceStagedTokens: []string{"t1"}})); err != nil {
		t.Fatal(err)
	}
	if want := (model.SubmitAppealInput{AccountID: 7, MeasureID: v.ID, ExpectedVersion: 3, Reason: "我沒有", EvidenceTokens: []string{"t1"}}); !reflect.DeepEqual(m.appealIn, want) {
		t.Fatalf("input=%+v", m.appealIn)
	}
}

// 解除請求：請求照轉給模組（帳號是登入的呼叫者）、etag 必填；回應帶解除請求紀錄（新到舊，不帶佐證與裁定的管理員）、可否請求解除、冷卻的結束時間與
// 下一次被駁回會開始的冷卻長度（都由模組推導，不在傳輸層算）。
func TestRequestRemediationAndTheRemediationFieldsOfTheMeasure(t *testing.T) {
	v := measureFixture()
	read := v.CreatedAt.Add(time.Hour)
	rejected := func(n int, at time.Time, kind model.RequestKind) model.MeasureRequest {
		reason := "不足"
		return model.MeasureRequest{ID: measureTestID(n), Kind: kind, State: model.RequestRejected, Reason: "r", SubmittedAt: at, DecisionReason: &reason, DecidedAt: &at, DecidedByAdminID: int64Ptr(7775)}
	}
	closeCause := model.CloseCauseEnded
	v.ReadAt = read
	v.Requests = []model.MeasureRequest{
		{ID: measureTestID(10), Kind: model.RequestKindRemediation, State: model.RequestClosed, Reason: "c", SubmittedAt: read, DecidedAt: &read, CloseCause: &closeCause},
		rejected(11, read.Add(-time.Minute), model.RequestKindRemediation),
		rejected(12, read.Add(-2*time.Minute), model.RequestKindAppeal),
		rejected(13, read.Add(-3*time.Minute), model.RequestKindRemediation),
	}
	out := toMeasure(uitext.ZhTW, v)
	if got := out.GetRequests(); len(got) != 4 || got[0].GetCloseCause() != pb.MeasureRequestCloseCause_MEASURE_REQUEST_CLOSE_CAUSE_ENDED ||
		got[0].GetKind() != pb.MeasureRequestKind_MEASURE_REQUEST_KIND_REMEDIATION || got[1].GetDecisionReason() != "不足" ||
		got[2].GetKind() != pb.MeasureRequestKind_MEASURE_REQUEST_KIND_APPEAL || got[2].GetReason() != "r" {
		t.Fatalf("requests=%v", out.GetRequests())
	}
	if want := read.Add(-time.Minute).Add(7 * 24 * time.Hour); !out.GetNextRemediationTime().AsTime().Equal(want) || out.GetCanRequestRemediation() ||
		out.GetRejectionCooldown().AsDuration() != 14*24*time.Hour {
		t.Fatalf("cooldown: next=%v can=%v warning=%v", out.GetNextRemediationTime(), out.GetCanRequestRemediation(), out.GetRejectionCooldown())
	}
	next := read.Add(-time.Minute).Add(7 * 24 * time.Hour)
	wantTime := uitext.FormatDateTime(uitext.ZhTW, twtime.MinuteCeil(next))
	if us := out.GetRemediationUnavailabilities(); len(us) != 1 || us[0].GetReason() != pb.RemediationUnavailableReason_REMEDIATION_UNAVAILABLE_REASON_COOLDOWN ||
		us[0].GetMessage().GetLocale() != "zh-TW" || us[0].GetMessage().GetMessage() != "解除請求目前在冷卻期內，"+wantTime+" 後可再提出" {
		t.Fatalf("冷卻中的原因與句子：%v", us)
	}
	if containsAdminID(out.String()) {
		t.Fatalf("管理員的識別不得出現在賣家的回應：%s", out)
	}
	v.Requests = nil
	out = toMeasure(uitext.ZhTW, v)
	if !out.GetCanRequestRemediation() || out.GetNextRemediationTime() != nil || out.GetRejectionCooldown() != nil || len(out.GetRequests()) != 0 || len(out.GetRemediationUnavailabilities()) != 0 {
		t.Fatalf("沒有請求：%v", out)
	}
	v.Basis = model.BasisViolation
	if out = toMeasure(uitext.ZhTW, v); out.GetCanRequestRemediation() {
		t.Fatal("違規依據沒有補正")
	}

	m := &kycModule{measure: v}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	if _, err := h.RequestRemediation(accountContext(7), connect.NewRequest(&pb.RequestRemediationRequest{MeasureId: v.ID.String(), Etag: "2", Reason: "已改善", EvidenceStagedTokens: []string{"t1"}})); err != nil {
		t.Fatal(err)
	}
	if want := (model.RequestRemediationInput{AccountID: 7, MeasureID: v.ID, ExpectedVersion: 2, Reason: "已改善", EvidenceTokens: []string{"t1"}}); !reflect.DeepEqual(m.remedyIn, want) {
		t.Fatalf("input=%+v", m.remedyIn)
	}
	m.remedyIn = model.RequestRemediationInput{}
	_, err := h.RequestRemediation(accountContext(7), connect.NewRequest(&pb.RequestRemediationRequest{MeasureId: v.ID.String(), Reason: "x"}))
	if apperr.KindOf(err) != apperr.KindInvalidInput || m.remedyIn.MeasureID != uuid.Nil() {
		t.Fatalf("沒有 etag：err=%v input=%+v", err, m.remedyIn)
	}
}

func TestSubmitAppealRequiresAnEtag(t *testing.T) {
	m := &kycModule{}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	_, err := h.SubmitAppeal(accountContext(7), connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: uuid.NewV7().String(), Reason: "x"}))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("err=%v", err)
	}
	if m.appealIn.MeasureID != uuid.Nil() {
		t.Fatal("沒有 etag 的申訴不能呼叫模組")
	}
}

func TestMeasureMethodsRejectMalformedIDsAndMissingPrincipals(t *testing.T) {
	h := mustApp(t, &kycModule{}, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	for name, call := range map[string]func(context.Context) error{
		"get": func(ctx context.Context) error {
			_, err := h.GetMyMeasure(ctx, connect.NewRequest(&pb.GetMyMeasureRequest{MeasureId: "not-a-uuid"}))
			return err
		},
		"appeal": func(ctx context.Context) error {
			_, err := h.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: "", Reason: "x"}))
			return err
		},
	} {
		if err := call(accountContext(7)); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	for name, call := range map[string]func(context.Context) error{
		"list": func(ctx context.Context) error {
			_, err := h.ListMyMeasures(ctx, connect.NewRequest(&pb.ListMyMeasuresRequest{}))
			return err
		},
		"get": func(ctx context.Context) error {
			_, err := h.GetMyMeasure(ctx, connect.NewRequest(&pb.GetMyMeasureRequest{}))
			return err
		},
		"appeal": func(ctx context.Context) error {
			_, err := h.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{}))
			return err
		},
	} {
		if err := call(context.Background()); apperr.KindOf(err) != apperr.KindInternal {
			t.Errorf("%s：沒有呼叫者 err=%v", name, err)
		}
	}
}

func int64Ptr(v int64) *int64 { return &v }

// 賣家看到的終止合作：事由、申覆期限與不能申覆的原因由伺服器給；期間的開始時間是 starts_at（預告終止在未來）。期限那一刻起 can_appeal 是 false、
// 原因含 APPEAL_WINDOW_ENDED；來源措施 id 不輸出（那是另一個帳號的措施）。
func TestSellerTerminationCarriesGroundDeadlineAndAppealReasons(t *testing.T) {
	m := terminationFixture(model.GroundNotice, model.BasisContract)
	source := uuid.NewV7()
	m.SourceMeasureID = &source
	out := toMeasure(uitext.ZhTW, m)
	if out.GetKind() != pb.MeasureKind_MEASURE_KIND_TERMINATION || out.GetBasis() != pb.MeasureBasis_MEASURE_BASIS_CONTRACT || out.GetState() != pb.MeasureState_MEASURE_STATE_SCHEDULED ||
		out.GetTerminationGround() != pb.TerminationGround_TERMINATION_GROUND_NOTICE || !out.GetCanAppeal() || len(out.GetAppealUnavailabilities()) != 0 {
		t.Fatalf("out=%v", out)
	}
	deadline := m.StartsAt.Add(model.AppealWindow)
	if !out.GetPeriod().GetStartTime().AsTime().Equal(m.StartsAt) || !out.GetAppealDeadlineTime().AsTime().Equal(deadline) {
		t.Fatalf("period=%v deadline=%v", out.GetPeriod(), out.GetAppealDeadlineTime())
	}
	if strings.Contains(out.String(), source.String()) {
		t.Fatal("來源措施的 id 不對賣家輸出")
	}
	m.ReadAt = deadline
	out = toMeasure(uitext.ZhTW, m)
	if us := out.GetAppealUnavailabilities(); out.GetCanAppeal() || len(us) != 1 || us[0].GetReason() != pb.AppealUnavailableReason_APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED ||
		us[0].GetMessage().GetMessage() != "終止合作的申覆期限已過" || us[0].GetMessage().GetLocale() != "zh-TW" {
		t.Fatalf("期限那一刻：%v", out)
	}
	m.Status, m.State = model.MeasureStatusRevoked, model.StateRevoked
	us := toMeasure(uitext.ZhTW, m).GetAppealUnavailabilities()
	if len(us) != 2 || us[0].GetReason() != pb.AppealUnavailableReason_APPEAL_UNAVAILABLE_REASON_REVOKED || us[0].GetMessage().GetMessage() != "這筆措施已撤銷，不需要申覆" ||
		us[1].GetReason() != pb.AppealUnavailableReason_APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED {
		t.Fatalf("已撤銷且期滿的順序：%v", us)
	}
	if plain := toMeasure(uitext.ZhTW, measureFixture()); plain.GetAppealDeadlineTime() != nil || plain.GetTerminationGround() != pb.TerminationGround_TERMINATION_GROUND_UNSPECIFIED {
		t.Fatalf("停權沒有申覆期限：%v", plain)
	}
}

func measureTestID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("0192f1c0-0000-7000-8000-%012d", n))
}

func containsAdminID(text string) bool {
	for _, id := range []string{"7771", "7772", "7773", "7774", "7775"} {
		if strings.Contains(text, id) {
			return true
		}
	}
	return false
}
