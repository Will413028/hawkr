//go:build integration

package identity_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/identity"

	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// admin 以第一次認證建立管理員（零角色），回傳 ID 與之後認證用的 token。
func (f *fixture) admin(t *testing.T, subject, email string) (int64, string) {
	t.Helper()
	f.admins.users[subject] = AdminIdentity{Email: email}
	tok := token(adminIssuer, subject)
	a, err := f.m.Authenticate(context.Background(), tok)
	if err != nil || a.Principal.Kind != PrincipalAdmin {
		t.Fatalf("建立管理員 = %+v, %v", a, err)
	}
	return a.Principal.ID, tok
}

func (f *fixture) adminRoles(t *testing.T, tok string) []adminperm.Role {
	t.Helper()
	a, err := f.m.Authenticate(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	return a.Principal.AdminRoles
}

func (f *fixture) roleEvents(t *testing.T, adminID int64) int {
	return f.count(t, `SELECT count(*) FROM identity.admin_role_events WHERE admin_id = $1`, adminID)
}

func change(adminID int64) AdminRoleChangeInput {
	return AdminRoleChangeInput{AdminID: adminID, Role: adminperm.RoleOwner, Reason: "  上線前授予自己  ", Source: AdminRoleSourceCLI, OSUser: "will"}
}

// 新管理員沒有角色；授予之後下一個請求就讀得到，撤銷之後下一個請求就沒有（不快取）；每次變更留下一筆事件。
func TestGrantAndRevokeAdminRoleWriteEventsAndTakeEffectImmediately(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, tok := f.admin(t, "user_ops", "Ops@Example.com")
	if roles := f.adminRoles(t, tok); len(roles) != 0 {
		t.Fatalf("新管理員的角色 = %v", roles)
	}

	res, err := f.m.GrantAdminRole(ctx, change(id))
	if err != nil || res.AdminID != id || res.Role != adminperm.RoleOwner || res.Action != AdminRoleActionGrant {
		t.Fatalf("授予 = %+v, %v", res, err)
	}
	if roles := f.adminRoles(t, tok); len(roles) != 1 || roles[0] != adminperm.RoleOwner {
		t.Fatalf("授予之後的角色 = %v", roles)
	}
	var action, reason, source, dbUser string
	var osUser *string
	var occurred time.Time
	if err := f.db.Pool().QueryRow(ctx, `SELECT action, reason, source, db_user, os_user, occurred_at FROM identity.admin_role_events WHERE admin_id = $1`, id).
		Scan(&action, &reason, &source, &dbUser, &osUser, &occurred); err != nil {
		t.Fatal(err)
	}
	var sessionUser string
	if err := f.db.Pool().QueryRow(ctx, `SELECT session_user`).Scan(&sessionUser); err != nil {
		t.Fatal(err)
	}
	if action != "GRANT" || reason != "上線前授予自己" || source != "CLI" || dbUser != sessionUser || osUser == nil || *osUser != "will" || !occurred.Equal(f.now) {
		t.Fatalf("事件 = %s %q %s %s %v %v", action, reason, source, dbUser, osUser, occurred)
	}

	// 重複授予回錯誤、不寫事件、角色列不變。
	_, err = f.m.GrantAdminRole(ctx, change(id))
	wantKind(t, err, apperr.KindAlreadyExists)
	if n := f.roleEvents(t, id); n != 1 {
		t.Fatalf("重複授予後事件 = %d", n)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.admin_roles WHERE admin_id = $1`, id); n != 1 {
		t.Fatalf("重複授予後角色列 = %d", n)
	}

	second, _ := f.admin(t, "user_two", "two@example.com")
	if _, err := f.m.GrantAdminRole(ctx, change(second)); err != nil {
		t.Fatal(err)
	}
	res, err = f.m.RevokeAdminRole(ctx, change(id))
	if err != nil || res.Action != AdminRoleActionRevoke || !res.HolderRemains {
		t.Fatalf("撤銷（還有別的 OWNER）= %+v, %v", res, err)
	}
	if roles := f.adminRoles(t, tok); len(roles) != 0 {
		t.Fatalf("撤銷之後的角色 = %v", roles)
	}
	if n := f.roleEvents(t, id); n != 2 {
		t.Fatalf("撤銷後事件 = %d", n)
	}

	// 撤銷沒有的角色回錯誤、不寫事件。
	_, err = f.m.RevokeAdminRole(ctx, change(id))
	wantKind(t, err, apperr.KindFailedPrecondition)
	if n := f.roleEvents(t, id); n != 2 {
		t.Fatalf("撤銷不存在的角色後事件 = %d", n)
	}

	// 撤掉最後一個 OWNER 是允許的，結果告知沒有人持有。
	res, err = f.m.RevokeAdminRole(ctx, change(second))
	if err != nil || res.HolderRemains {
		t.Fatalf("撤銷最後一個 OWNER = %+v, %v", res, err)
	}
}

func TestAdminRoleEventsLeaveTheOSUserEmptyWhenUnknown(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.admin(t, "user_ops", "ops@example.com")
	in := change(id)
	in.OSUser = ""
	if _, err := f.m.GrantAdminRole(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.admin_role_events WHERE admin_id = $1 AND os_user IS NULL`, id); n != 1 {
		t.Fatalf("os_user 為 NULL 的事件 = %d", n)
	}
}

// 以 email 指定管理員（不分大小寫）；只指定既有的管理員，不預建；email 對應多位時要求改用 ID。
func TestAdminRoleChangeFindsExistingAdminsOnly(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, tok := f.admin(t, "user_ops", "Ops@Example.com")
	byEmail := change(0)
	byEmail.Email = "ops@example.com"
	if res, err := f.m.GrantAdminRole(ctx, byEmail); err != nil || res.AdminID != id {
		t.Fatalf("以 email 授予 = %+v, %v", res, err)
	}
	if roles := f.adminRoles(t, tok); len(roles) != 1 {
		t.Fatalf("角色 = %v", roles)
	}

	admins := f.count(t, `SELECT count(*) FROM identity.admins`)
	missing := change(0)
	missing.Email = "nobody@example.com"
	_, err := f.m.GrantAdminRole(ctx, missing)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.m.GrantAdminRole(ctx, change(9999))
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.m.RevokeAdminRole(ctx, change(9999))
	wantKind(t, err, apperr.KindNotFound)
	if got := f.count(t, `SELECT count(*) FROM identity.admins`); got != admins {
		t.Fatalf("管理員數 %d → %d：授予不得預建管理員", admins, got)
	}

	f.admin(t, "user_a", "same@example.com")
	f.admin(t, "user_b", "SAME@example.com")
	ambiguous := change(0)
	ambiguous.Email = "same@example.com"
	_, err = f.m.GrantAdminRole(ctx, ambiguous)
	wantKind(t, err, apperr.KindInvalidInput)
}

func TestAdminRoleChangeValidatesItsInputBeforeWriting(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.admin(t, "user_ops", "ops@example.com")
	for name, mutate := range map[string]func(*AdminRoleChangeInput){
		"ID 與 email 都給了": func(in *AdminRoleChangeInput) { in.Email = "ops@example.com" },
		"ID 與 email 都沒給": func(in *AdminRoleChangeInput) { in.AdminID = 0 },
		"負的 ID":          func(in *AdminRoleChangeInput) { in.AdminID = -1 },
		"不是角色":           func(in *AdminRoleChangeInput) { in.Role = "VIEWER" },
		"沒有角色":           func(in *AdminRoleChangeInput) { in.Role = "" },
		"沒有來源":           func(in *AdminRoleChangeInput) { in.Source = "" },
		"不認得的來源":         func(in *AdminRoleChangeInput) { in.Source = "UI" },
		"沒有理由":           func(in *AdminRoleChangeInput) { in.Reason = "" },
		"理由只有空白":         func(in *AdminRoleChangeInput) { in.Reason = " \n\t " },
		"理由太長":           func(in *AdminRoleChangeInput) { in.Reason = strings.Repeat("理", 1001) },
	} {
		in := change(id)
		mutate(&in)
		_, err := f.m.GrantAdminRole(context.Background(), in)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("授予，%s：err=%v", name, err)
		}
		_, err = f.m.RevokeAdminRole(context.Background(), in)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("撤銷，%s：err=%v", name, err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM identity.admin_roles`) + f.count(t, `SELECT count(*) FROM identity.admin_role_events`); n != 0 {
		t.Fatalf("驗證失敗之後寫入了 %d 列", n)
	}
}

// 撤銷發出的 AdminRoleRevoked 與撤銷在同一個 transaction：訂閱者看得到未 commit 的刪除，失敗時角色與事件一起回滾。
func TestRevokePublishesAdminRoleRevokedInTheSameTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, tok := f.admin(t, "user_ops", "ops@example.com")
	if _, err := f.m.GrantAdminRole(ctx, change(id)); err != nil {
		t.Fatal(err)
	}
	var got []AdminRoleRevoked
	var insideTx, rolesInTx, rolesOutside int
	var subscriberErr error
	events.Subscribe(f.deps.Events, func(ctx context.Context, e AdminRoleRevoked) error {
		got = append(got, e)
		if _, ok := db.TxFrom(ctx); ok {
			insideTx++
		}
		if err := f.db.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM identity.admin_roles WHERE admin_id = $1`, e.AdminID).Scan(&rolesInTx); err != nil {
			return err
		}
		if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM identity.admin_roles WHERE admin_id = $1`, e.AdminID).Scan(&rolesOutside); err != nil {
			return err
		}
		return subscriberErr
	})

	subscriberErr = errors.New("訂閱者失敗")
	if _, err := f.m.RevokeAdminRole(ctx, change(id)); !errors.Is(err, subscriberErr) {
		t.Fatalf("訂閱者失敗時 err = %v", err)
	}
	if roles := f.adminRoles(t, tok); len(roles) != 1 {
		t.Fatalf("訂閱者失敗之後角色 = %v，撤銷應該回滾", roles)
	}
	if n := f.roleEvents(t, id); n != 1 {
		t.Fatalf("訂閱者失敗之後事件 = %d，只有授予那一筆", n)
	}

	subscriberErr = nil
	got = nil
	if _, err := f.m.RevokeAdminRole(ctx, change(id)); err != nil {
		t.Fatal(err)
	}
	want := AdminRoleRevoked{AdminID: id, Role: adminperm.RoleOwner, RevokedAt: f.now}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("事件 = %+v，預期 %+v", got, want)
	}
	if insideTx != 2 || rolesInTx != 0 || rolesOutside != 1 {
		t.Fatalf("訂閱者看到：在 transaction 內 %d 次、transaction 內的角色列 %d、其他連線看到的角色列 %d", insideTx, rolesInTx, rolesOutside)
	}
}

func TestGrantDoesNotPublishAdminRoleRevoked(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id, _ := f.admin(t, "user_ops", "ops@example.com")
	var n int
	events.Subscribe(f.deps.Events, func(context.Context, AdminRoleRevoked) error { n++; return nil })
	if _, err := f.m.GrantAdminRole(context.Background(), change(id)); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("授予發出了 %d 個 AdminRoleRevoked", n)
	}
}

func TestAdminAllowsReadsRolesInsideTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	owner, _ := f.admin(t, "user_owner", "owner@example.com")
	zero, _ := f.admin(t, "user_zero", "zero@example.com")
	if _, err := f.m.GrantAdminRole(ctx, change(owner)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.AdminAllows(ctx, owner, adminperm.DelegationSessionsCreate); err == nil {
		t.Fatal("不在 transaction 內呼叫應該回錯誤")
	}
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		for _, tc := range []struct {
			name  string
			admin int64
			perm  adminperm.Permission
			want  bool
		}{
			{"OWNER", owner, adminperm.DelegationSessionsCreate, true},
			{"零角色", zero, adminperm.DelegationSessionsCreate, false},
			{"不存在的管理員", 9999, adminperm.DelegationSessionsCreate, false},
			{"不在目錄的權限", owner, "delegation.no_such.create", false},
		} {
			got, err := f.m.AdminAllows(ctx, tc.admin, tc.perm)
			if err != nil || got != tc.want {
				t.Errorf("%s = %v, %v，預期 %v", tc.name, got, err, tc.want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// AdminAllows 以 FOR SHARE 鎖住角色列：它所在的 transaction 還沒結束時，撤銷的 DELETE 要等；結束之後撤銷才完成，
// 之後的檢查讀不到角色。以 pg_stat_activity 的鎖等待判斷，不看時間。
func TestRevokeWaitsForATransactionThatCheckedThePermission(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id, _ := f.admin(t, "user_ops", "ops@example.com")
	if _, err := f.m.GrantAdminRole(ctx, change(id)); err != nil {
		t.Fatal(err)
	}

	checked := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseCheck := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseCheck()
	var wg sync.WaitGroup
	var allowed bool
	var checkErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		checkErr = f.db.InTx(ctx, func(ctx context.Context) error {
			var err error
			if allowed, err = f.m.AdminAllows(ctx, id, adminperm.DelegationSessionsCreate); err != nil {
				return err
			}
			close(checked)
			<-release
			return nil
		})
	}()
	<-checked

	revoked := make(chan error, 1)
	go func() {
		_, err := f.m.RevokeAdminRole(ctx, change(id))
		revoked <- err
	}()
	waiting := false
	for !waiting {
		select {
		case err := <-revoked:
			t.Fatalf("撤銷在檢查的 transaction 結束之前就完成了：%v", err)
		default:
		}
		if err := f.db.Pool().QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE '%admin_roles%')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	releaseCheck()
	wg.Wait()
	if err := <-revoked; err != nil {
		t.Fatalf("撤銷 = %v", err)
	}
	if checkErr != nil || !allowed {
		t.Fatalf("檢查 = %v, %v", allowed, checkErr)
	}
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		got, err := f.m.AdminAllows(ctx, id, adminperm.DelegationSessionsCreate)
		if got {
			t.Error("撤銷之後仍然允許")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
