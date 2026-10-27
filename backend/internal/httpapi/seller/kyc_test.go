package seller

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// kycModule 記下收到的輸入，回傳設定好的結果。
type kycModule struct {
	Module
	draft      model.KYCDraftInput
	draftOut   model.KYCApplication
	draftErr   error
	submit     model.SubmitKYCInput
	submitErr  error
	cancel     model.CancelKYCInput
	listIn     model.AccountMeasureListInput
	listOut    model.MeasurePage
	getAccount int64
	getID      uuid.UUID
	measure    model.Measure
	appealIn   model.SubmitAppealInput
	remedyIn   model.RequestRemediationInput
}

func (m *kycModule) SaveKYCDraft(_ context.Context, in model.KYCDraftInput) (model.KYCApplication, error) {
	m.draft = in
	return m.draftOut, m.draftErr
}

func (m *kycModule) MyKYC(_ context.Context, accountID int64) (model.KYCApplication, error) {
	m.draft.AccountID = accountID
	return m.draftOut, nil
}

func (m *kycModule) SubmitKYC(_ context.Context, in model.SubmitKYCInput) (model.KYCApplication, error) {
	m.submit = in
	return m.draftOut, m.submitErr
}

func (m *kycModule) CancelKYC(_ context.Context, in model.CancelKYCInput) (model.KYCApplication, error) {
	m.cancel = in
	return m.draftOut, nil
}

func (m *kycModule) AccountMeasures(_ context.Context, in model.AccountMeasureListInput) (model.MeasurePage, error) {
	m.listIn = in
	return m.listOut, nil
}

func (m *kycModule) AccountMeasure(_ context.Context, accountID int64, id uuid.UUID) (model.Measure, error) {
	m.getAccount, m.getID = accountID, id
	return m.measure, nil
}

func (m *kycModule) SubmitAppeal(_ context.Context, in model.SubmitAppealInput) (model.Measure, error) {
	m.appealIn = in
	return m.measure, nil
}

func (m *kycModule) RequestRemediation(_ context.Context, in model.RequestRemediationInput) (model.Measure, error) {
	m.remedyIn = in
	return m.measure, nil
}

func mask(paths ...string) *fieldmaskpb.FieldMask { return &fieldmaskpb.FieldMask{Paths: paths} }

func saveKYC(t *testing.T, m *kycModule, app *pb.KYCApplication, paths ...string) (*pb.KYCApplication, error) {
	t.Helper()
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	resp, err := h.SaveKYCDraft(ctxWithStore(currentStore()), connect.NewRequest(&pb.SaveKYCDraftRequest{Application: app, UpdateMask: mask(paths...)}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetApplication(), nil
}

func individualOf(idNo string) *pb.KYCApplication_Individual {
	return &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{IdCardNo: idNo}}
}

func businessOf(b *pb.BusinessKYC) *pb.KYCApplication_Business {
	return &pb.KYCApplication_Business{Business: b}
}

// 只有 update_mask 選到的欄位送給模組（沒選到的是 nil，保留原值）；日期轉成 YYYY-MM-DD，etag 是版本，帳號來自店家；
// 賣家型態只在 mask 列到那一支的欄位時才送（更新沒列到就不切換）。
func TestSaveKYCDraftSendsOnlyTheFieldsInTheMask(t *testing.T) {
	m := &kycModule{}
	_, err := saveKYC(t, m, &pb.KYCApplication{
		Etag: "4", RealName: "王小明", BirthDate: &date.Date{Year: 1990, Month: 5, Day: 6}, EnglishName: "不在 mask 內",
		Subject:     individualOf("A123456789"),
		IdCardFront: &pb.KYCDocument{StagedToken: "front"}, BankBook: &pb.KYCDocument{AssetId: "not-in-mask"},
	}, "real_name", "birth_date", "individual.id_card_no", "id_card_front")
	if err != nil {
		t.Fatal(err)
	}
	in := m.draft
	if in.AccountID != 7 || in.ExpectedVersion != 4 || *in.RealName != "王小明" || *in.BirthDate != "1990-05-06" || *in.IDCardNo != "A123456789" {
		t.Fatalf("input=%+v", in)
	}
	if in.EnglishName != nil || in.BankBook != nil || in.IDCardBack != nil || in.CompanyDocuments != nil {
		t.Fatalf("沒選到的欄位被送給模組：%+v", in)
	}
	if in.SellerType == nil || *in.SellerType != model.KYCIndividual {
		t.Fatalf("mask 列到 individual 的欄位，賣家型態就是 individual：%v", in.SellerType)
	}
	if in.IDCardFront == nil || *in.IDCardFront != (model.KYCDocumentRef{StagedToken: "front"}) {
		t.Fatalf("front=%+v", in.IDCardFront)
	}
	// 更新時 mask 沒有列到那一支的欄位：不送賣家型態。
	m = &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "4", RealName: "改", Subject: individualOf("")}, "real_name"); err != nil || m.draft.SellerType != nil {
		t.Fatalf("err=%v type=%v", err, m.draft.SellerType)
	}
}

