package seller

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 模組輸入的欄位對應到請求的路徑。
var (
	submitFoodRegistrationPaths = rpc.FieldPaths{"RegistrationNumber": "registration_number", "AcceptedAgreements": "accepted_agreements"}
	decideFoodRegistrationPaths = rpc.FieldPaths{"Reason": "reason", "InternalNote": "internal_note", "Outcome": "decision"}
	listFoodRegistrationPaths   = rpc.FieldPaths{"State": "state", "Inbox": "inbox"}
	revokeFoodRegistrationPaths = rpc.FieldPaths{"Reason": "reason"}
)

// toFoodRegistration 是賣家看得到的一筆登錄（核准中、待審或被退回、被平台撤銷的那一筆）；沒有時回 nil。
func toFoodRegistration(r *model.FoodRegistration) *pb.FoodRegistration {
	if r == nil {
		return nil
	}
	out := &pb.FoodRegistration{
		Id: r.ID.String(), State: rpc.EnumToProto[pb.FoodRegistrationState](r.State), RegistrationNumber: r.RegistrationNumber,
		SubmitTime: timestamppb.New(r.SubmittedAt), RejectReason: r.RejectReason, EndReason: r.EndReason,
	}
	if r.DecidedAt != nil {
		out.DecideTime = timestamppb.New(*r.DecidedAt)
	}
	if r.EndedAt != nil {
		out.EndTime = timestamppb.New(*r.EndedAt)
	}
	return out
}

// toAdminStoreFoodRegistration 是後台店家上的一筆登錄：只帶登錄本身的欄位，審核用的欄位（KYC、主體、內部註記、裁定者）在 GetFoodRegistration；沒有時回 nil。
func toAdminStoreFoodRegistration(storeID int64, r *model.FoodRegistration) *adminpb.FoodRegistration {
	if r == nil {
		return nil
	}
	out := &adminpb.FoodRegistration{
		Id: r.ID.String(), StoreId: rpc.FormatID(storeID), State: rpc.EnumToProto[adminpb.FoodRegistrationState](r.State),
		RegistrationNumber: r.RegistrationNumber, SubmitTime: timestamppb.New(r.SubmittedAt), RejectReason: r.RejectReason, EndReason: r.EndReason,
	}
	if r.DecidedAt != nil {
		out.DecideTime = timestamppb.New(*r.DecidedAt)
	}
	if r.EndedAt != nil {
		out.EndTime = timestamppb.New(*r.EndedAt)
	}
	return out
}

