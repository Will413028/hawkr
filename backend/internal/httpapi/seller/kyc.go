package seller

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// kycFields 是模組的草稿輸入的欄位名對應到 KYCApplication 的欄位路徑（與 KYCField 一對一）。
var kycFields = map[string]string{
	"SellerType": "individual", "RealName": "real_name", "BirthDate": "birth_date", "IDCardNo": "individual.id_card_no",
	"IDCardIssueDate": "id_card_issue_date", "IDCardIssueType": "id_card_issue_type",
	"IDCardIssueLocation": "id_card_issue_location", "EnglishName": "english_name", "BankName": "bank_name",
	"BankCode": "bank_code", "BankAccount": "bank_account", "ProductType": "product_type",
	"WebsiteURL":  "website_url",
	"CompanyName": "business.company_name",
	"TaxID":       "business.tax_id", "CompanyRegisterDate": "business.company_register_date", "CompanyTownCode": "business.company_address.town_code",
	"CompanyAddressDetail": "business.company_address.address_detail", "CompanyPhone": "business.company_phone",
	"RepIDCardNo": "business.rep_id_card_no", "IDCardFront": "id_card_front", "IDCardBack": "id_card_back", "BankBook": "bank_book",
	"CompanyDocuments[]": "business.company_documents[]",
}

// kycDraftPaths 是草稿的錯誤指向 SaveKYCDraftRequest.application 的欄位。
func kycDraftPaths() rpc.FieldPaths {
	out := rpc.FieldPaths{"ExpectedVersion": "application.etag"}
	for field, path := range kycFields {
		out[field] = "application." + path
	}
	return out
}

// kycSubmitPaths 是送出時缺少的項目（模組以欄位路徑回報）：BadRequest 的 field 就是 KYCApplication 的欄位路徑（與 KYCField
// 一對一），SubmitKYC 的請求沒有申請本身。店家資料的缺項不是欄位違規（店家的狀態不在請求裡）：回 failed_precondition，缺哪些看
// 申請檢視的 missing_store_fields。同意的文件（AcceptedAgreements）是請求自己的欄位。
func kycSubmitPaths() rpc.FieldPaths {
	out := rpc.FieldPaths{"ExpectedVersion": "etag", "AcceptedAgreements": "accepted_agreements"}
	for _, field := range model.KYCFieldNames() {
		out[field] = field
	}
	return out
}

var (
	kycPaths       = kycDraftPaths()
	submitKYCPaths = kycSubmitPaths()
	cancelKYCPaths = rpc.FieldPaths{"ExpectedVersion": "etag", "Reason": "reason"}
)

// secureDownloadURI 是申請證件的下載網址：目前是 gateway 的有授權下載（owner 驗證，只有上傳者能下載）；證件圖片的
// media 引用改到業務列之後（B1）換成新的下載方式時只改這裡。
func secureDownloadURI(assetID string) string {
	return "/api/v1/app/media/secure_download/" + url.PathEscape(assetID)
}

func (h *App) SaveKYCDraft(ctx context.Context, req *connect.Request[pb.SaveKYCDraftRequest]) (*connect.Response[pb.SaveKYCDraftResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	app := req.Msg.GetApplication()
	if app == nil {
		return nil, rpc.RequiredField("application", "application 必填")
	}
	// etag 不帶是建立（店家還沒有申請）；已經有申請時模組回 already_exists（VERSION_CONFLICT）。
	version, err := rpc.VersionFromEtag(app.GetEtag())
	if err != nil {
		return nil, err
	}
	in, err := kycDraftInput(app, req.Msg.GetUpdateMask(), version == 0)
	if err != nil {
		return nil, err
	}
	in.AccountID, in.ExpectedVersion = current.AccountID, version
	saved, err := h.m.SaveKYCDraft(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, kycPaths)
	}
	return connect.NewResponse(&pb.SaveKYCDraftResponse{Application: toKYC(ctx, saved)}), nil
}

