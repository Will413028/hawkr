package adminauth

import (
	"bytes"
	"context"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func principal(kind identity.PrincipalKind, id int64, roles ...adminperm.Role) identity.Principal {
	return identity.Principal{Kind: kind, ID: id, AdminRoles: roles}
}

// declaredServices 是宣告表涵蓋的 service 的 descriptor（由 protoregistry 查，生成碼的 init 已註冊）。實際掛載的對照在
// httpapi 的 connectServices；這裡只驗證 Check 本身的雙向比對。
func declaredServices(t *testing.T) []protoreflect.ServiceDescriptor {
	t.Helper()
	seen := map[string]bool{}
	var out []protoreflect.ServiceDescriptor
	for procedure := range declarations {
		name, _, _ := strings.Cut(strings.TrimPrefix(procedure, "/"), "/")
		if seen[name] {
			continue
		}
		seen[name] = true
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		out = append(out, d.(protoreflect.ServiceDescriptor))
	}
	return out
}

func methodCount(t *testing.T) int {
	n := 0
	for _, svc := range declaredServices(t) {
		n += svc.Methods().Len()
	}
	return n
}

// 宣告表與每個 admin service 的方法一一對應；少一個宣告、多一個指向不存在的方法的宣告、宣告了不在目錄的權限，都不能啟動。
func TestDeclarationsMatchTheServiceDescriptors(t *testing.T) {
	svcs := declaredServices(t)
	if err := Check(svcs); err != nil {
		t.Fatal(err)
	}
	if got, want := len(declarations), methodCount(t); got != want || got != 80 {
		t.Fatalf("declarations=%d methods=%d，預期 80", got, want)
	}

	for procedure := range declarations {
		table := maps.Clone(declarations)
		delete(table, procedure)
		if err := checkDeclarations(table, svcs); err == nil || !strings.Contains(err.Error(), procedure+" 沒有授權宣告") {
			t.Errorf("移除 %s 的宣告：err=%v", procedure, err)
		}
	}

	stale := maps.Clone(declarations)
	stale["/hawkr.admin.identity.v1alpha1.IdentityAdminService/NoSuchMethod"] = requires(adminperm.IdentityAccountsList)
	if err := checkDeclarations(stale, svcs); err == nil || !strings.Contains(err.Error(), "NoSuchMethod 有宣告，但 service 沒有這個方法") {
		t.Errorf("多出不存在的方法：err=%v", err)
	}

	const procedure = "/hawkr.admin.identity.v1alpha1.IdentityAdminService/ListAccounts"
	for name, d := range map[string]declaration{
		"空宣告":        {},
		"權限又豁免":      {permission: adminperm.IdentityAccountsList, exemptReason: "理由"},
		"不在目錄的權限":    requires("identity.no_such.list"),
		"沒有理由的豁免":    exempt(""),
		"權限的值是空字串後綴": requires(" "),
	} {
		table := maps.Clone(declarations)
		table[procedure] = d
		if err := checkDeclarations(table, svcs); err == nil || !strings.Contains(err.Error(), procedure) {
			t.Errorf("%s：err=%v", name, err)
		}
	}
}

// 豁免只有「我是誰」與登出（ADR 2026-10-04-admin-roles〈權限的定義與檢查點〉）。
func TestOnlyGetSessionAndSignOutAreExempt(t *testing.T) {
	var exempted []string
	for procedure, d := range declarations {
		if d.exemptReason != "" {
			exempted = append(exempted, procedure)
		}
	}
	slices.Sort(exempted)
	want := []string{
		"/hawkr.admin.identity.v1alpha1.IdentityAdminService/GetSession",
		"/hawkr.admin.identity.v1alpha1.IdentityAdminService/SignOut",
	}
	if !slices.Equal(exempted, want) {
		t.Fatalf("豁免 = %v，預期 %v", exempted, want)
	}
}

// 每個權限最多被一支方法使用：一支方法一個權限。
func TestEachPermissionBelongsToOneMethod(t *testing.T) {
	seen := map[adminperm.Permission]string{}
	for procedure, d := range declarations {
		if d.permission == "" {
			continue
		}
		if other, dup := seen[d.permission]; dup {
			t.Errorf("%s 與 %s 共用 %s", procedure, other, d.permission)
		}
		seen[d.permission] = procedure
	}
}

func TestInterceptorDecisions(t *testing.T) {
	var logs bytes.Buffer
	i := &interceptor{table: declarations, log: slog.New(slog.NewTextHandler(&logs, nil))}
	owner := principal(identity.PrincipalAdmin, 3, adminperm.RoleOwner)
	zero := principal(identity.PrincipalAdmin, 4)

	for procedure, d := range declarations {
		ctx := rpc.WithPrincipal(context.Background(), zero)
		err := i.enter(ctx, procedure)
		switch {
		case d.exemptReason != "" && err != nil:
			t.Errorf("零角色的管理員呼叫豁免的 %s = %v", procedure, err)
		case d.exemptReason == "" && apperr.KindOf(err) != apperr.KindPermissionDenied:
			t.Errorf("零角色的管理員呼叫 %s = %v，預期 permission denied", procedure, err)
		}
		if err := i.enter(rpc.WithPrincipal(context.Background(), owner), procedure); err != nil {
			t.Errorf("OWNER 呼叫 %s = %v", procedure, err)
		}
	}

	// default deny：沒有宣告的 procedure 連 OWNER 也拒絕。
	const undeclared = "/hawkr.admin.identity.v1alpha1.IdentityAdminService/NoSuchMethod"
	logs.Reset()
	if err := i.enter(rpc.WithPrincipal(context.Background(), owner), undeclared); apperr.KindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("沒有宣告的 procedure = %v", err)
	}
	if !strings.Contains(logs.String(), "admin_id=3") || !strings.Contains(logs.String(), undeclared) {
		t.Errorf("拒絕沒有記下管理員與 procedure：%s", logs.String())
	}

	// 非 admin audience 的 procedure 不歸這支 interceptor 管：即使沒有 principal 也放行。
	for _, procedure := range []string{
		"/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners",
		"/hawkr.seller.v1alpha1.SellerAppService/GetMyStore",
	} {
		if err := i.enter(context.Background(), procedure); err != nil {
			t.Errorf("非 admin 的 %s = %v", procedure, err)
		}
	}

	// 帳號與沒有呼叫者的請求不是管理員。
	for name, ctx := range map[string]context.Context{
		"帳號":    rpc.WithPrincipal(context.Background(), principal(identity.PrincipalAccount, 7)),
		"沒有呼叫者": context.Background(),
	} {
		if err := i.enter(ctx, "/hawkr.admin.identity.v1alpha1.IdentityAdminService/GetSession"); apperr.KindOf(err) != apperr.KindPermissionDenied {
			t.Errorf("%s = %v", name, err)
		}
	}
}

