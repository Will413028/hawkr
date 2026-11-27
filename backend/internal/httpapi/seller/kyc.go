package seller

import (
	"context"
	"fmt"
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
)

var kycFields = map[string]string{
	"SellerType": "individual", "ProductType": "product_type",
	"WebsiteURL":  "website_url",
	"CompanyName": "business.company_name",
	"TaxID":       "business.tax_id", "CompanyRegisterDate": "business.company_register_date", "CompanyTownCode": "business.company_address.town_code",
	"CompanyAddressDetail": "business.company_address.address_detail", "CompanyPhone": "business.company_phone",
	"CompanyDocuments[]": "business.company_documents[]",
}

func kycDraftPaths() rpc.FieldPaths {
	out := rpc.FieldPaths{"ExpectedVersion": "application.etag"}
	for field, path := range kycFields {
		out[field] = "application." + path
	}
	return out
}

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

func (h *App) SaveKYCDraft(ctx context.Context, req *connect.Request[pb.SaveKYCDraftRequest]) (*connect.Response[pb.SaveKYCDraftResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	app := req.Msg.GetApplication()
	if app == nil {
		return nil, rpc.RequiredField("application", "application 必填")
	}
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
	maskLists := func(path string) bool { return mask != nil && slices.Contains(mask.GetPaths(), path) }
	touchesBranch := func(branch string) bool { return touches(branch+".") || maskLists(branch) }
	selected := func(branch string) bool {
		return touchesBranch(branch) || maskLists("*") || len(mask.GetPaths()) == 0
	}
	switch {
	case touchesBranch("individual") && business != nil,
		touchesBranch("business") && individual != nil:
		return in, rpc.Error(connect.CodeInvalidArgument, "update_mask 列到的欄位與設定的 individual 或 business 不一致")
	case touchesBranch("individual") && individual == nil,
		touchesBranch("business") && business == nil:
		return in, rpc.Error(connect.CodeInvalidArgument, "update_mask 列到的 individual 或 business 沒有設定")
	case touchesBranch("individual") && touchesBranch("business"):
		return in, rpc.Error(connect.CodeInvalidArgument, "update_mask 不能同時列到 individual 與 business 的欄位")
	}
	switch {
	case individual != nil && (create || selected("individual")):
		t := model.KYCIndividual
		in.SellerType = &t
	case business != nil && (create || selected("business")):
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
	text("product_type", &in.ProductType, app.GetProductType())
	text("website_url", &in.WebsiteURL, app.GetWebsiteUrl())
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
	if has("business.company_documents") {
		refs := make([]model.KYCDocumentRef, 0, len(business.GetCompanyDocuments()))
		for _, d := range business.GetCompanyDocuments() {
			refs = append(refs, documentRef(d))
		}
		in.CompanyDocuments = &refs
	}
	return in, nil
}

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

func toKYC(ctx context.Context, a model.KYCApplication) *pb.KYCApplication {
	out := &pb.KYCApplication{
		Id: a.ID.String(), Status: rpc.EnumToProto[pb.KYCStatus](a.Status), Etag: rpc.Etag(a.Version),
		ProductType: deref(a.ProductType),
		WebsiteUrl:  deref(a.WebsiteURL),
		CreateTime:  timestamppb.New(a.CreatedAt), UpdateTime: timestamppb.New(a.UpdatedAt),
	}
	switch a.SellerType {
	case model.KYCIndividual:
		out.Subject = &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{}}
	case model.KYCCompany:
		b := &pb.BusinessKYC{
			CompanyName: deref(a.CompanyName), TaxId: deref(a.TaxID),
			CompanyRegisterDate: toDate(a.CompanyRegisterDate), CompanyPhone: deref(a.CompanyPhone),
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
		doc := &pb.KYCDocument{AssetId: d.AssetID, DownloadUri: ownDocumentURI(d.Position)}
		if business := out.GetBusiness(); business != nil {
			business.CompanyDocuments = append(business.CompanyDocuments, doc)
		}
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

func kycFieldPB(name string) pb.KYCField {
	return rpc.EnumToProto[pb.KYCField](strings.ToUpper(strings.ReplaceAll(name, ".", "_")))
}

func storeFieldPB(name string) pb.StoreField {
	return rpc.EnumToProto[pb.StoreField](strings.ToUpper(name))
}

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
