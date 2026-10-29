// Package identity 是 identity 模組的 Connect handler：登入的帳號（IdentityAppService）與後台
// （IdentityAdminService）。登入、登出由 Clerk 處理，認證在入口的中介層（ADR 2026-09-27-identity-clerk）。
package identity

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

type appIdentity interface {
	Account(ctx context.Context, id int64) (identity.Account, error)
	LinkAppleCredential(ctx context.Context, in identity.LinkAppleCredentialInput) error
}

// App 實作 IdentityAppService。授權（登入的帳號）由 rpc 的 interceptor 依 service 名稱檢查。
type App struct {
	m        appIdentity
	deletion *accountdeletion.Module
}

var _ identityv1alpha1connect.IdentityAppServiceHandler = (*App)(nil)

// NewApp 建立登入後的 handler。
func NewApp(m appIdentity, deletion *accountdeletion.Module) *App {
	return &App{m: m, deletion: deletion}
}

// DeleteAccount 刪除呼叫者的帳號，或回報仍有未完成訂單。
func (h *App) DeleteAccount(ctx context.Context, _ *connect.Request[pb.DeleteAccountRequest]) (*connect.Response[pb.DeleteAccountResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errors.New("httpapi/identity：登入後的請求沒有呼叫者")
	}
	state, err := h.deletion.Request(ctx, accountdeletion.RequestInput{AccountID: p.ID, Requester: accountdeletion.AccountOwner})
	if err != nil {
		return nil, err
	}
	result := pb.DeleteAccountResponse_DELETION_STATE_DELETED
	if state == accountdeletion.PendingOrders {
		result = pb.DeleteAccountResponse_DELETION_STATE_PENDING_ORDERS
	}
	return connect.NewResponse(&pb.DeleteAccountResponse{State: result}), nil
}

// GetSession 回傳登入的帳號；買家身分在建帳交易內建立。帳號停權時另帶停權狀態（入口認證時讀的，A7-D17）。
func (h *App) GetSession(ctx context.Context, _ *connect.Request[pb.GetSessionRequest]) (*connect.Response[pb.GetSessionResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errors.New("httpapi/identity：登入後的請求沒有呼叫者，interceptor 沒有掛上")
	}
	a, err := h.m.Account(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetSessionResponse{Account: account(a)}
	if at, suspended := rpc.SuspensionFrom(ctx); suspended {
		resp.Suspension = &pb.Suspension{SuspendedAt: timestamppb.New(at)}
	}
	return connect.NewResponse(resp), nil
}

// LinkAppleCredential 接收原生 Apple 登入的憑證；換 token 與保存在背景執行。
func (h *App) LinkAppleCredential(ctx context.Context, req *connect.Request[pb.LinkAppleCredentialRequest]) (*connect.Response[pb.LinkAppleCredentialResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errors.New("httpapi/identity：登入後的請求沒有呼叫者，interceptor 沒有掛上")
	}
	if err := h.m.LinkAppleCredential(ctx, identity.LinkAppleCredentialInput{AccountID: p.ID, IDToken: req.Msg.GetIdToken(), AuthorizationCode: req.Msg.GetAuthorizationCode()}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.LinkAppleCredentialResponse{}), nil
}

func account(a identity.Account) *pb.Account {
	roles := make([]pb.Role, len(a.Roles))
	for i, r := range a.Roles {
		roles[i] = rpc.EnumToProto[pb.Role](r)
	}
	return &pb.Account{Id: rpc.FormatID(a.ID), Roles: roles}
}