// 帶著管理員角色的非管理員不會被當成管理員：Principal.AdminRoles 只對管理員有意義。
func TestAccountWithAdminRolesIsStillNotAdmin(t *testing.T) {
	p := principal(identity.PrincipalAccount, 7, adminperm.RoleOwner)
	if p.AdminAllows(adminperm.IdentityAccountsList) {
		t.Fatal("帳號不得因 AdminRoles 而被允許")
	}
}

// 實際掛在 Connect handler 上：宣告的方法在 OWNER 時進入 handler；零角色與沒有宣告的 procedure 被擋在 handler 之外。
func TestInterceptorGuardsAConnectHandler(t *testing.T) {
	const declared = "/hawkr.admin.identity.v1alpha1.IdentityAdminService/ListAccounts"
	const undeclared = "/hawkr.admin.identity.v1alpha1.IdentityAdminService/NoSuchMethod"
	guard := NewInterceptor(nil)
	var called int
	var current identity.Principal
	mux := http.NewServeMux()
	for _, procedure := range []string{declared, undeclared} {
		mux.Handle(procedure, connect.NewUnaryHandler(procedure,
			func(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[emptypb.Empty], error) {
				called++
				return connect.NewResponse(&emptypb.Empty{}), nil
			}, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler)), guard)))
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), current)))
	}))
	t.Cleanup(srv.Close)
	call := func(procedure string) error {
		_, err := connect.NewClient[emptypb.Empty, emptypb.Empty](srv.Client(), srv.URL+procedure, connect.WithProtoJSON()).
			CallUnary(t.Context(), connect.NewRequest(&emptypb.Empty{}))
		return err
	}

	current = principal(identity.PrincipalAdmin, 3, adminperm.RoleOwner)
	if err := call(declared); err != nil || called != 1 {
		t.Fatalf("OWNER 呼叫宣告的方法 = %v，handler %d 次", err, called)
	}
	for name, p := range map[string]identity.Principal{"零角色": principal(identity.PrincipalAdmin, 4), "OWNER 但沒有宣告": principal(identity.PrincipalAdmin, 3, adminperm.RoleOwner)} {
		current, called = p, 0
		procedure := declared
		if strings.HasPrefix(name, "OWNER") {
			procedure = undeclared
		}
		if err := call(procedure); connect.CodeOf(err) != connect.CodePermissionDenied || called != 0 {
			t.Errorf("%s：err=%v，handler %d 次", name, err, called)
		}
	}
}

