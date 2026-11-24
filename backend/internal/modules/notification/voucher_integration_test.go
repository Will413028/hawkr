//go:build integration

package notification

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

type scheduleJobs struct {
	*recorder
	mu sync.Mutex
	at map[uuid.UUID]time.Time
}

func (s *scheduleJobs) Enqueue(ctx context.Context, a river.JobArgs, opts *river.InsertOpts) error {
	if d, ok := a.(intake.DeliverArgs); ok {
		s.mu.Lock()
		s.at[d.DeliveryID] = opts.ScheduledAt
		s.mu.Unlock()
	}
	return s.recorder.Enqueue(ctx, a, opts)
}

func (s *scheduleJobs) scheduledAt(id uuid.UUID) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.at[id]
	return t, ok
}

type buyerResolver struct {
	registry.Resolver
	buyers *buyer.Module
}

func (r buyerResolver) BuyerAccounts(ctx context.Context, ids []int64) (map[int64]int64, error) {
	return r.buyers.ResolvePrincipals(ctx, ids)
}

type fakeVouchers struct {
	mu          sync.Mutex
	unavailable map[uuid.UUID]bool
}

func (f *fakeVouchers) VoucherAvailable(_ context.Context, _ int64, id uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.unavailable[id], nil
}

func (f *fakeVouchers) retire(id uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unavailable[id] = true
}

type voucherEnv struct {
	*fixture
	clock    *testClock
	sched    *scheduleJobs
	vouchers *fakeVouchers
}

func setupVoucher(t *testing.T) *voucherEnv {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "notification", Dir: Migrations()},
	))
	logs := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(logs, nil))
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Media: media.NewFake(), Now: time.Now, Logger: log})
	events.Subscribe[identity.AccountCreated](bus, buyers.OnAccountCreated)
	rec := &recorder{}
	sched := &scheduleJobs{recorder: rec, at: map[uuid.UUID]time.Time{}}
	clock := &testClock{t: time.Date(2026, 11, 24, 10, 0, 0, 0, twtime.Taipei)}
	vouchers := &fakeVouchers{unavailable: map[uuid.UUID]bool{}}
	push, mail, contacts := fcm.NewFake(nil), resend.NewFake(nil), &fakeContacts{m: map[int64]identity.ContactEmail{}}
	m := newModule(Deps{
		DB: d, Identity: id, Buyer: buyers, Seller: &fakeStores{retired: map[int64]bool{}}, Resolver: buyerResolver{buyers: buyers}, Validity: vouchers,
		Jobs: sched, Now: clock.Now, NewID: uuid.NewV7, Logger: log,
		Contacts: contacts, Locales: id, Push: push, Mail: mail, ImageURL: testImageURL, SuppressionKeys: testSuppressionKeys,
	}, testRegistry{})
	events.SubscribeNamed(bus, coupon.ExpiringHandler, m.OnVoucherExpiring)
	f := &fixture{db: d, identity: id, buyer: buyers, bus: bus, jobs: rec, module: m, logs: logs, push: push, mail: mail, contacts: contacts}
	return &voucherEnv{fixture: f, clock: clock, sched: sched, vouchers: vouchers}
}

func (e *voucherEnv) expiring(t *testing.T, buyerID int64, voucher uuid.UUID) {
	t.Helper()
	err := e.db.InTx(context.Background(), func(ctx context.Context) error {
		return events.PublishRequired(ctx, e.bus, coupon.VoucherExpiring{
			EventID: uuid.NewV7(), OccurredAt: e.clock.Now(), BuyerID: buyerID, VoucherID: voucher, CampaignID: uuid.NewV7(), ExpiresAt: e.clock.Now().Add(24 * time.Hour).UTC(),
		}, coupon.ExpiringHandler)
	})
	if err != nil {
		t.Fatal(err)
	}
	e.run(t)
}

func (e *voucherEnv) buyerWithToken(t *testing.T, token string) (account, buyerID int64) {
	t.Helper()
	account, buyerID = e.newBuyer(t)
	e.addToken(t, account, "BUYER", token)
	return account, buyerID
}