// 選到卻沒有值的欄位是清掉（空字串）；沒帶 mask 時只有有值的欄位；OUTPUT_ONLY 的欄位列在 mask 裡也不送；建立（沒有 etag）必須
// 設定 individual 或 business 其中一支，賣家型態就是那一支。
func TestSaveKYCDraftClearsSelectedEmptyFieldsAndIgnoresOutputOnlyOnes(t *testing.T) {
	m := &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1"}, "birth_date", "real_name", "individual.id_card_no_masked", "status", "flow", "etag", "id", "missing_fields"); err != nil {
		t.Fatal(err)
	}
	if in := m.draft; *in.BirthDate != "" || *in.RealName != "" || in.IDCardNo != nil || in.BankAccount != nil || in.SellerType != nil {
		t.Fatalf("input=%+v", in)
	}
	m = &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Subject: businessOf(&pb.BusinessKYC{TaxId: "33330333", RepIdCardNoMasked: "H2******10"})}); err != nil {
		t.Fatal(err)
	}
	in := m.draft
	if *in.SellerType != model.KYCCompany || *in.TaxID != "33330333" || in.ExpectedVersion != 0 || in.RealName != nil || in.RepIDCardNo != nil {
		t.Fatalf("input=%+v", in)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{RealName: "沒有選型態"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("建立時沒有設定 individual 或 business：err=%v", err)
	}
	// 沒有 mask 的建立：subject 設定的那一支就是型態（空的 individual 也算）。
	if _, err := saveKYC(t, m, &pb.KYCApplication{Subject: &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{}}}); err != nil || *m.draft.SellerType != model.KYCIndividual {
		t.Fatalf("err=%v input=%+v", err, m.draft)
	}

	// business.company_address 列在 mask 但沒有值時清成空字串。
	m = &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1", Subject: businessOf(&pb.BusinessKYC{})}, "business.company_address"); err != nil {
		t.Fatal(err)
	}
	if in = m.draft; in.CompanyTownCode == nil || *in.CompanyTownCode != "" || in.CompanyAddressDetail == nil || *in.CompanyAddressDetail != "" {
		t.Fatalf("clearing company_address failed: %+v", in)
	}

	// 只有鄉鎮沒有明細時正常送出，明細為空字串。
	m = &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{
		Etag: "1",
		Subject: businessOf(&pb.BusinessKYC{
			CompanyAddress: &commonpb.PostalAddress{TownCode: "63000050"},
		}),
	}, "business.company_address"); err != nil {
		t.Fatal(err)
	}
	if in = m.draft; in.CompanyTownCode == nil || *in.CompanyTownCode != "63000050" || in.CompanyAddressDetail == nil || *in.CompanyAddressDetail != "" {
		t.Fatalf("town only company_address failed: %+v", in)
	}
}

// KYC 申請不再有食品業者登錄：food_business 不是申請的欄位，列進 update_mask 是 invalid_argument。
func TestSaveKYCDraftRejectsTheRemovedFoodBusinessPath(t *testing.T) {
	m := &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1"}, "food_business"); err == nil || !strings.HasPrefix(err.Error(), "invalid_argument") {
		t.Fatalf("err=%v", err)
	}
}

// 證件是狀態：單張的位置列進 mask 時整個換掉（沒有值是清掉），公司文件整個換成請求的清單，每個元素是現有的圖片或新的
// staged token；沒列進 mask 的位置不動。
func TestSaveKYCDraftTreatsDocumentsAsState(t *testing.T) {
	m := &kycModule{}
	app := &pb.KYCApplication{
		Etag: "2", IdCardFront: &pb.KYCDocument{AssetId: "keep-front"},
		Subject: businessOf(&pb.BusinessKYC{CompanyDocuments: []*pb.KYCDocument{{AssetId: "keep-co"}, {StagedToken: "new-co"}}}),
	}
	if _, err := saveKYC(t, m, app, "id_card_front", "id_card_back", "business.company_documents"); err != nil {
		t.Fatal(err)
	}
	in := m.draft
	if *in.IDCardFront != (model.KYCDocumentRef{AssetID: "keep-front"}) || *in.IDCardBack != (model.KYCDocumentRef{}) || in.BankBook != nil {
		t.Fatalf("input=%+v", in)
	}
	if want := []model.KYCDocumentRef{{AssetID: "keep-co"}, {StagedToken: "new-co"}}; in.CompanyDocuments == nil || !slices.Equal(*in.CompanyDocuments, want) {
		t.Fatalf("company=%+v", in.CompanyDocuments)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "2", Subject: businessOf(&pb.BusinessKYC{})}, "business.company_documents"); err != nil || m.draft.CompanyDocuments == nil || len(*m.draft.CompanyDocuments) != 0 {
		t.Fatalf("空清單是清掉全部：err=%v company=%+v", err, m.draft.CompanyDocuments)
	}
}

