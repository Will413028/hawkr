package moderation

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/adminauth"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/moderation/v1alpha1/moderationv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	sellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

type fakeCases struct {
	page      moderation.CasePage
	detail    moderation.CaseDetail
	err       error
	gotFilter moderation.CaseFilter
	gotAfter  *moderation.CaseCursor
	gotSize   int32
	gotID     uuid.UUID
	decided   []moderation.DecideInput
	reversed  []moderation.ReverseInput
	snapshot  *moderation.Snapshot
	calls     map[string]int
}

func (f *fakeCases) called(method string) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[method]++
}

func (f *fakeCases) Cases(_ context.Context, fl moderation.CaseFilter, after *moderation.CaseCursor, size int32) (moderation.CasePage, error) {
	f.called("Cases")
	f.gotFilter, f.gotAfter, f.gotSize = fl, after, size
	return f.page, f.err
}

func (f *fakeCases) CaseWithSnapshot(_ context.Context, id uuid.UUID) (moderation.CaseDetail, *moderation.Snapshot, error) {
	f.called("CaseWithSnapshot")
	f.gotID = id
	return f.detail, f.snapshot, f.err
}

func (f *fakeCases) DecideCase(_ context.Context, in moderation.DecideInput) (moderation.CaseDetail, error) {
	f.called("DecideCase")
	f.decided = append(f.decided, in)
	return f.detail, f.err
}

func (f *fakeCases) ReverseHide(_ context.Context, in moderation.ReverseInput) (moderation.CaseDetail, error) {
	f.called("ReverseHide")
	f.reversed = append(f.reversed, in)
	return f.detail, f.err
}

func newAdminEnv(t *testing.T, f *fakeCases) moderationv1alpha1connect.ModerationAdminServiceClient {
	t.Helper()
	path, handler := moderationv1alpha1connect.NewModerationAdminServiceHandler(NewAdmin(f), connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		req = req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 42}))
		handler.ServeHTTP(rw, req)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return moderationv1alpha1connect.NewModerationAdminServiceClient(srv.Client(), srv.URL)
}