// kycDraftInput 把 update_mask 選到的欄位轉成模組的部分更新：沒有選到的欄位是 nil（保留原值），選到卻沒有值的是空字串
// （清成沒有值）。目前的值不在請求裡（身分證號與銀行帳號只能讀到遮蔽後的值），所以不能先合併再整筆送（API 設計規範 §七）。
// 可寫的欄位集合由 proto 的 descriptor 決定（rpc.WritableFields），這裡沒有「哪個型態有哪些欄位」的清單：賣家型態是 subject
// 設定的那一支。建立時必須設定其中一支；update_mask 列到的欄位在另一支底下是請求自相矛盾；設定的那一支與目前的型態不同
// 時，模組切換型態並清掉另一型態專屬的欄位。
func kycDraftInput(app *pb.KYCApplication, mask *fieldmaskpb.FieldMask, create bool) (model.KYCDraftInput, error) {
	var in model.KYCDraftInput
	paths, err := rpc.WritableFields(app, mask)
	if err != nil {
		return in, err
	}
	has := func(path string) bool { return slices.Contains(paths, path) }
	touches := func(prefix string) bool {
		return slices.ContainsFunc(paths, func(p string) bool { return strings.HasPrefix(p, prefix) })
	}
	individual, business := app.GetIndividual(), app.GetBusiness()
	switch {
	case touches("individual.") && app.GetSubject() != nil && business != nil,
		touches("business.") && app.GetSubject() != nil && individual != nil:
		return in, rpc.Error(connect.CodeInvalidArgument, "update_mask 列到的欄位與設定的 individual 或 business 不一致")
	case touches("individual.") && touches("business."):
		return in, rpc.Error(connect.CodeInvalidArgument, "update_mask 不能同時列到 individual 與 business 的欄位")
	}
	switch {
	case individual != nil && (create || touches("individual.")):
		t := model.KYCIndividual
		in.SellerType = &t
	case business != nil && (create || touches("business.")):
		t := model.KYCCompany
		in.SellerType = &t
	case create:
		return in, rpc.RequiredField("application.individual", "建立申請時 individual 或 business 必須設定其中一支")
	}
	text := func(path string, dst **string, v string) {
		if has(path) {
			*dst = &v
		}
	}
	day := func(path string, dst **string, d *date.Date) {
		if !has(path) {
			return
		}
		v := ""
		if d != nil {
			v = fmt.Sprintf("%04d-%02d-%02d", d.GetYear(), d.GetMonth(), d.GetDay())
		}
		*dst = &v
	}
	single := func(path string, dst **model.KYCDocumentRef, d *pb.KYCDocument) {
		if has(path) {
			*dst = ptr(documentRef(d))
		}
	}
	text("real_name", &in.RealName, app.GetRealName())
	day("birth_date", &in.BirthDate, app.GetBirthDate())
	day("id_card_issue_date", &in.IDCardIssueDate, app.GetIdCardIssueDate())
	text("id_card_issue_type", &in.IDCardIssueType, app.GetIdCardIssueType())
	text("id_card_issue_location", &in.IDCardIssueLocation, app.GetIdCardIssueLocation())
	text("english_name", &in.EnglishName, app.GetEnglishName())
	text("bank_name", &in.BankName, app.GetBankName())
	text("bank_code", &in.BankCode, app.GetBankCode())
	text("bank_account", &in.BankAccount, app.GetBankAccount())
	text("product_type", &in.ProductType, app.GetProductType())
	text("website_url", &in.WebsiteURL, app.GetWebsiteUrl())
	single("id_card_front", &in.IDCardFront, app.GetIdCardFront())
	single("id_card_back", &in.IDCardBack, app.GetIdCardBack())
	single("bank_book", &in.BankBook, app.GetBankBook())
	text("individual.id_card_no", &in.IDCardNo, individual.GetIdCardNo())
	text("business.company_name", &in.CompanyName, business.GetCompanyName())
	text("business.tax_id", &in.TaxID, business.GetTaxId())
	day("business.company_register_date", &in.CompanyRegisterDate, business.GetCompanyRegisterDate())
	if has("business.company_address") {
		if addr := business.GetCompanyAddress(); addr != nil {
			townCode := addr.GetTownCode()
			addressDetail := addr.GetAddressDetail()
			in.CompanyTownCode = &townCode
			in.CompanyAddressDetail = &addressDetail
		} else {
			empty := ""
			in.CompanyTownCode = &empty
			in.CompanyAddressDetail = &empty
		}
	}
	text("business.company_phone", &in.CompanyPhone, business.GetCompanyPhone())
	text("business.rep_id_card_no", &in.RepIDCardNo, business.GetRepIdCardNo())
	if has("business.company_documents") {
		refs := make([]model.KYCDocumentRef, 0, len(business.GetCompanyDocuments()))
		for _, d := range business.GetCompanyDocuments() {
			refs = append(refs, documentRef(d))
		}
		in.CompanyDocuments = &refs
	}
	return in, nil
}

// documentRef 是請求裡一張證件的指定；沒有值（nil 訊息）是空的指定：單張的位置清掉。
func documentRef(d *pb.KYCDocument) model.KYCDocumentRef {
	return model.KYCDocumentRef{AssetID: d.GetAssetId(), StagedToken: d.GetStagedToken()}
}

