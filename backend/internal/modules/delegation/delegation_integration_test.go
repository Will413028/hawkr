//go:build integration

package delegation

import (
	"context"
	"crypto/sha256"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type fixture struct {
	m            *Module
	identity     *identity.Module
	db           *db.DB
	bus          *events.Bus
	now          time.Time
	completed    []DelegationOperationCompleted
	completedErr error
	// storeFn 是 seller 的替身：依店家 ID 回店家或錯誤，預設是營業中的店家。
	storeFn func(id int64) (seller.Store, error)
}

type fakeSeller struct{ f *fixture }

func (s fakeSeller) Store(_ context.Context, id int64) (seller.Store, error) { return s.f.storeFn(id) }

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{now: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC), storeFn: liveFn}
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "delegation", Dir: Migrations()},
	))
	f.db = d
	f.bus = events.NewBus()
	events.Subscribe(f.bus, func(_ context.Context, e DelegationOperationCompleted) error {
		if f.completedErr != nil {
			return f.completedErr
		}
		f.completed = append(f.completed, e)
		return nil
	})
	f.identity = identity.New(identity.Deps{DB: d, Events: f.bus, Now: func() time.Time { return f.now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	f.m = f.newDelegation()
	events.Subscribe(f.bus, f.m.OnAdminRoleRevoked)
	return f
}

func (f *fixture) newDelegation() *Module {
	return New(Deps{DB: f.db, Events: f.bus, Identity: f.identity, Seller: fakeSeller{f},
		NewID: uuid.NewV7, Now: func() time.Time { return f.now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func (f *fixture) admin(t *testing.T, email string) int64 {
	t.Helper()
	var id int64
	err := f.db.Pool().QueryRow(context.Background(), `INSERT INTO identity.admins (access_subject, email) VALUES ($1, $2) RETURNING id`, uuid.NewV7().String(), email).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// grantOwner 授予管理員 OWNER：開始代行要有 delegation.sessions.create 權限（ADR 2026-10-04-admin-roles）。
func (f *fixture) grantOwner(t *testing.T, id int64) {
	t.Helper()
	_, err := f.identity.GrantAdminRole(context.Background(), identity.AdminRoleChangeInput{
		AdminID: id, Role: adminperm.RoleOwner, Reason: "測試的管理員", Source: identity.AdminRoleSourceCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// deleteAdmin 模擬管理員的列不見了（測試顯示名稱查不到時的容錯）：角色與事件參照管理員，要先清掉。
func (f *fixture) deleteAdmin(t *testing.T, id int64) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		`DELETE FROM identity.admin_role_events WHERE admin_id = $1`,
		`DELETE FROM identity.admin_roles WHERE admin_id = $1`,
		`DELETE FROM identity.admins WHERE id = $1`,
	} {
		if _, err := f.db.Pool().Exec(ctx, q, id); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.Pool().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func tokenHash(value string) []byte { hash := sha256.Sum256([]byte(value)); return hash[:] }

func ptr[T any](value T) *T { return &value }
