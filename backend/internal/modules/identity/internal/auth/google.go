package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// MaxGoogleTokenAge 是 vendor portal step-up 接受的 Google ID token 的最大年齡（iat 距現在）。Google Identity Services 不提供
// max_age，iat 是唯一的新鮮度訊號；iat 在未來的容忍度是 oidc 驗證的 5 秒時間差。
const MaxGoogleTokenAge = 10 * time.Minute

// VerifyGoogle 只驗證 Google 的 ID token，不登入也不寫入：vendor portal 的 step-up（單體入口的 VerifySession）。token 的 nonce
// 必須等於 wantNonce（OIDC Core §3.2.2.1，呼叫方由 session 推導），iat 不得早於 MaxGoogleTokenAge 之前；兩者不符與其他無效
// token 同樣回 KindUnauthenticated，呼叫方分不出原因。wantNonce 是空字串是呼叫方的程式錯誤，回沒有分類的錯誤（轉成 internal），
// 不放行也不當成 token 無效。回傳的 Identity 帶 EmailVerified，這裡不強制它：以 email 授權的呼叫方必須自己檢查。沒有設定
// Google 或問不到公鑰時回 KindUnavailable。
func (m *Service) VerifyGoogle(ctx context.Context, idToken, wantNonce string) (model.Identity, error) {
	if wantNonce == "" {
		return model.Identity{}, errors.New("identity：VerifyGoogle 需要 nonce")
	}
	v := m.d.Google
	if v == nil {
		return model.Identity{}, apperr.Unavailable("Google 驗證目前無法使用")
	}
	if idToken == "" {
		return model.Identity{}, apperr.Unauthenticated("Google 驗證失敗")
	}
	id, err := v.Verify(ctx, idToken)
	switch {
	case errors.Is(err, model.ErrInvalidToken):
		return model.Identity{}, apperr.WithCause(apperr.Unauthenticated("Google 驗證失敗"), err)
	case err != nil:
		return model.Identity{}, apperr.WithCause(apperr.Unavailable("Google 驗證目前無法使用"), err)
	}
	if subtle.ConstantTimeCompare([]byte(id.Nonce), []byte(wantNonce)) != 1 {
		return model.Identity{}, apperr.Unauthenticated("Google 驗證失敗")
	}
	if id.IssuedAt.IsZero() || id.IssuedAt.Before(m.d.Now().Add(-MaxGoogleTokenAge)) {
		return model.Identity{}, apperr.Unauthenticated("Google 驗證失敗")
	}
	return id, nil
}