// SellerProspectAdminService 的 12 支方法各自一個不同的權限，授予 OWNER；零角色的管理員沒有任何一個。
func TestEveryProspectMethodDeclaresItsOwnPermission(t *testing.T) {
	want := map[string]adminperm.Permission{
		sellerv1alpha1connect.SellerProspectAdminServiceListProspectsProcedure:            adminperm.SellerProspectsList,
		sellerv1alpha1connect.SellerProspectAdminServiceCreateProspectProcedure:           adminperm.SellerProspectsCreate,
		sellerv1alpha1connect.SellerProspectAdminServicePreviewProspectProcedure:          adminperm.SellerProspectsPreview,
		sellerv1alpha1connect.SellerProspectAdminServiceGetProspectProcedure:              adminperm.SellerProspectsGet,
		sellerv1alpha1connect.SellerProspectAdminServiceUpdateProspectProcedure:           adminperm.SellerProspectsUpdate,
		sellerv1alpha1connect.SellerProspectAdminServiceArchiveProspectProcedure:          adminperm.SellerProspectsArchive,
		sellerv1alpha1connect.SellerProspectAdminServiceRestoreProspectProcedure:          adminperm.SellerProspectsRestore,
		sellerv1alpha1connect.SellerProspectAdminServiceMergeProspectsProcedure:           adminperm.SellerProspectsMerge,
		sellerv1alpha1connect.SellerProspectAdminServiceConvertProspectProcedure:          adminperm.SellerProspectsConvert,
		sellerv1alpha1connect.SellerProspectAdminServiceListProspectDuplicatesProcedure:   adminperm.SellerProspectDuplicatesList,
		sellerv1alpha1connect.SellerProspectAdminServiceListProspectTaxIDMatchesProcedure: adminperm.SellerProspectTaxIDMatchesList,
		sellerv1alpha1connect.SellerProspectAdminServiceMarkProspectsDistinctProcedure:    adminperm.SellerProspectDistinctPairsCreate,
	}
	svc := sellerv1alpha1connect.SellerProspectAdminServiceName
	d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(svc))
	if err != nil {
		t.Fatal(err)
	}
	if n := d.(protoreflect.ServiceDescriptor).Methods().Len(); n != len(want) {
		t.Fatalf("service 有 %d 支方法，測試列了 %d 支", n, len(want))
	}
	seen := map[adminperm.Permission]bool{}
	for procedure, permission := range want {
		got, ok := declarations[procedure]
		if !ok || got.permission != permission || got.exemptReason != "" {
			t.Errorf("%s 的宣告 = %+v，預期 %s", procedure, got, permission)
		}
		if seen[permission] {
			t.Errorf("權限 %s 被兩支方法共用", permission)
		}
		seen[permission] = true
		if !adminperm.Allows([]adminperm.Role{adminperm.RoleOwner}, permission) || adminperm.Allows(nil, permission) {
			t.Errorf("%s：OWNER 要允許、零角色要拒絕", permission)
		}
	}
}