// 申請人不填組織型態：business.company_type 不是可寫的欄位，列在 update_mask 裡被拒絕，不會到模組。
func TestSaveKYCDraftRejectsTheRetiredCompanyTypePath(t *testing.T) {
	m := &kycModule{}
	_, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1", Subject: businessOf(&pb.BusinessKYC{CompanyName: "某某公司"})}, "business.company_type")
	if err == nil || !strings.Contains(err.Error(), "不是可以修改的欄位") {
		t.Fatalf("err=%v，預期 update_mask 不是可以修改的欄位", err)
	}
	if m.draft.CompanyName != nil {
		t.Fatalf("請求被拒絕後不應進到模組：%+v", m.draft)
	}
}

// update_mask 是 "*"：所有最上層的欄位加上設定的那一支，沒有型態的特例：個人與企業都能用，沒設定的那一支不展開；
// 列到的欄位在另一支底下是請求自相矛盾。
func TestSaveKYCDraftWildcardMaskCoversTheSetBranch(t *testing.T) {
	m := &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1", RealName: "王小明", Subject: individualOf("A123456789")}, "*"); err != nil {
		t.Fatal(err)
	}
	in := m.draft
	if *in.SellerType != model.KYCIndividual || in.IDCardNo == nil || in.RealName == nil || in.BankName == nil || *in.BankName != "" ||
		in.CompanyName != nil || in.TaxID != nil || in.RepIDCardNo != nil || in.CompanyDocuments != nil || in.CompanyPhone != nil {
		t.Fatalf("個人賣家：%+v", in)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1", Subject: businessOf(&pb.BusinessKYC{CompanyName: "某某公司", TaxId: "33330333", RepIdCardNo: "X223456789"})}, "*"); err != nil {
		t.Fatal(err)
	}
	in = m.draft
	if *in.SellerType != model.KYCCompany || in.IDCardNo != nil || in.CompanyName == nil || *in.CompanyName != "某某公司" || in.TaxID == nil || in.RepIDCardNo == nil ||
		in.CompanyDocuments == nil {
		t.Fatalf("企業賣家：%+v", in)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1", Subject: individualOf("A123456789")}, "business.company_name"); err == nil || !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("mask 與設定的那一支不一致：err=%v", err)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1"}, "individual.id_card_no", "business.tax_id"); err == nil {
		t.Fatal("同時列到兩支的欄位通過了")
	}
}

func TestSaveKYCDraftRejectsBadRequests(t *testing.T) {
	m := &kycModule{}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	if _, err := h.SaveKYCDraft(ctxWithStore(currentStore()), connect.NewRequest(&pb.SaveKYCDraftRequest{})); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("沒有 application：err=%v", err)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1"}, "no_such_field"); err == nil || !strings.Contains(err.Error(), "invalid_argument") {
		t.Fatalf("不認得的路徑：err=%v", err)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "abc"}, "real_name"); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("etag 不合法：err=%v", err)
	}
	if _, err := h.SaveKYCDraft(context.Background(), connect.NewRequest(&pb.SaveKYCDraftRequest{Application: &pb.KYCApplication{}})); apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("沒有店家：err=%v", err)
	}
}

// 模組的草稿輸入每個欄位都有對應到請求的路徑（錯誤才指得到欄位），路徑在請求訊息裡真的存在。
func TestKYCDraftFieldPathsCoverTheModuleInput(t *testing.T) {
	typ := reflect.TypeFor[model.KYCDraftInput]()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		switch name {
		case "AccountID":
			continue
		}
		_, plain := kycPaths[name]
		_, repeated := kycPaths[name+"[]"] // repeated 欄位只列一筆 X[]，同時涵蓋 X 與 X[i]
		if !plain && !repeated {
			t.Errorf("KYCDraftInput.%s 沒有對應到請求的欄位", name)
		}
	}
	for name, paths := range map[string]struct {
		paths rpc.FieldPaths
		req   interface{ ProtoReflect() protoreflect.Message }
	}{
		"draft":          {kycPaths, &pb.SaveKYCDraftRequest{}},
		"submit":         {withoutKeys(submitKYCPaths, "ExpectedVersion", "AcceptedAgreements"), &pb.KYCApplication{}},
		"submit request": {rpc.FieldPaths{"ExpectedVersion": submitKYCPaths["ExpectedVersion"], "AcceptedAgreements": submitKYCPaths["AcceptedAgreements"]}, &pb.SubmitKYCRequest{}},
		"cancel":         {cancelKYCPaths, &pb.CancelKYCRequest{}},
		"appeal":         {appealPaths, &pb.SubmitAppealRequest{}},
		"remediation":    {appealPaths, &pb.RequestRemediationRequest{}},
		"listing":        {measurePaths, &pb.ListMyMeasuresRequest{}},
	} {
		if missing := rpc.CheckFieldPaths(paths.req.ProtoReflect().Descriptor(), paths.paths); len(missing) > 0 {
			t.Errorf("%s：不存在的路徑 %v", name, missing)
		}
	}
}

