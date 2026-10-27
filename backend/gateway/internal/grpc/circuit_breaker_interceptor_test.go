package grpc

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestLoadBreakerConfig_Defaults(t *testing.T) {
	cfg := loadBreakerConfig()
	if cfg.minRequests != 10 {
		t.Errorf("minRequests = %d, want 10", cfg.minRequests)
	}
	if cfg.failureRatio != 0.5 {
		t.Errorf("failureRatio = %v, want 0.5", cfg.failureRatio)
	}
	if cfg.timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", cfg.timeout)
	}
	if cfg.halfOpenMax != 1 {
		t.Errorf("halfOpenMax = %d, want 1", cfg.halfOpenMax)
	}
	if cfg.interval != 60*time.Second {
		t.Errorf("interval = %v, want 60s", cfg.interval)
	}
}

func TestBreakerIsSuccessful(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool // true = 視為成功（不計失敗）
	}{
		{"nil", nil, true},
		{"unavailable", status.Error(codes.Unavailable, "down"), false},
		{"deadline", status.Error(codes.DeadlineExceeded, "slow"), false},
		// #3230：ResourceExhausted 在本 repo 全部承載業務語意（節流／配額／上限），
		// 不是「該 target 不健康」的證據 ⇒ 不得計入失敗。
		// 這一列曾經是 false，正是它 pin 住了缺陷本身。
		{"resource_exhausted_是業務節流不是過載", status.Error(codes.ResourceExhausted, "[9352] 重寄通知過於頻繁"), true},
		{"invalid_argument", status.Error(codes.InvalidArgument, "bad"), true},
		{"not_found", status.Error(codes.NotFound, "missing"), true},
		{"failed_precondition", status.Error(codes.FailedPrecondition, "biz"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := breakerIsSuccessful(c.err); got != c.want {
				t.Errorf("breakerIsSuccessful(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// cbTestInvoker 回固定 error，計數呼叫次數。
func cbTestInvoker(retErr error, calls *int) grpc.UnaryInvoker {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		*calls++
		return retErr
	}
}

// dialTarget 建一個只用來提供 cc.Target() 的 ClientConn（不實際連線）。
func dialTarget(t *testing.T, target string) *grpc.ClientConn {
	t.Helper()
	cc, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient(%q): %v", target, err)
	}
	t.Cleanup(func() { cc.Close() })
	return cc
}

func TestBreaker_OpensAndFailFast(t *testing.T) {
	resetBreakers()
	cfg := breakerConfig{minRequests: 3, failureRatio: 0.5, timeout: time.Minute, halfOpenMax: 1, interval: time.Minute}
	intercept := circuitBreakerInterceptorWith(cfg)
	cc := dialTarget(t, "dns:///svc-a:50051")

	calls := 0
	inv := cbTestInvoker(status.Error(codes.Unavailable, "down"), &calls)

	for i := 0; i < 5; i++ {
		_ = intercept(context.Background(), "/m", nil, nil, cc, inv)
	}
	callsBeforeOpen := calls

	err := intercept(context.Background(), "/m", nil, nil, cc, inv)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable after open, got %v", err)
	}
	if !strings.Contains(err.Error(), "circuit open") {
		t.Fatalf("expected 'circuit open' in error, got %q", err.Error())
	}
	if calls != callsBeforeOpen {
		t.Fatalf("invoker called after open (fail-fast broken): before=%d after=%d", callsBeforeOpen, calls)
	}
}

func TestBreaker_BusinessErrorsDoNotTrip(t *testing.T) {
	resetBreakers()
	cfg := breakerConfig{minRequests: 3, failureRatio: 0.5, timeout: time.Minute, halfOpenMax: 1, interval: time.Minute}
	intercept := circuitBreakerInterceptorWith(cfg)
	cc := dialTarget(t, "dns:///svc-b:50051")

	calls := 0
	inv := cbTestInvoker(status.Error(codes.InvalidArgument, "bad"), &calls)

	for i := 0; i < 20; i++ {
		err := intercept(context.Background(), "/m", nil, nil, cc, inv)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("call %d: expected InvalidArgument passthrough, got %v", i, err)
		}
	}
	if calls != 20 {
		t.Fatalf("business errors tripped breaker: invoker called %d/20 times", calls)
	}
}

// #3230：單一使用者踩滿自己的業務配額，不得讓全體使用者的該服務所有端點 503。
//
// breaker 以 cc.Target() 為 key、一個服務的所有路由共用一顆 ⇒ 若 ResourceExhausted
// 計入失敗，一個賣家連按「重寄通知」踩滿 9352 節流（或踩滿匯入 job 上限），就會把
// 整個 invoice-service／product-import-service 的 breaker 推開。
//
// 這一支驗的是**行為**（breaker 不開、invoker 每次都被呼叫），不是判定函式的回傳值
// ——後者由 TestBreakerIsSuccessful 守，兩者缺一都不足以擋住這個缺陷。
func TestBreaker_QuotaErrorsDoNotTripAcrossUsers(t *testing.T) {
	resetBreakers()
	cfg := breakerConfig{minRequests: 3, failureRatio: 0.5, timeout: time.Minute, halfOpenMax: 1, interval: time.Minute}
	intercept := circuitBreakerInterceptorWith(cfg)
	cc := dialTarget(t, "dns:///invoice-service:10041")

	calls := 0
	// 同一個賣家連續踩滿節流，服務端回 9352 -> ResourceExhausted
	inv := cbTestInvoker(status.Error(codes.ResourceExhausted, "[9352] 重寄通知過於頻繁"), &calls)

	for i := 0; i < 20; i++ {
		err := intercept(context.Background(), "/invoice.InvoiceService/ResendInvoiceNotification", nil, nil, cc, inv)
		if status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("call %d: 業務節流碼必須原樣透傳，got %v", i, err)
		}
	}
	if calls != 20 {
		t.Fatalf("業務配額錯誤推開了 breaker：invoker 只被呼叫 %d/20 次 —— "+
			"這代表其他賣家的發票端點也會一起 503", calls)
	}
}

