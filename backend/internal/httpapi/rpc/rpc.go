// Package rpc 是單體各模組的 Connect handler 共用的傳輸層元件：身分與授權、錯誤轉換、逾時、log，
// 以及分頁 token 與 field mask。規則見 `docs/共通規範/API 設計規範（Connect）.md`。
package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel/trace"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Audience 是 service 的使用者類別，授權跟著它走（API 設計規範（Connect）§二）。
type Audience int

const (
	// AudiencePublic 是 <模組>PublicService：任何人都能呼叫，不要求登入；請求帶有效 session 時 handler 可以讀呼叫者（例如依觀看者過濾、viewer_can_*），
	// 停權帳號與無效 token 以訪客執行。依觀看者而定的方法（httpapi 的 publicViewerDependent）在請求帶憑證時，GET 回應是 private, no-store（API 設計規範（Connect）§九）。
	AudiencePublic Audience = iota + 1
	// AudienceApp 是 <模組>AppService：登入的買家、賣家。
	AudienceApp
	// AudienceAdmin 是 hawkr.admin.* 的 <模組>AdminService：管理員。
	AudienceAdmin
	// AudienceDelegated 是 <模組>DelegatedService：代行營運中的平台管理員，憑證是請求標頭 X-HK-Operation-Session 的代行 session，由 service 自己的 interceptor
	// 解析並逐方法檢查能力，不讀顧客或管理員的身分（也不經 adminauth）。
	AudienceDelegated
)

func (a Audience) String() string {
	switch a {
	case AudiencePublic:
		return "public"
	case AudienceApp:
		return "app"
	case AudienceAdmin:
		return "admin"
	case AudienceDelegated:
		return "delegated"
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
	case strings.HasSuffix(svc, "DelegatedService"):
		return AudienceDelegated, true
	case strings.HasSuffix(svc, "PublicService"):
		return AudiencePublic, true
	}
	return 0, false
}

type principalKey struct{}

// WithPrincipal 把認證過的呼叫者放進 ctx；只有入口的認證中介層呼叫：它以 identity 驗證 session token（cookie 或
// Bearer）後放進來（ADR 2026-10-30-identity-self-hosted-federated-login）。
func WithPrincipal(ctx context.Context, p identity.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom 回傳 ctx 裡的呼叫者。App 與 Admin 的 service 一定有；Public 的請求有帶有效的 session、而且帳號沒有停權時也有。
func PrincipalFrom(ctx context.Context) (identity.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(identity.Principal)
	return p, ok
}

type suspensionKey struct{}

// WithSuspension 把呼叫者的帳號被停權的時間放進 ctx；只有入口的認證中介層呼叫，且只在帳號停權、而且請求以登入身分繼續時放。
func WithSuspension(ctx context.Context, at time.Time) context.Context {
	return context.WithValue(ctx, suspensionKey{}, at)
}

// SuspensionFrom 回傳呼叫者的帳號被停權的時間；沒有停權（或請求以訪客執行）時 ok 是 false。
func SuspensionFrom(ctx context.Context) (at time.Time, ok bool) {
	at, ok = ctx.Value(suspensionKey{}).(time.Time)
	return at, ok
}

func authorize(ctx context.Context, procedure string) error {
	audience, ok := AudienceOf(procedure)
	if !ok {
		return Error(connect.CodePermissionDenied, "%s", "這個方法不對外開放")
	}
	if audience == AudiencePublic || audience == AudienceDelegated {
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
	case AudiencePublic, AudienceDelegated:
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

// BuyerInvariant 把「存活的帳號查不到現任買家」轉成 internal：帳號活著 ⇔ 買家活著（ADR
// 2026-09-30-account-created-provisions-buyer），認證過的帳號都活著，所以這是不變量被破壞（interceptor 記 error log），
// 不當成「沒有買家」。其他錯誤原樣回傳，err 是 nil 時回 nil。
func BuyerInvariant(accountID int64, err error) error {
	if apperr.KindOf(err) == apperr.KindNotFound {
		return fmt.Errorf("存活的帳號 %d 沒有現任買家（不變量被破壞）", accountID)
	}
	return err
}

// BuyerLookup 把登入的帳號對到買家身分（*buyer.Module）。
type BuyerLookup interface {
	CurrentBuyerID(ctx context.Context, accountID int64) (int64, error)
}

// ErrNoPrincipal 是請求通過了授權卻沒有呼叫者：授權鏈壞了，沒有分類（internal），不是使用者的錯。
var ErrNoPrincipal = errors.New("httpapi/rpc：ctx 裡沒有呼叫者")

// RequireAccount 回傳呼叫者的顧客帳號身分：只有顧客帳號能呼叫（沒有登入由 interceptor 擋，ctx 裡沒有呼叫者是授權鏈壞了，回沒有分類的 internal；管理員等非帳號身分回 permission_denied）。
func RequireAccount(ctx context.Context) (identity.Principal, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return identity.Principal{}, ErrNoPrincipal
	}
	if p.Kind != identity.PrincipalAccount {
		return identity.Principal{}, Error(connect.CodePermissionDenied, "%s", "需要顧客帳號")
	}
	return p, nil
}

// RequireAdmin 回傳呼叫的管理員 ID：只有管理員能呼叫（interceptor 已依 service 名稱擋下非管理員；ctx 裡沒有呼叫者是授權鏈壞了，回沒有分類的 internal；非管理員身分回 permission_denied）。
func RequireAdmin(ctx context.Context) (int64, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return 0, ErrNoPrincipal
	}
	if p.Kind != identity.PrincipalAdmin {
		return 0, Error(connect.CodePermissionDenied, "%s", "需要管理員")
	}
	return p.ID, nil
}

// AccountBuyerID 回傳呼叫者的買家 ID：只有顧客帳號能呼叫（沒有登入由 interceptor 擋，管理員等非帳號身分回 permission_denied）；
// 存活的帳號查不到買家是不變量被破壞（BuyerInvariant）。
func AccountBuyerID(ctx context.Context, buyers BuyerLookup) (int64, error) {
	p, err := RequireAccount(ctx)
	if err != nil {
		return 0, err
	}
	id, err := buyers.CurrentBuyerID(ctx, p.ID)
	if err != nil {
		return 0, BuyerInvariant(p.ID, err)
	}
	return id, nil
}
