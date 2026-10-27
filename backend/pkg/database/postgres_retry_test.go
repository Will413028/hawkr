package database

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var fastPolicy = retryPolicy{base: 4 * time.Millisecond, max: 16 * time.Millisecond}

func pingSeq(errs ...error) (func(context.Context) error, *int) {
	calls := 0
	return func(context.Context) error {
		i := calls
		calls++
		if i >= len(errs) {
			i = len(errs) - 1
		}
		return errs[i]
	}, &calls
}

func TestPingWithRetry_SucceedsAfterTransientFailures(t *testing.T) {
	transient := errors.New("dial tcp: connection refused")
	ping, calls := pingSeq(transient, transient, nil)

	if err := pingWithRetry(context.Background(), ping, time.Second, fastPolicy); err != nil {
		t.Fatalf("DB 在預算內回來時應該成功，實得 error: %v", err)
	}
	if *calls != 3 {
		t.Errorf("應該 Ping 3 次（2 次失敗 + 1 次成功），實得 %d", *calls)
	}
}

func TestPingWithRetry_ZeroBudgetDoesNotRetry(t *testing.T) {
	transient := errors.New("dial tcp: connection refused")
	ping, calls := pingSeq(transient)

	if err := pingWithRetry(context.Background(), ping, 0, fastPolicy); err == nil {
		t.Fatal("預算 0 應該直接回 error")
	}
	if *calls != 1 {
		t.Errorf("預算 0 時只能 Ping 1 次（＝重構前的行為），實得 %d", *calls)
	}
}

func TestPingWithRetry_ExhaustsBudgetAndReportsLastError(t *testing.T) {
	transient := errors.New("dial tcp: connection refused")
	ping, calls := pingSeq(transient)

	start := time.Now()
	err := pingWithRetry(context.Background(), ping, 60*time.Millisecond, fastPolicy)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("預算用完應該回 error")
	}
	if !errors.Is(err, transient) {
		t.Errorf("應該包住最後一次的 error（%%w），實得：%v", err)
	}
	for _, want := range []string{"重試預算", "嘗試"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error 訊息要說明是重試預算用完與嘗試次數，缺 %q：%v", want, err)
		}
	}
	if *calls < 2 {
		t.Errorf("預算 > 0 時至少要重試一次，實得 Ping %d 次", *calls)
	}

	if elapsed > 300*time.Millisecond {
		t.Errorf("耗時 %v 明顯超出 60ms 預算，退避沒有尊重 deadline", elapsed)
	}
}

func TestPingWithRetry_DoesNotRetryNonTransientErrors(t *testing.T) {
	for code, desc := range nonRetryableSQLStates {
		pgErr := &pgconn.PgError{Code: code, Message: desc}
		ping, calls := pingSeq(pgErr)

		err := pingWithRetry(context.Background(), ping, 200*time.Millisecond, fastPolicy)
		if err == nil {
			t.Fatalf("SQLSTATE %s（%s）應該回 error", code, desc)
		}
		if !errors.Is(err, error(pgErr)) {
			t.Errorf("SQLSTATE %s 應原樣回傳，不包裝成重試預算錯誤：%v", code, err)
		}
		if *calls != 1 {
			t.Errorf("SQLSTATE %s（%s）重試不會變好，只能 Ping 1 次，實得 %d",
				code, desc, *calls)
		}
	}
}

func TestPingWithRetry_StopsRetryingWhenErrorTurnsNonTransient(t *testing.T) {
	transient := errors.New("dial tcp: connection refused")
	authErr := &pgconn.PgError{Code: "28P01", Message: "password authentication failed"}
	ping, calls := pingSeq(transient, authErr)

	err := pingWithRetry(context.Background(), ping, 200*time.Millisecond, fastPolicy)
	if !errors.Is(err, error(authErr)) {
		t.Fatalf("轉成認證失敗之後應立即回該 error，實得：%v", err)
	}
	if *calls != 2 {
		t.Errorf("應該在第 2 次 Ping 後停止，實得 %d 次", *calls)
	}
}

func TestPingWithRetry_HonoursContextCancellation(t *testing.T) {
	transient := errors.New("dial tcp: connection refused")
	ping, _ := pingSeq(transient)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := pingWithRetry(ctx, ping, 200*time.Millisecond, fastPolicy)
	if err == nil {
		t.Fatal("ctx 已取消時應該回 error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("應該包住 ctx.Err()，實得：%v", err)
	}
	if time.Since(start) > time.Second {
		t.Error("ctx 取消時不該還在等退避")
	}
}

func TestConnectRetryBudget_EnvOverride(t *testing.T) {
	cases := map[string]time.Duration{

		"":     60 * time.Second,
		"0":    0,
		"30":   30 * time.Second,
		"-5":   60 * time.Second,
		"abc":  60 * time.Second,
		"  12": 60 * time.Second,
	}
	for raw, want := range cases {
		t.Setenv("DB_CONNECT_RETRY_BUDGET", raw)
		if raw == "" {
			_ = os.Unsetenv("DB_CONNECT_RETRY_BUDGET")
		}
		if got := connectRetryBudget(); got != want {
			t.Errorf("DB_CONNECT_RETRY_BUDGET=%q → %v, want %v", raw, got, want)
		}
	}
}
