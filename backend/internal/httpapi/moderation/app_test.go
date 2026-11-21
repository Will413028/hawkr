package moderation

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1/moderationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

type fakeReports struct {
	in       []moderation.SubmitInput
	result   moderation.SubmitResult
	page     moderation.ReportPage
	err      error
	gotBuyer int64
	gotSize  int32
	gotAfter *moderation.ReportCursor
}

func (f *fakeReports) SubmitReport(_ context.Context, in moderation.SubmitInput) (moderation.SubmitResult, error) {
	f.in = append(f.in, in)
	return f.result, f.err
}

func (f *fakeReports) MyReports(_ context.Context, b int64, after *moderation.ReportCursor, size int32) (moderation.ReportPage, error) {
	f.gotBuyer, f.gotSize, f.gotAfter = b, size, after
	return f.page, f.err
}

type fakeBuyers struct{}

func (fakeBuyers) CurrentBuyerID(context.Context, int64) (int64, error) { return 7, nil }

type env struct {
	r      *fakeReports
	client moderationv1alpha1connect.ModerationAppServiceClient
	url    string
	logs   *bytes.Buffer
}

// newEnv 掛上真的 Connect handler 與 rpc 的 interceptor；header X-Test-Account 非空時以買家帳號登入，X-Test-Admin 非空時以管理員登入。
func newEnv(t *testing.T, r *fakeReports) *env {
	t.Helper()
	return newEnvWith(t, r, &fakeStores{})
}

// newEnvWith 以指定的社群與店家來源掛上 handler；env.r 只在來源是 *fakeReports 時有值。
func newEnvWith(t *testing.T, reports Reports, stores StoreReports) *env {
	t.Helper()
	r, _ := reports.(*fakeReports)
	logs := &bytes.Buffer{}
	path, handler := moderationv1alpha1connect.NewModerationAppServiceHandler(NewApp(reports, stores, fakeBuyers{}), connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.NewTextHandler(logs, nil)))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case req.Header.Get("X-Test-Account") != "":
			req = req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 7}))
		case req.Header.Get("X-Test-Admin") != "":
			req = req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 7}))
		}
		handler.ServeHTTP(rw, req)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &env{r: r, logs: logs, url: srv.URL, client: moderationv1alpha1connect.NewModerationAppServiceClient(srv.Client(), srv.URL)}
}

func signedIn[T any](msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("X-Test-Account", "1")
	return req
}

func TestModerationAppRequiresALogin(t *testing.T) {
	e := newEnv(t, &fakeReports{})
	methods := pb.File_hawkr_moderation_v1alpha1_moderation_proto.Services().ByName("ModerationAppService").Methods()
	if methods.Len() != 2 {
		t.Fatalf("ModerationAppService 有 %d 支方法，預期 2", methods.Len())
	}
	for i := range methods.Len() {
		name := string(methods.Get(i).Name())
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, e.url+"/hawkr.moderation.v1alpha1.ModerationAppService/"+name, strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s 未登入回 %d，預期 401", name, resp.StatusCode)
		}
	}
	if len(e.r.in) != 0 || e.r.gotBuyer != 0 {
		t.Fatalf("未登入不該到模組：%v", e.r.in)
	}
}

func TestOnlyABuyerAccountCanCall(t *testing.T) {
	e := newEnv(t, &fakeReports{})
	req := connect.NewRequest(&pb.ListMyReportsRequest{})
	req.Header().Set("X-Test-Admin", "1")
	if _, err := e.client.ListMyReports(context.Background(), req); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("非買家身分：%v", err)
	}
}

