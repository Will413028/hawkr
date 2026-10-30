// Package identity 是 identity 模組的 Connect handler：登入的帳號（IdentityAppService）與後台
// （IdentityAdminService）。登入、登出由 Clerk 處理，認證在入口的中介層（ADR 2026-09-27-identity-clerk）。
package identity

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

type appIdentity interface {
	Account(ctx context.Context, id int64) (identity.Account, error)
	LinkAppleCredential(ctx context.Context, in identity.LinkAppleCredentialInput) error
	LinkIdentity(ctx context.Context, accountID int64, id identity.ProviderIdentity) error
	ListSessions(ctx context.Context, accountID int64) ([]identity.SessionInfo, error)
	ListLinkedIdentities(ctx context.Context, accountID int64) ([]identity.LinkedIdentity, error)
	RevokeSession(ctx context.Context, accountID int64, sessionID identity.SessionID) error
}

// App 實作 IdentityAppService。授權（登入的帳號）由 rpc 的 interceptor 依 service 名稱檢查。
type App struct {
	m        appIdentity
	deletion *accountdeletion.Module
	login    *AppLogin
	now      func() time.Time
}

var _ identityv1alpha1connect.IdentityAppServiceHandler = (*App)(nil)

// NewApp 建立登入後的 handler。login 驗證 LinkIdentity 的 ID token；now 判斷 RevokeSession 的重新驗證期限。
func NewApp(m appIdentity, deletion *accountdeletion.Module, login *AppLogin, now func() time.Time) *App {
	return &App{m: m, deletion: deletion, login: login, now: now}
}

// 列表的每頁筆數：一個帳號的有效 session 通常只有幾個。
const (
	defaultSessionPageSize = 20
	maxSessionPageSize     = 100
)

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

// caller 回傳登入的帳號與它這次請求所用的 session；interceptor 保證登入的帳號有，沒有是組裝錯誤。
func caller(ctx context.Context) (identity.Principal, rpc.Session, error) {
	p, ok := rpc.PrincipalFrom(ctx)
	s, hasSession := rpc.SessionFrom(ctx)
	if !ok || !hasSession {
		return p, s, errors.New("httpapi/identity：登入後的請求沒有帳號或 session，interceptor 沒有掛上")
	}
	return p, s, nil
}

// SignOut 撤銷目前的 session；web（cookie 認證）的回應另清掉 session cookie。session 已經不在時也成功。
func (h *App) SignOut(ctx context.Context, _ *connect.Request[pb.SignOutRequest]) (*connect.Response[pb.SignOutResponse], error) {
	p, s, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.m.RevokeSession(ctx, p.ID, s.ID); err != nil && apperr.KindOf(err) != apperr.KindNotFound {
		return nil, err
	}
	resp := connect.NewResponse(&pb.SignOutResponse{})
	if s.Source == rpc.CredentialCookie {
		resp.Header().Add("Set-Cookie", expiredCookie(rpc.SessionCookie).String())
	}
	return resp, nil
}

// LinkIdentity 把 App 原生登入的身分連結到呼叫者的帳號；身分已屬於別的帳號回 already_exists 與 IDENTITY_ALREADY_LINKED。
func (h *App) LinkIdentity(ctx context.Context, req *connect.Request[pb.LinkIdentityRequest]) (*connect.Response[pb.LinkIdentityResponse], error) {
	p, _, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := h.login.identityOf(ctx, req.Msg.GetProvider(), req.Msg.GetIdToken(), req.Msg.GetNonce())
	if err != nil {
		return nil, err
	}
	if err := h.m.LinkIdentity(ctx, p.ID, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.LinkIdentityResponse{}), nil
}

// sessionCursor 是 ListSessions 上一頁最後一筆的排序鍵（建立時間新到舊，同時間以 ID）。
type sessionCursor struct {
	Time time.Time `json:"t"`
	ID   uuid.UUID `json:"i"`
}

func (c sessionCursor) Valid() bool { return !c.Time.IsZero() && c.ID != uuid.Nil() }

// after 回報 s 在排序上是不是排在 c 之後（建立時間較舊，或同時間而 ID 較小）。
func (c sessionCursor) after(s identity.SessionInfo) bool {
	return s.CreatedAt.Before(c.Time) || s.CreatedAt.Equal(c.Time) && s.ID.Compare(c.ID) < 0
}

