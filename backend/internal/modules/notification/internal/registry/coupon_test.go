package registry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const couponVoucher = "0190a000-0000-7000-8000-0000000000d1"

const voucherExpiringParams = `{"event_id":"` + contentEvent + `","occurred_at":"2026-11-23T04:30:00Z","buyer_id":9,"voucher_id":"` + couponVoucher +
	`","campaign_id":"` + contentObj + `","expires_at":"2026-11-24T04:30:00Z"}`

func TestGoldenVoucherExpiringRendering(t *testing.T) {
	e, ok := Lookup("voucher_expiring")
	if !ok {
		t.Fatal("沒有型別 voucher_expiring")
	}
	cases := []struct {
		l           uitext.Locale
		title, body string
		link        string
	}{
		{uitext.ZhTW, "優惠券即將到期", "你有一張優惠券將於 2026/11/24 12:30 到期，把握時間使用。", "/account/coupons"},
		{uitext.EN, "Coupon expiring soon", "You have a coupon that expires on 2026/11/24 12:30. Use it before then.", "/en/account/coupons"},
	}
	for _, c := range cases {
		got, err := e.Render(c.l, []byte(voucherExpiringParams))
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != c.title || got.Body != c.body || got.DeepLink != c.link || got.AggregationKey != "" || got.Locale != c.l {
			t.Errorf("%s 的輸出 %+v", c.l, got)
		}
	}
}

func TestVoucherExpiringIdempotencyKeyIsTheVoucherID(t *testing.T) {
	e, _ := Lookup("voucher_expiring")
	other := `{"event_id":"` + contentObj + `","occurred_at":"2026-11-23T05:30:00Z","buyer_id":9,"voucher_id":"` + couponVoucher + `","campaign_id":"` + contentObj + `","expires_at":"2026-11-24T04:30:00Z"}`
	a, err := e.IdempotencyKey([]byte(voucherExpiringParams))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := e.IdempotencyKey([]byte(other))
	if a != b || a != "voucher_expiring:"+couponVoucher {
		t.Errorf("冪等鍵 %q、%q", a, b)
	}
}

type fakeValidity struct {
	available bool
	err       error
	gotBuyer  int64
	gotID     uuid.UUID
}

func (f *fakeValidity) VoucherAvailable(_ context.Context, buyerID int64, id uuid.UUID) (bool, error) {
	f.gotBuyer, f.gotID = buyerID, id
	return f.available, f.err
}

func TestVoucherExpiringStillValidAsksTheVoucherOfTheBuyer(t *testing.T) {
	e, _ := Lookup("voucher_expiring")
	for _, want := range []bool{true, false} {
		f := &fakeValidity{available: want}
		got, err := e.StillValid(context.Background(), f, json.RawMessage(voucherExpiringParams))
		if err != nil || got != want || f.gotBuyer != 9 || f.gotID.String() != couponVoucher {
			t.Errorf("StillValid = %v（%v），問了 %d %s", got, err, f.gotBuyer, f.gotID)
		}
	}
	boom := errors.New("boom")
	if _, err := e.StillValid(context.Background(), &fakeValidity{err: boom}, json.RawMessage(voucherExpiringParams)); !errors.Is(err, boom) {
		t.Errorf("讀取失敗要原樣回傳（暫時性），得到 %v", err)
	}
	if _, err := e.StillValid(context.Background(), &fakeValidity{}, json.RawMessage(`{`)); !errors.Is(err, ErrBadParams) {
		t.Errorf("壞參數要是永久錯誤，得到 %v", err)
	}
}

func TestTypesWithoutStillValidAreAlwaysValid(t *testing.T) {
	e, _ := Lookup("product_restocked")
	raw, _, _ := e.ExampleParams()
	if ok, err := e.StillValid(context.Background(), nil, raw); !ok || err != nil {
		t.Errorf("StillValid = %v（%v）", ok, err)
	}
}

func TestVoucherExpiringDeclaresItsExpiryAsTheNotAfterTime(t *testing.T) {
	e, _ := Lookup("voucher_expiring")
	got, ok, err := e.NotAfter(json.RawMessage(voucherExpiringParams))
	want := time.Date(2026, 11, 24, 4, 30, 0, 0, time.UTC)
	if err != nil || !ok || !got.Equal(want) {
		t.Errorf("NotAfter = %v、%v（%v），預期 %v", got, ok, err, want)
	}
	if _, _, err := e.NotAfter(json.RawMessage(`{`)); !errors.Is(err, ErrBadParams) {
		t.Errorf("壞參數要是永久錯誤，得到 %v", err)
	}
	if !e.Spec().Revalidates {
		t.Error("voucher_expiring 宣告了 StillValid，Spec.Revalidates 要是 true")
	}
}

func TestTypesWithoutNotAfterHaveNoDeadline(t *testing.T) {
	e, _ := Lookup("product_restocked")
	raw, _, _ := e.ExampleParams()
	if _, ok, err := e.NotAfter(raw); ok || err != nil {
		t.Errorf("NotAfter ok = %v（%v）", ok, err)
	}
	if e.Spec().Revalidates {
		t.Error("沒有 StillValid 的型別 Revalidates 要是 false")
	}
}