func TestVoucherExpiringCreatesOneInboxRowAndOnePushPerVoucher(t *testing.T) {
	e := setupVoucher(t)
	account, buyerID := e.buyerWithToken(t, "tok-1")
	voucher := uuid.NewV7()
	e.expiring(t, buyerID, voucher)
	e.expiring(t, buyerID, voucher)
	if n := e.typeCount(t, account, "voucher_expiring"); n != 1 {
		t.Fatalf("重複事件後收件匣有 %d 列，預期 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1 AND channel = 'PUSH'`, account); n != 1 {
		t.Fatalf("重複事件後有 %d 筆推播投遞，預期 1", n)
	}
	id := e.deliveryID(t, account, "PUSH")
	if err := e.deliver(id, false); err != nil {
		t.Fatal(err)
	}
	if s := e.state(t, id); s.Status != "SENT" || e.push.Delivered() != 1 {
		t.Fatalf("狀態 %+v，送出 %d 次", s, e.push.Delivered())
	}
	other := uuid.NewV7()
	e.expiring(t, buyerID, other)
	if n := e.typeCount(t, account, "voucher_expiring"); n != 2 {
		t.Errorf("兩張券應有兩則（不聚合），現在 %d 列", n)
	}
}

func TestVoucherNoLongerAvailableAtSendTimeSendsNoPush(t *testing.T) {
	e := setupVoucher(t)
	account, buyerID := e.buyerWithToken(t, "tok-1")
	voucher := uuid.NewV7()
	e.expiring(t, buyerID, voucher)
	id := e.deliveryID(t, account, "PUSH")
	e.vouchers.retire(voucher)
	if err := e.deliver(id, false); err != nil {
		t.Fatal(err)
	}
	s := e.state(t, id)
	if s.Status != "FAILED_PERMANENT" || s.Reason == nil || *s.Reason != "no_longer_valid" || e.push.Delivered() != 0 || len(e.push.Calls()) != 0 {
		t.Fatalf("狀態 %+v，呼叫 %d 次", s, len(e.push.Calls()))
	}
	if n := e.typeCount(t, account, "voucher_expiring"); n != 1 {
		t.Errorf("收件匣列仍應保留，現在 %d 列", n)
	}
}

func TestMarketingPushIsHeldDuringQuietHoursButTheInboxRowIsImmediate(t *testing.T) {
	taipei := func(day, hour, minute int) time.Time {
		return time.Date(2026, 11, day, hour, minute, 0, 0, twtime.Taipei)
	}
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"23:00 延到隔天 09:00", taipei(24, 23, 0), taipei(25, 9, 0)},
		{"凌晨 02:00 延到當天 09:00", taipei(25, 2, 0), taipei(25, 9, 0)},
		{"10:00 立即", taipei(24, 10, 0), time.Time{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := setupVoucher(t)
			account, buyerID := e.buyerWithToken(t, "tok-1")
			e.clock.set(c.now)
			e.expiring(t, buyerID, uuid.NewV7())
			if n := e.typeCount(t, account, "voucher_expiring"); n != 1 {
				t.Fatalf("收件匣 %d 列，預期立即建立 1 列", n)
			}
			got, ok := e.sched.scheduledAt(e.deliveryID(t, account, "PUSH"))
			if !ok || !got.Equal(c.want) {
				t.Errorf("推播排程 %v（有排：%v），預期 %v", got, ok, c.want)
			}
		})
	}
}

func TestQuietHoursOnlyDelayMarketingPush(t *testing.T) {
	e := setupVoucher(t)
	account, _ := e.buyerWithToken(t, "tok-1")
	e.clock.set(time.Date(2026, 11, 24, 23, 0, 0, 0, twtime.Taipei))
	id := e.pushDelivery(t, account, "e1")
	if got, ok := e.sched.scheduledAt(id); !ok || !got.IsZero() {
		t.Errorf("非行銷類推播在 23:00 的排程 %v（有排：%v），預期立即", got, ok)
	}
}
