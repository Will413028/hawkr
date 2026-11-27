package write

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/textfilter"
)

const (
	buyerID   = int64(11)
	storeID   = int64(22)
	buyerAcct = int64(1001)
	storeAcct = int64(2001)
)

type fakeBuyers struct {
	blocking    bool
	blockingErr error
	profileErr  error
	accountErr  error
	account     int64
}

func (f *fakeBuyers) AccountIDByBuyerID(context.Context, int64) (int64, error) {
	return f.account, f.accountErr
}

func (f *fakeBuyers) ProfileByBuyerID(context.Context, int64) (buyer.Profile, error) {
	return buyer.Profile{}, f.profileErr
}

func (f *fakeBuyers) LockNotClosed(context.Context, int64) error { return nil }

func (f *fakeBuyers) IsBlockingStore(context.Context, int64, int64) (bool, error) {
	return f.blocking, f.blockingErr
}

type fakeStores struct {
	store       seller.Store
	err         error
	blocks      bool
	blocksErr   error
	storeCalled int
	blocksCalls int
}

func (f *fakeStores) Store(context.Context, int64) (seller.Store, error) {
	f.storeCalled++
	return f.store, f.err
}

func (f *fakeStores) LockNotRetired(context.Context, int64) error { return nil }

func (f *fakeStores) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) {
	f.blocksCalls++
	return f.blocks, f.blocksErr
}

func (f *fakeStores) StoresBlockingBuyer(context.Context, int64, []int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

func (f *fakeStores) BuyersBlockedByStore(context.Context, int64, []int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

type fakeOrders struct {
	open  bool
	err   error
	calls int
}

func (o *fakeOrders) OpenOrderBetween(context.Context, int64, int64) (bool, error) {
	o.calls++
	return o.open, o.err
}

type fixture struct {
	s       *Service
	buyers  *fakeBuyers
	stores  *fakeStores
	orders  *fakeOrders
	gone    map[string]bool
	goneErr error
}

func activeStore() seller.Store {
	return seller.Store{
		StoreID: storeID, AccountID: storeAcct, Status: seller.Active,
		Sanctions: seller.NoSanctions(), Conditions: seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true},
	}
}

func newFixture() *fixture {
	f := &fixture{
		buyers: &fakeBuyers{account: buyerAcct}, stores: &fakeStores{store: activeStore()}, orders: &fakeOrders{},
		gone: map[string]bool{},
	}
	f.s = New(Deps{Buyers: f.buyers, Stores: f.stores, Orders: f.orders, Now: time.Now, NewID: uuid.New})
	f.s.inactive = func(_ context.Context, kind model.PartyKind, id int64) (bool, error) {
		return f.gone[string(kind)+":"+itoa(id)], f.goneErr
	}
	return f
}

func itoa(v int64) string {
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	var b []byte
	for ; v > 0; v /= 10 {
		b = append([]byte{digits[v%10]}, b...)
	}
	return string(b)
}

func (f *fixture) sender(kind model.PartyKind) model.Party {
	if kind == model.PartyBuyer {
		return model.Party{Kind: model.PartyBuyer, BuyerID: buyerID}
	}
	st := f.stores.store
	return model.Party{Kind: model.PartyStore, Store: &st}
}

func (f *fixture) input(kind model.PartyKind) model.SendInput {
	in := model.SendInput{Sender: f.sender(kind), Type: model.TypeText, Text: "你好", RequestID: uuid.NewV4()}
	if kind == model.PartyBuyer {
		in.CounterpartID = storeID
	} else {
		in.CounterpartID = buyerID
	}
	return in
}

func (f *fixture) check(kind model.PartyKind, mutate func(*model.SendInput)) error {
	in := f.input(kind)
	if mutate != nil {
		mutate(&in)
	}
	body, err := f.s.checkInput("zh-TW", &in)
	if err != nil {
		return err
	}
	t, _, err := f.s.resolve(context.Background(), "zh-TW", in)
	if err != nil {
		return err
	}
	return f.s.guard(context.Background(), "zh-TW", in.Sender, t, body)
}

var kinds = []model.PartyKind{model.PartyBuyer, model.PartyStore}

func wantKind(t *testing.T, err error, kind apperr.Kind, reason model.Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("要回 %v/%q，實際成功", kind, reason)
	}
	if got := apperr.KindOf(err); got != kind || model.ReasonOf(err) != reason {
		t.Fatalf("got %v/%q（%v），want %v/%q", got, model.ReasonOf(err), err, kind, reason)
	}
}

