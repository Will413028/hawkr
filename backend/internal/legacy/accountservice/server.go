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

// 舊的業務碼（go-pkg/errors）。
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
// 帳號：account_type 的買家位元由 accounttype 合成（帳號活著就是買家）。帳號沒有手機，
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

// GetAccountByPhone 是舊的賣家 CRM 以手機找顧客：帳號沒有手機，一律回「找不到」，
// 與原本查無此手機相同。改用什麼資料找顧客隨 A3（seller、buyer）決定。
func (s *Server) GetAccountByPhone(context.Context, *connect.Request[pb.GetAccountByPhoneRequest]) (*connect.Response[pb.AccountInfoResponse], error) {
	return connect.NewResponse(&pb.AccountInfoResponse{Code: codeNotFound, Message: "這支手機沒有帳號"}), nil
}
