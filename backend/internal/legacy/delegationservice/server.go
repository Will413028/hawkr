// Package delegationservice 是遷移期的暫時轉接：單體替 gateway 與 social-service 提供 account-service 的
// DelegationService（設計書 56 的代行營運），實作是 delegation 模組。代行的編排搬進單體（遷移計畫 A10）、gateway 刪除之後
// 刪除。
//
// wire 的行為照舊（呼叫方是這樣判斷的）：業務的拒絕放在回應的 code 與 message（92xx 的業務碼），成功是 code 0 與
// "success"；其他錯誤一律是 gRPC 的 internal（#3330：gateway 的斷路器把 internal 當成「對方是健康的」，unavailable 會
// 連帶打開整個 account-service 的斷路器）。ID 是字串：管理員、店家與 session 是數字，稽核列與授權列是 UUID。時間是
// ISO 8601 UTC、秒精度。
package delegationservice

import (
	"context"
	"log/slog"
	"strconv"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/account"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/account/accountconnect"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

// Server 實作 DelegationService。RevokeSessionsByActor 沒有實作（回 unimplemented）：
// 後台登出由單體的 HTTP handler 直接呼叫 delegation 撤銷代行。
type Server struct {
	accountconnect.UnimplementedDelegationServiceHandler
	m   *delegation.Module
	log *slog.Logger
}

var _ accountconnect.DelegationServiceHandler = (*Server)(nil)

// New 建立 server。
func New(m *delegation.Module, log *slog.Logger) *Server {
	return &Server{m: m, log: log}
}

// 舊的業務碼（設計書 56 §6.5.1）。
const (
	codeInvalidCommand       = 9260
	codeAdminSessionRequired = 9261
	codeSessionNotFound      = 9277
	codeAuditIntentFailed    = 9279
)

// bizCodes 是模組的拒絕對應的業務碼。每一種拒絕都要有（TestEveryDenialHasABusinessCode）。
var bizCodes = map[delegation.DelegationDenial]int32{
	delegation.DenialInvalidCommand:             codeInvalidCommand,
	delegation.DenialAdminRequired:              codeAdminSessionRequired,
	delegation.DenialCapabilityDenied:           9262,
	delegation.DenialAuthorizationRevoked:       9264,
	delegation.DenialStoreNotDelegable:          9265,
	delegation.DenialStoreClosed:                9265,
	delegation.DenialIdempotencyKeyReused:       9267,
	delegation.DenialActorBusy:                  9268,
	delegation.DenialStoreBusy:                  9269,
	delegation.DenialSessionExpired:             9270,
	delegation.DenialSessionRevoked:             9271,
	delegation.DenialStoreNotOperable:           9272,
	delegation.DenialCredentialInvalid:          9274,
	delegation.DenialSessionNotFound:            codeSessionNotFound,
	delegation.DenialAuditIntentFailed:          codeAuditIntentFailed,
	delegation.DenialHandoffInvalid:             9282,
	delegation.DenialAuthorizationAlreadyActive: 9285,
	delegation.DenialAuthorizationNotFound:      9286,
	delegation.DenialAdminNotAllowlisted:        9288,
}

// reject 把模組的錯誤轉成舊的回應：業務的拒絕回傳 code 與 message（rpcErr 是 nil）；其他錯誤回傳 gRPC 的 internal。
func (s *Server) reject(ctx context.Context, rpc string, err error) (code int32, msg string, rpcErr error) {
	if d, ok := delegation.DelegationDenialOf(err); ok {
		if c, ok := bizCodes[d.Denial]; ok {
			return c, d.Message(), nil
		}
	}
	s.log.ErrorContext(ctx, "delegationservice：失敗", "rpc", rpc, "err", err)
	return 0, "", connect.NewError(connect.CodeInternal, err)
}

const success = "success"

func (s *Server) Preflight(ctx context.Context, req *connect.Request[pb.PreflightRequest]) (*connect.Response[pb.PreflightResponse], error) {
	adminID, ok := parseID(req.Msg.GetActorAdminId())
	if !ok {
		return connect.NewResponse(&pb.PreflightResponse{Code: codeInvalidCommand, Message: "actorAdminId 不正確"}), nil
	}
	storeID, ok := parseID(req.Msg.GetTargetStoreId())
	if !ok {
		return connect.NewResponse(&pb.PreflightResponse{Code: codeInvalidCommand, Message: "targetStoreId 不正確"}), nil
	}
	res, err := s.m.DelegationPreflight(ctx, delegation.DelegationPreflightInput{
		AdminID: adminID, StoreID: storeID,
	})
	if err != nil {
		code, msg, err := s.reject(ctx, "Preflight", err)
		return connect.NewResponse(&pb.PreflightResponse{Code: code, Message: msg}), err
	}
	target := toTarget(res.Target)
	if !res.TargetKnown {
		// 契約要求非空；還沒讀到店家的拒絕只顯示拒絕的原因，不顯示店家。
		target.SellerAccountId, target.StoreName = "0", "（查無此店家）"
	}
	data := &pb.PreflightData{
		Allowed:               res.Allowed,
		Target:                target,
		Actor:                 toActor(res.Actor),
		Capabilities:          capabilityStrings(res.Capabilities),
		TtlSeconds:            int32(res.TTL / time.Second),         //nolint:gosec // module TTL is fixed at 1800 seconds
		IdleSeconds:           int32(res.IdleTimeout / time.Second), //nolint:gosec // module idle timeout is fixed at 900 seconds
		CaseReferenceRequired: res.CaseReferenceRequired,
		DeniedLabel:           res.DeniedLabel,
	}
	if !res.Allowed {
		data.DeniedCode = bizCodes[res.Denial]
	}
	for _, o := range res.ReasonOptions {
		data.ReasonOptions = append(data.ReasonOptions, &pb.ReasonOption{
			Code: toWire(o.Code), Label: o.Label, CaseReferenceRequired: o.CaseReferenceRequired,
		})
	}
	return connect.NewResponse(&pb.PreflightResponse{Message: success, Data: data}), nil
}

func (s *Server) CreateSession(ctx context.Context, req *connect.Request[pb.CreateSessionRequest]) (*connect.Response[pb.CreateSessionResponse], error) {
	m := req.Msg
	adminID, ok := parseID(m.GetActorAdminId())
	if !ok {
		return connect.NewResponse(&pb.CreateSessionResponse{Code: codeInvalidCommand, Message: "actorAdminId 不正確"}), nil
	}
	storeID, ok := parseID(m.GetTargetStoreId())
	if !ok {
		return connect.NewResponse(&pb.CreateSessionResponse{Code: codeInvalidCommand, Message: "storeId 不正確"}), nil
	}
	res, err := s.m.StartDelegation(ctx, delegation.StartDelegationInput{
		AdminID:          adminID,
		AdminDisplayName: m.GetActorDisplayName(),
		StoreID:          storeID,
		Reason:           fromWire[delegation.DelegationReason](m.GetReasonCode()),
		// proto3 optional：沒有帶是 null（合法），空字串是帶了一個不合法的值。
		CaseReference:  m.CaseReference,
		Capabilities:   toCapabilities(m.GetRequestedCapabilities()),
		TTLSeconds:     m.GetRequestedTtlSeconds(),
		IdempotencyKey: m.GetIdempotencyKey(),
		RequestID:      m.GetRequestId(),
		CorrelationID:  m.GetCorrelationId(),
		UserAgent:      m.GetUserAgent(),
	})
	if err != nil {
		code, msg, err := s.reject(ctx, "CreateSession", err)
		return connect.NewResponse(&pb.CreateSessionResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.CreateSessionResponse{Message: success, Data: &pb.CreateSessionData{
		HandoffCode: res.HandoffCode, Session: toSummary(res.Session),
	}}), nil
}

func (s *Server) RedeemHandoff(ctx context.Context, req *connect.Request[pb.RedeemHandoffRequest]) (*connect.Response[pb.RedeemHandoffResponse], error) {
	res, err := s.m.RedeemDelegationHandoff(ctx, req.Msg.GetHandoffCode())
	if err != nil {
		code, msg, err := s.reject(ctx, "RedeemHandoff", err)
		return connect.NewResponse(&pb.RedeemHandoffResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.RedeemHandoffResponse{Message: success, Data: &pb.RedeemHandoffData{
		OperationCredential: res.Credential, Session: toSummary(res.Session),
	}}), nil
}

func (s *Server) GetCurrentSession(ctx context.Context, req *connect.Request[pb.GetCurrentSessionRequest]) (*connect.Response[pb.GetCurrentSessionResponse], error) {
	sess, err := s.m.ResolveDelegation(ctx, req.Msg.GetOperationCredential(), req.Msg.GetCountsAsActivity())
	if err != nil {
		code, msg, err := s.reject(ctx, "GetCurrentSession", err)
		return connect.NewResponse(&pb.GetCurrentSessionResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.GetCurrentSessionResponse{Message: success, Data: toSummary(sess)}), nil
}

func (s *Server) EndSession(ctx context.Context, req *connect.Request[pb.EndSessionRequest]) (*connect.Response[pb.EndSessionResponse], error) {
	if err := s.m.EndDelegation(ctx, req.Msg.GetOperationCredential()); err != nil {
		code, msg, err := s.reject(ctx, "EndSession", err)
		return connect.NewResponse(&pb.EndSessionResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.EndSessionResponse{Message: success, Data: &pb.EndSessionData{Ended: true}}), nil
}

func (s *Server) RevokeSession(ctx context.Context, req *connect.Request[pb.RevokeSessionRequest]) (*connect.Response[pb.RevokeSessionResponse], error) {
	sessionID, ok := parseID(req.Msg.GetSessionId())
	if !ok {
		return connect.NewResponse(&pb.RevokeSessionResponse{Code: codeSessionNotFound, Message: "找不到指定的代行工作階段"}), nil
	}
	adminID, ok := parseID(req.Msg.GetActorAdminId())
	if !ok {
		return connect.NewResponse(&pb.RevokeSessionResponse{Code: codeInvalidCommand, Message: "actorAdminId 不正確"}), nil
	}
	sess, err := s.m.RevokeDelegation(ctx, delegation.RevokeDelegationInput{
		SessionID: sessionID, AdminID: adminID, Reason: req.Msg.GetRevokeReason(),
	})
	if err != nil {
		code, msg, err := s.reject(ctx, "RevokeSession", err)
		return connect.NewResponse(&pb.RevokeSessionResponse{Code: code, Message: msg}), err
	}
	return connect.NewResponse(&pb.RevokeSessionResponse{Message: success, Data: &pb.RevokeSessionData{
		SessionId: formatID(sess.ID), Status: toWire(delegation.DelegationRevoked), Revision: int64(sess.Revision),
		RevokedAt: formatTimePtr(sess.RevokedAt),
	}}), nil
}

// ---- 轉換 ----

// parseID 解析管理員、店家與代行 session 的 ID（正的十進位整數）。
func parseID(s string) (int64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil && v > 0
}

func parseUUID(s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	return id, err == nil
}

// parseOptionalID 解析選填的 ID 篩選：空字串是不篩選。
func parseOptionalID(s string) (*int64, bool) {
	if s == "" {
		return nil, true
	}
	v, ok := parseID(s)
	return &v, ok
}

// optionalString 是沒有帶（空字串）時的 nil。
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// parseOptionalInt64 是舊的寬鬆解析：不是數字時當作沒有帶（資源的版本）。
func parseOptionalInt64(s string) *int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if s == "" || err != nil {
		return nil
	}
	return &v
}

func parseOptionalInt32(s string) (int32, bool) {
	if s == "" {
		return 0, true
	}
	v, err := strconv.ParseInt(s, 10, 32)
	return int32(v), err == nil
}

// parseTime 解析 ISO 8601（RFC 3339，可以有小數秒）；空字串是沒有帶。
func parseTime(s string) (*time.Time, bool) {
	if s == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil, false
	}
	t = t.UTC()
	return &t, true
}

// formatTime 是對外的時間：UTC、截到秒（前端的格式最多三位小數，TIMESTAMPTZ 是微秒）。
func formatTime(t time.Time) string { return t.UTC().Truncate(time.Second).Format(time.RFC3339) }

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

// optionalTime 是 proto3 optional 的時間：nil 輸出 JSON null。
func optionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := formatTime(*t)
	return &v
}

func optionalID(id *int64) *string {
	if id == nil {
		return nil
	}
	v := strconv.FormatInt(*id, 10)
	return &v
}

func formatID(id int64) string { return strconv.FormatInt(id, 10) }

func toCapabilities(s []string) []delegation.Capability {
	out := make([]delegation.Capability, len(s))
	for i, c := range s {
		out[i] = delegation.Capability(c)
	}
	return out
}

func capabilityStrings(c []delegation.Capability) []string {
	out := make([]string, len(c))
	for i, v := range c {
		out[i] = string(v)
	}
	return out
}

func toTarget(t delegation.DelegationTarget) *pb.DelegationTarget {
	return &pb.DelegationTarget{StoreId: formatID(t.StoreID), SellerAccountId: formatID(t.SellerAccountID), StoreName: t.StoreName}
}

func toActor(a delegation.DelegationActor) *pb.DelegationActor {
	return &pb.DelegationActor{AdminId: formatID(a.AdminID), DisplayName: a.DisplayName}
}

func toReason(s delegation.DelegationSession) *pb.DelegationReason {
	return &pb.DelegationReason{Code: toWire(s.Reason), Label: s.ReasonLabel, CaseReferenceRequired: s.Reason.CaseReferenceRequired()}
}

// toSummary 是 create、current 與兌換共用的投影。不含任何 credential 或 token；連續兩次讀取必須逐位元相同（前端以
// 九個欄位對帳）。case_reference 沒有時是 null，不是省略。
func toSummary(s delegation.DelegationSession) *pb.OperationSessionSummary {
	return &pb.OperationSessionSummary{
		SessionId:     formatID(s.ID),
		DisplayCode:   s.DisplayCode,
		Target:        toTarget(s.Target),
		Actor:         toActor(s.Actor),
		Reason:        toReason(s),
		CaseReference: s.CaseReference,
		Capabilities:  capabilityStrings(s.Capabilities),
		IssuedAt:      formatTime(s.IssuedAt),
		ExpiresAt:     formatTime(s.ExpiresAt),
		Status:        toWire(s.Status),
		Revision:      int64(s.Revision),
	}
}
