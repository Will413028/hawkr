//go:build integration

package delegation

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func roleChange(admin int64) identity.AdminRoleChangeInput {
	return identity.AdminRoleChangeInput{AdminID: admin, Role: adminperm.RoleOwner, Reason: "測試撤銷", Source: identity.AdminRoleSourceCLI}
}

// 開始代行要有 delegation.sessions.create：資格開通、店家授權都齊了，沒有角色的管理員仍被拒絕，沒有寫入任何 session。
func TestStartDelegationRequiresTheCreatePermission(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	admin := f.admin(t, "ops@hawkr.example")
	if err := f.m.SetAdminEligibility(ctx, AdminEligibilityChange{AdminID: admin, OperatorAdminID: admin, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	f.grant(t, admin, testStore)

	_, err := f.m.StartDelegation(ctx, startInput(admin, testStore, "ik-1"))
	wantDenial(t, err, DenialAdminNotAllowlisted)
	wantKind(t, err, apperr.KindPermissionDenied)
	if n := f.count(t, `SELECT count(*) FROM delegation.delegated_operation_sessions`); n != 0 {
		t.Fatalf("被拒絕之後寫入了 %d 個 session", n)
	}

	f.grantOwner(t, admin)
	if _, err := f.m.StartDelegation(ctx, startInput(admin, testStore, "ik-1")); err != nil {
		t.Fatalf("有 OWNER 之後 = %v", err)
	}
}

// 撤銷 OWNER 在同一個 transaction 內撤銷該管理員進行中的代行（原因 admin_role_revoked），其他管理員的代行不受影響；撤銷之後
// 開始代行立即被拒（不快取）。
func TestRevokingTheRoleEndsRunningDelegationsInTheSameTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	other := f.delegationAdmin(t, "other@hawkr.example", true)
	const otherStore = testStore + 1
	f.grant(t, admin, testStore)
	f.grant(t, other, otherStore)
	mine := f.start(t, startInput(admin, testStore, "ik-1"))
	theirs := f.start(t, startInput(other, otherStore, "ik-2"))

	if _, err := f.identity.RevokeAdminRole(ctx, roleChange(admin)); err != nil {
		t.Fatal(err)
	}
	var status, reason string
	var by *int64
	if err := f.db.Pool().QueryRow(ctx, `SELECT status, revoke_reason, revoked_by_admin_id FROM delegation.delegated_operation_sessions WHERE id = $1`, mine.Session.ID).
		Scan(&status, &reason, &by); err != nil {
		t.Fatal(err)
	}
	if status != "revoked" || reason != RevokeReasonAdminRoleRevoked || by != nil {
		t.Fatalf("被撤銷角色的管理員的代行 = %s %s %v，預期 revoked_by_admin_id 為 NULL（系統撤銷沒有操作者）", status, reason, by)
	}
	if err := f.db.Pool().QueryRow(ctx, `SELECT status FROM delegation.delegated_operation_sessions WHERE id = $1`, theirs.Session.ID).Scan(&status); err != nil || status != "active" {
		t.Fatalf("其他管理員的代行 = %s, %v", status, err)
	}
	_, err := f.m.StartDelegation(ctx, startInput(admin, testStore, "ik-3"))
	wantDenial(t, err, DenialAdminNotAllowlisted)
}

// 同一個 transaction：訂閱鏈上後面的訂閱者失敗時，撤銷角色與撤銷代行一起回滾。
func TestRoleRevocationAndDelegationRevocationRollBackTogether(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))
	boom := errors.New("後面的訂閱者失敗")
	events.Subscribe(f.bus, func(context.Context, identity.AdminRoleRevoked) error { return boom })

	if _, err := f.identity.RevokeAdminRole(ctx, roleChange(admin)); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	var status string
	if err := f.db.Pool().QueryRow(ctx, `SELECT status FROM delegation.delegated_operation_sessions WHERE id = $1`, s.Session.ID).Scan(&status); err != nil || status != "active" {
		t.Fatalf("回滾後的代行 = %s, %v", status, err)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.admin_roles WHERE admin_id = $1`, admin); n != 1 {
		t.Fatalf("回滾後的角色列 = %d", n)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.admin_role_events WHERE admin_id = $1 AND action = 'REVOKE'`, admin); n != 0 {
		t.Fatalf("回滾後的撤銷事件 = %d", n)
	}
}
