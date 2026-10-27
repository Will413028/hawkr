package seller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// adminKYCModule 是後台 KYC 與 PayUni 方法的模組替身：記下輸入，回預先放好的結果。
type adminKYCModule struct {
	AdminModule
	reads      int
	requery    model.RequeryKYCRegistrationInput
	requeried  model.RegistrationCheck
	auditIn    model.KYCAuditListInput
	revealPage model.KYCRevealPage
	accessPage model.KYCDocumentAccessPage
	list       model.KYCListInput
	listPage   model.KYCPage
	got        model.KYCAdminApplication
	approve    model.ApproveKYCInput
	reject     model.RejectKYCInput
	reveal     model.RevealKYCInput
	revealed   model.KYCSensitive
	approveErr error
	calls      int
	reg        model.AdminPayUniRegistration
	rebind     model.RebindPayUniInput
	obsIn      model.PayUniObservationListInput
	obsPage    model.PayUniObservationPage
	refreshed  int64
}

func (m *adminKYCModule) AdminKYCApplications(_ context.Context, in model.KYCListInput) (model.KYCPage, error) {
	m.list = in
	return m.listPage, nil
}

func (m *adminKYCModule) AdminKYCApplication(context.Context, uuid.UUID) (model.KYCAdminApplication, error) {
	m.reads++
	return m.got, nil
}

func (m *adminKYCModule) ListKYCReveals(_ context.Context, in model.KYCAuditListInput) (model.KYCRevealPage, error) {
	m.auditIn = in
	return m.revealPage, nil
}

func (m *adminKYCModule) ListKYCDocumentAccesses(_ context.Context, in model.KYCAuditListInput) (model.KYCDocumentAccessPage, error) {
	m.auditIn = in
	return m.accessPage, nil
}

func (m *adminKYCModule) ApproveKYC(_ context.Context, in model.ApproveKYCInput) (model.KYCAdminApplication, error) {
	m.approve, m.calls = in, m.calls+1
	return m.got, m.approveErr
}

func (m *adminKYCModule) RejectKYC(_ context.Context, in model.RejectKYCInput) (model.KYCAdminApplication, error) {
	m.reject, m.calls = in, m.calls+1
	return m.got, nil
}

func (m *adminKYCModule) RevealKYCSensitive(_ context.Context, in model.RevealKYCInput) (model.KYCSensitive, error) {
	m.reveal, m.calls = in, m.calls+1
	return m.revealed, nil
}

func (m *adminKYCModule) PayUniRegistrationOf(context.Context, int64) (model.AdminPayUniRegistration, error) {
	return m.reg, nil
}

func (m *adminKYCModule) RebindPayUni(_ context.Context, in model.RebindPayUniInput) (model.AdminPayUniRegistration, error) {
	m.rebind, m.calls = in, m.calls+1
	return m.reg, nil
}

func (m *adminKYCModule) PayUniObservations(_ context.Context, in model.PayUniObservationListInput) (model.PayUniObservationPage, error) {
	m.obsIn = in
	return m.obsPage, nil
}

func (m *adminKYCModule) RequeryKYCRegistration(_ context.Context, in model.RequeryKYCRegistrationInput) (model.RegistrationCheck, error) {
	m.requery, m.calls = in, m.calls+1
	return m.requeried, nil
}

func (m *adminKYCModule) RefreshPayUniStatus(_ context.Context, id int64) error {
	m.refreshed, m.calls = id, m.calls+1
	return nil
}

const (
	plainID        = "A123456789"
	plainBank      = "012345678901"
	frontAssetID   = "asset-secret-front"
	frontObjectKey = "secure/7111/secret-object-key"
)

// adminApplication 是一份帶遮蔽值、證件（含 asset ID 與 object key）、審核紀錄與讀取紀錄的後台檢視。
func adminApplication() model.KYCAdminApplication {
	now := time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	admin := int64(9)
	return model.KYCAdminApplication{
		KYCApplication: model.KYCApplication{
			ID: uuid.NewV7(), StoreID: 11, AccountID: 7, Status: model.KYCStatusSubmitted, Version: 3, SellerType: model.KYCIndividual,
			RealName: ptr("王小明"), IDCardNoMasked: ptr("A1******89"), BankAccountMasked: ptr("********8901"),
			Documents:   []model.KYCDocument{{Slot: model.SlotIDCardFront, AssetID: frontAssetID, ObjectKey: frontObjectKey}, {Slot: model.SlotBankBook, AssetID: "asset-book", ObjectKey: "secure/7111/book"}},
			SubmittedAt: &now, CreatedAt: now, UpdatedAt: now,
		},
		Reviews: []model.KYCReview{{ID: uuid.NewV7(), Action: model.ReviewApprove, ReviewerAdminID: &admin, Reason: ptr("資料齊全"), DecidedAt: now}, {ID: uuid.NewV7(), Action: model.ReviewSubmit, DecidedAt: now}},

		SharesBankAccountWithSanctionedAccount: true,
	}
}

// 後台的申請只有遮蔽值，證件只有位置：回應裡不出現明文、asset ID、object key 與下載網址。
func TestGetKYCApplicationHasOnlyMaskedValuesAndNoDocumentIdentifiers(t *testing.T) {
	// 全文就在模組手上（revealed），讀取方法如果去拿它、或寫入稽核（呼叫 Reveal），都會被抓到。
	m := &adminKYCModule{got: adminApplication(), revealed: model.KYCSensitive{IDCardNo: ptr(plainID), BankAccount: ptr(plainBank)}}
	resp, err := newTestAdmin(m).GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: m.got.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := protojson.Marshal(resp.Msg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{plainID, plainBank, frontAssetID, "asset-book", frontObjectKey, "secure/", "download", "view_uri", "viewUri"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("回應不能含 %q：%s", forbidden, text)
		}
	}
	if m.calls != 0 || m.reveal.AdminID != 0 {
		t.Fatalf("讀取不能顯示全文（會寫稽核）：%+v", m.reveal)
	}
	a := resp.Msg.GetApplication()
	if a.GetIndividual().GetIdCardNoMasked() != "A1******89" || a.GetBankAccountMasked() != "********8901" || len(a.GetDocuments()) != 2 ||
		a.GetDocuments()[0].GetField() != pb.KYCField_KYC_FIELD_ID_CARD_FRONT || !a.GetSharesBankAccountWithSanctionedAccount() ||
		a.GetEtag() != "3.0" || a.GetStoreId() != "11" || a.GetAccountId() != "7" {
		t.Fatalf("application=%+v", a)
	}
	if len(a.GetReviews()) != 2 || a.GetReviews()[0].GetReviewerAdminId() != "9" || a.GetReviews()[0].GetAction() != adminpb.KYCReviewAction_KYC_REVIEW_ACTION_APPROVE || a.GetReviews()[1].GetReviewerAdminId() != "" {
		t.Fatalf("reviews=%+v", a.GetReviews())
	}
	// 訊息的型別本身沒有能放明文、asset ID 或網址的欄位。
	for _, name := range []string{"id_card_no", "rep_id_card_no", "bank_account", "asset_id", "download_uri", "view_uri", "object_key", "reviewer_name"} {
		for _, d := range []protoreflect.MessageDescriptor{(&adminpb.KYCApplication{}).ProtoReflect().Descriptor(), (&adminpb.KYCDocument{}).ProtoReflect().Descriptor(), (&adminpb.KYCReview{}).ProtoReflect().Descriptor(), (&adminpb.IndividualKYC{}).ProtoReflect().Descriptor(), (&adminpb.BusinessKYC{}).ProtoReflect().Descriptor()} {
			if d.Fields().ByName(protoreflect.Name(name)) != nil {
				t.Fatalf("%s 不該有欄位 %s", d.FullName(), name)
			}
		}
	}
	if _, err := newTestAdmin(m).GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: "not-a-uuid"})); err == nil {
		t.Fatal("bad id")
	}
}

