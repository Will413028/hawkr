//go:build integration

package delegation

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
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
	storeFn func(id int64) (seller.Store, error)
	social Social
	realSocial *social.Module
	media      *media.Fake
	sellers    *seller.Reader
	nextID     int64
}

type noSchedules struct{}

func (noSchedules) CancelScheduledBy(context.Context, social.ScheduleKind, []string, social.CancelReason) ([]uuid.UUID, error) {
	return nil, nil
}

func (noSchedules) CancelDelegatedSchedulesOfStore(context.Context, int64, social.CancelReason) ([]uuid.UUID, error) {
	return nil, nil
}

type fakeSeller struct{ f *fixture }

func (s fakeSeller) Store(_ context.Context, id int64) (seller.Store, error) { return s.f.storeFn(id) }

func setup(t *testing.T) *fixture { return build(t, false) }

func build(t *testing.T, withSocial bool) *fixture {
	t.Helper()
	f := &fixture{now: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC), storeFn: liveFn, social: noSchedules{}}
	modules := []migrate.Module{
		{Name: "identity", Dir: identity.Migrations()},
		{Name: "delegation", Dir: Migrations()},
	}
	if withSocial {
		modules = append(modules,
			migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
			migrate.Module{Name: "seller", Dir: seller.Migrations()},
			migrate.Module{Name: "social", Dir: social.Migrations()},
		)
	}
	steps := []testdb.Step{testdb.Migrations(modules...)}
	if withSocial {
		steps = append([]testdb.Step{testdb.Jobs()}, steps...)
	}
	d := pg.NewDB(t, steps...)
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
	if withSocial {
		f.media = media.NewFake()
		f.media.Key = func(token string) string { return "post/" + token + ".jpg" }
		f.sellers = seller.NewReader(d, func() time.Time { return f.now })
		f.realSocial = social.New(social.Deps{
			DB: d, Events: f.bus, Media: f.media, Jobs: &queue{}, Stores: f.sellers, StoreBlocks: noStoreBlocks{},
			ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: NewScheduleAuthorizer(f.deps())},
			NewID:               uuid.NewV7, Now: func() time.Time { return f.now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
		f.social = f.realSocial
		if _, err := d.Conn(context.Background()).Exec(context.Background(), `DO $$ DECLARE r record; BEGIN
			FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
			         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
			LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`); err != nil {
			t.Fatal(err)
		}
	}
	f.m = f.newDelegation()
	events.Subscribe(f.bus, f.m.OnAdminRoleRevoked)
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, f.m.OnPostPublished)
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, f.m.OnPostCreated)
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, f.m.OnPostUpdated)
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, f.m.OnPostUnpublished)
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, f.m.OnPostScheduled)
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, f.m.OnPostPublishRepeated)
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, f.m.OnPostUnpublishRepeated)
	events.Subscribe(f.bus, f.m.OnStoreSuspended)
	events.Subscribe(f.bus, f.m.OnStoreClosed)
	return f
}

func (f *fixture) deps() Deps {
	return Deps{DB: f.db, Events: f.bus, Identity: f.identity, Seller: fakeSeller{f}, Social: f.social,
		NewID: uuid.NewV7, Now: func() time.Time { return f.now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func (f *fixture) newDelegation() *Module { return New(f.deps()) }

func (f *fixture) admin(t *testing.T, email string) int64 {
	t.Helper()
	var id int64
	err := f.db.Pool().QueryRow(context.Background(), `INSERT INTO identity.admins (access_subject, email) VALUES ($1, $2) RETURNING id`, uuid.NewV7().String(), email).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *fixture) grantOwner(t *testing.T, id int64) {
	t.Helper()
	_, err := f.identity.GrantAdminRole(context.Background(), identity.AdminRoleChangeInput{
		AdminID: id, Role: adminperm.RoleOwner, Reason: "測試的管理員", Source: identity.AdminRoleSourceCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
}

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

type queue struct{}

func (*queue) Enqueue(ctx context.Context, _ river.JobArgs, _ *river.InsertOpts) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("enqueue outside transaction")
	}
	return nil
}

type noStoreBlocks struct{}

func (noStoreBlocks) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) { return false, nil }

func (noSchedules) PendingSchedulerRefs(context.Context, social.ScheduleKind) ([]string, error) {
	return nil, nil
}
