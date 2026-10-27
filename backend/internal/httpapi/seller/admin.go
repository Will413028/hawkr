package seller

import (
	"context"
	"strconv"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// AdminModule 是後台 handler 使用的店家讀寫介面。
type AdminModule interface {
	Store(ctx context.Context, storeID int64) (model.Store, error)
	AdminStores(context.Context, model.StoreListInput) (model.StorePage, error)
	AdminStoreStats(context.Context) (model.StoreStats, error)
	StoreKYC(ctx context.Context, storeID int64) (model.KYCApplication, bool, error)
	StoreWithActiveMeasures(ctx context.Context, storeID int64) (model.StoreMeasures, error)
	AdminKYCApplications(context.Context, model.KYCListInput) (model.KYCPage, error)
	AdminKYCApplication(ctx context.Context, applicationID uuid.UUID) (model.KYCAdminApplication, error)
	ListKYCReveals(context.Context, model.KYCAuditListInput) (model.KYCRevealPage, error)
	ListKYCDocumentAccesses(context.Context, model.KYCAuditListInput) (model.KYCDocumentAccessPage, error)
	ApproveKYC(context.Context, model.ApproveKYCInput) (model.KYCAdminApplication, error)
	RejectKYC(context.Context, model.RejectKYCInput) (model.KYCAdminApplication, error)
	RevealKYCSensitive(context.Context, model.RevealKYCInput) (model.KYCSensitive, error)
	PayUniRegistrationOf(ctx context.Context, storeID int64) (model.AdminPayUniRegistration, error)
	RebindPayUni(context.Context, model.RebindPayUniInput) (model.AdminPayUniRegistration, error)
	PayUniObservations(context.Context, model.PayUniObservationListInput) (model.PayUniObservationPage, error)
	RefreshPayUniStatus(ctx context.Context, storeID int64) error
	RequeryKYCRegistration(context.Context, model.RequeryKYCRegistrationInput) (model.RegistrationCheck, error)
}

// Admin 是 SellerAdminService 的 handler。授權（只有管理員）由 rpc 的 interceptor 依 service 名稱檢查。
type Admin struct {
	m      AdminModule
	images rpc.Images
}

var _ sellerv1alpha1connect.SellerAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。措施此刻的狀態由模組算好（Measure.State），handler 沒有時鐘。
func NewAdmin(m AdminModule, images rpc.Images) *Admin {
	return &Admin{m: m, images: images}
}

// 模組輸入的欄位對應到請求的路徑。
var (
	listStoresPaths = rpc.FieldPaths{"Statuses": "status", "KYCStatus": "kyc_status"}
)

// adminFrom 取出呼叫的管理員；interceptor 已經依 service 名稱擋下非管理員，到這裡沒有是授權鏈壞了。
func adminFrom(ctx context.Context) (int64, error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAdmin {
		return 0, errNoPrincipal
	}
	return p.ID, nil
}

func (h *Admin) toStore(s model.Store) (*adminpb.Store, error) { return toAdminStore(h.images, s) }

// toAdminStore 轉成後台看到的店家（SellerAdminService 與治理的轉報資料共用）。
func toAdminStore(images rpc.Images, s model.Store) (*adminpb.Store, error) {
	out := &adminpb.Store{
		Id: rpc.FormatID(s.StoreID), StoreCode: rpc.FormatID(s.StoreCode), AccountId: rpc.FormatID(s.AccountID),
		StoreName: s.StoreName, Description: deref(s.Description), ContactPhone: deref(s.ContactPhone),
		ContactName: deref(s.ContactName), ContactMobile: deref(s.ContactMobile), SupportEmail: deref(s.SupportEmail),
		Prize: deref(s.Prize), WebsiteUrl: deref(s.WebsiteURL),
		OnboardingCompleted: s.OnboardingCompleted,
		Status:              rpc.EnumToProto[pb.StoreStatus](s.Status),
		CreateTime:          timestamppb.New(s.CreatedAt), UpdateTime: timestamppb.New(s.UpdatedAt),
	}
	if s.TownCode != nil {
		out.Address = &commonpb.PostalAddress{
			TownCode:      *s.TownCode,
			AddressDetail: deref(s.AddressDetail),
		}
	}
	if s.Closure != nil {
		out.CloseTime = timestamppb.New(s.Closure.ClosedAt)
		out.Closure = &adminpb.Closure{Closer: rpc.EnumToProto[adminpb.StoreCloser](s.Closure.By)}
		if s.Closure.MeasureID != nil {
			out.Closure.MeasureId = s.Closure.MeasureID.String()
		}
	}
	for _, image := range []struct {
		key *string
		dst **commonpb.Image
	}{{s.LogoKey, &out.Logo}, {s.CoverKey, &out.CoverImage}} {
		if image.key == nil {
			continue
		}
		if err := rpc.ObjectKey(*image.key); err != nil {
			return nil, err
		}
		*image.dst = images.Image(*image.key)
	}
	return out, nil
}

// suspendedScope 是 suspended 篩選在 page token 指紋裡的樣子（沒有設定與 false 要分得開）。
func suspendedScope(v *bool) string {
	switch {
	case v == nil:
		return "-"
	case *v:
		return "true"
	}
	return "false"
}

func (h *Admin) ListStores(ctx context.Context, req *connect.Request[adminpb.ListStoresRequest]) (*connect.Response[adminpb.ListStoresResponse], error) {
	msg := req.Msg
	in := model.StoreListInput{Query: msg.GetQuery(), PageSize: msg.GetPageSize(), KYCAwaitingReview: msg.GetKycAwaitingReview()}
	if msg.GetAccountId() != "" {
		accountID, err := rpc.ParseID("account_id", msg.GetAccountId())
		if err != nil {
			return nil, err
		}
		in.AccountID = &accountID
	}
	if bind := rpc.EnumFromProto[model.PayUniBindStatus](msg.GetPayuniBindStatus()); bind != "" {
		in.PayUniBindStatus = &bind
	}
	if msg.Suspended != nil {
		suspended := msg.GetSuspended()
		in.Suspended = &suspended
	}
	if status := rpc.EnumFromProto[model.Status](msg.GetStatus()); status != "" {
		in.Statuses = []model.Status{status}
	}
	if kyc := rpc.EnumFromProto[model.KYCStatus](msg.GetKycStatus()); kyc != "" {
		in.KYCStatus = &kyc
	}
	// page token 只能用在同一組篩選：換了篩選就從第一頁開始。
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerAdminServiceListStoresProcedure, in.Query, string(rpc.EnumFromProto[model.Status](msg.GetStatus())), msg.GetKycStatus().String(), msg.GetAccountId(), msg.GetPayuniBindStatus().String(), suspendedScope(msg.Suspended), strconv.FormatBool(msg.GetKycAwaitingReview()))
	after, err := rpc.ValidCursor[model.StoreCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in.After = after
	page, err := h.m.AdminStores(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listStoresPaths)
	}
	out := &adminpb.ListStoresResponse{Stores: make([]*adminpb.StoreListItem, len(page.Items))}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	for i, item := range page.Items {
		store, err := h.toStore(item.Store)
		if err != nil {
			return nil, err
		}
		out.Stores[i] = &adminpb.StoreListItem{
			Store: store, Suspended: item.Store.Suspended(),
			DelegationUnavailabilities: toStoreActionUnavailabilities(item.Store.ActionUnavailableReasons(model.StoreActionDelegate), model.StoreActionDelegate),
			KycFlowStatus:              rpc.EnumToProto[pb.KYCFlowStatus](item.KYCFlow),
		}
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) GetStoreStats(ctx context.Context, _ *connect.Request[adminpb.GetStoreStatsRequest]) (*connect.Response[adminpb.GetStoreStatsResponse], error) {
	stats, err := h.m.AdminStoreStats(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetStoreStatsResponse{
		TotalStores: stats.Total, KycAwaitingReviewStores: stats.KYCAwaitingReview, KycApprovedStores: stats.KYCApproved,
		KycRejectedStores: stats.KYCRejected, NewStoresThisMonth: stats.NewThisMonth,
	}), nil
}

func (h *Admin) GetStore(ctx context.Context, req *connect.Request[adminpb.GetStoreRequest]) (*connect.Response[adminpb.GetStoreResponse], error) {
	id, err := rpc.ParseID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	// 店家與它生效中的措施是同一次讀取：停權狀態與清單來自同一個快照。
	state, err := h.m.StoreWithActiveMeasures(ctx, id)
	if err != nil {
		return nil, err
	}
	store, err := h.toStore(state.Store)
	if err != nil {
		return nil, err
	}
	out := &adminpb.GetStoreResponse{
		Store: store, ActiveMeasures: toAdminMeasures(state.Active),
		CanCreateTermination:          len(state.Terminations) > 0,
		CanCreateMeasure:              len(state.CreateMeasure) == 0,
		CreateMeasureUnavailabilities: toStoreActionUnavailabilities(state.CreateMeasure, model.StoreActionCreateMeasure),
	}
	for _, option := range state.Terminations {
		out.AvailableTerminations = append(out.AvailableTerminations, toTerminationOption(option))
	}
	application, ok, err := h.m.StoreKYC(ctx, id)
	if err != nil {
		return nil, err
	}
	if ok {
		out.Kyc = &adminpb.KYCSummary{
			ApplicationId: application.ID.String(), Status: rpc.EnumToProto[pb.KYCStatus](application.Status),
		}
		if application.Flow != nil {
			out.Kyc.FlowStatus = rpc.EnumToProto[pb.KYCFlowStatus](application.Flow.Status)
		}
		if application.SubmittedAt != nil {
			out.Kyc.SubmitTime = timestamppb.New(*application.SubmittedAt)
		}
		if application.DecidedAt != nil {
			out.Kyc.DecideTime = timestamppb.New(*application.DecidedAt)
		}
		out.Kyc.DecideDeadline = toDeadline(application.DecideDeadline)
	}
	return connect.NewResponse(out), nil
}

// toStoreActionUnavailabilities 轉出 admin 不能對店家做 action 的每個原因與它的文字（model.StoreActionUnavailableText，與方法被拒絕時錯誤的
// LocalizedMessage 同一個函式）。idempotent 的原因（PAYUNI_BIND_IN_PROGRESS）不是失敗，沒有句子，message 不設。
func toStoreActionUnavailabilities(reasons []model.StoreActionUnavailableReason, action model.StoreAction) []*adminpb.StoreActionUnavailability {
	var out []*adminpb.StoreActionUnavailability
	for _, r := range reasons {
		item := &adminpb.StoreActionUnavailability{Reason: rpc.EnumToProto[adminpb.StoreActionUnavailableReason](r)}
		if msg := model.StoreActionUnavailableText(r, action); msg.Text != "" {
			item.Message = rpc.LocalizedMessage(msg)
		}
		out = append(out, item)
	}
	return out
}