func TestApproveKYCUsesTheCallingAdminTheETagAndTheComment(t *testing.T) {
	m := &adminKYCModule{got: adminApplication()}
	h := newTestAdmin(m)
	id := m.got.ID.String()
	if _, err := h.ApproveKYC(adminCtx(42), connect.NewRequest(&adminpb.ApproveKYCRequest{Id: id, Etag: "3.0", Comment: "資料齊全"})); err != nil {
		t.Fatal(err)
	}
	if m.approve.AdminID != 42 || m.approve.ExpectedVersion != 3 || m.approve.Comment != "資料齊全" || m.approve.ApplicationID != m.got.ID {
		t.Fatalf("approve=%+v", m.approve)
	}
	// 回應就是模組在同一個 transaction 內讀的檢視，handler 不另外再讀一次。
	if m.reads != 0 {
		t.Fatalf("handler 不該事後重讀：%d", m.reads)
	}
	m.calls = 0
	for name, req := range map[string]*adminpb.ApproveKYCRequest{
		"no etag": {Id: id}, "bad etag": {Id: id, Etag: "x"}, "zero etag": {Id: id, Etag: "0.0"}, "bare version": {Id: id, Etag: "3"}, "no seq": {Id: id, Etag: "3."}, "no version": {Id: id, Etag: ".1"}, "negative seq": {Id: id, Etag: "3.-1"}, "extra part": {Id: id, Etag: "3.1.1"}, "leading zero": {Id: id, Etag: "03.1"}, "plus sign": {Id: id, Etag: "+3.1"}, "bad id": {Id: "x", Etag: "3.0"}, "no id": {Etag: "3.0"},
	} {
		_, err := h.ApproveKYC(adminCtx(42), connect.NewRequest(req))
		wantInvalidArgument(t, err)
		if m.calls != 0 {
			t.Fatalf("%s：模組不該被呼叫", name)
		}
	}
	for name, ctx := range map[string]context.Context{"no principal": context.Background(), "customer": rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})} {
		if _, err := h.ApproveKYC(ctx, connect.NewRequest(&adminpb.ApproveKYCRequest{Id: id, Etag: "3.0"})); !errors.Is(err, errNoPrincipal) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// 模組的欄位錯誤原樣交回（欄位路徑的對應由 interceptor 套用，integration 測試在線上確認）。
	m.approveErr = apperr.InvalidInputOn([]string{"Comment"}, "說明太長")
	_, err := h.ApproveKYC(adminCtx(42), connect.NewRequest(&adminpb.ApproveKYCRequest{Id: id, Etag: "3.0", Comment: "x"}))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "Comment" {
		t.Fatalf("err=%v", err)
	}
}

func TestRejectKYCMapsIssuesAndNeverPassesAnUnspecifiedOne(t *testing.T) {
	m := &adminKYCModule{got: adminApplication()}
	h := newTestAdmin(m)
	id := m.got.ID.String()
	defer func() {
		if m.reads != 0 {
			t.Errorf("handler 不該事後重讀：%d", m.reads)
		}
	}()
	if _, err := h.RejectKYC(adminCtx(42), connect.NewRequest(&adminpb.RejectKYCRequest{
		Id: id, Etag: "3.0", Reason: "存摺不清楚", InternalNote: "疑似翻拍",
		Issues: []*pb.KYCIssue{
			{Field: pb.KYCField_KYC_FIELD_BANK_BOOK, DocumentAssetId: "asset-bank", Code: pb.KYCIssueCode_KYC_ISSUE_CODE_UNREADABLE, Message: "反光"},
			{Field: pb.KYCField_KYC_FIELD_BUSINESS_TAX_ID, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_MISMATCH, Message: "與登記不符"},
		},
	})); err != nil {
		t.Fatal(err)
	}
	want := []model.KYCIssue{
		{Field: "bank_book", DocumentAssetID: "asset-bank", Code: model.IssueUnreadable, Message: "反光"},
		{Field: "business.tax_id", Code: model.IssueMismatch, Message: "與登記不符"},
	}
	if m.reject.AdminID != 42 || m.reject.ExpectedVersion != 3 || m.reject.Reason != "存摺不清楚" || m.reject.InternalNote != "疑似翻拍" || !slices.Equal(m.reject.Issues, want) {
		t.Fatalf("reject=%+v", m.reject)
	}
	m.calls = 0
	valid := &pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_BANK_BOOK, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "x"}
	for name, c := range map[string]struct {
		issues []*pb.KYCIssue
		path   string
	}{
		"unspecified field":          {[]*pb.KYCIssue{{Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "x"}}, "issues[0].field"},
		"unspecified field after ok": {[]*pb.KYCIssue{valid, {Field: pb.KYCField_KYC_FIELD_UNSPECIFIED, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "x"}}, "issues[1].field"},
		"unknown field number":       {[]*pb.KYCIssue{{Field: pb.KYCField(999), Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "x"}}, "issues[0].field"},
		"unspecified code":           {[]*pb.KYCIssue{{Field: pb.KYCField_KYC_FIELD_BANK_BOOK, Message: "x"}}, "issues[0].code"},
		"unknown code number":        {[]*pb.KYCIssue{{Field: pb.KYCField_KYC_FIELD_BANK_BOOK, Code: pb.KYCIssueCode(99), Message: "x"}}, "issues[0].code"},
	} {
		_, err := h.RejectKYC(adminCtx(42), connect.NewRequest(&adminpb.RejectKYCRequest{Id: id, Etag: "3.0", Reason: "x", Issues: c.issues}))
		wantInvalidArgument(t, err)
		var ae *apperr.Error
		if !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 {
			t.Fatalf("%s：err=%v", name, err)
		}
		if m.calls != 0 {
			t.Fatalf("%s：模組不該被呼叫", name)
		}
	}
	_, err := h.RejectKYC(adminCtx(42), connect.NewRequest(&adminpb.RejectKYCRequest{Id: id, Reason: "x"}))
	wantInvalidArgument(t, err)
}

// 退回的問題的欄位違規對應到請求的路徑（含索引），模組的內部備註與總理由也各有路徑。
func TestRejectKYCPathsExistInTheRequest(t *testing.T) {
	if missing := rpc.CheckFieldPaths((&adminpb.RejectKYCRequest{}).ProtoReflect().Descriptor(), rejectKYCPaths); len(missing) > 0 {
		t.Fatalf("rejectKYCPaths 在請求裡不存在：%v", missing)
	}
}

