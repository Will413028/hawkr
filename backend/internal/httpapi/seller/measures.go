package seller

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 措施與申覆的方法以帳號為範圍（宣告為豁免，不要求賣家角色或店家）：所有店家都已關閉的帳號照樣受處分，必須還能查措施與
// 申覆。帳號是登入的呼叫者；措施是不是這個帳號的，由模組以 account ID 檢查，別人的與不存在的一律 not_found。

var (
	appealPaths  = rpc.FieldPaths{"MeasureID": "measure_id", "Reason": "reason", "EvidenceTokens[]": "evidence_staged_tokens[]", "ExpectedVersion": "etag"}
	measurePaths = rpc.FieldPaths{"PageSize": "page_size", "State": "state"}
)

func (h *App) ListMyMeasures(ctx context.Context, req *connect.Request[pb.ListMyMeasuresRequest]) (*connect.Response[pb.ListMyMeasuresResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	msg := req.Msg
	// page token 只能用在同一組篩選：換了篩選就從第一頁開始。
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerAppServiceListMyMeasuresProcedure, msg.GetState().String())
	scope = rpc.PageScope(scope, p.ID)
	after, err := rpc.ValidCursor[model.MeasureCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in := model.AccountMeasureListInput{AccountID: p.ID, PageSize: msg.GetPageSize(), After: after}
	if state := rpc.EnumFromProto[model.MeasureState](msg.GetState()); state != "" {
		in.State = &state
	}
	page, err := h.m.AccountMeasures(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, measurePaths)
	}
	l := uitext.LocaleFrom(ctx)
	out := &pb.ListMyMeasuresResponse{Measures: make([]*pb.Measure, len(page.Items))}
	for i, m := range page.Items {
		out.Measures[i] = toMeasure(l, m)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *App) GetMyMeasure(ctx context.Context, req *connect.Request[pb.GetMyMeasureRequest]) (*connect.Response[pb.GetMyMeasureResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	id, err := rpc.ParseUUID("measure_id", req.Msg.GetMeasureId())
	if err != nil {
		return nil, err
	}
	m, err := h.m.AccountMeasure(ctx, p.ID, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetMyMeasureResponse{Measure: toMeasure(uitext.LocaleFrom(ctx), m)}), nil
}

func (h *App) SubmitAppeal(ctx context.Context, req *connect.Request[pb.SubmitAppealRequest]) (*connect.Response[pb.SubmitAppealResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	id, err := rpc.ParseUUID("measure_id", req.Msg.GetMeasureId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	m, err := h.m.SubmitAppeal(ctx, model.SubmitAppealInput{
		AccountID: p.ID, MeasureID: id, ExpectedVersion: version, Reason: req.Msg.GetReason(), EvidenceTokens: req.Msg.GetEvidenceStagedTokens(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, appealPaths)
	}
	return connect.NewResponse(&pb.SubmitAppealResponse{Measure: toMeasure(uitext.LocaleFrom(ctx), m)}), nil
}

func (h *App) RequestRemediation(ctx context.Context, req *connect.Request[pb.RequestRemediationRequest]) (*connect.Response[pb.RequestRemediationResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	id, err := rpc.ParseUUID("measure_id", req.Msg.GetMeasureId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	m, err := h.m.RequestRemediation(ctx, model.RequestRemediationInput{
		AccountID: p.ID, MeasureID: id, ExpectedVersion: version, Reason: req.Msg.GetReason(), EvidenceTokens: req.Msg.GetEvidenceStagedTokens(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, appealPaths)
	}
	return connect.NewResponse(&pb.RequestRemediationResponse{Measure: toMeasure(uitext.LocaleFrom(ctx), m)}), nil
}

// toRequest 轉成賣家看到的請求（r 是 Measure.EffectiveRequests 的結果，不是儲存值）：不輸出佐證與裁定的管理員。
func toRequest(r model.MeasureRequest) *pb.MeasureRequest {
	out := &pb.MeasureRequest{
		Id: r.ID.String(), Kind: rpc.EnumToProto[pb.MeasureRequestKind](r.Kind), State: rpc.EnumToProto[pb.MeasureRequestState](r.State), Reason: r.Reason,
		SubmitTime: timestamppb.New(r.SubmittedAt), DecisionReason: deref(r.DecisionReason),
	}
	if r.DecidedAt != nil {
		out.DecideTime = timestamppb.New(*r.DecidedAt)
	}
	if r.CloseCause != nil {
		out.CloseCause = rpc.EnumToProto[pb.MeasureRequestCloseCause](*r.CloseCause)
	}
	return out
}

// toMeasure 轉成賣家看到的措施：給賣家的理由（StatementToSeller）、依據的認定與期間，平台內部的註記、複審期限、建立與撤銷的
// 管理員、來源通報與佐證圖片不對外。etag 由賣家看得到的版本產生。不能申覆與不能請求解除的原因帶句子（語系 l），與兩個方法被拒絕時錯誤的
// LocalizedMessage 是同一份文字。
func toMeasure(l uitext.Locale, m model.Measure) *pb.Measure {
	out := &pb.Measure{
		Id: m.ID.String(), Kind: rpc.EnumToProto[pb.MeasureKind](m.Kind), Basis: rpc.EnumToProto[pb.MeasureBasis](m.Basis),
		Statement: m.StatementToSeller, State: rpc.EnumToProto[pb.MeasureState](m.State), CreateTime: timestamppb.New(m.CreatedAt),
		Etag: rpc.Etag(m.SellerVersion()), CanAppeal: m.CanAppeal(), CanRequestRemediation: m.CanRequestRemediation(),
	}
	if m.RiskGround != nil {
		out.RiskGround = rpc.EnumToProto[pb.RiskGround](*m.RiskGround)
	}
	if m.RevokedReason != nil {
		out.RevokeReason = rpc.EnumToProto[pb.RevokeReason](*m.RevokedReason)
	}
	for _, f := range m.Findings {
		out.Findings = append(out.Findings, &pb.MeasureFinding{
			Id: f.ID.String(), Category: rpc.EnumToProto[commonpb.PolicyCategory](f.Category),
			Severity: rpc.EnumToProto[pb.ViolationSeverity](f.Severity), CreateTime: timestamppb.New(f.CreatedAt),
		})
	}
	for _, c := range m.Contents {
		out.Contents = append(out.Contents, &pb.ContentRef{Kind: rpc.EnumToProto[pb.ContentKind](c.Kind), Id: c.ID.String()})
	}
	if m.Kind.Restrictive() {
		out.Period = &pb.MeasurePeriod{StartTime: timestamppb.New(m.StartsAt)}
		if m.EndsAt != nil {
			out.Period.EndTime = timestamppb.New(*m.EndsAt)
		}
	}
	if m.TerminationGround != nil {
		out.TerminationGround = rpc.EnumToProto[pb.TerminationGround](*m.TerminationGround)
	}
	if deadline := m.AppealDeadline(); deadline != nil {
		out.AppealDeadlineTime = timestamppb.New(*deadline)
	}
	for _, u := range m.AppealUnavailabilities(l) {
		out.AppealUnavailabilities = append(out.AppealUnavailabilities, &pb.AppealUnavailability{
			Reason: rpc.EnumToProto[pb.AppealUnavailableReason](u.Reason), Message: rpc.LocalizedMessage(u.Message),
		})
	}
	for _, r := range m.EffectiveRequests() {
		out.Requests = append(out.Requests, toRequest(r))
	}
	for _, u := range m.RemediationUnavailabilities(l) {
		out.RemediationUnavailabilities = append(out.RemediationUnavailabilities, &pb.RemediationUnavailability{
			Reason: rpc.EnumToProto[pb.RemediationUnavailableReason](u.Reason), Message: rpc.LocalizedMessage(u.Message),
		})
	}
	if next := m.NextRemediationTime(); next != nil {
		out.NextRemediationTime = timestamppb.New(*next)
	}
	if d := m.RejectionCooldown(); d > 0 {
		out.RejectionCooldown = durationpb.New(d)
	}
	return out
}
