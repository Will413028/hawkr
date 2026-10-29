package seller

import (
	"context"
	"errors"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func toNote(n model.GovernanceNote) *adminpb.GovernanceNote {
	out := &adminpb.GovernanceNote{
		Id: n.ID.String(), Body: n.Body, CreatedByAdminId: rpc.FormatID(n.AdminID), CreateTime: timestamppb.New(n.CreatedAt),
	}
	if c := n.SeverityChange; c != nil {
		out.SeverityChange = &adminpb.SeverityChange{
			FromSeverity: rpc.EnumToProto[pb.ViolationSeverity](c.From), ToSeverity: rpc.EnumToProto[pb.ViolationSeverity](c.To),
		}
	}
	return out
}

func toRevision(r model.MeasureRevision) *adminpb.MeasureRevision {
	out := &adminpb.MeasureRevision{
		Revision: r.Revision, Statement: r.StatementToSeller, CreatedByAdminId: formatAdminID(r.AdminID), CreateTime: timestamppb.New(r.CreatedAt),
	}
	if r.SystemCause != nil {
		out.SystemCause = rpc.EnumToProto[adminpb.MeasureRevisionSystemCause](*r.SystemCause)
	}
	if r.EndsAt != nil {
		out.EndTime = timestamppb.New(*r.EndsAt)
	}
	return out
}

// toAdminViolation 轉成後台看到的認定（只讀）。
func toAdminViolation(v model.Violation) *adminpb.Violation {
	out := &adminpb.Violation{
		Id: v.ID.String(), StoreId: rpc.FormatID(v.StoreID), AccountId: rpc.FormatID(v.AccountID),
		Category: rpc.EnumToProto[commonpb.PolicyCategory](v.Category), Severity: rpc.EnumToProto[pb.ViolationSeverity](v.Severity),
		Status: rpc.EnumToProto[pb.ViolationStatus](v.Status), OriginMeasureId: v.OriginMeasureID.String(), CreateTime: timestamppb.New(v.CreatedAt),
	}
	for _, id := range v.SourceReportIDs {
		out.SourceReportIds = append(out.SourceReportIds, id.String())
	}
	if v.EscalatedFrom != nil {
		out.EscalatedFrom = rpc.EnumToProto[pb.ViolationSeverity](*v.EscalatedFrom)
	}
	if v.RevokedReason != nil {
		out.RevokeReason = rpc.EnumToProto[pb.RevokeReason](*v.RevokedReason)
	}
	if v.RevokedAt != nil {
		out.RevokeTime = timestamppb.New(*v.RevokedAt)
	}
	return out
}

// toActionUnavailabilities 轉出 admin 不能做 action 的每個原因與它的文字（model.ActionUnavailableText，與被拒絕時錯誤的 LocalizedMessage 同一個函式）。
func toActionUnavailabilities(m model.Measure, action model.MeasureAction) []*adminpb.MeasureActionUnavailability {
	var out []*adminpb.MeasureActionUnavailability
	for _, r := range m.ActionUnavailableReasons(action) {
		out = append(out, &adminpb.MeasureActionUnavailability{
			Reason:  rpc.EnumToProto[adminpb.MeasureActionUnavailableReason](r),
			Message: rpc.LocalizedMessage(model.ActionUnavailableText(r, action)),
		})
	}
	return out
}

// toAdminMeasure 轉成後台看到的措施：etag 由後台的版本產生；can_* 與各動作的前置檢查是 model.Measure 的同一組方法。
func toAdminMeasure(m model.Measure) *adminpb.Measure {
	out := &adminpb.Measure{
		Id: m.ID.String(), StoreId: rpc.FormatID(m.StoreID), AccountId: rpc.FormatID(m.AccountID),
		Kind: rpc.EnumToProto[pb.MeasureKind](m.Kind), Basis: rpc.EnumToProto[pb.MeasureBasis](m.Basis),
		Statement: m.StatementToSeller, State: rpc.EnumToProto[pb.MeasureState](m.State),
		RevokedByAdminId: formatAdminID(m.RevokedByAdminID), CreatedByAdminId: rpc.FormatID(m.CreatedByAdminID),
		CreateTime: timestamppb.New(m.CreatedAt), UpdateTime: timestamppb.New(m.UpdatedAt),
		Etag: rpc.Etag(m.Version), CurrentRevision: m.CurrentRevision,
		CanUpdateStatement: m.CanUpdateStatement(), CanExtend: m.CanExtend(), CanShorten: m.CanShorten(), CanEnd: m.CanEnd(),
		CanRevoke: m.CanRevoke(), CanScheduleReview: m.CanScheduleReview(), CanDecideAppeal: m.CanDecideAppeal(),
		CanDecideRemediation: m.CanDecideRemediation(), CanRevokeFindings: m.CanRevokeFindings(), StoreName: m.StoreName,
		UpdateStatementUnavailabilities: toActionUnavailabilities(m, model.MeasureActionUpdateStatement),
		ExtendUnavailabilities:          toActionUnavailabilities(m, model.MeasureActionExtend),
		EndUnavailabilities:             toActionUnavailabilities(m, model.MeasureActionEnd),
		ShortenUnavailabilities:         toActionUnavailabilities(m, model.MeasureActionShorten),
		RevokeUnavailabilities:          toActionUnavailabilities(m, model.MeasureActionRevoke),
	}
	if m.RiskGround != nil {
		out.RiskGround = rpc.EnumToProto[pb.RiskGround](*m.RiskGround)
	}
	if m.EndsAt != nil {
		out.EndTime = timestamppb.New(*m.EndsAt)
	}
	if m.ReviewBy != nil {
		out.ReviewBy = timestamppb.New(*m.ReviewBy)
	}
	out.ReviewDeadline = toDeadline(m.ReviewDeadline)
	if m.RevokedReason != nil {
		out.RevokeReason = rpc.EnumToProto[pb.RevokeReason](*m.RevokedReason)
	}
	if m.RevokedAt != nil {
		out.RevokeTime = timestamppb.New(*m.RevokedAt)
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
	if m.SourceMeasureID != nil {
		out.SourceMeasureId = m.SourceMeasureID.String()
	}
	if deadline := m.AppealDeadline(); deadline != nil {
		out.AppealDeadlineTime = timestamppb.New(*deadline)
	}
	for _, l := range m.LinkedAccounts {
		out.LinkedAccounts = append(out.LinkedAccounts, toLinkedAccount(l))
	}
	for _, f := range m.Findings {
		out.Findings = append(out.Findings, toAdminViolation(f))
	}
	for _, c := range m.Contents {
		out.Contents = append(out.Contents, &pb.ContentRef{Kind: rpc.EnumToProto[pb.ContentKind](c.Kind), Id: c.ID.String()})
	}
	for _, r := range m.Revisions {
		out.Revisions = append(out.Revisions, toRevision(r))
	}
	for _, r := range m.EffectiveRequests() {
		out.Requests = append(out.Requests, toAdminRequest(r))
	}
	if next := m.NextRemediationTime(); next != nil {
		out.NextRemediationTime = timestamppb.New(*next)
	}
	return out
}

// toTerminationOption 轉出可建立的終止合作的生效時機與說明句（model.TerminationEffect.Message，admin RPC 固定 zh-TW）與依據的需求。
func toTerminationOption(e model.TerminationEffect) *adminpb.TerminationOption {
	out := &adminpb.TerminationOption{
		Ground:      rpc.EnumToProto[pb.TerminationGround](e.Ground),
		Explanation: rpc.LocalizedMessage(e.Message()),
		// 與 CreateMeasure 的驗證是同一個規則（model.TerminationGround.ViolationRequirement）。
		ViolationRequirement: rpc.EnumToProto[adminpb.TerminationViolationRequirement](e.Ground.ViolationRequirement()),
	}
	if e.EffectiveAt != nil {
		out.EffectiveTime = timestamppb.New(*e.EffectiveAt)
	}
	return out
}

func toLinkedAccount(l model.LinkedAccount) *adminpb.LinkedAccount {
	out := &adminpb.LinkedAccount{
		AccountId: rpc.FormatID(l.AccountID), StoreId: rpc.FormatID(l.StoreID), StoreCode: rpc.FormatID(l.StoreCode),
		MatchedIdentity: rpc.EnumToProto[adminpb.LinkedIdentity](l.Matched), CanCreateLinkedTermination: l.CanCreateLinkedTermination,
	}
	if l.LinkedMeasureID != nil {
		out.LinkedMeasureId = l.LinkedMeasureID.String()
	}
	if l.TerminationEffect != nil {
		out.TerminationOption = toTerminationOption(*l.TerminationEffect)
	}
	return out
}

// toAdminRequest 轉成後台看到的請求（申覆或解除請求，r 是 Measure.EffectiveRequests 的結果，不是儲存值）：帶佐證的檔案路徑、提出時的版本與裁定或結案的管理員（系統結案沒有）。
func toAdminRequest(r model.MeasureRequest) *adminpb.MeasureRequest {
	out := &adminpb.MeasureRequest{
		Id: r.ID.String(), Kind: rpc.EnumToProto[pb.MeasureRequestKind](r.Kind), State: rpc.EnumToProto[pb.MeasureRequestState](r.State), Reason: r.Reason,
		SubmitTime: timestamppb.New(r.SubmittedAt), DecisionReason: deref(r.DecisionReason),
		DecidedByAdminId: formatAdminID(r.DecidedByAdminID), Revision: r.Revision,
		Evidence:       toEvidence(len(r.Evidence), func(i int) string { return requestEvidenceURI(r.ID, i) }),
		DecideDeadline: toDeadline(r.DecideDeadline),
	}
	if r.DecidedAt != nil {
		out.DecideTime = timestamppb.New(*r.DecidedAt)
	}
	if r.CloseCause != nil {
		out.CloseCause = rpc.EnumToProto[pb.MeasureRequestCloseCause](*r.CloseCause)
	}
	return out
}

func toAdminMeasures(items []model.Measure) []*adminpb.Measure {
	out := make([]*adminpb.Measure, len(items))
	for i, m := range items {
		out[i] = toAdminMeasure(m)
	}
	return out
}

func boolScope(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func optionalTime(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}

// requestID 解析選填的 request_id：UUIDv4（API 契約 field_info.format = UUID4）；不設是 nil。
func requestID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := rpc.ParseUUID("request_id", raw)
	if err != nil {
		return nil, err
	}
	if id[6]>>4 != 4 {
		return nil, rpc.RequiredField("request_id", "request_id 必須是 UUIDv4")
	}
	return &id, nil
}

// violationBasisFrom 轉換請求裡的違規依據；prefix 是它在請求裡的路徑（解析失敗時指出哪個欄位）。
func violationBasisFrom(msg *adminpb.ViolationBasis, prefix string) (*model.ViolationBasis, error) {
	v := &model.ViolationBasis{}
	if f := msg.GetFinding(); f != nil {
		v.Finding = &model.NewFinding{
			Category: rpc.EnumFromProto[policy.Category](f.GetCategory()), Severity: rpc.EnumFromProto[model.Severity](f.GetSeverity()),
		}
		for i, raw := range f.GetSourceReportIds() {
			source, err := rpc.ParseUUID(apperr.IndexedField(prefix+".finding.source_report_ids", i), raw)
			if err != nil {
				return nil, err
			}
			v.Finding.SourceReportIDs = append(v.Finding.SourceReportIDs, source)
		}
		for i, raw := range f.GetSourceAuthorityOrderIds() {
			source, err := rpc.ParseUUID(apperr.IndexedField(prefix+".finding.source_authority_order_ids", i), raw)
			if err != nil {
				return nil, err
			}
			v.Finding.SourceAuthorityOrderIDs = append(v.Finding.SourceAuthorityOrderIDs, source)
		}
	}
	for i, raw := range msg.GetCitedFindingIds() {
		id, err := rpc.ParseUUID(apperr.IndexedField(prefix+".cited_finding_ids", i), raw)
		if err != nil {
			return nil, err
		}
		v.CitedFindingIDs = append(v.CitedFindingIDs, id)
	}
	return v, nil
}

// CreateMeasure 的種類就是設定的是哪一支輸入（warning、suspension、termination）：每一支只有自己的欄位，所以轉換時種類與依據、結束時間、複審期限
// 的對應是由結構保證，不需要跨欄位檢查；各支專屬的規則（停權依據認定必填結束時間、風險必填複審期限、終止事由的條件）由模組驗證。
func (h *Governance) CreateMeasure(ctx context.Context, req *connect.Request[adminpb.CreateMeasureRequest]) (*connect.Response[adminpb.CreateMeasureResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	storeID, err := rpc.ParseID("store_id", msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	reqID, err := requestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	in := model.CreateMeasureInput{
		AdminID: adminID, RequestID: reqID, StoreID: storeID, StatementToSeller: msg.GetStatement(), Note: msg.GetNote(),
	}
	if raw := msg.GetSourceCaseId(); raw != "" {
		source, err := rpc.ParseUUID("source_case_id", raw)
		if err != nil {
			return nil, err
		}
		in.SourceCaseID = &source
	}
	if raw := msg.GetSourceAuthorityOrderId(); raw != "" {
		source, err := rpc.ParseUUID("source_authority_order_id", raw)
		if err != nil {
			return nil, err
		}
		in.SourceAuthorityOrderID = &source
	}
	var paths rpc.FieldPaths
	switch input := msg.GetMeasure().(type) {
	case *adminpb.CreateMeasureRequest_Warning:
		in.Kind, paths = model.MeasureWarning, createWarningPaths
		if in.Violation, err = violationBasisFrom(input.Warning.GetViolation(), "warning.violation"); err != nil {
			return nil, err
		}
	case *adminpb.CreateMeasureRequest_Suspension:
		in.Kind, paths = model.MeasureSuspension, createSuspensionPaths
		in.EndsAt, in.ReviewBy = optionalTime(input.Suspension.GetEndTime()), optionalTime(input.Suspension.GetReviewBy())
		switch basis := input.Suspension.GetBasis().(type) {
		case *adminpb.SuspensionInput_Violation:
			if in.Violation, err = violationBasisFrom(basis.Violation, "suspension.violation"); err != nil {
				return nil, err
			}
		case *adminpb.SuspensionInput_Risk:
			in.Risk = &model.RiskBasis{Ground: rpc.EnumFromProto[model.RiskGround](basis.Risk.GetGround())}
		default:
			return nil, rpc.RequiredField("suspension.violation", "停權必須指定依據（violation 或 risk）")
		}
	case *adminpb.CreateMeasureRequest_Termination:
		in.Kind, paths = model.MeasureTermination, createTerminationPaths
		t := &model.TerminationBasis{Ground: rpc.EnumFromProto[model.TerminationGround](input.Termination.GetGround())}
		if v := input.Termination.GetViolation(); v != nil {
			if t.Violation, err = violationBasisFrom(v, "termination.violation"); err != nil {
				return nil, err
			}
		}
		if raw := input.Termination.GetSourceMeasureId(); raw != "" {
			source, err := rpc.ParseUUID("termination.source_measure_id", raw)
			if err != nil {
				return nil, err
			}
			t.SourceMeasureID = &source
		}
		in.Termination = t
	case *adminpb.CreateMeasureRequest_ContentTakedown:
		in.Kind, paths = model.MeasureContentTakedown, createTakedownPaths
		if in.Violation, err = violationBasisFrom(input.ContentTakedown.GetViolation(), "content_takedown.violation"); err != nil {
			return nil, err
		}
		t := &model.ContentTakedown{}
		for i, ref := range input.ContentTakedown.GetContents() {
			id, err := rpc.ParseUUID(apperr.IndexedField("content_takedown.contents", i)+".id", ref.GetId())
			if err != nil {
				return nil, err
			}
			t.Contents = append(t.Contents, model.ContentRef{Kind: rpc.EnumFromProto[model.ContentKind](ref.GetKind()), ID: id})
		}
		in.Takedown = t
	default:
		return nil, rpc.RequiredField("warning", "必須指定措施種類（warning、suspension、termination 或 content_takedown）")
	}
	if in.Takedown != nil {
		if err := h.checkTakedownContents(ctx, storeID, in.Takedown.Contents); err != nil {
			return nil, err
		}
	}
	result, err := h.m.CreateMeasure(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, paths)
	}
	return connect.NewResponse(&adminpb.CreateMeasureResponse{Measure: toAdminMeasure(result.Measure)}), nil
}

var takedownTargetKinds = map[model.ContentKind]social.TargetKind{
	model.ContentPost:       social.TargetPost,
	model.ContentComment:    social.TargetComment,
	model.ContentForumEntry: social.TargetForumEntry,
}

// checkTakedownContents 向 social 核對每則內容存在而且屬於 storeID（不存在的歸屬是零值，店家 ID 對不上）。核對與建立措施不在同一個 transaction：
// 帶 request_id 的建立以快照重播，不能加入呼叫方的 transaction；內容的歸屬店家不可變、列不硬刪，核對之後不會變成不成立。沒有 social 時關閉。
func (h *Governance) checkTakedownContents(ctx context.Context, storeID int64, contents []model.ContentRef) error {
	if h.content == nil {
		return errors.New("httpapi/seller：內容下架需要 social 核對內容歸屬")
	}
	for i, c := range contents {
		invalid := func(message string) error {
			return rpc.WithFieldPaths(apperr.InvalidInputOn([]string{apperr.IndexedField("Takedown.Contents", i)}, "%s", message), createTakedownPaths)
		}
		kind, ok := takedownTargetKinds[c.Kind]
		if !ok {
			return invalid("內容種類必須是貼文、留言或討論區留言")
		}
		owner, err := h.content.ContentOwner(ctx, social.Target{Kind: kind, ID: c.ID})
		if err != nil {
			return err
		}
		if owner.StoreID != storeID {
			return invalid("內容不存在，或不屬於這家店")
		}
	}
	return nil
}

func (h *Governance) GetMeasure(ctx context.Context, req *connect.Request[adminpb.GetMeasureRequest]) (*connect.Response[adminpb.GetMeasureResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	m, err := h.m.Measure(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetMeasureResponse{Measure: toAdminMeasure(m)}), nil
}

func (h *Governance) ListMeasures(ctx context.Context, req *connect.Request[adminpb.ListMeasuresRequest]) (*connect.Response[adminpb.ListMeasuresResponse], error) {
	msg := req.Msg
	in := model.AdminMeasureListInput{PageSize: msg.GetPageSize(), ReviewDue: msg.GetReviewDue()}
	var err error
	if msg.GetStoreId() != "" {
		if in.StoreID, err = rpc.ParseID("store_id", msg.GetStoreId()); err != nil {
			return nil, err
		}
	}
	if msg.GetAccountId() != "" {
		if in.AccountID, err = rpc.ParseID("account_id", msg.GetAccountId()); err != nil {
			return nil, err
		}
	}
	if state := rpc.EnumFromProto[model.MeasureState](msg.GetState()); state != "" {
		in.State = &state
	}
	if basis := rpc.EnumFromProto[model.MeasureBasis](msg.GetBasis()); basis != "" {
		in.Basis = &basis
	}
	if requestState := rpc.EnumFromProto[model.RequestState](msg.GetRequestState()); requestState != "" {
		in.RequestState = &requestState
	}
	if requestKind := rpc.EnumFromProto[model.RequestKind](msg.GetRequestKind()); requestKind != "" {
		in.RequestKind = &requestKind
	}
	if msg.GetSourceCaseId() != "" {
		id, err := rpc.ParseUUID("source_case_id", msg.GetSourceCaseId())
		if err != nil {
			return nil, err
		}
		in.SourceCaseID = &id
	}
	if msg.GetSourceReportId() != "" {
		id, err := rpc.ParseUUID("source_report_id", msg.GetSourceReportId())
		if err != nil {
			return nil, err
		}
		in.SourceReportID = &id
	}
	if msg.GetSourceAuthorityOrderId() != "" {
		id, err := rpc.ParseUUID("source_authority_order_id", msg.GetSourceAuthorityOrderId())
		if err != nil {
			return nil, err
		}
		in.SourceAuthorityOrderID = &id
	}
	// page token 只能用在同一組篩選：換了篩選就從第一頁開始。
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListMeasuresProcedure, msg.GetStoreId(), msg.GetAccountId(),
		msg.GetState().String(), msg.GetBasis().String(), msg.GetRequestState().String(), msg.GetRequestKind().String(), boolScope(msg.GetReviewDue()),
		msg.GetSourceCaseId(), msg.GetSourceReportId(), msg.GetSourceAuthorityOrderId())
	after, err := rpc.ValidCursor[model.MeasureCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in.After = after
	page, err := h.m.AdminMeasures(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listMeasuresPaths)
	}
	out := &adminpb.ListMeasuresResponse{Measures: toAdminMeasures(page.Items)}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

// UpdateMeasure 只改得動 statement、end_time：Measure 裡其他欄位都是 OUTPUT_ONLY，update_mask 照〈API 設計規範（Connect）〉§七
// 處理（rpc.ApplyUpdateMask）。合併的基準是目前的措施；end_time 只在合併後與目前不同時才交給模組（警告沒有 end_time，原樣送回不算
// 修改），mask 列了 end_time 卻沒有值是清除。
func (h *Governance) UpdateMeasure(ctx context.Context, req *connect.Request[adminpb.UpdateMeasureRequest]) (*connect.Response[adminpb.UpdateMeasureResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetMeasure()
	if msg == nil {
		return nil, rpc.RequiredField("measure", "measure 必填")
	}
	id, err := rpc.ParseUUID("measure.id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "measure.etag")
	if err != nil {
		return nil, err
	}
	current, err := h.m.CurrentMeasure(ctx, id)
	if err != nil {
		return nil, err
	}
	merged := toAdminMeasure(current)
	before := current.EndsAt
	if err := rpc.ApplyUpdateMask(merged, msg, req.Msg.GetUpdateMask()); err != nil {
		return nil, err
	}
	in := model.UpdateMeasureInput{MeasureID: id, AdminID: adminID, ExpectedVersion: version, StatementToSeller: &merged.Statement}
	if after := merged.GetEndTime(); !sameInstant(before, after) {
		in.ChangeEndsAt, in.EndsAt = true, optionalTime(after)
	}
	updated, err := h.m.UpdateMeasure(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, updateMeasurePaths)
	}
	return connect.NewResponse(&adminpb.UpdateMeasureResponse{Measure: toAdminMeasure(updated)}), nil
}

// sameInstant 比較合併前的結束時間與合併後的時間（兩者都可能不設）。
func sameInstant(before *time.Time, after *timestamppb.Timestamp) bool {
	if before == nil || after == nil {
		return before == nil && after == nil
	}
	return before.Equal(after.AsTime())
}

func (h *Governance) EndMeasure(ctx context.Context, req *connect.Request[adminpb.EndMeasureRequest]) (*connect.Response[adminpb.EndMeasureResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	m, err := h.m.EndMeasure(ctx, model.EndMeasureInput{MeasureID: id, AdminID: adminID, ExpectedVersion: version})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, endMeasurePaths)
	}
	return connect.NewResponse(&adminpb.EndMeasureResponse{Measure: toAdminMeasure(m)}), nil
}

func (h *Governance) RevokeMeasure(ctx context.Context, req *connect.Request[adminpb.RevokeMeasureRequest]) (*connect.Response[adminpb.RevokeMeasureResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	m, err := h.m.RevokeMeasure(ctx, model.RevokeMeasureInput{
		MeasureID: id, AdminID: adminID, ExpectedVersion: version, Reason: rpc.EnumFromProto[model.RevokeReason](req.Msg.GetReason()),
		RevokeFindings: req.Msg.GetRevokeFindings(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, revokeMeasurePaths)
	}
	return connect.NewResponse(&adminpb.RevokeMeasureResponse{Measure: toAdminMeasure(m)}), nil
}

func (h *Governance) ScheduleMeasureReview(ctx context.Context, req *connect.Request[adminpb.ScheduleMeasureReviewRequest]) (*connect.Response[adminpb.ScheduleMeasureReviewResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	if req.Msg.GetReviewBy() == nil {
		return nil, rpc.RequiredField("review_by", "review_by 必填")
	}
	m, err := h.m.ScheduleMeasureReview(ctx, model.ScheduleReviewInput{
		MeasureID: id, AdminID: adminID, ExpectedVersion: version, ReviewBy: req.Msg.GetReviewBy().AsTime(), Note: req.Msg.GetNote(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, scheduleReviewPaths)
	}
	return connect.NewResponse(&adminpb.ScheduleMeasureReviewResponse{Measure: toAdminMeasure(m)}), nil
}

func (h *Governance) DecideAppeal(ctx context.Context, req *connect.Request[adminpb.DecideAppealRequest]) (*connect.Response[adminpb.DecideAppealResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("measure_id", req.Msg.GetMeasureId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	m, err := h.m.DecideAppeal(ctx, model.DecideAppealInput{
		MeasureID: id, AdminID: adminID, ExpectedVersion: version, Outcome: rpc.EnumFromProto[model.RequestState](req.Msg.GetDecision()),
		Reason: req.Msg.GetReason(), RevokeFindings: req.Msg.GetRevokeFindings(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, decideAppealPaths)
	}
	return connect.NewResponse(&adminpb.DecideAppealResponse{Measure: toAdminMeasure(m)}), nil
}

func (h *Governance) DecideRemediation(ctx context.Context, req *connect.Request[adminpb.DecideRemediationRequest]) (*connect.Response[adminpb.DecideRemediationResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("measure_id", req.Msg.GetMeasureId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	m, err := h.m.DecideRemediation(ctx, model.DecideRemediationInput{
		MeasureID: id, AdminID: adminID, ExpectedVersion: version, Outcome: rpc.EnumFromProto[model.RequestState](req.Msg.GetDecision()),
		Reason: req.Msg.GetReason(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, decideRemediationPaths)
	}
	return connect.NewResponse(&adminpb.DecideRemediationResponse{Measure: toAdminMeasure(m)}), nil
}

// CreateMeasureNote 新增內部註記：不帶 etag，也不改措施的任何 etag。
func (h *Governance) CreateMeasureNote(ctx context.Context, req *connect.Request[adminpb.CreateMeasureNoteRequest]) (*connect.Response[adminpb.CreateMeasureNoteResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("measure_id", req.Msg.GetMeasureId())
	if err != nil {
		return nil, err
	}
	note, err := h.m.CreateMeasureNote(ctx, model.CreateMeasureNoteInput{MeasureID: id, AdminID: adminID, Body: req.Msg.GetNote().GetBody()})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createMeasureNotePaths)
	}
	return connect.NewResponse(&adminpb.CreateMeasureNoteResponse{Note: toNote(note)}), nil
}

// ListMeasureNotes 是措施內部註記的分頁讀取（舊到新）。page token 綁定措施，換一筆措施就從第一頁開始。
func (h *Governance) ListMeasureNotes(ctx context.Context, req *connect.Request[adminpb.ListMeasureNotesRequest]) (*connect.Response[adminpb.ListMeasureNotesResponse], error) {
	msg := req.Msg
	id, err := rpc.ParseUUID("measure_id", msg.GetMeasureId())
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListMeasureNotesProcedure, msg.GetMeasureId())
	after, err := rpc.ValidCursor[model.NoteCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.MeasureNotes(ctx, model.MeasureNoteListInput{MeasureID: id, PageSize: msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListMeasureNotesResponse{Notes: make([]*adminpb.GovernanceNote, len(page.Items))}
	for i, n := range page.Items {
		out.Notes[i] = toNote(n)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Governance) GetViolation(ctx context.Context, req *connect.Request[adminpb.GetViolationRequest]) (*connect.Response[adminpb.GetViolationResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	v, err := h.m.Violation(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetViolationResponse{Violation: toAdminViolation(v)}), nil
}

func (h *Governance) ListViolations(ctx context.Context, req *connect.Request[adminpb.ListViolationsRequest]) (*connect.Response[adminpb.ListViolationsResponse], error) {
	msg := req.Msg
	in := model.ViolationListInput{PageSize: msg.GetPageSize()}
	var err error
	if msg.GetStoreId() != "" {
		if in.StoreID, err = rpc.ParseID("store_id", msg.GetStoreId()); err != nil {
			return nil, err
		}
	}
	if msg.GetAccountId() != "" {
		if in.AccountID, err = rpc.ParseID("account_id", msg.GetAccountId()); err != nil {
			return nil, err
		}
	}
	if status := rpc.EnumFromProto[model.ViolationStatus](msg.GetStatus()); status != "" {
		in.Status = &status
	}
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListViolationsProcedure, msg.GetStoreId(), msg.GetAccountId(), msg.GetStatus().String())
	after, err := rpc.ValidCursor[model.ViolationCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in.After = after
	page, err := h.m.AdminViolations(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listViolationsPaths)
	}
	out := &adminpb.ListViolationsResponse{Violations: make([]*adminpb.Violation, len(page.Items))}
	for i, v := range page.Items {
		out.Violations[i] = toAdminViolation(v)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}
