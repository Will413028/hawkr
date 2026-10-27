package delegationservice

import (
	"context"
	"strconv"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/account"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

// 代行授權的治理（六支）。actorAdminId 不合法是 9261（沒有管理員身分），其他欄位不合法是 9260。

func (s *Server) ListDelegationAuthorizations(ctx context.Context, req *connect.Request[pb.ListDelegationAuthorizationsRequest]) (*connect.Response[pb.ListDelegationAuthorizationsResponse], error) {
	m := req.Msg
	reply := func(code int32, msg string) (*connect.Response[pb.ListDelegationAuthorizationsResponse], error) {
		return connect.NewResponse(&pb.ListDelegationAuthorizationsResponse{Code: code, Message: msg}), nil
	}
	adminID, ok := parseID(m.GetActorAdminId())
	if !ok {
		return reply(codeAdminSessionRequired, "actorAdminId 不正確")
	}
	var p delegation.DelegationPaging
	if p.Page, ok = parseOptionalInt32(m.GetPage()); !ok {
		return reply(codeInvalidCommand, "page 不正確")
	}
	if p.PageSize, ok = parseOptionalInt32(m.GetPageSize()); !ok {
		return reply(codeInvalidCommand, "pageSize 不正確")
	}
	storeID, ok := parseOptionalID(m.GetTargetStoreId())
	if !ok {
		return reply(codeInvalidCommand, "targetStoreId 不正確")
	}
	page, err := s.m.DelegationAuthorizations(ctx, delegation.ListDelegationAuthorizationsInput{
		AdminID: adminID, Filter: delegation.DelegationAuthorizationFilter{
			Status: delegation.AuthorizationStatusFilter(m.GetStatus()), StoreID: storeID,
		}, Paging: p,
	})
	if err != nil {
		code, msg, err := s.reject(ctx, "ListDelegationAuthorizations", err)
		return connect.NewResponse(&pb.ListDelegationAuthorizationsResponse{Code: code, Message: msg}), err
	}
	out := &pb.ListDelegationAuthorizationsResponse{
		Message: success, Total: strconv.FormatInt(page.Total, 10),
		Page: strconv.Itoa(int(page.Paging.Page)), PageSize: strconv.Itoa(int(page.Paging.PageSize)),
	}
	for _, a := range page.Authorizations {
		out.Records = append(out.Records, toAuthorization(a))
	}
	return connect.NewResponse(out), nil
}

func (s *Server) GetDelegationAuthorization(ctx context.Context, req *connect.Request[pb.GetDelegationAuthorizationRequest]) (*connect.Response[pb.GetDelegationAuthorizationResponse], error) {
	adminID, ok := parseID(req.Msg.GetActorAdminId())
	if !ok {
		return connect.NewResponse(&pb.GetDelegationAuthorizationResponse{Code: codeAdminSessionRequired, Message: "actorAdminId 不正確"}), nil
	}
	storeID, ok := parseID(req.Msg.GetTargetStoreId())
	if !ok {
		return connect.NewResponse(&pb.GetDelegationAuthorizationResponse{Code: codeInvalidCommand, Message: "targetStoreId 不正確"}), nil
	}
	panel, err := s.m.DelegationAuthorizationOfStore(ctx, adminID, storeID)
	if err != nil {
		code, msg, err := s.reject(ctx, "GetDelegationAuthorization", err)
		return connect.NewResponse(&pb.GetDelegationAuthorizationResponse{Code: code, Message: msg}), err
	}
	data := &pb.GetDelegationAuthorizationData{Exists: panel.Exists, ActiveSessionCount: strconv.FormatInt(panel.ActiveSessions, 10)}
	if panel.Exists {
		data.Authorization = toAuthorization(panel.Authorization)
	}
	return connect.NewResponse(&pb.GetDelegationAuthorizationResponse{Message: success, Data: data}), nil
}

// authorizationChange 是 Grant 與 Update 共用的請求欄位（兩者的形狀逐項相同）。
type authorizationChange interface {
	GetActorAdminId() string
	GetTargetStoreId() string
	GetSource() string
	GetSourceReference() string
	GetGrantedCapabilities() []string
	GetOperationReason() string
	GetIdempotencyKey() string
	GetRequestId() string
	GetCorrelationId() string
}

// toChange 解析開通與調整的請求；失敗時回傳業務碼與訊息。
func toChange(m authorizationChange, effectiveUntil *string) (delegation.DelegationAuthorizationChange, int32, string) {
	adminID, ok := parseID(m.GetActorAdminId())
	if !ok {
		return delegation.DelegationAuthorizationChange{}, codeAdminSessionRequired, "actorAdminId 不正確"
	}
	storeID, ok := parseID(m.GetTargetStoreId())
	if !ok {
		return delegation.DelegationAuthorizationChange{}, codeInvalidCommand, "targetStoreId 不正確"
	}
	in := delegation.DelegationAuthorizationChange{
		AdminID:         adminID,
		StoreID:         storeID,
		Source:          delegation.AuthorizationSource(m.GetSource()),
		SourceReference: m.GetSourceReference(),
		Capabilities:    toCapabilities(m.GetGrantedCapabilities()),
		OperationReason: m.GetOperationReason(),
		IdempotencyKey:  m.GetIdempotencyKey(),
		RequestID:       m.GetRequestId(),
		CorrelationID:   m.GetCorrelationId(),
	}
	// proto3 optional：沒有帶是無期限（重新開通時也不沿用舊值）。
	if effectiveUntil != nil {
		t, ok := parseTime(*effectiveUntil)
		if !ok || t == nil {
			return in, codeInvalidCommand, "effectiveUntil 格式不正確"
		}
		in.EffectiveUntil = t
	}
	return in, 0, ""
}

func (s *Server) GrantDelegationAuthorization(ctx context.Context, req *connect.Request[pb.GrantDelegationAuthorizationRequest]) (*connect.Response[pb.GrantDelegationAuthorizationResponse], error) {
	in, code, msg := toChange(req.Msg, req.Msg.EffectiveUntil)
	if code != 0 {
		return connect.NewResponse(&pb.GrantDelegationAuthorizationResponse{Code: code, Message: msg}), nil
	}
	res, err := s.m.GrantDelegationAuthorization(ctx, in)
	if err != nil {
		code, msg, err := s.reject(ctx, "GrantDelegationAuthorization", err)
		return connect.NewResponse(&pb.GrantDelegationAuthorizationResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.GrantDelegationAuthorizationResponse{Message: success, Data: &pb.GrantDelegationAuthorizationData{
		Authorization: toAuthorization(res.Authorization), Outcome: string(res.Outcome),
	}}), nil
}

func (s *Server) UpdateDelegationAuthorization(ctx context.Context, req *connect.Request[pb.UpdateDelegationAuthorizationRequest]) (*connect.Response[pb.UpdateDelegationAuthorizationResponse], error) {
	in, code, msg := toChange(req.Msg, req.Msg.EffectiveUntil)
	if code != 0 {
		return connect.NewResponse(&pb.UpdateDelegationAuthorizationResponse{Code: code, Message: msg}), nil
	}
	a, err := s.m.UpdateDelegationAuthorization(ctx, in)
	if err != nil {
		code, msg, err := s.reject(ctx, "UpdateDelegationAuthorization", err)
		return connect.NewResponse(&pb.UpdateDelegationAuthorizationResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.UpdateDelegationAuthorizationResponse{Message: success, Data: &pb.UpdateDelegationAuthorizationData{
		Authorization: toAuthorization(a),
	}}), nil
}

func (s *Server) RevokeDelegationAuthorization(ctx context.Context, req *connect.Request[pb.RevokeDelegationAuthorizationRequest]) (*connect.Response[pb.RevokeDelegationAuthorizationResponse], error) {
	m := req.Msg
	adminID, ok := parseID(m.GetActorAdminId())
	if !ok {
		return connect.NewResponse(&pb.RevokeDelegationAuthorizationResponse{Code: codeAdminSessionRequired, Message: "actorAdminId 不正確"}), nil
	}
	storeID, ok := parseID(m.GetTargetStoreId())
	if !ok {
		return connect.NewResponse(&pb.RevokeDelegationAuthorizationResponse{Code: codeInvalidCommand, Message: "targetStoreId 不正確"}), nil
	}
	res, err := s.m.RevokeDelegationAuthorization(ctx, delegation.DelegationAuthorizationRevocation{
		AdminID: adminID, StoreID: storeID, Reason: m.GetRevokeReason(),
		IdempotencyKey: m.GetIdempotencyKey(), RequestID: m.GetRequestId(), CorrelationID: m.GetCorrelationId(),
	})
	if err != nil {
		code, msg, err := s.reject(ctx, "RevokeDelegationAuthorization", err)
		return connect.NewResponse(&pb.RevokeDelegationAuthorizationResponse{Code: code, Message: msg}), err
	}
	ids := make([]string, len(res.RevokedSessions))
	for i, id := range res.RevokedSessions {
		ids[i] = formatID(id)
	}
	return connect.NewResponse(&pb.RevokeDelegationAuthorizationResponse{Message: success, Data: &pb.RevokeDelegationAuthorizationData{
		Authorization: toAuthorization(res.Authorization), AlreadyRevoked: res.AlreadyRevoked, RevokedSessionIds: ids,
	}}), nil
}

func (s *Server) ListDelegationAuthorizationHistory(ctx context.Context, req *connect.Request[pb.ListDelegationAuthorizationHistoryRequest]) (*connect.Response[pb.ListDelegationAuthorizationHistoryResponse], error) {
	m := req.Msg
	reply := func(code int32, msg string) (*connect.Response[pb.ListDelegationAuthorizationHistoryResponse], error) {
		return connect.NewResponse(&pb.ListDelegationAuthorizationHistoryResponse{Code: code, Message: msg}), nil
	}
	adminID, ok := parseID(m.GetActorAdminId())
	if !ok {
		return reply(codeAdminSessionRequired, "actorAdminId 不正確")
	}
	storeID, ok := parseID(m.GetTargetStoreId())
	if !ok {
		return reply(codeInvalidCommand, "targetStoreId 不正確")
	}
	var p delegation.DelegationPaging
	if p.Page, ok = parseOptionalInt32(m.GetPage()); !ok {
		return reply(codeInvalidCommand, "page 不正確")
	}
	if p.PageSize, ok = parseOptionalInt32(m.GetPageSize()); !ok {
		return reply(codeInvalidCommand, "pageSize 不正確")
	}
	page, err := s.m.DelegationAuthorizationHistory(ctx, delegation.DelegationAuthorizationHistoryInput{
		AdminID: adminID, StoreID: storeID, Paging: p,
	})
	if err != nil {
		code, msg, err := s.reject(ctx, "ListDelegationAuthorizationHistory", err)
		return connect.NewResponse(&pb.ListDelegationAuthorizationHistoryResponse{Code: code, Message: msg}), err
	}
	out := &pb.ListDelegationAuthorizationHistoryResponse{
		Message: success, Total: strconv.FormatInt(page.Total, 10),
		Page: strconv.Itoa(int(page.Paging.Page)), PageSize: strconv.Itoa(int(page.Paging.PageSize)),
	}
	for _, e := range page.Entries {
		r := &pb.DelegationAuthorizationAuditEntry{
			Id:                   e.ID.String(),
			AuthorizationId:      e.AuthorizationID.String(),
			TargetStoreId:        formatID(e.StoreID),
			Action:               string(e.Action),
			ActionLabel:          e.Action.Label(),
			ActorAdminId:         formatID(e.ActorAdminID),
			ActorDisplayName:     e.ActorDisplayName,
			OperationReason:      e.OperationReason,
			SourceReference:      e.SourceReference,
			CapabilitiesBefore:   capabilityStrings(e.CapabilitiesBefore),
			CapabilitiesAfter:    capabilityStrings(e.CapabilitiesAfter),
			EffectiveUntilBefore: optionalTime(e.EffectiveUntilBefore),
			EffectiveUntilAfter:  optionalTime(e.EffectiveUntilAfter),
			CreatedAt:            formatTime(e.CreatedAt),
		}
		if e.Source != nil {
			src := string(*e.Source)
			r.Source = &src
		}
		out.Records = append(out.Records, r)
	}
	return connect.NewResponse(out), nil
}

// toAuthorization 是授權列對外的唯一投影。可空的欄位以 proto3 optional 輸出 null。
func toAuthorization(a delegation.DelegationAuthorization) *pb.DelegationAuthorization {
	return &pb.DelegationAuthorization{
		Id:                         a.ID.String(),
		TargetStoreId:              formatID(a.StoreID),
		Source:                     string(a.Source),
		SourceReference:            a.SourceReference,
		GrantedCapabilities:        capabilityStrings(a.Capabilities),
		EffectiveFrom:              formatTime(a.EffectiveFrom),
		EffectiveUntil:             optionalTime(a.EffectiveUntil),
		ApprovedByAdminId:          formatID(a.ApprovedByAdminID),
		ApprovedByAdminDisplayName: a.ApprovedByDisplayName,
		ApprovedAt:                 formatTime(a.ApprovedAt),
		RevokedAt:                  optionalTime(a.RevokedAt),
		RevokedByAdminId:           optionalID(a.RevokedByAdminID),
		RevokedByAdminDisplayName:  a.RevokedByDisplayName,
		RevokeReason:               a.RevokeReason,
		Effective:                  a.Effective,
		CreatedAt:                  formatTime(a.CreatedAt),
		UpdatedAt:                  formatTime(a.UpdatedAt),
	}
}
