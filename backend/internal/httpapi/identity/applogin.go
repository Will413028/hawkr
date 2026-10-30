package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// App 的原生登入（ADR 2026-10-30-identity-self-hosted-federated-login D2）：原生 SDK 給 App 的 ID token 換 session，或連結到已登入的帳號。
// nonce 是伺服器發的單次使用值，ID token 的 nonce claim 必須等於它。

// AppVerifier 驗證一家 provider 的原生 SDK 簽發給 App 的 ID token；*identity.AppLogin 滿足它。
type AppVerifier interface {
	Provider() identity.Provider
	// Verify 驗證 ID token，nonce claim 必須等於 wantNonce；無效的 token 與 nonce 不符回含 identity.ErrInvalidToken 的錯誤，其他錯誤是 provider 故障。
	Verify(ctx context.Context, idToken, wantNonce string) (identity.ProviderIdentity, error)
}

// appLoginModule 是 App 登入需要的 identity 方法；*identity.Module 滿足它。
type appLoginModule interface {
	IssueAppNonce(ctx context.Context) (string, time.Time, error)
	ConsumeAppNonce(ctx context.Context, nonce string) error
	Login(ctx context.Context, in identity.LoginInput) (identity.LoginResult, error)
	LinkIdentity(ctx context.Context, accountID int64, id identity.ProviderIdentity) error
}

// AppLogin 是 App 原生登入的共用部分：IdentityPublicService（換 session）與 IdentityAppService.LinkIdentity 都經它驗證 ID token 與 nonce。
type AppLogin struct {
	m         appLoginModule
	verifiers map[identity.Provider]AppVerifier
}

// NewAppLogin 建立 App 登入；沒有設定驗證器的 provider，ExchangeIdToken 與 LinkIdentity 回 unavailable。同一家設定兩次是錯誤。
func NewAppLogin(m appLoginModule, verifiers ...AppVerifier) (*AppLogin, error) {
	if m == nil {
		return nil, errors.New("httpapi：App 登入需要 identity 模組")
	}
	a := &AppLogin{m: m, verifiers: map[identity.Provider]AppVerifier{}}
	for _, v := range verifiers {
		if !v.Provider().Valid() {
			return nil, fmt.Errorf("httpapi：App 登入的 provider %q 不是已知的值", v.Provider())
		}
		if _, dup := a.verifiers[v.Provider()]; dup {
			return nil, fmt.Errorf("httpapi：App 登入的 provider %q 設定了兩次", v.Provider())
		}
		a.verifiers[v.Provider()] = v
	}
	return a, nil
}

// identityOf 驗證請求帶的 ID token 與 nonce，回傳 provider 的身分：先驗證 ID token（含 nonce claim 與請求的 nonce 相同），通過才消耗 nonce
// （單次使用）。無效的 ID token 不消耗 nonce，所以拿到別人 nonce 的人不能用垃圾請求把它作廢。
func (a *AppLogin) identityOf(ctx context.Context, provider pb.LoginProvider, idToken, nonce string) (identity.ProviderIdentity, error) {
	if provider == pb.LoginProvider_LOGIN_PROVIDER_UNSPECIFIED {
		return identity.ProviderIdentity{}, rpc.RequiredField("provider", "provider 必填")
	}
	p := rpc.EnumFromProto[identity.Provider](provider)
	if !p.Valid() {
		return identity.ProviderIdentity{}, rpc.Error(connect.CodeInvalidArgument, "provider %s 不是已知的登入方式", provider)
	}
	if idToken == "" {
		return identity.ProviderIdentity{}, rpc.RequiredField("id_token", "id_token 必填")
	}
	if nonce == "" {
		return identity.ProviderIdentity{}, rpc.RequiredField("nonce", "nonce 必填")
	}
	v, ok := a.verifiers[p]
	if !ok {
		return identity.ProviderIdentity{}, apperr.Unavailable("這個登入方式目前無法使用")
	}
	id, err := v.Verify(ctx, idToken, nonce)
	switch {
	case errors.Is(err, identity.ErrInvalidToken):
		return identity.ProviderIdentity{}, apperr.WithCause(apperr.Unauthenticated("登入憑證無效，請重新登入"), err)
	case err != nil:
		return identity.ProviderIdentity{}, apperr.WithCause(apperr.Unavailable("暫時無法向登入服務驗證，請稍後再試"), err)
	}
	if err := a.m.ConsumeAppNonce(ctx, nonce); err != nil {
		return identity.ProviderIdentity{}, err
	}
	return id, nil
}

// Public 實作 IdentityPublicService。
type Public struct{ login *AppLogin }

var _ identityv1alpha1connect.IdentityPublicServiceHandler = (*Public)(nil)

// NewPublic 建立登入前的 handler。
func NewPublic(login *AppLogin) *Public { return &Public{login: login} }

// IssueAppLoginNonce 發一個單次使用的 nonce。
func (h *Public) IssueAppLoginNonce(ctx context.Context, _ *connect.Request[pb.IssueAppLoginNonceRequest]) (*connect.Response[pb.IssueAppLoginNonceResponse], error) {
	nonce, expires, err := h.login.m.IssueAppNonce(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.IssueAppLoginNonceResponse{Nonce: nonce, ExpireTime: timestamppb.New(expires)}), nil
}

// ExchangeIdToken 以原生 SDK 的 ID token 登入，換發 App 種類的 session；請求帶 Bearer 的舊 session 時一併撤銷（登入一律換發新 session）。
func (h *Public) ExchangeIdToken(ctx context.Context, req *connect.Request[pb.ExchangeIdTokenRequest]) (*connect.Response[pb.ExchangeIdTokenResponse], error) {
	id, err := h.login.identityOf(ctx, req.Msg.GetProvider(), req.Msg.GetIdToken(), req.Msg.GetNonce())
	if err != nil {
		return nil, err
	}
	previous := ""
	if token, source := rpc.CredentialFromHeader(req.Header()); source == rpc.CredentialBearer {
		previous = token
	}
	res, err := h.login.m.Login(ctx, identity.LoginInput{Identity: id, Client: identity.ClientApp, PreviousToken: previous})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ExchangeIdTokenResponse{
		SessionToken:       res.Token,
		AbsoluteExpireTime: timestamppb.New(res.AbsoluteExpiresAt),
		IdleTimeout:        durationpb.New(identity.SessionIdleTimeout(identity.ClientApp)),
	}), nil
}