func withoutKeys(p rpc.FieldPaths, keys ...string) rpc.FieldPaths {
	out := rpc.FieldPaths{}
	for k, v := range p {
		if !slices.Contains(keys, k) {
			out[k] = v
		}
	}
	return out
}

func fullKYC(t model.KYCSellerType) model.KYCApplication {
	text := func(s string) *string { return &s }
	flow := model.KYCFlow{Status: model.FlowBindFailed, CanResubmit: true, IdentityLocked: true}
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	return model.KYCApplication{
		ID: uuid.MustParse("0192f1c0-0000-7000-8000-000000000001"), Status: model.KYCStatusRejected, Version: 3, SellerType: t,
		RealName: text("王小明"), BirthDate: text("1990-05-06"), IDCardNoMasked: text("A1******89"),
		BankAccountMasked: text("********8901"), RepIDCardNoMasked: text("H2******10"), CompanyRegisterDate: text("2015-07-13"), TaxID: text("33330333"),
		Documents: []model.KYCDocument{
			{Slot: model.SlotIDCardFront, Position: 0, AssetID: "asset front", ObjectKey: "certification/secret-key"},
			{Slot: model.SlotCompanyDocument, Position: 0, AssetID: "co-1"}, {Slot: model.SlotCompanyDocument, Position: 1, AssetID: "co-2"},
		},
		MissingFields: []string{"business.tax_id", "individual.id_card_no", "real_name"},
		Flow:          &flow,
		LastRejection: &model.KYCRejection{Reason: "模糊", DecidedAt: now, Issues: []model.KYCIssue{
			{Field: "real_name", Code: model.IssueMismatch, Message: "與證件上的姓名不同"},
			{Field: "id_card_front", DocumentAssetID: "asset front", Code: model.IssueUnreadable, Message: "反光看不清楚"},
			{Field: "business.company_documents", DocumentAssetID: "co-2", Code: model.IssueExpired, Message: "登記文件已過期"},
		}},
		OutstandingIssues: []model.KYCIssue{{Field: "id_card_front", DocumentAssetID: "asset front", Code: model.IssueUnreadable, Message: "反光看不清楚"}},
		CreatedAt:         now, UpdatedAt: now, SubmittedAt: &now,
	}
}

// onlyInputFields 回傳訊息（含巢狀）裡有值的 INPUT_ONLY 欄位路徑。
func inputOnlyFieldsSet(m protoreflect.Message, prefix string) (out []string) {
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if rpc.HasBehavior(fd, annotations.FieldBehavior_INPUT_ONLY) {
			out = append(out, prefix+string(fd.Name()))
			return true
		}
		switch {
		case fd.IsList() && fd.Kind() == protoreflect.MessageKind:
			for i := range v.List().Len() {
				out = append(out, inputOnlyFieldsSet(v.List().Get(i).Message(), prefix+string(fd.Name())+".")...)
			}
		case !fd.IsList() && !fd.IsMap() && fd.Kind() == protoreflect.MessageKind:
			out = append(out, inputOnlyFieldsSet(v.Message(), prefix+string(fd.Name())+".")...)
		}
		return true
	})
	return out
}