// G3 封鎖的表格：寄件人 × 封鎖方向 × 進行中的訂單。無封鎖時放行而且不碰 Orders；有封鎖且無進行中時 CHAT_BLOCKED；
// 有封鎖且有進行中時放行；查詢失敗時 Unavailable，不放行。
func TestBlockGuardTable(t *testing.T) {
	type block struct {
		name          string
		buyerBlocks   bool
		storeBlocks   bool
		wantOrderCall bool
	}
	blocks := []block{{"none", false, false, false}, {"buyer blocks store", true, false, true}, {"store blocks buyer", false, true, true}}
	type open struct {
		name string
		open bool
		err  error
	}
	opens := []open{{"no open order", false, nil}, {"open order", true, nil}, {"lookup fails", false, errors.New("boom")}}
	for _, kind := range kinds {
		for _, b := range blocks {
			for _, o := range opens {
				t.Run(string(kind)+"/"+b.name+"/"+o.name, func(t *testing.T) {
					f := newFixture()
					f.buyers.blocking, f.stores.blocks = b.buyerBlocks, b.storeBlocks
					f.orders.open, f.orders.err = o.open, o.err
					err := f.check(kind, nil)
					switch {
					case !b.buyerBlocks && !b.storeBlocks:
						if err != nil || f.orders.calls != 0 {
							t.Fatalf("無封鎖：err=%v，Orders 被查 %d 次（不該查）", err, f.orders.calls)
						}
					case o.err != nil:
						wantKind(t, err, apperr.KindUnavailable, "")
					case o.open:
						if err != nil {
							t.Fatalf("有進行中的訂單要放行：%v", err)
						}
					default:
						wantKind(t, err, apperr.KindFailedPrecondition, model.ReasonChatBlocked)
					}
					if b.wantOrderCall && f.orders.calls != 1 {
						t.Fatalf("Orders 被查 %d 次，預期 1 次", f.orders.calls)
					}
				})
			}
		}
	}
}

func TestBlockLookupErrorsAreUnavailableNotAllowed(t *testing.T) {
	for _, kind := range kinds {
		f := newFixture()
		f.buyers.blockingErr = errors.New("boom")
		wantKind(t, f.check(kind, nil), apperr.KindUnavailable, "")
		f = newFixture()
		f.stores.blocksErr = errors.New("boom")
		wantKind(t, f.check(kind, nil), apperr.KindUnavailable, "")
	}
}

