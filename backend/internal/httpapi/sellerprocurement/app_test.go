package sellerprocurement

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	callerAccount = 1
	callerStore   = 11
)

const goodID = "11111111-1111-4111-8111-111111111111"

var at = time.Date(2026, 11, 24, 3, 4, 5, 0, time.UTC)

type fakeModule struct {
	submitIn  procurement.SubmitSellerResponseInput
	draftIn   procurement.SellerQuoteInput
	draftExp  string
	withdraw  [3]string
	declined  string
	calls     []string
	storeSeen int64
	result    procurement.SellerResponseVersion
}

func (f *fakeModule) ListSellerPublications(_ context.Context, storeID int64, _ int32, _ string) (procurement.SellerPublicationPage, error) {
	f.calls, f.storeSeen = append(f.calls, "List"), storeID
	return procurement.SellerPublicationPage{}, nil
}

func (f *fakeModule) GetSellerPublication(_ context.Context, storeID int64, _ uuid.UUID) (procurement.SellerPublicationDetail, error) {
	f.calls, f.storeSeen = append(f.calls, "Get"), storeID
	return procurement.SellerPublicationDetail{}, apperr.NotFound("x")
}

func (f *fakeModule) UpsertSellerResponseDraft(_ context.Context, storeID int64, _ uuid.UUID, expected string, draft procurement.SellerQuoteInput) (procurement.SellerDraft, error) {
	f.calls, f.storeSeen, f.draftIn, f.draftExp = append(f.calls, "Draft"), storeID, draft, expected
	return procurement.SellerDraft{Version: 3, UpdatedAt: at, Quote: draft}, nil
}

func (f *fakeModule) SubmitSellerResponse(_ context.Context, in procurement.SubmitSellerResponseInput) (procurement.SellerResponseVersion, error) {
	f.calls, f.storeSeen, f.submitIn = append(f.calls, "Submit"), in.StoreID, in
	return f.result, nil
}

func (f *fakeModule) WithdrawSellerResponse(_ context.Context, storeID int64, _, _ uuid.UUID, expected, reason string) (procurement.SellerResponseClosure, error) {
	f.calls, f.storeSeen, f.withdraw = append(f.calls, "Withdraw"), storeID, [3]string{expected, reason}
	return procurement.SellerResponseClosure{WithdrawnVersion: 2, LatestVersion: 2}, nil
}

func (f *fakeModule) DeclineRfqPublication(_ context.Context, storeID int64, _ uuid.UUID, reason string) (procurement.SellerResponseClosure, error) {
	f.calls, f.storeSeen, f.declined = append(f.calls, "Decline"), storeID, reason
	closed := at
	return procurement.SellerResponseClosure{LatestVersion: 0, WriteClosedAt: &closed, WriteClosedReason: "DECLINED"}, nil
}

type fakeStores struct{ own bool }

func (f fakeStores) CurrentStore(_ context.Context, accountID int64) (seller.Store, error) {
	if accountID != callerAccount || !f.own {
		return seller.Store{}, apperr.NotFound("no store")
	}
	return seller.Store{StoreID: callerStore}, nil
}

func signedIn() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount})
}