// 回應只有遮蔽後的值：INPUT_ONLY 的欄位（身分證號、銀行帳號、staged token，含 individual 與 business 裡的）一律不填；證件只有
// asset ID 與有授權的下載網址，沒有 object key；賣家型態是 subject 設定的那一支；缺的欄位依列舉的數字順序。
func TestKYCResponseCarriesOnlyMaskedValuesAndNoDocumentImages(t *testing.T) {
	ind := toKYC(context.Background(), fullKYC(model.KYCIndividual))
	if ind.GetIndividual().GetIdCardNoMasked() != "A1******89" || ind.GetBusiness() != nil || ind.GetBankAccountMasked() != "********8901" {
		t.Fatalf("individual=%v", ind)
	}
	out := toKYC(context.Background(), fullKYC(model.KYCCompany))
	if out.GetBusiness().GetRepIdCardNoMasked() != "H2******10" || out.GetIndividual() != nil || out.GetBusiness().GetTaxId() != "33330333" {
		t.Fatalf("business=%v", out)
	}
	for name, a := range map[string]*pb.KYCApplication{"individual": ind, "business": out} {
		if got := inputOnlyFieldsSet(a.ProtoReflect(), ""); len(got) > 0 {
			t.Errorf("%s：INPUT_ONLY 的欄位出現在回應：%v", name, got)
		}
	}
	if out.GetIdCardFront().GetAssetId() != "asset front" || out.GetIdCardFront().GetDownloadUri() != "/api/v1/app/media/secure_download/asset%20front" ||
		out.GetIdCardBack() != nil || out.GetBankBook() != nil || len(out.GetBusiness().GetCompanyDocuments()) != 2 || out.GetBusiness().GetCompanyDocuments()[1].GetAssetId() != "co-2" {
		t.Fatalf("documents=%v", out)
	}
	if text := out.String() + prototext.Format(out); strings.Contains(text, "secret-key") {
		t.Fatalf("證件的 object key 出現在回應：%s", text)
	}
	if d := out.GetBusiness().GetCompanyRegisterDate(); d.GetYear() != 2015 || d.GetMonth() != 7 || d.GetDay() != 13 || out.GetEtag() != "3" {
		t.Fatalf("out=%v", out)
	}
	rej := out.GetLastRejection()
	wantIssues := []*pb.KYCIssue{
		{Field: pb.KYCField_KYC_FIELD_REAL_NAME, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_MISMATCH, Message: "與證件上的姓名不同"},
		{Field: pb.KYCField_KYC_FIELD_ID_CARD_FRONT, DocumentAssetId: "asset front", Code: pb.KYCIssueCode_KYC_ISSUE_CODE_UNREADABLE, Message: "反光看不清楚"},
		{Field: pb.KYCField_KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS, DocumentAssetId: "co-2", Code: pb.KYCIssueCode_KYC_ISSUE_CODE_EXPIRED, Message: "登記文件已過期"},
	}
	if len(rej.GetIssues()) != 3 || rej.GetReason() != "模糊" {
		t.Fatalf("rejection=%v", rej)
	}
	for i, want := range wantIssues {
		if !proto.Equal(rej.GetIssues()[i], want) {
			t.Fatalf("issues[%d]=%v，預期 %v", i, rej.GetIssues()[i], want)
		}
	}
	if got := out.GetOutstandingIssues(); len(got) != 1 || !proto.Equal(got[0], wantIssues[1]) {
		t.Fatalf("outstanding_issues=%v", got)
	}
	if want := []pb.KYCField{pb.KYCField_KYC_FIELD_REAL_NAME, pb.KYCField_KYC_FIELD_INDIVIDUAL_ID_CARD_NO, pb.KYCField_KYC_FIELD_BUSINESS_TAX_ID}; !slices.Equal(out.GetMissingFields(), want) {
		t.Fatalf("missing_fields=%v", out.GetMissingFields())
	}
	flow := out.GetFlow()
	if flow.GetStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_BIND_FAILED || !flow.GetCanResubmit() || !flow.GetIdentityLocked() {
		t.Fatalf("flow=%v", flow)
	}
}

// SubmitKYC 的身分比對失敗是模組的固定文案：handler 原樣交回，不加欄位、帳號或原因的細節。
func TestSubmitKYCPassesTheGenericIdentityErrorThroughUnchanged(t *testing.T) {
	generic := apperr.FailedPrecondition("無法通過認證，請聯絡客服")
	m := &kycModule{submitErr: generic}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	req := connect.NewRequest(&pb.SubmitKYCRequest{Etag: "2", AcceptedAgreements: []*pb.AgreementAcceptance{
		{Kind: pb.AgreementKind_AGREEMENT_KIND_KYC_PRIVACY_NOTICE, Version: "2026-10-07"},
		{Kind: pb.AgreementKind_AGREEMENT_KIND_PAYUNI_TERMS, Version: "2026-10-07"},
	}})
	req.Header().Set("User-Agent", "hk-test/1.0")
	_, err := h.SubmitKYC(rpc.WithClientIP(ctxWithStore(currentStore()), "203.0.113.9"), req)
	if !errors.Is(err, generic) || err.Error() != generic.Error() || apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("handler 改寫了錯誤：%v", err)
	}
	wantSubmit := model.SubmitKYCInput{
		AccountID: 7, ExpectedVersion: 2, ClientIP: "203.0.113.9", UserAgent: "hk-test/1.0",
		AcceptedAgreements: []model.AgreementAcceptance{
			{Kind: model.AgreementKYCPrivacyNotice, Version: "2026-10-07"},
			{Kind: model.AgreementPayUniTerms, Version: "2026-10-07"},
		},
	}
	if !reflect.DeepEqual(m.submit, wantSubmit) {
		t.Fatalf("input=%+v", m.submit)
	}
	if _, err := h.SubmitKYC(ctxWithStore(currentStore()), connect.NewRequest(&pb.SubmitKYCRequest{})); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("沒有 etag：err=%v", err)
	}
}

