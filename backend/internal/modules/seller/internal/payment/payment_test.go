package payment

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

func details(ds ...StatusDetail) []StatusDetail { return ds }

func seller(reason string) StatusDetail {
	return StatusDetail{Resolution: ResolutionSellerAction, Reason: reason}
}

func platform(reason string) StatusDetail {
	return StatusDetail{Resolution: ResolutionPlatformAction, Reason: reason}
}

// StatusOf 逐列對應 D13 修訂的對應表；原因碼是導出狀態的那一筆細節的原因碼。
func TestStatusOfFollowsTheDecisionTable(t *testing.T) {
	for name, tc := range map[string]struct {
		snap       Snapshot
		want       model.PaymentAccountStatus
		wantReason string
		wantWarn   bool
	}{
		"active":         {Snapshot{Transfers: CapabilityActive}, model.PaymentEnabled, "", false},
		"active 忽略殘留的細節": {Snapshot{Transfers: CapabilityActive, Details: details(platform("x"))}, model.PaymentEnabled, "", false},
		"新帳戶：restricted 加 seller_action":     {Snapshot{Transfers: CapabilityRestricted, Details: details(seller("routine_onboarding"))}, model.PaymentActionRequired, "routine_onboarding", false},
		"pending 但有 seller_action":           {Snapshot{Transfers: CapabilityPending, Details: details(seller("verification_failed"))}, model.PaymentActionRequired, "verification_failed", false},
		"pending 沒有細節":                       {Snapshot{Transfers: CapabilityPending}, model.PaymentUnderReview, "", false},
		"pending 只有平台行動的細節":                  {Snapshot{Transfers: CapabilityPending, Details: details(platform("in_review"))}, model.PaymentUnderReview, "in_review", false},
		"rejected":                           {Snapshot{Transfers: CapabilityRejected, Details: details(platform("rejected_fraud"))}, model.PaymentDisabled, "rejected_fraud", false},
		"rejected 沒有細節，原因碼用狀態名":              {Snapshot{Transfers: CapabilityRejected}, model.PaymentDisabled, "rejected", false},
		"unsupported":                        {Snapshot{Transfers: CapabilityUnsupported}, model.PaymentDisabled, "unsupported", false},
		"restricted 加 platform_action":       {Snapshot{Transfers: CapabilityRestricted, Details: details(platform("contact_stripe_reason"))}, model.PaymentDisabled, "contact_stripe_reason", false},
		"restricted 加 none":                  {Snapshot{Transfers: CapabilityRestricted, Details: details(StatusDetail{Resolution: ResolutionNone, Reason: "no_fix"})}, model.PaymentDisabled, "no_fix", false},
		"restricted 但沒有細節":                   {Snapshot{Transfers: CapabilityRestricted}, model.PaymentUnderReview, "", true},
		"restricted 只有不認得的行動方":               {Snapshot{Transfers: CapabilityRestricted, Details: details(StatusDetail{Resolution: ResolutionUnknown, Reason: "new_kind"})}, model.PaymentUnderReview, "new_kind", true},
		"未知的轉帳能力狀態":                          {Snapshot{Transfers: CapabilityUnknown}, model.PaymentUnderReview, "", true},
		"空的轉帳能力狀態":                           {Snapshot{}, model.PaymentUnderReview, "", true},
		"pending 有不認得的行動方":                   {Snapshot{Transfers: CapabilityPending, Details: details(StatusDetail{Resolution: ResolutionUnknown})}, model.PaymentUnderReview, "", true},
		"多筆：賣家能行動的優先於平台處理的":                  {Snapshot{Transfers: CapabilityRestricted, Details: details(platform("p"), seller("s"))}, model.PaymentActionRequired, "s", false},
		"多筆：平台處理的優先於不認得的":                    {Snapshot{Transfers: CapabilityRestricted, Details: details(StatusDetail{Resolution: ResolutionUnknown, Reason: "u"}, platform("p"))}, model.PaymentDisabled, "p", false},
		"多筆：多個賣家行動取第一筆的原因碼":                  {Snapshot{Transfers: CapabilityRestricted, Details: details(seller("a"), seller("b"))}, model.PaymentActionRequired, "a", false},
		"原因碼超過上限時截斷":                         {Snapshot{Transfers: CapabilityRestricted, Details: details(platform(strings.Repeat("字", 250)))}, model.PaymentDisabled, strings.Repeat("字", maxReasonLen), false},
		"轉帳能力 active 與撥款能力無關：撥款受限也是 ENABLED": {Snapshot{Transfers: CapabilityActive, PayoutsActive: false}, model.PaymentEnabled, "", false},
	} {
		var buf bytes.Buffer
		got, reason := StatusOf(slog.New(slog.NewTextHandler(&buf, nil)), tc.snap)
		if got != tc.want || reason != tc.wantReason {
			t.Errorf("%s：%s %q，want %s %q", name, got, reason, tc.want, tc.wantReason)
		}
		if warned := strings.Contains(buf.String(), "level=WARN"); warned != tc.wantWarn {
			t.Errorf("%s：警示 log=%v，want %v", name, warned, tc.wantWarn)
		}
	}
}

// 每個狀態都被某個快照推導得到（NOT_CREATED 除外：沒有外部帳戶就沒有快照），狀態值域與 model 一致。
func TestEveryStatusButNotCreatedIsReachableFromASnapshot(t *testing.T) {
	reached := map[model.PaymentAccountStatus]bool{}
	for _, s := range []Snapshot{
		{}, {Transfers: CapabilityActive}, {Transfers: CapabilityRestricted, Details: details(seller("x"))}, {Transfers: CapabilityRejected},
	} {
		st, _ := StatusOf(slog.New(slog.DiscardHandler), s)
		reached[st] = true
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

func TestOnlyTheTwoOpenStatusesArePollable(t *testing.T) {
	for _, st := range model.PaymentAccountStatusValues() {
		want := st == model.PaymentUnderReview || st == model.PaymentActionRequired
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