// 後台的申請帶逐項問題與內部備註（文件仍只有位置，沒有 asset ID）；賣家端的 KYCRejection 沒有內部備註（型別上就沒有這個欄位）。
func TestAdminKYCCarriesIssuesAndInternalNote(t *testing.T) {
	a := adminApplication()
	note := "疑似翻拍"
	a.Documents = []model.KYCDocument{{Slot: model.SlotBankBook, AssetID: "asset-bank"}}
	a.OutstandingIssues = []model.KYCIssue{{Field: "bank_book", DocumentAssetID: "asset-bank", Code: model.IssueUnreadable, Message: "反光"}}
	a.Reviews = []model.KYCReview{{
		ID: uuid.MustParse("0192f1c0-0000-7000-8000-0000000000aa"), Action: model.ReviewReject, Reason: &note, InternalNote: &note,
		Issues: []model.KYCIssue{{Field: "bank_book", DocumentAssetID: "asset-bank", Code: model.IssueUnreadable, Message: "反光"}},
	}}
	out := toAdminKYC(a)
	want := &pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_BANK_BOOK, DocumentAssetId: "asset-bank", Code: pb.KYCIssueCode_KYC_ISSUE_CODE_UNREADABLE, Message: "反光"}
	if got := out.GetOutstandingIssues(); len(got) != 1 || !proto.Equal(got[0], want) {
		t.Fatalf("outstanding_issues=%v", got)
	}
	review := out.GetReviews()[0]
	if review.GetInternalNote() != note || len(review.GetIssues()) != 1 || !proto.Equal(review.GetIssues()[0], want) {
		t.Fatalf("review=%v", review)
	}
}

// 契約的 KYCField 與模組的欄位路徑一對一：每個模組的欄位都對得到，每個非 UNSPECIFIED 的契約值也都是模組的欄位。
func TestKYCFieldEnumMatchesTheModuleNames(t *testing.T) {
	for _, name := range model.KYCFieldNames() {
		if got, ok := kycFieldNames[kycFieldPB(name)]; !ok || got != name {
			t.Errorf("%s ↔ %v", name, kycFieldPB(name))
		}
	}
	values := pb.KYCField(0).Descriptor().Values()
	for i := range values.Len() {
		if n := pb.KYCField(values.Get(i).Number()); n != pb.KYCField_KYC_FIELD_UNSPECIFIED {
			if _, ok := kycFieldNames[n]; !ok {
				t.Errorf("%v 不是模組的欄位", n)
			}
		}
	}
}

func TestAdminKYCEnumsMatchTheModule(t *testing.T) {
	if got := rpc.EnumMismatches[adminpb.KYCSellerType](model.KYCSellerTypeValues()); len(got) != 0 {
		t.Fatalf("KYCSellerType：%v", got)
	}
	// 後台契約沒有證件位置的列舉：每個位置都對到一個文件類的欄位（KYCField），而且是 KYCIssue 能指到文件的那四個欄位。
	documentFields := map[pb.KYCField]bool{}
	for _, slot := range model.KYCDocumentSlotValues() {
		field := documentFieldPB(slot)
		if field == pb.KYCField_KYC_FIELD_UNSPECIFIED || documentFields[field] {
			t.Fatalf("證件位置 %s 沒有對到獨一的欄位：%v", slot, field)
		}
		documentFields[field] = true
	}
	if len(documentFields) != 4 {
		t.Fatalf("文件類的欄位：%v", documentFields)
	}
	if got := rpc.EnumMismatches[adminpb.KYCReviewAction](model.ReviewActionValues()); len(got) != 0 {
		t.Fatalf("KYCReviewAction：%v", got)
	}
	if got := rpc.EnumMismatches[pb.KYCIssueCode](model.KYCIssueCodeValues()); len(got) != 0 {
		t.Fatalf("KYCIssueCode：%v", got)
	}
	if got := rpc.EnumMismatches[adminpb.PayUniObservationSource](model.PayUniSourceValues()); len(got) != 0 {
		t.Fatalf("PayUniObservationSource：%v", got)
	}
}

func TestRevealKYCSensitiveUsesTheCallingAdminReturnsViewURIsAndAssetIDs(t *testing.T) {
	appID := uuid.NewV7()
	expires := time.Date(2026, 10, 1, 4, 15, 0, 0, time.UTC)
	m := &adminKYCModule{revealed: model.KYCSensitive{
		IDCardNo: ptr(plainID), BankAccount: ptr(plainBank), GrantID: uuid.NewV7(), ExpiresAt: expires,
		Documents: []model.KYCDocumentLocator{{Slot: model.SlotIDCardFront, AssetID: "asset-front"}, {Slot: model.SlotCompanyDocument, Position: 2, AssetID: "asset-co-3"}},
	}}
	resp, err := newTestAdmin(m).RevealKYCSensitive(adminCtx(42), connect.NewRequest(&adminpb.RevealKYCSensitiveRequest{Id: appID.String(), Reason: "核對證件"}))
	if err != nil {
		t.Fatal(err)
	}
	if m.reveal.AdminID != 42 || m.reveal.ApplicationID != appID || m.reveal.Reason != "核對證件" {
		t.Fatalf("reveal=%+v", m.reveal)
	}
	r := resp.Msg
	if r.GetIdCardNo() != plainID || r.GetRepIdCardNo() != "" || r.GetBankAccount() != plainBank || !r.GetExpireTime().AsTime().Equal(expires) || len(r.GetDocuments()) != 2 {
		t.Fatalf("response=%+v", r)
	}
	wantFront := "/api/files/admin/seller/kyc_applications/" + appID.String() + "/documents/id_card_front/0"
	wantCompany := "/api/files/admin/seller/kyc_applications/" + appID.String() + "/documents/company_document/2"
	if r.GetDocuments()[0].GetViewUri() != wantFront || r.GetDocuments()[1].GetViewUri() != wantCompany || r.GetDocuments()[1].GetPosition() != 2 {
		t.Fatalf("documents=%+v", r.GetDocuments())
	}
	// asset_id 只在顯示全文的回應，退回時管理員用它指定某一張文件。
	if r.GetDocuments()[0].GetAssetId() != "asset-front" || r.GetDocuments()[1].GetAssetId() != "asset-co-3" {
		t.Fatalf("asset ids=%+v", r.GetDocuments())
	}
	// view_uri 的格式與端點的路由一致：路由能把它解回同一個 (申請, 位置)。
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, wantCompany, nil)
	mux := http.NewServeMux()
	var parsed model.ReadKYCDocumentInput
	mux.HandleFunc(DocumentRoute, func(_ http.ResponseWriter, r *http.Request) { parsed, _ = documentRequest(r) })
	mux.ServeHTTP(httptest.NewRecorder(), req)
	if parsed.ApplicationID != appID || parsed.Slot != model.SlotCompanyDocument || parsed.Position != 2 {
		t.Fatalf("parsed=%+v", parsed)
	}
	for name, req := range map[string]*adminpb.RevealKYCSensitiveRequest{"no id": {Reason: "x"}, "bad id": {Id: "x", Reason: "x"}} {
		_, err := newTestAdmin(m).RevealKYCSensitive(adminCtx(42), connect.NewRequest(req))
		if err == nil {
			t.Fatal(name)
		}
	}
	if _, err := newTestAdmin(m).RevealKYCSensitive(context.Background(), connect.NewRequest(&adminpb.RevealKYCSensitiveRequest{Id: appID.String(), Reason: "x"})); !errors.Is(err, errNoPrincipal) {
		t.Fatalf("no principal: %v", err)
	}
}

