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

func individualOf() *pb.KYCApplication_Individual {
	return &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{}}
}

func businessOf(b *pb.BusinessKYC) *pb.KYCApplication_Business {
	return &pb.KYCApplication_Business{Business: b}
}

// 只有 update_mask 選到的欄位送給模組（沒選到的是 nil，保留原值）；etag 是版本，帳號來自店家；
// 賣家型態只在 mask 列到那一支（它的欄位，或直接列 business／individual）時才送（更新沒列到就不切換）。
func TestSaveKYCDraftSendsOnlyTheFieldsInTheMask(t *testing.T) {
	m := &kycModule{}
	_, err := saveKYC(t, m, &pb.KYCApplication{
		Etag: "4", ProductType: "有機蔬果", WebsiteUrl: "https://不在-mask.example.com",
		Subject: businessOf(&pb.BusinessKYC{TaxId: "33330333", CompanyName: "不在 mask 內"}),
	}, "product_type", "business.tax_id")
	if err != nil {
		t.Fatal(err)
	}
	in := m.draft
	if in.AccountID != 7 || in.ExpectedVersion != 4 || *in.ProductType != "有機蔬果" || *in.TaxID != "33330333" {
		t.Fatalf("input=%+v", in)
	}
	if in.WebsiteURL != nil || in.CompanyName != nil || in.CompanyDocuments != nil {
		t.Fatalf("沒選到的欄位被送給模組：%+v", in)
	}
	if in.SellerType == nil || *in.SellerType != model.KYCCompany {
		t.Fatalf("mask 列到 business 的欄位，賣家型態就是 company：%v", in.SellerType)
	}
	// 更新時 mask 沒有列到那一支：不送賣家型態。
	m = &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "4", ProductType: "改", Subject: individualOf()}, "product_type"); err != nil || m.draft.SellerType != nil {
		t.Fatalf("err=%v type=%v", err, m.draft.SellerType)
	}
	// individual 沒有欄位（D22）：型態的切換靠 mask 直接列 individual 或 business。
	m = &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "4", Subject: individualOf()}, "individual"); err != nil || m.draft.SellerType == nil || *m.draft.SellerType != model.KYCIndividual {
		t.Fatalf("mask 直接列 individual：err=%v type=%v", err, m.draft.SellerType)
	}
	m = &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "4", Subject: businessOf(&pb.BusinessKYC{})}, "business"); err != nil || m.draft.SellerType == nil || *m.draft.SellerType != model.KYCCompany {
		t.Fatalf("mask 直接列 business：err=%v type=%v", err, m.draft.SellerType)
	}
}

// 選到卻沒有值的欄位是清掉（空字串）；沒帶 mask 時只有有值的欄位；OUTPUT_ONLY 的欄位列在 mask 裡也不送；建立（沒有 etag）必須
// 設定 individual 或 business 其中一支，賣家型態就是那一支。
func TestSaveKYCDraftClearsSelectedEmptyFieldsAndIgnoresOutputOnlyOnes(t *testing.T) {
	m := &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1"}, "website_url", "product_type", "status", "last_rejection", "etag", "id", "missing_fields"); err != nil {
		t.Fatal(err)
	}
	if in := m.draft; *in.WebsiteURL != "" || *in.ProductType != "" || in.TaxID != nil || in.SellerType != nil {
		t.Fatalf("input=%+v", in)
	}
	m = &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Subject: businessOf(&pb.BusinessKYC{TaxId: "33330333"})}); err != nil {
		t.Fatal(err)
	}
	in := m.draft
	if *in.SellerType != model.KYCCompany || *in.TaxID != "33330333" || in.ExpectedVersion != 0 || in.ProductType != nil {
		t.Fatalf("input=%+v", in)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{ProductType: "沒有選型態"}); apperr.KindOf(err) != apperr.KindInvalidInput {
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

// 公司文件是狀態：整個換成請求的清單，每個元素是現有的圖片或新的 staged token；沒列進 mask 時不動。
func TestSaveKYCDraftTreatsDocumentsAsState(t *testing.T) {
	m := &kycModule{}
	app := &pb.KYCApplication{
		Etag: "2", Subject: businessOf(&pb.BusinessKYC{CompanyDocuments: []*pb.KYCDocument{{AssetId: "keep-co"}, {StagedToken: "new-co"}}}),
	}
	if _, err := saveKYC(t, m, app, "business.company_documents"); err != nil {
		t.Fatal(err)
	}
	in := m.draft
	if want := []model.KYCDocumentRef{{AssetID: "keep-co"}, {StagedToken: "new-co"}}; in.CompanyDocuments == nil || !slices.Equal(*in.CompanyDocuments, want) {
		t.Fatalf("company=%+v", in.CompanyDocuments)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "2", Subject: businessOf(&pb.BusinessKYC{})}, "business.company_documents"); err != nil || m.draft.CompanyDocuments == nil || len(*m.draft.CompanyDocuments) != 0 {
		t.Fatalf("空清單是清掉全部：err=%v company=%+v", err, m.draft.CompanyDocuments)
	}
	m = &kycModule{}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "2", ProductType: "x"}, "product_type"); err != nil || m.draft.CompanyDocuments != nil {
		t.Fatalf("沒列進 mask 的文件被送給模組：err=%v company=%+v", err, m.draft.CompanyDocuments)
	}
}

