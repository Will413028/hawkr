package account

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
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
		wantNote   StatusNote
	}{
		"active":         {Snapshot{Transfers: CapabilityActive}, model.PaymentEnabled, "", NoteNone},
		"active 忽略殘留的細節": {Snapshot{Transfers: CapabilityActive, Details: details(platform("x"))}, model.PaymentEnabled, "", NoteNone},
		"新帳戶：restricted 加 seller_action":     {Snapshot{Transfers: CapabilityRestricted, Details: details(seller("routine_onboarding"))}, model.PaymentActionRequired, "routine_onboarding", NoteNone},
		"pending 但有 seller_action":           {Snapshot{Transfers: CapabilityPending, Details: details(seller("verification_failed"))}, model.PaymentActionRequired, "verification_failed", NoteNone},
		"pending 沒有細節":                       {Snapshot{Transfers: CapabilityPending}, model.PaymentUnderReview, "", NoteNone},
		"pending 只有平台行動的細節":                  {Snapshot{Transfers: CapabilityPending, Details: details(platform("in_review"))}, model.PaymentUnderReview, "in_review", NoteNone},
		"rejected":                           {Snapshot{Transfers: CapabilityRejected, Details: details(platform("rejected_fraud"))}, model.PaymentDisabled, "rejected_fraud", NoteNone},
		"rejected 沒有細節，原因碼用狀態名":              {Snapshot{Transfers: CapabilityRejected}, model.PaymentDisabled, "rejected", NoteNone},
		"unsupported":                        {Snapshot{Transfers: CapabilityUnsupported}, model.PaymentDisabled, "unsupported", NoteNone},
		"restricted 加 platform_action":       {Snapshot{Transfers: CapabilityRestricted, Details: details(platform("contact_stripe_reason"))}, model.PaymentDisabled, "contact_stripe_reason", NoteNone},
		"restricted 加 none":                  {Snapshot{Transfers: CapabilityRestricted, Details: details(StatusDetail{Resolution: ResolutionNone, Reason: "no_fix"})}, model.PaymentDisabled, "no_fix", NoteNone},
		"restricted 但沒有細節":                   {Snapshot{Transfers: CapabilityRestricted}, model.PaymentUnderReview, "", NoteRestrictedWithoutDetail},
		"restricted 只有不認得的行動方":               {Snapshot{Transfers: CapabilityRestricted, Details: details(StatusDetail{Resolution: ResolutionUnknown, Reason: "new_kind"})}, model.PaymentUnderReview, "new_kind", NoteRestrictedWithoutDetail},
		"未知的轉帳能力狀態":                          {Snapshot{Transfers: CapabilityUnknown}, model.PaymentUnderReview, "", NoteUnknownState},
		"空的轉帳能力狀態":                           {Snapshot{}, model.PaymentUnderReview, "", NoteUnknownState},
		"pending 有不認得的行動方":                   {Snapshot{Transfers: CapabilityPending, Details: details(StatusDetail{Resolution: ResolutionUnknown})}, model.PaymentUnderReview, "", NoteUnknownResolution},
		"多筆：賣家能行動的優先於平台處理的":                  {Snapshot{Transfers: CapabilityRestricted, Details: details(platform("p"), seller("s"))}, model.PaymentActionRequired, "s", NoteNone},
		"多筆：平台處理的優先於不認得的":                    {Snapshot{Transfers: CapabilityRestricted, Details: details(StatusDetail{Resolution: ResolutionUnknown, Reason: "u"}, platform("p"))}, model.PaymentDisabled, "p", NoteNone},
		"多筆：多個賣家行動取第一筆的原因碼":                  {Snapshot{Transfers: CapabilityRestricted, Details: details(seller("a"), seller("b"))}, model.PaymentActionRequired, "a", NoteNone},
		"原因碼超過上限時截斷":                         {Snapshot{Transfers: CapabilityRestricted, Details: details(platform(strings.Repeat("字", 250)))}, model.PaymentDisabled, strings.Repeat("字", maxReasonLen), NoteNone},
		"轉帳能力 active 與撥款能力無關：撥款受限也是 ENABLED": {Snapshot{Transfers: CapabilityActive, PayoutsActive: false}, model.PaymentEnabled, "", NoteNone},
	} {
		got, reason, note := StatusOf(tc.snap)
		if got != tc.want || reason != tc.wantReason || note != tc.wantNote {
			t.Errorf("%s：%s %q %q，want %s %q %q", name, got, reason, note, tc.want, tc.wantReason, tc.wantNote)
		}
	}
}

// 每個狀態都被某個快照推導得到（NOT_CREATED 除外：沒有外部帳戶就沒有快照），狀態值域與 model 一致。
func TestEveryStatusButNotCreatedIsReachableFromASnapshot(t *testing.T) {
	reached := map[model.PaymentAccountStatus]bool{}
	for _, s := range []Snapshot{
		{}, {Transfers: CapabilityActive}, {Transfers: CapabilityRestricted, Details: details(seller("x"))}, {Transfers: CapabilityRejected},
	} {
		st, _, _ := StatusOf(s)
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

func TestCapabilityStateValidListsExactlyTheFiveKnownStates(t *testing.T) {
	for st, want := range map[CapabilityState]bool{
		CapabilityActive: true, CapabilityPending: true, CapabilityRestricted: true, CapabilityRejected: true, CapabilityUnsupported: true,
		CapabilityUnknown: false, "": false, "brand_new": false,
	} {
		if st.Valid() != want {
			t.Errorf("%q：Valid=%v", st, st.Valid())
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

type noopJobs struct{}

func (noopJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

type activeOwners struct{}

func (activeOwners) OwnerActive(context.Context, int64) (bool, error) { return true, nil }

// 設定了 provider 時，組裝缺 Owners 要在啟動就失敗，不能讓建立 job 不檢查擁有者是否營運中；沒有 provider（沒有設定收款開通）時不需要。設定了 provider 時
// 把事件處理登記到 inbox（同一個 provider 登記兩次是組裝錯誤）。
// mutation：New 不檢查 Owners → 第一段沒有 panic；New 不登記 inbox → 第二次 New 沒有 panic。
func TestNewRequiresOwnersAndRegistersTheProviderWithTheInbox(t *testing.T) {
	d := Deps{DB: &db.DB{}, Events: events.NewBus(), Jobs: noopJobs{}, Now: time.Now, Logger: slog.Default()}
	New(d)
	d.Inbox = inbox.New(inbox.Deps{DB: d.DB, Jobs: d.Jobs, Now: d.Now, Logger: d.Logger})
	d.Provider = NewFake()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("缺 Owners 沒有 panic")
			}
		}()
		New(d)
	}()
	d.Owners = activeOwners{}
	New(d)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("同一個 inbox 登記兩次 fake 沒有 panic")
			}
		}()
		New(d)
	}()
}