func TestListKYCApplicationsMapsFiltersAndBindsThePageTokenToThem(t *testing.T) {
	next := &model.KYCCursor{SubmittedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ApplicationID: uuid.NewV7()}
	m := &adminKYCModule{listPage: model.KYCPage{Items: []model.KYCApplication{adminApplication().KYCApplication}, Next: next}}
	h := newTestAdmin(m)
	if _, err := h.ListKYCApplications(adminCtx(1), connect.NewRequest(&adminpb.ListKYCApplicationsRequest{})); err != nil {
		t.Fatal(err)
	}
	if m.list.Status != nil || m.list.Query != "" || m.list.After != nil {
		t.Fatalf("UNSPECIFIED 不是篩選值：%+v", m.list)
	}
	first, err := h.ListKYCApplications(adminCtx(1), connect.NewRequest(&adminpb.ListKYCApplicationsRequest{
		Status: pb.KYCStatus_KYC_STATUS_SUBMITTED, Query: "範例", PageSize: 1,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if m.list.Status == nil || *m.list.Status != model.KYCStatusSubmitted || m.list.Query != "範例" || m.list.PageSize != 1 {
		t.Fatalf("list=%+v", m.list)
	}
	token := first.Msg.GetNextPageToken()
	if token == "" || len(first.Msg.GetApplications()) != 1 || first.Msg.GetApplications()[0].GetSellerType() != adminpb.KYCSellerType_KYC_SELLER_TYPE_INDIVIDUAL {
		t.Fatalf("response=%+v", first.Msg)
	}
	if _, err := h.ListKYCApplications(adminCtx(1), connect.NewRequest(&adminpb.ListKYCApplicationsRequest{Status: pb.KYCStatus_KYC_STATUS_SUBMITTED, Query: "範例", PageToken: token})); err != nil {
		t.Fatal(err)
	}
	if m.list.After == nil || *m.list.After != *next {
		t.Fatalf("after=%+v", m.list.After)
	}
	for name, req := range map[string]*adminpb.ListKYCApplicationsRequest{
		"other query": {Status: pb.KYCStatus_KYC_STATUS_SUBMITTED, Query: "別的", PageToken: token}, "other status": {Status: pb.KYCStatus_KYC_STATUS_APPROVED, Query: "範例", PageToken: token},
		"no status": {Query: "範例", PageToken: token},
	} {
		_, err := h.ListKYCApplications(adminCtx(1), connect.NewRequest(req))
		if err == nil {
			t.Fatalf("%s：token 不該跨篩選使用", name)
		}
		wantInvalidArgument(t, err)
	}
	scope := sellerv1alpha1connect.SellerAdminServiceListKYCApplicationsProcedure + "\x00" + pb.KYCStatus_KYC_STATUS_UNSPECIFIED.String() + "\x00"
	_, err = h.ListKYCApplications(adminCtx(1), connect.NewRequest(&adminpb.ListKYCApplicationsRequest{PageToken: rpc.TokenFor(scope, &model.KYCCursor{})}))
	wantInvalidArgument(t, err)
}

func TestPayUniAdminMethodsUseTheStoreAndTheCallingAdmin(t *testing.T) {
	reg := model.PayUniRegistration{
		StoreID: 11, MerID: "HAWK2000001", ApplicationID: uuid.NewV7(), BindStatus: model.BindStatusFailed, BindErrorCode: ptr("AGENT04003"), BindAttempts: 2,
		SellerType: model.KYCIndividual, Status: model.PayUniStatus{MerStatus: ptr("2"), Outcome: model.PayUniOutcomeRejected},
	}
	view := model.AdminPayUniRegistration{
		PayUniRegistration: reg, FlowStatus: model.FlowBindFailed,
		RebindUnavailable: []model.StoreActionUnavailableReason{model.StoreReasonStoreClosed},
	}
	m := &adminKYCModule{reg: view}
	h := newTestAdmin(m)
	got, err := h.GetPayUniRegistration(adminCtx(1), connect.NewRequest(&adminpb.GetPayUniRegistrationRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	r := got.Msg.GetRegistration()
	if r.GetMerId() != "HAWK2000001" || r.GetBindAttempts() != 2 || r.GetApplicationId() != reg.ApplicationID.String() || r.GetStatusCodes().GetMerStatus() != "2" ||
		r.GetBindErrorCode() != "AGENT04003" || r.GetBindStatus() != pb.PayUniBindStatus_PAY_UNI_BIND_STATUS_FAILED {
		t.Fatalf("registration=%+v", r)
	}
	// 階段與重新綁定的可用性由模組推導、handler 原樣轉出；原因的句子是 StoreActionUnavailableText（與 RebindPayUni 被拒絕時的錯誤同一個函式）。
	if r.GetFlowStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_BIND_FAILED || r.GetCanRebind() || len(r.GetRebindUnavailabilities()) != 1 ||
		r.GetRebindUnavailabilities()[0].GetReason() != adminpb.StoreActionUnavailableReason_STORE_ACTION_UNAVAILABLE_REASON_STORE_CLOSED ||
		r.GetRebindUnavailabilities()[0].GetMessage().GetMessage() != "店家已關閉，不能重新綁定 PayUni。" {
		t.Fatalf("rebind=%v flow=%v", r.GetRebindUnavailabilities(), r.GetFlowStatus())
	}
	if _, err := h.RebindPayUni(adminCtx(42), connect.NewRequest(&adminpb.RebindPayUniRequest{StoreId: "11"})); err != nil {
		t.Fatal(err)
	}
	if m.rebind.AdminID != 42 || m.rebind.StoreID != 11 {
		t.Fatalf("rebind=%+v", m.rebind)
	}
	if _, err := h.RefreshPayUniStatus(adminCtx(42), connect.NewRequest(&adminpb.RefreshPayUniStatusRequest{StoreId: "11"})); err != nil || m.refreshed != 11 {
		t.Fatalf("refreshed=%d err=%v", m.refreshed, err)
	}
	m.calls = 0
	for name, call := range map[string]func() error{
		"get": func() error {
			_, err := h.GetPayUniRegistration(adminCtx(1), connect.NewRequest(&adminpb.GetPayUniRegistrationRequest{StoreId: "x"}))
			return err
		},
		"rebind": func() error {
			_, err := h.RebindPayUni(adminCtx(1), connect.NewRequest(&adminpb.RebindPayUniRequest{StoreId: "0"}))
			return err
		},
		"refresh": func() error {
			_, err := h.RefreshPayUniStatus(adminCtx(1), connect.NewRequest(&adminpb.RefreshPayUniStatusRequest{}))
			return err
		},
		"list": func() error {
			_, err := h.ListPayUniObservations(adminCtx(1), connect.NewRequest(&adminpb.ListPayUniObservationsRequest{StoreId: "-1"}))
			return err
		},
	} {
		wantInvalidArgument(t, call())
		if m.calls != 0 {
			t.Fatalf("%s：模組不該被呼叫", name)
		}
	}
	if _, err := h.RebindPayUni(context.Background(), connect.NewRequest(&adminpb.RebindPayUniRequest{StoreId: "11"})); !errors.Is(err, errNoPrincipal) {
		t.Fatalf("no principal: %v", err)
	}
}

func TestListPayUniObservationsPagesWithAStoreBoundToken(t *testing.T) {
	cursor := &model.PayUniObservationCursor{ExecutedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ObservationID: uuid.NewV7()}
	m := &adminKYCModule{obsPage: model.PayUniObservationPage{Items: []model.PayUniObservation{{ID: uuid.NewV7(), Source: model.PayUniSourceWebhook, EventKey: ptr("k"), MerStatus: ptr("1"), ExecutedAt: time.Now(), ReceivedAt: time.Now()}}, Next: cursor}}
	h := newTestAdmin(m)
	first, err := h.ListPayUniObservations(adminCtx(1), connect.NewRequest(&adminpb.ListPayUniObservationsRequest{StoreId: "11", PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	o := first.Msg.GetObservations()[0]
	if o.GetSource() != adminpb.PayUniObservationSource_PAY_UNI_OBSERVATION_SOURCE_WEBHOOK || o.GetEventKey() != "k" || o.GetStatusCodes().GetMerStatus() != "1" || m.obsIn.StoreID != 11 || m.obsIn.PageSize != 1 {
		t.Fatalf("o=%+v in=%+v", o, m.obsIn)
	}
	token := first.Msg.GetNextPageToken()
	if _, err := h.ListPayUniObservations(adminCtx(1), connect.NewRequest(&adminpb.ListPayUniObservationsRequest{StoreId: "11", PageToken: token})); err != nil {
		t.Fatal(err)
	}
	if m.obsIn.After == nil || *m.obsIn.After != *cursor {
		t.Fatalf("after=%+v", m.obsIn.After)
	}
	_, err = h.ListPayUniObservations(adminCtx(1), connect.NewRequest(&adminpb.ListPayUniObservationsRequest{StoreId: "12", PageToken: token}))
	wantInvalidArgument(t, err)
	scope := rpc.PageScope(sellerv1alpha1connect.SellerAdminServiceListPayUniObservationsProcedure, 11)
	_, err = h.ListPayUniObservations(adminCtx(1), connect.NewRequest(&adminpb.ListPayUniObservationsRequest{StoreId: "11", PageToken: rpc.TokenFor(scope, &model.PayUniObservationCursor{})}))
	wantInvalidArgument(t, err)
}

// 後台的 KYC 訊息不得有只存在於前台、帶明文的欄位，而且全部是 JSON 可序列化（沿用 protojson 的輸出沒有 INPUT_ONLY）。
func TestAdminKYCMessagesCarryNoInputOnlyFields(t *testing.T) {
	for _, m := range []proto.Message{&adminpb.KYCApplication{}, &adminpb.KYCApplicationSummary{}, &adminpb.RevealKYCSensitiveResponse{}} {
		var out map[string]any
		raw, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(m)
		if err != nil || json.Unmarshal(raw, &out) != nil {
			t.Fatal(err)
		}
	}
}

type docModule struct {
	in      model.ReadKYCDocumentInput
	content model.FileContent
	err     error
	calls   int
}

func (d *docModule) ReadKYCDocument(_ context.Context, in model.ReadKYCDocumentInput) (model.FileContent, error) {
	d.in, d.calls = in, d.calls+1
	return d.content, d.err
}

func serveDocument(t *testing.T, m *docModule, ctx context.Context, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(DocumentRoute, NewAdminDocuments(m, nil))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil))
	return rec
}

func TestDocumentEndpointAuthorizesAdminsAndMapsErrorsWithoutLeaking(t *testing.T) {
	app := uuid.NewV7().String()
	path := "/api/files/admin/seller/kyc_applications/" + app + "/documents/id_card_front/0"
	customer := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 5})
	for name, c := range map[string]struct {
		ctx  context.Context
		want int
	}{"visitor": {context.Background(), 401}, "customer": {customer, 403}} {
		m := &docModule{}
		rec := serveDocument(t, m, c.ctx, path)
		if rec.Code != c.want || m.calls != 0 || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s：code=%d calls=%d", name, rec.Code, m.calls)
		}
	}
	m := &docModule{content: model.FileContent{Data: []byte("IMG"), ContentType: "image/png"}}
	rec := serveDocument(t, m, adminCtx(42), path)
	if rec.Code != 200 || rec.Body.String() != "IMG" || m.in.AdminID != 42 || m.in.Slot != model.SlotIDCardFront || m.in.Position != 0 || m.in.ApplicationID.String() != app {
		t.Fatalf("code=%d in=%+v", rec.Code, m.in)
	}
	for k, v := range map[string]string{
		"Content-Type": "image/png", "Cache-Control": "private, no-store", "Content-Disposition": "inline", "X-Content-Type-Options": "nosniff",
	} {
		if rec.Header().Get(k) != v {
			t.Fatalf("%s = %q，要 %q", k, rec.Header().Get(k), v)
		}
	}
	for name, c := range map[string]struct {
		err  error
		want int
	}{
		"no grant": {apperr.PermissionDenied("沒有讀取這份證件的授權"), 403}, "purged": {apperr.NotFound("找不到這張證件"), 404},
		"media down": {errors.New("secret internal detail"), 500}, "unavailable": {apperr.Unavailable("x"), 503},
	} {
		rec := serveDocument(t, &docModule{err: c.err}, adminCtx(42), path)
		if rec.Code != c.want || strings.Contains(rec.Body.String(), "secret") || strings.Contains(rec.Body.String(), "授權") || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s：code=%d body=%q", name, rec.Code, rec.Body.String())
		}
	}
	// 位置格式不對：模組不被呼叫。
	for _, bad := range []string{"/documents/selfie/0", "/documents/id_card_front/x", "/documents/id_card_front/-1"} {
		m := &docModule{}
		if rec := serveDocument(t, m, adminCtx(42), "/api/files/admin/seller/kyc_applications/"+app+bad); rec.Code != 404 || m.calls != 0 {
			t.Fatalf("%s：code=%d calls=%d", bad, rec.Code, m.calls)
		}
	}
	if rec := serveDocument(t, &docModule{}, adminCtx(42), "/api/files/admin/seller/kyc_applications/not-a-uuid/documents/id_card_front/0"); rec.Code != 404 {
		t.Fatalf("bad application id: %d", rec.Code)
	}
}

// 媒體給的型別不是圖片或 PDF（例如 text/html、svg）時，端點以不透明的二進位送出，瀏覽器不會把上傳的內容當網頁執行。
func TestDocumentEndpointNeverServesActiveContentTypes(t *testing.T) {
	path := "/api/files/admin/seller/kyc_applications/" + uuid.NewV7().String() + "/documents/bank_book/0"
	for ct, want := range map[string]string{
		"image/jpeg": "image/jpeg", "IMAGE/PNG; charset=x": "image/png", "application/pdf": "application/pdf",
		"text/html": "application/octet-stream", "image/svg+xml": "application/octet-stream", "": "application/octet-stream",
	} {
		rec := serveDocument(t, &docModule{content: model.FileContent{Data: []byte("x"), ContentType: ct}}, adminCtx(1), path)
		if got := rec.Header().Get("Content-Type"); got != want {
			t.Errorf("%q → %q，要 %q", ct, got, want)
		}
	}
}

// 稽核紀錄各自分頁：token 綁申請與方法，換了申請就不能沿用；位置不完整被拒絕。
func TestAuditListsPageWithAToken(t *testing.T) {
	now := time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	cursor := &model.KYCAuditCursor{At: now, ID: uuid.NewV7()}
	m := &adminKYCModule{
		revealPage: model.KYCRevealPage{Items: []model.KYCReveal{{ID: uuid.NewV7(), AdminID: 9, Reason: "核對", RevealedAt: now, ExpiresAt: now.Add(model.RevealGrantTTL)}}, Next: cursor},
		accessPage: model.KYCDocumentAccessPage{Items: []model.KYCDocumentAccess{{ID: uuid.NewV7(), RevealID: uuid.NewV7(), AdminID: 9, Slot: model.SlotBankBook, AccessedAt: now}}, Next: cursor},
	}
	h := newTestAdmin(m)
	app, other := uuid.NewV7(), uuid.NewV7()
	reveals, err := h.ListKYCReveals(adminCtx(1), connect.NewRequest(&adminpb.ListKYCRevealsRequest{Id: app.String(), PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	r := reveals.Msg.GetReveals()[0]
	if r.GetAdminId() != "9" || r.GetExpireTime() == nil || m.auditIn.ApplicationID != app || m.auditIn.PageSize != 1 {
		t.Fatalf("reveal=%+v in=%+v", r, m.auditIn)
	}
	accesses, err := h.ListKYCDocumentAccesses(adminCtx(1), connect.NewRequest(&adminpb.ListKYCDocumentAccessesRequest{Id: app.String()}))
	if err != nil {
		t.Fatal(err)
	}
	if a := accesses.Msg.GetDocumentAccesses()[0]; a.GetField() != pb.KYCField_KYC_FIELD_BANK_BOOK || a.GetAdminId() != "9" {
		t.Fatalf("access=%+v", a)
	}
	token := reveals.Msg.GetNextPageToken()
	if _, err := h.ListKYCReveals(adminCtx(1), connect.NewRequest(&adminpb.ListKYCRevealsRequest{Id: app.String(), PageToken: token})); err != nil || m.auditIn.After == nil || *m.auditIn.After != *cursor {
		t.Fatalf("after=%+v err=%v", m.auditIn.After, err)
	}
	_, err = h.ListKYCReveals(adminCtx(1), connect.NewRequest(&adminpb.ListKYCRevealsRequest{Id: other.String(), PageToken: token}))
	wantInvalidArgument(t, err)
	// 別的方法的 token 也不能用。
	_, err = h.ListKYCDocumentAccesses(adminCtx(1), connect.NewRequest(&adminpb.ListKYCDocumentAccessesRequest{Id: app.String(), PageToken: token}))
	wantInvalidArgument(t, err)
	scope := auditScope(sellerv1alpha1connect.SellerAdminServiceListKYCRevealsProcedure, app)
	_, err = h.ListKYCReveals(adminCtx(1), connect.NewRequest(&adminpb.ListKYCRevealsRequest{Id: app.String(), PageToken: rpc.TokenFor(scope, &model.KYCAuditCursor{})}))
	wantInvalidArgument(t, err)
	_, err = h.ListKYCReveals(adminCtx(1), connect.NewRequest(&adminpb.ListKYCRevealsRequest{Id: "x"}))
	wantInvalidArgument(t, err)
}

// 證件讀取的錯誤各有自己的 HTTP 結果：引用還沒同步完是 503 加 Retry-After（warn）；media 不是機敏資產、或同步過了卻說沒有，是資料不一致，
// 404 加 error log；沒有授權 403；一般找不到 404 不記 error。
func TestDocumentEndpointDistinguishesUnsyncedFromMissing(t *testing.T) {
	path := "/api/files/admin/seller/kyc_applications/" + uuid.NewV7().String() + "/documents/id_card_front/0"
	for name, c := range map[string]struct {
		err        error
		code       int
		retryAfter bool
		level      string
	}{
		"not synced yet":     {fmt.Errorf("x：%w", model.ErrFileNotSynced), 503, true, "WARN"},
		"synced but missing": {fmt.Errorf("x：%w", model.ErrFileMissing), 404, false, "ERROR"},
		"not secure":         {fmt.Errorf("x：%w", media.ErrNotSecure), 404, false, "ERROR"},
		"plain not found":    {apperr.NotFound("找不到這張證件"), 404, false, ""},
		"no grant":           {apperr.PermissionDenied("x"), 403, false, ""},
	} {
		var logs strings.Builder
		mux := http.NewServeMux()
		mux.Handle(DocumentRoute, NewAdminDocuments(&docModule{err: c.err}, slog.New(slog.NewTextHandler(&logs, nil))))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequestWithContext(adminCtx(42), http.MethodGet, path, nil))
		if rec.Code != c.code || (rec.Header().Get("Retry-After") != "") != c.retryAfter {
			t.Fatalf("%s：code=%d retry-after=%q", name, rec.Code, rec.Header().Get("Retry-After"))
		}
		if c.level == "" && logs.Len() != 0 || c.level != "" && !strings.Contains(logs.String(), "level="+c.level) {
			t.Fatalf("%s：log=%q，要 %q", name, logs.String(), c.level)
		}
	}
}

// 後台 etag 是申請的版本加顯示的登記資料查詢的序號：核准與退回把兩個部分都交給模組比對，回應的 etag 用目前顯示的序號。
func TestAdminEtagCarriesTheVersionAndTheDisplayedCheckSeq(t *testing.T) {
	m := &adminKYCModule{got: adminApplication()}
	h := newTestAdmin(m)
	id := m.got.ID.String()
	if _, err := h.ApproveKYC(adminCtx(42), connect.NewRequest(&adminpb.ApproveKYCRequest{Id: id, Etag: "3.2"})); err != nil {
		t.Fatal(err)
	}
	if m.approve.ExpectedVersion != 3 || m.approve.ExpectedCheckSeq != 2 {
		t.Fatalf("approve=%+v", m.approve)
	}
	if _, err := h.RejectKYC(adminCtx(42), connect.NewRequest(&adminpb.RejectKYCRequest{Id: id, Etag: "7.11", Reason: "x"})); err != nil {
		t.Fatal(err)
	}
	if m.reject.ExpectedVersion != 7 || m.reject.ExpectedCheckSeq != 11 {
		t.Fatalf("reject=%+v", m.reject)
	}
	for _, etag := range []string{"x", "3", "3.", "3.1.1"} {
		_, err := h.RejectKYC(adminCtx(42), connect.NewRequest(&adminpb.RejectKYCRequest{Id: id, Etag: etag, Reason: "x"}))
		wantInvalidArgument(t, err)
	}
	m.got.RegistrationCheck = &model.RegistrationCheck{Seq: 4, Outcome: model.RegistrationNotFound}
	resp, err := h.GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Msg.GetApplication().GetEtag(); got != "3.4" {
		t.Fatalf("etag=%q", got)
	}
}

// 登記資料查詢的呈現：FOUND 帶登記資料（拆得開的地址有 PostalAddress、拆不開只有原文）與比對欄位；其他結果沒有 registration；
// 審核紀錄帶出依據的序號與時間，沒有引用時不設。
func TestGetKYCApplicationShowsTheRegistrationCheck(t *testing.T) {
	m := &adminKYCModule{got: adminApplication()}
	queried := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	requested := queried.Add(time.Hour)
	deadline := requested.Add(100 * time.Minute)
	m.got.PendingRegistrationCheck = &model.RegistrationCheck{Seq: 3, Outcome: model.RegistrationPending, RequestedAt: requested, Deadline: &deadline}
	m.got.RegistrationCheck = &model.RegistrationCheck{
		Seq: 2, Outcome: model.RegistrationFound, QueriedAt: &queried,
		Registration: &model.Registration{
			TaxID: "33330333", Name: "某某有限公司", Address: "臺北市中正區某路1號", TownCode: "63000050", AddressDetail: "某路1號",
			SetupDate: time.Date(2015, 3, 4, 0, 0, 0, 0, time.UTC), OrganizationType: "有限公司", Status: model.RegistrationStatusTerminated, StatusText: "廢止",
			SuspendedFrom: time.Date(1993, 11, 1, 0, 0, 0, 0, time.UTC), SuspendedUntil: time.Date(1994, 3, 31, 0, 0, 0, 0, time.UTC),
		},
		MismatchedFields:   []string{"business.company_name"},
		UnverifiableFields: []string{"business.company_register_date", "business.company_address"},
	}
	m.got.Reviews[0].RegistrationCheckSeq, m.got.Reviews[0].RegistrationCheckQueriedAt = 1, &queried
	resp, err := newTestAdmin(m).GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: m.got.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	a := resp.Msg.GetApplication()
	c := a.GetRegistrationCheck()
	r := c.GetRegistration()
	if p := a.GetPendingRegistrationCheck(); p.GetSeq() != 3 || p.GetOutcome() != adminpb.KYCRegistrationOutcome_KYC_REGISTRATION_OUTCOME_UNSPECIFIED ||
		!p.GetRequestTime().AsTime().Equal(requested) || !p.GetDeadlineTime().AsTime().Equal(deadline) || p.GetRegistration() != nil {
		t.Fatalf("pending=%+v", p)
	}
	if c.GetDeadlineTime() != nil {
		t.Fatalf("已完成的查詢不帶期限：%+v", c)
	}
	if c.GetSeq() != 2 || c.GetOutcome() != adminpb.KYCRegistrationOutcome_KYC_REGISTRATION_OUTCOME_FOUND ||
		!c.GetQueryTime().AsTime().Equal(queried) || r.GetTaxId() != "33330333" || r.GetName() != "某某有限公司" || r.GetAddress() != "臺北市中正區某路1號" ||
		r.GetPostalAddress().GetTownCode() != "63000050" || r.GetPostalAddress().GetAddressDetail() != "某路1號" ||
		r.GetSetupDate().GetYear() != 2015 || r.GetSetupDate().GetMonth() != 3 || r.GetSetupDate().GetDay() != 4 ||
		r.GetOrganizationType() != "有限公司" || r.GetStatus() != adminpb.KYCRegistrationStatus_KYC_REGISTRATION_STATUS_TERMINATED || r.GetStatusText() != "廢止" ||
		r.GetSuspendStartDate().GetYear() != 1993 || r.GetSuspendEndDate().GetDay() != 31 {
		t.Fatalf("check=%+v", c)
	}
	if len(c.GetMismatchedFields()) != 1 || c.GetMismatchedFields()[0] != pb.KYCField_KYC_FIELD_BUSINESS_COMPANY_NAME ||
		len(c.GetUnverifiableFields()) != 2 || c.GetUnverifiableFields()[1] != pb.KYCField_KYC_FIELD_BUSINESS_COMPANY_ADDRESS {
		t.Fatalf("fields=%v %v", c.GetMismatchedFields(), c.GetUnverifiableFields())
	}
	if a.GetReviews()[0].GetRegistrationCheckSeq() != 1 || !a.GetReviews()[0].GetRegistrationCheckQueryTime().AsTime().Equal(queried) ||
		a.GetReviews()[1].GetRegistrationCheckSeq() != 0 || a.GetReviews()[1].GetRegistrationCheckQueryTime() != nil {
		t.Fatalf("reviews=%+v", a.GetReviews())
	}
	// 拆不開的地址只有原文；查無沒有 registration；沒有已完成的查詢時不設。
	m.got.RegistrationCheck.Registration.TownCode, m.got.RegistrationCheck.Registration.AddressDetail = "", ""
	resp, err = newTestAdmin(m).GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: m.got.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Msg.GetApplication().GetRegistrationCheck().GetRegistration(); got.GetPostalAddress() != nil || got.GetAddress() == "" {
		t.Fatalf("registration=%+v", got)
	}
	m.got.RegistrationCheck = &model.RegistrationCheck{Seq: 3, Outcome: model.RegistrationUnavailable, QueriedAt: &queried}
	resp, _ = newTestAdmin(m).GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: m.got.ID.String()}))
	if c := resp.Msg.GetApplication().GetRegistrationCheck(); c.GetRegistration() != nil || c.GetOutcome() != adminpb.KYCRegistrationOutcome_KYC_REGISTRATION_OUTCOME_UNAVAILABLE {
		t.Fatalf("check=%+v", c)
	}
	m.got.RegistrationCheck, m.got.PendingRegistrationCheck = nil, nil
	resp, _ = newTestAdmin(m).GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: m.got.ID.String()}))
	if resp.Msg.GetApplication().GetRegistrationCheck() != nil || resp.Msg.GetApplication().GetPendingRegistrationCheck() != nil {
		t.Fatalf("application=%+v", resp.Msg.GetApplication())
	}
}

