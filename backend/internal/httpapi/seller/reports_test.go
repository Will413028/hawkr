package seller

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// supplementNotice 是模組給登入檢舉人的「補充到原本檢舉」那一句（模組的 default 句子 report.duplicate_supplemented；這裡只看 handler 原樣轉出）。
const supplementNotice = "你今天已經檢舉過這家店的這個類別，這次的內容已經補充到原本的檢舉。"

// reportModule 是檢舉入口的模組替身：supplement 決定 SubmitReport 是新增還是補充到原本那一筆（匿名的補充不帶 Report）。
type reportModule struct {
	supplement   bool
	err          error
	submitted    []model.SubmitReportInput
	detail       model.Report
	readBuyer    int64
	readID       uuid.UUID
	supplemented []model.AddSupplementInput
}

func (m *reportModule) SubmitReport(_ context.Context, in model.SubmitReportInput) (model.SubmitReportResult, error) {
	m.submitted = append(m.submitted, in)
	if m.err != nil {
		return model.SubmitReportResult{}, m.err
	}
	r := model.Report{ID: uuid.NewV7(), StoreID: in.StoreID, Category: in.Category, Description: in.Description, CreatedAt: governanceNow}
	if m.supplement {
		result := model.SubmitReportResult{ReportID: r.ID, Supplemented: true, Notice: uitext.Message{Text: supplementNotice, Locale: uitext.ZhTW}}
		result.Report = &r
		return result, nil
	}
	return model.SubmitReportResult{ReportID: r.ID, Report: &r}, nil
}

func (m *reportModule) BuyerReport(_ context.Context, buyerID int64, id uuid.UUID) (model.Report, error) {
	m.readBuyer, m.readID = buyerID, id
	return m.detail, m.err
}

func (m *reportModule) AddReportSupplement(_ context.Context, in model.AddSupplementInput) (model.Report, error) {
	if m.err != nil {
		return model.Report{}, m.err
	}
	m.supplemented = append(m.supplemented, in)
	return m.detail, nil
}

type buyersOf map[int64]int64

func (b buyersOf) CurrentBuyerID(_ context.Context, accountID int64) (int64, error) {
	if id, ok := b[accountID]; ok {
		return id, nil
	}
	return 0, apperr.NotFound("沒有買家")
}

func newTestReports(t *testing.T, m ReportModule, buyers Buyers) *Reports {
	t.Helper()
	h, err := NewReports(m, buyers, &limiterFake{})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func accountCtx(id int64) context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: id})
}

