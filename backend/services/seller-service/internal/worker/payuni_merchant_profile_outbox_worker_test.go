package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// 同一個 process 內 outbox.NewMetrics 用同名字註冊兩次會 panic（Prometheus
// duplicate registration），所以整包測試共用這一個。
var testPayuniProfileMetrics = outbox.NewMetrics("payuni_merchant_profile_test")

type fakeProfileUpdater struct {
	calls []*usecase.UpdateMerchantProfileRequest
	err   error
}

func (f *fakeProfileUpdater) UpdateMerchantProfile(_ context.Context, req *usecase.UpdateMerchantProfileRequest) (*usecase.UpdateMerchantProfileResult, error) {
	cp := *req
	f.calls = append(f.calls, &cp)
	if f.err != nil {
		return nil, f.err
	}
	return &usecase.UpdateMerchantProfileResult{MerchantProfileVersion: 1}, nil
}

type fakePayuniProfileOutboxRepo struct {
	pending    []*domain.PayuniMerchantProfileOutbox
	ClaimLost  bool
	ClaimErr   error
	ClaimCalls []int64
	doneIDs    map[int64]bool
	failedIDs  map[int64]string
	attempts   map[int64]int
}

func newFakePayuniProfileOutboxRepo(rows ...*domain.PayuniMerchantProfileOutbox) *fakePayuniProfileOutboxRepo {
	return &fakePayuniProfileOutboxRepo{
		pending:   rows,
		doneIDs:   map[int64]bool{},
		failedIDs: map[int64]string{},
		attempts:  map[int64]int{},
	}
}

func (r *fakePayuniProfileOutboxRepo) Insert(context.Context, *domain.PayuniMerchantProfileOutbox) error {
	return nil
}
func (r *fakePayuniProfileOutboxRepo) FetchPendingDue(context.Context, time.Time, int) ([]*domain.PayuniMerchantProfileOutbox, error) {
	return r.pending, nil
}
func (r *fakePayuniProfileOutboxRepo) Claim(_ context.Context, id int64, _, _ time.Time) (int64, error) {
	r.ClaimCalls = append(r.ClaimCalls, id)
	if r.ClaimErr != nil {
		return 0, r.ClaimErr
	}
	if r.ClaimLost {
		return 0, nil
	}
	return 1, nil
}
func (r *fakePayuniProfileOutboxRepo) UpdateAttempt(_ context.Context, id int64, attempt int, _ time.Time, _ string) error {
	r.attempts[id] = attempt
	return nil
}
func (r *fakePayuniProfileOutboxRepo) MarkDone(_ context.Context, id int64, _ time.Time) error {
	r.doneIDs[id] = true
	return nil
}
func (r *fakePayuniProfileOutboxRepo) MarkFailed(_ context.Context, id int64, lastErr string, _ time.Time) error {
	r.failedIDs[id] = lastErr
	return nil
}
func (r *fakePayuniProfileOutboxRepo) CountPending(context.Context) (int, error) {
	return len(r.pending), nil
}
func (r *fakePayuniProfileOutboxRepo) OldestPendingAge(context.Context, time.Time) (time.Duration, bool, error) {
	return 0, false, nil
}
func (r *fakePayuniProfileOutboxRepo) DeleteDoneBefore(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func mustRow(t *testing.T, id, storeID, accountID int64, merAddr, post string) *domain.PayuniMerchantProfileOutbox {
	t.Helper()
	row, err := domain.NewPayuniMerchantProfileOutboxRow(id, storeID, accountID, merAddr, post, time.Now().UTC())
	if err != nil {
		t.Fatalf("build row: %v", err)
	}
	return row
}

func versionLookupStub(v int64) merchantProfileVersionLookup {
	return func(context.Context, int64) (int64, error) { return v, nil }
}

// Idempotency key 必須是 outbox row id：重試時不能變，否則同一筆變動會被
// UpdateMerchantProfile 當成兩次不同的請求。反過來說，key 不變才讓「PayUni
// 已成功但 MarkDone 失敗」的下一輪走 SUCCEEDED replay 而不是再打一次 PayUni。
func TestPayuniMerchantProfileOutboxWorker_UsesRowIDAsIdempotencyKey(t *testing.T) {
	updater := &fakeProfileUpdater{}
	repo := newFakePayuniProfileOutboxRepo(
		mustRow(t, 4242, 202, 303, "台北市信義區松高路1號", "110"),
	)
	w := NewPayuniMerchantProfileOutboxWorker(repo, updater, versionLookupStub(7), testPayuniProfileMetrics)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(updater.calls) != 1 {
		t.Fatalf("UpdateMerchantProfile called %d times, want 1", len(updater.calls))
	}
	got := updater.calls[0]
	if got.IdempotencyKey != "4242" {
		t.Errorf("IdempotencyKey = %q, want \"4242\"（outbox row id）", got.IdempotencyKey)
	}
	if got.ExpectedVersion != 7 {
		t.Errorf("ExpectedVersion = %d, want 7（執行當下現讀，不是存進 payload）", got.ExpectedVersion)
	}
	if got.StoreID != 202 || got.AccountID != 303 {
		t.Errorf("ids = %d/%d, want 202/303", got.StoreID, got.AccountID)
	}
	if got.MerAddr != "台北市信義區松高路1號" || got.MerAddrPost != "110" {
		t.Errorf("address = %q/%q, want payload 裡的值", got.MerAddr, got.MerAddrPost)
	}
	// 非地址欄位留空是刻意的，兩端都把空字串當「不要動」：PayUni 的 wire struct
	// 這幾欄全帶 omitempty（pkg/payuni/client.go），本地回寫是
	// COALESCE(NULLIF(...), 既有值)（UpdatePayuniMerchantProfile）。填值反而會
	// 用商店資料頁的內容蓋掉 PayUni 上的聯絡資料。
	if got.ContactName != "" || got.ContactMobile != "" || got.ContactTel != "" || got.MerEmail != "" || got.MerTel != "" {
		t.Errorf("非地址欄位必須留空，否則會誤改 PayUni 上的其他資料: %+v", got)
	}
	if !repo.doneIDs[4242] {
		t.Errorf("成功的 row 必須 MarkDone")
	}
}

func TestPayuniMerchantProfileOutboxWorker_FailureIsRetried(t *testing.T) {
	updater := &fakeProfileUpdater{err: errors.New("payuni unavailable")}
	repo := newFakePayuniProfileOutboxRepo(mustRow(t, 4243, 202, 303, "台北市信義區松高路1號", "110"))
	w := NewPayuniMerchantProfileOutboxWorker(repo, updater, versionLookupStub(1), testPayuniProfileMetrics)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce 應吸收單筆失敗: %v", err)
	}
	if repo.doneIDs[4243] {
		t.Errorf("失敗的 row 不可 MarkDone")
	}
	if repo.attempts[4243] != 1 {
		t.Errorf("attempt_count = %d, want 1", repo.attempts[4243])
	}
	if _, failed := repo.failedIDs[4243]; failed {
		t.Errorf("第一次失敗不該直接終結，要留給重試")
	}
}