func (h *App) GetMyKYC(ctx context.Context, _ *connect.Request[pb.GetMyKYCRequest]) (*connect.Response[pb.GetMyKYCResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	app, err := h.m.MyKYC(ctx, current.AccountID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetMyKYCResponse{Application: toKYC(ctx, app)}), nil
}

// SubmitKYC 的身分比對沒有通過時，模組回固定文字的 failed_precondition（不透露欄位或帳號）；這裡原樣交回，不加任何細節。
func (h *App) SubmitKYC(ctx context.Context, req *connect.Request[pb.SubmitKYCRequest]) (*connect.Response[pb.SubmitKYCResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	v, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	submitted, err := h.m.SubmitKYC(ctx, model.SubmitKYCInput{
		AccountID: current.AccountID, ExpectedVersion: v,
		AcceptedAgreements: acceptances(req.Msg.GetAcceptedAgreements()),
		ClientIP:           rpc.ClientIPFrom(ctx), UserAgent: req.Header().Get("User-Agent"),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, submitKYCPaths)
	}
	return connect.NewResponse(&pb.SubmitKYCResponse{Application: toKYC(ctx, submitted)}), nil
}

func (h *App) CancelKYC(ctx context.Context, req *connect.Request[pb.CancelKYCRequest]) (*connect.Response[pb.CancelKYCResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	v, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	cancelled, err := h.m.CancelKYC(ctx, model.CancelKYCInput{AccountID: current.AccountID, ExpectedVersion: v, Reason: req.Msg.GetReason()})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, cancelKYCPaths)
	}
	return connect.NewResponse(&pb.CancelKYCResponse{Application: toKYC(ctx, cancelled)}), nil
}

// toKYC 轉成契約的申請：只有遮蔽後的值（模組的型別沒有明文欄位），證件只有 asset ID 與有授權的下載網址。
func toKYC(ctx context.Context, a model.KYCApplication) *pb.KYCApplication {
	out := &pb.KYCApplication{
		Id: a.ID.String(), Status: rpc.EnumToProto[pb.KYCStatus](a.Status), Etag: rpc.Etag(a.Version),
		RealName: deref(a.RealName), BirthDate: toDate(a.BirthDate),
		IdCardIssueDate: toDate(a.IDCardIssueDate), IdCardIssueType: deref(a.IDCardIssueType), IdCardIssueLocation: deref(a.IDCardIssueLocation),
		EnglishName: deref(a.EnglishName), BankName: deref(a.BankName), BankCode: deref(a.BankCode), BankAccountMasked: deref(a.BankAccountMasked),
		ProductType: deref(a.ProductType),
		WebsiteUrl:  deref(a.WebsiteURL),
		CreateTime:  timestamppb.New(a.CreatedAt), UpdateTime: timestamppb.New(a.UpdatedAt),
	}
	switch a.SellerType {
	case model.KYCIndividual:
		out.Subject = &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{IdCardNoMasked: deref(a.IDCardNoMasked)}}
	case model.KYCCompany:
		b := &pb.BusinessKYC{
			CompanyName: deref(a.CompanyName), TaxId: deref(a.TaxID),
			CompanyRegisterDate: toDate(a.CompanyRegisterDate), CompanyPhone: deref(a.CompanyPhone), RepIdCardNoMasked: deref(a.RepIDCardNoMasked),
		}
		if a.CompanyTownCode != nil {
			b.CompanyAddress = &commonpb.PostalAddress{
				TownCode:      *a.CompanyTownCode,
				AddressDetail: deref(a.CompanyAddressDetail),
			}
		}
		out.Subject = &pb.KYCApplication_Business{Business: b}
	}
	for _, f := range a.MissingFields {
		out.MissingFields = append(out.MissingFields, kycFieldPB(f))
	}
	slices.Sort(out.MissingFields)
	for _, f := range a.MissingStoreFields {
		out.MissingStoreFields = append(out.MissingStoreFields, storeFieldPB(f))
	}
	slices.Sort(out.MissingStoreFields)
	for _, d := range a.Documents {
		doc := &pb.KYCDocument{AssetId: d.AssetID, DownloadUri: secureDownloadURI(d.AssetID)}
		switch d.Slot {
		case model.SlotIDCardFront:
			out.IdCardFront = doc
		case model.SlotIDCardBack:
			out.IdCardBack = doc
		case model.SlotBankBook:
			out.BankBook = doc
		case model.SlotCompanyDocument:
			if business := out.GetBusiness(); business != nil {
				business.CompanyDocuments = append(business.CompanyDocuments, doc)
			}
		}
	}
	if a.Flow != nil {
		out.Flow = toSellerFlow(ctx, *a.Flow, a)
	}
	if r := a.LastRejection; r != nil {
		out.LastRejection = &pb.KYCRejection{Reason: r.Reason, DecideTime: timestamppb.New(r.DecidedAt), Issues: toKYCIssues(r.Issues)}
	}
	out.OutstandingIssues = toKYCIssues(a.OutstandingIssues)
	if a.SubmittedAt != nil {
		out.SubmitTime = timestamppb.New(*a.SubmittedAt)
	}
	if a.DecidedAt != nil {
		out.DecideTime = timestamppb.New(*a.DecidedAt)
	}
	return out
}

