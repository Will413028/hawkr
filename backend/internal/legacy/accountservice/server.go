// Package accountservice 是遷移期的暫時轉接：單體替舊服務提供 account-service 的 gRPC（AccountService），讓還沒搬進
// 單體的呼叫方（seller、crm、coupon 與 gateway）照舊呼叫，實作是 identity 模組。呼叫方都搬進單體
// （遷移計畫 A3、A6、B7，以及 gateway 刪除）之後刪除。
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
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	codeInvalidInput = 400
	codeNotFound     = 404
)

// Server 實作 AccountService 的子集。
type Server struct {
	accountconnect.UnimplementedAccountServiceHandler
	m   *identity.Module
	log *slog.Logger
}

var _ accountconnect.AccountServiceHandler = (*Server)(nil)

// New 建立 server。
func New(m *identity.Module, log *slog.Logger) *Server {
	return &Server{m: m, log: log}
}

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