func TestListCasesPassesFiltersAndMapsTheCasesAndTheNextToken(t *testing.T) {
	open := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	closed := open.Add(time.Hour)
	id1, id2 := uuid.New(), uuid.New()
	dec, act, cat := moderation.DecisionViolation, moderation.ActionHide, moderationCategory()
	admin, store, measure := int64(9), int64(12), uuid.New()
	f := &fakeCases{page: moderation.CasePage{
		Cases: []moderation.Case{
			{ID: id1, Target: moderation.Target{Kind: moderation.TargetBuyer, BuyerID: 5}, Version: 3, OpenedAt: open, ReportCount: 7},
			{ID: id2, Target: moderation.Target{Kind: moderation.TargetComment, ContentID: uuid.New()}, Version: 2, OpenedAt: open, Decision: &dec, Action: &act, Category: &cat, DecidedByAdmin: &admin, ClosedAt: &closed, ReportCount: 2, MeasureID: &measure, StoreID: &store},
		},
		Next: &moderation.CaseCursor{OpenedAt: open, ID: id2},
	}}
	c := newAdminEnv(t, f)
	res, err := c.ListCases(t.Context(), connect.NewRequest(&adminpb.ListCasesRequest{
		PageSize: 2, State: adminpb.CaseState_CASE_STATE_OPEN, TargetKind: pb.ReportTargetKind_REPORT_TARGET_KIND_COMMENT,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if f.gotFilter != (moderation.CaseFilter{State: moderation.CaseOpen, TargetKind: moderation.TargetComment}) || f.gotSize != 2 || f.gotAfter != nil {
		t.Fatalf("傳給模組的篩選：%+v size=%d after=%v", f.gotFilter, f.gotSize, f.gotAfter)
	}
	got := res.Msg.GetCases()
	if len(got) != 2 {
		t.Fatalf("cases = %d", len(got))
	}
	if got[0].GetState() != adminpb.CaseState_CASE_STATE_OPEN || got[0].GetTarget().GetId() != "5" || got[0].GetEtag() != "3" ||
		got[0].GetDecision() != adminpb.CaseDecision_CASE_DECISION_UNSPECIFIED || got[0].ResolveTime != nil || got[0].GetDecidedBy() != "" ||
		got[0].GetReportCount() != 7 || got[0].MeasureId != nil || got[0].StoreId != nil {
		t.Fatalf("進行中的案件：%+v", got[0])
	}
	if got[1].GetState() != adminpb.CaseState_CASE_STATE_CLOSED || got[1].GetDecision() != adminpb.CaseDecision_CASE_DECISION_VIOLATION ||
		got[1].GetAction() != adminpb.CaseAction_CASE_ACTION_HIDE || got[1].GetCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_SPAM ||
		got[1].GetDecidedBy() != "9" || !got[1].GetResolveTime().AsTime().Equal(closed) ||
		got[1].GetReportCount() != 2 || got[1].GetMeasureId() != measure.String() || got[1].GetStoreId() != "12" {
		t.Fatalf("已結案的案件：%+v", got[1])
	}
	if res.Msg.GetNextPageToken() == "" {
		t.Fatal("有下一頁要給 token")
	}

	if _, err := c.ListCases(t.Context(), connect.NewRequest(&adminpb.ListCasesRequest{
		State: adminpb.CaseState_CASE_STATE_OPEN, TargetKind: pb.ReportTargetKind_REPORT_TARGET_KIND_COMMENT, PageToken: res.Msg.GetNextPageToken(),
	})); err != nil {
		t.Fatal(err)
	}
	if f.gotAfter == nil || f.gotAfter.ID != id2 || !f.gotAfter.OpenedAt.Equal(open) {
		t.Fatalf("游標 = %+v", f.gotAfter)
	}
	_, err = c.ListCases(t.Context(), connect.NewRequest(&adminpb.ListCasesRequest{State: adminpb.CaseState_CASE_STATE_CLOSED, PageToken: res.Msg.GetNextPageToken()}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("換篩選沿用 token：%v", err)
	}
}

func TestListCasesWithoutFiltersAsksForNoFilter(t *testing.T) {
	f := &fakeCases{}
	c := newAdminEnv(t, f)
	if _, err := c.ListCases(t.Context(), connect.NewRequest(&adminpb.ListCasesRequest{})); err != nil {
		t.Fatal(err)
	}
	if f.gotFilter != (moderation.CaseFilter{}) {
		t.Fatalf("UNSPECIFIED 要是不篩：%+v", f.gotFilter)
	}
}

func TestListCasesInvalidFilterErrorPointsAtTheRequestField(t *testing.T) {
	f := &fakeCases{err: apperr.InvalidInputOn([]string{"State"}, "狀態不合法")}
	c := newAdminEnv(t, f)
	_, err := c.ListCases(t.Context(), connect.NewRequest(&adminpb.ListCasesRequest{State: adminpb.CaseState(99)}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("err = %v", err)
	}
	if f.gotFilter.State != "99" {
		t.Fatalf("不認得的列舉值要原樣（十進位字串）交給模組回報：%q", f.gotFilter.State)
	}
}

func TestGetCaseReturnsTheCaseWithItsReports(t *testing.T) {
	caseID, reportID, measure, store := uuid.New(), uuid.New(), uuid.New(), int64(12)
	at := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	f := &fakeCases{detail: moderation.CaseDetail{
		Case: moderation.Case{ID: caseID, Target: moderation.Target{Kind: moderation.TargetPost, ContentID: uuid.New()}, Version: 4, OpenedAt: at, ReportCount: 1, MeasureID: &measure, StoreID: &store},
		Reports: []moderation.Report{{
			ID: reportID, CaseID: caseID, ReporterBuyerID: 11, Category: moderationCategory(), Description: "廣告", CreatedAt: at, Outcome: moderation.OutcomeNone,
		}},
	}}
	c := newAdminEnv(t, f)
	res, err := c.GetCase(t.Context(), connect.NewRequest(&adminpb.GetCaseRequest{Id: caseID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	r := res.Msg.GetReports()
	if f.gotID != caseID || res.Msg.GetCase().GetId() != caseID.String() || len(r) != 1 || r[0].GetReporterBuyerId() != "11" || r[0].GetDescription() != "廣告" ||
		r[0].Outcome != nil || r[0].GetId() != reportID.String() ||
		res.Msg.GetCase().GetReportCount() != 1 || res.Msg.GetCase().GetMeasureId() != measure.String() || res.Msg.GetCase().GetStoreId() != "12" {
		t.Fatalf("response = %+v", res.Msg)
	}
	_, err = c.GetCase(t.Context(), connect.NewRequest(&adminpb.GetCaseRequest{Id: "not-a-uuid"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("壞的 ID：%v", err)
	}
}

func TestGetCaseNotFoundIsNotFound(t *testing.T) {
	c := newAdminEnv(t, &fakeCases{err: apperr.NotFound("案件不存在")})
	_, err := c.GetCase(t.Context(), connect.NewRequest(&adminpb.GetCaseRequest{Id: uuid.New().String()}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
}

func TestDecideCaseSendsTheAdminProcedureAndTheFieldsToTheModule(t *testing.T) {
	caseID := uuid.New()
	f := &fakeCases{detail: moderation.CaseDetail{Case: moderation.Case{ID: caseID, Version: 5, OpenedAt: time.Now()}}}
	c := newAdminEnv(t, f)
	_, err := c.DecideCase(t.Context(), connect.NewRequest(&adminpb.DecideCaseRequest{
		Id: caseID.String(), Etag: "4", Decision: adminpb.CaseDecision_CASE_DECISION_VIOLATION,
		Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM, Action: adminpb.CaseAction_CASE_ACTION_TAKEDOWN,
		Reason: "廣告", Severity: sellerpb.ViolationSeverity_VIOLATION_SEVERITY_HIGH,
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := moderation.DecideInput{
		CaseID: caseID, Version: 4, Actor: adminaudit.Actor{AdminID: 42, Action: moderationv1alpha1connect.ModerationAdminServiceDecideCaseProcedure},
		Decision: moderation.DecisionViolation, Category: moderationCategory(), Action: moderation.ActionTakedown, Text: "廣告", Severity: "HIGH",
	}
	if len(f.decided) != 1 || f.decided[0] != want {
		t.Fatalf("decided = %+v\nwant %+v", f.decided, want)
	}
}

func TestDecideCaseRequiresTheEtagAndAValidID(t *testing.T) {
	f := &fakeCases{}
	c := newAdminEnv(t, f)
	for name, req := range map[string]*adminpb.DecideCaseRequest{
		"沒有 etag": {Id: uuid.New().String(), Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION},
		"壞的 ID":   {Id: "x", Etag: "1", Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION},
	} {
		_, err := c.DecideCase(t.Context(), connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：err = %v", name, err)
		}
	}
	if len(f.decided) != 0 {
		t.Fatalf("格式錯誤不該到模組：%+v", f.decided)
	}
}

func TestDecideCaseVersionConflictIsAborted(t *testing.T) {
	c := newAdminEnv(t, &fakeCases{err: apperr.VersionConflict("案件已被結案")})
	_, err := c.DecideCase(t.Context(), connect.NewRequest(&adminpb.DecideCaseRequest{
		Id: uuid.New().String(), Etag: "1", Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION,
	}))
	if connect.CodeOf(err) != connect.CodeAborted {
		t.Fatalf("err = %v", err)
	}
}

func TestDecideCaseModuleFieldErrorsPointAtTheRequestFields(t *testing.T) {
	c := newAdminEnv(t, &fakeCases{err: apperr.InvalidInputOn([]string{"Text"}, "理由必填")})
	_, err := c.DecideCase(t.Context(), connect.NewRequest(&adminpb.DecideCaseRequest{
		Id: uuid.New().String(), Etag: "1", Decision: adminpb.CaseDecision_CASE_DECISION_VIOLATION,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("err = %v", err)
	}
}

func TestAdminHandlerWithoutAnAdminPrincipalDoesNotReachTheModule(t *testing.T) {
	f := &fakeCases{}
	h := NewAdmin(f)
	_, err := h.DecideCase(t.Context(), connect.NewRequest(&adminpb.DecideCaseRequest{Id: uuid.New().String(), Etag: "1"}))
	if err == nil || len(f.decided) != 0 {
		t.Fatalf("沒有管理員要失敗且不到模組：err=%v decided=%d", err, len(f.decided))
	}
}

func moderationCategory() policy.Category { return policy.Spam }

func newGuardedEnv(t *testing.T, f *fakeCases) moderationv1alpha1connect.ModerationAdminServiceClient {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	path, handler := moderationv1alpha1connect.NewModerationAdminServiceHandler(NewAdmin(f), connect.WithInterceptors(rpc.NewInterceptor(log), adminauth.NewInterceptor(log)))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		p := identity.Principal{Kind: identity.PrincipalAdmin, ID: 42}
		if req.Header.Get("X-Test-Role") != "" {
			p.AdminRoles = []adminperm.Role{adminperm.Role(req.Header.Get("X-Test-Role"))}
		}
		handler.ServeHTTP(rw, req.WithContext(rpc.WithPrincipal(req.Context(), p)))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return moderationv1alpha1connect.NewModerationAdminServiceClient(srv.Client(), srv.URL)
}

// 沒有權限的管理員每支方法都回 PERMISSION_DENIED、不到模組；OWNER 通過。
func TestEveryMethodRequiresItsPermission(t *testing.T) {
	f := &fakeCases{}
	c := newGuardedEnv(t, f)
	id := uuid.New().String()
	calls := map[string]func(role string) error{
		"ListCases": func(role string) error {
			_, err := c.ListCases(t.Context(), withRole(&adminpb.ListCasesRequest{}, role))
			return err
		},
		"GetCase": func(role string) error {
			_, err := c.GetCase(t.Context(), withRole(&adminpb.GetCaseRequest{Id: id}, role))
			return err
		},
		"DecideCase": func(role string) error {
			_, err := c.DecideCase(t.Context(), withRole(&adminpb.DecideCaseRequest{Id: id, Etag: "1", Decision: adminpb.CaseDecision_CASE_DECISION_NO_VIOLATION}, role))
			return err
		},
		"ReverseHide": func(role string) error {
			_, err := c.ReverseHide(t.Context(), withRole(&adminpb.ReverseHideRequest{Id: id, Reason: "誤判"}, role))
			return err
		},
	}
	methods := adminpb.File_hawkr_admin_moderation_v1alpha1_moderation_proto.Services().ByName("ModerationAdminService").Methods()
	if methods.Len() != len(calls) {
		t.Fatalf("ModerationAdminService 有 %d 支方法，測試涵蓋 %d 支", methods.Len(), len(calls))
	}
	for name, call := range calls {
		if err := call(""); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s 沒有角色：err = %v，預期 PERMISSION_DENIED", name, err)
		}
	}
	if len(f.calls) != 0 {
		t.Fatalf("被拒絕的請求不該到模組（每個方法 0 次呼叫）：%v", f.calls)
	}
	for name, call := range calls {
		if err := call(string(adminperm.RoleOwner)); err != nil {
			t.Errorf("%s OWNER：err = %v", name, err)
		}
	}
	if f.calls["Cases"] != 1 || f.calls["CaseWithSnapshot"] != 1 || f.calls["DecideCase"] != 1 || f.calls["ReverseHide"] != 1 {
		t.Fatalf("OWNER 通過後每個方法恰好到模組一次：%v", f.calls)
	}
}

func withRole[T any](msg *T, role string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("X-Test-Role", role)
	return req
}

func TestReverseHidePassesTheAdminProcedureAndReasonAndMapsTheReversal(t *testing.T) {
	caseID := uuid.NewV7()
	at := time.Date(2026, 11, 21, 1, 2, 3, 0, time.UTC)
	f := &fakeCases{detail: moderation.CaseDetail{Case: moderation.Case{ID: caseID, Version: 5, OpenedAt: at, Reversal: &moderation.Reversal{At: at, AdminID: 42, Reason: "誤判"}}}}
	c := newAdminEnv(t, f)
	resp, err := c.ReverseHide(t.Context(), connect.NewRequest(&adminpb.ReverseHideRequest{Id: caseID.String(), Reason: "誤判"}))
	if err != nil {
		t.Fatal(err)
	}
	want := moderation.ReverseInput{CaseID: caseID, Actor: adminaudit.Actor{AdminID: 42, Action: moderationv1alpha1connect.ModerationAdminServiceReverseHideProcedure}, Reason: "誤判"}
	if len(f.reversed) != 1 || f.reversed[0] != want {
		t.Fatalf("reversed = %+v\nwant %+v", f.reversed, want)
	}
	if r := resp.Msg.GetCase().GetReversal(); r == nil || r.GetReversedBy() != "42" || r.GetReason() != "誤判" || !r.GetReverseTime().AsTime().Equal(at) {
		t.Fatalf("reversal = %+v", r)
	}
}

func TestReverseHideRejectsABadID(t *testing.T) {
	f := &fakeCases{}
	c := newAdminEnv(t, f)
	_, err := c.ReverseHide(t.Context(), connect.NewRequest(&adminpb.ReverseHideRequest{Id: "x", Reason: "r"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || len(f.reversed) != 0 {
		t.Fatalf("err = %v, reversed = %d", err, len(f.reversed))
	}
}

func TestGetCaseMapsEachSnapshotKindToTheOneof(t *testing.T) {
	at := time.Date(2026, 11, 21, 1, 2, 3, 0, time.UTC)
	for name, tc := range map[string]struct {
		snap  *moderation.Snapshot
		check func(*adminpb.CaseSnapshot) bool
	}{
		"社群": {&moderation.Snapshot{Kind: moderation.TargetComment, Social: &moderation.SocialSnapshot{AuthorKind: "BUYER", AuthorID: 7, Body: "本文", MediaRefs: []string{"a"}, AuthoredAt: at}},
			func(s *adminpb.CaseSnapshot) bool {
				x := s.GetSocial()
				return x.GetAuthorId() == "7" && x.GetBody() == "本文" && len(x.GetAssetIds()) == 1
			}},
		"買家": {&moderation.Snapshot{Kind: moderation.TargetBuyer, Buyer: &moderation.BuyerSnapshot{DisplayName: "小明", AvatarAssetID: "k"}},
			func(s *adminpb.CaseSnapshot) bool {
				return s.GetBuyer().GetDisplayName() == "小明" && s.GetBuyer().GetAvatarAssetId() == "k"
			}},
		"評價": {&moderation.Snapshot{Kind: moderation.TargetReview, Review: &moderation.ReviewSnapshot{AuthorID: 9, SubjectStoreID: 4, ProductID: uuid.UUID{15: 5}, Rating: 2, Body: "差", ReplyBody: "抱歉"}},
			func(s *adminpb.CaseSnapshot) bool {
				x := s.GetReview()
				return x.GetAuthorId() == "9" && x.GetStoreId() == "4" && x.GetProductId() == "00000000-0000-0000-0000-000000000005" && x.GetRating() == 2 && x.GetReplyBody() == "抱歉"
			}},
		"聊天": {&moderation.Snapshot{Kind: moderation.TargetChatMessage, Chat: &moderation.ChatSnapshot{AuthorKind: "STORE", AuthorID: 4, ConversationID: "c", Seq: 6,
			Context: []moderation.ChatContextMessage{{Seq: 5, SenderKind: "BUYER", SenderID: 9, Type: "TEXT", Hidden: true, SentAt: at}}}},
			func(s *adminpb.CaseSnapshot) bool {
				x := s.GetChat()
				return x.GetSeq() == 6 && len(x.GetContext()) == 1 && x.GetContext()[0].GetHidden() && x.GetContext()[0].GetSenderId() == "9"
			}},
	} {
		f := &fakeCases{detail: moderation.CaseDetail{Case: moderation.Case{ID: uuid.NewV7(), Version: 1, OpenedAt: at}}, snapshot: tc.snap}
		c := newAdminEnv(t, f)
		resp, err := c.GetCase(t.Context(), connect.NewRequest(&adminpb.GetCaseRequest{Id: uuid.NewV7().String()}))
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if !tc.check(resp.Msg.GetSnapshot()) {
			t.Errorf("%s：%+v", name, resp.Msg.GetSnapshot())
		}
	}
	f := &fakeCases{detail: moderation.CaseDetail{Case: moderation.Case{ID: uuid.NewV7(), Version: 1, OpenedAt: at}}}
	resp, err := newAdminEnv(t, f).GetCase(t.Context(), connect.NewRequest(&adminpb.GetCaseRequest{Id: uuid.NewV7().String()}))
	if err != nil || resp.Msg.Snapshot != nil {
		t.Fatalf("沒有快照不設：%v %+v", err, resp.Msg.Snapshot)
	}
}

func TestReportsPBCarriesTheReporterKind(t *testing.T) {
	got := reportsPB([]moderation.Report{
		{ID: uuid.NewV7(), ReporterBuyerID: 5},
		{ID: uuid.NewV7(), ReporterStoreID: 7},
	})
	if got[0].GetReporterBuyerId() != "5" || got[0].GetReporterStoreId() != "" {
		t.Errorf("買家檢舉人：%+v", got[0])
	}
	if got[1].GetReporterStoreId() != "7" || got[1].GetReporterBuyerId() != "" {
		t.Errorf("店家檢舉人：%+v", got[1])
	}
}
