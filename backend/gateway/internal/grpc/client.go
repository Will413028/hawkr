package grpc

import (
	"github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"google.golang.org/grpc"
)

func getEnvOrDefault(key, defaultVal string) string {
	return config.GetEnvOrDefault(key, defaultVal)
}

// DefaultDialOpts returns the standard gRPC client dial options used by all Gateway clients.
// Includes: OTel tracing and the unary interceptor chain: auth metadata forwarding,
// envelope guard, circuit breaker.
//
// Transport credentials are supplied by grpcclient.Dial (every call site dials via
// grpcclient.Dial(target, DefaultDialOpts()...), which prepends insecure creds — same
// final option order as before this migration: insecure creds, then stats handler,
// then chain interceptors).
//
// Interceptor chain order matters: CircuitBreaker is INNERMOST (last) so it observes raw
// transport errors only. EnvelopeGuard runs in an outer layer, translating business errors
// (resp.Code != 0) into gRPC errors — keeping those out of the breaker's failure counts.
func DefaultDialOpts() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
		grpc.WithChainUnaryInterceptor(
			AuthMetadataInterceptor(),
			EnvelopeGuardInterceptor(),
			CircuitBreakerInterceptor(),
		),
	}
}