func TestCancelKYCSendsTheVersionAndReason(t *testing.T) {
	m := &kycModule{}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	if _, err := h.CancelKYC(ctxWithStore(currentStore()), connect.NewRequest(&pb.CancelKYCRequest{Etag: "5", Reason: "改天再填"})); err != nil {
		t.Fatal(err)
	}
	if want := (model.CancelKYCInput{AccountID: 7, ExpectedVersion: 5, Reason: "改天再填"}); m.cancel != want {
		t.Fatalf("input=%+v", m.cancel)
	}
}

// 模組的值域與契約的列舉一一對應。
func TestKYCAndGovernanceEnumsMatchTheModule(t *testing.T) {
	fields := make([]string, 0, len(model.KYCFieldNames()))
	for _, name := range model.KYCFieldNames() {
		fields = append(fields, strings.ToUpper(strings.ReplaceAll(name, ".", "_")))
	}
	for name, mismatches := range map[string][]string{
		"KYCStatus":                    rpc.EnumMismatches[pb.KYCStatus](model.KYCStatusValues()),
		"KYCField":                     rpc.EnumMismatches[pb.KYCField](fields),
		"KYCFlowStatus":                rpc.EnumMismatches[pb.KYCFlowStatus](model.KYCFlowStatusValues()),
		"PayUniBindStatus":             rpc.EnumMismatches[pb.PayUniBindStatus](model.PayUniBindStatusValues()),
		"PayUniState":                  rpc.EnumMismatches[pb.PayUniState](model.PayUniStateValues()),
		"KYCResubmitUnavailableReason": rpc.EnumMismatches[pb.KYCResubmitUnavailableReason](model.KYCResubmitUnavailableReasonValues()),
		"PolicyCategory":               rpc.EnumMismatches[commonpb.PolicyCategory](policy.Values()),
		"ViolationSeverity":            rpc.EnumMismatches[pb.ViolationSeverity](model.SeverityValues()),
		"ViolationStatus":              rpc.EnumMismatches[pb.ViolationStatus](model.ViolationStatusValues()),
		"RevokeReason":                 rpc.EnumMismatches[pb.RevokeReason](model.RevokeReasonValues()),
		"MeasureKind":                  rpc.EnumMismatches[pb.MeasureKind](model.MeasureKindValues()),
		"MeasureBasis":                 rpc.EnumMismatches[pb.MeasureBasis](model.BasisValues()),
		"RiskGround":                   rpc.EnumMismatches[pb.RiskGround](model.RiskGroundValues()),
		"MeasureState":                 rpc.EnumMismatches[pb.MeasureState](model.MeasureStateValues()),
		"MeasureRequestState":          rpc.EnumMismatches[pb.MeasureRequestState](model.RequestStateValues()),
		"MeasureRequestCloseCause":     rpc.EnumMismatches[pb.MeasureRequestCloseCause](model.RequestCloseCauseValues()),
	} {
		if len(mismatches) > 0 {
			t.Errorf("%s：%v", name, mismatches)
		}
	}
}

// 後台看的 PayUni 問題：碼、審核意見與欄位；處置類別不在契約裡（賣家看的是 KYCFlow.payuni）。
func TestToProblemMapsCodeReasonAndFields(t *testing.T) {
	text := func(s string) *string { return &s }
	reg := model.PayUniRegistration{
		StoreID: 11, BindStatus: model.BindStatusBound,
		Status: model.PayUniStatus{MerStatus: text("8"), MemberStatus: text("0"), Reason: text("  請補上店面照片\n"), Outcome: model.PayUniOutcomeRejected},
	}
	want := model.PayUniProblemOf(reg)
	got := toProblem(want)
	if got.GetCode() != want.Code || got.GetReason() != "請補上店面照片" || len(got.GetFields()) != len(want.Fields) || len(want.Fields) == 0 {
		t.Fatalf("problem=%v want=%+v", got, want)
	}
	if p := toProblem(model.PayUniProblemOf(model.PayUniRegistration{BindStatus: model.BindStatusPending})); p != nil {
		t.Fatalf("綁定中不該有問題：%v", p)
	}
}

