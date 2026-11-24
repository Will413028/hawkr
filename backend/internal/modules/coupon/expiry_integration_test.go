//go:build integration

package coupon_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

type expiryFixture struct {
	*fixture
	module *coupon.Module
	mu     sync.Mutex
	got    []coupon.VoucherExpiring
}

// expirySetup 在 fixture 的資料庫上另建一個帶 Events 的模組，並在測試接線 VoucherExpiring 的訂閱者（cmd/hawkr 不登記）。
func expirySetup(t *testing.T) *expiryFixture {
	t.Helper()
	f := &expiryFixture{fixture: setup(t)}
	bus := events.NewBus()
	events.Subscribe(bus, func(_ context.Context, e coupon.VoucherExpiring) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.got = append(f.got, e)
		return nil
	})
	f.module = coupon.New(coupon.Deps{DB: f.db, Stores: f.stores, Members: f.members, Buyers: f.buyers, StoreDirectory: f.dir, Events: bus, NewID: uuid.NewV7, Now: f.now})
	return f
}

func (f *expiryFixture) setNow(t time.Time) { f.clock.Store(t.UnixNano()) }

// run 執行一次提醒並回傳這次新發布的事件。
func (f *expiryFixture) run() []coupon.VoucherExpiring {
	f.t.Helper()
	f.mu.Lock()
	before := len(f.got)
	f.mu.Unlock()
	n, err := f.module.RunExpiryReminder(context.Background())
	if err != nil {
		f.t.Fatalf("到期提醒：%v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if n != len(f.got)-before {
		f.t.Fatalf("回報發布 %d 則，訂閱者收到 %d 則", n, len(f.got)-before)
	}
	return slices.Clone(f.got[before:])
}

func voucherIDs(es []coupon.VoucherExpiring) []uuid.UUID {
	ids := make([]uuid.UUID, len(es))
	for i, e := range es {
		ids[i] = e.VoucherID
	}
	return ids
}

// 時鐘跨 UTC 日界：窗是「到下一個 UTC 日結束」，游標讓重跑不重發；只有 AVAILABLE 的券算。
func TestExpiryReminderEmitsOncePerVoucherAcrossAUTCDayBoundary(t *testing.T) {
	t.Parallel()
	f := expirySetup(t)
	endAt := func(at time.Time) func(*coupon.CreateCampaignInput) {
		return func(in *coupon.CreateCampaignInput) { in.EndAt = at }
	}
	day2Evening := time.Date(2026, 12, 2, 20, 0, 0, 0, time.UTC)
	day3Noon := time.Date(2026, 12, 3, 12, 0, 0, 0, time.UTC)
	soon := f.campaign(endAt(day2Evening))
	later := f.campaign(endAt(day3Noon))
	held := f.campaign(endAt(day2Evening.Add(-time.Hour)))
	ended := f.campaign(endAt(day2Evening.Add(-2 * time.Hour)))
	vSoon, vLater := f.claim(soon, buyer1), f.claim(later, buyer1)
	vHeld := f.claim(held, buyer2)
	f.claim(ended, buyer3)
	f.mustLock(vHeld, "ORD-EXPIRY-1")
	if _, err := f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: ended.ID, AdminID: adminID, Reason: "測試"}); err != nil {
		t.Fatal(err)
	}

	// UTC 12/1 17:30（台北已是 12/2 01:30）：窗到 12/3 00:00Z，只含 soon；LOCKED 與 VOIDED 不算。
	f.setNow(time.Date(2026, 12, 1, 17, 30, 0, 0, time.UTC))
	first := f.run()
	if !slices.Equal(voucherIDs(first), []uuid.UUID{vSoon.ID}) {
		t.Fatalf("第一次應只發 soon 的券 %s，實際 %v", vSoon.ID, voucherIDs(first))
	}
	e := first[0]
	if e.BuyerID != buyer1 || e.CampaignID != soon.ID || e.CampaignTitle != soon.Title || !e.ExpiresAt.Equal(day2Evening) || !e.OccurredAt.Equal(f.now()) || e.EventID == uuid.Nil() {
		t.Fatalf("事件內容不對：%+v", e)
	}
	if again := f.run(); len(again) != 0 {
		t.Fatalf("同一個時鐘重跑不該再發：%v", voucherIDs(again))
	}
	f.setNow(time.Date(2026, 12, 1, 23, 59, 59, 0, time.UTC))
	if again := f.run(); len(again) != 0 {
		t.Fatalf("同一個 UTC 日內不該再發：%v", voucherIDs(again))
	}

	// 跨過 UTC 日界：窗到 12/4 00:00Z，later 進窗；soon 不重發。
	f.setNow(time.Date(2026, 12, 2, 0, 0, 1, 0, time.UTC))
	second := f.run()
	if !slices.Equal(voucherIDs(second), []uuid.UUID{vLater.ID}) {
		t.Fatalf("跨日後應只發 later 的券 %s，實際 %v", vLater.ID, voucherIDs(second))
	}
	if again := f.run(); len(again) != 0 {
		t.Fatalf("跨日後重跑不該再發：%v", voucherIDs(again))
	}
}

func TestExpiryReminderSkipsVouchersThatAlreadyExpired(t *testing.T) {
	t.Parallel()
	f := expirySetup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) { in.EndAt = base.Add(6 * time.Hour) })
	f.claim(c, buyer1)
	f.setNow(base.Add(7 * time.Hour))
	if got := f.run(); len(got) != 0 {
		t.Fatalf("已過期的券不提醒：%v", voucherIDs(got))
	}
}

func TestModuleRegistersTheReminderJobOnlyWhenEventsAreGiven(t *testing.T) {
	t.Parallel()
	f := expirySetup(t)
	workers := river.NewWorkers()
	f.module.AddWorkers(workers)
	if len(f.module.PeriodicJobs()) != 1 {
		t.Fatal("要有一個排程")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("沒給 Events 時 PeriodicJobs 應該 panic")
		}
	}()
	f.m.PeriodicJobs()
}
