package buyerprocurement

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/date"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// rfqFake 記下詢價發布鏈三支方法收到的輸入，並回預設的結果或指定的錯誤。
type rfqFake struct {
	prices     []procurement.PreviewRfqPriceInput
	previews   []procurement.PreviewRfqPublicationInput
	executes   []procurement.ExecuteRfqPublicationInput
	err        error
	publicaton procurement.Publication
}

var (
	rfqID   = uuid.MustParse("33333333-3333-4333-8333-333333333333")
	rfqTime = time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)
)

func (f *fakeModule) PreviewRfqPrice(_ context.Context, in procurement.PreviewRfqPriceInput) (procurement.PriceQuote, error) {
	f.rfq.prices = append(f.rfq.prices, in)
	if f.rfq.err != nil {
		return procurement.PriceQuote{}, f.rfq.err
	}
	return procurement.PriceQuote{
		LowerBoundAmount: in.LowerBoundAmount, UpperBoundAmount: in.UpperBoundAmount, BasisQuantity: 1000, BasisUnit: "GRAM",
		Total: &procurement.QuoteTotal{LowerBoundAmount: 25, UpperBoundAmount: 50, Quantity: 250, Unit: "GRAM"},
	}, nil
}

func (f *fakeModule) PreviewRfqPublication(_ context.Context, in procurement.PreviewRfqPublicationInput) (procurement.PublicationPreview, error) {
	f.rfq.previews = append(f.rfq.previews, in)
	if f.rfq.err != nil {
		return procurement.PublicationPreview{}, f.rfq.err
	}
	day := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	qty, unit := int32(3), "KG"
	return procurement.PublicationPreview{
		ApprovalID: rfqID, ApprovalVersion: 1, AudienceHash: "a", SellerFacingHash: "s", ExpiresAt: rfqTime, RecipientCount: 2,
		Payload: procurement.SellerFacing{
			Item: "小白菜", ItemLabel: "小白菜", Quantity: &qty, Unit: &unit, DeliveryCity: "台北市", DeliveryDate: &day, ResponseDate: &day, BuyerNote: "請盡快",
			BasisQuantity: 1, BasisUnit: "KG", LowerBoundAmount: 100, UpperBoundAmount: 200,
		},
	}, nil
}

func (f *fakeModule) ExecuteRfqPublication(_ context.Context, in procurement.ExecuteRfqPublicationInput) (procurement.Publication, error) {
	f.rfq.executes = append(f.rfq.executes, in)
	if f.rfq.err != nil {
		return procurement.Publication{}, f.rfq.err
	}
	return f.rfq.publicaton, nil
}

func TestRfqMethodsValidateAtTheEntryAndNameTheField(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	rng := &pb.BuyerRequestedRange{LowerBoundAmount: 1, UpperBoundAmount: 2}
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
	price := func(r *pb.PreviewRfqPriceRequest) func() error {
		return func() error {
			_, err := c.PreviewRfqPrice(t.Context(), as(connect.NewRequest(r), accountID))
			return err
		}
	}
	call("price 沒有 request_id", price(&pb.PreviewRfqPriceRequest{ExpectedRevisionId: goodID, BuyerRequestedRange: rng}), "request_id")
	call("price expected_revision_id 不合法", price(&pb.PreviewRfqPriceRequest{RequestId: goodID, ExpectedRevisionId: "x", BuyerRequestedRange: rng}), "expected_revision_id")
	call("price 沒有區間", price(&pb.PreviewRfqPriceRequest{RequestId: goodID, ExpectedRevisionId: goodID}), "buyer_requested_range")

	preview := func(r *pb.PreviewRfqPublicationRequest) func() error {
		return func() error {
			_, err := c.PreviewRfqPublication(t.Context(), as(connect.NewRequest(r), accountID))
			return err
		}
	}
	pgood := func() *pb.PreviewRfqPublicationRequest {
		return &pb.PreviewRfqPublicationRequest{
			RequestId: goodID, ClientRequestId: goodID, ExpectedRevisionId: goodID, BuyerRequestedRange: &pb.BuyerRequestedRange{LowerBoundAmount: 1, UpperBoundAmount: 2},
		}
	}
	p := pgood()
	p.BuyerRequestedRange = nil
	call("preview 沒有區間", preview(p), "buyer_requested_range")
	p = pgood()
	p.CandidateSetId = "x"
	call("preview 候選集 ID 不合法", preview(p), "candidate_set_id")
	p = pgood()
	p.SelectedCandidateIds = []string{goodID, "x"}
	call("preview 第二個候選 ID 不合法", preview(p), "selected_candidate_ids[1]")
	p = pgood()
	p.ResponseDeadline = &date.Date{Year: 2026, Month: 2, Day: 30}
	call("preview 日期不合法", preview(p), "response_deadline")

	execute := func(r *pb.ExecuteRfqPublicationRequest) func() error {
		return func() error {
			_, err := c.ExecuteRfqPublication(t.Context(), as(connect.NewRequest(r), accountID))
			return err
		}
	}
	egood := func() *pb.ExecuteRfqPublicationRequest {
		return &pb.ExecuteRfqPublicationRequest{RequestId: goodID, ClientRequestId: goodID, ApprovalId: goodID, ApprovalVersion: "1", ExpectedRevisionId: goodID, PayloadHash: "s"}
	}
	for name, version := range map[string]string{"空": "", "不是數字": "x", "零": "0", "負數": "-1", "超過 int32": "4294967296"} {
		e := egood()
		e.ApprovalVersion = version
		call("execute approval_version "+name, execute(e), "approval_version")
	}
	e := egood()
	e.ApprovalId = ""
	call("execute 沒有核准", execute(e), "approval_id")
	e = egood()
	e.PayloadHash = ""
	call("execute 沒有 hash", execute(e), "payload_hash")

	if n := len(m.rfq.prices) + len(m.rfq.previews) + len(m.rfq.executes); n != 0 {
		t.Fatalf("被拒絕的請求不得到達模組：%+v", m.rfq)
	}
}

