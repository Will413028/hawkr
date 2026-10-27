package adminauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

const adminFile = "GET /api/files/admin/test/{id}"

func serve(ctx context.Context, h http.Handler, path string, p *identity.Principal) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if p != nil {
		r = r.WithContext(rpc.WithPrincipal(r.Context(), *p))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func counter(n *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { *n++; w.WriteHeader(http.StatusOK) })
}

func TestGuardFileValidatesTheDeclaration(t *testing.T) {
	var n int
	for name, tc := range map[string]struct {
		pattern string
		perm    adminperm.Permission
	}{
		"後台路由沒有宣告":      {adminFile, ""},
		"後台路由宣告不在目錄的權限": {adminFile, "seller.no_such.get"},
		"非後台路由宣告了權限":    {"GET /api/files/app/x/{id}", adminperm.SellerKYCDocumentsGet},
	} {
		if _, err := GuardFile(tc.pattern, tc.perm, counter(&n), nil); err == nil {
			t.Errorf("%s：應該失敗", name)
		}
	}
	for _, tc := range []struct {
		pattern string
		perm    adminperm.Permission
	}{
		{adminFile, adminperm.SellerKYCDocumentsGet},
		{"/api/files/admin/test/{id}", adminperm.SellerKYCDocumentsGet},
		{"GET /api/files/app/x/{id}", ""},
	} {
		if _, err := GuardFile(tc.pattern, tc.perm, counter(&n), nil); err != nil {
			t.Errorf("%s：%v", tc.pattern, err)
		}
	}
}

func TestIsAdminFileRoute(t *testing.T) {
	for pattern, want := range map[string]bool{
		"GET /api/files/admin/a/{id}": true,
		"/api/files/admin/a":          true,
		"GET /api/files/app/a":        false,
		"GET /api/files/administer/a": false,
		"GET /api/v1/admin/a":         false,
	} {
		if got := IsAdminFileRoute(pattern); got != want {
			t.Errorf("%s = %v，預期 %v", pattern, got, want)
		}
	}
}

func TestGuardFileChecksTheRoleBeforeTheHandler(t *testing.T) {
	var n int
	h, err := GuardFile(adminFile, adminperm.SellerKYCDocumentsGet, counter(&n), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	owner := principal(identity.PrincipalAdmin, 3, adminperm.RoleOwner)
	zero := principal(identity.PrincipalAdmin, 4)
	account := principal(identity.PrincipalAccount, 7)
	for name, tc := range map[string]struct {
		p      *identity.Principal
		status int
		served int
	}{
		"沒有呼叫者": {nil, http.StatusUnauthorized, 0},
		"帳號":    {&account, http.StatusForbidden, 0},
		"零角色":   {&zero, http.StatusForbidden, 0},
		"OWNER": {&owner, http.StatusOK, 1},
	} {
		n = 0
		rec := serve(ctx, h, "/api/files/admin/test/1", tc.p)
		if rec.Code != tc.status || n != tc.served || rec.Header().Get("Cache-Control") != "private, no-store" && tc.served == 0 {
			t.Errorf("%s：status=%d handler=%d cache=%q", name, rec.Code, n, rec.Header().Get("Cache-Control"))
		}
	}
}

func TestGuardLegacyOnlyBlocksAdminsWithoutTheForwardPermission(t *testing.T) {
	var n int
	h := GuardLegacy(counter(&n), nil)
	ctx := t.Context()
	owner := principal(identity.PrincipalAdmin, 3, adminperm.RoleOwner)
	zero := principal(identity.PrincipalAdmin, 4)
	account := principal(identity.PrincipalAccount, 7)

	for name, tc := range map[string]struct {
		p      *identity.Principal
		status int
		served int
	}{
		"零角色的管理員": {&zero, http.StatusForbidden, 0},
		"OWNER":   {&owner, http.StatusOK, 1},
		"帳號":      {&account, http.StatusOK, 1},
		"沒有呼叫者":   {nil, http.StatusOK, 1},
	} {
		n = 0
		rec := serve(ctx, h, "/api/v1/admin/stores", tc.p)
		if rec.Code != tc.status || n != tc.served {
			t.Errorf("%s：status=%d forwarded=%d", name, rec.Code, n)
		}
		if tc.status == http.StatusForbidden {
			if body := rec.Body.String(); body != `{"code":403,"message":"無權操作"}` || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Errorf("%s：403 的格式 = %q %q", name, rec.Header().Get("Content-Type"), body)
			}
		}
	}
}
