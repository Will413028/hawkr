package buyerprocurement

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type quoteFake struct {
	lists    []procurement.ListSellerResponsesInput
	puts     []int32
	deletes  []int32
	accepts  []procurement.AcceptSellerQuoteInput
	err      error
	page     procurement.SellerResponsePage
	entry    procurement.ShortlistEntry
	accepted procurement.QuoteAcceptance
}

func (f *fakeModule) ListSellerResponses(_ context.Context, in procurement.ListSellerResponsesInput) (procurement.SellerResponsePage, error) {
	f.quote.lists = append(f.quote.lists, in)
	return f.quote.page, f.quote.err
}

func (f *fakeModule) PutShortlistEntry(_ context.Context, _ int64, _, _ uuid.UUID, version int32) (procurement.ShortlistEntry, error) {
	f.quote.puts = append(f.quote.puts, version)
	e := f.quote.entry
	e.Version = version
	return e, f.quote.err
}

func (f *fakeModule) DeleteShortlistEntry(_ context.Context, _ int64, _, _ uuid.UUID, version int32) (procurement.ShortlistEntry, error) {
	f.quote.deletes = append(f.quote.deletes, version)
	e := f.quote.entry
	e.Version, e.Present = version, false
	return e, f.quote.err
}

func (f *fakeModule) AcceptSellerQuote(_ context.Context, in procurement.AcceptSellerQuoteInput) (procurement.QuoteAcceptance, error) {
	f.quote.accepts = append(f.quote.accepts, in)
	return f.quote.accepted, f.quote.err
}

func testQuote() procurement.SellerQuote {
	lead, day := int32(3), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	return procurement.SellerQuote{
		ResponseType: "MANUAL_QUOTE", SpecNote: "規格如圖", UnitBasisQuantity: "1.000", UnitBasisUnit: "KG", OfferedQuantity: "3.000", OfferedQuantityUnit: "KG",
		ConversionBasis: "x", Currency: "TWD", GoodsAmount: 900, ShippingAmount: 100, TaxAmount: 50, TaxInclusionBasis: "TAX_EXCLUDED", TotalAmount: 1050,
		LeadTimeDays: &lead, PromisedDelivery: &day, ValidUntil: day.Add(96 * time.Hour), TermsNote: "月結", DeviationDisclosure: "無",
	}
}

func TestQuoteMethodsValidateAtTheEntryAndNameTheField(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	call := func(name string, f func() error, field string) {
		t.Helper()
		err := f()
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，要 INVALID_ARGUMENT", name, err)
			return
		}
		if got := violationFields(t, err); len(got) != 1 || got[0] != field {
			t.Errorf("%s：欄位 = %v，要 [%s]", name, got, field)
		}
	}
	list := func(r *pb.ListSellerResponsesRequest) func() error {
		return func() error {
			_, err := c.ListSellerResponses(t.Context(), as(connect.NewRequest(r), accountID))
			return err
		}
	}
	call("list 沒有 request_id", list(&pb.ListSellerResponsesRequest{PublicationId: goodID}), "request_id")
	call("list 沒有 publication_id", list(&pb.ListSellerResponsesRequest{RequestId: goodID}), "publication_id")
	call("list publication_id 不合法", list(&pb.ListSellerResponsesRequest{RequestId: goodID, PublicationId: "x"}), "publication_id")

	put := func(r *pb.PutShortlistEntryRequest) func() error {
		return func() error {
			_, err := c.PutShortlistEntry(t.Context(), as(connect.NewRequest(r), accountID))
			return err
		}
	}
	del := func(r *pb.DeleteShortlistEntryRequest) func() error {
		return func() error {
			_, err := c.DeleteShortlistEntry(t.Context(), as(connect.NewRequest(r), accountID))
			return err
		}
	}
	for name, version := range map[string]string{"空": "", "不是數字": "x", "零": "0", "負數": "-1", "超過 int32": "4294967296"} {
		call("put 版本 "+name, put(&pb.PutShortlistEntryRequest{RequestId: goodID, SellerResponseId: goodID, SellerResponseVersion: version}), "seller_response_version")
		call("delete 版本 "+name, del(&pb.DeleteShortlistEntryRequest{RequestId: goodID, SellerResponseId: goodID, SellerResponseVersion: version}), "seller_response_version")
	}
	call("put 沒有回價", put(&pb.PutShortlistEntryRequest{RequestId: goodID, SellerResponseVersion: "1"}), "seller_response_id")
	call("delete 沒有需求", del(&pb.DeleteShortlistEntryRequest{SellerResponseId: goodID, SellerResponseVersion: "1"}), "request_id")

	accept := func(r *pb.AcceptSellerQuoteRequest) func() error {
		return func() error {
			_, err := c.AcceptSellerQuote(t.Context(), as(connect.NewRequest(r), accountID))
			return err
		}
	}
	good := func() *pb.AcceptSellerQuoteRequest {
		return &pb.AcceptSellerQuoteRequest{RequestId: goodID, ClientRequestId: goodID, SellerResponseId: goodID, ExpectedSellerResponseVersion: "2", ExpectedSellerFacingHash: "h"}
	}
	a := good()
	a.ClientRequestId = ""
	call("accept 沒有冪等鍵", accept(a), "client_request_id")
	a = good()
	a.ClientRequestId = "x"
	call("accept 冪等鍵不是 UUIDv4", accept(a), "client_request_id")
	a = good()
	a.SellerResponseId = ""
	call("accept 沒有回價", accept(a), "seller_response_id")
	a = good()
	a.ExpectedSellerResponseVersion = "0"
	call("accept 版本是零", accept(a), "expected_seller_response_version")
	a = good()
	a.ExpectedSellerFacingHash = ""
	call("accept 沒有 hash", accept(a), "expected_seller_facing_hash")

	if len(m.quote.lists)+len(m.quote.puts)+len(m.quote.deletes)+len(m.quote.accepts) != 0 {
		t.Fatalf("驗證失敗的請求不該進到模組：%+v", m.quote)
	}
}

