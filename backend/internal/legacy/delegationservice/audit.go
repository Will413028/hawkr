package delegationservice

import (
	"context"
	"encoding/json"
	"strconv"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/account"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

// 兩階段稽核與唯讀的查詢。

func toIntentInput(m *pb.RecordAuditIntentRequest) (delegation.AuditIntentInput, bool) {
	sessionID, ok := parseID(m.GetSessionId())
	if !ok {
		return delegation.AuditIntentInput{}, false
	}
	in := delegation.AuditIntentInput{
		SessionID:             sessionID,
		Capability:            delegation.Capability(m.GetCapability()),
		Action:                delegation.AuditAction(m.GetAction()),
		ResourceType:          fromWire[delegation.AuditResourceType](m.GetResourceType()),
		ResourceID:            m.GetResourceId(),
		ResourceName:          m.GetResourceName(),
		OperationReason:       m.GetOperationReason(),
		ResourceVersionBefore: parseOptionalInt64(m.GetResourceVersionBefore()),
		IdempotencyKey:        m.GetIdempotencyKey(),
		RequestID:             m.GetRequestId(),
		CorrelationID:         m.GetCorrelationId(),
		UserAgent:             m.GetUserAgent(),
		PayloadHash:           m.GetPayloadHash(),
		NotifySeller:          m.GetNotifySeller(),
		Origin:                fromWire[delegation.AuditOrigin](m.GetOrigin()),
	}
	if raw := m.GetBeforeSnapshotJson(); raw != "" {
		in.BeforeSnapshot = json.RawMessage(raw)
	}
	return in, true
}

func toIntentData(i delegation.AuditIntent) *pb.RecordAuditIntentData {
	return &pb.RecordAuditIntentData{AuditLogId: i.AuditLogID.String(), Replayed: i.Replayed, Result: string(i.Result)}
}

func (s *Server) RecordAuditIntent(ctx context.Context, req *connect.Request[pb.RecordAuditIntentRequest]) (*connect.Response[pb.RecordAuditIntentResponse], error) {
	in, ok := toIntentInput(req.Msg)
	if !ok {
		return connect.NewResponse(&pb.RecordAuditIntentResponse{Code: codeInvalidCommand, Message: "sessionId 不正確"}), nil
	}
	res, err := s.m.RecordDelegationAuditIntent(ctx, in)
	if err != nil {
		code, msg, err := s.reject(ctx, "RecordAuditIntent", err)
		return connect.NewResponse(&pb.RecordAuditIntentResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.RecordAuditIntentResponse{Message: success, Data: toIntentData(res)}), nil
}

// RecordAuditIntentBatch 逐項成敗：一項寫不進去只讓那一項以業務碼收尾（沒有業務碼的錯誤是 9279），其他項照跑。
func (s *Server) RecordAuditIntentBatch(ctx context.Context, req *connect.Request[pb.RecordAuditIntentBatchRequest]) (*connect.Response[pb.RecordAuditIntentBatchResponse], error) {
	items := req.Msg.GetItems()
	if len(items) == 0 || len(items) > delegation.DelegationMaxBatch {
		return connect.NewResponse(&pb.RecordAuditIntentBatchResponse{
			Code: codeInvalidCommand, Message: "items 必須為 1–" + strconv.Itoa(delegation.DelegationMaxBatch) + " 筆",
		}), nil
	}
	results := make([]*pb.RecordAuditIntentItemResult, len(items))
	var inputs []delegation.AuditIntentInput
	var positions []int
	for i, it := range items {
		in, ok := toIntentInput(it)
		if !ok {
			results[i] = &pb.RecordAuditIntentItemResult{ResourceId: it.GetResourceId(), BizCode: codeInvalidCommand}
			continue
		}
		inputs = append(inputs, in)
		positions = append(positions, i)
	}
	if len(inputs) > 0 {
		outcomes, err := s.m.RecordDelegationAuditIntents(ctx, inputs)
		if err != nil {
			code, msg, err := s.reject(ctx, "RecordAuditIntentBatch", err)
			return connect.NewResponse(&pb.RecordAuditIntentBatchResponse{Code: code, Message: msg}), err
		}
		for j, o := range outcomes {
			r := &pb.RecordAuditIntentItemResult{ResourceId: inputs[j].ResourceID}
			if o.Err != nil {
				r.BizCode = codeAuditIntentFailed
				if d, ok := delegation.DelegationDenialOf(o.Err); ok {
					r.BizCode = bizCodes[d.Denial]
				} else {
					s.log.ErrorContext(ctx, "delegationservice：批次稽核 intent 的一項失敗", "err", o.Err)
				}
			} else {
				r.Ok, r.Data = true, toIntentData(o.Intent)
			}
			results[positions[j]] = r
		}
	}
	return connect.NewResponse(&pb.RecordAuditIntentBatchResponse{Message: success, Results: results}), nil
}

func toConfirmInput(m *pb.ConfirmAuditResultRequest) (delegation.AuditConfirmInput, bool) {
	id, ok := parseUUID(m.GetAuditLogId())
	if !ok {
		return delegation.AuditConfirmInput{}, false
	}
	in := delegation.AuditConfirmInput{
		AuditLogID:           id,
		Result:               delegation.AuditResult(m.GetResult()),
		BizCode:              m.GetBizCode(),
		ResourceID:           optionalString(m.GetResourceId()),
		ResourceVersionAfter: parseOptionalInt64(m.GetResourceVersionAfter()),
	}
	if raw := m.GetAfterSnapshotJson(); raw != "" {
		in.AfterSnapshot = json.RawMessage(raw)
	}
	for _, f := range m.GetChangedFields() {
		in.ChangedFields = append(in.ChangedFields, delegation.ChangedField{
			Field: f.GetField(), Label: f.GetLabel(), Before: f.GetBefore(), After: f.GetAfter(),
		})
	}
	return in, true
}

func (s *Server) ConfirmAuditResult(ctx context.Context, req *connect.Request[pb.ConfirmAuditResultRequest]) (*connect.Response[pb.ConfirmAuditResultResponse], error) {
	in, ok := toConfirmInput(req.Msg)
	if !ok {
		return connect.NewResponse(&pb.ConfirmAuditResultResponse{Code: codeInvalidCommand, Message: "auditLogId 不正確"}), nil
	}
	l, err := s.m.ConfirmDelegationAudit(ctx, in)
	if err != nil {
		code, msg, err := s.reject(ctx, "ConfirmAuditResult", err)
		return connect.NewResponse(&pb.ConfirmAuditResultResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.ConfirmAuditResultResponse{Message: success, Data: &pb.ConfirmAuditResultData{
		AuditLogId: l.ID.String(), Result: string(l.Result),
	}}), nil
}

// ConfirmAuditResultBatch 全有或全無：失敗時所有列都留在 PENDING。
func (s *Server) ConfirmAuditResultBatch(ctx context.Context, req *connect.Request[pb.ConfirmAuditResultBatchRequest]) (*connect.Response[pb.ConfirmAuditResultBatchResponse], error) {
	items := req.Msg.GetItems()
	if len(items) == 0 || len(items) > delegation.DelegationMaxBatch {
		return connect.NewResponse(&pb.ConfirmAuditResultBatchResponse{
			Code: codeInvalidCommand, Message: "items 必須為 1–" + strconv.Itoa(delegation.DelegationMaxBatch) + " 筆",
		}), nil
	}
	inputs := make([]delegation.AuditConfirmInput, 0, len(items))
	for _, it := range items {
		in, ok := toConfirmInput(it)
		if !ok {
			return connect.NewResponse(&pb.ConfirmAuditResultBatchResponse{Code: codeInvalidCommand, Message: "auditLogId 不正確"}), nil
		}
		inputs = append(inputs, in)
	}
	logs, err := s.m.ConfirmDelegationAudits(ctx, inputs)
	if err != nil {
		code, msg, err := s.reject(ctx, "ConfirmAuditResultBatch", err)
		return connect.NewResponse(&pb.ConfirmAuditResultBatchResponse{Code: code, Message: msg}), err
	}
	out := &pb.ConfirmAuditResultBatchResponse{Message: success}
	for _, l := range logs {
		out.Results = append(out.Results, &pb.ConfirmAuditResultData{AuditLogId: l.ID.String(), Result: string(l.Result)})
	}
	return connect.NewResponse(out), nil
}

// auditFilter 解析兩支稽核清單共用的篩選與分頁；失敗時回傳給操作者的訊息。
func auditFilter(action, resourceType, result, from, to, page, pageSize string) (delegation.AuditLogFilter, delegation.DelegationPaging, string) {
	f := delegation.AuditLogFilter{
		Action: delegation.AuditAction(action), ResourceType: fromWire[delegation.AuditResourceType](resourceType), Result: delegation.AuditResult(result),
	}
	var p delegation.DelegationPaging
	var ok bool
	if p.Page, ok = parseOptionalInt32(page); !ok {
		return f, p, "page 不正確"
	}
	if p.PageSize, ok = parseOptionalInt32(pageSize); !ok {
		return f, p, "pageSize 不正確"
	}
	if f.From, ok = parseTime(from); !ok {
		return f, p, "dateFrom 格式不正確"
	}
	if f.To, ok = parseTime(to); !ok {
		return f, p, "dateTo 格式不正確"
	}
	return f, p, ""
}

func (s *Server) ListAuditLogs(ctx context.Context, req *connect.Request[pb.ListAuditLogsRequest]) (*connect.Response[pb.ListAuditLogsResponse], error) {
	m := req.Msg
	sessionID, ok := parseID(m.GetSessionId())
	if !ok {
		return connect.NewResponse(&pb.ListAuditLogsResponse{Code: codeSessionNotFound, Message: "找不到指定的代行工作階段"}), nil
	}
	f, p, bad := auditFilter(m.GetActionFilter(), m.GetResourceTypeFilter(), m.GetResultFilter(), m.GetDateFrom(), m.GetDateTo(), m.GetPage(), m.GetPageSize())
	if bad != "" {
		return connect.NewResponse(&pb.ListAuditLogsResponse{Code: codeInvalidCommand, Message: bad}), nil
	}
	f.SessionID = &sessionID
	page, err := s.m.DelegationAuditLogs(ctx, f, p)
	if err != nil {
		code, msg, err := s.reject(ctx, "ListAuditLogs", err)
		return connect.NewResponse(&pb.ListAuditLogsResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.ListAuditLogsResponse{
		Message: success, Records: toAuditEntries(page.Logs), Total: strconv.FormatInt(page.Total, 10),
		Page: strconv.Itoa(int(page.Paging.Page)), PageSize: strconv.Itoa(int(page.Paging.PageSize)),
	}), nil
}

func (s *Server) ListAllAuditLogs(ctx context.Context, req *connect.Request[pb.ListAllAuditLogsRequest]) (*connect.Response[pb.ListAllAuditLogsResponse], error) {
	m := req.Msg
	adminID, ok := parseOptionalID(m.GetActorAdminId())
	if !ok {
		return connect.NewResponse(&pb.ListAllAuditLogsResponse{Code: codeInvalidCommand, Message: "actorAdminId 不正確"}), nil
	}
	storeID, ok := parseOptionalID(m.GetTargetStoreId())
	if !ok {
		return connect.NewResponse(&pb.ListAllAuditLogsResponse{Code: codeInvalidCommand, Message: "targetStoreId 不正確"}), nil
	}
	f, p, bad := auditFilter(m.GetActionFilter(), m.GetResourceTypeFilter(), m.GetResultFilter(), m.GetDateFrom(), m.GetDateTo(), m.GetPage(), m.GetPageSize())
	if bad != "" {
		return connect.NewResponse(&pb.ListAllAuditLogsResponse{Code: codeInvalidCommand, Message: bad}), nil
	}
	f.AdminID, f.StoreID = adminID, storeID
	page, err := s.m.DelegationAuditLogs(ctx, f, p)
	if err != nil {
		code, msg, err := s.reject(ctx, "ListAllAuditLogs", err)
		return connect.NewResponse(&pb.ListAllAuditLogsResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.ListAllAuditLogsResponse{
		Message: success, Records: toAuditEntries(page.Logs), Total: strconv.FormatInt(page.Total, 10),
		Page: strconv.Itoa(int(page.Paging.Page)), PageSize: strconv.Itoa(int(page.Paging.PageSize)),
	}), nil
}

// GetSessionSummary 服務兩支端點（以 URL 的 session 查任意一個、以 credential 查自己的），差別只在 session 從哪裡來。
func (s *Server) GetSessionSummary(ctx context.Context, req *connect.Request[pb.GetSessionSummaryRequest]) (*connect.Response[pb.GetSessionSummaryResponse], error) {
	id, ok := parseID(req.Msg.GetSessionId())
	if !ok {
		return connect.NewResponse(&pb.GetSessionSummaryResponse{Code: codeSessionNotFound, Message: "找不到指定的代行工作階段"}), nil
	}
	sum, err := s.m.DelegationSessionSummary(ctx, id)
	if err != nil {
		code, msg, err := s.reject(ctx, "GetSessionSummary", err)
		return connect.NewResponse(&pb.GetSessionSummaryResponse{Code: code, Message: msg}), err
	}
	sess := sum.Session
	return connect.NewResponse(&pb.GetSessionSummaryResponse{Message: success, Data: &pb.SessionSummaryData{
		Session:                   toSummary(sess),
		SessionId:                 formatID(sess.ID),
		DisplayCode:               sess.DisplayCode,
		Target:                    toTarget(sess.Target),
		Actor:                     toActor(sess.Actor),
		IssuedAt:                  formatTime(sess.IssuedAt),
		EndedAt:                   optionalTime(sess.EndedAt),
		DurationSeconds:           strconv.FormatInt(int64(sum.Duration.Seconds()), 10),
		Status:                    toWire(sess.Status),
		OperationCount:            strconv.FormatInt(sum.OperationCount, 10),
		RiskOperationCount:        strconv.FormatInt(sum.RiskOperationCount, 10),
		PendingAuditCount:         strconv.FormatInt(sum.PendingAuditCount, 10),
		NotifiedSeller:            sum.NotifiedSeller,
		FulfillmentActionCount:    strconv.FormatInt(sum.FulfillmentActionCount, 10),
		FinanceActionCount:        strconv.FormatInt(sum.FinanceActionCount, 10),
		AuditRetentionDays:        strconv.Itoa(sum.AuditRetentionDays),
		RevokedAt:                 optionalTime(sess.RevokedAt),
		RevokedByAdminId:          optionalID(sess.RevokedByAdminID),
		RevokedByAdminDisplayName: sess.RevokedByDisplayName,
		RevokeReason:              sess.RevokeReason,
	}}), nil
}

func (s *Server) ListSessions(ctx context.Context, req *connect.Request[pb.ListSessionsRequest]) (*connect.Response[pb.ListSessionsResponse], error) {
	m := req.Msg
	bad := func(msg string) (*connect.Response[pb.ListSessionsResponse], error) {
		return connect.NewResponse(&pb.ListSessionsResponse{Code: codeInvalidCommand, Message: msg}), nil
	}
	var (
		f  = delegation.DelegationSessionFilter{DisplayCode: m.GetDisplayCode()}
		p  delegation.DelegationPaging
		ok bool
	)
	if f.AdminID, ok = parseOptionalID(m.GetActorAdminId()); !ok {
		return bad("actorAdminId 不正確")
	}
	if f.StoreID, ok = parseOptionalID(m.GetTargetStoreId()); !ok {
		return bad("targetStoreId 不正確")
	}
	if p.Page, ok = parseOptionalInt32(m.GetPage()); !ok {
		return bad("page 不正確")
	}
	if p.PageSize, ok = parseOptionalInt32(m.GetPageSize()); !ok {
		return bad("pageSize 不正確")
	}
	if f.From, ok = parseTime(m.GetDateFrom()); !ok {
		return bad("dateFrom 格式不正確")
	}
	if f.To, ok = parseTime(m.GetDateTo()); !ok {
		return bad("dateTo 格式不正確")
	}
	page, err := s.m.DelegationSessions(ctx, f, p)
	if err != nil {
		code, msg, err := s.reject(ctx, "ListSessions", err)
		return connect.NewResponse(&pb.ListSessionsResponse{Code: code, Message: msg}), err
	}
	out := &pb.ListSessionsResponse{
		Message: success, Records: []*pb.DelegatedOperationSessionRecord{}, Total: strconv.FormatInt(page.Total, 10),
		Page: strconv.Itoa(int(page.Paging.Page)), PageSize: strconv.Itoa(int(page.Paging.PageSize)),
	}
	for _, sess := range page.Sessions {
		out.Records = append(out.Records, &pb.DelegatedOperationSessionRecord{
			SessionId:                 formatID(sess.ID),
			DisplayCode:               sess.DisplayCode,
			Target:                    toTarget(sess.Target),
			Actor:                     toActor(sess.Actor),
			Reason:                    toReason(sess),
			CaseReference:             sess.CaseReference,
			Capabilities:              capabilityStrings(sess.Capabilities),
			IssuedAt:                  formatTime(sess.IssuedAt),
			ExpiresAt:                 formatTime(sess.ExpiresAt),
			EndedAt:                   optionalTime(sess.EndedAt),
			Status:                    toWire(sess.Status),
			RevokedAt:                 optionalTime(sess.RevokedAt),
			RevokedByAdminId:          optionalID(sess.RevokedByAdminID),
			RevokedByAdminDisplayName: sess.RevokedByDisplayName,
			RevokeReason:              sess.RevokeReason,
		})
	}
	return connect.NewResponse(out), nil
}

// toAuditEntries 是稽核列對外的投影。action_label 由動作推導（表沒有這一欄）；resource_image_key 恆為空字串（舊的
// intent 從來沒有寫入它，欄位已經拿掉）。resource_id 原樣回傳（訂單是訂單號；原本存不進 bigint 而是空字串）。
func toAuditEntries(logs []delegation.AuditLog) []*pb.AuditLogEntry {
	out := make([]*pb.AuditLogEntry, 0, len(logs))
	for _, l := range logs {
		e := &pb.AuditLogEntry{
			AuditLogId:    l.ID.String(),
			CreatedAt:     formatTime(l.CreatedAt),
			ConfirmedAt:   optionalTime(l.ConfirmedAt),
			Action:        string(l.Action),
			ActionLabel:   l.Action.Label(),
			RiskLevel:     toWire(l.RiskLevel),
			Capability:    string(l.Capability),
			ResourceType:  toWire(l.ResourceType),
			Result:        string(l.Result),
			CaseReference: l.CaseReference,
			CorrelationId: l.CorrelationID,
			RequestId:     l.RequestID,
			NotifiedAt:    optionalTime(l.NotifiedAt),
			Origin:        toWire(l.Origin),
			Actor:         toActor(l.Actor),
			Target:        toTarget(l.Target),
		}
		e.ResourceId = deref(l.ResourceID)
		e.ResourceName = deref(l.ResourceName)
		e.OperationReason = deref(l.OperationReason)
		e.IdempotencyKey = deref(l.IdempotencyKey)
		e.UserAgent = deref(l.UserAgent)
		if l.BizCode != nil {
			e.BizCode = *l.BizCode
		}
		if l.ResourceVersionBefore != nil {
			e.ResourceVersionBefore = strconv.FormatInt(*l.ResourceVersionBefore, 10)
		}
		if l.ResourceVersionAfter != nil {
			e.ResourceVersionAfter = strconv.FormatInt(*l.ResourceVersionAfter, 10)
		}
		for _, f := range l.ChangedFields {
			e.ChangedFields = append(e.ChangedFields, &pb.ChangedField{Field: f.Field, Label: f.Label, Before: f.Before, After: f.After})
		}
		out = append(out, e)
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
