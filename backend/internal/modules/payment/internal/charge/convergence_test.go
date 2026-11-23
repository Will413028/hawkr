package charge

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// 搬錢呼叫的結果三分：終局拒絕、可重試拒絕、其餘一律不明；兩個哨兵同時出現時終局優先。
// mutation：classifyCall 把一般錯誤歸成 callRetryable → 不明的結果被換新鍵，下面「一般錯誤」列失敗。
func TestClassifyCallSeparatesRefusalsFromUnknownOutcomes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want callOutcome
	}{
		{"success", nil, callSucceeded},
		{"permanent", fmt.Errorf("x：%w", ErrPermanent), callRefused},
		{"retryable refusal", fmt.Errorf("x：%w", ErrRetryableRefusal), callRetryable},
		{"a plain error is unknown", errors.New("連線逾時"), callUnknown},
		{"a deadline is unknown", context.DeadlineExceeded, callUnknown},
		{"permanent wins over retryable", errors.Join(ErrRetryableRefusal, ErrPermanent), callRefused},
	}
	for _, c := range cases {
		if got := classifyCall(c.err); got != c.want {
			t.Errorf("%s：%v，預期 %v", c.name, got, c.want)
		}
	}
}

// 同一把鍵只在 ResendWindow 內重送；從沒呼叫過的列永遠可以呼叫。
// mutation：inResendWindow 改成永遠 true → 超過視窗仍重送，下面的 false 列失敗。
func TestInResendWindowStopsExactlyAtTheWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	called := func(ago time.Duration) *time.Time { v := now.Add(-ago); return &v }
	for _, c := range []struct {
		name   string
		called *time.Time
		want   bool
	}{
		{"never called", nil, true},
		{"just called", called(time.Minute), true},
		{"one second before the window ends", called(ResendWindow - time.Second), true},
		{"at the window", called(ResendWindow), false},
		{"past the window", called(ResendWindow + time.Hour), false},
	} {
		if got := inResendWindow(c.called, now); got != c.want {
			t.Errorf("%s：%v，預期 %v", c.name, got, c.want)
		}
	}
}

// 重送的視窗在冪等鍵保存期之內，查無要在最後一次重送之後再等 NotFoundWait：兩個常數的關係不能被改壞。
func TestTheResendWindowAndTheNotFoundWaitKeepEveryResendInsideTheRetention(t *testing.T) {
	t.Parallel()
	if ResendWindow >= IdempotencyRetention {
		t.Fatalf("ResendWindow %v 必須小於 IdempotencyRetention %v", ResendWindow, IdempotencyRetention)
	}
	if NotFoundAfter != ResendWindow+NotFoundWait || NotFoundWait <= 0 {
		t.Fatalf("NotFoundAfter=%v，ResendWindow=%v，NotFoundWait=%v", NotFoundAfter, ResendWindow, NotFoundWait)
	}
	if LookupFailureLimit < 2 {
		t.Fatalf("LookupFailureLimit=%d：單次查回失敗不該進 ANOMALY", LookupFailureLimit)
	}
}