// 賣家讀自己的申請：流程帶 PayUni 狀態（標題以外的事實與句子：說明的語系與內容、審核意見、能修正的欄位）與不能重新送出的原因；
// 後台的 KYCApplication.flow 兩者都不設（後台的 PayUni 呈現在 PayUniRegistration）。
func TestSellerFlowCarriesPayUniAndResubmitUnavailabilitiesButAdminFlowDoesNot(t *testing.T) {
	flow := model.KYCFlow{Status: model.FlowPayUniRejected}
	app := model.KYCApplication{
		Flow: &flow,
		PayUni: &model.SellerPayUni{
			State: model.PayUniState("FIX_VIA_SUPPORT"), Reason: "地址不完整", FixStoreFields: []string{"address", "contact_phone"},
		},
		ResubmitReasons: []model.KYCResubmitUnavailableReason{model.KYCResubmitUnavailableReason("PAYUNI_FIX_VIA_SUPPORT")},
	}
	got := toKYC(context.Background(), app).GetFlow()
	explanation, _ := app.PayUni.State.Explanation(uitext.ZhTW)
	p := got.GetPayuni()
	if p.GetState() != pb.PayUniState_PAY_UNI_STATE_FIX_VIA_SUPPORT || p.GetExplanation().GetLocale() != "zh-TW" || p.GetExplanation().GetMessage() != explanation.Text ||
		explanation.Text == "" || p.GetReason() != "地址不完整" || len(p.GetFixFields()) != 0 ||
		!slices.Equal(p.GetFixStoreFields(), []pb.StoreField{pb.StoreField_STORE_FIELD_ADDRESS, pb.StoreField_STORE_FIELD_CONTACT_PHONE}) {
		t.Fatalf("payuni=%v", p)
	}
	reason := model.KYCResubmitUnavailableReason("PAYUNI_FIX_VIA_SUPPORT")
	u := got.GetResubmitUnavailabilities()
	if len(u) != 1 || u[0].GetReason() != pb.KYCResubmitUnavailableReason_KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_FIX_VIA_SUPPORT ||
		u[0].GetMessage().GetMessage() != reason.Text(uitext.ZhTW).Text || u[0].GetMessage().GetMessage() == "" {
		t.Fatalf("resubmit_unavailabilities=%v", u)
	}
	// 沒有 PayUni 狀態（申請不在 PayUni 階段）時不設。
	if f := toKYC(context.Background(), model.KYCApplication{Flow: &flow}).GetFlow(); f.GetPayuni() != nil || len(f.GetResubmitUnavailabilities()) != 0 {
		t.Fatalf("flow=%v", f)
	}
	admin := toAdminKYC(model.KYCAdminApplication{KYCApplication: app}).GetFlow()
	if admin.GetPayuni() != nil || len(admin.GetResubmitUnavailabilities()) != 0 || admin.GetStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_PAYUNI_REJECTED {
		t.Fatalf("admin flow=%v", admin)
	}
}

// lookupModule 是統編查詢的模組替身（登記資料來源、現況分類與地址拆分都在模組，這裡只測 handler 的格式檢查與回應對照）。
type lookupModule struct {
	Module
	reg    model.Registration
	found  bool
	err    error
	called []string
}

func (m *lookupModule) LookupBusinessRegistration(_ context.Context, taxID string) (model.Registration, bool, error) {
	m.called = append(m.called, taxID)
	return m.reg, m.found, m.err
}

