package account

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// 賣家觸發的重讀的搶名額門檻（D33）：now 往前推 ReadDebounce。時間都是注入的值，不睡眠。
// mutation 1：staleBefore 回 now（沒有 debounce）→ 失敗。
// mutation 2：方向改成 now.Add(ReadDebounce)（永遠搶不到）→ 失敗。
func TestStaleBeforeIsOneDebounceWindowBack(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	if got, want := staleBefore(now), now.Add(-10*time.Second); !got.Equal(want) {
		t.Fatalf("staleBefore = %v，要 %v", got, want)
	}
	if ReadDebounce != 10*time.Second {
		t.Fatalf("ReadDebounce = %v，D33 是 10 秒", ReadDebounce)
	}
}

// 搶不到名額時的回答：快取已涵蓋占名額的讀取（fetched_at 不早於 claimed_at）才成功，否則是窗的剩餘時間（至少一秒）。時間都是注入的值。
// mutation 1：占名額的讀取失敗（fetched_at 早於 claimed_at）仍回 0 → 失敗案例 "failed read" 與 "in flight"。
// mutation 2：剩餘時間回 0 或固定值 → 失敗案例的 retry-after 不符。
func TestBusyRetryAfterDecidesBetweenSuccessAndRetry(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 11, 12, 0, 10, 0, time.UTC)
	at := func(sec int) *time.Time { v := time.Date(2026, 11, 11, 12, 0, sec, 0, time.UTC); return &v }
	cases := []struct {
		name             string
		claimed, fetched *time.Time
		want             time.Duration
	}{
		{"no seller claim, a job refreshed the cache", nil, at(8), 0},
		{"claimed read wrote the cache", at(5), at(5), 0},
		{"a later write covers the claim", at(5), at(9), 0},
		{"failed read: cache older than the claim", at(5), at(2), 5 * time.Second},
		{"failed read: never fetched", at(8), nil, 8 * time.Second},
		{"in flight: claim just taken", at(10), at(1), 10 * time.Second},
		{"window about to end", at(1), nil, time.Second},
		{"window already over: still at least one second", at(0), nil, time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := busyRetryAfter(tc.claimed, tc.fetched, now); got != tc.want {
				t.Fatalf("busyRetryAfter = %v，要 %v", got, tc.want)
			}
		})
	}
}

// 占名額的讀取沒有被快取涵蓋時是 ResourceExhausted 並帶窗的剩餘時間（傳輸層放進 RetryInfo）；涵蓋了是成功。
// mutation：busyOutcome 不帶 retryAfter → RetryAfter 為 0，失敗；失敗時回 nil → 失敗。
func TestBusyOutcomeCarriesTheRetryDelay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 11, 12, 0, 10, 0, time.UTC)
	claimed := time.Date(2026, 11, 11, 12, 0, 5, 0, time.UTC)
	err := busyOutcome(&claimed, nil, now)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindResourceExhausted || ae.RetryAfter != 5*time.Second {
		t.Fatalf("busyOutcome = %v，要帶 5 秒的 ResourceExhausted", err)
	}
	if err := busyOutcome(&claimed, &claimed, now); err != nil {
		t.Fatalf("快取已涵蓋占名額的讀取卻回 %v", err)
	}
}

// 沒有設定 provider 的賣家重讀是 FailedPrecondition，不是靜默成功。
// mutation：沒有 provider 時回 nil → 失敗。
func TestSyncUnlessFreshWithoutProviderIsFailedPrecondition(t *testing.T) {
	t.Parallel()
	s := New(Deps{DB: &db.DB{}, Events: events.NewBus(), Jobs: noopJobs{}, Now: time.Now, Logger: slog.Default()})
	if err := s.SyncUnlessFresh(context.Background(), 1); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("沒有 provider 的重讀是 %v，要 FailedPrecondition", err)
	}
}
