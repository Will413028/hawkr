package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/adminauth"
	httpmedia "github.com/Will413028/hawkr/backend/internal/httpapi/media"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

// 後台的段上傳路由（B1-D16）：啟動的檔案路由表宣告了目錄裡的權限，而且擋在 handler 之前：買家與賣家的帳號、零角色的管理員都被拒絕，handler 一次也沒被呼叫。
func TestAdminChunkRouteDeclaresAPermissionAndGuardsTheHandler(t *testing.T) {
	var route *fileRoute
	routes := fileRoutes(Deps{Media: &media.Module{}})
	for i := range routes {
		if routes[i].pattern == httpmedia.AdminChunkRoute {
			route = &routes[i]
		}
	}
	if route == nil {
		t.Fatalf("檔案路由表沒有 %s", httpmedia.AdminChunkRoute)
	}
	if route.permission != adminperm.DelegationSessionsCreate {
		t.Fatalf("後台段路由的權限 %q，預期 %q", route.permission, adminperm.DelegationSessionsCreate)
	}
	for _, r := range routes {
		if _, err := adminauth.GuardFile(r.pattern, r.permission, r.handler, nil); err != nil {
			t.Fatalf("路由 %s 的宣告不合法：%v", r.pattern, err)
		}
	}

	served := 0
	h, err := adminauth.GuardFile(route.pattern, route.permission, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served++
		w.WriteHeader(http.StatusNoContent)
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := identity.Principal{Kind: identity.PrincipalAdmin, ID: 3, AdminRoles: []adminperm.Role{adminperm.RoleOwner}}
	for name, tc := range map[string]struct {
		p      *identity.Principal
		status int
		served int
	}{
		"沒有呼叫者":   {nil, http.StatusUnauthorized, 0},
		"買家或賣家帳號": {&identity.Principal{Kind: identity.PrincipalAccount, ID: 7}, http.StatusForbidden, 0},
		"零角色的管理員": {&identity.Principal{Kind: identity.PrincipalAdmin, ID: 4}, http.StatusForbidden, 0},
		"OWNER":   {&owner, http.StatusNoContent, 1},
	} {
		served = 0
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/files/admin/secure_uploads/tok/chunks/0", strings.NewReader("a"))
		if tc.p != nil {
			req = req.WithContext(rpc.WithPrincipal(req.Context(), *tc.p))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		_, _ = io.Copy(io.Discard, rec.Body)
		if rec.Code != tc.status || served != tc.served {
			t.Errorf("%s：status=%d handler=%d，預期 %d、%d", name, rec.Code, served, tc.status, tc.served)
		}
	}
}

// 後台的暫存預覽路由（B1-D16）宣告與段路由相同的權限。
func TestAdminPreviewRouteDeclaresTheSamePermission(t *testing.T) {
	for _, r := range fileRoutes(Deps{Media: &media.Module{}}) {
		if r.pattern == httpmedia.AdminPreviewRoute {
			if r.permission != adminperm.DelegationSessionsCreate {
				t.Fatalf("後台預覽路由的權限 %q，預期 %q", r.permission, adminperm.DelegationSessionsCreate)
			}
			return
		}
	}
	t.Fatalf("檔案路由表沒有 %s", httpmedia.AdminPreviewRoute)
}