// toKYCIssues 轉成契約的問題；沒有問題時是 nil。
func toKYCIssues(issues []model.KYCIssue) []*pb.KYCIssue {
	var out []*pb.KYCIssue
	for _, issue := range issues {
		out = append(out, &pb.KYCIssue{
			Field: kycFieldPB(issue.Field), DocumentAssetId: issue.DocumentAssetID,
			Code: rpc.EnumToProto[pb.KYCIssueCode](issue.Code), Message: issue.Message,
		})
	}
	return out
}

// kycFieldPB 把申請的欄位路徑（model.KYCFieldNames，巢狀路徑以 . 分隔）轉成契約的列舉。
func kycFieldPB(name string) pb.KYCField {
	return rpc.EnumToProto[pb.KYCField](strings.ToUpper(strings.ReplaceAll(name, ".", "_")))
}

// storeFieldPB 把店家的欄位路徑（model.StoreFieldNames）轉成契約的列舉。
func storeFieldPB(name string) pb.StoreField {
	return rpc.EnumToProto[pb.StoreField](strings.ToUpper(name))
}

// toFlow 是流程狀態的共用部分（賣家與後台）；賣家受眾的 payuni 與 resubmit_unavailabilities 只在 toSellerFlow 設。
func toFlow(f model.KYCFlow) *pb.KYCFlow {
	return &pb.KYCFlow{Status: rpc.EnumToProto[pb.KYCFlowStatus](f.Status), CanResubmit: f.CanResubmit, IdentityLocked: f.IdentityLocked}
}

// toSellerFlow 是賣家讀自己的申請時的流程狀態：共用部分加上賣家看到的 PayUni 狀態與不能重新送出的原因（句子的語系取自 ctx）。
func toSellerFlow(ctx context.Context, f model.KYCFlow, a model.KYCApplication) *pb.KYCFlow {
	out := toFlow(f)
	l := uitext.LocaleFrom(ctx)
	for _, r := range a.ResubmitReasons {
		out.ResubmitUnavailabilities = append(out.ResubmitUnavailabilities, &pb.KYCResubmitUnavailability{
			Reason: rpc.EnumToProto[pb.KYCResubmitUnavailableReason](r), Message: rpc.LocalizedMessage(r.Text(l)),
		})
	}
	if p := a.PayUni; p != nil {
		out.Payuni = &pb.SellerPayUni{State: rpc.EnumToProto[pb.PayUniState](p.State), Reason: p.Reason}
		if m, ok := p.State.Explanation(l); ok {
			out.Payuni.Explanation = rpc.LocalizedMessage(m)
		}
		for _, f := range p.FixFields {
			out.Payuni.FixFields = append(out.Payuni.FixFields, kycFieldPB(f))
		}
		for _, f := range p.FixStoreFields {
			out.Payuni.FixStoreFields = append(out.Payuni.FixStoreFields, storeFieldPB(f))
		}
	}
	return out
}

func toProblem(p *model.PayUniProblem) *pb.PayUniProblem {
	if p == nil {
		return nil
	}
	out := &pb.PayUniProblem{Code: p.Code, Reason: deref(p.Reason)}
	for _, f := range p.Fields {
		out.Fields = append(out.Fields, kycFieldPB(f))
	}
	for _, f := range p.StoreFields {
		out.StoreFields = append(out.StoreFields, storeFieldPB(f))
	}
	return out
}

// toDate 把模組的日曆日（YYYY-MM-DD）轉成 google.type.Date；沒有值或格式不合時不設。
func toDate(s *string) *date.Date {
	if s == nil {
		return nil
	}
	d, err := time.Parse(time.DateOnly, *s)
	if err != nil {
		return nil
	}
	return &date.Date{Year: int32(d.Year()), Month: int32(d.Month()), Day: int32(d.Day())} //nolint:gosec // time.Time 的日曆欄位在 Date 的範圍內
}

func ptr[T any](v T) *T { return &v }
