// Package rpc 是單體各模組的 Connect handler 共用的傳輸層元件：身分與授權、錯誤轉換、逾時、log，
// 以及分頁 token 與 field mask。規則見 docs/開發規範/API 設計規範（Connect）.md。
package rpc

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel/trace"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// Audience 是 service 的使用者類別，授權跟著它走（API 設計規範（Connect）§二）。
type Audience int

const (
	// AudiencePublic 是 <模組>PublicService：任何人都能呼叫，不讀身分。
	AudiencePublic Audience = iota + 1
	// AudienceApp 是 <模組>AppService：登入的買家、賣家。
	AudienceApp
	// AudienceAdmin 是 hawkr.admin.* 的 <模組>AdminService：管理員。
	AudienceAdmin
	// AudienceInvitee 是 <模組>InviteeService：兌換了邀請的供應商（沒有帳號），憑證是 service 自己的 cookie，不讀身分。
	AudienceInvitee
)

func (a Audience) String() string {
	switch a {
	case AudiencePublic:
		return "public"
	case AudienceApp:
		return "app"
	case AudienceAdmin:
		return "admin"
	case AudienceInvitee:
		return "invitee"
	}
	return "unknown"
}

// AudienceOf 從 procedure（/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners）或
// handler 的路徑（/hawkr.content.v1alpha1.ContentPublicService/）判斷使用者類別。
// 不符合命名規則的回 false，呼叫方不得掛上這種 service。
func AudienceOf(procedure string) (Audience, bool) {
	svc, _, _ := strings.Cut(strings.TrimPrefix(procedure, "/"), "/")
	admin := strings.HasPrefix(svc, "hawkr.admin.")
	switch {
	case strings.HasSuffix(svc, "AdminService"):
		return AudienceAdmin, admin
	case admin:
		return 0, false
	case strings.HasSuffix(svc, "AppService"):
		return AudienceApp, true
	case strings.HasSuffix(svc, "InviteeService"):
		return AudienceInvitee, true
	case strings.HasSuffix(svc, "PublicService"):
		return AudiencePublic, true
	}
	return 0, false
}

type principalKey struct{}

// WithPrincipal 把認證過的呼叫者放進 ctx；只有入口的認證中介層呼叫：它以 identity 驗證 Clerk 的 session token（cookie 或
// Bearer）後放進來（ADR 2026-09-27-identity-clerk）。
func WithPrincipal(ctx context.Context, p identity.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom 回傳 ctx 裡的呼叫者。App 與 Admin 的 service 一定有；Public 的請求有帶有效的 session 時也有。
func PrincipalFrom(ctx context.Context) (identity.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(identity.Principal)
	return p, ok
}

// authorize 依 service 的使用者類別檢查 ctx 裡的呼叫者。
func authorize(ctx context.Context, procedure string) error {
	audience, ok := AudienceOf(procedure)
	if !ok {
		return Error(connect.CodePermissionDenied, "%s", "這個方法不對外開放")
	}
	if audience == AudiencePublic || audience == AudienceInvitee {
		return nil
	}
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return userError(connect.CodeUnauthenticated, CommonText(ctx, connect.CodeUnauthenticated))
	}
	switch audience {
	case AudienceAdmin:
		if p.Kind != identity.PrincipalAdmin {
			return userError(connect.CodePermissionDenied, CommonText(ctx, connect.CodePermissionDenied))
		}
	case AudienceApp:
		if p.Kind != identity.PrincipalAccount {
			return userError(connect.CodePermissionDenied, CommonText(ctx, connect.CodePermissionDenied))
		}
	case AudiencePublic, AudienceInvitee:
	}
	return nil
}

// TraceID 回傳這個請求的 trace id（httpapi 為每個請求開的 span）；沒有時是空字串。
func TraceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}
