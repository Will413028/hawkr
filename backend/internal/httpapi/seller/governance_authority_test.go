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
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// authorityModule 記下來函指令收到的輸入，回傳設定好的結果。
type authorityModule struct {
	GovernanceModule
	create   model.CreateAuthorityOrderInput
	list     model.AuthorityOrderListInput
	complete model.CompleteAuthorityOrderInput
	extend   model.ExtendAuthorityOrderHoldInput
	notice   model.RecordAuthorityTargetNoticeInput
	revoke   model.RevokeAuthorityOrderInput
	order    model.AdminAuthorityOrder
	page     model.AdminAuthorityOrderPage
	err      error
	calls    int
}

func (m *authorityModule) CreateAuthorityOrder(_ context.Context, in model.CreateAuthorityOrderInput) (model.CreateAuthorityOrderResult, error) {
	m.create, m.calls = in, m.calls+1
	return model.CreateAuthorityOrderResult{Order: m.order}, m.err
}

func (m *authorityModule) AuthorityOrders(_ context.Context, in model.AuthorityOrderListInput) (model.AdminAuthorityOrderPage, error) {
	m.list = in
	return m.page, m.err
}

func (m *authorityModule) AuthorityOrder(context.Context, uuid.UUID) (model.AdminAuthorityOrder, error) {
	return m.order, m.err
}

func (m *authorityModule) CompleteAuthorityOrder(_ context.Context, in model.CompleteAuthorityOrderInput) (model.AdminAuthorityOrder, error) {
	m.complete, m.calls = in, m.calls+1
	return m.order, m.err
}

func (m *authorityModule) ExtendAuthorityOrderHold(_ context.Context, in model.ExtendAuthorityOrderHoldInput) (model.AdminAuthorityOrder, error) {
	m.extend, m.calls = in, m.calls+1
	return m.order, m.err
}

func (m *authorityModule) RecordAuthorityTargetNotice(_ context.Context, in model.RecordAuthorityTargetNoticeInput) (model.AdminAuthorityOrder, error) {
	m.notice, m.calls = in, m.calls+1
	return m.order, m.err
}

func (m *authorityModule) RevokeAuthorityOrder(_ context.Context, in model.RevokeAuthorityOrderInput) (model.RevokeAuthorityOrderResult, error) {
	m.revoke, m.calls = in, m.calls+1
	return model.RevokeAuthorityOrderResult{Order: m.order, AlreadyRevoked: true}, m.err
}

func authorityFixture(completed bool) model.AdminAuthorityOrder {
	store := int64(12)
	name := "範例小農"
	recheck := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	o := model.AuthorityOrder{
		ID: uuid.NewV7(), Agency: "衛生局", DocumentNo: "字第 1 號", ReceivedAt: governanceNow, Kind: model.AuthorityDataRequest, Summary: "調取", DueAt: governanceNow.Add(72 * time.Hour),
		HoldUntil: governanceNow.Add(model.AuthorityHold), RegisteredAt: governanceNow, RegisteredByAdminID: 7, Version: 3,
		Targets: []model.AuthorityOrderTarget{
			{AccountID: 5, StoreID: &store, ProductIDs: []string{"p1"}, Stores: []model.AuthorityTargetStore{{StoreID: 12, StoreName: &name, Coverage: model.CoverageHeld}}},
			{Position: 1, AccountID: 6, NoSellerData: true},
		},
		HoldExtensions: []model.AuthorityHoldExtension{{From: governanceNow, To: governanceNow.Add(time.Hour), Cause: model.HoldExtensionAdmin, Reason: "訴訟", AdminID: 9, CreatedAt: governanceNow}},
	}
	// 處理期限由模組的讀取路徑填入（seller.review_authority_order）：處理中才有，已完成的沒有。
	if !completed {
		o.Deadline = &model.Deadline{Start: o.ReceivedAt, Due: o.DueAt}
	}
	if completed {
		o.Completed = &model.AuthorityCompletion{At: governanceNow.Add(time.Hour), AdminID: 8, Summary: "已回覆"}
		o.Targets[0].Notice = &model.AuthorityNotice{Kind: model.AuthorityNoticeWithheld, RecheckOn: &recheck, RecordedAt: governanceNow, RecordedByAdminID: 8}
	}
	return model.AdminAuthorityOrder{AuthorityOrder: o, Availability: o.Availability(), HoldActive: true}
}

func TestAuthorityEnumsMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"AuthorityOrderKind":              rpc.EnumMismatches[adminpb.AuthorityOrderKind](model.AuthorityOrderKindValues()),
		"AuthorityOrderState":             rpc.EnumMismatches[adminpb.AuthorityOrderState](model.AuthorityOrderStateValues()),
		"AuthorityTargetNoticeKind":       rpc.EnumMismatches[adminpb.AuthorityTargetNoticeKind](model.AuthorityNoticeKindValues()),
		"AuthorityHoldExtensionCause":     rpc.EnumMismatches[adminpb.AuthorityHoldExtensionCause](model.HoldExtensionCauseValues()),
		"AuthorityOrderUnavailableReason": rpc.EnumMismatches[adminpb.AuthorityOrderUnavailableReason](model.AuthorityOrderBlockValues()),
		"AuthorityStoreCoverage": rpc.EnumMismatches[adminpb.AuthorityStoreCoverage]([]model.StoreCoverage{
			model.CoverageHeld, model.CoveragePurgedBeforeRegistration, model.CoverageHoldEnded, model.CoveragePurgedAfterHoldEnded, model.CoverageHoldReleasedByRevocation,
		}),
	} {
		if len(got) != 0 {
			t.Errorf("%s：%v", name, got)
		}
	}
}

func TestAuthorityFieldPathsExistInTheRequests(t *testing.T) {
	for name, c := range map[string]struct {
		paths rpc.FieldPaths
		req   interface{ ProtoReflect() protoreflect.Message }
	}{
		"create":   {createAuthorityOrderPaths, &adminpb.CreateAuthorityOrderRequest{}},
		"complete": {completeAuthorityOrderPaths, &adminpb.CompleteAuthorityOrderRequest{}},
		"extend":   {extendAuthorityHoldPaths, &adminpb.ExtendAuthorityOrderHoldRequest{}},
		"notice":   {recordTargetNoticePaths, &adminpb.RecordAuthorityTargetNoticeRequest{}},
		"revoke":   {revokeAuthorityOrderPaths, &adminpb.RevokeAuthorityOrderRequest{}},
		"measure":  {createMeasurePaths(violationPaths("Violation", "warning.violation")), &adminpb.CreateMeasureRequest{}},
		"list":     {listMeasuresPaths, &adminpb.ListMeasuresRequest{}},
	} {
		if missing := rpc.CheckFieldPaths(c.req.ProtoReflect().Descriptor(), c.paths); len(missing) > 0 {
			t.Errorf("%s：不存在的路徑 %v", name, missing)
		}
	}
}

