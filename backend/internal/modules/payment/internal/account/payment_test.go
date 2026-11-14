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
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
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
	a, err := f.AcquireAccount(ctx, AcquireInput{StoreID: 1, IdempotencyKey: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := f.AcquireAccount(ctx, AcquireInput{StoreID: 1, IdempotencyKey: "1"}); b != a {
		t.Fatalf("同一個冪等鍵建出另一個帳戶：%s %s", a, b)
	}
	if c, _ := f.AcquireAccount(ctx, AcquireInput{StoreID: 2, IdempotencyKey: "2"}); c == a {
		t.Fatal("不同的冪等鍵得到同一個帳戶")
	}
	if len(f.Violations()) != 0 {
		t.Fatalf("violations=%v", f.Violations())
	}
	if _, err := f.AcquireAccount(context.Background(), AcquireInput{StoreID: 3, IdempotencyKey: "3"}); err != nil || len(f.Violations()) != 1 {
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

func (activeOwners) OwnerContactEmail(context.Context, int64) (string, bool, error) {
	return "owner@example.com", true, nil
}

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

// 清理一列的處理方式（D59）：從沒呼叫過 provider 的列（沒有「已呼叫」時刻）直接刪，不需要 provider；呼叫過的列問不到 provider（不同或沒有設定）時不動它；
// 有外部 ID 且 provider 相同就關閉；沒有外部 ID 時 UnresolvedDeadline（7 天）內以同一把鍵重送，超過進終局（D63，不列出）。
// mutation：不看「已呼叫」時刻、沒有外部 ID 一律直接刪 → 「呼叫過、沒有外部 ID」的案例失敗；超過 7 天仍重送 → 逾期案例失敗；provider 不符也走關閉 → defer 案例失敗。
func TestPlanPurgeDeletesNeverCalledRowsAndOtherwiseClosesReplaysBlocksOrDefers(t *testing.T) {
	external := "acct_1"
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	fresh := now.Add(-UnresolvedDeadline + time.Hour)
	old := now.Add(-UnresolvedDeadline - time.Hour)
	for name, tc := range map[string]struct {
		row      store.PaymentAccount
		provider string
		want     purgePlan
	}{
		"從沒呼叫、provider 相同":             {store.PaymentAccount{Provider: "fake"}, "fake", purgeDelete},
		"從沒呼叫、沒有設定 provider":           {store.PaymentAccount{Provider: "stripe"}, "", purgeDelete},
		"從沒呼叫、provider 不同":             {store.PaymentAccount{Provider: "stripe"}, "fake", purgeDelete},
		"呼叫過、沒有 ID、7 天內、provider 相同":   {store.PaymentAccount{Provider: "fake", CreateCalledAt: &fresh}, "fake", purgeReplay},
		"呼叫過、沒有 ID、超過 7 天、provider 相同": {store.PaymentAccount{Provider: "fake", CreateCalledAt: &old}, "fake", purgeUnresolved},
		"呼叫過、沒有 ID、沒有設定 provider":      {store.PaymentAccount{Provider: "stripe", CreateCalledAt: &fresh}, "", purgeDefer},
		"呼叫過、沒有 ID、provider 不同":        {store.PaymentAccount{Provider: "stripe", CreateCalledAt: &old}, "fake", purgeDefer},
		"有外部 ID、provider 相同":           {store.PaymentAccount{Provider: "fake", ExternalAccountID: &external, CreateCalledAt: &old}, "fake", purgeClose},
		"有外部 ID、provider 不同":           {store.PaymentAccount{Provider: "stripe", ExternalAccountID: &external, CreateCalledAt: &fresh}, "fake", purgeDefer},
		"有外部 ID、沒有設定 provider":         {store.PaymentAccount{Provider: "stripe", ExternalAccountID: &external, CreateCalledAt: &fresh}, "", purgeDefer},
	} {
		if got := planPurge(tc.row, tc.provider, now); got != tc.want {
			t.Errorf("%s：planPurge=%d，want %d", name, got, tc.want)
		}
	}
}

// 刪列前在鎖住的列上再檢查：計畫是直接刪（沒有外部 ID）時，列上不能已經有外部 ID、「已呼叫」的時刻也不能變（並行的建立剛呼叫過 provider），否則回傳錯誤讓 job 重試並重新決定；
// 計畫是關閉成功後刪時，列上的外部 ID 要還是關閉的那一個。
// mutation：沒有外部 ID 的計畫不檢查鎖住的列 → 「並行寫入外部 ID」「並行呼叫過」案例失敗。
func TestDeleteRowRecheckAfterLockRefusesARowThatChanged(t *testing.T) {
	a, b := "acct_a", "acct_b"
	t0 := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	for name, tc := range map[string]struct {
		locked, plan store.PaymentAccount
		planned      *string
		wantOK       bool
		wantFail     bool
	}{
		"直接刪、列沒變":        {store.PaymentAccount{}, store.PaymentAccount{}, nil, true, false},
		"直接刪、並行寫入外部 ID":  {store.PaymentAccount{ExternalAccountID: &a, CreateCalledAt: &t0}, store.PaymentAccount{CreateCalledAt: &t0}, nil, false, true},
		"直接刪、並行呼叫過":      {store.PaymentAccount{CreateCalledAt: &t0}, store.PaymentAccount{}, nil, false, true},
		"直接刪、已呼叫的時刻換了":   {store.PaymentAccount{CreateCalledAt: &t1}, store.PaymentAccount{CreateCalledAt: &t0}, nil, false, true},
		"對帳沒有帳戶後刪、列沒變":   {store.PaymentAccount{CreateCalledAt: &t0}, store.PaymentAccount{CreateCalledAt: &t0}, nil, true, false},
		"關閉後刪、外部 ID 相同":  {store.PaymentAccount{ExternalAccountID: &a}, store.PaymentAccount{}, &a, true, false},
		"關閉後刪、外部 ID 不同":  {store.PaymentAccount{ExternalAccountID: &b}, store.PaymentAccount{}, &a, false, false},
		"關閉後刪、外部 ID 不見了": {store.PaymentAccount{}, store.PaymentAccount{}, &a, false, false},
	} {
		ok, err := lockedRowMayBeDeleted(tc.locked, tc.plan, tc.planned)
		if ok != tc.wantOK || (err != nil) != tc.wantFail {
			t.Errorf("%s：ok=%v err=%v，want ok=%v fail=%v", name, ok, err, tc.wantOK, tc.wantFail)
		}
	}
}

// 建立（Ensure 與 RequestAccount 排 job）只對還沒完成建立（第二步成功）、沒有建立失敗、沒有等待清理的列進行；有外部 ID 但第二步沒成功的半建立列照樣要建立。
// mutation：改看外部 ID → 「半建立」案例失敗；不看 PurgePendingAt → 已標記的案例失敗。
func TestCreateIsWantedUntilTheSecondStepSucceeds(t *testing.T) {
	a := "acct_a"
	now := time.Now()
	for name, tc := range map[string]struct {
		row  store.PaymentAccount
		want bool
	}{
		"建立中的列": {store.PaymentAccount{}, true},
		"半建立（有 ID、第二步沒成功）": {store.PaymentAccount{ExternalAccountID: &a, CreateCalledAt: &now}, true},
		"已完成建立":       {store.PaymentAccount{ExternalAccountID: &a, CreateCalledAt: &now, CreateCompletedAt: &now}, false},
		"建立失敗":        {store.PaymentAccount{CreateFailedAt: &now}, false},
		"已標記、沒有外部 ID": {store.PaymentAccount{PurgePendingAt: &now}, false},
		"已標記、半建立":     {store.PaymentAccount{PurgePendingAt: &now, ExternalAccountID: &a, CreateCalledAt: &now}, false},
	} {
		if got := createWanted(tc.row); got != tc.want {
			t.Errorf("%s：createWanted=%v，want %v", name, got, tc.want)
		}
	}
}

// RequestAccount 讀到已標記等待清理的列回 FailedPrecondition 且不排建立 job；其餘列依 createWanted 決定。
// mutation：已標記時回 nil → 已標記的案例失敗。
func TestRequestOutcomeRejectsAMarkedRow(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := "acct_1"
	for name, tc := range map[string]struct {
		row     store.PaymentAccount
		enqueue bool
		fail    bool
	}{
		"沒有外部 ID":     {store.PaymentAccount{}, true, false},
		"已完成建立":       {store.PaymentAccount{ExternalAccountID: &a, CreateCompletedAt: &now}, false, false},
		"半建立（有 ID）":   {store.PaymentAccount{ExternalAccountID: &a}, true, false},
		"建立失敗":        {store.PaymentAccount{CreateFailedAt: &now}, false, false},
		"已標記、沒有外部 ID": {store.PaymentAccount{PurgePendingAt: &now}, false, true},
		"已標記、有外部 ID":  {store.PaymentAccount{PurgePendingAt: &now, ExternalAccountID: &a}, false, true},
	} {
		enqueue, err := requestOutcome(tc.row)
		if tc.fail != (apperr.KindOf(err) == apperr.KindFailedPrecondition) || (!tc.fail && err != nil) || enqueue != tc.enqueue {
			t.Errorf("%s：enqueue=%v err=%v", name, enqueue, err)
		}
	}
}

// 結果不明的列「呼叫過、沒有外部 ID、還沒完成建立」超過 UnresolvedDeadline 才算逾期（D63，自動的路徑不再重送）：恰在期限上不算；有外部 ID、已完成建立、從沒呼叫的列都不算。
// 對外的 CreateUnresolved 看冪等鍵保存期（IdempotencyRetention）：必須已記建立失敗、沒有等待清理、而且超過保存期；7 天到 30 天之間為假（管理員重試以同一把鍵重送，不需要確認）。
// mutation：把 > 改成 >= → 「恰在期限上」案例失敗；不看外部 ID → 「有外部 ID」案例失敗；CreateUnresolved 改看 UnresolvedDeadline → 「超過 7 天、保存期內」案例失敗；不看建立失敗 → 「還沒記失敗」案例失敗。
func TestUnresolvedExpiredOnlyForCalledRowsWithoutAnIDPastTheDeadline(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	id := "acct_1"
	for name, tc := range map[string]struct {
		row                    store.PaymentAccount
		wantExpired, wantModel bool
	}{
		"從沒呼叫":        {store.PaymentAccount{}, false, false},
		"期限內":         {store.PaymentAccount{CreateCalledAt: at(UnresolvedDeadline - time.Second), CreateFailedAt: &now}, false, false},
		"恰在期限上":       {store.PaymentAccount{CreateCalledAt: at(UnresolvedDeadline), CreateFailedAt: &now}, false, false},
		"超過、還沒記失敗":    {store.PaymentAccount{CreateCalledAt: at(UnresolvedDeadline + time.Second)}, true, false},
		"超過 7 天、保存期內": {store.PaymentAccount{CreateCalledAt: at(UnresolvedDeadline + time.Second), CreateFailedAt: &now}, true, false},
		"恰在保存期上":      {store.PaymentAccount{CreateCalledAt: at(IdempotencyRetention), CreateFailedAt: &now}, true, false},
		"超過保存期、已記失敗":  {store.PaymentAccount{CreateCalledAt: at(IdempotencyRetention + time.Second), CreateFailedAt: &now}, true, true},
		"超過保存期、還沒記失敗": {store.PaymentAccount{CreateCalledAt: at(IdempotencyRetention + time.Second)}, true, false},
		"超過、有外部 ID":   {store.PaymentAccount{CreateCalledAt: at(UnresolvedDeadline + time.Second), ExternalAccountID: &id, CreateFailedAt: &now}, false, false},
		"超過、已完成建立":    {store.PaymentAccount{CreateCalledAt: at(UnresolvedDeadline + time.Second), ExternalAccountID: &id, CreateCompletedAt: &now}, false, false},
		"超過保存期、等待清理":  {store.PaymentAccount{CreateCalledAt: at(IdempotencyRetention + time.Second), CreateFailedAt: &now, PurgePendingAt: &now}, true, false},
	} {
		if got := unresolvedExpired(tc.row, now); got != tc.wantExpired {
			t.Errorf("%s：unresolvedExpired=%v，want %v", name, got, tc.wantExpired)
		}
		if got := ToModel(tc.row, now).CreateUnresolved; got != tc.wantModel {
			t.Errorf("%s：CreateUnresolved=%v，want %v", name, got, tc.wantModel)
		}
	}
}