func TestRequeryKYCRegistrationUsesTheCallingAdmin(t *testing.T) {
	m := &adminKYCModule{got: adminApplication()}
	h := newTestAdmin(m)
	requested := time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
	deadline := requested.Add(100 * time.Minute)
	m.requeried = model.RegistrationCheck{Seq: 4, Outcome: model.RegistrationPending, RequestedAt: requested, Deadline: &deadline}
	resp, err := h.RequeryKYCRegistration(adminCtx(42), connect.NewRequest(&adminpb.RequeryKYCRegistrationRequest{Id: m.got.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	if m.requery.AdminID != 42 || m.requery.ApplicationID != m.got.ID {
		t.Fatalf("requery=%+v", m.requery)
	}
	// 回應是這筆進行中的查詢（帶期限），不是空的。
	if c := resp.Msg.GetRegistrationCheck(); c.GetSeq() != 4 || !c.GetRequestTime().AsTime().Equal(requested) || !c.GetDeadlineTime().AsTime().Equal(deadline) {
		t.Fatalf("response=%+v", c)
	}
	m.calls = 0
	_, err = h.RequeryKYCRegistration(adminCtx(42), connect.NewRequest(&adminpb.RequeryKYCRegistrationRequest{Id: "x"}))
	wantInvalidArgument(t, err)
	if _, err := h.RequeryKYCRegistration(context.Background(), connect.NewRequest(&adminpb.RequeryKYCRegistrationRequest{Id: m.got.ID.String()})); !errors.Is(err, errNoPrincipal) {
		t.Fatalf("no principal: %v", err)
	}
	if m.calls != 0 {
		t.Fatal("模組不該被呼叫")
	}
}

// 登記資料的列舉與契約一一對應；現況與結果的值域各一份。
func TestRegistrationEnumsMatchTheContract(t *testing.T) {
	// 契約只有顯示得出來的結果：CANCELLED 是不顯示的終態，PENDING 不是結果（查詢中用 pending_registration_check）；顯示的查詢條件排除它們。
	displayed := slices.DeleteFunc(model.RegistrationOutcomeValues(), func(o model.RegistrationOutcome) bool {
		return o == model.RegistrationCancelled || o == model.RegistrationPending
	})
	for name, mismatches := range map[string][]string{
		"KYCRegistrationOutcome": rpc.EnumMismatches[adminpb.KYCRegistrationOutcome](displayed),
		"KYCRegistrationStatus":  rpc.EnumMismatches[adminpb.KYCRegistrationStatus](model.RegistrationStatusValues()),
	} {
		if len(mismatches) != 0 {
			t.Errorf("%s：%v", name, mismatches)
		}
	}
}

// 可以指出問題的欄位與「查詢當天是否在停業期間」都由伺服器算好：後台顯示端只讀，不自己寫型態對欄位的表或比對日期。
func TestGetKYCApplicationCarriesServerComputedRules(t *testing.T) {
	m := &adminKYCModule{got: adminApplication()}
	queried := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	m.got.IssuableFields = model.IssuableFields(model.KYCCompany)
	m.got.RegistrationCheck = &model.RegistrationCheck{
		Seq: 1, Outcome: model.RegistrationFound, QueriedAt: &queried,
		Registration: &model.Registration{
			TaxID: "33330333", Name: "某某有限公司", Address: "臺北市中正區某路1號", Status: model.RegistrationStatusActive, StatusText: "核准設立",
			SuspendedFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), SuspendedUntil: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
		},
	}
	resp, err := newTestAdmin(m).GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: m.got.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	a := resp.Msg.GetApplication()
	var got []string
	for _, f := range a.GetIssuableFields() {
		got = append(got, f.String())
	}
	if len(got) != len(model.IssuableFields(model.KYCCompany)) || got[0] != "KYC_FIELD_REAL_NAME" {
		t.Fatalf("issuable_fields=%v", got)
	}
	for _, f := range a.GetIssuableFields() {
		if f == pb.KYCField_KYC_FIELD_INDIVIDUAL_ID_CARD_NO || f == pb.KYCField_KYC_FIELD_UNSPECIFIED {
			t.Fatalf("企業申請的欄位不該有 %v", f)
		}
	}
	if !a.GetRegistrationCheck().GetRegistration().GetSuspendedAtQuery() {
		t.Fatal("查詢當天（台北 10/7）在停業期間內")
	}
	later := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)
	m.got.RegistrationCheck.QueriedAt = &later
	resp, _ = newTestAdmin(m).GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: m.got.ID.String()}))
	if resp.Msg.GetApplication().GetRegistrationCheck().GetRegistration().GetSuspendedAtQuery() {
		t.Fatal("期間結束之後的查詢不在停業期間內")
	}
}