// 來函只輸出契約的欄位：期限只在處理中有；can_* 取自 Availability（不輸出原因）；對象逐家列出涵蓋狀態與店名；沒有店家的帳號標 no_seller_data；
// 只有指名單一店家時 store_name 有值；etag 是版本；延長歷史與告知。
func TestAuthorityOrderCarriesDueCoverageAndNotice(t *testing.T) {
	open := toAuthorityOrder(authorityFixture(false))
	if open.GetState() != adminpb.AuthorityOrderState_AUTHORITY_ORDER_STATE_OPEN || !open.GetDeadline().GetStartTime().AsTime().Equal(governanceNow) ||
		!open.GetDeadline().GetDueTime().AsTime().Equal(governanceNow.Add(72*time.Hour)) || !open.GetCanComplete() || !open.GetCanExtendHold() ||
		open.GetEtag() != "3" || !open.GetHoldActive() || open.GetHoldUntil() == nil || open.GetCompleteTime() != nil {
		t.Fatalf("open=%v", open)
	}
	first, second := open.GetTargets()[0], open.GetTargets()[1]
	if first.GetAccountId() != "5" || first.GetStoreId() != "12" || first.GetStoreName() != "範例小農" || len(first.GetCoveredStores()) != 1 ||
		first.GetCoveredStores()[0].GetCoverage() != adminpb.AuthorityStoreCoverage_AUTHORITY_STORE_COVERAGE_HELD || first.GetNoSellerData() || first.GetNotice() != nil {
		t.Fatalf("first=%v", first)
	}
	if !second.GetNoSellerData() || second.GetStoreId() != "" || len(second.GetCoveredStores()) != 0 {
		t.Fatalf("second=%v", second)
	}
	if len(open.GetHoldExtensions()) != 1 || open.GetHoldExtensions()[0].GetReason() != "訴訟" || open.GetHoldExtensions()[0].GetCause() != adminpb.AuthorityHoldExtensionCause_AUTHORITY_HOLD_EXTENSION_CAUSE_ADMIN {
		t.Fatalf("extensions=%v", open.GetHoldExtensions())
	}
	done := toAuthorityOrder(authorityFixture(true))
	n := done.GetTargets()[0].GetNotice()
	if done.GetDeadline() != nil || done.GetCanComplete() || !done.GetCanExtendHold() || done.GetResolutionSummary() != "已回覆" || done.GetCompletedByAdminId() != "8" ||
		n.GetKind() != adminpb.AuthorityTargetNoticeKind_AUTHORITY_TARGET_NOTICE_KIND_WITHHELD || n.GetRecheckDate().GetDay() != 20 || n.GetRecordedByAdminId() != "8" {
		t.Fatalf("done=%v", done)
	}
}