func TestSubmitReportSendsTheCallersBuyerAndRequestIDAndReturnsOnlyTheReportIDAndTime(t *testing.T) {
	at := time.Date(2026, 11, 1, 3, 4, 5, 0, time.UTC)
	reportID, caseID := uuid.New(), uuid.New()
	r := &fakeReports{result: moderation.SubmitResult{Report: moderation.Report{ID: reportID, CaseID: caseID, CreatedAt: at}, Case: moderation.Case{ID: caseID}, Created: true}}
	e := newEnv(t, r)
	commentID, rid := uuid.New(), uuid.New()
	resp, err := e.client.SubmitReport(context.Background(), signedIn(&pb.SubmitReportRequest{
		Target:      &pb.ReportTarget{Kind: pb.ReportTargetKind_REPORT_TARGET_KIND_COMMENT, Id: commentID.String()},
		Category:    commonpb.PolicyCategory_POLICY_CATEGORY_SPAM,
		Description: "廣告",
		RequestId:   rid.String(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.in) != 1 {
		t.Fatalf("模組呼叫 %d 次", len(r.in))
	}
	in := r.in[0]
	if in.ReporterBuyerID != 7 || in.Target != (moderation.Target{Kind: moderation.TargetComment, ContentID: commentID}) || in.Category != policy.Spam || in.Description != "廣告" || in.RequestID == nil || *in.RequestID != rid {
		t.Fatalf("送給模組的輸入：%+v", in)
	}
	if resp.Msg.GetReportId() != reportID.String() || !resp.Msg.GetCreateTime().AsTime().Equal(at) {
		t.Fatalf("回應：%v", resp.Msg)
	}
	if strings.Contains(resp.Msg.String(), caseID.String()) {
		t.Fatalf("回應不能露案件 ID：%v", resp.Msg)
	}
	if n := pb.File_hawkr_moderation_v1alpha1_moderation_proto.Messages().ByName("SubmitReportResponse").Fields().Len(); n != 2 {
		t.Fatalf("SubmitReportResponse 有 %d 個欄位，預期只有 report_id 與 create_time", n)
	}
}

func TestSubmitReportBuyerTargetUsesTheIntegerID(t *testing.T) {
	r := &fakeReports{}
	e := newEnv(t, r)
	_, err := e.client.SubmitReport(context.Background(), signedIn(&pb.SubmitReportRequest{
		Target:   &pb.ReportTarget{Kind: pb.ReportTargetKind_REPORT_TARGET_KIND_BUYER, Id: "42"},
		Category: commonpb.PolicyCategory_POLICY_CATEGORY_HARASSMENT,
	}))
	if err != nil || len(r.in) != 1 || r.in[0].Target != (moderation.Target{Kind: moderation.TargetBuyer, BuyerID: 42}) || r.in[0].RequestID != nil {
		t.Fatalf("買家對象：%v %+v", err, r.in)
	}
}

func TestSubmitReportRejectsMalformedTargetsAndRequestIDsBeforeTheModule(t *testing.T) {
	r := &fakeReports{}
	e := newEnv(t, r)
	good := &pb.ReportTarget{Kind: pb.ReportTargetKind_REPORT_TARGET_KIND_POST, Id: uuid.New().String()}
	for name, req := range map[string]*pb.SubmitReportRequest{
		"沒有種類":                 {Target: &pb.ReportTarget{Id: uuid.New().String()}, Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM},
		"內容 ID 不是 UUID":        {Target: &pb.ReportTarget{Kind: pb.ReportTargetKind_REPORT_TARGET_KIND_POST, Id: "123"}, Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM},
		"買家 ID 不是整數":           {Target: &pb.ReportTarget{Kind: pb.ReportTargetKind_REPORT_TARGET_KIND_BUYER, Id: "x"}, Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM},
		"request_id 不是 UUIDv4": {Target: good, Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM, RequestId: "not-a-uuid"},
	} {
		if _, err := e.client.SubmitReport(context.Background(), signedIn(req)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v", name, err)
		}
	}
	if len(r.in) != 0 {
		t.Fatalf("格式錯誤不該到模組：%v", r.in)
	}
}

func TestListMyReportsMapsCommunityItemsAndPages(t *testing.T) {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	closed := at.Add(time.Hour)
	id1, id2, target := uuid.UUID{1}, uuid.UUID{2}, uuid.UUID{3}
	r := &fakeReports{page: moderation.ReportPage{
		Items: []moderation.ReportItem{
			{Report: moderation.Report{ID: id1, Category: policy.Spam, Description: "廣告", CreatedAt: at, Outcome: moderation.OutcomeViolation}, Target: moderation.Target{Kind: moderation.TargetPost, ContentID: target}, ClosedAt: &closed},
			{Report: moderation.Report{ID: id2, Category: policy.Harassment, CreatedAt: at.Add(-time.Minute), Outcome: moderation.OutcomePending}, Target: moderation.Target{Kind: moderation.TargetBuyer, BuyerID: 9}},
		},
		Next: &moderation.ReportCursor{CreatedAt: at.Add(-time.Minute), ID: id2},
	}}
	e := newEnv(t, r)
	resp, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{PageSize: 5}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetReports()
	if len(got) != 2 {
		t.Fatalf("回 %d 筆", len(got))
	}
	a, b := got[0], got[1]
	if a.GetId() != id1.String() || a.GetCommunity().GetTarget().GetKind() != pb.ReportTargetKind_REPORT_TARGET_KIND_POST || a.GetCommunity().GetTarget().GetId() != target.String() ||
		a.GetCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_SPAM || a.GetDescription() != "廣告" || a.GetCommunity().GetOutcome() != pb.ReportOutcome_REPORT_OUTCOME_VIOLATION ||
		!a.GetCreateTime().AsTime().Equal(at) || !a.GetResolveTime().AsTime().Equal(closed) || a.GetStore() != nil {
		t.Fatalf("第一筆：%v", a)
	}
	if b.GetCommunity().GetTarget().GetKind() != pb.ReportTargetKind_REPORT_TARGET_KIND_BUYER || b.GetCommunity().GetTarget().GetId() != "9" || b.GetCommunity().GetOutcome() != pb.ReportOutcome_REPORT_OUTCOME_PENDING || b.ResolveTime != nil {
		t.Fatalf("第二筆：%v", b)
	}
	if r.gotBuyer != 7 || r.gotSize != 5 || resp.Msg.GetNextPageToken() == "" {
		t.Fatalf("要以呼叫者的買家身分讀、帶頁大小與下一頁 token：%d %d %q", r.gotBuyer, r.gotSize, resp.Msg.GetNextPageToken())
	}
	if _, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{PageToken: resp.Msg.GetNextPageToken()})); err != nil || r.gotAfter == nil || r.gotAfter.ID != id2 {
		t.Fatalf("游標：%v %+v", err, r.gotAfter)
	}
	if _, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{PageToken: "garbage"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("壞 token：%v", err)
	}
}

// 模組對頁大小的 InvalidInput 指到請求的 page_size（BadRequest 的欄位路徑）；錯誤由模組實際呼叫的 pagination.PageSize 產生，不在測試裡自己組欄位。
func TestListMyReportsPointsAtPageSizeWhenTheModuleRejectsIt(t *testing.T) {
	_, pageSizeErr := pagination.PageSize(-1, 20, 100)
	r := &fakeReports{err: pageSizeErr}
	e := newEnv(t, r)
	_, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{PageSize: -1}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("code = %v，err = %v", connect.CodeOf(err), err)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 connect 錯誤：%v", err)
	}
	var fields []string
	for _, d := range ce.Details() {
		v, e := d.Value()
		if e != nil {
			t.Fatal(e)
		}
		if br, ok := v.(*errdetails.BadRequest); ok {
			for _, fv := range br.GetFieldViolations() {
				fields = append(fields, fv.GetField())
			}
		}
	}
	if len(fields) != 1 || fields[0] != "page_size" {
		t.Fatalf("BadRequest 的欄位 = %v，預期 [page_size]", fields)
	}
	if strings.Contains(e.logs.String(), "level=ERROR") {
		t.Fatalf("欄位已附給 client，不該有 ERROR log：%s", e.logs.String())
	}
}
