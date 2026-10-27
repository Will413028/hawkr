package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

func TestParseAdminRole(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		action adminRoleAction
		id     int64
		email  string
		valid  bool
	}{
		{"以 ID 授予", []string{"grant", "-admin", "7", "-role", "OWNER", "-reason", "上線前授予自己"}, adminRoleGrant, 7, "", true},
		{"以 email 撤銷", []string{"revoke", "-admin", "ops@gmail.com", "-role", "OWNER", "-reason", "離職"}, adminRoleRevoke, 0, "ops@gmail.com", true},
		{"沒有子命令", nil, "", 0, "", false},
		{"不認得的子命令", []string{"list", "-admin", "7", "-role", "OWNER", "-reason", "x"}, "", 0, "", false},
		{"沒有 -admin", []string{"grant", "-role", "OWNER", "-reason", "x"}, "", 0, "", false},
		{"-admin 不是 ID 也不是 email", []string{"grant", "-admin", "ops", "-role", "OWNER", "-reason", "x"}, "", 0, "", false},
		{"-admin 是 0", []string{"grant", "-admin", "0", "-role", "OWNER", "-reason", "x"}, "", 0, "", false},
		{"-admin 是負數", []string{"grant", "-admin", "-3", "-role", "OWNER", "-reason", "x"}, "", 0, "", false},
		{"沒有 -role（沒有預設）", []string{"grant", "-admin", "7", "-reason", "x"}, "", 0, "", false},
		{"-role 不是角色", []string{"grant", "-admin", "7", "-role", "VIEWER", "-reason", "x"}, "", 0, "", false},
		{"沒有 -reason", []string{"grant", "-admin", "7", "-role", "OWNER"}, "", 0, "", false},
		{"-reason 只有空白", []string{"grant", "-admin", "7", "-role", "OWNER", "-reason", "  "}, "", 0, "", false},
		{"多餘的參數", []string{"grant", "-admin", "7", "-role", "OWNER", "-reason", "x", "extra"}, "", 0, "", false},
		{"不認得的旗標", []string{"grant", "-admin", "7", "-role", "OWNER", "-reason", "x", "-by", "me"}, "", 0, "", false},
	} {
		action, in, err := parseAdminRole(tc.args, "will")
		if (err == nil) != tc.valid {
			t.Errorf("%s：err = %v", tc.name, err)
			continue
		}
		if !tc.valid {
			continue
		}
		if action != tc.action || in.AdminID != tc.id || in.Email != tc.email || in.Role != adminperm.RoleOwner ||
			in.Source != identity.AdminRoleSourceCLI || in.OSUser != "will" || in.Reason == "" {
			t.Errorf("%s：%q %+v", tc.name, action, in)
		}
	}
}

type fakeAdminRoles struct {
	grant, revoke identity.AdminRoleChange
	err           error
	calls         []string
}

func (f *fakeAdminRoles) GrantAdminRole(context.Context, identity.AdminRoleChangeInput) (identity.AdminRoleChange, error) {
	f.calls = append(f.calls, "grant")
	return f.grant, f.err
}

func (f *fakeAdminRoles) RevokeAdminRole(context.Context, identity.AdminRoleChangeInput) (identity.AdminRoleChange, error) {
	f.calls = append(f.calls, "revoke")
	return f.revoke, f.err
}

// 撤掉之後沒有任何管理員持有該角色時印出警告（允許，CLI 本身就是 break-glass）；還有人持有時不警告。
func TestApplyAdminRoleWarnsWhenTheLastOwnerIsRevoked(t *testing.T) {
	in := identity.AdminRoleChangeInput{AdminID: 7, Role: adminperm.RoleOwner}
	for _, tc := range []struct {
		name   string
		action adminRoleAction
		res    identity.AdminRoleChange
		warns  bool
	}{
		{"撤銷最後一個 OWNER", adminRoleRevoke, identity.AdminRoleChange{AdminID: 7, Role: adminperm.RoleOwner, HolderRemains: false}, true},
		{"撤銷之後還有 OWNER", adminRoleRevoke, identity.AdminRoleChange{AdminID: 7, Role: adminperm.RoleOwner, HolderRemains: true}, false},
		{"授予不警告", adminRoleGrant, identity.AdminRoleChange{AdminID: 7, Role: adminperm.RoleOwner}, false},
	} {
		api := &fakeAdminRoles{grant: tc.res, revoke: tc.res}
		var out bytes.Buffer
		if err := applyAdminRole(t.Context(), api, tc.action, in, &out); err != nil {
			t.Fatalf("%s：%v", tc.name, err)
		}
		if got := strings.Contains(out.String(), "警告"); got != tc.warns {
			t.Errorf("%s：警告 = %v，輸出 %q", tc.name, got, out.String())
		}
		if len(api.calls) != 1 || api.calls[0] != string(tc.action) {
			t.Errorf("%s：呼叫 %v", tc.name, api.calls)
		}
	}
}

func TestApplyAdminRolePropagatesErrorsWithoutPrinting(t *testing.T) {
	boom := errors.New("管理員不存在")
	for _, action := range []adminRoleAction{adminRoleGrant, adminRoleRevoke} {
		var out bytes.Buffer
		err := applyAdminRole(t.Context(), &fakeAdminRoles{err: boom}, action, identity.AdminRoleChangeInput{}, &out)
		if !errors.Is(err, boom) || out.Len() != 0 {
			t.Errorf("%s：err = %v，輸出 %q", action, err, out.String())
		}
	}
}

func TestRunRejectsAdminRoleWithoutAValidSubcommand(t *testing.T) {
	if err := run(t.Context(), []string{"admin-role"}); err == nil {
		t.Fatal("沒有子命令應該失敗")
	}
}
