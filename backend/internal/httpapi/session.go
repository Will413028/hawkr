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

func withSession(auth Authenticator, log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, source := rpc.Credential(r)
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
		if a.Session != nil {
			ctx = rpc.WithSession(ctx, rpc.Session{ID: a.Session.ID, Client: a.Session.Client, Source: source})
		}
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
	accessAllowed suspendedAccessKind = iota + 1
	accessGuest
	accessDenied
)

func suspendedAccess(r *http.Request) suspendedAccessKind {
	audience, connectPath := rpc.AudienceOf(r.URL.Path)
	if !connectPath {
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && !asksForUpgrade(r) {
			return accessAllowed
		}
		return accessDenied
	}
	switch audience {
	case rpc.AudiencePublic, rpc.AudienceDelegated:
		return accessGuest
	case rpc.AudienceApp:
		if effectOf(r.URL.Path) == EffectWrite {
			return accessDenied
		}
	case rpc.AudienceAdmin:
	}
	return accessAllowed
}

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

func writeUnavailable(w http.ResponseWriter, r *http.Request) {
	if _, ok := rpc.AudienceOf(r.URL.Path); ok {
		_ = connect.NewErrorWriter().Write(w, r, rpc.NegotiatedCommonError(w, r, connect.CodeUnavailable))
		return
	}
	text := rpc.NegotiatedCommonText(w, r, connect.CodeUnavailable).Text
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	body, _ := json.Marshal(map[string]any{"code": http.StatusServiceUnavailable, "message": text})
	_, _ = w.Write(body)
}

func writePermissionDenied(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apperr.Error
	hasText := errors.As(err, &ae) && ae.Message != ""
	ce := rpc.Negotiated(w, r, func(ctx context.Context) *connect.Error {
		if hasText {
			return rpc.ConnectError(ctx, ae)
		}
		return rpc.CommonError(ctx, connect.CodePermissionDenied)
	})
	if _, ok := rpc.AudienceOf(r.URL.Path); ok {
		_ = connect.NewErrorWriter().Write(w, r, ce)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	envelope := map[string]any{"code": http.StatusForbidden, "message": ce.Message()}
	if ae != nil && ae.Reason != (apperr.Reason{}) {
		envelope["reason"] = ae.Reason.String()
	}
	body, _ := json.Marshal(envelope)
	_, _ = w.Write(body)
}
