package grpc

import (
	"context"
	"errors"
	"io"
	"sync"

	gobreaker "github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// streamBreakers maps cc.Target() → its own TwoStepCircuitBreaker. Separate
// registry from unary's `breakers` sync.Map — TwoStepCircuitBreaker and
// CircuitBreaker are different gobreaker types with independent internal
// state; there is no public API to share one breaker's counters between
// them (see design spec §3.1's correction). Both registries are keyed by
// the same target string and use the same tuning, so they track correlated
// but not identical health signals for the same backend.
var streamBreakers sync.Map // map[string]*gobreaker.TwoStepCircuitBreaker[any]

func resetStreamBreakers() {
	streamBreakers.Range(func(k, _ any) bool {
		streamBreakers.Delete(k)
		return true
	})
}

func getStreamBreaker(target string, cfg breakerConfig) *gobreaker.TwoStepCircuitBreaker[any] {
	if cb, ok := streamBreakers.Load(target); ok {
		return cb.(*gobreaker.TwoStepCircuitBreaker[any])
	}
	cb := gobreaker.NewTwoStepCircuitBreaker[any](gobreaker.Settings{
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
			// 與 unary breaker 相同的告警，但多一個 kind="stream"：兩個 registry
			// 用同一個 target 當 Name，少了這個欄位就分不出跳的是哪一邊。事故當下
			// 「只看到 unary 的 log」會被讀成「stream 沒事」，正是這行要防的誤判。
			zap.L().Warn("circuit breaker state change",
				zap.String("kind", "stream"),
				zap.String("target", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()))
		},
	})
	actual, _ := streamBreakers.LoadOrStore(target, cb)
	return actual.(*gobreaker.TwoStepCircuitBreaker[any])
}

// breakerClientStream wraps a grpc.ClientStream so the FIRST RecvMsg call
// that returns a terminal result (io.EOF for a clean end, or any other
// non-nil error) reports that single outcome to the circuit breaker via
// `done`. Subsequent RecvMsg calls (which should not normally happen after
// a terminal result in real usage) do not record a second outcome.
type breakerClientStream struct {
	grpc.ClientStream
	done     func(err error)
	reported bool
	mu       sync.Mutex
}

func (s *breakerClientStream) RecvMsg(m any) error {
	err := s.ClientStream.RecvMsg(m)
	if err == nil {
		return nil // 還有更多訊息，stream 尚未結束
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.reported {
		s.reported = true
		if errors.Is(err, io.EOF) {
			s.done(nil) // 正常結束 = 成功
		} else {
			s.done(err) // breakerIsSuccessful 會判斷這是不是 transport 失敗
		}
	}
	return err
}

// CircuitBreakerStreamInterceptor is the streaming counterpart of
// CircuitBreakerInterceptor. Uses TwoStepCircuitBreaker instead of the
// unary version's Execute(): Allow() is called before streamer(...) (fails
// fast if the breaker is open, mirroring the unary interceptor's open-state
// behavior exactly); the returned `done` callback is deferred until the
// wrapped stream's RecvMsg observes a terminal result, so the whole stream
// lifecycle counts as exactly one breaker sample — not one per message.
func CircuitBreakerStreamInterceptor() grpc.StreamClientInterceptor {
	return circuitBreakerStreamInterceptorWith(loadBreakerConfig())
}

func circuitBreakerStreamInterceptorWith(cfg breakerConfig) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {

		cb := getStreamBreaker(cc.Target(), cfg)
		done, err := cb.Allow()
		if err != nil {
			if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
				return nil, status.Error(codes.Unavailable, cc.Target()+": circuit open")
			}
			return nil, err
		}

		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			done(err) // 開啟階段本身失敗，立刻記一次帳，不用等 RecvMsg
			return nil, err
		}

		return &breakerClientStream{ClientStream: stream, done: done}, nil
	}
}