// 登入的檢舉以呼叫者的買家為檢舉人、帳號為佐證的上傳者與「不能檢舉自己的店」的比對；去重期間內的重複送出補充到原本那一筆，回成功與那一句。
func TestCreateReportUsesTheCallersBuyer(t *testing.T) {
	m := &reportModule{}
	h := newTestReports(t, m, buyersOf{5: 500})
	req := &pb.CreateReportRequest{StoreId: "12", Category: commonpb.PolicyCategory_POLICY_CATEGORY_COUNTERFEIT, Description: "仿冒", EvidenceStagedTokens: []string{"t1"}}
	got, err := h.CreateReport(accountCtx(5), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if in := m.submitted[0]; in.ReporterBuyerID != 500 || in.AccountID != 5 || in.StoreID != 12 || in.Category != policy.Counterfeit ||
		len(in.EvidenceTokens) != 1 {
		t.Fatalf("in=%+v", in)
	}
	if got.Msg.GetReport().Outcome != nil || got.Msg.GetReport().GetStoreId() != "12" ||
		got.Msg.GetSupplemented() || got.Msg.GetNotice() != nil {
		t.Fatalf("report=%v", got.Msg)
	}
	m.supplement = true
	again, err := h.CreateReport(accountCtx(5), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("補充到原本那一筆是成功：%v", err)
	}
	if !again.Msg.GetSupplemented() || again.Msg.GetReport() == nil || again.Msg.GetNotice().GetMessage() != supplementNotice || again.Msg.GetNotice().GetLocale() != "zh-TW" {
		t.Fatalf("again=%v", again.Msg)
	}
	if _, err := h.CreateReport(accountCtx(6), connect.NewRequest(req)); err == nil || apperr.KindOf(err) == apperr.KindNotFound {
		t.Fatalf("存活的帳號沒有買家是不變量被破壞（internal），不是 not_found：%v", err)
	}
}

func TestGetMyReportUnresolvedHasNoOutcome(t *testing.T) {
	reportID := uuid.NewV7()
	m := &reportModule{detail: model.Report{ID: reportID, StoreID: 12, Category: policy.Fraud, Description: "d", CreatedAt: governanceNow}}
	h := newTestReports(t, m, buyersOf{5: 500})
	got, err := h.GetMyReport(accountCtx(5), connect.NewRequest(&pb.GetMyReportRequest{ReportId: reportID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	if r := got.Msg.GetReport(); r.Outcome != nil || r.GetResolveTime() != nil {
		t.Fatalf("outcome=%v resolve_time=%v", r.Outcome, r.GetResolveTime())
	}
}

// GetMyReport 以呼叫者的買家為範圍，回應帶補充與佐證的順序，不帶平台內部的分案（案件、嚴重度、處理的管理員）；模組的 NotFound 原樣轉出。
func TestGetMyReportIsScopedToTheCallersBuyer(t *testing.T) {
	admin := int64(9)
	assessed := policy.Fraud
	caseID := uuid.NewV7()
	outcome := model.OutcomeDuplicate
	supplementID := uuid.NewV7()
	reportID := uuid.NewV7()
	m := &reportModule{detail: model.Report{
		ID: reportID, StoreID: 12, Category: policy.Fraud, Description: "d", Outcome: &outcome, ResolvedAt: &governanceNow,
		CaseID: &caseID, AssessedCategory: &assessed, TriagedByAdminID: &admin, CreatedAt: governanceNow,
		Supplements: []model.Supplement{{ID: supplementID, Description: "more", CreatedAt: governanceNow, EvidencePositions: []int{1}}},
		Evidence:    []model.Evidence{{Position: 0}, {Position: 1, SupplementID: &supplementID}},
	}}
	h := newTestReports(t, m, buyersOf{5: 500, 6: 600})
	got, err := h.GetMyReport(accountCtx(5), connect.NewRequest(&pb.GetMyReportRequest{ReportId: reportID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	if m.readBuyer != 500 || m.readID != reportID || got.Msg.GetReport().GetOutcome() != pb.ReportOutcome_REPORT_OUTCOME_DUPLICATE ||
		len(got.Msg.GetSupplements()) != 1 || got.Msg.GetSupplements()[0].GetEvidencePositions()[0] != 1 || len(got.Msg.GetEvidence()) != 2 {
		t.Fatalf("buyer=%d got=%v", m.readBuyer, got.Msg)
	}
	raw := protojson.Format(got.Msg)
	for _, leaked := range []string{caseID.String(), "caseId", "assessed", "triage", "severity", "objectKey"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("檢舉人看不到案件、平台判定類別與嚴重度（%s）：%s", leaked, raw)
		}
	}
	m.err = apperr.NotFound("x")
	if _, err := h.GetMyReport(accountCtx(6), connect.NewRequest(&pb.GetMyReportRequest{ReportId: reportID.String()})); apperr.KindOf(err) != apperr.KindNotFound || m.readBuyer != 600 {
		t.Fatalf("buyer=%d err=%v", m.readBuyer, err)
	}
	_, err = h.GetMyReport(accountCtx(5), connect.NewRequest(&pb.GetMyReportRequest{ReportId: "nope"}))
	wantInvalidArgument(t, err)
}

// AddReportSupplement 以呼叫者的買家與帳號寫入，額度與 CreateReport 同樣的兩個時間窗（各自計數）；超過額度不呼叫模組。
func TestAddReportSupplementUsesTheCallersBuyerAndQuota(t *testing.T) {
	reportID := uuid.NewV7()
	m := &reportModule{detail: model.Report{ID: reportID, StoreID: 12, Category: policy.Fraud, CreatedAt: governanceNow}}
	limiter := &limiterFake{}
	h, err := NewReports(m, buyersOf{5: 500}, limiter)
	if err != nil {
		t.Fatal(err)
	}
	req := &pb.AddReportSupplementRequest{ReportId: reportID.String(), Description: "補充", EvidenceStagedTokens: []string{"t1"}}
	got, err := h.AddReportSupplement(accountCtx(5), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if in := m.supplemented[0]; in.ReportID != reportID || in.BuyerID != 500 || in.AccountID != 5 || in.Description != "補充" || len(in.EvidenceTokens) != 1 ||
		got.Msg.GetReport().GetId() != reportID.String() {
		t.Fatalf("in=%+v got=%v", in, got.Msg)
	}
	if len(limiter.calls) != len(ReportSubmitQuotas) || !strings.Contains(limiter.calls[0].key, sellerv1alpha1connect.SellerReportAppServiceAddReportSupplementProcedure) {
		t.Fatalf("calls=%+v", limiter.calls)
	}
	limiter.calls, limiter.deny = nil, true
	if _, err := h.AddReportSupplement(accountCtx(5), connect.NewRequest(req)); apperr.KindOf(err) != apperr.KindResourceExhausted || len(m.supplemented) != 1 {
		t.Fatalf("超過額度：%v supplemented=%d", err, len(m.supplemented))
	}
	limiter.deny = false
	m.err = apperr.FailedPrecondition("closed")
	if _, err := h.AddReportSupplement(accountCtx(5), connect.NewRequest(req)); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("模組的 failed_precondition 原樣轉出：%v", err)
	}
	_, err = h.AddReportSupplement(accountCtx(5), connect.NewRequest(&pb.AddReportSupplementRequest{ReportId: "nope", Description: "x"}))
	wantInvalidArgument(t, err)
}

// 登入通報的帳號額度：兩個時間窗在同一次 Allow 一起計（key 帶帳號與時間窗；全部在額度內才都計，由 limiter 保證），任何一個
// 超過就回 resource_exhausted，不呼叫模組。匿名的入口不在 handler 計數。
func TestCreateReportCountsBothAccountQuotasTogether(t *testing.T) {
	m := &reportModule{}
	limiter := &limiterFake{}
	h, err := NewReports(m, buyersOf{5: 500}, limiter)
	if err != nil {
		t.Fatal(err)
	}
	req := &pb.CreateReportRequest{StoreId: "12", Category: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD, Description: "x"}
	if _, err := h.CreateReport(accountCtx(5), connect.NewRequest(req)); err != nil {
		t.Fatal(err)
	}
	if len(limiter.calls) != len(ReportSubmitQuotas) {
		t.Fatalf("calls=%+v", limiter.calls)
	}
	for i, q := range ReportSubmitQuotas {
		c := limiter.calls[i]
		if c.limit != q.Limit || c.window != q.Window || !strings.HasSuffix(c.key, "|"+q.Window.String()+"|5") {
			t.Fatalf("第 %d 個時間窗：%+v", i, c)
		}
	}
	limiter.calls, limiter.deny = nil, true
	if _, err := h.CreateReport(accountCtx(5), connect.NewRequest(req)); apperr.KindOf(err) != apperr.KindResourceExhausted || len(m.submitted) != 1 || len(limiter.calls) != len(ReportSubmitQuotas) {
		t.Fatalf("超過額度：%v submitted=%d calls=%d", err, len(m.submitted), len(limiter.calls))
	}
	if _, err := NewReports(m, buyersOf{}, nil); err == nil {
		t.Fatal("沒有 limiter 不能建立")
	}
}

func reportErrFields(t *testing.T, err error) []string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 connect 錯誤：%v", err)
	}
	var out []string
	for _, d := range ce.Details() {
		v, e := d.Value()
		if e != nil {
			t.Fatal(e)
		}
		if br, ok := v.(*errdetails.BadRequest); ok {
			for _, fv := range br.GetFieldViolations() {
				out = append(out, fv.GetField())
			}
		}
	}
	return out
}

// 模組把被拒絕的第二張佐證指成 EvidenceTokens[1]；對外是 BadRequest 的 evidence_staged_tokens[1]。
func TestCreateReportPointsAtTheRejectedEvidenceToken(t *testing.T) {
	m := &reportModule{err: apperr.InvalidInputOn([]string{"EvidenceTokens[1]"}, "圖片無法使用，請重新上傳")}
	h := newTestReports(t, m, buyersOf{5: 50})
	req := &pb.CreateReportRequest{StoreId: "12", Category: commonpb.PolicyCategory_POLICY_CATEGORY_FRAUD, Description: "x", EvidenceStagedTokens: []string{"ok", "expired"}}
	// 錯誤經過和線上相同的攔截器才轉成 connect 錯誤（欄位路徑在那裡對應）。
	call := rpc.NewInterceptor(slog.New(slog.DiscardHandler)).WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return h.CreateReport(ctx, connect.NewRequest(req))
	})
	_, err := call(accountCtx(5), fakeRequest{procedure: sellerv1alpha1connect.SellerReportAppServiceCreateReportProcedure})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("code = %v", connect.CodeOf(err))
	}
	if got := reportErrFields(t, err); !slices.Equal(got, []string{"evidence_staged_tokens[1]"}) {
		t.Fatalf("fields = %v", got)
	}
}

// 對應表的每個路徑都要在請求訊息裡存在，proto 改名時這裡會失敗。
func TestReportPathsExistInTheRequests(t *testing.T) {
	for name, c := range map[string]struct {
		paths rpc.FieldPaths
		req   interface{ ProtoReflect() protoreflect.Message }
	}{
		"create": {createReportPaths, &pb.CreateReportRequest{}},
	} {
		if missing := rpc.CheckFieldPaths(c.req.ProtoReflect().Descriptor(), c.paths); len(missing) > 0 {
			t.Errorf("%s：不存在的路徑 %v", name, missing)
		}
	}
}
