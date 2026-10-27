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
	"google.golang.org/protobuf/proto"
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
	supplement bool
	err        error
	submitted  []model.SubmitReportInput
	listed     model.BuyerReportListInput
	page       model.ReportPage
}

func (m *reportModule) SubmitReport(_ context.Context, in model.SubmitReportInput) (model.SubmitReportResult, error) {
	m.submitted = append(m.submitted, in)
	if m.err != nil {
		return model.SubmitReportResult{}, m.err
	}
	r := model.Report{ID: uuid.NewV7(), StoreID: in.StoreID, Category: in.Category, Description: in.Description, CreatedAt: governanceNow}
	if m.supplement {
		result := model.SubmitReportResult{ReportID: r.ID, Supplemented: true, Notice: uitext.Message{Text: supplementNotice, Locale: uitext.ZhTW}}
		if in.ReporterBuyerID > 0 {
			result.Report = &r
		}
		return result, nil
	}
	return model.SubmitReportResult{ReportID: r.ID, Report: &r}, nil
}

func (m *reportModule) BuyerReports(_ context.Context, in model.BuyerReportListInput) (model.ReportPage, error) {
	m.listed = in
	return m.page, nil
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
		len(in.EvidenceTokens) != 1 || in.Phone+in.Email+in.Name != "" {
		t.Fatalf("in=%+v", in)
	}
	if got.Msg.GetReport().GetOutcome() != pb.ReportOutcome_REPORT_OUTCOME_UNSPECIFIED || got.Msg.GetReport().GetStoreId() != "12" ||
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

// 匿名的食安檢舉：新的、補充的與超過上限而丟掉的送出回應完全相同（不洩漏這支電話用過）；類別固定是食安；登入的呼叫者照樣以匿名記錄，只帶帳號給「不能檢舉自己的店」。
// 不在 handler 計帳號額度（匿名的額度在入口以 IP 計）。
func TestFoodSafetyReportResponsesDoNotRevealDuplicates(t *testing.T) {
	m := &reportModule{}
	h := newTestReports(t, m, buyersOf{})
	req := &pb.SubmitFoodSafetyReportRequest{StoreId: "12", Description: "吃了不舒服", Phone: "0912345678", Name: "王"}
	first, err := h.SubmitFoodSafetyReport(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	m.supplement = true
	again, err := h.SubmitFoodSafetyReport(accountCtx(5), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(first.Msg, again.Msg) || !proto.Equal(first.Msg, &pb.SubmitFoodSafetyReportResponse{}) {
		t.Fatalf("new=%v supplement=%v", first.Msg, again.Msg)
	}
	for i, in := range m.submitted {
		wantAccount := int64(0)
		if i == 1 {
			wantAccount = 5 // 登入的呼叫者：帳號只給「不能檢舉自己的店」，檢舉人仍是匿名
		}
		if in.Category != policy.FoodSafety || in.ReporterBuyerID != 0 || in.AccountID != wantAccount || in.Phone != "0912345678" || in.Name != "王" {
			t.Fatalf("in=%+v", in)
		}
	}
}

// 我的檢舉以呼叫者的買家為範圍，page token 綁在買家上；回應只帶結果，不帶平台內部的分案（案件、嚴重度、處理的管理員）。
func TestListMyReportsIsScopedToTheCallersBuyer(t *testing.T) {
	admin := int64(9)
	assessed := policy.Fraud
	caseID := uuid.NewV7()
	outcome := model.OutcomeDuplicate
	cursor := &model.ReportCursor{CreatedAt: governanceNow, ID: uuid.NewV7()}
	m := &reportModule{page: model.ReportPage{Items: []model.Report{{
		ID: uuid.NewV7(), StoreID: 12, Category: policy.Fraud, Description: "d", Outcome: &outcome, ResolvedAt: &governanceNow,
		CaseID: &caseID, AssessedCategory: &assessed, TriagedByAdminID: &admin, CreatedAt: governanceNow,
	}}, Next: cursor}}
	h := newTestReports(t, m, buyersOf{5: 500, 6: 600})
	got, err := h.ListMyReports(accountCtx(5), connect.NewRequest(&pb.ListMyReportsRequest{PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if m.listed.BuyerID != 500 || m.listed.PageSize != 1 || len(got.Msg.GetReports()) != 1 || got.Msg.GetReports()[0].GetOutcome() != pb.ReportOutcome_REPORT_OUTCOME_DUPLICATE ||
		got.Msg.GetReports()[0].GetResolveTime() == nil {
		t.Fatalf("listed=%+v got=%v", m.listed, got.Msg)
	}
	token := got.Msg.GetNextPageToken()
	if _, err := h.ListMyReports(accountCtx(5), connect.NewRequest(&pb.ListMyReportsRequest{PageToken: token})); err != nil || m.listed.After == nil || *m.listed.After != *cursor {
		t.Fatalf("after=%+v err=%v", m.listed.After, err)
	}
	_, err = h.ListMyReports(accountCtx(6), connect.NewRequest(&pb.ListMyReportsRequest{PageToken: token}))
	wantInvalidArgument(t, err)
	scope := rpc.PageScope(sellerv1alpha1connect.SellerReportAppServiceListMyReportsProcedure, 500)
	_, err = h.ListMyReports(accountCtx(5), connect.NewRequest(&pb.ListMyReportsRequest{PageToken: rpc.TokenFor(scope, &model.ReportCursor{})}))
	wantInvalidArgument(t, err)
	raw := protojson.Format(got.Msg)
	for _, leaked := range []string{caseID.String(), "caseId", "assessed", "triage", "severity"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("檢舉人看不到案件、平台判定類別與嚴重度（%s）：%s", leaked, raw)
		}
	}
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
	limiter.calls = nil
	if _, err := h.SubmitFoodSafetyReport(context.Background(), connect.NewRequest(&pb.SubmitFoodSafetyReportRequest{StoreId: "12", Description: "x", Phone: "0912345678"})); err != nil || len(limiter.calls) != 0 {
		t.Fatalf("匿名入口不以帳號計數：%v calls=%d", err, len(limiter.calls))
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
		"create":   {createReportPaths, &pb.CreateReportRequest{}},
		"foodsafe": {foodSafetyReportPaths, &pb.SubmitFoodSafetyReportRequest{}},
	} {
		if missing := rpc.CheckFieldPaths(c.req.ProtoReflect().Descriptor(), c.paths); len(missing) > 0 {
			t.Errorf("%s：不存在的路徑 %v", name, missing)
		}
	}
}