// G1 停權：只擋寄件人是店家；有進行中的訂單或售後時放行；查詢失敗 Unavailable。買家寄給停權的店家不受影響。
func TestSuspensionGuard(t *testing.T) {
	suspend := func(f *fixture) {
		f.stores.store.Sanctions = seller.SanctionedBy(seller.EffectiveMeasure{})
	}
	t.Run("store suspended, no open order", func(t *testing.T) {
		f := newFixture()
		suspend(f)
		wantKind(t, f.check(model.PartyStore, nil), apperr.KindPermissionDenied, model.ReasonSellerSuspended)
	})
	t.Run("store suspended, open order", func(t *testing.T) {
		f := newFixture()
		suspend(f)
		f.orders.open = true
		if err := f.check(model.PartyStore, nil); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("store suspended, lookup fails", func(t *testing.T) {
		f := newFixture()
		suspend(f)
		f.orders.err = errors.New("boom")
		wantKind(t, f.check(model.PartyStore, nil), apperr.KindUnavailable, "")
	})
	t.Run("buyer to suspended store", func(t *testing.T) {
		f := newFixture()
		suspend(f)
		if err := f.check(model.PartyBuyer, nil); err != nil || f.orders.calls != 0 {
			t.Fatalf("err=%v orders=%d", err, f.orders.calls)
		}
	})
	t.Run("not suspended never asks orders", func(t *testing.T) {
		f := newFixture()
		if err := f.check(model.PartyStore, nil); err != nil || f.orders.calls != 0 {
			t.Fatalf("err=%v orders=%d", err, f.orders.calls)
		}
	})
	t.Run("suspension and block share one lookup", func(t *testing.T) {
		f := newFixture()
		suspend(f)
		f.stores.blocks = true
		f.orders.open = true
		if err := f.check(model.PartyStore, nil); err != nil || f.orders.calls != 1 {
			t.Fatalf("err=%v orders=%d", err, f.orders.calls)
		}
	})
}

// G2 停業：只有 CLOSED 算停業（雙向都擋）；等待刪除雙向都放行。
func TestClosedGuard(t *testing.T) {
	status := func(s seller.Status) func(*fixture) {
		return func(f *fixture) {
			f.stores.store.Status = s
			f.stores.store.Conditions = seller.Conditions{Derived: true, Active: s != seller.Closed, NotPendingDeletion: s != seller.PendingDeletion}
		}
	}
	cases := []struct {
		name     string
		mut      func(*fixture)
		wantPass bool
	}{
		{"active", status(seller.Active), true},
		{"pending deletion", status(seller.PendingDeletion), true},
		{"closed", status(seller.Closed), false},
	}
	for _, kind := range kinds {
		for _, tc := range cases {
			t.Run(string(kind)+"/"+tc.name, func(t *testing.T) {
				f := newFixture()
				tc.mut(f)
				err := f.check(kind, nil)
				if tc.wantPass {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				wantKind(t, err, apperr.KindFailedPrecondition, model.ReasonStoreClosed)
			})
		}
	}
	t.Run("conditions not derived fails closed", func(t *testing.T) {
		f := newFixture()
		f.stores.store.Conditions = seller.Conditions{}
		wantKind(t, f.check(model.PartyBuyer, nil), apperr.KindUnavailable, "")
	})
}

// 店家寄首則訊息放行。
func TestStoreMayStartAConversation(t *testing.T) {
	f := newFixture()
	if err := f.check(model.PartyStore, nil); err != nil {
		t.Fatal(err)
	}
}

func TestReceiverDeletedGuard(t *testing.T) {
	f := newFixture()
	f.gone["STORE:22"] = true
	wantKind(t, f.check(model.PartyBuyer, nil), apperr.KindFailedPrecondition, model.ReasonReceiverDeleted)
	f = newFixture()
	f.gone["BUYER:11"] = true
	wantKind(t, f.check(model.PartyStore, nil), apperr.KindFailedPrecondition, model.ReasonReceiverDeleted)
	f = newFixture()
	f.gone["BUYER:11"] = true
	if err := f.check(model.PartyBuyer, nil); err != nil {
		t.Fatal(err)
	}
	f = newFixture()
	f.goneErr = errors.New("boom")
	if err := f.check(model.PartyBuyer, nil); err == nil {
		t.Fatal("查詢失敗不能放行")
	}
}

func TestModerationGuardOnlyChecksText(t *testing.T) {
	f := newFixture()
	err := f.check(model.PartyBuyer, func(in *model.SendInput) { in.Text = "x" + textfilter.Words[0] + "y" })
	wantKind(t, err, apperr.KindInvalidInput, model.ReasonModerationRejected)
}

func TestSelfMessageIsRejected(t *testing.T) {
	for _, kind := range kinds {
		f := newFixture()
		f.buyers.account = storeAcct
		wantKind(t, f.check(kind, nil), apperr.KindInvalidInput, model.ReasonSelfMessageNotAllowed)
	}
}

func TestCounterpartLookups(t *testing.T) {
	f := newFixture()
	f.stores.err = apperr.NotFound("no store")
	wantKind(t, f.check(model.PartyBuyer, nil), apperr.KindNotFound, "")
	f = newFixture()
	f.stores.err = errors.New("boom")
	wantKind(t, f.check(model.PartyBuyer, nil), apperr.KindUnavailable, "")
	f = newFixture()
	f.buyers.profileErr = apperr.NotFound("no buyer")
	wantKind(t, f.check(model.PartyStore, nil), apperr.KindNotFound, "")
	f = newFixture()
	f.buyers.profileErr = errors.New("boom")
	wantKind(t, f.check(model.PartyStore, nil), apperr.KindUnavailable, "")
	f = newFixture()
	f.buyers.accountErr = errors.New("boom")
	wantKind(t, f.check(model.PartyBuyer, nil), apperr.KindUnavailable, "")
}

func TestInputValidation(t *testing.T) {
	f := newFixture()
	for name, mut := range map[string]func(*model.SendInput){
		"unknown type":         func(in *model.SendInput) { in.Type = "CARD" },
		"empty type":           func(in *model.SendInput) { in.Type = "" },
		"lowercase type":       func(in *model.SendInput) { in.Type = "text" },
		"blank text":           func(in *model.SendInput) { in.Text = " \n\t" },
		"too long":             func(in *model.SendInput) { in.Text = strings.Repeat("字", model.MaxTextRunes+1) },
		"invalid utf8":         func(in *model.SendInput) { in.Text = "a\xffb" },
		"nul byte":             func(in *model.SendInput) { in.Text = "a\x00b" },
		"image no token":       func(in *model.SendInput) { in.Type, in.Text, in.StagedToken = model.TypeImage, "", "  " },
		"no counterpart":       func(in *model.SendInput) { in.CounterpartID = 0 },
		"negative counterpart": func(in *model.SendInput) { in.CounterpartID = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			wantKind(t, f.check(model.PartyBuyer, mut), apperr.KindInvalidInput, "")
		})
	}
	if err := f.check(model.PartyBuyer, func(in *model.SendInput) { in.Text = "  " + strings.Repeat("字", model.MaxTextRunes) + "\n" }); err != nil {
		t.Fatalf("4096 個字要通過：%v", err)
	}
	in := f.input(model.PartyBuyer)
	in.Sender = model.Party{Kind: model.PartyStore}
	if _, err := f.s.checkInput("zh-TW", &in); err == nil || apperr.KindOf(err) == apperr.KindInvalidInput {
		t.Fatalf("store 沒有 Store：%v", err)
	}
	in.Sender = model.Party{}
	if _, err := f.s.checkInput("zh-TW", &in); err == nil {
		t.Fatal("空的寄件人要回錯誤")
	}
}

// RequestID 必填而且必須是 UUIDv4：零值、其他版本、其他 variant 都回 InvalidInput 指向 RequestID。
func TestCheckInputRequestID(t *testing.T) {
	v7 := uuid.NewV7()
	badVariant := uuid.NewV4()
	badVariant[8] = 0x00
	cases := []struct {
		name string
		id   uuid.UUID
		ok   bool
	}{
		{"v4", uuid.NewV4(), true},
		{"零值", uuid.UUID{}, false},
		{"v7", v7, false},
		{"variant 不是 RFC 9562", badVariant, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture()
			in := f.input(model.PartyBuyer)
			in.RequestID = c.id
			_, err := f.s.checkInput("zh-TW", &in)
			if c.ok {
				if err != nil {
					t.Fatalf("v4 應通過，實際 %v", err)
				}
				return
			}
			var ae *apperr.Error
			if apperr.KindOf(err) != apperr.KindInvalidInput || !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "RequestID" {
				t.Fatalf("要回 InvalidInput 指向 RequestID，實際 %v", err)
			}
		})
	}
}