func TestPreviewRfqPricePassesTheCallerAndMapsTheResult(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	res, err := c.PreviewRfqPrice(t.Context(), as(connect.NewRequest(&pb.PreviewRfqPriceRequest{
		RequestId: goodID, ExpectedRevisionId: goodID, BuyerRequestedRange: &pb.BuyerRequestedRange{LowerBoundAmount: 100, UpperBoundAmount: 200},
	}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	got := m.rfq.prices[0]
	if got.BuyerID != buyerID || got.LowerBoundAmount != 100 || got.UpperBoundAmount != 200 || got.RequestID.String() != goodID || got.ExpectedRevisionID.String() != goodID {
		t.Fatalf("模組收到 %+v", got)
	}
	r := res.Msg
	if r.GetBuyerRequestedRange().GetBasisQuantity() != 1000 || r.GetBuyerRequestedRange().GetBasisUnit() != "GRAM" ||
		r.GetBuyerRequestedRange().GetLowerBoundAmount() != 100 || r.GetQuoteTotalRange().GetLowerBoundAmount() != 25 || r.GetQuoteTotalRange().GetQuantity() != 250 || r.GetQuoteTotalRange().GetQuantityUnit() != "GRAM" {
		t.Fatalf("回應 = %v", r)
	}
}

func TestRfqMethodsMapModuleErrors(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	price := &pb.PreviewRfqPriceRequest{RequestId: goodID, ExpectedRevisionId: goodID, BuyerRequestedRange: &pb.BuyerRequestedRange{LowerBoundAmount: 1, UpperBoundAmount: 2}}
	m.rfq.err = apperr.InvalidInputOn([]string{"LowerBoundAmount", "UpperBoundAmount"}, "區間不合法")
	_, err := c.PreviewRfqPrice(t.Context(), as(connect.NewRequest(price), accountID))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("區間不合法 = %v", err)
	}
	if got := violationFields(t, err); len(got) != 2 || got[0] != "buyer_requested_range.lower_bound_amount" || got[1] != "buyer_requested_range.upper_bound_amount" {
		t.Fatalf("欄位 = %v", got)
	}
	pub := &pb.PreviewRfqPublicationRequest{RequestId: goodID, ClientRequestId: goodID, ExpectedRevisionId: goodID, BuyerRequestedRange: &pb.BuyerRequestedRange{LowerBoundAmount: 1, UpperBoundAmount: 2}}
	m.rfq.err = apperr.InvalidInputOn([]string{"client_request_id"}, "內容不同")
	_, err = c.PreviewRfqPublication(t.Context(), as(connect.NewRequest(pub), accountID))
	if got := violationFields(t, err); len(got) != 1 || got[0] != "client_request_id" {
		t.Fatalf("冪等鍵衝突的欄位 = %v", got)
	}
	m.rfq.err = apperr.InvalidInputOn([]string{"BuyerNote", "ResponseDeadline"}, "不合法")
	_, err = c.PreviewRfqPublication(t.Context(), as(connect.NewRequest(pub), accountID))
	if got := violationFields(t, err); len(got) != 2 || got[0] != "buyer_note" || got[1] != "response_deadline" {
		t.Fatalf("補充說明與期限的欄位 = %v", got)
	}

	execute := &pb.ExecuteRfqPublicationRequest{RequestId: goodID, ClientRequestId: goodID, ApprovalId: goodID, ApprovalVersion: "1", ExpectedRevisionId: goodID, PayloadHash: "s"}
	m.rfq.err = apperr.VersionConflict("過時").WithMetadata("conflictTarget", "publication_approval")
	_, err = c.ExecuteRfqPublication(t.Context(), as(connect.NewRequest(execute), accountID))
	if connect.CodeOf(err) != connect.CodeAborted || reasonOf(t, err) != apperr.ReasonVersionConflict.String() {
		t.Fatalf("核准過時 = %v，要 ABORTED／VERSION_CONFLICT", err)
	}
	m.rfq.err = apperr.NotFound("找不到")
	_, err = c.ExecuteRfqPublication(t.Context(), as(connect.NewRequest(execute), accountID))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("別人的核准 = %v，要 NOT_FOUND", err)
	}
	m.rfq.err = apperr.FailedPrecondition("算不出").WithReason(apperr.ReasonRequirementNotReady)
	_, err = c.PreviewRfqPrice(t.Context(), as(connect.NewRequest(price), accountID))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || reasonOf(t, err) != apperr.ReasonRequirementNotReady.String() {
		t.Fatalf("沒有單位 = %v", err)
	}
}

func TestPreviewRfqPublicationPassesTheAudienceAndMapsThePayload(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	second := "44444444-4444-4444-8444-444444444444"
	res, err := c.PreviewRfqPublication(t.Context(), as(connect.NewRequest(&pb.PreviewRfqPublicationRequest{
		RequestId: goodID, ClientRequestId: goodID, ExpectedRevisionId: goodID,
		BuyerRequestedRange: &pb.BuyerRequestedRange{LowerBoundAmount: 100, UpperBoundAmount: 200}, BuyerNote: "  請盡快 ",
		CandidateSetId: goodID, SelectedCandidateIds: []string{second, goodID},
		ResponseDeadline: &date.Date{Year: 2026, Month: 12, Day: 15},
	}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	in := m.rfq.previews[0]
	if in.BuyerID != buyerID || in.CandidateSetID.String() != goodID ||
		len(in.SelectedCandidateIDs) != 2 || in.SelectedCandidateIDs[0].String() != second || in.LowerBoundAmount != 100 || in.UpperBoundAmount != 200 || in.BuyerNote != "  請盡快 " ||
		in.ResponseDeadline == nil || !in.ResponseDeadline.Equal(time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("模組收到 %+v", in)
	}
	r := res.Msg
	pl := r.GetSellerFacingPayload()
	if r.GetApprovalId() != rfqID.String() || r.GetApprovalVersion() != "1" || r.GetAudienceHash() != "a" || r.GetSellerFacingPayloadHash() != "s" || r.GetRecipientCount() != 2 ||
		!r.GetExpireTime().AsTime().Equal(rfqTime) {
		t.Fatalf("回應 = %v", r)
	}
	if pl.GetItem() != "小白菜" || pl.GetQuantity() != 3 || pl.GetUnit() != "KG" || pl.GetDeliveryCity() != "台北市" || pl.GetDeliveryDate().GetDay() != 1 ||
		pl.GetResponseDeadline().GetMonth() != 12 || pl.GetBuyerNote() != "請盡快" || pl.GetBuyerRequestedRange().GetLowerBoundAmount() != 100 || pl.GetBuyerRequestedRange().GetBasisUnit() != "KG" {
		t.Fatalf("賣家看得到的內容 = %v", pl)
	}
	if pl.Packaging != nil {
		t.Fatalf("沒有包裝不設值：%v", pl.Packaging)
	}

	_, err = c.PreviewRfqPublication(t.Context(), as(connect.NewRequest(&pb.PreviewRfqPublicationRequest{
		RequestId: goodID, ClientRequestId: goodID, ExpectedRevisionId: goodID, BuyerRequestedRange: &pb.BuyerRequestedRange{LowerBoundAmount: 1, UpperBoundAmount: 2},
	}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	if in := m.rfq.previews[1]; in.CandidateSetID != (uuid.UUID{}) || len(in.SelectedCandidateIDs) != 0 || in.ResponseDeadline != nil {
		t.Fatalf("沒帶的候選與期限不設值：%+v", in)
	}
}

func TestPreviewRfqPublicationMapsModuleFieldErrorsToTheRequestPaths(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	m.rfq.err = apperr.InvalidInputOn([]string{apperr.IndexedField("SelectedCandidateIDs", 1)}, "候選重複")
	_, err := c.PreviewRfqPublication(t.Context(), as(connect.NewRequest(&pb.PreviewRfqPublicationRequest{
		RequestId: goodID, ClientRequestId: goodID, ExpectedRevisionId: goodID, BuyerRequestedRange: &pb.BuyerRequestedRange{LowerBoundAmount: 1, UpperBoundAmount: 2},
	}), accountID))
	if got := violationFields(t, err); len(got) != 1 || got[0] != "selected_candidate_ids[1]" {
		t.Fatalf("欄位 = %v", got)
	}
}

func TestExecuteRfqPublicationPassesTheCallerAndMapsTheResult(t *testing.T) {
	m := &fakeModule{rfq: rfqFake{publicaton: procurement.Publication{
		ID: rfqID, Version: 1, State: "PUBLISHED", AudienceHash: "a", SellerFacingHash: "s", PublishedAt: rfqTime, Replayed: true,
	}}}
	c := setup(t, m)
	res, err := c.ExecuteRfqPublication(t.Context(), as(connect.NewRequest(&pb.ExecuteRfqPublicationRequest{
		RequestId: goodID, ClientRequestId: goodID, ApprovalId: goodID, ApprovalVersion: "7", ExpectedRevisionId: goodID, PayloadHash: "s",
	}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	if in := m.rfq.executes[0]; in.BuyerID != buyerID || in.ApprovalVersion != 7 || in.PayloadHash != "s" || in.ApprovalID.String() != goodID || in.ExpectedRevisionID.String() != goodID {
		t.Fatalf("模組收到 %+v", in)
	}
	r := res.Msg
	if r.GetPublicationId() != rfqID.String() || r.GetPublicationVersion() != "1" || r.GetState() != pb.PublicationState_PUBLICATION_STATE_PUBLISHED ||
		r.GetAudienceHash() != "a" || r.GetSellerFacingPayloadHash() != "s" ||
		!r.GetPublishTime().AsTime().Equal(rfqTime) || !r.GetReplayed() {
		t.Fatalf("回應 = %v", r)
	}
}

func TestExecuteRfqPublicationRejectsAResultItCannotRepresent(t *testing.T) {
	good := procurement.Publication{ID: rfqID, Version: 1, State: "PUBLISHED", PublishedAt: rfqTime}
	for name, bad := range map[string]procurement.Publication{
		"沒有狀態":   {ID: good.ID, Version: 1},
		"不認得的狀態": {ID: good.ID, Version: 1, State: "GONE"},
	} {
		c := setup(t, &fakeModule{rfq: rfqFake{publicaton: bad}})
		_, err := c.ExecuteRfqPublication(t.Context(), as(connect.NewRequest(&pb.ExecuteRfqPublicationRequest{
			RequestId: goodID, ClientRequestId: goodID, ApprovalId: goodID, ApprovalVersion: "1", ExpectedRevisionId: goodID, PayloadHash: "s",
		}), accountID))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Errorf("%s：%v，要 INTERNAL", name, err)
		}
	}
}

func TestDateFromProto(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   *date.Date
		ok   bool
	}{
		{"nil", nil, true}, {"valid", &date.Date{Year: 2026, Month: 12, Day: 31}, true}, {"leap day", &date.Date{Year: 2028, Month: 2, Day: 29}, true},
		{"not a leap year", &date.Date{Year: 2026, Month: 2, Day: 29}, false}, {"month 13", &date.Date{Year: 2026, Month: 13, Day: 1}, false},
		{"day 0", &date.Date{Year: 2026, Month: 1, Day: 0}, false}, {"year 0", &date.Date{Year: 0, Month: 1, Day: 1}, false}, {"all zero", &date.Date{}, false},
		{"year 10000", &date.Date{Year: 10000, Month: 1, Day: 1}, false},
	} {
		got, err := dateFromProto("response_deadline", tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("%s：err = %v", tc.name, err)
			continue
		}
		if tc.ok && tc.in != nil && (got == nil || got.Year() != int(tc.in.Year) || int(got.Month()) != int(tc.in.Month) || got.Day() != int(tc.in.Day)) {
			t.Errorf("%s：得到 %v", tc.name, got)
		}
		if tc.in == nil && got != nil {
			t.Errorf("nil 要回 nil：%v", got)
		}
	}
}

func TestRfqFieldPathsExistInTheRequestMessages(t *testing.T) {
	for name, tc := range map[string]struct {
		paths   rpc.FieldPaths
		missing []string
	}{
		"price":   {pricePaths, rpc.CheckFieldPaths((&pb.PreviewRfqPriceRequest{}).ProtoReflect().Descriptor(), pricePaths)},
		"publish": {publishPaths, rpc.CheckFieldPaths((&pb.PreviewRfqPublicationRequest{}).ProtoReflect().Descriptor(), publishPaths)},
		"execute": {executePaths, rpc.CheckFieldPaths((&pb.ExecuteRfqPublicationRequest{}).ProtoReflect().Descriptor(), executePaths)},
	} {
		if len(tc.missing) != 0 {
			t.Errorf("%s：請求訊息沒有這些路徑：%v", name, tc.missing)
		}
	}
}