// ListSessions 列出呼叫者的有效 session（新到舊）；已逾時但還沒被清掉的不列。
func (h *App) ListSessions(ctx context.Context, req *connect.Request[pb.ListSessionsRequest]) (*connect.Response[pb.ListSessionsResponse], error) {
	p, cur, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	size, err := pagination.PageSize(req.Msg.GetPageSize(), defaultSessionPageSize, maxSessionPageSize)
	if err != nil {
		return nil, rpc.RequiredField("page_size", "page_size 不可為負數")
	}
	scope := rpc.PageScope(identityv1alpha1connect.IdentityAppServiceListSessionsProcedure, p.ID)
	after, err := rpc.ValidCursor[sessionCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	all, err := h.m.ListSessions(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	now := h.now()
	out := &pb.ListSessionsResponse{}
	var last identity.SessionInfo
	for _, s := range all {
		if identity.SessionExpired(s, now) || after != nil && !after.after(s) {
			continue
		}
		if len(out.Sessions) == int(size) {
			out.NextPageToken = rpc.TokenFor(scope, sessionCursor{Time: last.CreatedAt, ID: last.ID})
			break
		}
		out.Sessions = append(out.Sessions, &pb.LoginSession{
			Id: s.ID.String(), ClientKind: rpc.EnumToProto[pb.ClientKind](s.Client), CreateTime: timestamppb.New(s.CreatedAt),
			LastUseTime: timestamppb.New(s.LastUsedAt), Current: s.ID == cur.ID,
		})
		last = s
	}
	return connect.NewResponse(out), nil
}

// ListLinkedIdentities 列出呼叫者的帳號已連結的登入身分（連結時間舊到新）；只回 provider 與連結時間。
func (h *App) ListLinkedIdentities(ctx context.Context, _ *connect.Request[pb.ListLinkedIdentitiesRequest]) (*connect.Response[pb.ListLinkedIdentitiesResponse], error) {
	p, _, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	all, err := h.m.ListLinkedIdentities(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := &pb.ListLinkedIdentitiesResponse{}
	for _, li := range all {
		provider := rpc.EnumToProto[pb.LoginProvider](li.Provider)
		if provider == pb.LoginProvider_LOGIN_PROVIDER_UNSPECIFIED {
			return nil, fmt.Errorf("httpapi/identity：帳號 %d 有不認得的登入方式 %q", p.ID, li.Provider)
		}
		out.Identities = append(out.Identities, &pb.LinkedIdentity{Provider: provider, LinkTime: timestamppb.New(li.LinkedAt)})
	}
	return connect.NewResponse(out), nil
}

// RevokeSession 終止呼叫者自己的一個 session：目前的 session 必須在 ReauthWindow 內以 provider 驗證過。
func (h *App) RevokeSession(ctx context.Context, req *connect.Request[pb.RevokeSessionRequest]) (*connect.Response[pb.RevokeSessionResponse], error) {
	p, cur, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	target, err := rpc.ParseUUID("session_id", req.Msg.GetSessionId())
	if err != nil {
		return nil, err
	}
	all, err := h.m.ListSessions(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(all, func(s identity.SessionInfo) bool { return s.ID == cur.ID })
	if i < 0 {
		return nil, apperr.Unauthenticated("登入已失效，請重新登入")
	}
	if err := requireRecentAuthentication(all[i].AuthTime, h.now()); err != nil {
		return nil, err
	}
	if err := h.m.RevokeSession(ctx, p.ID, target); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RevokeSessionResponse{}), nil
}

// requireRecentAuthentication 要求最近一次以 provider 驗證的時間 authTime 不早於 now 之前的 ReauthWindow（ASVS；計畫 2026-10-30-self-hosted-auth
// 步驟 2「session 管理」）：不符回 failed_precondition 與 REAUTHENTICATION_REQUIRED。
func requireRecentAuthentication(authTime, now time.Time) error {
	if now.Sub(authTime) > identity.ReauthWindow {
		return apperr.FailedPrecondition("這個操作需要重新登入確認身分").WithReason(apperr.ReasonReauthenticationRequired)
	}
	return nil
}

func account(a identity.Account) *pb.Account {
	roles := make([]pb.Role, len(a.Roles))
	for i, r := range a.Roles {
		roles[i] = rpc.EnumToProto[pb.Role](r)
	}
	return &pb.Account{Id: rpc.FormatID(a.ID), Roles: roles}
}