func newTestApp(t *testing.T, own bool) (*App, *fakeModule) {
	t.Helper()
	m := &fakeModule{}
	app, err := NewApp(m, fakeStores{own: own}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return app, m
}

func TestDeclarationsMustMatchTheServiceExactly(t *testing.T) {
	if err := checkDeclarations(declarations, serviceDescriptor()); err != nil {
		t.Fatal(err)
	}
	if got, want := len(declarations), serviceDescriptor().Methods().Len(); got != want || want != 6 {
		t.Fatalf("宣告 %d 支、service %d 支，要都是 6", got, want)
	}
	for proc := range declarations {
		short := map[string]seller.Capability{}
		for k, v := range declarations {
			if k != proc {
				short[k] = v
			}
		}
		if _, err := newApp(short, &fakeModule{}, fakeStores{}, slog.New(slog.DiscardHandler)); err == nil {
			t.Errorf("少了 %s 的宣告仍能啟動", proc)
		}
	}
	extra := map[string]seller.Capability{"/hawkr.procurement.v1alpha1.SellerProcurementAppService/Nope": procurement.RequiredSellerCapability}
	for k, v := range declarations {
		extra[k] = v
	}
	if _, err := newApp(extra, &fakeModule{}, fakeStores{}, slog.New(slog.DiscardHandler)); err == nil {
		t.Error("多出的宣告仍能啟動")
	}
	bad := map[string]seller.Capability{}
	for k := range declarations {
		bad[k] = seller.Capability("CAP_NOPE")
	}
	if _, err := newApp(bad, &fakeModule{}, fakeStores{}, slog.New(slog.DiscardHandler)); err == nil {
		t.Error("不存在的能力仍能啟動")
	}
}

func TestEveryQuoteFieldHasAProtoPathForDraftAndSubmit(t *testing.T) {
	typ := reflect.TypeFor[procurement.SellerQuoteInput]()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if draftPaths[name] == "" || submitPaths[name] == "" {
			t.Errorf("欄位 %s 沒有對應的請求路徑（draft %q、quote %q）", name, draftPaths[name], submitPaths[name])
		}
	}
	if draftPaths["SpecNote"] != "draft.spec_note" || submitPaths["UnitBasisUnit"] != "quote.unit_basis.unit" || submitPaths["ValidUntil"] != "quote.valid_until_time" {
		t.Errorf("路徑 = %v / %v", draftPaths, submitPaths)
	}
}

func wireQuote() *pb.SellerQuote {
	lead := int32(0)
	return &pb.SellerQuote{
		ResponseType: pb.SellerResponseType_SELLER_RESPONSE_TYPE_ACCEPT_NON_PRICE_TERMS_WITH_QUOTE, SpecNote: "規格",
		UnitBasis: &pb.UnitBasis{Quantity: "1", Unit: "KG"}, OfferedQuantity: "12.5", OfferedQuantityUnit: "KG", ConversionBasis: "一箱十公斤",
		GoodsAmount: 1000, ShippingAmount: 50, TaxAmount: 20, TaxInclusionBasis: pb.TaxInclusionBasis_TAX_INCLUSION_BASIS_TAX_EXCLUDED, TotalAmount: 1070,
		LeadTimeDays: &lead, PromisedDeliveryTime: timestamppb.New(at.Add(time.Hour)), ValidUntilTime: timestamppb.New(at.Add(48 * time.Hour)), TermsNote: "月結", DeviationDisclosure: "無",
	}
}

