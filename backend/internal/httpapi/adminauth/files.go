package adminauth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

// filesPrefix 是後台檔案路由的路徑前綴（API 設計規範（Connect）§十三）。
const filesPrefix = "/api/files/admin/"

// IsAdminFileRoute 回傳 ServeMux 樣式（可帶 method，例如 "GET /api/files/admin/…"）是不是後台的檔案路由。
func IsAdminFileRoute(pattern string) bool {
	path := pattern
	if _, rest, ok := strings.Cut(pattern, " "); ok {
		path = strings.TrimSpace(rest)
	}
	return strings.HasPrefix(path, filesPrefix)
}

// GuardFile 為一條檔案路由套上授權宣告：後台路由（/api/files/admin/…）必須宣告目錄裡的權限，handler 在模組自己的授權
// 之前先檢查呼叫者是管理員、而且角色允許這個權限；其他路由不得宣告權限。宣告不合法回錯誤，呼叫方因此不能啟動。
func GuardFile(pattern string, perm adminperm.Permission, next http.Handler, log *slog.Logger) (http.Handler, error) {
	if !IsAdminFileRoute(pattern) {
		if perm != "" {
			return nil, fmt.Errorf("httpapi/adminauth：%s 不是後台的檔案路由，不能宣告權限 %s", pattern, perm)
		}
		return next, nil
	}
	if perm == "" || !adminperm.InCatalog(perm) {
		return nil, fmt.Errorf("httpapi/adminauth：%s 沒有宣告目錄裡的權限（%q）", pattern, perm)
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := rpc.PrincipalFrom(r.Context())
		switch {
		case !ok:
			deny(w, http.StatusUnauthorized, "unauthenticated")
		case p.Kind != identity.PrincipalAdmin:
			deny(w, http.StatusForbidden, "forbidden")
		case !p.AdminAllows(perm):
			log.WarnContext(r.Context(), "adminauth：拒絕", "admin_id", strconv.FormatInt(p.ID, 10), "route", pattern, "need", string(perm))
			deny(w, http.StatusForbidden, "forbidden")
		default:
			next.ServeHTTP(w, r)
		}
	}), nil
}

// deny 回應只有 HTTP 狀態，與檔案 handler 自己的錯誤一樣不可快取、不帶細節。
func deny(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.Error(w, text, status)
}

// GuardLegacy 為轉給舊 gateway 的 handler 加上轉發閘（ADR 2026-10-04-admin-roles）：呼叫者是管理員、而角色不允許
// adminperm.LegacyGatewayForward 時回 403，不轉發。沒有呼叫者與非管理員的呼叫者原樣放行（代行工作區只帶
// X-HK-Operation-Session，到這裡沒有管理員 principal）。403 的 body 是舊 gateway 路徑的錯誤格式。
func GuardLegacy(next http.Handler, log *slog.Logger) http.Handler {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := rpc.PrincipalFrom(r.Context()); ok && p.Kind == identity.PrincipalAdmin && !p.AdminAllows(adminperm.LegacyGatewayForward) {
			log.WarnContext(r.Context(), "adminauth：拒絕", "admin_id", strconv.FormatInt(p.ID, 10), "path", r.URL.Path, "need", string(adminperm.LegacyGatewayForward))
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			body, _ := json.Marshal(struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{http.StatusForbidden, rpc.CommonText(r.Context(), connect.CodePermissionDenied).Text})
			_, _ = w.Write(body)
			return
		}
		next.ServeHTTP(w, r)
	})
}
