//go:build integration

package notification

import (
	"context"
	"errors"
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

type buyerResolver struct {
	registry.Resolver
	buyers *buyer.Module
}

func (r buyerResolver) BuyerAccounts(ctx context.Context, ids []int64) (map[int64]int64, error) {
	return r.buyers.ResolvePrincipals(ctx, ids)
}

type noVouchers struct{}

func (noVouchers) VoucherAvailable(context.Context, int64, uuid.UUID) (bool, error) { return true, nil }

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
	clock := &testClock{t: time.Date(2026, 11, 24, 10, 0, 0, 0, twtime.Taipei)}
	vouchers := &fakeVouchers{unavailable: map[uuid.UUID]bool{}}
	push, mail, contacts := fcm.NewFake(nil), resend.NewFake(nil), &fakeContacts{m: map[int64]identity.ContactEmail{}}
	m := newModule(Deps{
		DB: d, Identity: id, Buyer: buyers, Seller: &fakeStores{retired: map[int64]bool{}}, Resolver: buyerResolver{buyers: buyers}, Validity: vouchers,
		Jobs: rec, Now: clock.Now, NewID: uuid.NewV7, Logger: log,
		Contacts: contacts, Locales: id, Push: push, Mail: mail, ImageURL: testImageURL, SuppressionKeys: testSuppressionKeys,
	}, testRegistry{})
	events.SubscribeNamed(bus, coupon.ExpiringHandler, m.OnVoucherExpiring)
	f := &fixture{db: d, identity: id, buyer: buyers, bus: bus, jobs: rec, module: m, logs: logs, push: push, mail: mail, contacts: contacts}
	return &voucherEnv{fixture: f, clock: clock, vouchers: vouchers}
}

func (e *voucherEnv) expiring(t *testing.T, buyerID int64, voucher uuid.UUID) {
	t.Helper()
	e.expiringAt(t, buyerID, voucher, e.clock.Now().Add(24*time.Hour).UTC())
}

func (e *voucherEnv) expiringAt(t *testing.T, buyerID int64, voucher uuid.UUID, expiresAt time.Time) {
	t.Helper()
	err := e.db.InTx(context.Background(), func(ctx context.Context) error {
		return events.PublishRequired(ctx, e.bus, coupon.VoucherExpiring{
			EventID: uuid.NewV7(), OccurredAt: e.clock.Now(), BuyerID: buyerID, VoucherID: voucher, CampaignID: uuid.NewV7(), ExpiresAt: expiresAt,
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

func taipeiAt(day, hour, minute int) time.Time {
	return time.Date(2026, 11, day, hour, minute, 0, 0, twtime.Taipei)
}

func requireSnooze(t *testing.T, err error, want time.Duration) {
	t.Helper()
	var snooze *river.JobSnoozeError
	if !errors.As(err, &snooze) || snooze.Duration != want {
		t.Fatalf("預期 snooze %v，得到 %v", want, err)
	}
}

func TestMarketingPushIsHeldDuringQuietHoursButTheInboxRowIsImmediate(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		hold time.Duration
	}{
		{"23:00 延到隔天 09:00", taipeiAt(24, 23, 0), 10 * time.Hour},
		{"凌晨 02:00 延到當天 09:00", taipeiAt(25, 2, 0), 7 * time.Hour},
		{"10:00 立即", taipeiAt(24, 10, 0), 0},
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
			id := e.deliveryID(t, account, "PUSH")
			err := e.deliver(id, false)
			if c.hold > 0 {
				requireSnooze(t, err, c.hold)
				if s := e.state(t, id); s.Status != "PENDING" || len(e.push.Calls()) != 0 {
					t.Fatalf("靜音時段狀態 %+v，呼叫 %d 次，預期仍 PENDING 且沒有送出", s, len(e.push.Calls()))
				}
				e.clock.set(c.now.Add(c.hold))
				err = e.deliver(id, false)
			}
			if err != nil {
				t.Fatal(err)
			}
			if s := e.state(t, id); s.Status != "SENT" || e.push.Delivered() != 1 {
				t.Errorf("靜音結束後狀態 %+v，送出 %d 次", s, e.push.Delivered())
			}
		})
	}
}

// 重試落在靜音時段：21:40 的第一次嘗試暫時性失敗，22:30 的重試被延到 09:00，不送、不佔重試次數（final 的那次也一樣延後而不是記永久失敗）。
func TestARetryInsideQuietHoursIsDeferredNotSent(t *testing.T) {
	e := setupVoucher(t)
	account, buyerID := e.buyerWithToken(t, "tok-1")
	e.clock.set(taipeiAt(24, 21, 40))
	e.expiring(t, buyerID, uuid.NewV7())
	id := e.deliveryID(t, account, "PUSH")
	e.push.Script("tok-1", fcm.OutcomeTransient)
	if err := e.deliver(id, false); err == nil {
		t.Fatal("暫時性錯誤要回錯誤讓 River 重試")
	}
	e.clock.set(taipeiAt(24, 22, 30))
	requireSnooze(t, e.deliver(id, true), 10*time.Hour+30*time.Minute)
	if s := e.state(t, id); s.Status != "PENDING" || e.push.Delivered() != 0 {
		t.Fatalf("狀態 %+v，送出 %d 次，預期 PENDING 且沒有送出", s, e.push.Delivered())
	}
	e.clock.set(taipeiAt(25, 9, 0))
	if err := e.deliver(id, false); err != nil {
		t.Fatal(err)
	}
	if s := e.state(t, id); s.Status != "SENT" || e.push.Delivered() != 1 {
		t.Errorf("09:00 後狀態 %+v，送出 %d 次", s, e.push.Delivered())
	}
}

// 時效性例外：券在靜音結束前就到期時不延後，立即送出。
func TestVoucherExpiringBeforeQuietHoursEndIsSentImmediately(t *testing.T) {
	cases := []struct {
		name    string
		expires time.Time
		hold    bool
	}{
		{"06:00 到期，23:30 建立：立即送出", taipeiAt(25, 6, 0), false},
		{"09:00 整到期：立即送出", taipeiAt(25, 9, 0), false},
		{"10:00 到期：延後到 09:00", taipeiAt(25, 10, 0), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := setupVoucher(t)
			account, buyerID := e.buyerWithToken(t, "tok-1")
			e.clock.set(taipeiAt(24, 23, 30))
			e.expiringAt(t, buyerID, uuid.NewV7(), c.expires.UTC())
			id := e.deliveryID(t, account, "PUSH")
			err := e.deliver(id, false)
			if c.hold {
				requireSnooze(t, err, 9*time.Hour+30*time.Minute)
				if s := e.state(t, id); s.Status != "PENDING" || e.push.Delivered() != 0 {
					t.Fatalf("狀態 %+v，送出 %d 次", s, e.push.Delivered())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if s := e.state(t, id); s.Status != "SENT" || e.push.Delivered() != 1 {
				t.Errorf("狀態 %+v，送出 %d 次，預期立即送出", s, e.push.Delivered())
			}
		})
	}
}

func TestQuietHoursOnlyDelayMarketingPush(t *testing.T) {
	e := setupVoucher(t)
	account, _ := e.buyerWithToken(t, "tok-1")
	e.clock.set(taipeiAt(24, 23, 0))
	id := e.pushDelivery(t, account, "e1")
	if err := e.deliver(id, false); err != nil {
		t.Fatalf("非行銷類推播在 23:00 不該延後：%v", err)
	}
	if s := e.state(t, id); s.Status != "SENT" || e.push.Delivered() != 1 {
		t.Errorf("狀態 %+v，送出 %d 次", s, e.push.Delivered())
	}
}