// 三組可用性由模組推導，handler 原樣轉出 can_* 與原因（句子是 StoreActionUnavailableText，與方法被拒絕時的錯誤同一個函式）；
// 空清單就是可以做；idempotent 的原因沒有句子。
func TestGetKYCApplicationCarriesTheActionAvailabilities(t *testing.T) {
	m := &adminKYCModule{got: adminApplication()}
	h := newTestAdmin(m)
	read := func() *adminpb.KYCApplication {
		t.Helper()
		resp, err := h.GetKYCApplication(adminCtx(1), connect.NewRequest(&adminpb.GetKYCApplicationRequest{Id: m.got.ID.String()}))
		if err != nil {
			t.Fatal(err)
		}
		return resp.Msg.GetApplication()
	}
	a := read()
	if !a.GetCanApprove() || !a.GetCanReject() || !a.GetCanRequeryRegistration() || len(a.GetApproveUnavailabilities()) != 0 {
		t.Fatalf("沒有原因就是可以：%v", a)
	}
	m.got.ApproveUnavailable = []model.StoreActionUnavailableReason{model.StoreReasonStoreClosed}
	m.got.RejectUnavailable = []model.StoreActionUnavailableReason{model.StoreReasonStoreClosed}
	m.got.RequeryUnavailable = []model.StoreActionUnavailableReason{model.StoreReasonKYCNotBusiness, model.StoreReasonStoreClosed}
	a = read()
	closed := "店家已關閉，這份商家認證申請暫停審核，店家恢復營業後才能審核或重新查詢。"
	if a.GetCanApprove() || a.GetCanReject() || a.GetCanRequeryRegistration() ||
		len(a.GetApproveUnavailabilities()) != 1 || a.GetApproveUnavailabilities()[0].GetReason() != adminpb.StoreActionUnavailableReason_STORE_ACTION_UNAVAILABLE_REASON_STORE_CLOSED ||
		a.GetApproveUnavailabilities()[0].GetMessage().GetMessage() != closed || a.GetRejectUnavailabilities()[0].GetMessage().GetMessage() != closed ||
		len(a.GetRequeryRegistrationUnavailabilities()) != 2 || a.GetRequeryRegistrationUnavailabilities()[0].GetMessage().GetMessage() != "只有公司型的申請有登記資料可以查詢。" ||
		a.GetRequeryRegistrationUnavailabilities()[1].GetMessage().GetMessage() != closed {
		t.Fatalf("approve=%v reject=%v requery=%v", a.GetApproveUnavailabilities(), a.GetRejectUnavailabilities(), a.GetRequeryRegistrationUnavailabilities())
	}
	// 停權只擋核准（退回的清單是空的）。
	m.got.ApproveUnavailable = []model.StoreActionUnavailableReason{model.StoreReasonAccountSuspended}
	m.got.RejectUnavailable = nil
	a = read()
	if a.GetCanApprove() || !a.GetCanReject() || a.GetApproveUnavailabilities()[0].GetMessage().GetMessage() != "帳號停權中，停權解除之後才能核准商家認證。" {
		t.Fatalf("approve=%v reject=%v", a.GetApproveUnavailabilities(), a.GetRejectUnavailabilities())
	}
}
