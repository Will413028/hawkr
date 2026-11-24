//go:build integration

package coupon_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
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

// expirySetup 在 fixture 的資料庫上另建一個帶 Events 的模組，並在測試接線具名的 VoucherExpiring 訂閱者（cmd/hawkr 不登記）。
// fail 回傳非 nil 時，那張券的 handler 失敗（在記錄事件之前，失敗的券不會出現在收到的事件裡）。
func expirySetup(t *testing.T, batch int32, fail func(uuid.UUID) error) *expiryFixture {
	t.Helper()
	f := &expiryFixture{fixture: setup(t)}
	bus := events.NewBus()
	events.SubscribeNamed(bus, coupon.ExpiringHandler, func(_ context.Context, e coupon.VoucherExpiring) error {
		if fail != nil {
			if err := fail(e.VoucherID); err != nil {
				return err
			}
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.got = append(f.got, e)
		return nil
	})
	f.module = f.newModule(bus, batch)
	return f
}

func (f *expiryFixture) newModule(bus *events.Bus, batch int32) *coupon.Module {
	return coupon.New(coupon.Deps{DB: f.db, Stores: f.stores, Buyers: f.buyers, StoreDirectory: f.dir, Events: bus, ExpiryBatchSize: batch, NewID: uuid.NewV7, Now: f.now})
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
	f.checkEmitted(n, before)
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.got[before:])
}

// runErr 執行一次提醒，回傳錯誤與這次新發布的事件。
func (f *expiryFixture) runErr() ([]coupon.VoucherExpiring, error) {
	f.t.Helper()
	f.mu.Lock()
	before := len(f.got)
	f.mu.Unlock()
	n, err := f.module.RunExpiryReminder(context.Background())
	f.checkEmitted(n, before)
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.got[before:]), err
}

func (f *expiryFixture) checkEmitted(n, before int) {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if n != len(f.got)-before {
		f.t.Fatalf("回報發布 %d 則，訂閱者收到 %d 則", n, len(f.got)-before)
	}
}

func voucherIDs(es []coupon.VoucherExpiring) []uuid.UUID {
	ids := make([]uuid.UUID, len(es))
	for i, e := range es {
		ids[i] = e.VoucherID
	}
	return ids
}

func (f *expiryFixture) reminders() int64 {
	return f.scalar("SELECT count(*) FROM coupon.voucher_expiry_reminders")
}

func endAt(at time.Time) func(*coupon.CreateCampaignInput) {
	return func(in *coupon.CreateCampaignInput) { in.EndAt = at }
}

func sameIDs(t *testing.T, label string, got []coupon.VoucherExpiring, want ...coupon.Voucher) {
	t.Helper()
	ids := voucherIDs(got)
	wantIDs := make([]uuid.UUID, len(want))
	for i, v := range want {
		wantIDs[i] = v.ID
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	slices.SortFunc(wantIDs, uuid.UUID.Compare)
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("%s：要提醒 %v，實際 %v", label, wantIDs, ids)
	}
}

// 提醒窗是 (now, now+24h]：進窗才提醒，重跑與時間往後都不重發；事件內容與寫入的紀錄正確。
func TestExpiryReminderEmitsOncePerVoucherWhenItEntersTheWindow(t *testing.T) {
	t.Parallel()
	f := expirySetup(t, 0, nil)
	end := base.Add(30 * time.Hour)
	c := f.campaign(endAt(end))
	v := f.claim(c, buyer1)

	f.setNow(base)
	if got := f.run(); len(got) != 0 {
		t.Fatalf("還有 30 小時，不該提醒：%v", voucherIDs(got))
	}
	f.setNow(base.Add(7 * time.Hour))
	first := f.run()
	sameIDs(t, "進窗", first, v)
	e := first[0]
	if e.BuyerID != buyer1 || e.CampaignID != c.ID || e.CampaignTitle != c.Title || !e.ExpiresAt.Equal(end) || !e.OccurredAt.Equal(f.now()) || e.EventID == uuid.Nil() {
		t.Fatalf("事件內容不對：%+v", e)
	}
	if f.reminders() != 1 {
		t.Fatalf("提醒紀錄 = %d，要 1", f.reminders())
	}
	if again := f.run(); len(again) != 0 {
		t.Fatalf("同一個時鐘重跑不該再發：%v", voucherIDs(again))
	}
	f.setNow(base.Add(20 * time.Hour))
	if again := f.run(); len(again) != 0 {
		t.Fatalf("之後的一輪不該再發：%v", voucherIDs(again))
	}
}