func TestQuoteMethodsRequireTheBuyerAccount(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	if _, err := c.ListSellerResponses(t.Context(), connect.NewRequest(&pb.ListSellerResponsesRequest{RequestId: goodID, PublicationId: goodID})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("沒有登入：%v", err)
	}
	if len(m.quote.lists) != 0 {
		t.Fatal("沒有登入不該進到模組")
	}
}

func TestListSellerResponsesMapsTheRowsAndStoreCards(t *testing.T) {
	m := &fakeModule{}
	id := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	m.quote.page = procurement.SellerResponsePage{
		NextPageToken: "next",
		Responses: []procurement.SellerResponse{
			{ID: id, Version: 2, Status: procurement.ResponseActive, PayloadHash: "p", SubmittedAt: rfqTime, StoreID: 7, Quote: testQuote(), OutsideBuyerRequestedRange: true, Shortlisted: true, Acceptability: procurement.AcceptabilityAcceptable},
			{ID: id, Version: 1, Status: procurement.ResponseWithdrawn, StoreID: 404, Quote: testQuote(), Acceptability: procurement.AcceptabilityWithdrawn},
		},
	}
	c := setup(t, m)
	res, err := c.ListSellerResponses(t.Context(), as(connect.NewRequest(&pb.ListSellerResponsesRequest{RequestId: goodID, PublicationId: goodID, PageSize: 5, PageToken: "t"}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	if in := m.quote.lists[0]; in.BuyerID != buyerID || in.PageSize != 5 || in.PageToken != "t" || in.PublicationID.String() != goodID {
		t.Fatalf("輸入 = %+v", in)
	}
	got := res.Msg.GetSellerResponses()
	if len(got) != 2 || res.Msg.GetNextPageToken() != "next" {
		t.Fatalf("回應 = %+v", res.Msg)
	}
	first, second := got[0], got[1]
	if first.GetSellerResponseVersion() != "2" || first.GetStatus() != pb.SellerResponseStatus_SELLER_RESPONSE_STATUS_ACTIVE || !first.GetOutsideBuyerRequestedRange() || !first.GetShortlisted() ||
		first.GetAcceptability() != pb.QuoteAcceptability_QUOTE_ACCEPTABILITY_ACCEPTABLE || first.GetPayloadHash() != "p" || first.GetStore().GetName() != "店7" {
		t.Fatalf("第一筆 = %+v", first)
	}
	q := first.GetQuote()
	if q.GetResponseType() != pb.SellerResponseType_SELLER_RESPONSE_TYPE_MANUAL_QUOTE || q.GetTaxInclusionBasis() != pb.TaxInclusionBasis_TAX_INCLUSION_BASIS_TAX_EXCLUDED ||
		q.GetUnitBasis().GetQuantity() != "1.000" || q.GetSpecNote() != "規格如圖" || q.GetTotalAmount() != 1050 || q.GetLeadTimeDays() != 3 ||
		q.GetPromisedDeliveryTime() == nil || q.GetValidUntilTime() == nil || q.GetTermsNote() != "月結" || q.GetDeviationDisclosure() != "無" {
		t.Fatalf("報價 = %+v", q)
	}
	if second.GetStore() != nil || second.GetStatus() != pb.SellerResponseStatus_SELLER_RESPONSE_STATUS_WITHDRAWN || second.GetAcceptability() != pb.QuoteAcceptability_QUOTE_ACCEPTABILITY_WITHDRAWN {
		t.Fatalf("沒公開過的店不設 store；第二筆 = %+v", second)
	}
}

func TestListSellerResponsesRejectsAnUnknownStatusCode(t *testing.T) {
	m := &fakeModule{}
	m.quote.page = procurement.SellerResponsePage{Responses: []procurement.SellerResponse{{ID: uuid.NewV7(), Version: 1, Status: "UNKNOWN", Quote: testQuote(), Acceptability: procurement.AcceptabilityAcceptable}}}
	c := setup(t, m)
	_, err := c.ListSellerResponses(t.Context(), as(connect.NewRequest(&pb.ListSellerResponsesRequest{RequestId: goodID, PublicationId: goodID}), accountID))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("不認得的狀態代碼是不變量被破壞：%v", err)
	}
}

func TestShortlistMethodsEchoTheVersionAndReportPresence(t *testing.T) {
	m := &fakeModule{}
	m.quote.entry = procurement.ShortlistEntry{SellerResponseID: uuid.MustParse(goodID), Present: true}
	c := setup(t, m)
	put, err := c.PutShortlistEntry(t.Context(), as(connect.NewRequest(&pb.PutShortlistEntryRequest{RequestId: goodID, SellerResponseId: goodID, SellerResponseVersion: "3"}), accountID))
	if err != nil || put.Msg.GetSellerResponseVersion() != "3" || !put.Msg.GetPresent() || put.Msg.GetSellerResponseId() != goodID {
		t.Fatalf("put = %+v %v", put, err)
	}
	del, err := c.DeleteShortlistEntry(t.Context(), as(connect.NewRequest(&pb.DeleteShortlistEntryRequest{RequestId: goodID, SellerResponseId: goodID, SellerResponseVersion: "3"}), accountID))
	if err != nil || del.Msg.GetSellerResponseVersion() != "3" || del.Msg.GetPresent() {
		t.Fatalf("delete = %+v %v", del, err)
	}
}

func TestAcceptSellerQuoteMapsTheAcceptanceAndTheErrors(t *testing.T) {
	m := &fakeModule{}
	order, deadline := "HK1", rfqTime.Add(24*time.Hour)
	m.quote.accepted = procurement.QuoteAcceptance{
		ID: rfqID, SellerResponseID: rfqID, Version: 2, AcceptedDealPrice: 1050, AcceptedAt: rfqTime, Provisioning: procurement.ProvisioningOrderCreated, OrderNo: &order, PaymentDeadline: &deadline, Replayed: true,
	}
	c := setup(t, m)
	req := &pb.AcceptSellerQuoteRequest{RequestId: goodID, ClientRequestId: goodID, SellerResponseId: goodID, ExpectedSellerResponseVersion: "2", ExpectedSellerFacingHash: "h"}
	res, err := c.AcceptSellerQuote(t.Context(), as(connect.NewRequest(req), accountID))
	if err != nil {
		t.Fatal(err)
	}
	in := m.quote.accepts[0]
	if in.BuyerID != buyerID || in.ExpectedVersion != 2 || in.ExpectedSellerFacingHash != "h" || in.ClientRequestID.String() != goodID || in.ResponseID.String() != goodID {
		t.Fatalf("輸入 = %+v", in)
	}
	r := res.Msg
	if r.GetBuyerAcceptanceId() != rfqID.String() || r.GetSellerResponseVersion() != "2" || r.GetAcceptedDealPrice() != 1050 || !r.GetReplayed() || r.GetOrderNo() != "HK1" ||
		r.GetOrderProvisioningState() != pb.OrderProvisioningState_ORDER_PROVISIONING_STATE_ORDER_CREATED || r.GetPaymentDeadline() == nil || r.GetAcceptTime() == nil {
		t.Fatalf("回應 = %+v", r)
	}

	m.quote.accepted = procurement.QuoteAcceptance{ID: rfqID, SellerResponseID: rfqID, Version: 2, Provisioning: procurement.ProvisioningCreating, AcceptedAt: rfqTime}
	res, err = c.AcceptSellerQuote(t.Context(), as(connect.NewRequest(req), accountID))
	if err != nil || res.Msg.GetOrderNo() != "" || res.Msg.GetPaymentDeadline() != nil || res.Msg.GetReplayed() {
		t.Fatalf("建單前沒有 order_no 與 payment_deadline：%+v %v", res, err)
	}

	for name, c2 := range map[string]struct {
		err  error
		code connect.Code
	}{
		"改版":    {apperr.VersionConflict("x").WithMetadata("conflictTarget", "seller_response"), connect.CodeAborted},
		"過期":    {apperr.FailedPrecondition("x").WithReason(apperr.ReasonRfqResponseExpired), connect.CodeFailedPrecondition},
		"收件人關閉": {apperr.FailedPrecondition("x").WithReason(apperr.ReasonRfqRecipientClosed), connect.CodeFailedPrecondition},
		"進行中":   {apperr.FailedPrecondition("x").WithReason(apperr.ReasonRfqCheckoutAlreadyActive).WithMetadata("orderNo", "HK1"), connect.CodeFailedPrecondition},
		"不屬於本人": {apperr.NotFound("x"), connect.CodeNotFound},
	} {
		m.quote.err = c2.err
		_, err := c.AcceptSellerQuote(t.Context(), as(connect.NewRequest(req), accountID))
		if connect.CodeOf(err) != c2.code {
			t.Errorf("%s：%v，要 %v", name, err, c2.code)
		}
	}
}
