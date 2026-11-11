//go:build integration

package identity_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/adminauth"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

// get 以 token 打 path，回傳狀態碼。
func (f *fixture) get(t *testing.T, path, tok string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, f.url+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// post 以 token 呼叫一支 Connect 方法（JSON 空訊息），回傳狀態碼與 Connect 的錯誤碼（成功時空字串）。
func (f *fixture) post(t *testing.T, procedure, tok string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, f.url+procedure, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body.Code
}

const kycDocument = "/api/files/admin/seller/kyc_applications/not-a-uuid/documents/id_card_front/0"

// 零角色的管理員（Access 白名單登入一次的人）：每一支 admin 方法都被拒絕，只有 GetSession 與 SignOut 成功；檔案與舊 gateway 的
// 請求被擋在 handler 與轉發之前。沒有管理員 principal 的請求（沒有 token、顧客）照舊轉發。
func TestAdminWithoutRolesIsDeniedEverywhereExceptTheExemptions(t *testing.T) {
	t.Parallel()
	var forwarded atomic.Int64
	f := setup(t, options{legacy: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		forwarded.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})})
	tok := f.adminToken(t, "dev_zero")

	exempt := map[string]bool{
		"/hawkr.admin.identity.v1alpha1.IdentityAdminService/GetSession": true,
		"/hawkr.admin.identity.v1alpha1.IdentityAdminService/SignOut":    true,
	}
	procedures := adminauth.Procedures()
	if len(procedures) != 80 {
		t.Fatalf("admin 方法 %d 支，預期 80", len(procedures))
	}
	for _, procedure := range procedures {
		status, code := f.post(t, procedure, tok)
		if exempt[procedure] {
			if status != http.StatusOK {
				t.Errorf("豁免的 %s = %d %s", procedure, status, code)
			}
			continue
		}
		if status != http.StatusForbidden || code != "permission_denied" {
			t.Errorf("零角色呼叫 %s = %d %q，預期 403 permission_denied", procedure, status, code)
		}
	}

	if got := f.get(t, kycDocument, tok); got != http.StatusForbidden {
		t.Errorf("零角色讀 /api/files/admin = %d", got)
	}
	if got := f.get(t, "/api/v1/admin/stores", tok); got != http.StatusForbidden {
		t.Errorf("零角色打 /api/v1/admin = %d", got)
	}
	if n := forwarded.Load(); n != 0 {
		t.Fatalf("零角色的管理員被轉發了 %d 次", n)
	}

	// 沒有管理員 principal：照舊轉發。
	if got := f.get(t, "/api/v1/public/stores", ""); got != http.StatusNoContent {
		t.Errorf("沒有 token = %d", got)
	}
	if got := f.get(t, "/api/v1/app/buyers/me", f.customerToken(t, "dev_customer")); got != http.StatusNoContent {
		t.Errorf("顧客 = %d", got)
	}
	if n := forwarded.Load(); n != 2 {
		t.Fatalf("轉發 %d 次，預期 2", n)
	}
}

// OWNER 通過每一道檢查；撤銷之後的下一個請求就被拒（不快取）。
func TestOwnerPassesEveryCheckAndRevocationTakesEffectOnTheNextRequest(t *testing.T) {
	t.Parallel()
	var forwarded atomic.Int64
	f := setup(t, options{legacy: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		forwarded.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})})
	ctx := context.Background()
	tok := f.adminToken(t, "dev_owner")
	a, err := f.m.Authenticate(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	change := identity.AdminRoleChangeInput{AdminID: a.Principal.ID, Role: adminperm.RoleOwner, Reason: "測試", Source: identity.AdminRoleSourceCLI}
	if _, err := f.m.GrantAdminRole(ctx, change); err != nil {
		t.Fatal(err)
	}
	admin := f.admin(headers("Authorization", "Bearer "+tok))
	if _, err := admin.ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{})); err != nil {
		t.Fatalf("OWNER 的 ListAccounts = %v", err)
	}
	// 檔案路由過了權限的檢查（不合法的申請 ID 由 handler 回 404）。
	if got := f.get(t, kycDocument, tok); got != http.StatusNotFound {
		t.Errorf("OWNER 讀 /api/files/admin = %d，預期通過檢查後由 handler 回 404", got)
	}
	if got := f.get(t, "/api/v1/admin/stores", tok); got != http.StatusNoContent || forwarded.Load() != 1 {
		t.Errorf("OWNER 打 /api/v1/admin = %d，轉發 %d 次", got, forwarded.Load())
	}

	if _, err := f.m.RevokeAdminRole(ctx, change); err != nil {
		t.Fatal(err)
	}
	_, err = admin.ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{}))
	wantCode(t, err, connect.CodePermissionDenied)
	if got := f.get(t, kycDocument, tok); got != http.StatusForbidden {
		t.Errorf("撤銷之後讀 /api/files/admin = %d", got)
	}
	if got := f.get(t, "/api/v1/admin/stores", tok); got != http.StatusForbidden || forwarded.Load() != 1 {
		t.Errorf("撤銷之後打 /api/v1/admin = %d，轉發 %d 次", got, forwarded.Load())
	}
	if _, err := admin.GetSession(ctx, connect.NewRequest(&adminpb.GetSessionRequest{})); err != nil {
		t.Errorf("撤銷之後的 GetSession = %v", err)
	}
}

// 顧客的 token 打 admin 的方法與檔案路由仍是 audience 的拒絕。
func TestCustomerTokenCannotReachAdminRoutes(t *testing.T) {
	t.Parallel()
	f := setup(t, options{})
	tok := f.customerToken(t, "dev_customer")
	status, code := f.post(t, "/hawkr.admin.identity.v1alpha1.IdentityAdminService/ListAccounts", tok)
	if status != http.StatusForbidden || code != "permission_denied" {
		t.Fatalf("顧客呼叫 admin 方法 = %d %q", status, code)
	}
	if got := f.get(t, kycDocument, tok); got != http.StatusForbidden {
		t.Fatalf("顧客讀 /api/files/admin = %d", got)
	}
}