// 只有 AVAILABLE 的券算：LOCKED、VOIDED（D5 提前結束）都不提醒。
func TestExpiryReminderSkipsLockedAndVoidedVouchers(t *testing.T) {
	t.Parallel()
	f := expirySetup(t, 0, nil)
	end := base.Add(30 * time.Hour)
	open, held, ended := f.campaign(endAt(end)), f.campaign(endAt(end)), f.campaign(endAt(end))
	vOpen, vHeld, vEnded := f.claim(open, buyer1), f.claim(held, buyer2), f.claim(ended, buyer3)
	f.mustLock(vHeld, "ORD-EXPIRY-1")
	if _, err := f.m.EndCampaign(context.Background(), coupon.EndCampaignInput{CampaignID: ended.ID, AdminID: adminID, Reason: "測試"}); err != nil {
		t.Fatal(err)
	}
	if f.voucherStatus(vEnded.ID) != "VOIDED" {
		t.Fatalf("前置：提前結束應作廢 AVAILABLE 券，實際 %s", f.voucherStatus(vEnded.ID))
	}
	f.setNow(base.Add(7 * time.Hour))
	sameIDs(t, "只有 AVAILABLE", f.run(), vOpen)
}

// 進窗時被 LOCKED、之後釋放回 AVAILABLE 的券，下一輪照樣提醒。
func TestExpiryReminderRemindsAVoucherReleasedAfterBeingLocked(t *testing.T) {
	t.Parallel()
	f := expirySetup(t, 0, nil)
	c := f.campaign(endAt(base.Add(30 * time.Hour)))
	v := f.claim(c, buyer1)
	f.mustLock(v, "ORD-EXPIRY-2")
	f.setNow(base.Add(7 * time.Hour))
	if got := f.run(); len(got) != 0 {
		t.Fatalf("LOCKED 的券不提醒：%v", voucherIDs(got))
	}
	if _, err := f.m.Release(context.Background(), "ORD-EXPIRY-2"); err != nil {
		t.Fatal(err)
	}
	f.setNow(base.Add(8 * time.Hour))
	sameIDs(t, "釋放後", f.run(), v)
}

// 晚領的券：領券時離到期不少於 24 小時就提醒；不足 24 小時的領到時已在窗內，不補提醒。
func TestExpiryReminderCoversLateClaimsWithEnoughLeadButNotShortWindowClaims(t *testing.T) {
	t.Parallel()
	f := expirySetup(t, 0, nil)
	c := f.campaign(endAt(base.Add(72 * time.Hour)))
	f.setNow(base.Add(40 * time.Hour))
	early := f.claim(c, buyer1) // 離到期 32 小時
	f.setNow(base.Add(60 * time.Hour))
	late := f.claim(c, buyer2) // 離到期 12 小時
	f.setNow(base.Add(49 * time.Hour))
	sameIDs(t, "晚領但提前量足夠", f.run(), early)
	f.setNow(base.Add(61 * time.Hour))
	if got := f.run(); len(got) != 0 {
		t.Fatalf("領券時只剩 12 小時的券不提醒（%s）：%v", late.ID, voucherIDs(got))
	}
}

func TestExpiryReminderSkipsVouchersThatAlreadyExpired(t *testing.T) {
	t.Parallel()
	f := expirySetup(t, 0, nil)
	c := f.campaign(endAt(base.Add(6 * time.Hour)))
	f.claim(c, buyer1)
	f.setNow(base.Add(7 * time.Hour))
	if got := f.run(); len(got) != 0 {
		t.Fatalf("已過期的券不提醒：%v", voucherIDs(got))
	}
}