// 用盡重試才終結。attempt_count 已達上限-1 的 row 這一輪失敗就進 failed，
// 不再排下一次——否則會無限重打 PayUni。
func TestPayuniMerchantProfileOutboxWorker_ExhaustedRetriesTerminate(t *testing.T) {
	updater := &fakeProfileUpdater{err: errors.New("payuni unavailable")}
	row := mustRow(t, 4244, 202, 303, "台北市信義區松高路1號", "110")
	row.AttemptCount = pmpoMaxAttempts - 1
	repo := newFakePayuniProfileOutboxRepo(row)
	w := NewPayuniMerchantProfileOutboxWorker(repo, updater, versionLookupStub(1), testPayuniProfileMetrics)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if _, failed := repo.failedIDs[4244]; !failed {
		t.Errorf("用盡重試的 row 必須 MarkFailed")
	}
	if _, scheduled := repo.attempts[4244]; scheduled {
		t.Errorf("已終結的 row 不該再排下一次重試")
	}
}

// 搶佔閘門：claim 輸掉的 row 一次 PayUni 都不能打——update_merchant 是對外的
// 寫入呼叫，兩個 replica 同時送就是兩次真實變更（多副本排程搶佔規範 §一 ②）。
func TestPayuniMerchantProfileOutboxWorker_LostClaimSkipsPayuni(t *testing.T) {
	updater := &fakeProfileUpdater{}
	repo := newFakePayuniProfileOutboxRepo(mustRow(t, 4245, 202, 303, "台北市信義區松高路1號", "110"))
	repo.ClaimLost = true
	w := NewPayuniMerchantProfileOutboxWorker(repo, updater, versionLookupStub(1), testPayuniProfileMetrics)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.ClaimCalls) != 1 {
		t.Fatalf("Claim 呼叫 %d 次，want 1", len(repo.ClaimCalls))
	}
	if len(updater.calls) != 0 {
		t.Errorf("claim 輸掉卻仍呼叫了 PayUni %d 次", len(updater.calls))
	}
	if repo.doneIDs[4245] {
		t.Errorf("claim 輸掉的 row 不可 MarkDone")
	}
}

// 版本讀不到就算失敗重試，不能拿 0 當版本硬送——那會被 UpdateMerchantProfile
// 的 CAS 擋掉，卻先白白佔用一組 idempotency key。
func TestPayuniMerchantProfileOutboxWorker_VersionLookupFailureRetries(t *testing.T) {
	updater := &fakeProfileUpdater{}
	repo := newFakePayuniProfileOutboxRepo(mustRow(t, 4246, 202, 303, "台北市信義區松高路1號", "110"))
	lookup := func(context.Context, int64) (int64, error) { return 0, errors.New("db down") }
	w := NewPayuniMerchantProfileOutboxWorker(repo, updater, lookup, testPayuniProfileMetrics)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("版本讀不到卻仍呼叫了 PayUni %d 次", len(updater.calls))
	}
	if repo.attempts[4246] != 1 {
		t.Errorf("attempt_count = %d, want 1", repo.attempts[4246])
	}
}

// 三張 outbox 表的 metrics 名稱必須互不相同：outbox.NewMetrics 走 promauto，
// 同一組 (metric name, outbox_name) 註冊兩次會 panic，而那只在 pod 啟動時才炸，
// fx.ValidateApp 驗圖不建構，抓不到。這裡實際建構一次三個 provider 來守。
func TestOutboxMetricsNamesDoNotCollide(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("outbox metrics 名稱撞了，pod 會在啟動時 panic: %v", r)
		}
	}()
	_ = newOutboxMetrics()
	_ = newMerchantVerificationOutboxMetrics()
	_ = newPayuniMerchantProfileOutboxMetrics()
}
