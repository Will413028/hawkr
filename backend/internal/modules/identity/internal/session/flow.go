package session

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// maxReturnPath 是返回網址的長度上限。
const maxReturnPath = 2048

func errFlowInvalid() error { return apperr.Unauthenticated("登入流程已失效，請重新登入") }

// ValidReturnPath 回報 p 是不是可以安全轉址的返回網址：只收 / 開頭的相對路徑，不是 //（協定相對網址）也不是 /\（部分瀏覽器當成
// //），不含控制字元，長度不超過上限（自建登入 ADR D3）。
func ValidReturnPath(p string) bool {
	if p == "" || len(p) > maxReturnPath || p[0] != '/' {
		return false
	}
	if len(p) > 1 && (p[1] == '/' || p[1] == '\\') {
		return false
	}
	return !strings.ContainsFunc(p, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

// BeginLoginFlow 寫一列 web 登入流程的暫存狀態（10 分鐘、單次使用）並回傳要交給 provider 與瀏覽器的原文值。順手清掉逾時的列。
func (s *Service) BeginLoginFlow(ctx context.Context, in model.LoginFlowInput) (model.LoginFlowStart, error) {
	if !in.Provider.Valid() {
		return model.LoginFlowStart{}, apperr.InvalidInputOn([]string{"Provider"}, "不支援的登入方式 %q", in.Provider)
	}
	if !ValidReturnPath(in.ReturnPath) {
		return model.LoginFlowStart{}, apperr.InvalidInputOn([]string{"ReturnPath"}, "返回網址必須是站內的相對路徑")
	}
	var secrets [4]string
	for i := range secrets {
		v, err := newSecret()
		if err != nil {
			return model.LoginFlowStart{}, err
		}
		secrets[i] = v
	}
	start := model.LoginFlowStart{State: secrets[0], Nonce: secrets[1], CodeVerifier: secrets[2], Binding: secrets[3]}
	now := s.d.Now()
	q := s.q(ctx)
	if _, err := q.DeleteExpiredLoginFlows(ctx, now); err != nil {
		return model.LoginFlowStart{}, fmt.Errorf("identity：清除逾時的登入流程：%w", err)
	}
	if err := q.InsertLoginFlow(ctx, store.InsertLoginFlowParams{
		StateHash: Hash(start.State), Provider: string(in.Provider), CodeVerifier: start.CodeVerifier, Nonce: start.Nonce,
		ReturnPath: in.ReturnPath, BindingHash: Hash(start.Binding), LinkAccountID: in.LinkAccountID,
		Now: now, ExpiresAt: now.Add(FlowTTL),
	}); err != nil {
		return model.LoginFlowStart{}, fmt.Errorf("identity：寫入登入流程：%w", err)
	}
	return start, nil
}

// ConsumeLoginFlow 以 state 取出登入流程並同時刪除：state 重放、逾時、provider 不是預期的（callback 的路徑與 start 的不同，防
// mix-up）、登入前 cookie 不符（防 login CSRF）都回 KindUnauthenticated，而且那一列已經作廢。
//
// 必須在沒有會 rollback 的 transaction 內呼叫：失敗要保留「已刪除」。
func (s *Service) ConsumeLoginFlow(ctx context.Context, state, binding string, want model.Provider) (model.LoginFlow, error) {
	if len(state) != tokenLen {
		return model.LoginFlow{}, errFlowInvalid()
	}
	row, err := s.q(ctx).ConsumeLoginFlow(ctx, Hash(state))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.LoginFlow{}, errFlowInvalid()
	}
	if err != nil {
		return model.LoginFlow{}, fmt.Errorf("identity：取出登入流程：%w", err)
	}
	now := s.d.Now()
	switch {
	case !now.Before(row.ExpiresAt):
		s.d.Logger.InfoContext(ctx, "identity：登入流程已逾時")
	case model.Provider(row.Provider) != want:
		s.d.Logger.WarnContext(ctx, "identity：登入流程的 provider 與 callback 不符", "want", want)
	case subtle.ConstantTimeCompare(Hash(binding), row.BindingHash) != 1:
		s.d.Logger.WarnContext(ctx, "identity：登入流程的登入前 cookie 不符")
	default:
		return model.LoginFlow{
			Provider: model.Provider(row.Provider), CodeVerifier: row.CodeVerifier, Nonce: row.Nonce,
			ReturnPath: row.ReturnPath, LinkAccountID: row.LinkAccountID,
		}, nil
	}
	return model.LoginFlow{}, errFlowInvalid()
}

// IssueAppNonce 發一個 App 登入用的單次 nonce（5 分鐘）。順手清掉逾時的列。
func (s *Service) IssueAppNonce(ctx context.Context) (string, time.Time, error) {
	nonce, err := newSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	now := s.d.Now()
	q := s.q(ctx)
	if _, err := q.DeleteExpiredAppLoginNonces(ctx, now); err != nil {
		return "", time.Time{}, fmt.Errorf("identity：清除逾時的 nonce：%w", err)
	}
	expires := now.Add(AppNonceTTL)
	if err := q.InsertAppLoginNonce(ctx, store.InsertAppLoginNonceParams{NonceHash: Hash(nonce), Now: now, ExpiresAt: expires}); err != nil {
		return "", time.Time{}, fmt.Errorf("identity：寫入 nonce：%w", err)
	}
	return nonce, expires, nil
}

// ConsumeAppNonce 取出並刪除 nonce；不存在（含重放）或逾時回 KindUnauthenticated。同 ConsumeLoginFlow，失敗時也已作廢。
func (s *Service) ConsumeAppNonce(ctx context.Context, nonce string) error {
	if len(nonce) != tokenLen {
		return errFlowInvalid()
	}
	expires, err := s.q(ctx).ConsumeAppLoginNonce(ctx, Hash(nonce))
	if errors.Is(err, pgx.ErrNoRows) {
		return errFlowInvalid()
	}
	if err != nil {
		return fmt.Errorf("identity：取出 nonce：%w", err)
	}
	if !s.d.Now().Before(expires) {
		return errFlowInvalid()
	}
	return nil
}