// 批次小於券數：一次 Run 處理完所有批。
func TestExpiryReminderProcessesEveryBatch(t *testing.T) {
	t.Parallel()
	f := expirySetup(t, 2, nil)
	c := f.campaign(endAt(base.Add(30 * time.Hour)))
	var all []coupon.Voucher
	for i := range 5 {
		all = append(all, f.claim(c, 500+int64(i)))
	}
	f.setNow(base.Add(7 * time.Hour))
	sameIDs(t, "五張分三批", f.run(), all...)
	if again := f.run(); len(again) != 0 {
		t.Fatalf("重跑不該再發：%v", voucherIDs(again))
	}
}

// 一張券的 handler 失敗不擋其他券，失敗的券不留提醒紀錄、下一輪重試。
func TestExpiryReminderIsolatesAFailingVoucher(t *testing.T) {
	t.Parallel()
	var bad atomic.Pointer[uuid.UUID]
	f := expirySetup(t, 0, func(id uuid.UUID) error {
		if b := bad.Load(); b != nil && *b == id {
			return errors.New("通知失敗")
		}
		return nil
	})
	c := f.campaign(endAt(base.Add(30 * time.Hour)))
	v1, v2, v3 := f.claim(c, buyer1), f.claim(c, buyer2), f.claim(c, buyer3)
	vs := []coupon.Voucher{v1, v2, v3}
	slices.SortFunc(vs, func(a, b coupon.Voucher) int { return a.ID.Compare(b.ID) })
	poison := vs[0].ID // 排在最前面，後面的券要不被它擋住
	bad.Store(&poison)
	f.setNow(base.Add(7 * time.Hour))
	got, err := f.runErr()
	if err == nil {
		t.Fatal("有券失敗，Run 要回錯誤讓失敗被看見")
	}
	sameIDs(t, "其他券照發", got, vs[1], vs[2])
	if f.reminders() != 2 {
		t.Fatalf("提醒紀錄 = %d，失敗的券不該有（要 2）", f.reminders())
	}
	bad.Store(nil)
	got, err = f.runErr()
	if err != nil {
		t.Fatal(err)
	}
	sameIDs(t, "下一輪重試失敗的券", got, vs[0])
}

// 沒有具名 handler 時失敗關閉：別的訂閱者不算，不寫提醒紀錄。
func TestExpiryReminderFailsClosedWithoutTheNamedSubscriber(t *testing.T) {
	t.Parallel()
	f := expirySetup(t, 0, nil)
	bus := events.NewBus()
	var seen atomic.Int32
	events.Subscribe(bus, func(context.Context, coupon.VoucherExpiring) error { seen.Add(1); return nil })
	f.module = f.newModule(bus, 0)
	c := f.campaign(endAt(base.Add(30 * time.Hour)))
	f.claim(c, buyer1)
	f.setNow(base.Add(7 * time.Hour))
	_, err := f.runErr()
	if !errors.Is(err, events.ErrNoSubscriber) {
		t.Fatalf("要回 ErrNoSubscriber，實際 %v", err)
	}
	if f.reminders() != 0 || seen.Load() != 0 {
		t.Fatalf("不該寫紀錄或發布：紀錄 %d、別的訂閱者收到 %d", f.reminders(), seen.Load())
	}
}

func TestExpiryReminderRequiresEvents(t *testing.T) {
	t.Parallel()
	f := setup(t)
	if _, err := f.m.RunExpiryReminder(context.Background()); err == nil {
		t.Fatal("沒給 Events，RunExpiryReminder 要回錯誤")
	}
	for name, fn := range map[string]func(){
		"AddWorkers":   func() { f.m.AddWorkers(river.NewWorkers()) },
		"PeriodicJobs": func() { f.m.PeriodicJobs() },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("沒給 Events 時 %s 應該 panic", name)
				}
			}()
			fn()
		}()
	}
}

func TestModuleRegistersTheReminderJobWhenEventsAreGiven(t *testing.T) {
	t.Parallel()
	f := expirySetup(t, 0, nil)
	f.module.AddWorkers(river.NewWorkers())
	if len(f.module.PeriodicJobs()) != 1 {
		t.Fatal("要有一個排程")
	}
}
