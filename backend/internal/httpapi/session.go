package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Authenticator 驗證 session token；identity.Module 滿足它。
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (identity.Authenticated, error)
}

// withSession 驗證請求帶的 session token（ADR 2026-10-30-identity-self-hosted-federated-login D1），把呼叫者放進 ctx（rpc.WithPrincipal）。
//
//   - 沒有帶 token：以訪客繼續；需要登入的 service 由 interceptor 回 unauthenticated。
//   - token 無效、過期，或帳號已刪除：以訪客繼續。
//   - 後台的使用者不是允許的帳號：回 permission_denied，不以訪客繼續（否則後台只看到 unauthenticated，一直回到登入）。
//   - 帳號停權：照樣登入，能做什麼依動作的性質（customerEffects，A7-D18），這裡是唯一的執行點。公開與受邀供應商的 service 以訪客執行
//     （ctx 沒有 principal）；其他顧客 service 的讀取與自助放行，ctx 另帶停權時間（rpc.SuspensionFrom），寫入回 permission_denied 與
//     ACCOUNT_SUSPENDED；不是 Connect 的路徑（轉給 gateway 的 REST）只放行不要求換協定的 GET、HEAD（WebSocket 升級有副作用，視為寫入），其餘回 403。
//   - 問不到公鑰或資料庫：回 503，不以訪客繼續：「問不到」不等於「沒有登入」，否則 client 會把使用者登出。
//
// 業務身分（buyer_id、store_id）不在這裡解析：只有需要它的地方——buyer 的 Connect handler 與轉給 gateway 的請求——
// 才查，其他請求（例如賣家的 Connect 方法、GetSession）不受 buyer 查詢失敗影響。
func withSession(auth Authenticator, log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := rpc.Credential(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		a, err := auth.Authenticate(r.Context(), token)
		if err != nil {
			switch apperr.KindOf(err) {
			case apperr.KindUnauthenticated:
				next.ServeHTTP(w, r)
			case apperr.KindPermissionDenied:
				writePermissionDenied(w, r, err)
			case apperr.KindInternal, apperr.KindInvalidInput, apperr.KindNotFound,
				apperr.KindVersionConflict, apperr.KindAlreadyExists, apperr.KindFailedPrecondition,
				apperr.KindUnavailable, apperr.KindResourceExhausted:
				log.ErrorContext(r.Context(), "httpapi：驗證 session token 失敗", "error", err)
				writeUnavailable(w, r)
			default:
				log.ErrorContext(r.Context(), "httpapi：驗證 session token 失敗", "error", err)
				writeUnavailable(w, r)
			}
			return
		}
		ctx := rpc.WithPrincipal(r.Context(), a.Principal)
		if a.SuspendedAt != nil {
			switch suspendedAccess(r) {
			case accessGuest:
				next.ServeHTTP(w, r)
				return
			case accessDenied:
				writePermissionDenied(w, r, apperr.PermissionDenied("帳號已停權，無法執行這個操作").WithReason(apperr.ReasonAccountSuspended))
				return
			case accessAllowed:
				ctx = rpc.WithSuspension(ctx, *a.SuspendedAt)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type suspendedAccessKind int

const (
	// accessAllowed 以登入身分繼續，帶停權時間。
	accessAllowed suspendedAccessKind = iota + 1
	// accessGuest 以訪客繼續。
	accessGuest
	// accessDenied 回 ACCOUNT_SUSPENDED。
	accessDenied
)

// suspendedAccess 決定停權的帳號這個請求怎麼處理（A7-D17、A7-D18）。
func suspendedAccess(r *http.Request) suspendedAccessKind {
	audience, connectPath := rpc.AudienceOf(r.URL.Path)
	if !connectPath {
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && !asksForUpgrade(r) {
			return accessAllowed
		}
		return accessDenied
	}
	switch audience {
	case rpc.AudiencePublic, rpc.AudienceInvitee:
		return accessGuest
	case rpc.AudienceApp:
		if effectOf(r.URL.Path) == EffectWrite {
			return accessDenied
		}
	case rpc.AudienceAdmin:
	}
	return accessAllowed
}

// asksForUpgrade 回報請求是否要求換協定（WebSocket）：升級後的連線可以持續送訊息，有副作用，視為寫入（A7-D18）。
func asksForUpgrade(r *http.Request) bool {
	if r.Header.Get("Upgrade") != "" {
		return true
	}
	for _, v := range r.Header.Values("Connection") {
		for _, token := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(token), "upgrade") {
				return true
			}
		}
	}
	return false
}

// writeUnavailable 回 503：Connect 的請求寫 Connect 的錯誤（unavailable），其他路徑（轉給 gateway 的 REST）寫 gateway
// 的 envelope，兩種 client 都以既有的方式處理（web 把 503 當成「暫時問不到」，保留登入狀態）。
func writeUnavailable(w http.ResponseWriter, r *http.Request) {
	if _, ok := rpc.AudienceOf(r.URL.Path); ok {
		_ = connect.NewErrorWriter().Write(w, r, rpc.CommonError(r.Context(), connect.CodeUnavailable))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	body, _ := json.Marshal(map[string]any{"code": http.StatusServiceUnavailable, "message": rpc.CommonText(r.Context(), connect.CodeUnavailable).Text})
	_, _ = w.Write(body)
}

// writePermissionDenied 回 403，帶模組給使用者的文字，沒有時用共用文案目錄的 PERMISSION_DENIED（語系取自 ctx）；形狀同 writeUnavailable。
// Connect 的請求另帶錯誤的 reason（apperr.Reason）；gateway 的 envelope 沒有 reason 欄位。
func writePermissionDenied(w http.ResponseWriter, r *http.Request, err error) {
	ce := rpc.CommonError(r.Context(), connect.CodePermissionDenied)
	var ae *apperr.Error
	if errors.As(err, &ae) && ae.Message != "" {
		ce = rpc.ConnectError(r.Context(), ae)
	}
	if _, ok := rpc.AudienceOf(r.URL.Path); ok {
		_ = connect.NewErrorWriter().Write(w, r, ce)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	body, _ := json.Marshal(map[string]any{"code": http.StatusForbidden, "message": ce.Message()})
	_, _ = w.Write(body)
}