func (h *App) SubmitFoodRegistration(ctx context.Context, req *connect.Request[pb.SubmitFoodRegistrationRequest]) (*connect.Response[pb.SubmitFoodRegistrationResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := h.m.SubmitFoodRegistration(ctx, model.SubmitFoodRegistrationInput{
		StoreID: current.StoreID, RegistrationNumber: req.Msg.GetRegistrationNumber(),
		AcceptedAgreements: acceptances(req.Msg.GetAcceptedAgreements()),
		ClientIP:           rpc.ClientIPFrom(ctx), UserAgent: req.Header().Get("User-Agent"),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, submitFoodRegistrationPaths)
	}
	return connect.NewResponse(&pb.SubmitFoodRegistrationResponse{Store: h.toStore(updated)}), nil
}

func (h *App) WithdrawFoodRegistration(ctx context.Context, _ *connect.Request[pb.WithdrawFoodRegistrationRequest]) (*connect.Response[pb.WithdrawFoodRegistrationResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := h.m.WithdrawFoodRegistration(ctx, current.StoreID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.WithdrawFoodRegistrationResponse{Store: h.toStore(updated)}), nil
}

func (h *App) ClearFoodRegistration(ctx context.Context, _ *connect.Request[pb.ClearFoodRegistrationRequest]) (*connect.Response[pb.ClearFoodRegistrationResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := h.m.ClearFoodRegistration(ctx, current.StoreID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ClearFoodRegistrationResponse{Store: h.toStore(updated)}), nil
}

// toAdminFoodRegistration 是後台看到的一筆登錄。
func toAdminFoodRegistration(r model.FoodRegistrationReview) *adminpb.FoodRegistration {
	out := &adminpb.FoodRegistration{
		Id: r.ID.String(), StoreId: rpc.FormatID(r.StoreID), StoreName: r.StoreName,
		StoreStatus:        rpc.EnumToProto[pb.StoreStatus](r.StoreStatus),
		RegistrationNumber: r.RegistrationNumber, State: rpc.EnumToProto[adminpb.FoodRegistrationState](r.State),
		SubmitTime: timestamppb.New(r.SubmittedAt), RejectReason: r.RejectReason, InternalNote: r.InternalNote,
		EndCause: rpc.EnumToProto[adminpb.FoodRegistrationEndCause](r.EndCause), Approved: toAdminStoreFoodRegistration(r.StoreID, r.Approved),
		CanApprove: r.CanApprove(), CanReject: r.CanReject(),
		ApproveUnavailabilities: toStoreActionUnavailabilities(r.ApproveUnavailable(), model.StoreActionApproveFoodRegistration),
		RejectUnavailabilities:  toStoreActionUnavailabilities(r.RejectUnavailable(), model.StoreActionRejectFoodRegistration),
		CanRevoke:               r.CanRevoke(), EndReason: r.EndReason, DecideDeadline: toDeadline(r.DecideDeadline),
	}
	if r.EndedByAdminID != nil {
		out.RevokerAdminId = rpc.FormatID(*r.EndedByAdminID)
	}
	if r.DecidedByAdminID != nil {
		out.DeciderAdminId = rpc.FormatID(*r.DecidedByAdminID)
	}
	if r.DecidedAt != nil {
		out.DecideTime = timestamppb.New(*r.DecidedAt)
	}
	if r.EndedAt != nil {
		out.EndTime = timestamppb.New(*r.EndedAt)
	}
	if r.KYCStatus != nil {
		status := rpc.EnumToProto[pb.KYCStatus](*r.KYCStatus)
		out.KycStatus = &status
	}
	if r.Subject != nil {
		out.Subject = &adminpb.FoodRegistrationSubject{
			SellerType: rpc.EnumToProto[adminpb.KYCSellerType](r.Subject.SellerType), RealName: r.Subject.RealName,
			CompanyName: r.Subject.CompanyName, TaxId: r.Subject.TaxID,
		}
	}
	return out
}

func (h *Admin) ListFoodRegistrations(ctx context.Context, req *connect.Request[adminpb.ListFoodRegistrationsRequest]) (*connect.Response[adminpb.ListFoodRegistrationsResponse], error) {
	msg := req.Msg
	in := model.FoodRegistrationListInput{Query: msg.GetQuery(), PageSize: msg.GetPageSize()}
	if state := rpc.EnumFromProto[model.FoodRegState](msg.GetState()); state != "" {
		in.State = &state
	}
	if inbox := rpc.EnumFromProto[model.FoodRegInbox](msg.GetInbox()); inbox != "" {
		in.Inbox = &inbox
	}
	// page token 只能用在同一組篩選：收件匣與歷史的排序不同、兩個隊列的內容不同，換了篩選就從第一頁開始。
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerAdminServiceListFoodRegistrationsProcedure, msg.GetState().String(), msg.GetInbox().String(), in.Query)
	after, err := rpc.ValidCursor[model.FoodRegistrationCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in.After = after
	page, err := h.m.FoodRegistrations(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listFoodRegistrationPaths)
	}
	out := &adminpb.ListFoodRegistrationsResponse{Registrations: make([]*adminpb.FoodRegistration, len(page.Items))}
	for i, item := range page.Items {
		out.Registrations[i] = toAdminFoodRegistration(item)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) GetFoodRegistration(ctx context.Context, req *connect.Request[adminpb.GetFoodRegistrationRequest]) (*connect.Response[adminpb.GetFoodRegistrationResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	review, err := h.m.FoodRegistrationReview(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetFoodRegistrationResponse{Registration: toAdminFoodRegistration(review)}), nil
}

func (h *Admin) DecideFoodRegistration(ctx context.Context, req *connect.Request[adminpb.DecideFoodRegistrationRequest]) (*connect.Response[adminpb.DecideFoodRegistrationResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	var outcome model.FoodRegState
	switch req.Msg.GetDecision() {
	case adminpb.FoodRegistrationDecision_FOOD_REGISTRATION_DECISION_APPROVE:
		outcome = model.FoodRegApproved
	case adminpb.FoodRegistrationDecision_FOOD_REGISTRATION_DECISION_REJECT:
		outcome = model.FoodRegRejected
	case adminpb.FoodRegistrationDecision_FOOD_REGISTRATION_DECISION_UNSPECIFIED:
		return nil, rpc.RequiredField("decision", "decision 必填")
	default:
		return nil, rpc.RequiredField("decision", "decision 不合法")
	}
	decided, err := h.m.DecideFoodRegistration(ctx, model.DecideFoodRegistrationInput{
		ID: id, AdminID: adminID, Outcome: outcome, Reason: req.Msg.GetReason(), InternalNote: req.Msg.GetInternalNote(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, decideFoodRegistrationPaths)
	}
	return connect.NewResponse(&adminpb.DecideFoodRegistrationResponse{Registration: toAdminFoodRegistration(decided)}), nil
}

func (h *Admin) RevokeFoodRegistration(ctx context.Context, req *connect.Request[adminpb.RevokeFoodRegistrationRequest]) (*connect.Response[adminpb.RevokeFoodRegistrationResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	revoked, err := h.m.RevokeFoodRegistration(ctx, model.RevokeFoodRegistrationInput{ID: id, AdminID: adminID, Reason: req.Msg.GetReason()})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, revokeFoodRegistrationPaths)
	}
	return connect.NewResponse(&adminpb.RevokeFoodRegistrationResponse{Registration: toAdminFoodRegistration(revoked)}), nil
}

// foodRegistrationNumber 是店家目前核准的食品業者登錄字號；沒有核准的是空字串。
func foodRegistrationNumber(s model.Store) string {
	if s.ApprovedFoodRegistration == nil {
		return ""
	}
	return s.ApprovedFoodRegistration.RegistrationNumber
}