func lookup(t *testing.T, m *lookupModule, taxID string) (*pb.LookupBusinessRegistrationResponse, error) {
	t.Helper()
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	resp, err := h.LookupBusinessRegistration(ctxWithStore(currentStore()), connect.NewRequest(&pb.LookupBusinessRegistrationRequest{TaxId: taxID}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

// 查無是回應沒有 registration（不是錯誤）；模組回的 unavailable 與沒有分類的錯誤原樣交回（兩者不混為一談）。
func TestLookupBusinessRegistrationDistinguishesNotFoundFromUnavailable(t *testing.T) {
	notFound, err := lookup(t, &lookupModule{}, "00000000")
	if err != nil || notFound.GetRegistration() != nil {
		t.Fatalf("查無：resp=%v err=%v", notFound, err)
	}
	if _, err := lookup(t, &lookupModule{err: apperr.Unavailable("暫時無法使用")}, "33330333"); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("不可用：err=%v", err)
	}
	if _, err := lookup(t, &lookupModule{err: errors.New("boom")}, "33330333"); apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("沒有分類的錯誤不能被當成查無或不可用：err=%v", err)
	}
}

func TestLookupBusinessRegistrationMapsTheRegistrationAndValidatesTheTaxID(t *testing.T) {
	m := &lookupModule{found: true, reg: model.Registration{
		TaxID: "33330333", Name: "某某有限公司", Address: "臺北市中正區台北路一號", OrganizationType: "有限公司",
		SetupDate: time.Date(1987, 2, 21, 0, 0, 0, 0, time.UTC), TownCode: "63000050", AddressDetail: "台北路一號",
	}}
	resp, err := lookup(t, m, " 3333 0333 ")
	if err != nil || !slices.Equal(m.called, []string{"33330333"}) {
		t.Fatalf("err=%v called=%v", err, m.called)
	}
	got := resp.GetRegistration()
	if got.GetName() != "某某有限公司" || got.GetPostalAddress().GetTownCode() != "63000050" || got.GetPostalAddress().GetAddressDetail() != "台北路一號" || got.GetOrganizationType() != "有限公司" ||
		got.GetSetupDate().GetYear() != 1987 || got.GetSetupDate().GetMonth() != 2 || got.GetSetupDate().GetDay() != 21 {
		t.Fatalf("registration=%v", got)
	}
	// 地址拆不開時只有原文。
	m.reg.TownCode, m.reg.AddressDetail = "", ""
	resp, err = lookup(t, m, "33330333")
	if err != nil || resp.GetRegistration().GetPostalAddress() != nil || resp.GetRegistration().GetAddress() == "" {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	m.called = nil
	for _, bad := range []string{"", "1234567", "abcdefgh", "123456789", "12345678"} { // 最後一個：格式對、檢查碼不對
		_, err := lookup(t, m, bad)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%q：err=%v", bad, err)
		}
	}
	if len(m.called) != 0 {
		t.Fatalf("格式不合的統編查了資料來源：%v", m.called)
	}
}

// KYCField 與 KYCApplication 的欄位路徑一對一：退回、PayUni 問題、缺項與送出的 BadRequest 用同一套語彙。可寫的欄位（"*" 展開，
// 兩支都算）去掉 etag 就是 KYCFieldNames；每個 KYCField 對到真的存在的路徑；PayUni 的欄位群組都在裡面。
func TestKYCFieldIsOneToOneWithTheWritableApplicationFields(t *testing.T) {
	var writableLeaves []string
	for _, subject := range []*pb.KYCApplication{
		{Subject: &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{}}},
		{Subject: &pb.KYCApplication_Business{Business: &pb.BusinessKYC{}}},
	} {
		paths, err := rpc.WritableFields(subject, mask("*"))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			if p != "etag" && !slices.Contains(writableLeaves, p) {
				writableLeaves = append(writableLeaves, p)
			}
		}
	}
	names := model.KYCFieldNames()
	slices.Sort(writableLeaves)
	sorted := slices.Clone(names)
	slices.Sort(sorted)
	if !slices.Equal(writableLeaves, sorted) {
		t.Fatalf("可寫的欄位與 KYCFieldNames 不一致：\n descriptor=%v\n names=%v", writableLeaves, sorted)
	}
	if missing := rpc.CheckFieldPaths((&pb.KYCApplication{}).ProtoReflect().Descriptor(), withoutKeys(kycSubmitPaths(), "AcceptedAgreements")); len(missing) > 0 {
		t.Fatalf("送出的欄位路徑不在 KYCApplication：%v", missing)
	}
	storePaths := rpc.FieldPaths{}
	for _, name := range model.StoreFieldNames() {
		storePaths[name] = name
	}
	if missing := rpc.CheckFieldPaths((&pb.Store{}).ProtoReflect().Descriptor(), storePaths); len(missing) > 0 {
		t.Fatalf("StoreFieldNames 的路徑不在 Store：%v", missing)
	}
	for _, name := range model.StoreFieldNames() {
		if storeFieldPB(name) == pb.StoreField_STORE_FIELD_UNSPECIFIED {
			t.Errorf("%s 沒有對應的 StoreField", name)
		}
	}
	if len(pb.StoreField_name) != len(model.StoreFieldNames())+1 {
		t.Errorf("StoreField 有 %d 個值（含 UNSPECIFIED），StoreFieldNames 有 %d 個", len(pb.StoreField_name), len(model.StoreFieldNames()))
	}
	for _, name := range names {
		if kycFieldPB(name) == pb.KYCField_KYC_FIELD_UNSPECIFIED {
			t.Errorf("%s 沒有對應的 KYCField", name)
		}
	}
}

func TestStoreFieldsAreMappedOnTheKYCViewAndTheProblem(t *testing.T) {
	out := toKYC(context.Background(), model.KYCApplication{MissingStoreFields: []string{"support_email", "address", "contact_name"}})
	want := []pb.StoreField{pb.StoreField_STORE_FIELD_ADDRESS, pb.StoreField_STORE_FIELD_CONTACT_NAME, pb.StoreField_STORE_FIELD_SUPPORT_EMAIL}
	if !slices.Equal(out.GetMissingStoreFields(), want) {
		t.Fatalf("missing_store_fields=%v", out.GetMissingStoreFields())
	}
	p := toProblem(&model.PayUniProblem{Code: "AGENT06006", Remedy: model.RemedySellerFix, StoreFields: []string{"contact_mobile", "contact_phone"}})
	if !slices.Equal(p.GetStoreFields(), []pb.StoreField{pb.StoreField_STORE_FIELD_CONTACT_MOBILE, pb.StoreField_STORE_FIELD_CONTACT_PHONE}) || len(p.GetFields()) != 0 {
		t.Fatalf("problem=%v", p)
	}
}