func TestBreaker_PerTargetIsolation(t *testing.T) {
	resetBreakers()
	cfg := breakerConfig{minRequests: 3, failureRatio: 0.5, timeout: time.Minute, halfOpenMax: 1, interval: time.Minute}
	intercept := circuitBreakerInterceptorWith(cfg)
	ccA := dialTarget(t, "dns:///svc-x:50051")
	ccB := dialTarget(t, "dns:///svc-y:50051")

	var a, b int
	down := cbTestInvoker(status.Error(codes.Unavailable, "down"), &a)
	ok := cbTestInvoker(nil, &b)

	for i := 0; i < 5; i++ {
		_ = intercept(context.Background(), "/m", nil, nil, ccA, down)
	}
	if status.Code(intercept(context.Background(), "/m", nil, nil, ccA, down)) != codes.Unavailable {
		t.Fatal("A should be open")
	}
	if err := intercept(context.Background(), "/m", nil, nil, ccB, ok); err != nil {
		t.Fatalf("B should be unaffected by A open, got %v", err)
	}
}

func TestBreaker_HalfOpenRecovers(t *testing.T) {
	resetBreakers()
	// timeout 設極短，讓 open → half-open 快速發生
	cfg := breakerConfig{minRequests: 3, failureRatio: 0.5, timeout: 20 * time.Millisecond, halfOpenMax: 1, interval: time.Minute}
	intercept := circuitBreakerInterceptorWith(cfg)
	cc := dialTarget(t, "dns:///svc-recover:50051")

	var calls int
	failing := cbTestInvoker(status.Error(codes.Unavailable, "down"), &calls)
	healthy := cbTestInvoker(nil, &calls)

	// 打到 open
	for i := 0; i < 5; i++ {
		_ = intercept(context.Background(), "/m", nil, nil, cc, failing)
	}
	if status.Code(intercept(context.Background(), "/m", nil, nil, cc, failing)) != codes.Unavailable {
		t.Fatal("expected open")
	}

	// 等過 timeout → half-open；探測成功 → closed
	time.Sleep(40 * time.Millisecond)
	if err := intercept(context.Background(), "/m", nil, nil, cc, healthy); err != nil {
		t.Fatalf("half-open probe should pass through, got %v", err)
	}
	// 已 closed：再來一個成功請求應正常
	if err := intercept(context.Background(), "/m", nil, nil, cc, healthy); err != nil {
		t.Fatalf("expected closed after successful probe, got %v", err)
	}
}
