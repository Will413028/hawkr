package grpc

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Will413028/hawkr/go-pkg/config"
	gobreaker "github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// breakerConfig holds the global circuit-breaker tuning, shared by every per-target breaker.
type breakerConfig struct {
	minRequests  uint32        // ReadyToTrip 樣本門檻，避免低流量誤觸
	failureRatio float64       // ReadyToTrip 失敗率門檻
	timeout      time.Duration // open → half-open 等待
	halfOpenMax  uint32        // half-open 探測併發數
	interval     time.Duration // closed 計數清零週期
}

// breakerIsSuccessful reports whether an invoker error counts as a SUCCESS
// for breaker purposes.
//
// 判準只有一條：**這個錯誤構不構成「該 target 自己不健康」的證據**。breaker 以
// cc.Target() 為 key、一個服務的所有路由共用一顆，它唯一的用途是「停止對一個已經
// 不健康的服務繼續打」。業務規則拒絕、資料未就緒、下游的下游失敗——都不是那個證據。
//
// 🔴 codes.ResourceExhausted 刻意**不**計入失敗（#3230）。本 repo 用它承載的全是業務
// 語意，五處無一例外：invoice 的 9352 重寄節流、ai 的配額、
// social 的檢舉限流、product-import 的兩條匯入 job 上限（Java 側，走同一套 DefaultDialOpts）。
// 讓它 trip 一顆 per-target 共用 breaker 的後果是：**單一使用者踩滿自己的業務配額，
// 就能讓全體使用者的該服務所有端點 503**。這與 pkg/sms 的 IsExcluded 是同一課
// （issue #2108：五個人打錯電話號碼會把第六個人鎖在門外）。
// gRPC 堆疊確實會在 MaxCallRecvMsgSize/MaxCallSendMsgSize 超限時客戶端合成此碼，但那是
// 單次呼叫的確定性 payload 故障、重試或等待都不會好，同樣不是 target 健康訊號。
//
// codes.Unavailable 與 DeadlineExceeded 維持計入：transport 層連不上時客戶端合成的
// 正是 Unavailable，把它排除會讓 breaker 對「服務真的下線」失明——而那是它存在的唯一理由。
// ⚠️ 因此服務端**不得**用 Unavailable 承載業務語意（目前 ai/account/payment 有五處誤用，
// 另案處理）：在 interceptor 這一層，伺服器送出的與客戶端合成的 Unavailable 同形，分不開。
func breakerIsSuccessful(err error) bool {
	if err == nil {
		return true
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return false
	default:
		// 只有 transport 層訊號才觸發 breaker；codes.Internal（可能是 server panic）
		// 與業務碼皆視為成功，避免因 application-layer 錯誤誤 trip。
		return true
	}
}

// loadBreakerConfig reads breaker tuning from env with safe defaults.
func loadBreakerConfig() breakerConfig {
	return breakerConfig{
		minRequests:  uint32(config.GetEnvAsIntOrDefault("GATEWAY_BREAKER_MIN_REQUESTS", 10)),
		failureRatio: 0.5, // 暫不 env 化（無 float helper）；env 化列 follow-up
		timeout:      config.GetEnvAsDurationOrDefault("GATEWAY_BREAKER_TIMEOUT", 10*time.Second),
		halfOpenMax:  uint32(config.GetEnvAsIntOrDefault("GATEWAY_BREAKER_HALF_OPEN_MAX", 1)),
		interval:     config.GetEnvAsDurationOrDefault("GATEWAY_BREAKER_INTERVAL", 60*time.Second),
	}
}

// breakers maps cc.Target() → its own breaker. Lazily populated.
var breakers sync.Map // map[string]*gobreaker.CircuitBreaker[any]

// resetBreakers clears the registry; test-only helper.
func resetBreakers() {
	breakers.Range(func(k, _ any) bool {
		breakers.Delete(k)
		return true
	})
}

func getBreaker(target string, cfg breakerConfig) *gobreaker.CircuitBreaker[any] {
	if cb, ok := breakers.Load(target); ok {
		return cb.(*gobreaker.CircuitBreaker[any])
	}
	cb := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        target,
		MaxRequests: cfg.halfOpenMax,
		Interval:    cfg.interval,
		Timeout:     cfg.timeout,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < cfg.minRequests {
				return false
			}
			ratio := float64(c.TotalFailures) / float64(c.Requests)
			return ratio >= cfg.failureRatio
		},
		IsSuccessful: breakerIsSuccessful,
		OnStateChange: func(name string, from, to gobreaker.State) {
			zap.L().Warn("circuit breaker state change",
				zap.String("target", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()))
		},
	})
	actual, _ := breakers.LoadOrStore(target, cb)
	return actual.(*gobreaker.CircuitBreaker[any])
}

// CircuitBreakerInterceptor wraps each downstream gRPC call in a per-target breaker.
// Placed as the INNERMOST chained interceptor so it observes raw transport errors
// (EnvelopeGuard's business-error translation happens in an outer layer).
func CircuitBreakerInterceptor() grpc.UnaryClientInterceptor {
	return circuitBreakerInterceptorWith(loadBreakerConfig())
}

func circuitBreakerInterceptorWith(cfg breakerConfig) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		cb := getBreaker(cc.Target(), cfg)
		_, err := cb.Execute(func() (any, error) {
			return nil, invoker(ctx, method, req, reply, cc, opts...)
		})
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return status.Error(codes.Unavailable, cc.Target()+": circuit open")
		}
		return err
	}
}
