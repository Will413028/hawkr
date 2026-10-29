// Package accountservice 是遷移期的暫時轉接：單體替舊服務提供 account-service 的 gRPC（AccountService），讓還沒搬進
// 單體的呼叫方（seller、social、crm、coupon 與 gateway）照舊呼叫，實作是 identity 模組。呼叫方都搬進單體
// （遷移計畫 A3、A6、A7、B7，以及 gateway 刪除）之後刪除。
//
// 只實作仍有呼叫方的方法（遷移計畫的 rpc-calls.csv）；其餘由 Unimplemented 回 unimplemented。業務錯誤照舊放在回應的
// code（舊的 go-pkg/errors：400、404），不是 gRPC 的 status：呼叫方是這樣判斷的。服務間的 HMAC 簽章不驗：legacy 的
// listener 只存在於本機與 CI（gateway 與舊服務不會部署，〈API 設計規範（Connect）〉§二）。
package accountservice

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/legacy/accounttype"
	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/account"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/account/accountconnect"
	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// 舊的業務碼（go-pkg/errors）。
const (
	codeInvalidInput = 400
	codeNotFound     = 404
)

// Suspender 是停權的入口（accountsuspension.Module）。
type Suspender interface {
	Suspend(ctx context.Context, accountID int64, rec accountsuspension.Record) (accountsuspension.Suspension, error)
}

// Server 實作 AccountService 的子集。
type Server struct {
	accountconnect.UnimplementedAccountServiceHandler
	m         *identity.Module
	suspender Suspender
	log       *slog.Logger
}

var _ accountconnect.AccountServiceHandler = (*Server)(nil)

// New 建立 server。
func New(m *identity.Module, suspender Suspender, log *slog.Logger) *Server {
	return &Server{m: m, suspender: suspender, log: log}
}

// businessCode 把模組的錯誤轉成舊的業務碼；沒有分類的錯誤回 error（gRPC 的 internal）。
func businessCode(err error) (int32, string, error) {
	switch apperr.KindOf(err) {
	case apperr.KindNotFound:
		return codeNotFound, err.Error(), nil
	case apperr.KindInvalidInput:
		return codeInvalidInput, err.Error(), nil
	case apperr.KindInternal, apperr.KindVersionConflict, apperr.KindAlreadyExists,
		apperr.KindFailedPrecondition, apperr.KindUnauthenticated, apperr.KindPermissionDenied,
		apperr.KindUnavailable, apperr.KindResourceExhausted:
		return 0, "", err
	default:
		return 0, "", err
	}
}

// accountInfo 是舊的 AccountInfo。刪除的帳號在舊的 account-service 查不到（404），這裡也一樣，所以 a 一定是存活的
// 帳號：account_type 的買家位元由 accounttype 合成（帳號活著就是買家）。帳號不再有手機（ADR 2026-09-27-identity-clerk），
// phone 一律是空字串。
func accountInfo(a identity.Account) *pb.AccountInfo {
	status := "active"
	if a.Suspended {
		status = "suspended"
	}
	return &pb.AccountInfo{
		AccountId: a.ID, Email: a.Email, Status: status, AccountType: int32(accounttype.OfLiveAccount(a.Roles)), //nolint:gosec // role bitmask is only 1 through 3
		CreatedAt: a.CreatedAt.Unix(), UpdatedAt: a.CreatedAt.Unix(),
	}
}

func (s *Server) GetAccountInfo(ctx context.Context, req *connect.Request[pb.GetAccountInfoRequest]) (*connect.Response[pb.AccountInfoResponse], error) {
	a, err := s.m.Account(ctx, req.Msg.GetAccountId())
	if err == nil && a.Deleted {
		err = apperr.NotFound("帳號 %d 已刪除", a.ID)
	}
	if err != nil {
		code, msg, err := businessCode(err)
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(&pb.AccountInfoResponse{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.AccountInfoResponse{Data: accountInfo(a)}), nil
}

func (s *Server) GetAccountInfoBatch(ctx context.Context, req *connect.Request[pb.GetAccountInfoBatchRequest]) (*connect.Response[pb.AccountInfoBatchResponse], error) {
	accounts, err := s.m.Accounts(ctx, req.Msg.GetAccountIds())
	if err != nil {
		return nil, err
	}
	out := &pb.AccountInfoBatchResponse{}
	for _, a := range accounts {
		if !a.Deleted {
			out.Data = append(out.Data, accountInfo(a))
		}
	}
	return connect.NewResponse(out), nil
}

// GetAccountByPhone 是舊的賣家 CRM 以手機找顧客：帳號不再有手機（ADR 2026-09-27-identity-clerk），一律回「找不到」，
// 與原本查無此手機相同。改用什麼資料找顧客隨 A3（seller、buyer）決定。
func (s *Server) GetAccountByPhone(context.Context, *connect.Request[pb.GetAccountByPhoneRequest]) (*connect.Response[pb.AccountInfoResponse], error) {
	return connect.NewResponse(&pb.AccountInfoResponse{Code: codeNotFound, Message: "這支手機沒有帳號"}), nil
}

// sourceOf 是舊契約的 source_id：0 表示沒有帶。
func sourceOf(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

// RemoveRole 只移除 SELLER。買家不是角色，舊的 ROLE_BUYER 一律拒絕：買家只隨整個帳號刪除（accountdeletion），
// 單獨移除會留下沒有買家的帳號。移除 SELLER 永遠不刪帳號（帳號活著就是買家），所以 other_role_state 不再有作用，
// account_fully_deleted 一律是 false。
func (s *Server) RemoveRole(ctx context.Context, req *connect.Request[pb.RemoveRoleRequest]) (*connect.Response[pb.RemoveRoleResponse], error) {
	switch req.Msg.GetRole() {
	case pb.Role_ROLE_SELLER:
	case pb.Role_ROLE_BUYER:
		return connect.NewResponse(&pb.RemoveRoleResponse{Code: codeInvalidInput, Message: "買家不能單獨移除，要刪除整個帳號"}), nil
	case pb.Role_ROLE_UNSPECIFIED:
		return connect.NewResponse(&pb.RemoveRoleResponse{Code: codeInvalidInput, Message: "unknown role"}), nil
	default:
		return connect.NewResponse(&pb.RemoveRoleResponse{Code: codeInvalidInput, Message: "unknown role"}), nil
	}
	err := s.m.RemoveRole(ctx, identity.RemoveRoleInput{
		AccountID: req.Msg.GetAccountId(), Role: identity.RoleSeller, SourceID: sourceOf(req.Msg.GetSourceId()),
	})
	if err != nil {
		code, msg, err := businessCode(err)
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(&pb.RemoveRoleResponse{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.RemoveRoleResponse{}), nil
}

// SuspendAccount 是 social 的 BAN_USER：舊的實作在錯誤時回 gRPC 的錯誤（沒有業務碼），這裡照舊。理由類別一律是 OTHER，舊的
// reason 字串是說明。
func (s *Server) SuspendAccount(ctx context.Context, req *connect.Request[pb.SuspendAccountRequest]) (*connect.Response[pb.SuspendAccountResponse], error) {
	_, err := s.suspender.Suspend(ctx, req.Msg.GetAccountId(), accountsuspension.Record{Category: policy.Other, Reason: req.Msg.GetReason()})
	if err != nil {
		if k := apperr.KindOf(err); k == apperr.KindNotFound {
			return nil, connect.NewError(connect.CodeNotFound, err)
		} else if k == apperr.KindFailedPrecondition {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, err
	}
	return connect.NewResponse(&pb.SuspendAccountResponse{Success: true}), nil
}
