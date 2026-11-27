package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// TestCrossPeriodWindow 釘住 §8.2.3 M20 的窗口判定。
//
// 規格逐字：「條件是**台北時間的日**（單月 3–5 日 ⇒ closing、6 日起至下一期開始
// ⇒ closed、1–2 日 ⇒ open）」。
//
// 🔴 為什麼判定不能放進 PromQL（規格自己寫的理由）：PromQL 沒有時區安全的日曆
// 函式，寫成 hour／day_of_month 會用 UTC 算，**每兩個月的邊界日剛好錯 8 小時**
// ——而那正是本指標唯一有意義的那幾天。下面兩格 UTC 月底的 case 就是在守這件事。
func TestCrossPeriodWindow(t *testing.T) {
	utc := func(s string) time.Time {
		ts, err := time.Parse("2006-01-02T15:04:05Z", s)
		if err != nil {
			t.Fatalf("bad fixture %q: %v", s, err)
		}
		return ts
	}
	cases := []struct {
		name string
		at   string
		want CrossPeriodWindow
	}{
		{"單月 1 日 → open", "2026-09-01T04:00:00Z", WindowOpen},
		{"單月 2 日 → open", "2026-09-02T04:00:00Z", WindowOpen},
		{"單月 3 日 → closing", "2026-09-03T04:00:00Z", WindowClosing},
		{"單月 5 日 → closing", "2026-09-05T04:00:00Z", WindowClosing},
		{"單月 6 日 → closed", "2026-09-06T04:00:00Z", WindowClosed},
		{"單月 20 日 → closed", "2026-09-20T04:00:00Z", WindowClosed},
		{"雙月 1 日 → closed（不是 open）", "2026-10-01T04:00:00Z", WindowClosed},
		{"雙月 4 日 → closed（不是 closing）", "2026-10-04T04:00:00Z", WindowClosed},
		{"UTC 08-31T17:00 ＝ 台北 09-01 → open", "2026-08-31T17:00:00Z", WindowOpen},
		{"UTC 09-05T17:00 ＝ 台北 09-06 → closed", "2026-09-05T17:00:00Z", WindowClosed},
	}
	for _, c := range cases {
		at := utc(c.at)
		if got := crossPeriodWindow(at); got != c.want {
			t.Errorf("%s: got=%s want=%s（台北 %s）", c.name, got, c.want,
				at.In(timeutil.LocationTaipei).Format("2006-01-02 15:04"))
		}
	}
}

type noopGovStore struct{}

func (noopGovStore) ExpirePending(context.Context, time.Time, time.Time, int32) ([]domain.SellerTaxProfile, error) {
	return nil, nil
}

func (noopGovStore) CountApprovalRecheckDue(context.Context, time.Time) (int64, error) { return 0, nil }

type noopCrossPeriodCounter struct{}

func (noopCrossPeriodCounter) CountCrossPeriodPending(context.Context, time.Time) (int64, error) {
	return 0, nil
}

type noopGovObserver struct{}

func (noopGovObserver) ObserveCrossPeriodPending(CrossPeriodWindow, int64) {}
func (noopGovObserver) ObserveApprovalRecheckDue(int64)                    {}

type govRetentionFixture struct {
	purgeCalls   int
	purgeCutoff  time.Time
	purgeN       int64
	purgeErr     error
	deleteCalls  int
	deleteCutoff time.Time
	deleteN      int64
	deleteErr    error
}

func (f *govRetentionFixture) PurgeProcessedPayloads(_ context.Context, _, cutoff time.Time, _ int32) (int64, error) {
	f.purgeCalls++
	f.purgeCutoff = cutoff
	return f.purgeN, f.purgeErr
}

func (f *govRetentionFixture) DeleteDoneBefore(_ context.Context, cutoff time.Time, _ int32) (int64, error) {
	f.deleteCalls++
	f.deleteCutoff = cutoff
	return f.deleteN, f.deleteErr
}

// #3261 ①②③：INV.gov 每日 tick 要帶跑保留期清理與屆期待複審計數。
//
// 🔴 三者在 2026-09-08 之前都零呼叫端：inbox 的 payload 永久保留（含 buyer_id／
// store_id／金額）、outbox 的 done 列永不刪除、而「屆期待複審」連 metric 都不存在。
func TestGovernanceRunOnce_RunsRetentionAndRecheckCount(t *testing.T) {
	now := govNow(t)
	ret := &govRetentionFixture{purgeN: 3, deleteN: 7}
	obs := &recordingGovObserver{}
	uc := NewGovernance(noopGovStore{}, noopCrossPeriodCounter{}, obs,
		zap.NewNop(), func() time.Time { return now }).
		WithRetention(ret, ret)

	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if ret.purgeCalls != 1 || ret.deleteCalls != 1 {
		t.Fatalf("兩支清理都該跑到，實得 purge=%d delete=%d", ret.purgeCalls, ret.deleteCalls)
	}
	if want := now.Add(-90 * 24 * time.Hour); !ret.purgeCutoff.Equal(want) {
		t.Fatalf("inbox cutoff = %s，want %s（90 天）", ret.purgeCutoff, want)
	}
	if want := now.Add(-30 * 24 * time.Hour); !ret.deleteCutoff.Equal(want) {
		t.Fatalf("outbox cutoff = %s，want %s（30 天）", ret.deleteCutoff, want)
	}
	if !obs.recheckSet {
		t.Fatal("屆期待複審家數必須每一輪都 Set（0 也要），否則 Prometheus 分不出「是 0」與「沒更新」")
	}
}

// 清理失敗不得讓 RunOnce 回錯誤 —— 它們是盡力而為的維護工作，而同一個 tick 還帶著
// 行事曆同步與跨期計數。失敗記 log、下一輪再試。
func TestGovernanceRunOnce_RetentionFailureDoesNotFailTheTick(t *testing.T) {
	now := govNow(t)
	ret := &govRetentionFixture{purgeErr: errors.New("boom"), deleteErr: errors.New("boom")}
	obs := &recordingGovObserver{}
	uc := NewGovernance(noopGovStore{}, noopCrossPeriodCounter{}, obs,
		zap.NewNop(), func() time.Time { return now }).
		WithRetention(ret, ret)

	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("清理失敗不該讓整個 tick 失敗：%v", err)
	}
	if ret.purgeCalls != 1 || ret.deleteCalls != 1 {
		t.Fatalf("兩支都該被嘗試，實得 purge=%d delete=%d", ret.purgeCalls, ret.deleteCalls)
	}
}

// 沒 wire 清理時安靜跳過（本地 wiring 缺件），不得 panic。
func TestGovernanceRunOnce_NilRetentionIsSkipped(t *testing.T) {
	now := govNow(t)
	uc := NewGovernance(noopGovStore{}, noopCrossPeriodCounter{}, &recordingGovObserver{},
		zap.NewNop(), func() time.Time { return now })
	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("未掛清理時應照常跑完：%v", err)
	}
}

type recordingGovObserver struct {
	recheckSet   bool
	recheckCount int64
}

func (o *recordingGovObserver) ObserveCrossPeriodPending(CrossPeriodWindow, int64) {}
func (o *recordingGovObserver) ObserveApprovalRecheckDue(count int64) {
	o.recheckSet = true
	o.recheckCount = count
}

func govNow(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, "2026-09-04T04:00:00Z")
	if err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return ts
}