// 已刪除的欄位（D22、D37）不是可寫的路徑：列進 update_mask 是 invalid_argument，不會到模組。
func TestSaveKYCDraftRejectsTheRemovedPIIPaths(t *testing.T) {
	for _, path := range []string{"real_name", "birth_date", "english_name", "bank_name", "bank_code", "bank_account", "id_card_front", "id_card_back", "bank_book", "individual.id_card_no", "business.rep_id_card_no"} {
		m := &kycModule{}
		if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1"}, path); err == nil || !strings.HasPrefix(err.Error(), "invalid_argument") {
			t.Errorf("%s：err=%v", path, err)
		}
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
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1", ProductType: "有機蔬果", Subject: individualOf()}, "*"); err != nil {
		t.Fatal(err)
	}
	in := m.draft
	if *in.SellerType != model.KYCIndividual || in.ProductType == nil || in.WebsiteURL == nil || *in.WebsiteURL != "" ||
		in.CompanyName != nil || in.TaxID != nil || in.CompanyDocuments != nil || in.CompanyPhone != nil {
		t.Fatalf("個人賣家：%+v", in)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1", Subject: businessOf(&pb.BusinessKYC{CompanyName: "某某公司", TaxId: "33330333"})}, "*"); err != nil {
		t.Fatal(err)
	}
	in = m.draft
	if *in.SellerType != model.KYCCompany || in.CompanyName == nil || *in.CompanyName != "某某公司" || in.TaxID == nil || in.CompanyDocuments == nil {
		t.Fatalf("企業賣家：%+v", in)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1", Subject: individualOf()}, "business.company_name"); err == nil || !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("mask 與設定的那一支不一致：err=%v", err)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1", Subject: businessOf(&pb.BusinessKYC{})}, "individual"); err == nil || !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("直接列 individual 但設定的是 business：err=%v", err)
	}
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1"}, "individual", "business.tax_id"); err == nil {
		t.Fatal("同時列到兩支的欄位通過了")
	}
	// mask 列了某一支、subject 卻沒設：不是靜默成功，是 invalid_argument。
	for _, branch := range []string{"individual", "business", "business.tax_id"} {
		before := m.draft
		_, err := saveKYC(t, m, &pb.KYCApplication{Etag: "1"}, branch)
		if err == nil || !strings.HasPrefix(err.Error(), "invalid_argument:") || !strings.Contains(err.Error(), "沒有設定") || !reflect.DeepEqual(before, m.draft) {
			t.Fatalf("mask 列了 %s 但 subject 沒設：err=%v", branch, err)
		}
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
	if _, err := saveKYC(t, m, &pb.KYCApplication{Etag: "abc"}, "product_type"); apperr.KindOf(err) != apperr.KindVersionConflict {
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
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	return model.KYCApplication{
		ID: uuid.MustParse("0192f1c0-0000-7000-8000-000000000001"), Status: model.KYCStatusRejected, Version: 3, SellerType: t,
		CompanyRegisterDate: text("2015-07-13"), TaxID: text("33330333"),
		Documents: []model.KYCDocument{
			{Position: 0, AssetID: "co-1", ObjectKey: "certification/secret-key"}, {Position: 1, AssetID: "co-2"},
		},
		MissingFields: []string{"business.company_name", "business.tax_id"},
		LastRejection: &model.KYCRejection{Reason: "模糊", DecidedAt: now, Issues: []model.KYCIssue{
			{Field: "business.tax_id", Code: model.IssueMismatch, Message: "與登記的統編不同"},
			{Field: "business.company_documents", DocumentAssetID: "co-1", Code: model.IssueUnreadable, Message: "反光看不清楚"},
			{Field: "business.company_documents", DocumentAssetID: "co-2", Code: model.IssueExpired, Message: "登記文件已過期"},
		}},
		OutstandingIssues: []model.KYCIssue{{Field: "business.company_documents", DocumentAssetID: "co-1", Code: model.IssueUnreadable, Message: "反光看不清楚"}},
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

// 回應不帶 INPUT_ONLY 的欄位（staged token，含 business 裡的）；補充文件只有 asset ID 與有授權的下載網址，沒有 object key；
// 賣家型態是 subject 設定的那一支；缺的欄位依列舉的數字順序。
func TestKYCResponseCarriesNoInputOnlyFieldsAndNoObjectKeys(t *testing.T) {
	ind := toKYC(context.Background(), fullKYC(model.KYCIndividual))
	if ind.GetIndividual() == nil || ind.GetBusiness() != nil {
		t.Fatalf("individual=%v", ind)
	}
	out := toKYC(context.Background(), fullKYC(model.KYCCompany))
	if out.GetIndividual() != nil || out.GetBusiness().GetTaxId() != "33330333" {
		t.Fatalf("business=%v", out)
	}
	for name, a := range map[string]*pb.KYCApplication{"individual": ind, "business": out} {
		if got := inputOnlyFieldsSet(a.ProtoReflect(), ""); len(got) > 0 {
			t.Errorf("%s：INPUT_ONLY 的欄位出現在回應：%v", name, got)
		}
	}
	if docs := out.GetBusiness().GetCompanyDocuments(); len(docs) != 2 || docs[0].GetAssetId() != "co-1" || docs[1].GetAssetId() != "co-2" ||
		docs[0].GetDownloadUri() != "/api/v1/app/media/secure_download/co-1" {
		t.Fatalf("documents=%v", out)
	}
	if text := out.String() + prototext.Format(out); strings.Contains(text, "secret-key") {
		t.Fatalf("文件的 object key 出現在回應：%s", text)
	}
	if d := out.GetBusiness().GetCompanyRegisterDate(); d.GetYear() != 2015 || d.GetMonth() != 7 || d.GetDay() != 13 || out.GetEtag() != "3" {
		t.Fatalf("out=%v", out)
	}
	rej := out.GetLastRejection()
	wantIssues := []*pb.KYCIssue{
		{Field: pb.KYCField_KYC_FIELD_BUSINESS_TAX_ID, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_MISMATCH, Message: "與登記的統編不同"},
		{Field: pb.KYCField_KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS, DocumentAssetId: "co-1", Code: pb.KYCIssueCode_KYC_ISSUE_CODE_UNREADABLE, Message: "反光看不清楚"},
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
	if want := []pb.KYCField{pb.KYCField_KYC_FIELD_BUSINESS_COMPANY_NAME, pb.KYCField_KYC_FIELD_BUSINESS_TAX_ID}; !slices.Equal(out.GetMissingFields(), want) {
		t.Fatalf("missing_fields=%v", out.GetMissingFields())
	}
}

// SubmitKYC 的身分比對失敗是模組的固定文案：handler 原樣交回，不加欄位、帳號或原因的細節。
func TestSubmitKYCPassesTheGenericIdentityErrorThroughUnchanged(t *testing.T) {
	generic := apperr.FailedPrecondition("無法通過認證，請聯絡客服")
	m := &kycModule{submitErr: generic}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	req := connect.NewRequest(&pb.SubmitKYCRequest{Etag: "2", AcceptedAgreements: []*pb.AgreementAcceptance{
		{Kind: pb.AgreementKind_AGREEMENT_KIND_KYC_PRIVACY_NOTICE, Version: "2026-10-07"},
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
		"KYCStatus":                rpc.EnumMismatches[pb.KYCStatus](model.KYCStatusValues()),
		"KYCField":                 rpc.EnumMismatches[pb.KYCField](fields),
		"PolicyCategory":           rpc.EnumMismatches[commonpb.PolicyCategory](policy.Values()),
		"ViolationSeverity":        rpc.EnumMismatches[pb.ViolationSeverity](model.SeverityValues()),
		"ViolationStatus":          rpc.EnumMismatches[pb.ViolationStatus](model.ViolationStatusValues()),
		"RevokeReason":             rpc.EnumMismatches[pb.RevokeReason](model.RevokeReasonValues()),
		"MeasureKind":              rpc.EnumMismatches[pb.MeasureKind](model.MeasureKindValues()),
		"MeasureBasis":             rpc.EnumMismatches[pb.MeasureBasis](model.BasisValues()),
		"RiskGround":               rpc.EnumMismatches[pb.RiskGround](model.RiskGroundValues()),
		"MeasureState":             rpc.EnumMismatches[pb.MeasureState](model.MeasureStateValues()),
		"MeasureRequestState":      rpc.EnumMismatches[pb.MeasureRequestState](model.RequestStateValues()),
		"MeasureRequestCloseCause": rpc.EnumMismatches[pb.MeasureRequestCloseCause](model.RequestCloseCauseValues()),
	} {
		if len(mismatches) > 0 {
			t.Errorf("%s：%v", name, mismatches)
		}
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

// KYCField 與 KYCApplication 的欄位路徑一對一：退回、缺項與送出的 BadRequest 用同一套語彙。可寫的欄位（"*" 展開，
// 兩支都算）去掉 etag 就是 KYCFieldNames；每個 KYCField 對到真的存在的路徑。
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

func TestStoreFieldsAreMappedOnTheKYCView(t *testing.T) {
	out := toKYC(context.Background(), model.KYCApplication{MissingStoreFields: []string{"support_email", "address", "contact_name"}})
	want := []pb.StoreField{pb.StoreField_STORE_FIELD_ADDRESS, pb.StoreField_STORE_FIELD_CONTACT_NAME, pb.StoreField_STORE_FIELD_SUPPORT_EMAIL}
	if !slices.Equal(out.GetMissingStoreFields(), want) {
		t.Fatalf("missing_store_fields=%v", out.GetMissingStoreFields())
	}
}