func TestSubmitPassesTheDerivedStoreAndTheWholeQuote(t *testing.T) {
	app, m := newTestApp(t, true)
	cid := uuid.MustParse(goodID)
	req := &pb.SubmitSellerResponseRequest{
		PublicationId: uuid.NewV7().String(), ClientRequestId: "", ExpectedPublicationVersion: "1", ExpectedSellerFacingPayloadHash: "h", ExpectedSellerResponseVersion: "2", Quote: wireQuote(),
	}
	if _, err := app.SubmitSellerResponse(signedIn(), connect.NewRequest(req)); err == nil || len(m.calls) != 0 {
		t.Fatalf("沒有冪等鍵要在到模組之前被擋：%v、呼叫 %v", err, m.calls)
	}
	req.ClientRequestId = "not-a-uuid"
	if _, err := app.SubmitSellerResponse(signedIn(), connect.NewRequest(req)); err == nil || len(m.calls) != 0 {
		t.Fatalf("格式不對的冪等鍵要被擋：%v", err)
	}
	req.ClientRequestId = cid.String()
	m.result = procurement.SellerResponseVersion{ResponseID: uuid.NewV7(), Version: 3, Status: procurement.ResponseActive, Replayed: true, Quote: procurement.SellerQuote{
		ResponseType: "MANUAL_QUOTE", TaxInclusionBasis: "TAX_INCLUDED", ValidUntil: at,
	}}
	resp, err := app.SubmitSellerResponse(signedIn(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	in := m.submitIn
	if m.storeSeen != callerStore || in.StoreID != callerStore || in.ClientRequestID != cid || in.ExpectedPublicationVersion != "1" || in.ExpectedSellerFacingHash != "h" || in.ExpectedResponseVersion != "2" {
		t.Fatalf("傳給模組的輸入 = %+v", in)
	}
	q := in.Quote
	if q.ResponseType != "ACCEPT_NON_PRICE_TERMS_WITH_QUOTE" || q.TaxInclusionBasis != "TAX_EXCLUDED" || q.UnitBasisQuantity != "1" || q.UnitBasisUnit != "KG" || q.OfferedQuantity != "12.5" ||
		q.LeadTimeDays == nil || *q.LeadTimeDays != 0 || q.PromisedDelivery == nil || !q.PromisedDelivery.Equal(at.Add(time.Hour)) || q.ValidUntil == nil || !q.ValidUntil.Equal(at.Add(48*time.Hour)) ||
		q.GoodsAmount != 1000 || q.TotalAmount != 1070 || q.ConversionBasis != "一箱十公斤" || q.TermsNote != "月結" || q.DeviationDisclosure != "無" || q.SpecNote != "規格" {
		t.Fatalf("報價內容 = %+v", q)
	}
	got := resp.Msg.GetSellerResponse()
	if got.GetSellerResponseVersion() != "3" || !got.GetReplayed() || got.GetSupersededByVersion() != "" || got.GetStatus() != pb.SellerResponseStatus_SELLER_RESPONSE_STATUS_ACTIVE {
		t.Fatalf("回應 = %v", got)
	}
}

func TestSubmitRequiresAQuote(t *testing.T) {
	app, m := newTestApp(t, true)
	_, err := app.SubmitSellerResponse(signedIn(), connect.NewRequest(&pb.SubmitSellerResponseRequest{PublicationId: uuid.NewV7().String(), ClientRequestId: goodID}))
	if err == nil || len(m.calls) != 0 {
		t.Fatalf("沒有 quote 要被擋：%v", err)
	}
}

func TestUnspecifiedEnumsAndMissingTimesStayEmptyInADraft(t *testing.T) {
	app, m := newTestApp(t, true)
	resp, err := app.UpsertSellerResponseDraft(signedIn(), connect.NewRequest(&pb.UpsertSellerResponseDraftRequest{
		PublicationId: uuid.NewV7().String(), ExpectedDraftVersion: "2", Draft: &pb.SellerQuote{SpecNote: "只填一點"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if m.draftExp != "2" || m.draftIn.ResponseType != "" || m.draftIn.TaxInclusionBasis != "" || m.draftIn.ValidUntil != nil || m.draftIn.PromisedDelivery != nil || m.draftIn.LeadTimeDays != nil {
		t.Fatalf("傳給模組的草稿 = %+v、expected %q", m.draftIn, m.draftExp)
	}
	d := resp.Msg.GetDraft()
	if d.GetDraftVersion() != "3" || d.GetDraft().GetSpecNote() != "只填一點" || d.GetDraft().GetValidUntilTime() != nil || d.GetDraft().LeadTimeDays != nil || d.GetDraft().GetUnitBasis() != nil {
		t.Fatalf("回應 = %v", d)
	}
	if _, err := app.UpsertSellerResponseDraft(signedIn(), connect.NewRequest(&pb.UpsertSellerResponseDraftRequest{PublicationId: uuid.NewV7().String()})); err == nil {
		t.Fatal("沒有 draft 要被擋")
	}
}

func TestDraftKeepsAZeroLeadTimeAndRoundTrips(t *testing.T) {
	in, err := quoteInputFromProto(wireQuote(), "draft")
	if err != nil {
		t.Fatal(err)
	}
	back, err := quoteInputToProto(in)
	if err != nil {
		t.Fatal(err)
	}
	if back.LeadTimeDays == nil || *back.LeadTimeDays != 0 || back.GetResponseType() != wireQuote().GetResponseType() || back.GetValidUntilTime().AsTime() != wireQuote().GetValidUntilTime().AsTime() {
		t.Fatalf("往返 = %v", back)
	}
}

func TestInvalidTimestampIsAFieldError(t *testing.T) {
	q := wireQuote()
	q.ValidUntilTime = &timestamppb.Timestamp{Seconds: 1 << 60}
	if _, err := quoteInputFromProto(q, "quote"); err == nil {
		t.Fatal("超出範圍的時間戳要被擋")
	}
}

func TestWithdrawDeclineAndReadsUseTheDerivedStore(t *testing.T) {
	app, m := newTestApp(t, true)
	pub, resp := uuid.NewV7().String(), uuid.NewV7().String()
	w, err := app.WithdrawSellerResponse(signedIn(), connect.NewRequest(&pb.WithdrawSellerResponseRequest{
		PublicationId: pub, SellerResponseId: resp, ExpectedSellerResponseVersion: "2", ReasonCode: "PRICE",
	}))
	if err != nil || m.withdraw[0] != "2" || m.withdraw[1] != "PRICE" || w.Msg.GetClosure().GetWithdrawnVersion() != "2" || w.Msg.GetClosure().GetWriteClosedTime() != nil {
		t.Fatalf("撤回：%v、%v、%v", err, m.withdraw, w.Msg)
	}
	d, err := app.DeclineRfqPublication(signedIn(), connect.NewRequest(&pb.DeclineRfqPublicationRequest{PublicationId: pub, ReasonCode: "BUSY"}))
	if err != nil || m.declined != "BUSY" || d.Msg.GetClosure().GetLatestVersion() != "" || d.Msg.GetClosure().GetWithdrawnVersion() != "" ||
		d.Msg.GetClosure().GetWriteClosedReason() != "DECLINED" || d.Msg.GetClosure().GetWriteClosedTime() == nil {
		t.Fatalf("婉拒：%v、%v", err, d)
	}
	if _, err := app.WithdrawSellerResponse(signedIn(), connect.NewRequest(&pb.WithdrawSellerResponseRequest{PublicationId: "x", SellerResponseId: resp})); err == nil {
		t.Error("壞的 publication_id 要被擋")
	}
	if _, err := app.ListSellerPublications(signedIn(), connect.NewRequest(&pb.ListSellerPublicationsRequest{PageSize: 5})); err != nil || m.storeSeen != callerStore {
		t.Fatalf("列表：%v、店 %d", err, m.storeSeen)
	}
}

func TestNoStoreOrAdminNeverReachesTheModule(t *testing.T) {
	app, m := newTestApp(t, false)
	pub := uuid.NewV7().String()
	if _, err := app.GetSellerPublication(signedIn(), connect.NewRequest(&pb.GetSellerPublicationRequest{PublicationId: pub})); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有店要回 NOT_FOUND：%v", err)
	}
	if _, err := app.DeclineRfqPublication(signedIn(), connect.NewRequest(&pb.DeclineRfqPublicationRequest{PublicationId: pub})); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有店要回 NOT_FOUND：%v", err)
	}
	admin := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 9})
	if _, err := app.ListSellerPublications(admin, connect.NewRequest(&pb.ListSellerPublicationsRequest{})); err == nil {
		t.Fatal("管理員不能呼叫")
	}
	if len(m.calls) != 0 {
		t.Fatalf("不該到模組：%v", m.calls)
	}
}
