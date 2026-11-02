package payment

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// 狀態的推導順序：被拒絕或暫停 → 轉帳能力啟用 → 還沒送出資料 → 有待補的條件 → 審核中（Stripe 計畫 D13）。
func TestStatusOfFollowsTheDocumentedPrecedence(t *testing.T) {
	for name, tc := range map[string]struct {
		snap Snapshot
		want model.PaymentAccountStatus
	}{
		"剛建立":                {Snapshot{RequirementsDue: true}, model.PaymentOnboarding},
		"還沒送出資料，有待補":         {Snapshot{RequirementsDue: true, DetailsSubmitted: false}, model.PaymentOnboarding},
		"已送出，有待補":            {Snapshot{DetailsSubmitted: true, RequirementsDue: true}, model.PaymentActionRequired},
		"已送出，沒有待補，能力未啟用":     {Snapshot{DetailsSubmitted: true}, model.PaymentUnderReview},
		"轉帳能力啟用":             {Snapshot{DetailsSubmitted: true, TransfersActive: true}, model.PaymentEnabled},
		"啟用但又有新的待補條件":        {Snapshot{DetailsSubmitted: true, TransfersActive: true, RequirementsDue: true}, model.PaymentEnabled},
		"被拒絕，能力仍標示啟用":        {Snapshot{DetailsSubmitted: true, TransfersActive: true, DisabledReason: "rejected.fraud"}, model.PaymentDisabled},
		"被暫停，還沒送出資料":         {Snapshot{DisabledReason: "platform_paused"}, model.PaymentDisabled},
		"只有撥款能力，轉帳能力未啟用，已送出": {Snapshot{DetailsSubmitted: true, PayoutsActive: true}, model.PaymentUnderReview},
	} {
		if got := StatusOf(tc.snap); got != tc.want {
			t.Errorf("%s：%s，want %s", name, got, tc.want)
		}
	}
}

// 每個狀態都被某個快照推導得到（NOT_CREATED 除外：沒有外部帳戶就沒有快照），狀態值域與 model 一致。
func TestEveryStatusButNotCreatedIsReachableFromASnapshot(t *testing.T) {
	reached := map[model.PaymentAccountStatus]bool{}
	for _, s := range []Snapshot{
		{}, {DetailsSubmitted: true}, {DetailsSubmitted: true, RequirementsDue: true}, {TransfersActive: true}, {DisabledReason: "x"},
	} {
		reached[StatusOf(s)] = true
	}
	for _, st := range model.PaymentAccountStatusValues() {
		if st != model.PaymentNotCreated && !reached[st] {
			t.Errorf("%s 沒有任何快照推導得到", st)
		}
	}
	if reached[model.PaymentNotCreated] {
		t.Error("快照推導出 NOT_CREATED")
	}
}

func TestOnlyTheThreeOpenStatusesArePollable(t *testing.T) {
	for _, st := range model.PaymentAccountStatusValues() {
		want := st == model.PaymentOnboarding || st == model.PaymentUnderReview || st == model.PaymentActionRequired
		if st.Pollable() != want {
			t.Errorf("%s：Pollable=%v", st, st.Pollable())
		}
	}
}

func TestFakeCreatesOneAccountPerIdempotencyKeyAndRejectsUnsafeCalls(t *testing.T) {
	f := NewFake()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	a, err := f.EnsureAccount(ctx, EnsureInput{StoreID: 1, IdempotencyKey: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := f.EnsureAccount(ctx, EnsureInput{StoreID: 1, IdempotencyKey: "1"}); b != a {
		t.Fatalf("同一個冪等鍵建出另一個帳戶：%s %s", a, b)
	}
	if c, _ := f.EnsureAccount(ctx, EnsureInput{StoreID: 2, IdempotencyKey: "2"}); c == a {
		t.Fatal("不同的冪等鍵得到同一個帳戶")
	}
	if len(f.Violations()) != 0 {
		t.Fatalf("violations=%v", f.Violations())
	}
	if _, err := f.EnsureAccount(context.Background(), EnsureInput{StoreID: 3, IdempotencyKey: "3"}); err != nil || len(f.Violations()) != 1 {
		t.Fatalf("沒有 deadline 的呼叫沒有被記錄：%v", f.Violations())
	}
	if _, err := f.FetchAccount(ctx, "acct_unknown"); !errors.Is(err, ErrPermanent) {
		t.Fatalf("不認得的帳戶 err=%v", err)
	}
	h := http.Header{}
	if _, err := f.ParseEvent(h, []byte("acct")); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("沒有事件 ID err=%v", err)
	}
	h.Set(FakeEventHeader, "evt_1")
	if ev, err := f.ParseEvent(h, []byte(" acct ")); err != nil || ev.ID != "evt_1" || ev.ExternalAccountID != "acct" {
		t.Fatalf("event=%+v err=%v", ev, err)
	}
}
