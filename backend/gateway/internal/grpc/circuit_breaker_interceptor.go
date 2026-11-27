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

type breakerConfig struct {
	minRequests  uint32
	failureRatio float64
	timeout      time.Duration
	halfOpenMax  uint32
	interval     time.Duration
}

func breakerIsSuccessful(err error) bool {
	if err == nil {
		return true
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return false
	default:
		return true
	}
}

func loadBreakerConfig() breakerConfig {
	return breakerConfig{
		minRequests:  uint32(config.GetEnvAsIntOrDefault("GATEWAY_BREAKER_MIN_REQUESTS", 10)),
		failureRatio: 0.5,
		timeout:      config.GetEnvAsDurationOrDefault("GATEWAY_BREAKER_TIMEOUT", 10*time.Second),
		halfOpenMax:  uint32(config.GetEnvAsIntOrDefault("GATEWAY_BREAKER_HALF_OPEN_MAX", 1)),
		interval:     config.GetEnvAsDurationOrDefault("GATEWAY_BREAKER_INTERVAL", 60*time.Second),
	}
}

var breakers sync.Map

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