func TestCreateAuthorityOrderParsesTargetsAndRequestID(t *testing.T) {
	m := &authorityModule{order: authorityFixture(false)}
	h := NewGovernance(m, rpc.Images{})
	req := uuid.NewV4()
	due := governanceNow.Add(5 * 24 * time.Hour)
	_, err := h.CreateAuthorityOrder(adminCtx(42), connect.NewRequest(&adminpb.CreateAuthorityOrderRequest{
		Agency: "衛生局", DocumentNo: "字第 1 號", ReceiveTime: timestamppb.New(governanceNow), Kind: adminpb.AuthorityOrderKind_AUTHORITY_ORDER_KIND_RESTRICT_PRODUCT, Summary: "下架",
		Targets: []*adminpb.AuthorityOrderTarget{{AccountId: "5", StoreId: "12", ProductIds: []string{"p1"}}, {AccountId: "6"}}, DueTime: timestamppb.New(due), RequestId: req.String(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	in := m.create
	if in.AdminID != 42 || in.Kind != model.AuthorityRestrictProduct || in.Agency != "衛生局" || !in.ReceivedAt.Equal(governanceNow) || in.DueAt == nil || !in.DueAt.Equal(due) || in.RequestID == nil || *in.RequestID != req ||
		len(in.Targets) != 2 || in.Targets[0].AccountID != 5 || in.Targets[0].StoreID == nil || *in.Targets[0].StoreID != 12 || in.Targets[1].StoreID != nil || in.Targets[1].AccountID != 6 {
		t.Fatalf("in=%+v", in)
	}
	calls := m.calls
	for name, r := range map[string]*adminpb.CreateAuthorityOrderRequest{
		"帳號不是數字":              {Targets: []*adminpb.AuthorityOrderTarget{{AccountId: "x"}}},
		"店家不是數字":              {Targets: []*adminpb.AuthorityOrderTarget{{AccountId: "1", StoreId: "x"}}},
		"request_id 不是 UUID":  {RequestId: "x"},
		"request_id 是 UUIDv7": {RequestId: uuid.NewV7().String()},
	} {
		_, err := h.CreateAuthorityOrder(adminCtx(42), connect.NewRequest(r))
		t.Run(name, func(t *testing.T) { wantInvalidArgument(t, err) })
	}
	if m.calls != calls {
		t.Fatal("格式不對的請求不得呼叫模組")
	}
	if _, err := h.CreateAuthorityOrder(context.Background(), connect.NewRequest(&adminpb.CreateAuthorityOrderRequest{})); err == nil || m.calls != calls {
		t.Fatal("不是管理員不得呼叫模組")
	}
}

// 模組的欄位錯誤（含索引欄位）指到請求的路徑。
func TestAuthorityFieldErrorsPointToRequestPaths(t *testing.T) {
	m := &authorityModule{order: authorityFixture(false)}
	h := NewGovernance(m, rpc.Images{})
	id := uuid.NewV7().String()
	type call func(ctx context.Context) error
	cases := []struct {
		procedure string
		call      call
		fields    map[string]string
	}{
		{sellerv1alpha1connect.SellerGovernanceAdminServiceCreateAuthorityOrderProcedure, func(ctx context.Context) error {
			_, err := h.CreateAuthorityOrder(ctx, connect.NewRequest(&adminpb.CreateAuthorityOrderRequest{}))
			return err
		}, map[string]string{
			"Agency": "agency", "DocumentNo": "document_no", "ReceivedAt": "receive_time", "Kind": "kind", "Summary": "summary", "DueAt": "due_time", "RequestID": "request_id",
			"Targets": "targets", "Targets[0].AccountID": "targets[0].account_id", "Targets[1].StoreID": "targets[1].store_id", "Targets[2].ProductIDs": "targets[2].product_ids",
		}},
		{sellerv1alpha1connect.SellerGovernanceAdminServiceCompleteAuthorityOrderProcedure, func(ctx context.Context) error {
			_, err := h.CompleteAuthorityOrder(ctx, connect.NewRequest(&adminpb.CompleteAuthorityOrderRequest{Id: id, Etag: "1"}))
			return err
		}, map[string]string{
			"ResolutionSummary": "resolution_summary", "ProvideTime": "provide_time", "TargetNotices": "target_notices", "TargetNotices[1]": "target_notices[1]",
			"TargetNotices[1].Notice.Kind": "target_notices[1].notice.kind", "TargetNotices[0].Notice.NotifiedAt": "target_notices[0].notice.notify_time",
			"TargetNotices[0].Notice.RecheckOn": "target_notices[0].notice.recheck_date", "TargetNotices[0].Notice.Note": "target_notices[0].notice.note",
		}},
		{sellerv1alpha1connect.SellerGovernanceAdminServiceExtendAuthorityOrderHoldProcedure, func(ctx context.Context) error {
			_, err := h.ExtendAuthorityOrderHold(ctx, connect.NewRequest(&adminpb.ExtendAuthorityOrderHoldRequest{Id: id, Etag: "1", HoldUntil: timestamppb.New(governanceNow)}))
			return err
		}, map[string]string{"Reason": "reason", "HoldUntil": "hold_until"}},
		{sellerv1alpha1connect.SellerGovernanceAdminServiceRecordAuthorityTargetNoticeProcedure, func(ctx context.Context) error {
			_, err := h.RecordAuthorityTargetNotice(ctx, connect.NewRequest(&adminpb.RecordAuthorityTargetNoticeRequest{Id: id, Etag: "1"}))
			return err
		}, map[string]string{
			"TargetPosition": "target_position", "Notice.Kind": "notice.kind", "Notice.RecheckOn": "notice.recheck_date", "Notice.NotifiedAt": "notice.notify_time", "Notice.Note": "notice.note",
		}},
	}
	for _, c := range cases {
		for module, path := range c.fields {
			m.err = apperr.InvalidInputOn([]string{module}, "x")
			var logs strings.Builder
			wrapped := rpc.NewInterceptor(slog.New(slog.NewTextHandler(&logs, nil))).WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
				return nil, c.call(ctx)
			})
			_, err := wrapped(adminCtx(42), fakeRequest{procedure: c.procedure})
			if got := reportErrFields(t, err); !slices.Equal(got, []string{path}) || strings.Contains(logs.String(), "欄位沒有附給 client") {
				t.Errorf("%s %s：BadRequest 的欄位 %v，要 [%s]；log=%s", c.procedure, module, got, path, logs.String())
			}
		}
	}
}

// 完成、延長與記下告知都要 etag；告知的日期與時間轉成模組的形狀；暫不告知的日期不是有效日期時交給模組（零值）。
func TestAuthorityCommandsRequireEtagAndConvertNotices(t *testing.T) {
	m := &authorityModule{order: authorityFixture(true)}
	h := NewGovernance(m, rpc.Images{})
	id := uuid.NewV7()
	at := governanceNow.Add(-time.Hour)
	provide := governanceNow.Add(-2 * time.Hour)
	_, err := h.CompleteAuthorityOrder(adminCtx(42), connect.NewRequest(&adminpb.CompleteAuthorityOrderRequest{
		Id: id.String(), ResolutionSummary: "已回覆", Etag: "3", ProvideTime: timestamppb.New(provide),
		TargetNotices: []*adminpb.TargetNoticeInput{
			{TargetPosition: 0, Notice: &adminpb.AuthorityTargetNotice{Kind: adminpb.AuthorityTargetNoticeKind_AUTHORITY_TARGET_NOTICE_KIND_NOTIFIED, NotifyTime: timestamppb.New(at), Note: "電話"}},
			{TargetPosition: 1, Notice: &adminpb.AuthorityTargetNotice{Kind: adminpb.AuthorityTargetNoticeKind_AUTHORITY_TARGET_NOTICE_KIND_WITHHELD, RecheckDate: &date.Date{Year: 2026, Month: 10, Day: 30}}},
			{TargetPosition: 2, Notice: &adminpb.AuthorityTargetNotice{Kind: adminpb.AuthorityTargetNoticeKind_AUTHORITY_TARGET_NOTICE_KIND_WITHHELD, RecheckDate: &date.Date{Year: 2026, Month: 2, Day: 30}}},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	c := m.complete
	if c.ID != id || c.AdminID != 42 || c.ExpectedVersion != 3 || c.ResolutionSummary != "已回覆" || c.ProvideTime == nil || !c.ProvideTime.Equal(provide) || len(c.TargetNotices) != 3 ||
		c.TargetNotices[0].Notice.Kind != model.AuthorityNoticeNotified || !c.TargetNotices[0].Notice.NotifiedAt.Equal(at) || c.TargetNotices[0].Notice.Note != "電話" ||
		c.TargetNotices[1].Notice.RecheckOn == nil || !c.TargetNotices[1].Notice.RecheckOn.Equal(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)) ||
		c.TargetNotices[2].Position != 2 || c.TargetNotices[2].Notice.RecheckOn == nil || !c.TargetNotices[2].Notice.RecheckOn.IsZero() {
		t.Fatalf("complete=%+v", c)
	}
	until := governanceNow.Add(400 * 24 * time.Hour)
	if _, err := h.ExtendAuthorityOrderHold(adminCtx(43), connect.NewRequest(&adminpb.ExtendAuthorityOrderHoldRequest{Id: id.String(), Etag: "3", HoldUntil: timestamppb.New(until), Reason: "訴訟"})); err != nil {
		t.Fatal(err)
	}
	if e := m.extend; e.AdminID != 43 || e.ExpectedVersion != 3 || !e.HoldUntil.Equal(until) || e.Reason != "訴訟" {
		t.Fatalf("extend=%+v", e)
	}
	if _, err := h.RecordAuthorityTargetNotice(adminCtx(44), connect.NewRequest(&adminpb.RecordAuthorityTargetNoticeRequest{
		Id: id.String(), Etag: "3", TargetPosition: 1, Notice: &adminpb.AuthorityTargetNotice{Kind: adminpb.AuthorityTargetNoticeKind_AUTHORITY_TARGET_NOTICE_KIND_NOTIFIED, NotifyTime: timestamppb.New(at)},
	})); err != nil {
		t.Fatal(err)
	}
	if n := m.notice; n.AdminID != 44 || n.Position != 1 || n.ExpectedVersion != 3 || n.Notice.Kind != model.AuthorityNoticeNotified {
		t.Fatalf("notice=%+v", n)
	}
	calls := m.calls
	for name, call := range map[string]func() error{
		"完成沒有 etag": func() error {
			_, err := h.CompleteAuthorityOrder(adminCtx(42), connect.NewRequest(&adminpb.CompleteAuthorityOrderRequest{Id: id.String()}))
			return err
		},
		"延長沒有 etag": func() error {
			_, err := h.ExtendAuthorityOrderHold(adminCtx(42), connect.NewRequest(&adminpb.ExtendAuthorityOrderHoldRequest{Id: id.String(), HoldUntil: timestamppb.New(until)}))
			return err
		},
		"延長沒有 hold_until": func() error {
			_, err := h.ExtendAuthorityOrderHold(adminCtx(42), connect.NewRequest(&adminpb.ExtendAuthorityOrderHoldRequest{Id: id.String(), Etag: "1"}))
			return err
		},
		"記下告知沒有 etag": func() error {
			_, err := h.RecordAuthorityTargetNotice(adminCtx(42), connect.NewRequest(&adminpb.RecordAuthorityTargetNoticeRequest{Id: id.String()}))
			return err
		},
		"id 不是 UUID": func() error {
			_, err := h.GetAuthorityOrder(adminCtx(42), connect.NewRequest(&adminpb.GetAuthorityOrderRequest{Id: "x"}))
			return err
		},
	} {
		t.Run(name, func(t *testing.T) { wantInvalidArgument(t, call()) })
	}
	if m.calls != calls {
		t.Fatal("格式不對的請求不得呼叫模組")
	}
}

// 來函清單的 page token 綁篩選（狀態與帳號）：換了篩選從第一頁開始。
func TestListAuthorityOrdersBindsThePageTokenToTheFilter(t *testing.T) {
	cursor := &model.AuthorityCursor{At: governanceNow, ID: uuid.NewV7()}
	m := &authorityModule{page: model.AdminAuthorityOrderPage{Items: []model.AdminAuthorityOrder{authorityFixture(false)}, Next: cursor}}
	h := NewGovernance(m, rpc.Images{})
	req := &adminpb.ListAuthorityOrdersRequest{PageSize: 2, State: adminpb.AuthorityOrderState_AUTHORITY_ORDER_STATE_OPEN, AccountId: "5"}
	got, err := h.ListAuthorityOrders(adminCtx(1), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if m.list.PageSize != 2 || m.list.After != nil || m.list.AccountID != 5 || m.list.State == nil || *m.list.State != model.AuthorityStateOpen || len(got.Msg.GetOrders()) != 1 || got.Msg.GetNextPageToken() == "" {
		t.Fatalf("list=%+v", m.list)
	}
	req.PageToken = got.Msg.GetNextPageToken()
	if _, err := h.ListAuthorityOrders(adminCtx(1), connect.NewRequest(req)); err != nil || m.list.After == nil || *m.list.After != *cursor {
		t.Fatalf("after=%+v err=%v", m.list.After, err)
	}
	for name, mutate := range map[string]func(*adminpb.ListAuthorityOrdersRequest){
		"換狀態": func(r *adminpb.ListAuthorityOrdersRequest) {
			r.State = adminpb.AuthorityOrderState_AUTHORITY_ORDER_STATE_COMPLETED
		},
		"換帳號": func(r *adminpb.ListAuthorityOrdersRequest) { r.AccountId = "6" },
	} {
		changed := proto.Clone(req).(*adminpb.ListAuthorityOrdersRequest)
		mutate(changed)
		_, err := h.ListAuthorityOrders(adminCtx(1), connect.NewRequest(changed))
		t.Run(name, func(t *testing.T) { wantInvalidArgument(t, err) })
	}
}

// 建立措施帶來源來函：解析成 UUID 放進輸入，認定引用的來函同；不是 UUID 回 invalid_argument。
func TestCreateMeasureCarriesTheSourceAuthorityOrders(t *testing.T) {
	m := &measureCapture{}
	h := NewGovernance(m, rpc.Images{})
	order, cited := uuid.NewV7(), uuid.NewV7()
	_, _ = h.CreateMeasure(adminCtx(42), connect.NewRequest(&adminpb.CreateMeasureRequest{
		StoreId: "12", Statement: "x", Note: "n", SourceAuthorityOrderId: order.String(),
		Measure: &adminpb.CreateMeasureRequest_Warning{Warning: &adminpb.WarningInput{Violation: &adminpb.ViolationBasis{
			Finding: &adminpb.NewFinding{SourceAuthorityOrderIds: []string{cited.String()}},
		}}},
	}))
	if m.in.SourceAuthorityOrderID == nil || *m.in.SourceAuthorityOrderID != order || m.in.Violation == nil || len(m.in.Violation.Finding.SourceAuthorityOrderIDs) != 1 || m.in.Violation.Finding.SourceAuthorityOrderIDs[0] != cited {
		t.Fatalf("in=%+v", m.in)
	}
	for name, r := range map[string]*adminpb.CreateMeasureRequest{
		"來函不是 UUID": {StoreId: "12", SourceAuthorityOrderId: "x"},
		"引用的來函不是 UUID": {StoreId: "12", Measure: &adminpb.CreateMeasureRequest_Warning{Warning: &adminpb.WarningInput{Violation: &adminpb.ViolationBasis{
			Finding: &adminpb.NewFinding{SourceAuthorityOrderIds: []string{"x"}}}}}},
	} {
		_, err := h.CreateMeasure(adminCtx(42), connect.NewRequest(r))
		t.Run(name, func(t *testing.T) { wantInvalidArgument(t, err) })
	}
	list := &adminpb.ListMeasuresRequest{SourceAuthorityOrderId: order.String()}
	if _, err := h.ListMeasures(adminCtx(42), connect.NewRequest(list)); err != nil || m.list.SourceAuthorityOrderID == nil || *m.list.SourceAuthorityOrderID != order {
		t.Fatalf("list=%+v err=%v", m.list, err)
	}
	_, err := h.ListMeasures(adminCtx(42), connect.NewRequest(&adminpb.ListMeasuresRequest{SourceAuthorityOrderId: "x"}))
	wantInvalidArgument(t, err)
}

type measureCapture struct {
	GovernanceModule
	in   model.CreateMeasureInput
	list model.AdminMeasureListInput
}

func (m *measureCapture) CreateMeasure(_ context.Context, in model.CreateMeasureInput) (model.CreateMeasureResult, error) {
	m.in = in
	return model.CreateMeasureResult{}, apperr.InvalidInput("stop")
}

func (m *measureCapture) AdminMeasures(_ context.Context, in model.AdminMeasureListInput) (model.MeasurePage, error) {
	m.list = in
	return model.MeasurePage{}, nil
}

// 清單的 page token 也綁來源來函：換一份來函從第一頁開始。
func TestListMeasuresPageTokenIsBoundToTheSourceAuthorityOrder(t *testing.T) {
	m := &measureCapture{}
	h := NewGovernance(m, rpc.Images{})
	order := uuid.NewV7()
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListMeasuresProcedure, "", "", "MEASURE_STATE_UNSPECIFIED",
		"MEASURE_BASIS_UNSPECIFIED", "MEASURE_REQUEST_STATE_UNSPECIFIED", "MEASURE_REQUEST_KIND_UNSPECIFIED", "false", "", "", order.String())
	token := rpc.TokenFor(scope, &model.MeasureCursor{SortAt: governanceNow, ID: uuid.NewV7()})
	if _, err := h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token, SourceAuthorityOrderId: order.String()})); err != nil || m.list.After == nil {
		t.Fatalf("同一組篩選可以沿用：%v", err)
	}
	_, err := h.ListMeasures(adminCtx(1), connect.NewRequest(&adminpb.ListMeasuresRequest{PageToken: token, SourceAuthorityOrderId: uuid.NewV7().String()}))
	wantInvalidArgument(t, err)
}

// 撤銷登記與可用性的輸出：can_* 與 *_unavailabilities 取自同一個 Availability，句子與指令被擋下時的錯誤同一個函式；撤銷的時間、管理員與原因；
// 每家店的建立措施入口；指令把 id、etag、原因交給模組，already_revoked 照回。
func TestAuthorityOrderCarriesRevocationAndServerDerivedAvailability(t *testing.T) {
	o := authorityFixture(true)
	o.Targets[0].Stores[0].CanCreateMeasure, o.Targets[0].Stores[0].CanCreateTermination = false, true
	got := toAuthorityOrder(o)
	if got.GetCanRevoke() || len(got.GetRevokeUnavailabilities()) != 1 ||
		got.GetRevokeUnavailabilities()[0].GetReason() != adminpb.AuthorityOrderUnavailableReason_AUTHORITY_ORDER_UNAVAILABLE_REASON_COMPLETED ||
		got.GetRevokeUnavailabilities()[0].GetMessage().GetMessage() != "這筆來函已完成。" {
		t.Fatalf("已完成的來函不能撤銷：%v", got.GetRevokeUnavailabilities())
	}
	first, second := got.GetTargets()[0], got.GetTargets()[1]
	if !first.GetCanRecordNotice() || len(first.GetRecordNoticeUnavailabilities()) != 0 {
		t.Fatalf("暫不告知的對象可以記下告知：%v", first)
	}
	if second.GetCanRecordNotice() || second.GetRecordNoticeUnavailabilities()[0].GetReason() != adminpb.AuthorityOrderUnavailableReason_AUTHORITY_ORDER_UNAVAILABLE_REASON_NOTICE_NOT_WITHHELD {
		t.Fatalf("沒有暫不告知的對象不能記下告知：%v", second)
	}
	if st := first.GetCoveredStores()[0]; st.GetCanCreateMeasure() || !st.GetCanCreateTermination() {
		t.Fatalf("每家店的建立措施入口：%v", st)
	}

	revoked := authorityFixture(false)
	revoked.Deadline = nil // 撤銷後函式不再回這份來函，讀取路徑不填
	at := governanceNow.Add(time.Hour)
	revoked.Revoked = &model.AuthorityRevocation{At: at, AdminID: 9, Reason: "登記錯誤"}
	revoked.Availability = revoked.AuthorityOrder.Availability()
	r := toAuthorityOrder(revoked)
	if r.GetState() != adminpb.AuthorityOrderState_AUTHORITY_ORDER_STATE_REVOKED || r.GetDeadline() != nil || r.GetRevokeReason() != "登記錯誤" ||
		r.GetRevokedByAdminId() != "9" || !r.GetRevokeTime().AsTime().Equal(at) || r.GetCanComplete() || r.GetCanExtendHold() || r.GetCanRevoke() {
		t.Fatalf("revoked=%v", r)
	}

	m := &authorityModule{order: authorityFixture(false)}
	h := NewGovernance(m, rpc.Images{})
	id := m.order.ID.String()
	resp, err := h.RevokeAuthorityOrder(adminCtx(42), connect.NewRequest(&adminpb.RevokeAuthorityOrderRequest{Id: id, Etag: "3", Reason: "登記錯誤"}))
	if err != nil {
		t.Fatal(err)
	}
	if v := m.revoke; v.AdminID != 42 || v.ExpectedVersion != 3 || v.Reason != "登記錯誤" || v.ID != m.order.ID || !resp.Msg.GetAlreadyRevoked() {
		t.Fatalf("revoke=%+v resp=%v", v, resp.Msg)
	}
	calls := m.calls
	_, err = h.RevokeAuthorityOrder(adminCtx(42), connect.NewRequest(&adminpb.RevokeAuthorityOrderRequest{Id: id, Reason: "x"}))
	wantInvalidArgument(t, err)
	if m.calls != calls {
		t.Fatal("沒有 etag 不呼叫模組")
	}
}
